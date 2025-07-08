package models

import "time"

// --- Request DTOs ---

// CreateOrderTemplateRequest is the DTO for creating an order template.
type CreateOrderTemplateRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"` // Array of existing Node IDs
	EdgeIDs     []string `json:"edgeIds"` // Array of existing Edge IDs
}

// NodeTemplateRequest is the DTO for creating or updating a node template.
type NodeTemplateRequest struct {
	NodeID      string                  `json:"nodeId" binding:"required"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	SequenceID  int                     `json:"sequenceId"`
	Released    bool                    `json:"released"`
	Position    NodePositionRequest     `json:"position"`
	Actions     []ActionTemplateRequest `json:"actions"`
}

// NodePositionRequest is a part of NodeTemplateRequest.
type NodePositionRequest struct {
	X                     float64 `json:"x"`
	Y                     float64 `json:"y"`
	Theta                 float64 `json:"theta"`
	AllowedDeviationXY    float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`
}

// EdgeTemplateRequest is the DTO for creating or updating an edge template.
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

// ActionTemplateRequest is the DTO for creating or updating an action template.
type ActionTemplateRequest struct {
	ActionType        string                           `json:"actionType" binding:"required"`
	ActionID          string                           `json:"actionId"`
	BlockingType      string                           `json:"blockingType"`
	ActionDescription string                           `json:"actionDescription"`
	Parameters        []ActionParameterTemplateRequest `json:"parameters"`
}

// ActionParameterTemplateRequest defines a parameter within an ActionTemplateRequest.
type ActionParameterTemplateRequest struct {
	Key       string      `json:"key" binding:"required"`
	Value     interface{} `json:"value"`
	ValueType string      `json:"valueType"`
}

// ExecuteOrderRequest is the DTO for executing an order from a template.
type ExecuteOrderRequest struct {
	TemplateID         uint                   `json:"templateId"`
	SerialNumber       string                 `json:"serialNumber" binding:"required"`
	ParameterOverrides map[string]interface{} `json:"parameterOverrides,omitempty"`
}

// AssociateNodesRequest is the DTO for associating nodes with an order template.
type AssociateNodesRequest struct {
	NodeIDs []string `json:"nodeIds" binding:"required"`
}

// AssociateEdgesRequest is the DTO for associating edges with an order template.
type AssociateEdgesRequest struct {
	EdgeIDs []string `json:"edgeIds" binding:"required"`
}

// OrderRequest is used for sending a direct order.
type OrderRequest struct {
	OrderID       string `json:"orderId"`
	OrderUpdateID int    `json:"orderUpdateId"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}

// CustomActionRequest is used for sending direct, immediate actions.
type CustomActionRequest struct {
	HeaderID int      `json:"headerId"`
	Actions  []Action `json:"actions"`
}

// --- Response DTOs ---

// OrderExecutionResponse is the DTO returned after successfully executing an order.
type OrderExecutionResponse struct {
	OrderID         string    `json:"orderId"`
	Status          string    `json:"status"`
	SerialNumber    string    `json:"serialNumber"`
	OrderTemplateID *uint     `json:"orderTemplateId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// RobotCapabilities is the DTO that represents all capabilities of a robot.
type RobotCapabilities struct {
	SerialNumber       string            `json:"serialNumber"`
	PhysicalParameters PhysicalParameter `json:"physicalParameters"`
	TypeSpecification  TypeSpecification `json:"typeSpecification"`
	AvailableActions   []AgvAction       `json:"availableActions"`
}

// RobotHealthStatus is the DTO for the robot's health check endpoint.
type RobotHealthStatus struct {
	SerialNumber        string    `json:"serialNumber"`
	IsOnline            bool      `json:"isOnline"`
	BatteryCharge       float64   `json:"batteryCharge"`
	BatteryVoltage      float64   `json:"batteryVoltage"`
	IsCharging          bool      `json:"isCharging"`
	PositionInitialized bool      `json:"positionInitialized"`
	HasErrors           bool      `json:"hasErrors"`
	ErrorCount          int       `json:"errorCount"`
	OperatingMode       string    `json:"operatingMode"`
	IsPaused            bool      `json:"isPaused"`
	IsDriving           bool      `json:"isDriving"`
	LastUpdate          time.Time `json:"lastUpdate"`
}

// NodeWithActions is a response DTO that includes a node and its actions.
type NodeWithActions struct {
	NodeTemplate NodeTemplate     `json:"nodeTemplate"`
	Actions      []ActionTemplate `json:"actions"`
}

// EdgeWithActions is a response DTO that includes an edge and its actions.
type EdgeWithActions struct {
	EdgeTemplate EdgeTemplate     `json:"edgeTemplate"`
	Actions      []ActionTemplate `json:"actions"`
}

// OrderTemplateWithDetails is a response DTO for a detailed view of an order template.
type OrderTemplateWithDetails struct {
	OrderTemplate    OrderTemplate     `json:"orderTemplate"`
	NodesWithActions []NodeWithActions `json:"nodesWithActions"`
	EdgesWithActions []EdgeWithActions `json:"edgesWithActions"`
}
