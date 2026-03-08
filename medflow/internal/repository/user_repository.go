package repository

import (
	"context"
	"errors"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) service.UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	result := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&u)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, result.Error
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	result := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&u)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, result.Error
	}
	return &u, nil
}

func (r *UserRepository) UpdateLoginAttempt(ctx context.Context, id uuid.UUID, success bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if success {
			return tx.Model(&domain.User{}).Where("id = ?", id).Updates(map[string]any{
				"failed_login_count": 0,
				"locked_until":       nil,
				"last_login_at":      time.Now(),
			}).Error
		}

		// Increment failed count; lock if threshold exceeded
		var user domain.User
		if err := tx.Where("id = ?", id).First(&user).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"failed_login_count": user.FailedLoginCount + 1,
		}
		if user.FailedLoginCount+1 >= 5 {
			lockUntil := time.Now().Add(15 * time.Minute)
			updates["locked_until"] = lockUntil
		}
		return tx.Model(&domain.User{}).Where("id = ?", id).Updates(updates).Error
	})
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Updates(map[string]any{
		"password_hash":       hash,
		"password_changed_at": time.Now(),
		"failed_login_count":  0,
	}).Error
}
