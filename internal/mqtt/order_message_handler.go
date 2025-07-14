// internal/mqtt/order_message_handler.go (수정된 버전)
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

// Float64 항상 소수점을 포함하는 float64
type Float64 float64

func (f Float64) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.1f", float64(f))), nil
}

// DirectOrderMessage Direct Action Order 전용 구조체
type DirectOrderMessage struct {
	HeaderID      int64             `json:"headerId"`
	Timestamp     string            `json:"timestamp"`
	Version       string            `json:"version"`
	Manufacturer  string            `json:"manufacturer"`
	SerialNumber  string            `json:"serialNumber"`
	OrderID       string            `json:"orderId"`
	OrderUpdateID int               `json:"orderUpdateId"`
	Nodes         []DirectOrderNode `json:"nodes"`
	Edges         []DirectOrderEdge `json:"edges"`
}

type DirectOrderNode struct {
	NodeID       string              `json:"nodeId"`
	Description  string              `json:"description"`
	SequenceID   int                 `json:"sequenceId"`
	Released     bool                `json:"released"`
	NodePosition DirectNodePosition  `json:"nodePosition"`
	Actions      []DirectOrderAction `json:"actions"`
}

type DirectNodePosition struct {
	X                     Float64 `json:"x"`
	Y                     Float64 `json:"y"`
	Theta                 Float64 `json:"theta"`
	AllowedDeviationXY    Float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta Float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`
}

type DirectOrderAction struct {
	ActionType        string                       `json:"actionType"`
	ActionID          string                       `json:"actionId"`
	ActionDescription string                       `json:"actionDescription"`
	BlockingType      string                       `json:"blockingType"`
	ActionParameters  []DirectOrderActionParameter `json:"actionParameters"`
}

type DirectOrderActionParameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DirectOrderEdge struct {
	EdgeID          string  `json:"edgeId"`
	SequenceID      int     `json:"sequenceId"`
	StartNodeID     string  `json:"startNodeId"`
	EndNodeID       string  `json:"endNodeId"`
	MaxSpeed        Float64 `json:"maxSpeed,omitempty"`
	MaxHeight       Float64 `json:"maxHeight,omitempty"`
	MinHeight       Float64 `json:"minHeight,omitempty"`
	Orientation     Float64 `json:"orientation,omitempty"`
	Direction       string  `json:"direction,omitempty"`
	RotationAllowed bool    `json:"rotationAllowed"`
	Released        bool    `json:"released"`
}

// OrderSubscriber Order 토픽 구독하여 전송 검증
type OrderSubscriber struct {
	mqttClient   mqtt.Client
	config       *config.Config
	lastOrderID  string
	lastReceived time.Time
}

func NewOrderSubscriber(mqttClient mqtt.Client, cfg *config.Config) *OrderSubscriber {
	return &OrderSubscriber{
		mqttClient: mqttClient,
		config:     cfg,
	}
}

// Subscribe order 토픽을 구독하여 전송된 데이터 확인
func (s *OrderSubscriber) Subscribe() error {
	orderTopic := fmt.Sprintf("meili/v2/%s/%s/order", s.config.RobotManufacturer, s.config.RobotSerialNumber)

	// 구독 시도 로그 추가
	utils.Logger.Infof("🔔 SUBSCRIBING TO: %s", orderTopic)

	token := s.mqttClient.Subscribe(orderTopic, 0, s.handleOrderMessage)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("❌ SUBSCRIPTION FAILED: %v", token.Error())
		return token.Error()
	}

	utils.Logger.Infof("✅ SUBSCRIPTION SUCCESS: %s", orderTopic)
	return nil
}

// handleOrderMessage 수신된 order 메시지를 로그로 출력 (검증용) - 중복 방지
func (s *OrderSubscriber) handleOrderMessage(client mqtt.Client, msg mqtt.Message) {
	// 호출 횟수 카운터 로그 추가
	utils.Logger.Infof("📞 HANDLER CALLED: %s (MessageID: %d)", msg.Topic(), msg.MessageID())

	// 중복 메시지 필터링
	var orderData map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &orderData); err != nil {
		utils.Logger.Errorf("❌ JSON PARSE ERROR: %v", err)
		return
	}

	orderID, ok := orderData["orderId"].(string)
	if !ok {
		utils.Logger.Warnf("⚠️ NO ORDER ID FOUND")
		return
	}

	now := time.Now()

	// 같은 orderID가 1초 이내에 들어오면 무시
	if s.lastOrderID == orderID && now.Sub(s.lastReceived) < time.Second {
		utils.Logger.Infof("🚫 DUPLICATE FILTERED: %s (within 1s)", orderID)
		return
	}

	s.lastOrderID = orderID
	s.lastReceived = now

	utils.Logger.Infof("🔍 RECEIVED ORDER: %s", string(msg.Payload()))
}

type OrderMessageHandler struct {
	mqttClient      mqtt.Client
	config          *config.Config
	orderSubscriber *OrderSubscriber
}

func NewOrderMessageHandler(mqttClient mqtt.Client, cfg *config.Config) *OrderMessageHandler {
	utils.Logger.Infof("🏗️ CREATING OrderMessageHandler")

	handler := &OrderMessageHandler{
		mqttClient: mqttClient,
		config:     cfg,
	}

	// Order Subscriber 생성 및 구독 시작
	utils.Logger.Infof("🔧 CREATING OrderSubscriber")
	handler.orderSubscriber = NewOrderSubscriber(mqttClient, cfg)
	if err := handler.orderSubscriber.Subscribe(); err != nil {
		utils.Logger.Errorf("Failed to start order subscription: %v", err)
	}

	utils.Logger.Infof("✅ OrderMessageHandler CREATED")
	return handler
}

// BuildOrderMessage 오더 메시지 생성
func (h *OrderMessageHandler) BuildOrderMessage(execution *models.OrderExecution, step *models.OrderStep) *models.OrderMessage {
	node := h.buildOrderNode(step)
	edges := h.buildOrderEdges(step)

	return &models.OrderMessage{
		HeaderID:      utils.GetNextHeaderID(),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Version:       "2.0.0",
		Manufacturer:  h.config.RobotManufacturer,
		SerialNumber:  h.config.RobotSerialNumber,
		OrderID:       execution.OrderID,
		OrderUpdateID: 0,
		Nodes:         []models.OrderNode{node},
		Edges:         edges,
	}
}

// SendDirectActionOrder :I 또는 :T 명령을 처리하는 함수 (orderID 반환)
func (h *OrderMessageHandler) SendDirectActionOrder(baseCommand string, commandType rune) (string, error) {
	var actionType, paramKey string

	switch commandType {
	case 'I':
		actionType = "Roboligent Robin - Inference"
		paramKey = "inference_name"
	case 'T':
		actionType = "Roboligent Robin - Follow Trajectory"
		paramKey = "trajectory_name"
	default:
		return "", fmt.Errorf("invalid direct action command type: %c", commandType)
	}

	orderID := h.GenerateOrderID()

	// 구조체를 사용하여 커스텀 마샬링 적용
	directOrder := DirectOrderMessage{
		HeaderID:      utils.GetNextHeaderID(),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Version:       "2.0.0",
		Manufacturer:  h.config.RobotManufacturer,
		SerialNumber:  h.config.RobotSerialNumber,
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes: []DirectOrderNode{
			{
				NodeID:      h.GenerateNodeID(),
				Description: fmt.Sprintf("Direct action for command %s", baseCommand),
				SequenceID:  1,
				Released:    true,
				NodePosition: DirectNodePosition{
					X:                     Float64(0.0),
					Y:                     Float64(0.0),
					Theta:                 Float64(0.0),
					AllowedDeviationXY:    Float64(0.0),
					AllowedDeviationTheta: Float64(0.0),
					MapID:                 "",
				},
				Actions: []DirectOrderAction{
					{
						ActionType:        actionType,
						ActionID:          h.GenerateActionID(),
						ActionDescription: fmt.Sprintf("Execute %s for %s", actionType, baseCommand),
						BlockingType:      "NONE",
						ActionParameters: []DirectOrderActionParameter{
							{
								Key:   paramKey,
								Value: baseCommand,
							},
						},
					},
				},
			},
		},
		Edges: []DirectOrderEdge{},
	}

	err := h.SendOrder(directOrder)
	if err != nil {
		return "", err
	}

	return orderID, nil
}

// SendOrder 로봇에 오더 전송
func (h *OrderMessageHandler) SendOrder(orderPayload interface{}) error {
	topic := fmt.Sprintf("meili/v2/%s/%s/order", h.config.RobotManufacturer, h.config.RobotSerialNumber)

	msgData, err := json.Marshal(orderPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %v", err)
	}

	// 최종 전송 메시지 로그
	utils.Logger.Infof("📤 SENDING ORDER: %s", string(msgData))

	token := h.mqttClient.Publish(topic, 0, false, msgData)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	return nil
}

// SendCancelOrder 로봇에 cancelOrder 요청 전송
func (h *OrderMessageHandler) SendCancelOrder() error {
	actionID := h.generateActionID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
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

	// 최종 전송 메시지 로그
	utils.Logger.Infof("📤 SENDING CANCEL ORDER: %s", string(reqData))

	token := h.mqttClient.Publish(topic, 0, false, reqData)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	return nil
}

// buildOrderNode 오더 노드 생성
func (h *OrderMessageHandler) buildOrderNode(step *models.OrderStep) models.OrderNode {
	nodeID := h.GenerateNodeID()

	nodePos := models.NodePosition{
		X:                     models.Float64(0.0),
		Y:                     models.Float64(0.0),
		Theta:                 models.Float64(0.0),
		AllowedDeviationXY:    models.Float64(0.0),
		AllowedDeviationTheta: models.Float64(0.0),
		MapID:                 "",
	}

	if step.NodeTemplate != nil {
		nodePos.X = models.Float64(step.NodeTemplate.X)
		nodePos.Y = models.Float64(step.NodeTemplate.Y)
		nodePos.Theta = models.Float64(step.NodeTemplate.Theta)
		nodePos.AllowedDeviationXY = models.Float64(step.NodeTemplate.AllowedDeviationXY)
		nodePos.AllowedDeviationTheta = models.Float64(step.NodeTemplate.AllowedDeviationTheta)
		nodePos.MapID = step.NodeTemplate.MapID
	}

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
			MaxSpeed:        models.Float64(edgeTemplate.MaxSpeed),
			MaxHeight:       models.Float64(edgeTemplate.MaxHeight),
			MinHeight:       models.Float64(edgeTemplate.MinHeight),
			Orientation:     models.Float64(edgeTemplate.Orientation),
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

func (h *OrderMessageHandler) generateActionID() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("action_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%d", hex.EncodeToString(randomBytes), time.Now().UnixNano())
}
