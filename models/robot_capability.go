package models

import (
	"time"
)

// Robot Capability Models
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
