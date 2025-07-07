package repositories

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// OrderTemplateRepository implements OrderTemplateRepositoryInterface
type OrderTemplateRepository struct {
	db *gorm.DB
}

// NewOrderTemplateRepository creates a new instance of OrderTemplateRepository
func NewOrderTemplateRepository(db *gorm.DB) interfaces.OrderTemplateRepositoryInterface {
	return &OrderTemplateRepository{
		db: db,
	}
}

// CreateOrderTemplate creates a new order template
func (otr *OrderTemplateRepository) CreateOrderTemplate(template *models.OrderTemplate) (*models.OrderTemplate, error) {
	if err := otr.db.Create(template).Error; err != nil {
		return nil, fmt.Errorf("failed to create order template: %w", err)
	}
	return otr.GetOrderTemplate(template.ID)
}

// GetOrderTemplate retrieves an order template by ID
func (otr *OrderTemplateRepository) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	var template models.OrderTemplate
	err := otr.db.Where("id = ?", id).First(&template).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("order template with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}
	return &template, nil
}

// GetOrderTemplateWithDetails retrieves an order template with associated nodes and edges
func (otr *OrderTemplateRepository) GetOrderTemplateWithDetails(id uint) (*models.OrderTemplate, []models.NodeTemplate, []models.EdgeTemplate, error) {
	// Get the order template
	template, err := otr.GetOrderTemplate(id)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get associated nodes
	nodes, err := otr.GetAssociatedNodes(id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get associated nodes: %w", err)
	}

	// Get associated edges
	edges, err := otr.GetAssociatedEdges(id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get associated edges: %w", err)
	}

	return template, nodes, edges, nil
}

// GetOrderTemplateWithFullDetails retrieves order template with nodes/edges and their actions
func (otr *OrderTemplateRepository) GetOrderTemplateWithFullDetails(id uint) (*models.OrderTemplate, []models.NodeTemplate, []models.ActionTemplate, []models.EdgeTemplate, []models.ActionTemplate, error) {
	// Get the order template
	template, err := otr.GetOrderTemplate(id)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// Get associated nodes
	nodes, err := otr.GetAssociatedNodes(id)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to get associated nodes: %w", err)
	}

	// Get node actions
	var nodeActions []models.ActionTemplate
	for _, node := range nodes {
		actions, err := otr.getActionTemplatesForNode(node.ID)
		if err != nil {
			continue // Skip if error getting actions for this node
		}
		nodeActions = append(nodeActions, actions...)
	}

	// Get associated edges
	edges, err := otr.GetAssociatedEdges(id)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to get associated edges: %w", err)
	}

	// Get edge actions
	var edgeActions []models.ActionTemplate
	for _, edge := range edges {
		actions, err := otr.getActionTemplatesForEdge(edge.ID)
		if err != nil {
			continue // Skip if error getting actions for this edge
		}
		edgeActions = append(edgeActions, actions...)
	}

	return template, nodes, nodeActions, edges, edgeActions, nil
}

// ListOrderTemplates retrieves all order templates with pagination
func (otr *OrderTemplateRepository) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	var templates []models.OrderTemplate
	query := otr.db.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&templates).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list order templates: %w", err)
	}

	return templates, nil
}

// UpdateOrderTemplate updates an existing order template
func (otr *OrderTemplateRepository) UpdateOrderTemplate(id uint, template *models.OrderTemplate) (*models.OrderTemplate, error) {
	// Check if template exists
	if _, err := otr.GetOrderTemplate(id); err != nil {
		return nil, fmt.Errorf("order template not found: %w", err)
	}

	// Update the template
	updateFields := map[string]interface{}{
		"name":        template.Name,
		"description": template.Description,
	}

	if err := otr.db.Model(&models.OrderTemplate{}).Where("id = ?", id).Updates(updateFields).Error; err != nil {
		return nil, fmt.Errorf("failed to update order template: %w", err)
	}

	return otr.GetOrderTemplate(id)
}

// DeleteOrderTemplate deletes an order template and its associations
func (otr *OrderTemplateRepository) DeleteOrderTemplate(id uint) error {
	return otr.db.Transaction(func(tx *gorm.DB) error {
		// Remove node associations
		if err := otr.removeNodeAssociationsWithTx(tx, id); err != nil {
			return fmt.Errorf("failed to remove node associations: %w", err)
		}

		// Remove edge associations
		if err := otr.removeEdgeAssociationsWithTx(tx, id); err != nil {
			return fmt.Errorf("failed to remove edge associations: %w", err)
		}

		// Delete the order template
		if err := tx.Delete(&models.OrderTemplate{}, id).Error; err != nil {
			return fmt.Errorf("failed to delete order template: %w", err)
		}

		return nil
	})
}

// AssociateNodes associates existing nodes with an order template
func (otr *OrderTemplateRepository) AssociateNodes(templateID uint, nodeIDs []string) error {
	return otr.db.Transaction(func(tx *gorm.DB) error {
		for _, nodeID := range nodeIDs {
			if err := otr.associateNodeWithTx(tx, templateID, nodeID); err != nil {
				return fmt.Errorf("failed to associate node '%s': %w", nodeID, err)
			}
		}
		return nil
	})
}

// AssociateEdges associates existing edges with an order template
func (otr *OrderTemplateRepository) AssociateEdges(templateID uint, edgeIDs []string) error {
	return otr.db.Transaction(func(tx *gorm.DB) error {
		for _, edgeID := range edgeIDs {
			if err := otr.associateEdgeWithTx(tx, templateID, edgeID); err != nil {
				return fmt.Errorf("failed to associate edge '%s': %w", edgeID, err)
			}
		}
		return nil
	})
}

// GetAssociatedNodes retrieves nodes associated with an order template
func (otr *OrderTemplateRepository) GetAssociatedNodes(templateID uint) ([]models.NodeTemplate, error) {
	var nodeAssociations []models.OrderTemplateNode
	err := otr.db.Where("order_template_id = ?", templateID).
		Preload("NodeTemplate").
		Find(&nodeAssociations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get associated nodes: %w", err)
	}

	var nodes []models.NodeTemplate
	for _, assoc := range nodeAssociations {
		nodes = append(nodes, assoc.NodeTemplate)
	}

	return nodes, nil
}

// GetAssociatedEdges retrieves edges associated with an order template
func (otr *OrderTemplateRepository) GetAssociatedEdges(templateID uint) ([]models.EdgeTemplate, error) {
	var edgeAssociations []models.OrderTemplateEdge
	err := otr.db.Where("order_template_id = ?", templateID).
		Preload("EdgeTemplate").
		Find(&edgeAssociations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get associated edges: %w", err)
	}

	var edges []models.EdgeTemplate
	for _, assoc := range edgeAssociations {
		edges = append(edges, assoc.EdgeTemplate)
	}

	return edges, nil
}

// RemoveNodeAssociations removes all node associations for an order template
func (otr *OrderTemplateRepository) RemoveNodeAssociations(templateID uint) error {
	return otr.removeNodeAssociationsWithTx(otr.db, templateID)
}

// RemoveEdgeAssociations removes all edge associations for an order template
func (otr *OrderTemplateRepository) RemoveEdgeAssociations(templateID uint) error {
	return otr.removeEdgeAssociationsWithTx(otr.db, templateID)
}

// GetNodeByNodeID retrieves a node template by its nodeID
func (otr *OrderTemplateRepository) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := otr.db.Where("node_id = ?", nodeID).First(&node).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("node with ID '%s' not found", nodeID)
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	return &node, nil
}

// GetEdgeByEdgeID retrieves an edge template by its edgeID
func (otr *OrderTemplateRepository) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	var edge models.EdgeTemplate
	err := otr.db.Where("edge_id = ?", edgeID).First(&edge).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("edge with ID '%s' not found", edgeID)
		}
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}
	return &edge, nil
}

// Private helper methods

func (otr *OrderTemplateRepository) associateNodeWithTx(tx *gorm.DB, templateID uint, nodeID string) error {
	// Get node by nodeID
	node, err := otr.GetNodeByNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("node '%s' not found: %w", nodeID, err)
	}

	// Check if association already exists
	var existing models.OrderTemplateNode
	err = tx.Where("order_template_id = ? AND node_template_id = ?", templateID, node.ID).First(&existing).Error
	if err == nil {
		return nil // Association already exists
	}

	// Create new association
	association := &models.OrderTemplateNode{
		OrderTemplateID: templateID,
		NodeTemplateID:  node.ID,
	}

	return tx.Create(association).Error
}

func (otr *OrderTemplateRepository) associateEdgeWithTx(tx *gorm.DB, templateID uint, edgeID string) error {
	// Get edge by edgeID
	edge, err := otr.GetEdgeByEdgeID(edgeID)
	if err != nil {
		return fmt.Errorf("edge '%s' not found: %w", edgeID, err)
	}

	// Check if association already exists
	var existing models.OrderTemplateEdge
	err = tx.Where("order_template_id = ? AND edge_template_id = ?", templateID, edge.ID).First(&existing).Error
	if err == nil {
		return nil // Association already exists
	}

	// Create new association
	association := &models.OrderTemplateEdge{
		OrderTemplateID: templateID,
		EdgeTemplateID:  edge.ID,
	}

	return tx.Create(association).Error
}

func (otr *OrderTemplateRepository) removeNodeAssociationsWithTx(tx *gorm.DB, templateID uint) error {
	return tx.Where("order_template_id = ?", templateID).Delete(&models.OrderTemplateNode{}).Error
}

func (otr *OrderTemplateRepository) removeEdgeAssociationsWithTx(tx *gorm.DB, templateID uint) error {
	return tx.Where("order_template_id = ?", templateID).Delete(&models.OrderTemplateEdge{}).Error
}

func (otr *OrderTemplateRepository) getActionTemplatesForNode(nodeID uint) ([]models.ActionTemplate, error) {
	var node models.NodeTemplate
	err := otr.db.Where("id = ?", nodeID).First(&node).Error
	if err != nil {
		return nil, err
	}

	actionIDs, err := node.GetActionTemplateIDs()
	if err != nil || len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}

	var actions []models.ActionTemplate
	err = otr.db.Where("id IN ?", actionIDs).
		Preload("Parameters").
		Find(&actions).Error
	return actions, err
}

func (otr *OrderTemplateRepository) getActionTemplatesForEdge(edgeID uint) ([]models.ActionTemplate, error) {
	var edge models.EdgeTemplate
	err := otr.db.Where("id = ?", edgeID).First(&edge).Error
	if err != nil {
		return nil, err
	}

	actionIDs, err := edge.GetActionTemplateIDs()
	if err != nil || len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}

	var actions []models.ActionTemplate
	err = otr.db.Where("id IN ?", actionIDs).
		Preload("Parameters").
		Find(&actions).Error
	return actions, err
}
