package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// RobotData represents data to be sent to robot
type RobotData struct {
	MessageID string      `json:"message_id"`
	Timestamp int64       `json:"timestamp"`
	Source    string      `json:"source"`
	Command   string      `json:"command"`
	Data      interface{} `json:"data"`
}

// RobotStatus represents robot status command
type RobotStatus struct {
	DeviceID string                 `json:"device_id"`
	Status   string                 `json:"status"`
	Values   []int                  `json:"values"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RobotCommand represents robot command
type RobotCommand struct {
	CommandID  string                 `json:"command_id"`
	Action     string                 `json:"action"`
	Parameters []string               `json:"parameters"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// RobotMovement represents robot movement command
type RobotMovement struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
	Speed       float64   `json:"speed"`
	Precision   float64   `json:"precision"`
}

// NewRobotData creates a new robot data structure
func NewRobotData(source, command string, data interface{}) *RobotData {
	return &RobotData{
		MessageID: generateMessageID(),
		Timestamp: time.Now().Unix(),
		Source:    source,
		Command:   command,
		Data:      data,
	}
}

// ToJSON converts robot data to JSON string
func (r *RobotData) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// NewRobotStatus creates robot status from PLC status
func NewRobotStatus(plcStatus *PLCStatus) *RobotStatus {
	return &RobotStatus{
		DeviceID: plcStatus.DeviceID,
		Status:   plcStatus.Status,
		Values:   plcStatus.Values,
		Metadata: map[string]interface{}{
			"converted_from":  "plc_status",
			"conversion_time": time.Now().Unix(),
		},
	}
}

// NewRobotCommand creates robot command from PLC command
func NewRobotCommand(plcCommand *PLCCommand) *RobotCommand {
	return &RobotCommand{
		CommandID:  plcCommand.CommandID,
		Action:     plcCommand.Action,
		Parameters: plcCommand.Params,
		Metadata: map[string]interface{}{
			"converted_from":  "plc_command",
			"conversion_time": time.Now().Unix(),
		},
	}
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}
