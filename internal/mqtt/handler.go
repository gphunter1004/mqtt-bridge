package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type MessageHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	mqttClient  mqtt.Client
}

func NewMessageHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client) *MessageHandler {
	return &MessageHandler{
		db:          db,
		redisClient: redisClient,
		mqttClient:  mqttClient,
	}
}

func (h *MessageHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	commandType := strings.TrimSpace(string(msg.Payload()))

	utils.Logger.Infof("Received command: %s", commandType)

	// 명령 유효성 검사
	if !models.IsValidCommand(commandType) {
		utils.Logger.Errorf("Invalid command received: %s", commandType)
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

	// Redis에 상태 저장
	ctx := context.Background()
	commandKey := fmt.Sprintf("command:%d", command.ID)
	commandData, _ := json.Marshal(command)

	if err := h.redisClient.Set(ctx, commandKey, commandData, 30*time.Minute).Err(); err != nil {
		utils.Logger.Errorf("Failed to save command to Redis: %v", err)
	}

	// 명령 처리 시작
	go h.processCommand(command)
}

// processCommand 명령 처리 로직
func (h *MessageHandler) processCommand(command *models.Command) {
	utils.Logger.Infof("Processing command: %s (ID: %d)", command.CommandType, command.ID)

	// 상태를 처리중으로 변경
	command.Status = models.StatusProcessing
	h.updateCommandStatus(command)

	// 실제 로봇 명령 처리 시뮬레이션
	// 여기서는 간단한 시뮬레이션을 진행하고, 실제로는 로봇 API 호출
	time.Sleep(2 * time.Second) // 처리 시간 시뮬레이션

	// 명령 타입에 따른 처리 결과 결정
	var finalStatus string
	switch command.CommandType {
	case models.CommandCataractRemoval, models.CommandGlaucomaRemoval:
		finalStatus = models.StatusSuccess // 실제로는 로봇 응답에 따라 결정
	case models.CommandGripperCleaning, models.CommandCameraCleaning, models.CommandKnifeCleaning:
		finalStatus = models.StatusSuccess
	case models.CommandCameraCheck:
		finalStatus = models.StatusNormal // 또는 StatusAbnormal
	default:
		finalStatus = models.StatusFailure
	}

	// 최종 상태 업데이트
	command.Status = finalStatus
	now := time.Now()
	command.ResponseTime = &now
	h.updateCommandStatus(command)

	// PLC에 응답 전송
	h.sendResponseToPLC(command)
}

// updateCommandStatus 명령 상태 업데이트
func (h *MessageHandler) updateCommandStatus(command *models.Command) {
	// 데이터베이스 업데이트
	if err := h.db.Save(command).Error; err != nil {
		utils.Logger.Errorf("Failed to update command status in database: %v", err)
	}

	// Redis 업데이트
	ctx := context.Background()
	commandKey := fmt.Sprintf("command:%d", command.ID)
	commandData, _ := json.Marshal(command)

	if err := h.redisClient.Set(ctx, commandKey, commandData, 30*time.Minute).Err(); err != nil {
		utils.Logger.Errorf("Failed to update command status in Redis: %v", err)
	}
}

// sendResponseToPLC PLC에 응답 전송
func (h *MessageHandler) sendResponseToPLC(command *models.Command) {
	responseCode := command.GetResponseCode()
	topic := "bridge/response"

	utils.Logger.Infof("Sending response to PLC: %s", responseCode)

	token := h.mqttClient.Publish(topic, 0, false, responseCode)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())
	} else {
		utils.Logger.Infof("Response sent successfully: %s", responseCode)
	}
}
