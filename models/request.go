package models

import (
	"time"
)

// Request/Response DTOs
type CreateOrderTemplateRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"` // Array of existing Node IDs
	EdgeIDs     []string `json:"edgeIds"` // Array of existing Edge IDs
}

type NodeTemplateRequest struct {
	NodeID      string                  `json:"nodeId" binding:"required"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	SequenceID  int                     `json:"sequenceId"`
	Released    bool                    `json:"released"`
	Position    NodePositionRequest     `json:"position"`
	Actions     []ActionTemplateRequest `json:"actions"`
}

type NodePositionRequest struct {
	X                     float64 `json:"x"`
	Y                     float64 `json:"y"`
	Theta                 float64 `json:"theta"`
	AllowedDeviationXY    float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`
}

type EdgeTemplateRequest struct {
	EdgeID      string                  `json:"edgeId" binding:"required"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	SequenceID  int                     `json:"sequenceId"`
	Released    bool                    `json:"released"`
	StartNodeID string                  `json:"startNodeId" binding:"required"`
	EndNodeID   string                  `json:"endNodeId" binding:"required"`
	Actions     []ActionTemplateRequest `json:"actions"`
}

type ActionTemplateRequest struct {
	ActionType        string                           `json:"actionType" binding:"required"`
	ActionID          string                           `json:"actionId"`
	BlockingType      string                           `json:"blockingType"`
	ActionDescription string                           `json:"actionDescription"`
	Parameters        []ActionParameterTemplateRequest `json:"parameters"`
}

type ActionParameterTemplateRequest struct {
	Key       string      `json:"key" binding:"required"`
	Value     interface{} `json:"value"`
	ValueType string      `json:"valueType"`
}

type ExecuteOrderRequest struct {
	TemplateID         uint                   `json:"templateId"`
	SerialNumber       string                 `json:"serialNumber" binding:"required"`
	ParameterOverrides map[string]interface{} `json:"parameterOverrides,omitempty"`
}

type OrderExecutionResponse struct {
	OrderID         string    `json:"orderId"`
	Status          string    `json:"status"`
	SerialNumber    string    `json:"serialNumber"`
	OrderTemplateID *uint     `json:"orderTemplateId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// Template Association Requests
type AssociateNodesRequest struct {
	NodeIDs []string `json:"nodeIds" binding:"required"`
}

type AssociateEdgesRequest struct {
	EdgeIDs []string `json:"edgeIds" binding:"required"`
}
