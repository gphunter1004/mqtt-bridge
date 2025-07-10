// internal/mqtt/handler.go
package mqtt

import (
	"encoding/json"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// MessageHandler 메인 메시지 핸들러 (분리된 핸들러들을 조합)
type MessageHandler struct {
	commandHandler  *CommandHandler
	robotHandler    *RobotHandler
	positionHandler *PositionHandler
}

func NewMessageHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *MessageHandler {
	// 각 핸들러 생성
	commandHandler := NewCommandHandler(db, redisClient, mqttClient, cfg)
	positionHandler := NewPositionHandler(db, redisClient, mqttClient, cfg)
	robotHandler := NewRobotHandler(db, redisClient, mqttClient, cfg, commandHandler, positionHandler)

	return &MessageHandler{
		commandHandler:  commandHandler,
		robotHandler:    robotHandler,
		positionHandler: positionHandler,
	}
}

// HandleCommand PLC 명령 처리 (위임)
func (h *MessageHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	h.commandHandler.HandleCommand(client, msg)
}

// HandleRobotConnectionState 로봇 연결 상태 처리 (위임)
func (h *MessageHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleRobotConnectionState(client, msg)
}

// HandleRobotState 로봇 상태 처리 (위임 + 위치 체크)
func (h *MessageHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	// 기본 로봇 상태 처리
	h.robotHandler.HandleRobotState(client, msg)

	// 위치 초기화 체크
	var stateMsg models.RobotStateMessage
	if json.Unmarshal(msg.Payload(), &stateMsg) == nil {
		h.positionHandler.CheckAndRequestInitPosition(&stateMsg)
	}
}

// HandleRobotFactsheet 로봇 팩트시트 처리 (위임)
func (h *MessageHandler) HandleRobotFactsheet(client mqtt.Client, msg mqtt.Message) {
	h.positionHandler.HandleFactsheet(client, msg)
}
