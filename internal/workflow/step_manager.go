// internal/workflow/step_manager.go (공통 기능 적용)
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
}

// NewStepManager 새 단계 관리자 생성
func NewStepManager(db *gorm.DB, redisClient *redisClient.Client, orderBuilder *OrderBuilder, messageSender MessageSender) *StepManager {
	return &StepManager{
		db:            db,
		redisClient:   redisClient,
		orderBuilder:  orderBuilder,
		messageSender: messageSender,
	}
}

// ExecuteNextStep 다음 단계 실행
func (s *StepManager) ExecuteNextStep(execution *models.OrderExecution, template *models.OrderTemplate) {
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
		utils.Logger.Infof("Order execution completed: %s", execution.OrderID)
		return
	}

	utils.Logger.Infof("Executing step %d for order %s", currentOrderStep.StepOrder, execution.OrderID)

	stepExecution := &models.StepExecution{
		ExecutionID:         execution.ID,
		StepOrder:           currentOrderStep.StepOrder,
		Status:              constants.StepExecutionStatusRunning,
		ExpectedActionCount: len(currentOrderStep.StepActionMappings),
		StartedAt:           time.Now(),
	}
	s.db.Create(stepExecution)

	// 오더 메시지 생성
	orderMsg := s.orderBuilder.BuildOrderMessage(execution, currentOrderStep)

	// Redis에 액션 상태 초기화 (공통 키 생성기 사용)
	s.initializeActionStatusInRedis(stepExecution, orderMsg)

	// 로봇에 오더 전송
	if err := s.messageSender.SendOrderMessage(orderMsg); err != nil {
		s.handleStepFailure(stepExecution, execution, fmt.Sprintf("failed to send order: %v", err))
		return
	}

	stepExecution.SentToRobot = true
	s.db.Save(stepExecution)

	// WaitForCompletion이 false인 경우에만 즉시 다음 단계로 진행
	if !currentOrderStep.WaitForCompletion {
		now := time.Now()
		repository.UpdateStepExecutionStatus(s.db, stepExecution, constants.StepExecutionStatusFinished, constants.PreviousResultSuccess, "", &now)
		execution.CurrentStep++
		s.db.Save(execution)
		s.ExecuteNextStep(execution, template)
	}
}

// HandleStepCompletion 단계 완료 처리
func (s *StepManager) HandleStepCompletion(stateMsg *models.RobotStateMessage) bool {
	if stateMsg.OrderID == "" {
		return false
	}

	var stepExecution models.StepExecution
	err := s.db.Joins("JOIN order_executions ON step_executions.execution_id = order_executions.id").
		Where("order_executions.order_id = ? AND step_executions.status = ?", stateMsg.OrderID, constants.StepExecutionStatusRunning).
		Preload("Execution.Template").
		First(&stepExecution).Error
	if err != nil {
		return false
	}

	ctx := context.Background()
	redisKey := redis.StepActions(int(stepExecution.ID)) // 공통 키 생성기 사용

	// 액션 상태 업데이트
	for _, actionState := range stateMsg.ActionStates {
		s.redisClient.HSet(ctx, redisKey, actionState.ActionID, actionState.ActionStatus)
	}

	// 모든 액션 상태 확인
	allStatuses, err := s.redisClient.HGetAll(ctx, redisKey).Result()
	if err != nil {
		utils.Logger.Errorf("Failed to get action statuses from Redis for step %d: %v", stepExecution.ID, err)
		return false
	}

	stepResult := s.determineStepResultFromMap(allStatuses, &stepExecution)
	if stepResult == "" {
		return false // 아직 진행 중
	}

	// Redis 정리
	s.redisClient.Del(ctx, redisKey)

	if stepResult == constants.PreviousResultFailure {
		s.handleStepFailure(&stepExecution, &stepExecution.Execution, "Action failed or robot reported a critical error.")
		return true
	}

	// 단계 완료 처리
	now := time.Now()
	repository.UpdateStepExecutionStatus(s.db, &stepExecution, constants.StepExecutionStatusFinished, constants.PreviousResultSuccess, "", &now)

	execution := stepExecution.Execution
	execution.CurrentStep++
	s.db.Save(&execution)

	utils.Logger.Infof("Step %d completed for order %s, moving to step %d",
		stepExecution.StepOrder, execution.OrderID, execution.CurrentStep)

	s.ExecuteNextStep(&execution, &execution.Template)
	return true
}

// CancelRunningSteps 실행 중인 단계들 취소 (공통 상수 사용)
func (s *StepManager) CancelRunningSteps(orderExecutionID uint, reason string) {
	var stepExecutions []models.StepExecution
	s.db.Where("execution_id = ? AND status = ?", orderExecutionID, constants.StepExecutionStatusRunning).
		Find(&stepExecutions)

	for _, stepExec := range stepExecutions {
		now := time.Now()
		repository.UpdateStepExecutionStatus(s.db, &stepExec, constants.StepExecutionStatusFailed, "", reason, &now)

		// Redis 정리 (공통 키 생성기 사용)
		ctx := context.Background()
		redisKey := redis.StepActions(int(stepExec.ID))
		s.redisClient.Del(ctx, redisKey)
	}
}

// initializeActionStatusInRedis Redis에 액션 상태 초기화 (공통 키 생성기 사용)
func (s *StepManager) initializeActionStatusInRedis(stepExec *models.StepExecution, orderMsg *models.OrderMessage) {
	ctx := context.Background()
	redisKey := redis.StepActions(int(stepExec.ID)) // 공통 키 생성기 사용
	s.redisClient.Del(ctx, redisKey)

	pipe := s.redisClient.Pipeline()
	for _, node := range orderMsg.Nodes {
		for _, action := range node.Actions {
			pipe.HSet(ctx, redisKey, action.ActionID, constants.ActionStatusWaiting)
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		utils.Logger.Errorf("Failed to initialize action status in Redis for step %d: %v", stepExec.ID, err)
	}
}

// handleStepFailure 단계 실패 처리 (공통 상수 사용)
func (s *StepManager) handleStepFailure(step *models.StepExecution, order *models.OrderExecution, reason string) {
	now := time.Now()
	repository.UpdateStepExecutionStatus(s.db, step, constants.StepExecutionStatusFailed, "", reason, &now)
	repository.UpdateOrderExecutionStatus(s.db, order, constants.OrderExecutionStatusFailed, &now)

	// Redis 정리 (공통 키 생성기 사용)
	ctx := context.Background()
	redisKey := redis.StepActions(int(step.ID))
	s.redisClient.Del(ctx, redisKey)

	utils.Logger.Errorf("Step %d failed for order %s: %s", step.StepOrder, order.OrderID, reason)
}

// determineStepResultFromMap 액션 상태 맵을 기반으로 단계 결과 결정 (공통 상수 사용)
func (s *StepManager) determineStepResultFromMap(statuses map[string]string, stepExec *models.StepExecution) string {
	if len(statuses) < stepExec.ExpectedActionCount {
		return ""
	}

	allFinished := true
	for _, status := range statuses {
		switch status {
		case constants.ActionStatusFailed:
			return constants.PreviousResultFailure
		case constants.ActionStatusFinished:
			continue
		default:
			allFinished = false
		}
	}

	if allFinished {
		return constants.PreviousResultSuccess
	}

	return ""
}

// GetRunningSteps 실행 중인 단계들 조회 (공통 상수 사용)
func (s *StepManager) GetRunningSteps() ([]models.StepExecution, error) {
	var steps []models.StepExecution
	err := s.db.Where("status = ?", constants.StepExecutionStatusRunning).
		Preload("Execution").Find(&steps).Error
	return steps, err
}

// GetStepsByOrderID 특정 오더의 모든 단계 조회
func (s *StepManager) GetStepsByOrderID(orderExecutionID uint) ([]models.StepExecution, error) {
	var steps []models.StepExecution
	err := s.db.Where("execution_id = ?", orderExecutionID).
		Order("step_order ASC").Find(&steps).Error
	return steps, err
}
