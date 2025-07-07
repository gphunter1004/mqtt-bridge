package services

import (
	"fmt"
	"time"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
)

type OrderExecutionService struct {
	db                   *database.Database
	redis                *redis.RedisClient
	mqttClient           *mqtt.Client
	orderTemplateService *OrderTemplateService
}

func NewOrderExecutionService(db *database.Database, redisClient *redis.RedisClient,
	mqttClient *mqtt.Client, orderTemplateService *OrderTemplateService) *OrderExecutionService {
	return &OrderExecutionService{
		db:                   db,
		redis:                redisClient,
		mqttClient:           mqttClient,
		orderTemplateService: orderTemplateService,
	}
}

func (oes *OrderExecutionService) GetRobotManufacturer(serialNumber string) string {
	var connectionState models.ConnectionState
	err := oes.db.DB.Where("serial_number = ?", serialNumber).
		Order("created_at desc").
		First(&connectionState).Error

	if err == nil && connectionState.Manufacturer != "" {
		return connectionState.Manufacturer
	}

	return "Roboligent"
}

func (oes *OrderExecutionService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	templateDetails, err := oes.orderTemplateService.GetOrderTemplateWithDetails(req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	connectionStatus, err := oes.redis.GetConnectionStatus(req.SerialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, fmt.Errorf("robot %s is not online", req.SerialNumber)
	}

	orderID := oes.generateUniqueOrderID()

	execution := &models.OrderExecution{
		OrderID:         orderID,
		OrderTemplateID: &templateDetails.OrderTemplate.ID,
		SerialNumber:    req.SerialNumber,
		OrderUpdateID:   0,
		Status:          "CREATED",
	}

	if err := oes.db.DB.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	orderMsg, err := oes.convertTemplateToOrderMessage(templateDetails, orderID, req.SerialNumber, req.ParameterOverrides)
	if err != nil {
		oes.updateOrderStatus(orderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to convert template to order: %w", err)
	}

	if err := oes.mqttClient.SendOrder(req.SerialNumber, orderMsg); err != nil {
		oes.updateOrderStatus(orderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	now := time.Now()
	execution.Status = "SENT"
	execution.StartedAt = &now
	oes.db.DB.Save(execution)

	return &models.OrderExecutionResponse{
		OrderID:         orderID,
		Status:          "SENT",
		SerialNumber:    req.SerialNumber,
		OrderTemplateID: &templateDetails.OrderTemplate.ID,
		CreatedAt:       execution.CreatedAt,
	}, nil
}

func (oes *OrderExecutionService) convertTemplateToOrderMessage(templateDetails *OrderTemplateWithDetails,
	orderID, serialNumber string, paramOverrides map[string]interface{}) (*models.OrderMessage, error) {

	manufacturer := oes.GetRobotManufacturer(serialNumber)

	nodes := make([]models.Node, len(templateDetails.NodesWithActions))
	for i, nodeWithActions := range templateDetails.NodesWithActions {
		node := nodeWithActions.NodeTemplate.ToNode()

		actions := make([]models.Action, len(nodeWithActions.Actions))
		for j, actionTemplate := range nodeWithActions.Actions {
			actions[j] = actionTemplate.ToAction()

			if paramOverrides != nil {
				oes.applyParameterOverrides(&actions[j], paramOverrides)
			}
		}

		node.Actions = actions
		nodes[i] = node
	}

	edges := make([]models.Edge, len(templateDetails.EdgesWithActions))
	for i, edgeWithActions := range templateDetails.EdgesWithActions {
		edge := edgeWithActions.EdgeTemplate.ToEdge()

		actions := make([]models.Action, len(edgeWithActions.Actions))
		for j, actionTemplate := range edgeWithActions.Actions {
			actions[j] = actionTemplate.ToAction()

			if paramOverrides != nil {
				oes.applyParameterOverrides(&actions[j], paramOverrides)
			}
		}

		edge.Actions = actions
		edges[i] = edge
	}

	orderMsg := &models.OrderMessage{
		HeaderID:      1,
		Timestamp:     time.Now().Format("2006-01-02T15:04:05.000000000Z"),
		Version:       "2.0.0",
		Manufacturer:  manufacturer,
		SerialNumber:  serialNumber,
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes:         nodes,
		Edges:         edges,
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

func (oes *OrderExecutionService) updateOrderStatus(orderID, status, errorMessage string) {
	update := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if errorMessage != "" {
		update["error_message"] = errorMessage
	}

	if status == "COMPLETED" || status == "FAILED" || status == "CANCELLED" {
		now := time.Now()
		update["completed_at"] = &now
	}

	if status == "EXECUTING" {
		now := time.Now()
		update["started_at"] = &now
	}

	oes.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(update)
}

func (oes *OrderExecutionService) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	var execution models.OrderExecution
	err := oes.db.DB.Where("order_id = ?", orderID).First(&execution).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get order execution: %w", err)
	}
	return &execution, nil
}

func (oes *OrderExecutionService) ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oes.db.DB.Order("created_at desc")

	if serialNumber != "" {
		query = query.Where("serial_number = ?", serialNumber)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&executions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list order executions: %w", err)
	}

	return executions, nil
}

func (oes *OrderExecutionService) CancelOrder(orderID string) error {
	execution, err := oes.GetOrderExecution(orderID)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	if execution.Status == "COMPLETED" || execution.Status == "FAILED" || execution.Status == "CANCELLED" {
		return fmt.Errorf("order cannot be cancelled, current status: %s", execution.Status)
	}

	oes.updateOrderStatus(orderID, "CANCELLED", "Order cancelled by user")
	return nil
}

func (oes *OrderExecutionService) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	var errMsg string
	if len(errorMessage) > 0 && errorMessage[0] != "" {
		errMsg = errorMessage[0]
	}

	oes.updateOrderStatus(orderID, status, errMsg)
	return nil
}

func (oes *OrderExecutionService) getOrderStatus(orderID string) string {
	var execution models.OrderExecution
	err := oes.db.DB.Select("status").Where("order_id = ?", orderID).First(&execution).Error
	if err != nil {
		return "UNKNOWN"
	}
	return execution.Status
}

func (oes *OrderExecutionService) ExecuteDirectOrder(serialNumber string, orderData *models.OrderMessage) (*models.OrderExecutionResponse, error) {
	connectionStatus, err := oes.redis.GetConnectionStatus(serialNumber)
	if err != nil || connectionStatus != "ONLINE" {
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

	if err := oes.db.DB.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	if err := oes.mqttClient.SendOrder(serialNumber, orderData); err != nil {
		oes.updateOrderStatus(orderData.OrderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to send direct order: %w", err)
	}

	now := time.Now()
	execution.Status = "SENT"
	execution.StartedAt = &now
	oes.db.DB.Save(execution)

	return &models.OrderExecutionResponse{
		OrderID:      orderData.OrderID,
		Status:       "SENT",
		SerialNumber: serialNumber,
		CreatedAt:    execution.CreatedAt,
	}, nil
}
