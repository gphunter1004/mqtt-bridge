package message

import (
	"encoding/json"
	"sync"

	"mqtt-bridge/models"
	"mqtt-bridge/utils"
)

// MessageType 정의
type MessageType string

const (
	MessageTypeOrder         MessageType = "order"
	MessageTypeInstantAction MessageType = "instantAction"
	MessageTypeFactsheetReq  MessageType = "factsheetRequest"
	MessageTypeInitPosition  MessageType = "initPosition"
)

// MessageGenerator 인터페이스
type MessageGenerator interface {
	GenerateOrderMessage(req *OrderMessageRequest) ([]byte, error)
	GenerateInstantActionMessage(req *InstantActionMessageRequest) ([]byte, error)
	GenerateFactsheetRequestMessage(req *FactsheetRequestMessageRequest) ([]byte, error)
	GenerateInitPositionMessage(req *InitPositionMessageRequest) ([]byte, error)
}

// DefaultMessageGenerator 구현체
type DefaultMessageGenerator struct {
	headerIDTracker map[string]int
	mutex           sync.RWMutex
}

func NewMessageGenerator() MessageGenerator {
	return &DefaultMessageGenerator{
		headerIDTracker: make(map[string]int),
	}
}

// =======================================================================
// MESSAGE REQUEST 구조체들
// =======================================================================

type OrderMessageRequest struct {
	SerialNumber   string        `json:"serialNumber"`
	Manufacturer   string        `json:"manufacturer"`
	OrderID        string        `json:"orderId"`
	OrderUpdateID  int           `json:"orderUpdateId"`
	Nodes          []models.Node `json:"nodes"`
	Edges          []models.Edge `json:"edges"`
	CustomHeaderID *int          `json:"customHeaderId,omitempty"`
}

type InstantActionMessageRequest struct {
	SerialNumber   string          `json:"serialNumber"`
	Manufacturer   string          `json:"manufacturer"`
	Actions        []models.Action `json:"actions"`
	CustomHeaderID *int            `json:"customHeaderId,omitempty"`
}

type FactsheetRequestMessageRequest struct {
	SerialNumber   string `json:"serialNumber"`
	Manufacturer   string `json:"manufacturer"`
	CustomHeaderID *int   `json:"customHeaderId,omitempty"`
}

type InitPositionMessageRequest struct {
	SerialNumber   string                 `json:"serialNumber"`
	Manufacturer   string                 `json:"manufacturer"`
	Pose           map[string]interface{} `json:"pose"`
	CustomHeaderID *int                   `json:"customHeaderId,omitempty"`
}

// =======================================================================
// MESSAGE GENERATOR 구현
// =======================================================================

func (g *DefaultMessageGenerator) GenerateOrderMessage(req *OrderMessageRequest) ([]byte, error) {
	headerID := g.getNextHeaderID(req.SerialNumber, req.CustomHeaderID)

	orderMsg := &models.OrderMessage{
		HeaderID:      headerID,
		Timestamp:     utils.GetCurrentTimestamp(),
		Version:       "2.0.0",
		Manufacturer:  req.Manufacturer,
		SerialNumber:  req.SerialNumber,
		OrderID:       req.OrderID,
		OrderUpdateID: req.OrderUpdateID,
		Nodes:         req.Nodes,
		Edges:         req.Edges,
	}

	return json.Marshal(orderMsg)
}

func (g *DefaultMessageGenerator) GenerateInstantActionMessage(req *InstantActionMessageRequest) ([]byte, error) {
	headerID := g.getNextHeaderID(req.SerialNumber, req.CustomHeaderID)

	actionMsg := &models.InstantActionMessage{
		HeaderID:     headerID,
		Timestamp:    utils.GetCurrentTimestamp(),
		Version:      "2.0.0",
		Manufacturer: req.Manufacturer,
		SerialNumber: req.SerialNumber,
		Actions:      req.Actions,
	}

	return json.Marshal(actionMsg)
}

func (g *DefaultMessageGenerator) GenerateFactsheetRequestMessage(req *FactsheetRequestMessageRequest) ([]byte, error) {
	headerID := g.getNextHeaderID(req.SerialNumber, req.CustomHeaderID)
	actionID := utils.GenerateActionID() + "_factsheet"

	actionMsg := &models.InstantActionMessage{
		HeaderID:     headerID,
		Timestamp:    utils.GetCurrentTimestamp(),
		Version:      "2.0.0",
		Manufacturer: req.Manufacturer,
		SerialNumber: req.SerialNumber,
		Actions: []models.Action{
			{
				ActionType:       "factsheetRequest",
				ActionID:         actionID,
				BlockingType:     "NONE",
				ActionParameters: []models.ActionParameter{},
			},
		},
	}

	return json.Marshal(actionMsg)
}

func (g *DefaultMessageGenerator) GenerateInitPositionMessage(req *InitPositionMessageRequest) ([]byte, error) {
	headerID := g.getNextHeaderID(req.SerialNumber, req.CustomHeaderID)
	actionID := utils.GenerateActionID() + "_initpos"

	actionMsg := &models.InstantActionMessage{
		HeaderID:     headerID,
		Timestamp:    utils.GetCurrentTimestamp(),
		Version:      "2.0.0",
		Manufacturer: req.Manufacturer,
		SerialNumber: req.SerialNumber,
		Actions: []models.Action{
			{
				ActionType:   "initPosition",
				ActionID:     actionID,
				BlockingType: "NONE",
				ActionParameters: []models.ActionParameter{
					{
						Key:   "pose",
						Value: req.Pose,
					},
				},
			},
		},
	}

	return json.Marshal(actionMsg)
}

func (g *DefaultMessageGenerator) getNextHeaderID(serialNumber string, customID *int) int {
	if customID != nil {
		return *customID
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.headerIDTracker[serialNumber]++
	return g.headerIDTracker[serialNumber]
}
