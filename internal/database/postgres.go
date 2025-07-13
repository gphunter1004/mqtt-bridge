package database

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgresDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	// 간소화된 테이블 마이그레이션
	if err := db.AutoMigrate(
		&models.Command{},
		&models.RobotStatus{},
		&models.ActionHistory{},
		&models.PLCStatusHistory{},
	); err != nil {
		return nil, err
	}

	// 기본 데이터 생성
	if err := createDefaultData(db); err != nil {
		return nil, err
	}

	return db, nil
}

// createDefaultData 기본 데이터 생성
func createDefaultData(db *gorm.DB) error {
	// 기본 로봇 상태 생성
	var robotCount int64
	db.Model(&models.RobotStatus{}).Count(&robotCount)

	if robotCount == 0 {
		defaultRobot := &models.RobotStatus{
			SerialNumber:    "DEX0002",
			Manufacturer:    "Roboligent",
			ConnectionState: models.ConnectionStateOffline,
			IsBusy:          false,
			OperationalData: models.JSON{
				"current": map[string]interface{}{
					"position": map[string]float64{"x": 0, "y": 0, "theta": 0},
					"battery":  map[string]interface{}{"charge": 100, "voltage": 40.0},
					"mode":     "MANUAL",
				},
				"history": []interface{}{},
			},
			FactsheetData: models.JSON{
				"version": "1.0",
				"series": map[string]string{
					"name":        "Robin",
					"description": "Humanoid robot with dual manipulators",
				},
				"specifications": map[string]interface{}{
					"agv_class":      "AMR",
					"agv_kinematics": "ForwardKinematics",
					"max_load_mass":  50,
					"speed":          map[string]float64{"min": 0.0, "max": 0.4},
				},
			},
			LastHeaderID: 0,
		}

		if err := db.Create(defaultRobot).Error; err != nil {
			return fmt.Errorf("failed to create default robot status: %w", err)
		}
	}

	// 기본 워크플로우 설정 생성 (파일이나 환경변수로 관리하는 것이 좋음)
	return createDefaultWorkflows(db)
}

// createDefaultWorkflows 기본 워크플로우 설정
func createDefaultWorkflows(db *gorm.DB) error {
	// 실제로는 설정 파일이나 환경변수에서 로드하는 것이 좋음
	// 여기서는 예시로만 작성

	// CR (백내장 적출) 워크플로우 예시
	crWorkflow := models.JSON{
		"workflow_id": "cr_workflow_v2",
		"version":     "2.0",
		"steps": []interface{}{
			map[string]interface{}{
				"order":   1,
				"name":    "직장 파지",
				"timeout": 300,
				"node": map[string]interface{}{
					"node_id":                 "node_gripping_point",
					"position":                map[string]float64{"x": 1.5, "y": 2.3, "theta": 0.5},
					"allowed_deviation_xy":    0.1,
					"allowed_deviation_theta": 0.05,
					"map_id":                  "surgical_room_1",
				},
				"actions": []interface{}{
					map[string]interface{}{
						"action_id":     "grip_rectum_action",
						"type":          "Roboligent Robin - Follow Trajectory",
						"description":   "직장 파지 동작",
						"blocking_type": "HARD",
						"params": map[string]interface{}{
							"arm":             "right",
							"trajectory_name": "trajectory_grip_rectum",
						},
					},
				},
				"on_success": "next",
				"on_failure": "abort",
			},
			map[string]interface{}{
				"order":   2,
				"name":    "직장 근막 절개",
				"timeout": 300,
				"node": map[string]interface{}{
					"node_id":  "node_cutting_point",
					"position": map[string]float64{"x": 1.6, "y": 2.4, "theta": 0.5},
				},
				"actions": []interface{}{
					map[string]interface{}{
						"action_id":   "cut_fascia_action",
						"type":        "Roboligent Robin - Follow Trajectory",
						"description": "근막 절개 동작",
						"params": map[string]interface{}{
							"arm":             "right",
							"trajectory_name": "trajectory_cut_fascia",
						},
					},
				},
				"on_success": "complete",
				"on_failure": "abort",
			},
		},
	}

	// GC (그리퍼 세정) 워크플로우 예시
	gcWorkflow := models.JSON{
		"workflow_id": "gc_workflow_v1",
		"version":     "1.0",
		"steps": []interface{}{
			map[string]interface{}{
				"order":   1,
				"name":    "그리퍼 세정",
				"timeout": 180,
				"node": map[string]interface{}{
					"node_id":  "node_cleaning_station",
					"position": map[string]float64{"x": 0.0, "y": 0.0, "theta": 0.0},
				},
				"actions": []interface{}{
					map[string]interface{}{
						"action_id":   "gripper_clean_action",
						"type":        "Roboligent Robin - Gripper Clean",
						"description": "그리퍼 세정 동작",
						"params": map[string]interface{}{
							"duration":  60,
							"intensity": "high",
						},
					},
				},
				"on_success": "complete",
				"on_failure": "retry",
			},
		},
	}

	// 워크플로우를 메모리나 Redis에 저장하는 것이 좋음
	// 여기서는 로그로만 출력
	fmt.Printf("Default workflows loaded: CR, GC\n")
	_ = crWorkflow
	_ = gcWorkflow

	return nil
}

// CleanupOldData 오래된 데이터 정리 (선택적)
func CleanupOldData(db *gorm.DB, days int) error {
	// 오래된 명령 삭제
	if err := db.Where("created_at < NOW() - INTERVAL ? DAY", days).
		Delete(&models.Command{}).Error; err != nil {
		return fmt.Errorf("failed to cleanup old commands: %w", err)
	}

	// 오래된 액션 이력 삭제
	if err := db.Where("created_at < NOW() - INTERVAL ? DAY", days).
		Delete(&models.ActionHistory{}).Error; err != nil {
		return fmt.Errorf("failed to cleanup old action history: %w", err)
	}

	// 오래된 PLC 상태 이력 삭제
	if err := db.Where("created_at < NOW() - INTERVAL ? DAY", days/2).
		Delete(&models.PLCStatusHistory{}).Error; err != nil {
		return fmt.Errorf("failed to cleanup old PLC status history: %w", err)
	}

	return nil
}

// GetWorkflowConfig 워크플로우 설정 가져오기
func GetWorkflowConfig(commandType string) models.JSON {
	// 실제로는 파일, DB, 또는 설정 서버에서 가져와야 함
	workflows := map[string]models.JSON{
		"CR": {
			"workflow_id": "cr_workflow_v2",
			"version":     "2.0",
			"steps":       []interface{}{
				// ... CR 워크플로우 내용
			},
		},
		"GC": {
			"workflow_id": "gc_workflow_v1",
			"version":     "1.0",
			"steps":       []interface{}{
				// ... GC 워크플로우 내용
			},
		},
		// 다른 명령 타입들...
	}

	if workflow, exists := workflows[commandType]; exists {
		return workflow
	}

	// 기본 워크플로우
	return models.JSON{
		"workflow_id": "default",
		"version":     "1.0",
		"steps":       []interface{}{},
	}
}
