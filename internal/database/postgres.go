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

	// 테이블 마이그레이션 (Command, RobotStatus, RobotFactsheet 테이블)
	if err := db.AutoMigrate(&models.Command{}, &models.RobotStatus{}, &models.RobotFactsheet{}); err != nil {
		return nil, err
	}

	return db, nil
}
