package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/appointment"
	mr "github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/medical_record"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/patient"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/domain/prescription"
	"github.com/dmehra2102/prod-golang-projects/medflow/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type APIResponse[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

type ValidationErrorResponse struct {
	Error  string   `json:"error"`
	Fields []string `json:"fields"`
}

func respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResponse[any]{Data: data})
}

func respondCreated(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, APIResponse[any]{Data: data})
}

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{Error: message})
}

func respondServiceError(c *gin.Context, err error) {
	var validErr *service.ValidationError
	if errors.As(err, &validErr) {
		c.JSON(http.StatusBadRequest, ValidationErrorResponse{
			Error:  "validation failed",
			Fields: validErr.Fields,
		})
		return
	}

	switch {
	case errors.Is(err, patient.ErrPatientNotFound),
		errors.Is(err, appointment.ErrAppointmentNotFound),
		errors.Is(err, mr.ErrRecordNotFound),
		errors.Is(err, prescription.ErrPrescriptionNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})

	case errors.Is(err, patient.ErrPatientAlreadyExists),
		errors.Is(err, appointment.ErrAppointmentConflict):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})

	case errors.Is(err, appointment.ErrScheduledInPast),
		errors.Is(err, appointment.ErrInvalidDuration),
		errors.Is(err, appointment.ErrInvalidStatusTransition),
		errors.Is(err, appointment.ErrInvalidAppointmentType),
		errors.Is(err, patient.ErrPatientDeceased),
		errors.Is(err, patient.ErrInvalidGender),
		errors.Is(err, prescription.ErrNotRefillable),
		errors.Is(err, prescription.ErrInvalidDEASchedule),
		errors.Is(err, mr.ErrInvalidRecordType):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})

	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "access denied"})

	case errors.Is(err, service.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid credentials"})

	case errors.Is(err, service.ErrAccountLocked):
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Error: "account temporarily locked",
			Code:  "ACCOUNT_LOCKED",
		})

	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})

	}
}

func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return false
	}

	return true
}

func parseUUID(c *gin.Context, param string) (uuid.UUID, bool) {
	raw := c.Param(param)
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid " + param + ": must be a valid UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func parseQueryInt(c *gin.Context, key string, defaultVal int) int {
	if raw := c.Query(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			return v
		}
	}
	return defaultVal
}
