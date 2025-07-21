package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
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
	logger *slog.Logger
}

// NewDatabaseHealthChecker creates a new database health checker
func NewDatabaseHealthChecker(db *gorm.DB, name string, logger *slog.Logger) *DatabaseHealthChecker {
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
	logger *slog.Logger
}

// NewRedisHealthChecker creates a new Redis health checker
func NewRedisHealthChecker(client *redis.Client, name string, logger *slog.Logger) *RedisHealthChecker {
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
	logger     *slog.Logger
	// We'll use a simple HTTP check for now
	httpClient *http.Client
}

// NewS3HealthChecker creates a new S3 health checker
func NewS3HealthChecker(bucketName, name string, logger *slog.Logger) *S3HealthChecker {
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
	logger     *slog.Logger
	httpClient *http.Client
}

// NewHTTPHealthChecker creates a new HTTP health checker
func NewHTTPHealthChecker(url, name string, logger *slog.Logger) *HTTPHealthChecker {
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

// MemoryHealthChecker checks memory usage
type MemoryHealthChecker struct {
	name           string
	logger         *slog.Logger
	maxMemoryBytes uint64
}

// NewMemoryHealthChecker creates a new memory health checker
func NewMemoryHealthChecker(name string, maxMemoryBytes uint64, logger *slog.Logger) *MemoryHealthChecker {
	return &MemoryHealthChecker{
		name:           name,
		logger:         logger,
		maxMemoryBytes: maxMemoryBytes,
	}
}

// Name returns the name of the health checker
func (h *MemoryHealthChecker) Name() string {
	return h.name
}

// Check performs the memory health check
func (h *MemoryHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Convert bytes to MB for readability
	allocMB := memStats.Alloc / 1024 / 1024
	sysMB := memStats.Sys / 1024 / 1024
	maxMemoryMB := h.maxMemoryBytes / 1024 / 1024

	details := map[string]interface{}{
		"alloc_mb":        allocMB,
		"sys_mb":          sysMB,
		"max_memory_mb":   maxMemoryMB,
		"num_gc":          memStats.NumGC,
		"gc_pause_ns":     memStats.PauseTotalNs,
		"heap_objects":    memStats.HeapObjects,
		"stack_in_use_mb": memStats.StackInuse / 1024 / 1024,
	}

	status := HealthStatusHealthy
	message := "memory usage is within limits"

	if h.maxMemoryBytes > 0 && memStats.Alloc > h.maxMemoryBytes {
		status = HealthStatusUnhealthy
		message = fmt.Sprintf("memory usage (%d MB) exceeds limit (%d MB)", allocMB, maxMemoryMB)
	}

	return HealthCheckResult{
		Status:       status,
		Message:      message,
		Timestamp:    time.Now(),
		ResponseTime: time.Since(start),
		Details:      details,
	}
}

// DiskHealthChecker checks disk usage
type DiskHealthChecker struct {
	name        string
	logger      *slog.Logger
	path        string
	maxUsagePct float64
}

// NewDiskHealthChecker creates a new disk health checker
func NewDiskHealthChecker(name, path string, maxUsagePct float64, logger *slog.Logger) *DiskHealthChecker {
	return &DiskHealthChecker{
		name:        name,
		logger:      logger,
		path:        path,
		maxUsagePct: maxUsagePct,
	}
}

// Name returns the name of the health checker
func (h *DiskHealthChecker) Name() string {
	return h.name
}

// Check performs the disk health check
func (h *DiskHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	var stat syscall.Statfs_t
	err := syscall.Statfs(h.path, &stat)
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("failed to get disk stats: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	// Calculate disk usage
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes
	usagePct := float64(usedBytes) / float64(totalBytes) * 100

	totalGB := float64(totalBytes) / 1024 / 1024 / 1024
	freeGB := float64(freeBytes) / 1024 / 1024 / 1024
	usedGB := float64(usedBytes) / 1024 / 1024 / 1024

	details := map[string]interface{}{
		"path":        h.path,
		"total_gb":    fmt.Sprintf("%.2f", totalGB),
		"used_gb":     fmt.Sprintf("%.2f", usedGB),
		"free_gb":     fmt.Sprintf("%.2f", freeGB),
		"usage_pct":   fmt.Sprintf("%.2f", usagePct),
		"max_usage":   fmt.Sprintf("%.2f", h.maxUsagePct),
	}

	status := HealthStatusHealthy
	message := "disk usage is within limits"

	if usagePct > h.maxUsagePct {
		status = HealthStatusUnhealthy
		message = fmt.Sprintf("disk usage (%.2f%%) exceeds limit (%.2f%%)", usagePct, h.maxUsagePct)
	}

	return HealthCheckResult{
		Status:       status,
		Message:      message,
		Timestamp:    time.Now(),
		ResponseTime: time.Since(start),
		Details:      details,
	}
}

// KafkaHealthChecker checks Kafka health
type KafkaHealthChecker struct {
	name    string
	logger  *slog.Logger
	brokers []string
	config  *sarama.Config
}

// NewKafkaHealthChecker creates a new Kafka health checker
func NewKafkaHealthChecker(name string, brokers []string, config *sarama.Config, logger *slog.Logger) *KafkaHealthChecker {
	if config == nil {
		config = sarama.NewConfig()
		config.Version = sarama.V2_6_0_0
		config.Net.DialTimeout = 10 * time.Second
		config.Metadata.Timeout = 10 * time.Second
	}

	return &KafkaHealthChecker{
		name:    name,
		logger:  logger,
		brokers: brokers,
		config:  config,
	}
}

// Name returns the name of the health checker
func (h *KafkaHealthChecker) Name() string {
	return h.name
}

// Check performs the Kafka health check
func (h *KafkaHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()

	if len(h.brokers) == 0 {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      "no kafka brokers configured",
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	// Create a new client to test connectivity
	client, err := sarama.NewClient(h.brokers, h.config)
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("failed to connect to kafka: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}
	defer client.Close()

	// Get broker information
	brokers := client.Brokers()
	connectedBrokers := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		if connected, _ := broker.Connected(); connected {
			connectedBrokers = append(connectedBrokers, broker.Addr())
		}
	}

	// Get topics (basic check)
	topics, err := client.Topics()
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusUnhealthy,
			Message:      fmt.Sprintf("failed to get kafka topics: %v", err),
			Timestamp:    time.Now(),
			ResponseTime: time.Since(start),
		}
	}

	details := map[string]interface{}{
		"configured_brokers": h.brokers,
		"connected_brokers":  connectedBrokers,
		"total_brokers":      len(brokers),
		"connected_count":    len(connectedBrokers),
		"topics_count":       len(topics),
	}

	status := HealthStatusHealthy
	message := "kafka cluster is healthy"

	if len(connectedBrokers) == 0 {
		status = HealthStatusUnhealthy
		message = "no kafka brokers are connected"
	}

	return HealthCheckResult{
		Status:       status,
		Message:      message,
		Timestamp:    time.Now(),
		ResponseTime: time.Since(start),
		Details:      details,
	}
}

// HealthService manages health checks for all dependencies
type HealthService struct {
	checkers []HealthChecker
	logger   *slog.Logger
	mu       sync.RWMutex
	results  map[string]HealthCheckResult
}

// NewHealthService creates a new health service
func NewHealthService(logger *slog.Logger) *HealthService {
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

		// For readiness, check critical dependencies
		ready := true
		criticalDeps := []string{"database", "redis", "kafka"}
		failedDeps := []string{}

		for _, dep := range criticalDeps {
			if result, exists := results[dep]; exists {
				if result.Status != HealthStatusHealthy {
					ready = false
					failedDeps = append(failedDeps, dep)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")

		response := map[string]interface{}{
			"status":    "ready",
			"timestamp": time.Now(),
		}

		if ready {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			response["status"] = "not_ready"
			response["failed_dependencies"] = failedDeps
		}

		json.NewEncoder(w).Encode(response)
	})
}

// LivenessHandler returns a liveness handler
func (s *HealthService) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Liveness check is typically more lenient
		// It should only fail if the application is completely broken
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"status":    "alive",
			"timestamp": time.Now(),
			"uptime":    time.Since(time.Now().Add(-time.Hour)), // Placeholder for actual uptime
		}

		json.NewEncoder(w).Encode(response)
	})
}

// DetailedHealthHandler returns a detailed health handler with all check results
func (s *HealthService) DetailedHealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		results := s.CheckAll(ctx)

		// Categorize results
		healthy := []string{}
		unhealthy := []string{}
		unknown := []string{}

		for name, result := range results {
			switch result.Status {
			case HealthStatusHealthy:
				healthy = append(healthy, name)
			case HealthStatusUnhealthy:
				unhealthy = append(unhealthy, name)
			case HealthStatusUnknown:
				unknown = append(unknown, name)
			}
		}

		// Determine overall status
		overall := HealthStatusHealthy
		if len(unhealthy) > 0 {
			overall = HealthStatusUnhealthy
		} else if len(unknown) > 0 {
			overall = HealthStatusUnknown
		}

		// Set appropriate status code
		switch overall {
		case HealthStatusHealthy:
			w.WriteHeader(http.StatusOK)
		case HealthStatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		w.Header().Set("Content-Type", "application/json")

		response := map[string]interface{}{
			"status":     overall,
			"timestamp":  time.Now(),
			"summary": map[string]interface{}{
				"total":     len(results),
				"healthy":   len(healthy),
				"unhealthy": len(unhealthy),
				"unknown":   len(unknown),
			},
			"services": map[string]interface{}{
				"healthy":   healthy,
				"unhealthy": unhealthy,
				"unknown":   unknown,
			},
			"details": results,
		}

		json.NewEncoder(w).Encode(response)
	})
}

// GetSystemInfo returns system information for monitoring
func (s *HealthService) GetSystemInfo() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"go_version":       runtime.Version(),
		"go_os":            runtime.GOOS,
		"go_arch":          runtime.GOARCH,
		"num_cpu":          runtime.NumCPU(),
		"num_goroutine":    runtime.NumGoroutine(),
		"alloc_mb":         memStats.Alloc / 1024 / 1024,
		"sys_mb":           memStats.Sys / 1024 / 1024,
		"gc_count":         memStats.NumGC,
		"last_gc":          time.Unix(0, int64(memStats.LastGC)),
		"next_gc_mb":       memStats.NextGC / 1024 / 1024,
		"heap_objects":     memStats.HeapObjects,
		"process_id":       os.Getpid(),
	}
}

// SystemInfoHandler returns system information
func (s *HealthService) SystemInfoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"timestamp":   time.Now(),
			"system_info": s.GetSystemInfo(),
		}

		json.NewEncoder(w).Encode(response)
	})
}
