package interfaces

import (
	"mqtt-bridge/models"
)

// NodeRepositoryInterface defines the contract for node template data access
type NodeRepositoryInterface interface {
	// CreateNode creates a new node template
	CreateNode(node *models.NodeTemplate) (*models.NodeTemplate, error)

	// GetNode retrieves a node template by database ID
	GetNode(nodeID uint) (*models.NodeTemplate, error)

	// GetNodeByNodeID retrieves a node template by node ID
	GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error)

	// GetNodeWithActions retrieves a node template with its associated action templates
	GetNodeWithActions(nodeID uint) (*models.NodeTemplate, []models.ActionTemplate, error)

	// ListNodes retrieves all node templates with pagination
	ListNodes(limit, offset int) ([]models.NodeTemplate, error)

	// UpdateNode updates an existing node template
	UpdateNode(nodeID uint, node *models.NodeTemplate) (*models.NodeTemplate, error)

	// DeleteNode deletes a node template and cleans up associations
	DeleteNode(nodeID uint) error

	// CheckNodeExists checks if a node with the given nodeID already exists
	CheckNodeExists(nodeID string) (bool, error)

	// CheckNodeExistsExcluding checks if a node exists excluding a specific database ID
	CheckNodeExistsExcluding(nodeID string, excludeID uint) (bool, error)

	// GetActionTemplatesByNodeID retrieves action templates associated with a node
	GetActionTemplatesByNodeID(nodeID uint) ([]models.ActionTemplate, error)
}
