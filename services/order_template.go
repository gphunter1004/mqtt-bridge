package services

import (
	"fmt"
	"log"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
)

type OrderTemplateService struct {
	db            *database.Database
	actionService *ActionService
}

func NewOrderTemplateService(db *database.Database) *OrderTemplateService {
	return &OrderTemplateService{
		db:            db,
		actionService: NewActionService(db),
	}
}

// Order Template Management

func (ots *OrderTemplateService) CreateOrderTemplate(req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	// Start transaction
	tx := ots.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create order template
	if err := tx.Create(template).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create order template: %w", err)
	}

	// Associate existing nodes with template
	for _, nodeID := range req.NodeIDs {
		var node models.NodeTemplate
		if err := tx.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("node '%s' not found: %w", nodeID, err)
		}

		association := &models.OrderTemplateNode{
			OrderTemplateID: template.ID,
			NodeTemplateID:  node.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to associate node '%s': %w", nodeID, err)
		}
	}

	// Associate existing edges with template
	for _, edgeID := range req.EdgeIDs {
		var edge models.EdgeTemplate
		if err := tx.Where("edge_id = ?", edgeID).First(&edge).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("edge '%s' not found: %w", edgeID, err)
		}

		association := &models.OrderTemplateEdge{
			OrderTemplateID: template.ID,
			EdgeTemplateID:  edge.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to associate edge '%s': %w", edgeID, err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Order Template Service] Order template created successfully: %s (ID: %d)", template.Name, template.ID)

	// Fetch the complete template with associations
	return ots.GetOrderTemplate(template.ID)
}

func (ots *OrderTemplateService) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	var template models.OrderTemplate
	err := ots.db.DB.Where("id = ?", id).First(&template).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	return &template, nil
}

func (ots *OrderTemplateService) GetOrderTemplateWithDetails(id uint) (*OrderTemplateWithDetails, error) {
	template, err := ots.GetOrderTemplate(id)
	if err != nil {
		return nil, err
	}

	// Get associated nodes
	var nodeAssociations []models.OrderTemplateNode
	err = ots.db.DB.Where("order_template_id = ?", id).
		Preload("NodeTemplate").
		Find(&nodeAssociations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get template nodes: %w", err)
	}

	// Get associated edges
	var edgeAssociations []models.OrderTemplateEdge
	err = ots.db.DB.Where("order_template_id = ?", id).
		Preload("EdgeTemplate").
		Find(&edgeAssociations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get template edges: %w", err)
	}

	// Extract nodes and edges with their actions
	nodesWithActions := make([]NodeWithActions, len(nodeAssociations))
	for i, assoc := range nodeAssociations {
		// Get actions for this node
		actionIDs, err := assoc.NodeTemplate.GetActionTemplateIDs()
		if err != nil {
			return nil, fmt.Errorf("failed to parse node action template IDs: %w", err)
		}

		var actions []models.ActionTemplate
		if len(actionIDs) > 0 {
			err = ots.db.DB.Where("id IN ?", actionIDs).
				Preload("Parameters").
				Find(&actions).Error
			if err != nil {
				return nil, fmt.Errorf("failed to get node actions: %w", err)
			}
		}

		nodesWithActions[i] = NodeWithActions{
			NodeTemplate: assoc.NodeTemplate,
			Actions:      actions,
		}
	}

	edgesWithActions := make([]EdgeWithActions, len(edgeAssociations))
	for i, assoc := range edgeAssociations {
		// Get actions for this edge
		actionIDs, err := assoc.EdgeTemplate.GetActionTemplateIDs()
		if err != nil {
			return nil, fmt.Errorf("failed to parse edge action template IDs: %w", err)
		}

		var actions []models.ActionTemplate
		if len(actionIDs) > 0 {
			err = ots.db.DB.Where("id IN ?", actionIDs).
				Preload("Parameters").
				Find(&actions).Error
			if err != nil {
				return nil, fmt.Errorf("failed to get edge actions: %w", err)
			}
		}

		edgesWithActions[i] = EdgeWithActions{
			EdgeTemplate: assoc.EdgeTemplate,
			Actions:      actions,
		}
	}

	result := &OrderTemplateWithDetails{
		OrderTemplate:    *template,
		NodesWithActions: nodesWithActions,
		EdgesWithActions: edgesWithActions,
	}

	return result, nil
}

func (ots *OrderTemplateService) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	var templates []models.OrderTemplate
	query := ots.db.DB.Order("created_at DESC")

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

func (ots *OrderTemplateService) UpdateOrderTemplate(id uint, req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	// Start transaction
	tx := ots.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update basic template info
	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := tx.Model(&models.OrderTemplate{}).Where("id = ?", id).Updates(template).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update order template: %w", err)
	}

	// Delete existing associations
	if err := tx.Where("order_template_id = ?", id).Delete(&models.OrderTemplateNode{}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete existing node associations: %w", err)
	}
	if err := tx.Where("order_template_id = ?", id).Delete(&models.OrderTemplateEdge{}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete existing edge associations: %w", err)
	}

	// Create new node associations
	for _, nodeID := range req.NodeIDs {
		var node models.NodeTemplate
		if err := tx.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("node '%s' not found: %w", nodeID, err)
		}

		association := &models.OrderTemplateNode{
			OrderTemplateID: id,
			NodeTemplateID:  node.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to associate node '%s': %w", nodeID, err)
		}
	}

	// Create new edge associations
	for _, edgeID := range req.EdgeIDs {
		var edge models.EdgeTemplate
		if err := tx.Where("edge_id = ?", edgeID).First(&edge).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("edge '%s' not found: %w", edgeID, err)
		}

		association := &models.OrderTemplateEdge{
			OrderTemplateID: id,
			EdgeTemplateID:  edge.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to associate edge '%s': %w", edgeID, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return ots.GetOrderTemplate(id)
}

func (ots *OrderTemplateService) DeleteOrderTemplate(id uint) error {
	tx := ots.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete associations
	if err := tx.Where("order_template_id = ?", id).Delete(&models.OrderTemplateNode{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete node associations: %w", err)
	}
	if err := tx.Where("order_template_id = ?", id).Delete(&models.OrderTemplateEdge{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete edge associations: %w", err)
	}

	// Delete template
	if err := tx.Delete(&models.OrderTemplate{}, id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete order template: %w", err)
	}

	return tx.Commit().Error
}

// Template Association Management

func (ots *OrderTemplateService) AssociateNodes(templateID uint, req *models.AssociateNodesRequest) error {
	tx := ots.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, nodeID := range req.NodeIDs {
		var node models.NodeTemplate
		if err := tx.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("node '%s' not found: %w", nodeID, err)
		}

		// Check if association already exists
		var existing models.OrderTemplateNode
		err := tx.Where("order_template_id = ? AND node_template_id = ?", templateID, node.ID).First(&existing).Error
		if err == nil {
			continue // Association already exists
		}

		association := &models.OrderTemplateNode{
			OrderTemplateID: templateID,
			NodeTemplateID:  node.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to associate node '%s': %w", nodeID, err)
		}
	}

	return tx.Commit().Error
}

func (ots *OrderTemplateService) AssociateEdges(templateID uint, req *models.AssociateEdgesRequest) error {
	tx := ots.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, edgeID := range req.EdgeIDs {
		var edge models.EdgeTemplate
		if err := tx.Where("edge_id = ?", edgeID).First(&edge).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("edge '%s' not found: %w", edgeID, err)
		}

		// Check if association already exists
		var existing models.OrderTemplateEdge
		err := tx.Where("order_template_id = ? AND edge_template_id = ?", templateID, edge.ID).First(&existing).Error
		if err == nil {
			continue // Association already exists
		}

		association := &models.OrderTemplateEdge{
			OrderTemplateID: templateID,
			EdgeTemplateID:  edge.ID,
		}
		if err := tx.Create(association).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to associate edge '%s': %w", edgeID, err)
		}
	}

	return tx.Commit().Error
}

// Helper structures are defined in types.go
