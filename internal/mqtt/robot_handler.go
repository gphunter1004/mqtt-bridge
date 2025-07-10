// internal/mqtt/robot_handler.go
package mqtt

import (
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type RobotHandler struct {
	db              *gorm.DB
	redisClient     *redis.Client
	mqttClient      mqtt.Client
	config          *config.Config
	commandHandler  *CommandHandler
	positionHandler *PositionHandler
}

func NewRobotHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config, cmdHandler *CommandHandler, posHandler *PositionHandler) *RobotHandler {
	return &RobotHandler{
		db:              db,
		redisClient:     redisClient,
		mqttClient:      mqttClient,
		config:          cfg,
		commandHandler:  cmdHandler,
		positionHandler: posHandler,
	}
}

// HandleRobotConnectionState 로봇 연결 상태 메시지 처리
func (h *RobotHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	var connMsg models.ConnectionStateMessage

	utils.Logger.Infof("Received robot connection state from topic: %s", msg.Topic())

	// JSON 파싱
	if err := json.Unmarshal(msg.Payload(), &connMsg); err != nil {
		utils.Logger.Errorf("Failed to parse connection state message: %v", err)
		return
	}

	// 연결 상태 유효성 검사
	if !models.IsValidConnectionState(connMsg.ConnectionState) {
		utils.Logger.Errorf("Invalid connection state received: %s", connMsg.ConnectionState)
		return
	}

	// 타임스탬프 파싱
	timestamp, err := time.Parse(time.RFC3339, connMsg.Timestamp)
	if err != nil {
		utils.Logger.Errorf("Failed to parse timestamp: %v", err)
		timestamp = time.Now()
	}

	// 로봇 상태 업데이트 또는 생성
	h.updateRobotStatus(&connMsg, timestamp)

	// 연결 상태 변경에 따른 처리
	h.handleConnectionStateChange(&connMsg)

	utils.Logger.Infof("Robot %s status updated: %s (HeaderID: %d)",
		connMsg.SerialNumber, connMsg.ConnectionState, connMsg.HeaderID)
}

// HandleRobotState 로봇 운영 상태 메시지 처리
func (h *RobotHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	var stateMsg models.RobotStateMessage

	utils.Logger.Debugf("Received robot state from topic: %s", msg.Topic())

	// JSON 파싱
	if err := json.Unmarshal(msg.Payload(), &stateMsg); err != nil {
		utils.Logger.Errorf("Failed to parse robot state message: %v", err)
		return
	}

	// 기본 유효성 검사
	if !models.IsValidOperatingMode(stateMsg.OperatingMode) {
		utils.Logger.Errorf("Invalid operating mode: %s", stateMsg.OperatingMode)
		return
	}

	// 타임스탬프 파싱
	timestamp := h.parseTimestamp(stateMsg.Timestamp)

	// 로봇 상태 저장
	h.saveRobotState(&stateMsg, timestamp)

	// 중요한 상태 변경 처리
	h.handleCriticalStates(&stateMsg)

	utils.Logger.Debugf("Robot state updated: %s (Battery: %.1f%%, Position: (%.2f, %.2f), PosInit: %v)",
		stateMsg.SerialNumber, stateMsg.BatteryState.BatteryCharge,
		stateMsg.AgvPosition.X, stateMsg.AgvPosition.Y, stateMsg.AgvPosition.PositionInitialized)
}

// updateRobotStatus 로봇 연결 상태 업데이트
func (h *RobotHandler) updateRobotStatus(connMsg *models.ConnectionStateMessage, timestamp time.Time) {
	var existingStatus models.RobotStatus
	result := h.db.Where("serial_number = ?", connMsg.SerialNumber).First(&existingStatus)

	if result.Error == gorm.ErrRecordNotFound {
		// 새로운 로봇 등록
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
		} else {
			utils.Logger.Infof("New robot registered: %s (%s)", connMsg.SerialNumber, connMsg.Manufacturer)
		}
	} else if result.Error == nil {
		// 기존 로봇 상태 업데이트
		existingStatus.ConnectionState = connMsg.ConnectionState
		existingStatus.LastHeaderID = connMsg.HeaderID
		existingStatus.LastTimestamp = timestamp
		existingStatus.Version = connMsg.Version

		if err := h.db.Save(&existingStatus).Error; err != nil {
			utils.Logger.Errorf("Failed to update robot status: %v", err)
		}
	}
}

// saveRobotState 로봇 상태 저장
func (h *RobotHandler) saveRobotState(stateMsg *models.RobotStateMessage, timestamp time.Time) {
	robotState := &models.RobotState{
		SerialNumber: stateMsg.SerialNumber,
		Manufacturer: stateMsg.Manufacturer,
		Version:      stateMsg.Version,
		HeaderID:     stateMsg.HeaderID,
		Timestamp:    timestamp,

		// 위치 정보
		PositionX:           stateMsg.AgvPosition.X,
		PositionY:           stateMsg.AgvPosition.Y,
		PositionTheta:       stateMsg.AgvPosition.Theta,
		LocalizationScore:   stateMsg.AgvPosition.LocalizationScore,
		PositionInitialized: stateMsg.AgvPosition.PositionInitialized,
		MapID:               stateMsg.AgvPosition.MapID,

		// 배터리 정보
		BatteryCharge:  stateMsg.BatteryState.BatteryCharge,
		BatteryVoltage: stateMsg.BatteryState.BatteryVoltage,
		BatteryHealth:  stateMsg.BatteryState.BatteryHealth,
		Charging:       stateMsg.BatteryState.Charging,

		// 운영 상태
		OperatingMode: stateMsg.OperatingMode,
		Driving:       stateMsg.Driving,
		Paused:        stateMsg.Paused,

		// 안전 상태
		EStop:          stateMsg.SafetyState.EStop,
		FieldViolation: stateMsg.SafetyState.FieldViolation,

		// 기타 정보
		OrderID:     stateMsg.OrderID,
		LastNodeID:  stateMsg.LastNodeID,
		ErrorCount:  len(stateMsg.Errors),
		ActionCount: len(stateMsg.ActionStates),
	}

	if err := h.db.Create(robotState).Error; err != nil {
		utils.Logger.Errorf("Failed to save robot state: %v", err)
	}
}

// handleConnectionStateChange 연결 상태 변경 처리
func (h *RobotHandler) handleConnectionStateChange(connMsg *models.ConnectionStateMessage) {
	switch connMsg.ConnectionState {
	case models.ConnectionStateOnline:
		utils.Logger.Infof("Robot %s is now ONLINE", connMsg.SerialNumber)

	case models.ConnectionStateOffline:
		utils.Logger.Warnf("Robot %s is now OFFLINE", connMsg.SerialNumber)
		h.commandHandler.FailProcessingCommands("Robot went offline")

	case models.ConnectionStateConnectionBroken:
		utils.Logger.Errorf("Robot %s connection is BROKEN", connMsg.SerialNumber)
		h.commandHandler.FailProcessingCommands("Robot connection broken")
	}
}

// handleCriticalStates 중요한 상태 변경 처리 (panic 방어)
func (h *RobotHandler) handleCriticalStates(stateMsg *models.RobotStateMessage) {
	if stateMsg == nil {
		utils.Logger.Errorf("State message is nil")
		return
	}

	// E-Stop 확인 (SafetyState 안전 접근)
	eStopStatus := models.EStopNone
	if stateMsg.SafetyState.EStop != "" {
		eStopStatus = stateMsg.SafetyState.EStop
	}

	if eStopStatus != models.EStopNone {
		utils.Logger.Warnf("Robot %s E-Stop activated: %s", stateMsg.SerialNumber, eStopStatus)
		h.commandHandler.FailProcessingCommands(fmt.Sprintf("E-Stop activated: %s", eStopStatus))
	}

	// 크리티컬 에러 확인 (Errors 배열 안전 접근)
	if stateMsg.Errors != nil && len(stateMsg.Errors) > 0 {
		for _, errorInfo := range stateMsg.Errors {
			if errorInfo.ErrorLevel == "FATAL" || errorInfo.ErrorLevel == "ERROR" {
				utils.Logger.Errorf("Robot %s critical error: %s - %s",
					stateMsg.SerialNumber, errorInfo.ErrorType, errorInfo.ErrorDescription)
				h.commandHandler.FailProcessingCommands(fmt.Sprintf("Robot error: %s", errorInfo.ErrorType))
				break // 첫 번째 크리티컬 에러만 처리
			}
		}
	}

	// 크리티컬 저배터리 확인 (BatteryState 안전 접근)
	batteryCharge := 100.0 // 기본값
	charging := false

	if stateMsg.BatteryState.BatteryCharge >= 0 {
		batteryCharge = stateMsg.BatteryState.BatteryCharge
	}
	charging = stateMsg.BatteryState.Charging

	if batteryCharge < 5.0 && !charging {
		utils.Logger.Errorf("Robot %s critical low battery: %.1f%%", stateMsg.SerialNumber, batteryCharge)
		h.commandHandler.FailProcessingCommands(fmt.Sprintf("Critical low battery: %.1f%%", batteryCharge))
	}

	// cancelOrder 액션 상태 확인 (ActionStates 배열 안전 접근)
	h.checkCancelOrderActions(stateMsg)
}

// checkCancelOrderActions cancelOrder 액션 상태 확인 (panic 방어)
func (h *RobotHandler) checkCancelOrderActions(stateMsg *models.RobotStateMessage) {
	if stateMsg == nil || stateMsg.ActionStates == nil {
		return
	}

	for _, actionState := range stateMsg.ActionStates {
		if actionState.ActionType == "cancelOrder" {
			utils.Logger.Infof("Robot %s cancelOrder action - ID: %s, Status: %s",
				stateMsg.SerialNumber, actionState.ActionID, actionState.ActionStatus)

			// cancelOrder 완료 시 로그 기록
			if actionState.ActionStatus == models.ActionStatusFinished {
				utils.Logger.Infof("Robot %s cancelOrder completed successfully - ActionID: %s",
					stateMsg.SerialNumber, actionState.ActionID)
			} else if actionState.ActionStatus == models.ActionStatusFailed {
				utils.Logger.Errorf("Robot %s cancelOrder failed - ActionID: %s, Description: %s",
					stateMsg.SerialNumber, actionState.ActionID, actionState.ResultDescription)
			}
		}
	}
}

// parseTimestamp 타임스탬프 파싱
func (h *RobotHandler) parseTimestamp(timestampStr string) time.Time {
	// 여러 형식 시도
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000000000Z",
	}

	for _, format := range formats {
		if timestamp, err := time.Parse(format, timestampStr); err == nil {
			return timestamp
		}
	}

	utils.Logger.Errorf("Failed to parse timestamp: %s", timestampStr)
	return time.Now()
}
