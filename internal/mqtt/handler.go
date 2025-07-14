// internal/mqtt/handler.go (수정된 버전)
package mqtt

import (
	"encoding/json"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type MessageHandler struct {
	commandHandler  *CommandHandler
	robotHandler    *RobotHandler
	positionHandler *PositionHandler
	orderExecutor   *OrderExecutor
}

func NewMessageHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *MessageHandler {
	commandHandler := NewCommandHandler(db, redisClient, mqttClient, cfg)
	positionHandler := NewPositionHandler(db, redisClient, mqttClient, cfg)
	robotHandler := NewRobotHandler(db, redisClient, mqttClient, cfg, commandHandler)
	orderExecutor := commandHandler.orderExecutor // Use the one from CommandHandler

	return &MessageHandler{
		commandHandler:  commandHandler,
		robotHandler:    robotHandler,
		positionHandler: positionHandler,
		orderExecutor:   orderExecutor,
	}
}

func (h *MessageHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	h.commandHandler.HandleCommand(client, msg)
}

func (h *MessageHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleRobotConnectionState(client, msg)
}

func (h *MessageHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	// Let robotHandler process the state first
	h.robotHandler.HandleRobotState(client, msg)

	// Then, pass the message to other relevant handlers
	var stateMsg models.RobotStateMessage
	if json.Unmarshal(msg.Payload(), &stateMsg) == nil {
		// The order executor listens for state changes to progress the workflow.
		h.orderExecutor.HandleOrderStateUpdate(&stateMsg)
		// Direct command state update handling (새로 추가)
		h.commandHandler.HandleDirectCommandStateUpdate(&stateMsg)
		// The position handler checks if the robot's position needs initialization.
		h.positionHandler.CheckAndRequestInitPosition(&stateMsg)
	}
}

func (h *MessageHandler) HandleRobotFactsheet(client mqtt.Client, msg mqtt.Message) {
	h.positionHandler.HandleFactsheet(client, msg)
}
