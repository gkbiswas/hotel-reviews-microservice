package infrastructure

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// QueryOptimizer provides query optimization utilities and patterns
type QueryOptimizer struct {
	db         *gorm.DB
	logger     *slog.Logger
	cacheHits  sync.Map // Track query cache hits
	slowQueries sync.Map // Track slow queries for analysis
}

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(db *gorm.DB, logger *slog.Logger) *QueryOptimizer {
	return &QueryOptimizer{
		db:     db,
		logger: logger,
	}
}

// OptimizedPreload provides optimized preloading patterns to avoid N+1 queries
type OptimizedPreload struct {
	Relation string
	Select   []string
	Limit    int
}

// PreloadConfig defines optimized preloading configurations
type PreloadConfig struct {
	Provider    *OptimizedPreload
	Hotel       *OptimizedPreload
	ReviewerInfo *OptimizedPreload
}

// GetOptimizedPreloadConfig returns the standard optimized preload configuration
func (qo *QueryOptimizer) GetOptimizedPreloadConfig() *PreloadConfig {
	return &PreloadConfig{
		Provider: &OptimizedPreload{
			Relation: "Provider",
			Select:   []string{"id", "name", "base_url", "is_active"},
		},
		Hotel: &OptimizedPreload{
			Relation: "Hotel",
			Select:   []string{"id", "name", "city", "country", "star_rating", "latitude", "longitude"},
		},
		ReviewerInfo: &OptimizedPreload{
			Relation: "ReviewerInfo",
			Select:   []string{"id", "name", "reviewer_level", "is_verified", "total_reviews"},
		},
	}
}

// ApplyOptimizedPreloads applies optimized preload patterns to a GORM query
func (qo *QueryOptimizer) ApplyOptimizedPreloads(db *gorm.DB, config *PreloadConfig) *gorm.DB {
	query := db
	
	if config.Provider != nil {
		query = query.Preload(config.Provider.Relation, func(db *gorm.DB) *gorm.DB {
			return db.Select(config.Provider.Select)
		})
	}
	
	if config.Hotel != nil {
		query = query.Preload(config.Hotel.Relation, func(db *gorm.DB) *gorm.DB {
			return db.Select(config.Hotel.Select)
		})
	}
	
	if config.ReviewerInfo != nil {
		query = query.Preload(config.ReviewerInfo.Relation, func(db *gorm.DB) *gorm.DB {
			return db.Select(config.ReviewerInfo.Select)
		})
	}
	
	return query
}

// FullTextSearchBuilder provides optimized full-text search capabilities
type FullTextSearchBuilder struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewFullTextSearchBuilder creates a new full-text search builder
func NewFullTextSearchBuilder(db *gorm.DB, logger *slog.Logger) *FullTextSearchBuilder {
	return &FullTextSearchBuilder{
		db:     db,
		logger: logger,
	}
}

// SearchConfig defines configuration for full-text search
type SearchConfig struct {
	Query      string
	Language   string
	Fields     []string
	MinRank    float32
	MaxResults int
}

// BuildFullTextQuery builds optimized full-text search queries for PostgreSQL
func (fts *FullTextSearchBuilder) BuildFullTextQuery(config *SearchConfig) *gorm.DB {
	if config.Query == "" {
		return fts.db
	}
	
	// Sanitize search query
	sanitized := fts.sanitizeSearchQuery(config.Query)
	
	// Default language
	language := config.Language
	if language == "" {
		language = "english"
	}
	
	// Build tsvector fields
	var tsvectorFields []string
	if len(config.Fields) == 0 {
		// Default search fields for reviews
		tsvectorFields = []string{"title", "comment"}
	} else {
		tsvectorFields = config.Fields
	}
	
	// Build PostgreSQL full-text search query
	searchVector := fts.buildSearchVector(tsvectorFields, language)
	searchQuery := fts.buildSearchQuery(sanitized, language)
	
	query := fts.db.Where(
		fmt.Sprintf("(%s) @@ (%s)", searchVector, searchQuery),
	)
	
	// Add ranking if specified
	if config.MinRank > 0 {
		rankExpression := fmt.Sprintf("ts_rank_cd(%s, %s)", searchVector, searchQuery)
		query = query.Where(fmt.Sprintf("%s >= ?", rankExpression), config.MinRank)
		query = query.Order(fmt.Sprintf("%s DESC", rankExpression))
	}
	
	// Add limit if specified
	if config.MaxResults > 0 {
		query = query.Limit(config.MaxResults)
	}
	
	return query
}

// buildSearchVector creates a tsvector expression for multiple fields
func (fts *FullTextSearchBuilder) buildSearchVector(fields []string, language string) string {
	vectors := make([]string, len(fields))
	for i, field := range fields {
		vectors[i] = fmt.Sprintf("to_tsvector('%s', COALESCE(%s, ''))", language, field)
	}
	return strings.Join(vectors, " || ")
}

// buildSearchQuery creates a tsquery expression
func (fts *FullTextSearchBuilder) buildSearchQuery(query, language string) string {
	// Handle phrase searches and boolean operators
	if strings.Contains(query, "\"") {
		// Phrase search
		return fmt.Sprintf("phraseto_tsquery('%s', '%s')", language, strings.ReplaceAll(query, "'", "''"))
	}
	
	// Plain text search with prefix matching
	return fmt.Sprintf("to_tsquery('%s', '%s:*')", language, strings.ReplaceAll(query, "'", "''"))
}

// sanitizeSearchQuery sanitizes user input for search queries
func (fts *FullTextSearchBuilder) sanitizeSearchQuery(query string) string {
	// Remove potentially dangerous characters
	dangerous := []string{";", "--", "/*", "*/", "xp_", "sp_"}
	cleaned := query
	
	for _, danger := range dangerous {
		cleaned = strings.ReplaceAll(cleaned, danger, "")
	}
	
	// Limit length
	if len(cleaned) > 200 {
		cleaned = cleaned[:200]
	}
	
	return strings.TrimSpace(cleaned)
}

// AggregationOptimizer provides optimized aggregation query patterns
type AggregationOptimizer struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewAggregationOptimizer creates a new aggregation optimizer
func NewAggregationOptimizer(db *gorm.DB, logger *slog.Logger) *AggregationOptimizer {
	return &AggregationOptimizer{
		db:     db,
		logger: logger,
	}
}

// HotelSummaryStats represents aggregated hotel statistics
type HotelSummaryStats struct {
	HotelID            string  `json:"hotel_id"`
	TotalReviews       int64   `json:"total_reviews"`
	AverageRating      float64 `json:"average_rating"`
	LatestReviewDate   time.Time `json:"latest_review_date"`
	RatingDistribution map[int]int64 `json:"rating_distribution"`
}

// GetOptimizedHotelSummary gets hotel summary using a single optimized query
func (ao *AggregationOptimizer) GetOptimizedHotelSummary(ctx context.Context, hotelID string) (*HotelSummaryStats, error) {
	var result struct {
		HotelID          string    `db:"hotel_id"`
		TotalReviews     int64     `db:"total_reviews"`
		AverageRating    float64   `db:"average_rating"`
		LatestReviewDate time.Time `db:"latest_review_date"`
		Rating1Count     int64     `db:"rating_1_count"`
		Rating2Count     int64     `db:"rating_2_count"`
		Rating3Count     int64     `db:"rating_3_count"`
		Rating4Count     int64     `db:"rating_4_count"`
		Rating5Count     int64     `db:"rating_5_count"`
	}
	
	// Single optimized query using CTE and window functions
	query := `
		WITH review_stats AS (
			SELECT 
				hotel_id,
				COUNT(*) as total_reviews,
				AVG(rating) as average_rating,
				MAX(review_date) as latest_review_date
			FROM reviews 
			WHERE hotel_id = ? AND deleted_at IS NULL
		),
		rating_distribution AS (
			SELECT 
				hotel_id,
				SUM(CASE WHEN rating >= 1 AND rating < 2 THEN 1 ELSE 0 END) as rating_1_count,
				SUM(CASE WHEN rating >= 2 AND rating < 3 THEN 1 ELSE 0 END) as rating_2_count,
				SUM(CASE WHEN rating >= 3 AND rating < 4 THEN 1 ELSE 0 END) as rating_3_count,
				SUM(CASE WHEN rating >= 4 AND rating < 5 THEN 1 ELSE 0 END) as rating_4_count,
				SUM(CASE WHEN rating = 5 THEN 1 ELSE 0 END) as rating_5_count
			FROM reviews 
			WHERE hotel_id = ? AND deleted_at IS NULL
		)
		SELECT 
			rs.hotel_id,
			rs.total_reviews,
			rs.average_rating,
			rs.latest_review_date,
			rd.rating_1_count,
			rd.rating_2_count,
			rd.rating_3_count,
			rd.rating_4_count,
			rd.rating_5_count
		FROM review_stats rs
		CROSS JOIN rating_distribution rd
	`
	
	err := ao.db.WithContext(ctx).Raw(query, hotelID, hotelID).Scan(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get hotel summary: %w", err)
	}
	
	// Convert to structured response
	summary := &HotelSummaryStats{
		HotelID:          result.HotelID,
		TotalReviews:     result.TotalReviews,
		AverageRating:    result.AverageRating,
		LatestReviewDate: result.LatestReviewDate,
		RatingDistribution: map[int]int64{
			1: result.Rating1Count,
			2: result.Rating2Count,
			3: result.Rating3Count,
			4: result.Rating4Count,
			5: result.Rating5Count,
		},
	}
	
	return summary, nil
}

// BatchHotelSummaries gets summaries for multiple hotels in a single query
func (ao *AggregationOptimizer) BatchHotelSummaries(ctx context.Context, hotelIDs []string) ([]*HotelSummaryStats, error) {
	if len(hotelIDs) == 0 {
		return []*HotelSummaryStats{}, nil
	}
	
	var results []struct {
		HotelID          string    `db:"hotel_id"`
		TotalReviews     int64     `db:"total_reviews"`
		AverageRating    float64   `db:"average_rating"`
		LatestReviewDate time.Time `db:"latest_review_date"`
		Rating1Count     int64     `db:"rating_1_count"`
		Rating2Count     int64     `db:"rating_2_count"`
		Rating3Count     int64     `db:"rating_3_count"`
		Rating4Count     int64     `db:"rating_4_count"`
		Rating5Count     int64     `db:"rating_5_count"`
	}
	
	// Batch query for multiple hotels
	query := `
		WITH review_stats AS (
			SELECT 
				hotel_id,
				COUNT(*) as total_reviews,
				AVG(rating) as average_rating,
				MAX(review_date) as latest_review_date
			FROM reviews 
			WHERE hotel_id = ANY(?) AND deleted_at IS NULL
			GROUP BY hotel_id
		),
		rating_distribution AS (
			SELECT 
				hotel_id,
				SUM(CASE WHEN rating >= 1 AND rating < 2 THEN 1 ELSE 0 END) as rating_1_count,
				SUM(CASE WHEN rating >= 2 AND rating < 3 THEN 1 ELSE 0 END) as rating_2_count,
				SUM(CASE WHEN rating >= 3 AND rating < 4 THEN 1 ELSE 0 END) as rating_3_count,
				SUM(CASE WHEN rating >= 4 AND rating < 5 THEN 1 ELSE 0 END) as rating_4_count,
				SUM(CASE WHEN rating = 5 THEN 1 ELSE 0 END) as rating_5_count
			FROM reviews 
			WHERE hotel_id = ANY(?) AND deleted_at IS NULL
			GROUP BY hotel_id
		)
		SELECT 
			rs.hotel_id,
			rs.total_reviews,
			rs.average_rating,
			rs.latest_review_date,
			COALESCE(rd.rating_1_count, 0) as rating_1_count,
			COALESCE(rd.rating_2_count, 0) as rating_2_count,
			COALESCE(rd.rating_3_count, 0) as rating_3_count,
			COALESCE(rd.rating_4_count, 0) as rating_4_count,
			COALESCE(rd.rating_5_count, 0) as rating_5_count
		FROM review_stats rs
		LEFT JOIN rating_distribution rd ON rs.hotel_id = rd.hotel_id
	`
	
	err := ao.db.WithContext(ctx).Raw(query, hotelIDs, hotelIDs).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get batch hotel summaries: %w", err)
	}
	
	// Convert to structured response
	summaries := make([]*HotelSummaryStats, len(results))
	for i, result := range results {
		summaries[i] = &HotelSummaryStats{
			HotelID:          result.HotelID,
			TotalReviews:     result.TotalReviews,
			AverageRating:    result.AverageRating,
			LatestReviewDate: result.LatestReviewDate,
			RatingDistribution: map[int]int64{
				1: result.Rating1Count,
				2: result.Rating2Count,
				3: result.Rating3Count,
				4: result.Rating4Count,
				5: result.Rating5Count,
			},
		}
	}
	
	return summaries, nil
}

// IndexOptimizer provides database index management and optimization
type IndexOptimizer struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewIndexOptimizer creates a new index optimizer
func NewIndexOptimizer(db *gorm.DB, logger *slog.Logger) *IndexOptimizer {
	return &IndexOptimizer{
		db:     db,
		logger: logger,
	}
}

// CreateOptimizedIndexes creates additional optimized indexes
func (io *IndexOptimizer) CreateOptimizedIndexes(ctx context.Context) error {
	if io.db == nil {
		io.logger.Warn("Database connection is nil, skipping index creation")
		return nil
	}
	indexes := []string{
		// Full-text search indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_fulltext_search 
		 ON reviews USING GIN (to_tsvector('english', title || ' ' || comment))`,
		
		// Composite indexes for common query patterns
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_hotel_rating_date 
		 ON reviews (hotel_id, rating, review_date) WHERE deleted_at IS NULL`,
		
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_provider_date_rating 
		 ON reviews (provider_id, review_date, rating) WHERE deleted_at IS NULL`,
		
		// Performance indexes for filtering
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_language 
		 ON reviews (language) WHERE deleted_at IS NULL`,
		
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_sentiment 
		 ON reviews (sentiment) WHERE deleted_at IS NULL`,
		
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_is_verified 
		 ON reviews (is_verified) WHERE deleted_at IS NULL`,
		
		// Partial indexes for active records
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hotels_active 
		 ON hotels (name, city, country) WHERE deleted_at IS NULL`,
		
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_providers_active 
		 ON providers (name, is_active) WHERE deleted_at IS NULL AND is_active = true`,
		
		// Covering indexes for common SELECT patterns
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_summary_covering 
		 ON reviews (hotel_id, rating, review_date) 
		 INCLUDE (title, comment, reviewer_name) WHERE deleted_at IS NULL`,
	}
	
	for _, indexSQL := range indexes {
		io.logger.Info("Creating optimized index", "sql", indexSQL)
		
		if err := io.db.WithContext(ctx).Exec(indexSQL).Error; err != nil {
			// Log error but continue with other indexes
			io.logger.Error("Failed to create index", "error", err, "sql", indexSQL)
		} else {
			io.logger.Info("Successfully created optimized index")
		}
	}
	
	return nil
}

// AnalyzeQueryPerformance analyzes query performance and suggests improvements
func (io *IndexOptimizer) AnalyzeQueryPerformance(ctx context.Context) error {
	// Enable query analysis
	if err := io.db.WithContext(ctx).Exec("LOAD 'pg_stat_statements'").Error; err != nil {
		io.logger.Warn("Could not load pg_stat_statements extension", "error", err)
		return nil // Not critical, continue
	}
	
	// Get slow queries
	var slowQueries []struct {
		Query       string  `db:"query"`
		TotalTime   float64 `db:"total_time"`
		Calls       int64   `db:"calls"`
		MeanTime    float64 `db:"mean_time"`
		Rows        int64   `db:"rows"`
	}
	
	slowQuerySQL := `
		SELECT 
			query,
			total_exec_time as total_time,
			calls,
			mean_exec_time as mean_time,
			rows
		FROM pg_stat_statements 
		WHERE query LIKE '%reviews%' 
		ORDER BY mean_exec_time DESC 
		LIMIT 10
	`
	
	if err := io.db.WithContext(ctx).Raw(slowQuerySQL).Scan(&slowQueries).Error; err != nil {
		io.logger.Warn("Could not analyze query performance", "error", err)
		return nil
	}
	
	// Log slow queries for analysis
	for _, sq := range slowQueries {
		io.logger.Warn("Slow query detected",
			"query", sq.Query,
			"mean_time_ms", sq.MeanTime,
			"calls", sq.Calls,
			"total_time_ms", sq.TotalTime)
	}
	
	return nil
}

// ConnectionPoolOptimizer optimizes database connection pool settings
type ConnectionPoolOptimizer struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewConnectionPoolOptimizer creates a new connection pool optimizer
func NewConnectionPoolOptimizer(db *gorm.DB, logger *slog.Logger) *ConnectionPoolOptimizer {
	return &ConnectionPoolOptimizer{
		db:     db,
		logger: logger,
	}
}

// OptimizeConnectionPool optimizes connection pool settings based on workload
func (cpo *ConnectionPoolOptimizer) OptimizeConnectionPool(ctx context.Context) error {
	sqlDB, err := cpo.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	// Get current stats
	stats := sqlDB.Stats()
	
	cpo.logger.Info("Current connection pool stats",
		"open_connections", stats.OpenConnections,
		"in_use", stats.InUse,
		"idle", stats.Idle,
		"wait_count", stats.WaitCount,
		"wait_duration", stats.WaitDuration,
		"max_idle_closed", stats.MaxIdleClosed,
		"max_lifetime_closed", stats.MaxLifetimeClosed)
	
	// Optimize based on current usage patterns
	maxOpenConns := stats.OpenConnections
	if stats.WaitCount > 0 {
		// If there's waiting, increase max connections
		maxOpenConns = int(float64(maxOpenConns) * 1.5)
		if maxOpenConns > 100 {
			maxOpenConns = 100 // Cap at reasonable limit
		}
		sqlDB.SetMaxOpenConns(maxOpenConns)
		cpo.logger.Info("Increased max open connections", "new_max", maxOpenConns)
	}
	
	// Optimize idle connections
	idealIdle := maxOpenConns / 4
	if idealIdle < 2 {
		idealIdle = 2
	}
	sqlDB.SetMaxIdleConns(idealIdle)
	
	// Set connection lifetime
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	
	cpo.logger.Info("Optimized connection pool settings",
		"max_open", maxOpenConns,
		"max_idle", idealIdle,
		"max_lifetime", "30m",
		"max_idle_time", "5m")
	
	return nil
}