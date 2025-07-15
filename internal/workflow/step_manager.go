// internal/workflow/step_manager.go (완전한 버전 - 누락된 메서드와 인터페이스 포함)
package workflow

import (
	"context"
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/common/redis"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/repository"
	"mqtt-bridge/internal/utils"
	"time"

	redisClient "github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// MessageSender 메시지 전송 인터페이스
type MessageSender interface {
	SendOrderMessage(orderMsg *models.OrderMessage) error
}

// StepManager 워크플로우 단계 관리
type StepManager struct {
	db            *gorm.DB
	redisClient   *redisClient.Client
	orderBuilder  *OrderBuilder
	messageSender MessageSender
	executor      *Executor // 🔥 Executor 참조 추가
}

// NewStepManager 새 단계 관리자 생성
func NewStepManager(db *gorm.DB, redisClient *redisClient.Client, orderBuilder *OrderBuilder, messageSender MessageSender) *StepManager {
	return &StepManager{
		db:            db,
		redisClient:   redisClient,
		orderBuilder:  orderBuilder,
		messageSender: messageSender,
		executor:      nil, // 기본값은 nil
	}
}

// SetExecutor Executor 참조 설정 (순환 의존성 해결용)
func (s *StepManager) SetExecutor(executor *Executor) {
	s.executor = executor
	utils.Logger.Infof("✅ StepManager: Executor reference set")
}

// ExecuteNextStep 다음 단계 실행
func (s *StepManager) ExecuteNextStep(execution *models.OrderExecution, template *models.OrderTemplate) {
	utils.Logger.Infof("🚀 ExecuteNextStep called: OrderID=%s, CurrentStep=%d",
		execution.OrderID, execution.CurrentStep)

	// 현재 단계에 해당하는 OrderStep 찾기
	var currentOrderStep *models.OrderStep
	for i := range template.OrderSteps {
		if template.OrderSteps[i].StepOrder == execution.CurrentStep {
			currentOrderStep = &template.OrderSteps[i]
			break
		}
	}

	if currentOrderStep == nil {
		// 모든 단계 완료
		now := time.Now()
		repository.UpdateOrderExecutionStatus(s.db, execution, constants.OrderExecutionStatusCompleted, &now)
		utils.Logger.Infof("🏁 Order execution completed: %s (no more steps)", execution.OrderID)

		// 🔥 워크플로우 실행기에 완료 알림
		s.notifyWorkflowExecutor(execution, true)
		return
	}

	utils.Logger.Infof("🔧 Executing step %d for order %s: %s",
		currentOrderStep.StepOrder, execution.OrderID,
		fmt.Sprintf("StepID=%d, WaitForCompletion=%t", currentOrderStep.ID, currentOrderStep.WaitForCompletion))

	// ExpectedActionCount 정확히 계산
	expectedCount := len(currentOrderStep.StepActionMappings)
	if expectedCount == 0 {
		expectedCount = 1 // 최소 1개의 액션은 있어야 함
	}

	stepExecution := &models.StepExecution{
		ExecutionID:         execution.ID,
		StepOrder:           currentOrderStep.StepOrder,
		Status:              constants.StepExecutionStatusRunning,
		ExpectedActionCount: expectedCount,
		StartedAt:           time.Now(),
	}

	if err := s.db.Create(stepExecution).Error; err != nil {
		utils.Logger.Errorf("❌ Failed to create step execution: %v", err)
		return
	}

	utils.Logger.Infof("📝 Step execution created: ID=%d, ExpectedActionCount=%d",
		stepExecution.ID, stepExecution.ExpectedActionCount)

	// 오더 메시지 생성
	orderMsg := s.orderBuilder.BuildOrderMessage(execution, currentOrderStep)

	// Redis에 액션 상태 초기화
	s.initializeActionStatusInRedis(stepExecution, orderMsg)

	// 로봇에 오더 전송
	if err := s.messageSender.SendOrderMessage(orderMsg); err != nil {
		s.handleStepFailure(stepExecution, execution, fmt.Sprintf("failed to send order: %v", err))
		return
	}

	stepExecution.SentToRobot = true
	s.db.Save(stepExecution)

	utils.Logger.Infof("📤 Order sent to robot: OrderID=%s, StepOrder=%d", execution.OrderID, currentOrderStep.StepOrder)

	// WaitForCompletion 처리
	if !currentOrderStep.WaitForCompletion {
		utils.Logger.Infof("⚡ Step %d does not wait for completion, moving to next step immediately", currentOrderStep.StepOrder)
		now := time.Now()
		repository.UpdateStepExecutionStatus(s.db, stepExecution, constants.StepExecutionStatusFinished, constants.PreviousResultSuccess, "", &now)
		execution.CurrentStep++
		s.db.Save(execution)
		s.ExecuteNextStep(execution, template)
	} else {
		utils.Logger.Infof("⏳ Step %d waiting for completion", currentOrderStep.StepOrder)
	}
}

// HandleStepCompletion 단계 완료 처리
func (s *StepManager) HandleStepCompletion(stateMsg *models.RobotStateMessage) bool {
	if stateMsg.OrderID == "" {
		utils.Logger.Debugf("🔍 State message has no OrderID, skipping")
		return false
	}

	utils.Logger.Infof("🔍 Checking step completion for OrderID: %s", stateMsg.OrderID)

	// 실행 중인 단계 조회
	var stepExecution models.StepExecution
	err := s.db.Joins("JOIN order_executions ON step_executions.execution_id = order_executions.id").
		Where("order_executions.order_id = ? AND step_executions.status = ?",
			stateMsg.OrderID, constants.StepExecutionStatusRunning).
		Preload("Execution.Template").
		First(&stepExecution).Error

	if err != nil {
		utils.Logger.Debugf("🔍 No running step found for OrderID: %s (%v)", stateMsg.OrderID, err)
		return false
	}

	utils.Logger.Infof("🔍 Found running step: ID=%d, StepOrder=%d, ExecutionID=%d",
		stepExecution.ID, stepExecution.StepOrder, stepExecution.ExecutionID)

	// 액션 상태 디버그 로깅
	utils.Logger.Infof("🔍 Analyzing %d action states:", len(stateMsg.ActionStates))
	for i, action := range stateMsg.ActionStates {
		utils.Logger.Infof("🔍 Action[%d]: ID=%s, Type=%s, Status=%s",
			i, action.ActionID, action.ActionType, action.ActionStatus)
	}

	ctx := context.Background()
	redisKey := redis.StepActions(int(stepExecution.ID))

	// 액션 상태 업데이트
	for _, actionState := range stateMsg.ActionStates {
		utils.Logger.Debugf("🔍 Updating Redis: %s -> %s", actionState.ActionID, actionState.ActionStatus)
		s.redisClient.HSet(ctx, redisKey, actionState.ActionID, actionState.ActionStatus)
	}

	// 모든 액션 상태 확인
	allStatuses, err := s.redisClient.HGetAll(ctx, redisKey).Result()
	if err != nil {
		utils.Logger.Errorf("❌ Failed to get action statuses from Redis for step %d: %v", stepExecution.ID, err)
		return false
	}

	utils.Logger.Infof("🔍 Redis action statuses: %+v", allStatuses)

	// 단계 결과 결정
	stepResult := s.determineStepResultFromActions(stateMsg.ActionStates, &stepExecution)

	utils.Logger.Infof("🔍 Step result determined: '%s' for step %d", stepResult, stepExecution.StepOrder)

	if stepResult == "" {
		utils.Logger.Infof("🔍 Step %d still in progress", stepExecution.StepOrder)
		return false // 아직 진행 중
	}

	// Redis 정리
	s.redisClient.Del(ctx, redisKey)

	if stepResult == constants.PreviousResultFailure {
		utils.Logger.Errorf("❌ Step %d failed", stepExecution.StepOrder)
		s.handleStepFailure(&stepExecution, &stepExecution.Execution, "Action failed or robot reported a critical error.")
		return true
	}

	// 단계 완료 처리
	utils.Logger.Infof("✅ Step %d completed successfully", stepExecution.StepOrder)
	now := time.Now()
	repository.UpdateStepExecutionStatus(s.db, &stepExecution, constants.StepExecutionStatusFinished, constants.PreviousResultSuccess, "", &now)

	execution := stepExecution.Execution
	execution.CurrentStep++
	s.db.Save(&execution)

	utils.Logger.Infof("📈 Moving to next step: OrderID=%s, CurrentStep=%d -> %d",
		execution.OrderID, stepExecution.StepOrder, execution.CurrentStep)

	// 다음 단계가 있는지 확인
	if execution.CurrentStep > len(execution.Template.OrderSteps) {
		// 모든 단계 완료
		now := time.Now()
		repository.UpdateOrderExecutionStatus(s.db, &execution, constants.OrderExecutionStatusCompleted, &now)
		utils.Logger.Infof("🏁 All steps completed for OrderID: %s", execution.OrderID)

		// 워크플로우 실행기에 완료 알림
		s.notifyWorkflowExecutor(&execution, true)
		return true
	}

	// 다음 단계 실행
	s.ExecuteNextStep(&execution, &execution.Template)
	return true
}

// CancelRunningSteps 실행 중인 단계들 취소
func (s *StepManager) CancelRunningSteps(orderExecutionID uint, reason string) {
	var stepExecutions []models.StepExecution
	s.db.Where("execution_id = ? AND status = ?", orderExecutionID, constants.StepExecutionStatusRunning).
		Find(&stepExecutions)

	for _, stepExec := range stepExecutions {
		now := time.Now()
		repository.UpdateStepExecutionStatus(s.db, &stepExec, constants.StepExecutionStatusFailed, "", reason, &now)

		// Redis 정리
		ctx := context.Background()
		redisKey := redis.StepActions(int(stepExec.ID))
		s.redisClient.Del(ctx, redisKey)
	}
}

// determineStepResultFromActions 액션 상태 기반 단계 결과 결정
func (s *StepManager) determineStepResultFromActions(actionStates []models.ActionState, stepExec *models.StepExecution) string {
	if len(actionStates) == 0 {
		utils.Logger.Debugf("🔍 No action states to evaluate")
		return ""
	}

	utils.Logger.Infof("🔍 Determining step result from %d actions (expected: %d)",
		len(actionStates), stepExec.ExpectedActionCount)

	allFinished := true
	hasFailure := false
	finishedCount := 0
	failedCount := 0

	for _, action := range actionStates {
		utils.Logger.Debugf("🔍 Evaluating action: %s -> %s", action.ActionID, action.ActionStatus)

		switch action.ActionStatus {
		case constants.ActionStatusFailed:
			utils.Logger.Infof("🔍 Action failed: %s", action.ActionID)
			hasFailure = true
			failedCount++
		case constants.ActionStatusFinished:
			utils.Logger.Debugf("🔍 Action finished: %s", action.ActionID)
			finishedCount++
		default:
			utils.Logger.Debugf("🔍 Action still running: %s -> %s", action.ActionID, action.ActionStatus)
			allFinished = false
		}
	}

	utils.Logger.Infof("🔍 Action summary: finished=%d, failed=%d, total=%d, expected=%d",
		finishedCount, failedCount, len(actionStates), stepExec.ExpectedActionCount)

	if hasFailure {
		utils.Logger.Infof("🔍 Result: FAILURE (some actions failed)")
		return constants.PreviousResultFailure
	}

	// 모든 예상 액션이 완료되었는지 확인
	if finishedCount >= stepExec.ExpectedActionCount {
		utils.Logger.Infof("🔍 Result: SUCCESS (all expected actions finished: %d/%d)",
			finishedCount, stepExec.ExpectedActionCount)
		return constants.PreviousResultSuccess
	}

	if allFinished && len(actionStates) > 0 {
		utils.Logger.Infof("🔍 Result: SUCCESS (all reported actions finished)")
		return constants.PreviousResultSuccess
	}

	utils.Logger.Debugf("🔍 Result: IN_PROGRESS (actions still running or incomplete)")
	return "" // 아직 진행 중
}

// notifyWorkflowExecutor 워크플로우 실행기에 알림
func (s *StepManager) notifyWorkflowExecutor(execution *models.OrderExecution, success bool) {
	if s.executor != nil {
		utils.Logger.Infof("📢 Calling executor OnOrderCompleted: OrderID=%s, Success=%t",
			execution.OrderID, success)
		s.executor.OnOrderCompleted(execution, success)
	} else {
		utils.Logger.Warnf("⚠️ Executor not set, cannot notify completion for OrderID: %s", execution.OrderID)
	}
}

// handleStepFailure 단계 실패 처리
func (s *StepManager) handleStepFailure(step *models.StepExecution, order *models.OrderExecution, reason string) {
	now := time.Now()
	repository.UpdateStepExecutionStatus(s.db, step, constants.StepExecutionStatusFailed, "", reason, &now)
	repository.UpdateOrderExecutionStatus(s.db, order, constants.OrderExecutionStatusFailed, &now)

	// Redis 정리
	ctx := context.Background()
	redisKey := redis.StepActions(int(step.ID))
	s.redisClient.Del(ctx, redisKey)

	utils.Logger.Errorf("❌ Step %d failed for order %s: %s", step.StepOrder, order.OrderID, reason)

	// 워크플로우 실행기에 실패 알림
	s.notifyWorkflowExecutor(order, false)
}

// initializeActionStatusInRedis Redis에 액션 상태 초기화
func (s *StepManager) initializeActionStatusInRedis(stepExec *models.StepExecution, orderMsg *models.OrderMessage) {
	ctx := context.Background()
	redisKey := redis.StepActions(int(stepExec.ID))
	s.redisClient.Del(ctx, redisKey)

	pipe := s.redisClient.Pipeline()
	actionCount := 0

	for _, node := range orderMsg.Nodes {
		for _, action := range node.Actions {
			pipe.HSet(ctx, redisKey, action.ActionID, constants.ActionStatusWaiting)
			actionCount++
			utils.Logger.Debugf("🔧 Initialized Redis action: %s -> %s", action.ActionID, constants.ActionStatusWaiting)
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		utils.Logger.Errorf("❌ Failed to initialize action status in Redis for step %d: %v", stepExec.ID, err)
	} else {
		utils.Logger.Infof("✅ Initialized %d actions in Redis for step %d", actionCount, stepExec.ID)
	}

	// ExpectedActionCount 업데이트 (실제 생성된 액션 수와 맞춤)
	if actionCount != stepExec.ExpectedActionCount {
		utils.Logger.Warnf("⚠️ Expected action count mismatch: expected=%d, actual=%d",
			stepExec.ExpectedActionCount, actionCount)
		stepExec.ExpectedActionCount = actionCount
		s.db.Save(stepExec)
	}
}
