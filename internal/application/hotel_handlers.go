package application

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/google/uuid"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

type HotelHandlers struct {
	hotelService domain.HotelService
	logger       *logger.Logger
}

func NewHotelHandlers(hotelService domain.HotelService, logger *logger.Logger) *HotelHandlers {
	return &HotelHandlers{
		hotelService: hotelService,
		logger:       logger,
	}
}

// GetHotels handles GET /api/v1/hotels
func (h *HotelHandlers) GetHotels(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	city := c.Query("city")
	country := c.Query("country")
	minRating, _ := strconv.Atoi(c.Query("min_rating"))
	maxRating, _ := strconv.Atoi(c.Query("max_rating"))

	// Validate pagination parameters
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// Create filter
	filter := domain.HotelFilter{
		City:      city,
		Country:   country,
		MinRating: minRating,
		MaxRating: maxRating,
		Limit:     limit,
		Offset:    offset,
	}

	// Get hotels from service
	hotels, err := h.hotelService.GetHotels(c.Request.Context(), filter)
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "Failed to get hotels", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve hotels",
		})
		return
	}

	// Return response
	c.JSON(http.StatusOK, gin.H{
		"hotels": hotels,
		"pagination": gin.H{
			"limit":  limit,
			"offset": offset,
			"total":  len(hotels), // In a real implementation, this would be a separate count query
		},
	})
}

// GetHotel handles GET /api/v1/hotels/:id
func (h *HotelHandlers) GetHotel(c *gin.Context) {
	// Parse hotel ID
	hotelIDStr := c.Param("id")
	hotelID, err := uuid.Parse(hotelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid hotel ID format",
		})
		return
	}

	// Get hotel from service
	hotel, err := h.hotelService.GetHotel(c.Request.Context(), hotelID)
	if err != nil {
		if err == domain.ErrHotelNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "HOTEL_NOT_FOUND",
				"message": "Hotel not found",
			})
			return
		}

		h.logger.ErrorContext(c.Request.Context(), "Failed to get hotel", "error", err, "hotel_id", hotelID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve hotel",
		})
		return
	}

	c.JSON(http.StatusOK, hotel)
}

// CreateHotel handles POST /api/v1/hotels
func (h *HotelHandlers) CreateHotel(c *gin.Context) {
	var hotel domain.Hotel
	if err := c.ShouldBindJSON(&hotel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid hotel data",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if err := h.validateHotelInput(&hotel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Generate ID if not provided
	if hotel.ID == uuid.Nil {
		hotel.ID = uuid.New()
	}

	// Create hotel
	if err := h.hotelService.CreateHotel(c.Request.Context(), &hotel); err != nil {
		if err == domain.ErrHotelAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "HOTEL_ALREADY_EXISTS",
				"message": "Hotel already exists",
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

		h.logger.ErrorContext(c.Request.Context(), "Failed to create hotel", "error", err, "hotel", hotel)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to create hotel",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Hotel created successfully",
		"hotel":   hotel,
	})
}

// UpdateHotel handles PUT /api/v1/hotels/:id
func (h *HotelHandlers) UpdateHotel(c *gin.Context) {
	// Parse hotel ID
	hotelIDStr := c.Param("id")
	hotelID, err := uuid.Parse(hotelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid hotel ID format",
		})
		return
	}

	var hotel domain.Hotel
	if err := c.ShouldBindJSON(&hotel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid hotel data",
			"details": err.Error(),
		})
		return
	}

	// Set the ID from URL
	hotel.ID = hotelID

	// Validate input
	if err := h.validateHotelInput(&hotel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	// Update hotel
	if err := h.hotelService.UpdateHotel(c.Request.Context(), &hotel); err != nil {
		if err == domain.ErrHotelNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "HOTEL_NOT_FOUND",
				"message": "Hotel not found",
			})
			return
		}

		h.logger.ErrorContext(c.Request.Context(), "Failed to update hotel", "error", err, "hotel_id", hotelID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to update hotel",
		})
		return
	}

	c.JSON(http.StatusOK, hotel)
}

// DeleteHotel handles DELETE /api/v1/hotels/:id
func (h *HotelHandlers) DeleteHotel(c *gin.Context) {
	// Parse hotel ID
	hotelIDStr := c.Param("id")
	hotelID, err := uuid.Parse(hotelIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid hotel ID format",
		})
		return
	}

	// Delete hotel
	if err := h.hotelService.DeleteHotel(c.Request.Context(), hotelID); err != nil {
		if err == domain.ErrHotelNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "HOTEL_NOT_FOUND",
				"message": "Hotel not found",
			})
			return
		}

		h.logger.ErrorContext(c.Request.Context(), "Failed to delete hotel", "error", err, "hotel_id", hotelID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to delete hotel",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// validateHotelInput validates hotel input data
func (h *HotelHandlers) validateHotelInput(hotel *domain.Hotel) error {
	if hotel.Name == "" {
		return domain.NewValidationError("name", "Hotel name is required")
	}
	if len(hotel.Name) < 2 {
		return domain.NewValidationError("name", "Hotel name must be at least 2 characters")
	}
	if len(hotel.Name) > 100 {
		return domain.NewValidationError("name", "Hotel name must be less than 100 characters")
	}

	if hotel.Address == "" {
		return domain.NewValidationError("address", "Hotel address is required")
	}
	if len(hotel.Address) > 255 {
		return domain.NewValidationError("address", "Hotel address must be less than 255 characters")
	}

	if hotel.City == "" {
		return domain.NewValidationError("city", "Hotel city is required")
	}
	if len(hotel.City) > 100 {
		return domain.NewValidationError("city", "Hotel city must be less than 100 characters")
	}

	if hotel.Country == "" {
		return domain.NewValidationError("country", "Hotel country is required")
	}
	if len(hotel.Country) > 100 {
		return domain.NewValidationError("country", "Hotel country must be less than 100 characters")
	}

	if hotel.StarRating < 0 || hotel.StarRating > 5 {
		return domain.NewValidationError("star_rating", "Star rating must be between 0 and 5")
	}

	if hotel.Email != "" && !isValidEmail(hotel.Email) {
		return domain.NewValidationError("email", "Invalid email format")
	}

	if len(hotel.Description) > 1000 {
		return domain.NewValidationError("description", "Description must be less than 1000 characters")
	}

	// Validate coordinates if provided
	if hotel.Latitude != 0 || hotel.Longitude != 0 {
		if hotel.Latitude < -90 || hotel.Latitude > 90 {
			return domain.NewValidationError("latitude", "Latitude must be between -90 and 90")
		}
		if hotel.Longitude < -180 || hotel.Longitude > 180 {
			return domain.NewValidationError("longitude", "Longitude must be between -180 and 180")
		}
	}

	return nil
}

// Helper function to validate email format
func isValidEmail(email string) bool {
	// Basic email validation - in production, use a proper email validation library
	return len(email) > 0 && 
		   len(email) <= 254 && 
		   containsAt(email) && 
		   containsDot(email)
}

func containsAt(s string) bool {
	for _, c := range s {
		if c == '@' {
			return true
		}
	}
	return false
}

func containsDot(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}