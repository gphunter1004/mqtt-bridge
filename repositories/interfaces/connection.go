package interfaces

import (
	"mqtt-bridge/models"
)

// ConnectionRepositoryInterface defines the contract for connection state data access
type ConnectionRepositoryInterface interface {
	// SaveConnectionState saves or updates the connection state for a robot
	SaveConnectionState(connectionMsg *models.ConnectionMessage) error

	// GetLastConnectionState retrieves the most recent connection state for a robot
	GetLastConnectionState(serialNumber string) (*models.ConnectionState, error)

	// GetConnectionHistory retrieves connection history for a robot with pagination
	GetConnectionHistory(serialNumber string, limit int) ([]models.ConnectionState, error)

	// GetConnectedRobots retrieves all robots with ONLINE connection state
	GetConnectedRobots() ([]string, error)

	// GetRobotManufacturer retrieves the manufacturer for a robot based on latest connection
	GetRobotManufacturer(serialNumber string) (string, error)
}
