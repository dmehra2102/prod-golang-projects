package circuitbreaker

import (
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
)

type Settings struct {
	Name          string
	MaxRequests   uint32
	Interval      time.Duration
	Timeout       time.Duration
	FailThreshold uint32
}

func DefaultSettings(name string) Settings {
	return Settings{
		Name:          name,
		MaxRequests:   3,
		Interval:      60 * time.Second,
		Timeout:       30 * time.Second,
		FailThreshold: 5,
	}
}

type Breaker struct {
	cb *gobreaker.CircuitBreaker[any]
}

func New(s Settings) *Breaker {
	return &Breaker{
		cb: gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
			Name:        s.Name,
			MaxRequests: s.MaxRequests,
			Interval:    s.Interval,
			Timeout:     s.Timeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= s.FailThreshold
			},
			OnStateChange: func(name string, from, to gobreaker.State) {
				fmt.Printf("[circuit-breaker] %s: %s -> %s\n", name, from, to)
			},
		}),
	}
}

// Execute wraps a function call with circuit breaker protection.
func Execute[T any](b *Breaker, fn func() (T, error)) (T, error) {
	result, err := b.cb.Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result.(T), nil
}

func (b *Breaker) State() gobreaker.State {
	return b.cb.State()
}
