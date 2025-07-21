package application

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/google/uuid"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

type ReviewHandlers struct {
	reviewService domain.ReviewService
	logger        *logger.Logger
}

func NewReviewHandlers(reviewService domain.ReviewService, logger *logger.Logger) *ReviewHandlers {
	return &ReviewHandlers{
		reviewService: reviewService,
		logger:        logger,
	}
}

// CreateReview handles POST /api/v1/reviews
func (r *ReviewHandlers) CreateReview(c *gin.Context) {
	var review domain.Review
	if err := c.ShouldBindJSON(&review); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid review data",
			"details": err.Error(),
		})
		return
	}

	// Generate ID if not provided
	if review.ID == uuid.Nil {
		review.ID = uuid.New()
	}

	// Set default values BEFORE validation
	if review.ReviewDate.IsZero() {
		review.ReviewDate = time.Now()
	}
	if review.Language == "" {
		review.Language = "en"
	}

	// Validate required fields
	if err := r.validateReviewInput(&review); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Create review
	if err := r.reviewService.CreateReview(c.Request.Context(), &review); err != nil {
		if err == domain.ErrReviewAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "REVIEW_ALREADY_EXISTS",
				"message": "Review already exists",
			})
			return
		}

		// Check if it's a validation error from the service
		if strings.Contains(err.Error(), "validation failed") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "VALIDATION_FAILED",
				"message": err.Error(),
			})
			return
		}

		r.logger.ErrorContext(c.Request.Context(), "Failed to create review", "error", err, "review", review)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to create review",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Review created successfully",
		"review":  review,
	})
}

// GetReview handles GET /api/v1/reviews/:id
func (r *ReviewHandlers) GetReview(c *gin.Context) {
	// Parse review ID
	reviewIDStr := c.Param("id")
	reviewID, err := uuid.Parse(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid review ID format",
		})
		return
	}

	// Get review
	review, err := r.reviewService.GetReviewByID(c.Request.Context(), reviewID)
	if err != nil {
		if err == domain.ErrReviewNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "REVIEW_NOT_FOUND",
				"message": "Review not found",
			})
			return
		}

		r.logger.ErrorContext(c.Request.Context(), "Failed to get review", "error", err, "review_id", reviewID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve review",
		})
		return
	}

	c.JSON(http.StatusOK, review)
}

// GetReviews handles GET /api/v1/reviews
func (r *ReviewHandlers) GetReviews(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	
	// Parse hotel ID filter if provided
	var hotelID *uuid.UUID
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if parsed, err := uuid.Parse(hotelIDStr); err == nil {
			hotelID = &parsed
		}
	}

	// Parse provider ID filter if provided
	var providerID *uuid.UUID
	if providerIDStr := c.Query("provider_id"); providerIDStr != "" {
		if parsed, err := uuid.Parse(providerIDStr); err == nil {
			providerID = &parsed
		}
	}

	// Build filter
	filter := domain.ReviewFilter{
		HotelID:    hotelID,
		ProviderID: providerID,
		MinRating:  0,
		MaxRating:  5,
		Limit:      limit,
		Offset:     offset,
	}

	// Parse rating filters
	if minRating := c.Query("min_rating"); minRating != "" {
		if parsed, err := strconv.ParseFloat(minRating, 64); err == nil {
			filter.MinRating = parsed
		}
	}
	if maxRating := c.Query("max_rating"); maxRating != "" {
		if parsed, err := strconv.ParseFloat(maxRating, 64); err == nil {
			filter.MaxRating = parsed
		}
	}

	// Get reviews
	var reviews []*domain.Review
	var err error

	if hotelID != nil {
		// Use hotel-specific method
		reviewsList, getErr := r.reviewService.GetReviewsByHotel(c.Request.Context(), *hotelID, limit, offset)
		if getErr != nil {
			err = getErr
		} else {
			reviews = make([]*domain.Review, len(reviewsList))
			for i, review := range reviewsList {
				reviews[i] = &review
			}
		}
	} else if providerID != nil {
		// Use provider-specific method
		reviewsList, getErr := r.reviewService.GetReviewsByProvider(c.Request.Context(), *providerID, limit, offset)
		if getErr != nil {
			err = getErr
		} else {
			reviews = make([]*domain.Review, len(reviewsList))
			for i, review := range reviewsList {
				reviews[i] = &review
			}
		}
	} else {
		// For now, return empty list for general queries
		// In a full implementation, this would use a SearchReviews method
		reviews = []*domain.Review{}
	}

	if err != nil {
		r.logger.ErrorContext(c.Request.Context(), "Failed to get reviews", "error", err, "filter", filter)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve reviews",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reviews": reviews,
		"meta": gin.H{
			"limit":  limit,
			"offset": offset,
			"count":  len(reviews),
		},
	})
}

// UpdateReview handles PUT /api/v1/reviews/:id
func (r *ReviewHandlers) UpdateReview(c *gin.Context) {
	// Parse review ID
	reviewIDStr := c.Param("id")
	reviewID, err := uuid.Parse(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid review ID format",
		})
		return
	}

	var review domain.Review
	if err := c.ShouldBindJSON(&review); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid review data",
			"details": err.Error(),
		})
		return
	}

	// Set the ID from the URL
	review.ID = reviewID

	// Set default values BEFORE validation
	if review.ReviewDate.IsZero() {
		review.ReviewDate = time.Now()
	}
	if review.Language == "" {
		review.Language = "en"
	}

	// Validate required fields
	if err := r.validateReviewInput(&review); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Update review
	if err := r.reviewService.UpdateReview(c.Request.Context(), &review); err != nil {
		if err == domain.ErrReviewNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "REVIEW_NOT_FOUND",
				"message": "Review not found",
			})
			return
		}

		// Check if it's a validation error from the service
		if strings.Contains(err.Error(), "validation failed") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "VALIDATION_FAILED",
				"message": err.Error(),
			})
			return
		}

		r.logger.ErrorContext(c.Request.Context(), "Failed to update review", "error", err, "review", review)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to update review",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Review updated successfully",
		"review":  review,
	})
}

// DeleteReview handles DELETE /api/v1/reviews/:id
func (r *ReviewHandlers) DeleteReview(c *gin.Context) {
	// Parse review ID
	reviewIDStr := c.Param("id")
	reviewID, err := uuid.Parse(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid review ID format",
		})
		return
	}

	// Delete review
	if err := r.reviewService.DeleteReview(c.Request.Context(), reviewID); err != nil {
		if err == domain.ErrReviewNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "REVIEW_NOT_FOUND",
				"message": "Review not found",
			})
			return
		}

		r.logger.ErrorContext(c.Request.Context(), "Failed to delete review", "error", err, "review_id", reviewID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to delete review",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Review deleted successfully",
	})
}

// validateReviewInput validates review input data
func (r *ReviewHandlers) validateReviewInput(review *domain.Review) error {
	if review.HotelID == uuid.Nil {
		return domain.NewValidationError("hotel_id", "Hotel ID is required")
	}

	if review.ProviderID == uuid.Nil {
		return domain.NewValidationError("provider_id", "Provider ID is required")
	}

	if review.Rating < 1.0 || review.Rating > 5.0 {
		return domain.NewValidationError("rating", "Rating must be between 1.0 and 5.0")
	}

	if review.Title == "" {
		return domain.NewValidationError("title", "Review title is required")
	}
	if len(review.Title) < 3 {
		return domain.NewValidationError("title", "Review title must be at least 3 characters")
	}
	if len(review.Title) > 500 {
		return domain.NewValidationError("title", "Review title must be less than 500 characters")
	}

	if review.Comment == "" {
		return domain.NewValidationError("comment", "Review comment is required")
	}
	if len(review.Comment) < 10 {
		return domain.NewValidationError("comment", "Review comment must be at least 10 characters")
	}
	if len(review.Comment) > 5000 {
		return domain.NewValidationError("comment", "Review comment must be less than 5000 characters")
	}

	if review.ReviewDate.IsZero() {
		return domain.NewValidationError("review_date", "Review date is required")
	}

	// ReviewDate should not be in the future
	if review.ReviewDate.After(time.Now()) {
		return domain.NewValidationError("review_date", "Review date cannot be in the future")
	}

	return nil
}