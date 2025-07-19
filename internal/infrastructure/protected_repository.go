package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// ProtectedReviewRepository wraps the review repository with circuit breaker protection
type ProtectedReviewRepository struct {
	dbWrapper *DatabaseWrapper
	logger    *logger.Logger
}

// NewProtectedReviewRepository creates a new protected review repository
func NewProtectedReviewRepository(dbWrapper *DatabaseWrapper, logger *logger.Logger) *ProtectedReviewRepository {
	return &ProtectedReviewRepository{
		dbWrapper: dbWrapper,
		logger:    logger,
	}
}

// CreateReview creates a new review with circuit breaker protection
func (r *ProtectedReviewRepository) CreateReview(ctx context.Context, review *domain.Review) error {
	return r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		// Set timestamps
		now := time.Now()
		review.CreatedAt = now
		review.UpdatedAt = now

		// Generate ID if not set
		if review.ID == uuid.Nil {
			review.ID = uuid.New()
		}

		// Create the review
		if err := db.Create(review).Error; err != nil {
			r.logger.Error("Failed to create review", "error", err, "review_id", review.ID)
			return err
		}

		r.logger.Info("Review created successfully", "review_id", review.ID)
		return nil
	})
}

// GetReviewByID retrieves a review by ID with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	var review domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.First(&review, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrReviewNotFound
			}
			r.logger.Error("Failed to get review by ID", "error", err, "review_id", id)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &review, nil
}

// GetReviewsByHotelID retrieves reviews for a specific hotel with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewsByHotelID(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Where("hotel_id = ?", hotelID).
			Limit(limit).Offset(offset).
			Order("created_at DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to get reviews by hotel ID", "error", err, "hotel_id", hotelID)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// GetReviewsByProviderID retrieves reviews for a specific provider with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewsByProviderID(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Where("provider_id = ?", providerID).
			Limit(limit).Offset(offset).
			Order("created_at DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to get reviews by provider ID", "error", err, "provider_id", providerID)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// GetReviews retrieves reviews with pagination and circuit breaker protection
func (r *ProtectedReviewRepository) GetReviews(ctx context.Context, limit, offset int) ([]*domain.Review, int64, error) {
	var reviews []*domain.Review
	var total int64

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		// Get total count
		if err := db.Model(&domain.Review{}).Count(&total).Error; err != nil {
			r.logger.Error("Failed to count reviews", "error", err)
			return err
		}

		// Get reviews with pagination
		if err := db.Limit(limit).Offset(offset).
			Order("created_at DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to get reviews", "error", err)
			return err
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// UpdateReview updates an existing review with circuit breaker protection
func (r *ProtectedReviewRepository) UpdateReview(ctx context.Context, review *domain.Review) error {
	return r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		// Update timestamp
		review.UpdatedAt = time.Now()

		// Update the review
		if err := db.Save(review).Error; err != nil {
			r.logger.Error("Failed to update review", "error", err, "review_id", review.ID)
			return err
		}

		r.logger.Info("Review updated successfully", "review_id", review.ID)
		return nil
	})
}

// DeleteReview deletes a review with circuit breaker protection
func (r *ProtectedReviewRepository) DeleteReview(ctx context.Context, id uuid.UUID) error {
	return r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		result := db.Delete(&domain.Review{}, "id = ?", id)
		if result.Error != nil {
			r.logger.Error("Failed to delete review", "error", result.Error, "review_id", id)
			return result.Error
		}

		if result.RowsAffected == 0 {
			return domain.ErrReviewNotFound
		}

		r.logger.Info("Review deleted successfully", "review_id", id)
		return nil
	})
}

// GetReviewsByExternalID retrieves reviews by external ID with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewsByExternalID(ctx context.Context, providerID uuid.UUID, externalID string) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Where("provider_id = ? AND external_id = ?", providerID, externalID).
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to get reviews by external ID",
				"error", err,
				"provider_id", providerID,
				"external_id", externalID)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// GetReviewCount returns the total number of reviews with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewCount(ctx context.Context) (int64, error) {
	var count int64

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Model(&domain.Review{}).Count(&count).Error; err != nil {
			r.logger.Error("Failed to count reviews", "error", err)
			return err
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetReviewStatistics returns review statistics with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewStatistics(ctx context.Context) (*domain.ReviewStatistics, error) {
	var stats domain.ReviewStatistics

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		// Get total count
		if err := db.Model(&domain.Review{}).Count(&stats.TotalReviews).Error; err != nil {
			r.logger.Error("Failed to count total reviews", "error", err)
			return err
		}

		// Get average rating
		var avgRating float64
		if err := db.Model(&domain.Review{}).
			Select("AVG(rating)").
			Scan(&avgRating).Error; err != nil {
			r.logger.Error("Failed to calculate average rating", "error", err)
			return err
		}
		stats.AverageRating = avgRating

		// Get rating distribution
		var ratingDistribution []struct {
			Rating float64
			Count  int64
		}
		if err := db.Model(&domain.Review{}).
			Select("rating, COUNT(*) as count").
			Group("rating").
			Scan(&ratingDistribution).Error; err != nil {
			r.logger.Error("Failed to get rating distribution", "error", err)
			return err
		}

		stats.RatingDistribution = make(map[int]int)
		for _, rd := range ratingDistribution {
			stats.RatingDistribution[int(rd.Rating)] = int(rd.Count)
		}

		// Get reviews by provider
		var providerStats []struct {
			ProviderID uuid.UUID
			Count      int64
		}
		if err := db.Model(&domain.Review{}).
			Select("provider_id, COUNT(*) as count").
			Group("provider_id").
			Scan(&providerStats).Error; err != nil {
			r.logger.Error("Failed to get provider statistics", "error", err)
			return err
		}

		stats.ReviewsByProvider = make(map[string]int)
		for _, ps := range providerStats {
			stats.ReviewsByProvider[ps.ProviderID.String()] = int(ps.Count)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// BatchCreateReviews creates multiple reviews in a transaction with circuit breaker protection
func (r *ProtectedReviewRepository) BatchCreateReviews(ctx context.Context, reviews []*domain.Review) error {
	return r.dbWrapper.ExecuteTransaction(ctx, func(db *gorm.DB) error {
		now := time.Now()

		for _, review := range reviews {
			// Set timestamps
			review.CreatedAt = now
			review.UpdatedAt = now

			// Generate ID if not set
			if review.ID == uuid.Nil {
				review.ID = uuid.New()
			}
		}

		// Batch create reviews
		if err := db.CreateInBatches(reviews, 100).Error; err != nil {
			r.logger.Error("Failed to batch create reviews", "error", err, "count", len(reviews))
			return err
		}

		r.logger.Info("Reviews batch created successfully", "count", len(reviews))
		return nil
	})
}

// FindReviewsByDateRange finds reviews within a date range with circuit breaker protection
func (r *ProtectedReviewRepository) FindReviewsByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Where("review_date BETWEEN ? AND ?", startDate, endDate).
			Limit(limit).Offset(offset).
			Order("review_date DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to find reviews by date range",
				"error", err,
				"start_date", startDate,
				"end_date", endDate)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// FindReviewsByRating finds reviews with a specific rating with circuit breaker protection
func (r *ProtectedReviewRepository) FindReviewsByRating(ctx context.Context, rating float64, limit, offset int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Where("rating = ?", rating).
			Limit(limit).Offset(offset).
			Order("created_at DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to find reviews by rating", "error", err, "rating", rating)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// SearchReviews searches reviews by text with circuit breaker protection
func (r *ProtectedReviewRepository) SearchReviews(ctx context.Context, query string, limit, offset int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		searchPattern := fmt.Sprintf("%%%s%%", query)
		if err := db.Where("title ILIKE ? OR comment ILIKE ?", searchPattern, searchPattern).
			Limit(limit).Offset(offset).
			Order("created_at DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to search reviews", "error", err, "query", query)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// GetReviewsByLanguage retrieves reviews by language with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewsByLanguage(ctx context.Context, language string, limit, offset int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Where("language = ?", language).
			Limit(limit).Offset(offset).
			Order("created_at DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to get reviews by language", "error", err, "language", language)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// GetLatestReviews retrieves the most recent reviews with circuit breaker protection
func (r *ProtectedReviewRepository) GetLatestReviews(ctx context.Context, limit int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Limit(limit).
			Order("created_at DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to get latest reviews", "error", err, "limit", limit)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// GetReviewsWithRatings retrieves reviews with specific rating criteria with circuit breaker protection
func (r *ProtectedReviewRepository) GetReviewsWithRatings(ctx context.Context, minRating, maxRating float64, limit, offset int) ([]*domain.Review, error) {
	var reviews []*domain.Review

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		if err := db.Where("rating BETWEEN ? AND ?", minRating, maxRating).
			Limit(limit).Offset(offset).
			Order("rating DESC").
			Find(&reviews).Error; err != nil {
			r.logger.Error("Failed to get reviews with ratings",
				"error", err,
				"min_rating", minRating,
				"max_rating", maxRating)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// Health check method for the repository
func (r *ProtectedReviewRepository) HealthCheck(ctx context.Context) error {
	return r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		// Simple health check query
		var count int64
		if err := db.Model(&domain.Review{}).Limit(1).Count(&count).Error; err != nil {
			r.logger.Error("Repository health check failed", "error", err)
			return err
		}
		return nil
	})
}

// GetRepositoryMetrics returns repository performance metrics
func (r *ProtectedReviewRepository) GetRepositoryMetrics(ctx context.Context) (*RepositoryMetrics, error) {
	var metrics RepositoryMetrics

	err := r.dbWrapper.ExecuteQuery(ctx, func(db *gorm.DB) error {
		// Get total reviews
		if err := db.Model(&domain.Review{}).Count(&metrics.TotalReviews).Error; err != nil {
			return err
		}

		// Get reviews created today
		today := time.Now().Truncate(24 * time.Hour)
		if err := db.Model(&domain.Review{}).
			Where("created_at >= ?", today).
			Count(&metrics.ReviewsToday).Error; err != nil {
			return err
		}

		// Get reviews created this week
		weekAgo := time.Now().AddDate(0, 0, -7)
		if err := db.Model(&domain.Review{}).
			Where("created_at >= ?", weekAgo).
			Count(&metrics.ReviewsThisWeek).Error; err != nil {
			return err
		}

		// Get reviews created this month
		monthAgo := time.Now().AddDate(0, -1, 0)
		if err := db.Model(&domain.Review{}).
			Where("created_at >= ?", monthAgo).
			Count(&metrics.ReviewsThisMonth).Error; err != nil {
			return err
		}

		// Get average rating
		if err := db.Model(&domain.Review{}).
			Select("AVG(rating)").
			Scan(&metrics.AverageRating).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	metrics.LastUpdated = time.Now()
	return &metrics, nil
}

// RepositoryMetrics represents repository performance metrics
type RepositoryMetrics struct {
	TotalReviews     int64     `json:"total_reviews"`
	ReviewsToday     int64     `json:"reviews_today"`
	ReviewsThisWeek  int64     `json:"reviews_this_week"`
	ReviewsThisMonth int64     `json:"reviews_this_month"`
	AverageRating    float64   `json:"average_rating"`
	LastUpdated      time.Time `json:"last_updated"`
}
