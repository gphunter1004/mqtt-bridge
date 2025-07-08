package base

import (
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"
)

// ===================================================================
// MANUFACTURER COMMON MANAGEMENT
// ===================================================================

// ManufacturerManager handles manufacturer-related operations
// Used across multiple services to avoid duplication
type ManufacturerManager struct {
	connectionRepo interfaces.ConnectionRepositoryInterface
}

// NewManufacturerManager creates a new manufacturer manager
func NewManufacturerManager(connectionRepo interfaces.ConnectionRepositoryInterface) *ManufacturerManager {
	return &ManufacturerManager{
		connectionRepo: connectionRepo,
	}
}

// GetRobotManufacturer retrieves manufacturer for a robot with fallback to default
func (mm *ManufacturerManager) GetRobotManufacturer(serialNumber string) string {
	manufacturer, err := mm.connectionRepo.GetRobotManufacturer(serialNumber)
	if err != nil {
		utils.LogError(utils.LogComponentService, "Failed to get manufacturer for robot %s: %v", serialNumber, err)
		return utils.GetDefaultManufacturer()
	}

	return utils.GetManufacturerOrDefault(manufacturer)
}

// GetRobotManufacturerWithLogging retrieves manufacturer with detailed logging
func (mm *ManufacturerManager) GetRobotManufacturerWithLogging(serialNumber string) string {
	manufacturer, err := mm.connectionRepo.GetRobotManufacturer(serialNumber)
	if err != nil {
		utils.LogWarn(utils.LogComponentService, "No manufacturer found for robot %s, using default: %s", serialNumber, utils.GetDefaultManufacturer())
		return utils.GetDefaultManufacturer()
	}

	if manufacturer == "" {
		utils.LogDebug(utils.LogComponentService, "Empty manufacturer for robot %s, using default: %s", serialNumber, utils.GetDefaultManufacturer())
		return utils.GetDefaultManufacturer()
	}

	utils.LogDebug(utils.LogComponentService, "Manufacturer for robot %s: %s", serialNumber, manufacturer)
	return manufacturer
}

// ValidateManufacturer validates manufacturer string
func (mm *ManufacturerManager) ValidateManufacturer(manufacturer string) string {
	return utils.GetManufacturerOrDefault(manufacturer)
}

// GetDefaultManufacturer returns the default manufacturer
func (mm *ManufacturerManager) GetDefaultManufacturer() string {
	return utils.GetDefaultManufacturer()
}

// SetManufacturerForMessage sets manufacturer field in message if empty
func (mm *ManufacturerManager) SetManufacturerForMessage(serialNumber string, currentManufacturer *string) {
	if currentManufacturer != nil && *currentManufacturer == "" {
		*currentManufacturer = mm.GetRobotManufacturer(serialNumber)
	}
}

// GetManufacturerMap returns manufacturer mapping for multiple robots
func (mm *ManufacturerManager) GetManufacturerMap(serialNumbers []string) map[string]string {
	manufacturerMap := make(map[string]string)

	for _, serialNumber := range serialNumbers {
		manufacturerMap[serialNumber] = mm.GetRobotManufacturer(serialNumber)
	}

	return manufacturerMap
}
