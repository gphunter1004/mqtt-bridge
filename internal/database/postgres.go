// internal/database/postgres.go
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

	// 테이블 마이그레이션 (핵심 테이블만)
	if err := db.AutoMigrate(
		&models.Command{},        // PLC 명령 정보
		&models.RobotStatus{},    // 로봇 연결 상태 정보
		&models.RobotFactsheet{}, // 로봇 팩트시트 정보
		&models.RobotState{},     // 로봇 운영 상태 정보
	); err != nil {
		return nil, err
	}

	return db, nil
}
