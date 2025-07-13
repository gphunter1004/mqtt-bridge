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
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// WorkflowManager 워크플로우 실행 관리자 (기존 OrderExecutor 대체)
type WorkflowManager struct {
	db          *gorm.DB
	redisClient *redis.Client
	mqttClient  mqtt.Client
	config      *config.Config
	plcNotifier *PLCNotifier
	ctx         context.Context
}

func NewWorkflowManager(
	db *gorm.DB,
	redisClient *redis.Client,
	mqttClient mqtt.Client,
	cfg *config.Config,
	plcNotifier *PLCNotifier,
) *WorkflowManager {
	return &WorkflowManager{
		db:          db,
		redisClient: redisClient,
		mqttClient:  mqttClient,
		config:      cfg,
		plcNotifier: plcNotifier,
		ctx:         context.Background(),
	}
}

// ExecuteWorkflow 워크플로우 실행
func (m *WorkflowManager) ExecuteWorkflow(command *models.Command) {
	utils.Logger.Infof("Starting workflow execution for command %d", command.ID)

	// 명령 상태를 PROCESSING으로 변경
	m.updateCommandStatus(command, models.StatusProcessing, 0)

	// 워크플로우 단계들 가져오기
	steps, ok := command.WorkflowConfig["steps"].([]interface{})
	if !ok || len(steps) == 0 {
		m.failCommand(command, "Invalid workflow configuration")
		return
	}

	// 각 단계 실행
	for i, stepData := range steps {
		step, ok := stepData.(map[string]interface{})
		if !ok {
			m.failCommand(command, fmt.Sprintf("Invalid step %d configuration", i+1))
			return
		}

		// 현재 단계 업데이트
		m.updateCommandStatus(command, models.StatusProcessing, i+1)

		// 단계 실행
		success := m.executeStep(command, step, i+1)

		if !success {
			// 실패 처리 로직
			failureAction := step["on_failure"].(string)
			if failureAction == "abort" {
				m.failCommand(command, fmt.Sprintf("Step %d failed", i+1))
				return
			}
			// retry 등 다른 실패 처리 로직 구현 가능
		}

		// 성공 처리 로직
		successAction := step["on_success"].(string)
		if successAction == "complete" {
			break
		}
		// next는 자동으로 다음 단계로 진행
	}

	// 워크플로우 완료
	m.completeCommand(command)
}

// executeStep 개별 단계 실행
func (m *WorkflowManager) executeStep(command *models.Command, step map[string]interface{}, stepNum int) bool {
	stepName := step["name"].(string)
	utils.Logger.Infof("Executing step %d: %s", stepNum, stepName)

	// 오더 메시지 생성
	orderMsg := m.buildOrderMessage(command, step)

	// Redis에 액션 상태 초기화
	orderKey := fmt.Sprintf("order:%s", orderMsg.OrderID)
	m.initializeOrderState(orderKey, orderMsg)

	// 액션 이력 생성
	actionHistory := &models.ActionHistory{
		CommandID: command.ID,
		ActionData: models.JSON{
			"step_number": stepNum,
			"step_name":   stepName,
			"order_id":    orderMsg.OrderID,
			"actions":     step["actions"],
		},
		Status:    models.StatusProcessing,
		StartedAt: time.Now(),
	}
	m.db.Create(actionHistory)

	// 로봇에 오더 전송
	if err := m.sendOrderToRobot(orderMsg); err != nil {
		m.updateActionHistory(actionHistory, models.StatusFailure, err.Error())
		return false
	}

	// 로봇 상태 업데이트
	m.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", command.RobotSerialNumber).
		Update("current_order_id", orderMsg.OrderID)

	// 타임아웃 설정
	timeout := time.Duration(300) * time.Second // 기본 5분
	if t, ok := step["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Second
	}

	// 완료 대기
	success := m.waitForStepCompletion(orderKey, actionHistory, timeout)

	// Redis 정리
	m.redisClient.Del(m.ctx, orderKey)

	return success
}

// buildOrderMessage 오더 메시지 생성
func (m *WorkflowManager) buildOrderMessage(command *models.Command, step map[string]interface{}) *OrderMessage {
	orderID := m.generateID()
	headerID := utils.GetNextHeaderID()

	// 노드 정보 추출
	node := m.buildNode(step, orderID)

	// 엣지 정보 추출 (있는 경우)
	edges := m.buildEdges(step)

	return &OrderMessage{
		HeaderID:      headerID,
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Version:       "2.0.0",
		Manufacturer:  m.config.RobotManufacturer,
		SerialNumber:  command.RobotSerialNumber,
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes:         []OrderNode{node},
		Edges:         edges,
	}
}

// buildNode 노드 정보 생성
func (m *WorkflowManager) buildNode(step map[string]interface{}, orderID string) OrderNode {
	node := OrderNode{
		NodeID:       m.generateID(),
		SequenceID:   1,
		Released:     true,
		NodePosition: NodePosition{},
		Actions:      []OrderAction{},
	}

	// 노드 위치 정보
	if nodeData, ok := step["node"].(map[string]interface{}); ok {
		if pos, ok := nodeData["position"].(map[string]interface{}); ok {
			node.NodePosition.X = getFloat64(pos["x"])
			node.NodePosition.Y = getFloat64(pos["y"])
			node.NodePosition.Theta = getFloat64(pos["theta"])
		}
		node.NodePosition.MapID = getString(nodeData["map_id"])
	}

	// 액션 정보
	if actions, ok := step["actions"].([]interface{}); ok {
		for _, actionData := range actions {
			if action, ok := actionData.(map[string]interface{}); ok {
				orderAction := OrderAction{
					ActionID:          m.generateID(),
					ActionType:        getString(action["type"]),
					ActionDescription: getString(action["description"]),
					BlockingType:      getString(action["blocking_type"]),
					ActionParameters:  []ActionParameter{},
				}

				// 액션 파라미터
				if params, ok := action["params"].(map[string]interface{}); ok {
					for key, value := range params {
						orderAction.ActionParameters = append(orderAction.ActionParameters,
							ActionParameter{Key: key, Value: value})
					}
				}

				node.Actions = append(node.Actions, orderAction)
			}
		}
	}

	return node
}

// buildEdges 엣지 정보 생성
func (m *WorkflowManager) buildEdges(step map[string]interface{}) []OrderEdge {
	edges := []OrderEdge{}

	if edgeList, ok := step["edges"].([]interface{}); ok {
		for i, edgeData := range edgeList {
			if edge, ok := edgeData.(map[string]interface{}); ok {
				orderEdge := OrderEdge{
					EdgeID:          getString(edge["edge_id"]),
					SequenceID:      i,
					StartNodeID:     getString(edge["start_node_id"]),
					EndNodeID:       getString(edge["end_node_id"]),
					Released:        true,
					RotationAllowed: getBool(edge["rotation_allowed"]),
				}
				edges = append(edges, orderEdge)
			}
		}
	}

	return edges
}

// sendOrderToRobot 로봇에 오더 전송
func (m *WorkflowManager) sendOrderToRobot(order *OrderMessage) error {
	topic := fmt.Sprintf("meili/v2/%s/%s/order", order.Manufacturer, order.SerialNumber)

	data, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to marshal order: %v", err)
	}

	utils.Logger.Infof("Sending order %s to robot %s", order.OrderID, order.SerialNumber)

	token := m.mqttClient.Publish(topic, 0, false, data)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish order: %v", token.Error())
	}

	return nil
}

// HandleRobotStateUpdate 로봇 상태 업데이트 처리
func (m *WorkflowManager) HandleRobotStateUpdate(state *models.RobotStateMessage) {
	if state.OrderID == "" {
		return
	}

	// Redis에서 오더 상태 업데이트
	orderKey := fmt.Sprintf("order:%s", state.OrderID)

	for _, actionState := range state.ActionStates {
		m.redisClient.HSet(m.ctx, orderKey, actionState.ActionID, actionState.ActionStatus)
	}

	// 현재 명령 찾기
	var robotStatus models.RobotStatus
	if err := m.db.Where("serial_number = ?", state.SerialNumber).First(&robotStatus).Error; err != nil {
		return
	}

	if robotStatus.CurrentCommandID != nil {
		var command models.Command
		if err := m.db.First(&command, *robotStatus.CurrentCommandID).Error; err == nil {
			// PLC 상태 업데이트 전송
			m.plcNotifier.SendStatus(&command)
		}
	}
}

// waitForStepCompletion 단계 완료 대기
func (m *WorkflowManager) waitForStepCompletion(orderKey string, history *models.ActionHistory, timeout time.Duration) bool {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ticker.C:
			// 액션 상태 확인
			statuses, _ := m.redisClient.HGetAll(m.ctx, orderKey).Result()

			allCompleted := true
			hasFailed := false

			for _, status := range statuses {
				switch status {
				case models.ActionStatusFinished:
					continue
				case models.ActionStatusFailed:
					hasFailed = true
				default:
					allCompleted = false
				}
			}

			if hasFailed {
				m.updateActionHistory(history, models.StatusFailure, "Action failed")
				return false
			}

			if allCompleted {
				m.updateActionHistory(history, models.StatusSuccess, "")
				return true
			}

		case <-timer.C:
			m.updateActionHistory(history, models.StatusFailure, "Timeout")
			return false
		}
	}
}

// 헬퍼 함수들
func (m *WorkflowManager) updateCommandStatus(command *models.Command, status string, currentStep int) {
	updates := map[string]interface{}{
		"status":       status,
		"current_step": currentStep,
		"updated_at":   time.Now(),
	}

	if status == models.StatusSuccess || status == models.StatusFailure {
		now := time.Now()
		updates["response_time"] = now
	}

	m.db.Model(command).Updates(updates)
	m.plcNotifier.SendStatus(command)
}

func (m *WorkflowManager) failCommand(command *models.Command, reason string) {
	m.db.Model(command).Updates(map[string]interface{}{
		"status":        models.StatusFailure,
		"error_message": reason,
		"response_time": time.Now(),
	})

	m.releaseRobot(command.RobotSerialNumber)
	m.plcNotifier.SendResponse(command.CommandType, "F", reason)
	m.plcNotifier.SendStatus(command)
}

func (m *WorkflowManager) completeCommand(command *models.Command) {
	m.db.Model(command).Updates(map[string]interface{}{
		"status":        models.StatusSuccess,
		"response_time": time.Now(),
	})

	m.releaseRobot(command.RobotSerialNumber)
	m.plcNotifier.SendResponse(command.CommandType, "S", "")
	m.plcNotifier.SendStatus(command)
}

func (m *WorkflowManager) releaseRobot(robotSerial string) {
	m.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", robotSerial).
		Updates(map[string]interface{}{
			"is_busy":            false,
			"current_command_id": nil,
			"current_order_id":   "",
		})
}

func (m *WorkflowManager) updateActionHistory(history *models.ActionHistory, status string, errorMsg string) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       status,
		"completed_at": now,
		"updated_at":   now,
	}

	if errorMsg != "" {
		actionData := history.ActionData
		actionData["error"] = errorMsg
		updates["action_data"] = actionData
	}

	m.db.Model(history).Updates(updates)
}

func (m *WorkflowManager) initializeOrderState(orderKey string, order *OrderMessage) {
	pipe := m.redisClient.Pipeline()

	for _, node := range order.Nodes {
		for _, action := range node.Actions {
			pipe.HSet(m.ctx, orderKey, action.ActionID, models.ActionStatusWaiting)
		}
	}

	pipe.Expire(m.ctx, orderKey, 1*time.Hour)
	pipe.Exec(m.ctx)
}

func (m *WorkflowManager) generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CancelAllActiveOrders 모든 활성 오더 취소
func (m *WorkflowManager) CancelAllActiveOrders(robotSerial string) error {
	// cancelOrder instantAction 전송
	actionID := m.generateID()
	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": m.config.RobotManufacturer,
		"serialNumber": robotSerial,
		"actions": []map[string]interface{}{
			{
				"actionType":       "cancelOrder",
				"actionId":         actionID,
				"blockingType":     "HARD",
				"actionParameters": []interface{}{},
			},
		},
	}

	data, _ := json.Marshal(request)
	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", m.config.RobotManufacturer, robotSerial)

	token := m.mqttClient.Publish(topic, 0, false, data)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// 유틸리티 함수들
func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getFloat64(v interface{}) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0.0
}

func getBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// OrderMessage 관련 구조체들
type OrderMessage struct {
	HeaderID      int64       `json:"headerId"`
	Timestamp     string      `json:"timestamp"`
	Version       string      `json:"version"`
	Manufacturer  string      `json:"manufacturer"`
	SerialNumber  string      `json:"serialNumber"`
	OrderID       string      `json:"orderId"`
	OrderUpdateID int         `json:"orderUpdateId"`
	Nodes         []OrderNode `json:"nodes"`
	Edges         []OrderEdge `json:"edges"`
}

type OrderNode struct {
	NodeID       string        `json:"nodeId"`
	SequenceID   int           `json:"sequenceId"`
	Released     bool          `json:"released"`
	NodePosition NodePosition  `json:"nodePosition"`
	Actions      []OrderAction `json:"actions"`
}

type NodePosition struct {
	X                     float64 `json:"x"`
	Y                     float64 `json:"y"`
	Theta                 float64 `json:"theta"`
	AllowedDeviationXY    float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`
}

type OrderAction struct {
	ActionType        string            `json:"actionType"`
	ActionID          string            `json:"actionId"`
	ActionDescription string            `json:"actionDescription"`
	BlockingType      string            `json:"blockingType"`
	ActionParameters  []ActionParameter `json:"actionParameters"`
}

type ActionParameter struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type OrderEdge struct {
	EdgeID          string `json:"edgeId"`
	SequenceID      int    `json:"sequenceId"`
	StartNodeID     string `json:"startNodeId"`
	EndNodeID       string `json:"endNodeId"`
	Released        bool   `json:"released"`
	RotationAllowed bool   `json:"rotationAllowed"`
}
