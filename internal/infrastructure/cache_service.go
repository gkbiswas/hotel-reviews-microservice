package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheService provides caching functionality
type CacheService struct {
	client *redis.Client
	logger interface{}
}

// NewCacheService creates a new cache service
func NewCacheService(client *redis.Client, logger interface{}) *CacheService {
	return &CacheService{
		client: client,
		logger: logger,
	}
}

// Get retrieves a value from cache
func (c *CacheService) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get from cache: %w", err)
	}
	return val, nil
}

// Set stores a value in cache with expiration
func (c *CacheService) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	err := c.client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}
	return nil
}

// Delete removes a value from cache
func (c *CacheService) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}
	return nil
}

// Exists checks if a key exists in cache
func (c *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}
	return exists > 0, nil
}

// SetJSON stores a JSON object in cache
func (c *CacheService) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return c.Set(ctx, key, string(data), expiration)
}

// GetJSON retrieves and unmarshals a JSON object from cache
func (c *CacheService) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	
	err = json.Unmarshal([]byte(val), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return nil
}

// Expire sets expiration for a key
func (c *CacheService) Expire(ctx context.Context, key string, expiration time.Duration) error {
	err := c.client.Expire(ctx, key, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration: %w", err)
	}
	return nil
}

// TTL returns the remaining time to live for a key
func (c *CacheService) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}
	return ttl, nil
}

// Clear removes all keys matching a pattern
func (c *CacheService) Clear(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete key %s: %w", iter.Val(), err)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}
	return nil
}

// Ping checks if cache is accessible
func (c *CacheService) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}