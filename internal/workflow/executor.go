// internal/workflow/executor.go
package workflow

import (
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/repository"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// CommandResultSender 명령 결과 전송 인터페이스
type CommandResultSender interface {
	SendResponseToPLC(command, status, errMsg string)
}

// Executor 워크플로우 실행 엔진
type Executor struct {
	db                  *gorm.DB
	redisClient         *redis.Client
	mqttClient          mqtt.Client
	config              *config.Config
	orderBuilder        *OrderBuilder
	stepManager         *StepManager
	commandResultSender CommandResultSender
}

// NewExecutor 새 워크플로우 실행기 생성
func NewExecutor(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config,
	commandResultSender CommandResultSender) *Executor {

	utils.Logger.Infof("🏗️ CREATING Workflow Executor")

	orderBuilder := NewOrderBuilder(cfg)
	messageSender := &MQTTMessageSender{
		mqttClient: mqttClient,
		config:     cfg,
	}
	stepManager := NewStepManager(db, redisClient, orderBuilder, messageSender)

	executor := &Executor{
		db:                  db,
		redisClient:         redisClient,
		mqttClient:          mqttClient,
		config:              cfg,
		orderBuilder:        orderBuilder,
		stepManager:         stepManager,
		commandResultSender: commandResultSender,
	}

	utils.Logger.Infof("✅ Workflow Executor CREATED")
	return executor
}

// ExecuteCommandOrder PLC 명령에 대한 워크플로우 실행 시작
func (e *Executor) ExecuteCommandOrder(command *models.Command) error {
	if command.CommandDefinition.CommandType == "" {
		e.db.Preload("CommandDefinition").First(&command, command.ID)
	}
	utils.Logger.Infof("Starting workflow for command: %s (ID: %d)", command.CommandDefinition.CommandType, command.ID)

	commandExecution := &models.CommandExecution{
		CommandID:         command.ID,
		Status:            models.CommandExecutionStatusRunning,
		CurrentOrderIndex: 1,
		StartedAt:         time.Now(),
	}
	if err := e.db.Create(commandExecution).Error; err != nil {
		return fmt.Errorf("failed to create command execution: %v", err)
	}

	return e.executeNextOrder(commandExecution)
}

// SendDirectActionOrder 직접 액션 오더 전송
func (e *Executor) SendDirectActionOrder(baseCommand string, commandType rune, armParam string) (string, error) {
	directOrder, orderID, err := e.orderBuilder.BuildDirectActionOrder(baseCommand, commandType, armParam)
	if err != nil {
		return "", err
	}

	if err := e.sendOrder(directOrder); err != nil {
		return "", err
	}

	return orderID, nil
}

// HandleOrderStateUpdate 로봇 상태 업데이트 처리
func (e *Executor) HandleOrderStateUpdate(stateMsg *models.RobotStateMessage) {
	// 단계 완료 확인 및 처리
	if e.stepManager.HandleStepCompletion(stateMsg) {
		// 단계가 완료되었으면 추가 처리 없음 (StepManager에서 이미 처리됨)
		return
	}
}

// CancelAllRunningOrders 모든 실행 중인 오더 취소
func (e *Executor) CancelAllRunningOrders() error {
	var commandExecutions []models.CommandExecution
	e.db.Where("status = ?", models.CommandExecutionStatusRunning).Find(&commandExecutions)

	for _, cmdExec := range commandExecutions {
		now := time.Now()
		repository.UpdateCommandExecutionStatus(e.db, &cmdExec, models.CommandExecutionStatusCancelled, &now)

		var orderExecutions []models.OrderExecution
		e.db.Where("command_execution_id = ? AND status IN ?",
			cmdExec.ID, []string{models.OrderExecutionStatusRunning, models.OrderExecutionStatusPending}).
			Find(&orderExecutions)

		for _, orderExec := range orderExecutions {
			nowOrderExec := time.Now()
			repository.UpdateOrderExecutionStatus(e.db, &orderExec, models.OrderExecutionStatusFailed, &nowOrderExec)

			// 실행 중인 단계들 취소
			e.stepManager.CancelRunningSteps(orderExec.ID, "Cancelled by order cancel command")
		}
	}

	// 로봇에 취소 메시지 전송
	return e.SendCancelOrder()
}

// SendCancelOrder 로봇에 cancelOrder 요청 전송
func (e *Executor) SendCancelOrder() error {
	cancelMessage, err := e.orderBuilder.BuildCancelOrderMessage()
	if err != nil {
		return fmt.Errorf("failed to build cancel order message: %v", err)
	}

	reqData, err := json.Marshal(cancelMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal cancelOrder request: %v", err)
	}

	topic := fmt.Sprintf("meili/v2/%s/%s/instantActions", e.config.RobotManufacturer, e.config.RobotSerialNumber)

	utils.Logger.Infof("📤 SENDING CANCEL ORDER: %s", string(reqData))

	token := e.mqttClient.Publish(topic, 0, false, reqData)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	return nil
}

// executeNextOrder 조건에 맞는 다음 오더를 찾아 실행
func (e *Executor) executeNextOrder(commandExecution *models.CommandExecution) error {
	e.db.Preload("Command.CommandDefinition").First(&commandExecution, commandExecution.ID)

	if commandExecution.CurrentOrderIndex == 0 {
		// 워크플로우 완료
		return e.completeCommandExecution(commandExecution)
	}

	// 다음 오더 매핑 조회
	var mapping models.CommandOrderMapping
	err := e.db.Where("command_definition_id = ? AND execution_order = ?",
		commandExecution.Command.CommandDefinitionID, commandExecution.CurrentOrderIndex).
		Preload("Template.OrderSteps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Preload("Template.OrderSteps.NodeTemplate").
		Preload("Template.OrderSteps.StepActionMappings.ActionTemplate.Parameters").
		Preload("Template.OrderSteps.Edges").
		First(&mapping).Error

	if err != nil {
		errMsg := fmt.Sprintf("no order mapping found for index %d: %v", commandExecution.CurrentOrderIndex, err)
		utils.Logger.Errorf("Workflow for CommandExecutionID %d will be terminated. Reason: %s", commandExecution.ID, errMsg)

		now := time.Now()
		repository.UpdateCommandExecutionStatus(e.db, commandExecution, models.CommandExecutionStatusFailed, &now)
		repository.UpdateCommandStatus(e.db, &commandExecution.Command, models.StatusFailure, errMsg)
		e.sendResponseToPLC(commandExecution.Command.CommandDefinition.CommandType, "F", errMsg)
		return fmt.Errorf(errMsg)
	}

	// 새 오더 실행 생성
	orderExecution := &models.OrderExecution{
		CommandExecutionID: commandExecution.ID,
		TemplateID:         mapping.TemplateID,
		OrderID:            e.orderBuilder.GenerateOrderID(),
		ExecutionOrder:     mapping.ExecutionOrder,
		CurrentStep:        1,
		Status:             models.OrderExecutionStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := e.db.Create(orderExecution).Error; err != nil {
		return fmt.Errorf("failed to create order execution: %v", err)
	}

	utils.Logger.Infof("Starting order execution: %s (Index: %d)", orderExecution.OrderID, orderExecution.ExecutionOrder)

	// 첫 번째 단계 실행
	e.stepManager.ExecuteNextStep(orderExecution, &mapping.Template)
	return nil
}

// completeCommandExecution 명령 실행 완료 처리
func (e *Executor) completeCommandExecution(commandExecution *models.CommandExecution) error {
	var failedOrderCount int64
	e.db.Model(&models.OrderExecution{}).Where("command_execution_id = ? AND status = ?",
		commandExecution.ID, models.OrderExecutionStatusFailed).Count(&failedOrderCount)

	finalStatus := models.CommandExecutionStatusCompleted
	finalCommandStatus := models.StatusSuccess
	if failedOrderCount > 0 {
		finalStatus = models.CommandExecutionStatusFailed
		finalCommandStatus = models.StatusFailure
	}

	now := time.Now()
	repository.UpdateCommandExecutionStatus(e.db, commandExecution, finalStatus, &now)
	repository.UpdateCommandStatus(e.db, &commandExecution.Command, finalCommandStatus, "")
	e.sendResponseToPLC(commandExecution.Command.CommandDefinition.CommandType, finalCommandStatus, "")

	utils.Logger.Infof("Workflow completed for command execution ID: %d with status: %s", commandExecution.ID, finalStatus)
	return nil
}

// triggerNextOrder 다음 오더 트리거 (성공/실패에 따라)
func (e *Executor) TriggerNextOrder(completedOrder *models.OrderExecution, success bool) {
	var cmdExec models.CommandExecution
	e.db.Preload("Command.CommandDefinition").First(&cmdExec, completedOrder.CommandExecutionID)

	var currentMapping models.CommandOrderMapping
	e.db.Where("command_definition_id = ? AND execution_order = ?",
		cmdExec.Command.CommandDefinitionID, completedOrder.ExecutionOrder).First(&currentMapping)

	var nextOrderIndex int
	if success {
		nextOrderIndex = currentMapping.NextExecutionOrder
	} else {
		nextOrderIndex = currentMapping.FailureOrder
	}

	cmdExec.CurrentOrderIndex = nextOrderIndex
	e.db.Save(&cmdExec)
	e.executeNextOrder(&cmdExec)
}

// sendResponseToPLC PLC에 응답 전송
func (e *Executor) sendResponseToPLC(command, status, errMsg string) {
	if e.commandResultSender != nil {
		e.commandResultSender.SendResponseToPLC(command, status, errMsg)
	} else {
		// 직접 전송 (fallback)
		var finalStatus string
		if status == models.StatusSuccess {
			finalStatus = "S"
		} else if status == models.StatusFailure {
			finalStatus = "F"
		} else {
			finalStatus = status
		}

		response := fmt.Sprintf("%s:%s", command, finalStatus)
		if finalStatus == "F" && errMsg != "" {
			utils.Logger.Errorf("Command %s failed: %s", command, errMsg)
		}

		topic := e.config.PlcResponseTopic
		utils.Logger.Infof("Sending response to PLC: %s", response)
		token := e.mqttClient.Publish(topic, 0, false, response)
		if token.Wait() && token.Error() != nil {
			utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())
		}
	}
}

// sendOrder 오더 메시지 전송
func (e *Executor) sendOrder(orderPayload interface{}) error {
	topic := fmt.Sprintf("meili/v2/%s/%s/order", e.config.RobotManufacturer, e.config.RobotSerialNumber)

	msgData, err := json.Marshal(orderPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %v", err)
	}

	utils.Logger.Infof("📤 SENDING ORDER: %s", string(msgData))

	token := e.mqttClient.Publish(topic, 0, false, msgData)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	return nil
}

// GetRunningExecutions 실행 중인 명령 실행들 조회
func (e *Executor) GetRunningExecutions() ([]models.CommandExecution, error) {
	var executions []models.CommandExecution
	err := e.db.Where("status = ?", models.CommandExecutionStatusRunning).
		Preload("Command.CommandDefinition").Find(&executions).Error
	return executions, err
}

// GetExecutionByID 특정 실행 조회
func (e *Executor) GetExecutionByID(id uint) (*models.CommandExecution, error) {
	var execution models.CommandExecution
	err := e.db.Preload("Command.CommandDefinition").
		Preload("OrderExecutions.Steps").First(&execution, id).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

// MQTTMessageSender MQTT 메시지 전송기 (MessageSender 인터페이스 구현)
type MQTTMessageSender struct {
	mqttClient mqtt.Client
	config     *config.Config
}

// SendOrderMessage 오더 메시지 전송
func (m *MQTTMessageSender) SendOrderMessage(orderMsg *models.OrderMessage) error {
	topic := fmt.Sprintf("meili/v2/%s/%s/order", m.config.RobotManufacturer, m.config.RobotSerialNumber)

	msgData, err := json.Marshal(orderMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %v", err)
	}

	utils.Logger.Infof("📤 SENDING ORDER MESSAGE: %s", string(msgData))

	token := m.mqttClient.Publish(topic, 0, false, msgData)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	return nil
}
