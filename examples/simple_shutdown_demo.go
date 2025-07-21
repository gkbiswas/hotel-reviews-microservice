//go:build examples
// +build examples

package main
import (
	"context"
	"fmt"
	"net/http"
	"time"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/server"
)
// Simple logger implementation
type simpleLogger struct{}
func (l *simpleLogger) Debug(msg string, fields ...interface{}) {
	fmt.Printf("DEBUG: %s %v\n", msg, fields)
}
func (l *simpleLogger) Info(msg string, fields ...interface{}) {
	fmt.Printf("INFO: %s %v\n", msg, fields)
}
func (l *simpleLogger) Warn(msg string, fields ...interface{}) {
	fmt.Printf("WARN: %s %v\n", msg, fields)
}
func (l *simpleLogger) Error(msg string, fields ...interface{}) {
	fmt.Printf("ERROR: %s %v\n", msg, fields)
}
// Simple worker implementation
type simpleWorker struct {
	name   string
	logger monitoring.Logger
	quit   chan struct{}
	done   chan struct{}
}
func newSimpleWorker(name string, logger monitoring.Logger) *simpleWorker {
	w := &simpleWorker{
		name:   name,
		logger: logger,
		quit:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go w.run()
	return w
}
func (w *simpleWorker) run() {
	defer close(w.done)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.quit:
			w.logger.Info("Worker stopping", "name", w.name)
			return
		case <-ticker.C:
			w.logger.Debug("Worker tick", "name", w.name)
		}
	}
}
func (w *simpleWorker) Stop(ctx context.Context) error {
	w.logger.Info("Stopping worker", "name", w.name)
	close(w.quit)
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (w *simpleWorker) Name() string {
	return w.name
}
func main() {
	logger := &simpleLogger{}
	logger.Info("Starting simple shutdown demo...")
	// Create shutdown manager
	config := server.DefaultShutdownConfig()
	config.GracefulTimeout = 10 * time.Second
	config.PreShutdownDelay = 1 * time.Second
	shutdownManager := server.NewShutdownManager(logger, config)
	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from shutdown demo!"))
	})
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	// Register resources
	err := shutdownManager.RegisterResource(httpServer)
	if err != nil {
		logger.Error("Failed to register HTTP server", "error", err)
		return
	}
	worker1 := newSimpleWorker("worker1", logger)
	err = shutdownManager.RegisterResource(worker1)
	if err != nil {
		logger.Error("Failed to register worker1", "error", err)
		return
	}
	worker2 := newSimpleWorker("worker2", logger)
	err = shutdownManager.RegisterResource(worker2)
	if err != nil {
		logger.Error("Failed to register worker2", "error", err)
		return
	}
	// Start HTTP server
	go func() {
		logger.Info("Starting HTTP server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()
	// Give server time to start
	time.Sleep(500 * time.Millisecond)
	logger.Info("Server started. Send SIGINT (Ctrl+C) to test graceful shutdown")
	// For demonstration, automatically shutdown after 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		logger.Info("Triggering automatic shutdown for demo...")
		shutdownManager.Shutdown()
	}()
	// Wait for shutdown
	if err := shutdownManager.WaitForShutdown(); err != nil {
		logger.Error("Shutdown completed with errors", "error", err)
	} else {
		logger.Info("Graceful shutdown completed successfully")
	}
	logger.Info("Demo completed")
}
