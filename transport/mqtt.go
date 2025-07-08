package transport

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTTransport implements the MessageTransport interface for MQTT communication.
type MQTTTransport struct {
	client  mqtt.Client
	qos     byte
	timeout time.Duration // Added timeout field
}

// NewMQTTTransport creates a new instance of MQTTTransport.
// The publish timeout is now configurable.
func NewMQTTTransport(client mqtt.Client, timeout time.Duration) *MQTTTransport {
	return &MQTTTransport{
		client:  client,
		qos:     1, // Default to QoS 1 (at least once delivery)
		timeout: timeout,
	}
}

// Send publishes a payload to a given MQTT topic.
func (mt *MQTTTransport) Send(ctx context.Context, topic string, payload []byte) error {
	if !mt.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	log.Printf("[MQTT Transport] Publishing to topic: %s", topic)
	log.Printf("[MQTT Transport] Payload size: %d bytes", len(payload))

	token := mt.client.Publish(topic, mt.qos, false, payload)

	// Use a select statement to handle context cancellation and timeout.
	select {
	case <-ctx.Done():
		return fmt.Errorf("MQTT publish cancelled by context: %w", ctx.Err())
	case <-time.After(mt.timeout):
		return fmt.Errorf("MQTT publish timed out after %v", mt.timeout)
	default:
		// token.Wait() blocks until the message is sent.
		if token.Wait() && token.Error() != nil {
			return fmt.Errorf("MQTT publish failed: %w", token.Error())
		}
	}

	log.Printf("[MQTT Transport] Message published successfully to: %s", topic)
	return nil
}

// GetTransportType returns the transport's type.
func (mt *MQTTTransport) GetTransportType() TransportType {
	return TransportTypeMQTT
}

// Close disconnects the MQTT client.
func (mt *MQTTTransport) Close() error {
	// The MQTT client's lifecycle is managed in the main package,
	// so this method doesn't need to do anything.
	// It's here to satisfy the interface.
	return nil
}

// SetQoS allows changing the Quality of Service for subsequent messages.
func (mt *MQTTTransport) SetQoS(qos byte) {
	if qos > 2 {
		qos = 2 // QoS can only be 0, 1, or 2.
	}
	mt.qos = qos
	log.Printf("[MQTT Transport] QoS set to: %d", qos)
}
