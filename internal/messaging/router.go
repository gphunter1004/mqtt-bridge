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
}

// PositionHandler 위치 처리 인터페이스
type PositionHandler interface {
	CheckAndRequestInitPosition(stateMsg *models.RobotStateMessage)
	HandleFactsheet(client mqtt.Client, msg mqtt.Message)
}

// WorkflowHandler 워크플로우 처리 인터페이스
type WorkflowHandler interface {
	HandleOrderStateUpdate(stateMsg *models.RobotStateMessage)
}

// Router 메시지 라우터
type Router struct {
	commandHandler  CommandHandler
	robotHandler    RobotHandler
	positionHandler PositionHandler
	workflowHandler WorkflowHandler
}

// NewRouter 새 메시지 라우터 생성
func NewRouter(commandHandler CommandHandler, robotHandler RobotHandler,
	positionHandler PositionHandler, workflowHandler WorkflowHandler) *Router {

	utils.Logger.Infof("🏗️ CREATING Message Router")

	router := &Router{
		commandHandler:  commandHandler,
		robotHandler:    robotHandler,
		positionHandler: positionHandler,
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
		// PLC 명령
		r.commandHandler.HandlePLCCommand(client, msg)

	case strings.Contains(topic, "/connection"):
		// 로봇 연결 상태
		r.robotHandler.HandleConnectionState(client, msg)

	case strings.Contains(topic, "/state"):
		// 로봇 상태 (여러 핸들러에서 처리)
		r.handleRobotState(client, msg)

	case strings.Contains(topic, "/factsheet"):
		// 팩트시트 (로봇과 위치 핸들러 모두 처리)
		r.robotHandler.HandleFactsheet(client, msg)
		r.positionHandler.HandleFactsheet(client, msg)

	default:
		utils.Logger.Warnf("Unhandled topic: %s", topic)
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

	if r.positionHandler != nil {
		r.positionHandler.CheckAndRequestInitPosition(&stateMsg)
	}
}

// GetCommandHandler 명령 핸들러 반환
func (r *Router) GetCommandHandler() CommandHandler {
	return r.commandHandler
}

// GetRobotHandler 로봇 핸들러 반환
func (r *Router) GetRobotHandler() RobotHandler {
	return r.robotHandler
}

// GetPositionHandler 위치 핸들러 반환
func (r *Router) GetPositionHandler() PositionHandler {
	return r.positionHandler
}

// GetWorkflowHandler 워크플로우 핸들러 반환
func (r *Router) GetWorkflowHandler() WorkflowHandler {
	return r.workflowHandler
}
