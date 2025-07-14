// internal/mqtt/order_executor.go (타임아웃 제거된 최종 수정본)
package mqtt

import (
	"context"
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

type OrderExecutor struct {
	db                  *gorm.DB
	redisClient         *redis.Client
	mqttClient          mqtt.Client
	config              *config.Config
	orderMessageHandler *OrderMessageHandler
}

func NewOrderExecutor(db *gorm.DB, redisClient *redis.Client, mqttClient mqtt.Client, cfg *config.Config) *OrderExecutor {
	orderMessageHandler := NewOrderMessageHandler(mqttClient, cfg)
	return &OrderExecutor{
		db:                  db,
		redisClient:         redisClient,
		mqttClient:          mqttClient,
		config:              cfg,
		orderMessageHandler: orderMessageHandler,
	}
}

// ExecuteCommandOrder PLC 명령에 대한 워크플로우 실행 시작
func (e *OrderExecutor) ExecuteCommandOrder(command *models.Command) error {
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

// executeNextOrder 조건에 맞는 다음 오더를 찾아 실행
func (e *OrderExecutor) executeNextOrder(commandExecution *models.CommandExecution) error {
	e.db.Preload("Command.CommandDefinition").First(&commandExecution, commandExecution.ID)

	if commandExecution.CurrentOrderIndex == 0 {
		var failedOrderCount int64
		e.db.Model(&models.OrderExecution{}).Where("command_execution_id = ? AND status = ?", commandExecution.ID, models.OrderExecutionStatusFailed).Count(&failedOrderCount)

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

	orderExecution := &models.OrderExecution{
		CommandExecutionID: commandExecution.ID,
		TemplateID:         mapping.TemplateID,
		OrderID:            e.orderMessageHandler.GenerateOrderID(),
		ExecutionOrder:     mapping.ExecutionOrder,
		CurrentStep:        1,
		Status:             models.OrderExecutionStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := e.db.Create(orderExecution).Error; err != nil {
		return fmt.Errorf("failed to create order execution: %v", err)
	}

	utils.Logger.Infof("Starting order execution: %s (Index: %d)", orderExecution.OrderID, orderExecution.ExecutionOrder)
	e.executeNextStep(orderExecution, &mapping.Template)
	return nil
}

// executeNextStep 다음 단계 실행 (타임아웃 제거)
func (e *OrderExecutor) executeNextStep(execution *models.OrderExecution, template *models.OrderTemplate) {
	var currentOrderStep *models.OrderStep
	for i := range template.OrderSteps {
		if template.OrderSteps[i].StepOrder == execution.CurrentStep {
			currentOrderStep = &template.OrderSteps[i]
			break
		}
	}

	if currentOrderStep == nil {
		now := time.Now()
		repository.UpdateOrderExecutionStatus(e.db, execution, models.OrderExecutionStatusCompleted, &now)
		e.triggerNextOrder(execution, true)
		return
	}

	stepExecution := &models.StepExecution{
		ExecutionID:         execution.ID,
		StepOrder:           currentOrderStep.StepOrder,
		Status:              models.StepExecutionStatusRunning,
		ExpectedActionCount: len(currentOrderStep.StepActionMappings),
		StartedAt:           time.Now(),
	}
	e.db.Create(stepExecution)

	orderMsg := e.orderMessageHandler.BuildOrderMessage(execution, currentOrderStep)
	e.initializeActionStatusInRedis(stepExecution, orderMsg)

	if err := e.orderMessageHandler.SendOrder(orderMsg); err != nil {
		e.handleStepFailure(stepExecution, execution, fmt.Sprintf("failed to send order: %v", err))
		return
	}
	stepExecution.SentToRobot = true
	e.db.Save(stepExecution)

	// WaitForCompletion이 false인 경우에만 즉시 다음 단계로 진행
	// true인 경우 state 메시지를 통해 완료 대기 (타임아웃 제거)
	if !currentOrderStep.WaitForCompletion {
		now := time.Now()
		repository.UpdateStepExecutionStatus(e.db, stepExecution, models.StepExecutionStatusFinished, models.PreviousResultSuccess, "", &now)
		execution.CurrentStep++
		e.db.Save(execution)
		e.executeNextStep(execution, template)
	}
}

func (e *OrderExecutor) initializeActionStatusInRedis(stepExec *models.StepExecution, orderMsg *models.OrderMessage) {
	ctx := context.Background()
	redisKey := fmt.Sprintf("step_actions:%d", stepExec.ID)
	e.redisClient.Del(ctx, redisKey)
	pipe := e.redisClient.Pipeline()
	for _, node := range orderMsg.Nodes {
		for _, action := range node.Actions {
			pipe.HSet(ctx, redisKey, action.ActionID, models.ActionStatusWaiting)
		}
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		utils.Logger.Errorf("Failed to initialize action status in Redis for step %d: %v", stepExec.ID, err)
	}
}

func (e *OrderExecutor) handleStepFailure(step *models.StepExecution, order *models.OrderExecution, reason string) {
	now := time.Now()
	repository.UpdateStepExecutionStatus(e.db, step, models.StepExecutionStatusFailed, "", reason, &now)
	repository.UpdateOrderExecutionStatus(e.db, order, models.OrderExecutionStatusFailed, &now)

	e.redisClient.Del(context.Background(), fmt.Sprintf("step_actions:%d", step.ID))
	e.triggerNextOrder(order, false)
}

func (e *OrderExecutor) triggerNextOrder(completedOrder *models.OrderExecution, success bool) {
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

// sendResponseToPLC PLC에 응답 전송 (함수명 통일)
func (e *OrderExecutor) sendResponseToPLC(command, status, errMsg string) {
	var finalStatus string
	if status == models.StatusSuccess {
		finalStatus = "S"
	} else if status == models.StatusFailure {
		finalStatus = "F"
	} else {
		finalStatus = status // "R" for Rejected
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
	} else {
		utils.Logger.Infof("Response sent successfully to PLC: %s", response)
	}
}

func (e *OrderExecutor) HandleOrderStateUpdate(stateMsg *models.RobotStateMessage) {
	if stateMsg.OrderID == "" {
		return
	}

	var stepExecution models.StepExecution
	err := e.db.Joins("JOIN order_executions ON step_executions.execution_id = order_executions.id").
		Where("order_executions.order_id = ? AND step_executions.status = ?", stateMsg.OrderID, models.StepExecutionStatusRunning).
		Preload("Execution.Template").
		First(&stepExecution).Error
	if err != nil {
		return
	}

	ctx := context.Background()
	redisKey := fmt.Sprintf("step_actions:%d", stepExecution.ID)

	for _, actionState := range stateMsg.ActionStates {
		e.redisClient.HSet(ctx, redisKey, actionState.ActionID, actionState.ActionStatus)
	}

	allStatuses, err := e.redisClient.HGetAll(ctx, redisKey).Result()
	if err != nil {
		utils.Logger.Errorf("Failed to get action statuses from Redis for step %d: %v", stepExecution.ID, err)
		return
	}

	stepResult := e.determineStepResultFromMap(allStatuses, &stepExecution)
	if stepResult == "" {
		return
	}

	e.redisClient.Del(ctx, redisKey)

	if stepResult == models.PreviousResultFailure {
		e.handleStepFailure(&stepExecution, &stepExecution.Execution, "Action failed or robot reported a critical error.")
		return
	}

	now := time.Now()
	repository.UpdateStepExecutionStatus(e.db, &stepExecution, models.StepExecutionStatusFinished, models.PreviousResultSuccess, "", &now)

	execution := stepExecution.Execution
	execution.CurrentStep++
	e.db.Save(&execution)
	e.executeNextStep(&execution, &execution.Template)
}

func (e *OrderExecutor) CancelAllRunningOrders() error {
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

			var stepExecutions []models.StepExecution
			e.db.Where("execution_id = ? AND status = ?", orderExec.ID, models.StepExecutionStatusRunning).
				Find(&stepExecutions)
			for _, stepExec := range stepExecutions {
				nowStepExec := time.Now()
				repository.UpdateStepExecutionStatus(e.db, &stepExec, models.StepExecutionStatusFailed, "", "Cancelled by order cancel command", &nowStepExec)
			}
		}
	}
	return nil
}

func (e *OrderExecutor) SendCancelOrder() error {
	return e.orderMessageHandler.SendCancelOrder()
}

func (e *OrderExecutor) determineStepResultFromMap(statuses map[string]string, stepExec *models.StepExecution) string {
	if len(statuses) < stepExec.ExpectedActionCount {
		return ""
	}

	allFinished := true
	for _, status := range statuses {
		switch status {
		case models.ActionStatusFailed:
			return models.PreviousResultFailure
		case models.ActionStatusFinished:
			continue
		default:
			allFinished = false
		}
	}

	if allFinished {
		return models.PreviousResultSuccess
	}

	return ""
}
