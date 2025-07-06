package services

import (
	"fmt"
	"log"
	"time"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
	"mqtt-bridge/utils"
)

type BridgeService struct {
	db         *database.Database
	redis      *redis.RedisClient
	mqttClient *mqtt.Client
}

func NewBridgeService(db *database.Database, redisClient *redis.RedisClient, mqttClient *mqtt.Client) *BridgeService {
	return &BridgeService{
		db:         db,
		redis:      redisClient,
		mqttClient: mqttClient,
	}
}

// Data structures for service responses
type RobotCapabilities struct {
	SerialNumber       string                   `json:"serialNumber"`
	PhysicalParameters models.PhysicalParameter `json:"physicalParameters"`
	TypeSpecification  models.TypeSpecification `json:"typeSpecification"`
	AvailableActions   []models.AgvAction       `json:"availableActions"`
}

type OrderRequest struct {
	OrderID       string        `json:"orderId"`
	OrderUpdateID int           `json:"orderUpdateId"`
	Nodes         []models.Node `json:"nodes"`
	Edges         []models.Edge `json:"edges"`
}

type CustomActionRequest struct {
	HeaderID int             `json:"headerId"`
	Actions  []models.Action `json:"actions"`
}

type RobotHealthStatus struct {
	SerialNumber        string    `json:"serialNumber"`
	IsOnline            bool      `json:"isOnline"`
	BatteryCharge       float64   `json:"batteryCharge"`
	BatteryVoltage      float64   `json:"batteryVoltage"`
	IsCharging          bool      `json:"isCharging"`
	PositionInitialized bool      `json:"positionInitialized"`
	HasErrors           bool      `json:"hasErrors"`
	ErrorCount          int       `json:"errorCount"`
	OperatingMode       string    `json:"operatingMode"`
	IsPaused            bool      `json:"isPaused"`
	IsDriving           bool      `json:"isDriving"`
	LastUpdate          time.Time `json:"lastUpdate"`
}

// Unified request structure for Enhanced APIs
type EnhancedOrderRequest struct {
	SerialNumber      string                 `json:"serialNumber"`
	OrderType         string                 `json:"orderType"` // "inference", "trajectory", "dynamic"
	InferenceName     string                 `json:"inferenceName,omitempty"`
	TrajectoryName    string                 `json:"trajectoryName,omitempty"`
	Arm               string                 `json:"arm,omitempty"`
	Description       string                 `json:"description,omitempty"`
	SequenceID        int                    `json:"sequenceId,omitempty"`
	Released          bool                   `json:"released"`
	Position          *models.NodePosition   `json:"position,omitempty"`
	ActionType        string                 `json:"actionType,omitempty"`
	ActionDescription string                 `json:"actionDescription,omitempty"`
	BlockingType      string                 `json:"blockingType,omitempty"`
	CustomParameters  map[string]interface{} `json:"customParameters,omitempty"`
	OrderUpdateID     int                    `json:"orderUpdateId,omitempty"`
	Nodes             []models.Node          `json:"nodes,omitempty"`
	Edges             []models.Edge          `json:"edges,omitempty"`
}

// Legacy type definitions for backward compatibility
type CustomInferenceOrderRequest struct {
	SerialNumber      string                 `json:"serialNumber"`
	InferenceName     string                 `json:"inferenceName"`
	Description       string                 `json:"description"`
	SequenceID        int                    `json:"sequenceId"`
	Released          bool                   `json:"released"`
	Position          models.NodePosition    `json:"position"`
	ActionType        string                 `json:"actionType"`
	ActionDescription string                 `json:"actionDescription"`
	BlockingType      string                 `json:"blockingType"`
	CustomParameters  map[string]interface{} `json:"customParameters"`
	Edges             []models.Edge          `json:"edges"`
}

type CustomTrajectoryOrderRequest struct {
	SerialNumber      string                 `json:"serialNumber"`
	TrajectoryName    string                 `json:"trajectoryName"`
	Arm               string                 `json:"arm"`
	Description       string                 `json:"description"`
	SequenceID        int                    `json:"sequenceId"`
	Released          bool                   `json:"released"`
	Position          models.NodePosition    `json:"position"`
	ActionType        string                 `json:"actionType"`
	ActionDescription string                 `json:"actionDescription"`
	BlockingType      string                 `json:"blockingType"`
	CustomParameters  map[string]interface{} `json:"customParameters"`
	Edges             []models.Edge          `json:"edges"`
}

type DynamicOrderRequest struct {
	SerialNumber  string        `json:"serialNumber"`
	OrderUpdateID int           `json:"orderUpdateId"`
	Nodes         []models.Node `json:"nodes"`
	Edges         []models.Edge `json:"edges"`
}

// ===================================================================
// BASIC SERVICE METHODS
// ===================================================================

// GetRobotState retrieves the current state of a robot from Redis
func (bs *BridgeService) GetRobotState(serialNumber string) (*models.StateMessage, error) {
	state, err := bs.redis.GetState(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get robot state: %w", err)
	}
	return state, nil
}

// GetRobotConnectionHistory retrieves connection history from database
func (bs *BridgeService) GetRobotConnectionHistory(serialNumber string, limit int) ([]models.ConnectionState, error) {
	var connections []models.ConnectionState

	query := bs.db.DB.Where("serial_number = ?", serialNumber).
		Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&connections).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get connection history: %w", err)
	}

	return connections, nil
}

// GetRobotCapabilities retrieves robot capabilities from database
func (bs *BridgeService) GetRobotCapabilities(serialNumber string) (*RobotCapabilities, error) {
	var physicalParams models.PhysicalParameter
	err := bs.db.DB.Where("serial_number = ?", serialNumber).First(&physicalParams).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get physical parameters: %w", err)
	}

	var typeSpec models.TypeSpecification
	err = bs.db.DB.Where("serial_number = ?", serialNumber).First(&typeSpec).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get type specification: %w", err)
	}

	var actions []models.AgvAction
	err = bs.db.DB.Where("serial_number = ?", serialNumber).
		Preload("Parameters").Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get AGV actions: %w", err)
	}

	return &RobotCapabilities{
		SerialNumber:       serialNumber,
		PhysicalParameters: physicalParams,
		TypeSpecification:  typeSpec,
		AvailableActions:   actions,
	}, nil
}

// GetRobotManufacturer retrieves the manufacturer from the database or defaults to "Roboligent"
func (bs *BridgeService) GetRobotManufacturer(serialNumber string) string {
	var connectionState models.ConnectionState
	err := bs.db.DB.Where("serial_number = ?", serialNumber).
		Order("created_at desc").
		First(&connectionState).Error

	if err == nil && connectionState.Manufacturer != "" {
		return connectionState.Manufacturer
	}

	return "Roboligent" // Default manufacturer
}

// SendOrderToRobot sends an order message to the specified robot
func (bs *BridgeService) SendOrderToRobot(serialNumber string, orderData OrderRequest) error {
	// Check if robot is online
	if !bs.redis.IsRobotOnline(serialNumber) {
		return fmt.Errorf("robot %s is not online", serialNumber)
	}

	// Create order message
	orderMsg := &models.OrderMessage{
		HeaderID:      1, // This should be managed per robot
		Timestamp:     utils.GetCurrentTimestamp(),
		Version:       "2.0.0",
		Manufacturer:  bs.GetRobotManufacturer(serialNumber),
		SerialNumber:  serialNumber,
		OrderID:       orderData.OrderID,
		OrderUpdateID: orderData.OrderUpdateID,
		Nodes:         orderData.Nodes,
		Edges:         orderData.Edges,
	}

	// Send via MQTT
	err := bs.mqttClient.SendOrder(serialNumber, orderMsg)
	if err != nil {
		return fmt.Errorf("failed to send order: %w", err)
	}

	logInfo := utils.CreateLogInfo(utils.LogOpExecute, "Order", orderData.OrderID,
		fmt.Sprintf("Order sent successfully to robot %s", serialNumber))
	log.Printf(logInfo.FormatLogMessage())

	return nil
}

// SendCustomAction sends a custom action to the robot
func (bs *BridgeService) SendCustomAction(serialNumber string, actionRequest CustomActionRequest) error {
	// Check if robot is online
	if !bs.redis.IsRobotOnline(serialNumber) {
		return fmt.Errorf("robot %s is not online", serialNumber)
	}

	actionMsg := &models.InstantActionMessage{
		HeaderID:     actionRequest.HeaderID,
		Timestamp:    utils.GetCurrentTimestamp(),
		Version:      "2.0.0",
		Manufacturer: bs.GetRobotManufacturer(serialNumber),
		SerialNumber: serialNumber,
		Actions:      actionRequest.Actions,
	}

	err := bs.mqttClient.SendCustomAction(serialNumber, bs.GetRobotManufacturer(serialNumber), actionMsg)
	if err != nil {
		return fmt.Errorf("failed to send custom action: %w", err)
	}

	logInfo := utils.CreateLogInfo(utils.LogOpExecute, "CustomAction", utils.GenerateActionID(),
		fmt.Sprintf("Custom action sent successfully to robot %s", serialNumber))
	log.Printf(logInfo.FormatLogMessage())

	return nil
}

// GetConnectedRobots returns a list of currently connected robots
func (bs *BridgeService) GetConnectedRobots() ([]string, error) {
	var robots []string
	var connections []models.ConnectionState

	err := bs.db.DB.Select("DISTINCT ON (serial_number) serial_number, connection_state").
		Where("connection_state = ?", utils.ConnectionStateOnline).
		Order("serial_number, created_at DESC").
		Find(&connections).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get connected robots: %w", err)
	}

	for _, conn := range connections {
		if bs.redis.IsRobotOnline(conn.SerialNumber) {
			robots = append(robots, conn.SerialNumber)
		}
	}

	return robots, nil
}

// MonitorRobotHealth checks robot health status
func (bs *BridgeService) MonitorRobotHealth(serialNumber string) (*RobotHealthStatus, error) {
	state, err := bs.GetRobotState(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get robot state for health check: %w", err)
	}

	isOnline := bs.redis.IsRobotOnline(serialNumber)

	return &RobotHealthStatus{
		SerialNumber:        serialNumber,
		IsOnline:            isOnline,
		BatteryCharge:       state.BatteryState.BatteryCharge,
		BatteryVoltage:      state.BatteryState.BatteryVoltage,
		IsCharging:          state.BatteryState.Charging,
		PositionInitialized: state.AgvPosition.PositionInitialized,
		HasErrors:           len(state.Errors) > 0,
		ErrorCount:          len(state.Errors),
		OperatingMode:       state.OperatingMode,
		IsPaused:            state.Paused,
		IsDriving:           state.Driving,
		LastUpdate:          time.Now(),
	}, nil
}

// ===================================================================
// UNIFIED ENHANCED ORDER CREATION METHODS
// ===================================================================

// CreateEnhancedOrder - Unified method for all enhanced order types
func (bs *BridgeService) CreateEnhancedOrder(req *EnhancedOrderRequest) error {
	// Validate required fields
	if err := bs.validateEnhancedOrderRequest(req); err != nil {
		return err
	}

	switch req.OrderType {
	case "inference":
		return bs.createInferenceOrder(req)
	case "trajectory":
		return bs.createTrajectoryOrder(req)
	case "dynamic":
		return bs.createDynamicOrder(req)
	default:
		return fmt.Errorf("unsupported order type: %s", req.OrderType)
	}
}

// Basic convenience methods that use the unified approach
func (bs *BridgeService) CreateInferenceOrder(serialNumber, inferenceName string) error {
	req := &EnhancedOrderRequest{
		SerialNumber:  serialNumber,
		OrderType:     "inference",
		InferenceName: inferenceName,
		Released:      true,
	}
	return bs.CreateEnhancedOrder(req)
}

func (bs *BridgeService) CreateInferenceOrderWithPosition(serialNumber, inferenceName string, position models.NodePosition) error {
	req := &EnhancedOrderRequest{
		SerialNumber:  serialNumber,
		OrderType:     "inference",
		InferenceName: inferenceName,
		Position:      &position,
		Released:      true,
	}
	return bs.CreateEnhancedOrder(req)
}

func (bs *BridgeService) CreateCustomInferenceOrder(customReq *CustomInferenceOrderRequest) error {
	req := &EnhancedOrderRequest{
		SerialNumber:      customReq.SerialNumber,
		OrderType:         "inference",
		InferenceName:     customReq.InferenceName,
		Description:       customReq.Description,
		SequenceID:        customReq.SequenceID,
		Released:          customReq.Released,
		Position:          &customReq.Position,
		ActionType:        customReq.ActionType,
		ActionDescription: customReq.ActionDescription,
		BlockingType:      customReq.BlockingType,
		CustomParameters:  customReq.CustomParameters,
		Edges:             customReq.Edges,
	}
	return bs.CreateEnhancedOrder(req)
}

func (bs *BridgeService) CreateTrajectoryOrder(serialNumber, trajectoryName, arm string) error {
	req := &EnhancedOrderRequest{
		SerialNumber:   serialNumber,
		OrderType:      "trajectory",
		TrajectoryName: trajectoryName,
		Arm:            arm,
		Released:       true,
	}
	return bs.CreateEnhancedOrder(req)
}

func (bs *BridgeService) CreateTrajectoryOrderWithPosition(serialNumber, trajectoryName, arm string, position models.NodePosition) error {
	req := &EnhancedOrderRequest{
		SerialNumber:   serialNumber,
		OrderType:      "trajectory",
		TrajectoryName: trajectoryName,
		Arm:            arm,
		Position:       &position,
		Released:       true,
	}
	return bs.CreateEnhancedOrder(req)
}

func (bs *BridgeService) CreateCustomTrajectoryOrder(customReq *CustomTrajectoryOrderRequest) error {
	req := &EnhancedOrderRequest{
		SerialNumber:      customReq.SerialNumber,
		OrderType:         "trajectory",
		TrajectoryName:    customReq.TrajectoryName,
		Arm:               customReq.Arm,
		Description:       customReq.Description,
		SequenceID:        customReq.SequenceID,
		Released:          customReq.Released,
		Position:          &customReq.Position,
		ActionType:        customReq.ActionType,
		ActionDescription: customReq.ActionDescription,
		BlockingType:      customReq.BlockingType,
		CustomParameters:  customReq.CustomParameters,
		Edges:             customReq.Edges,
	}
	return bs.CreateEnhancedOrder(req)
}

func (bs *BridgeService) CreateDynamicOrder(dynamicReq *DynamicOrderRequest) error {
	req := &EnhancedOrderRequest{
		SerialNumber:  dynamicReq.SerialNumber,
		OrderType:     "dynamic",
		OrderUpdateID: dynamicReq.OrderUpdateID,
		Nodes:         dynamicReq.Nodes,
		Edges:         dynamicReq.Edges,
	}
	return bs.CreateEnhancedOrder(req)
}

// ===================================================================
// PRIVATE IMPLEMENTATION METHODS
// ===================================================================

// validateEnhancedOrderRequest validates the enhanced order request
func (bs *BridgeService) validateEnhancedOrderRequest(req *EnhancedOrderRequest) error {
	// Basic validation
	if req.SerialNumber == "" {
		return fmt.Errorf("serialNumber is required")
	}
	if req.OrderType == "" {
		return fmt.Errorf("orderType is required")
	}

	// Type-specific validation
	switch req.OrderType {
	case "inference":
		if req.InferenceName == "" {
			return fmt.Errorf("inferenceName is required for inference orders")
		}
	case "trajectory":
		if req.TrajectoryName == "" {
			return fmt.Errorf("trajectoryName is required for trajectory orders")
		}
		if req.Arm == "" {
			return fmt.Errorf("arm is required for trajectory orders")
		}
	case "dynamic":
		if len(req.Nodes) == 0 {
			return fmt.Errorf("at least one node is required for dynamic orders")
		}
	}

	// Check robot online status
	return bs.validateRobotOnline(req.SerialNumber)
}

// validateRobotOnline checks if robot is online
func (bs *BridgeService) validateRobotOnline(serialNumber string) error {
	connectionStatus, err := bs.redis.GetConnectionStatus(serialNumber)
	if err != nil || connectionStatus != string(utils.ConnectionStateOnline) {
		return fmt.Errorf("robot %s is not online", serialNumber)
	}
	return nil
}

// createInferenceOrder handles inference order creation
func (bs *BridgeService) createInferenceOrder(req *EnhancedOrderRequest) error {
	// Generate IDs using common helpers
	orderID := utils.GenerateOrderID()
	nodeID := utils.GenerateNodeID()
	actionID := utils.GenerateActionID()

	// Set defaults using common helpers
	position := utils.GetPositionOrDefault(req.Position)
	actionType := utils.GetValueOrDefault(req.ActionType, "Roboligent Robin - Inference")
	actionDescription := utils.GetValueOrDefault(req.ActionDescription, "This is an action will trigger the behavior tree for executing inference.")
	blockingType := utils.GetValueOrDefault(req.BlockingType, "NONE")

	// Build action parameters using common helpers
	actionParams := []models.ActionParameter{
		{Key: "inference_name", Value: req.InferenceName},
	}
	actionParams = utils.AddCustomParameters(actionParams, req.CustomParameters)

	// Create order data
	orderData := OrderRequest{
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes: []models.Node{
			{
				NodeID:       nodeID,
				Description:  utils.GetValueOrDefault(req.Description, "Inference Task"),
				SequenceID:   req.SequenceID,
				Released:     req.Released,
				NodePosition: position,
				Actions: []models.Action{
					{
						ActionType:        actionType,
						ActionID:          actionID,
						ActionDescription: actionDescription,
						BlockingType:      blockingType,
						ActionParameters:  actionParams,
					},
				},
			},
		},
		Edges: req.Edges,
	}

	// Log operation using common helpers
	logInfo := utils.CreateLogInfo(utils.LogOpExecute, "InferenceOrder", orderID,
		fmt.Sprintf("Inference order created for robot %s", req.SerialNumber))
	log.Printf(logInfo.FormatLogMessage())

	return bs.SendOrderToRobot(req.SerialNumber, orderData)
}

// createTrajectoryOrder handles trajectory order creation
func (bs *BridgeService) createTrajectoryOrder(req *EnhancedOrderRequest) error {
	// Generate IDs using common helpers
	orderID := utils.GenerateOrderID()
	nodeID := utils.GenerateNodeID()
	actionID := utils.GenerateActionID()

	// Set defaults using common helpers
	position := utils.GetPositionOrDefault(req.Position)
	actionType := utils.GetValueOrDefault(req.ActionType, "Roboligent Robin - Follow Trajectory")
	actionDescription := utils.GetValueOrDefault(req.ActionDescription, "This action will trigger the behavior tree for following a recorded trajectory.")
	blockingType := utils.GetValueOrDefault(req.BlockingType, "NONE")

	// Build action parameters using common helpers
	actionParams := []models.ActionParameter{
		{Key: "trajectory_name", Value: req.TrajectoryName},
		{Key: "arm", Value: req.Arm},
	}
	actionParams = utils.AddCustomParameters(actionParams, req.CustomParameters)

	// Create order data
	orderData := OrderRequest{
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes: []models.Node{
			{
				NodeID:       nodeID,
				Description:  utils.GetValueOrDefault(req.Description, "Trajectory Task"),
				SequenceID:   req.SequenceID,
				Released:     req.Released,
				NodePosition: position,
				Actions: []models.Action{
					{
						ActionType:        actionType,
						ActionID:          actionID,
						ActionDescription: actionDescription,
						BlockingType:      blockingType,
						ActionParameters:  actionParams,
					},
				},
			},
		},
		Edges: req.Edges,
	}

	// Log operation using common helpers
	logInfo := utils.CreateLogInfo(utils.LogOpExecute, "TrajectoryOrder", orderID,
		fmt.Sprintf("Trajectory order created for robot %s", req.SerialNumber))
	log.Printf(logInfo.FormatLogMessage())

	return bs.SendOrderToRobot(req.SerialNumber, orderData)
}

// createDynamicOrder handles dynamic order creation
func (bs *BridgeService) createDynamicOrder(req *EnhancedOrderRequest) error {
	// Generate unique order ID using common helpers
	orderID := utils.GenerateOrderID()

	// Process nodes and edges with IDs using common helpers
	nodes := utils.ProcessNodesWithIDs(req.Nodes)
	edges := utils.ProcessEdgesWithIDs(req.Edges)

	// Create order execution record
	execution := &models.OrderExecution{
		OrderID:       orderID,
		SerialNumber:  req.SerialNumber,
		OrderUpdateID: req.OrderUpdateID,
		Status:        string(utils.OrderStatusCreated),
	}

	if err := bs.db.DB.Create(execution).Error; err != nil {
		return fmt.Errorf("failed to create order execution record: %w", err)
	}

	// Create and send order message
	orderMsg := &models.OrderMessage{
		HeaderID:      1,
		Timestamp:     utils.GetCurrentTimestamp(),
		Version:       "2.0.0",
		Manufacturer:  bs.GetRobotManufacturer(req.SerialNumber),
		SerialNumber:  req.SerialNumber,
		OrderID:       orderID,
		OrderUpdateID: req.OrderUpdateID,
		Nodes:         nodes,
		Edges:         edges,
	}

	if err := bs.mqttClient.SendOrder(req.SerialNumber, orderMsg); err != nil {
		bs.updateOrderStatus(orderID, string(utils.OrderStatusFailed), err.Error())
		return fmt.Errorf("failed to send order: %w", err)
	}

	// Update status to sent using common helpers
	bs.updateOrderStatus(orderID, string(utils.OrderStatusSent), "")

	// Log operation using common helpers
	logInfo := utils.CreateLogInfo(utils.LogOpExecute, "DynamicOrder", orderID,
		fmt.Sprintf("Dynamic order with %d nodes and %d edges created for robot %s",
			len(nodes), len(edges), req.SerialNumber))
	log.Printf(logInfo.FormatLogMessage())

	return nil
}

// updateOrderStatus updates order execution status using common helpers
func (bs *BridgeService) updateOrderStatus(orderID, status, errorMessage string) {
	// Validate status using common helpers
	if !utils.IsValidOrderStatus(status) {
		log.Printf("Invalid order status: %s", status)
		return
	}

	// Create update fields using common helpers
	updateFields := utils.CreateUpdateFields(map[string]interface{}{
		"status": status,
	})

	if errorMessage != "" {
		updateFields["error_message"] = errorMessage
	}

	// Add completion fields if needed
	updateFields = updateFields.AddCompletionFields(status)

	// Update in database
	result := bs.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(updateFields)

	if result.Error != nil {
		log.Printf("Failed to update order status: %v", result.Error)
		return
	}

	// Log status update using common helpers
	logInfo := utils.CreateLogInfo(utils.LogOpUpdate, "OrderExecution", orderID,
		fmt.Sprintf("Status updated to %s", status))
	log.Printf(logInfo.FormatLogMessage())
}
