// Package domain contains the pure business rules of the expense system:
// roles and permissions, the expense status state machine, and validation.
// It has no dependency on storage or transport so it can be unit tested in isolation.
package domain

import (
	"errors"
	"strings"
)

// Sentinel errors used across layers. The HTTP layer maps them to status codes.
var (
	ErrUnauthenticated   = errors.New("authentication required")
	ErrForbidden         = errors.New("forbidden")
	ErrNotFound          = errors.New("not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrConflict          = errors.New("conflict")
	ErrBadRequest        = errors.New("bad request")
)

// FieldError describes a validation failure on a single field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError aggregates one or more field errors.
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

func (e *ValidationError) Error() string {
	msgs := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		msgs = append(msgs, fe.Field+": "+fe.Message)
	}
	return "validation failed: " + strings.Join(msgs, "; ")
}

// Add appends a field error.
func (e *ValidationError) Add(field, message string) {
	e.Errors = append(e.Errors, FieldError{Field: field, Message: message})
}

// OrNil returns nil when no errors were collected, so callers can `return v.OrNil()`.
func (e *ValidationError) OrNil() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}
