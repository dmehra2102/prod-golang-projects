package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

type contextKey struct{}

var contextKeyLogger = contextKey{}

type Logger struct {
	zerolog.Logger
}

type Config struct {
	Level       string
	Pretty      bool
	ServiceName string
	Environment string
	Version     string
}

func New(cfg Config) *Logger {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339Nano

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	var w io.Writer = os.Stdout
	if cfg.Pretty {
		w = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	base := zerolog.New(w).
		With().
		Timestamp().
		Str("service", cfg.ServiceName).
		Str("env", cfg.Environment).
		Str("version", cfg.Version).
		Logger()

	return &Logger{Logger: base}
}

func (l *Logger) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKeyLogger, l)
}

func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(contextKeyLogger).(*Logger); ok {
		return l
	}

	l := New(Config{Level: "info", ServiceName: "unknown"})

	return l
}

func (l *Logger) WithTraceID(traceID, spanID string) *Logger {
	child := l.Logger.With().
		Str("trace_id", traceID).
		Str("span_id", spanID).
		Logger()

	return &Logger{Logger: child}
}

func (l *Logger) WithRequestID(requestID string) *Logger {
	child := l.Logger.With().Str("request_id", requestID).Logger()
	return &Logger{Logger: child}
}

func (l *Logger) WithUserID(userID string) *Logger {
	child := l.Logger.With().Str("user_id", userID).Logger()
	return &Logger{Logger: child}
}

func (l *Logger) WithErr(err error) *Logger {
	child := l.Logger.With().Stack().Err(err).Logger()
	return &Logger{Logger: child}
}
