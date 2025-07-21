package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// IntegrationTestSuite holds the test environment
type IntegrationTestSuite struct {
	// Containers
	postgresContainer   *postgres.PostgresContainer
	redisContainer      testcontainers.Container
	localstackContainer *localstack.LocalStackContainer

	// Services
	db            *gorm.DB
	redisClient   *redis.Client
	reviewService domain.ReviewService
	s3Client      *infrastructure.S3Client

	// HTTP Server
	router *gin.Engine
	server *httptest.Server

	// Test data
	testProvider *domain.Provider
	testHotel    *domain.Hotel
	testReviews  []*domain.Review

	// Logger
	logger *logger.Logger
}

// SetupSuite initializes the complete test environment
func SetupSuite(t *testing.T) *IntegrationTestSuite {
	suite := &IntegrationTestSuite{}
	ctx := context.Background()

	// Initialize logger
	var err error
	suite.logger, err = logger.New(&logger.Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	require.NoError(t, err)

	// Start containers
	suite.setupContainers(t, ctx)

	// Initialize database
	suite.setupDatabase(t, ctx)

	// Initialize Redis
	suite.setupRedis(t, ctx)

	// Initialize S3
	suite.setupS3(t, ctx)

	// Initialize services
	suite.setupServices(t, ctx)

	// Setup HTTP server
	suite.setupHTTPServer(t)

	// Create test data
	suite.createTestData(t, ctx)

	return suite
}

// setupContainers initializes all test containers
func (suite *IntegrationTestSuite) setupContainers(t *testing.T, ctx context.Context) {
	var err error

	// PostgreSQL
	t.Log("Starting PostgreSQL container...")
	suite.postgresContainer, err = postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("integration_test"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	// Redis
	t.Log("Starting Redis container...")
	suite.redisContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	require.NoError(t, err)

	// LocalStack for S3
	t.Log("Starting LocalStack container...")
	suite.localstackContainer, err = localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:3.0"),
	)
	require.NoError(t, err)
}

// setupDatabase initializes the database connection and schema
func (suite *IntegrationTestSuite) setupDatabase(t *testing.T, ctx context.Context) {
	connectionString, err := suite.postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	suite.db, err = gorm.Open(postgresDriver.Open(connectionString), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate schema
	err = suite.db.AutoMigrate(
		&domain.Hotel{},
		&domain.Provider{},
		&domain.ReviewerInfo{},
		&domain.Review{},
		&domain.ReviewSummary{},
		&domain.ReviewProcessingStatus{},
	)
	require.NoError(t, err)

	t.Log("Database initialized successfully")
}

// setupRedis initializes Redis connection
func (suite *IntegrationTestSuite) setupRedis(t *testing.T, ctx context.Context) {
	redisHost, err := suite.redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := suite.redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})

	_, err = suite.redisClient.Ping(ctx).Result()
	require.NoError(t, err)

	t.Log("Redis initialized successfully")
}

// setupS3 initializes S3 client with LocalStack
func (suite *IntegrationTestSuite) setupS3(t *testing.T, ctx context.Context) {
	endpoint, err := suite.localstackContainer.PortEndpoint(ctx, "4566", "http")
	require.NoError(t, err)

	s3Config := &config.S3Config{
		Region:          "us-east-1",
		Bucket:          "test-bucket",
		Endpoint:        endpoint,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}

	suite.s3Client, err = infrastructure.NewS3Client(s3Config, suite.logger)
	require.NoError(t, err)

	// Create test bucket
	err = suite.s3Client.CreateBucket(ctx, "test-bucket")
	require.NoError(t, err)

	t.Log("S3 initialized successfully")
}

// setupServices initializes all domain services
func (suite *IntegrationTestSuite) setupServices(t *testing.T, ctx context.Context) {
	// Initialize infrastructure components
	database := &infrastructure.Database{DB: suite.db}
	reviewRepo := infrastructure.NewReviewRepository(database, suite.logger)
	
	// Initialize cache service
	cacheService := infrastructure.NewRedisCacheService(suite.redisClient, suite.logger)
	
	// Initialize JSON processor
	jsonProcessor := infrastructure.NewJSONLinesProcessor(reviewRepo, suite.logger)

	// Create slog logger for components that need it
	slogLogger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create review service
	suite.reviewService = domain.NewReviewService(
		reviewRepo,
		suite.s3Client,
		jsonProcessor,
		nil, // notification service
		cacheService,
		nil, // metrics service
		nil, // event publisher
		slogLogger,
	)

	t.Log("Services initialized successfully")
}

// setupHTTPServer creates the HTTP server with all routes and middleware
func (suite *IntegrationTestSuite) setupHTTPServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suite.router = gin.New()

	// Add basic middleware
	suite.router.Use(gin.Recovery())

	// Security configuration for testing
	securityConfig := &config.SecurityConfig{
		RateLimit:       1000, // High limit for testing
		RateLimitWindow: time.Minute,
	}

	// Add security middleware
	securityMiddleware := middleware.NewSecurityMiddleware(securityConfig, suite.logger, suite.redisClient)
	suite.router.Use(securityMiddleware.SecurityHeadersMiddleware())
	suite.router.Use(securityMiddleware.CORSMiddleware())
	suite.router.Use(securityMiddleware.InputValidationMiddleware())

	// Initialize handlers
	handlers := application.NewSimplifiedIntegratedHandlers(
		suite.reviewService,
		nil, // auth service
		nil, // rbac service
		nil, // circuit breaker
		nil, // retry manager
		nil, // redis client
		nil, // kafka producer
		suite.s3Client,
		nil, // error handler
		nil, // processing engine
		suite.logger,
	)

	// Hotel service adapter
	hotelService := application.NewHotelServiceAdapter(suite.reviewService)
	hotelHandlers := application.NewHotelHandlers(hotelService, suite.logger)

	// Provider service adapter  
	providerService := application.NewProviderServiceAdapter(suite.reviewService)
	providerHandlers := application.NewProviderHandlers(providerService, suite.logger)

	// Search and analytics handlers
	searchAnalyticsHandlers := application.NewSearchAnalyticsHandlers(suite.reviewService, suite.logger)

	// API routes
	api := suite.router.Group("/api/v1")
	{
		// Review routes
		reviews := api.Group("/reviews")
		{
			reviews.GET("", handlers.ListReviews)
			reviews.GET("/:id", handlers.GetReview)
			reviews.POST("", handlers.CreateReview)
			reviews.PUT("/:id", handlers.UpdateReview)
			reviews.DELETE("/:id", handlers.DeleteReview)
		}

		// Hotel routes
		hotels := api.Group("/hotels")
		{
			hotels.GET("", hotelHandlers.GetHotels)
			hotels.GET("/:id", hotelHandlers.GetHotel)
			hotels.POST("", hotelHandlers.CreateHotel)
			hotels.PUT("/:id", hotelHandlers.UpdateHotel)
			hotels.DELETE("/:id", hotelHandlers.DeleteHotel)
		}

		// Provider routes
		providers := api.Group("/providers")
		{
			providers.GET("", providerHandlers.GetProviders)
			providers.GET("/:id", providerHandlers.GetProvider)
			providers.POST("", providerHandlers.CreateProvider)
			providers.PUT("/:id", providerHandlers.UpdateProvider)
			providers.DELETE("/:id", providerHandlers.DeleteProvider)
		}

		// Search routes
		search := api.Group("/search")
		{
			search.GET("/reviews", searchAnalyticsHandlers.SearchReviews)
			search.GET("/hotels", searchAnalyticsHandlers.SearchHotels)
		}

		// Analytics routes
		analytics := api.Group("/analytics")
		{
			analytics.GET("/overview", searchAnalyticsHandlers.GetOverallAnalytics)
			analytics.GET("/hotels/top-rated", searchAnalyticsHandlers.GetTopRatedHotels)
			analytics.GET("/hotels/:id/stats", searchAnalyticsHandlers.GetHotelStats)
			analytics.GET("/hotels/:id/summary", searchAnalyticsHandlers.GetReviewSummary)
			analytics.GET("/providers/:id/stats", searchAnalyticsHandlers.GetProviderStats)
			analytics.GET("/reviews/recent", searchAnalyticsHandlers.GetRecentReviews)
			analytics.GET("/trends/reviews", searchAnalyticsHandlers.GetReviewTrends)
		}
	}

	// Health check
	suite.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Start test server
	suite.server = httptest.NewServer(suite.router)
}

// createTestData creates initial test data
func (suite *IntegrationTestSuite) createTestData(t *testing.T, ctx context.Context) {
	// Create test provider
	suite.testProvider = &domain.Provider{
		ID:       uuid.New(),
		Name:     "Test Provider",
		BaseURL:  "https://testprovider.com",
		IsActive: true,
	}
	err := suite.reviewService.CreateProvider(ctx, suite.testProvider)
	require.NoError(t, err)

	// Create test hotel
	suite.testHotel = &domain.Hotel{
		ID:          uuid.New(),
		Name:        "Test Hotel",
		Address:     "123 Test Street",
		City:        "Test City",
		Country:     "Test Country",
		StarRating:  4,
		Description: "A wonderful test hotel",
		Latitude:    40.7128,
		Longitude:   -74.0060,
	}
	err = suite.reviewService.CreateHotel(ctx, suite.testHotel)
	require.NoError(t, err)

	// Create test reviews
	suite.testReviews = []*domain.Review{
		{
			ID:         uuid.New(),
			ProviderID: suite.testProvider.ID,
			HotelID:    suite.testHotel.ID,
			ExternalID: "review-1",
			Rating:     4.5,
			Title:      "Great stay!",
			Comment:    "Really enjoyed our stay at this hotel.",
			ReviewDate: time.Now().Add(-48 * time.Hour),
			Language:   "en",
		},
		{
			ID:         uuid.New(),
			ProviderID: suite.testProvider.ID,
			HotelID:    suite.testHotel.ID,
			ExternalID: "review-2",
			Rating:     3.0,
			Title:      "Average experience",
			Comment:    "The hotel was okay, nothing special.",
			ReviewDate: time.Now().Add(-24 * time.Hour),
			Language:   "en",
		},
	}

	for _, review := range suite.testReviews {
		err := suite.reviewService.CreateReview(ctx, review)
		require.NoError(t, err)
	}

	t.Log("Test data created successfully")
}

// Cleanup tears down the test environment
func (suite *IntegrationTestSuite) Cleanup(t *testing.T) {
	ctx := context.Background()

	if suite.server != nil {
		suite.server.Close()
	}

	if suite.redisClient != nil {
		suite.redisClient.Close()
	}

	if suite.postgresContainer != nil {
		suite.postgresContainer.Terminate(ctx)
	}

	if suite.redisContainer != nil {
		suite.redisContainer.Terminate(ctx)
	}

	if suite.localstackContainer != nil {
		suite.localstackContainer.Terminate(ctx)
	}
}

// Test suite execution

func TestComprehensiveIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite := SetupSuite(t)
	defer suite.Cleanup(t)

	// Run all integration tests
	t.Run("Hotel CRUD Operations", func(t *testing.T) {
		suite.TestHotelCRUD(t)
	})

	t.Run("Provider CRUD Operations", func(t *testing.T) {
		suite.TestProviderCRUD(t)
	})

	t.Run("Review CRUD Operations", func(t *testing.T) {
		suite.TestReviewCRUD(t)
	})

	t.Run("Search Functionality", func(t *testing.T) {
		suite.TestSearchFunctionality(t)
	})

	t.Run("Analytics Endpoints", func(t *testing.T) {
		suite.TestAnalyticsEndpoints(t)
	})

	t.Run("Cache Functionality", func(t *testing.T) {
		suite.TestCacheFunctionality(t)
	})

	t.Run("Error Handling", func(t *testing.T) {
		suite.TestErrorHandling(t)
	})

	t.Run("Security Middleware", func(t *testing.T) {
		suite.TestSecurityMiddleware(t)
	})

	t.Run("Data Validation", func(t *testing.T) {
		suite.TestDataValidation(t)
	})

	t.Run("Performance Under Load", func(t *testing.T) {
		suite.TestPerformanceUnderLoad(t)
	})
}

// TestHotelCRUD tests hotel CRUD operations
func (suite *IntegrationTestSuite) TestHotelCRUD(t *testing.T) {
	// Test Create Hotel
	t.Run("Create Hotel", func(t *testing.T) {
		hotelData := map[string]interface{}{
			"name":        "New Test Hotel",
			"address":     "456 New Street",
			"city":        "New City",
			"country":     "New Country",
			"star_rating": 5,
			"latitude":    51.5074,
			"longitude":   -0.1278,
		}

		body, _ := json.Marshal(hotelData)
		resp, err := http.Post(suite.server.URL+"/api/v1/hotels", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test Get All Hotels
	t.Run("Get All Hotels", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/hotels")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 1)
	})

	// Test Get Hotel by ID
	t.Run("Get Hotel by ID", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/hotels/%s", suite.server.URL, suite.testHotel.ID.String())
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Equal(t, suite.testHotel.Name, data["name"].(string))
	})

	// Test Update Hotel
	t.Run("Update Hotel", func(t *testing.T) {
		updateData := map[string]interface{}{
			"name":        "Updated Test Hotel",
			"star_rating": 5,
		}

		body, _ := json.Marshal(updateData)
		url := fmt.Sprintf("%s/api/v1/hotels/%s", suite.server.URL, suite.testHotel.ID.String())
		
		req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})
}

// TestProviderCRUD tests provider CRUD operations
func (suite *IntegrationTestSuite) TestProviderCRUD(t *testing.T) {
	// Test Create Provider
	t.Run("Create Provider", func(t *testing.T) {
		providerData := map[string]interface{}{
			"name":      "New Test Provider",
			"base_url":  "https://newtestprovider.com",
			"is_active": true,
		}

		body, _ := json.Marshal(providerData)
		resp, err := http.Post(suite.server.URL+"/api/v1/providers", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test Get All Providers
	t.Run("Get All Providers", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 1)
	})
}

// TestReviewCRUD tests review CRUD operations
func (suite *IntegrationTestSuite) TestReviewCRUD(t *testing.T) {
	// Test Create Review
	t.Run("Create Review", func(t *testing.T) {
		reviewData := map[string]interface{}{
			"provider_id":  suite.testProvider.ID.String(),
			"hotel_id":     suite.testHotel.ID.String(),
			"external_id":  "new-review-1",
			"rating":       4.0,
			"title":        "Nice stay",
			"comment":      "Had a pleasant experience at this hotel.",
			"review_date":  time.Now().Format(time.RFC3339),
			"language":     "en",
		}

		body, _ := json.Marshal(reviewData)
		resp, err := http.Post(suite.server.URL+"/api/v1/reviews", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test Get All Reviews
	t.Run("Get All Reviews", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/reviews")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 2) // We created 2 test reviews + 1 new
	})
}

// TestSearchFunctionality tests search capabilities
func (suite *IntegrationTestSuite) TestSearchFunctionality(t *testing.T) {
	// Test Review Search
	t.Run("Search Reviews", func(t *testing.T) {
		url := suite.server.URL + "/api/v1/search/reviews?q=Great&limit=10"
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test Hotel Search
	t.Run("Search Hotels", func(t *testing.T) {
		url := suite.server.URL + "/api/v1/search/hotels?q=Test&limit=10"
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})
}

// TestAnalyticsEndpoints tests analytics functionality
func (suite *IntegrationTestSuite) TestAnalyticsEndpoints(t *testing.T) {
	// Test Overall Analytics
	t.Run("Get Overall Analytics", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/analytics/overview")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test Top Rated Hotels
	t.Run("Get Top Rated Hotels", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/analytics/hotels/top-rated?limit=5")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test Hotel Stats
	t.Run("Get Hotel Stats", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/analytics/hotels/%s/stats", suite.server.URL, suite.testHotel.ID.String())
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test Recent Reviews
	t.Run("Get Recent Reviews", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/analytics/reviews/recent?limit=10")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})
}

// TestCacheFunctionality tests caching behavior
func (suite *IntegrationTestSuite) TestCacheFunctionality(t *testing.T) {
	t.Run("Cache Performance", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/analytics/hotels/%s/summary", suite.server.URL, suite.testHotel.ID.String())

		// First request (should hit database)
		start := time.Now()
		resp1, err := http.Get(url)
		require.NoError(t, err)
		resp1.Body.Close()
		dbTime := time.Since(start)

		// Second request (should hit cache)
		start = time.Now()
		resp2, err := http.Get(url)
		require.NoError(t, err)
		resp2.Body.Close()
		cacheTime := time.Since(start)

		// Cache should be significantly faster
		assert.Less(t, cacheTime.Nanoseconds(), dbTime.Nanoseconds())
		assert.Equal(t, http.StatusOK, resp1.StatusCode)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
	})
}

// TestErrorHandling tests error scenarios
func (suite *IntegrationTestSuite) TestErrorHandling(t *testing.T) {
	// Test Not Found Error
	t.Run("Not Found Error", func(t *testing.T) {
		randomID := uuid.New().String()
		url := fmt.Sprintf("%s/api/v1/hotels/%s", suite.server.URL, randomID)
		
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"], "not found")
	})

	// Test Invalid Data Error
	t.Run("Invalid Data Error", func(t *testing.T) {
		invalidData := map[string]interface{}{
			"rating": 6.0, // Invalid rating > 5
			"title":  "",  // Empty title
		}

		body, _ := json.Marshal(invalidData)
		resp, err := http.Post(suite.server.URL+"/api/v1/reviews", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})
}

// TestSecurityMiddleware tests security measures
func (suite *IntegrationTestSuite) TestSecurityMiddleware(t *testing.T) {
	// Test Security Headers
	t.Run("Security Headers", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/hotels")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Check security headers
		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
		assert.NotEmpty(t, resp.Header.Get("Content-Security-Policy"))
	})

	// Test CORS Headers
	t.Run("CORS Headers", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", suite.server.URL+"/api/v1/hotels", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://localhost:3000")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Methods"))
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Headers"))
	})

	// Test SQL Injection Protection
	t.Run("SQL Injection Protection", func(t *testing.T) {
		maliciousQuery := "'; DROP TABLE hotels; --"
		url := fmt.Sprintf("%s/api/v1/search/hotels?q=%s", suite.server.URL, maliciousQuery)
		
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should reject malicious input
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// TestDataValidation tests input validation
func (suite *IntegrationTestSuite) TestDataValidation(t *testing.T) {
	// Test Invalid Content Type
	t.Run("Invalid Content Type", func(t *testing.T) {
		req, err := http.NewRequest("POST", suite.server.URL+"/api/v1/hotels", strings.NewReader("<xml>data</xml>"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/xml")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	})

	// Test Large Request Body
	t.Run("Large Request Body", func(t *testing.T) {
		largeData := strings.Repeat("a", 11*1024*1024) // 11MB
		req, err := http.NewRequest("POST", suite.server.URL+"/api/v1/hotels", strings.NewReader(largeData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})
}

// TestPerformanceUnderLoad tests system performance
func (suite *IntegrationTestSuite) TestPerformanceUnderLoad(t *testing.T) {
	t.Run("Concurrent Requests", func(t *testing.T) {
		const numRequests = 50
		const numWorkers = 10

		requests := make(chan string, numRequests)
		results := make(chan int, numRequests)

		// Fill request channel
		for i := 0; i < numRequests; i++ {
			requests <- suite.server.URL + "/api/v1/hotels"
		}
		close(requests)

		// Start workers
		for i := 0; i < numWorkers; i++ {
			go func() {
				for url := range requests {
					resp, err := http.Get(url)
					if err != nil {
						results <- 500
					} else {
						results <- resp.StatusCode
						resp.Body.Close()
					}
				}
			}()
		}

		// Collect results
		successCount := 0
		for i := 0; i < numRequests; i++ {
			statusCode := <-results
			if statusCode == 200 {
				successCount++
			}
		}

		// Assert success rate
		successRate := float64(successCount) / float64(numRequests)
		assert.GreaterOrEqual(t, successRate, 0.95) // 95% success rate minimum
	})
}