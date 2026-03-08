package repository

import (
	"context"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/service"
	"gorm.io/gorm"
)

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) service.AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, entry *domain.AuditLog) error {
	return r.db.WithContext(ctx).Create(entry).Error
}
