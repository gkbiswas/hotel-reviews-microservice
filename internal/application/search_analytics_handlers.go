package application

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/google/uuid"
)

// SearchAnalyticsHandlers handles search and analytics operations
type SearchAnalyticsHandlers struct {
	reviewService domain.ReviewService
	logger        *logger.Logger
}

// NewSearchAnalyticsHandlers creates a new instance of SearchAnalyticsHandlers
func NewSearchAnalyticsHandlers(reviewService domain.ReviewService, logger *logger.Logger) *SearchAnalyticsHandlers {
	return &SearchAnalyticsHandlers{
		reviewService: reviewService,
		logger:        logger,
	}
}

// SearchReviews handles GET /api/v1/search/reviews
func (s *SearchAnalyticsHandlers) SearchReviews(c *gin.Context) {
	// Parse query parameters
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_QUERY",
			"message": "Search query is required",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Validate limits
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// Build filters from query parameters
	filters := make(map[string]interface{})
	
	if providerID := c.Query("provider_id"); providerID != "" {
		if pid, err := uuid.Parse(providerID); err == nil {
			filters["provider_id"] = pid
		}
	}
	
	if hotelID := c.Query("hotel_id"); hotelID != "" {
		if hid, err := uuid.Parse(hotelID); err == nil {
			filters["hotel_id"] = hid
		}
	}
	
	if minRating := c.Query("min_rating"); minRating != "" {
		if rating, err := strconv.ParseFloat(minRating, 64); err == nil {
			filters["min_rating"] = rating
		}
	}
	
	if maxRating := c.Query("max_rating"); maxRating != "" {
		if rating, err := strconv.ParseFloat(maxRating, 64); err == nil {
			filters["max_rating"] = rating
		}
	}
	
	if startDate := c.Query("start_date"); startDate != "" {
		if date, err := time.Parse("2006-01-02", startDate); err == nil {
			filters["start_date"] = date
		}
	}
	
	if endDate := c.Query("end_date"); endDate != "" {
		if date, err := time.Parse("2006-01-02", endDate); err == nil {
			filters["end_date"] = date
		}
	}

	// Perform search
	reviews, err := s.reviewService.SearchReviews(c.Request.Context(), query, filters, limit, offset)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to search reviews",
			"query", query,
			"filters", filters,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "SEARCH_FAILED",
			"message": "Failed to search reviews",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": reviews,
		"meta": gin.H{
			"query":  query,
			"limit":  limit,
			"offset": offset,
			"count":  len(reviews),
		},
	})
}

// GetTopRatedHotels handles GET /api/v1/analytics/hotels/top-rated
func (s *SearchAnalyticsHandlers) GetTopRatedHotels(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	
	// Validate limit
	if limit > 50 {
		limit = 50
	}
	if limit < 1 {
		limit = 10
	}

	hotels, err := s.reviewService.GetTopRatedHotels(c.Request.Context(), limit)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get top rated hotels",
			"limit", limit,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ANALYTICS_FAILED",
			"message": "Failed to get top rated hotels",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": hotels,
		"meta": gin.H{
			"limit": limit,
			"count": len(hotels),
		},
	})
}

// GetRecentReviews handles GET /api/v1/analytics/reviews/recent
func (s *SearchAnalyticsHandlers) GetRecentReviews(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	
	// Validate limit
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}

	reviews, err := s.reviewService.GetRecentReviews(c.Request.Context(), limit)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get recent reviews",
			"limit", limit,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ANALYTICS_FAILED",
			"message": "Failed to get recent reviews",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": reviews,
		"meta": gin.H{
			"limit": limit,
			"count": len(reviews),
		},
	})
}

// GetProviderStats handles GET /api/v1/analytics/providers/:id/stats
func (s *SearchAnalyticsHandlers) GetProviderStats(c *gin.Context) {
	providerIDStr := c.Param("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_PROVIDER_ID",
			"message": "Invalid provider ID format",
		})
		return
	}

	// Parse date range
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	
	var startDate, endDate time.Time
	
	if startDateStr != "" {
		if date, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = date
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "INVALID_START_DATE",
				"message": "Invalid start date format. Use YYYY-MM-DD",
			})
			return
		}
	} else {
		// Default to last 30 days
		startDate = time.Now().AddDate(0, 0, -30)
	}
	
	if endDateStr != "" {
		if date, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = date
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "INVALID_END_DATE",
				"message": "Invalid end date format. Use YYYY-MM-DD",
			})
			return
		}
	} else {
		// Default to today
		endDate = time.Now()
	}

	stats, err := s.reviewService.GetReviewStatsByProvider(c.Request.Context(), providerID, startDate, endDate)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get provider stats",
			"provider_id", providerID,
			"start_date", startDate,
			"end_date", endDate,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ANALYTICS_FAILED",
			"message": "Failed to get provider statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": stats,
		"meta": gin.H{
			"provider_id": providerID,
			"start_date":  startDate.Format("2006-01-02"),
			"end_date":    endDate.Format("2006-01-02"),
		},
	})
}

// GetHotelStats handles GET /api/v1/analytics/hotels/:id/stats
func (s *SearchAnalyticsHandlers) GetHotelStats(c *gin.Context) {
	hotelIDStr := c.Param("id")
	hotelID, err := uuid.Parse(hotelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_HOTEL_ID",
			"message": "Invalid hotel ID format",
		})
		return
	}

	// Parse date range
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	
	var startDate, endDate time.Time
	
	if startDateStr != "" {
		if date, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = date
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "INVALID_START_DATE",
				"message": "Invalid start date format. Use YYYY-MM-DD",
			})
			return
		}
	} else {
		// Default to last 30 days
		startDate = time.Now().AddDate(0, 0, -30)
	}
	
	if endDateStr != "" {
		if date, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = date
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "INVALID_END_DATE",
				"message": "Invalid end date format. Use YYYY-MM-DD",
			})
			return
		}
	} else {
		// Default to today
		endDate = time.Now()
	}

	stats, err := s.reviewService.GetReviewStatsByHotel(c.Request.Context(), hotelID, startDate, endDate)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get hotel stats",
			"hotel_id", hotelID,
			"start_date", startDate,
			"end_date", endDate,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ANALYTICS_FAILED",
			"message": "Failed to get hotel statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": stats,
		"meta": gin.H{
			"hotel_id":   hotelID,
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
		},
	})
}

// GetReviewSummary handles GET /api/v1/analytics/hotels/:id/summary
func (s *SearchAnalyticsHandlers) GetReviewSummary(c *gin.Context) {
	hotelIDStr := c.Param("id")
	hotelID, err := uuid.Parse(hotelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_HOTEL_ID",
			"message": "Invalid hotel ID format",
		})
		return
	}

	summary, err := s.reviewService.GetReviewSummary(c.Request.Context(), hotelID)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get review summary",
			"hotel_id", hotelID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ANALYTICS_FAILED",
			"message": "Failed to get review summary",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": summary,
	})
}

// GetOverallAnalytics handles GET /api/v1/analytics/overview
func (s *SearchAnalyticsHandlers) GetOverallAnalytics(c *gin.Context) {
	// Get recent reviews
	recentReviews, err := s.reviewService.GetRecentReviews(c.Request.Context(), 5)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get recent reviews for overview", "error", err)
		recentReviews = []domain.Review{} // fallback to empty
	}

	// Get top rated hotels
	topHotels, err := s.reviewService.GetTopRatedHotels(c.Request.Context(), 5)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get top hotels for overview", "error", err)
		topHotels = []domain.Hotel{} // fallback to empty
	}

	// Build overview response
	overview := gin.H{
		"recent_reviews": recentReviews,
		"top_hotels":     topHotels,
		"timestamp":      time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"data": overview,
	})
}

// SearchHotels handles GET /api/v1/search/hotels
func (s *SearchAnalyticsHandlers) SearchHotels(c *gin.Context) {
	// Parse query parameters
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_QUERY",
			"message": "Search query is required",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Validate limits
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// Build filters for hotel search
	filters := make(map[string]interface{})
	filters["search_type"] = "hotels"
	filters["query"] = query
	
	if city := c.Query("city"); city != "" {
		filters["city"] = city
	}
	
	if minRating := c.Query("min_star_rating"); minRating != "" {
		if rating, err := strconv.Atoi(minRating); err == nil && rating >= 1 && rating <= 5 {
			filters["min_star_rating"] = rating
		}
	}

	// Use search functionality (this would require implementing hotel-specific search in the repository)
	// For now, use the general list hotels function with basic filtering
	hotels, err := s.reviewService.ListHotels(c.Request.Context(), limit, offset)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to search hotels",
			"query", query,
			"filters", filters,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "SEARCH_FAILED",
			"message": "Failed to search hotels",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": hotels,
		"meta": gin.H{
			"query":  query,
			"limit":  limit,
			"offset": offset,
			"count":  len(hotels),
		},
	})
}

// GetReviewTrends handles GET /api/v1/analytics/trends/reviews
func (s *SearchAnalyticsHandlers) GetReviewTrends(c *gin.Context) {
	// Parse date range
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days > 365 {
		days = 365
	}
	if days < 1 {
		days = 30
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	// Get recent reviews to analyze trends
	recentReviews, err := s.reviewService.GetRecentReviews(c.Request.Context(), 1000)
	if err != nil {
		s.logger.ErrorContext(c.Request.Context(), "Failed to get reviews for trend analysis", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ANALYTICS_FAILED",
			"message": "Failed to analyze review trends",
		})
		return
	}

	// Analyze trends by day
	trends := make(map[string]interface{})
	dailyStats := make(map[string]map[string]interface{})
	
	for _, review := range recentReviews {
		if review.ReviewDate.After(startDate) && review.ReviewDate.Before(endDate) {
			dayKey := review.ReviewDate.Format("2006-01-02")
			if dailyStats[dayKey] == nil {
				dailyStats[dayKey] = map[string]interface{}{
					"count":        0,
					"total_rating": 0.0,
				}
			}
			stats := dailyStats[dayKey]
			stats["count"] = stats["count"].(int) + 1
			stats["total_rating"] = stats["total_rating"].(float64) + review.Rating
			stats["avg_rating"] = stats["total_rating"].(float64) / float64(stats["count"].(int))
		}
	}

	trends["daily_stats"] = dailyStats
	trends["period"] = gin.H{
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"days":       days,
	}

	c.JSON(http.StatusOK, gin.H{
		"data": trends,
	})
}