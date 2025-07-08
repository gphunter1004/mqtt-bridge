package base

import (
	"strconv"

	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// ===================================================================
// PARAMETER EXTRACTION HELPERS
// ===================================================================

// ExtractIDParam extracts and validates ID parameter from URL
func ExtractIDParam(c echo.Context, paramName string) (uint, error) {
	idStr := c.Param(paramName)
	if idStr == "" {
		return 0, BadRequestError("%s parameter is required", paramName)
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return 0, BadRequestError("Invalid %s parameter: must be a valid integer", paramName)
	}

	return uint(id), nil
}

// ExtractStringParam extracts string parameter from URL with validation
func ExtractStringParam(c echo.Context, paramName string, required bool) (string, error) {
	value := c.Param(paramName)
	if required && value == "" {
		return "", BadRequestError("%s parameter is required", paramName)
	}

	return value, nil
}

// ExtractSerialNumber extracts and validates serial number parameter
func ExtractSerialNumber(c echo.Context) (string, error) {
	return ExtractStringParam(c, "serialNumber", true)
}

// ExtractOrderID extracts and validates order ID parameter
func ExtractOrderID(c echo.Context) (string, error) {
	return ExtractStringParam(c, "orderId", true)
}

// ExtractActionID extracts action ID parameter (database ID)
func ExtractActionID(c echo.Context) (uint, error) {
	return ExtractIDParam(c, "actionId")
}

// ExtractNodeID extracts node ID parameter (database ID)
func ExtractNodeID(c echo.Context) (uint, error) {
	return ExtractIDParam(c, "nodeId")
}

// ExtractEdgeID extracts edge ID parameter (database ID)
func ExtractEdgeID(c echo.Context) (uint, error) {
	return ExtractIDParam(c, "edgeId")
}

// ExtractTemplateID extracts template ID parameter
func ExtractTemplateID(c echo.Context) (uint, error) {
	return ExtractIDParam(c, "id")
}

// ===================================================================
// QUERY PARAMETER HELPERS
// ===================================================================

// ExtractPaginationParams extracts pagination parameters from query
func ExtractPaginationParams(c echo.Context, defaultLimit int) utils.PaginationParams {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	return utils.GetPaginationParams(limitStr, offsetStr, defaultLimit)
}

// ExtractSearchParam extracts search parameter from query
func ExtractSearchParam(c echo.Context) string {
	return c.QueryParam("search")
}

// ExtractFilterParam extracts filter parameter from query
func ExtractFilterParam(c echo.Context, paramName string) string {
	return c.QueryParam(paramName)
}

// ExtractOptionalStringParam extracts optional string parameter with default
func ExtractOptionalStringParam(c echo.Context, paramName, defaultValue string) string {
	value := c.QueryParam(paramName)
	return utils.GetValueOrDefault(value, defaultValue)
}

// ExtractOptionalIntParam extracts optional integer parameter with default
func ExtractOptionalIntParam(c echo.Context, paramName string, defaultValue int) int {
	valueStr := c.QueryParam(paramName)
	return utils.GetIntOrDefault(valueStr, defaultValue)
}

// ExtractOptionalBoolParam extracts optional boolean parameter with default
func ExtractOptionalBoolParam(c echo.Context, paramName string, defaultValue bool) bool {
	valueStr := c.QueryParam(paramName)
	return utils.GetBoolOrDefault(valueStr, defaultValue)
}

// ===================================================================
// PARAMETER VALIDATION HELPERS
// ===================================================================

// ValidatePaginationParams validates pagination parameters
func ValidatePaginationParams(pagination utils.PaginationParams) error {
	if pagination.Limit < 0 {
		return BadRequestError("limit must be non-negative")
	}

	if pagination.Offset < 0 {
		return BadRequestError("offset must be non-negative")
	}

	// Optional: set maximum limit
	if pagination.Limit > 1000 {
		return BadRequestError("limit cannot exceed 1000")
	}

	return nil
}

// ValidateID validates that ID is positive
func ValidateID(id uint, fieldName string) error {
	if id == 0 {
		return BadRequestError("%s must be a positive integer", fieldName)
	}
	return nil
}

// ValidateStringParam validates string parameter
func ValidateStringParam(value, fieldName string, required bool) error {
	if required && value == "" {
		return BadRequestError("%s is required", fieldName)
	}
	return nil
}

// ===================================================================
// REQUEST BODY HELPERS
// ===================================================================

// BindAndValidateJSON binds JSON request body and handles errors
func BindAndValidateJSON(c echo.Context, target interface{}) error {
	if err := c.Bind(target); err != nil {
		return HandleBindError(c, err)
	}
	return nil
}

// BindJSONWithValidation binds JSON and performs custom validation
func BindJSONWithValidation(c echo.Context, target interface{}, validator func(interface{}) error) error {
	if err := BindAndValidateJSON(c, target); err != nil {
		return err
	}

	if validator != nil {
		if err := validator(target); err != nil {
			return HandleValidationError(c, err)
		}
	}

	return nil
}

// ===================================================================
// PARAMETER COMBINATION HELPERS
// ===================================================================

// ExtractCRUDParams extracts common CRUD operation parameters
type CRUDParams struct {
	ID         uint                   `json:"id"`
	Pagination utils.PaginationParams `json:"pagination"`
	Search     string                 `json:"search"`
}

// ExtractStandardCRUDParams extracts standard CRUD parameters
func ExtractStandardCRUDParams(c echo.Context, defaultLimit int) (*CRUDParams, error) {
	params := &CRUDParams{
		Pagination: ExtractPaginationParams(c, defaultLimit),
		Search:     ExtractSearchParam(c),
	}

	// Extract ID if present (for GET, PUT, DELETE operations)
	if idStr := c.Param("id"); idStr != "" {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return nil, BadRequestError("Invalid ID parameter")
		}
		params.ID = uint(id)
	}

	return params, nil
}

// ===================================================================
// FILTER PARAMETER HELPERS
// ===================================================================

// FilterParams represents common filter parameters
type FilterParams struct {
	Type         string `json:"type"`
	Status       string `json:"status"`
	SerialNumber string `json:"serialNumber"`
	DateFrom     string `json:"dateFrom"`
	DateTo       string `json:"dateTo"`
}

// ExtractFilterParams extracts common filter parameters
func ExtractFilterParams(c echo.Context) FilterParams {
	return FilterParams{
		Type:         c.QueryParam("type"),
		Status:       c.QueryParam("status"),
		SerialNumber: c.QueryParam("serialNumber"),
		DateFrom:     c.QueryParam("dateFrom"),
		DateTo:       c.QueryParam("dateTo"),
	}
}

// ExtractActionFilterParams extracts action-specific filter parameters
func ExtractActionFilterParams(c echo.Context) map[string]string {
	return map[string]string{
		"actionType":   c.QueryParam("actionType"),
		"blockingType": c.QueryParam("blockingType"),
		"search":       c.QueryParam("search"),
	}
}

// ===================================================================
// PARAMETER LOGGING HELPERS
// ===================================================================

// LogExtractedParams logs extracted parameters for debugging
func LogExtractedParams(component utils.LogComponent, operation string, params interface{}) {
	utils.LogDebug(component, "Extracted parameters for %s: %+v", operation, params)
}

// LogInvalidParam logs invalid parameter extraction
func LogInvalidParam(component utils.LogComponent, paramName string, value string, err error) {
	utils.LogWarn(component, "Invalid parameter %s='%s': %v", paramName, value, err)
}
