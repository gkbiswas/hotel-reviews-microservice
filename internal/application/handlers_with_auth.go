package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
)

// AuthIntegratedHandlers extends the existing handlers with authentication middleware integration
type AuthIntegratedHandlers struct {
	*Handlers // Embed existing handlers
	authMiddleware *AuthMiddleware
	authService    *infrastructure.AuthenticationService
	logger         *slog.Logger
}

// NewAuthIntegratedHandlers creates new handlers with authentication integration
func NewAuthIntegratedHandlers(
	handlers *Handlers,
	authMiddleware *AuthMiddleware,
	authService *infrastructure.AuthenticationService,
	logger *slog.Logger,
) *AuthIntegratedHandlers {
	return &AuthIntegratedHandlers{
		Handlers:       handlers,
		authMiddleware: authMiddleware,
		authService:    authService,
		logger:         logger,
	}
}

// SetupAuthenticatedRoutes sets up all routes with proper authentication middleware
func (h *AuthIntegratedHandlers) SetupAuthenticatedRoutes(router *mux.Router) {
	// Create middleware chain
	chain := NewAuthMiddlewareChain(h.authMiddleware)
	
	// Health check endpoint (no authentication required)
	router.HandleFunc("/health", h.HealthCheck).Methods("GET")
	router.HandleFunc("/metrics", h.GetMetrics).Methods("GET")
	
	// API version prefix
	api := router.PathPrefix("/api/v1").Subrouter()
	
	// Public endpoints (optional authentication)
	h.setupPublicRoutes(api, chain)
	
	// Authentication endpoints
	h.setupAuthRoutes(api, chain)
	
	// Protected endpoints (authentication required)
	h.setupProtectedRoutes(api, chain)
	
	// Admin endpoints (admin role required)
	h.setupAdminRoutes(api, chain)
	
	// Moderator endpoints (moderator role required)
	h.setupModeratorRoutes(api, chain)
}

// setupPublicRoutes configures public endpoints with optional authentication
func (h *AuthIntegratedHandlers) setupPublicRoutes(api *mux.Router, chain *AuthMiddlewareChain) {
	public := api.PathPrefix("/public").Subrouter()
	public.Use(chain.ForPublicEndpoints())
	
	// Public review endpoints
	public.HandleFunc("/reviews", h.GetPublicReviews).Methods("GET")
	public.HandleFunc("/reviews/{id}", h.GetPublicReview).Methods("GET")
	public.HandleFunc("/reviews/search", h.SearchPublicReviews).Methods("GET")
	public.HandleFunc("/reviews/stats", h.GetPublicReviewStats).Methods("GET")
	
	// Public hotel endpoints
	public.HandleFunc("/hotels", h.GetPublicHotels).Methods("GET")
	public.HandleFunc("/hotels/{id}", h.GetPublicHotel).Methods("GET")
	public.HandleFunc("/hotels/search", h.SearchPublicHotels).Methods("GET")
	public.HandleFunc("/hotels/stats", h.GetPublicHotelStats).Methods("GET")
	
	// Public provider endpoints
	public.HandleFunc("/providers", h.GetPublicProviders).Methods("GET")
}

// setupAuthRoutes configures authentication endpoints
func (h *AuthIntegratedHandlers) setupAuthRoutes(api *mux.Router, chain *AuthMiddlewareChain) {
	auth := api.PathPrefix("/auth").Subrouter()
	auth.Use(chain.ForPublicEndpoints()) // Public for login/register
	
	// Create auth handlers
	authHandlers := NewAuthHandlers(h.authService, h.authService, h.logger)
	
	// Authentication endpoints
	auth.HandleFunc("/register", authHandlers.Register).Methods("POST")
	auth.HandleFunc("/login", authHandlers.Login).Methods("POST")
	auth.HandleFunc("/refresh", authHandlers.RefreshToken).Methods("POST")
	auth.HandleFunc("/logout", authHandlers.Logout).Methods("POST")
	auth.HandleFunc("/logout-all", authHandlers.LogoutAll).Methods("POST")
	
	// Password management
	auth.HandleFunc("/forgot-password", h.ForgotPassword).Methods("POST")
	auth.HandleFunc("/reset-password", h.ResetPassword).Methods("POST")
	auth.HandleFunc("/change-password", h.ChangePassword).Methods("POST")
	
	// Email verification
	auth.HandleFunc("/verify-email", h.VerifyEmail).Methods("POST")
	auth.HandleFunc("/resend-verification", h.ResendVerification).Methods("POST")
}

// setupProtectedRoutes configures protected endpoints requiring authentication
func (h *AuthIntegratedHandlers) setupProtectedRoutes(api *mux.Router, chain *AuthMiddlewareChain) {
	protected := api.PathPrefix("/protected").Subrouter()
	protected.Use(chain.ForProtectedEndpoints())
	
	// User profile endpoints
	h.setupUserProfileRoutes(protected, chain)
	
	// Review endpoints with permissions
	h.setupReviewRoutes(protected, chain)
	
	// Hotel endpoints with permissions
	h.setupHotelRoutes(protected, chain)
	
	// Provider endpoints with permissions
	h.setupProviderRoutes(protected, chain)
	
	// File processing endpoints
	h.setupFileProcessingRoutes(protected, chain)
	
	// API key management
	h.setupAPIKeyRoutes(protected, chain)
}

// setupUserProfileRoutes configures user profile endpoints
func (h *AuthIntegratedHandlers) setupUserProfileRoutes(protected *mux.Router, chain *AuthMiddlewareChain) {
	profile := protected.PathPrefix("/profile").Subrouter()
	
	// Basic profile operations
	profile.HandleFunc("", h.GetUserProfile).Methods("GET")
	profile.HandleFunc("", h.UpdateUserProfile).Methods("PUT")
	profile.HandleFunc("/avatar", h.UpdateUserAvatar).Methods("POST")
	profile.HandleFunc("/preferences", h.GetUserPreferences).Methods("GET")
	profile.HandleFunc("/preferences", h.UpdateUserPreferences).Methods("PUT")
	
	// User's reviews
	profile.HandleFunc("/reviews", h.GetUserReviews).Methods("GET")
	profile.HandleFunc("/reviews/stats", h.GetUserReviewStats).Methods("GET")
	
	// User's sessions
	profile.HandleFunc("/sessions", h.GetUserSessions).Methods("GET")
	profile.HandleFunc("/sessions/{id}", h.DeleteUserSession).Methods("DELETE")
	profile.HandleFunc("/sessions/all", h.DeleteAllUserSessions).Methods("DELETE")
}

// setupReviewRoutes configures review endpoints with permission checks
func (h *AuthIntegratedHandlers) setupReviewRoutes(protected *mux.Router, chain *AuthMiddlewareChain) {
	reviews := protected.PathPrefix("/reviews").Subrouter()
	
	// Read reviews - requires 'reviews:read' permission
	reviews.Handle("", chain.WithPermission("reviews", "read")(
		http.HandlerFunc(h.GetReviews),
	)).Methods("GET")
	
	reviews.Handle("/{id}", chain.WithPermission("reviews", "read")(
		http.HandlerFunc(h.GetReview),
	)).Methods("GET")
	
	reviews.Handle("/search", chain.WithPermission("reviews", "read")(
		http.HandlerFunc(h.SearchReviews),
	)).Methods("GET")
	
	// Create reviews - requires 'reviews:create' permission
	reviews.Handle("", chain.WithPermission("reviews", "create")(
		http.HandlerFunc(h.CreateReview),
	)).Methods("POST")
	
	// Update reviews - requires 'reviews:update' permission or ownership
	reviews.Handle("/{id}", chain.WithPermission("reviews", "update")(
		http.HandlerFunc(h.UpdateReviewWithOwnershipCheck),
	)).Methods("PUT")
	
	// Delete reviews - requires 'reviews:delete' permission or ownership
	reviews.Handle("/{id}", chain.WithPermission("reviews", "delete")(
		http.HandlerFunc(h.DeleteReviewWithOwnershipCheck),
	)).Methods("DELETE")
	
	// Bulk operations - requires higher permissions
	reviews.Handle("/bulk", chain.WithPermission("reviews", "create")(
		http.HandlerFunc(h.BulkCreateReviews),
	)).Methods("POST")
	
	reviews.Handle("/bulk", chain.WithPermission("reviews", "update")(
		http.HandlerFunc(h.BulkUpdateReviews),
	)).Methods("PUT")
	
	reviews.Handle("/bulk", chain.WithPermission("reviews", "delete")(
		http.HandlerFunc(h.BulkDeleteReviews),
	)).Methods("DELETE")
	
	// Export reviews - requires 'reviews:read' permission
	reviews.Handle("/export", chain.WithPermission("reviews", "read")(
		http.HandlerFunc(h.ExportReviews),
	)).Methods("GET")
}

// setupHotelRoutes configures hotel endpoints with permission checks
func (h *AuthIntegratedHandlers) setupHotelRoutes(protected *mux.Router, chain *AuthMiddlewareChain) {
	hotels := protected.PathPrefix("/hotels").Subrouter()
	
	// Read hotels - requires 'hotels:read' permission
	hotels.Handle("", chain.WithPermission("hotels", "read")(
		http.HandlerFunc(h.GetHotels),
	)).Methods("GET")
	
	hotels.Handle("/{id}", chain.WithPermission("hotels", "read")(
		http.HandlerFunc(h.GetHotel),
	)).Methods("GET")
	
	hotels.Handle("/search", chain.WithPermission("hotels", "read")(
		http.HandlerFunc(h.SearchHotels),
	)).Methods("GET")
	
	// Create hotels - requires 'hotels:create' permission
	hotels.Handle("", chain.WithPermission("hotels", "create")(
		http.HandlerFunc(h.CreateHotel),
	)).Methods("POST")
	
	// Update hotels - requires 'hotels:update' permission
	hotels.Handle("/{id}", chain.WithPermission("hotels", "update")(
		http.HandlerFunc(h.UpdateHotel),
	)).Methods("PUT")
	
	// Delete hotels - requires 'hotels:delete' permission
	hotels.Handle("/{id}", chain.WithPermission("hotels", "delete")(
		http.HandlerFunc(h.DeleteHotel),
	)).Methods("DELETE")
	
	// Hotel analytics - requires 'hotels:read' permission
	hotels.Handle("/{id}/analytics", chain.WithPermission("hotels", "read")(
		http.HandlerFunc(h.GetHotelAnalytics),
	)).Methods("GET")
	
	hotels.Handle("/{id}/reviews", chain.WithPermission("hotels", "read")(
		http.HandlerFunc(h.GetHotelReviews),
	)).Methods("GET")
}

// setupProviderRoutes configures provider endpoints with permission checks
func (h *AuthIntegratedHandlers) setupProviderRoutes(protected *mux.Router, chain *AuthMiddlewareChain) {
	providers := protected.PathPrefix("/providers").Subrouter()
	
	// Read providers - requires 'providers:read' permission
	providers.Handle("", chain.WithPermission("providers", "read")(
		http.HandlerFunc(h.GetProviders),
	)).Methods("GET")
	
	providers.Handle("/{id}", chain.WithPermission("providers", "read")(
		http.HandlerFunc(h.GetProvider),
	)).Methods("GET")
	
	// Create providers - requires 'providers:create' permission
	providers.Handle("", chain.WithPermission("providers", "create")(
		http.HandlerFunc(h.CreateProvider),
	)).Methods("POST")
	
	// Update providers - requires 'providers:update' permission
	providers.Handle("/{id}", chain.WithPermission("providers", "update")(
		http.HandlerFunc(h.UpdateProvider),
	)).Methods("PUT")
	
	// Delete providers - requires 'providers:delete' permission
	providers.Handle("/{id}", chain.WithPermission("providers", "delete")(
		http.HandlerFunc(h.DeleteProvider),
	)).Methods("DELETE")
}

// setupFileProcessingRoutes configures file processing endpoints
func (h *AuthIntegratedHandlers) setupFileProcessingRoutes(protected *mux.Router, chain *AuthMiddlewareChain) {
	files := protected.PathPrefix("/files").Subrouter()
	
	// Upload file - requires 'files:upload' permission
	files.Handle("/upload", chain.WithPermission("files", "upload")(
		http.HandlerFunc(h.UploadFile),
	)).Methods("POST")
	
	// Process file - requires 'files:process' permission
	files.Handle("/process", chain.WithPermission("files", "process")(
		http.HandlerFunc(h.ProcessFile),
	)).Methods("POST")
	
	// Get processing status - requires 'files:read' permission
	files.Handle("/status/{id}", chain.WithPermission("files", "read")(
		http.HandlerFunc(h.GetProcessingStatus),
	)).Methods("GET")
	
	// List processing history - requires 'files:read' permission
	files.Handle("/history", chain.WithPermission("files", "read")(
		http.HandlerFunc(h.GetProcessingHistory),
	)).Methods("GET")
	
	// Cancel processing - requires 'files:cancel' permission
	files.Handle("/cancel/{id}", chain.WithPermission("files", "cancel")(
		http.HandlerFunc(h.CancelProcessing),
	)).Methods("POST")
}

// setupAPIKeyRoutes configures API key management endpoints
func (h *AuthIntegratedHandlers) setupAPIKeyRoutes(protected *mux.Router, chain *AuthMiddlewareChain) {
	apiKeys := protected.PathPrefix("/api-keys").Subrouter()
	
	// Create auth handlers for API key management
	authHandlers := NewAuthHandlers(h.authService, h.authService, h.logger)
	
	// List API keys
	apiKeys.HandleFunc("", authHandlers.ListApiKeys).Methods("GET")
	
	// Create API key
	apiKeys.HandleFunc("", authHandlers.CreateApiKey).Methods("POST")
	
	// Delete API key
	apiKeys.HandleFunc("/{id}", authHandlers.DeleteApiKey).Methods("DELETE")
	
	// Get API key usage statistics
	apiKeys.HandleFunc("/{id}/usage", h.GetAPIKeyUsage).Methods("GET")
	
	// Rotate API key
	apiKeys.HandleFunc("/{id}/rotate", h.RotateAPIKey).Methods("POST")
}

// setupAdminRoutes configures admin endpoints requiring admin role
func (h *AuthIntegratedHandlers) setupAdminRoutes(api *mux.Router, chain *AuthMiddlewareChain) {
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(chain.ForAdminEndpoints())
	
	// Create auth handlers for admin operations
	authHandlers := NewAuthHandlers(h.authService, h.authService, h.logger)
	
	// User management
	h.setupAdminUserRoutes(admin, authHandlers)
	
	// Role and permission management
	h.setupAdminRoleRoutes(admin, authHandlers)
	
	// System management
	h.setupAdminSystemRoutes(admin, chain)
	
	// Analytics and reporting
	h.setupAdminAnalyticsRoutes(admin, chain)
}

// setupAdminUserRoutes configures admin user management endpoints
func (h *AuthIntegratedHandlers) setupAdminUserRoutes(admin *mux.Router, authHandlers *AuthHandlers) {
	users := admin.PathPrefix("/users").Subrouter()
	
	// User CRUD operations
	users.HandleFunc("", authHandlers.ListUsers).Methods("GET")
	users.HandleFunc("/{id}", authHandlers.GetUser).Methods("GET")
	users.HandleFunc("/{id}", authHandlers.DeleteUser).Methods("DELETE")
	users.HandleFunc("/{id}/roles", authHandlers.AssignRole).Methods("POST")
	
	// User status management
	users.HandleFunc("/{id}/activate", h.ActivateUser).Methods("POST")
	users.HandleFunc("/{id}/deactivate", h.DeactivateUser).Methods("POST")
	users.HandleFunc("/{id}/lock", h.LockUser).Methods("POST")
	users.HandleFunc("/{id}/unlock", h.UnlockUser).Methods("POST")
	
	// User analytics
	users.HandleFunc("/{id}/analytics", h.GetUserAnalytics).Methods("GET")
	users.HandleFunc("/{id}/activity", h.GetUserActivity).Methods("GET")
	users.HandleFunc("/{id}/sessions", h.GetUserSessions).Methods("GET")
}

// setupAdminRoleRoutes configures admin role management endpoints
func (h *AuthIntegratedHandlers) setupAdminRoleRoutes(admin *mux.Router, authHandlers *AuthHandlers) {
	roles := admin.PathPrefix("/roles").Subrouter()
	
	// Role CRUD operations
	roles.HandleFunc("", h.ListRoles).Methods("GET")
	roles.HandleFunc("", h.CreateRole).Methods("POST")
	roles.HandleFunc("/{id}", h.GetRole).Methods("GET")
	roles.HandleFunc("/{id}", h.UpdateRole).Methods("PUT")
	roles.HandleFunc("/{id}", h.DeleteRole).Methods("DELETE")
	
	// Role-permission management
	roles.HandleFunc("/{id}/permissions", h.GetRolePermissions).Methods("GET")
	roles.HandleFunc("/{id}/permissions", h.AssignRolePermissions).Methods("POST")
	roles.HandleFunc("/{id}/permissions/{permissionId}", h.RemoveRolePermission).Methods("DELETE")
	
	// Permission CRUD operations
	permissions := admin.PathPrefix("/permissions").Subrouter()
	permissions.HandleFunc("", h.ListPermissions).Methods("GET")
	permissions.HandleFunc("", h.CreatePermission).Methods("POST")
	permissions.HandleFunc("/{id}", h.GetPermission).Methods("GET")
	permissions.HandleFunc("/{id}", h.UpdatePermission).Methods("PUT")
	permissions.HandleFunc("/{id}", h.DeletePermission).Methods("DELETE")
}

// setupAdminSystemRoutes configures admin system management endpoints
func (h *AuthIntegratedHandlers) setupAdminSystemRoutes(admin *mux.Router, chain *AuthMiddlewareChain) {
	system := admin.PathPrefix("/system").Subrouter()
	
	// System health and metrics
	system.HandleFunc("/health", h.GetSystemHealth).Methods("GET")
	system.HandleFunc("/metrics", h.GetSystemMetrics).Methods("GET")
	system.HandleFunc("/status", h.GetSystemStatus).Methods("GET")
	
	// Configuration management
	system.HandleFunc("/config", h.GetSystemConfig).Methods("GET")
	system.HandleFunc("/config", h.UpdateSystemConfig).Methods("PUT")
	
	// Cache management
	system.HandleFunc("/cache/clear", h.ClearCache).Methods("POST")
	system.HandleFunc("/cache/stats", h.GetCacheStats).Methods("GET")
	
	// Background job management
	system.HandleFunc("/jobs", h.GetBackgroundJobs).Methods("GET")
	system.HandleFunc("/jobs/{id}", h.GetBackgroundJob).Methods("GET")
	system.HandleFunc("/jobs/{id}/cancel", h.CancelBackgroundJob).Methods("POST")
	
	// Audit log management
	system.HandleFunc("/audit", h.GetAuditLogs).Methods("GET")
	system.HandleFunc("/audit/export", h.ExportAuditLogs).Methods("GET")
}

// setupAdminAnalyticsRoutes configures admin analytics endpoints
func (h *AuthIntegratedHandlers) setupAdminAnalyticsRoutes(admin *mux.Router, chain *AuthMiddlewareChain) {
	analytics := admin.PathPrefix("/analytics").Subrouter()
	
	// Authentication analytics
	analytics.HandleFunc("/auth", h.GetAuthAnalytics).Methods("GET")
	analytics.HandleFunc("/auth/trends", h.GetAuthTrends).Methods("GET")
	analytics.HandleFunc("/auth/failures", h.GetAuthFailures).Methods("GET")
	
	// Usage analytics
	analytics.HandleFunc("/usage", h.GetUsageAnalytics).Methods("GET")
	analytics.HandleFunc("/usage/trends", h.GetUsageTrends).Methods("GET")
	analytics.HandleFunc("/usage/top-users", h.GetTopUsers).Methods("GET")
	
	// Performance analytics
	analytics.HandleFunc("/performance", h.GetPerformanceAnalytics).Methods("GET")
	analytics.HandleFunc("/performance/endpoints", h.GetEndpointPerformance).Methods("GET")
	analytics.HandleFunc("/performance/errors", h.GetErrorAnalytics).Methods("GET")
}

// setupModeratorRoutes configures moderator endpoints
func (h *AuthIntegratedHandlers) setupModeratorRoutes(api *mux.Router, chain *AuthMiddlewareChain) {
	moderator := api.PathPrefix("/moderator").Subrouter()
	moderator.Use(chain.ForProtectedEndpoints())
	moderator.Use(chain.WithRole("moderator", "admin"))
	
	// Review moderation
	reviews := moderator.PathPrefix("/reviews").Subrouter()
	reviews.HandleFunc("/pending", h.GetPendingReviews).Methods("GET")
	reviews.HandleFunc("/flagged", h.GetFlaggedReviews).Methods("GET")
	reviews.HandleFunc("/{id}/approve", h.ApproveReview).Methods("POST")
	reviews.HandleFunc("/{id}/reject", h.RejectReview).Methods("POST")
	reviews.HandleFunc("/{id}/flag", h.FlagReview).Methods("POST")
	reviews.HandleFunc("/{id}/unflag", h.UnflagReview).Methods("POST")
	
	// User moderation
	users := moderator.PathPrefix("/users").Subrouter()
	users.HandleFunc("/suspicious", h.GetSuspiciousUsers).Methods("GET")
	users.HandleFunc("/{id}/warn", h.WarnUser).Methods("POST")
	users.HandleFunc("/{id}/suspend", h.SuspendUser).Methods("POST")
	users.HandleFunc("/{id}/unsuspend", h.UnsuspendUser).Methods("POST")
	
	// Content moderation
	content := moderator.PathPrefix("/content").Subrouter()
	content.HandleFunc("/reports", h.GetContentReports).Methods("GET")
	content.HandleFunc("/reports/{id}", h.GetContentReport).Methods("GET")
	content.HandleFunc("/reports/{id}/resolve", h.ResolveContentReport).Methods("POST")
}

// Enhanced handler methods with authentication integration

// GetUserProfile returns the authenticated user's profile
func (h *AuthIntegratedHandlers) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authMiddleware.GetUserFromContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}
	
	// Get additional user data
	profile, err := h.authService.GetUser(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("Failed to get user profile", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get profile", "")
		return
	}
	
	// Remove sensitive data
	profile.PasswordHash = ""
	profile.TwoFactorSecret = ""
	
	h.writeSuccessResponse(w, http.StatusOK, "Profile retrieved successfully", profile)
}

// UpdateUserProfile updates the authenticated user's profile
func (h *AuthIntegratedHandlers) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authMiddleware.GetUserFromContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}
	
	var updateReq UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	
	// Update user profile
	if updateReq.FirstName != "" {
		user.FirstName = updateReq.FirstName
	}
	if updateReq.LastName != "" {
		user.LastName = updateReq.LastName
	}
	if updateReq.Email != "" {
		user.Email = updateReq.Email
	}
	
	user.UpdatedAt = time.Now()
	
	if err := h.authService.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("Failed to update user profile", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update profile", "")
		return
	}
	
	// Audit the update
	clientIP, _ := h.authMiddleware.GetClientIPFromContext(r)
	_ = h.authService.AuditAction(r.Context(), &user.ID, "update", "profile", &user.ID, nil, nil, clientIP, r.Header.Get("User-Agent"))
	
	h.writeSuccessResponse(w, http.StatusOK, "Profile updated successfully", user)
}

// UpdateReviewWithOwnershipCheck updates a review with ownership validation
func (h *AuthIntegratedHandlers) UpdateReviewWithOwnershipCheck(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authMiddleware.GetUserFromContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}
	
	vars := mux.Vars(r)
	reviewID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid review ID", err.Error())
		return
	}
	
	// Get the review to check ownership
	review, err := h.reviewService.GetReviewByID(r.Context(), reviewID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "Review not found", err.Error())
		return
	}
	
	// Check if user owns the review or has admin privileges
	canUpdate := false
	if review.ReviewerInfo.Email == user.Email {
		canUpdate = true
	} else {
		// Check if user has admin role
		for _, role := range user.Roles {
			if role.Name == "admin" || role.Name == "moderator" {
				canUpdate = true
				break
			}
		}
	}
	
	if !canUpdate {
		h.writeErrorResponse(w, http.StatusForbidden, "Cannot update this review", "")
		return
	}
	
	// Proceed with update
	h.UpdateReview(w, r)
}

// DeleteReviewWithOwnershipCheck deletes a review with ownership validation
func (h *AuthIntegratedHandlers) DeleteReviewWithOwnershipCheck(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authMiddleware.GetUserFromContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}
	
	vars := mux.Vars(r)
	reviewID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid review ID", err.Error())
		return
	}
	
	// Get the review to check ownership
	review, err := h.reviewService.GetReviewByID(r.Context(), reviewID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "Review not found", err.Error())
		return
	}
	
	// Check if user owns the review or has admin privileges
	canDelete := false
	if review.ReviewerInfo.Email == user.Email {
		canDelete = true
	} else {
		// Check if user has admin role
		for _, role := range user.Roles {
			if role.Name == "admin" || role.Name == "moderator" {
				canDelete = true
				break
			}
		}
	}
	
	if !canDelete {
		h.writeErrorResponse(w, http.StatusForbidden, "Cannot delete this review", "")
		return
	}
	
	// Proceed with deletion
	h.DeleteReview(w, r)
}

// GetUserReviews returns reviews created by the authenticated user
func (h *AuthIntegratedHandlers) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authMiddleware.GetUserFromContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}
	
	// Parse pagination parameters
	limit, offset := h.parsePaginationParams(r)
	
	// Get reviews for the user (using email as identifier)
	reviews, err := h.reviewService.GetReviewsByUserEmail(r.Context(), user.Email, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get user reviews", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get reviews", "")
		return
	}
	
	h.writeSuccessResponse(w, http.StatusOK, "Reviews retrieved successfully", reviews)
}

// GetUserReviewStats returns review statistics for the authenticated user
func (h *AuthIntegratedHandlers) GetUserReviewStats(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authMiddleware.GetUserFromContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}
	
	// Get review statistics for the user
	stats, err := h.reviewService.GetUserReviewStats(r.Context(), user.Email)
	if err != nil {
		h.logger.Error("Failed to get user review stats", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get statistics", "")
		return
	}
	
	h.writeSuccessResponse(w, http.StatusOK, "Statistics retrieved successfully", stats)
}

// GetAPIKeyUsage returns usage statistics for an API key
func (h *AuthIntegratedHandlers) GetAPIKeyUsage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authMiddleware.GetUserFromContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}
	
	vars := mux.Vars(r)
	keyID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid API key ID", err.Error())
		return
	}
	
	// Get API key usage statistics
	usage, err := h.authService.GetAPIKeyUsage(r.Context(), keyID, user.ID)
	if err != nil {
		h.logger.Error("Failed to get API key usage", "error", err, "key_id", keyID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get usage statistics", "")
		return
	}
	
	h.writeSuccessResponse(w, http.StatusOK, "Usage statistics retrieved successfully", usage)
}

// Helper methods for new request types

// UpdateUserRequest represents a user profile update request
type UpdateUserRequest struct {
	FirstName string `json:"first_name" validate:"max=100"`
	LastName  string `json:"last_name" validate:"max=100"`
	Email     string `json:"email" validate:"email,max=255"`
}

// Helper method to parse pagination parameters
func (h *AuthIntegratedHandlers) parsePaginationParams(r *http.Request) (int, int) {
	limit := 20 // default limit
	offset := 0 // default offset
	
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}
	
	// Limit maximum page size
	if limit > 100 {
		limit = 100
	}
	
	return limit, offset
}

// GetAuthMiddlewareStatus returns the status of the authentication middleware
func (h *AuthIntegratedHandlers) GetAuthMiddlewareStatus(w http.ResponseWriter, r *http.Request) {
	status := h.authMiddleware.GetAuthMiddlewareStatus()
	h.writeSuccessResponse(w, http.StatusOK, "Authentication middleware status", status)
}

// Response helper methods

func (h *AuthIntegratedHandlers) writeSuccessResponse(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
		"time":    time.Now().UTC().Format(time.RFC3339),
	}
	
	if requestID, ok := h.authMiddleware.GetRequestIDFromContext(w.(*http.Request)); ok {
		response["request_id"] = requestID
	}
	
	json.NewEncoder(w).Encode(response)
}

func (h *AuthIntegratedHandlers) writeErrorResponse(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"success": false,
		"error":   message,
		"code":    statusCode,
		"time":    time.Now().UTC().Format(time.RFC3339),
	}
	
	if details != "" {
		response["details"] = details
	}
	
	json.NewEncoder(w).Encode(response)
}

// Public endpoint handlers (these would be implemented)

func (h *AuthIntegratedHandlers) GetPublicReviews(w http.ResponseWriter, r *http.Request) {
	// Implementation for public reviews
	h.writeSuccessResponse(w, http.StatusOK, "Public reviews retrieved", nil)
}

func (h *AuthIntegratedHandlers) GetPublicReview(w http.ResponseWriter, r *http.Request) {
	// Implementation for public review
	h.writeSuccessResponse(w, http.StatusOK, "Public review retrieved", nil)
}

func (h *AuthIntegratedHandlers) SearchPublicReviews(w http.ResponseWriter, r *http.Request) {
	// Implementation for public review search
	h.writeSuccessResponse(w, http.StatusOK, "Public reviews searched", nil)
}

func (h *AuthIntegratedHandlers) GetPublicReviewStats(w http.ResponseWriter, r *http.Request) {
	// Implementation for public review stats
	h.writeSuccessResponse(w, http.StatusOK, "Public review stats retrieved", nil)
}

func (h *AuthIntegratedHandlers) GetPublicHotels(w http.ResponseWriter, r *http.Request) {
	// Implementation for public hotels
	h.writeSuccessResponse(w, http.StatusOK, "Public hotels retrieved", nil)
}

func (h *AuthIntegratedHandlers) GetPublicHotel(w http.ResponseWriter, r *http.Request) {
	// Implementation for public hotel
	h.writeSuccessResponse(w, http.StatusOK, "Public hotel retrieved", nil)
}

func (h *AuthIntegratedHandlers) SearchPublicHotels(w http.ResponseWriter, r *http.Request) {
	// Implementation for public hotel search
	h.writeSuccessResponse(w, http.StatusOK, "Public hotels searched", nil)
}

func (h *AuthIntegratedHandlers) GetPublicHotelStats(w http.ResponseWriter, r *http.Request) {
	// Implementation for public hotel stats
	h.writeSuccessResponse(w, http.StatusOK, "Public hotel stats retrieved", nil)
}

func (h *AuthIntegratedHandlers) GetPublicProviders(w http.ResponseWriter, r *http.Request) {
	// Implementation for public providers
	h.writeSuccessResponse(w, http.StatusOK, "Public providers retrieved", nil)
}