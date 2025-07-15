// internal/bridge/service.go (이전과 동일)
package bridge

import (
	"context"
	"mqtt-bridge/internal/command"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/messaging"
	"mqtt-bridge/internal/robot"
	"mqtt-bridge/internal/utils"
	"mqtt-bridge/internal/workflow"

	redisClient "github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Service 브릿지 서비스
type Service struct {
	db             *gorm.DB
	redis          *redisClient.Client
	config         *config.Config
	mqttClient     messaging.Client
	subscriber     *messaging.Subscriber
	router         *messaging.Router
	commandHandler command.CommandHandler
	robotHandler   *robot.Handler
}

// NewService 새 브릿지 서비스 생성
func NewService(db *gorm.DB, redisClient *redisClient.Client, cfg *config.Config) (*Service, error) {
	utils.Logger.Infof("🏗️ CREATING Bridge Service")

	mqttClient, err := messaging.NewMQTTClient(cfg)
	if err != nil {
		return nil, err
	}
	plcSender := messaging.NewPLCResponseSender(mqttClient.GetNativeClient(), cfg.PlcResponseTopic)

	// --- Domain Dependencies ---
	robotStatusManager := robot.NewStatusManager(db)
	robotFactsheetManager := robot.NewFactsheetManager(db)

	workflowExecutor := workflow.NewExecutor(
		db, redisClient, mqttClient.GetNativeClient(), cfg, plcSender,
	)

	commandHandler := command.NewHandler(
		db, cfg, plcSender, workflowExecutor, robotStatusManager,
	)

	workflowExecutor.SetCommandHandler(commandHandler)

	robotHandler := robot.NewHandler(
		robotStatusManager, robotFactsheetManager, commandHandler, mqttClient.GetNativeClient(),
	)

	// --- Messaging ---
	router := messaging.NewRouter(commandHandler, robotHandler, workflowExecutor)
	subscriber := messaging.NewSubscriber(mqttClient, router)

	service := &Service{
		db:             db,
		redis:          redisClient,
		config:         cfg,
		mqttClient:     mqttClient,
		subscriber:     subscriber,
		router:         router,
		commandHandler: commandHandler,
		robotHandler:   robotHandler,
	}

	utils.Logger.Infof("✅ Bridge Service CREATED")
	return service, nil
}

// Start 브릿지 서비스 시작
func (s *Service) Start(ctx context.Context) error {
	utils.Logger.Infof("🚀 STARTING Bridge Service")
	if err := s.subscriber.SubscribeAll(); err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		utils.Logger.Info("Context cancelled, stopping bridge service")
	}()
	return nil
}

// Stop 브릿지 서비스 중지
func (s *Service) Stop() {
	utils.Logger.Info("🛑 STOPPING Bridge Service")
	s.mqttClient.Disconnect(250)
	s.redis.Close()
	utils.Logger.Info("✅ Bridge Service STOPPED")
}
