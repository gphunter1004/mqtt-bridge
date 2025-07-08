package base

import (
	"fmt"
	"net/http"

	"mqtt-bridge/repositories/base"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// ===================================================================
// STANDARD RESPONSE PATTERNS
// ===================================================================

// CreateSuccessResponse creates a standard success response
func CreateSuccessResponse(message string, data interface{}) map[string]interface{} {
	response := map[string]interface{}{
		"status":  "success",
		"message": message,
	}

	if data != nil {
		response["data"] = data
	}

	return response
}

// CreateErrorResponse creates a standard error response
func CreateErrorResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"status":  "error",
		"message": message,
	}
}

// CreateListResponse creates a standard list response with pagination
func CreateListResponse(items interface{}, count int, pagination *utils.PaginationParams) map[string]interface{} {
	response := map[string]interface{}{
		"items": items,
		"count": count,
	}

	if pagination != nil {
		response["limit"] = pagination.Limit
		response["offset"] = pagination.Offset
	}

	return response
}

// CreateDeletionResponse creates a standard deletion success response
func CreateDeletionResponse(resourceType string, identifier interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("%s %v deleted successfully", resourceType, identifier),
	}
}

// ===================================================================
// HTTP ERROR HANDLING
// ===================================================================

// HandleRepositoryError converts repository errors to appropriate HTTP errors
func HandleRepositoryError(c echo.Context, err error) error {
	if err == nil {
		return nil
	}

	// Handle specific repository error types
	if base.IsEntityNotFound(err) {
		return echo.NewHTTPError(http.StatusNotFound, base.GetErrorMessage(err))
	}

	if base.IsDuplicateEntity(err) {
		return echo.NewHTTPError(http.StatusConflict, base.GetErrorMessage(err))
	}

	if base.IsValidationError(err) {
		return echo.NewHTTPError(http.StatusBadRequest, base.GetErrorMessage(err))
	}

	// Default to internal server error
	return echo.NewHTTPError(http.StatusInternalServerError, base.GetErrorMessage(err))
}

// CreateHTTPError creates a standard HTTP error with formatted message
func CreateHTTPError(statusCode int, message string, args ...interface{}) error {
	formattedMessage := fmt.Sprintf(message, args...)
	return echo.NewHTTPError(statusCode, formattedMessage)
}

// BadRequestError creates a 400 Bad Request error
func BadRequestError(message string, args ...interface{}) error {
	return CreateHTTPError(http.StatusBadRequest, message, args...)
}

// NotFoundError creates a 404 Not Found error
func NotFoundError(message string, args ...interface{}) error {
	return CreateHTTPError(http.StatusNotFound, message, args...)
}

// ConflictError creates a 409 Conflict error
func ConflictError(message string, args ...interface{}) error {
	return CreateHTTPError(http.StatusConflict, message, args...)
}

// InternalServerError creates a 500 Internal Server Error
func InternalServerError(message string, args ...interface{}) error {
	return CreateHTTPError(http.StatusInternalServerError, message, args...)
}

// ===================================================================
// RESPONSE HELPERS
// ===================================================================

// SendSuccessJSON sends a success response with JSON data
func SendSuccessJSON(c echo.Context, statusCode int, message string, data interface{}) error {
	response := CreateSuccessResponse(message, data)
	return c.JSON(statusCode, response)
}

// SendErrorJSON sends an error response with JSON
func SendErrorJSON(c echo.Context, statusCode int, message string) error {
	response := CreateErrorResponse(message)
	return c.JSON(statusCode, response)
}

// SendCreatedJSON sends a 201 Created response
func SendCreatedJSON(c echo.Context, message string, data interface{}) error {
	return SendSuccessJSON(c, http.StatusCreated, message, data)
}

// SendOKJSON sends a 200 OK response
func SendOKJSON(c echo.Context, message string, data interface{}) error {
	return SendSuccessJSON(c, http.StatusOK, message, data)
}

// SendListJSON sends a paginated list response
func SendListJSON(c echo.Context, items interface{}, count int, pagination *utils.PaginationParams) error {
	response := CreateListResponse(items, count, pagination)
	return c.JSON(http.StatusOK, response)
}

// SendDeletionJSON sends a deletion success response
func SendDeletionJSON(c echo.Context, resourceType string, identifier interface{}) error {
	response := CreateDeletionResponse(resourceType, identifier)
	return c.JSON(http.StatusOK, response)
}

// ===================================================================
// VALIDATION ERROR HELPERS
// ===================================================================

// HandleBindError handles request binding errors
func HandleBindError(c echo.Context, err error) error {
	return BadRequestError("Invalid request body: %v", err)
}

// HandleValidationError handles validation errors
func HandleValidationError(c echo.Context, err error) error {
	return BadRequestError("Validation failed: %v", err)
}

// ValidateRequiredFields validates that required fields are present
func ValidateRequiredFields(fields map[string]string) error {
	return utils.ValidateRequired(fields)
}

// ===================================================================
// OPERATION RESPONSE PATTERNS
// ===================================================================

// CreateOperationResponse creates response for CRUD operations
func CreateOperationResponse(operation, resourceType string, identifier interface{}, data interface{}) map[string]interface{} {
	message := fmt.Sprintf("%s %s successfully", resourceType, operation)
	if identifier != nil {
		message = fmt.Sprintf("%s %v %s successfully", resourceType, identifier, operation)
	}

	return CreateSuccessResponse(message, data)
}

// SendCreateResponse sends response for create operations
func SendCreateResponse(c echo.Context, resourceType string, data interface{}) error {
	response := CreateOperationResponse("created", resourceType, nil, data)
	return c.JSON(http.StatusCreated, response)
}

// SendUpdateResponse sends response for update operations
func SendUpdateResponse(c echo.Context, resourceType string, identifier interface{}, data interface{}) error {
	response := CreateOperationResponse("updated", resourceType, identifier, data)
	return c.JSON(http.StatusOK, response)
}

// SendGetResponse sends response for get operations
func SendGetResponse(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, data)
}

// ===================================================================
// CONDITIONAL RESPONSES
// ===================================================================

// SendConditionalResponse sends different responses based on condition
func SendConditionalResponse(c echo.Context, condition bool, successMessage, errorMessage string, data interface{}) error {
	if condition {
		return SendOKJSON(c, successMessage, data)
	}
	return SendErrorJSON(c, http.StatusInternalServerError, errorMessage)
}

// SendRepositoryResult handles repository operation results
func SendRepositoryResult(c echo.Context, data interface{}, err error, successMessage string) error {
	if err != nil {
		return HandleRepositoryError(c, err)
	}

	return SendOKJSON(c, successMessage, data)
}

// SendCreationResult handles creation operation results
func SendCreationResult(c echo.Context, data interface{}, err error, resourceType string) error {
	if err != nil {
		return HandleRepositoryError(c, err)
	}

	return SendCreateResponse(c, resourceType, data)
}

// SendUpdateResult handles update operation results
func SendUpdateResult(c echo.Context, data interface{}, err error, resourceType string, identifier interface{}) error {
	if err != nil {
		return HandleRepositoryError(c, err)
	}

	return SendUpdateResponse(c, resourceType, identifier, data)
}

// SendDeletionResult handles deletion operation results
func SendDeletionResult(c echo.Context, err error, resourceType string, identifier interface{}) error {
	if err != nil {
		return HandleRepositoryError(c, err)
	}

	return SendDeletionJSON(c, resourceType, identifier)
}

// SendListResult handles list operation results
func SendListResult(c echo.Context, items interface{}, err error, pagination *utils.PaginationParams) error {
	if err != nil {
		return HandleRepositoryError(c, err)
	}

	// Extract count from items if it's a slice
	count := 0
	if itemSlice, ok := items.([]interface{}); ok {
		count = len(itemSlice)
	}

	return SendListJSON(c, items, count, pagination)
}
