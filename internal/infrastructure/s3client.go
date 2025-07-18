package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	pkgConfig "github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// S3Client represents the S3 client wrapper
type S3Client struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	config     *pkgConfig.S3Config
	logger     *logger.Logger
}

// ObjectInfo represents S3 object information
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
	ContentType  string
}

// ListObjectsResult represents the result of listing objects
type ListObjectsResult struct {
	Objects       []ObjectInfo
	NextToken     *string
	IsTruncated   bool
	CommonPrefixes []string
}

// DownloadResult represents the result of downloading an object
type DownloadResult struct {
	Body          io.ReadCloser
	ContentLength int64
	ContentType   string
	LastModified  time.Time
	ETag          string
	Metadata      map[string]string
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg *pkgConfig.S3Config, log *logger.Logger) (*S3Client, error) {
	ctx := context.Background()
	
	// Create AWS config
	awsConfig, err := createAWSConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}
	
	// Create S3 client
	s3Client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})
	
	// Create uploader and downloader with custom configurations
	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) {
		u.PartSize = cfg.UploadPartSize
		u.Concurrency = 5
	})
	
	downloader := manager.NewDownloader(s3Client, func(d *manager.Downloader) {
		d.PartSize = cfg.DownloadPartSize
		d.Concurrency = 5
	})
	
	client := &S3Client{
		client:     s3Client,
		uploader:   uploader,
		downloader: downloader,
		config:     cfg,
		logger:     log,
	}
	
	// Test connection
	if err := client.testConnection(ctx); err != nil {
		return nil, fmt.Errorf("S3 connection test failed: %w", err)
	}
	
	log.Info("S3 client initialized successfully",
		"region", cfg.Region,
		"bucket", cfg.Bucket,
		"endpoint", cfg.Endpoint,
	)
	
	return client, nil
}

// createAWSConfig creates AWS configuration
func createAWSConfig(ctx context.Context, cfg *pkgConfig.S3Config) (aws.Config, error) {
	var awsConfig aws.Config
	var err error
	
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use static credentials
		credProvider := credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
		
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credProvider),
		)
	} else {
		// Use default credential chain (IAM roles, environment variables, etc.)
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}
	
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	return awsConfig, nil
}

// testConnection tests the S3 connection
func (s *S3Client) testConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	
	// Try to get bucket location
	_, err := s.client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(s.config.Bucket),
	})
	
	if err != nil {
		return fmt.Errorf("failed to get bucket location: %w", err)
	}
	
	return nil
}

// UploadFile uploads a file to S3
func (s *S3Client) UploadFile(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	start := time.Now()
	
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	
	err := s.retryOperation(ctx, func() error {
		_, err := s.uploader.Upload(ctx, input)
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to upload file to S3",
			"bucket", bucket,
			"key", key,
			"error", err,
		)
		return fmt.Errorf("failed to upload file: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "File uploaded to S3 successfully",
		"bucket", bucket,
		"key", key,
		"duration_ms", duration.Milliseconds(),
	)
	
	return nil
}

// DownloadFile downloads a file from S3
func (s *S3Client) DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	start := time.Now()
	
	// Get object metadata first
	headOutput, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}
	
	// Create a streaming downloader
	getOutput, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "File download from S3 started",
		"bucket", bucket,
		"key", key,
		"size", headOutput.ContentLength,
		"duration_ms", duration.Milliseconds(),
	)
	
	return getOutput.Body, nil
}

// GetFileURL generates a presigned URL for file access
func (s *S3Client) GetFileURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	
	s.logger.DebugContext(ctx, "Generated presigned URL",
		"bucket", bucket,
		"key", key,
		"expiration", expiration,
	)
	
	return request.URL, nil
}

// DeleteFile deletes a file from S3
func (s *S3Client) DeleteFile(ctx context.Context, bucket, key string) error {
	start := time.Now()
	
	err := s.retryOperation(ctx, func() error {
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to delete file from S3",
			"bucket", bucket,
			"key", key,
			"error", err,
		)
		return fmt.Errorf("failed to delete file: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "File deleted from S3 successfully",
		"bucket", bucket,
		"key", key,
		"duration_ms", duration.Milliseconds(),
	)
	
	return nil
}

// ListFiles lists files in S3 bucket with pagination support
func (s *S3Client) ListFiles(ctx context.Context, bucket, prefix string, limit int) ([]string, error) {
	result, err := s.ListObjects(ctx, bucket, prefix, "", limit)
	if err != nil {
		return nil, err
	}
	
	files := make([]string, len(result.Objects))
	for i, obj := range result.Objects {
		files[i] = obj.Key
	}
	
	return files, nil
}

// ListObjects lists objects in S3 bucket with detailed information and pagination
func (s *S3Client) ListObjects(ctx context.Context, bucket, prefix, continuationToken string, maxKeys int) (*ListObjectsResult, error) {
	start := time.Now()
	
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(maxKeys)),
	}
	
	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}
	
	var output *s3.ListObjectsV2Output
	err := s.retryOperation(ctx, func() error {
		var err error
		output, err = s.client.ListObjectsV2(ctx, input)
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list objects in S3",
			"bucket", bucket,
			"prefix", prefix,
			"error", err,
		)
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}
	
	// Convert to ObjectInfo slice
	objects := make([]ObjectInfo, len(output.Contents))
	for i, obj := range output.Contents {
		objects[i] = ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
			StorageClass: string(obj.StorageClass),
		}
	}
	
	// Extract common prefixes
	commonPrefixes := make([]string, len(output.CommonPrefixes))
	for i, cp := range output.CommonPrefixes {
		commonPrefixes[i] = aws.ToString(cp.Prefix)
	}
	
	result := &ListObjectsResult{
		Objects:        objects,
		IsTruncated:    aws.ToBool(output.IsTruncated),
		CommonPrefixes: commonPrefixes,
	}
	
	if output.NextContinuationToken != nil {
		result.NextToken = output.NextContinuationToken
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "Objects listed successfully",
		"bucket", bucket,
		"prefix", prefix,
		"count", len(objects),
		"is_truncated", result.IsTruncated,
		"duration_ms", duration.Milliseconds(),
	)
	
	return result, nil
}

// GetFileMetadata gets file metadata from S3
func (s *S3Client) GetFileMetadata(ctx context.Context, bucket, key string) (map[string]string, error) {
	start := time.Now()
	
	var output *s3.HeadObjectOutput
	err := s.retryOperation(ctx, func() error {
		var err error
		output, err = s.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get file metadata from S3",
			"bucket", bucket,
			"key", key,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}
	
	metadata := make(map[string]string)
	
	// Add standard metadata
	metadata["content-type"] = aws.ToString(output.ContentType)
	metadata["content-length"] = fmt.Sprintf("%d", output.ContentLength)
	metadata["last-modified"] = aws.ToTime(output.LastModified).Format(time.RFC3339)
	metadata["etag"] = aws.ToString(output.ETag)
	
	if output.StorageClass != "" {
		metadata["storage-class"] = string(output.StorageClass)
	}
	
	// Add custom metadata
	for k, v := range output.Metadata {
		metadata[k] = v
	}
	
	duration := time.Since(start)
	s.logger.DebugContext(ctx, "File metadata retrieved successfully",
		"bucket", bucket,
		"key", key,
		"duration_ms", duration.Milliseconds(),
	)
	
	return metadata, nil
}

// UpdateFileMetadata updates file metadata in S3
func (s *S3Client) UpdateFileMetadata(ctx context.Context, bucket, key string, metadata map[string]string) error {
	start := time.Now()
	
	// First get the current object
	getOutput, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object for metadata update: %w", err)
	}
	defer getOutput.Body.Close()
	
	// Prepare metadata for update
	s3Metadata := make(map[string]string)
	var contentType string
	
	for k, v := range metadata {
		if k == "content-type" {
			contentType = v
		} else {
			s3Metadata[k] = v
		}
	}
	
	// Copy object with new metadata
	err = s.retryOperation(ctx, func() error {
		_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:            aws.String(bucket),
			Key:               aws.String(key),
			CopySource:        aws.String(fmt.Sprintf("%s/%s", bucket, key)),
			Metadata:          s3Metadata,
			MetadataDirective: types.MetadataDirectiveReplace,
			ContentType:       aws.String(contentType),
		})
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to update file metadata in S3",
			"bucket", bucket,
			"key", key,
			"error", err,
		)
		return fmt.Errorf("failed to update file metadata: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "File metadata updated successfully",
		"bucket", bucket,
		"key", key,
		"duration_ms", duration.Milliseconds(),
	)
	
	return nil
}

// CreateBucket creates a new S3 bucket
func (s *S3Client) CreateBucket(ctx context.Context, bucket string) error {
	start := time.Now()
	
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}
	
	// Add location constraint if not in us-east-1
	if s.config.Region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(s.config.Region),
		}
	}
	
	err := s.retryOperation(ctx, func() error {
		_, err := s.client.CreateBucket(ctx, input)
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to create S3 bucket",
			"bucket", bucket,
			"error", err,
		)
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "S3 bucket created successfully",
		"bucket", bucket,
		"duration_ms", duration.Milliseconds(),
	)
	
	return nil
}

// DeleteBucket deletes an S3 bucket
func (s *S3Client) DeleteBucket(ctx context.Context, bucket string) error {
	start := time.Now()
	
	err := s.retryOperation(ctx, func() error {
		_, err := s.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to delete S3 bucket",
			"bucket", bucket,
			"error", err,
		)
		return fmt.Errorf("failed to delete bucket: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "S3 bucket deleted successfully",
		"bucket", bucket,
		"duration_ms", duration.Milliseconds(),
	)
	
	return nil
}

// BucketExists checks if a bucket exists
func (s *S3Client) BucketExists(ctx context.Context, bucket string) (bool, error) {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	
	return true, nil
}

// FileExists checks if a file exists in S3
func (s *S3Client) FileExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	
	return true, nil
}

// GetFileSize gets the size of a file in S3
func (s *S3Client) GetFileSize(ctx context.Context, bucket, key string) (int64, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}
	
	return aws.ToInt64(output.ContentLength), nil
}

// retryOperation performs an operation with retry logic
func (s *S3Client) retryOperation(ctx context.Context, operation func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= s.config.RetryCount; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.config.RetryDelay * time.Duration(attempt)):
				// Exponential backoff
			}
		}
		
		err := operation()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !s.isRetryableError(err) {
			break
		}
		
		s.logger.WarnContext(ctx, "S3 operation failed, retrying...",
			"attempt", attempt+1,
			"max_retries", s.config.RetryCount,
			"error", err,
		)
	}
	
	return lastErr
}

// isRetryableError checks if an error is retryable
func (s *S3Client) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		errorCode := apiErr.ErrorCode()
		
		// Common retryable errors
		retryableErrors := []string{
			"InternalError",
			"ServiceUnavailable",
			"SlowDown",
			"RequestTimeout",
			"ThrottlingException",
		}
		
		for _, retryable := range retryableErrors {
			if errorCode == retryable {
				return true
			}
		}
		
		// HTTP 5xx errors are generally retryable
		if strings.HasPrefix(errorCode, "5") {
			return true
		}
	}
	
	return false
}

// DownloadToWriter downloads a file from S3 to a writer (streaming)
func (s *S3Client) DownloadToWriter(ctx context.Context, bucket, key string, writer io.WriterAt) error {
	start := time.Now()
	
	err := s.retryOperation(ctx, func() error {
		_, err := s.downloader.Download(ctx, writer, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to download file from S3",
			"bucket", bucket,
			"key", key,
			"error", err,
		)
		return fmt.Errorf("failed to download file: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "File downloaded from S3 successfully",
		"bucket", bucket,
		"key", key,
		"duration_ms", duration.Milliseconds(),
	)
	
	return nil
}

// UploadFromReader uploads a file to S3 from a reader (streaming)
func (s *S3Client) UploadFromReader(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	start := time.Now()
	
	err := s.retryOperation(ctx, func() error {
		_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        reader,
			ContentType: aws.String(contentType),
		})
		return err
	})
	
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to upload file to S3",
			"bucket", bucket,
			"key", key,
			"error", err,
		)
		return fmt.Errorf("failed to upload file: %w", err)
	}
	
	duration := time.Since(start)
	s.logger.InfoContext(ctx, "File uploaded to S3 successfully",
		"bucket", bucket,
		"key", key,
		"duration_ms", duration.Milliseconds(),
	)
	
	return nil
}