# Code Generation Guide

## Overview

This guide covers code generation tools and practices used in the Hotel Reviews Microservice, including automatic generation of mocks, API documentation, database models, and client SDKs.

## Code Generation Tools

### 1. Mockery - Mock Generation

Mockery automatically generates mocks for Go interfaces, essential for unit testing.

#### Installation
```bash
go install github.com/vektra/mockery/v2@latest
```

#### Configuration
```yaml
# .mockery.yaml
with-expecter: true
output: "mocks"
outpkg: "mocks"
filename: "{{.InterfaceName}}.go"
mockname: "{{.InterfaceName}}"
inpackage: false
testonly: false
exported: true

packages:
  github.com/gkbiswas/hotel-reviews-microservice/internal/domain:
    interfaces:
      ReviewRepository:
      HotelRepository:
      ReviewService:
      CacheService:
      EventPublisher:
```

#### Usage Examples
```go
//go:generate mockery --name=ReviewRepository --output=../mocks --outpkg=mocks

type ReviewRepository interface {
    CreateReview(ctx context.Context, review *Review) error
    GetReview(ctx context.Context, id uuid.UUID) (*Review, error)
    UpdateReview(ctx context.Context, review *Review) error
    DeleteReview(ctx context.Context, id uuid.UUID) error
    SearchReviews(ctx context.Context, filters *SearchFilters) ([]*Review, error)
}

// Generated mock usage in tests
func TestReviewService_CreateReview(t *testing.T) {
    mockRepo := mocks.NewReviewRepository(t)
    service := NewReviewService(mockRepo)
    
    review := &Review{
        Title: "Test Review",
        Rating: 4.5,
    }
    
    mockRepo.EXPECT().
        CreateReview(mock.Anything, review).
        Return(nil).
        Once()
    
    err := service.CreateReview(context.Background(), review)
    assert.NoError(t, err)
}
```

### 2. Swagger/OpenAPI Documentation Generation

Swaggo generates OpenAPI documentation from Go annotations.

#### Installation
```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

#### Configuration
```go
// cmd/api/main.go

//go:generate swag init -g main.go --output ../../docs/swagger

// @title Hotel Reviews API
// @version 1.0
// @description A high-performance microservice for managing hotel reviews
// @termsOfService https://example.com/terms

// @contact.name API Support
// @contact.url https://github.com/gkbiswas/hotel-reviews-microservice
// @contact.email gkbiswas@gmail.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
```

#### Handler Annotations
```go
// CreateReview creates a new hotel review
// @Summary Create a new review
// @Description Create a new hotel review with the provided information
// @Tags reviews
// @Accept json
// @Produce json
// @Param review body dto.CreateReviewRequest true "Review information"
// @Success 201 {object} dto.ReviewResponse
// @Failure 400 {object} dto.ErrorResponse "Bad Request"
// @Failure 401 {object} dto.ErrorResponse "Unauthorized"
// @Failure 422 {object} dto.ErrorResponse "Validation Error"
// @Failure 500 {object} dto.ErrorResponse "Internal Server Error"
// @Security BearerAuth
// @Router /reviews [post]
func (h *ReviewHandler) CreateReview(c *gin.Context) {
    var req dto.CreateReviewRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, dto.ErrorResponse{
            Error: dto.Error{
                Code:    "INVALID_REQUEST",
                Message: "Invalid request format",
                Details: map[string]interface{}{"validation": err.Error()},
            },
        })
        return
    }
    
    // Implementation...
}
```

#### Data Transfer Objects
```go
// dto/review.go

// CreateReviewRequest represents the request to create a new review
type CreateReviewRequest struct {
    HotelID      uuid.UUID `json:"hotel_id" validate:"required" example:"123e4567-e89b-12d3-a456-426614174000"`
    ProviderID   uuid.UUID `json:"provider_id" validate:"required" example:"123e4567-e89b-12d3-a456-426614174001"`
    Title        string    `json:"title" validate:"required,max=500" example:"Amazing stay!"`
    Content      string    `json:"content" validate:"required,max=5000" example:"Had a wonderful experience..."`
    Rating       float64   `json:"rating" validate:"required,min=0,max=5" example:"4.5"`
    Language     string    `json:"language,omitempty" validate:"omitempty,len=2" example:"en"`
    AuthorName   string    `json:"author_name" validate:"required,max=255" example:"John Doe"`
    AuthorLocation string  `json:"author_location,omitempty" validate:"omitempty,max=255" example:"New York, NY"`
    ReviewDate   time.Time `json:"review_date" validate:"required" example:"2024-01-15T10:30:00Z"`
} // @name CreateReviewRequest

// ReviewResponse represents a review in API responses
type ReviewResponse struct {
    ID           uuid.UUID `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
    HotelID      uuid.UUID `json:"hotel_id" example:"123e4567-e89b-12d3-a456-426614174001"`
    ProviderID   uuid.UUID `json:"provider_id" example:"123e4567-e89b-12d3-a456-426614174002"`
    Title        string    `json:"title" example:"Amazing stay!"`
    Content      string    `json:"content" example:"Had a wonderful experience at this hotel..."`
    Rating       float64   `json:"rating" example:"4.5"`
    Language     string    `json:"language" example:"en"`
    AuthorName   string    `json:"author_name" example:"John Doe"`
    AuthorLocation string  `json:"author_location" example:"New York, NY"`
    ReviewDate   time.Time `json:"review_date" example:"2024-01-15T10:30:00Z"`
    CreatedAt    time.Time `json:"created_at" example:"2024-01-15T10:30:00Z"`
    UpdatedAt    time.Time `json:"updated_at" example:"2024-01-15T10:30:00Z"`
} // @name ReviewResponse

// ErrorResponse represents an error response
type ErrorResponse struct {
    Error Error `json:"error"`
} // @name ErrorResponse

// Error represents error details
type Error struct {
    Code      string                 `json:"code" example:"INVALID_REQUEST"`
    Message   string                 `json:"message" example:"The request is invalid"`
    Details   map[string]interface{} `json:"details,omitempty"`
    Timestamp time.Time              `json:"timestamp" example:"2024-01-15T10:30:00Z"`
    RequestID string                 `json:"request_id,omitempty" example:"req-123"`
} // @name Error
```

### 3. GORM Model Generation

Generate GORM models from database schema.

#### Installation
```bash
go install gorm.io/gen/tools/gentool@latest
```

#### Generator Configuration
```go
// scripts/generate-models.go

package main

import (
    "gorm.io/driver/postgres"
    "gorm.io/gen"
    "gorm.io/gorm"
)

func main() {
    // Initialize the generator with configuration
    g := gen.NewGenerator(gen.Config{
        OutPath:           "./internal/infrastructure/database/query",
        ModelPkgPath:      "./internal/infrastructure/database/model",
        Mode:              gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
        FieldNullable:     true,
        FieldWithIndexTag: true,
        FieldWithTypeTag:  true,
    })

    // Connect to database
    db, err := gorm.Open(postgres.Open("postgres://user:pass@localhost/hotel_reviews?sslmode=disable"))
    if err != nil {
        panic(err)
    }

    g.UseDB(db)

    // Generate models for all tables
    g.ApplyBasic(
        g.GenerateModel("hotels"),
        g.GenerateModel("reviews"),
        g.GenerateModel("providers"),
        g.GenerateModel("users"),
        g.GenerateModel("api_keys"),
    )

    // Generate with custom methods
    reviews := g.GenerateModel("reviews",
        gen.FieldType("id", "uuid.UUID"),
        gen.FieldType("hotel_id", "uuid.UUID"),
        gen.FieldType("provider_id", "uuid.UUID"),
        gen.FieldJSONTagWithNS(func(columnName string) string {
            return columnName
        }),
    )

    // Add custom query methods
    g.ApplyInterface(func(ReviewMethod) {}, reviews)

    g.Execute()
}

type ReviewMethod interface {
    // WHERE rating >= ?
    FindByRatingGTE(rating float64) (gen.T, error)
    
    // WHERE hotel_id = ? AND rating >= ?
    FindByHotelAndMinRating(hotelID uuid.UUID, minRating float64) ([]*model.Review, error)
    
    // Custom SQL query
    // SELECT AVG(rating) as avg_rating, COUNT(*) as review_count FROM @@table WHERE hotel_id = ?
    GetHotelStats(hotelID uuid.UUID) (gen.T, error)
}
```

### 4. Protocol Buffer Generation

For gRPC services and efficient serialization.

#### Installation
```bash
# Install protoc
# macOS
brew install protobuf

# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

#### Proto Definition
```proto
// api/proto/reviews.proto
syntax = "proto3";

package hotel.reviews.v1;

option go_package = "github.com/gkbiswas/hotel-reviews-microservice/internal/grpc/generated";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";

service ReviewService {
    rpc CreateReview(CreateReviewRequest) returns (ReviewResponse);
    rpc GetReview(GetReviewRequest) returns (ReviewResponse);
    rpc ListReviews(ListReviewsRequest) returns (ListReviewsResponse);
    rpc UpdateReview(UpdateReviewRequest) returns (ReviewResponse);
    rpc DeleteReview(DeleteReviewRequest) returns (google.protobuf.Empty);
}

message Review {
    string id = 1;
    string hotel_id = 2;
    string provider_id = 3;
    string title = 4;
    string content = 5;
    double rating = 6;
    string language = 7;
    string author_name = 8;
    string author_location = 9;
    google.protobuf.Timestamp review_date = 10;
    google.protobuf.Timestamp created_at = 11;
    google.protobuf.Timestamp updated_at = 12;
}

message CreateReviewRequest {
    string hotel_id = 1;
    string provider_id = 2;
    string title = 3;
    string content = 4;
    double rating = 5;
    string language = 6;
    string author_name = 7;
    string author_location = 8;
    google.protobuf.Timestamp review_date = 9;
}

message ReviewResponse {
    Review review = 1;
}

message GetReviewRequest {
    string id = 1;
}

message ListReviewsRequest {
    string hotel_id = 1;
    int32 limit = 2;
    int32 offset = 3;
    string sort_by = 4;
}

message ListReviewsResponse {
    repeated Review reviews = 1;
    int32 total_count = 2;
}

message UpdateReviewRequest {
    string id = 1;
    Review review = 2;
}

message DeleteReviewRequest {
    string id = 1;
}
```

#### Generation Script
```bash
#!/bin/bash
# scripts/generate-proto.sh

# Generate Go code from proto files
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/*.proto

# Generate OpenAPI documentation
protoc --openapi_out=docs/openapi \
       api/proto/*.proto
```

### 5. Client SDK Generation

Generate client SDKs for different programming languages.

#### OpenAPI Generator
```yaml
# codegen/openapi-generator-config.yaml
generatorName: go
outputDir: sdk/go
inputSpec: docs/swagger/swagger.yaml

templateDir: codegen/templates
packageName: hotelreviews
packageVersion: 1.0.0
packageUrl: github.com/gkbiswas/hotel-reviews-sdk-go

additionalProperties:
  packageName: hotelreviews
  clientPackage: client
  modelPackage: models
  generateInterfaces: true
  structPrefix: true
  enumClassPrefix: true
```

#### SDK Generation Script
```bash
#!/bin/bash
# scripts/generate-sdks.sh

# Generate Go SDK
openapi-generator generate \
  -i docs/swagger/swagger.yaml \
  -g go \
  -o sdk/go \
  --additional-properties=packageName=hotelreviews,packageVersion=1.0.0

# Generate Python SDK
openapi-generator generate \
  -i docs/swagger/swagger.yaml \
  -g python \
  -o sdk/python \
  --additional-properties=packageName=hotel-reviews,packageVersion=1.0.0

# Generate JavaScript SDK
openapi-generator generate \
  -i docs/swagger/swagger.yaml \
  -g javascript \
  -o sdk/javascript \
  --additional-properties=projectName=hotel-reviews,projectVersion=1.0.0

# Generate Java SDK
openapi-generator generate \
  -i docs/swagger/swagger.yaml \
  -g java \
  -o sdk/java \
  --additional-properties=groupId=com.hotelreviews,artifactId=hotel-reviews-sdk,artifactVersion=1.0.0
```

### 6. Database Migration Generation

Generate migrations from model changes.

#### Atlas Migration Tool
```go
//go:build ignore

package main

import (
    "context"
    "log"
    "os"

    "ariga.io/atlas/sql/migrate"
    "ariga.io/atlas/sql/schema"
    "ariga.io/atlas/sql/postgres"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/database/model"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    // Connect to database
    db, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")))
    if err != nil {
        log.Fatal(err)
    }

    // Get current schema
    current := getCurrentSchema(db)
    
    // Get desired schema from models
    desired := getDesiredSchema()
    
    // Generate migration
    changes, err := schema.Changes(current, desired)
    if err != nil {
        log.Fatal(err)
    }
    
    // Write migration file
    migration := &migrate.Migration{
        Version: generateVersion(),
        Desc:    "Auto-generated from model changes",
    }
    
    for _, change := range changes {
        migration.Stmts = append(migration.Stmts, change.SQL())
    }
    
    writeToFile(migration)
}

func getCurrentSchema(db *gorm.DB) *schema.Schema {
    // Implementation to read current database schema
    // ...
}

func getDesiredSchema() *schema.Schema {
    // Implementation to build schema from GORM models
    // ...
}
```

## Build Integration

### Makefile Integration
```makefile
# Generate all code
.PHONY: generate
generate: generate-mocks generate-docs generate-models generate-proto generate-sdk

.PHONY: generate-mocks
generate-mocks:  ## Generate mocks
	mockery --all --output=mocks --case=underscore

.PHONY: generate-docs
generate-docs:  ## Generate API documentation
	swag init -g cmd/api/main.go --output docs/swagger

.PHONY: generate-models
generate-models:  ## Generate database models
	go run scripts/generate-models.go

.PHONY: generate-proto
generate-proto:  ## Generate protobuf code
	./scripts/generate-proto.sh

.PHONY: generate-sdk
generate-sdk:  ## Generate client SDKs
	./scripts/generate-sdks.sh

.PHONY: verify-generate
verify-generate: generate  ## Verify generated code is up to date
	git diff --exit-code || (echo "Generated code is out of date. Run 'make generate'" && exit 1)
```

### CI/CD Integration
```yaml
# .github/workflows/codegen.yml
name: Code Generation Check

on:
  pull_request:
    branches: [main]

jobs:
  check-generated-code:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      
      - name: Install dependencies
        run: |
          go install github.com/vektra/mockery/v2@latest
          go install github.com/swaggo/swag/cmd/swag@latest
          
      - name: Generate code
        run: make generate
        
      - name: Check for changes
        run: |
          if [[ -n $(git status --porcelain) ]]; then
            echo "Generated code is out of date:"
            git diff
            echo "Please run 'make generate' and commit the changes"
            exit 1
          fi
```

### Pre-commit Hooks
```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: generate-code
        name: Generate code
        entry: make generate
        language: system
        files: '.*\.(go|proto|yaml)$'
        pass_filenames: false
        
      - id: verify-generate
        name: Verify generated code
        entry: make verify-generate
        language: system
        files: '.*\.(go|proto|yaml)$'
        pass_filenames: false
```

## Custom Generators

### Custom Template for API Handlers
```go
// codegen/templates/handler.go.tmpl
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/gkbiswas/hotel-reviews-microservice/internal/application/dto"
    "github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
)

type {{.ResourceName}}Handler struct {
    service domain.{{.ResourceName}}Service
}

func New{{.ResourceName}}Handler(service domain.{{.ResourceName}}Service) *{{.ResourceName}}Handler {
    return &{{.ResourceName}}Handler{
        service: service,
    }
}

{{range .Operations}}
// {{.Name}} {{.Description}}
// @Summary {{.Summary}}
// @Description {{.Description}}
// @Tags {{$.ResourceName | lower}}
// @Accept json
// @Produce json
{{if .RequestBody}}// @Param {{$.ResourceName | lower}} body dto.{{.RequestBody}} true "{{.RequestBody}} information"{{end}}
{{range .Parameters}}// @Param {{.Name}} {{.In}} {{.Type}} {{.Required}} "{{.Description}}"{{end}}
// @Success {{.SuccessCode}} {object} dto.{{.ResponseBody}}
{{range .ErrorResponses}}// @Failure {{.Code}} {object} dto.ErrorResponse "{{.Description}}"{{end}}
// @Router {{.Path}} [{{.Method | lower}}]
func (h *{{$.ResourceName}}Handler) {{.Name}}(c *gin.Context) {
    // TODO: Implement {{.Name}}
    c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}
{{end}}
```

### Generator Script
```go
// codegen/generator.go
package main

import (
    "os"
    "text/template"
)

type Resource struct {
    ResourceName string
    Operations   []Operation
}

type Operation struct {
    Name            string
    Description     string
    Summary         string
    Method          string
    Path            string
    RequestBody     string
    ResponseBody    string
    Parameters      []Parameter
    SuccessCode     int
    ErrorResponses  []ErrorResponse
}

type Parameter struct {
    Name        string
    In          string
    Type        string
    Required    string
    Description string
}

type ErrorResponse struct {
    Code        int
    Description string
}

func main() {
    tmpl, err := template.ParseFiles("codegen/templates/handler.go.tmpl")
    if err != nil {
        panic(err)
    }

    resource := Resource{
        ResourceName: "Review",
        Operations: []Operation{
            {
                Name:        "CreateReview",
                Description: "Create a new hotel review",
                Summary:     "Create review",
                Method:      "POST",
                Path:        "/api/v1/reviews",
                RequestBody: "CreateReviewRequest",
                ResponseBody: "ReviewResponse",
                SuccessCode: 201,
                ErrorResponses: []ErrorResponse{
                    {Code: 400, Description: "Bad Request"},
                    {Code: 422, Description: "Validation Error"},
                    {Code: 500, Description: "Internal Server Error"},
                },
            },
            // More operations...
        },
    }

    file, err := os.Create("internal/application/handlers/review_handler_generated.go")
    if err != nil {
        panic(err)
    }
    defer file.Close()

    err = tmpl.Execute(file, resource)
    if err != nil {
        panic(err)
    }
}
```

## Best Practices

### 1. Code Generation Guidelines
- **Version Control**: Always commit generated code
- **Automation**: Use CI/CD to verify generated code is up-to-date
- **Documentation**: Document generation steps and requirements
- **Consistency**: Use consistent naming and structure across generators

### 2. Template Management
- **Reusability**: Create reusable template components
- **Validation**: Validate generated code with linters and tests
- **Customization**: Allow customization through configuration files
- **Maintenance**: Keep templates updated with language/framework changes

### 3. Integration Testing
- **Test Generated Code**: Include generated code in test suites
- **Mock Validation**: Ensure mocks match actual interfaces
- **API Compatibility**: Test API documentation matches implementation
- **SDK Testing**: Test generated SDKs against live API

This comprehensive code generation guide ensures consistent, maintainable, and automated code generation across the Hotel Reviews Microservice project.