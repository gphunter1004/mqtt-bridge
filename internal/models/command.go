package models

import (
	"time"

	"gorm.io/gorm"
)

type Command struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	CommandType  string         `gorm:"size:10;not null" json:"command_type"`
	Status       string         `gorm:"size:20;not null" json:"status"`
	RequestTime  time.Time      `gorm:"not null" json:"request_time"`
	ResponseTime *time.Time     `json:"response_time"`
	ErrorMessage string         `gorm:"size:500" json:"error_message"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

const (
	// 명령 타입
	CommandCataractRemoval = "CR" // 백내장 적출
	CommandGlaucomaRemoval = "GR" // 적내장 적출
	CommandGripperCleaning = "GC" // 그리퍼 세정
	CommandCameraCheck     = "CC" // 카메라 확인
	CommandCameraCleaning  = "CL" // 카메라 세정
	CommandKnifeCleaning   = "KC" // 나이프 세정

	// 상태
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusSuccess    = "SUCCESS"
	StatusFailure    = "FAILURE"
	StatusAbnormal   = "ABNORMAL"
	StatusNormal     = "NORMAL"
)

// GetResponseCode 응답 코드 생성
func (c *Command) GetResponseCode() string {
	switch c.Status {
	case StatusSuccess:
		return c.CommandType + ":S"
	case StatusFailure:
		return c.CommandType + ":F"
	case StatusAbnormal:
		return c.CommandType + ":A"
	case StatusNormal:
		return c.CommandType + ":N"
	default:
		return c.CommandType + ":F"
	}
}

// IsValidCommand 유효한 명령인지 확인
func IsValidCommand(cmd string) bool {
	validCommands := []string{
		CommandCataractRemoval,
		CommandGlaucomaRemoval,
		CommandGripperCleaning,
		CommandCameraCheck,
		CommandCameraCleaning,
		CommandKnifeCleaning,
	}

	for _, validCmd := range validCommands {
		if cmd == validCmd {
			return true
		}
	}
	return false
}
