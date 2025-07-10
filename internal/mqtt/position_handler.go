// internal/mqtt/position_handler.go
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

// CheckAndRequestInitPosition 위치 초기화 필요 시 자동 요청
func (h *PositionHandler) CheckAndRequestInitPosition(stateMsg *models.RobotStateMessage) {
	// 이미 초기화되었으면 무시
	if stateMsg.AgvPosition.PositionInitialized {
		return
	}

	// 자동 모드에서만 처리
	if stateMsg.OperatingMode != models.OperatingModeAutomatic {
		utils.Logger.Debugf("Robot %s not in automatic mode, skipping initPosition", stateMsg.SerialNumber)
		return
	}

	utils.Logger.Infof("Robot %s position not initialized, sending initPosition request", stateMsg.SerialNumber)

	if err := h.sendInitPositionRequest(stateMsg); err != nil {
		utils.Logger.Errorf("Failed to send initPosition request: %v", err)
	}
}

// sendInitPositionRequest initPosition 요청 전송
func (h *PositionHandler) sendInitPositionRequest(stateMsg *models.RobotStateMessage) error {
	actionID := h.generateUniqueID()

	// 현재 위치를 기준으로 초기 위치 설정
	pose := map[string]interface{}{
		"lastNodeId": "",
		"mapId":      stateMsg.AgvPosition.MapID,
		"theta":      stateMsg.AgvPosition.Theta,
		"x":          stateMsg.AgvPosition.X,
		"y":          stateMsg.AgvPosition.Y,
	}

	// 위치가 모두 0이면 원점 사용
	if stateMsg.AgvPosition.X == 0 && stateMsg.AgvPosition.Y == 0 && stateMsg.AgvPosition.Theta == 0 {
		pose["x"] = 0.0
		pose["y"] = 0.0
		pose["theta"] = 0.0
		utils.Logger.Infof("Using origin position for robot %s", stateMsg.SerialNumber)
	}

	// 요청 메시지 생성
	request := map[string]interface{}{
		"headerId":     time.Now().Unix(),
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
	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", stateMsg.Manufacturer, stateMsg.SerialNumber)

	utils.Logger.Infof("Sending initPosition request to %s (ActionID: %s)", topic, actionID)
	utils.Logger.Debugf("Request payload: %s", string(reqData))

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	utils.Logger.Infof("InitPosition request sent successfully to robot: %s", stateMsg.SerialNumber)
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
	timestamp, _ := time.Parse(time.RFC3339, resp.Timestamp)
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	factsheet := &models.RobotFactsheet{
		SerialNumber:      resp.SerialNumber,
		Manufacturer:      resp.Manufacturer,
		Version:           resp.Version,
		SeriesName:        resp.TypeSpecification.SeriesName,
		SeriesDescription: resp.TypeSpecification.SeriesDescription,
		AgvClass:          resp.TypeSpecification.AgvClass,
		MaxLoadMass:       resp.TypeSpecification.MaxLoadMass,
		SpeedMax:          resp.PhysicalParameters.SpeedMax,
		SpeedMin:          resp.PhysicalParameters.SpeedMin,
		AccelerationMax:   resp.PhysicalParameters.AccelerationMax,
		DecelerationMax:   resp.PhysicalParameters.DecelerationMax,
		Length:            resp.PhysicalParameters.Length,
		Width:             resp.PhysicalParameters.Width,
		HeightMax:         resp.PhysicalParameters.HeightMax,
		HeightMin:         resp.PhysicalParameters.HeightMin,
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
			utils.Logger.Infof("Factsheet created for robot: %s", resp.SerialNumber)
		}
	} else if result.Error == nil {
		// 기존 업데이트
		if err := h.db.Model(&existing).Updates(factsheet).Error; err != nil {
			utils.Logger.Errorf("Failed to update factsheet: %v", err)
		} else {
			utils.Logger.Infof("Factsheet updated for robot: %s", resp.SerialNumber)
		}
	} else {
		utils.Logger.Errorf("Database error: %v", result.Error)
	}
}

// RequestFactsheet 팩트시트 요청 전송
func (h *PositionHandler) RequestFactsheet(serialNumber, manufacturer string) error {
	actionID := h.generateUniqueID()

	request := map[string]interface{}{
		"headerId":     time.Now().Unix(),
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
