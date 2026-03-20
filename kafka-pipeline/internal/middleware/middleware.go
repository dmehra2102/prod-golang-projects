package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	"go.uber.org/zap"
)

type Middleware func(kafka.MessageHandler) kafka.MessageHandler

// Chain composes middlewares left-to-right
func Chain(middlewares ...Middleware) Middleware {
	return func(next kafka.MessageHandler) kafka.MessageHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Logging adds structured request/response logging.
func Logging(logger *zap.Logger) Middleware {
	return func(next kafka.MessageHandler) kafka.MessageHandler {
		return func(ctx context.Context, key, value []byte, headers map[string]string) error {
			start := time.Now()

			logger.Debug("processing message",
				zap.String("key", string(key)),
				zap.Int("value_bytes", len(value)),
			)

			err := next(ctx, key, value, headers)

			fields := []zap.Field{
				zap.String("key", string(key)),
				zap.Duration("duration", time.Since(start)),
			}

			if err != nil {
				fields = append(fields, zap.Error(err))
				logger.Error("message processing failed", fields...)
			} else {
				logger.Debug("message processed successfully", fields...)
			}

			return err
		}
	}
}

func Recovery(logger *zap.Logger) Middleware {
	return func(next kafka.MessageHandler) kafka.MessageHandler {
		return func(ctx context.Context, key, value []byte, headers map[string]string) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("PANIC recovered in message handler",
						zap.Any("panic", r),
						zap.String("key", string(key)),
					)

					err = &PanicError{Value: r}
				}
			}()

			return next(ctx, key, value, headers)
		}
	}
}

// Timeout wraps the handler with a context deadline.
// If processing exceeds the timeout, the context is cancelled.
func Timeout(d time.Duration) Middleware {
	return func(next kafka.MessageHandler) kafka.MessageHandler {
		return func(ctx context.Context, key []byte, value []byte, headers map[string]string) error {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			return next(ctx, key, value, headers)
		}
	}
}

func Dedupilcation(logger *zap.Logger, ttl time.Duration) Middleware {
	seen := newTTLCache(ttl)
	return func(next kafka.MessageHandler) kafka.MessageHandler {
		return func(ctx context.Context, key, value []byte, headers map[string]string) error {
			idempotencyKey := headers["idempotency_key"]
			if idempotencyKey == "" {
				idempotencyKey = string(key)
			}
			if seen.Has(idempotencyKey) {
				logger.Debug("depulicate message skipped", zap.String("key", idempotencyKey))
				return nil
			}

			err := next(ctx, key, value, headers)
			if err == nil {
				seen.Add(idempotencyKey)
			}
			return err
		}
	}
}

// PanicError wraps a recovered panic value.
type PanicError struct {
	Value any
}

func (e *PanicError) Error() string {
	return "panic in message handler"
}

// -----------------------------------------------------------------------
// Simple TTL cache for deduplication. In production, use Redis.
// -----------------------------------------------------------------------
type ttlCache struct {
	mu    sync.RWMutex
	items map[string]time.Time
	ttl   time.Duration
}

func newTTLCache(ttl time.Duration) *ttlCache {
	c := &ttlCache{
		items: make(map[string]time.Time),
		ttl:   ttl,
	}
	go c.cleanup()
	return c
}

func (c *ttlCache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	exp, ok := c.items[key]
	return ok && time.Now().Before(exp)
}

func (c *ttlCache) Add(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = time.Now().Add(c.ttl)
}

func (c *ttlCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, exp := range c.items {
			if now.After(exp) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
