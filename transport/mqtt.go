package transport

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTTransport struct {
	client mqtt.Client
	qos    byte
}

func NewMQTTTransport(client mqtt.Client) *MQTTTransport {
	return &MQTTTransport{
		client: client,
		qos:    1, // QoS 1 (at least once delivery)
	}
}

func (mt *MQTTTransport) Send(ctx context.Context, topic string, payload []byte) error {
	if !mt.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	log.Printf("[MQTT Transport] Publishing to topic: %s", topic)
	log.Printf("[MQTT Transport] Payload size: %d bytes", len(payload))
	log.Printf("[MQTT Transport] Payload preview: %s", string(payload)[:min(200, len(payload))])

	token := mt.client.Publish(topic, mt.qos, false, payload)

	// Context와 타임아웃을 함께 처리
	done := make(chan bool)
	go func() {
		token.Wait()
		done <- true
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("MQTT publish cancelled: %w", ctx.Err())
	case <-time.After(10 * time.Second):
		return fmt.Errorf("MQTT publish timeout after 10 seconds")
	case <-done:
		if token.Error() != nil {
			return fmt.Errorf("MQTT publish failed: %w", token.Error())
		}
	}

	log.Printf("[MQTT Transport] Message published successfully to: %s", topic)
	return nil
}

func (mt *MQTTTransport) GetTransportType() TransportType {
	return TransportTypeMQTT
}

func (mt *MQTTTransport) Close() error {
	if mt.client.IsConnected() {
		mt.client.Disconnect(250)
		log.Println("[MQTT Transport] Client disconnected")
	}
	return nil
}

func (mt *MQTTTransport) SetQoS(qos byte) {
	mt.qos = qos
	log.Printf("[MQTT Transport] QoS set to: %d", qos)
}

// 헬퍼 함수
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
