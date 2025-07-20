package infrastructure

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// Test RateLimiter with local limiter (no Redis)
func TestRateLimiter_LocalLimiter_Creation(t *testing.T) {
	maxRequests := 10
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, nil)

	assert.NotNil(t, limiter)
	assert.NotNil(t, limiter.localLimiter)
	assert.Nil(t, limiter.redisClient)
	assert.Equal(t, maxRequests, limiter.maxRequests)
	assert.Equal(t, window, limiter.window)
}

func TestRateLimiter_LocalLimiter_Allow(t *testing.T) {
	maxRequests := 5
	window := 1 * time.Second

	limiter := NewRateLimiter(maxRequests, window, nil)
	ctx := context.Background()

	// First few requests should be allowed
	for i := 0; i < maxRequests; i++ {
		allowed, err := limiter.Allow(ctx, "test-key")
		assert.NoError(t, err)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	// Additional requests should be rate limited
	allowed, err := limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	assert.False(t, allowed, "Request should be rate limited")
}

func TestRateLimiter_LocalLimiter_GetLimit(t *testing.T) {
	maxRequests := 10
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, nil)
	ctx := context.Background()

	limit, remaining, resetAt, err := limiter.GetLimit(ctx, "test-key")
	assert.NoError(t, err)
	assert.Equal(t, maxRequests, limit)
	assert.True(t, remaining >= 0)
	assert.True(t, resetAt.After(time.Now()))
}

func TestRateLimiter_LocalLimiter_Reset(t *testing.T) {
	maxRequests := 5
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, nil)
	ctx := context.Background()

	// Reset should not error even with local limiter
	err := limiter.Reset(ctx, "test-key")
	assert.NoError(t, err)
}

// Test RateLimiter with Redis
func TestRateLimiter_Redis_Creation(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 10
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)

	assert.NotNil(t, limiter)
	assert.NotNil(t, limiter.localLimiter)
	assert.NotNil(t, limiter.redisClient)
	assert.Equal(t, maxRequests, limiter.maxRequests)
	assert.Equal(t, window, limiter.window)
}

func TestRateLimiter_Redis_Allow_NewKey(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 5
	window := 1 * time.Second

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()

	// First request for new key should be allowed
	allowed, err := limiter.Allow(ctx, "new-key")
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Verify key exists in Redis with count 1
	count, err := client.Get(ctx, "rate_limit:new-key").Int()
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRateLimiter_Redis_Allow_ExistingKey(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 3
	window := 10 * time.Second

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "existing-key"

	// Make requests up to the limit
	for i := 0; i < maxRequests; i++ {
		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	// Next request should be denied
	allowed, err := limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.False(t, allowed, "Request should be rate limited")
}

func TestRateLimiter_Redis_Allow_KeyExpiration(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 2
	window := 100 * time.Millisecond

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "expiring-key"

	// Use up the limit
	for i := 0; i < maxRequests; i++ {
		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Should be rate limited
	allowed, err := limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again after expiration
	allowed, err = limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiter_Redis_GetLimit_NewKey(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 10
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()

	limit, remaining, resetAt, err := limiter.GetLimit(ctx, "new-key")
	assert.NoError(t, err)
	assert.Equal(t, maxRequests, limit)
	assert.Equal(t, maxRequests, remaining)
	assert.True(t, resetAt.After(time.Now()))
}

func TestRateLimiter_Redis_GetLimit_ExistingKey(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 5
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "existing-key"

	// Make some requests
	requestsMade := 3
	for i := 0; i < requestsMade; i++ {
		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Check remaining limit
	limit, remaining, resetAt, err := limiter.GetLimit(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, maxRequests, limit)
	assert.Equal(t, maxRequests-requestsMade, remaining)
	assert.True(t, resetAt.After(time.Now()))
}

func TestRateLimiter_Redis_GetLimit_ExceededLimit(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 2
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "exceeded-key"

	// Exceed the limit
	for i := 0; i < maxRequests+2; i++ {
		limiter.Allow(ctx, key)
	}

	// Check remaining limit (should be 0)
	limit, remaining, resetAt, err := limiter.GetLimit(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, maxRequests, limit)
	assert.Equal(t, 0, remaining)
	assert.True(t, resetAt.After(time.Now()))
}

func TestRateLimiter_Redis_Reset(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 3
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "reset-key"

	// Use up the limit
	for i := 0; i < maxRequests; i++ {
		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Should be rate limited
	allowed, err := limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Reset the rate limit
	err = limiter.Reset(ctx, key)
	assert.NoError(t, err)

	// Should be allowed again after reset
	allowed, err = limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// Test Redis connection failure fallback
func TestRateLimiter_Redis_ConnectionFailure(t *testing.T) {
	// Create Redis client with invalid address
	client := redis.NewClient(&redis.Options{
		Addr: "invalid:9999",
	})
	defer client.Close()

	maxRequests := 5
	window := 1 * time.Second

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()

	// Should fallback to local limiter when Redis fails
	allowed, err := limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	_ = allowed // Mark as used - the result depends on local limiter state, but should not error
}

func TestRateLimiter_Redis_GetLimit_ConnectionFailure(t *testing.T) {
	// Create Redis client with invalid address
	client := redis.NewClient(&redis.Options{
		Addr: "invalid:9999",
	})
	defer client.Close()

	maxRequests := 5
	window := 1 * time.Second

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()

	// Should return error when Redis fails for GetLimit
	_, _, _, err := limiter.GetLimit(ctx, "test-key")
	assert.Error(t, err)
}

// Test concurrent access to rate limiter
func TestRateLimiter_Redis_ConcurrentAccess(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 10
	window := 1 * time.Second

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "concurrent-key"

	// Launch multiple goroutines
	numGoroutines := 20
	results := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			allowed, err := limiter.Allow(ctx, key)
			if err != nil {
				results <- false
				return
			}
			results <- allowed
		}()
	}

	// Collect results
	allowedCount := 0
	deniedCount := 0

	for i := 0; i < numGoroutines; i++ {
		result := <-results
		if result {
			allowedCount++
		} else {
			deniedCount++
		}
	}

	// Should have allowed exactly maxRequests
	assert.Equal(t, maxRequests, allowedCount)
	assert.Equal(t, numGoroutines-maxRequests, deniedCount)
}

// Test different keys have independent limits
func TestRateLimiter_Redis_IndependentKeys(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 2
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()

	key1 := "key1"
	key2 := "key2"

	// Use up limit for key1
	for i := 0; i < maxRequests; i++ {
		allowed, err := limiter.Allow(ctx, key1)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// key1 should be rate limited
	allowed, err := limiter.Allow(ctx, key1)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// key2 should still be allowed
	allowed, err = limiter.Allow(ctx, key2)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// Test rate limiter with zero max requests
func TestRateLimiter_ZeroMaxRequests(t *testing.T) {
	maxRequests := 0
	window := 1 * time.Second

	limiter := NewRateLimiter(maxRequests, window, nil)
	ctx := context.Background()

	// Should always be rate limited with zero max requests
	allowed, err := limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	assert.False(t, allowed)
}

// Test rate limiter with very short window
func TestRateLimiter_ShortWindow(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 1
	window := 10 * time.Millisecond

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "short-window-key"

	// First request should be allowed
	allowed, err := limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Immediate second request should be denied
	allowed, err = limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Wait for window to expire
	time.Sleep(15 * time.Millisecond)

	// Should be allowed again
	allowed, err = limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// Test Redis key format
func TestRateLimiter_Redis_KeyFormat(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 5
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)
	ctx := context.Background()
	key := "test-key"

	// Make a request
	allowed, err := limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Verify the Redis key format
	expectedRedisKey := "rate_limit:test-key"
	exists := client.Exists(ctx, expectedRedisKey).Val()
	assert.Equal(t, int64(1), exists)
}

// Test context cancellation
func TestRateLimiter_ContextCancellation(t *testing.T) {
	// Start mini Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	maxRequests := 5
	window := 1 * time.Minute

	limiter := NewRateLimiter(maxRequests, window, client)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should handle cancelled context gracefully
	allowed, err := limiter.Allow(ctx, "test-key")
	// Should fallback to local limiter when context is cancelled
	assert.NoError(t, err)
	_ = allowed // Mark as used - allowed result depends on local limiter state

	// Reset should also handle cancelled context
	err = limiter.Reset(ctx, "test-key")
	assert.Error(t, err) // Redis operations should fail with cancelled context

	// GetLimit should handle cancelled context
	_, _, _, err = limiter.GetLimit(ctx, "test-key")
	assert.Error(t, err) // Redis operations should fail with cancelled context
}

// Benchmark tests
func BenchmarkRateLimiter_LocalLimiter_Allow(b *testing.B) {
	limiter := NewRateLimiter(1000000, 1*time.Minute, nil) // High limit to avoid blocking
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench-key-%d", i%100) // Use multiple keys
			limiter.Allow(ctx, key)
			i++
		}
	})
}

func BenchmarkRateLimiter_Redis_Allow(b *testing.B) {
	// Start mini Redis server
	s := miniredis.RunT(b)
	defer s.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	limiter := NewRateLimiter(1000000, 1*time.Minute, client) // High limit to avoid blocking
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench-key-%d", i%100) // Use multiple keys
			limiter.Allow(ctx, key)
			i++
		}
	})
}

