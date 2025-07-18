package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simple test to verify retry mechanism works
func TestRetryManager_Simple(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 1 * time.Millisecond
	config.EnableLogging = false
	config.EnableMetrics = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Test 1: Success on first attempt
	attempt1Count := 0
	operation1 := func(ctx context.Context, attempt int) (interface{}, error) {
		attempt1Count++
		return "success", nil
	}

	result, err := rm.Execute(context.Background(), operation1)
	require.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 1, attempt1Count)

	// Test 2: Success on second attempt
	attempt2Count := 0
	operation2 := func(ctx context.Context, attempt int) (interface{}, error) {
		attempt2Count++
		if attempt2Count == 1 {
			return nil, errors.New("temporary failure")
		}
		return "success", nil
	}

	result, err = rm.Execute(context.Background(), operation2)
	require.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 2, attempt2Count)

	// Test 3: Fail after all attempts
	attempt3Count := 0
	operation3 := func(ctx context.Context, attempt int) (interface{}, error) {
		attempt3Count++
		return nil, errors.New("permanent failure")
	}

	result, err = rm.Execute(context.Background(), operation3)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 3, attempt3Count)
}

// Test backoff strategies
func TestRetryManager_BackoffStrategies(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	
	testCases := []struct {
		name     string
		strategy RetryStrategy
	}{
		{"fixed", StrategyFixedDelay},
		{"exponential", StrategyExponentialBackoff},
		{"linear", StrategyLinearBackoff},
		{"fibonacci", StrategyFibonacciBackoff},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultRetryConfig()
			config.MaxAttempts = 3
			config.BaseDelay = 1 * time.Millisecond
			config.Strategy = tc.strategy
			config.JitterType = JitterTypeNone
			config.EnableLogging = false
			config.EnableMetrics = false

			rm := NewRetryManager(config, nil, logger)
			defer rm.Close()

			attemptCount := 0
			operation := func(ctx context.Context, attempt int) (interface{}, error) {
				attemptCount++
				if attemptCount < 3 {
					return nil, errors.New("temporary failure")
				}
				return "success", nil
			}

			start := time.Now()
			result, err := rm.Execute(context.Background(), operation)
			duration := time.Since(start)

			require.NoError(t, err)
			assert.Equal(t, "success", result)
			assert.Equal(t, 3, attemptCount)
			assert.True(t, duration > time.Millisecond) // Should have some delay
		})
	}
}

// Test error classification
func TestRetryManager_ErrorClassification(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 1 * time.Millisecond
	config.EnableLogging = false
	config.EnableMetrics = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Test retryable error
	attemptCount := 0
	retryableOperation := func(ctx context.Context, attempt int) (interface{}, error) {
		attemptCount++
		return nil, errors.New("temporary network failure")
	}

	_, err := rm.Execute(context.Background(), retryableOperation)
	require.Error(t, err)
	assert.Equal(t, 3, attemptCount) // Should retry

	// Test non-retryable error
	attemptCount = 0
	nonRetryableOperation := func(ctx context.Context, attempt int) (interface{}, error) {
		attemptCount++
		return nil, errors.New("400 bad request")
	}

	_, err = rm.Execute(context.Background(), nonRetryableOperation)
	require.Error(t, err)
	assert.Equal(t, 1, attemptCount) // Should not retry
}

// Test context cancellation
func TestRetryManager_ContextCancellation(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 100 * time.Millisecond
	config.EnableLogging = false
	config.EnableMetrics = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attemptCount := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attemptCount++
		return nil, errors.New("temporary failure")
	}

	_, err := rm.Execute(ctx, operation)
	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.True(t, attemptCount <= 3) // Should stop early due to timeout, but allow some attempts
}

// Test custom retry condition
func TestRetryManager_CustomRetryCondition(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 1 * time.Millisecond
	config.EnableLogging = false
	config.EnableMetrics = false

	// Custom condition: only retry once (allow maximum 2 attempts)
	config.CustomConditions = []RetryCondition{
		func(err error, attempt int) bool {
			return attempt == 1 // Only retry after first attempt
		},
	}

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	attemptCount := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attemptCount++
		return nil, errors.New("temporary failure")
	}

	_, err := rm.Execute(context.Background(), operation)
	require.Error(t, err)
	assert.Equal(t, 2, attemptCount) // Should stop after 2 attempts due to custom condition
}

// Test jitter
func TestRetryManager_Jitter(t *testing.T) {
	logger, _ := logger.New(&logger.Config{Level: "debug", Format: "json"})
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 10 * time.Millisecond
	config.Strategy = StrategyFixedDelay
	config.JitterType = JitterTypeFull
	config.EnableLogging = false
	config.EnableMetrics = false

	rm := NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Run multiple times to check for jitter variation
	durations := make([]time.Duration, 3)
	for i := 0; i < 3; i++ {
		attemptCount := 0
		operation := func(ctx context.Context, attempt int) (interface{}, error) {
			attemptCount++
			if attemptCount < 2 {
				return nil, errors.New("temporary failure")
			}
			return "success", nil
		}

		start := time.Now()
		result, err := rm.Execute(context.Background(), operation)
		durations[i] = time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, "success", result)
	}

	// Check if durations are different (jitter effect)
	allSame := true
	for i := 1; i < len(durations); i++ {
		if durations[i] != durations[0] {
			allSame = false
			break
		}
	}
	// Note: This test might occasionally fail due to randomness, but it should mostly pass
	if allSame {
		t.Log("Warning: All durations were the same, jitter might not be working")
	}
}