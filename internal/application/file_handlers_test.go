package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock S3Client
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) UploadFile(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	args := m.Called(ctx, bucket, key, body, contentType)
	return args.Error(0)
}

func (m *MockS3Client) DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockS3Client) GetFileMetadata(ctx context.Context, bucket, key string) (map[string]string, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

// Mock JSONProcessor
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockJSONProcessor) ParseHotel(ctx context.Context, jsonLine []byte) (*domain.Hotel, error) {
	args := m.Called(ctx, jsonLine)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Hotel), args.Error(1)
}

func (m *MockJSONProcessor) ParseReviewerInfo(ctx context.Context, jsonLine []byte) (*domain.ReviewerInfo, error) {
	args := m.Called(ctx, jsonLine)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

// Helper to create multipart file upload
func createMultipartFileUpload(fieldName, fileName string, content []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, "", err
	}

	_, err = part.Write(content)
	if err != nil {
		return nil, "", err
	}

	err = writer.Close()
	if err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}

// Test setup
func setupFileHandlersTest() (*gin.Engine, *MockReviewService, *MockS3Client, *MockJSONProcessor, *ProcessingEngine) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockReviewService := &MockReviewService{}
	mockS3Client := &MockS3Client{}
	mockJSONProcessor := &MockJSONProcessor{}

	// Create a mock processing engine with proper config
	testLogger, err := logger.New(&logger.Config{
		Level:            "info",
		Format:           "json",
		EnableStacktrace: false,
	})
	if err != nil {
		panic(err)
	}
	processingConfig := &ProcessingConfig{
		MaxWorkers:         2,
		MaxConcurrentFiles: 1,
		MaxRetries:         3,
		RetryDelay:         time.Second,
		ProcessingTimeout:  time.Minute * 30,
		WorkerIdleTimeout:  time.Minute * 5,
		MetricsInterval:    time.Second * 30,
	}
	processingEngine := NewProcessingEngine(mockReviewService, mockS3Client, mockJSONProcessor, testLogger, processingConfig)

	fileHandlers := NewFileHandlers(
		mockReviewService,
		mockS3Client,
		mockJSONProcessor,
		processingEngine,
		testLogger,
		"test-bucket",
	)

	api := router.Group("/api/v1")
	{
		files := api.Group("/files")
		{
			files.POST("/upload", fileHandlers.UploadAndProcessFile)
			files.GET("/processing/:id", fileHandlers.GetProcessingStatus)
			files.GET("/processing/history", fileHandlers.GetProcessingHistory)
			files.POST("/processing/:id/cancel", fileHandlers.CancelProcessing)
			files.POST("/validate", fileHandlers.ValidateFile)
			files.GET("/metrics", fileHandlers.GetProcessingMetrics)
		}
	}

	return router, mockReviewService, mockS3Client, mockJSONProcessor, processingEngine
}

func TestFileHandlers_UploadAndProcessFile(t *testing.T) {
	providerID := uuid.New()

	tests := []struct {
		name           string
		providerID     string
		fileContent    string
		fileName       string
		setupMocks     func(*MockReviewService, *MockS3Client, *MockJSONProcessor)
		expectedStatus int
		expectError    bool
	}{
		{
			name:        "successful file upload and processing",
			providerID:  providerID.String(),
			fileContent: `{"hotel": "Test Hotel", "rating": 4.5}`,
			fileName:    "reviews.jsonl",
			setupMocks: func(mrs *MockReviewService, ms3 *MockS3Client, mjp *MockJSONProcessor) {
				ms3.On("UploadFile", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string")).Return(nil)
				processingStatus := &domain.ReviewProcessingStatus{
					ID:       uuid.New(),
					Status:   "processing",
					FileURL:  fmt.Sprintf("s3://test-bucket/reviews/%s/", providerID.String()),
				}
				mrs.On("ProcessReviewFile", mock.Anything, mock.AnythingOfType("string"), providerID).Return(processingStatus, nil)
			},
			expectedStatus: http.StatusAccepted,
			expectError:    false,
		},
		{
			name:        "missing provider ID",
			providerID:  "",
			fileContent: `{"hotel": "Test Hotel", "rating": 4.5}`,
			fileName:    "reviews.jsonl",
			setupMocks:  func(mrs *MockReviewService, ms3 *MockS3Client, mjp *MockJSONProcessor) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:        "invalid provider ID",
			providerID:  "invalid-uuid",
			fileContent: `{"hotel": "Test Hotel", "rating": 4.5}`,
			fileName:    "reviews.jsonl",
			setupMocks:  func(mrs *MockReviewService, ms3 *MockS3Client, mjp *MockJSONProcessor) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:        "unsupported file format",
			providerID:  providerID.String(),
			fileContent: "This is a text file",
			fileName:    "reviews.txt",
			setupMocks:  func(mrs *MockReviewService, ms3 *MockS3Client, mjp *MockJSONProcessor) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:        "S3 upload failure",
			providerID:  providerID.String(),
			fileContent: `{"hotel": "Test Hotel", "rating": 4.5}`,
			fileName:    "reviews.jsonl",
			setupMocks: func(mrs *MockReviewService, ms3 *MockS3Client, mjp *MockJSONProcessor) {
				ms3.On("UploadFile", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string")).Return(errors.New("S3 error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockReviewService, mockS3Client, mockJSONProcessor, _ := setupFileHandlersTest()
			tt.setupMocks(mockReviewService, mockS3Client, mockJSONProcessor)

			// Create multipart upload
			body, contentType, err := createMultipartFileUpload("file", tt.fileName, []byte(tt.fileContent))
			require.NoError(t, err)

			// Create request
			url := "/api/v1/files/upload"
			if tt.providerID != "" {
				url += "?provider_id=" + tt.providerID
			}

			req := httptest.NewRequest(http.MethodPost, url, body)
			req.Header.Set("Content-Type", contentType)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectError {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			} else {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "data")
			}

			mockReviewService.AssertExpectations(t)
			mockS3Client.AssertExpectations(t)
			mockJSONProcessor.AssertExpectations(t)
		})
	}
}

func TestFileHandlers_GetProcessingStatus(t *testing.T) {
	processingID := uuid.New()

	tests := []struct {
		name           string
		processingID   string
		setupMocks     func(*MockReviewService)
		expectedStatus int
	}{
		{
			name:         "successful status retrieval",
			processingID: processingID.String(),
			setupMocks: func(mrs *MockReviewService) {
				status := &domain.ReviewProcessingStatus{
					ID:               processingID,
					Status:           "completed",
					RecordsProcessed: 100,
					RecordsTotal:     100,
				}
				mrs.On("GetProcessingStatus", mock.Anything, processingID).Return(status, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid processing ID",
			processingID:   "invalid-uuid",
			setupMocks:     func(mrs *MockReviewService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:         "processing not found",
			processingID: processingID.String(),
			setupMocks: func(mrs *MockReviewService) {
				mrs.On("GetProcessingStatus", mock.Anything, processingID).Return(nil, domain.ErrProcessingNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockReviewService, _, _, _ := setupFileHandlersTest()
			tt.setupMocks(mockReviewService)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/processing/%s", tt.processingID), nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockReviewService.AssertExpectations(t)
		})
	}
}

func TestFileHandlers_ValidateFile(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		fileName       string
		setupMocks     func(*MockJSONProcessor)
		expectedStatus int
		expectValid    bool
	}{
		{
			name:        "valid JSON Lines file",
			fileContent: `{"hotel": "Test Hotel", "rating": 4.5}`,
			fileName:    "reviews.jsonl",
			setupMocks: func(mjp *MockJSONProcessor) {
				mjp.On("ValidateFile", mock.Anything, mock.Anything).Return(nil)
				mjp.On("CountRecords", mock.Anything, mock.Anything).Return(1, nil)
			},
			expectedStatus: http.StatusOK,
			expectValid:    true,
		},
		{
			name:        "invalid file content",
			fileContent: `{"hotel": "Test Hotel", "rating": 4.5}`,
			fileName:    "reviews.jsonl",
			setupMocks: func(mjp *MockJSONProcessor) {
				mjp.On("ValidateFile", mock.Anything, mock.Anything).Return(errors.New("invalid JSON format"))
				mjp.On("CountRecords", mock.Anything, mock.Anything).Return(0, errors.New("count error"))
			},
			expectedStatus: http.StatusOK,
			expectValid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _, _, mockJSONProcessor, _ := setupFileHandlersTest()
			tt.setupMocks(mockJSONProcessor)

			// Create multipart upload
			body, contentType, err := createMultipartFileUpload("file", tt.fileName, []byte(tt.fileContent))
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/files/validate", body)
			req.Header.Set("Content-Type", contentType)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectValid, response["valid"])

			mockJSONProcessor.AssertExpectations(t)
		})
	}
}