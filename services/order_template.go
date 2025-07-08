package services

import (
	"fmt"
	"log/slog"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"
)

// OrderTemplateService handles the business logic for managing order templates.
type OrderTemplateService struct {
	orderTemplateRepo interfaces.OrderTemplateRepositoryInterface
	actionRepo        interfaces.ActionRepositoryInterface
	uow               database.UnitOfWorkInterface
	logger            *slog.Logger
}

// NewOrderTemplateService creates a new instance of OrderTemplateService.
func NewOrderTemplateService(
	orderTemplateRepo interfaces.OrderTemplateRepositoryInterface,
	actionRepo interfaces.ActionRepositoryInterface,
	uow database.UnitOfWorkInterface,
	logger *slog.Logger,
) *OrderTemplateService {
	return &OrderTemplateService{
		orderTemplateRepo: orderTemplateRepo,
		actionRepo:        actionRepo,
		uow:               uow,
		logger:            logger.With("service", "order_template_service"),
	}
}

// CreateOrderTemplate creates a new order template and associates nodes/edges within a single transaction.
func (ots *OrderTemplateService) CreateOrderTemplate(req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	tx := ots.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			ots.uow.Rollback(tx)
			panic(r)
		}
	}()

	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	createdTemplate, err := ots.orderTemplateRepo.CreateOrderTemplate(tx, template)
	if err != nil {
		ots.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to create order template.", err)
	}

	if len(req.NodeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateNodes(tx, createdTemplate.ID, req.NodeIDs); err != nil {
			ots.uow.Rollback(tx)
			return nil, utils.NewInternalServerError("Failed to associate nodes during template creation.", err)
		}
	}

	if len(req.EdgeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateEdges(tx, createdTemplate.ID, req.EdgeIDs); err != nil {
			ots.uow.Rollback(tx)
			return nil, utils.NewInternalServerError("Failed to associate edges during template creation.", err)
		}
	}

	if err := ots.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit transaction for order template creation.", err)
	}

	ots.logger.Info("Successfully created order template", "templateId", createdTemplate.ID, "name", createdTemplate.Name)
	return createdTemplate, nil
}

// GetOrderTemplate retrieves a single order template by its ID.
func (ots *OrderTemplateService) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	template, err := ots.orderTemplateRepo.GetOrderTemplate(id)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Order template with ID %d not found.", id))
	}
	return template, nil
}

// GetOrderTemplateWithDetails retrieves a template with its associated nodes, edges, and their actions.
func (ots *OrderTemplateService) GetOrderTemplateWithDetails(id uint) (*models.OrderTemplateWithDetails, error) {
	template, nodes, edges, err := ots.orderTemplateRepo.GetOrderTemplateWithDetails(id)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Order template with ID %d not found.", id))
	}

	nodesWithActions := make([]models.NodeWithActions, 0, len(nodes))
	for _, node := range nodes {
		var actions []models.ActionTemplate
		actionIDs, _ := node.GetActionTemplateIDs()
		for _, actionID := range actionIDs {
			action, err := ots.actionRepo.GetActionTemplate(actionID)
			if err != nil {
				ots.logger.Warn("Could not find action template for node", "nodeId", node.NodeID, "actionTemplateId", actionID, slog.Any("error", err))
				continue
			}
			actions = append(actions, *action)
		}
		nodesWithActions = append(nodesWithActions, models.NodeWithActions{NodeTemplate: node, Actions: actions})
	}

	edgesWithActions := make([]models.EdgeWithActions, 0, len(edges))
	for _, edge := range edges {
		var actions []models.ActionTemplate
		actionIDs, _ := edge.GetActionTemplateIDs()
		for _, actionID := range actionIDs {
			action, err := ots.actionRepo.GetActionTemplate(actionID)
			if err != nil {
				ots.logger.Warn("Could not find action template for edge", "edgeId", edge.EdgeID, "actionTemplateId", actionID, slog.Any("error", err))
				continue
			}
			actions = append(actions, *action)
		}
		edgesWithActions = append(edgesWithActions, models.EdgeWithActions{EdgeTemplate: edge, Actions: actions})
	}

	return &models.OrderTemplateWithDetails{
		OrderTemplate:    *template,
		NodesWithActions: nodesWithActions,
		EdgesWithActions: edgesWithActions,
	}, nil
}

// ListOrderTemplates retrieves a paginated list of order templates.
func (ots *OrderTemplateService) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	templates, err := ots.orderTemplateRepo.ListOrderTemplates(limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to list order templates.", err)
	}
	return templates, nil
}

// UpdateOrderTemplate updates an existing order template and its associations within a single transaction.
func (ots *OrderTemplateService) UpdateOrderTemplate(id uint, req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	tx := ots.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			ots.uow.Rollback(tx)
			panic(r)
		}
	}()

	template := &models.OrderTemplate{Name: req.Name, Description: req.Description}
	updatedTemplate, err := ots.orderTemplateRepo.UpdateOrderTemplate(tx, id, template)
	if err != nil {
		ots.uow.Rollback(tx)
		return nil, utils.NewInternalServerError(fmt.Sprintf("Failed to update order template with ID %d.", id), err)
	}

	if err := ots.orderTemplateRepo.RemoveNodeAssociations(tx, id); err != nil {
		ots.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to remove old node associations.", err)
	}
	if err := ots.orderTemplateRepo.RemoveEdgeAssociations(tx, id); err != nil {
		ots.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to remove old edge associations.", err)
	}

	if len(req.NodeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateNodes(tx, id, req.NodeIDs); err != nil {
			ots.uow.Rollback(tx)
			return nil, utils.NewInternalServerError("Failed to re-associate nodes.", err)
		}
	}
	if len(req.EdgeIDs) > 0 {
		if err := ots.orderTemplateRepo.AssociateEdges(tx, id, req.EdgeIDs); err != nil {
			ots.uow.Rollback(tx)
			return nil, utils.NewInternalServerError("Failed to re-associate edges.", err)
		}
	}

	if err := ots.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit transaction for order template update.", err)
	}
	ots.logger.Info("Successfully updated order template", "templateId", id)
	return updatedTemplate, nil
}

// DeleteOrderTemplate deletes an order template within a single transaction.
func (ots *OrderTemplateService) DeleteOrderTemplate(id uint) error {
	tx := ots.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			ots.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := ots.orderTemplateRepo.DeleteOrderTemplate(tx, id); err != nil {
		ots.uow.Rollback(tx)
		return utils.NewInternalServerError(fmt.Sprintf("Failed to delete order template with ID %d.", id), err)
	}

	if err := ots.uow.Commit(tx); err != nil {
		return utils.NewInternalServerError("Failed to commit transaction for order template deletion.", err)
	}
	ots.logger.Info("Successfully deleted order template", "templateId", id)
	return nil
}

// AssociateNodes handles the logic for associating nodes with a template.
func (ots *OrderTemplateService) AssociateNodes(templateID uint, req *models.AssociateNodesRequest) error {
	tx := ots.uow.Begin()
	if err := ots.orderTemplateRepo.AssociateNodes(tx, templateID, req.NodeIDs); err != nil {
		ots.uow.Rollback(tx)
		return utils.NewInternalServerError("Failed to associate nodes.", err)
	}
	return ots.uow.Commit(tx)
}

// AssociateEdges handles the logic for associating edges with a template.
func (ots *OrderTemplateService) AssociateEdges(templateID uint, req *models.AssociateEdgesRequest) error {
	tx := ots.uow.Begin()
	if err := ots.orderTemplateRepo.AssociateEdges(tx, templateID, req.EdgeIDs); err != nil {
		ots.uow.Rollback(tx)
		return utils.NewInternalServerError("Failed to associate edges.", err)
	}
	return ots.uow.Commit(tx)
}
