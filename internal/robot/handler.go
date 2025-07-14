// internal/robot/handler.go (통합된 최종 버전)
package robot

import (
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/common/idgen"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// CommandFailureHandler 명령 실패 처리 인터페이스
type CommandFailureHandler interface {
	FailAllProcessingCommands(reason string)
}

// Handler 로봇 메시지 처리 핸들러 (Position 기능 통합)
type Handler struct {
	statusManager         *StatusManager
	factsheetManager      *FactsheetManager
	commandFailureHandler CommandFailureHandler
	mqttClient            mqtt.Client
}

// NewHandler 새 로봇 핸들러 생성
func NewHandler(statusManager *StatusManager, factsheetManager *FactsheetManager,
	commandFailureHandler CommandFailureHandler, mqttClient mqtt.Client) *Handler {

	utils.Logger.Infof("🏗️ CREATING Robot Handler")

	handler := &Handler{
		statusManager:         statusManager,
		factsheetManager:      factsheetManager,
		commandFailureHandler: commandFailureHandler,
		mqttClient:            mqttClient,
	}

	utils.Logger.Infof("✅ Robot Handler CREATED")
	return handler
}

// HandleConnectionState 로봇 연결 상태 메시지 처리
func (h *Handler) HandleConnectionState(client mqtt.Client, msg mqtt.Message) {
	var connMsg models.ConnectionStateMessage
	if err := json.Unmarshal(msg.Payload(), &connMsg); err != nil {
		utils.Logger.Errorf("Failed to parse connection state message: %v", err)
		return
	}

	utils.Logger.Infof("Received robot connection state from topic: %s with state: %s",
		msg.Topic(), connMsg.ConnectionState)

	// 연결 상태 유효성 검사 (직접 constants 함수 사용)
	if !constants.IsValidConnectionState(connMsg.ConnectionState) {
		utils.Logger.Errorf("Invalid connection state received: %s", connMsg.ConnectionState)
		return
	}

	// 타임스탬프 파싱
	timestamp, err := time.Parse(time.RFC3339, connMsg.Timestamp)
	if err != nil {
		timestamp = time.Now()
		utils.Logger.Warnf("Failed to parse timestamp, using current time: %v", err)
	}

	// 상태 업데이트
	if err := h.statusManager.UpdateConnectionState(&connMsg, timestamp); err != nil {
		utils.Logger.Errorf("Failed to update robot status: %v", err)
		return
	}

	// 연결 상태 변화에 따른 후속 처리
	h.handleConnectionStateChange(&connMsg)
}

// HandleRobotState 로봇 상태 메시지 처리
func (h *Handler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	var stateMsg models.RobotStateMessage
	if err := json.Unmarshal(msg.Payload(), &stateMsg); err != nil {
		utils.Logger.Errorf("Failed to parse robot state message: %v", err)
		return
	}

	// 마지막 접속 시간 업데이트
	if err := h.statusManager.UpdateLastSeen(stateMsg.SerialNumber); err != nil {
		utils.Logger.Errorf("Failed to update last seen time: %v", err)
	}

	utils.Logger.Debugf("Robot state updated for %s", stateMsg.SerialNumber)
}

// HandleFactsheet 팩트시트 응답 처리
func (h *Handler) HandleFactsheet(client mqtt.Client, msg mqtt.Message) {
	var factsheetResp models.FactsheetResponse

	utils.Logger.Infof("Received factsheet response from topic: %s", msg.Topic())

	if err := json.Unmarshal(msg.Payload(), &factsheetResp); err != nil {
		utils.Logger.Errorf("Failed to parse factsheet response: %v", err)
		return
	}

	// 팩트시트 저장
	if err := h.factsheetManager.SaveFactsheet(&factsheetResp); err != nil {
		utils.Logger.Errorf("Failed to save factsheet: %v", err)
	}
}

// RequestFactsheet 팩트시트 요청 전송 (통합됨)
func (h *Handler) RequestFactsheet(manufacturer, serialNumber string) error {
	actionID := idgen.UniqueID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": manufacturer,
		"serialNumber": serialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":       constants.ActionTypeFactsheetRequest,
				"actionId":         actionID,
				"blockingType":     constants.BlockingTypeNone,
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	reqData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal factsheet request: %v", err)
	}

	topic := constants.GetMeiliInstantActionsTopic(manufacturer, serialNumber)
	utils.Logger.Infof("📤 SENDING factsheet request to %s (ActionID: %s)", topic, actionID)

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to send factsheet request: %v", token.Error())
	}

	utils.Logger.Infof("✅ Factsheet request sent successfully to robot: %s", serialNumber)
	return nil
}

// CheckAndRequestInitPosition 위치 초기화 확인 및 요청 (Position에서 통합됨)
func (h *Handler) CheckAndRequestInitPosition(stateMsg *models.RobotStateMessage) {
	if stateMsg == nil {
		utils.Logger.Errorf("State message is nil")
		return
	}

	// 이미 초기화되었으면 무시
	if stateMsg.AgvPosition.PositionInitialized {
		return
	}

	// 자동 모드에서만 처리
	operatingMode := stateMsg.OperatingMode
	if operatingMode == "" {
		operatingMode = "UNKNOWN"
	}

	if operatingMode != constants.OperatingModeAutomatic {
		utils.Logger.Debugf("Robot %s not in automatic mode (%s), skipping initPosition",
			stateMsg.SerialNumber, operatingMode)
		return
	}

	utils.Logger.Infof("Robot %s position not initialized, sending initPosition request",
		stateMsg.SerialNumber)

	if err := h.sendInitPositionRequest(stateMsg); err != nil {
		utils.Logger.Errorf("Failed to send initPosition request: %v", err)
	}
}

// sendInitPositionRequest initPosition 요청 전송 (Position에서 통합됨)
func (h *Handler) sendInitPositionRequest(stateMsg *models.RobotStateMessage) error {
	if stateMsg == nil {
		return fmt.Errorf("state message is nil")
	}

	actionID := idgen.UniqueID()

	// 안전한 필드 접근
	safeString := func(val string) string {
		if val == "" {
			return ""
		}
		return val
	}

	safeFloat := func(val float64) float64 {
		if val != val { // NaN 체크
			return 0.0
		}
		return val
	}

	// 현재 위치를 기준으로 초기 위치 설정
	pose := map[string]interface{}{
		"lastNodeId": "",
		"mapId":      safeString(stateMsg.AgvPosition.MapID),
		"theta":      safeFloat(stateMsg.AgvPosition.Theta),
		"x":          safeFloat(stateMsg.AgvPosition.X),
		"y":          safeFloat(stateMsg.AgvPosition.Y),
	}

	// 위치가 모두 0이면 원점 사용
	x := safeFloat(stateMsg.AgvPosition.X)
	y := safeFloat(stateMsg.AgvPosition.Y)
	theta := safeFloat(stateMsg.AgvPosition.Theta)

	if x == 0 && y == 0 && theta == 0 {
		pose["x"] = 0.0
		pose["y"] = 0.0
		pose["theta"] = 0.0
		utils.Logger.Infof("Using origin position for robot %s", stateMsg.SerialNumber)
	}

	// 요청 메시지 생성
	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": safeString(stateMsg.Manufacturer),
		"serialNumber": safeString(stateMsg.SerialNumber),
		"actions": []map[string]interface{}{
			{
				"actionType":   constants.ActionTypeInitPosition,
				"actionId":     actionID,
				"blockingType": constants.BlockingTypeNone,
				"actionParameters": []map[string]interface{}{
					{
						"key":   "pose",
						"value": pose,
					},
				},
			},
		},
	}

	// JSON 직렬화
	reqData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	// MQTT 토픽 생성 및 전송
	manufacturer := safeString(stateMsg.Manufacturer)
	serialNumber := safeString(stateMsg.SerialNumber)

	if manufacturer == "" || serialNumber == "" {
		return fmt.Errorf("invalid manufacturer or serial number")
	}

	topic := constants.GetMeiliInstantActionsTopic(manufacturer, serialNumber)

	utils.Logger.Infof("📤 SENDING initPosition request to %s (ActionID: %s)", topic, actionID)
	utils.Logger.Debugf("Request payload: %s", string(reqData))

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	utils.Logger.Infof("✅ InitPosition request sent successfully to robot: %s", serialNumber)
	return nil
}

// GetStatusManager 상태 관리자 반환 (필수 Getter - 실제 사용됨)
func (h *Handler) GetStatusManager() *StatusManager {
	return h.statusManager
}

// GetFactsheetManager 팩트시트 관리자 반환 (필수 Getter - 실제 사용됨)
func (h *Handler) GetFactsheetManager() *FactsheetManager {
	return h.factsheetManager
}

// handleConnectionStateChange 연결 상태 변화에 따른 후속 처리
func (h *Handler) handleConnectionStateChange(connMsg *models.ConnectionStateMessage) {
	switch connMsg.ConnectionState {
	case constants.ConnectionStateOnline:
		utils.Logger.Infof("Robot %s is now ONLINE", connMsg.SerialNumber)

		// 온라인 상태가 되면 팩트시트 요청
		go func() {
			if err := h.RequestFactsheet(connMsg.Manufacturer, connMsg.SerialNumber); err != nil {
				utils.Logger.Errorf("Failed to request factsheet for robot %s: %v", connMsg.SerialNumber, err)
			}
		}()

	case constants.ConnectionStateOffline:
		utils.Logger.Warnf("Robot %s is now OFFLINE", connMsg.SerialNumber)

		// 오프라인 상태가 되면 모든 진행 중인 명령 실패 처리
		if h.commandFailureHandler != nil {
			h.commandFailureHandler.FailAllProcessingCommands("Robot went offline")
		}

	case constants.ConnectionStateConnectionBroken:
		utils.Logger.Errorf("Robot %s connection is BROKEN", connMsg.SerialNumber)

		// 연결이 끊어지면 모든 진행 중인 명령 실패 처리
		if h.commandFailureHandler != nil {
			h.commandFailureHandler.FailAllProcessingCommands("Robot connection broken")
		}
	}
}
