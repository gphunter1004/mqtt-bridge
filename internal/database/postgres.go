// internal/database/postgres.go (다중 오더 지원 업데이트)
package database

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgresDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// 테이블 마이그레이션 (다중 오더 시스템 포함)
	if err := db.AutoMigrate(
		// 기존 테이블
		&models.Command{},        // PLC 명령 정보
		&models.RobotStatus{},    // 로봇 연결 상태 정보
		&models.RobotFactsheet{}, // 로봇 팩트시트 정보
		&models.RobotState{},     // 로봇 운영 상태 정보

		// 다중 오더 시스템 테이블
		&models.OrderTemplate{},       // 오더 템플릿
		&models.CommandOrderMapping{}, // 명령-오더 매핑 (1:N)
		&models.OrderStep{},           // 오더 단계
		&models.NodeTemplate{},        // 노드 템플릿
		&models.ActionTemplate{},      // 액션 템플릿
		&models.ActionParameter{},     // 액션 파라미터
		&models.EdgeTemplate{},        // 엣지 템플릿
		&models.CommandExecution{},    // 명령 실행 (여러 오더 포함)
		&models.OrderExecution{},      // 개별 오더 실행
		&models.StepExecution{},       // 단계 실행
	); err != nil {
		return nil, err
	}

	// 샘플 데이터 생성 (개발용)
	if err := createMultiOrderSampleData(db); err != nil {
		return nil, err
	}

	return db, nil
}

// createMultiOrderSampleData 다중 오더 지원 샘플 데이터 생성
func createMultiOrderSampleData(db *gorm.DB) error {
	// 기본 노드 템플릿 생성
	defaultNode := &models.NodeTemplate{
		Name:        "Default Origin",
		Description: "Default origin position",
		X:           0.0,
		Y:           0.0,
		Theta:       0.0,
	}

	var existingNode models.NodeTemplate
	if err := db.Where("name = ?", defaultNode.Name).First(&existingNode).Error; err == gorm.ErrRecordNotFound {
		if err := db.Create(defaultNode).Error; err != nil {
			return err
		}
	} else if err == nil {
		defaultNode = &existingNode
	}

	return nil
}
