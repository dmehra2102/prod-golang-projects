package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Status string

const (
	StatusUp       Status = "UP"
	StatusDown     Status = "DOWN"
	StatusDegraded Status = "DEGRADED"
)

type CheckFunc func(ctx context.Context) error

type Result struct {
	Status  Status        `json:"status"`
	Latency time.Duration `json:"latency_ms"`
	Error   string        `json:"error,omitempty"`
}

// Report is the aggregate health response payload.
type Report struct {
	Status     Status            `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Components map[string]Result `json:"components"`
}

type checker struct {
	name string
	fn   CheckFunc
}

// Checker aggregates multiple health probes.
type Checker struct {
	mu       sync.RWMutex
	checkers []checker
	timeout  time.Duration
}

func New(timeout time.Duration) *Checker {
	return &Checker{timeout: timeout}
}

func (c *Checker) Register(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkers = append(c.checkers, checker{name: name, fn: fn})
}

func (c *Checker) Run(ctx context.Context) Report {
	c.mu.RLock()
	checks := make([]checker, len(c.checkers))
	copy(checks, c.checkers)
	c.mu.RUnlock()

	type indexedResult struct {
		name   string
		result Result
	}

	results := make(chan indexedResult, len(checks))
	for _, ch := range checks {
		ch := ch
		go func() {
			tctx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			start := time.Now()
			err := ch.fn(tctx)
			latency := time.Since(start)

			res := Result{Status: StatusUp, Latency: latency / time.Millisecond}
			if err != nil {
				res.Status = StatusDown
				res.Error = err.Error()
			}
			results <- indexedResult{name: ch.name, result: res}
		}()
	}

	components := make(map[string]Result, len(checks))
	overall := StatusUp
	for range checks {
		ir := <-results
		components[ir.name] = ir.result
		if ir.result.Status == StatusDown {
			overall = StatusDown
		}
	}

	return Report{
		Status:     overall,
		Timestamp:  time.Now().UTC(),
		Components: components,
	}
}

// LivenessHandler always returns 200 (the process is alive).
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"alive"}`))
	}
}

// ReadinessHandler returns 200 when all probes pass, 503 otherwise.
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := c.Run(r.Context())

		w.Header().Set("Content-Type", "application/json")
		if report.Status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(report)
	}
}
