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

type ProviderHandlers struct {
	providerService domain.ProviderService
	logger          *logger.Logger
}

func NewProviderHandlers(providerService domain.ProviderService, logger *logger.Logger) *ProviderHandlers {
	return &ProviderHandlers{
		providerService: providerService,
		logger:          logger,
	}
}

// GetProviders handles GET /api/v1/providers
func (p *ProviderHandlers) GetProviders(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	isActive := c.Query("is_active")
	name := c.Query("name")

	// Validate pagination parameters
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// Create filter
	filter := domain.ProviderFilter{
		Name:   name,
		Limit:  limit,
		Offset: offset,
	}

	// Parse is_active parameter
	if isActive != "" {
		if isActive == "true" {
			filter.IsActive = &[]bool{true}[0]
		} else if isActive == "false" {
			filter.IsActive = &[]bool{false}[0]
		}
	}

	// Get providers from service
	providers, err := p.providerService.GetProviders(c.Request.Context(), filter)
	if err != nil {
		p.logger.ErrorContext(c.Request.Context(), "Failed to get providers", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve providers",
		})
		return
	}

	// Return response
	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
		"pagination": gin.H{
			"limit":  limit,
			"offset": offset,
			"total":  len(providers), // In a real implementation, this would be a separate count query
		},
	})
}

// GetProvider handles GET /api/v1/providers/:id
func (p *ProviderHandlers) GetProvider(c *gin.Context) {
	// Parse provider ID
	providerIDStr := c.Param("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid provider ID format",
		})
		return
	}

	// Get provider from service
	provider, err := p.providerService.GetProvider(c.Request.Context(), providerID)
	if err != nil {
		if err == domain.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "PROVIDER_NOT_FOUND",
				"message": "Provider not found",
			})
			return
		}

		p.logger.ErrorContext(c.Request.Context(), "Failed to get provider", "error", err, "provider_id", providerID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve provider",
		})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// CreateProvider handles POST /api/v1/providers
func (p *ProviderHandlers) CreateProvider(c *gin.Context) {
	var provider domain.Provider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid provider data",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if err := p.validateProviderInput(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Generate ID if not provided
	if provider.ID == uuid.Nil {
		provider.ID = uuid.New()
	}

	// Set default values
	if provider.IsActive == false {
		provider.IsActive = true // Default to active
	}

	// Create provider
	if err := p.providerService.CreateProvider(c.Request.Context(), &provider); err != nil {
		if err == domain.ErrProviderAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "PROVIDER_ALREADY_EXISTS",
				"message": "Provider already exists",
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

		p.logger.ErrorContext(c.Request.Context(), "Failed to create provider", "error", err, "provider", provider)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to create provider",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Provider created successfully",
		"provider": provider,
	})
}

// UpdateProvider handles PUT /api/v1/providers/:id
func (p *ProviderHandlers) UpdateProvider(c *gin.Context) {
	// Parse provider ID
	providerIDStr := c.Param("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid provider ID format",
		})
		return
	}

	var provider domain.Provider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "Invalid provider data",
			"details": err.Error(),
		})
		return
	}

	// Set the ID from URL
	provider.ID = providerID

	// Validate input
	if err := p.validateProviderInput(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	// Update provider
	if err := p.providerService.UpdateProvider(c.Request.Context(), &provider); err != nil {
		if err == domain.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "PROVIDER_NOT_FOUND",
				"message": "Provider not found",
			})
			return
		}

		p.logger.ErrorContext(c.Request.Context(), "Failed to update provider", "error", err, "provider_id", providerID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to update provider",
		})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// DeleteProvider handles DELETE /api/v1/providers/:id
func (p *ProviderHandlers) DeleteProvider(c *gin.Context) {
	// Parse provider ID
	providerIDStr := c.Param("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid provider ID format",
		})
		return
	}

	// Delete provider
	if err := p.providerService.DeleteProvider(c.Request.Context(), providerID); err != nil {
		if err == domain.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "PROVIDER_NOT_FOUND",
				"message": "Provider not found",
			})
			return
		}

		p.logger.ErrorContext(c.Request.Context(), "Failed to delete provider", "error", err, "provider_id", providerID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to delete provider",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// ToggleProviderStatus handles PATCH /api/v1/providers/:id/toggle
func (p *ProviderHandlers) ToggleProviderStatus(c *gin.Context) {
	// Parse provider ID
	providerIDStr := c.Param("id")
	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid provider ID format",
		})
		return
	}

	// Get current provider
	provider, err := p.providerService.GetProvider(c.Request.Context(), providerID)
	if err != nil {
		if err == domain.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "PROVIDER_NOT_FOUND",
				"message": "Provider not found",
			})
			return
		}

		p.logger.ErrorContext(c.Request.Context(), "Failed to get provider for toggle", "error", err, "provider_id", providerID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve provider",
		})
		return
	}

	// Toggle status
	provider.IsActive = !provider.IsActive

	// Update provider
	if err := p.providerService.UpdateProvider(c.Request.Context(), provider); err != nil {
		p.logger.ErrorContext(c.Request.Context(), "Failed to toggle provider status", "error", err, "provider_id", providerID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to update provider status",
		})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// validateProviderInput validates provider input data
func (p *ProviderHandlers) validateProviderInput(provider *domain.Provider) error {
	if provider.Name == "" {
		return domain.NewValidationError("name", "Provider name is required")
	}
	if len(provider.Name) < 2 {
		return domain.NewValidationError("name", "Provider name must be at least 2 characters")
	}
	if len(provider.Name) > 100 {
		return domain.NewValidationError("name", "Provider name must be less than 100 characters")
	}

	if provider.BaseURL != "" {
		if len(provider.BaseURL) > 255 {
			return domain.NewValidationError("base_url", "Base URL must be less than 255 characters")
		}
		if !isValidURL(provider.BaseURL) {
			return domain.NewValidationError("base_url", "Invalid URL format")
		}
	}

	return nil
}

// Helper function to validate URL format
func isValidURL(url string) bool {
	// Basic URL validation - in production, use a proper URL validation library
	return len(url) > 0 && 
		   (containsHTTP(url) || containsHTTPS(url))
}

func containsHTTP(s string) bool {
	return len(s) >= 7 && s[:7] == "http://"
}

func containsHTTPS(s string) bool {
	return len(s) >= 8 && s[:8] == "https://"
}