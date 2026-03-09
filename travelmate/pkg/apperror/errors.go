package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
)

type Code string

const (
	CodeInternal      Code = "INTERNAL"
	CodeNotFound      Code = "NOT_FOUND"
	CodeAlreadyExists Code = "ALREADY_EXISTS"
	CodeInvalidInput  Code = "INVALID_INPUT"
	CodeUnauthorized  Code = "UNAUTHORIZED"
	CodeForbidden     Code = "FORBIDDEN"
	CodeConflict      Code = "CONFLICT"
	CodeRateLimit     Code = "RATE_LIMITED"
	CodeUnavailable   Code = "UNAVAILABLE"
	CodeTimeout       Code = "TIMEOUT"
	CodeUnprocessable Code = "UNPROCESSABLE"
)

type AppError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
	File    string `json:"-"`
	Line    int    `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists, CodeConflict:
		return http.StatusConflict
	case CodeInvalidInput:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimit:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeUnprocessable:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func (e *AppError) Location() string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d", e.File, e.Line)
	}
	return ""
}

func newError(code Code, msg string, err error) *AppError {
	_, file, line, _ := runtime.Caller(2)
	return &AppError{Code: code, Message: msg, Err: err, File: file, Line: line}
}

// ---------- constructors ----------

func Internal(msg string, err error) *AppError { return newError(CodeInternal, msg, err) }
func NotFound(msg string) *AppError            { return newError(CodeNotFound, msg, nil) }
func NotFoundf(format string, a ...any) *AppError {
	return newError(CodeNotFound, fmt.Sprintf(format, a...), nil)
}
func AlreadyExists(msg string) *AppError { return newError(CodeAlreadyExists, msg, nil) }
func InvalidInput(msg string) *AppError  { return newError(CodeInvalidInput, msg, nil) }
func InvalidInputf(format string, a ...any) *AppError {
	return newError(CodeInvalidInput, fmt.Sprintf(format, a...), nil)
}
func Unauthorized(msg string) *AppError           { return newError(CodeUnauthorized, msg, nil) }
func Forbidden(msg string) *AppError              { return newError(CodeForbidden, msg, nil) }
func Conflict(msg string) *AppError               { return newError(CodeConflict, msg, nil) }
func RateLimit(msg string) *AppError              { return newError(CodeRateLimit, msg, nil) }
func Unavailable(msg string, err error) *AppError { return newError(CodeUnavailable, msg, err) }
func Timeout(msg string, err error) *AppError     { return newError(CodeTimeout, msg, err) }

func Wrap(code Code, msg string, err error) *AppError { return newError(code, msg, err) }

// ---------- introspection helpers ----------

// IsAppError extracts *AppError from the chain, if any.
func IsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// IsCode returns true if the error (or any wrapped error) has the given code.
func IsCode(err error, code Code) bool {
	ae, ok := IsAppError(err)
	return ok && ae.Code == code
}

func IsNotFound(err error) bool     { return IsCode(err, CodeNotFound) }
func IsConflict(err error) bool     { return IsCode(err, CodeConflict) }
func IsUnauthorized(err error) bool { return IsCode(err, CodeUnauthorized) }
func IsInvalidInput(err error) bool { return IsCode(err, CodeInvalidInput) }
