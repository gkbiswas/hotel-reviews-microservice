# Testing Guide

## Overview

This comprehensive testing guide covers all aspects of testing the Hotel Reviews Microservice, including unit tests, integration tests, end-to-end tests, performance tests, and testing best practices.

## Testing Strategy

### Test Pyramid

```
                    /\
                   /  \
                  /    \
                 / E2E  \      (Few, Slow, High Confidence)
                /  Tests \
               /_________ \
              /            \
             / Integration  \   (Some, Medium Speed)
            /    Tests      \
           /________________ \
          /                  \
         /    Unit Tests      \  (Many, Fast, Focused)
        /____________________\
```

### Testing Levels

1. **Unit Tests (70%)**
   - Test individual components in isolation
   - Mock external dependencies
   - Fast execution (<1ms per test)
   - High code coverage (>90%)

2. **Integration Tests (20%)**
   - Test component interactions
   - Use real databases/services
   - Medium execution speed
   - Focus on critical paths

3. **End-to-End Tests (10%)**
   - Test complete user scenarios
   - Use production-like environment
   - Slower execution
   - Cover critical business flows

## Unit Testing

### Testing Framework Setup

```go
// internal/domain/review_test.go
package domain

import (
    "context"
    "testing"
    "time"
    
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
    
    "github.com/gkbiswas/hotel-reviews-microservice/mocks"
)

func TestReviewService_CreateReview(t *testing.T) {
    type fields struct {
        repo      ReviewRepository
        validator ReviewValidator
        publisher EventPublisher
    }
    type args struct {
        ctx    context.Context
        review *Review
    }
    tests := []struct {
        name    string
        fields  fields
        args    args
        setup   func(*mocks.ReviewRepository, *mocks.ReviewValidator, *mocks.EventPublisher)
        wantErr bool
        errType error
    }{
        {
            name: "successful review creation",
            args: args{
                ctx: context.Background(),
                review: &Review{
                    HotelID:     uuid.New(),
                    ProviderID:  uuid.New(),
                    Title:       "Great hotel!",
                    Content:     "Had an amazing stay here.",
                    Rating:      5.0,
                    AuthorName:  "John Doe",
                    ReviewDate:  time.Now(),
                },
            },
            setup: func(repo *mocks.ReviewRepository, validator *mocks.ReviewValidator, publisher *mocks.EventPublisher) {
                validator.EXPECT().
                    ValidateReview(mock.Anything).
                    Return(nil).
                    Once()
                
                repo.EXPECT().
                    CreateReview(mock.Anything, mock.AnythingOfType("*domain.Review")).
                    Return(nil).
                    Once()
                
                publisher.EXPECT().
                    PublishReviewCreated(mock.Anything, mock.AnythingOfType("*domain.Review")).
                    Return(nil).
                    Once()
            },
            wantErr: false,
        },
        {
            name: "validation error",
            args: args{
                ctx: context.Background(),
                review: &Review{
                    HotelID: uuid.New(),
                    Title:   "", // Invalid: empty title
                    Rating:  5.0,
                },
            },
            setup: func(repo *mocks.ReviewRepository, validator *mocks.ReviewValidator, publisher *mocks.EventPublisher) {
                validator.EXPECT().
                    ValidateReview(mock.Anything).
                    Return(&ValidationError{Field: "title", Message: "title is required"}).
                    Once()
            },
            wantErr: true,
            errType: &ValidationError{},
        },
        {
            name: "repository error",
            args: args{
                ctx: context.Background(),
                review: &Review{
                    HotelID:    uuid.New(),
                    Title:      "Good hotel",
                    Content:    "Nice experience",
                    Rating:     4.0,
                    ReviewDate: time.Now(),
                },
            },
            setup: func(repo *mocks.ReviewRepository, validator *mocks.ReviewValidator, publisher *mocks.EventPublisher) {
                validator.EXPECT().
                    ValidateReview(mock.Anything).
                    Return(nil).
                    Once()
                
                repo.EXPECT().
                    CreateReview(mock.Anything, mock.AnythingOfType("*domain.Review")).
                    Return(errors.New("database connection failed")).
                    Once()
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mocks
            mockRepo := mocks.NewReviewRepository(t)
            mockValidator := mocks.NewReviewValidator(t)
            mockPublisher := mocks.NewEventPublisher(t)
            
            if tt.setup != nil {
                tt.setup(mockRepo, mockValidator, mockPublisher)
            }
            
            // Create service
            service := NewReviewService(mockRepo, mockValidator, mockPublisher)
            
            // Execute test
            err := service.CreateReview(tt.args.ctx, tt.args.review)
            
            // Assertions
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errType != nil {
                    assert.IsType(t, tt.errType, errors.Unwrap(err))
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Test Helpers and Utilities

```go
// internal/testutil/helpers.go
package testutil

import (
    "testing"
    "time"
    
    "github.com/google/uuid"
    "github.com/stretchr/testify/require"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
)

// ReviewBuilder provides a fluent interface for creating test reviews
type ReviewBuilder struct {
    review *domain.Review
}

func NewReview() *ReviewBuilder {
    return &ReviewBuilder{
        review: &domain.Review{
            ID:         uuid.New(),
            HotelID:    uuid.New(),
            ProviderID: uuid.New(),
            Title:      "Test Review",
            Content:    "This is a test review content",
            Rating:     4.0,
            Language:   "en",
            AuthorName: "Test Author",
            ReviewDate: time.Now(),
            CreatedAt:  time.Now(),
            UpdatedAt:  time.Now(),
        },
    }
}

func (b *ReviewBuilder) WithTitle(title string) *ReviewBuilder {
    b.review.Title = title
    return b
}

func (b *ReviewBuilder) WithRating(rating float64) *ReviewBuilder {
    b.review.Rating = rating
    return b
}

func (b *ReviewBuilder) WithHotelID(hotelID uuid.UUID) *ReviewBuilder {
    b.review.HotelID = hotelID
    return b
}

func (b *ReviewBuilder) WithInvalidRating() *ReviewBuilder {
    b.review.Rating = -1.0
    return b
}

func (b *ReviewBuilder) Build() *domain.Review {
    return b.review
}

// AssertReviewEqual compares two reviews for equality in tests
func AssertReviewEqual(t *testing.T, expected, actual *domain.Review) {
    t.Helper()
    
    require.Equal(t, expected.ID, actual.ID, "Review ID should match")
    require.Equal(t, expected.HotelID, actual.HotelID, "Hotel ID should match")
    require.Equal(t, expected.Title, actual.Title, "Title should match")
    require.Equal(t, expected.Content, actual.Content, "Content should match")
    require.Equal(t, expected.Rating, actual.Rating, "Rating should match")
    require.Equal(t, expected.AuthorName, actual.AuthorName, "Author name should match")
}

// TimeNearEqual checks if two times are approximately equal (within 1 second)
func TimeNearEqual(t *testing.T, expected, actual time.Time, msgAndArgs ...interface{}) {
    t.Helper()
    
    diff := expected.Sub(actual)
    if diff < 0 {
        diff = -diff
    }
    
    require.True(t, diff < time.Second, 
        "Times should be within 1 second of each other: expected %v, got %v", 
        expected, actual)
}

// MockContext creates a test context with timeout
func MockContext() (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), 5*time.Second)
}
```

### Benchmarking

```go
// internal/domain/review_benchmark_test.go
package domain

import (
    "context"
    "testing"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/testutil"
)

func BenchmarkReviewService_CreateReview(b *testing.B) {
    service := setupBenchmarkService()
    review := testutil.NewReview().Build()
    ctx := context.Background()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        // Create a new review for each iteration to avoid conflicts
        testReview := testutil.NewReview().
            WithTitle(fmt.Sprintf("Benchmark Review %d", i)).
            Build()
        
        err := service.CreateReview(ctx, testReview)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkReviewValidator_ValidateReview(b *testing.B) {
    validator := NewReviewValidator()
    review := testutil.NewReview().Build()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        err := validator.ValidateReview(review)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Benchmark with different data sizes
func BenchmarkReviewService_SearchReviews(b *testing.B) {
    dataSizes := []int{10, 100, 1000, 10000}
    
    for _, size := range dataSizes {
        b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
            service := setupBenchmarkServiceWithData(size)
            filters := &SearchFilters{
                HotelID: uuid.New(),
                Limit:   20,
            }
            ctx := context.Background()
            
            b.ResetTimer()
            
            for i := 0; i < b.N; i++ {
                _, err := service.SearchReviews(ctx, filters)
                if err != nil {
                    b.Fatal(err)
                }
            }
        })
    }
}
```

## Integration Testing

### Database Integration Tests

```go
// tests/integration/database_test.go
//go:build integration

package integration

import (
    "context"
    "database/sql"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/database"
    "github.com/gkbiswas/hotel-reviews-microservice/internal/testutil"
)

type DatabaseIntegrationTestSuite struct {
    suite.Suite
    container *postgres.PostgresContainer
    db        *sql.DB
    repo      *database.ReviewRepository
}

func (suite *DatabaseIntegrationTestSuite) SetupSuite() {
    ctx := context.Background()
    
    // Start PostgreSQL container
    container, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:15-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second)),
    )
    require.NoError(suite.T(), err)
    suite.container = container
    
    // Connect to database
    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    require.NoError(suite.T(), err)
    
    db, err := sql.Open("postgres", connStr)
    require.NoError(suite.T(), err)
    suite.db = db
    
    // Run migrations
    err = suite.runMigrations()
    require.NoError(suite.T(), err)
    
    // Create repository
    suite.repo = database.NewReviewRepository(db)
}

func (suite *DatabaseIntegrationTestSuite) TearDownSuite() {
    if suite.db != nil {
        suite.db.Close()
    }
    if suite.container != nil {
        suite.container.Terminate(context.Background())
    }
}

func (suite *DatabaseIntegrationTestSuite) SetupTest() {
    // Clean up database before each test
    _, err := suite.db.Exec("TRUNCATE TABLE reviews, hotels, providers CASCADE")
    require.NoError(suite.T(), err)
}

func (suite *DatabaseIntegrationTestSuite) TestCreateAndGetReview() {
    ctx := context.Background()
    
    // Create test data
    review := testutil.NewReview().Build()
    
    // Create review
    err := suite.repo.CreateReview(ctx, review)
    require.NoError(suite.T(), err)
    
    // Retrieve review
    retrieved, err := suite.repo.GetReview(ctx, review.ID)
    require.NoError(suite.T(), err)
    require.NotNil(suite.T(), retrieved)
    
    // Assertions
    testutil.AssertReviewEqual(suite.T(), review, retrieved)
    testutil.TimeNearEqual(suite.T(), review.CreatedAt, retrieved.CreatedAt)
}

func (suite *DatabaseIntegrationTestSuite) TestSearchReviewsWithFilters() {
    ctx := context.Background()
    hotelID := uuid.New()
    
    // Create test reviews
    reviews := []*domain.Review{
        testutil.NewReview().WithHotelID(hotelID).WithRating(5.0).Build(),
        testutil.NewReview().WithHotelID(hotelID).WithRating(4.0).Build(),
        testutil.NewReview().WithHotelID(hotelID).WithRating(3.0).Build(),
        testutil.NewReview().WithRating(5.0).Build(), // Different hotel
    }
    
    for _, review := range reviews {
        err := suite.repo.CreateReview(ctx, review)
        require.NoError(suite.T(), err)
    }
    
    // Test search with hotel filter
    filters := &domain.SearchFilters{
        HotelID: hotelID,
        Limit:   10,
    }
    
    results, err := suite.repo.SearchReviews(ctx, filters)
    require.NoError(suite.T(), err)
    assert.Len(suite.T(), results, 3) // Only reviews for specific hotel
    
    // Test search with rating filter
    filters = &domain.SearchFilters{
        HotelID:   hotelID,
        MinRating: 4.0,
        Limit:     10,
    }
    
    results, err = suite.repo.SearchReviews(ctx, filters)
    require.NoError(suite.T(), err)
    assert.Len(suite.T(), results, 2) // Only reviews with rating >= 4.0
}

func (suite *DatabaseIntegrationTestSuite) runMigrations() error {
    // Implementation to run database migrations
    migrator := migrate.NewMigrator(suite.db, "file://../../migrations")
    return migrator.Up()
}

func TestDatabaseIntegration(t *testing.T) {
    suite.Run(t, new(DatabaseIntegrationTestSuite))
}
```

### API Integration Tests

```go
// tests/integration/api_test.go
//go:build integration

package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/application/dto"
    "github.com/gkbiswas/hotel-reviews-microservice/internal/server"
)

type APIIntegrationTestSuite struct {
    suite.Suite
    server     *httptest.Server
    client     *http.Client
    authToken  string
}

func (suite *APIIntegrationTestSuite) SetupSuite() {
    // Setup test server
    gin.SetMode(gin.TestMode)
    
    app := server.NewServer(&server.Config{
        Database: testDatabaseConfig,
        Redis:    testRedisConfig,
        Auth:     testAuthConfig,
    })
    
    suite.server = httptest.NewServer(app.Handler())
    suite.client = &http.Client{}
    
    // Get auth token for tests
    suite.authToken = suite.getTestAuthToken()
}

func (suite *APIIntegrationTestSuite) TearDownSuite() {
    suite.server.Close()
}

func (suite *APIIntegrationTestSuite) TestCreateReviewFlow() {
    // Create review request
    request := dto.CreateReviewRequest{
        HotelID:        uuid.New(),
        ProviderID:     uuid.New(),
        Title:          "Great hotel experience",
        Content:        "Had an amazing stay at this hotel. Staff was friendly and rooms were clean.",
        Rating:         4.5,
        AuthorName:     "John Doe",
        AuthorLocation: "New York, NY",
        ReviewDate:     time.Now(),
    }
    
    // Send create request
    response := suite.postJSON("/api/v1/reviews", request)
    assert.Equal(suite.T(), http.StatusCreated, response.StatusCode)
    
    // Parse response
    var reviewResponse dto.ReviewResponse
    err := json.NewDecoder(response.Body).Decode(&reviewResponse)
    require.NoError(suite.T(), err)
    
    // Validate response
    assert.NotEmpty(suite.T(), reviewResponse.ID)
    assert.Equal(suite.T(), request.Title, reviewResponse.Title)
    assert.Equal(suite.T(), request.Rating, reviewResponse.Rating)
    
    // Test get review
    getResponse := suite.get(fmt.Sprintf("/api/v1/reviews/%s", reviewResponse.ID))
    assert.Equal(suite.T(), http.StatusOK, getResponse.StatusCode)
    
    var getReviewResponse dto.ReviewResponse
    err = json.NewDecoder(getResponse.Body).Decode(&getReviewResponse)
    require.NoError(suite.T(), err)
    
    assert.Equal(suite.T(), reviewResponse.ID, getReviewResponse.ID)
    assert.Equal(suite.T(), reviewResponse.Title, getReviewResponse.Title)
}

func (suite *APIIntegrationTestSuite) TestReviewValidation() {
    // Test with invalid rating
    request := dto.CreateReviewRequest{
        HotelID:    uuid.New(),
        ProviderID: uuid.New(),
        Title:      "Invalid review",
        Content:    "This review has invalid rating",
        Rating:     -1.0, // Invalid rating
        AuthorName: "Test User",
        ReviewDate: time.Now(),
    }
    
    response := suite.postJSON("/api/v1/reviews", request)
    assert.Equal(suite.T(), http.StatusBadRequest, response.StatusCode)
    
    var errorResponse dto.ErrorResponse
    err := json.NewDecoder(response.Body).Decode(&errorResponse)
    require.NoError(suite.T(), err)
    
    assert.Equal(suite.T(), "VALIDATION_ERROR", errorResponse.Error.Code)
    assert.Contains(suite.T(), errorResponse.Error.Message, "rating")
}

func (suite *APIIntegrationTestSuite) TestAuthentication() {
    // Test without authentication
    request := dto.CreateReviewRequest{
        HotelID:    uuid.New(),
        ProviderID: uuid.New(),
        Title:      "Test review",
        Content:    "Test content",
        Rating:     4.0,
        AuthorName: "Test User",
        ReviewDate: time.Now(),
    }
    
    response := suite.postJSONWithoutAuth("/api/v1/reviews", request)
    assert.Equal(suite.T(), http.StatusUnauthorized, response.StatusCode)
}

func (suite *APIIntegrationTestSuite) TestRateLimiting() {
    // Make many requests quickly to trigger rate limiting
    for i := 0; i < 200; i++ {
        response := suite.get("/api/v1/hotels")
        if response.StatusCode == http.StatusTooManyRequests {
            // Rate limiting triggered
            return
        }
    }
    
    suite.T().Fatal("Rate limiting was not triggered")
}

// Helper methods
func (suite *APIIntegrationTestSuite) get(path string) *http.Response {
    req, _ := http.NewRequest("GET", suite.server.URL+path, nil)
    req.Header.Set("Authorization", "Bearer "+suite.authToken)
    resp, _ := suite.client.Do(req)
    return resp
}

func (suite *APIIntegrationTestSuite) postJSON(path string, data interface{}) *http.Response {
    jsonData, _ := json.Marshal(data)
    req, _ := http.NewRequest("POST", suite.server.URL+path, bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+suite.authToken)
    resp, _ := suite.client.Do(req)
    return resp
}

func (suite *APIIntegrationTestSuite) postJSONWithoutAuth(path string, data interface{}) *http.Response {
    jsonData, _ := json.Marshal(data)
    req, _ := http.NewRequest("POST", suite.server.URL+path, bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    resp, _ := suite.client.Do(req)
    return resp
}

func (suite *APIIntegrationTestSuite) getTestAuthToken() string {
    // Implementation to get test authentication token
    // This could involve creating a test user and getting a JWT token
    return "test-jwt-token"
}

func TestAPIIntegration(t *testing.T) {
    suite.Run(t, new(APIIntegrationTestSuite))
}
```

## End-to-End Testing

### E2E Test Setup

```go
// tests/e2e/review_flow_test.go
//go:build e2e

package e2e

import (
    "encoding/json"
    "testing"
    "time"
    
    "github.com/gavv/httpexpect/v2"
    "github.com/stretchr/testify/suite"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/application/dto"
)

type ReviewFlowTestSuite struct {
    suite.Suite
    api   *httpexpect.Expect
    token string
}

func (suite *ReviewFlowTestSuite) SetupSuite() {
    // Setup HTTP expect client
    suite.api = httpexpect.WithConfig(httpexpect.Config{
        BaseURL:  getTestBaseURL(),
        Reporter: httpexpect.NewAssertReporter(suite.T()),
        Printers: []httpexpect.Printer{
            httpexpect.NewDebugPrinter(suite.T(), true),
        },
    })
    
    // Get authentication token
    suite.token = suite.authenticateTestUser()
}

func (suite *ReviewFlowTestSuite) TestCompleteReviewWorkflow() {
    hotelID := uuid.New()
    providerID := uuid.New()
    
    // Step 1: Create a hotel (prerequisite)
    hotel := suite.createTestHotel()
    
    // Step 2: Create a review
    reviewRequest := dto.CreateReviewRequest{
        HotelID:        hotel.ID,
        ProviderID:     providerID,
        Title:          "Amazing stay at luxury hotel",
        Content:        "This hotel exceeded all expectations. The service was impeccable, rooms were spotless, and the location was perfect.",
        Rating:         5.0,
        Language:       "en",
        AuthorName:     "Alice Johnson",
        AuthorLocation: "San Francisco, CA",
        ReviewDate:     time.Now(),
    }
    
    review := suite.api.POST("/api/v1/reviews").
        WithHeader("Authorization", "Bearer "+suite.token).
        WithJSON(reviewRequest).
        Expect().
        Status(201).
        JSON().Object()
    
    reviewID := review.Value("id").String().NotEmpty().Raw()
    
    // Step 3: Verify review appears in hotel reviews
    hotelReviews := suite.api.GET("/api/v1/hotels/{hotelId}/reviews", hotel.ID).
        WithQuery("limit", 10).
        Expect().
        Status(200).
        JSON().Object()
    
    hotelReviews.Value("data").Array().Length().Equal(1)
    hotelReviews.Value("data").Array().Element(0).Object().
        Value("id").Equal(reviewID)
    
    // Step 4: Search for reviews
    searchResults := suite.api.POST("/api/v1/reviews/search").
        WithJSON(map[string]interface{}{
            "query": "amazing luxury",
            "filters": map[string]interface{}{
                "rating_min": 4.0,
            },
            "limit": 20,
        }).
        Expect().
        Status(200).
        JSON().Object()
    
    searchResults.Value("data").Array().Length().Gte(1)
    
    // Step 5: Update review
    updateRequest := dto.UpdateReviewRequest{
        Title:   "Updated: Amazing stay at luxury hotel",
        Content: reviewRequest.Content + " Updated with additional feedback.",
        Rating:  4.8,
    }
    
    updatedReview := suite.api.PUT("/api/v1/reviews/{reviewId}", reviewID).
        WithHeader("Authorization", "Bearer "+suite.token).
        WithJSON(updateRequest).
        Expect().
        Status(200).
        JSON().Object()
    
    updatedReview.Value("title").Equal("Updated: Amazing stay at luxury hotel")
    updatedReview.Value("rating").Equal(4.8)
    
    // Step 6: Get analytics
    analytics := suite.api.GET("/api/v1/analytics/hotels/{hotelId}/stats", hotel.ID).
        Expect().
        Status(200).
        JSON().Object()
    
    analytics.Value("review_count").Equal(1)
    analytics.Value("average_rating").Equal(4.8)
    
    // Step 7: Delete review
    suite.api.DELETE("/api/v1/reviews/{reviewId}", reviewID).
        WithHeader("Authorization", "Bearer "+suite.token).
        Expect().
        Status(204)
    
    // Step 8: Verify review is deleted
    suite.api.GET("/api/v1/reviews/{reviewId}", reviewID).
        WithHeader("Authorization", "Bearer "+suite.token).
        Expect().
        Status(404)
}

func (suite *ReviewFlowTestSuite) TestBulkReviewProcessing() {
    // Step 1: Submit bulk job
    bulkRequest := dto.BulkReviewRequest{
        ProviderID:  uuid.New(),
        FileURL:     "https://example.com/test-reviews.jsonl",
        CallbackURL: "https://webhook.example.com/bulk-complete",
    }
    
    bulkResponse := suite.api.POST("/api/v1/reviews/bulk").
        WithHeader("Authorization", "Bearer "+suite.token).
        WithJSON(bulkRequest).
        Expect().
        Status(202).
        JSON().Object()
    
    jobID := bulkResponse.Value("job_id").String().NotEmpty().Raw()
    
    // Step 2: Check job status
    statusResponse := suite.api.GET("/api/v1/reviews/bulk/{jobId}/status", jobID).
        WithHeader("Authorization", "Bearer "+suite.token).
        Expect().
        Status(200).
        JSON().Object()
    
    statusResponse.Value("job_id").Equal(jobID)
    statusResponse.Value("status").String().OneOf("queued", "processing", "completed")
    
    // Step 3: Wait for job completion (with timeout)
    suite.waitForJobCompletion(jobID, 60*time.Second)
}

func (suite *ReviewFlowTestSuite) waitForJobCompletion(jobID string, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    
    for time.Now().Before(deadline) {
        status := suite.api.GET("/api/v1/reviews/bulk/{jobId}/status", jobID).
            WithHeader("Authorization", "Bearer "+suite.token).
            Expect().
            Status(200).
            JSON().Object()
        
        jobStatus := status.Value("status").String().Raw()
        if jobStatus == "completed" {
            return
        }
        if jobStatus == "failed" {
            suite.T().Fatalf("Bulk job failed: %v", status.Raw())
        }
        
        time.Sleep(2 * time.Second)
    }
    
    suite.T().Fatal("Bulk job did not complete within timeout")
}

func (suite *ReviewFlowTestSuite) createTestHotel() *dto.HotelResponse {
    hotelRequest := dto.CreateHotelRequest{
        Name:        "Luxury Test Hotel",
        Description: "A premium hotel for testing purposes",
        Address: dto.Address{
            Street:     "123 Test Street",
            City:       "Test City",
            State:      "TC",
            Country:    "Test Country",
            PostalCode: "12345",
            Coordinates: dto.Coordinates{
                Latitude:  40.7128,
                Longitude: -74.0060,
            },
        },
        Amenities:   []string{"wifi", "pool", "spa", "gym"},
        PriceRange:  "$$$$",
        StarRating:  5,
    }
    
    hotel := suite.api.POST("/api/v1/hotels").
        WithHeader("Authorization", "Bearer "+suite.token).
        WithJSON(hotelRequest).
        Expect().
        Status(201).
        JSON().Object()
    
    var hotelResponse dto.HotelResponse
    err := json.Unmarshal([]byte(hotel.Raw()), &hotelResponse)
    suite.Require().NoError(err)
    
    return &hotelResponse
}

func (suite *ReviewFlowTestSuite) authenticateTestUser() string {
    loginRequest := dto.LoginRequest{
        Email:    "test@example.com",
        Password: "testpassword123",
    }
    
    loginResponse := suite.api.POST("/api/v1/auth/login").
        WithJSON(loginRequest).
        Expect().
        Status(200).
        JSON().Object()
    
    return loginResponse.Value("token").String().NotEmpty().Raw()
}

func getTestBaseURL() string {
    if url := os.Getenv("TEST_BASE_URL"); url != "" {
        return url
    }
    return "http://localhost:8080"
}

func TestReviewFlowE2E(t *testing.T) {
    suite.Run(t, new(ReviewFlowTestSuite))
}
```

## Performance Testing

### Load Test Integration

```go
// tests/performance/load_test.go
//go:build performance

package performance

import (
    "context"
    "sync"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/testutil"
)

func TestReviewCreationConcurrency(t *testing.T) {
    const (
        numWorkers  = 10
        reviewsPerWorker = 100
        totalReviews = numWorkers * reviewsPerWorker
    )
    
    service := setupPerformanceTestService(t)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    var wg sync.WaitGroup
    successCount := int64(0)
    errorCount := int64(0)
    
    start := time.Now()
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for j := 0; j < reviewsPerWorker; j++ {
                review := testutil.NewReview().
                    WithTitle(fmt.Sprintf("Load test review %d-%d", workerID, j)).
                    Build()
                
                err := service.CreateReview(ctx, review)
                if err != nil {
                    atomic.AddInt64(&errorCount, 1)
                } else {
                    atomic.AddInt64(&successCount, 1)
                }
            }
        }(i)
    }
    
    wg.Wait()
    duration := time.Since(start)
    
    // Performance assertions
    successRate := float64(successCount) / float64(totalReviews) * 100
    throughput := float64(successCount) / duration.Seconds()
    
    t.Logf("Performance Results:")
    t.Logf("  Total Reviews: %d", totalReviews)
    t.Logf("  Successful: %d", successCount)
    t.Logf("  Failed: %d", errorCount)
    t.Logf("  Success Rate: %.2f%%", successRate)
    t.Logf("  Duration: %v", duration)
    t.Logf("  Throughput: %.2f reviews/sec", throughput)
    
    assert.GreaterOrEqual(t, successRate, 95.0, "Success rate should be at least 95%")
    assert.GreaterOrEqual(t, throughput, 100.0, "Throughput should be at least 100 reviews/sec")
}

func BenchmarkDatabaseOperations(b *testing.B) {
    service := setupBenchmarkService(b)
    ctx := context.Background()
    
    // Pre-create reviews for search benchmarks
    reviews := make([]*domain.Review, 1000)
    for i := 0; i < len(reviews); i++ {
        reviews[i] = testutil.NewReview().Build()
        service.CreateReview(ctx, reviews[i])
    }
    
    b.Run("CreateReview", func(b *testing.B) {
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            review := testutil.NewReview().Build()
            err := service.CreateReview(ctx, review)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
    
    b.Run("GetReview", func(b *testing.B) {
        reviewID := reviews[0].ID
        b.ResetTimer()
        
        for i := 0; i < b.N; i++ {
            _, err := service.GetReview(ctx, reviewID)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
    
    b.Run("SearchReviews", func(b *testing.B) {
        filters := &domain.SearchFilters{
            HotelID: reviews[0].HotelID,
            Limit:   20,
        }
        b.ResetTimer()
        
        for i := 0; i < b.N; i++ {
            _, err := service.SearchReviews(ctx, filters)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## Test Configuration and Setup

### Test Configuration

```yaml
# configs/test.yaml
server:
  host: "localhost"
  port: 8081
  read_timeout: 10s
  write_timeout: 10s

database:
  host: "localhost"
  port: 5433  # Different port for test database
  user: "test_user"
  password: "test_password"
  name: "hotel_reviews_test"
  ssl_mode: "disable"
  max_conns: 10

redis:
  addr: "localhost:6380"  # Different port for test Redis
  password: ""
  db: 1  # Different database number

logging:
  level: "debug"
  format: "text"

testing:
  cleanup_after_test: true
  parallel_execution: false
  timeout: 30s
```

### Test Makefile Targets

```makefile
# Testing targets
.PHONY: test test-unit test-integration test-e2e test-performance test-coverage

test: test-unit test-integration  ## Run all tests

test-unit:  ## Run unit tests
	go test -v -race -count=1 ./internal/...

test-integration:  ## Run integration tests
	docker-compose -f docker-compose.test.yml up -d
	go test -v -tags=integration -count=1 ./tests/integration/...
	docker-compose -f docker-compose.test.yml down

test-e2e:  ## Run end-to-end tests
	docker-compose -f docker-compose.e2e.yml up -d
	sleep 10  # Wait for services to be ready
	go test -v -tags=e2e -count=1 ./tests/e2e/...
	docker-compose -f docker-compose.e2e.yml down

test-performance:  ## Run performance tests
	go test -v -tags=performance -count=1 ./tests/performance/...

test-coverage:  ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-benchmark:  ## Run benchmarks
	go test -bench=. -benchmem -count=3 ./internal/...

test-race:  ## Run tests with race detection
	go test -v -race -count=10 ./internal/...

test-clean:  ## Clean test artifacts
	rm -f coverage.out coverage.html
	docker-compose -f docker-compose.test.yml down -v
	docker-compose -f docker-compose.e2e.yml down -v
```

## Best Practices Summary

### Test Organization
- **Structure**: Group tests by functionality and layer
- **Naming**: Use descriptive test names that explain the scenario
- **Setup**: Use test suites for complex setup/teardown
- **Isolation**: Ensure tests don't depend on each other

### Test Data Management
- **Builders**: Use builder pattern for test data creation
- **Fixtures**: Create reusable test fixtures
- **Cleanup**: Always clean up test data
- **Randomization**: Use random data where appropriate

### Performance Testing
- **Baselines**: Establish performance baselines
- **Monitoring**: Monitor resource usage during tests
- **Realistic Load**: Use realistic data volumes and patterns
- **Continuous**: Run performance tests in CI/CD

### Test Maintenance
- **Regular Updates**: Keep tests updated with code changes
- **Flaky Tests**: Address and fix flaky tests immediately
- **Documentation**: Document complex test scenarios
- **Review**: Include tests in code review process

This comprehensive testing guide ensures high-quality, reliable testing across all levels of the Hotel Reviews Microservice.