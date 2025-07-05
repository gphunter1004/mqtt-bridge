package services

import (
	"encoding/json"
	"fmt"
	"log"

	"mqtt-bridge/database"
	"mqtt-bridge/models"

	"gorm.io/gorm"
)

type ActionService struct {
	db *database.Database
}

func NewActionService(db *database.Database) *ActionService {
	return &ActionService{
		db: db,
	}
}

// Independent Action Template Management

func (as *ActionService) CreateActionTemplate(req *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	// Create independent action template
	action := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}

	// Start transaction
	tx := as.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create action
	if err := tx.Create(action).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create action template: %w", err)
	}

	// Create action parameters
	for _, paramReq := range req.Parameters {
		if err := as.createActionParameter(tx, action.ID, &paramReq); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action parameter: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Action Service] Action template created successfully: %s (ID: %d)", action.ActionType, action.ID)

	// Fetch the complete action with parameters
	return as.GetActionTemplate(action.ID)
}

func (as *ActionService) GetActionTemplate(actionID uint) (*models.ActionTemplate, error) {
	var action models.ActionTemplate
	err := as.db.DB.Where("id = ?", actionID).
		Preload("Parameters").
		First(&action).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get action template: %w", err)
	}

	return &action, nil
}

func (as *ActionService) GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error) {
	var action models.ActionTemplate
	err := as.db.DB.Where("action_id = ?", actionID).
		Preload("Parameters").
		First(&action).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get action template: %w", err)
	}

	return &action, nil
}

func (as *ActionService) ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := as.db.DB.Preload("Parameters").Order("created_at DESC")

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

	log.Printf("[Action Service] Retrieved %d independent action templates", len(actions))
	return actions, nil
}

func (as *ActionService) ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := as.db.DB.Where("action_type = ?", actionType).
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

func (as *ActionService) UpdateActionTemplate(actionID uint, req *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	// Get existing action
	existingAction, err := as.GetActionTemplate(actionID)
	if err != nil {
		return nil, fmt.Errorf("action template not found: %w", err)
	}

	// Check for actionID conflicts (if actionID is being changed)
	if existingAction.ActionID != req.ActionID {
		var conflictEdge models.ActionTemplate
		err := as.db.DB.Where("action_id = ? AND id != ?",
			req.ActionID, actionID).First(&conflictEdge).Error
		if err == nil {
			return nil, fmt.Errorf("action with ID '%s' already exists", req.ActionID)
		}
	}

	// Start transaction
	tx := as.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update action
	updateAction := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}

	if err := tx.Model(&models.ActionTemplate{}).Where("id = ?", actionID).Updates(updateAction).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update action template: %w", err)
	}

	// Delete existing parameters
	if err := tx.Where("action_template_id = ?", actionID).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete existing parameters: %w", err)
	}

	// Create new parameters
	for _, paramReq := range req.Parameters {
		if err := as.createActionParameter(tx, actionID, &paramReq); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create parameter: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Action Service] Action template updated successfully: %s (ID: %d)", req.ActionType, actionID)
	return as.GetActionTemplate(actionID)
}

func (as *ActionService) DeleteActionTemplate(actionID uint) error {
	// Get action info for logging
	action, err := as.GetActionTemplate(actionID)
	if err != nil {
		return fmt.Errorf("action template not found: %w", err)
	}

	// Start transaction
	tx := as.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete parameters
	if err := tx.Where("action_template_id = ?", actionID).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete action parameters: %w", err)
	}

	// Delete action
	if err := tx.Delete(&models.ActionTemplate{}, actionID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete action template: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Action Service] Action template deleted successfully: %s (ID: %d)", action.ActionType, actionID)
	return nil
}

// Action Library Management - Predefined action templates

func (as *ActionService) CreateActionLibrary(req *models.ActionLibraryRequest) (*models.ActionTemplate, error) {
	// Create a library action template that can be reused
	actionReq := &models.ActionTemplateRequest{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
		Parameters:        req.Parameters,
	}

	return as.CreateActionTemplate(actionReq)
}

func (as *ActionService) GetActionLibrary(limit, offset int) ([]models.ActionTemplate, error) {
	// Return all independent action templates as library
	return as.ListActionTemplates(limit, offset)
}

func (as *ActionService) CloneActionTemplate(sourceActionID uint, newActionID string) (*models.ActionTemplate, error) {
	// Get source action
	sourceAction, err := as.GetActionTemplate(sourceActionID)
	if err != nil {
		return nil, fmt.Errorf("source action template not found: %w", err)
	}

	// Create request for new action
	req := &models.ActionTemplateRequest{
		ActionType:        sourceAction.ActionType,
		ActionID:          newActionID,
		BlockingType:      sourceAction.BlockingType,
		ActionDescription: sourceAction.ActionDescription + " (cloned)",
		Parameters:        make([]models.ActionParameterTemplateRequest, len(sourceAction.Parameters)),
	}

	// Copy parameters
	for i, param := range sourceAction.Parameters {
		var value interface{}
		if param.Value != "" {
			switch param.ValueType {
			case "object", "number", "boolean":
				json.Unmarshal([]byte(param.Value), &value)
			default:
				value = param.Value
			}
		}

		req.Parameters[i] = models.ActionParameterTemplateRequest{
			Key:       param.Key,
			Value:     value,
			ValueType: param.ValueType,
		}
	}

	return as.CreateActionTemplate(req)
}

// Search and filter actions

func (as *ActionService) SearchActionTemplates(searchTerm string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := as.db.DB.Where("action_type ILIKE ? OR action_description ILIKE ?", "%"+searchTerm+"%", "%"+searchTerm+"%").
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

func (as *ActionService) GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error) {
	var actions []models.ActionTemplate
	query := as.db.DB.Where("blocking_type = ?", blockingType).
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

// Helper functions

func (as *ActionService) createActionParameter(tx *gorm.DB, actionID uint, paramReq *models.ActionParameterTemplateRequest) error {
	// Convert value to JSON string based on type
	valueStr, err := as.convertValueToString(paramReq.Value, paramReq.ValueType)
	if err != nil {
		return fmt.Errorf("failed to convert parameter value: %w", err)
	}

	param := &models.ActionParameterTemplate{
		ActionTemplateID: actionID,
		Key:              paramReq.Key,
		Value:            valueStr,
		ValueType:        paramReq.ValueType,
	}

	return tx.Create(param).Error
}

func (as *ActionService) convertValueToString(value interface{}, valueType string) (string, error) {
	if value == nil {
		return "", nil
	}

	switch valueType {
	case "string":
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
	case "object", "number", "boolean":
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}
