package model

import "time"

type User struct {
	ID              string     `json:"id" db:"id"`
	Email           string     `json:"email" db:"email"`
	EmailHash       string     `json:"-" db:"email_hash"`
	PasswordHash    string     `json:"-" db:"password_hash"`
	FirstName       string     `json:"first_name" db:"first_name"`
	LastName        string     `json:"last_name" db:"last_name"`
	PhoneNumber     string     `json:"phone_number,omitempty" db:"phone_number"`
	CountryCode     string     `json:"country_code" db:"country_code"`
	EmailVerified   bool       `json:"email_verified" db:"email_verified"`
	MFAEnabled      bool       `json:"mfa_enabled" db:"mfa_enabled"`
	MFASecret       string     `json:"-" db:"mfa_secret"`
	RecoveryCodes   []string   `json:"-" db:"recovery_codes"`
	Roles           []string   `json:"roles" db:"roles"`
	Status          UserStatus `json:"status" db:"status"`
	FailedAttempts  int        `json:"-" db:"failed_attempts"`
	LockedUntil     *time.Time `json:"-" db:"locked_until"`
	LastLoginAt     *time.Time `json:"last_login_at" db:"last_login_at"`
	PasswordChanged *time.Time `json:"-" db:"password_changed_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusPending   UserStatus = "pending"
)

// Session represents an active user session.
type Session struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	RefreshToken string    `json:"-" db:"refresh_token_hash"`
	DeviceID     string    `json:"device_id" db:"device_id"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	LastActiveAt time.Time `json:"last_active_at" db:"last_active_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// VerificationToken for email verification and password resets.
type VerificationToken struct {
	ID        string                `json:"id" db:"id"`
	UserID    string                `json:"user_id" db:"user_id"`
	TokenHash string                `json:"-" db:"token_hash"`
	Type      VerificationTokenType `json:"type" db:"type"`
	Used      bool                  `json:"used" db:"used"`
	ExpiresAt time.Time             `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time             `json:"created_at" db:"created_at"`
}

type VerificationTokenType string

const (
	TokenTypeEmailVerification VerificationTokenType = "email_verification"
	TokenTypePasswordReset     VerificationTokenType = "password_reset"
)

// AuditLog records security-relevant events.
type AuditLog struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Action    string    `json:"action" db:"action"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	Details   string    `json:"details" db:"details"`
	Success   bool      `json:"success" db:"success"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RegisterRequest is the API-level registration request.
type RegisterRequest struct {
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=12,max=128"`
	FirstName   string `json:"first_name" validate:"required,min=1,max=100"`
	LastName    string `json:"last_name" validate:"required,min=1,max=100"`
	PhoneNumber string `json:"phone_number" validate:"omitempty,e164"`
	CountryCode string `json:"country_code" validate:"required,len=2"`
}

// LoginRequest is the API-level login request.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	DeviceID string `json:"device_id" validate:"omitempty,max=255"`
}

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// MFASetupResponse is returned when enabling MFA.
type MFASetupResponse struct {
	Secret        string   `json:"secret"`
	QRCodeURL     string   `json:"qr_code_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// MFAVerifyRequest verifies an MFA code.
type MFAVerifyRequest struct {
	MFAToken string `json:"mfa_token" validate:"required"`
	Code     string `json:"code" validate:"required,len=6"`
}

// PasswordResetRequest initiates a password reset.
type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// PasswordResetConfirm completes a password reset.
type PasswordResetConfirm struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=12,max=128"`
}

// ChangePasswordRequest changes an existing password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=12,max=128"`
}
