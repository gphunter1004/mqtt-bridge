// internal/command/interfaces.go (수정됨)
package command

import (
	"mqtt-bridge/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// CommandHandler는 PLC 명령 및 로봇 상태를 처리하는 최상위 인터페이스
type CommandHandler interface {
	HandlePLCCommand(client mqtt.Client, msg mqtt.Message)
	HandleRobotStateUpdate(stateMsg *models.RobotStateMessage)
	FailAllProcessingCommands(reason string)
	FinishCommand(commandID uint, success bool)
}

// WorkflowExecutor는 워크플로우 실행을 담당하는 인터페이스
type WorkflowExecutor interface {
	// 인자를 *models.CommandExecution에서 다시 *models.Command로 변경
	ExecuteCommandOrder(command *models.Command) error
	SendDirectActionOrder(baseCommand string, commandType rune, armParam string) (string, error)
	CancelAllRunningOrders() error
}

// RobotStatusChecker는 로봇의 온라인 상태를 확인하는 인터페이스
type RobotStatusChecker interface {
	IsOnline(serialNumber string) bool
}
