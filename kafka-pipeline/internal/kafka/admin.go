package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"go.uber.org/zap"
)

// TopicAdmin creates topics programmatically on startup.

type TopicAdmin struct {
	admin  sarama.ClusterAdmin
	logger *zap.Logger
}

// WHY NOT AUTO-CREATE:
// Kafka's auto.create.topics.enable=true creates topics with the cluster defaults,
// which are almost never what you want (usually 1 partition, RF=1).
// A typo in a topic name silently creates a garbage topic.
// Programmatic creation with validation prevents both issues.
// -------------------------------------------------------------------------------

func NewTopicAdmin(cfg *config.KafkaConfig, logger *zap.Logger) (*TopicAdmin, error) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V3_6_0_0

	if cfg.SecurityProtocol == "SASL_SSL" {
		saramaCfg.Net.TLS.Enable = true
		saramaCfg.Net.SASL.Enable = true
		saramaCfg.Net.SASL.Mechanism = sarama.SASLMechanism(cfg.SASLMechanism)
		saramaCfg.Net.SASL.User = cfg.SASLUsername
		saramaCfg.Net.SASL.Password = cfg.SASLPassword
	}

	admin, err := sarama.NewClusterAdmin(cfg.Brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("creating cluster admin: %w", err)
	}
	return &TopicAdmin{admin: admin, logger: logger}, nil
}

func (ta *TopicAdmin) EnsureTopics(topics []config.TopicDef) error {
	existing, err := ta.admin.ListTopics()
	if err != nil {
		return fmt.Errorf("listing topics: %w", err)
	}

	for _, topic := range topics {
		if _, exists := existing[topic.Name]; exists {
			ta.logger.Info("topic already exists", zap.String("topic", topic.Name))
			continue
		}

		configEntries := map[string]*string{
			"retention.ms":        strPtr(fmt.Sprintf("%d", topic.RetentionMs)),
			"cleanup.policy":      strPtr(topic.CleanupPolicy),
			"min.insync.replicas": strPtr(fmt.Sprintf("%d", topic.MinISR)),
		}
		for k, v := range topic.ExtraConfig {
			v := v // capture loop variable
			configEntries[k] = &v
		}

		detail := &sarama.TopicDetail{
			NumPartitions:     topic.Partitions,
			ReplicationFactor: topic.ReplicationFactor,
			ConfigEntries:     configEntries,
		}

		if err := ta.admin.CreateTopic(topic.Name, detail, false); err != nil {
			if topicErr, ok := err.(*sarama.TopicError); ok && topicErr.Err == sarama.ErrTopicAlreadyExists {
				ta.logger.Info("topic created concurrently by another instance", zap.String("topic", topic.Name))
				continue
			}
			return fmt.Errorf("creating topic %s: %w", topic.Name, err)
		}

		ta.logger.Info("topic created",
			zap.String("topic", topic.Name),
			zap.Int32("partitions", topic.Partitions),
			zap.Int16("replication_factor", topic.ReplicationFactor),
		)
	}

	return nil
}

func (ta *TopicAdmin) Close() error {
	return ta.admin.Close()
}

func strPtr(s string) *string {
	return &s
}
