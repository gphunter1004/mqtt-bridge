package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/base"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// ActionRepository implements ActionRepositoryInterface using base CRUD
type ActionRepository struct {
	*base.BaseCRUDRepository[models.ActionTemplate]
	db *gorm.DB
}

// NewActionRepository creates a new instance of ActionRepository
func NewActionRepository(db *gorm.DB) interfaces.ActionRepositoryInterface {
	baseCRUD := base.NewBaseCRUDRepository[models.ActionTemplate](db, "action_templates")
	return &ActionRepository{
		BaseCRUDRepository: baseCRUD,
		db:                 db,
	}
}

// ===================================================================
// ACTION TEMPLATE CRUD OPERATIONS
// ===================================================================

// CreateActionTemplate creates a new action template with parameters
func (ar *ActionRepository) CreateActionTemplate(action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error) {
	var result *models.ActionTemplate
	var err error

	err = ar.WithTransaction(func(tx *gorm.DB) error {
		// Create the action template
		if err := ar.CreateWithTransaction(tx, action); err != nil {
			return err
		}

		// Create parameters using utils
		if err := ar.createActionParametersWithTx(tx, action.ID, parameters); err != nil {
			return fmt.Errorf("failed to create action parameters: %w", err)
		}

		// Get the created action with parameters
		result, err = ar.getActionTemplateWithTx(tx, action.ID)
		return err
	})

	return result, err
}

// GetActionTemplate retrieves an action template by database ID with parameters
func (ar *ActionRepository) GetActionTemplate(actionID uint) (*models.ActionTemplate, error) {
	return ar.getActionTemplateWithTx(ar.db, actionID)
}

// GetActionTemplateByActionID retrieves an action template by action ID
func (ar *ActionRepository) GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error) {
	var action models.ActionTemplate
	err := ar.db.Where("action_id = ?", actionID).
		Preload("Parameters").
		First(&action).Error

	return &action, base.HandleDBError("get", "action_templates", fmt.Sprintf("action ID '%s'", actionID), err)
}

// ListActionTemplates retrieves all action templates with pagination
func (ar *ActionRepository) ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error) {
	return ar.ListWithPagination(limit, offset, "created_at DESC")
}

// ListActionTemplatesByType retrieves action templates filtered by action type
func (ar *ActionRepository) ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error) {
	return ar.FilterByField("action_type", actionType, limit, offset)
}

// SearchActionTemplates searches action templates by term
func (ar *ActionRepository) SearchActionTemplates(searchTerm string, limit, offset int) ([]models.ActionTemplate, error) {
	return ar.SearchByField("action_type", searchTerm, limit, offset)
}

// GetActionTemplatesByBlockingType retrieves action templates filtered by blocking type
func (ar *ActionRepository) GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error) {
	return ar.FilterByField("blocking_type", blockingType, limit, offset)
}

// UpdateActionTemplate updates an existing action template
func (ar *ActionRepository) UpdateActionTemplate(actionID uint, action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error) {
	var result *models.ActionTemplate
	var err error

	err = ar.WithTransaction(func(tx *gorm.DB) error {
		// Update the action template using base method
		updateFields := map[string]interface{}{
			"action_type":        action.ActionType,
			"action_id":          action.ActionID,
			"blocking_type":      action.BlockingType,
			"action_description": action.ActionDescription,
		}

		if err := ar.UpdateWithTransaction(tx, actionID, updateFields); err != nil {
			return err
		}

		// Delete and recreate parameters
		if err := tx.Where("action_template_id = ?", actionID).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
			return base.WrapDBError("delete parameters", "action_parameter_templates", err)
		}

		if err := ar.createActionParametersWithTx(tx, actionID, parameters); err != nil {
			return err
		}

		result, err = ar.getActionTemplateWithTx(tx, actionID)
		return err
	})

	return result, err
}

// DeleteActionTemplate deletes an action template and its parameters
func (ar *ActionRepository) DeleteActionTemplate(actionID uint) error {
	return ar.WithTransaction(func(tx *gorm.DB) error {
		// Delete parameters first
		if err := tx.Where("action_template_id = ?", actionID).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
			return base.WrapDBError("delete parameters", "action_parameter_templates", err)
		}

		// Delete the action template using base method
		return ar.DeleteWithTransaction(tx, actionID)
	})
}

// CreateActionParameter creates a new action parameter
func (ar *ActionRepository) CreateActionParameter(actionTemplateID uint, parameter *models.ActionParameterTemplate) error {
	parameter.ActionTemplateID = actionTemplateID
	if err := ar.db.Create(parameter).Error; err != nil {
		return base.WrapDBError("create", "action_parameter_templates", err)
	}
	return nil
}

// DeleteActionParameters deletes all parameters for an action template
func (ar *ActionRepository) DeleteActionParameters(actionTemplateID uint) error {
	if err := ar.db.Where("action_template_id = ?", actionTemplateID).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
		return base.WrapDBError("delete parameters", "action_parameter_templates", err)
	}
	return nil
}

// ===================================================================
// PRIVATE HELPER METHODS
// ===================================================================

// getActionTemplateWithTx retrieves action template with parameters
func (ar *ActionRepository) getActionTemplateWithTx(tx *gorm.DB, actionID uint) (*models.ActionTemplate, error) {
	var action models.ActionTemplate
	err := tx.Where("id = ?", actionID).
		Preload("Parameters").
		First(&action).Error

	return &action, base.HandleDBError("get", "action_templates", fmt.Sprintf("ID %d", actionID), err)
}

// createActionParametersWithTx creates action parameters within transaction
func (ar *ActionRepository) createActionParametersWithTx(tx *gorm.DB, actionTemplateID uint, parameters []models.ActionParameterTemplateRequest) error {
	for _, paramReq := range parameters {
		// Use utils helper for value conversion
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
			return base.WrapDBError("create parameter", "action_parameter_templates", err)
		}
	}
	return nil
}
