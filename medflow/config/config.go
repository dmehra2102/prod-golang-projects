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
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Log       LogConfig
	Tracing   TracingConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Name        string
	Environment string
	Version     string
}

type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Host               string
	Port               int
	Name               string
	User               string
	Password           string
	SSLMode            string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	SlowQueryThreshold time.Duration
}

func (d DatabaseConfig) DNS() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s Timezone=UTC",
		d.Host, d.User, d.Password, d.Name, d.Port, d.SSLMode,
	)
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
}

type LogConfig struct {
	Level      string
	Format     string
	OutputPath string
}

type TracingConfig struct {
	Enabled     bool
	ServiceName string
	JaegerURL   string
	SampleRate  float64
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         time.Duration
}

type RateLimitConfig struct {
	// Global Rate limit per IP
	RequestsPerSecond float64
	BurstSize         int
	// Auth endpoints have stricter limits
	AuthRequestsPerMinute int
}

func Load() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "medflow-api"),
			Environment: getEnv("APP_ENV", "development"),
			Version:     getEnv("APP_VERSION", "0.0.0"),
		},
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			Host:               getEnv("DB_HOST", "localhost"),
			Port:               getEnvInt("DB_PORT", 5432),
			Name:               getEnv("DB_NAME", "medflow"),
			User:               getEnv("DB_USER", "medflow"),
			Password:           getEnv("DB_PASSWORD", ""),
			SSLMode:            getEnv("DB_SSLMODE", "require"),
			MaxOpenConns:       getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:       getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime:    getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime:    getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
			SlowQueryThreshold: getEnvDuration("DB_SLOW_QUERY_THRESHOLD", 200*time.Millisecond),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", ""),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			Issuer:          getEnv("JWT_ISSUER", "medflow-api"),
		},
		Log: LogConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "json"),
			OutputPath: getEnv("LOG_OUTPUT", "stdout"),
		},
		Tracing: TracingConfig{
			Enabled:     getEnvBool("TRACING_ENABLED", true),
			ServiceName: getEnv("TRACING_SERVICE_NAME", "medflow-api"),
			JaegerURL:   getEnv("JAEGER_ENDPOINT", "http://jaeger-collector:14268/api/traces"),
			SampleRate:  getEnvFloat("TRACING_SAMPLE_RATE", 0.1),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"https://app.medflow.io"}),
			AllowedMethods: getEnvSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
			AllowedHeaders: getEnvSlice("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type", "X-Request-ID"}),
			MaxAge:         getEnvDuration("CORS_MAX_AGE", 12*time.Hour),
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond:     getEnvFloat("RATE_LIMIT_RPS", 100),
			BurstSize:             getEnvInt("RATE_LIMIT_BURST", 200),
			AuthRequestsPerMinute: getEnvInt("RATE_LIMIT_AUTH_RPM", 10),
		},
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate enforces production security requirements.
func validate(cfg *Config) error {
	var errs []string

	if cfg.JWT.Secret == "" {
		errs = append(errs, "JWT_SECRET is required")
	} else if len(cfg.JWT.Secret) < 32 && cfg.App.Environment == "production" {
		errs = append(errs, "JWT_SECRET must be at least 32 characters in production")
	}

	if cfg.Database.Password == "" && cfg.App.Environment != "development" {
		errs = append(errs, "DB_PASSWORD is required in non-development environments")
	}

	if cfg.Database.SSLMode == "disable" && cfg.App.Environment == "production" {
		errs = append(errs, "DB_SSLMODE=disable is not allowed in production")
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
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

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
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

func getEnvSlice(key string, fallback []string) []string {
	if v, ok := os.LookupEnv(key); ok {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				result = append(result, t)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}
