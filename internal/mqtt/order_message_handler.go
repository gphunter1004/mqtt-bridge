// internal/mqtt/order_message_handler.go (최종 수정본)
package mqtt

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"sort"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type OrderMessageHandler struct {
	mqttClient mqtt.Client
	config     *config.Config
}

func NewOrderMessageHandler(mqttClient mqtt.Client, cfg *config.Config) *OrderMessageHandler {
	return &OrderMessageHandler{
		mqttClient: mqttClient,
		config:     cfg,
	}
}

// BuildOrderMessage 오더 메시지 생성
func (h *OrderMessageHandler) BuildOrderMessage(execution *models.OrderExecution, step *models.OrderStep) *models.OrderMessage {
	node := h.buildOrderNode(step)
	edges := h.buildOrderEdges(step)

	orderMsg := &models.OrderMessage{
		HeaderID:      utils.GetNextHeaderID(), // 수정: 1씩 증가하는 ID 사용
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Version:       "2.0.0",
		Manufacturer:  h.config.RobotManufacturer,
		SerialNumber:  h.config.RobotSerialNumber,
		OrderID:       execution.OrderID,
		OrderUpdateID: 0, // 수정: 항상 0으로 고정
		Nodes:         []models.OrderNode{node},
		Edges:         edges,
	}

	return orderMsg
}

// SendOrder 로봇에 오더 전송
func (h *OrderMessageHandler) SendOrder(orderMsg *models.OrderMessage) error {
	topic := fmt.Sprintf("meili/v2/%s/%s/order", h.config.RobotManufacturer, h.config.RobotSerialNumber)

	msgData, err := json.Marshal(orderMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %v", err)
	}

	utils.Logger.Infof("Sending order to robot via topic: %s", topic)
	utils.Logger.Debugf("Order payload: %s", string(msgData))

	token := h.mqttClient.Publish(topic, 0, false, msgData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	utils.Logger.Infof("Order sent successfully: %s", orderMsg.OrderID)
	return nil
}

// SendCancelOrder 로봇에 cancelOrder 요청 전송
func (h *OrderMessageHandler) SendCancelOrder() error {
	actionID := h.generateActionID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(), // 수정: 1씩 증가하는 ID 사용
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": h.config.RobotManufacturer,
		"serialNumber": h.config.RobotSerialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":       "cancelOrder",
				"actionId":         actionID,
				"blockingType":     "HARD",
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	reqData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal cancelOrder request: %v", err)
	}

	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", h.config.RobotManufacturer, h.config.RobotSerialNumber)

	utils.Logger.Infof("Sending cancelOrder request to %s (ActionID: %s)", topic, actionID)
	utils.Logger.Debugf("CancelOrder request payload: %s", string(reqData))

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	utils.Logger.Infof("CancelOrder request sent successfully to robot: %s (ActionID: %s)", h.config.RobotSerialNumber, actionID)
	return nil
}

// buildOrderNode 오더 노드 생성
func (h *OrderMessageHandler) buildOrderNode(step *models.OrderStep) models.OrderNode {
	nodeID := h.GenerateNodeID()

	nodePos := models.NodePosition{
		X:                     0.0,
		Y:                     0.0,
		Theta:                 0.0,
		AllowedDeviationXY:    0.0,
		AllowedDeviationTheta: 0.0,
		MapID:                 "",
	}

	if step.NodeTemplate != nil {
		nodePos.X = step.NodeTemplate.X
		nodePos.Y = step.NodeTemplate.Y
		nodePos.Theta = step.NodeTemplate.Theta
		nodePos.AllowedDeviationXY = step.NodeTemplate.AllowedDeviationXY
		nodePos.AllowedDeviationTheta = step.NodeTemplate.AllowedDeviationTheta
		nodePos.MapID = step.NodeTemplate.MapID
	}

	// StepActionMappings를 ExecutionOrder 순서로 정렬
	sort.Slice(step.StepActionMappings, func(i, j int) bool {
		return step.StepActionMappings[i].ExecutionOrder < step.StepActionMappings[j].ExecutionOrder
	})

	actions := make([]models.OrderAction, 0, len(step.StepActionMappings))
	for _, mapping := range step.StepActionMappings {
		actionTemplate := mapping.ActionTemplate
		action := models.OrderAction{
			ActionType:        actionTemplate.ActionType,
			ActionID:          h.GenerateActionID(),
			ActionDescription: actionTemplate.ActionDescription,
			BlockingType:      actionTemplate.BlockingType,
			ActionParameters:  h.buildActionParameters(actionTemplate.Parameters),
		}
		actions = append(actions, action)
	}

	return models.OrderNode{
		NodeID:       nodeID,
		Description:  "",
		SequenceID:   step.StepOrder,
		Released:     true,
		NodePosition: nodePos,
		Actions:      actions,
	}
}

// buildOrderEdges 오더 엣지 생성
func (h *OrderMessageHandler) buildOrderEdges(step *models.OrderStep) []models.OrderEdge {
	edges := make([]models.OrderEdge, 0, len(step.Edges))

	for i, edgeTemplate := range step.Edges {
		edge := models.OrderEdge{
			EdgeID:          h.GenerateEdgeID(),
			SequenceID:      i,
			StartNodeID:     edgeTemplate.StartNodeID,
			EndNodeID:       edgeTemplate.EndNodeID,
			MaxSpeed:        edgeTemplate.MaxSpeed,
			MaxHeight:       edgeTemplate.MaxHeight,
			MinHeight:       edgeTemplate.MinHeight,
			Orientation:     edgeTemplate.Orientation,
			Direction:       edgeTemplate.Direction,
			RotationAllowed: edgeTemplate.RotationAllowed,
			Released:        true,
		}
		edges = append(edges, edge)
	}

	return edges
}

// buildActionParameters 액션 파라미터 생성
func (h *OrderMessageHandler) buildActionParameters(params []models.ActionParameter) []models.OrderActionParameter {
	actionParams := make([]models.OrderActionParameter, 0, len(params))

	for _, param := range params {
		var value interface{}

		switch param.ValueType {
		case "NUMBER":
			if floatVal, err := strconv.ParseFloat(param.Value, 64); err == nil {
				value = floatVal
			} else {
				value = param.Value
			}
		case "BOOLEAN":
			if boolVal, err := strconv.ParseBool(param.Value); err == nil {
				value = boolVal
			} else {
				value = param.Value
			}
		default: // STRING
			value = param.Value
		}

		actionParam := models.OrderActionParameter{
			Key:   param.Key,
			Value: value,
		}
		actionParams = append(actionParams, actionParam)
	}

	return actionParams
}

// ID 생성 함수들
func (h *OrderMessageHandler) GenerateOrderID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (h *OrderMessageHandler) GenerateNodeID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (h *OrderMessageHandler) GenerateActionID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (h *OrderMessageHandler) GenerateEdgeID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

// generateActionID cancelOrder용 액션 ID 생성 (내부용)
func (h *OrderMessageHandler) generateActionID() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("action_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%d", hex.EncodeToString(randomBytes), time.Now().UnixNano())
}
