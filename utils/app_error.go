package utils

import (
	"net/http"
)

// AppError defines the standard application error structure.
type AppError struct {
	Code    int    // HTTP status code
	Message string // User-facing message
	err     error  // Internal-facing error for logging
}

// Error satisfies the error interface.
func (e *AppError) Error() string {
	return e.Message
}

// Unwrap provides compatibility with errors.Is and errors.As.
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
