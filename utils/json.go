package utils

import (
	"encoding/json"
	"fmt"
)

// ===================================================================
// JSON CONVERSION HELPERS
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

// ConvertValueByType converts value to appropriate type based on valueType
func ConvertValueByType(value string, valueType string) interface{} {
	if value == "" {
		return nil
	}

	switch valueType {
	case "object", "number", "boolean":
		var result interface{}
		if err := json.Unmarshal([]byte(value), &result); err == nil {
			return result
		}
		return value
	default:
		return value
	}
}

// SafeJSONMarshal marshals to JSON with fallback
func SafeJSONMarshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		// Fallback to string representation
		return json.Marshal(fmt.Sprintf("%v", v))
	}
	return data, nil
}

// SafeJSONUnmarshal unmarshals JSON with error handling
func SafeJSONUnmarshal(data []byte, v interface{}) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

// GetMapKeys returns all keys from a map[string]interface{}
func GetMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
