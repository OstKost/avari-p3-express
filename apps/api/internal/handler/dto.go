package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// ErrorResponse represents a standard API error response.
type ErrorResponse struct {
	Error   string            `json:"error" example:"Validation failed"`
	Details map[string]string `json:"details,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}

func respondValidationError(w http.ResponseWriter, errs validator.ValidationErrors) {
	details := make(map[string]string)
	for _, e := range errs {
		details[e.Field()] = e.Tag()
	}
	respondJSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "Validation failed",
		Details: details,
	})
}
