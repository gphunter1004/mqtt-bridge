package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Broker configuration
	BrokerURL      string
	BrokerClientID string
	BrokerUsername string
	BrokerPassword string

	// Topic configuration
	PLCTopicPrefix   string
	RobotTopicPrefix string

	// Logging configuration
	LogLevel string
	LogFile  string

	// Application configuration
	AppName    string
	AppVersion string
}

func Load() (*Config, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		// .env file is optional, so we don't fail if it doesn't exist
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	config := &Config{
		BrokerURL:        getEnv("BROKER_URL", "tcp://localhost:1883"),
		BrokerClientID:   getEnv("BROKER_CLIENT_ID", "data_converter_client"),
		BrokerUsername:   getEnv("BROKER_USERNAME", ""),
		BrokerPassword:   getEnv("BROKER_PASSWORD", ""),
		PLCTopicPrefix:   getEnv("PLC_TOPIC_PREFIX", "plc/"),
		RobotTopicPrefix: getEnv("ROBOT_TOPIC_PREFIX", "robot/"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		LogFile:          getEnv("LOG_FILE", "logs/converter.log"),
		AppName:          getEnv("APP_NAME", "Topic Data Converter"),
		AppVersion:       getEnv("APP_VERSION", "1.0.0"),
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

func (c *Config) validate() error {
	if c.BrokerURL == "" {
		return fmt.Errorf("BROKER_URL is required")
	}
	if c.BrokerClientID == "" {
		return fmt.Errorf("BROKER_CLIENT_ID is required")
	}
	return nil
}

func (c *Config) GetPLCTopics() []string {
	// Define PLC topics to subscribe to
	return []string{
		c.PLCTopicPrefix + "status",
		c.PLCTopicPrefix + "data",
		c.PLCTopicPrefix + "command",
	}
}

func (c *Config) GetRobotTopic(plcTopic string) string {
	// Convert PLC topic to Robot topic
	topicName := strings.TrimPrefix(plcTopic, c.PLCTopicPrefix)
	return c.RobotTopicPrefix + topicName
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
