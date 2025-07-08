package models

import (
	"encoding/json"
	"time"
)

// --- Action Template Models ---

// ActionTemplate represents a reusable action definition.
type ActionTemplate struct {
	ID                uint                      `gorm:"primaryKey" json:"id"`
	ActionType        string                    `gorm:"not null" json:"actionType"`
	ActionID          string                    `json:"actionId,omitempty"`
	BlockingType      string                    `json:"blockingType,omitempty"`
	ActionDescription string                    `json:"actionDescription,omitempty"`
	CreatedAt         time.Time                 `json:"createdAt"`
	UpdatedAt         time.Time                 `json:"updatedAt"`
	Parameters        []ActionParameterTemplate `gorm:"foreignKey:ActionTemplateID" json:"parameters"`
}

// ActionParameterTemplate defines a parameter for an ActionTemplate.
type ActionParameterTemplate struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	ActionTemplateID uint   `gorm:"index" json:"actionTemplateId"`
	Key              string `gorm:"not null" json:"key"`
	Value            string `json:"value,omitempty"`     // JSON string to store any value type
	ValueType        string `json:"valueType,omitempty"` // "string", "number", "boolean", "object"
}

// --- Node & Edge Template Models ---

// NodeTemplate represents a location point in a workflow.
type NodeTemplate struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	NodeID                string    `gorm:"unique;not null" json:"nodeId"`
	Name                  string    `json:"name,omitempty"`
	Description           string    `json:"description,omitempty"`
	SequenceID            int       `json:"sequenceId"`
	Released              bool      `json:"released"`
	X                     float64   `json:"x"`
	Y                     float64   `json:"y"`
	Theta                 float64   `json:"theta"`
	AllowedDeviationXY    float64   `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64   `json:"allowedDeviationTheta"`
	MapID                 string    `json:"mapId,omitempty"`
	ActionTemplateIDs     string    `json:"actionTemplateIds,omitempty"` // JSON array of action template IDs
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// EdgeTemplate represents a path between two NodeTemplates.
type EdgeTemplate struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	EdgeID            string    `gorm:"unique;not null" json:"edgeId"`
	Name              string    `json:"name,omitempty"`
	Description       string    `json:"description,omitempty"`
	SequenceID        int       `json:"sequenceId"`
	Released          bool      `json:"released"`
	StartNodeID       string    `json:"startNodeId"`
	EndNodeID         string    `json:"endNodeId"`
	ActionTemplateIDs string    `json:"actionTemplateIds,omitempty"` // JSON array of action template IDs
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// --- Order Template & Association Models ---

// OrderTemplate defines a reusable workflow template.
type OrderTemplate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OrderTemplateNode is the join table for the many-to-many relationship between orders and nodes.
type OrderTemplateNode struct {
	ID              uint          `gorm:"primaryKey" json:"id"`
	OrderTemplateID uint          `gorm:"index" json:"orderTemplateId"`
	NodeTemplateID  uint          `gorm:"index" json:"nodeTemplateId"`
	OrderTemplate   OrderTemplate `gorm:"foreignKey:OrderTemplateID" json:"-"`
	NodeTemplate    NodeTemplate  `gorm:"foreignKey:NodeTemplateID" json:"-"`
}

// OrderTemplateEdge is the join table for the many-to-many relationship between orders and edges.
type OrderTemplateEdge struct {
	ID              uint          `gorm:"primaryKey" json:"id"`
	OrderTemplateID uint          `gorm:"index" json:"orderTemplateId"`
	EdgeTemplateID  uint          `gorm:"index" json:"edgeTemplateId"`
	OrderTemplate   OrderTemplate `gorm:"foreignKey:OrderTemplateID" json:"-"`
	EdgeTemplate    EdgeTemplate  `gorm:"foreignKey:EdgeTemplateID" json:"-"`
}

// --- Order Execution History Model ---

// OrderExecution tracks the state of a specific order sent to a robot.
type OrderExecution struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	OrderID         string     `gorm:"unique;not null" json:"orderId"`
	OrderTemplateID *uint      `gorm:"index" json:"orderTemplateId,omitempty"`
	SerialNumber    string     `gorm:"index" json:"serialNumber"`
	OrderUpdateID   int        `json:"orderUpdateId"`
	Status          string     `json:"status"` // e.g., "CREATED", "SENT", "EXECUTING", "COMPLETED"
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
}

// --- Helper Methods ---

// GetActionTemplateIDs unmarshals the JSON array of IDs from a NodeTemplate.
func (nt *NodeTemplate) GetActionTemplateIDs() ([]uint, error) {
	if nt.ActionTemplateIDs == "" {
		return []uint{}, nil
	}
	var ids []uint
	return ids, json.Unmarshal([]byte(nt.ActionTemplateIDs), &ids)
}

// SetActionTemplateIDs marshals a slice of IDs into a JSON string for a NodeTemplate.
func (nt *NodeTemplate) SetActionTemplateIDs(ids []uint) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	nt.ActionTemplateIDs = string(data)
	return nil
}

// GetActionTemplateIDs unmarshals the JSON array of IDs from an EdgeTemplate.
func (et *EdgeTemplate) GetActionTemplateIDs() ([]uint, error) {
	if et.ActionTemplateIDs == "" {
		return []uint{}, nil
	}
	var ids []uint
	return ids, json.Unmarshal([]byte(et.ActionTemplateIDs), &ids)
}

// SetActionTemplateIDs marshals a slice of IDs into a JSON string for an EdgeTemplate.
func (et *EdgeTemplate) SetActionTemplateIDs(ids []uint) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	et.ActionTemplateIDs = string(data)
	return nil
}

// --- Conversion Helper Methods (FIXED) ---

// ToNode converts a NodeTemplate to an MQTT-compatible Node struct.
func (nt *NodeTemplate) ToNode() Node {
	return Node{
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
		Actions: []Action{}, // Actions should be populated by the service layer.
	}
}

// ToEdge converts an EdgeTemplate to an MQTT-compatible Edge struct.
func (et *EdgeTemplate) ToEdge() Edge {
	return Edge{
		EdgeID:      et.EdgeID,
		SequenceID:  et.SequenceID,
		Released:    et.Released,
		StartNodeID: et.StartNodeID,
		EndNodeID:   et.EndNodeID,
		Actions:     []Action{}, // Actions should be populated by the service layer.
	}
}

// ToAction converts an ActionTemplate to an MQTT-compatible Action struct.
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
			// Attempt to unmarshal to preserve original type (number, boolean, object)
			// Fallback to string if unmarshal fails
			if json.Unmarshal([]byte(param.Value), &value) != nil {
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
