package services

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/services/base"
	"mqtt-bridge/utils"
)

type OrderExecutionService struct {
	orderExecutionRepo  interfaces.OrderExecutionRepositoryInterface
	orderTemplateRepo   interfaces.OrderTemplateRepositoryInterface
	redis               *redis.RedisClient
	mqttClient          *mqtt.Client
	manufacturerManager *base.ManufacturerManager
}

func NewOrderExecutionService(
	orderExecutionRepo interfaces.OrderExecutionRepositoryInterface,
	orderTemplateRepo interfaces.OrderTemplateRepositoryInterface,
	connectionRepo interfaces.ConnectionRepositoryInterface,
	redisClient *redis.RedisClient,
	mqttClient *mqtt.Client,
) *OrderExecutionService {
	return &OrderExecutionService{
		orderExecutionRepo:  orderExecutionRepo,
		orderTemplateRepo:   orderTemplateRepo,
		redis:               redisClient,
		mqttClient:          mqttClient,
		manufacturerManager: base.NewManufacturerManager(connectionRepo),
	}
}

func (oes *OrderExecutionService) GetRobotManufacturer(serialNumber string) string {
	return oes.manufacturerManager.GetRobotManufacturer(serialNumber)
}

func (oes *OrderExecutionService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	template, nodes, edges, err := oes.orderTemplateRepo.GetOrderTemplateWithDetails(req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	if !oes.redis.IsRobotOnline(req.SerialNumber) {
		return nil, fmt.Errorf("robot %s is not online", req.SerialNumber)
	}

	orderID := utils.GenerateUniqueOrderID()

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

	orderMsg, err := oes.convertTemplateToOrderMessage(template, nodes, edges, orderID, req.SerialNumber, req.ParameterOverrides)
	if err != nil {
		oes.orderExecutionRepo.SetOrderFailed(orderID, err.Error())
		return nil, fmt.Errorf("failed to convert template to order: %w", err)
	}

	if err := oes.mqttClient.SendOrder(req.SerialNumber, orderMsg); err != nil {
		oes.orderExecutionRepo.SetOrderFailed(orderID, err.Error())
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	oes.orderExecutionRepo.SetOrderStarted(orderID)

	return &models.OrderExecutionResponse{
		OrderID:         orderID,
		Status:          "SENT",
		SerialNumber:    req.SerialNumber,
		OrderTemplateID: &template.ID,
		CreatedAt:       createdExecution.CreatedAt,
	}, nil
}

func (oes *OrderExecutionService) convertTemplateToOrderMessage(template *models.OrderTemplate, nodes []models.NodeTemplate, edges []models.EdgeTemplate, orderID, serialNumber string, paramOverrides map[string]interface{}) (*models.OrderMessage, error) {
	return &models.OrderMessage{
		HeaderID:      1,
		Timestamp:     utils.GetCurrentTimestamp(),
		Version:       "2.0.0",
		Manufacturer:  oes.GetRobotManufacturer(serialNumber),
		SerialNumber:  serialNumber,
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes:         oes.convertNodesToOrder(nodes, paramOverrides),
		Edges:         oes.convertEdgesToOrder(edges, paramOverrides),
	}, nil
}

// 중복되던 노드/엣지 변환 로직 분리
func (oes *OrderExecutionService) convertNodesToOrder(nodes []models.NodeTemplate, paramOverrides map[string]interface{}) []models.Node {
	orderNodes := make([]models.Node, len(nodes))
	for i, nodeTemplate := range nodes {
		node := nodeTemplate.ToNode()
		node.Actions = oes.getActionsFromTemplate(nodeTemplate.ActionTemplateIDs, paramOverrides)
		orderNodes[i] = node
	}
	return orderNodes
}

func (oes *OrderExecutionService) convertEdgesToOrder(edges []models.EdgeTemplate, paramOverrides map[string]interface{}) []models.Edge {
	orderEdges := make([]models.Edge, len(edges))
	for i, edgeTemplate := range edges {
		edge := edgeTemplate.ToEdge()
		edge.Actions = oes.getActionsFromTemplate(edgeTemplate.ActionTemplateIDs, paramOverrides)
		orderEdges[i] = edge
	}
	return orderEdges
}

// 공통 액션 생성 로직
func (oes *OrderExecutionService) getActionsFromTemplate(actionTemplateIDs string, paramOverrides map[string]interface{}) []models.Action {
	actionIDs, err := utils.ParseJSONToUintSlice(actionTemplateIDs)
	if err != nil || len(actionIDs) == 0 {
		return []models.Action{}
	}

	actions := make([]models.Action, 0, len(actionIDs))
	for _, actionID := range actionIDs {
		action := models.Action{
			ActionType:       "default",
			ActionID:         fmt.Sprintf("action_%d", actionID),
			BlockingType:     "NONE",
			ActionParameters: []models.ActionParameter{},
		}

		if paramOverrides != nil {
			oes.applyParameterOverrides(&action, paramOverrides)
		}

		actions = append(actions, action)
	}
	return actions
}

func (oes *OrderExecutionService) applyParameterOverrides(action *models.Action, overrides map[string]interface{}) {
	for i, param := range action.ActionParameters {
		if overrideValue, exists := overrides[param.Key]; exists {
			action.ActionParameters[i].Value = overrideValue
		}
	}
}

func (oes *OrderExecutionService) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	return oes.orderExecutionRepo.GetOrderExecution(orderID)
}

func (oes *OrderExecutionService) ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error) {
	return oes.orderExecutionRepo.ListOrderExecutions(serialNumber, limit, offset)
}

func (oes *OrderExecutionService) CancelOrder(orderID string) error {
	execution, err := oes.orderExecutionRepo.GetOrderExecution(orderID)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	if !utils.IsValidOrderStatus(execution.Status) ||
		execution.Status == string(utils.OrderStatusCompleted) ||
		execution.Status == string(utils.OrderStatusFailed) ||
		execution.Status == string(utils.OrderStatusCancelled) {
		return fmt.Errorf("order cannot be cancelled, current status: %s", execution.Status)
	}

	return oes.orderExecutionRepo.SetOrderCancelled(orderID, "Order cancelled by user")
}

func (oes *OrderExecutionService) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	if !utils.IsValidOrderStatus(status) {
		return fmt.Errorf("invalid order status: %s", status)
	}

	var errMsg string
	if len(errorMessage) > 0 && errorMessage[0] != "" {
		errMsg = errorMessage[0]
	}

	return oes.orderExecutionRepo.UpdateOrderStatus(orderID, status, errMsg)
}

func (oes *OrderExecutionService) ExecuteDirectOrder(serialNumber string, orderData *models.OrderMessage) (*models.OrderExecutionResponse, error) {
	if !oes.redis.IsRobotOnline(serialNumber) {
		return nil, fmt.Errorf("robot %s is not online", serialNumber)
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

	createdExecution, err := oes.orderExecutionRepo.CreateOrderExecution(execution)
	if err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	if err := oes.mqttClient.SendOrder(serialNumber, orderData); err != nil {
		oes.orderExecutionRepo.SetOrderFailed(orderData.OrderID, err.Error())
		return nil, fmt.Errorf("failed to send direct order: %w", err)
	}

	oes.orderExecutionRepo.SetOrderStarted(orderData.OrderID)

	return &models.OrderExecutionResponse{
		OrderID:      orderData.OrderID,
		Status:       "SENT",
		SerialNumber: serialNumber,
		CreatedAt:    createdExecution.CreatedAt,
	}, nil
}
