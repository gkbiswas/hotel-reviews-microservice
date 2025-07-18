package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
)

// This example demonstrates how to use the authentication system
// in the hotel reviews microservice

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Initialize database
	db, err := gorm.Open(postgres.Open(cfg.GetDatabaseURL()), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate authentication tables
	err = db.AutoMigrate(
		&domain.User{},
		&domain.Role{},
		&domain.Permission{},
		&domain.Session{},
		&domain.ApiKey{},
		&domain.AuditLog{},
		&domain.LoginAttempt{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize circuit breaker and retry mechanisms
	circuitBreaker := infrastructure.NewCircuitBreaker("auth", 5, 10*time.Second)
	retryPolicy := infrastructure.NewRetryPolicy(3, 100*time.Millisecond, 2.0)

	// Initialize authentication components
	authRepo := infrastructure.NewAuthRepository(db, logger, circuitBreaker, retryPolicy)
	jwtService := infrastructure.NewJWTService(cfg, logger, circuitBreaker, retryPolicy)
	passwordService := infrastructure.NewPasswordService(logger)
	rbacService := infrastructure.NewRBACService(authRepo, logger, circuitBreaker, retryPolicy)
	apiKeyService := infrastructure.NewApiKeyService(authRepo, logger, circuitBreaker, retryPolicy)
	rateLimitService := infrastructure.NewRateLimitService(authRepo, logger, circuitBreaker, retryPolicy)
	auditService := infrastructure.NewAuditService(authRepo, logger, circuitBreaker, retryPolicy)

	// Initialize comprehensive authentication service
	authService := infrastructure.NewAuthenticationService(
		authRepo,
		jwtService,
		passwordService,
		rbacService,
		apiKeyService,
		rateLimitService,
		auditService,
		logger,
		circuitBreaker,
		retryPolicy,
	)

	// Initialize authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, logger)
	authChain := middleware.NewAuthMiddlewareChain(authMiddleware)

	// Initialize authentication handlers
	authHandlers := application.NewAuthHandlers(authService, passwordService, logger)

	// Setup HTTP server
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", authHandlers.HealthCheck).Methods("GET")

	// Authentication routes (public)
	authRouter := router.PathPrefix("/api/v1/auth").Subrouter()
	authRouter.Use(authChain.ForAuthEndpoints())
	authHandlers.SetupAuthRoutes(authRouter)

	// Protected routes (requires authentication)
	protectedRouter := router.PathPrefix("/api/v1/protected").Subrouter()
	protectedRouter.Use(authChain.ForProtectedEndpoints())
	protectedRouter.HandleFunc("/profile", authHandlers.GetProfile).Methods("GET")
	protectedRouter.HandleFunc("/profile", authHandlers.UpdateProfile).Methods("PUT")

	// Admin routes (requires admin role)
	adminRouter := router.PathPrefix("/api/v1/admin").Subrouter()
	adminRouter.Use(authChain.ForAdminEndpoints())
	adminRouter.HandleFunc("/users", authHandlers.ListUsers).Methods("GET")
	adminRouter.HandleFunc("/users/{id}", authHandlers.GetUser).Methods("GET")
	adminRouter.HandleFunc("/users/{id}", authHandlers.DeleteUser).Methods("DELETE")

	// Service routes (requires API key)
	serviceRouter := router.PathPrefix("/api/v1/service").Subrouter()
	serviceRouter.Use(authChain.ForServiceEndpoints())
	serviceRouter.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}).Methods("GET")

	// Example usage of authentication system
	go func() {
		// Wait for server to start
		time.Sleep(2 * time.Second)
		
		// Demonstrate authentication system usage
		demonstrateAuthSystem(authService, logger)
	}()

	// Start HTTP server
	logger.Info("Starting HTTP server", "address", cfg.GetServerAddress())
	if err := http.ListenAndServe(cfg.GetServerAddress(), router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func demonstrateAuthSystem(authService *infrastructure.AuthenticationService, logger *slog.Logger) {
	ctx := context.Background()

	logger.Info("=== Authentication System Demo ===")

	// 1. Create default roles and permissions
	logger.Info("Creating default roles and permissions...")
	err := setupDefaultRolesAndPermissions(ctx, authService, logger)
	if err != nil {
		logger.Error("Failed to setup default roles and permissions", "error", err)
		return
	}

	// 2. Register a new user
	logger.Info("Registering a new user...")
	user := &domain.User{
		ID:        uuid.New(),
		Username:  "john_doe",
		Email:     "john@example.com",
		FirstName: "John",
		LastName:  "Doe",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = authService.Register(ctx, user, "SecurePassword123!")
	if err != nil {
		logger.Error("Failed to register user", "error", err)
		return
	}
	logger.Info("User registered successfully", "user_id", user.ID, "email", user.Email)

	// 3. Login user
	logger.Info("Logging in user...")
	loginResponse, err := authService.Login(ctx, user.Email, "SecurePassword123!", "127.0.0.1", "Demo-Client/1.0")
	if err != nil {
		logger.Error("Failed to login user", "error", err)
		return
	}
	logger.Info("User logged in successfully", "access_token", loginResponse.AccessToken[:20]+"...")

	// 4. Validate token
	logger.Info("Validating access token...")
	validatedUser, err := authService.ValidateToken(ctx, loginResponse.AccessToken)
	if err != nil {
		logger.Error("Failed to validate token", "error", err)
		return
	}
	logger.Info("Token validated successfully", "user_id", validatedUser.ID)

	// 5. Check permissions
	logger.Info("Checking user permissions...")
	hasPermission, err := authService.CheckPermission(ctx, user.ID, "reviews", "read")
	if err != nil {
		logger.Error("Failed to check permission", "error", err)
		return
	}
	logger.Info("Permission check result", "has_permission", hasPermission)

	// 6. Create API key
	logger.Info("Creating API key...")
	scopes := []string{"reviews.read", "hotels.read"}
	expiresAt := time.Now().Add(30 * 24 * time.Hour) // 30 days
	apiKey, err := authService.CreateApiKey(ctx, user.ID, "Demo API Key", scopes, &expiresAt)
	if err != nil {
		logger.Error("Failed to create API key", "error", err)
		return
	}
	logger.Info("API key created successfully", "api_key", apiKey.Key[:20]+"...")

	// 7. Validate API key
	logger.Info("Validating API key...")
	validatedApiKey, err := authService.ValidateApiKey(ctx, apiKey.Key)
	if err != nil {
		logger.Error("Failed to validate API key", "error", err)
		return
	}
	logger.Info("API key validated successfully", "api_key_id", validatedApiKey.ID)

	// 8. Refresh token
	logger.Info("Refreshing token...")
	refreshResponse, err := authService.RefreshToken(ctx, loginResponse.RefreshToken)
	if err != nil {
		logger.Error("Failed to refresh token", "error", err)
		return
	}
	logger.Info("Token refreshed successfully", "new_access_token", refreshResponse.AccessToken[:20]+"...")

	// 9. Logout
	logger.Info("Logging out user...")
	err = authService.Logout(ctx, refreshResponse.AccessToken)
	if err != nil {
		logger.Error("Failed to logout user", "error", err)
		return
	}
	logger.Info("User logged out successfully")

	logger.Info("=== Authentication System Demo Complete ===")
}

func setupDefaultRolesAndPermissions(ctx context.Context, authService *infrastructure.AuthenticationService, logger *slog.Logger) error {
	// Create default roles
	roles := []domain.Role{
		{
			ID:          uuid.New(),
			Name:        "admin",
			Description: "Administrator with full system access",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "user",
			Description: "Regular user with limited access",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "moderator",
			Description: "Moderator with content management privileges",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, role := range roles {
		err := authService.CreateRole(ctx, &role)
		if err != nil {
			logger.Warn("Role already exists or failed to create", "role", role.Name, "error", err)
		}
	}

	// Create default permissions
	permissions := []domain.Permission{
		{
			ID:          uuid.New(),
			Name:        "reviews.read",
			Resource:    "reviews",
			Action:      "read",
			Description: "View reviews",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "reviews.create",
			Resource:    "reviews",
			Action:      "create",
			Description: "Create reviews",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "hotels.read",
			Resource:    "hotels",
			Action:      "read",
			Description: "View hotels",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "users.read",
			Resource:    "users",
			Action:      "read",
			Description: "View users",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "users.update",
			Resource:    "users",
			Action:      "update",
			Description: "Update users",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "users.delete",
			Resource:    "users",
			Action:      "delete",
			Description: "Delete users",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, permission := range permissions {
		err := authService.CreatePermission(ctx, &permission)
		if err != nil {
			logger.Warn("Permission already exists or failed to create", "permission", permission.Name, "error", err)
		}
	}

	// Get created roles and permissions for assignment
	userRole, err := authService.GetRole(ctx, roles[1].ID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %w", err)
	}

	// Assign permissions to user role
	reviewsReadPerm, err := authService.GetPermission(ctx, permissions[0].ID)
	if err != nil {
		return fmt.Errorf("failed to get reviews.read permission: %w", err)
	}

	hotelsReadPerm, err := authService.GetPermission(ctx, permissions[2].ID)
	if err != nil {
		return fmt.Errorf("failed to get hotels.read permission: %w", err)
	}

	// Assign permissions to user role
	err = authService.AssignPermission(ctx, userRole.ID, reviewsReadPerm.ID)
	if err != nil {
		logger.Warn("Failed to assign reviews.read permission to user role", "error", err)
	}

	err = authService.AssignPermission(ctx, userRole.ID, hotelsReadPerm.ID)
	if err != nil {
		logger.Warn("Failed to assign hotels.read permission to user role", "error", err)
	}

	return nil
}

// HTTP Client example for testing authentication endpoints
func exampleHTTPClient() {
	// Example API calls using the authentication system
	
	// 1. Register a user
	registerPayload := `{
		"username": "jane_doe",
		"email": "jane@example.com",
		"password": "SecurePassword123!",
		"first_name": "Jane",
		"last_name": "Doe"
	}`
	
	// POST /api/v1/auth/register
	fmt.Println("Register:", registerPayload)
	
	// 2. Login
	loginPayload := `{
		"email": "jane@example.com",
		"password": "SecurePassword123!"
	}`
	
	// POST /api/v1/auth/login
	fmt.Println("Login:", loginPayload)
	
	// 3. Access protected endpoint
	// GET /api/v1/protected/profile
	// Headers: Authorization: Bearer <access_token>
	
	// 4. Use API key
	// GET /api/v1/service/status
	// Headers: X-API-Key: <api_key>
	
	// 5. Refresh token
	refreshPayload := `{
		"refresh_token": "<refresh_token>"
	}`
	
	// POST /api/v1/auth/refresh
	fmt.Println("Refresh token:", refreshPayload)
	
	// 6. Logout
	// POST /api/v1/auth/logout
	// Headers: Authorization: Bearer <access_token>
}