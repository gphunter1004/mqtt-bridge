package repositories

import (
	"fmt"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// ConnectionRepository implements ConnectionRepositoryInterface.
type ConnectionRepository struct {
	db *gorm.DB
}

// NewConnectionRepository creates a new instance of ConnectionRepository.
func NewConnectionRepository(db *gorm.DB) interfaces.ConnectionRepositoryInterface {
	return &ConnectionRepository{
		db: db,
	}
}

// SaveConnectionState saves or updates the connection state for a robot within a transaction.
func (cr *ConnectionRepository) SaveConnectionState(tx *gorm.DB, connectionMsg *models.ConnectionMessage) error {
	// 1. Upsert the current connection state.
	connectionState := &models.ConnectionState{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}
	if err := tx.Where("serial_number = ?", connectionMsg.SerialNumber).Assign(connectionState).FirstOrCreate(connectionState).Error; err != nil {
		return fmt.Errorf("failed to save connection state: %w", err)
	}

	// 2. Always create a new record in the history table.
	connectionHistory := &models.ConnectionStateHistory{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}
	if err := tx.Create(connectionHistory).Error; err != nil {
		return fmt.Errorf("failed to save connection history: %w", err)
	}

	return nil
}

// GetLastConnectionState retrieves the most recent connection state for a robot.
func (cr *ConnectionRepository) GetLastConnectionState(serialNumber string) (*models.ConnectionState, error) {
	var connectionState models.ConnectionState
	err := cr.db.Where("serial_number = ?", serialNumber).Order("created_at desc").First(&connectionState).Error
	if err != nil {
		return nil, fmt.Errorf("connection state not found for robot %s: %w", serialNumber, err)
	}
	return &connectionState, nil
}

// GetConnectionHistory retrieves connection history for a robot with pagination.
func (cr *ConnectionRepository) GetConnectionHistory(serialNumber string, limit int) ([]models.ConnectionState, error) {
	var connections []models.ConnectionState
	query := cr.db.Where("serial_number = ?", serialNumber).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&connections).Error; err != nil {
		return nil, fmt.Errorf("failed to get connection history: %w", err)
	}
	return connections, nil
}

// GetConnectedRobots retrieves all robots with ONLINE connection state.
func (cr *ConnectionRepository) GetConnectedRobots() ([]string, error) {
	var robots []string
	err := cr.db.Model(&models.ConnectionState{}).Where("connection_state = ?", "ONLINE").Distinct().Pluck("serial_number", &robots).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get connected robots: %w", err)
	}
	return robots, nil
}

// GetRobotManufacturer retrieves the manufacturer for a robot based on the latest connection.
func (cr *ConnectionRepository) GetRobotManufacturer(serialNumber string) (string, error) {
	var connectionState models.ConnectionState
	err := cr.db.Where("serial_number = ?", serialNumber).Order("created_at desc").First(&connectionState).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "Roboligent", nil // Return default if no record found
		}
		return "", fmt.Errorf("failed to get robot manufacturer: %w", err)
	}
	if connectionState.Manufacturer == "" {
		return "Roboligent", nil // Return default if manufacturer field is empty
	}
	return connectionState.Manufacturer, nil
}
