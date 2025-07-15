// internal/command/statemachine.go (수정됨: 파일 통합 및 컴파일 오류 해결)
package command

import (
	"context"
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/messaging"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"

	"github.com/looplab/fsm"
	"gorm.io/gorm"
)

// CommandStateMachine은 단일 명령(표준 또는 직접)의 생명주기를 관리합니다.
type CommandStateMachine struct {
	FSM              *fsm.FSM
	db               *gorm.DB
	plcSender        *messaging.PLCResponseSender
	workflowExecutor WorkflowExecutor

	// 상태 머신이 관리하는 데이터
	IsDirectAction   bool
	FullCommand      string
	Command          *models.Command // 표준 명령용. 직접 액션의 경우 nil
	CommandExecution *models.CommandExecution
	OrderID          string // 직접 액션용
}

// NewCommandStateMachine은 새 상태 머신 인스턴스를 생성합니다.
func NewCommandStateMachine(
	db *gorm.DB,
	plcSender *messaging.PLCResponseSender,
	executor WorkflowExecutor,
) *CommandStateMachine {
	csm := &CommandStateMachine{
		db:               db,
		plcSender:        plcSender,
		workflowExecutor: executor,
	}
	csm.initializeFSM()
	return csm
}

// ForStandardCommand는 표준 명령용으로 상태 머신을 설정합니다.
func (csm *CommandStateMachine) ForStandardCommand(cmd *models.Command) *CommandStateMachine {
	csm.IsDirectAction = false
	csm.Command = cmd
	csm.FullCommand = cmd.CommandDefinition.CommandType
	return csm
}

// ForDirectAction은 직접 액션용으로 상태 머신을 설정합니다.
func (csm *CommandStateMachine) ForDirectAction(fullCommand, orderID string) *CommandStateMachine {
	csm.IsDirectAction = true
	csm.FullCommand = fullCommand
	csm.OrderID = orderID
	// 생성과 동시에 order_sent 이벤트를 발생시켜 Acknowledged 상태로 전환
	csm.FSM.Event(context.Background(), "order_sent")
	return csm
}

// initializeFSM은 FSM의 상태와 콜백을 설정합니다.
func (csm *CommandStateMachine) initializeFSM() {
	callbacks := fsm.Callbacks{
		"enter_state":        csm.onEnterState,
		"enter_Acknowledged": csm.onEnterAcknowledged,
		"enter_Running": func(ctx context.Context, e *fsm.Event) {
			utils.Logger.Infof("COMMAND '%s' is now in Running state.", csm.FullCommand)
		},
		"enter_Completed": csm.onEnterCompleted,
		"enter_Failed":    csm.onEnterFailed,
	}

	csm.FSM = fsm.NewFSM(
		"Pending",
		fsm.Events{
			{Name: "order_sent", Src: []string{"Pending"}, Dst: "Acknowledged"},
			{Name: "robot_started_running", Src: []string{"Acknowledged"}, Dst: "Running"},
			{Name: "robot_order_finished", Src: []string{"Running"}, Dst: "Running"},
			{Name: "succeeded", Src: []string{"Running"}, Dst: "Completed"},
			{Name: "robot_failed", Src: []string{"Pending", "Acknowledged", "Running"}, Dst: "Failed"},
			{Name: "command_succeeded", Src: []string{"Running"}, Dst: "Completed"},
		},
		callbacks,
	)
}

func (csm *CommandStateMachine) onEnterState(ctx context.Context, e *fsm.Event) {
	utils.Logger.Infof("COMMAND '%s': state changed from %s -> %s (Event: %s)", csm.FullCommand, e.Src, e.Dst, e.Event)
}

func (csm *CommandStateMachine) onEnterAcknowledged(ctx context.Context, e *fsm.Event) {
	csm.plcSender.SendResponse(csm.FullCommand, constants.StatusAcknowledged, "Order acknowledged by robot")
}

func (csm *CommandStateMachine) onEnterCompleted(ctx context.Context, e *fsm.Event) {
	if csm.IsDirectAction {
		csm.plcSender.SendResponse(csm.FullCommand, constants.StatusSuccess, "Direct action completed successfully")
	} else {
		utils.Logger.Infof("COMMAND '%s' workflow finished. Final status will be determined by Executor.", csm.FullCommand)
	}
}

func (csm *CommandStateMachine) onEnterFailed(ctx context.Context, e *fsm.Event) {
	errMsg := "Command failed"
	if len(e.Args) > 0 {
		if str, ok := e.Args[0].(string); ok {
			errMsg = str
		} else if err, ok := e.Args[0].(error); ok {
			errMsg = err.Error()
		}
	}
	if csm.IsDirectAction {
		csm.plcSender.SendResponse(csm.FullCommand, constants.StatusFailure, errMsg)
	} else {
		// 표준 명령 실패 시 최종 응답은 Executor가 담당하므로 로그만 남김
		utils.Logger.Warnf("COMMAND '%s' workflow failed. Final status will be determined by Executor.", csm.FullCommand)
	}
}

// StartWorkflow는 '표준 명령'의 워크플로우 실행을 시작합니다.
func (csm *CommandStateMachine) StartWorkflow() error {
	if csm.IsDirectAction {
		return fmt.Errorf("StartWorkflow can only be called for standard commands")
	}

	if err := csm.FSM.Event(context.Background(), "order_sent"); err != nil {
		csm.Fail(err.Error())
		return err
	}

	go func() {
		// (수정!) ExecuteCommandOrder에는 *models.Command 타입인 csm.Command를 전달합니다.
		err := csm.workflowExecutor.ExecuteCommandOrder(csm.Command)
		if err != nil {
			csm.Fail(fmt.Sprintf("workflow execution failed: %v", err))
		}
	}()

	return nil
}

// HandleRobotStateUpdate는 로봇 상태 메시지를 받아 적절한 FSM 이벤트를 발생시킵니다.
func (csm *CommandStateMachine) HandleRobotStateUpdate(stateMsg *models.RobotStateMessage) {
	if !csm.IsRelevantOrder(stateMsg.OrderID) {
		return
	}

	hasRunning, hasFinished, hasFailed := false, true, false
	for _, action := range stateMsg.ActionStates {
		switch action.ActionStatus {
		case constants.ActionStatusRunning:
			hasRunning = true
			hasFinished = false
		case constants.ActionStatusFailed:
			hasFailed = true
		case constants.ActionStatusWaiting, constants.ActionStatusInitializing:
			hasFinished = false
		}
	}

	if hasRunning && csm.FSM.Is("Running") {
		csm.plcSender.SendResponse(csm.GetFullCommand(), constants.StatusRunning, "Order is running")
	}

	if hasFailed {
		csm.Fail("An action has failed")
	} else if hasFinished && csm.FSM.Is("Running") {
		if csm.IsDirectAction {
			csm.FSM.Event(context.Background(), "succeeded")
		} else {
			csm.FSM.Event(context.Background(), "robot_order_finished")
		}
	} else if hasRunning && csm.FSM.Is("Acknowledged") {
		csm.FSM.Event(context.Background(), "robot_started_running")
	}
}

func (csm *CommandStateMachine) IsRelevantOrder(receivedOrderID string) bool {
	if csm.IsDirectAction {
		return csm.OrderID == receivedOrderID
	}
	if csm.Command == nil {
		return false
	}
	// 표준 명령의 경우, CommandExecution 기록을 찾아 OrderID를 비교
	var execution models.CommandExecution
	if err := csm.db.Where("command_id = ?", csm.Command.ID).First(&execution).Error; err != nil {
		return false
	}
	var orderIDs []string
	csm.db.Model(&models.OrderExecution{}).Where("command_execution_id = ?", execution.ID).Pluck("order_id", &orderIDs)
	for _, id := range orderIDs {
		if id == receivedOrderID {
			return true
		}
	}
	return false
}

func (csm *CommandStateMachine) Fail(reason string) {
	csm.FSM.Event(context.Background(), "robot_failed", reason)
}

func (csm *CommandStateMachine) GetFullCommand() string {
	return csm.FullCommand
}
