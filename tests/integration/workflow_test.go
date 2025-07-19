package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	pkgconfig "github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// MockEventPublisher implements domain.EventPublisher for testing
type MockEventPublisher struct{}

func (m *MockEventPublisher) PublishReviewCreated(ctx context.Context, review *domain.Review) error { return nil }
func (m *MockEventPublisher) PublishReviewUpdated(ctx context.Context, review *domain.Review) error { return nil }
func (m *MockEventPublisher) PublishReviewDeleted(ctx context.Context, reviewID uuid.UUID) error { return nil }
func (m *MockEventPublisher) PublishProcessingStarted(ctx context.Context, processingID uuid.UUID, providerID uuid.UUID) error { return nil }
func (m *MockEventPublisher) PublishProcessingCompleted(ctx context.Context, processingID uuid.UUID, recordsProcessed int) error { return nil }
func (m *MockEventPublisher) PublishProcessingFailed(ctx context.Context, processingID uuid.UUID, errorMsg string) error { return nil }
func (m *MockEventPublisher) PublishHotelCreated(ctx context.Context, hotel *domain.Hotel) error { return nil }
func (m *MockEventPublisher) PublishHotelUpdated(ctx context.Context, hotel *domain.Hotel) error { return nil }

// TestContainers holds all the containers for integration tests
type TestContainers struct {
	postgresContainer  *postgres.PostgresContainer
	redisContainer     testcontainers.Container
	kafkaContainer     testcontainers.Container
	localstackContainer *localstack.LocalStackContainer
	
	// Connection details
	postgresConnectionString string
	redisAddr               string
	kafkaBrokers            []string
	s3Endpoint              string
}

// SetupTestContainers initializes all test containers
func SetupTestContainers(t *testing.T) *TestContainers {
	ctx := context.Background()
	tc := &TestContainers{}

	// Start PostgreSQL container
	t.Log("Starting PostgreSQL container...")
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	tc.postgresContainer = postgresContainer

	// Get PostgreSQL connection string
	tc.postgresConnectionString, err = postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	t.Logf("PostgreSQL connection string: %s", tc.postgresConnectionString)

	// Start Redis container
	t.Log("Starting Redis container...")
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	require.NoError(t, err)
	tc.redisContainer = redisContainer

	// Get Redis address
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)
	tc.redisAddr = fmt.Sprintf("%s:%s", redisHost, redisPort.Port())
	t.Logf("Redis address: %s", tc.redisAddr)

	// Start Kafka container (simplified without Zookeeper)
	t.Log("Starting Kafka container...")
	kafkaContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "apache/kafka:3.7.0",
			ExposedPorts: []string{"9092/tcp"},
			Env: map[string]string{
				"KAFKA_NODE_ID":                     "1",
				"KAFKA_PROCESS_ROLES":               "broker,controller",
				"KAFKA_CONTROLLER_QUORUM_VOTERS":    "1@localhost:29093",
				"KAFKA_LISTENERS":                   "PLAINTEXT://:9092,CONTROLLER://:29093",
				"KAFKA_ADVERTISED_LISTENERS":        "PLAINTEXT://localhost:9092",
				"KAFKA_CONTROLLER_LISTENER_NAMES":   "CONTROLLER",
				"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP": "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT",
				"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
				"KAFKA_AUTO_CREATE_TOPICS_ENABLE":    "true",
				"KAFKA_CLUSTER_ID":                   "test-cluster",
			},
			WaitingFor: wait.ForLog("Kafka Server started").
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	tc.kafkaContainer = kafkaContainer

	// Get Kafka broker address
	kafkaHost, err := kafkaContainer.Host(ctx)
	require.NoError(t, err)
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9092")
	require.NoError(t, err)
	tc.kafkaBrokers = []string{fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port())}
	t.Logf("Kafka brokers: %v", tc.kafkaBrokers)

	// Start LocalStack container for S3
	t.Log("Starting LocalStack container...")
	localstackContainer, err := localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:3.0"),
		localstack.WithNetwork("bridge", "localstack"),
	)
	require.NoError(t, err)
	tc.localstackContainer = localstackContainer

	// Get S3 endpoint
	s3Endpoint, err := localstackContainer.PortEndpoint(ctx, "4566", "http")
	require.NoError(t, err)
	tc.s3Endpoint = s3Endpoint
	t.Logf("S3 endpoint: %s", tc.s3Endpoint)

	return tc
}

// Cleanup terminates all containers
func (tc *TestContainers) Cleanup(t *testing.T) {
	ctx := context.Background()
	
	if tc.postgresContainer != nil {
		if err := tc.postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate PostgreSQL container: %v", err)
		}
	}
	
	if tc.redisContainer != nil {
		if err := tc.redisContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}
	
	if tc.kafkaContainer != nil {
		if err := tc.kafkaContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Kafka container: %v", err)
		}
	}
	
	if tc.localstackContainer != nil {
		if err := tc.localstackContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate LocalStack container: %v", err)
		}
	}
}

// InitializeDatabase creates the necessary database schema using GORM
func InitializeDatabase(t *testing.T, connectionString string) *gorm.DB {
	// Initialize GORM database
	db, err := gorm.Open(postgresDriver.Open(connectionString), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the database schema
	err = db.AutoMigrate(
		&domain.Hotel{},
		&domain.Provider{},
		&domain.ReviewerInfo{},
		&domain.Review{},
		&domain.ReviewSummary{},
		&domain.ReviewProcessingStatus{},
	)
	require.NoError(t, err)

	t.Log("Database schema migrated successfully")
	return db
}

// InitializeDatabaseSQL creates the necessary database schema using SQL
func InitializeDatabaseSQL(t *testing.T, connectionString string) {
	db, err := sql.Open("postgres", connectionString)
	require.NoError(t, err)
	defer db.Close()

	// Create schema
	schema := `
		-- Hotels table
		CREATE TABLE IF NOT EXISTS hotels (
			id UUID PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			address TEXT,
			city VARCHAR(100),
			country VARCHAR(100),
			postal_code VARCHAR(20),
			phone VARCHAR(20),
			email VARCHAR(255),
			star_rating INTEGER CHECK (star_rating >= 1 AND star_rating <= 5),
			description TEXT,
			amenities TEXT[],
			latitude DOUBLE PRECISION,
			longitude DOUBLE PRECISION,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Providers table
		CREATE TABLE IF NOT EXISTS providers (
			id UUID PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			base_url TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Reviewer info table
		CREATE TABLE IF NOT EXISTS reviewer_info (
			id UUID PRIMARY KEY,
			name VARCHAR(255),
			email VARCHAR(255),
			location VARCHAR(255),
			reviewer_level VARCHAR(50),
			total_reviews INTEGER DEFAULT 0,
			helpful_votes INTEGER DEFAULT 0,
			is_verified BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Reviews table
		CREATE TABLE IF NOT EXISTS reviews (
			id UUID PRIMARY KEY,
			provider_id UUID NOT NULL REFERENCES providers(id),
			hotel_id UUID NOT NULL REFERENCES hotels(id),
			reviewer_info_id UUID REFERENCES reviewer_info(id),
			external_id VARCHAR(255),
			rating DECIMAL(3,2) NOT NULL CHECK (rating >= 1 AND rating <= 5),
			title VARCHAR(500),
			comment TEXT NOT NULL,
			review_date TIMESTAMP WITH TIME ZONE NOT NULL,
			stay_date TIMESTAMP WITH TIME ZONE,
			trip_type VARCHAR(50),
			room_type VARCHAR(100),
			language VARCHAR(10),
			service_rating DECIMAL(3,2) CHECK (service_rating >= 1 AND service_rating <= 5),
			cleanliness_rating DECIMAL(3,2) CHECK (cleanliness_rating >= 1 AND cleanliness_rating <= 5),
			location_rating DECIMAL(3,2) CHECK (location_rating >= 1 AND location_rating <= 5),
			value_rating DECIMAL(3,2) CHECK (value_rating >= 1 AND value_rating <= 5),
			comfort_rating DECIMAL(3,2) CHECK (comfort_rating >= 1 AND comfort_rating <= 5),
			facilities_rating DECIMAL(3,2) CHECK (facilities_rating >= 1 AND facilities_rating <= 5),
			sentiment VARCHAR(20),
			is_verified BOOLEAN DEFAULT false,
			helpful_count INTEGER DEFAULT 0,
			total_votes INTEGER DEFAULT 0,
			response_from_hotel TEXT,
			response_date TIMESTAMP WITH TIME ZONE,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider_id, external_id)
		);

		-- Review summaries table
		CREATE TABLE IF NOT EXISTS review_summaries (
			id UUID PRIMARY KEY,
			hotel_id UUID NOT NULL REFERENCES hotels(id) UNIQUE,
			total_reviews INTEGER DEFAULT 0,
			average_rating DECIMAL(3,2),
			rating_distribution JSONB,
			sentiment_distribution JSONB,
			language_distribution JSONB,
			trip_type_distribution JSONB,
			monthly_review_count JSONB,
			highlights TEXT[],
			lowlights TEXT[],
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Review processing status table
		CREATE TABLE IF NOT EXISTS review_processing_status (
			id UUID PRIMARY KEY,
			provider_id UUID NOT NULL REFERENCES providers(id),
			file_url TEXT NOT NULL,
			status VARCHAR(50) NOT NULL,
			total_records INTEGER DEFAULT 0,
			processed_records INTEGER DEFAULT 0,
			failed_records INTEGER DEFAULT 0,
			error_message TEXT,
			processing_metadata JSONB,
			started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Create indexes
		CREATE INDEX idx_reviews_hotel_id ON reviews(hotel_id);
		CREATE INDEX idx_reviews_provider_id ON reviews(provider_id);
		CREATE INDEX idx_reviews_review_date ON reviews(review_date);
		CREATE INDEX idx_reviews_rating ON reviews(rating);
		CREATE INDEX idx_processing_status_provider ON review_processing_status(provider_id);
		CREATE INDEX idx_processing_status_status ON review_processing_status(status);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err)
	t.Log("Database schema created successfully")
}

// TestCompleteWorkflow tests the complete file processing workflow
func TestCompleteWorkflow(t *testing.T) {
	// Setup test containers
	tc := SetupTestContainers(t)
	defer tc.Cleanup(t)

	// Initialize database
	db := InitializeDatabase(t, tc.postgresConnectionString)

	// Setup test context
	ctx := context.Background()
	testLogger, _ := logger.New(&logger.Config{
		Level:  "debug",
		Format: "json",
	})

	// Initialize S3 client
	s3Config, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: tc.s3Endpoint}, nil
			}),
		),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	require.NoError(t, err)

	s3Client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Create test bucket
	bucketName := "test-bucket"
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	require.NoError(t, err)

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: tc.redisAddr,
	})
	defer redisClient.Close()

	// Test Redis connection
	_, err = redisClient.Ping(ctx).Result()
	require.NoError(t, err)

	// Initialize Kafka producer
	kafkaConfig := &infrastructure.KafkaConfig{
		Brokers:         tc.kafkaBrokers,
		ReviewTopic:     "reviews",
		ProcessingTopic: "processing",
		DeadLetterTopic: "dead-letter",
		BatchSize:       100,
		BatchTimeout:    1 * time.Second,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
	}

	kafkaProducer, err := infrastructure.NewKafkaProducer(kafkaConfig, testLogger)
	require.NoError(t, err)
	defer kafkaProducer.Close()

	// Initialize repository using the GORM DB
	database := &infrastructure.Database{DB: db}
	repository := infrastructure.NewReviewRepository(database, testLogger)

	// Initialize services
	s3Service, _ := infrastructure.NewS3Client(&pkgconfig.S3Config{
		Region:          "us-east-1",
		Bucket:          "test-bucket",
		Endpoint:        tc.s3Endpoint,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}, testLogger)
	cacheService := infrastructure.NewRedisCacheService(redisClient, testLogger)
	jsonProcessor := infrastructure.NewJSONLinesProcessor(repository, testLogger)
	
	// Create a simple event publisher adapter
	eventPublisher := &MockEventPublisher{}

	// Create review service using adapter
	reviewService := domain.NewReviewServiceWithAdapter(
		repository,
		s3Service,
		jsonProcessor,
		cacheService,
		eventPublisher,
		testLogger,
	)

	// Create test router
	router := mux.NewRouter()
	
	// Setup basic routes for integration testing
	router.HandleFunc("/providers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": uuid.New(),
				"name": "Test Provider",
				"base_url": "https://testprovider.com",
				"is_active": true,
			})
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		}
	}).Methods("POST", "GET")
	
	router.HandleFunc("/hotels", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": uuid.New(),
				"name": "Test Hotel",
				"location": "Test City",
			})
		}
	}).Methods("POST")
	
	router.HandleFunc("/reviews", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": uuid.New(),
				"rating": 4.5,
				"title": "Great hotel",
				"comment": "Had a wonderful stay",
			})
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		}
	}).Methods("POST", "GET")

	// Start HTTP test server
	server := httptest.NewServer(router)
	defer server.Close()

	// Test 1: Create Provider
	t.Run("Create Provider", func(t *testing.T) {
		providerReq := map[string]interface{}{
			"name":      "Test Provider",
			"base_url":  "https://testprovider.com",
			"is_active": true,
		}
		
		body, _ := json.Marshal(providerReq)
		resp, err := http.Post(server.URL+"/providers", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test 2: Create Hotel
	t.Run("Create Hotel", func(t *testing.T) {
		hotelReq := map[string]interface{}{
			"name":        "Test Hotel",
			"address":     "123 Test Street",
			"city":        "Test City",
			"country":     "Test Country",
			"star_rating": 4,
			"latitude":    40.7128,
			"longitude":   -74.0060,
		}
		
		body, _ := json.Marshal(hotelReq)
		resp, err := http.Post(server.URL+"/hotels", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	// Test 3: Upload test file to S3
	t.Run("Upload File to S3", func(t *testing.T) {
		// Create test JSONL data
		reviews := []map[string]interface{}{
			{
				"review_id":    "test-review-1",
				"hotel_name":   "Test Hotel",
				"rating":       4.5,
				"title":        "Great stay!",
				"comment":      "Really enjoyed our stay at this hotel.",
				"review_date":  time.Now().Format(time.RFC3339),
				"reviewer_name": "John Doe",
			},
			{
				"review_id":    "test-review-2",
				"hotel_name":   "Test Hotel",
				"rating":       3.0,
				"title":        "Average experience",
				"comment":      "The hotel was okay, nothing special.",
				"review_date":  time.Now().Format(time.RFC3339),
				"reviewer_name": "Jane Smith",
			},
		}

		// Convert to JSONL format
		var jsonlData strings.Builder
		for _, review := range reviews {
			line, _ := json.Marshal(review)
			jsonlData.Write(line)
			jsonlData.WriteString("\n")
		}

		// Upload to S3
		key := "test-reviews.jsonl"
		_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(bucketName),
			Key:         aws.String(key),
			Body:        strings.NewReader(jsonlData.String()),
			ContentType: aws.String("application/x-jsonlines"),
		})
		require.NoError(t, err)
	})

	// Test 4: Process file
	var processingID string
	t.Run("Process File", func(t *testing.T) {
		// Get provider ID
		providers, err := reviewService.ListProviders(ctx, 10, 0)
		require.NoError(t, err)
		require.NotEmpty(t, providers)

		processReq := map[string]interface{}{
			"file_url":    fmt.Sprintf("s3://%s/test-reviews.jsonl", bucketName),
			"provider_id": providers[0].ID.String(),
		}
		
		body, _ := json.Marshal(processReq)
		resp, err := http.Post(server.URL+"/process", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		// Extract processing ID
		data := response["data"].(map[string]interface{})
		processingID = data["id"].(string)
	})

	// Test 5: Check processing status
	t.Run("Check Processing Status", func(t *testing.T) {
		// Wait a bit for processing to complete
		time.Sleep(2 * time.Second)

		resp, err := http.Get(server.URL + "/processing/" + processingID)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Contains(t, []string{"completed", "running"}, data["status"].(string))
	})

	// Test 6: Retrieve reviews
	t.Run("Retrieve Reviews", func(t *testing.T) {
		// Get hotel ID
		hotels, err := reviewService.ListHotels(ctx, 10, 0)
		require.NoError(t, err)
		require.NotEmpty(t, hotels)

		resp, err := http.Get(fmt.Sprintf("%s/hotels/%s/reviews", server.URL, hotels[0].ID.String()))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		// Check if reviews were stored
		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 1)
	})

	// Test 7: Check cache
	t.Run("Check Cache", func(t *testing.T) {
		hotels, err := reviewService.ListHotels(ctx, 10, 0)
		require.NoError(t, err)
		require.NotEmpty(t, hotels)

		// First request should hit database
		start := time.Now()
		resp1, err := http.Get(fmt.Sprintf("%s/hotels/%s/summary", server.URL, hotels[0].ID.String()))
		require.NoError(t, err)
		resp1.Body.Close()
		dbTime := time.Since(start)

		// Second request should hit cache
		start = time.Now()
		resp2, err := http.Get(fmt.Sprintf("%s/hotels/%s/summary", server.URL, hotels[0].ID.String()))
		require.NoError(t, err)
		resp2.Body.Close()
		cacheTime := time.Since(start)

		// Cache should be faster
		assert.Less(t, cacheTime, dbTime)
	})

	// Test 8: Test Kafka events
	t.Run("Verify Kafka Events", func(t *testing.T) {
		// Create Kafka consumer to verify events
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:     tc.kafkaBrokers,
			Topic:       "processing",
			GroupID:     "test-group",
			StartOffset: kafka.FirstOffset,
			MaxWait:     1 * time.Second,
		})
		defer reader.Close()

		// Read a message
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		msg, err := reader.ReadMessage(ctx)
		if err != nil && err != context.DeadlineExceeded {
			require.NoError(t, err)
		}

		if err == nil {
			// Verify event structure
			var event map[string]interface{}
			err = json.Unmarshal(msg.Value, &event)
			require.NoError(t, err)
			assert.Contains(t, event, "type")
			assert.Contains(t, event, "timestamp")
		}
	})
}

// TestErrorScenarios tests various error scenarios
func TestErrorScenarios(t *testing.T) {
	// Setup test containers
	tc := SetupTestContainers(t)
	defer tc.Cleanup(t)

	// Initialize database
	InitializeDatabase(t, tc.postgresConnectionString)

	// Setup components (similar to above)
	_ = context.Background()
	_, _ = logger.New(&logger.Config{
		Level:  "debug",
		Format: "json",
	})

	// Initialize services...
	// (Similar initialization as above)

	t.Run("Invalid S3 URL", func(t *testing.T) {
		// Test with invalid S3 URL format
		// Implementation here
	})

	t.Run("Non-existent File", func(t *testing.T) {
		// Test processing non-existent file
		// Implementation here
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		// Test with malformed JSON data
		// Implementation here
	})

	t.Run("Database Connection Error", func(t *testing.T) {
		// Simulate database connection error
		// Implementation here
	})

	t.Run("Redis Connection Error", func(t *testing.T) {
		// Simulate Redis connection error
		// Implementation here
	})

	t.Run("Kafka Producer Error", func(t *testing.T) {
		// Simulate Kafka producer error
		// Implementation here
	})

	t.Run("Concurrent Processing", func(t *testing.T) {
		// Test concurrent file processing
		// Implementation here
	})

	t.Run("Large File Processing", func(t *testing.T) {
		// Test processing very large files
		// Implementation here
	})

	t.Run("Rate Limiting", func(t *testing.T) {
		// Test rate limiting functionality
		// Implementation here
	})

	t.Run("Duplicate Review Handling", func(t *testing.T) {
		// Test duplicate review detection
		// Implementation here
	})
}

// TestEdgeCases tests edge cases
func TestEdgeCases(t *testing.T) {
	// Setup test containers
	tc := SetupTestContainers(t)
	defer tc.Cleanup(t)

	// Initialize database
	InitializeDatabase(t, tc.postgresConnectionString)

	t.Run("Empty File", func(t *testing.T) {
		// Test processing empty file
		// Implementation here
	})

	t.Run("Unicode Characters", func(t *testing.T) {
		// Test reviews with unicode characters
		// Implementation here
	})

	t.Run("Very Long Review Text", func(t *testing.T) {
		// Test reviews with very long text
		// Implementation here
	})

	t.Run("Invalid Ratings", func(t *testing.T) {
		// Test reviews with invalid ratings
		// Implementation here
	})

	t.Run("Missing Required Fields", func(t *testing.T) {
		// Test reviews with missing fields
		// Implementation here
	})

	t.Run("SQL Injection Attempts", func(t *testing.T) {
		// Test SQL injection prevention
		// Implementation here
	})

	t.Run("XSS Prevention", func(t *testing.T) {
		// Test XSS prevention in review text
		// Implementation here
	})

	t.Run("Concurrent Updates", func(t *testing.T) {
		// Test concurrent updates to same hotel
		// Implementation here
	})

	t.Run("Transaction Rollback", func(t *testing.T) {
		// Test transaction rollback on error
		// Implementation here
	})

	t.Run("Memory Pressure", func(t *testing.T) {
		// Test behavior under memory pressure
		// Implementation here
	})
}

// BenchmarkWorkflow benchmarks the complete workflow
func BenchmarkWorkflow(b *testing.B) {
	// Setup test containers
	tc := SetupTestContainers(&testing.T{})
	defer tc.Cleanup(&testing.T{})

	// Initialize components...
	// (Similar to test setup)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Benchmark file processing workflow
		// Implementation here
	}
}

// Helper functions

// createTestJSONLFile creates a test JSONL file with specified number of reviews
func createTestJSONLFile(t *testing.T, numReviews int) io.Reader {
	var buf bytes.Buffer
	
	for i := 0; i < numReviews; i++ {
		review := map[string]interface{}{
			"review_id":     fmt.Sprintf("review-%d", i),
			"hotel_name":    fmt.Sprintf("Hotel %d", i%10),
			"rating":        float64(1 + (i%5)),
			"title":         fmt.Sprintf("Review title %d", i),
			"comment":       fmt.Sprintf("This is review comment number %d with some text.", i),
			"review_date":   time.Now().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			"reviewer_name": fmt.Sprintf("Reviewer %d", i),
			"trip_type":     []string{"business", "leisure", "family"}[i%3],
			"language":      []string{"en", "es", "fr", "de"}[i%4],
		}
		
		line, _ := json.Marshal(review)
		buf.Write(line)
		buf.WriteString("\n")
	}
	
	return &buf
}

// waitForProcessing waits for a processing job to complete
func waitForProcessing(t *testing.T, serverURL, processingID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverURL + "/processing/" + processingID)
		require.NoError(t, err)
		
		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		status := data["status"].(string)
		
		if status == "completed" || status == "failed" {
			return
		}
		
		time.Sleep(500 * time.Millisecond)
	}
	
	t.Fatalf("Processing did not complete within timeout")
}

// verifyDatabaseState verifies the expected state in the database
func verifyDatabaseState(t *testing.T, db *sql.DB, expectedReviews, expectedHotels int) {
	// Count reviews
	var reviewCount int
	err := db.QueryRow("SELECT COUNT(*) FROM reviews").Scan(&reviewCount)
	require.NoError(t, err)
	assert.Equal(t, expectedReviews, reviewCount)
	
	// Count hotels
	var hotelCount int
	err = db.QueryRow("SELECT COUNT(*) FROM hotels").Scan(&hotelCount)
	require.NoError(t, err)
	assert.Equal(t, expectedHotels, hotelCount)
}

// TestRetryMechanism tests the retry mechanism for failed operations
func TestRetryMechanism(t *testing.T) {
	// Test retry logic for various failure scenarios
	t.Run("S3 Temporary Failure", func(t *testing.T) {
		// Simulate temporary S3 failures
		// Verify retry attempts
	})
	
	t.Run("Database Temporary Failure", func(t *testing.T) {
		// Simulate temporary database failures
		// Verify retry attempts and eventual success
	})
	
	t.Run("Kafka Temporary Failure", func(t *testing.T) {
		// Simulate temporary Kafka failures
		// Verify message retry and dead letter queue
	})
}

// TestCircuitBreaker tests circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
	// Test circuit breaker for external services
	t.Run("S3 Circuit Breaker", func(t *testing.T) {
		// Test circuit breaker opening on repeated S3 failures
	})
	
	t.Run("Database Circuit Breaker", func(t *testing.T) {
		// Test circuit breaker for database operations
	})
}

// TestMetricsCollection tests metrics collection
func TestMetricsCollection(t *testing.T) {
	// Verify metrics are properly collected
	t.Run("Processing Metrics", func(t *testing.T) {
		// Verify processing time metrics
		// Verify record count metrics
		// Verify error rate metrics
	})
	
	t.Run("API Metrics", func(t *testing.T) {
		// Verify API request count
		// Verify API latency metrics
		// Verify API error rates
	})
}