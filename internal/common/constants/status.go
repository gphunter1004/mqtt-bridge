// internal/common/constants/status.go (개선된 버전)
package constants

// Response Status 응답 상태 상수
const (
	StatusSuccess  = "S"
	StatusFailure  = "F"
	StatusRejected = "X" // R 대신 X 사용 (더 명확한 거부 의미)
	StatusRunning  = "R" // 새로 추가: Running (진행 중)
	StatusAbnormal = "A" // 비정상 상태
	StatusNormal   = "N" // 정상 상태
)

// Command Status DB 저장용 상태 상수
const (
	CommandStatusPending  = "PENDING"
	CommandStatusRunning  = "RUNNING"
	CommandStatusSuccess  = "SUCCESS"
	CommandStatusFailure  = "FAILURE"
	CommandStatusAbnormal = "ABNORMAL"
	CommandStatusNormal   = "NORMAL"
	CommandStatusRejected = "REJECTED"
)

// Execution Status 실행 상태 상수
const (
	CommandExecutionStatusPending   = "PENDING"
	CommandExecutionStatusRunning   = "RUNNING"
	CommandExecutionStatusCompleted = "COMPLETED"
	CommandExecutionStatusFailed    = "FAILED"
	CommandExecutionStatusCancelled = "CANCELLED"

	OrderExecutionStatusPending   = "PENDING"
	OrderExecutionStatusRunning   = "RUNNING"
	OrderExecutionStatusWaiting   = "WAITING"
	OrderExecutionStatusCompleted = "COMPLETED"
	OrderExecutionStatusFailed    = "FAILED"

	StepExecutionStatusPending  = "PENDING"
	StepExecutionStatusRunning  = "RUNNING"
	StepExecutionStatusFinished = "FINISHED"
	StepExecutionStatusFailed   = "FAILED"
	StepExecutionStatusSkipped  = "SKIPPED"
	StepExecutionStatusTimeout  = "TIMEOUT"
)

// Robot Connection State 로봇 연결 상태 상수
const (
	ConnectionStateOnline           = "ONLINE"
	ConnectionStateOffline          = "OFFLINE"
	ConnectionStateConnectionBroken = "CONNECTIONBROKEN"
)

// Operating Mode 운영 모드 상수
const (
	OperatingModeAutomatic     = "AUTOMATIC"
	OperatingModeManual        = "MANUAL"
	OperatingModeSemiautomatic = "SEMIAUTOMATIC"
	OperatingModeService       = "SERVICE"
	OperatingModeTeach         = "TEACH"
)

// Action Status 액션 상태 상수
const (
	ActionStatusWaiting      = "WAITING"
	ActionStatusInitializing = "INITIALIZING"
	ActionStatusRunning      = "RUNNING"
	ActionStatusPaused       = "PAUSED"
	ActionStatusFinished     = "FINISHED"
	ActionStatusFailed       = "FAILED"
)

// Step Result 단계 결과 상수
const (
	PreviousResultAny      = "ALWAYS"
	PreviousResultSuccess  = "SUCCESS"
	PreviousResultFailure  = "FAILURE"
	PreviousResultAbnormal = "ABNORMAL"
	PreviousResultNormal   = "NORMAL"
)

// Blocking Type 블로킹 타입 상수
const (
	BlockingTypeNone = "NONE"
	BlockingTypeSoft = "SOFT"
	BlockingTypeHard = "HARD"
)

// Direction 방향 상수
const (
	DirectionStraight = "STRAIGHT"
	DirectionLeft     = "LEFT"
	DirectionRight    = "RIGHT"
)

// E-Stop Status E-Stop 상태 상수
const (
	EStopNone      = "NONE"
	EStopAutoAck   = "AUTOACK"
	EStopManualAck = "MANUALACK"
)

// Command Type 명령 타입 상수
const (
	CommandTypeInference  = 'I'
	CommandTypeTrajectory = 'T'
	CommandOrderCancel    = "OC"
)

// Arm Type 팔 타입 상수
const (
	ArmRight      = "right"
	ArmLeft       = "left"
	ArmParamRight = "R"
	ArmParamLeft  = "L"
)

// Action Type 액션 타입 상수
const (
	ActionTypeInitPosition     = "initPosition"
	ActionTypeFactsheetRequest = "factsheetRequest"
	ActionTypeCancelOrder      = "cancelOrder"
	ActionTypeInference        = "Roboligent Robin - Inference"
	ActionTypeTrajectory       = "Roboligent Robin - Follow Trajectory"
)

// MQTT Topics MQTT 토픽 상수
const (
	TopicBridgeCommand   = "bridge/command"
	TopicBridgeResponse  = "bridge/response"
	TopicMeiliConnection = "meili/v2/+/+/connection"
	TopicMeiliState      = "meili/v2/+/+/state"
	TopicMeiliFactsheet  = "meili/v2/+/+/factsheet"
)

// MQTT Topic Patterns MQTT 토픽 패턴
func GetMeiliOrderTopic(manufacturer, serialNumber string) string {
	return "meili/v2/" + manufacturer + "/" + serialNumber + "/order"
}

func GetMeiliInstantActionsTopic(manufacturer, serialNumber string) string {
	return "meili/v2/" + manufacturer + "/" + serialNumber + "/instantActions"
}

// IsValidConnectionState 유효한 연결 상태인지 확인
func IsValidConnectionState(state string) bool {
	validStates := []string{
		ConnectionStateOnline,
		ConnectionStateOffline,
		ConnectionStateConnectionBroken,
	}
	for _, s := range validStates {
		if state == s {
			return true
		}
	}
	return false
}

// ParseArmParam 팔 파라미터를 파싱
func ParseArmParam(armParam string) string {
	switch armParam {
	case ArmParamRight, "":
		return ArmRight
	case ArmParamLeft:
		return ArmLeft
	default:
		return ArmRight // 기본값
	}
}

// ValidateArmParam 팔 파라미터 유효성 검사
func ValidateArmParam(armParam string) bool {
	return armParam == "" || armParam == ArmParamRight || armParam == ArmParamLeft
}
