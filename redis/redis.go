package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/models"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient(cfg *config.Config) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx := context.Background()

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	fmt.Println("Redis connected successfully")
	return &RedisClient{
		client: rdb,
		ctx:    ctx,
	}, nil
}

func (r *RedisClient) SaveState(serialNumber string, state *models.StateMessage) error {
	key := fmt.Sprintf("robot:state:%s", serialNumber)

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Set with expiration (24 hours)
	err = r.client.Set(r.ctx, key, stateJSON, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to save state to Redis: %w", err)
	}

	return nil
}

func (r *RedisClient) GetState(serialNumber string) (*models.StateMessage, error) {
	key := fmt.Sprintf("robot:state:%s", serialNumber)

	val, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("state not found for robot %s", serialNumber)
		}
		return nil, fmt.Errorf("failed to get state from Redis: %w", err)
	}

	var state models.StateMessage
	err = json.Unmarshal([]byte(val), &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

func (r *RedisClient) SaveConnectionStatus(serialNumber, status string) error {
	key := fmt.Sprintf("robot:connection:%s", serialNumber)

	connectionInfo := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().Unix(),
	}

	infoJSON, err := json.Marshal(connectionInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal connection info: %w", err)
	}

	err = r.client.Set(r.ctx, key, infoJSON, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to save connection status to Redis: %w", err)
	}

	return nil
}

func (r *RedisClient) GetConnectionStatus(serialNumber string) (string, error) {
	key := fmt.Sprintf("robot:connection:%s", serialNumber)

	val, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("connection status not found for robot %s", serialNumber)
		}
		return "", fmt.Errorf("failed to get connection status from Redis: %w", err)
	}

	var connectionInfo map[string]interface{}
	err = json.Unmarshal([]byte(val), &connectionInfo)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal connection info: %w", err)
	}

	status, ok := connectionInfo["status"].(string)
	if !ok {
		return "", fmt.Errorf("invalid connection status format")
	}

	return status, nil
}

func (r *RedisClient) SetRobotOnline(serialNumber string) error {
	return r.SaveConnectionStatus(serialNumber, "ONLINE")
}

func (r *RedisClient) SetRobotOffline(serialNumber string) error {
	return r.SaveConnectionStatus(serialNumber, "OFFLINE")
}

func (r *RedisClient) IsRobotOnline(serialNumber string) bool {
	status, err := r.GetConnectionStatus(serialNumber)
	if err != nil {
		return false
	}
	return status == "ONLINE"
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
