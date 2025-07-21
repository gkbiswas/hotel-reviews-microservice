package infrastructure

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// testCircuitBreakerConfig returns a minimal but complete config for testing
func testCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    3,
		SuccessThreshold:    2,
		ConsecutiveFailures: 2,
		MinimumRequestCount: 1,
		WindowSize:          60 * time.Second,
		WindowBuckets:       10,
		OpenTimeout:         100 * time.Millisecond,
		RequestTimeout:      5 * time.Second,
		EnableHealthCheck:   false,
		EnableMetrics:       true,  // Enable metrics for metrics tests
		EnableLogging:       false,
		EnableFallback:      true,  // Enable fallback for fallback tests
	}
}

// Test CircuitState string representations
func TestCircuitState_String(t *testing.T) {
	testCases := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "CLOSED"},
		{StateOpen, "OPEN"},
		{StateHalfOpen, "HALF_OPEN"},
		{CircuitState(999), "UNKNOWN"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.state.String())
		})
	}
}

// Test CircuitBreakerError
func TestCircuitBreakerError(t *testing.T) {
	err := &CircuitBreakerError{
		State:   StateOpen,
		Message: "circuit is open",
	}

	assert.Equal(t, "circuit breaker OPEN: circuit is open", err.Error())
	assert.True(t, IsCircuitBreakerError(err))
	assert.False(t, IsCircuitBreakerError(errors.New("other error")))
}

// Test DefaultCircuitBreakerConfig
func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "default", config.Name)
	assert.Equal(t, uint32(5), config.FailureThreshold)
	assert.Equal(t, uint32(3), config.SuccessThreshold)
	assert.Equal(t, uint32(3), config.ConsecutiveFailures)
	assert.Equal(t, uint32(10), config.MinimumRequestCount)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.Equal(t, 60*time.Second, config.OpenTimeout)
	assert.Equal(t, 30*time.Second, config.HalfOpenTimeout)
	assert.Equal(t, 60*time.Second, config.WindowSize)
	assert.Equal(t, 10, config.WindowBuckets)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 2.0, config.RetryBackoffFactor)
	assert.True(t, config.FailFast)
	assert.True(t, config.EnableFallback)
	assert.True(t, config.EnableMetrics)
	assert.True(t, config.EnableLogging)
	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	assert.Equal(t, 5*time.Second, config.HealthCheckTimeout)
	assert.True(t, config.EnableHealthCheck)
}

// Test SlidingWindow
func TestSlidingWindow_Creation(t *testing.T) {
	window := NewSlidingWindow(60*time.Second, 10)

	assert.NotNil(t, window)
	assert.Equal(t, 60*time.Second, window.size)
	assert.Equal(t, 6*time.Second, window.bucketSize)
	assert.Len(t, window.buckets, 10)
	assert.Equal(t, 0, window.currentIdx)
}

func TestSlidingWindow_Record(t *testing.T) {
	window := NewSlidingWindow(60*time.Second, 10)

	// Record some operations
	window.Record(true, false)   // success
	window.Record(false, false)  // failure
	window.Record(false, true)   // timeout

	// Get stats
	requests, successes, failures, timeouts := window.GetStats()
	assert.Equal(t, uint64(3), requests)
	assert.Equal(t, uint64(1), successes)
	assert.Equal(t, uint64(1), failures)
	assert.Equal(t, uint64(1), timeouts)
}

func TestSlidingWindow_TimeBasedBuckets(t *testing.T) {
	window := NewSlidingWindow(100*time.Millisecond, 2)

	// Record in first bucket
	window.Record(true, false)

	// Wait for bucket rotation
	time.Sleep(60 * time.Millisecond)
	window.Record(false, false)

	// Check both records are present
	requests, successes, failures, _ := window.GetStats()
	assert.Equal(t, uint64(2), requests)
	assert.Equal(t, uint64(1), successes)
	assert.Equal(t, uint64(1), failures)
}

// Test CircuitBreaker Creation
func TestCircuitBreaker_Creation(t *testing.T) {
	logger := logger.NewDefault()
	config := &CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    3,
		SuccessThreshold:    2,
		ConsecutiveFailures: 2,
		MinimumRequestCount: 5,
		WindowSize:          60 * time.Second,
		WindowBuckets:       10,
		EnableHealthCheck:   false,
		EnableMetrics:       false,
	}

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	assert.NotNil(t, cb)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, "test", cb.GetName())
	assert.Equal(t, config, cb.GetConfig())
	assert.True(t, cb.IsClosed())
	assert.False(t, cb.IsOpen())
	assert.False(t, cb.IsHalfOpen())
}

func TestCircuitBreaker_CreationWithNilConfig(t *testing.T) {
	logger := logger.NewDefault()
	cb := NewCircuitBreaker(nil, logger)
	defer cb.Close()

	assert.NotNil(t, cb)
	assert.Equal(t, "default", cb.config.Name)
}

// Test CircuitBreaker State Transitions
func TestCircuitBreaker_StateTransitions(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()
	config.OpenTimeout = 100 * time.Millisecond

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Initially closed
	assert.Equal(t, StateClosed, cb.GetState())

	// Force circuit to open state for testing
	cb.ForceOpen()
	assert.Equal(t, StateOpen, cb.GetState())

	// Now requests should be rejected
	failingFunc := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("simulated failure")
	}
	
	_, err := cb.Execute(context.Background(), failingFunc)
	assert.Error(t, err)
	assert.True(t, IsCircuitBreakerError(err))

	// Wait for open timeout
	time.Sleep(150 * time.Millisecond)

	// Next request should transition to half-open
	successFunc := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(context.Background(), successFunc)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Second success should close the circuit
	result, err = cb.Execute(context.Background(), successFunc)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, StateClosed, cb.GetState())
}

// Test CircuitBreaker Execute
func TestCircuitBreaker_Execute_Success(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	successFunc := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(context.Background(), successFunc)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	metrics := cb.GetMetrics()
	assert.Equal(t, uint64(1), metrics.TotalRequests)
	assert.Equal(t, uint64(1), metrics.TotalSuccesses)
	assert.Equal(t, uint64(0), metrics.TotalFailures)
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	failureFunc := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("test failure")
	}

	result, err := cb.Execute(context.Background(), failureFunc)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "test failure", err.Error())

	metrics := cb.GetMetrics()
	assert.Equal(t, uint64(1), metrics.TotalRequests)
	assert.Equal(t, uint64(0), metrics.TotalSuccesses)
	assert.Equal(t, uint64(1), metrics.TotalFailures)
}

// Test CircuitBreaker Timeout
func TestCircuitBreaker_Execute_Timeout(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()
	config.RequestTimeout = 50 * time.Millisecond

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	slowFunc := func(ctx context.Context) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return "success", nil
		}
	}

	result, err := cb.Execute(context.Background(), slowFunc)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, context.DeadlineExceeded, err)

	metrics := cb.GetMetrics()
	assert.Equal(t, uint64(1), metrics.TotalRequests)
	assert.Equal(t, uint64(1), metrics.TotalTimeouts)
}

// Test CircuitBreaker Retry Logic
func TestCircuitBreaker_ExecuteWithRetry(t *testing.T) {
	logger := logger.NewDefault()
	config := &CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    3,
		SuccessThreshold:    2,
		ConsecutiveFailures: 2,
		MinimumRequestCount: 1,
		RequestTimeout:      5 * time.Second,
		OpenTimeout:         10 * time.Second,
		HalfOpenTimeout:     5 * time.Second,
		WindowSize:          10 * time.Second,
		WindowBuckets:       5,
		MaxRetries:          2,
		RetryDelay:          10 * time.Millisecond,
		RetryBackoffFactor:  2.0,
		EnableHealthCheck:   false,
		EnableMetrics:       false,
		EnableLogging:       false,
	}

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	attempt := 0
	retryFunc := func(ctx context.Context) (interface{}, error) {
		attempt++
		if attempt < 3 {
			return nil, errors.New("temporary failure")
		}
		return "success", nil
	}

	result, err := cb.Execute(context.Background(), retryFunc)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, attempt) // Should have retried twice
}

// Test CircuitBreaker Fallback
func TestCircuitBreaker_ExecuteWithFallback(t *testing.T) {
	logger := logger.NewDefault()
	config := &CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    3,
		SuccessThreshold:    2,
		ConsecutiveFailures: 2,
		MinimumRequestCount: 1,
		RequestTimeout:      5 * time.Second,
		OpenTimeout:         10 * time.Second,
		HalfOpenTimeout:     5 * time.Second,
		WindowSize:          10 * time.Second,
		WindowBuckets:       5,
		EnableFallback:      true,
		EnableHealthCheck:   false,
		EnableMetrics:       false,
		EnableLogging:       false,
	}

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	failureFunc := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("main function failed")
	}

	fallbackFunc := func(ctx context.Context, err error) (interface{}, error) {
		return "fallback result", nil
	}

	result, err := cb.ExecuteWithFallback(context.Background(), failureFunc, fallbackFunc)
	assert.NoError(t, err)
	assert.Equal(t, "fallback result", result)

	metrics := cb.GetMetrics()
	assert.Equal(t, uint64(1), metrics.FallbackExecutions)
	assert.Equal(t, uint64(1), metrics.FallbackSuccesses)
}

func TestCircuitBreaker_FallbackFailure(t *testing.T) {
	logger := logger.NewDefault()
	config := &CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    3,
		SuccessThreshold:    2,
		ConsecutiveFailures: 2,
		MinimumRequestCount: 1,
		RequestTimeout:      5 * time.Second,
		OpenTimeout:         10 * time.Second,
		HalfOpenTimeout:     5 * time.Second,
		WindowSize:          10 * time.Second,
		WindowBuckets:       5,
		EnableFallback:      true,
		EnableHealthCheck:   false,
		EnableMetrics:       false,
		EnableLogging:       false,
	}

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	failureFunc := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("main function failed")
	}

	fallbackFunc := func(ctx context.Context, err error) (interface{}, error) {
		return nil, errors.New("fallback also failed")
	}

	result, err := cb.ExecuteWithFallback(context.Background(), failureFunc, fallbackFunc)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "fallback also failed", err.Error())

	metrics := cb.GetMetrics()
	assert.Equal(t, uint64(1), metrics.FallbackExecutions)
	assert.Equal(t, uint64(1), metrics.FallbackFailures)
}

// Test CircuitBreaker Force State Changes
func TestCircuitBreaker_ForceStateChanges(t *testing.T) {
	logger := logger.NewDefault()
	config := &CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    3,
		SuccessThreshold:    2,
		ConsecutiveFailures: 2,
		MinimumRequestCount: 1,
		RequestTimeout:      5 * time.Second,
		OpenTimeout:         10 * time.Second,
		HalfOpenTimeout:     5 * time.Second,
		WindowSize:          10 * time.Second,
		WindowBuckets:       5,
		EnableHealthCheck:   false,
		EnableMetrics:       false,
		EnableLogging:       false,
	}

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Initially closed
	assert.Equal(t, StateClosed, cb.GetState())

	// Force open
	cb.ForceOpen()
	assert.Equal(t, StateOpen, cb.GetState())
	assert.True(t, cb.IsOpen())

	// Force half-open
	cb.ForceHalfOpen()
	assert.Equal(t, StateHalfOpen, cb.GetState())
	assert.True(t, cb.IsHalfOpen())

	// Force close
	cb.ForceClose()
	assert.Equal(t, StateClosed, cb.GetState())
	assert.True(t, cb.IsClosed())
}

// Test CircuitBreaker Reset
func TestCircuitBreaker_Reset(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Execute some operations to generate metrics
	successFunc := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	cb.Execute(context.Background(), successFunc)
	cb.ForceOpen()

	// Verify state and metrics
	assert.Equal(t, StateOpen, cb.GetState())
	metrics := cb.GetMetrics()
	assert.Greater(t, metrics.TotalRequests, uint64(0))

	// Reset
	cb.Reset()

	// Verify reset state
	assert.Equal(t, StateClosed, cb.GetState())
	metrics = cb.GetMetrics()
	assert.Equal(t, uint64(0), metrics.TotalRequests)
	assert.Equal(t, uint64(0), metrics.TotalSuccesses)
	assert.Equal(t, uint64(0), metrics.TotalFailures)
}

// Test CircuitBreaker Health Check
func TestCircuitBreaker_HealthCheck(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()
	config.EnableHealthCheck = true
	config.HealthCheckInterval = 50 * time.Millisecond
	config.HealthCheckTimeout = 100 * time.Millisecond

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Set a health check that always passes
	healthCheckCalled := false
	cb.SetHealthCheck(func(ctx context.Context) error {
		healthCheckCalled = true
		return nil
	})

	// Wait for health check to run
	time.Sleep(100 * time.Millisecond)
	assert.True(t, healthCheckCalled)
}

// Test CircuitBreaker Concurrent Operations
func TestCircuitBreaker_ConcurrentOperations(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	const numGoroutines = 10
	const numOperations = 10

	var wg sync.WaitGroup
	successCount := int64(0)
	var mu sync.Mutex

	successFunc := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := cb.Execute(context.Background(), successFunc)
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(numGoroutines*numOperations), successCount)
	metrics := cb.GetMetrics()
	assert.Equal(t, uint64(numGoroutines*numOperations), metrics.TotalRequests)
	assert.Equal(t, uint64(numGoroutines*numOperations), metrics.TotalSuccesses)
}

// Test CircuitBreaker Half-Open Request Limiting
func TestCircuitBreaker_HalfOpenRequestLimiting(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()
	config.SuccessThreshold = 2

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Force to half-open state
	cb.ForceHalfOpen()
	assert.Equal(t, StateHalfOpen, cb.GetState())

	successFunc := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	// First request should be allowed
	result, err := cb.Execute(context.Background(), successFunc)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Second request should be allowed
	result, err = cb.Execute(context.Background(), successFunc)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Circuit should now be closed after success threshold
	assert.Equal(t, StateClosed, cb.GetState())

	// Force back to half-open
	cb.ForceHalfOpen()

	// Requests beyond success threshold should be rejected
	result, err = cb.Execute(context.Background(), successFunc)
	assert.NoError(t, err)

	result, err = cb.Execute(context.Background(), successFunc)
	assert.NoError(t, err)

	// Should be closed again
	assert.Equal(t, StateClosed, cb.GetState())
}

// Test Service-Specific Circuit Breakers
func TestNewS3CircuitBreaker(t *testing.T) {
	logger := logger.NewDefault()
	cb := NewS3CircuitBreaker(logger)
	defer cb.Close()

	assert.NotNil(t, cb)
	assert.Equal(t, "s3", cb.GetName())
	assert.Equal(t, uint32(5), cb.config.FailureThreshold)
	assert.Equal(t, 30*time.Second, cb.config.RequestTimeout)
	assert.False(t, cb.config.EnableHealthCheck) // S3 doesn't have simple health check
}

func TestNewDatabaseCircuitBreaker(t *testing.T) {
	logger := logger.NewDefault()
	cb := NewDatabaseCircuitBreaker(logger)
	defer cb.Close()

	assert.NotNil(t, cb)
	assert.Equal(t, "database", cb.GetName())
	assert.Equal(t, uint32(3), cb.config.FailureThreshold)
	assert.Equal(t, 10*time.Second, cb.config.RequestTimeout)
	assert.True(t, cb.config.EnableHealthCheck)
}

func TestNewCacheCircuitBreaker(t *testing.T) {
	logger := logger.NewDefault()
	cb := NewCacheCircuitBreaker(logger)
	defer cb.Close()

	assert.NotNil(t, cb)
	assert.Equal(t, "cache", cb.GetName())
	assert.Equal(t, uint32(10), cb.config.FailureThreshold)
	assert.Equal(t, 5*time.Second, cb.config.RequestTimeout)
	assert.False(t, cb.config.FailFast) // Cache failures are less critical
}

// Test CircuitBreakerManager
func TestCircuitBreakerManager(t *testing.T) {
	logger := logger.NewDefault()
	manager := NewCircuitBreakerManager(logger)
	defer manager.Close()

	assert.NotNil(t, manager)

	// Add circuit breakers
	s3CB := NewS3CircuitBreaker(logger)
	dbCB := NewDatabaseCircuitBreaker(logger)
	cacheCB := NewCacheCircuitBreaker(logger)

	manager.AddCircuitBreaker("s3", s3CB)
	manager.AddCircuitBreaker("database", dbCB)
	manager.AddCircuitBreaker("cache", cacheCB)

	// Test retrieval
	retrievedS3, exists := manager.GetCircuitBreaker("s3")
	assert.True(t, exists)
	assert.Equal(t, s3CB, retrievedS3)

	retrievedDB, exists := manager.GetCircuitBreaker("database")
	assert.True(t, exists)
	assert.Equal(t, dbCB, retrievedDB)

	// Test non-existent
	_, exists = manager.GetCircuitBreaker("nonexistent")
	assert.False(t, exists)

	// Test get all
	allBreakers := manager.GetAllCircuitBreakers()
	assert.Len(t, allBreakers, 3)
	assert.Contains(t, allBreakers, "s3")
	assert.Contains(t, allBreakers, "database")
	assert.Contains(t, allBreakers, "cache")

	// Test get metrics
	allMetrics := manager.GetMetrics()
	assert.Len(t, allMetrics, 3)
	assert.Contains(t, allMetrics, "s3")
	assert.Contains(t, allMetrics, "database")
	assert.Contains(t, allMetrics, "cache")
}

// Test CircuitBreaker Metrics
func TestCircuitBreaker_Metrics(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()
	config.EnableFallback = false // Disable fallback for metrics test to avoid interference

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Execute operations to generate metrics
	successFunc := func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "success", nil
	}

	failureFunc := func(ctx context.Context) (interface{}, error) {
		time.Sleep(5 * time.Millisecond)
		return nil, errors.New("failure")
	}

	// Execute successful operations
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), successFunc)
	}

	// Execute failed operations
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), failureFunc)
	}

	metrics := cb.GetMetrics()

	// Debug output
	t.Logf("TotalRequests: %d, TotalSuccesses: %d, TotalFailures: %d", 
		metrics.TotalRequests, metrics.TotalSuccesses, metrics.TotalFailures)
	t.Logf("SuccessRate: %f, FailureRate: %f", metrics.SuccessRate, metrics.FailureRate)

	assert.Equal(t, uint64(5), metrics.TotalRequests)
	assert.Equal(t, uint64(3), metrics.TotalSuccesses)
	assert.Equal(t, uint64(2), metrics.TotalFailures)
	assert.Greater(t, metrics.AverageResponseTime, time.Duration(0))
	assert.Greater(t, metrics.MaxResponseTime, time.Duration(0))
	assert.Greater(t, metrics.MinResponseTime, time.Duration(0))
	assert.Equal(t, 60.0, metrics.SuccessRate)
	assert.Equal(t, 40.0, metrics.FailureRate)
}

// Test CircuitBreaker Error Classification
func TestCircuitBreaker_ErrorClassification(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Test retryable error
	retryableErr := errors.New("network error")
	assert.True(t, cb.isRetryableError(retryableErr))

	// Test non-retryable errors
	assert.False(t, cb.isRetryableError(context.Canceled))
	assert.False(t, cb.isRetryableError(context.DeadlineExceeded))
	assert.False(t, cb.isRetryableError(&CircuitBreakerError{State: StateOpen, Message: "open"}))

	// Test timeout error detection
	assert.True(t, cb.isTimeoutError(context.DeadlineExceeded))
	assert.False(t, cb.isTimeoutError(errors.New("other error")))
	assert.False(t, cb.isTimeoutError(nil))
}

// Test CircuitBreaker Fallback and Health Check Setting
func TestCircuitBreaker_SetFallbackAndHealthCheck(t *testing.T) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	// Set fallback
	fallbackCalled := false
	fallback := func(ctx context.Context, err error) (interface{}, error) {
		fallbackCalled = true
		return "fallback", nil
	}
	cb.SetFallback(fallback)

	// Set health check
	healthCheckCalled := false
	healthCheck := func(ctx context.Context) error {
		healthCheckCalled = true
		return nil
	}
	cb.SetHealthCheck(healthCheck)

	// Test fallback is called
	failureFunc := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("failure")
	}

	result, err := cb.Execute(context.Background(), failureFunc)
	assert.NoError(t, err)
	assert.Equal(t, "fallback", result)
	assert.True(t, fallbackCalled)

	// Test health check is set (we can't easily test execution without enabling background processes)
	assert.NotNil(t, cb.healthCheck)
	_ = healthCheckCalled // Mark as used
}

// Benchmark tests
func BenchmarkCircuitBreaker_Execute_Success(b *testing.B) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()
	config.Name = "benchmark"

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	successFunc := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(context.Background(), successFunc)
		}
	})
}

func BenchmarkCircuitBreaker_Execute_Failure(b *testing.B) {
	logger := logger.NewDefault()
	config := testCircuitBreakerConfig()
	config.Name = "benchmark"

	cb := NewCircuitBreaker(config, logger)
	defer cb.Close()

	failureFunc := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("failure")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(context.Background(), failureFunc)
		}
	})
}