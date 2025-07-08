package base

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ===================================================================
// CUSTOM ERROR TYPES
// ===================================================================

// RepositoryError represents base repository error
type RepositoryError struct {
	Operation string
	Table     string
	Message   string
	Cause     error
}

func (e *RepositoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("failed to %s %s: %s (caused by: %v)", e.Operation, e.Table, e.Message, e.Cause)
	}
	return fmt.Sprintf("failed to %s %s: %s", e.Operation, e.Table, e.Message)
}

func (e *RepositoryError) Unwrap() error {
	return e.Cause
}

// EntityNotFoundError represents entity not found error
type EntityNotFoundError struct {
	Table      string
	Identifier string
}

func (e *EntityNotFoundError) Error() string {
	return fmt.Sprintf("%s with %s not found", e.Table, e.Identifier)
}

// DuplicateEntityError represents duplicate entity error
type DuplicateEntityError struct {
	Table string
	Field string
	Value string
}

func (e *DuplicateEntityError) Error() string {
	return fmt.Sprintf("%s with %s '%s' already exists", e.Table, e.Field, e.Value)
}

// ValidationError represents validation error
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field %s (value: %s): %s", e.Field, e.Value, e.Message)
}

// TransactionError represents transaction-related error
type TransactionError struct {
	Operation string
	Message   string
	Cause     error
}

func (e *TransactionError) Error() string {
	return fmt.Sprintf("transaction failed during %s: %s (caused by: %v)", e.Operation, e.Message, e.Cause)
}

func (e *TransactionError) Unwrap() error {
	return e.Cause
}

// ===================================================================
// ERROR CONSTRUCTORS
// ===================================================================

// NewRepositoryError creates a new repository error
func NewRepositoryError(operation, table, message string, cause error) *RepositoryError {
	return &RepositoryError{
		Operation: operation,
		Table:     table,
		Message:   message,
		Cause:     cause,
	}
}

// NewEntityNotFoundError creates a new entity not found error
func NewEntityNotFoundError(table, identifier string) *EntityNotFoundError {
	return &EntityNotFoundError{
		Table:      table,
		Identifier: identifier,
	}
}

// NewDuplicateEntityError creates a new duplicate entity error
func NewDuplicateEntityError(table, field, value string) *DuplicateEntityError {
	return &DuplicateEntityError{
		Table: table,
		Field: field,
		Value: value,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(field, value, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewTransactionError creates a new transaction error
func NewTransactionError(operation, message string, cause error) *TransactionError {
	return &TransactionError{
		Operation: operation,
		Message:   message,
		Cause:     cause,
	}
}

// ===================================================================
// ERROR HANDLING HELPERS
// ===================================================================

// HandleDBError handles database errors with consistent error wrapping
func HandleDBError(operation, table, identifier string, err error) error {
	if err == nil {
		return nil
	}

	// Handle GORM specific errors
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewEntityNotFoundError(table, identifier)
	}

	// Handle other database errors
	return NewRepositoryError(operation, table, "database operation failed", err)
}

// WrapDBError wraps database error with operation context
func WrapDBError(operation, table string, err error) error {
	if err == nil {
		return nil
	}

	return NewRepositoryError(operation, table, "database operation failed", err)
}

// IsEntityNotFound checks if error is an entity not found error
func IsEntityNotFound(err error) bool {
	var entityNotFoundError *EntityNotFoundError
	return errors.As(err, &entityNotFoundError)
}

// IsDuplicateEntity checks if error is a duplicate entity error
func IsDuplicateEntity(err error) bool {
	var duplicateEntityError *DuplicateEntityError
	return errors.As(err, &duplicateEntityError)
}

// IsValidationError checks if error is a validation error
func IsValidationError(err error) bool {
	var validationError *ValidationError
	return errors.As(err, &validationError)
}

// IsRepositoryError checks if error is a repository error
func IsRepositoryError(err error) bool {
	var repositoryError *RepositoryError
	return errors.As(err, &repositoryError)
}

// IsTransactionError checks if error is a transaction error
func IsTransactionError(err error) bool {
	var transactionError *TransactionError
	return errors.As(err, &transactionError)
}

// ===================================================================
// ERROR MESSAGE HELPERS
// ===================================================================

// GetErrorMessage extracts user-friendly error message
func GetErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	// Check for custom error types
	switch e := err.(type) {
	case *EntityNotFoundError:
		return e.Error()
	case *DuplicateEntityError:
		return e.Error()
	case *ValidationError:
		return e.Error()
	case *RepositoryError:
		return fmt.Sprintf("Database operation failed: %s", e.Message)
	case *TransactionError:
		return fmt.Sprintf("Transaction failed: %s", e.Message)
	default:
		return "An unexpected error occurred"
	}
}

// GetErrorCode returns error code for API responses
func GetErrorCode(err error) string {
	if err == nil {
		return ""
	}

	switch err.(type) {
	case *EntityNotFoundError:
		return "ENTITY_NOT_FOUND"
	case *DuplicateEntityError:
		return "DUPLICATE_ENTITY"
	case *ValidationError:
		return "VALIDATION_ERROR"
	case *RepositoryError:
		return "REPOSITORY_ERROR"
	case *TransactionError:
		return "TRANSACTION_ERROR"
	default:
		return "UNKNOWN_ERROR"
	}
}

// ===================================================================
// RECOVERY HELPERS
// ===================================================================

// SafeExecute executes a function and recovers from panics
func SafeExecute(operation, table string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = NewRepositoryError(operation, table, fmt.Sprintf("panic recovered: %v", r), nil)
		}
	}()

	return fn()
}

// RetryOnError retries operation on specific errors
func RetryOnError(maxRetries int, operation func() error, shouldRetry func(error) bool) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on the last attempt or if error is not retryable
		if i == maxRetries || !shouldRetry(err) {
			break
		}
	}

	return lastErr
}

// IsRetryableError determines if error is retryable
func IsRetryableError(err error) bool {
	// Only retry on non-business logic errors
	return !IsEntityNotFound(err) && !IsDuplicateEntity(err) && !IsValidationError(err)
}
