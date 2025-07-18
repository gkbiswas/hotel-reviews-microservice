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

	"github.com/gorilla/mux"
	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Application represents the main application
type Application struct {
	config            *config.Config
	logger            *logger.Logger
	database          *infrastructure.Database
	s3Client          *infrastructure.S3Client
	reviewRepository  domain.ReviewRepository
	jsonProcessor     domain.JSONProcessor
	reviewService     domain.ReviewService
	processingEngine  *application.ProcessingEngine
	handlers          *application.Handlers
	server            *http.Server
}

// CLI flags
var (
	configFile    = flag.String("config", "", "Path to configuration file")
	mode          = flag.String("mode", "server", "Application mode: server, cli, migrate, process-files")
	logLevel      = flag.String("log-level", "", "Log level override (debug, info, warn, error)")
	host          = flag.String("host", "", "Server host override")
	port          = flag.Int("port", 0, "Server port override")
	
	// CLI specific flags
	fileURL       = flag.String("file-url", "", "S3 URL of the file to process")
	providerName  = flag.String("provider", "", "Provider name for file processing")
	resetDB       = flag.Bool("reset-db", false, "Reset database (drop all tables)")
	seedDB        = flag.Bool("seed-db", false, "Seed database with initial data")
	
	// Migration flags
	migrateUp     = flag.Bool("migrate-up", false, "Run database migrations")
	migrateDown   = flag.Bool("migrate-down", false, "Rollback database migrations")
	
	// Display flags
	version       = flag.Bool("version", false, "Show version information")
	help          = flag.Bool("help", false, "Show help information")
)

const (
	AppName    = "hotel-reviews-microservice"
	AppVersion = "1.0.0"
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
	
	// Initialize application
	app, err := initializeApplication()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
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
	default:
		err = fmt.Errorf("unknown mode: %s", *mode)
	}
	
	if err != nil {
		app.logger.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

// initializeApplication initializes the application with all dependencies
func initializeApplication() (*Application, error) {
	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Initialize logger
	loggerConfig := &logger.Config{
		Level:           cfg.Log.Level,
		Format:          cfg.Log.Format,
		Output:          cfg.Log.Output,
		FilePath:        cfg.Log.FilePath,
		MaxSize:         cfg.Log.MaxSize,
		MaxBackups:      cfg.Log.MaxBackups,
		MaxAge:          cfg.Log.MaxAge,
		Compress:        cfg.Log.Compress,
		EnableCaller:    cfg.Log.EnableCaller,
		EnableStacktrace: cfg.Log.EnableStacktrace,
	}
	log, err := logger.New(loggerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	
	log.Info("Starting application",
		"name", AppName,
		"version", AppVersion,
		"mode", *mode,
	)
	
	// Initialize database
	database, err := infrastructure.NewDatabase(&cfg.Database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	
	// Initialize S3 client
	s3Client, err := infrastructure.NewS3Client(&cfg.S3, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}
	
	// Initialize repository
	reviewRepository := infrastructure.NewReviewRepository(database, log)
	
	// Initialize JSON processor
	jsonProcessor := infrastructure.NewJSONLinesProcessor(reviewRepository, log)
	
	// Initialize review service
	reviewService := domain.NewReviewService(
		reviewRepository,
		s3Client,
		jsonProcessor,
		nil, // notificationService - TODO: implement
		nil, // cacheService - TODO: implement
		nil, // metricsService - TODO: implement
		nil, // eventPublisher - TODO: implement
		log.Logger,
	)
	
	// Initialize processing engine
	processingConfig := &application.ProcessingConfig{
		MaxWorkers:         cfg.Processing.WorkerCount,
		MaxConcurrentFiles: 10,
		MaxRetries:         cfg.Processing.MaxRetries,
		RetryDelay:         cfg.Processing.RetryDelay,
		ProcessingTimeout:  cfg.Processing.ProcessingTimeout,
		WorkerIdleTimeout:  5 * time.Minute,
		MetricsInterval:    30 * time.Second,
	}
	
	processingEngine := application.NewProcessingEngine(
		reviewService,
		s3Client,
		jsonProcessor,
		log,
		processingConfig,
	)
	
	// Initialize handlers
	handlers := application.NewHandlers(reviewService, log)
	
	app := &Application{
		config:            cfg,
		logger:            log,
		database:          database,
		s3Client:          s3Client,
		reviewRepository:  reviewRepository,
		jsonProcessor:     jsonProcessor,
		reviewService:     reviewService,
		processingEngine:  processingEngine,
		handlers:          handlers,
	}
	
	return app, nil
}

// loadConfiguration loads configuration from file and environment variables
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

// runServer runs the application in server mode
func (app *Application) runServer() error {
	ctx := context.Background()
	
	app.logger.Info("Starting server mode")
	
	// Run database migrations
	if err := app.database.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}
	
	// Seed database if needed
	if *seedDB {
		if err := app.database.Seed(ctx); err != nil {
			return fmt.Errorf("failed to seed database: %w", err)
		}
	}
	
	// Start processing engine
	if err := app.processingEngine.Start(); err != nil {
		return fmt.Errorf("failed to start processing engine: %w", err)
	}
	
	// Setup HTTP server
	router := mux.NewRouter()
	
	// Add middleware
	router.Use(app.handlers.LoggingMiddleware)
	router.Use(app.handlers.RecoveryMiddleware)
	router.Use(app.handlers.CORSMiddleware)
	router.Use(app.handlers.ContentTypeMiddleware)
	
	// Setup routes
	api := router.PathPrefix("/api/v1").Subrouter()
	app.handlers.SetupRoutes(api)
	
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
		app.logger.Info("HTTP server starting",
			"address", app.server.Addr,
			"read_timeout", app.config.Server.ReadTimeout,
			"write_timeout", app.config.Server.WriteTimeout,
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
	
	app.logger.Info("Server started successfully", "address", app.server.Addr)
	
	// Wait for shutdown signal
	return app.waitForShutdown()
}

// runCLI runs the application in CLI mode
func (app *Application) runCLI() error {
	ctx := context.Background()
	
	app.logger.Info("Starting CLI mode")
	
	// Validate CLI flags
	if *fileURL == "" {
		return fmt.Errorf("file-url is required for CLI mode")
	}
	
	if *providerName == "" {
		return fmt.Errorf("provider is required for CLI mode")
	}
	
	// Get provider by name
	provider, err := app.reviewService.GetProviderByName(ctx, *providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider '%s': %w", *providerName, err)
	}
	
	// Start processing engine
	if err := app.processingEngine.Start(); err != nil {
		return fmt.Errorf("failed to start processing engine: %w", err)
	}
	defer app.processingEngine.Stop()
	
	// Submit job
	job, err := app.processingEngine.SubmitJob(ctx, provider.ID, *fileURL)
	if err != nil {
		return fmt.Errorf("failed to submit job: %w", err)
	}
	
	app.logger.Info("File processing job submitted",
		"job_id", job.ID,
		"file_url", *fileURL,
		"provider", *providerName,
	)
	
	// Monitor job progress
	return app.monitorJob(ctx, job.ID)
}

// runMigrations runs database migrations
func (app *Application) runMigrations() error {
	ctx := context.Background()
	
	app.logger.Info("Starting migration mode")
	
	if *resetDB {
		app.logger.Warn("Resetting database - all data will be lost!")
		if err := app.database.Reset(ctx); err != nil {
			return fmt.Errorf("failed to reset database: %w", err)
		}
		app.logger.Info("Database reset completed")
	}
	
	if *migrateUp {
		app.logger.Info("Running database migrations...")
		if err := app.database.Migrate(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		app.logger.Info("Database migrations completed")
	}
	
	if *seedDB {
		app.logger.Info("Seeding database...")
		if err := app.database.Seed(ctx); err != nil {
			return fmt.Errorf("failed to seed database: %w", err)
		}
		app.logger.Info("Database seeding completed")
	}
	
	return nil
}

// processFiles processes multiple files (batch processing)
func (app *Application) processFiles() error {
	ctx := context.Background()
	
	app.logger.Info("Starting batch file processing mode")
	
	// This is a placeholder for batch processing functionality
	// In a real implementation, you might read a list of files from a config file
	// or accept multiple file URLs as command-line arguments
	
	if *fileURL == "" {
		return fmt.Errorf("file-url is required for process-files mode")
	}
	
	if *providerName == "" {
		return fmt.Errorf("provider is required for process-files mode")
	}
	
	// Get provider by name
	provider, err := app.reviewService.GetProviderByName(ctx, *providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider '%s': %w", *providerName, err)
	}
	
	// Start processing engine
	if err := app.processingEngine.Start(); err != nil {
		return fmt.Errorf("failed to start processing engine: %w", err)
	}
	defer app.processingEngine.Stop()
	
	// Process files (for now, just one file)
	fileURLs := strings.Split(*fileURL, ",")
	
	var jobs []*application.ProcessingJob
	for _, url := range fileURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		
		job, err := app.processingEngine.SubmitJob(ctx, provider.ID, url)
		if err != nil {
			app.logger.Error("Failed to submit job", "file_url", url, "error", err)
			continue
		}
		
		jobs = append(jobs, job)
		app.logger.Info("Job submitted", "job_id", job.ID, "file_url", url)
	}
	
	// Monitor all jobs
	return app.monitorJobs(ctx, jobs)
}

// monitorJob monitors a single job until completion
func (app *Application) monitorJob(ctx context.Context, jobID uuid.UUID) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			job, exists := app.processingEngine.GetJobStatus(jobID)
			if !exists {
				return fmt.Errorf("job not found: %s", jobID)
			}
			
			app.logger.Info("Job status",
				"job_id", job.ID,
				"status", job.Status,
				"records_processed", job.RecordsProcessed,
				"records_total", job.RecordsTotal,
			)
			
			switch job.Status {
			case application.StatusCompleted:
				app.logger.Info("Job completed successfully", "job_id", job.ID)
				return nil
			case application.StatusFailed:
				return fmt.Errorf("job failed: %s", job.ErrorMessage)
			case application.StatusCancelled:
				return fmt.Errorf("job was cancelled")
			}
		}
	}
}

// monitorJobs monitors multiple jobs until all complete
func (app *Application) monitorJobs(ctx context.Context, jobs []*application.ProcessingJob) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	completed := make(map[uuid.UUID]bool)
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			allCompleted := true
			
			for _, job := range jobs {
				if completed[job.ID] {
					continue
				}
				
				currentJob, exists := app.processingEngine.GetJobStatus(job.ID)
				if !exists {
					app.logger.Error("Job not found", "job_id", job.ID)
					continue
				}
				
				app.logger.Info("Job status",
					"job_id", currentJob.ID,
					"status", currentJob.Status,
					"records_processed", currentJob.RecordsProcessed,
					"records_total", currentJob.RecordsTotal,
				)
				
				switch currentJob.Status {
				case application.StatusCompleted:
					completed[job.ID] = true
					app.logger.Info("Job completed", "job_id", job.ID)
				case application.StatusFailed:
					completed[job.ID] = true
					app.logger.Error("Job failed", "job_id", job.ID, "error", currentJob.ErrorMessage)
				case application.StatusCancelled:
					completed[job.ID] = true
					app.logger.Warn("Job cancelled", "job_id", job.ID)
				default:
					allCompleted = false
				}
			}
			
			if allCompleted {
				app.logger.Info("All jobs completed")
				return nil
			}
		}
	}
}

// waitForShutdown waits for shutdown signal and performs graceful shutdown
func (app *Application) waitForShutdown() error {
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
	
	// Close database connection
	if err := app.database.Close(); err != nil {
		app.logger.Error("Failed to close database connection", "error", err)
	}
	
	app.logger.Info("Shutdown completed successfully")
	return nil
}

// printHelp prints help information
func printHelp() {
	fmt.Printf(`%s - Hotel Reviews Microservice

USAGE:
    %s [OPTIONS]

OPTIONS:
    -mode string
        Application mode: server, cli, migrate, process-files (default "server")
    -config string
        Path to configuration file
    -log-level string
        Log level override (debug, info, warn, error)
    -host string
        Server host override
    -port int
        Server port override

CLI MODE OPTIONS:
    -file-url string
        S3 URL of the file to process
    -provider string
        Provider name for file processing

MIGRATION OPTIONS:
    -reset-db
        Reset database (drop all tables)
    -migrate-up
        Run database migrations
    -seed-db
        Seed database with initial data

PROCESS-FILES OPTIONS:
    -file-url string
        Comma-separated list of S3 URLs to process
    -provider string
        Provider name for file processing

OTHER OPTIONS:
    -version
        Show version information
    -help
        Show this help message

EXAMPLES:
    # Start server
    %s -mode server

    # Process a single file
    %s -mode cli -file-url s3://bucket/file.jsonl -provider booking

    # Run migrations
    %s -mode migrate -migrate-up -seed-db

    # Process multiple files
    %s -mode process-files -file-url "s3://bucket/file1.jsonl,s3://bucket/file2.jsonl" -provider booking

ENVIRONMENT VARIABLES:
    Configuration can also be provided via environment variables with HOTEL_REVIEWS_ prefix.
    For example: HOTEL_REVIEWS_DATABASE_HOST=localhost

`, AppName, AppName, AppName, AppName, AppName, AppName)
}

// printVersion prints version information
func printVersion() {
	fmt.Printf(`%s
Version: %s
Build: %s
`, AppName, AppVersion, time.Now().Format("2006-01-02"))
}