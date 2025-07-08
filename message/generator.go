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

// MessageHeader defines a common interface for all MQTT messages with a header.
type MessageHeader interface {
	SetHeader(id int, timestamp, version, manufacturer, serialNumber string)
}

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
	orderMsg := &models.OrderMessage{
		OrderID:       req.OrderID,
		OrderUpdateID: req.OrderUpdateID,
		Nodes:         req.Nodes,
		Edges:         req.Edges,
	}

	// Use helper to populate common header and marshal to JSON
	return g.populateBaseMessage(orderMsg, req.SerialNumber, req.Manufacturer, req.CustomHeaderID)
}

func (g *DefaultMessageGenerator) GenerateInstantActionMessage(req *InstantActionMessageRequest) ([]byte, error) {
	actionMsg := &models.InstantActionMessage{
		Actions: req.Actions,
	}

	return g.populateBaseMessage(actionMsg, req.SerialNumber, req.Manufacturer, req.CustomHeaderID)
}

func (g *DefaultMessageGenerator) GenerateFactsheetRequestMessage(req *FactsheetRequestMessageRequest) ([]byte, error) {
	actionMsg := &models.InstantActionMessage{
		Actions: []models.Action{
			{
				ActionType:       "factsheetRequest",
				ActionID:         utils.GenerateActionID(),
				BlockingType:     "NONE",
				ActionParameters: []models.ActionParameter{},
			},
		},
	}

	return g.populateBaseMessage(actionMsg, req.SerialNumber, req.Manufacturer, req.CustomHeaderID)
}

func (g *DefaultMessageGenerator) GenerateInitPositionMessage(req *InitPositionMessageRequest) ([]byte, error) {
	actionMsg := &models.InstantActionMessage{
		Actions: []models.Action{
			{
				ActionType:   "initPosition",
				ActionID:     utils.GenerateActionID(),
				BlockingType: "NONE",
				ActionParameters: []models.ActionParameter{
					{Key: "pose", Value: req.Pose},
				},
			},
		},
	}

	return g.populateBaseMessage(actionMsg, req.SerialNumber, req.Manufacturer, req.CustomHeaderID)
}

// =======================================================================
// HELPER METHODS
// =======================================================================

// populateBaseMessage is a private helper to fill common header fields and marshal the message.
func (g *DefaultMessageGenerator) populateBaseMessage(msg MessageHeader, serial, manufacturer string, customID *int) ([]byte, error) {
	headerID := g.getNextHeaderID(serial, customID)
	// Populate common header fields using the interface method
	msg.SetHeader(headerID, utils.GetCurrentTimestamp(), "2.0.0", manufacturer, serial)
	// Marshal the final message to JSON
	return json.Marshal(msg)
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
