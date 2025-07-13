// internal/mqtt/robot_handler.go (Corrected)
package mqtt

import (
	"encoding/json"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type RobotHandler struct {
	db             *gorm.DB
	redisClient    *redis.Client
	mqttClient     mqtt.Client
	config         *config.Config
	commandHandler *CommandHandler
}

func NewRobotHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config, cmdHandler *CommandHandler) *RobotHandler {
	return &RobotHandler{
		db:             db,
		redisClient:    redisClient,
		mqttClient:     mqttClient,
		config:         cfg,
		commandHandler: cmdHandler,
	}
}

func (h *RobotHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	var connMsg models.ConnectionStateMessage
	if err := json.Unmarshal(msg.Payload(), &connMsg); err != nil {
		utils.Logger.Errorf("Failed to parse connection state message: %v", err)
		return
	}
	utils.Logger.Infof("Received robot connection state from topic: %s with state: %s", msg.Topic(), connMsg.ConnectionState)

	if !models.IsValidConnectionState(connMsg.ConnectionState) {
		utils.Logger.Errorf("Invalid connection state received: %s", connMsg.ConnectionState)
		return
	}

	timestamp, err := time.Parse(time.RFC3339, connMsg.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	h.updateRobotStatus(&connMsg, timestamp)
	h.handleConnectionStateChange(&connMsg)
}

func (h *RobotHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	var stateMsg models.RobotStateMessage
	if err := json.Unmarshal(msg.Payload(), &stateMsg); err != nil {
		utils.Logger.Errorf("Failed to parse robot state message: %v", err)
		return
	}
	// Further processing is now handled by MessageHandler
}

func (h *RobotHandler) updateRobotStatus(connMsg *models.ConnectionStateMessage, timestamp time.Time) {
	var existingStatus models.RobotStatus
	result := h.db.Where("serial_number = ?", connMsg.SerialNumber).First(&existingStatus)

	if result.Error == gorm.ErrRecordNotFound {
		robotStatus := &models.RobotStatus{
			Manufacturer:    connMsg.Manufacturer,
			SerialNumber:    connMsg.SerialNumber,
			ConnectionState: connMsg.ConnectionState,
			LastHeaderID:    connMsg.HeaderID,
			LastTimestamp:   timestamp,
			Version:         connMsg.Version,
		}
		if err := h.db.Create(robotStatus).Error; err != nil {
			utils.Logger.Errorf("Failed to create robot status: %v", err)
		}
	} else if result.Error == nil {
		existingStatus.ConnectionState = connMsg.ConnectionState
		existingStatus.LastHeaderID = connMsg.HeaderID
		existingStatus.LastTimestamp = timestamp
		existingStatus.Version = connMsg.Version
		if err := h.db.Save(&existingStatus).Error; err != nil {
			utils.Logger.Errorf("Failed to update robot status: %v", err)
		}
	}
}

func (h *RobotHandler) handleConnectionStateChange(connMsg *models.ConnectionStateMessage) {
	switch connMsg.ConnectionState {
	case models.ConnectionStateOnline:
		utils.Logger.Infof("Robot %s is now ONLINE", connMsg.SerialNumber)
		// Factsheet request is now handled in the main handler to have access to positionHandler
	case models.ConnectionStateOffline:
		utils.Logger.Warnf("Robot %s is now OFFLINE", connMsg.SerialNumber)
		h.commandHandler.FailAllProcessingCommands("Robot went offline")
	case models.ConnectionStateConnectionBroken:
		utils.Logger.Errorf("Robot %s connection is BROKEN", connMsg.SerialNumber)
		h.commandHandler.FailAllProcessingCommands("Robot connection broken")
	}
}
