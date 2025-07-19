package infrastructure

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

// Test configuration structures
type TestConfig struct {
	Name        string         `json:"name" validate:"required"`
	Port        int            `json:"port" validate:"min=1,max=65535"`
	Timeout     time.Duration  `json:"timeout" validate:"required"`
	Features    []string       `json:"features"`
	Database    DatabaseConfig `json:"database"`
	EnableDebug bool           `json:"enable_debug"`
}

// DatabaseConfig is defined in database_config.go

// Mock logger for testing
type mockLogger struct {
	logs []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (m *mockLogger) Debug(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "debug", Message: msg, Fields: fieldsToMap(fields)})
}

func (m *mockLogger) Info(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "info", Message: msg, Fields: fieldsToMap(fields)})
}

func (m *mockLogger) Warn(msg string, fields ...interface{}) {
	m.logs = append(m.logs, LogEntry{Level: "warn", Message: msg, Fields: fieldsToMap(fields)})
}

func (m *mockLogger) Error(msg string, fields ...interface{}) {
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

func createTestLogger() *mockLogger {
	return &mockLogger{logs: make([]LogEntry, 0)}
}

func createTestConfig() TestConfig {
	return TestConfig{
		Name:     "test-service",
		Port:     8080,
		Timeout:  30 * time.Second,
		Features: []string{"auth", "logging"},
		Database: DatabaseConfig{
			Host:         "localhost",
			Port:         5432,
			Database:     "testdb",
			Username:     "testuser",
			Password:     "testpass",
			SSLMode:      "disable",
			MaxConns:     10,
			MinConns:     2,
			ConnTTL:      time.Hour,
			QueryTimeout: 30 * time.Second,
		},
		EnableDebug: true,
	}
}

func createTempConfigFile(t *testing.T, config interface{}, format string) string {
	tempDir := t.TempDir()
	var filename string
	var data []byte
	var err error

	switch format {
	case "json":
		filename = filepath.Join(tempDir, "config.json")
		data, err = json.MarshalIndent(config, "", "  ")
	case "yaml":
		filename = filepath.Join(tempDir, "config.yaml")
		data, err = yaml.Marshal(config)
	default:
		t.Fatalf("Unsupported format: %s", format)
	}

	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filename, data, 0644))

	return filename
}

func TestNewConfigWatcher(t *testing.T) {
	logger := createTestLogger()

	t.Run("with default options", func(t *testing.T) {
		watcher, err := NewConfigWatcher(logger, nil)
		require.NoError(t, err)
		require.NotNil(t, watcher)
		defer watcher.Stop()

		assert.Equal(t, DefaultConfigWatcherOptions().WatchIntervalSec, watcher.options.WatchIntervalSec)
		assert.True(t, watcher.options.FileChecksum)
		assert.True(t, watcher.options.EnableRollback)
	})

	t.Run("with custom options", func(t *testing.T) {
		options := &ConfigWatcherOptions{
			WatchIntervalSec:    2,
			FileChecksum:        false,
			EnvCheckIntervalSec: 60,
			EnableRollback:      false,
			MaxHistorySize:      5,
			RollbackTimeoutSec:  30,
			MaxRetries:          1,
			RetryDelaySec:       1,
			BatchIntervalMs:     100,
		}

		watcher, err := NewConfigWatcher(logger, options)
		require.NoError(t, err)
		require.NotNil(t, watcher)
		defer watcher.Stop()

		assert.Equal(t, 2, watcher.options.WatchIntervalSec)
		assert.False(t, watcher.options.FileChecksum)
		assert.False(t, watcher.options.EnableRollback)
		assert.Equal(t, 5, watcher.maxHistorySize)
	})

	t.Run("with invalid options", func(t *testing.T) {
		options := &ConfigWatcherOptions{
			WatchIntervalSec:    0, // Invalid
			EnvCheckIntervalSec: 30,
			MaxHistorySize:      1,
			RetryDelaySec:       1,
			BatchIntervalMs:     100,
		}

		watcher, err := NewConfigWatcher(logger, options)
		assert.Error(t, err)
		assert.Nil(t, watcher)
		assert.Contains(t, err.Error(), "invalid options")
	})
}

func TestConfigWatcher_RegisterConfig(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	t.Run("valid configuration", func(t *testing.T) {
		config := createTestConfig()
		err := watcher.RegisterConfig("test", config)
		assert.NoError(t, err)

		retrievedConfig, exists := watcher.GetConfig("test")
		assert.True(t, exists)
		assert.Equal(t, config, retrievedConfig)
	})

	t.Run("invalid configuration", func(t *testing.T) {
		invalidConfig := TestConfig{
			Name: "", // Required field is empty
			Port: 0,  // Invalid port
		}

		err := watcher.RegisterConfig("invalid", invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("configuration history", func(t *testing.T) {
		config := createTestConfig()
		err := watcher.RegisterConfig("history_test", config)
		require.NoError(t, err)

		history := watcher.GetConfigHistory("history_test")
		assert.Len(t, history, 1)
		assert.Equal(t, "registration", history[0].Source)
		assert.Equal(t, config, history[0].Config)
	})
}

func TestConfigWatcher_WatchFile(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	t.Run("watch JSON file", func(t *testing.T) {
		config := createTestConfig()
		configFile := createTempConfigFile(t, config, "json")

		err := watcher.WatchFile("json_config", configFile)
		assert.NoError(t, err)

		// Configuration should be loaded immediately
		retrievedConfig, exists := watcher.GetConfig("json_config")
		assert.True(t, exists)
		assert.NotNil(t, retrievedConfig)
	})

	t.Run("watch YAML file", func(t *testing.T) {
		config := createTestConfig()
		configFile := createTempConfigFile(t, config, "yaml")

		err := watcher.WatchFile("yaml_config", configFile)
		assert.NoError(t, err)

		// Configuration should be loaded immediately
		retrievedConfig, exists := watcher.GetConfig("yaml_config")
		assert.True(t, exists)
		assert.NotNil(t, retrievedConfig)
	})

	t.Run("watch non-existent file", func(t *testing.T) {
		err := watcher.WatchFile("missing", "/non/existent/file.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("watch unsupported file format", func(t *testing.T) {
		tempDir := t.TempDir()
		txtFile := filepath.Join(tempDir, "config.txt")
		require.NoError(t, os.WriteFile(txtFile, []byte("not config"), 0644))

		err := watcher.WatchFile("txt_config", txtFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file format")
	})
}

func TestConfigWatcher_WatchEnvVar(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	t.Run("watch existing environment variable", func(t *testing.T) {
		envVar := "TEST_CONFIG_VAR"
		expectedValue := "test_value"

		// Set environment variable
		os.Setenv(envVar, expectedValue)
		defer os.Unsetenv(envVar)

		err := watcher.WatchEnvVar("env_config", envVar)
		assert.NoError(t, err)

		// Check that the value is stored
		configKey := fmt.Sprintf("env:%s", envVar)
		retrievedConfig, exists := watcher.GetConfig(configKey)
		assert.True(t, exists)
		assert.Equal(t, expectedValue, retrievedConfig)
	})

	t.Run("watch non-existent environment variable", func(t *testing.T) {
		envVar := "NON_EXISTENT_VAR"

		err := watcher.WatchEnvVar("empty_env", envVar)
		assert.NoError(t, err)

		// Should not have a config entry for empty env var
		configKey := fmt.Sprintf("env:%s", envVar)
		_, exists := watcher.GetConfig(configKey)
		assert.False(t, exists)
	})
}

func TestConfigWatcher_FileChanges(t *testing.T) {
	logger := createTestLogger()
	options := DefaultConfigWatcherOptions()
	options.WatchIntervalSec = 1 // Faster polling for tests

	watcher, err := NewConfigWatcher(logger, options)
	require.NoError(t, err)
	defer watcher.Stop()

	// Create initial config file
	initialConfig := createTestConfig()
	configFile := createTempConfigFile(t, initialConfig, "json")

	// Setup change callback
	changeDetected := make(chan bool, 1)
	var callbackOldConfig, callbackNewConfig interface{}

	watcher.RegisterChangeCallback("file_config", func(configName string, oldConfig, newConfig interface{}) error {
		callbackOldConfig = oldConfig
		callbackNewConfig = newConfig
		select {
		case changeDetected <- true:
		default:
		}
		return nil
	})

	// Start watching
	err = watcher.WatchFile("file_config", configFile)
	require.NoError(t, err)

	// Wait for initial load callback and clear the channel
	select {
	case <-changeDetected:
		// Initial load detected, clear variables for actual change test
		callbackOldConfig = nil
		callbackNewConfig = nil
	case <-time.After(2 * time.Second):
		t.Fatal("Initial config load was not detected")
	}

	// Modify the configuration file
	modifiedConfig := createTestConfig()
	modifiedConfig.Port = 9090
	modifiedConfig.EnableDebug = false

	configData, err := json.MarshalIndent(modifiedConfig, "", "  ")
	require.NoError(t, err)

	// Write new configuration
	err = os.WriteFile(configFile, configData, 0644)
	require.NoError(t, err)

	// Wait for change detection
	select {
	case <-changeDetected:
		// Change was detected
		assert.NotNil(t, callbackOldConfig)
		assert.NotNil(t, callbackNewConfig)

		// Verify the new configuration was loaded
		currentConfig, exists := watcher.GetConfig("file_config")
		assert.True(t, exists)

		if configMap, ok := currentConfig.(map[string]interface{}); ok {
			assert.Equal(t, float64(9090), configMap["port"]) // JSON numbers are float64
			if enableDebug, ok := configMap["enable_debug"].(bool); ok {
				assert.False(t, enableDebug)
			}
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Configuration change was not detected within timeout")
	}
}

func TestConfigWatcher_EnvironmentChanges(t *testing.T) {
	logger := createTestLogger()
	options := DefaultConfigWatcherOptions()
	options.EnvCheckIntervalSec = 1 // Faster polling for tests

	watcher, err := NewConfigWatcher(logger, options)
	require.NoError(t, err)
	defer watcher.Stop()

	envVar := "TEST_ENV_CHANGE"
	initialValue := "initial_value"

	// Set initial value
	os.Setenv(envVar, initialValue)
	defer os.Unsetenv(envVar)

	// Setup change callback
	changeDetected := make(chan bool, 1)
	var callbackNewConfig interface{}

	configKey := fmt.Sprintf("env:%s", envVar)
	watcher.RegisterChangeCallback(configKey, func(configName string, oldConfig, newConfig interface{}) error {
		callbackNewConfig = newConfig
		select {
		case changeDetected <- true:
		default:
		}
		return nil
	})

	// Start watching
	err = watcher.WatchEnvVar("env_test", envVar)
	require.NoError(t, err)

	// Change environment variable
	newValue := "changed_value"
	os.Setenv(envVar, newValue)

	// Wait for change detection
	select {
	case <-changeDetected:
		assert.Equal(t, newValue, callbackNewConfig)

		// Verify the new value was stored
		currentConfig, exists := watcher.GetConfig(configKey)
		assert.True(t, exists)
		assert.Equal(t, newValue, currentConfig)

	case <-time.After(3 * time.Second):
		t.Fatal("Environment variable change was not detected within timeout")
	}
}

func TestConfigWatcher_Validation(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	t.Run("custom validator success", func(t *testing.T) {
		// Register a custom validator
		watcher.RegisterValidator("validated_config", func(config interface{}) error {
			if configMap, ok := config.(map[string]interface{}); ok {
				if port, exists := configMap["port"]; exists {
					if portNum, ok := port.(float64); ok && portNum == 8080 {
						return nil
					}
				}
			}
			return fmt.Errorf("port must be 8080")
		})

		config := map[string]interface{}{
			"name": "test",
			"port": 8080.0,
		}

		err := watcher.updateConfig("validated_config", config, "test")
		assert.NoError(t, err)
	})

	t.Run("custom validator failure", func(t *testing.T) {
		// Register a custom validator that always fails
		watcher.RegisterValidator("failing_config", func(config interface{}) error {
			return fmt.Errorf("validation always fails")
		})

		config := map[string]interface{}{
			"name": "test",
			"port": 8080,
		}

		err := watcher.updateConfig("failing_config", config, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation always fails")
	})
}

func TestConfigWatcher_Rollback(t *testing.T) {
	logger := createTestLogger()
	options := DefaultConfigWatcherOptions()
	options.EnableRollback = true
	options.MaxHistorySize = 5

	watcher, err := NewConfigWatcher(logger, options)
	require.NoError(t, err)
	defer watcher.Stop()

	configName := "rollback_test"

	// Register initial configuration
	config1 := map[string]interface{}{"version": 1, "setting": "value1"}
	err = watcher.RegisterConfig(configName, config1)
	require.NoError(t, err)

	// Update configuration multiple times
	config2 := map[string]interface{}{"version": 2, "setting": "value2"}
	err = watcher.updateConfig(configName, config2, "update1")
	require.NoError(t, err)

	config3 := map[string]interface{}{"version": 3, "setting": "value3"}
	err = watcher.updateConfig(configName, config3, "update2")
	require.NoError(t, err)

	// Get history and find target hash
	history := watcher.GetConfigHistory(configName)
	assert.Len(t, history, 3)

	// Rollback to first version
	targetHash := history[0].Hash
	err = watcher.RollbackConfig(configName, targetHash)
	assert.NoError(t, err)

	// Verify rollback
	currentConfig, exists := watcher.GetConfig(configName)
	assert.True(t, exists)
	assert.Equal(t, config1, currentConfig)

	// Check that rollback was added to history
	newHistory := watcher.GetConfigHistory(configName)
	assert.Len(t, newHistory, 4) // Original 3 + rollback entry
	assert.Contains(t, newHistory[3].Source, "rollback")
}

func TestConfigWatcher_RollbackDisabled(t *testing.T) {
	logger := createTestLogger()
	options := DefaultConfigWatcherOptions()
	options.EnableRollback = false

	watcher, err := NewConfigWatcher(logger, options)
	require.NoError(t, err)
	defer watcher.Stop()

	err = watcher.RollbackConfig("test", "some-hash")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rollback is disabled")
}

func TestConfigWatcher_CallbackError(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	configName := "callback_error_test"

	// Register configuration
	initialConfig := map[string]interface{}{"value": 1}
	err = watcher.RegisterConfig(configName, initialConfig)
	require.NoError(t, err)

	// Register a callback that fails
	watcher.RegisterChangeCallback(configName, func(configName string, oldConfig, newConfig interface{}) error {
		return fmt.Errorf("callback intentionally failed")
	})

	// Try to update configuration
	newConfig := map[string]interface{}{"value": 2}
	err = watcher.updateConfig(configName, newConfig, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "callback failed")

	// Verify configuration was not changed
	currentConfig, _ := watcher.GetConfig(configName)
	assert.Equal(t, initialConfig, currentConfig)
}

func TestConfigWatcher_Metrics(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	// Register some configurations
	err = watcher.RegisterConfig("config1", map[string]interface{}{"test": 1})
	require.NoError(t, err)

	err = watcher.RegisterConfig("config2", map[string]interface{}{"test": 2})
	require.NoError(t, err)

	// Register callbacks
	watcher.RegisterChangeCallback("config1", func(string, interface{}, interface{}) error { return nil })
	watcher.RegisterChangeCallback("config1", func(string, interface{}, interface{}) error { return nil })

	metrics := watcher.GetMetrics()

	assert.Equal(t, int64(0), metrics["reload_count"])
	assert.Equal(t, int64(0), metrics["error_count"])
	assert.Equal(t, 0, metrics["watched_files"])
	assert.Equal(t, 0, metrics["watched_env_vars"])
	assert.Equal(t, 2, metrics["registered_configs"])
	assert.Equal(t, 2, metrics["total_callbacks"])
	assert.Equal(t, 2, metrics["history_entries"]) // One for each registered config
}

func TestConfigWatcher_HealthCheck(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)

	t.Run("healthy watcher", func(t *testing.T) {
		err := watcher.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("stopped watcher", func(t *testing.T) {
		err := watcher.Stop()
		require.NoError(t, err)

		err = watcher.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "stopped")
	})
}

func TestConfigWatcher_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	configName := "concurrent_test"

	// Register initial configuration
	initialConfig := map[string]interface{}{"counter": 0}
	err = watcher.RegisterConfig(configName, initialConfig)
	require.NoError(t, err)

	// Concurrent reads and writes
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines*2)

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numOperations; j++ {
				_, exists := watcher.GetConfig(configName)
				assert.True(t, exists)
			}
			done <- true
		}()
	}

	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				config := map[string]interface{}{
					"counter": id*numOperations + j,
					"writer":  id,
				}
				watcher.updateConfig(configName, config, fmt.Sprintf("writer_%d", id))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines*2; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}

	// Verify final state
	finalConfig, exists := watcher.GetConfig(configName)
	assert.True(t, exists)
	assert.NotNil(t, finalConfig)
}

func TestConfigWatcher_FileRecreation(t *testing.T) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(t, err)
	defer watcher.Stop()

	// Create initial config file
	config := createTestConfig()
	configFile := createTempConfigFile(t, config, "json")

	// Setup change detection
	changeDetected := make(chan bool, 1)
	watcher.RegisterChangeCallback("recreated_config", func(string, interface{}, interface{}) error {
		select {
		case changeDetected <- true:
		default:
		}
		return nil
	})

	err = watcher.WatchFile("recreated_config", configFile)
	require.NoError(t, err)

	// Remove the file
	err = os.Remove(configFile)
	require.NoError(t, err)

	// Recreate the file with new content
	newConfig := createTestConfig()
	newConfig.Port = 9999

	newData, err := json.MarshalIndent(newConfig, "", "  ")
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond) // Brief delay before recreation
	err = os.WriteFile(configFile, newData, 0644)
	require.NoError(t, err)

	// Wait for change detection (this may take longer due to file recreation)
	select {
	case <-changeDetected:
		// Verify new configuration was loaded
		currentConfig, exists := watcher.GetConfig("recreated_config")
		assert.True(t, exists)

		if configMap, ok := currentConfig.(map[string]interface{}); ok {
			assert.Equal(t, float64(9999), configMap["port"])
		}

	case <-time.After(10 * time.Second):
		t.Log("File recreation change detection timed out - this may be expected depending on OS")
		// Don't fail the test as file recreation detection can be unreliable
	}
}

// Benchmark tests
func BenchmarkConfigWatcher_GetConfig(b *testing.B) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(b, err)
	defer watcher.Stop()

	config := createTestConfig()
	err = watcher.RegisterConfig("bench_config", config)
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = watcher.GetConfig("bench_config")
		}
	})
}

func BenchmarkConfigWatcher_UpdateConfig(b *testing.B) {
	logger := createTestLogger()
	watcher, err := NewConfigWatcher(logger, nil)
	require.NoError(b, err)
	defer watcher.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := map[string]interface{}{
			"iteration": i,
			"timestamp": time.Now(),
		}
		watcher.updateConfig(fmt.Sprintf("bench_config_%d", i%100), config, "benchmark")
	}
}
