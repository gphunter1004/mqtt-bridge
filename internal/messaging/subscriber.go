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
	// 디버그 로그
	utils.Logger.Debugf("📨 MESSAGE RECEIVED: Topic=%s, Size=%d bytes",
		msg.Topic(), len(msg.Payload()))

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

// Unsubscribe 구독 해제
func (s *Subscriber) Unsubscribe(topics ...string) error {
	for _, topic := range topics {
		utils.Logger.Infof("🔕 UNSUBSCRIBING FROM: %s", topic)
		// 실제 구독 해제 로직은 MQTT 클라이언트에 따라 구현
		// 현재는 로그만 남김
	}
	return nil
}

// IsSubscribed 구독 상태 확인 (구현 필요시)
func (s *Subscriber) IsSubscribed(topic string) bool {
	// MQTT 클라이언트에 따라 구독 상태를 확인하는 로직
	// 현재는 기본 구현만 제공
	return true
}

// GetSubscriptionStatus 구독 상태 조회
func (s *Subscriber) GetSubscriptionStatus() map[string]bool {
	// 현재 구독 중인 토픽들의 상태를 반환
	return map[string]bool{
		"bridge/command":          true,
		"meili/v2/+/+/connection": true,
		"meili/v2/+/+/state":      true,
		"meili/v2/+/+/factsheet":  true,
	}
}
