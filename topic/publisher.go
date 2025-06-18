package topic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"topic-data-converter/config"
	"topic-data-converter/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Publisher struct {
	config *config.Config
	logger *utils.Logger
	client mqtt.Client
}

func NewPublisher(cfg *config.Config, logger *utils.Logger) *Publisher {
	return &Publisher{
		config: cfg,
		logger: logger,
	}
}

func (p *Publisher) Start(ctx context.Context) error {
	// Create MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(p.config.BrokerURL)
	opts.SetClientID(p.config.BrokerClientID + "_pub")
	opts.SetUsername(p.config.BrokerUsername)
	opts.SetPassword(p.config.BrokerPassword)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	// Set connection lost handler
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		p.logger.Errorf("MQTT connection lost: %v", err)
	})

	// Set on connect handler
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		p.logger.Info("MQTT publisher connected")
	})

	// Create and start client
	p.client = mqtt.NewClient(opts)
	if token := p.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		p.logger.Info("Shutting down publisher...")
		p.client.Disconnect(250)
	}()

	return nil
}

func (p *Publisher) Publish(topic string, payload []byte) error {
	if !p.client.IsConnected() {
		p.logger.Errorf("❌ MQTT PUBLISH FAILED - Client not connected")
		return fmt.Errorf("MQTT client is not connected")
	}

	p.logger.Debugf("📡 MQTT PUBLISHING - Topic: %s, Size: %d bytes", topic, len(payload))

	token := p.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		p.logger.Errorf("❌ MQTT PUBLISH FAILED - Topic: %s, Error: %v", topic, token.Error())
		return fmt.Errorf("failed to publish message: %w", token.Error())
	}

	p.logger.Infof("📡 MQTT PUBLISHED - Topic: %s", topic)
	p.logger.Debugf("📡 MQTT PUBLISHED - Message: %s", string(payload))
	return nil
}

func (p *Publisher) PublishJSON(topic string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return p.Publish(topic, jsonData)
}
