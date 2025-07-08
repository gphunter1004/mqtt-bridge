package repositories

import (
	"fmt"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// ActionRepository implements ActionRepositoryInterface.
type ActionRepository struct {
	db *gorm.DB
}

// NewActionRepository creates a new instance of ActionRepository.
func NewActionRepository(db *gorm.DB) interfaces.ActionRepositoryInterface {
	return &ActionRepository{
		db: db,
	}
}

// CreateActionTemplate creates a new action template with parameters within a given transaction.
func (ar *ActionRepository) CreateActionTemplate(tx *gorm.DB, action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error) {
	if err := tx.Create(action).Error; err != nil {
		return nil, fmt.Errorf("failed to create action template: %w", err)
	}

	if err := ar.createActionParameters(tx, action.ID, parameters); err != nil {
		return nil, fmt.Errorf("failed to create action parameters: %w", err)
	}

	return ar.getActionTemplateWithTx(tx, action.ID)
}

// GetActionTemplate retrieves an action template by database ID with parameters.
func (ar *ActionRepository) GetActionTemplate(actionID uint) (*models.ActionTemplate, error) {
	return ar.getActionTemplateWithTx(ar.db, actionID)
}

// GetActionTemplateByActionID retrieves an action template by action ID with parameters.
func (ar *ActionRepository) GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error) {
	var action models.ActionTemplate
	err := ar.db.Where("action_id = ?", actionID).Preload("Parameters").First(&action).Error
	if err != nil {
		return nil, fmt.Errorf("action template with action ID '%s' not found: %w", actionID, err)
	}
	return &action, nil
}

// ListActionTemplates retrieves all action templates with pagination.
func (ar *ActionRepository) ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := ar.db.Preload("Parameters").Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&actions).Error; err != nil {
		return nil, fmt.Errorf("failed to list action templates: %w", err)
	}
	return actions, nil
}

// ListActionTemplatesByType retrieves action templates filtered by action type.
func (ar *ActionRepository) ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := ar.db.Where("action_type = ?", actionType).Preload("Parameters").Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&actions).Error; err != nil {
		return nil, fmt.Errorf("failed to list action templates by type: %w", err)
	}
	return actions, nil
}

// SearchActionTemplates searches action templates by term in type or description.
func (ar *ActionRepository) SearchActionTemplates(searchTerm string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	searchPattern := "%" + searchTerm + "%"
	query := ar.db.Where("action_type ILIKE ? OR action_description ILIKE ?", searchPattern, searchPattern).
		Preload("Parameters").Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&actions).Error; err != nil {
		return nil, fmt.Errorf("failed to search action templates: %w", err)
	}
	return actions, nil
}

// GetActionTemplatesByBlockingType retrieves action templates filtered by blocking type.
func (ar *ActionRepository) GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := ar.db.Where("blocking_type = ?", blockingType).Preload("Parameters").Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&actions).Error; err != nil {
		return nil, fmt.Errorf("failed to get action templates by blocking type: %w", err)
	}
	return actions, nil
}

// UpdateActionTemplate updates an existing action template within a transaction.
func (ar *ActionRepository) UpdateActionTemplate(tx *gorm.DB, actionID uint, action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error) {
	if err := tx.First(&models.ActionTemplate{}, actionID).Error; err != nil {
		return nil, fmt.Errorf("action template with ID %d not found: %w", actionID, err)
	}

	updateFields := map[string]interface{}{
		"action_type":        action.ActionType,
		"action_id":          action.ActionID,
		"blocking_type":      action.BlockingType,
		"action_description": action.ActionDescription,
	}
	if err := tx.Model(&models.ActionTemplate{}).Where("id = ?", actionID).Updates(updateFields).Error; err != nil {
		return nil, fmt.Errorf("failed to update action template: %w", err)
	}

	if err := ar.deleteActionParametersWithTx(tx, actionID); err != nil {
		return nil, fmt.Errorf("failed to delete existing parameters: %w", err)
	}

	if err := ar.createActionParameters(tx, actionID, parameters); err != nil {
		return nil, fmt.Errorf("failed to create new parameters: %w", err)
	}

	return ar.getActionTemplateWithTx(tx, actionID)
}

// DeleteActionTemplate deletes one or more action templates and their parameters within a transaction.
func (ar *ActionRepository) DeleteActionTemplate(tx *gorm.DB, actionIDs ...uint) error {
	if len(actionIDs) == 0 {
		return nil
	}
	// First, delete all associated parameters in a single query.
	if err := tx.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
		return fmt.Errorf("failed to delete action parameters: %w", err)
	}
	// Then, delete all action templates in a single query.
	if err := tx.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{}).Error; err != nil {
		return fmt.Errorf("failed to delete action templates: %w", err)
	}
	return nil
}

// Private helper methods
func (ar *ActionRepository) getActionTemplateWithTx(tx *gorm.DB, actionID uint) (*models.ActionTemplate, error) {
	var action models.ActionTemplate
	err := tx.Where("id = ?", actionID).Preload("Parameters").First(&action).Error
	if err != nil {
		return nil, fmt.Errorf("action template with ID %d not found: %w", actionID, err)
	}
	return &action, nil
}

func (ar *ActionRepository) createActionParameters(tx *gorm.DB, actionTemplateID uint, parameters []models.ActionParameterTemplateRequest) error {
	if len(parameters) == 0 {
		return nil
	}
	var paramsToCreate []models.ActionParameterTemplate
	for _, paramReq := range parameters {
		valueStr, err := utils.ConvertValueToString(paramReq.Value, paramReq.ValueType)
		if err != nil {
			return fmt.Errorf("failed to convert parameter value for key '%s': %w", paramReq.Key, err)
		}
		paramsToCreate = append(paramsToCreate, models.ActionParameterTemplate{
			ActionTemplateID: actionTemplateID,
			Key:              paramReq.Key,
			Value:            valueStr,
			ValueType:        paramReq.ValueType,
		})
	}
	// Use GORM's batch insert for efficiency
	if err := tx.Create(&paramsToCreate).Error; err != nil {
		return fmt.Errorf("failed to bulk create parameters: %w", err)
	}
	return nil
}

func (ar *ActionRepository) deleteActionParametersWithTx(tx *gorm.DB, actionTemplateID uint) error {
	return tx.Where("action_template_id = ?", actionTemplateID).Delete(&models.ActionParameterTemplate{}).Error
}
