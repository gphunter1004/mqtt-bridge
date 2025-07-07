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
	return nr.GetNode(node.ID)
}

// GetNode retrieves a node template by database ID
func (nr *NodeRepository) GetNode(nodeID uint) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := nr.db.Where("id = ?", nodeID).First(&node).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("node template with ID %d not found", nodeID)
		}
		return nil, fmt.Errorf("failed to get node template: %w", err)
	}
	return &node, nil
}

// GetNodeByNodeID retrieves a node template by node ID
func (nr *NodeRepository) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := nr.db.Where("node_id = ?", nodeID).First(&node).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("node template with node ID '%s' not found", nodeID)
		}
		return nil, fmt.Errorf("failed to get node template: %w", err)
	}
	return &node, nil
}

// GetNodeWithActions retrieves a node template with its associated action templates
func (nr *NodeRepository) GetNodeWithActions(nodeID uint) (*models.NodeTemplate, []models.ActionTemplate, error) {
	// Get the node template
	node, err := nr.GetNode(nodeID)
	if err != nil {
		return nil, nil, err
	}

	// Get associated action templates
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
	// Check if node exists
	if _, err := nr.GetNode(nodeID); err != nil {
		return nil, fmt.Errorf("node template not found: %w", err)
	}

	// Check for nodeID conflicts (if nodeID is changing)
	if node.NodeID != "" {
		exists, err := nr.CheckNodeExistsExcluding(node.NodeID, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to check node ID conflict: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("node with ID '%s' already exists", node.NodeID)
		}
	}

	// Update the node template
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
		// Remove order template associations first
		if err := tx.Where("node_template_id = ?", nodeID).Delete(&models.OrderTemplateNode{}).Error; err != nil {
			return fmt.Errorf("failed to delete order template associations: %w", err)
		}

		// Get the node to access action template IDs
		node, err := nr.GetNode(nodeID)
		if err != nil {
			return fmt.Errorf("failed to get node for deletion: %w", err)
		}

		// Delete associated action templates if any
		if node.ActionTemplateIDs != "" {
			actionIDs, err := node.GetActionTemplateIDs()
			if err == nil && len(actionIDs) > 0 {
				// Delete action parameters first
				if err := tx.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
					return fmt.Errorf("failed to delete action parameters: %w", err)
				}
				// Delete action templates
				if err := tx.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{}).Error; err != nil {
					return fmt.Errorf("failed to delete action templates: %w", err)
				}
			}
		}

		// Delete the node template
		if err := tx.Delete(&models.NodeTemplate{}, nodeID).Error; err != nil {
			return fmt.Errorf("failed to delete node template: %w", err)
		}

		return nil
	})
}

// CheckNodeExists checks if a node with the given nodeID already exists
func (nr *NodeRepository) CheckNodeExists(nodeID string) (bool, error) {
	var count int64
	err := nr.db.Model(&models.NodeTemplate{}).Where("node_id = ?", nodeID).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check node existence: %w", err)
	}
	return count > 0, nil
}

// CheckNodeExistsExcluding checks if a node exists excluding a specific database ID
func (nr *NodeRepository) CheckNodeExistsExcluding(nodeID string, excludeID uint) (bool, error) {
	var count int64
	err := nr.db.Model(&models.NodeTemplate{}).
		Where("node_id = ? AND id != ?", nodeID, excludeID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check node existence excluding ID: %w", err)
	}
	return count > 0, nil
}

// GetActionTemplatesByNodeID retrieves action templates associated with a node
func (nr *NodeRepository) GetActionTemplatesByNodeID(nodeID uint) ([]models.ActionTemplate, error) {
	// Get the node to access action template IDs
	node, err := nr.GetNode(nodeID)
	if err != nil {
		return nil, err
	}

	// Parse action template IDs from JSON
	actionIDs, err := node.GetActionTemplateIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse action template IDs: %w", err)
	}

	if len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}

	// Get action templates
	var actions []models.ActionTemplate
	err = nr.db.Where("id IN ?", actionIDs).
		Preload("Parameters").
		Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get action templates: %w", err)
	}

	return actions, nil
}
