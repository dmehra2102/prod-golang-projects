package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the complete runtime configuration for the service.
type Config struct {
	App      AppConfig
	Server   ServerConfig
	AWS      AWSConfig
	Aurora   AuroraConfig
	DynamoDB DynamoDBConfig
	Secrets  SecretsConfig
	OTEL     OTELConfig
}

type AppConfig struct {
	Name        string
	Environment string // local | staging | production
	Version     string
	LogLevel    string
	LogPretty   bool
}

type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	APIKeyHeader    string
}

type AWSConfig struct {
	Region          string
	AccessKeyID     string // empty in production (use IAM role)
	SecretAccessKey string // empty in production (use IAM role)
	Endpoint        string // empty in production; set for LocalStack
}

type AuroraConfig struct {
	// Connection details are fetched from Secrets Manager at startup.
	// These fields hold the secret name/ARN to look up.
	SecretName string
	DBName     string
	// Pool tuning.
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

type DynamoDBConfig struct {
	AuditTableName   string
	SessionTableName string
	// TTL field name in DynamoDB tables.
	TTLAttributeName string
}

type SecretsConfig struct {
	// SecretRotationInterval is how often we re-fetch secrets from AWS.
	// This handles automatic rotation by AWS Secrets Manager.
	RotationInterval time.Duration
	// AuroraDSNSecretName is the ARN/name of the Aurora credential secret.
	AuroraDSNSecretName string
	// EncryptionKeySecretName is the ARN/name of the field-encryption key.
	EncryptionKeySecretName string
	// APIKeySecretName is the ARN/name of the API keys secret.
	APIKeySecretName string
}

type OTELConfig struct {
	Enabled      bool
	Endpoint     string
	SamplingRate float64
}

func Load() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "securevault"),
			Environment: getEnv("APP_ENV", "local"),
			Version:     getEnv("APP_VERSION", "dev"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			LogPretty:   getEnvBool("LOG_PRETTY", true),
		},
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
			APIKeyHeader:    getEnv("API_KEY_HEADER", "X-API-Key"),
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "us-east-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
			Endpoint:        getEnv("AWS_ENDPOINT", ""), // e.g. http://localhost:4566 for LocalStack
		},
		Aurora: AuroraConfig{
			SecretName:        mustGetEnv("AURORA_SECRET_NAME"),
			DBName:            getEnv("AURORA_DB_NAME", "securevault"),
			MaxConns:          int32(getEnvInt("AURORA_MAX_CONNS", 25)),
			MinConns:          int32(getEnvInt("AURORA_MIN_CONNS", 5)),
			MaxConnLifetime:   getEnvDuration("AURORA_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:   getEnvDuration("AURORA_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthCheckPeriod: getEnvDuration("AURORA_HEALTH_CHECK_PERIOD", 1*time.Minute),
			ConnectTimeout:    getEnvDuration("AURORA_CONNECT_TIMEOUT", 5*time.Second),
		},
		DynamoDB: DynamoDBConfig{
			AuditTableName:   getEnv("DYNAMO_AUDIT_TABLE", "securevault-audit"),
			SessionTableName: getEnv("DYNAMO_SESSION_TABLE", "securevault-sessions"),
			TTLAttributeName: getEnv("DYNAMO_TTL_ATTR", "expires_at"),
		},
		Secrets: SecretsConfig{
			RotationInterval:        getEnvDuration("SECRETS_ROTATION_INTERVAL", 10*time.Minute),
			AuroraDSNSecretName:     mustGetEnv("AURORA_SECRET_NAME"),
			EncryptionKeySecretName: mustGetEnv("ENCRYPTION_KEY_SECRET_NAME"),
			APIKeySecretName:        mustGetEnv("API_KEY_SECRET_NAME"),
		},
		OTEL: OTELConfig{
			Enabled:      getEnvBool("OTEL_ENABLED", false),
			Endpoint:     getEnv("OTEL_ENDPOINT", "http://localhost:4318"),
			SamplingRate: getEnvFloat("OTEL_SAMPLING_RATE", 0.1),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	validEnvs := map[string]struct{}{"local": {}, "staging": {}, "production": {}}
	if _, ok := validEnvs[c.App.Environment]; !ok {
		return fmt.Errorf("config: APP_ENV must be one of: %s", strings.Join(keys(validEnvs), ", "))
	}
	if c.Aurora.MaxConns < c.Aurora.MinConns {
		return fmt.Errorf("config: AURORA_MAX_CONNS (%d) must be >= AURORA_MIN_CONNS (%d)",
			c.Aurora.MaxConns, c.Aurora.MinConns)
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("config: SERVER_PORT must be between 1 and 65535")
	}
	return nil
}

func (c *Config) IsProduction() bool { return c.App.Environment == "production" }

// --- helpers ---

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fmt.Sprintf("MISSING_%s", key)
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
