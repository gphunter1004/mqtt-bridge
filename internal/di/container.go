// internal/di/container.go
package di

import (
	"context"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/database"
	"mqtt-bridge/internal/handlers"
	"mqtt-bridge/internal/interfaces"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/redis"
	"mqtt-bridge/internal/services"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Container 의존성 주입 컨테이너
type Container struct {
	// Core Services
	Database         interfaces.DatabaseService
	Cache            interfaces.CacheService
	MessagePublisher interfaces.MessagePublisher
	OrderBuilder     interfaces.OrderMessageBuilder
	Config           interfaces.ConfigProvider
	Logger           interfaces.Logger
	HeaderIDGen      interfaces.HeaderIDGenerator
	UniqueIDGen      interfaces.UniqueIDGenerator

	// Handlers
	CommandHandler *handlers.CommandHandler
	RobotHandler   *handlers.RobotHandler
	OrderExecutor  *handlers.OrderExecutor
	MessageHandler *handlers.MessageHandler

	// Service
	BridgeService *BridgeService
}

// NewContainer 새로운 컨테이너 생성
func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{}

	// 1. 기본 서비스들 초기화
	if err := container.initCoreServices(cfg); err != nil {
		return nil, fmt.Errorf("failed to init core services: %v", err)
	}

	// 2. 인프라 서비스들 초기화
	if err := container.initInfraServices(cfg); err != nil {
		return nil, fmt.Errorf("failed to init infra services: %v", err)
	}

	// 3. 비즈니스 서비스들 초기화
	if err := container.initBusinessServices(); err != nil {
		return nil, fmt.Errorf("failed to init business services: %v", err)
	}

	// 4. 핸들러들 초기화
	if err := container.initHandlers(); err != nil {
		return nil, fmt.Errorf("failed to init handlers: %v", err)
	}

	// 5. 브릿지 서비스 초기화
	container.BridgeService = NewBridgeService(container)

	return container, nil
}

// initCoreServices 핵심 서비스들 초기화
func (c *Container) initCoreServices(cfg *config.Config) error {
	c.Config = services.NewConfigProvider(cfg)
	c.Logger = services.NewLogger(cfg.LogLevel)
	c.HeaderIDGen = services.NewHeaderIDGenerator()
	c.UniqueIDGen = services.NewUniqueIDGenerator()

	return nil
}

// initInfraServices 인프라 서비스들 초기화
func (c *Container) initInfraServices(cfg *config.Config) error {
	// Database 초기화
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		return fmt.Errorf("database init failed: %v", err)
	}
	c.Database = services.NewDatabaseService(db)

	// Redis 초기화
	redisClient, err := redis.NewRedisClient(cfg)
	if err != nil {
		return fmt.Errorf("redis init failed: %v", err)
	}
	c.Cache = services.NewCacheService(redisClient)

	// MQTT 초기화
	mqttClient, err := createMQTTClient(cfg)
	if err != nil {
		return fmt.Errorf("mqtt init failed: %v", err)
	}
	c.MessagePublisher = services.NewMessagePublisher(mqttClient)

	return nil
}

// initBusinessServices 비즈니스 서비스들 초기화
func (c *Container) initBusinessServices() error {
	c.OrderBuilder = services.NewOrderMessageBuilder(
		c.MessagePublisher,
		c.Config,
		c.HeaderIDGen,
		c.UniqueIDGen,
	)

	return nil
}

// initHandlers 핸들러들 초기화
func (c *Container) initHandlers() error {
	// OrderExecutor 먼저 생성
	c.OrderExecutor = handlers.NewOrderExecutor(
		c.Database,
		c.Cache,
		c.OrderBuilder,
		c.Config,
		c.Logger,
	)

	// CommandHandler 생성 (OrderExecutor 의존)
	c.CommandHandler = handlers.NewCommandHandler(
		c.Database,
		c.Cache,
		c.MessagePublisher,
		c.OrderExecutor,
		c.Config,
		c.Logger,
	)

	// RobotHandler 생성 (CommandHandler 의존)
	c.RobotHandler = handlers.NewRobotHandler(
		c.Database,
		c.CommandHandler,
		c.Config,
		c.Logger,
	)

	// MessageHandler 생성 (모든 핸들러 의존)
	c.MessageHandler = handlers.NewMessageHandler(
		c.CommandHandler,
		c.RobotHandler,
		c.OrderExecutor,
	)

	return nil
}

// createMQTTClient MQTT 클라이언트 생성
func createMQTTClient(cfg *config.Config) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTBroker)
	opts.SetClientID(cfg.MQTTClientID)
	opts.SetUsername(cfg.MQTTUsername)
	opts.SetPassword(cfg.MQTTPassword)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		// Logger는 아직 초기화되지 않을 수 있으므로 직접 로그
		fmt.Println("MQTT client connected")
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		fmt.Printf("MQTT connection lost: %v\n", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return client, nil
}

// Cleanup 리소스 정리
func (c *Container) Cleanup() {
	if c.MessagePublisher != nil {
		c.MessagePublisher.Disconnect(250)
	}
	c.Logger.Infof("Container cleanup completed")
}

// =============================================================================
// Bridge Service
// =============================================================================

type BridgeService struct {
	container *Container
}

func NewBridgeService(container *Container) *BridgeService {
	return &BridgeService{container: container}
}

// Start 브릿지 서비스 시작
func (s *BridgeService) Start(ctx context.Context) error {
	// MQTT 토픽 구독 설정
	subscriptions := map[string]mqtt.MessageHandler{
		"bridge/command":          s.container.MessageHandler.HandleCommand,
		"meili/v2/+/+/connection": s.container.MessageHandler.HandleRobotConnectionState,
		"meili/v2/+/+/state":      s.container.MessageHandler.HandleRobotState,
	}

	// 토픽들 구독
	for topic, handler := range subscriptions {
		if err := s.container.MessagePublisher.Subscribe(topic, 0, handler); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %v", topic, err)
		}
		s.container.Logger.Infof("✅ Subscribed to topic: %s", topic)
	}

	s.container.Logger.Infof("🚀 MQTT Bridge service started successfully")

	// 컨텍스트 취소 신호 대기
	go func() {
		<-ctx.Done()
		s.container.Logger.Infof("💤 Bridge service context cancelled")
	}()

	return nil
}

// GetHealthStatus 헬스 체크 상태 반환
func (s *BridgeService) GetHealthStatus() map[string]interface{} {
	return map[string]interface{}{
		"mqtt_connected": s.container.MessagePublisher.IsConnected(),
		"timestamp":      time.Now().Format(time.RFC3339),
		"status":         "running",
	}
}

// =============================================================================
// 팩토리 함수들 (테스트용)
// =============================================================================

// NewTestContainer 테스트용 컨테이너 생성 (Mock 의존성 사용)
func NewTestContainer(
	database interfaces.DatabaseService,
	cache interfaces.CacheService,
	messagePublisher interfaces.MessagePublisher,
	logger interfaces.Logger,
	config interfaces.ConfigProvider,
) *Container {
	container := &Container{
		Database:         database,
		Cache:            cache,
		MessagePublisher: messagePublisher,
		Logger:           logger,
		Config:           config,
		HeaderIDGen:      services.NewHeaderIDGenerator(),
		UniqueIDGen:      services.NewUniqueIDGenerator(),
	}

	// OrderBuilder 초기화
	container.OrderBuilder = services.NewOrderMessageBuilder(
		container.MessagePublisher,
		container.Config,
		container.HeaderIDGen,
		container.UniqueIDGen,
	)

	// 핸들러들 초기화
	container.OrderExecutor = handlers.NewOrderExecutor(
		container.Database,
		container.Cache,
		container.OrderBuilder,
		container.Config,
		container.Logger,
	)

	container.CommandHandler = handlers.NewCommandHandler(
		container.Database,
		container.Cache,
		container.MessagePublisher,
		container.OrderExecutor,
		container.Config,
		container.Logger,
	)

	container.RobotHandler = handlers.NewRobotHandler(
		container.Database,
		container.CommandHandler,
		container.Config,
		container.Logger,
	)

	container.MessageHandler = handlers.NewMessageHandler(
		container.CommandHandler,
		container.RobotHandler,
		container.OrderExecutor,
	)

	container.BridgeService = NewBridgeService(container)

	return container
}

// NewMockContainer Mock 구현체들로 구성된 테스트 컨테이너 생성
func NewMockContainer() *Container {
	mockConfig := &MockConfigProvider{
		robotSerialNumber: "TEST001",
		robotManufacturer: "TestMfg",
		plcResponseTopic:  "bridge/response",
		logLevel:          "debug",
		timeout:           30 * time.Second,
	}

	return NewTestContainer(
		NewMockDatabaseService(),
		NewMockCacheService(),
		NewMockMessagePublisher(),
		NewMockLogger(),
		mockConfig,
	)
}

// =============================================================================
// Mock 구현체들 (테스트용)
// =============================================================================

type MockConfigProvider struct {
	robotSerialNumber string
	robotManufacturer string
	plcResponseTopic  string
	logLevel          string
	timeout           time.Duration
}

func (m *MockConfigProvider) GetRobotSerialNumber() string { return m.robotSerialNumber }
func (m *MockConfigProvider) GetRobotManufacturer() string { return m.robotManufacturer }
func (m *MockConfigProvider) GetPlcResponseTopic() string  { return m.plcResponseTopic }
func (m *MockConfigProvider) GetLogLevel() string          { return m.logLevel }
func (m *MockConfigProvider) GetTimeout() time.Duration    { return m.timeout }

type MockDatabaseService struct {
	commands      []models.Command
	robotStatuses map[string]*models.RobotStatus
	cmdDefs       map[string]*models.CommandDefinition
}

func NewMockDatabaseService() *MockDatabaseService {
	return &MockDatabaseService{
		commands:      make([]models.Command, 0),
		robotStatuses: make(map[string]*models.RobotStatus),
		cmdDefs: map[string]*models.CommandDefinition{
			"CR": {ID: 1, CommandType: "CR", Description: "백내장 적출", IsActive: true},
			"GR": {ID: 2, CommandType: "GR", Description: "적내장 적출", IsActive: true},
			"OC": {ID: 3, CommandType: "OC", Description: "명령 취소", IsActive: true},
		},
	}
}

func (m *MockDatabaseService) GetCommandDefinition(commandType string) (*models.CommandDefinition, error) {
	if cmd, exists := m.cmdDefs[commandType]; exists {
		return cmd, nil
	}
	return nil, fmt.Errorf("command not found")
}

func (m *MockDatabaseService) CreateCommand(command *models.Command) error {
	command.ID = uint(len(m.commands) + 1)
	m.commands = append(m.commands, *command)
	return nil
}

func (m *MockDatabaseService) UpdateCommandStatus(command *models.Command, status, errMsg string) error {
	command.Status = status
	command.ErrorMessage = errMsg
	now := time.Now()
	command.ResponseTime = &now
	return nil
}

func (m *MockDatabaseService) GetRobotStatus(serialNumber string) (*models.RobotStatus, error) {
	if status, exists := m.robotStatuses[serialNumber]; exists {
		return status, nil
	}
	return nil, fmt.Errorf("robot not found")
}

func (m *MockDatabaseService) UpdateRobotStatus(status *models.RobotStatus) error {
	m.robotStatuses[status.SerialNumber] = status
	return nil
}

func (m *MockDatabaseService) CreateRobotStatus(status *models.RobotStatus) error {
	m.robotStatuses[status.SerialNumber] = status
	return nil
}

// 나머지 DatabaseService 메서드들을 위한 기본 구현
func (m *MockDatabaseService) CreateCommandExecution(execution *models.CommandExecution) error {
	return nil
}
func (m *MockDatabaseService) UpdateCommandExecutionStatus(execution *models.CommandExecution, status string, completedAt *time.Time) error {
	return nil
}
func (m *MockDatabaseService) GetRunningCommandExecution() (*models.CommandExecution, error) {
	return nil, fmt.Errorf("not found")
}
func (m *MockDatabaseService) CreateOrderExecution(execution *models.OrderExecution) error {
	return nil
}
func (m *MockDatabaseService) UpdateOrderExecutionStatus(execution *models.OrderExecution, status string, completedAt *time.Time) error {
	return nil
}
func (m *MockDatabaseService) GetOrderExecutionByID(id uint) (*models.OrderExecution, error) {
	return nil, fmt.Errorf("not found")
}
func (m *MockDatabaseService) CreateStepExecution(execution *models.StepExecution) error { return nil }
func (m *MockDatabaseService) UpdateStepExecutionStatus(execution *models.StepExecution, status, result, errMsg string, completedAt *time.Time) error {
	return nil
}
func (m *MockDatabaseService) GetRunningStepExecution(orderID string) (*models.StepExecution, error) {
	return nil, fmt.Errorf("not found")
}
func (m *MockDatabaseService) GetCommandOrderMapping(commandDefID uint, executionOrder int) (*models.CommandOrderMapping, error) {
	return nil, fmt.Errorf("not found")
}
func (m *MockDatabaseService) GetRobotFactsheet(serialNumber string) (*models.RobotFactsheet, error) {
	return nil, fmt.Errorf("not found")
}
func (m *MockDatabaseService) CreateRobotFactsheet(factsheet *models.RobotFactsheet) error {
	return nil
}
func (m *MockDatabaseService) UpdateRobotFactsheet(factsheet *models.RobotFactsheet) error {
	return nil
}
func (m *MockDatabaseService) GetOrderTemplateWithSteps(templateID uint) (*models.OrderTemplate, error) {
	return nil, fmt.Errorf("not found")
}
func (m *MockDatabaseService) FailAllProcessingCommands(reason string) error { return nil }
func (m *MockDatabaseService) CancelAllRunningOrders() error                 { return nil }

// 테스트 헬퍼 메서드들
func (m *MockDatabaseService) GetLastCommand() *models.Command {
	if len(m.commands) == 0 {
		return nil
	}
	return &m.commands[len(m.commands)-1]
}

func (m *MockDatabaseService) SetRobotOnline(serialNumber string) {
	m.robotStatuses[serialNumber] = &models.RobotStatus{
		SerialNumber:    serialNumber,
		ConnectionState: models.ConnectionStateOnline,
	}
}

type MockCacheService struct {
	data     map[string]string
	hashData map[string]map[string]string
}

func NewMockCacheService() *MockCacheService {
	return &MockCacheService{
		data:     make(map[string]string),
		hashData: make(map[string]map[string]string),
	}
}

func (m *MockCacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.data[key] = fmt.Sprintf("%v", value)
	return nil
}

func (m *MockCacheService) Get(ctx context.Context, key string) (string, error) {
	return m.data[key], nil
}

func (m *MockCacheService) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
		delete(m.hashData, key)
	}
	return nil
}

func (m *MockCacheService) HSet(ctx context.Context, key, field string, value interface{}) error {
	if m.hashData[key] == nil {
		m.hashData[key] = make(map[string]string)
	}
	m.hashData[key][field] = fmt.Sprintf("%v", value)
	return nil
}

func (m *MockCacheService) HGet(ctx context.Context, key, field string) (string, error) {
	if hash, ok := m.hashData[key]; ok {
		return hash[field], nil
	}
	return "", nil
}

func (m *MockCacheService) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if hash, ok := m.hashData[key]; ok {
		return hash, nil
	}
	return make(map[string]string), nil
}

func (m *MockCacheService) HDel(ctx context.Context, key string, fields ...string) error {
	if hash, ok := m.hashData[key]; ok {
		for _, field := range fields {
			delete(hash, field)
		}
	}
	return nil
}

func (m *MockCacheService) Pipeline() interfaces.CachePipeline {
	return &MockCachePipeline{cache: m}
}

type MockCachePipeline struct {
	cache *MockCacheService
}

func (m *MockCachePipeline) HSet(ctx context.Context, key, field string, value interface{}) error {
	return m.cache.HSet(ctx, key, field, value)
}

func (m *MockCachePipeline) Exec(ctx context.Context) error {
	return nil
}

type MockMessagePublisher struct {
	publishedMessages []MockMessage
	subscriptions     map[string]mqtt.MessageHandler
	connected         bool
}

type MockMessage struct {
	Topic   string
	Payload interface{}
}

func NewMockMessagePublisher() *MockMessagePublisher {
	return &MockMessagePublisher{
		publishedMessages: make([]MockMessage, 0),
		subscriptions:     make(map[string]mqtt.MessageHandler),
		connected:         true,
	}
}

func (m *MockMessagePublisher) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	m.publishedMessages = append(m.publishedMessages, MockMessage{
		Topic:   topic,
		Payload: payload,
	})
	return nil
}

func (m *MockMessagePublisher) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	m.subscriptions[topic] = callback
	return nil
}

func (m *MockMessagePublisher) IsConnected() bool {
	return m.connected
}

func (m *MockMessagePublisher) Disconnect(quiesce uint) {
	m.connected = false
}

func (m *MockMessagePublisher) GetLastMessage() *MockMessage {
	if len(m.publishedMessages) == 0 {
		return nil
	}
	return &m.publishedMessages[len(m.publishedMessages)-1]
}

func (m *MockMessagePublisher) GetPublishedMessages() []MockMessage {
	return m.publishedMessages
}

type MockLogger struct {
	logs []string
}

func NewMockLogger() *MockLogger {
	return &MockLogger{logs: make([]string, 0)}
}

func (m *MockLogger) Debug(args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("DEBUG: %v", args))
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("DEBUG: "+format, args...))
}

func (m *MockLogger) Info(args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("INFO: %v", args))
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("INFO: "+format, args...))
}

func (m *MockLogger) Warn(args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("WARN: %v", args))
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("WARN: "+format, args...))
}

func (m *MockLogger) Error(args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("ERROR: %v", args))
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("ERROR: "+format, args...))
}

func (m *MockLogger) Fatal(args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("FATAL: %v", args))
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf("FATAL: "+format, args...))
}

func (m *MockLogger) ContainsLog(substring string) bool {
	for _, log := range m.logs {
		if strings.Contains(log, substring) {
			return true
		}
	}
	return false
}

func (m *MockLogger) GetLogs() []string {
	return m.logs
}
