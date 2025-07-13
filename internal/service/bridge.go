package service

import (
	"context"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/database"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/mqtt"
	"mqtt-bridge/internal/utils"
	"time"

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
	cancelFunc  context.CancelFunc
}

func NewBridgeService(
	db *gorm.DB,
	redisClient *redis.Client,
	mqttClient mqttLib.Client,
	cfg *config.Config,
) *BridgeService {
	// 메시지 핸들러 생성
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
	// 컨텍스트 생성
	serviceCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	// MQTT 토픽 구독
	if err := s.subscribeToTopics(); err != nil {
		return err
	}

	// 백그라운드 작업 시작
	go s.startBackgroundTasks(serviceCtx)

	// 시작 완료 로그
	utils.Logger.Info("MQTT Bridge service started successfully")

	// 컨텍스트 취소 대기
	go func() {
		<-serviceCtx.Done()
		s.cleanup()
	}()

	return nil
}

func (s *BridgeService) subscribeToTopics() error {
	subscriptions := []struct {
		topic   string
		handler mqttLib.MessageHandler
		desc    string
	}{
		{
			topic:   s.config.PlcCommandTopic,
			handler: s.handler.HandleCommand,
			desc:    "PLC commands",
		},
		{
			topic:   "meili/v2/+/+/connection",
			handler: s.handler.HandleRobotConnectionState,
			desc:    "robot connection states",
		},
		{
			topic:   "meili/v2/+/+/factsheet",
			handler: s.handler.HandleRobotFactsheet,
			desc:    "robot factsheets",
		},
		{
			topic:   "meili/v2/+/+/state",
			handler: s.handler.HandleRobotState,
			desc:    "robot states",
		},
	}

	for _, sub := range subscriptions {
		token := s.mqttClient.Subscribe(sub.topic, 0, sub.handler)
		if token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to subscribe to %s: %v", sub.desc, token.Error())
		}
		utils.Logger.Infof("Subscribed to %s topic: %s", sub.desc, sub.topic)
	}

	return nil
}

func (s *BridgeService) startBackgroundTasks(ctx context.Context) {
	// 1. 타임아웃 체크 (1분마다)
	timeoutTicker := time.NewTicker(1 * time.Minute)
	defer timeoutTicker.Stop()

	// 2. 데이터 정리 (1시간마다)
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	// 3. 헬스체크 (30초마다)
	healthTicker := time.NewTicker(30 * time.Second)
	defer healthTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			utils.Logger.Info("Stopping background tasks")
			return

		case <-timeoutTicker.C:
			s.checkTimeouts()

		case <-cleanupTicker.C:
			s.cleanupOldData()

		case <-healthTicker.C:
			s.performHealthCheck()
		}
	}
}

func (s *BridgeService) checkTimeouts() {
	// 타임아웃된 명령 확인
	var timedOutCommands []models.Command

	timeout := time.Now().Add(-10 * time.Minute)
	s.db.Where("status = ? AND updated_at < ?",
		models.StatusProcessing, timeout).Find(&timedOutCommands)

	for _, cmd := range timedOutCommands {
		utils.Logger.Warnf("Command %d timed out", cmd.ID)

		// 타임아웃 처리
		now := time.Now()
		s.db.Model(&cmd).Updates(map[string]interface{}{
			"status":        models.StatusFailure,
			"response_time": now,
			"error_message": "Timeout - No response from robot",
		})

		// 로봇 상태 해제
		s.db.Model(&models.RobotStatus{}).
			Where("serial_number = ?", cmd.RobotSerialNumber).
			Updates(map[string]interface{}{
				"is_busy":            false,
				"current_command_id": nil,
				"current_order_id":   "",
			})
	}
}

func (s *BridgeService) cleanupOldData() {
	if s.config.DataRetentionDays <= 0 {
		return
	}

	utils.Logger.Info("Starting old data cleanup")

	if err := database.CleanupOldData(s.db, s.config.DataRetentionDays); err != nil {
		utils.Logger.Errorf("Failed to cleanup old data: %v", err)
	} else {
		utils.Logger.Info("Old data cleanup completed")
	}
}

func (s *BridgeService) performHealthCheck() {
	// MQTT 연결 상태 확인
	if !s.mqttClient.IsConnected() {
		utils.Logger.Error("MQTT client is disconnected")
		// 재연결은 자동으로 처리됨 (AutoReconnect 설정)
	}

	// Redis 연결 상태 확인
	if err := s.redisClient.Ping(context.Background()).Err(); err != nil {
		utils.Logger.Errorf("Redis health check failed: %v", err)
	}

	// DB 연결 상태 확인
	sqlDB, err := s.db.DB()
	if err != nil {
		utils.Logger.Errorf("Failed to get DB connection: %v", err)
		return
	}

	if err := sqlDB.Ping(); err != nil {
		utils.Logger.Errorf("Database health check failed: %v", err)
	}
}

func (s *BridgeService) cleanup() {
	utils.Logger.Info("Cleaning up bridge service")

	// MQTT 구독 해제
	topics := []string{
		s.config.PlcCommandTopic,
		"meili/v2/+/+/connection",
		"meili/v2/+/+/factsheet",
		"meili/v2/+/+/state",
	}

	for _, topic := range topics {
		if token := s.mqttClient.Unsubscribe(topic); token.Wait() && token.Error() != nil {
			utils.Logger.Errorf("Failed to unsubscribe from %s: %v", topic, token.Error())
		}
	}

	// 모든 진행 중인 명령을 실패로 처리
	var activeCommands []models.Command
	s.db.Where("status = ?", models.StatusProcessing).Find(&activeCommands)

	for _, cmd := range activeCommands {
		now := time.Now()
		s.db.Model(&cmd).Updates(map[string]interface{}{
			"status":        models.StatusFailure,
			"response_time": now,
			"error_message": "Bridge service stopped",
		})
	}

	// 모든 로봇 상태 초기화
	s.db.Model(&models.RobotStatus{}).Updates(map[string]interface{}{
		"is_busy":            false,
		"current_command_id": nil,
		"current_order_id":   "",
	})

	utils.Logger.Info("Bridge service cleanup completed")
}

func (s *BridgeService) Stop() error {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	return nil
}
