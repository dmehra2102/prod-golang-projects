package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/dmehra2102/prod-golang-projects/travelmate/internal/config"
	"github.com/dmehra2102/prod-golang-projects/travelmate/internal/domain"
	"github.com/dmehra2102/prod-golang-projects/travelmate/pkg/logger"
)

// MessageHandler processes a single kafka message. If it returns an error,
// the message may be retried or sent to the DLQ depending on configuration.
type MessageHandler func(ctx context.Context, event *domain.Event) error

type ConsumerGroup struct {
	client sarama.ConsumerGroup
	topics []string
	handler *groupHandler
	log     *logger.Logger
	cfg     config.KafkaConfig
	metrics *ConsumerMetrics
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type ConsumerMetrics struct {
	mu                sync.Mutex
	messagesReceived  int64
	MessagesProcessed int64
	MessagesFailed    int64
	MessagesDLQ       int64
	LastProcessedAt   time.Time
	ProcessingLag     time.Duration
}

func (m *ConsumerMetrics) recordProcessed(lag time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MessagesProcessed++
	m.LastProcessedAt = time.Now()
	m.ProcessingLag = lag
}

func (m *ConsumerMetrics) recordFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesFailed++
}

func (m *ConsumerMetrics) recordDLQ() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesDLQ++
}

func (m *ConsumerMetrics) Snapshot() ConsumerMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	return ConsumerMetrics{
		messagesReceived: m.messagesReceived,
		MessagesProcessed: m.MessagesProcessed,
		MessagesFailed:    m.MessagesFailed,
		MessagesDLQ:       m.MessagesDLQ,
		LastProcessedAt:   m.LastProcessedAt,
		ProcessingLag:     m.ProcessingLag,
	}
}

// NewConsumerGroup creates a new Kafka consumer group.
func NewConsumerGroup(
	cfg config.KafkaConfig,
	topics []string,
	handler MessageHandler,
	dlqProducer *Producer,
	log *logger.Logger,
) (*ConsumerGroup, error) {
	saramCfg := consumerConfig(cfg)

	client,err := sarama.NewConsumerGroup(cfg.Brokers, cfg.ConsumerGroup, saramCfg)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer group: %w", err)
	}

	metrics := &ConsumerMetrics{}
	gh := &groupHandler{
		handler: handler,
		dlqProducer: dlqProducer,
		dlqSuffix: cfg.DLQ.TopicSuffix,
		maxRetries: cfg.DLQ.MaxRetries,
		retryBackoff: cfg.DLQ.RetryBackoff,
		log: log,
		metrics: metrics,
		ready: make(chan struct{}),
	}

	cg := &ConsumerGroup{
		client: client,
		topics: topics,
		handler: gh,
		log: log,
		cfg: cfg,
		metrics: metrics,
	}

	log.Info().
		Strs("topics", topics).
		Str("group", cfg.ConsumerGroup).
		Msg("kafka consumer group created")

	return cg, nil
}

func (cg *ConsumerGroup) Start(ctx context.Context) error {
	ctx, cg.cancel = context.WithCancel(ctx)

	cg.wg.Add(1)
	go func(){
		defer cg.wg.Done()
		for {
			if err := cg.client.Consume(ctx, cg.topics, cg.handler); err != nil {
				if ctx.Err() != nil {
					return
				}
				cg.log.Error().Err(err).Msg("consumer group error, restarting in 5s")
				time.Sleep(5 * time.Second)
				continue
			}

			if ctx.Err() != nil {
				return
			}
			// After a rebalance, reset the readiness channel.
			cg.handler.ready = make(chan struct{})
		}
	}()

	<-cg.handler.ready
	cg.log.Info().Msg("kafka consumer group is ready and consuming")

	cg.wg.Add(1)
	go func(){
		defer cg.wg.Done()
		for {
			select {
			case err, ok := <-cg.client.Errors():
				if !ok {
					return
				}
				cg.log.Error().Err(err).Msg("kafka consumer group error")
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	return cg.Close()
}

// Metrics returns a snapshot of consumer metrics.
func (cg *ConsumerGroup) Metrics() ConsumerMetrics {
	return cg.metrics.Snapshot()
}

// Close gracefully shuts down the consumer group.
func (cg *ConsumerGroup) Close() error {
	if cg.cancel != nil {
		cg.cancel()
	}
	cg.wg.Wait()
	if err := cg.client.Close(); err != nil {
		return fmt.Errorf("consumer group close: %w", err)
	}
	cg.log.Info().Msg("kafka consumer group closed")
	return nil
}

type groupHandler struct {
	handler MessageHandler
	dlqProducer *Producer
	dlqSuffix string
	maxRetries int
	retryBackoff time.Duration
	log *logger.Logger
	metrics *ConsumerMetrics
	ready chan struct{}
}

func consumerConfig(cfg config.KafkaConfig) *sarama.Config {
	c := sarama.NewConfig()
	c.Version = sarama.V3_5_0_0

	c.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	c.Consumer.Group.Heartbeat.Interval = cfg.HeartbeatInterval

	switch cfg.RebalanceStrategy {
	case "sticky":
		c.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategySticky()}
	case "roundrobin":
		c.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	default:
		c.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
	}

	c.Consumer.Offsets.Initial = cfg.OffsetsInitial
	c.Consumer.Offsets.AutoCommit.Enable = true
	c.Consumer.Offsets.AutoCommit.Interval = time.Second

	c.Consumer.Return.Errors = true

	c.Consumer.Fetch.Default = 1 << 20  // 1MB
	c.Consumer.Fetch.Max = 10 << 20     // 10MB
	c.Consumer.MaxProcessingTime = 30 * time.Second

	return c
}