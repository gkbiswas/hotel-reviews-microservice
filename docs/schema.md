# Database Schema Documentation

## Overview

The Hotel Reviews Microservice uses PostgreSQL as the primary database with a carefully designed schema that supports high performance, data integrity, and scalability.

## Entity Relationship Diagram

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│    providers    │    │     hotels      │    │    reviews      │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ id (PK)         │    │ id (PK)         │    │ id (PK)         │
│ name            │    │ name            │    │ hotel_id (FK)   │
│ api_endpoint    │    │ description     │    │ provider_id (FK)│
│ api_key         │    │ address         │    │ external_id     │
│ rate_limit      │    │ city            │    │ title           │
│ active          │    │ country         │    │ content         │
│ created_at      │    │ postal_code     │    │ rating          │
│ updated_at      │    │ latitude        │    │ language        │
└─────────────────┘    │ longitude       │    │ author_name     │
                       │ phone           │    │ author_location │
                       │ email           │    │ review_date     │
                       │ website         │    │ sentiment_score │
                       │ amenities       │    │ created_at      │
                       │ rating          │    │ updated_at      │
                       │ price_range     │    │ deleted_at      │
                       │ created_at      │    └─────────────────┘
                       │ updated_at      │             │
                       │ deleted_at      │             │
                       └─────────────────┘             │
                                │                      │
                                └──────────────────────┘

┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ review_batches  │    │ processing_jobs │    │    audit_log    │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ id (PK)         │    │ id (PK)         │    │ id (PK)         │
│ provider_id (FK)│    │ batch_id (FK)   │    │ entity_type     │
│ file_url        │    │ status          │    │ entity_id       │
│ total_records   │    │ started_at      │    │ action          │
│ processed       │    │ completed_at    │    │ old_values      │
│ failed          │    │ error_message   │    │ new_values      │
│ status          │    │ retry_count     │    │ user_id         │
│ created_at      │    │ created_at      │    │ ip_address      │
│ updated_at      │    │ updated_at      │    │ created_at      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Core Tables

### 1. providers

Stores information about review data providers (Booking.com, Expedia, etc.)

```sql
CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    api_endpoint VARCHAR(512),
    api_key VARCHAR(255),
    rate_limit INTEGER DEFAULT 100,
    active BOOLEAN DEFAULT true,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_providers_active ON providers(active);
CREATE INDEX idx_providers_name ON providers(name);
```

### 2. hotels

Central hotel registry with comprehensive hotel information

```sql
CREATE TABLE hotels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    address TEXT,
    city VARCHAR(255),
    state VARCHAR(100),
    country VARCHAR(100),
    postal_code VARCHAR(20),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    phone VARCHAR(50),
    email VARCHAR(255),
    website VARCHAR(512),
    amenities TEXT[],
    rating DECIMAL(3, 2) DEFAULT 0.0,
    rating_count INTEGER DEFAULT 0,
    price_range VARCHAR(10),
    star_rating INTEGER,
    chain_id UUID,
    brand VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes
CREATE INDEX idx_hotels_city ON hotels(city) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_country ON hotels(country) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_rating ON hotels(rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_location ON hotels USING GIST (
    ll_to_earth(latitude, longitude)
) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_external_id ON hotels(external_id);
CREATE INDEX idx_hotels_amenities ON hotels USING GIN(amenities);
CREATE UNIQUE INDEX idx_hotels_name_city ON hotels(name, city) 
    WHERE deleted_at IS NULL;
```

### 3. reviews

Core review data with comprehensive review information

```sql
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hotel_id UUID NOT NULL REFERENCES hotels(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    external_id VARCHAR(255),
    title VARCHAR(500),
    content TEXT NOT NULL,
    rating DECIMAL(3, 2) NOT NULL CHECK (rating >= 0 AND rating <= 5),
    language VARCHAR(10) DEFAULT 'en',
    author_name VARCHAR(255),
    author_location VARCHAR(255),
    review_date TIMESTAMP WITH TIME ZONE NOT NULL,
    sentiment_score DECIMAL(3, 2),
    helpful_count INTEGER DEFAULT 0,
    verified BOOLEAN DEFAULT false,
    response TEXT,
    response_date TIMESTAMP WITH TIME ZONE,
    tags TEXT[],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes
CREATE INDEX idx_reviews_hotel_id ON reviews(hotel_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_provider_id ON reviews(provider_id);
CREATE INDEX idx_reviews_rating ON reviews(rating) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_review_date ON reviews(review_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_language ON reviews(language) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_sentiment ON reviews(sentiment_score) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_external_id ON reviews(provider_id, external_id);
CREATE INDEX idx_reviews_content_search ON reviews USING GIN(to_tsvector('english', content));
CREATE INDEX idx_reviews_title_search ON reviews USING GIN(to_tsvector('english', title));
CREATE INDEX idx_reviews_tags ON reviews USING GIN(tags);
CREATE UNIQUE INDEX idx_reviews_unique_provider_external 
    ON reviews(provider_id, external_id) WHERE deleted_at IS NULL;
```

### 4. review_batches

Tracks bulk review processing jobs

```sql
CREATE TABLE review_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id),
    file_url VARCHAR(1024) NOT NULL,
    file_size BIGINT,
    total_records INTEGER DEFAULT 0,
    processed_records INTEGER DEFAULT 0,
    failed_records INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    callback_url VARCHAR(1024),
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_review_batches_provider_id ON review_batches(provider_id);
CREATE INDEX idx_review_batches_status ON review_batches(status);
CREATE INDEX idx_review_batches_created_at ON review_batches(created_at);
```

### 5. processing_jobs

Individual job tracking for async processing

```sql
CREATE TABLE processing_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID REFERENCES review_batches(id),
    job_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    payload JSONB NOT NULL,
    result JSONB,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    scheduled_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_processing_jobs_status ON processing_jobs(status);
CREATE INDEX idx_processing_jobs_batch_id ON processing_jobs(batch_id);
CREATE INDEX idx_processing_jobs_scheduled_at ON processing_jobs(scheduled_at);
CREATE INDEX idx_processing_jobs_job_type ON processing_jobs(job_type);
```

### 6. audit_log

Comprehensive audit trail for all changes

```sql
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    old_values JSONB,
    new_values JSONB,
    user_id UUID,
    user_email VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX idx_audit_log_action ON audit_log(action);
```

## Supporting Tables

### 7. users

User management and authentication

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    role VARCHAR(50) DEFAULT 'user',
    verified BOOLEAN DEFAULT false,
    last_login TIMESTAMP WITH TIME ZONE,
    login_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_role ON users(role) WHERE deleted_at IS NULL;
```

### 8. api_keys

API key management for external access

```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    key_prefix VARCHAR(20) NOT NULL,
    permissions TEXT[],
    rate_limit INTEGER DEFAULT 1000,
    last_used TIMESTAMP WITH TIME ZONE,
    usage_count BIGINT DEFAULT 0,
    active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_active ON api_keys(active);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
```

### 9. cache_invalidation

Cache invalidation tracking

```sql
CREATE TABLE cache_invalidation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cache_key VARCHAR(255) NOT NULL,
    pattern VARCHAR(255),
    reason VARCHAR(255),
    triggered_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_cache_invalidation_cache_key ON cache_invalidation(cache_key);
CREATE INDEX idx_cache_invalidation_created_at ON cache_invalidation(created_at);
```

## Views

### hotel_stats

Aggregated hotel statistics for analytics

```sql
CREATE VIEW hotel_stats AS
SELECT 
    h.id,
    h.name,
    h.city,
    h.country,
    COUNT(r.id) as review_count,
    AVG(r.rating) as average_rating,
    MIN(r.review_date) as first_review_date,
    MAX(r.review_date) as last_review_date,
    COUNT(CASE WHEN r.rating >= 4.0 THEN 1 END) as positive_reviews,
    COUNT(CASE WHEN r.rating <= 2.0 THEN 1 END) as negative_reviews,
    AVG(r.sentiment_score) as average_sentiment
FROM hotels h
LEFT JOIN reviews r ON h.id = r.hotel_id AND r.deleted_at IS NULL
WHERE h.deleted_at IS NULL
GROUP BY h.id, h.name, h.city, h.country;
```

### recent_reviews

Recent reviews with hotel information

```sql
CREATE VIEW recent_reviews AS
SELECT 
    r.id,
    r.title,
    r.rating,
    r.review_date,
    r.author_name,
    h.name as hotel_name,
    h.city,
    h.country,
    p.name as provider_name
FROM reviews r
JOIN hotels h ON r.hotel_id = h.id
JOIN providers p ON r.provider_id = p.id
WHERE r.deleted_at IS NULL 
    AND h.deleted_at IS NULL
ORDER BY r.review_date DESC;
```

## Functions and Triggers

### update_hotel_rating()

Automatically updates hotel rating when reviews change

```sql
CREATE OR REPLACE FUNCTION update_hotel_rating()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE hotels SET 
        rating = (
            SELECT COALESCE(AVG(rating), 0)
            FROM reviews 
            WHERE hotel_id = COALESCE(NEW.hotel_id, OLD.hotel_id)
                AND deleted_at IS NULL
        ),
        rating_count = (
            SELECT COUNT(*)
            FROM reviews 
            WHERE hotel_id = COALESCE(NEW.hotel_id, OLD.hotel_id)
                AND deleted_at IS NULL
        ),
        updated_at = NOW()
    WHERE id = COALESCE(NEW.hotel_id, OLD.hotel_id);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_hotel_rating
    AFTER INSERT OR UPDATE OR DELETE ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_hotel_rating();
```

### audit_trigger()

Automatic audit logging for data changes

```sql
CREATE OR REPLACE FUNCTION audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    audit_action VARCHAR(10);
    old_data JSONB;
    new_data JSONB;
BEGIN
    IF TG_OP = 'DELETE' THEN
        audit_action = 'DELETE';
        old_data = to_jsonb(OLD);
        new_data = NULL;
    ELSIF TG_OP = 'UPDATE' THEN
        audit_action = 'UPDATE';
        old_data = to_jsonb(OLD);
        new_data = to_jsonb(NEW);
    ELSIF TG_OP = 'INSERT' THEN
        audit_action = 'INSERT';
        old_data = NULL;
        new_data = to_jsonb(NEW);
    END IF;
    
    INSERT INTO audit_log (
        entity_type,
        entity_id,
        action,
        old_values,
        new_values
    ) VALUES (
        TG_TABLE_NAME,
        COALESCE(NEW.id, OLD.id),
        audit_action,
        old_data,
        new_data
    );
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Apply audit trigger to important tables
CREATE TRIGGER audit_reviews
    AFTER INSERT OR UPDATE OR DELETE ON reviews
    FOR EACH ROW EXECUTE FUNCTION audit_trigger();

CREATE TRIGGER audit_hotels
    AFTER INSERT OR UPDATE OR DELETE ON hotels
    FOR EACH ROW EXECUTE FUNCTION audit_trigger();
```

## Indexes and Performance

### Performance Considerations

1. **Composite Indexes**: Used for common query patterns
2. **Partial Indexes**: Include WHERE clauses for soft deletes
3. **GIN Indexes**: For full-text search and array operations
4. **GIST Indexes**: For geographical queries

### Query Optimization

1. **Review Searches**: Full-text search on content and title
2. **Hotel Lookups**: City, rating, and location-based searches
3. **Analytics**: Pre-aggregated views for common metrics
4. **Audit Trail**: Time-based partitioning for large volumes

## Data Types and Constraints

### UUID Strategy

All primary keys use UUID v4 for:
- Global uniqueness across distributed systems
- Security (non-sequential IDs)
- Future-proofing for microservices

### Soft Deletes

Important tables use `deleted_at` timestamps:
- Maintains referential integrity
- Enables data recovery
- Supports audit requirements

### JSON Columns

JSONB columns for flexible data:
- `config` in providers
- `metadata` in reviews
- `payload` and `result` in processing_jobs

## Migration Strategy

### Version Control

Database changes are managed through numbered migrations:

```
migrations/
├── 001_initial_schema.up.sql
├── 001_initial_schema.down.sql
├── 002_add_audit_log.up.sql
├── 002_add_audit_log.down.sql
└── ...
```

### Deployment Process

1. **Backup**: Full database backup before migration
2. **Test**: Run migrations on staging environment
3. **Deploy**: Apply migrations during maintenance window
4. **Verify**: Validate data integrity post-migration
5. **Rollback**: Ready rollback plan for issues

## Monitoring and Maintenance

### Database Health

Key metrics to monitor:
- Connection pool usage
- Query performance (slow query log)
- Index usage statistics
- Table sizes and growth
- Cache hit ratios

### Maintenance Tasks

1. **Daily**: Update table statistics
2. **Weekly**: Analyze and vacuum tables
3. **Monthly**: Review and optimize indexes
4. **Quarterly**: Partition maintenance for audit logs

This schema design supports the microservice's requirements for high performance, data integrity, and scalability while maintaining flexibility for future enhancements.