package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/models"

	"github.com/go-redis/redis/v8"
)

// RedisClient wraps the go-redis client to provide application-specific methods.
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
	logger *slog.Logger
}

// NewRedisClient creates and connects a new Redis client.
func NewRedisClient(cfg *config.Config, logger *slog.Logger) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx := context.Background()

	// Test connection
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Redis connected successfully")
	return &RedisClient{
		client: rdb,
		ctx:    ctx,
		logger: logger.With("component", "redis_client"),
	}, nil
}

// SaveState saves the robot's state message to Redis with an expiration.
func (r *RedisClient) SaveState(serialNumber string, state *models.StateMessage) error {
	key := fmt.Sprintf("robot:state:%s", serialNumber)
	logger := r.logger.With("key", key, "serialNumber", serialNumber)

	stateJSON, err := json.Marshal(state)
	if err != nil {
		logger.Error("Failed to marshal state", slog.Any("error", err))
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Set with expiration (e.g., 24 hours)
	err = r.client.Set(r.ctx, key, stateJSON, 24*time.Hour).Err()
	if err != nil {
		logger.Error("Failed to save state to Redis", slog.Any("error", err))
		return fmt.Errorf("failed to save state to Redis: %w", err)
	}

	logger.Debug("Robot state saved to Redis")
	return nil
}

// GetState retrieves the robot's state message from Redis.
func (r *RedisClient) GetState(serialNumber string) (*models.StateMessage, error) {
	key := fmt.Sprintf("robot:state:%s", serialNumber)
	logger := r.logger.With("key", key, "serialNumber", serialNumber)

	val, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("state not found for robot %s", serialNumber)
		}
		logger.Error("Failed to get state from Redis", slog.Any("error", err))
		return nil, fmt.Errorf("failed to get state from Redis: %w", err)
	}

	var state models.StateMessage
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		logger.Error("Failed to unmarshal state", slog.Any("error", err))
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// SaveConnectionStatus saves the robot's connection status (ONLINE/OFFLINE) to Redis.
func (r *RedisClient) SaveConnectionStatus(serialNumber, status string) error {
	key := fmt.Sprintf("robot:connection:%s", serialNumber)
	logger := r.logger.With("key", key, "serialNumber", serialNumber, "status", status)

	connectionInfo := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().Unix(),
	}

	infoJSON, err := json.Marshal(connectionInfo)
	if err != nil {
		logger.Error("Failed to marshal connection info", slog.Any("error", err))
		return fmt.Errorf("failed to marshal connection info: %w", err)
	}

	err = r.client.Set(r.ctx, key, infoJSON, 24*time.Hour).Err()
	if err != nil {
		logger.Error("Failed to save connection status to Redis", slog.Any("error", err))
		return fmt.Errorf("failed to save connection status to Redis: %w", err)
	}

	logger.Info("Robot connection status updated")
	return nil
}

// GetConnectionStatus retrieves the robot's connection status from Redis.
func (r *RedisClient) GetConnectionStatus(serialNumber string) (string, error) {
	key := fmt.Sprintf("robot:connection:%s", serialNumber)
	val, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "OFFLINE", nil // Return OFFLINE if key doesn't exist
		}
		r.logger.Error("Failed to get connection status from Redis", "key", key, slog.Any("error", err))
		return "", fmt.Errorf("failed to get connection status from Redis: %w", err)
	}

	var connectionInfo map[string]interface{}
	if err := json.Unmarshal([]byte(val), &connectionInfo); err != nil {
		r.logger.Error("Failed to unmarshal connection info", "key", key, slog.Any("error", err))
		return "", fmt.Errorf("failed to unmarshal connection info: %w", err)
	}

	status, ok := connectionInfo["status"].(string)
	if !ok {
		return "", fmt.Errorf("invalid connection status format in Redis")
	}

	return status, nil
}

// IsRobotOnline is a convenient helper to check if a robot is online.
func (r *RedisClient) IsRobotOnline(serialNumber string) bool {
	status, err := r.GetConnectionStatus(serialNumber)
	if err != nil {
		r.logger.Warn("Could not determine robot online status, assuming offline", "serialNumber", serialNumber, slog.Any("error", err))
		return false
	}
	return status == "ONLINE"
}

// Close gracefully closes the Redis client connection.
func (r *RedisClient) Close() error {
	r.logger.Info("Closing Redis client connection.")
	return r.client.Close()
}
