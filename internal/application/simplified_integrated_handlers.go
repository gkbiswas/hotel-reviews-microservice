package application

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// SimplifiedIntegratedHandlers provides integrated handlers using existing interfaces
type SimplifiedIntegratedHandlers struct {
	// Core services
	reviewService domain.ReviewService
	logger        *logger.Logger

	// Authentication & Authorization
	authService *infrastructure.AuthenticationService
	rbacService *infrastructure.RBACService

	// Resilience components
	circuitBreaker *infrastructure.CircuitBreakerIntegration
	retryManager   *infrastructure.RetryManager

	// Infrastructure components
	redisClient   *infrastructure.RedisClient
	kafkaProducer *infrastructure.KafkaProducer
	s3Client      domain.S3Client

	// Monitoring components
	errorHandler *infrastructure.ErrorHandler
	tracer       trace.Tracer

	// Processing engine
	processingEngine *ProcessingEngine
}

// NewSimplifiedIntegratedHandlers creates a new simplified integrated handlers instance
func NewSimplifiedIntegratedHandlers(
	reviewService domain.ReviewService,
	authService *infrastructure.AuthenticationService,
	rbacService *infrastructure.RBACService,
	circuitBreaker *infrastructure.CircuitBreakerIntegration,
	retryManager *infrastructure.RetryManager,
	redisClient *infrastructure.RedisClient,
	kafkaProducer *infrastructure.KafkaProducer,
	s3Client domain.S3Client,
	errorHandler *infrastructure.ErrorHandler,
	processingEngine *ProcessingEngine,
	logger *logger.Logger,
) *SimplifiedIntegratedHandlers {
	tracer := otel.Tracer("hotel-reviews-handlers")

	return &SimplifiedIntegratedHandlers{
		reviewService:    reviewService,
		authService:      authService,
		rbacService:      rbacService,
		circuitBreaker:   circuitBreaker,
		retryManager:     retryManager,
		redisClient:      redisClient,
		kafkaProducer:    kafkaProducer,
		s3Client:         s3Client,
		errorHandler:     errorHandler,
		tracer:           tracer,
		processingEngine: processingEngine,
		logger:           logger,
	}
}

// Request and Response types

// CreateReviewRequest represents the request to create a new review
type CreateReviewRequest struct {
	HotelID        string    `json:"hotel_id" binding:"required"`
	ReviewerInfoID string    `json:"reviewer_info_id" binding:"required"`
	ExternalID     *string   `json:"external_id,omitempty"`
	Rating         float64   `json:"rating" binding:"required,min=1,max=5"`
	Title          string    `json:"title" binding:"required"`
	Comment        string    `json:"comment" binding:"required"`
	ReviewDate     time.Time `json:"review_date"`
	Language       string    `json:"language" binding:"required"`
}

// UpdateReviewRequest represents the request to update a review
type UpdateReviewRequest struct {
	Rating   *float64 `json:"rating,omitempty" binding:"omitempty,min=1,max=5"`
	Title    *string  `json:"title,omitempty"`
	Comment  *string  `json:"comment,omitempty"`
	Language *string  `json:"language,omitempty"`
}

// StandardAPIResponse represents the standard API response format
type StandardAPIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	ErrorCode string      `json:"error_code,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	TraceID   string      `json:"trace_id,omitempty"`
}

// CreateReview creates a new review with resilience and monitoring
func (h *SimplifiedIntegratedHandlers) CreateReview(c *gin.Context) {
	ctx, span := h.tracer.Start(c.Request.Context(), "CreateReview")
	defer span.End()

	// Extract user from context (set by auth middleware)
	user, exists := c.Get("user")
	if !exists {
		h.respondWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	userObj := user.(*domain.User)
	span.SetAttributes(attribute.String("user.id", userObj.ID.String()))

	// Check permissions with RBAC and resilience
	hasPermission, err := h.executeWithResilience(ctx, "rbac", func(ctx context.Context) (interface{}, error) {
		return h.rbacService.CheckPermission(ctx, userObj.ID, "reviews", "create")
	})
	if err != nil {
		h.handleError(c, err, "Failed to check permissions")
		return
	}
	if !hasPermission.(bool) {
		h.respondWithError(c, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	// Parse request
	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Create review with resilience
	review, err := h.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.createReviewInternal(ctx, &req, userObj.ID)
	})
	if err != nil {
		h.handleError(c, err, "Failed to create review")
		return
	}

	reviewObj := review.(*domain.Review)

	// Publish event to Kafka (asynchronously)
	go h.publishReviewEvent(context.Background(), "review.created", reviewObj)

	span.SetAttributes(
		attribute.String("review.id", reviewObj.ID.String()),
		attribute.Float64("review.rating", reviewObj.Rating),
	)

	h.respondWithSuccess(c, http.StatusCreated, "Review created successfully", reviewObj)
}

// GetReview retrieves a review by ID
func (h *SimplifiedIntegratedHandlers) GetReview(c *gin.Context) {
	ctx, span := h.tracer.Start(c.Request.Context(), "GetReview")
	defer span.End()

	reviewID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, "INVALID_ID", "Invalid review ID")
		return
	}

	span.SetAttributes(attribute.String("review.id", reviewID.String()))

	// Get from database with resilience
	review, err := h.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.reviewService.GetReviewByID(ctx, reviewID)
	})
	if err != nil {
		h.handleError(c, err, "Failed to get review")
		return
	}

	h.respondWithSuccess(c, http.StatusOK, "Review retrieved successfully", review)
}

// ListReviews lists reviews with pagination
func (h *SimplifiedIntegratedHandlers) ListReviews(c *gin.Context) {
	ctx, span := h.tracer.Start(c.Request.Context(), "ListReviews")
	defer span.End()

	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	hotelID := c.Query("hotel_id")

	// Limit maximum page size
	if limit > 100 {
		limit = 100
	}

	span.SetAttributes(
		attribute.Int("query.limit", limit),
		attribute.Int("query.offset", offset),
		attribute.String("query.hotel_id", hotelID),
	)

	// Get from database with resilience
	reviews, err := h.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		if hotelID != "" {
			hotelUUID, err := uuid.Parse(hotelID)
			if err != nil {
				return nil, err
			}
			return h.reviewService.GetReviewsByHotel(ctx, hotelUUID, limit, offset)
		}
		// For general listing, use search with empty query
		return h.reviewService.SearchReviews(ctx, "", make(map[string]interface{}), limit, offset)
	})
	if err != nil {
		h.handleError(c, err, "Failed to list reviews")
		return
	}

	h.respondWithSuccess(c, http.StatusOK, "Reviews retrieved successfully", reviews)
}

// UpdateReview updates a review
func (h *SimplifiedIntegratedHandlers) UpdateReview(c *gin.Context) {
	ctx, span := h.tracer.Start(c.Request.Context(), "UpdateReview")
	defer span.End()

	user, _ := c.Get("user")
	userObj := user.(*domain.User)

	reviewID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, "INVALID_ID", "Invalid review ID")
		return
	}

	span.SetAttributes(
		attribute.String("review.id", reviewID.String()),
		attribute.String("user.id", userObj.ID.String()),
	)

	// Parse request
	var req UpdateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Get existing review to check ownership
	existingReview, err := h.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.reviewService.GetReviewByID(ctx, reviewID)
	})
	if err != nil {
		h.handleError(c, err, "Failed to get existing review")
		return
	}

	existingReviewObj := existingReview.(*domain.Review)

	// Check ownership (unless user is admin)
	if existingReviewObj.ProviderID != userObj.ID {
		adminCheck, err := h.rbacService.CheckPermission(ctx, userObj.ID, "reviews", "admin")
		if err != nil || !adminCheck {
			h.respondWithError(c, http.StatusForbidden, "FORBIDDEN", "Can only update own reviews")
			return
		}
	}

	// Update review with resilience
	updatedReview, err := h.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.updateReviewInternal(ctx, existingReviewObj, &req)
	})
	if err != nil {
		h.handleError(c, err, "Failed to update review")
		return
	}

	// Publish update event
	go h.publishReviewEvent(context.Background(), "review.updated", updatedReview.(*domain.Review))

	h.respondWithSuccess(c, http.StatusOK, "Review updated successfully", updatedReview)
}

// DeleteReview deletes a review
func (h *SimplifiedIntegratedHandlers) DeleteReview(c *gin.Context) {
	ctx, span := h.tracer.Start(c.Request.Context(), "DeleteReview")
	defer span.End()

	user, _ := c.Get("user")
	userObj := user.(*domain.User)

	reviewID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, "INVALID_ID", "Invalid review ID")
		return
	}

	span.SetAttributes(
		attribute.String("review.id", reviewID.String()),
		attribute.String("user.id", userObj.ID.String()),
	)

	// Get existing review to check ownership
	existingReview, err := h.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.reviewService.GetReviewByID(ctx, reviewID)
	})
	if err != nil {
		h.handleError(c, err, "Failed to get existing review")
		return
	}

	existingReviewObj := existingReview.(*domain.Review)

	// Check ownership (unless user is admin)
	if existingReviewObj.ProviderID != userObj.ID {
		adminCheck, err := h.rbacService.CheckPermission(ctx, userObj.ID, "reviews", "admin")
		if err != nil || !adminCheck {
			h.respondWithError(c, http.StatusForbidden, "FORBIDDEN", "Can only delete own reviews")
			return
		}
	}

	// Delete review with resilience
	_, err = h.executeWithResilience(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return nil, h.reviewService.DeleteReview(ctx, reviewID)
	})
	if err != nil {
		h.handleError(c, err, "Failed to delete review")
		return
	}

	// Publish delete event
	go h.publishReviewEvent(context.Background(), "review.deleted", existingReviewObj)

	h.respondWithSuccess(c, http.StatusOK, "Review deleted successfully", nil)
}

// HealthCheck provides comprehensive health check
func (h *SimplifiedIntegratedHandlers) HealthCheck(c *gin.Context) {
	_, span := h.tracer.Start(c.Request.Context(), "HealthCheck")
	defer span.End()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
		"services":  make(map[string]interface{}),
	}

	// Check circuit breakers
	if h.circuitBreaker != nil {
		health["services"].(map[string]interface{})["circuit_breakers"] = h.circuitBreaker.GetHealthStatus()
	}

	h.respondWithSuccess(c, http.StatusOK, "Health check completed", health)
}

// GetMetrics returns application metrics
func (h *SimplifiedIntegratedHandlers) GetMetrics(c *gin.Context) {
	_, span := h.tracer.Start(c.Request.Context(), "GetMetrics")
	defer span.End()

	metrics := map[string]interface{}{
		"timestamp": time.Now().UTC(),
	}

	if h.circuitBreaker != nil {
		metrics["circuit_breakers"] = h.circuitBreaker.GetHealthStatus()
	}

	if h.retryManager != nil {
		metrics["retry"] = h.retryManager.GetMetrics()
	}

	if h.processingEngine != nil {
		metrics["processing"] = h.processingEngine.GetMetrics()
	}

	h.respondWithSuccess(c, http.StatusOK, "Metrics retrieved successfully", metrics)
}

// Helper methods

// executeWithResilience executes a function with circuit breaker and retry protection
func (h *SimplifiedIntegratedHandlers) executeWithResilience(
	ctx context.Context,
	serviceName string,
	operation func(ctx context.Context) (interface{}, error),
) (interface{}, error) {
	ctx, span := h.tracer.Start(ctx, fmt.Sprintf("ExecuteWithResilience-%s", serviceName))
	defer span.End()

	span.SetAttributes(attribute.String("service.name", serviceName))

	// Try circuit breaker first
	if h.circuitBreaker != nil {
		breaker, exists := h.circuitBreaker.GetManager().GetCircuitBreaker(serviceName)
		if exists {
			result, err := breaker.Execute(ctx, operation)
			if err != nil {
				span.RecordError(err)
			}
			return result, err
		}
	}

	// Direct execution as fallback
	return operation(ctx)
}

// createReviewInternal creates a review internally
func (h *SimplifiedIntegratedHandlers) createReviewInternal(ctx context.Context, req *CreateReviewRequest, providerID uuid.UUID) (*domain.Review, error) {
	hotelID, err := uuid.Parse(req.HotelID)
	if err != nil {
		return nil, fmt.Errorf("invalid hotel ID: %w", err)
	}

	reviewerInfoID, err := uuid.Parse(req.ReviewerInfoID)
	if err != nil {
		return nil, fmt.Errorf("invalid reviewer info ID: %w", err)
	}

	review := &domain.Review{
		ID:             uuid.New(),
		ProviderID:     providerID,
		HotelID:        hotelID,
		ReviewerInfoID: &reviewerInfoID,
		Rating:         req.Rating,
		Title:          req.Title,
		Comment:        req.Comment,
		ReviewDate:     req.ReviewDate,
		Language:       req.Language,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Set ExternalID if provided
	if req.ExternalID != nil {
		review.ExternalID = *req.ExternalID
	}

	err = h.reviewService.CreateReview(ctx, review)
	return review, err
}

// updateReviewInternal updates a review internally
func (h *SimplifiedIntegratedHandlers) updateReviewInternal(ctx context.Context, review *domain.Review, req *UpdateReviewRequest) (*domain.Review, error) {
	// Update fields
	if req.Rating != nil {
		review.Rating = *req.Rating
	}
	if req.Title != nil {
		review.Title = *req.Title
	}
	if req.Comment != nil {
		review.Comment = *req.Comment
	}
	if req.Language != nil {
		review.Language = *req.Language
	}

	review.UpdatedAt = time.Now()

	err := h.reviewService.UpdateReview(ctx, review)
	return review, err
}

// publishReviewEvent publishes review events to Kafka
func (h *SimplifiedIntegratedHandlers) publishReviewEvent(ctx context.Context, eventType string, review *domain.Review) {
	if h.kafkaProducer == nil {
		return
	}

	// Create a ReviewEvent using the proper Kafka event structure
	reviewEvent := &infrastructure.ReviewEvent{
		BaseEvent: infrastructure.BaseEvent{
			ID:            uuid.New().String(),
			Type:          infrastructure.EventType(eventType),
			Timestamp:     time.Now().UTC(),
			Source:        "hotel-reviews-service",
			CorrelationID: review.ID.String(),
		},
		ReviewID:   review.ID,
		HotelID:    review.HotelID,
		ProviderID: review.ProviderID,
		Rating:     review.Rating,
		Language:   review.Language,
	}

	err := h.kafkaProducer.PublishReviewEvent(ctx, reviewEvent)
	if err != nil {
		h.logger.Error("Failed to publish review event", "error", err, "event_type", eventType, "review_id", review.ID)
	}
}

// Response helpers

// respondWithSuccess sends a successful API response
func (h *SimplifiedIntegratedHandlers) respondWithSuccess(c *gin.Context, statusCode int, message string, data interface{}) {
	traceID := ""
	if span := trace.SpanFromContext(c.Request.Context()); span != nil {
		traceID = span.SpanContext().TraceID().String()
	}

	response := StandardAPIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC(),
		TraceID:   traceID,
	}

	c.JSON(statusCode, response)
}

// respondWithError sends an error API response
func (h *SimplifiedIntegratedHandlers) respondWithError(c *gin.Context, statusCode int, errorCode, message string) {
	traceID := ""
	if span := trace.SpanFromContext(c.Request.Context()); span != nil {
		traceID = span.SpanContext().TraceID().String()
	}

	response := StandardAPIResponse{
		Success:   false,
		Error:     message,
		ErrorCode: errorCode,
		Timestamp: time.Now().UTC(),
		TraceID:   traceID,
	}

	c.JSON(statusCode, response)

	// Log the error
	h.logger.Error("API Error",
		"status_code", statusCode,
		"error_code", errorCode,
		"message", message,
		"trace_id", traceID,
	)
}

// handleError handles errors with proper logging and response
func (h *SimplifiedIntegratedHandlers) handleError(c *gin.Context, err error, message string) {
	var statusCode int
	var errorCode string

	// Use error handler if available
	if h.errorHandler != nil {
		appError := h.errorHandler.Handle(c.Request.Context(), err)
		if appError != nil {
			switch appError.Type {
			case "NotFound":
				statusCode = http.StatusNotFound
				errorCode = "NOT_FOUND"
			case "Validation":
				statusCode = http.StatusBadRequest
				errorCode = "VALIDATION_ERROR"
			case "Authorization":
				statusCode = http.StatusForbidden
				errorCode = "FORBIDDEN"
			case "Authentication":
				statusCode = http.StatusUnauthorized
				errorCode = "UNAUTHORIZED"
			case "RateLimit":
				statusCode = http.StatusTooManyRequests
				errorCode = "RATE_LIMITED"
			case "CircuitBreaker":
				statusCode = http.StatusServiceUnavailable
				errorCode = "SERVICE_UNAVAILABLE"
			default:
				statusCode = http.StatusInternalServerError
				errorCode = "INTERNAL_ERROR"
			}
		} else {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}
	} else {
		// Fallback error handling
		statusCode = http.StatusInternalServerError
		errorCode = "INTERNAL_ERROR"
	}

	h.respondWithError(c, statusCode, errorCode, message)
}
