-- Hotel Reviews Database Initialization Script
-- This script runs when the PostgreSQL container starts for the first time

-- Create database if it doesn't exist (this is handled by POSTGRES_DB env var)
-- CREATE DATABASE IF NOT EXISTS hotel_reviews;

-- Connect to the database
\c hotel_reviews;

-- Create extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- Create custom types
DO $$ BEGIN
    CREATE TYPE processing_status AS ENUM ('pending', 'processing', 'completed', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create indexes for better performance (these will be created by GORM migrations too)
-- But having them here ensures they exist early

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create a function to generate random UUID for testing
CREATE OR REPLACE FUNCTION generate_test_uuid() RETURNS UUID AS $$
BEGIN
    RETURN uuid_generate_v4();
END;
$$ LANGUAGE plpgsql;

-- Create a function to calculate review statistics
CREATE OR REPLACE FUNCTION calculate_review_stats(hotel_uuid UUID)
RETURNS TABLE(
    total_reviews BIGINT,
    avg_rating DECIMAL(3,2),
    rating_1 BIGINT,
    rating_2 BIGINT,
    rating_3 BIGINT,
    rating_4 BIGINT,
    rating_5 BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*) as total_reviews,
        ROUND(AVG(rating), 2) as avg_rating,
        COUNT(*) FILTER (WHERE rating >= 1 AND rating < 2) as rating_1,
        COUNT(*) FILTER (WHERE rating >= 2 AND rating < 3) as rating_2,
        COUNT(*) FILTER (WHERE rating >= 3 AND rating < 4) as rating_3,
        COUNT(*) FILTER (WHERE rating >= 4 AND rating < 5) as rating_4,
        COUNT(*) FILTER (WHERE rating >= 5) as rating_5
    FROM reviews 
    WHERE hotel_id = hotel_uuid AND deleted_at IS NULL;
END;
$$ LANGUAGE plpgsql;

-- Create a function for full-text search
CREATE OR REPLACE FUNCTION search_reviews(search_term TEXT)
RETURNS TABLE(
    review_id UUID,
    hotel_name TEXT,
    comment TEXT,
    rating DECIMAL(3,2),
    review_date TIMESTAMP,
    relevance REAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        r.id as review_id,
        h.name as hotel_name,
        r.comment,
        r.rating,
        r.review_date,
        similarity(r.comment, search_term) as relevance
    FROM reviews r
    JOIN hotels h ON r.hotel_id = h.id
    WHERE r.comment % search_term
       OR r.title % search_term
       OR h.name % search_term
    ORDER BY relevance DESC
    LIMIT 100;
END;
$$ LANGUAGE plpgsql;

-- Create some utility views for common queries
CREATE OR REPLACE VIEW hotel_review_summary AS
SELECT 
    h.id as hotel_id,
    h.name as hotel_name,
    h.city,
    h.country,
    h.star_rating,
    COUNT(r.id) as total_reviews,
    ROUND(AVG(r.rating), 2) as avg_rating,
    MIN(r.review_date) as first_review_date,
    MAX(r.review_date) as last_review_date
FROM hotels h
LEFT JOIN reviews r ON h.id = r.hotel_id AND r.deleted_at IS NULL
WHERE h.deleted_at IS NULL
GROUP BY h.id, h.name, h.city, h.country, h.star_rating;

CREATE OR REPLACE VIEW provider_stats AS
SELECT 
    p.id as provider_id,
    p.name as provider_name,
    COUNT(r.id) as total_reviews,
    ROUND(AVG(r.rating), 2) as avg_rating,
    COUNT(DISTINCT r.hotel_id) as unique_hotels,
    MIN(r.review_date) as first_review_date,
    MAX(r.review_date) as last_review_date
FROM providers p
LEFT JOIN reviews r ON p.id = r.provider_id AND r.deleted_at IS NULL
WHERE p.deleted_at IS NULL
GROUP BY p.id, p.name;

-- Create some sample data for testing (optional)
-- This will only run in development/testing environments

INSERT INTO providers (name, base_url, is_active) VALUES
('Booking.com', 'https://www.booking.com', true),
('Expedia', 'https://www.expedia.com', true),
('Hotels.com', 'https://www.hotels.com', true),
('TripAdvisor', 'https://www.tripadvisor.com', true),
('Agoda', 'https://www.agoda.com', true)
ON CONFLICT (name) DO NOTHING;

-- Create some sample hotels for testing
INSERT INTO hotels (name, address, city, country, star_rating, description) VALUES
('Grand Plaza Hotel', '123 Main St', 'New York', 'USA', 4, 'Luxury hotel in downtown Manhattan'),
('Seaside Resort', '456 Ocean Ave', 'Miami', 'USA', 5, 'Beautiful beachfront resort'),
('Mountain Lodge', '789 Hill Rd', 'Denver', 'USA', 3, 'Cozy mountain retreat'),
('City Center Inn', '321 Business St', 'Chicago', 'USA', 3, 'Convenient downtown location'),
('Lakeside Hotel', '654 Lake Dr', 'Seattle', 'USA', 4, 'Scenic lakefront property')
ON CONFLICT (name) DO NOTHING;

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_reviews_hotel_rating ON reviews(hotel_id, rating) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_provider_date ON reviews(provider_id, review_date) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_date ON reviews(review_date) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_rating ON reviews(rating) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_comment_gin ON reviews USING gin(comment gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_hotels_name_gin ON hotels USING gin(name gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_hotels_city_country ON hotels(city, country) WHERE deleted_at IS NULL;

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO postgres;

-- Log completion
DO $$
BEGIN
    RAISE NOTICE 'Hotel Reviews database initialization completed successfully';
END $$;