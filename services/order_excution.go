package services

import (
	"fmt"
	"time"

	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
	"mqtt-bridge/repositories/interfaces"
)

type OrderExecutionService struct {
	orderExecutionRepo interfaces.OrderExecutionRepositoryInterface
	orderTemplateRepo  interfaces.OrderTemplateRepositoryInterface
	connectionRepo     interfaces.ConnectionRepositoryInterface
	redis              *redis.RedisClient
	mqttClient         *mqtt.Client
}

func NewOrderExecutionService(
	orderExecutionRepo interfaces.OrderExecutionRepositoryInterface,
	orderTemplateRepo interfaces.OrderTemplateRepositoryInterface,
	connectionRepo interfaces.ConnectionRepositoryInterface,
	redisClient *redis.RedisClient,
	mqttClient *mqtt.Client,
) *OrderExecutionService {
	return &OrderExecutionService{
		orderExecutionRepo: orderExecutionRepo,
		orderTemplateRepo:  orderTemplateRepo,
		connectionRepo:     connectionRepo,
		redis:              redisClient,
		mqttClient:         mqttClient,
	}
}

func (oes *OrderExecutionService) GetRobotManufacturer(serialNumber string) string {
	manufacturer, err := oes.connectionRepo.GetRobotManufacturer(serialNumber)
	if err != nil {
		return "Roboligent" // Default fallback
	}
	return manufacturer
}

func (oes *OrderExecutionService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	// Get template with details using repository
	template, nodes, edges, err := oes.orderTemplateRepo.GetOrderTemplateWithDetails(req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	// Check robot connection status
	connectionStatus, err := oes.redis.GetConnectionStatus(req.SerialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, fmt.Errorf("robot %s is not online", req.SerialNumber)
	}

	// Generate unique order ID
	orderID := oes.generateUniqueOrderID()

	// Create order execution record
	execution := &models.OrderExecution{
		OrderID:         orderID,
		OrderTemplateID: &template.ID,
		SerialNumber:    req.SerialNumber,
		OrderUpdateID:   0,
		Status:          "CREATED",
	}

	createdExecution, err := oes.orderExecutionRepo.CreateOrderExecution(execution)
	if err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	// Convert template to order message
	orderMsg, err := oes.convertTemplateToOrderMessage(template, nodes, edges, orderID, req.SerialNumber, req.ParameterOverrides)
	if err != nil {
		oes.orderExecutionRepo.SetOrderFailed(orderID, err.Error())
		return nil, fmt.Errorf("failed to convert template to order: %w", err)
	}

	// Send order via MQTT
	if err := oes.mqttClient.SendOrder(req.SerialNumber, orderMsg); err != nil {
		oes.orderExecutionRepo.SetOrderFailed(orderID, err.Error())
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	// Update execution status to SENT
	oes.orderExecutionRepo.SetOrderStarted(orderID)

	return &models.OrderExecutionResponse{
		OrderID:         orderID,
		Status:          "SENT",
		SerialNumber:    req.SerialNumber,
		OrderTemplateID: &template.ID,
		CreatedAt:       createdExecution.CreatedAt,
	}, nil
}

func (oes *OrderExecutionService) convertTemplateToOrderMessage(
	template *models.OrderTemplate,
	nodes []models.NodeTemplate,
	edges []models.EdgeTemplate,
	orderID, serialNumber string,
	paramOverrides map[string]interface{},
) (*models.OrderMessage, error) {
	manufacturer := oes.GetRobotManufacturer(serialNumber)

	// Convert node templates to nodes
	orderNodes := make([]models.Node, len(nodes))
	for i, nodeTemplate := range nodes {
		node := nodeTemplate.ToNode()

		// Get action templates for this node and convert to actions
		actionIDs, err := nodeTemplate.GetActionTemplateIDs()
		if err == nil && len(actionIDs) > 0 {
			actions := make([]models.Action, 0, len(actionIDs))
			for _, actionID := range actionIDs {
				// Note: In a full implementation, you'd get action templates from repository
				// For now, we'll create empty actions array
				actions = append(actions, models.Action{
					ActionType:       "default",
					ActionID:         fmt.Sprintf("action_%d", actionID),
					BlockingType:     "NONE",
					ActionParameters: []models.ActionParameter{},
				})
			}
			node.Actions = actions
		}

		// Apply parameter overrides if provided
		if paramOverrides != nil {
			for j := range node.Actions {
				oes.applyParameterOverrides(&node.Actions[j], paramOverrides)
			}
		}

		orderNodes[i] = node
	}

	// Convert edge templates to edges
	orderEdges := make([]models.Edge, len(edges))
	for i, edgeTemplate := range edges {
		edge := edgeTemplate.ToEdge()

		// Get action templates for this edge and convert to actions
		actionIDs, err := edgeTemplate.GetActionTemplateIDs()
		if err == nil && len(actionIDs) > 0 {
			actions := make([]models.Action, 0, len(actionIDs))
			for _, actionID := range actionIDs {
				// Note: In a full implementation, you'd get action templates from repository
				// For now, we'll create empty actions array
				actions = append(actions, models.Action{
					ActionType:       "default",
					ActionID:         fmt.Sprintf("action_%d", actionID),
					BlockingType:     "NONE",
					ActionParameters: []models.ActionParameter{},
				})
			}
			edge.Actions = actions
		}

		// Apply parameter overrides if provided
		if paramOverrides != nil {
			for j := range edge.Actions {
				oes.applyParameterOverrides(&edge.Actions[j], paramOverrides)
			}
		}

		orderEdges[i] = edge
	}

	orderMsg := &models.OrderMessage{
		HeaderID:      1,
		Timestamp:     time.Now().Format("2006-01-02T15:04:05.000000000Z"),
		Version:       "2.0.0",
		Manufacturer:  manufacturer,
		SerialNumber:  serialNumber,
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes:         orderNodes,
		Edges:         orderEdges,
	}

	return orderMsg, nil
}

func (oes *OrderExecutionService) applyParameterOverrides(action *models.Action, overrides map[string]interface{}) {
	for i, param := range action.ActionParameters {
		if overrideValue, exists := overrides[param.Key]; exists {
			action.ActionParameters[i].Value = overrideValue
		}
	}
}

func (oes *OrderExecutionService) generateUniqueOrderID() string {
	return fmt.Sprintf("order_%x", time.Now().UnixNano())
}

func (oes *OrderExecutionService) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	return oes.orderExecutionRepo.GetOrderExecution(orderID)
}

func (oes *OrderExecutionService) ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error) {
	return oes.orderExecutionRepo.ListOrderExecutions(serialNumber, limit, offset)
}

func (oes *OrderExecutionService) CancelOrder(orderID string) error {
	// Get current execution status
	execution, err := oes.orderExecutionRepo.GetOrderExecution(orderID)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	// Check if order can be cancelled
	if execution.Status == "COMPLETED" || execution.Status == "FAILED" || execution.Status == "CANCELLED" {
		return fmt.Errorf("order cannot be cancelled, current status: %s", execution.Status)
	}

	// Cancel the order
	return oes.orderExecutionRepo.SetOrderCancelled(orderID, "Order cancelled by user")
}

func (oes *OrderExecutionService) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	var errMsg string
	if len(errorMessage) > 0 && errorMessage[0] != "" {
		errMsg = errorMessage[0]
	}

	return oes.orderExecutionRepo.UpdateOrderStatus(orderID, status, errMsg)
}

func (oes *OrderExecutionService) getOrderStatus(orderID string) string {
	status, err := oes.orderExecutionRepo.GetOrderStatus(orderID)
	if err != nil {
		return "UNKNOWN"
	}
	return status
}

func (oes *OrderExecutionService) ExecuteDirectOrder(serialNumber string, orderData *models.OrderMessage) (*models.OrderExecutionResponse, error) {
	// Check robot connection status
	connectionStatus, err := oes.redis.GetConnectionStatus(serialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, fmt.Errorf("robot %s is not online", serialNumber)
	}

	// Set manufacturer if not provided
	if orderData.Manufacturer == "" {
		orderData.Manufacturer = oes.GetRobotManufacturer(serialNumber)
	}

	// Create order execution record
	execution := &models.OrderExecution{
		OrderID:       orderData.OrderID,
		SerialNumber:  serialNumber,
		OrderUpdateID: orderData.OrderUpdateID,
		Status:        "CREATED",
	}

	createdExecution, err := oes.orderExecutionRepo.CreateOrderExecution(execution)
	if err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	// Send order via MQTT
	if err := oes.mqttClient.SendOrder(serialNumber, orderData); err != nil {
		oes.orderExecutionRepo.SetOrderFailed(orderData.OrderID, err.Error())
		return nil, fmt.Errorf("failed to send direct order: %w", err)
	}

	// Update execution status to SENT
	oes.orderExecutionRepo.SetOrderStarted(orderData.OrderID)

	return &models.OrderExecutionResponse{
		OrderID:      orderData.OrderID,
		Status:       "SENT",
		SerialNumber: serialNumber,
		CreatedAt:    createdExecution.CreatedAt,
	}, nil
}
