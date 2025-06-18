package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PLCData represents data received from PLC
type PLCData struct {
	Topic     string      `json:"topic"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// PLCStatus represents PLC status information
type PLCStatus struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
	Values   []int  `json:"values"`
}

// PLCCommand represents PLC command information
type PLCCommand struct {
	CommandID string   `json:"command_id"`
	Action    string   `json:"action"`
	Params    []string `json:"params"`
}

// ParsePLCData parses PLC data from different formats
func ParsePLCData(topic string, payload []byte) (*PLCData, error) {
	plcData := &PLCData{
		Topic:     topic,
		Timestamp: getCurrentTimestamp(),
	}

	// Try to parse as JSON first
	var jsonData interface{}
	if err := json.Unmarshal(payload, &jsonData); err == nil {
		plcData.Data = jsonData
		return plcData, nil
	}

	// Parse as string array (comma-separated)
	payloadStr := string(payload)
	if strings.Contains(payloadStr, ",") {
		parts := strings.Split(payloadStr, ",")
		var values []string
		for _, part := range parts {
			values = append(values, strings.TrimSpace(part))
		}
		plcData.Data = values
		return plcData, nil
	}

	// Parse as byte array or single string
	if isNumericBytes(payload) {
		var values []int
		for _, b := range payload {
			values = append(values, int(b))
		}
		plcData.Data = values
	} else {
		plcData.Data = payloadStr
	}

	return plcData, nil
}

// ParsePLCStatus parses PLC status data specifically
func ParsePLCStatus(payload []byte) (*PLCStatus, error) {
	// Try JSON format first
	var status PLCStatus
	if err := json.Unmarshal(payload, &status); err == nil {
		return &status, nil
	}

	// Parse from string format: "device_id:status:value1,value2,value3"
	payloadStr := string(payload)
	parts := strings.Split(payloadStr, ":")
	if len(parts) >= 3 {
		status.DeviceID = parts[0]
		status.Status = parts[1]

		// Parse values
		valueStr := strings.Join(parts[2:], ":")
		if valueParts := strings.Split(valueStr, ","); len(valueParts) > 0 {
			for _, v := range valueParts {
				if val, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					status.Values = append(status.Values, val)
				}
			}
		}
		return &status, nil
	}

	return nil, fmt.Errorf("invalid PLC status format")
}

// ParsePLCCommand parses PLC command data
func ParsePLCCommand(payload []byte) (*PLCCommand, error) {
	// Try JSON format first
	var command PLCCommand
	if err := json.Unmarshal(payload, &command); err == nil {
		return &command, nil
	}

	// Parse from string format: "command_id:action:param1,param2,param3"
	payloadStr := string(payload)
	parts := strings.Split(payloadStr, ":")
	if len(parts) >= 2 {
		command.CommandID = parts[0]
		command.Action = parts[1]

		if len(parts) > 2 {
			paramStr := strings.Join(parts[2:], ":")
			if paramParts := strings.Split(paramStr, ","); len(paramParts) > 0 {
				for _, p := range paramParts {
					command.Params = append(command.Params, strings.TrimSpace(p))
				}
			}
		}
		return &command, nil
	}

	return nil, fmt.Errorf("invalid PLC command format")
}

func isNumericBytes(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 { // Not printable ASCII
			return true
		}
	}
	return false
}

func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
