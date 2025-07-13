// internal/config/config.go (Corrected)
package config

import (
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
	MQTTBroker       string
	MQTTPort         string
	MQTTClientID     string
	MQTTUsername     string
	MQTTPassword     string
	PlcResponseTopic string // Added PLC response topic

	// Robot Configuration
	RobotSerialNumber string
	RobotManufacturer string

	// Application
	LogLevel       string
	TimeoutSeconds int
	Timeout        time.Duration
}

func Load() (*Config, error) {
	// .env 파일 로드
	if err := godotenv.Load(); err != nil {
		// .env 파일이 없어도 에러를 반환하지 않도록 처리
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	timeoutSeconds, _ := strconv.Atoi(getEnv("TIMEOUT_SECONDS", "30"))

	return &Config{
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnv("DB_PORT", "5432"),
		DBUser:            getEnv("DB_USER", "postgres"),
		DBPassword:        getEnv("DB_PASSWORD", "password"),
		DBName:            getEnv("DB_NAME", "mqtt_bridge"),
		RedisHost:         getEnv("REDIS_HOST", "localhost"),
		RedisPort:         getEnv("REDIS_PORT", "6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           redisDB,
		MQTTBroker:        getEnv("MQTT_BROKER", "tcp://localhost:1883"),
		MQTTPort:          getEnv("MQTT_PORT", "1883"),
		MQTTClientID:      getEnv("MQTT_CLIENT_ID", "DEX0002_PLC_BRIDGE"),
		MQTTUsername:      getEnv("MQTT_USERNAME", "DEX0002_PLC_BRIDGE"),
		MQTTPassword:      getEnv("MQTT_PASSWORD", "DEX0002_PLC_BRIDGE"),
		PlcResponseTopic:  getEnv("PLC_RESPONSE_TOPIC", "bridge/response"),
		RobotSerialNumber: getEnv("ROBOT_SERIAL_NUMBER", "DEX0002"),
		RobotManufacturer: getEnv("ROBOT_MANUFACTURER", "Roboligent"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		TimeoutSeconds:    timeoutSeconds,
		Timeout:           time.Duration(timeoutSeconds) * time.Second,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
