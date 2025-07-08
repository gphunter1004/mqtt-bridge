package repositories

import (
	"encoding/json"
	"fmt"
	"log"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// FactsheetRepository implements FactsheetRepositoryInterface.
type FactsheetRepository struct {
	db *gorm.DB
}

// NewFactsheetRepository creates a new instance of FactsheetRepository.
func NewFactsheetRepository(db *gorm.DB) interfaces.FactsheetRepositoryInterface {
	return &FactsheetRepository{
		db: db,
	}
}

// SaveOrUpdateFactsheet saves or updates the complete factsheet for a robot within a transaction.
func (fr *FactsheetRepository) SaveOrUpdateFactsheet(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	// The transaction is now managed by the service layer.
	// This method will use the provided 'tx' for all its operations.
	if err := fr.deleteExistingFactsheetData(tx, factsheetMsg.SerialNumber); err != nil {
		return fmt.Errorf("failed to delete existing factsheet data: %w", err)
	}
	if err := fr.savePhysicalParameters(tx, factsheetMsg); err != nil {
		return fmt.Errorf("failed to save physical parameters: %w", err)
	}
	if err := fr.saveTypeSpecification(tx, factsheetMsg); err != nil {
		return fmt.Errorf("failed to save type specification: %w", err)
	}
	if err := fr.saveAgvActions(tx, factsheetMsg); err != nil {
		return fmt.Errorf("failed to save AGV actions: %w", err)
	}
	return nil
}

// GetPhysicalParameters retrieves physical parameters for a robot.
func (fr *FactsheetRepository) GetPhysicalParameters(serialNumber string) (*models.PhysicalParameter, error) {
	return FindByField[models.PhysicalParameter](fr.db, "serial_number", serialNumber)
}

// GetTypeSpecification retrieves type specification for a robot.
func (fr *FactsheetRepository) GetTypeSpecification(serialNumber string) (*models.TypeSpecification, error) {
	return FindByField[models.TypeSpecification](fr.db, "serial_number", serialNumber)
}

// GetAgvActions retrieves all AGV actions for a robot with parameters.
func (fr *FactsheetRepository) GetAgvActions(serialNumber string) ([]models.AgvAction, error) {
	var actions []models.AgvAction
	err := fr.db.Where("serial_number = ?", serialNumber).Preload("Parameters").Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get AGV actions: %w", err)
	}
	return actions, nil
}

// GetRobotCapabilities retrieves complete robot capabilities.
func (fr *FactsheetRepository) GetRobotCapabilities(serialNumber string) (*models.RobotCapabilities, error) {
	physicalParams, err := fr.GetPhysicalParameters(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get physical parameters: %w", err)
	}
	typeSpec, err := fr.GetTypeSpecification(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get type specification: %w", err)
	}
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

// DebugAgvActions logs debug information about AGV actions for a robot.
func (fr *FactsheetRepository) DebugAgvActions(serialNumber string) {
	var actions []models.AgvAction
	fr.db.Where("serial_number = ?", serialNumber).Preload("Parameters").Find(&actions)
	log.Printf("[DB DEBUG] Found %d AGV actions for robot: %s", len(actions), serialNumber)
}

// Private helper methods
func (fr *FactsheetRepository) deleteExistingFactsheetData(tx *gorm.DB, serialNumber string) error {
	// Use tx instead of fr.db
	if err := tx.Exec("DELETE FROM agv_action_parameters WHERE agv_action_id IN (SELECT id FROM agv_actions WHERE serial_number = ?)", serialNumber).Error; err != nil {
		return err
	}
	if err := tx.Where("serial_number = ?", serialNumber).Delete(&models.AgvAction{}).Error; err != nil {
		return err
	}
	if err := tx.Where("serial_number = ?", serialNumber).Delete(&models.PhysicalParameter{}).Error; err != nil {
		return err
	}
	if err := tx.Where("serial_number = ?", serialNumber).Delete(&models.TypeSpecification{}).Error; err != nil {
		return err
	}
	return nil
}

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
	return tx.Create(physicalParam).Error
}

func (fr *FactsheetRepository) saveTypeSpecification(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	localizationTypesJSON, _ := json.Marshal(factsheetMsg.TypeSpecification.LocalizationTypes)
	navigationTypesJSON, _ := json.Marshal(factsheetMsg.TypeSpecification.NavigationTypes)
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
	return tx.Create(typeSpec).Error
}

func (fr *FactsheetRepository) saveAgvActions(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	for _, action := range factsheetMsg.ProtocolFeatures.AgvActions {
		actionScopesJSON, _ := json.Marshal(action.ActionScopes)
		agvAction := &models.AgvAction{
			SerialNumber:      factsheetMsg.SerialNumber,
			ActionType:        action.ActionType,
			ActionDescription: action.ActionDescription,
			ActionScopes:      string(actionScopesJSON),
			ResultDescription: action.ResultDescription,
		}
		if err := tx.Create(agvAction).Error; err != nil {
			continue
		}
		for _, param := range action.ActionParameters {
			actionParam := &models.AgvActionParameter{
				AgvActionID:   agvAction.ID,
				Key:           param.Key,
				Description:   param.Description,
				IsOptional:    param.IsOptional,
				ValueDataType: param.ValueDataType,
			}
			tx.Create(actionParam)
		}
	}
	return nil
}
