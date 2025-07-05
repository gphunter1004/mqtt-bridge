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

	log.Printf("[Order Service] Order template created successfully: %s (ID: %d)", template.Name, template.ID)

	// Fetch the complete template with associations
	return os.GetOrderTemplate(template.ID)
}

func (os *OrderService) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	var template models.OrderTemplate
	err := os.db.DB.Where("id = ?", id).First(&template).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get order template: %w", err)
	}

	return &template, nil
}

func (os *OrderService) GetOrderTemplateWithDetails(id uint) (*OrderTemplateWithDetails, error) {
	template, err := os.GetOrderTemplate(id)
	if err != nil {
		return nil, err
	}

	// Get associated nodes
	var nodeAssociations []models.OrderTemplateNode
	err = os.db.DB.Where("order_template_id = ?", id).
		Preload("NodeTemplate.Actions.Parameters").
		Find(&nodeAssociations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get template nodes: %w", err)
	}

	// Get associated edges
	var edgeAssociations []models.OrderTemplateEdge
	err = os.db.DB.Where("order_template_id = ?", id).
		Preload("EdgeTemplate.Actions.Parameters").
		Find(&edgeAssociations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get template edges: %w", err)
	}

	// Extract nodes and edges
	nodes := make([]models.NodeTemplate, len(nodeAssociations))
	for i, assoc := range nodeAssociations {
		nodes[i] = assoc.NodeTemplate
	}

	edges := make([]models.EdgeTemplate, len(edgeAssociations))
	for i, assoc := range edgeAssociations {
		edges[i] = assoc.EdgeTemplate
	}

	result := &OrderTemplateWithDetails{
		OrderTemplate: *template,
		Nodes:         nodes,
		Edges:         edges,
	}

	return result, nil
}

func (os *OrderService) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	var templates []models.OrderTemplate
	query := os.db.DB.Order("created_at DESC")

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

	return os.GetOrderTemplate(id)
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

// Order Execution

func (os *OrderService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	// Get order template with details
	templateDetails, err := os.GetOrderTemplateWithDetails(req.TemplateID)
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
		OrderTemplateID: &templateDetails.OrderTemplate.ID,
		SerialNumber:    req.SerialNumber,
		OrderUpdateID:   0,
		Status:          "CREATED",
	}

	if err := os.db.DB.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create order execution record: %w", err)
	}

	// Convert template to MQTT order message
	orderMsg, err := os.convertTemplateToOrderMessage(templateDetails, orderID, req.SerialNumber, req.ParameterOverrides)
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
		OrderTemplateID: &templateDetails.OrderTemplate.ID,
		CreatedAt:       execution.CreatedAt,
	}, nil
}

func (os *OrderService) convertTemplateToOrderMessage(templateDetails *OrderTemplateWithDetails,
	orderID, serialNumber string, paramOverrides map[string]interface{}) (*models.OrderMessage, error) {

	// Convert nodes
	nodes := make([]models.Node, len(templateDetails.Nodes))
	for i, nodeTemplate := range templateDetails.Nodes {
		nodes[i] = nodeTemplate.ToNode()

		// Apply parameter overrides
		if paramOverrides != nil {
			for j := range nodes[i].Actions {
				os.applyParameterOverrides(&nodes[i].Actions[j], paramOverrides)
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

// Template Association Management

func (os *OrderService) AssociateNodes(templateID uint, req *models.AssociateNodesRequest) error {
	tx := os.db.DB.Begin()
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

func (os *OrderService) AssociateEdges(templateID uint, req *models.AssociateEdgesRequest) error {
	tx := os.db.DB.Begin()
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

// Helper structures
type OrderTemplateWithDetails struct {
	OrderTemplate models.OrderTemplate  `json:"orderTemplate"`
	Nodes         []models.NodeTemplate `json:"nodes"`
	Edges         []models.EdgeTemplate `json:"edges"`
}
