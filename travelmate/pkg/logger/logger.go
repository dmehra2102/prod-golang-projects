package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type ctxKey string

const (
	requestIDKey   ctxKey = "request_id"
	correlationKey ctxKey = "correlation_key"
	userIDKey      ctxKey = "user_id"
)

type Logger struct {
	zl zerolog.Logger
}

func New(level, env string) *Logger {
	var w io.Writer
	if env != "production" {
		w = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	zl := zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Str("service", "travelmate").
		Logger()

	return &Logger{zl: zl}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	zl := l.zl.With().Logger()

	if rid, ok := ctx.Value(requestIDKey).(string); ok && rid != "" {
		zl = zl.With().Str("request_id", rid).Logger()
	}
	if cid, ok := ctx.Value(correlationKey).(string); ok && cid != "" {
		zl = zl.With().Str("correlation_id", cid).Logger()
	}
	if uid, ok := ctx.Value(userIDKey).(string); ok && uid != "" {
		zl = zl.With().Str("user_id", uid).Logger()
	}

	return &Logger{zl: zl}
}

func (l *Logger) With() zerolog.Context { return l.zl.With() }

func (l *Logger) Info() *zerolog.Event  { return l.zl.Info() }
func (l *Logger) Debug() *zerolog.Event { return l.zl.Debug() }
func (l *Logger) Warn() *zerolog.Event  { return l.zl.Warn() }
func (l *Logger) Error() *zerolog.Event { return l.zl.Error() }
func (l *Logger) Fatal() *zerolog.Event { return l.zl.Fatal() }

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey, id)
}

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func RequestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func UserIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}
