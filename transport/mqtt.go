package transport

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTTransport implements the MessageTransport interface for MQTT communication.
type MQTTTransport struct {
	client  mqtt.Client
	qos     byte
	timeout time.Duration
	logger  *slog.Logger
}

// NewMQTTTransport creates a new instance of MQTTTransport.
func NewMQTTTransport(client mqtt.Client, timeout time.Duration, logger *slog.Logger) *MQTTTransport {
	return &MQTTTransport{
		client:  client,
		qos:     1, // Default to QoS 1
		timeout: timeout,
		logger:  logger.With("transport_type", "mqtt"),
	}
}

// Send publishes a payload to a given MQTT topic.
func (mt *MQTTTransport) Send(ctx context.Context, topic string, payload []byte) error {
	if !mt.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	logger := mt.logger.With("topic", topic)
	logger.Debug("Publishing message", "payload_size", len(payload), "qos", mt.qos)

	token := mt.client.Publish(topic, mt.qos, false, payload)

	// Use a select statement to handle context cancellation and timeout.
	select {
	case <-ctx.Done():
		err := fmt.Errorf("MQTT publish cancelled by context: %w", ctx.Err())
		logger.Error("Publish context cancelled", slog.Any("error", err))
		return err
	case <-time.After(mt.timeout):
		err := fmt.Errorf("MQTT publish timed out after %v", mt.timeout)
		logger.Error("Publish timed out", slog.Any("error", err))
		return err
	default:
		// token.Wait() blocks until the message is sent.
		if token.Wait() && token.Error() != nil {
			logger.Error("MQTT publish failed", slog.Any("error", token.Error()))
			return fmt.Errorf("MQTT publish failed: %w", token.Error())
		}
	}

	logger.Info("Message published successfully")
	return nil
}

// GetTransportType returns the transport's type.
func (mt *MQTTTransport) GetTransportType() TransportType {
	return TransportTypeMQTT
}

// Close disconnects the MQTT client.
func (mt *MQTTTransport) Close() error {
	// The MQTT client's lifecycle is managed in the main package,
	// so this method doesn't need to do anything. It's here to satisfy the interface.
	return nil
}

// SetQoS allows changing the Quality of Service for subsequent messages.
func (mt *MQTTTransport) SetQoS(qos byte) {
	if qos > 2 {
		qos = 2
	}
	mt.qos = qos
	mt.logger.Debug("QoS set", "new_qos", qos)
}
