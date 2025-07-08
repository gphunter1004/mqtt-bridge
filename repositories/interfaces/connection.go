package interfaces

import (
	"mqtt-bridge/models"

	"gorm.io/gorm" // gorm 패키지 임포트
)

// ConnectionRepositoryInterface defines the contract for connection state data access.
type ConnectionRepositoryInterface interface {
	// SaveConnectionState saves or updates the connection state for a robot within a transaction.
	SaveConnectionState(tx *gorm.DB, connectionMsg *models.ConnectionMessage) error

	// GetLastConnectionState retrieves the most recent connection state for a robot.
	GetLastConnectionState(serialNumber string) (*models.ConnectionState, error)

	// GetConnectionHistory retrieves connection history for a robot with pagination.
	GetConnectionHistory(serialNumber string, limit int) ([]models.ConnectionState, error)

	// GetConnectedRobots retrieves all robots with ONLINE connection state.
	GetConnectedRobots() ([]string, error)

	// GetRobotManufacturer retrieves the manufacturer for a robot based on latest connection.
	GetRobotManufacturer(serialNumber string) (string, error)
}
