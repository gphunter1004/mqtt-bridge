package models

import (
	"encoding/json"
	"time"
)

// Order Template Models
type OrderTemplate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Independent Node Model
type NodeTemplate struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	NodeID      string `gorm:"unique;not null" json:"nodeId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SequenceID  int    `json:"sequenceId"`
	Released    bool   `json:"released"`

	// Position fields
	X                     float64 `json:"x"`
	Y                     float64 `json:"y"`
	Theta                 float64 `json:"theta"`
	AllowedDeviationXY    float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`

	// JSON field to store action template IDs
	ActionTemplateIDs string `json:"actionTemplateIds"` // JSON array of action template IDs

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Independent Edge Model
type EdgeTemplate struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	EdgeID      string `gorm:"unique;not null" json:"edgeId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SequenceID  int    `json:"sequenceId"`
	Released    bool   `json:"released"`
	StartNodeID string `json:"startNodeId"`
	EndNodeID   string `json:"endNodeId"`

	// JSON field to store action template IDs
	ActionTemplateIDs string `json:"actionTemplateIds"` // JSON array of action template IDs

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Order Template Association Tables (Many-to-Many)
type OrderTemplateNode struct {
	ID              uint          `gorm:"primaryKey" json:"id"`
	OrderTemplateID uint          `gorm:"index" json:"orderTemplateId"`
	NodeTemplateID  uint          `gorm:"index" json:"nodeTemplateId"`
	OrderTemplate   OrderTemplate `gorm:"foreignKey:OrderTemplateID" json:"orderTemplate"`
	NodeTemplate    NodeTemplate  `gorm:"foreignKey:NodeTemplateID" json:"nodeTemplate"`
}

type OrderTemplateEdge struct {
	ID              uint          `gorm:"primaryKey" json:"id"`
	OrderTemplateID uint          `gorm:"index" json:"orderTemplateId"`
	EdgeTemplateID  uint          `gorm:"index" json:"edgeTemplateId"`
	OrderTemplate   OrderTemplate `gorm:"foreignKey:OrderTemplateID" json:"orderTemplate"`
	EdgeTemplate    EdgeTemplate  `gorm:"foreignKey:EdgeTemplateID" json:"edgeTemplate"`
}

type ActionTemplate struct {
	ID                uint                      `gorm:"primaryKey" json:"id"`
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

// Helper methods to handle action template IDs

func (nt *NodeTemplate) GetActionTemplateIDs() ([]uint, error) {
	if nt.ActionTemplateIDs == "" {
		return []uint{}, nil
	}

	var ids []uint
	err := json.Unmarshal([]byte(nt.ActionTemplateIDs), &ids)
	return ids, err
}

func (nt *NodeTemplate) SetActionTemplateIDs(ids []uint) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	nt.ActionTemplateIDs = string(data)
	return nil
}

func (et *EdgeTemplate) GetActionTemplateIDs() ([]uint, error) {
	if et.ActionTemplateIDs == "" {
		return []uint{}, nil
	}

	var ids []uint
	err := json.Unmarshal([]byte(et.ActionTemplateIDs), &ids)
	return ids, err
}

func (et *EdgeTemplate) SetActionTemplateIDs(ids []uint) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	et.ActionTemplateIDs = string(data)
	return nil
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
		Actions: []Action{}, // Will be populated by service when needed
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
		Actions:     []Action{}, // Will be populated by service when needed
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
