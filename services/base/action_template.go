package base

import (
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
)

// ===================================================================
// ACTION TEMPLATE COMMON MANAGEMENT
// ===================================================================

// ActionTemplateManager handles common ActionTemplate operations
// Used by both NodeService and EdgeService to avoid duplication
type ActionTemplateManager struct {
	actionRepo interfaces.ActionRepositoryInterface
}

// NewActionTemplateManager creates a new ActionTemplate manager
func NewActionTemplateManager(actionRepo interfaces.ActionRepositoryInterface) *ActionTemplateManager {
	return &ActionTemplateManager{
		actionRepo: actionRepo,
	}
}

// CreateActionTemplates creates multiple action templates from requests
func (atm *ActionTemplateManager) CreateActionTemplates(requests []models.ActionTemplateRequest) ([]uint, error) {
	var actionTemplateIDs []uint

	for _, actionReq := range requests {
		actionTemplate, err := atm.createSingleActionTemplate(&actionReq)
		if err != nil {
			// Continue with other actions if one fails
			continue
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	return actionTemplateIDs, nil
}

// CreateSingleActionTemplate creates a single action template
func (atm *ActionTemplateManager) createSingleActionTemplate(actionReq *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	action := &models.ActionTemplate{
		ActionType:        actionReq.ActionType,
		ActionID:          actionReq.ActionID,
		BlockingType:      actionReq.BlockingType,
		ActionDescription: actionReq.ActionDescription,
	}

	return atm.actionRepo.CreateActionTemplate(action, actionReq.Parameters)
}

// DeleteActionTemplates deletes multiple action templates by IDs
func (atm *ActionTemplateManager) DeleteActionTemplates(actionIDs []uint) {
	for _, actionID := range actionIDs {
		atm.actionRepo.DeleteActionTemplate(actionID)
	}
}

// GetActionTemplatesByIDs retrieves multiple action templates by their IDs
func (atm *ActionTemplateManager) GetActionTemplatesByIDs(actionIDs []uint) ([]models.ActionTemplate, error) {
	if len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}

	var actions []models.ActionTemplate
	for _, actionID := range actionIDs {
		action, err := atm.actionRepo.GetActionTemplate(actionID)
		if err == nil {
			actions = append(actions, *action)
		}
	}

	return actions, nil
}

// UpdateActionTemplatesFromRequests updates action templates based on new requests
// Deletes old ones and creates new ones
func (atm *ActionTemplateManager) UpdateActionTemplatesFromRequests(oldActionIDs []uint, newRequests []models.ActionTemplateRequest) ([]uint, error) {
	// Delete old action templates
	if len(oldActionIDs) > 0 {
		atm.DeleteActionTemplates(oldActionIDs)
	}

	// Create new action templates
	return atm.CreateActionTemplates(newRequests)
}

// ConvertToActionArray converts action templates to Action array for MQTT messages
func (atm *ActionTemplateManager) ConvertToActionArray(actionTemplates []models.ActionTemplate) []models.Action {
	actions := make([]models.Action, len(actionTemplates))

	for i, template := range actionTemplates {
		actions[i] = template.ToAction()
	}

	return actions
}

// ValidateActionTemplateRequests validates action template requests
func (atm *ActionTemplateManager) ValidateActionTemplateRequests(requests []models.ActionTemplateRequest) error {
	// Basic validation - can be extended if needed
	for _, req := range requests {
		if req.ActionType == "" {
			continue // Skip invalid requests
		}
	}
	return nil
}
