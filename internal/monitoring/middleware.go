package monitoring

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware provides monitoring middleware for HTTP handlers
type HTTPMiddleware struct {
	service *Service
}

// NewHTTPMiddleware creates a new HTTP monitoring middleware
func NewHTTPMiddleware(service *Service) *HTTPMiddleware {
	return &HTTPMiddleware{
		service: service,
	}
}

// MetricsMiddleware records HTTP metrics for incoming requests
func (m *HTTPMiddleware) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.service.metricsService == nil {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		
		// Get route template if available
		route := mux.CurrentRoute(r)
		endpoint := r.URL.Path
		if route != nil {
			if template, err := route.GetPathTemplate(); err == nil {
				endpoint = template
			}
		}
		
		// Record request in flight
		m.service.metricsService.metrics.HTTPRequestsInFlight.WithLabelValues(r.Method, endpoint).Inc()
		defer m.service.metricsService.metrics.HTTPRequestsInFlight.WithLabelValues(r.Method, endpoint).Dec()
		
		// Wrap response writer to capture status code and response size
		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			responseSize:   0,
		}
		
		// Process request
		next.ServeHTTP(wrappedWriter, r)
		
		// Record metrics
		duration := time.Since(start)
		statusCode := strconv.Itoa(wrappedWriter.statusCode)
		
		m.service.RecordHTTPRequest(r.Method, endpoint, statusCode, duration, wrappedWriter.responseSize)
	})
}

// TracingMiddleware adds distributed tracing to HTTP requests
func (m *HTTPMiddleware) TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.service.tracingService == nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		
		// Extract trace context from headers
		ctx = m.service.tracingService.ExtractTraceContext(ctx, &HTTPHeaderCarrier{headers: r.Header})
		
		// Get route template if available
		route := mux.CurrentRoute(r)
		endpoint := r.URL.Path
		if route != nil {
			if template, err := route.GetPathTemplate(); err == nil {
				endpoint = template
			}
		}
		
		// Start span
		ctx, span := m.service.tracingService.StartSpan(ctx, r.Method+" "+endpoint,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.host", r.Host),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.remote_addr", r.RemoteAddr),
			),
		)
		defer span.End()
		
		// Add trace context to response headers
		m.service.tracingService.InjectTraceContext(ctx, &HTTPHeaderCarrier{headers: w.Header()})
		
		// Process request with tracing context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HealthCheckMiddleware provides health check functionality
func (m *HTTPMiddleware) HealthCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip health check for monitoring endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/healthz" || 
		   r.URL.Path == "/readiness" || r.URL.Path == "/liveness" ||
		   r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		
		// Check if service is healthy
		if m.service.healthService != nil && !m.service.healthService.IsHealthy() {
			// Log the unhealthy state but don't fail requests
			m.service.logger.Warn("Service is not healthy but processing request")
		}
		
		next.ServeHTTP(w, r)
	})
}

// BusinessMetricsMiddleware records business-specific metrics
func (m *HTTPMiddleware) BusinessMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.service.businessMetrics == nil {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		
		// Process request
		next.ServeHTTP(w, r)
		
		// Record business metrics based on the endpoint
		duration := time.Since(start)
		
		// Example: Record different metrics based on the endpoint
		switch r.URL.Path {
		case "/api/v1/reviews":
			if r.Method == "POST" {
				// This would be a review creation/ingestion
				m.service.businessMetrics.RecordReviewIngestion("api", "", "api")
			}
		case "/api/v1/process":
			if r.Method == "POST" {
				// This would be file processing
				m.service.businessMetrics.RecordFileProcessing("api", "manual", "success", duration, 0, 0)
			}
		}
	})
}

// CombinedMiddleware combines all monitoring middleware
func (m *HTTPMiddleware) CombinedMiddleware(next http.Handler) http.Handler {
	// Chain middleware in order: health -> tracing -> metrics -> business metrics
	return m.HealthCheckMiddleware(
		m.TracingMiddleware(
			m.MetricsMiddleware(
				m.BusinessMetricsMiddleware(next),
			),
		),
	)
}

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.responseSize += int64(size)
	return size, err
}

// HTTPHeaderCarrier implements the TextMapCarrier interface for HTTP headers
type HTTPHeaderCarrier struct {
	headers http.Header
}

// Get returns the value for the given key
func (h *HTTPHeaderCarrier) Get(key string) string {
	return h.headers.Get(key)
}

// Set sets the value for the given key
func (h *HTTPHeaderCarrier) Set(key, value string) {
	h.headers.Set(key, value)
}

// Keys returns all keys in the carrier
func (h *HTTPHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(h.headers))
	for k := range h.headers {
		keys = append(keys, k)
	}
	return keys
}

// DatabaseMiddleware provides database monitoring for GORM
type DatabaseMiddleware struct {
	service *Service
}

// NewDatabaseMiddleware creates a new database monitoring middleware
func NewDatabaseMiddleware(service *Service) *DatabaseMiddleware {
	return &DatabaseMiddleware{
		service: service,
	}
}

// GORMCallback provides GORM callback for database monitoring
func (m *DatabaseMiddleware) GORMCallback(tableName string) func(interface{}) {
	return func(tx interface{}) {
		// This would be implemented with actual GORM callback logic
		// For now, it's a placeholder
		start := time.Now()
		
		// After the query executes, record metrics
		duration := time.Since(start)
		m.service.RecordDatabaseQuery(tableName, "query", duration, nil)
	}
}

// FileProcessingMiddleware provides monitoring for file processing operations
type FileProcessingMiddleware struct {
	service *Service
}

// NewFileProcessingMiddleware creates a new file processing monitoring middleware
func NewFileProcessingMiddleware(service *Service) *FileProcessingMiddleware {
	return &FileProcessingMiddleware{
		service: service,
	}
}

// WrapFileProcessor wraps file processing operations with monitoring
func (m *FileProcessingMiddleware) WrapFileProcessor(provider, fileURL string, fn func(ctx context.Context) error) error {
	ctx := context.Background()
	
	return m.service.TraceFileProcessing(ctx, provider, fileURL, func(ctx context.Context) error {
		start := time.Now()
		
		err := fn(ctx)
		
		duration := time.Since(start)
		status := "success"
		if err != nil {
			status = "error"
		}
		
		// Record metrics
		m.service.RecordReviewProcessed(provider, status, duration)
		
		if m.service.businessMetrics != nil {
			m.service.businessMetrics.RecordFileProcessing(provider, "jsonl", status, duration, 0, 0)
		}
		
		return err
	})
}

// S3OperationMiddleware provides monitoring for S3 operations
type S3OperationMiddleware struct {
	service *Service
}

// NewS3OperationMiddleware creates a new S3 operation monitoring middleware
func NewS3OperationMiddleware(service *Service) *S3OperationMiddleware {
	return &S3OperationMiddleware{
		service: service,
	}
}

// WrapS3Operation wraps S3 operations with monitoring
func (m *S3OperationMiddleware) WrapS3Operation(ctx context.Context, operation, bucket, key string, fn func(ctx context.Context) error) error {
	return m.service.TraceS3Operation(ctx, operation, bucket, key, func(ctx context.Context) error {
		start := time.Now()
		
		err := fn(ctx)
		
		duration := time.Since(start)
		status := "success"
		if err != nil {
			status = "error"
		}
		
		// Record metrics
		m.service.RecordS3Operation(operation, bucket, status, duration, 0)
		
		return err
	})
}