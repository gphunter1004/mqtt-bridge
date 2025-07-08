package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/base"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// EdgeRepository implements EdgeRepositoryInterface using base CRUD
type EdgeRepository struct {
	*base.BaseCRUDRepository[models.EdgeTemplate]
	db *gorm.DB
}

// NewEdgeRepository creates a new instance of EdgeRepository
func NewEdgeRepository(db *gorm.DB) interfaces.EdgeRepositoryInterface {
	baseCRUD := base.NewBaseCRUDRepository[models.EdgeTemplate](db, "edge_templates")
	return &EdgeRepository{
		BaseCRUDRepository: baseCRUD,
		db:                 db,
	}
}

// ===================================================================
// EDGE TEMPLATE CRUD OPERATIONS
// ===================================================================

// CreateEdge creates a new edge template
func (er *EdgeRepository) CreateEdge(edge *models.EdgeTemplate) (*models.EdgeTemplate, error) {
	// Use base method for uniqueness check
	if err := er.CheckUniqueConstraint(&edgeEntity{edge.EdgeID}, 0); err != nil {
		return nil, err
	}

	return er.CreateAndGet(edge)
}

// GetEdge retrieves an edge template by database ID
func (er *EdgeRepository) GetEdge(edgeID uint) (*models.EdgeTemplate, error) {
	return er.GetByID(edgeID)
}

// GetEdgeByEdgeID retrieves an edge template by edge ID
func (er *EdgeRepository) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	var edge models.EdgeTemplate
	err := er.db.Where("edge_id = ?", edgeID).First(&edge).Error
	return &edge, base.HandleDBError("get", "edge_templates", fmt.Sprintf("edge ID '%s'", edgeID), err)
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
	return er.ListWithPagination(limit, offset, "created_at DESC")
}

// UpdateEdge updates an existing edge template
func (er *EdgeRepository) UpdateEdge(edgeID uint, edge *models.EdgeTemplate) (*models.EdgeTemplate, error) {
	// Check uniqueness using base method
	if edge.EdgeID != "" {
		if err := er.CheckUniqueConstraint(&edgeEntity{edge.EdgeID}, edgeID); err != nil {
			return nil, err
		}
	}

	// Use base update method
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

	return er.UpdateAndGet(edgeID, updateFields)
}

// DeleteEdge deletes an edge template and cleans up associations
func (er *EdgeRepository) DeleteEdge(edgeID uint) error {
	return er.WithTransaction(func(tx *gorm.DB) error {
		// Remove order template associations
		if err := tx.Where("edge_template_id = ?", edgeID).Delete(&models.OrderTemplateEdge{}).Error; err != nil {
			return base.WrapDBError("delete order template associations", "order_template_edges", err)
		}

		// Get edge for action template cleanup
		edge, err := er.GetEdge(edgeID)
		if err != nil {
			return err
		}

		// Delete associated action templates using utils helper
		if edge.ActionTemplateIDs != "" {
			actionIDs, err := utils.ParseJSONToUintSlice(edge.ActionTemplateIDs)
			if err == nil && len(actionIDs) > 0 {
				// Delete action parameters first
				if err := tx.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
					return base.WrapDBError("delete action parameters", "action_parameter_templates", err)
				}
				// Delete action templates
				if err := tx.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{}).Error; err != nil {
					return base.WrapDBError("delete action templates", "action_templates", err)
				}
			}
		}

		// Delete the edge template using base method
		return er.DeleteWithTransaction(tx, edgeID)
	})
}

// CheckEdgeExists checks if an edge with the given edgeID already exists
func (er *EdgeRepository) CheckEdgeExists(edgeID string) (bool, error) {
	count, err := er.CountByField("edge_id", edgeID)
	return count > 0, err
}

// CheckEdgeExistsExcluding checks if an edge exists excluding a specific database ID
func (er *EdgeRepository) CheckEdgeExistsExcluding(edgeID string, excludeID uint) (bool, error) {
	var count int64
	err := er.db.Model(&models.EdgeTemplate{}).
		Where("edge_id = ? AND id != ?", edgeID, excludeID).
		Count(&count).Error
	return count > 0, base.WrapDBError("check edge existence", "edge_templates", err)
}

// GetActionTemplatesByEdgeID retrieves action templates associated with an edge
func (er *EdgeRepository) GetActionTemplatesByEdgeID(edgeID uint) ([]models.ActionTemplate, error) {
	edge, err := er.GetEdge(edgeID)
	if err != nil {
		return nil, err
	}

	// Use utils helper for JSON parsing
	actionIDs, err := utils.ParseJSONToUintSlice(edge.ActionTemplateIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse action template IDs: %w", err)
	}

	if len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}

	var actions []models.ActionTemplate
	err = er.db.Where("id IN ?", actionIDs).
		Preload("Parameters").
		Find(&actions).Error

	return actions, base.WrapDBError("get action templates", "action_templates", err)
}

// ===================================================================
// HELPER ENTITY FOR BASE INTERFACE COMPLIANCE
// ===================================================================

// edgeEntity implements NamedEntity interface for uniqueness checking
type edgeEntity struct {
	edgeID string
}

func (e *edgeEntity) GetIdentifier() string {
	return e.edgeID
}

func (e *edgeEntity) GetIdentifierField() string {
	return "edge_id"
}
