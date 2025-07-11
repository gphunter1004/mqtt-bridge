// internal/mqtt/order_executor.go (업데이트된 버전)
package mqtt

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
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

// ExecuteCommandOrder PLC 명령에 매핑된 여러 오더들을 순차 실행
func (e *OrderExecutor) ExecuteCommandOrder(command *models.Command) error {
	utils.Logger.Infof("Executing orders for command: %s (ID: %d)", command.CommandType, command.ID)

	// 명령 타입에 매핑된 모든 오더 템플릿 조회 (실행 순서대로)
	var mappings []models.CommandOrderMapping
	err := e.db.Preload("Template").
		Preload("Template.OrderSteps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Preload("Template.OrderSteps.NodeTemplate").
		Preload("Template.OrderSteps.Actions").
		Preload("Template.OrderSteps.Actions.Parameters").
		Preload("Template.OrderSteps.Edges").
		Where("command_type = ? AND is_active = ?", command.CommandType, true).
		Order("execution_order ASC").
		Find(&mappings).Error

	if err != nil {
		return fmt.Errorf("failed to load command mappings: %v", err)
	}

	if len(mappings) == 0 {
		return fmt.Errorf("no active order mappings found for command %s", command.CommandType)
	}

	// 명령 실행 생성
	commandExecution := &models.CommandExecution{
		CommandID:         command.ID,
		Status:            models.CommandExecutionStatusPending,
		CurrentOrderIndex: 0,
		StartedAt:         time.Now(),
	}

	if err := e.db.Create(commandExecution).Error; err != nil {
		return fmt.Errorf("failed to create command execution: %v", err)
	}

	// 각 오더에 대한 실행 계획 생성
	for i, mapping := range mappings {
		orderExecution := &models.OrderExecution{
			CommandExecutionID: commandExecution.ID,
			TemplateID:         mapping.TemplateID,
			OrderID:            e.orderMessageHandler.GenerateOrderID(),
			ExecutionOrder:     i,
			CurrentStep:        0,
			Status:             models.OrderExecutionStatusPending,
		}

		if err := e.db.Create(orderExecution).Error; err != nil {
			return fmt.Errorf("failed to create order execution: %v", err)
		}
	}

	// 첫 번째 오더 실행 시작
	return e.executeNextOrder(commandExecution)
}

// executeNextOrder 다음 오더 실행 (순차 실행 보장)
func (e *OrderExecutor) executeNextOrder(commandExecution *models.CommandExecution) error {
	// 현재 실행할 오더 조회
	var orderExecution models.OrderExecution
	err := e.db.Preload("Template").
		Preload("Template.OrderSteps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Preload("Template.OrderSteps.NodeTemplate").
		Preload("Template.OrderSteps.Actions").
		Preload("Template.OrderSteps.Actions.Parameters").
		Preload("Template.OrderSteps.Edges").
		Where("command_execution_id = ? AND execution_order = ?",
			commandExecution.ID, commandExecution.CurrentOrderIndex).
		First(&orderExecution).Error

	if err != nil {
		// 모든 오더 완료
		commandExecution.Status = models.CommandExecutionStatusCompleted
		now := time.Now()
		commandExecution.CompletedAt = &now
		e.db.Save(commandExecution)
		utils.Logger.Infof("All orders completed for command execution: %d", commandExecution.ID)
		return nil
	}

	// 오더 실행 시작
	orderExecution.Status = models.OrderExecutionStatusRunning
	orderExecution.StartedAt = time.Now()
	e.db.Save(&orderExecution)

	commandExecution.Status = models.CommandExecutionStatusRunning
	e.db.Save(commandExecution)

	utils.Logger.Infof("Starting order execution: %s (Order %d/%d)",
		orderExecution.OrderID, commandExecution.CurrentOrderIndex+1,
		e.getTotalOrderCount(commandExecution.ID))

	// 첫 번째 단계 실행
	return e.executeNextStep(&orderExecution, &orderExecution.Template, "")
}

// executeNextStep 다음 단계 실행 (액션 완료 대기 포함)
func (e *OrderExecutor) executeNextStep(execution *models.OrderExecution, template *models.OrderTemplate, previousResult string) error {
	// 현재 단계의 OrderStep 찾기
	var currentOrderStep *models.OrderStep
	for _, step := range template.OrderSteps {
		if step.StepOrder == execution.CurrentStep {
			currentOrderStep = &step
			break
		}
	}

	if currentOrderStep == nil {
		// 현재 오더의 모든 단계 완료
		execution.Status = models.OrderExecutionStatusCompleted
		now := time.Now()
		execution.CompletedAt = &now
		e.db.Save(execution)

		utils.Logger.Infof("Order execution completed: %s", execution.OrderID)

		// 다음 오더로 이동
		var commandExecution models.CommandExecution
		e.db.First(&commandExecution, execution.CommandExecutionID)
		commandExecution.CurrentOrderIndex++
		e.db.Save(&commandExecution)

		return e.executeNextOrder(&commandExecution)
	}

	// 이전 단계 결과에 따른 실행 조건 확인
	if !e.shouldExecuteStep(currentOrderStep, previousResult) {
		// 단계 스킵
		stepExecution := &models.StepExecution{
			ExecutionID: execution.ID,
			StepOrder:   currentOrderStep.StepOrder,
			Status:      models.StepExecutionStatusSkipped,
			StartedAt:   time.Now(),
		}
		now := time.Now()
		stepExecution.CompletedAt = &now
		e.db.Create(stepExecution)

		// 다음 단계로
		execution.CurrentStep++
		e.db.Save(execution)
		return e.executeNextStep(execution, template, previousResult)
	}

	// 단계 실행 기록 생성
	stepExecution := &models.StepExecution{
		ExecutionID:     execution.ID,
		StepOrder:       currentOrderStep.StepOrder,
		Status:          models.StepExecutionStatusRunning,
		SentToRobot:     false,
		ActionCompleted: false,
		StartedAt:       time.Now(),
		LastActionCheck: time.Now(),
	}
	e.db.Create(stepExecution)

	// 오더 메시지 생성 및 전송
	orderMsg := e.orderMessageHandler.BuildOrderMessage(execution, currentOrderStep)
	if err := e.orderMessageHandler.SendOrder(orderMsg); err != nil {
		stepExecution.Status = models.StepExecutionStatusFailed
		stepExecution.ErrorMessage = err.Error()
		now := time.Now()
		stepExecution.CompletedAt = &now
		e.db.Save(stepExecution)

		execution.Status = models.OrderExecutionStatusFailed
		execution.CompletedAt = &now
		e.db.Save(execution)

		// 명령 실행 실패 처리
		var commandExecution models.CommandExecution
		e.db.First(&commandExecution, execution.CommandExecutionID)
		commandExecution.Status = models.CommandExecutionStatusFailed
		commandExecution.CompletedAt = &now
		e.db.Save(&commandExecution)

		return fmt.Errorf("failed to send order to robot: %v", err)
	}

	// 로봇에 전송 완료 표시
	stepExecution.SentToRobot = true
	e.db.Save(stepExecution)

	utils.Logger.Infof("Order step %d sent to robot: %s (waiting for completion)",
		currentOrderStep.StepOrder, execution.OrderID)

	// 액션 완료 대기를 위한 고루틴 시작 (WaitForCompletion이 true인 경우)
	if currentOrderStep.WaitForCompletion {
		go e.waitForActionCompletion(stepExecution.ID, currentOrderStep.TimeoutSeconds)
	} else {
		// 즉시 다음 단계로 (완료 대기하지 않음)
		stepExecution.Status = models.StepExecutionStatusCompleted
		stepExecution.Result = models.PreviousResultSuccess
		stepExecution.ActionCompleted = true
		now := time.Now()
		stepExecution.CompletedAt = &now
		e.db.Save(stepExecution)

		execution.CurrentStep++
		e.db.Save(execution)
		return e.executeNextStep(execution, template, models.PreviousResultSuccess)
	}

	return nil
}

// waitForActionCompletion 액션 완료 대기 (타임아웃 포함)
func (e *OrderExecutor) waitForActionCompletion(stepExecutionID uint, timeoutSeconds int) {
	timeout := time.Duration(timeoutSeconds) * time.Second
	ticker := time.NewTicker(5 * time.Second) // 5초마다 체크
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ticker.C:
			var stepExecution models.StepExecution
			if err := e.db.Preload("Execution").Preload("Execution.Template").
				Preload("Execution.Template.OrderSteps", func(db *gorm.DB) *gorm.DB {
					return db.Order("step_order ASC")
				}).
				First(&stepExecution, stepExecutionID).Error; err != nil {
				utils.Logger.Errorf("Failed to load step execution: %v", err)
				return
			}

			// 이미 완료되었으면 종료
			if stepExecution.ActionCompleted {
				return
			}

			// 타임아웃 체크
			if time.Since(startTime) > timeout {
				utils.Logger.Warnf("Action timeout for step execution: %d", stepExecutionID)
				stepExecution.Status = models.StepExecutionStatusTimeout
				stepExecution.Result = models.PreviousResultFailure
				now := time.Now()
				stepExecution.CompletedAt = &now
				e.db.Save(&stepExecution)

				// 오더 실행 실패 처리
				stepExecution.Execution.Status = models.OrderExecutionStatusFailed
				stepExecution.Execution.CompletedAt = &now
				e.db.Save(&stepExecution.Execution)
				return
			}

			// 마지막 체크 시간 업데이트
			stepExecution.LastActionCheck = time.Now()
			e.db.Save(&stepExecution)

		case <-time.After(timeout):
			// 타임아웃 처리
			var stepExecution models.StepExecution
			e.db.First(&stepExecution, stepExecutionID)
			stepExecution.Status = models.StepExecutionStatusTimeout
			stepExecution.Result = models.PreviousResultFailure
			now := time.Now()
			stepExecution.CompletedAt = &now
			e.db.Save(&stepExecution)
			return
		}
	}
}

// HandleOrderStateUpdate 로봇 상태 업데이트를 통한 액션 완료 감지
func (e *OrderExecutor) HandleOrderStateUpdate(stateMsg *models.RobotStateMessage) {
	if stateMsg.OrderID == "" {
		return
	}

	// 해당 오더의 실행 중인 단계 찾기
	var stepExecution models.StepExecution
	err := e.db.Joins("JOIN order_executions ON step_executions.execution_id = order_executions.id").
		Where("order_executions.order_id = ? AND step_executions.status = ? AND step_executions.sent_to_robot = ?",
			stateMsg.OrderID, models.StepExecutionStatusRunning, true).
		Preload("Execution").
		Preload("Execution.Template").
		Preload("Execution.Template.OrderSteps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		First(&stepExecution).Error

	if err != nil {
		return // 해당 단계가 없거나 이미 완료됨
	}

	// 액션 완료 상태 확인
	stepResult := e.determineStepResult(stateMsg)
	if stepResult == "" {
		return // 아직 진행 중
	}

	// 단계 완료 처리
	stepExecution.Status = models.StepExecutionStatusCompleted
	stepExecution.Result = stepResult
	stepExecution.ActionCompleted = true
	now := time.Now()
	stepExecution.CompletedAt = &now
	e.db.Save(&stepExecution)

	utils.Logger.Infof("Action completed for order %s, step %d with result: %s",
		stateMsg.OrderID, stepExecution.StepOrder, stepResult)

	// 다음 단계 실행
	stepExecution.Execution.CurrentStep++
	e.db.Save(&stepExecution.Execution)

	e.executeNextStep(&stepExecution.Execution, &stepExecution.Execution.Template, stepResult)
}

// CancelAllRunningOrders 실행 중인 모든 오더 취소
func (e *OrderExecutor) CancelAllRunningOrders() error {
	// 실행 중인 명령 실행들 찾기
	var commandExecutions []models.CommandExecution
	e.db.Where("status = ?", models.CommandExecutionStatusRunning).Find(&commandExecutions)

	for _, cmdExec := range commandExecutions {
		cmdExec.Status = models.CommandExecutionStatusCancelled
		now := time.Now()
		cmdExec.CompletedAt = &now
		e.db.Save(&cmdExec)

		// 해당 명령의 모든 오더 실행들 취소
		var orderExecutions []models.OrderExecution
		e.db.Where("command_execution_id = ? AND status IN ?",
			cmdExec.ID, []string{models.OrderExecutionStatusRunning, models.OrderExecutionStatusPending}).
			Find(&orderExecutions)

		for _, orderExec := range orderExecutions {
			orderExec.Status = models.OrderExecutionStatusFailed
			orderExec.CompletedAt = &now
			e.db.Save(&orderExec)

			// 실행 중인 단계들도 취소
			var stepExecutions []models.StepExecution
			e.db.Where("execution_id = ? AND status = ?", orderExec.ID, models.StepExecutionStatusRunning).
				Find(&stepExecutions)

			for _, stepExec := range stepExecutions {
				stepExec.Status = models.StepExecutionStatusFailed
				stepExec.CompletedAt = &now
				stepExec.ErrorMessage = "Cancelled by order cancel command"
				e.db.Save(&stepExec)
			}

			utils.Logger.Infof("Order execution cancelled: %s", orderExec.OrderID)
		}

		utils.Logger.Infof("Command execution cancelled: %d", cmdExec.ID)
	}

	return nil
}

// SendCancelOrder 취소 명령 전송 (OrderMessageHandler로 위임)
func (e *OrderExecutor) SendCancelOrder() error {
	return e.orderMessageHandler.SendCancelOrder()
}

// 헬퍼 함수들
func (e *OrderExecutor) shouldExecuteStep(step *models.OrderStep, previousResult string) bool {
	if step.PreviousStepResult == models.PreviousResultAny {
		return true
	}

	if step.StepOrder == 0 {
		return true // 첫 번째 단계는 항상 실행
	}

	return step.PreviousStepResult == previousResult
}

func (e *OrderExecutor) determineStepResult(stateMsg *models.RobotStateMessage) string {
	// 액션 상태 확인
	for _, actionState := range stateMsg.ActionStates {
		if actionState.ActionStatus == models.ActionStatusFinished {
			return models.PreviousResultSuccess
		} else if actionState.ActionStatus == models.ActionStatusFailed {
			return models.PreviousResultFailure
		}
	}

	// 에러 확인
	if len(stateMsg.Errors) > 0 {
		for _, errorInfo := range stateMsg.Errors {
			if errorInfo.ErrorLevel == "FATAL" || errorInfo.ErrorLevel == "ERROR" {
				return models.PreviousResultFailure
			}
		}
	}

	return "" // 아직 진행 중
}

func (e *OrderExecutor) getTotalOrderCount(commandExecutionID uint) int {
	var count int64
	e.db.Model(&models.OrderExecution{}).Where("command_execution_id = ?", commandExecutionID).Count(&count)
	return int(count)
}
