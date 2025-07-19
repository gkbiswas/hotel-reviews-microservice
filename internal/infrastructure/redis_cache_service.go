package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// RedisCacheService implements domain.CacheService using Redis
type RedisCacheService struct {
	client *redis.Client
	logger *logger.Logger
	ttl    time.Duration
}

// NewRedisCacheService creates a new Redis cache service
func NewRedisCacheService(client *redis.Client, logger *logger.Logger) *RedisCacheService {
	return &RedisCacheService{
		client: client,
		logger: logger,
		ttl:    5 * time.Minute, // Default TTL
	}
}

// Get retrieves a value from cache
func (r *RedisCacheService) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}
	return []byte(val), nil
}

// Set stores a value in cache
func (r *RedisCacheService) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	if expiration == 0 {
		expiration = r.ttl
	}

	err := r.client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}
	return nil
}

// Delete removes a key from cache
func (r *RedisCacheService) Delete(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}
	return nil
}

// Exists checks if a key exists in cache
func (r *RedisCacheService) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check cache existence: %w", err)
	}
	return n > 0, nil
}

// FlushAll clears all cache
func (r *RedisCacheService) FlushAll(ctx context.Context) error {
	err := r.client.FlushAll(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to flush cache: %w", err)
	}
	return nil
}

// GetReviewSummary retrieves a review summary from cache
func (r *RedisCacheService) GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	key := fmt.Sprintf("hotel:summary:%s", hotelID.String())

	data, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil // Cache miss
	}

	var summary domain.ReviewSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to unmarshal review summary: %w", err)
	}

	return &summary, nil
}

// SetReviewSummary stores a review summary in cache
func (r *RedisCacheService) SetReviewSummary(ctx context.Context, hotelID uuid.UUID, summary *domain.ReviewSummary, expiration time.Duration) error {
	key := fmt.Sprintf("hotel:summary:%s", hotelID.String())

	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal review summary: %w", err)
	}

	return r.Set(ctx, key, data, expiration)
}

// InvalidateReviewSummary removes a review summary from cache
func (r *RedisCacheService) InvalidateReviewSummary(ctx context.Context, hotelID uuid.UUID) error {
	key := fmt.Sprintf("hotel:summary:%s", hotelID.String())
	return r.Delete(ctx, key)
}
