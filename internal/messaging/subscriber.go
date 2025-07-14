// internal/messaging/subscriber.go
package messaging

import (
	"fmt"
	"mqtt-bridge/internal/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Subscriber MQTT 구독 관리자
type Subscriber struct {
	client Client
	router *Router
}

// NewSubscriber 새 구독자 생성
func NewSubscriber(client Client, router *Router) *Subscriber {
	utils.Logger.Infof("🏗️ CREATING MQTT Subscriber")

	subscriber := &Subscriber{
		client: client,
		router: router,
	}

	utils.Logger.Infof("✅ MQTT Subscriber CREATED")
	return subscriber
}

// SubscribeAll 모든 필요한 토픽 구독
func (s *Subscriber) SubscribeAll() error {
	utils.Logger.Infof("🔔 STARTING All Subscriptions")

	// 구독할 토픽들 정의
	subscriptions := []struct {
		topic       string
		description string
	}{
		{
			topic:       "bridge/command",
			description: "PLC Commands",
		},
		{
			topic:       "meili/v2/+/+/connection",
			description: "Robot Connection States",
		},
		{
			topic:       "meili/v2/+/+/state",
			description: "Robot States",
		},
		{
			topic:       "meili/v2/+/+/factsheet",
			description: "Robot Factsheets",
		},
		{
			topic:       "meili/v2/+/+/order",
			description: "Robot Order Responses",
		},
	}

	// 각 토픽 구독
	for _, sub := range subscriptions {
		utils.Logger.Infof("🔔 SUBSCRIBING TO: %s (%s)", sub.topic, sub.description)

		err := s.client.Subscribe(sub.topic, 0, s.handleMessage)
		if err != nil {
			utils.Logger.Errorf("❌ SUBSCRIPTION FAILED: %s - %v", sub.topic, err)
			return fmt.Errorf("failed to subscribe to %s: %v", sub.topic, err)
		}

		utils.Logger.Infof("✅ SUBSCRIPTION SUCCESS: %s", sub.topic)
	}

	utils.Logger.Infof("🎉 ALL SUBSCRIPTIONS COMPLETED")
	return nil
}

// handleMessage 수신된 메시지를 라우터에 전달
func (s *Subscriber) handleMessage(client mqtt.Client, msg mqtt.Message) {
	// 📨 수신 메시지 로그
	utils.Logger.Infof("📨 MESSAGE RECEIVED Topic  : %s", msg.Topic())
	utils.Logger.Infof("📨 MESSAGE RECEIVED Content: %s", string(msg.Payload()))

	// 라우터에 메시지 전달
	s.router.RouteMessage(client, msg)
}

// Subscribe 특정 토픽 구독
func (s *Subscriber) Subscribe(topic string, qos byte, handler MessageHandler) error {
	utils.Logger.Infof("🔔 SUBSCRIBING TO: %s", topic)

	err := s.client.Subscribe(topic, qos, handler)
	if err != nil {
		utils.Logger.Errorf("❌ SUBSCRIPTION FAILED: %s - %v", topic, err)
		return err
	}

	utils.Logger.Infof("✅ SUBSCRIPTION SUCCESS: %s", topic)
	return nil
}
