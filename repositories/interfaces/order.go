package interfaces

import (
	"mqtt-bridge/models"
)

// OrderTemplateRepositoryInterface defines the contract for order template data access
type OrderTemplateRepositoryInterface interface {
	// CreateOrderTemplate creates a new order template
	CreateOrderTemplate(template *models.OrderTemplate) (*models.OrderTemplate, error)

	// GetOrderTemplate retrieves an order template by ID
	GetOrderTemplate(id uint) (*models.OrderTemplate, error)

	// GetOrderTemplateWithDetails retrieves an order template with associated nodes and edges
	GetOrderTemplateWithDetails(id uint) (*models.OrderTemplate, []models.NodeTemplate, []models.EdgeTemplate, error)

	// GetOrderTemplateWithFullDetails retrieves order template with nodes/edges and their actions
	GetOrderTemplateWithFullDetails(id uint) (*models.OrderTemplate, []models.NodeTemplate, []models.ActionTemplate, []models.EdgeTemplate, []models.ActionTemplate, error)

	// ListOrderTemplates retrieves all order templates with pagination
	ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error)

	// UpdateOrderTemplate updates an existing order template
	UpdateOrderTemplate(id uint, template *models.OrderTemplate) (*models.OrderTemplate, error)

	// DeleteOrderTemplate deletes an order template and its associations
	DeleteOrderTemplate(id uint) error

	// AssociateNodes associates existing nodes with an order template
	AssociateNodes(templateID uint, nodeIDs []string) error

	// AssociateEdges associates existing edges with an order template
	AssociateEdges(templateID uint, edgeIDs []string) error

	// GetAssociatedNodes retrieves nodes associated with an order template
	GetAssociatedNodes(templateID uint) ([]models.NodeTemplate, error)

	// GetAssociatedEdges retrieves edges associated with an order template
	GetAssociatedEdges(templateID uint) ([]models.EdgeTemplate, error)

	// RemoveNodeAssociations removes all node associations for an order template
	RemoveNodeAssociations(templateID uint) error

	// RemoveEdgeAssociations removes all edge associations for an order template
	RemoveEdgeAssociations(templateID uint) error

	// GetNodeByNodeID retrieves a node template by its nodeID
	GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error)

	// GetEdgeByEdgeID retrieves an edge template by its edgeID
	GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error)
}
