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

// ===================================================================
// BASIC SERVICE METHODS
// ===================================================================

func (bs *BridgeService) GetRobotState(serialNumber string) (*models.StateMessage, error) {
	return bs.redis.GetState(serialNumber)
}

func (bs *BridgeService) GetRobotConnectionHistory(serialNumber string, limit int) ([]models.ConnectionState, error) {
	var connections []models.ConnectionState
	query := bs.db.DB.Where("serial_number = ?", serialNumber).Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&connections).Error
	return connections, err
}

func (bs *BridgeService) GetRobotCapabilities(serialNumber string) (*RobotCapabilities, error) {
	var physicalParams models.PhysicalParameter
	if err := bs.db.DB.Where("serial_number = ?", serialNumber).First(&physicalParams).Error; err != nil {
		return nil, fmt.Errorf("failed to get physical parameters: %w", err)
	}

	var typeSpec models.TypeSpecification
	if err := bs.db.DB.Where("serial_number = ?", serialNumber).First(&typeSpec).Error; err != nil {
		return nil, fmt.Errorf("failed to get type specification: %w", err)
	}

	var actions []models.AgvAction
	if err := bs.db.DB.Where("serial_number = ?", serialNumber).Preload("Parameters").Find(&actions).Error; err != nil {
		return nil, fmt.Errorf("failed to get AGV actions: %w", err)
	}

	return &RobotCapabilities{
		SerialNumber:       serialNumber,
		PhysicalParameters: physicalParams,
		TypeSpecification:  typeSpec,
		AvailableActions:   actions,
	}, nil
}

func (bs *BridgeService) GetRobotManufacturer(serialNumber string) string {
	var connectionState models.ConnectionState
	err := bs.db.DB.Where("serial_number = ?", serialNumber).
		Order("created_at desc").First(&connectionState).Error

	if err == nil && connectionState.Manufacturer != "" {
		return connectionState.Manufacturer
	}
	return "Roboligent" // Default
}

func (bs *BridgeService) SendOrderToRobot(serialNumber string, orderData OrderRequest) error {
	if !bs.redis.IsRobotOnline(serialNumber) {
		return fmt.Errorf("robot %s is not online", serialNumber)
	}

	orderMsg := &models.OrderMessage{
		HeaderID:      1,
		Timestamp:     utils.GetCurrentTimestamp(),
		Version:       "2.0.0",
		Manufacturer:  bs.GetRobotManufacturer(serialNumber),
		SerialNumber:  serialNumber,
		OrderID:       orderData.OrderID,
		OrderUpdateID: orderData.OrderUpdateID,
		Nodes:         orderData.Nodes,
		Edges:         orderData.Edges,
	}

	if err := bs.mqttClient.SendOrder(serialNumber, orderMsg); err != nil {
		return fmt.Errorf("failed to send order: %w", err)
	}

	log.Printf("Order %s sent successfully to robot %s", orderData.OrderID, serialNumber)
	return nil
}

func (bs *BridgeService) SendCustomAction(serialNumber string, actionRequest CustomActionRequest) error {
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

	log.Printf("Custom action sent successfully to robot %s", serialNumber)
	return nil
}

func (bs *BridgeService) GetConnectedRobots() ([]string, error) {
	var robots []string
	var connections []models.ConnectionState

	err := bs.db.DB.Select("DISTINCT ON (serial_number) serial_number").
		Where("connection_state = ?", "ONLINE").
		Order("serial_number, created_at DESC").
		Find(&connections).Error

	if err != nil {
		return nil, err
	}

	for _, conn := range connections {
		if bs.redis.IsRobotOnline(conn.SerialNumber) {
			robots = append(robots, conn.SerialNumber)
		}
	}

	return robots, nil
}

func (bs *BridgeService) MonitorRobotHealth(serialNumber string) (*RobotHealthStatus, error) {
	state, err := bs.GetRobotState(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get robot state for health check: %w", err)
	}

	return &RobotHealthStatus{
		SerialNumber:        serialNumber,
		IsOnline:            bs.redis.IsRobotOnline(serialNumber),
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
// ENHANCED ORDER CREATION (SIMPLIFIED)
// ===================================================================

func (bs *BridgeService) CreateInferenceOrder(serialNumber, inferenceName string) error {
	return bs.createEnhancedOrder(serialNumber, "inference", map[string]interface{}{
		"inferenceName": inferenceName,
	})
}

func (bs *BridgeService) CreateInferenceOrderWithPosition(serialNumber, inferenceName string, position models.NodePosition) error {
	return bs.createEnhancedOrder(serialNumber, "inference", map[string]interface{}{
		"inferenceName": inferenceName,
		"position":      position,
	})
}

func (bs *BridgeService) CreateCustomInferenceOrder(req *CustomInferenceOrderRequest) error {
	return bs.createEnhancedOrder(req.SerialNumber, "inference", map[string]interface{}{
		"inferenceName":     req.InferenceName,
		"position":          req.Position,
		"customParameters":  req.CustomParameters,
		"actionType":        req.ActionType,
		"actionDescription": req.ActionDescription,
		"blockingType":      req.BlockingType,
		"description":       req.Description,
		"sequenceId":        req.SequenceID,
		"released":          req.Released,
		"edges":             req.Edges,
	})
}

func (bs *BridgeService) CreateTrajectoryOrder(serialNumber, trajectoryName, arm string) error {
	return bs.createEnhancedOrder(serialNumber, "trajectory", map[string]interface{}{
		"trajectoryName": trajectoryName,
		"arm":            arm,
	})
}

func (bs *BridgeService) CreateTrajectoryOrderWithPosition(serialNumber, trajectoryName, arm string, position models.NodePosition) error {
	return bs.createEnhancedOrder(serialNumber, "trajectory", map[string]interface{}{
		"trajectoryName": trajectoryName,
		"arm":            arm,
		"position":       position,
	})
}

func (bs *BridgeService) CreateCustomTrajectoryOrder(req *CustomTrajectoryOrderRequest) error {
	return bs.createEnhancedOrder(req.SerialNumber, "trajectory", map[string]interface{}{
		"trajectoryName":    req.TrajectoryName,
		"arm":               req.Arm,
		"position":          req.Position,
		"customParameters":  req.CustomParameters,
		"actionType":        req.ActionType,
		"actionDescription": req.ActionDescription,
		"blockingType":      req.BlockingType,
		"description":       req.Description,
		"sequenceId":        req.SequenceID,
		"released":          req.Released,
		"edges":             req.Edges,
	})
}

func (bs *BridgeService) CreateDynamicOrder(req *DynamicOrderRequest) error {
	if !bs.redis.IsRobotOnline(req.SerialNumber) {
		return fmt.Errorf("robot %s is not online", req.SerialNumber)
	}

	orderID := utils.GenerateOrderID()

	// Create order execution record
	execution := &models.OrderExecution{
		OrderID:       orderID,
		SerialNumber:  req.SerialNumber,
		OrderUpdateID: req.OrderUpdateID,
		Status:        "CREATED",
	}

	if err := bs.db.DB.Create(execution).Error; err != nil {
		return fmt.Errorf("failed to create order execution record: %w", err)
	}

	// Process nodes and edges with IDs
	nodes := utils.ProcessNodesWithIDs(req.Nodes)
	edges := utils.ProcessEdgesWithIDs(req.Edges)

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
		bs.updateOrderStatus(orderID, "FAILED", err.Error())
		return fmt.Errorf("failed to send order: %w", err)
	}

	bs.updateOrderStatus(orderID, "SENT", "")
	log.Printf("Dynamic order %s with %d nodes and %d edges sent to robot %s",
		orderID, len(nodes), len(edges), req.SerialNumber)

	return nil
}

// ===================================================================
// PRIVATE HELPER METHODS
// ===================================================================

func (bs *BridgeService) createEnhancedOrder(serialNumber, orderType string, params map[string]interface{}) error {
	if !bs.redis.IsRobotOnline(serialNumber) {
		return fmt.Errorf("robot %s is not online", serialNumber)
	}

	orderID := utils.GenerateOrderID()
	nodeID := utils.GenerateNodeID()
	actionID := utils.GenerateActionID()

	// Get or create default values
	position := bs.getPositionFromParams(params)
	actionType := bs.getActionTypeFromParams(orderType, params)
	actionDescription := bs.getActionDescriptionFromParams(orderType, params)
	blockingType := utils.GetValueOrDefault(bs.getStringFromParams(params, "blockingType"), "NONE")
	description := utils.GetValueOrDefault(bs.getStringFromParams(params, "description"),
		fmt.Sprintf("%s Task", utils.GetValueOrDefault(orderType, "Unknown")))

	// Build action parameters
	actionParams := bs.buildActionParameters(orderType, params)

	// Get additional parameters
	sequenceID := bs.getIntFromParams(params, "sequenceId")
	released := bs.getBoolFromParams(params, "released", true)
	edges := bs.getEdgesFromParams(params)

	// Create order data
	orderData := OrderRequest{
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes: []models.Node{
			{
				NodeID:       nodeID,
				Description:  description,
				SequenceID:   sequenceID,
				Released:     released,
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
		Edges: edges,
	}

	return bs.SendOrderToRobot(serialNumber, orderData)
}

func (bs *BridgeService) getPositionFromParams(params map[string]interface{}) models.NodePosition {
	if pos, ok := params["position"].(models.NodePosition); ok {
		return pos
	}
	return utils.GetDefaultNodePosition()
}

func (bs *BridgeService) getActionTypeFromParams(orderType string, params map[string]interface{}) string {
	if actionType := bs.getStringFromParams(params, "actionType"); actionType != "" {
		return actionType
	}

	switch orderType {
	case "inference":
		return "Roboligent Robin - Inference"
	case "trajectory":
		return "Roboligent Robin - Follow Trajectory"
	default:
		return "Unknown Action"
	}
}

func (bs *BridgeService) getActionDescriptionFromParams(orderType string, params map[string]interface{}) string {
	if desc := bs.getStringFromParams(params, "actionDescription"); desc != "" {
		return desc
	}

	switch orderType {
	case "inference":
		return "This is an action will trigger the behavior tree for executing inference."
	case "trajectory":
		return "This action will trigger the behavior tree for following a recorded trajectory."
	default:
		return "Unknown action description"
	}
}

func (bs *BridgeService) buildActionParameters(orderType string, params map[string]interface{}) []models.ActionParameter {
	var actionParams []models.ActionParameter

	// Add specific parameters based on order type
	switch orderType {
	case "inference":
		if inferenceName := bs.getStringFromParams(params, "inferenceName"); inferenceName != "" {
			actionParams = append(actionParams, models.ActionParameter{
				Key: "inference_name", Value: inferenceName,
			})
		}
	case "trajectory":
		if trajectoryName := bs.getStringFromParams(params, "trajectoryName"); trajectoryName != "" {
			actionParams = append(actionParams, models.ActionParameter{
				Key: "trajectory_name", Value: trajectoryName,
			})
		}
		if arm := bs.getStringFromParams(params, "arm"); arm != "" {
			actionParams = append(actionParams, models.ActionParameter{
				Key: "arm", Value: arm,
			})
		}
	}

	// Add custom parameters
	if customParams, ok := params["customParameters"].(map[string]interface{}); ok {
		actionParams = utils.AddCustomParameters(actionParams, customParams)
	}

	return actionParams
}

func (bs *BridgeService) getStringFromParams(params map[string]interface{}, key string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return ""
}

func (bs *BridgeService) getIntFromParams(params map[string]interface{}, key string) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	return 0
}

func (bs *BridgeService) getBoolFromParams(params map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultVal
}

func (bs *BridgeService) getEdgesFromParams(params map[string]interface{}) []models.Edge {
	if edges, ok := params["edges"].([]models.Edge); ok {
		return edges
	}
	return []models.Edge{}
}

func (bs *BridgeService) updateOrderStatus(orderID, status, errorMessage string) {
	updateFields := utils.CreateUpdateFields(map[string]interface{}{
		"status": status,
	})

	if errorMessage != "" {
		updateFields["error_message"] = errorMessage
	}

	updateFields = updateFields.AddCompletionFields(status)

	result := bs.db.DB.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(updateFields)

	if result.Error != nil {
		log.Printf("Failed to update order status: %v", result.Error)
		return
	}

	log.Printf("Order %s status updated to %s", orderID, status)
}

// ===================================================================
// TYPE DEFINITIONS
// ===================================================================

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
