package circuitbreaker

import (
	"errors"
	"sync"
	"time"

	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/metrics"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type State int

const (
	StateClosed   State = 0
	StateHalfOpen State = 1
	StateOpen     State = 2
)

type CircuitBreaker struct {
	name             string
	mu               sync.RWMutex
	state            State
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailure      time.Time
}

type Config struct {
	Name             string
	FailureThreshold int           // failures before opening (default: 5)
	SuccessThreshold int           // successes in half-open before closing (default: 2)
	Timeout          time.Duration // time in open state before half-open (default: 30s)
}

func New(cfg Config) *CircuitBreaker {
	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold == 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	cb := &CircuitBreaker{
		name:             cfg.Name,
		state:            StateClosed,
		failureThreshold: cfg.FailureThreshold,
		successThreshold: cfg.SuccessThreshold,
		timeout:          cfg.Timeout,
	}

	metrics.CircuitBreakerState.WithLabelValues(cfg.Name).Set(0)
	return cb
}

// Execute runs fn if the circuit allows it. Returns ErrCircuitOpen if the
// circuit is open. Tracks successes/failures to manage state transitions.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()

			if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.timeout {
				cb.state = StateHalfOpen
				cb.successCount = 0
				metrics.CircuitBreakerState.WithLabelValues(cb.name).Set(float64(StateHalfOpen))
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == StateHalfOpen
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()
		cb.successCount = 0

		switch cb.state {
		case StateClosed:
			if cb.failureCount >= cb.failureThreshold {
				cb.state = StateOpen
				metrics.CircuitBreakerState.WithLabelValues(cb.name).Set(float64(StateOpen))
			}
		case StateHalfOpen:
			cb.state = StateOpen
			metrics.CircuitBreakerState.WithLabelValues(cb.name).Set(float64(StateOpen))
		}
	} else {
		cb.successCount++

		switch cb.state {
		case StateHalfOpen:
			if cb.successCount >= cb.successThreshold {
				cb.state = StateClosed
				cb.failureCount = 0
				metrics.CircuitBreakerState.WithLabelValues(cb.name).Set(float64(StateClosed))
			}
		case StateClosed:
			cb.failureCount = 0
		}
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
