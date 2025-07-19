package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	localLimiter *rate.Limiter
	redisClient  *redis.Client
	maxRequests  int
	window       time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxRequests int, window time.Duration, redisClient *redis.Client) *RateLimiter {
	// Create local rate limiter as fallback
	localLimiter := rate.NewLimiter(rate.Limit(float64(maxRequests)/window.Seconds()), maxRequests)
	
	return &RateLimiter{
		localLimiter: localLimiter,
		redisClient:  redisClient,
		maxRequests:  maxRequests,
		window:       window,
	}
}

// Allow checks if a request is allowed for the given key
func (r *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	// If Redis is not available, use local limiter
	if r.redisClient == nil {
		return r.localLimiter.Allow(), nil
	}

	// Use Redis for distributed rate limiting
	redisKey := fmt.Sprintf("rate_limit:%s", key)
	
	// Get current count
	count, err := r.redisClient.Get(ctx, redisKey).Int()
	if err == redis.Nil {
		// Key doesn't exist, create it
		pipe := r.redisClient.Pipeline()
		pipe.Incr(ctx, redisKey)
		pipe.Expire(ctx, redisKey, r.window)
		_, err = pipe.Exec(ctx)
		if err != nil {
			// Fallback to local limiter
			return r.localLimiter.Allow(), nil
		}
		return true, nil
	} else if err != nil {
		// Error occurred, fallback to local limiter
		return r.localLimiter.Allow(), nil
	}

	// Check if limit exceeded
	if count >= r.maxRequests {
		return false, nil
	}

	// Increment counter
	_, err = r.redisClient.Incr(ctx, redisKey).Result()
	if err != nil {
		// Fallback to local limiter
		return r.localLimiter.Allow(), nil
	}

	return true, nil
}

// Reset resets the rate limit for a key
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	if r.redisClient == nil {
		return nil
	}

	redisKey := fmt.Sprintf("rate_limit:%s", key)
	return r.redisClient.Del(ctx, redisKey).Err()
}

// GetLimit returns the current limit and remaining requests for a key
func (r *RateLimiter) GetLimit(ctx context.Context, key string) (limit, remaining int, resetAt time.Time, err error) {
	limit = r.maxRequests
	
	if r.redisClient == nil {
		// For local limiter, we can't get exact remaining
		if r.localLimiter.Allow() {
			remaining = 1
		} else {
			remaining = 0
		}
		resetAt = time.Now().Add(r.window)
		return
	}

	redisKey := fmt.Sprintf("rate_limit:%s", key)
	
	// Get current count
	count, err := r.redisClient.Get(ctx, redisKey).Int()
	if err == redis.Nil {
		remaining = limit
		resetAt = time.Now().Add(r.window)
		return limit, remaining, resetAt, nil
	} else if err != nil {
		return 0, 0, time.Time{}, err
	}

	remaining = limit - count
	if remaining < 0 {
		remaining = 0
	}

	// Get TTL for reset time
	ttl, err := r.redisClient.TTL(ctx, redisKey).Result()
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	resetAt = time.Now().Add(ttl)
	return limit, remaining, resetAt, nil
}