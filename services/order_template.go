package services

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
)

type OrderTemplateService struct {
	orderTemplateRepo interfaces.OrderTemplateRepositoryInterface
	actionRepo        interfaces.ActionRepositoryInterface
}

func NewOrderTemplateService(orderTemplateRepo interfaces.OrderTemplateRepositoryInterface, actionRepo interfaces.ActionRepositoryInterface) *OrderTemplateService {
	return &OrderTemplateService{
		orderTemplateRepo: orderTemplateRepo,
		actionRepo:        actionRepo,
	}
}

func (ots *OrderTemplateService) CreateOrderTemplate(req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	// Create basic order template
	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	// Create order template using repository
	createdTemplate, err := ots.orderTemplateRepo.CreateOrderTemplate(template)
	if err != nil {
		return nil, fmt.Errorf("failed to create order template: %w", err)
	}

	// Associate nodes if provided
	if len(req.NodeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateNodes(createdTemplate.ID, req.NodeIDs); err != nil {
			// Log error but don't fail the creation
			fmt.Printf("Warning: failed to associate some nodes: %v\n", err)
		}
	}

	// Associate edges if provided
	if len(req.EdgeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateEdges(createdTemplate.ID, req.EdgeIDs); err != nil {
			// Log error but don't fail the creation
			fmt.Printf("Warning: failed to associate some edges: %v\n", err)
		}
	}

	return ots.orderTemplateRepo.GetOrderTemplate(createdTemplate.ID)
}

func (ots *OrderTemplateService) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	return ots.orderTemplateRepo.GetOrderTemplate(id)
}

func (ots *OrderTemplateService) GetOrderTemplateWithDetails(id uint) (*models.OrderTemplateWithDetails, error) {
	// Get template with basic node and edge associations
	template, nodes, edges, err := ots.orderTemplateRepo.GetOrderTemplateWithDetails(id)
	if err != nil {
		return nil, err
	}

	// Build NodesWithActions
	nodesWithActions := make([]models.NodeWithActions, 0, len(nodes))
	for _, node := range nodes {
		// Get action templates for this node
		actionIDs, err := node.GetActionTemplateIDs()
		if err != nil {
			actionIDs = []uint{} // If error parsing, use empty slice
		}

		var actions []models.ActionTemplate
		for _, actionID := range actionIDs {
			action, err := ots.actionRepo.GetActionTemplate(actionID)
			if err == nil {
				actions = append(actions, *action)
			}
		}

		nodesWithActions = append(nodesWithActions, models.NodeWithActions{
			NodeTemplate: node,
			Actions:      actions,
		})
	}

	// Build EdgesWithActions
	edgesWithActions := make([]models.EdgeWithActions, 0, len(edges))
	for _, edge := range edges {
		// Get action templates for this edge
		actionIDs, err := edge.GetActionTemplateIDs()
		if err != nil {
			actionIDs = []uint{} // If error parsing, use empty slice
		}

		var actions []models.ActionTemplate
		for _, actionID := range actionIDs {
			action, err := ots.actionRepo.GetActionTemplate(actionID)
			if err == nil {
				actions = append(actions, *action)
			}
		}

		edgesWithActions = append(edgesWithActions, models.EdgeWithActions{
			EdgeTemplate: edge,
			Actions:      actions,
		})
	}

	return &models.OrderTemplateWithDetails{
		OrderTemplate:    *template,
		NodesWithActions: nodesWithActions,
		EdgesWithActions: edgesWithActions,
	}, nil
}

func (ots *OrderTemplateService) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	return ots.orderTemplateRepo.ListOrderTemplates(limit, offset)
}

func (ots *OrderTemplateService) UpdateOrderTemplate(id uint, req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	// Update basic template information
	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	updatedTemplate, err := ots.orderTemplateRepo.UpdateOrderTemplate(id, template)
	if err != nil {
		return nil, fmt.Errorf("failed to update order template: %w", err)
	}

	// Remove existing associations
	ots.orderTemplateRepo.RemoveNodeAssociations(id)
	ots.orderTemplateRepo.RemoveEdgeAssociations(id)

	// Associate new nodes
	if len(req.NodeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateNodes(id, req.NodeIDs); err != nil {
			fmt.Printf("Warning: failed to associate some nodes: %v\n", err)
		}
	}

	// Associate new edges
	if len(req.EdgeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateEdges(id, req.EdgeIDs); err != nil {
			fmt.Printf("Warning: failed to associate some edges: %v\n", err)
		}
	}

	return updatedTemplate, nil
}

func (ots *OrderTemplateService) DeleteOrderTemplate(id uint) error {
	return ots.orderTemplateRepo.DeleteOrderTemplate(id)
}

func (ots *OrderTemplateService) AssociateNodes(templateID uint, req *models.AssociateNodesRequest) error {
	return ots.orderTemplateRepo.AssociateNodes(templateID, req.NodeIDs)
}

func (ots *OrderTemplateService) AssociateEdges(templateID uint, req *models.AssociateEdgesRequest) error {
	return ots.orderTemplateRepo.AssociateEdges(templateID, req.EdgeIDs)
}
