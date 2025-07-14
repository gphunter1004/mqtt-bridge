// cmd/main.go (새로운 구조 적용)
package main

import (
	"context"
	"mqtt-bridge/internal/bridge"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/database"
	"mqtt-bridge/internal/redis"
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
	utils.Logger.Infof("🚀 Starting MQTT Bridge with new architecture")

	// 데이터베이스 연결
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		utils.Logger.Fatalf("Failed to connect to database: %v", err)
	}
	utils.Logger.Infof("✅ Database connected")

	// Redis 연결
	redisClient, err := redis.NewRedisClient(cfg)
	if err != nil {
		utils.Logger.Fatalf("Failed to connect to Redis: %v", err)
	}
	utils.Logger.Infof("✅ Redis connected")

	// 브릿지 서비스 생성 (모든 도메인과 인프라 조합)
	bridgeService, err := bridge.NewService(db, redisClient, cfg)
	if err != nil {
		utils.Logger.Fatalf("Failed to create bridge service: %v", err)
	}
	utils.Logger.Infof("✅ Bridge service created")

	// 브릿지 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	if err := bridgeService.Start(ctx); err != nil {
		utils.Logger.Fatalf("Failed to start bridge service: %v", err)
	}

	// 우아한 종료 처리
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	utils.Logger.Info("🎉 MQTT Bridge started successfully with new architecture")
	utils.Logger.Info("📋 Available domains:")
	utils.Logger.Info("  - Command Domain: PLC 명령 처리")
	utils.Logger.Info("  - Robot Domain: 로봇 상태 관리")
	utils.Logger.Info("  - Workflow Domain: 워크플로우 실행")
	utils.Logger.Info("  - Position Domain: 위치 관리")
	utils.Logger.Info("  - Messaging Infrastructure: MQTT 통신")

	<-sigChan
	utils.Logger.Info("🛑 Shutting down...")

	// 컨텍스트 취소
	cancel()

	// 브릿지 서비스 정리
	bridgeService.Stop()

	utils.Logger.Info("✅ Shutdown complete")
}
