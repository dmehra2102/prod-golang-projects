package service

import (
	"context"
	"crypto/subtle"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/dmehra2102/prod-golang-projects/finguard/internal/auth/model"
	"github.com/dmehra2102/prod-golang-projects/finguard/internal/auth/repository"
	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/config"
	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/encryption"
	appErrors "github.com/dmehra2102/prod-golang-projects/finguard/pkg/errors"
	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/kafka"
	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/logger"
	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo      *repository.AuthRepository
	cfg       config.AuthConfig
	encryptor *encryption.Encryptor
	producer  *kafka.Producer
}

func NewAuthService(
	repo *repository.AuthRepository,
	cfg config.AuthConfig,
	encryptor *encryption.Encryptor,
	producer *kafka.Producer,
) *AuthService {
	return &AuthService{
		repo:      repo,
		cfg:       cfg,
		encryptor: encryptor,
		producer:  producer,
	}
}

func (s *AuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.User, string, error) {
	if err := validatePasswordStrength(req.Password, s.cfg.PasswordMinLength); err != nil {
		return nil, "", err
	}

	emailHash := encryption.HashSensitiveData(strings.ToLower(req.Email))
	existing, err := s.repo.GetUserByEmail(ctx, emailHash)
	if err != nil {
		return nil, "", appErrors.Internal("failed to check existing user", err)
	}
	if existing != nil {
		return nil, "", appErrors.Conflict("an account with this email already exists")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.cfg.BCryptCost)
	if err != nil {
		return nil, "", appErrors.Internal("failed to hash password", err)
	}

	// Encrypt email at rest
	encryptedEmail, err := s.encryptor.Encrypt([]byte(strings.ToLower(req.Email)))
	if err != nil {
		return nil, "", appErrors.Internal("failed to encrypt email", err)
	}

	now := time.Now().UTC()
	user := &model.User{
		ID:            uuid.New().String(),
		Email:         encryptedEmail,
		EmailHash:     emailHash,
		PasswordHash:  string(passwordHash),
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		PhoneNumber:   req.PhoneNumber,
		CountryCode:   req.CountryCode,
		EmailVerified: false,
		MFAEnabled:    false,
		Roles:         []string{"user"},
		Status:        model.UserStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, "", appErrors.Internal("failed to create user", err)
	}

	// Generate email verification token
	verificationToken, err := s.createVerificationToken(ctx, user.ID, model.TokenTypeEmailVerification, 24*time.Hour)
	if err != nil {
		logger.Error(ctx, "failed to create verification token", zap.Error(err))
	}

	// Publish user registered event
	event, _ := kafka.NewEvent(kafka.TopicUserRegistered, "auth-service", user.ID, map[string]string{
		"email":      req.Email,
		"first_name": req.FirstName,
		"last_name":  req.LastName,
		"token":      verificationToken,
	})

	if err := s.producer.Publish(ctx, kafka.TopicUserRegistered, event); err != nil {
		logger.Error(ctx, "failed to publish user registered event", zap.Error(err))
	}

	notifEvent, _ := kafka.NewEvent(kafka.TopicNotificationSend, "auth-service", user.ID, map[string]any{
		"type":     "email",
		"template": "email_verification",
		"to":       req.Email,
		"data": map[string]string{
			"first_name":         req.FirstName,
			"verification_token": verificationToken,
		},
	})

	if err := s.producer.Publish(ctx, kafka.TopicNotificationSend, notifEvent); err != nil {
		logger.Error(ctx, "failed to publish notification event", zap.Error(err))
	}

	return user, verificationToken, nil
}

func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest, ipAddress, userAgent string) (*model.TokenPair, *model.User,
	bool, string, error) {
	emailHash := encryption.HashSensitiveData(strings.ToLower(req.Email))

	user, err := s.repo.GetUserByEmail(ctx, emailHash)
	if err != nil {
		return nil, nil, false, "", appErrors.Internal("failed to find user", err)
	}
	if user == nil {
		return nil, nil, false, "", appErrors.Unauthorized("invalid email or password")
	}

	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remaining := time.Until(*user.LockedUntil).Round(time.Minute)
		return nil, nil, false, "", appErrors.AccountLocked(remaining.String())
	}

	if user.Status == model.UserStatusSuspended {
		return nil, nil, false, "", appErrors.Forbidden("account has been suspended")
	}

	// verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		// Increment failed attempts
		attempts, _ := s.repo.IncrementFailedAttempts(ctx, user.ID)
		if attempts >= s.cfg.MaxLoginAttempts {
			lockUntil := time.Now().Add(s.cfg.LockoutDuration)
			_ = s.repo.LockAccount(ctx, user.ID, lockUntil)

			s.auditLog(ctx, user.ID, "login_failed_locked", ipAddress, userAgent, "Account locked due to too many failed attempts", false)
			return nil, nil, false, "", appErrors.AccountLocked(s.cfg.LockoutDuration.String())
		}

		s.auditLog(ctx, user.ID, "login_failed", ipAddress, userAgent, "Invalid password", false)
		return nil, nil, false, "", appErrors.Unauthorized("invalid email or password")
	}

	// Check email verification
	if !user.EmailVerified {
		return nil, nil, false, "", appErrors.Forbidden("please verify your email address before logging in")
	}

	// Check if MFA is required
	if user.MFAEnabled {
		mfaToken, err := s.generateMFAToken(user.ID)
		if err != nil {
			return nil, nil, false, "", appErrors.Internal("failed to generate MFA token", err)
		}
		return nil, user, true, mfaToken, nil
	}

	// Generate tokens
	tokenPair, sessionID, err := s.generateTokenPair(ctx, user, req.DeviceID, ipAddress, userAgent)
	if err != nil {
		return nil, nil, false, "", err
	}

	// Reset failed attempts
	_ = s.repo.ResetFailedAttempts(ctx, user.ID)
	s.auditLog(ctx, user.ID, "login_success", ipAddress, userAgent, fmt.Sprintf("Session: %s", sessionID), true)

	return tokenPair, user, false, "", nil
}

func (s *AuthService) VerifyMFA(ctx context.Context, mfaToken, code, ipAddress, userAgent, deviceID string) (*model.TokenPair, error) {
	claims, err := s.parseMFAToken(mfaToken)
	if err != nil {
		return nil, appErrors.Unauthorized("invalid or expired MFA token")
	}

	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil || user == nil {
		return nil, appErrors.Unauthorized("user not found")
	}

	// Decrypt MFA secret
	secretBytes, err := s.encryptor.Decrypt(user.MFASecret)
	if err != nil {
		return nil, appErrors.Internal("failed to decrypt MFA secret", err)
	}

	valid := totp.Validate(code, string(secretBytes))
	if !valid {
		valid = s.verifyRecoveryCode(ctx, user, code)
		if !valid {
			s.auditLog(ctx, user.ID, "mfa_failed", ipAddress, userAgent, "Invalid MFA code", false)
			return nil, appErrors.Unauthorized("invalid MFA code")
		}
	}

	tokenPair, sessionID, err := s.generateTokenPair(ctx, user, deviceID, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	_ = s.repo.ResetFailedAttempts(ctx, user.ID)
	s.auditLog(ctx, user.ID, "mfa_success", ipAddress, userAgent, fmt.Sprintf("Session: %s", sessionID), true)

	return tokenPair, nil
}

// VerifyEmail verifies a user's email address.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	tokenHash := encryption.HashSensitiveData(token)
	vt, err := s.repo.GetVerificationToken(ctx, tokenHash, model.TokenTypeEmailVerification)
	if err != nil {
		return appErrors.Internal("failed to verify token", err)
	}
	if vt == nil {
		return appErrors.BadRequest("invalid or expired verification token")
	}

	if err := s.repo.UpdateEmailVerified(ctx, vt.UserID); err != nil {
		return appErrors.Internal("failed to verify email", err)
	}

	_ = s.repo.MarkTokenUsed(ctx, vt.ID)

	// Publish event
	event, _ := kafka.NewEvent(kafka.TopicUserVerified, "auth-service", vt.UserID, nil)
	_ = s.producer.Publish(ctx, kafka.TopicUserVerified, event)

	return nil
}

func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	tokenHash := encryption.HashSensitiveData(refreshToken)
	session, err := s.repo.GetSessionByRefreshToken(ctx, tokenHash)
	if err != nil || session == nil {
		return nil, appErrors.Unauthorized("invalid refresh token")
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil || user == nil {
		return nil, appErrors.Unauthorized("user not found")
	}

	// Delete old session
	_ = s.repo.DeleteSession(ctx, session.ID)

	// Create new session with rotated refresh token
	tokenPair, _, err := s.generateTokenPair(ctx, user, session.DeviceID, session.IPAddress, session.UserAgent)
	return tokenPair, err
}

// Logout invalidates a session.
func (s *AuthService) Logout(ctx context.Context, sessionID, userID string) error {
	if err := s.repo.DeleteSession(ctx, sessionID); err != nil {
		return appErrors.Internal("failed to delete session", err)
	}
	s.auditLog(ctx, userID, "logout", "", "", fmt.Sprintf("Session: %s", sessionID), true)
	return nil
}

func (s *AuthService) generateTokenPair(ctx context.Context, user *model.User, deviceID, ipAddress, userAgent string) (*model.TokenPair, string, error) {
	sessionID := uuid.New().String()
	now := time.Now().UTC()

	// Generate access token
	accessClaims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.EmailHash, // Use hash, not plaintext
		"roles": user.Roles,
		"sid":   sessionID,
		"iat":   now.Unix(),
		"exp":   now.Add(s.cfg.JWTExpiry).Unix(),
		"iss":   "finguard",
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, "", appErrors.Internal("failed to generate access token", err)
	}

	// Generate refresh token
	refreshTokenRaw, err := encryption.GenerateSecureToken(48)
	if err != nil {
		return nil, "", appErrors.Internal("failed to generate refresh token", err)
	}
	refreshTokenHash := encryption.HashSensitiveData(refreshTokenRaw)

	// Store session
	session := &model.Session{
		ID:           sessionID,
		UserID:       user.ID,
		RefreshToken: refreshTokenHash,
		DeviceID:     deviceID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		ExpiresAt:    now.Add(s.cfg.RefreshTokenExpiry),
		LastActiveAt: now,
		CreatedAt:    now,
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, "", appErrors.Internal("failed to create session", err)
	}

	return &model.TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenRaw,
		ExpiresIn:    int64(s.cfg.JWTExpiry.Seconds()),
		TokenType:    "Bearer",
	}, sessionID, nil
}

func (s *AuthService) generateMFAToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"type": "mfa",
		"exp":  time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) parseMFAToken(tokenString string) (*middleware.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid MFA token")
	}

	claims := token.Claims.(jwt.MapClaims)
	if claims["type"] != "mfa" {
		return nil, fmt.Errorf("not an MFA token")
	}

	return &middleware.TokenClaims{UserID: claims["sub"].(string)}, nil
}

func (s *AuthService) createVerificationToken(ctx context.Context, userID string, tokenType model.VerificationTokenType, ttl time.Duration) (string, error) {
	rawToken, err := encryption.GenerateSecureToken(32)
	if err != nil {
		return "", err
	}

	tokenHash := encryption.HashSensitiveData(rawToken)
	now := time.Now().UTC()

	v1 := &model.VerificationToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		TokenHash: tokenHash,
		Type:      tokenType,
		Used:      false,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}

	if err := s.repo.CreateVerificationToken(ctx, v1); err != nil {
		return "", err
	}

	return tokenHash, nil
}

func (s *AuthService) verifyRecoveryCode(ctx context.Context, user *model.User, code string) bool {
	for i, hashedCode := range user.RecoveryCodes {
		if err := bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(code)); err == nil {
			// Remove used recovery code
			user.RecoveryCodes = append(user.RecoveryCodes[:i], user.RecoveryCodes[i+1:]...)
			_ = s.repo.UpdateUserMFA(ctx, user.ID, true, user.MFASecret, user.RecoveryCodes)
			return true
		}
	}
	return false
}

func (s *AuthService) auditLog(ctx context.Context, userID, action, ipAddress, userAgent, details string, success bool) {
	log := &model.AuditLog{
		ID:        uuid.New().String(),
		UserID:    userID,
		Action:    action,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   details,
		Success:   success,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.CreateAuditLog(ctx, log); err != nil {
		logger.Error(ctx, "failed to create audit log", zap.Error(err))
	}
}

func validatePasswordStrength(password string, minLength int) error {
	if len(password) < minLength {
		return appErrors.Validation("password too weak", map[string]string{
			"password": fmt.Sprintf("must be at leat %d characters", minLength),
		})
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	issues := make(map[string]string)
	if !hasUpper {
		issues["uppercase"] = "must contain at least one uppercase letter"
	}
	if !hasLower {
		issues["lowercase"] = "must contain at least one lowercase letter"
	}
	if !hasDigit {
		issues["digit"] = "must contain at least one digit"
	}
	if !hasSpecial {
		issues["special"] = "must contain at least one special character"
	}

	if len(issues) > 0 {
		return appErrors.Validation("password does not meet requirements", issues)
	}

	commonPasswords := []string{"password123!", "Password123!"}
	for _, common := range commonPasswords {
		if subtle.ConstantTimeCompare([]byte(password), []byte(common)) == 1 {
			return appErrors.Validation("password too common", map[string]string{
				"password": "this password is too common",
			})
		}
	}

	_ = base32.StdEncoding
	return nil
}
