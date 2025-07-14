// internal/workflow/order_builder.go
package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"sort"
	"strconv"
	"strings"
	"time"
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

// OrderBuilder 오더 메시지 생성기
type OrderBuilder struct {
	config *config.Config
}

// NewOrderBuilder 새 오더 빌더 생성
func NewOrderBuilder(cfg *config.Config) *OrderBuilder {
	return &OrderBuilder{
		config: cfg,
	}
}

// BuildOrderMessage 표준 오더 메시지 생성
func (b *OrderBuilder) BuildOrderMessage(execution *models.OrderExecution, step *models.OrderStep) *models.OrderMessage {
	node := b.buildOrderNode(step)
	edges := b.buildOrderEdges(step)

	return &models.OrderMessage{
		HeaderID:      utils.GetNextHeaderID(),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Version:       "2.0.0",
		Manufacturer:  b.config.RobotManufacturer,
		SerialNumber:  b.config.RobotSerialNumber,
		OrderID:       execution.OrderID,
		OrderUpdateID: 0,
		Nodes:         []models.OrderNode{node},
		Edges:         edges,
	}
}

// BuildDirectActionOrder 직접 액션 오더 메시지 생성
func (b *OrderBuilder) BuildDirectActionOrder(baseCommand string, commandType rune, armParam string) (*DirectOrderMessage, string, error) {
	var actionType string
	var actionParameters []DirectOrderActionParameter

	switch commandType {
	case 'I':
		actionType = "Roboligent Robin - Inference"
		actionParameters = []DirectOrderActionParameter{
			{
				Key:   "inference_name",
				Value: baseCommand,
			},
		}
	case 'T':
		actionType = "Roboligent Robin - Follow Trajectory"
		actionParameters = []DirectOrderActionParameter{
			{
				Key:   "trajectory_name",
				Value: baseCommand,
			},
		}

		// arm 파라미터 처리
		arm := "right" // 기본값
		if armParam != "" {
			switch strings.ToUpper(armParam) {
			case "R":
				arm = "right"
			case "L":
				arm = "left"
			default:
				return nil, "", fmt.Errorf("invalid arm parameter: %s (use R or L)", armParam)
			}
		}

		actionParameters = append(actionParameters, DirectOrderActionParameter{
			Key:   "arm",
			Value: arm,
		})

	default:
		return nil, "", fmt.Errorf("invalid direct action command type: %c", commandType)
	}

	orderID := b.GenerateOrderID()

	directOrder := &DirectOrderMessage{
		HeaderID:      utils.GetNextHeaderID(),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Version:       "2.0.0",
		Manufacturer:  b.config.RobotManufacturer,
		SerialNumber:  b.config.RobotSerialNumber,
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes: []DirectOrderNode{
			{
				NodeID:      b.GenerateNodeID(),
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
						ActionID:          b.GenerateActionID(),
						ActionDescription: fmt.Sprintf("Execute %s for %s", actionType, baseCommand),
						BlockingType:      "NONE",
						ActionParameters:  actionParameters,
					},
				},
			},
		},
		Edges: []DirectOrderEdge{},
	}

	return directOrder, orderID, nil
}

// BuildCancelOrderMessage 취소 오더 메시지 생성
func (b *OrderBuilder) BuildCancelOrderMessage() (map[string]interface{}, error) {
	actionID := b.generateActionID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": b.config.RobotManufacturer,
		"serialNumber": b.config.RobotSerialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":       "cancelOrder",
				"actionId":         actionID,
				"blockingType":     "HARD",
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	return request, nil
}

// buildOrderNode 오더 노드 생성
func (b *OrderBuilder) buildOrderNode(step *models.OrderStep) models.OrderNode {
	nodeID := b.GenerateNodeID()

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
			ActionID:          b.GenerateActionID(),
			ActionDescription: actionTemplate.ActionDescription,
			BlockingType:      actionTemplate.BlockingType,
			ActionParameters:  b.buildActionParameters(actionTemplate.Parameters),
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
func (b *OrderBuilder) buildOrderEdges(step *models.OrderStep) []models.OrderEdge {
	edges := make([]models.OrderEdge, 0, len(step.Edges))

	for i, edgeTemplate := range step.Edges {
		edge := models.OrderEdge{
			EdgeID:          b.GenerateEdgeID(),
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
func (b *OrderBuilder) buildActionParameters(params []models.ActionParameter) []models.OrderActionParameter {
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
func (b *OrderBuilder) GenerateOrderID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (b *OrderBuilder) GenerateNodeID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (b *OrderBuilder) GenerateActionID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (b *OrderBuilder) GenerateEdgeID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (b *OrderBuilder) generateActionID() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("action_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%d", hex.EncodeToString(randomBytes), time.Now().UnixNano())
}
