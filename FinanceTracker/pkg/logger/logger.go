package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type ctxKey struct{}

var defaultLogger *zerolog.Logger

func init(){
	l := zerolog.New(os.Stdout).With().Timestamp().Caller().Logger()
	defaultLogger = &l
}

func Init(level string, pretty bool, serviceName string) {
	var w io.Writer = os.Stdout
	if pretty {
		w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	l := zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Str("service", serviceName).
		Logger()

	defaultLogger = &l
}

func Get() *zerolog.Logger {
	return defaultLogger
}

func WithContext(ctx context.Context, l *zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) *zerolog.Logger {
	if l,ok := ctx.Value(ctxKey{}).(*zerolog.Logger); ok {
		return l
	}
	return defaultLogger
}