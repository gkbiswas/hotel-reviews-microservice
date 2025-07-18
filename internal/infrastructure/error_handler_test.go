package infrastructure

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorHandler_Handle(t *testing.T) {
	// Create logger
	loggerInstance := logger.NewDefault()
	
	// Create error handler
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	ctx := context.Background()
	
	t.Run("Handle nil error", func(t *testing.T) {
		result := errorHandler.Handle(ctx, nil)
		assert.Nil(t, result)
	})
	
	t.Run("Handle AppError", func(t *testing.T) {
		appErr := &AppError{
			Type:      ErrorTypeValidation,
			Code:      "TEST_ERROR",
			Message:   "Test error message",
			Timestamp: time.Now(),
		}
		
		result := errorHandler.Handle(ctx, appErr)
		assert.NotNil(t, result)
		assert.Equal(t, appErr.Type, result.Type)
		assert.Equal(t, appErr.Code, result.Code)
		assert.Equal(t, appErr.Message, result.Message)
		assert.True(t, result.Logged)
	})
	
	t.Run("Handle regular error", func(t *testing.T) {
		regularErr := errors.New("regular error")
		
		result := errorHandler.Handle(ctx, regularErr)
		assert.NotNil(t, result)
		assert.Equal(t, regularErr.Error(), result.Message)
		assert.NotEmpty(t, result.ID)
		assert.NotEmpty(t, result.Code)
		assert.True(t, result.Logged)
	})
}

func TestErrorHandler_ClassifyError(t *testing.T) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	testCases := []struct {
		name           string
		error          error
		expectedType   ErrorType
		expectedStatus int
	}{
		{
			name:           "Timeout error",
			error:          errors.New("timeout occurred"),
			expectedType:   ErrorTypeTimeout,
			expectedStatus: http.StatusRequestTimeout,
		},
		{
			name:           "Network error",
			error:          errors.New("network connection failed"),
			expectedType:   ErrorTypeNetwork,
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "Database error",
			error:          errors.New("database connection failed"),
			expectedType:   ErrorTypeDatabase,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "Validation error",
			error:          errors.New("validation failed"),
			expectedType:   ErrorTypeValidation,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Authentication error",
			error:          errors.New("authentication failed"),
			expectedType:   ErrorTypeAuthentication,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Authorization error",
			error:          errors.New("authorization failed"),
			expectedType:   ErrorTypeAuthorization,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Not found error",
			error:          errors.New("not found"),
			expectedType:   ErrorTypeNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Conflict error",
			error:          errors.New("conflict detected"),
			expectedType:   ErrorTypeConflict,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Rate limit error",
			error:          errors.New("rate limit exceeded"),
			expectedType:   ErrorTypeRateLimit,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "Generic error",
			error:          errors.New("unknown error"),
			expectedType:   ErrorTypeSystem,
			expectedStatus: http.StatusInternalServerError,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := errorHandler.classifyError(tc.error)
			assert.Equal(t, tc.expectedType, pattern.Type)
			assert.Equal(t, tc.expectedStatus, pattern.HTTPStatus)
		})
	}
}

func TestErrorHandler_HTTPResponse(t *testing.T) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	ctx := context.Background()
	
	t.Run("JSON response", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		
		testErr := errors.New("test error")
		errorHandler.HandleHTTP(ctx, w, req, testErr)
		
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
		assert.NotEmpty(t, w.Header().Get("X-Error-ID"))
		assert.NotEmpty(t, w.Header().Get("X-Error-Type"))
	})
	
	t.Run("XML response", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", "application/xml")
		w := httptest.NewRecorder()
		
		testErr := errors.New("test error")
		errorHandler.HandleHTTP(ctx, w, req, testErr)
		
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/xml")
		assert.NotEmpty(t, w.Header().Get("X-Error-ID"))
		assert.NotEmpty(t, w.Header().Get("X-Error-Type"))
	})
	
	t.Run("Text response", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", "text/plain")
		w := httptest.NewRecorder()
		
		testErr := errors.New("test error")
		errorHandler.HandleHTTP(ctx, w, req, testErr)
		
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
		assert.NotEmpty(t, w.Header().Get("X-Error-ID"))
		assert.NotEmpty(t, w.Header().Get("X-Error-Type"))
	})
}

func TestErrorHandler_ErrorPatterns(t *testing.T) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	// Add custom pattern
	customPattern := ErrorPattern{
		Type:           ErrorTypeBusiness,
		MessagePattern: "custom error",
		HTTPStatus:     http.StatusBadRequest,
		Category:       CategoryPermanent,
		Severity:       SeverityMedium,
		Retryable:      false,
	}
	
	errorHandler.AddErrorPattern(customPattern)
	
	// Test custom pattern matching
	testErr := errors.New("custom error occurred")
	ctx := context.Background()
	result := errorHandler.Handle(ctx, testErr)
	
	assert.NotNil(t, result)
	assert.Equal(t, ErrorTypeBusiness, result.Type)
	assert.Equal(t, http.StatusBadRequest, result.HTTPStatus)
	assert.False(t, result.Retryable)
}

func TestErrorHandler_ErrorStats(t *testing.T) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	ctx := context.Background()
	
	// Handle multiple errors
	for i := 0; i < 5; i++ {
		testErr := errors.New("test error")
		errorHandler.Handle(ctx, testErr)
	}
	
	// Check error stats
	stats := errorHandler.GetErrorStats()
	assert.NotEmpty(t, stats)
	
	// Find system error stats
	found := false
	for key, metrics := range stats {
		if metrics.Count == 5 {
			found = true
			assert.Equal(t, int64(5), metrics.Count)
			assert.NotZero(t, metrics.FirstSeen)
			assert.NotZero(t, metrics.LastSeen)
			break
		}
	}
	assert.True(t, found, "Expected to find error stats with count 5")
}

func TestErrorHandler_HealthCheck(t *testing.T) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	// Initially should be healthy
	assert.True(t, errorHandler.IsHealthy())
	
	// Get health status
	healthStatus := errorHandler.GetHealthStatus()
	assert.NotNil(t, healthStatus)
	assert.Contains(t, healthStatus, "status")
	assert.Contains(t, healthStatus, "checks")
	assert.Contains(t, healthStatus, "metrics")
}

func TestAppError_Methods(t *testing.T) {
	t.Run("Error method", func(t *testing.T) {
		appErr := &AppError{
			Type:    ErrorTypeValidation,
			Code:    "TEST_ERROR",
			Message: "Test error message",
		}
		
		assert.Equal(t, "Test error message", appErr.Error())
	})
	
	t.Run("Error method with empty message", func(t *testing.T) {
		appErr := &AppError{
			Type: ErrorTypeValidation,
			Code: "TEST_ERROR",
		}
		
		expected := "error type: validation, code: TEST_ERROR"
		assert.Equal(t, expected, appErr.Error())
	})
	
	t.Run("IsRetryable method", func(t *testing.T) {
		// Test retryable error
		retryableErr := &AppError{
			Category:  CategoryRetryable,
			Retryable: false, // Category should override
		}
		assert.True(t, retryableErr.IsRetryable())
		
		// Test non-retryable error
		nonRetryableErr := &AppError{
			Category:  CategoryNonRetryable,
			Retryable: true, // Category should override
		}
		assert.False(t, nonRetryableErr.IsRetryable())
		
		// Test with Retryable flag
		retryableFlagErr := &AppError{
			Category:  CategoryPermanent,
			Retryable: true,
		}
		assert.True(t, retryableFlagErr.IsRetryable())
	})
	
	t.Run("IsCritical method", func(t *testing.T) {
		// Test critical severity
		criticalErr := &AppError{
			Severity: SeverityCritical,
		}
		assert.True(t, criticalErr.IsCritical())
		
		// Test critical category
		criticalCatErr := &AppError{
			Severity: SeverityMedium,
			Category: CategoryCritical,
		}
		assert.True(t, criticalCatErr.IsCritical())
		
		// Test non-critical
		nonCriticalErr := &AppError{
			Severity: SeverityLow,
			Category: CategoryTransient,
		}
		assert.False(t, nonCriticalErr.IsCritical())
	})
	
	t.Run("IsTemporary method", func(t *testing.T) {
		// Test temporary error
		tempErr := &AppError{
			Category: CategoryTransient,
		}
		assert.True(t, tempErr.IsTemporary())
		
		// Test non-temporary error
		nonTempErr := &AppError{
			Category: CategoryPermanent,
		}
		assert.False(t, nonTempErr.IsTemporary())
	})
	
	t.Run("ToJSON method", func(t *testing.T) {
		appErr := &AppError{
			Type:    ErrorTypeValidation,
			Code:    "TEST_ERROR",
			Message: "Test error message",
		}
		
		jsonData, err := appErr.ToJSON()
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonData)
		assert.Contains(t, string(jsonData), "TEST_ERROR")
	})
	
	t.Run("ToXML method", func(t *testing.T) {
		appErr := &AppError{
			Type:    ErrorTypeValidation,
			Code:    "TEST_ERROR",
			Message: "Test error message",
		}
		
		xmlData, err := appErr.ToXML()
		assert.NoError(t, err)
		assert.NotEmpty(t, xmlData)
		assert.Contains(t, string(xmlData), "TEST_ERROR")
	})
	
	t.Run("Unwrap method", func(t *testing.T) {
		originalErr := errors.New("original error")
		appErr := &AppError{
			Type:    ErrorTypeValidation,
			Code:    "TEST_ERROR",
			Message: "Test error message",
			Cause:   originalErr,
		}
		
		unwrapped := appErr.Unwrap()
		assert.Equal(t, originalErr, unwrapped)
	})
}

func TestErrorHandlerConfig(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		config := DefaultErrorHandlerConfig()
		
		assert.NotNil(t, config)
		assert.True(t, config.EnableMetrics)
		assert.True(t, config.EnableAlerting)
		assert.True(t, config.EnableStackTrace)
		assert.True(t, config.EnableDetailedLogging)
		assert.True(t, config.EnableErrorAggregation)
		assert.True(t, config.EnableRateLimiting)
		assert.Equal(t, 50, config.MaxStackTraceDepth)
		assert.Equal(t, 24*time.Hour, config.ErrorRetentionPeriod)
		assert.Equal(t, 30*time.Second, config.MetricsInterval)
		assert.Equal(t, 10, config.AlertingThreshold)
		assert.Equal(t, 5*time.Minute, config.AlertingWindow)
		assert.Equal(t, time.Minute, config.RateLimitWindow)
		assert.Equal(t, 100, config.RateLimitThreshold)
		assert.Equal(t, FormatJSON, config.DefaultFormat)
		assert.False(t, config.IncludeInternalErrors)
		assert.True(t, config.SanitizeUserData)
	})
}

func TestErrorHandler_Integration(t *testing.T) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	config.EnableMetrics = true
	config.EnableAlerting = true
	config.EnableErrorAggregation = true
	
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	ctx := context.Background()
	
	// Simulate multiple errors
	errors := []error{
		errors.New("database connection failed"),
		errors.New("timeout occurred"),
		errors.New("validation failed: email is required"),
		errors.New("authentication failed"),
		errors.New("rate limit exceeded"),
	}
	
	var handledErrors []*AppError
	for _, err := range errors {
		handledErr := errorHandler.Handle(ctx, err)
		handledErrors = append(handledErrors, handledErr)
	}
	
	// Verify all errors were handled
	assert.Len(t, handledErrors, 5)
	
	// Verify error stats
	stats := errorHandler.GetErrorStats()
	assert.NotEmpty(t, stats)
	
	// Verify health status
	healthStatus := errorHandler.GetHealthStatus()
	assert.NotNil(t, healthStatus)
	
	// Wait a bit for background processes
	time.Sleep(100 * time.Millisecond)
	
	// Check if error handler is still healthy
	isHealthy := errorHandler.IsHealthy()
	assert.NotNil(t, isHealthy) // Just verify it doesn't panic
}

func TestErrorHandler_Concurrency(t *testing.T) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	ctx := context.Background()
	
	// Run concurrent error handling
	const numGoroutines = 10
	const errorsPerGoroutine = 100
	
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()
			
			for j := 0; j < errorsPerGoroutine; j++ {
				testErr := errors.New("concurrent test error")
				result := errorHandler.Handle(ctx, testErr)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.ID)
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Goroutine timed out")
		}
	}
	
	// Verify error stats
	stats := errorHandler.GetErrorStats()
	assert.NotEmpty(t, stats)
	
	// Check total error count
	totalErrors := int64(0)
	for _, metrics := range stats {
		totalErrors += metrics.Count
	}
	assert.Equal(t, int64(numGoroutines*errorsPerGoroutine), totalErrors)
}

func BenchmarkErrorHandler_Handle(b *testing.B) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	ctx := context.Background()
	testErr := errors.New("benchmark test error")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		errorHandler.Handle(ctx, testErr)
	}
}

func BenchmarkErrorHandler_HandleHTTP(b *testing.B) {
	loggerInstance := logger.NewDefault()
	config := DefaultErrorHandlerConfig()
	errorHandler := NewErrorHandler(config, loggerInstance, nil, nil)
	defer errorHandler.Close()
	
	ctx := context.Background()
	testErr := errors.New("benchmark test error")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		errorHandler.HandleHTTP(ctx, w, req, testErr)
	}
}