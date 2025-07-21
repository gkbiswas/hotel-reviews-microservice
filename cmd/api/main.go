package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/server"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

func main() {
	// Parse command line flags
	var (
		mode        = flag.String("mode", "production", "Application mode (development/production)")
		logLevel    = flag.String("log-level", "info", "Log level (debug/info/warn/error)")
		host        = flag.String("host", "", "Server host (overrides config)")
		port        = flag.Int("port", 0, "Server port (overrides config)")
		createAdmin = flag.Bool("create-admin", false, "Create admin user on startup")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Apply command line overrides
	if *host != "" {
		cfg.Server.Host = *host
	}
	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}

	// Initialize structured logger
	loggerConfig := &logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
	}
	appLogger, err := logger.New(loggerConfig)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Create slog logger for components that need it
	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Initialize graceful shutdown manager
	shutdownConfig := server.DefaultShutdownConfig()
	shutdownConfig.GracefulTimeout = time.Duration(cfg.Server.ShutdownTimeout)

	loggerAdapter := &LoggerAdapter{Logger: appLogger}
	shutdownManager := server.NewShutdownManager(loggerAdapter, shutdownConfig)

	// Initialize database
	database, err := infrastructure.NewDatabase(&cfg.Database, appLogger)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize S3 client
	s3Client, err := infrastructure.NewS3Client(&cfg.S3, appLogger)
	if err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}

	// Initialize Redis client for caching
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetCacheAddress(),
		Password: cfg.Cache.Password,
		DB:       cfg.Cache.Database,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		appLogger.Warn("Redis connection failed, continuing without cache", "error", err)
		redisClient = nil
	}

	// Initialize cache service
	var cacheService domain.CacheService
	if redisClient != nil {
		cacheService = infrastructure.NewRedisCacheService(redisClient, appLogger)
	}

	// Initialize circuit breaker and retry manager
	cbConfig := infrastructure.DefaultCircuitBreakerConfig()
	cbConfig.Name = "api-circuit-breaker"
	circuitBreaker := infrastructure.NewCircuitBreaker(cbConfig, appLogger)

	retryConfig := infrastructure.DefaultRetryConfig()
	retryManager := infrastructure.NewRetryManager(retryConfig, circuitBreaker, appLogger)

	// Initialize repositories
	reviewRepo := infrastructure.NewReviewRepository(database, appLogger)

	// Initialize JSON processor
	jsonProcessor := infrastructure.NewJSONLinesProcessor(reviewRepo, appLogger)

	// Initialize auth repository
	authRepo := infrastructure.NewAuthRepository(database.DB, slogLogger, circuitBreaker, retryManager)

	// Initialize auth services
	jwtService := infrastructure.NewJWTService(cfg, slogLogger, circuitBreaker, retryManager)
	passwordService := infrastructure.NewPasswordService(slogLogger)
	rbacService := infrastructure.NewRBACService(authRepo, slogLogger, circuitBreaker, retryManager)
	apiKeyService := infrastructure.NewApiKeyService(authRepo, slogLogger, circuitBreaker, retryManager)
	rateLimitService := infrastructure.NewRateLimitService(authRepo, slogLogger, circuitBreaker, retryManager)
	auditService := infrastructure.NewAuditService(authRepo, slogLogger, circuitBreaker, retryManager)

	// Initialize authentication service
	authService := infrastructure.NewAuthenticationService(
		authRepo, jwtService, passwordService, rbacService,
		apiKeyService, rateLimitService, auditService,
		slogLogger, circuitBreaker, retryManager,
	)

	// Initialize error handler
	errorHandlerConfig := &infrastructure.ErrorHandlerConfig{}
	errorHandler := infrastructure.NewErrorHandler(errorHandlerConfig, appLogger, circuitBreaker, retryConfig)

	// Initialize review service
	reviewService := domain.NewReviewService(
		reviewRepo,
		s3Client,
		jsonProcessor,
		nil, // notification service - not implemented
		cacheService,
		nil, // metrics service - not implemented
		nil, // event publisher - not implemented
		slogLogger,
	)

	// Initialize application handlers
	handlers := application.NewSimplifiedIntegratedHandlers(
		reviewService,
		authService,
		rbacService,
		nil, // circuit breaker integration - not configured
		retryManager,
		nil, // redis client - not configured
		nil, // kafka producer - not configured
		s3Client,
		errorHandler,
		nil, // processing engine - not configured
		appLogger,
	)

	// Initialize specific handlers
	authHandlers := application.NewAuthHandlers(authService, passwordService, slogLogger)
	
	// Initialize hotel and provider services using adapters
	hotelService := application.NewHotelServiceAdapter(reviewService)
	providerService := application.NewProviderServiceAdapter(reviewService)
	hotelHandlers := application.NewHotelHandlers(hotelService, appLogger)
	providerHandlers := application.NewProviderHandlers(providerService, appLogger)
	
	// Initialize search and analytics handlers
	searchAnalyticsHandlers := application.NewSearchAnalyticsHandlers(reviewService, appLogger)
	
	// Initialize security middleware
	securityMiddleware := middleware.NewSecurityMiddleware(&cfg.Security, appLogger, redisClient)
	
	// Initialize rate limiting middleware
	rateLimitConfig := middleware.UserBasedRateLimitConfig(cfg.Security.RateLimit)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(rateLimitConfig, redisClient, appLogger)
	
	// Initialize processing engine and file handlers
	processingConfig := &application.ProcessingConfig{
		MaxWorkers:         4, // default workers
		MaxConcurrentFiles: 10,
		MaxRetries:         3,
		RetryDelay:         time.Second * 5,
		ProcessingTimeout:  time.Minute * 30,
		WorkerIdleTimeout:  time.Minute * 5,
		MetricsInterval:    time.Second * 30,
	}
	processingEngine := application.NewProcessingEngine(reviewService, s3Client, jsonProcessor, appLogger, processingConfig)
	fileHandlers := application.NewFileHandlers(reviewService, s3Client, jsonProcessor, processingEngine, appLogger, cfg.S3.Bucket)

	// Initialize business metrics system
	metricsRegistry := prometheus.NewRegistry()
	businessMetrics := monitoring.NewBusinessMetrics(slogLogger, metricsRegistry)
	businessMetricsMiddleware := middleware.NewBusinessMetricsMiddleware(businessMetrics, slogLogger)
	
	// Initialize dashboard manager
	dashboardManager := monitoring.NewDashboardManager(slogLogger)

	// Initialize simple health checker (using existing implementation)
	healthChecker := &HealthChecker{logger: loggerAdapter, checks: make(map[string]func(context.Context) error)}
	healthChecker.AddCheck("database", func(ctx context.Context) error {
		sqlDB, err := database.DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	})
	if redisClient != nil {
		healthChecker.AddCheck("redis", func(ctx context.Context) error {
			return redisClient.Ping(ctx).Err()
		})
	}
	healthChecker.AddCheck("s3", func(ctx context.Context) error {
		// Test S3 connection by checking if bucket exists
		_, err := s3Client.BucketExists(ctx, cfg.S3.Bucket)
		return err
	})

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	if *mode == "development" {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Add essential middleware
	router.Use(gin.Recovery())
	router.Use(requestIDMiddleware())
	router.Use(loggingMiddleware(appLogger))
	
	// Add security middleware
	router.Use(securityMiddleware.SecurityHeadersMiddleware())
	router.Use(securityMiddleware.CORSMiddleware())
	router.Use(securityMiddleware.InputValidationMiddleware())
	router.Use(securityMiddleware.DDoSProtectionMiddleware())
	
	// Add rate limiting middleware
	router.Use(rateLimitMiddleware.Middleware())
	
	// Add business metrics middleware
	router.Use(businessMetricsMiddleware.Middleware())

	// Health check routes
	router.GET("/health", gin.HandlerFunc(healthChecker.Handler))
	router.GET("/healthz", gin.HandlerFunc(healthChecker.Handler))
	
	// Metrics endpoints
	router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{})))
	router.GET("/metrics/prometheus", gin.WrapH(promhttp.Handler())) // Default prometheus metrics
	
	// Dashboard management routes
	router.GET("/dashboards", gin.WrapH(dashboardManager.GetDashboardHTTPHandler()))
	router.POST("/dashboards", gin.WrapH(dashboardManager.GetDashboardHTTPHandler()))
	router.PUT("/dashboards", gin.WrapH(dashboardManager.GetDashboardHTTPHandler()))
	router.DELETE("/dashboards", gin.WrapH(dashboardManager.GetDashboardHTTPHandler()))

	// API routes
	api := router.Group("/api/v1")
	{
		// Authentication routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", gin.WrapF(authHandlers.Register))
			auth.POST("/login", gin.WrapF(authHandlers.Login))
			auth.POST("/refresh", gin.WrapF(authHandlers.RefreshToken))
			auth.POST("/logout", gin.WrapF(authHandlers.Logout))
		}

		// Review routes
		reviews := api.Group("/reviews")
		{
			reviews.GET("", handlers.ListReviews)
			reviews.GET("/:id", handlers.GetReview)
			reviews.POST("", handlers.CreateReview)
			reviews.PUT("/:id", handlers.UpdateReview)
			reviews.DELETE("/:id", handlers.DeleteReview)
			// Bulk operations not implemented in simplified handlers
		}

		// Hotel routes
		hotels := api.Group("/hotels")
		{
			hotels.GET("", hotelHandlers.GetHotels)
			hotels.GET("/:id", hotelHandlers.GetHotel)
			hotels.POST("", hotelHandlers.CreateHotel)
			hotels.PUT("/:id", hotelHandlers.UpdateHotel)
			hotels.DELETE("/:id", hotelHandlers.DeleteHotel)
		}

		// Provider routes
		providers := api.Group("/providers")
		{
			providers.GET("", providerHandlers.GetProviders)
			providers.GET("/:id", providerHandlers.GetProvider)
			providers.POST("", providerHandlers.CreateProvider)
			providers.PUT("/:id", providerHandlers.UpdateProvider)
			providers.DELETE("/:id", providerHandlers.DeleteProvider)
		}

		// File processing routes
		files := api.Group("/files")
		{
			files.POST("/upload", fileHandlers.UploadAndProcessFile)
			files.GET("/processing/:id", fileHandlers.GetProcessingStatus)
			files.GET("/processing/history", fileHandlers.GetProcessingHistory)
			files.POST("/processing/:id/cancel", fileHandlers.CancelProcessing)
			files.POST("/validate", fileHandlers.ValidateFile)
			files.GET("/metrics", fileHandlers.GetProcessingMetrics)
		}

		// Search routes
		search := api.Group("/search")
		{
			search.GET("/reviews", searchAnalyticsHandlers.SearchReviews)
			search.GET("/hotels", searchAnalyticsHandlers.SearchHotels)
		}

		// Analytics routes
		analytics := api.Group("/analytics")
		{
			analytics.GET("/overview", searchAnalyticsHandlers.GetOverallAnalytics)
			analytics.GET("/hotels/top-rated", searchAnalyticsHandlers.GetTopRatedHotels)
			analytics.GET("/hotels/:id/stats", searchAnalyticsHandlers.GetHotelStats)
			analytics.GET("/hotels/:id/summary", searchAnalyticsHandlers.GetReviewSummary)
			analytics.GET("/providers/:id/stats", searchAnalyticsHandlers.GetProviderStats)
			analytics.GET("/reviews/recent", searchAnalyticsHandlers.GetRecentReviews)
			analytics.GET("/trends/reviews", searchAnalyticsHandlers.GetReviewTrends)
		}

		// User management routes
		users := api.Group("/users")
		{
			users.GET("/me", gin.WrapF(authHandlers.GetProfile))
			users.PUT("/me", gin.WrapF(authHandlers.UpdateProfile))
			// Delete account not implemented in auth handlers
		}

		// Admin routes
		admin := api.Group("/admin")
		{
			admin.GET("/users", gin.WrapF(authHandlers.ListUsers))
			admin.PUT("/users/:id/role", gin.WrapF(authHandlers.AssignRole))
			admin.GET("/users/:id", gin.WrapF(authHandlers.GetUser))
			admin.DELETE("/users/:id", gin.WrapF(authHandlers.DeleteUser))
			admin.GET("/api-keys", gin.WrapF(authHandlers.ListApiKeys))
			admin.POST("/api-keys", gin.WrapF(authHandlers.CreateApiKey))
			admin.DELETE("/api-keys/:id", gin.WrapF(authHandlers.DeleteApiKey))
		}
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// Register HTTP server for graceful shutdown
	shutdownManager.RegisterResource(httpServer)

	// Create admin user if requested
	if *createAdmin {
		if err := createAdminUser(authService, slogLogger); err != nil {
			appLogger.Error("Failed to create admin user", "error", err)
		} else {
			appLogger.Info("Admin user created successfully")
		}
	}

	// Start processing engine
	if err := processingEngine.Start(); err != nil {
		appLogger.Error("Failed to start processing engine", "error", err)
	} else {
		appLogger.Info("Processing engine started successfully")
	}

	// Start server
	go func() {
		appLogger.Info("Starting HTTP server",
			"addr", httpServer.Addr,
			"mode", *mode,
			"features", map[string]bool{
				"authentication":  true,
				"rbac":            true,
				"circuit_breaker": true,
				"retry_logic":     true,
				"caching":         redisClient != nil,
				"s3_storage":      true,
				"health_checks":   true,
				"metrics":         true,
				"json_processing": true,
			},
		)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Log startup information
	appLogger.Info("Hotel Reviews Microservice started successfully",
		"version", cfg.Metrics.Version,
		"environment", cfg.Metrics.Environment,
		"service_name", cfg.Metrics.ServiceName,
	)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	appLogger.Info("Shutdown signal received, initiating graceful shutdown...")

	// Trigger graceful shutdown
	if err := shutdownManager.WaitForShutdown(); err != nil {
		appLogger.Error("Shutdown completed with errors", "error", err)
		os.Exit(1)
	}

	appLogger.Info("Graceful shutdown completed successfully")
}

// createAdminUser creates an initial admin user
func createAdminUser(authService *infrastructure.AuthenticationService, logger *slog.Logger) error {
	adminUser := &domain.User{
		Email:    "admin@example.com",
		Username: "admin",
		// Role will be set through RBAC service
		IsActive: true,
	}

	ctx := context.Background()
	if err := authService.CreateUser(ctx, adminUser, "admin123!"); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	// Generate API key for admin
	apiKey, err := authService.CreateApiKey(ctx, adminUser.ID, "admin-key", []string{"*"}, nil)
	if err != nil {
		logger.Warn("Failed to create admin API key", "error", err)
	} else {
		logger.Info("Admin API key created", "key", apiKey.Key)
	}

	return nil
}

// LoggerAdapter adapts the logger to monitoring.Logger interface
type LoggerAdapter struct {
	Logger *logger.Logger
}

func (l *LoggerAdapter) Debug(msg string, fields ...interface{}) {
	l.Logger.Debug(msg, fields...)
}

func (l *LoggerAdapter) Info(msg string, fields ...interface{}) {
	l.Logger.Info(msg, fields...)
}

func (l *LoggerAdapter) Warn(msg string, fields ...interface{}) {
	l.Logger.Warn(msg, fields...)
}

func (l *LoggerAdapter) Error(msg string, fields ...interface{}) {
	l.Logger.Error(msg, fields...)
}

// Simple middleware implementations

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func loggingMiddleware(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		logger.Info("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", duration,
			"client_ip", c.ClientIP(),
			"request_id", c.GetString("request_id"),
		)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization,X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Adapter types for compatibility

type HealthChecker struct {
	logger *LoggerAdapter
	checks map[string]func(context.Context) error
}

func (h *HealthChecker) AddCheck(name string, check func(context.Context) error) {
	h.checks[name] = check
}

func (h *HealthChecker) Handler(c *gin.Context) {
	ctx := c.Request.Context()
	status := "ok"
	details := make(map[string]string)

	for name, check := range h.checks {
		if err := check(ctx); err != nil {
			status = "error"
			details[name] = err.Error()
		} else {
			details[name] = "ok"
		}
	}

	if status == "error" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  status,
			"details": details,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  status,
		"details": details,
	})
}

type CircuitBreakerAdapter struct {
	cb *infrastructure.CircuitBreaker
}

type RedisAdapter struct {
	client *redis.Client
}

type ProcessingEngineAdapter struct{}

func (p *ProcessingEngineAdapter) ProcessFile(ctx context.Context, fileURL string) error {
	return nil
}
