// internal/mqtt/command_handler.go (업데이트된 버전)
package mqtt

import (
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type CommandHandler struct {
	db            *gorm.DB
	redisClient   *redis.Client
	mqttClient    mqtt.Client
	config        *config.Config
	orderExecutor *OrderExecutor
}

func NewCommandHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *CommandHandler {
	orderExecutor := NewOrderExecutor(db, redisClient, mqttClient, cfg)

	return &CommandHandler{
		db:            db,
		redisClient:   redisClient,
		mqttClient:    mqttClient,
		config:        cfg,
		orderExecutor: orderExecutor,
	}
}

// HandleCommand PLC에서 받은 명령 처리
func (h *CommandHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	commandType := strings.TrimSpace(string(msg.Payload()))

	utils.Logger.Infof("Received command from PLC: %s", commandType)

	// 명령 유효성 검사
	if !models.IsValidCommand(commandType) {
		utils.Logger.Errorf("Invalid command received: %s", commandType)
		return
	}

	// orderCancel 명령은 별도 처리
	if commandType == models.CommandOrderCancel {
		h.handleOrderCancelCommand()
		return
	}

	// 실행 중인 명령이 있는지 확인
	var runningCount int64
	h.db.Model(&models.CommandExecution{}).Where("status = ?", models.CommandExecutionStatusRunning).Count(&runningCount)

	if runningCount > 0 {
		utils.Logger.Warnf("Command %s rejected: Another command is currently processing", commandType)
		h.handleRejectedCommand(commandType)
		return
	}

	// 해당 명령에 매핑된 오더가 있는지 확인
	var mappingCount int64
	h.db.Model(&models.CommandOrderMapping{}).Where("command_type = ? AND is_active = ?", commandType, true).Count(&mappingCount)

	if mappingCount == 0 {
		utils.Logger.Errorf("No active order mappings found for command: %s", commandType)
		h.handleRejectedCommand(commandType)
		return
	}

	// 명령 정보를 데이터베이스에 저장
	command := &models.Command{
		CommandType: commandType,
		Status:      models.StatusPending,
		RequestTime: time.Now(),
	}

	if err := h.db.Create(command).Error; err != nil {
		utils.Logger.Errorf("Failed to save command to database: %v", err)
		return
	}

	utils.Logger.Infof("Command accepted: ID=%d, Type=%s (will execute %d orders)", command.ID, command.CommandType, mappingCount)

	// 명령 처리 시작
	go h.processCommand(command)
}

// processCommand 명령 처리
func (h *CommandHandler) processCommand(command *models.Command) {
	utils.Logger.Infof("Processing command: %s (ID: %d)", command.CommandType, command.ID)

	// 로봇 온라인 상태 확인
	if !h.isRobotOnline() {
		h.failCommand(command, "Robot is not online")
		return
	}

	// 상태를 처리중으로 변경
	command.Status = models.StatusProcessing
	h.updateCommand(command)

	utils.Logger.Infof("Command %d status changed to PROCESSING", command.ID)

	// 오더 실행
	if err := h.orderExecutor.ExecuteCommandOrder(command); err != nil {
		h.failCommand(command, err.Error())
		return
	}

	// 오더 실행이 시작되면 성공 응답
	command.Status = models.StatusSuccess
	now := time.Now()
	command.ResponseTime = &now
	h.updateCommand(command)

	utils.Logger.Infof("Command %d order execution started successfully", command.ID)

	// PLC에 응답 전송
	h.sendResponseToPLC(command.GetResponseCode())
}

// HandleOrderStateUpdate 로봇 상태 업데이트 처리 (OrderExecutor로 위임)
func (h *CommandHandler) HandleOrderStateUpdate(stateMsg *models.RobotStateMessage) {
	h.orderExecutor.HandleOrderStateUpdate(stateMsg)
}

// handleOrderCancelCommand orderCancel 명령 처리
func (h *CommandHandler) handleOrderCancelCommand() {
	utils.Logger.Infof("Processing orderCancel command (OC)")

	// orderCancel 명령을 DB에 기록
	command := &models.Command{
		CommandType: models.CommandOrderCancel,
		Status:      models.StatusProcessing,
		RequestTime: time.Now(),
	}

	if err := h.db.Create(command).Error; err != nil {
		utils.Logger.Errorf("Failed to save orderCancel command: %v", err)
		h.sendResponseToPLC("OC:F")
		return
	}

	// 모든 실행 중인 오더들을 취소
	if err := h.orderExecutor.CancelAllRunningOrders(); err != nil {
		utils.Logger.Errorf("Failed to cancel running orders: %v", err)
		h.failCommand(command, err.Error())
		return
	}

	// 로봇에 cancelOrder 요청 전송
	if err := h.orderExecutor.SendCancelOrder(); err != nil {
		utils.Logger.Errorf("Failed to send cancelOrder to robot: %v", err)
		h.failCommand(command, err.Error())
		return
	}

	// 성공 응답을 PLC에 전송
	command.Status = models.StatusSuccess
	now := time.Now()
	command.ResponseTime = &now
	h.updateCommand(command)

	h.sendResponseToPLC("OC:S")
	utils.Logger.Infof("OrderCancel command completed successfully (Command ID: %d)", command.ID)
}

// FailProcessingCommands 외부에서 진행 중인 명령들을 실패 처리할 때 사용
func (h *CommandHandler) FailProcessingCommands(reason string) {
	// PLC 명령 실패 처리
	var processingCommands []models.Command
	h.db.Where("status = ?", models.StatusProcessing).Find(&processingCommands)

	for _, command := range processingCommands {
		h.failCommand(&command, reason)
		utils.Logger.Warnf("Command %d failed due to: %s", command.ID, reason)
	}

	// 실행 중인 오더들도 취소
	h.orderExecutor.CancelAllRunningOrders()
}

// handleRejectedCommand 거부된 명령 처리
func (h *CommandHandler) handleRejectedCommand(commandType string) {
	rejectedCommand := &models.Command{
		CommandType:  commandType,
		Status:       models.StatusRejected,
		RequestTime:  time.Now(),
		ErrorMessage: "Command rejected: Another command is currently processing or no order mappings found",
	}
	now := time.Now()
	rejectedCommand.ResponseTime = &now

	if err := h.db.Create(rejectedCommand).Error; err != nil {
		utils.Logger.Errorf("Failed to save rejected command to database: %v", err)
	}

	h.sendResponseToPLC(commandType + ":R")
}

// isRobotOnline 로봇 온라인 상태 확인
func (h *CommandHandler) isRobotOnline() bool {
	var robotStatus models.RobotStatus
	err := h.db.Where("serial_number = ?", h.config.RobotSerialNumber).First(&robotStatus).Error

	if err != nil {
		utils.Logger.Errorf("Failed to get robot status: %v", err)
		return false
	}

	return robotStatus.ConnectionState == models.ConnectionStateOnline
}

// failCommand 명령 실패 처리
func (h *CommandHandler) failCommand(command *models.Command, reason string) {
	command.Status = models.StatusFailure
	command.ErrorMessage = reason
	now := time.Now()
	command.ResponseTime = &now

	h.updateCommand(command)
	h.sendResponseToPLC(command.GetResponseCode())

	utils.Logger.Errorf("Command %d failed: %s", command.ID, reason)
}

// updateCommand 명령 상태 업데이트
func (h *CommandHandler) updateCommand(command *models.Command) {
	if err := h.db.Save(command).Error; err != nil {
		utils.Logger.Errorf("Failed to update command status: %v", err)
	}
}

// sendResponseToPLC PLC에 응답 전송
func (h *CommandHandler) sendResponseToPLC(responseCode string) {
	topic := "bridge/response"

	utils.Logger.Infof("Sending response to PLC: %s", responseCode)

	token := h.mqttClient.Publish(topic, 0, false, responseCode)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())
	} else {
		utils.Logger.Infof("Response sent successfully to PLC: %s", responseCode)
	}
}
