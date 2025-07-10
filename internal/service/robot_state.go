// internal/service/robot_state.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type RobotStateService struct {
	db          *gorm.DB
	redisClient *redis.Client
}

func NewRobotStateService(db *gorm.DB, redisClient *redis.Client) *RobotStateService {
	return &RobotStateService{
		db:          db,
		redisClient: redisClient,
	}
}

// GetLatestRobotState Redis에서 최신 로봇 상태 조회
func (s *RobotStateService) GetLatestRobotState(serialNumber string) (*models.RobotState, error) {
	ctx := context.Background()
	stateKey := fmt.Sprintf("robot_state:%s", serialNumber)

	stateData, err := s.redisClient.Get(ctx, stateKey).Result()
	if err == redis.Nil {
		// Redis에 없으면 DB에서 최신 상태 조회
		return s.GetLatestRobotStateFromDB(serialNumber)
	} else if err != nil {
		utils.Logger.Errorf("Failed to get robot state from Redis: %v", err)
		return s.GetLatestRobotStateFromDB(serialNumber)
	}

	var robotState models.RobotState
	if err := json.Unmarshal([]byte(stateData), &robotState); err != nil {
		utils.Logger.Errorf("Failed to unmarshal robot state from Redis: %v", err)
		return s.GetLatestRobotStateFromDB(serialNumber)
	}

	return &robotState, nil
}

// GetLatestRobotStateFromDB DB에서 최신 로봇 상태 조회
func (s *RobotStateService) GetLatestRobotStateFromDB(serialNumber string) (*models.RobotState, error) {
	var robotState models.RobotState

	err := s.db.Where("serial_number = ?", serialNumber).
		Order("header_id DESC").
		First(&robotState).Error

	if err != nil {
		return nil, err
	}

	return &robotState, nil
}

// GetRobotStateHistory 로봇 상태 히스토리 조회
func (s *RobotStateService) GetRobotStateHistory(serialNumber string, limit int, offset int) ([]models.RobotState, error) {
	var states []models.RobotState

	query := s.db.Where("serial_number = ?", serialNumber).
		Order("header_id DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&states).Error
	return states, err
}

// GetRobotStateByTimeRange 시간 범위로 로봇 상태 조회
func (s *RobotStateService) GetRobotStateByTimeRange(serialNumber string, startTime, endTime time.Time) ([]models.RobotState, error) {
	var states []models.RobotState

	err := s.db.Where("serial_number = ? AND timestamp BETWEEN ? AND ?",
		serialNumber, startTime, endTime).
		Order("timestamp ASC").
		Find(&states).Error

	return states, err
}

// GetAllActiveRobots 활성 상태인 모든 로봇 조회
func (s *RobotStateService) GetAllActiveRobots() ([]models.RobotState, error) {
	var states []models.RobotState

	// 최근 5분 이내에 상태를 보고한 로봇들을 활성 상태로 간주
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	// 각 로봇별로 최신 상태만 조회
	err := s.db.Table("robot_states").
		Select("DISTINCT ON (serial_number) *").
		Where("timestamp > ?", fiveMinutesAgo).
		Order("serial_number, header_id DESC").
		Find(&states).Error

	return states, err
}

// GetRobotBatteryStatus 로봇 배터리 상태 조회
func (s *RobotStateService) GetRobotBatteryStatus(serialNumber string) (*RobotBatteryInfo, error) {
	robotState, err := s.GetLatestRobotState(serialNumber)
	if err != nil {
		return nil, err
	}

	batteryInfo := &RobotBatteryInfo{
		SerialNumber:   robotState.SerialNumber,
		BatteryCharge:  robotState.BatteryCharge,
		BatteryVoltage: robotState.BatteryVoltage,
		BatteryHealth:  robotState.BatteryHealth,
		Charging:       robotState.Charging,
		LastUpdate:     robotState.Timestamp,
	}

	// 배터리 상태 평가
	if batteryInfo.BatteryCharge < 10 {
		batteryInfo.Status = "CRITICAL"
	} else if batteryInfo.BatteryCharge < 20 {
		batteryInfo.Status = "LOW"
	} else if batteryInfo.BatteryCharge < 50 {
		batteryInfo.Status = "MEDIUM"
	} else {
		batteryInfo.Status = "HIGH"
	}

	return batteryInfo, nil
}

// GetRobotPosition 로봇 위치 정보 조회
func (s *RobotStateService) GetRobotPosition(serialNumber string) (*RobotPositionInfo, error) {
	robotState, err := s.GetLatestRobotState(serialNumber)
	if err != nil {
		return nil, err
	}

	positionInfo := &RobotPositionInfo{
		SerialNumber:        robotState.SerialNumber,
		X:                   robotState.PositionX,
		Y:                   robotState.PositionY,
		Theta:               robotState.PositionTheta,
		LocalizationScore:   robotState.LocalizationScore,
		PositionInitialized: robotState.PositionInitialized,
		MapID:               robotState.MapID,
		LastUpdate:          robotState.Timestamp,
	}

	return positionInfo, nil
}

// GetRobotOperationalStatus 로봇 운영 상태 조회
func (s *RobotStateService) GetRobotOperationalStatus(serialNumber string) (*RobotOperationalInfo, error) {
	robotState, err := s.GetLatestRobotState(serialNumber)
	if err != nil {
		return nil, err
	}

	operationalInfo := &RobotOperationalInfo{
		SerialNumber:   robotState.SerialNumber,
		OperatingMode:  robotState.OperatingMode,
		Driving:        robotState.Driving,
		Paused:         robotState.Paused,
		EStop:          robotState.EStop,
		FieldViolation: robotState.FieldViolation,
		OrderID:        robotState.OrderID,
		LastNodeID:     robotState.LastNodeID,
		ErrorCount:     robotState.ErrorCount,
		ActionCount:    robotState.ActionCount,
		LastUpdate:     robotState.Timestamp,
	}

	// 전체 상태 평가
	if operationalInfo.EStop != models.EStopNone {
		operationalInfo.OverallStatus = "E_STOP"
	} else if operationalInfo.ErrorCount > 0 {
		operationalInfo.OverallStatus = "ERROR"
	} else if operationalInfo.Paused {
		operationalInfo.OverallStatus = "PAUSED"
	} else if operationalInfo.Driving {
		operationalInfo.OverallStatus = "DRIVING"
	} else if operationalInfo.OperatingMode == models.OperatingModeAutomatic {
		operationalInfo.OverallStatus = "READY"
	} else {
		operationalInfo.OverallStatus = "MANUAL"
	}

	return operationalInfo, nil
}

// CleanupOldRobotStates 오래된 로봇 상태 데이터 정리
func (s *RobotStateService) CleanupOldRobotStates(olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	result := s.db.Where("timestamp < ?", cutoffTime).Delete(&models.RobotState{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		utils.Logger.Infof("Cleaned up %d old robot state records older than %v",
			result.RowsAffected, olderThan)
	}

	return nil
}

// RobotBatteryInfo 로봇 배터리 정보
type RobotBatteryInfo struct {
	SerialNumber   string    `json:"serial_number"`
	BatteryCharge  float64   `json:"battery_charge"`
	BatteryVoltage float64   `json:"battery_voltage"`
	BatteryHealth  int       `json:"battery_health"`
	Charging       bool      `json:"charging"`
	Status         string    `json:"status"` // CRITICAL, LOW, MEDIUM, HIGH
	LastUpdate     time.Time `json:"last_update"`
}

// RobotPositionInfo 로봇 위치 정보
type RobotPositionInfo struct {
	SerialNumber        string    `json:"serial_number"`
	X                   float64   `json:"x"`
	Y                   float64   `json:"y"`
	Theta               float64   `json:"theta"`
	LocalizationScore   float64   `json:"localization_score"`
	PositionInitialized bool      `json:"position_initialized"`
	MapID               string    `json:"map_id"`
	LastUpdate          time.Time `json:"last_update"`
}

// RobotOperationalInfo 로봇 운영 정보
type RobotOperationalInfo struct {
	SerialNumber   string    `json:"serial_number"`
	OperatingMode  string    `json:"operating_mode"`
	Driving        bool      `json:"driving"`
	Paused         bool      `json:"paused"`
	EStop          string    `json:"e_stop"`
	FieldViolation bool      `json:"field_violation"`
	OrderID        string    `json:"order_id"`
	LastNodeID     string    `json:"last_node_id"`
	ErrorCount     int       `json:"error_count"`
	ActionCount    int       `json:"action_count"`
	OverallStatus  string    `json:"overall_status"` // E_STOP, ERROR, PAUSED, DRIVING, READY, MANUAL
	LastUpdate     time.Time `json:"last_update"`
}
