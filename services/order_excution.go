package services

import (
	"fmt"
	"log"
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

// Order Execution

func (oes *OrderExecutionService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	// Get order template with details
	templateDetails, err := oes.orderTemplateService.GetOrderTemplateWithDetails(req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	// Check if robot is online
	connectionStatus, err := oes.redis.GetConnectionStatus(req.SerialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, fmt.Errorf("robot %s is not online", req.SerialNumber)
	}

	// Generate unique order ID
	orderID := oes.generateUniqueOrderID()

	// Create order execution record
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

	// Convert template to MQTT order message
	orderMsg, err := oes.convertTemplateToOrderMessage(templateDetails, orderID, req.SerialNumber, req.ParameterOverrides)
	if err != nil {
		// Update status to failed
		oes.updateOrderStatus(orderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to convert template to order: %w", err)
	}

	// Send order via MQTT
	if err := oes.mqttClient.SendOrder(req.SerialNumber, orderMsg); err != nil {
		// Update status to failed
		oes.updateOrderStatus(orderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	// Update status to sent
	now := time.Now()
	execution.Status = "SENT"
	execution.StartedAt = &now
	oes.db.DB.Save(execution)

	log.Printf("Order %s executed successfully for robot %s using template %d",
		orderID, req.SerialNumber, req.TemplateID)

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

	// Convert nodes
	nodes := make([]models.Node, len(templateDetails.Nodes))
	for i, nodeTemplate := range templateDetails.Nodes {
		nodes[i] = nodeTemplate.ToNode()

		// Apply parameter overrides
		if paramOverrides != nil {
			for j := range nodes[i].Actions {
				oes.applyParameterOverrides(&nodes[i].Actions[j], paramOverrides)
			}
		}
	}

	// Convert edges
	edges := make([]models.Edge, len(templateDetails.Edges))
	for i, edgeTemplate := range templateDetails.Edges {
		edges[i] = edgeTemplate.ToEdge()

		// Apply parameter overrides
		if paramOverrides != nil {
			for j := range edges[i].Actions {
				oes.applyParameterOverrides(&edges[i].Actions[j], paramOverrides)
			}
		}
	}

	orderMsg := &models.OrderMessage{
		HeaderID:      1, // Should be managed per robot
		Timestamp:     time.Now().Format("2006-01-02T15:04:05.000000000Z"),
		Version:       "2.0.0",
		Manufacturer:  "Roboligent",
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

	if status == "COMPLETED" || status == "FAILED" {
		now := time.Now()
		update["completed_at"] = &now
	}

	oes.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(update)
}

// Order Monitoring and Management

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
	// Check if order exists and is cancellable
	execution, err := oes.GetOrderExecution(orderID)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	if execution.Status == "COMPLETED" || execution.Status == "FAILED" || execution.Status == "CANCELLED" {
		return fmt.Errorf("order cannot be cancelled, current status: %s", execution.Status)
	}

	// TODO: Send cancel command to robot via MQTT
	// For now, we'll just update the status in database

	// Update order status to cancelled
	err = oes.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(map[string]interface{}{
			"status":       "CANCELLED",
			"updated_at":   time.Now(),
			"completed_at": time.Now(),
		}).Error

	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	log.Printf("Order %s cancelled successfully", orderID)
	return nil
}

// Order Status Management

func (oes *OrderExecutionService) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	update := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if len(errorMessage) > 0 && errorMessage[0] != "" {
		update["error_message"] = errorMessage[0]
	}

	if status == "EXECUTING" && oes.getOrderStatus(orderID) != "EXECUTING" {
		now := time.Now()
		update["started_at"] = &now
	}

	if status == "COMPLETED" || status == "FAILED" || status == "CANCELLED" {
		now := time.Now()
		update["completed_at"] = &now
	}

	err := oes.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(update).Error

	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	log.Printf("Order %s status updated to: %s", orderID, status)
	return nil
}

func (oes *OrderExecutionService) getOrderStatus(orderID string) string {
	var execution models.OrderExecution
	err := oes.db.DB.Select("status").Where("order_id = ?", orderID).First(&execution).Error
	if err != nil {
		return ""
	}
	return execution.Status
}

// Direct Order Execution (without template)

func (oes *OrderExecutionService) ExecuteDirectOrder(serialNumber string, orderData *models.OrderMessage) (*models.OrderExecutionResponse, error) {
	// Check if robot is online
	connectionStatus, err := oes.redis.GetConnectionStatus(serialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, fmt.Errorf("robot %s is not online", serialNumber)
	}

	// Create order execution record
	execution := &models.OrderExecution{
		OrderID:       orderData.OrderID,
		SerialNumber:  serialNumber,
		OrderUpdateID: orderData.OrderUpdateID,
		Status:        "CREATED",
	}

	if err := oes.db.DB.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	// Send order via MQTT
	if err := oes.mqttClient.SendOrder(serialNumber, orderData); err != nil {
		// Update status to failed
		oes.updateOrderStatus(orderData.OrderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	// Update status to sent
	now := time.Now()
	execution.Status = "SENT"
	execution.StartedAt = &now
	oes.db.DB.Save(execution)

	log.Printf("Direct order %s executed successfully for robot %s", orderData.OrderID, serialNumber)

	return &models.OrderExecutionResponse{
		OrderID:      orderData.OrderID,
		Status:       "SENT",
		SerialNumber: serialNumber,
		CreatedAt:    execution.CreatedAt,
	}, nil
}

// Helper structures - none needed
