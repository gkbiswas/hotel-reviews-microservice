package application

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// ProcessingEngine orchestrates file processing with worker pools and concurrency control
type ProcessingEngine struct {
	reviewService       domain.ReviewService
	s3Client           domain.S3Client
	jsonProcessor      domain.JSONProcessor
	logger             *logger.Logger
	
	// Worker pool configuration
	maxWorkers         int
	maxConcurrentFiles int
	workerPool         chan *worker
	fileQueue          chan *ProcessingJob
	
	// Processing state
	activeJobs         map[uuid.UUID]*ProcessingJob
	jobsMutex          sync.RWMutex
	
	// Metrics and monitoring
	metrics            *ProcessingMetrics
	
	// Lifecycle management
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	started            bool
	shutdown           chan struct{}
}

// ProcessingJob represents a file processing job
type ProcessingJob struct {
	mu               sync.RWMutex
	ID               uuid.UUID
	ProviderID       uuid.UUID
	FileURL          string
	Status           ProcessingStatus
	StartTime        time.Time
	EndTime          time.Time
	RecordsTotal     int64
	RecordsProcessed int64
	ErrorCount       int64
	ErrorMessage     string
	RetryCount       int
	MaxRetries       int
	Worker           *worker
	Context          context.Context
	Cancel           context.CancelFunc
}

// ProcessingStatus represents the status of a processing job
type ProcessingStatus string

const (
	StatusPending    ProcessingStatus = "pending"
	StatusRunning    ProcessingStatus = "running"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
	StatusCancelled  ProcessingStatus = "cancelled"
	StatusRetrying   ProcessingStatus = "retrying"
)

// worker represents a processing worker
type worker struct {
	mu           sync.RWMutex
	id           int
	engine       *ProcessingEngine
	busy         bool
	currentJob   *ProcessingJob
	lastActivity time.Time
}

// ProcessingMetrics tracks processing statistics
type ProcessingMetrics struct {
	mu                    sync.RWMutex
	TotalJobsProcessed    int64
	TotalRecordsProcessed int64
	TotalErrors           int64
	ActiveWorkers         int
	QueuedJobs            int
	ProcessingRate        float64
	AverageProcessingTime time.Duration
	LastProcessingTime    time.Time
}

// ProcessingConfig contains configuration for the processing engine
type ProcessingConfig struct {
	MaxWorkers         int
	MaxConcurrentFiles int
	MaxRetries         int
	RetryDelay         time.Duration
	ProcessingTimeout  time.Duration
	WorkerIdleTimeout  time.Duration
	MetricsInterval    time.Duration
}

// NewProcessingEngine creates a new processing engine
func NewProcessingEngine(
	reviewService domain.ReviewService,
	s3Client domain.S3Client,
	jsonProcessor domain.JSONProcessor,
	logger *logger.Logger,
	config *ProcessingConfig,
) *ProcessingEngine {
	ctx, cancel := context.WithCancel(context.Background())
	
	engine := &ProcessingEngine{
		reviewService:       reviewService,
		s3Client:           s3Client,
		jsonProcessor:      jsonProcessor,
		logger:             logger,
		maxWorkers:         config.MaxWorkers,
		maxConcurrentFiles: config.MaxConcurrentFiles,
		workerPool:         make(chan *worker, config.MaxWorkers),
		fileQueue:          make(chan *ProcessingJob, config.MaxConcurrentFiles*2),
		activeJobs:         make(map[uuid.UUID]*ProcessingJob),
		metrics:            &ProcessingMetrics{},
		ctx:                ctx,
		cancel:             cancel,
		shutdown:           make(chan struct{}),
	}
	
	return engine
}

// Start starts the processing engine
func (e *ProcessingEngine) Start() error {
	e.jobsMutex.Lock()
	defer e.jobsMutex.Unlock()
	
	if e.started {
		return fmt.Errorf("processing engine already started")
	}
	
	e.logger.Info("Starting processing engine",
		"max_workers", e.maxWorkers,
		"max_concurrent_files", e.maxConcurrentFiles,
	)
	
	// Start worker pool
	e.startWorkerPool()
	
	// Start job dispatcher
	e.wg.Add(1)
	go e.jobDispatcher()
	
	// Start metrics collector
	e.wg.Add(1)
	go e.metricsCollector()
	
	e.started = true
	e.logger.Info("Processing engine started successfully")
	
	return nil
}

// Stop stops the processing engine gracefully
func (e *ProcessingEngine) Stop() error {
	e.jobsMutex.Lock()
	defer e.jobsMutex.Unlock()
	
	if !e.started {
		return fmt.Errorf("processing engine not started")
	}
	
	e.logger.Info("Stopping processing engine...")
	
	// Cancel all active jobs
	for _, job := range e.activeJobs {
		if job.Cancel != nil {
			job.Cancel()
		}
	}
	
	// Signal shutdown
	e.cancel()
	close(e.shutdown)
	
	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	
	// Mark as stopped first to prevent double stopping
	e.started = false
	
	// Wait for graceful shutdown with timeout
	select {
	case <-done:
		e.logger.Info("Processing engine stopped gracefully")
		return nil
	case <-time.After(30 * time.Second):
		e.logger.Warn("Processing engine shutdown timeout, forcing stop")
		return fmt.Errorf("shutdown timeout")
	}
}

// SubmitJob submits a new file processing job
func (e *ProcessingEngine) SubmitJob(ctx context.Context, providerID uuid.UUID, fileURL string) (*ProcessingJob, error) {
	// Check if engine is started
	if !e.started {
		return nil, fmt.Errorf("processing engine not started")
	}
	
	// Validate file URL format
	_, _, err := e.parseS3URL(fileURL)
	if err != nil {
		return nil, fmt.Errorf("invalid file URL: %w", err)
	}
	
	// Check for idempotency
	if existingJob, exists := e.checkIdempotency(providerID, fileURL); exists {
		e.logger.InfoContext(ctx, "Idempotent job found, returning existing job",
			"job_id", existingJob.ID,
			"provider_id", providerID,
			"file_url", fileURL,
		)
		return existingJob, nil
	}
	
	// Create new job
	jobCtx, jobCancel := context.WithCancel(ctx)
	job := &ProcessingJob{
		ID:           uuid.New(),
		ProviderID:   providerID,
		FileURL:      fileURL,
		Status:       StatusPending,
		StartTime:    time.Now(),
		MaxRetries:   3,
		Context:      jobCtx,
		Cancel:       jobCancel,
	}
	
	// Register job
	e.jobsMutex.Lock()
	e.activeJobs[job.ID] = job
	e.jobsMutex.Unlock()
	
	// Submit to queue
	select {
	case e.fileQueue <- job:
		e.logger.InfoContext(ctx, "Job submitted successfully",
			"job_id", job.ID,
			"provider_id", providerID,
			"file_url", fileURL,
		)
		return job, nil
	case <-ctx.Done():
		e.removeJob(job.ID)
		return nil, ctx.Err()
	default:
		e.removeJob(job.ID)
		return nil, fmt.Errorf("processing queue full, try again later")
	}
}

// GetJobStatus returns the status of a processing job
func (e *ProcessingEngine) GetJobStatus(jobID uuid.UUID) (*ProcessingJob, bool) {
	e.jobsMutex.RLock()
	defer e.jobsMutex.RUnlock()
	
	job, exists := e.activeJobs[jobID]
	return job, exists
}

// CancelJob cancels a processing job
func (e *ProcessingEngine) CancelJob(jobID uuid.UUID) error {
	e.jobsMutex.Lock()
	defer e.jobsMutex.Unlock()
	
	job, exists := e.activeJobs[jobID]
	if !exists {
		return fmt.Errorf("job not found")
	}
	
	if job.Cancel != nil {
		job.Cancel()
	}
	
	job.mu.Lock()
	job.Status = StatusCancelled
	job.EndTime = time.Now()
	job.mu.Unlock()
	
	e.logger.Info("Job cancelled", "job_id", jobID)
	return nil
}

// GetMetrics returns current processing metrics
func (e *ProcessingEngine) GetMetrics() *ProcessingMetrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := &ProcessingMetrics{
		TotalJobsProcessed:    e.metrics.TotalJobsProcessed,
		TotalRecordsProcessed: e.metrics.TotalRecordsProcessed,
		TotalErrors:           e.metrics.TotalErrors,
		ActiveWorkers:         e.metrics.ActiveWorkers,
		QueuedJobs:            e.metrics.QueuedJobs,
		ProcessingRate:        e.metrics.ProcessingRate,
		AverageProcessingTime: e.metrics.AverageProcessingTime,
		LastProcessingTime:    e.metrics.LastProcessingTime,
	}
	
	return metrics
}

// startWorkerPool initializes and starts the worker pool
func (e *ProcessingEngine) startWorkerPool() {
	for i := 0; i < e.maxWorkers; i++ {
		worker := &worker{
			id:           i,
			engine:       e,
			lastActivity: time.Now(),
		}
		
		e.workerPool <- worker
		
		e.wg.Add(1)
		go e.workerLoop(worker)
	}
	
	e.logger.Info("Worker pool started", "workers", e.maxWorkers)
}

// workerLoop is the main loop for a worker
func (e *ProcessingEngine) workerLoop(w *worker) {
	defer e.wg.Done()
	
	e.logger.Debug("Worker started", "worker_id", w.id)
	
	for {
		select {
		case <-e.ctx.Done():
			e.logger.Debug("Worker stopping", "worker_id", w.id)
			return
		default:
			// Worker is available, put it back in pool
			select {
			case e.workerPool <- w:
				w.mu.Lock()
				w.busy = false
				w.lastActivity = time.Now()
				w.mu.Unlock()
			case <-e.ctx.Done():
				return
			}
		}
	}
}

// jobDispatcher dispatches jobs to available workers
func (e *ProcessingEngine) jobDispatcher() {
	defer e.wg.Done()
	
	e.logger.Info("Job dispatcher started")
	
	for {
		select {
		case <-e.ctx.Done():
			e.logger.Info("Job dispatcher stopping")
			return
		case job := <-e.fileQueue:
			e.dispatchJob(job)
		}
	}
}

// dispatchJob dispatches a single job to an available worker
func (e *ProcessingEngine) dispatchJob(job *ProcessingJob) {
	select {
	case <-e.ctx.Done():
		e.updateJobStatus(job, StatusCancelled, "System shutdown")
		return
	case worker := <-e.workerPool:
		worker.mu.Lock()
		worker.busy = true
		worker.currentJob = job
		worker.mu.Unlock()
		job.Worker = worker
		
		e.wg.Add(1)
		go e.processJobWithWorker(worker, job)
		
		e.logger.Info("Job dispatched to worker",
			"job_id", job.ID,
			"worker_id", worker.id,
			"file_url", job.FileURL,
		)
	}
}

// processJobWithWorker processes a job with a specific worker
func (e *ProcessingEngine) processJobWithWorker(w *worker, job *ProcessingJob) {
	defer e.wg.Done()
	
	e.updateJobStatus(job, StatusRunning, "")
	
	// Process the job
	err := e.processFile(job)
	
	// Handle job completion
	if err != nil {
		job.mu.Lock()
		retryCount := job.RetryCount
		maxRetries := job.MaxRetries
		if retryCount < maxRetries {
			job.RetryCount++
			retryCount = job.RetryCount
			job.mu.Unlock()
			
			e.updateJobStatus(job, StatusRetrying, fmt.Sprintf("Retry %d/%d: %v", retryCount, maxRetries, err))
			
			// Retry with exponential backoff
			retryDelay := time.Duration(retryCount) * time.Second
			time.Sleep(retryDelay)
			
			// Requeue the job
			select {
			case e.fileQueue <- job:
				e.logger.Info("Job requeued for retry",
					"job_id", job.ID,
					"retry_count", retryCount,
				)
			default:
				e.updateJobStatus(job, StatusFailed, "Failed to requeue for retry")
			}
		} else {
			job.mu.Unlock()
			e.updateJobStatus(job, StatusFailed, err.Error())
		}
	} else {
		e.updateJobStatus(job, StatusCompleted, "")
	}
	
	// Clean up worker
	w.mu.Lock()
	w.currentJob = nil
	w.lastActivity = time.Now()
	w.mu.Unlock()
}

// processFile processes a single file
func (e *ProcessingEngine) processFile(job *ProcessingJob) error {
	ctx := job.Context
	
	e.logger.InfoContext(ctx, "Starting file processing",
		"job_id", job.ID,
		"file_url", job.FileURL,
		"provider_id", job.ProviderID,
	)
	
	// Download file from S3
	bucket, key, err := e.parseS3URL(job.FileURL)
	if err != nil {
		return fmt.Errorf("invalid S3 URL: %w", err)
	}
	
	// Check if file exists
	exists, err := e.s3Client.FileExists(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("failed to check file existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("file does not exist: %s", job.FileURL)
	}
	
	// Get file size for progress tracking
	fileSize, err := e.s3Client.GetFileSize(ctx, bucket, key)
	if err != nil {
		e.logger.WarnContext(ctx, "Failed to get file size", "error", err)
	}
	
	// Download file
	reader, err := e.s3Client.DownloadFile(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer reader.Close()
	
	// Count records for progress tracking
	recordCount, err := e.jsonProcessor.CountRecords(ctx, reader)
	if err != nil {
		e.logger.WarnContext(ctx, "Failed to count records", "error", err)
	} else {
		job.RecordsTotal = int64(recordCount)
	}
	
	// Re-download for processing (since we consumed the reader)
	reader, err = e.s3Client.DownloadFile(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("failed to re-download file: %w", err)
	}
	defer reader.Close()
	
	// Process the file
	err = e.jsonProcessor.ProcessFile(ctx, reader, job.ProviderID, job.ID)
	if err != nil {
		return fmt.Errorf("failed to process file: %w", err)
	}
	
	e.logger.InfoContext(ctx, "File processing completed successfully",
		"job_id", job.ID,
		"file_url", job.FileURL,
		"file_size", fileSize,
		"records_total", job.RecordsTotal,
	)
	
	return nil
}

// updateJobStatus updates the status of a job
func (e *ProcessingEngine) updateJobStatus(job *ProcessingJob, status ProcessingStatus, errorMessage string) {
	job.mu.Lock()
	job.Status = status
	job.ErrorMessage = errorMessage
	job.mu.Unlock()
	
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		job.EndTime = time.Now()
		
		// Update metrics
		e.metrics.mu.Lock()
		e.metrics.TotalJobsProcessed++
		e.metrics.TotalRecordsProcessed += job.RecordsProcessed
		if status == StatusFailed {
			e.metrics.TotalErrors++
		}
		e.metrics.LastProcessingTime = time.Now()
		e.metrics.mu.Unlock()
		
		// Remove from active jobs after a delay to allow status queries
		go func() {
			time.Sleep(5 * time.Minute)
			e.removeJob(job.ID)
		}()
	}
	
	// Note: Processing status updates are handled by the ReviewService layer
	
	e.logger.InfoContext(job.Context, "Job status updated",
		"job_id", job.ID,
		"status", status,
		"error_message", errorMessage,
	)
}

// checkIdempotency checks if a similar job is already running
func (e *ProcessingEngine) checkIdempotency(providerID uuid.UUID, fileURL string) (*ProcessingJob, bool) {
	e.jobsMutex.RLock()
	defer e.jobsMutex.RUnlock()
	
	for _, job := range e.activeJobs {
		if job.ProviderID == providerID && job.FileURL == fileURL {
			job.mu.RLock()
			status := job.Status
			job.mu.RUnlock()
			if status == StatusPending || status == StatusRunning || status == StatusRetrying {
				return job, true
			}
		}
	}
	
	return nil, false
}

// removeJob removes a job from active jobs
func (e *ProcessingEngine) removeJob(jobID uuid.UUID) {
	e.jobsMutex.Lock()
	defer e.jobsMutex.Unlock()
	
	delete(e.activeJobs, jobID)
}

// metricsCollector collects and updates processing metrics
func (e *ProcessingEngine) metricsCollector() {
	defer e.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.updateMetrics()
		}
	}
}

// updateMetrics updates processing metrics
func (e *ProcessingEngine) updateMetrics() {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()
	
	e.jobsMutex.RLock()
	activeJobs := len(e.activeJobs)
	e.jobsMutex.RUnlock()
	
	e.metrics.QueuedJobs = len(e.fileQueue)
	e.metrics.ActiveWorkers = e.maxWorkers - len(e.workerPool)
	
	// Calculate processing rate (jobs per minute)
	if e.metrics.TotalJobsProcessed > 0 {
		duration := time.Since(e.metrics.LastProcessingTime)
		if duration > 0 {
			e.metrics.ProcessingRate = float64(e.metrics.TotalJobsProcessed) / duration.Minutes()
		}
	}
	
	e.logger.Debug("Metrics updated",
		"total_jobs_processed", e.metrics.TotalJobsProcessed,
		"total_records_processed", e.metrics.TotalRecordsProcessed,
		"total_errors", e.metrics.TotalErrors,
		"active_workers", e.metrics.ActiveWorkers,
		"queued_jobs", e.metrics.QueuedJobs,
		"processing_rate", e.metrics.ProcessingRate,
		"active_jobs", activeJobs,
	)
}

// parseS3URL parses an S3 URL into bucket and key components
func (e *ProcessingEngine) parseS3URL(url string) (bucket, key string, err error) {
	var path string
	
	// Handle different S3 URL formats
	if strings.HasPrefix(url, "s3://") {
		// s3://bucket/key format
		path = strings.TrimPrefix(url, "s3://")
	} else if strings.HasPrefix(url, "https://") {
		// Handle HTTPS S3 URLs
		if strings.Contains(url, ".s3.amazonaws.com/") {
			// https://bucket.s3.amazonaws.com/key format
			url = strings.TrimPrefix(url, "https://")
			parts := strings.SplitN(url, ".s3.amazonaws.com/", 2)
			if len(parts) != 2 {
				return "", "", fmt.Errorf("invalid S3 URL format: %s", url)
			}
			return parts[0], parts[1], nil
		} else if strings.Contains(url, "s3.amazonaws.com/") {
			// https://s3.amazonaws.com/bucket/key format
			url = strings.TrimPrefix(url, "https://s3.amazonaws.com/")
			parts := strings.SplitN(url, "/", 2)
			if len(parts) < 2 {
				return "", "", fmt.Errorf("invalid S3 URL format: %s", url)
			}
			return parts[0], parts[1], nil
		} else {
			return "", "", fmt.Errorf("invalid S3 URL format: %s", url)
		}
	} else {
		return "", "", fmt.Errorf("invalid S3 URL format: %s", url)
	}
	
	// For s3:// format
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid S3 URL format: %s", url)
	}
	
	return parts[0], parts[1], nil
}

// BackpressureController manages backpressure for the processing engine
type BackpressureController struct {
	engine           *ProcessingEngine
	maxQueueSize     int
	maxMemoryUsage   int64
	currentMemoryUsage int64
	mu               sync.RWMutex
}

// NewBackpressureController creates a new backpressure controller
func NewBackpressureController(engine *ProcessingEngine, maxQueueSize int, maxMemoryUsage int64) *BackpressureController {
	return &BackpressureController{
		engine:         engine,
		maxQueueSize:   maxQueueSize,
		maxMemoryUsage: maxMemoryUsage,
	}
}

// ShouldAcceptJob determines if a new job should be accepted based on backpressure
func (bc *BackpressureController) ShouldAcceptJob() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	// Check queue size
	if len(bc.engine.fileQueue) >= bc.maxQueueSize {
		return false
	}
	
	// Check memory usage
	if bc.currentMemoryUsage >= bc.maxMemoryUsage {
		return false
	}
	
	return true
}

// UpdateMemoryUsage updates the current memory usage
func (bc *BackpressureController) UpdateMemoryUsage(usage int64) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.currentMemoryUsage = usage
}

// JobProgressTracker tracks the progress of individual jobs
type JobProgressTracker struct {
	job       *ProcessingJob
	logger    *logger.Logger
	lastUpdate time.Time
	mu        sync.RWMutex
}

// NewJobProgressTracker creates a new job progress tracker
func NewJobProgressTracker(job *ProcessingJob, logger *logger.Logger) *JobProgressTracker {
	return &JobProgressTracker{
		job:       job,
		logger:    logger,
		lastUpdate: time.Now(),
	}
}

// UpdateProgress updates the progress of a job
func (jpt *JobProgressTracker) UpdateProgress(recordsProcessed int64) {
	jpt.mu.Lock()
	defer jpt.mu.Unlock()
	
	jpt.job.RecordsProcessed = recordsProcessed
	
	// Log progress every 10 seconds
	if time.Since(jpt.lastUpdate) > 10*time.Second {
		progress := float64(recordsProcessed) / float64(jpt.job.RecordsTotal) * 100
		jpt.logger.InfoContext(jpt.job.Context, "Job progress update",
			"job_id", jpt.job.ID,
			"records_processed", recordsProcessed,
			"records_total", jpt.job.RecordsTotal,
			"progress_percent", fmt.Sprintf("%.2f%%", progress),
		)
		jpt.lastUpdate = time.Now()
	}
}

// GetProgress returns the current progress of a job
func (jpt *JobProgressTracker) GetProgress() (int64, int64, float64) {
	jpt.mu.RLock()
	defer jpt.mu.RUnlock()
	
	processed := jpt.job.RecordsProcessed
	total := jpt.job.RecordsTotal
	
	var progress float64
	if total > 0 {
		progress = float64(processed) / float64(total) * 100
	}
	
	return processed, total, progress
}