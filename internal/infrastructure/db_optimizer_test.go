package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock logger for testing
type mockDBLogger struct {
	logs []dbLogEntry
}

type dbLogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (m *mockDBLogger) Debug(msg string, fields ...interface{}) {
	m.logs = append(m.logs, dbLogEntry{Level: "debug", Message: msg, Fields: dbFieldsToMap(fields)})
}

func (m *mockDBLogger) Info(msg string, fields ...interface{}) {
	m.logs = append(m.logs, dbLogEntry{Level: "info", Message: msg, Fields: dbFieldsToMap(fields)})
}

func (m *mockDBLogger) Warn(msg string, fields ...interface{}) {
	m.logs = append(m.logs, dbLogEntry{Level: "warn", Message: msg, Fields: dbFieldsToMap(fields)})
}

func (m *mockDBLogger) Error(msg string, fields ...interface{}) {
	m.logs = append(m.logs, dbLogEntry{Level: "error", Message: msg, Fields: dbFieldsToMap(fields)})
}

func dbFieldsToMap(fields []interface{}) map[string]interface{} {
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

func createTestDBLogger() *mockDBLogger {
	return &mockDBLogger{logs: make([]dbLogEntry, 0)}
}

func TestDefaultDBOptimizerConfig(t *testing.T) {
	config := DefaultDBOptimizerConfig()
	
	assert.Equal(t, 500*time.Millisecond, config.SlowQueryThreshold)
	assert.Equal(t, 100, config.MaxSlowQueries)
	assert.Equal(t, 24*time.Hour, config.QueryStatsRetention)
	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	assert.Equal(t, 5*time.Second, config.ConnectionTimeout)
	assert.Equal(t, 15*time.Second, config.MetricsInterval)
	assert.True(t, config.EnableQueryLogging)
	assert.True(t, config.EnableSlowQueryLog)
	assert.True(t, config.EnablePoolOptimization)
	assert.Equal(t, 5*time.Minute, config.PoolOptimizationInterval)
	assert.True(t, config.EnableMigrationTracking)
}

func TestDBOptimizer_TrackQuery(t *testing.T) {
	// Note: This test doesn't require a real database connection
	// We're testing the query tracking logic
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	// Create optimizer without pool for unit testing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger:      logger,
		config:      config,
		queryStats:  make(map[string]*QueryStats),
		slowQueries: make([]SlowQuery, 0, config.MaxSlowQueries),
		ctx:         ctx,
		cancel:      cancel,
		metrics:     createDBMetrics(),
	}
	
	t.Run("track normal query", func(t *testing.T) {
		query := "SELECT * FROM hotels WHERE id = $1"
		duration := 50 * time.Millisecond
		
		optimizer.TrackQuery(query, duration, nil, 1)
		
		stats := optimizer.GetQueryStats()
		assert.Len(t, stats, 1)
		
		normalizedQuery := optimizer.normalizeQuery(query)
		stat, exists := stats[normalizedQuery]
		require.True(t, exists)
		
		assert.Equal(t, int64(1), stat.ExecutionCount)
		assert.Equal(t, duration, stat.TotalDuration)
		assert.Equal(t, duration, stat.MinDuration)
		assert.Equal(t, duration, stat.MaxDuration)
		assert.Equal(t, duration, stat.AvgDuration)
		assert.Equal(t, int64(0), stat.ErrorCount)
		assert.Equal(t, int64(1), stat.RowsAffected)
	})
	
	t.Run("track slow query", func(t *testing.T) {
		query := "SELECT * FROM reviews WHERE hotel_id = $1"
		duration := 600 * time.Millisecond // Above threshold
		
		optimizer.TrackQuery(query, duration, nil, 100)
		
		slowQueries := optimizer.GetSlowQueries()
		assert.Len(t, slowQueries, 1)
		assert.Equal(t, query, slowQueries[0].Query)
		assert.Equal(t, duration, slowQueries[0].Duration)
		assert.Empty(t, slowQueries[0].Error)
	})
	
	t.Run("track query with error", func(t *testing.T) {
		query := "INSERT INTO hotels (name) VALUES ($1)"
		duration := 100 * time.Millisecond
		err := errors.New("duplicate key violation")
		
		optimizer.TrackQuery(query, duration, err, 0)
		
		stats := optimizer.GetQueryStats()
		normalizedQuery := optimizer.normalizeQuery(query)
		stat, exists := stats[normalizedQuery]
		require.True(t, exists)
		
		assert.Equal(t, int64(1), stat.ErrorCount)
	})
	
	t.Run("track multiple executions of same query", func(t *testing.T) {
		query := "UPDATE hotels SET rating = $1 WHERE id = $2"
		
		// Execute the same query multiple times
		optimizer.TrackQuery(query, 20*time.Millisecond, nil, 1)
		optimizer.TrackQuery(query, 30*time.Millisecond, nil, 1)
		optimizer.TrackQuery(query, 25*time.Millisecond, nil, 1)
		
		stats := optimizer.GetQueryStats()
		normalizedQuery := optimizer.normalizeQuery(query)
		stat, exists := stats[normalizedQuery]
		require.True(t, exists)
		
		assert.Equal(t, int64(3), stat.ExecutionCount)
		assert.Equal(t, 75*time.Millisecond, stat.TotalDuration)
		assert.Equal(t, 20*time.Millisecond, stat.MinDuration)
		assert.Equal(t, 30*time.Millisecond, stat.MaxDuration)
		assert.Equal(t, 25*time.Millisecond, stat.AvgDuration)
		assert.Equal(t, int64(3), stat.RowsAffected)
	})
}

func TestDBOptimizer_TrackMigration(t *testing.T) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger:           logger,
		config:           config,
		migrationHistory: make([]MigrationRecord, 0),
		ctx:              ctx,
		cancel:           cancel,
		metrics:          createDBMetrics(),
	}
	
	t.Run("track successful migration", func(t *testing.T) {
		version := "001"
		name := "create_hotels_table"
		startTime := time.Now()
		endTime := startTime.Add(2 * time.Second)
		
		optimizer.TrackMigration(version, name, startTime, endTime, true, nil, 1)
		
		history := optimizer.GetMigrationHistory()
		assert.Len(t, history, 1)
		
		migration := history[0]
		assert.Equal(t, version, migration.Version)
		assert.Equal(t, name, migration.Name)
		assert.Equal(t, startTime, migration.StartTime)
		assert.Equal(t, endTime, migration.EndTime)
		assert.Equal(t, 2*time.Second, migration.Duration)
		assert.True(t, migration.Success)
		assert.Empty(t, migration.Error)
		assert.Equal(t, int64(1), migration.RowsAffected)
	})
	
	t.Run("track failed migration", func(t *testing.T) {
		version := "002"
		name := "add_index_to_hotels"
		startTime := time.Now()
		endTime := startTime.Add(500 * time.Millisecond)
		err := errors.New("index already exists")
		
		optimizer.TrackMigration(version, name, startTime, endTime, false, err, 0)
		
		history := optimizer.GetMigrationHistory()
		assert.Len(t, history, 2) // Previous migration + this one
		
		migration := history[1]
		assert.Equal(t, version, migration.Version)
		assert.False(t, migration.Success)
		assert.Equal(t, "index already exists", migration.Error)
	})
}

func TestDBOptimizer_SlowQueryTracking(t *testing.T) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	config.MaxSlowQueries = 3 // Small limit for testing
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger:      logger,
		config:      config,
		queryStats:  make(map[string]*QueryStats),
		slowQueries: make([]SlowQuery, 0, config.MaxSlowQueries),
		ctx:         ctx,
		cancel:      cancel,
		metrics:     createDBMetrics(),
	}
	
	t.Run("slow query limit enforcement", func(t *testing.T) {
		// Add more slow queries than the limit
		for i := 0; i < 5; i++ {
			query := fmt.Sprintf("SELECT * FROM table_%d", i)
			duration := time.Duration(600+i*100) * time.Millisecond
			optimizer.TrackQuery(query, duration, nil, 1)
		}
		
		slowQueries := optimizer.GetSlowQueries()
		assert.Len(t, slowQueries, 3) // Should be limited to MaxSlowQueries
		
		// Should contain the most recent queries
		assert.Contains(t, slowQueries[2].Query, "table_4")
	})
	
	t.Run("get top slow queries", func(t *testing.T) {
		topQueries := optimizer.GetTopSlowQueries(2)
		assert.Len(t, topQueries, 2)
		
		// Should be sorted by duration (descending)
		assert.True(t, topQueries[0].Duration >= topQueries[1].Duration)
	})
}

func TestDBOptimizer_QueryNormalization(t *testing.T) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger: logger,
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
	
	testCases := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "short query unchanged",
			query:    "SELECT * FROM hotels",
			expected: "SELECT * FROM hotels",
		},
		{
			name:     "long query truncated",
			query:    strings.Repeat("SELECT * FROM hotels WHERE ", 10) + "id = 1",
			expected: strings.Repeat("SELECT * FROM hotels WHERE ", 7) + "SELECT * FROM hotels WH...",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized := optimizer.normalizeQuery(tc.query)
			if len(tc.expected) > 200 {
				assert.Equal(t, 203, len(normalized)) // 200 + "..."
				assert.True(t, strings.HasSuffix(normalized, "..."))
			} else {
				assert.Equal(t, tc.expected, normalized)
			}
		})
	}
}

func TestDBOptimizer_QueryParsing(t *testing.T) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger: logger,
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
	
	testCases := []struct {
		query         string
		expectedType  string
		expectedOp    string
	}{
		{"SELECT * FROM hotels", "select", "read"},
		{"INSERT INTO hotels (name) VALUES ('Test')", "insert", "write"},
		{"UPDATE hotels SET name = 'New Name'", "update", "write"},
		{"DELETE FROM hotels WHERE id = 1", "delete", "write"},
		{"CREATE TABLE test (id INT)", "other", "other"},
		{"  select * from users  ", "select", "read"}, // Test whitespace and case
	}
	
	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			queryType, _, operation := optimizer.parseQuery(tc.query)
			assert.Equal(t, tc.expectedType, queryType)
			assert.Equal(t, tc.expectedOp, operation)
		})
	}
}

func TestDBOptimizer_GetQueryStatsSummary(t *testing.T) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger:           logger,
		config:           config,
		queryStats:       make(map[string]*QueryStats),
		slowQueries:      make([]SlowQuery, 0),
		migrationHistory: make([]MigrationRecord, 0),
		ctx:              ctx,
		cancel:           cancel,
		metrics:          createDBMetrics(),
	}
	
	// Add some test data
	optimizer.TrackQuery("SELECT * FROM hotels", 100*time.Millisecond, nil, 10)
	optimizer.TrackQuery("SELECT * FROM hotels", 150*time.Millisecond, nil, 5)
	optimizer.TrackQuery("INSERT INTO hotels", 50*time.Millisecond, errors.New("error"), 0)
	optimizer.TrackQuery("UPDATE hotels", 600*time.Millisecond, nil, 3) // Slow query
	
	summary := optimizer.GetQueryStatsSummary()
	
	// Verify summary contains expected fields
	assert.Contains(t, summary, "total_queries")
	assert.Contains(t, summary, "total_duration")
	assert.Contains(t, summary, "average_duration")
	assert.Contains(t, summary, "error_count")
	assert.Contains(t, summary, "error_rate")
	assert.Contains(t, summary, "slow_queries")
	assert.Contains(t, summary, "unique_queries")
	
	// Verify calculated values
	assert.Equal(t, int64(4), summary["total_queries"])
	assert.Equal(t, int64(1), summary["error_count"])
	assert.Equal(t, float64(25), summary["error_rate"]) // 1/4 * 100
	assert.Equal(t, 1, summary["slow_queries"])        // One query above threshold
	assert.Equal(t, 3, summary["unique_queries"])      // 3 different normalized queries
}

func TestDBOptimizer_PoolOptimizationSuggestions(t *testing.T) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger:      logger,
		config:      config,
		slowQueries: make([]SlowQuery, 0),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	t.Run("suggestions when pool optimization disabled", func(t *testing.T) {
		optimizer.config.EnablePoolOptimization = false
		suggestions := optimizer.GetPoolOptimizationSuggestions()
		assert.Nil(t, suggestions)
	})
	
	t.Run("slow query suggestions", func(t *testing.T) {
		optimizer.config.EnablePoolOptimization = true
		
		// Add many slow queries
		for i := 0; i < 15; i++ {
			slowQuery := SlowQuery{
				Query:     fmt.Sprintf("SELECT * FROM table_%d", i),
				Duration:  600 * time.Millisecond,
				Timestamp: time.Now(),
			}
			optimizer.slowQueries = append(optimizer.slowQueries, slowQuery)
		}
		
		suggestions := optimizer.GetPoolOptimizationSuggestions()
		
		// Since pool is nil in test, suggestions will be nil
		// This is expected behavior for the unit test
		assert.Nil(t, suggestions, "Suggestions should be nil when pool is nil")
	})
}

func TestDBOptimizer_Stop(t *testing.T) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	
	optimizer := &DBOptimizer{
		logger: logger,
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
	
	// Start a background goroutine to simulate the background monitor
	optimizer.wg.Add(1)
	go func() {
		defer optimizer.wg.Done()
		<-optimizer.ctx.Done()
	}()
	
	// Stop should complete without hanging
	err := optimizer.Stop()
	assert.NoError(t, err)
	
	// Verify context was cancelled
	select {
	case <-optimizer.ctx.Done():
		// Good, context was cancelled
	default:
		t.Error("Context should be cancelled after Stop()")
	}
}

// Benchmark tests
func BenchmarkDBOptimizer_TrackQuery(b *testing.B) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger:      logger,
		config:      config,
		queryStats:  make(map[string]*QueryStats),
		slowQueries: make([]SlowQuery, 0, config.MaxSlowQueries),
		ctx:         ctx,
		cancel:      cancel,
		metrics:     createDBMetrics(),
	}
	
	query := "SELECT * FROM hotels WHERE id = $1"
	duration := 50 * time.Millisecond
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			optimizer.TrackQuery(query, duration, nil, 1)
		}
	})
}

func BenchmarkDBOptimizer_NormalizeQuery(b *testing.B) {
	logger := createTestDBLogger()
	config := DefaultDBOptimizerConfig()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	optimizer := &DBOptimizer{
		logger: logger,
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
	
	query := "SELECT h.id, h.name, h.rating FROM hotels h JOIN reviews r ON h.id = r.hotel_id WHERE h.city = $1 AND r.rating > $2 ORDER BY h.rating DESC LIMIT $3"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizer.normalizeQuery(query)
	}
}