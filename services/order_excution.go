package services

import (
	"fmt"
	"time"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"
)

// OrderExecutionService handles the business logic for executing and managing orders.
type OrderExecutionService struct {
	orderExecutionRepo interfaces.OrderExecutionRepositoryInterface
	orderTemplateRepo  interfaces.OrderTemplateRepositoryInterface
	connectionRepo     interfaces.ConnectionRepositoryInterface
	actionRepo         interfaces.ActionRepositoryInterface
	redis              *redis.RedisClient
	mqttClient         *mqtt.Client
	uow                database.UnitOfWorkInterface
}

// NewOrderExecutionService creates a new instance of OrderExecutionService with all its dependencies.
func NewOrderExecutionService(
	orderExecutionRepo interfaces.OrderExecutionRepositoryInterface,
	orderTemplateRepo interfaces.OrderTemplateRepositoryInterface,
	connectionRepo interfaces.ConnectionRepositoryInterface,
	actionRepo interfaces.ActionRepositoryInterface,
	redisClient *redis.RedisClient,
	mqttClient *mqtt.Client,
	uow database.UnitOfWorkInterface,
) *OrderExecutionService {
	return &OrderExecutionService{
		orderExecutionRepo: orderExecutionRepo,
		orderTemplateRepo:  orderTemplateRepo,
		connectionRepo:     connectionRepo,
		actionRepo:         actionRepo,
		redis:              redisClient,
		mqttClient:         mqttClient,
		uow:                uow,
	}
}

// GetRobotManufacturer retrieves the manufacturer for a given robot, providing a default if not found.
func (oes *OrderExecutionService) GetRobotManufacturer(serialNumber string) string {
	manufacturer, err := oes.connectionRepo.GetRobotManufacturer(serialNumber)
	if err != nil {
		return "Roboligent" // Default fallback
	}
	return manufacturer
}

// ExecuteOrder validates conditions and executes an order based on a template within a single transaction.
func (oes *OrderExecutionService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	template, nodes, edges, err := oes.orderTemplateRepo.GetOrderTemplateWithDetails(req.TemplateID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Order template with ID %d not found.", req.TemplateID))
	}

	connectionStatus, err := oes.redis.GetConnectionStatus(req.SerialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, utils.NewBadRequestError(fmt.Sprintf("Cannot execute order: Robot %s is not online.", req.SerialNumber))
	}

	orderID := oes.generateUniqueOrderID()
	execution := &models.OrderExecution{
		OrderID:         orderID,
		OrderTemplateID: &template.ID,
		SerialNumber:    req.SerialNumber,
		Status:          "CREATED",
	}

	tx := oes.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			oes.uow.Rollback(tx)
			panic(r)
		}
	}()

	createdExecution, err := oes.orderExecutionRepo.CreateOrderExecution(tx, execution)
	if err != nil {
		oes.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to create order execution record.", err)
	}

	orderMsg, err := oes.convertTemplateToOrderMessage(nodes, edges, orderID, req.SerialNumber, req.ParameterOverrides)
	if err != nil {
		oes.orderExecutionRepo.SetOrderFailed(tx, orderID, err.Error())
		oes.uow.Commit(tx)
		return nil, utils.NewInternalServerError("Failed to convert template to order message.", err)
	}

	if err := oes.mqttClient.SendOrder(req.SerialNumber, orderMsg); err != nil {
		oes.orderExecutionRepo.SetOrderFailed(tx, orderID, err.Error())
		oes.uow.Commit(tx)
		return nil, utils.NewInternalServerError("Failed to send order via MQTT.", err)
	}

	if err := oes.orderExecutionRepo.UpdateOrderStatus(tx, orderID, "SENT"); err != nil {
		oes.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to update order status to SENT.", err)
	}

	if err := oes.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit order execution transaction.", err)
	}

	return &models.OrderExecutionResponse{
		OrderID:         orderID,
		Status:          "SENT",
		SerialNumber:    req.SerialNumber,
		OrderTemplateID: &template.ID,
		CreatedAt:       createdExecution.CreatedAt,
	}, nil
}

// convertTemplateToOrderMessage builds an MQTT-compatible OrderMessage from database templates.
func (oes *OrderExecutionService) convertTemplateToOrderMessage(
	nodes []models.NodeTemplate,
	edges []models.EdgeTemplate,
	orderID, serialNumber string,
	paramOverrides map[string]interface{},
) (*models.OrderMessage, error) {

	orderNodes := make([]models.Node, len(nodes))
	for i, nodeTmpl := range nodes {
		node := nodeTmpl.ToNode()
		actionIDs, _ := nodeTmpl.GetActionTemplateIDs()
		if len(actionIDs) > 0 {
			actions, err := oes.fetchAndConvertActions(actionIDs, paramOverrides)
			if err != nil {
				return nil, fmt.Errorf("error processing actions for node %s: %w", nodeTmpl.NodeID, err)
			}
			node.Actions = actions
		}
		orderNodes[i] = node
	}

	orderEdges := make([]models.Edge, len(edges))
	for i, edgeTmpl := range edges {
		edge := edgeTmpl.ToEdge()
		actionIDs, _ := edgeTmpl.GetActionTemplateIDs()
		if len(actionIDs) > 0 {
			actions, err := oes.fetchAndConvertActions(actionIDs, paramOverrides)
			if err != nil {
				return nil, fmt.Errorf("error processing actions for edge %s: %w", edgeTmpl.EdgeID, err)
			}
			edge.Actions = actions
		}
		orderEdges[i] = edge
	}

	orderMsg := &models.OrderMessage{
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes:         orderNodes,
		Edges:         orderEdges,
	}
	return orderMsg, nil
}

// fetchAndConvertActions retrieves action templates from DB and converts them to MQTT Action structs.
func (oes *OrderExecutionService) fetchAndConvertActions(ids []uint, overrides map[string]interface{}) ([]models.Action, error) {
	actions := make([]models.Action, 0, len(ids))
	for _, actionID := range ids {
		actionTmpl, err := oes.actionRepo.GetActionTemplate(actionID)
		if err != nil {
			return nil, fmt.Errorf("could not find action template with ID %d: %w", actionID, err)
		}
		action := actionTmpl.ToAction()
		if overrides != nil {
			oes.applyParameterOverrides(&action, overrides)
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// applyParameterOverrides modifies action parameters based on runtime overrides.
func (oes *OrderExecutionService) applyParameterOverrides(action *models.Action, overrides map[string]interface{}) {
	for i, param := range action.ActionParameters {
		if overrideValue, exists := overrides[param.Key]; exists {
			action.ActionParameters[i].Value = overrideValue
		}
	}
}

// generateUniqueOrderID creates a unique ID for the order.
func (oes *OrderExecutionService) generateUniqueOrderID() string {
	return fmt.Sprintf("order_%x", time.Now().UnixNano())
}

// GetOrderExecution retrieves a single order execution by its string ID.
func (oes *OrderExecutionService) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	execution, err := oes.orderExecutionRepo.GetOrderExecution(orderID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Order execution with ID '%s' not found.", orderID))
	}
	return execution, nil
}

// ListOrderExecutions retrieves a paginated list of order executions.
func (oes *OrderExecutionService) ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error) {
	executions, err := oes.orderExecutionRepo.ListOrderExecutions(serialNumber, limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to list order executions.", err)
	}
	return executions, nil
}

// CancelOrder cancels an active order within a transaction.
func (oes *OrderExecutionService) CancelOrder(orderID string) error {
	execution, err := oes.orderExecutionRepo.GetOrderExecution(orderID)
	if err != nil {
		return utils.NewNotFoundError(fmt.Sprintf("Order with ID '%s' not found for cancellation.", orderID))
	}

	if execution.Status == "COMPLETED" || execution.Status == "FAILED" || execution.Status == "CANCELLED" {
		return utils.NewBadRequestError(fmt.Sprintf("Order cannot be cancelled, current status: %s", execution.Status))
	}

	tx := oes.uow.Begin()
	if err := oes.orderExecutionRepo.SetOrderCancelled(tx, orderID, "Order cancelled by user"); err != nil {
		oes.uow.Rollback(tx)
		return utils.NewInternalServerError("Failed to cancel order.", err)
	}
	return oes.uow.Commit(tx)
}

// UpdateOrderStatus updates the status of an order.
func (oes *OrderExecutionService) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	var errMsg string
	if len(errorMessage) > 0 {
		errMsg = errorMessage[0]
	}
	tx := oes.uow.Begin()
	if err := oes.orderExecutionRepo.UpdateOrderStatus(tx, orderID, status, errMsg); err != nil {
		oes.uow.Rollback(tx)
		return utils.NewInternalServerError("Failed to update order status.", err)
	}
	return oes.uow.Commit(tx)
}

// ExecuteDirectOrder sends an order defined directly in the request body.
func (oes *OrderExecutionService) ExecuteDirectOrder(serialNumber string, orderData *models.OrderMessage) (*models.OrderExecutionResponse, error) {
	connectionStatus, err := oes.redis.GetConnectionStatus(serialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, utils.NewBadRequestError(fmt.Sprintf("Robot %s is not online.", serialNumber))
	}

	if orderData.Manufacturer == "" {
		orderData.Manufacturer = oes.GetRobotManufacturer(serialNumber)
	}

	execution := &models.OrderExecution{
		OrderID:       orderData.OrderID,
		SerialNumber:  serialNumber,
		OrderUpdateID: orderData.OrderUpdateID,
		Status:        "CREATED",
	}

	tx := oes.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			oes.uow.Rollback(tx)
			panic(r)
		}
	}()

	createdExecution, err := oes.orderExecutionRepo.CreateOrderExecution(tx, execution)
	if err != nil {
		oes.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to create direct order execution record.", err)
	}

	if err := oes.mqttClient.SendOrder(serialNumber, orderData); err != nil {
		oes.orderExecutionRepo.SetOrderFailed(tx, orderData.OrderID, err.Error())
		oes.uow.Commit(tx)
		return nil, utils.NewInternalServerError("Failed to send direct order.", err)
	}

	oes.orderExecutionRepo.SetOrderStarted(tx, orderData.OrderID)

	if err := oes.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit direct order transaction.", err)
	}

	return &models.OrderExecutionResponse{
		OrderID:      orderData.OrderID,
		Status:       "SENT",
		SerialNumber: serialNumber,
		CreatedAt:    createdExecution.CreatedAt,
	}, nil
}
