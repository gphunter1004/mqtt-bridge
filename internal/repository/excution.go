package repository

import (
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	"gorm.io/gorm"
)

// UpdateCommandStatus 는 Command의 최종 상태를 업데이트합니다.
func UpdateCommandStatus(db *gorm.DB, command *models.Command, status, errMsg string) {
	command.Status = status
	if errMsg != "" {
		command.ErrorMessage = errMsg
	}
	now := time.Now()
	command.ResponseTime = &now
	db.Save(command)
	utils.Logger.Infof("Command %d status updated to %s", command.ID, status)
}

// UpdateCommandExecutionStatus 는 CommandExecution의 상태를 업데이트합니다.
func UpdateCommandExecutionStatus(db *gorm.DB, exec *models.CommandExecution, status string, completedAt *time.Time) {
	exec.Status = status
	if completedAt != nil {
		exec.CompletedAt = completedAt
	}
	db.Save(exec)
	utils.Logger.Infof("CommandExecution %d status updated to %s", exec.ID, status)
}

// UpdateOrderExecutionStatus 는 OrderExecution의 상태를 업데이트합니다.
func UpdateOrderExecutionStatus(db *gorm.DB, exec *models.OrderExecution, status string, completedAt *time.Time) {
	exec.Status = status
	if completedAt != nil {
		exec.CompletedAt = completedAt
	}
	db.Save(exec)
	utils.Logger.Infof("OrderExecution for order %s status updated to %s", exec.OrderID, status)
}

// UpdateStepExecutionStatus 는 StepExecution의 상태를 업데이트합니다.
func UpdateStepExecutionStatus(db *gorm.DB, exec *models.StepExecution, status, result, errMsg string, completedAt *time.Time) {
	exec.Status = status
	exec.Result = result
	if errMsg != "" {
		exec.ErrorMessage = errMsg
	}
	if completedAt != nil {
		exec.CompletedAt = completedAt
	}
	db.Save(exec)
	utils.Logger.Infof("StepExecution %d for order %d status updated to %s", exec.ID, exec.ExecutionID, status)
}
