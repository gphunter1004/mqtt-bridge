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

	capabilities := &RobotCapabilities{
		SerialNumber:       serialNumber,
		PhysicalParameters: physicalParams,
		TypeSpecification:  typeSpec,
		AvailableActions:   actions,
	}

	return capabilities, nil
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
		Timestamp:     time.Now().Format("2006-01-02T15:04:05.000000000Z"),
		Version:       "2.0.0",
		Manufacturer:  "Roboligent",
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

	log.Printf("Order sent successfully to robot %s", serialNumber)
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
		Timestamp:    time.Now().Format("2006-01-02T15:04:05.000000000Z"),
		Version:      "2.0.0",
		Manufacturer: "Roboligent",
		SerialNumber: serialNumber,
		Actions:      actionRequest.Actions,
	}

	payload, err := json.Marshal(actionMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal action message: %w", err)
	}

	topic := fmt.Sprintf("meili/v2/Roboligent/%s/instantActions", serialNumber)
	err = bs.mqttClient.PublishMessage(topic, payload)
	if err != nil {
		return fmt.Errorf("failed to send custom action: %w", err)
	}

	log.Printf("Custom action sent successfully to robot %s", serialNumber)
	return nil
}

// GetConnectedRobots returns a list of currently connected robots
func (bs *BridgeService) GetConnectedRobots() ([]string, error) {
	var robots []string
	var connections []models.ConnectionState

	// Get latest connection state for each robot
	err := bs.db.DB.Select("DISTINCT ON (serial_number) serial_number, connection_state").
		Where("connection_state = ?", "ONLINE").
		Order("serial_number, created_at DESC").
		Find(&connections).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get connected robots: %w", err)
	}

	for _, conn := range connections {
		// Double check with Redis
		if bs.redis.IsRobotOnline(conn.SerialNumber) {
			robots = append(robots, conn.SerialNumber)
		}
	}

	return robots, nil
}

// CreateInferenceOrder creates an order with an inference action
func (bs *BridgeService) CreateInferenceOrder(serialNumber, inferenceName string) error {
	orderID := bs.generateUniqueID()
	nodeID := bs.generateUniqueID()
	actionID := bs.generateUniqueID()

	orderData := OrderRequest{
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes: []models.Node{
			{
				NodeID:      nodeID,
				Description: "Inference Task",
				SequenceID:  0,
				Released:    true,
				NodePosition: models.NodePosition{
					X:                     0.0,
					Y:                     0.0,
					Theta:                 0.0,
					AllowedDeviationXY:    0.0,
					AllowedDeviationTheta: 0.0,
					MapID:                 "",
				},
				Actions: []models.Action{
					{
						ActionType:        "Roboligent Robin - Inference",
						ActionID:          actionID,
						ActionDescription: "This is an action will trigger the behavior tree for executing inference.",
						BlockingType:      "NONE",
						ActionParameters: []models.ActionParameter{
							{
								Key:   "inference_name",
								Value: inferenceName,
							},
						},
					},
				},
			},
		},
		Edges: []models.Edge{},
	}

	return bs.SendOrderToRobot(serialNumber, orderData)
}

// CreateTrajectoryOrder creates an order with a trajectory action
func (bs *BridgeService) CreateTrajectoryOrder(serialNumber, trajectoryName, arm string) error {
	orderID := bs.generateUniqueID()
	nodeID := bs.generateUniqueID()
	actionID := bs.generateUniqueID()

	orderData := OrderRequest{
		OrderID:       orderID,
		OrderUpdateID: 0,
		Nodes: []models.Node{
			{
				NodeID:      nodeID,
				Description: "Trajectory Task",
				SequenceID:  0,
				Released:    true,
				NodePosition: models.NodePosition{
					X:                     0.0,
					Y:                     0.0,
					Theta:                 0.0,
					AllowedDeviationXY:    0.0,
					AllowedDeviationTheta: 0.0,
					MapID:                 "",
				},
				Actions: []models.Action{
					{
						ActionType:        "Roboligent Robin - Follow Trajectory",
						ActionID:          actionID,
						ActionDescription: "This action will trigger the behavior tree for following a recorded trajectory.",
						BlockingType:      "NONE",
						ActionParameters: []models.ActionParameter{
							{
								Key:   "trajectory_name",
								Value: trajectoryName,
							},
							{
								Key:   "arm",
								Value: arm,
							},
						},
					},
				},
			},
		},
		Edges: []models.Edge{},
	}

	return bs.SendOrderToRobot(serialNumber, orderData)
}

// generateUniqueID generates a unique ID for orders, nodes, and actions
func (bs *BridgeService) generateUniqueID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// MonitorRobotHealth checks robot health status
func (bs *BridgeService) MonitorRobotHealth(serialNumber string) (*RobotHealthStatus, error) {
	state, err := bs.GetRobotState(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get robot state for health check: %w", err)
	}

	isOnline := bs.redis.IsRobotOnline(serialNumber)

	healthStatus := &RobotHealthStatus{
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
	}

	return healthStatus, nil
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
