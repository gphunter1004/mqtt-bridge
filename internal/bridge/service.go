// internal/bridge/service.go
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

// Service 브릿지 서비스 (Position 도메인 제거)
type Service struct {
	db     *gorm.DB
	redis  *redisClient.Client
	config *config.Config

	// Infrastructure
	mqttClient messaging.Client
	subscriber *messaging.Subscriber
	router     *messaging.Router
	plcSender  *messaging.PLCResponseSender

	// Domain Handlers (Position 제거)
	commandHandler   *command.Handler
	robotHandler     *robot.Handler
	workflowExecutor *workflow.Executor
}

// NewService 새 브릿지 서비스 생성
func NewService(db *gorm.DB, redisClient *redisClient.Client, cfg *config.Config) (*Service, error) {
	utils.Logger.Infof("🏗️ CREATING Bridge Service")

	// 1. MQTT 클라이언트 생성
	mqttClient, err := messaging.NewMQTTClient(cfg)
	if err != nil {
		return nil, err
	}

	// 2. 통합된 PLC 응답 전송기 생성
	plcSender := messaging.NewPLCResponseSender(
		mqttClient.GetNativeClient(),
		cfg.PlcResponseTopic,
	)

	// 3. Robot Domain 생성 (Position 기능 포함)
	robotStatusManager := robot.NewStatusManager(db)
	robotFactsheetManager := robot.NewFactsheetManager(db)

	// 4. Workflow Domain 생성
	workflowExecutor := workflow.NewExecutor(
		db, redisClient, mqttClient.GetNativeClient(), cfg,
		plcSender,
	)

	// 5. Command Domain 생성
	commandProcessor := command.NewProcessor(
		db, redisClient, cfg,
		robotStatusManager,
		workflowExecutor,
	)

	commandHandler := command.NewHandler(
		db, cfg, commandProcessor, plcSender,
	)

	// 6. 🔥 순환 참조 설정: Workflow Executor에 Command Handler 참조 설정
	workflowExecutor.SetCommandHandler(commandHandler)

	// 7. Robot Handler 생성
	robotHandler := robot.NewHandler(
		robotStatusManager,
		robotFactsheetManager,
		commandHandler,               // commandFailureHandler
		mqttClient.GetNativeClient(), // MQTT 클라이언트
	)

	// 8. 메시지 라우터 생성 (Position Handler 제거)
	router := messaging.NewRouter(
		commandHandler,   // CommandHandler
		robotHandler,     // RobotHandler (Position 기능 포함)
		workflowExecutor, // WorkflowHandler
	)

	// 9. 구독자 생성
	subscriber := messaging.NewSubscriber(mqttClient, router)

	service := &Service{
		db:               db,
		redis:            redisClient,
		config:           cfg,
		mqttClient:       mqttClient,
		subscriber:       subscriber,
		router:           router,
		plcSender:        plcSender,
		commandHandler:   commandHandler,
		robotHandler:     robotHandler,
		workflowExecutor: workflowExecutor,
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
