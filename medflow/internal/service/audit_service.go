package service

import (
	"context"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain"
	"go.uber.org/zap"
)

type AuditRepository interface {
	Create(ctx context.Context, entry *domain.AuditLog) error
}

type AuditService struct {
	repo    AuditRepository
	log     *zap.Logger
	entries chan *domain.AuditLog
	done    chan struct{}
}

const auditBufferSize = 10_000

func NewAuditService(repo AuditRepository, log *zap.Logger) *AuditService {
	svc := &AuditService{
		repo:    repo,
		log:     log,
		entries: make(chan *domain.AuditLog, auditBufferSize),
		done:    make(chan struct{}),
	}
	go svc.worker()
	return svc
}

// LogAsync enqueues an audit entry for async persistence.
// If the buffer is full, the entry is dropped and a warning is emitted.
func (s *AuditService) LogAsync(ctx context.Context, entry AuditEntry) {
	al := &domain.AuditLog{
		UserRole:     domain.Role(entry.UserRole),
		Action:       domain.AuditAction(entry.Action),
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		IPAddress:    entry.IPAddress,
		RequestID:    entry.RequestID,
		StatusCode:   entry.StatusCode,
		Changes:      entry.Changes,
	}

	select {
	case s.entries <- al:
	default:
		s.log.Warn("audit log buffer full, dropping entry",
			zap.String("action", entry.Action),
			zap.String("resource", entry.ResourceType),
		)
	}
}

func (s *AuditService) Shutdown() {
	close(s.entries)
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		s.log.Warn("audit service shutdown timed out; some entries may be lost")
	}
}

func (s *AuditService) worker() {
	defer close(s.done)
	for entry := range s.entries {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.repo.Create(ctx, entry); err != nil {
			s.log.Error("failed to persist audit log", zap.Error(err))
		}
		cancel()
	}
}
