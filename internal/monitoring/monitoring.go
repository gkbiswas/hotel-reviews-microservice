package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// Config holds configuration for monitoring services
type Config struct {
	// Metrics configuration
	MetricsEnabled bool
	MetricsPath    string

	// Tracing configuration
	TracingEnabled      bool
	TracingServiceName  string
	TracingVersion      string
	TracingEnvironment  string
	JaegerEndpoint      string
	TracingSamplingRate float64

	// Health check configuration
	HealthEnabled       bool
	HealthPath          string
	HealthCheckInterval time.Duration

	// Business metrics configuration
	BusinessMetricsEnabled  bool
	BusinessMetricsInterval time.Duration

	// SLO monitoring configuration
	SLOMonitoringEnabled  bool
	SLOMonitoringInterval time.Duration
}

// Service represents the main monitoring service
type Service struct {
	config *Config
	logger *logrus.Logger

	// Core monitoring services
	metricsService  *MonitoringService
	tracingService  *TracingService
	healthService   *HealthService
	businessMetrics *BusinessMetrics
	sloManager      *SLOManager

	// Dependencies
	db          *gorm.DB
	redisClient *redis.Client
}

// NewService creates a new monitoring service
func NewService(config *Config, logger *logrus.Logger, db *gorm.DB, redisClient *redis.Client) (*Service, error) {
	service := &Service{
		config:      config,
		logger:      logger,
		db:          db,
		redisClient: redisClient,
	}

	// Initialize monitoring services
	if err := service.initializeServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize monitoring services: %w", err)
	}

	return service, nil
}

// initializeServices initializes all monitoring services
func (s *Service) initializeServices() error {
	// Initialize metrics service
	if s.config.MetricsEnabled {
		s.metricsService = NewMonitoringService(s.logger)
		s.logger.Info("Metrics service initialized")
	}

	// Initialize tracing service
	if s.config.TracingEnabled {
		tracingConfig := &TracingConfig{
			ServiceName:    s.config.TracingServiceName,
			ServiceVersion: s.config.TracingVersion,
			Environment:    s.config.TracingEnvironment,
			JaegerEndpoint: s.config.JaegerEndpoint,
			SamplingRate:   s.config.TracingSamplingRate,
			Enabled:        s.config.TracingEnabled,
		}

		var err error
		s.tracingService, err = NewTracingService(tracingConfig, s.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize tracing service: %w", err)
		}
		s.logger.Info("Tracing service initialized")
	}

	// Initialize health service
	if s.config.HealthEnabled {
		s.healthService = NewHealthService(s.logger)

		// Add health checkers
		if s.db != nil {
			dbChecker := NewDatabaseHealthChecker(s.db, "database", s.logger)
			s.healthService.AddChecker(dbChecker)
		}

		if s.redisClient != nil {
			redisChecker := NewRedisHealthChecker(s.redisClient, "redis", s.logger)
			s.healthService.AddChecker(redisChecker)
		}

		// Add S3 health checker (placeholder)
		s3Checker := NewS3HealthChecker("hotel-reviews-files", "s3", s.logger)
		s.healthService.AddChecker(s3Checker)

		s.logger.Info("Health service initialized")
	}

	// Initialize business metrics
	if s.config.BusinessMetricsEnabled {
		// Use the same registry as the metrics service
		var registry *prometheus.Registry
		if s.metricsService != nil {
			registry = s.metricsService.GetMetrics().GetRegistry()
		} else {
			registry = prometheus.NewRegistry()
		}
		s.businessMetrics = NewBusinessMetrics(s.logger, registry)
		s.logger.Info("Business metrics service initialized")
	}

	// Initialize SLO manager
	if s.config.SLOMonitoringEnabled && s.metricsService != nil {
		s.sloManager = NewSLOManager(s.metricsService.GetMetrics(), s.logger)
		s.logger.Info("SLO manager initialized")
	}

	return nil
}

// Start starts all monitoring services
func (s *Service) Start(ctx context.Context) error {
	// Start system metrics collection
	if s.metricsService != nil {
		s.metricsService.StartSystemMetricsCollector(ctx)
		s.logger.Info("System metrics collection started")
	}

	// Start periodic health checks
	if s.healthService != nil {
		s.healthService.StartPeriodicHealthChecks(ctx, s.config.HealthCheckInterval)
		s.logger.Info("Periodic health checks started")
	}

	// Start business metrics collection
	if s.businessMetrics != nil {
		s.businessMetrics.StartBusinessMetricsCollection(ctx)
		s.logger.Info("Business metrics collection started")
	}

	// Start SLO monitoring
	if s.sloManager != nil {
		s.sloManager.StartSLOMonitoring(ctx, s.config.SLOMonitoringInterval)
		s.logger.Info("SLO monitoring started")
	}

	return nil
}

// Stop gracefully stops all monitoring services
func (s *Service) Stop(ctx context.Context) error {
	// Stop tracing service
	if s.tracingService != nil {
		if err := s.tracingService.Close(ctx); err != nil {
			s.logger.WithError(err).Error("Failed to close tracing service")
		}
	}

	s.logger.Info("Monitoring services stopped")
	return nil
}

// GetMetricsService returns the metrics service
func (s *Service) GetMetricsService() *MonitoringService {
	return s.metricsService
}

// GetTracingService returns the tracing service
func (s *Service) GetTracingService() *TracingService {
	return s.tracingService
}

// GetHealthService returns the health service
func (s *Service) GetHealthService() *HealthService {
	return s.healthService
}

// GetBusinessMetrics returns the business metrics service
func (s *Service) GetBusinessMetrics() *BusinessMetrics {
	return s.businessMetrics
}

// GetSLOManager returns the SLO manager
func (s *Service) GetSLOManager() *SLOManager {
	return s.sloManager
}

// RegisterHTTPHandlers registers HTTP handlers for monitoring endpoints
func (s *Service) RegisterHTTPHandlers(router interface{}) {
	// Support both http.ServeMux and gorilla mux
	switch r := router.(type) {
	case *http.ServeMux:
		s.registerWithServeMux(r)
	default:
		// Assume it's a gorilla mux router
		s.registerWithGorillaMux(router)
	}
}

// registerWithServeMux registers handlers with http.ServeMux
func (s *Service) registerWithServeMux(mux *http.ServeMux) {
	// Metrics endpoint
	if s.metricsService != nil {
		mux.Handle(s.config.MetricsPath, promhttp.Handler())
		s.logger.Info("Metrics endpoint registered", "path", s.config.MetricsPath)
	}

	// Health endpoints
	if s.healthService != nil {
		mux.Handle(s.config.HealthPath, s.healthService.GetHTTPHandler())
		mux.Handle("/healthz", s.healthService.HealthzHandler())
		mux.Handle("/readiness", s.healthService.ReadinessHandler())
		mux.Handle("/liveness", s.healthService.LivenessHandler())
		s.logger.Info("Health endpoints registered", "path", s.config.HealthPath)
	}

	// SLO report endpoint
	if s.sloManager != nil {
		mux.HandleFunc("/slo-report", s.handleSLOReport)
		s.logger.Info("SLO report endpoint registered", "path", "/slo-report")
	}
}

// registerWithGorillaMux registers handlers with gorilla mux
func (s *Service) registerWithGorillaMux(router interface{}) {
	// Use reflection or type assertion to work with gorilla mux
	// For now, we'll use a simple approach
	if r, ok := router.(interface {
		Handle(string, http.Handler)
		HandleFunc(string, func(http.ResponseWriter, *http.Request))
	}); ok {
		// Metrics endpoint
		if s.metricsService != nil {
			r.Handle(s.config.MetricsPath, promhttp.Handler())
			s.logger.Info("Metrics endpoint registered", "path", s.config.MetricsPath)
		}

		// Health endpoints
		if s.healthService != nil {
			r.Handle(s.config.HealthPath, s.healthService.GetHTTPHandler())
			r.Handle("/healthz", s.healthService.HealthzHandler())
			r.Handle("/readiness", s.healthService.ReadinessHandler())
			r.Handle("/liveness", s.healthService.LivenessHandler())
			s.logger.Info("Health endpoints registered", "path", s.config.HealthPath)
		}

		// SLO report endpoint
		if s.sloManager != nil {
			r.HandleFunc("/slo-report", s.handleSLOReport)
			s.logger.Info("SLO report endpoint registered", "path", "/slo-report")
		}
	}
}

// handleSLOReport handles SLO report requests
func (s *Service) handleSLOReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if s.sloManager == nil {
		http.Error(w, "SLO manager not available", http.StatusServiceUnavailable)
		return
	}

	report, err := s.sloManager.GetSLOReport(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate SLO report: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Simple JSON response (in production, use a proper JSON library)
	fmt.Fprintf(w, `{
		"total_slos": %v,
		"violated_slos": %v,
		"slo_success_rate": %v,
		"timestamp": "%v"
	}`,
		report["total_slos"],
		report["violated_slos"],
		report["slo_success_rate"],
		report["timestamp"])
}

// RecordHTTPRequest records HTTP request metrics
func (s *Service) RecordHTTPRequest(method, endpoint, statusCode string, duration time.Duration, responseSize int64) {
	if s.metricsService != nil {
		s.metricsService.RecordHTTPRequest(method, endpoint, statusCode, duration, responseSize)
	}
}

// RecordReviewProcessed records review processing metrics
func (s *Service) RecordReviewProcessed(provider, status string, duration time.Duration) {
	if s.metricsService != nil {
		s.metricsService.RecordReviewProcessed(provider, status, duration)
	}
	if s.businessMetrics != nil {
		s.businessMetrics.RecordReviewIngestion(provider, "", "file")
	}
}

// RecordDatabaseQuery records database query metrics
func (s *Service) RecordDatabaseQuery(table, operation string, duration time.Duration, err error) {
	if s.metricsService != nil {
		s.metricsService.RecordDatabaseQuery(table, operation, duration, err)
	}
}

// RecordS3Operation records S3 operation metrics
func (s *Service) RecordS3Operation(operation, bucket, status string, duration time.Duration, objectSize int64) {
	if s.metricsService != nil {
		s.metricsService.RecordS3Operation(operation, bucket, status, duration, objectSize)
	}
}

// TraceHTTPRequest traces an HTTP request
func (s *Service) TraceHTTPRequest(ctx context.Context, method, path, userAgent string, fn func(ctx context.Context) error) error {
	if s.tracingService != nil {
		return s.tracingService.TraceHTTPRequest(ctx, method, path, userAgent, func(ctx context.Context, span trace.Span) error {
			return fn(ctx)
		})
	}
	return fn(ctx)
}

// TraceDatabaseQuery traces a database query
func (s *Service) TraceDatabaseQuery(ctx context.Context, table, operation, query string, fn func(ctx context.Context) error) error {
	if s.tracingService != nil {
		return s.tracingService.TraceDatabaseQuery(ctx, table, operation, query, func(ctx context.Context, span trace.Span) error {
			return fn(ctx)
		})
	}
	return fn(ctx)
}

// TraceS3Operation traces an S3 operation
func (s *Service) TraceS3Operation(ctx context.Context, operation, bucket, key string, fn func(ctx context.Context) error) error {
	if s.tracingService != nil {
		return s.tracingService.TraceS3Operation(ctx, operation, bucket, key, func(ctx context.Context, span trace.Span) error {
			return fn(ctx)
		})
	}
	return fn(ctx)
}

// TraceFileProcessing traces file processing operations
func (s *Service) TraceFileProcessing(ctx context.Context, provider, fileURL string, fn func(ctx context.Context) error) error {
	if s.tracingService != nil {
		return s.tracingService.TraceFileProcessing(ctx, provider, fileURL, func(ctx context.Context, span trace.Span) error {
			return fn(ctx)
		})
	}
	return fn(ctx)
}

// GetDefaultConfig returns default monitoring configuration
func GetDefaultConfig() *Config {
	return &Config{
		MetricsEnabled:          true,
		MetricsPath:             "/metrics",
		TracingEnabled:          true,
		TracingServiceName:      "hotel-reviews-api",
		TracingVersion:          "1.0.0",
		TracingEnvironment:      "development",
		JaegerEndpoint:          "http://localhost:14268/api/traces",
		TracingSamplingRate:     0.1,
		HealthEnabled:           true,
		HealthPath:              "/health",
		HealthCheckInterval:     30 * time.Second,
		BusinessMetricsEnabled:  true,
		BusinessMetricsInterval: 1 * time.Minute,
		SLOMonitoringEnabled:    true,
		SLOMonitoringInterval:   1 * time.Minute,
	}
}

// LoadConfigFromEnv loads monitoring configuration from environment variables
func LoadConfigFromEnv() *Config {
	config := GetDefaultConfig()

	// This would typically read from environment variables
	// For now, return default config
	return config
}
