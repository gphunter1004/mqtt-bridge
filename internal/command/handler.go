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

// HandlePLCCommand PLC 명령 수신 처리
func (h *Handler) HandlePLCCommand(client mqtt.Client, msg mqtt.Message) {
	utils.Logger.Infof("🎯 COMMAND HANDLER CALLED")
	utils.Logger.Infof("📨 RAW COMMAND: %s (MessageID: %d, QoS: %d)",
		string(msg.Payload()), msg.MessageID(), msg.Qos())
	utils.Logger.Infof("🕒 TIMESTAMP: %s", time.Now().Format("2006-01-02 15:04:05.000"))

	commandStr := strings.TrimSpace(string(msg.Payload()))
	utils.Logger.Infof("Received command from PLC: %s", commandStr)

	// Direct Action 명령인지 확인 (:I 또는 :T 포함)
	if h.isDirectActionCommand(commandStr) {
		h.handleDirectActionCommand(commandStr)
		return
	}

	// 표준 명령 처리 (CR, GR, OC 등)
	h.handleStandardCommand(commandStr)
}

// HandleRobotStateUpdate 로봇 상태 업데이트 처리 (직접 명령 완료 확인용)
func (h *Handler) HandleRobotStateUpdate(stateMsg *models.RobotStateMessage) {
	result := h.processor.HandleDirectCommandStateUpdate(stateMsg)
	if result != nil {
		// 🔥 수정: 성공/실패 모든 경우에 PLC에 응답 전송
		utils.Logger.Infof("📤 Sending direct command result to PLC: %s:%s",
			result.Command, result.Status)
		h.SendResponseToPLC(*result)
	}
}

// SendResponseToPLC PLC에 응답 전송
func (h *Handler) SendResponseToPLC(result CommandResult) {
	if err := h.plcSender.SendResponse(result.Command, result.Status, result.Message); err != nil {
		utils.Logger.Errorf("Failed to send PLC response: %v", err)
	}
}

// FailAllProcessingCommands 모든 처리 중인 명령 실패 처리 (로봇 연결 끊김 등)
func (h *Handler) FailAllProcessingCommands(reason string) {
	// 직접 명령들 실패 처리
	results := h.processor.FailAllPendingCommands(reason)
	for _, result := range results {
		h.SendResponseToPLC(result)
	}

	// 표준 명령들 실패 처리
	var executions []models.CommandExecution
	h.db.Where("status = ?", constants.CommandExecutionStatusRunning).
		Preload("Command.CommandDefinition").Find(&executions)

	for _, execution := range executions {
		if err := h.plcSender.SendFailure(execution.Command.CommandDefinition.CommandType, reason); err != nil {
			utils.Logger.Errorf("Failed to send failure response for command %s: %v",
				execution.Command.CommandDefinition.CommandType, err)
		}
	}
}

// isDirectActionCommand 직접 액션 명령인지 확인
func (h *Handler) isDirectActionCommand(commandStr string) bool {
	return strings.HasSuffix(commandStr, ":I") || strings.Contains(commandStr, ":T")
}

// handleDirectActionCommand 직접 액션 명령 처리
func (h *Handler) handleDirectActionCommand(commandStr string) {
	parts := strings.Split(commandStr, ":")
	if len(parts) < 2 {
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

	req := DirectActionRequest{
		FullCommand: commandStr,
		BaseCommand: baseCommand,
		CommandType: commandType,
		ArmParam:    armParam,
		Timestamp:   time.Now(),
	}

	// 비동기로 처리 (여러 명령 동시 처리 허용)
	go func() {
		result, err := h.processor.ProcessDirectAction(req)
		if err != nil {
			utils.Logger.Errorf("Error processing direct action: %v", err)
		}

		// 🔥 수정: 직접 액션은 즉시 응답하지 않음 (state 기반 완료 대기)
		// 에러가 발생한 경우에만 즉시 응답
		if result != nil && result.Status == constants.StatusFailure {
			utils.Logger.Infof("📤 Sending direct action error to PLC: %s:%s",
				result.Command, result.Status)
			h.SendResponseToPLC(*result)
		} else if result != nil && result.Status == constants.StatusSuccess {
			// 성공적으로 전송된 경우 로그만 남기고 state 완료 대기
			utils.Logger.Infof("✅ Direct action order sent successfully: %s (OrderID: %s) - Waiting for completion via state message",
				result.Command, result.OrderID)
		}
	}()
}

// handleStandardCommand 표준 명령 처리
func (h *Handler) handleStandardCommand(commandStr string) {
	// 명령 정의 조회
	var cmdDef models.CommandDefinition
	if err := h.db.Where("command_type = ? AND is_active = true", commandStr).First(&cmdDef).Error; err != nil {
		result := CommandResult{
			Command:   commandStr,
			Status:    constants.StatusFailure,
			Message:   fmt.Sprintf("Command '%s' not defined or inactive", commandStr),
			Timestamp: time.Now(),
		}
		h.SendResponseToPLC(result)
		return
	}

	// DB에 명령 기록
	command := &models.Command{
		CommandDefinitionID: cmdDef.ID,
		Status:              constants.CommandStatusPending,
		RequestTime:         time.Now(),
	}
	h.db.Create(command)
	utils.Logger.Infof("Command accepted: ID=%d, Type=%s", command.ID, cmdDef.CommandType)

	// 비동기로 처리
	go func() {
		result, err := h.processor.ProcessStandardCommand(command)
		if err != nil {
			utils.Logger.Errorf("Error processing standard command: %v", err)
		}

		// 취소 명령은 즉시 응답, 나머지는 워크플로우 완료 후 응답
		if result != nil && (cmdDef.CommandType == constants.CommandOrderCancel || result.Status != constants.StatusSuccess) {
			h.SendResponseToPLC(*result)
		}
	}()
}
