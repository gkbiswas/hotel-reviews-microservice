# Development Guide

## Overview

This comprehensive development guide covers setup, workflows, coding standards, and best practices for contributing to the Hotel Reviews Microservice.

## Development Environment Setup

### Prerequisites

- **Go 1.21+** - Latest stable version
- **Docker & Docker Compose** - For local infrastructure
- **Git** - Version control
- **Make** - Build automation
- **IDE/Editor** with Go support (VS Code, GoLand, Vim)

### Initial Setup

1. **Clone the Repository**
   ```bash
   git clone https://github.com/gkbiswas/hotel-reviews-microservice.git
   cd hotel-reviews-microservice
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Start Local Infrastructure**
   ```bash
   docker-compose up -d postgres redis kafka
   ```

4. **Run Database Migrations**
   ```bash
   go run cmd/migrate/main.go up
   ```

5. **Start the Application**
   ```bash
   go run cmd/api/main.go server --config config/development.yaml
   ```

### Development Tools

#### Required Tools
```bash
# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/air-verse/air@latest  # Hot reload
go install github.com/swaggo/swag/cmd/swag@latest  # API docs
go install github.com/vektra/mockery/v2@latest  # Mocks
go install gotest.tools/gotestsum@latest  # Better test output
```

#### VS Code Extensions
```json
{
  "recommendations": [
    "golang.go",
    "ms-vscode.vscode-json",
    "ms-vscode.docker",
    "humao.rest-client",
    "bradlc.vscode-tailwindcss",
    "ms-vscode.test-adapter-converter"
  ]
}
```

## Project Structure

### Directory Layout
```
hotel-reviews-microservice/
├── cmd/                    # Application entry points
│   ├── api/               # HTTP API server
│   └── migrate/           # Database migration tool
├── internal/              # Private application code
│   ├── domain/           # Business logic and entities
│   ├── application/      # Use cases and HTTP handlers
│   ├── infrastructure/   # External dependencies
│   ├── monitoring/       # Observability components
│   └── server/           # Server configuration
├── pkg/                   # Public libraries
├── tests/                 # Integration tests
├── examples/             # Usage examples
├── docs/                 # Documentation
├── scripts/              # Build and utility scripts
├── migrations/           # Database migrations
├── configs/              # Configuration files
└── docker/               # Docker configurations
```

### Architecture Layers

#### Domain Layer (`internal/domain/`)
- **Purpose**: Core business logic, entities, and domain services
- **Dependencies**: None (pure business logic)
- **Testing**: 100% unit test coverage required

#### Application Layer (`internal/application/`)
- **Purpose**: Use cases, HTTP handlers, middleware
- **Dependencies**: Domain layer only
- **Testing**: Integration tests with mocked dependencies

#### Infrastructure Layer (`internal/infrastructure/`)
- **Purpose**: Database, external APIs, messaging, caching
- **Dependencies**: Application and Domain layers
- **Testing**: Integration tests with real external services

## Coding Standards

### Go Style Guidelines

1. **Follow Effective Go**: https://golang.org/doc/effective_go.html
2. **Use gofmt**: Code must be formatted with `gofmt`
3. **Follow Go Code Review Comments**: https://github.com/golang/go/wiki/CodeReviewComments

### Naming Conventions

```go
// Good examples
type ReviewService struct{}
func (rs *ReviewService) CreateReview(ctx context.Context, review *Review) error {}
var ErrReviewNotFound = errors.New("review not found")
const DefaultTimeout = 30 * time.Second

// Interface names
type ReviewRepository interface {
    GetReview(ctx context.Context, id uuid.UUID) (*Review, error)
}

// Package names (lowercase, single word)
package review
package database
```

### Error Handling

```go
// Define custom error types
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

// Wrap errors with context
func (rs *ReviewService) CreateReview(ctx context.Context, review *Review) error {
    if err := rs.validateReview(review); err != nil {
        return fmt.Errorf("review validation failed: %w", err)
    }
    
    if err := rs.repo.CreateReview(ctx, review); err != nil {
        return fmt.Errorf("failed to create review in repository: %w", err)
    }
    
    return nil
}
```

### Testing Standards

#### Unit Tests
```go
func TestReviewService_CreateReview(t *testing.T) {
    tests := []struct {
        name    string
        review  *Review
        wantErr bool
        errType error
    }{
        {
            name: "valid review",
            review: &Review{
                Title:   "Great hotel",
                Content: "Had an amazing stay",
                Rating:  5.0,
            },
            wantErr: false,
        },
        {
            name: "invalid rating",
            review: &Review{
                Title:   "Bad review",
                Content: "Terrible",
                Rating:  -1.0,
            },
            wantErr: true,
            errType: &ValidationError{},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := NewReviewService(mockRepo)
            err := service.CreateReview(context.Background(), tt.review)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateReview() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if tt.errType != nil {
                assert.IsType(t, tt.errType, errors.Unwrap(err))
            }
        })
    }
}
```

#### Integration Tests
```go
func TestReviewAPI_Integration(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer db.Close()
    
    // Setup test server
    server := setupTestServer(t, db)
    defer server.Close()
    
    t.Run("create and retrieve review", func(t *testing.T) {
        review := &Review{
            HotelID: uuid.New(),
            Title:   "Test Review",
            Content: "This is a test review",
            Rating:  4.5,
        }
        
        // Create review
        resp := testCreateReview(t, server.URL, review)
        assert.Equal(t, http.StatusCreated, resp.StatusCode)
        
        var createdReview Review
        json.NewDecoder(resp.Body).Decode(&createdReview)
        
        // Retrieve review
        getResp := testGetReview(t, server.URL, createdReview.ID)
        assert.Equal(t, http.StatusOK, getResp.StatusCode)
        
        var retrievedReview Review
        json.NewDecoder(getResp.Body).Decode(&retrievedReview)
        
        assert.Equal(t, review.Title, retrievedReview.Title)
        assert.Equal(t, review.Rating, retrievedReview.Rating)
    })
}
```

## Development Workflow

### Git Workflow

1. **Branch Naming**
   ```bash
   feature/add-user-authentication
   bugfix/fix-rating-calculation
   hotfix/security-vulnerability
   refactor/improve-error-handling
   ```

2. **Commit Messages**
   ```bash
   feat: add user authentication middleware
   fix: correct hotel rating calculation
   docs: update API documentation
   test: add integration tests for reviews
   refactor: improve error handling structure
   ```

3. **Pull Request Process**
   - Create feature branch from `main`
   - Make changes with tests
   - Ensure CI passes
   - Request code review
   - Address review feedback
   - Merge to `main`

### Code Review Guidelines

#### What to Look For
- **Correctness**: Does the code do what it's supposed to do?
- **Performance**: Are there any obvious performance issues?
- **Security**: Are there any security vulnerabilities?
- **Maintainability**: Is the code readable and well-organized?
- **Testing**: Are there adequate tests?

#### Review Checklist
- [ ] Code follows project style guidelines
- [ ] All tests pass
- [ ] No sensitive information in code
- [ ] Error handling is appropriate
- [ ] Documentation is updated
- [ ] Performance impact considered

### Local Development Commands

#### Makefile Targets
```makefile
.PHONY: help build test lint run clean docker-up docker-down

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build:  ## Build the application
	go build -o bin/hotel-reviews cmd/api/main.go

test:  ## Run all tests
	go test -v -race -coverprofile=coverage.out ./...

test-integration:  ## Run integration tests
	go test -v -tags=integration ./tests/...

lint:  ## Run linters
	golangci-lint run

run:  ## Run the application
	air -c .air.toml

clean:  ## Clean build artifacts
	rm -rf bin/ coverage.out

docker-up:  ## Start local infrastructure
	docker-compose up -d

docker-down:  ## Stop local infrastructure
	docker-compose down

generate:  ## Generate code (mocks, docs)
	go generate ./...
	swag init -g cmd/api/main.go

migrate-up:  ## Run database migrations
	go run cmd/migrate/main.go up

migrate-down:  ## Rollback database migrations
	go run cmd/migrate/main.go down

seed-data:  ## Seed development data
	go run scripts/seed-data.go
```

### Hot Reload Configuration

#### Air Configuration (`.air.toml`)
```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = ["server", "--config", "configs/development.yaml"]
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main cmd/api/main.go"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "yaml", "yml", "json"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false
```

## Database Development

### Migration Guidelines

1. **Create Migration Files**
   ```bash
   # Create new migration
   migrate create -ext sql -dir migrations add_review_sentiment
   ```

2. **Migration File Structure**
   ```sql
   -- migrations/001_create_reviews_table.up.sql
   CREATE TABLE reviews (
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       hotel_id UUID NOT NULL,
       title VARCHAR(500),
       content TEXT NOT NULL,
       rating DECIMAL(3,2) NOT NULL CHECK (rating >= 0 AND rating <= 5),
       created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
       updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
   );
   
   CREATE INDEX idx_reviews_hotel_id ON reviews(hotel_id);
   CREATE INDEX idx_reviews_rating ON reviews(rating);
   ```
   
   ```sql
   -- migrations/001_create_reviews_table.down.sql
   DROP INDEX IF EXISTS idx_reviews_rating;
   DROP INDEX IF EXISTS idx_reviews_hotel_id;
   DROP TABLE IF EXISTS reviews;
   ```

3. **Migration Best Practices**
   - Always create both `up` and `down` migrations
   - Test migrations on copy of production data
   - Use transactions for complex migrations
   - Add appropriate indexes
   - Include data validation checks

### Database Testing

```go
func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/test_db?sslmode=disable")
    require.NoError(t, err)
    
    // Run migrations
    migrate := &migrate.Migrate{
        Database: postgres.WithInstance(db, &postgres.Config{}),
        Source:   "file://migrations",
    }
    
    err = migrate.Up()
    require.NoError(t, err)
    
    // Clean up after test
    t.Cleanup(func() {
        migrate.Down()
        db.Close()
    })
    
    return db
}
```

## API Development

### API Design Principles

1. **RESTful Design**
   - Use proper HTTP methods (GET, POST, PUT, DELETE)
   - Consistent URL patterns (`/api/v1/resources/{id}`)
   - Appropriate HTTP status codes

2. **Request/Response Format**
   ```go
   // Request
   type CreateReviewRequest struct {
       HotelID     uuid.UUID `json:"hotel_id" validate:"required"`
       Title       string    `json:"title" validate:"required,max=500"`
       Content     string    `json:"content" validate:"required,max=5000"`
       Rating      float64   `json:"rating" validate:"required,min=0,max=5"`
       AuthorName  string    `json:"author_name" validate:"required,max=255"`
   }
   
   // Response
   type ReviewResponse struct {
       ID          uuid.UUID `json:"id"`
       HotelID     uuid.UUID `json:"hotel_id"`
       Title       string    `json:"title"`
       Content     string    `json:"content"`
       Rating      float64   `json:"rating"`
       AuthorName  string    `json:"author_name"`
       CreatedAt   time.Time `json:"created_at"`
       UpdatedAt   time.Time `json:"updated_at"`
   }
   ```

3. **Error Responses**
   ```go
   type ErrorResponse struct {
       Error struct {
           Code    string                 `json:"code"`
           Message string                 `json:"message"`
           Details map[string]interface{} `json:"details,omitempty"`
       } `json:"error"`
   }
   ```

### API Documentation

#### Swagger Annotations
```go
// CreateReview creates a new hotel review
// @Summary Create a new review
// @Description Create a new hotel review with the provided information
// @Tags reviews
// @Accept json
// @Produce json
// @Param review body CreateReviewRequest true "Review information"
// @Success 201 {object} ReviewResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/reviews [post]
func (h *ReviewHandler) CreateReview(c *gin.Context) {
    // Implementation
}
```

## Performance Optimization

### Profiling

1. **Enable Profiling**
   ```go
   import _ "net/http/pprof"
   
   // In development mode
   if os.Getenv("ENABLE_PPROF") == "true" {
       go func() {
           log.Println(http.ListenAndServe("localhost:6060", nil))
       }()
   }
   ```

2. **Analyze Performance**
   ```bash
   # CPU profiling
   go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
   
   # Memory profiling
   go tool pprof http://localhost:6060/debug/pprof/heap
   
   # Goroutine analysis
   go tool pprof http://localhost:6060/debug/pprof/goroutine
   ```

### Benchmarking

```go
func BenchmarkReviewCreation(b *testing.B) {
    service := setupBenchmarkService()
    review := &Review{
        Title:   "Benchmark Review",
        Content: "This is a benchmark review",
        Rating:  4.5,
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        service.CreateReview(context.Background(), review)
    }
}

func BenchmarkDatabaseQuery(b *testing.B) {
    db := setupBenchmarkDB()
    defer db.Close()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var reviews []Review
        db.Find(&reviews).Limit(100)
    }
}
```

## Debugging

### Logging Configuration

```go
// Development logging
func setupDevelopmentLogging() *logrus.Logger {
    log := logrus.New()
    log.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
        ForceColors:   true,
    })
    log.SetLevel(logrus.DebugLevel)
    return log
}

// Production logging
func setupProductionLogging() *logrus.Logger {
    log := logrus.New()
    log.SetFormatter(&logrus.JSONFormatter{})
    log.SetLevel(logrus.InfoLevel)
    return log
}
```

### Debug Utilities

```go
// Debug middleware for request/response logging
func DebugMiddleware() gin.HandlerFunc {
    return gin.LoggerWithConfig(gin.LoggerConfig{
        Formatter: func(param gin.LogFormatterParams) string {
            return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
                param.ClientIP,
                param.TimeStamp.Format(time.RFC1123),
                param.Method,
                param.Path,
                param.Request.Proto,
                param.StatusCode,
                param.Latency,
                param.Request.UserAgent(),
                param.ErrorMessage,
            )
        },
    })
}
```

## Deployment

### Environment Configuration

```yaml
# configs/development.yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  host: "localhost"
  port: 5432
  user: "hotel_reviews"
  password: "development"
  name: "hotel_reviews"
  ssl_mode: "disable"
  max_conns: 25

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

logging:
  level: "debug"
  format: "text"

monitoring:
  enable_metrics: true
  enable_pprof: true
```

### Docker Development

```dockerfile
# Dockerfile.dev
FROM golang:1.21-alpine AS development

WORKDIR /app

# Install Air for hot reload
RUN go install github.com/air-verse/air@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]
```

## Best Practices Summary

### Code Organization
- Follow Clean Architecture principles
- Keep domain logic pure
- Use dependency injection
- Separate concerns clearly

### Testing
- Write tests first (TDD)
- Aim for high test coverage
- Use table-driven tests
- Mock external dependencies

### Performance
- Profile before optimizing
- Use appropriate data structures
- Implement caching strategies
- Monitor resource usage

### Security
- Validate all inputs
- Use parameterized queries
- Implement proper authentication
- Log security events

### Documentation
- Document public APIs
- Keep README updated
- Write inline comments for complex logic
- Maintain architectural decision records

This development guide ensures consistent, high-quality development practices across the Hotel Reviews Microservice project.