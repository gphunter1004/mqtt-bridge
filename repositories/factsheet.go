package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/base"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// FactsheetRepository implements FactsheetRepositoryInterface using base CRUD
type FactsheetRepository struct {
	db *gorm.DB
}

// NewFactsheetRepository creates a new instance of FactsheetRepository
func NewFactsheetRepository(db *gorm.DB) interfaces.FactsheetRepositoryInterface {
	return &FactsheetRepository{
		db: db,
	}
}

// ===================================================================
// FACTSHEET OPERATIONS
// ===================================================================

// SaveOrUpdateFactsheet saves or updates the complete factsheet for a robot
func (fr *FactsheetRepository) SaveOrUpdateFactsheet(factsheetMsg *models.FactsheetMessage) error {
	return fr.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing data for this robot
		if err := fr.deleteExistingFactsheetData(tx, factsheetMsg.SerialNumber); err != nil {
			return fmt.Errorf("failed to delete existing factsheet data: %w", err)
		}

		// Save physical parameters
		if err := fr.savePhysicalParameters(tx, factsheetMsg); err != nil {
			return fmt.Errorf("failed to save physical parameters: %w", err)
		}

		// Save type specification
		if err := fr.saveTypeSpecification(tx, factsheetMsg); err != nil {
			return fmt.Errorf("failed to save type specification: %w", err)
		}

		// Save AGV actions
		if err := fr.saveAgvActions(tx, factsheetMsg); err != nil {
			return fmt.Errorf("failed to save AGV actions: %w", err)
		}

		return nil
	})
}

// GetPhysicalParameters retrieves physical parameters for a robot
func (fr *FactsheetRepository) GetPhysicalParameters(serialNumber string) (*models.PhysicalParameter, error) {
	var physicalParams models.PhysicalParameter
	err := fr.db.Where("serial_number = ?", serialNumber).First(&physicalParams).Error
	return &physicalParams, base.HandleDBError("get", "physical_parameters", fmt.Sprintf("serial number '%s'", serialNumber), err)
}

// GetTypeSpecification retrieves type specification for a robot
func (fr *FactsheetRepository) GetTypeSpecification(serialNumber string) (*models.TypeSpecification, error) {
	var typeSpec models.TypeSpecification
	err := fr.db.Where("serial_number = ?", serialNumber).First(&typeSpec).Error
	return &typeSpec, base.HandleDBError("get", "type_specifications", fmt.Sprintf("serial number '%s'", serialNumber), err)
}

// GetAgvActions retrieves all AGV actions for a robot with parameters
func (fr *FactsheetRepository) GetAgvActions(serialNumber string) ([]models.AgvAction, error) {
	var actions []models.AgvAction
	err := fr.db.Where("serial_number = ?", serialNumber).
		Preload("Parameters").
		Find(&actions).Error
	if err != nil {
		return nil, base.WrapDBError("get AGV actions", "agv_actions", err)
	}
	return actions, nil
}

// GetRobotCapabilities retrieves complete robot capabilities
func (fr *FactsheetRepository) GetRobotCapabilities(serialNumber string) (*models.RobotCapabilities, error) {
	// Get physical parameters
	physicalParams, err := fr.GetPhysicalParameters(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get physical parameters: %w", err)
	}

	// Get type specification
	typeSpec, err := fr.GetTypeSpecification(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get type specification: %w", err)
	}

	// Get AGV actions
	actions, err := fr.GetAgvActions(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get AGV actions: %w", err)
	}

	return &models.RobotCapabilities{
		SerialNumber:       serialNumber,
		PhysicalParameters: *physicalParams,
		TypeSpecification:  *typeSpec,
		AvailableActions:   actions,
	}, nil
}

// DebugAgvActions logs debug information about AGV actions for a robot
func (fr *FactsheetRepository) DebugAgvActions(serialNumber string) {
	var actions []models.AgvAction
	fr.db.Where("serial_number = ?", serialNumber).
		Preload("Parameters").Find(&actions)

	utils.LogDebug(utils.LogComponentDB, "Found %d AGV actions for robot: %s", len(actions), serialNumber)
	for i, action := range actions {
		utils.LogDebug(utils.LogComponentDB, "Action %d: %s (%d parameters)", i+1, action.ActionType, len(action.Parameters))
	}
}

// ===================================================================
// PRIVATE HELPER METHODS
// ===================================================================

// deleteExistingFactsheetData deletes existing factsheet data for a robot
func (fr *FactsheetRepository) deleteExistingFactsheetData(tx *gorm.DB, serialNumber string) error {
	// Delete AGV action parameters first (foreign key constraint)
	if err := tx.Exec(`DELETE FROM agv_action_parameters 
		WHERE agv_action_id IN (
			SELECT id FROM agv_actions WHERE serial_number = ?
		)`, serialNumber).Error; err != nil {
		return base.WrapDBError("delete AGV action parameters", "agv_action_parameters", err)
	}

	// Delete AGV actions
	if err := tx.Where("serial_number = ?", serialNumber).Delete(&models.AgvAction{}).Error; err != nil {
		return base.WrapDBError("delete AGV actions", "agv_actions", err)
	}

	// Delete physical parameters
	if err := tx.Where("serial_number = ?", serialNumber).Delete(&models.PhysicalParameter{}).Error; err != nil {
		return base.WrapDBError("delete physical parameters", "physical_parameters", err)
	}

	// Delete type specification
	if err := tx.Where("serial_number = ?", serialNumber).Delete(&models.TypeSpecification{}).Error; err != nil {
		return base.WrapDBError("delete type specification", "type_specifications", err)
	}

	return nil
}

// savePhysicalParameters saves physical parameters for a robot
func (fr *FactsheetRepository) savePhysicalParameters(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	physicalParam := &models.PhysicalParameter{
		SerialNumber:    factsheetMsg.SerialNumber,
		AccelerationMax: factsheetMsg.PhysicalParameters.AccelerationMax,
		DecelerationMax: factsheetMsg.PhysicalParameters.DecelerationMax,
		HeightMax:       factsheetMsg.PhysicalParameters.HeightMax,
		HeightMin:       factsheetMsg.PhysicalParameters.HeightMin,
		Length:          factsheetMsg.PhysicalParameters.Length,
		SpeedMax:        factsheetMsg.PhysicalParameters.SpeedMax,
		SpeedMin:        factsheetMsg.PhysicalParameters.SpeedMin,
		Width:           factsheetMsg.PhysicalParameters.Width,
	}

	if err := tx.Create(physicalParam).Error; err != nil {
		return base.WrapDBError("create physical parameters", "physical_parameters", err)
	}
	return nil
}

// saveTypeSpecification saves type specification for a robot
func (fr *FactsheetRepository) saveTypeSpecification(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	// Use utils helper for JSON marshaling
	localizationTypesJSON, err := utils.SafeJSONMarshal(factsheetMsg.TypeSpecification.LocalizationTypes)
	if err != nil {
		return fmt.Errorf("failed to marshal localization types: %w", err)
	}

	navigationTypesJSON, err := utils.SafeJSONMarshal(factsheetMsg.TypeSpecification.NavigationTypes)
	if err != nil {
		return fmt.Errorf("failed to marshal navigation types: %w", err)
	}

	typeSpec := &models.TypeSpecification{
		SerialNumber:      factsheetMsg.SerialNumber,
		AgvClass:          factsheetMsg.TypeSpecification.AgvClass,
		AgvKinematics:     factsheetMsg.TypeSpecification.AgvKinematics,
		LocalizationTypes: string(localizationTypesJSON),
		MaxLoadMass:       factsheetMsg.TypeSpecification.MaxLoadMass,
		NavigationTypes:   string(navigationTypesJSON),
		SeriesDescription: factsheetMsg.TypeSpecification.SeriesDescription,
		SeriesName:        factsheetMsg.TypeSpecification.SeriesName,
	}

	if err := tx.Create(typeSpec).Error; err != nil {
		return base.WrapDBError("create type specification", "type_specifications", err)
	}
	return nil
}

// saveAgvActions saves AGV actions for a robot
func (fr *FactsheetRepository) saveAgvActions(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	for _, action := range factsheetMsg.ProtocolFeatures.AgvActions {
		// Use utils helper for JSON marshaling
		actionScopesJSON, err := utils.SafeJSONMarshal(action.ActionScopes)
		if err != nil {
			utils.LogError(utils.LogComponentDB, "Failed to marshal action scopes for %s: %v", action.ActionType, err)
			continue
		}

		agvAction := &models.AgvAction{
			SerialNumber:      factsheetMsg.SerialNumber,
			ActionType:        action.ActionType,
			ActionDescription: action.ActionDescription,
			ActionScopes:      string(actionScopesJSON),
			ResultDescription: action.ResultDescription,
		}

		if err := tx.Create(agvAction).Error; err != nil {
			utils.LogError(utils.LogComponentDB, "Failed to create AGV action %s: %v", action.ActionType, err)
			continue // Skip this action if creation fails
		}

		// Save action parameters
		for _, param := range action.ActionParameters {
			actionParam := &models.AgvActionParameter{
				AgvActionID:   agvAction.ID,
				Key:           param.Key,
				Description:   param.Description,
				IsOptional:    param.IsOptional,
				ValueDataType: param.ValueDataType,
			}

			if err := tx.Create(actionParam).Error; err != nil {
				utils.LogError(utils.LogComponentDB, "Failed to create action parameter %s for action %s: %v", param.Key, action.ActionType, err)
			}
		}
	}
	return nil
}
