package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// MQTT
	MQTTBroker   string
	MQTTPort     string
	MQTTClientID string
	MQTTUsername string
	MQTTPassword string

	// PLC Topics
	PlcCommandTopic  string // PLC에서 명령 수신
	PlcResponseTopic string // PLC로 최종 응답
	PlcStatusTopic   string // PLC로 실시간 상태 전송 (추가)

	// Robot Configuration
	RobotSerialNumber string
	RobotManufacturer string

	// Application
	LogLevel       string
	TimeoutSeconds int
	Timeout        time.Duration

	// Performance
	StatusUpdateInterval time.Duration // 상태 업데이트 주기
	EnableStatusHistory  bool          // PLC 상태 이력 저장 여부
	MaxRetryAttempts     int           // 최대 재시도 횟수

	// Cleanup
	DataRetentionDays int // 데이터 보존 기간 (일)
}

func Load() (*Config, error) {
	// .env 파일 로드
	if err := godotenv.Load(); err != nil {
		// .env 파일이 없어도 에러를 반환하지 않도록 처리
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	timeoutSeconds, _ := strconv.Atoi(getEnv("TIMEOUT_SECONDS", "30"))
	statusUpdateMs, _ := strconv.Atoi(getEnv("STATUS_UPDATE_INTERVAL_MS", "1000"))
	maxRetry, _ := strconv.Atoi(getEnv("MAX_RETRY_ATTEMPTS", "3"))
	retentionDays, _ := strconv.Atoi(getEnv("DATA_RETENTION_DAYS", "30"))

	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "mqtt_bridge"),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		// MQTT
		MQTTBroker:   getEnv("MQTT_BROKER", "tcp://localhost:1883"),
		MQTTPort:     getEnv("MQTT_PORT", "1883"),
		MQTTClientID: getEnv("MQTT_CLIENT_ID", "DEX0002_PLC_BRIDGE"),
		MQTTUsername: getEnv("MQTT_USERNAME", "DEX0002_PLC_BRIDGE"),
		MQTTPassword: getEnv("MQTT_PASSWORD", "DEX0002_PLC_BRIDGE"),

		// PLC Topics
		PlcCommandTopic:  getEnv("PLC_COMMAND_TOPIC", "bridge/command"),
		PlcResponseTopic: getEnv("PLC_RESPONSE_TOPIC", "bridge/response"),
		PlcStatusTopic:   getEnv("PLC_STATUS_TOPIC", "bridge/status"), // 추가

		// Robot Configuration
		RobotSerialNumber: getEnv("ROBOT_SERIAL_NUMBER", "DEX0002"),
		RobotManufacturer: getEnv("ROBOT_MANUFACTURER", "Roboligent"),

		// Application
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		TimeoutSeconds: timeoutSeconds,
		Timeout:        time.Duration(timeoutSeconds) * time.Second,

		// Performance
		StatusUpdateInterval: time.Duration(statusUpdateMs) * time.Millisecond,
		EnableStatusHistory:  getEnvBool("ENABLE_STATUS_HISTORY", false),
		MaxRetryAttempts:     maxRetry,

		// Cleanup
		DataRetentionDays: retentionDays,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return boolValue
}

// IsProduction 프로덕션 환경인지 확인
func (c *Config) IsProduction() bool {
	env := getEnv("ENVIRONMENT", "development")
	return env == "production" || env == "prod"
}

// GetRobotTopic 로봇 관련 MQTT 토픽 생성
func (c *Config) GetRobotTopic(topicType string) string {
	return fmt.Sprintf("meili/v2/%s/%s/%s", c.RobotManufacturer, c.RobotSerialNumber, topicType)
}

// GetWorkflowPath 워크플로우 설정 파일 경로
func (c *Config) GetWorkflowPath() string {
	return getEnv("WORKFLOW_CONFIG_PATH", "./configs/workflows.json")
}
