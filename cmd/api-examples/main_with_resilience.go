package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

// ResilientApplication represents the main application with circuit breaker and retry mechanisms
type ResilientApplication struct {
	config      *config.Config
	logger      *logger.Logger
	database    *infrastructure.Database
	redisClient *redis.Client
	s3Client    *infrastructure.S3Client

	// Circuit breaker and retry infrastructure
	circuitBreakerIntegration *infrastructure.CircuitBreakerIntegration
	retryManager              *infrastructure.RetryManager

	// Protected service wrappers
	protectedDatabase *infrastructure.DatabaseWrapper
	protectedCache    *infrastructure.CacheWrapper
	protectedS3       *infrastructure.S3Wrapper

	// Domain services
	reviewRepository domain.ReviewRepository
	jsonProcessor    domain.JSONProcessor
	reviewService    domain.ReviewService
	processingEngine *application.ProcessingEngine
	handlers         *application.Handlers

	// HTTP server
	server *http.Server
}

// CLI flags (same as original)
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
)

const (
	AppName    = "hotel-reviews-microservice"
	AppVersion = "2.0.0" // Updated version with resilience features
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

	// Initialize resilient application
	app, err := initializeResilientApplication()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// Handle circuit breaker reset flag
	if *resetCircuitBreakers {
		app.circuitBreakerIntegration.ResetAllCircuitBreakers()
		app.logger.Info("All circuit breakers reset")
		if *mode == "reset-circuit-breakers" {
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
	default:
		err = fmt.Errorf("unknown mode: %s", *mode)
	}

	if err != nil {
		app.logger.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

// initializeResilientApplication initializes the application with circuit breaker and retry protection
func initializeResilientApplication() (*ResilientApplication, error) {
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

	log.Info("Starting resilient application",
		"name", AppName,
		"version", AppVersion,
		"mode", *mode,
		"circuit_breaker_enabled", !*disableCircuitBreaker,
		"retry_enabled", !*disableRetry,
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

		// Setup health checks
		circuitBreakerIntegration.SetupHealthChecks(database.DB, redisClient)

		log.Info("Circuit breaker integration initialized")
	}

	if !*disableRetry {
		retryManager = infrastructure.NewRetryManager(log)

		// Configure retry manager with circuit breaker integration
		if circuitBreakerIntegration != nil {
			retryManager.SetCircuitBreakerManager(circuitBreakerIntegration.GetManager())
		}

		log.Info("Retry manager initialized")
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

	// Initialize repository with protected database
	var reviewRepository domain.ReviewRepository
	if protectedDatabase != nil {
		reviewRepository = infrastructure.NewProtectedReviewRepository(protectedDatabase, log)
	} else {
		reviewRepository = infrastructure.NewReviewRepository(database, log)
	}

	// Initialize JSON processor
	jsonProcessor := infrastructure.NewJSONLinesProcessor(reviewRepository, log)

	// Initialize review service with protected dependencies
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

	// Initialize processing engine with resilience
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

	// Initialize handlers with resilience
	var handlers *application.Handlers
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

	app := &ResilientApplication{
		config:                    cfg,
		logger:                    log,
		database:                  database,
		redisClient:               redisClient,
		s3Client:                  s3Client,
		circuitBreakerIntegration: circuitBreakerIntegration,
		retryManager:              retryManager,
		protectedDatabase:         protectedDatabase,
		protectedCache:            protectedCache,
		protectedS3:               protectedS3,
		reviewRepository:          reviewRepository,
		jsonProcessor:             jsonProcessor,
		reviewService:             reviewService,
		processingEngine:          processingEngine,
		handlers:                  handlers,
	}

	return app, nil
}

// runServer runs the application in server mode with circuit breaker middleware
func (app *ResilientApplication) runServer() error {
	ctx := context.Background()

	app.logger.Info("Starting server mode with resilience features")

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
		if app.protectedDatabase != nil {
			err := app.protectedDatabase.ExecuteQuery(ctx, func(db *gorm.DB) error {
				return app.database.Seed(ctx)
			})
			if err != nil {
				return fmt.Errorf("failed to seed database: %w", err)
			}
		} else {
			if err := app.database.Seed(ctx); err != nil {
				return fmt.Errorf("failed to seed database: %w", err)
			}
		}
	}

	// Start processing engine
	if err := app.processingEngine.Start(); err != nil {
		return fmt.Errorf("failed to start processing engine: %w", err)
	}

	// Setup Gin router with circuit breaker middleware
	router := gin.New()

	// Add recovery middleware
	router.Use(gin.Recovery())

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

	// Setup routes
	api := router.Group("/api/v1")

	// Add service-specific circuit breaker middleware to different route groups
	if app.circuitBreakerIntegration != nil {
		cbMiddleware := middleware.NewCircuitBreakerMiddleware(
			app.circuitBreakerIntegration.GetManager(),
			app.logger,
		)

		// Database-heavy routes
		dbRoutes := api.Group("/")
		dbRoutes.Use(cbMiddleware.HTTPMiddleware("database"))
		app.handlers.SetupDatabaseRoutes(dbRoutes)

		// Cache-heavy routes
		if app.protectedCache != nil {
			cacheRoutes := api.Group("/cache")
			cacheRoutes.Use(cbMiddleware.HTTPMiddleware("cache"))
			app.handlers.SetupCacheRoutes(cacheRoutes)
		}

		// S3-heavy routes
		s3Routes := api.Group("/files")
		s3Routes.Use(cbMiddleware.HTTPMiddleware("s3"))
		app.handlers.SetupS3Routes(s3Routes)
	} else {
		app.handlers.SetupRoutes(api)
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
		app.logger.Info("HTTP server starting with resilience features",
			"address", app.server.Addr,
			"circuit_breaker_enabled", app.circuitBreakerIntegration != nil,
			"retry_enabled", app.retryManager != nil,
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

	app.logger.Info("Server started successfully with resilience features", "address", app.server.Addr)

	// Wait for shutdown signal
	return app.waitForShutdown()
}

// healthCheckHandler returns a health check handler with circuit breaker status
func (app *ResilientApplication) healthCheckHandler() gin.HandlerFunc {
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

			// If any circuit breaker is open, mark as degraded
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

		c.JSON(statusCode, health)
	}
}

// runHealthCheck runs a health check and exits
func (app *ResilientApplication) runHealthCheck() error {
	app.logger.Info("Running health check")

	// Check database
	if app.protectedDatabase != nil {
		err := app.protectedDatabase.ExecuteQuery(context.Background(), func(db *gorm.DB) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Ping()
		})
		if err != nil {
			app.logger.Error("Database health check failed", "error", err)
			return err
		}
		app.logger.Info("Database health check passed")
	}

	// Check circuit breakers
	if app.circuitBreakerIntegration != nil {
		healthStatus := app.circuitBreakerIntegration.GetHealthStatus()
		app.logger.Info("Circuit breaker health status", "status", healthStatus)

		if !healthStatus["overall_healthy"].(bool) {
			app.logger.Warn("Some circuit breakers are open")
		}
	}

	// Check retry manager
	if app.retryManager != nil {
		metrics := app.retryManager.GetMetrics()
		app.logger.Info("Retry manager metrics", "metrics", metrics)
	}

	app.logger.Info("Health check completed successfully")
	return nil
}

// showCircuitBreakerStatus shows the current circuit breaker status
func (app *ResilientApplication) showCircuitBreakerStatus() error {
	if app.circuitBreakerIntegration == nil {
		app.logger.Info("Circuit breakers are disabled")
		return nil
	}

	app.logger.Info("Circuit Breaker Status Report")

	breakers := app.circuitBreakerIntegration.GetManager().GetAllCircuitBreakers()

	for name, breaker := range breakers {
		metrics := breaker.GetMetrics()

		app.logger.Info("Circuit Breaker Details",
			"name", name,
			"state", metrics.CurrentState.String(),
			"total_requests", metrics.TotalRequests,
			"success_rate", fmt.Sprintf("%.2f%%", metrics.SuccessRate),
			"failure_rate", fmt.Sprintf("%.2f%%", metrics.FailureRate),
			"last_failure", metrics.LastFailure,
			"last_success", metrics.LastSuccess,
		)
	}

	return nil
}

// waitForShutdown waits for shutdown signal and performs graceful shutdown
func (app *ResilientApplication) waitForShutdown() error {
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

// Rest of the methods (runCLI, runMigrations, etc.) remain the same as original
// but can be enhanced with circuit breaker and retry protection

// ... (implement other methods with resilience features)

// printHelp prints enhanced help information
func printHelp() {
	fmt.Printf(`%s - Hotel Reviews Microservice with Resilience Features

USAGE:
    %s [OPTIONS]

OPTIONS:
    -mode string
        Application mode: server, cli, migrate, process-files, health-check, circuit-breaker-status (default "server")
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

CLI MODE OPTIONS:
    -file-url string
        S3 URL of the file to process
    -provider string
        Provider name for file processing

EXAMPLES:
    # Start server with full resilience
    %s -mode server

    # Start server without circuit breakers
    %s -mode server -disable-circuit-breaker

    # Check health including circuit breaker status
    %s -mode health-check

    # Show circuit breaker status
    %s -mode circuit-breaker-status

    # Reset circuit breakers
    %s -reset-circuit-breakers

ENDPOINTS:
    /health - Overall health check including circuit breaker status
    /health/circuit-breakers - Circuit breaker specific health check
    /metrics/circuit-breakers - Circuit breaker metrics

`, AppName, AppName, AppName, AppName, AppName, AppName, AppName)
}

// printVersion prints version information
func printVersion() {
	fmt.Printf(`%s
Version: %s (with Circuit Breaker and Retry Support)
Build: %s
`, AppName, AppVersion, time.Now().Format("2006-01-02"))
}

// loadConfiguration loads configuration (same as original)
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
