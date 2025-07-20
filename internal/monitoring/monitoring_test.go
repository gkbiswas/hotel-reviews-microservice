package monitoring

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	assert.NotNil(t, config)
	assert.True(t, config.MetricsEnabled)
	assert.Equal(t, "/metrics", config.MetricsPath)
	assert.True(t, config.TracingEnabled)
	assert.Equal(t, "hotel-reviews-api", config.TracingServiceName)
	assert.Equal(t, "1.0.0", config.TracingVersion)
	assert.Equal(t, "development", config.TracingEnvironment)
	assert.Equal(t, "http://localhost:14268/api/traces", config.JaegerEndpoint)
	assert.Equal(t, 0.1, config.TracingSamplingRate)
	assert.True(t, config.HealthEnabled)
	assert.Equal(t, "/health", config.HealthPath)
	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	assert.True(t, config.BusinessMetricsEnabled)
	assert.Equal(t, 1*time.Minute, config.BusinessMetricsInterval)
	assert.True(t, config.SLOMonitoringEnabled)
	assert.Equal(t, 1*time.Minute, config.SLOMonitoringInterval)
}

func TestLoadConfigFromEnv(t *testing.T) {
	config := LoadConfigFromEnv()
	
	// Should return default config for now
	defaultConfig := GetDefaultConfig()
	assert.Equal(t, defaultConfig.MetricsEnabled, config.MetricsEnabled)
	assert.Equal(t, defaultConfig.TracingServiceName, config.TracingServiceName)
}

func TestNewService_MinimalConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled:         false,
		TracingEnabled:         false,
		HealthEnabled:          false,
		BusinessMetricsEnabled: false,
		SLOMonitoringEnabled:   false,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, config, service.config)
	assert.Nil(t, service.metricsService)
	assert.Nil(t, service.tracingService)
	assert.Nil(t, service.healthService)
	assert.Nil(t, service.businessMetrics)
	assert.Nil(t, service.sloManager)
}

func TestNewService_FullConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled:          true,
		MetricsPath:             "/metrics",
		TracingEnabled:          false, // Disable to avoid Jaeger connection
		HealthEnabled:           true,
		HealthPath:              "/health",
		HealthCheckInterval:     30 * time.Second,
		BusinessMetricsEnabled:  true,
		BusinessMetricsInterval: 1 * time.Minute,
		SLOMonitoringEnabled:    true,
		SLOMonitoringInterval:   1 * time.Minute,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.NotNil(t, service.metricsService)
	assert.Nil(t, service.tracingService) // Disabled
	assert.NotNil(t, service.healthService)
	assert.NotNil(t, service.businessMetrics)
	assert.NotNil(t, service.sloManager)
}

func TestNewService_TracingError(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		TracingEnabled:      true,
		TracingServiceName:  "test-service",
		TracingVersion:      "1.0.0",
		TracingEnvironment:  "test",
		JaegerEndpoint:      "invalid://endpoint",
		TracingSamplingRate: 1.0,
	}

	_, err := NewService(config, logger, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize monitoring services")
}

func TestService_GetServices(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled:         true,
		HealthEnabled:          true,
		BusinessMetricsEnabled: true,
		SLOMonitoringEnabled:   true,
		TracingEnabled:         false,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	assert.NotNil(t, service.GetMetricsService())
	assert.Nil(t, service.GetTracingService())
	assert.NotNil(t, service.GetHealthService())
	assert.NotNil(t, service.GetBusinessMetrics())
	assert.NotNil(t, service.GetSLOManager())
}

func TestService_Start(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled:          true,
		HealthEnabled:           true,
		HealthCheckInterval:     100 * time.Millisecond, // Short interval for testing
		BusinessMetricsEnabled:  true,
		BusinessMetricsInterval: 100 * time.Millisecond,
		SLOMonitoringEnabled:    true,
		SLOMonitoringInterval:   100 * time.Millisecond,
		TracingEnabled:          false,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = service.Start(ctx)
	assert.NoError(t, err)

	// Wait for some cycles
	time.Sleep(150 * time.Millisecond)

	// Stop the service
	err = service.Stop(ctx)
	assert.NoError(t, err)
}

func TestService_Stop(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		TracingEnabled: false, // No tracing to close
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = service.Stop(ctx)
	assert.NoError(t, err)
}

func TestService_RecordHTTPRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	// Record HTTP request
	service.RecordHTTPRequest("GET", "/api/reviews", "200", 100*time.Millisecond, 1024)

	// Should not panic with disabled service
	disabledService, err := NewService(&Config{MetricsEnabled: false}, logger, nil, nil)
	require.NoError(t, err)
	disabledService.RecordHTTPRequest("GET", "/api/reviews", "200", 100*time.Millisecond, 1024)
}

func TestService_RecordReviewProcessed(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled:         true,
		BusinessMetricsEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	// Record review processing
	service.RecordReviewProcessed("booking.com", "success", 500*time.Millisecond)

	// Should not panic with disabled services
	disabledService, err := NewService(&Config{
		MetricsEnabled:         false,
		BusinessMetricsEnabled: false,
	}, logger, nil, nil)
	require.NoError(t, err)
	disabledService.RecordReviewProcessed("booking.com", "success", 500*time.Millisecond)
}

func TestService_RecordDatabaseQuery(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	// Record database query
	service.RecordDatabaseQuery("reviews", "SELECT", 10*time.Millisecond, nil)
	service.RecordDatabaseQuery("reviews", "UPDATE", 15*time.Millisecond, assert.AnError)

	// Should not panic with disabled service
	disabledService, err := NewService(&Config{MetricsEnabled: false}, logger, nil, nil)
	require.NoError(t, err)
	disabledService.RecordDatabaseQuery("reviews", "SELECT", 10*time.Millisecond, nil)
}

func TestService_RecordS3Operation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	// Record S3 operation
	service.RecordS3Operation("GetObject", "test-bucket", "success", 200*time.Millisecond, 1024*1024)

	// Should not panic with disabled service
	disabledService, err := NewService(&Config{MetricsEnabled: false}, logger, nil, nil)
	require.NoError(t, err)
	disabledService.RecordS3Operation("GetObject", "test-bucket", "success", 200*time.Millisecond, 1024*1024)
}

func TestService_TraceHTTPRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		TracingEnabled: false, // Use noop tracer
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceHTTPRequest(ctx, "GET", "/api/reviews", "test-agent", func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)

	// Test with disabled tracing
	disabledService, err := NewService(&Config{TracingEnabled: false}, logger, nil, nil)
	require.NoError(t, err)

	executed = false
	err = disabledService.TraceHTTPRequest(ctx, "GET", "/api/reviews", "test-agent", func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestService_TraceDatabaseQuery(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		TracingEnabled: false,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceDatabaseQuery(ctx, "reviews", "SELECT", "SELECT * FROM reviews", func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestService_TraceS3Operation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		TracingEnabled: false,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceS3Operation(ctx, "GetObject", "test-bucket", "test-key", func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestService_TraceFileProcessing(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		TracingEnabled: false,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceFileProcessing(ctx, "booking.com", "https://example.com/file.csv", func(ctx context.Context) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestService_RegisterHTTPHandlers_ServeMux(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled: true,
		MetricsPath:    "/metrics",
		HealthEnabled:  true,
		HealthPath:     "/health",
		SLOMonitoringEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	mux := http.NewServeMux()
	service.RegisterHTTPHandlers(mux)

	// Test metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test health endpoint
	req = httptest.NewRequest("GET", "/health", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test healthz endpoint
	req = httptest.NewRequest("GET", "/healthz", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test readiness endpoint
	req = httptest.NewRequest("GET", "/readiness", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test liveness endpoint
	req = httptest.NewRequest("GET", "/liveness", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test SLO report endpoint
	req = httptest.NewRequest("GET", "/slo-report", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestService_RegisterHTTPHandlers_GorillaMux(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled:       true,
		MetricsPath:          "/metrics",
		HealthEnabled:        true,
		HealthPath:           "/health",
		SLOMonitoringEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	// Mock gorilla mux-like router
	mockRouter := &mockRouter{
		handlers: make(map[string]http.Handler),
		funcs:    make(map[string]func(http.ResponseWriter, *http.Request)),
	}

	service.RegisterHTTPHandlers(mockRouter)

	// Verify handlers were registered
	assert.NotNil(t, mockRouter.handlers["/metrics"])
	assert.NotNil(t, mockRouter.handlers["/health"])
	assert.NotNil(t, mockRouter.handlers["/healthz"])
	assert.NotNil(t, mockRouter.handlers["/readiness"])
	assert.NotNil(t, mockRouter.handlers["/liveness"])
	assert.NotNil(t, mockRouter.funcs["/slo-report"])
}

func TestService_HandleSLOReport(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		SLOMonitoringEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/slo-report", nil)
	rec := httptest.NewRecorder()

	service.handleSLOReport(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	
	body := rec.Body.String()
	assert.Contains(t, body, "total_slos")
	assert.Contains(t, body, "violated_slos")
	assert.Contains(t, body, "slo_success_rate")
	assert.Contains(t, body, "timestamp")
}

func TestService_InitializeServices_WithDependencies(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	// Create mock database and Redis client
	// Note: In a real test, you would use actual mock implementations
	// For this test, we'll pass nil and verify the service handles it gracefully

	config := &Config{
		HealthEnabled: true,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, service.healthService)
}

func TestService_ConcurrentOperations(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		MetricsEnabled:         true,
		BusinessMetricsEnabled: true,
		TracingEnabled:         false,
	}

	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	done := make(chan bool)

	// Run concurrent operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 10; j++ {
				service.RecordHTTPRequest("GET", "/test", "200", time.Millisecond, 1024)
				service.RecordReviewProcessed("provider", "success", time.Millisecond)
				service.RecordDatabaseQuery("table", "select", time.Millisecond, nil)
				service.RecordS3Operation("get", "bucket", "success", time.Millisecond, 1024)

				_ = service.TraceHTTPRequest(ctx, "GET", "/test", "agent", func(ctx context.Context) error {
					return nil
				})
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// mockRouter implements the interface expected by registerWithGorillaMux
type mockRouter struct {
	handlers map[string]http.Handler
	funcs    map[string]func(http.ResponseWriter, *http.Request)
}

func (m *mockRouter) Handle(path string, handler http.Handler) {
	m.handlers[path] = handler
}

func (m *mockRouter) HandleFunc(path string, handlerFunc func(http.ResponseWriter, *http.Request)) {
	m.funcs[path] = handlerFunc
}

// Test configuration validation
func TestConfig_Validation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	tests := []struct {
		name   string
		config *Config
		valid  bool
	}{
		{
			name: "valid minimal config",
			config: &Config{
				MetricsEnabled: false,
				TracingEnabled: false,
				HealthEnabled:  false,
			},
			valid: true,
		},
		{
			name: "valid full config",
			config: &Config{
				MetricsEnabled:          true,
				MetricsPath:             "/metrics",
				TracingEnabled:          false, // Disable to avoid connection
				HealthEnabled:           true,
				HealthPath:              "/health",
				HealthCheckInterval:     30 * time.Second,
				BusinessMetricsEnabled:  true,
				BusinessMetricsInterval: 1 * time.Minute,
				SLOMonitoringEnabled:    true,
				SLOMonitoringInterval:   1 * time.Minute,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(tt.config, logger, nil, nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, service)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestService_NilDependencies(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &Config{
		HealthEnabled: true,
	}

	// Test with nil database and Redis client
	service, err := NewService(config, logger, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.NotNil(t, service.healthService)

	// Health service should still be functional with minimal checkers
	ctx := context.Background()
	results := service.healthService.CheckAll(ctx)
	assert.NotEmpty(t, results) // Should have at least S3 checker
}