// internal/position/manager.go
package position

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"
)

// Manager 위치 관리
type Manager struct {
	db         *gorm.DB
	mqttClient mqtt.Client
	config     *config.Config
}

// NewManager 새 위치 관리자 생성
func NewManager(db *gorm.DB, mqttClient mqtt.Client, cfg *config.Config) *Manager {
	return &Manager{
		db:         db,
		mqttClient: mqttClient,
		config:     cfg,
	}
}

// CheckAndRequestInitPosition 위치 초기화 필요 시 자동 요청
func (m *Manager) CheckAndRequestInitPosition(stateMsg *models.RobotStateMessage) {
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

	if operatingMode != models.OperatingModeAutomatic {
		utils.Logger.Debugf("Robot %s not in automatic mode (%s), skipping initPosition",
			stateMsg.SerialNumber, operatingMode)
		return
	}

	utils.Logger.Infof("Robot %s position not initialized, sending initPosition request",
		stateMsg.SerialNumber)

	if err := m.sendInitPositionRequest(stateMsg); err != nil {
		utils.Logger.Errorf("Failed to send initPosition request: %v", err)
	}
}

// RequestFactsheet 팩트시트 요청 전송
func (m *Manager) RequestFactsheet(serialNumber, manufacturer string) error {
	actionID := m.generateUniqueID()

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
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	reqData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal factsheet request: %v", err)
	}

	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", manufacturer, serialNumber)

	utils.Logger.Infof("Sending factsheet request to %s", topic)

	token := m.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to send factsheet request: %v", token.Error())
	}

	utils.Logger.Infof("Factsheet request sent to robot: %s", serialNumber)
	return nil
}

// GetCurrentPosition 현재 위치 조회 (로봇 상태에서)
func (m *Manager) GetCurrentPosition(serialNumber string) (*models.AgvPosition, error) {
	// 실제로는 최신 상태 메시지에서 위치 정보를 가져와야 함
	// 여기서는 기본 구현만 제공
	return &models.AgvPosition{
		X:                   0.0,
		Y:                   0.0,
		Theta:               0.0,
		MapID:               "",
		PositionInitialized: false,
	}, nil
}

// ValidatePosition 위치 유효성 검사
func (m *Manager) ValidatePosition(position *models.AgvPosition) error {
	if position == nil {
		return fmt.Errorf("position is nil")
	}

	// 기본 유효성 검사
	if position.MapID == "" {
		utils.Logger.Warnf("Map ID is empty")
	}

	// 위치 범위 검사 (필요에 따라 추가)
	// if math.Abs(position.X) > MAX_X || math.Abs(position.Y) > MAX_Y {
	//     return fmt.Errorf("position out of bounds")
	// }

	return nil
}

// sendInitPositionRequest initPosition 요청 전송
func (m *Manager) sendInitPositionRequest(stateMsg *models.RobotStateMessage) error {
	if stateMsg == nil {
		return fmt.Errorf("state message is nil")
	}

	actionID := m.generateUniqueID()

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
				"actionType":   "initPosition",
				"actionId":     actionID,
				"blockingType": "NONE",
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

	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", manufacturer, serialNumber)

	utils.Logger.Infof("Sending initPosition request to %s (ActionID: %s)", topic, actionID)
	utils.Logger.Debugf("Request payload: %s", string(reqData))

	token := m.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	utils.Logger.Infof("InitPosition request sent successfully to robot: %s", serialNumber)
	return nil
}

// generateUniqueID 고유 ID 생성
func (m *Manager) generateUniqueID() string {
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return fmt.Sprintf("%s_%d", hex.EncodeToString(randomBytes), time.Now().UnixNano())
}

// IsPositionInitialized 위치 초기화 여부 확인
func (m *Manager) IsPositionInitialized(serialNumber string) (bool, error) {
	// 실제로는 최신 상태에서 확인해야 함
	// 여기서는 기본 구현만 제공
	return false, nil
}

// GetPositionHistory 위치 이력 조회 (필요시 구현)
func (m *Manager) GetPositionHistory(serialNumber string, limit int) ([]models.AgvPosition, error) {
	// 위치 이력을 별도 테이블에 저장한다면 여기서 조회
	return []models.AgvPosition{}, nil
}

// SetDefaultPosition 기본 위치 설정
func (m *Manager) SetDefaultPosition(serialNumber string, position *models.AgvPosition) error {
	// 로봇별 기본 위치를 DB에 저장하는 로직
	// 현재는 기본 구현만 제공
	return nil
}
