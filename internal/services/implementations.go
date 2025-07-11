// internal/services/implementations.go
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/interfaces"
	"mqtt-bridge/internal/models"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// =============================================================================
// Database Service Implementation
// =============================================================================

type DatabaseServiceImpl struct {
	db *gorm.DB
}

func NewDatabaseService(db *gorm.DB) interfaces.DatabaseService {
	return &DatabaseServiceImpl{db: db}
}

// Command 관련 메서드들
func (d *DatabaseServiceImpl) GetCommandDefinition(commandType string) (*models.CommandDefinition, error) {
	var cmdDef models.CommandDefinition
	err := d.db.Where("command_type = ? AND is_active = true", commandType).First(&cmdDef).Error
	return &cmdDef, err
}

func (d *DatabaseServiceImpl) CreateCommand(command *models.Command) error {
	return d.db.Create(command).Error
}

func (d *DatabaseServiceImpl) UpdateCommandStatus(command *models.Command, status, errMsg string) error {
	command.Status = status
	command.ErrorMessage = errMsg
	now := time.Now()
	command.ResponseTime = &now
	return d.db.Save(command).Error
}

// CommandExecution 관련 메서드들
func (d *DatabaseServiceImpl) CreateCommandExecution(execution *models.CommandExecution) error {
	return d.db.Create(execution).Error
}

func (d *DatabaseServiceImpl) UpdateCommandExecutionStatus(execution *models.CommandExecution, status string, completedAt *time.Time) error {
	execution.Status = status
	if completedAt != nil {
		execution.CompletedAt = completedAt
	}
	return d.db.Save(execution).Error
}

func (d *DatabaseServiceImpl) GetRunningCommandExecution() (*models.CommandExecution, error) {
	var execution models.CommandExecution
	err := d.db.Where("status = ?", models.CommandExecutionStatusRunning).
		Preload("Command.CommandDefinition").First(&execution).Error
	return &execution, err
}

// OrderExecution 관련 메서드들
func (d *DatabaseServiceImpl) CreateOrderExecution(execution *models.OrderExecution) error {
	return d.db.Create(execution).Error
}

func (d *DatabaseServiceImpl) UpdateOrderExecutionStatus(execution *models.OrderExecution, status string, completedAt *time.Time) error {
	execution.Status = status
	if completedAt != nil {
		execution.CompletedAt = completedAt
	}
	return d.db.Save(execution).Error
}

func (d *DatabaseServiceImpl) GetOrderExecutionByID(id uint) (*models.OrderExecution, error) {
	var execution models.OrderExecution
	err := d.db.Preload("Template").First(&execution, id).Error
	return &execution, err
}

// StepExecution 관련 메서드들
func (d *DatabaseServiceImpl) CreateStepExecution(execution *models.StepExecution) error {
	return d.db.Create(execution).Error
}

func (d *DatabaseServiceImpl) UpdateStepExecutionStatus(execution *models.StepExecution, status, result, errMsg string, completedAt *time.Time) error {
	execution.Status = status
	execution.Result = result
	if errMsg != "" {
		execution.ErrorMessage = errMsg
	}
	if completedAt != nil {
		execution.CompletedAt = completedAt
	}
	return d.db.Save(execution).Error
}

func (d *DatabaseServiceImpl) GetRunningStepExecution(orderID string) (*models.StepExecution, error) {
	var stepExecution models.StepExecution
	err := d.db.Joins("JOIN order_executions ON step_executions.execution_id = order_executions.id").
		Where("order_executions.order_id = ? AND step_executions.status = ?", orderID, models.StepExecutionStatusRunning).
		Preload("Execution.Template").First(&stepExecution).Error
	return &stepExecution, err
}

// CommandOrderMapping 관련 메서드들
func (d *DatabaseServiceImpl) GetCommandOrderMapping(commandDefID uint, executionOrder int) (*models.CommandOrderMapping, error) {
	var mapping models.CommandOrderMapping
	err := d.db.Where("command_definition_id = ? AND execution_order = ?", commandDefID, executionOrder).
		Preload("Template.OrderSteps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Preload("Template.OrderSteps.StepActionMappings.ActionTemplate.Parameters").
		First(&mapping).Error
	return &mapping, err
}

// RobotStatus 관련 메서드들
func (d *DatabaseServiceImpl) GetRobotStatus(serialNumber string) (*models.RobotStatus, error) {
	var status models.RobotStatus
	err := d.db.Where("serial_number = ?", serialNumber).First(&status).Error
	return &status, err
}

func (d *DatabaseServiceImpl) UpdateRobotStatus(status *models.RobotStatus) error {
	return d.db.Save(status).Error
}

func (d *DatabaseServiceImpl) CreateRobotStatus(status *models.RobotStatus) error {
	return d.db.Create(status).Error
}

// RobotFactsheet 관련 메서드들
func (d *DatabaseServiceImpl) GetRobotFactsheet(serialNumber string) (*models.RobotFactsheet, error) {
	var factsheet models.RobotFactsheet
	err := d.db.Where("serial_number = ?", serialNumber).First(&factsheet).Error
	return &factsheet, err
}

func (d *DatabaseServiceImpl) CreateRobotFactsheet(factsheet *models.RobotFactsheet) error {
	return d.db.Create(factsheet).Error
}

func (d *DatabaseServiceImpl) UpdateRobotFactsheet(factsheet *models.RobotFactsheet) error {
	return d.db.Save(factsheet).Error
}

// Workflow 관련 메서드들
func (d *DatabaseServiceImpl) GetOrderTemplateWithSteps(templateID uint) (*models.OrderTemplate, error) {
	var template models.OrderTemplate
	err := d.db.Where("id = ?", templateID).
		Preload("OrderSteps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Preload("OrderSteps.StepActionMappings.ActionTemplate.Parameters").
		First(&template).Error
	return &template, err
}

// Batch operations
func (d *DatabaseServiceImpl) FailAllProcessingCommands(reason string) error {
	return d.db.Model(&models.Command{}).
		Where("status = ?", models.StatusProcessing).
		Updates(map[string]interface{}{
			"status":        models.StatusFailure,
			"error_message": reason,
			"response_time": time.Now(),
		}).Error
}

func (d *DatabaseServiceImpl) CancelAllRunningOrders() error {
	now := time.Now()

	// CommandExecution 취소
	if err := d.db.Model(&models.CommandExecution{}).
		Where("status = ?", models.CommandExecutionStatusRunning).
		Updates(map[string]interface{}{
			"status":       models.CommandExecutionStatusCancelled,
			"completed_at": now,
		}).Error; err != nil {
		return err
	}

	// OrderExecution 취소
	if err := d.db.Model(&models.OrderExecution{}).
		Where("status IN ?", []string{models.OrderExecutionStatusRunning, models.OrderExecutionStatusPending}).
		Updates(map[string]interface{}{
			"status":       models.OrderExecutionStatusFailed,
			"completed_at": now,
		}).Error; err != nil {
		return err
	}

	// StepExecution 취소
	return d.db.Model(&models.StepExecution{}).
		Where("status = ?", models.StepExecutionStatusRunning).
		Updates(map[string]interface{}{
			"status":        models.StepExecutionStatusFailed,
			"error_message": "Cancelled by order cancel command",
			"completed_at":  now,
		}).Error
}

// =============================================================================
// Cache Service Implementation
// =============================================================================

type CacheServiceImpl struct {
	client *redis.Client
}

func NewCacheService(client *redis.Client) interfaces.CacheService {
	return &CacheServiceImpl{client: client}
}

func (c *CacheServiceImpl) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *CacheServiceImpl) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *CacheServiceImpl) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *CacheServiceImpl) HSet(ctx context.Context, key, field string, value interface{}) error {
	return c.client.HSet(ctx, key, field, value).Err()
}

func (c *CacheServiceImpl) HGet(ctx context.Context, key, field string) (string, error) {
	return c.client.HGet(ctx, key, field).Result()
}

func (c *CacheServiceImpl) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

func (c *CacheServiceImpl) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
}

func (c *CacheServiceImpl) Pipeline() interfaces.CachePipeline {
	return &CachePipelineImpl{pipeline: c.client.Pipeline()}
}

type CachePipelineImpl struct {
	pipeline redis.Pipeliner
}

func (c *CachePipelineImpl) HSet(ctx context.Context, key, field string, value interface{}) error {
	c.pipeline.HSet(ctx, key, field, value)
	return nil
}

func (c *CachePipelineImpl) Exec(ctx context.Context) error {
	_, err := c.pipeline.Exec(ctx)
	return err
}

// =============================================================================
// Message Publisher Implementation
// =============================================================================

type MessagePublisherImpl struct {
	client mqtt.Client
}

func NewMessagePublisher(client mqtt.Client) interfaces.MessagePublisher {
	return &MessagePublisherImpl{client: client}
}

func (m *MessagePublisherImpl) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	token := m.client.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}

func (m *MessagePublisherImpl) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	token := m.client.Subscribe(topic, qos, callback)
	token.Wait()
	return token.Error()
}

func (m *MessagePublisherImpl) IsConnected() bool {
	return m.client.IsConnected()
}

func (m *MessagePublisherImpl) Disconnect(quiesce uint) {
	m.client.Disconnect(quiesce)
}

// =============================================================================
// Order Message Builder Implementation
// =============================================================================

type OrderMessageBuilderImpl struct {
	publisher   interfaces.MessagePublisher
	config      interfaces.ConfigProvider
	headerIDGen interfaces.HeaderIDGenerator
	uniqueIDGen interfaces.UniqueIDGenerator
}

func NewOrderMessageBuilder(
	publisher interfaces.MessagePublisher,
	config interfaces.ConfigProvider,
	headerIDGen interfaces.HeaderIDGenerator,
	uniqueIDGen interfaces.UniqueIDGenerator,
) interfaces.OrderMessageBuilder {
	return &OrderMessageBuilderImpl{
		publisher:   publisher,
		config:      config,
		headerIDGen: headerIDGen,
		uniqueIDGen: uniqueIDGen,
	}
}

func (o *OrderMessageBuilderImpl) BuildOrderMessage(execution *models.OrderExecution, step *models.OrderStep) *models.OrderMessage {
	node := o.buildOrderNode(step)

	return &models.OrderMessage{
		HeaderID:      o.headerIDGen.GetNextHeaderID(),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Version:       "2.0.0",
		Manufacturer:  o.config.GetRobotManufacturer(),
		SerialNumber:  o.config.GetRobotSerialNumber(),
		OrderID:       execution.OrderID,
		OrderUpdateID: 0,
		Nodes:         []models.OrderNode{node},
		Edges:         []models.OrderEdge{},
	}
}

func (o *OrderMessageBuilderImpl) buildOrderNode(step *models.OrderStep) models.OrderNode {
	// StepActionMappings를 ExecutionOrder 순서로 정렬
	sort.Slice(step.StepActionMappings, func(i, j int) bool {
		return step.StepActionMappings[i].ExecutionOrder < step.StepActionMappings[j].ExecutionOrder
	})

	actions := make([]models.OrderAction, 0, len(step.StepActionMappings))
	for _, mapping := range step.StepActionMappings {
		actionTemplate := mapping.ActionTemplate
		action := models.OrderAction{
			ActionType:        actionTemplate.ActionType,
			ActionID:          o.GenerateActionID(),
			ActionDescription: actionTemplate.ActionDescription,
			BlockingType:      actionTemplate.BlockingType,
			ActionParameters:  o.buildActionParameters(actionTemplate.Parameters),
		}
		actions = append(actions, action)
	}

	return models.OrderNode{
		NodeID:     fmt.Sprintf("node_%d", step.StepOrder),
		SequenceID: step.StepOrder,
		Released:   true,
		NodePosition: models.NodePosition{
			X: 0.0, Y: 0.0, Theta: 0.0,
			AllowedDeviationXY: 0.0, AllowedDeviationTheta: 0.0,
			MapID: "",
		},
		Actions: actions,
	}
}

func (o *OrderMessageBuilderImpl) buildActionParameters(params []models.ActionParameter) []models.OrderActionParameter {
	actionParams := make([]models.OrderActionParameter, 0, len(params))

	for _, param := range params {
		var value interface{}
		switch param.ValueType {
		case "NUMBER":
			if floatVal, err := strconv.ParseFloat(param.Value, 64); err == nil {
				value = floatVal
			} else {
				value = param.Value
			}
		case "BOOLEAN":
			if boolVal, err := strconv.ParseBool(param.Value); err == nil {
				value = boolVal
			} else {
				value = param.Value
			}
		default:
			value = param.Value
		}

		actionParams = append(actionParams, models.OrderActionParameter{
			Key:   param.Key,
			Value: value,
		})
	}

	return actionParams
}

func (o *OrderMessageBuilderImpl) SendOrder(orderMsg *models.OrderMessage) error {
	topic := fmt.Sprintf("meili/v2/%s/%s/order", orderMsg.Manufacturer, orderMsg.SerialNumber)

	msgData, err := json.Marshal(orderMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %v", err)
	}

	return o.publisher.Publish(topic, 0, false, msgData)
}

func (o *OrderMessageBuilderImpl) SendCancelOrder() error {
	actionID := o.GenerateActionID()

	request := map[string]interface{}{
		"headerId":     o.headerIDGen.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": o.config.GetRobotManufacturer(),
		"serialNumber": o.config.GetRobotSerialNumber(),
		"actions": []map[string]interface{}{
			{
				"actionType":       "cancelOrder",
				"actionId":         actionID,
				"blockingType":     "HARD",
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	reqData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal cancelOrder request: %v", err)
	}

	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions",
		o.config.GetRobotManufacturer(), o.config.GetRobotSerialNumber())

	return o.publisher.Publish(topic, 0, false, reqData)
}

func (o *OrderMessageBuilderImpl) GenerateOrderID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (o *OrderMessageBuilderImpl) GenerateActionID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

// =============================================================================
// Config Provider Implementation
// =============================================================================

type ConfigProviderImpl struct {
	cfg *config.Config
}

func NewConfigProvider(cfg *config.Config) interfaces.ConfigProvider {
	return &ConfigProviderImpl{cfg: cfg}
}

func (c *ConfigProviderImpl) GetRobotSerialNumber() string {
	return c.cfg.RobotSerialNumber
}

func (c *ConfigProviderImpl) GetRobotManufacturer() string {
	return c.cfg.RobotManufacturer
}

func (c *ConfigProviderImpl) GetPlcResponseTopic() string {
	return c.cfg.PlcResponseTopic
}

func (c *ConfigProviderImpl) GetLogLevel() string {
	return c.cfg.LogLevel
}

func (c *ConfigProviderImpl) GetTimeout() time.Duration {
	return c.cfg.Timeout
}

// =============================================================================
// Logger Implementation
// =============================================================================

type LoggerImpl struct {
	logger *logrus.Logger
}

func NewLogger(level string) interfaces.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	return &LoggerImpl{logger: logger}
}

func (l *LoggerImpl) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

func (l *LoggerImpl) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *LoggerImpl) Info(args ...interface{}) {
	l.logger.Info(args...)
}

func (l *LoggerImpl) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *LoggerImpl) Warn(args ...interface{}) {
	l.logger.Warn(args...)
}

func (l *LoggerImpl) Warnf(format string, args ...interface{}) {
	l.logger.Warnf(format, args...)
}

func (l *LoggerImpl) Error(args ...interface{}) {
	l.logger.Error(args...)
}

func (l *LoggerImpl) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

func (l *LoggerImpl) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

func (l *LoggerImpl) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}

// =============================================================================
// ID Generators Implementation
// =============================================================================

type HeaderIDGeneratorImpl struct {
	counter int64
}

func NewHeaderIDGenerator() interfaces.HeaderIDGenerator {
	return &HeaderIDGeneratorImpl{}
}

func (h *HeaderIDGeneratorImpl) GetNextHeaderID() int64 {
	return atomic.AddInt64(&h.counter, 1)
}

type UniqueIDGeneratorImpl struct{}

func NewUniqueIDGenerator() interfaces.UniqueIDGenerator {
	return &UniqueIDGeneratorImpl{}
}

func (u *UniqueIDGeneratorImpl) GenerateUniqueID() string {
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return fmt.Sprintf("%s_%d", hex.EncodeToString(randomBytes), time.Now().UnixNano())
}
