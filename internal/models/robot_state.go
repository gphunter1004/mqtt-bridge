// internal/models/robot_state.go (수정된 버전 - 공통 기능 적용)
package models

import "mqtt-bridge/internal/common/constants"

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
	NodeID       string `json:"nodeId"`
	NodePosition struct {
		Theta float64 `json:"theta"`
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
	} `json:"nodePosition"`
	Released   bool `json:"released"`
	SequenceID int  `json:"sequenceId"`
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

// IsValidOperatingMode 유효한 운영 모드인지 확인 (공통 함수 사용)
func IsValidOperatingMode(mode string) bool {
	return constants.IsValidOperatingMode(mode)
}

// IsValidActionStatus 유효한 액션 상태인지 확인 (공통 함수 사용)
func IsValidActionStatus(status string) bool {
	return constants.IsValidActionStatus(status)
}

// IsValidEStopStatus 유효한 E-Stop 상태인지 확인
func IsValidEStopStatus(status string) bool {
	validStatuses := []string{
		constants.EStopNone,
		constants.EStopAutoAck,
		constants.EStopManualAck,
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}
