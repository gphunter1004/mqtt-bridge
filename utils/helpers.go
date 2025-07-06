package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"mqtt-bridge/models"
)

// ===================================================================
// ID GENERATION HELPERS
// ===================================================================

// GenerateUniqueID generates a unique ID based on current timestamp
func GenerateUniqueID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// GenerateOrderID generates a unique order ID with prefix
func GenerateOrderID() string {
	return fmt.Sprintf("order_%s", GenerateUniqueID())
}

// GenerateNodeID generates a unique node ID with prefix
func GenerateNodeID() string {
	return fmt.Sprintf("node_%s", GenerateUniqueID())
}

// GenerateEdgeID generates a unique edge ID with prefix
func GenerateEdgeID() string {
	return fmt.Sprintf("edge_%s", GenerateUniqueID())
}

// GenerateActionID generates a unique action ID with prefix
func GenerateActionID() string {
	return fmt.Sprintf("action_%s", GenerateUniqueID())
}

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
// JSON HELPERS
// ===================================================================

// ConvertValueToString converts interface{} to JSON string based on type
func ConvertValueToString(value interface{}, valueType string) (string, error) {
	if value == nil {
		return "", nil
	}

	switch valueType {
	case "string":
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
	case "object", "number", "boolean":
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

// ParseJSONToUintSlice parses JSON string to []uint
func ParseJSONToUintSlice(jsonStr string) ([]uint, error) {
	if jsonStr == "" {
		return []uint{}, nil
	}

	var ids []uint
	err := json.Unmarshal([]byte(jsonStr), &ids)
	return ids, err
}

// ConvertUintSliceToJSON converts []uint to JSON string
func ConvertUintSliceToJSON(ids []uint) (string, error) {
	data, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(data), nil
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

// ValidateNodeRequest validates node template request
func ValidateNodeRequest(req interface{}) error {
	// Can be extended based on specific validation needs
	return nil
}

// ValidateEdgeRequest validates edge template request
func ValidateEdgeRequest(req interface{}) error {
	// Can be extended based on specific validation needs
	return nil
}

// ValidateActionRequest validates action template request
func ValidateActionRequest(req interface{}) error {
	// Can be extended based on specific validation needs
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
// DATABASE HELPERS
// ===================================================================

// UpdateFields represents fields to update in database
type UpdateFields map[string]interface{}

// CreateUpdateFields creates update fields with timestamp
func CreateUpdateFields(fields map[string]interface{}) UpdateFields {
	update := make(UpdateFields)
	for k, v := range fields {
		update[k] = v
	}
	update["updated_at"] = time.Now()
	return update
}

// AddCompletionFields adds completion timestamp and status
func (uf UpdateFields) AddCompletionFields(status string) UpdateFields {
	if status == "COMPLETED" || status == "FAILED" || status == "CANCELLED" {
		uf["completed_at"] = time.Now()
	}
	return uf
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
// LOGGING HELPERS
// ===================================================================

// LogOperation represents an operation type for logging
type LogOperation string

const (
	LogOpCreate  LogOperation = "CREATE"
	LogOpUpdate  LogOperation = "UPDATE"
	LogOpDelete  LogOperation = "DELETE"
	LogOpExecute LogOperation = "EXECUTE"
)

// LogInfo represents structured log information
type LogInfo struct {
	Operation  LogOperation `json:"operation"`
	Resource   string       `json:"resource"`
	ResourceID string       `json:"resourceId"`
	Message    string       `json:"message"`
	Timestamp  time.Time    `json:"timestamp"`
}

// CreateLogInfo creates structured log information
func CreateLogInfo(operation LogOperation, resource, resourceID, message string) LogInfo {
	return LogInfo{
		Operation:  operation,
		Resource:   resource,
		ResourceID: resourceID,
		Message:    message,
		Timestamp:  time.Now(),
	}
}

// FormatLogMessage formats log message with operation info
func (li LogInfo) FormatLogMessage() string {
	return fmt.Sprintf("[%s] %s %s (ID: %s): %s",
		li.Operation, li.Resource, li.ResourceID, li.ResourceID, li.Message)
}
