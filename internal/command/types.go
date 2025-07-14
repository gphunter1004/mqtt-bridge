// internal/command/types.go (수정된 버전 - 공통 기능 적용)
package command

import (
	"mqtt-bridge/internal/common/constants"
	"time"
)

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

// IsValidCommandType 유효한 명령 타입인지 확인
func IsValidCommandType(cmdType rune) bool {
	return cmdType == constants.CommandTypeInference || cmdType == constants.CommandTypeTrajectory
}

// ParseArmParam 팔 파라미터를 파싱 (공통 함수 사용)
func ParseArmParam(armParam string) string {
	return constants.ParseArmParam(armParam)
}

// ValidateArmParam 팔 파라미터 유효성 검사 (공통 함수 사용)
func ValidateArmParam(armParam string) bool {
	return constants.ValidateArmParam(armParam)
}
