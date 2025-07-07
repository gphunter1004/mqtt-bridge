package interfaces

import (
	"mqtt-bridge/models"
)

// ActionRepositoryInterface defines the contract for action template data access
type ActionRepositoryInterface interface {
	// CreateActionTemplate creates a new action template with parameters
	CreateActionTemplate(action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error)

	// GetActionTemplate retrieves an action template by database ID with parameters
	GetActionTemplate(actionID uint) (*models.ActionTemplate, error)

	// GetActionTemplateByActionID retrieves an action template by action ID with parameters
	GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error)

	// ListActionTemplates retrieves all action templates with pagination
	ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error)

	// ListActionTemplatesByType retrieves action templates filtered by action type
	ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error)

	// SearchActionTemplates searches action templates by term in type or description
	SearchActionTemplates(searchTerm string, limit, offset int) ([]models.ActionTemplate, error)

	// GetActionTemplatesByBlockingType retrieves action templates filtered by blocking type
	GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error)

	// UpdateActionTemplate updates an existing action template
	UpdateActionTemplate(actionID uint, action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error)

	// DeleteActionTemplate deletes an action template and its parameters
	DeleteActionTemplate(actionID uint) error

	// CreateActionParameter creates a new action parameter for an action template
	CreateActionParameter(actionTemplateID uint, parameter *models.ActionParameterTemplate) error

	// DeleteActionParameters deletes all parameters for an action template
	DeleteActionParameters(actionTemplateID uint) error
}
