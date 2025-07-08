package interfaces

import (
	"mqtt-bridge/models"

	"gorm.io/gorm" // gorm 패키지 임포트
)

// EdgeRepositoryInterface defines the contract for edge template data access.
type EdgeRepositoryInterface interface {
	// CreateEdge creates a new edge template within a transaction.
	CreateEdge(tx *gorm.DB, edge *models.EdgeTemplate) (*models.EdgeTemplate, error)

	// GetEdge retrieves an edge template by database ID.
	GetEdge(edgeID uint) (*models.EdgeTemplate, error)

	// GetEdgeByEdgeID retrieves an edge template by edge ID.
	GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error)

	// GetEdgeWithActions retrieves an edge template with its associated action templates.
	GetEdgeWithActions(edgeID uint) (*models.EdgeTemplate, []models.ActionTemplate, error)

	// ListEdges retrieves all edge templates with pagination.
	ListEdges(limit, offset int) ([]models.EdgeTemplate, error)

	// UpdateEdge updates an existing edge template within a transaction.
	UpdateEdge(tx *gorm.DB, edgeID uint, edge *models.EdgeTemplate) (*models.EdgeTemplate, error)

	// DeleteEdge deletes an edge template and its associations within a transaction.
	DeleteEdge(tx *gorm.DB, edgeID uint) error

	// CheckEdgeExists checks if an edge with the given edgeID already exists.
	CheckEdgeExists(edgeID string) (bool, error)

	// CheckEdgeExistsExcluding checks if an edge exists excluding a specific database ID.
	CheckEdgeExistsExcluding(edgeID string, excludeID uint) (bool, error)

	// GetActionTemplatesByEdgeID retrieves action templates associated with an edge.
	GetActionTemplatesByEdgeID(edgeID uint) ([]models.ActionTemplate, error)
}
