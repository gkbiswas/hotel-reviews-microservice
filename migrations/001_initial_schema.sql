-- ============================================================================
-- Migration: 001_initial_schema.sql
-- Description: Initial database schema for Hotel Reviews Microservice
-- Version: 1.0.0
-- Created: 2024-01-01
-- ============================================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "btree_gin";
CREATE EXTENSION IF NOT EXISTS "btree_gist";

-- ============================================================================
-- ENUMS AND CUSTOM TYPES
-- ============================================================================

-- Processing status enum
CREATE TYPE processing_status AS ENUM (
    'pending',
    'processing',
    'completed',
    'failed',
    'cancelled',
    'retrying'
);

-- Review sentiment enum
CREATE TYPE sentiment_type AS ENUM (
    'positive',
    'negative',
    'neutral'
);

-- Trip type enum
CREATE TYPE trip_type AS ENUM (
    'business',
    'leisure',
    'family',
    'couples',
    'solo',
    'group'
);

-- ============================================================================
-- PROVIDERS TABLE
-- ============================================================================

CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    base_url VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT true,
    api_key VARCHAR(255),
    rate_limit_per_minute INTEGER DEFAULT 60,
    retry_count INTEGER DEFAULT 3,
    timeout_seconds INTEGER DEFAULT 30,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for providers
CREATE INDEX idx_providers_name ON providers(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_providers_is_active ON providers(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_providers_created_at ON providers(created_at);

-- ============================================================================
-- HOTELS TABLE
-- ============================================================================

CREATE TABLE hotels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    address TEXT,
    city VARCHAR(100),
    country VARCHAR(100),
    postal_code VARCHAR(20),
    phone VARCHAR(20),
    email VARCHAR(255),
    star_rating INTEGER CHECK (star_rating >= 1 AND star_rating <= 5),
    description TEXT,
    amenities JSONB,
    latitude DECIMAL(10,8) CHECK (latitude >= -90 AND latitude <= 90),
    longitude DECIMAL(11,8) CHECK (longitude >= -180 AND longitude <= 180),
    website VARCHAR(500),
    check_in_time TIME,
    check_out_time TIME,
    total_rooms INTEGER,
    year_opened INTEGER,
    last_renovated INTEGER,
    chain_id UUID,
    brand VARCHAR(100),
    property_type VARCHAR(50),
    rating_source VARCHAR(50),
    external_ids JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for hotels
CREATE INDEX idx_hotels_name ON hotels(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_city_country ON hotels(city, country) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_star_rating ON hotels(star_rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_location ON hotels(latitude, longitude) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_chain_id ON hotels(chain_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_created_at ON hotels(created_at);
CREATE INDEX idx_hotels_name_gin ON hotels USING gin(name gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_city_gin ON hotels USING gin(city gin_trgm_ops) WHERE deleted_at IS NULL;

-- Unique constraint for hotel identification
CREATE UNIQUE INDEX idx_hotels_name_city_country ON hotels(name, city, country) WHERE deleted_at IS NULL;

-- ============================================================================
-- REVIEWER_INFOS TABLE
-- ============================================================================

CREATE TABLE reviewer_infos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255),
    email VARCHAR(255),
    country VARCHAR(100),
    is_verified BOOLEAN NOT NULL DEFAULT false,
    total_reviews INTEGER NOT NULL DEFAULT 0,
    average_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0 CHECK (average_rating >= 0 AND average_rating <= 5),
    member_since TIMESTAMP WITH TIME ZONE,
    profile_image_url VARCHAR(500),
    bio TEXT,
    age_range VARCHAR(20),
    travel_style VARCHAR(50),
    languages JSONB,
    badges JSONB,
    external_ids JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for reviewer_infos
CREATE INDEX idx_reviewer_infos_email ON reviewer_infos(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviewer_infos_country ON reviewer_infos(country) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviewer_infos_is_verified ON reviewer_infos(is_verified) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviewer_infos_total_reviews ON reviewer_infos(total_reviews) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviewer_infos_average_rating ON reviewer_infos(average_rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviewer_infos_created_at ON reviewer_infos(created_at);

-- ============================================================================
-- REVIEWS TABLE
-- ============================================================================

CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    hotel_id UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
    reviewer_info_id UUID NOT NULL REFERENCES reviewer_infos(id) ON DELETE CASCADE,
    external_id VARCHAR(255),
    rating DECIMAL(3,2) NOT NULL CHECK (rating >= 1.0 AND rating <= 5.0),
    title VARCHAR(500),
    comment TEXT NOT NULL,
    review_date TIMESTAMP WITH TIME ZONE NOT NULL,
    stay_date TIMESTAMP WITH TIME ZONE,
    trip_type trip_type,
    room_type VARCHAR(100),
    nights_stayed INTEGER,
    is_verified BOOLEAN NOT NULL DEFAULT false,
    helpful_votes INTEGER NOT NULL DEFAULT 0,
    total_votes INTEGER NOT NULL DEFAULT 0,
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    sentiment sentiment_type,
    confidence_score DECIMAL(3,2),
    source VARCHAR(100),
    
    -- Detailed ratings
    service_rating DECIMAL(3,2) CHECK (service_rating >= 1.0 AND service_rating <= 5.0),
    cleanliness_rating DECIMAL(3,2) CHECK (cleanliness_rating >= 1.0 AND cleanliness_rating <= 5.0),
    location_rating DECIMAL(3,2) CHECK (location_rating >= 1.0 AND location_rating <= 5.0),
    value_rating DECIMAL(3,2) CHECK (value_rating >= 1.0 AND value_rating <= 5.0),
    comfort_rating DECIMAL(3,2) CHECK (comfort_rating >= 1.0 AND comfort_rating <= 5.0),
    facilities_rating DECIMAL(3,2) CHECK (facilities_rating >= 1.0 AND facilities_rating <= 5.0),
    food_rating DECIMAL(3,2) CHECK (food_rating >= 1.0 AND food_rating <= 5.0),
    staff_rating DECIMAL(3,2) CHECK (staff_rating >= 1.0 AND staff_rating <= 5.0),
    
    -- Additional fields
    tags JSONB,
    pros JSONB,
    cons JSONB,
    images JSONB,
    
    -- Processing metadata
    metadata JSONB,
    processed_at TIMESTAMP WITH TIME ZONE,
    processing_hash VARCHAR(64),
    duplicate_of UUID REFERENCES reviews(id),
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for reviews
CREATE INDEX idx_reviews_provider_id ON reviews(provider_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_hotel_id ON reviews(hotel_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_reviewer_info_id ON reviews(reviewer_info_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_hotel_id_rating ON reviews(hotel_id, rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_provider_id_review_date ON reviews(provider_id, review_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_review_date ON reviews(review_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_rating ON reviews(rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_language ON reviews(language) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_sentiment ON reviews(sentiment) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_is_verified ON reviews(is_verified) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_external_id ON reviews(external_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_processing_hash ON reviews(processing_hash) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_duplicate_of ON reviews(duplicate_of) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_created_at ON reviews(created_at);

-- Full-text search indexes
CREATE INDEX idx_reviews_comment_gin ON reviews USING gin(comment gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_title_gin ON reviews USING gin(title gin_trgm_ops) WHERE deleted_at IS NULL;

-- Composite indexes for common queries
CREATE INDEX idx_reviews_hotel_date_rating ON reviews(hotel_id, review_date, rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_provider_hotel_date ON reviews(provider_id, hotel_id, review_date) WHERE deleted_at IS NULL;

-- Unique constraint for external_id per provider
CREATE UNIQUE INDEX idx_reviews_provider_external_id ON reviews(provider_id, external_id) WHERE deleted_at IS NULL AND external_id IS NOT NULL;

-- ============================================================================
-- REVIEW_SUMMARIES TABLE
-- ============================================================================

CREATE TABLE review_summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    hotel_id UUID NOT NULL UNIQUE REFERENCES hotels(id) ON DELETE CASCADE,
    total_reviews INTEGER NOT NULL DEFAULT 0,
    average_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0 CHECK (average_rating >= 0 AND average_rating <= 5),
    rating_distribution JSONB,
    
    -- Average detailed ratings
    avg_service_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    avg_cleanliness_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    avg_location_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    avg_value_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    avg_comfort_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    avg_facilities_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    avg_food_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    avg_staff_rating DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    
    -- Additional statistics
    sentiment_distribution JSONB,
    language_distribution JSONB,
    trip_type_distribution JSONB,
    monthly_review_counts JSONB,
    
    -- Date ranges
    first_review_date TIMESTAMP WITH TIME ZONE,
    last_review_date TIMESTAMP WITH TIME ZONE,
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for review_summaries
CREATE INDEX idx_review_summaries_hotel_id ON review_summaries(hotel_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_review_summaries_average_rating ON review_summaries(average_rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_review_summaries_total_reviews ON review_summaries(total_reviews) WHERE deleted_at IS NULL;
CREATE INDEX idx_review_summaries_last_review_date ON review_summaries(last_review_date) WHERE deleted_at IS NULL;

-- ============================================================================
-- REVIEW_PROCESSING_STATUSES TABLE
-- ============================================================================

CREATE TABLE review_processing_statuses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    status processing_status NOT NULL DEFAULT 'pending',
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_msg TEXT,
    records_processed INTEGER NOT NULL DEFAULT 0,
    records_total INTEGER NOT NULL DEFAULT 0,
    records_failed INTEGER NOT NULL DEFAULT 0,
    file_url VARCHAR(500) NOT NULL,
    file_size BIGINT,
    file_hash VARCHAR(64),
    processing_node VARCHAR(100),
    processing_version VARCHAR(50),
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    
    -- Performance metrics
    processing_duration_ms BIGINT,
    records_per_second DECIMAL(10,2),
    memory_usage_mb DECIMAL(10,2),
    
    -- Additional metadata
    configuration JSONB,
    metrics JSONB,
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for review_processing_statuses
CREATE INDEX idx_review_processing_statuses_provider_id ON review_processing_statuses(provider_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_review_processing_statuses_status ON review_processing_statuses(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_review_processing_statuses_provider_id_status ON review_processing_statuses(provider_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_review_processing_statuses_created_at ON review_processing_statuses(created_at);
CREATE INDEX idx_review_processing_statuses_completed_at ON review_processing_statuses(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX idx_review_processing_statuses_file_url ON review_processing_statuses(file_url) WHERE deleted_at IS NULL;

-- ============================================================================
-- HOTEL_CHAINS TABLE (for grouping hotels)
-- ============================================================================

CREATE TABLE hotel_chains (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    headquarters_country VARCHAR(100),
    founded_year INTEGER,
    website VARCHAR(500),
    logo_url VARCHAR(500),
    total_hotels INTEGER DEFAULT 0,
    total_rooms INTEGER DEFAULT 0,
    external_ids JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Add foreign key to hotels table
ALTER TABLE hotels ADD CONSTRAINT fk_hotels_chain_id FOREIGN KEY (chain_id) REFERENCES hotel_chains(id);

-- Indexes for hotel_chains
CREATE INDEX idx_hotel_chains_name ON hotel_chains(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotel_chains_headquarters_country ON hotel_chains(headquarters_country) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotel_chains_created_at ON hotel_chains(created_at);

-- ============================================================================
-- AUDIT_LOGS TABLE (for tracking all changes)
-- ============================================================================

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    table_name VARCHAR(100) NOT NULL,
    record_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE')),
    old_values JSONB,
    new_values JSONB,
    changed_fields JSONB,
    user_id UUID,
    session_id VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for audit_logs
CREATE INDEX idx_audit_logs_table_name ON audit_logs(table_name);
CREATE INDEX idx_audit_logs_record_id ON audit_logs(record_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id) WHERE user_id IS NOT NULL;

-- Partition audit_logs by month for better performance
-- CREATE TABLE audit_logs_y2024m01 PARTITION OF audit_logs FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

-- ============================================================================
-- FUNCTIONS AND TRIGGERS
-- ============================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Function to create audit log entries
CREATE OR REPLACE FUNCTION create_audit_log()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        INSERT INTO audit_logs (table_name, record_id, action, old_values)
        VALUES (TG_TABLE_NAME, OLD.id, TG_OP, row_to_json(OLD));
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO audit_logs (table_name, record_id, action, old_values, new_values)
        VALUES (TG_TABLE_NAME, NEW.id, TG_OP, row_to_json(OLD), row_to_json(NEW));
        RETURN NEW;
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO audit_logs (table_name, record_id, action, new_values)
        VALUES (TG_TABLE_NAME, NEW.id, TG_OP, row_to_json(NEW));
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Function to calculate review summary statistics
CREATE OR REPLACE FUNCTION update_review_summary(hotel_uuid UUID)
RETURNS VOID AS $$
DECLARE
    summary_data RECORD;
    rating_dist JSONB;
    sentiment_dist JSONB;
    language_dist JSONB;
    trip_type_dist JSONB;
BEGIN
    -- Calculate basic statistics
    SELECT 
        COUNT(*) as total_reviews,
        ROUND(AVG(rating), 2) as avg_rating,
        ROUND(AVG(service_rating), 2) as avg_service_rating,
        ROUND(AVG(cleanliness_rating), 2) as avg_cleanliness_rating,
        ROUND(AVG(location_rating), 2) as avg_location_rating,
        ROUND(AVG(value_rating), 2) as avg_value_rating,
        ROUND(AVG(comfort_rating), 2) as avg_comfort_rating,
        ROUND(AVG(facilities_rating), 2) as avg_facilities_rating,
        ROUND(AVG(food_rating), 2) as avg_food_rating,
        ROUND(AVG(staff_rating), 2) as avg_staff_rating,
        MIN(review_date) as first_review_date,
        MAX(review_date) as last_review_date
    INTO summary_data
    FROM reviews 
    WHERE hotel_id = hotel_uuid AND deleted_at IS NULL;

    -- Calculate rating distribution
    SELECT jsonb_object_agg(rating_group, count)
    INTO rating_dist
    FROM (
        SELECT 
            CASE 
                WHEN rating >= 4.5 THEN '5_stars'
                WHEN rating >= 3.5 THEN '4_stars'
                WHEN rating >= 2.5 THEN '3_stars'
                WHEN rating >= 1.5 THEN '2_stars'
                ELSE '1_star'
            END as rating_group,
            COUNT(*) as count
        FROM reviews 
        WHERE hotel_id = hotel_uuid AND deleted_at IS NULL
        GROUP BY rating_group
    ) rd;

    -- Calculate sentiment distribution
    SELECT jsonb_object_agg(sentiment, count)
    INTO sentiment_dist
    FROM (
        SELECT sentiment, COUNT(*) as count
        FROM reviews 
        WHERE hotel_id = hotel_uuid AND deleted_at IS NULL AND sentiment IS NOT NULL
        GROUP BY sentiment
    ) sd;

    -- Calculate language distribution
    SELECT jsonb_object_agg(language, count)
    INTO language_dist
    FROM (
        SELECT language, COUNT(*) as count
        FROM reviews 
        WHERE hotel_id = hotel_uuid AND deleted_at IS NULL
        GROUP BY language
    ) ld;

    -- Calculate trip type distribution
    SELECT jsonb_object_agg(trip_type, count)
    INTO trip_type_dist
    FROM (
        SELECT trip_type, COUNT(*) as count
        FROM reviews 
        WHERE hotel_id = hotel_uuid AND deleted_at IS NULL AND trip_type IS NOT NULL
        GROUP BY trip_type
    ) td;

    -- Upsert summary record
    INSERT INTO review_summaries (
        hotel_id, total_reviews, average_rating,
        avg_service_rating, avg_cleanliness_rating, avg_location_rating,
        avg_value_rating, avg_comfort_rating, avg_facilities_rating,
        avg_food_rating, avg_staff_rating,
        rating_distribution, sentiment_distribution, 
        language_distribution, trip_type_distribution,
        first_review_date, last_review_date
    ) VALUES (
        hotel_uuid, summary_data.total_reviews, summary_data.avg_rating,
        COALESCE(summary_data.avg_service_rating, 0), COALESCE(summary_data.avg_cleanliness_rating, 0),
        COALESCE(summary_data.avg_location_rating, 0), COALESCE(summary_data.avg_value_rating, 0),
        COALESCE(summary_data.avg_comfort_rating, 0), COALESCE(summary_data.avg_facilities_rating, 0),
        COALESCE(summary_data.avg_food_rating, 0), COALESCE(summary_data.avg_staff_rating, 0),
        rating_dist, sentiment_dist, language_dist, trip_type_dist,
        summary_data.first_review_date, summary_data.last_review_date
    )
    ON CONFLICT (hotel_id) DO UPDATE SET
        total_reviews = EXCLUDED.total_reviews,
        average_rating = EXCLUDED.average_rating,
        avg_service_rating = EXCLUDED.avg_service_rating,
        avg_cleanliness_rating = EXCLUDED.avg_cleanliness_rating,
        avg_location_rating = EXCLUDED.avg_location_rating,
        avg_value_rating = EXCLUDED.avg_value_rating,
        avg_comfort_rating = EXCLUDED.avg_comfort_rating,
        avg_facilities_rating = EXCLUDED.avg_facilities_rating,
        avg_food_rating = EXCLUDED.avg_food_rating,
        avg_staff_rating = EXCLUDED.avg_staff_rating,
        rating_distribution = EXCLUDED.rating_distribution,
        sentiment_distribution = EXCLUDED.sentiment_distribution,
        language_distribution = EXCLUDED.language_distribution,
        trip_type_distribution = EXCLUDED.trip_type_distribution,
        first_review_date = EXCLUDED.first_review_date,
        last_review_date = EXCLUDED.last_review_date,
        updated_at = CURRENT_TIMESTAMP;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- TRIGGERS
-- ============================================================================

-- Updated_at triggers for all tables
CREATE TRIGGER update_providers_updated_at BEFORE UPDATE ON providers FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_hotels_updated_at BEFORE UPDATE ON hotels FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_reviewer_infos_updated_at BEFORE UPDATE ON reviewer_infos FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_reviews_updated_at BEFORE UPDATE ON reviews FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_review_summaries_updated_at BEFORE UPDATE ON review_summaries FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_review_processing_statuses_updated_at BEFORE UPDATE ON review_processing_statuses FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_hotel_chains_updated_at BEFORE UPDATE ON hotel_chains FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Audit triggers (optional - can be enabled as needed)
-- CREATE TRIGGER audit_providers AFTER INSERT OR UPDATE OR DELETE ON providers FOR EACH ROW EXECUTE FUNCTION create_audit_log();
-- CREATE TRIGGER audit_hotels AFTER INSERT OR UPDATE OR DELETE ON hotels FOR EACH ROW EXECUTE FUNCTION create_audit_log();
-- CREATE TRIGGER audit_reviews AFTER INSERT OR UPDATE OR DELETE ON reviews FOR EACH ROW EXECUTE FUNCTION create_audit_log();

-- ============================================================================
-- VIEWS FOR COMMON QUERIES
-- ============================================================================

-- View for hotel statistics
CREATE VIEW hotel_statistics AS
SELECT 
    h.id,
    h.name,
    h.city,
    h.country,
    h.star_rating,
    rs.total_reviews,
    rs.average_rating,
    rs.avg_service_rating,
    rs.avg_cleanliness_rating,
    rs.avg_location_rating,
    rs.avg_value_rating,
    rs.first_review_date,
    rs.last_review_date,
    hc.name as chain_name
FROM hotels h
LEFT JOIN review_summaries rs ON h.id = rs.hotel_id
LEFT JOIN hotel_chains hc ON h.chain_id = hc.id
WHERE h.deleted_at IS NULL;

-- View for provider statistics
CREATE VIEW provider_statistics AS
SELECT 
    p.id,
    p.name,
    p.is_active,
    COUNT(r.id) as total_reviews,
    ROUND(AVG(r.rating), 2) as avg_rating,
    COUNT(DISTINCT r.hotel_id) as unique_hotels,
    COUNT(DISTINCT r.reviewer_info_id) as unique_reviewers,
    MIN(r.review_date) as first_review_date,
    MAX(r.review_date) as last_review_date
FROM providers p
LEFT JOIN reviews r ON p.id = r.provider_id AND r.deleted_at IS NULL
WHERE p.deleted_at IS NULL
GROUP BY p.id, p.name, p.is_active;

-- View for recent reviews with hotel and provider info
CREATE VIEW recent_reviews AS
SELECT 
    r.id,
    r.rating,
    r.title,
    r.comment,
    r.review_date,
    r.language,
    r.sentiment,
    h.name as hotel_name,
    h.city as hotel_city,
    h.country as hotel_country,
    p.name as provider_name,
    ri.name as reviewer_name
FROM reviews r
JOIN hotels h ON r.hotel_id = h.id
JOIN providers p ON r.provider_id = p.id
LEFT JOIN reviewer_infos ri ON r.reviewer_info_id = ri.id
WHERE r.deleted_at IS NULL AND h.deleted_at IS NULL AND p.deleted_at IS NULL
ORDER BY r.review_date DESC;

-- ============================================================================
-- SAMPLE DATA (for development/testing)
-- ============================================================================

-- Insert sample providers
INSERT INTO providers (name, base_url, is_active) VALUES
('Booking.com', 'https://www.booking.com', true),
('Expedia', 'https://www.expedia.com', true),
('Hotels.com', 'https://www.hotels.com', true),
('TripAdvisor', 'https://www.tripadvisor.com', true),
('Agoda', 'https://www.agoda.com', true)
ON CONFLICT (name) DO NOTHING;

-- Insert sample hotel chains
INSERT INTO hotel_chains (name, description, headquarters_country) VALUES
('Marriott International', 'American multinational hospitality company', 'USA'),
('Hilton Worldwide', 'American multinational hospitality company', 'USA'),
('InterContinental Hotels Group', 'British multinational hospitality company', 'UK'),
('Accor', 'French multinational hospitality company', 'France'),
('Wyndham Hotels & Resorts', 'American hotel company', 'USA')
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS AND SECURITY
-- ============================================================================

-- Create application user (if not exists)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'hotel_reviews_app') THEN
        CREATE ROLE hotel_reviews_app WITH LOGIN PASSWORD 'secure_password_change_in_production';
    END IF;
END
$$;

-- Grant permissions
GRANT USAGE ON SCHEMA public TO hotel_reviews_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO hotel_reviews_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO hotel_reviews_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO hotel_reviews_app;

-- Create read-only user for analytics
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'hotel_reviews_readonly') THEN
        CREATE ROLE hotel_reviews_readonly WITH LOGIN PASSWORD 'readonly_password_change_in_production';
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO hotel_reviews_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO hotel_reviews_readonly;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO hotel_reviews_readonly;

-- ============================================================================
-- MIGRATION COMPLETION
-- ============================================================================

-- Insert migration record
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_migrations (version) VALUES ('001_initial_schema')
ON CONFLICT (version) DO NOTHING;

-- Final message
DO $$
BEGIN
    RAISE NOTICE 'Migration 001_initial_schema completed successfully';
    RAISE NOTICE 'Database schema initialized for Hotel Reviews Microservice';
END $$;