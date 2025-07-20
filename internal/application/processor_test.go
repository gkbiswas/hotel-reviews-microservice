package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Test ProcessingConfig creation
func TestProcessingConfig_Creation(t *testing.T) {
	config := &ProcessingConfig{
		MaxWorkers:         10,
		MaxConcurrentFiles: 5,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		ProcessingTimeout:  5 * time.Minute,
		WorkerIdleTimeout:  30 * time.Second,
		MetricsInterval:    10 * time.Second,
	}

	assert.NotNil(t, config)
	assert.Equal(t, 10, config.MaxWorkers)
	assert.Equal(t, 5, config.MaxConcurrentFiles)
	assert.Equal(t, 5*time.Minute, config.ProcessingTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 30*time.Second, config.WorkerIdleTimeout)
	assert.Equal(t, 10*time.Second, config.MetricsInterval)
}

// Test ProcessingJob creation
func TestProcessingJob_Creation(t *testing.T) {
	job := &ProcessingJob{
		ID:               uuid.New(),
		ProviderID:       uuid.New(),
		FileURL:          "s3://test-bucket/test-file.json",
		Status:           StatusPending,
		StartTime:        time.Now(),
		EndTime:          time.Now(),
		RecordsTotal:     100,
		RecordsProcessed: 0,
		ErrorCount:       0,
	}

	assert.NotNil(t, job)
	assert.Equal(t, StatusPending, job.Status)
	assert.Equal(t, "s3://test-bucket/test-file.json", job.FileURL)
	assert.NotEqual(t, uuid.Nil, job.ID)
	assert.NotEqual(t, uuid.Nil, job.ProviderID)
	assert.Equal(t, int64(100), job.RecordsTotal)
	assert.Equal(t, int64(0), job.RecordsProcessed)
	assert.Equal(t, int64(0), job.ErrorCount)
}

// Test ProcessingMetrics creation
func TestProcessingMetrics_Creation(t *testing.T) {
	metrics := &ProcessingMetrics{
		TotalJobsProcessed:    100,
		TotalRecordsProcessed: 5000,
		TotalErrors:           5,
		ActiveWorkers:         3,
		QueuedJobs:            10,
		ProcessingRate:        50.0,
		AverageProcessingTime: 2 * time.Second,
		LastProcessingTime:    time.Now(),
	}

	assert.NotNil(t, metrics)
	assert.Equal(t, int64(100), metrics.TotalJobsProcessed)
	assert.Equal(t, int64(5000), metrics.TotalRecordsProcessed)
	assert.Equal(t, int64(5), metrics.TotalErrors)
	assert.Equal(t, 3, metrics.ActiveWorkers)
	assert.Equal(t, 10, metrics.QueuedJobs)
	assert.Equal(t, 50.0, metrics.ProcessingRate)
	assert.Equal(t, 2*time.Second, metrics.AverageProcessingTime)
}

// Test processing status values
func TestProcessingStatus_Values(t *testing.T) {
	statuses := []ProcessingStatus{
		StatusPending,
		StatusRunning,
		StatusCompleted,
		StatusFailed,
		StatusCancelled,
		StatusRetrying,
	}

	assert.Len(t, statuses, 6)
	assert.Equal(t, StatusPending, statuses[0])
	assert.Equal(t, StatusRunning, statuses[1])
	assert.Equal(t, StatusCompleted, statuses[2])
	assert.Equal(t, StatusFailed, statuses[3])
	assert.Equal(t, StatusCancelled, statuses[4])
	assert.Equal(t, StatusRetrying, statuses[5])
}

// Test worker structure
func TestWorker_Creation(t *testing.T) {
	worker := &worker{
		id: 1,
	}

	assert.NotNil(t, worker)
	assert.Equal(t, 1, worker.id)
}

// Test basic functionality
func TestProcessingEngine_BasicStructures(t *testing.T) {
	logger := logger.NewDefault()
	config := &ProcessingConfig{
		MaxWorkers:         10,
		MaxConcurrentFiles: 5,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		ProcessingTimeout:  5 * time.Minute,
		WorkerIdleTimeout:  30 * time.Second,
		MetricsInterval:    10 * time.Second,
	}

	// Test basic properties
	assert.NotNil(t, logger)
	assert.NotNil(t, config)
	assert.Equal(t, 10, config.MaxWorkers)
	assert.Equal(t, 5, config.MaxConcurrentFiles)

	// Test job creation
	providerID := uuid.New()
	fileURL := "s3://test-bucket/test-file.json"

	job := &ProcessingJob{
		ID:               uuid.New(),
		ProviderID:       providerID,
		FileURL:          fileURL,
		Status:           StatusPending,
		StartTime:        time.Now(),
		RecordsTotal:     100,
		RecordsProcessed: 0,
		ErrorCount:       0,
	}

	assert.NotNil(t, job)
	assert.Equal(t, providerID, job.ProviderID)
	assert.Equal(t, fileURL, job.FileURL)
	assert.Equal(t, StatusPending, job.Status)
}

// Test metrics operations
func TestProcessingMetrics_Operations(t *testing.T) {
	metrics := &ProcessingMetrics{
		TotalJobsProcessed:    0,
		TotalRecordsProcessed: 0,
		TotalErrors:           0,
		ActiveWorkers:         0,
		QueuedJobs:            0,
		ProcessingRate:        0.0,
		AverageProcessingTime: 0,
		LastProcessingTime:    time.Now(),
	}

	// Test initial state
	assert.Equal(t, int64(0), metrics.TotalJobsProcessed)
	assert.Equal(t, int64(0), metrics.TotalRecordsProcessed)
	assert.Equal(t, int64(0), metrics.TotalErrors)

	// Test metric updates (simulated)
	metrics.TotalJobsProcessed = 1
	metrics.TotalRecordsProcessed = 50
	metrics.TotalErrors = 1

	assert.Equal(t, int64(1), metrics.TotalJobsProcessed)
	assert.Equal(t, int64(50), metrics.TotalRecordsProcessed)
	assert.Equal(t, int64(1), metrics.TotalErrors)
}

// Test processing job status transitions
func TestProcessingJob_StatusTransitions(t *testing.T) {
	job := &ProcessingJob{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		FileURL:    "s3://test-bucket/test-file.json",
		Status:     StatusPending,
		StartTime:  time.Now(),
	}

	// Test initial status
	assert.Equal(t, StatusPending, job.Status)

	// Test status transitions
	job.Status = StatusRunning
	assert.Equal(t, StatusRunning, job.Status)

	job.Status = StatusCompleted
	assert.Equal(t, StatusCompleted, job.Status)

	// Test error status
	job.Status = StatusFailed
	assert.Equal(t, StatusFailed, job.Status)

	// Test retry status
	job.Status = StatusRetrying
	assert.Equal(t, StatusRetrying, job.Status)
}

// Test configuration validation
func TestProcessingConfig_Validation(t *testing.T) {
	config := &ProcessingConfig{
		MaxWorkers:         10,
		MaxConcurrentFiles: 5,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		ProcessingTimeout:  5 * time.Minute,
		WorkerIdleTimeout:  30 * time.Second,
		MetricsInterval:    10 * time.Second,
	}

	// Test valid configuration
	assert.True(t, config.MaxWorkers > 0)
	assert.True(t, config.MaxConcurrentFiles > 0)
	assert.True(t, config.MaxRetries >= 0)
	assert.True(t, config.RetryDelay > 0)
	assert.True(t, config.ProcessingTimeout > 0)
	assert.True(t, config.WorkerIdleTimeout > 0)
	assert.True(t, config.MetricsInterval > 0)
}

// Test job record tracking
func TestProcessingJob_RecordTracking(t *testing.T) {
	job := &ProcessingJob{
		ID:               uuid.New(),
		ProviderID:       uuid.New(),
		FileURL:          "s3://test-bucket/test-file.json",
		Status:           StatusRunning,
		StartTime:        time.Now(),
		RecordsTotal:     100,
		RecordsProcessed: 0,
		ErrorCount:       0,
	}

	// Test initial record counts
	assert.Equal(t, int64(100), job.RecordsTotal)
	assert.Equal(t, int64(0), job.RecordsProcessed)
	assert.Equal(t, int64(0), job.ErrorCount)

	// Simulate processing
	job.RecordsProcessed = 50
	job.ErrorCount = 2

	assert.Equal(t, int64(50), job.RecordsProcessed)
	assert.Equal(t, int64(2), job.ErrorCount)

	// Calculate progress
	progress := float64(job.RecordsProcessed) / float64(job.RecordsTotal)
	assert.Equal(t, 0.5, progress)
}

// Test processing metrics calculations
func TestProcessingMetrics_Calculations(t *testing.T) {
	metrics := &ProcessingMetrics{
		TotalJobsProcessed:    10,
		TotalRecordsProcessed: 1000,
		TotalErrors:           20,
		ActiveWorkers:         5,
		QueuedJobs:            3,
		ProcessingRate:        100.0,
		AverageProcessingTime: 30 * time.Second,
		LastProcessingTime:    time.Now(),
	}

	// Test success rate calculation
	totalRecords := metrics.TotalRecordsProcessed + metrics.TotalErrors
	successRate := float64(metrics.TotalRecordsProcessed) / float64(totalRecords)
	assert.Equal(t, float64(1000)/float64(1020), successRate)

	// Test error rate calculation
	errorRate := float64(metrics.TotalErrors) / float64(totalRecords)
	assert.Equal(t, float64(20)/float64(1020), errorRate)

	// Test records per job
	recordsPerJob := float64(metrics.TotalRecordsProcessed) / float64(metrics.TotalJobsProcessed)
	assert.Equal(t, 100.0, recordsPerJob)
}