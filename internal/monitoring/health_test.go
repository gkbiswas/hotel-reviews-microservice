package monitoring

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock structures for testing

// Test DatabaseHealthChecker
func TestDatabaseHealthChecker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("constructor and name", func(t *testing.T) {
		checker := NewDatabaseHealthChecker(nil, "test-db", logger)
		assert.Equal(t, "test-db", checker.Name())
		assert.NotNil(t, checker)
	})

	t.Run("check with nil database", func(t *testing.T) {
		checker := NewDatabaseHealthChecker(nil, "test-db", logger)
		ctx := context.Background()
		result := checker.Check(ctx)

		// Should handle nil DB gracefully (will fail but not panic)
		assert.Equal(t, HealthStatusUnhealthy, result.Status)
		assert.Equal(t, "database connection is nil", result.Message)
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.ResponseTime)
	})
}

// Test RedisHealthChecker
func TestRedisHealthChecker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("constructor and name", func(t *testing.T) {
		// Test with nil client (we'll focus on the interface)
		checker := NewRedisHealthChecker(nil, "test-redis", logger)
		assert.Equal(t, "test-redis", checker.Name())
		assert.NotNil(t, checker)
	})

	// Note: Full Redis health check testing would require actual Redis instance
	// or more complex mocking. For now, we test the basic structure.
	t.Run("check with nil client", func(t *testing.T) {
		checker := NewRedisHealthChecker(nil, "test-redis", logger)
		ctx := context.Background()
		result := checker.Check(ctx)

		// Should handle nil client gracefully (will fail but not panic)
		assert.Equal(t, HealthStatusUnhealthy, result.Status)
		assert.Equal(t, "redis client is nil", result.Message)
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.ResponseTime)
	})
}

// Test S3HealthChecker
func TestS3HealthChecker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("constructor and name", func(t *testing.T) {
		checker := NewS3HealthChecker("test-bucket", "test-s3", logger)
		assert.Equal(t, "test-s3", checker.Name())
		assert.NotNil(t, checker)
	})

	t.Run("health check", func(t *testing.T) {
		checker := NewS3HealthChecker("test-bucket", "test-s3", logger)
		
		// Perform health check (will try to reach actual S3 and likely fail, but that's expected)
		ctx := context.Background()
		result := checker.Check(ctx)

		// Should return a result (likely unhealthy due to network/S3 access)
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.ResponseTime)
		// Status can be either healthy or unhealthy depending on network access
	})
}

// Test HTTPHealthChecker
func TestHTTPHealthChecker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("healthy endpoint", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create health checker
		checker := NewHTTPHealthChecker(server.URL, "test-http", logger)
		assert.Equal(t, "test-http", checker.Name())

		// Perform health check
		ctx := context.Background()
		result := checker.Check(ctx)

		// Assertions
		assert.Equal(t, HealthStatusHealthy, result.Status)
		assert.Equal(t, "http endpoint is healthy", result.Message)
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.ResponseTime)
		assert.Equal(t, server.URL, result.Details["url"])
		assert.Equal(t, http.StatusOK, result.Details["status_code"])
	})

	t.Run("unhealthy endpoint - 500 error", func(t *testing.T) {
		// Create test server that returns 500
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		// Create health checker
		checker := NewHTTPHealthChecker(server.URL, "test-http", logger)

		// Perform health check
		ctx := context.Background()
		result := checker.Check(ctx)

		// Assertions
		assert.Equal(t, HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "http endpoint returned status 500")
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.ResponseTime)
	})

	t.Run("unhealthy endpoint - connection refused", func(t *testing.T) {
		// Create health checker with invalid URL
		checker := NewHTTPHealthChecker("http://localhost:99999", "test-http", logger)

		// Perform health check
		ctx := context.Background()
		result := checker.Check(ctx)

		// Assertions
		assert.Equal(t, HealthStatusUnhealthy, result.Status)
		assert.Contains(t, result.Message, "http health check failed")
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.ResponseTime)
	})
}

// Test HealthService
func TestHealthService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("add checker and check all", func(t *testing.T) {
		service := NewHealthService(logger)

		// Create mock checker
		mockChecker := &mockHealthChecker{
			name: "test-checker",
			result: HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "test is healthy",
			},
		}

		// Add checker
		service.AddChecker(mockChecker)

		// Check all
		ctx := context.Background()
		results := service.CheckAll(ctx)

		// Assertions
		assert.Len(t, results, 1)
		assert.Equal(t, HealthStatusHealthy, results["test-checker"].Status)
		assert.Equal(t, "test is healthy", results["test-checker"].Message)

		// Verify last results
		lastResults := service.GetLastResults()
		assert.Len(t, lastResults, 1)
		assert.Equal(t, results["test-checker"], lastResults["test-checker"])
	})

	t.Run("is healthy with all healthy checks", func(t *testing.T) {
		service := NewHealthService(logger)

		// Add healthy checkers
		for i := 0; i < 3; i++ {
			service.AddChecker(&mockHealthChecker{
				name: strings.Repeat("checker", i+1),
				result: HealthCheckResult{
					Status:  HealthStatusHealthy,
					Message: "healthy",
				},
			})
		}

		// Check all
		ctx := context.Background()
		service.CheckAll(ctx)

		// Should be healthy
		assert.True(t, service.IsHealthy())
	})

	t.Run("is unhealthy with one unhealthy check", func(t *testing.T) {
		service := NewHealthService(logger)

		// Add mix of healthy and unhealthy checkers
		service.AddChecker(&mockHealthChecker{
			name: "healthy-checker",
			result: HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "healthy",
			},
		})
		service.AddChecker(&mockHealthChecker{
			name: "unhealthy-checker",
			result: HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "unhealthy",
			},
		})

		// Check all
		ctx := context.Background()
		service.CheckAll(ctx)

		// Should be unhealthy
		assert.False(t, service.IsHealthy())
	})

	t.Run("HTTP handler returns correct status", func(t *testing.T) {
		service := NewHealthService(logger)

		// Add healthy checker
		service.AddChecker(&mockHealthChecker{
			name: "test-checker",
			result: HealthCheckResult{
				Status:    HealthStatusHealthy,
				Message:   "healthy",
				Timestamp: time.Now(),
			},
		})

		// Get handler
		handler := service.GetHTTPHandler()

		// Create request
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		// Serve request
		handler.ServeHTTP(rec, req)

		// Assertions
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse response
		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, string(HealthStatusHealthy), response["status"])
		assert.NotNil(t, response["checks"])
	})

	t.Run("HTTP handler returns 503 for unhealthy", func(t *testing.T) {
		service := NewHealthService(logger)

		// Add unhealthy checker
		service.AddChecker(&mockHealthChecker{
			name: "test-checker",
			result: HealthCheckResult{
				Status:    HealthStatusUnhealthy,
				Message:   "unhealthy",
				Timestamp: time.Now(),
			},
		})

		// Get handler
		handler := service.GetHTTPHandler()

		// Create request
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		// Serve request
		handler.ServeHTTP(rec, req)

		// Assertions
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("healthz handler", func(t *testing.T) {
		service := NewHealthService(logger)

		// Add healthy checker
		service.AddChecker(&mockHealthChecker{
			name: "test-checker",
			result: HealthCheckResult{
				Status: HealthStatusHealthy,
			},
		})

		// Check all first
		ctx := context.Background()
		service.CheckAll(ctx)

		// Get handler
		handler := service.HealthzHandler()

		// Create request
		req := httptest.NewRequest("GET", "/healthz", nil)
		rec := httptest.NewRecorder()

		// Serve request
		handler.ServeHTTP(rec, req)

		// Assertions
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("readiness handler", func(t *testing.T) {
		service := NewHealthService(logger)

		// Add database checker (critical for readiness)
		service.AddChecker(&mockHealthChecker{
			name: "database",
			result: HealthCheckResult{
				Status: HealthStatusHealthy,
			},
		})

		// Get handler
		handler := service.ReadinessHandler()

		// Create request
		req := httptest.NewRequest("GET", "/ready", nil)
		rec := httptest.NewRecorder()

		// Serve request
		handler.ServeHTTP(rec, req)

		// Assertions
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "\"status\":\"ready\"")
	})

	t.Run("readiness handler - not ready", func(t *testing.T) {
		service := NewHealthService(logger)

		// Add unhealthy database checker
		service.AddChecker(&mockHealthChecker{
			name: "database",
			result: HealthCheckResult{
				Status: HealthStatusUnhealthy,
			},
		})

		// Get handler
		handler := service.ReadinessHandler()

		// Create request
		req := httptest.NewRequest("GET", "/ready", nil)
		rec := httptest.NewRecorder()

		// Serve request
		handler.ServeHTTP(rec, req)

		// Assertions
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "\"status\":\"not_ready\"")
	})

	t.Run("liveness handler", func(t *testing.T) {
		service := NewHealthService(logger)

		// Get handler
		handler := service.LivenessHandler()

		// Create request
		req := httptest.NewRequest("GET", "/live", nil)
		rec := httptest.NewRecorder()

		// Serve request
		handler.ServeHTTP(rec, req)

		// Assertions - always returns OK
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "\"status\":\"alive\"")
	})

	t.Run("periodic health checks start", func(t *testing.T) {
		service := NewHealthService(logger)

		service.AddChecker(&mockHealthChecker{
			name: "test-checker",
			result: HealthCheckResult{
				Status: HealthStatusHealthy,
			},
		})

		// Just verify the method doesn't panic
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		// This should not panic
		service.StartPeriodicHealthChecks(ctx, 100*time.Millisecond)
		
		// Verify we can manually call CheckAll
		results := service.CheckAll(ctx)
		assert.Len(t, results, 1)
	})
}

// Mock implementations for testing

type mockHealthChecker struct {
	name    string
	result  HealthCheckResult
	onCheck func()
}

func (m *mockHealthChecker) Name() string {
	return m.name
}

func (m *mockHealthChecker) Check(ctx context.Context) HealthCheckResult {
	if m.onCheck != nil {
		m.onCheck()
	}
	// Ensure timestamp is set
	if m.result.Timestamp.IsZero() {
		m.result.Timestamp = time.Now()
	}
	if m.result.ResponseTime == 0 {
		m.result.ResponseTime = 1 * time.Millisecond
	}
	return m.result
}


