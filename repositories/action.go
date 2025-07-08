package repositories

import (
	"fmt"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// ActionRepository implements ActionRepositoryInterface
type ActionRepository struct {
	db *gorm.DB
}

// NewActionRepository creates a new instance of ActionRepository
func NewActionRepository(db *gorm.DB) interfaces.ActionRepositoryInterface {
	return &ActionRepository{
		db: db,
	}
}

// CreateActionTemplate creates a new action template with parameters
func (ar *ActionRepository) CreateActionTemplate(action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error) {
	var result *models.ActionTemplate
	var err error

	err = ar.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(action).Error; err != nil {
			return fmt.Errorf("failed to create action template: %w", err)
		}
		if err := ar.createActionParameters(tx, action.ID, parameters); err != nil {
			return fmt.Errorf("failed to create action parameters: %w", err)
		}
		result, err = ar.getActionTemplateWithTx(tx, action.ID)
		if err != nil {
			return fmt.Errorf("failed to get created action template: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetActionTemplate retrieves an action template by database ID with parameters
func (ar *ActionRepository) GetActionTemplate(actionID uint) (*models.ActionTemplate, error) {
	// Use the generic helper function
	return FindByField[models.ActionTemplate](ar.db, "id", actionID)
}

// GetActionTemplateByActionID retrieves an action template by action ID with parameters
func (ar *ActionRepository) GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error) {
	// Use the generic helper function
	return FindByField[models.ActionTemplate](ar.db, "action_id", actionID)
}

// ListActionTemplates retrieves all action templates with pagination
func (ar *ActionRepository) ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := ar.db.Preload("Parameters").Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list action templates: %w", err)
	}
	return actions, nil
}

// ListActionTemplatesByType retrieves action templates filtered by action type
func (ar *ActionRepository) ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := ar.db.Where("action_type = ?", actionType).
		Preload("Parameters").
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list action templates by type: %w", err)
	}
	return actions, nil
}

// SearchActionTemplates searches action templates by term in type or description
func (ar *ActionRepository) SearchActionTemplates(searchTerm string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	searchPattern := "%" + searchTerm + "%"

	query := ar.db.Where("action_type ILIKE ? OR action_description ILIKE ?", searchPattern, searchPattern).
		Preload("Parameters").
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to search action templates: %w", err)
	}
	return actions, nil
}

// GetActionTemplatesByBlockingType retrieves action templates filtered by blocking type
func (ar *ActionRepository) GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := ar.db.Where("blocking_type = ?", blockingType).
		Preload("Parameters").
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get action templates by blocking type: %w", err)
	}
	return actions, nil
}

// UpdateActionTemplate updates an existing action template
func (ar *ActionRepository) UpdateActionTemplate(actionID uint, action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error) {
	var result *models.ActionTemplate
	var err error

	err = ar.db.Transaction(func(tx *gorm.DB) error {
		var existingAction models.ActionTemplate
		if err := tx.Where("id = ?", actionID).First(&existingAction).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("action template with ID %d not found", actionID)
			}
			return fmt.Errorf("failed to check existing action: %w", err)
		}

		if existingAction.ActionID != action.ActionID {
			var conflictAction models.ActionTemplate
			err := tx.Where("action_id = ? AND id != ?", action.ActionID, actionID).First(&conflictAction).Error
			if err == nil {
				return fmt.Errorf("action with ID '%s' already exists", action.ActionID)
			}
		}

		updateFields := map[string]interface{}{
			"action_type":        action.ActionType,
			"action_id":          action.ActionID,
			"blocking_type":      action.BlockingType,
			"action_description": action.ActionDescription,
		}

		if err := tx.Model(&models.ActionTemplate{}).Where("id = ?", actionID).Updates(updateFields).Error; err != nil {
			return fmt.Errorf("failed to update action template: %w", err)
		}

		if err := ar.deleteActionParametersWithTx(tx, actionID); err != nil {
			return fmt.Errorf("failed to delete existing parameters: %w", err)
		}

		if err := ar.createActionParameters(tx, actionID, parameters); err != nil {
			return fmt.Errorf("failed to create new parameters: %w", err)
		}

		result, err = ar.getActionTemplateWithTx(tx, actionID)
		if err != nil {
			return fmt.Errorf("failed to get updated action template: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteActionTemplate deletes an action template and its parameters
func (ar *ActionRepository) DeleteActionTemplate(actionID uint) error {
	return ar.db.Transaction(func(tx *gorm.DB) error {
		if err := ar.deleteActionParametersWithTx(tx, actionID); err != nil {
			return fmt.Errorf("failed to delete action parameters: %w", err)
		}
		if err := tx.Delete(&models.ActionTemplate{}, actionID).Error; err != nil {
			return fmt.Errorf("failed to delete action template: %w", err)
		}
		return nil
	})
}

// CreateActionParameter creates a new action parameter for an action template
func (ar *ActionRepository) CreateActionParameter(actionTemplateID uint, parameter *models.ActionParameterTemplate) error {
	parameter.ActionTemplateID = actionTemplateID
	if err := ar.db.Create(parameter).Error; err != nil {
		return fmt.Errorf("failed to create action parameter: %w", err)
	}
	return nil
}

// DeleteActionParameters deletes all parameters for an action template
func (ar *ActionRepository) DeleteActionParameters(actionTemplateID uint) error {
	return ar.deleteActionParametersWithTx(ar.db, actionTemplateID)
}

// Private helper methods
func (ar *ActionRepository) getActionTemplateWithTx(tx *gorm.DB, actionID uint) (*models.ActionTemplate, error) {
	var action models.ActionTemplate
	err := tx.Where("id = ?", actionID).Preload("Parameters").First(&action).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("action template with ID %d not found", actionID)
		}
		return nil, fmt.Errorf("failed to get action template: %w", err)
	}
	return &action, nil
}

func (ar *ActionRepository) createActionParameters(tx *gorm.DB, actionTemplateID uint, parameters []models.ActionParameterTemplateRequest) error {
	for _, paramReq := range parameters {
		valueStr, err := utils.ConvertValueToString(paramReq.Value, paramReq.ValueType)
		if err != nil {
			return fmt.Errorf("failed to convert parameter value: %w", err)
		}
		param := &models.ActionParameterTemplate{
			ActionTemplateID: actionTemplateID,
			Key:              paramReq.Key,
			Value:            valueStr,
			ValueType:        paramReq.ValueType,
		}
		if err := tx.Create(param).Error; err != nil {
			return fmt.Errorf("failed to create parameter '%s': %w", paramReq.Key, err)
		}
	}
	return nil
}

func (ar *ActionRepository) deleteActionParametersWithTx(tx *gorm.DB, actionTemplateID uint) error {
	if err := tx.Where("action_template_id = ?", actionTemplateID).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
		return fmt.Errorf("failed to delete action parameters: %w", err)
	}
	return nil
}
