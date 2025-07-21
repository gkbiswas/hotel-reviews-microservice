# Comprehensive Integration Tests

This directory contains enterprise-grade integration tests for the hotel reviews microservice. The test suite covers complete business workflows, performance testing, security validation, and data integrity checks.

## Test Architecture

### Three-Tier Integration Testing

1. **Comprehensive Integration Tests** (`comprehensive_integration_test.go`)
   - Complete API workflow testing
   - CRUD operations for all entities
   - Search and analytics functionality
   - Cache performance verification
   - Security middleware validation

2. **File Processing Integration Tests** (`file_processing_integration_test.go`)
   - End-to-end file upload and processing
   - Large file processing (100+ reviews)
   - File validation and error handling
   - Concurrent processing scenarios
   - Processing cancellation workflows

3. **End-to-End Business Workflow Tests** (`end_to_end_integration_test.go`)
   - Complete business scenario simulation
   - Multi-provider, multi-hotel data processing
   - Comprehensive analytics and reporting
   - Data consistency and integrity validation
   - Performance under load testing

## Test Infrastructure

### Containerized Test Environment

- **PostgreSQL 16**: Full database with GORM auto-migration
- **Redis 7**: Cache layer for performance testing and caching validation
- **LocalStack**: S3-compatible storage for file operations and upload testing
- **Testcontainers**: Automated container lifecycle management

### Test Utilities

- **TestRunner** (`test_runner.go`): Centralized test execution and container management
- **MetricsCollector**: Test execution metrics and performance tracking
- **Environment Validation**: Docker availability and system requirements checking

## Test Categories

### 1. API Integration Tests

**Hotel CRUD Operations**
- Create, read, update, delete hotels
- Validation of hotel data and constraints
- Multi-hotel scenarios with different characteristics

**Provider CRUD Operations**
- Provider management across multiple review sources
- Active/inactive provider handling
- Provider-specific data processing

**Review CRUD Operations**
- Individual review creation and management
- Bulk review operations
- Review validation and data integrity

### 2. Search and Analytics Tests

**Search Functionality**
- Text-based search across reviews and hotels
- Advanced filtering (rating, date, location, etc.)
- Search performance and relevance testing

**Analytics and Reporting**
- Overall system analytics and metrics
- Hotel-specific statistics and trends
- Provider performance analytics
- Review trend analysis and reporting

### 3. File Processing Tests

**File Upload and Processing**
- JSONL file upload to S3 storage
- Batch processing of review data
- Processing status tracking and monitoring

**Large File Handling**
- Processing files with 100+ reviews
- Memory efficiency and performance optimization
- Concurrent file processing scenarios

**Error Handling and Validation**
- Malformed JSON detection and handling
- Invalid data validation and rejection
- Processing failure recovery and retry logic

### 4. Performance and Load Tests

**Caching Performance**
- Cache hit/miss ratio verification
- Cache performance vs database queries
- Cache invalidation and consistency

**Concurrent Request Handling**
- Multi-user concurrent access patterns
- Rate limiting and throttling verification
- System stability under load

**Resource Management**
- Memory usage optimization
- Database connection pooling
- Container resource efficiency

### 5. Security and Validation Tests

**Input Validation**
- SQL injection attack prevention
- XSS protection and input sanitization
- Data type and constraint validation

**Security Middleware**
- CORS policy enforcement
- Security headers validation
- Rate limiting and DDoS protection

**Authentication and Authorization**
- API access control mechanisms
- User role and permission validation
- Secure data access patterns

### 6. Data Integrity Tests

**Referential Integrity**
- Hotel-review relationship validation
- Provider-review association verification
- Data consistency across operations

**Transaction Management**
- Atomic operation verification
- Rollback scenario testing
- Concurrent modification handling

**Aggregation Accuracy**
- Rating calculation verification
- Summary statistics accuracy
- Real-time vs cached data consistency

## Running Tests

### Prerequisites

- **Docker**: Container runtime for test infrastructure
- **Go 1.23+**: Programming language and runtime
- **Memory**: Minimum 4GB RAM for container orchestration
- **Disk Space**: 2GB for container images and test data
- **Network**: Internet access for pulling container images

### Quick Start

```bash
# Run all integration tests (recommended)
go test -v ./tests/integration/... -timeout 15m

# Run specific test suites
go test -v ./tests/integration/ -run TestComprehensiveIntegration -timeout 10m
go test -v ./tests/integration/ -run TestFileProcessingIntegration -timeout 10m
go test -v ./tests/integration/ -run TestCompleteBusinessWorkflow -timeout 15m

# Run with race detection for concurrency testing
go test -race -v ./tests/integration/... -timeout 20m

# Skip integration tests in CI/CD environments
go test -short ./tests/integration/...
```

### Test Configuration and Environment

**Automatic Test Environment Setup:**
- PostgreSQL 16 database with schema auto-migration
- Redis 7 cache server with performance optimization
- LocalStack S3-compatible storage simulation
- Testcontainers lifecycle management
- Comprehensive test data generation

**Environment Variables:**
```bash
# Skip tests in CI/CD environments
export CI=true

# Enable debug logging
export LOG_LEVEL=debug

# Customize test timeout
export TEST_TIMEOUT=20m
```

### Test Execution Patterns

**Development Workflow:**
```bash
# Quick smoke test
go test -v ./tests/integration/ -run TestComprehensiveIntegration/Hotel_CRUD -timeout 5m

# Full regression testing
go test -v ./tests/integration/... -timeout 30m

# Performance and load testing
go test -v ./tests/integration/ -run TestCompleteBusinessWorkflow/Performance -timeout 10m
```

**Continuous Integration:**
```bash
# Parallel test execution
go test -v ./tests/integration/... -parallel 4 -timeout 25m

# Test result reporting
go test -v ./tests/integration/... -json | tee test-results.json
```

### Test Data and Scenarios

**Business Scenarios Tested:**
- Multi-provider review aggregation (Booking.com, Expedia, Hotels.com)
- Hotel categorization (Luxury, Business, Budget, Boutique, Family)
- Review processing workflows (5-100+ reviews per file)
- Real-world rating distributions and comment patterns
- Comprehensive search and analytics use cases

**Data Volume Testing:**
- Small files: 5-10 reviews for functional testing
- Medium files: 25-50 reviews for performance validation
- Large files: 100+ reviews for scalability testing
- Concurrent processing: Multiple files simultaneously

## Performance Benchmarks

### Expected Performance Metrics

**API Response Times:**
- CRUD operations: < 100ms (95th percentile)
- Search queries: < 200ms (95th percentile)
- Analytics endpoints: < 500ms (95th percentile)
- Cache-enabled requests: < 50ms (95th percentile)

**File Processing Performance:**
- Small files (5-10 reviews): < 5 seconds
- Medium files (25-50 reviews): < 15 seconds
- Large files (100+ reviews): < 60 seconds
- Concurrent processing: 3-5 files simultaneously

**System Resource Usage:**
- Memory usage: < 512MB during normal operations
- CPU utilization: < 70% during peak processing
- Database connections: < 20 active connections
- Cache hit ratio: > 80% for frequently accessed data

## Error Handling and Resilience Testing

### Failure Scenarios Validated

**Infrastructure Failures:**
- Database connection interruptions
- Redis cache unavailability
- S3 storage temporary failures
- Network connectivity issues

**Data Integrity Challenges:**
- Malformed JSON in uploaded files
- Invalid rating values and constraints
- Missing required fields and relationships
- Duplicate review detection and handling

**Security Attack Simulations:**
- SQL injection attempts in search queries
- XSS payload injection in review content
- CORS policy violation attempts
- Rate limiting bypass attempts

### Resilience Patterns Tested

**Retry and Circuit Breaker:**
- Automatic retry for transient failures
- Exponential backoff with jitter
- Circuit breaker trip and recovery
- Graceful degradation under load

**Data Consistency:**
- Transaction rollback scenarios
- Concurrent modification handling
- Cache invalidation strategies
- Eventual consistency verification

## Monitoring and Observability

### Test Metrics Collection

**Execution Metrics:**
- Total test execution time and duration
- Pass/fail rates across test categories
- Resource utilization during testing
- Container startup and cleanup times

**Performance Metrics:**
- API response time distributions
- Database query performance analysis
- Cache hit/miss ratios and effectiveness
- File processing throughput measurements

**Quality Metrics:**
- Code coverage across integration scenarios
- Test flakiness and reliability scores
- Error rate and failure pattern analysis
- Security validation coverage metrics

## Test Maintenance and Evolution

### Continuous Improvement

**Automated Test Enhancement:**
- Performance regression detection
- Test data generation and variation
- Flaky test identification and resolution
- Coverage gap analysis and expansion

**Business Scenario Evolution:**
- New provider integration testing
- Additional hotel types and categories
- Enhanced analytics and reporting validation
- Advanced search feature verification

### Future Test Enhancements

**Planned Improvements:**
- Chaos engineering for fault injection
- Multi-region deployment testing
- API versioning and compatibility testing
- Advanced security penetration testing
- Kubernetes integration testing
- Microservice communication patterns

**Scalability Testing:**
- Database sharding and partitioning
- Horizontal scaling validation
- Load balancer effectiveness
- Auto-scaling behavior verification