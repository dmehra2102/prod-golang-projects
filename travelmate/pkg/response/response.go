package response

import (
	"encoding/json"
	"net/http"

	"github.com/dmehra2102/prod-golang-projects/travelmate/pkg/apperror"
)

type Envelope struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorBody `json:"error,omitempty"`
	Meta    *Meta      `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

type Meta struct {
	RequestID  string `json:"request_id,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	TotalCount int64  `json:"total_count,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "applicattion/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Success: true, Data: data})
}

func JSONWithMeta(w http.ResponseWriter, status int, data any, meta *Meta) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Success: true, Data: data, Meta: meta})
}

func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func Err(w http.ResponseWriter, err error) {
	ae, ok := apperror.IsAppError(err)
	if !ok {
		ae = apperror.Internal("an unexpected error occured", err)
	}

	status := ae.HTTPStatus()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{
		Success: false,
		Error: &ErrorBody{
			Code:    string(ae.Code),
			Message: ae.Message,
		},
	})
}

func ErrWithDetails(w http.ResponseWriter, err error, details map[string]string) {
	ae, ok := apperror.IsAppError(err)
	if !ok {
		ae = apperror.Internal("an unexpected error occurred", err)
	}

	status := ae.HTTPStatus()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{
		Success: false,
		Error: &ErrorBody{
			Code:    string(ae.Code),
			Message: ae.Message,
			Details: details,
		},
	})
}
