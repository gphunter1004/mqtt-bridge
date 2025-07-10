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
	handler := mqtt.NewMessageHandler(db, redisClient, mqttClient, cfg)

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

	// 로봇 연결 상태 토픽 구독 (와일드카드 사용)
	robotConnectionTopic := "meili/v2/+/+/connection"

	token = s.mqttClient.Subscribe(robotConnectionTopic, 0, s.handler.HandleRobotConnectionState)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	utils.Logger.Infof("Subscribed to robot connection topic: %s", robotConnectionTopic)

	// 로봇 factsheet 토픽 구독 (와일드카드 사용)
	robotFactsheetTopic := "meili/v2/+/+/factsheet"

	token = s.mqttClient.Subscribe(robotFactsheetTopic, 0, s.handler.HandleRobotFactsheet)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	utils.Logger.Infof("Subscribed to robot factsheet topic: %s", robotFactsheetTopic)

	// 컨텍스트 취소 대기
	go func() {
		<-ctx.Done()
		utils.Logger.Info("Context cancelled, stopping bridge service")
	}()

	return nil
}
