package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// DBOptimizer provides database optimization utilities and monitoring
type DBOptimizer struct {
	pool   *pgxpool.Pool
	logger monitoring.Logger
	config *DBOptimizerConfig
	
	// Query performance tracking
	queryStats     map[string]*QueryStats
	queryStatsMux  sync.RWMutex
	
	// Slow query tracking
	slowQueries    []SlowQuery
	slowQueriesMux sync.RWMutex
	
	// Migration tracking
	migrationHistory []MigrationRecord
	migrationMux     sync.RWMutex
	
	// Health monitoring
	lastHealthCheck time.Time
	healthStatus    *DatabaseHealth
	healthMux       sync.RWMutex
	
	// Metrics
	metrics *DBMetrics
	
	// Background monitoring
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// DBOptimizerConfig configures the database optimizer
type DBOptimizerConfig struct {
	// Query monitoring
	SlowQueryThreshold    time.Duration `json:"slow_query_threshold" validate:"min=1ms"`
	MaxSlowQueries        int           `json:"max_slow_queries" validate:"min=1"`
	QueryStatsRetention   time.Duration `json:"query_stats_retention" validate:"min=1h"`
	
	// Health monitoring
	HealthCheckInterval   time.Duration `json:"health_check_interval" validate:"min=10s"`
	ConnectionTimeout     time.Duration `json:"connection_timeout" validate:"min=1s"`
	
	// Performance monitoring
	MetricsInterval       time.Duration `json:"metrics_interval" validate:"min=10s"`
	EnableQueryLogging    bool          `json:"enable_query_logging"`
	EnableSlowQueryLog    bool          `json:"enable_slow_query_log"`
	
	// Connection pool optimization
	EnablePoolOptimization bool `json:"enable_pool_optimization"`
	PoolOptimizationInterval time.Duration `json:"pool_optimization_interval" validate:"min=1m"`
	
	// Migration tracking
	EnableMigrationTracking bool `json:"enable_migration_tracking"`
}

// QueryStats tracks performance statistics for database queries
type QueryStats struct {
	Query          string        `json:"query"`
	ExecutionCount int64         `json:"execution_count"`
	TotalDuration  time.Duration `json:"total_duration"`
	MinDuration    time.Duration `json:"min_duration"`
	MaxDuration    time.Duration `json:"max_duration"`
	AvgDuration    time.Duration `json:"avg_duration"`
	LastExecuted   time.Time     `json:"last_executed"`
	ErrorCount     int64         `json:"error_count"`
	RowsAffected   int64         `json:"rows_affected"`
}

// SlowQuery represents a slow query entry
type SlowQuery struct {
	Query     string        `json:"query"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Error     string        `json:"error,omitempty"`
	Context   string        `json:"context,omitempty"`
}

// MigrationRecord tracks migration performance
type MigrationRecord struct {
	Version     string        `json:"version"`
	Name        string        `json:"name"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	RowsAffected int64        `json:"rows_affected"`
}

// DatabaseHealth represents the current health status of the database
type DatabaseHealth struct {
	IsHealthy          bool          `json:"is_healthy"`
	LastCheck          time.Time     `json:"last_check"`
	ResponseTime       time.Duration `json:"response_time"`
	ActiveConnections  int32         `json:"active_connections"`
	IdleConnections    int32         `json:"idle_connections"`
	MaxConnections     int32         `json:"max_connections"`
	TotalConnections   int32         `json:"total_connections"`
	WaitingClients     int32         `json:"waiting_clients"`
	DatabaseSize       int64         `json:"database_size_bytes"`
	TransactionsPerSec float64       `json:"transactions_per_sec"`
	QueriesPerSec      float64       `json:"queries_per_sec"`
	CacheHitRatio      float64       `json:"cache_hit_ratio"`
}

// PoolOptimizationSuggestion represents a suggestion for pool optimization
type PoolOptimizationSuggestion struct {
	Parameter   string      `json:"parameter"`
	Current     interface{} `json:"current"`
	Suggested   interface{} `json:"suggested"`
	Reason      string      `json:"reason"`
	Impact      string      `json:"impact"`
	Confidence  float64     `json:"confidence"`
}

// DBMetrics contains Prometheus metrics for database monitoring
type DBMetrics struct {
	QueryDuration    *prometheus.HistogramVec
	QueryCount       *prometheus.CounterVec
	SlowQueryCount   prometheus.Counter
	ConnectionsGauge *prometheus.GaugeVec
	HealthGauge      prometheus.Gauge
	MigrationDuration *prometheus.HistogramVec
}

// DefaultDBOptimizerConfig returns the default configuration
func DefaultDBOptimizerConfig() *DBOptimizerConfig {
	return &DBOptimizerConfig{
		SlowQueryThreshold:       500 * time.Millisecond,
		MaxSlowQueries:           100,
		QueryStatsRetention:      24 * time.Hour,
		HealthCheckInterval:      30 * time.Second,
		ConnectionTimeout:        5 * time.Second,
		MetricsInterval:          15 * time.Second,
		EnableQueryLogging:       true,
		EnableSlowQueryLog:       true,
		EnablePoolOptimization:   true,
		PoolOptimizationInterval: 5 * time.Minute,
		EnableMigrationTracking:  true,
	}
}

// NewDBOptimizer creates a new database optimizer
func NewDBOptimizer(pool *pgxpool.Pool, logger monitoring.Logger, config *DBOptimizerConfig) (*DBOptimizer, error) {
	if config == nil {
		config = DefaultDBOptimizerConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	optimizer := &DBOptimizer{
		pool:             pool,
		logger:           logger,
		config:           config,
		queryStats:       make(map[string]*QueryStats),
		slowQueries:      make([]SlowQuery, 0, config.MaxSlowQueries),
		migrationHistory: make([]MigrationRecord, 0),
		ctx:              ctx,
		cancel:           cancel,
		metrics:          createDBMetrics(),
	}
	
	// Start background monitoring
	optimizer.wg.Add(1)
	go optimizer.backgroundMonitor()
	
	if config.EnablePoolOptimization {
		optimizer.wg.Add(1)
		go optimizer.poolOptimizer()
	}
	
	logger.Info("Database optimizer started", 
		"slow_query_threshold", config.SlowQueryThreshold,
		"health_check_interval", config.HealthCheckInterval)
	
	return optimizer, nil
}

// createDBMetrics initializes Prometheus metrics
func createDBMetrics() *DBMetrics {
	// Create metrics manually to avoid registration conflicts in tests
	queryDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query execution duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~32s
		},
		[]string{"query_type", "table", "operation"},
	)
	
	queryCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_query_total",
			Help: "Total number of database queries executed",
		},
		[]string{"query_type", "table", "operation", "status"},
	)
	
	slowQueryCount := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "db_slow_query_total",
			Help: "Total number of slow database queries",
		},
	)
	
	connectionsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections",
			Help: "Number of database connections by state",
		},
		[]string{"state"},
	)
	
	healthGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_health_status",
			Help: "Database health status (1 = healthy, 0 = unhealthy)",
		},
	)
	
	migrationDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_migration_duration_seconds",
			Help:    "Database migration execution duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 12), // 100ms to ~6m
		},
		[]string{"version", "operation"},
	)
	
	return &DBMetrics{
		QueryDuration:     queryDuration,
		QueryCount:        queryCount,
		SlowQueryCount:    slowQueryCount,
		ConnectionsGauge:  connectionsGauge,
		HealthGauge:       healthGauge,
		MigrationDuration: migrationDuration,
	}
}

// TrackQuery records query execution statistics
func (opt *DBOptimizer) TrackQuery(query string, duration time.Duration, err error, rowsAffected int64) {
	// Normalize query for statistics (remove specific values)
	normalizedQuery := opt.normalizeQuery(query)
	
	opt.queryStatsMux.Lock()
	stats, exists := opt.queryStats[normalizedQuery]
	if !exists {
		stats = &QueryStats{
			Query:       normalizedQuery,
			MinDuration: duration,
			MaxDuration: duration,
		}
		opt.queryStats[normalizedQuery] = stats
	}
	
	stats.ExecutionCount++
	stats.TotalDuration += duration
	stats.LastExecuted = time.Now()
	stats.RowsAffected += rowsAffected
	
	if duration < stats.MinDuration {
		stats.MinDuration = duration
	}
	if duration > stats.MaxDuration {
		stats.MaxDuration = duration
	}
	stats.AvgDuration = time.Duration(int64(stats.TotalDuration) / stats.ExecutionCount)
	
	if err != nil {
		stats.ErrorCount++
	}
	opt.queryStatsMux.Unlock()
	
	// Track slow queries
	if duration >= opt.config.SlowQueryThreshold {
		opt.trackSlowQuery(query, duration, err)
	}
	
	// Update metrics
	queryType, table, operation := opt.parseQuery(query)
	opt.metrics.QueryDuration.WithLabelValues(queryType, table, operation).Observe(duration.Seconds())
	
	status := "success"
	if err != nil {
		status = "error"
	}
	opt.metrics.QueryCount.WithLabelValues(queryType, table, operation, status).Inc()
	
	if opt.config.EnableQueryLogging {
		logLevel := "debug"
		if duration >= opt.config.SlowQueryThreshold {
			logLevel = "warn"
		}
		
		if logLevel == "warn" {
			opt.logger.Warn("Slow query detected",
				"query", query,
				"duration", duration,
				"rows_affected", rowsAffected,
				"error", err)
		} else {
			opt.logger.Debug("Query executed",
				"duration", duration,
				"rows_affected", rowsAffected)
		}
	}
}

// trackSlowQuery records a slow query
func (opt *DBOptimizer) trackSlowQuery(query string, duration time.Duration, err error) {
	opt.metrics.SlowQueryCount.Inc()
	
	if !opt.config.EnableSlowQueryLog {
		return
	}
	
	slowQuery := SlowQuery{
		Query:     query,
		Duration:  duration,
		Timestamp: time.Now(),
	}
	
	if err != nil {
		slowQuery.Error = err.Error()
	}
	
	opt.slowQueriesMux.Lock()
	opt.slowQueries = append(opt.slowQueries, slowQuery)
	
	// Keep only the most recent slow queries
	if len(opt.slowQueries) > opt.config.MaxSlowQueries {
		opt.slowQueries = opt.slowQueries[1:]
	}
	opt.slowQueriesMux.Unlock()
}

// TrackMigration records migration performance
func (opt *DBOptimizer) TrackMigration(version, name string, startTime, endTime time.Time, success bool, err error, rowsAffected int64) {
	if !opt.config.EnableMigrationTracking {
		return
	}
	
	duration := endTime.Sub(startTime)
	
	migration := MigrationRecord{
		Version:      version,
		Name:         name,
		StartTime:    startTime,
		EndTime:      endTime,
		Duration:     duration,
		Success:      success,
		RowsAffected: rowsAffected,
	}
	
	if err != nil {
		migration.Error = err.Error()
	}
	
	opt.migrationMux.Lock()
	opt.migrationHistory = append(opt.migrationHistory, migration)
	opt.migrationMux.Unlock()
	
	// Update metrics
	operation := "up"
	if !success {
		operation = "failed"
	}
	opt.metrics.MigrationDuration.WithLabelValues(version, operation).Observe(duration.Seconds())
	
	opt.logger.Info("Migration tracked",
		"version", version,
		"name", name,
		"duration", duration,
		"success", success,
		"rows_affected", rowsAffected,
		"error", err)
}

// GetQueryStats returns current query statistics
func (opt *DBOptimizer) GetQueryStats() map[string]*QueryStats {
	opt.queryStatsMux.RLock()
	defer opt.queryStatsMux.RUnlock()
	
	// Return a copy to prevent concurrent access issues
	result := make(map[string]*QueryStats)
	for k, v := range opt.queryStats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	
	return result
}

// GetSlowQueries returns recent slow queries
func (opt *DBOptimizer) GetSlowQueries() []SlowQuery {
	opt.slowQueriesMux.RLock()
	defer opt.slowQueriesMux.RUnlock()
	
	// Return a copy
	result := make([]SlowQuery, len(opt.slowQueries))
	copy(result, opt.slowQueries)
	
	return result
}

// GetMigrationHistory returns migration history
func (opt *DBOptimizer) GetMigrationHistory() []MigrationRecord {
	opt.migrationMux.RLock()
	defer opt.migrationMux.RUnlock()
	
	// Return a copy
	result := make([]MigrationRecord, len(opt.migrationHistory))
	copy(result, opt.migrationHistory)
	
	return result
}

// GetDatabaseHealth returns current database health status
func (opt *DBOptimizer) GetDatabaseHealth() *DatabaseHealth {
	opt.healthMux.RLock()
	defer opt.healthMux.RUnlock()
	
	if opt.healthStatus == nil {
		return nil
	}
	
	// Return a copy
	health := *opt.healthStatus
	return &health
}

// PerformHealthCheck executes a comprehensive database health check
func (opt *DBOptimizer) PerformHealthCheck() error {
	start := time.Now()
	
	ctx, cancel := context.WithTimeout(opt.ctx, opt.config.ConnectionTimeout)
	defer cancel()
	
	health := &DatabaseHealth{
		LastCheck: start,
	}
	
	// Test basic connectivity
	err := opt.pool.Ping(ctx)
	if err != nil {
		health.IsHealthy = false
		opt.updateHealthStatus(health)
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	health.ResponseTime = time.Since(start)
	
	// Get connection pool stats
	poolStats := opt.pool.Stat()
	health.ActiveConnections = poolStats.AcquiredConns()
	health.IdleConnections = poolStats.IdleConns()
	health.MaxConnections = poolStats.MaxConns()
	health.TotalConnections = poolStats.TotalConns()
	
	// Get database statistics
	if err := opt.collectDatabaseStats(ctx, health); err != nil {
		opt.logger.Warn("Failed to collect database statistics", "error", err)
	}
	
	health.IsHealthy = true
	opt.updateHealthStatus(health)
	
	// Update metrics
	opt.metrics.HealthGauge.Set(1)
	opt.metrics.ConnectionsGauge.WithLabelValues("active").Set(float64(health.ActiveConnections))
	opt.metrics.ConnectionsGauge.WithLabelValues("idle").Set(float64(health.IdleConnections))
	opt.metrics.ConnectionsGauge.WithLabelValues("max").Set(float64(health.MaxConnections))
	
	return nil
}

// collectDatabaseStats gathers detailed database statistics
func (opt *DBOptimizer) collectDatabaseStats(ctx context.Context, health *DatabaseHealth) error {
	// Database size
	var dbSize sql.NullInt64
	err := opt.pool.QueryRow(ctx, `
		SELECT pg_database_size(current_database())
	`).Scan(&dbSize)
	if err == nil && dbSize.Valid {
		health.DatabaseSize = dbSize.Int64
	}
	
	// Cache hit ratio
	var cacheHit, cacheRead sql.NullFloat64
	err = opt.pool.QueryRow(ctx, `
		SELECT 
			sum(heap_blks_hit) as cache_hit,
			sum(heap_blks_hit + heap_blks_read) as cache_read
		FROM pg_statio_user_tables
	`).Scan(&cacheHit, &cacheRead)
	if err == nil && cacheHit.Valid && cacheRead.Valid && cacheRead.Float64 > 0 {
		health.CacheHitRatio = cacheHit.Float64 / cacheRead.Float64 * 100
	}
	
	// Transaction and query rates (approximate)
	var xactCommit, xactRollback sql.NullInt64
	err = opt.pool.QueryRow(ctx, `
		SELECT xact_commit, xact_rollback 
		FROM pg_stat_database 
		WHERE datname = current_database()
	`).Scan(&xactCommit, &xactRollback)
	if err == nil && xactCommit.Valid {
		// This is cumulative, so we'd need to track over time for true rate
		health.TransactionsPerSec = float64(xactCommit.Int64 + xactRollback.Int64)
	}
	
	return nil
}

// updateHealthStatus updates the health status thread-safely
func (opt *DBOptimizer) updateHealthStatus(health *DatabaseHealth) {
	opt.healthMux.Lock()
	opt.healthStatus = health
	opt.lastHealthCheck = health.LastCheck
	opt.healthMux.Unlock()
}

// GetPoolOptimizationSuggestions analyzes pool usage and provides optimization suggestions
func (opt *DBOptimizer) GetPoolOptimizationSuggestions() []PoolOptimizationSuggestion {
	if !opt.config.EnablePoolOptimization || opt.pool == nil {
		return nil
	}
	
	poolStats := opt.pool.Stat()
	health := opt.GetDatabaseHealth()
	
	var suggestions []PoolOptimizationSuggestion
	
	// Analyze connection pool utilization
	utilizationRatio := float64(poolStats.AcquiredConns()) / float64(poolStats.MaxConns())
	
	if utilizationRatio > 0.9 {
		suggestions = append(suggestions, PoolOptimizationSuggestion{
			Parameter:  "max_connections",
			Current:    poolStats.MaxConns(),
			Suggested:  int32(float64(poolStats.MaxConns()) * 1.5),
			Reason:     "High connection pool utilization detected",
			Impact:     "Increased throughput under high load",
			Confidence: 0.8,
		})
	}
	
	if utilizationRatio < 0.3 && poolStats.MaxConns() > 10 {
		suggestions = append(suggestions, PoolOptimizationSuggestion{
			Parameter:  "max_connections",
			Current:    poolStats.MaxConns(),
			Suggested:  int32(float64(poolStats.MaxConns()) * 0.7),
			Reason:     "Low connection pool utilization detected",
			Impact:     "Reduced memory usage and overhead",
			Confidence: 0.6,
		})
	}
	
	// Analyze slow queries
	slowQueries := opt.GetSlowQueries()
	if len(slowQueries) > 10 {
		suggestions = append(suggestions, PoolOptimizationSuggestion{
			Parameter:  "query_optimization",
			Current:    len(slowQueries),
			Suggested:  "Index analysis recommended",
			Reason:     "High number of slow queries detected",
			Impact:     "Improved query performance",
			Confidence: 0.9,
		})
	}
	
	// Analyze cache hit ratio
	if health != nil && health.CacheHitRatio < 95 {
		suggestions = append(suggestions, PoolOptimizationSuggestion{
			Parameter:  "shared_buffers",
			Current:    fmt.Sprintf("%.2f%% cache hit ratio", health.CacheHitRatio),
			Suggested:  "Increase shared_buffers",
			Reason:     "Low cache hit ratio indicates insufficient memory allocation",
			Impact:     "Reduced disk I/O and improved performance",
			Confidence: 0.7,
		})
	}
	
	return suggestions
}

// backgroundMonitor runs periodic health checks and maintenance
func (opt *DBOptimizer) backgroundMonitor() {
	defer opt.wg.Done()
	
	healthTicker := time.NewTicker(opt.config.HealthCheckInterval)
	metricsTicker := time.NewTicker(opt.config.MetricsInterval)
	cleanupTicker := time.NewTicker(opt.config.QueryStatsRetention)
	
	defer healthTicker.Stop()
	defer metricsTicker.Stop()
	defer cleanupTicker.Stop()
	
	for {
		select {
		case <-opt.ctx.Done():
			return
			
		case <-healthTicker.C:
			if err := opt.PerformHealthCheck(); err != nil {
				opt.logger.Error("Health check failed", "error", err)
				opt.metrics.HealthGauge.Set(0)
			}
			
		case <-metricsTicker.C:
			opt.updateMetrics()
			
		case <-cleanupTicker.C:
			opt.cleanupOldStats()
		}
	}
}

// poolOptimizer runs periodic pool optimization analysis
func (opt *DBOptimizer) poolOptimizer() {
	defer opt.wg.Done()
	
	ticker := time.NewTicker(opt.config.PoolOptimizationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-opt.ctx.Done():
			return
			
		case <-ticker.C:
			suggestions := opt.GetPoolOptimizationSuggestions()
			if len(suggestions) > 0 {
				opt.logger.Info("Pool optimization suggestions available",
					"count", len(suggestions))
				
				for _, suggestion := range suggestions {
					opt.logger.Info("Pool optimization suggestion",
						"parameter", suggestion.Parameter,
						"current", suggestion.Current,
						"suggested", suggestion.Suggested,
						"reason", suggestion.Reason,
						"confidence", suggestion.Confidence)
				}
			}
		}
	}
}

// updateMetrics updates Prometheus metrics
func (opt *DBOptimizer) updateMetrics() {
	// Update query statistics metrics
	stats := opt.GetQueryStats()
	_ = stats // Query stats are already tracked in TrackQuery
	// Additional metrics could be exposed here based on query stats
}

// cleanupOldStats removes old query statistics
func (opt *DBOptimizer) cleanupOldStats() {
	cutoff := time.Now().Add(-opt.config.QueryStatsRetention)
	
	opt.queryStatsMux.Lock()
	for query, stats := range opt.queryStats {
		if stats.LastExecuted.Before(cutoff) {
			delete(opt.queryStats, query)
		}
	}
	opt.queryStatsMux.Unlock()
	
	opt.logger.Debug("Cleaned up old query statistics",
		"cutoff", cutoff,
		"remaining_stats", len(opt.queryStats))
}

// normalizeQuery removes specific values from queries for statistics grouping
func (opt *DBOptimizer) normalizeQuery(query string) string {
	// This is a simple implementation - could be enhanced with proper SQL parsing
	// For now, just limit the length and remove some obvious parameters
	if len(query) > 200 {
		query = query[:200] + "..."
	}
	
	// Remove common parameter patterns (this is basic, could be improved)
	// In a production system, you'd want more sophisticated query normalization
	return query
}

// parseQuery extracts query metadata for metrics labeling
func (opt *DBOptimizer) parseQuery(query string) (queryType, table, operation string) {
	// This is a simplified parser - in production you'd want proper SQL parsing
	query = strings.ToUpper(strings.TrimSpace(query))
	
	if strings.HasPrefix(query, "SELECT") {
		queryType = "select"
		operation = "read"
	} else if strings.HasPrefix(query, "INSERT") {
		queryType = "insert"
		operation = "write"
	} else if strings.HasPrefix(query, "UPDATE") {
		queryType = "update"
		operation = "write"
	} else if strings.HasPrefix(query, "DELETE") {
		queryType = "delete"
		operation = "write"
	} else {
		queryType = "other"
		operation = "other"
	}
	
	// Extract table name (very basic)
	table = "unknown"
	// This would need proper SQL parsing for accurate table extraction
	
	return queryType, table, operation
}

// GetTopSlowQueries returns the slowest queries sorted by duration
func (opt *DBOptimizer) GetTopSlowQueries(limit int) []SlowQuery {
	queries := opt.GetSlowQueries()
	
	// Sort by duration descending
	sort.Slice(queries, func(i, j int) bool {
		return queries[i].Duration > queries[j].Duration
	})
	
	if limit > 0 && len(queries) > limit {
		queries = queries[:limit]
	}
	
	return queries
}

// GetQueryStatsSummary returns a summary of query statistics
func (opt *DBOptimizer) GetQueryStatsSummary() map[string]interface{} {
	stats := opt.GetQueryStats()
	slowQueries := opt.GetSlowQueries()
	migrations := opt.GetMigrationHistory()
	health := opt.GetDatabaseHealth()
	
	var totalQueries int64
	var totalDuration time.Duration
	var errorCount int64
	
	for _, stat := range stats {
		totalQueries += stat.ExecutionCount
		totalDuration += stat.TotalDuration
		errorCount += stat.ErrorCount
	}
	
	var avgDuration time.Duration
	if totalQueries > 0 {
		avgDuration = time.Duration(int64(totalDuration) / totalQueries)
	}
	
	summary := map[string]interface{}{
		"total_queries":      totalQueries,
		"total_duration":     totalDuration,
		"average_duration":   avgDuration,
		"error_count":        errorCount,
		"error_rate":         float64(errorCount) / float64(totalQueries) * 100,
		"slow_queries":       len(slowQueries),
		"unique_queries":     len(stats),
		"migrations_count":   len(migrations),
		"last_health_check":  opt.lastHealthCheck,
	}
	
	if health != nil {
		summary["health"] = health
	}
	
	return summary
}

// Stop gracefully shuts down the database optimizer
func (opt *DBOptimizer) Stop() error {
	opt.logger.Info("Stopping database optimizer")
	
	opt.cancel()
	opt.wg.Wait()
	
	opt.logger.Info("Database optimizer stopped")
	return nil
}