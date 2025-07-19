package infrastructure

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// S3ClientWrapper implements domain.S3Client using AWS SDK v2
type S3ClientWrapper struct {
	client *s3.Client
	logger *logger.Logger
}

// NewS3ClientWrapper creates a new S3 client wrapper
func NewS3ClientWrapper(client *s3.Client, logger *logger.Logger) domain.S3Client {
	return &S3ClientWrapper{
		client: client,
		logger: logger,
	}
}

// UploadFile uploads a file to S3
func (s *S3ClientWrapper) UploadFile(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

// DownloadFile downloads a file from S3
func (s *S3ClientWrapper) DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

// GetFileURL generates a presigned URL for a file
func (s *S3ClientWrapper) GetFileURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	// For simplicity, return a direct URL - in real implementation this would be presigned
	return "s3://" + bucket + "/" + key, nil
}

// DeleteFile deletes a file from S3
func (s *S3ClientWrapper) DeleteFile(ctx context.Context, bucket, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// ListFiles lists files in S3
func (s *S3ClientWrapper) ListFiles(ctx context.Context, bucket, prefix string, limit int) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(limit)),
	}

	output, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, object := range output.Contents {
		files = append(files, *object.Key)
	}

	return files, nil
}

// GetFileMetadata gets metadata for a file
func (s *S3ClientWrapper) GetFileMetadata(ctx context.Context, bucket, key string) (map[string]string, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	metadata := make(map[string]string)
	for k, v := range output.Metadata {
		metadata[k] = v
	}

	return metadata, nil
}

// UpdateFileMetadata updates metadata for a file
func (s *S3ClientWrapper) UpdateFileMetadata(ctx context.Context, bucket, key string, metadata map[string]string) error {
	// Copy object with new metadata
	copySource := bucket + "/" + key
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		CopySource: aws.String(copySource),
		Metadata:   metadata,
	})
	return err
}

// CreateBucket creates a new S3 bucket
func (s *S3ClientWrapper) CreateBucket(ctx context.Context, bucket string) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

// DeleteBucket deletes an S3 bucket
func (s *S3ClientWrapper) DeleteBucket(ctx context.Context, bucket string) error {
	_, err := s.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

// BucketExists checks if a bucket exists
func (s *S3ClientWrapper) BucketExists(ctx context.Context, bucket string) (bool, error) {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FileExists checks if a file exists
func (s *S3ClientWrapper) FileExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetFileSize gets the size of a file
func (s *S3ClientWrapper) GetFileSize(ctx context.Context, bucket, key string) (int64, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}

	if output.ContentLength != nil {
		return *output.ContentLength, nil
	}
	return 0, nil
}
