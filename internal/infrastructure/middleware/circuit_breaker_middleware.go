package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// CircuitBreakerMiddleware provides HTTP middleware for circuit breaker protection
type CircuitBreakerMiddleware struct {
	manager *infrastructure.CircuitBreakerManager
	logger  *logger.Logger
}

// NewCircuitBreakerMiddleware creates a new circuit breaker middleware
func NewCircuitBreakerMiddleware(manager *infrastructure.CircuitBreakerManager, logger *logger.Logger) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		manager: manager,
		logger:  logger,
	}
}

// HTTPMiddleware returns a Gin middleware that protects HTTP handlers with circuit breaker
func (m *CircuitBreakerMiddleware) HTTPMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		breaker, exists := m.manager.GetCircuitBreaker(serviceName)
		if !exists {
			m.logger.Warn("Circuit breaker not found for service", "service", serviceName)
			c.Next()
			return
		}

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Execute the handler with circuit breaker protection
		result, err := breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
			// Create a new context for the handler
			c.Request = c.Request.WithContext(ctx)

			// Execute the handler
			c.Next()

			// Check if there was an error (status >= 500)
			if c.Writer.Status() >= 500 {
				return nil, &HTTPError{
					StatusCode: c.Writer.Status(),
					Message:    "HTTP error occurred",
				}
			}

			return nil, nil
		}, m.createHTTPFallback(c, serviceName))

		// Handle circuit breaker errors
		if err != nil {
			if infrastructure.IsCircuitBreakerError(err) {
				m.logger.Warn("Circuit breaker rejected request",
					"service", serviceName,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"error", err,
				)

				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error":   "Service temporarily unavailable",
					"service": serviceName,
					"code":    "CIRCUIT_BREAKER_OPEN",
				})
				c.Abort()
				return
			}

			// If fallback was executed, the response might already be set
			if result != nil {
				m.logger.Info("Circuit breaker fallback executed",
					"service", serviceName,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
			}
		}
	}
}

// createHTTPFallback creates a fallback function for HTTP requests
func (m *CircuitBreakerMiddleware) createHTTPFallback(c *gin.Context, serviceName string) infrastructure.FallbackFunc {
	return func(ctx context.Context, err error) (interface{}, error) {
		m.logger.Info("Executing HTTP fallback",
			"service", serviceName,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"original_error", err,
		)

		// Provide a generic fallback response
		switch c.Request.URL.Path {
		case "/health":
			c.JSON(http.StatusOK, gin.H{
				"status":  "degraded",
				"service": serviceName,
				"message": "Service is experiencing issues",
			})
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Service temporarily unavailable",
				"service": serviceName,
				"code":    "FALLBACK_RESPONSE",
				"message": "Please try again later",
			})
		}

		return "fallback_executed", nil
	}
}

// HTTPError represents an HTTP error
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// DatabaseMiddleware provides database operation protection with circuit breaker
type DatabaseMiddleware struct {
	breaker *infrastructure.CircuitBreaker
	logger  *logger.Logger
}

// NewDatabaseMiddleware creates a new database middleware
func NewDatabaseMiddleware(breaker *infrastructure.CircuitBreaker, logger *logger.Logger) *DatabaseMiddleware {
	return &DatabaseMiddleware{
		breaker: breaker,
		logger:  logger,
	}
}

// ExecuteQuery executes a database query with circuit breaker protection
func (m *DatabaseMiddleware) ExecuteQuery(ctx context.Context, queryFunc func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	return m.breaker.ExecuteWithFallback(ctx, queryFunc, m.createDBFallback())
}

// createDBFallback creates a fallback function for database operations
func (m *DatabaseMiddleware) createDBFallback() infrastructure.FallbackFunc {
	return func(ctx context.Context, err error) (interface{}, error) {
		m.logger.Warn("Database circuit breaker fallback executed", "error", err)

		// Return cached data or default values
		// This is a placeholder - in real implementation, you would
		// return cached data or appropriate fallback response
		return nil, &DatabaseFallbackError{
			OriginalError: err,
			Message:       "Database temporarily unavailable, using fallback",
		}
	}
}

// DatabaseFallbackError represents a database fallback error
type DatabaseFallbackError struct {
	OriginalError error
	Message       string
}

func (e *DatabaseFallbackError) Error() string {
	return e.Message
}

// CacheMiddleware provides cache operation protection with circuit breaker
type CacheMiddleware struct {
	breaker *infrastructure.CircuitBreaker
	logger  *logger.Logger
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(breaker *infrastructure.CircuitBreaker, logger *logger.Logger) *CacheMiddleware {
	return &CacheMiddleware{
		breaker: breaker,
		logger:  logger,
	}
}

// ExecuteCache executes a cache operation with circuit breaker protection
func (m *CacheMiddleware) ExecuteCache(ctx context.Context, cacheFunc func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	return m.breaker.ExecuteWithFallback(ctx, cacheFunc, m.createCacheFallback())
}

// createCacheFallback creates a fallback function for cache operations
func (m *CacheMiddleware) createCacheFallback() infrastructure.FallbackFunc {
	return func(ctx context.Context, err error) (interface{}, error) {
		m.logger.Info("Cache circuit breaker fallback executed", "error", err)

		// For cache operations, fallback usually means proceed without cache
		return nil, nil
	}
}

// S3Middleware provides S3 operation protection with circuit breaker
type S3Middleware struct {
	breaker *infrastructure.CircuitBreaker
	logger  *logger.Logger
}

// NewS3Middleware creates a new S3 middleware
func NewS3Middleware(breaker *infrastructure.CircuitBreaker, logger *logger.Logger) *S3Middleware {
	return &S3Middleware{
		breaker: breaker,
		logger:  logger,
	}
}

// ExecuteS3Operation executes an S3 operation with circuit breaker protection
func (m *S3Middleware) ExecuteS3Operation(ctx context.Context, s3Func func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	return m.breaker.ExecuteWithFallback(ctx, s3Func, m.createS3Fallback())
}

// createS3Fallback creates a fallback function for S3 operations
func (m *S3Middleware) createS3Fallback() infrastructure.FallbackFunc {
	return func(ctx context.Context, err error) (interface{}, error) {
		m.logger.Warn("S3 circuit breaker fallback executed", "error", err)

		// For S3 operations, fallback might mean using local storage
		// or returning an error indicating the operation failed
		return nil, &S3FallbackError{
			OriginalError: err,
			Message:       "S3 service temporarily unavailable",
		}
	}
}

// S3FallbackError represents an S3 fallback error
type S3FallbackError struct {
	OriginalError error
	Message       string
}

func (e *S3FallbackError) Error() string {
	return e.Message
}

// CircuitBreakerHealthCheck provides health check endpoints for circuit breakers
type CircuitBreakerHealthCheck struct {
	manager *infrastructure.CircuitBreakerManager
	logger  *logger.Logger
}

// NewCircuitBreakerHealthCheck creates a new circuit breaker health check
func NewCircuitBreakerHealthCheck(manager *infrastructure.CircuitBreakerManager, logger *logger.Logger) *CircuitBreakerHealthCheck {
	return &CircuitBreakerHealthCheck{
		manager: manager,
		logger:  logger,
	}
}

// HealthCheckHandler returns a Gin handler for circuit breaker health checks
func (h *CircuitBreakerHealthCheck) HealthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		breakers := h.manager.GetAllCircuitBreakers()

		status := "healthy"
		statusCode := http.StatusOK
		breakerStatus := make(map[string]interface{})

		for name, breaker := range breakers {
			metrics := breaker.GetMetrics()

			breakerInfo := gin.H{
				"state":          metrics.CurrentState.String(),
				"total_requests": metrics.TotalRequests,
				"success_rate":   metrics.SuccessRate,
				"failure_rate":   metrics.FailureRate,
				"last_failure":   metrics.LastFailure,
				"last_success":   metrics.LastSuccess,
			}

			breakerStatus[name] = breakerInfo

			// If any breaker is open, mark overall status as degraded
			if breaker.IsOpen() {
				status = "degraded"
				statusCode = http.StatusPartialContent
			}
		}

		c.JSON(statusCode, gin.H{
			"status":           status,
			"circuit_breakers": breakerStatus,
			"timestamp":        time.Now().UTC(),
		})
	}
}

// MetricsHandler returns a Gin handler for circuit breaker metrics
func (h *CircuitBreakerHealthCheck) MetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		metrics := h.manager.GetMetrics()

		c.JSON(http.StatusOK, gin.H{
			"metrics":   metrics,
			"timestamp": time.Now().UTC(),
		})
	}
}

// CircuitBreakerConfig provides configuration for circuit breaker middleware
type CircuitBreakerConfig struct {
	Enabled      bool                                            `json:"enabled"`
	Services     map[string]*infrastructure.CircuitBreakerConfig `json:"services"`
	GlobalConfig *infrastructure.CircuitBreakerConfig            `json:"global_config"`
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Enabled: true,
		Services: map[string]*infrastructure.CircuitBreakerConfig{
			"database": {
				Name:                "database",
				FailureThreshold:    3,
				SuccessThreshold:    2,
				ConsecutiveFailures: 2,
				MinimumRequestCount: 5,
				RequestTimeout:      10 * time.Second,
				OpenTimeout:         30 * time.Second,
				HalfOpenTimeout:     15 * time.Second,
				WindowSize:          30 * time.Second,
				WindowBuckets:       6,
				MaxRetries:          2,
				RetryDelay:          500 * time.Millisecond,
				RetryBackoffFactor:  2.0,
				FailFast:            true,
				EnableFallback:      true,
				EnableMetrics:       true,
				EnableLogging:       true,
				HealthCheckInterval: 15 * time.Second,
				HealthCheckTimeout:  3 * time.Second,
				EnableHealthCheck:   true,
			},
			"cache": {
				Name:                "cache",
				FailureThreshold:    10,
				SuccessThreshold:    5,
				ConsecutiveFailures: 5,
				MinimumRequestCount: 20,
				RequestTimeout:      5 * time.Second,
				OpenTimeout:         15 * time.Second,
				HalfOpenTimeout:     10 * time.Second,
				WindowSize:          30 * time.Second,
				WindowBuckets:       6,
				MaxRetries:          1,
				RetryDelay:          100 * time.Millisecond,
				RetryBackoffFactor:  1.5,
				FailFast:            false,
				EnableFallback:      true,
				EnableMetrics:       true,
				EnableLogging:       true,
				HealthCheckInterval: 10 * time.Second,
				HealthCheckTimeout:  2 * time.Second,
				EnableHealthCheck:   true,
			},
			"s3": {
				Name:                "s3",
				FailureThreshold:    5,
				SuccessThreshold:    3,
				ConsecutiveFailures: 3,
				MinimumRequestCount: 10,
				RequestTimeout:      30 * time.Second,
				OpenTimeout:         60 * time.Second,
				HalfOpenTimeout:     30 * time.Second,
				WindowSize:          60 * time.Second,
				WindowBuckets:       10,
				MaxRetries:          3,
				RetryDelay:          1 * time.Second,
				RetryBackoffFactor:  2.0,
				FailFast:            true,
				EnableFallback:      true,
				EnableMetrics:       true,
				EnableLogging:       true,
				HealthCheckInterval: 30 * time.Second,
				HealthCheckTimeout:  5 * time.Second,
				EnableHealthCheck:   false,
			},
		},
		GlobalConfig: infrastructure.DefaultCircuitBreakerConfig(),
	}
}
