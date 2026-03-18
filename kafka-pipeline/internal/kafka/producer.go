package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/metrics"
	"go.uber.org/zap"
)

type Producer struct {
	producer sarama.SyncProducer
	cfg      *config.KafkaConfig
	logger   *zap.Logger
}

func NewProducer(cfg *config.KafkaConfig, logger *zap.Logger) (*Producer, error) {
	saramaCfg := sarama.NewConfig()

	// --- Idempotency ---
	saramaCfg.Producer.Idempotent = cfg.Producer.IdempotentEnabled
	saramaCfg.Net.MaxOpenRequests = cfg.Producer.MaxInFlight

	// --- Reliability ---
	saramaCfg.Producer.RequiredAcks = sarama.RequiredAcks(cfg.Producer.RequiredAcks)
	saramaCfg.Producer.Retry.Max = cfg.Producer.MaxRetries
	saramaCfg.Producer.Retry.Backoff = cfg.Producer.RetryBackoff
	saramaCfg.Producer.Return.Successes = true // Required for SyncProducer
	saramaCfg.Producer.Return.Errors = true

	// --- Throughput tuning ---
	switch cfg.Producer.Compression {
	case "lz4":
		saramaCfg.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaCfg.Producer.Compression = sarama.CompressionZSTD
	case "snappy":
		saramaCfg.Producer.Compression = sarama.CompressionSnappy
	case "gzip":
		saramaCfg.Producer.Compression = sarama.CompressionGZIP
	default:
		saramaCfg.Producer.Compression = sarama.CompressionNone
	}

	saramaCfg.Producer.Flush.Frequency = time.Duration(cfg.Producer.LingerMs) * time.Millisecond
	saramaCfg.Producer.Flush.Bytes = cfg.Producer.BatchSize

	// --- Security ---
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

	producer, err := sarama.NewSyncProducer(cfg.Brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("creating sync producer: %w", err)
	}

	logger.Info("kafka producer initialized",
		zap.Strings("brokers", cfg.Brokers),
		zap.Bool("idempotent", cfg.Producer.IdempotentEnabled),
		zap.String("compression", cfg.Producer.Compression),
	)

	return &Producer{
		producer: producer,
		cfg:      cfg,
		logger:   logger,
	}, nil
}

func (p *Producer) ProduceMessage(ctx context.Context, topic, key string, value any, headers map[string]string) (partition int32, offset int64, err error) {
	start := time.Now()

	// Serialize
	payload, err := json.Marshal(value)
	if err != nil {
		metrics.MessagesProduced.WithLabelValues(topic, "error").Inc()
		return 0, 0, fmt.Errorf("marshaling message: %w", err)
	}

	// Build headers. Always include trace context for distributed tracing.
	var recordHeaders []sarama.RecordHeader
	for k, v := range headers {
		recordHeaders = append(recordHeaders, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}
	// Inject produced-at timestamp for end-to-end latency tracking.
	recordHeaders = append(recordHeaders, sarama.RecordHeader{
		Key:   []byte("produced_at"),
		Value: []byte(time.Now().UTC().Format(time.RFC3339Nano)),
	})

	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Key:     sarama.StringEncoder(key),
		Value:   sarama.ByteEncoder(payload),
		Headers: recordHeaders,
	}

	partition, offset, err = p.producer.SendMessage(msg)
	elapsed := time.Since(start).Seconds()
	metrics.ProduceLatency.WithLabelValues(topic).Observe(elapsed)

	if err != nil {
		metrics.MessagesProduced.WithLabelValues(topic, "error").Inc()
		p.logger.Error("failed to produce message",
			zap.String("topic", topic),
			zap.String("key", key),
			zap.Error(err),
		)
		return 0, 0, fmt.Errorf("sending message to %s: %w", topic, err)
	}

	metrics.MessagesProduced.WithLabelValues(topic, "success").Inc()
	p.logger.Debug("message produced",
		zap.String("topic", topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
		zap.Float64("latency_sec", elapsed),
	)

	return partition, offset, err
}

// Close shuts down the producer, flushing any pending messages.
// ALWAYS defer this — unflushed messages are lost.
func (p *Producer) Close() error {
	p.logger.Info("shutting down kafka producer")
	return p.producer.Close()
}
