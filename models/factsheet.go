package models

// Factsheet MQTT Message Structures
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
