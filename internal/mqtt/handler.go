// internal/mqtt/handler.go (service 의존성 제거)
package mqtt

import (
	"encoding/json"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// MessageHandler 메인 메시지 핸들러 (service 의존성 제거)
type MessageHandler struct {
	commandHandler  *CommandHandler
	robotHandler    *RobotHandler
	positionHandler *PositionHandler
}

func NewMessageHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *MessageHandler {
	// 각 핸들러 생성 (service 의존성 제거)
	commandHandler := NewCommandHandler(db, redisClient, mqttClient, cfg)
	positionHandler := NewPositionHandler(db, redisClient, mqttClient, cfg)
	robotHandler := NewRobotHandler(db, redisClient, mqttClient, cfg, commandHandler, positionHandler)

	return &MessageHandler{
		commandHandler:  commandHandler,
		robotHandler:    robotHandler,
		positionHandler: positionHandler,
	}
}

// HandleCommand PLC 명령 처리 (오더 시스템으로 위임)
func (h *MessageHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	h.commandHandler.HandleCommand(client, msg)
}

// HandleRobotConnectionState 로봇 연결 상태 처리 (위임)
func (h *MessageHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleRobotConnectionState(client, msg)
}

// HandleRobotState 로봇 상태 처리 (오더 시스템 상태 업데이트 포함)
func (h *MessageHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	// 기본 로봇 상태 처리
	h.robotHandler.HandleRobotState(client, msg)

	// 오더 상태 업데이트
	var stateMsg models.RobotStateMessage
	if json.Unmarshal(msg.Payload(), &stateMsg) == nil {
		// 오더 진행 상황 업데이트 (commandHandler에서 직접 처리)
		h.commandHandler.HandleOrderStateUpdate(&stateMsg)

		// 위치 초기화 체크
		h.positionHandler.CheckAndRequestInitPosition(&stateMsg)
	}
}

// HandleRobotFactsheet 로봇 팩트시트 처리 (위임)
func (h *MessageHandler) HandleRobotFactsheet(client mqtt.Client, msg mqtt.Message) {
	h.positionHandler.HandleFactsheet(client, msg)
}
