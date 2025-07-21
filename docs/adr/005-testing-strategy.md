# ADR-005: Testing Strategy

* Status: Accepted
* Date: 2024-01-15
* Authors: Goutam Kumar Biswas
* Deciders: Technical Leadership

## Context

The hotel reviews microservice requires a comprehensive testing strategy that:
- Ensures high code quality and reliability
- Supports confident refactoring and feature development
- Integrates with CI/CD pipeline for automated quality gates
- Balances test coverage with development velocity
- Supports multiple testing levels (unit, integration, contract, load)

Current challenges:
- Complex business logic requiring thorough validation
- Multiple external dependencies (PostgreSQL, Redis, S3)
- Asynchronous processing workflows
- Authentication and authorization requirements
- Need for performance validation under load

## Decision

We will implement a **multi-layered testing strategy** following the testing pyramid:

```
           🔺 Manual Testing
         /   \
        /  E2E \     (Few, High-Value)
       /Testing \
      /_________\
     /           \
    / Integration \   (Some, Key Workflows)
   /   Testing     \
  /________________\
 /                  \
/   Unit Testing     \  (Many, Fast, Isolated)
/____________________\
```

### Testing Levels:

1. **Unit Tests** (70% of tests): Fast, isolated, comprehensive coverage
2. **Integration Tests** (20% of tests): Key workflows with real dependencies  
3. **Contract Tests** (5% of tests): API contract validation
4. **End-to-End Tests** (5% of tests): Critical user journeys
5. **Performance Tests**: Load and stress testing for key endpoints

### Coverage Targets:
- **Overall Coverage**: 42% baseline → 80% target
- **Domain Layer**: 90%+ (business logic critical)
- **Application Layer**: 80%+ (handler and middleware logic)
- **Infrastructure Layer**: 60%+ (integration focused)

## Rationale

**Benefits of Multi-layered Testing:**
- **Fast Feedback**: Unit tests provide immediate feedback
- **Confidence**: Integration tests validate real-world scenarios
- **Regression Prevention**: Comprehensive coverage prevents bugs
- **Documentation**: Tests serve as living documentation
- **Refactoring Safety**: High coverage enables confident code changes

**Alternatives Considered:**
- **Testing only at integration level**: Rejected due to slow feedback loops
- **Manual testing only**: Rejected due to reliability and velocity concerns
- **End-to-end testing focus**: Rejected due to brittleness and maintenance cost

**Trade-offs:**
- Initial development overhead for test setup
- Maintenance burden for test suites
- CI/CD pipeline execution time
- Mock/stub maintenance complexity

## Consequences

### Positive
- High confidence in code changes and deployments
- Fast feedback loop for developers
- Reduced production bugs and incidents
- Better code design through test-driven development
- Living documentation through test cases
- Automated quality gates in CI/CD

### Negative
- Additional development time for writing tests
- Test maintenance overhead
- Increased CI/CD execution time
- Learning curve for testing best practices
- Mock implementation maintenance

### Neutral
- Testing tools and frameworks need to be standardized
- Test data management strategies required
- Team training on testing practices needed

## Implementation

### 1. Unit Testing Strategy

**Framework**: Go's built-in `testing` package with `testify` for assertions

**Patterns**:
```go
// Table-driven tests for multiple scenarios
func TestReviewService_ValidateReview(t *testing.T) {
    tests := []struct {
        name        string
        review      *Review
        expectError bool
        errorType   string
    }{
        {
            name: "valid review",
            review: &Review{
                Rating:  4.5,
                Comment: "Great hotel!",
                HotelID: uuid.New(),
            },
            expectError: false,
        },
        {
            name: "invalid rating too low",
            review: &Review{
                Rating:  0.5,
                Comment: "Terrible",
            },
            expectError: true,
            errorType:   "ValidationError",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := NewReviewService(nil, nil, nil)
            err := service.ValidateReview(context.Background(), tt.review)
            
            if tt.expectError {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorType)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

**Mock Strategy**:
```go
// Interface-based mocking
type MockReviewRepository struct {
    mock.Mock
}

func (m *MockReviewRepository) Create(ctx context.Context, review *Review) error {
    args := m.Called(ctx, review)
    return args.Error(0)
}

func TestReviewService_CreateReview(t *testing.T) {
    mockRepo := new(MockReviewRepository)
    mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*Review")).Return(nil)
    
    service := NewReviewService(mockRepo, nil, nil)
    err := service.CreateReview(context.Background(), &Review{})
    
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

### 2. Integration Testing Strategy

**Framework**: TestContainers for real dependencies

```go
//go:build integration

func TestReviewRepository_Integration(t *testing.T) {
    // Setup real PostgreSQL container
    ctx := context.Background()
    postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "postgres:15",
            ExposedPorts: []string{"5432/tcp"},
            Env: map[string]string{
                "POSTGRES_DB":       "testdb",
                "POSTGRES_USER":     "test",
                "POSTGRES_PASSWORD": "test",
            },
        },
        Started: true,
    })
    require.NoError(t, err)
    defer postgres.Terminate(ctx)
    
    // Get connection details
    host, _ := postgres.Host(ctx)
    port, _ := postgres.MappedPort(ctx, "5432")
    
    // Setup repository with real database
    db := setupTestDatabase(host, port.Port())
    repo := NewPostgreSQLRepository(db)
    
    // Test real database operations
    review := &Review{
        ID:      uuid.New(),
        Rating:  4.5,
        Comment: "Test review",
    }
    
    err = repo.Create(ctx, review)
    assert.NoError(t, err)
    
    retrieved, err := repo.GetByID(ctx, review.ID)
    assert.NoError(t, err)
    assert.Equal(t, review.Rating, retrieved.Rating)
}
```

### 3. Contract Testing Strategy

**API Contract Tests**:
```go
func TestReviewHandlers_APIContract(t *testing.T) {
    tests := []struct {
        name           string
        method         string
        endpoint       string
        body           string
        expectedStatus int
        expectedFields []string
    }{
        {
            name:           "create review success",
            method:         "POST",
            endpoint:       "/api/v1/reviews",
            body:           `{"hotel_id":"...","rating":4.5,"comment":"Great!"}`,
            expectedStatus: 201,
            expectedFields: []string{"id", "hotel_id", "rating", "comment", "created_at"},
        },
        {
            name:           "create review validation error",
            method:         "POST",
            endpoint:       "/api/v1/reviews",
            body:           `{"rating":0.5}`,
            expectedStatus: 400,
            expectedFields: []string{"error", "message", "details"},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(tt.method, tt.endpoint, strings.NewReader(tt.body))
            rec := httptest.NewRecorder()
            
            handler.ServeHTTP(rec, req)
            
            assert.Equal(t, tt.expectedStatus, rec.Code)
            
            var response map[string]interface{}
            json.Unmarshal(rec.Body.Bytes(), &response)
            
            for _, field := range tt.expectedFields {
                assert.Contains(t, response, field)
            }
        })
    }
}
```

### 4. Performance Testing Strategy

**Benchmark Tests**:
```go
func BenchmarkReviewService_CreateReview(b *testing.B) {
    service := setupBenchmarkService()
    review := &Review{
        Rating:  4.5,
        Comment: "Benchmark review",
        HotelID: uuid.New(),
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        service.CreateReview(context.Background(), review)
    }
}

func BenchmarkReviewRepository_GetByHotel(b *testing.B) {
    repo := setupBenchmarkRepository()
    hotelID := uuid.New()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        repo.GetByHotel(context.Background(), hotelID, 10, 0)
    }
}
```

**Load Testing** (using K6):
```javascript
import http from 'k6/http';
import { check } from 'k6';

export let options = {
    stages: [
        { duration: '2m', target: 10 },   // Ramp up
        { duration: '5m', target: 50 },   // Stay at 50 users
        { duration: '2m', target: 0 },    // Ramp down
    ],
};

export default function() {
    let response = http.get('http://localhost:8080/api/v1/reviews');
    check(response, {
        'status is 200': (r) => r.status === 200,
        'response time < 500ms': (r) => r.timings.duration < 500,
    });
}
```

### 5. Test Data Management

**Test Fixtures**:
```go
func createTestReview() *Review {
    return &Review{
        ID:         uuid.New(),
        HotelID:    uuid.New(),
        ProviderID: uuid.New(),
        Rating:     4.5,
        Comment:    "Test review comment",
        ReviewDate: time.Now().Add(-24 * time.Hour),
        CreatedAt:  time.Now(),
    }
}

func createTestHotel() *Hotel {
    return &Hotel{
        ID:       uuid.New(),
        Name:     "Test Hotel",
        Location: "Test City",
        Rating:   4.2,
    }
}
```

**Database Seeding**:
```go
func seedTestDatabase(db *sql.DB) error {
    hotels := []Hotel{createTestHotel(), createTestHotel()}
    for _, hotel := range hotels {
        if err := insertHotel(db, hotel); err != nil {
            return err
        }
    }
    return nil
}
```

### 6. CI/CD Integration

**Quality Gates**:
- Unit tests must pass with 42%+ coverage
- Integration tests must pass
- No security vulnerabilities in dependencies
- Performance benchmarks within acceptable range

**Test Execution**:
```yaml
# In GitHub Actions
- name: Run unit tests
  run: go test -v -race -coverprofile=coverage.out ./... -short

- name: Run integration tests  
  run: go test -v -tags=integration ./...
  
- name: Run benchmarks
  run: go test -bench=. -benchmem ./...
```

## Related Decisions

- [ADR-001](001-clean-architecture-adoption.md): Architecture enables isolated unit testing
- [ADR-002](002-dependency-injection-strategy.md): DI enables easy mock injection
- [ADR-003](003-error-handling-strategy.md): Error testing validates error scenarios

## Notes

This testing strategy emphasizes:
1. **Fast Feedback**: Unit tests provide immediate feedback
2. **Confidence**: Integration tests validate real scenarios
3. **Maintainability**: Clear test organization and data management
4. **Automation**: Full integration with CI/CD pipeline

**Testing Best Practices**:
- Test behavior, not implementation
- Use meaningful test names and descriptions
- Keep tests simple and focused
- Avoid testing third-party libraries
- Use test doubles appropriately (prefer real objects when practical)

**Future Enhancements**:
- Mutation testing to validate test quality
- Property-based testing for complex business logic
- Chaos engineering for resilience testing
- A/B testing infrastructure for feature validation