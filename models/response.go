package models

import (
	"time"
)

// Service Response Models
type RobotCapabilities struct {
	SerialNumber       string            `json:"serialNumber"`
	PhysicalParameters PhysicalParameter `json:"physicalParameters"`
	TypeSpecification  TypeSpecification `json:"typeSpecification"`
	AvailableActions   []AgvAction       `json:"availableActions"`
}

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

type OrderTemplateWithDetails struct {
	OrderTemplate OrderTemplate  `json:"orderTemplate"`
	Nodes         []NodeTemplate `json:"nodes"`
	Edges         []EdgeTemplate `json:"edges"`
}

// Service Request Models
type OrderRequest struct {
	OrderID       string `json:"orderId"`
	OrderUpdateID int    `json:"orderUpdateId"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}

type CustomActionRequest struct {
	HeaderID int      `json:"headerId"`
	Actions  []Action `json:"actions"`
}
