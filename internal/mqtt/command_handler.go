package mqtt

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/database"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type CommandHandler struct {
	db              *gorm.DB
	redisClient     *redis.Client
	mqttClient      mqtt.Client
	config          *config.Config
	workflowManager *WorkflowManager
	plcNotifier     *PLCNotifier
	processingMutex sync.Mutex
}

func NewCommandHandler(
	db *gorm.DB,
	redisClient *redis.Client,
	mqttClient mqtt.Client,
	cfg *config.Config,
	plcNotifier *PLCNotifier,
) *CommandHandler {
	return &CommandHandler{
		db:              db,
		redisClient:     redisClient,
		mqttClient:      mqttClient,
		config:          cfg,
		workflowManager: NewWorkflowManager(db, redisClient, mqttClient, cfg, plcNotifier),
		plcNotifier:     plcNotifier,
	}
}

// HandleCommand PLC 명령 처리
func (h *CommandHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	payload := strings.TrimSpace(string(msg.Payload()))
	utils.Logger.Infof("Received command from PLC: %s", payload)

	// 명령 파싱 (형식: "CR:DEX0002" 또는 "CR")
	commandType, robotSerial := h.parseCommand(payload)

	// 로봇 시리얼이 없으면 기본값 사용
	if robotSerial == "" {
		robotSerial = h.config.RobotSerialNumber
	}

	// 로봇 상태 확인
	robotStatus, err := h.checkRobotAvailability(robotSerial)
	if err != nil {
		h.plcNotifier.SendResponse(commandType, "F", err.Error())
		return
	}

	// 로봇이 바쁜 경우 즉시 거부
	if robotStatus.IsBusy {
		h.handleBusyRobot(commandType, robotSerial, robotStatus.CurrentCommandID)
		return
	}

	// 명령 처리 시작
	h.processCommand(commandType, robotSerial)
}

// parseCommand 명령 파싱
func (h *CommandHandler) parseCommand(payload string) (commandType, robotSerial string) {
	parts := strings.Split(payload, ":")
	commandType = parts[0]

	if len(parts) > 1 {
		robotSerial = parts[1]
	}

	return commandType, robotSerial
}

// checkRobotAvailability 로봇 가용성 확인
func (h *CommandHandler) checkRobotAvailability(robotSerial string) (*models.RobotStatus, error) {
	var robotStatus models.RobotStatus

	err := h.db.Where("serial_number = ?", robotSerial).First(&robotStatus).Error
	if err != nil {
		return nil, fmt.Errorf("robot %s not found", robotSerial)
	}

	if robotStatus.ConnectionState != models.ConnectionStateOnline {
		return nil, fmt.Errorf("robot %s is not online", robotSerial)
	}

	return &robotStatus, nil
}

// handleBusyRobot 로봇이 바쁜 경우 처리
func (h *CommandHandler) handleBusyRobot(commandType, robotSerial string, currentCommandID *uint) {
	reason := fmt.Sprintf("Robot %s is busy", robotSerial)

	if currentCommandID != nil {
		reason = fmt.Sprintf("Robot %s is busy with command %d", robotSerial, *currentCommandID)
	}

	// 거부 응답 전송
	h.plcNotifier.SendResponse(commandType, "R", reason)

	// 명령 기록
	now := time.Now()
	command := &models.Command{
		CommandType:       commandType,
		RobotSerialNumber: robotSerial,
		Status:            models.StatusRejected,
		RejectionReason:   reason,
		RequestTime:       now,
		ResponseTime:      &now,
		WorkflowConfig:    models.JSON{},
	}
	h.db.Create(command)

	utils.Logger.Warnf("Command %s rejected: %s", commandType, reason)
}

// processCommand 명령 처리
func (h *CommandHandler) processCommand(commandType, robotSerial string) {
	// 워크플로우 설정 가져오기
	workflowConfig := database.GetWorkflowConfig(commandType)

	// 총 단계 수 계산
	steps := workflowConfig["steps"].([]interface{})
	totalSteps := len(steps)

	// 명령 생성
	command := &models.Command{
		CommandType:       commandType,
		RobotSerialNumber: robotSerial,
		WorkflowConfig:    workflowConfig,
		Status:            models.StatusPending,
		TotalSteps:        totalSteps,
		CurrentStep:       0,
		RequestTime:       time.Now(),
	}

	if err := h.db.Create(command).Error; err != nil {
		utils.Logger.Errorf("Failed to create command: %v", err)
		h.plcNotifier.SendResponse(commandType, "F", "Database error")
		return
	}

	// 로봇 상태를 BUSY로 변경
	h.processingMutex.Lock()
	err := h.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", robotSerial).
		Updates(map[string]interface{}{
			"is_busy":            true,
			"current_command_id": command.ID,
		}).Error
	h.processingMutex.Unlock()

	if err != nil {
		utils.Logger.Errorf("Failed to update robot status: %v", err)
		h.db.Delete(command)
		h.plcNotifier.SendResponse(commandType, "F", "Failed to lock robot")
		return
	}

	// 수락 응답 전송
	h.plcNotifier.SendResponse(commandType, "A", "")
	h.plcNotifier.SendStatus(command)

	utils.Logger.Infof("Command accepted: ID=%d, Type=%s, Robot=%s",
		command.ID, commandType, robotSerial)

	// 특수 명령 처리
	if commandType == models.CommandOrderCancel {
		h.handleCancelCommand(command)
		return
	}

	// 비동기로 워크플로우 실행
	go h.workflowManager.ExecuteWorkflow(command)
}

// handleCancelCommand 취소 명령 처리
func (h *CommandHandler) handleCancelCommand(command *models.Command) {
	utils.Logger.Infof("Processing order cancel command")

	// 현재 실행 중인 모든 명령 취소
	err := h.workflowManager.CancelAllActiveOrders(command.RobotSerialNumber)

	// 명령 상태 업데이트
	now := time.Now()
	status := models.StatusSuccess
	errorMsg := ""

	if err != nil {
		status = models.StatusFailure
		errorMsg = err.Error()
	}

	h.db.Model(command).Updates(map[string]interface{}{
		"status":        status,
		"response_time": now,
		"error_message": errorMsg,
		"current_step":  1,
		"total_steps":   1,
	})

	// 로봇 상태 해제
	h.releaseRobot(command.RobotSerialNumber)

	// 최종 응답
	if err != nil {
		h.plcNotifier.SendResponse(command.CommandType, "F", errorMsg)
	} else {
		h.plcNotifier.SendResponse(command.CommandType, "S", "")
	}
}

// releaseRobot 로봇 상태 해제
func (h *CommandHandler) releaseRobot(robotSerial string) {
	h.processingMutex.Lock()
	defer h.processingMutex.Unlock()

	err := h.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", robotSerial).
		Updates(map[string]interface{}{
			"is_busy":            false,
			"current_command_id": nil,
			"current_order_id":   "",
			"last_action_status": "",
		}).Error

	if err != nil {
		utils.Logger.Errorf("Failed to release robot %s: %v", robotSerial, err)
	}
}

// FailCommandsForRobot 로봇 연결 끊김 등으로 명령 실패 처리
func (h *CommandHandler) FailCommandsForRobot(robotSerial string, reason string) {
	var activeCommands []models.Command

	h.db.Where("robot_serial_number = ? AND status = ?",
		robotSerial, models.StatusProcessing).Find(&activeCommands)

	for _, cmd := range activeCommands {
		now := time.Now()
		h.db.Model(&cmd).Updates(map[string]interface{}{
			"status":        models.StatusFailure,
			"response_time": now,
			"error_message": reason,
		})

		h.plcNotifier.SendResponse(cmd.CommandType, "F", reason)
		h.plcNotifier.SendStatus(&cmd)
	}

	h.releaseRobot(robotSerial)
}
