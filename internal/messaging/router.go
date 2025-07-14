// internal/messaging/router.go
package messaging

import (
	"encoding/json"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// CommandHandler 명령 처리 인터페이스
type CommandHandler interface {
	HandlePLCCommand(client mqtt.Client, msg mqtt.Message)
	HandleRobotStateUpdate(stateMsg *models.RobotStateMessage)
}

// RobotHandler 로봇 처리 인터페이스
type RobotHandler interface {
	HandleConnectionState(client mqtt.Client, msg mqtt.Message)
	HandleRobotState(client mqtt.Client, msg mqtt.Message)
	HandleFactsheet(client mqtt.Client, msg mqtt.Message)
	CheckAndRequestInitPosition(stateMsg *models.RobotStateMessage)
}

// WorkflowHandler 워크플로우 처리 인터페이스
type WorkflowHandler interface {
	HandleOrderStateUpdate(stateMsg *models.RobotStateMessage)
}

// Router 메시지 라우터
type Router struct {
	commandHandler  CommandHandler
	robotHandler    RobotHandler
	workflowHandler WorkflowHandler
}

// NewRouter 새 메시지 라우터 생성
func NewRouter(commandHandler CommandHandler, robotHandler RobotHandler, workflowHandler WorkflowHandler) *Router {
	utils.Logger.Infof("🏗️ CREATING Message Router")

	router := &Router{
		commandHandler:  commandHandler,
		robotHandler:    robotHandler,
		workflowHandler: workflowHandler,
	}

	utils.Logger.Infof("✅ Message Router CREATED")
	return router
}

// RouteMessage 토픽에 따라 메시지 라우팅
func (r *Router) RouteMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	utils.Logger.Debugf("Routing message from topic: %s", topic)

	// 토픽 패턴에 따라 라우팅
	switch {
	case topic == "bridge/command":
		utils.Logger.Infof("🎯 ROUTING to Command Handler")
		r.commandHandler.HandlePLCCommand(client, msg)

	case strings.Contains(topic, "/connection"):
		utils.Logger.Infof("🔗 ROUTING to Robot Connection Handler")
		r.robotHandler.HandleConnectionState(client, msg)

	case strings.Contains(topic, "/state"):
		utils.Logger.Infof("📊 ROUTING to Robot State Handler")
		r.handleRobotState(client, msg)

	case strings.Contains(topic, "/factsheet"):
		utils.Logger.Infof("📋 ROUTING to Robot Factsheet Handler")
		r.robotHandler.HandleFactsheet(client, msg)

	case strings.Contains(topic, "/order"):
		// Order 메시지는 이미 Subscriber에서 로그했으므로 간소화
		utils.Logger.Infof("📦 ROUTING to Order Handler (log only)")

	default:
		utils.Logger.Warnf("❓ UNHANDLED topic: %s", topic)
	}
}

// handleRobotState 로봇 상태 메시지를 여러 핸들러에 분배
func (r *Router) handleRobotState(client mqtt.Client, msg mqtt.Message) {
	// 로봇 핸들러에서 기본 처리
	r.robotHandler.HandleRobotState(client, msg)

	// 상태 메시지 파싱
	var stateMsg models.RobotStateMessage
	if err := json.Unmarshal(msg.Payload(), &stateMsg); err != nil {
		utils.Logger.Errorf("Failed to parse robot state message: %v", err)
		return
	}

	// 각 핸들러에 상태 업데이트 전달
	if r.commandHandler != nil {
		r.commandHandler.HandleRobotStateUpdate(&stateMsg)
	}

	if r.workflowHandler != nil {
		r.workflowHandler.HandleOrderStateUpdate(&stateMsg)
	}

	if r.robotHandler != nil {
		r.robotHandler.CheckAndRequestInitPosition(&stateMsg)
	}
}
