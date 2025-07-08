package services

import (
	"encoding/json"
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
)

type ActionService struct {
	actionRepo interfaces.ActionRepositoryInterface
}

func NewActionService(actionRepo interfaces.ActionRepositoryInterface) *ActionService {
	return &ActionService{
		actionRepo: actionRepo,
	}
}

func (as *ActionService) CreateActionTemplate(req *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	// Prepare action template
	action := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}

	// Create action template with parameters using repository
	return as.actionRepo.CreateActionTemplate(action, req.Parameters)
}

func (as *ActionService) GetActionTemplate(actionID uint) (*models.ActionTemplate, error) {
	return as.actionRepo.GetActionTemplate(actionID)
}

func (as *ActionService) GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error) {
	return as.actionRepo.GetActionTemplateByActionID(actionID)
}

func (as *ActionService) ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error) {
	return as.actionRepo.ListActionTemplates(limit, offset)
}

func (as *ActionService) ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error) {
	return as.actionRepo.ListActionTemplatesByType(actionType, limit, offset)
}

func (as *ActionService) UpdateActionTemplate(actionID uint, req *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	// Prepare updated action template
	action := &models.ActionTemplate{
		ActionType:        req.ActionType,
		ActionID:          req.ActionID,
		BlockingType:      req.BlockingType,
		ActionDescription: req.ActionDescription,
	}

	// Update action template with parameters using repository
	return as.actionRepo.UpdateActionTemplate(actionID, action, req.Parameters)
}

func (as *ActionService) DeleteActionTemplate(actionID uint) error {
	return as.actionRepo.DeleteActionTemplate(actionID)
}

func (as *ActionService) CloneActionTemplate(sourceActionID uint, newActionID string) (*models.ActionTemplate, error) {
	// Get source action template
	sourceAction, err := as.actionRepo.GetActionTemplate(sourceActionID)
	if err != nil {
		return nil, fmt.Errorf("source action template not found: %w", err)
	}

	// Prepare cloned action template request
	req := &models.ActionTemplateRequest{
		ActionType:        sourceAction.ActionType,
		ActionID:          newActionID,
		BlockingType:      sourceAction.BlockingType,
		ActionDescription: sourceAction.ActionDescription + " (cloned)",
		Parameters:        make([]models.ActionParameterTemplateRequest, len(sourceAction.Parameters)),
	}

	// Convert parameters
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
	return as.actionRepo.SearchActionTemplates(searchTerm, limit, offset)
}

func (as *ActionService) GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error) {
	return as.actionRepo.GetActionTemplatesByBlockingType(blockingType, limit, offset)
}
