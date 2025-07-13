// internal/service/bridge.go (디버깅 로그 추가)
package service

import (
	"context"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/mqtt"
	"mqtt-bridge/internal/utils"

	mqttLib "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type BridgeService struct {
	db          *gorm.DB
	redisClient *redis.Client
	mqttClient  mqttLib.Client
	config      *config.Config
	handler     *mqtt.MessageHandler
}

func NewBridgeService(db *gorm.DB, redisClient *redis.Client, mqttClient mqttLib.Client, cfg *config.Config) *BridgeService {
	utils.Logger.Infof("🏗️ CREATING BridgeService")

	handler := mqtt.NewMessageHandler(db, redisClient, mqttClient, cfg)

	service := &BridgeService{
		db:          db,
		redisClient: redisClient,
		mqttClient:  mqttClient,
		config:      cfg,
		handler:     handler,
	}

	utils.Logger.Infof("✅ BridgeService CREATED with handler: %p", handler)
	return service
}

func (s *BridgeService) Start(ctx context.Context) error {
	utils.Logger.Infof("🚀 STARTING BridgeService")

	// PLC 명령 토픽 구독
	commandTopic := "bridge/command"
	utils.Logger.Infof("🔔 SUBSCRIBING TO COMMAND: %s", commandTopic)
	utils.Logger.Infof("🎯 HANDLER ADDRESS: %p", s.handler)
	utils.Logger.Infof("🎯 COMMAND HANDLER ADDRESS: %p", s.handler.HandleCommand)

	token := s.mqttClient.Subscribe(commandTopic, 0, s.handler.HandleCommand)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("❌ COMMAND SUBSCRIPTION FAILED: %v", token.Error())
		return token.Error()
	}
	utils.Logger.Infof("✅ COMMAND SUBSCRIPTION SUCCESS: %s", commandTopic)

	// 로봇 연결 상태 토픽 구독 (와일드카드 사용)
	robotConnectionTopic := "meili/v2/+/+/connection"
	utils.Logger.Infof("🔔 SUBSCRIBING TO ROBOT CONNECTION: %s", robotConnectionTopic)
	token = s.mqttClient.Subscribe(robotConnectionTopic, 0, s.handler.HandleRobotConnectionState)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("❌ ROBOT CONNECTION SUBSCRIPTION FAILED: %v", token.Error())
		return token.Error()
	}
	utils.Logger.Infof("✅ ROBOT CONNECTION SUBSCRIPTION SUCCESS: %s", robotConnectionTopic)

	// 로봇 factsheet 토픽 구독 (와일드카드 사용)
	robotFactsheetTopic := "meili/v2/+/+/factsheet"
	utils.Logger.Infof("🔔 SUBSCRIBING TO ROBOT FACTSHEET: %s", robotFactsheetTopic)
	token = s.mqttClient.Subscribe(robotFactsheetTopic, 0, s.handler.HandleRobotFactsheet)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("❌ ROBOT FACTSHEET SUBSCRIPTION FAILED: %v", token.Error())
		return token.Error()
	}
	utils.Logger.Infof("✅ ROBOT FACTSHEET SUBSCRIPTION SUCCESS: %s", robotFactsheetTopic)

	// 로봇 상태 토픽 구독 (와일드카드 사용)
	robotStateTopic := "meili/v2/+/+/state"
	utils.Logger.Infof("🔔 SUBSCRIBING TO ROBOT STATE: %s", robotStateTopic)
	token = s.mqttClient.Subscribe(robotStateTopic, 0, s.handler.HandleRobotState)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("❌ ROBOT STATE SUBSCRIPTION FAILED: %v", token.Error())
		return token.Error()
	}
	utils.Logger.Infof("✅ ROBOT STATE SUBSCRIPTION SUCCESS: %s", robotStateTopic)

	// 컨텍스트 취소 대기
	go func() {
		<-ctx.Done()
		utils.Logger.Info("Context cancelled, stopping bridge service")
	}()

	utils.Logger.Infof("🎉 ALL SUBSCRIPTIONS COMPLETED")
	return nil
}
