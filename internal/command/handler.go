// internal/command/handler.go (수정됨: 단일 FSM 생성자 사용)
package command

import (
	"context"
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/messaging"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"
)

// Handler는 PLC 명령을 수신하여 적절한 상태 머신을 생성하고 관리합니다.
type Handler struct {
	db               *gorm.DB
	config           *config.Config
	plcSender        *messaging.PLCResponseSender
	workflowExecutor WorkflowExecutor
	robotChecker     RobotStatusChecker

	activeFSMs map[string]*CommandStateMachine
	mu         sync.Mutex
}

// NewHandler는 새 명령 핸들러를 생성합니다.
func NewHandler(
	db *gorm.DB,
	cfg *config.Config,
	plcSender *messaging.PLCResponseSender,
	executor WorkflowExecutor,
	robotChecker RobotStatusChecker,
) *Handler {
	utils.Logger.Infof("🏗️ CREATING Command Handler (State Machine Enabled)")
	return &Handler{
		db:               db,
		config:           cfg,
		plcSender:        plcSender,
		workflowExecutor: executor,
		robotChecker:     robotChecker,
		activeFSMs:       make(map[string]*CommandStateMachine),
	}
}

// HandlePLCCommand는 PLC 명령을 받아 표준 또는 직접 액션 FSM을 생성합니다.
func (h *Handler) HandlePLCCommand(client mqtt.Client, msg mqtt.Message) {
	commandStr := strings.TrimSpace(string(msg.Payload()))
	utils.Logger.Infof("🎯 PLC Command received: '%s'", commandStr)

	if !h.robotChecker.IsOnline(h.config.RobotSerialNumber) {
		utils.Logger.Errorf("❌ Robot is offline. Rejecting command: %s", commandStr)
		h.plcSender.SendFailure(commandStr, "Robot is not online")
		return
	}

	if IsDirectActionCommand(commandStr) {
		h.handleDirectAction(commandStr)
	} else {
		h.handleStandardCommand(commandStr)
	}
}

func (h *Handler) handleStandardCommand(commandStr string) {
	var cmdDef models.CommandDefinition
	if err := h.db.Where("command_type = ? AND is_active = true", commandStr).First(&cmdDef).Error; err != nil {
		utils.Logger.Errorf("❌ Command definition not found: %s", commandStr)
		h.plcSender.SendFailure(commandStr, "Command not defined or inactive")
		return
	}

	command := &models.Command{
		CommandDefinitionID: cmdDef.ID,
		Status:              constants.CommandStatusPending,
		RequestTime:         time.Now(),
	}
	if err := h.db.Create(command).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to create command record: %v", err)
		h.plcSender.SendFailure(commandStr, "Failed to record command")
		return
	}
	h.db.Preload("CommandDefinition").First(&command, command.ID)

	csm := NewCommandStateMachine(h.db, h.plcSender, h.workflowExecutor).ForStandardCommand(command)
	h.addStateMachine(fmt.Sprintf("std-%d", command.ID), csm)

	if err := csm.StartWorkflow(); err != nil {
		utils.Logger.Errorf("❌ Error starting workflow for Command ID %d: %v", command.ID, err)
		h.removeStateMachine(fmt.Sprintf("std-%d", command.ID))
	}
}

func (h *Handler) handleDirectAction(commandStr string) {
	parts := strings.Split(commandStr, ":")
	baseCommand, cmdType, armParam := parts[0], rune(parts[1][0]), ""
	if len(parts) >= 3 {
		armParam = parts[2]
	}

	orderID, err := h.workflowExecutor.SendDirectActionOrder(baseCommand, cmdType, armParam)
	if err != nil {
		utils.Logger.Errorf("❌ Failed to send direct action order: %v", err)
		h.plcSender.SendFailure(commandStr, "Failed to send order to robot")
		return
	}

	csm := NewCommandStateMachine(h.db, h.plcSender, h.workflowExecutor).ForDirectAction(commandStr, orderID)
	h.addStateMachine(orderID, csm)
}

// HandleRobotStateUpdate는 state 메시지를 적절한 FSM에 전달합니다.
func (h *Handler) HandleRobotStateUpdate(stateMsg *models.RobotStateMessage) {
	if stateMsg.OrderID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	var targetKey string
	var targetFsm *CommandStateMachine

	if csm, exists := h.activeFSMs[stateMsg.OrderID]; exists {
		targetKey = stateMsg.OrderID
		targetFsm = csm
	} else {
		for key, csm := range h.activeFSMs {
			if csm.IsRelevantOrder(stateMsg.OrderID) {
				targetKey = key
				targetFsm = csm
				break
			}
		}
	}

	if targetFsm != nil {
		targetFsm.HandleRobotStateUpdate(stateMsg)
		if targetFsm.IsDirectAction && (targetFsm.FSM.Is("Completed") || targetFsm.FSM.Is("Failed")) {
			delete(h.activeFSMs, targetKey)
			utils.Logger.Infof("Direct action FSM for order %s has been finalized and removed.", targetKey)
		}
	}
}

// FinishCommand는 Executor가 호출하여 FSM을 최종 완료 상태로 만듭니다.
func (h *Handler) FinishCommand(commandID uint, success bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	fsmKey := fmt.Sprintf("std-%d", commandID)
	if csm, exists := h.activeFSMs[fsmKey]; exists {
		if success {
			csm.FSM.Event(context.Background(), "command_succeeded")
		} else {
			csm.Fail("Command execution failed by executor")
		}
		delete(h.activeFSMs, fsmKey)
		utils.Logger.Infof("FSM for command %d has been finalized and removed.", commandID)
	}
}

// FailAllProcessingCommands는 모든 활성 FSM에 실패 이벤트를 전송합니다.
func (h *Handler) FailAllProcessingCommands(reason string) {
	utils.Logger.Warnf("⚠️ Failing all processing commands due to: %s", reason)
	h.mu.Lock()
	keysToRemove := []string{}
	for key, csm := range h.activeFSMs {
		csm.Fail(reason)
		keysToRemove = append(keysToRemove, key)
	}
	h.mu.Unlock()

	for _, key := range keysToRemove {
		h.removeStateMachine(key)
	}
}

func (h *Handler) addStateMachine(key string, csm *CommandStateMachine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activeFSMs[key] = csm
}

func (h *Handler) removeStateMachine(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.activeFSMs, key)
}

func IsDirectActionCommand(commandStr string) bool {
	return strings.HasSuffix(commandStr, ":I") || strings.Contains(commandStr, ":T")
}
