package mqtt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type MessageHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	mqttClient  mqtt.Client
	config      *config.Config
}

func NewMessageHandler(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *MessageHandler {
	return &MessageHandler{
		db:          db,
		redisClient: redisClient,
		mqttClient:  mqttClient,
		config:      cfg,
	}
}

// HandleCommand PLC에서 받은 명령 처리
func (h *MessageHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	commandType := strings.TrimSpace(string(msg.Payload()))

	utils.Logger.Infof("Received command from PLC: %s", commandType)

	// 명령 유효성 검사
	if !models.IsValidCommand(commandType) {
		utils.Logger.Errorf("Invalid command received: %s", commandType)
		return
	}

	// 실행 중인 명령이 있는지 확인
	var processingCount int64
	h.db.Model(&models.Command{}).Where("status = ?", models.StatusProcessing).Count(&processingCount)

	if processingCount > 0 {
		utils.Logger.Warnf("Command %s rejected: Another command is currently processing", commandType)

		// 거부된 명령을 데이터베이스에 기록
		rejectedCommand := &models.Command{
			CommandType:  commandType,
			Status:       models.StatusRejected,
			RequestTime:  time.Now(),
			ErrorMessage: "Command rejected: Another command is currently processing",
		}
		now := time.Now()
		rejectedCommand.ResponseTime = &now

		if err := h.db.Create(rejectedCommand).Error; err != nil {
			utils.Logger.Errorf("Failed to save rejected command to database: %v", err)
		}

		// PLC에 거부 응답 전송
		h.sendRejectedResponseToPLC(commandType)
		return
	}

	// 명령 정보를 데이터베이스에 저장
	command := &models.Command{
		CommandType: commandType,
		Status:      models.StatusPending,
		RequestTime: time.Now(),
	}

	if err := h.db.Create(command).Error; err != nil {
		utils.Logger.Errorf("Failed to save command to database: %v", err)
		return
	}

	// Redis에 상태 저장
	ctx := context.Background()
	commandKey := fmt.Sprintf("command:%d", command.ID)
	commandData, _ := json.Marshal(command)

	if err := h.redisClient.Set(ctx, commandKey, commandData, 30*time.Minute).Err(); err != nil {
		utils.Logger.Errorf("Failed to save command to Redis: %v", err)
	}

	utils.Logger.Infof("Command accepted: ID=%d, Type=%s", command.ID, command.CommandType)

	// 명령 처리 시작
	go h.processCommand(command)
}

// HandleRobotConnectionState 로봇 연결 상태 메시지 처리
func (h *MessageHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	var connMsg models.ConnectionStateMessage

	utils.Logger.Infof("Received robot connection state message from topic: %s", msg.Topic())
	utils.Logger.Debugf("Message payload: %s", string(msg.Payload()))

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
	robotStatus := &models.RobotStatus{
		Manufacturer:    connMsg.Manufacturer,
		SerialNumber:    connMsg.SerialNumber,
		ConnectionState: connMsg.ConnectionState,
		LastHeaderID:    connMsg.HeaderID,
		LastTimestamp:   timestamp,
		Version:         connMsg.Version,
	}

	// 기존 로봇 상태 확인
	var existingStatus models.RobotStatus
	result := h.db.Where("serial_number = ?", connMsg.SerialNumber).First(&existingStatus)

	if result.Error == gorm.ErrRecordNotFound {
		// 새로운 로봇 등록
		if err := h.db.Create(robotStatus).Error; err != nil {
			utils.Logger.Errorf("Failed to create robot status: %v", err)
			return
		}
		utils.Logger.Infof("New robot registered: %s (%s)", connMsg.SerialNumber, connMsg.Manufacturer)
	} else if result.Error == nil {
		// 기존 로봇 상태 업데이트
		existingStatus.ConnectionState = connMsg.ConnectionState
		existingStatus.LastHeaderID = connMsg.HeaderID
		existingStatus.LastTimestamp = timestamp
		existingStatus.Version = connMsg.Version

		if err := h.db.Save(&existingStatus).Error; err != nil {
			utils.Logger.Errorf("Failed to update robot status: %v", err)
			return
		}
		robotStatus = &existingStatus
	} else {
		utils.Logger.Errorf("Failed to query robot status: %v", result.Error)
		return
	}

	// Redis에 로봇 상태 저장
	ctx := context.Background()
	robotKey := fmt.Sprintf("robot:%s", connMsg.SerialNumber)
	robotData, _ := json.Marshal(robotStatus)

	if err := h.redisClient.Set(ctx, robotKey, robotData, 24*time.Hour).Err(); err != nil {
		utils.Logger.Errorf("Failed to save robot status to Redis: %v", err)
	}

	// 연결 상태별 처리
	h.handleConnectionStateChange(robotStatus)

	utils.Logger.Infof("Robot %s status updated: %s (HeaderID: %d)",
		connMsg.SerialNumber, connMsg.ConnectionState, connMsg.HeaderID)
}

// HandleRobotFactsheet 로봇 factsheet 응답 처리
func (h *MessageHandler) HandleRobotFactsheet(client mqtt.Client, msg mqtt.Message) {
	var factsheetResp models.FactsheetResponse

	utils.Logger.Infof("Received factsheet response from topic: %s", msg.Topic())
	utils.Logger.Debugf("Factsheet payload: %s", string(msg.Payload()))

	// JSON 파싱
	if err := json.Unmarshal(msg.Payload(), &factsheetResp); err != nil {
		utils.Logger.Errorf("Failed to parse factsheet response: %v", err)
		return
	}

	// 타임스탬프 파싱
	timestamp, err := time.Parse(time.RFC3339, factsheetResp.Timestamp)
	if err != nil {
		utils.Logger.Errorf("Failed to parse factsheet timestamp: %v", err)
		timestamp = time.Now()
	}

	// factsheet 정보를 데이터베이스에 저장/업데이트
	robotFactsheet := &models.RobotFactsheet{
		SerialNumber:      factsheetResp.SerialNumber,
		Manufacturer:      factsheetResp.Manufacturer,
		Version:           factsheetResp.Version,
		SeriesName:        factsheetResp.TypeSpecification.SeriesName,
		SeriesDescription: factsheetResp.TypeSpecification.SeriesDescription,
		AgvClass:          factsheetResp.TypeSpecification.AgvClass,
		MaxLoadMass:       factsheetResp.TypeSpecification.MaxLoadMass,
		SpeedMax:          factsheetResp.PhysicalParameters.SpeedMax,
		SpeedMin:          factsheetResp.PhysicalParameters.SpeedMin,
		AccelerationMax:   factsheetResp.PhysicalParameters.AccelerationMax,
		DecelerationMax:   factsheetResp.PhysicalParameters.DecelerationMax,
		Length:            factsheetResp.PhysicalParameters.Length,
		Width:             factsheetResp.PhysicalParameters.Width,
		HeightMax:         factsheetResp.PhysicalParameters.HeightMax,
		HeightMin:         factsheetResp.PhysicalParameters.HeightMin,
		LastUpdated:       timestamp,
	}

	// 기존 factsheet 확인
	var existingFactsheet models.RobotFactsheet
	result := h.db.Where("serial_number = ?", factsheetResp.SerialNumber).First(&existingFactsheet)

	if result.Error == gorm.ErrRecordNotFound {
		// 새로운 factsheet 생성
		if err := h.db.Create(robotFactsheet).Error; err != nil {
			utils.Logger.Errorf("Failed to create robot factsheet: %v", err)
			return
		}
		utils.Logger.Infof("New robot factsheet created: %s (%s)",
			factsheetResp.SerialNumber, factsheetResp.TypeSpecification.SeriesName)
	} else if result.Error == nil {
		// 기존 factsheet 업데이트
		existingFactsheet.Version = factsheetResp.Version
		existingFactsheet.SeriesName = factsheetResp.TypeSpecification.SeriesName
		existingFactsheet.SeriesDescription = factsheetResp.TypeSpecification.SeriesDescription
		existingFactsheet.AgvClass = factsheetResp.TypeSpecification.AgvClass
		existingFactsheet.MaxLoadMass = factsheetResp.TypeSpecification.MaxLoadMass
		existingFactsheet.SpeedMax = factsheetResp.PhysicalParameters.SpeedMax
		existingFactsheet.SpeedMin = factsheetResp.PhysicalParameters.SpeedMin
		existingFactsheet.AccelerationMax = factsheetResp.PhysicalParameters.AccelerationMax
		existingFactsheet.DecelerationMax = factsheetResp.PhysicalParameters.DecelerationMax
		existingFactsheet.Length = factsheetResp.PhysicalParameters.Length
		existingFactsheet.Width = factsheetResp.PhysicalParameters.Width
		existingFactsheet.HeightMax = factsheetResp.PhysicalParameters.HeightMax
		existingFactsheet.HeightMin = factsheetResp.PhysicalParameters.HeightMin
		existingFactsheet.LastUpdated = timestamp

		if err := h.db.Save(&existingFactsheet).Error; err != nil {
			utils.Logger.Errorf("Failed to update robot factsheet: %v", err)
			return
		}
		robotFactsheet = &existingFactsheet
	} else {
		utils.Logger.Errorf("Failed to query robot factsheet: %v", result.Error)
		return
	}

	// Redis에 factsheet 정보 저장
	ctx := context.Background()
	factsheetKey := fmt.Sprintf("factsheet:%s", factsheetResp.SerialNumber)
	factsheetData, _ := json.Marshal(robotFactsheet)

	if err := h.redisClient.Set(ctx, factsheetKey, factsheetData, 24*time.Hour).Err(); err != nil {
		utils.Logger.Errorf("Failed to save factsheet to Redis: %v", err)
	}

	utils.Logger.Infof("Robot factsheet updated: %s (Series: %s, Class: %s)",
		factsheetResp.SerialNumber,
		factsheetResp.TypeSpecification.SeriesName,
		factsheetResp.TypeSpecification.AgvClass)
}

// handleConnectionStateChange 연결 상태 변경에 따른 처리
func (h *MessageHandler) handleConnectionStateChange(robotStatus *models.RobotStatus) {
	switch robotStatus.ConnectionState {
	case models.ConnectionStateOnline:
		utils.Logger.Infof("Robot %s is now ONLINE", robotStatus.SerialNumber)
		h.onRobotOnline(robotStatus)

	case models.ConnectionStateOffline:
		utils.Logger.Warnf("Robot %s is now OFFLINE", robotStatus.SerialNumber)
		h.onRobotOffline(robotStatus)

	case models.ConnectionStateConnectionBroken:
		utils.Logger.Errorf("Robot %s connection is BROKEN", robotStatus.SerialNumber)
		h.onRobotConnectionBroken(robotStatus)
	}
}

// onRobotOnline 로봇이 온라인 상태가 될 때 처리
func (h *MessageHandler) onRobotOnline(robotStatus *models.RobotStatus) {
	utils.Logger.Infof("Robot %s is now online - ready to accept commands", robotStatus.SerialNumber)

	// 실행 중인 명령이 있는지 확인하여 상태 동기화
	var processingCount int64
	h.db.Model(&models.Command{}).Where("status = ?", models.StatusProcessing).Count(&processingCount)

	if processingCount > 0 {
		utils.Logger.Infof("Robot %s online: %d command(s) currently processing", robotStatus.SerialNumber, processingCount)
	} else {
		utils.Logger.Infof("Robot %s online: Ready to accept new commands", robotStatus.SerialNumber)
	}

	// 로봇이 온라인이 되면 factsheet 요청
	if err := h.RequestFactsheet(robotStatus.SerialNumber, robotStatus.Manufacturer); err != nil {
		utils.Logger.Errorf("Failed to request factsheet from robot %s: %v", robotStatus.SerialNumber, err)
	}
}

// onRobotOffline 로봇이 오프라인 상태가 될 때 처리
func (h *MessageHandler) onRobotOffline(robotStatus *models.RobotStatus) {
	// 진행 중인 명령이 있다면 실패로 처리
	var processingCommands []models.Command
	h.db.Where("status = ?", models.StatusProcessing).Find(&processingCommands)

	for _, command := range processingCommands {
		utils.Logger.Warnf("Command %d failed due to robot offline: %s", command.ID, robotStatus.SerialNumber)

		// 명령을 실패 상태로 변경
		command.Status = models.StatusFailure
		command.ErrorMessage = fmt.Sprintf("Robot went offline during processing: %s", robotStatus.SerialNumber)
		now := time.Now()
		command.ResponseTime = &now

		if err := h.db.Save(&command).Error; err != nil {
			utils.Logger.Errorf("Failed to update command status: %v", err)
			continue
		}

		// Redis에서 명령 상태 업데이트
		h.updateCommandStatusInRedis(&command)

		// PLC에 실패 응답 전송
		h.sendResponseToPLC(&command)
	}

	if len(processingCommands) > 0 {
		utils.Logger.Warnf("Failed %d commands due to robot offline: %s",
			len(processingCommands), robotStatus.SerialNumber)
	}
}

// onRobotConnectionBroken 로봇 연결이 끊어질 때 처리
func (h *MessageHandler) onRobotConnectionBroken(robotStatus *models.RobotStatus) {
	// 진행 중인 명령을 실패 처리
	var processingCommands []models.Command
	h.db.Where("status = ?", models.StatusProcessing).Find(&processingCommands)

	for _, command := range processingCommands {
		command.Status = models.StatusFailure
		command.ErrorMessage = fmt.Sprintf("Robot connection broken: %s", robotStatus.SerialNumber)
		now := time.Now()
		command.ResponseTime = &now

		if err := h.db.Save(&command).Error; err != nil {
			utils.Logger.Errorf("Failed to update command status: %v", err)
			continue
		}

		// Redis 상태 업데이트
		h.updateCommandStatusInRedis(&command)

		// PLC에 실패 응답 전송
		h.sendResponseToPLC(&command)

		utils.Logger.Errorf("Command %d failed due to robot connection broken: %s",
			command.ID, robotStatus.SerialNumber)
	}

	if len(processingCommands) > 0 {
		utils.Logger.Errorf("Failed %d commands due to robot connection broken: %s",
			len(processingCommands), robotStatus.SerialNumber)
	}
}

// processCommand 명령 처리 로직
func (h *MessageHandler) processCommand(command *models.Command) {
	utils.Logger.Infof("Processing command: %s (ID: %d)", command.CommandType, command.ID)

	// 설정에서 로봇 정보 가져오기
	robotSerial := h.config.RobotSerialNumber
	ctx := context.Background()
	robotKey := fmt.Sprintf("robot:%s", robotSerial)
	robotData, err := h.redisClient.Get(ctx, robotKey).Result()

	var robotOnline bool = false

	if err == nil {
		var robotStatus models.RobotStatus
		if json.Unmarshal([]byte(robotData), &robotStatus) == nil {
			robotOnline = robotStatus.ConnectionState == models.ConnectionStateOnline
		}
	} else {
		utils.Logger.Warnf("Could not get robot status from Redis: %v", err)
		// Redis에서 상태를 가져올 수 없는 경우 DB에서 확인
		var robotStatus models.RobotStatus
		if h.db.Where("serial_number = ?", robotSerial).First(&robotStatus).Error == nil {
			robotOnline = robotStatus.ConnectionState == models.ConnectionStateOnline
		}
	}

	if !robotOnline {
		utils.Logger.Errorf("Robot %s is not online, failing command: %d", robotSerial, command.ID)

		// 로봇이 오프라인이면 명령 실패 처리
		command.Status = models.StatusFailure
		command.ErrorMessage = fmt.Sprintf("Robot %s is not online", robotSerial)
		now := time.Now()
		command.ResponseTime = &now

		h.updateCommandStatus(command)
		h.sendResponseToPLC(command)
		return
	}

	// 상태를 처리중으로 변경
	command.Status = models.StatusProcessing
	h.updateCommandStatus(command)

	utils.Logger.Infof("Command %d status changed to PROCESSING", command.ID)

	// 실제 로봇 명령 처리 시뮬레이션
	// TODO: 여기서 실제 로봇 API 호출로 대체해야 함
	processingTime := 2 * time.Second
	utils.Logger.Infof("Simulating robot operation for command %d (estimated %v)", command.ID, processingTime)
	time.Sleep(processingTime)

	// 명령 타입에 따른 처리 결과 결정 (시뮬레이션)
	var finalStatus string
	switch command.CommandType {
	case models.CommandCataractRemoval, models.CommandGlaucomaRemoval:
		finalStatus = models.StatusSuccess // 실제로는 로봇 응답에 따라 결정
	case models.CommandGripperCleaning, models.CommandCameraCleaning, models.CommandKnifeCleaning:
		finalStatus = models.StatusSuccess
	case models.CommandCameraCheck:
		// 시뮬레이션: 80% 확률로 정상
		if time.Now().Unix()%5 != 0 {
			finalStatus = models.StatusNormal
		} else {
			finalStatus = models.StatusAbnormal
		}
	default:
		finalStatus = models.StatusFailure
	}

	// 최종 상태 업데이트
	command.Status = finalStatus
	now := time.Now()
	command.ResponseTime = &now
	h.updateCommandStatus(command)

	utils.Logger.Infof("Command %d completed with status: %s", command.ID, finalStatus)

	// PLC에 응답 전송
	h.sendResponseToPLC(command)
}

// RequestFactsheet 로봇에 factsheet 요청 전송
func (h *MessageHandler) RequestFactsheet(serialNumber, manufacturer string) error {
	utils.Logger.Infof("Requesting factsheet from robot: %s", serialNumber)

	// 고유한 액션 ID 생성
	actionID, err := h.generateActionID()
	if err != nil {
		return fmt.Errorf("failed to generate action ID: %v", err)
	}

	// factsheet 요청 메시지 생성
	factsheetReq := models.FactsheetRequest{
		HeaderID:     time.Now().Unix(),
		Timestamp:    time.Now().Format(time.RFC3339Nano),
		Version:      "2.0.0",
		Manufacturer: manufacturer,
		SerialNumber: serialNumber,
		Actions: []models.Action{
			{
				ActionType:       "factsheetRequest",
				ActionID:         actionID,
				BlockingType:     "NONE",
				ActionParameters: []models.ActionParameter{},
			},
		},
	}

	// JSON 직렬화
	reqData, err := json.Marshal(factsheetReq)
	if err != nil {
		return fmt.Errorf("failed to marshal factsheet request: %v", err)
	}

	// 토픽 생성
	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", manufacturer, serialNumber)

	utils.Logger.Infof("Sending factsheet request to topic: %s", topic)
	utils.Logger.Debugf("Factsheet request payload: %s", string(reqData))

	// MQTT 메시지 전송
	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to send factsheet request: %v", token.Error())
	}

	utils.Logger.Infof("Factsheet request sent successfully to robot: %s", serialNumber)
	return nil
}

// generateActionID 고유한 액션 ID 생성
func (h *MessageHandler) generateActionID() (string, error) {
	// 랜덤 바이트 생성
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// 현재 타임스탬프와 랜덤 문자열 결합
	timestamp := time.Now().UnixNano()
	randomString := hex.EncodeToString(randomBytes)

	return fmt.Sprintf("%s_%d", randomString, timestamp), nil
}

// updateCommandStatus 명령 상태 업데이트 (DB + Redis)
func (h *MessageHandler) updateCommandStatus(command *models.Command) {
	// 데이터베이스 업데이트
	if err := h.db.Save(command).Error; err != nil {
		utils.Logger.Errorf("Failed to update command status in database: %v", err)
	}

	// Redis 업데이트
	h.updateCommandStatusInRedis(command)
}

// updateCommandStatusInRedis Redis에서 명령 상태 업데이트
func (h *MessageHandler) updateCommandStatusInRedis(command *models.Command) {
	ctx := context.Background()
	commandKey := fmt.Sprintf("command:%d", command.ID)
	commandData, _ := json.Marshal(command)

	if err := h.redisClient.Set(ctx, commandKey, commandData, 30*time.Minute).Err(); err != nil {
		utils.Logger.Errorf("Failed to update command status in Redis: %v", err)
	}
}

// sendResponseToPLC PLC에 응답 전송
func (h *MessageHandler) sendResponseToPLC(command *models.Command) {
	responseCode := command.GetResponseCode()
	topic := "bridge/response"

	utils.Logger.Infof("Sending response to PLC: %s (Command ID: %d)", responseCode, command.ID)

	token := h.mqttClient.Publish(topic, 0, false, responseCode)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())

		// 전송 실패 시 에러 로그에 추가 정보 기록
		command.ErrorMessage = fmt.Sprintf("Failed to send response to PLC: %v", token.Error())
		h.db.Save(command)
	} else {
		utils.Logger.Infof("Response sent successfully to PLC: %s", responseCode)
	}
}

// sendRejectedResponseToPLC 거부된 명령에 대한 응답 전송
func (h *MessageHandler) sendRejectedResponseToPLC(commandType string) {
	responseCode := commandType + ":R" // R for Rejected
	topic := "bridge/response"

	utils.Logger.Infof("Sending rejection response to PLC: %s", responseCode)

	token := h.mqttClient.Publish(topic, 0, false, responseCode)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send rejection response to PLC: %v", token.Error())
	} else {
		utils.Logger.Infof("Rejection response sent successfully to PLC: %s", responseCode)
	}
}
