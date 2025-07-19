package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// ConfigWatcher provides hot-reloading capabilities for configuration files and environment variables
type ConfigWatcher struct {
	mu        sync.RWMutex
	watcher   *fsnotify.Watcher
	validator *validator.Validate
	logger    monitoring.Logger
	ctx       context.Context
	cancel    context.CancelFunc

	// Configuration storage
	configs     map[string]interface{}
	configFiles map[string]string // config name -> file path
	envVars     map[string]string // env var name -> current value

	// Callbacks and notifications
	changeCallbacks map[string][]ConfigChangeCallback
	validators      map[string]ConfigValidator

	// History and rollback
	configHistory  map[string][]ConfigSnapshot
	maxHistorySize int

	// Metrics
	reloadCount    int64
	errorCount     int64
	lastReloadTime time.Time

	// Options
	options *ConfigWatcherOptions
}

// ConfigChangeCallback is called when configuration changes
type ConfigChangeCallback func(configName string, oldConfig, newConfig interface{}) error

// ConfigValidator validates configuration before applying changes
type ConfigValidator func(config interface{}) error

// ConfigSnapshot stores a configuration state for rollback
type ConfigSnapshot struct {
	Config    interface{} `json:"config"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"` // "file" or "env"
	Hash      string      `json:"hash"`
}

// ConfigWatcherOptions configures the behavior of ConfigWatcher
type ConfigWatcherOptions struct {
	// File watching
	WatchIntervalSec  int  `json:"watch_interval_sec" validate:"min=1"`
	FileChecksum      bool `json:"file_checksum"`
	IgnoreHiddenFiles bool `json:"ignore_hidden_files"`

	// Environment variable monitoring
	EnvCheckIntervalSec int    `json:"env_check_interval_sec" validate:"min=1"`
	EnvVarPrefix        string `json:"env_var_prefix"`

	// Validation and rollback
	ValidateOnLoad     bool `json:"validate_on_load"`
	EnableRollback     bool `json:"enable_rollback"`
	MaxHistorySize     int  `json:"max_history_size" validate:"min=1"`
	RollbackTimeoutSec int  `json:"rollback_timeout_sec" validate:"min=1"`

	// Error handling
	MaxRetries       int  `json:"max_retries" validate:"min=0"`
	RetryDelaySec    int  `json:"retry_delay_sec" validate:"min=1"`
	FailOnValidation bool `json:"fail_on_validation"`

	// Performance
	BatchUpdates    bool `json:"batch_updates"`
	BatchIntervalMs int  `json:"batch_interval_ms" validate:"min=100"`

	// Debugging
	EnableDebugLogging bool `json:"enable_debug_logging"`
	LogConfigChanges   bool `json:"log_config_changes"`
}

// DefaultConfigWatcherOptions returns sensible defaults
func DefaultConfigWatcherOptions() *ConfigWatcherOptions {
	return &ConfigWatcherOptions{
		WatchIntervalSec:    1,
		FileChecksum:        true,
		IgnoreHiddenFiles:   true,
		EnvCheckIntervalSec: 30,
		EnvVarPrefix:        "APP_",
		ValidateOnLoad:      true,
		EnableRollback:      true,
		MaxHistorySize:      10,
		RollbackTimeoutSec:  30,
		MaxRetries:          3,
		RetryDelaySec:       5,
		FailOnValidation:    false,
		BatchUpdates:        true,
		BatchIntervalMs:     500,
		EnableDebugLogging:  false,
		LogConfigChanges:    true,
	}
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(logger monitoring.Logger, options *ConfigWatcherOptions) (*ConfigWatcher, error) {
	if options == nil {
		options = DefaultConfigWatcherOptions()
	}

	// Validate options
	validate := validator.New()
	if err := validate.Struct(options); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	cw := &ConfigWatcher{
		watcher:         watcher,
		validator:       validate,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		configs:         make(map[string]interface{}),
		configFiles:     make(map[string]string),
		envVars:         make(map[string]string),
		changeCallbacks: make(map[string][]ConfigChangeCallback),
		validators:      make(map[string]ConfigValidator),
		configHistory:   make(map[string][]ConfigSnapshot),
		maxHistorySize:  options.MaxHistorySize,
		options:         options,
	}

	// Start monitoring goroutines
	go cw.watchFiles()
	go cw.watchEnvironment()

	if options.BatchUpdates {
		go cw.batchProcessor()
	}

	logger.Info("Configuration watcher started",
		"file_checksum", options.FileChecksum,
		"env_check_interval", options.EnvCheckIntervalSec,
		"rollback_enabled", options.EnableRollback)

	return cw, nil
}

// RegisterConfig registers a configuration for monitoring
func (cw *ConfigWatcher) RegisterConfig(name string, config interface{}) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.options.ValidateOnLoad {
		if err := cw.validateConfig(name, config); err != nil {
			return fmt.Errorf("config validation failed for %s: %w", name, err)
		}
	}

	cw.configs[name] = config
	cw.addToHistory(name, config, "registration")

	cw.logger.Debug("Registered configuration", "name", name, "type", reflect.TypeOf(config))
	return nil
}

// WatchFile starts watching a configuration file
func (cw *ConfigWatcher) WatchFile(configName, filePath string) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %w", filePath, err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file does not exist: %s", absPath)
	}

	// Add to watcher
	if err := cw.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to watch file %s: %w", absPath, err)
	}

	// Also watch the directory for file recreations
	dir := filepath.Dir(absPath)
	if err := cw.watcher.Add(dir); err != nil {
		cw.logger.Warn("Failed to watch directory", "dir", dir, "error", err)
	}

	// Update the configFiles map (needs lock)
	cw.mu.Lock()
	cw.configFiles[configName] = absPath
	cw.mu.Unlock()

	// Load initial configuration (this will acquire its own lock in updateConfig)
	if err := cw.loadFileConfig(configName, absPath); err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}

	cw.logger.Info("Started watching configuration file",
		"config", configName,
		"file", absPath)

	return nil
}

// WatchEnvVar starts monitoring an environment variable
func (cw *ConfigWatcher) WatchEnvVar(configName, envVarName string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	currentValue := os.Getenv(envVarName)
	cw.envVars[envVarName] = currentValue

	// Store mapping for updates
	key := fmt.Sprintf("env:%s", envVarName)
	if currentValue != "" {
		cw.configs[key] = currentValue
		cw.addToHistory(key, currentValue, "env")
	}

	cw.logger.Info("Started watching environment variable",
		"config", configName,
		"env_var", envVarName,
		"has_value", currentValue != "")

	return nil
}

// RegisterChangeCallback registers a callback for configuration changes
func (cw *ConfigWatcher) RegisterChangeCallback(configName string, callback ConfigChangeCallback) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.changeCallbacks[configName] = append(cw.changeCallbacks[configName], callback)
	cw.logger.Debug("Registered change callback", "config", configName)
}

// RegisterValidator registers a validator for a configuration
func (cw *ConfigWatcher) RegisterValidator(configName string, validator ConfigValidator) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.validators[configName] = validator
	cw.logger.Debug("Registered validator", "config", configName)
}

// GetConfig retrieves the current configuration
func (cw *ConfigWatcher) GetConfig(name string) (interface{}, bool) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	config, exists := cw.configs[name]
	return config, exists
}

// GetConfigHistory returns the configuration history
func (cw *ConfigWatcher) GetConfigHistory(name string) []ConfigSnapshot {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	history := cw.configHistory[name]
	result := make([]ConfigSnapshot, len(history))
	copy(result, history)
	return result
}

// RollbackConfig rolls back to a previous configuration
func (cw *ConfigWatcher) RollbackConfig(name string, targetHash string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.options.EnableRollback {
		return fmt.Errorf("rollback is disabled")
	}

	history := cw.configHistory[name]
	if len(history) == 0 {
		return fmt.Errorf("no history available for config %s", name)
	}

	var targetSnapshot *ConfigSnapshot
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Hash == targetHash {
			targetSnapshot = &history[i]
			break
		}
	}

	if targetSnapshot == nil {
		return fmt.Errorf("configuration snapshot with hash %s not found", targetHash)
	}

	// Validate the target configuration
	if err := cw.validateConfig(name, targetSnapshot.Config); err != nil {
		return fmt.Errorf("target configuration is invalid: %w", err)
	}

	cw.configs[name] = targetSnapshot.Config

	// Add rollback to history immediately
	cw.addToHistory(name, targetSnapshot.Config, fmt.Sprintf("rollback:%s", targetHash))

	cw.logger.Info("Configuration rolled back",
		"config", name,
		"target_hash", targetHash,
		"timestamp", targetSnapshot.Timestamp)

	// Note: We skip notifying callbacks during rollback to avoid deadlocks
	// and race conditions. Callbacks should be designed to handle configuration
	// changes gracefully without requiring notification during rollback.

	return nil
}

// watchFiles monitors file system events
func (cw *ConfigWatcher) watchFiles() {
	defer cw.watcher.Close()

	for {
		select {
		case <-cw.ctx.Done():
			return

		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			if cw.options.EnableDebugLogging {
				cw.logger.Debug("File event received",
					"file", event.Name,
					"op", event.Op.String())
			}

			// Skip hidden files if configured
			if cw.options.IgnoreHiddenFiles && filepath.Base(event.Name)[0] == '.' {
				continue
			}

			cw.handleFileEvent(event)

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}

			cw.errorCount++
			cw.logger.Error("File watcher error", "error", err)
		}
	}
}

// handleFileEvent processes a file system event
func (cw *ConfigWatcher) handleFileEvent(event fsnotify.Event) {
	cw.mu.RLock()
	configName := ""
	for name, path := range cw.configFiles {
		if path == event.Name {
			configName = name
			break
		}
	}
	cw.mu.RUnlock()

	if configName == "" {
		return // Not a watched config file
	}

	// Handle different event types
	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		cw.reloadFileConfig(configName, event.Name)

	case event.Op&fsnotify.Create == fsnotify.Create:
		cw.reloadFileConfig(configName, event.Name)

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		cw.logger.Warn("Configuration file removed",
			"config", configName,
			"file", event.Name)

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		cw.logger.Warn("Configuration file renamed",
			"config", configName,
			"file", event.Name)
	}
}

// reloadFileConfig reloads configuration from file
func (cw *ConfigWatcher) reloadFileConfig(configName, filePath string) {
	if cw.options.EnableDebugLogging {
		cw.logger.Debug("Reloading file configuration",
			"config", configName,
			"file", filePath)
	}

	var retries int
	for retries < cw.options.MaxRetries {
		if err := cw.loadFileConfig(configName, filePath); err != nil {
			retries++
			cw.errorCount++
			cw.logger.Error("Failed to reload configuration",
				"config", configName,
				"file", filePath,
				"error", err,
				"retry", retries)

			if retries < cw.options.MaxRetries {
				time.Sleep(time.Duration(cw.options.RetryDelaySec) * time.Second)
				continue
			}

			cw.logger.Error("Max retries exceeded for configuration reload",
				"config", configName,
				"file", filePath)
			return
		}
		break
	}

	cw.reloadCount++
	cw.lastReloadTime = time.Now()
}

// loadFileConfig loads configuration from a file
func (cw *ConfigWatcher) loadFileConfig(configName, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Determine file format and parse
	var newConfig interface{}
	ext := filepath.Ext(filePath)

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &newConfig); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}

	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &newConfig); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}

	default:
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	return cw.updateConfig(configName, newConfig, "file")
}

// watchEnvironment monitors environment variable changes
func (cw *ConfigWatcher) watchEnvironment() {
	ticker := time.NewTicker(time.Duration(cw.options.EnvCheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cw.ctx.Done():
			return

		case <-ticker.C:
			cw.checkEnvironmentChanges()
		}
	}
}

// checkEnvironmentChanges checks for environment variable changes
func (cw *ConfigWatcher) checkEnvironmentChanges() {
	cw.mu.RLock()
	envVarsCopy := make(map[string]string)
	for k, v := range cw.envVars {
		envVarsCopy[k] = v
	}
	cw.mu.RUnlock()

	for envVar, oldValue := range envVarsCopy {
		currentValue := os.Getenv(envVar)

		if currentValue != oldValue {
			if cw.options.EnableDebugLogging {
				cw.logger.Debug("Environment variable changed",
					"var", envVar,
					"old", oldValue,
					"new", currentValue)
			}

			configName := fmt.Sprintf("env:%s", envVar)
			// Update the env var tracking first to avoid deadlock
			cw.mu.Lock()
			cw.envVars[envVar] = currentValue
			cw.mu.Unlock()

			// Then update the config (this will acquire its own lock)
			if err := cw.updateConfig(configName, currentValue, "env"); err != nil {
				cw.logger.Error("Failed to update environment variable config",
					"var", envVar,
					"error", err)
				// Revert the env var change on error
				cw.mu.Lock()
				cw.envVars[envVar] = oldValue
				cw.mu.Unlock()
			}
		}
	}
}

// updateConfig updates a configuration and notifies callbacks
func (cw *ConfigWatcher) updateConfig(configName string, newConfig interface{}, source string) error {
	cw.mu.Lock()

	oldConfig := cw.configs[configName]

	// Skip if configuration hasn't changed
	if reflect.DeepEqual(oldConfig, newConfig) {
		cw.mu.Unlock()
		return nil
	}

	// Validate new configuration
	if err := cw.validateConfig(configName, newConfig); err != nil {
		cw.mu.Unlock()
		if cw.options.FailOnValidation {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
		cw.logger.Warn("Configuration validation failed, proceeding anyway",
			"config", configName,
			"error", err)
		// Continue with the update even if validation failed (but not failing on validation)
		cw.mu.Lock()
	}

	// Update configuration
	cw.configs[configName] = newConfig
	cw.addToHistory(configName, newConfig, source)

	// Notify callbacks (without holding the lock)
	cw.mu.Unlock()
	callbackErr := cw.notifyCallbacks(configName, oldConfig, newConfig)
	cw.mu.Lock()

	if callbackErr != nil {
		// Rollback on callback failure
		cw.configs[configName] = oldConfig
		cw.mu.Unlock()
		return fmt.Errorf("callback failed, configuration rolled back: %w", callbackErr)
	}

	if cw.options.LogConfigChanges {
		cw.logger.Info("Configuration updated",
			"config", configName,
			"source", source)
	}

	cw.mu.Unlock()

	return nil
}

// validateConfig validates a configuration
func (cw *ConfigWatcher) validateConfig(configName string, config interface{}) error {
	// Handle nil config
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// Only run struct validation if the config is actually a struct
	// Skip struct validation for maps (they will be validated by custom validators)
	configType := reflect.TypeOf(config)
	if configType != nil && configType.Kind() == reflect.Struct {
		if err := cw.validator.Struct(config); err != nil {
			return err
		}
	}

	// Custom validator (this is the main validation for map-based configs)
	if validator, exists := cw.validators[configName]; exists {
		if err := validator(config); err != nil {
			return err
		}
	}

	return nil
}

// notifyCallbacks notifies all registered callbacks
func (cw *ConfigWatcher) notifyCallbacks(configName string, oldConfig, newConfig interface{}) error {
	cw.mu.RLock()
	callbacks := cw.changeCallbacks[configName]
	cw.mu.RUnlock()

	for _, callback := range callbacks {
		if err := callback(configName, oldConfig, newConfig); err != nil {
			return err
		}
	}

	return nil
}

// addToHistory adds a configuration to history
func (cw *ConfigWatcher) addToHistory(configName string, config interface{}, source string) {
	snapshot := ConfigSnapshot{
		Config:    config,
		Timestamp: time.Now(),
		Source:    source,
		Hash:      cw.generateConfigHash(config),
	}

	history := cw.configHistory[configName]
	history = append(history, snapshot)

	// Limit history size
	if len(history) > cw.maxHistorySize {
		history = history[len(history)-cw.maxHistorySize:]
	}

	cw.configHistory[configName] = history
}

// generateConfigHash generates a hash for configuration deduplication
func (cw *ConfigWatcher) generateConfigHash(config interface{}) string {
	data, _ := json.Marshal(config)
	hexStr := fmt.Sprintf("%x", data)
	if len(hexStr) < 16 {
		return hexStr // Return the full string if it's shorter than 16 chars
	}
	return hexStr[:16] // Use first 16 chars as hash
}

// batchProcessor handles batched configuration updates
func (cw *ConfigWatcher) batchProcessor() {
	ticker := time.NewTicker(time.Duration(cw.options.BatchIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	pendingUpdates := make(map[string]interface{})

	for {
		select {
		case <-cw.ctx.Done():
			return

		case <-ticker.C:
			if len(pendingUpdates) > 0 {
				cw.processBatchUpdates(pendingUpdates)
				pendingUpdates = make(map[string]interface{})
			}
		}
	}
}

// processBatchUpdates processes batched configuration updates
func (cw *ConfigWatcher) processBatchUpdates(updates map[string]interface{}) {
	for configName, config := range updates {
		if err := cw.updateConfig(configName, config, "batch"); err != nil {
			cw.logger.Error("Failed to process batched update",
				"config", configName,
				"error", err)
		}
	}
}

// GetMetrics returns watcher metrics
func (cw *ConfigWatcher) GetMetrics() map[string]interface{} {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	return map[string]interface{}{
		"reload_count":       cw.reloadCount,
		"error_count":        cw.errorCount,
		"last_reload_time":   cw.lastReloadTime,
		"watched_files":      len(cw.configFiles),
		"watched_env_vars":   len(cw.envVars),
		"registered_configs": len(cw.configs),
		"total_callbacks":    cw.getTotalCallbacks(),
		"history_entries":    cw.getTotalHistoryEntries(),
	}
}

// getTotalCallbacks returns total number of registered callbacks
func (cw *ConfigWatcher) getTotalCallbacks() int {
	total := 0
	for _, callbacks := range cw.changeCallbacks {
		total += len(callbacks)
	}
	return total
}

// getTotalHistoryEntries returns total number of history entries
func (cw *ConfigWatcher) getTotalHistoryEntries() int {
	total := 0
	for _, history := range cw.configHistory {
		total += len(history)
	}
	return total
}

// Stop gracefully stops the configuration watcher
func (cw *ConfigWatcher) Stop() error {
	cw.logger.Info("Stopping configuration watcher")

	cw.cancel()

	if cw.watcher != nil {
		return cw.watcher.Close()
	}

	return nil
}

// Health check for configuration watcher
func (cw *ConfigWatcher) HealthCheck() error {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	// Check if watcher is still running
	select {
	case <-cw.ctx.Done():
		return fmt.Errorf("configuration watcher is stopped")
	default:
	}

	// Check for excessive errors
	if cw.errorCount > 100 {
		return fmt.Errorf("configuration watcher has too many errors: %d", cw.errorCount)
	}

	return nil
}
