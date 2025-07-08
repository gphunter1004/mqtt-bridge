package mqtt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/database"
	"mqtt-bridge/message"
	"mqtt-bridge/models"
	"mqtt-bridge/redis"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Client wraps the PAHO MQTT client and adds application-specific logic.
type Client struct {
	client       mqtt.Client
	db           *database.Database
	redis        *redis.RedisClient
	uow          database.UnitOfWorkInterface
	msgGenerator message.MessageGenerator
	logger       *slog.Logger
	headerIDMap  map[string]int
	headerIDMux  sync.RWMutex
}

// NewClient creates and connects a new MQTT client.
func NewClient(cfg *config.Config, db *database.Database, redisClient *redis.RedisClient, uow database.UnitOfWorkInterface, logger *slog.Logger) (*Client, error) {
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
		logger:       logger.With("component", "mqtt_client"),
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
		c.logger.Info("MQTT Client disconnected")
	}
}

func (c *Client) onConnect(client mqtt.Client) {
	c.logger.Info("Successfully connected to MQTT broker. Subscribing to topics...")
	c.subscribeToAllTopics()
}

func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	c.logger.Error("Connection lost. Reconnecting...", slog.Any("error", err))
}

func (c *Client) subscribeToAllTopics() {
	c.subscribe("meili/v2/+/+/connection", c.handleConnectionMessage)
	c.subscribe("meili/v2/+/+/factsheet", c.handleFactsheetMessage)
	c.subscribe("meili/v2/+/+/state", c.handleStateMessage)
	c.subscribe("meili/v2/+/+/orderResponse", c.handleOrderResponse)
}

func (c *Client) subscribe(topic string, handler mqtt.MessageHandler) {
	if token := c.client.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
		c.logger.Error("Failed to subscribe to topic", "topic", topic, slog.Any("error", token.Error()))
	} else {
		c.logger.Info("Successfully subscribed to topic", "topic", topic)
	}
}

func (c *Client) handleConnectionMessage(client mqtt.Client, msg mqtt.Message) {
	c.logger.Info("Connection message received", "topic", msg.Topic())
	var connMsg models.ConnectionMessage
	manufacturer, serialNumber, err := c.parseMessage(msg, &connMsg)
	if err != nil {
		return // parseMessage already logs the error
	}
	logger := c.logger.With("serialNumber", serialNumber, "manufacturer", manufacturer)

	tx := c.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			c.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := c.db.SaveConnectionState(tx, &connMsg); err != nil {
		c.uow.Rollback(tx)
		logger.Error("Failed to save connection state", slog.Any("error", err))
		return
	}

	if err := c.uow.Commit(tx); err != nil {
		logger.Error("Failed to commit connection state transaction", slog.Any("error", err))
		return
	}

	if err := c.redis.SaveConnectionStatus(serialNumber, connMsg.ConnectionState); err != nil {
		logger.Error("Failed to save connection status to Redis", slog.Any("error", err))
	}

	if connMsg.ConnectionState == "ONLINE" {
		logger.Info("Robot came online, requesting factsheet.")
		go c.requestFactsheet(serialNumber, manufacturer)
	}
}

func (c *Client) handleFactsheetMessage(client mqtt.Client, msg mqtt.Message) {
	c.logger.Info("Factsheet message received", "topic", msg.Topic())
	var factsheetMsg models.FactsheetMessage
	_, serialNumber, err := c.parseMessage(msg, &factsheetMsg)
	if err != nil {
		return
	}
	logger := c.logger.With("serialNumber", serialNumber)

	tx := c.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			c.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := c.db.SaveOrUpdateFactsheet(tx, &factsheetMsg); err != nil {
		c.uow.Rollback(tx)
		logger.Error("Failed to save factsheet", slog.Any("error", err))
		return
	}

	if err := c.uow.Commit(tx); err != nil {
		logger.Error("Failed to commit factsheet transaction", slog.Any("error", err))
	} else {
		logger.Info("Factsheet saved successfully")
	}
}

func (c *Client) handleStateMessage(client mqtt.Client, msg mqtt.Message) {
	var stateMsg models.StateMessage
	manufacturer, serialNumber, err := c.parseMessage(msg, &stateMsg)
	if err != nil {
		return
	}
	logger := c.logger.With("serialNumber", serialNumber)

	if err := c.redis.SaveState(serialNumber, &stateMsg); err != nil {
		logger.Error("Failed to save state to Redis", slog.Any("error", err))
	}

	if !stateMsg.AgvPosition.PositionInitialized {
		logger.Warn("Robot position not initialized, sending initPosition command.")
		go c.sendInitPosition(serialNumber, manufacturer)
	}
}

func (c *Client) handleOrderResponse(client mqtt.Client, msg mqtt.Message) {
	c.logger.Info("Order response received", "topic", msg.Topic(), "payload", string(msg.Payload()))
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
		return fmt.Errorf("failed to generate order message: %w", err)
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
		return fmt.Errorf("failed to generate factsheet request: %w", err)
	}
	c.logger.Info("Requesting factsheet", "serialNumber", serialNumber)
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
		return fmt.Errorf("failed to generate initPosition message: %w", err)
	}
	c.logger.Info("Sending initPosition command", "serialNumber", serialNumber)
	return c.publish(topic, payload)
}

func (c *Client) publish(topic string, payload []byte) error {
	if !c.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}
	token := c.client.Publish(topic, 1, false, payload)
	go func() {
		if token.WaitTimeout(5*time.Second) && token.Error() != nil {
			c.logger.Error("Failed to publish message", "topic", topic, slog.Any("error", token.Error()))
		}
	}()
	return nil
}

func (c *Client) parseMessage(msg mqtt.Message, v interface{}) (manufacturer, serialNumber string, err error) {
	topic := msg.Topic()
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		err = fmt.Errorf("invalid topic structure: %s", topic)
		c.logger.Error("Failed to parse MQTT message", "topic", topic, slog.Any("error", err))
		return
	}
	manufacturer, serialNumber = parts[2], parts[3]
	if err = json.Unmarshal(msg.Payload(), v); err != nil {
		c.logger.Error("Failed to unmarshal JSON payload", "topic", topic, slog.Any("error", err))
		return
	}
	return
}

func (c *Client) getTopic(manufacturer, serialNumber, messageType string) string {
	return fmt.Sprintf("meili/v2/%s/%s/%s", manufacturer, serialNumber, messageType)
}

func (c *Client) LogSubscribedTopics() {
	c.logger.Info("--- Subscribed Topics (Robot -> Bridge) ---")
	c.logger.Info("1. meili/v2/+/+/connection")
	c.logger.Info("2. meili/v2/+/+/factsheet")
	c.logger.Info("3. meili/v2/+/+/state")
	c.logger.Info("4. meili/v2/+/+/orderResponse")
	c.logger.Info("-------------------------------------------")
}
