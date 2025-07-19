package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock logger for testing
type mockShutdownLogger struct {
	logs []shutdownLogEntry
	mu   sync.Mutex
}

type shutdownLogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (m *mockShutdownLogger) Debug(msg string, fields ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, shutdownLogEntry{Level: "debug", Message: msg, Fields: shutdownFieldsToMap(fields)})
}

func (m *mockShutdownLogger) Info(msg string, fields ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, shutdownLogEntry{Level: "info", Message: msg, Fields: shutdownFieldsToMap(fields)})
}

func (m *mockShutdownLogger) Warn(msg string, fields ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, shutdownLogEntry{Level: "warn", Message: msg, Fields: shutdownFieldsToMap(fields)})
}

func (m *mockShutdownLogger) Error(msg string, fields ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, shutdownLogEntry{Level: "error", Message: msg, Fields: shutdownFieldsToMap(fields)})
}

func shutdownFieldsToMap(fields []interface{}) map[string]interface{} {
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

func createTestShutdownLogger() *mockShutdownLogger {
	return &mockShutdownLogger{logs: make([]shutdownLogEntry, 0)}
}

// Mock implementations for testing
type mockWorker struct {
	name      string
	stopCalled bool
	stopError error
	stopDelay time.Duration
	mu        sync.Mutex
}

func (w *mockWorker) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	w.stopCalled = true
	
	if w.stopDelay > 0 {
		select {
		case <-time.After(w.stopDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return w.stopError
}

func (w *mockWorker) Name() string {
	return w.name
}

type mockExternalConnection struct {
	name        string
	closeCalled bool
	closeError  error
	closeDelay  time.Duration
	mu          sync.Mutex
}

func (c *mockExternalConnection) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.closeCalled = true
	
	if c.closeDelay > 0 {
		select {
		case <-time.After(c.closeDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return c.closeError
}

func (c *mockExternalConnection) Name() string {
	return c.name
}

type mockOptimizer struct {
	name       string
	stopCalled bool
	stopError  error
	mu         sync.Mutex
}

func (o *mockOptimizer) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	o.stopCalled = true
	return o.stopError
}

func (o *mockOptimizer) Name() string {
	return o.name
}

type mockConfigWatcher struct {
	name       string
	stopCalled bool
	stopError  error
	mu         sync.Mutex
}

func (w *mockConfigWatcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	w.stopCalled = true
	return w.stopError
}

func (w *mockConfigWatcher) Name() string {
	return w.name
}

func TestDefaultShutdownConfig(t *testing.T) {
	config := DefaultShutdownConfig()
	
	assert.Equal(t, 30*time.Second, config.GracefulTimeout)
	assert.Equal(t, 10*time.Second, config.ServerTimeout)
	assert.Equal(t, 5*time.Second, config.DatabaseTimeout)
	assert.Equal(t, 15*time.Second, config.WorkerTimeout)
	assert.Equal(t, 5*time.Second, config.ExternalTimeout)
	assert.Equal(t, 45*time.Second, config.ForceKillTimeout)
	assert.True(t, config.EnablePreShutdownHook)
	assert.True(t, config.LogShutdownProgress)
	assert.False(t, config.DisableHealthCheck)
	assert.Contains(t, config.Signals, syscall.SIGINT)
	assert.Contains(t, config.Signals, syscall.SIGTERM)
}

func TestNewShutdownManager(t *testing.T) {
	logger := createTestShutdownLogger()
	
	t.Run("with default config", func(t *testing.T) {
		manager := NewShutdownManager(logger, nil)
		require.NotNil(t, manager)
		assert.NotNil(t, manager.config)
		assert.Equal(t, 30*time.Second, manager.config.GracefulTimeout)
	})
	
	t.Run("with custom config", func(t *testing.T) {
		config := &ShutdownConfig{
			GracefulTimeout: 60 * time.Second,
			ServerTimeout:   20 * time.Second,
		}
		
		manager := NewShutdownManager(logger, config)
		require.NotNil(t, manager)
		assert.Equal(t, 60*time.Second, manager.config.GracefulTimeout)
		assert.Equal(t, 20*time.Second, manager.config.ServerTimeout)
	})
}

func TestShutdownManager_RegisterResource(t *testing.T) {
	logger := createTestShutdownLogger()
	manager := NewShutdownManager(logger, nil)
	
	t.Run("register HTTP server", func(t *testing.T) {
		server := &http.Server{}
		err := manager.RegisterResource(server)
		assert.NoError(t, err)
		assert.Len(t, manager.hooks, 1)
		assert.Equal(t, "http_server", manager.hooks[0].Name)
		assert.Equal(t, 1, manager.hooks[0].Priority)
	})
	
	t.Run("register worker", func(t *testing.T) {
		worker := &mockWorker{name: "test_worker"}
		err := manager.RegisterResource(worker)
		assert.NoError(t, err)
		
		// Should have HTTP server + worker
		assert.Len(t, manager.hooks, 2)
		
		// Find worker hook
		var workerHook *ShutdownHook
		for _, hook := range manager.hooks {
			if hook.Name == "worker_test_worker" {
				workerHook = &hook
				break
			}
		}
		require.NotNil(t, workerHook)
		assert.Equal(t, 2, workerHook.Priority)
	})
	
	t.Run("register external connection", func(t *testing.T) {
		conn := &mockExternalConnection{name: "test_connection"}
		err := manager.RegisterResource(conn)
		assert.NoError(t, err)
		
		// Find connection hook
		var connHook *ShutdownHook
		for _, hook := range manager.hooks {
			if hook.Name == "external_test_connection" {
				connHook = &hook
				break
			}
		}
		require.NotNil(t, connHook)
		assert.Equal(t, 4, connHook.Priority)
	})
	
	t.Run("register unsupported type", func(t *testing.T) {
		unsupported := "string type"
		err := manager.RegisterResource(unsupported)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported resource type")
	})
}

func TestShutdownManager_ResourceManager(t *testing.T) {
	logger := createTestShutdownLogger()
	manager := NewShutdownManager(logger, nil)
	
	// Create resource manager with various resources
	rm := NewResourceManager()
	rm.AddHTTPServer(&http.Server{})
	rm.AddWorker(&mockWorker{name: "worker1"})
	rm.AddWorker(&mockWorker{name: "worker2"})
	rm.AddExternalConnection(&mockExternalConnection{name: "conn1"})
	rm.AddOptimizer(&mockOptimizer{name: "opt1"})
	rm.AddWatcher(&mockConfigWatcher{name: "watcher1"})
	
	err := manager.RegisterResource(rm)
	assert.NoError(t, err)
	
	// Should have hooks for all resources
	expectedHooks := []string{
		"http_server",
		"worker_worker1",
		"worker_worker2", 
		"external_conn1",
		"optimizer_opt1",
		"watcher_watcher1",
	}
	
	assert.Len(t, manager.hooks, len(expectedHooks))
	
	hookNames := make([]string, len(manager.hooks))
	for i, hook := range manager.hooks {
		hookNames[i] = hook.Name
	}
	
	for _, expected := range expectedHooks {
		assert.Contains(t, hookNames, expected)
	}
}

func TestShutdownManager_AddCustomHook(t *testing.T) {
	logger := createTestShutdownLogger()
	manager := NewShutdownManager(logger, nil)
	
	hook := ShutdownHook{
		Name:     "custom_hook",
		Priority: 5,
		Timeout:  10 * time.Second,
		Cleanup: func(ctx context.Context) error {
			return nil
		},
	}
	
	manager.AddCustomHook(hook)
	
	assert.Len(t, manager.hooks, 1)
	assert.Equal(t, "custom_hook", manager.hooks[0].Name)
	assert.Equal(t, 5, manager.hooks[0].Priority)
}

func TestShutdownManager_HookPrioritySorting(t *testing.T) {
	logger := createTestShutdownLogger()
	manager := NewShutdownManager(logger, nil)
	
	// Add hooks in random order
	manager.AddCustomHook(ShutdownHook{Name: "priority_3", Priority: 3, Cleanup: func(ctx context.Context) error { return nil }})
	manager.AddCustomHook(ShutdownHook{Name: "priority_1", Priority: 1, Cleanup: func(ctx context.Context) error { return nil }})
	manager.AddCustomHook(ShutdownHook{Name: "priority_2", Priority: 2, Cleanup: func(ctx context.Context) error { return nil }})
	
	// Sort hooks
	manager.sortHooksByPriority()
	
	// Verify order
	assert.Equal(t, "priority_1", manager.hooks[0].Name)
	assert.Equal(t, "priority_2", manager.hooks[1].Name)
	assert.Equal(t, "priority_3", manager.hooks[2].Name)
}

func TestShutdownManager_Shutdown(t *testing.T) {
	logger := createTestShutdownLogger()
	config := DefaultShutdownConfig()
	config.GracefulTimeout = 5 * time.Second
	config.PreShutdownDelay = 100 * time.Millisecond
	config.ForceKillTimeout = 10 * time.Second
	
	manager := NewShutdownManager(logger, config)
	
	// Add some test resources
	worker := &mockWorker{name: "test_worker"}
	conn := &mockExternalConnection{name: "test_conn"}
	optimizer := &mockOptimizer{name: "test_optimizer"}
	
	err := manager.RegisterResource(worker)
	require.NoError(t, err)
	err = manager.RegisterResource(conn)
	require.NoError(t, err)
	err = manager.RegisterResource(optimizer)
	require.NoError(t, err)
	
	// Trigger shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		manager.Shutdown()
	}()
	
	// Wait for shutdown
	err = manager.WaitForShutdown()
	assert.NoError(t, err)
	
	// Verify all resources were stopped
	assert.True(t, worker.stopCalled)
	assert.True(t, conn.closeCalled)
	assert.True(t, optimizer.stopCalled)
}

func TestShutdownManager_ShutdownWithErrors(t *testing.T) {
	logger := createTestShutdownLogger()
	config := DefaultShutdownConfig()
	config.GracefulTimeout = 2 * time.Second
	config.PreShutdownDelay = 0
	
	manager := NewShutdownManager(logger, config)
	
	// Add worker that will fail
	worker := &mockWorker{
		name:      "failing_worker",
		stopError: errors.New("worker failed to stop"),
	}
	
	err := manager.RegisterResource(worker)
	require.NoError(t, err)
	
	// Trigger shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		manager.Shutdown()
	}()
	
	// Wait for shutdown - should return error
	err = manager.WaitForShutdown()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown completed with")
	
	// Verify worker was called even though it failed
	assert.True(t, worker.stopCalled)
}

func TestShutdownManager_ShutdownTimeout(t *testing.T) {
	logger := createTestShutdownLogger()
	config := DefaultShutdownConfig()
	config.GracefulTimeout = 1 * time.Second
	config.WorkerTimeout = 500 * time.Millisecond
	config.PreShutdownDelay = 0
	config.ForceKillTimeout = 5 * time.Second // Prevent force kill during test
	
	manager := NewShutdownManager(logger, config)
	
	// Add worker that takes too long to stop
	worker := &mockWorker{
		name:      "slow_worker",
		stopDelay: 2 * time.Second, // Longer than timeout
	}
	
	err := manager.RegisterResource(worker)
	require.NoError(t, err)
	
	// Trigger shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		manager.Shutdown()
	}()
	
	// Wait for shutdown - should return error due to timeout
	err = manager.WaitForShutdown()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown completed with")
	
	// Verify worker was called
	assert.True(t, worker.stopCalled)
}

func TestShutdownManager_GetShutdownMetrics(t *testing.T) {
	logger := createTestShutdownLogger()
	manager := NewShutdownManager(logger, nil)
	
	// Add some hooks
	err := manager.RegisterResource(&mockWorker{name: "worker1"})
	require.NoError(t, err)
	err = manager.RegisterResource(&mockWorker{name: "worker2"})
	require.NoError(t, err)
	
	metrics := manager.GetShutdownMetrics()
	
	assert.Equal(t, 2, metrics["registered_hooks"])
	assert.Equal(t, 30*time.Second, metrics["graceful_timeout"])
	assert.Equal(t, 45*time.Second, metrics["force_kill_timeout"])
	assert.False(t, metrics["shutdown_initiated"].(bool))
}

func TestShutdownManager_ConcurrentShutdown(t *testing.T) {
	logger := createTestShutdownLogger()
	config := DefaultShutdownConfig()
	config.GracefulTimeout = 2 * time.Second
	config.PreShutdownDelay = 0
	
	manager := NewShutdownManager(logger, config)
	
	// Add multiple workers
	for i := 0; i < 5; i++ {
		worker := &mockWorker{name: fmt.Sprintf("worker_%d", i)}
		err := manager.RegisterResource(worker)
		require.NoError(t, err)
	}
	
	// Trigger multiple concurrent shutdowns
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Shutdown()
		}()
	}
	
	// Wait for shutdown
	err := manager.WaitForShutdown()
	assert.NoError(t, err)
	
	wg.Wait()
}

func TestDBOptimizerWrapper(t *testing.T) {
	// Test with nil optimizer - should handle gracefully
	wrapper := &DBOptimizerWrapper{optimizer: nil}
	
	assert.Equal(t, "db_optimizer", wrapper.Name())
	
	// Test stop with nil optimizer - should return error, not panic
	err := wrapper.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db optimizer is nil")
}

func TestConfigWatcherWrapper(t *testing.T) {
	// Test with nil watcher - should handle gracefully
	wrapper := &ConfigWatcherWrapper{watcher: nil}
	
	assert.Equal(t, "config_watcher", wrapper.Name())
	
	// Test stop with nil watcher - should return error, not panic
	err := wrapper.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config watcher is nil")
}

// Benchmark tests
func BenchmarkShutdownManager_RegisterResource(b *testing.B) {
	logger := createTestShutdownLogger()
	manager := NewShutdownManager(logger, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		worker := &mockWorker{name: fmt.Sprintf("worker_%d", i)}
		manager.RegisterResource(worker)
	}
}

func BenchmarkShutdownManager_Shutdown(b *testing.B) {
	logger := createTestShutdownLogger()
	config := DefaultShutdownConfig()
	config.PreShutdownDelay = 0
	config.GracefulTimeout = 100 * time.Millisecond
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := NewShutdownManager(logger, config)
		
		// Add a simple worker
		worker := &mockWorker{name: "bench_worker"}
		manager.RegisterResource(worker)
		
		// Trigger shutdown
		go manager.Shutdown()
		manager.WaitForShutdown()
	}
}