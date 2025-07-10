package main

import (
	"context"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/database"
	"mqtt-bridge/internal/mqtt"
	"mqtt-bridge/internal/redis"
	"mqtt-bridge/internal/service"
	"mqtt-bridge/internal/utils"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 설정 로드
	cfg, err := config.Load()
	if err != nil {
		utils.Logger.Fatalf("Failed to load config: %v", err)
	}

	// 로거 설정
	utils.SetupLogger(cfg.LogLevel)

	// 데이터베이스 연결
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		utils.Logger.Fatalf("Failed to connect to database: %v", err)
	}

	// Redis 연결
	redisClient, err := redis.NewRedisClient(cfg)
	if err != nil {
		utils.Logger.Fatalf("Failed to connect to Redis: %v", err)
	}

	// MQTT 클라이언트 생성
	mqttClient, err := mqtt.NewClient(cfg)
	if err != nil {
		utils.Logger.Fatalf("Failed to create MQTT client: %v", err)
	}

	// 브릿지 서비스 생성
	bridgeService := service.NewBridgeService(db, redisClient, mqttClient, cfg)

	// 브릿지 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	if err := bridgeService.Start(ctx); err != nil {
		utils.Logger.Fatalf("Failed to start bridge service: %v", err)
	}

	// 우아한 종료 처리
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	utils.Logger.Info("MQTT Bridge started successfully")
	<-sigChan

	utils.Logger.Info("Shutting down...")
	cancel()

	// 연결 종료
	mqttClient.Disconnect(250)
	redisClient.Close()

	utils.Logger.Info("Shutdown complete")
}
