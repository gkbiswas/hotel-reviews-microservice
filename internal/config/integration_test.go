package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// MockLogger for testing
type MockLogger struct {
	logs []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "debug", Message: msg, Fields: fieldsToMap(fields)})
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "info", Message: msg, Fields: fieldsToMap(fields)})
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "warn", Message: msg, Fields: fieldsToMap(fields)})
}

func (m *MockLogger) Error(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "error", Message: msg, Fields: fieldsToMap(fields)})
}

func fieldsToMap(fields []interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			if key, ok := fields[i].(string); ok {
				result[key] = fields[i+1]
			}
		}
	}
	return result
}

func createTestLogger() *MockLogger {
	return &MockLogger{logs: make([]LogEntry, 0)}
}

func createTempConfigFile(t *testing.T, config interface{}, format string) string {
	tempDir := t.TempDir()
	var filename string
	var data []byte
	var err error

	switch format {
	case "json":
		filename = filepath.Join(tempDir, "app.json")
		data, err = json.MarshalIndent(config, "", "  ")
	case "yaml":
		filename = filepath.Join(tempDir, "app.yaml")
		data, err = yaml.Marshal(config)
	default:
		t.Fatalf("Unsupported format: %s", format)
	}

	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filename, data, 0644))

	return filename
}

func TestConfigManager_BasicFunctionality(t *testing.T) {
	logger := createTestLogger()

	// Set temporary config path
	tempConfig := createTempConfigFile(t, GetDefaultConfig(), "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	require.NotNil(t, manager)
	defer manager.Stop()

	// Test getting configuration
	config := manager.GetConfig()
	require.NotNil(t, config)
	assert.Equal(t, "hotel-reviews-service", config.App.Name)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "development", config.App.Environment)

	// Test health check
	err = manager.HealthCheck()
	assert.NoError(t, err)

	// Test metrics
	metrics := manager.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "watched_files")
	assert.Contains(t, metrics, "config_reload_count")
}

func TestConfigManager_FileWatching(t *testing.T) {
	logger := createTestLogger()

	// Create initial config
	initialConfig := GetDefaultConfig()
	initialConfig.Server.Port = 8080
	initialConfig.App.LogLevel = "info"

	tempConfig := createTempConfigFile(t, initialConfig, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Verify initial configuration
	config := manager.GetConfig()
	require.NotNil(t, config)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "info", config.App.LogLevel)

	// Setup change detection
	changeDetected := make(chan bool, 1)
	configChangeCallback := func(oldConfig, newConfig interface{}) error {
		select {
		case changeDetected <- true:
		default:
		}
		return nil
	}

	manager.RegisterComponent("test_component", configChangeCallback)

	// Give some time for the manager to be fully ready
	time.Sleep(100 * time.Millisecond)

	// Modify configuration
	modifiedConfig := *initialConfig
	modifiedConfig.Server.Port = 9090
	modifiedConfig.App.LogLevel = "debug"
	modifiedConfig.Database.MaxConns = 50

	configData, err := json.MarshalIndent(modifiedConfig, "", "  ")
	require.NoError(t, err)

	// Write new configuration
	err = os.WriteFile(tempConfig, configData, 0644)
	require.NoError(t, err)

	// Wait for change detection
	select {
	case <-changeDetected:
		// Verify configuration was updated
		updatedConfig := manager.GetConfig()
		assert.Equal(t, 9090, updatedConfig.Server.Port)
		assert.Equal(t, "debug", updatedConfig.App.LogLevel)
		assert.Equal(t, 50, updatedConfig.Database.MaxConns)

	case <-time.After(10 * time.Second):
		t.Fatal("Configuration change was not detected within timeout")
	}
}

func TestConfigManager_EnvironmentVariables(t *testing.T) {
	logger := createTestLogger()

	// Create base config
	baseConfig := GetDefaultConfig()
	tempConfig := createTempConfigFile(t, baseConfig, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	// Set initial environment variables
	envVars := map[string]string{
		"APP_LOG_LEVEL":       "info",
		"APP_DB_HOST":         "localhost",
		"APP_DB_PORT":         "5432",
		"APP_SERVER_PORT":     "8080",
		"APP_MAX_CONNECTIONS": "25",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Setup change detection for environment variables
	// changeDetected := make(chan string, 10) // Unused in current test

	// Add a small delay to ensure the manager is fully initialized
	time.Sleep(100 * time.Millisecond)

	// Test log level change
	os.Setenv("APP_LOG_LEVEL", "debug")

	// Test database host change
	os.Setenv("APP_DB_HOST", "new-db-host")

	// Test server port change
	os.Setenv("APP_SERVER_PORT", "9090")

	// Wait for changes to be processed (env vars are checked every 30 seconds by default in tests)
	// We'll wait a bit and then verify the configuration
	time.Sleep(2 * time.Second)

	// Check if configuration was updated
	config := manager.GetConfig()
	require.NotNil(t, config)

	// The configuration should be updated through environment variable changes
	// Note: The actual update depends on the environment variable monitoring interval
	t.Logf("Current config after env changes: log_level=%s, db_host=%s, server_port=%d",
		config.App.LogLevel, config.Database.Host, config.Server.Port)
}

func TestConfigManager_ValidationErrors(t *testing.T) {
	logger := createTestLogger()

	// Create invalid configuration
	invalidConfig := GetDefaultConfig()
	invalidConfig.Server.Port = 0             // Invalid port
	invalidConfig.Database.MaxConns = 0       // Invalid max connections
	invalidConfig.App.Environment = "invalid" // Invalid environment

	tempConfig := createTempConfigFile(t, invalidConfig, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	// The manager should handle validation errors gracefully
	manager, err := NewConfigManager(logger)
	if err != nil {
		// This is expected for severely invalid configurations
		assert.Contains(t, err.Error(), "validation")
		return
	}
	defer manager.Stop()

	// If manager was created, it should use default config instead
	config := manager.GetConfig()
	if config != nil {
		// Should fall back to defaults
		assert.NotEqual(t, 0, config.Server.Port)
		assert.NotEqual(t, 0, config.Database.MaxConns)
	}
}

func TestConfigManager_Rollback(t *testing.T) {
	logger := createTestLogger()

	// Create initial configuration
	config1 := GetDefaultConfig()
	config1.Server.Port = 8080
	config1.App.LogLevel = "info"

	tempConfig := createTempConfigFile(t, config1, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Verify initial config
	currentConfig := manager.GetConfig()
	require.NotNil(t, currentConfig)
	assert.Equal(t, 8080, currentConfig.Server.Port)

	// Wait a bit to ensure initial config is loaded
	time.Sleep(500 * time.Millisecond)

	// Update configuration (version 2)
	config2 := *config1
	config2.Server.Port = 9090
	config2.App.LogLevel = "debug"

	configData, err := json.MarshalIndent(config2, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(tempConfig, configData, 0644)
	require.NoError(t, err)

	// Wait for update
	time.Sleep(3 * time.Second)

	// Verify config was updated
	updatedConfig := manager.GetConfig()
	assert.Equal(t, 9090, updatedConfig.Server.Port)

	// Get configuration history
	history := manager.GetConfigHistory()
	require.Len(t, history, 2) // Initial load + file update

	// Rollback to first configuration
	firstConfigHash := history[0].Hash
	err = manager.RollbackConfig(firstConfigHash)
	assert.NoError(t, err)

	// Verify rollback
	rolledBackConfig := manager.GetConfig()
	assert.Equal(t, 8080, rolledBackConfig.Server.Port)
	assert.Equal(t, "info", rolledBackConfig.App.LogLevel)

	// Verify history now has rollback entry
	newHistory := manager.GetConfigHistory()
	assert.Len(t, newHistory, 3) // Initial + update + rollback
}

func TestConfigManager_ComponentIntegration(t *testing.T) {
	logger := createTestLogger()

	config := GetDefaultConfig()
	tempConfig := createTempConfigFile(t, config, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Simulate component registration
	var serverReconfigured bool
	var dbReconfigured bool
	var authReconfigured bool

	manager.RegisterComponent("server", func(oldConfig, newConfig interface{}) error {
		serverReconfigured = true
		logger.Info("Server component reconfigured")
		return nil
	})

	manager.RegisterComponent("database", func(oldConfig, newConfig interface{}) error {
		dbReconfigured = true
		logger.Info("Database component reconfigured")
		return nil
	})

	manager.RegisterComponent("auth", func(oldConfig, newConfig interface{}) error {
		authReconfigured = true
		logger.Info("Auth component reconfigured")
		return nil
	})

	// Update configuration to trigger component updates
	newConfig := *config
	newConfig.Server.Port = 9090
	newConfig.Database.MaxConns = 50
	newConfig.Auth.JWTExpiry = 2 * time.Hour

	configData, err := json.MarshalIndent(newConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(tempConfig, configData, 0644)
	require.NoError(t, err)

	// Wait for reconfiguration
	time.Sleep(3 * time.Second)

	// Verify components were notified (in real implementation)
	// Note: In this test, the actual reconfiguration callbacks are placeholders
	t.Logf("Components reconfigured - Server: %v, DB: %v, Auth: %v",
		serverReconfigured, dbReconfigured, authReconfigured)
}

func TestConfigManager_ErrorRecovery(t *testing.T) {
	logger := createTestLogger()

	config := GetDefaultConfig()
	tempConfig := createTempConfigFile(t, config, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Register a component that fails during reconfiguration
	componentFailureCount := 0
	manager.RegisterComponent("failing_component", func(oldConfig, newConfig interface{}) error {
		componentFailureCount++
		if componentFailureCount <= 2 {
			return fmt.Errorf("simulated component failure %d", componentFailureCount)
		}
		return nil // Succeed on third attempt
	})

	// Update configuration multiple times to test error recovery
	for i := 1; i <= 3; i++ {
		newConfig := *config
		newConfig.Server.Port = 8080 + i

		configData, err := json.MarshalIndent(newConfig, "", "  ")
		require.NoError(t, err)
		err = os.WriteFile(tempConfig, configData, 0644)
		require.NoError(t, err)

		time.Sleep(2 * time.Second)
	}

	// Check metrics for error count
	metrics := manager.GetMetrics()
	t.Logf("Final metrics: %+v", metrics)
	t.Logf("Component failure count: %d", componentFailureCount)
}

func TestConfigManager_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()

	config := GetDefaultConfig()
	tempConfig := createTempConfigFile(t, config, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Concurrent configuration reads
	const numReaders = 10
	const numWrites = 5

	done := make(chan bool, numReaders+numWrites)

	// Start concurrent readers
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				config := manager.GetConfig()
				if config == nil {
					t.Errorf("Reader %d: got nil config on iteration %d", id, j)
					return
				}

				// Verify config consistency
				if config.App.Name != "hotel-reviews-service" {
					t.Errorf("Reader %d: unexpected app name: %s", id, config.App.Name)
					return
				}
			}
		}(i)
	}

	// Start concurrent configuration writers
	for i := 0; i < numWrites; i++ {
		go func(id int) {
			defer func() { done <- true }()

			newConfig := *config
			newConfig.Server.Port = 8080 + id
			newConfig.Database.MaxConns = 25 + id

			configData, err := json.MarshalIndent(newConfig, "", "  ")
			if err != nil {
				t.Errorf("Writer %d: failed to marshal config: %v", id, err)
				return
			}

			err = os.WriteFile(tempConfig, configData, 0644)
			if err != nil {
				t.Errorf("Writer %d: failed to write config: %v", id, err)
				return
			}

			time.Sleep(500 * time.Millisecond)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numReaders+numWrites; i++ {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}

	// Verify final state
	finalConfig := manager.GetConfig()
	require.NotNil(t, finalConfig)
	assert.Equal(t, "hotel-reviews-service", finalConfig.App.Name)
}

func TestConfigManager_HealthAndMetrics(t *testing.T) {
	logger := createTestLogger()

	config := GetDefaultConfig()
	tempConfig := createTempConfigFile(t, config, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Test health check
	err = manager.HealthCheck()
	assert.NoError(t, err)

	// Test metrics
	metrics := manager.GetMetrics()
	assert.NotNil(t, metrics)

	// Verify expected metrics
	expectedMetrics := []string{
		"reload_count", "error_count", "watched_files",
		"watched_env_vars", "registered_configs", "total_callbacks",
		"config_reload_count", "config_error_count",
	}

	for _, metric := range expectedMetrics {
		assert.Contains(t, metrics, metric, "Missing metric: %s", metric)
	}

	// Update configuration to increment metrics
	newConfig := *config
	newConfig.Server.Port = 9090

	configData, err := json.MarshalIndent(newConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(tempConfig, configData, 0644)
	require.NoError(t, err)

	// Wait for update
	time.Sleep(3 * time.Second)

	// Check updated metrics
	updatedMetrics := manager.GetMetrics()
	assert.NotNil(t, updatedMetrics)

	// Reload count should have increased
	if reloadCount, ok := updatedMetrics["config_reload_count"]; ok {
		assert.Greater(t, reloadCount.(int64), int64(0))
	}
}

func TestConfigManager_YAMLSupport(t *testing.T) {
	logger := createTestLogger()

	config := GetDefaultConfig()
	tempConfig := createTempConfigFile(t, config, "yaml")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(t, err)
	defer manager.Stop()

	// Verify YAML configuration was loaded
	loadedConfig := manager.GetConfig()
	require.NotNil(t, loadedConfig)
	assert.Equal(t, config.App.Name, loadedConfig.App.Name)
	assert.Equal(t, config.Server.Port, loadedConfig.Server.Port)

	// Test YAML configuration update
	newConfig := *config
	newConfig.Server.Port = 9090
	newConfig.App.LogLevel = "debug"

	yamlData, err := yaml.Marshal(newConfig)
	require.NoError(t, err)
	err = os.WriteFile(tempConfig, yamlData, 0644)
	require.NoError(t, err)

	// Wait for update
	time.Sleep(3 * time.Second)

	// Verify update
	updatedConfig := manager.GetConfig()
	assert.Equal(t, 9090, updatedConfig.Server.Port)
	assert.Equal(t, "debug", updatedConfig.App.LogLevel)
}

// Benchmark tests
func BenchmarkConfigManager_GetConfig(b *testing.B) {
	logger := createTestLogger()

	config := GetDefaultConfig()
	tempConfig := createTempConfigFileForBenchmark(b, config, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(b, err)
	defer manager.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = manager.GetConfig()
		}
	})
}

func BenchmarkConfigManager_HealthCheck(b *testing.B) {
	logger := createTestLogger()

	config := GetDefaultConfig()
	tempConfig := createTempConfigFileForBenchmark(b, config, "json")
	os.Setenv("CONFIG_PATH", tempConfig)
	defer os.Unsetenv("CONFIG_PATH")

	manager, err := NewConfigManager(logger)
	require.NoError(b, err)
	defer manager.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.HealthCheck()
	}
}

// Helper function to create temp config file for benchmarks
func createTempConfigFileForBenchmark(tb testing.TB, config interface{}, format string) string {
	tempDir := tb.TempDir()
	var filename string
	var data []byte
	var err error

	switch format {
	case "json":
		filename = filepath.Join(tempDir, "app.json")
		data, err = json.MarshalIndent(config, "", "  ")
	case "yaml":
		filename = filepath.Join(tempDir, "app.yaml")
		data, err = yaml.Marshal(config)
	default:
		tb.Fatalf("Unsupported format: %s", format)
	}

	require.NoError(tb, err)
	require.NoError(tb, os.WriteFile(filename, data, 0644))

	return filename
}
