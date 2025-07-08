package services

import (
	"encoding/json"
	"fmt"
	"log"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// ActionService handles business logic related to action templates.
type ActionService struct {
	actionRepo interfaces.ActionRepositoryInterface
	uow        database.UnitOfWorkInterface
}

// NewActionService creates a new instance of ActionService.
func NewActionService(actionRepo interfaces.ActionRepositoryInterface, uow database.UnitOfWorkInterface) *ActionService {
	return &ActionService{
		actionRepo: actionRepo,
		uow:        uow,
	}
}

// CreateActionTemplate creates a new action template.
// It manages its own transaction if one isn't provided (tx is nil).
func (as *ActionService) CreateActionTemplate(tx *gorm.DB, req *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	isOwnTransaction := tx == nil
	if isOwnTransaction {
		tx = as.uow.Begin()
		defer func() {
			if r := recover(); r != nil {
				as.uow.Rollback(tx)
				panic(r)
			}
		}()
	}

	action := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}

	createdAction, err := as.actionRepo.CreateActionTemplate(tx, action, req.Parameters)
	if err != nil {
		if isOwnTransaction {
			as.uow.Rollback(tx)
		}
		return nil, utils.NewInternalServerError("Failed to create action template in repository", err)
	}

	if isOwnTransaction {
		if err := as.uow.Commit(tx); err != nil {
			return nil, utils.NewInternalServerError("Failed to commit transaction for action creation", err)
		}
	}
	return createdAction, nil
}

// RecreateActionTemplatesForOwner is a helper function used within a larger transaction by other services.
func (as *ActionService) RecreateActionTemplatesForOwner(tx *gorm.DB, oldActionIDsJSON string, newActions []models.ActionTemplateRequest) ([]uint, error) {
	if oldActionIDsJSON != "" {
		oldActionIDs, err := utils.ParseJSONToUintSlice(oldActionIDsJSON)
		if err == nil && len(oldActionIDs) > 0 {
			if err := as.actionRepo.DeleteActionTemplate(tx, oldActionIDs...); err != nil {
				log.Printf("Warning: failed to delete old action templates %v: %v", oldActionIDs, err)
			}
		}
	}

	var newActionIDs []uint
	for _, actionReq := range newActions {
		createdAction, err := as.CreateActionTemplate(tx, &actionReq)
		if err != nil {
			log.Printf("Warning: failed to create new action template during recreation: %v", err)
			continue
		}
		newActionIDs = append(newActionIDs, createdAction.ID)
	}
	return newActionIDs, nil
}

// UpdateActionTemplate updates an existing action template, managing its own transaction.
func (as *ActionService) UpdateActionTemplate(actionID uint, req *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	tx := as.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			as.uow.Rollback(tx)
			panic(r)
		}
	}()

	action := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}
	updatedAction, err := as.actionRepo.UpdateActionTemplate(tx, actionID, action, req.Parameters)
	if err != nil {
		as.uow.Rollback(tx)
		return nil, utils.NewInternalServerError(fmt.Sprintf("Failed to update action template with ID %d.", actionID), err)
	}

	if err := as.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit action template update.", err)
	}
	return updatedAction, nil
}

// DeleteActionTemplate deletes one or more action templates, managing its own transaction.
func (as *ActionService) DeleteActionTemplate(actionIDs ...uint) error {
	if len(actionIDs) == 0 {
		return nil
	}
	tx := as.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			as.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := as.actionRepo.DeleteActionTemplate(tx, actionIDs...); err != nil {
		as.uow.Rollback(tx)
		return utils.NewInternalServerError(fmt.Sprintf("Failed to delete action templates with IDs %v.", actionIDs), err)
	}
	return as.uow.Commit(tx)
}

// GetActionTemplate retrieves a single action template by its database ID.
func (as *ActionService) GetActionTemplate(actionID uint) (*models.ActionTemplate, error) {
	action, err := as.actionRepo.GetActionTemplate(actionID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Action template with ID %d not found.", actionID))
	}
	return action, nil
}

// GetActionTemplateByActionID retrieves a single action template by its string ID.
func (as *ActionService) GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error) {
	action, err := as.actionRepo.GetActionTemplateByActionID(actionID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Action template with actionId '%s' not found.", actionID))
	}
	return action, nil
}

// ListActionTemplates retrieves a paginated list of all action templates.
func (as *ActionService) ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error) {
	actions, err := as.actionRepo.ListActionTemplates(limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to list action templates.", err)
	}
	return actions, nil
}

// ListActionTemplatesByType retrieves action templates filtered by type.
func (as *ActionService) ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error) {
	actions, err := as.actionRepo.ListActionTemplatesByType(actionType, limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to list action templates by type.", err)
	}
	return actions, nil
}

// SearchActionTemplates searches for action templates based on a search term.
func (as *ActionService) SearchActionTemplates(searchTerm string, limit, offset int) ([]models.ActionTemplate, error) {
	actions, err := as.actionRepo.SearchActionTemplates(searchTerm, limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to search action templates.", err)
	}
	return actions, nil
}

// GetActionTemplatesByBlockingType retrieves action templates filtered by blocking type.
func (as *ActionService) GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error) {
	actions, err := as.actionRepo.GetActionTemplatesByBlockingType(blockingType, limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to get action templates by blocking type.", err)
	}
	return actions, nil
}

// CloneActionTemplate creates a copy of an existing action template with a new ID.
func (as *ActionService) CloneActionTemplate(sourceActionID uint, newActionID string) (*models.ActionTemplate, error) {
	sourceAction, err := as.GetActionTemplate(sourceActionID)
	if err != nil {
		return nil, err
	}

	req := &models.ActionTemplateRequest{
		ActionType:        sourceAction.ActionType,
		ActionID:          newActionID,
		BlockingType:      sourceAction.BlockingType,
		ActionDescription: sourceAction.ActionDescription + " (cloned)",
		Parameters:        make([]models.ActionParameterTemplateRequest, len(sourceAction.Parameters)),
	}

	for i, param := range sourceAction.Parameters {
		var value interface{}
		if param.Value != "" {
			if json.Unmarshal([]byte(param.Value), &value) != nil {
				value = param.Value
			}
		}
		req.Parameters[i] = models.ActionParameterTemplateRequest{Key: param.Key, Value: value, ValueType: param.ValueType}
	}

	// Since CreateActionTemplate can now manage its own transaction, we pass nil.
	return as.CreateActionTemplate(nil, req)
}
