// internal/common/redis/keys.go
package redis

import "fmt"

// Redis Key Patterns Redis 키 패턴 상수
const (
	// Direct Command 관련
	PendingDirectCommandPattern = "pending_direct_command:%s"

	// Step Action 관련
	StepActionsPattern = "step_actions:%d"

	// Robot Status 관련 (필요시 확장)
	RobotStatusPattern = "robot_status:%s"

	// Command Execution 관련 (필요시 확장)
	CommandExecutionPattern = "command_execution:%d"

	// Session 관련 (필요시 확장)
	SessionPattern = "session:%s"
)

// KeyGenerator Redis 키 생성기
type KeyGenerator struct{}

// NewKeyGenerator 새 키 생성기 생성
func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{}
}

// PendingDirectCommand 대기 중인 직접 명령 키 생성
func (k *KeyGenerator) PendingDirectCommand(orderID string) string {
	return fmt.Sprintf(PendingDirectCommandPattern, orderID)
}

// StepActions 단계 액션 키 생성
func (k *KeyGenerator) StepActions(stepID int) string {
	return fmt.Sprintf(StepActionsPattern, stepID)
}

// RobotStatus 로봇 상태 키 생성
func (k *KeyGenerator) RobotStatus(serialNumber string) string {
	return fmt.Sprintf(RobotStatusPattern, serialNumber)
}

// CommandExecution 명령 실행 키 생성
func (k *KeyGenerator) CommandExecution(executionID int) string {
	return fmt.Sprintf(CommandExecutionPattern, executionID)
}

// Session 세션 키 생성
func (k *KeyGenerator) Session(sessionID string) string {
	return fmt.Sprintf(SessionPattern, sessionID)
}

// 전역 키 생성기 인스턴스
var Keys = NewKeyGenerator()

// 편의 함수들 (전역 키 생성기 사용)

// PendingDirectCommand 대기 중인 직접 명령 키 생성
func PendingDirectCommand(orderID string) string {
	return Keys.PendingDirectCommand(orderID)
}

// StepActions 단계 액션 키 생성
func StepActions(stepID int) string {
	return Keys.StepActions(stepID)
}

// RobotStatus 로봇 상태 키 생성
func RobotStatus(serialNumber string) string {
	return Keys.RobotStatus(serialNumber)
}

// CommandExecution 명령 실행 키 생성
func CommandExecution(executionID int) string {
	return Keys.CommandExecution(executionID)
}

// Session 세션 키 생성
func Session(sessionID string) string {
	return Keys.Session(sessionID)
}

// Pattern Matching 패턴 매칭용 함수들

// AllPendingDirectCommands 모든 대기 중인 직접 명령 키 패턴
func AllPendingDirectCommands() string {
	return "pending_direct_command:*"
}

// AllStepActions 모든 단계 액션 키 패턴
func AllStepActions() string {
	return "step_actions:*"
}

// AllRobotStatuses 모든 로봇 상태 키 패턴
func AllRobotStatuses() string {
	return "robot_status:*"
}

// AllCommandExecutions 모든 명령 실행 키 패턴
func AllCommandExecutions() string {
	return "command_execution:*"
}

// AllSessions 모든 세션 키 패턴
func AllSessions() string {
	return "session:*"
}

// KeyType 키 타입 정의
type KeyType string

const (
	KeyTypePendingDirectCommand KeyType = "pending_direct_command"
	KeyTypeStepActions          KeyType = "step_actions"
	KeyTypeRobotStatus          KeyType = "robot_status"
	KeyTypeCommandExecution     KeyType = "command_execution"
	KeyTypeSession              KeyType = "session"
)

// GetKeyType 키에서 타입 추출
func GetKeyType(key string) KeyType {
	if len(key) == 0 {
		return ""
	}

	switch {
	case key[:len("pending_direct_command")] == "pending_direct_command":
		return KeyTypePendingDirectCommand
	case key[:len("step_actions")] == "step_actions":
		return KeyTypeStepActions
	case key[:len("robot_status")] == "robot_status":
		return KeyTypeRobotStatus
	case key[:len("command_execution")] == "command_execution":
		return KeyTypeCommandExecution
	case key[:len("session")] == "session":
		return KeyTypeSession
	default:
		return ""
	}
}
