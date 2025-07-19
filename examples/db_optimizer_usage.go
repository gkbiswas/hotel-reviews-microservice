package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// Example logger implementation
type exampleLogger struct{}

func (l *exampleLogger) Debug(msg string, fields ...interface{}) {
	fmt.Printf("DEBUG: %s %v\n", msg, fields)
}

func (l *exampleLogger) Info(msg string, fields ...interface{}) {
	fmt.Printf("INFO: %s %v\n", msg, fields)
}

func (l *exampleLogger) Warn(msg string, fields ...interface{}) {
	fmt.Printf("WARN: %s %v\n", msg, fields)
}

func (l *exampleLogger) Error(msg string, fields ...interface{}) {
	fmt.Printf("ERROR: %s %v\n", msg, fields)
}

// DBWrapper wraps a database connection with optimization tracking
type DBWrapper struct {
	pool      *pgxpool.Pool
	optimizer *infrastructure.DBOptimizer
}

// NewDBWrapper creates a new database wrapper with optimization
func NewDBWrapper(databaseURL string, logger monitoring.Logger) (*DBWrapper, error) {
	// Create connection pool
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Optimize pool configuration
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create database optimizer
	optimizerConfig := infrastructure.DefaultDBOptimizerConfig()
	optimizerConfig.SlowQueryThreshold = 200 * time.Millisecond
	optimizerConfig.HealthCheckInterval = 30 * time.Second

	optimizer, err := infrastructure.NewDBOptimizer(pool, logger, optimizerConfig)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create database optimizer: %w", err)
	}

	return &DBWrapper{
		pool:      pool,
		optimizer: optimizer,
	}, nil
}

// ExecWithTracking executes a query with performance tracking
func (db *DBWrapper) ExecWithTracking(ctx context.Context, query string, args ...interface{}) error {
	start := time.Now()
	
	result, err := db.pool.Exec(ctx, query, args...)
	duration := time.Since(start)
	
	var rowsAffected int64
	if result != nil {
		rowsAffected = result.RowsAffected()
	}
	
	// Track the query performance
	db.optimizer.TrackQuery(query, duration, err, rowsAffected)
	
	return err
}

// QueryWithTracking executes a query with performance tracking
func (db *DBWrapper) QueryWithTracking(ctx context.Context, query string, args ...interface{}) error {
	start := time.Now()
	
	rows, err := db.pool.Query(ctx, query, args...)
	if rows != nil {
		rows.Close()
	}
	
	duration := time.Since(start)
	
	// For SELECT queries, we can't easily get rows affected, so we use 0
	db.optimizer.TrackQuery(query, duration, err, 0)
	
	return err
}

// MigrationRunner demonstrates migration tracking
type MigrationRunner struct {
	db       *DBWrapper
	logger   monitoring.Logger
}

func NewMigrationRunner(db *DBWrapper, logger monitoring.Logger) *MigrationRunner {
	return &MigrationRunner{
		db:     db,
		logger: logger,
	}
}

func (mr *MigrationRunner) RunMigration(version, name, sql string) error {
	startTime := time.Now()
	
	mr.logger.Info("Starting migration", "version", version, "name", name)
	
	err := mr.db.ExecWithTracking(context.Background(), sql)
	
	endTime := time.Now()
	success := err == nil
	
	// Track migration performance
	mr.db.optimizer.TrackMigration(version, name, startTime, endTime, success, err, 1)
	
	if err != nil {
		mr.logger.Error("Migration failed", "version", version, "error", err)
		return err
	}
	
	mr.logger.Info("Migration completed", "version", version, "duration", endTime.Sub(startTime))
	return nil
}

// PerformanceMonitor demonstrates how to monitor database performance
type PerformanceMonitor struct {
	optimizer *infrastructure.DBOptimizer
	logger    monitoring.Logger
}

func NewPerformanceMonitor(optimizer *infrastructure.DBOptimizer, logger monitoring.Logger) *PerformanceMonitor {
	return &PerformanceMonitor{
		optimizer: optimizer,
		logger:    logger,
	}
}

func (pm *PerformanceMonitor) PrintPerformanceReport() {
	fmt.Println("\n=== Database Performance Report ===")
	
	// Get query statistics summary
	summary := pm.optimizer.GetQueryStatsSummary()
	fmt.Printf("Total Queries: %v\n", summary["total_queries"])
	fmt.Printf("Average Duration: %v\n", summary["average_duration"])
	fmt.Printf("Error Rate: %.2f%%\n", summary["error_rate"])
	fmt.Printf("Slow Queries: %v\n", summary["slow_queries"])
	fmt.Printf("Unique Queries: %v\n", summary["unique_queries"])
	
	// Get database health
	health := pm.optimizer.GetDatabaseHealth()
	if health != nil {
		fmt.Printf("\n=== Database Health ===\n")
		fmt.Printf("Healthy: %v\n", health.IsHealthy)
		fmt.Printf("Response Time: %v\n", health.ResponseTime)
		fmt.Printf("Active Connections: %d\n", health.ActiveConnections)
		fmt.Printf("Idle Connections: %d\n", health.IdleConnections)
		fmt.Printf("Max Connections: %d\n", health.MaxConnections)
		if health.CacheHitRatio > 0 {
			fmt.Printf("Cache Hit Ratio: %.2f%%\n", health.CacheHitRatio)
		}
	}
	
	// Get slow queries
	slowQueries := pm.optimizer.GetTopSlowQueries(5)
	if len(slowQueries) > 0 {
		fmt.Printf("\n=== Top Slow Queries ===\n")
		for i, query := range slowQueries {
			fmt.Printf("%d. Duration: %v\n", i+1, query.Duration)
			fmt.Printf("   Query: %s\n", query.Query)
			if query.Error != "" {
				fmt.Printf("   Error: %s\n", query.Error)
			}
			fmt.Println()
		}
	}
	
	// Get migration history
	migrations := pm.optimizer.GetMigrationHistory()
	if len(migrations) > 0 {
		fmt.Printf("\n=== Recent Migrations ===\n")
		for _, migration := range migrations {
			status := "SUCCESS"
			if !migration.Success {
				status = "FAILED"
			}
			fmt.Printf("%s - %s (%v) [%s]\n", 
				migration.Version, 
				migration.Name, 
				migration.Duration, 
				status)
		}
	}
	
	// Get optimization suggestions
	suggestions := pm.optimizer.GetPoolOptimizationSuggestions()
	if len(suggestions) > 0 {
		fmt.Printf("\n=== Optimization Suggestions ===\n")
		for _, suggestion := range suggestions {
			fmt.Printf("Parameter: %s\n", suggestion.Parameter)
			fmt.Printf("Current: %v\n", suggestion.Current)
			fmt.Printf("Suggested: %v\n", suggestion.Suggested)
			fmt.Printf("Reason: %s\n", suggestion.Reason)
			fmt.Printf("Confidence: %.1f\n", suggestion.Confidence)
			fmt.Println()
		}
	}
}

func (pm *PerformanceMonitor) StartPeriodicReporting(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			pm.PrintPerformanceReport()
		}
	}()
}

func main() {
	logger := &exampleLogger{}
	
	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		// Use a default for example purposes
		databaseURL = "postgres://user:password@localhost:5432/hotel_reviews?sslmode=disable"
		fmt.Println("Using default database URL (set DATABASE_URL environment variable)")
	}
	
	// Create database wrapper with optimization
	db, err := NewDBWrapper(databaseURL, logger)
	if err != nil {
		log.Fatalf("Failed to create database wrapper: %v", err)
	}
	defer db.pool.Close()
	defer db.optimizer.Stop()
	
	fmt.Println("Database optimizer example started...")
	
	// Create performance monitor
	monitor := NewPerformanceMonitor(db.optimizer, logger)
	
	// Example: Run some database operations
	ctx := context.Background()
	
	// Simulate various database operations
	queries := []string{
		"SELECT COUNT(*) FROM hotels",
		"SELECT * FROM hotels WHERE city = $1",
		"INSERT INTO hotels (name, city, rating) VALUES ($1, $2, $3)",
		"UPDATE hotels SET rating = $1 WHERE id = $2",
		"DELETE FROM hotels WHERE id = $1",
		// Simulate a slow query
		"SELECT h.*, COUNT(r.id) as review_count FROM hotels h LEFT JOIN reviews r ON h.id = r.hotel_id GROUP BY h.id ORDER BY review_count DESC",
	}
	
	// Execute queries with tracking
	for i, query := range queries {
		if err := db.ExecWithTracking(ctx, query, "Test City", 4.5, i+1); err != nil {
			logger.Warn("Query execution failed", "query", query, "error", err)
		}
		
		// Add some delay to simulate real usage
		time.Sleep(100 * time.Millisecond)
	}
	
	// Example: Run a migration
	migrationRunner := NewMigrationRunner(db, logger)
	err = migrationRunner.RunMigration(
		"001", 
		"create_example_table", 
		"CREATE TABLE IF NOT EXISTS example (id SERIAL PRIMARY KEY, name TEXT)",
	)
	if err != nil {
		logger.Error("Migration failed", "error", err)
	}
	
	// Wait a bit for health checks to run
	time.Sleep(2 * time.Second)
	
	// Print performance report
	monitor.PrintPerformanceReport()
	
	// Start periodic reporting (in a real application)
	// monitor.StartPeriodicReporting(5 * time.Minute)
	
	fmt.Println("\nExample completed. In a real application, the optimizer would continue monitoring in the background.")
}