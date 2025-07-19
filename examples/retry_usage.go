package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Example demonstrates comprehensive usage of the retry mechanism

func main() {
	// Initialize logger
	logger := logger.NewLogger(&logger.Config{
		Level:  "info",
		Format: "json",
	})

	// Example 1: Basic retry with default configuration
	fmt.Println("=== Example 1: Basic Retry ===")
	basicRetryExample(logger)

	// Example 2: Database operations with custom retry
	fmt.Println("\n=== Example 2: Database Operations ===")
	databaseRetryExample(logger)

	// Example 3: HTTP requests with circuit breaker
	fmt.Println("\n=== Example 3: HTTP Requests with Circuit Breaker ===")
	httpRetryExample(logger)

	// Example 4: S3 operations with dead letter queue
	fmt.Println("\n=== Example 4: S3 Operations with Dead Letter Queue ===")
	s3RetryExample(logger)

	// Example 5: Custom retry strategies
	fmt.Println("\n=== Example 5: Custom Retry Strategies ===")
	customRetryExample(logger)

	// Example 6: Retry manager pool
	fmt.Println("\n=== Example 6: Retry Manager Pool ===")
	retryPoolExample(logger)

	// Example 7: Metrics and monitoring
	fmt.Println("\n=== Example 7: Metrics and Monitoring ===")
	metricsExample(logger)
}

// Example 1: Basic retry with default configuration
func basicRetryExample(logger *logger.Logger) {
	config := infrastructure.DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 100 * time.Millisecond
	config.OperationName = "basic_operation"

	rm := infrastructure.NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Simulate an operation that fails twice then succeeds
	attemptCount := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attemptCount++
		if attemptCount <= 2 {
			return nil, errors.New("temporary failure")
		}
		return "success", nil
	}

	result, err := rm.Execute(context.Background(), operation)
	if err != nil {
		log.Printf("Operation failed: %v", err)
	} else {
		log.Printf("Operation succeeded: %v", result)
	}

	// Print metrics
	metrics := rm.GetMetrics()
	log.Printf("Total operations: %d, Retries: %d, Success rate: %.2f%%",
		metrics.TotalOperations, metrics.TotalRetries, metrics.SuccessRate)
}

// Example 2: Database operations with custom retry
func databaseRetryExample(logger *logger.Logger) {
	config := infrastructure.DatabaseRetryConfig()
	config.OperationName = "database_query"
	config.Tags = map[string]string{
		"table": "hotels",
		"query": "SELECT",
	}

	rm := infrastructure.NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Simulate database operation
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		// Simulate database query
		if attempt == 1 {
			return nil, errors.New("connection timeout")
		}
		return []map[string]interface{}{
			{"id": 1, "name": "Hotel A"},
			{"id": 2, "name": "Hotel B"},
		}, nil
	}

	result, err := rm.Execute(context.Background(), operation)
	if err != nil {
		log.Printf("Database operation failed: %v", err)
	} else {
		log.Printf("Database operation succeeded: %v", result)
	}
}

// Example 3: HTTP requests with circuit breaker
func httpRetryExample(logger *logger.Logger) {
	// Create circuit breaker
	cbConfig := infrastructure.DefaultCircuitBreakerConfig()
	cbConfig.Name = "http_client"
	cbConfig.FailureThreshold = 3
	cbConfig.EnableLogging = true
	circuitBreaker := infrastructure.NewCircuitBreaker(cbConfig, logger)
	defer circuitBreaker.Close()

	config := infrastructure.HTTPRetryConfig()
	config.OperationName = "http_request"
	config.EnableCircuitBreaker = true
	config.Tags = map[string]string{
		"endpoint": "/api/reviews",
		"method":   "GET",
	}

	rm := infrastructure.NewRetryManager(config, circuitBreaker, logger)
	defer rm.Close()

	// Simulate HTTP request
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		// Simulate HTTP client
		if attempt <= 2 {
			return nil, errors.New("500 internal server error")
		}
		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
		}, nil
	}

	result, err := rm.Execute(context.Background(), operation)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
	} else {
		response := result.(*http.Response)
		log.Printf("HTTP request succeeded: %s", response.Status)
	}
}

// Example 4: S3 operations with dead letter queue
func s3RetryExample(logger *logger.Logger) {
	config := infrastructure.S3RetryConfig()
	config.OperationName = "s3_upload"
	config.EnableDeadLetter = true
	config.Tags = map[string]string{
		"bucket": "hotel-reviews-data",
		"key":    "reviews/2024/01/data.json",
	}

	// Setup dead letter handler
	config.DeadLetterHandler = func(ctx context.Context, operation string, err error, attempts int, metadata map[string]interface{}) {
		log.Printf("Dead letter: Operation=%s, Error=%v, Attempts=%d, Metadata=%+v",
			operation, err, attempts, metadata)

		// In a real scenario, you might:
		// 1. Send to a dead letter queue (SQS, Kafka, etc.)
		// 2. Store in database for later processing
		// 3. Send alert to monitoring system
		// 4. Write to a file for manual review
	}

	rm := infrastructure.NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Simulate S3 upload that always fails
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		return nil, errors.New("access denied")
	}

	result, err := rm.Execute(context.Background(), operation)
	if err != nil {
		log.Printf("S3 upload failed: %v", err)
	} else {
		log.Printf("S3 upload succeeded: %v", result)
	}
}

// Example 5: Custom retry strategies
func customRetryExample(logger *logger.Logger) {
	config := infrastructure.DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 100 * time.Millisecond
	config.Strategy = infrastructure.StrategyCustom
	config.OperationName = "custom_operation"

	// Custom backoff function: quadratic backoff with cap
	config.CustomBackoff = func(attempt int, baseDelay time.Duration) time.Duration {
		delay := time.Duration(attempt*attempt) * baseDelay
		maxDelay := 5 * time.Second
		if delay > maxDelay {
			delay = maxDelay
		}
		return delay
	}

	// Custom retry condition: only retry network errors
	config.CustomConditions = []infrastructure.RetryCondition{
		func(err error, attempt int) bool {
			// Only retry if it's a network-related error
			return err != nil &&
				(contains(err.Error(), "network") ||
					contains(err.Error(), "connection") ||
					contains(err.Error(), "timeout"))
		},
	}

	rm := infrastructure.NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Test with network error (should retry)
	operation1 := func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt <= 2 {
			return nil, errors.New("network connection failed")
		}
		return "success", nil
	}

	result, err := rm.Execute(context.Background(), operation1)
	if err != nil {
		log.Printf("Network operation failed: %v", err)
	} else {
		log.Printf("Network operation succeeded: %v", result)
	}

	// Test with non-network error (should not retry)
	operation2 := func(ctx context.Context, attempt int) (interface{}, error) {
		return nil, errors.New("invalid input")
	}

	result, err = rm.Execute(context.Background(), operation2)
	if err != nil {
		log.Printf("Invalid input operation failed (expected): %v", err)
	} else {
		log.Printf("Invalid input operation succeeded: %v", result)
	}
}

// Example 6: Retry manager pool
func retryPoolExample(logger *logger.Logger) {
	pool := infrastructure.NewRetryManagerPool(logger)
	defer pool.Close()

	// Get different managers for different operation types
	dbManager := pool.GetManager("database")
	httpManager := pool.GetManager("http")
	s3Manager := pool.GetManager("s3")

	// Execute operations with different managers
	operations := []struct {
		name    string
		manager *infrastructure.RetryManager
		op      infrastructure.RetryableFunc
	}{
		{
			name:    "database",
			manager: dbManager,
			op: func(ctx context.Context, attempt int) (interface{}, error) {
				if attempt == 1 {
					return nil, errors.New("connection timeout")
				}
				return "db_result", nil
			},
		},
		{
			name:    "http",
			manager: httpManager,
			op: func(ctx context.Context, attempt int) (interface{}, error) {
				if attempt <= 2 {
					return nil, errors.New("502 bad gateway")
				}
				return "http_result", nil
			},
		},
		{
			name:    "s3",
			manager: s3Manager,
			op: func(ctx context.Context, attempt int) (interface{}, error) {
				if attempt == 1 {
					return nil, errors.New("rate limit exceeded")
				}
				return "s3_result", nil
			},
		},
	}

	// Execute all operations concurrently
	results := make(chan string, len(operations))
	for _, op := range operations {
		go func(name string, manager *infrastructure.RetryManager, operation infrastructure.RetryableFunc) {
			result, err := manager.Execute(context.Background(), operation)
			if err != nil {
				results <- fmt.Sprintf("%s failed: %v", name, err)
			} else {
				results <- fmt.Sprintf("%s succeeded: %v", name, result)
			}
		}(op.name, op.manager, op.op)
	}

	// Collect results
	for i := 0; i < len(operations); i++ {
		log.Printf("Result: %s", <-results)
	}

	// Print aggregated metrics
	metrics := pool.GetAggregatedMetrics()
	for operationType, metric := range metrics {
		log.Printf("Operation %s: Success rate: %.2f%%, Retries: %d",
			operationType, metric.SuccessRate, metric.TotalRetries)
	}
}

// Example 7: Metrics and monitoring
func metricsExample(logger *logger.Logger) {
	config := infrastructure.DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 50 * time.Millisecond
	config.OperationName = "metrics_demo"
	config.EnableMetrics = true

	rm := infrastructure.NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Execute various operations to generate metrics
	operations := []infrastructure.RetryableFunc{
		// Success on first attempt
		func(ctx context.Context, attempt int) (interface{}, error) {
			return "success", nil
		},
		// Success on second attempt
		func(ctx context.Context, attempt int) (interface{}, error) {
			if attempt == 1 {
				return nil, errors.New("temporary failure")
			}
			return "success", nil
		},
		// Success on third attempt
		func(ctx context.Context, attempt int) (interface{}, error) {
			if attempt <= 2 {
				return nil, errors.New("temporary failure")
			}
			return "success", nil
		},
		// Always fails
		func(ctx context.Context, attempt int) (interface{}, error) {
			return nil, errors.New("permanent failure")
		},
	}

	// Execute operations
	for i, op := range operations {
		result, err := rm.Execute(context.Background(), op)
		if err != nil {
			log.Printf("Operation %d failed: %v", i+1, err)
		} else {
			log.Printf("Operation %d succeeded: %v", i+1, result)
		}
	}

	// Print detailed metrics
	metrics := rm.GetMetrics()
	log.Println("=== Detailed Metrics ===")
	log.Printf("Total Operations: %d", metrics.TotalOperations)
	log.Printf("Successful Operations: %d", metrics.SuccessfulOperations)
	log.Printf("Failed Operations: %d", metrics.FailedOperations)
	log.Printf("Total Retries: %d", metrics.TotalRetries)
	log.Printf("Success Rate: %.2f%%", metrics.SuccessRate)
	log.Printf("Failure Rate: %.2f%%", metrics.FailureRate)
	log.Printf("Average Retries per Operation: %.2f", metrics.AverageRetriesPerOperation)
	log.Printf("Average Duration: %v", metrics.AverageDuration)
	log.Printf("Average Backoff Time: %v", metrics.AverageBackoffTime)
	log.Printf("Dead Letter Count: %d", metrics.DeadLetterCount)

	// Print retries by error type
	log.Println("=== Retries by Error Type ===")
	for errorType, count := range metrics.RetriesByErrorType {
		log.Printf("%s: %d", errorType.String(), count)
	}

	// Print retries by attempt
	log.Println("=== Retries by Attempt ===")
	for attempt, count := range metrics.RetriesByAttempt {
		log.Printf("Attempt %d: %d", attempt, count)
	}

	// Print operation-specific metrics
	opMetrics := rm.GetOperationMetrics("metrics_demo", "unknown")
	if opMetrics != nil {
		log.Println("=== Operation Metrics ===")
		log.Printf("Operation Name: %s", opMetrics.OperationName)
		log.Printf("Operation Type: %s", opMetrics.OperationType)
		log.Printf("Total Operations: %d", opMetrics.TotalOperations)
		log.Printf("Success Rate: %.2f%%", opMetrics.SuccessRate)
		log.Printf("Average Duration: %v", opMetrics.AverageDuration)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)/2-len(substr)/2:len(s)/2+len(substr)/2] == substr))
}

// Advanced examples for specific use cases

// Example: Retry with context propagation
func contextPropagationExample(logger *logger.Logger) {
	config := infrastructure.DefaultRetryConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 100 * time.Millisecond
	config.OperationName = "context_propagation"

	rm := infrastructure.NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Add custom values to context
	ctx = context.WithValue(ctx, "user_id", "12345")
	ctx = context.WithValue(ctx, "request_id", "req-abc-123")

	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		userID := ctx.Value("user_id")
		requestID := ctx.Value("request_id")

		log.Printf("Attempt %d: Processing for user %v, request %v", attempt, userID, requestID)

		if attempt <= 1 {
			return nil, errors.New("temporary failure")
		}

		return map[string]interface{}{
			"user_id":    userID,
			"request_id": requestID,
			"result":     "success",
		}, nil
	}

	result, err := rm.Execute(ctx, operation)
	if err != nil {
		log.Printf("Context propagation example failed: %v", err)
	} else {
		log.Printf("Context propagation example succeeded: %v", result)
	}
}

// Example: Retry with different strategies for different error types
func adaptiveRetryExample(logger *logger.Logger) {
	config := infrastructure.DefaultRetryConfig()
	config.MaxAttempts = 5
	config.BaseDelay = 100 * time.Millisecond
	config.OperationName = "adaptive_retry"

	// Custom retry condition based on error type
	config.CustomConditions = []infrastructure.RetryCondition{
		func(err error, attempt int) bool {
			if err == nil {
				return false
			}

			errorStr := err.Error()

			// Rate limit errors: use longer delays
			if contains(errorStr, "rate limit") {
				return attempt <= 3
			}

			// Network errors: retry more aggressively
			if contains(errorStr, "network") || contains(errorStr, "connection") {
				return attempt <= 5
			}

			// Server errors: moderate retry
			if contains(errorStr, "500") || contains(errorStr, "502") || contains(errorStr, "503") {
				return attempt <= 3
			}

			// Client errors: don't retry
			if contains(errorStr, "400") || contains(errorStr, "401") || contains(errorStr, "403") {
				return false
			}

			// Default: retry once
			return attempt <= 1
		},
	}

	rm := infrastructure.NewRetryManager(config, nil, logger)
	defer rm.Close()

	// Test different error types
	errorTypes := []string{
		"rate limit exceeded",
		"network connection failed",
		"500 internal server error",
		"400 bad request",
		"unknown error",
	}

	for _, errorType := range errorTypes {
		log.Printf("Testing error type: %s", errorType)

		operation := func(ctx context.Context, attempt int) (interface{}, error) {
			return nil, errors.New(errorType)
		}

		_, err := rm.Execute(context.Background(), operation)
		if err != nil {
			log.Printf("Error type %s failed as expected: %v", errorType, err)
		}
	}

	// Print metrics showing different retry patterns
	metrics := rm.GetMetrics()
	log.Printf("Adaptive retry metrics - Total retries: %d", metrics.TotalRetries)
}
