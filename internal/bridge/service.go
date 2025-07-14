// internal/bridge/service.go
package bridge

import (
	"context"
	"mqtt-bridge/internal/command"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/messaging"
	"mqtt-bridge/internal/position"
	"mqtt-bridge/internal/robot"
	"mqtt-bridge/internal/utils"
	"mqtt-bridge/internal/workflow"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Service 브릿지 서비스 (전체 시스템 조합)
type Service struct {
	db     *gorm.DB
	redis  *redis.Client
	config *config.Config

	// Infrastructure
	mqttClient messaging.Client
	subscriber *messaging.Subscriber
	router     *messaging.Router

	// Domain Handlers
	commandHandler   *command.Handler
	robotHandler     *robot.Handler
	positionHandler  *position.Handler
	workflowExecutor *workflow.Executor
}

// NewService 새 브릿지 서비스 생성
func NewService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) (*Service, error) {
	utils.Logger.Infof("🏗️ CREATING Bridge Service")

	// 1. MQTT 클라이언트 생성
	mqttClient, err := messaging.NewMQTTClient(cfg)
	if err != nil {
		return nil, err
	}

	// 2. Robot Domain 생성
	robotStatusManager := robot.NewStatusManager(db)
	robotFactsheetManager := robot.NewFactsheetManager(db)

	// 3. Position Domain 생성
	positionManager := position.NewManager(db, mqttClient.GetNativeClient(), cfg)
	positionHandler := position.NewHandler(positionManager, robotFactsheetManager)

	// 4. Robot Handler 생성 (의존성 주입)
	robotHandler := robot.NewHandler(
		robotStatusManager,
		robotFactsheetManager,
		nil,             // commandFailureHandler는 나중에 설정
		positionHandler, // factsheetRequester
	)

	// 5. Workflow Domain 생성
	workflowExecutor := workflow.NewExecutor(
		db, redisClient, mqttClient.GetNativeClient(), cfg,
		nil, // commandResultSender는 나중에 설정
	)

	// 6. Command Domain 생성
	commandProcessor := command.NewProcessor(
		db, redisClient, cfg,
		robotStatusManager, // robotChecker
		workflowExecutor,   // workflowExecutor
	)

	commandHandler := command.NewHandler(
		db, cfg, commandProcessor, mqttClient,
	)

	// 7. 순환 의존성 해결 (상호 참조 설정)
	robotHandler = robot.NewHandler(
		robotStatusManager,
		robotFactsheetManager,
		commandHandler,  // commandFailureHandler
		positionHandler, // factsheetRequester
	)

	// 8. 메시지 라우터 생성
	router := messaging.NewRouter(
		commandHandler,   // CommandHandler
		robotHandler,     // RobotHandler
		positionHandler,  // PositionHandler
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
		commandHandler:   commandHandler,
		robotHandler:     robotHandler,
		positionHandler:  positionHandler,
		workflowExecutor: workflowExecutor,
	}

	utils.Logger.Infof("✅ Bridge Service CREATED")
	return service, nil
}

// Start 브릿지 서비스 시작
func (s *Service) Start(ctx context.Context) error {
	utils.Logger.Infof("🚀 STARTING Bridge Service")

	// 모든 토픽 구독
	if err := s.subscriber.SubscribeAll(); err != nil {
		return err
	}

	// 컨텍스트 취소 대기
	go func() {
		<-ctx.Done()
		utils.Logger.Info("Context cancelled, stopping bridge service")
	}()

	utils.Logger.Infof("🎉 Bridge Service STARTED Successfully")
	return nil
}

// Stop 브릿지 서비스 중지
func (s *Service) Stop() {
	utils.Logger.Info("🛑 STOPPING Bridge Service")

	// MQTT 연결 해제
	s.mqttClient.Disconnect(250)

	// Redis 연결 해제
	s.redis.Close()

	utils.Logger.Info("✅ Bridge Service STOPPED")
}

// GetCommandHandler 명령 핸들러 반환
func (s *Service) GetCommandHandler() *command.Handler {
	return s.commandHandler
}

// GetRobotHandler 로봇 핸들러 반환
func (s *Service) GetRobotHandler() *robot.Handler {
	return s.robotHandler
}

// GetPositionHandler 위치 핸들러 반환
func (s *Service) GetPositionHandler() *position.Handler {
	return s.positionHandler
}

// GetWorkflowExecutor 워크플로우 실행기 반환
func (s *Service) GetWorkflowExecutor() *workflow.Executor {
	return s.workflowExecutor
}

// GetMQTTClient MQTT 클라이언트 반환
func (s *Service) GetMQTTClient() messaging.Client {
	return s.mqttClient
}

// GetRouter 메시지 라우터 반환
func (s *Service) GetRouter() *messaging.Router {
	return s.router
}

// IsConnected 연결 상태 확인
func (s *Service) IsConnected() bool {
	return s.mqttClient.IsConnected()
}

// GetSubscriptionStatus 구독 상태 조회
func (s *Service) GetSubscriptionStatus() map[string]bool {
	return s.subscriber.GetSubscriptionStatus()
}
