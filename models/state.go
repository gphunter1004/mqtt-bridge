package models

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
