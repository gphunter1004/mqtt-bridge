// internal/interfaces/services.go
package interfaces

import (
	"context"
	"mqtt-bridge/internal/models"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// DatabaseService 데이터베이스 관련 서비스 인터페이스
type DatabaseService interface {
	// Command 관련
	GetCommandDefinition(commandType string) (*models.CommandDefinition, error)
	CreateCommand(command *models.Command) error
	UpdateCommandStatus(command *models.Command, status, errMsg string) error

	// CommandExecution 관련
	CreateCommandExecution(execution *models.CommandExecution) error
	UpdateCommandExecutionStatus(execution *models.CommandExecution, status string, completedAt *time.Time) error
	GetRunningCommandExecution() (*models.CommandExecution, error)

	// OrderExecution 관련
	CreateOrderExecution(execution *models.OrderExecution) error
	UpdateOrderExecutionStatus(execution *models.OrderExecution, status string, completedAt *time.Time) error
	GetOrderExecutionByID(id uint) (*models.OrderExecution, error)

	// StepExecution 관련
	CreateStepExecution(execution *models.StepExecution) error
	UpdateStepExecutionStatus(execution *models.StepExecution, status, result, errMsg string, completedAt *time.Time) error
	GetRunningStepExecution(orderID string) (*models.StepExecution, error)

	// CommandOrderMapping 관련
	GetCommandOrderMapping(commandDefID uint, executionOrder int) (*models.CommandOrderMapping, error)

	// RobotStatus 관련
	GetRobotStatus(serialNumber string) (*models.RobotStatus, error)
	UpdateRobotStatus(status *models.RobotStatus) error
	CreateRobotStatus(status *models.RobotStatus) error

	// RobotFactsheet 관련
	GetRobotFactsheet(serialNumber string) (*models.RobotFactsheet, error)
	CreateRobotFactsheet(factsheet *models.RobotFactsheet) error
	UpdateRobotFactsheet(factsheet *models.RobotFactsheet) error

	// Workflow 관련
	GetOrderTemplateWithSteps(templateID uint) (*models.OrderTemplate, error)

	// Batch operations
	FailAllProcessingCommands(reason string) error
	CancelAllRunningOrders() error
}

// CacheService Redis 캐시 관련 서비스 인터페이스
type CacheService interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error

	// Hash operations for action status tracking
	HSet(ctx context.Context, key, field string, value interface{}) error
	HGet(ctx context.Context, key, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) error

	// Pipeline operations
	Pipeline() CachePipeline
}

// CachePipeline Redis 파이프라인 인터페이스
type CachePipeline interface {
	HSet(ctx context.Context, key, field string, value interface{}) error
	Exec(ctx context.Context) error
}

// MessagePublisher MQTT 메시지 발행 인터페이스
type MessagePublisher interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) error
	Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error
	IsConnected() bool
	Disconnect(quiesce uint)
}

// OrderMessageBuilder 오더 메시지 생성 인터페이스
type OrderMessageBuilder interface {
	BuildOrderMessage(execution *models.OrderExecution, step *models.OrderStep) *models.OrderMessage
	SendOrder(orderMsg *models.OrderMessage) error
	SendCancelOrder() error
	GenerateOrderID() string
	GenerateActionID() string
}

// ConfigProvider 설정 제공 인터페이스
type ConfigProvider interface {
	GetRobotSerialNumber() string
	GetRobotManufacturer() string
	GetPlcResponseTopic() string
	GetLogLevel() string
	GetTimeout() time.Duration
}

// Logger 로깅 인터페이스
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

// HeaderIDGenerator 헤더 ID 생성 인터페이스
type HeaderIDGenerator interface {
	GetNextHeaderID() int64
}

// UniqueIDGenerator 고유 ID 생성 인터페이스
type UniqueIDGenerator interface {
	GenerateUniqueID() string
}
