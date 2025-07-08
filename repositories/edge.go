package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// EdgeRepository implements EdgeRepositoryInterface
type EdgeRepository struct {
	db *gorm.DB
}

// NewEdgeRepository creates a new instance of EdgeRepository
func NewEdgeRepository(db *gorm.DB) interfaces.EdgeRepositoryInterface {
	return &EdgeRepository{
		db: db,
	}
}

// CreateEdge creates a new edge template
func (er *EdgeRepository) CreateEdge(edge *models.EdgeTemplate) (*models.EdgeTemplate, error) {
	if err := er.db.Create(edge).Error; err != nil {
		return nil, fmt.Errorf("failed to create edge template: %w", err)
	}
	return FindByField[models.EdgeTemplate](er.db, "id", edge.ID)
}

// GetEdge retrieves an edge template by database ID
func (er *EdgeRepository) GetEdge(edgeID uint) (*models.EdgeTemplate, error) {
	// Use the generic helper function
	return FindByField[models.EdgeTemplate](er.db, "id", edgeID)
}

// GetEdgeByEdgeID retrieves an edge template by edge ID
func (er *EdgeRepository) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	// Use the generic helper function
	return FindByField[models.EdgeTemplate](er.db, "edge_id", edgeID)
}

// GetEdgeWithActions retrieves an edge template with its associated action templates
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

// ListEdges retrieves all edge templates with pagination
func (er *EdgeRepository) ListEdges(limit, offset int) ([]models.EdgeTemplate, error) {
	var edges []models.EdgeTemplate
	query := er.db.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&edges).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list edge templates: %w", err)
	}

	return edges, nil
}

// UpdateEdge updates an existing edge template
func (er *EdgeRepository) UpdateEdge(edgeID uint, edge *models.EdgeTemplate) (*models.EdgeTemplate, error) {
	if _, err := er.GetEdge(edgeID); err != nil {
		return nil, fmt.Errorf("edge template not found: %w", err)
	}

	if edge.EdgeID != "" {
		exists, err := er.CheckEdgeExistsExcluding(edge.EdgeID, edgeID)
		if err != nil {
			return nil, fmt.Errorf("failed to check edge ID conflict: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("edge with ID '%s' already exists", edge.EdgeID)
		}
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

	if err := er.db.Model(&models.EdgeTemplate{}).Where("id = ?", edgeID).Updates(updateFields).Error; err != nil {
		return nil, fmt.Errorf("failed to update edge template: %w", err)
	}

	return er.GetEdge(edgeID)
}

// DeleteEdge deletes an edge template and cleans up associations
func (er *EdgeRepository) DeleteEdge(edgeID uint) error {
	return er.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("edge_template_id = ?", edgeID).Delete(&models.OrderTemplateEdge{}).Error; err != nil {
			return fmt.Errorf("failed to delete order template associations: %w", err)
		}

		edge, err := FindByField[models.EdgeTemplate](tx, "id", edgeID)
		if err != nil {
			return fmt.Errorf("failed to get edge for deletion: %w", err)
		}

		if edge.ActionTemplateIDs != "" {
			actionIDs, err := edge.GetActionTemplateIDs()
			if err == nil && len(actionIDs) > 0 {
				if err := tx.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
					return fmt.Errorf("failed to delete action parameters: %w", err)
				}
				if err := tx.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{}).Error; err != nil {
					return fmt.Errorf("failed to delete action templates: %w", err)
				}
			}
		}

		if err := tx.Delete(&models.EdgeTemplate{}, edgeID).Error; err != nil {
			return fmt.Errorf("failed to delete edge template: %w", err)
		}

		return nil
	})
}

// CheckEdgeExists checks if an edge with the given edgeID already exists
func (er *EdgeRepository) CheckEdgeExists(edgeID string) (bool, error) {
	// Use the generic helper function
	return ExistsByField[models.EdgeTemplate](er.db, "edge_id", edgeID)
}

// CheckEdgeExistsExcluding checks if an edge exists excluding a specific database ID
func (er *EdgeRepository) CheckEdgeExistsExcluding(edgeID string, excludeID uint) (bool, error) {
	// Use the generic helper function
	return ExistsByFieldExcluding[models.EdgeTemplate](er.db, "edge_id", edgeID, excludeID)
}

// GetActionTemplatesByEdgeID retrieves action templates associated with an edge
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
