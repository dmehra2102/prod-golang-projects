package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/finguard/internal/auth/model"
	"github.com/dmehra2102/prod-golang-projects/finguard/pkg/database"
	"github.com/jackc/pgx/v5"
)

type AuthRepository struct {
	db *database.Pool
}

func NewAuthRepository(db *database.Pool) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) CreateUser(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (id, email, email_hash, password_hash, first_name, last_name, phone_number,
			country_code, email_verified, mfa_enabled, roles, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.Exec(ctx, query,
		user.ID, user.Email, user.EmailHash, user.PasswordHash, user.FirstName, user.LastName, user.PhoneNumber,
		user.CountryCode, user.EmailVerified, user.MFAEnabled, user.Roles, user.Status, user.CreatedAt, user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	query := `
		SELECT id, email, email_hash, password_hash, first_name, last_name,
			phone_number, country_code, email_verified, mfa_enabled, mfa_secret,
			roles, status, failed_attempts, locked_until, last_login_at,
			password_changed_at, created_at, updated_at
		FROM users WHERE id = $1 AND status != 'deleted'
	`

	return r.scanUser(ctx, query, userID)
}

func (r *AuthRepository) GetUserByEmail(ctx context.Context, emailHash string) (*model.User, error) {
	query := `
		SELECT id, email, email_hash, password_hash, first_name, last_name,
			phone_number, country_code, email_verified, mfa_enabled, mfa_secret,
			roles, status, failed_attempts, locked_until, last_login_at,
			password_changed_at, created_at, updated_at
		FROM users WHERE email_hash = $1 AND status != 'deleted'
	`

	return r.scanUser(ctx, query, emailHash)
}

func (r *AuthRepository) scanUser(ctx context.Context, query string, args ...any) (*model.User, error) {
	var user model.User
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.Email, &user.EmailHash, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.PhoneNumber, &user.CountryCode,
		&user.EmailVerified, &user.MFAEnabled, &user.MFASecret,
		&user.Roles, &user.Status, &user.FailedAttempts, &user.LockedUntil,
		&user.LastLoginAt, &user.PasswordChanged, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *AuthRepository) UpdateUserMFA(ctx context.Context, userID string, enabled bool, secret string, recoveryCodes []string) error {
	query := `
		UPDATE users SET mfa_enabled = $2, mfa_secret = $3, recovery_codes = $4, updated_at = $5
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, userID, enabled, secret, recoveryCodes, time.Now().UTC())
	return err
}

func (r *AuthRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `
		UPDATE users SET password_hash = $2, password_changed_at = $3, updated_at = $4
		WHERE id = $1
	`

	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, query, userID, passwordHash, now)
	return err
}

func (r *AuthRepository) UpdateEmailVerified(ctx context.Context, userID string) error {
	query := `UPDATE users SET email_verified = true, status = 'active', updated_at = $2 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, userID, time.Now().UTC())
	return err
}

// IncrementFailedAttempts increments the failed login counter.
func (r *AuthRepository) IncrementFailedAttempts(ctx context.Context, userID string) (int, error) {
	query := `
		UPDATE users SET failed_attempts = failed_attempts + 1, updated_at = $2
		WHERE id = $1 RETURNING failed_attempts
	`
	var attempts int
	err := r.db.QueryRow(ctx, query, userID, time.Now().UTC()).Scan(&attempts)
	return attempts, err
}

func (r *AuthRepository) ResetFailedAttempts(ctx context.Context, userID string) error {
	query := `
		UPDATE users SET failed_attempts = 0, locked_until = NULL,
			last_login_at = $2, updated_at = $2
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, userID, time.Now().UTC())
	return err
}

func (r *AuthRepository) LockAccount(ctx context.Context, userID string, until time.Time) error {
	query := `UPDATE users SET locked_until = $2, updated_at = $3 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, userID, until, time.Now().UTC())
	return err
}

func (r *AuthRepository) CreateSession(ctx context.Context, session *model.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, refresh_token_hash, device_id, ip_address,
			user_agent, expires_at, last_active_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		session.ID, session.UserID, session.RefreshToken, session.DeviceID,
		session.IPAddress, session.UserAgent, session.ExpiresAt,
		session.LastActiveAt, session.CreatedAt,
	)
	return err
}

func (r *AuthRepository) GetSessionByID(ctx context.Context, sessionID string) (*model.Session, error) {
	query := `
		SELECT id, user_id, refresh_token_hash, device_id, ip_address,
			user_agent, expires_at, last_active_at, created_at
		FROM sessions WHERE id = $1 AND expires_at > NOW()
	`
	var s model.Session
	err := r.db.QueryRow(ctx, query, sessionID).Scan(
		&s.ID, &s.UserID, &s.RefreshToken, &s.DeviceID, &s.IPAddress,
		&s.UserAgent, &s.ExpiresAt, &s.LastActiveAt, &s.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *AuthRepository) GetSessionByRefreshToken(ctx context.Context, tokenHash string) (*model.Session, error) {
	query := `
		SELECT id, user_id, refresh_token_hash, device_id, ip_address,
			user_agent, expires_at, last_active_at, created_at
		FROM sessions WHERE refresh_token_hash = $1 AND expires_at > NOW()
	`
	var s model.Session
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&s.ID, &s.UserID, &s.RefreshToken, &s.DeviceID, &s.IPAddress,
		&s.UserAgent, &s.ExpiresAt, &s.LastActiveAt, &s.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// DeleteSession removes a session.
func (r *AuthRepository) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM sessions WHERE id = $1", sessionID)
	return err
}

// DeleteAllUserSessions removes all sessions for a user.
func (r *AuthRepository) DeleteAllUserSessions(ctx context.Context, userID string) (int, error) {
	tag, err := r.db.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", userID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (r *AuthRepository) ListUserSessions(ctx context.Context, userID string) ([]*model.Session, error) {
	query := `
		SELECT id, user_id, device_id, ip_address, user_agent,
			expires_at, last_active_at, created_at
		FROM sessions WHERE user_id = $1 AND expires_at > NOW()
		ORDER BY last_active_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*model.Session
	for rows.Next() {
		var s model.Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.DeviceID, &s.IPAddress,
			&s.UserAgent, &s.ExpiresAt, &s.LastActiveAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}
	return sessions, rows.Err()
}

func (r *AuthRepository) CreateVerificationToken(ctx context.Context, token *model.VerificationToken) error {
	query := `
		INSERT INTO verification_tokens (id, user_id, token_hash, type, used, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		token.ID, token.UserID, token.TokenHash, token.Type,
		token.Used, token.ExpiresAt, token.CreatedAt,
	)
	return err
}

func (r *AuthRepository) GetVerificationToken(ctx context.Context, tokenHash string, tokenType model.VerificationTokenType) (*model.VerificationToken, error) {
	query := `
		SELECT id, user_id, token_hash, type, used, expires_at, created_at
		FROM verification_tokens
		WHERE token_hash = $1 AND type = $2 AND used = false AND expires_at > NOW()
	`
	var t model.VerificationToken
	err := r.db.QueryRow(ctx, query, tokenHash, tokenType).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.Type, &t.Used, &t.ExpiresAt, &t.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *AuthRepository) MarkTokenUsed(ctx context.Context, tokenID string) error {
	_, err := r.db.Exec(ctx, "UPDATE verification_tokens SET used = true WHERE id = $1", tokenID)
	return err
}

func (r *AuthRepository) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, user_id, action, ip_address, user_agent, details, success, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID, log.UserID, log.Action, log.IPAddress,
		log.UserAgent, log.Details, log.Success, log.CreatedAt,
	)

	return err
}

// Transaction helper
func (r *AuthRepository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return r.db.Transaction(ctx, fn)
}
