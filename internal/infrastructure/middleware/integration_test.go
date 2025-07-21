package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestSecurityMiddlewareIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Setup security configuration
	securityConfig := &config.SecurityConfig{
		RateLimit:       10,
		RateLimitWindow: time.Minute,
	}
	
	testLogger := logger.NewDefault()
	securityMiddleware := NewSecurityMiddleware(securityConfig, testLogger, nil)
	
	// Setup rate limiting with more permissive limits for testing
	rateLimitConfig := UserBasedRateLimitConfig(10)
	rateLimitMiddleware := NewRateLimitMiddleware(rateLimitConfig, nil, testLogger)
	
	// Create router with all security middleware
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(securityMiddleware.SecurityHeadersMiddleware())
	router.Use(securityMiddleware.CORSMiddleware())
	router.Use(securityMiddleware.InputValidationMiddleware())
	router.Use(rateLimitMiddleware.Middleware())
	
	// Add test routes
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	router.POST("/api/data", func(c *gin.Context) {
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": data})
	})
	
	t.Run("Complete security check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		// Check security headers
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
		assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
		
		// Check CORS headers
		assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
		
		// Check rate limit headers
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	})
	
	t.Run("Rate limiting protection", func(t *testing.T) {
		// Create a separate router with very restrictive rate limits for this test
		// Use a custom key generator to isolate this test
		restrictiveConfig := &RateLimitConfig{
			RequestsPerSecond: 1,
			BurstSize:         1,
			WindowSize:        time.Second * 10,
			KeyGenerator: func(c *gin.Context) string {
				return "test-rate-limit:" + c.ClientIP()
			},
		}
		restrictiveLimiter := NewRateLimitMiddleware(restrictiveConfig, nil, testLogger)
		
		testRouter := gin.New()
		testRouter.Use(restrictiveLimiter.Middleware())
		testRouter.GET("/test-rate", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		
		// First request should succeed
		req := httptest.NewRequest(http.MethodGet, "/test-rate", nil)
		w := httptest.NewRecorder()
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "First request should succeed")
		
		// Second request should be rate limited immediately due to burst=1
		req = httptest.NewRequest(http.MethodGet, "/test-rate", nil)
		w = httptest.NewRecorder()
		testRouter.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "RATE_LIMIT_EXCEEDED")
	})
	
	t.Run("Input validation protection", func(t *testing.T) {
		// Test with invalid content type
		req := httptest.NewRequest(http.MethodPost, "/api/data", strings.NewReader("<xml>data</xml>"))
		req.Header.Set("Content-Type", "application/xml")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
		assert.Contains(t, w.Body.String(), "UNSUPPORTED_MEDIA_TYPE")
	})
	
	t.Run("SQL injection protection", func(t *testing.T) {
		// Test with potential SQL injection in query parameters
		// Properly construct URL with encoded parameters
		testURL := "/api/test?search=" + url.QueryEscape("'; DROP TABLE users; --")
		req := httptest.NewRequest(http.MethodGet, testURL, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_QUERY_PARAMS")
	})
	
	t.Run("CORS protection", func(t *testing.T) {
		// Test with disallowed origin
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Origin", "http://malicious.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Origin should not be reflected for disallowed origins
		assert.NotEqual(t, "http://malicious.com", w.Header().Get("Access-Control-Allow-Origin"))
	})
	
	t.Run("Request size protection", func(t *testing.T) {
		// Create a router with size limit
		sizeLimitRouter := gin.New()
		sizeLimitRouter.Use(securityMiddleware.RequestSizeLimitMiddleware(100))
		sizeLimitRouter.POST("/api/data", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		
		// Test with oversized request
		largeData := strings.Repeat("a", 200)
		req := httptest.NewRequest(http.MethodPost, "/api/data", strings.NewReader(largeData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		sizeLimitRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Contains(t, w.Body.String(), "REQUEST_TOO_LARGE")
	})
}

func TestSecurityPerformance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	securityConfig := &config.SecurityConfig{
		RateLimit:       1000,
		RateLimitWindow: time.Minute,
	}
	
	testLogger := logger.NewDefault()
	securityMiddleware := NewSecurityMiddleware(securityConfig, testLogger, nil)
	
	router := gin.New()
	router.Use(securityMiddleware.SecurityHeadersMiddleware())
	router.Use(securityMiddleware.CORSMiddleware())
	router.Use(securityMiddleware.InputValidationMiddleware())
	
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Test performance impact
	start := time.Now()
	
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
	
	duration := time.Since(start)
	
	// Security middleware should have minimal performance impact
	assert.True(t, duration < time.Second, "Security middleware should process 100 requests in under 1 second")
	
	avgPerRequest := duration / 100
	assert.True(t, avgPerRequest < 10*time.Millisecond, "Average request processing time should be under 10ms")
}

func TestSecurityWithDifferentEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	securityConfig := &config.SecurityConfig{
		RateLimit:       100,
		RateLimitWindow: time.Minute,
	}
	
	testLogger := logger.NewDefault()
	securityMiddleware := NewSecurityMiddleware(securityConfig, testLogger, nil)
	
	// Create different rate limits for different endpoints
	publicLimiter := NewRateLimitMiddleware(IPBasedRateLimitConfig(60), nil, testLogger)
	authLimiter := NewRateLimitMiddleware(UserBasedRateLimitConfig(1000), nil, testLogger)
	
	router := gin.New()
	router.Use(securityMiddleware.SecurityHeadersMiddleware())
	router.Use(securityMiddleware.CORSMiddleware())
	router.Use(securityMiddleware.InputValidationMiddleware())
	
	// Public routes with lower rate limits
	public := router.Group("/api/public")
	public.Use(publicLimiter.Middleware())
	public.GET("/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public info"})
	})
	
	// Authenticated routes with higher rate limits
	auth := router.Group("/api/auth")
	auth.Use(authLimiter.Middleware())
	auth.GET("/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "user profile"})
	})
	
	// Test public endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/public/info", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
	
	// Test authenticated endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
}

func TestSecurityErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	securityConfig := &config.SecurityConfig{
		RateLimit:       1,
		RateLimitWindow: time.Minute,
	}
	
	testLogger := logger.NewDefault()
	securityMiddleware := NewSecurityMiddleware(securityConfig, testLogger, nil)
	
	router := gin.New()
	router.Use(securityMiddleware.SecurityHeadersMiddleware())
	router.Use(securityMiddleware.InputValidationMiddleware())
	
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	tests := []struct {
		name           string
		contentType    string
		body           string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid content type",
			contentType:    "application/xml",
			body:           "<data>test</data>",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "UNSUPPORTED_MEDIA_TYPE",
		},
		{
			name:           "Valid JSON",
			contentType:    "application/json",
			body:           `{"test": "data"}`,
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedError, response["error"])
			}
		})
	}
}