package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------
// Config is loaded from YAML then overlaid with env vars.
// In real production Kubernetes, you mount the YAML as a ConfigMap and use env vars
// (from Secrets) only for credentials. This two-layer approach lets you
// version-control non-sensitive config while keeping secrets out of git.
// -------------------------------------------------------------------------------
type Config struct {
	Service ServiceConfig `yaml:"service"`
	Kafka   KafkaConfig   `yaml:"kafka"`
	Metrics MetricsConfig `yaml:"metrics"`
	Health  HealthConfig  `yaml:"health"`
}

type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Env     string `yaml:"env"`
}

type KafkaConfig struct {
	Brokers          []string `yaml:"brokers"`
	SecurityProtocol string   `yaml:"security_protocol"`
	SASLMechanism    string   `yaml:"sasl_mechanism"`
	SASLUsername     string   `yaml:"sasl_username"`
	SASLPassword     string   `yaml:"sasl_password"`

	// Producer tuning
	Producer ProducerConfig `yaml:"producer"`

	// Consumer tuning.
	Consumer ConsumerConfig `yaml:"consumer"`

	// Topic declarations. We create topics programmatically rather than
	// relying on auto-creation, which is a ticking time bomb in prod
	Topics TopicConfig `yaml:"topics"`
}

type ProducerConfig struct {
	RequiredAcks int           `yaml:"required_acks"`
	MaxRetries   int           `yaml:"max_retries"`
	RetryBackoff time.Duration `yaml:"retry_backoff"`
	// IdempotentEnabled: true enables the idempotent producer (PID + sequence number).
	// Combined with acks=all, this gives you exactly-once producer semantics.
	IdempotentEnabled bool `yaml:"idempotent_enabled"`
	// Compression: lz4 is the sweet spot for latency-sensitive pipelines.
	// zstd gives better ratios but higher CPU. snappy is legacy.
	Compression string `yaml:"compression"`
	// LingerMs: how long to wait before sending a batch. Higher = better throughput,
	// higher latency. 5ms is a reasonable starting point.
	LingerMs int `yaml:"linger_ms"`
	// BatchSize: max bytes per batch. 64KB is conservative. Go up to 1MB for
	// throughput-heavy workloads, but watch your memory.
	BatchSize int `yaml:"batch_size"`
	// MaxInFlight: with idempotent=true, Kafka guarantees ordering even with
	// MaxInFlight=5. Without idempotent, set this to 1 or accept reordering.
	MaxInFlight int `yaml:"max_in_flight"`
}

type ConsumerConfig struct {
	GroupID string `yaml:"group_id"`
	// SessionTimeout: how long a consumer can be "silent" before the broker
	// considers it dead and triggers a rebalance. 30s is a safe default.
	// Lower = faster failover but more spurious rebalances.
	SessionTimeout time.Duration `yaml:"session_timeout"`
	// HeartbeatInterval: must be < SessionTimeout/3. This is the consumer's
	// "I'm alive" signal to the group coordinator.
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	// MaxPollInterval: max time between poll() calls. If your processing
	// takes longer than this, the consumer gets kicked from the group.
	// Set this based on your worst-case processing time.
	MaxPollInterval time.Duration `yaml:"max_poll_interval"`
	// OffsetInitial: -2 = earliest, -1 = latest. Use earliest for new consumer
	// groups that need full history, latest for real-time-only consumers.
	OffsetInitial int64 `yaml:"offset_initial"`
	// AutoCommit: DISABLED. We commit offsets manually after processing.
	// Auto-commit is the root cause of 90% of "lost messages" bugs because
	// it commits before processing completes.
	AutoCommitEnabled bool `yaml:"auto_commit_enabled"`
	// FetchMinBytes / FetchMaxWait: controls batching on the fetch side.
	// Higher FetchMinBytes = fewer round trips but higher latency.
	FetchMinBytes int           `yaml:"fetch_min_bytes"`
	FetchMaxWait  time.Duration `yaml:"fetch_max_wait"`
	// MaxProcessingWorkers: number of goroutines processing messages per partition.
	// >1 means you lose ordering within a partition — only do this if ordering
	// doesn't matter for your use case.
	MaxProcessingWorkers int `yaml:"max_processing_workers"`
	// DLQ settings
	DLQTopic     string        `yaml:"dlq_topic"`
	MaxRetries   int           `yaml:"max_retries"`
	RetryBackoff time.Duration `yaml:"retry_backoff"`
}

type TopicConfig struct {
	Transactions         TopicDef `yaml:"transactions"`
	FraudResults         TopicDef `yaml:"fraud_results"`
	EnrichedTransactions TopicDef `yaml:"enriched_transactions"`
	Notifications        TopicDef `yaml:"notifications"`
	DLQ                  TopicDef `yaml:"dlq"`
}

type TopicDef struct {
	Name              string            `yaml:"name"`
	Partitions        int32             `yaml:"partitions"`
	ReplicationFactor int16             `yaml:"replication_factor"`
	RetentionMs       int64             `yaml:"retention_ms"`
	CleanupPolicy     string            `yaml:"cleanup_policy"` // delete / compact / compact,delete
	MinISR            int               `yaml:"min_isr"`
	ExtraConfig       map[string]string `yaml:"extra_config,omitempty"`
}

type MetricsConfig struct {
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

type HealthConfig struct {
	Port int `yaml:"port"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Hard overrides from environment (secrets, broker discovery).
	applyEnvOverrides(&cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		cfg.Kafka.Brokers = strings.Split(v, ",")
	}
	if v := os.Getenv("KAFKA_SASL_USERNAME"); v != "" {
		cfg.Kafka.SASLUsername = v
	}
	if v := os.Getenv("KAFKA_SASL_PASSWORD"); v != "" {
		cfg.Kafka.SASLPassword = v
	}
	if v := os.Getenv("KAFKA_CONSUMER_GROUP_ID"); v != "" {
		cfg.Kafka.Consumer.GroupID = v
	}
	if v := os.Getenv("SERVICE_ENV"); v != "" {
		cfg.Service.Env = v
	}
	if v := os.Getenv("METRICS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Metrics.Port = port
		}
	}
}

func (c *Config) Validate() error {
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers is required")
	}
	if c.Service.Name == "" {
		return fmt.Errorf("service.name is required")
	}
	if c.Kafka.Producer.RequiredAcks == 0 {
		c.Kafka.Producer.RequiredAcks = -1 // all
	}
	if c.Kafka.Consumer.SessionTimeout == 0 {
		c.Kafka.Consumer.SessionTimeout = 30 * time.Second
	}
	if c.Kafka.Consumer.HeartbeatInterval == 0 {
		c.Kafka.Consumer.HeartbeatInterval = 10 * time.Second
	}
	if c.Kafka.Consumer.MaxPollInterval == 0 {
		c.Kafka.Consumer.MaxPollInterval = 5 * time.Minute
	}
	if c.Kafka.Consumer.MaxProcessingWorkers == 0 {
		c.Kafka.Consumer.MaxProcessingWorkers = 1
	}
	if c.Metrics.Port == 0 {
		c.Metrics.Port = 9090
	}
	if c.Health.Port == 0 {
		c.Health.Port = 8080
	}
	return nil
}

func (c *Config) IsProd() bool {
	return c.Service.Env == "prod" || c.Service.Env == "production"
}
