// internal/mqtt/position_handler.go (최종 수정본)
package mqtt

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
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type PositionHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	mqttClient  mqtt.Client
	config      *config.Config
}

func NewPositionHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *PositionHandler {
	return &PositionHandler{
		db:          db,
		redisClient: redisClient,
		mqttClient:  mqttClient,
		config:      cfg,
	}
}

// CheckAndRequestInitPosition 위치 초기화 필요 시 자동 요청 (panic 방어)
func (h *PositionHandler) CheckAndRequestInitPosition(stateMsg *models.RobotStateMessage) {
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
		utils.Logger.Debugf("Robot %s not in automatic mode (%s), skipping initPosition", stateMsg.SerialNumber, operatingMode)
		return
	}

	utils.Logger.Infof("Robot %s position not initialized, sending initPosition request", stateMsg.SerialNumber)

	if err := h.sendInitPositionRequest(stateMsg); err != nil {
		utils.Logger.Errorf("Failed to send initPosition request: %v", err)
	}
}

// sendInitPositionRequest initPosition 요청 전송 (panic 방어)
func (h *PositionHandler) sendInitPositionRequest(stateMsg *models.RobotStateMessage) error {
	if stateMsg == nil {
		return fmt.Errorf("state message is nil")
	}

	actionID := h.generateUniqueID()

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

	// 현재 위치를 기준으로 초기 위치 설정 (안전한 접근)
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
		"headerId":     utils.GetNextHeaderID(), // 1씩 증가하는 ID 사용
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

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	utils.Logger.Infof("InitPosition request sent successfully to robot: %s", serialNumber)
	return nil
}

// HandleFactsheet 팩트시트 응답 처리
func (h *PositionHandler) HandleFactsheet(client mqtt.Client, msg mqtt.Message) {
	var factsheetResp models.FactsheetResponse

	utils.Logger.Infof("Received factsheet response from topic: %s", msg.Topic())

	if err := json.Unmarshal(msg.Payload(), &factsheetResp); err != nil {
		utils.Logger.Errorf("Failed to parse factsheet response: %v", err)
		return
	}

	h.saveFactsheet(&factsheetResp)
}

// saveFactsheet 팩트시트 저장
func (h *PositionHandler) saveFactsheet(resp *models.FactsheetResponse) {
	if resp == nil {
		utils.Logger.Errorf("Factsheet response is nil")
		return
	}

	timestamp, _ := time.Parse(time.RFC3339, resp.Timestamp)
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// 안전한 필드 접근을 위한 헬퍼 함수들
	getStringField := func(field string) string {
		if field == "" {
			return "unknown"
		}
		return field
	}

	getIntField := func(field int) int {
		if field < 0 {
			return 0
		}
		return field
	}

	getFloatField := func(field float64) float64 {
		if field < 0 {
			return 0.0
		}
		return field
	}

	// TypeSpecification 안전 접근
	var seriesName, seriesDescription, agvClass, agvKinematics string
	var maxLoadMass int
	var localizationTypesJSON, navigationTypesJSON []byte

	if resp.TypeSpecification.SeriesName != "" {
		seriesName = resp.TypeSpecification.SeriesName
	} else {
		seriesName = "Unknown"
	}

	if resp.TypeSpecification.SeriesDescription != "" {
		seriesDescription = resp.TypeSpecification.SeriesDescription
	} else {
		seriesDescription = "No description available"
	}

	agvClass = getStringField(resp.TypeSpecification.AgvClass)
	agvKinematics = getStringField(resp.TypeSpecification.AgvKinematics)
	maxLoadMass = getIntField(resp.TypeSpecification.MaxLoadMass)

	// 배열 필드 안전 처리
	if len(resp.TypeSpecification.LocalizationTypes) > 0 {
		localizationTypesJSON, _ = json.Marshal(resp.TypeSpecification.LocalizationTypes)
	} else {
		localizationTypesJSON = []byte("[]")
	}

	if len(resp.TypeSpecification.NavigationTypes) > 0 {
		navigationTypesJSON, _ = json.Marshal(resp.TypeSpecification.NavigationTypes)
	} else {
		navigationTypesJSON = []byte("[]")
	}

	// PhysicalParameters 안전 접근
	speedMax := getFloatField(resp.PhysicalParameters.SpeedMax)
	speedMin := getFloatField(resp.PhysicalParameters.SpeedMin)
	accelerationMax := getFloatField(resp.PhysicalParameters.AccelerationMax)
	decelerationMax := getFloatField(resp.PhysicalParameters.DecelerationMax)
	length := getFloatField(resp.PhysicalParameters.Length)
	width := getFloatField(resp.PhysicalParameters.Width)
	heightMax := getFloatField(resp.PhysicalParameters.HeightMax)
	heightMin := getFloatField(resp.PhysicalParameters.HeightMin)

	factsheet := &models.RobotFactsheet{
		SerialNumber:      getStringField(resp.SerialNumber),
		Manufacturer:      getStringField(resp.Manufacturer),
		Version:           getStringField(resp.Version),
		SeriesName:        seriesName,
		SeriesDescription: seriesDescription,
		AgvClass:          agvClass,
		AgvKinematics:     agvKinematics,
		MaxLoadMass:       maxLoadMass,
		SpeedMax:          speedMax,
		SpeedMin:          speedMin,
		AccelerationMax:   accelerationMax,
		DecelerationMax:   decelerationMax,
		Length:            length,
		Width:             width,
		HeightMax:         heightMax,
		HeightMin:         heightMin,
		LocalizationTypes: string(localizationTypesJSON),
		NavigationTypes:   string(navigationTypesJSON),
		LastUpdated:       timestamp,
	}

	// 기존 팩트시트 확인
	var existing models.RobotFactsheet
	result := h.db.Where("serial_number = ?", resp.SerialNumber).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// 새로 생성
		if err := h.db.Create(factsheet).Error; err != nil {
			utils.Logger.Errorf("Failed to create factsheet: %v", err)
		} else {
			utils.Logger.Infof("Factsheet created for robot: %s (Series: %s, Class: %s, Kinematics: %s)",
				factsheet.SerialNumber, factsheet.SeriesName,
				factsheet.AgvClass, factsheet.AgvKinematics)
		}
	} else if result.Error == nil {
		// 기존 업데이트
		if err := h.db.Model(&existing).Updates(factsheet).Error; err != nil {
			utils.Logger.Errorf("Failed to update factsheet: %v", err)
		} else {
			utils.Logger.Infof("Factsheet updated for robot: %s (Series: %s, Class: %s, Kinematics: %s)",
				factsheet.SerialNumber, factsheet.SeriesName,
				factsheet.AgvClass, factsheet.AgvKinematics)
		}
	} else {
		utils.Logger.Errorf("Database error: %v", result.Error)
	}
}

// RequestFactsheet 팩트시트 요청 전송
func (h *PositionHandler) RequestFactsheet(serialNumber, manufacturer string) error {
	actionID := h.generateUniqueID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(), // 1씩 증가하는 ID 사용
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

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to send factsheet request: %v", token.Error())
	}

	utils.Logger.Infof("Factsheet request sent to robot: %s", serialNumber)
	return nil
}

// generateUniqueID 고유 ID 생성
func (h *PositionHandler) generateUniqueID() string {
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return fmt.Sprintf("%s_%d", hex.EncodeToString(randomBytes), time.Now().UnixNano())
}
