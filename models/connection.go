package models

import (
	"time"
)

// Connection State Models
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

// MQTT Connection Message Structure
type ConnectionMessage struct {
	HeaderID        int       `json:"headerId"`
	Timestamp       time.Time `json:"timestamp"`
	Version         string    `json:"version"`
	Manufacturer    string    `json:"manufacturer"`
	SerialNumber    string    `json:"serialNumber"`
	ConnectionState string    `json:"connectionState"`
}
