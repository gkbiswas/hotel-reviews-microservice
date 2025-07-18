package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Handlers contains all HTTP handlers for the review system
type Handlers struct {
	reviewService domain.ReviewService
	logger        *logger.Logger
}

// NewHandlers creates a new Handlers instance
func NewHandlers(reviewService domain.ReviewService, logger *logger.Logger) *Handlers {
	return &Handlers{
		reviewService: reviewService,
		logger:        logger,
	}
}

// Response structures
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Total  int64 `json:"total,omitempty"`
	Limit  int   `json:"limit,omitempty"`
	Offset int   `json:"offset,omitempty"`
	Page   int   `json:"page,omitempty"`
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// Request structures
type ProcessFileRequest struct {
	FileURL    string `json:"file_url" validate:"required,url"`
	ProviderID string `json:"provider_id" validate:"required,uuid"`
}

type CreateReviewRequest struct {
	ProviderID        string                 `json:"provider_id" validate:"required,uuid"`
	HotelID           string                 `json:"hotel_id" validate:"required,uuid"`
	ReviewerInfoID    string                 `json:"reviewer_info_id" validate:"required,uuid"`
	ExternalID        string                 `json:"external_id"`
	Rating            float64                `json:"rating" validate:"required,min=1,max=5"`
	Title             string                 `json:"title" validate:"max=500"`
	Comment           string                 `json:"comment" validate:"required,max=10000"`
	ReviewDate        time.Time              `json:"review_date" validate:"required"`
	StayDate          *time.Time             `json:"stay_date,omitempty"`
	TripType          string                 `json:"trip_type" validate:"max=50"`
	RoomType          string                 `json:"room_type" validate:"max=100"`
	Language          string                 `json:"language" validate:"max=10"`
	ServiceRating     *float64               `json:"service_rating,omitempty" validate:"omitempty,min=1,max=5"`
	CleanlinessRating *float64               `json:"cleanliness_rating,omitempty" validate:"omitempty,min=1,max=5"`
	LocationRating    *float64               `json:"location_rating,omitempty" validate:"omitempty,min=1,max=5"`
	ValueRating       *float64               `json:"value_rating,omitempty" validate:"omitempty,min=1,max=5"`
	ComfortRating     *float64               `json:"comfort_rating,omitempty" validate:"omitempty,min=1,max=5"`
	FacilitiesRating  *float64               `json:"facilities_rating,omitempty" validate:"omitempty,min=1,max=5"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type CreateHotelRequest struct {
	Name        string   `json:"name" validate:"required,min=1,max=255"`
	Address     string   `json:"address"`
	City        string   `json:"city" validate:"max=100"`
	Country     string   `json:"country" validate:"max=100"`
	PostalCode  string   `json:"postal_code" validate:"max=20"`
	Phone       string   `json:"phone" validate:"max=20"`
	Email       string   `json:"email" validate:"email,max=255"`
	StarRating  int      `json:"star_rating" validate:"min=1,max=5"`
	Description string   `json:"description"`
	Amenities   []string `json:"amenities"`
	Latitude    float64  `json:"latitude" validate:"min=-90,max=90"`
	Longitude   float64  `json:"longitude" validate:"min=-180,max=180"`
}

type CreateProviderRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=100"`
	BaseURL  string `json:"base_url" validate:"url"`
	IsActive bool   `json:"is_active"`
}

// Health check handler
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Check service health
	services := make(map[string]string)
	services["api"] = "healthy"
	
	// TODO: Add database health check
	// TODO: Add S3 health check
	// TODO: Add cache health check
	
	response := HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Timestamp: time.Now(),
		Services:  services,
	}
	
	h.logger.DebugContext(ctx, "Health check requested")
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}

// File processing handlers
func (h *Handlers) ProcessFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req ProcessFileRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	
	// Validate request
	if err := h.validateRequest(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Validation failed", err)
		return
	}
	
	// Parse provider ID
	providerID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid provider ID", err)
		return
	}
	
	// Start file processing
	processingStatus, err := h.reviewService.ProcessReviewFile(ctx, req.FileURL, providerID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to start file processing",
			"file_url", req.FileURL,
			"provider_id", providerID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to start file processing", err)
		return
	}
	
	h.logger.InfoContext(ctx, "File processing started",
		"processing_id", processingStatus.ID,
		"file_url", req.FileURL,
		"provider_id", providerID,
	)
	
	h.writeJSONResponse(w, http.StatusAccepted, APIResponse{
		Success: true,
		Data:    processingStatus,
	})
}

func (h *Handlers) GetProcessingStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get processing ID from URL
	vars := mux.Vars(r)
	processingID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid processing ID", err)
		return
	}
	
	// Get processing status
	status, err := h.reviewService.GetProcessingStatus(ctx, processingID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get processing status",
			"processing_id", processingID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusNotFound, "Processing status not found", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

func (h *Handlers) GetProcessingHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get provider ID from URL
	vars := mux.Vars(r)
	providerID, err := uuid.Parse(vars["provider_id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid provider ID", err)
		return
	}
	
	// Get pagination parameters
	limit, offset := h.getPaginationParams(r)
	
	// Get processing history
	history, err := h.reviewService.GetProcessingHistory(ctx, providerID, limit, offset)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get processing history",
			"provider_id", providerID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get processing history", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    history,
		Meta: &Meta{
			Limit:  limit,
			Offset: offset,
		},
	})
}

// Review handlers
func (h *Handlers) CreateReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req CreateReviewRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	
	// Validate request
	if err := h.validateRequest(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Validation failed", err)
		return
	}
	
	// Convert request to domain model
	review, err := h.convertToReview(&req)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request data", err)
		return
	}
	
	// Create review
	if err := h.reviewService.CreateReview(ctx, review); err != nil {
		h.logger.ErrorContext(ctx, "Failed to create review",
			"review_id", review.ID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create review", err)
		return
	}
	
	h.logger.InfoContext(ctx, "Review created successfully",
		"review_id", review.ID,
		"hotel_id", review.HotelID,
		"provider_id", review.ProviderID,
	)
	
	h.writeJSONResponse(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    review,
	})
}

func (h *Handlers) GetReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get review ID from URL
	vars := mux.Vars(r)
	reviewID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid review ID", err)
		return
	}
	
	// Get review
	review, err := h.reviewService.GetReviewByID(ctx, reviewID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get review",
			"review_id", reviewID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusNotFound, "Review not found", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    review,
	})
}

func (h *Handlers) GetReviewsByHotel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get hotel ID from URL
	vars := mux.Vars(r)
	hotelID, err := uuid.Parse(vars["hotel_id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid hotel ID", err)
		return
	}
	
	// Get pagination parameters
	limit, offset := h.getPaginationParams(r)
	
	// Get reviews
	reviews, err := h.reviewService.GetReviewsByHotel(ctx, hotelID, limit, offset)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get reviews by hotel",
			"hotel_id", hotelID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get reviews", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    reviews,
		Meta: &Meta{
			Limit:  limit,
			Offset: offset,
		},
	})
}

func (h *Handlers) GetReviewsByProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get provider ID from URL
	vars := mux.Vars(r)
	providerID, err := uuid.Parse(vars["provider_id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid provider ID", err)
		return
	}
	
	// Get pagination parameters
	limit, offset := h.getPaginationParams(r)
	
	// Get reviews
	reviews, err := h.reviewService.GetReviewsByProvider(ctx, providerID, limit, offset)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get reviews by provider",
			"provider_id", providerID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get reviews", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    reviews,
		Meta: &Meta{
			Limit:  limit,
			Offset: offset,
		},
	})
}

func (h *Handlers) SearchReviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get search parameters
	query := r.URL.Query().Get("q")
	filters := h.buildFilters(r)
	limit, offset := h.getPaginationParams(r)
	
	// Search reviews
	reviews, err := h.reviewService.SearchReviews(ctx, query, filters, limit, offset)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to search reviews",
			"query", query,
			"filters", filters,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to search reviews", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    reviews,
		Meta: &Meta{
			Limit:  limit,
			Offset: offset,
		},
	})
}

// Hotel handlers
func (h *Handlers) CreateHotel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req CreateHotelRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	
	// Validate request
	if err := h.validateRequest(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Validation failed", err)
		return
	}
	
	// Convert request to domain model
	hotel := &domain.Hotel{
		ID:          uuid.New(),
		Name:        req.Name,
		Address:     req.Address,
		City:        req.City,
		Country:     req.Country,
		PostalCode:  req.PostalCode,
		Phone:       req.Phone,
		Email:       req.Email,
		StarRating:  req.StarRating,
		Description: req.Description,
		Amenities:   req.Amenities,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
	}
	
	// Create hotel
	if err := h.reviewService.CreateHotel(ctx, hotel); err != nil {
		h.logger.ErrorContext(ctx, "Failed to create hotel",
			"hotel_id", hotel.ID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create hotel", err)
		return
	}
	
	h.logger.InfoContext(ctx, "Hotel created successfully",
		"hotel_id", hotel.ID,
		"name", hotel.Name,
	)
	
	h.writeJSONResponse(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    hotel,
	})
}

func (h *Handlers) GetHotel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get hotel ID from URL
	vars := mux.Vars(r)
	hotelID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid hotel ID", err)
		return
	}
	
	// Get hotel
	hotel, err := h.reviewService.GetHotelByID(ctx, hotelID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get hotel",
			"hotel_id", hotelID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusNotFound, "Hotel not found", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    hotel,
	})
}

func (h *Handlers) ListHotels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get pagination parameters
	limit, offset := h.getPaginationParams(r)
	
	// List hotels
	hotels, err := h.reviewService.ListHotels(ctx, limit, offset)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to list hotels", "error", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to list hotels", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    hotels,
		Meta: &Meta{
			Limit:  limit,
			Offset: offset,
		},
	})
}

// Provider handlers
func (h *Handlers) CreateProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req CreateProviderRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	
	// Validate request
	if err := h.validateRequest(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Validation failed", err)
		return
	}
	
	// Convert request to domain model
	provider := &domain.Provider{
		ID:       uuid.New(),
		Name:     req.Name,
		BaseURL:  req.BaseURL,
		IsActive: req.IsActive,
	}
	
	// Create provider
	if err := h.reviewService.CreateProvider(ctx, provider); err != nil {
		h.logger.ErrorContext(ctx, "Failed to create provider",
			"provider_id", provider.ID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create provider", err)
		return
	}
	
	h.logger.InfoContext(ctx, "Provider created successfully",
		"provider_id", provider.ID,
		"name", provider.Name,
	)
	
	h.writeJSONResponse(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    provider,
	})
}

func (h *Handlers) GetProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get provider ID from URL
	vars := mux.Vars(r)
	providerID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid provider ID", err)
		return
	}
	
	// Get provider
	provider, err := h.reviewService.GetProviderByID(ctx, providerID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get provider",
			"provider_id", providerID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusNotFound, "Provider not found", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    provider,
	})
}

func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get pagination parameters
	limit, offset := h.getPaginationParams(r)
	
	// List providers
	providers, err := h.reviewService.ListProviders(ctx, limit, offset)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to list providers", "error", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to list providers", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    providers,
		Meta: &Meta{
			Limit:  limit,
			Offset: offset,
		},
	})
}

// Analytics handlers
func (h *Handlers) GetReviewSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get hotel ID from URL
	vars := mux.Vars(r)
	hotelID, err := uuid.Parse(vars["hotel_id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid hotel ID", err)
		return
	}
	
	// Get review summary
	summary, err := h.reviewService.GetReviewSummary(ctx, hotelID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get review summary",
			"hotel_id", hotelID,
			"error", err,
		)
		h.writeErrorResponse(w, http.StatusNotFound, "Review summary not found", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    summary,
	})
}

func (h *Handlers) GetTopRatedHotels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get limit parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	
	// Get top rated hotels
	hotels, err := h.reviewService.GetTopRatedHotels(ctx, limit)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get top rated hotels", "error", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get top rated hotels", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    hotels,
		Meta: &Meta{
			Limit: limit,
		},
	})
}

func (h *Handlers) GetRecentReviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get limit parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	
	// Get recent reviews
	reviews, err := h.reviewService.GetRecentReviews(ctx, limit)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get recent reviews", "error", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get recent reviews", err)
		return
	}
	
	h.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    reviews,
		Meta: &Meta{
			Limit: limit,
		},
	})
}

// Helper methods
func (h *Handlers) parseJSONRequest(r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (h *Handlers) validateRequest(req interface{}) error {
	// TODO: Implement request validation using a validation library
	// For now, return nil (basic validation should be implemented)
	return nil
}

func (h *Handlers) writeJSONResponse(w http.ResponseWriter, statusCode int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	
	if err != nil {
		h.logger.Error("API error", "message", message, "error", err)
	}
	
	h.writeJSONResponse(w, statusCode, response)
}

func (h *Handlers) getPaginationParams(r *http.Request) (limit, offset int) {
	limit = 20 // default limit
	offset = 0 // default offset
	
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}
	
	// Support page-based pagination
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			offset = (p - 1) * limit
		}
	}
	
	return limit, offset
}

func (h *Handlers) buildFilters(r *http.Request) map[string]interface{} {
	filters := make(map[string]interface{})
	
	if rating := r.URL.Query().Get("rating"); rating != "" {
		if r, err := strconv.ParseFloat(rating, 64); err == nil {
			filters["rating"] = r
		}
	}
	
	if minRating := r.URL.Query().Get("min_rating"); minRating != "" {
		if r, err := strconv.ParseFloat(minRating, 64); err == nil {
			filters["min_rating"] = r
		}
	}
	
	if maxRating := r.URL.Query().Get("max_rating"); maxRating != "" {
		if r, err := strconv.ParseFloat(maxRating, 64); err == nil {
			filters["max_rating"] = r
		}
	}
	
	if providerID := r.URL.Query().Get("provider_id"); providerID != "" {
		if id, err := uuid.Parse(providerID); err == nil {
			filters["provider_id"] = id
		}
	}
	
	if hotelID := r.URL.Query().Get("hotel_id"); hotelID != "" {
		if id, err := uuid.Parse(hotelID); err == nil {
			filters["hotel_id"] = id
		}
	}
	
	if language := r.URL.Query().Get("language"); language != "" {
		filters["language"] = language
	}
	
	if sentiment := r.URL.Query().Get("sentiment"); sentiment != "" {
		filters["sentiment"] = sentiment
	}
	
	if isVerified := r.URL.Query().Get("is_verified"); isVerified != "" {
		if verified, err := strconv.ParseBool(isVerified); err == nil {
			filters["is_verified"] = verified
		}
	}
	
	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if date, err := time.Parse("2006-01-02", startDate); err == nil {
			filters["start_date"] = date
		}
	}
	
	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if date, err := time.Parse("2006-01-02", endDate); err == nil {
			filters["end_date"] = date
		}
	}
	
	return filters
}

func (h *Handlers) convertToReview(req *CreateReviewRequest) (*domain.Review, error) {
	providerID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("invalid provider ID: %w", err)
	}
	
	hotelID, err := uuid.Parse(req.HotelID)
	if err != nil {
		return nil, fmt.Errorf("invalid hotel ID: %w", err)
	}
	
	reviewerInfoID, err := uuid.Parse(req.ReviewerInfoID)
	if err != nil {
		return nil, fmt.Errorf("invalid reviewer info ID: %w", err)
	}
	
	review := &domain.Review{
		ID:                uuid.New(),
		ProviderID:        providerID,
		HotelID:           hotelID,
		ReviewerInfoID:    reviewerInfoID,
		ExternalID:        req.ExternalID,
		Rating:            req.Rating,
		Title:             req.Title,
		Comment:           req.Comment,
		ReviewDate:        req.ReviewDate,
		StayDate:          req.StayDate,
		TripType:          req.TripType,
		RoomType:          req.RoomType,
		Language:          req.Language,
		ServiceRating:     req.ServiceRating,
		CleanlinessRating: req.CleanlinessRating,
		LocationRating:    req.LocationRating,
		ValueRating:       req.ValueRating,
		ComfortRating:     req.ComfortRating,
		FacilitiesRating:  req.FacilitiesRating,
		Metadata:          req.Metadata,
	}
	
	return review, nil
}

// SetupRoutes sets up all HTTP routes
func (h *Handlers) SetupRoutes(router *mux.Router) {
	// Health check
	router.HandleFunc("/health", h.HealthCheck).Methods("GET")
	
	// File processing
	router.HandleFunc("/process", h.ProcessFile).Methods("POST")
	router.HandleFunc("/processing/{id}", h.GetProcessingStatus).Methods("GET")
	router.HandleFunc("/processing/history/{provider_id}", h.GetProcessingHistory).Methods("GET")
	
	// Reviews
	router.HandleFunc("/reviews", h.CreateReview).Methods("POST")
	router.HandleFunc("/reviews/{id}", h.GetReview).Methods("GET")
	router.HandleFunc("/reviews", h.SearchReviews).Methods("GET")
	router.HandleFunc("/hotels/{hotel_id}/reviews", h.GetReviewsByHotel).Methods("GET")
	router.HandleFunc("/providers/{provider_id}/reviews", h.GetReviewsByProvider).Methods("GET")
	
	// Hotels
	router.HandleFunc("/hotels", h.CreateHotel).Methods("POST")
	router.HandleFunc("/hotels/{id}", h.GetHotel).Methods("GET")
	router.HandleFunc("/hotels", h.ListHotels).Methods("GET")
	
	// Providers
	router.HandleFunc("/providers", h.CreateProvider).Methods("POST")
	router.HandleFunc("/providers/{id}", h.GetProvider).Methods("GET")
	router.HandleFunc("/providers", h.ListProviders).Methods("GET")
	
	// Analytics
	router.HandleFunc("/hotels/{hotel_id}/summary", h.GetReviewSummary).Methods("GET")
	router.HandleFunc("/analytics/top-hotels", h.GetTopRatedHotels).Methods("GET")
	router.HandleFunc("/analytics/recent-reviews", h.GetRecentReviews).Methods("GET")
}

// Middleware
func (h *Handlers) LoggingMiddleware(next http.Handler) http.Handler {
	return logger.RequestLoggerMiddleware(h.logger)(next)
}

func (h *Handlers) RecoveryMiddleware(next http.Handler) http.Handler {
	return logger.RecoveryMiddleware(h.logger)(next)
}

func (h *Handlers) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Correlation-ID")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) ContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" {
			contentType := r.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, "application/json") {
				h.writeErrorResponse(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json", nil)
				return
			}
		}
		
		next.ServeHTTP(w, r)
	})
}