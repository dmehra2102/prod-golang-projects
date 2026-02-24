package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain"
	"github.com/dmehra2102/prod-golang-projects/medflow/pkg/auth"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked      = errors.New("account is temporarily locked due to multiple failed login attempts")
	ErrAccountInactive    = errors.New("account is inactive")
)

const maxFailedAttempts = 5

const lockDuration = 15 * time.Minute

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateLoginAttempt(ctx context.Context, id uuid.UUID, success bool) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error
}

type AuthService struct {
	userRepo   UserRepository
	jwtManager *auth.JWTManager
	log        *zap.Logger
}

func NewAuthService(userRepo UserRepository, jwtManager *auth.JWTManager, log *zap.Logger) *AuthService {
	return &AuthService{userRepo: userRepo, jwtManager: jwtManager, log: log}
}

func (s *AuthService) Login(ctx context.Context, email, password string, ip string) (*domain.TokenPair, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Use bcrypt dummy hash to prevent timing-based user enumeration.
		// An attacker measuring response time should not be able to determine
		// whether the email exists in the system.
		_, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	if user.IsLocked() {
		return nil, ErrAccountLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Record failed attempt; lock if threshold exceeded
		_ = s.userRepo.UpdateLoginAttempt(ctx, user.ID, false)
		s.log.Warn("failed login attempt",
			zap.String("email", email),
			zap.String("ip", ip),
		)
		return nil, ErrInvalidCredentials
	}

	_ = s.userRepo.UpdateLoginAttempt(ctx, user.ID, true)

	claims := &domain.Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		StaffID:   user.StaffID,
		PatientID: user.PatientID,
	}

	pair, err := s.jwtManager.GenerateTokenPair(claims)
	if err != nil {
		s.log.Error("failed to generate token pair", zap.Error(err))
		return nil, fmt.Errorf("generating tokens: %w", err)
	}

	s.log.Info("user logged in",
		zap.String("user_id", user.ID.String()),
		zap.String("ip", ip),
	)

	return pair, nil
}

// RefreshToken issues a new access token given a valid refresh token.
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Re-validate user is still active
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil || !user.IsActive {
		return nil, ErrInvalidCredentials
	}

	updatedClaims := &domain.Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		StaffID:   user.StaffID,
		PatientID: user.PatientID,
	}

	return s.jwtManager.GenerateTokenPair(updatedClaims)
}

// ChangePassword updates a user's password after verifying the current one.
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	if err := validatePasswordStrength(newPassword); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return s.userRepo.UpdatePassword(ctx, userID, string(hash))
}

func validatePasswordStrength(password string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	return nil
}
