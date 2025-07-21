package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int           // Requests per second
	BurstSize         int           // Burst size
	WindowSize        time.Duration // Time window for counting
	KeyGenerator      func(*gin.Context) string // Function to generate rate limit key
}

// DefaultRateLimitConfig returns a default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         20,
		WindowSize:        time.Minute,
		KeyGenerator: func(c *gin.Context) string {
			// Default to IP-based rate limiting
			return c.ClientIP()
		},
	}
}

// RateLimitMiddleware provides flexible rate limiting
type RateLimitMiddleware struct {
	config      *RateLimitConfig
	redisClient *redis.Client
	logger      *logger.Logger
	limiters    map[string]*rate.Limiter // In-memory limiters for fallback
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(config *RateLimitConfig, redisClient *redis.Client, logger *logger.Logger) *RateLimitMiddleware {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	return &RateLimitMiddleware{
		config:      config,
		redisClient: redisClient,
		logger:      logger,
		limiters:    make(map[string]*rate.Limiter),
	}
}

// Middleware returns the rate limiting middleware function
func (rl *RateLimitMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.config.KeyGenerator(c)
		allowed, remainingRequests, resetTime, err := rl.checkRateLimit(c.Request.Context(), key)

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerSecond))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remainingRequests))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		if err != nil {
			rl.logger.ErrorContext(c.Request.Context(), "Rate limit check failed",
				"key", key,
				"error", err,
			)
			// On error, allow the request but log it
			c.Next()
			return
		}

		if !allowed {
			rl.logger.WarnContext(c.Request.Context(), "Rate limit exceeded",
				"key", key,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"user_agent", c.GetHeader("User-Agent"),
			)

			c.Header("Retry-After", strconv.Itoa(int(rl.config.WindowSize.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "RATE_LIMIT_EXCEEDED",
				"message":     "Too many requests",
				"retry_after": int(rl.config.WindowSize.Seconds()),
				"reset_time":  resetTime.Unix(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkRateLimit checks if a request is allowed
func (rl *RateLimitMiddleware) checkRateLimit(ctx context.Context, key string) (allowed bool, remaining int, resetTime time.Time, err error) {
	if rl.redisClient != nil {
		return rl.checkRedisRateLimit(ctx, key)
	}
	return rl.checkMemoryRateLimit(key)
}

// checkRedisRateLimit implements distributed rate limiting using Redis
func (rl *RateLimitMiddleware) checkRedisRateLimit(ctx context.Context, key string) (bool, int, time.Time, error) {
	redisKey := fmt.Sprintf("rate_limit:%s", key)
	now := time.Now()
	window := now.Truncate(rl.config.WindowSize)
	resetTime := window.Add(rl.config.WindowSize)

	// Use Redis pipeline for atomic operations
	pipe := rl.redisClient.Pipeline()
	incrCmd := pipe.Incr(ctx, redisKey)
	expireCmd := pipe.ExpireAt(ctx, redisKey, resetTime)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, resetTime, err
	}

	count := int(incrCmd.Val())
	_ = expireCmd.Val() // Ensure expire command was executed

	allowed := count <= rl.config.RequestsPerSecond
	remaining := rl.config.RequestsPerSecond - count
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, resetTime, nil
}

// checkMemoryRateLimit implements in-memory rate limiting
func (rl *RateLimitMiddleware) checkMemoryRateLimit(key string) (bool, int, time.Time, error) {
	limiter, exists := rl.limiters[key]
	if !exists {
		// Create new limiter for this key
		limiter = rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize)
		rl.limiters[key] = limiter

		// Clean up old limiters periodically (simple cleanup)
		if len(rl.limiters) > 10000 {
			// Reset limiters map when it gets too large
			rl.limiters = make(map[string]*rate.Limiter)
			rl.limiters[key] = limiter
		}
	}

	allowed := limiter.Allow()
	
	// Calculate remaining tokens (approximate)
	remaining := int(limiter.Tokens())
	resetTime := time.Now().Add(rl.config.WindowSize)

	return allowed, remaining, resetTime, nil
}

// UserBasedRateLimitConfig creates a configuration for user-based rate limiting
func UserBasedRateLimitConfig(requestsPerMinute int) *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: requestsPerMinute,
		BurstSize:         requestsPerMinute / 4, // Allow 25% burst
		WindowSize:        time.Minute,
		KeyGenerator: func(c *gin.Context) string {
			// Try to get user ID from context
			if userID, exists := c.Get("user_id"); exists {
				return fmt.Sprintf("user:%v", userID)
			}
			// Fall back to IP
			return fmt.Sprintf("ip:%s", c.ClientIP())
		},
	}
}

// IPBasedRateLimitConfig creates a configuration for IP-based rate limiting
func IPBasedRateLimitConfig(requestsPerMinute int) *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: requestsPerMinute,
		BurstSize:         requestsPerMinute / 4,
		WindowSize:        time.Minute,
		KeyGenerator: func(c *gin.Context) string {
			return fmt.Sprintf("ip:%s", c.ClientIP())
		},
	}
}

// EndpointBasedRateLimitConfig creates endpoint-specific rate limiting
func EndpointBasedRateLimitConfig(requestsPerMinute int) *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: requestsPerMinute,
		BurstSize:         requestsPerMinute / 4,
		WindowSize:        time.Minute,
		KeyGenerator: func(c *gin.Context) string {
			// Combine IP and endpoint for granular control
			return fmt.Sprintf("endpoint:%s:%s:%s", c.ClientIP(), c.Request.Method, c.FullPath())
		},
	}
}

// TieredRateLimitMiddleware provides different limits for different user types
func TieredRateLimitMiddleware(redisClient *redis.Client, logger *logger.Logger) gin.HandlerFunc {
	// Define different tiers
	premiumLimiter := NewRateLimitMiddleware(
		UserBasedRateLimitConfig(1000), // 1000 requests per minute for premium users
		redisClient,
		logger,
	)
	
	standardLimiter := NewRateLimitMiddleware(
		UserBasedRateLimitConfig(100), // 100 requests per minute for standard users
		redisClient,
		logger,
	)
	
	defaultLimiter := NewRateLimitMiddleware(
		IPBasedRateLimitConfig(60), // 60 requests per minute for unauthenticated users
		redisClient,
		logger,
	)

	return func(c *gin.Context) {
		// Determine user tier
		userTier := "default"
		if tier, exists := c.Get("user_tier"); exists {
			userTier = tier.(string)
		}

		// Apply appropriate rate limit
		switch userTier {
		case "premium":
			premiumLimiter.Middleware()(c)
		case "standard":
			standardLimiter.Middleware()(c)
		default:
			defaultLimiter.Middleware()(c)
		}
	}
}