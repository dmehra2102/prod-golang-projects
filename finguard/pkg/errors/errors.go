package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Error codes.
const (
	CodeInternal          = "INTERNAL_ERROR"
	CodeNotFound          = "NOT_FOUND"
	CodeBadRequest        = "BAD_REQUEST"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeForbidden         = "FORBIDDEN"
	CodeConflict          = "CONFLICT"
	CodeRateLimited       = "RATE_LIMITED"
	CodeValidation        = "VALIDATION_ERROR"
	CodeTokenExpired      = "TOKEN_EXPIRED"
	CodeTokenInvalid      = "TOKEN_INVALID"
	CodeMFARequired       = "MFA_REQUIRED"
	CodeAccountLocked     = "ACCOUNT_LOCKED"
	CodePaymentFailed     = "PAYMENT_FAILED"
	CodeBudgetExceeded    = "BUDGET_EXCEEDED"
	CodeBankSyncFailed    = "BANK_SYNC_FAILED"
	CodeExternalService   = "EXTERNAL_SERVICE_ERROR"
	CodeInsufficientFunds = "INSUFFICIENT_FUNDS"
)

// AppError represents a structured application error.
type AppError struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Details    string            `json:"details,omitempty"`
	HTTPStatus int               `json:"-"`
	Fields     map[string]string `json:"fields,omitempty"`
	Err        error             `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Standard error constructors.

func Internal(msg string, err error) *AppError {
	return &AppError{Code: CodeInternal, Message: msg, HTTPStatus: http.StatusInternalServerError, Err: err}
}

func NotFound(resource, identifier string) *AppError {
	return &AppError{
		Code: CodeNotFound, Message: fmt.Sprintf("%s not found: %s", resource, identifier),
		HTTPStatus: http.StatusNotFound,
	}
}

func BadRequest(msg string) *AppError {
	return &AppError{Code: CodeBadRequest, Message: msg, HTTPStatus: http.StatusBadRequest}
}

func Unauthorized(msg string) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: msg, HTTPStatus: http.StatusUnauthorized}
}

func Forbidden(msg string) *AppError {
	return &AppError{Code: CodeForbidden, Message: msg, HTTPStatus: http.StatusForbidden}
}

func Conflict(msg string) *AppError {
	return &AppError{Code: CodeConflict, Message: msg, HTTPStatus: http.StatusConflict}
}

func RateLimited() *AppError {
	return &AppError{Code: CodeRateLimited, Message: "rate limit exceeded", HTTPStatus: http.StatusTooManyRequests}
}

func Validation(msg string, fields map[string]string) *AppError {
	return &AppError{Code: CodeValidation, Message: msg, HTTPStatus: http.StatusBadRequest, Fields: fields}
}

func TokenExpired() *AppError {
	return &AppError{Code: CodeTokenExpired, Message: "token has expired", HTTPStatus: http.StatusUnauthorized}
}

func MFARequired() *AppError {
	return &AppError{Code: CodeMFARequired, Message: "multi-factor authentication required", HTTPStatus: http.StatusForbidden}
}

func AccountLocked(duration string) *AppError {
	return &AppError{
		Code: CodeAccountLocked, Message: fmt.Sprintf("account locked for %s due to too many failed attempts", duration),
		HTTPStatus: http.StatusForbidden,
	}
}

func PaymentFailed(msg string, err error) *AppError {
	return &AppError{Code: CodePaymentFailed, Message: msg, HTTPStatus: http.StatusBadGateway, Err: err}
}

func BudgetExceeded(budgetName string, limit, spent float64) *AppError {
	return &AppError{
		Code:       CodeBudgetExceeded,
		Message:    fmt.Sprintf("budget '%s' exceeded: limit %.2f, spent %.2f", budgetName, limit, spent),
		HTTPStatus: http.StatusForbidden,
	}
}

func ExternalService(service string, err error) *AppError {
	return &AppError{
		Code: CodeExternalService, Message: fmt.Sprintf("external service error: %s", service),
		HTTPStatus: http.StatusBadGateway, Err: err,
	}
}

// As is a convenience wrapper around errors.As.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Is is a convenience wrapper around errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// Wrap wraps an error with additional context.
func Wrap(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
