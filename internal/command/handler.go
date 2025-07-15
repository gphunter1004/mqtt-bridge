// internal/command/handler.go
package command

import (
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/messaging"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"
)

// Handler PLC 명령 처리 핸들러
type Handler struct {
	db        *gorm.DB
	config    *config.Config
	processor *Processor
	plcSender *messaging.PLCResponseSender
}

// HandleRobotStateUpdate 로봇 상태 업데이트 처리
func (h *Handler) HandleRobotStateUpdate(stateMsg *models.RobotStateMessage) {
	utils.Logger.Debugf("🔍 COMMAND HANDLER: HandleRobotStateUpdate called")
	utils.Logger.Debugf("🔍 State message: OrderID=%s, ActionStates=%d",
		stateMsg.OrderID, len(stateMsg.ActionStates))

	// 액션 상태 상세 로깅
	for i, action := range stateMsg.ActionStates {
		utils.Logger.Debugf("🔍 Action[%d]: ID=%s, Type=%s, Status=%s, Description=%s",
			i, action.ActionID, action.ActionType, action.ActionStatus, action.ActionDescription)
	}

	// 1. 직접 명령 완료 확인 및 처리
	result := h.processor.HandleDirectCommandStateUpdate(stateMsg)
	if result != nil {
		utils.Logger.Infof("📤 COMMAND HANDLER: Direct command result found: %s:%s",
			result.Command, result.Status)
		h.SendResponseToPLC(*result)
	} else {
		utils.Logger.Debugf("🔍 COMMAND HANDLER: No direct command result for OrderID: %s", stateMsg.OrderID)
	}

	// 2. 🔥 새로운 기능: RUNNING 상태 전송
	h.handleRunningStateNotification(stateMsg)
}

// 🔥 새로운 메서드: RUNNING 상태 알림 처리
func (h *Handler) handleRunningStateNotification(stateMsg *models.RobotStateMessage) {
	if stateMsg.OrderID == "" {
		return
	}

	// 현재 실행 중인 명령이 있는지 확인
	var orderExecution models.OrderExecution
	if err := h.db.Preload("CommandExecution.Command.CommandDefinition").
		Where("order_id = ? AND status = ?", stateMsg.OrderID, constants.OrderExecutionStatusRunning).
		First(&orderExecution).Error; err != nil {
		// 실행 중인 오더가 없으면 무시
		return
	}

	// 액션 상태 확인
	hasRunningAction := false
	for _, action := range stateMsg.ActionStates {
		if action.ActionStatus == constants.ActionStatusRunning {
			hasRunningAction = true
			break
		}
	}

	// RUNNING 액션이 있고, 아직 RUNNING 상태 알림을 보내지 않았다면 전송
	if hasRunningAction {
		commandType := orderExecution.CommandExecution.Command.CommandDefinition.CommandType

		// 🔥 중복 전송 방지: 이미 RUNNING 상태를 보냈는지 확인
		if !h.hasRunningStatusSent(orderExecution.ID) {
			utils.Logger.Infof("📤 Sending RUNNING status to PLC: %s:%s", commandType, constants.StatusRunning)

			if err := h.plcSender.SendResponse(commandType, constants.StatusRunning, "Command is now running"); err != nil {
				utils.Logger.Errorf("❌ Failed to send RUNNING status to PLC: %v", err)
			} else {
				utils.Logger.Infof("✅ RUNNING status sent to PLC: %s:%s", commandType, constants.StatusRunning)
				h.markRunningStatusSent(orderExecution.ID)
			}
		}
	}
}

// 🔥 RUNNING 상태 전송 여부 확인 (간단한 메모리 기반 구현)
var runningStatusSent = make(map[uint]bool)

func (h *Handler) hasRunningStatusSent(orderExecutionID uint) bool {
	return runningStatusSent[orderExecutionID]
}

func (h *Handler) markRunningStatusSent(orderExecutionID uint) {
	runningStatusSent[orderExecutionID] = true
}

// 🔥 오더 완료 시 메모리 정리 (대문자로 변경 - 인터페이스 구현)
func (h *Handler) ClearRunningStatusFlag(orderExecutionID uint) {
	delete(runningStatusSent, orderExecutionID)
}

// HandlePLCCommand PLC 명령 수신 처리 (로깅 강화)
func (h *Handler) HandlePLCCommand(client mqtt.Client, msg mqtt.Message) {
	utils.Logger.Infof("🎯 COMMAND HANDLER: PLC Command received")
	utils.Logger.Infof("📨 RAW COMMAND: %s (Topic: %s, QoS: %d)",
		string(msg.Payload()), msg.Topic(), msg.Qos())

	commandStr := strings.TrimSpace(string(msg.Payload()))
	utils.Logger.Infof("🔧 Processing command: '%s'", commandStr)

	// 명령 타입 확인
	if h.isDirectActionCommand(commandStr) {
		utils.Logger.Infof("⚡ Direct action command detected: %s", commandStr)
		h.handleDirectActionCommand(commandStr)
		return
	}

	utils.Logger.Infof("📋 Standard command detected: %s", commandStr)
	h.handleStandardCommand(commandStr)
}

// handleStandardCommand 표준 명령 처리
func (h *Handler) handleStandardCommand(commandStr string) {
	utils.Logger.Infof("📋 Processing standard command: %s", commandStr)

	// 명령 정의 조회
	var cmdDef models.CommandDefinition
	if err := h.db.Where("command_type = ? AND is_active = true", commandStr).First(&cmdDef).Error; err != nil {
		utils.Logger.Errorf("❌ Command definition not found: %s (%v)", commandStr, err)
		result := CommandResult{
			Command:   commandStr,
			Status:    constants.StatusFailure,
			Message:   fmt.Sprintf("Command '%s' not defined or inactive", commandStr),
			Timestamp: time.Now(),
		}
		h.SendResponseToPLC(result)
		return
	}

	utils.Logger.Infof("✅ Command definition found: ID=%d, Type=%s, Description=%s",
		cmdDef.ID, cmdDef.CommandType, cmdDef.Description)

	// DB에 명령 기록
	command := &models.Command{
		CommandDefinitionID: cmdDef.ID,
		Status:              constants.CommandStatusPending,
		RequestTime:         time.Now(),
	}
	if err := h.db.Create(command).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to create command record: %v", err)
		result := CommandResult{
			Command:   commandStr,
			Status:    constants.StatusFailure,
			Message:   "Failed to create command record",
			Timestamp: time.Now(),
		}
		h.SendResponseToPLC(result)
		return
	}

	utils.Logger.Infof("📝 Command record created: ID=%d, Status=%s", command.ID, command.Status)

	// 비동기로 처리
	go func() {
		utils.Logger.Infof("🚀 Starting async processing for command ID=%d", command.ID)

		result, err := h.processor.ProcessStandardCommand(command)
		if err != nil {
			utils.Logger.Errorf("❌ Error processing standard command ID=%d: %v", command.ID, err)
		}

		// 🔥 중요: CR 명령의 경우 즉시 응답하지 않고 워크플로우 완료 대기
		if result != nil {
			if cmdDef.CommandType == constants.CommandOrderCancel {
				utils.Logger.Infof("📤 Sending immediate response for cancel command: %s:%s",
					result.Command, result.Status)
				h.SendResponseToPLC(*result)
			} else if result.Status == constants.StatusFailure {
				utils.Logger.Infof("📤 Sending immediate failure response: %s:%s",
					result.Command, result.Status)
				h.SendResponseToPLC(*result)
			} else {
				utils.Logger.Infof("⏳ Command started successfully, waiting for workflow completion: %s",
					result.Command)
				// 성공적으로 시작된 워크플로우는 완료 시 자동 응답됨
			}
		}
	}()
}

// handleDirectActionCommand 직접 액션 명령 처리 (로깅 강화)
func (h *Handler) handleDirectActionCommand(commandStr string) {
	utils.Logger.Infof("⚡ Processing direct action command: %s", commandStr)

	parts := strings.Split(commandStr, ":")
	if len(parts) < 2 {
		utils.Logger.Errorf("❌ Invalid direct action format: %s", commandStr)
		result := CommandResult{
			Command:   commandStr,
			Status:    constants.StatusFailure,
			Message:   "Invalid command format",
			Timestamp: time.Now(),
		}
		h.SendResponseToPLC(result)
		return
	}

	baseCommand := parts[0]
	commandType := rune(parts[1][0])

	var armParam string
	if commandType == constants.CommandTypeTrajectory && len(parts) >= 3 {
		armParam = parts[2]
	}

	utils.Logger.Infof("🔧 Parsed direct action: BaseCommand=%s, Type=%c, Arm=%s",
		baseCommand, commandType, armParam)

	req := DirectActionRequest{
		FullCommand: commandStr,
		BaseCommand: baseCommand,
		CommandType: commandType,
		ArmParam:    armParam,
		Timestamp:   time.Now(),
	}

	// 비동기로 처리
	go func() {
		utils.Logger.Infof("🚀 Starting async processing for direct action: %s", commandStr)

		result, err := h.processor.ProcessDirectAction(req)
		if err != nil {
			utils.Logger.Errorf("❌ Error processing direct action %s: %v", commandStr, err)
		}

		// 🔥 직접 액션은 에러만 즉시 응답, 성공은 state 기반 완료 대기
		if result != nil && result.Status == constants.StatusFailure {
			utils.Logger.Infof("📤 Sending direct action error response: %s:%s",
				result.Command, result.Status)
			h.SendResponseToPLC(*result)
		} else if result != nil && result.Status == constants.StatusSuccess {
			utils.Logger.Infof("✅ Direct action order sent successfully: %s (OrderID: %s) - Waiting for state completion",
				result.Command, result.OrderID)
		}
	}()
}

// SendResponseToPLC PLC에 응답 전송
func (h *Handler) SendResponseToPLC(result CommandResult) {
	utils.Logger.Infof("📤 SENDING PLC RESPONSE: %s:%s (%s)",
		result.Command, result.Status, result.Message)

	if err := h.plcSender.SendResponse(result.Command, result.Status, result.Message); err != nil {
		utils.Logger.Errorf("❌ Failed to send PLC response: %v", err)
	} else {
		utils.Logger.Infof("✅ PLC response sent successfully: %s:%s", result.Command, result.Status)
	}
}

// FailAllProcessingCommands 모든 처리 중인 명령 실패 처리
func (h *Handler) FailAllProcessingCommands(reason string) {
	utils.Logger.Warnf("⚠️ Failing all processing commands due to: %s", reason)

	// 직접 명령들 실패 처리
	results := h.processor.FailAllPendingCommands(reason)
	utils.Logger.Infof("📋 Found %d pending direct commands to fail", len(results))

	for i, result := range results {
		utils.Logger.Infof("📤 Failing direct command %d: %s:%s", i+1, result.Command, result.Status)
		h.SendResponseToPLC(result)
	}

	// 표준 명령들 실패 처리
	var executions []models.CommandExecution
	h.db.Where("status = ?", constants.CommandExecutionStatusRunning).
		Preload("Command.CommandDefinition").Find(&executions)

	utils.Logger.Infof("📋 Found %d running command executions to fail", len(executions))

	for i, execution := range executions {
		utils.Logger.Infof("📤 Failing command execution %d: %s", i+1, execution.Command.CommandDefinition.CommandType)
		if err := h.plcSender.SendFailure(execution.Command.CommandDefinition.CommandType, reason); err != nil {
			utils.Logger.Errorf("❌ Failed to send failure response for command %s: %v",
				execution.Command.CommandDefinition.CommandType, err)
		}

		// 🔥 실패 처리 시 RUNNING 상태 플래그 정리
		for _, orderExec := range execution.OrderExecutions {
			h.ClearRunningStatusFlag(orderExec.ID)
		}
	}

	utils.Logger.Infof("✅ All processing commands failed with reason: %s", reason)
}

// isDirectActionCommand 직접 액션 명령인지 확인
func (h *Handler) isDirectActionCommand(commandStr string) bool {
	isDirect := strings.HasSuffix(commandStr, ":I") || strings.Contains(commandStr, ":T")
	utils.Logger.Debugf("🔍 Is direct action command '%s': %t", commandStr, isDirect)
	return isDirect
}

// NewHandler 새 명령 핸들러 생성
func NewHandler(db *gorm.DB, cfg *config.Config, processor *Processor,
	plcSender *messaging.PLCResponseSender) *Handler {

	utils.Logger.Infof("🏗️ CREATING Command Handler")

	handler := &Handler{
		db:        db,
		config:    cfg,
		processor: processor,
		plcSender: plcSender,
	}

	utils.Logger.Infof("✅ Command Handler CREATED")
	return handler
}
