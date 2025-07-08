package repositories

import (
	"fmt"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// EdgeRepository implements EdgeRepositoryInterface.
type EdgeRepository struct {
	db *gorm.DB
}

// NewEdgeRepository creates a new instance of EdgeRepository.
func NewEdgeRepository(db *gorm.DB) interfaces.EdgeRepositoryInterface {
	return &EdgeRepository{
		db: db,
	}
}

// CreateEdge creates a new edge template within a given transaction.
func (er *EdgeRepository) CreateEdge(tx *gorm.DB, edge *models.EdgeTemplate) (*models.EdgeTemplate, error) {
	if err := tx.Create(edge).Error; err != nil {
		return nil, fmt.Errorf("failed to create edge template: %w", err)
	}
	// Retrieve the created edge to return the full object.
	var createdEdge models.EdgeTemplate
	if err := tx.First(&createdEdge, edge.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve created edge: %w", err)
	}
	return &createdEdge, nil
}

// GetEdge retrieves an edge template by database ID.
func (er *EdgeRepository) GetEdge(edgeID uint) (*models.EdgeTemplate, error) {
	return FindByField[models.EdgeTemplate](er.db, "id", edgeID)
}

// GetEdgeByEdgeID retrieves an edge template by edge ID.
func (er *EdgeRepository) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	return FindByField[models.EdgeTemplate](er.db, "edge_id", edgeID)
}

// GetEdgeWithActions retrieves an edge template with its associated action templates.
func (er *EdgeRepository) GetEdgeWithActions(edgeID uint) (*models.EdgeTemplate, []models.ActionTemplate, error) {
	edge, err := er.GetEdge(edgeID)
	if err != nil {
		return nil, nil, err
	}
	actions, err := er.GetActionTemplatesByEdgeID(edgeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get action templates for edge: %w", err)
	}
	return edge, actions, nil
}

// ListEdges retrieves all edge templates with pagination.
func (er *EdgeRepository) ListEdges(limit, offset int) ([]models.EdgeTemplate, error) {
	var edges []models.EdgeTemplate
	query := er.db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&edges).Error; err != nil {
		return nil, fmt.Errorf("failed to list edge templates: %w", err)
	}
	return edges, nil
}

// UpdateEdge updates an existing edge template within a transaction.
func (er *EdgeRepository) UpdateEdge(tx *gorm.DB, edgeID uint, edge *models.EdgeTemplate) (*models.EdgeTemplate, error) {
	// First, verify the record exists.
	if err := tx.First(&models.EdgeTemplate{}, edgeID).Error; err != nil {
		return nil, fmt.Errorf("edge template with ID %d not found: %w", edgeID, err)
	}

	updateFields := map[string]interface{}{
		"edge_id":             edge.EdgeID,
		"name":                edge.Name,
		"description":         edge.Description,
		"sequence_id":         edge.SequenceID,
		"released":            edge.Released,
		"start_node_id":       edge.StartNodeID,
		"end_node_id":         edge.EndNodeID,
		"action_template_ids": edge.ActionTemplateIDs,
	}

	if err := tx.Model(&models.EdgeTemplate{}).Where("id = ?", edgeID).Updates(updateFields).Error; err != nil {
		return nil, fmt.Errorf("failed to update edge template: %w", err)
	}

	var updatedEdge models.EdgeTemplate
	if err := tx.First(&updatedEdge, edgeID).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve updated edge: %w", err)
	}
	return &updatedEdge, nil
}

// DeleteEdge deletes an edge template and its associations within a transaction.
func (er *EdgeRepository) DeleteEdge(tx *gorm.DB, edgeID uint) error {
	// The transaction is managed by the service layer.
	// We just perform the required deletions using the provided 'tx'.
	edge, err := FindByField[models.EdgeTemplate](er.db, "id", edgeID) // Use er.db for pre-check read
	if err != nil {
		return fmt.Errorf("failed to get edge for deletion: %w", err)
	}

	if err := tx.Where("edge_template_id = ?", edgeID).Delete(&models.OrderTemplateEdge{}).Error; err != nil {
		return fmt.Errorf("failed to delete order template associations for edge: %w", err)
	}

	if edge.ActionTemplateIDs != "" {
		actionIDs, err := edge.GetActionTemplateIDs()
		if err == nil && len(actionIDs) > 0 {
			if err := tx.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
				return fmt.Errorf("failed to delete action parameters for edge: %w", err)
			}
			if err := tx.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{}).Error; err != nil {
				return fmt.Errorf("failed to delete action templates for edge: %w", err)
			}
		}
	}

	if err := tx.Delete(&models.EdgeTemplate{}, edgeID).Error; err != nil {
		return fmt.Errorf("failed to delete edge template: %w", err)
	}
	return nil
}

// CheckEdgeExists checks if an edge with the given edgeID already exists.
func (er *EdgeRepository) CheckEdgeExists(edgeID string) (bool, error) {
	return ExistsByField[models.EdgeTemplate](er.db, "edge_id", edgeID)
}

// CheckEdgeExistsExcluding checks if an edge exists excluding a specific database ID.
func (er *EdgeRepository) CheckEdgeExistsExcluding(edgeID string, excludeID uint) (bool, error) {
	return ExistsByFieldExcluding[models.EdgeTemplate](er.db, "edge_id", edgeID, excludeID)
}

// GetActionTemplatesByEdgeID retrieves action templates associated with an edge.
func (er *EdgeRepository) GetActionTemplatesByEdgeID(edgeID uint) ([]models.ActionTemplate, error) {
	edge, err := er.GetEdge(edgeID)
	if err != nil {
		return nil, err
	}
	actionIDs, err := edge.GetActionTemplateIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse action template IDs: %w", err)
	}
	if len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}
	var actions []models.ActionTemplate
	err = er.db.Where("id IN ?", actionIDs).Preload("Parameters").Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get action templates: %w", err)
	}
	return actions, nil
}
