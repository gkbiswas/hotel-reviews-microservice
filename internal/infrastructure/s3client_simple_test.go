package infrastructure

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/suite"

	pkgConfig "github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// S3ClientSimpleTestSuite provides basic testing for S3Client business logic
type S3ClientSimpleTestSuite struct {
	suite.Suite
	ctx      context.Context
	s3Client *S3Client
	logger   *logger.Logger
	config   *pkgConfig.S3Config
}

func (suite *S3ClientSimpleTestSuite) SetupTest() {
	suite.ctx = context.Background()

	slogLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	suite.logger = &logger.Logger{Logger: slogLogger}

	suite.config = &pkgConfig.S3Config{
		Region:           "us-east-1",
		Bucket:           "test-bucket",
		AccessKeyID:      "test-access-key",
		SecretAccessKey:  "test-secret-key",
		Endpoint:         "http://localhost:9000",
		ForcePathStyle:   true,
		UploadPartSize:   5 * 1024 * 1024,
		DownloadPartSize: 5 * 1024 * 1024,
		RetryCount:       3,
		RetryDelay:       time.Second,
		Timeout:          30 * time.Second,
	}

	suite.s3Client = &S3Client{
		config: suite.config,
		logger: suite.logger,
	}
}

// Test isRetryableError method comprehensively
func (suite *S3ClientSimpleTestSuite) TestIsRetryableError_Comprehensive() {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error should not be retryable",
			err:       nil,
			retryable: false,
		},
		{
			name:      "non-API error should not be retryable",
			err:       errors.New("regular error"),
			retryable: false,
		},
		{
			name: "InternalError should be retryable",
			err: &smithy.GenericAPIError{
				Code:    "InternalError",
				Message: "internal server error",
			},
			retryable: true,
		},
		{
			name: "ServiceUnavailable should be retryable",
			err: &smithy.GenericAPIError{
				Code:    "ServiceUnavailable",
				Message: "service unavailable",
			},
			retryable: true,
		},
		{
			name: "SlowDown should be retryable",
			err: &smithy.GenericAPIError{
				Code:    "SlowDown",
				Message: "slow down",
			},
			retryable: true,
		},
		{
			name: "RequestTimeout should be retryable",
			err: &smithy.GenericAPIError{
				Code:    "RequestTimeout",
				Message: "request timeout",
			},
			retryable: true,
		},
		{
			name: "ThrottlingException should be retryable",
			err: &smithy.GenericAPIError{
				Code:    "ThrottlingException",
				Message: "throttling exception",
			},
			retryable: true,
		},
		{
			name: "5xx HTTP errors should be retryable",
			err: &smithy.GenericAPIError{
				Code:    "500",
				Message: "internal server error",
			},
			retryable: true,
		},
		{
			name: "503 HTTP error should be retryable",
			err: &smithy.GenericAPIError{
				Code:    "503",
				Message: "service unavailable",
			},
			retryable: true,
		},
		{
			name: "NotFound should not be retryable",
			err: &smithy.GenericAPIError{
				Code:    "NotFound",
				Message: "not found",
			},
			retryable: false,
		},
		{
			name: "AccessDenied should not be retryable",
			err: &smithy.GenericAPIError{
				Code:    "AccessDenied",
				Message: "access denied",
			},
			retryable: false,
		},
		{
			name: "BadRequest should not be retryable",
			err: &smithy.GenericAPIError{
				Code:    "BadRequest",
				Message: "bad request",
			},
			retryable: false,
		},
		{
			name: "4xx HTTP errors should not be retryable",
			err: &smithy.GenericAPIError{
				Code:    "400",
				Message: "bad request",
			},
			retryable: false,
		},
		{
			name: "403 HTTP error should not be retryable",
			err: &smithy.GenericAPIError{
				Code:    "403",
				Message: "forbidden",
			},
			retryable: false,
		},
		{
			name: "404 HTTP error should not be retryable",
			err: &smithy.GenericAPIError{
				Code:    "404",
				Message: "not found",
			},
			retryable: false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := suite.s3Client.isRetryableError(tt.err)
			suite.Equal(tt.retryable, result, "Error classification mismatch for %s", tt.name)
		})
	}
}

// Test S3 URL parsing logic (re-implemented for testing)
func (suite *S3ClientSimpleTestSuite) TestS3URLParsing() {
	// Test S3 URL parsing logic
	parseS3URL := func(url string) (bucket, key string, err error) {
		if !strings.HasPrefix(url, "s3://") {
			return "", "", errors.New("invalid S3 URL format")
		}
		parts := strings.Split(strings.TrimPrefix(url, "s3://"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return "", "", errors.New("invalid S3 URL format")
		}
		return parts[0], strings.Join(parts[1:], "/"), nil
	}

	tests := []struct {
		name           string
		url            string
		expectedBucket string
		expectedKey    string
		expectError    bool
	}{
		{
			name:           "valid S3 URL with single file",
			url:            "s3://bucket-name/file.txt",
			expectedBucket: "bucket-name",
			expectedKey:    "file.txt",
			expectError:    false,
		},
		{
			name:           "valid S3 URL with path",
			url:            "s3://bucket-name/path/to/file.txt",
			expectedBucket: "bucket-name",
			expectedKey:    "path/to/file.txt",
			expectError:    false,
		},
		{
			name:           "valid S3 URL with nested path",
			url:            "s3://my-bucket/folder/subfolder/file.json",
			expectedBucket: "my-bucket",
			expectedKey:    "folder/subfolder/file.json",
			expectError:    false,
		},
		{
			name:           "valid S3 URL with special characters",
			url:            "s3://test-bucket-123/path_with-special.chars/file-name_123.json",
			expectedBucket: "test-bucket-123",
			expectedKey:    "path_with-special.chars/file-name_123.json",
			expectError:    false,
		},
		{
			name:        "invalid S3 URL - no key",
			url:         "s3://bucket-only",
			expectError: true,
		},
		{
			name:        "invalid S3 URL - no bucket",
			url:         "s3:///file.txt",
			expectError: true,
		},
		{
			name:        "invalid S3 URL - malformed",
			url:         "not-an-s3-url",
			expectError: true,
		},
		{
			name:        "invalid S3 URL - wrong protocol",
			url:         "http://bucket/file.txt",
			expectError: true,
		},
		{
			name:        "empty URL",
			url:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			bucket, key, err := parseS3URL(tt.url)

			if tt.expectError {
				suite.Error(err, "Expected error for URL: %s", tt.url)
			} else {
				suite.NoError(err, "Unexpected error for URL: %s", tt.url)
				suite.Equal(tt.expectedBucket, bucket, "Bucket mismatch for URL: %s", tt.url)
				suite.Equal(tt.expectedKey, key, "Key mismatch for URL: %s", tt.url)
			}
		})
	}
}

// Test retryOperation with various scenarios
func (suite *S3ClientSimpleTestSuite) TestRetryOperation_ComprehensiveScenarios() {
	suite.Run("operation succeeds on first try", func() {
		callCount := 0
		operation := func() error {
			callCount++
			return nil
		}

		err := suite.s3Client.retryOperation(suite.ctx, operation)

		suite.NoError(err)
		suite.Equal(1, callCount, "Should succeed on first try")
	})

	suite.Run("operation succeeds after retries", func() {
		callCount := 0
		operation := func() error {
			callCount++
			if callCount < 3 {
				return &smithy.GenericAPIError{
					Code:    "InternalError",
					Message: "temporary error",
				}
			}
			return nil
		}

		err := suite.s3Client.retryOperation(suite.ctx, operation)

		suite.NoError(err)
		suite.Equal(3, callCount, "Should retry until success")
	})

	suite.Run("operation fails with non-retryable error", func() {
		callCount := 0
		operation := func() error {
			callCount++
			return &smithy.GenericAPIError{
				Code:    "NotFound",
				Message: "resource not found",
			}
		}

		err := suite.s3Client.retryOperation(suite.ctx, operation)

		suite.Error(err)
		suite.Equal(1, callCount, "Should not retry non-retryable error")
		suite.Contains(err.Error(), "NotFound")
	})

	suite.Run("operation exhausts all retries", func() {
		callCount := 0
		operation := func() error {
			callCount++
			return &smithy.GenericAPIError{
				Code:    "ServiceUnavailable",
				Message: "service unavailable",
			}
		}

		err := suite.s3Client.retryOperation(suite.ctx, operation)

		suite.Error(err)
		suite.Equal(suite.s3Client.config.RetryCount+1, callCount, "Should exhaust all retries")
		suite.Contains(err.Error(), "ServiceUnavailable")
	})

	suite.Run("context cancellation during retry", func() {
		// Create a context that will be cancelled
		ctx, cancel := context.WithCancel(suite.ctx)

		callCount := 0
		operation := func() error {
			callCount++
			if callCount == 2 {
				// Cancel context on second call
				cancel()
			}
			return &smithy.GenericAPIError{
				Code:    "ThrottlingException",
				Message: "throttled",
			}
		}

		err := suite.s3Client.retryOperation(ctx, operation)

		suite.Error(err)
		// Should stop retrying when context is cancelled
		suite.LessOrEqual(callCount, suite.s3Client.config.RetryCount+1)
	})
}

// Test configuration edge cases
func (suite *S3ClientSimpleTestSuite) TestConfigurationHandling() {
	suite.Run("retry configuration values", func() {
		suite.Equal(3, suite.s3Client.config.RetryCount)
		suite.Equal(time.Second, suite.s3Client.config.RetryDelay)
		suite.Equal(30*time.Second, suite.s3Client.config.Timeout)
	})

	suite.Run("S3 configuration values", func() {
		suite.Equal("us-east-1", suite.s3Client.config.Region)
		suite.Equal("test-bucket", suite.s3Client.config.Bucket)
		suite.Equal("http://localhost:9000", suite.s3Client.config.Endpoint)
		suite.True(suite.s3Client.config.ForcePathStyle)
		suite.Equal(int64(5*1024*1024), suite.s3Client.config.UploadPartSize)
		suite.Equal(int64(5*1024*1024), suite.s3Client.config.DownloadPartSize)
	})
}

// Test performance of retry logic
func (suite *S3ClientSimpleTestSuite) TestRetryPerformance() {
	// Test that successful operations are fast
	operationCount := 100
	start := time.Now()

	for i := 0; i < operationCount; i++ {
		operation := func() error {
			return nil // Always succeed
		}

		err := suite.s3Client.retryOperation(suite.ctx, operation)
		suite.NoError(err)
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(operationCount)

	suite.T().Logf("Performance test: %d retry operations in %v, avg: %v per operation",
		operationCount, duration, avgDuration)

	// Each successful operation should be very fast (< 1ms)
	suite.Less(avgDuration, time.Millisecond, "Successful retry operations should be fast")
}

// Test error message formatting
func (suite *S3ClientSimpleTestSuite) TestErrorHandling() {
	suite.Run("API error preserves details", func() {
		apiErr := &smithy.GenericAPIError{
			Code:    "TestError",
			Message: "Test error message",
		}

		result := suite.s3Client.isRetryableError(apiErr)
		suite.False(result) // TestError is not in retryable list
	})

	suite.Run("nil error handling", func() {
		result := suite.s3Client.isRetryableError(nil)
		suite.False(result)
	})

	suite.Run("generic error handling", func() {
		genericErr := errors.New("generic error")
		result := suite.s3Client.isRetryableError(genericErr)
		suite.False(result)
	})
}

// Test struct initialization
func (suite *S3ClientSimpleTestSuite) TestStructFields() {
	suite.Run("S3Client has required fields", func() {
		suite.NotNil(suite.s3Client.config)
		suite.NotNil(suite.s3Client.logger)
	})

	suite.Run("ObjectInfo struct can be created", func() {
		obj := ObjectInfo{
			Key:          "test-key",
			Size:         1024,
			LastModified: time.Now(),
			ETag:         "test-etag",
			StorageClass: "STANDARD",
			ContentType:  "application/json",
		}

		suite.Equal("test-key", obj.Key)
		suite.Equal(int64(1024), obj.Size)
		suite.Equal("test-etag", obj.ETag)
		suite.Equal("STANDARD", obj.StorageClass)
		suite.Equal("application/json", obj.ContentType)
	})

	suite.Run("ListObjectsResult struct can be created", func() {
		result := ListObjectsResult{
			Objects: []ObjectInfo{
				{Key: "file1.txt", Size: 100},
				{Key: "file2.txt", Size: 200},
			},
			IsTruncated:    false,
			CommonPrefixes: []string{"folder1/", "folder2/"},
		}

		suite.Len(result.Objects, 2)
		suite.Equal("file1.txt", result.Objects[0].Key)
		suite.Equal("file2.txt", result.Objects[1].Key)
		suite.False(result.IsTruncated)
		suite.Len(result.CommonPrefixes, 2)
	})

	suite.Run("DownloadResult struct can be created", func() {
		result := DownloadResult{
			ContentLength: 1024,
			ContentType:   "text/plain",
			LastModified:  time.Now(),
			ETag:          "test-etag",
			Metadata:      map[string]string{"key": "value"},
		}

		suite.Equal(int64(1024), result.ContentLength)
		suite.Equal("text/plain", result.ContentType)
		suite.Equal("test-etag", result.ETag)
		suite.Equal("value", result.Metadata["key"])
	})
}

func TestS3ClientSimpleSuite(t *testing.T) {
	suite.Run(t, new(S3ClientSimpleTestSuite))
}
