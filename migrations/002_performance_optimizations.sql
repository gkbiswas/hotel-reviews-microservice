-- ============================================================================
-- Migration: 002_performance_optimizations.sql
-- Description: Performance optimizations and additional indexes
-- Version: 1.0.0
-- Created: 2024-01-01
-- ============================================================================

-- ============================================================================
-- ADDITIONAL INDEXES FOR PERFORMANCE
-- ============================================================================

-- Partial indexes for active/non-deleted records
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_providers_active_name ON providers(name) WHERE is_active = true AND deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hotels_active_rating ON hotels(star_rating) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_recent ON reviews(review_date DESC) WHERE deleted_at IS NULL AND review_date > CURRENT_DATE - INTERVAL '1 year';

-- Covering indexes for common queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_hotel_summary ON reviews(hotel_id) INCLUDE (rating, review_date, sentiment) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_provider_summary ON reviews(provider_id) INCLUDE (rating, review_date, hotel_id) WHERE deleted_at IS NULL;

-- Multi-column indexes for complex queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_hotel_date_sentiment ON reviews(hotel_id, review_date, sentiment) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_rating_date ON reviews(rating, review_date DESC) WHERE deleted_at IS NULL;

-- GIN indexes for JSONB columns
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_metadata_gin ON reviews USING gin(metadata) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hotels_amenities_gin ON hotels USING gin(amenities) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hotels_external_ids_gin ON hotels USING gin(external_ids) WHERE deleted_at IS NULL;

-- Indexes for full-text search
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_comment_title_gin ON reviews USING gin(to_tsvector('english', COALESCE(comment, '') || ' ' || COALESCE(title, ''))) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hotels_searchable_gin ON hotels USING gin(to_tsvector('english', COALESCE(name, '') || ' ' || COALESCE(city, '') || ' ' || COALESCE(country, ''))) WHERE deleted_at IS NULL;

-- Indexes for analytics and reporting
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_monthly_stats ON reviews(DATE_TRUNC('month', review_date), provider_id) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reviews_yearly_stats ON reviews(DATE_TRUNC('year', review_date), hotel_id) WHERE deleted_at IS NULL;

-- ============================================================================
-- MATERIALIZED VIEWS FOR PERFORMANCE
-- ============================================================================

-- Materialized view for hotel performance metrics
CREATE MATERIALIZED VIEW hotel_performance_metrics AS
SELECT 
    h.id as hotel_id,
    h.name as hotel_name,
    h.city,
    h.country,
    h.star_rating,
    COUNT(r.id) as total_reviews,
    ROUND(AVG(r.rating), 2) as avg_rating,
    ROUND(AVG(r.service_rating), 2) as avg_service_rating,
    ROUND(AVG(r.cleanliness_rating), 2) as avg_cleanliness_rating,
    ROUND(AVG(r.location_rating), 2) as avg_location_rating,
    ROUND(AVG(r.value_rating), 2) as avg_value_rating,
    COUNT(r.id) FILTER (WHERE r.sentiment = 'positive') as positive_reviews,
    COUNT(r.id) FILTER (WHERE r.sentiment = 'negative') as negative_reviews,
    COUNT(r.id) FILTER (WHERE r.sentiment = 'neutral') as neutral_reviews,
    COUNT(r.id) FILTER (WHERE r.rating >= 4.0) as high_rating_reviews,
    COUNT(r.id) FILTER (WHERE r.rating <= 2.0) as low_rating_reviews,
    MIN(r.review_date) as first_review_date,
    MAX(r.review_date) as last_review_date,
    COUNT(r.id) FILTER (WHERE r.review_date > CURRENT_DATE - INTERVAL '30 days') as reviews_last_30_days,
    COUNT(r.id) FILTER (WHERE r.review_date > CURRENT_DATE - INTERVAL '90 days') as reviews_last_90_days,
    COUNT(DISTINCT r.provider_id) as provider_count,
    COUNT(DISTINCT r.reviewer_info_id) as unique_reviewers
FROM hotels h
LEFT JOIN reviews r ON h.id = r.hotel_id AND r.deleted_at IS NULL
WHERE h.deleted_at IS NULL
GROUP BY h.id, h.name, h.city, h.country, h.star_rating;

-- Create index on materialized view
CREATE INDEX idx_hotel_performance_metrics_rating ON hotel_performance_metrics(avg_rating DESC);
CREATE INDEX idx_hotel_performance_metrics_reviews ON hotel_performance_metrics(total_reviews DESC);
CREATE INDEX idx_hotel_performance_metrics_city ON hotel_performance_metrics(city, country);
CREATE INDEX idx_hotel_performance_metrics_recent ON hotel_performance_metrics(reviews_last_30_days DESC);

-- Materialized view for provider performance metrics
CREATE MATERIALIZED VIEW provider_performance_metrics AS
SELECT 
    p.id as provider_id,
    p.name as provider_name,
    p.is_active,
    COUNT(r.id) as total_reviews,
    ROUND(AVG(r.rating), 2) as avg_rating,
    COUNT(DISTINCT r.hotel_id) as unique_hotels,
    COUNT(DISTINCT r.reviewer_info_id) as unique_reviewers,
    MIN(r.review_date) as first_review_date,
    MAX(r.review_date) as last_review_date,
    COUNT(r.id) FILTER (WHERE r.review_date > CURRENT_DATE - INTERVAL '30 days') as reviews_last_30_days,
    COUNT(r.id) FILTER (WHERE r.review_date > CURRENT_DATE - INTERVAL '90 days') as reviews_last_90_days,
    COUNT(r.id) FILTER (WHERE r.sentiment = 'positive') as positive_reviews,
    COUNT(r.id) FILTER (WHERE r.sentiment = 'negative') as negative_reviews,
    COUNT(r.id) FILTER (WHERE r.is_verified = true) as verified_reviews,
    ROUND(AVG(r.helpful_votes::decimal / NULLIF(r.total_votes, 0)), 2) as avg_helpfulness_ratio
FROM providers p
LEFT JOIN reviews r ON p.id = r.provider_id AND r.deleted_at IS NULL
WHERE p.deleted_at IS NULL
GROUP BY p.id, p.name, p.is_active;

-- Create index on materialized view
CREATE INDEX idx_provider_performance_metrics_reviews ON provider_performance_metrics(total_reviews DESC);
CREATE INDEX idx_provider_performance_metrics_hotels ON provider_performance_metrics(unique_hotels DESC);
CREATE INDEX idx_provider_performance_metrics_active ON provider_performance_metrics(is_active, total_reviews DESC);

-- ============================================================================
-- FUNCTIONS FOR PERFORMANCE OPTIMIZATION
-- ============================================================================

-- Function to refresh materialized views
CREATE OR REPLACE FUNCTION refresh_performance_metrics()
RETURNS VOID AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY hotel_performance_metrics;
    REFRESH MATERIALIZED VIEW CONCURRENTLY provider_performance_metrics;
    
    -- Update statistics
    ANALYZE hotel_performance_metrics;
    ANALYZE provider_performance_metrics;
    
    RAISE NOTICE 'Performance metrics refreshed successfully';
END;
$$ LANGUAGE plpgsql;

-- Function to get hotel ranking
CREATE OR REPLACE FUNCTION get_hotel_ranking(hotel_uuid UUID)
RETURNS TABLE(
    hotel_id UUID,
    hotel_name TEXT,
    city TEXT,
    country TEXT,
    avg_rating DECIMAL(3,2),
    total_reviews BIGINT,
    rank_by_rating INTEGER,
    rank_by_reviews INTEGER,
    rank_in_city INTEGER
) AS $$
BEGIN
    RETURN QUERY
    WITH hotel_ranks AS (
        SELECT 
            hpm.hotel_id,
            hpm.hotel_name,
            hpm.city,
            hpm.country,
            hpm.avg_rating,
            hpm.total_reviews,
            ROW_NUMBER() OVER (ORDER BY hpm.avg_rating DESC, hpm.total_reviews DESC) as rank_by_rating,
            ROW_NUMBER() OVER (ORDER BY hpm.total_reviews DESC, hpm.avg_rating DESC) as rank_by_reviews,
            ROW_NUMBER() OVER (PARTITION BY hpm.city, hpm.country ORDER BY hpm.avg_rating DESC, hpm.total_reviews DESC) as rank_in_city
        FROM hotel_performance_metrics hpm
        WHERE hpm.total_reviews > 0
    )
    SELECT 
        hr.hotel_id,
        hr.hotel_name,
        hr.city,
        hr.country,
        hr.avg_rating,
        hr.total_reviews,
        hr.rank_by_rating::INTEGER,
        hr.rank_by_reviews::INTEGER,
        hr.rank_in_city::INTEGER
    FROM hotel_ranks hr
    WHERE hr.hotel_id = hotel_uuid;
END;
$$ LANGUAGE plpgsql;

-- Function for efficient similarity search
CREATE OR REPLACE FUNCTION find_similar_hotels(
    target_hotel_id UUID,
    similarity_threshold DECIMAL DEFAULT 0.3,
    max_results INTEGER DEFAULT 10
)
RETURNS TABLE(
    hotel_id UUID,
    hotel_name TEXT,
    city TEXT,
    country TEXT,
    similarity_score DECIMAL(3,2),
    avg_rating DECIMAL(3,2),
    total_reviews BIGINT
) AS $$
BEGIN
    RETURN QUERY
    WITH target_hotel AS (
        SELECT h.name, h.city, h.country, h.amenities, h.star_rating
        FROM hotels h
        WHERE h.id = target_hotel_id AND h.deleted_at IS NULL
    )
    SELECT 
        h.id,
        h.name,
        h.city,
        h.country,
        ROUND(
            (similarity(h.name, th.name) * 0.3 +
             similarity(h.city, th.city) * 0.2 +
             similarity(h.country, th.country) * 0.1 +
             CASE WHEN h.star_rating = th.star_rating THEN 0.4 ELSE 0 END)::DECIMAL, 2
        ) as similarity_score,
        hpm.avg_rating,
        hpm.total_reviews
    FROM hotels h
    CROSS JOIN target_hotel th
    LEFT JOIN hotel_performance_metrics hpm ON h.id = hpm.hotel_id
    WHERE h.id != target_hotel_id 
        AND h.deleted_at IS NULL
        AND (
            similarity(h.name, th.name) > similarity_threshold OR
            similarity(h.city, th.city) > similarity_threshold OR
            h.star_rating = th.star_rating
        )
    ORDER BY similarity_score DESC, hpm.avg_rating DESC NULLS LAST
    LIMIT max_results;
END;
$$ LANGUAGE plpgsql;

-- Function for efficient review aggregation
CREATE OR REPLACE FUNCTION get_review_aggregations(
    hotel_uuid UUID,
    start_date TIMESTAMP DEFAULT NULL,
    end_date TIMESTAMP DEFAULT NULL
)
RETURNS TABLE(
    total_reviews BIGINT,
    avg_rating DECIMAL(3,2),
    rating_distribution JSONB,
    sentiment_distribution JSONB,
    language_distribution JSONB,
    monthly_trends JSONB
) AS $$
DECLARE
    date_filter TEXT := '';
BEGIN
    -- Build date filter if provided
    IF start_date IS NOT NULL AND end_date IS NOT NULL THEN
        date_filter := ' AND review_date BETWEEN ''' || start_date || ''' AND ''' || end_date || '''';
    END IF;
    
    RETURN QUERY
    EXECUTE format('
        WITH review_stats AS (
            SELECT 
                COUNT(*) as total_reviews,
                ROUND(AVG(rating), 2) as avg_rating
            FROM reviews 
            WHERE hotel_id = $1 AND deleted_at IS NULL %s
        ),
        rating_dist AS (
            SELECT jsonb_object_agg(
                CASE 
                    WHEN rating >= 4.5 THEN ''5_stars''
                    WHEN rating >= 3.5 THEN ''4_stars''
                    WHEN rating >= 2.5 THEN ''3_stars''
                    WHEN rating >= 1.5 THEN ''2_stars''
                    ELSE ''1_star''
                END,
                count
            ) as distribution
            FROM (
                SELECT 
                    CASE 
                        WHEN rating >= 4.5 THEN ''5_stars''
                        WHEN rating >= 3.5 THEN ''4_stars''
                        WHEN rating >= 2.5 THEN ''3_stars''
                        WHEN rating >= 1.5 THEN ''2_stars''
                        ELSE ''1_star''
                    END as rating_group,
                    COUNT(*) as count
                FROM reviews 
                WHERE hotel_id = $1 AND deleted_at IS NULL %s
                GROUP BY rating_group
            ) rd
        ),
        sentiment_dist AS (
            SELECT jsonb_object_agg(sentiment, count) as distribution
            FROM (
                SELECT sentiment, COUNT(*) as count
                FROM reviews 
                WHERE hotel_id = $1 AND deleted_at IS NULL AND sentiment IS NOT NULL %s
                GROUP BY sentiment
            ) sd
        ),
        language_dist AS (
            SELECT jsonb_object_agg(language, count) as distribution
            FROM (
                SELECT language, COUNT(*) as count
                FROM reviews 
                WHERE hotel_id = $1 AND deleted_at IS NULL %s
                GROUP BY language
            ) ld
        ),
        monthly_trends AS (
            SELECT jsonb_object_agg(month_year, metrics) as trends
            FROM (
                SELECT 
                    TO_CHAR(review_date, ''YYYY-MM'') as month_year,
                    jsonb_build_object(
                        ''count'', COUNT(*),
                        ''avg_rating'', ROUND(AVG(rating), 2)
                    ) as metrics
                FROM reviews 
                WHERE hotel_id = $1 AND deleted_at IS NULL %s
                GROUP BY TO_CHAR(review_date, ''YYYY-MM'')
                ORDER BY month_year
            ) mt
        )
        SELECT 
            rs.total_reviews,
            rs.avg_rating,
            COALESCE(rd.distribution, ''{}''::jsonb) as rating_distribution,
            COALESCE(sd.distribution, ''{}''::jsonb) as sentiment_distribution,
            COALESCE(ld.distribution, ''{}''::jsonb) as language_distribution,
            COALESCE(mt.trends, ''{}''::jsonb) as monthly_trends
        FROM review_stats rs
        CROSS JOIN rating_dist rd
        CROSS JOIN sentiment_dist sd
        CROSS JOIN language_dist ld
        CROSS JOIN monthly_trends mt
    ', date_filter, date_filter, date_filter, date_filter, date_filter)
    USING hotel_uuid;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- PERFORMANCE MONITORING VIEWS
-- ============================================================================

-- View for monitoring slow queries
CREATE VIEW slow_query_candidates AS
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_blks_read,
    idx_blks_hit,
    idx_blks_read + idx_blks_hit as total_idx_access,
    ROUND(100.0 * idx_blks_hit / NULLIF(idx_blks_read + idx_blks_hit, 0), 2) as idx_hit_rate
FROM pg_stat_user_indexes
WHERE idx_blks_read + idx_blks_hit > 0
ORDER BY idx_blks_read DESC;

-- View for table statistics
CREATE VIEW table_performance_stats AS
SELECT 
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_tup_ins + n_tup_upd + n_tup_del as total_operations,
    seq_scan,
    seq_tup_read,
    idx_scan,
    idx_tup_fetch,
    ROUND(100.0 * idx_scan / NULLIF(seq_scan + idx_scan, 0), 2) as idx_scan_ratio
FROM pg_stat_user_tables
ORDER BY total_operations DESC;

-- ============================================================================
-- SCHEDULED JOBS / PROCEDURES
-- ============================================================================

-- Function to update review statistics (should be called periodically)
CREATE OR REPLACE FUNCTION update_all_review_summaries()
RETURNS VOID AS $$
DECLARE
    hotel_record RECORD;
    updated_count INTEGER := 0;
BEGIN
    FOR hotel_record IN 
        SELECT DISTINCT hotel_id 
        FROM reviews 
        WHERE deleted_at IS NULL
    LOOP
        PERFORM update_review_summary(hotel_record.hotel_id);
        updated_count := updated_count + 1;
    END LOOP;
    
    RAISE NOTICE 'Updated review summaries for % hotels', updated_count;
END;
$$ LANGUAGE plpgsql;

-- Function to cleanup old processing logs
CREATE OR REPLACE FUNCTION cleanup_old_processing_logs(retention_days INTEGER DEFAULT 30)
RETURNS VOID AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM review_processing_statuses 
    WHERE created_at < CURRENT_DATE - INTERVAL '1 day' * retention_days
    AND status IN ('completed', 'failed', 'cancelled');
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    RAISE NOTICE 'Cleaned up % old processing log records', deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Function to analyze and optimize database
CREATE OR REPLACE FUNCTION optimize_database()
RETURNS VOID AS $$
BEGIN
    -- Update statistics for all tables
    ANALYZE;
    
    -- Refresh materialized views
    PERFORM refresh_performance_metrics();
    
    -- Cleanup old processing logs
    PERFORM cleanup_old_processing_logs();
    
    RAISE NOTICE 'Database optimization completed';
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- MIGRATION COMPLETION
-- ============================================================================

-- Insert migration record
INSERT INTO schema_migrations (version) VALUES ('002_performance_optimizations')
ON CONFLICT (version) DO NOTHING;

-- Update table statistics
ANALYZE;

-- Initial refresh of materialized views
SELECT refresh_performance_metrics();

-- Final message
DO $$
BEGIN
    RAISE NOTICE 'Migration 002_performance_optimizations completed successfully';
    RAISE NOTICE 'Performance optimizations applied to Hotel Reviews database';
END $$;