package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/config"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type tokenType string

const (
	accessTokenType  tokenType = "access"
	refreshTokenType tokenType = "refresh"
)

var (
	ErrTokenExpired      = errors.New("token has expired")
	ErrTokenInvalid      = errors.New("token is invalid")
	ErrTokenTypeMismatch = errors.New("wrong token type")
)

type medflowClaims struct {
	jwt.RegisteredClaims
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	StaffID   *uuid.UUID `json:"staff_id,omitempty"`
	PatientID *uuid.UUID `json:"patient_id,omitempty"`
	TokenType tokenType  `json:"token_type"`
}

type JWTManager struct {
	cfg config.JWTConfig
}

func NewJWTManager(cfg config.JWTConfig) *JWTManager {
	return &JWTManager{cfg: cfg}
}

func (m *JWTManager) GenerateTokenPair(claims *domain.Claims) (*domain.TokenPair, error) {
	accessToken, expiresAt, err := m.generateToken(claims, accessTokenType, m.cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, _, err := m.generateToken(claims, refreshTokenType, m.cfg.RefreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
	}, nil
}

func (m *JWTManager) ValidateAccessToken(tokenString string) (*domain.Claims, error) {
	return m.validateToken(tokenString, accessTokenType)
}

func (m *JWTManager) ValidateRefreshToken(tokenString string) (*domain.Claims, error) {
	return m.validateToken(tokenString, refreshTokenType)
}

func (m *JWTManager) generateToken(claims *domain.Claims, ttype tokenType, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := time.Now().Add(ttl)

	jwtClaims := medflowClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			Subject:   claims.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			// NotBefore prevents a token from being used immediately after issuance
			// (skew tolerance of 10 seconds handles clock drift in distributed systems)
			NotBefore: jwt.NewNumericDate(now.Add(-10 * time.Second)),
		},
		Email:     claims.Email,
		Role:      string(claims.Role),
		StaffID:   claims.StaffID,
		PatientID: claims.PatientID,
		TokenType: ttype,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	signed, err := token.SignedString([]byte(m.cfg.Secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func (m *JWTManager) validateToken(tokenString string, expectedType tokenType) (*domain.Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&medflowClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(m.cfg.Secret), nil
		},
		jwt.WithIssuer(m.cfg.Issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*medflowClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	if claims.TokenType != expectedType {
		return nil, ErrTokenTypeMismatch
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	return &domain.Claims{
		UserID:    userID,
		Email:     claims.Email,
		Role:      domain.Role(claims.Role),
		StaffID:   claims.StaffID,
		PatientID: claims.PatientID,
	}, nil
}
