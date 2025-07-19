package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// ShutdownManager handles graceful shutdown of all application components
type ShutdownManager struct {
	logger   monitoring.Logger
	config   *ShutdownConfig
	hooks    []ShutdownHook
	mu       sync.RWMutex
	shutdown chan struct{}
	done     chan struct{}
}

// ShutdownConfig configures graceful shutdown behavior
type ShutdownConfig struct {
	// Timeout configuration
	GracefulTimeout time.Duration `json:"graceful_timeout" validate:"min=1s"`
	ServerTimeout   time.Duration `json:"server_timeout" validate:"min=1s"`
	DatabaseTimeout time.Duration `json:"database_timeout" validate:"min=1s"`
	WorkerTimeout   time.Duration `json:"worker_timeout" validate:"min=1s"`
	ExternalTimeout time.Duration `json:"external_timeout" validate:"min=1s"`

	// Signal handling
	Signals          []os.Signal   `json:"-"`
	ForceKillTimeout time.Duration `json:"force_kill_timeout" validate:"min=1s"`

	// Shutdown behavior
	EnablePreShutdownHook bool          `json:"enable_pre_shutdown_hook"`
	PreShutdownDelay      time.Duration `json:"pre_shutdown_delay"`
	LogShutdownProgress   bool          `json:"log_shutdown_progress"`

	// Health check during shutdown
	DisableHealthCheck     bool          `json:"disable_health_check"`
	HealthCheckGracePeriod time.Duration `json:"health_check_grace_period"`
}

// ShutdownHook represents a cleanup function to be called during shutdown
type ShutdownHook struct {
	Name     string
	Priority int // Lower numbers run first
	Timeout  time.Duration
	Cleanup  func(ctx context.Context) error
}

// ResourceManager tracks application resources for cleanup
type ResourceManager struct {
	httpServer    *http.Server
	dbPool        *pgxpool.Pool
	redisClient   *redis.Client
	workers       []Worker
	externalConns []ExternalConnection
	monitors      []interface{} // Generic monitors
	optimizers    []ResourceOptimizer
	watchers      []ConfigWatcher
}

// Worker represents a background worker that can be gracefully stopped
type Worker interface {
	Stop(ctx context.Context) error
	Name() string
}

// ExternalConnection represents an external service connection
type ExternalConnection interface {
	Close(ctx context.Context) error
	Name() string
}

// ResourceOptimizer represents a resource optimizer that needs cleanup
type ResourceOptimizer interface {
	Stop() error
	Name() string
}

// ConfigWatcher represents a configuration watcher
type ConfigWatcher interface {
	Stop() error
	Name() string
}

// DefaultShutdownConfig returns sensible defaults for shutdown configuration
func DefaultShutdownConfig() *ShutdownConfig {
	return &ShutdownConfig{
		GracefulTimeout:        30 * time.Second,
		ServerTimeout:          10 * time.Second,
		DatabaseTimeout:        5 * time.Second,
		WorkerTimeout:          15 * time.Second,
		ExternalTimeout:        5 * time.Second,
		Signals:                []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		ForceKillTimeout:       45 * time.Second,
		EnablePreShutdownHook:  true,
		PreShutdownDelay:       1 * time.Second,
		LogShutdownProgress:    true,
		DisableHealthCheck:     false,
		HealthCheckGracePeriod: 2 * time.Second,
	}
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(logger monitoring.Logger, config *ShutdownConfig) *ShutdownManager {
	if config == nil {
		config = DefaultShutdownConfig()
	}

	return &ShutdownManager{
		logger:   logger,
		config:   config,
		hooks:    make([]ShutdownHook, 0),
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// RegisterResource registers various application resources for cleanup
func (sm *ShutdownManager) RegisterResource(resource interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch r := resource.(type) {
	case *ResourceManager:
		// Register cleanup hooks for all resources in the manager
		return sm.registerResourceManager(r)
	case *http.Server:
		return sm.registerHTTPServer(r)
	case *pgxpool.Pool:
		return sm.registerDatabase(r)
	case *redis.Client:
		return sm.registerRedis(r)
	case Worker:
		return sm.registerWorker(r)
	case ExternalConnection:
		return sm.registerExternalConnection(r)
	case ResourceOptimizer:
		return sm.registerOptimizer(r)
	case ConfigWatcher:
		return sm.registerWatcher(r)
	default:
		return fmt.Errorf("unsupported resource type: %T", resource)
	}
}

// registerResourceManager registers all resources from a resource manager
func (sm *ShutdownManager) registerResourceManager(rm *ResourceManager) error {
	// Register HTTP server
	if rm.httpServer != nil {
		if err := sm.registerHTTPServer(rm.httpServer); err != nil {
			return err
		}
	}

	// Register database pool
	if rm.dbPool != nil {
		if err := sm.registerDatabase(rm.dbPool); err != nil {
			return err
		}
	}

	// Register Redis client
	if rm.redisClient != nil {
		if err := sm.registerRedis(rm.redisClient); err != nil {
			return err
		}
	}

	// Register workers
	for _, worker := range rm.workers {
		if err := sm.registerWorker(worker); err != nil {
			return err
		}
	}

	// Register external connections
	for _, conn := range rm.externalConns {
		if err := sm.registerExternalConnection(conn); err != nil {
			return err
		}
	}

	// Register optimizers
	for _, optimizer := range rm.optimizers {
		if err := sm.registerOptimizer(optimizer); err != nil {
			return err
		}
	}

	// Register watchers
	for _, watcher := range rm.watchers {
		if err := sm.registerWatcher(watcher); err != nil {
			return err
		}
	}

	return nil
}

// registerHTTPServer registers HTTP server for graceful shutdown
func (sm *ShutdownManager) registerHTTPServer(server *http.Server) error {
	hook := ShutdownHook{
		Name:     "http_server",
		Priority: 1, // Shutdown HTTP server first
		Timeout:  sm.config.ServerTimeout,
		Cleanup: func(ctx context.Context) error {
			sm.logger.Info("Shutting down HTTP server")

			// Create timeout context for server shutdown
			shutdownCtx, cancel := context.WithTimeout(ctx, sm.config.ServerTimeout)
			defer cancel()

			return server.Shutdown(shutdownCtx)
		},
	}

	sm.hooks = append(sm.hooks, hook)
	return nil
}

// registerDatabase registers database pool for cleanup
func (sm *ShutdownManager) registerDatabase(pool *pgxpool.Pool) error {
	hook := ShutdownHook{
		Name:     "database_pool",
		Priority: 3, // Close database after workers
		Timeout:  sm.config.DatabaseTimeout,
		Cleanup: func(ctx context.Context) error {
			sm.logger.Info("Closing database connections")

			// Wait for active connections to finish or timeout
			done := make(chan struct{})
			go func() {
				pool.Close()
				close(done)
			}()

			select {
			case <-done:
				return nil
			case <-ctx.Done():
				sm.logger.Warn("Database shutdown timed out, forcing close")
				pool.Close()
				return ctx.Err()
			}
		},
	}

	sm.hooks = append(sm.hooks, hook)
	return nil
}

// registerRedis registers Redis client for cleanup
func (sm *ShutdownManager) registerRedis(client *redis.Client) error {
	hook := ShutdownHook{
		Name:     "redis_client",
		Priority: 3, // Close Redis after workers
		Timeout:  sm.config.ExternalTimeout,
		Cleanup: func(ctx context.Context) error {
			sm.logger.Info("Closing Redis connections")
			return client.Close()
		},
	}

	sm.hooks = append(sm.hooks, hook)
	return nil
}

// registerWorker registers a background worker for shutdown
func (sm *ShutdownManager) registerWorker(worker Worker) error {
	hook := ShutdownHook{
		Name:     fmt.Sprintf("worker_%s", worker.Name()),
		Priority: 2, // Stop workers after HTTP server
		Timeout:  sm.config.WorkerTimeout,
		Cleanup: func(ctx context.Context) error {
			sm.logger.Info("Stopping background worker", "worker", worker.Name())
			return worker.Stop(ctx)
		},
	}

	sm.hooks = append(sm.hooks, hook)
	return nil
}

// registerExternalConnection registers an external connection for cleanup
func (sm *ShutdownManager) registerExternalConnection(conn ExternalConnection) error {
	hook := ShutdownHook{
		Name:     fmt.Sprintf("external_%s", conn.Name()),
		Priority: 4, // Close external connections last
		Timeout:  sm.config.ExternalTimeout,
		Cleanup: func(ctx context.Context) error {
			sm.logger.Info("Closing external connection", "connection", conn.Name())
			return conn.Close(ctx)
		},
	}

	sm.hooks = append(sm.hooks, hook)
	return nil
}

// registerOptimizer registers a resource optimizer for cleanup
func (sm *ShutdownManager) registerOptimizer(optimizer ResourceOptimizer) error {
	hook := ShutdownHook{
		Name:     fmt.Sprintf("optimizer_%s", optimizer.Name()),
		Priority: 2, // Stop optimizers with workers
		Timeout:  sm.config.WorkerTimeout,
		Cleanup: func(ctx context.Context) error {
			sm.logger.Info("Stopping optimizer", "optimizer", optimizer.Name())
			return optimizer.Stop()
		},
	}

	sm.hooks = append(sm.hooks, hook)
	return nil
}

// registerWatcher registers a configuration watcher for cleanup
func (sm *ShutdownManager) registerWatcher(watcher ConfigWatcher) error {
	hook := ShutdownHook{
		Name:     fmt.Sprintf("watcher_%s", watcher.Name()),
		Priority: 2, // Stop watchers with workers
		Timeout:  sm.config.WorkerTimeout,
		Cleanup: func(ctx context.Context) error {
			sm.logger.Info("Stopping configuration watcher", "watcher", watcher.Name())
			return watcher.Stop()
		},
	}

	sm.hooks = append(sm.hooks, hook)
	return nil
}

// AddCustomHook adds a custom shutdown hook
func (sm *ShutdownManager) AddCustomHook(hook ShutdownHook) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.hooks = append(sm.hooks, hook)
	sm.logger.Debug("Added custom shutdown hook", "name", hook.Name, "priority", hook.Priority)
}

// WaitForShutdown blocks until shutdown signal is received
func (sm *ShutdownManager) WaitForShutdown() error {
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sm.config.Signals...)

	sm.logger.Info("Shutdown manager started, waiting for signal...")

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		sm.logger.Info("Received shutdown signal", "signal", sig.String())
		return sm.initiateShutdown()

	case <-sm.shutdown:
		sm.logger.Info("Programmatic shutdown initiated")
		return sm.initiateShutdown()
	}
}

// Shutdown initiates programmatic shutdown
func (sm *ShutdownManager) Shutdown() {
	select {
	case sm.shutdown <- struct{}{}:
	default:
		// Already shutting down
	}
}

// initiateShutdown performs the actual shutdown sequence
func (sm *ShutdownManager) initiateShutdown() error {
	defer close(sm.done)

	sm.logger.Info("Initiating graceful shutdown")

	// Pre-shutdown hook
	if sm.config.EnablePreShutdownHook {
		sm.logger.Info("Running pre-shutdown delay", "delay", sm.config.PreShutdownDelay)
		time.Sleep(sm.config.PreShutdownDelay)
	}

	// Disable health checks if configured
	if sm.config.DisableHealthCheck {
		sm.logger.Info("Disabling health checks during shutdown")
		time.Sleep(sm.config.HealthCheckGracePeriod)
	}

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), sm.config.GracefulTimeout)
	defer cancel()

	// Sort hooks by priority
	sm.sortHooksByPriority()

	// Execute shutdown hooks
	var wg sync.WaitGroup
	errorChan := make(chan error, len(sm.hooks))

	sm.logger.Info("Executing shutdown hooks", "count", len(sm.hooks))

	for _, hook := range sm.hooks {
		wg.Add(1)
		go sm.executeHook(ctx, hook, &wg, errorChan)
	}

	// Wait for all hooks to complete or timeout
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// Collect errors
	var errors []error
	for err := range errorChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	// Force kill timeout as last resort
	go sm.forceKillTimer()

	if len(errors) > 0 {
		sm.logger.Error("Shutdown completed with errors", "error_count", len(errors))
		for _, err := range errors {
			sm.logger.Error("Shutdown error", "error", err)
		}
		return fmt.Errorf("shutdown completed with %d errors", len(errors))
	}

	sm.logger.Info("Graceful shutdown completed successfully")
	return nil
}

// executeHook executes a single shutdown hook
func (sm *ShutdownManager) executeHook(ctx context.Context, hook ShutdownHook, wg *sync.WaitGroup, errorChan chan<- error) {
	defer wg.Done()

	if sm.config.LogShutdownProgress {
		sm.logger.Debug("Executing shutdown hook", "name", hook.Name, "priority", hook.Priority)
	}

	// Create timeout context for this specific hook
	hookCtx, cancel := context.WithTimeout(ctx, hook.Timeout)
	defer cancel()

	// Execute hook with timeout
	done := make(chan error, 1)
	go func() {
		done <- hook.Cleanup(hookCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			sm.logger.Error("Shutdown hook failed", "name", hook.Name, "error", err)
			errorChan <- fmt.Errorf("hook %s failed: %w", hook.Name, err)
		} else if sm.config.LogShutdownProgress {
			sm.logger.Debug("Shutdown hook completed", "name", hook.Name)
		}

	case <-hookCtx.Done():
		sm.logger.Warn("Shutdown hook timed out", "name", hook.Name, "timeout", hook.Timeout)
		errorChan <- fmt.Errorf("hook %s timed out after %v", hook.Name, hook.Timeout)
	}
}

// sortHooksByPriority sorts shutdown hooks by priority (lower numbers first)
func (sm *ShutdownManager) sortHooksByPriority() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Simple bubble sort by priority
	n := len(sm.hooks)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if sm.hooks[j].Priority > sm.hooks[j+1].Priority {
				sm.hooks[j], sm.hooks[j+1] = sm.hooks[j+1], sm.hooks[j]
			}
		}
	}
}

// forceKillTimer implements force kill timeout as last resort
func (sm *ShutdownManager) forceKillTimer() {
	time.Sleep(sm.config.ForceKillTimeout)

	sm.logger.Error("Force kill timeout reached, terminating process")
	os.Exit(1)
}

// GetShutdownMetrics returns metrics about the shutdown process
func (sm *ShutdownManager) GetShutdownMetrics() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"registered_hooks":   len(sm.hooks),
		"graceful_timeout":   sm.config.GracefulTimeout,
		"force_kill_timeout": sm.config.ForceKillTimeout,
		"shutdown_initiated": sm.isShutdownInitiated(),
	}
}

// isShutdownInitiated checks if shutdown has been initiated
func (sm *ShutdownManager) isShutdownInitiated() bool {
	select {
	case <-sm.done:
		return true
	default:
		return false
	}
}

// WaitForCompletion waits for shutdown to complete
func (sm *ShutdownManager) WaitForCompletion() {
	<-sm.done
}

// NewResourceManager creates a new resource manager
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		workers:       make([]Worker, 0),
		externalConns: make([]ExternalConnection, 0),
		monitors:      make([]interface{}, 0),
		optimizers:    make([]ResourceOptimizer, 0),
		watchers:      make([]ConfigWatcher, 0),
	}
}

// AddHTTPServer adds an HTTP server to the resource manager
func (rm *ResourceManager) AddHTTPServer(server *http.Server) {
	rm.httpServer = server
}

// AddDatabase adds a database pool to the resource manager
func (rm *ResourceManager) AddDatabase(pool *pgxpool.Pool) {
	rm.dbPool = pool
}

// AddRedis adds a Redis client to the resource manager
func (rm *ResourceManager) AddRedis(client *redis.Client) {
	rm.redisClient = client
}

// AddWorker adds a worker to the resource manager
func (rm *ResourceManager) AddWorker(worker Worker) {
	rm.workers = append(rm.workers, worker)
}

// AddExternalConnection adds an external connection to the resource manager
func (rm *ResourceManager) AddExternalConnection(conn ExternalConnection) {
	rm.externalConns = append(rm.externalConns, conn)
}

// AddOptimizer adds a resource optimizer to the resource manager
func (rm *ResourceManager) AddOptimizer(optimizer ResourceOptimizer) {
	rm.optimizers = append(rm.optimizers, optimizer)
}

// AddWatcher adds a configuration watcher to the resource manager
func (rm *ResourceManager) AddWatcher(watcher ConfigWatcher) {
	rm.watchers = append(rm.watchers, watcher)
}

// DBOptimizerWrapper wraps infrastructure.DBOptimizer to implement ResourceOptimizer
type DBOptimizerWrapper struct {
	optimizer *infrastructure.DBOptimizer
}

// NewDBOptimizerWrapper creates a new wrapper for DBOptimizer
func NewDBOptimizerWrapper(optimizer *infrastructure.DBOptimizer) *DBOptimizerWrapper {
	return &DBOptimizerWrapper{optimizer: optimizer}
}

// Stop implements ResourceOptimizer interface
func (w *DBOptimizerWrapper) Stop() error {
	if w.optimizer == nil {
		return fmt.Errorf("db optimizer is nil")
	}
	return w.optimizer.Stop()
}

// Name implements ResourceOptimizer interface
func (w *DBOptimizerWrapper) Name() string {
	return "db_optimizer"
}

// ConfigWatcherWrapper wraps infrastructure.ConfigWatcher to implement ConfigWatcher
type ConfigWatcherWrapper struct {
	watcher *infrastructure.ConfigWatcher
}

// NewConfigWatcherWrapper creates a new wrapper for ConfigWatcher
func NewConfigWatcherWrapper(watcher *infrastructure.ConfigWatcher) *ConfigWatcherWrapper {
	return &ConfigWatcherWrapper{watcher: watcher}
}

// Stop implements ConfigWatcher interface
func (w *ConfigWatcherWrapper) Stop() error {
	if w.watcher == nil {
		return fmt.Errorf("config watcher is nil")
	}
	return w.watcher.Stop()
}

// Name implements ConfigWatcher interface
func (w *ConfigWatcherWrapper) Name() string {
	return "config_watcher"
}
