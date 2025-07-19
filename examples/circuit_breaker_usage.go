package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"gorm.io/gorm"
)

// CircuitBreakerUsageExample demonstrates how to use circuit breakers in the application
type CircuitBreakerUsageExample struct {
	db     *gorm.DB
	redis  *redis.Client
	logger *logger.Logger
}

// NewCircuitBreakerUsageExample creates a new usage example
func NewCircuitBreakerUsageExample(db *gorm.DB, redis *redis.Client, logger *logger.Logger) *CircuitBreakerUsageExample {
	return &CircuitBreakerUsageExample{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// SetupCircuitBreakers demonstrates how to set up circuit breakers
func (e *CircuitBreakerUsageExample) SetupCircuitBreakers() *infrastructure.CircuitBreakerIntegration {
	// Create circuit breaker integration
	integration := infrastructure.NewCircuitBreakerIntegration(e.logger)
	
	// Setup health checks for database and cache
	integration.SetupHealthChecks(e.db, e.redis)
	
	e.logger.Info("Circuit breakers initialized successfully")
	
	return integration
}

// SetupHTTPMiddleware demonstrates how to set up HTTP middleware with circuit breakers
func (e *CircuitBreakerUsageExample) SetupHTTPMiddleware(integration *infrastructure.CircuitBreakerIntegration) *gin.Engine {
	// Create Gin router
	router := gin.Default()
	
	// Create circuit breaker middleware
	cbMiddleware := middleware.NewCircuitBreakerMiddleware(integration.GetManager(), e.logger)
	
	// Create health check handler
	healthCheck := middleware.NewCircuitBreakerHealthCheck(integration.GetManager(), e.logger)
	
	// Add global middleware for database operations
	router.Use(cbMiddleware.HTTPMiddleware("database"))
	
	// Health check endpoints
	router.GET("/health", healthCheck.HealthCheckHandler())
	router.GET("/metrics", healthCheck.MetricsHandler())
	
	// Example protected routes
	v1 := router.Group("/api/v1")
	{
		// This route will be protected by the database circuit breaker
		v1.GET("/reviews", e.getReviewsHandler(integration))
		
		// This route will be protected by cache circuit breaker
		v1.GET("/reviews/:id", cbMiddleware.HTTPMiddleware("cache"), e.getReviewByIDHandler(integration))
		
		// This route will be protected by S3 circuit breaker
		v1.POST("/reviews/upload", cbMiddleware.HTTPMiddleware("s3"), e.uploadReviewsHandler(integration))
	}
	
	return router
}

// getReviewsHandler demonstrates database operations with circuit breaker
func (e *CircuitBreakerUsageExample) getReviewsHandler(integration *infrastructure.CircuitBreakerIntegration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create database wrapper with circuit breaker
		dbWrapper := integration.NewDatabaseWrapper(e.db)
		
		// Execute database query with circuit breaker protection
		var reviews []map[string]interface{}
		err := dbWrapper.ExecuteQuery(c.Request.Context(), func(db *gorm.DB) error {
			// Simulate database query
			return db.Raw("SELECT id, hotel_id, rating, comment FROM reviews LIMIT 10").Scan(&reviews).Error
		})
		
		if err != nil {
			// Check if it's a circuit breaker error
			if _, ok := err.(*infrastructure.DatabaseCircuitBreakerError); ok {
				c.JSON(503, gin.H{
					"error":   "Database service temporarily unavailable",
					"message": "Please try again later",
				})
				return
			}
			
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		
		c.JSON(200, gin.H{
			"reviews": reviews,
			"count":   len(reviews),
		})
	}
}

// getReviewByIDHandler demonstrates cache operations with circuit breaker
func (e *CircuitBreakerUsageExample) getReviewByIDHandler(integration *infrastructure.CircuitBreakerIntegration) gin.HandlerFunc {
	return func(c *gin.Context) {
		reviewID := c.Param("id")
		
		// Create cache wrapper with circuit breaker
		cacheWrapper := integration.NewCacheWrapper(e.redis)
		
		// Try to get from cache first
		cachedReview, err := cacheWrapper.Get(c.Request.Context(), fmt.Sprintf("review:%s", reviewID))
		
		if err == nil && cachedReview != "" {
			c.JSON(200, gin.H{
				"review": cachedReview,
				"source": "cache",
			})
			return
		}
		
		// If cache fails or miss, get from database
		dbWrapper := integration.NewDatabaseWrapper(e.db)
		
		var review map[string]interface{}
		err = dbWrapper.ExecuteQuery(c.Request.Context(), func(db *gorm.DB) error {
			return db.Raw("SELECT id, hotel_id, rating, comment FROM reviews WHERE id = ?", reviewID).Scan(&review).Error
		})
		
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		
		// Try to cache the result (if cache is available)
		if integration.IsCacheAvailable() {
			cacheWrapper.Set(c.Request.Context(), fmt.Sprintf("review:%s", reviewID), review, time.Hour)
		}
		
		c.JSON(200, gin.H{
			"review": review,
			"source": "database",
		})
	}
}

// uploadReviewsHandler demonstrates S3 operations with circuit breaker
func (e *CircuitBreakerUsageExample) uploadReviewsHandler(integration *infrastructure.CircuitBreakerIntegration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create S3 wrapper with circuit breaker
		s3Wrapper := integration.NewS3Wrapper()
		
		// Simulate S3 upload operation
		result, err := s3Wrapper.ExecuteS3Operation(c.Request.Context(), func(ctx context.Context) (interface{}, error) {
			// Simulate S3 upload
			time.Sleep(100 * time.Millisecond)
			
			// For demo purposes, we'll return a simulated result
			return map[string]interface{}{
				"bucket":    "hotel-reviews-bucket",
				"key":       "uploads/reviews-batch-" + fmt.Sprintf("%d", time.Now().Unix()),
				"upload_id": "upload-123456",
				"size":      1024,
			}, nil
		})
		
		if err != nil {
			// Check if it's a circuit breaker error
			if _, ok := err.(*infrastructure.S3CircuitBreakerError); ok {
				c.JSON(503, gin.H{
					"error":   "File upload service temporarily unavailable",
					"message": "Please try again later",
				})
				return
			}
			
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		
		c.JSON(200, gin.H{
			"message": "File uploaded successfully",
			"result":  result,
		})
	}
}

// DemoCircuitBreakerStates demonstrates different circuit breaker states
func (e *CircuitBreakerUsageExample) DemoCircuitBreakerStates(integration *infrastructure.CircuitBreakerIntegration) {
	dbBreaker := integration.GetDatabaseBreaker()
	
	e.logger.Info("=== Circuit Breaker States Demo ===")
	
	// 1. Closed State (Normal operation)
	e.logger.Info("1. Testing CLOSED state (normal operation)")
	ctx := context.Background()
	
	// Simulate successful operations
	for i := 0; i < 5; i++ {
		_, err := dbBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			time.Sleep(10 * time.Millisecond)
			return "success", nil
		})
		
		if err != nil {
			e.logger.Error("Unexpected error in closed state", "error", err)
		} else {
			e.logger.Info("Successful operation", "attempt", i+1)
		}
	}
	
	e.logger.Info("Database circuit breaker state", "state", dbBreaker.GetState())
	
	// 2. Open State (Circuit breaker trips)
	e.logger.Info("2. Testing OPEN state (simulating failures)")
	
	// Simulate consecutive failures to trip the circuit breaker
	for i := 0; i < 5; i++ {
		_, err := dbBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return nil, fmt.Errorf("simulated database error %d", i+1)
		})
		
		if err != nil {
			e.logger.Info("Failed operation (expected)", "attempt", i+1, "error", err)
		}
	}
	
	e.logger.Info("Database circuit breaker state after failures", "state", dbBreaker.GetState())
	
	// 3. Verify circuit breaker is open
	e.logger.Info("3. Testing requests while circuit breaker is OPEN")
	
	_, err := dbBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return "should not execute", nil
	})
	
	if err != nil {
		e.logger.Info("Request rejected by circuit breaker (expected)", "error", err)
	}
	
	// 4. Wait for half-open state
	e.logger.Info("4. Waiting for HALF_OPEN state transition...")
	
	// Force transition to half-open for demo
	dbBreaker.ForceHalfOpen()
	e.logger.Info("Circuit breaker state", "state", dbBreaker.GetState())
	
	// 5. Test half-open state
	e.logger.Info("5. Testing HALF_OPEN state")
	
	// Simulate successful operation to close the circuit
	_, err = dbBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return "recovery success", nil
	})
	
	if err != nil {
		e.logger.Error("Unexpected error in half-open state", "error", err)
	} else {
		e.logger.Info("Successful operation in half-open state")
	}
	
	e.logger.Info("Final circuit breaker state", "state", dbBreaker.GetState())
	
	// 6. Show metrics
	e.logger.Info("6. Circuit breaker metrics")
	metrics := dbBreaker.GetMetrics()
	e.logger.Info("Circuit breaker metrics",
		"total_requests", metrics.TotalRequests,
		"total_successes", metrics.TotalSuccesses,
		"total_failures", metrics.TotalFailures,
		"success_rate", fmt.Sprintf("%.2f%%", metrics.SuccessRate),
		"failure_rate", fmt.Sprintf("%.2f%%", metrics.FailureRate),
		"state_transitions", metrics.StateTransitions,
	)
}

// DemoFallbackMechanisms demonstrates fallback mechanisms
func (e *CircuitBreakerUsageExample) DemoFallbackMechanisms(integration *infrastructure.CircuitBreakerIntegration) {
	e.logger.Info("=== Fallback Mechanisms Demo ===")
	
	// Setup a circuit breaker with a custom fallback
	breaker := infrastructure.NewCircuitBreaker(
		&infrastructure.CircuitBreakerConfig{
			Name:                "demo-fallback",
			FailureThreshold:    2,
			SuccessThreshold:    1,
			ConsecutiveFailures: 2,
			MinimumRequestCount: 1,
			RequestTimeout:      1 * time.Second,
			OpenTimeout:         5 * time.Second,
			EnableFallback:      true,
			EnableLogging:       true,
		},
		e.logger,
	)
	defer breaker.Close()
	
	// Set up a fallback function
	fallbackFunc := func(ctx context.Context, err error) (interface{}, error) {
		e.logger.Info("Fallback function executed", "original_error", err)
		return map[string]interface{}{
			"message": "Fallback response",
			"data":    "cached or default data",
			"source":  "fallback",
		}, nil
	}
	
	breaker.SetFallback(fallbackFunc)
	
	ctx := context.Background()
	
	// 1. Cause circuit breaker to open
	e.logger.Info("1. Causing circuit breaker to open")
	for i := 0; i < 3; i++ {
		_, err := breaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return nil, fmt.Errorf("simulated failure %d", i+1)
		})
		e.logger.Info("Operation result", "attempt", i+1, "error", err)
	}
	
	// 2. Test fallback when circuit is open
	e.logger.Info("2. Testing fallback when circuit is open")
	result, err := breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		return "should not execute", nil
	}, fallbackFunc)
	
	if err != nil {
		e.logger.Error("Unexpected error with fallback", "error", err)
	} else {
		e.logger.Info("Fallback executed successfully", "result", result)
	}
	
	e.logger.Info("Circuit breaker state", "state", breaker.GetState())
}

// DemoHealthChecks demonstrates health check functionality
func (e *CircuitBreakerUsageExample) DemoHealthChecks(integration *infrastructure.CircuitBreakerIntegration) {
	e.logger.Info("=== Health Checks Demo ===")
	
	// Get health status
	healthStatus := integration.GetHealthStatus()
	e.logger.Info("Current health status", "status", healthStatus)
	
	// Test individual service availability
	e.logger.Info("Service availability check",
		"database", integration.IsDatabaseAvailable(),
		"cache", integration.IsCacheAvailable(),
		"s3", integration.IsS3Available(),
	)
	
	// Force a circuit breaker open to test health status
	integration.GetDatabaseBreaker().ForceOpen()
	
	e.logger.Info("Health status after forcing database circuit breaker open")
	healthStatus = integration.GetHealthStatus()
	e.logger.Info("Updated health status", "status", healthStatus)
	
	// Reset circuit breaker
	integration.GetDatabaseBreaker().Reset()
	e.logger.Info("Database circuit breaker reset")
}

// RunFullDemo runs a complete demonstration of circuit breaker functionality
func (e *CircuitBreakerUsageExample) RunFullDemo() {
	e.logger.Info("Starting Circuit Breaker Full Demo")
	
	// Setup circuit breakers
	integration := e.SetupCircuitBreakers()
	defer integration.Close()
	
	// Demo different states
	e.DemoCircuitBreakerStates(integration)
	
	// Demo fallback mechanisms
	e.DemoFallbackMechanisms(integration)
	
	// Demo health checks
	e.DemoHealthChecks(integration)
	
	e.logger.Info("Circuit Breaker Demo completed")
}

// BenchmarkCircuitBreaker demonstrates performance characteristics
func (e *CircuitBreakerUsageExample) BenchmarkCircuitBreaker(integration *infrastructure.CircuitBreakerIntegration) {
	e.logger.Info("=== Circuit Breaker Benchmark ===")
	
	breaker := integration.GetDatabaseBreaker()
	ctx := context.Background()
	
	// Benchmark normal operations
	start := time.Now()
	successCount := 0
	totalOps := 1000
	
	for i := 0; i < totalOps; i++ {
		_, err := breaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			// Simulate fast operation
			time.Sleep(1 * time.Millisecond)
			return "success", nil
		})
		
		if err == nil {
			successCount++
		}
	}
	
	duration := time.Since(start)
	opsPerSecond := float64(totalOps) / duration.Seconds()
	
	e.logger.Info("Benchmark results",
		"total_operations", totalOps,
		"successful_operations", successCount,
		"duration", duration,
		"ops_per_second", fmt.Sprintf("%.2f", opsPerSecond),
	)
	
	metrics := breaker.GetMetrics()
	e.logger.Info("Final metrics",
		"total_requests", metrics.TotalRequests,
		"success_rate", fmt.Sprintf("%.2f%%", metrics.SuccessRate),
		"average_response_time", metrics.AverageResponseTime,
	)
}