// internal/database/postgres.go (최종 수정본)
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
		Logger: logger.Default.LogMode(logger.Info), // 개발 중 SQL 로그 확인용
	})
	if err != nil {
		return nil, err
	}

	// 테이블 마이그레이션
	if err := db.AutoMigrate(
		&models.CommandDefinition{},
		&models.Command{},
		&models.CommandOrderMapping{},
		&models.RobotStatus{},
		&models.RobotFactsheet{},
		&models.OrderTemplate{},
		&models.OrderStep{},
		&models.NodeTemplate{},
		&models.ActionTemplate{},
		&models.ActionParameter{},
		&models.StepActionMapping{},
		&models.EdgeTemplate{},
		&models.CommandExecution{},
		&models.OrderExecution{},
		&models.StepExecution{},
	); err != nil {
		return nil, err
	}

	// 샘플 데이터 생성
	if err := createSampleData(db); err != nil {
		return nil, err
	}

	return db, nil
}

// createOrUpdateAction 헬퍼 함수: 액션 템플릿과 파라미터를 안전하게 생성
func createOrUpdateAction(db *gorm.DB, actionInfo models.ActionTemplate, params []models.ActionParameter) (models.ActionTemplate, error) {
	// ActionType과 ActionDescription이 모두 일치하는 경우를 조건으로 조회 또는 생성
	if err := db.Where(models.ActionTemplate{
		ActionType:        actionInfo.ActionType,
		ActionDescription: actionInfo.ActionDescription,
	}).FirstOrCreate(&actionInfo).Error; err != nil {
		return actionInfo, err
	}

	// 해당 액션 템플릿에 파라미터 생성
	for _, p := range params {
		// ActionTemplateID와 Key가 모두 일치하는 경우를 조건으로 조회 또는 생성
		p.ActionTemplateID = actionInfo.ID
		db.FirstOrCreate(&p, models.ActionParameter{
			ActionTemplateID: p.ActionTemplateID,
			Key:              p.Key,
		})
	}
	return actionInfo, nil
}

// createSampleData 샘플 데이터 생성
func createSampleData(db *gorm.DB) error {
	// 1. 모든 기본 명령 정의 생성
	cmdDefs := []models.CommandDefinition{
		{CommandType: "CR", Description: "백내장 적출", IsActive: true},
		{CommandType: "GR", Description: "적내장 적출", IsActive: true},
		{CommandType: "GC", Description: "그리퍼 세정", IsActive: true},
		{CommandType: "CC", Description: "카메라 확인", IsActive: true},
		{CommandType: "CL", Description: "카메라 세정", IsActive: true},
		{CommandType: "KC", Description: "나이프 세정", IsActive: true},
		{CommandType: "OC", Description: "명령 취소", IsActive: true},
	}

	var cataractDef models.CommandDefinition
	for _, def := range cmdDefs {
		db.FirstOrCreate(&def, models.CommandDefinition{CommandType: def.CommandType})
		if def.CommandType == "CR" {
			cataractDef = def
		}
	}

	// 2. 기본 노드 템플릿 생성
	db.FirstOrCreate(&models.NodeTemplate{}, &models.NodeTemplate{
		Name: "Default Origin",
	})

	// 3. "백내장 적출"에 필요한 액션 템플릿 및 파라미터 생성
	// 3-1. "수정체 유화술" 액션
	phacoAction, err := createOrUpdateAction(db,
		models.ActionTemplate{ActionType: "Roboligent Robin - Follow Trajectory", ActionDescription: "직장 파지"},
		[]models.ActionParameter{
			{Key: "arm", Value: "right", ValueType: "STRING"},
			{Key: "trajectory_name", Value: "trajectory_1", ValueType: "STRING"},
		},
	)
	if err != nil {
		return err
	}

	// 3-2. "인공수정체 삽입" 액션
	iolAction, err := createOrUpdateAction(db,
		models.ActionTemplate{ActionType: "Roboligent Robin - Follow Trajectory", ActionDescription: "직장 근막 절개"},
		[]models.ActionParameter{
			{Key: "arm", Value: "right", ValueType: "STRING"},
			{Key: "trajectory_name", Value: "trajectory_2", ValueType: "STRING"},
		},
	)
	if err != nil {
		return err
	}

	// 4. "백내장 적출"을 위한 오더 템플릿 생성
	var phacoOrderTpl, iolOrderTpl models.OrderTemplate
	db.FirstOrCreate(&phacoOrderTpl, models.OrderTemplate{Name: "직장 파지"})
	db.FirstOrCreate(&iolOrderTpl, models.OrderTemplate{Name: "직장 근막 절개"})

	// 5. 각 오더 템플릿에 대한 단계(Step) 생성
	var phacoStep, iolStep models.OrderStep
	db.FirstOrCreate(&phacoStep, models.OrderStep{TemplateID: phacoOrderTpl.ID, StepOrder: 1, WaitForCompletion: true})
	db.FirstOrCreate(&iolStep, models.OrderStep{TemplateID: iolOrderTpl.ID, StepOrder: 1, WaitForCompletion: true})

	// 6. 각 단계와 액션을 매핑
	db.FirstOrCreate(&models.StepActionMapping{}, &models.StepActionMapping{
		OrderStepID:      phacoStep.ID,
		ActionTemplateID: phacoAction.ID,
		ExecutionOrder:   1,
	})
	db.FirstOrCreate(&models.StepActionMapping{}, &models.StepActionMapping{
		OrderStepID:      iolStep.ID,
		ActionTemplateID: iolAction.ID,
		ExecutionOrder:   1,
	})

	// 7. "CR" 명령에 대한 워크플로우(오더 순서) 정의
	// 기존 매핑 데이터 삭제 (중복 방지)
	db.Unscoped().Where("command_definition_id = ?", cataractDef.ID).Delete(&models.CommandOrderMapping{})

	cataractMappings := []models.CommandOrderMapping{
		// ExecutionOrder 1: "CR" 명령 시 가장 먼저 "직장 파지" 실행. 성공 시 2번 오더로 이동.
		{
			CommandDefinitionID: cataractDef.ID,
			TemplateID:          phacoOrderTpl.ID,
			ExecutionOrder:      1,
			NextExecutionOrder:  2, // 성공하면 다음 순번인 2로 간다.
			FailureOrder:        0, // 실패하면 워크플로우 종료.
			IsActive:            true,
		},
		// ExecutionOrder 2: 1번 오더 성공 후 "직장 근막 절개" 실행. 성공/실패 모두 워크플로우 종료.
		{
			CommandDefinitionID: cataractDef.ID,
			TemplateID:          iolOrderTpl.ID,
			ExecutionOrder:      2,
			NextExecutionOrder:  0, // 성공 시 종료.
			FailureOrder:        0, // 실패 시 종료.
			IsActive:            true,
		},
	}

	if err := db.Create(&cataractMappings).Error; err != nil {
		return fmt.Errorf("failed to create sample 'CR' command mappings: %w", err)
	}

	return nil
}
