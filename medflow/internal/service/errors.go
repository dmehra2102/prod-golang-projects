package service

import (
	"errors"
	"strings"
)

var ErrForbidden = errors.New("forbidden: insufficient permissions")

type ValidationError struct {
	Fields []string
}

func (e *ValidationError) Error() string {
	return "validation failed: " + strings.Join(e.Fields, "; ")
}

type AuditEntry struct {
	UserID       interface{} // uuid.UUID
	UserRole     string
	Action       string
	ResourceType string
	ResourceID   string
	IPAddress    string
	RequestID    string
	StatusCode   int
	Changes      string
}
