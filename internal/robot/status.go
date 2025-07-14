// internal/robot/status.go
package robot

import (
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
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
	return robotStatus.ConnectionState == models.ConnectionStateOnline
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

// GetAllRobotStatuses 모든 로봇 상태 조회
func (s *StatusManager) GetAllRobotStatuses() ([]models.RobotStatus, error) {
	var statuses []models.RobotStatus
	err := s.db.Find(&statuses).Error
	return statuses, err
}

// IsValidConnectionState 유효한 연결 상태인지 확인
func (s *StatusManager) IsValidConnectionState(state string) bool {
	return models.IsValidConnectionState(state)
}

// GetOnlineRobots 온라인 상태인 로봇들 조회
func (s *StatusManager) GetOnlineRobots() ([]models.RobotStatus, error) {
	var statuses []models.RobotStatus
	err := s.db.Where("connection_state = ?", models.ConnectionStateOnline).Find(&statuses).Error
	return statuses, err
}

// GetOfflineRobots 오프라인 상태인 로봇들 조회
func (s *StatusManager) GetOfflineRobots() ([]models.RobotStatus, error) {
	var statuses []models.RobotStatus
	err := s.db.Where("connection_state IN ?",
		[]string{models.ConnectionStateOffline, models.ConnectionStateConnectionBroken}).Find(&statuses).Error
	return statuses, err
}

// UpdateLastSeen 마지막 접속 시간 업데이트
func (s *StatusManager) UpdateLastSeen(serialNumber string) error {
	return s.db.Model(&models.RobotStatus{}).
		Where("serial_number = ?", serialNumber).
		Update("last_timestamp", time.Now()).Error
}

// CleanupStaleConnections 오래된 연결 정리 (일정 시간 이상 응답 없는 로봇들을 오프라인으로 처리)
func (s *StatusManager) CleanupStaleConnections(timeout time.Duration) error {
	cutoffTime := time.Now().Add(-timeout)

	utils.Logger.Infof("Cleaning up stale connections older than %v", cutoffTime)

	result := s.db.Model(&models.RobotStatus{}).
		Where("connection_state = ? AND last_timestamp < ?", models.ConnectionStateOnline, cutoffTime).
		Update("connection_state", models.ConnectionStateConnectionBroken)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		utils.Logger.Warnf("Marked %d robots as connection broken due to stale connections", result.RowsAffected)
	}

	return nil
}
