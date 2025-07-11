// internal/mqtt/command_handler.go (최종 수정본)
package mqtt

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
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

	// 1. 명령 정의 조회
	var cmdDef models.CommandDefinition
	if err := h.db.Where("command_type = ? AND is_active = true", commandStr).First(&cmdDef).Error; err != nil {
		errMsg := fmt.Sprintf("Command '%s' not defined or inactive", commandStr)
		utils.Logger.Errorf(errMsg)
		h.orderExecutor.sendFinalResponseToPLC(commandStr, "F", errMsg)
		return
	}

	// 2. 명령 취소(OC) 특별 처리
	if cmdDef.CommandType == "OC" {
		utils.Logger.Infof("Processing 'OC' (Order Cancel) command")
		if err := h.orderExecutor.CancelAllRunningOrders(); err != nil {
			h.orderExecutor.sendFinalResponseToPLC(cmdDef.CommandType, "F", err.Error())
		} else {
			h.orderExecutor.sendFinalResponseToPLC(cmdDef.CommandType, "S", "")
		}
		return
	}

	// 3. 동시 실행 방지
	h.processingMutex.Lock()
	if h.isProcessing {
		h.processingMutex.Unlock()
		errMsg := "Command rejected: Another command is currently processing"
		utils.Logger.Warnf("Command %s rejected: %s", commandStr, errMsg)
		h.orderExecutor.sendFinalResponseToPLC(commandStr, "R", errMsg) // R for Rejected

		now := time.Now()
		h.db.Create(&models.Command{
			CommandDefinitionID: cmdDef.ID,
			Status:              "REJECTED",
			RequestTime:         now,
			ResponseTime:        &now,
			ErrorMessage:        errMsg,
		})
		return
	}
	h.isProcessing = true
	h.processingMutex.Unlock()

	// 4. 명령 처리 시작
	command := &models.Command{
		CommandDefinitionID: cmdDef.ID,
		Status:              "PENDING",
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
	if robotStatus.ConnectionState != "ONLINE" { // VDA5050 스펙에 따라 CONNECTED -> ONLINE
		errMsg := "Robot is not online"
		h.updateCommandStatus(command, "FAILURE", errMsg)
		h.orderExecutor.sendFinalResponseToPLC(command.CommandDefinition.CommandType, "F", errMsg)
		return
	}

	h.updateCommandStatus(command, "PROCESSING", "")

	if err := h.orderExecutor.ExecuteCommandOrder(command); err != nil {
		errMsg := fmt.Sprintf("Failed to start command execution: %v", err)
		utils.Logger.Errorf(errMsg)
		h.updateCommandStatus(command, "FAILURE", errMsg)
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
		execution.Status = models.CommandExecutionStatusFailed
		execution.CompletedAt = &now
		h.db.Save(&execution)

		h.updateCommandStatus(&execution.Command, "FAILURE", reason)
		h.orderExecutor.sendFinalResponseToPLC(execution.Command.CommandDefinition.CommandType, "F", reason)
	}

	h.isProcessing = false
}

// updateCommandStatus 명령 상태 업데이트
func (h *CommandHandler) updateCommandStatus(command *models.Command, status, errMsg string) {
	command.Status = status
	if errMsg != "" {
		command.ErrorMessage = errMsg
	}
	if status == "SUCCESS" || status == "FAILURE" || status == "REJECTED" {
		now := time.Now()
		command.ResponseTime = &now
	}
	h.db.Save(command)
	utils.Logger.Infof("Command %d status changed to %s", command.ID, status)
}
