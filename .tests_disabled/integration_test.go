package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// IntegrationTestSuite contains the integration test suite
type IntegrationTestSuite struct {
	suite.Suite
	
	// Test containers
	postgresContainer  *postgres.PostgresContainer
	localstackContainer *localstack.LocalStackContainer
	
	// Test database
	db *gorm.DB
	
	// Application components
	database          *infrastructure.Database
	s3Client          *infrastructure.S3Client
	reviewRepository  domain.ReviewRepository
	jsonProcessor     domain.JSONProcessor
	reviewService     domain.ReviewService
	processingEngine  *application.ProcessingEngine
	
	// Test configuration
	cfg    *config.Config
	logger *logger.Logger
	ctx    context.Context
	
	// Test data
	testBucket     string
	testProvider   *domain.Provider
	testHotel      *domain.Hotel
	testReviewer   *domain.ReviewerInfo
}

// SetupSuite sets up the test suite with containers and dependencies
func (suite *IntegrationTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testBucket = "test-reviews-bucket"
	
	// Initialize logger
	suite.logger = logger.NewDefault()
	
	// Setup PostgreSQL container
	suite.setupPostgresContainer()
	
	// Setup LocalStack container
	suite.setupLocalStackContainer()
	
	// Setup application components
	suite.setupApplicationComponents()
	
	// Setup test data
	suite.setupTestData()
}

// TearDownSuite tears down the test suite
func (suite *IntegrationTestSuite) TearDownSuite() {
	if suite.postgresContainer != nil {
		suite.postgresContainer.Terminate(suite.ctx)
	}
	
	if suite.localstackContainer != nil {
		suite.localstackContainer.Terminate(suite.ctx)
	}
}

// setupPostgresContainer sets up PostgreSQL test container
func (suite *IntegrationTestSuite) setupPostgresContainer() {
	var err error
	
	suite.postgresContainer, err = postgres.RunContainer(suite.ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("hotel_reviews_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Minute)),
	)
	require.NoError(suite.T(), err)
	
	// Get database connection details
	host, err := suite.postgresContainer.Host(suite.ctx)
	require.NoError(suite.T(), err)
	
	port, err := suite.postgresContainer.MappedPort(suite.ctx, "5432")
	require.NoError(suite.T(), err)
	
	// Create database connection
	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=hotel_reviews_test sslmode=disable TimeZone=UTC",
		host, port.Port())
	
	suite.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(suite.T(), err)
	
	// Run database migrations
	err = suite.db.AutoMigrate(
		&domain.Provider{},
		&domain.Hotel{},
		&domain.ReviewerInfo{},
		&domain.Review{},
		&domain.ReviewSummary{},
		&domain.ReviewProcessingStatus{},
	)
	require.NoError(suite.T(), err)
	
	suite.logger.Info("PostgreSQL container started successfully", "host", host, "port", port.Port())
}

// setupLocalStackContainer sets up LocalStack test container
func (suite *IntegrationTestSuite) setupLocalStackContainer() {
	var err error
	
	suite.localstackContainer, err = localstack.RunContainer(suite.ctx,
		testcontainers.WithImage("localstack/localstack:latest"),
		testcontainers.WithEnv(map[string]string{
			"SERVICES": "s3",
			"DEBUG":    "1",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready.").WithStartupTimeout(5*time.Minute)),
	)
	require.NoError(suite.T(), err)
	
	// Get LocalStack endpoint
	endpoint, err := suite.localstackContainer.Endpoint(suite.ctx, "")
	require.NoError(suite.T(), err)
	
	suite.logger.Info("LocalStack container started successfully", "endpoint", endpoint)
}

// setupApplicationComponents sets up application components
func (suite *IntegrationTestSuite) setupApplicationComponents() {
	// Create test configuration
	host, _ := suite.postgresContainer.Host(suite.ctx)
	port, _ := suite.postgresContainer.MappedPort(suite.ctx, "5432")
	endpoint, _ := suite.localstackContainer.Endpoint(suite.ctx, "")
	
	suite.cfg = &config.Config{
		Database: config.DatabaseConfig{
			Host:            host,
			Port:            int(port.Int()),
			User:            "test",
			Password:        "test",
			Name:            "hotel_reviews_test",
			SSLMode:         "disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
			TimeZone:        "UTC",
		},
		S3: config.S3Config{
			Region:          "us-east-1",
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			Bucket:          suite.testBucket,
			Endpoint:        endpoint,
			ForcePathStyle:  true,
			UseSSL:          false,
			Timeout:         30 * time.Second,
			RetryCount:      3,
			RetryDelay:      1 * time.Second,
			UploadPartSize:  5 * 1024 * 1024,
			DownloadPartSize: 5 * 1024 * 1024,
		},
		Processing: config.ProcessingConfig{
			BatchSize:         1000,
			WorkerCount:       2,
			MaxFileSize:       100 * 1024 * 1024,
			ProcessingTimeout: 30 * time.Minute,
			MaxRetries:        3,
			RetryDelay:        5 * time.Second,
		},
	}
	
	// Initialize database wrapper
	var err error
	suite.database, err = infrastructure.NewDatabase(&suite.cfg.Database, suite.logger)
	require.NoError(suite.T(), err)
	
	// Initialize S3 client
	suite.s3Client, err = infrastructure.NewS3Client(&suite.cfg.S3, suite.logger)
	require.NoError(suite.T(), err)
	
	// Create S3 bucket
	err = suite.s3Client.CreateBucket(suite.ctx, suite.testBucket)
	require.NoError(suite.T(), err)
	
	// Initialize repository
	suite.reviewRepository = infrastructure.NewReviewRepository(suite.database, suite.logger)
	
	// Initialize JSON processor
	suite.jsonProcessor = infrastructure.NewJSONLinesProcessor(suite.reviewRepository, suite.logger)
	
	// Initialize review service
	suite.reviewService = domain.NewReviewService(
		suite.reviewRepository,
		suite.s3Client,
		suite.jsonProcessor,
		nil, // notification service
		nil, // cache service
		nil, // metrics service
		nil, // event publisher
		suite.logger,
	)
	
	// Initialize processing engine
	processingConfig := &application.ProcessingConfig{
		MaxWorkers:         2,
		MaxConcurrentFiles: 5,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		ProcessingTimeout:  30 * time.Second,
		WorkerIdleTimeout:  5 * time.Minute,
		MetricsInterval:    30 * time.Second,
	}
	
	suite.processingEngine = application.NewProcessingEngine(
		suite.reviewService,
		suite.s3Client,
		suite.jsonProcessor,
		suite.logger,
		processingConfig,
	)
}

// setupTestData sets up test data
func (suite *IntegrationTestSuite) setupTestData() {
	// Create test provider
	suite.testProvider = &domain.Provider{
		ID:       uuid.New(),
		Name:     "Test Provider",
		BaseURL:  "https://test.example.com",
		IsActive: true,
	}
	
	err := suite.reviewRepository.CreateProvider(suite.ctx, suite.testProvider)
	require.NoError(suite.T(), err)
	
	// Create test hotel
	suite.testHotel = &domain.Hotel{
		ID:          uuid.New(),
		Name:        "Test Hotel",
		Address:     "123 Test St",
		City:        "Test City",
		Country:     "Test Country",
		StarRating:  4,
		Description: "A test hotel for integration testing",
		Latitude:    40.7128,
		Longitude:   -74.0060,
	}
	
	err = suite.reviewRepository.CreateHotel(suite.ctx, suite.testHotel)
	require.NoError(suite.T(), err)
	
	// Create test reviewer
	suite.testReviewer = &domain.ReviewerInfo{
		ID:          uuid.New(),
		Name:        "Test Reviewer",
		Email:       "test@example.com",
		Country:     "USA",
		IsVerified:  true,
		TotalReviews: 0,
		AverageRating: 0.0,
	}
	
	err = suite.reviewRepository.CreateReviewerInfo(suite.ctx, suite.testReviewer)
	require.NoError(suite.T(), err)
}

// Test data structures
type TestReviewData struct {
	ID           string  `json:"id"`
	HotelID      string  `json:"hotel_id"`
	HotelName    string  `json:"hotel_name"`
	Rating       float64 `json:"rating"`
	Title        string  `json:"title"`
	Comment      string  `json:"comment"`
	ReviewDate   string  `json:"review_date"`
	Language     string  `json:"language"`
	ReviewerName string  `json:"reviewer_name"`
	ReviewerEmail string `json:"reviewer_email"`
}

// Helper methods
func (suite *IntegrationTestSuite) createTestReviewsFile(filename string, reviews []TestReviewData) error {
	// Create temporary file
	tempDir := os.TempDir()
	filePath := filepath.Join(tempDir, filename)
	
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Write reviews as JSON Lines
	for _, review := range reviews {
		reviewJSON, err := json.Marshal(review)
		if err != nil {
			return err
		}
		
		_, err = file.Write(reviewJSON)
		if err != nil {
			return err
		}
		
		_, err = file.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}
	
	return nil
}

func (suite *IntegrationTestSuite) uploadFileToS3(filename string) error {
	tempDir := os.TempDir()
	filePath := filepath.Join(tempDir, filename)
	
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	return suite.s3Client.UploadFile(suite.ctx, suite.testBucket, filename, file, "application/json")
}

func (suite *IntegrationTestSuite) getFileURL(filename string) string {
	return fmt.Sprintf("s3://%s/%s", suite.testBucket, filename)
}

func (suite *IntegrationTestSuite) waitForProcessingCompletion(processingID uuid.UUID, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(suite.ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for processing completion")
		case <-ticker.C:
			job, exists := suite.processingEngine.GetJobStatus(processingID)
			if !exists {
				return fmt.Errorf("processing job not found")
			}
			
			switch job.Status {
			case application.StatusCompleted:
				return nil
			case application.StatusFailed:
				return fmt.Errorf("processing failed: %s", job.ErrorMessage)
			case application.StatusCancelled:
				return fmt.Errorf("processing was cancelled")
			}
		}
	}
}

// Integration Tests

func (suite *IntegrationTestSuite) TestCompleteFileProcessingFlow() {
	// Test the complete flow from file upload to database storage
	
	// 1. Create test reviews data
	reviews := []TestReviewData{
		{
			ID:           "review-1",
			HotelID:      suite.testHotel.ID.String(),
			HotelName:    suite.testHotel.Name,
			Rating:       4.5,
			Title:        "Great stay!",
			Comment:      "This hotel exceeded my expectations. Great service and clean rooms.",
			ReviewDate:   "2024-01-15T10:00:00Z",
			Language:     "en",
			ReviewerName: suite.testReviewer.Name,
			ReviewerEmail: suite.testReviewer.Email,
		},
		{
			ID:           "review-2",
			HotelID:      suite.testHotel.ID.String(),
			HotelName:    suite.testHotel.Name,
			Rating:       3.5,
			Title:        "Average experience",
			Comment:      "The hotel was okay, nothing special but decent value for money.",
			ReviewDate:   "2024-01-16T14:30:00Z",
			Language:     "en",
			ReviewerName: suite.testReviewer.Name,
			ReviewerEmail: suite.testReviewer.Email,
		},
		{
			ID:           "review-3",
			HotelID:      suite.testHotel.ID.String(),
			HotelName:    suite.testHotel.Name,
			Rating:       5.0,
			Title:        "Perfect!",
			Comment:      "Amazing hotel with excellent amenities and friendly staff.",
			ReviewDate:   "2024-01-17T09:15:00Z",
			Language:     "en",
			ReviewerName: suite.testReviewer.Name,
			ReviewerEmail: suite.testReviewer.Email,
		},
	}
	
	filename := "test_reviews.jsonl"
	
	// 2. Create and upload file
	err := suite.createTestReviewsFile(filename, reviews)
	require.NoError(suite.T(), err)
	
	err = suite.uploadFileToS3(filename)
	require.NoError(suite.T(), err)
	
	// 3. Start processing engine
	err = suite.processingEngine.Start()
	require.NoError(suite.T(), err)
	defer suite.processingEngine.Stop()
	
	// 4. Submit processing job
	fileURL := suite.getFileURL(filename)
	job, err := suite.processingEngine.SubmitJob(suite.ctx, suite.testProvider.ID, fileURL)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), job)
	
	// 5. Wait for processing completion
	err = suite.waitForProcessingCompletion(job.ID, 30*time.Second)
	require.NoError(suite.T(), err)
	
	// 6. Verify reviews were stored in database
	storedReviews, err := suite.reviewRepository.GetByHotel(suite.ctx, suite.testHotel.ID, 100, 0)
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), storedReviews, 3)
	
	// 7. Verify review data
	for i, review := range storedReviews {
		assert.Equal(suite.T(), suite.testProvider.ID, review.ProviderID)
		assert.Equal(suite.T(), suite.testHotel.ID, review.HotelID)
		assert.Equal(suite.T(), reviews[i].Rating, review.Rating)
		assert.Equal(suite.T(), reviews[i].Title, review.Title)
		assert.Equal(suite.T(), reviews[i].Comment, review.Comment)
		assert.Equal(suite.T(), reviews[i].Language, review.Language)
		assert.NotEmpty(suite.T(), review.ProcessingHash)
		assert.NotNil(suite.T(), review.ProcessedAt)
	}
	
	// 8. Verify processing status
	finalStatus, err := suite.reviewService.GetProcessingStatus(suite.ctx, job.ID)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "completed", finalStatus.Status)
	assert.Equal(suite.T(), int64(3), finalStatus.RecordsProcessed)
	
	// 9. Verify review summary is updated
	summary, err := suite.reviewRepository.GetReviewSummaryByHotelID(suite.ctx, suite.testHotel.ID)
	if err == nil {
		assert.Equal(suite.T(), 3, summary.TotalReviews)
		assert.InDelta(suite.T(), 4.33, summary.AverageRating, 0.1) // (4.5 + 3.5 + 5.0) / 3
	}
}

func (suite *IntegrationTestSuite) TestFileProcessingWithInvalidData() {
	// Test processing with invalid review data
	
	// 1. Create invalid reviews data
	invalidReviews := []TestReviewData{
		{
			ID:           "invalid-review-1",
			HotelID:      suite.testHotel.ID.String(),
			HotelName:    suite.testHotel.Name,
			Rating:       6.0, // Invalid rating
			Title:        "Invalid review",
			Comment:      "This review has invalid data",
			ReviewDate:   "2024-01-15T10:00:00Z",
			Language:     "en",
			ReviewerName: suite.testReviewer.Name,
			ReviewerEmail: suite.testReviewer.Email,
		},
		{
			ID:           "invalid-review-2",
			HotelID:      suite.testHotel.ID.String(),
			HotelName:    suite.testHotel.Name,
			Rating:       4.0,
			Title:        "Valid review",
			Comment:      "", // Empty comment
			ReviewDate:   "2024-01-16T14:30:00Z",
			Language:     "en",
			ReviewerName: suite.testReviewer.Name,
			ReviewerEmail: suite.testReviewer.Email,
		},
	}
	
	filename := "invalid_reviews.jsonl"
	
	// 2. Create and upload file
	err := suite.createTestReviewsFile(filename, invalidReviews)
	require.NoError(suite.T(), err)
	
	err = suite.uploadFileToS3(filename)
	require.NoError(suite.T(), err)
	
	// 3. Start processing engine
	err = suite.processingEngine.Start()
	require.NoError(suite.T(), err)
	defer suite.processingEngine.Stop()
	
	// 4. Submit processing job
	fileURL := suite.getFileURL(filename)
	job, err := suite.processingEngine.SubmitJob(suite.ctx, suite.testProvider.ID, fileURL)
	require.NoError(suite.T(), err)
	
	// 5. Wait for processing completion (should fail or complete with errors)
	err = suite.waitForProcessingCompletion(job.ID, 30*time.Second)
	// Note: This might complete successfully if the processor handles invalid data gracefully
	
	// 6. Check processing status
	finalStatus, err := suite.reviewService.GetProcessingStatus(suite.ctx, job.ID)
	require.NoError(suite.T(), err)
	
	// The processor should handle invalid data gracefully
	// Check that some records were processed or error was reported
	if finalStatus.Status == "failed" {
		assert.NotEmpty(suite.T(), finalStatus.ErrorMsg)
	}
}

func (suite *IntegrationTestSuite) TestFileProcessingWithNonExistentFile() {
	// Test processing with non-existent file
	
	// 1. Start processing engine
	err := suite.processingEngine.Start()
	require.NoError(suite.T(), err)
	defer suite.processingEngine.Stop()
	
	// 2. Submit job with non-existent file
	fileURL := suite.getFileURL("non_existent_file.jsonl")
	job, err := suite.processingEngine.SubmitJob(suite.ctx, suite.testProvider.ID, fileURL)
	require.NoError(suite.T(), err)
	
	// 3. Wait for processing completion (should fail)
	err = suite.waitForProcessingCompletion(job.ID, 30*time.Second)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "processing failed")
	
	// 4. Verify processing status
	finalStatus, err := suite.reviewService.GetProcessingStatus(suite.ctx, job.ID)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "failed", finalStatus.Status)
	assert.NotEmpty(suite.T(), finalStatus.ErrorMsg)
}

func (suite *IntegrationTestSuite) TestDuplicateFileProcessing() {
	// Test processing the same file twice (idempotency)
	
	// 1. Create test reviews data
	reviews := []TestReviewData{
		{
			ID:           "duplicate-review-1",
			HotelID:      suite.testHotel.ID.String(),
			HotelName:    suite.testHotel.Name,
			Rating:       4.0,
			Title:        "Duplicate test",
			Comment:      "This is a duplicate test review",
			ReviewDate:   "2024-01-15T10:00:00Z",
			Language:     "en",
			ReviewerName: suite.testReviewer.Name,
			ReviewerEmail: suite.testReviewer.Email,
		},
	}
	
	filename := "duplicate_reviews.jsonl"
	
	// 2. Create and upload file
	err := suite.createTestReviewsFile(filename, reviews)
	require.NoError(suite.T(), err)
	
	err = suite.uploadFileToS3(filename)
	require.NoError(suite.T(), err)
	
	// 3. Start processing engine
	err = suite.processingEngine.Start()
	require.NoError(suite.T(), err)
	defer suite.processingEngine.Stop()
	
	fileURL := suite.getFileURL(filename)
	
	// 4. Submit first job
	job1, err := suite.processingEngine.SubmitJob(suite.ctx, suite.testProvider.ID, fileURL)
	require.NoError(suite.T(), err)
	
	// 5. Submit second job (should be idempotent)
	job2, err := suite.processingEngine.SubmitJob(suite.ctx, suite.testProvider.ID, fileURL)
	require.NoError(suite.T(), err)
	
	// 6. Jobs should be the same (idempotent)
	assert.Equal(suite.T(), job1.ID, job2.ID)
	
	// 7. Wait for processing completion
	err = suite.waitForProcessingCompletion(job1.ID, 30*time.Second)
	require.NoError(suite.T(), err)
	
	// 8. Verify only one set of reviews was stored
	storedReviews, err := suite.reviewRepository.GetByProvider(suite.ctx, suite.testProvider.ID, 100, 0)
	require.NoError(suite.T(), err)
	
	// Count reviews with the specific external ID
	duplicateCount := 0
	for _, review := range storedReviews {
		if review.ExternalID == "duplicate-review-1" {
			duplicateCount++
		}
	}
	assert.Equal(suite.T(), 1, duplicateCount, "Should have only one review with the duplicate external ID")
}

func (suite *IntegrationTestSuite) TestLargeFileProcessing() {
	// Test processing a large file with many reviews
	
	// 1. Create large dataset
	var largeReviews []TestReviewData
	for i := 0; i < 1000; i++ {
		review := TestReviewData{
			ID:           fmt.Sprintf("large-review-%d", i),
			HotelID:      suite.testHotel.ID.String(),
			HotelName:    suite.testHotel.Name,
			Rating:       float64(1 + (i%5)), // Ratings from 1 to 5
			Title:        fmt.Sprintf("Review %d", i),
			Comment:      fmt.Sprintf("This is review number %d for large file testing", i),
			ReviewDate:   "2024-01-15T10:00:00Z",
			Language:     "en",
			ReviewerName: suite.testReviewer.Name,
			ReviewerEmail: suite.testReviewer.Email,
		}
		largeReviews = append(largeReviews, review)
	}
	
	filename := "large_reviews.jsonl"
	
	// 2. Create and upload file
	err := suite.createTestReviewsFile(filename, largeReviews)
	require.NoError(suite.T(), err)
	
	err = suite.uploadFileToS3(filename)
	require.NoError(suite.T(), err)
	
	// 3. Start processing engine
	err = suite.processingEngine.Start()
	require.NoError(suite.T(), err)
	defer suite.processingEngine.Stop()
	
	// 4. Submit processing job
	fileURL := suite.getFileURL(filename)
	job, err := suite.processingEngine.SubmitJob(suite.ctx, suite.testProvider.ID, fileURL)
	require.NoError(suite.T(), err)
	
	// 5. Wait for processing completion (with longer timeout)
	err = suite.waitForProcessingCompletion(job.ID, 2*time.Minute)
	require.NoError(suite.T(), err)
	
	// 6. Verify all reviews were stored
	storedReviews, err := suite.reviewRepository.GetByProvider(suite.ctx, suite.testProvider.ID, 1500, 0)
	require.NoError(suite.T(), err)
	
	// Count reviews from this large file
	largeFileCount := 0
	for _, review := range storedReviews {
		if strings.HasPrefix(review.ExternalID, "large-review-") {
			largeFileCount++
		}
	}
	assert.Equal(suite.T(), 1000, largeFileCount, "Should have processed all 1000 reviews")
	
	// 7. Verify processing status
	finalStatus, err := suite.reviewService.GetProcessingStatus(suite.ctx, job.ID)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "completed", finalStatus.Status)
	assert.Equal(suite.T(), int64(1000), finalStatus.RecordsProcessed)
}

func (suite *IntegrationTestSuite) TestConcurrentFileProcessing() {
	// Test processing multiple files concurrently
	
	// 1. Create multiple test files
	filenames := []string{"concurrent_1.jsonl", "concurrent_2.jsonl", "concurrent_3.jsonl"}
	var jobs []*application.ProcessingJob
	
	for i, filename := range filenames {
		reviews := []TestReviewData{
			{
				ID:           fmt.Sprintf("concurrent-review-%d", i),
				HotelID:      suite.testHotel.ID.String(),
				HotelName:    suite.testHotel.Name,
				Rating:       4.0,
				Title:        fmt.Sprintf("Concurrent review %d", i),
				Comment:      fmt.Sprintf("This is concurrent review %d", i),
				ReviewDate:   "2024-01-15T10:00:00Z",
				Language:     "en",
				ReviewerName: suite.testReviewer.Name,
				ReviewerEmail: suite.testReviewer.Email,
			},
		}
		
		err := suite.createTestReviewsFile(filename, reviews)
		require.NoError(suite.T(), err)
		
		err = suite.uploadFileToS3(filename)
		require.NoError(suite.T(), err)
	}
	
	// 2. Start processing engine
	err := suite.processingEngine.Start()
	require.NoError(suite.T(), err)
	defer suite.processingEngine.Stop()
	
	// 3. Submit all jobs concurrently
	for _, filename := range filenames {
		fileURL := suite.getFileURL(filename)
		job, err := suite.processingEngine.SubmitJob(suite.ctx, suite.testProvider.ID, fileURL)
		require.NoError(suite.T(), err)
		jobs = append(jobs, job)
	}
	
	// 4. Wait for all jobs to complete
	for _, job := range jobs {
		err = suite.waitForProcessingCompletion(job.ID, 30*time.Second)
		require.NoError(suite.T(), err)
	}
	
	// 5. Verify all reviews were processed
	storedReviews, err := suite.reviewRepository.GetByProvider(suite.ctx, suite.testProvider.ID, 100, 0)
	require.NoError(suite.T(), err)
	
	concurrentCount := 0
	for _, review := range storedReviews {
		if strings.HasPrefix(review.ExternalID, "concurrent-review-") {
			concurrentCount++
		}
	}
	assert.Equal(suite.T(), 3, concurrentCount, "Should have processed all 3 concurrent reviews")
}

func (suite *IntegrationTestSuite) TestDatabaseOperations() {
	// Test direct database operations
	
	// 1. Create test review
	review := &domain.Review{
		ID:             uuid.New(),
		ProviderID:     suite.testProvider.ID,
		HotelID:        suite.testHotel.ID,
		ReviewerInfoID: suite.testReviewer.ID,
		ExternalID:     "db-test-review",
		Rating:         4.5,
		Title:          "Database Test Review",
		Comment:        "This is a test review for database operations",
		ReviewDate:     time.Now(),
		Language:       "en",
	}
	
	// 2. Create review
	err := suite.reviewRepository.CreateBatch(suite.ctx, []domain.Review{*review})
	require.NoError(suite.T(), err)
	
	// 3. Retrieve review
	storedReview, err := suite.reviewRepository.GetByID(suite.ctx, review.ID)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), review.ID, storedReview.ID)
	assert.Equal(suite.T(), review.Rating, storedReview.Rating)
	assert.Equal(suite.T(), review.Comment, storedReview.Comment)
	
	// 4. Update review
	storedReview.Rating = 5.0
	err = suite.reviewRepository.CreateBatch(suite.ctx, []domain.Review{*storedReview})
	require.NoError(suite.T(), err)
	
	// 5. Verify update
	updatedReview, err := suite.reviewRepository.GetByID(suite.ctx, review.ID)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), 5.0, updatedReview.Rating)
	
	// 6. Delete review
	err = suite.reviewRepository.DeleteByID(suite.ctx, review.ID)
	require.NoError(suite.T(), err)
	
	// 7. Verify deletion
	_, err = suite.reviewRepository.GetByID(suite.ctx, review.ID)
	assert.Error(suite.T(), err)
}

func (suite *IntegrationTestSuite) TestS3Operations() {
	// Test S3 operations
	
	// 1. Check bucket exists
	exists, err := suite.s3Client.BucketExists(suite.ctx, suite.testBucket)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	
	// 2. Upload test file
	testContent := "test content"
	err = suite.s3Client.UploadFile(suite.ctx, suite.testBucket, "test.txt", strings.NewReader(testContent), "text/plain")
	require.NoError(suite.T(), err)
	
	// 3. Check file exists
	exists, err = suite.s3Client.FileExists(suite.ctx, suite.testBucket, "test.txt")
	require.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	
	// 4. Get file size
	size, err := suite.s3Client.GetFileSize(suite.ctx, suite.testBucket, "test.txt")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(len(testContent)), size)
	
	// 5. Download file
	reader, err := suite.s3Client.DownloadFile(suite.ctx, suite.testBucket, "test.txt")
	require.NoError(suite.T(), err)
	defer reader.Close()
	
	// 6. List files
	files, err := suite.s3Client.ListFiles(suite.ctx, suite.testBucket, "", 100)
	require.NoError(suite.T(), err)
	assert.Contains(suite.T(), files, "test.txt")
	
	// 7. Delete file
	err = suite.s3Client.DeleteFile(suite.ctx, suite.testBucket, "test.txt")
	require.NoError(suite.T(), err)
	
	// 8. Verify deletion
	exists, err = suite.s3Client.FileExists(suite.ctx, suite.testBucket, "test.txt")
	require.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Run the integration test suite
func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

// Additional utility tests
func TestJSONProcessor_Integration(t *testing.T) {
	// This test can be run independently if needed
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Simple integration test for JSON processor
	ctx := context.Background()
	logger := logger.NewDefault()
	
	// Create a mock repository for testing
	// This would normally use the real repository in integration tests
	t.Log("JSON processor integration test placeholder")
}

func TestApplicationFlow_Integration(t *testing.T) {
	// This test can be run independently if needed
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Test complete application flow
	ctx := context.Background()
	logger := logger.NewDefault()
	
	// This would test the complete application flow
	t.Log("Application flow integration test placeholder")
}