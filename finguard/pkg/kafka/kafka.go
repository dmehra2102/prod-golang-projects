package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/config"
	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/logger"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
	"go.uber.org/zap"
)

const (
	TopicUserRegistered         = "finguard.user.registered"
	TopicUserVerified           = "finguard.user.verified"
	TopicBankAccountLinked      = "finguard.bank.account.linked"
	TopicBankAccountUnlinked    = "finguard.bank.account.unlinked"
	TopicTransactionSynced      = "finguard.transaction.synced"
	TopicTransactionCategorized = "finguard.transaction.categorized"
	TopicBudgetCreated          = "finguard.budget.created"
	TopicBudgetExceeded         = "finguard.budget.exceeded"
	TopicBudgetWarning          = "finguard.budget.warning"
	TopicPaymentScheduled       = "finguard.payment.scheduled"
	TopicPaymentExecuted        = "finguard.payment.executed"
	TopicPaymentFailed          = "finguard.payment.failed"
	TopicPaymentBlocked         = "finguard.payment.blocked"
	TopicNotificationSend       = "finguard.notification.send"
	TopicAlertTriggered         = "finguard.alert.triggered"
	TopicAnalyticsEvent         = "finguard.analytics.event"
)

type Event struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Source    string            `json:"source"`
	UserID    string            `json:"user_id,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Data      json.RawMessage   `json:"data"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func NewEvent(eventType, source, userID string, data any) (*Event, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	return &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		UserID:    userID,
		Timestamp: time.Now().UTC(),
		Data:      payload,
		Metadata:  make(map[string]string),
	}, nil
}

// ------------------ Kafka-Producer -------------------------

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(cfg config.KafkaConfig) (*Producer, error) {
	transport := &kafka.Transport{
		TLS: &tls.Config{MinVersion: tls.VersionTLS12},
	}

	if cfg.SASLMechanism != "" {
		mechanism, err := scram.Mechanism(scram.SHA512, cfg.SASLUsername, cfg.SASLPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to create SASL mechanism: %w", err)
		}
		transport.SASL = mechanism
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.Murmur2Balancer{},
		Transport:    transport,
		BatchTimeout: 10 * time.Millisecond,
		BatchSize:    100,
		Async:        false, // Synchronous for reliability
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  3,
		Compression:  kafka.Lz4,
	}

	return &Producer{writer: writer}, nil
}

func (p *Producer) Publish(ctx context.Context, topic string, event *Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(event.UserID),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.Type)},
			{Key: "event_id", Value: []byte(event.ID)},
			{Key: "source", Value: []byte(event.Source)},
			{Key: "timestamp", Value: []byte(event.Timestamp.Format(time.RFC3339Nano))},
		},
		Time: event.Timestamp,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		logger.Error(ctx, "failed to publish kafka event",
			zap.String("topic", topic),
			zap.String("event_type", event.Type),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish event: %w", err)
	}

	logger.Debug(ctx, "kafka event published",
		zap.String("topic", topic),
		zap.String("event_id", event.ID),
		zap.String("event_type", event.Type),
	)

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// ------------------ Kafka-Consumer --------------------------

type Consumer struct {
	reader  *kafka.Reader
	handler EventHandler
}

// EventHandler processes a Kafka event.
type EventHandler func(ctx context.Context, event *Event) error

func NewConsumer(cfg config.KafkaConfig, topics []string, handler EventHandler) (*Consumer, error) {
	dialer := &kafka.Dialer{
		Timeout: 10 * time.Second,
		TLS:     &tls.Config{MinVersion: tls.VersionTLS12},
	}

	if cfg.SASLMechanism != "" {
		mechanism, err := scram.Mechanism(scram.SHA512, cfg.SASLUsername, cfg.SASLPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to create SASL mechanism: %w", err)
		}
		dialer.SASLMechanism = mechanism
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           cfg.Brokers,
		GroupID:           cfg.ConsumerGroupID,
		GroupTopics:       topics,
		Dialer:            dialer,
		MinBytes:          1e3,
		MaxBytes:          10e6,
		MaxWait:           500 * time.Millisecond,
		CommitInterval:    time.Second,
		StartOffset:       kafka.LastOffset,
		SessionTimeout:    cfg.SessionTimeout,
		HeartbeatInterval: cfg.HeartbeatInterval,
	})

	return &Consumer{reader: reader, handler: handler}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	logger.Info(ctx, "kafka consumer started",
		zap.Strings("topics", c.reader.Config().GroupTopics),
		zap.String("group_id", c.reader.Config().GroupID),
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "kafka consumer shutting down")
			return c.reader.Close()
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				logger.Error(ctx, "failed to fetch kafka message", zap.Error(err))
				continue
			}

			var event Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				logger.Error(ctx, "failed to unmarshal kafka event",
					zap.Error(err),
					zap.String("topic", msg.Topic),
				)
				// Commit to avoid reprocessing malformed messages
				_ = c.reader.CommitMessages(ctx, msg)
				continue
			}

			if err := c.handler(ctx, &event); err != nil {
				logger.Error(ctx, "failed to handle kafka event",
					zap.Error(err),
					zap.String("event_id", event.ID),
					zap.String("event_type", event.Type),
				)
				// Implement dead letter queue logic here
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				logger.Error(ctx, "failed to commit kafka message",
					zap.Error(err),
					zap.String("event_id", event.ID),
				)
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

// TopicManager handles Kafka topic creation and management.
type TopicManager struct {
	conn *kafka.Conn
}

// EnsureTopics creates Kafka topics if they don't exist.
func EnsureTopics(brokers []string, topics []kafka.TopicConfig) error {
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get kafka controller: %w", err)
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer controllerConn.Close()

	if err := controllerConn.CreateTopics(topics...); err != nil {
		return fmt.Errorf("failed to create topics: %w", err)
	}

	return nil
}

// DefaultTopicConfigs returns topic configurations for all services.
func DefaultTopicConfigs() []kafka.TopicConfig {
	topics := []string{
		TopicUserRegistered, TopicUserVerified,
		TopicBankAccountLinked, TopicBankAccountUnlinked,
		TopicTransactionSynced, TopicTransactionCategorized,
		TopicBudgetCreated, TopicBudgetExceeded, TopicBudgetWarning,
		TopicPaymentScheduled, TopicPaymentExecuted, TopicPaymentFailed, TopicPaymentBlocked,
		TopicNotificationSend, TopicAlertTriggered,
		TopicAnalyticsEvent,
	}

	configs := make([]kafka.TopicConfig, len(topics))
	for i, topic := range topics {
		configs[i] = kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     12,
			ReplicationFactor: 3,
		}
	}

	return configs
}
