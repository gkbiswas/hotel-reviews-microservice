# Integration Tests

This directory contains comprehensive integration tests for the hotel reviews microservice. The tests cover the complete file processing workflow from S3 file upload to database storage and API retrieval.

## Test Coverage

### Complete Workflow Test (`TestCompleteWorkflow`)

Tests the end-to-end workflow:

1. **Setup Phase**
   - PostgreSQL database with GORM auto-migration
   - Redis cache service
   - Kafka event streaming
   - LocalStack for S3 operations

2. **Workflow Steps**
   - Create a provider
   - Create a hotel
   - Upload test JSONL file to S3
   - Process the file through the API
   - Verify processing status
   - Retrieve reviews via API
   - Test cache functionality
   - Verify Kafka events

### Error Scenarios (`TestErrorScenarios`)

Tests various error conditions:
- Invalid S3 URLs
- Non-existent files
- Malformed JSON data
- Database connection errors
- Redis connection errors
- Kafka producer errors
- Concurrent processing
- Large file processing
- Rate limiting
- Duplicate review handling

### Edge Cases (`TestEdgeCases`)

Tests edge cases:
- Empty files
- Unicode characters
- Very long review text
- Invalid ratings
- Missing required fields
- SQL injection attempts
- XSS prevention
- Concurrent updates
- Transaction rollbacks
- Memory pressure

### Performance Tests

- Benchmark tests for the complete workflow
- Load testing scenarios
- Memory usage analysis

## Test Infrastructure

### Testcontainers

The tests use testcontainers to provide real infrastructure:

- **PostgreSQL**: Full database with GORM migrations
- **Redis**: Cache layer for performance testing
- **Kafka**: Event streaming for workflow verification
- **LocalStack**: S3-compatible storage for file operations

### Test Data

- Sample JSONL files with hotel reviews
- Multiple data formats and edge cases
- Various hotel and provider configurations

## Implementation Components

### Core Services

1. **Repository Layer**
   - GORM-based PostgreSQL repository
   - Batch operations for performance
   - Transaction support

2. **Cache Service**
   - Redis-based caching
   - Review summary caching
   - Cache invalidation strategies

3. **Event Publishing**
   - Kafka producer for domain events
   - Event serialization and partitioning
   - Dead letter queue handling

4. **File Processing**
   - JSON Lines parser
   - Batch processing with configurable sizes
   - Error handling and retry logic

5. **API Layer**
   - REST endpoints for all operations
   - Request validation
   - Response formatting

### Domain Models

All models include proper GORM tags for database operations:

- `Hotel`: Hotel information with relationships
- `Provider`: Review providers (Booking.com, etc.)
- `ReviewerInfo`: Reviewer profiles and statistics
- `Review`: Core review entity with ratings and metadata
- `ReviewSummary`: Aggregated hotel review statistics
- `ReviewProcessingStatus`: File processing tracking

## Running Tests

### Prerequisites

- Docker and Docker Compose
- Go 1.23+
- Internet connection for container images

### Execution

```bash
# Run all integration tests
go test -v ./tests/integration/...

# Run specific test
go test -v ./tests/integration/ -run TestCompleteWorkflow

# Run with race detection
go test -race -v ./tests/integration/...

# Run benchmarks
go test -bench=. ./tests/integration/...
```

### Test Configuration

Tests automatically:
- Start required containers
- Initialize database schemas
- Create test data
- Clean up resources after completion

## Error Handling and Resilience

The integration tests verify:

1. **Retry Mechanisms**
   - Automatic retries for transient failures
   - Exponential backoff strategies
   - Circuit breaker patterns

2. **Error Propagation**
   - Proper error messages
   - Status code handling
   - Event publishing on failures

3. **Resource Management**
   - Connection pooling
   - Memory usage limits
   - Graceful shutdowns

## Monitoring and Observability

Integration tests include:
- Metrics collection verification
- Log output validation
- Health check endpoints
- Performance benchmarking

## Future Enhancements

Planned improvements:
- Performance regression testing
- Chaos engineering scenarios
- Multi-region testing
- Security penetration testing
- API versioning tests