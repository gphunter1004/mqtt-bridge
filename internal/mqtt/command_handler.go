// internal/mqtt/command_handler.go (최종 수정본)
package mqtt

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/repository" // (수정) repository 패키지 import
	"mqtt-bridge/internal/utils"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type CommandHandler struct {
	db              *gorm.DB
	redisClient     *redis.Client
	mqttClient      mqtt.Client
	config          *config.Config
	orderExecutor   *OrderExecutor
	processingMutex sync.Mutex
	isProcessing    bool
}

func NewCommandHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *CommandHandler {
	return &CommandHandler{
		db:            db,
		redisClient:   redisClient,
		mqttClient:    mqttClient,
		config:        cfg,
		orderExecutor: NewOrderExecutor(db, redisClient, mqttClient, cfg),
	}
}

// HandleCommand PLC 명령 처리
func (h *CommandHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	commandStr := strings.TrimSpace(string(msg.Payload()))
	utils.Logger.Infof("Received command from PLC: %s", commandStr)

	var cmdDef models.CommandDefinition
	if err := h.db.Where("command_type = ? AND is_active = true", commandStr).First(&cmdDef).Error; err != nil {
		errMsg := fmt.Sprintf("Command '%s' not defined or inactive", commandStr)
		utils.Logger.Errorf(errMsg)
		h.orderExecutor.sendFinalResponseToPLC(commandStr, "F", errMsg)
		return
	}

	if cmdDef.CommandType == models.CommandOrderCancel {
		utils.Logger.Infof("Processing 'OC' (Order Cancel) command")
		if err := h.orderExecutor.CancelAllRunningOrders(); err != nil {
			h.orderExecutor.sendFinalResponseToPLC(cmdDef.CommandType, "F", err.Error())
		} else {
			// OC 명령은 성공 시 별도 응답 없음 (CancelAllRunningOrders에서 이미 실패 응답 전송)
			h.orderExecutor.sendFinalResponseToPLC(cmdDef.CommandType, "S", "")
		}
		return
	}

	h.processingMutex.Lock()
	if h.isProcessing {
		h.processingMutex.Unlock()
		errMsg := "Command rejected: Another command is currently processing"
		utils.Logger.Warnf("Command %s rejected: %s", commandStr, errMsg)
		h.orderExecutor.sendFinalResponseToPLC(commandStr, "R", errMsg) // R for Rejected
		now := time.Now()
		command := &models.Command{
			CommandDefinitionID: cmdDef.ID,
			Status:              models.StatusRejected,
			RequestTime:         now,
			ResponseTime:        &now,
			ErrorMessage:        errMsg,
		}
		h.db.Create(command)
		return
	}
	h.isProcessing = true
	h.processingMutex.Unlock()

	command := &models.Command{
		CommandDefinitionID: cmdDef.ID,
		Status:              models.StatusPending,
		RequestTime:         time.Now(),
	}
	h.db.Create(command)
	utils.Logger.Infof("Command accepted: ID=%d, Type=%s", command.ID, cmdDef.CommandType)

	go h.processCommand(command)
}

// processCommand 실제 명령 처리 로직 (비동기 실행)
func (h *CommandHandler) processCommand(command *models.Command) {
	defer func() {
		h.processingMutex.Lock()
		h.isProcessing = false
		h.processingMutex.Unlock()
	}()

	var robotStatus models.RobotStatus
	h.db.Where("serial_number = ?", h.config.RobotSerialNumber).First(&robotStatus)
	if robotStatus.ConnectionState != models.ConnectionStateOnline {
		errMsg := "Robot is not online"
		repository.UpdateCommandStatus(h.db, command, models.StatusFailure, errMsg)
		h.orderExecutor.sendFinalResponseToPLC(command.CommandDefinition.CommandType, "F", errMsg)
		return
	}

	repository.UpdateCommandStatus(h.db, command, models.StatusProcessing, "")

	if err := h.orderExecutor.ExecuteCommandOrder(command); err != nil {
		errMsg := fmt.Sprintf("Failed to start command execution: %v", err)
		utils.Logger.Errorf(errMsg)
		repository.UpdateCommandStatus(h.db, command, models.StatusFailure, errMsg)
		h.orderExecutor.sendFinalResponseToPLC(command.CommandDefinition.CommandType, "F", errMsg)
	}
}

// FailAllProcessingCommands 외부 요인(로봇 연결 끊김 등)으로 모든 진행중인 명령을 실패처리
func (h *CommandHandler) FailAllProcessingCommands(reason string) {
	h.processingMutex.Lock()
	defer h.processingMutex.Unlock()

	if !h.isProcessing {
		return
	}

	// 현재 실행 중인 CommandExecution을 찾아서 실패 처리
	var execution models.CommandExecution
	if err := h.db.Where("status = ?", models.CommandExecutionStatusRunning).Preload("Command.CommandDefinition").First(&execution).Error; err == nil {
		now := time.Now()
		repository.UpdateCommandExecutionStatus(h.db, &execution, models.CommandExecutionStatusFailed, &now)
		repository.UpdateCommandStatus(h.db, &execution.Command, models.StatusFailure, reason)
		h.orderExecutor.sendFinalResponseToPLC(execution.Command.CommandDefinition.CommandType, "F", reason)
	}

	h.isProcessing = false
}
