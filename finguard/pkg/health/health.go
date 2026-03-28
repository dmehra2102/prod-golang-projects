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
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type Result struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency"`
}

type Response struct {
	Status    Status   `json:"status"`
	Timestamp string   `json:"timestamp"`
	Version   string   `json:"version"`
	Service   string   `json:"service"`
	Results   []Result `json:"checks"`
}

type Handler struct {
	mu       sync.RWMutex
	checkers []Checker
	service  string
	version  string
}

func NewHandler(service, version string) *Handler {
	return &Handler{
		service: service,
		version: version,
	}
}

func (h *Handler) Register(checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers = append(h.checkers, checker)
}

func (h *Handler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "alive",
			"service": h.service,
		})
	}
}

func (h *Handler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		response := h.check(ctx)

		w.Header().Set("Context-Type", "application/json")
		if response.Status == StatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(response)

	}
}

func (h *Handler) check(ctx context.Context) Response {
	h.mu.RLock()
	defer h.mu.RUnlock()

	response := Response{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   h.version,
		Service:   h.service,
		Results:   make([]Result, 0, len(h.checkers)),
	}

	for _, checker := range h.checkers {
		start := time.Now()
		err := checker.Check(ctx)
		latency := time.Since(start)

		result := Result{
			Name:    checker.Name(),
			Status:  StatusHealthy,
			Latency: latency.String(),
		}

		if err != nil {
			result.Status = StatusUnhealthy
			result.Error = err.Error()
			response.Status = StatusUnhealthy
		}

		response.Results = append(response.Results, result)
	}

	return response
}

// DatabaseChecker checks database connectivity.
type DatabaseChecker struct {
	name    string
	checkFn func(ctx context.Context) error
}

func NewDatabaseChecker(name string, checkFn func(ctx context.Context) error) *DatabaseChecker {
	return &DatabaseChecker{name: name, checkFn: checkFn}
}

func (d *DatabaseChecker) Name() string                    { return d.name }
func (d *DatabaseChecker) Check(ctx context.Context) error { return d.checkFn(ctx) }

// RedisChecker checks Redis connectivity.
type RedisChecker struct {
	name    string
	checkFn func(ctx context.Context) error
}

func NewRedisChecker(name string, checkFn func(ctx context.Context) error) *RedisChecker {
	return &RedisChecker{name: name, checkFn: checkFn}
}

func (r *RedisChecker) Name() string                    { return r.name }
func (r *RedisChecker) Check(ctx context.Context) error { return r.checkFn(ctx) }
