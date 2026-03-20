package runner

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/health"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Runner is the standard lifecycle manager for every microservice in the pipeline.
// Every service follows the same startup sequence:
//   1. Load config
//   2. Initialize logger
//   3. Ensure topics exist
//   4. Start health server (K8s probes start hitting immediately)
//   5. Start metrics server
//   6. Start application logic (producer/consumer)
//   7. Wait for SIGTERM
//   8. Graceful shutdown with deadline
// -------------------------------------------------------------------------------

type ServiceFunc func(ctx context.Context, cfg *config.Config, producer *kafka.Producer, logger *zap.Logger, healthSrv *health.Server) error

func Run(configPath string, serviceFn ServiceFunc) {
	// Step 1 : Load Config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to lead config: %v\n", err)
		os.Exit(1)
	}

	// Step 2 : Initialize logger
	logger := newLogger(cfg)
	defer logger.Sync()

	logger.Info("starting service",
		zap.String("name", cfg.Service.Name),
		zap.String("version", cfg.Service.Version),
		zap.String("env", cfg.Service.Env),
	)

	// Step 3 : Ensure Topics exist
	if err := ensureTopics(cfg, logger); err != nil {
		logger.Fatal("failed to ensure topics", zap.Error(err))
	}

	// Step 4 : Create Shared producer
	producer, err := kafka.NewProducer(&cfg.Kafka, logger.Named("producer"))
	if err != nil {
		logger.Fatal("failed to create producer", zap.Error(err))
	}
	defer producer.Close()

	// Step 5 : Start health server
	healthSrv := health.NewServer(cfg.Health.Port, logger.Named("health"))
	go func() {
		if err := healthSrv.Start(); err != nil {
			logger.Error("health server error", zap.Error(err))
		}
	}()

	// Step 6: Start metrics server
	go func() {
		mux := http.NewServeMux()
		mux.Handle(cfg.Metrics.Path, promhttp.Handler())
		addr := fmt.Sprintf(":%d", cfg.Metrics.Port)
		logger.Info("metrics server starting", zap.String("addr", addr))
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Error("metrics server error", zap.Error(err))
		}
	}()

	metrics.BuildInfo.WithLabelValues(cfg.Service.Name, cfg.Service.Version, cfg.Service.Env).Set(1)

	// 7. Run the service with graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() {
		errCh <- serviceFn(ctx, cfg, producer, logger, healthSrv)
	}()

	select {
	case sig := <-sigCh:
		logger.Info("recieved shutdown signal", zap.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil {
			logger.Error("service exited with error", zap.Error(err))
		}
	}

	// 8. Graceful shutdown with 30s deadline.
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	healthSrv.Shutdown(shutdownCtx)
	logger.Info("service shutdown greacefully")
}

func newLogger(cfg *config.Config) *zap.Logger {
	var logCfg zap.Config

	if cfg.IsProd() {
		logCfg = zap.NewProductionConfig()
		logCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	} else {
		logCfg = zap.NewDevelopmentConfig()
		logCfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}
	logCfg.EncoderConfig.TimeKey = "ts"
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := logCfg.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}

	return logger.Named(cfg.Service.Name)
}

func ensureTopics(cfg *config.Config, logger *zap.Logger) error {
	admin, err := kafka.NewTopicAdmin(&cfg.Kafka, logger.Named("admin"))
	if err != nil {
		return err
	}
	defer admin.Close()

	topics := []config.TopicDef{
		cfg.Kafka.Topics.Transactions,
		cfg.Kafka.Topics.FraudResults,
		cfg.Kafka.Topics.EnrichedTransactions,
		cfg.Kafka.Topics.Notifications,
		cfg.Kafka.Topics.DLQ,
	}

	return admin.EnsureTopics(topics)
}
