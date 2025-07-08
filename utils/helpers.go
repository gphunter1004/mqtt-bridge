package utils

import (
	"fmt"
	"strconv"
	"time"

	"mqtt-bridge/models"
)

// ===================================================================
// STRING HELPERS
// ===================================================================

// GetValueOrDefault returns value if not empty, otherwise returns defaultValue
func GetValueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// GetIntOrDefault returns value if valid, otherwise returns defaultValue
func GetIntOrDefault(valueStr string, defaultValue int) int {
	if valueStr == "" {
		return defaultValue
	}
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// GetBoolOrDefault returns value if valid, otherwise returns defaultValue
func GetBoolOrDefault(valueStr string, defaultValue bool) bool {
	if valueStr == "" {
		return defaultValue
	}
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// ===================================================================
// PAGINATION HELPERS
// ===================================================================

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// GetPaginationParams extracts and validates pagination parameters
func GetPaginationParams(limitStr, offsetStr string, defaultLimit int) PaginationParams {
	limit := defaultLimit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// ===================================================================
// MODEL HELPERS
// ===================================================================

// GetDefaultNodePosition returns a default NodePosition
func GetDefaultNodePosition() models.NodePosition {
	return models.NodePosition{
		X:                     0.0,
		Y:                     0.0,
		Theta:                 0.0,
		AllowedDeviationXY:    0.0,
		AllowedDeviationTheta: 0.0,
		MapID:                 "",
	}
}

// GetPositionOrDefault returns position if not nil, otherwise returns default
func GetPositionOrDefault(position *models.NodePosition) models.NodePosition {
	if position != nil {
		return *position
	}
	return GetDefaultNodePosition()
}

// AddCustomParameters adds custom parameters to base parameters
func AddCustomParameters(baseParams []models.ActionParameter, customParams map[string]interface{}) []models.ActionParameter {
	for key, value := range customParams {
		baseParams = append(baseParams, models.ActionParameter{
			Key:   key,
			Value: value,
		})
	}
	return baseParams
}

// ProcessNodesWithIDs ensures all nodes and their actions have unique IDs
func ProcessNodesWithIDs(nodes []models.Node) []models.Node {
	for i := range nodes {
		if nodes[i].NodeID == "" {
			nodes[i].NodeID = GenerateNodeID()
		}
		for j := range nodes[i].Actions {
			if nodes[i].Actions[j].ActionID == "" {
				nodes[i].Actions[j].ActionID = GenerateActionID()
			}
		}
	}
	return nodes
}

// ProcessEdgesWithIDs ensures all edges and their actions have unique IDs
func ProcessEdgesWithIDs(edges []models.Edge) []models.Edge {
	for i := range edges {
		if edges[i].EdgeID == "" {
			edges[i].EdgeID = GenerateEdgeID()
		}
		for j := range edges[i].Actions {
			if edges[i].Actions[j].ActionID == "" {
				edges[i].Actions[j].ActionID = GenerateActionID()
			}
		}
	}
	return edges
}

// ===================================================================
// TIME HELPERS
// ===================================================================

// GetCurrentTimestamp returns current timestamp in ISO format
func GetCurrentTimestamp() string {
	return time.Now().Format("2006-01-02T15:04:05.000000000Z")
}

// GetUnixTimestamp returns current Unix timestamp
func GetUnixTimestamp() int64 {
	return time.Now().Unix()
}

// ===================================================================
// VALIDATION HELPERS
// ===================================================================

// ValidateRequired checks if required fields are not empty
func ValidateRequired(fields map[string]string) error {
	for fieldName, value := range fields {
		if value == "" {
			return fmt.Errorf("%s is required", fieldName)
		}
	}
	return nil
}

// ===================================================================
// RESPONSE HELPERS
// ===================================================================

// StandardResponse represents a standard API response
type StandardResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse creates a success response
func SuccessResponse(message string, data interface{}) StandardResponse {
	return StandardResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	}
}

// ErrorResponse creates an error response
func ErrorResponse(message string) StandardResponse {
	return StandardResponse{
		Status:  "error",
		Message: message,
	}
}

// ListResponse represents a paginated list response
type ListResponse struct {
	Items  interface{} `json:"items"`
	Count  int         `json:"count"`
	Limit  int         `json:"limit,omitempty"`
	Offset int         `json:"offset,omitempty"`
}

// CreateListResponse creates a standardized list response
func CreateListResponse(items interface{}, count int, pagination *PaginationParams) ListResponse {
	response := ListResponse{
		Items: items,
		Count: count,
	}

	if pagination != nil {
		response.Limit = pagination.Limit
		response.Offset = pagination.Offset
	}

	return response
}

// ===================================================================
// ENUM HELPERS
// ===================================================================

// OrderStatus represents order execution status
type OrderStatus string

const (
	OrderStatusCreated      OrderStatus = "CREATED"
	OrderStatusSent         OrderStatus = "SENT"
	OrderStatusAcknowledged OrderStatus = "ACKNOWLEDGED"
	OrderStatusExecuting    OrderStatus = "EXECUTING"
	OrderStatusCompleted    OrderStatus = "COMPLETED"
	OrderStatusFailed       OrderStatus = "FAILED"
	OrderStatusCancelled    OrderStatus = "CANCELLED"
)

// IsValidOrderStatus checks if status is valid
func IsValidOrderStatus(status string) bool {
	validStatuses := []OrderStatus{
		OrderStatusCreated,
		OrderStatusSent,
		OrderStatusAcknowledged,
		OrderStatusExecuting,
		OrderStatusCompleted,
		OrderStatusFailed,
		OrderStatusCancelled,
	}

	for _, validStatus := range validStatuses {
		if OrderStatus(status) == validStatus {
			return true
		}
	}
	return false
}

// ConnectionState represents robot connection state
type ConnectionState string

const (
	ConnectionStateOnline  ConnectionState = "ONLINE"
	ConnectionStateOffline ConnectionState = "OFFLINE"
)

// IsValidConnectionState checks if connection state is valid
func IsValidConnectionState(state string) bool {
	return ConnectionState(state) == ConnectionStateOnline ||
		ConnectionState(state) == ConnectionStateOffline
}

// ===================================================================
// MANUFACTURER HELPERS
// ===================================================================

// GetDefaultManufacturer returns default manufacturer name
func GetDefaultManufacturer() string {
	return "Roboligent"
}

// GetManufacturerOrDefault returns manufacturer or default if empty
func GetManufacturerOrDefault(manufacturer string) string {
	return GetValueOrDefault(manufacturer, GetDefaultManufacturer())
}
