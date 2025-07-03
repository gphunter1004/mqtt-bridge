package mqtt

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/redis"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client      mqtt.Client
	db          *database.Database
	redis       *redis.RedisClient
	headerIDMap map[string]int
	headerIDMux sync.RWMutex
}

func NewClient(cfg *config.Config, db *database.Database, redisClient *redis.RedisClient) (*Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTBroker)
	opts.SetClientID(cfg.MQTTClientID)
	opts.SetUsername(cfg.MQTTUsername)
	opts.SetPassword(cfg.MQTTPassword)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetCleanSession(true)

	mqttClient := &Client{
		db:          db,
		redis:       redisClient,
		headerIDMap: make(map[string]int),
	}

	// Set default message handler
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[MQTT DEFAULT] Unhandled message on topic: %s, payload: %s", msg.Topic(), string(msg.Payload()))
	})

	// Set connection handlers
	opts.SetOnConnectHandler(mqttClient.onConnect)
	opts.SetConnectionLostHandler(mqttClient.onConnectionLost)

	client := mqtt.NewClient(opts)
	mqttClient.client = client

	log.Printf("[MQTT] Attempting to connect to broker: %s", cfg.MQTTBroker)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	log.Println("[MQTT] MQTT client connected successfully")
	return mqttClient, nil
}

func (c *Client) onConnect(client mqtt.Client) {
	log.Println("====================================")
	log.Println("[MQTT] MQTT client connected successfully")
	log.Println("[MQTT] Starting subscription to MQTT topics...")
	log.Println("====================================")

	// Subscribe to all required topics
	c.subscribeToAllTopics()

	log.Println("====================================")
	log.Println("[MQTT] All MQTT topic subscriptions completed")
	log.Println("[MQTT] Bridge server is ready to handle robot communications")
	log.Println("====================================")
}

func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	log.Printf("[MQTT ERROR] MQTT connection lost: %v", err)
	log.Println("[MQTT] Attempting to reconnect...")
}

func (c *Client) subscribeToAllTopics() {
	// Subscribe to connection topics
	c.subscribeToConnectionTopics()

	// Subscribe to factsheet topics
	c.subscribeToFactsheetTopics()

	// Subscribe to state topics
	c.subscribeToStateTopics()

	// Subscribe to order topics (for order confirmations/responses)
	c.subscribeToOrderTopics()
}

func (c *Client) subscribeToConnectionTopics() {
	topic := "meili/v2/Roboligent/+/connection"
	log.Printf("[MQTT] Attempting to subscribe to connection topic: %s", topic)

	token := c.client.Subscribe(topic, 1, c.handleConnectionMessage)
	token.Wait()
	if token.Error() != nil {
		log.Printf("[MQTT ERROR] Failed to subscribe to connection topic: %v", token.Error())
	} else {
		log.Printf("[MQTT SUCCESS] Successfully subscribed to connection topic: %s", topic)
	}
}

func (c *Client) subscribeToFactsheetTopics() {
	topic := "meili/v2/+/+/factsheet"
	log.Printf("[MQTT] Attempting to subscribe to factsheet topic: %s", topic)

	token := c.client.Subscribe(topic, 1, c.handleFactsheetMessage)
	token.Wait()
	if token.Error() != nil {
		log.Printf("[MQTT ERROR] Failed to subscribe to factsheet topic: %v", token.Error())
	} else {
		log.Printf("[MQTT SUCCESS] Successfully subscribed to factsheet topic: %s", topic)
	}
}

func (c *Client) subscribeToStateTopics() {
	topic := "meili/v2/Roboligent/+/state"
	log.Printf("[MQTT] Attempting to subscribe to state topic: %s", topic)

	token := c.client.Subscribe(topic, 1, c.handleStateMessage)
	token.Wait()
	if token.Error() != nil {
		log.Printf("[MQTT ERROR] Failed to subscribe to state topic: %v", token.Error())
	} else {
		log.Printf("[MQTT SUCCESS] Successfully subscribed to state topic: %s", topic)
	}
}

func (c *Client) subscribeToOrderTopics() {
	topic := "meili/v2/Roboligent/+/orderResponse"
	log.Printf("[MQTT] Attempting to subscribe to order response topic: %s", topic)

	token := c.client.Subscribe(topic, 1, c.handleOrderResponse)
	token.Wait()
	if token.Error() != nil {
		log.Printf("[MQTT ERROR] Failed to subscribe to order response topic: %v", token.Error())
	} else {
		log.Printf("[MQTT SUCCESS] Successfully subscribed to order response topic: %s", topic)
	}
}

func (c *Client) handleConnectionMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("[MQTT RECEIVE] Connection message received from topic: %s", msg.Topic())
	log.Printf("[MQTT PAYLOAD] Connection payload: %s", string(msg.Payload()))

	var connectionMsg models.ConnectionMessage
	if err := json.Unmarshal(msg.Payload(), &connectionMsg); err != nil {
		log.Printf("[MQTT ERROR] Failed to unmarshal connection message: %v", err)
		log.Printf("[MQTT ERROR] Raw payload: %s", string(msg.Payload()))
		return
	}

	log.Printf("[MQTT PARSED] Connection state: %s for robot: %s (HeaderID: %d)",
		connectionMsg.ConnectionState, connectionMsg.SerialNumber, connectionMsg.HeaderID)

	// Save to database
	if err := c.db.SaveConnectionState(&connectionMsg); err != nil {
		log.Printf("[DB ERROR] Failed to save connection state: %v", err)
	} else {
		log.Printf("[DB SUCCESS] Connection state saved to database for robot: %s", connectionMsg.SerialNumber)
	}

	// Save to Redis
	if err := c.redis.SaveConnectionStatus(connectionMsg.SerialNumber, connectionMsg.ConnectionState); err != nil {
		log.Printf("[REDIS ERROR] Failed to save connection status to Redis: %v", err)
	} else {
		log.Printf("[REDIS SUCCESS] Connection status saved to Redis for robot: %s", connectionMsg.SerialNumber)
	}

	// If robot comes online, request factsheet
	if connectionMsg.ConnectionState == "ONLINE" {
		log.Printf("[MQTT ACTION] Robot %s is online, requesting factsheet", connectionMsg.SerialNumber)
		go func() {
			time.Sleep(1 * time.Second) // Small delay before requesting factsheet
			c.requestFactsheet(connectionMsg.SerialNumber)
		}()
	}
}

func (c *Client) handleFactsheetMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("[MQTT RECEIVE] Factsheet message received from topic: %s", msg.Topic())
	log.Printf("[MQTT PAYLOAD] Factsheet payload size: %d bytes", len(msg.Payload()))

	// First, let's check if we can parse the JSON at all
	var rawData map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &rawData); err != nil {
		log.Printf("[MQTT ERROR] Failed to unmarshal raw JSON: %v", err)
		return
	}

	log.Printf("[MQTT DEBUG] Raw JSON keys: %v", getKeys(rawData))

	if protocolFeatures, ok := rawData["protocolFeatures"].(map[string]interface{}); ok {
		if agvActions, ok := protocolFeatures["AgvActions"].([]interface{}); ok {
			log.Printf("[MQTT DEBUG] Found %d AGV actions in raw JSON", len(agvActions))
			for i, action := range agvActions {
				if actionMap, ok := action.(map[string]interface{}); ok {
					if actionType, ok := actionMap["ActionType"].(string); ok {
						log.Printf("[MQTT DEBUG] Raw action %d: %s", i+1, actionType)
					}
					if params, ok := actionMap["ActionParameters"].([]interface{}); ok {
						log.Printf("[MQTT DEBUG] Raw action %d has %d parameters", i+1, len(params))
						for j, param := range params {
							if paramMap, ok := param.(map[string]interface{}); ok {
								log.Printf("[MQTT DEBUG] Raw param %d: %v", j+1, paramMap)
							}
						}
					}
				}
			}
		}
	}

	var factsheetMsg models.FactsheetMessage
	if err := json.Unmarshal(msg.Payload(), &factsheetMsg); err != nil {
		log.Printf("[MQTT ERROR] Failed to unmarshal factsheet message: %v", err)
		log.Printf("[MQTT ERROR] Raw payload: %s", string(msg.Payload()))
		return
	}

	log.Printf("[MQTT PARSED] Factsheet received for robot: %s, version: %s",
		factsheetMsg.SerialNumber, factsheetMsg.Version)
	log.Printf("[MQTT PARSED] Robot capabilities: %d actions available",
		len(factsheetMsg.ProtocolFeatures.AgvActions))

	// Log action details
	for i, action := range factsheetMsg.ProtocolFeatures.AgvActions {
		log.Printf("[MQTT PARSED] Action %d: %s (%d parameters)",
			i+1, action.ActionType, len(action.ActionParameters))
		for j, param := range action.ActionParameters {
			log.Printf("[MQTT PARSED]   Param %d: Key=%s, Description=%s, DataType=%s, Optional=%t",
				j+1, param.Key, param.Description, param.ValueDataType, param.IsOptional)
		}
	}

	// Save to database
	if err := c.db.SaveOrUpdateFactsheet(&factsheetMsg); err != nil {
		log.Printf("[DB ERROR] Failed to save factsheet: %v", err)
		return
	}

	log.Printf("[DB SUCCESS] Factsheet saved successfully for robot: %s", factsheetMsg.SerialNumber)

	// Debug: Check what was actually saved
	c.db.DebugAgvActions(factsheetMsg.SerialNumber)
}

// Helper function to get map keys
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (c *Client) handleStateMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("[MQTT RECEIVE] State message received from topic: %s", msg.Topic())

	var stateMsg models.StateMessage
	if err := json.Unmarshal(msg.Payload(), &stateMsg); err != nil {
		log.Printf("[MQTT ERROR] Failed to unmarshal state message: %v", err)
		return
	}

	log.Printf("[MQTT PARSED] State update for robot: %s, headerID: %d, battery: %.1f%%, position initialized: %t",
		stateMsg.SerialNumber, stateMsg.HeaderID, stateMsg.BatteryState.BatteryCharge, stateMsg.AgvPosition.PositionInitialized)

	// Save to Redis
	if err := c.redis.SaveState(stateMsg.SerialNumber, &stateMsg); err != nil {
		log.Printf("[REDIS ERROR] Failed to save state to Redis: %v", err)
		return
	}
	log.Printf("[REDIS SUCCESS] State saved to Redis for robot: %s", stateMsg.SerialNumber)

	// Check if position needs initialization
	if !stateMsg.AgvPosition.PositionInitialized {
		log.Printf("[MQTT ACTION] Robot %s position not initialized, sending initPosition command", stateMsg.SerialNumber)
		c.sendInitPosition(stateMsg.SerialNumber)
	}
}

func (c *Client) handleOrderResponse(client mqtt.Client, msg mqtt.Message) {
	log.Printf("[MQTT RECEIVE] Order response received from topic: %s", msg.Topic())
	log.Printf("[MQTT PAYLOAD] Order response payload: %s", string(msg.Payload()))

	// Extract serial number from topic
	topicParts := strings.Split(msg.Topic(), "/")
	if len(topicParts) >= 4 {
		serialNumber := topicParts[3]
		log.Printf("[MQTT PARSED] Order response from robot: %s", serialNumber)

		// You can add additional processing here for order responses
		// For example, updating order status in database
	}
}

func (c *Client) requestFactsheet(serialNumber string) {
	headerID := c.getNextHeaderID(serialNumber)
	actionID := c.generateActionID() + "_" + strconv.FormatInt(time.Now().UnixNano()/1000000, 10)

	factsheetRequest := models.InstantActionMessage{
		HeaderID:     headerID,
		Timestamp:    time.Now().Format("2006-01-02T15:04:05.000000000Z"),
		Version:      "2.0.0",
		Manufacturer: "Roboligent",
		SerialNumber: serialNumber,
		Actions: []models.Action{
			{
				ActionType:       "factsheetRequest",
				ActionID:         actionID,
				BlockingType:     "NONE",
				ActionParameters: []models.ActionParameter{},
			},
		},
	}

	topic := fmt.Sprintf("meili/v2/Roboligent/%s/instantActions", serialNumber)
	log.Printf("[MQTT SEND] Sending factsheet request to topic: %s", topic)
	log.Printf("[MQTT SEND] Factsheet request - HeaderID: %d, ActionID: %s", headerID, actionID)

	if err := c.publishMessage(topic, factsheetRequest); err != nil {
		log.Printf("[MQTT ERROR] Failed to send factsheet request: %v", err)
	} else {
		log.Printf("[MQTT SUCCESS] Factsheet request sent successfully to robot: %s", serialNumber)
	}
}

func (c *Client) sendInitPosition(serialNumber string) {
	headerID := c.getNextHeaderID(serialNumber)
	actionID := c.generateActionID() + "_" + strconv.FormatInt(time.Now().UnixNano()/1000000, 10)

	pose := map[string]interface{}{
		"lastNodeId": "",
		"mapId":      "",
		"theta":      0.0,
		"x":          0.0,
		"y":          0.0,
	}

	initPositionRequest := models.InstantActionMessage{
		HeaderID:     headerID,
		Timestamp:    time.Now().Format("2006-01-02T15:04:05.000000000Z"),
		Version:      "2.0.0",
		Manufacturer: "Roboligent",
		SerialNumber: serialNumber,
		Actions: []models.Action{
			{
				ActionType:   "initPosition",
				ActionID:     actionID,
				BlockingType: "NONE",
				ActionParameters: []models.ActionParameter{
					{
						Key:   "pose",
						Value: pose,
					},
				},
			},
		},
	}

	topic := fmt.Sprintf("meili/v2/Roboligent/%s/instantActions", serialNumber)
	log.Printf("[MQTT SEND] Sending initPosition command to topic: %s", topic)
	log.Printf("[MQTT SEND] InitPosition request - HeaderID: %d, ActionID: %s", headerID, actionID)

	if err := c.publishMessage(topic, initPositionRequest); err != nil {
		log.Printf("[MQTT ERROR] Failed to send initPosition command: %v", err)
	} else {
		log.Printf("[MQTT SUCCESS] InitPosition command sent successfully to robot: %s", serialNumber)
	}
}

func (c *Client) SendOrder(serialNumber string, orderMsg *models.OrderMessage) error {
	topic := fmt.Sprintf("meili/v2/Roboligent/%s/order", serialNumber)
	log.Printf("[MQTT SEND] Sending order to topic: %s", topic)
	log.Printf("[MQTT SEND] Order details - OrderID: %s, UpdateID: %d, Nodes: %d",
		orderMsg.OrderID, orderMsg.OrderUpdateID, len(orderMsg.Nodes))

	if err := c.publishMessage(topic, orderMsg); err != nil {
		log.Printf("[MQTT ERROR] Failed to send order: %v", err)
		return err
	}

	log.Printf("[MQTT SUCCESS] Order sent successfully to robot: %s", serialNumber)
	return nil
}

func (c *Client) publishMessage(topic string, message interface{}) error {
	payload, err := json.Marshal(message)
	if err != nil {
		log.Printf("[MQTT ERROR] Failed to marshal message for topic %s: %v", topic, err)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	log.Printf("[MQTT PUBLISH] Publishing to topic: %s", topic)
	log.Printf("[MQTT PUBLISH] Payload size: %d bytes", len(payload))
	log.Printf("[MQTT PUBLISH] Payload preview: %s", string(payload)[:min(200, len(payload))])

	token := c.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		log.Printf("[MQTT ERROR] Failed to publish message to topic %s: %v", topic, token.Error())
		return fmt.Errorf("failed to publish message: %w", token.Error())
	}

	log.Printf("[MQTT SUCCESS] Message published successfully to topic: %s", topic)
	return nil
}

func (c *Client) PublishMessage(topic string, payload []byte) error {
	log.Printf("[MQTT PUBLISH] Publishing raw message to topic: %s", topic)
	log.Printf("[MQTT PUBLISH] Raw payload size: %d bytes", len(payload))

	token := c.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		log.Printf("[MQTT ERROR] Failed to publish raw message to topic %s: %v", topic, token.Error())
		return fmt.Errorf("failed to publish message: %w", token.Error())
	}

	log.Printf("[MQTT SUCCESS] Raw message published successfully to topic: %s", topic)
	return nil
}

func (c *Client) getNextHeaderID(serialNumber string) int {
	c.headerIDMux.Lock()
	defer c.headerIDMux.Unlock()

	c.headerIDMap[serialNumber]++
	return c.headerIDMap[serialNumber]
}

func (c *Client) generateActionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (c *Client) LogSubscribedTopics() {
	log.Println("====================================")
	log.Println("[MQTT] Currently subscribed topics:")
	log.Println("[MQTT] 1. meili/v2/Roboligent/+/connection (Robot connection states)")
	log.Println("[MQTT] 2. meili/v2/+/+/factsheet (Robot factsheet responses)")
	log.Println("[MQTT] 3. meili/v2/Roboligent/+/state (Robot state updates)")
	log.Println("[MQTT] 4. meili/v2/Roboligent/+/orderResponse (Order confirmations)")
	log.Println("====================================")
	log.Println("[MQTT] Published topics by bridge:")
	log.Println("[MQTT] 1. meili/v2/Roboligent/{serial}/instantActions (Commands to robots)")
	log.Println("[MQTT] 2. meili/v2/Roboligent/{serial}/order (Orders to robots)")
	log.Println("====================================")
}

func (c *Client) Disconnect() {
	c.client.Disconnect(250)
	log.Println("MQTT client disconnected")
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
