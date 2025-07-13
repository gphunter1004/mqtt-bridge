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
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RobotHandler struct {
	db              *gorm.DB
	redisClient     *redis.Client
	mqttClient      mqtt.Client
	config          *config.Config
	commandHandler  *CommandHandler
	workflowManager *WorkflowManager
	plcNotifier     *PLCNotifier
}

func NewRobotHandler(
	db *gorm.DB,
	redisClient *redis.Client,
	mqttClient mqtt.Client,
	cfg *config.Config,
	cmdHandler *CommandHandler,
	workflowManager *WorkflowManager,
	plcNotifier *PLCNotifier,
) *RobotHandler {
	return &RobotHandler{
		db:              db,
		redisClient:     redisClient,
		mqttClient:      mqttClient,
		config:          cfg,
		commandHandler:  cmdHandler,
		workflowManager: workflowManager,
		plcNotifier:     plcNotifier,
	}
}

// HandleRobotConnectionState 로봇 연결 상태 처리
func (h *RobotHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	var connMsg models.ConnectionStateMessage
	if err := json.Unmarshal(msg.Payload(), &connMsg); err != nil {
		utils.Logger.Errorf("Failed to parse connection state message: %v", err)
		return
	}

	utils.Logger.Infof("Robot %s connection state: %s", connMsg.SerialNumber, connMsg.ConnectionState)

	// 상태 유효성 검사
	if !models.IsValidConnectionState(connMsg.ConnectionState) {
		utils.Logger.Errorf("Invalid connection state: %s", connMsg.ConnectionState)
		return
	}

	// 타임스탬프 파싱
	timestamp, err := time.Parse(time.RFC3339, connMsg.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	// 로봇 상태 업데이트
	h.updateRobotConnectionState(&connMsg, timestamp)

	// 연결 상태 변경에 따른 처리
	h.handleConnectionStateChange(&connMsg)
}

// HandleRobotState 로봇 상태 메시지 처리
func (h *RobotHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	var stateMsg models.RobotStateMessage
	if err := json.Unmarshal(msg.Payload(), &stateMsg); err != nil {
		utils.Logger.Errorf("Failed to parse robot state message: %v", err)
		return
	}

	// 로봇 운영 데이터 업데이트
	h.updateRobotOperationalData(&stateMsg)

	// 워크플로우 매니저에 상태 업데이트 전달
	h.workflowManager.HandleRobotStateUpdate(&stateMsg)

	// 위치 초기화 필요 여부 확인
	h.checkPositionInitialization(&stateMsg)

	// 안전 상태 확인
	h.checkSafetyState(&stateMsg)
}

// HandleFactsheet 팩트시트 응답 처리
func (h *RobotHandler) HandleFactsheet(client mqtt.Client, msg mqtt.Message) {
	var factsheet models.FactsheetResponse
	if err := json.Unmarshal(msg.Payload(), &factsheet); err != nil {
		utils.Logger.Errorf("Failed to parse factsheet response: %v", err)
		return
	}

	utils.Logger.Infof("Received factsheet for robot %s", factsheet.SerialNumber)

	// 팩트시트 데이터 저장
	h.saveFactsheetData(&factsheet)
}

// updateRobotConnectionState 로봇 연결 상태 업데이트
func (h *RobotHandler) updateRobotConnectionState(connMsg *models.ConnectionStateMessage, timestamp time.Time) {
	var robotStatus models.RobotStatus

	err := h.db.Where("serial_number = ?", connMsg.SerialNumber).First(&robotStatus).Error

	if err == gorm.ErrRecordNotFound {
		// 새 로봇 등록
		robotStatus = models.RobotStatus{
			SerialNumber:    connMsg.SerialNumber,
			Manufacturer:    connMsg.Manufacturer,
			ConnectionState: connMsg.ConnectionState,
			LastHeaderID:    connMsg.HeaderID,
			LastUpdated:     timestamp,
			OperationalData: models.JSON{
				"current": map[string]interface{}{
					"position": map[string]float64{"x": 0, "y": 0, "theta": 0},
					"battery":  map[string]interface{}{"charge": 100, "voltage": 0},
					"mode":     "MANUAL",
				},
				"history": []interface{}{},
			},
		}
		h.db.Create(&robotStatus)
	} else if err == nil {
		// 기존 로봇 업데이트
		updates := map[string]interface{}{
			"connection_state": connMsg.ConnectionState,
			"last_header_id":   connMsg.HeaderID,
			"last_updated":     timestamp,
		}

		// 연결 상태 이력 추가
		if robotStatus.OperationalData == nil {
			robotStatus.OperationalData = models.JSON{}
		}

		history := robotStatus.OperationalData["history"].([]interface{})
		history = append(history, map[string]interface{}{
			"timestamp": timestamp,
			"event":     "connection_state_changed",
			"data": map[string]interface{}{
				"from": robotStatus.ConnectionState,
				"to":   connMsg.ConnectionState,
			},
		})

		// 최근 10개 이벤트만 유지
		if len(history) > 10 {
			history = history[len(history)-10:]
		}

		robotStatus.OperationalData["history"] = history
		updates["operational_data"] = robotStatus.OperationalData

		h.db.Model(&robotStatus).Updates(updates)
	}
}

// handleConnectionStateChange 연결 상태 변경 처리
func (h *RobotHandler) handleConnectionStateChange(connMsg *models.ConnectionStateMessage) {
	switch connMsg.ConnectionState {
	case models.ConnectionStateOnline:
		utils.Logger.Infof("Robot %s is now ONLINE", connMsg.SerialNumber)
		// 팩트시트 요청
		h.requestFactsheet(connMsg.SerialNumber, connMsg.Manufacturer)

	case models.ConnectionStateOffline:
		utils.Logger.Warnf("Robot %s is now OFFLINE", connMsg.SerialNumber)
		h.commandHandler.FailCommandsForRobot(connMsg.SerialNumber, "Robot went offline")

	case models.ConnectionStateConnectionBroken:
		utils.Logger.Errorf("Robot %s connection is BROKEN", connMsg.SerialNumber)
		h.commandHandler.FailCommandsForRobot(connMsg.SerialNumber, "Robot connection broken")
	}
}

// updateRobotOperationalData 로봇 운영 데이터 업데이트
func (h *RobotHandler) updateRobotOperationalData(stateMsg *models.RobotStateMessage) {
	operationalData := models.JSON{
		"current": map[string]interface{}{
			"position": map[string]interface{}{
				"x":           stateMsg.AgvPosition.X,
				"y":           stateMsg.AgvPosition.Y,
				"theta":       stateMsg.AgvPosition.Theta,
				"initialized": stateMsg.AgvPosition.PositionInitialized,
				"map_id":      stateMsg.AgvPosition.MapID,
			},
			"battery": map[string]interface{}{
				"charge":   stateMsg.BatteryState.BatteryCharge,
				"voltage":  stateMsg.BatteryState.BatteryVoltage,
				"charging": stateMsg.BatteryState.Charging,
			},
			"mode":     stateMsg.OperatingMode,
			"driving":  stateMsg.Driving,
			"paused":   stateMsg.Paused,
			"order_id": stateMsg.OrderID,
			"e_stop":   stateMsg.SafetyState.EStop,
		},
	}

	// 마지막 액션 상태 추출
	lastActionStatus := ""
	for _, action := range stateMsg.ActionStates {
		lastActionStatus = action.ActionStatus
	}

	h.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", stateMsg.SerialNumber).
		Updates(map[string]interface{}{
			"operational_data":   operationalData,
			"last_action_status": lastActionStatus,
			"last_header_id":     stateMsg.HeaderID,
			"last_updated":       time.Now(),
		})
}

// checkPositionInitialization 위치 초기화 확인
func (h *RobotHandler) checkPositionInitialization(stateMsg *models.RobotStateMessage) {
	// 위치가 초기화되지 않았고 자동 모드인 경우
	if !stateMsg.AgvPosition.PositionInitialized &&
		stateMsg.OperatingMode == "AUTOMATIC" {
		utils.Logger.Infof("Robot %s needs position initialization", stateMsg.SerialNumber)
		h.sendInitPositionRequest(stateMsg)
	}
}

// checkSafetyState 안전 상태 확인
func (h *RobotHandler) checkSafetyState(stateMsg *models.RobotStateMessage) {
	// E-Stop 활성화 확인
	if stateMsg.SafetyState.EStop != "NONE" {
		utils.Logger.Warnf("Robot %s E-Stop activated: %s",
			stateMsg.SerialNumber, stateMsg.SafetyState.EStop)

		// 진행 중인 명령 실패 처리
		h.commandHandler.FailCommandsForRobot(stateMsg.SerialNumber,
			fmt.Sprintf("E-Stop activated: %s", stateMsg.SafetyState.EStop))
	}

	// 배터리 부족 확인
	if stateMsg.BatteryState.BatteryCharge < 5.0 && !stateMsg.BatteryState.Charging {
		utils.Logger.Errorf("Robot %s battery critically low: %.1f%%",
			stateMsg.SerialNumber, stateMsg.BatteryState.BatteryCharge)

		h.commandHandler.FailCommandsForRobot(stateMsg.SerialNumber,
			"Battery critically low")
	}
}

// sendInitPositionRequest 위치 초기화 요청
func (h *RobotHandler) sendInitPositionRequest(stateMsg *models.RobotStateMessage) {
	actionID := generateID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": stateMsg.Manufacturer,
		"serialNumber": stateMsg.SerialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":   "initPosition",
				"actionId":     actionID,
				"blockingType": "NONE",
				"actionParameters": []map[string]interface{}{
					{
						"key": "pose",
						"value": map[string]interface{}{
							"lastNodeId": "",
							"mapId":      stateMsg.AgvPosition.MapID,
							"theta":      stateMsg.AgvPosition.Theta,
							"x":          stateMsg.AgvPosition.X,
							"y":          stateMsg.AgvPosition.Y,
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(request)
	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions",
		stateMsg.Manufacturer, stateMsg.SerialNumber)

	token := h.mqttClient.Publish(topic, 0, false, data)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send initPosition request: %v", token.Error())
	} else {
		utils.Logger.Infof("InitPosition request sent to robot %s", stateMsg.SerialNumber)
	}
}

// requestFactsheet 팩트시트 요청
func (h *RobotHandler) requestFactsheet(serialNumber, manufacturer string) {
	actionID := generateID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": manufacturer,
		"serialNumber": serialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":       "factsheetRequest",
				"actionId":         actionID,
				"blockingType":     "NONE",
				"actionParameters": []interface{}{},
			},
		},
	}

	data, _ := json.Marshal(request)
	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", manufacturer, serialNumber)

	token := h.mqttClient.Publish(topic, 0, false, data)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to request factsheet: %v", token.Error())
	} else {
		utils.Logger.Infof("Factsheet request sent to robot %s", serialNumber)
	}
}

// saveFactsheetData 팩트시트 데이터 저장
func (h *RobotHandler) saveFactsheetData(factsheet *models.FactsheetResponse) {
	factsheetData := models.JSON{
		"version":            factsheet.Version,
		"type_specification": factsheet.TypeSpecification,
		"physical_params":    factsheet.PhysicalParameters,
		"protocol_features":  factsheet.ProtocolFeatures,
		"last_updated":       time.Now(),
	}

	h.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", factsheet.SerialNumber).
		Update("factsheet_data", factsheetData)

	utils.Logger.Infof("Factsheet saved for robot %s", factsheet.SerialNumber)
}

// generateID generates a new unique ID (UUID) for an action.
func generateID() string {
	return uuid.New().String()
}
