package config

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// ConfigManager manages application configuration with hot-reloading
type ConfigManager struct {
	watcher     *infrastructure.ConfigWatcher
	config      *AppConfig
	configMutex sync.RWMutex
	logger      monitoring.Logger
	ctx         context.Context
	cancel      context.CancelFunc

	// Component references for reconfiguration
	server           *http.Server
	dbPool           interface{} // Database pool interface
	redisClient      interface{} // Redis client interface
	cacheService     *application.CacheService
	authMiddleware   *application.AuthMiddleware
	eventHandler     *application.BaseEventHandler
	errorHandler     *infrastructure.ErrorHandler
	monitoringSystem *monitoring.Service
	circuitBreaker   *infrastructure.CircuitBreaker

	// Configuration change metrics
	reloadCount    int64
	lastReloadTime time.Time
	errorCount     int64

	// Callbacks for component updates
	componentCallbacks map[string][]ComponentUpdateCallback
}

// ComponentUpdateCallback is called when a component needs reconfiguration
type ComponentUpdateCallback func(oldConfig, newConfig interface{}) error

// NewConfigManager creates a new configuration manager with hot-reloading
func NewConfigManager(logger monitoring.Logger) (*ConfigManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create config watcher with optimized settings
	watcherOptions := &infrastructure.ConfigWatcherOptions{
		WatchIntervalSec:    2,
		FileChecksum:        true,
		IgnoreHiddenFiles:   true,
		EnvCheckIntervalSec: 30,
		EnvVarPrefix:        "APP_",
		ValidateOnLoad:      true,
		EnableRollback:      true,
		MaxHistorySize:      50,
		RollbackTimeoutSec:  30,
		MaxRetries:          3,
		RetryDelaySec:       5,
		FailOnValidation:    false,
		BatchUpdates:        true,
		BatchIntervalMs:     1000,
		EnableDebugLogging:  true,
		LogConfigChanges:    true,
	}

	watcher, err := infrastructure.NewConfigWatcher(logger, watcherOptions)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create config watcher: %w", err)
	}

	manager := &ConfigManager{
		watcher:            watcher,
		logger:             logger,
		ctx:                ctx,
		cancel:             cancel,
		componentCallbacks: make(map[string][]ComponentUpdateCallback),
	}

	// Setup configuration monitoring
	if err := manager.setupConfigurationWatching(); err != nil {
		cancel()
		watcher.Stop()
		return nil, fmt.Errorf("failed to setup configuration watching: %w", err)
	}

	return manager, nil
}

// setupConfigurationWatching configures all configuration monitoring
func (cm *ConfigManager) setupConfigurationWatching() error {
	// Register main config change callback
	cm.watcher.RegisterChangeCallback("app_config", cm.handleAppConfigChange)

	// Register environment variable callbacks
	envCallbacks := map[string]func(string, interface{}, interface{}) error{
		"env:APP_LOG_LEVEL":        cm.handleLogLevelChange,
		"env:APP_DB_HOST":          cm.handleDatabaseHostChange,
		"env:APP_DB_PORT":          cm.handleDatabasePortChange,
		"env:APP_REDIS_HOST":       cm.handleRedisHostChange,
		"env:APP_REDIS_PORT":       cm.handleRedisPortChange,
		"env:APP_SERVER_PORT":      cm.handleServerPortChange,
		"env:APP_JWT_SECRET":       cm.handleJWTSecretChange,
		"env:APP_ENVIRONMENT":      cm.handleEnvironmentChange,
		"env:APP_MAX_CONNECTIONS":  cm.handleMaxConnectionsChange,
	}

	for envKey, callback := range envCallbacks {
		cm.watcher.RegisterChangeCallback(envKey, callback)
	}

	// Register custom validators
	cm.watcher.RegisterValidator("app_config", cm.validateAppConfig)

	// Watch main configuration file
	configPath := cm.getConfigFilePath()
	if _, err := os.Stat(configPath); err == nil {
		if err := cm.watcher.WatchFile("app_config", configPath); err != nil {
			return fmt.Errorf("failed to watch config file %s: %w", configPath, err)
		}
		cm.logger.Info("Watching main configuration file", "path", configPath)
	} else {
		cm.logger.Info("Configuration file not found, using defaults", "path", configPath)
		
		// Register default configuration
		defaultConfig := GetDefaultConfig()
		if err := cm.watcher.RegisterConfig("app_config", defaultConfig); err != nil {
			return fmt.Errorf("failed to register default config: %w", err)
		}
		cm.setConfig(defaultConfig)
	}

	// Watch environment variables
	envVars := []string{
		"APP_LOG_LEVEL", "APP_DB_HOST", "APP_DB_PORT", "APP_REDIS_HOST",
		"APP_REDIS_PORT", "APP_SERVER_PORT", "APP_JWT_SECRET", 
		"APP_ENVIRONMENT", "APP_MAX_CONNECTIONS",
	}

	for _, envVar := range envVars {
		if err := cm.watcher.WatchEnvVar("env_monitoring", envVar); err != nil {
			cm.logger.Warn("Failed to watch environment variable", "var", envVar, "error", err)
		}
	}

	return nil
}

// getConfigFilePath determines the configuration file path
func (cm *ConfigManager) getConfigFilePath() string {
	// Check environment variable first
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		return configPath
	}

	// Check common locations
	possiblePaths := []string{
		"./config/app.json",
		"./config/app.yaml",
		"./app.json",
		"./app.yaml",
		"/etc/hotel-reviews/app.json",
		"/etc/hotel-reviews/app.yaml",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to JSON in config directory
	return "./config/app.json"
}

// handleAppConfigChange handles changes to the main application configuration
func (cm *ConfigManager) handleAppConfigChange(configName string, oldConfig, newConfig interface{}) error {
	cm.logger.Info("Application configuration change detected", "config", configName)

	// Convert to AppConfig
	var appConfig AppConfig
	if err := cm.mapToStruct(newConfig, &appConfig); err != nil {
		return fmt.Errorf("failed to convert config to AppConfig: %w", err)
	}

	// Validate the new configuration
	if err := appConfig.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	oldAppConfig := cm.GetConfig()

	// Update configuration atomically
	cm.setConfig(&appConfig)

	// Reconfigure components
	if err := cm.reconfigureComponents(oldAppConfig, &appConfig); err != nil {
		// Rollback on failure
		cm.setConfig(oldAppConfig)
		return fmt.Errorf("failed to reconfigure components, rolled back: %w", err)
	}

	cm.reloadCount++
	cm.lastReloadTime = time.Now()

	cm.logger.Info("Application configuration successfully updated",
		"reload_count", cm.reloadCount,
		"changes", cm.getConfigChanges(oldAppConfig, &appConfig))

	return nil
}

// Environment variable change handlers
func (cm *ConfigManager) handleLogLevelChange(configName string, oldConfig, newConfig interface{}) error {
	newLevel := newConfig.(string)
	cm.logger.Info("Log level changed", "old", oldConfig, "new", newLevel)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.App.LogLevel = newLevel
		// Note: Monitoring config doesn't have LogLevel field, handle separately if needed
		
		if err := cm.reconfigureLogging(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure logging: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleDatabaseHostChange(configName string, oldConfig, newConfig interface{}) error {
	newHost := newConfig.(string)
	cm.logger.Info("Database host changed", "old", oldConfig, "new", newHost)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.Database.Host = newHost
		
		if err := cm.reconfigureDatabase(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure database: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleDatabasePortChange(configName string, oldConfig, newConfig interface{}) error {
	newPort := int(newConfig.(float64)) // JSON numbers are float64
	cm.logger.Info("Database port changed", "old", oldConfig, "new", newPort)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.Database.Port = newPort
		
		if err := cm.reconfigureDatabase(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure database: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleRedisHostChange(configName string, oldConfig, newConfig interface{}) error {
	newHost := newConfig.(string)
	cm.logger.Info("Redis host changed", "old", oldConfig, "new", newHost)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.Redis.Host = newHost
		
		if err := cm.reconfigureRedis(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure Redis: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleRedisPortChange(configName string, oldConfig, newConfig interface{}) error {
	newPort := int(newConfig.(float64))
	cm.logger.Info("Redis port changed", "old", oldConfig, "new", newPort)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.Redis.Port = newPort
		
		if err := cm.reconfigureRedis(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure Redis: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleServerPortChange(configName string, oldConfig, newConfig interface{}) error {
	newPort := int(newConfig.(float64))
	cm.logger.Info("Server port changed", "old", oldConfig, "new", newPort)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.Server.Port = newPort
		
		if err := cm.reconfigureServer(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure server: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleJWTSecretChange(configName string, oldConfig, newConfig interface{}) error {
	newSecret := newConfig.(string)
	
	if len(newSecret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters long")
	}

	cm.logger.Info("JWT secret changed", "length", len(newSecret))

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.Auth.JWTSecret = newSecret
		
		if err := cm.reconfigureAuth(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure auth: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleEnvironmentChange(configName string, oldConfig, newConfig interface{}) error {
	newEnv := newConfig.(string)
	cm.logger.Info("Environment changed", "old", oldConfig, "new", newEnv)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.App.Environment = newEnv
		
		// Environment changes may require full reconfiguration
		if err := cm.reconfigureComponents(config, &configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure for new environment: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

func (cm *ConfigManager) handleMaxConnectionsChange(configName string, oldConfig, newConfig interface{}) error {
	newMaxConns := int(newConfig.(float64))
	cm.logger.Info("Max connections changed", "old", oldConfig, "new", newMaxConns)

	config := cm.GetConfig()
	if config != nil {
		configCopy := *config
		configCopy.Database.MaxConns = newMaxConns
		
		if err := cm.reconfigureDatabase(&configCopy); err != nil {
			return fmt.Errorf("failed to reconfigure database connections: %w", err)
		}
		
		cm.setConfig(&configCopy)
	}

	return nil
}

// validateAppConfig provides comprehensive validation
func (cm *ConfigManager) validateAppConfig(config interface{}) error {
	// Handle nil config
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// If it's already an AppConfig struct, validate directly
	if appConfig, ok := config.(*AppConfig); ok {
		return appConfig.Validate()
	}
	if appConfig, ok := config.(AppConfig); ok {
		return appConfig.Validate()
	}

	// Otherwise, try to convert from map to struct
	var appConfig AppConfig
	if err := cm.mapToStruct(config, &appConfig); err != nil {
		return fmt.Errorf("invalid config format: %w", err)
	}

	return appConfig.Validate()
}

// Component reconfiguration methods
func (cm *ConfigManager) reconfigureComponents(oldConfig, newConfig *AppConfig) error {
	var errors []error

	// If oldConfig is nil, treat all components as changed (initial configuration)
	// Reconfigure in dependency order
	if oldConfig == nil || !reflect.DeepEqual(oldConfig.Monitoring, newConfig.Monitoring) {
		if err := cm.reconfigureMonitoring(newConfig); err != nil {
			errors = append(errors, fmt.Errorf("monitoring: %w", err))
		}
	}

	if oldConfig == nil || !reflect.DeepEqual(oldConfig.Database, newConfig.Database) {
		if err := cm.reconfigureDatabase(newConfig); err != nil {
			errors = append(errors, fmt.Errorf("database: %w", err))
		}
	}

	if oldConfig == nil || !reflect.DeepEqual(oldConfig.Redis, newConfig.Redis) {
		if err := cm.reconfigureRedis(newConfig); err != nil {
			errors = append(errors, fmt.Errorf("redis: %w", err))
		}
	}

	if oldConfig == nil || !reflect.DeepEqual(oldConfig.Auth, newConfig.Auth) {
		if err := cm.reconfigureAuth(newConfig); err != nil {
			errors = append(errors, fmt.Errorf("auth: %w", err))
		}
	}

	if oldConfig == nil || !reflect.DeepEqual(oldConfig.Server, newConfig.Server) {
		if err := cm.reconfigureServer(newConfig); err != nil {
			errors = append(errors, fmt.Errorf("server: %w", err))
		}
	}

	if oldConfig == nil || !reflect.DeepEqual(oldConfig.CacheService, newConfig.CacheService) {
		if err := cm.reconfigureCacheService(newConfig); err != nil {
			errors = append(errors, fmt.Errorf("cache service: %w", err))
		}
	}

	if oldConfig == nil || !reflect.DeepEqual(oldConfig.EventHandler, newConfig.EventHandler) {
		if err := cm.reconfigureEventHandler(newConfig); err != nil {
			errors = append(errors, fmt.Errorf("event handler: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("reconfiguration errors: %v", errors)
	}

	return nil
}

func (cm *ConfigManager) reconfigureLogging(config *AppConfig) error {
	cm.logger.Info("Reconfiguring logging", "level", config.App.LogLevel)
	// Implementation would reconfigure the logging system
	// This is a placeholder for actual logging reconfiguration
	return nil
}

func (cm *ConfigManager) reconfigureMonitoring(config *AppConfig) error {
	cm.logger.Info("Reconfiguring monitoring system")
	if cm.monitoringSystem != nil {
		// Implementation would reconfigure the monitoring system
		// This is a placeholder for actual monitoring reconfiguration
	}
	return nil
}

func (cm *ConfigManager) reconfigureDatabase(config *AppConfig) error {
	cm.logger.Info("Reconfiguring database connection",
		"host", config.Database.Host,
		"port", config.Database.Port,
		"max_conns", config.Database.MaxConns)
	
	// Implementation would reconfigure the database pool
	// This is a placeholder for actual database reconfiguration
	return nil
}

func (cm *ConfigManager) reconfigureRedis(config *AppConfig) error {
	cm.logger.Info("Reconfiguring Redis connection",
		"host", config.Redis.Host,
		"port", config.Redis.Port,
		"pool_size", config.Redis.PoolSize)
	
	// Implementation would reconfigure the Redis client
	// This is a placeholder for actual Redis reconfiguration
	return nil
}

func (cm *ConfigManager) reconfigureAuth(config *AppConfig) error {
	cm.logger.Info("Reconfiguring authentication middleware")
	if cm.authMiddleware != nil {
		// Implementation would reconfigure the auth middleware
		// This is a placeholder for actual auth reconfiguration
	}
	return nil
}

func (cm *ConfigManager) reconfigureServer(config *AppConfig) error {
	cm.logger.Info("Reconfiguring HTTP server",
		"host", config.Server.Host,
		"port", config.Server.Port,
		"tls_enabled", config.Server.EnableTLS)
	
	// For server reconfiguration, we might need to restart the server
	// This is a placeholder for actual server reconfiguration
	return nil
}

func (cm *ConfigManager) reconfigureCacheService(config *AppConfig) error {
	cm.logger.Info("Reconfiguring cache service")
	if cm.cacheService != nil {
		// Implementation would reconfigure the cache service
		// This is a placeholder for actual cache service reconfiguration
	}
	return nil
}

func (cm *ConfigManager) reconfigureEventHandler(config *AppConfig) error {
	cm.logger.Info("Reconfiguring event handler",
		"max_workers", config.EventHandler.MaxWorkers,
		"buffer_size", config.EventHandler.BufferSize)
	
	if cm.eventHandler != nil {
		// Implementation would reconfigure the event handler
		// This is a placeholder for actual event handler reconfiguration
	}
	return nil
}

// Utility methods
func (cm *ConfigManager) mapToStruct(input interface{}, output interface{}) error {
	// Handle nil input
	if input == nil {
		return fmt.Errorf("input is nil")
	}

	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   output,
		TagName:  "json",
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		WeaklyTypedInput: true, // Allow type coercion
		ErrorUnused:      false, // Don't error on unused fields
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(input); err != nil {
		return fmt.Errorf("failed to decode input to struct: %w", err)
	}

	return nil
}

func (cm *ConfigManager) getConfigChanges(oldConfig, newConfig *AppConfig) map[string]interface{} {
	changes := make(map[string]interface{})

	// If oldConfig is nil, don't show changes (initial configuration)
	if oldConfig == nil {
		return changes
	}

	if oldConfig.Server.Port != newConfig.Server.Port {
		changes["server_port"] = fmt.Sprintf("%d -> %d", oldConfig.Server.Port, newConfig.Server.Port)
	}
	if oldConfig.Database.Host != newConfig.Database.Host {
		changes["db_host"] = fmt.Sprintf("%s -> %s", oldConfig.Database.Host, newConfig.Database.Host)
	}
	if oldConfig.App.LogLevel != newConfig.App.LogLevel {
		changes["log_level"] = fmt.Sprintf("%s -> %s", oldConfig.App.LogLevel, newConfig.App.LogLevel)
	}

	return changes
}

// Public API methods
func (cm *ConfigManager) GetConfig() *AppConfig {
	cm.configMutex.RLock()
	defer cm.configMutex.RUnlock()
	
	if cm.config == nil {
		return nil
	}
	
	// Return a copy to prevent external modifications
	configCopy := *cm.config
	return &configCopy
}

func (cm *ConfigManager) setConfig(config *AppConfig) {
	cm.configMutex.Lock()
	defer cm.configMutex.Unlock()
	cm.config = config
}

func (cm *ConfigManager) GetConfigHistory() []infrastructure.ConfigSnapshot {
	return cm.watcher.GetConfigHistory("app_config")
}

func (cm *ConfigManager) RollbackConfig(targetHash string) error {
	cm.logger.Info("Rolling back configuration", "target_hash", targetHash)
	
	if err := cm.watcher.RollbackConfig("app_config", targetHash); err != nil {
		cm.errorCount++
		return fmt.Errorf("rollback failed: %w", err)
	}
	
	cm.logger.Info("Configuration rollback completed", "target_hash", targetHash)
	return nil
}

func (cm *ConfigManager) GetMetrics() map[string]interface{} {
	watcherMetrics := cm.watcher.GetMetrics()
	
	// Add our own metrics
	watcherMetrics["config_reload_count"] = cm.reloadCount
	watcherMetrics["config_error_count"] = cm.errorCount
	watcherMetrics["last_config_reload"] = cm.lastReloadTime
	
	return watcherMetrics
}

func (cm *ConfigManager) HealthCheck() error {
	if err := cm.watcher.HealthCheck(); err != nil {
		return fmt.Errorf("config watcher health check failed: %w", err)
	}
	
	if cm.config == nil {
		return fmt.Errorf("no configuration loaded")
	}
	
	return nil
}

func (cm *ConfigManager) Stop() error {
	cm.logger.Info("Stopping configuration manager")
	
	cm.cancel()
	
	if cm.watcher != nil {
		return cm.watcher.Stop()
	}
	
	return nil
}

// RegisterComponent allows components to register for configuration updates
func (cm *ConfigManager) RegisterComponent(componentName string, callback ComponentUpdateCallback) {
	cm.componentCallbacks[componentName] = append(cm.componentCallbacks[componentName], callback)
	cm.logger.Debug("Registered component for config updates", "component", componentName)
}

// SetComponentReferences allows setting references to application components
func (cm *ConfigManager) SetComponentReferences(components map[string]interface{}) {
	if server, ok := components["server"].(*http.Server); ok {
		cm.server = server
	}
	if dbPool, ok := components["database"]; ok {
		cm.dbPool = dbPool
	}
	if redisClient, ok := components["redis"]; ok {
		cm.redisClient = redisClient
	}
	if cacheService, ok := components["cache_service"].(*application.CacheService); ok {
		cm.cacheService = cacheService
	}
	if authMiddleware, ok := components["auth_middleware"].(*application.AuthMiddleware); ok {
		cm.authMiddleware = authMiddleware
	}
	if eventHandler, ok := components["event_handler"].(*application.BaseEventHandler); ok {
		cm.eventHandler = eventHandler
	}
	if errorHandler, ok := components["error_handler"].(*infrastructure.ErrorHandler); ok {
		cm.errorHandler = errorHandler
	}
	if monitoringSystem, ok := components["monitoring"].(*monitoring.Service); ok {
		cm.monitoringSystem = monitoringSystem
	}
	if circuitBreaker, ok := components["circuit_breaker"].(*infrastructure.CircuitBreaker); ok {
		cm.circuitBreaker = circuitBreaker
	}
}