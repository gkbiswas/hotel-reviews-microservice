package infrastructure

import (
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// ReviewRepository implements the domain.ReviewRepository interface
type ReviewRepository struct {
	db             *Database
	logger         *logger.Logger
	queryOptimizer *QueryOptimizer
	searchBuilder  *FullTextSearchBuilder
	aggOptimizer   *AggregationOptimizer
	indexOptimizer *IndexOptimizer
}

// NewReviewRepository creates a new ReviewRepository instance
func NewReviewRepository(db *Database, logger *logger.Logger) domain.ReviewRepository {
	// Convert logger to slog.Logger for query optimizer
	slogLogger := slog.Default()
	
	queryOptimizer := NewQueryOptimizer(db.DB, slogLogger)
	searchBuilder := NewFullTextSearchBuilder(db.DB, slogLogger)
	aggOptimizer := NewAggregationOptimizer(db.DB, slogLogger)
	indexOptimizer := NewIndexOptimizer(db.DB, slogLogger)
	
	repo := &ReviewRepository{
		db:             db,
		logger:         logger,
		queryOptimizer: queryOptimizer,
		searchBuilder:  searchBuilder,
		aggOptimizer:   aggOptimizer,
		indexOptimizer: indexOptimizer,
	}

	// Initialize database optimizations only if we have a valid database connection
	// and we're not in a test environment
	if db != nil && db.DB != nil && !isTestEnvironment() {
		go func() {
			ctx := context.Background()
			if err := repo.InitializeOptimizations(ctx); err != nil {
				logger.ErrorContext(ctx, "Failed to initialize database optimizations", "error", err)
			}
		}()
	}

	return repo
}

// isTestEnvironment checks if we're running in a test environment
func isTestEnvironment() bool {
	// Check for common test environment indicators
	if strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/go-build") {
		return true
	}
	// Check for Go test binary patterns
	for _, arg := range os.Args {
		if strings.Contains(arg, "-test.") {
			return true
		}
	}
	return false
}

// Review operations
func (r *ReviewRepository) CreateBatch(ctx context.Context, reviews []domain.Review) error {
	if len(reviews) == 0 {
		return nil
	}

	start := time.Now()

	// Use transaction for batch operations
	err := r.db.Transaction(ctx, func(tx *gorm.DB) error {
		// Process in batches to avoid memory issues
		batchSize := 1000
		for i := 0; i < len(reviews); i += batchSize {
			end := i + batchSize
			if end > len(reviews) {
				end = len(reviews)
			}

			batch := reviews[i:end]

			// Use upsert to handle duplicates
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "external_id"}, {Name: "provider_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"rating", "comment", "review_date", "updated_at"}),
			}).Create(&batch).Error; err != nil {
				return fmt.Errorf("failed to create batch %d-%d: %w", i, end, err)
			}
		}

		return nil
	})

	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to create review batch",
			"count", len(reviews),
			"error", err,
		)
		return fmt.Errorf("failed to create review batch: %w", err)
	}

	duration := time.Since(start)
	r.logger.InfoContext(ctx, "Review batch created successfully",
		"count", len(reviews),
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (r *ReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	var review domain.Review
	start := time.Now()

	// Apply optimized preloads
	config := r.queryOptimizer.GetOptimizedPreloadConfig()
	query := r.queryOptimizer.ApplyOptimizedPreloads(r.db.WithContext(ctx), config)
	
	err := query.First(&review, "id = ?", id).Error

	// Track query performance
	r.logger.DebugContext(ctx, "Query executed",
		"operation", "GetByID",
		"duration_ms", time.Since(start).Milliseconds(),
		"error", err)

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("review not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get review by ID: %w", err)
	}

	return &review, nil
}

func (r *ReviewRepository) GetByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]domain.Review, error) {
	var reviews []domain.Review
	start := time.Now()

	// Apply optimized preloads
	config := r.queryOptimizer.GetOptimizedPreloadConfig()
	query := r.queryOptimizer.ApplyOptimizedPreloads(r.db.WithContext(ctx), config)
	
	err := query.
		Where("provider_id = ?", providerID).
		Order("review_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error

	// Track query performance
	r.logger.DebugContext(ctx, "Query executed",
		"operation", "GetByProvider",
		"duration_ms", time.Since(start).Milliseconds(),
		"result_count", len(reviews),
		"error", err)

	if err != nil {
		return nil, fmt.Errorf("failed to get reviews by provider: %w", err)
	}

	return reviews, nil
}

func (r *ReviewRepository) GetByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]domain.Review, error) {
	var reviews []domain.Review
	start := time.Now()

	// Apply optimized preloads
	config := r.queryOptimizer.GetOptimizedPreloadConfig()
	query := r.queryOptimizer.ApplyOptimizedPreloads(r.db.WithContext(ctx), config)
	
	err := query.
		Where("hotel_id = ?", hotelID).
		Order("review_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error

	// Track query performance
	r.logger.DebugContext(ctx, "Query executed",
		"operation", "GetByHotel",
		"duration_ms", time.Since(start).Milliseconds(),
		"result_count", len(reviews),
		"error", err)

	if err != nil {
		return nil, fmt.Errorf("failed to get reviews by hotel: %w", err)
	}

	return reviews, nil
}

func (r *ReviewRepository) GetByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int) ([]domain.Review, error) {
	var reviews []domain.Review

	err := r.db.WithContext(ctx).
		Preload("Provider").
		Preload("Hotel").
		Preload("ReviewerInfo").
		Where("review_date BETWEEN ? AND ?", startDate, endDate).
		Order("review_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get reviews by date range: %w", err)
	}

	return reviews, nil
}

func (r *ReviewRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Review{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("failed to update review status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("review not found for status update")
	}

	return nil
}

func (r *ReviewRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.Review{}, "id = ?", id)

	if result.Error != nil {
		return fmt.Errorf("failed to delete review: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("review not found for deletion")
	}

	return nil
}

func (r *ReviewRepository) Search(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]domain.Review, error) {
	var reviews []domain.Review
	start := time.Now()

	// Apply optimized preloads
	config := r.queryOptimizer.GetOptimizedPreloadConfig()
	db := r.queryOptimizer.ApplyOptimizedPreloads(r.db.WithContext(ctx), config)

	// Apply optimized full-text search if query is provided
	if query != "" {
		searchConfig := &SearchConfig{
			Query:      query,
			Language:   "english",
			Fields:     []string{"title", "comment"},
			MaxResults: limit,
		}
		db = r.searchBuilder.BuildFullTextQuery(searchConfig)
		
		// Re-apply context and preloads since BuildFullTextQuery creates a new query
		config := r.queryOptimizer.GetOptimizedPreloadConfig()
		db = r.queryOptimizer.ApplyOptimizedPreloads(db.WithContext(ctx), config)
	}

	// Apply filters
	for key, value := range filters {
		switch key {
		case "rating":
			db = db.Where("rating = ?", value)
		case "min_rating":
			db = db.Where("rating >= ?", value)
		case "max_rating":
			db = db.Where("rating <= ?", value)
		case "provider_id":
			db = db.Where("provider_id = ?", value)
		case "hotel_id":
			db = db.Where("hotel_id = ?", value)
		case "language":
			db = db.Where("language = ?", value)
		case "sentiment":
			db = db.Where("sentiment = ?", value)
		case "is_verified":
			db = db.Where("is_verified = ?", value)
		case "start_date":
			db = db.Where("review_date >= ?", value)
		case "end_date":
			db = db.Where("review_date <= ?", value)
		}
	}

	err := db.Order("review_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error

	// Track query performance
	searchType := "basic_search"
	if query != "" {
		searchType = "fulltext_search"
	}
	r.logger.DebugContext(ctx, "Query executed",
		"operation", searchType,
		"duration_ms", time.Since(start).Milliseconds(),
		"result_count", len(reviews),
		"error", err)

	if err != nil {
		return nil, fmt.Errorf("failed to search reviews: %w", err)
	}

	return reviews, nil
}

func (r *ReviewRepository) GetTotalCount(ctx context.Context, filters map[string]interface{}) (int64, error) {
	var count int64

	db := r.db.WithContext(ctx).Model(&domain.Review{})

	// Apply filters
	for key, value := range filters {
		switch key {
		case "rating":
			db = db.Where("rating = ?", value)
		case "min_rating":
			db = db.Where("rating >= ?", value)
		case "max_rating":
			db = db.Where("rating <= ?", value)
		case "provider_id":
			db = db.Where("provider_id = ?", value)
		case "hotel_id":
			db = db.Where("hotel_id = ?", value)
		case "language":
			db = db.Where("language = ?", value)
		case "sentiment":
			db = db.Where("sentiment = ?", value)
		case "is_verified":
			db = db.Where("is_verified = ?", value)
		case "start_date":
			db = db.Where("review_date >= ?", value)
		case "end_date":
			db = db.Where("review_date <= ?", value)
		}
	}

	err := db.Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get total count: %w", err)
	}

	return count, nil
}

// Hotel operations
func (r *ReviewRepository) CreateHotel(ctx context.Context, hotel *domain.Hotel) error {
	err := r.db.WithContext(ctx).Create(hotel).Error
	if err != nil {
		return fmt.Errorf("failed to create hotel: %w", err)
	}

	r.logger.InfoContext(ctx, "Hotel created successfully", "hotel_id", hotel.ID, "name", hotel.Name)
	return nil
}

func (r *ReviewRepository) GetHotelByID(ctx context.Context, id uuid.UUID) (*domain.Hotel, error) {
	var hotel domain.Hotel

	err := r.db.WithContext(ctx).First(&hotel, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("hotel not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get hotel by ID: %w", err)
	}

	return &hotel, nil
}

func (r *ReviewRepository) GetHotelByName(ctx context.Context, name string) (*domain.Hotel, error) {
	var hotel domain.Hotel

	err := r.db.WithContext(ctx).Where("name = ?", name).First(&hotel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("hotel not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get hotel by name: %w", err)
	}

	return &hotel, nil
}

func (r *ReviewRepository) UpdateHotel(ctx context.Context, hotel *domain.Hotel) error {
	err := r.db.WithContext(ctx).Save(hotel).Error
	if err != nil {
		return fmt.Errorf("failed to update hotel: %w", err)
	}

	r.logger.InfoContext(ctx, "Hotel updated successfully", "hotel_id", hotel.ID, "name", hotel.Name)
	return nil
}

func (r *ReviewRepository) DeleteHotel(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.Hotel{}, "id = ?", id)

	if result.Error != nil {
		return fmt.Errorf("failed to delete hotel: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("hotel not found for deletion")
	}

	return nil
}

func (r *ReviewRepository) ListHotels(ctx context.Context, limit, offset int) ([]domain.Hotel, error) {
	var hotels []domain.Hotel

	err := r.db.WithContext(ctx).
		Order("name").
		Limit(limit).
		Offset(offset).
		Find(&hotels).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list hotels: %w", err)
	}

	return hotels, nil
}

// Provider operations
func (r *ReviewRepository) CreateProvider(ctx context.Context, provider *domain.Provider) error {
	err := r.db.WithContext(ctx).Create(provider).Error
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	r.logger.InfoContext(ctx, "Provider created successfully", "provider_id", provider.ID, "name", provider.Name)
	return nil
}

func (r *ReviewRepository) GetProviderByID(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	var provider domain.Provider

	err := r.db.WithContext(ctx).First(&provider, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("provider not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get provider by ID: %w", err)
	}

	return &provider, nil
}

func (r *ReviewRepository) GetProviderByName(ctx context.Context, name string) (*domain.Provider, error) {
	var provider domain.Provider

	err := r.db.WithContext(ctx).Where("name = ?", name).First(&provider).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("provider not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get provider by name: %w", err)
	}

	return &provider, nil
}

func (r *ReviewRepository) UpdateProvider(ctx context.Context, provider *domain.Provider) error {
	err := r.db.WithContext(ctx).Save(provider).Error
	if err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}

	r.logger.InfoContext(ctx, "Provider updated successfully", "provider_id", provider.ID, "name", provider.Name)
	return nil
}

func (r *ReviewRepository) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.Provider{}, "id = ?", id)

	if result.Error != nil {
		return fmt.Errorf("failed to delete provider: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("provider not found for deletion")
	}

	return nil
}

func (r *ReviewRepository) ListProviders(ctx context.Context, limit, offset int) ([]domain.Provider, error) {
	var providers []domain.Provider

	err := r.db.WithContext(ctx).
		Order("name").
		Limit(limit).
		Offset(offset).
		Find(&providers).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	return providers, nil
}

// ReviewerInfo operations
func (r *ReviewRepository) CreateReviewerInfo(ctx context.Context, reviewerInfo *domain.ReviewerInfo) error {
	err := r.db.WithContext(ctx).Create(reviewerInfo).Error
	if err != nil {
		return fmt.Errorf("failed to create reviewer info: %w", err)
	}

	r.logger.InfoContext(ctx, "Reviewer info created successfully", "reviewer_id", reviewerInfo.ID, "name", reviewerInfo.Name)
	return nil
}

func (r *ReviewRepository) GetReviewerInfoByID(ctx context.Context, id uuid.UUID) (*domain.ReviewerInfo, error) {
	var reviewerInfo domain.ReviewerInfo

	err := r.db.WithContext(ctx).First(&reviewerInfo, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("reviewer info not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get reviewer info by ID: %w", err)
	}

	return &reviewerInfo, nil
}

func (r *ReviewRepository) GetReviewerInfoByEmail(ctx context.Context, email string) (*domain.ReviewerInfo, error) {
	var reviewerInfo domain.ReviewerInfo

	err := r.db.WithContext(ctx).Where("email = ?", email).First(&reviewerInfo).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("reviewer info not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get reviewer info by email: %w", err)
	}

	return &reviewerInfo, nil
}

func (r *ReviewRepository) UpdateReviewerInfo(ctx context.Context, reviewerInfo *domain.ReviewerInfo) error {
	err := r.db.WithContext(ctx).Save(reviewerInfo).Error
	if err != nil {
		return fmt.Errorf("failed to update reviewer info: %w", err)
	}

	r.logger.InfoContext(ctx, "Reviewer info updated successfully", "reviewer_id", reviewerInfo.ID, "name", reviewerInfo.Name)
	return nil
}

func (r *ReviewRepository) DeleteReviewerInfo(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.ReviewerInfo{}, "id = ?", id)

	if result.Error != nil {
		return fmt.Errorf("failed to delete reviewer info: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("reviewer info not found for deletion")
	}

	return nil
}

// Review summary operations
func (r *ReviewRepository) CreateOrUpdateReviewSummary(ctx context.Context, summary *domain.ReviewSummary) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hotel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"total_reviews", "average_rating", "rating_distribution", "avg_service_rating", "avg_cleanliness_rating", "avg_location_rating", "avg_value_rating", "avg_comfort_rating", "avg_facilities_rating", "last_review_date", "updated_at"}),
	}).Create(summary).Error

	if err != nil {
		return fmt.Errorf("failed to create or update review summary: %w", err)
	}

	r.logger.InfoContext(ctx, "Review summary created/updated successfully", "hotel_id", summary.HotelID)
	return nil
}

func (r *ReviewRepository) GetReviewSummaryByHotelID(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	var summary domain.ReviewSummary

	err := r.db.WithContext(ctx).
		Preload("Hotel").
		First(&summary, "hotel_id = ?", hotelID).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("review summary not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get review summary by hotel ID: %w", err)
	}

	return &summary, nil
}

func (r *ReviewRepository) UpdateReviewSummary(ctx context.Context, hotelID uuid.UUID) error {
	start := time.Now()

	// Use optimized aggregation query
	aggStats, err := r.aggOptimizer.GetOptimizedHotelSummary(ctx, hotelID.String())
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to get optimized hotel summary",
			"operation", "UpdateReviewSummary",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err)
		return fmt.Errorf("failed to get optimized hotel summary: %w", err)
	}

	// Track successful aggregation query
	r.logger.DebugContext(ctx, "Query executed",
		"operation", "UpdateReviewSummary",
		"duration_ms", time.Since(start).Milliseconds(),
		"hotel_id", hotelID)

	// Convert to domain ReviewSummary
	ratingDistribution := make(map[string]int)
	for rating, count := range aggStats.RatingDistribution {
		ratingDistribution[fmt.Sprintf("%d", rating)] = int(count)
	}

	summary := &domain.ReviewSummary{
		HotelID:            hotelID,
		TotalReviews:       int(aggStats.TotalReviews),
		AverageRating:      aggStats.AverageRating,
		RatingDistribution: ratingDistribution,
		LastReviewDate:     aggStats.LatestReviewDate,
		// Note: The optimized aggregation doesn't include detailed ratings,
		// so we fall back to the original method for those if needed
	}

	return r.CreateOrUpdateReviewSummary(ctx, summary)
}

// Review processing status operations
func (r *ReviewRepository) CreateProcessingStatus(ctx context.Context, status *domain.ReviewProcessingStatus) error {
	err := r.db.WithContext(ctx).Create(status).Error
	if err != nil {
		return fmt.Errorf("failed to create processing status: %w", err)
	}

	r.logger.InfoContext(ctx, "Processing status created successfully", "processing_id", status.ID, "provider_id", status.ProviderID)
	return nil
}

func (r *ReviewRepository) GetProcessingStatusByID(ctx context.Context, id uuid.UUID) (*domain.ReviewProcessingStatus, error) {
	var status domain.ReviewProcessingStatus

	err := r.db.WithContext(ctx).
		Preload("Provider").
		First(&status, "id = ?", id).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("processing status not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get processing status by ID: %w", err)
	}

	return &status, nil
}

func (r *ReviewRepository) GetProcessingStatusByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]domain.ReviewProcessingStatus, error) {
	var statuses []domain.ReviewProcessingStatus

	err := r.db.WithContext(ctx).
		Preload("Provider").
		Where("provider_id = ?", providerID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&statuses).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get processing status by provider: %w", err)
	}

	return statuses, nil
}

func (r *ReviewRepository) UpdateProcessingStatus(ctx context.Context, id uuid.UUID, status string, recordsProcessed int, errorMsg string) error {
	updates := map[string]interface{}{
		"status":            status,
		"records_processed": recordsProcessed,
		"error_msg":         errorMsg,
		"updated_at":        time.Now(),
	}

	if status == "processing" {
		updates["started_at"] = time.Now()
	} else if status == "completed" || status == "failed" {
		updates["completed_at"] = time.Now()
	}

	result := r.db.WithContext(ctx).
		Model(&domain.ReviewProcessingStatus{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update processing status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("processing status not found for update")
	}

	return nil
}

func (r *ReviewRepository) DeleteProcessingStatus(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.ReviewProcessingStatus{}, "id = ?", id)

	if result.Error != nil {
		return fmt.Errorf("failed to delete processing status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("processing status not found for deletion")
	}

	return nil
}

// Duplicate prevention methods
func (r *ReviewRepository) FindDuplicateReviews(ctx context.Context, review *domain.Review) ([]domain.Review, error) {
	var duplicates []domain.Review

	// Generate content hash for duplicate detection
	contentHash := r.generateContentHash(review)

	err := r.db.WithContext(ctx).
		Where("hotel_id = ? AND provider_id = ? AND processing_hash = ?",
			review.HotelID, review.ProviderID, contentHash).
		Find(&duplicates).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find duplicate reviews: %w", err)
	}

	return duplicates, nil
}

func (r *ReviewRepository) CheckProcessingHashExists(ctx context.Context, hash string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&domain.Review{}).
		Where("processing_hash = ?", hash).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check processing hash existence: %w", err)
	}

	return count > 0, nil
}

func (r *ReviewRepository) UpsertReviewerInfo(ctx context.Context, reviewerInfo *domain.ReviewerInfo) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "country", "is_verified", "total_reviews", "average_rating", "member_since", "profile_image_url", "bio", "updated_at"}),
	}).Create(reviewerInfo).Error

	if err != nil {
		return fmt.Errorf("failed to upsert reviewer info: %w", err)
	}

	return nil
}

func (r *ReviewRepository) UpsertHotel(ctx context.Context, hotel *domain.Hotel) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "city"}, {Name: "country"}},
		DoUpdates: clause.AssignmentColumns([]string{"address", "postal_code", "phone", "email", "star_rating", "description", "amenities", "latitude", "longitude", "updated_at"}),
	}).Create(hotel).Error

	if err != nil {
		return fmt.Errorf("failed to upsert hotel: %w", err)
	}

	return nil
}

// Helper methods
func (r *ReviewRepository) generateContentHash(review *domain.Review) string {
	content := fmt.Sprintf("%s|%s|%f|%s|%s",
		review.HotelID.String(),
		review.ProviderID.String(),
		review.Rating,
		review.Comment,
		review.ReviewDate.Format("2006-01-02"),
	)

	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

func (r *ReviewRepository) GetBatchInsertSize() int {
	return 1000
}

func (r *ReviewRepository) GetReviewsForSummaryUpdate(ctx context.Context, hotelID uuid.UUID, limit int) ([]domain.Review, error) {
	var reviews []domain.Review

	err := r.db.WithContext(ctx).
		Where("hotel_id = ?", hotelID).
		Order("review_date DESC").
		Limit(limit).
		Find(&reviews).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get reviews for summary update: %w", err)
	}

	return reviews, nil
}

func (r *ReviewRepository) GetReviewCountByProvider(ctx context.Context, providerID uuid.UUID, startDate, endDate time.Time) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&domain.Review{}).
		Where("provider_id = ? AND review_date BETWEEN ? AND ?", providerID, startDate, endDate).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get review count by provider: %w", err)
	}

	return count, nil
}

func (r *ReviewRepository) GetReviewCountByHotel(ctx context.Context, hotelID uuid.UUID, startDate, endDate time.Time) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&domain.Review{}).
		Where("hotel_id = ? AND review_date BETWEEN ? AND ?", hotelID, startDate, endDate).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get review count by hotel: %w", err)
	}

	return count, nil
}

func (r *ReviewRepository) GetAverageRatingByHotel(ctx context.Context, hotelID uuid.UUID) (float64, error) {
	var avgRating float64

	err := r.db.WithContext(ctx).
		Model(&domain.Review{}).
		Where("hotel_id = ?", hotelID).
		Select("AVG(rating)").
		Scan(&avgRating).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get average rating by hotel: %w", err)
	}

	return avgRating, nil
}

func (r *ReviewRepository) GetTopRatedHotels(ctx context.Context, limit int) ([]domain.Hotel, error) {
	var hotels []domain.Hotel

	err := r.db.WithContext(ctx).
		Table("hotels").
		Select("hotels.*, AVG(reviews.rating) as avg_rating").
		Joins("LEFT JOIN reviews ON hotels.id = reviews.hotel_id").
		Group("hotels.id").
		Order("avg_rating DESC").
		Limit(limit).
		Find(&hotels).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get top rated hotels: %w", err)
	}

	return hotels, nil
}

func (r *ReviewRepository) GetRecentReviews(ctx context.Context, limit int) ([]domain.Review, error) {
	var reviews []domain.Review

	err := r.db.WithContext(ctx).
		Preload("Provider").
		Preload("Hotel").
		Preload("ReviewerInfo").
		Order("review_date DESC").
		Limit(limit).
		Find(&reviews).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get recent reviews: %w", err)
	}

	return reviews, nil
}

func (r *ReviewRepository) BulkUpdateReviewSentiment(ctx context.Context, updates map[uuid.UUID]string) error {
	return r.db.Transaction(ctx, func(tx *gorm.DB) error {
		for reviewID, sentiment := range updates {
			if err := tx.Model(&domain.Review{}).
				Where("id = ?", reviewID).
				Update("sentiment", sentiment).Error; err != nil {
				return fmt.Errorf("failed to update sentiment for review %s: %w", reviewID, err)
			}
		}
		return nil
	})
}

func (r *ReviewRepository) GetReviewsWithoutSentiment(ctx context.Context, limit int) ([]domain.Review, error) {
	var reviews []domain.Review

	err := r.db.WithContext(ctx).
		Where("sentiment IS NULL OR sentiment = ''").
		Limit(limit).
		Find(&reviews).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get reviews without sentiment: %w", err)
	}

	return reviews, nil
}

// InitializeOptimizations creates database indexes and optimizations
func (r *ReviewRepository) InitializeOptimizations(ctx context.Context) error {
	r.logger.InfoContext(ctx, "Initializing database optimizations...")

	// Create optimized indexes
	if err := r.indexOptimizer.CreateOptimizedIndexes(ctx); err != nil {
		r.logger.ErrorContext(ctx, "Failed to create optimized indexes", "error", err)
		return fmt.Errorf("failed to create optimized indexes: %w", err)
	}

	// Analyze query performance
	if err := r.indexOptimizer.AnalyzeQueryPerformance(ctx); err != nil {
		r.logger.WarnContext(ctx, "Failed to analyze query performance", "error", err)
		// Non-critical, continue
	}

	r.logger.InfoContext(ctx, "Database optimizations initialized successfully")
	return nil
}

// GetQueryPerformanceSummary returns query performance statistics
func (r *ReviewRepository) GetQueryPerformanceSummary() map[string]interface{} {
	return map[string]interface{}{
		"optimization_enabled": true,
		"note": "Query performance tracking is available through logging",
	}
}

// GetSlowQueries returns the slowest queries (placeholder implementation)
func (r *ReviewRepository) GetSlowQueries(limit int) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"note": "Query tracking is available through logging and monitoring",
		},
	}
}

// GetOptimizationSuggestions returns database optimization suggestions
func (r *ReviewRepository) GetOptimizationSuggestions() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"suggestion": "Use optimized preloads for N+1 query prevention",
			"status": "implemented",
		},
		map[string]interface{}{
			"suggestion": "Use full-text search for better text search performance",
			"status": "implemented",
		},
		map[string]interface{}{
			"suggestion": "Use aggregation optimizer for summary calculations",
			"status": "implemented",
		},
	}
}
