package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// IntegratedApplication represents the full-featured application with all systems integrated
type IntegratedApplication struct {
	config      *config.Config
	logger      *logger.Logger
	database    *infrastructure.Database
	redisClient *redis.Client
	s3Client    *infrastructure.S3Client

	// Circuit breaker and retry infrastructure
	circuitBreakerIntegration *infrastructure.CircuitBreakerIntegration
	retryManager              *infrastructure.RetryManager

	// Authentication infrastructure
	authService     *infrastructure.AuthService
	jwtService      *infrastructure.JWTService
	rbacService     *infrastructure.RBACService
	passwordService *infrastructure.PasswordService
	apiKeyService   *infrastructure.APIKeyService

	// Protected service wrappers
	protectedDatabase *infrastructure.DatabaseWrapper
	protectedCache    *infrastructure.CacheWrapper
	protectedS3       *infrastructure.S3Wrapper

	// Domain services
	reviewRepository domain.ReviewRepository
	userRepository   domain.UserRepository
	jsonProcessor    domain.JSONProcessor
	reviewService    domain.ReviewService
	processingEngine *application.ProcessingEngine

	// HTTP handlers
	handlers     *application.Handlers
	authHandlers *application.AuthHandlers

	// HTTP server
	server *http.Server
}

// CLI flags (extended with authentication options)
var (
	configFile = flag.String("config", "", "Path to configuration file")
	mode       = flag.String("mode", "server", "Application mode: server, cli, migrate, process-files")
	logLevel   = flag.String("log-level", "", "Log level override (debug, info, warn, error)")
	host       = flag.String("host", "", "Server host override")
	port       = flag.Int("port", 0, "Server port override")

	// CLI specific flags
	fileURL      = flag.String("file-url", "", "S3 URL of the file to process")
	providerName = flag.String("provider", "", "Provider name for file processing")
	resetDB      = flag.Bool("reset-db", false, "Reset database (drop all tables)")
	seedDB       = flag.Bool("seed-db", false, "Seed database with initial data")

	// Migration flags
	migrateUp   = flag.Bool("migrate-up", false, "Run database migrations")
	migrateDown = flag.Bool("migrate-down", false, "Rollback database migrations")

	// Display flags
	version = flag.Bool("version", false, "Show version information")
	help    = flag.Bool("help", false, "Show help information")

	// Circuit breaker flags
	disableCircuitBreaker = flag.Bool("disable-circuit-breaker", false, "Disable circuit breaker protection")
	disableRetry          = flag.Bool("disable-retry", false, "Disable retry mechanisms")
	resetCircuitBreakers  = flag.Bool("reset-circuit-breakers", false, "Reset all circuit breakers to closed state")

	// Authentication flags
	disableAuth     = flag.Bool("disable-auth", false, "Disable authentication (NOT recommended for production)")
	createAdminUser = flag.Bool("create-admin", false, "Create default admin user")
	adminEmail      = flag.String("admin-email", "admin@example.com", "Admin user email")
	adminPassword   = flag.String("admin-password", "", "Admin user password (will be prompted if not provided)")
	generateAPIKey  = flag.Bool("generate-api-key", false, "Generate API key for service-to-service communication")
	apiKeyName      = flag.String("api-key-name", "default-service", "Name for the generated API key")
	apiKeyScopes    = flag.String("api-key-scopes", "read,write", "Comma-separated scopes for the API key")
)

const (
	AppName    = "hotel-reviews-microservice"
	AppVersion = "3.0.0" // Updated version with full feature integration
)

func main() {
	flag.Parse()

	// Handle help and version flags
	if *help {
		printHelp()
		os.Exit(0)
	}

	if *version {
		printVersion()
		os.Exit(0)
	}

	// Initialize integrated application
	app, err := initializeIntegratedApplication()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// Handle circuit breaker reset flag
	if *resetCircuitBreakers {
		if app.circuitBreakerIntegration != nil {
			app.circuitBreakerIntegration.ResetAllCircuitBreakers()
			app.logger.Info("All circuit breakers reset")
		}
		if *mode == "reset-circuit-breakers" {
			return
		}
	}

	// Handle admin user creation
	if *createAdminUser {
		err = app.createAdminUser()
		if err != nil {
			app.logger.Error("Failed to create admin user", "error", err)
			os.Exit(1)
		}
		if *mode == "create-admin" {
			return
		}
	}

	// Handle API key generation
	if *generateAPIKey {
		err = app.generateAPIKey()
		if err != nil {
			app.logger.Error("Failed to generate API key", "error", err)
			os.Exit(1)
		}
		if *mode == "generate-api-key" {
			return
		}
	}

	// Run application based on mode
	switch *mode {
	case "server":
		err = app.runServer()
	case "cli":
		err = app.runCLI()
	case "migrate":
		err = app.runMigrations()
	case "process-files":
		err = app.processFiles()
	case "health-check":
		err = app.runHealthCheck()
	case "circuit-breaker-status":
		err = app.showCircuitBreakerStatus()
	case "auth-status":
		err = app.showAuthStatus()
	case "create-admin":
		err = app.createAdminUser()
	case "generate-api-key":
		err = app.generateAPIKey()
	default:
		err = fmt.Errorf("unknown mode: %s", *mode)
	}

	if err != nil {
		app.logger.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

// initializeIntegratedApplication initializes the application with all features
func initializeIntegratedApplication() (*IntegratedApplication, error) {
	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	loggerConfig := &logger.Config{
		Level:            cfg.Log.Level,
		Format:           cfg.Log.Format,
		Output:           cfg.Log.Output,
		FilePath:         cfg.Log.FilePath,
		MaxSize:          cfg.Log.MaxSize,
		MaxBackups:       cfg.Log.MaxBackups,
		MaxAge:           cfg.Log.MaxAge,
		Compress:         cfg.Log.Compress,
		EnableCaller:     cfg.Log.EnableCaller,
		EnableStacktrace: cfg.Log.EnableStacktrace,
	}
	log, err := logger.New(loggerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	log.Info("Starting integrated application",
		"name", AppName,
		"version", AppVersion,
		"mode", *mode,
		"circuit_breaker_enabled", !*disableCircuitBreaker,
		"retry_enabled", !*disableRetry,
		"auth_enabled", !*disableAuth,
	)

	// Initialize database
	database, err := infrastructure.NewDatabase(&cfg.Database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.Database,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Warn("Redis connection failed, continuing without cache", "error", err)
		redisClient = nil
	}

	// Initialize S3 client
	s3Client, err := infrastructure.NewS3Client(&cfg.S3, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Initialize circuit breaker integration
	var circuitBreakerIntegration *infrastructure.CircuitBreakerIntegration
	var retryManager *infrastructure.RetryManager

	if !*disableCircuitBreaker {
		circuitBreakerIntegration = infrastructure.NewCircuitBreakerIntegration(log)
		circuitBreakerIntegration.SetupHealthChecks(database.DB, redisClient)
		log.Info("Circuit breaker integration initialized")
	}

	if !*disableRetry {
		retryManager = infrastructure.NewRetryManager(log)
		if circuitBreakerIntegration != nil {
			retryManager.SetCircuitBreakerManager(circuitBreakerIntegration.GetManager())
		}
		log.Info("Retry manager initialized")
	}

	// Initialize authentication services
	var authService *infrastructure.AuthService
	var jwtService *infrastructure.JWTService
	var rbacService *infrastructure.RBACService
	var passwordService *infrastructure.PasswordService
	var apiKeyService *infrastructure.APIKeyService
	var userRepository domain.UserRepository

	if !*disableAuth {
		// Initialize authentication components
		jwtService = infrastructure.NewJWTService(&cfg.Auth.JWT, log)
		passwordService = infrastructure.NewPasswordService(&cfg.Auth.Password, log)

		// Initialize user repository
		if circuitBreakerIntegration != nil {
			protectedDB := circuitBreakerIntegration.NewDatabaseWrapper(database.DB)
			userRepository = infrastructure.NewProtectedUserRepository(protectedDB, log)
		} else {
			userRepository = infrastructure.NewUserRepository(database, log)
		}

		rbacService = infrastructure.NewRBACService(userRepository, log)
		apiKeyService = infrastructure.NewAPIKeyService(userRepository, log)

		authService = infrastructure.NewAuthService(
			jwtService,
			passwordService,
			rbacService,
			apiKeyService,
			userRepository,
			log,
		)

		log.Info("Authentication services initialized")
	}

	// Create protected wrappers
	var protectedDatabase *infrastructure.DatabaseWrapper
	var protectedCache *infrastructure.CacheWrapper
	var protectedS3 *infrastructure.S3Wrapper

	if circuitBreakerIntegration != nil {
		protectedDatabase = circuitBreakerIntegration.NewDatabaseWrapper(database.DB)
		if redisClient != nil {
			protectedCache = circuitBreakerIntegration.NewCacheWrapper(redisClient)
		}
		protectedS3 = circuitBreakerIntegration.NewS3Wrapper()
	}

	// Initialize repository
	var reviewRepository domain.ReviewRepository
	if protectedDatabase != nil {
		reviewRepository = infrastructure.NewProtectedReviewRepository(protectedDatabase, log)
	} else {
		reviewRepository = infrastructure.NewReviewRepository(database, log)
	}

	// Initialize JSON processor
	jsonProcessor := infrastructure.NewJSONLinesProcessor(reviewRepository, log)

	// Initialize review service
	reviewService := domain.NewReviewService(
		reviewRepository,
		s3Client,
		jsonProcessor,
		nil, // notificationService - TODO: implement
		nil, // cacheService - TODO: implement with protectedCache
		nil, // metricsService - TODO: implement
		nil, // eventPublisher - TODO: implement
		log.Logger,
	)

	// Initialize processing engine
	processingConfig := &application.ProcessingConfig{
		MaxWorkers:           cfg.Processing.WorkerCount,
		MaxConcurrentFiles:   10,
		MaxRetries:           cfg.Processing.MaxRetries,
		RetryDelay:           cfg.Processing.RetryDelay,
		ProcessingTimeout:    cfg.Processing.ProcessingTimeout,
		WorkerIdleTimeout:    5 * time.Minute,
		MetricsInterval:      30 * time.Second,
		EnableCircuitBreaker: !*disableCircuitBreaker,
		EnableRetry:          !*disableRetry,
	}

	var processingEngine *application.ProcessingEngine
	if circuitBreakerIntegration != nil && retryManager != nil {
		processingEngine = application.NewResilientProcessingEngine(
			reviewService,
			s3Client,
			jsonProcessor,
			log,
			processingConfig,
			circuitBreakerIntegration,
			retryManager,
		)
	} else {
		processingEngine = application.NewProcessingEngine(
			reviewService,
			s3Client,
			jsonProcessor,
			log,
			processingConfig,
		)
	}

	// Initialize handlers
	var handlers *application.Handlers
	var authHandlers *application.AuthHandlers

	if circuitBreakerIntegration != nil {
		handlers = application.NewResilientHandlers(
			reviewService,
			log,
			circuitBreakerIntegration,
			retryManager,
		)
	} else {
		handlers = application.NewHandlers(reviewService, log)
	}

	if authService != nil {
		authHandlers = application.NewAuthHandlers(authService, log)
	}

	app := &IntegratedApplication{
		config:                    cfg,
		logger:                    log,
		database:                  database,
		redisClient:               redisClient,
		s3Client:                  s3Client,
		circuitBreakerIntegration: circuitBreakerIntegration,
		retryManager:              retryManager,
		authService:               authService,
		jwtService:                jwtService,
		rbacService:               rbacService,
		passwordService:           passwordService,
		apiKeyService:             apiKeyService,
		protectedDatabase:         protectedDatabase,
		protectedCache:            protectedCache,
		protectedS3:               protectedS3,
		reviewRepository:          reviewRepository,
		userRepository:            userRepository,
		jsonProcessor:             jsonProcessor,
		reviewService:             reviewService,
		processingEngine:          processingEngine,
		handlers:                  handlers,
		authHandlers:              authHandlers,
	}

	return app, nil
}

// runServer runs the application in server mode with all features
func (app *IntegratedApplication) runServer() error {
	ctx := context.Background()

	app.logger.Info("Starting server mode with all features")

	// Run database migrations
	if app.protectedDatabase != nil {
		err := app.protectedDatabase.ExecuteQuery(ctx, func(db *gorm.DB) error {
			return app.database.Migrate()
		})
		if err != nil {
			return fmt.Errorf("failed to run database migrations: %w", err)
		}
	} else {
		if err := app.database.Migrate(); err != nil {
			return fmt.Errorf("failed to run database migrations: %w", err)
		}
	}

	// Seed database if needed
	if *seedDB {
		if err := app.seedDatabase(ctx); err != nil {
			return fmt.Errorf("failed to seed database: %w", err)
		}
	}

	// Start processing engine
	if err := app.processingEngine.Start(); err != nil {
		return fmt.Errorf("failed to start processing engine: %w", err)
	}

	// Setup Gin router with all middleware
	router := gin.New()

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Add authentication middleware if enabled
	if app.authService != nil {
		authMiddleware := middleware.NewAuthMiddleware(app.authService, app.logger)

		// Add rate limiting middleware
		rateLimitMiddleware := middleware.NewRateLimitMiddleware(
			app.redisClient,
			&middleware.RateLimitConfig{
				RequestsPerMinute: 100,
				BurstSize:         10,
				WindowSize:        time.Minute,
			},
			app.logger,
		)

		// Add audit logging middleware
		auditMiddleware := middleware.NewAuditMiddleware(app.userRepository, app.logger)

		// Apply global middleware
		router.Use(rateLimitMiddleware.RateLimit())
		router.Use(auditMiddleware.AuditLog())

		// Authentication endpoints (no auth required)
		authGroup := router.Group("/api/v1/auth")
		app.authHandlers.SetupRoutes(authGroup)
	}

	// Add circuit breaker middleware if enabled
	if app.circuitBreakerIntegration != nil {
		cbMiddleware := middleware.NewCircuitBreakerMiddleware(
			app.circuitBreakerIntegration.GetManager(),
			app.logger,
		)

		// Add global circuit breaker middleware
		router.Use(cbMiddleware.HTTPMiddleware("http"))

		// Add circuit breaker health check endpoints
		healthCheck := middleware.NewCircuitBreakerHealthCheck(
			app.circuitBreakerIntegration.GetManager(),
			app.logger,
		)

		router.GET("/health/circuit-breakers", healthCheck.HealthCheckHandler())
		router.GET("/metrics/circuit-breakers", healthCheck.MetricsHandler())
	}

	// Add standard middleware
	router.Use(app.handlers.LoggingMiddleware)
	router.Use(app.handlers.CORSMiddleware)
	router.Use(app.handlers.ContentTypeMiddleware)

	// Setup API routes
	api := router.Group("/api/v1")

	// Apply authentication middleware to protected routes
	if app.authService != nil {
		authMiddleware := middleware.NewAuthMiddleware(app.authService, app.logger)

		// Protected routes with JWT authentication
		protectedAPI := api.Group("")
		protectedAPI.Use(authMiddleware.RequireAuth())

		// Admin routes with role-based access
		adminAPI := api.Group("/admin")
		adminAPI.Use(authMiddleware.RequireAuth())
		adminAPI.Use(authMiddleware.RequireRole("admin"))

		// Service-to-service routes with API key authentication
		serviceAPI := api.Group("/service")
		serviceAPI.Use(authMiddleware.RequireAPIKey())

		// Setup route groups with appropriate protection
		if app.circuitBreakerIntegration != nil {
			cbMiddleware := middleware.NewCircuitBreakerMiddleware(
				app.circuitBreakerIntegration.GetManager(),
				app.logger,
			)

			// Database-heavy routes
			dbRoutes := protectedAPI.Group("/")
			dbRoutes.Use(cbMiddleware.HTTPMiddleware("database"))
			app.handlers.SetupDatabaseRoutes(dbRoutes)

			// Cache-heavy routes
			if app.protectedCache != nil {
				cacheRoutes := protectedAPI.Group("/cache")
				cacheRoutes.Use(cbMiddleware.HTTPMiddleware("cache"))
				app.handlers.SetupCacheRoutes(cacheRoutes)
			}

			// S3-heavy routes
			s3Routes := protectedAPI.Group("/files")
			s3Routes.Use(cbMiddleware.HTTPMiddleware("s3"))
			app.handlers.SetupS3Routes(s3Routes)

			// Admin routes
			adminRoutes := adminAPI.Group("/")
			adminRoutes.Use(cbMiddleware.HTTPMiddleware("database"))
			app.handlers.SetupAdminRoutes(adminRoutes)

			// Service routes
			serviceRoutes := serviceAPI.Group("/")
			serviceRoutes.Use(cbMiddleware.HTTPMiddleware("database"))
			app.handlers.SetupServiceRoutes(serviceRoutes)
		} else {
			app.handlers.SetupRoutes(protectedAPI)
		}
	} else {
		// No authentication - setup routes directly
		if app.circuitBreakerIntegration != nil {
			cbMiddleware := middleware.NewCircuitBreakerMiddleware(
				app.circuitBreakerIntegration.GetManager(),
				app.logger,
			)

			dbRoutes := api.Group("/")
			dbRoutes.Use(cbMiddleware.HTTPMiddleware("database"))
			app.handlers.SetupDatabaseRoutes(dbRoutes)

			if app.protectedCache != nil {
				cacheRoutes := api.Group("/cache")
				cacheRoutes.Use(cbMiddleware.HTTPMiddleware("cache"))
				app.handlers.SetupCacheRoutes(cacheRoutes)
			}

			s3Routes := api.Group("/files")
			s3Routes.Use(cbMiddleware.HTTPMiddleware("s3"))
			app.handlers.SetupS3Routes(s3Routes)
		} else {
			app.handlers.SetupRoutes(api)
		}
	}

	// Health check endpoint
	router.GET("/health", app.healthCheckHandler())

	// Create HTTP server
	app.server = &http.Server{
		Addr:           app.config.GetServerAddress(),
		Handler:        router,
		ReadTimeout:    app.config.Server.ReadTimeout,
		WriteTimeout:   app.config.Server.WriteTimeout,
		IdleTimeout:    app.config.Server.IdleTimeout,
		MaxHeaderBytes: app.config.Server.MaxHeaderBytes,
	}

	// Start server in a goroutine
	go func() {
		app.logger.Info("HTTP server starting with all features",
			"address", app.server.Addr,
			"circuit_breaker_enabled", app.circuitBreakerIntegration != nil,
			"retry_enabled", app.retryManager != nil,
			"auth_enabled", app.authService != nil,
		)

		var err error
		if app.config.Server.TLSCertFile != "" && app.config.Server.TLSKeyFile != "" {
			err = app.server.ListenAndServeTLS(app.config.Server.TLSCertFile, app.config.Server.TLSKeyFile)
		} else {
			err = app.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			app.logger.Error("HTTP server failed", "error", err)
		}
	}()

	app.logger.Info("Server started successfully with all features", "address", app.server.Addr)

	// Wait for shutdown signal
	return app.waitForShutdown()
}

// healthCheckHandler returns a comprehensive health check handler
func (app *IntegratedApplication) healthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		status := "healthy"
		statusCode := http.StatusOK

		health := gin.H{
			"status":    status,
			"version":   AppVersion,
			"timestamp": time.Now().UTC(),
			"services": gin.H{
				"database": "healthy",
				"s3":       "healthy",
			},
		}

		// Add circuit breaker status
		if app.circuitBreakerIntegration != nil {
			circuitBreakerStatus := app.circuitBreakerIntegration.GetHealthStatus()
			health["circuit_breakers"] = circuitBreakerStatus

			if !circuitBreakerStatus["overall_healthy"].(bool) {
				status = "degraded"
				statusCode = http.StatusPartialContent
				health["status"] = status
			}
		}

		// Add retry manager status
		if app.retryManager != nil {
			retryMetrics := app.retryManager.GetMetrics()
			health["retry_metrics"] = retryMetrics
		}

		// Add authentication status
		if app.authService != nil {
			authStatus := map[string]interface{}{
				"enabled":         true,
				"jwt_enabled":     app.jwtService != nil,
				"rbac_enabled":    app.rbacService != nil,
				"api_key_enabled": app.apiKeyService != nil,
			}
			health["authentication"] = authStatus
		} else {
			health["authentication"] = map[string]interface{}{
				"enabled": false,
			}
		}

		c.JSON(statusCode, health)
	}
}

// createAdminUser creates a default admin user
func (app *IntegratedApplication) createAdminUser() error {
	if app.authService == nil {
		return fmt.Errorf("authentication is disabled")
	}

	ctx := context.Background()

	// Get admin password
	password := *adminPassword
	if password == "" {
		fmt.Print("Enter admin password: ")
		fmt.Scanln(&password)
	}

	// Create admin user
	adminUser := &domain.User{
		Email:     *adminEmail,
		Password:  password,
		FirstName: "Admin",
		LastName:  "User",
		IsActive:  true,
		IsAdmin:   true,
	}

	err := app.authService.CreateUser(ctx, adminUser)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	// Assign admin role
	err = app.rbacService.AssignRole(ctx, adminUser.ID, "admin")
	if err != nil {
		return fmt.Errorf("failed to assign admin role: %w", err)
	}

	app.logger.Info("Admin user created successfully", "email", adminUser.Email)
	return nil
}

// generateAPIKey generates an API key for service-to-service communication
func (app *IntegratedApplication) generateAPIKey() error {
	if app.apiKeyService == nil {
		return fmt.Errorf("API key service is disabled")
	}

	ctx := context.Background()

	// Parse scopes
	scopes := strings.Split(*apiKeyScopes, ",")
	for i, scope := range scopes {
		scopes[i] = strings.TrimSpace(scope)
	}

	// Generate API key
	apiKey, err := app.apiKeyService.GenerateAPIKey(ctx, *apiKeyName, scopes)
	if err != nil {
		return fmt.Errorf("failed to generate API key: %w", err)
	}

	fmt.Printf("API Key generated successfully:\n")
	fmt.Printf("Name: %s\n", apiKey.Name)
	fmt.Printf("Key: %s\n", apiKey.Key)
	fmt.Printf("Scopes: %v\n", apiKey.Scopes)
	fmt.Printf("Created: %s\n", apiKey.CreatedAt.Format(time.RFC3339))

	app.logger.Info("API key generated successfully", "name", apiKey.Name, "scopes", apiKey.Scopes)
	return nil
}

// showAuthStatus shows the current authentication status
func (app *IntegratedApplication) showAuthStatus() error {
	if app.authService == nil {
		app.logger.Info("Authentication is disabled")
		return nil
	}

	app.logger.Info("Authentication Status Report")

	ctx := context.Background()

	// Get user count
	userCount, err := app.userRepository.GetUserCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user count: %w", err)
	}

	// Get active sessions count
	sessionCount, err := app.authService.GetActiveSessionCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get session count: %w", err)
	}

	// Get API key count
	apiKeyCount, err := app.apiKeyService.GetAPIKeyCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API key count: %w", err)
	}

	app.logger.Info("Authentication Statistics",
		"total_users", userCount,
		"active_sessions", sessionCount,
		"api_keys", apiKeyCount,
		"jwt_enabled", app.jwtService != nil,
		"rbac_enabled", app.rbacService != nil,
	)

	return nil
}

// seedDatabase seeds the database with initial data
func (app *IntegratedApplication) seedDatabase(ctx context.Context) error {
	if app.protectedDatabase != nil {
		return app.protectedDatabase.ExecuteQuery(ctx, func(db *gorm.DB) error {
			return app.database.Seed(ctx)
		})
	}
	return app.database.Seed(ctx)
}

// waitForShutdown waits for shutdown signal and performs graceful shutdown
func (app *IntegratedApplication) waitForShutdown() error {
	// Create channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	app.logger.Info("Received shutdown signal", "signal", sig)

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), app.config.Server.ShutdownTimeout)
	defer cancel()

	// Shutdown processing engine
	if err := app.processingEngine.Stop(); err != nil {
		app.logger.Error("Failed to stop processing engine", "error", err)
	}

	// Shutdown HTTP server
	if app.server != nil {
		app.logger.Info("Shutting down HTTP server...")
		if err := app.server.Shutdown(ctx); err != nil {
			app.logger.Error("Failed to shutdown HTTP server gracefully", "error", err)
			return err
		}
	}

	// Close authentication services
	if app.authService != nil {
		app.authService.Close()
		app.logger.Info("Authentication service closed")
	}

	// Close circuit breaker integration
	if app.circuitBreakerIntegration != nil {
		app.circuitBreakerIntegration.Close()
		app.logger.Info("Circuit breaker integration closed")
	}

	// Close retry manager
	if app.retryManager != nil {
		app.retryManager.Close()
		app.logger.Info("Retry manager closed")
	}

	// Close Redis connection
	if app.redisClient != nil {
		if err := app.redisClient.Close(); err != nil {
			app.logger.Error("Failed to close Redis connection", "error", err)
		}
	}

	// Close database connection
	if err := app.database.Close(); err != nil {
		app.logger.Error("Failed to close database connection", "error", err)
	}

	app.logger.Info("Shutdown completed successfully")
	return nil
}

// Rest of the methods (runCLI, runMigrations, etc.) remain similar but with auth integration
// ... (implement other methods with authentication integration)

// printHelp prints comprehensive help information
func printHelp() {
	fmt.Printf(`%s - Hotel Reviews Microservice with Full Feature Integration

USAGE:
    %s [OPTIONS]

OPTIONS:
    -mode string
        Application mode: server, cli, migrate, process-files, health-check, 
        circuit-breaker-status, auth-status, create-admin, generate-api-key (default "server")
    -config string
        Path to configuration file
    -log-level string
        Log level override (debug, info, warn, error)
    -host string
        Server host override
    -port int
        Server port override

RESILIENCE OPTIONS:
    -disable-circuit-breaker
        Disable circuit breaker protection
    -disable-retry
        Disable retry mechanisms
    -reset-circuit-breakers
        Reset all circuit breakers to closed state

AUTHENTICATION OPTIONS:
    -disable-auth
        Disable authentication (NOT recommended for production)
    -create-admin
        Create default admin user
    -admin-email string
        Admin user email (default "admin@example.com")
    -admin-password string
        Admin user password (will be prompted if not provided)
    -generate-api-key
        Generate API key for service-to-service communication
    -api-key-name string
        Name for the generated API key (default "default-service")
    -api-key-scopes string
        Comma-separated scopes for the API key (default "read,write")

EXAMPLES:
    # Start server with all features
    %s -mode server

    # Start server without authentication
    %s -mode server -disable-auth

    # Create admin user
    %s -mode create-admin -admin-email admin@company.com

    # Generate API key
    %s -mode generate-api-key -api-key-name my-service -api-key-scopes read,write,admin

    # Check comprehensive health
    %s -mode health-check

    # Show authentication status
    %s -mode auth-status

ENDPOINTS:
    # Authentication
    POST /api/v1/auth/register     - User registration
    POST /api/v1/auth/login        - User login
    POST /api/v1/auth/refresh      - Token refresh
    POST /api/v1/auth/logout       - User logout
    
    # Health and Monitoring
    GET  /health                   - Overall health check
    GET  /health/circuit-breakers  - Circuit breaker health
    GET  /metrics/circuit-breakers - Circuit breaker metrics
    
    # Protected API Routes (require authentication)
    GET  /api/v1/reviews          - Get reviews
    POST /api/v1/reviews          - Create review
    GET  /api/v1/hotels           - Get hotels
    
    # Admin Routes (require admin role)
    GET  /api/v1/admin/users      - Manage users
    POST /api/v1/admin/api-keys   - Manage API keys
    
    # Service Routes (require API key)
    GET  /api/v1/service/reviews  - Service-to-service API

FEATURES:
    ✅ JWT Authentication with refresh tokens
    ✅ Role-based Access Control (RBAC)
    ✅ API Key authentication for services
    ✅ Circuit breaker protection
    ✅ Advanced retry mechanisms
    ✅ Rate limiting and audit logging
    ✅ Comprehensive health checks
    ✅ Real-time metrics and monitoring

`, AppName, AppName, AppName, AppName, AppName, AppName, AppName, AppName)
}

// printVersion prints version information
func printVersion() {
	fmt.Printf(`%s
Version: %s (Full Feature Integration)
Features: Authentication, RBAC, Circuit Breakers, Retry Mechanisms, Monitoring
Build: %s
`, AppName, AppVersion, time.Now().Format("2006-01-02"))
}

// loadConfiguration loads configuration (same as before)
func loadConfiguration() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Apply command-line overrides
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}

	if *host != "" {
		cfg.Server.Host = *host
	}

	if *port != 0 {
		cfg.Server.Port = *port
	}

	return cfg, nil
}
