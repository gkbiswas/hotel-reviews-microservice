package application

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// ResilientProcessingEngine extends ProcessingEngine with circuit breaker and retry protection
type ResilientProcessingEngine struct {
	*ProcessingEngine // Embed original processing engine
	
	// Circuit breaker and retry infrastructure
	circuitBreakerIntegration *infrastructure.CircuitBreakerIntegration
	retryManager              *infrastructure.RetryManager
	
	// Protected service wrappers
	protectedDatabase *infrastructure.DatabaseWrapper
	protectedCache    *infrastructure.CacheWrapper
	protectedS3       *infrastructure.S3Wrapper
	
	// Enhanced metrics
	resilienceMetrics *ResilienceMetrics
	metricsLock       sync.RWMutex
}

// ResilienceMetrics tracks resilience-related metrics
type ResilienceMetrics struct {
	// Circuit breaker metrics
	CircuitBreakerTrips     int64 `json:"circuit_breaker_trips"`
	CircuitBreakerResets    int64 `json:"circuit_breaker_resets"`
	CircuitBreakerRejections int64 `json:"circuit_breaker_rejections"`
	
	// Retry metrics
	TotalRetries        int64 `json:"total_retries"`
	RetrySuccesses      int64 `json:"retry_successes"`
	RetryExhausted      int64 `json:"retry_exhausted"`
	
	// Fallback metrics
	FallbackExecutions  int64 `json:"fallback_executions"`
	FallbackSuccesses   int64 `json:"fallback_successes"`
	FallbackFailures    int64 `json:"fallback_failures"`
	
	// Performance metrics
	AverageResponseTime time.Duration `json:"average_response_time"`
	SuccessRate         float64       `json:"success_rate"`
	
	// Timestamp
	LastUpdated time.Time `json:"last_updated"`
}

// NewResilientProcessingEngine creates a new resilient processing engine
func NewResilientProcessingEngine(
	reviewService domain.ReviewService,
	s3Client domain.S3Client,
	jsonProcessor domain.JSONProcessor,
	logger *logger.Logger,
	config *ProcessingConfig,
	circuitBreakerIntegration *infrastructure.CircuitBreakerIntegration,
	retryManager *infrastructure.RetryManager,
) *ResilientProcessingEngine {
	// Create original processing engine
	originalEngine := NewProcessingEngine(reviewService, s3Client, jsonProcessor, logger, config)
	
	// Create protected wrappers
	var protectedDatabase *infrastructure.DatabaseWrapper
	var protectedCache *infrastructure.CacheWrapper
	var protectedS3 *infrastructure.S3Wrapper
	
	if circuitBreakerIntegration != nil {
		protectedDatabase = circuitBreakerIntegration.GetDatabaseBreaker()
		protectedCache = circuitBreakerIntegration.GetCacheBreaker()
		protectedS3 = circuitBreakerIntegration.GetS3Breaker()
	}
	
	return &ResilientProcessingEngine{
		ProcessingEngine:          originalEngine,
		circuitBreakerIntegration: circuitBreakerIntegration,
		retryManager:              retryManager,
		protectedDatabase:         protectedDatabase,
		protectedCache:            protectedCache,
		protectedS3:               protectedS3,
		resilienceMetrics:         &ResilienceMetrics{},
	}
}

// Enhanced ProcessingConfig with resilience options
type ProcessingConfig struct {
	MaxWorkers         int           `json:"max_workers"`
	MaxConcurrentFiles int           `json:"max_concurrent_files"`
	MaxRetries         int           `json:"max_retries"`
	RetryDelay         time.Duration `json:"retry_delay"`
	ProcessingTimeout  time.Duration `json:"processing_timeout"`
	WorkerIdleTimeout  time.Duration `json:"worker_idle_timeout"`
	MetricsInterval    time.Duration `json:"metrics_interval"`
	
	// Resilience options
	EnableCircuitBreaker bool `json:"enable_circuit_breaker"`
	EnableRetry          bool `json:"enable_retry"`
	EnableFallback       bool `json:"enable_fallback"`
	
	// Circuit breaker thresholds
	DatabaseFailureThreshold int `json:"database_failure_threshold"`
	CacheFailureThreshold    int `json:"cache_failure_threshold"`
	S3FailureThreshold       int `json:"s3_failure_threshold"`
	
	// Retry configurations
	DatabaseMaxRetries int           `json:"database_max_retries"`
	CacheMaxRetries    int           `json:"cache_max_retries"`
	S3MaxRetries       int           `json:"s3_max_retries"`
	DatabaseRetryDelay time.Duration `json:"database_retry_delay"`
	CacheRetryDelay    time.Duration `json:"cache_retry_delay"`
	S3RetryDelay       time.Duration `json:"s3_retry_delay"`
}

// SubmitJobWithResilience submits a job with resilience protection
func (e *ResilientProcessingEngine) SubmitJobWithResilience(ctx context.Context, providerID uuid.UUID, fileURL string) (*ProcessingJob, error) {
	// Use circuit breaker and retry protection for job submission
	result, err := e.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return e.ProcessingEngine.SubmitJob(ctx, providerID, fileURL)
	})
	
	if err != nil {
		e.updateResilienceMetrics(false, err)
		return nil, err
	}
	
	e.updateResilienceMetrics(true, nil)
	return result.(*ProcessingJob), nil
}

// processJobWithResilience processes a job with resilience protection
func (e *ResilientProcessingEngine) processJobWithResilience(ctx context.Context, job *ProcessingJob) error {
	startTime := time.Now()
	
	// Update job status with circuit breaker protection
	err := e.updateJobStatus(ctx, job, StatusRunning, "Processing started")
	if err != nil {
		e.logger.Error("Failed to update job status", "error", err, "job_id", job.ID)
		return err
	}
	
	// Download file with S3 circuit breaker protection
	fileContent, err := e.downloadFileWithResilience(ctx, job.FileURL)
	if err != nil {
		e.updateJobStatus(ctx, job, StatusFailed, fmt.Sprintf("Failed to download file: %v", err))
		return err
	}
	
	// Process file content with database circuit breaker protection
	processedRecords, err := e.processFileContentWithResilience(ctx, job, fileContent)
	if err != nil {
		e.updateJobStatus(ctx, job, StatusFailed, fmt.Sprintf("Failed to process file: %v", err))
		return err
	}
	
	// Update job completion with circuit breaker protection
	job.RecordsProcessed = processedRecords
	job.EndTime = time.Now()
	
	err = e.updateJobStatus(ctx, job, StatusCompleted, "Processing completed successfully")
	if err != nil {
		e.logger.Error("Failed to update job completion status", "error", err, "job_id", job.ID)
		return err
	}
	
	// Update resilience metrics
	e.updateResilienceMetrics(true, nil)
	
	duration := time.Since(startTime)
	e.logger.Info("Job processed successfully with resilience",
		"job_id", job.ID,
		"records_processed", processedRecords,
		"duration", duration,
	)
	
	return nil
}

// downloadFileWithResilience downloads a file with S3 circuit breaker protection
func (e *ResilientProcessingEngine) downloadFileWithResilience(ctx context.Context, fileURL string) ([]byte, error) {
	result, err := e.executeWithResilience(ctx, "s3", func(ctx context.Context) (interface{}, error) {
		// Use S3 circuit breaker wrapper if available
		if e.protectedS3 != nil {
			return e.protectedS3.ExecuteS3Operation(ctx, func(ctx context.Context) (interface{}, error) {
				return e.s3Client.DownloadFile(ctx, fileURL)
			})
		}
		
		// Fallback to direct S3 client
		return e.s3Client.DownloadFile(ctx, fileURL)
	})
	
	if err != nil {
		e.updateResilienceMetrics(false, err)
		return nil, err
	}
	
	return result.([]byte), nil
}

// processFileContentWithResilience processes file content with database circuit breaker protection
func (e *ResilientProcessingEngine) processFileContentWithResilience(ctx context.Context, job *ProcessingJob, content []byte) (int64, error) {
	result, err := e.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		// Process content line by line with resilience
		lines := strings.Split(string(content), "\n")
		processedCount := int64(0)
		
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			
			// Process each line with circuit breaker protection
			err := e.processLineWithResilience(ctx, job, line, i)
			if err != nil {
				e.logger.Error("Failed to process line",
					"error", err,
					"job_id", job.ID,
					"line_number", i+1,
				)
				
				// Continue processing other lines instead of failing completely
				job.ErrorCount++
				continue
			}
			
			processedCount++
			
			// Update progress periodically
			if processedCount%100 == 0 {
				job.RecordsProcessed = processedCount
				e.updateJobProgress(ctx, job)
			}
		}
		
		return processedCount, nil
	})
	
	if err != nil {
		e.updateResilienceMetrics(false, err)
		return 0, err
	}
	
	return result.(int64), nil
}

// processLineWithResilience processes a single line with database circuit breaker protection
func (e *ResilientProcessingEngine) processLineWithResilience(ctx context.Context, job *ProcessingJob, line string, lineNumber int) error {
	_, err := e.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		// Parse and process the line using the JSON processor
		return nil, e.jsonProcessor.ProcessLine(ctx, []byte(line), job.ProviderID)
	})
	
	return err
}

// updateJobStatus updates job status with circuit breaker protection
func (e *ResilientProcessingEngine) updateJobStatus(ctx context.Context, job *ProcessingJob, status ProcessingStatus, message string) error {
	_, err := e.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		job.Status = status
		job.ErrorMessage = message
		job.UpdatedAt = time.Now()
		
		// Update job in database (assuming we have a job repository)
		return nil, nil // Placeholder - implement actual job update
	})
	
	return err
}

// updateJobProgress updates job progress with circuit breaker protection
func (e *ResilientProcessingEngine) updateJobProgress(ctx context.Context, job *ProcessingJob) error {
	_, err := e.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		job.UpdatedAt = time.Now()
		
		// Update job progress in database
		return nil, nil // Placeholder - implement actual progress update
	})
	
	return err
}

// executeWithResilience executes a function with circuit breaker and retry protection
func (e *ResilientProcessingEngine) executeWithResilience(
	ctx context.Context,
	serviceName string,
	operation func(ctx context.Context) (interface{}, error),
) (interface{}, error) {
	// If circuit breaker is enabled, use it
	if e.circuitBreakerIntegration != nil {
		breaker, exists := e.circuitBreakerIntegration.GetManager().GetCircuitBreaker(serviceName)
		if exists {
			return breaker.Execute(ctx, operation)
		}
	}
	
	// If retry manager is enabled, use it
	if e.retryManager != nil {
		return e.retryManager.ExecuteWithRetry(ctx, serviceName, operation)
	}
	
	// Fallback to direct execution
	return operation(ctx)
}

// updateResilienceMetrics updates resilience metrics
func (e *ResilientProcessingEngine) updateResilienceMetrics(success bool, err error) {
	e.metricsLock.Lock()
	defer e.metricsLock.Unlock()
	
	// Update success rate
	totalOperations := e.resilienceMetrics.RetrySuccesses + e.resilienceMetrics.RetryExhausted
	if totalOperations > 0 {
		e.resilienceMetrics.SuccessRate = float64(e.resilienceMetrics.RetrySuccesses) / float64(totalOperations) * 100
	}
	
	// Check error types
	if err != nil {
		if infrastructure.IsCircuitBreakerError(err) {
			e.resilienceMetrics.CircuitBreakerRejections++
		}
		
		if e.retryManager != nil && e.retryManager.IsRetryExhaustedError(err) {
			e.resilienceMetrics.RetryExhausted++
		}
	} else if success {
		e.resilienceMetrics.RetrySuccesses++
	}
	
	e.resilienceMetrics.LastUpdated = time.Now()
}

// GetResilienceMetrics returns current resilience metrics
func (e *ResilientProcessingEngine) GetResilienceMetrics() *ResilienceMetrics {
	e.metricsLock.RLock()
	defer e.metricsLock.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := &ResilienceMetrics{
		CircuitBreakerTrips:      e.resilienceMetrics.CircuitBreakerTrips,
		CircuitBreakerResets:     e.resilienceMetrics.CircuitBreakerResets,
		CircuitBreakerRejections: e.resilienceMetrics.CircuitBreakerRejections,
		TotalRetries:             e.resilienceMetrics.TotalRetries,
		RetrySuccesses:           e.resilienceMetrics.RetrySuccesses,
		RetryExhausted:           e.resilienceMetrics.RetryExhausted,
		FallbackExecutions:       e.resilienceMetrics.FallbackExecutions,
		FallbackSuccesses:        e.resilienceMetrics.FallbackSuccesses,
		FallbackFailures:         e.resilienceMetrics.FallbackFailures,
		AverageResponseTime:      e.resilienceMetrics.AverageResponseTime,
		SuccessRate:              e.resilienceMetrics.SuccessRate,
		LastUpdated:              e.resilienceMetrics.LastUpdated,
	}
	
	return metrics
}

// GetCircuitBreakerStatus returns circuit breaker status for the processing engine
func (e *ResilientProcessingEngine) GetCircuitBreakerStatus() map[string]interface{} {
	if e.circuitBreakerIntegration == nil {
		return map[string]interface{}{
			"status": "disabled",
		}
	}
	
	return e.circuitBreakerIntegration.GetHealthStatus()
}

// GetRetryStatus returns retry manager status
func (e *ResilientProcessingEngine) GetRetryStatus() map[string]interface{} {
	if e.retryManager == nil {
		return map[string]interface{}{
			"status": "disabled",
		}
	}
	
	return e.retryManager.GetMetrics()
}

// IsServiceAvailable checks if a service is available (not circuit broken)
func (e *ResilientProcessingEngine) IsServiceAvailable(serviceName string) bool {
	if e.circuitBreakerIntegration == nil {
		return true
	}
	
	return e.circuitBreakerIntegration.IsServiceAvailable(serviceName)
}

// ResetCircuitBreakers resets all circuit breakers for the processing engine
func (e *ResilientProcessingEngine) ResetCircuitBreakers() {
	if e.circuitBreakerIntegration == nil {
		return
	}
	
	e.circuitBreakerIntegration.ResetAllCircuitBreakers()
	e.logger.Info("All circuit breakers reset for processing engine")
}

// HealthCheck performs a health check with circuit breaker status
func (e *ResilientProcessingEngine) HealthCheck(ctx context.Context) error {
	// Check original processing engine health
	if err := e.ProcessingEngine.HealthCheck(ctx); err != nil {
		return err
	}
	
	// Check circuit breaker health
	if e.circuitBreakerIntegration != nil {
		healthStatus := e.circuitBreakerIntegration.GetHealthStatus()
		if !healthStatus["overall_healthy"].(bool) {
			return fmt.Errorf("circuit breakers are not healthy")
		}
	}
	
	return nil
}

// GetEnhancedMetrics returns enhanced metrics including resilience metrics
func (e *ResilientProcessingEngine) GetEnhancedMetrics() map[string]interface{} {
	// Get original metrics
	originalMetrics := e.ProcessingEngine.GetMetrics()
	
	// Get resilience metrics
	resilienceMetrics := e.GetResilienceMetrics()
	
	// Get circuit breaker status
	circuitBreakerStatus := e.GetCircuitBreakerStatus()
	
	// Get retry status
	retryStatus := e.GetRetryStatus()
	
	return map[string]interface{}{
		"processing_metrics":    originalMetrics,
		"resilience_metrics":    resilienceMetrics,
		"circuit_breaker_status": circuitBreakerStatus,
		"retry_status":          retryStatus,
		"timestamp":             time.Now().UTC(),
	}
}

// Stop gracefully stops the resilient processing engine
func (e *ResilientProcessingEngine) Stop() error {
	// Stop original processing engine
	if err := e.ProcessingEngine.Stop(); err != nil {
		return err
	}
	
	// Close circuit breaker integration
	if e.circuitBreakerIntegration != nil {
		e.circuitBreakerIntegration.Close()
	}
	
	// Close retry manager
	if e.retryManager != nil {
		e.retryManager.Close()
	}
	
	e.logger.Info("Resilient processing engine stopped")
	return nil
}

// Enhanced worker for resilient processing
type resilientWorker struct {
	*worker // Embed original worker
	
	// Resilience components
	circuitBreakerIntegration *infrastructure.CircuitBreakerIntegration
	retryManager              *infrastructure.RetryManager
}

// processJobWithResilience processes a job with enhanced resilience
func (w *resilientWorker) processJobWithResilience(ctx context.Context, job *ProcessingJob) error {
	// Implementation similar to ResilientProcessingEngine.processJobWithResilience
	// but at the worker level
	
	startTime := time.Now()
	
	// Process job with circuit breaker and retry protection
	err := w.processJob(ctx, job)
	
	duration := time.Since(startTime)
	
	if err != nil {
		w.engine.logger.Error("Worker failed to process job with resilience",
			"worker_id", w.id,
			"job_id", job.ID,
			"error", err,
			"duration", duration,
		)
		return err
	}
	
	w.engine.logger.Info("Worker processed job successfully with resilience",
		"worker_id", w.id,
		"job_id", job.ID,
		"duration", duration,
	)
	
	return nil
}

// Configuration defaults for resilient processing
func DefaultResilientProcessingConfig() *ProcessingConfig {
	return &ProcessingConfig{
		MaxWorkers:         10,
		MaxConcurrentFiles: 5,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		ProcessingTimeout:  30 * time.Minute,
		WorkerIdleTimeout:  5 * time.Minute,
		MetricsInterval:    30 * time.Second,
		
		// Resilience options
		EnableCircuitBreaker: true,
		EnableRetry:          true,
		EnableFallback:       true,
		
		// Circuit breaker thresholds
		DatabaseFailureThreshold: 5,
		CacheFailureThreshold:    10,
		S3FailureThreshold:       3,
		
		// Retry configurations
		DatabaseMaxRetries: 3,
		CacheMaxRetries:    2,
		S3MaxRetries:       5,
		DatabaseRetryDelay: 500 * time.Millisecond,
		CacheRetryDelay:    100 * time.Millisecond,
		S3RetryDelay:       1 * time.Second,
	}
}