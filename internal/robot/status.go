package robot

import (
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/models"
	"time"

	"gorm.io/gorm"
)

// StatusManager 로봇 상태 관리
type StatusManager struct {
	db *gorm.DB
}

// NewStatusManager 새 상태 관리자 생성
func NewStatusManager(db *gorm.DB) *StatusManager {
	return &StatusManager{
		db: db,
	}
}

// IsOnline 로봇이 온라인 상태인지 확인
func (s *StatusManager) IsOnline(serialNumber string) bool {
	var robotStatus models.RobotStatus
	err := s.db.Where("serial_number = ?", serialNumber).First(&robotStatus).Error
	if err != nil {
		return false
	}
	return robotStatus.ConnectionState == constants.ConnectionStateOnline
}

// UpdateConnectionState 연결 상태 업데이트
func (s *StatusManager) UpdateConnectionState(connMsg *models.ConnectionStateMessage, timestamp time.Time) error {
	var existingStatus models.RobotStatus
	result := s.db.Where("serial_number = ?", connMsg.SerialNumber).First(&existingStatus)

	if result.Error == gorm.ErrRecordNotFound {
		// 새로 생성
		robotStatus := &models.RobotStatus{
			Manufacturer:    connMsg.Manufacturer,
			SerialNumber:    connMsg.SerialNumber,
			ConnectionState: connMsg.ConnectionState,
			LastHeaderID:    connMsg.HeaderID,
			LastTimestamp:   timestamp,
			Version:         connMsg.Version,
		}
		return s.db.Create(robotStatus).Error
	} else if result.Error == nil {
		// 기존 업데이트
		existingStatus.ConnectionState = connMsg.ConnectionState
		existingStatus.LastHeaderID = connMsg.HeaderID
		existingStatus.LastTimestamp = timestamp
		existingStatus.Version = connMsg.Version
		return s.db.Save(&existingStatus).Error
	}

	return result.Error
}

// GetRobotStatus 로봇 상태 조회
func (s *StatusManager) GetRobotStatus(serialNumber string) (*models.RobotStatus, error) {
	var status models.RobotStatus
	err := s.db.Where("serial_number = ?", serialNumber).First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// UpdateLastSeen 마지막 접속 시간 업데이트
func (s *StatusManager) UpdateLastSeen(serialNumber string) error {
	return s.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", serialNumber).
		Update("last_timestamp", time.Now()).Error
}
