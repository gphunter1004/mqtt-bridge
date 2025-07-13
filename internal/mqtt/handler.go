package mqtt

import (
	"mqtt-bridge/internal/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// MessageHandler 모든 MQTT 메시지 처리를 담당하는 메인 핸들러
type MessageHandler struct {
	commandHandler  *CommandHandler
	robotHandler    *RobotHandler
	workflowManager *WorkflowManager
	plcNotifier     *PLCNotifier
}

// NewMessageHandler 메시지 핸들러 생성
func NewMessageHandler(
	db *gorm.DB,
	redisClient *redis.Client,
	mqttClient mqtt.Client,
	cfg *config.Config,
) *MessageHandler {
	// PLC 알림 전송자 생성
	plcNotifier := NewPLCNotifier(mqttClient, cfg, db)

	// 워크플로우 매니저 생성 (기존 OrderExecutor 대체)
	workflowManager := NewWorkflowManager(db, redisClient, mqttClient, cfg, plcNotifier)

	// 명령 핸들러 생성
	commandHandler := NewCommandHandler(db, redisClient, mqttClient, cfg, plcNotifier)
	commandHandler.workflowManager = workflowManager

	// 로봇 핸들러 생성
	robotHandler := NewRobotHandler(db, redisClient, mqttClient, cfg,
		commandHandler, workflowManager, plcNotifier)

	return &MessageHandler{
		commandHandler:  commandHandler,
		robotHandler:    robotHandler,
		workflowManager: workflowManager,
		plcNotifier:     plcNotifier,
	}
}

// HandleCommand PLC 명령 처리
func (h *MessageHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	h.commandHandler.HandleCommand(client, msg)
}

// HandleRobotConnectionState 로봇 연결 상태 처리
func (h *MessageHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleRobotConnectionState(client, msg)
}

// HandleRobotState 로봇 상태 처리
func (h *MessageHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleRobotState(client, msg)
}

// HandleRobotFactsheet 로봇 팩트시트 처리
func (h *MessageHandler) HandleRobotFactsheet(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleFactsheet(client, msg)
}
