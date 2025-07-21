# ADR-002: Dependency Injection Strategy

* Status: Accepted
* Date: 2024-01-15
* Authors: Goutam Kumar Biswas
* Deciders: Technical Leadership

## Context

Following the adoption of Clean Architecture (ADR-001), we need a strategy for managing dependencies between layers. The application has multiple external dependencies:
- PostgreSQL database
- Redis cache
- S3 storage
- Authentication services
- Logging and metrics

We need to:
- Inject dependencies without violating architectural boundaries
- Enable easy testing with mock implementations
- Support different configurations for different environments
- Maintain type safety and compile-time dependency resolution

## Decision

We will use **Constructor-based Dependency Injection** with **interface-based dependencies** and a **manual wiring approach** in the main application.

### Key Principles:
1. **Interfaces in Domain Layer**: All external dependencies defined as interfaces in domain
2. **Constructor Injection**: Dependencies passed through constructors
3. **Manual Wiring**: Dependencies composed in main.go, not using a DI framework
4. **Factory Pattern**: Use factory functions for complex object construction

### Implementation Pattern:
```go
// Domain layer defines interfaces
type ReviewRepository interface {
    Create(ctx context.Context, review *Review) error
    GetByID(ctx context.Context, id uuid.UUID) (*Review, error)
}

// Infrastructure layer implements interfaces
type PostgreSQLRepository struct {
    db *sql.DB
}

func NewPostgreSQLRepository(db *sql.DB) ReviewRepository {
    return &PostgreSQLRepository{db: db}
}

// Application layer receives interfaces
type ReviewService struct {
    repo ReviewRepository
}

func NewReviewService(repo ReviewRepository) *ReviewService {
    return &ReviewService{repo: repo}
}

// Manual wiring in main.go
func main() {
    db := setupDatabase()
    repo := infrastructure.NewPostgreSQLRepository(db)
    service := application.NewReviewService(repo)
}
```

## Rationale

**Benefits of Manual DI:**
- **Explicit Dependencies**: Clear visibility of what depends on what
- **Compile-time Safety**: Type checking ensures correct wiring
- **No Magic**: No reflection or runtime dependency resolution
- **Simple Debugging**: Easy to trace dependency chains
- **No Framework Lock-in**: Not tied to specific DI frameworks

**Alternative Approaches Considered:**
- **DI Frameworks (Wire, Dig)**: Rejected due to complexity and runtime overhead
- **Service Locator**: Rejected due to hidden dependencies and testing difficulties
- **Global Variables**: Rejected due to testing and concurrency issues

**Trade-offs:**
- Manual wiring requires more boilerplate in main.go
- Dependency changes require updates to wiring code
- Complex dependency graphs can become unwieldy

## Consequences

### Positive
- Clear, explicit dependency relationships
- Easy to test with mock implementations
- No runtime dependency resolution overhead
- Simple to understand and debug
- Supports interface segregation principle
- Enables different implementations per environment

### Negative
- More verbose dependency setup
- Changes to dependencies require main.go updates
- Large dependency graphs can become complex
- No automatic lifecycle management

### Neutral
- Developers need to understand dependency injection principles
- Testing requires creating mock implementations
- Configuration becomes part of the wiring process

## Implementation

### Dependency Injection Rules:
1. **Interfaces First**: Define interfaces in domain layer before implementations
2. **Constructor Injection**: All dependencies passed through constructors
3. **Immutable Dependencies**: Dependencies set once during construction
4. **Factory Functions**: Use `NewXxx()` functions for object creation
5. **Interface Segregation**: Keep interfaces small and focused

### Testing Strategy:
```go
// Easy testing with mock implementations
func TestReviewService_CreateReview(t *testing.T) {
    mockRepo := &MockReviewRepository{}
    service := NewReviewService(mockRepo)
    // Test with mock
}
```

### Configuration Pattern:
```go
type Config struct {
    Database DatabaseConfig
    Redis    RedisConfig
    S3       S3Config
}

func NewApplication(cfg Config) *Application {
    db := setupDatabase(cfg.Database)
    redis := setupRedis(cfg.Redis)
    s3 := setupS3(cfg.S3)
    
    repo := NewReviewRepository(db)
    cache := NewCacheService(redis)
    storage := NewS3Client(s3)
    
    service := NewReviewService(repo, cache, storage)
    return &Application{service: service}
}
```

### Lifecycle Management:
- **Startup**: Dependencies created in main.go
- **Shutdown**: Cleanup handled by main.go with defer statements
- **Health Checks**: Dependencies expose health check methods

## Related Decisions

- [ADR-001](001-clean-architecture-adoption.md): Clean architecture adoption
- [ADR-003](003-interface-segregation.md): Interface design principles
- [ADR-007](007-testing-strategy.md): Testing approach and mock usage

## Notes

This approach prioritizes simplicity and explicitness over automation. The manual wiring in main.go serves as documentation of the application's dependency structure.

For future consideration: If the dependency graph becomes too complex, we may evaluate adopting Google Wire for compile-time dependency injection while maintaining the same architectural principles.

Examples of this pattern can be found in:
- Go standard library's approach to dependency management
- "Let's Go" by Alex Edwards
- "Clean Architecture in Go" examples