package domain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReviewServiceImpl implements the ReviewService interface
type ReviewServiceImpl struct {
	reviewRepo          ReviewRepository
	s3Client            S3Client
	jsonProcessor       JSONProcessor
	notificationService NotificationService
	cacheService        CacheService
	metricsService      MetricsService
	eventPublisher      EventPublisher
	logger              *slog.Logger
}

// NewReviewService creates a new ReviewService instance
func NewReviewService(
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

// Review operations
func (s *ReviewServiceImpl) CreateReview(ctx context.Context, review *Review) error {
	if err := s.ValidateReviewData(ctx, review); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.EnrichReviewData(ctx, review); err != nil {
		s.logger.Warn("Failed to enrich review data", "error", err, "review_id", review.ID)
	}

	if err := s.reviewRepo.CreateBatch(ctx, []Review{*review}); err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}

	if err := s.eventPublisher.PublishReviewCreated(ctx, review); err != nil {
		s.logger.Error("Failed to publish review created event", "error", err, "review_id", review.ID)
	}

	if err := s.cacheService.InvalidateReviewSummary(ctx, review.HotelID); err != nil {
		s.logger.Warn("Failed to invalidate review summary cache", "error", err, "hotel_id", review.HotelID)
	}

	s.metricsService.IncrementCounter(ctx, "reviews_created", map[string]string{
		"provider_id": review.ProviderID.String(),
		"hotel_id":    review.HotelID.String(),
	})

	return nil
}

func (s *ReviewServiceImpl) GetReviewByID(ctx context.Context, id uuid.UUID) (*Review, error) {
	review, err := s.reviewRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get review: %w", err)
	}
	return review, nil
}

func (s *ReviewServiceImpl) GetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]Review, error) {
	reviews, err := s.reviewRepo.GetByHotel(ctx, hotelID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews by hotel: %w", err)
	}
	return reviews, nil
}

func (s *ReviewServiceImpl) GetReviewsByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]Review, error) {
	reviews, err := s.reviewRepo.GetByProvider(ctx, providerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews by provider: %w", err)
	}
	return reviews, nil
}

func (s *ReviewServiceImpl) UpdateReview(ctx context.Context, review *Review) error {
	if err := s.ValidateReviewData(ctx, review); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	existingReview, err := s.reviewRepo.GetByID(ctx, review.ID)
	if err != nil {
		return fmt.Errorf("failed to get existing review: %w", err)
	}

	if err := s.reviewRepo.CreateBatch(ctx, []Review{*review}); err != nil {
		return fmt.Errorf("failed to update review: %w", err)
	}

	if err := s.eventPublisher.PublishReviewUpdated(ctx, review); err != nil {
		s.logger.Error("Failed to publish review updated event", "error", err, "review_id", review.ID)
	}

	if existingReview.HotelID != review.HotelID {
		s.cacheService.InvalidateReviewSummary(ctx, existingReview.HotelID)
	}
	s.cacheService.InvalidateReviewSummary(ctx, review.HotelID)

	return nil
}

func (s *ReviewServiceImpl) DeleteReview(ctx context.Context, id uuid.UUID) error {
	review, err := s.reviewRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get review: %w", err)
	}

	if err := s.reviewRepo.DeleteByID(ctx, id); err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}

	if err := s.eventPublisher.PublishReviewDeleted(ctx, id); err != nil {
		s.logger.Error("Failed to publish review deleted event", "error", err, "review_id", id)
	}

	if err := s.cacheService.InvalidateReviewSummary(ctx, review.HotelID); err != nil {
		s.logger.Warn("Failed to invalidate review summary cache", "error", err, "hotel_id", review.HotelID)
	}

	return nil
}

func (s *ReviewServiceImpl) SearchReviews(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]Review, error) {
	reviews, err := s.reviewRepo.Search(ctx, query, filters, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search reviews: %w", err)
	}
	return reviews, nil
}

// Hotel operations
func (s *ReviewServiceImpl) CreateHotel(ctx context.Context, hotel *Hotel) error {
	if err := s.validateHotel(ctx, hotel); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.reviewRepo.CreateHotel(ctx, hotel); err != nil {
		return fmt.Errorf("failed to create hotel: %w", err)
	}

	if err := s.eventPublisher.PublishHotelCreated(ctx, hotel); err != nil {
		s.logger.Error("Failed to publish hotel created event", "error", err, "hotel_id", hotel.ID)
	}

	return nil
}

func (s *ReviewServiceImpl) GetHotelByID(ctx context.Context, id uuid.UUID) (*Hotel, error) {
	hotel, err := s.reviewRepo.GetHotelByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get hotel: %w", err)
	}
	return hotel, nil
}

func (s *ReviewServiceImpl) UpdateHotel(ctx context.Context, hotel *Hotel) error {
	if err := s.validateHotel(ctx, hotel); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.reviewRepo.UpdateHotel(ctx, hotel); err != nil {
		return fmt.Errorf("failed to update hotel: %w", err)
	}

	if err := s.eventPublisher.PublishHotelUpdated(ctx, hotel); err != nil {
		s.logger.Error("Failed to publish hotel updated event", "error", err, "hotel_id", hotel.ID)
	}

	return nil
}

func (s *ReviewServiceImpl) DeleteHotel(ctx context.Context, id uuid.UUID) error {
	if err := s.reviewRepo.DeleteHotel(ctx, id); err != nil {
		return fmt.Errorf("failed to delete hotel: %w", err)
	}

	if err := s.cacheService.InvalidateReviewSummary(ctx, id); err != nil {
		s.logger.Warn("Failed to invalidate review summary cache", "error", err, "hotel_id", id)
	}

	return nil
}

func (s *ReviewServiceImpl) ListHotels(ctx context.Context, limit, offset int) ([]Hotel, error) {
	hotels, err := s.reviewRepo.ListHotels(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list hotels: %w", err)
	}
	return hotels, nil
}

// Provider operations
func (s *ReviewServiceImpl) CreateProvider(ctx context.Context, provider *Provider) error {
	if err := s.validateProvider(ctx, provider); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.reviewRepo.CreateProvider(ctx, provider); err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	return nil
}

func (s *ReviewServiceImpl) GetProviderByID(ctx context.Context, id uuid.UUID) (*Provider, error) {
	provider, err := s.reviewRepo.GetProviderByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}
	return provider, nil
}

func (s *ReviewServiceImpl) GetProviderByName(ctx context.Context, name string) (*Provider, error) {
	provider, err := s.reviewRepo.GetProviderByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider by name: %w", err)
	}
	return provider, nil
}

func (s *ReviewServiceImpl) UpdateProvider(ctx context.Context, provider *Provider) error {
	if err := s.validateProvider(ctx, provider); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.reviewRepo.UpdateProvider(ctx, provider); err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}

	return nil
}

func (s *ReviewServiceImpl) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	if err := s.reviewRepo.DeleteProvider(ctx, id); err != nil {
		return fmt.Errorf("failed to delete provider: %w", err)
	}
	return nil
}

func (s *ReviewServiceImpl) ListProviders(ctx context.Context, limit, offset int) ([]Provider, error) {
	providers, err := s.reviewRepo.ListProviders(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	return providers, nil
}

// File processing operations
func (s *ReviewServiceImpl) ProcessReviewFile(ctx context.Context, fileURL string, providerID uuid.UUID) (*ReviewProcessingStatus, error) {
	processingID := uuid.New()
	
	processingStatus := &ReviewProcessingStatus{
		ID:               processingID,
		ProviderID:       providerID,
		Status:           "pending",
		FileURL:          fileURL,
		RecordsProcessed: 0,
		RecordsTotal:     0,
	}

	if err := s.reviewRepo.CreateProcessingStatus(ctx, processingStatus); err != nil {
		return nil, fmt.Errorf("failed to create processing status: %w", err)
	}

	go s.processFileAsync(context.Background(), processingID, fileURL, providerID)

	return processingStatus, nil
}

func (s *ReviewServiceImpl) processFileAsync(ctx context.Context, processingID uuid.UUID, fileURL string, providerID uuid.UUID) {
	startTime := time.Now()
	
	if err := s.reviewRepo.UpdateProcessingStatus(ctx, processingID, "processing", 0, ""); err != nil {
		s.logger.Error("Failed to update processing status", "error", err, "processing_id", processingID)
		return
	}

	if err := s.eventPublisher.PublishProcessingStarted(ctx, processingID, providerID); err != nil {
		s.logger.Error("Failed to publish processing started event", "error", err, "processing_id", processingID)
	}

	bucket, key, err := s.parseS3URL(fileURL)
	if err != nil {
		s.handleProcessingError(ctx, processingID, fmt.Sprintf("Invalid S3 URL: %v", err))
		return
	}

	reader, err := s.s3Client.DownloadFile(ctx, bucket, key)
	if err != nil {
		s.handleProcessingError(ctx, processingID, fmt.Sprintf("Failed to download file: %v", err))
		return
	}
	defer reader.Close()

	if err := s.jsonProcessor.ProcessFile(ctx, reader, providerID, processingID); err != nil {
		s.handleProcessingError(ctx, processingID, fmt.Sprintf("Failed to process file: %v", err))
		return
	}

	if err := s.reviewRepo.UpdateProcessingStatus(ctx, processingID, "completed", 0, ""); err != nil {
		s.logger.Error("Failed to update processing status to completed", "error", err, "processing_id", processingID)
		return
	}

	if err := s.eventPublisher.PublishProcessingCompleted(ctx, processingID, 0); err != nil {
		s.logger.Error("Failed to publish processing completed event", "error", err, "processing_id", processingID)
	}

	if err := s.notificationService.SendProcessingComplete(ctx, processingID, "completed", 0); err != nil {
		s.logger.Error("Failed to send processing complete notification", "error", err, "processing_id", processingID)
	}

	s.metricsService.RecordProcessingTime(ctx, processingID, time.Since(startTime))
}

func (s *ReviewServiceImpl) handleProcessingError(ctx context.Context, processingID uuid.UUID, errorMsg string) {
	s.logger.Error("Processing failed", "processing_id", processingID, "error", errorMsg)
	
	if err := s.reviewRepo.UpdateProcessingStatus(ctx, processingID, "failed", 0, errorMsg); err != nil {
		s.logger.Error("Failed to update processing status to failed", "error", err, "processing_id", processingID)
	}

	if err := s.eventPublisher.PublishProcessingFailed(ctx, processingID, errorMsg); err != nil {
		s.logger.Error("Failed to publish processing failed event", "error", err, "processing_id", processingID)
	}

	if err := s.notificationService.SendProcessingFailed(ctx, processingID, errorMsg); err != nil {
		s.logger.Error("Failed to send processing failed notification", "error", err, "processing_id", processingID)
	}

	s.metricsService.IncrementCounter(ctx, "processing_errors", map[string]string{
		"processing_id": processingID.String(),
	})
}

func (s *ReviewServiceImpl) GetProcessingStatus(ctx context.Context, id uuid.UUID) (*ReviewProcessingStatus, error) {
	status, err := s.reviewRepo.GetProcessingStatusByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get processing status: %w", err)
	}
	return status, nil
}

func (s *ReviewServiceImpl) GetProcessingHistory(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]ReviewProcessingStatus, error) {
	history, err := s.reviewRepo.GetProcessingStatusByProvider(ctx, providerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get processing history: %w", err)
	}
	return history, nil
}

func (s *ReviewServiceImpl) CancelProcessing(ctx context.Context, id uuid.UUID) error {
	if err := s.reviewRepo.UpdateProcessingStatus(ctx, id, "cancelled", 0, "Processing cancelled by user"); err != nil {
		return fmt.Errorf("failed to cancel processing: %w", err)
	}
	return nil
}

// Analytics operations
func (s *ReviewServiceImpl) GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*ReviewSummary, error) {
	if summary, err := s.cacheService.GetReviewSummary(ctx, hotelID); err == nil {
		return summary, nil
	}

	summary, err := s.reviewRepo.GetReviewSummaryByHotelID(ctx, hotelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get review summary: %w", err)
	}

	if err := s.cacheService.SetReviewSummary(ctx, hotelID, summary, 1*time.Hour); err != nil {
		s.logger.Warn("Failed to cache review summary", "error", err, "hotel_id", hotelID)
	}

	return summary, nil
}

func (s *ReviewServiceImpl) GetReviewStatsByProvider(ctx context.Context, providerID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error) {
	reviews, err := s.reviewRepo.GetByDateRange(ctx, startDate, endDate, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews by date range: %w", err)
	}

	stats := make(map[string]interface{})
	var totalReviews int
	var totalRating float64

	for _, review := range reviews {
		if review.ProviderID == providerID {
			totalReviews++
			totalRating += review.Rating
		}
	}

	if totalReviews > 0 {
		stats["total_reviews"] = totalReviews
		stats["average_rating"] = totalRating / float64(totalReviews)
	}

	return stats, nil
}

func (s *ReviewServiceImpl) GetReviewStatsByHotel(ctx context.Context, hotelID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error) {
	reviews, err := s.reviewRepo.GetByHotel(ctx, hotelID, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews by hotel: %w", err)
	}

	stats := make(map[string]interface{})
	var totalReviews int
	var totalRating float64

	for _, review := range reviews {
		if review.ReviewDate.After(startDate) && review.ReviewDate.Before(endDate) {
			totalReviews++
			totalRating += review.Rating
		}
	}

	if totalReviews > 0 {
		stats["total_reviews"] = totalReviews
		stats["average_rating"] = totalRating / float64(totalReviews)
	}

	return stats, nil
}

func (s *ReviewServiceImpl) GetTopRatedHotels(ctx context.Context, limit int) ([]Hotel, error) {
	hotels, err := s.reviewRepo.ListHotels(ctx, limit*2, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get hotels: %w", err)
	}

	if len(hotels) < limit {
		return hotels, nil
	}
	return hotels[:limit], nil
}

func (s *ReviewServiceImpl) GetRecentReviews(ctx context.Context, limit int) ([]Review, error) {
	reviews, err := s.reviewRepo.GetByDateRange(ctx, time.Now().AddDate(0, 0, -30), time.Now(), limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent reviews: %w", err)
	}
	return reviews, nil
}

// Review validation and enrichment
func (s *ReviewServiceImpl) ValidateReviewData(ctx context.Context, review *Review) error {
	if review.Rating < 1.0 || review.Rating > 5.0 {
		return fmt.Errorf("rating must be between 1.0 and 5.0")
	}

	if review.Comment == "" {
		return fmt.Errorf("comment cannot be empty")
	}

	if review.ReviewDate.IsZero() {
		return fmt.Errorf("review date cannot be empty")
	}

	if review.ReviewDate.After(time.Now()) {
		return fmt.Errorf("review date cannot be in the future")
	}

	if review.ProviderID == uuid.Nil {
		return fmt.Errorf("provider ID cannot be empty")
	}

	if review.HotelID == uuid.Nil {
		return fmt.Errorf("hotel ID cannot be empty")
	}

	return nil
}

func (s *ReviewServiceImpl) EnrichReviewData(ctx context.Context, review *Review) error {
	if review.Language == "" {
		review.Language = "en"
	}

	if review.Sentiment == "" {
		review.Sentiment = s.detectSentiment(review.Comment)
	}

	if review.ProcessingHash == "" {
		review.ProcessingHash = s.generateProcessingHash(review)
	}

	now := time.Now()
	review.ProcessedAt = &now

	return nil
}

func (s *ReviewServiceImpl) DetectDuplicateReviews(ctx context.Context, review *Review) ([]Review, error) {
	filters := map[string]interface{}{
		"hotel_id":    review.HotelID,
		"provider_id": review.ProviderID,
		"rating":      review.Rating,
	}

	duplicates, err := s.reviewRepo.Search(ctx, review.Comment, filters, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to search for duplicates: %w", err)
	}

	return duplicates, nil
}

// Batch operations
func (s *ReviewServiceImpl) ProcessReviewBatch(ctx context.Context, reviews []Review) error {
	for i := range reviews {
		if err := s.ValidateReviewData(ctx, &reviews[i]); err != nil {
			return fmt.Errorf("validation failed for review %d: %w", i, err)
		}

		if err := s.EnrichReviewData(ctx, &reviews[i]); err != nil {
			s.logger.Warn("Failed to enrich review data", "error", err, "review_index", i)
		}
	}

	if err := s.reviewRepo.CreateBatch(ctx, reviews); err != nil {
		return fmt.Errorf("failed to create review batch: %w", err)
	}

	for _, review := range reviews {
		if err := s.eventPublisher.PublishReviewCreated(ctx, &review); err != nil {
			s.logger.Error("Failed to publish review created event", "error", err, "review_id", review.ID)
		}
	}

	return nil
}

func (s *ReviewServiceImpl) ImportReviewsFromFile(ctx context.Context, fileURL string, providerID uuid.UUID) error {
	_, err := s.ProcessReviewFile(ctx, fileURL, providerID)
	return err
}

func (s *ReviewServiceImpl) ExportReviewsToFile(ctx context.Context, filters map[string]interface{}, format string) (string, error) {
	return "", fmt.Errorf("export functionality not implemented yet")
}

// Helper methods
func (s *ReviewServiceImpl) validateHotel(ctx context.Context, hotel *Hotel) error {
	if hotel.Name == "" {
		return fmt.Errorf("hotel name cannot be empty")
	}

	if hotel.StarRating < 1 || hotel.StarRating > 5 {
		return fmt.Errorf("star rating must be between 1 and 5")
	}

	return nil
}

func (s *ReviewServiceImpl) validateProvider(ctx context.Context, provider *Provider) error {
	if provider.Name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	return nil
}

func (s *ReviewServiceImpl) parseS3URL(url string) (bucket, key string, err error) {
	parts := strings.Split(strings.TrimPrefix(url, "s3://"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid S3 URL format")
	}
	return parts[0], strings.Join(parts[1:], "/"), nil
}

func (s *ReviewServiceImpl) detectSentiment(comment string) string {
	comment = strings.ToLower(comment)
	
	positiveWords := []string{"good", "great", "excellent", "amazing", "wonderful", "perfect", "love", "fantastic"}
	negativeWords := []string{"bad", "terrible", "awful", "horrible", "worst", "hate", "disappointing", "poor"}
	
	positiveCount := 0
	negativeCount := 0
	
	for _, word := range positiveWords {
		if strings.Contains(comment, word) {
			positiveCount++
		}
	}
	
	for _, word := range negativeWords {
		if strings.Contains(comment, word) {
			negativeCount++
		}
	}
	
	if positiveCount > negativeCount {
		return "positive"
	} else if negativeCount > positiveCount {
		return "negative"
	}
	
	return "neutral"
}

func (s *ReviewServiceImpl) generateProcessingHash(review *Review) string {
	return fmt.Sprintf("%x", review.ID)[0:32] // Take first 32 characters
}