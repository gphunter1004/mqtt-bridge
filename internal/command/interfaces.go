// internal/command/interfaces.go (새로운 파일)
package command

import "mqtt-bridge/internal/models"

// CommandHandler 인터페이스 정의 (순환 참조 방지용)
type CommandHandler interface {
	// RUNNING 상태 플래그 관리
	ClearRunningStatusFlag(orderExecutionID uint)

	// 기존 메서드들
	FailAllProcessingCommands(reason string)
	HandleRobotStateUpdate(stateMsg *models.RobotStateMessage)
}

// 구현 확인을 위한 컴파일 타임 검증
var _ CommandHandler = (*Handler)(nil)
