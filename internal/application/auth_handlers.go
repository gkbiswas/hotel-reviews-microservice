package application

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
)

// AuthHandlers handles authentication-related HTTP requests
type AuthHandlers struct {
	authService     *infrastructure.AuthenticationService
	passwordService domain.PasswordService
	logger          *slog.Logger
}

// NewAuthHandlers creates a new authentication handlers instance
func NewAuthHandlers(authService *infrastructure.AuthenticationService, passwordService domain.PasswordService, logger *slog.Logger) *AuthHandlers {
	return &AuthHandlers{
		authService:     authService,
		passwordService: passwordService,
		logger:          logger,
	}
}

// Request and Response structures

type RegisterRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=50"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"max=100"`
	LastName  string `json:"last_name" validate:"max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type CreateApiKeyRequest struct {
	Name      string     `json:"name" validate:"required,min=1,max=100"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type UpdateUserRequest struct {
	FirstName string `json:"first_name" validate:"max=100"`
	LastName  string `json:"last_name" validate:"max=100"`
	Email     string `json:"email" validate:"email"`
}

type AssignRoleRequest struct {
	RoleID uuid.UUID `json:"role_id" validate:"required"`
}

type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	Description string `json:"description"`
}

type CreatePermissionRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Resource    string `json:"resource" validate:"required,max=50"`
	Action      string `json:"action" validate:"required,oneof=create read update delete execute"`
	Description string `json:"description"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Time    string `json:"time"`
	Details string `json:"details,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Time    string      `json:"time"`
}

// Authentication endpoints

// Register handles user registration
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate input
	if err := h.validateRequest(req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Hash password
	hashedPassword, err := h.passwordService.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("password hashing failed", "error", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "registration failed", "")
		return
	}

	// Create user
	user := &domain.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		IsActive:     true,
		IsVerified:   false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Register user through auth service
	if err := h.authService.Register(r.Context(), user, req.Password); err != nil {
		h.logger.Error("user registration failed", "error", err, "email", req.Email)
		h.writeErrorResponse(w, http.StatusConflict, "registration failed", err.Error())
		return
	}

	// Audit the registration
	_ = h.authService.AuditAction(r.Context(), &user.ID, "register", "user", &user.ID, nil, nil, h.getClientIP(r), r.Header.Get("User-Agent"))

	h.writeSuccessResponse(w, http.StatusCreated, "user registered successfully", map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	})
}

// Login handles user login
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate input
	if err := h.validateRequest(req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Authenticate user
	ipAddress := h.getClientIP(r)
	userAgent := r.Header.Get("User-Agent")
	
	loginResponse, err := h.authService.Login(r.Context(), req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		h.logger.Warn("login failed", "error", err, "email", req.Email, "ip", ipAddress)
		h.writeErrorResponse(w, http.StatusUnauthorized, "login failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "login successful", loginResponse)
}

// RefreshToken handles token refresh
func (h *AuthHandlers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate input
	if err := h.validateRequest(req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Refresh token
	loginResponse, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.logger.Warn("token refresh failed", "error", err, "ip", h.getClientIP(r))
		h.writeErrorResponse(w, http.StatusUnauthorized, "token refresh failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "token refreshed successfully", loginResponse)
}

// Logout handles user logout
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "authorization header required", "")
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid authorization header format", "")
		return
	}

	accessToken := parts[1]
	if err := h.authService.Logout(r.Context(), accessToken); err != nil {
		h.logger.Error("logout failed", "error", err, "ip", h.getClientIP(r))
		h.writeErrorResponse(w, http.StatusInternalServerError, "logout failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "logged out successfully", nil)
}

// LogoutAll handles logging out from all devices
func (h *AuthHandlers) LogoutAll(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	if user == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "authentication required", "")
		return
	}

	// TODO: Implement LogoutAll functionality in AuthenticationService
	h.logger.Info("logout all requested", "user_id", user.ID)

	h.writeSuccessResponse(w, http.StatusOK, "logged out from all devices", nil)
}

// User management endpoints

// GetProfile returns the current user's profile
func (h *AuthHandlers) GetProfile(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	if user == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "authentication required", "")
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "profile retrieved successfully", user)
}

// UpdateProfile updates the current user's profile
func (h *AuthHandlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	if user == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "authentication required", "")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate input
	if err := h.validateRequest(req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Update user
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Email != "" {
		user.Email = req.Email
	}

	if err := h.authService.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("profile update failed", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "profile update failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "profile updated successfully", user)
}

// ChangePassword handles password change
func (h *AuthHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	if user == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "authentication required", "")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate input
	if err := h.validateRequest(req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Change password
	if err := h.authService.ChangePassword(r.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
		h.logger.Error("password change failed", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusBadRequest, "password change failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "password changed successfully", nil)
}

// API Key management endpoints

// CreateApiKey creates a new API key
func (h *AuthHandlers) CreateApiKey(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	if user == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "authentication required", "")
		return
	}

	var req CreateApiKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate input
	if err := h.validateRequest(req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Create API key
	apiKey, err := h.authService.CreateApiKey(r.Context(), user.ID, req.Name, req.Scopes, req.ExpiresAt)
	if err != nil {
		h.logger.Error("API key creation failed", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "API key creation failed", err.Error())
		return
	}

	// Return the API key (this is the only time it will be shown)
	response := map[string]interface{}{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        apiKey.Key,
		"scopes":     apiKey.Scopes,
		"expires_at": apiKey.ExpiresAt,
		"created_at": apiKey.CreatedAt,
	}

	h.writeSuccessResponse(w, http.StatusCreated, "API key created successfully", response)
}

// ListApiKeys lists user's API keys
func (h *AuthHandlers) ListApiKeys(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	if user == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "authentication required", "")
		return
	}

	// Parse pagination parameters
	limit, offset := h.parsePaginationParams(r)

	// Get API keys
	apiKeys, err := h.authService.ListApiKeys(r.Context(), user.ID, limit, offset)
	if err != nil {
		h.logger.Error("API key listing failed", "error", err, "user_id", user.ID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "API key listing failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "API keys retrieved successfully", apiKeys)
}

// DeleteApiKey deletes an API key
func (h *AuthHandlers) DeleteApiKey(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	if user == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "authentication required", "")
		return
	}

	// Parse API key ID from URL
	vars := mux.Vars(r)
	apiKeyID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid API key ID", err.Error())
		return
	}

	// Delete API key
	if err := h.authService.DeleteApiKey(r.Context(), apiKeyID); err != nil {
		h.logger.Error("API key deletion failed", "error", err, "user_id", user.ID, "api_key_id", apiKeyID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "API key deletion failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "API key deleted successfully", nil)
}

// Admin endpoints (require admin role)

// ListUsers lists all users (admin only)
func (h *AuthHandlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	limit, offset := h.parsePaginationParams(r)

	// Get users
	users, err := h.authService.ListUsers(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("user listing failed", "error", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "user listing failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "users retrieved successfully", users)
}

// GetUser gets a specific user (admin only)
func (h *AuthHandlers) GetUser(w http.ResponseWriter, r *http.Request) {
	// Parse user ID from URL
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid user ID", err.Error())
		return
	}

	// Get user
	user, err := h.authService.GetUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("user retrieval failed", "error", err, "user_id", userID)
		h.writeErrorResponse(w, http.StatusNotFound, "user not found", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "user retrieved successfully", user)
}

// DeleteUser deletes a user (admin only)
func (h *AuthHandlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Parse user ID from URL
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid user ID", err.Error())
		return
	}

	// Delete user
	if err := h.authService.DeleteUser(r.Context(), userID); err != nil {
		h.logger.Error("user deletion failed", "error", err, "user_id", userID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "user deletion failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "user deleted successfully", nil)
}

// AssignRole assigns a role to a user (admin only)
func (h *AuthHandlers) AssignRole(w http.ResponseWriter, r *http.Request) {
	// Parse user ID from URL
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid user ID", err.Error())
		return
	}

	var req AssignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate input
	if err := h.validateRequest(req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Assign role
	if err := h.authService.AssignRole(r.Context(), userID, req.RoleID); err != nil {
		h.logger.Error("role assignment failed", "error", err, "user_id", userID, "role_id", req.RoleID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "role assignment failed", err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "role assigned successfully", nil)
}

// Helper methods

func (h *AuthHandlers) writeErrorResponse(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:   message,
		Code:    statusCode,
		Time:    time.Now().UTC().Format(time.RFC3339),
		Details: details,
	}

	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandlers) writeSuccessResponse(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := SuccessResponse{
		Message: message,
		Data:    data,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandlers) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

func (h *AuthHandlers) getUserFromContext(r *http.Request) *domain.User {
	user, ok := r.Context().Value("user").(*domain.User)
	if !ok {
		return nil
	}
	return user
}

func (h *AuthHandlers) parsePaginationParams(r *http.Request) (int, int) {
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

func (h *AuthHandlers) validateRequest(req interface{}) error {
	// This is a placeholder for request validation
	// In a real implementation, you would use a validation library like go-playground/validator
	return nil
}

// SetupAuthRoutes sets up all authentication routes
func (h *AuthHandlers) SetupAuthRoutes(router *mux.Router) {
	// Authentication routes
	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", h.Register).Methods("POST")
	authRouter.HandleFunc("/login", h.Login).Methods("POST")
	authRouter.HandleFunc("/refresh", h.RefreshToken).Methods("POST")
	authRouter.HandleFunc("/logout", h.Logout).Methods("POST")
	authRouter.HandleFunc("/logout-all", h.LogoutAll).Methods("POST")

	// User profile routes
	userRouter := router.PathPrefix("/user").Subrouter()
	userRouter.HandleFunc("/profile", h.GetProfile).Methods("GET")
	userRouter.HandleFunc("/profile", h.UpdateProfile).Methods("PUT")
	userRouter.HandleFunc("/change-password", h.ChangePassword).Methods("POST")

	// API key routes
	apiKeyRouter := router.PathPrefix("/api-keys").Subrouter()
	apiKeyRouter.HandleFunc("", h.CreateApiKey).Methods("POST")
	apiKeyRouter.HandleFunc("", h.ListApiKeys).Methods("GET")
	apiKeyRouter.HandleFunc("/{id}", h.DeleteApiKey).Methods("DELETE")

	// Admin routes
	adminRouter := router.PathPrefix("/admin").Subrouter()
	adminRouter.HandleFunc("/users", h.ListUsers).Methods("GET")
	adminRouter.HandleFunc("/users/{id}", h.GetUser).Methods("GET")
	adminRouter.HandleFunc("/users/{id}", h.DeleteUser).Methods("DELETE")
	adminRouter.HandleFunc("/users/{id}/roles", h.AssignRole).Methods("POST")
}

// HealthCheck provides a health check endpoint
func (h *AuthHandlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "authentication",
	}

	h.writeSuccessResponse(w, http.StatusOK, "service is healthy", health)
}