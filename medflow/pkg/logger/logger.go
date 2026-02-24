package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/config"
)

func New(cfg config.LogConfig) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}

	var zapCfg zap.Config
	if cfg.Format == "json" {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.OutputPaths = []string{cfg.OutputPath}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	// Disable the caller field in production to reduce log volume
	if cfg.Format == "json" {
		zapCfg.DisableCaller = false
	}

	logger, err := zapCfg.Build(
		zap.WithCaller(true),
		// Stack traces for errors and above
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	return logger, nil
}
