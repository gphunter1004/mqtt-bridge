package services

import (
	"encoding/json"
	"fmt"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/utils"
)

type ActionService struct {
	db *database.Database
}

func NewActionService(db *database.Database) *ActionService {
	return &ActionService{
		db: db,
	}
}

func (as *ActionService) CreateActionTemplate(req *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	action := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}

	if err := as.db.DB.Create(action).Error; err != nil {
		return nil, fmt.Errorf("failed to create action template: %w", err)
	}

	for _, paramReq := range req.Parameters {
		as.createActionParameter(action.ID, &paramReq)
	}

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
	existingAction, err := as.GetActionTemplate(actionID)
	if err != nil {
		return nil, fmt.Errorf("action template not found: %w", err)
	}

	if existingAction.ActionID != req.ActionID {
		var conflictAction models.ActionTemplate
		err := as.db.DB.Where("action_id = ? AND id != ?",
			req.ActionID, actionID).First(&conflictAction).Error
		if err == nil {
			return nil, fmt.Errorf("action with ID '%s' already exists", req.ActionID)
		}
	}

	updateAction := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}

	if err := as.db.DB.Model(&models.ActionTemplate{}).Where("id = ?", actionID).Updates(updateAction).Error; err != nil {
		return nil, fmt.Errorf("failed to update action template: %w", err)
	}

	as.db.DB.Where("action_template_id = ?", actionID).Delete(&models.ActionParameterTemplate{})

	for _, paramReq := range req.Parameters {
		as.createActionParameter(actionID, &paramReq)
	}

	return as.GetActionTemplate(actionID)
}

func (as *ActionService) DeleteActionTemplate(actionID uint) error {
	as.db.DB.Where("action_template_id = ?", actionID).Delete(&models.ActionParameterTemplate{})

	if err := as.db.DB.Delete(&models.ActionTemplate{}, actionID).Error; err != nil {
		return fmt.Errorf("failed to delete action template: %w", err)
	}

	return nil
}

func (as *ActionService) CreateActionLibrary(req *models.ActionLibraryRequest) (*models.ActionTemplate, error) {
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
	return as.ListActionTemplates(limit, offset)
}

func (as *ActionService) CloneActionTemplate(sourceActionID uint, newActionID string) (*models.ActionTemplate, error) {
	sourceAction, err := as.GetActionTemplate(sourceActionID)
	if err != nil {
		return nil, fmt.Errorf("source action template not found: %w", err)
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

func (as *ActionService) createActionParameter(actionID uint, paramReq *models.ActionParameterTemplateRequest) error {
	valueStr, err := utils.ConvertValueToString(paramReq.Value, paramReq.ValueType)
	if err != nil {
		return fmt.Errorf("failed to convert parameter value: %w", err)
	}

	param := &models.ActionParameterTemplate{
		ActionTemplateID: actionID,
		Key:              paramReq.Key,
		Value:            valueStr,
		ValueType:        paramReq.ValueType,
	}

	return as.db.DB.Create(param).Error
}
