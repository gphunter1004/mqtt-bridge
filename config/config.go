package config

import (
	"log"
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

	// Application
	LogLevel string
	Timeout  time.Duration
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	timeoutSec, _ := strconv.Atoi(getEnv("TIMEOUT_SECONDS", "30"))

	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "mqtt_bridge"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		MQTTBroker:   getEnv("MQTT_BROKER", "tcp://localhost:1883"),
		MQTTPort:     getEnv("MQTT_PORT", "1883"),
		MQTTClientID: getEnv("MQTT_CLIENT_ID", "bridge-server"),
		MQTTUsername: getEnv("MQTT_USERNAME", ""),
		MQTTPassword: getEnv("MQTT_PASSWORD", ""),

		LogLevel: getEnv("LOG_LEVEL", "info"),
		Timeout:  time.Duration(timeoutSec) * time.Second,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
