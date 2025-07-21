package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// BusinessMetricsMiddleware provides business metrics tracking for API requests
type BusinessMetricsMiddleware struct {
	businessMetrics *monitoring.BusinessMetrics
	logger          *slog.Logger
}

// NewBusinessMetricsMiddleware creates a new business metrics middleware
func NewBusinessMetricsMiddleware(businessMetrics *monitoring.BusinessMetrics, logger *slog.Logger) *BusinessMetricsMiddleware {
	return &BusinessMetricsMiddleware{
		businessMetrics: businessMetrics,
		logger:          logger,
	}
}

// Middleware returns the Gin middleware function for business metrics tracking
func (m *BusinessMetricsMiddleware) Middleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()
		
		// Process request
		c.Next()
		
		// Calculate duration
		duration := time.Since(start)
		
		// Extract metrics data
		method := c.Request.Method
		path := c.FullPath()
		statusCode := c.Writer.Status()
		
		// Track HTTP metrics
		m.trackHTTPMetrics(method, path, statusCode, duration)
		
		// Track business-specific metrics based on endpoint
		m.trackBusinessMetrics(c, method, path, statusCode, duration)
	})
}

// trackHTTPMetrics tracks general HTTP metrics
func (m *BusinessMetricsMiddleware) trackHTTPMetrics(method, path string, statusCode int, duration time.Duration) {
	if m.businessMetrics == nil {
		return
	}
	
	statusStr := strconv.Itoa(statusCode)
	
	// Track API response times (we'll reuse the provider response time metric for API responses)
	if m.businessMetrics.ProviderResponseTime != nil {
		m.businessMetrics.ProviderResponseTime.WithLabelValues("api", method, path).Observe(duration.Seconds())
	}
	
	// Track error rates
	if statusCode >= 400 && m.businessMetrics.ProviderErrors != nil {
		m.businessMetrics.ProviderErrors.WithLabelValues("api", method, statusStr).Inc()
	}
}

// trackBusinessMetrics tracks business-specific metrics based on the endpoint
func (m *BusinessMetricsMiddleware) trackBusinessMetrics(c *gin.Context, method, path string, statusCode int, duration time.Duration) {
	if m.businessMetrics == nil {
		return
	}
	
	switch {
	case path == "/api/v1/reviews" && method == "POST":
		m.trackReviewCreation(c, statusCode)
		
	case path == "/api/v1/reviews" && method == "GET":
		m.trackReviewQuery(c, statusCode)
		
	case path == "/api/v1/files/upload" && method == "POST":
		m.trackFileUpload(c, statusCode, duration)
		
	case path == "/api/v1/search/reviews" && method == "GET":
		m.trackSearchOperation(c, statusCode, duration)
		
	case path == "/api/v1/search/hotels" && method == "GET":
		m.trackSearchOperation(c, statusCode, duration)
		
	case path == "/api/v1/hotels" && method == "POST":
		m.trackHotelCreation(c, statusCode)
		
	case path == "/api/v1/providers" && method == "POST":
		m.trackProviderOperation(c, statusCode, "create")
		
	default:
		// Track general API usage patterns
		m.trackGeneralUsage(c, method, path, statusCode)
	}
}

// trackReviewCreation tracks metrics for review creation
func (m *BusinessMetricsMiddleware) trackReviewCreation(c *gin.Context, statusCode int) {
	provider := m.extractProvider(c)
	hotelID := m.extractHotelID(c)
	
	if statusCode >= 200 && statusCode < 300 {
		// Successful review creation
		if m.businessMetrics.ReviewsIngested != nil {
			m.businessMetrics.ReviewsIngested.WithLabelValues(provider, hotelID, "api").Inc()
		}
		
		if m.businessMetrics.ReviewsStored != nil {
			m.businessMetrics.ReviewsStored.WithLabelValues(provider, hotelID).Inc()
		}
		
		// Update review velocity
		if m.businessMetrics.ReviewVelocity != nil {
			m.businessMetrics.ReviewVelocity.WithLabelValues(provider).Inc()
		}
		
	} else if statusCode >= 400 {
		// Failed review creation
		reason := m.determineRejectionReason(statusCode)
		if m.businessMetrics.ReviewsRejected != nil {
			m.businessMetrics.ReviewsRejected.WithLabelValues(provider, reason).Inc()
		}
	}
}

// trackReviewQuery tracks metrics for review queries
func (m *BusinessMetricsMiddleware) trackReviewQuery(c *gin.Context, statusCode int) {
	if statusCode >= 200 && statusCode < 300 {
		// Track user activity pattern
		hour := time.Now().Hour()
		if m.businessMetrics.UserActivityPattern != nil {
			m.businessMetrics.UserActivityPattern.WithLabelValues("query", strconv.Itoa(hour)).Inc()
		}
	}
}

// trackFileUpload tracks metrics for file upload operations
func (m *BusinessMetricsMiddleware) trackFileUpload(c *gin.Context, statusCode int, duration time.Duration) {
	provider := m.extractProvider(c)
	
	if statusCode >= 200 && statusCode < 300 {
		// Successful file upload
		if m.businessMetrics.FilesProcessed != nil {
			m.businessMetrics.FilesProcessed.WithLabelValues(provider, "upload", "success").Inc()
		}
		
		if m.businessMetrics.FileProcessingTime != nil {
			m.businessMetrics.FileProcessingTime.WithLabelValues(provider, "upload").Observe(duration.Seconds())
		}
		
		// Estimate file size from content length
		if contentLength := c.Request.ContentLength; contentLength > 0 {
			if m.businessMetrics.FileSize != nil {
				m.businessMetrics.FileSize.WithLabelValues(provider).Observe(float64(contentLength))
			}
		}
		
	} else if statusCode >= 400 {
		// Failed file upload
		if m.businessMetrics.FilesProcessed != nil {
			m.businessMetrics.FilesProcessed.WithLabelValues(provider, "upload", "failed").Inc()
		}
	}
}

// trackSearchOperation tracks metrics for search operations
func (m *BusinessMetricsMiddleware) trackSearchOperation(c *gin.Context, statusCode int, duration time.Duration) {
	searchType := "reviews"
	if c.FullPath() == "/api/v1/search/hotels" {
		searchType = "hotels"
	}
	
	if statusCode >= 200 && statusCode < 300 {
		// Track search performance
		if m.businessMetrics.ProviderResponseTime != nil {
			m.businessMetrics.ProviderResponseTime.WithLabelValues("search", searchType, "api").Observe(duration.Seconds())
		}
		
		// Track user activity
		hour := time.Now().Hour()
		if m.businessMetrics.UserActivityPattern != nil {
			m.businessMetrics.UserActivityPattern.WithLabelValues("search", strconv.Itoa(hour)).Inc()
		}
	}
}

// trackHotelCreation tracks metrics for hotel creation
func (m *BusinessMetricsMiddleware) trackHotelCreation(c *gin.Context, statusCode int) {
	if statusCode >= 200 && statusCode < 300 {
		// Update hotels total count
		if m.businessMetrics.HotelsTotal != nil {
			m.businessMetrics.HotelsTotal.WithLabelValues("all").Inc()
		}
	}
}

// trackProviderOperation tracks metrics for provider operations
func (m *BusinessMetricsMiddleware) trackProviderOperation(c *gin.Context, statusCode int, operation string) {
	provider := m.extractProvider(c)
	
	if statusCode >= 200 && statusCode < 300 {
		// Track successful provider operations
		if m.businessMetrics.ProviderRequestsTotal != nil {
			m.businessMetrics.ProviderRequestsTotal.WithLabelValues(provider, operation, "success").Inc()
		}
	} else if statusCode >= 400 {
		// Track failed provider operations
		if m.businessMetrics.ProviderErrors != nil {
			m.businessMetrics.ProviderErrors.WithLabelValues(provider, operation, strconv.Itoa(statusCode)).Inc()
		}
	}
}

// trackGeneralUsage tracks general API usage patterns
func (m *BusinessMetricsMiddleware) trackGeneralUsage(c *gin.Context, method, path string, statusCode int) {
	// Track user activity patterns
	hour := time.Now().Hour()
	if m.businessMetrics.UserActivityPattern != nil {
		m.businessMetrics.UserActivityPattern.WithLabelValues("api", strconv.Itoa(hour)).Inc()
	}
	
	// Track error rates for monitoring
	if statusCode >= 500 {
		if m.businessMetrics.ErrorRate != nil {
			m.businessMetrics.ErrorRate.WithLabelValues("api", path).Set(1.0)
		}
	} else {
		if m.businessMetrics.ErrorRate != nil {
			m.businessMetrics.ErrorRate.WithLabelValues("api", path).Set(0.0)
		}
	}
}

// Helper functions to extract data from context

// extractProvider extracts provider information from request
func (m *BusinessMetricsMiddleware) extractProvider(c *gin.Context) string {
	// Try to get provider from query parameters
	if provider := c.Query("provider"); provider != "" {
		return provider
	}
	
	// Try to get provider from form data
	if provider := c.PostForm("provider"); provider != "" {
		return provider
	}
	
	// Try to get provider from headers
	if provider := c.GetHeader("X-Provider"); provider != "" {
		return provider
	}
	
	// Default to unknown
	return "unknown"
}

// extractHotelID extracts hotel ID from request
func (m *BusinessMetricsMiddleware) extractHotelID(c *gin.Context) string {
	// Try to get hotel ID from URL parameters
	if hotelID := c.Param("id"); hotelID != "" {
		return hotelID
	}
	
	// Try to get hotel ID from query parameters
	if hotelID := c.Query("hotel_id"); hotelID != "" {
		return hotelID
	}
	
	// Try to get hotel ID from form data
	if hotelID := c.PostForm("hotel_id"); hotelID != "" {
		return hotelID
	}
	
	// Default to unknown
	return "unknown"
}

// determineRejectionReason determines the reason for review rejection based on status code
func (m *BusinessMetricsMiddleware) determineRejectionReason(statusCode int) string {
	switch statusCode {
	case 400:
		return "invalid_data"
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 409:
		return "duplicate"
	case 422:
		return "validation_failed"
	case 429:
		return "rate_limited"
	case 500:
		return "server_error"
	default:
		return "unknown_error"
	}
}

// TrackCustomMetric allows manual tracking of custom business metrics
func (m *BusinessMetricsMiddleware) TrackCustomMetric(metricType, provider, action string, value float64) {
	if m.businessMetrics == nil {
		return
	}
	
	switch metricType {
	case "review_sentiment":
		if m.businessMetrics.ReviewSentimentDistribution != nil {
			m.businessMetrics.ReviewSentimentDistribution.WithLabelValues(provider, action).Add(value)
		}
		
	case "data_quality":
		if m.businessMetrics.DataQualityScore != nil {
			m.businessMetrics.DataQualityScore.WithLabelValues(provider).Set(value)
		}
		
	case "customer_satisfaction":
		if m.businessMetrics.CustomerSatisfactionScore != nil {
			m.businessMetrics.CustomerSatisfactionScore.WithLabelValues(provider).Set(value)
		}
		
	case "processing_capacity":
		if m.businessMetrics.ProcessingCapacity != nil {
			m.businessMetrics.ProcessingCapacity.WithLabelValues(provider).Set(value)
		}
		
	case "queue_depth":
		if m.businessMetrics.QueueDepth != nil {
			m.businessMetrics.QueueDepth.WithLabelValues(provider).Set(value)
		}
	}
}

// UpdateDataQualityMetrics updates data quality related metrics
func (m *BusinessMetricsMiddleware) UpdateDataQualityMetrics(provider string, qualityScore, completeness float64) {
	if m.businessMetrics == nil {
		return
	}
	
	if m.businessMetrics.DataQualityScore != nil {
		m.businessMetrics.DataQualityScore.WithLabelValues(provider).Set(qualityScore)
	}
	
	if m.businessMetrics.DataCompleteness != nil {
		m.businessMetrics.DataCompleteness.WithLabelValues(provider).Set(completeness)
	}
	
	// Update provider data quality
	if m.businessMetrics.ProviderDataQuality != nil {
		m.businessMetrics.ProviderDataQuality.WithLabelValues(provider).Set(qualityScore)
	}
}

// UpdateHotelMetrics updates hotel-related metrics
func (m *BusinessMetricsMiddleware) UpdateHotelMetrics(hotelID string, averageRating float64, reviewCount int) {
	if m.businessMetrics == nil {
		return
	}
	
	if m.businessMetrics.AverageRatingByHotel != nil {
		m.businessMetrics.AverageRatingByHotel.WithLabelValues(hotelID).Set(averageRating)
	}
	
	if m.businessMetrics.ReviewCountByHotel != nil {
		m.businessMetrics.ReviewCountByHotel.WithLabelValues(hotelID).Set(float64(reviewCount))
	}
}

// UpdateProcessingMetrics updates processing-related metrics
func (m *BusinessMetricsMiddleware) UpdateProcessingMetrics(capacity, utilization, queueDepth float64) {
	if m.businessMetrics == nil {
		return
	}
	
	if m.businessMetrics.ProcessingCapacity != nil {
		m.businessMetrics.ProcessingCapacity.WithLabelValues("system").Set(capacity)
	}
	
	if m.businessMetrics.ProcessingUtilization != nil {
		m.businessMetrics.ProcessingUtilization.WithLabelValues("system").Set(utilization)
	}
	
	if m.businessMetrics.QueueDepth != nil {
		m.businessMetrics.QueueDepth.WithLabelValues("system").Set(queueDepth)
	}
}