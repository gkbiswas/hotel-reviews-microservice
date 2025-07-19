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

	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// Demo application showing the config watcher in action
func main() {
	fmt.Println("🚀 Hotel Reviews Config Watcher Demo")
	fmt.Println("=====================================")

	// Create logger
	logger := monitoring.NewLogger("config-demo", "info")

	// Create config watcher with fast polling for demo
	options := &infrastructure.ConfigWatcherOptions{
		WatchIntervalSec:    1, // Check every second for demo
		FileChecksum:        true,
		EnvCheckIntervalSec: 5, // Check env vars every 5 seconds
		EnableDebugLogging:  true,
		LogConfigChanges:    true,
		MaxHistorySize:      10,
		EnableRollback:      true,
	}

	watcher, err := infrastructure.NewConfigWatcher(logger, options)
	if err != nil {
		logger.Error("Failed to create config watcher", "error", err)
		os.Exit(1)
	}

	// Demo configuration
	demoConfig := map[string]interface{}{
		"server": map[string]interface{}{
			"port":    8080,
			"host":    "localhost",
			"timeout": "30s",
		},
		"app": map[string]interface{}{
			"name":      "hotel-reviews-demo",
			"log_level": "info",
			"debug":     false,
		},
		"cache": map[string]interface{}{
			"ttl":     "1h",
			"enabled": true,
		},
	}

	// Register initial configuration
	err = watcher.RegisterConfig("demo_config", demoConfig)
	if err != nil {
		logger.Error("Failed to register demo config", "error", err)
		os.Exit(1)
	}

	// Setup change tracking
	configChangeCount := 0
	watcher.RegisterChangeCallback("demo_config", func(configName string, oldConfig, newConfig interface{}) error {
		configChangeCount++
		logger.Info("🔄 Configuration changed!",
			"change_number", configChangeCount,
			"config_name", configName)

		// Show what changed
		if oldConfigMap, ok := oldConfig.(map[string]interface{}); ok {
			if newConfigMap, ok := newConfig.(map[string]interface{}); ok {
				showConfigDiff(logger, oldConfigMap, newConfigMap)
			}
		}

		return nil
	})

	// Watch environment variables
	envVars := []string{"DEMO_LOG_LEVEL", "DEMO_PORT", "DEMO_DEBUG"}
	for _, envVar := range envVars {
		err := watcher.WatchEnvVar("demo_env", envVar)
		if err != nil {
			logger.Warn("Failed to watch env var", "var", envVar, "error", err)
		}
	}

	// Setup env var callbacks
	watcher.RegisterChangeCallback("env:DEMO_LOG_LEVEL", func(configName string, oldConfig, newConfig interface{}) error {
		logger.Info("🌍 Environment variable changed",
			"var", "DEMO_LOG_LEVEL",
			"old", oldConfig,
			"new", newConfig)
		return nil
	})

	watcher.RegisterChangeCallback("env:DEMO_PORT", func(configName string, oldConfig, newConfig interface{}) error {
		logger.Info("🌍 Environment variable changed",
			"var", "DEMO_PORT",
			"old", oldConfig,
			"new", newConfig)
		return nil
	})

	// Create HTTP server for demo API
	router := mux.NewRouter()
	setupDemoRoutes(router, watcher, logger)

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start HTTP server
	go func() {
		logger.Info("🌐 Starting demo HTTP server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
		}
	}()

	// Print demo instructions
	printDemoInstructions()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("🛑 Shutting down demo...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
	}

	if err := watcher.Stop(); err != nil {
		logger.Error("Config watcher stop failed", "error", err)
	}

	logger.Info("✅ Demo shutdown complete")
}

func setupDemoRoutes(router *mux.Router, watcher *infrastructure.ConfigWatcher, logger monitoring.Logger) {
	// API to get current configuration
	router.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		config, exists := watcher.GetConfig("demo_config")
		if !exists {
			http.Error(w, "No configuration found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"config":  config,
		})
	}).Methods("GET")

	// API to get configuration history
	router.HandleFunc("/config/history", func(w http.ResponseWriter, r *http.Request) {
		history := watcher.GetConfigHistory("demo_config")

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
			"success": true,
			"history": historyResponse,
			"total":   len(history),
		})
	}).Methods("GET")

	// API to rollback configuration
	router.HandleFunc("/config/rollback/{hash}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		hash := vars["hash"]

		if hash == "" {
			http.Error(w, "Hash parameter required", http.StatusBadRequest)
			return
		}

		logger.Info("🔄 Rollback requested", "hash", hash)

		err := watcher.RollbackConfig("demo_config", hash)
		if err != nil {
			logger.Error("Rollback failed", "error", err, "hash", hash)
			http.Error(w, fmt.Sprintf("Rollback failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Configuration rolled back successfully",
			"hash":    hash,
		})
	}).Methods("POST")

	// API to get metrics
	router.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := watcher.GetMetrics()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"metrics": metrics,
		})
	}).Methods("GET")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":         "healthy",
			"watcher_health": watcher.HealthCheck() == nil,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	}).Methods("GET")
}

func showConfigDiff(logger monitoring.Logger, oldConfig, newConfig map[string]interface{}) {
	logger.Info("📋 Configuration differences:")

	// Check for changes in nested maps
	for key, newValue := range newConfig {
		if oldValue, exists := oldConfig[key]; exists {
			if !isEqual(oldValue, newValue) {
				logger.Info("  🔸 Changed", "key", key, "old", oldValue, "new", newValue)
			}
		} else {
			logger.Info("  ➕ Added", "key", key, "value", newValue)
		}
	}

	// Check for removed keys
	for key, oldValue := range oldConfig {
		if _, exists := newConfig[key]; !exists {
			logger.Info("  ➖ Removed", "key", key, "value", oldValue)
		}
	}
}

func isEqual(a, b interface{}) bool {
	aBytes, _ := json.Marshal(a)
	bBytes, _ := json.Marshal(b)
	return string(aBytes) == string(bBytes)
}

func printDemoInstructions() {
	fmt.Println("")
	fmt.Println("🎯 Demo Instructions:")
	fmt.Println("====================")
	fmt.Println("")
	fmt.Println("📡 API Endpoints:")
	fmt.Println("  GET  http://localhost:8080/config          - View current config")
	fmt.Println("  GET  http://localhost:8080/config/history   - View config history")
	fmt.Println("  POST http://localhost:8080/config/rollback/{hash} - Rollback config")
	fmt.Println("  GET  http://localhost:8080/metrics          - View watcher metrics")
	fmt.Println("  GET  http://localhost:8080/health           - Health check")
	fmt.Println("")
	fmt.Println("🌍 Environment Variables to Test:")
	fmt.Println("  export DEMO_LOG_LEVEL=debug")
	fmt.Println("  export DEMO_PORT=9090")
	fmt.Println("  export DEMO_DEBUG=true")
	fmt.Println("")
	fmt.Println("📝 Configuration Updates:")
	fmt.Println("  The watcher is monitoring for changes every second")
	fmt.Println("  Watch the logs to see real-time configuration updates!")
	fmt.Println("")
	fmt.Println("🎮 Try these commands in another terminal:")
	fmt.Println("  curl http://localhost:8080/config")
	fmt.Println("  curl http://localhost:8080/config/history")
	fmt.Println("  curl http://localhost:8080/metrics")
	fmt.Println("  export DEMO_LOG_LEVEL=debug  # Watch the logs!")
	fmt.Println("")
}
