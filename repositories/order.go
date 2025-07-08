package repositories

import (
	"fmt"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// OrderTemplateRepository implements OrderTemplateRepositoryInterface.
type OrderTemplateRepository struct {
	db *gorm.DB
}

// NewOrderTemplateRepository creates a new instance of OrderTemplateRepository.
func NewOrderTemplateRepository(db *gorm.DB) interfaces.OrderTemplateRepositoryInterface {
	return &OrderTemplateRepository{
		db: db,
	}
}

// CreateOrderTemplate creates a new order template within a transaction.
func (otr *OrderTemplateRepository) CreateOrderTemplate(tx *gorm.DB, template *models.OrderTemplate) (*models.OrderTemplate, error) {
	if err := tx.Create(template).Error; err != nil {
		return nil, fmt.Errorf("failed to create order template: %w", err)
	}
	var createdTemplate models.OrderTemplate
	if err := tx.First(&createdTemplate, template.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve created order template: %w", err)
	}
	return &createdTemplate, nil
}

// GetOrderTemplate retrieves an order template by ID.
func (otr *OrderTemplateRepository) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	return FindByField[models.OrderTemplate](otr.db, "id", id)
}

// GetOrderTemplateWithDetails retrieves an order template with associated nodes and edges.
func (otr *OrderTemplateRepository) GetOrderTemplateWithDetails(id uint) (*models.OrderTemplate, []models.NodeTemplate, []models.EdgeTemplate, error) {
	template, err := otr.GetOrderTemplate(id)
	if err != nil {
		return nil, nil, nil, err
	}
	nodes, err := otr.GetAssociatedNodes(id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get associated nodes: %w", err)
	}
	edges, err := otr.GetAssociatedEdges(id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get associated edges: %w", err)
	}
	return template, nodes, edges, nil
}

// GetOrderTemplateWithFullDetails retrieves order template with nodes/edges and their actions.
func (otr *OrderTemplateRepository) GetOrderTemplateWithFullDetails(id uint) (*models.OrderTemplate, []models.NodeTemplate, []models.ActionTemplate, []models.EdgeTemplate, []models.ActionTemplate, error) {
	template, err := otr.GetOrderTemplate(id)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	nodes, err := otr.GetAssociatedNodes(id)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to get associated nodes: %w", err)
	}
	var nodeActions []models.ActionTemplate
	for _, node := range nodes {
		actions, _ := otr.getActionTemplatesForNode(node.ID)
		nodeActions = append(nodeActions, actions...)
	}
	edges, err := otr.GetAssociatedEdges(id)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to get associated edges: %w", err)
	}
	var edgeActions []models.ActionTemplate
	for _, edge := range edges {
		actions, _ := otr.getActionTemplatesForEdge(edge.ID)
		edgeActions = append(edgeActions, actions...)
	}
	return template, nodes, nodeActions, edges, edgeActions, nil
}

// ListOrderTemplates retrieves all order templates with pagination.
func (otr *OrderTemplateRepository) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	var templates []models.OrderTemplate
	query := otr.db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("failed to list order templates: %w", err)
	}
	return templates, nil
}

// UpdateOrderTemplate updates an existing order template within a transaction.
func (otr *OrderTemplateRepository) UpdateOrderTemplate(tx *gorm.DB, id uint, template *models.OrderTemplate) (*models.OrderTemplate, error) {
	if err := tx.First(&models.OrderTemplate{}, id).Error; err != nil {
		return nil, fmt.Errorf("order template with ID %d not found: %w", id, err)
	}
	updateFields := map[string]interface{}{
		"name":        template.Name,
		"description": template.Description,
	}
	if err := tx.Model(&models.OrderTemplate{}).Where("id = ?", id).Updates(updateFields).Error; err != nil {
		return nil, fmt.Errorf("failed to update order template: %w", err)
	}
	var updatedTemplate models.OrderTemplate
	if err := tx.First(&updatedTemplate, id).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve updated order template: %w", err)
	}
	return &updatedTemplate, nil
}

// DeleteOrderTemplate deletes an order template and its associations within a transaction.
func (otr *OrderTemplateRepository) DeleteOrderTemplate(tx *gorm.DB, id uint) error {
	if err := otr.RemoveNodeAssociations(tx, id); err != nil {
		return fmt.Errorf("failed to remove node associations: %w", err)
	}
	if err := otr.RemoveEdgeAssociations(tx, id); err != nil {
		return fmt.Errorf("failed to remove edge associations: %w", err)
	}
	if err := tx.Delete(&models.OrderTemplate{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete order template: %w", err)
	}
	return nil
}

// AssociateNodes associates existing nodes with an order template within a transaction.
func (otr *OrderTemplateRepository) AssociateNodes(tx *gorm.DB, templateID uint, nodeIDs []string) error {
	for _, nodeID := range nodeIDs {
		node, err := otr.GetNodeByNodeID(nodeID)
		if err != nil {
			return fmt.Errorf("node '%s' not found: %w", nodeID, err)
		}
		association := &models.OrderTemplateNode{
			OrderTemplateID: templateID,
			NodeTemplateID:  node.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			return fmt.Errorf("failed to associate node '%s': %w", nodeID, err)
		}
	}
	return nil
}

// AssociateEdges associates existing edges with an order template within a transaction.
func (otr *OrderTemplateRepository) AssociateEdges(tx *gorm.DB, templateID uint, edgeIDs []string) error {
	for _, edgeID := range edgeIDs {
		edge, err := otr.GetEdgeByEdgeID(edgeID)
		if err != nil {
			return fmt.Errorf("edge '%s' not found: %w", edgeID, err)
		}
		association := &models.OrderTemplateEdge{
			OrderTemplateID: templateID,
			EdgeTemplateID:  edge.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			return fmt.Errorf("failed to associate edge '%s': %w", edgeID, err)
		}
	}
	return nil
}

// GetAssociatedNodes retrieves nodes associated with an order template.
func (otr *OrderTemplateRepository) GetAssociatedNodes(templateID uint) ([]models.NodeTemplate, error) {
	var nodes []models.NodeTemplate
	if err := otr.db.Table("node_templates").
		Joins("join order_template_nodes on order_template_nodes.node_template_id = node_templates.id").
		Where("order_template_nodes.order_template_id = ?", templateID).
		Find(&nodes).Error; err != nil {
		return nil, fmt.Errorf("failed to get associated nodes: %w", err)
	}
	return nodes, nil
}

// GetAssociatedEdges retrieves edges associated with an order template.
func (otr *OrderTemplateRepository) GetAssociatedEdges(templateID uint) ([]models.EdgeTemplate, error) {
	var edges []models.EdgeTemplate
	if err := otr.db.Table("edge_templates").
		Joins("join order_template_edges on order_template_edges.edge_template_id = edge_templates.id").
		Where("order_template_edges.order_template_id = ?", templateID).
		Find(&edges).Error; err != nil {
		return nil, fmt.Errorf("failed to get associated edges: %w", err)
	}
	return edges, nil
}

// RemoveNodeAssociations removes all node associations for an order template within a transaction.
func (otr *OrderTemplateRepository) RemoveNodeAssociations(tx *gorm.DB, templateID uint) error {
	return tx.Where("order_template_id = ?", templateID).Delete(&models.OrderTemplateNode{}).Error
}

// RemoveEdgeAssociations removes all edge associations for an order template within a transaction.
func (otr *OrderTemplateRepository) RemoveEdgeAssociations(tx *gorm.DB, templateID uint) error {
	return tx.Where("order_template_id = ?", templateID).Delete(&models.OrderTemplateEdge{}).Error
}

// GetNodeByNodeID retrieves a node template by its nodeID.
func (otr *OrderTemplateRepository) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	return FindByField[models.NodeTemplate](otr.db, "node_id", nodeID)
}

// GetEdgeByEdgeID retrieves an edge template by its edgeID.
func (otr *OrderTemplateRepository) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	return FindByField[models.EdgeTemplate](otr.db, "edge_id", edgeID)
}

// Private helper methods
func (otr *OrderTemplateRepository) getActionTemplatesForNode(nodeID uint) ([]models.ActionTemplate, error) {
	node, err := FindByField[models.NodeTemplate](otr.db, "id", nodeID)
	if err != nil {
		return nil, err
	}
	actionIDs, err := node.GetActionTemplateIDs()
	if err != nil || len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}
	var actions []models.ActionTemplate
	err = otr.db.Where("id IN ?", actionIDs).Preload("Parameters").Find(&actions).Error
	return actions, err
}

func (otr *OrderTemplateRepository) getActionTemplatesForEdge(edgeID uint) ([]models.ActionTemplate, error) {
	edge, err := FindByField[models.EdgeTemplate](otr.db, "id", edgeID)
	if err != nil {
		return nil, err
	}
	actionIDs, err := edge.GetActionTemplateIDs()
	if err != nil || len(actionIDs) == 0 {
		return []models.ActionTemplate{}, nil
	}
	var actions []models.ActionTemplate
	err = otr.db.Where("id IN ?", actionIDs).Preload("Parameters").Find(&actions).Error
	return actions, err
}
