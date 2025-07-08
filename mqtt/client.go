package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/database"
	"mqtt-bridge/message"
	"mqtt-bridge/models"
	"mqtt-bridge/redis"

	// --- (FIXED) Corrected the import path ---
	mqtt "github.com/eclipse/paho.mqtt.golang"
	// --- END OF FIX ---
)

// Client wraps the PAHO MQTT client and adds application-specific logic.
type Client struct {
	client       mqtt.Client
	db           *database.Database
	redis        *redis.RedisClient
	uow          database.UnitOfWorkInterface
	msgGenerator message.MessageGenerator
	headerIDMap  map[string]int
	headerIDMux  sync.RWMutex
}

// NewClient creates and connects a new MQTT client.
func NewClient(cfg *config.Config, db *database.Database, redisClient *redis.RedisClient, uow database.UnitOfWorkInterface) (*Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.MQTTBroker).
		SetClientID(cfg.MQTTClientID).
		SetUsername(cfg.MQTTUsername).
		SetPassword(cfg.MQTTPassword).
		SetKeepAlive(60 * time.Second).
		SetPingTimeout(1 * time.Second).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(10 * time.Second).
		SetCleanSession(true)

	mqttClient := &Client{
		db:           db,
		redis:        redisClient,
		uow:          uow,
		msgGenerator: message.NewMessageGenerator(),
		headerIDMap:  make(map[string]int),
	}

	opts.SetOnConnectHandler(mqttClient.onConnect)
	opts.SetConnectionLostHandler(mqttClient.onConnectionLost)
	client := mqtt.NewClient(opts)
	mqttClient.client = client

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}
	return mqttClient, nil
}

// GetClient returns the underlying PAHO MQTT client.
func (c *Client) GetClient() mqtt.Client {
	return c.client
}

// Disconnect gracefully disconnects the client.
func (c *Client) Disconnect() {
	if c.client.IsConnected() {
		c.client.Disconnect(250)
		log.Println("[MQTT] Client disconnected")
	}
}

func (c *Client) onConnect(client mqtt.Client) {
	log.Println("[MQTT] Successfully connected. Subscribing to topics...")
	c.subscribeToAllTopics()
}

func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	log.Printf("[MQTT ERROR] Connection lost: %v. Reconnecting...", err)
}

func (c *Client) subscribeToAllTopics() {
	c.subscribe("meili/v2/+/+/connection", c.handleConnectionMessage)
	c.subscribe("meili/v2/+/+/factsheet", c.handleFactsheetMessage)
	c.subscribe("meili/v2/+/+/state", c.handleStateMessage)
	c.subscribe("meili/v2/+/+/orderResponse", c.handleOrderResponse)
}

func (c *Client) subscribe(topic string, handler mqtt.MessageHandler) {
	if token := c.client.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
		log.Printf("[MQTT ERROR] Failed to subscribe to topic '%s': %v", topic, token.Error())
	} else {
		log.Printf("[MQTT] Successfully subscribed to topic: %s", topic)
	}
}

// handleConnectionMessage processes connection status updates within a transaction.
func (c *Client) handleConnectionMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("[MQTT RECV] Connection message on topic: %s", msg.Topic())
	var connMsg models.ConnectionMessage
	manufacturer, serialNumber, err := c.parseMessage(msg, &connMsg)
	if err != nil {
		return
	}

	tx := c.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			c.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := c.db.SaveConnectionState(tx, &connMsg); err != nil {
		c.uow.Rollback(tx)
		log.Printf("[DB ERROR] Failed to save connection state for %s: %v", serialNumber, err)
		return
	}

	if err := c.uow.Commit(tx); err != nil {
		log.Printf("[DB ERROR] Failed to commit connection state transaction for %s: %v", serialNumber, err)
		return
	}

	if err := c.redis.SaveConnectionStatus(serialNumber, connMsg.ConnectionState); err != nil {
		log.Printf("[REDIS ERROR] Failed to save connection status for %s: %v", serialNumber, err)
	}

	if connMsg.ConnectionState == "ONLINE" {
		log.Printf("[MQTT ACTION] Robot %s is online, requesting factsheet.", serialNumber)
		go c.requestFactsheet(serialNumber, manufacturer)
	}
}

// handleFactsheetMessage processes capability information within a transaction.
func (c *Client) handleFactsheetMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("[MQTT RECV] Factsheet message on topic: %s", msg.Topic())
	var factsheetMsg models.FactsheetMessage
	_, serialNumber, err := c.parseMessage(msg, &factsheetMsg)
	if err != nil {
		return
	}

	tx := c.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			c.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := c.db.SaveOrUpdateFactsheet(tx, &factsheetMsg); err != nil {
		c.uow.Rollback(tx)
		log.Printf("[DB ERROR] Failed to save factsheet for %s: %v", serialNumber, err)
		return
	}

	if err := c.uow.Commit(tx); err != nil {
		log.Printf("[DB ERROR] Failed to commit factsheet transaction for %s: %v", serialNumber, err)
	} else {
		log.Printf("[DB SUCCESS] Factsheet for %s saved successfully.", serialNumber)
	}
}

func (c *Client) handleStateMessage(client mqtt.Client, msg mqtt.Message) {
	var stateMsg models.StateMessage
	manufacturer, serialNumber, err := c.parseMessage(msg, &stateMsg)
	if err != nil {
		return
	}
	if err := c.redis.SaveState(serialNumber, &stateMsg); err != nil {
		log.Printf("[REDIS ERROR] Failed to save state for %s: %v", serialNumber, err)
	}
	if !stateMsg.AgvPosition.PositionInitialized {
		log.Printf("[MQTT ACTION] Robot %s position not initialized, sending initPosition command.", serialNumber)
		go c.sendInitPosition(serialNumber, manufacturer)
	}
}

func (c *Client) handleOrderResponse(client mqtt.Client, msg mqtt.Message) {
	log.Printf("[MQTT RECV] Order response on topic: %s", msg.Topic())
}

func (c *Client) SendOrder(serialNumber string, orderMsg *models.OrderMessage) error {
	manufacturer := orderMsg.Manufacturer
	if manufacturer == "" {
		manufacturer = "Roboligent"
	}
	topic := c.getTopic(manufacturer, serialNumber, "order")
	req := &message.OrderMessageRequest{
		SerialNumber:   serialNumber,
		Manufacturer:   manufacturer,
		OrderID:        orderMsg.OrderID,
		OrderUpdateID:  orderMsg.OrderUpdateID,
		Nodes:          orderMsg.Nodes,
		Edges:          orderMsg.Edges,
		CustomHeaderID: &orderMsg.HeaderID,
	}
	payload, err := c.msgGenerator.GenerateOrderMessage(req)
	if err != nil {
		return err
	}
	return c.publish(topic, payload)
}

func (c *Client) requestFactsheet(serialNumber, manufacturer string) error {
	topic := c.getTopic(manufacturer, serialNumber, "instantActions")
	req := &message.FactsheetRequestMessageRequest{
		SerialNumber: serialNumber,
		Manufacturer: manufacturer,
	}
	payload, err := c.msgGenerator.GenerateFactsheetRequestMessage(req)
	if err != nil {
		return err
	}
	return c.publish(topic, payload)
}

func (c *Client) sendInitPosition(serialNumber, manufacturer string) error {
	topic := c.getTopic(manufacturer, serialNumber, "instantActions")
	req := &message.InitPositionMessageRequest{
		SerialNumber: serialNumber,
		Manufacturer: manufacturer,
		Pose:         map[string]interface{}{"x": 0.0, "y": 0.0, "theta": 0.0, "mapId": ""},
	}
	payload, err := c.msgGenerator.GenerateInitPositionMessage(req)
	if err != nil {
		return err
	}
	return c.publish(topic, payload)
}

func (c *Client) publish(topic string, payload []byte) error {
	if !c.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}
	token := c.client.Publish(topic, 1, false, payload)
	go func() {
		if token.WaitTimeout(5*time.Second) && token.Error() != nil {
			log.Printf("[MQTT ERROR] Failed to publish to topic '%s': %v", topic, token.Error())
		}
	}()
	return nil
}

func (c *Client) parseMessage(msg mqtt.Message, v interface{}) (manufacturer, serialNumber string, err error) {
	topic := msg.Topic()
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		err = fmt.Errorf("invalid topic structure: %s", topic)
		log.Printf("[MQTT ERROR] %v", err)
		return
	}
	manufacturer, serialNumber = parts[2], parts[3]
	if err = json.Unmarshal(msg.Payload(), v); err != nil {
		log.Printf("[MQTT ERROR] Failed to unmarshal JSON for topic %s: %v", topic, err)
		return
	}
	return
}

func (c *Client) getTopic(manufacturer, serialNumber, messageType string) string {
	return fmt.Sprintf("meili/v2/%s/%s/%s", manufacturer, serialNumber, messageType)
}

func (c *Client) LogSubscribedTopics() {
	log.Println("--- Subscribed Topics (Robot -> Bridge) ---")
	log.Println("1. meili/v2/+/+/connection")
	log.Println("2. meili/v2/+/+/factsheet")
	log.Println("3. meili/v2/+/+/state")
	log.Println("4. meili/v2/+/+/orderResponse")
	log.Println("-------------------------------------------")
}
