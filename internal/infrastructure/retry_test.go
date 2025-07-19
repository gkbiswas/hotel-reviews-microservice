package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"testing"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions and mocks

// mockRetryableFunc creates a mock function that fails for the first few attempts
func mockRetryableFunc(failAttempts int, returnValue interface{}) RetryableFunc {
	return func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt <= failAttempts {
			return nil, errors.New("mock failure")
		}
		return returnValue, nil
	}
}

// mockTimeoutFunc creates a mock function that times out
func mockTimeoutFunc(delay time.Duration) RetryableFunc {
	return func(ctx context.Context, attempt int) (interface{}, error) {
		select {
		case <-time.After(delay):
			return "success", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// mockNetworkErrorFunc creates a mock function that returns network errors
func mockNetworkErrorFunc() RetryableFunc {
	return func(ctx context.Context, attempt int) (interface{}, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	}
}

// mockDeadLetterHandler creates a mock dead letter handler
func mockDeadLetterHandler(received *[]string) DeadLetterHandler {
	return func(ctx context.Context, operation string, err error, attempts int, metadata map[string]interface{}) {
		*received = append(*received, fmt.Sprintf("operation:%s, error:%v, attempts:%d", operation, err, attempts))
	}
}

// Test cases

func TestRetryManager_BasicRetry(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 10 * time.Millisecond
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Test successful operation on first attempt
	t.Run("success_on_first_attempt", func(t *testing.T) {
		fn := mockRetryableFunc(0, "success")
		result, err := rm.Execute(context.Background(), fn)

		require.NoError(t, err)
		assert.Equal(t, "success", result)

		metrics := rm.GetMetrics()
		assert.Equal(t, uint64(1), metrics.TotalOperations)
		assert.Equal(t, uint64(1), metrics.SuccessfulOperations)
		assert.Equal(t, uint64(0), metrics.TotalRetries)
	})

	// Test successful operation on second attempt
	t.Run("success_on_second_attempt", func(t *testing.T) {
		fn := mockRetryableFunc(1, "success")
		result, err := rm.Execute(context.Background(), fn)

		require.NoError(t, err)
		assert.Equal(t, "success", result)

		metrics := rm.GetMetrics()
		assert.Equal(t, uint64(2), metrics.TotalOperations)
		assert.Equal(t, uint64(2), metrics.SuccessfulOperations)
		assert.Equal(t, uint64(1), metrics.TotalRetries)
	})

	// Test failure after all attempts
	t.Run("failure_after_all_attempts", func(t *testing.T) {
		fn := mockRetryableFunc(5, "success") // Fail more than max attempts
		result, err := rm.Execute(context.Background(), fn)

		require.Error(t, err)
		assert.Nil(t, result)

		metrics := rm.GetMetrics()
		assert.Equal(t, uint64(3), metrics.TotalOperations)
		assert.Equal(t, uint64(1), metrics.FailedOperations)
		assert.Equal(t, uint64(3), metrics.TotalRetries) // 2 retries for this operation
	})
}

func TestRetryManager_ExponentialBackoff(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 4
	config.BaseDelay = 100 * time.Millisecond
	config.Strategy = StrategyExponentialBackoff
	config.Multiplier = 2.0
	config.JitterType = JitterTypeNone
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	start := time.Now()
	fn := mockRetryableFunc(3, "success")
	result, err := rm.Execute(context.Background(), fn)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", result)

	// Expected delays: 100ms, 200ms, 400ms = 700ms total
	// Allow some tolerance for execution time
	assert.True(t, duration >= 700*time.Millisecond)
	assert.True(t, duration < 1*time.Second)
}

func TestRetryManager_FixedDelay(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 100 * time.Millisecond
	config.Strategy = StrategyFixedDelay
	config.JitterType = JitterTypeNone
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	start := time.Now()
	fn := mockRetryableFunc(2, "success")
	result, err := rm.Execute(context.Background(), fn)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", result)

	// Expected delays: 100ms, 100ms = 200ms total
	assert.True(t, duration >= 200*time.Millisecond)
	assert.True(t, duration < 300*time.Millisecond)
}

func TestRetryManager_LinearBackoff(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 100 * time.Millisecond
	config.Strategy = StrategyLinearBackoff
	config.Multiplier = 1.0
	config.JitterType = JitterTypeNone
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	start := time.Now()
	fn := mockRetryableFunc(2, "success")
	result, err := rm.Execute(context.Background(), fn)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", result)

	// Expected delays: 100ms (attempt 2), 200ms (attempt 3) = 300ms total
	assert.True(t, duration >= 300*time.Millisecond)
	assert.True(t, duration < 400*time.Millisecond)
}

func TestRetryManager_FibonacciBackoff(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 4
	config.BaseDelay = 100 * time.Millisecond
	config.Strategy = StrategyFibonacciBackoff
	config.JitterType = JitterTypeNone
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	start := time.Now()
	fn := mockRetryableFunc(3, "success")
	result, err := rm.Execute(context.Background(), fn)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", result)

	// Expected delays: 100ms (fib[1] = 1), 100ms (fib[2] = 1), 200ms (fib[3] = 2) = 400ms total
	assert.True(t, duration >= 400*time.Millisecond)
	assert.True(t, duration < 500*time.Millisecond)
}

func TestRetryManager_CustomBackoff(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 100 * time.Millisecond
	config.Strategy = StrategyCustom
	config.JitterType = JitterTypeNone
	config.EnableLogging = false
	config.CustomBackoff = func(attempt int, baseDelay time.Duration) time.Duration {
		return time.Duration(attempt*attempt) * baseDelay // Quadratic backoff
	}

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	start := time.Now()
	fn := mockRetryableFunc(2, "success")
	result, err := rm.Execute(context.Background(), fn)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", result)

	// Expected delays: 100ms (1^2 * 100ms), 400ms (2^2 * 100ms) = 500ms total
	assert.True(t, duration >= 500*time.Millisecond)
	assert.True(t, duration < 600*time.Millisecond)
}

func TestRetryManager_Jitter(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 100 * time.Millisecond
	config.Strategy = StrategyFixedDelay
	config.JitterType = JitterTypeFull
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Run multiple times to test jitter variability
	durations := make([]time.Duration, 5)
	for i := 0; i < 5; i++ {
		start := time.Now()
		fn := mockRetryableFunc(2, "success")
		result, err := rm.Execute(context.Background(), fn)
		durations[i] = time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, "success", result)
	}

	// With full jitter, durations should vary
	allSame := true
	for i := 1; i < len(durations); i++ {
		if durations[i] != durations[0] {
			allSame = false
			break
		}
	}
	assert.False(t, allSame, "Jitter should introduce variability in delays")
}

func TestRetryManager_MaxDelay(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 100 * time.Millisecond
	config.MaxDelay = 200 * time.Millisecond
	config.Strategy = StrategyExponentialBackoff
	config.Multiplier = 10.0 // Large multiplier to test max delay
	config.JitterType = JitterTypeNone
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	start := time.Now()
	fn := mockRetryableFunc(4, "success")
	result, err := rm.Execute(context.Background(), fn)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", result)

	// All delays should be capped at MaxDelay
	// Expected delays: 100ms, 200ms, 200ms, 200ms = 700ms total
	assert.True(t, duration >= 700*time.Millisecond)
	assert.True(t, duration < 800*time.Millisecond)
}

func TestRetryManager_ContextTimeout(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 100 * time.Millisecond
	config.EnableLogging = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	fn := mockTimeoutFunc(100 * time.Millisecond) // Takes longer than context timeout
	result, err := rm.Execute(ctx, fn)

	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Nil(t, result)
}

func TestRetryManager_OperationTimeout(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 10 * time.Millisecond
	config.Timeout = 50 * time.Millisecond
	config.EnableLogging = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	fn := mockTimeoutFunc(100 * time.Millisecond) // Takes longer than operation timeout
	result, err := rm.Execute(context.Background(), fn)

	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Nil(t, result)
}

func TestRetryManager_ErrorClassification(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	rm := NewRetryManager(DefaultRetryConfig(), nil, logger)
	defer rm.Close()

	testCases := []struct {
		name     string
		error    error
		expected ErrorType
	}{
		{
			name:     "network_error",
			error:    &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			expected: ErrorTypeNetwork,
		},
		{
			name:     "timeout_error",
			error:    context.DeadlineExceeded,
			expected: ErrorTypeTimeout,
		},
		{
			name:     "context_canceled",
			error:    context.Canceled,
			expected: ErrorTypeTimeout,
		},
		{
			name:     "rate_limit_error",
			error:    errors.New("rate limit exceeded"),
			expected: ErrorTypeRateLimit,
		},
		{
			name:     "server_error",
			error:    errors.New("500 internal server error"),
			expected: ErrorTypeExternal,
		},
		{
			name:     "client_error",
			error:    errors.New("400 bad request"),
			expected: ErrorTypeClient,
		},
		{
			name:     "invalid_error",
			error:    errors.New("invalid input"),
			expected: ErrorTypeValidation,
		},
		{
			name:     "unknown_error",
			error:    errors.New("some unknown error"),
			expected: ErrorTypeSystem,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errorType := rm.classifyError(tc.error)
			assert.Equal(t, tc.expected, errorType)
		})
	}
}

func TestRetryManager_RetryConditions(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 10 * time.Millisecond
	config.EnableLogging = false

	// Test network errors are retryable
	t.Run("network_error_retryable", func(t *testing.T) {
		rm := NewRetryManager(config, nil, logger)
		defer rm.Close()

		fn := mockNetworkErrorFunc()
		result, err := rm.Execute(context.Background(), fn)

		require.Error(t, err)
		assert.Nil(t, result)

		metrics := rm.GetMetrics()
		assert.Equal(t, uint64(1), metrics.TotalOperations)
		assert.Equal(t, uint64(1), metrics.FailedOperations)
		assert.Equal(t, uint64(4), metrics.TotalRetries) // 4 retries for network error
	})

	// Test client errors are not retryable
	t.Run("client_error_not_retryable", func(t *testing.T) {
		rm := NewRetryManager(config, nil, logger)
		defer rm.Close()

		fn := func(ctx context.Context, attempt int) (interface{}, error) {
			return nil, errors.New("400 bad request")
		}

		result, err := rm.Execute(context.Background(), fn)

		require.Error(t, err)
		assert.Nil(t, result)

		metrics := rm.GetMetrics()
		assert.Equal(t, uint64(1), metrics.TotalOperations)
		assert.Equal(t, uint64(1), metrics.FailedOperations)
		assert.Equal(t, uint64(0), metrics.TotalRetries) // No retries for client error
	})
}

func TestRetryManager_CustomRetryConditions(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 10 * time.Millisecond
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	// Add custom condition: don't retry if attempt > 2
	config.CustomConditions = []RetryCondition{
		func(err error, attempt int) bool {
			return attempt <= 2
		},
	}

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	fn := mockRetryableFunc(5, "success") // Would normally retry
	result, err := rm.Execute(context.Background(), fn)

	require.Error(t, err)
	assert.Nil(t, result)

	metrics := rm.GetMetrics()
	assert.Equal(t, uint64(1), metrics.TotalOperations)
	assert.Equal(t, uint64(1), metrics.FailedOperations)
	assert.Equal(t, uint64(2), metrics.TotalRetries) // 2 retries: attempts 1->2 and 2->3, then condition stops at attempt 3
}

func TestRetryManager_DeadLetterQueue(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 2
	config.BaseDelay = 10 * time.Millisecond
	config.EnableDeadLetter = true
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	var deadLetterMessages []string
	config.DeadLetterHandler = mockDeadLetterHandler(&deadLetterMessages)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	fn := mockRetryableFunc(5, "success") // Always fails
	result, err := rm.Execute(context.Background(), fn)

	require.Error(t, err)
	assert.Nil(t, result)

	metrics := rm.GetMetrics()
	assert.Equal(t, uint64(1), metrics.DeadLetterCount)
	assert.Len(t, deadLetterMessages, 1)
	assert.Contains(t, deadLetterMessages[0], "operation:default")
	assert.Contains(t, deadLetterMessages[0], "attempts:2")
}

func TestRetryManager_CircuitBreakerIntegration(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})

	// Create circuit breaker with low thresholds for testing
	cbConfig := DefaultCircuitBreakerConfig()
	cbConfig.FailureThreshold = 2
	cbConfig.ConsecutiveFailures = 2
	cbConfig.MinimumRequestCount = 1
	cbConfig.EnableLogging = false
	circuitBreaker := NewCircuitBreaker(cbConfig, logger)
	defer circuitBreaker.Close()

	config := DefaultRetryConfig()
	config.MaxAttempts = 2
	config.BaseDelay = 10 * time.Millisecond
	config.EnableCircuitBreaker = true
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, circuitBreaker, logger)
	defer rm.Close()

	// First few operations should fail and open the circuit
	fn := mockRetryableFunc(5, "success") // Always fails

	for i := 0; i < 3; i++ {
		result, err := rm.Execute(context.Background(), fn)
		require.Error(t, err)
		assert.Nil(t, result)
	}

	// Circuit should be open now
	assert.True(t, circuitBreaker.IsOpen())

	// Next operation should be rejected by circuit breaker
	result, err := rm.Execute(context.Background(), fn)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, IsCircuitBreakerError(err))

	metrics := rm.GetMetrics()
	assert.Greater(t, metrics.CircuitBreakerRejects, uint64(0))
}

func TestRetryManager_Metrics(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 10 * time.Millisecond
	config.EnableLogging = false

	// Make system errors retryable for this test
	config.RetryableErrors = append(config.RetryableErrors, ErrorTypeSystem)

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Execute some operations
	fn1 := mockRetryableFunc(0, "success") // Success on first attempt
	fn2 := mockRetryableFunc(1, "success") // Success on second attempt
	fn3 := mockRetryableFunc(5, "success") // Always fails

	_, err := rm.Execute(context.Background(), fn1)
	require.NoError(t, err)

	_, err = rm.Execute(context.Background(), fn2)
	require.NoError(t, err)

	_, err = rm.Execute(context.Background(), fn3)
	require.Error(t, err)

	metrics := rm.GetMetrics()

	assert.Equal(t, uint64(3), metrics.TotalOperations)
	assert.Equal(t, uint64(2), metrics.SuccessfulOperations)
	assert.Equal(t, uint64(1), metrics.FailedOperations)
	assert.Equal(t, uint64(3), metrics.TotalRetries)                // 0 + 1 + 2 retries
	assert.Equal(t, 66.67, math.Round(metrics.SuccessRate*100)/100) // 2/3 * 100
	assert.Equal(t, 33.33, math.Round(metrics.FailureRate*100)/100) // 1/3 * 100
}

func TestRetryManager_OperationMetrics(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 2
	config.BaseDelay = 10 * time.Millisecond
	config.OperationName = "test_operation"
	config.OperationType = "test_type"
	config.EnableLogging = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	fn := mockRetryableFunc(0, "success")
	_, err := rm.Execute(context.Background(), fn)
	require.NoError(t, err)

	opMetrics := rm.GetOperationMetrics("test_operation", "test_type")
	require.NotNil(t, opMetrics)

	assert.Equal(t, "test_operation", opMetrics.OperationName)
	assert.Equal(t, "test_type", opMetrics.OperationType)
	assert.Equal(t, uint64(1), opMetrics.TotalOperations)
	assert.Equal(t, uint64(1), opMetrics.SuccessfulOperations)
	assert.Equal(t, uint64(0), opMetrics.TotalRetries)
	assert.Equal(t, 100.0, opMetrics.SuccessRate)
}

func TestRetryManager_PredefinedConfigs(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})

	testCases := []struct {
		name   string
		config *RetryConfig
	}{
		{"database", DatabaseRetryConfig()},
		{"http", HTTPRetryConfig()},
		{"s3", S3RetryConfig()},
		{"cache", CacheRetryConfig()},
		{"kafka", KafkaRetryConfig()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rm := NewRetryManager(tc.config, nil, logger)
			defer rm.Close()

			assert.NotNil(t, rm)
			assert.Equal(t, tc.config, rm.config)
		})
	}
}

func TestRetryManagerPool(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	pool := NewRetryManagerPool(logger)
	defer pool.Close()

	// Test getting managers for different operation types
	dbManager := pool.GetManager("database")
	httpManager := pool.GetManager("http")
	s3Manager := pool.GetManager("s3")

	assert.NotNil(t, dbManager)
	assert.NotNil(t, httpManager)
	assert.NotNil(t, s3Manager)

	// Test that same manager is returned for same operation type
	dbManager2 := pool.GetManager("database")
	assert.Equal(t, dbManager, dbManager2)

	// Test getting all managers
	allManagers := pool.GetAllManagers()
	assert.Len(t, allManagers, 3)
	assert.Contains(t, allManagers, "database")
	assert.Contains(t, allManagers, "http")
	assert.Contains(t, allManagers, "s3")
}

func TestRetryManagerPool_CustomManager(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	pool := NewRetryManagerPool(logger)
	defer pool.Close()

	// Add custom manager
	customConfig := DefaultRetryConfig()
	customConfig.OperationName = "custom"
	customManager := NewRetryManager(customConfig, nil, logger)
	pool.AddManager("custom", customManager)

	// Test getting custom manager
	retrievedManager := pool.GetManager("custom")
	assert.Equal(t, customManager, retrievedManager)
}

func TestRetryManagerPool_AggregatedMetrics(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	pool := NewRetryManagerPool(logger)
	defer pool.Close()

	// Execute some operations
	dbManager := pool.GetManager("database")
	fn := mockRetryableFunc(0, "success")
	_, err := dbManager.Execute(context.Background(), fn)
	require.NoError(t, err)

	httpManager := pool.GetManager("http")
	_, err = httpManager.Execute(context.Background(), fn)
	require.NoError(t, err)

	// Get aggregated metrics
	metrics := pool.GetAggregatedMetrics()
	assert.Len(t, metrics, 2)
	assert.Contains(t, metrics, "database")
	assert.Contains(t, metrics, "http")

	dbMetrics := metrics["database"]
	httpMetrics := metrics["http"]
	assert.Equal(t, uint64(1), dbMetrics.TotalOperations)
	assert.Equal(t, uint64(1), httpMetrics.TotalOperations)
}

// Benchmark tests

func BenchmarkRetryManager_SuccessfulOperation(b *testing.B) {
	logger, _ := logger.New(&logger.Config{Level: "error", Format: "json"})
	config := DefaultRetryConfig()
	config.EnableLogging = false
	config.EnableMetrics = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	fn := mockRetryableFunc(0, "success")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rm.Execute(context.Background(), fn)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRetryManager_RetryOperation(b *testing.B) {
	logger, _ := logger.New(&logger.Config{Level: "error", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 1 * time.Millisecond
	config.EnableLogging = false
	config.EnableMetrics = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	fn := mockRetryableFunc(2, "success") // Success on 3rd attempt

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rm.Execute(context.Background(), fn)
		if err != nil {
			b.Fatal(err)
		}
	}
}
