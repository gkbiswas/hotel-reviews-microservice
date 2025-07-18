package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"gorm.io/gorm"
)

// CircuitBreakerIntegration provides integration with circuit breakers for various services
type CircuitBreakerIntegration struct {
	manager      *CircuitBreakerManager
	dbBreaker    *CircuitBreaker
	cacheBreaker *CircuitBreaker
	s3Breaker    *CircuitBreaker
	logger       *logger.Logger
}

// NewCircuitBreakerIntegration creates a new circuit breaker integration
func NewCircuitBreakerIntegration(logger *logger.Logger) *CircuitBreakerIntegration {
	manager := NewCircuitBreakerManager(logger)
	
	// Create service-specific circuit breakers
	dbBreaker := NewDatabaseCircuitBreaker(logger)
	cacheBreaker := NewCacheCircuitBreaker(logger)
	s3Breaker := NewS3CircuitBreaker(logger)
	
	// Add circuit breakers to manager
	manager.AddCircuitBreaker("database", dbBreaker)
	manager.AddCircuitBreaker("cache", cacheBreaker)
	manager.AddCircuitBreaker("s3", s3Breaker)
	
	return &CircuitBreakerIntegration{
		manager:      manager,
		dbBreaker:    dbBreaker,
		cacheBreaker: cacheBreaker,
		s3Breaker:    s3Breaker,
		logger:       logger,
	}
}

// SetupHealthChecks sets up health check functions for all circuit breakers
func (i *CircuitBreakerIntegration) SetupHealthChecks(db *gorm.DB, redisClient *redis.Client) {
	// Database health check
	if db != nil {
		i.dbBreaker.SetHealthCheck(func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			
			return sqlDB.PingContext(ctx)
		})
	}
	
	// Cache health check
	if redisClient != nil {
		i.cacheBreaker.SetHealthCheck(func(ctx context.Context) error {
			return redisClient.Ping(ctx).Err()
		})
	}
	
	// S3 health check is not set up as it doesn't have a simple ping operation
}

// DatabaseWrapper wraps database operations with circuit breaker protection
type DatabaseWrapper struct {
	db      *gorm.DB
	breaker *CircuitBreaker
	logger  *logger.Logger
}

// NewDatabaseWrapper creates a new database wrapper with circuit breaker
func (i *CircuitBreakerIntegration) NewDatabaseWrapper(db *gorm.DB) *DatabaseWrapper {
	return &DatabaseWrapper{
		db:      db,
		breaker: i.dbBreaker,
		logger:  i.logger,
	}
}

// ExecuteQuery executes a database query with circuit breaker protection
func (w *DatabaseWrapper) ExecuteQuery(ctx context.Context, queryFunc func(*gorm.DB) error) error {
	_, err := w.breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		// Add context to the database session
		dbWithCtx := w.db.WithContext(ctx)
		
		// Execute the query
		return nil, queryFunc(dbWithCtx)
	}, w.createDatabaseFallback())
	
	return err
}

// ExecuteTransaction executes a database transaction with circuit breaker protection
func (w *DatabaseWrapper) ExecuteTransaction(ctx context.Context, txFunc func(*gorm.DB) error) error {
	_, err := w.breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		// Start transaction with context
		tx := w.db.WithContext(ctx).Begin()
		if tx.Error != nil {
			return nil, tx.Error
		}
		
		// Execute transaction function
		if err := txFunc(tx); err != nil {
			tx.Rollback()
			return nil, err
		}
		
		// Commit transaction
		return nil, tx.Commit().Error
	}, w.createDatabaseFallback())
	
	return err
}

// FindWithFallback executes a find query with circuit breaker protection
func (w *DatabaseWrapper) FindWithFallback(ctx context.Context, dest interface{}, where ...interface{}) error {
	return w.ExecuteQuery(ctx, func(db *gorm.DB) error {
		return db.Find(dest, where...).Error
	})
}

// CreateWithFallback executes a create operation with circuit breaker protection
func (w *DatabaseWrapper) CreateWithFallback(ctx context.Context, value interface{}) error {
	return w.ExecuteQuery(ctx, func(db *gorm.DB) error {
		return db.Create(value).Error
	})
}

// UpdateWithFallback executes an update operation with circuit breaker protection
func (w *DatabaseWrapper) UpdateWithFallback(ctx context.Context, value interface{}) error {
	return w.ExecuteQuery(ctx, func(db *gorm.DB) error {
		return db.Save(value).Error
	})
}

// DeleteWithFallback executes a delete operation with circuit breaker protection
func (w *DatabaseWrapper) DeleteWithFallback(ctx context.Context, value interface{}, where ...interface{}) error {
	return w.ExecuteQuery(ctx, func(db *gorm.DB) error {
		return db.Delete(value, where...).Error
	})
}

// createDatabaseFallback creates a fallback function for database operations
func (w *DatabaseWrapper) createDatabaseFallback() FallbackFunc {
	return func(ctx context.Context, err error) (interface{}, error) {
		w.logger.Warn("Database operation failed, using fallback",
			"error", err,
			"circuit_state", w.breaker.GetState(),
		)
		
		// In a real application, this might return cached data or default values
		// For now, we'll return a specific error indicating fallback was used
		return nil, &DatabaseCircuitBreakerError{
			OriginalError: err,
			Message:       "Database circuit breaker is open, operation failed",
		}
	}
}

// CacheWrapper wraps cache operations with circuit breaker protection
type CacheWrapper struct {
	client  *redis.Client
	breaker *CircuitBreaker
	logger  *logger.Logger
}

// NewCacheWrapper creates a new cache wrapper with circuit breaker
func (i *CircuitBreakerIntegration) NewCacheWrapper(client *redis.Client) *CacheWrapper {
	return &CacheWrapper{
		client:  client,
		breaker: i.cacheBreaker,
		logger:  i.logger,
	}
}

// Get executes a cache get operation with circuit breaker protection
func (w *CacheWrapper) Get(ctx context.Context, key string) (string, error) {
	result, err := w.breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		return w.client.Get(ctx, key).Result()
	}, w.createCacheFallback())
	
	if err != nil {
		return "", err
	}
	
	if result == nil {
		return "", redis.Nil
	}
	
	return result.(string), nil
}

// Set executes a cache set operation with circuit breaker protection
func (w *CacheWrapper) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	_, err := w.breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, w.client.Set(ctx, key, value, expiration).Err()
	}, w.createCacheFallback())
	
	return err
}

// Del executes a cache delete operation with circuit breaker protection
func (w *CacheWrapper) Del(ctx context.Context, keys ...string) error {
	_, err := w.breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, w.client.Del(ctx, keys...).Err()
	}, w.createCacheFallback())
	
	return err
}

// HGet executes a hash get operation with circuit breaker protection
func (w *CacheWrapper) HGet(ctx context.Context, key, field string) (string, error) {
	result, err := w.breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		return w.client.HGet(ctx, key, field).Result()
	}, w.createCacheFallback())
	
	if err != nil {
		return "", err
	}
	
	if result == nil {
		return "", redis.Nil
	}
	
	return result.(string), nil
}

// HSet executes a hash set operation with circuit breaker protection
func (w *CacheWrapper) HSet(ctx context.Context, key string, values ...interface{}) error {
	_, err := w.breaker.ExecuteWithFallback(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, w.client.HSet(ctx, key, values...).Err()
	}, w.createCacheFallback())
	
	return err
}

// createCacheFallback creates a fallback function for cache operations
func (w *CacheWrapper) createCacheFallback() FallbackFunc {
	return func(ctx context.Context, err error) (interface{}, error) {
		w.logger.Info("Cache operation failed, continuing without cache",
			"error", err,
			"circuit_state", w.breaker.GetState(),
		)
		
		// For cache operations, we typically continue without cache
		// Return nil to indicate cache miss
		return nil, nil
	}
}

// S3Wrapper wraps S3 operations with circuit breaker protection
type S3Wrapper struct {
	breaker *CircuitBreaker
	logger  *logger.Logger
}

// NewS3Wrapper creates a new S3 wrapper with circuit breaker
func (i *CircuitBreakerIntegration) NewS3Wrapper() *S3Wrapper {
	return &S3Wrapper{
		breaker: i.s3Breaker,
		logger:  i.logger,
	}
}

// ExecuteS3Operation executes an S3 operation with circuit breaker protection
func (w *S3Wrapper) ExecuteS3Operation(ctx context.Context, operation func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	return w.breaker.ExecuteWithFallback(ctx, operation, w.createS3Fallback())
}

// createS3Fallback creates a fallback function for S3 operations
func (w *S3Wrapper) createS3Fallback() FallbackFunc {
	return func(ctx context.Context, err error) (interface{}, error) {
		w.logger.Warn("S3 operation failed",
			"error", err,
			"circuit_state", w.breaker.GetState(),
		)
		
		return nil, &S3CircuitBreakerError{
			OriginalError: err,
			Message:       "S3 service is unavailable",
		}
	}
}

// Custom error types for circuit breaker fallbacks

// DatabaseCircuitBreakerError represents a database circuit breaker error
type DatabaseCircuitBreakerError struct {
	OriginalError error
	Message       string
}

func (e *DatabaseCircuitBreakerError) Error() string {
	return fmt.Sprintf("database circuit breaker: %s (original: %v)", e.Message, e.OriginalError)
}

// S3CircuitBreakerError represents an S3 circuit breaker error
type S3CircuitBreakerError struct {
	OriginalError error
	Message       string
}

func (e *S3CircuitBreakerError) Error() string {
	return fmt.Sprintf("s3 circuit breaker: %s (original: %v)", e.Message, e.OriginalError)
}

// GetManager returns the circuit breaker manager
func (i *CircuitBreakerIntegration) GetManager() *CircuitBreakerManager {
	return i.manager
}

// GetDatabaseBreaker returns the database circuit breaker
func (i *CircuitBreakerIntegration) GetDatabaseBreaker() *CircuitBreaker {
	return i.dbBreaker
}

// GetCacheBreaker returns the cache circuit breaker
func (i *CircuitBreakerIntegration) GetCacheBreaker() *CircuitBreaker {
	return i.cacheBreaker
}

// GetS3Breaker returns the S3 circuit breaker
func (i *CircuitBreakerIntegration) GetS3Breaker() *CircuitBreaker {
	return i.s3Breaker
}

// Close closes all circuit breakers
func (i *CircuitBreakerIntegration) Close() {
	i.manager.Close()
}

// GetHealthStatus returns the health status of all circuit breakers
func (i *CircuitBreakerIntegration) GetHealthStatus() map[string]interface{} {
	status := make(map[string]interface{})
	
	breakers := i.manager.GetAllCircuitBreakers()
	overallHealthy := true
	
	for name, breaker := range breakers {
		metrics := breaker.GetMetrics()
		
		isHealthy := !breaker.IsOpen()
		if !isHealthy {
			overallHealthy = false
		}
		
		status[name] = map[string]interface{}{
			"healthy":       isHealthy,
			"state":         metrics.CurrentState.String(),
			"total_requests": metrics.TotalRequests,
			"success_rate":  fmt.Sprintf("%.2f%%", metrics.SuccessRate),
			"failure_rate":  fmt.Sprintf("%.2f%%", metrics.FailureRate),
			"last_failure":  metrics.LastFailure,
			"last_success":  metrics.LastSuccess,
		}
	}
	
	status["overall_healthy"] = overallHealthy
	status["timestamp"] = time.Now().UTC()
	
	return status
}

// ResetAllCircuitBreakers resets all circuit breakers to their initial state
func (i *CircuitBreakerIntegration) ResetAllCircuitBreakers() {
	breakers := i.manager.GetAllCircuitBreakers()
	
	for name, breaker := range breakers {
		breaker.Reset()
		i.logger.Info("Circuit breaker reset", "name", name)
	}
}

// IsServiceAvailable checks if a service is available (circuit breaker is not open)
func (i *CircuitBreakerIntegration) IsServiceAvailable(serviceName string) bool {
	breaker, exists := i.manager.GetCircuitBreaker(serviceName)
	if !exists {
		return true // If no circuit breaker exists, assume service is available
	}
	
	return !breaker.IsOpen()
}

// IsDatabaseAvailable checks if the database is available
func (i *CircuitBreakerIntegration) IsDatabaseAvailable() bool {
	return !i.dbBreaker.IsOpen()
}

// IsCacheAvailable checks if the cache is available
func (i *CircuitBreakerIntegration) IsCacheAvailable() bool {
	return !i.cacheBreaker.IsOpen()
}

// IsS3Available checks if S3 is available
func (i *CircuitBreakerIntegration) IsS3Available() bool {
	return !i.s3Breaker.IsOpen()
}