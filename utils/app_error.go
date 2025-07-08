package utils

import (
	"net/http"
)

type AppError struct {
	Code    int    // HTTP status code (e.g., 404, 400, 500)
	Message string // User-facing message
	err     error  // Internal-facing error for logging purposes
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.err
}

// --- Error Helper Functions ---

// NewNotFoundError creates a 404 Not Found error.
func NewNotFoundError(message string) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: message,
	}
}

// NewBadRequestError creates a 400 Bad Request error.
func NewBadRequestError(message string, originalError ...error) *AppError {
	e := &AppError{
		Code:    http.StatusBadRequest,
		Message: message,
	}
	if len(originalError) > 0 {
		e.err = originalError[0]
	}
	return e
}

// NewInternalServerError creates a 500 Internal Server Error.
func NewInternalServerError(message string, originalError error) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: message,
		err:     originalError,
	}
}

// NewForbiddenError creates a 403 Forbidden error.
func NewForbiddenError(message string) *AppError {
	return &AppError{
		Code:    http.StatusForbidden,
		Message: message,
	}
}
