package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Test CircuitBreakerIntegration Creation
func TestCircuitBreakerIntegration_Creation(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	assert.NotNil(t, integration)
	assert.NotNil(t, integration.GetManager())
	assert.NotNil(t, integration.GetDatabaseBreaker())
	assert.NotNil(t, integration.GetCacheBreaker())
	assert.NotNil(t, integration.GetS3Breaker())

	// Verify circuit breakers are registered in manager
	dbBreaker, exists := integration.GetManager().GetCircuitBreaker("database")
	assert.True(t, exists)
	assert.Equal(t, integration.GetDatabaseBreaker(), dbBreaker)

	cacheBreaker, exists := integration.GetManager().GetCircuitBreaker("cache")
	assert.True(t, exists)
	assert.Equal(t, integration.GetCacheBreaker(), cacheBreaker)

	s3Breaker, exists := integration.GetManager().GetCircuitBreaker("s3")
	assert.True(t, exists)
	assert.Equal(t, integration.GetS3Breaker(), s3Breaker)
}

// Test Database Circuit Breaker Integration
func TestCircuitBreakerIntegration_DatabaseWrapper(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// Create in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Setup health checks
	integration.SetupHealthChecks(db, nil)

	// Create database wrapper
	dbWrapper := integration.NewDatabaseWrapper(db)
	assert.NotNil(t, dbWrapper)

	ctx := context.Background()

	// Test simple table creation
	type TestTable struct {
		ID   uint   `gorm:"primaryKey"`
		Name string `gorm:"size:100"`
	}

	err = dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		return db.AutoMigrate(&TestTable{})
	})
	assert.NoError(t, err)

	// Test create operation
	testRecord := &TestTable{Name: "test"}
	err = dbWrapper.CreateWithFallback(ctx, testRecord)
	assert.NoError(t, err)

	// Test find operation
	var results []TestTable
	err = dbWrapper.FindWithFallback(ctx, &results)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "test", results[0].Name)

	// Test update operation
	testRecord.Name = "updated"
	err = dbWrapper.UpdateWithFallback(ctx, testRecord)
	assert.NoError(t, err)

	// Test delete operation
	err = dbWrapper.DeleteWithFallback(ctx, testRecord)
	assert.NoError(t, err)
}

// Test Cache Circuit Breaker Integration
func TestCircuitBreakerIntegration_CacheWrapper(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// Create mini Redis server for testing
	s := miniredis.RunT(t)
	defer s.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer redisClient.Close()

	// Setup health checks
	integration.SetupHealthChecks(nil, redisClient)

	// Create cache wrapper
	cacheWrapper := integration.NewCacheWrapper(redisClient)
	assert.NotNil(t, cacheWrapper)

	ctx := context.Background()

	// Test set operation
	err := cacheWrapper.Set(ctx, "test-key", "test-value", 1*time.Minute)
	assert.NoError(t, err)

	// Test get operation
	value, err := cacheWrapper.Get(ctx, "test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-value", value)

	// Test hash operations
	err = cacheWrapper.HSet(ctx, "test-hash", "field1", "value1")
	assert.NoError(t, err)

	hashValue, err := cacheWrapper.HGet(ctx, "test-hash", "field1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", hashValue)

	// Test delete operation
	err = cacheWrapper.Del(ctx, "test-key")
	assert.NoError(t, err)

	// Verify key is deleted
	_, err = cacheWrapper.Get(ctx, "test-key")
	assert.Error(t, err)
}

// Test S3 Circuit Breaker Integration
func TestCircuitBreakerIntegration_S3Wrapper(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// Create S3 wrapper
	s3Wrapper := integration.NewS3Wrapper()
	assert.NotNil(t, s3Wrapper)

	ctx := context.Background()

	// Test successful S3 operation
	result, err := s3Wrapper.ExecuteS3Operation(ctx, func(ctx context.Context) (interface{}, error) {
		return "s3 success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "s3 success", result)

	// Test failed S3 operation (should use fallback)
	result, err = s3Wrapper.ExecuteS3Operation(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("s3 error")
	})
	assert.Error(t, err)
	assert.Nil(t, result)

	// Should be S3CircuitBreakerError
	var s3Err *S3CircuitBreakerError
	assert.ErrorAs(t, err, &s3Err)
}

// Test Service Availability Checks
func TestCircuitBreakerIntegration_ServiceAvailability(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// All services should be available initially
	assert.True(t, integration.IsDatabaseAvailable())
	assert.True(t, integration.IsCacheAvailable())
	assert.True(t, integration.IsS3Available())

	// Test generic service availability
	assert.True(t, integration.IsServiceAvailable("database"))
	assert.True(t, integration.IsServiceAvailable("cache"))
	assert.True(t, integration.IsServiceAvailable("s3"))
	assert.True(t, integration.IsServiceAvailable("nonexistent")) // Should return true for unknown services

	// Force database circuit breaker open
	integration.GetDatabaseBreaker().ForceOpen()
	assert.False(t, integration.IsDatabaseAvailable())
	assert.False(t, integration.IsServiceAvailable("database"))

	// Other services should still be available
	assert.True(t, integration.IsCacheAvailable())
	assert.True(t, integration.IsS3Available())
}

// Test Health Status
func TestCircuitBreakerIntegration_HealthStatus(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// Test health status
	healthStatus := integration.GetHealthStatus()
	assert.NotNil(t, healthStatus)

	// Should have entries for all three services plus overall
	assert.Contains(t, healthStatus, "database")
	assert.Contains(t, healthStatus, "cache")
	assert.Contains(t, healthStatus, "s3")
	assert.Contains(t, healthStatus, "overall_healthy")
	assert.Contains(t, healthStatus, "timestamp")

	// All should be healthy initially
	assert.True(t, healthStatus["overall_healthy"].(bool))

	// Check individual service health
	dbStatus := healthStatus["database"].(map[string]interface{})
	assert.True(t, dbStatus["healthy"].(bool))
	assert.Equal(t, "CLOSED", dbStatus["state"])

	// Force one circuit breaker open
	integration.GetDatabaseBreaker().ForceOpen()

	healthStatus = integration.GetHealthStatus()
	assert.False(t, healthStatus["overall_healthy"].(bool))

	dbStatus = healthStatus["database"].(map[string]interface{})
	assert.False(t, dbStatus["healthy"].(bool))
	assert.Equal(t, "OPEN", dbStatus["state"])
}

// Test Circuit Breaker Reset
func TestCircuitBreakerIntegration_Reset(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// Force all circuit breakers open
	integration.GetDatabaseBreaker().ForceOpen()
	integration.GetCacheBreaker().ForceOpen()
	integration.GetS3Breaker().ForceOpen()

	// Verify they are open
	assert.Equal(t, StateOpen, integration.GetDatabaseBreaker().GetState())
	assert.Equal(t, StateOpen, integration.GetCacheBreaker().GetState())
	assert.Equal(t, StateOpen, integration.GetS3Breaker().GetState())

	// Reset all circuit breakers
	integration.ResetAllCircuitBreakers()

	// Verify they are closed
	assert.Equal(t, StateClosed, integration.GetDatabaseBreaker().GetState())
	assert.Equal(t, StateClosed, integration.GetCacheBreaker().GetState())
	assert.Equal(t, StateClosed, integration.GetS3Breaker().GetState())
}

// Test Database Transaction with Circuit Breaker
func TestCircuitBreakerIntegration_DatabaseTransaction(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// Create in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create database wrapper
	dbWrapper := integration.NewDatabaseWrapper(db)

	ctx := context.Background()

	// Define test model
	type TestUser struct {
		ID   uint   `gorm:"primaryKey"`
		Name string `gorm:"size:100"`
	}

	// Create table
	err = dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		return db.AutoMigrate(&TestUser{})
	})
	assert.NoError(t, err)

	// Test successful transaction
	err = dbWrapper.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		user1 := &TestUser{Name: "User1"}
		if err := tx.Create(user1).Error; err != nil {
			return err
		}

		user2 := &TestUser{Name: "User2"}
		return tx.Create(user2).Error
	})
	assert.NoError(t, err)

	// Verify both users were created
	var users []TestUser
	err = dbWrapper.FindWithFallback(ctx, &users)
	assert.NoError(t, err)
	assert.Len(t, users, 2)

	// Test failed transaction (should rollback)
	err = dbWrapper.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		user3 := &TestUser{Name: "User3"}
		if err := tx.Create(user3).Error; err != nil {
			return err
		}

		// Force an error
		return errors.New("intentional error")
	})
	assert.Error(t, err)

	// Verify User3 was not created (rollback worked)
	err = dbWrapper.FindWithFallback(ctx, &users)
	assert.NoError(t, err)
	assert.Len(t, users, 2) // Still only 2 users
}

// Test Circuit Breaker Error Types
func TestCircuitBreakerIntegration_ErrorTypes(t *testing.T) {
	// Test DatabaseCircuitBreakerError
	originalErr := errors.New("connection failed")
	dbErr := &DatabaseCircuitBreakerError{
		OriginalError: originalErr,
		Message:       "Database is down",
	}

	assert.Contains(t, dbErr.Error(), "database circuit breaker")
	assert.Contains(t, dbErr.Error(), "Database is down")
	assert.Contains(t, dbErr.Error(), "connection failed")

	// Test S3CircuitBreakerError
	s3Err := &S3CircuitBreakerError{
		OriginalError: originalErr,
		Message:       "S3 is unavailable",
	}

	assert.Contains(t, s3Err.Error(), "s3 circuit breaker")
	assert.Contains(t, s3Err.Error(), "S3 is unavailable")
	assert.Contains(t, s3Err.Error(), "connection failed")
}

// Test Circuit Breaker with Failing Database
func TestCircuitBreakerIntegration_DatabaseFailures(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	// Create valid database initially
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	dbWrapper := integration.NewDatabaseWrapper(db)

	ctx := context.Background()

	// Test that deliberately failing operations trigger fallback
	err = dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		// Return a deliberate error to trigger fallback
		return errors.New("simulated database failure")
	})

	// Should return a DatabaseCircuitBreakerError (from fallback)
	assert.Error(t, err)
	var dbErr *DatabaseCircuitBreakerError
	assert.ErrorAs(t, err, &dbErr)
	assert.Contains(t, err.Error(), "Database circuit breaker is open")
}

// Test Circuit Breaker with Metrics
func TestCircuitBreakerIntegration_Metrics(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)
	defer integration.Close()

	ctx := context.Background()

	// Execute some operations to generate metrics
	s3Wrapper := integration.NewS3Wrapper()

	// Successful operations
	for i := 0; i < 5; i++ {
		s3Wrapper.ExecuteS3Operation(ctx, func(ctx context.Context) (interface{}, error) {
			return "success", nil
		})
	}

	// Failed operations
	for i := 0; i < 3; i++ {
		s3Wrapper.ExecuteS3Operation(ctx, func(ctx context.Context) (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	// Check metrics
	s3Metrics := integration.GetS3Breaker().GetMetrics()
	assert.Equal(t, uint64(8), s3Metrics.TotalRequests)
	assert.Equal(t, uint64(5), s3Metrics.TotalSuccesses)
	assert.Equal(t, uint64(3), s3Metrics.TotalFailures)
	assert.Equal(t, uint64(3), s3Metrics.FallbackExecutions) // Failures should trigger fallback
	assert.Greater(t, s3Metrics.SuccessRate, 0.0)
	assert.Greater(t, s3Metrics.FailureRate, 0.0)
}

// Test Integration Close
func TestCircuitBreakerIntegration_Close(t *testing.T) {
	logger := logger.NewDefault()
	integration := NewCircuitBreakerIntegration(logger)

	// Verify integration is working
	assert.NotNil(t, integration.GetManager())

	// Close the integration
	integration.Close()

	// After close, operations should still work but background processes should stop
	assert.NotPanics(t, func() {
		integration.GetHealthStatus()
		integration.IsDatabaseAvailable()
		integration.ResetAllCircuitBreakers()
	})
}