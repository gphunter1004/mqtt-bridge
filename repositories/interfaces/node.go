package interfaces

import (
	"mqtt-bridge/models"

	"gorm.io/gorm" // gorm 패키지 임포트
)

// NodeRepositoryInterface defines the contract for node template data access.
type NodeRepositoryInterface interface {
	// CreateNode creates a new node template within a transaction.
	CreateNode(tx *gorm.DB, node *models.NodeTemplate) (*models.NodeTemplate, error)

	// GetNode retrieves a node template by database ID.
	GetNode(nodeID uint) (*models.NodeTemplate, error)

	// GetNodeByNodeID retrieves a node template by node ID.
	GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error)

	// GetNodeWithActions retrieves a node template with its associated action templates.
	GetNodeWithActions(nodeID uint) (*models.NodeTemplate, []models.ActionTemplate, error)

	// ListNodes retrieves all node templates with pagination.
	ListNodes(limit, offset int) ([]models.NodeTemplate, error)

	// UpdateNode updates an existing node template within a transaction.
	UpdateNode(tx *gorm.DB, nodeID uint, node *models.NodeTemplate) (*models.NodeTemplate, error)

	// DeleteNode deletes a node template and its associations within a transaction.
	DeleteNode(tx *gorm.DB, nodeID uint) error

	// CheckNodeExists checks if a node with the given nodeID already exists.
	CheckNodeExists(nodeID string) (bool, error)

	// CheckNodeExistsExcluding checks if a node exists excluding a specific database ID.
	CheckNodeExistsExcluding(nodeID string, excludeID uint) (bool, error)

	// GetActionTemplatesByNodeID retrieves action templates associated with a node.
	GetActionTemplatesByNodeID(nodeID uint) ([]models.ActionTemplate, error)
}
