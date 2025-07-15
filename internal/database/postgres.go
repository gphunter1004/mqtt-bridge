// internal/database/postgres.go (개선된 버전 - 최소한의 데이터만 로딩)
package database

import (
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgresDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

	// 로그 레벨을 환경에 따라 조정
	logLevel := logger.Silent // 기본값은 Silent
	if cfg.LogLevel == "debug" {
		logLevel = logger.Info // 디버그 모드에서만 SQL 로그 출력
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
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

// createSampleData 샘플 데이터 생성
func createSampleData(db *gorm.DB) error {
	utils.Logger.Info("🔧 Setting up minimal database data...")

	// 1. 모든 기본 명령 정의 생성
	commandDefs := []models.CommandDefinition{
		{CommandType: "CR", Description: "백내장 적출", IsActive: true},
		{CommandType: "GR", Description: "적내장 적출", IsActive: true},
		{CommandType: "GC", Description: "그리퍼 세정", IsActive: true},
		{CommandType: "CC", Description: "카메라 확인", IsActive: true},
		{CommandType: "CL", Description: "카메라 세정", IsActive: true},
		{CommandType: "KC", Description: "나이프 세정", IsActive: true},
		{CommandType: constants.CommandOrderCancel, Description: "명령 취소", IsActive: true},
	}

	for _, def := range commandDefs {
		var existing models.CommandDefinition
		result := db.Where("command_type = ?", def.CommandType).First(&existing)
		if result.Error != nil {
			// 존재하지 않으면 생성
			if err := db.Create(&def).Error; err != nil {
				return fmt.Errorf("failed to create command definition %s: %w", def.CommandType, err)
			}
			utils.Logger.Infof("✅ Command definition created: %s", def.CommandType)
		}
	}

	// 2. 기본 노드 템플릿 생성
	var defaultNode models.NodeTemplate
	result := db.Where("name = ?", "Default Origin").First(&defaultNode)
	if result.Error != nil {
		defaultNode = models.NodeTemplate{
			Name:                  "Default Origin",
			Description:           "기본 원점 노드",
			X:                     0.0,
			Y:                     0.0,
			Theta:                 0.0,
			AllowedDeviationXY:    0.0,
			AllowedDeviationTheta: 0.0,
			MapID:                 "",
		}
		if err := db.Create(&defaultNode).Error; err != nil {
			return fmt.Errorf("failed to create default node template: %w", err)
		}
		utils.Logger.Info("✅ Default node template created")
	}

	// 3. CR 명령용 최소 샘플 데이터 (선택적 - 개발 편의를 위해)
	if shouldCreateSampleWorkflow(db) {
		if err := createCRWorkflowSample(db); err != nil {
			utils.Logger.Warnf("Failed to create CR workflow sample: %v", err)
			// 샘플 데이터 생성 실패는 치명적이지 않음
		}
	}

	utils.Logger.Info("✅ Minimal database setup completed")
	return nil
}

// shouldCreateSampleWorkflow 샘플 워크플로우를 생성할지 결정
func shouldCreateSampleWorkflow(db *gorm.DB) bool {
	var count int64
	db.Model(&models.OrderTemplate{}).Count(&count)
	return count == 0 // OrderTemplate이 없으면 샘플 생성
}

// createCRWorkflowSample CR 명령용 최소 샘플 워크플로우 생성 (2단계: 직장파지 → 직장근막절개)
func createCRWorkflowSample(db *gorm.DB) error {
	utils.Logger.Info("🔧 Creating CR workflow sample (직장파지 → 직장근막절개)...")

	// 1. 액션 템플릿 생성
	// 1-1. "직장 파지" 액션 템플릿
	phacoAction := models.ActionTemplate{
		ActionType:        constants.ActionTypeTrajectory,
		ActionDescription: "직장 파지",
		BlockingType:      constants.BlockingTypeNone,
	}
	db.FirstOrCreate(&phacoAction, models.ActionTemplate{
		ActionType:        phacoAction.ActionType,
		ActionDescription: phacoAction.ActionDescription,
	})

	// "직장 파지" 액션 파라미터 (trajectory_name: RG)
	phacoParams := []models.ActionParameter{
		{ActionTemplateID: phacoAction.ID, Key: "arm", Value: constants.ArmRight, ValueType: "STRING"},
		{ActionTemplateID: phacoAction.ID, Key: "trajectory_name", Value: "RG", ValueType: "STRING"},
	}
	for _, param := range phacoParams {
		db.FirstOrCreate(&param, models.ActionParameter{
			ActionTemplateID: param.ActionTemplateID,
			Key:              param.Key,
		})
	}

	// 1-2. "직장 근막 절개" 액션 템플릿
	iolAction := models.ActionTemplate{
		ActionType:        constants.ActionTypeTrajectory,
		ActionDescription: "직장 근막 절개",
		BlockingType:      constants.BlockingTypeNone,
	}
	db.FirstOrCreate(&iolAction, models.ActionTemplate{
		ActionType:        iolAction.ActionType,
		ActionDescription: iolAction.ActionDescription,
	})

	// "직장 근막 절개" 액션 파라미터 (trajectory_name: FI)
	iolParams := []models.ActionParameter{
		{ActionTemplateID: iolAction.ID, Key: "arm", Value: constants.ArmRight, ValueType: "STRING"},
		{ActionTemplateID: iolAction.ID, Key: "trajectory_name", Value: "FI", ValueType: "STRING"},
	}
	for _, param := range iolParams {
		db.FirstOrCreate(&param, models.ActionParameter{
			ActionTemplateID: param.ActionTemplateID,
			Key:              param.Key,
		})
	}

	// 2. 오더 템플릿 생성
	var phacoOrderTpl, iolOrderTpl models.OrderTemplate
	db.FirstOrCreate(&phacoOrderTpl, models.OrderTemplate{Name: "직장 파지"})
	db.FirstOrCreate(&iolOrderTpl, models.OrderTemplate{Name: "직장 근막 절개"})

	// 3. 각 오더 템플릿의 스텝 생성
	var phacoStep, iolStep models.OrderStep
	db.FirstOrCreate(&phacoStep, models.OrderStep{
		TemplateID:         phacoOrderTpl.ID,
		StepOrder:          1,
		WaitForCompletion:  true,
		PreviousStepResult: constants.PreviousResultAny,
	})
	db.FirstOrCreate(&iolStep, models.OrderStep{
		TemplateID:         iolOrderTpl.ID,
		StepOrder:          1,
		WaitForCompletion:  true,
		PreviousStepResult: constants.PreviousResultAny,
	})

	// 4. 스텝-액션 매핑 생성
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

	// 5. CR 명령 매핑 생성 (2단계 순차 실행)
	var crDef models.CommandDefinition
	db.Where("command_type = ?", "CR").First(&crDef)

	// 기존 매핑 삭제 후 재생성
	db.Unscoped().Where("command_definition_id = ?", crDef.ID).Delete(&models.CommandOrderMapping{})

	// ExecutionOrder 1: "직장 파지" 실행 → 성공 시 2번으로 이동
	crMappings := []models.CommandOrderMapping{
		{
			CommandDefinitionID: crDef.ID,
			TemplateID:          phacoOrderTpl.ID,
			ExecutionOrder:      1,
			NextExecutionOrder:  2, // 성공하면 2번 오더로
			FailureOrder:        0, // 실패하면 종료
			IsActive:            true,
		},
		// ExecutionOrder 2: "직장 근막 절개" 실행 → 성공/실패 모두 종료
		{
			CommandDefinitionID: crDef.ID,
			TemplateID:          iolOrderTpl.ID,
			ExecutionOrder:      2,
			NextExecutionOrder:  0, // 성공 시 종료
			FailureOrder:        0, // 실패 시 종료
			IsActive:            true,
		},
	}

	if err := db.Create(&crMappings).Error; err != nil {
		return fmt.Errorf("failed to create CR command mappings: %w", err)
	}

	utils.Logger.Info("✅ CR workflow sample created:")
	utils.Logger.Info("   Step 1: 직장 파지 (trajectory_name: RG)")
	utils.Logger.Info("   Step 2: 직장 근막 절개 (trajectory_name: FI)")
	return nil
}
