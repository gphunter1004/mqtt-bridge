// cmd/main.go - DI 패턴 적용된 최종 메인 파일
package main

import (
	"context"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/di"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 설정 로드
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	// DI 컨테이너 생성
	container, err := di.NewContainer(cfg)
	if err != nil {
		panic("Failed to create DI container: " + err.Error())
	}
	defer container.Cleanup()

	// 브릿지 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := container.BridgeService.Start(ctx); err != nil {
		container.Logger.Fatalf("Failed to start bridge service: %v", err)
	}

	// 시작 완료 로그
	container.Logger.Infof("🎯 MQTT Bridge with DI pattern started successfully")
	container.Logger.Infof("📊 Services initialized:")
	container.Logger.Infof("   ✅ Database Service")
	container.Logger.Infof("   ✅ Cache Service")
	container.Logger.Infof("   ✅ Message Publisher")
	container.Logger.Infof("   ✅ Order Builder")
	container.Logger.Infof("   ✅ Command Handler")
	container.Logger.Infof("   ✅ Robot Handler")
	container.Logger.Infof("   ✅ Order Executor")

	// 우아한 종료 처리
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 종료 신호 대기
	<-sigChan

	container.Logger.Infof("🛑 Shutdown signal received")
	cancel()

	container.Logger.Infof("✅ MQTT Bridge shutdown completed")
}
