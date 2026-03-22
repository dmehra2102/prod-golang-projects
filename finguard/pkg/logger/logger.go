package logger

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
	traceIDKey   contextKey = "trace_id"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

func Init(level, format, serviceName string) {
	once.Do(func() {
		var l zapcore.Level
		if err := l.UnmarshalText([]byte(level)); err != nil {
			l = zapcore.InfoLevel
		}

		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoderConfig.StacktraceKey = "stacktrace"

		var encoder zapcore.Encoder
		if format == "console" {
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		}

		core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), l)

		globalLogger = zap.New(core,
			zap.AddCaller(),
			zap.AddCallerSkip(1),
			zap.AddStacktrace(zapcore.ErrorLevel),
			zap.Fields(zap.String("service", serviceName)),
		)
	})
}

func Get() *zap.Logger {
	if globalLogger == nil {
		Init("info", "json", "finguard")
	}
	return globalLogger
}

// Sugar returns a sugared logger.
func Sugar() *zap.SugaredLogger {
	return Get().Sugar()
}

func WithContext(ctx context.Context) *zap.Logger {
	l := Get()

	if requestID, ok := ctx.Value(requestIDKey).(string); ok && requestID != "" {
		l = l.With(zap.String("request_id", requestID))
	}
	if userID, ok := ctx.Value(userIDKey).(string); ok && userID != "" {
		l = l.With(zap.String("user_id", userID))
	}
	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		l = l.With(zap.String("trace_id", traceID))
	}

	return l
}

// ContextWithRequestID adds a request ID to the context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// ContextWithUserID adds a user ID to the context.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// ContextWithTraceID adds a trace ID to the context.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Info(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Error(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Warn(msg, fields...)
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Debug(msg, fields...)
}

func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Fatal(msg, fields...)
}
