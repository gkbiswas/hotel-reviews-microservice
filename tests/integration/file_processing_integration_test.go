package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	pkgConfig "github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
)

// FileProcessingIntegrationSuite focuses on file processing workflows
type FileProcessingIntegrationSuite struct {
	*IntegrationTestSuite
	s3Client         *s3.Client
	s3Service        *infrastructure.S3Client
	fileHandlers     *application.FileHandlers
	processingEngine *application.ProcessingEngine
}

// SetupFileProcessingSuite initializes file processing test environment
func SetupFileProcessingSuite(t *testing.T) *FileProcessingIntegrationSuite {
	baseSuite := SetupSuite(t)
	
	suite := &FileProcessingIntegrationSuite{
		IntegrationTestSuite: baseSuite,
	}

	// Setup S3 client for direct operations
	suite.setupS3Client(t)
	
	// Setup file processing components
	suite.setupFileProcessing(t)

	return suite
}

// setupS3Client creates AWS S3 client for LocalStack
func (suite *FileProcessingIntegrationSuite) setupS3Client(t *testing.T) {
	ctx := context.Background()
	
	endpoint, err := suite.localstackContainer.PortEndpoint(ctx, "4566", "http")
	require.NoError(t, err)

	s3Config, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint}, nil
			}),
		),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	require.NoError(t, err)

	suite.s3Client = s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Create S3 service wrapper
	s3Cfg := &pkgConfig.S3Config{
		Endpoint:       endpoint,
		Bucket:         "test-bucket",
		Region:         "us-east-1",
		ForcePathStyle: true,
	}
	var err2 error
	suite.s3Service, err2 = infrastructure.NewS3Client(s3Cfg, suite.logger)
	require.NoError(t, err2)
}

// setupFileProcessing initializes file processing components
func (suite *FileProcessingIntegrationSuite) setupFileProcessing(t *testing.T) {
	// Initialize processing engine
	processingConfig := &application.ProcessingConfig{
		MaxWorkers:         2,
		MaxConcurrentFiles: 5,
		MaxRetries:         3,
		RetryDelay:         time.Second,
		ProcessingTimeout:  time.Minute * 5,
		WorkerIdleTimeout:  time.Minute,
		MetricsInterval:    time.Second * 10,
	}

	database := &infrastructure.Database{DB: suite.db}
	reviewRepo := infrastructure.NewReviewRepository(database, suite.logger)
	jsonProcessor := infrastructure.NewJSONLinesProcessor(reviewRepo, suite.logger)

	suite.processingEngine = application.NewProcessingEngine(
		suite.reviewService,
		suite.s3Service,
		jsonProcessor,
		suite.logger,
		processingConfig,
	)

	// Start processing engine
	err := suite.processingEngine.Start()
	require.NoError(t, err)

	// Initialize file handlers
	suite.fileHandlers = application.NewFileHandlers(
		suite.reviewService,
		suite.s3Service,
		jsonProcessor,
		suite.processingEngine,
		suite.logger,
		"test-bucket",
	)

	// Add file processing routes to router
	files := suite.router.Group("/api/v1/files")
	{
		files.POST("/upload", suite.fileHandlers.UploadAndProcessFile)
		files.GET("/processing/:id", suite.fileHandlers.GetProcessingStatus)
		files.GET("/processing/history", suite.fileHandlers.GetProcessingHistory)
		files.POST("/processing/:id/cancel", suite.fileHandlers.CancelProcessing)
		files.POST("/validate", suite.fileHandlers.ValidateFile)
		files.GET("/metrics", suite.fileHandlers.GetProcessingMetrics)
	}
}

// Test file processing workflows
func TestFileProcessingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file processing integration tests in short mode")
	}

	suite := SetupFileProcessingSuite(t)
	defer suite.Cleanup(t)

	t.Run("Upload and Process JSONL File", func(t *testing.T) {
		suite.TestUploadAndProcessFile(t)
	})

	t.Run("Process Large File", func(t *testing.T) {
		suite.TestProcessLargeFile(t)
	})

	t.Run("File Validation", func(t *testing.T) {
		suite.TestFileValidation(t)
	})

	t.Run("Processing Status Tracking", func(t *testing.T) {
		suite.TestProcessingStatusTracking(t)
	})

	t.Run("Processing Metrics", func(t *testing.T) {
		suite.TestProcessingMetrics(t)
	})

	t.Run("Error Scenarios", func(t *testing.T) {
		suite.TestFileProcessingErrors(t)
	})

	t.Run("Concurrent File Processing", func(t *testing.T) {
		suite.TestConcurrentFileProcessing(t)
	})

	t.Run("Processing Cancellation", func(t *testing.T) {
		suite.TestProcessingCancellation(t)
	})
}

// TestUploadAndProcessFile tests the complete file upload and processing workflow
func (suite *FileProcessingIntegrationSuite) TestUploadAndProcessFile(t *testing.T) {
	ctx := context.Background()

	// Create test JSONL data
	reviews := []map[string]interface{}{
		{
			"review_id":     "file-review-1",
			"hotel_name":    suite.testHotel.Name,
			"rating":        4.5,
			"title":         "Excellent service!",
			"comment":       "The staff was very friendly and helpful.",
			"review_date":   time.Now().Add(-72 * time.Hour).Format(time.RFC3339),
			"reviewer_name": "Alice Johnson",
			"language":      "en",
			"trip_type":     "business",
		},
		{
			"review_id":     "file-review-2",
			"hotel_name":    suite.testHotel.Name,
			"rating":        3.5,
			"title":         "Good location",
			"comment":       "Hotel is well located but rooms could be better.",
			"review_date":   time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
			"reviewer_name": "Bob Smith",
			"language":      "en",
			"trip_type":     "leisure",
		},
		{
			"review_id":     "file-review-3",
			"hotel_name":    suite.testHotel.Name,
			"rating":        5.0,
			"title":         "Perfect stay!",
			"comment":       "Everything was perfect. Highly recommended!",
			"review_date":   time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			"reviewer_name": "Carol Williams",
			"language":      "en",
			"trip_type":     "family",
		},
	}

	// Convert to JSONL format
	var jsonlData strings.Builder
	for _, review := range reviews {
		line, _ := json.Marshal(review)
		jsonlData.Write(line)
		jsonlData.WriteString("\n")
	}

	// Upload file to S3
	key := fmt.Sprintf("test-reviews-%d.jsonl", time.Now().Unix())
	_, err := suite.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String("test-bucket"),
		Key:         aws.String(key),
		Body:        strings.NewReader(jsonlData.String()),
		ContentType: aws.String("application/x-jsonlines"),
	})
	require.NoError(t, err)

	// Process the file via API
	processRequest := map[string]interface{}{
		"file_url":    fmt.Sprintf("s3://test-bucket/%s", key),
		"provider_id": suite.testProvider.ID.String(),
	}

	body, _ := json.Marshal(processRequest)
	resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	// Extract processing ID
	data := response["data"].(map[string]interface{})
	processingID := data["id"].(string)

	// Wait for processing to complete
	suite.waitForProcessingCompletion(t, processingID, 30*time.Second)

	// Verify processing status
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/files/processing/%s", suite.server.URL, processingID))
	require.NoError(t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	statusData := response["data"].(map[string]interface{})
	assert.Equal(t, "completed", statusData["status"].(string))
	assert.Equal(t, float64(3), statusData["processed_records"].(float64))
	assert.Equal(t, float64(0), statusData["failed_records"].(float64))

	// Verify reviews were created
	reviewsResp, err := http.Get(suite.server.URL + "/api/v1/reviews?limit=20")
	require.NoError(t, err)
	defer reviewsResp.Body.Close()

	err = json.NewDecoder(reviewsResp.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	reviewsData := response["data"].([]interface{})
	assert.GreaterOrEqual(t, len(reviewsData), 5) // Original 2 + new 3
}

// TestProcessLargeFile tests processing a large file with many reviews
func (suite *FileProcessingIntegrationSuite) TestProcessLargeFile(t *testing.T) {
	ctx := context.Background()

	// Create large JSONL data (100 reviews)
	var jsonlData strings.Builder
	for i := 0; i < 100; i++ {
		review := map[string]interface{}{
			"review_id":     fmt.Sprintf("large-file-review-%d", i),
			"hotel_name":    suite.testHotel.Name,
			"rating":        1.0 + float64(i%4) + 0.5, // Ratings from 1.5 to 5.0
			"title":         fmt.Sprintf("Review title %d", i),
			"comment":       fmt.Sprintf("This is review number %d with detailed feedback about the hotel stay.", i),
			"review_date":   time.Now().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			"reviewer_name": fmt.Sprintf("Reviewer %d", i),
			"language":      []string{"en", "es", "fr", "de"}[i%4],
			"trip_type":     []string{"business", "leisure", "family"}[i%3],
		}

		line, _ := json.Marshal(review)
		jsonlData.Write(line)
		jsonlData.WriteString("\n")
	}

	// Upload large file to S3
	key := fmt.Sprintf("large-reviews-%d.jsonl", time.Now().Unix())
	_, err := suite.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String("test-bucket"),
		Key:         aws.String(key),
		Body:        strings.NewReader(jsonlData.String()),
		ContentType: aws.String("application/x-jsonlines"),
	})
	require.NoError(t, err)

	// Process the large file
	processRequest := map[string]interface{}{
		"file_url":    fmt.Sprintf("s3://test-bucket/%s", key),
		"provider_id": suite.testProvider.ID.String(),
	}

	body, _ := json.Marshal(processRequest)
	resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	processingID := data["id"].(string)

	// Wait for processing (longer timeout for large file)
	suite.waitForProcessingCompletion(t, processingID, 60*time.Second)

	// Verify all records were processed
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/files/processing/%s", suite.server.URL, processingID))
	require.NoError(t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	statusData := response["data"].(map[string]interface{})
	assert.Equal(t, "completed", statusData["status"].(string))
	assert.Equal(t, float64(100), statusData["processed_records"].(float64))
}

// TestFileValidation tests file validation functionality
func (suite *FileProcessingIntegrationSuite) TestFileValidation(t *testing.T) {
	ctx := context.Background()

	// Test cases for validation
	testCases := []struct {
		name           string
		data           []map[string]interface{}
		expectedValid  bool
		expectedErrors []string
	}{
		{
			name: "Valid JSONL",
			data: []map[string]interface{}{
				{
					"review_id":  "valid-1",
					"hotel_name": suite.testHotel.Name,
					"rating":     4.0,
					"title":      "Good hotel",
					"comment":    "Nice stay",
					"review_date": time.Now().Format(time.RFC3339),
				},
			},
			expectedValid: true,
		},
		{
			name: "Invalid Rating",
			data: []map[string]interface{}{
				{
					"review_id":  "invalid-1",
					"hotel_name": suite.testHotel.Name,
					"rating":     6.0, // Invalid rating > 5
					"title":      "Good hotel",
					"comment":    "Nice stay",
					"review_date": time.Now().Format(time.RFC3339),
				},
			},
			expectedValid:  false,
			expectedErrors: []string{"rating"},
		},
		{
			name: "Missing Required Fields",
			data: []map[string]interface{}{
				{
					"review_id": "missing-fields-1",
					"rating":    4.0,
					// Missing hotel_name, title, comment, review_date
				},
			},
			expectedValid:  false,
			expectedErrors: []string{"hotel_name", "comment"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create JSONL data
			var jsonlData strings.Builder
			for _, item := range tc.data {
				line, _ := json.Marshal(item)
				jsonlData.Write(line)
				jsonlData.WriteString("\n")
			}

			// Upload to S3
			key := fmt.Sprintf("validation-test-%s-%d.jsonl", tc.name, time.Now().Unix())
			_, err := suite.s3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket:      aws.String("test-bucket"),
				Key:         aws.String(key),
				Body:        strings.NewReader(jsonlData.String()),
				ContentType: aws.String("application/x-jsonlines"),
			})
			require.NoError(t, err)

			// Validate file
			validateRequest := map[string]interface{}{
				"file_url": fmt.Sprintf("s3://test-bucket/%s", key),
			}

			body, _ := json.Marshal(validateRequest)
			resp, err := http.Post(suite.server.URL+"/api/v1/files/validate", "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			defer resp.Body.Close()

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			if tc.expectedValid {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.True(t, response["success"].(bool))
			} else {
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
				assert.False(t, response["success"].(bool))
			}
		})
	}
}

// TestProcessingStatusTracking tests processing status tracking
func (suite *FileProcessingIntegrationSuite) TestProcessingStatusTracking(t *testing.T) {
	// Get processing history
	resp, err := http.Get(suite.server.URL + "/api/v1/files/processing/history?limit=10")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	data := response["data"].([]interface{})
	assert.GreaterOrEqual(t, len(data), 1) // Should have at least one processing record
}

// TestProcessingMetrics tests processing metrics endpoint
func (suite *FileProcessingIntegrationSuite) TestProcessingMetrics(t *testing.T) {
	resp, err := http.Get(suite.server.URL + "/api/v1/files/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	data := response["data"].(map[string]interface{})
	assert.Contains(t, data, "total_files_processed")
	assert.Contains(t, data, "total_records_processed")
	assert.Contains(t, data, "average_processing_time")
}

// TestFileProcessingErrors tests error scenarios in file processing
func (suite *FileProcessingIntegrationSuite) TestFileProcessingErrors(t *testing.T) {
	// Test with non-existent file
	t.Run("Non-existent File", func(t *testing.T) {
		processRequest := map[string]interface{}{
			"file_url":    "s3://test-bucket/non-existent.jsonl",
			"provider_id": suite.testProvider.ID.String(),
		}

		body, _ := json.Marshal(processRequest)
		resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})

	// Test with invalid S3 URL
	t.Run("Invalid S3 URL", func(t *testing.T) {
		processRequest := map[string]interface{}{
			"file_url":    "invalid-url",
			"provider_id": suite.testProvider.ID.String(),
		}

		body, _ := json.Marshal(processRequest)
		resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	// Test with malformed JSON
	t.Run("Malformed JSON", func(t *testing.T) {
		ctx := context.Background()

		// Create malformed JSONL data
		malformedData := `{"review_id": "test", "rating": 4.0, "invalid": json}
{"another": "invalid" json: "line"}`

		key := fmt.Sprintf("malformed-%d.jsonl", time.Now().Unix())
		_, err := suite.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String(key),
			Body:   strings.NewReader(malformedData),
		})
		require.NoError(t, err)

		processRequest := map[string]interface{}{
			"file_url":    fmt.Sprintf("s3://test-bucket/%s", key),
			"provider_id": suite.testProvider.ID.String(),
		}

		body, _ := json.Marshal(processRequest)
		resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should accept the request but processing will fail
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		processingID := data["id"].(string)

		// Wait and check that processing failed
		time.Sleep(5 * time.Second)

		resp, err = http.Get(fmt.Sprintf("%s/api/v1/files/processing/%s", suite.server.URL, processingID))
		require.NoError(t, err)
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		statusData := response["data"].(map[string]interface{})
		status := statusData["status"].(string)
		assert.Contains(t, []string{"failed", "completed"}, status)

		if status == "completed" {
			// Some records should have failed
			assert.Greater(t, statusData["failed_records"].(float64), float64(0))
		}
	})
}

// TestConcurrentFileProcessing tests concurrent file processing
func (suite *FileProcessingIntegrationSuite) TestConcurrentFileProcessing(t *testing.T) {
	ctx := context.Background()
	const numFiles = 3

	processingIDs := make([]string, numFiles)

	// Upload and start processing multiple files concurrently
	for i := 0; i < numFiles; i++ {
		// Create test data
		reviews := []map[string]interface{}{
			{
				"review_id":     fmt.Sprintf("concurrent-%d-1", i),
				"hotel_name":    suite.testHotel.Name,
				"rating":        4.0,
				"title":         fmt.Sprintf("Concurrent review %d-1", i),
				"comment":       fmt.Sprintf("This is concurrent review %d-1", i),
				"review_date":   time.Now().Format(time.RFC3339),
				"reviewer_name": fmt.Sprintf("Concurrent Reviewer %d", i),
			},
			{
				"review_id":     fmt.Sprintf("concurrent-%d-2", i),
				"hotel_name":    suite.testHotel.Name,
				"rating":        3.5,
				"title":         fmt.Sprintf("Concurrent review %d-2", i),
				"comment":       fmt.Sprintf("This is concurrent review %d-2", i),
				"review_date":   time.Now().Format(time.RFC3339),
				"reviewer_name": fmt.Sprintf("Concurrent Reviewer %d", i),
			},
		}

		var jsonlData strings.Builder
		for _, review := range reviews {
			line, _ := json.Marshal(review)
			jsonlData.Write(line)
			jsonlData.WriteString("\n")
		}

		// Upload to S3
		key := fmt.Sprintf("concurrent-%d-%d.jsonl", i, time.Now().Unix())
		_, err := suite.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String("test-bucket"),
			Key:    aws.String(key),
			Body:   strings.NewReader(jsonlData.String()),
		})
		require.NoError(t, err)

		// Start processing
		processRequest := map[string]interface{}{
			"file_url":    fmt.Sprintf("s3://test-bucket/%s", key),
			"provider_id": suite.testProvider.ID.String(),
		}

		body, _ := json.Marshal(processRequest)
		resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		processingIDs[i] = data["id"].(string)
	}

	// Wait for all processing to complete
	for _, processingID := range processingIDs {
		suite.waitForProcessingCompletion(t, processingID, 60*time.Second)

		// Verify completion
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/files/processing/%s", suite.server.URL, processingID))
		require.NoError(t, err)
		defer resp.Body.Close()

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		statusData := response["data"].(map[string]interface{})
		assert.Equal(t, "completed", statusData["status"].(string))
		assert.Equal(t, float64(2), statusData["processed_records"].(float64))
	}
}

// TestProcessingCancellation tests cancelling processing jobs
func (suite *FileProcessingIntegrationSuite) TestProcessingCancellation(t *testing.T) {
	ctx := context.Background()

	// Create a larger file that takes time to process
	var jsonlData strings.Builder
	for i := 0; i < 50; i++ {
		review := map[string]interface{}{
			"review_id":     fmt.Sprintf("cancel-test-%d", i),
			"hotel_name":    suite.testHotel.Name,
			"rating":        4.0,
			"title":         fmt.Sprintf("Cancel test review %d", i),
			"comment":       fmt.Sprintf("This is cancel test review %d with some content", i),
			"review_date":   time.Now().Format(time.RFC3339),
			"reviewer_name": fmt.Sprintf("Cancel Test Reviewer %d", i),
		}

		line, _ := json.Marshal(review)
		jsonlData.Write(line)
		jsonlData.WriteString("\n")
	}

	// Upload to S3
	key := fmt.Sprintf("cancel-test-%d.jsonl", time.Now().Unix())
	_, err := suite.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String(key),
		Body:   strings.NewReader(jsonlData.String()),
	})
	require.NoError(t, err)

	// Start processing
	processRequest := map[string]interface{}{
		"file_url":    fmt.Sprintf("s3://test-bucket/%s", key),
		"provider_id": suite.testProvider.ID.String(),
	}

	body, _ := json.Marshal(processRequest)
	resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	processingID := data["id"].(string)

	// Wait a bit to let processing start
	time.Sleep(1 * time.Second)

	// Cancel processing
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/files/processing/%s/cancel", suite.server.URL, processingID), nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Cancel request should succeed (even if processing already completed)
	assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, resp.StatusCode)
}

// Helper function to wait for processing completion
func (suite *FileProcessingIntegrationSuite) waitForProcessingCompletion(t *testing.T, processingID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/files/processing/%s", suite.server.URL, processingID))
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()
		require.NoError(t, err)

		if !response["success"].(bool) {
			t.Fatalf("Failed to get processing status: %v", response["error"])
		}

		data := response["data"].(map[string]interface{})
		status := data["status"].(string)

		if status == "completed" || status == "failed" {
			return
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("Processing did not complete within timeout of %v", timeout)
}