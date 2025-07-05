package models

// MQTT Action and Order Message Structures
type ActionParameter struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type Action struct {
	ActionType        string            `json:"actionType"`
	ActionID          string            `json:"actionId"`
	BlockingType      string            `json:"blockingType"`
	ActionParameters  []ActionParameter `json:"actionParameters"`
	ActionDescription string            `json:"actionDescription,omitempty"`
}

type InstantActionMessage struct {
	HeaderID     int      `json:"headerId"`
	Timestamp    string   `json:"timestamp"`
	Version      string   `json:"version"`
	Manufacturer string   `json:"manufacturer"`
	SerialNumber string   `json:"serialNumber"`
	Actions      []Action `json:"actions"`
}

// Order Message Structures
type NodePosition struct {
	X                     float64 `json:"x"`
	Y                     float64 `json:"y"`
	Theta                 float64 `json:"theta"`
	AllowedDeviationXY    float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`
}

type Node struct {
	NodeID       string       `json:"nodeId"`
	Description  string       `json:"description"`
	SequenceID   int          `json:"sequenceId"`
	Released     bool         `json:"released"`
	NodePosition NodePosition `json:"nodePosition"`
	Actions      []Action     `json:"actions"`
}

type Edge struct {
	EdgeID      string   `json:"edgeId"`
	SequenceID  int      `json:"sequenceId"`
	Released    bool     `json:"released"`
	StartNodeID string   `json:"startNodeId"`
	EndNodeID   string   `json:"endNodeId"`
	Actions     []Action `json:"actions"`
}

type OrderMessage struct {
	HeaderID      int    `json:"headerId"`
	Timestamp     string `json:"timestamp"`
	Version       string `json:"version"`
	Manufacturer  string `json:"manufacturer"`
	SerialNumber  string `json:"serialNumber"`
	OrderID       string `json:"orderId"`
	OrderUpdateID int    `json:"orderUpdateId"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}
