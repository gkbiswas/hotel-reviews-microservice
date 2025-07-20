package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status       HealthStatus           `json:"status"`
	Message      string                 `json:"message"`
	Timestamp    time.Time              `json:"timestamp"`
	ResponseTime time.Duration          `json:"response_time"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// HealthChecker interface for health checks
type HealthChecker interface {
	Check(ctx context.Context) HealthCheckResult
	Name() string
}

// DatabaseHealthChecker checks database health
type DatabaseHealthChecker struct {
	db     *gorm.DB
	name   string
	logger *logrus.Logger
}

// NewDatabaseHealthChecker creates a new database health checker
func NewDatabaseHealthChecker(db *gorm.DB, name string, logger *logrus.Logger) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		db:     db,
		name:   name,
		logger: logger,
	}
}

// Name returns the name of the health checker
func (h *DatabaseHealthChecker) Name() string {
	return h.name
}

// Check performs the health check
func (h *DatabaseHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	if h.db == nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      "database connection is nil",
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("failed to get database connection: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	// Check if database is alive
	if err := sqlDB.PingContext(ctx); err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("database ping failed: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	// Get database stats
	stats := sqlDB.Stats()
	details := map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use_connections":   stats.InUse,
		"idle_connections":     stats.Idle,
		"max_open_connections": stats.MaxOpenConnections,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration,
	}

	return HealthCheckResult{
		Status:       HealthStatusHealthy,
		Message:      "database is healthy",
		Timestamp:    time.Now(),
		ResponseTime: time.Since(start),
		Details:      details,
	}
}

// RedisHealthChecker checks Redis health
type RedisHealthChecker struct {
	client *redis.Client
	name   string
	logger *logrus.Logger
}

// NewRedisHealthChecker creates a new Redis health checker
func NewRedisHealthChecker(client *redis.Client, name string, logger *logrus.Logger) *RedisHealthChecker {
	return &RedisHealthChecker{
		client: client,
		name:   name,
		logger: logger,
	}
}

// Name returns the name of the health checker
func (h *RedisHealthChecker) Name() string {
	return h.name
}

// Check performs the health check
func (h *RedisHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	if h.client == nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      "redis client is nil",
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	// Ping Redis
	if err := h.client.Ping(ctx).Err(); err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("redis ping failed: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	// Get Redis info
	info := h.client.Info(ctx, "server", "memory", "stats")
	details := map[string]interface{}{
		"redis_version":     "unknown",
		"used_memory":       "unknown",
		"connected_clients": "unknown",
	}

	if info.Err() == nil {
		// Parse info for key metrics (simplified)
		details["info_available"] = true
	}

	return HealthCheckResult{
		Status:       HealthStatusHealthy,
		Message:      "redis is healthy",
		Timestamp:    time.Now(),
		ResponseTime: time.Since(start),
		Details:      details,
	}
}

// S3HealthChecker checks S3 health
type S3HealthChecker struct {
	bucketName string
	name       string
	logger     *logrus.Logger
	// We'll use a simple HTTP check for now
	httpClient *http.Client
}

// NewS3HealthChecker creates a new S3 health checker
func NewS3HealthChecker(bucketName, name string, logger *logrus.Logger) *S3HealthChecker {
	return &S3HealthChecker{
		bucketName: bucketName,
		name:       name,
		logger:     logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the name of the health checker
func (h *S3HealthChecker) Name() string {
	return h.name
}

// Check performs the health check
func (h *S3HealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	// For now, we'll just check if we can reach S3
	// In a real implementation, you would use the AWS SDK to check bucket access
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://s3.amazonaws.com", nil)
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("failed to create request: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("s3 health check failed: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("s3 returned status %d", resp.StatusCode),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	return HealthCheckResult{
		Status:       HealthStatusHealthy,
		Message:      "s3 is healthy",
		Timestamp:    time.Now(),
		ResponseTime: time.Since(start),
		Details: map[string]interface{}{
			"bucket":      h.bucketName,
			"status_code": resp.StatusCode,
		},
	}
}

// HTTPHealthChecker checks HTTP endpoint health
type HTTPHealthChecker struct {
	url        string
	name       string
	logger     *logrus.Logger
	httpClient *http.Client
}

// NewHTTPHealthChecker creates a new HTTP health checker
func NewHTTPHealthChecker(url, name string, logger *logrus.Logger) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		url:    url,
		name:   name,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the name of the health checker
func (h *HTTPHealthChecker) Name() string {
	return h.name
}

// Check performs the health check
func (h *HTTPHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", h.url, nil)
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("failed to create request: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("http health check failed: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("http endpoint returned status %d", resp.StatusCode),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	return HealthCheckResult{
		Status:       HealthStatusHealthy,
		Message:      "http endpoint is healthy",
		Timestamp:    time.Now(),
		ResponseTime: time.Since(start),
		Details: map[string]interface{}{
			"url":         h.url,
			"status_code": resp.StatusCode,
		},
	}
}

// HealthService manages health checks for all dependencies
type HealthService struct {
	checkers []HealthChecker
	logger   *logrus.Logger
	mu       sync.RWMutex
	results  map[string]HealthCheckResult
}

// NewHealthService creates a new health service
func NewHealthService(logger *logrus.Logger) *HealthService {
	return &HealthService{
		checkers: make([]HealthChecker, 0),
		logger:   logger,
		results:  make(map[string]HealthCheckResult),
	}
}

// AddChecker adds a health checker
func (s *HealthService) AddChecker(checker HealthChecker) {
	s.checkers = append(s.checkers, checker)
}

// CheckAll runs all health checks
func (s *HealthService) CheckAll(ctx context.Context) map[string]HealthCheckResult {
	results := make(map[string]HealthCheckResult)

	for _, checker := range s.checkers {
		result := checker.Check(ctx)
		results[checker.Name()] = result

		s.mu.Lock()
		s.results[checker.Name()] = result
		s.mu.Unlock()
	}

	return results
}

// GetLastResults returns the last health check results
func (s *HealthService) GetLastResults() map[string]HealthCheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make(map[string]HealthCheckResult)
	for name, result := range s.results {
		results[name] = result
	}

	return results
}

// IsHealthy returns true if all health checks are passing
func (s *HealthService) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, result := range s.results {
		if result.Status != HealthStatusHealthy {
			return false
		}
	}

	return true
}

// StartPeriodicHealthChecks starts periodic health checks
func (s *HealthService) StartPeriodicHealthChecks(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.CheckAll(ctx)
			}
		}
	}()
}

// GetHTTPHandler returns an HTTP handler for health checks
func (s *HealthService) GetHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		results := s.CheckAll(ctx)

		// Determine overall health
		overall := HealthStatusHealthy
		for _, result := range results {
			if result.Status != HealthStatusHealthy {
				overall = HealthStatusUnhealthy
				break
			}
		}

		// Set appropriate status code
		if overall == HealthStatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")

		response := map[string]interface{}{
			"status":    overall,
			"timestamp": time.Now(),
			"checks":    results,
		}

		// Encode response as JSON
		json.NewEncoder(w).Encode(response)
	})
}

// HealthzHandler returns a simple healthz handler
func (s *HealthService) HealthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.IsHealthy() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service Unavailable"))
		}
	})
}

// ReadinessHandler returns a readiness handler
func (s *HealthService) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if critical dependencies are ready
		ctx := r.Context()
		results := s.CheckAll(ctx)

		// For readiness, we might be more strict about which checks must pass
		ready := true
		for name, result := range results {
			// Consider database as critical for readiness
			if name == "database" && result.Status != HealthStatusHealthy {
				ready = false
				break
			}
		}

		if ready {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Not Ready"))
		}
	})
}

// LivenessHandler returns a liveness handler
func (s *HealthService) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Liveness check is typically more lenient
		// It should only fail if the application is completely broken
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Alive"))
	})
}
