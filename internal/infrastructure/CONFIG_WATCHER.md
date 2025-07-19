# Configuration Hot-Reloading with ConfigWatcher

The `ConfigWatcher` provides comprehensive configuration hot-reloading capabilities for the Hotel Reviews microservice, enabling graceful configuration changes without service restarts.

## Features

- **File Watching**: Monitor JSON and YAML configuration files for changes
- **Environment Variable Monitoring**: Track changes to environment variables
- **Validation**: Built-in struct validation and custom validation functions
- **Rollback**: Complete configuration history with rollback capabilities
- **Graceful Updates**: Atomic configuration updates with callback notifications
- **Error Handling**: Retry logic and graceful error recovery
- **Metrics**: Comprehensive metrics and health checks
- **Concurrency Safe**: Thread-safe operations with proper locking

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/hotel-reviews/internal/infrastructure"
    "github.com/hotel-reviews/internal/monitoring"
)

func main() {
    logger := monitoring.NewLogger("config-watcher", "info")
    
    // Create watcher with default options
    watcher, err := infrastructure.NewConfigWatcher(logger, nil)
    if err != nil {
        panic(err)
    }
    defer watcher.Stop()
    
    // Register a configuration
    config := map[string]interface{}{
        "server": map[string]interface{}{
            "port": 8080,
            "host": "localhost",
        },
    }
    
    err = watcher.RegisterConfig("app_config", config)
    if err != nil {
        panic(err)
    }
    
    // Watch a configuration file
    err = watcher.WatchFile("app_config", "./config/app.json")
    if err != nil {
        panic(err)
    }
    
    // Register change callback
    watcher.RegisterChangeCallback("app_config", func(configName string, oldConfig, newConfig interface{}) error {
        fmt.Printf("Configuration changed: %s\n", configName)
        // Reconfigure your application here
        return nil
    })
    
    // Keep running
    select {}
}
```

### Advanced Configuration

```go
options := &infrastructure.ConfigWatcherOptions{
    // File watching options
    WatchIntervalSec:    2,           // Check files every 2 seconds
    FileChecksum:        true,        // Use checksums to detect changes
    IgnoreHiddenFiles:   true,        // Ignore .hidden files
    
    // Environment variable monitoring
    EnvCheckIntervalSec: 30,          // Check env vars every 30 seconds
    EnvVarPrefix:        "APP_",      // Monitor vars starting with APP_
    
    // Validation and rollback
    ValidateOnLoad:      true,        // Validate configs when loaded
    EnableRollback:      true,        // Enable rollback functionality
    MaxHistorySize:      20,          // Keep 20 historical snapshots
    RollbackTimeoutSec:  30,          // Rollback timeout
    
    // Error handling
    MaxRetries:          3,           // Retry failed operations 3 times
    RetryDelaySec:       5,           // Wait 5 seconds between retries
    FailOnValidation:    false,       // Don't fail on validation errors
    
    // Performance
    BatchUpdates:        true,        // Batch multiple updates
    BatchIntervalMs:     1000,        // Batch updates every second
    
    // Debugging
    EnableDebugLogging:  true,        // Enable debug logs
    LogConfigChanges:    true,        // Log all config changes
}

watcher, err := infrastructure.NewConfigWatcher(logger, options)
```

## Configuration File Formats

### JSON Configuration

```json
{
  "server": {
    "host": "localhost",
    "port": 8080,
    "timeout": "30s"
  },
  "database": {
    "host": "localhost",
    "port": 5432,
    "name": "hotel_reviews",
    "max_connections": 25
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

### YAML Configuration

```yaml
server:
  host: localhost
  port: 8080
  timeout: 30s

database:
  host: localhost
  port: 5432
  name: hotel_reviews
  max_connections: 25

logging:
  level: info
  format: json
```

## Environment Variable Monitoring

Monitor environment variables for configuration changes:

```go
// Watch specific environment variables
envVars := []string{
    "APP_LOG_LEVEL",
    "APP_DB_HOST",
    "APP_DB_PORT",
    "APP_REDIS_HOST",
    "APP_SERVER_PORT",
}

for _, envVar := range envVars {
    err := watcher.WatchEnvVar("env_config", envVar)
    if err != nil {
        logger.Warn("Failed to watch env var", "var", envVar, "error", err)
    }
}

// Handle environment variable changes
watcher.RegisterChangeCallback("env:APP_LOG_LEVEL", func(configName string, oldConfig, newConfig interface{}) error {
    newLevel := newConfig.(string)
    logger.Info("Log level changed", "old", oldConfig, "new", newLevel)
    
    // Reconfigure your logger here
    return reconfigureLogger(newLevel)
})
```

## Validation

### Struct Validation

Use struct tags for automatic validation:

```go
type ServerConfig struct {
    Host    string        `json:"host" validate:"required"`
    Port    int           `json:"port" validate:"min=1,max=65535"`
    Timeout time.Duration `json:"timeout" validate:"min=1s"`
}
```

### Custom Validation

Register custom validation functions:

```go
watcher.RegisterValidator("app_config", func(config interface{}) error {
    configMap := config.(map[string]interface{})
    
    // Custom business logic validation
    if server, exists := configMap["server"].(map[string]interface{}); exists {
        if port, exists := server["port"].(float64); exists {
            if port < 1024 && os.Getuid() != 0 {
                return fmt.Errorf("non-root user cannot bind to port %d", int(port))
            }
        }
    }
    
    return nil
})
```

## Change Callbacks

Register callbacks to handle configuration changes:

```go
watcher.RegisterChangeCallback("app_config", func(configName string, oldConfig, newConfig interface{}) error {
    logger.Info("Configuration updated", "config", configName)
    
    // Parse the new configuration
    var newAppConfig AppConfig
    if err := mapToStruct(newConfig.(map[string]interface{}), &newAppConfig); err != nil {
        return fmt.Errorf("failed to parse config: %w", err)
    }
    
    // Reconfigure application components
    if err := reconfigureServer(newAppConfig.Server); err != nil {
        return fmt.Errorf("failed to reconfigure server: %w", err)
    }
    
    if err := reconfigureDatabase(newAppConfig.Database); err != nil {
        return fmt.Errorf("failed to reconfigure database: %w", err)
    }
    
    logger.Info("Application reconfigured successfully")
    return nil
})
```

## Rollback Functionality

The ConfigWatcher maintains a history of configuration changes and supports rollback:

```go
// Get configuration history
history := watcher.GetConfigHistory("app_config")
for _, snapshot := range history {
    fmt.Printf("Snapshot: %s (source: %s, time: %s)\n", 
        snapshot.Hash, snapshot.Source, snapshot.Timestamp)
}

// Rollback to a previous configuration
if len(history) > 1 {
    targetHash := history[len(history)-2].Hash // Previous configuration
    err := watcher.RollbackConfig("app_config", targetHash)
    if err != nil {
        logger.Error("Rollback failed", "error", err)
    } else {
        logger.Info("Configuration rolled back", "hash", targetHash)
    }
}
```

## Error Handling

The ConfigWatcher provides several error handling mechanisms:

### Retry Logic

Failed operations are automatically retried:

```go
options := &ConfigWatcherOptions{
    MaxRetries:    3,   // Retry up to 3 times
    RetryDelaySec: 5,   // Wait 5 seconds between retries
}
```

### Validation Errors

Control how validation errors are handled:

```go
options := &ConfigWatcherOptions{
    ValidateOnLoad:   true,   // Validate configurations when loaded
    FailOnValidation: false,  // Don't fail on validation errors (log warning instead)
}
```

### Callback Errors

If a callback fails, the configuration change is rolled back:

```go
watcher.RegisterChangeCallback("app_config", func(configName string, oldConfig, newConfig interface{}) error {
    if err := reconfigureApplication(newConfig); err != nil {
        // This error will cause the configuration change to be rolled back
        return fmt.Errorf("reconfiguration failed: %w", err)
    }
    return nil
})
```

## Metrics and Monitoring

Get comprehensive metrics about the configuration watcher:

```go
metrics := watcher.GetMetrics()
fmt.Printf("Metrics: %+v\n", metrics)

// Example output:
// {
//   "reload_count": 15,
//   "error_count": 2,
//   "last_reload_time": "2023-01-15T10:30:00Z",
//   "watched_files": 3,
//   "watched_env_vars": 5,
//   "registered_configs": 2,
//   "total_callbacks": 4,
//   "history_entries": 25
// }
```

### Health Check

Monitor the health of the configuration watcher:

```go
if err := watcher.HealthCheck(); err != nil {
    logger.Error("Config watcher health check failed", "error", err)
    // Take corrective action
}
```

## Performance Optimization

### Batch Updates

Enable batch updates to improve performance when multiple configuration changes occur:

```go
options := &ConfigWatcherOptions{
    BatchUpdates:    true,  // Enable batching
    BatchIntervalMs: 1000,  // Batch updates every second
}
```

### File Checksums

Use checksums to avoid unnecessary reloads:

```go
options := &ConfigWatcherOptions{
    FileChecksum: true,  // Only reload if file content actually changed
}
```

## Best Practices

### 1. Graceful Reconfiguration

Always implement graceful reconfiguration in your callbacks:

```go
func reconfigureServer(newConfig ServerConfig) error {
    // Validate the new configuration first
    if newConfig.Port < 1 || newConfig.Port > 65535 {
        return fmt.Errorf("invalid port: %d", newConfig.Port)
    }
    
    // Create new server with new configuration
    newServer, err := createServer(newConfig)
    if err != nil {
        return fmt.Errorf("failed to create new server: %w", err)
    }
    
    // Gracefully shutdown old server
    if oldServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := oldServer.Shutdown(ctx); err != nil {
            logger.Warn("Failed to gracefully shutdown old server", "error", err)
        }
    }
    
    // Start new server
    go newServer.Start()
    
    return nil
}
```

### 2. Configuration Validation

Always validate configurations thoroughly:

```go
type DatabaseConfig struct {
    Host        string        `json:"host" validate:"required,hostname_rfc1123"`
    Port        int           `json:"port" validate:"min=1,max=65535"`
    Database    string        `json:"database" validate:"required,min=1,max=63"`
    Username    string        `json:"username" validate:"required"`
    Password    string        `json:"password" validate:"required,min=8"`
    MaxConns    int           `json:"max_conns" validate:"min=1,max=1000"`
    ConnTimeout time.Duration `json:"conn_timeout" validate:"min=1s,max=1m"`
}
```

### 3. Error Recovery

Implement proper error recovery mechanisms:

```go
watcher.RegisterChangeCallback("critical_config", func(configName string, oldConfig, newConfig interface{}) error {
    // Attempt to apply new configuration
    if err := applyNewConfig(newConfig); err != nil {
        logger.Error("Failed to apply new config, rolling back", "error", err)
        
        // Rollback to previous configuration
        if oldConfig != nil {
            if rollbackErr := applyNewConfig(oldConfig); rollbackErr != nil {
                logger.Error("Rollback also failed!", "error", rollbackErr)
                // Emergency shutdown or alerting
                return fmt.Errorf("both config update and rollback failed: %w", rollbackErr)
            }
        }
        
        return err
    }
    
    logger.Info("Configuration applied successfully")
    return nil
})
```

### 4. Testing Configuration Changes

Test configuration changes in a safe environment:

```go
func TestConfigurationHotReload(t *testing.T) {
    logger := createTestLogger()
    watcher, err := NewConfigWatcher(logger, nil)
    require.NoError(t, err)
    defer watcher.Stop()
    
    // Setup test configuration
    configFile := createTempConfigFile(t, testConfig, "json")
    
    changeDetected := make(chan bool, 1)
    watcher.RegisterChangeCallback("test_config", func(string, interface{}, interface{}) error {
        changeDetected <- true
        return nil
    })
    
    err = watcher.WatchFile("test_config", configFile)
    require.NoError(t, err)
    
    // Modify configuration
    newConfig := modifyTestConfig(testConfig)
    writeConfigFile(t, configFile, newConfig)
    
    // Verify change detection
    select {
    case <-changeDetected:
        // Test passed
    case <-time.After(5 * time.Second):
        t.Fatal("Configuration change not detected")
    }
}
```

## Integration Examples

### With HTTP Server

```go
type HTTPServerWrapper struct {
    server *http.Server
    config ServerConfig
    mu     sync.RWMutex
}

func (w *HTTPServerWrapper) ReconfigureServer(newConfig ServerConfig) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    // Create new server
    newServer := &http.Server{
        Addr:         fmt.Sprintf("%s:%d", newConfig.Host, newConfig.Port),
        Handler:      w.server.Handler,
        ReadTimeout:  newConfig.ReadTimeout,
        WriteTimeout: newConfig.WriteTimeout,
        IdleTimeout:  newConfig.IdleTimeout,
    }
    
    // Graceful shutdown of old server
    if w.server != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        w.server.Shutdown(ctx)
    }
    
    // Start new server
    w.server = newServer
    w.config = newConfig
    
    go func() {
        if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Printf("Server error: %v", err)
        }
    }()
    
    return nil
}
```

### With Database Pool

```go
func reconfigureDatabasePool(newConfig DatabaseConfig) error {
    // Create new connection pool with new configuration
    newPool, err := createDatabasePool(newConfig)
    if err != nil {
        return fmt.Errorf("failed to create new database pool: %w", err)
    }
    
    // Test the new connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := newPool.Ping(ctx); err != nil {
        newPool.Close()
        return fmt.Errorf("failed to ping database with new config: %w", err)
    }
    
    // Replace old pool
    oldPool := getCurrentPool()
    setCurrentPool(newPool)
    
    // Close old pool gracefully
    if oldPool != nil {
        go func() {
            time.Sleep(30 * time.Second) // Give time for existing operations
            oldPool.Close()
        }()
    }
    
    return nil
}
```

## Troubleshooting

### Common Issues

1. **File permissions**: Ensure the process has read permissions for configuration files
2. **File system events**: Some file systems may not support file watching reliably
3. **Environment variables**: Changes to environment variables require process restart on some systems
4. **Validation errors**: Check struct tags and custom validators for correct syntax

### Debug Logging

Enable debug logging to troubleshoot issues:

```go
options := &ConfigWatcherOptions{
    EnableDebugLogging: true,
    LogConfigChanges:   true,
}
```

### Health Monitoring

Regularly check the health of the configuration watcher:

```go
// Set up a health check endpoint
http.HandleFunc("/health/config", func(w http.ResponseWriter, r *http.Request) {
    if err := watcher.HealthCheck(); err != nil {
        http.Error(w, err.Error(), http.StatusServiceUnavailable)
        return
    }
    
    metrics := watcher.GetMetrics()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
})
```