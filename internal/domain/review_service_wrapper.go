package domain

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// NewReviewServiceWithAdapter creates a new ReviewService instance with logger adapter
func NewReviewServiceWithAdapter(
	reviewRepo ReviewRepository,
	s3Client S3Client,
	jsonProcessor JSONProcessor,
	cacheService CacheService,
	eventPublisher EventPublisher,
	logger *logger.Logger,
) ReviewService {
	// Create a simple slog adapter
	slogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	
	// Create a stub notification service for now
	notificationService := &stubNotificationService{}
	
	// Create a stub metrics service for now  
	metricsService := &stubMetricsService{}
	
	return NewReviewServiceWithDeps(
		reviewRepo,
		s3Client,
		jsonProcessor,
		notificationService,
		cacheService,
		metricsService,
		eventPublisher,
		slogger,
	)
}

// NewReviewServiceWithDeps creates a new ReviewService with all dependencies
func NewReviewServiceWithDeps(
	reviewRepo ReviewRepository,
	s3Client S3Client,
	jsonProcessor JSONProcessor,
	notificationService NotificationService,
	cacheService CacheService,
	metricsService MetricsService,
	eventPublisher EventPublisher,
	logger *slog.Logger,
) ReviewService {
	return &ReviewServiceImpl{
		reviewRepo:          reviewRepo,
		s3Client:            s3Client,
		jsonProcessor:       jsonProcessor,
		notificationService: notificationService,
		cacheService:        cacheService,
		metricsService:      metricsService,
		eventPublisher:      eventPublisher,
		logger:              logger,
	}
}

// Stub implementations for missing services

type stubNotificationService struct{}

func (s *stubNotificationService) SendProcessingComplete(ctx context.Context, processingID uuid.UUID, status string, recordsProcessed int) error {
	return nil
}

func (s *stubNotificationService) SendProcessingFailed(ctx context.Context, processingID uuid.UUID, errorMsg string) error {
	return nil
}

func (s *stubNotificationService) SendSystemAlert(ctx context.Context, message string, severity string) error {
	return nil
}

func (s *stubNotificationService) SendEmailNotification(ctx context.Context, to []string, subject, body string) error {
	return nil
}

func (s *stubNotificationService) SendSlackNotification(ctx context.Context, channel, message string) error {
	return nil
}

type stubMetricsService struct{}

func (s *stubMetricsService) IncrementCounter(ctx context.Context, name string, labels map[string]string) error {
	return nil
}

func (s *stubMetricsService) RecordHistogram(ctx context.Context, name string, value float64, labels map[string]string) error {
	return nil
}

func (s *stubMetricsService) RecordGauge(ctx context.Context, name string, value float64, labels map[string]string) error {
	return nil
}

func (s *stubMetricsService) RecordProcessingTime(ctx context.Context, processingID uuid.UUID, duration time.Duration) error {
	return nil
}

func (s *stubMetricsService) RecordProcessingCount(ctx context.Context, providerID uuid.UUID, count int) error {
	return nil
}

func (s *stubMetricsService) RecordErrorCount(ctx context.Context, errorType string, count int) error {
	return nil
}

func (s *stubMetricsService) RecordAPIRequestCount(ctx context.Context, endpoint string, method string, statusCode int) error {
	return nil
}