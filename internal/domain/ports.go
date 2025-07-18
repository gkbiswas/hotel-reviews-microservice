package domain

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// ReviewRepository defines the interface for review data persistence
type ReviewRepository interface {
	// Review operations
	CreateBatch(ctx context.Context, reviews []Review) error
	GetByID(ctx context.Context, id uuid.UUID) (*Review, error)
	GetByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]Review, error)
	GetByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]Review, error)
	GetByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int) ([]Review, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
	Search(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]Review, error)
	GetTotalCount(ctx context.Context, filters map[string]interface{}) (int64, error)
	
	// Hotel operations
	CreateHotel(ctx context.Context, hotel *Hotel) error
	GetHotelByID(ctx context.Context, id uuid.UUID) (*Hotel, error)
	GetHotelByName(ctx context.Context, name string) (*Hotel, error)
	UpdateHotel(ctx context.Context, hotel *Hotel) error
	DeleteHotel(ctx context.Context, id uuid.UUID) error
	ListHotels(ctx context.Context, limit, offset int) ([]Hotel, error)
	
	// Provider operations
	CreateProvider(ctx context.Context, provider *Provider) error
	GetProviderByID(ctx context.Context, id uuid.UUID) (*Provider, error)
	GetProviderByName(ctx context.Context, name string) (*Provider, error)
	UpdateProvider(ctx context.Context, provider *Provider) error
	DeleteProvider(ctx context.Context, id uuid.UUID) error
	ListProviders(ctx context.Context, limit, offset int) ([]Provider, error)
	
	// ReviewerInfo operations
	CreateReviewerInfo(ctx context.Context, reviewerInfo *ReviewerInfo) error
	GetReviewerInfoByID(ctx context.Context, id uuid.UUID) (*ReviewerInfo, error)
	GetReviewerInfoByEmail(ctx context.Context, email string) (*ReviewerInfo, error)
	UpdateReviewerInfo(ctx context.Context, reviewerInfo *ReviewerInfo) error
	DeleteReviewerInfo(ctx context.Context, id uuid.UUID) error
	
	// Review summary operations
	CreateOrUpdateReviewSummary(ctx context.Context, summary *ReviewSummary) error
	GetReviewSummaryByHotelID(ctx context.Context, hotelID uuid.UUID) (*ReviewSummary, error)
	UpdateReviewSummary(ctx context.Context, hotelID uuid.UUID) error
	
	// Review processing status operations
	CreateProcessingStatus(ctx context.Context, status *ReviewProcessingStatus) error
	GetProcessingStatusByID(ctx context.Context, id uuid.UUID) (*ReviewProcessingStatus, error)
	GetProcessingStatusByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]ReviewProcessingStatus, error)
	UpdateProcessingStatus(ctx context.Context, id uuid.UUID, status string, recordsProcessed int, errorMsg string) error
	DeleteProcessingStatus(ctx context.Context, id uuid.UUID) error
}

// S3Client defines the interface for AWS S3 operations
type S3Client interface {
	// File operations
	UploadFile(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
	DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	GetFileURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error)
	DeleteFile(ctx context.Context, bucket, key string) error
	ListFiles(ctx context.Context, bucket, prefix string, limit int) ([]string, error)
	
	// File metadata operations
	GetFileMetadata(ctx context.Context, bucket, key string) (map[string]string, error)
	UpdateFileMetadata(ctx context.Context, bucket, key string, metadata map[string]string) error
	
	// Bucket operations
	CreateBucket(ctx context.Context, bucket string) error
	DeleteBucket(ctx context.Context, bucket string) error
	BucketExists(ctx context.Context, bucket string) (bool, error)
	
	// File existence and size
	FileExists(ctx context.Context, bucket, key string) (bool, error)
	GetFileSize(ctx context.Context, bucket, key string) (int64, error)
}

// JSONProcessor defines the interface for processing JSON Lines files
type JSONProcessor interface {
	// File processing
	ProcessFile(ctx context.Context, reader io.Reader, providerID uuid.UUID, processingID uuid.UUID) error
	ValidateFile(ctx context.Context, reader io.Reader) error
	CountRecords(ctx context.Context, reader io.Reader) (int, error)
	
	// Data transformation
	ParseReview(ctx context.Context, jsonLine []byte, providerID uuid.UUID) (*Review, error)
	ParseHotel(ctx context.Context, jsonLine []byte) (*Hotel, error)
	ParseReviewerInfo(ctx context.Context, jsonLine []byte) (*ReviewerInfo, error)
	
	// Validation
	ValidateReview(ctx context.Context, review *Review) error
	ValidateHotel(ctx context.Context, hotel *Hotel) error
	ValidateReviewerInfo(ctx context.Context, reviewerInfo *ReviewerInfo) error
	
	// Batch processing
	ProcessBatch(ctx context.Context, reviews []Review) error
	GetBatchSize() int
	SetBatchSize(size int)
}

// ReviewService defines the interface for review business logic
type ReviewService interface {
	// Review operations
	CreateReview(ctx context.Context, review *Review) error
	GetReviewByID(ctx context.Context, id uuid.UUID) (*Review, error)
	GetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]Review, error)
	GetReviewsByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]Review, error)
	UpdateReview(ctx context.Context, review *Review) error
	DeleteReview(ctx context.Context, id uuid.UUID) error
	SearchReviews(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]Review, error)
	
	// Hotel operations
	CreateHotel(ctx context.Context, hotel *Hotel) error
	GetHotelByID(ctx context.Context, id uuid.UUID) (*Hotel, error)
	UpdateHotel(ctx context.Context, hotel *Hotel) error
	DeleteHotel(ctx context.Context, id uuid.UUID) error
	ListHotels(ctx context.Context, limit, offset int) ([]Hotel, error)
	
	// Provider operations
	CreateProvider(ctx context.Context, provider *Provider) error
	GetProviderByID(ctx context.Context, id uuid.UUID) (*Provider, error)
	GetProviderByName(ctx context.Context, name string) (*Provider, error)
	UpdateProvider(ctx context.Context, provider *Provider) error
	DeleteProvider(ctx context.Context, id uuid.UUID) error
	ListProviders(ctx context.Context, limit, offset int) ([]Provider, error)
	
	// File processing operations
	ProcessReviewFile(ctx context.Context, fileURL string, providerID uuid.UUID) (*ReviewProcessingStatus, error)
	GetProcessingStatus(ctx context.Context, id uuid.UUID) (*ReviewProcessingStatus, error)
	GetProcessingHistory(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]ReviewProcessingStatus, error)
	CancelProcessing(ctx context.Context, id uuid.UUID) error
	
	// Analytics operations
	GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*ReviewSummary, error)
	GetReviewStatsByProvider(ctx context.Context, providerID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error)
	GetReviewStatsByHotel(ctx context.Context, hotelID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error)
	GetTopRatedHotels(ctx context.Context, limit int) ([]Hotel, error)
	GetRecentReviews(ctx context.Context, limit int) ([]Review, error)
	
	// Review validation and enrichment
	ValidateReviewData(ctx context.Context, review *Review) error
	EnrichReviewData(ctx context.Context, review *Review) error
	DetectDuplicateReviews(ctx context.Context, review *Review) ([]Review, error)
	
	// Batch operations
	ProcessReviewBatch(ctx context.Context, reviews []Review) error
	ImportReviewsFromFile(ctx context.Context, fileURL string, providerID uuid.UUID) error
	ExportReviewsToFile(ctx context.Context, filters map[string]interface{}, format string) (string, error)
}

// NotificationService defines the interface for sending notifications
type NotificationService interface {
	SendProcessingComplete(ctx context.Context, processingID uuid.UUID, status string, recordsProcessed int) error
	SendProcessingFailed(ctx context.Context, processingID uuid.UUID, errorMsg string) error
	SendSystemAlert(ctx context.Context, message string, severity string) error
	SendEmailNotification(ctx context.Context, to []string, subject, body string) error
	SendSlackNotification(ctx context.Context, channel, message string) error
}

// CacheService defines the interface for caching operations
type CacheService interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	FlushAll(ctx context.Context) error
	
	// Specific cache operations
	GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*ReviewSummary, error)
	SetReviewSummary(ctx context.Context, hotelID uuid.UUID, summary *ReviewSummary, expiration time.Duration) error
	InvalidateReviewSummary(ctx context.Context, hotelID uuid.UUID) error
}

// MetricsService defines the interface for collecting metrics
type MetricsService interface {
	IncrementCounter(ctx context.Context, name string, labels map[string]string) error
	RecordHistogram(ctx context.Context, name string, value float64, labels map[string]string) error
	RecordGauge(ctx context.Context, name string, value float64, labels map[string]string) error
	
	// Specific metrics
	RecordProcessingTime(ctx context.Context, processingID uuid.UUID, duration time.Duration) error
	RecordProcessingCount(ctx context.Context, providerID uuid.UUID, count int) error
	RecordErrorCount(ctx context.Context, errorType string, count int) error
	RecordAPIRequestCount(ctx context.Context, endpoint string, method string, statusCode int) error
}

// EventPublisher defines the interface for publishing domain events
type EventPublisher interface {
	PublishReviewCreated(ctx context.Context, review *Review) error
	PublishReviewUpdated(ctx context.Context, review *Review) error
	PublishReviewDeleted(ctx context.Context, reviewID uuid.UUID) error
	PublishProcessingStarted(ctx context.Context, processingID uuid.UUID, providerID uuid.UUID) error
	PublishProcessingCompleted(ctx context.Context, processingID uuid.UUID, recordsProcessed int) error
	PublishProcessingFailed(ctx context.Context, processingID uuid.UUID, errorMsg string) error
	PublishHotelCreated(ctx context.Context, hotel *Hotel) error
	PublishHotelUpdated(ctx context.Context, hotel *Hotel) error
}