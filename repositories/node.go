package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/base"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// NodeRepository implements NodeRepositoryInterface using base CRUD
type NodeRepository struct {
	*base.BaseCRUDRepository[models.NodeTemplate]
	db *gorm.DB
}

// NewNodeRepository creates a new instance of NodeRepository
func NewNodeRepository(db *gorm.DB) interfaces.NodeRepositoryInterface {
	baseCRUD := base.NewBaseCRUDRepository[models.NodeTemplate](db, "node_templates")
	return &NodeRepository{
		BaseCRUDRepository: baseCRUD,
		db:                 db,
	}
}

// ===================================================================
// NODE TEMPLATE CRUD OPERATIONS
// ===================================================================

// CreateNode creates a new node template
func (nr *NodeRepository) CreateNode(node *models.NodeTemplate) (*models.NodeTemplate, error) {
	// Use base method for uniqueness check
	if err := nr.CheckUniqueConstraint(&nodeEntity{node.NodeID}); err != nil {
		return nil, err
	}

	return nr.CreateAndGet(node)
}

// GetNode retrieves a node template by database ID
func (nr *NodeRepository) GetNode(nodeID uint) (*models.NodeTemplate, error) {
	return nr.GetByID(nodeID)
}

// GetNodeByNodeID retrieves a node template by node ID
func (nr *NodeRepository) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := nr.db.Where("node_id = ?", nodeID).First(&node).Error
	return &node, base.HandleDBError("get", "node_templates", fmt.Sprintf("node ID '%s'", nodeID), err)
}

// GetNodeWithActions retrieves a node template with its associated action templates
func (nr *NodeRepository) GetNodeWithActions(nodeID uint) (*models.NodeTemplate, []models.ActionTemplate, error) {
	node, err := nr.GetNode(nodeID)
	if err != nil {
		return nil, nil, err
	}

	actions, err := nr.GetActionTemplatesByNodeID(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get action templates for node: %w", err)
	}

	return node, actions, nil
}

// ListNodes retrieves all node templates with pagination
func (nr *NodeRepository) ListNodes(limit, offset int) ([]models.NodeTemplate, error) {
	return nr.ListWithPagination(limit, offset, "created_at DESC")
}

// UpdateNode updates an existing node template
func (nr *NodeRepository) UpdateNode(nodeID uint, node *models.NodeTemplate) (*models.NodeTemplate, error) {
	// Check uniqueness using base method
	if node.NodeID != "" {
		if err := nr.CheckUniqueConstraint(&nodeEntity{node.NodeID}, nodeID); err != nil {
			return nil, err
		}
	}

	// Use base update method
	updateFields := map[string]interface{}{
		"node_id":                 node.NodeID,
		"name":                    node.Name,
		"description":             node.Description,
		"sequence_id":             node.SequenceID,
		"released":                node.Released,
		"x":                       node.X,
		"y":                       node.Y,
		"theta":                   node.Theta,
		"allowed_deviation_xy":    node.AllowedDeviationXY,
		"allowed_deviation_theta": node.AllowedDeviationTheta,
		"map_id":                  node.MapID,
		"action_template_ids":     node.ActionTemplateIDs,
	}

	return nr.UpdateAndGet(nodeID, updateFields)
}

// DeleteNode deletes a node template and cleans up associations
func (nr *NodeRepository) DeleteNode(nodeID uint) error {
	return nr.WithTransaction(func(tx *gorm.DB) error {
		// Remove order template associations
		if err := tx.Where("node_template_id = ?", nodeID).Delete(&models.OrderTemplateNode{}).Error; err != nil {
			return base.WrapDBError("delete order template associations", "order_template_nodes", err)
		}

		// Get node for action template cleanup
		node, err := nr.GetNode(nodeID)
		if err != nil {
			return err
		}

		// Delete associated action templates using utils helper
		if node.ActionTemplateIDs != "" {
			actionIDs, err := utils.ParseJSONToUintSlice(node.ActionTemplateIDs)
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

		// Delete the node template using base method
		return nr.DeleteWithTransaction(tx, nodeID)
	})
}

// CheckNodeExists checks if a node with the given nodeID already exists
func (nr *NodeRepository) CheckNodeExists(nodeID string) (bool, error) {
	count, err := nr.CountByField("node_id", nodeID)
	return count > 0, err
}

// CheckNodeExistsExcluding checks if a node exists excluding a specific database ID
func (nr *NodeRepository) CheckNodeExistsExcluding(nodeID string, excludeID uint) (bool, error) {
	var count int64
	err := nr.db.Model(&models.NodeTemplate{}).
		Where("node_id = ? AND id != ?", nodeID, excludeID).
		Count(&count).Error
	return count > 0, base.WrapDBError("check node existence", "node_templates", err)
}

// GetActionTemplatesByNodeID retrieves action templates associated with a node
func (nr *NodeRepository) GetActionTemplatesByNodeID(nodeID uint) ([]models.ActionTemplate, error) {
	node, err := nr.GetNode(nodeID)
	if err != nil {
		return nil, err
	}

	// Use utils helper for JSON parsing
	actionIDs, err := utils.ParseJSONToUintSlice(node.ActionTemplateIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse action template IDs: %w", err)
	}

	if len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}

	var actions []models.ActionTemplate
	err = nr.db.Where("id IN ?", actionIDs).
		Preload("Parameters").
		Find(&actions).Error

	return actions, base.WrapDBError("get action templates", "action_templates", err)
}

// ===================================================================
// HELPER ENTITY FOR BASE INTERFACE COMPLIANCE
// ===================================================================

// nodeEntity implements NamedEntity interface for uniqueness checking
type nodeEntity struct {
	nodeID string
}

func (n *nodeEntity) GetIdentifier() string {
	return n.nodeID
}

func (n *nodeEntity) GetIdentifierField() string {
	return "node_id"
}
