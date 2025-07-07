package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// ConnectionRepository implements ConnectionRepositoryInterface
type ConnectionRepository struct {
	db *gorm.DB
}

// NewConnectionRepository creates a new instance of ConnectionRepository
func NewConnectionRepository(db *gorm.DB) interfaces.ConnectionRepositoryInterface {
	return &ConnectionRepository{
		db: db,
	}
}

// SaveConnectionState saves or updates the connection state for a robot
func (cr *ConnectionRepository) SaveConnectionState(connectionMsg *models.ConnectionMessage) error {
	// Save current connection state (upsert)
	connectionState := &models.ConnectionState{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}

	err := cr.db.Where("serial_number = ?", connectionMsg.SerialNumber).
		Assign(connectionState).
		FirstOrCreate(connectionState).Error

	if err != nil {
		return fmt.Errorf("failed to save connection state: %w", err)
	}

	// Save to history (always create new record)
	connectionHistory := &models.ConnectionStateHistory{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}

	if err := cr.db.Create(connectionHistory).Error; err != nil {
		return fmt.Errorf("failed to save connection history: %w", err)
	}

	return nil
}

// GetLastConnectionState retrieves the most recent connection state for a robot
func (cr *ConnectionRepository) GetLastConnectionState(serialNumber string) (*models.ConnectionState, error) {
	var connectionState models.ConnectionState
	err := cr.db.Where("serial_number = ?", serialNumber).
		Order("created_at desc").
		First(&connectionState).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("connection state not found for robot %s", serialNumber)
		}
		return nil, fmt.Errorf("failed to get connection state: %w", err)
	}

	return &connectionState, nil
}

// GetConnectionHistory retrieves connection history for a robot with pagination
func (cr *ConnectionRepository) GetConnectionHistory(serialNumber string, limit int) ([]models.ConnectionState, error) {
	var connections []models.ConnectionState
	query := cr.db.Where("serial_number = ?", serialNumber).Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&connections).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get connection history: %w", err)
	}

	return connections, nil
}

// GetConnectedRobots retrieves all robots with ONLINE connection state
func (cr *ConnectionRepository) GetConnectedRobots() ([]string, error) {
	var robots []string
	var connections []models.ConnectionState

	// Get distinct robots with ONLINE status, using subquery to get latest record per robot
	err := cr.db.Raw(`
		SELECT DISTINCT ON (serial_number) serial_number, connection_state, created_at
		FROM connection_states 
		WHERE connection_state = 'ONLINE'
		ORDER BY serial_number, created_at DESC
	`).Scan(&connections).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get connected robots: %w", err)
	}

	for _, conn := range connections {
		robots = append(robots, conn.SerialNumber)
	}

	return robots, nil
}

// GetRobotManufacturer retrieves the manufacturer for a robot based on latest connection
func (cr *ConnectionRepository) GetRobotManufacturer(serialNumber string) (string, error) {
	var connectionState models.ConnectionState
	err := cr.db.Where("serial_number = ?", serialNumber).
		Order("created_at desc").
		First(&connectionState).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default manufacturer if no connection record found
			return "Roboligent", nil
		}
		return "", fmt.Errorf("failed to get robot manufacturer: %w", err)
	}

	if connectionState.Manufacturer == "" {
		return "Roboligent", nil
	}

	return connectionState.Manufacturer, nil
}
