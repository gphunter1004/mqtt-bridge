// internal/workflow/executor.go (수정됨: ExecuteCommandOrder 로직 복원)
package workflow

import (
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/command"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/common/idgen"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/messaging"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/repository"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Executor 워크플로우 실행 엔진
type Executor struct {
	db             *gorm.DB
	redisClient    *redis.Client
	mqttClient     mqtt.Client
	config         *config.Config
	orderBuilder   *OrderBuilder
	stepManager    *StepManager
	plcSender      *messaging.PLCResponseSender
	commandHandler command.CommandHandler
}

// NewExecutor 새 워크플로우 실행기 생성
func NewExecutor(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config,
	plcSender *messaging.PLCResponseSender) *Executor {

	utils.Logger.Infof("🏗️ CREATING Workflow Executor")

	orderBuilder := NewOrderBuilder(cfg)
	messageSender := &MQTTMessageSender{
		mqttClient: mqttClient,
		config:     cfg,
	}

	executor := &Executor{
		db:             db,
		redisClient:    redisClient,
		mqttClient:     mqttClient,
		config:         cfg,
		orderBuilder:   orderBuilder,
		plcSender:      plcSender,
		commandHandler: nil,
	}

	stepManager := NewStepManager(db, redisClient, orderBuilder, messageSender)
	stepManager.SetExecutor(executor)
	executor.stepManager = stepManager

	utils.Logger.Infof("✅ Workflow Executor CREATED")
	return executor
}

// SetCommandHandler는 순환 의존성을 피하기 위해 사용됩니다.
func (e *Executor) SetCommandHandler(handler command.CommandHandler) {
	e.commandHandler = handler
	utils.Logger.Infof("✅ Workflow Executor: Command Handler reference set")
}

// ExecuteCommandOrder는 전달받은 Command를 기반으로 워크플로우를 시작합니다. (수정됨)
func (e *Executor) ExecuteCommandOrder(command *models.Command) error {
	if command == nil {
		return fmt.Errorf("command cannot be nil")
	}

	if command.CommandDefinition.CommandType == "" {
		e.db.Preload("CommandDefinition").First(&command, command.ID)
	}

	utils.Logger.Infof("🚀 Starting workflow for command: %s (CommandID: %d)",
		command.CommandDefinition.CommandType, command.ID)

	// Executor가 다시 CommandExecution을 생성합니다.
	commandExecution := &models.CommandExecution{
		CommandID:         command.ID,
		Status:            constants.CommandExecutionStatusRunning,
		CurrentOrderIndex: 1,
		StartedAt:         time.Now(),
	}
	if err := e.db.Create(&commandExecution).Error; err != nil {
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
	utils.Logger.Debugf("🔍 HandleOrderStateUpdate called for OrderID: %s", stateMsg.OrderID)
	if e.stepManager.HandleStepCompletion(stateMsg) {
		utils.Logger.Infof("✅ Step completion handled for OrderID: %s", stateMsg.OrderID)
		return
	}
	utils.Logger.Debugf("🔍 No step completion detected for OrderID: %s", stateMsg.OrderID)
}

// OnOrderCompleted 오더 완료 콜백 (StepManager에서 호출)
func (e *Executor) OnOrderCompleted(orderExecution *models.OrderExecution, success bool) {
	utils.Logger.Infof("📢 OnOrderCompleted called: OrderID=%s, Success=%t",
		orderExecution.OrderID, success)

	var cmdExec models.CommandExecution
	if err := e.db.Preload("Command.CommandDefinition").First(&cmdExec, orderExecution.CommandExecutionID).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to load command execution: %v", err)
		return
	}

	var currentMapping models.CommandOrderMapping
	if err := e.db.Where("command_definition_id = ? AND execution_order = ?",
		cmdExec.Command.CommandDefinitionID, orderExecution.ExecutionOrder).First(&currentMapping).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to load command mapping: %v", err)
		e.completeCommandExecution(&cmdExec, false)
		return
	}

	var nextOrderIndex int
	if success {
		nextOrderIndex = currentMapping.NextExecutionOrder
	} else {
		nextOrderIndex = currentMapping.FailureOrder
	}

	cmdExec.CurrentOrderIndex = nextOrderIndex
	if err := e.db.Save(&cmdExec).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to update command execution: %v", err)
		e.completeCommandExecution(&cmdExec, false)
		return
	}

	if err := e.executeNextOrder(&cmdExec); err != nil {
		utils.Logger.Errorf("❌ Failed to execute next order: %v", err)
	}
}

// CancelAllRunningOrders 모든 실행 중인 오더 취소
func (e *Executor) CancelAllRunningOrders() error {
	var commandExecutions []models.CommandExecution
	e.db.Where("status = ?", constants.CommandExecutionStatusRunning).
		Preload("Command").
		Find(&commandExecutions)

	for _, cmdExec := range commandExecutions {
		now := time.Now()
		repository.UpdateCommandExecutionStatus(e.db, &cmdExec, constants.CommandExecutionStatusCancelled, &now)
		repository.UpdateCommandStatus(e.db, &cmdExec.Command, constants.CommandStatusFailure, "Cancelled by user")

		var orderExecutions []models.OrderExecution
		e.db.Where("command_execution_id = ? AND status IN ?",
			cmdExec.ID, []string{constants.OrderExecutionStatusRunning, constants.OrderExecutionStatusPending}).
			Find(&orderExecutions)

		for _, orderExec := range orderExecutions {
			nowOrderExec := time.Now()
			repository.UpdateOrderExecutionStatus(e.db, &orderExec, constants.OrderExecutionStatusFailed, &nowOrderExec)
			e.stepManager.CancelRunningSteps(orderExec.ID, "Cancelled by order cancel command")
		}
		if e.commandHandler != nil {
			e.commandHandler.FinishCommand(cmdExec.CommandID, false)
		}
	}

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
	topic := constants.GetMeiliInstantActionsTopic(e.config.RobotManufacturer, e.config.RobotSerialNumber)
	token := e.mqttClient.Publish(topic, 0, false, reqData)
	token.Wait()
	return token.Error()
}

// executeNextOrder 조건에 맞는 다음 오더를 찾아 실행
func (e *Executor) executeNextOrder(commandExecution *models.CommandExecution) error {
	e.db.Preload("Command.CommandDefinition").First(&commandExecution, commandExecution.ID)
	if commandExecution.CurrentOrderIndex == 0 {
		return e.completeCommandExecution(commandExecution, true)
	}

	var mapping models.CommandOrderMapping
	err := e.db.Where("command_definition_id = ? AND execution_order = ?",
		commandExecution.Command.CommandDefinitionID, commandExecution.CurrentOrderIndex).
		Preload("Template.OrderSteps", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_steps.step_order ASC")
		}).
		Preload("Template.OrderSteps.NodeTemplate").
		Preload("Template.OrderSteps.StepActionMappings.ActionTemplate.Parameters").
		Preload("Template.OrderSteps.Edges").
		First(&mapping).Error

	if err != nil {
		errMsg := fmt.Sprintf("no order mapping found for index %d: %v", commandExecution.CurrentOrderIndex, err)
		utils.Logger.Errorf(errMsg)
		e.completeCommandExecution(commandExecution, false)
		return fmt.Errorf(errMsg)
	}

	orderExecution := &models.OrderExecution{
		CommandExecutionID: commandExecution.ID,
		TemplateID:         mapping.TemplateID,
		OrderID:            idgen.OrderID(),
		ExecutionOrder:     mapping.ExecutionOrder,
		CurrentStep:        1,
		Status:             constants.OrderExecutionStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := e.db.Create(orderExecution).Error; err != nil {
		e.completeCommandExecution(commandExecution, false)
		return fmt.Errorf("failed to create order execution: %v", err)
	}

	e.stepManager.ExecuteNextStep(orderExecution, &mapping.Template)
	return nil
}

// completeCommandExecution 명령 실행 완료 처리
func (e *Executor) completeCommandExecution(commandExecution *models.CommandExecution, success bool) error {
	var finalStatus string
	var finalCommandStatus string
	var message string

	if success {
		finalStatus = constants.CommandExecutionStatusCompleted
		finalCommandStatus = constants.StatusSuccess
		message = "Command completed successfully"
	} else {
		finalStatus = constants.CommandExecutionStatusFailed
		finalCommandStatus = constants.StatusFailure
		message = "Command failed during execution"
	}

	now := time.Now()
	repository.UpdateCommandExecutionStatus(e.db, commandExecution, finalStatus, &now)
	repository.UpdateCommandStatus(e.db, &commandExecution.Command, finalCommandStatus, message)

	e.sendResponseToPLC(commandExecution.Command.CommandDefinition.CommandType, finalCommandStatus, message)

	if e.commandHandler != nil {
		e.commandHandler.FinishCommand(commandExecution.CommandID, success)
	}

	return nil
}

// sendResponseToPLC PLC에 응답 전송
func (e *Executor) sendResponseToPLC(command, status, errMsg string) {
	var finalStatus string
	switch status {
	case constants.CommandStatusSuccess:
		finalStatus = constants.StatusSuccess
	case constants.CommandStatusFailure:
		finalStatus = constants.StatusFailure
	case constants.CommandStatusRunning:
		finalStatus = constants.StatusRunning
	case constants.CommandStatusAbnormal:
		finalStatus = constants.StatusAbnormal
	case constants.CommandStatusNormal:
		finalStatus = constants.StatusNormal
	default:
		finalStatus = status
	}
	if err := e.plcSender.SendResponse(command, finalStatus, errMsg); err != nil {
		utils.Logger.Errorf("❌ Failed to send PLC response: %v", err)
	}
}

// sendOrder 오더 메시지 전송
func (e *Executor) sendOrder(orderPayload interface{}) error {
	topic := constants.GetMeiliOrderTopic(e.config.RobotManufacturer, e.config.RobotSerialNumber)
	msgData, err := json.Marshal(orderPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %v", err)
	}
	token := e.mqttClient.Publish(topic, 0, false, msgData)
	token.Wait()
	return token.Error()
}

// MQTTMessageSender 구현
type MQTTMessageSender struct {
	mqttClient mqtt.Client
	config     *config.Config
}

func (m *MQTTMessageSender) SendOrderMessage(orderMsg *models.OrderMessage) error {
	topic := constants.GetMeiliOrderTopic(m.config.RobotManufacturer, m.config.RobotSerialNumber)
	msgData, err := json.Marshal(orderMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal order message: %v", err)
	}
	token := m.mqttClient.Publish(topic, 0, false, msgData)
	token.Wait()
	return token.Error()
}
