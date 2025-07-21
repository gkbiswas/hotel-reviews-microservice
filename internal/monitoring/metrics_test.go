package monitoring

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsRegistry(t *testing.T) {
	registry := NewMetricsRegistry()

	// Verify registry is created
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.registry)

	// Verify all metrics are initialized
	assert.NotNil(t, registry.HTTPRequestsTotal)
	assert.NotNil(t, registry.HTTPRequestDuration)
	assert.NotNil(t, registry.HTTPResponseSize)
	assert.NotNil(t, registry.HTTPRequestsInFlight)
	assert.NotNil(t, registry.ReviewsProcessed)
	assert.NotNil(t, registry.ReviewsTotal)
	assert.NotNil(t, registry.ProcessingErrors)
	assert.NotNil(t, registry.ProcessingDuration)
	assert.NotNil(t, registry.FileProcessingJobs)
	assert.NotNil(t, registry.DatabaseConnections)
	assert.NotNil(t, registry.DatabaseQueries)
	assert.NotNil(t, registry.DatabaseQueryDuration)
	assert.NotNil(t, registry.DatabaseErrors)
	assert.NotNil(t, registry.CacheHits)
	assert.NotNil(t, registry.CacheMisses)
	assert.NotNil(t, registry.CacheOperations)
	assert.NotNil(t, registry.CacheSize)
	assert.NotNil(t, registry.S3Operations)
	assert.NotNil(t, registry.S3OperationDuration)
	assert.NotNil(t, registry.S3Errors)
	assert.NotNil(t, registry.S3ObjectSize)
	assert.NotNil(t, registry.GoRoutines)
	assert.NotNil(t, registry.MemoryUsage)
	assert.NotNil(t, registry.CPUUsage)
	assert.NotNil(t, registry.GCPauses)
	assert.NotNil(t, registry.CircuitBreakerState)
	assert.NotNil(t, registry.CircuitBreakerRequests)
	assert.NotNil(t, registry.CircuitBreakerFailures)
	assert.NotNil(t, registry.SLIAvailability)
	assert.NotNil(t, registry.SLILatency)
	assert.NotNil(t, registry.SLIErrorRate)
	assert.NotNil(t, registry.SLIThroughput)
}

func TestMetricsRegistry_GetRegistry(t *testing.T) {
	registry := NewMetricsRegistry()
	promRegistry := registry.GetRegistry()
	assert.NotNil(t, promRegistry)
	assert.Equal(t, registry.registry, promRegistry)
}

func TestMetricsRegistry_GetHandler(t *testing.T) {
	registry := NewMetricsRegistry()
	handler := registry.GetHandler()
	assert.NotNil(t, handler)

	// Initialize some metrics so they appear in the output
	registry.HTTPRequestsTotal.WithLabelValues("GET", "/test", "200").Inc()
	registry.ProcessingDuration.WithLabelValues("provider1", "operation1").Observe(0.5)

	// Test the handler serves metrics
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
	
	// Should contain some of our custom metrics
	body := rec.Body.String()
	assert.Contains(t, body, "hotel_reviews_http_requests_total")
	assert.Contains(t, body, "hotel_reviews_processing_duration_seconds")
}

func TestNewMonitoringService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)
	assert.NotNil(t, service)
	assert.NotNil(t, service.metrics)
	assert.NotNil(t, service.logger)
}

func TestMonitoringService_GetMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)
	metrics := service.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, service.metrics, metrics)
}

func TestMonitoringService_RecordHTTPRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Record some HTTP requests
	service.RecordHTTPRequest("GET", "/reviews", "200", 100*time.Millisecond, 1024)
	service.RecordHTTPRequest("POST", "/reviews", "201", 200*time.Millisecond, 2048)
	service.RecordHTTPRequest("GET", "/reviews", "404", 50*time.Millisecond, 512)

	// Verify counters
	counter, err := service.metrics.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/reviews", "200")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	counter, err = service.metrics.HTTPRequestsTotal.GetMetricWithLabelValues("POST", "/reviews", "201")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	counter, err = service.metrics.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/reviews", "404")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	// Verify histogram observations (we'll just check that they don't error)
	_, err = service.metrics.HTTPRequestDuration.GetMetricWithLabelValues("GET", "/reviews")
	require.NoError(t, err)

	// Verify response size observations
	_, err = service.metrics.HTTPResponseSize.GetMetricWithLabelValues("GET", "/reviews")
	require.NoError(t, err)
}

func TestMonitoringService_RecordReviewProcessed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Record review processing
	service.RecordReviewProcessed("booking.com", "success", 500*time.Millisecond)
	service.RecordReviewProcessed("expedia", "failure", 100*time.Millisecond)
	service.RecordReviewProcessed("booking.com", "success", 300*time.Millisecond)

	// Verify counters
	counter, err := service.metrics.ReviewsProcessed.GetMetricWithLabelValues("booking.com", "success")
	require.NoError(t, err)
	assert.Equal(t, float64(2), testutil.ToFloat64(counter))

	counter, err = service.metrics.ReviewsProcessed.GetMetricWithLabelValues("expedia", "failure")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	// Verify duration histogram
	_, err = service.metrics.ProcessingDuration.GetMetricWithLabelValues("booking.com", "process")
	require.NoError(t, err)
}

func TestMonitoringService_RecordDatabaseQuery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Record successful queries
	service.RecordDatabaseQuery("reviews", "select", 10*time.Millisecond, nil)
	service.RecordDatabaseQuery("hotels", "insert", 20*time.Millisecond, nil)
	
	// Record failed query
	service.RecordDatabaseQuery("reviews", "update", 5*time.Millisecond, assert.AnError)

	// Verify query counters
	counter, err := service.metrics.DatabaseQueries.GetMetricWithLabelValues("reviews", "select")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	counter, err = service.metrics.DatabaseQueries.GetMetricWithLabelValues("hotels", "insert")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	// Verify error counter
	errorCounter, err := service.metrics.DatabaseErrors.GetMetricWithLabelValues("reviews", "update", "*errors.errorString")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(errorCounter))

	// Verify duration histogram
	_, err = service.metrics.DatabaseQueryDuration.GetMetricWithLabelValues("reviews", "select")
	require.NoError(t, err)
}

func TestMonitoringService_RecordCacheOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Record cache operations
	service.RecordCacheOperation("redis", "get", true)  // hit
	service.RecordCacheOperation("redis", "get", false) // miss
	service.RecordCacheOperation("redis", "set", true)  // hit (for set operations)
	service.RecordCacheOperation("memory", "get", true) // hit

	// Verify operation counters
	counter, err := service.metrics.CacheOperations.GetMetricWithLabelValues("redis", "get")
	require.NoError(t, err)
	assert.Equal(t, float64(2), testutil.ToFloat64(counter))

	counter, err = service.metrics.CacheOperations.GetMetricWithLabelValues("redis", "set")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	// Verify hit counters
	hitCounter, err := service.metrics.CacheHits.GetMetricWithLabelValues("redis", "get")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(hitCounter))

	hitCounter, err = service.metrics.CacheHits.GetMetricWithLabelValues("memory", "get")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(hitCounter))

	// Verify miss counter
	missCounter, err := service.metrics.CacheMisses.GetMetricWithLabelValues("redis", "get")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(missCounter))
}

func TestMonitoringService_RecordS3Operation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Record S3 operations
	service.RecordS3Operation("get", "reviews-bucket", "success", 200*time.Millisecond, 1024*1024)
	service.RecordS3Operation("put", "reviews-bucket", "success", 300*time.Millisecond, 2048*1024)
	service.RecordS3Operation("get", "reviews-bucket", "failure", 50*time.Millisecond, 0)

	// Verify operation counters
	counter, err := service.metrics.S3Operations.GetMetricWithLabelValues("get", "reviews-bucket", "success")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	counter, err = service.metrics.S3Operations.GetMetricWithLabelValues("put", "reviews-bucket", "success")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	counter, err = service.metrics.S3Operations.GetMetricWithLabelValues("get", "reviews-bucket", "failure")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	// Verify duration histogram
	_, err = service.metrics.S3OperationDuration.GetMetricWithLabelValues("get", "reviews-bucket")
	require.NoError(t, err)

	// Verify object size histogram (only for non-zero sizes)
	_, err = service.metrics.S3ObjectSize.GetMetricWithLabelValues("get", "reviews-bucket")
	require.NoError(t, err)
}

func TestMonitoringService_RecordCircuitBreakerState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Record circuit breaker states
	service.RecordCircuitBreakerState("database", 0) // closed
	service.RecordCircuitBreakerState("s3", 1)       // open
	service.RecordCircuitBreakerState("redis", 2)    // half-open

	// Verify state gauges
	gauge, err := service.metrics.CircuitBreakerState.GetMetricWithLabelValues("database")
	require.NoError(t, err)
	assert.Equal(t, float64(0), testutil.ToFloat64(gauge))

	gauge, err = service.metrics.CircuitBreakerState.GetMetricWithLabelValues("s3")
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(gauge))

	gauge, err = service.metrics.CircuitBreakerState.GetMetricWithLabelValues("redis")
	require.NoError(t, err)
	assert.Equal(t, float64(2), testutil.ToFloat64(gauge))
}

func TestMonitoringService_RecordSLI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Record SLI metrics
	service.RecordSLI("api", "/reviews", 0.999, 0.001, 1000.0, 50*time.Millisecond)
	service.RecordSLI("api", "/hotels", 0.995, 0.005, 500.0, 100*time.Millisecond)

	// Verify availability gauge
	gauge, err := service.metrics.SLIAvailability.GetMetricWithLabelValues("api", "/reviews")
	require.NoError(t, err)
	assert.Equal(t, 0.999, testutil.ToFloat64(gauge))

	// Verify error rate gauge
	gauge, err = service.metrics.SLIErrorRate.GetMetricWithLabelValues("api", "/reviews")
	require.NoError(t, err)
	assert.Equal(t, 0.001, testutil.ToFloat64(gauge))

	// Verify throughput gauge
	gauge, err = service.metrics.SLIThroughput.GetMetricWithLabelValues("api", "/reviews")
	require.NoError(t, err)
	assert.Equal(t, 1000.0, testutil.ToFloat64(gauge))

	// Verify latency histogram
	_, err = service.metrics.SLILatency.GetMetricWithLabelValues("api", "/reviews")
	require.NoError(t, err)
}

func TestMonitoringService_StartSystemMetricsCollector(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start collector
	service.StartSystemMetricsCollector(ctx)

	// Manually trigger metrics collection since the ticker interval is 30s
	// which is much longer than our test timeout
	service.collectSystemMetrics()

	// Cancel context to stop collector
	cancel()

	// Give it time to clean up
	time.Sleep(10 * time.Millisecond)

	// Verify system metrics were set (from collectSystemMetrics)
	gauge, err := service.metrics.GoRoutines.GetMetricWithLabelValues("total")
	require.NoError(t, err)
	assert.Equal(t, float64(100), testutil.ToFloat64(gauge))

	gauge, err = service.metrics.MemoryUsage.GetMetricWithLabelValues("heap")
	require.NoError(t, err)
	assert.Equal(t, float64(1024*1024*50), testutil.ToFloat64(gauge))

	gauge, err = service.metrics.CPUUsage.GetMetricWithLabelValues("application")
	require.NoError(t, err)
	assert.Equal(t, float64(25.0), testutil.ToFloat64(gauge))
}

func TestMetricsRegistry_MetricNames(t *testing.T) {
	registry := NewMetricsRegistry()

	// Test that metrics have correct names
	tests := []struct {
		name     string
		metric   prometheus.Collector
		expected string
	}{
		{"HTTPRequestsTotal", registry.HTTPRequestsTotal, "hotel_reviews_http_requests_total"},
		{"ReviewsProcessed", registry.ReviewsProcessed, "hotel_reviews_processed_total"},
		{"DatabaseQueries", registry.DatabaseQueries, "hotel_reviews_database_queries_total"},
		{"CacheHits", registry.CacheHits, "hotel_reviews_cache_hits_total"},
		{"S3Operations", registry.S3Operations, "hotel_reviews_s3_operations_total"},
		{"CircuitBreakerState", registry.CircuitBreakerState, "hotel_reviews_circuit_breaker_state"},
		{"SLIAvailability", registry.SLIAvailability, "hotel_reviews_sli_availability"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get metric description
			ch := make(chan *prometheus.Desc, 1)
			tt.metric.Describe(ch)
			desc := <-ch
			close(ch)

			// Verify metric name
			assert.Contains(t, desc.String(), tt.expected)
		})
	}
}

func TestMetricsRegistry_HistogramBuckets(t *testing.T) {
	registry := NewMetricsRegistry()

	// Test histogram configurations - just verify they can be created and used
	tests := []struct {
		name       string
		histogram  *prometheus.HistogramVec
		labelValues []string
	}{
		{"HTTPRequestDuration", registry.HTTPRequestDuration, []string{"GET", "/test"}},
		{"HTTPResponseSize", registry.HTTPResponseSize, []string{"GET", "/test"}},
		{"ProcessingDuration", registry.ProcessingDuration, []string{"provider1", "operation1"}},
		{"DatabaseQueryDuration", registry.DatabaseQueryDuration, []string{"table1", "SELECT"}},
		{"S3OperationDuration", registry.S3OperationDuration, []string{"GET", "bucket1"}},
		{"SLILatency", registry.SLILatency, []string{"service1", "/endpoint"}},
		{"GCPauses", registry.GCPauses, []string{"STW"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a histogram with test values
			hist, err := tt.histogram.GetMetricWithLabelValues(tt.labelValues...)
			require.NoError(t, err)
			
			// Observe some values
			for i := 0; i < 10; i++ {
				hist.Observe(float64(i) / 10.0)
			}
			
			// Just verify it doesn't panic
			assert.NotNil(t, hist)
		})
	}
}

func TestMonitoringService_ConcurrentOperations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	service := NewMonitoringService(logger)

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				service.RecordHTTPRequest("GET", "/test", "200", time.Millisecond, 1024)
				service.RecordReviewProcessed("provider", "success", time.Millisecond)
				service.RecordDatabaseQuery("table", "select", time.Millisecond, nil)
				service.RecordCacheOperation("redis", "get", true)
				service.RecordS3Operation("get", "bucket", "success", time.Millisecond, 1024)
				service.RecordCircuitBreakerState("component", 0)
				service.RecordSLI("service", "/endpoint", 0.99, 0.01, 100.0, time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics were recorded
	counter, err := service.metrics.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/test", "200")
	require.NoError(t, err)
	assert.Equal(t, float64(1000), testutil.ToFloat64(counter))
}

