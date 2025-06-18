package topic

import (
	"context"
	"fmt"
	"time"

	"topic-data-converter/config"
	"topic-data-converter/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Subscriber struct {
	config         *config.Config
	logger         *utils.Logger
	client         mqtt.Client
	messageHandler func(topic string, payload []byte)
}

func NewSubscriber(cfg *config.Config, logger *utils.Logger) *Subscriber {
	return &Subscriber{
		config: cfg,
		logger: logger,
	}
}

func (s *Subscriber) SetMessageHandler(handler func(topic string, payload []byte)) {
	s.messageHandler = handler
}

func (s *Subscriber) Start(ctx context.Context) error {
	// Create MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(s.config.BrokerURL)
	opts.SetClientID(s.config.BrokerClientID + "_sub")
	opts.SetUsername(s.config.BrokerUsername)
	opts.SetPassword(s.config.BrokerPassword)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	// Set connection lost handler
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		s.logger.Errorf("MQTT connection lost: %v", err)
	})

	// Set on connect handler
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		s.logger.Info("MQTT subscriber connected")
		s.subscribeToTopics()
	})

	// Create and start client
	s.client = mqtt.NewClient(opts)
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		s.logger.Info("Shutting down subscriber...")
		s.client.Disconnect(250)
	}()

	return nil
}

func (s *Subscriber) subscribeToTopics() {
	topics := s.config.GetPLCTopics()

	for _, topic := range topics {
		if token := s.client.Subscribe(topic, 1, s.messageCallback); token.Wait() && token.Error() != nil {
			s.logger.Errorf("Failed to subscribe to topic %s: %v", topic, token.Error())
		} else {
			s.logger.Infof("Subscribed to topic: %s", topic)
		}
	}
}

func (s *Subscriber) messageCallback(client mqtt.Client, msg mqtt.Message) {
	s.logger.Infof("📨 MQTT MESSAGE RECEIVED - Topic: %s, Size: %d bytes", msg.Topic(), len(msg.Payload()))
	s.logger.Debugf("📨 MQTT MESSAGE DETAILS - QoS: %d, Retained: %t, Duplicate: %t",
		msg.Qos(), msg.Retained(), msg.Duplicate())

	if s.messageHandler != nil {
		go s.messageHandler(msg.Topic(), msg.Payload())
	} else {
		s.logger.Warnf("⚠️  NO MESSAGE HANDLER - Message ignored for topic: %s", msg.Topic())
	}
}
