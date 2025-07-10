// internal/mqtt/command_handler.go
package mqtt

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	db          *gorm.DB
	redisClient *redis.Client
	mqttClient  mqtt.Client
	config      *config.Config
}

func NewCommandHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *CommandHandler {
	return &CommandHandler{
		db:          db,
		redisClient: redisClient,
		mqttClient:  mqttClient,
		config:      cfg,
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
	var processingCount int64
	h.db.Model(&models.Command{}).Where("status = ?", models.StatusProcessing).Count(&processingCount)

	if processingCount > 0 {
		utils.Logger.Warnf("Command %s rejected: Another command is currently processing", commandType)
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

	utils.Logger.Infof("Command accepted: ID=%d, Type=%s", command.ID, command.CommandType)

	// 명령 처리 시작
	go h.processCommand(command)
}

// handleRejectedCommand 거부된 명령 처리
func (h *CommandHandler) handleRejectedCommand(commandType string) {
	// 거부된 명령을 데이터베이스에 기록
	rejectedCommand := &models.Command{
		CommandType:  commandType,
		Status:       models.StatusRejected,
		RequestTime:  time.Now(),
		ErrorMessage: "Command rejected: Another command is currently processing",
	}
	now := time.Now()
	rejectedCommand.ResponseTime = &now

	if err := h.db.Create(rejectedCommand).Error; err != nil {
		utils.Logger.Errorf("Failed to save rejected command to database: %v", err)
	}

	// PLC에 거부 응답 전송
	h.sendResponseToPLC(commandType + ":R")
}

// processCommand 명령 처리 로직
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

	// 실제 로봇 명령 처리 시뮬레이션
	processingTime := 2 * time.Second
	utils.Logger.Infof("Processing robot operation for command %d", command.ID)
	time.Sleep(processingTime)

	// 명령 타입에 따른 처리 결과 결정
	finalStatus := h.determineCommandResult(command.CommandType)

	// 최종 상태 업데이트
	command.Status = finalStatus
	now := time.Now()
	command.ResponseTime = &now
	h.updateCommand(command)

	utils.Logger.Infof("Command %d completed with status: %s", command.ID, finalStatus)

	// PLC에 응답 전송
	h.sendResponseToPLC(command.GetResponseCode())
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

// determineCommandResult 명령 결과 결정
func (h *CommandHandler) determineCommandResult(commandType string) string {
	switch commandType {
	case models.CommandCataractRemoval, models.CommandGlaucomaRemoval:
		return models.StatusSuccess
	case models.CommandGripperCleaning, models.CommandCameraCleaning, models.CommandKnifeCleaning:
		return models.StatusSuccess
	case models.CommandOrderCancel:
		return models.StatusSuccess // orderCancel은 항상 성공
	case models.CommandCameraCheck:
		// 시뮬레이션: 80% 확률로 정상
		if time.Now().Unix()%5 != 0 {
			return models.StatusNormal
		} else {
			return models.StatusAbnormal
		}
	default:
		return models.StatusFailure
	}
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

// FailProcessingCommands 외부에서 진행 중인 명령들을 실패 처리할 때 사용
func (h *CommandHandler) FailProcessingCommands(reason string) {
	var processingCommands []models.Command
	h.db.Where("status = ?", models.StatusProcessing).Find(&processingCommands)

	for _, command := range processingCommands {
		h.failCommand(&command, reason)
		utils.Logger.Warnf("Command %d failed due to: %s", command.ID, reason)
	}
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
		h.sendResponseToPLC("OC:F") // 실패 응답
		return
	}

	// 로봇에 cancelOrder 요청 전송
	robotSerial := h.config.RobotSerialNumber
	robotManufacturer := h.config.RobotManufacturer

	if err := h.sendCancelOrderToRobot(command, robotSerial, robotManufacturer); err != nil {
		utils.Logger.Errorf("Failed to send cancelOrder to robot: %v", err)
		h.failCommand(command, fmt.Sprintf("Failed to send cancelOrder: %v", err))
		return
	}

	// 성공 응답을 PLC에 전송
	command.Status = models.StatusSuccess
	now := time.Now()
	command.ResponseTime = &now
	h.updateCommand(command)

	h.sendResponseToPLC("OC:S") // 성공 응답
	utils.Logger.Infof("OrderCancel command completed successfully (Command ID: %d)", command.ID)
}

// sendCancelOrderToRobot 로봇에 cancelOrder 요청 전송
func (h *CommandHandler) sendCancelOrderToRobot(command *models.Command, serialNumber, manufacturer string) error {
	actionID := h.generateActionID()

	request := map[string]interface{}{
		"headerId":     time.Now().Unix(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": manufacturer,
		"serialNumber": serialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":       "cancelOrder",
				"actionId":         actionID,
				"blockingType":     "HARD",
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	reqData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal cancelOrder request: %v", err)
	}

	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", manufacturer, serialNumber)

	utils.Logger.Infof("Sending cancelOrder request to %s (ActionID: %s)", topic, actionID)
	utils.Logger.Debugf("CancelOrder request payload: %s", string(reqData))

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	utils.Logger.Infof("CancelOrder request sent successfully to robot: %s (ActionID: %s)", serialNumber, actionID)
	return nil
}

// generateActionID 액션 ID 생성
func (h *CommandHandler) generateActionID() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("action_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%d", hex.EncodeToString(randomBytes), time.Now().UnixNano())
}
