package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
)

// AuthMiddleware handles authentication for HTTP requests
type AuthMiddleware struct {
	authService *infrastructure.AuthenticationService
	logger      *slog.Logger
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authService *infrastructure.AuthenticationService, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		logger:      logger,
	}
}

// JWTAuthMiddleware validates JWT tokens for protected routes
func (m *AuthMiddleware) JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.writeErrorResponse(w, http.StatusUnauthorized, "authorization header required")
			return
		}

		// Check if it's a Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			m.writeErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		token := parts[1]
		if token == "" {
			m.writeErrorResponse(w, http.StatusUnauthorized, "token is required")
			return
		}

		// Validate token
		user, err := m.authService.ValidateToken(r.Context(), token)
		if err != nil {
			m.logger.Warn("token validation failed", "error", err, "ip", m.getClientIP(r))
			m.writeErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), "user", user)
		ctx = context.WithValue(ctx, "user_id", user.ID)
		ctx = context.WithValue(ctx, "user_email", user.Email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// APIKeyAuthMiddleware validates API keys for service-to-service calls
func (m *AuthMiddleware) APIKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from X-API-Key header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			m.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
			return
		}

		// Validate API key
		apiKeyData, err := m.authService.ValidateApiKey(r.Context(), apiKey)
		if err != nil {
			m.logger.Warn("API key validation failed", "error", err, "ip", m.getClientIP(r))
			m.writeErrorResponse(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		// Add API key data to context
		ctx := context.WithValue(r.Context(), "api_key", apiKeyData)
		ctx = context.WithValue(ctx, "user_id", apiKeyData.UserID)
		ctx = context.WithValue(ctx, "scopes", apiKeyData.Scopes)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission creates middleware that requires specific permissions
func (m *AuthMiddleware) RequirePermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (set by JWTAuthMiddleware)
			user, ok := r.Context().Value("user").(*domain.User)
			if !ok {
				m.writeErrorResponse(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Check permission
			hasPermission, err := m.authService.CheckPermission(r.Context(), user.ID, resource, action)
			if err != nil {
				m.logger.Error("permission check failed", "error", err, "user_id", user.ID, "resource", resource, "action", action)
				m.writeErrorResponse(w, http.StatusInternalServerError, "permission check failed")
				return
			}

			if !hasPermission {
				m.logger.Warn("permission denied", "user_id", user.ID, "resource", resource, "action", action, "ip", m.getClientIP(r))
				m.writeErrorResponse(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole creates middleware that requires specific roles
func (m *AuthMiddleware) RequireRole(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (set by JWTAuthMiddleware)
			user, ok := r.Context().Value("user").(*domain.User)
			if !ok {
				m.writeErrorResponse(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			userRoles := make(map[string]bool)
			for _, role := range user.Roles {
				userRoles[role.Name] = true
			}

			for _, requiredRole := range requiredRoles {
				if userRoles[requiredRole] {
					hasRole = true
					break
				}
			}

			if !hasRole {
				m.logger.Warn("role check failed", "user_id", user.ID, "required_roles", requiredRoles, "user_roles", userRoles, "ip", m.getClientIP(r))
				m.writeErrorResponse(w, http.StatusForbidden, "insufficient role permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAPIKeyScopes creates middleware that requires specific API key scopes
func (m *AuthMiddleware) RequireAPIKeyScopes(requiredScopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key scopes from context (set by APIKeyAuthMiddleware)
			scopes, ok := r.Context().Value("scopes").([]string)
			if !ok {
				m.writeErrorResponse(w, http.StatusUnauthorized, "API key authentication required")
				return
			}

			// Check if API key has required scopes
			apiKeyScopes := make(map[string]bool)
			for _, scope := range scopes {
				apiKeyScopes[scope] = true
			}

			for _, requiredScope := range requiredScopes {
				if !apiKeyScopes[requiredScope] {
					m.logger.Warn("API key scope check failed", "required_scopes", requiredScopes, "api_key_scopes", scopes, "ip", m.getClientIP(r))
					m.writeErrorResponse(w, http.StatusForbidden, "insufficient API key permissions")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware implements rate limiting for authentication endpoints
func (m *AuthMiddleware) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply rate limiting to authentication endpoints
		if !m.isAuthEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract email from request body for login attempts
		email := m.extractEmailFromRequest(r)
		if email == "" {
			// If we can't extract email, skip rate limiting
			next.ServeHTTP(w, r)
			return
		}

		ipAddress := m.getClientIP(r)

		// Check if rate limited
		isRateLimited, err := m.authService.IsRateLimited(r.Context(), email, ipAddress)
		if err != nil {
			m.logger.Error("rate limit check failed", "error", err, "email", email, "ip", ipAddress)
			// Continue on error to avoid blocking legitimate requests
			next.ServeHTTP(w, r)
			return
		}

		if isRateLimited {
			m.logger.Warn("rate limit exceeded", "email", email, "ip", ipAddress)
			m.writeErrorResponse(w, http.StatusTooManyRequests, "too many login attempts, please try again later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuditMiddleware logs authentication-related actions
func (m *AuthMiddleware) AuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures the status code
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(wrappedWriter, r)

		// Log audit information
		go func() {
			ctx := context.Background()

			var userID *uuid.UUID
			if user, ok := r.Context().Value("user").(*domain.User); ok {
				userID = &user.ID
			}

			// Extract resource and action from URL path
			resource, action := m.extractResourceAndAction(r.URL.Path, r.Method)

			// Skip audit for health checks and metrics
			if resource == "health" || resource == "metrics" {
				return
			}

			err := m.authService.AuditAction(
				ctx,
				userID,
				action,
				resource,
				nil, // resourceID - could be extracted from URL params
				nil, // oldValues
				nil, // newValues
				m.getClientIP(r),
				r.Header.Get("User-Agent"),
			)

			if err != nil {
				m.logger.Error("audit logging failed", "error", err)
			}
		}()

		// Log request details
		m.logger.Info("request processed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrappedWriter.statusCode,
			"duration", time.Since(start),
			"ip", m.getClientIP(r),
		)
	})
}

// OptionalAuthMiddleware provides optional authentication (doesn't fail if no auth)
func (m *AuthMiddleware) OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try JWT authentication first
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				token := parts[1]
				if token != "" {
					user, err := m.authService.ValidateToken(r.Context(), token)
					if err == nil {
						ctx := context.WithValue(r.Context(), "user", user)
						ctx = context.WithValue(ctx, "user_id", user.ID)
						ctx = context.WithValue(ctx, "user_email", user.Email)
						r = r.WithContext(ctx)
					}
				}
			}
		}

		// Try API key authentication
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			apiKeyData, err := m.authService.ValidateApiKey(r.Context(), apiKey)
			if err == nil {
				ctx := context.WithValue(r.Context(), "api_key", apiKeyData)
				ctx = context.WithValue(ctx, "user_id", apiKeyData.UserID)
				ctx = context.WithValue(ctx, "scopes", apiKeyData.Scopes)
				r = r.WithContext(ctx)
			}
		}

		next.ServeHTTP(w, r)
	})
}

// Helper methods

func (m *AuthMiddleware) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error": message,
		"code":  statusCode,
		"time":  time.Now().UTC().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (m *AuthMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
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

func (m *AuthMiddleware) isAuthEndpoint(path string) bool {
	authEndpoints := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/refresh",
		"/api/v1/auth/forgot-password",
		"/api/v1/auth/reset-password",
	}

	for _, endpoint := range authEndpoints {
		if path == endpoint {
			return true
		}
	}

	return false
}

func (m *AuthMiddleware) extractEmailFromRequest(r *http.Request) string {
	// This is a simplified implementation
	// In practice, you might want to parse the request body based on content type
	if r.Header.Get("Content-Type") == "application/json" {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return ""
		}

		if email, ok := body["email"].(string); ok {
			return email
		}
	}

	return ""
}

func (m *AuthMiddleware) extractResourceAndAction(path, method string) (string, string) {
	// Extract resource from URL path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		return "unknown", strings.ToLower(method)
	}

	// Skip "api" and "v1" parts
	resource := parts[2]
	if len(parts) > 3 && parts[3] != "" {
		resource = parts[3]
	}

	// Map HTTP methods to actions
	actionMap := map[string]string{
		"GET":    "read",
		"POST":   "create",
		"PUT":    "update",
		"PATCH":  "update",
		"DELETE": "delete",
	}

	action := actionMap[strings.ToUpper(method)]
	if action == "" {
		action = strings.ToLower(method)
	}

	return resource, action
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// AuthMiddlewareChain provides a convenient way to chain authentication middlewares
type AuthMiddlewareChain struct {
	middleware *AuthMiddleware
}

// NewAuthMiddlewareChain creates a new middleware chain
func NewAuthMiddlewareChain(middleware *AuthMiddleware) *AuthMiddlewareChain {
	return &AuthMiddlewareChain{middleware: middleware}
}

// ForPublicEndpoints applies middleware for public endpoints (audit only)
func (c *AuthMiddlewareChain) ForPublicEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.OptionalAuthMiddleware(next),
		)
	}
}

// ForProtectedEndpoints applies middleware for protected endpoints (requires authentication)
func (c *AuthMiddlewareChain) ForProtectedEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.JWTAuthMiddleware(next),
		)
	}
}

// ForServiceEndpoints applies middleware for service-to-service endpoints (requires API key)
func (c *AuthMiddlewareChain) ForServiceEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.APIKeyAuthMiddleware(next),
		)
	}
}

// ForAuthEndpoints applies middleware for authentication endpoints (includes rate limiting)
func (c *AuthMiddlewareChain) ForAuthEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.RateLimitMiddleware(next),
		)
	}
}

// ForAdminEndpoints applies middleware for admin endpoints (requires admin role)
func (c *AuthMiddlewareChain) ForAdminEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.RequireRole("admin")(
				c.middleware.JWTAuthMiddleware(next),
			),
		)
	}
}

// WithPermission adds permission requirement to any middleware chain
func (c *AuthMiddlewareChain) WithPermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.RequirePermission(resource, action)(next)
	}
}

// WithRole adds role requirement to any middleware chain
func (c *AuthMiddlewareChain) WithRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.RequireRole(roles...)(next)
	}
}

// WithAPIKeyScopes adds API key scope requirement to any middleware chain
func (c *AuthMiddlewareChain) WithAPIKeyScopes(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.RequireAPIKeyScopes(scopes...)(next)
	}
}

// SetupRoutes demonstrates how to use the middleware with different route types
func (c *AuthMiddlewareChain) SetupRoutes(router *mux.Router) {
	// Public routes (no authentication required)
	publicRouter := router.PathPrefix("/api/v1/public").Subrouter()
	publicRouter.Use(c.ForPublicEndpoints())

	// Authentication routes (with rate limiting)
	authRouter := router.PathPrefix("/api/v1/auth").Subrouter()
	authRouter.Use(c.ForAuthEndpoints())

	// Protected routes (requires authentication)
	protectedRouter := router.PathPrefix("/api/v1/protected").Subrouter()
	protectedRouter.Use(c.ForProtectedEndpoints())

	// Service routes (requires API key)
	serviceRouter := router.PathPrefix("/api/v1/service").Subrouter()
	serviceRouter.Use(c.ForServiceEndpoints())

	// Admin routes (requires admin role)
	adminRouter := router.PathPrefix("/api/v1/admin").Subrouter()
	adminRouter.Use(c.ForAdminEndpoints())

	// Routes with specific permissions
	reviewRouter := router.PathPrefix("/api/v1/reviews").Subrouter()
	reviewRouter.Use(c.ForProtectedEndpoints())
	reviewRouter.Use(c.WithPermission("reviews", "read"))
}
