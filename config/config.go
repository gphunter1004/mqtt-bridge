package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application, loaded from environment variables.
type Config struct {
	// Database settings
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis settings
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// MQTT settings
	MQTTBroker   string
	MQTTPort     string
	MQTTClientID string
	MQTTUsername string
	MQTTPassword string

	// Application settings
	LogLevel string        // Added for structured logging (e.g., "DEBUG", "INFO")
	Timeout  time.Duration // General purpose timeout for operations
	Version  string        // Added for application version tracking
}

// LoadConfig loads configuration from a .env file or environment variables.
func LoadConfig() *Config {
	// Attempt to load .env file. It's okay if it doesn't exist; environment variables will be used as a fallback.
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables.")
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	timeoutSec, _ := strconv.Atoi(getEnv("TIMEOUT_SECONDS", "30"))

	return &Config{
		// Database settings
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "mqtt_bridge"),

		// Redis settings
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		// MQTT settings
		MQTTBroker:   getEnv("MQTT_BROKER", "tcp://localhost:1883"),
		MQTTPort:     getEnv("MQTT_PORT", "1883"),
		MQTTClientID: getEnv("MQTT_CLIENT_ID", "bridge-server"),
		MQTTUsername: getEnv("MQTT_USERNAME", ""),
		MQTTPassword: getEnv("MQTT_PASSWORD", ""),

		// Application settings
		LogLevel: getEnv("LOG_LEVEL", "INFO"),             // Default log level is INFO
		Timeout:  time.Duration(timeoutSec) * time.Second, // Default timeout is 30 seconds
		Version:  getEnv("APP_VERSION", "1.0.0-snapshot"), // Default version
	}
}

// getEnv retrieves an environment variable by key or returns a default value.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
