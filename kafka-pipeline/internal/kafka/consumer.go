package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/metrics"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/models"
	"go.uber.org/zap"
)

type MessageHandler func(ctx context.Context, key []byte, value []byte, headers map[string]string) error

type ConsumerGroup struct {
	group   sarama.ConsumerGroup
	handler MessageHandler
	dlqProd *Producer
	topics  []string
	cfg     *config.KafkaConfig
	logger  *zap.Logger
	ready   chan struct{}
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewConsumerGroup(cfg *config.KafkaConfig, topics []string, handler MessageHandler, dlqProducer *Producer, logger *zap.Logger) (*ConsumerGroup, error) {
	saramaCfg := sarama.NewConfig()

	saramaCfg.Consumer.Group.Session.Timeout = cfg.Consumer.SessionTimeout
	saramaCfg.Consumer.Group.Heartbeat.Interval = cfg.Consumer.HeartbeatInterval
	saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		// CooperativeSticky: incremental rebalancing. Only migrates partitions
		// that need to move, instead of revoking ALL partitions and reassigning.
		// Reduces rebalance downtime from seconds to milliseconds.
		sarama.NewBalanceStrategySticky(),
	}
	saramaCfg.Consumer.MaxProcessingTime = cfg.Consumer.MaxPollInterval

	// Manual offset management
	saramaCfg.Consumer.Offsets.AutoCommit.Enable = false
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	if cfg.Consumer.OffsetInitial == -1 {
		saramaCfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	}

	saramaCfg.Consumer.Fetch.Min = int32(cfg.Consumer.FetchMinBytes)
	saramaCfg.Consumer.MaxWaitTime = cfg.Consumer.FetchMaxWait

	// Security
	if cfg.SecurityProtocol == "SASL_SSL" {
		saramaCfg.Net.TLS.Enable = true
		saramaCfg.Net.SASL.Enable = true
		saramaCfg.Net.SASL.Mechanism = sarama.SASLMechanism(cfg.SASLMechanism)
		saramaCfg.Net.SASL.User = cfg.SASLUsername
		saramaCfg.Net.SASL.Password = cfg.SASLPassword
		if cfg.SASLMechanism == "SCRAM-SHA-512" {
			saramaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
			}
		}
	}

	group, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.Consumer.GroupID, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("creating consumer group: %w", err)
	}

	return &ConsumerGroup{
		group:   group,
		handler: handler,
		dlqProd: dlqProducer,
		topics:  topics,
		cfg:     cfg,
		logger:  logger,
		ready:   make(chan struct{}),
	}, nil
}

func (cg *ConsumerGroup) Run(ctx context.Context) error {
	ctx, cg.cancel = context.WithCancel(ctx)

	cg.wg.Add(1)
	go func() {
		defer cg.wg.Done()

		for {
			handler := &groupHandler{
				handler: cg.handler,
				dlqProd: cg.dlqProd,
				cfg:     cg.cfg,
				logger:  cg.logger,
				ready:   cg.ready,
			}

			if err := cg.group.Consume(ctx, cg.topics, handler); err != nil {
				cg.logger.Error("consumer group session error", zap.Error(err))
			}
			if ctx.Err() != nil {
				return
			}
			// Reset ready channel for next session.
			cg.ready = make(chan struct{})
		}
	}()

	// Wait for the first session to be established.
	<-cg.ready
	cg.logger.Info("consumer group ready",
		zap.String("group", cg.cfg.Consumer.GroupID),
		zap.Strings("topics", cg.topics),
	)

	<-ctx.Done()
	cg.logger.Info("consumer group shutting down")
	cg.wg.Wait()
	return cg.group.Close()
}

// Ready returns a channel that closes when the consumer is ready.
func (cg *ConsumerGroup) Ready() <-chan struct{} {
	return cg.ready
}

// -------------------------------------------------------------------------------
// groupHandler implements sarama.ConsumerGroupHandler.
// A new instance is created for each rebalance session.
// -------------------------------------------------------------------------------
type groupHandler struct {
	handler MessageHandler
	dlqProd *Producer
	cfg     *config.KafkaConfig
	logger  *zap.Logger
	ready   chan struct{}
}

// Setup is called when the consumer group is (re)balanced and partitions are assigned.
func (h *groupHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.logger.Info("consumer group rebalanced — partitions assigned",
		zap.Any("claims", session.Claims()),
		zap.Int32("generation", session.GenerationID()),
	)
	metrics.ConsumerRebalances.WithLabelValues(h.cfg.Consumer.GroupID, "assigned").Inc()
	close(h.ready)
	return nil
}

// Cleanup is called when the session ends (before the next rebalance).
// Commit any pending offsets here.
func (h *groupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.logger.Info("consumer group cleanup — committing offsets")
	metrics.ConsumerRebalances.WithLabelValues(h.cfg.Consumer.GroupID, "revoked").Inc()
	session.Commit()
	return nil
}

// ConsumeClaim processes messages from a single partition.
// This is called in a separate goroutine per partition — sarama handles the fan-out.
func (h *groupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	topic := claim.Topic()
	partition := claim.Partition()
	groupID := h.cfg.Consumer.GroupID

	h.logger.Info("starting partition consumer",
		zap.String("topic", topic),
		zap.Int32("partition", partition),
	)

	for msg := range claim.Messages() {
		select {
		case <-session.Context().Done():
			return nil
		default:
		}

		start := time.Now()

		// Extract headers into a map for the handler.
		headers := make(map[string]string, len(msg.Headers))
		for _, hdr := range msg.Headers {
			headers[string(hdr.Key)] = string(hdr.Value)
		}

		// Process with retry → DLQ.
		err := h.processWithRetry(session.Context(), msg.Key, msg.Value, headers, topic)
		elapsed := time.Since(start).Seconds()
		metrics.ConsumeLatency.WithLabelValues(topic, groupID).Observe(elapsed)

		if err != nil {
			metrics.MessagesConsumed.WithLabelValues(topic, groupID, "dlq").Inc()
			h.logger.Error("message sent to DLQ after all retries",
				zap.String("topic", topic),
				zap.Int32("partition", partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
		} else {
			metrics.MessagesConsumed.WithLabelValues(topic, groupID, "success").Inc()
		}

		// WHY NOT COMMIT PER MESSAGE:
		// Committing per message is an RPC to the group coordinator per message.
		// At 10k msg/s, that's 10k RPCs/s just for offset commits. Instead,
		// we mark offsets and commit in batches.
		session.MarkMessage(msg, "")

		lag := max(claim.HighWaterMarkOffset()-msg.Offset-1, 0)

		metrics.ConsumerLag.WithLabelValues(
			topic, groupID, strconv.Itoa(int(partition)),
		).Set(float64(lag))
	}

	return nil
}

// processWithRetry attempts processing with exponential backoff.
// If all retries fail, the message is sent to the DLQ.
func (h *groupHandler) processWithRetry(ctx context.Context, key, value []byte, headers map[string]string, topic string) error {
	var lastErr error
	maxRetries := h.cfg.Consumer.MaxRetries
	backoff := h.cfg.Consumer.RetryBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			h.logger.Warn("retrying message processing",
				zap.String("topic", topic),
				zap.Int("attempt", attempt),
				zap.Int("max_retries", maxRetries),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Exponential backoff with a cap at 30s.
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		lastErr = h.handler(ctx, key, value, headers)
		if lastErr == nil {
			return nil
		}
	}

	// All retries exhausted → DLQ.
	if h.dlqProd != nil {
		h.sendToDLQ(ctx, key, value, headers, topic, lastErr)
	}

	return lastErr
}

func (h *groupHandler) sendToDLQ(ctx context.Context, key, value []byte, headers map[string]string, sourceTopic string, processingErr error) {
	envelope := models.DeadLetterEnvelope{
		OriginalTopic:   sourceTopic,
		OriginalKey:     string(key),
		OriginalValue:   value,
		OriginalHeaders: headers,
		ErrorMessage:    processingErr.Error(),
		ErrorType:       classifyError(processingErr),
		RetryCount:      h.cfg.Consumer.MaxRetries,
		MaxRetries:      h.cfg.Consumer.MaxRetries,
		FirstFailedAt:   time.Now().UTC(),
		LastFailedAt:    time.Now().UTC(),
		ServiceName:     "consumer",
	}

	dlqTopic := h.cfg.Consumer.DLQTopic
	_, _, err := h.dlqProd.ProduceMessage(ctx, dlqTopic, string(key), envelope, map[string]string{
		"dlq.source_topic": sourceTopic,
		"dlq.error_type":   envelope.ErrorType,
	})

	if err != nil {
		// If we can't even write to the DLQ, log loudly. This is a P0 alert situation.
		h.logger.Error("CRITICAL: failed to produce DLQ message — data loss risk",
			zap.String("source_topic", sourceTopic),
			zap.String("key", string(key)),
			zap.Error(err),
		)
	}
	metrics.DLQMessages.WithLabelValues(sourceTopic, envelope.ErrorType).Inc()
}

func classifyError(err error) string {
	if err == nil {
		return "NONE"
	}
	// In a real system, use error types or sentinel errors.
	switch {
	case isDeserializationError(err):
		return "DESERIALIZATION"
	case isTransientError(err):
		return "TRANSIENT"
	default:
		return "PROCESSING"
	}
}

func isDeserializationError(err error) bool {
	_, ok := err.(*json.SyntaxError)
	return ok
}

func isTransientError(_ error) bool {
	// In production, check for specific transient errors:
	// - context.DeadlineExceeded
	// - specific HTTP status codes from downstream services
	// - database connection errors
	return false
}
