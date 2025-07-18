package application

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// IntegratedHandlers provides comprehensive handlers with all features integrated
type IntegratedHandlers struct {
	*ResilientHandlers // Embed resilient handlers
	
	// Authentication components
	authService *infrastructure.AuthService
	rbacService *infrastructure.RBACService
}

// NewIntegratedHandlers creates handlers with all features integrated
func NewIntegratedHandlers(
	reviewService domain.ReviewService,
	logger *logger.Logger,
	circuitBreakerIntegration *infrastructure.CircuitBreakerIntegration,
	retryManager *infrastructure.RetryManager,
	authService *infrastructure.AuthService,
	rbacService *infrastructure.RBACService,
) *IntegratedHandlers {
	resilientHandlers := NewResilientHandlers(
		reviewService,
		logger,
		circuitBreakerIntegration,
		retryManager,
	)
	
	return &IntegratedHandlers{
		ResilientHandlers: resilientHandlers,
		authService:       authService,
		rbacService:       rbacService,
	}
}

// SetupAdminRoutes sets up admin-only routes
func (h *IntegratedHandlers) SetupAdminRoutes(router gin.IRouter) {
	// User management routes
	router.GET("/users", h.listUsers)
	router.GET("/users/:id", h.getUserByID)
	router.PUT("/users/:id", h.updateUser)
	router.DELETE("/users/:id", h.deleteUser)
	router.POST("/users/:id/activate", h.activateUser)
	router.POST("/users/:id/deactivate", h.deactivateUser)
	
	// Role management routes
	router.GET("/roles", h.listRoles)
	router.POST("/roles", h.createRole)
	router.GET("/roles/:id", h.getRoleByID)
	router.PUT("/roles/:id", h.updateRole)
	router.DELETE("/roles/:id", h.deleteRole)
	
	// User-role assignment routes
	router.POST("/users/:user_id/roles/:role_id", h.assignRole)
	router.DELETE("/users/:user_id/roles/:role_id", h.unassignRole)
	router.GET("/users/:user_id/roles", h.getUserRoles)
	
	// API key management routes
	router.GET("/api-keys", h.listAPIKeys)
	router.POST("/api-keys", h.createAPIKey)
	router.GET("/api-keys/:id", h.getAPIKeyByID)
	router.PUT("/api-keys/:id", h.updateAPIKey)
	router.DELETE("/api-keys/:id", h.deleteAPIKey)
	router.POST("/api-keys/:id/regenerate", h.regenerateAPIKey)
	
	// System statistics routes
	router.GET("/stats/users", h.getUserStats)
	router.GET("/stats/sessions", h.getSessionStats)
	router.GET("/stats/api-keys", h.getAPIKeyStats)
	router.GET("/stats/audit", h.getAuditStats)
	
	// System management routes
	router.POST("/system/reset-circuit-breakers", h.resetCircuitBreakers)
	router.POST("/system/clear-cache", h.clearCache)
	router.GET("/system/health", h.getSystemHealth)
}

// SetupServiceRoutes sets up service-to-service routes
func (h *IntegratedHandlers) SetupServiceRoutes(router gin.IRouter) {
	// Bulk operations for services
	router.POST("/reviews/bulk", h.bulkCreateReviews)
	router.GET("/reviews/export", h.exportReviews)
	router.POST("/reviews/import", h.importReviews)
	
	// Service statistics
	router.GET("/stats/reviews", h.getServiceReviewStats)
	router.GET("/stats/hotels", h.getServiceHotelStats)
	router.GET("/stats/processing", h.getServiceProcessingStats)
	
	// Service health checks
	router.GET("/health", h.getServiceHealth)
	router.GET("/metrics", h.getServiceMetrics)
}

// Admin handler implementations

// listUsers lists all users (admin only)
func (h *IntegratedHandlers) listUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	
	// Execute with resilience protection
	result, err := h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.authService.GetUsers(ctx, limit, offset)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to list users")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// getUserByID gets a user by ID (admin only)
func (h *IntegratedHandlers) getUserByID(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	// Execute with resilience protection
	result, err := h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.authService.GetUserByID(ctx, userID)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to get user")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// updateUser updates a user (admin only)
func (h *IntegratedHandlers) updateUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	var updateReq struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		IsActive  *bool  `json:"is_active"`
		IsAdmin   *bool  `json:"is_admin"`
	}
	
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}
	
	// Execute with resilience protection
	result, err := h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		// Get existing user
		user, err := h.authService.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		
		// Update fields
		if updateReq.FirstName != "" {
			user.FirstName = updateReq.FirstName
		}
		if updateReq.LastName != "" {
			user.LastName = updateReq.LastName
		}
		if updateReq.Email != "" {
			user.Email = updateReq.Email
		}
		if updateReq.IsActive != nil {
			user.IsActive = *updateReq.IsActive
		}
		if updateReq.IsAdmin != nil {
			user.IsAdmin = *updateReq.IsAdmin
		}
		
		// Update user
		return user, h.authService.UpdateUser(ctx, user)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to update user")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// deleteUser deletes a user (admin only)
func (h *IntegratedHandlers) deleteUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	// Execute with resilience protection
	_, err = h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return nil, h.authService.DeleteUser(ctx, userID)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to delete user")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]interface{}{"message": "User deleted successfully"},
	})
}

// activateUser activates a user (admin only)
func (h *IntegratedHandlers) activateUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	// Execute with resilience protection
	_, err = h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return nil, h.authService.ActivateUser(ctx, userID)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to activate user")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]interface{}{"message": "User activated successfully"},
	})
}

// deactivateUser deactivates a user (admin only)
func (h *IntegratedHandlers) deactivateUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	// Execute with resilience protection
	_, err = h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return nil, h.authService.DeactivateUser(ctx, userID)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to deactivate user")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]interface{}{"message": "User deactivated successfully"},
	})
}

// Role management handlers

// listRoles lists all roles (admin only)
func (h *IntegratedHandlers) listRoles(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	result, err := h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.rbacService.GetRoles(ctx)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to list roles")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// createRole creates a new role (admin only)
func (h *IntegratedHandlers) createRole(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	var createReq struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	
	if err := c.ShouldBindJSON(&createReq); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}
	
	result, err := h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.rbacService.CreateRole(ctx, createReq.Name, createReq.Description, createReq.Permissions)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to create role")
		return
	}
	
	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    result,
	})
}

// getRoleByID gets a role by ID (admin only)
func (h *IntegratedHandlers) getRoleByID(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	roleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid role ID",
		})
		return
	}
	
	result, err := h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.rbacService.GetRoleByID(ctx, roleID)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to get role")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// updateRole updates a role (admin only)
func (h *IntegratedHandlers) updateRole(c *gin.Context) {
	// Implementation similar to updateUser
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// deleteRole deletes a role (admin only)
func (h *IntegratedHandlers) deleteRole(c *gin.Context) {
	// Implementation similar to deleteUser
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// assignRole assigns a role to a user (admin only)
func (h *IntegratedHandlers) assignRole(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	roleName := c.Param("role_id") // This should be role name, not ID
	
	_, err = h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return nil, h.rbacService.AssignRole(ctx, userID, roleName)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to assign role")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]interface{}{"message": "Role assigned successfully"},
	})
}

// unassignRole removes a role from a user (admin only)
func (h *IntegratedHandlers) unassignRole(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	roleName := c.Param("role_id") // This should be role name, not ID
	
	_, err = h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return nil, h.rbacService.UnassignRole(ctx, userID, roleName)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to unassign role")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]interface{}{"message": "Role unassigned successfully"},
	})
}

// getUserRoles gets all roles for a user (admin only)
func (h *IntegratedHandlers) getUserRoles(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}
	
	result, err := h.executeWithRetryAndCircuitBreaker(ctx, "database", func(ctx context.Context) (interface{}, error) {
		return h.rbacService.GetUserRoles(ctx, userID)
	})
	
	if err != nil {
		h.handleProtectedError(c, err, "Failed to get user roles")
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// API Key management handlers

// listAPIKeys lists all API keys (admin only)
func (h *IntegratedHandlers) listAPIKeys(c *gin.Context) {
	// Implementation for listing API keys
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// createAPIKey creates a new API key (admin only)
func (h *IntegratedHandlers) createAPIKey(c *gin.Context) {
	// Implementation for creating API keys
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getAPIKeyByID gets an API key by ID (admin only)
func (h *IntegratedHandlers) getAPIKeyByID(c *gin.Context) {
	// Implementation for getting API key by ID
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// updateAPIKey updates an API key (admin only)
func (h *IntegratedHandlers) updateAPIKey(c *gin.Context) {
	// Implementation for updating API keys
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// deleteAPIKey deletes an API key (admin only)
func (h *IntegratedHandlers) deleteAPIKey(c *gin.Context) {
	// Implementation for deleting API keys
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// regenerateAPIKey regenerates an API key (admin only)
func (h *IntegratedHandlers) regenerateAPIKey(c *gin.Context) {
	// Implementation for regenerating API keys
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// Statistics handlers

// getUserStats gets user statistics (admin only)
func (h *IntegratedHandlers) getUserStats(c *gin.Context) {
	// Implementation for user statistics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getSessionStats gets session statistics (admin only)
func (h *IntegratedHandlers) getSessionStats(c *gin.Context) {
	// Implementation for session statistics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getAPIKeyStats gets API key statistics (admin only)
func (h *IntegratedHandlers) getAPIKeyStats(c *gin.Context) {
	// Implementation for API key statistics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getAuditStats gets audit statistics (admin only)
func (h *IntegratedHandlers) getAuditStats(c *gin.Context) {
	// Implementation for audit statistics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// System management handlers

// resetCircuitBreakers resets all circuit breakers (admin only)
func (h *IntegratedHandlers) resetCircuitBreakers(c *gin.Context) {
	if h.circuitBreakerIntegration == nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Circuit breakers are disabled",
		})
		return
	}
	
	h.circuitBreakerIntegration.ResetAllCircuitBreakers()
	h.logger.Info("All circuit breakers reset via admin API")
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "All circuit breakers reset successfully",
			"timestamp": time.Now().UTC(),
		},
	})
}

// clearCache clears all caches (admin only)
func (h *IntegratedHandlers) clearCache(c *gin.Context) {
	// Implementation for clearing caches
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getSystemHealth gets comprehensive system health (admin only)
func (h *IntegratedHandlers) getSystemHealth(c *gin.Context) {
	// Implementation for system health
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// Service handler implementations

// bulkCreateReviews creates multiple reviews in bulk (service only)
func (h *IntegratedHandlers) bulkCreateReviews(c *gin.Context) {
	// Implementation for bulk review creation
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// exportReviews exports reviews in various formats (service only)
func (h *IntegratedHandlers) exportReviews(c *gin.Context) {
	// Implementation for review export
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// importReviews imports reviews from various formats (service only)
func (h *IntegratedHandlers) importReviews(c *gin.Context) {
	// Implementation for review import
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getServiceReviewStats gets review statistics for services
func (h *IntegratedHandlers) getServiceReviewStats(c *gin.Context) {
	// Implementation for service review statistics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getServiceHotelStats gets hotel statistics for services
func (h *IntegratedHandlers) getServiceHotelStats(c *gin.Context) {
	// Implementation for service hotel statistics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getServiceProcessingStats gets processing statistics for services
func (h *IntegratedHandlers) getServiceProcessingStats(c *gin.Context) {
	// Implementation for service processing statistics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getServiceHealth gets service health (service only)
func (h *IntegratedHandlers) getServiceHealth(c *gin.Context) {
	// Implementation for service health
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}

// getServiceMetrics gets service metrics (service only)
func (h *IntegratedHandlers) getServiceMetrics(c *gin.Context) {
	// Implementation for service metrics
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "Not implemented yet",
	})
}