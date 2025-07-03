package models

import (
	"time"
)

// Database Models
type ConnectionState struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	SerialNumber    string    `gorm:"index:idx_serial_state,unique" json:"serialNumber"`
	ConnectionState string    `json:"connectionState"`
	HeaderID        int       `json:"headerId"`
	Timestamp       time.Time `json:"timestamp"`
	Version         string    `json:"version"`
	Manufacturer    string    `json:"manufacturer"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ConnectionStateHistory for keeping history of all connection changes
type ConnectionStateHistory struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	SerialNumber    string    `gorm:"index" json:"serialNumber"`
	ConnectionState string    `json:"connectionState"`
	HeaderID        int       `json:"headerId"`
	Timestamp       time.Time `json:"timestamp"`
	Version         string    `json:"version"`
	Manufacturer    string    `json:"manufacturer"`
	CreatedAt       time.Time `json:"createdAt"`
}

type AgvAction struct {
	ID                uint                 `gorm:"primaryKey" json:"id"`
	SerialNumber      string               `gorm:"index" json:"serialNumber"`
	ActionType        string               `json:"actionType"`
	ActionDescription string               `json:"actionDescription"`
	ActionScopes      string               `json:"actionScopes"` // JSON string
	ResultDescription string               `json:"resultDescription"`
	CreatedAt         time.Time            `json:"createdAt"`
	UpdatedAt         time.Time            `json:"updatedAt"`
	Parameters        []AgvActionParameter `gorm:"foreignKey:AgvActionID" json:"parameters"`
}

type AgvActionParameter struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	AgvActionID   uint   `gorm:"index" json:"agvActionId"`
	Key           string `json:"key"`
	Description   string `json:"description"`
	IsOptional    bool   `json:"isOptional"`
	ValueDataType string `json:"valueDataType"`
}

type PhysicalParameter struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	SerialNumber    string    `gorm:"index;unique" json:"serialNumber"`
	AccelerationMax float64   `json:"accelerationMax"`
	DecelerationMax float64   `json:"decelerationMax"`
	HeightMax       float64   `json:"heightMax"`
	HeightMin       float64   `json:"heightMin"`
	Length          float64   `json:"length"`
	SpeedMax        float64   `json:"speedMax"`
	SpeedMin        float64   `json:"speedMin"`
	Width           float64   `json:"width"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type TypeSpecification struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	SerialNumber      string    `gorm:"index;unique" json:"serialNumber"`
	AgvClass          string    `json:"agvClass"`
	AgvKinematics     string    `json:"agvKinematics"`
	LocalizationTypes string    `json:"localizationTypes"` // JSON string
	MaxLoadMass       int       `json:"maxLoadMass"`
	NavigationTypes   string    `json:"navigationTypes"` // JSON string
	SeriesDescription string    `json:"seriesDescription"`
	SeriesName        string    `json:"seriesName"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// MQTT Message Structures
type ConnectionMessage struct {
	HeaderID        int       `json:"headerId"`
	Timestamp       time.Time `json:"timestamp"`
	Version         string    `json:"version"`
	Manufacturer    string    `json:"manufacturer"`
	SerialNumber    string    `json:"serialNumber"`
	ConnectionState string    `json:"connectionState"`
}

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

type FactsheetActionParam struct {
	Description   string `json:"Description"`
	IsOptional    bool   `json:"IsOptional"`
	Key           string `json:"Key"`
	ValueDataType string `json:"ValueDataType"`
}

type FactsheetAction struct {
	ActionDescription string                 `json:"ActionDescription"`
	ActionParameters  []FactsheetActionParam `json:"ActionParameters"`
	ActionScopes      []string               `json:"ActionScopes"`
	ActionType        string                 `json:"ActionType"`
	ResultDescription string                 `json:"ResultDescription"`
}

type ProtocolFeatures struct {
	AgvActions         []FactsheetAction `json:"AgvActions"`
	OptionalParameters []interface{}     `json:"OptionalParameters"`
}

type PhysicalParams struct {
	AccelerationMax float64 `json:"AccelerationMax"`
	DecelerationMax float64 `json:"DecelerationMax"`
	HeightMax       float64 `json:"HeightMax"`
	HeightMin       float64 `json:"HeightMin"`
	Length          float64 `json:"Length"`
	SpeedMax        float64 `json:"SpeedMax"`
	SpeedMin        float64 `json:"SpeedMin"`
	Width           float64 `json:"Width"`
}

type TypeSpec struct {
	AgvClass          string   `json:"AgvClass"`
	AgvKinematics     string   `json:"AgvKinematics"`
	LocalizationTypes []string `json:"LocalizationTypes"`
	MaxLoadMass       int      `json:"MaxLoadMass"`
	NavigationTypes   []string `json:"NavigationTypes"`
	SeriesDescription string   `json:"SeriesDescription"`
	SeriesName        string   `json:"SeriesName"`
}

type FactsheetMessage struct {
	HeaderID           int              `json:"headerId"`
	Manufacturer       string           `json:"manufacturer"`
	SerialNumber       string           `json:"serialNumber"`
	Timestamp          string           `json:"timestamp"`
	Version            string           `json:"version"`
	ProtocolFeatures   ProtocolFeatures `json:"protocolFeatures"`
	PhysicalParameters PhysicalParams   `json:"physicalParameters"`
	TypeSpecification  TypeSpec         `json:"typeSpecification"`
}

// State Message Structures
type ActionState struct {
	ActionDescription string `json:"actionDescription"`
	ActionID          string `json:"actionId"`
	ActionStatus      string `json:"actionStatus"`
	ActionType        string `json:"actionType"`
	ResultDescription string `json:"resultDescription"`
}

type AgvPosition struct {
	DeviationRange      int     `json:"deviationRange"`
	LocalizationScore   float64 `json:"localizationScore"`
	MapDescription      string  `json:"mapDescription"`
	MapID               string  `json:"mapId"`
	PositionInitialized bool    `json:"positionInitialized"`
	Theta               float64 `json:"theta"`
	X                   float64 `json:"x"`
	Y                   float64 `json:"y"`
}

type BatteryState struct {
	BatteryCharge  float64 `json:"batteryCharge"`
	BatteryHealth  int     `json:"batteryHealth"`
	BatteryVoltage float64 `json:"batteryVoltage"`
	Charging       bool    `json:"charging"`
	Reach          int     `json:"reach"`
}

type SafetyState struct {
	EStop          string `json:"eStop"`
	FieldViolation bool   `json:"fieldViolation"`
}

type Velocity struct {
	Omega float64 `json:"omega"`
	Vx    float64 `json:"vx"`
	Vy    float64 `json:"vy"`
}

type StateMessage struct {
	ActionStates          []ActionState `json:"actionStates"`
	AgvPosition           AgvPosition   `json:"agvPosition"`
	BatteryState          BatteryState  `json:"batteryState"`
	DistanceSinceLastNode int           `json:"distanceSinceLastNode"`
	Driving               bool          `json:"driving"`
	EdgeStates            []interface{} `json:"edgeStates"`
	Errors                []interface{} `json:"errors"`
	HeaderID              int           `json:"headerId"`
	Information           []interface{} `json:"information"`
	LastNodeID            string        `json:"lastNodeId"`
	LastNodeSequenceID    int           `json:"lastNodeSequenceId"`
	Manufacturer          string        `json:"manufacturer"`
	NewBaseRequest        bool          `json:"newBaseRequest"`
	OperatingMode         string        `json:"operatingMode"`
	OrderID               string        `json:"orderId"`
	OrderUpdateID         int           `json:"orderUpdateId"`
	Paused                bool          `json:"paused"`
	SafetyState           SafetyState   `json:"safetyState"`
	SerialNumber          string        `json:"serialNumber"`
	Timestamp             string        `json:"timestamp"`
	Velocity              Velocity      `json:"velocity"`
	Version               string        `json:"version"`
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
