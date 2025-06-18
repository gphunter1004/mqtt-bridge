package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"topic-data-converter/config"
	"topic-data-converter/converter"
	"topic-data-converter/topic"
	"topic-data-converter/utils"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := utils.NewLogger(cfg.LogLevel, cfg.LogFile)
	logger.Infof("Starting %s v%s", cfg.AppName, cfg.AppVersion)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize converter
	conv := converter.NewConverter(cfg, logger)

	// Initialize topic subscriber and publisher
	subscriber := topic.NewSubscriber(cfg, logger)
	publisher := topic.NewPublisher(cfg, logger)

	// Set up message handler
	subscriber.SetMessageHandler(func(topic string, payload []byte) {
		// Log received message
		logger.Infof("📥 RECEIVED - Topic: %s", topic)
		logger.Infof("📥 RECEIVED - Message: %s", string(payload))
		logger.Debugf("📥 RECEIVED - Raw bytes: %v", payload)

		// Convert data using converter
		convertedData, targetTopic, err := conv.Convert(topic, payload)
		if err != nil {
			logger.Errorf("❌ CONVERSION FAILED - Topic: %s, Error: %v", topic, err)
			logger.Errorf("❌ CONVERSION FAILED - Original message: %s", string(payload))
			return
		}

		// Log converted message before publishing
		logger.Infof("📤 SENDING - Topic: %s", targetTopic)
		logger.Infof("📤 SENDING - Message: %s", string(convertedData))

		// Publish converted data
		if err := publisher.Publish(targetTopic, convertedData); err != nil {
			logger.Errorf("❌ PUBLISH FAILED - Topic: %s, Error: %v", targetTopic, err)
			logger.Errorf("❌ PUBLISH FAILED - Message: %s", string(convertedData))
		} else {
			logger.Infof("✅ CONVERSION SUCCESS - %s → %s", topic, targetTopic)
			logger.Debugf("✅ CONVERSION SUCCESS - Original: %s → Converted: %s", string(payload), string(convertedData))
		}
	})

	// Start subscriber
	if err := subscriber.Start(ctx); err != nil {
		logger.Fatalf("Failed to start subscriber: %v", err)
	}

	// Start publisher
	if err := publisher.Start(ctx); err != nil {
		logger.Fatalf("Failed to start publisher: %v", err)
	}

	logger.Info("Data converter started successfully")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	logger.Info("Shutdown completed")
}
