# ADR-004: Data Storage Strategy

* Status: Accepted
* Date: 2024-01-15
* Authors: Goutam Kumar Biswas
* Deciders: Technical Leadership

## Context

The hotel reviews microservice needs to handle multiple types of data with different characteristics:

**Structured Data:**
- Hotel information (name, location, amenities)
- Review metadata (ratings, dates, user IDs)
- User profiles and authentication data
- Provider information

**Semi-structured Data:**
- Review content (comments, free-text)
- Processing metadata and status

**File Data:**
- Bulk review uploads (JSON/CSV files)
- Review attachments (images, documents)

**Cache Data:**
- Frequently accessed review summaries
- Session data
- API rate limiting data

We need a storage strategy that supports:
- ACID transactions for critical data
- High-performance reads for review queries
- Scalable file storage for bulk uploads
- Fast caching for performance optimization

## Decision

We will adopt a **polyglot persistence approach** with:

1. **PostgreSQL** as the primary transactional database
2. **Redis** for caching and session storage  
3. **Amazon S3** for file storage and bulk data processing
4. **Repository pattern** to abstract storage implementations

### Data Distribution:
```
PostgreSQL:
├── Users and authentication
├── Hotels and providers  
├── Reviews (structured metadata)
└── Processing status and audit logs

Redis:
├── Review summary cache
├── Session data
├── Rate limiting counters
└── Background job queues

Amazon S3:
├── Bulk review upload files
├── Review attachments
├── Backup data
└── Analytics exports
```

## Rationale

**PostgreSQL as Primary Database:**
- **ACID Compliance**: Ensures data consistency for critical operations
- **Rich Query Support**: Complex joins and aggregations for analytics
- **JSON Support**: Handles semi-structured data when needed
- **Mature Ecosystem**: Well-understood operational characteristics
- **Horizontal Scaling**: Supports read replicas for scaling reads

**Redis for Caching:**
- **High Performance**: Sub-millisecond access times
- **Data Structures**: Native support for counters, sets, sorted sets
- **Pub/Sub**: Real-time notifications and events
- **TTL Support**: Automatic data expiration
- **Memory Efficiency**: Optimized for caching use cases

**S3 for File Storage:**
- **Scalability**: Virtually unlimited storage capacity
- **Durability**: 99.999999999% (11 9's) durability
- **Integration**: Native AWS ecosystem integration
- **Cost Effective**: Pay-per-use storage pricing
- **Event Processing**: S3 events can trigger processing workflows

**Alternatives Considered:**
- **MongoDB**: Rejected due to eventual consistency and team expertise with SQL
- **Single Database**: Rejected due to performance and scalability concerns
- **DynamoDB**: Rejected due to cost and limited query flexibility
- **File System Storage**: Rejected due to scalability and durability concerns

## Consequences

### Positive
- **Performance**: Optimized storage for each data type
- **Scalability**: Each storage system can scale independently
- **Reliability**: Distributed storage reduces single points of failure
- **Cost Optimization**: Pay for appropriate storage characteristics
- **Flexibility**: Can evolve storage strategy per component

### Negative
- **Complexity**: Multiple storage systems to manage and monitor
- **Consistency**: Eventual consistency between different storage systems
- **Operational Overhead**: More systems to backup, monitor, and maintain
- **Data Synchronization**: Need to keep cache in sync with primary data

### Neutral
- **Development**: Developers need to understand multiple storage paradigms
- **Testing**: More complex test setup with multiple storage systems
- **Deployment**: Container orchestration needs to manage multiple services

## Implementation

### Repository Pattern Implementation:
```go
// Domain interfaces
type ReviewRepository interface {
    Create(ctx context.Context, review *Review) error
    GetByID(ctx context.Context, id uuid.UUID) (*Review, error)
    GetByHotel(ctx context.Context, hotelID uuid.UUID) ([]Review, error)
}

type CacheService interface {
    GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*ReviewSummary, error)
    SetReviewSummary(ctx context.Context, hotelID uuid.UUID, summary *ReviewSummary) error
}

type S3Client interface {
    UploadFile(ctx context.Context, bucket, key string, data io.Reader) error
    DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error)
}
```

### Database Schema Design:
```sql
-- Core entities
CREATE TABLE hotels (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    location JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE reviews (
    id UUID PRIMARY KEY,
    hotel_id UUID REFERENCES hotels(id),
    provider_id UUID REFERENCES providers(id),
    rating DECIMAL(2,1) CHECK (rating >= 1.0 AND rating <= 5.0),
    comment TEXT,
    review_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    INDEX idx_reviews_hotel_id (hotel_id),
    INDEX idx_reviews_rating (rating),
    INDEX idx_reviews_date (review_date)
);
```

### Caching Strategy:
```go
func (s *Service) GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*ReviewSummary, error) {
    // Try cache first
    if summary, err := s.cache.GetReviewSummary(ctx, hotelID); err == nil {
        return summary, nil
    }
    
    // Fallback to database
    summary, err := s.repo.GetReviewSummaryByHotelID(ctx, hotelID)
    if err != nil {
        return nil, err
    }
    
    // Update cache for next time
    go func() {
        s.cache.SetReviewSummary(context.Background(), hotelID, summary)
    }()
    
    return summary, nil
}
```

### File Processing Workflow:
```go
func (s *Service) ProcessReviewFile(ctx context.Context, fileURL string) error {
    // Download from S3
    reader, err := s.s3Client.DownloadFile(ctx, bucket, key)
    if err != nil {
        return fmt.Errorf("failed to download file: %w", err)
    }
    defer reader.Close()
    
    // Process and store in PostgreSQL
    return s.processJSONFile(ctx, reader)
}
```

### Data Consistency Strategy:
- **Write-through Cache**: Update database first, then cache
- **Cache Invalidation**: Remove cache entries when data changes
- **Eventual Consistency**: Accept temporary inconsistency for performance
- **Idempotent Operations**: Support safe retry of failed operations

### Backup and Recovery:
- **PostgreSQL**: Daily automated backups with point-in-time recovery
- **Redis**: Persistent snapshots for cache warmup
- **S3**: Cross-region replication for critical files

## Related Decisions

- [ADR-001](001-clean-architecture-adoption.md): Repository pattern aligns with clean architecture
- [ADR-002](002-dependency-injection-strategy.md): Storage dependencies injected via interfaces
- [ADR-008](008-caching-strategy.md): Detailed caching implementation
- [ADR-009](009-file-processing-architecture.md): S3-based file processing

## Notes

This polyglot persistence approach balances performance, consistency, and operational complexity. The strategy can evolve as the system scales:

**Future Considerations:**
- **Read Replicas**: PostgreSQL read replicas for scaling read operations
- **Sharding**: Horizontal partitioning of review data by hotel or date
- **Event Sourcing**: For audit trails and complex business event tracking
- **Search Engine**: Elasticsearch for full-text search capabilities

**Monitoring Requirements:**
- Database performance metrics (connection pools, query times)
- Cache hit rates and memory usage
- S3 request metrics and error rates
- Data consistency monitoring between systems

The implementation follows cloud-native patterns and is designed for containerized deployment with proper service discovery and configuration management.