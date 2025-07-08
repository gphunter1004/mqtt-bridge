package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/base"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// ConnectionRepository implements ConnectionRepositoryInterface using base CRUD
type ConnectionRepository struct {
	*base.BaseCRUDRepository[models.ConnectionState]
	db *gorm.DB
}

// NewConnectionRepository creates a new instance of ConnectionRepository
func NewConnectionRepository(db *gorm.DB) interfaces.ConnectionRepositoryInterface {
	baseCRUD := base.NewBaseCRUDRepository[models.ConnectionState](db, "connection_states")
	return &ConnectionRepository{
		BaseCRUDRepository: baseCRUD,
		db:                 db,
	}
}

// ===================================================================
// CONNECTION STATE OPERATIONS
// ===================================================================

// SaveConnectionState saves or updates the connection state for a robot
func (cr *ConnectionRepository) SaveConnectionState(connectionMsg *models.ConnectionMessage) error {
	return cr.WithTransaction(func(tx *gorm.DB) error {
		// Save current connection state (upsert)
		connectionState := &models.ConnectionState{
			SerialNumber:    connectionMsg.SerialNumber,
			ConnectionState: connectionMsg.ConnectionState,
			HeaderID:        connectionMsg.HeaderID,
			Timestamp:       connectionMsg.Timestamp,
			Version:         connectionMsg.Version,
			Manufacturer:    connectionMsg.Manufacturer,
		}

		err := tx.Where("serial_number = ?", connectionMsg.SerialNumber).
			Assign(connectionState).
			FirstOrCreate(connectionState).Error

		if err != nil {
			return base.WrapDBError("save connection state", "connection_states", err)
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

		if err := tx.Create(connectionHistory).Error; err != nil {
			return base.WrapDBError("save connection history", "connection_state_histories", err)
		}

		return nil
	})
}

// GetLastConnectionState retrieves the most recent connection state for a robot
func (cr *ConnectionRepository) GetLastConnectionState(serialNumber string) (*models.ConnectionState, error) {
	var connectionState models.ConnectionState
	err := cr.db.Where("serial_number = ?", serialNumber).
		Order("created_at desc").
		First(&connectionState).Error

	return &connectionState, base.HandleDBError("get", "connection_states", fmt.Sprintf("serial number '%s'", serialNumber), err)
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
		return nil, base.WrapDBError("get connection history", "connection_states", err)
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
		return nil, base.WrapDBError("get connected robots", "connection_states", err)
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
			utils.LogWarn(utils.LogComponentDB, "No connection record found for robot %s, using default manufacturer", serialNumber)
			return utils.GetDefaultManufacturer(), nil
		}
		return "", base.WrapDBError("get robot manufacturer", "connection_states", err)
	}

	return utils.GetManufacturerOrDefault(connectionState.Manufacturer), nil
}
