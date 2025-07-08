package interfaces

import (
	"mqtt-bridge/models"

	"gorm.io/gorm" // gorm 패키지 임포트
)

// FactsheetRepositoryInterface defines the contract for factsheet data access.
type FactsheetRepositoryInterface interface {
	// SaveOrUpdateFactsheet saves or updates the complete factsheet for a robot within a transaction.
	SaveOrUpdateFactsheet(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error

	// GetPhysicalParameters retrieves physical parameters for a robot.
	GetPhysicalParameters(serialNumber string) (*models.PhysicalParameter, error)

	// GetTypeSpecification retrieves type specification for a robot.
	GetTypeSpecification(serialNumber string) (*models.TypeSpecification, error)

	// GetAgvActions retrieves all AGV actions for a robot with parameters.
	GetAgvActions(serialNumber string) ([]models.AgvAction, error)

	// GetRobotCapabilities retrieves complete robot capabilities (physical params, type spec, actions).
	GetRobotCapabilities(serialNumber string) (*models.RobotCapabilities, error)

	// DebugAgvActions logs debug information about AGV actions for a robot.
	DebugAgvActions(serialNumber string)
}
