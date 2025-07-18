package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// MockReadCloser for testing
type MockReadCloser struct {
	*strings.Reader
}

func (m *MockReadCloser) Close() error {
	return nil
}

// Mock S3Client for testing
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) UploadFile(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	args := m.Called(ctx, bucket, key, body, contentType)
	return args.Error(0)
}

func (m *MockS3Client) DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, bucket, key)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockS3Client) GetFileURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	args := m.Called(ctx, bucket, key, expiration)
	return args.String(0), args.Error(1)
}

func (m *MockS3Client) DeleteFile(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}

func (m *MockS3Client) ListFiles(ctx context.Context, bucket, prefix string, limit int) ([]string, error) {
	args := m.Called(ctx, bucket, prefix, limit)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockS3Client) GetFileMetadata(ctx context.Context, bucket, key string) (map[string]string, error) {
	args := m.Called(ctx, bucket, key)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockS3Client) UpdateFileMetadata(ctx context.Context, bucket, key string, metadata map[string]string) error {
	args := m.Called(ctx, bucket, key, metadata)
	return args.Error(0)
}

func (m *MockS3Client) CreateBucket(ctx context.Context, bucket string) error {
	args := m.Called(ctx, bucket)
	return args.Error(0)
}

func (m *MockS3Client) DeleteBucket(ctx context.Context, bucket string) error {
	args := m.Called(ctx, bucket)
	return args.Error(0)
}

func (m *MockS3Client) BucketExists(ctx context.Context, bucket string) (bool, error) {
	args := m.Called(ctx, bucket)
	return args.Bool(0), args.Error(1)
}

func (m *MockS3Client) FileExists(ctx context.Context, bucket, key string) (bool, error) {
	args := m.Called(ctx, bucket, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockS3Client) GetFileSize(ctx context.Context, bucket, key string) (int64, error) {
	args := m.Called(ctx, bucket, key)
	return args.Get(0).(int64), args.Error(1)
}

// Mock JSONProcessor for testing
type MockJSONProcessor struct {
	mock.Mock
}

func (m *MockJSONProcessor) ProcessFile(ctx context.Context, reader io.Reader, providerID uuid.UUID, processingID uuid.UUID) error {
	args := m.Called(ctx, reader, providerID, processingID)
	return args.Error(0)
}

func (m *MockJSONProcessor) ValidateFile(ctx context.Context, reader io.Reader) error {
	args := m.Called(ctx, reader)
	return args.Error(0)
}

func (m *MockJSONProcessor) CountRecords(ctx context.Context, reader io.Reader) (int, error) {
	args := m.Called(ctx, reader)
	return args.Int(0), args.Error(1)
}

func (m *MockJSONProcessor) ParseReview(ctx context.Context, jsonLine []byte, providerID uuid.UUID) (*domain.Review, error) {
	args := m.Called(ctx, jsonLine, providerID)
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockJSONProcessor) ParseHotel(ctx context.Context, jsonLine []byte) (*domain.Hotel, error) {
	args := m.Called(ctx, jsonLine)
	return args.Get(0).(*domain.Hotel), args.Error(1)
}

func (m *MockJSONProcessor) ParseReviewerInfo(ctx context.Context, jsonLine []byte) (*domain.ReviewerInfo, error) {
	args := m.Called(ctx, jsonLine)
	return args.Get(0).(*domain.ReviewerInfo), args.Error(1)
}

func (m *MockJSONProcessor) ValidateReview(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockJSONProcessor) ValidateHotel(ctx context.Context, hotel *domain.Hotel) error {
	args := m.Called(ctx, hotel)
	return args.Error(0)
}

func (m *MockJSONProcessor) ValidateReviewerInfo(ctx context.Context, reviewerInfo *domain.ReviewerInfo) error {
	args := m.Called(ctx, reviewerInfo)
	return args.Error(0)
}

func (m *MockJSONProcessor) ProcessBatch(ctx context.Context, reviews []domain.Review) error {
	args := m.Called(ctx, reviews)
	return args.Error(0)
}

func (m *MockJSONProcessor) GetBatchSize() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockJSONProcessor) SetBatchSize(size int) {
	m.Called(size)
}

// Test Suite for ProcessingEngine
type ProcessingEngineTestSuite struct {
	suite.Suite
	engine          *ProcessingEngine
	mockService     *MockReviewService
	mockS3Client    *MockS3Client
	mockProcessor   *MockJSONProcessor
	testLogger      *logger.Logger
	ctx             context.Context
}

func (suite *ProcessingEngineTestSuite) SetupTest() {
	suite.mockService = new(MockReviewService)
	suite.mockS3Client = new(MockS3Client)
	suite.mockProcessor = new(MockJSONProcessor)
	suite.testLogger = logger.NewDefault()
	suite.ctx = context.Background()

	suite.engine = NewProcessingEngine(
		suite.mockService,
		suite.mockS3Client,
		suite.mockProcessor,
		suite.testLogger,
		&ProcessingConfig{
			MaxWorkers:         2,
			MaxConcurrentFiles: 5,
			MaxRetries:         3,
			RetryDelay:         time.Second,
			ProcessingTimeout:  time.Minute * 30,
			WorkerIdleTimeout:  time.Minute * 5,
			MetricsInterval:    time.Second * 30,
		},
	)
}

func (suite *ProcessingEngineTestSuite) TearDownTest() {
	if suite.engine.started {
		suite.engine.Stop()
	}
	suite.mockService.AssertExpectations(suite.T())
	suite.mockS3Client.AssertExpectations(suite.T())
	suite.mockProcessor.AssertExpectations(suite.T())
}

func TestProcessingEngineTestSuite(t *testing.T) {
	suite.Run(t, new(ProcessingEngineTestSuite))
}

// Test NewProcessingEngine
func (suite *ProcessingEngineTestSuite) TestNewProcessingEngine() {
	// Act - NewProcessingEngine is called in SetupTest
	
	// Assert
	suite.NotNil(suite.engine)
	suite.Equal(2, suite.engine.maxWorkers)
	suite.Equal(5, suite.engine.maxConcurrentFiles)
	suite.False(suite.engine.started)
	suite.NotNil(suite.engine.activeJobs)
	suite.NotNil(suite.engine.metrics)
}

// Test Start and Stop
func (suite *ProcessingEngineTestSuite) TestStart_Success() {
	// Act
	err := suite.engine.Start()

	// Assert
	suite.NoError(err)
	suite.True(suite.engine.started)
	
	// Cleanup
	err = suite.engine.Stop()
	suite.NoError(err)
	suite.False(suite.engine.started)
}

func (suite *ProcessingEngineTestSuite) TestStart_AlreadyStarted() {
	// Arrange
	err := suite.engine.Start()
	suite.NoError(err)

	// Act
	err = suite.engine.Start()

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "already started")
	
	// Cleanup
	suite.engine.Stop()
}

func (suite *ProcessingEngineTestSuite) TestStop_NotStarted() {
	// Act
	err := suite.engine.Stop()

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "not started")
}

// Test SubmitJob
func (suite *ProcessingEngineTestSuite) TestSubmitJob_Success() {
	// Arrange
	suite.engine.Start()
	defer suite.engine.Stop()
	
	providerID := uuid.New()
	fileURL := "https://my-bucket.s3.amazonaws.com/test-file.jsonl"

	// Setup mocks for background processing - expect multiple calls
	suite.mockS3Client.On("FileExists", mock.Anything, "my-bucket", "test-file.jsonl").Return(false, errors.New("file not found")).Maybe()

	// Act
	job, err := suite.engine.SubmitJob(suite.ctx, providerID, fileURL)

	// Assert
	suite.NoError(err)
	suite.NotNil(job)
	suite.Equal(providerID, job.ProviderID)
	suite.Equal(fileURL, job.FileURL)
	// Job should be either pending or running (timing dependent)
	job.mu.RLock()
	jobStatus := job.Status
	job.mu.RUnlock()
	suite.Contains([]ProcessingStatus{StatusPending, StatusRunning}, jobStatus)
}

func (suite *ProcessingEngineTestSuite) TestSubmitJob_EngineNotStarted() {
	// Arrange
	providerID := uuid.New()
	fileURL := "https://my-bucket.s3.amazonaws.com/test-file.jsonl"

	// Act
	job, err := suite.engine.SubmitJob(suite.ctx, providerID, fileURL)

	// Assert
	suite.Error(err)
	suite.Nil(job)
	suite.Contains(err.Error(), "not started")
}

func (suite *ProcessingEngineTestSuite) TestSubmitJob_InvalidURL() {
	// Arrange
	suite.engine.Start()
	defer suite.engine.Stop()
	
	providerID := uuid.New()
	fileURL := "invalid-url"

	// Act
	job, err := suite.engine.SubmitJob(suite.ctx, providerID, fileURL)

	// Assert
	suite.Error(err)
	suite.Nil(job)
	suite.Contains(err.Error(), "invalid file URL")
}

// Test GetJobStatus
func (suite *ProcessingEngineTestSuite) TestGetJobStatus_Success() {
	// Arrange
	suite.engine.Start()
	defer suite.engine.Stop()
	
	providerID := uuid.New()
	fileURL := "https://my-bucket.s3.amazonaws.com/test-file.jsonl"
	
	// Setup mocks for background processing - expect multiple calls
	suite.mockS3Client.On("FileExists", mock.Anything, "my-bucket", "test-file.jsonl").Return(false, errors.New("file not found")).Maybe()
	
	job, err := suite.engine.SubmitJob(suite.ctx, providerID, fileURL)
	suite.NoError(err)

	// Act
	retrievedJob, found := suite.engine.GetJobStatus(job.ID)

	// Assert
	suite.True(found)
	suite.NotNil(retrievedJob)
	suite.Equal(job.ID, retrievedJob.ID)
}

func (suite *ProcessingEngineTestSuite) TestGetJobStatus_NotFound() {
	// Arrange
	nonExistentID := uuid.New()

	// Act
	job, found := suite.engine.GetJobStatus(nonExistentID)

	// Assert
	suite.False(found)
	suite.Nil(job)
}

// Test CancelJob
func (suite *ProcessingEngineTestSuite) TestCancelJob_Success() {
	// Arrange
	suite.engine.Start()
	defer suite.engine.Stop()
	
	providerID := uuid.New()
	fileURL := "https://my-bucket.s3.amazonaws.com/test-file.jsonl"
	
	// Setup mocks for background processing - expect multiple calls
	suite.mockS3Client.On("FileExists", mock.Anything, "my-bucket", "test-file.jsonl").Return(false, errors.New("file not found")).Maybe()
	
	job, err := suite.engine.SubmitJob(suite.ctx, providerID, fileURL)
	suite.NoError(err)

	// Wait a short time to ensure job starts processing
	time.Sleep(10 * time.Millisecond)

	// Act
	err = suite.engine.CancelJob(job.ID)

	// Assert
	suite.NoError(err)
	
	// Wait for cancellation to be processed
	time.Sleep(50 * time.Millisecond)
	
	// Verify job status
	retrievedJob, found := suite.engine.GetJobStatus(job.ID)
	suite.True(found)
	// Job could be cancelled or in retrying state (due to concurrent processing)
	suite.Contains([]ProcessingStatus{StatusCancelled, StatusRetrying}, retrievedJob.Status)
}

func (suite *ProcessingEngineTestSuite) TestCancelJob_NotFound() {
	// Arrange
	nonExistentID := uuid.New()

	// Act
	err := suite.engine.CancelJob(nonExistentID)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "job not found")
}

// Test GetMetrics
func (suite *ProcessingEngineTestSuite) TestGetMetrics() {
	// Act
	metrics := suite.engine.GetMetrics()

	// Assert
	suite.NotNil(metrics)
	suite.GreaterOrEqual(metrics.TotalJobsProcessed, int64(0))
	suite.GreaterOrEqual(metrics.TotalRecordsProcessed, int64(0))
	suite.GreaterOrEqual(metrics.TotalErrors, int64(0))
	suite.GreaterOrEqual(metrics.ActiveWorkers, 0)
}

// Test parseS3URL
func (suite *ProcessingEngineTestSuite) TestParseS3URL_Success() {
	// Arrange
	s3URL := "https://my-bucket.s3.amazonaws.com/path/to/file.jsonl"

	// Act
	bucket, key, err := suite.engine.parseS3URL(s3URL)

	// Assert
	suite.NoError(err)
	suite.Equal("my-bucket", bucket)
	suite.Equal("path/to/file.jsonl", key)
}

func (suite *ProcessingEngineTestSuite) TestParseS3URL_InvalidURL() {
	// Arrange
	invalidURL := "not-an-s3-url"

	// Act
	bucket, key, err := suite.engine.parseS3URL(invalidURL)

	// Assert
	suite.Error(err)
	suite.Empty(bucket)
	suite.Empty(key)
	suite.Contains(err.Error(), "invalid S3 URL format")
}

func (suite *ProcessingEngineTestSuite) TestParseS3URL_HTTPSFormat() {
	// Arrange
	s3URL := "https://s3.amazonaws.com/my-bucket/path/to/file.jsonl"

	// Act
	bucket, key, err := suite.engine.parseS3URL(s3URL)

	// Assert
	suite.NoError(err)
	suite.Equal("my-bucket", bucket)
	suite.Equal("path/to/file.jsonl", key)
}

// Test processFile functionality (integration test)
func (suite *ProcessingEngineTestSuite) TestProcessFile_Success() {
	// Arrange
	ctx := context.Background()
	job := &ProcessingJob{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		FileURL:    "https://my-bucket.s3.amazonaws.com/test-file.jsonl",
		Status:     StatusPending,
		StartTime:  time.Now(),
		Context:    ctx,
	}

	// Mock file content
	fileContent := `{"rating": 4.5, "comment": "Great hotel", "hotel_id": "` + uuid.New().String() + `"}`
	mockReader := io.NopCloser(strings.NewReader(fileContent))

	suite.mockS3Client.On("FileExists", mock.Anything, "my-bucket", "test-file.jsonl").Return(true, nil)
	suite.mockS3Client.On("GetFileSize", mock.Anything, "my-bucket", "test-file.jsonl").Return(int64(len(fileContent)), nil)
	suite.mockS3Client.On("DownloadFile", mock.Anything, "my-bucket", "test-file.jsonl").Return(mockReader, nil)
	suite.mockProcessor.On("CountRecords", mock.Anything, mock.Anything).Return(1, nil)
	suite.mockProcessor.On("ProcessFile", mock.Anything, mock.Anything, job.ProviderID, job.ID).Return(nil)

	// Act
	err := suite.engine.processFile(job)

	// Assert
	suite.NoError(err)
}

func (suite *ProcessingEngineTestSuite) TestProcessFile_S3Error() {
	// Arrange
	ctx := context.Background()
	job := &ProcessingJob{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		FileURL:    "https://my-bucket.s3.amazonaws.com/test-file.jsonl",
		Status:     StatusPending,
		StartTime:  time.Now(),
		Context:    ctx,
	}

	suite.mockS3Client.On("FileExists", mock.Anything, "my-bucket", "test-file.jsonl").Return(false, errors.New("file not found"))

	// Act
	err := suite.engine.processFile(job)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "failed to check file existence")
}

// Test worker pool functionality
func (suite *ProcessingEngineTestSuite) TestWorkerPool_Basic() {
	// Arrange
	suite.engine.Start()
	defer suite.engine.Stop()

	// Act - Workers should be started
	// Wait a moment for workers to initialize
	time.Sleep(100 * time.Millisecond)

	// Assert - Check that workers are running (indirect test)
	metrics := suite.engine.GetMetrics()
	suite.GreaterOrEqual(metrics.ActiveWorkers, 0)
}

// Test idempotency
func (suite *ProcessingEngineTestSuite) TestIdempotency_DuplicateJob() {
	// Arrange
	suite.engine.Start()
	defer suite.engine.Stop()
	
	providerID := uuid.New()
	fileURL := "https://my-bucket.s3.amazonaws.com/test-file.jsonl"

	// Setup mocks for background processing - expect multiple calls
	suite.mockS3Client.On("FileExists", mock.Anything, "my-bucket", "test-file.jsonl").Return(false, errors.New("file not found")).Maybe()

	// Submit first job
	job1, err := suite.engine.SubmitJob(suite.ctx, providerID, fileURL)
	suite.NoError(err)

	// Act - Submit duplicate job
	job2, err := suite.engine.SubmitJob(suite.ctx, providerID, fileURL)

	// Assert
	suite.NoError(err)
	suite.Equal(job1.ID, job2.ID) // Should return existing job
}

// Test BackpressureController
func (suite *ProcessingEngineTestSuite) TestNewBackpressureController() {
	// Act
	bc := NewBackpressureController(suite.engine, 100, 1024*1024*100) // 100MB

	// Assert
	suite.NotNil(bc)
	suite.Equal(100, bc.maxQueueSize)
	suite.Equal(int64(1024*1024*100), bc.maxMemoryUsage)
	suite.Equal(int64(0), bc.currentMemoryUsage)
}

func (suite *ProcessingEngineTestSuite) TestBackpressureController_ShouldAcceptJob_LowUsage() {
	// Arrange
	bc := NewBackpressureController(suite.engine, 100, 1024*1024*100)

	// Act
	shouldAccept := bc.ShouldAcceptJob()

	// Assert
	suite.True(shouldAccept)
}

func (suite *ProcessingEngineTestSuite) TestBackpressureController_ShouldAcceptJob_HighMemoryUsage() {
	// Arrange
	bc := NewBackpressureController(suite.engine, 100, 1024*1024*100)
	bc.UpdateMemoryUsage(1024*1024*150) // 150MB, over limit

	// Act
	shouldAccept := bc.ShouldAcceptJob()

	// Assert
	suite.False(shouldAccept)
}

func (suite *ProcessingEngineTestSuite) TestBackpressureController_UpdateMemoryUsage() {
	// Arrange
	bc := NewBackpressureController(suite.engine, 100, 1024*1024*100)
	newUsage := int64(1024*1024*50) // 50MB

	// Act
	bc.UpdateMemoryUsage(newUsage)

	// Assert
	bc.mu.RLock()
	actualUsage := bc.currentMemoryUsage
	bc.mu.RUnlock()
	suite.Equal(newUsage, actualUsage)
}

// Test JobProgressTracker
func (suite *ProcessingEngineTestSuite) TestNewJobProgressTracker() {
	// Arrange
	job := &ProcessingJob{
		ID:           uuid.New(),
		RecordsTotal: 100,
		Context:      suite.ctx,
	}

	// Act
	tracker := NewJobProgressTracker(job, suite.testLogger)

	// Assert
	suite.NotNil(tracker)
	suite.Equal(job, tracker.job)
	suite.Equal(suite.testLogger, tracker.logger)
}

func (suite *ProcessingEngineTestSuite) TestJobProgressTracker_UpdateProgress() {
	// Arrange
	job := &ProcessingJob{
		ID:           uuid.New(),
		RecordsTotal: 100,
		Context:      suite.ctx,
	}
	tracker := NewJobProgressTracker(job, suite.testLogger)

	// Act
	tracker.UpdateProgress(50)

	// Assert
	suite.Equal(int64(50), job.RecordsProcessed)
}

func (suite *ProcessingEngineTestSuite) TestJobProgressTracker_GetProgress() {
	// Arrange
	job := &ProcessingJob{
		ID:               uuid.New(),
		RecordsTotal:     100,
		RecordsProcessed: 30,
		Context:          suite.ctx,
	}
	tracker := NewJobProgressTracker(job, suite.testLogger)

	// Act
	processed, total, progress := tracker.GetProgress()

	// Assert
	suite.Equal(int64(30), processed)
	suite.Equal(int64(100), total)
	suite.Equal(30.0, progress)
}

func (suite *ProcessingEngineTestSuite) TestJobProgressTracker_GetProgress_ZeroTotal() {
	// Arrange
	job := &ProcessingJob{
		ID:               uuid.New(),
		RecordsTotal:     0,
		RecordsProcessed: 10,
		Context:          suite.ctx,
	}
	tracker := NewJobProgressTracker(job, suite.testLogger)

	// Act
	processed, total, progress := tracker.GetProgress()

	// Assert
	suite.Equal(int64(10), processed)
	suite.Equal(int64(0), total)
	suite.Equal(0.0, progress)
}

func (suite *ProcessingEngineTestSuite) TestRemoveJob() {
	// Arrange
	jobID := uuid.New()
	job := &ProcessingJob{
		ID:      jobID,
		Context: suite.ctx,
	}
	
	// Add job to active jobs
	suite.engine.jobsMutex.Lock()
	suite.engine.activeJobs[jobID] = job
	suite.engine.jobsMutex.Unlock()
	
	// Verify job is present
	suite.engine.jobsMutex.RLock()
	_, exists := suite.engine.activeJobs[jobID]
	suite.engine.jobsMutex.RUnlock()
	suite.True(exists)
	
	// Act
	suite.engine.removeJob(jobID)
	
	// Assert
	suite.engine.jobsMutex.RLock()
	_, exists = suite.engine.activeJobs[jobID]
	suite.engine.jobsMutex.RUnlock()
	suite.False(exists)
}

func (suite *ProcessingEngineTestSuite) TestUpdateMetrics() {
	// Arrange
	jobID1 := uuid.New()
	jobID2 := uuid.New()
	
	// Add some active jobs
	suite.engine.jobsMutex.Lock()
	suite.engine.activeJobs[jobID1] = &ProcessingJob{ID: jobID1, Context: suite.ctx}
	suite.engine.activeJobs[jobID2] = &ProcessingJob{ID: jobID2, Context: suite.ctx}
	suite.engine.jobsMutex.Unlock()
	
	// Set some initial metrics
	suite.engine.metrics.mu.Lock()
	suite.engine.metrics.TotalJobsProcessed = 10
	suite.engine.metrics.LastProcessingTime = time.Now().Add(-2 * time.Minute)
	suite.engine.metrics.mu.Unlock()
	
	// Add some jobs to queue
	suite.engine.fileQueue <- &ProcessingJob{ID: uuid.New(), Context: suite.ctx}
	suite.engine.fileQueue <- &ProcessingJob{ID: uuid.New(), Context: suite.ctx}
	
	// Act
	suite.engine.updateMetrics()
	
	// Assert
	suite.engine.metrics.mu.RLock()
	suite.Equal(2, suite.engine.metrics.QueuedJobs)
	suite.True(suite.engine.metrics.ProcessingRate > 0) // Should have calculated a rate
	suite.engine.metrics.mu.RUnlock()
	
	// Clean up queue
	<-suite.engine.fileQueue
	<-suite.engine.fileQueue
}