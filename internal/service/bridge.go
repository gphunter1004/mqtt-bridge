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
	handler := mqtt.NewMessageHandler(db, redisClient, mqttClient)

	return &BridgeService{
		db:          db,
		redisClient: redisClient,
		mqttClient:  mqttClient,
		config:      cfg,
		handler:     handler,
	}
}

func (s *BridgeService) Start(ctx context.Context) error {
	// PLC 명령 토픽 구독
	commandTopic := "bridge/command"

	token := s.mqttClient.Subscribe(commandTopic, 0, s.handler.HandleCommand)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	utils.Logger.Infof("Subscribed to topic: %s", commandTopic)

	// 컨텍스트 취소 대기
	go func() {
		<-ctx.Done()
		utils.Logger.Info("Context cancelled, stopping bridge service")
	}()

	return nil
}
