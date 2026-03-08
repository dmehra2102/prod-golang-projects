package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App       AppConfig
	HTTP      HTTPConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Kafka     KafkaConfig
	AWS       AWSConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Name        string
	Environment string
	Version     string
	Debug       bool
	LogLevel    string
}

type HTTPConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxBodySize     int64
	TrustedProxies  []string
}

type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	MigrationsPath  string
}

type RedisConfig struct {
	Addresses    []string
	Password     string
	DB           int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
	MinIdleConns int
	ClusterMode  bool
}

type KafkaConfig struct {
	Brokers           []string
	ConsumerGroup     string
	ProducerRetries   int
	ProducerTimeout   time.Duration
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration
	RebalanceStrategy string
	OffsetsInitial    int64
	RequiredAcks      int
	EnableIdempotent  bool
	MaxMessageBytes   int
	Topics            KafkaTopics
	DLQ               DLQConfig
}

type KafkaTopics struct {
	TripEvents         string
	UserEvents         string
	FeedEvents         string
	NotificationEvents string
	MatchingEvents     string
	AnalyticsEvents    string
}

type DLQConfig struct {
	TopicSuffix  string
	MaxRetries   int
	RetryBackoff time.Duration
}

type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // for localstack in dev
	S3Bucket        string
	S3PresignExpiry time.Duration
	SQSQueueURL     string
	SNSTopicARN     string
	CloudWatchNS    string
	UseLocalStack   bool
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
	Audience        string
}

type RateLimitConfig struct {
	Enabled         bool
	RequestsPerSec  float64
	BurstSize       int
	WindowSize      time.Duration
	CleanupInterval time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "travelmate"),
			Environment: getEnv("APP_ENV", "development"),
			Version:     getEnv("APP_VERSION", "0.1.0"),
			Debug:       getEnvBool("APP_DEBUG", true),
			LogLevel:    getEnv("LOG_LEVEL", "debug"),
		},
		HTTP: HTTPConfig{
			Host:            getEnv("HTTP_HOST", "0.0.0.0"),
			Port:            getEnvInt("HTTP_PORT", 8080),
			ReadTimeout:     getEnvDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getEnvDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     getEnvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
			MaxBodySize:     int64(getEnvInt("HTTP_MAX_BODY_SIZE", 10<<20)), // 10MB
			TrustedProxies:  getEnvSlice("HTTP_TRUSTED_PROXIES", []string{}),
		},
		Postgres: PostgresConfig{
			Host:            getEnv("PG_HOST", "localhost"),
			Port:            getEnvInt("PG_PORT", 5432),
			User:            getEnv("PG_USER", "travelmate"),
			Password:        getEnv("PG_PASSWORD", "travelmate_secret"),
			Database:        getEnv("PG_DATABASE", "travelmate"),
			SSLMode:         getEnv("PG_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvInt("PG_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("PG_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("PG_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("PG_CONN_MAX_IDLE_TIME", 5*time.Minute),
			MigrationsPath:  getEnv("PG_MIGRATIONS_PATH", "migrations"),
		},
		Redis: RedisConfig{
			Addresses:    getEnvSlice("REDIS_ADDRESSES", []string{"localhost:6379"}),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			MaxRetries:   getEnvInt("REDIS_MAX_RETRIES", 3),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 100),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 10),
			ClusterMode:  getEnvBool("REDIS_CLUSTER_MODE", false),
		},
		Kafka: KafkaConfig{
			Brokers:           getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			ConsumerGroup:     getEnv("KAFKA_CONSUMER_GROUP", "travelmate-workers"),
			ProducerRetries:   getEnvInt("KAFKA_PRODUCER_RETRIES", 3),
			ProducerTimeout:   getEnvDuration("KAFKA_PRODUCER_TIMEOUT", 10*time.Second),
			SessionTimeout:    getEnvDuration("KAFKA_SESSION_TIMEOUT", 30*time.Second),
			HeartbeatInterval: getEnvDuration("KAFKA_HEARTBEAT_INTERVAL", 3*time.Second),
			RebalanceStrategy: getEnv("KAFKA_REBALANCE_STRATEGY", "sticky"),
			OffsetsInitial:    int64(getEnvInt("KAFKA_OFFSETS_INITIAL", -1)), // newest
			RequiredAcks:      getEnvInt("KAFKA_REQUIRED_ACKS", -1),          // all
			EnableIdempotent:  getEnvBool("KAFKA_ENABLE_IDEMPOTENT", true),
			MaxMessageBytes:   getEnvInt("KAFKA_MAX_MESSAGE_BYTES", 1<<20), // 1MB
			Topics: KafkaTopics{
				TripEvents:         getEnv("KAFKA_TOPIC_TRIP", "travelmate.trips"),
				UserEvents:         getEnv("KAFKA_TOPIC_USER", "travelmate.users"),
				FeedEvents:         getEnv("KAFKA_TOPIC_FEED", "travelmate.feed"),
				NotificationEvents: getEnv("KAFKA_TOPIC_NOTIFICATION", "travelmate.notifications"),
				MatchingEvents:     getEnv("KAFKA_TOPIC_MATCHING", "travelmate.matching"),
				AnalyticsEvents:    getEnv("KAFKA_TOPIC_ANALYTICS", "travelmate.analytics"),
			},
			DLQ: DLQConfig{
				TopicSuffix:  getEnv("KAFKA_DLQ_SUFFIX", ".dlq"),
				MaxRetries:   getEnvInt("KAFKA_DLQ_MAX_RETRIES", 3),
				RetryBackoff: getEnvDuration("KAFKA_DLQ_RETRY_BACKOFF", 5*time.Second),
			},
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "us-east-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
			Endpoint:        getEnv("AWS_ENDPOINT", ""),
			S3Bucket:        getEnv("AWS_S3_BUCKET", "travelmate-media"),
			S3PresignExpiry: getEnvDuration("AWS_S3_PRESIGN_EXPIRY", 15*time.Minute),
			SQSQueueURL:     getEnv("AWS_SQS_QUEUE_URL", ""),
			SNSTopicARN:     getEnv("AWS_SNS_TOPIC_ARN", ""),
			CloudWatchNS:    getEnv("AWS_CLOUDWATCH_NAMESPACE", "TravelMate"),
			UseLocalStack:   getEnvBool("AWS_USE_LOCALSTACK", false),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "CHANGE_ME_IN_PRODUCTION"),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			Issuer:          getEnv("JWT_ISSUER", "travelmate"),
			Audience:        getEnv("JWT_AUDIENCE", "travelmate-api"),
		},
		RateLimit: RateLimitConfig{
			Enabled:         getEnvBool("RATE_LIMIT_ENABLED", true),
			RequestsPerSec:  getEnvFloat("RATE_LIMIT_RPS", 100),
			BurstSize:       getEnvInt("RATE_LIMIT_BURST", 200),
			WindowSize:      getEnvDuration("RATE_LIMIT_WINDOW", time.Minute),
			CleanupInterval: getEnvDuration("RATE_LIMIT_CLEANUP", 5*time.Minute),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.App.Environment == "production" {
		if c.JWT.Secret == "CHANGE_ME_IN_PRODUCTION" {
			return fmt.Errorf("JWT secret must be changed in production")
		}
		if c.Postgres.SSLMode == "disable" {
			return fmt.Errorf("postgres SSL must be enabled in production")
		}
	}
	return nil
}

// DSN builds the PostgreSQL connection string.
func (c *PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return fallback
}
