package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App           AppConfig           `mapstructure:"app"`
	Server        ServerConfig        `mapstructure:"server"`
	GRPC          GRPCConfig          `mapstructure:"grpc"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Kafka         KafkaConfig         `mapstructure:"kafka"`
	AWS           AWSConfig           `mapstructure:"aws"`
	Auth          AuthConfig          `mapstructure:"auth"`
	Banking       BankingConfig       `mapstructure:"banking"`
	Payment       PaymentConfig       `mapstructure:"payment"`
	Notification  NotificationConfig  `mapstructure:"notification"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
	Version     string `mapstructure:"version"`
	Debug       bool   `mapstructure:"debug"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	TLSCertFile     string        `mapstructure:"tls_cert_file"`
	TLSKeyFile      string        `mapstructure:"tls_key_file"`
}

type GRPCConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	Name           string `mapstructure:"name"`
	SSLMode        string `mapstructure:"ssl_mode"`
	MaxOpenConns   int    `mapstructure:"max_open_conns"`
	MaxIdleConns   int    `mapstructure:"max_idle_conns"`
	MigrationsPath string `mapstructure:"migrations_path"`
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

type RedisConfig struct {
	Addresses  []string      `mapstructure:"addresses"`
	Password   string        `mapstructure:"password"`
	DB         int           `mapstructure:"db"`
	MaxRetries int           `mapstructure:"max_retries"`
	PoolSize   int           `mapstructure:"pool_size"`
	TLSEnabled bool          `mapstructure:"tls_enabled"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type KafkaConfig struct {
	Brokers           []string      `mapstructure:"brokers"`
	ConsumerGroupID   string        `mapstructure:"consumer_group_id"`
	SecurityProtocol  string        `mapstructure:"security_protocol"`
	SASLMechanism     string        `mapstructure:"sasl_mechanism"`
	SASLUsername      string        `mapstructure:"sasl_username"`
	SASLPassword      string        `mapstructure:"sasl_password"`
	SessionTimeout    time.Duration `mapstructure:"session_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	MaxPollRecords    int           `mapstructure:"max_poll_records"`
}

type AWSConfig struct {
	Region          string `mapstructure:"region"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	KMSKeyID        string `mapstructure:"kms_key_id"`
	S3Bucket        string `mapstructure:"s3_bucket"`
}

type AuthConfig struct {
	JWTSecret          string        `mapstructure:"jwt_secret"`
	JWTExpiry          time.Duration `mapstructure:"jwt_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	BCryptCost         int           `mapstructure:"bcrypt_cost"`
	MFAIssuer          string        `mapstructure:"mfa_issuer"`
	MaxLoginAttempts   int           `mapstructure:"max_login_attempts"`
	LockoutDuration    time.Duration `mapstructure:"lockout_duration"`
	PasswordMinLength  int           `mapstructure:"password_min_length"`
	SessionMaxAge      time.Duration `mapstructure:"session_max_age"`
	RateLimitPerMinute int           `mapstructure:"rate_limit_per_minute"`
}

type BankingConfig struct {
	// Setu Account Aggregator
	SetuBaseURL      string `mapstructure:"setu_base_url"`
	SetuClientID     string `mapstructure:"setu_client_id"`
	SetuClientSecret string `mapstructure:"setu_client_secret"`
	SetuProductID    string `mapstructure:"setu_product_id"`

	// Plaid (Global)
	PlaidClientID    string `mapstructure:"plaid_client_id"`
	PlaidSecret      string `mapstructure:"plaid_secret"`
	PlaidEnvironment string `mapstructure:"plaid_environment"` // sandbox, development, production
	PlaidProducts    string `mapstructure:"plaid_products"`
	PlaidCountries   string `mapstructure:"plaid_countries"`

	// Yodlee (Alternative)
	YodleeBaseURL    string `mapstructure:"yodlee_base_url"`
	YodleeAdminLogin string `mapstructure:"yodlee_admin_login"`
	YodleeClientID   string `mapstructure:"yodlee_client_id"`
	YodleeSecret     string `mapstructure:"yodlee_secret"`

	SyncIntervalMinutes int `mapstructure:"sync_interval_minutes"`
}

type PaymentConfig struct {
	// Razorpay
	RazorpayKeyID     string `mapstructure:"razorpay_key_id"`
	RazorpayKeySecret string `mapstructure:"razorpay_key_secret"`

	// Stripe
	StripeSecretKey      string `mapstructure:"stripe_secret_key"`
	StripeWebhookSecret  string `mapstructure:"stripe_webhook_secret"`
	StripePublishableKey string `mapstructure:"stripe_publishable_key"`

	MaxScheduledPayments int `mapstructure:"max_scheduled_payments"`
}

type NotificationConfig struct {
	// AWS SES
	SESFromEmail string `mapstructure:"ses_from_email"`
	SESFromName  string `mapstructure:"ses_from_name"`
	SESRegion    string `mapstructure:"ses_region"`

	// AWS SNS for Push
	SNSPlatformARN string `mapstructure:"sns_platform_arn"`

	// Twilio for SMS
	TwilioAccountSID string `mapstructure:"twilio_account_sid"`
	TwilioAuthToken  string `mapstructure:"twilio_auth_token"`
	TwilioFromNumber string `mapstructure:"twilio_from_number"`

	// FCM for Push Notifications
	FCMServerKey string `mapstructure:"fcm_server_key"`
	FCMProjectID string `mapstructure:"fcm_project_id"`

	TemplatesPath string `mapstructure:"templates_path"`
}

type ObservabilityConfig struct {
	OTLPEndpoint    string  `mapstructure:"otlp_endpoint"`
	MetricsPort     int     `mapstructure:"metrics_port"`
	LogLevel        string  `mapstructure:"log_level"`
	LogFormat       string  `mapstructure:"log_format"`
	TraceSampleRate float64 `mapstructure:"trace_sample_rate"`
}

func Load(serviceName string) (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/finguard")
	v.AddConfigPath(fmt.Sprintf("/etc/finguard/%s", serviceName))

	v.SetEnvPrefix("FINGUARD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.version", "0.1.0")
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "15s")
	v.SetDefault("grpc.host", "0.0.0.0")
	v.SetDefault("grpc.port", 9090)
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.ssl_mode", "require")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.conn_max_idle_time", "1m")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.max_retries", 3)
	v.SetDefault("redis.pool_size", 50)
	v.SetDefault("redis.timeout", "5s")
	v.SetDefault("kafka.security_protocol", "SASL_SSL")
	v.SetDefault("kafka.sasl_mechanism", "SCRAM-SHA-512")
	v.SetDefault("kafka.session_timeout", "30s")
	v.SetDefault("kafka.heartbeat_interval", "10s")
	v.SetDefault("kafka.max_poll_records", 500)
	v.SetDefault("auth.jwt_expiry", "15m")
	v.SetDefault("auth.refresh_token_expiry", "168h")
	v.SetDefault("auth.bcrypt_cost", 12)
	v.SetDefault("auth.mfa_issuer", "FinGuard")
	v.SetDefault("auth.max_login_attempts", 5)
	v.SetDefault("auth.lockout_duration", "30m")
	v.SetDefault("auth.password_min_length", 12)
	v.SetDefault("auth.session_max_age", "24h")
	v.SetDefault("auth.rate_limit_per_minute", 60)
	v.SetDefault("banking.plaid_environment", "sandbox")
	v.SetDefault("banking.sync_interval_minutes", 30)
	v.SetDefault("payment.max_scheduled_payments", 50)
	v.SetDefault("observability.log_level", "info")
	v.SetDefault("observability.log_format", "json")
	v.SetDefault("observability.metrics_port", 9091)
	v.SetDefault("observability.trace_sample_rate", 0.1)
}
