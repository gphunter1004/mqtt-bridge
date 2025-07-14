// internal/command/types.go
package command

import "time"

// DirectActionRequest 직접 액션 명령 요청
type DirectActionRequest struct {
	FullCommand string    `json:"full_command"`
	BaseCommand string    `json:"base_command"`
	CommandType rune      `json:"command_type"`
	ArmParam    string    `json:"arm_param"`
	Timestamp   time.Time `json:"timestamp"`
}

// CommandResult 명령 처리 결과
type CommandResult struct {
	Command   string    `json:"command"`
	Status    string    `json:"status"` // S, F, R
	Message   string    `json:"message"`
	OrderID   string    `json:"order_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ResponseStatus 응답 상태 상수
const (
	StatusSuccess  = "S"
	StatusFailure  = "F"
	StatusRejected = "R"
)

// CommandType 명령 타입 상수
const (
	CommandTypeInference  = 'I'
	CommandTypeTrajectory = 'T'
)

// ArmType 팔 타입 상수
const (
	ArmRight      = "right"
	ArmLeft       = "left"
	ArmParamRight = "R"
	ArmParamLeft  = "L"
)

// IsValidCommandType 유효한 명령 타입인지 확인
func IsValidCommandType(cmdType rune) bool {
	return cmdType == CommandTypeInference || cmdType == CommandTypeTrajectory
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
