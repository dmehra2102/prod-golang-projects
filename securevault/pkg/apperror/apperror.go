package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	CodeNotFound          Code = "NOT_FOUND"
	CodeAlreadyExists     Code = "ALREADY_EXISTS"
	CodeInvalidInput      Code = "INVALID_INPUT"
	CodeUnauthorized      Code = "UNAUTHORIZED"
	CodeForbidden         Code = "FORBIDDEN"
	CodeInternal          Code = "INTERNAL_ERROR"
	CodeUnavailable       Code = "SERVICE_UNAVAILABLE"
	CodeConflict          Code = "CONFLICT"
	CodeRateLimited       Code = "RATE_LIMITED"
	CodeSecretFetchFailed Code = "SECRET_FETCH_FAILED"
	CodeDatabaseError     Code = "DATABASE_ERROR"
)

// AppError is a structured application error that carries an HTTP status,
// a machine-readable code, a human-readable message, and an optional cause.
type AppError struct {
	HTTPStatus int    `json:"-"`
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	Details    any    `json:"details,omitempty"`
	cause      error
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.cause }

func New(status int, code Code, message string) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: message}
}

func Wrap(cause error, status int, code Code, message string) *AppError {
	return &AppError{HTTPStatus: status, cause: cause, Code: code, Message: message}
}

func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}

func As(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// --- Constructors for common errors ---

func NotFound(resource, id string) *AppError {
	return New(http.StatusNotFound, CodeNotFound,
		fmt.Sprintf("%s with id %q not found", resource, id))
}

func AlreadyExists(resource, field, value string) *AppError {
	return New(http.StatusConflict, CodeAlreadyExists,
		fmt.Sprintf("%s with %s=%q already exists", resource, field, value))
}

func InvalidInput(message string) *AppError {
	return New(http.StatusBadRequest, CodeInvalidInput, message)
}

func Unauthorized(message string) *AppError {
	return New(http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(message string) *AppError {
	return New(http.StatusForbidden, CodeForbidden, message)
}

func Internal(cause error) *AppError {
	return Wrap(cause, http.StatusInternalServerError, CodeInternal, "an internal error occurred")
}

func Unavailable(cause error) *AppError {
	return Wrap(cause, http.StatusServiceUnavailable, CodeUnavailable, "service temporarily unavailable")
}
