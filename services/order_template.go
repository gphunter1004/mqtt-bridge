package services

import (
	"fmt"

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

func (ots *OrderTemplateService) CreateOrderTemplate(req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := ots.db.DB.Create(template).Error; err != nil {
		return nil, fmt.Errorf("failed to create order template: %w", err)
	}

	for _, nodeID := range req.NodeIDs {
		ots.associateNode(template.ID, nodeID)
	}

	for _, edgeID := range req.EdgeIDs {
		ots.associateEdge(template.ID, edgeID)
	}

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

	var nodeAssociations []models.OrderTemplateNode
	ots.db.DB.Where("order_template_id = ?", id).
		Preload("NodeTemplate").
		Find(&nodeAssociations)

	var edgeAssociations []models.OrderTemplateEdge
	ots.db.DB.Where("order_template_id = ?", id).
		Preload("EdgeTemplate").
		Find(&edgeAssociations)

	nodesWithActions := make([]NodeWithActions, 0, len(nodeAssociations))
	for _, assoc := range nodeAssociations {
		actionIDs, err := assoc.NodeTemplate.GetActionTemplateIDs()
		if err != nil {
			actionIDs = []uint{}
		}

		var actions []models.ActionTemplate
		if len(actionIDs) > 0 {
			ots.db.DB.Where("id IN ?", actionIDs).
				Preload("Parameters").
				Find(&actions)
		}

		nodesWithActions = append(nodesWithActions, NodeWithActions{
			NodeTemplate: assoc.NodeTemplate,
			Actions:      actions,
		})
	}

	edgesWithActions := make([]EdgeWithActions, 0, len(edgeAssociations))
	for _, assoc := range edgeAssociations {
		actionIDs, err := assoc.EdgeTemplate.GetActionTemplateIDs()
		if err != nil {
			actionIDs = []uint{}
		}

		var actions []models.ActionTemplate
		if len(actionIDs) > 0 {
			ots.db.DB.Where("id IN ?", actionIDs).
				Preload("Parameters").
				Find(&actions)
		}

		edgesWithActions = append(edgesWithActions, EdgeWithActions{
			EdgeTemplate: assoc.EdgeTemplate,
			Actions:      actions,
		})
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
	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := ots.db.DB.Model(&models.OrderTemplate{}).Where("id = ?", id).Updates(template).Error; err != nil {
		return nil, fmt.Errorf("failed to update order template: %w", err)
	}

	ots.db.DB.Where("order_template_id = ?", id).Delete(&models.OrderTemplateNode{})
	ots.db.DB.Where("order_template_id = ?", id).Delete(&models.OrderTemplateEdge{})

	for _, nodeID := range req.NodeIDs {
		ots.associateNode(id, nodeID)
	}

	for _, edgeID := range req.EdgeIDs {
		ots.associateEdge(id, edgeID)
	}

	return ots.GetOrderTemplate(id)
}

func (ots *OrderTemplateService) DeleteOrderTemplate(id uint) error {
	ots.db.DB.Where("order_template_id = ?", id).Delete(&models.OrderTemplateNode{})
	ots.db.DB.Where("order_template_id = ?", id).Delete(&models.OrderTemplateEdge{})

	if err := ots.db.DB.Delete(&models.OrderTemplate{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete order template: %w", err)
	}

	return nil
}

func (ots *OrderTemplateService) AssociateNodes(templateID uint, req *models.AssociateNodesRequest) error {
	for _, nodeID := range req.NodeIDs {
		ots.associateNode(templateID, nodeID)
	}
	return nil
}

func (ots *OrderTemplateService) AssociateEdges(templateID uint, req *models.AssociateEdgesRequest) error {
	for _, edgeID := range req.EdgeIDs {
		ots.associateEdge(templateID, edgeID)
	}
	return nil
}

func (ots *OrderTemplateService) associateNode(templateID uint, nodeID string) error {
	var node models.NodeTemplate
	if err := ots.db.DB.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return fmt.Errorf("node '%s' not found: %w", nodeID, err)
	}

	var existing models.OrderTemplateNode
	err := ots.db.DB.Where("order_template_id = ? AND node_template_id = ?", templateID, node.ID).First(&existing).Error
	if err == nil {
		return nil
	}

	association := &models.OrderTemplateNode{
		OrderTemplateID: templateID,
		NodeTemplateID:  node.ID,
	}

	return ots.db.DB.Create(association).Error
}

func (ots *OrderTemplateService) associateEdge(templateID uint, edgeID string) error {
	var edge models.EdgeTemplate
	if err := ots.db.DB.Where("edge_id = ?", edgeID).First(&edge).Error; err != nil {
		return fmt.Errorf("edge '%s' not found: %w", edgeID, err)
	}

	var existing models.OrderTemplateEdge
	err := ots.db.DB.Where("order_template_id = ? AND edge_template_id = ?", templateID, edge.ID).First(&existing).Error
	if err == nil {
		return nil
	}

	association := &models.OrderTemplateEdge{
		OrderTemplateID: templateID,
		EdgeTemplateID:  edge.ID,
	}

	return ots.db.DB.Create(association).Error
}
