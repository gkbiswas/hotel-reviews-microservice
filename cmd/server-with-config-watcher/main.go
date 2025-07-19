package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/config"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// Application represents the main application with configuration hot-reloading
type Application struct {
	configManager *config.ConfigManager
	httpServer    *http.Server
	router        *mux.Router
	logger        monitoring.Logger
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewApplication creates a new application instance
func NewApplication() (*Application, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create logger (in a real app, this would be more sophisticated)
	logger := &SimpleLogger{}

	// Create configuration manager
	configManager, err := config.NewConfigManager(logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	app := &Application{
		configManager: configManager,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Initialize application components
	if err := app.initialize(); err != nil {
		cancel()
		configManager.Stop()
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	return app, nil
}

// initialize sets up the application components
func (app *Application) initialize() error {
	// Setup HTTP router
	app.router = mux.NewRouter()
	app.setupRoutes()

	// Create HTTP server with initial configuration
	config := app.configManager.GetConfig()
	if config == nil {
		return fmt.Errorf("no configuration available")
	}

	app.httpServer = &http.Server{
		Addr:         config.GetServerAddr(),
		Handler:      app.router,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
		IdleTimeout:  config.Server.IdleTimeout,
	}

	// Register for configuration changes
	app.configManager.RegisterComponent("http_server", app.handleServerConfigChange)
	app.configManager.RegisterComponent("application", app.handleAppConfigChange)

	// Set component references (for more sophisticated reconfiguration)
	components := map[string]interface{}{
		"server": app.httpServer,
	}
	app.configManager.SetComponentReferences(components)

	app.logger.Info("Application initialized",
		"server_addr", config.GetServerAddr(),
		"environment", config.App.Environment,
		"log_level", config.App.LogLevel)

	return nil
}

// setupRoutes configures HTTP routes
func (app *Application) setupRoutes() {
	// Health check endpoint
	app.router.HandleFunc("/health", app.healthHandler).Methods("GET")
	
	// Configuration endpoints
	app.router.HandleFunc("/config", app.configHandler).Methods("GET")
	app.router.HandleFunc("/config/history", app.configHistoryHandler).Methods("GET")
	app.router.HandleFunc("/config/rollback/{hash}", app.configRollbackHandler).Methods("POST")
	app.router.HandleFunc("/config/metrics", app.configMetricsHandler).Methods("GET")
	
	// Example business endpoints
	app.router.HandleFunc("/api/v1/reviews", app.reviewsHandler).Methods("GET")
	app.router.HandleFunc("/api/v1/hotels", app.hotelsHandler).Methods("GET")
	
	// Admin endpoints
	app.router.HandleFunc("/admin/status", app.statusHandler).Methods("GET")
	
	app.logger.Info("HTTP routes configured")
}

// HTTP Handlers
func (app *Application) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	}

	// Check config manager health
	if err := app.configManager.HealthCheck(); err != nil {
		health["status"] = "unhealthy"
		health["config_manager_error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (app *Application) configHandler(w http.ResponseWriter, r *http.Request) {
	config := app.configManager.GetConfig()
	if config == nil {
		http.Error(w, "No configuration available", http.StatusInternalServerError)
		return
	}

	// Return a summary instead of the full config (security)
	summary := config.GetConfigSummary()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (app *Application) configHistoryHandler(w http.ResponseWriter, r *http.Request) {
	history := app.configManager.GetConfigHistory()
	
	// Return simplified history for API response
	historyResponse := make([]map[string]interface{}, len(history))
	for i, snapshot := range history {
		historyResponse[i] = map[string]interface{}{
			"hash":      snapshot.Hash,
			"timestamp": snapshot.Timestamp,
			"source":    snapshot.Source,
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"history": historyResponse,
		"total":   len(history),
	})
}

func (app *Application) configRollbackHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetHash := vars["hash"]
	
	if targetHash == "" {
		http.Error(w, "Hash parameter is required", http.StatusBadRequest)
		return
	}
	
	app.logger.Info("Configuration rollback requested", "target_hash", targetHash)
	
	if err := app.configManager.RollbackConfig(targetHash); err != nil {
		app.logger.Error("Configuration rollback failed", "error", err, "target_hash", targetHash)
		http.Error(w, fmt.Sprintf("Rollback failed: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "Configuration rolled back successfully",
		"target_hash": targetHash,
		"timestamp":   time.Now().UTC(),
	})
}

func (app *Application) configMetricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := app.configManager.GetMetrics()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (app *Application) reviewsHandler(w http.ResponseWriter, r *http.Request) {
	config := app.configManager.GetConfig()
	
	// Example response that shows current configuration affects business logic
	response := map[string]interface{}{
		"reviews": []map[string]interface{}{
			{
				"id":       1,
				"hotel_id": 1,
				"rating":   4.5,
				"comment":  "Great hotel!",
			},
			{
				"id":       2,
				"hotel_id": 1,
				"rating":   5.0,
				"comment":  "Excellent service!",
			},
		},
		"config_info": map[string]interface{}{
			"cache_ttl":         config.Cache.ReviewTTL.String(),
			"max_request_size":  config.App.MaxRequestSize,
			"rate_limit_enabled": config.App.EnableRateLimit,
		},
		"timestamp": time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (app *Application) hotelsHandler(w http.ResponseWriter, r *http.Request) {
	config := app.configManager.GetConfig()
	
	response := map[string]interface{}{
		"hotels": []map[string]interface{}{
			{
				"id":      1,
				"name":    "Grand Hotel",
				"city":    "New York",
				"rating":  4.5,
			},
			{
				"id":      2,
				"name":    "Ocean View Resort",
				"city":    "Miami",
				"rating":  4.8,
			},
		},
		"config_info": map[string]interface{}{
			"cache_ttl":        config.Cache.HotelTTL.String(),
			"database_host":    config.Database.Host,
			"database_max_conns": config.Database.MaxConns,
		},
		"timestamp": time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (app *Application) statusHandler(w http.ResponseWriter, r *http.Request) {
	config := app.configManager.GetConfig()
	metrics := app.configManager.GetMetrics()
	
	status := map[string]interface{}{
		"application": map[string]interface{}{
			"name":        config.App.Name,
			"version":     config.App.Version,
			"environment": config.App.Environment,
			"uptime":      time.Since(time.Now()).String(), // This would be actual uptime
		},
		"configuration": map[string]interface{}{
			"current_config": config.GetConfigSummary(),
			"metrics":        metrics,
			"health":         app.configManager.HealthCheck() == nil,
		},
		"server": map[string]interface{}{
			"address":      app.httpServer.Addr,
			"tls_enabled":  config.Server.EnableTLS,
			"read_timeout": config.Server.ReadTimeout.String(),
		},
		"timestamp": time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Configuration change handlers
func (app *Application) handleServerConfigChange(oldConfig, newConfig interface{}) error {
	app.logger.Info("Server configuration change detected")
	
	newAppConfig, ok := newConfig.(*config.AppConfig)
	if !ok {
		return fmt.Errorf("invalid config type for server reconfiguration")
	}
	
	// In a real application, we would gracefully restart the server
	// For this example, we'll just update timeouts
	app.httpServer.ReadTimeout = newAppConfig.Server.ReadTimeout
	app.httpServer.WriteTimeout = newAppConfig.Server.WriteTimeout
	app.httpServer.IdleTimeout = newAppConfig.Server.IdleTimeout
	
	app.logger.Info("Server configuration updated",
		"read_timeout", newAppConfig.Server.ReadTimeout,
		"write_timeout", newAppConfig.Server.WriteTimeout,
		"idle_timeout", newAppConfig.Server.IdleTimeout)
	
	return nil
}

func (app *Application) handleAppConfigChange(oldConfig, newConfig interface{}) error {
	app.logger.Info("Application configuration change detected")
	
	newAppConfig, ok := newConfig.(*config.AppConfig)
	if !ok {
		return fmt.Errorf("invalid config type for app reconfiguration")
	}
	
	// Example: Update logging level
	if newAppConfig.App.LogLevel != "" {
		app.logger.Info("Log level updated", "new_level", newAppConfig.App.LogLevel)
		// In a real app, you would reconfigure the actual logger here
	}
	
	// Example: Update max request size (this would affect middleware)
	app.logger.Info("Application settings updated",
		"max_request_size", newAppConfig.App.MaxRequestSize,
		"rate_limit_enabled", newAppConfig.App.EnableRateLimit,
		"auth_enabled", newAppConfig.App.EnableAuthentication)
	
	return nil
}

// Run starts the application
func (app *Application) Run() error {
	// Start HTTP server
	go func() {
		app.logger.Info("Starting HTTP server", "addr", app.httpServer.Addr)
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Error("HTTP server failed", "error", err)
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		app.logger.Info("Received shutdown signal", "signal", sig)
	case <-app.ctx.Done():
		app.logger.Info("Application context cancelled")
	}

	return app.shutdown()
}

// shutdown gracefully shuts down the application
func (app *Application) shutdown() error {
	app.logger.Info("Shutting down application")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if app.httpServer != nil {
		if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
			app.logger.Error("HTTP server shutdown failed", "error", err)
		} else {
			app.logger.Info("HTTP server shut down gracefully")
		}
	}

	// Stop configuration manager
	if app.configManager != nil {
		if err := app.configManager.Stop(); err != nil {
			app.logger.Error("Config manager stop failed", "error", err)
		} else {
			app.logger.Info("Configuration manager stopped")
		}
	}

	app.cancel()
	app.logger.Info("Application shutdown complete")
	return nil
}

// SimpleLogger is a basic logger implementation for the example
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, fields ...interface{}) {
	fmt.Printf("[DEBUG] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Info(msg string, fields ...interface{}) {
	fmt.Printf("[INFO] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Warn(msg string, fields ...interface{}) {
	fmt.Printf("[WARN] %s %v\n", msg, fields)
}

func (l *SimpleLogger) Error(msg string, fields ...interface{}) {
	fmt.Printf("[ERROR] %s %v\n", msg, fields)
}

// main function
func main() {
	fmt.Println("Starting Hotel Reviews Service with Configuration Hot-Reloading...")

	app, err := NewApplication()
	if err != nil {
		fmt.Printf("Failed to create application: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Printf("Application failed: %v\n", err)
		os.Exit(1)
	}
}

// Example usage instructions:
//
// 1. Build and run the application:
//    go run cmd/server-with-config-watcher/main.go
//
// 2. The application will start on http://localhost:8080 (or as configured)
//
// 3. Test endpoints:
//    curl http://localhost:8080/health
//    curl http://localhost:8080/config
//    curl http://localhost:8080/config/history
//    curl http://localhost:8080/config/metrics
//    curl http://localhost:8080/api/v1/reviews
//    curl http://localhost:8080/admin/status
//
// 4. Test configuration hot-reloading:
//    - Edit config/app.json and change the server port to 9090
//    - The application will detect the change and update accordingly
//    - Check the logs to see the configuration change being processed
//
// 5. Test environment variable changes:
//    export APP_LOG_LEVEL=debug
//    export APP_SERVER_PORT=9090
//    - The application will detect these changes and update the configuration
//
// 6. Test configuration rollback:
//    - Get configuration history: curl http://localhost:8080/config/history
//    - Rollback to a previous version: curl -X POST http://localhost:8080/config/rollback/{hash}
//
// 7. Monitor configuration changes:
//    - Watch the application logs to see real-time configuration updates
//    - Check metrics endpoint for statistics about configuration reloads