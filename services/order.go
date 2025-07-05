package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"

	"gorm.io/gorm"
)

type OrderService struct {
	db         *database.Database
	redis      *redis.RedisClient
	mqttClient *mqtt.Client
}

func NewOrderService(db *database.Database, redisClient *redis.RedisClient, mqttClient *mqtt.Client) *OrderService {
	return &OrderService{
		db:         db,
		redis:      redisClient,
		mqttClient: mqttClient,
	}
}

// Order Template Management

func (os *OrderService) CreateOrderTemplate(req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	template := &models.OrderTemplate{
		Name:        req.Name,
		Description: req.Description,
	}

	// Start transaction
	tx := os.db.DB.Begin()
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

	// Create nodes
	for _, nodeReq := range req.Nodes {
		node := &models.NodeTemplate{
			OrderTemplateID:       template.ID,
			NodeID:                nodeReq.NodeID,
			Description:           nodeReq.Description,
			SequenceID:            nodeReq.SequenceID,
			Released:              nodeReq.Released,
			X:                     nodeReq.Position.X,
			Y:                     nodeReq.Position.Y,
			Theta:                 nodeReq.Position.Theta,
			AllowedDeviationXY:    nodeReq.Position.AllowedDeviationXY,
			AllowedDeviationTheta: nodeReq.Position.AllowedDeviationTheta,
			MapID:                 nodeReq.Position.MapID,
		}

		if err := tx.Create(node).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create node template: %w", err)
		}

		// Create actions for node
		for _, actionReq := range nodeReq.Actions {
			if err := os.createActionTemplate(tx, &actionReq, &node.ID, nil); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create node action: %w", err)
			}
		}
	}

	// Create edges
	for _, edgeReq := range req.Edges {
		edge := &models.EdgeTemplate{
			OrderTemplateID: template.ID,
			EdgeID:          edgeReq.EdgeID,
			SequenceID:      edgeReq.SequenceID,
			Released:        edgeReq.Released,
			StartNodeID:     edgeReq.StartNodeID,
			EndNodeID:       edgeReq.EndNodeID,
		}

		if err := tx.Create(edge).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create edge template: %w", err)
		}

		// Create actions for edge
		for _, actionReq := range edgeReq.Actions {
			if err := os.createActionTemplate(tx, &actionReq, nil, &edge.ID); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create edge action: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch the complete template with preloaded data
	return os.GetOrderTemplate(template.ID)
}

func (os *OrderService) createActionTemplate(tx *gorm.DB, actionReq *models.ActionTemplateRequest,
	nodeID *uint, edgeID *uint) error {

	action := &models.ActionTemplate{
		NodeTemplateID:    nodeID,
		EdgeTemplateID:    edgeID,
		ActionType:        actionReq.ActionType,
		ActionID:          actionReq.ActionID,
		BlockingType:      actionReq.BlockingType,
		ActionDescription: actionReq.ActionDescription,
	}

	if err := tx.Create(action).Error; err != nil {
		return err
	}

	// Create action parameters
	for _, paramReq := range actionReq.Parameters {
		// Convert value to JSON string based on type
		valueStr, err := os.convertValueToString(paramReq.Value, paramReq.ValueType)
		if err != nil {
			return fmt.Errorf("failed to convert parameter value: %w", err)
		}

		param := &models.ActionParameterTemplate{
			ActionTemplateID: action.ID,
			Key:              paramReq.Key,
			Value:            valueStr,
			ValueType:        paramReq.ValueType,
		}

		if err := tx.Create(param).Error; err != nil {
			return err
		}
	}

	return nil
}

func (os *OrderService) convertValueToString(value interface{}, valueType string) (string, error) {
	if value == nil {
		return "", nil
	}

	switch valueType {
	case "string":
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
	case "object", "number", "boolean":
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

func (os *OrderService) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	var template models.OrderTemplate
	err := os.db.DB.Where("id = ?", id).
		Preload("Nodes.Actions.Parameters").
		Preload("Edges.Actions.Parameters").
		First(&template).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	return &template, nil
}

func (os *OrderService) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	var templates []models.OrderTemplate
	query := os.db.DB.Preload("Nodes").Preload("Edges")

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

func (os *OrderService) UpdateOrderTemplate(id uint, req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	// Start transaction
	tx := os.db.DB.Begin()
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

	// Delete existing nodes, edges, and their actions
	if err := os.deleteTemplateContent(tx, id); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete existing template content: %w", err)
	}

	// Set the ID for the template object
	template.ID = id

	// Recreate nodes and edges (similar to CreateOrderTemplate)
	// ... (implementation similar to CreateOrderTemplate)

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return os.GetOrderTemplate(id)
}

func (os *OrderService) deleteTemplateContent(tx *gorm.DB, templateID uint) error {
	// Delete action parameters
	if err := tx.Exec(`
		DELETE FROM action_parameter_templates 
		WHERE action_template_id IN (
			SELECT id FROM action_templates 
			WHERE node_template_id IN (SELECT id FROM node_templates WHERE order_template_id = ?)
			OR edge_template_id IN (SELECT id FROM edge_templates WHERE order_template_id = ?)
		)
	`, templateID, templateID).Error; err != nil {
		return err
	}

	// Delete actions
	if err := tx.Exec(`
		DELETE FROM action_templates 
		WHERE node_template_id IN (SELECT id FROM node_templates WHERE order_template_id = ?)
		OR edge_template_id IN (SELECT id FROM edge_templates WHERE order_template_id = ?)
	`, templateID, templateID).Error; err != nil {
		return err
	}

	// Delete nodes and edges
	if err := tx.Where("order_template_id = ?", templateID).Delete(&models.NodeTemplate{}).Error; err != nil {
		return err
	}
	if err := tx.Where("order_template_id = ?", templateID).Delete(&models.EdgeTemplate{}).Error; err != nil {
		return err
	}

	return nil
}

func (os *OrderService) DeleteOrderTemplate(id uint) error {
	tx := os.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete content first
	if err := os.deleteTemplateContent(tx, id); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete template content: %w", err)
	}

	// Delete template
	if err := tx.Delete(&models.OrderTemplate{}, id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete order template: %w", err)
	}

	return tx.Commit().Error
}

// Order Execution

func (os *OrderService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	// Get order template
	template, err := os.GetOrderTemplate(req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	// Check if robot is online
	connectionStatus, err := os.redis.GetConnectionStatus(req.SerialNumber)
	if err != nil || connectionStatus != "ONLINE" {
		return nil, fmt.Errorf("robot %s is not online", req.SerialNumber)
	}

	// Generate unique order ID
	orderID := os.generateUniqueOrderID()

	// Create order execution record
	execution := &models.OrderExecution{
		OrderID:         orderID,
		OrderTemplateID: &template.ID,
		SerialNumber:    req.SerialNumber,
		OrderUpdateID:   0,
		Status:          "CREATED",
	}

	if err := os.db.DB.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	// Convert template to MQTT order message
	orderMsg, err := os.convertTemplateToOrderMessage(template, orderID, req.SerialNumber, req.ParameterOverrides)
	if err != nil {
		// Update status to failed
		os.updateOrderStatus(orderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to convert template to order: %w", err)
	}

	// Send order via MQTT
	if err := os.mqttClient.SendOrder(req.SerialNumber, orderMsg); err != nil {
		// Update status to failed
		os.updateOrderStatus(orderID, "FAILED", err.Error())
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	// Update status to sent
	now := time.Now()
	execution.Status = "SENT"
	execution.StartedAt = &now
	os.db.DB.Save(execution)

	log.Printf("Order %s executed successfully for robot %s using template %d",
		orderID, req.SerialNumber, req.TemplateID)

	return &models.OrderExecutionResponse{
		OrderID:         orderID,
		Status:          "SENT",
		SerialNumber:    req.SerialNumber,
		OrderTemplateID: &template.ID,
		CreatedAt:       execution.CreatedAt,
	}, nil
}

func (os *OrderService) convertTemplateToOrderMessage(template *models.OrderTemplate,
	orderID, serialNumber string, paramOverrides map[string]interface{}) (*models.OrderMessage, error) {

	// Convert nodes
	nodes := make([]models.Node, len(template.Nodes))
	for i, nodeTemplate := range template.Nodes {
		nodes[i] = nodeTemplate.ToNode()

		// Apply parameter overrides
		if paramOverrides != nil {
			for j := range nodes[i].Actions {
				os.applyParameterOverrides(&nodes[i].Actions[j], paramOverrides)
			}
		}
	}

	// Convert edges
	edges := make([]models.Edge, len(template.Edges))
	for i, edgeTemplate := range template.Edges {
		edges[i] = edgeTemplate.ToEdge()

		// Apply parameter overrides
		if paramOverrides != nil {
			for j := range edges[i].Actions {
				os.applyParameterOverrides(&edges[i].Actions[j], paramOverrides)
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

func (os *OrderService) applyParameterOverrides(action *models.Action, overrides map[string]interface{}) {
	for i, param := range action.ActionParameters {
		if overrideValue, exists := overrides[param.Key]; exists {
			action.ActionParameters[i].Value = overrideValue
		}
	}
}

func (os *OrderService) generateUniqueOrderID() string {
	return fmt.Sprintf("order_%x", time.Now().UnixNano())
}

func (os *OrderService) updateOrderStatus(orderID, status, errorMessage string) {
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

	os.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(update)
}

// Order Monitoring

func (os *OrderService) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	var execution models.OrderExecution
	err := os.db.DB.Where("order_id = ?", orderID).First(&execution).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get order execution: %w", err)
	}
	return &execution, nil
}

func (os *OrderService) ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := os.db.DB.Order("created_at desc")

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

func (os *OrderService) CancelOrder(orderID string) error {
	// This would require sending a cancel command to the robot
	// For now, we'll just update the status
	return os.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ? AND status IN ('CREATED', 'SENT', 'EXECUTING')", orderID).
		Update("status", "CANCELLED").Error
}
