// internal/command/processor.go (공통 기능 적용)
package command

import (
	"context"
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/common/redis"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/repository"
	"mqtt-bridge/internal/utils"
	"time"

	redisClient "github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// RobotStatusChecker 로봇 상태 확인 인터페이스
type RobotStatusChecker interface {
	IsOnline(serialNumber string) bool
}

// WorkflowExecutor 워크플로우 실행 인터페이스
type WorkflowExecutor interface {
	ExecuteCommandOrder(command *models.Command) error
	SendDirectActionOrder(baseCommand string, commandType rune, armParam string) (string, error)
	CancelAllRunningOrders() error
}

// Processor 명령 처리 로직
type Processor struct {
	db               *gorm.DB
	redisClient      *redisClient.Client
	config           *config.Config
	robotChecker     RobotStatusChecker
	workflowExecutor WorkflowExecutor
}

// NewProcessor 새 프로세서 생성
func NewProcessor(db *gorm.DB, redisClient *redisClient.Client, cfg *config.Config,
	robotChecker RobotStatusChecker, workflowExecutor WorkflowExecutor) *Processor {
	return &Processor{
		db:               db,
		redisClient:      redisClient,
		config:           cfg,
		robotChecker:     robotChecker,
		workflowExecutor: workflowExecutor,
	}
}

// ProcessDirectAction 직접 액션 명령 처리
func (p *Processor) ProcessDirectAction(req DirectActionRequest) (*CommandResult, error) {
	utils.Logger.Infof("Processing direct action: %s, Type: %c, Arm: %s",
		req.BaseCommand, req.CommandType, req.ArmParam)

	// 명령 타입 유효성 검사
	if !IsValidCommandType(req.CommandType) {
		return &CommandResult{
			Command:   req.FullCommand,
			Status:    constants.StatusFailure,
			Message:   fmt.Sprintf("Invalid command type: %c", req.CommandType),
			Timestamp: time.Now(),
		}, nil
	}

	// 팔 파라미터 유효성 검사 (T 타입인 경우)
	if req.CommandType == constants.CommandTypeTrajectory && !ValidateArmParam(req.ArmParam) {
		return &CommandResult{
			Command:   req.FullCommand,
			Status:    constants.StatusFailure,
			Message:   fmt.Sprintf("Invalid arm parameter: %s (use R or L)", req.ArmParam),
			Timestamp: time.Now(),
		}, nil
	}

	// 로봇 온라인 상태 확인
	if !p.robotChecker.IsOnline(p.config.RobotSerialNumber) {
		return &CommandResult{
			Command:   req.FullCommand,
			Status:    constants.StatusFailure,
			Message:   "Robot is not online",
			Timestamp: time.Now(),
		}, nil
	}

	// 워크플로우 실행기를 통해 오더 전송
	orderID, err := p.workflowExecutor.SendDirectActionOrder(req.BaseCommand, req.CommandType, req.ArmParam)
	if err != nil {
		return &CommandResult{
			Command:   req.FullCommand,
			Status:    constants.StatusFailure,
			Message:   fmt.Sprintf("Failed to send order: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// Redis에 대기 중인 명령 저장 (공통 키 생성기 사용)
	if err := p.storePendingDirectCommand(req.FullCommand, orderID); err != nil {
		utils.Logger.Errorf("Failed to store pending command: %v", err)
		// Redis 저장 실패해도 명령은 이미 전송됨
	}

	utils.Logger.Infof("Direct action order sent successfully: %s (OrderID: %s)",
		req.FullCommand, orderID)

	return &CommandResult{
		Command:   req.FullCommand,
		Status:    constants.StatusSuccess,
		OrderID:   orderID,
		Message:   "Order sent successfully",
		Timestamp: time.Now(),
	}, nil
}

// ProcessStandardCommand 표준 명령 처리 (CR, GR, OC 등)
func (p *Processor) ProcessStandardCommand(command *models.Command) (*CommandResult, error) {
	// CommandDefinition 로드
	p.db.Preload("CommandDefinition").First(&command, command.ID)

	utils.Logger.Infof("Processing standard command: %s (ID: %d)",
		command.CommandDefinition.CommandType, command.ID)

	// 취소 명령 특별 처리
	if command.CommandDefinition.CommandType == constants.CommandOrderCancel {
		if err := p.workflowExecutor.CancelAllRunningOrders(); err != nil {
			repository.UpdateCommandStatus(p.db, command, constants.CommandStatusFailure, err.Error())
			return &CommandResult{
				Command:   command.CommandDefinition.CommandType,
				Status:    constants.StatusFailure,
				Message:   err.Error(),
				Timestamp: time.Now(),
			}, nil
		}

		repository.UpdateCommandStatus(p.db, command, constants.CommandStatusSuccess, "")
		return &CommandResult{
			Command:   command.CommandDefinition.CommandType,
			Status:    constants.StatusSuccess,
			Message:   "All orders cancelled",
			Timestamp: time.Now(),
		}, nil
	}

	// 로봇 온라인 상태 확인
	if !p.robotChecker.IsOnline(p.config.RobotSerialNumber) {
		errMsg := "Robot is not online"
		repository.UpdateCommandStatus(p.db, command, constants.CommandStatusFailure, errMsg)
		return &CommandResult{
			Command:   command.CommandDefinition.CommandType,
			Status:    constants.StatusFailure,
			Message:   errMsg,
			Timestamp: time.Now(),
		}, nil
	}

	// 처리 상태로 업데이트
	repository.UpdateCommandStatus(p.db, command, constants.CommandStatusProcessing, "")

	// 워크플로우 실행
	if err := p.workflowExecutor.ExecuteCommandOrder(command); err != nil {
		errMsg := fmt.Sprintf("Failed to start command execution: %v", err)
		repository.UpdateCommandStatus(p.db, command, constants.CommandStatusFailure, errMsg)
		return &CommandResult{
			Command:   command.CommandDefinition.CommandType,
			Status:    constants.StatusFailure,
			Message:   errMsg,
			Timestamp: time.Now(),
		}, err
	}

	return &CommandResult{
		Command:   command.CommandDefinition.CommandType,
		Status:    constants.StatusSuccess,
		Message:   "Command execution started",
		Timestamp: time.Now(),
	}, nil
}

// HandleDirectCommandStateUpdate state 메시지를 통한 직접 명령 결과 처리
func (p *Processor) HandleDirectCommandStateUpdate(stateMsg *models.RobotStateMessage) *CommandResult {
	if stateMsg.OrderID == "" {
		return nil
	}

	ctx := context.Background()
	key := redis.PendingDirectCommand(stateMsg.OrderID) // 공통 키 생성기 사용

	// Redis에서 대기 중인 명령 확인
	commandData, err := p.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(commandData) == 0 {
		return nil // 대기 중인 직접 명령이 아님
	}

	fullCommand := commandData["full_command"]
	if fullCommand == "" {
		return nil
	}

	// 액션 상태 확인
	result := p.determineDirectCommandResult(stateMsg.ActionStates)

	if result != "" {
		// 결과가 확정되면 Redis에서 제거
		p.redisClient.Del(ctx, key)

		utils.Logger.Infof("Direct command completed: %s -> %s", fullCommand, result)

		return &CommandResult{
			Command:   fullCommand,
			Status:    result,
			OrderID:   stateMsg.OrderID,
			Message:   "Command completed based on robot state",
			Timestamp: time.Now(),
		}
	}

	return nil // 아직 진행 중
}

// FailAllPendingCommands 모든 대기 중인 명령 실패 처리
func (p *Processor) FailAllPendingCommands(reason string) []CommandResult {
	var results []CommandResult

	ctx := context.Background()
	pattern := redis.AllPendingDirectCommands() // 공통 패턴 사용
	keys, err := p.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		utils.Logger.Errorf("Failed to get pending commands: %v", err)
		return results
	}

	for _, key := range keys {
		commandData, err := p.redisClient.HGetAll(ctx, key).Result()
		if err == nil && len(commandData) > 0 {
			fullCommand := commandData["full_command"]
			orderID := commandData["order_id"]

			if fullCommand != "" {
				results = append(results, CommandResult{
					Command:   fullCommand,
					Status:    constants.StatusFailure,
					OrderID:   orderID,
					Message:   reason,
					Timestamp: time.Now(),
				})

				p.redisClient.Del(ctx, key)
			}
		}
	}

	return results
}

// storePendingDirectCommand Redis에 대기 중인 직접 명령 저장
func (p *Processor) storePendingDirectCommand(fullCommand, orderID string) error {
	ctx := context.Background()
	key := redis.PendingDirectCommand(orderID) // 공통 키 생성기 사용

	commandData := map[string]interface{}{
		"full_command": fullCommand,
		"order_id":     orderID,
		"timestamp":    time.Now().Unix(),
	}

	return p.redisClient.HMSet(ctx, key, commandData).Err()
}

// determineDirectCommandResult 액션 상태를 기반으로 명령 결과 결정
func (p *Processor) determineDirectCommandResult(actionStates []models.ActionState) string {
	if len(actionStates) == 0 {
		return ""
	}

	allFinished := true
	hasFailure := false

	for _, action := range actionStates {
		switch action.ActionStatus {
		case constants.ActionStatusFailed:
			hasFailure = true
		case constants.ActionStatusFinished:
			continue
		default:
			allFinished = false
		}
	}

	if hasFailure {
		return constants.StatusFailure
	}

	if allFinished {
		return constants.StatusSuccess
	}

	return "" // 아직 진행 중
}
