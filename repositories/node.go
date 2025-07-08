package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// NodeRepository implements NodeRepositoryInterface
type NodeRepository struct {
	db *gorm.DB
}

// NewNodeRepository creates a new instance of NodeRepository
func NewNodeRepository(db *gorm.DB) interfaces.NodeRepositoryInterface {
	return &NodeRepository{
		db: db,
	}
}

// CreateNode creates a new node template
func (nr *NodeRepository) CreateNode(node *models.NodeTemplate) (*models.NodeTemplate, error) {
	if err := nr.db.Create(node).Error; err != nil {
		return nil, fmt.Errorf("failed to create node template: %w", err)
	}
	// Use the generic helper to get the created node
	return FindByField[models.NodeTemplate](nr.db, "id", node.ID)
}

// GetNode retrieves a node template by database ID
func (nr *NodeRepository) GetNode(nodeID uint) (*models.NodeTemplate, error) {
	// Use the generic helper function
	return FindByField[models.NodeTemplate](nr.db, "id", nodeID)
}

// GetNodeByNodeID retrieves a node template by node ID
func (nr *NodeRepository) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	// Use the generic helper function
	return FindByField[models.NodeTemplate](nr.db, "node_id", nodeID)
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
	var nodes []models.NodeTemplate
	query := nr.db.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&nodes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list node templates: %w", err)
	}
	return nodes, nil
}

// UpdateNode updates an existing node template
func (nr *NodeRepository) UpdateNode(nodeID uint, node *models.NodeTemplate) (*models.NodeTemplate, error) {
	if _, err := nr.GetNode(nodeID); err != nil {
		return nil, fmt.Errorf("node template not found: %w", err)
	}

	if node.NodeID != "" {
		exists, err := nr.CheckNodeExistsExcluding(node.NodeID, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to check node ID conflict: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("node with ID '%s' already exists", node.NodeID)
		}
	}

	// Using a map for updates is safer and more flexible
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

	if err := nr.db.Model(&models.NodeTemplate{}).Where("id = ?", nodeID).Updates(updateFields).Error; err != nil {
		return nil, fmt.Errorf("failed to update node template: %w", err)
	}

	return nr.GetNode(nodeID)
}

// DeleteNode deletes a node template and cleans up associations
func (nr *NodeRepository) DeleteNode(nodeID uint) error {
	return nr.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("node_template_id = ?", nodeID).Delete(&models.OrderTemplateNode{}).Error; err != nil {
			return fmt.Errorf("failed to delete order template associations: %w", err)
		}

		node, err := FindByField[models.NodeTemplate](tx, "id", nodeID)
		if err != nil {
			return fmt.Errorf("failed to get node for deletion: %w", err)
		}

		if node.ActionTemplateIDs != "" {
			actionIDs, err := node.GetActionTemplateIDs()
			if err == nil && len(actionIDs) > 0 {
				if err := tx.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
					return fmt.Errorf("failed to delete action parameters: %w", err)
				}
				if err := tx.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{}).Error; err != nil {
					return fmt.Errorf("failed to delete action templates: %w", err)
				}
			}
		}

		if err := tx.Delete(&models.NodeTemplate{}, nodeID).Error; err != nil {
			return fmt.Errorf("failed to delete node template: %w", err)
		}

		return nil
	})
}

// CheckNodeExists checks if a node with the given nodeID already exists
func (nr *NodeRepository) CheckNodeExists(nodeID string) (bool, error) {
	// Use the generic helper function
	return ExistsByField[models.NodeTemplate](nr.db, "node_id", nodeID)
}

// CheckNodeExistsExcluding checks if a node exists excluding a specific database ID
func (nr *NodeRepository) CheckNodeExistsExcluding(nodeID string, excludeID uint) (bool, error) {
	// Use the generic helper function
	return ExistsByFieldExcluding[models.NodeTemplate](nr.db, "node_id", nodeID, excludeID)
}

// GetActionTemplatesByNodeID retrieves action templates associated with a node
func (nr *NodeRepository) GetActionTemplatesByNodeID(nodeID uint) ([]models.ActionTemplate, error) {
	node, err := nr.GetNode(nodeID)
	if err != nil {
		return nil, err
	}

	actionIDs, err := node.GetActionTemplateIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse action template IDs: %w", err)
	}

	if len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}

	var actions []models.ActionTemplate
	err = nr.db.Where("id IN ?", actionIDs).Preload("Parameters").Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get action templates: %w", err)
	}

	return actions, nil
}
