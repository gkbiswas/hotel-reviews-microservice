package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsRegistry holds all Prometheus metrics
type MetricsRegistry struct {
	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPResponseSize     *prometheus.HistogramVec
	HTTPRequestsInFlight *prometheus.GaugeVec

	// Business metrics
	ReviewsProcessed   *prometheus.CounterVec
	ReviewsTotal       *prometheus.GaugeVec
	ProcessingErrors   *prometheus.CounterVec
	ProcessingDuration *prometheus.HistogramVec
	FileProcessingJobs *prometheus.GaugeVec

	// Database metrics
	DatabaseConnections   *prometheus.GaugeVec
	DatabaseQueries       *prometheus.CounterVec
	DatabaseQueryDuration *prometheus.HistogramVec
	DatabaseErrors        *prometheus.CounterVec

	// Cache metrics
	CacheHits       *prometheus.CounterVec
	CacheMisses     *prometheus.CounterVec
	CacheOperations *prometheus.CounterVec
	CacheSize       *prometheus.GaugeVec

	// S3 metrics
	S3Operations        *prometheus.CounterVec
	S3OperationDuration *prometheus.HistogramVec
	S3Errors            *prometheus.CounterVec
	S3ObjectSize        *prometheus.HistogramVec

	// System metrics
	GoRoutines  *prometheus.GaugeVec
	MemoryUsage *prometheus.GaugeVec
	CPUUsage    *prometheus.GaugeVec
	GCPauses    *prometheus.HistogramVec

	// Circuit breaker metrics
	CircuitBreakerState    *prometheus.GaugeVec
	CircuitBreakerRequests *prometheus.CounterVec
	CircuitBreakerFailures *prometheus.CounterVec

	// SLI/SLO metrics
	SLIAvailability *prometheus.GaugeVec
	SLILatency      *prometheus.HistogramVec
	SLIErrorRate    *prometheus.GaugeVec
	SLIThroughput   *prometheus.GaugeVec

	registry *prometheus.Registry
}

// NewMetricsRegistry creates a new metrics registry
func NewMetricsRegistry() *MetricsRegistry {
	registry := prometheus.NewRegistry()
	factory := promauto.With(registry)

	// Create custom metrics
	m := &MetricsRegistry{
		// HTTP metrics
		HTTPRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),
		HTTPRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		HTTPResponseSize: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_http_response_size_bytes",
				Help:    "Size of HTTP responses in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "endpoint"},
		),
		HTTPRequestsInFlight: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
			[]string{"method", "endpoint"},
		),

		// Business metrics
		ReviewsProcessed: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_processed_total",
				Help: "Total number of reviews processed",
			},
			[]string{"provider", "status"},
		),
		ReviewsTotal: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_total",
				Help: "Total number of reviews in the system",
			},
			[]string{"provider", "hotel_id"},
		),
		ProcessingErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_processing_errors_total",
				Help: "Total number of processing errors",
			},
			[]string{"provider", "error_type"},
		),
		ProcessingDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_processing_duration_seconds",
				Help:    "Duration of review processing in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
			},
			[]string{"provider", "operation"},
		),
		FileProcessingJobs: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_file_processing_jobs",
				Help: "Number of file processing jobs by status",
			},
			[]string{"status"},
		),

		// Database metrics
		DatabaseConnections: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_database_connections",
				Help: "Number of database connections",
			},
			[]string{"state"},
		),
		DatabaseQueries: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_database_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"table", "operation"},
		),
		DatabaseQueryDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_database_query_duration_seconds",
				Help:    "Duration of database queries in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"table", "operation"},
		),
		DatabaseErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_database_errors_total",
				Help: "Total number of database errors",
			},
			[]string{"table", "operation", "error_type"},
		),

		// Cache metrics
		CacheHits: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type", "key_type"},
		),
		CacheMisses: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type", "key_type"},
		),
		CacheOperations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_cache_operations_total",
				Help: "Total number of cache operations",
			},
			[]string{"cache_type", "operation"},
		),
		CacheSize: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_cache_size_bytes",
				Help: "Size of cache in bytes",
			},
			[]string{"cache_type"},
		),

		// S3 metrics
		S3Operations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_s3_operations_total",
				Help: "Total number of S3 operations",
			},
			[]string{"operation", "bucket", "status"},
		),
		S3OperationDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_s3_operation_duration_seconds",
				Help:    "Duration of S3 operations in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
			},
			[]string{"operation", "bucket"},
		),
		S3Errors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_s3_errors_total",
				Help: "Total number of S3 errors",
			},
			[]string{"operation", "bucket", "error_type"},
		),
		S3ObjectSize: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_s3_object_size_bytes",
				Help:    "Size of S3 objects in bytes",
				Buckets: prometheus.ExponentialBuckets(1024, 10, 8),
			},
			[]string{"operation", "bucket"},
		),

		// System metrics
		GoRoutines: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_goroutines",
				Help: "Number of goroutines",
			},
			[]string{"component"},
		),
		MemoryUsage: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_memory_usage_bytes",
				Help: "Memory usage in bytes",
			},
			[]string{"type"},
		),
		CPUUsage: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_cpu_usage_percent",
				Help: "CPU usage percentage",
			},
			[]string{"component"},
		),
		GCPauses: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_gc_pause_duration_seconds",
				Help:    "GC pause duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.00001, 2, 15),
			},
			[]string{"type"},
		),

		// Circuit breaker metrics
		CircuitBreakerState: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_circuit_breaker_state",
				Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
			},
			[]string{"component"},
		),
		CircuitBreakerRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_circuit_breaker_requests_total",
				Help: "Total number of circuit breaker requests",
			},
			[]string{"component", "state"},
		),
		CircuitBreakerFailures: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_circuit_breaker_failures_total",
				Help: "Total number of circuit breaker failures",
			},
			[]string{"component", "failure_type"},
		),

		// SLI/SLO metrics
		SLIAvailability: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_sli_availability",
				Help: "Service availability SLI (0-1)",
			},
			[]string{"service", "endpoint"},
		),
		SLILatency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_sli_latency_seconds",
				Help:    "Service latency SLI in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"service", "endpoint"},
		),
		SLIErrorRate: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_sli_error_rate",
				Help: "Service error rate SLI (0-1)",
			},
			[]string{"service", "endpoint"},
		),
		SLIThroughput: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_sli_throughput_requests_per_second",
				Help: "Service throughput SLI in requests per second",
			},
			[]string{"service", "endpoint"},
		),

		registry: registry,
	}

	// Metrics are automatically registered with the registry by promauto.With(registry)
	return m
}

// GetRegistry returns the Prometheus registry
func (m *MetricsRegistry) GetRegistry() *prometheus.Registry {
	return m.registry
}

// GetHandler returns the Prometheus HTTP handler
func (m *MetricsRegistry) GetHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          m.registry,
	})
}

// MonitoringService provides monitoring functionality
type MonitoringService struct {
	metrics *MetricsRegistry
	logger  *slog.Logger
}

// NewMonitoringService creates a new monitoring service
func NewMonitoringService(logger *slog.Logger) *MonitoringService {
	return &MonitoringService{
		metrics: NewMetricsRegistry(),
		logger:  logger,
	}
}

// GetMetrics returns the metrics registry
func (s *MonitoringService) GetMetrics() *MetricsRegistry {
	return s.metrics
}

// RecordHTTPRequest records HTTP request metrics
func (s *MonitoringService) RecordHTTPRequest(method, endpoint, statusCode string, duration time.Duration, responseSize int64) {
	s.metrics.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
	s.metrics.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	s.metrics.HTTPResponseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
}

// RecordReviewProcessed records review processing metrics
func (s *MonitoringService) RecordReviewProcessed(provider, status string, duration time.Duration) {
	s.metrics.ReviewsProcessed.WithLabelValues(provider, status).Inc()
	s.metrics.ProcessingDuration.WithLabelValues(provider, "process").Observe(duration.Seconds())
}

// RecordDatabaseQuery records database query metrics
func (s *MonitoringService) RecordDatabaseQuery(table, operation string, duration time.Duration, err error) {
	s.metrics.DatabaseQueries.WithLabelValues(table, operation).Inc()
	s.metrics.DatabaseQueryDuration.WithLabelValues(table, operation).Observe(duration.Seconds())

	if err != nil {
		errorType := "unknown"
		if err != nil {
			errorType = fmt.Sprintf("%T", err)
		}
		s.metrics.DatabaseErrors.WithLabelValues(table, operation, errorType).Inc()
	}
}

// RecordCacheOperation records cache operation metrics
func (s *MonitoringService) RecordCacheOperation(cacheType, operation string, hit bool) {
	s.metrics.CacheOperations.WithLabelValues(cacheType, operation).Inc()

	if hit {
		s.metrics.CacheHits.WithLabelValues(cacheType, operation).Inc()
	} else {
		s.metrics.CacheMisses.WithLabelValues(cacheType, operation).Inc()
	}
}

// RecordS3Operation records S3 operation metrics
func (s *MonitoringService) RecordS3Operation(operation, bucket, status string, duration time.Duration, objectSize int64) {
	s.metrics.S3Operations.WithLabelValues(operation, bucket, status).Inc()
	s.metrics.S3OperationDuration.WithLabelValues(operation, bucket).Observe(duration.Seconds())

	if objectSize > 0 {
		s.metrics.S3ObjectSize.WithLabelValues(operation, bucket).Observe(float64(objectSize))
	}
}

// RecordCircuitBreakerState records circuit breaker state
func (s *MonitoringService) RecordCircuitBreakerState(component string, state int) {
	s.metrics.CircuitBreakerState.WithLabelValues(component).Set(float64(state))
}

// RecordSLI records SLI metrics
func (s *MonitoringService) RecordSLI(service, endpoint string, availability, errorRate, throughput float64, latency time.Duration) {
	s.metrics.SLIAvailability.WithLabelValues(service, endpoint).Set(availability)
	s.metrics.SLIErrorRate.WithLabelValues(service, endpoint).Set(errorRate)
	s.metrics.SLIThroughput.WithLabelValues(service, endpoint).Set(throughput)
	s.metrics.SLILatency.WithLabelValues(service, endpoint).Observe(latency.Seconds())
}

// StartSystemMetricsCollector starts collecting system metrics
func (s *MonitoringService) StartSystemMetricsCollector(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.collectSystemMetrics()
			}
		}
	}()
}

// collectSystemMetrics collects system-level metrics
func (s *MonitoringService) collectSystemMetrics() {
	// This would typically collect real system metrics
	// For now, we'll just record some placeholder values
	s.metrics.GoRoutines.WithLabelValues("total").Set(float64(100))              // placeholder
	s.metrics.MemoryUsage.WithLabelValues("heap").Set(float64(1024 * 1024 * 50)) // placeholder
	s.metrics.CPUUsage.WithLabelValues("application").Set(float64(25.0))         // placeholder
}
