// internal/workflow/executor.go (RUNNING 상태 메모리 정리 추가)
package workflow

import (
	"encoding/json"
	"fmt"
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

// CommandHandler 인터페이스 정의 (순환 참조 방지)
type CommandHandler interface {
	ClearRunningStatusFlag(orderExecutionID uint)
}

// Executor 워크플로우 실행 엔진
type Executor struct {
	db             *gorm.DB
	redisClient    *redis.Client
	mqttClient     mqtt.Client
	config         *config.Config
	orderBuilder   *OrderBuilder
	stepManager    *StepManager
	plcSender      *messaging.PLCResponseSender
	commandHandler CommandHandler
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

	// 🔥 먼저 Executor 생성
	executor := &Executor{
		db:             db,
		redisClient:    redisClient,
		mqttClient:     mqttClient,
		config:         cfg,
		orderBuilder:   orderBuilder,
		plcSender:      plcSender,
		commandHandler: nil,
	}

	// StepManager 생성 후 Executor 참조 설정
	stepManager := NewStepManager(db, redisClient, orderBuilder, messageSender)
	stepManager.SetExecutor(executor)
	executor.stepManager = stepManager

	utils.Logger.Infof("✅ Workflow Executor CREATED")
	return executor
}

// Command Handler 참조 설정
func (e *Executor) SetCommandHandler(handler CommandHandler) {
	e.commandHandler = handler
	utils.Logger.Infof("✅ Workflow Executor: Command Handler reference set")
}

// ExecuteCommandOrder PLC 명령에 대한 워크플로우 실행 시작
func (e *Executor) ExecuteCommandOrder(command *models.Command) error {
	if command.CommandDefinition.CommandType == "" {
		e.db.Preload("CommandDefinition").First(&command, command.ID)
	}

	utils.Logger.Infof("🚀 Starting workflow for command: %s (ID: %d)",
		command.CommandDefinition.CommandType, command.ID)

	commandExecution := &models.CommandExecution{
		CommandID:         command.ID,
		Status:            constants.CommandExecutionStatusRunning,
		CurrentOrderIndex: 1,
		StartedAt:         time.Now(),
	}
	if err := e.db.Create(commandExecution).Error; err != nil {
		return fmt.Errorf("failed to create command execution: %v", err)
	}

	utils.Logger.Infof("📝 Command execution created: ID=%d, CurrentOrderIndex=%d",
		commandExecution.ID, commandExecution.CurrentOrderIndex)

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

	// 단계 완료 확인 및 처리
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

	// 🔥 RUNNING 상태 플래그 정리
	if e.commandHandler != nil {
		e.commandHandler.ClearRunningStatusFlag(orderExecution.ID)
		utils.Logger.Debugf("🧹 Cleared RUNNING status flag for OrderExecution ID: %d", orderExecution.ID)
	}

	// CommandExecution 조회
	var cmdExec models.CommandExecution
	if err := e.db.Preload("Command.CommandDefinition").First(&cmdExec, orderExecution.CommandExecutionID).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to load command execution: %v", err)
		return
	}

	// 현재 매핑 조회
	var currentMapping models.CommandOrderMapping
	if err := e.db.Where("command_definition_id = ? AND execution_order = ?",
		cmdExec.Command.CommandDefinitionID, orderExecution.ExecutionOrder).First(&currentMapping).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to load command mapping: %v", err)
		return
	}

	// 다음 오더 인덱스 결정
	var nextOrderIndex int
	if success {
		nextOrderIndex = currentMapping.NextExecutionOrder
		utils.Logger.Infof("📈 Order succeeded, next order index: %d", nextOrderIndex)
	} else {
		nextOrderIndex = currentMapping.FailureOrder
		utils.Logger.Infof("📉 Order failed, failure order index: %d", nextOrderIndex)
	}

	// CommandExecution 업데이트
	cmdExec.CurrentOrderIndex = nextOrderIndex
	if err := e.db.Save(&cmdExec).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to update command execution: %v", err)
		return
	}

	utils.Logger.Infof("🔄 CommandExecution updated: ID=%d, CurrentOrderIndex=%d",
		cmdExec.ID, cmdExec.CurrentOrderIndex)

	// 다음 오더 실행
	if err := e.executeNextOrder(&cmdExec); err != nil {
		utils.Logger.Errorf("❌ Failed to execute next order: %v", err)
	}
}

// CancelAllRunningOrders 모든 실행 중인 오더 취소
func (e *Executor) CancelAllRunningOrders() error {
	var commandExecutions []models.CommandExecution
	e.db.Where("status = ?", constants.CommandExecutionStatusRunning).Find(&commandExecutions)

	for _, cmdExec := range commandExecutions {
		now := time.Now()
		repository.UpdateCommandExecutionStatus(e.db, &cmdExec, constants.CommandExecutionStatusCancelled, &now)

		var orderExecutions []models.OrderExecution
		e.db.Where("command_execution_id = ? AND status IN ?",
			cmdExec.ID, []string{constants.OrderExecutionStatusRunning, constants.OrderExecutionStatusPending}).
			Find(&orderExecutions)

		for _, orderExec := range orderExecutions {
			nowOrderExec := time.Now()
			repository.UpdateOrderExecutionStatus(e.db, &orderExec, constants.OrderExecutionStatusFailed, &nowOrderExec)

			// 🔥 RUNNING 상태 플래그 정리
			if e.commandHandler != nil {
				e.commandHandler.ClearRunningStatusFlag(orderExec.ID)
			}

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

	topic := constants.GetMeiliInstantActionsTopic(e.config.RobotManufacturer, e.config.RobotSerialNumber)

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

	utils.Logger.Infof("🔍 executeNextOrder: CommandID=%d, CurrentOrderIndex=%d",
		commandExecution.CommandID, commandExecution.CurrentOrderIndex)

	if commandExecution.CurrentOrderIndex == 0 {
		// 워크플로우 완료
		utils.Logger.Infof("🏁 Workflow completed (CurrentOrderIndex=0)")
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
		utils.Logger.Errorf("❌ %s", errMsg)

		now := time.Now()
		repository.UpdateCommandExecutionStatus(e.db, commandExecution, constants.CommandExecutionStatusFailed, &now)
		repository.UpdateCommandStatus(e.db, &commandExecution.Command, constants.CommandStatusFailure, errMsg)
		e.sendResponseToPLC(commandExecution.Command.CommandDefinition.CommandType, constants.CommandStatusFailure, errMsg)
		return fmt.Errorf(errMsg)
	}

	utils.Logger.Infof("📋 Found order mapping: TemplateID=%d, ExecutionOrder=%d, NextOrder=%d",
		mapping.TemplateID, mapping.ExecutionOrder, mapping.NextExecutionOrder)

	// 새 오더 실행 생성
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
		return fmt.Errorf("failed to create order execution: %v", err)
	}

	utils.Logger.Infof("✅ Order execution created: OrderID=%s, ExecutionOrder=%d",
		orderExecution.OrderID, orderExecution.ExecutionOrder)

	// 첫 번째 단계 실행
	e.stepManager.ExecuteNextStep(orderExecution, &mapping.Template)
	return nil
}

// completeCommandExecution 명령 실행 완료 처리
func (e *Executor) completeCommandExecution(commandExecution *models.CommandExecution) error {
	utils.Logger.Infof("🏁 Completing command execution: ID=%d", commandExecution.ID)

	// 🔥 모든 관련 OrderExecution의 RUNNING 상태 플래그 정리
	if e.commandHandler != nil {
		var orderExecutions []models.OrderExecution
		e.db.Where("command_execution_id = ?", commandExecution.ID).Find(&orderExecutions)

		for _, orderExec := range orderExecutions {
			e.commandHandler.ClearRunningStatusFlag(orderExec.ID)
			utils.Logger.Debugf("🧹 Cleared RUNNING status flag for OrderExecution ID: %d", orderExec.ID)
		}
	}

	var failedOrderCount int64
	e.db.Model(&models.OrderExecution{}).Where("command_execution_id = ? AND status = ?",
		commandExecution.ID, constants.OrderExecutionStatusFailed).Count(&failedOrderCount)

	finalStatus := constants.CommandExecutionStatusCompleted
	finalCommandStatus := constants.CommandStatusSuccess
	if failedOrderCount > 0 {
		finalStatus = constants.CommandExecutionStatusFailed
		finalCommandStatus = constants.CommandStatusFailure
	}

	now := time.Now()
	repository.UpdateCommandExecutionStatus(e.db, commandExecution, finalStatus, &now)
	repository.UpdateCommandStatus(e.db, &commandExecution.Command, finalCommandStatus, "")
	e.sendResponseToPLC(commandExecution.Command.CommandDefinition.CommandType, finalCommandStatus, "")

	utils.Logger.Infof("🎉 Workflow completed: CommandExecutionID=%d, Status=%s",
		commandExecution.ID, finalStatus)
	return nil
}

// TriggerNextOrder 다음 오더 트리거 (성공/실패에 따라) - 레거시 메서드
func (e *Executor) TriggerNextOrder(completedOrder *models.OrderExecution, success bool) {
	utils.Logger.Infof("🔄 TriggerNextOrder (legacy method): OrderID=%s, Success=%t",
		completedOrder.OrderID, success)

	// 새로운 OnOrderCompleted 메서드로 리다이렉트
	e.OnOrderCompleted(completedOrder, success)
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

	utils.Logger.Infof("📤 Sending PLC response: %s:%s", command, finalStatus)

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

	utils.Logger.Infof("📤 SENDING ORDER: %s", string(msgData))

	token := e.mqttClient.Publish(topic, 0, false, msgData)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", token.Error())
	}

	return nil
}

// =============================================================================
// MQTTMessageSender MQTT 메시지 전송기 (MessageSender 인터페이스 구현)
// =============================================================================

type MQTTMessageSender struct {
	mqttClient mqtt.Client
	config     *config.Config
}

// SendOrderMessage 오더 메시지 전송
func (m *MQTTMessageSender) SendOrderMessage(orderMsg *models.OrderMessage) error {
	topic := constants.GetMeiliOrderTopic(m.config.RobotManufacturer, m.config.RobotSerialNumber)

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
