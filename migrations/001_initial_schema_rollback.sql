-- ============================================================================
-- Migration Rollback: 001_initial_schema_rollback.sql
-- Description: Rollback initial database schema for Hotel Reviews Microservice
-- Version: 1.0.0
-- Created: 2024-01-01
-- ============================================================================

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '001_initial_schema';

-- ============================================================================
-- DROP TRIGGERS
-- ============================================================================

-- Drop updated_at triggers
DROP TRIGGER IF EXISTS update_providers_updated_at ON providers;
DROP TRIGGER IF EXISTS update_hotels_updated_at ON hotels;
DROP TRIGGER IF EXISTS update_reviewer_infos_updated_at ON reviewer_infos;
DROP TRIGGER IF EXISTS update_reviews_updated_at ON reviews;
DROP TRIGGER IF EXISTS update_review_summaries_updated_at ON review_summaries;
DROP TRIGGER IF EXISTS update_review_processing_statuses_updated_at ON review_processing_statuses;
DROP TRIGGER IF EXISTS update_hotel_chains_updated_at ON hotel_chains;

-- Drop audit triggers (if they were enabled)
DROP TRIGGER IF EXISTS audit_providers ON providers;
DROP TRIGGER IF EXISTS audit_hotels ON hotels;
DROP TRIGGER IF EXISTS audit_reviews ON reviews;

-- ============================================================================
-- DROP VIEWS
-- ============================================================================

DROP VIEW IF EXISTS hotel_statistics;
DROP VIEW IF EXISTS provider_statistics;
DROP VIEW IF EXISTS recent_reviews;

-- ============================================================================
-- DROP FUNCTIONS
-- ============================================================================

DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS create_audit_log();
DROP FUNCTION IF EXISTS update_review_summary(UUID);

-- ============================================================================
-- DROP TABLES (in reverse order due to foreign keys)
-- ============================================================================

-- Drop dependent tables first
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS review_processing_statuses;
DROP TABLE IF EXISTS review_summaries;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS reviewer_infos;
DROP TABLE IF EXISTS hotels;
DROP TABLE IF EXISTS hotel_chains;
DROP TABLE IF EXISTS providers;

-- ============================================================================
-- DROP CUSTOM TYPES
-- ============================================================================

DROP TYPE IF EXISTS processing_status;
DROP TYPE IF EXISTS sentiment_type;
DROP TYPE IF EXISTS trip_type;

-- ============================================================================
-- DROP EXTENSIONS (optional - be careful with this in production)
-- ============================================================================

-- Note: Only drop extensions if they're not used by other applications
-- DROP EXTENSION IF EXISTS "btree_gist";
-- DROP EXTENSION IF EXISTS "btree_gin";
-- DROP EXTENSION IF EXISTS "pg_trgm";
-- DROP EXTENSION IF EXISTS "uuid-ossp";

-- ============================================================================
-- REVOKE PERMISSIONS
-- ============================================================================

-- Revoke permissions from application user
REVOKE ALL ON SCHEMA public FROM hotel_reviews_app;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM hotel_reviews_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM hotel_reviews_app;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM hotel_reviews_app;

-- Revoke permissions from readonly user
REVOKE ALL ON SCHEMA public FROM hotel_reviews_readonly;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM hotel_reviews_readonly;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM hotel_reviews_readonly;

-- ============================================================================
-- DROP USERS (optional - be careful with this in production)
-- ============================================================================

-- Note: Only drop users if they're not used by other applications
-- DROP ROLE IF EXISTS hotel_reviews_app;
-- DROP ROLE IF EXISTS hotel_reviews_readonly;

-- ============================================================================
-- ROLLBACK COMPLETION
-- ============================================================================

-- Final message
DO $$
BEGIN
    RAISE NOTICE 'Migration 001_initial_schema rollback completed successfully';
    RAISE NOTICE 'Database schema removed for Hotel Reviews Microservice';
END $$;