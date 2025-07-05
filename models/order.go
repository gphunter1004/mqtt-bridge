package models

import (
	"encoding/json"
	"time"
)

// Order Template Models
type OrderTemplate struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"unique;not null" json:"name"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	Nodes       []NodeTemplate `gorm:"foreignKey:OrderTemplateID" json:"nodes"`
	Edges       []EdgeTemplate `gorm:"foreignKey:OrderTemplateID" json:"edges"`
}

type NodeTemplate struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	OrderTemplateID uint   `gorm:"index" json:"orderTemplateId"`
	NodeID          string `gorm:"not null" json:"nodeId"`
	Description     string `json:"description"`
	SequenceID      int    `json:"sequenceId"`
	Released        bool   `json:"released"`

	// Position fields
	X                     float64 `json:"x"`
	Y                     float64 `json:"y"`
	Theta                 float64 `json:"theta"`
	AllowedDeviationXY    float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`

	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Actions   []ActionTemplate `gorm:"foreignKey:NodeTemplateID" json:"actions"`
}

type EdgeTemplate struct {
	ID              uint             `gorm:"primaryKey" json:"id"`
	OrderTemplateID uint             `gorm:"index" json:"orderTemplateId"`
	EdgeID          string           `gorm:"not null" json:"edgeId"`
	SequenceID      int              `json:"sequenceId"`
	Released        bool             `json:"released"`
	StartNodeID     string           `json:"startNodeId"`
	EndNodeID       string           `json:"endNodeId"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	Actions         []ActionTemplate `gorm:"foreignKey:EdgeTemplateID" json:"actions"`
}

type ActionTemplate struct {
	ID                uint                      `gorm:"primaryKey" json:"id"`
	NodeTemplateID    *uint                     `gorm:"index" json:"nodeTemplateId,omitempty"`
	EdgeTemplateID    *uint                     `gorm:"index" json:"edgeTemplateId,omitempty"`
	ActionType        string                    `gorm:"not null" json:"actionType"`
	ActionID          string                    `json:"actionId"`
	BlockingType      string                    `json:"blockingType"`
	ActionDescription string                    `json:"actionDescription"`
	CreatedAt         time.Time                 `json:"createdAt"`
	UpdatedAt         time.Time                 `json:"updatedAt"`
	Parameters        []ActionParameterTemplate `gorm:"foreignKey:ActionTemplateID" json:"parameters"`
}

type ActionParameterTemplate struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	ActionTemplateID uint   `gorm:"index" json:"actionTemplateId"`
	Key              string `gorm:"not null" json:"key"`
	Value            string `json:"value"`     // JSON string to store any value type
	ValueType        string `json:"valueType"` // "string", "number", "boolean", "object"
}

// Order Execution History Models
type OrderExecution struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	OrderID         string     `gorm:"unique;not null" json:"orderId"`
	OrderTemplateID *uint      `gorm:"index" json:"orderTemplateId,omitempty"`
	SerialNumber    string     `gorm:"index" json:"serialNumber"`
	OrderUpdateID   int        `json:"orderUpdateId"`
	Status          string     `json:"status"` // "CREATED", "SENT", "ACKNOWLEDGED", "EXECUTING", "COMPLETED", "FAILED"
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
}

// Request/Response DTOs
type CreateOrderTemplateRequest struct {
	Name        string                `json:"name" binding:"required"`
	Description string                `json:"description"`
	Nodes       []NodeTemplateRequest `json:"nodes"`
	Edges       []EdgeTemplateRequest `json:"edges"`
}

type NodeTemplateRequest struct {
	NodeID      string                  `json:"nodeId" binding:"required"`
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

// Helper methods to convert between template and MQTT models
func (nt *NodeTemplate) ToNode() Node {
	node := Node{
		NodeID:      nt.NodeID,
		Description: nt.Description,
		SequenceID:  nt.SequenceID,
		Released:    nt.Released,
		NodePosition: NodePosition{
			X:                     nt.X,
			Y:                     nt.Y,
			Theta:                 nt.Theta,
			AllowedDeviationXY:    nt.AllowedDeviationXY,
			AllowedDeviationTheta: nt.AllowedDeviationTheta,
			MapID:                 nt.MapID,
		},
		Actions: make([]Action, len(nt.Actions)),
	}

	for i, action := range nt.Actions {
		node.Actions[i] = action.ToAction()
	}

	return node
}

func (et *EdgeTemplate) ToEdge() Edge {
	edge := Edge{
		EdgeID:      et.EdgeID,
		SequenceID:  et.SequenceID,
		Released:    et.Released,
		StartNodeID: et.StartNodeID,
		EndNodeID:   et.EndNodeID,
		Actions:     make([]Action, len(et.Actions)),
	}

	for i, action := range et.Actions {
		edge.Actions[i] = action.ToAction()
	}

	return edge
}

func (at *ActionTemplate) ToAction() Action {
	action := Action{
		ActionType:        at.ActionType,
		ActionID:          at.ActionID,
		BlockingType:      at.BlockingType,
		ActionDescription: at.ActionDescription,
		ActionParameters:  make([]ActionParameter, len(at.Parameters)),
	}

	for i, param := range at.Parameters {
		var value interface{}
		if param.Value != "" {
			switch param.ValueType {
			case "object":
				json.Unmarshal([]byte(param.Value), &value)
			case "number":
				json.Unmarshal([]byte(param.Value), &value)
			case "boolean":
				json.Unmarshal([]byte(param.Value), &value)
			default:
				value = param.Value
			}
		}

		action.ActionParameters[i] = ActionParameter{
			Key:   param.Key,
			Value: value,
		}
	}

	return action
}
