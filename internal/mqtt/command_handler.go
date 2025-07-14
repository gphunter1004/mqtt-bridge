// internal/mqtt/command_handler.go (동시 처리 방지 로직 제거)
package mqtt

import (
	"context"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/repository"
	"mqtt-bridge/internal/utils"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type CommandHandler struct {
	db                  *gorm.DB
	redisClient         *redis.Client
	mqttClient          mqtt.Client
	config              *config.Config
	orderExecutor       *OrderExecutor
	orderMessageHandler *OrderMessageHandler
}

func NewCommandHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *CommandHandler {
	utils.Logger.Infof("🏗️ CREATING CommandHandler: %p", &CommandHandler{})

	orderExecutor := NewOrderExecutor(db, redisClient, mqttClient, cfg)
	handler := &CommandHandler{
		db:                  db,
		redisClient:         redisClient,
		mqttClient:          mqttClient,
		config:              cfg,
		orderExecutor:       orderExecutor,
		orderMessageHandler: orderExecutor.orderMessageHandler,
	}

	utils.Logger.Infof("✅ CommandHandler CREATED: %p", handler)
	return handler
}

// HandleCommand PLC 명령 처리
func (h *CommandHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	utils.Logger.Infof("🎯 COMMAND HANDLER CALLED: %p", h)
	utils.Logger.Infof("📨 RAW COMMAND: %s (MessageID: %d, QoS: %d)", string(msg.Payload()), msg.MessageID(), msg.Qos())
	utils.Logger.Infof("🕒 TIMESTAMP: %s", time.Now().Format("2006-01-02 15:04:05.000"))

	commandStr := strings.TrimSpace(string(msg.Payload()))
	utils.Logger.Infof("Received command from PLC: %s", commandStr)

	// :I 또는 :T 접미사 확인 (Direct Action 명령)
	if strings.HasSuffix(commandStr, ":I") || strings.Contains(commandStr, ":T") {
		parts := strings.Split(commandStr, ":")
		if len(parts) >= 2 {
			baseCommand := parts[0]
			commandType := rune(parts[1][0])

			var armParam string = ""
			if commandType == 'T' && len(parts) >= 3 {
				// :T:R 또는 :T:L 형태
				armParam = parts[2]
			}

			utils.Logger.Infof("Processing direct action command: %s, Type: %c, Arm: %s", baseCommand, commandType, armParam)

			// 직접 액션 명령 처리
			go h.processDirectActionCommand(commandStr, baseCommand, commandType, armParam)
			return
		}
	}

	// 기존 명령 처리 로직 (CR, GR, OC 등)
	var cmdDef models.CommandDefinition
	if err := h.db.Where("command_type = ? AND is_active = true", commandStr).First(&cmdDef).Error; err != nil {
		errMsg := fmt.Sprintf("Command '%s' not defined or inactive", commandStr)
		utils.Logger.Errorf(errMsg)
		h.sendResponseToPLC(commandStr, "F", errMsg)
		return
	}

	if cmdDef.CommandType == models.CommandOrderCancel {
		utils.Logger.Infof("Processing 'OC' (Order Cancel) command")
		if err := h.orderExecutor.CancelAllRunningOrders(); err != nil {
			h.sendResponseToPLC(cmdDef.CommandType, "F", err.Error())
		} else {
			h.sendResponseToPLC(cmdDef.CommandType, "S", "")
		}
		return
	}

	command := &models.Command{
		CommandDefinitionID: cmdDef.ID,
		Status:              models.StatusPending,
		RequestTime:         time.Now(),
	}
	h.db.Create(command)
	utils.Logger.Infof("Command accepted: ID=%d, Type=%s", command.ID, cmdDef.CommandType)

	go h.processCommand(command)
}

// processDirectActionCommand 직접 액션 명령 처리 (이벤트 기반)
func (h *CommandHandler) processDirectActionCommand(fullCommand, baseCommand string, commandType rune, armParam string) {
	// 로봇 온라인 상태 확인
	var robotStatus models.RobotStatus
	h.db.Where("serial_number = ?", h.config.RobotSerialNumber).First(&robotStatus)
	if robotStatus.ConnectionState != models.ConnectionStateOnline {
		errMsg := "Robot is not online"
		utils.Logger.Errorf(errMsg)
		h.sendResponseToPLC(fullCommand, "F", errMsg)
		return
	}

	// 오더 전송
	orderID, err := h.orderMessageHandler.SendDirectActionOrder(baseCommand, commandType, armParam)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to send direct action order: %v", err)
		utils.Logger.Errorf(errMsg)
		h.sendResponseToPLC(fullCommand, "F", errMsg)
		return
	}

	utils.Logger.Infof("Direct action order sent successfully: %s (OrderID: %s)", fullCommand, orderID)

	// Redis에 명령 상태 저장 (state 기반 응답 대기용)
	h.storePendingDirectCommand(fullCommand, orderID)
}

// processCommand 실제 명령 처리 로직 (비동기 실행)
func (h *CommandHandler) processCommand(command *models.Command) {
	var robotStatus models.RobotStatus
	// Preload CommandDefinition
	h.db.Preload("CommandDefinition").First(&command, command.ID)
	h.db.Where("serial_number = ?", h.config.RobotSerialNumber).First(&robotStatus)
	if robotStatus.ConnectionState != models.ConnectionStateOnline {
		errMsg := "Robot is not online"
		repository.UpdateCommandStatus(h.db, command, models.StatusFailure, errMsg)
		h.sendResponseToPLC(command.CommandDefinition.CommandType, "F", errMsg)
		return
	}

	repository.UpdateCommandStatus(h.db, command, models.StatusProcessing, "")

	if err := h.orderExecutor.ExecuteCommandOrder(command); err != nil {
		errMsg := fmt.Sprintf("Failed to start command execution: %v", err)
		utils.Logger.Errorf(errMsg)
		repository.UpdateCommandStatus(h.db, command, models.StatusFailure, errMsg)
		h.sendResponseToPLC(command.CommandDefinition.CommandType, "F", errMsg)
	}
}

// storePendingDirectCommand Redis에 대기 중인 직접 명령 저장
func (h *CommandHandler) storePendingDirectCommand(fullCommand, orderID string) {
	ctx := context.Background()
	key := fmt.Sprintf("pending_direct_command:%s", orderID)

	commandData := map[string]interface{}{
		"full_command": fullCommand,
		"order_id":     orderID,
		"timestamp":    time.Now().Unix(),
	}

	// TTL 없이 저장 (이벤트 기반으로만 제거)
	h.redisClient.HMSet(ctx, key, commandData)

	utils.Logger.Infof("Stored pending direct command: %s -> %s", fullCommand, orderID)
}

// HandleDirectCommandStateUpdate state 메시지를 통한 직접 명령 결과 처리
func (h *CommandHandler) HandleDirectCommandStateUpdate(stateMsg *models.RobotStateMessage) {
	if stateMsg.OrderID == "" {
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("pending_direct_command:%s", stateMsg.OrderID)

	// Redis에서 대기 중인 명령 확인
	commandData, err := h.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(commandData) == 0 {
		return // 대기 중인 직접 명령이 아님
	}

	fullCommand := commandData["full_command"]
	if fullCommand == "" {
		return
	}

	// 액션 상태 확인
	result := h.determineDirectCommandResult(stateMsg.ActionStates)

	if result != "" {
		// 결과가 확정되면 Redis에서 제거하고 PLC에 응답
		h.redisClient.Del(ctx, key)
		h.sendResponseToPLC(fullCommand, result, "")
		utils.Logger.Infof("Direct command completed: %s -> %s", fullCommand, result)
	}
}

// determineDirectCommandResult 액션 상태를 기반으로 명령 결과 결정
func (h *CommandHandler) determineDirectCommandResult(actionStates []models.ActionState) string {
	if len(actionStates) == 0 {
		return ""
	}

	allFinished := true
	hasFailure := false

	for _, action := range actionStates {
		switch action.ActionStatus {
		case models.ActionStatusFailed:
			hasFailure = true
		case models.ActionStatusFinished:
			continue
		default:
			allFinished = false
		}
	}

	if hasFailure {
		return "F"
	}

	if allFinished {
		return "S"
	}

	return "" // 아직 진행 중
}

// sendResponseToPLC PLC에 응답 전송
func (h *CommandHandler) sendResponseToPLC(command, status, errMsg string) {
	response := fmt.Sprintf("%s:%s", command, status)
	if status == "F" && errMsg != "" {
		utils.Logger.Errorf("Command %s failed: %s", command, errMsg)
	}

	topic := h.config.PlcResponseTopic
	utils.Logger.Infof("Sending response to PLC: %s", response)
	token := h.mqttClient.Publish(topic, 0, false, response)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())
	} else {
		utils.Logger.Infof("Response sent successfully to PLC: %s", response)
	}
}

// FailAllProcessingCommands 로봇 연결 끊김 등으로 모든 대기중인 명령을 실패처리
func (h *CommandHandler) FailAllProcessingCommands(reason string) {
	// Redis에서 대기 중인 직접 명령들 실패 처리
	ctx := context.Background()
	pattern := "pending_direct_command:*"
	keys, err := h.redisClient.Keys(ctx, pattern).Result()
	if err == nil {
		for _, key := range keys {
			commandData, err := h.redisClient.HGetAll(ctx, key).Result()
			if err == nil && len(commandData) > 0 {
				fullCommand := commandData["full_command"]
				if fullCommand != "" {
					h.sendResponseToPLC(fullCommand, "F", reason)
				}
				h.redisClient.Del(ctx, key)
			}
		}
	}

	// 현재 실행 중인 CommandExecution을 찾아서 실패 처리
	var executions []models.CommandExecution
	h.db.Where("status = ?", models.CommandExecutionStatusRunning).Preload("Command.CommandDefinition").Find(&executions)

	for _, execution := range executions {
		now := time.Now()
		repository.UpdateCommandExecutionStatus(h.db, &execution, models.CommandExecutionStatusFailed, &now)
		repository.UpdateCommandStatus(h.db, &execution.Command, models.StatusFailure, reason)
		h.sendResponseToPLC(execution.Command.CommandDefinition.CommandType, "F", reason)
	}
}
