//go:build examples
// +build examples

package main
import (
	"context"
	"log"
	"net/http"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/server"
)
// Example logger implementation
type exampleShutdownLogger struct{}
func (l *exampleShutdownLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("DEBUG: %s %v\n", msg, fields)
}
func (l *exampleShutdownLogger) Info(msg string, fields ...interface{}) {
	log.Printf("INFO: %s %v\n", msg, fields)
}
func (l *exampleShutdownLogger) Warn(msg string, fields ...interface{}) {
	log.Printf("WARN: %s %v\n", msg, fields)
}
func (l *exampleShutdownLogger) Error(msg string, fields ...interface{}) {
	log.Printf("ERROR: %s %v\n", msg, fields)
}
// Example background worker
type BackgroundProcessor struct {
	name    string
	logger  monitoring.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	stopped chan struct{}
}
func NewBackgroundProcessor(name string, logger monitoring.Logger) *BackgroundProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	bp := &BackgroundProcessor{
		name:    name,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		stopped: make(chan struct{}),
	}
	// Start processing
	go bp.run()
	return bp
}
func (bp *BackgroundProcessor) run() {
	defer close(bp.stopped)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-bp.ctx.Done():
			bp.logger.Info("Background processor stopping", "name", bp.name)
			return
		case <-ticker.C:
			bp.logger.Debug("Background processor tick", "name", bp.name)
			// Simulate work
			time.Sleep(100 * time.Millisecond)
		}
	}
}
func (bp *BackgroundProcessor) Stop(ctx context.Context) error {
	bp.logger.Info("Stopping background processor", "name", bp.name)
	bp.cancel()
	select {
	case <-bp.stopped:
		bp.logger.Info("Background processor stopped gracefully", "name", bp.name)
		return nil
	case <-ctx.Done():
		bp.logger.Warn("Background processor stop timed out", "name", bp.name)
		return ctx.Err()
	}
}
func (bp *BackgroundProcessor) Name() string {
	return bp.name
}
// Example external service connection
type ExternalAPIClient struct {
	name   string
	logger monitoring.Logger
	client *http.Client
}
func NewExternalAPIClient(name string, logger monitoring.Logger) *ExternalAPIClient {
	return &ExternalAPIClient{
		name:   name,
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}
func (c *ExternalAPIClient) Close(ctx context.Context) error {
	c.logger.Info("Closing external API client", "name", c.name)
	// Simulate cleanup work
	time.Sleep(100 * time.Millisecond)
	c.client.CloseIdleConnections()
	c.logger.Info("External API client closed", "name", c.name)
	return nil
}
func (c *ExternalAPIClient) Name() string {
	return c.name
}
// Example HTTP handler
func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, World!"))
}
// Example application setup and graceful shutdown
func main() {
	logger := &exampleShutdownLogger{}
	logger.Info("Starting hotel reviews microservice...")
	// Create shutdown manager with custom configuration
	shutdownConfig := server.DefaultShutdownConfig()
	shutdownConfig.GracefulTimeout = 45 * time.Second
	shutdownConfig.PreShutdownDelay = 2 * time.Second
	shutdownConfig.LogShutdownProgress = true
	shutdownManager := server.NewShutdownManager(logger, shutdownConfig)
	// Create resource manager to organize all resources
	resourceManager := server.NewResourceManager()
	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	resourceManager.AddHTTPServer(httpServer)
	// Setup database connection (mock for example)
	// In real application, you would create actual database pool
	logger.Info("Setting up database connection pool...")
	// dbPool, err := pgxpool.New(context.Background(), "postgres://...")
	// if err != nil {
	//     log.Fatalf("Failed to create database pool: %v", err)
	// }
	// resourceManager.AddDatabase(dbPool)
	// Setup Redis connection (mock for example)
	logger.Info("Setting up Redis connection...")
	// redisClient := redis.NewClient(&redis.Options{
	//     Addr: "localhost:6379",
	// })
	// resourceManager.AddRedis(redisClient)
	// Setup background workers
	logger.Info("Starting background workers...")
	reviewProcessor := NewBackgroundProcessor("review_processor", logger)
	resourceManager.AddWorker(reviewProcessor)
	emailWorker := NewBackgroundProcessor("email_worker", logger)
	resourceManager.AddWorker(emailWorker)
	analyticsWorker := NewBackgroundProcessor("analytics_worker", logger)
	resourceManager.AddWorker(analyticsWorker)
	// Setup external service connections
	logger.Info("Setting up external service connections...")
	paymentAPI := NewExternalAPIClient("payment_service", logger)
	resourceManager.AddExternalConnection(paymentAPI)
	notificationAPI := NewExternalAPIClient("notification_service", logger)
	resourceManager.AddExternalConnection(notificationAPI)
	// Setup database optimizer (mock for example)
	// In real application, you would create actual optimizer
	logger.Info("Setting up database optimizer...")
	// dbOptimizer, err := infrastructure.NewDBOptimizer(dbPool, logger, nil)
	// if err != nil {
	//     log.Fatalf("Failed to create database optimizer: %v", err)
	// }
	// optimizerWrapper := server.NewDBOptimizerWrapper(dbOptimizer)
	// resourceManager.AddOptimizer(optimizerWrapper)
	// Setup configuration watcher (mock for example)
	logger.Info("Setting up configuration watcher...")
	// configWatcher, err := infrastructure.NewConfigWatcher(logger, nil)
	// if err != nil {
	//     log.Fatalf("Failed to create config watcher: %v", err)
	// }
	// watcherWrapper := server.NewConfigWatcherWrapper(configWatcher)
	// resourceManager.AddWatcher(watcherWrapper)
	// Register all resources with shutdown manager
	err := shutdownManager.RegisterResource(resourceManager)
	if err != nil {
		log.Fatalf("Failed to register resources: %v", err)
	}
	// Add custom shutdown hooks
	shutdownManager.AddCustomHook(server.ShutdownHook{
		Name:     "cleanup_temp_files",
		Priority: 5, // Run after other resources
		Timeout:  5 * time.Second,
		Cleanup: func(ctx context.Context) error {
			logger.Info("Cleaning up temporary files...")
			// Simulate cleanup work
			time.Sleep(500 * time.Millisecond)
			logger.Info("Temporary files cleaned up")
			return nil
		},
	})
	shutdownManager.AddCustomHook(server.ShutdownHook{
		Name:     "flush_metrics",
		Priority: 4, // Run before file cleanup
		Timeout:  3 * time.Second,
		Cleanup: func(ctx context.Context) error {
			logger.Info("Flushing metrics...")
			// Simulate metrics flush
			time.Sleep(200 * time.Millisecond)
			logger.Info("Metrics flushed")
			return nil
		},
	})
	// Start HTTP server in background
	go func() {
		logger.Info("Starting HTTP server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
		}
	}()
	// Print shutdown metrics
	metrics := shutdownManager.GetShutdownMetrics()
	logger.Info("Shutdown manager initialized",
		"registered_hooks", metrics["registered_hooks"],
		"graceful_timeout", metrics["graceful_timeout"])
	logger.Info("Application started successfully. Press Ctrl+C to shutdown gracefully.")
	// Wait for shutdown signal and perform graceful shutdown
	if err := shutdownManager.WaitForShutdown(); err != nil {
		logger.Error("Shutdown completed with errors", "error", err)
	} else {
		logger.Info("Graceful shutdown completed successfully")
	}
}
// Example of how to integrate shutdown manager with different resource types
func ExampleIntegrationPatterns() {
	logger := &exampleShutdownLogger{}
	shutdownManager := server.NewShutdownManager(logger, nil)
	// Pattern 1: Direct resource registration
	worker := NewBackgroundProcessor("direct_worker", logger)
	shutdownManager.RegisterResource(worker)
	// Pattern 2: Using resource manager for bulk registration
	rm := server.NewResourceManager()
	rm.AddWorker(NewBackgroundProcessor("rm_worker1", logger))
	rm.AddWorker(NewBackgroundProcessor("rm_worker2", logger))
	rm.AddExternalConnection(NewExternalAPIClient("api_client", logger))
	shutdownManager.RegisterResource(rm)
	// Pattern 3: Custom shutdown hooks for specialized cleanup
	shutdownManager.AddCustomHook(server.ShutdownHook{
		Name:     "custom_cleanup",
		Priority: 10,
		Timeout:  5 * time.Second,
		Cleanup: func(ctx context.Context) error {
			// Custom cleanup logic
			return nil
		},
	})
	// Pattern 4: Programmatic shutdown trigger
	go func() {
		// Some condition that triggers shutdown
		time.Sleep(30 * time.Second)
		shutdownManager.Shutdown()
	}()
	// Wait for shutdown
	shutdownManager.WaitForShutdown()
}
// Example of how to handle different database types
func ExampleDatabaseIntegration(logger monitoring.Logger, shutdownManager *server.ShutdownManager) {
	// PostgreSQL with pgxpool
	dbConfig, _ := pgxpool.ParseConfig("postgres://user:pass@localhost/db")
	dbPool, _ := pgxpool.NewWithConfig(context.Background(), dbConfig)
	shutdownManager.RegisterResource(dbPool)
	// Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	shutdownManager.RegisterResource(redisClient)
	// Database optimizer integration
	// dbOptimizer, _ := infrastructure.NewDBOptimizer(dbPool, logger, nil)
	// optimizerWrapper := server.NewDBOptimizerWrapper(dbOptimizer)
	// shutdownManager.RegisterResource(optimizerWrapper)
}
// Example of monitoring shutdown metrics
func ExampleMonitoringIntegration(shutdownManager *server.ShutdownManager) {
	// Get current metrics
	metrics := shutdownManager.GetShutdownMetrics()
	// Example of how you might expose these via HTTP endpoint
	http.HandleFunc("/metrics/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// In real application, you would properly serialize metrics
		log.Printf("Shutdown metrics: %+v", metrics)
	})
	// Example of periodic metrics collection
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			metrics := shutdownManager.GetShutdownMetrics()
			log.Printf("Current shutdown state: %+v", metrics)
		}
	}()
}
