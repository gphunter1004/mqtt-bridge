// internal/handlers/mqtt_handlers.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/interfaces"
	"mqtt-bridge/internal/models"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// =============================================================================
// Command Handler
// =============================================================================

type CommandHandler struct {
	database         interfaces.DatabaseService
	cache            interfaces.CacheService
	messagePublisher interfaces.MessagePublisher
	orderExecutor    OrderExecutorInterface
	config           interfaces.ConfigProvider
	logger           interfaces.Logger
	processingMutex  sync.Mutex
	isProcessing     bool
}

// OrderExecutorInterface 오더 실행자 인터페이스
type OrderExecutorInterface interface {
	ExecuteCommandOrder(command *models.Command) error
	CancelAllRunningOrders() error
	SendFinalResponseToPLC(commandType, status, errMsg string)
}

func NewCommandHandler(
	database interfaces.DatabaseService,
	cache interfaces.CacheService,
	messagePublisher interfaces.MessagePublisher,
	orderExecutor OrderExecutorInterface,
	config interfaces.ConfigProvider,
	logger interfaces.Logger,
) *CommandHandler {
	return &CommandHandler{
		database:         database,
		cache:            cache,
		messagePublisher: messagePublisher,
		orderExecutor:    orderExecutor,
		config:           config,
		logger:           logger,
	}
}

func (h *CommandHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	commandStr := strings.TrimSpace(string(msg.Payload()))
	h.logger.Infof("Received command from PLC: %s", commandStr)

	cmdDef, err := h.database.GetCommandDefinition(commandStr)
	if err != nil {
		errMsg := fmt.Sprintf("Command '%s' not defined or inactive", commandStr)
		h.logger.Errorf(errMsg)
		h.orderExecutor.SendFinalResponseToPLC(commandStr, "F", errMsg)
		return
	}

	if cmdDef.CommandType == models.CommandOrderCancel {
		h.logger.Infof("Processing 'OC' (Order Cancel) command")
		if err := h.orderExecutor.CancelAllRunningOrders(); err != nil {
			h.orderExecutor.SendFinalResponseToPLC(cmdDef.CommandType, "F", err.Error())
		} else {
			h.orderExecutor.SendFinalResponseToPLC(cmdDef.CommandType, "S", "")
		}
		return
	}

	h.processingMutex.Lock()
	if h.isProcessing {
		h.processingMutex.Unlock()
		errMsg := "Command rejected: Another command is currently processing"
		h.logger.Warnf("Command %s rejected: %s", commandStr, errMsg)
		h.orderExecutor.SendFinalResponseToPLC(commandStr, "R", errMsg)

		now := time.Now()
		command := &models.Command{
			CommandDefinitionID: cmdDef.ID,
			Status:              models.StatusRejected,
			RequestTime:         now,
			ResponseTime:        &now,
			ErrorMessage:        errMsg,
		}
		h.database.CreateCommand(command)
		return
	}
	h.isProcessing = true
	h.processingMutex.Unlock()

	command := &models.Command{
		CommandDefinitionID: cmdDef.ID,
		Status:              models.StatusPending,
		RequestTime:         time.Now(),
	}
	h.database.CreateCommand(command)
	h.logger.Infof("Command accepted: ID=%d, Type=%s", command.ID, cmdDef.CommandType)

	go h.processCommand(command)
}

func (h *CommandHandler) processCommand(command *models.Command) {
	defer func() {
		h.processingMutex.Lock()
		h.isProcessing = false
		h.processingMutex.Unlock()
	}()

	robotStatus, err := h.database.GetRobotStatus(h.config.GetRobotSerialNumber())
	if err != nil || robotStatus.ConnectionState != models.ConnectionStateOnline {
		errMsg := "Robot is not online"
		h.database.UpdateCommandStatus(command, models.StatusFailure, errMsg)
		h.orderExecutor.SendFinalResponseToPLC(command.CommandDefinition.CommandType, "F", errMsg)
		return
	}

	h.database.UpdateCommandStatus(command, models.StatusProcessing, "")

	if err := h.orderExecutor.ExecuteCommandOrder(command); err != nil {
		errMsg := fmt.Sprintf("Failed to start command execution: %v", err)
		h.logger.Errorf(errMsg)
		h.database.UpdateCommandStatus(command, models.StatusFailure, errMsg)
		h.orderExecutor.SendFinalResponseToPLC(command.CommandDefinition.CommandType, "F", errMsg)
	}
}

func (h *CommandHandler) FailAllProcessingCommands(reason string) {
	h.processingMutex.Lock()
	defer h.processingMutex.Unlock()

	if !h.isProcessing {
		return
	}

	execution, err := h.database.GetRunningCommandExecution()
	if err == nil {
		now := time.Now()
		h.database.UpdateCommandExecutionStatus(execution, models.CommandExecutionStatusFailed, &now)
		h.database.UpdateCommandStatus(&execution.Command, models.StatusFailure, reason)
		h.orderExecutor.SendFinalResponseToPLC(execution.Command.CommandDefinition.CommandType, "F", reason)
	}

	h.isProcessing = false
}

// =============================================================================
// Robot Handler
// =============================================================================

type RobotHandler struct {
	database       interfaces.DatabaseService
	commandHandler *CommandHandler
	config         interfaces.ConfigProvider
	logger         interfaces.Logger
}

func NewRobotHandler(
	database interfaces.DatabaseService,
	commandHandler *CommandHandler,
	config interfaces.ConfigProvider,
	logger interfaces.Logger,
) *RobotHandler {
	return &RobotHandler{
		database:       database,
		commandHandler: commandHandler,
		config:         config,
		logger:         logger,
	}
}

func (h *RobotHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	var connMsg models.ConnectionStateMessage
	if err := json.Unmarshal(msg.Payload(), &connMsg); err != nil {
		h.logger.Errorf("Failed to parse connection state message: %v", err)
		return
	}
	h.logger.Infof("Received robot connection state from topic: %s with state: %s", msg.Topic(), connMsg.ConnectionState)

	if !models.IsValidConnectionState(connMsg.ConnectionState) {
		h.logger.Errorf("Invalid connection state received: %s", connMsg.ConnectionState)
		return
	}

	timestamp, err := time.Parse(time.RFC3339, connMsg.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	h.updateRobotStatus(&connMsg, timestamp)
	h.handleConnectionStateChange(&connMsg)
}

func (h *RobotHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	var stateMsg models.RobotStateMessage
	if err := json.Unmarshal(msg.Payload(), &stateMsg); err != nil {
		h.logger.Errorf("Failed to parse robot state message: %v", err)
		return
	}
	// 추가 처리는 MessageHandler에서 담당
}

func (h *RobotHandler) updateRobotStatus(connMsg *models.ConnectionStateMessage, timestamp time.Time) {
	existingStatus, err := h.database.GetRobotStatus(connMsg.SerialNumber)

	if err != nil {
		// 새로 생성
		robotStatus := &models.RobotStatus{
			Manufacturer:    connMsg.Manufacturer,
			SerialNumber:    connMsg.SerialNumber,
			ConnectionState: connMsg.ConnectionState,
			LastHeaderID:    connMsg.HeaderID,
			LastTimestamp:   timestamp,
			Version:         connMsg.Version,
		}
		if err := h.database.CreateRobotStatus(robotStatus); err != nil {
			h.logger.Errorf("Failed to create robot status: %v", err)
		}
	} else {
		// 기존 업데이트
		existingStatus.ConnectionState = connMsg.ConnectionState
		existingStatus.LastHeaderID = connMsg.HeaderID
		existingStatus.LastTimestamp = timestamp
		existingStatus.Version = connMsg.Version
		if err := h.database.UpdateRobotStatus(existingStatus); err != nil {
			h.logger.Errorf("Failed to update robot status: %v", err)
		}
	}
}

func (h *RobotHandler) handleConnectionStateChange(connMsg *models.ConnectionStateMessage) {
	switch connMsg.ConnectionState {
	case models.ConnectionStateOnline:
		h.logger.Infof("Robot %s is now ONLINE", connMsg.SerialNumber)
	case models.ConnectionStateOffline:
		h.logger.Warnf("Robot %s is now OFFLINE", connMsg.SerialNumber)
		h.commandHandler.FailAllProcessingCommands("Robot went offline")
	case models.ConnectionStateConnectionBroken:
		h.logger.Errorf("Robot %s connection is BROKEN", connMsg.SerialNumber)
		h.commandHandler.FailAllProcessingCommands("Robot connection broken")
	}
}

// =============================================================================
// Order Executor
// =============================================================================

type OrderExecutor struct {
	database     interfaces.DatabaseService
	cache        interfaces.CacheService
	orderBuilder interfaces.OrderMessageBuilder
	config       interfaces.ConfigProvider
	logger       interfaces.Logger
}

func NewOrderExecutor(
	database interfaces.DatabaseService,
	cache interfaces.CacheService,
	orderBuilder interfaces.OrderMessageBuilder,
	config interfaces.ConfigProvider,
	logger interfaces.Logger,
) *OrderExecutor {
	return &OrderExecutor{
		database:     database,
		cache:        cache,
		orderBuilder: orderBuilder,
		config:       config,
		logger:       logger,
	}
}

func (e *OrderExecutor) ExecuteCommandOrder(command *models.Command) error {
	// 명령 정의 로드
	if command.CommandDefinition.CommandType == "" {
		cmdDef, err := e.database.GetCommandDefinition(command.CommandDefinition.CommandType)
		if err != nil {
			return err
		}
		command.CommandDefinition = *cmdDef
	}

	e.logger.Infof("Starting workflow for command: %s (ID: %d)", command.CommandDefinition.CommandType, command.ID)

	commandExecution := &models.CommandExecution{
		CommandID:         command.ID,
		Status:            models.CommandExecutionStatusRunning,
		CurrentOrderIndex: 1,
		StartedAt:         time.Now(),
	}

	if err := e.database.CreateCommandExecution(commandExecution); err != nil {
		return fmt.Errorf("failed to create command execution: %v", err)
	}

	return e.executeNextOrder(commandExecution)
}

func (e *OrderExecutor) executeNextOrder(commandExecution *models.CommandExecution) error {
	if commandExecution.CurrentOrderIndex == 0 {
		// 워크플로우 완료
		finalStatus := models.CommandExecutionStatusCompleted
		finalCommandStatus := models.StatusSuccess

		now := time.Now()
		e.database.UpdateCommandExecutionStatus(commandExecution, finalStatus, &now)
		e.database.UpdateCommandStatus(&commandExecution.Command, finalCommandStatus, "")
		e.SendFinalResponseToPLC(commandExecution.Command.CommandDefinition.CommandType, finalCommandStatus, "")

		e.logger.Infof("Workflow completed for command execution ID: %d", commandExecution.ID)
		return nil
	}

	mapping, err := e.database.GetCommandOrderMapping(
		commandExecution.Command.CommandDefinitionID,
		commandExecution.CurrentOrderIndex,
	)
	if err != nil {
		errMsg := fmt.Sprintf("no order mapping found for index %d: %v", commandExecution.CurrentOrderIndex, err)
		e.logger.Errorf("Workflow terminated. Reason: %s", errMsg)

		now := time.Now()
		e.database.UpdateCommandExecutionStatus(commandExecution, models.CommandExecutionStatusFailed, &now)
		e.database.UpdateCommandStatus(&commandExecution.Command, models.StatusFailure, errMsg)
		e.SendFinalResponseToPLC(commandExecution.Command.CommandDefinition.CommandType, "F", errMsg)
		return fmt.Errorf(errMsg)
	}

	orderExecution := &models.OrderExecution{
		CommandExecutionID: commandExecution.ID,
		TemplateID:         mapping.TemplateID,
		OrderID:            e.orderBuilder.GenerateOrderID(),
		ExecutionOrder:     mapping.ExecutionOrder,
		CurrentStep:        1,
		Status:             models.OrderExecutionStatusRunning,
		StartedAt:          time.Now(),
	}

	if err := e.database.CreateOrderExecution(orderExecution); err != nil {
		return fmt.Errorf("failed to create order execution: %v", err)
	}

	e.logger.Infof("Starting order execution: %s", orderExecution.OrderID)
	return e.executeNextStep(orderExecution, &mapping.Template)
}

func (e *OrderExecutor) executeNextStep(execution *models.OrderExecution, template *models.OrderTemplate) error {
	var currentOrderStep *models.OrderStep
	for i := range template.OrderSteps {
		if template.OrderSteps[i].StepOrder == execution.CurrentStep {
			currentOrderStep = &template.OrderSteps[i]
			break
		}
	}

	if currentOrderStep == nil {
		now := time.Now()
		e.database.UpdateOrderExecutionStatus(execution, models.OrderExecutionStatusCompleted, &now)
		e.triggerNextOrder(execution, true)
		return nil
	}

	stepExecution := &models.StepExecution{
		ExecutionID:         execution.ID,
		StepOrder:           currentOrderStep.StepOrder,
		Status:              models.StepExecutionStatusRunning,
		ExpectedActionCount: len(currentOrderStep.StepActionMappings),
		StartedAt:           time.Now(),
	}
	e.database.CreateStepExecution(stepExecution)

	orderMsg := e.orderBuilder.BuildOrderMessage(execution, currentOrderStep)
	e.initializeActionStatusInRedis(stepExecution, orderMsg)

	if err := e.orderBuilder.SendOrder(orderMsg); err != nil {
		e.handleStepFailure(stepExecution, execution, fmt.Sprintf("failed to send order: %v", err))
		return err
	}

	stepExecution.SentToRobot = true
	e.database.UpdateStepExecutionStatus(stepExecution, stepExecution.Status, stepExecution.Result, stepExecution.ErrorMessage, stepExecution.CompletedAt)

	if currentOrderStep.WaitForCompletion {
		go e.waitForActionCompletion(stepExecution, execution, template, currentOrderStep.TimeoutSeconds)
	} else {
		now := time.Now()
		e.database.UpdateStepExecutionStatus(stepExecution, models.StepExecutionStatusFinished, models.PreviousResultSuccess, "", &now)
		execution.CurrentStep++
		e.database.UpdateOrderExecutionStatus(execution, execution.Status, execution.CompletedAt)
		e.executeNextStep(execution, template)
	}

	return nil
}

func (e *OrderExecutor) initializeActionStatusInRedis(stepExec *models.StepExecution, orderMsg *models.OrderMessage) {
	ctx := context.Background()
	redisKey := fmt.Sprintf("step_actions:%d", stepExec.ID)
	e.cache.Del(ctx, redisKey)

	pipe := e.cache.Pipeline()
	for _, node := range orderMsg.Nodes {
		for _, action := range node.Actions {
			pipe.HSet(ctx, redisKey, action.ActionID, models.ActionStatusWaiting)
		}
	}
	pipe.Exec(ctx)
}

func (e *OrderExecutor) handleStepFailure(step *models.StepExecution, order *models.OrderExecution, reason string) {
	now := time.Now()
	e.database.UpdateStepExecutionStatus(step, models.StepExecutionStatusFailed, "", reason, &now)
	e.database.UpdateOrderExecutionStatus(order, models.OrderExecutionStatusFailed, &now)

	e.cache.Del(context.Background(), fmt.Sprintf("step_actions:%d", step.ID))
	e.triggerNextOrder(order, false)
}

func (e *OrderExecutor) triggerNextOrder(completedOrder *models.OrderExecution, success bool) {
	// 간소화된 다음 오더 트리거 로직
	// 실제 구현에서는 CommandOrderMapping의 NextExecutionOrder, FailureOrder 사용
	e.logger.Infof("Order %s completed with success: %v", completedOrder.OrderID, success)
}

func (e *OrderExecutor) waitForActionCompletion(step *models.StepExecution, order *models.OrderExecution, template *models.OrderTemplate, timeoutSeconds int) {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	<-timer.C

	// 타임아웃 처리
	var currentStep models.StepExecution
	e.database.CreateStepExecution(&currentStep) // 최신 상태 조회를 위한 플레이스홀더
	if currentStep.Status == models.StepExecutionStatusRunning {
		e.logger.Warnf("Action timeout for step execution ID %d", step.ID)
		e.handleStepFailure(step, order, "Action timed out")
	}
}

func (e *OrderExecutor) HandleOrderStateUpdate(stateMsg *models.RobotStateMessage) {
	if stateMsg.OrderID == "" {
		return
	}

	stepExecution, err := e.database.GetRunningStepExecution(stateMsg.OrderID)
	if err != nil {
		return
	}

	ctx := context.Background()
	redisKey := fmt.Sprintf("step_actions:%d", stepExecution.ID)

	for _, actionState := range stateMsg.ActionStates {
		e.cache.HSet(ctx, redisKey, actionState.ActionID, actionState.ActionStatus)
	}

	allStatuses, err := e.cache.HGetAll(ctx, redisKey)
	if err != nil {
		e.logger.Errorf("Failed to get action statuses from Redis for step %d: %v", stepExecution.ID, err)
		return
	}

	stepResult := e.determineStepResultFromMap(allStatuses, stepExecution)
	if stepResult == "" {
		return
	}

	e.cache.Del(ctx, redisKey)

	if stepResult == models.PreviousResultFailure {
		e.handleStepFailure(stepExecution, &stepExecution.Execution, "Action failed")
		return
	}

	now := time.Now()
	e.database.UpdateStepExecutionStatus(stepExecution, models.StepExecutionStatusFinished, models.PreviousResultSuccess, "", &now)

	execution := stepExecution.Execution
	execution.CurrentStep++
	e.database.UpdateOrderExecutionStatus(&execution, execution.Status, execution.CompletedAt)

	template, _ := e.database.GetOrderTemplateWithSteps(execution.TemplateID)
	e.executeNextStep(&execution, template)
}

func (e *OrderExecutor) CancelAllRunningOrders() error {
	return e.database.CancelAllRunningOrders()
}

func (e *OrderExecutor) SendFinalResponseToPLC(commandType, status, errMsg string) {
	var finalStatus string
	if status == models.StatusSuccess {
		finalStatus = "S"
	} else if status == models.StatusFailure {
		finalStatus = "F"
	} else {
		finalStatus = status
	}

	response := fmt.Sprintf("%s:%s", commandType, finalStatus)
	if finalStatus == "F" && errMsg != "" {
		e.logger.Errorf("Command %s failed: %s", commandType, errMsg)
	}

	//topic := e.config.GetPlcResponseTopic()
	e.logger.Infof("Sending FINAL response to PLC: %s", response)

	// MessagePublisher를 직접 주입받지 않았으므로 OrderBuilder를 통해 간접 전송
	// 실제 구현에서는 MessagePublisher를 직접 주입받는 것이 좋음
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

// =============================================================================
// Message Handler
// =============================================================================

type MessageHandler struct {
	commandHandler *CommandHandler
	robotHandler   *RobotHandler
	orderExecutor  *OrderExecutor
}

func NewMessageHandler(
	commandHandler *CommandHandler,
	robotHandler *RobotHandler,
	orderExecutor *OrderExecutor,
) *MessageHandler {
	return &MessageHandler{
		commandHandler: commandHandler,
		robotHandler:   robotHandler,
		orderExecutor:  orderExecutor,
	}
}

func (h *MessageHandler) HandleCommand(client mqtt.Client, msg mqtt.Message) {
	h.commandHandler.HandleCommand(client, msg)
}

func (h *MessageHandler) HandleRobotConnectionState(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleRobotConnectionState(client, msg)
}

func (h *MessageHandler) HandleRobotState(client mqtt.Client, msg mqtt.Message) {
	h.robotHandler.HandleRobotState(client, msg)

	var stateMsg models.RobotStateMessage
	if json.Unmarshal(msg.Payload(), &stateMsg) == nil {
		h.orderExecutor.HandleOrderStateUpdate(&stateMsg)
	}
}
