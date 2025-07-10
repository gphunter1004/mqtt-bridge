// internal/models/robot_state.go
package models

import (
	"time"

	"gorm.io/gorm"
)

// RobotStateMessage 로봇 상태 메시지
type RobotStateMessage struct {
	ActionStates          []ActionState `json:"actionStates"`
	AgvPosition           AgvPosition   `json:"agvPosition"`
	BatteryState          BatteryState  `json:"batteryState"`
	DistanceSinceLastNode float64       `json:"distanceSinceLastNode"`
	Driving               bool          `json:"driving"`
	EdgeStates            []EdgeState   `json:"edgeStates"`
	Errors                []ErrorInfo   `json:"errors"`
	HeaderID              int64         `json:"headerId"`
	Information           []InfoMessage `json:"information"`
	LastNodeID            string        `json:"lastNodeId"`
	LastNodeSequenceID    int           `json:"lastNodeSequenceId"`
	Manufacturer          string        `json:"manufacturer"`
	NewBaseRequest        bool          `json:"newBaseRequest"`
	NodeStates            []NodeState   `json:"nodeStates"`
	OperatingMode         string        `json:"operatingMode"`
	OrderID               string        `json:"orderId"`
	OrderUpdateID         int           `json:"orderUpdateId"`
	Paused                bool          `json:"paused"`
	SafetyState           SafetyState   `json:"safetyState"`
	SerialNumber          string        `json:"serialNumber"`
	Timestamp             string        `json:"timestamp"`
	Velocity              Velocity      `json:"velocity"`
	Version               string        `json:"version"`
}

// ActionState 액션 상태 정보
type ActionState struct {
	ActionDescription string `json:"actionDescription"`
	ActionID          string `json:"actionId"`
	ActionStatus      string `json:"actionStatus"`
	ActionType        string `json:"actionType"`
	ResultDescription string `json:"resultDescription"`
}

// AgvPosition AGV 위치 정보
type AgvPosition struct {
	DeviationRange      float64 `json:"deviationRange"`
	LocalizationScore   float64 `json:"localizationScore"`
	MapDescription      string  `json:"mapDescription"`
	MapID               string  `json:"mapId"`
	PositionInitialized bool    `json:"positionInitialized"`
	Theta               float64 `json:"theta"`
	X                   float64 `json:"x"`
	Y                   float64 `json:"y"`
}

// BatteryState 배터리 상태 정보
type BatteryState struct {
	BatteryCharge  float64 `json:"batteryCharge"`
	BatteryHealth  int     `json:"batteryHealth"`
	BatteryVoltage float64 `json:"batteryVoltage"`
	Charging       bool    `json:"charging"`
	Reach          int     `json:"reach"`
}

// EdgeState 엣지 상태 정보
type EdgeState struct {
	EdgeID     string `json:"edgeId"`
	EndNodeID  string `json:"endNodeId"`
	Released   bool   `json:"released"`
	SequenceID int    `json:"sequenceId"`
}

// ErrorInfo 에러 정보
type ErrorInfo struct {
	ErrorType        string                 `json:"errorType"`
	ErrorDescription string                 `json:"errorDescription"`
	ErrorLevel       string                 `json:"errorLevel"`
	ErrorReferences  []ErrorReference       `json:"errorReferences"`
	AdditionalData   map[string]interface{} `json:"additionalData"`
}

// ErrorReference 에러 참조 정보
type ErrorReference struct {
	ReferenceKey   string `json:"referenceKey"`
	ReferenceValue string `json:"referenceValue"`
}

// InfoMessage 정보 메시지
type InfoMessage struct {
	InfoType        string                 `json:"infoType"`
	InfoDescription string                 `json:"infoDescription"`
	InfoLevel       string                 `json:"infoLevel"`
	InfoReferences  []InfoReference        `json:"infoReferences"`
	AdditionalData  map[string]interface{} `json:"additionalData"`
}

// InfoReference 정보 참조
type InfoReference struct {
	ReferenceKey   string `json:"referenceKey"`
	ReferenceValue string `json:"referenceValue"`
}

// NodeState 노드 상태 정보
type NodeState struct {
	NodeID       string       `json:"nodeId"`
	NodePosition NodePosition `json:"nodePosition"`
	Released     bool         `json:"released"`
	SequenceID   int          `json:"sequenceId"`
}

// NodePosition 노드 위치 정보
type NodePosition struct {
	Theta float64 `json:"theta"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

// SafetyState 안전 상태 정보
type SafetyState struct {
	EStop          string `json:"eStop"`
	FieldViolation bool   `json:"fieldViolation"`
}

// Velocity 속도 정보
type Velocity struct {
	Omega float64 `json:"omega"`
	Vx    float64 `json:"vx"`
	Vy    float64 `json:"vy"`
}

// RobotState 로봇 상태 정보 (DB 저장용)
type RobotState struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SerialNumber string    `gorm:"size:50;not null;index" json:"serial_number"`
	Manufacturer string    `gorm:"size:50;not null" json:"manufacturer"`
	Version      string    `gorm:"size:10" json:"version"`
	HeaderID     int64     `gorm:"not null" json:"header_id"`
	Timestamp    time.Time `gorm:"not null" json:"timestamp"`

	// 위치 정보
	PositionX           float64 `json:"position_x"`
	PositionY           float64 `json:"position_y"`
	PositionTheta       float64 `json:"position_theta"`
	LocalizationScore   float64 `json:"localization_score"`
	PositionInitialized bool    `json:"position_initialized"`
	MapID               string  `gorm:"size:100" json:"map_id"`

	// 배터리 정보
	BatteryCharge  float64 `json:"battery_charge"`
	BatteryVoltage float64 `json:"battery_voltage"`
	BatteryHealth  int     `json:"battery_health"`
	Charging       bool    `json:"charging"`

	// 운영 상태
	OperatingMode string `gorm:"size:20" json:"operating_mode"`
	Driving       bool   `json:"driving"`
	Paused        bool   `json:"paused"`

	// 안전 상태
	EStop          string `gorm:"size:20" json:"e_stop"`
	FieldViolation bool   `json:"field_violation"`

	// 주문 정보
	OrderID    string `gorm:"size:100" json:"order_id"`
	LastNodeID string `gorm:"size:100" json:"last_node_id"`

	// 상태 정보
	ErrorCount  int `json:"error_count"`
	ActionCount int `json:"action_count"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// InitPositionRequest initPosition 요청 메시지
type InitPositionRequest struct {
	HeaderID     int64    `json:"headerId"`
	Timestamp    string   `json:"timestamp"`
	Version      string   `json:"version"`
	Manufacturer string   `json:"manufacturer"`
	SerialNumber string   `json:"serialNumber"`
	Actions      []Action `json:"actions"`
}

// PoseValue 위치 정보
type PoseValue struct {
	LastNodeID string  `json:"lastNodeId"`
	MapID      string  `json:"mapId"`
	Theta      float64 `json:"theta"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
}

// 운영 모드 상수
const (
	OperatingModeAutomatic     = "AUTOMATIC"
	OperatingModeManual        = "MANUAL"
	OperatingModeSemiautomatic = "SEMIAUTOMATIC"
	OperatingModeService       = "SERVICE"
	OperatingModeTeach         = "TEACH"
)

// 액션 상태 상수
const (
	ActionStatusWaiting      = "WAITING"
	ActionStatusInitializing = "INITIALIZING"
	ActionStatusRunning      = "RUNNING"
	ActionStatusPaused       = "PAUSED"
	ActionStatusFinished     = "FINISHED"
	ActionStatusFailed       = "FAILED"
)

// E-Stop 상태 상수
const (
	EStopNone      = "NONE"
	EStopAutoAck   = "AUTOACK"
	EStopManualAck = "MANUALACK"
)

// 액션 타입 상수
const (
	ActionTypeInitPosition     = "initPosition"
	ActionTypeFactsheetRequest = "factsheetRequest"
)

// 블로킹 타입 상수
const (
	BlockingTypeNone = "NONE"
	BlockingTypeHard = "HARD"
	BlockingTypeSoft = "SOFT"
)

// IsValidOperatingMode 유효한 운영 모드인지 확인
func IsValidOperatingMode(mode string) bool {
	validModes := []string{
		OperatingModeAutomatic,
		OperatingModeManual,
		OperatingModeSemiautomatic,
		OperatingModeService,
		OperatingModeTeach,
	}

	for _, validMode := range validModes {
		if mode == validMode {
			return true
		}
	}
	return false
}

// IsValidActionStatus 유효한 액션 상태인지 확인
func IsValidActionStatus(status string) bool {
	validStatuses := []string{
		ActionStatusWaiting,
		ActionStatusInitializing,
		ActionStatusRunning,
		ActionStatusPaused,
		ActionStatusFinished,
		ActionStatusFailed,
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// IsValidEStopStatus 유효한 E-Stop 상태인지 확인
func IsValidEStopStatus(status string) bool {
	validStatuses := []string{
		EStopNone,
		EStopAutoAck,
		EStopManualAck,
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}
