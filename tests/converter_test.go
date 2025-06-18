package tests

import (
	"encoding/json"
	"testing"
	"topic-data-converter/config"
	"topic-data-converter/converter"
	"topic-data-converter/models"
	"topic-data-converter/utils"
)

func TestConverter(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		PLCTopicPrefix:   "plc/",
		RobotTopicPrefix: "robot/",
	}

	// Create test logger
	logger := utils.NewLogger("debug", "")

	// Create converter
	conv := converter.NewConverter(cfg, logger)

	t.Run("Test Status Conversion", func(t *testing.T) {
		topic := "plc/status"
		payload := []byte("device1:running:100,200,300")

		convertedData, targetTopic, err := conv.Convert(topic, payload)
		if err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		if targetTopic != "robot/status" {
			t.Errorf("Expected target topic 'robot/status', got '%s'", targetTopic)
		}

		// Parse the converted JSON
		var robotData models.RobotData
		if err := json.Unmarshal(convertedData, &robotData); err != nil {
			t.Fatalf("Failed to parse converted JSON: %v", err)
		}

		if robotData.Source != topic {
			t.Errorf("Expected source '%s', got '%s'", topic, robotData.Source)
		}

		if robotData.Command != "status_update" {
			t.Errorf("Expected command 'status_update', got '%s'", robotData.Command)
		}
	})

	t.Run("Test Command Conversion", func(t *testing.T) {
		topic := "plc/command"
		payload := []byte("cmd001:move:10.5,20.3,30.1")

		convertedData, targetTopic, err := conv.Convert(topic, payload)
		if err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		if targetTopic != "robot/command" {
			t.Errorf("Expected target topic 'robot/command', got '%s'", targetTopic)
		}

		// Parse the converted JSON
		var robotData models.RobotData
		if err := json.Unmarshal(convertedData, &robotData); err != nil {
			t.Fatalf("Failed to parse converted JSON: %v", err)
		}

		if robotData.Command != "command_execute" {
			t.Errorf("Expected command 'command_execute', got '%s'", robotData.Command)
		}
	})

	t.Run("Test JSON Data Conversion", func(t *testing.T) {
		topic := "plc/data"
		payload := []byte(`{"temperature": 25.5, "humidity": 60.2}`)

		convertedData, targetTopic, err := conv.Convert(topic, payload)
		if err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		if targetTopic != "robot/data" {
			t.Errorf("Expected target topic 'robot/data', got '%s'", targetTopic)
		}

		// Parse the converted JSON
		var robotData models.RobotData
		if err := json.Unmarshal(convertedData, &robotData); err != nil {
			t.Fatalf("Failed to parse converted JSON: %v", err)
		}

		if robotData.Command != "data_update" {
			t.Errorf("Expected command 'data_update', got '%s'", robotData.Command)
		}
	})
}
