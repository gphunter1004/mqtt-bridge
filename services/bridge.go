package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"mqtt-bridge/database"
	"mqtt-bridge/message"
	"mqtt-bridge/models"
	"mqtt-bridge/redis"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/transport"
	"mqtt-bridge/utils"
)

// BridgeService acts as a facade, coordinating high-level operations.
type BridgeService struct {
	connectionRepo     interfaces.ConnectionRepositoryInterface
	factsheetRepo      interfaces.FactsheetRepositoryInterface
	orderExecutionRepo interfaces.OrderExecutionRepositoryInterface
	redis              *redis.RedisClient
	messageService     *MessageService
	uow                database.UnitOfWorkInterface
	logger             *slog.Logger
}

// Custom request types for enhanced order creation.
type (
	CustomInferenceOrderRequest struct {
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

	CustomTrajectoryOrderRequest struct {
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

	DynamicOrderRequest struct {
		SerialNumber  string        `json:"serialNumber"`
		OrderUpdateID int           `json:"orderUpdateId"`
		Nodes         []models.Node `json:"nodes"`
		Edges         []models.Edge `json:"edges"`
	}
)

// NewBridgeService creates a new instance of BridgeService.
func NewBridgeService(
	connectionRepo interfaces.ConnectionRepositoryInterface,
	factsheetRepo interfaces.FactsheetRepositoryInterface,
	orderExecutionRepo interfaces.OrderExecutionRepositoryInterface,
	redisClient *redis.RedisClient,
	messageService *MessageService,
	uow database.UnitOfWorkInterface,
	logger *slog.Logger,
) *BridgeService {
	return &BridgeService{
		connectionRepo:     connectionRepo,
		factsheetRepo:      factsheetRepo,
		orderExecutionRepo: orderExecutionRepo,
		redis:              redisClient,
		messageService:     messageService,
		uow:                uow,
		logger:             logger.With("service", "bridge_service"),
	}
}

// ===================================================================
// BASIC SERVICE METHODS
// ===================================================================

func (bs *BridgeService) GetRobotState(serialNumber string) (*models.StateMessage, error) {
	state, err := bs.redis.GetState(serialNumber)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("State not found for robot %s.", serialNumber))
	}
	return state, nil
}

func (bs *BridgeService) GetRobotConnectionHistory(serialNumber string, limit int) ([]models.ConnectionState, error) {
	history, err := bs.connectionRepo.GetConnectionHistory(serialNumber, limit)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to retrieve connection history.", err)
	}
	return history, nil
}

func (bs *BridgeService) GetRobotCapabilities(serialNumber string) (*models.RobotCapabilities, error) {
	capabilities, err := bs.factsheetRepo.GetRobotCapabilities(serialNumber)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Capabilities not found for robot %s.", serialNumber))
	}
	return capabilities, nil
}

func (bs *BridgeService) GetRobotManufacturer(serialNumber string) string {
	manufacturer, err := bs.connectionRepo.GetRobotManufacturer(serialNumber)
	if err != nil {
		bs.logger.Warn("Could not get manufacturer, falling back to default", "serialNumber", serialNumber, slog.Any("error", err))
		return "Roboligent"
	}
	return manufacturer
}

func (bs *BridgeService) GetConnectedRobots() ([]string, error) {
	robots, err := bs.connectionRepo.GetConnectedRobots()
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to get connected robots from database.", err)
	}
	var onlineRobots []string
	for _, robot := range robots {
		if bs.redis.IsRobotOnline(robot) {
			onlineRobots = append(onlineRobots, robot)
		}
	}
	return onlineRobots, nil
}

func (bs *BridgeService) MonitorRobotHealth(serialNumber string) (*models.RobotHealthStatus, error) {
	state, err := bs.GetRobotState(serialNumber)
	if err != nil {
		return nil, err
	}
	return &models.RobotHealthStatus{
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
// UNIFIED ORDER & ACTION SENDING METHODS
// ===================================================================

func (bs *BridgeService) SendOrderToRobot(serialNumber string, orderData models.OrderRequest) error {
	return bs.SendOrderToRobotWithTransport(serialNumber, orderData, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) SendOrderToRobotWithTransport(serialNumber string, orderData models.OrderRequest, transportType transport.TransportType) error {
	if !bs.redis.IsRobotOnline(serialNumber) {
		return utils.NewBadRequestError(fmt.Sprintf("Cannot send order: Robot %s is not online.", serialNumber))
	}

	req := &message.OrderMessageRequest{
		SerialNumber:  serialNumber,
		Manufacturer:  bs.GetRobotManufacturer(serialNumber),
		OrderID:       orderData.OrderID,
		OrderUpdateID: orderData.OrderUpdateID,
		Nodes:         orderData.Nodes,
		Edges:         orderData.Edges,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := bs.messageService.SendOrderMessage(ctx, req, transportType); err != nil {
		return utils.NewInternalServerError(fmt.Sprintf("Failed to send order via %s.", transportType), err)
	}
	return nil
}

func (bs *BridgeService) SendCustomAction(serialNumber string, actionRequest models.CustomActionRequest) error {
	return bs.SendCustomActionWithTransport(serialNumber, actionRequest, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) SendCustomActionWithTransport(serialNumber string, actionRequest models.CustomActionRequest, transportType transport.TransportType) error {
	if !bs.redis.IsRobotOnline(serialNumber) {
		return utils.NewBadRequestError(fmt.Sprintf("Cannot send action: Robot %s is not online.", serialNumber))
	}

	req := &message.InstantActionMessageRequest{
		SerialNumber: serialNumber,
		Manufacturer: bs.GetRobotManufacturer(serialNumber),
		Actions:      actionRequest.Actions,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := bs.messageService.SendInstantActionMessage(ctx, req, transportType); err != nil {
		return utils.NewInternalServerError(fmt.Sprintf("Failed to send custom action via %s.", transportType), err)
	}
	return nil
}

// ===================================================================
// ENHANCED ORDER CREATION METHODS
// ===================================================================

func (bs *BridgeService) CreateInferenceOrder(serialNumber, inferenceName string) error {
	return bs.createEnhancedOrder(serialNumber, "inference", map[string]interface{}{"inferenceName": inferenceName}, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) CreateInferenceOrderWithTransport(serialNumber, inferenceName string, transportType transport.TransportType) error {
	return bs.createEnhancedOrder(serialNumber, "inference", map[string]interface{}{"inferenceName": inferenceName}, transportType)
}

func (bs *BridgeService) CreateTrajectoryOrder(serialNumber, trajectoryName, arm string) error {
	params := map[string]interface{}{"trajectoryName": trajectoryName, "arm": arm}
	return bs.createEnhancedOrder(serialNumber, "trajectory", params, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) CreateTrajectoryOrderWithTransport(serialNumber, trajectoryName, arm string, transportType transport.TransportType) error {
	params := map[string]interface{}{"trajectoryName": trajectoryName, "arm": arm}
	return bs.createEnhancedOrder(serialNumber, "trajectory", params, transportType)
}

func (bs *BridgeService) CreateInferenceOrderWithPosition(serialNumber, inferenceName string, position models.NodePosition) error {
	params := map[string]interface{}{"inferenceName": inferenceName, "position": position}
	return bs.createEnhancedOrder(serialNumber, "inference", params, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) CreateTrajectoryOrderWithPosition(serialNumber, trajectoryName, arm string, position models.NodePosition) error {
	params := map[string]interface{}{"trajectoryName": trajectoryName, "arm": arm, "position": position}
	return bs.createEnhancedOrder(serialNumber, "trajectory", params, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) CreateCustomInferenceOrder(req *CustomInferenceOrderRequest) error {
	params := map[string]interface{}{
		"inferenceName": req.InferenceName, "position": req.Position, "customParameters": req.CustomParameters,
		"actionType": req.ActionType, "actionDescription": req.ActionDescription, "blockingType": req.BlockingType,
		"description": req.Description, "sequenceId": req.SequenceID, "released": req.Released, "edges": req.Edges,
	}
	return bs.createEnhancedOrder(req.SerialNumber, "inference", params, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) CreateCustomTrajectoryOrder(req *CustomTrajectoryOrderRequest) error {
	params := map[string]interface{}{
		"trajectoryName": req.TrajectoryName, "arm": req.Arm, "position": req.Position, "customParameters": req.CustomParameters,
		"actionType": req.ActionType, "actionDescription": req.ActionDescription, "blockingType": req.BlockingType,
		"description": req.Description, "sequenceId": req.SequenceID, "released": req.Released, "edges": req.Edges,
	}
	return bs.createEnhancedOrder(req.SerialNumber, "trajectory", params, bs.messageService.GetDefaultTransport())
}

func (bs *BridgeService) CreateDynamicOrder(req *DynamicOrderRequest) error {
	return bs.createDynamicOrder(req, bs.messageService.GetDefaultTransport())
}

// ===================================================================
// CORE HELPER METHODS
// ===================================================================

func (bs *BridgeService) createEnhancedOrder(serialNumber, orderType string, params map[string]interface{}, transportType transport.TransportType) error {
	if !bs.redis.IsRobotOnline(serialNumber) {
		return utils.NewBadRequestError(fmt.Sprintf("Cannot create order: Robot %s is not online.", serialNumber))
	}

	orderID := utils.GenerateOrderID()
	nodeID := utils.GenerateNodeID()
	actionID := utils.GenerateActionID()

	position := bs.getPositionFromParams(params)
	actionType := bs.getActionTypeFromParams(orderType, params)
	actionDescription := bs.getActionDescriptionFromParams(orderType, params)
	blockingType := utils.GetValueOrDefault(bs.getStringFromParams(params, "blockingType"), "NONE")
	description := utils.GetValueOrDefault(bs.getStringFromParams(params, "description"), fmt.Sprintf("%s Task", orderType))
	actionParams := bs.buildActionParameters(orderType, params)
	sequenceID := bs.getIntFromParams(params, "sequenceId")
	released := bs.getBoolFromParams(params, "released", true)
	edges := bs.getEdgesFromParams(params)

	nodes := []models.Node{
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
	}

	req := &message.OrderMessageRequest{
		SerialNumber:  serialNumber,
		Manufacturer:  bs.GetRobotManufacturer(serialNumber),
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes:         nodes,
		Edges:         edges,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return bs.messageService.SendOrderMessage(ctx, req, transportType)
}

func (bs *BridgeService) createDynamicOrder(req *DynamicOrderRequest, transportType transport.TransportType) error {
	if !bs.redis.IsRobotOnline(req.SerialNumber) {
		return utils.NewBadRequestError(fmt.Sprintf("Cannot create dynamic order: Robot %s is not online.", req.SerialNumber))
	}

	tx := bs.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			bs.uow.Rollback(tx)
			panic(r)
		}
	}()

	orderID := utils.GenerateOrderID()
	execution := &models.OrderExecution{
		OrderID:       orderID,
		SerialNumber:  req.SerialNumber,
		OrderUpdateID: req.OrderUpdateID,
		Status:        "CREATED",
	}

	if _, err := bs.orderExecutionRepo.CreateOrderExecution(tx, execution); err != nil {
		bs.uow.Rollback(tx)
		return utils.NewInternalServerError("Failed to create order execution record.", err)
	}

	nodes := utils.ProcessNodesWithIDs(req.Nodes)
	edges := utils.ProcessEdgesWithIDs(req.Edges)
	msgReq := &message.OrderMessageRequest{
		SerialNumber:  req.SerialNumber,
		Manufacturer:  bs.GetRobotManufacturer(req.SerialNumber),
		OrderID:       orderID,
		OrderUpdateID: req.OrderUpdateID,
		Nodes:         nodes,
		Edges:         edges,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := bs.messageService.SendOrderMessage(ctx, msgReq, transportType); err != nil {
		bs.updateOrderStatus(orderID, "FAILED", err.Error())
		bs.uow.Rollback(tx)
		return utils.NewInternalServerError(fmt.Sprintf("Failed to send dynamic order via %s.", transportType), err)
	}

	if err := bs.orderExecutionRepo.UpdateOrderStatus(tx, orderID, "SENT"); err != nil {
		bs.uow.Rollback(tx)
		return utils.NewInternalServerError("Failed to update dynamic order status to SENT.", err)
	}

	if err := bs.uow.Commit(tx); err != nil {
		return utils.NewInternalServerError("Failed to commit dynamic order transaction.", err)
	}
	bs.logger.Info("Dynamic order sent successfully", "orderId", orderID, "serialNumber", req.SerialNumber)
	return nil
}

func (bs *BridgeService) updateOrderStatus(orderID, status, errorMessage string) {
	tx := bs.uow.Begin()
	if err := bs.orderExecutionRepo.UpdateOrderStatus(tx, orderID, status, errorMessage); err != nil {
		bs.uow.Rollback(tx)
		bs.logger.Error("Failed to update order status", "orderId", orderID, "status", status, slog.Any("error", err))
	} else {
		if err := tx.Commit().Error; err != nil {
			bs.logger.Error("Failed to commit order status update", "orderId", orderID, slog.Any("error", err))
		} else {
			bs.logger.Info("Order status updated", "orderId", orderID, "status", status)
		}
	}
}

// ===================================================================
// TRANSPORT MANAGEMENT METHODS
// ===================================================================

func (bs *BridgeService) GetAvailableTransports() []transport.TransportType {
	return bs.messageService.GetAvailableTransports()
}

func (bs *BridgeService) GetDefaultTransport() transport.TransportType {
	return bs.messageService.GetDefaultTransport()
}

func (bs *BridgeService) SetDefaultTransport(transportType transport.TransportType) error {
	availableTransports := bs.GetAvailableTransports()
	for _, t := range availableTransports {
		if t == transportType {
			bs.messageService.SetDefaultTransport(transportType)
			bs.logger.Info("Default transport changed", "transport", transportType)
			return nil
		}
	}
	return utils.NewBadRequestError(fmt.Sprintf("Transport type '%s' is not available. Available: %v", transportType, availableTransports))
}

// ===================================================================
// PRIVATE HELPER METHODS
// ===================================================================

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
		return "This action will trigger the behavior tree for executing inference."
	case "trajectory":
		return "This action will trigger the behavior tree for following a recorded trajectory."
	default:
		return "Unknown action description"
	}
}

func (bs *BridgeService) buildActionParameters(orderType string, params map[string]interface{}) []models.ActionParameter {
	var actionParams []models.ActionParameter
	switch orderType {
	case "inference":
		if inferenceName := bs.getStringFromParams(params, "inferenceName"); inferenceName != "" {
			actionParams = append(actionParams, models.ActionParameter{Key: "inference_name", Value: inferenceName})
		}
	case "trajectory":
		if trajectoryName := bs.getStringFromParams(params, "trajectoryName"); trajectoryName != "" {
			actionParams = append(actionParams, models.ActionParameter{Key: "trajectory_name", Value: trajectoryName})
		}
		if arm := bs.getStringFromParams(params, "arm"); arm != "" {
			actionParams = append(actionParams, models.ActionParameter{Key: "arm", Value: arm})
		}
	}
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
