package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Health server exposes /healthz (liveness) and /readyz (readiness) endpoints.

type Check func(ctx context.Context) error

type Server struct {
	httpServer  *http.Server
	logger      *zap.Logger
	mu          sync.RWMutex
	readyChecks map[string]Check
	liveChecks  map[string]Check
	isReady     bool
}

func NewServer(port int, logger *zap.Logger) *Server {
	s := &Server{
		logger:      logger,
		readyChecks: make(map[string]Check),
		liveChecks:  make(map[string]Check),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleLiveness)
	mux.HandleFunc("/readyz", s.handleReadiness)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%d", port),
		Handler: mux,
	}

	return s
}

func (s *Server) RegisterLivenessCheck(name string, check Check) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.liveChecks[name] = check
}

func (s *Server) RegisterReadinessCheck(name string, check Check) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readyChecks[name] = check
}

func (s *Server) SetReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isReady = ready
}

func (s *Server) Start() error {
	s.logger.Info("health server starting", zap.String("addr", s.httpServer.Addr))
	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result := s.runChecks(ctx, s.liveChecks)
	if !result.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isReady {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result := s.runChecks(ctx, s.readyChecks)
	if !result.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type HealthResult struct {
	Healthy bool              `json:"healthy"`
	Checks  map[string]string `json:"checks"`
}

func (s *Server) runChecks(ctx context.Context, checks map[string]Check) HealthResult {
	result := HealthResult{
		Healthy: true,
		Checks:  make(map[string]string),
	}

	for name, check := range checks {
		if err := check(ctx); err != nil {
			result.Healthy = false
			result.Checks[name] = fmt.Sprintf("FAIL: %v", err)
		} else {
			result.Checks[name] = "OK"
		}
	}

	return result
}
