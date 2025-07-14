// internal/robot/handler.go
package robot

import (
	"encoding/json"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// CommandFailureHandler 명령 실패 처리 인터페이스
type CommandFailureHandler interface {
	FailAllProcessingCommands(reason string)
}

// FactsheetRequester 팩트시트 요청 인터페이스
type FactsheetRequester interface {
	RequestFactsheet(serialNumber, manufacturer string) error
}

// Handler 로봇 메시지 처리 핸들러
type Handler struct {
	statusManager         *StatusManager
	factsheetManager      *FactsheetManager
	commandFailureHandler CommandFailureHandler
	factsheetRequester    FactsheetRequester
}

// NewHandler 새 로봇 핸들러 생성
func NewHandler(statusManager *StatusManager, factsheetManager *FactsheetManager,
	commandFailureHandler CommandFailureHandler, factsheetRequester FactsheetRequester) *Handler {

	utils.Logger.Infof("🏗️ CREATING Robot Handler")

	handler := &Handler{
		statusManager:         statusManager,
		factsheetManager:      factsheetManager,
		commandFailureHandler: commandFailureHandler,
		factsheetRequester:    factsheetRequester,
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

	// 연결 상태 유효성 검사
	if !h.statusManager.IsValidConnectionState(connMsg.ConnectionState) {
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

// HandleRobotState 로봇 상태 메시지 처리 (기본 처리만, 상세 처리는 다른 핸들러에서)
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

	// 추가 상태 처리는 다른 핸들러들에서 수행
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

// GetStatusManager 상태 관리자 반환 (다른 컴포넌트에서 사용)
func (h *Handler) GetStatusManager() *StatusManager {
	return h.statusManager
}

// GetFactsheetManager 팩트시트 관리자 반환 (다른 컴포넌트에서 사용)
func (h *Handler) GetFactsheetManager() *FactsheetManager {
	return h.factsheetManager
}

// handleConnectionStateChange 연결 상태 변화에 따른 후속 처리
func (h *Handler) handleConnectionStateChange(connMsg *models.ConnectionStateMessage) {
	switch connMsg.ConnectionState {
	case models.ConnectionStateOnline:
		utils.Logger.Infof("Robot %s is now ONLINE", connMsg.SerialNumber)

		// 온라인 상태가 되면 팩트시트 요청
		if h.factsheetRequester != nil {
			go func() {
				if err := h.factsheetRequester.RequestFactsheet(connMsg.SerialNumber, connMsg.Manufacturer); err != nil {
					utils.Logger.Errorf("Failed to request factsheet for robot %s: %v", connMsg.SerialNumber, err)
				}
			}()
		}

	case models.ConnectionStateOffline:
		utils.Logger.Warnf("Robot %s is now OFFLINE", connMsg.SerialNumber)

		// 오프라인 상태가 되면 모든 진행 중인 명령 실패 처리
		if h.commandFailureHandler != nil {
			h.commandFailureHandler.FailAllProcessingCommands("Robot went offline")
		}

	case models.ConnectionStateConnectionBroken:
		utils.Logger.Errorf("Robot %s connection is BROKEN", connMsg.SerialNumber)

		// 연결이 끊어지면 모든 진행 중인 명령 실패 처리
		if h.commandFailureHandler != nil {
			h.commandFailureHandler.FailAllProcessingCommands("Robot connection broken")
		}
	}
}

// CleanupStaleConnections 오래된 연결 정리 (주기적으로 호출)
func (h *Handler) CleanupStaleConnections(timeout time.Duration) error {
	return h.statusManager.CleanupStaleConnections(timeout)
}

// GetOnlineRobots 온라인 로봇 목록 조회
func (h *Handler) GetOnlineRobots() ([]models.RobotStatus, error) {
	return h.statusManager.GetOnlineRobots()
}

// GetAllRobotStatuses 모든 로봇 상태 조회
func (h *Handler) GetAllRobotStatuses() ([]models.RobotStatus, error) {
	return h.statusManager.GetAllRobotStatuses()
}

// IsRobotOnline 특정 로봇의 온라인 상태 확인
func (h *Handler) IsRobotOnline(serialNumber string) bool {
	return h.statusManager.IsOnline(serialNumber)
}
