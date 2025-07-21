package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRateLimitTest() (*gin.Engine, *RateLimitMiddleware) {
	gin.SetMode(gin.TestMode)
	
	config := &RateLimitConfig{
		RequestsPerSecond: 5, // Very low for testing
		BurstSize:         2,
		WindowSize:        time.Second * 10,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}
	
	testLogger := logger.NewDefault()
	rateLimiter := NewRateLimitMiddleware(config, nil, testLogger) // No Redis for testing
	
	router := gin.New()
	
	return router, rateLimiter
}

func TestRateLimitMiddleware(t *testing.T) {
	router, rateLimiter := setupRateLimitTest()
	
	router.Use(rateLimiter.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make multiple requests rapidly
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if i < 2 {
			// First few requests should succeed
			assert.Equal(t, http.StatusOK, w.Code)
			
			// Check rate limit headers
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
		}
	}
}

func TestRateLimitExceeded(t *testing.T) {
	router, rateLimiter := setupRateLimitTest()
	
	// Set very restrictive rate limit
	rateLimiter.config.RequestsPerSecond = 1
	rateLimiter.config.BurstSize = 1
	
	router.Use(rateLimiter.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.NotEmpty(t, w2.Header().Get("Retry-After"))
	
	// Check response contains error information
	assert.Contains(t, w2.Body.String(), "RATE_LIMIT_EXCEEDED")
}

func TestRateLimitHeaders(t *testing.T) {
	router, rateLimiter := setupRateLimitTest()
	
	router.Use(rateLimiter.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	// Check all required rate limit headers are present
	limitHeader := w.Header().Get("X-RateLimit-Limit")
	remainingHeader := w.Header().Get("X-RateLimit-Remaining")
	resetHeader := w.Header().Get("X-RateLimit-Reset")
	
	assert.NotEmpty(t, limitHeader)
	assert.NotEmpty(t, remainingHeader)
	assert.NotEmpty(t, resetHeader)
	
	// Verify header values make sense
	limit, err := strconv.Atoi(limitHeader)
	require.NoError(t, err)
	assert.Equal(t, rateLimiter.config.RequestsPerSecond, limit)
	
	remaining, err := strconv.Atoi(remainingHeader)
	require.NoError(t, err)
	assert.True(t, remaining >= 0)
	
	resetTime, err := strconv.ParseInt(resetHeader, 10, 64)
	require.NoError(t, err)
	assert.True(t, resetTime > time.Now().Unix())
}

func TestUserBasedRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := UserBasedRateLimitConfig(10)
	testLogger := logger.NewDefault()
	rateLimiter := NewRateLimitMiddleware(config, nil, testLogger)
	
	router := gin.New()
	
	// Middleware to set user ID
	router.Use(func(c *gin.Context) {
		userID := c.Query("user_id")
		if userID != "" {
			c.Set("user_id", userID)
		}
		c.Next()
	})
	
	router.Use(rateLimiter.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test requests from different users
	req1 := httptest.NewRequest(http.MethodGet, "/test?user_id=user1", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/test?user_id=user2", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	
	// Both should succeed as they're different users
}

func TestIPBasedRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := IPBasedRateLimitConfig(5)
	testLogger := logger.NewDefault()
	rateLimiter := NewRateLimitMiddleware(config, nil, testLogger)
	
	router := gin.New()
	router.Use(rateLimiter.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify key generator is working for IP-based limiting
	// Create a proper test context
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	expectedKey := config.KeyGenerator(c)
	assert.Contains(t, expectedKey, "ip:")
}

func TestEndpointBasedRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Use higher limit to ensure burst size is adequate
	config := EndpointBasedRateLimitConfig(20)
	testLogger := logger.NewDefault()
	rateLimiter := NewRateLimitMiddleware(config, nil, testLogger)
	
	router := gin.New()
	router.Use(rateLimiter.Middleware())
	router.GET("/test-endpoint", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	router.POST("/test-endpoint", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// GET and POST to same path should have different rate limits
	getReq := httptest.NewRequest(http.MethodGet, "/test-endpoint", nil)
	postReq := httptest.NewRequest(http.MethodPost, "/test-endpoint", nil)
	
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, getReq)
	assert.Equal(t, http.StatusOK, w1.Code)
	
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, postReq)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestTieredRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	testLogger := logger.NewDefault()
	
	router := gin.New()
	
	// Middleware to set user tier
	router.Use(func(c *gin.Context) {
		tier := c.Query("tier")
		if tier != "" {
			c.Set("user_tier", tier)
		}
		c.Next()
	})
	
	router.Use(TieredRateLimitMiddleware(nil, testLogger))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	tests := []struct {
		name string
		tier string
	}{
		{"premium user", "premium"},
		{"standard user", "standard"},
		{"default user", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/test"
			if tt.tier != "" {
				url += "?tier=" + tt.tier
			}
			
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// All should succeed initially
			assert.Equal(t, http.StatusOK, w.Code)
			
			// Check that rate limit headers are present
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
		})
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()
	
	assert.Equal(t, 100, config.RequestsPerSecond)
	assert.Equal(t, 20, config.BurstSize)
	assert.Equal(t, time.Minute, config.WindowSize)
	assert.NotNil(t, config.KeyGenerator)
	
	// Test key generator
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	
	key := config.KeyGenerator(c)
	assert.NotEmpty(t, key)
}