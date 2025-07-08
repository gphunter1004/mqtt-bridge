package interfaces

import (
	"mqtt-bridge/models"

	"gorm.io/gorm"
)

// ActionRepositoryInterface defines the contract for action template data access.
type ActionRepositoryInterface interface {
	// CreateActionTemplate creates a new action template with parameters within a transaction.
	CreateActionTemplate(tx *gorm.DB, action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error)

	// GetActionTemplate retrieves an action template by database ID with parameters.
	GetActionTemplate(actionID uint) (*models.ActionTemplate, error)

	// GetActionTemplateByActionID retrieves an action template by action ID with parameters.
	GetActionTemplateByActionID(actionID string) (*models.ActionTemplate, error)

	// ListActionTemplates retrieves all action templates with pagination.
	ListActionTemplates(limit, offset int) ([]models.ActionTemplate, error)

	// ListActionTemplatesByType retrieves action templates filtered by action type.
	ListActionTemplatesByType(actionType string, limit, offset int) ([]models.ActionTemplate, error)

	// SearchActionTemplates searches action templates by term in type or description.
	SearchActionTemplates(searchTerm string, limit, offset int) ([]models.ActionTemplate, error)

	// GetActionTemplatesByBlockingType retrieves action templates filtered by blocking type.
	GetActionTemplatesByBlockingType(blockingType string, limit, offset int) ([]models.ActionTemplate, error)

	// UpdateActionTemplate updates an existing action template within a transaction.
	UpdateActionTemplate(tx *gorm.DB, actionID uint, action *models.ActionTemplate, parameters []models.ActionParameterTemplateRequest) (*models.ActionTemplate, error)

	// DeleteActionTemplate deletes one or more action templates and their parameters within a transaction.
	DeleteActionTemplate(tx *gorm.DB, actionIDs ...uint) error
}
