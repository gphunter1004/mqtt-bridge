package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"mqtt-bridge/models"

	"github.com/labstack/echo/v4"
)

// ===================================================================
// ID GENERATION HELPERS
// ===================================================================

func GenerateUniqueID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
func GenerateOrderID() string {
	return fmt.Sprintf("order_%s", GenerateUniqueID())
}
func GenerateNodeID() string {
	return fmt.Sprintf("node_%s", GenerateUniqueID())
}
func GenerateEdgeID() string {
	return fmt.Sprintf("edge_%s", GenerateUniqueID())
}
func GenerateActionID() string {
	return fmt.Sprintf("action_%s", GenerateUniqueID())
}

// ===================================================================
// STRING & TYPE CONVERSION HELPERS
// ===================================================================

func GetValueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}
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
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value), nil
		}
		return string(jsonBytes), nil
	}
}

// ===================================================================
// JSON HELPERS
// ===================================================================

func ParseJSONToUintSlice(jsonStr string) ([]uint, error) {
	if jsonStr == "" {
		return []uint{}, nil
	}
	var ids []uint
	return ids, json.Unmarshal([]byte(jsonStr), &ids)
}
func ConvertUintSliceToJSON(ids []uint) (string, error) {
	data, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ===================================================================
// HANDLER HELPERS
// ===================================================================

type PaginationParams struct {
	Limit  int
	Offset int
}

func GetPaginationParams(limitStr, offsetStr string, defaultLimit int) PaginationParams {
	limit := defaultLimit
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}
	return PaginationParams{Limit: limit, Offset: offset}
}

// ParseUintParam parses a URL parameter into a uint, returning a custom AppError on failure.
func ParseUintParam(c echo.Context, paramName string) (uint, error) {
	idStr := c.Param(paramName)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return 0, NewBadRequestError(fmt.Sprintf("Invalid %s parameter: must be a positive integer.", paramName))
	}
	return uint(id), nil
}

// BindAndValidate binds the request body and returns a custom AppError on failure.
func BindAndValidate(c echo.Context, req interface{}) error {
	if err := c.Bind(req); err != nil {
		return NewBadRequestError("Invalid request body: please check the JSON format.", err)
	}
	// TODO: Add struct validation if needed (e.g., using 'go-playground/validator')
	return nil
}

// ===================================================================
// API RESPONSE HELPERS
// ===================================================================

type StandardResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func SuccessResponse(message string, data interface{}) StandardResponse {
	return StandardResponse{Status: "success", Message: message, Data: data}
}
func ErrorResponse(message string) StandardResponse {
	return StandardResponse{Status: "error", Message: message}
}

type ListResponse struct {
	Items  interface{} `json:"items"`
	Count  int         `json:"count"`
	Limit  int         `json:"limit,omitempty"`
	Offset int         `json:"offset,omitempty"`
}

func CreateListResponse(items interface{}, count int, pagination *PaginationParams) ListResponse {
	response := ListResponse{Items: items, Count: count}
	if pagination != nil {
		response.Limit = pagination.Limit
		response.Offset = pagination.Offset
	}
	return response
}

// ===================================================================
// TIME HELPERS
// ===================================================================

func GetCurrentTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
}
func GetUnixTimestamp() int64 {
	return time.Now().Unix()
}

// ===================================================================
// VALIDATION HELPERS
// ===================================================================

func ValidateRequired(fields map[string]string) error {
	for fieldName, value := range fields {
		if value == "" {
			return fmt.Errorf("%s is a required field", fieldName)
		}
	}
	return nil
}

// ===================================================================
// MODEL HELPERS
// ===================================================================

func GetDefaultNodePosition() models.NodePosition {
	return models.NodePosition{
		X: 0.0, Y: 0.0, Theta: 0.0,
		AllowedDeviationXY: 0.0, AllowedDeviationTheta: 0.0,
		MapID: "",
	}
}
func AddCustomParameters(baseParams []models.ActionParameter, customParams map[string]interface{}) []models.ActionParameter {
	for key, value := range customParams {
		baseParams = append(baseParams, models.ActionParameter{Key: key, Value: value})
	}
	return baseParams
}
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
