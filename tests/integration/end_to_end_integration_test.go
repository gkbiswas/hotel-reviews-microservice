package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end integration tests that simulate complete business workflows

// TestCompleteBusinessWorkflow tests the complete business workflow from file upload to analytics
func TestCompleteBusinessWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration tests in short mode")
	}

	// Initialize comprehensive test suite
	suite := SetupSuite(t)
	defer suite.Cleanup(t)

	// Initialize file processing suite
	fileSuite := &FileProcessingIntegrationSuite{
		IntegrationTestSuite: suite,
	}
	fileSuite.setupS3Client(t)
	fileSuite.setupFileProcessing(t)

	ctx := context.Background()

	t.Run("Complete Business Workflow", func(t *testing.T) {
		// Step 1: Create multiple providers (simulating different review sources)
		providers := suite.createMultipleProviders(t, ctx, []string{"Booking.com", "Expedia", "Hotels.com"})

		// Step 2: Create multiple hotels with different characteristics
		hotels := suite.createMultipleHotels(t, ctx)

		// Step 3: Upload and process multiple review files
		suite.processMultipleReviewFiles(t, ctx, fileSuite, providers, hotels)

		// Step 4: Test comprehensive search functionality
		suite.testComprehensiveSearch(t, hotels)

		// Step 5: Generate and verify analytics
		suite.testBusinessAnalytics(t, hotels, providers)

		// Step 6: Test caching and performance
		suite.testCachingAndPerformance(t, hotels)

		// Step 7: Test data integrity and consistency
		suite.testDataIntegrityAndConsistency(t, hotels, providers)

		// Step 8: Test security and validation
		suite.testSecurityAndValidation(t)
	})
}

// createMultipleProviders creates test providers
func (suite *IntegrationTestSuite) createMultipleProviders(t *testing.T, ctx context.Context, providerNames []string) []*uuid.UUID {
	var providerIDs []*uuid.UUID

	for i, name := range providerNames {
		providerData := map[string]interface{}{
			"name":      name,
			"base_url":  fmt.Sprintf("https://%s.example.com", strings.ToLower(strings.Replace(name, ".", "", -1))),
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

		data := response["data"].(map[string]interface{})
		providerID, err := uuid.Parse(data["id"].(string))
		require.NoError(t, err)
		providerIDs = append(providerIDs, &providerID)

		t.Logf("Created provider %d: %s (ID: %s)", i+1, name, providerID.String())
	}

	return providerIDs
}

// createMultipleHotels creates test hotels with different characteristics
func (suite *IntegrationTestSuite) createMultipleHotels(t *testing.T, ctx context.Context) []*uuid.UUID {
	hotels := []map[string]interface{}{
		{
			"name":        "Luxury Grand Hotel",
			"address":     "123 Luxury Avenue",
			"city":        "New York",
			"country":     "USA",
			"star_rating": 5,
			"description": "A luxury hotel in the heart of Manhattan",
			"latitude":    40.7589,
			"longitude":   -73.9851,
			"amenities":   []string{"spa", "pool", "gym", "restaurant", "bar", "wifi"},
		},
		{
			"name":        "Business Express Hotel",
			"address":     "456 Business District",
			"city":        "Chicago",
			"country":     "USA",
			"star_rating": 4,
			"description": "Perfect for business travelers",
			"latitude":    41.8781,
			"longitude":   -87.6298,
			"amenities":   []string{"wifi", "gym", "business_center", "restaurant"},
		},
		{
			"name":        "Budget Comfort Inn",
			"address":     "789 Economy Street",
			"city":        "Austin",
			"country":     "USA",
			"star_rating": 3,
			"description": "Comfortable and affordable accommodation",
			"latitude":    30.2672,
			"longitude":   -97.7431,
			"amenities":   []string{"wifi", "parking"},
		},
		{
			"name":        "Boutique Design Hotel",
			"address":     "321 Art District",
			"city":        "San Francisco",
			"country":     "USA",
			"star_rating": 4,
			"description": "Unique boutique hotel with artistic design",
			"latitude":    37.7749,
			"longitude":   -122.4194,
			"amenities":   []string{"wifi", "restaurant", "bar", "art_gallery"},
		},
		{
			"name":        "Family Resort Hotel",
			"address":     "555 Family Lane",
			"city":        "Orlando",
			"country":     "USA",
			"star_rating": 4,
			"description": "Perfect for family vacations",
			"latitude":    28.5383,
			"longitude":   -81.3792,
			"amenities":   []string{"pool", "kids_club", "restaurant", "wifi", "playground"},
		},
	}

	var hotelIDs []*uuid.UUID

	for i, hotelData := range hotels {
		body, _ := json.Marshal(hotelData)
		resp, err := http.Post(suite.server.URL+"/api/v1/hotels", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		hotelID, err := uuid.Parse(data["id"].(string))
		require.NoError(t, err)
		hotelIDs = append(hotelIDs, &hotelID)

		t.Logf("Created hotel %d: %s (ID: %s)", i+1, hotelData["name"], hotelID.String())
	}

	return hotelIDs
}

// processMultipleReviewFiles uploads and processes multiple review files
func (suite *IntegrationTestSuite) processMultipleReviewFiles(t *testing.T, ctx context.Context, fileSuite *FileProcessingIntegrationSuite, providers []*uuid.UUID, hotels []*uuid.UUID) {
	// Create review data for each provider-hotel combination
	reviewFiles := []struct {
		filename   string
		providerID *uuid.UUID
		reviews    []map[string]interface{}
	}{
		{
			filename:   "booking_luxury_reviews.jsonl",
			providerID: providers[0], // Booking.com
			reviews: suite.generateReviews(hotels[0], "Luxury Grand Hotel", []float64{4.8, 4.9, 5.0, 4.7, 4.8}, []string{
				"Exceptional service and luxurious amenities",
				"Outstanding hotel with world-class service",
				"Perfect location and amazing staff",
				"Beautiful rooms and excellent facilities",
				"Unforgettable experience at this luxury hotel",
			}),
		},
		{
			filename:   "expedia_business_reviews.jsonl",
			providerID: providers[1], // Expedia
			reviews: suite.generateReviews(hotels[1], "Business Express Hotel", []float64{4.2, 4.0, 4.3, 4.1, 4.4}, []string{
				"Great for business trips, good WiFi",
				"Efficient check-in and convenient location",
				"Perfect for work travel",
				"Good facilities for business meetings",
				"Reliable hotel for corporate stays",
			}),
		},
		{
			filename:   "hotelscom_budget_reviews.jsonl",
			providerID: providers[2], // Hotels.com
			reviews: suite.generateReviews(hotels[2], "Budget Comfort Inn", []float64{3.5, 3.8, 3.2, 3.6, 3.4}, []string{
				"Good value for money",
				"Basic but clean accommodation",
				"Affordable option with decent service",
				"Simple but comfortable stay",
				"Budget-friendly with necessary amenities",
			}),
		},
		{
			filename:   "booking_boutique_reviews.jsonl",
			providerID: providers[0], // Booking.com
			reviews: suite.generateReviews(hotels[3], "Boutique Design Hotel", []float64{4.5, 4.6, 4.7, 4.4, 4.5}, []string{
				"Unique design and artistic ambiance",
				"Stylish hotel with great character",
				"Beautiful art and creative interiors",
				"Trendy boutique with excellent service",
				"Amazing design and attention to detail",
			}),
		},
		{
			filename:   "expedia_family_reviews.jsonl",
			providerID: providers[1], // Expedia
			reviews: suite.generateReviews(hotels[4], "Family Resort Hotel", []float64{4.3, 4.1, 4.5, 4.2, 4.4}, []string{
				"Perfect for kids, great pool area",
				"Family-friendly with lots of activities",
				"Excellent kids club and facilities",
				"Great family vacation destination",
				"Children loved the playground and pool",
			}),
		},
	}

	var processingIDs []string

	// Upload and process each file
	for _, file := range reviewFiles {
		t.Logf("Processing file: %s with %d reviews", file.filename, len(file.reviews))

		// Create JSONL content
		var jsonlData strings.Builder
		for _, review := range file.reviews {
			line, _ := json.Marshal(review)
			jsonlData.Write(line)
			jsonlData.WriteString("\n")
		}

		// Upload to S3
		_, err := fileSuite.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String("test-bucket"),
			Key:         aws.String(file.filename),
			Body:        strings.NewReader(jsonlData.String()),
			ContentType: aws.String("application/x-jsonlines"),
		})
		require.NoError(t, err)

		// Process file
		processRequest := map[string]interface{}{
			"file_url":    fmt.Sprintf("s3://test-bucket/%s", file.filename),
			"provider_id": file.providerID.String(),
		}

		body, _ := json.Marshal(processRequest)
		resp, err := http.Post(suite.server.URL+"/api/v1/files/upload", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		processingIDs = append(processingIDs, data["id"].(string))
	}

	// Wait for all files to be processed
	for i, processingID := range processingIDs {
		t.Logf("Waiting for processing completion: %s", reviewFiles[i].filename)
		fileSuite.waitForProcessingCompletion(t, processingID, 60*time.Second)

		// Verify processing status
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/files/processing/%s", suite.server.URL, processingID))
		require.NoError(t, err)
		defer resp.Body.Close()

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		statusData := response["data"].(map[string]interface{})
		assert.Equal(t, "completed", statusData["status"].(string))
		assert.Equal(t, float64(len(reviewFiles[i].reviews)), statusData["processed_records"].(float64))

		t.Logf("File %s processed successfully: %v records", reviewFiles[i].filename, statusData["processed_records"])
	}

	t.Logf("All %d review files processed successfully", len(reviewFiles))
}

// generateReviews creates review data for testing
func (suite *IntegrationTestSuite) generateReviews(hotelID *uuid.UUID, hotelName string, ratings []float64, comments []string) []map[string]interface{} {
	var reviews []map[string]interface{}

	reviewerNames := []string{
		"John Smith", "Jane Doe", "Michael Johnson", "Emily Davis", "David Wilson",
		"Sarah Brown", "Robert Jones", "Lisa Garcia", "William Miller", "Jennifer Taylor",
	}

	tripTypes := []string{"business", "leisure", "family", "couple", "solo"}
	languages := []string{"en", "es", "fr", "de", "it"}

	for i := 0; i < len(ratings); i++ {
		review := map[string]interface{}{
			"review_id":        fmt.Sprintf("%s-review-%d", strings.Replace(hotelName, " ", "-", -1), i+1),
			"hotel_id":         hotelID.String(),
			"hotel_name":       hotelName,
			"rating":           ratings[i],
			"title":            fmt.Sprintf("Review %d for %s", i+1, hotelName),
			"comment":          comments[i],
			"review_date":      time.Now().Add(-time.Duration(i*24+i*3) * time.Hour).Format(time.RFC3339),
			"stay_date":        time.Now().Add(-time.Duration(i*24+i*5) * time.Hour).Format(time.RFC3339),
			"reviewer_name":    reviewerNames[i%len(reviewerNames)],
			"trip_type":        tripTypes[i%len(tripTypes)],
			"language":         languages[i%len(languages)],
			"service_rating":   ratings[i] + (float64(i%3)-1)*0.1,
			"cleanliness_rating": ratings[i] + (float64(i%2))*0.1,
			"location_rating":  ratings[i] + (float64(i%4)-1.5)*0.1,
			"value_rating":     ratings[i] - 0.2 + (float64(i%3))*0.1,
		}

		// Ensure ratings are within valid bounds
		for _, field := range []string{"service_rating", "cleanliness_rating", "location_rating", "value_rating"} {
			if review[field].(float64) > 5.0 {
				review[field] = 5.0
			}
			if review[field].(float64) < 1.0 {
				review[field] = 1.0
			}
		}

		reviews = append(reviews, review)
	}

	return reviews
}

// testComprehensiveSearch tests all search functionality
func (suite *IntegrationTestSuite) testComprehensiveSearch(t *testing.T, hotels []*uuid.UUID) {
	t.Run("Search Reviews", func(t *testing.T) {
		// Test different search queries
		searchQueries := []struct {
			query    string
			expected int // minimum expected results
		}{
			{"luxury", 1},
			{"business", 1},
			{"family", 1},
			{"excellent", 1},
			{"service", 1},
		}

		for _, sq := range searchQueries {
			url := fmt.Sprintf("%s/api/v1/search/reviews?q=%s&limit=20", suite.server.URL, sq.query)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))

			data := response["data"].([]interface{})
			assert.GreaterOrEqual(t, len(data), sq.expected, "Search query '%s' should return at least %d results", sq.query, sq.expected)
		}
	})

	t.Run("Search Hotels", func(t *testing.T) {
		// Test hotel search queries
		searchQueries := []struct {
			query    string
			expected int
		}{
			{"Grand", 1},
			{"Business", 1},
			{"Budget", 1},
			{"New York", 1},
			{"Hotel", 5}, // All hotels should match
		}

		for _, sq := range searchQueries {
			url := fmt.Sprintf("%s/api/v1/search/hotels?q=%s&limit=20", suite.server.URL, sq.query)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))

			data := response["data"].([]interface{})
			assert.GreaterOrEqual(t, len(data), sq.expected, "Hotel search query '%s' should return at least %d results", sq.query, sq.expected)
		}
	})

	t.Run("Search with Filters", func(t *testing.T) {
		// Test search with rating filters
		url := fmt.Sprintf("%s/api/v1/search/reviews?q=hotel&min_rating=4.0&limit=10", suite.server.URL)
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].([]interface{})
		// Verify all results have rating >= 4.0
		for _, item := range data {
			review := item.(map[string]interface{})
			rating := review["rating"].(float64)
			assert.GreaterOrEqual(t, rating, 4.0)
		}
	})
}

// testBusinessAnalytics tests comprehensive analytics functionality
func (suite *IntegrationTestSuite) testBusinessAnalytics(t *testing.T, hotels []*uuid.UUID, providers []*uuid.UUID) {
	t.Run("Overall Analytics", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/analytics/overview")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Contains(t, data, "total_reviews")
		assert.Contains(t, data, "total_hotels")
		assert.Contains(t, data, "total_providers")
		assert.Contains(t, data, "average_rating")

		// Verify metrics make sense
		totalReviews := data["total_reviews"].(float64)
		assert.Greater(t, totalReviews, float64(20)) // We created 25 reviews (5 per hotel)

		totalHotels := data["total_hotels"].(float64)
		assert.Equal(t, float64(6), totalHotels) // 5 new + 1 original test hotel

		totalProviders := data["total_providers"].(float64)
		assert.Equal(t, float64(4), totalProviders) // 3 new + 1 original test provider
	})

	t.Run("Top Rated Hotels", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/analytics/hotels/top-rated?limit=5")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 3)

		// Verify hotels are sorted by rating (highest first)
		var prevRating float64 = 6.0 // Start with impossibly high rating
		for _, item := range data {
			hotel := item.(map[string]interface{})
			rating := hotel["average_rating"].(float64)
			assert.LessOrEqual(t, rating, prevRating, "Hotels should be sorted by rating in descending order")
			prevRating = rating
		}
	})

	t.Run("Hotel Specific Analytics", func(t *testing.T) {
		for i, hotelID := range hotels {
			url := fmt.Sprintf("%s/api/v1/analytics/hotels/%s/stats", suite.server.URL, hotelID.String())
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))

			data := response["data"].(map[string]interface{})
			assert.Contains(t, data, "total_reviews")
			assert.Contains(t, data, "average_rating")
			assert.Contains(t, data, "rating_distribution")

			// Each hotel should have 5 reviews (except the original test hotel which might have more)
			totalReviews := data["total_reviews"].(float64)
			if i < 5 { // New hotels
				assert.Equal(t, float64(5), totalReviews)
			}

			t.Logf("Hotel %d stats: %v reviews, avg rating: %v", i+1, totalReviews, data["average_rating"])
		}
	})

	t.Run("Provider Analytics", func(t *testing.T) {
		for i, providerID := range providers {
			url := fmt.Sprintf("%s/api/v1/analytics/providers/%s/stats", suite.server.URL, providerID.String())
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))

			data := response["data"].(map[string]interface{})
			assert.Contains(t, data, "total_reviews")
			assert.Contains(t, data, "average_rating")

			t.Logf("Provider %d stats: %v reviews, avg rating: %v", i+1, data["total_reviews"], data["average_rating"])
		}
	})

	t.Run("Review Trends", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/analytics/trends/reviews?period=7d")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Contains(t, data, "trend_data")
		assert.Contains(t, data, "period")
		assert.Contains(t, data, "total_reviews")
	})

	t.Run("Recent Reviews", func(t *testing.T) {
		resp, err := http.Get(suite.server.URL + "/api/v1/analytics/reviews/recent?limit=10")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].([]interface{})
		assert.LessOrEqual(t, len(data), 10)
		assert.GreaterOrEqual(t, len(data), 5)

		// Verify reviews are sorted by date (most recent first)
		var prevDate time.Time = time.Now().Add(24 * time.Hour) // Future date
		for _, item := range data {
			review := item.(map[string]interface{})
			dateStr := review["review_date"].(string)
			reviewDate, err := time.Parse(time.RFC3339, dateStr)
			require.NoError(t, err)
			assert.True(t, reviewDate.Before(prevDate) || reviewDate.Equal(prevDate), "Reviews should be sorted by date in descending order")
			prevDate = reviewDate
		}
	})
}

// testCachingAndPerformance tests caching behavior and performance under load
func (suite *IntegrationTestSuite) testCachingAndPerformance(t *testing.T, hotels []*uuid.UUID) {
	t.Run("Cache Performance", func(t *testing.T) {
		if len(hotels) == 0 {
			t.Skip("No hotels available for cache testing")
		}

		hotelID := hotels[0]
		url := fmt.Sprintf("%s/api/v1/analytics/hotels/%s/summary", suite.server.URL, hotelID.String())

		// First request (database hit)
		start := time.Now()
		resp1, err := http.Get(url)
		require.NoError(t, err)
		resp1.Body.Close()
		dbTime := time.Since(start)

		// Second request (cache hit)
		start = time.Now()
		resp2, err := http.Get(url)
		require.NoError(t, err)
		resp2.Body.Close()
		cacheTime := time.Since(start)

		// Third request (cache hit)
		start = time.Now()
		resp3, err := http.Get(url)
		require.NoError(t, err)
		resp3.Body.Close()
		cacheTime2 := time.Since(start)

		assert.Equal(t, http.StatusOK, resp1.StatusCode)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
		assert.Equal(t, http.StatusOK, resp3.StatusCode)

		// Cache requests should be significantly faster
		assert.Less(t, cacheTime.Nanoseconds(), dbTime.Nanoseconds())
		assert.Less(t, cacheTime2.Nanoseconds(), dbTime.Nanoseconds())

		t.Logf("Database request: %v, Cache request 1: %v, Cache request 2: %v", dbTime, cacheTime, cacheTime2)
	})

	t.Run("Concurrent Request Performance", func(t *testing.T) {
		const numRequests = 20
		const numWorkers = 5

		requests := make(chan string, numRequests)
		results := make(chan struct {
			statusCode int
			duration   time.Duration
		}, numRequests)

		// Fill request channel
		for i := 0; i < numRequests; i++ {
			requests <- suite.server.URL + "/api/v1/analytics/overview"
		}
		close(requests)

		// Start workers
		for i := 0; i < numWorkers; i++ {
			go func() {
				for url := range requests {
					start := time.Now()
					resp, err := http.Get(url)
					duration := time.Since(start)
					
					if err != nil {
						results <- struct {
							statusCode int
							duration   time.Duration
						}{500, duration}
					} else {
						results <- struct {
							statusCode int
							duration   time.Duration
						}{resp.StatusCode, duration}
						resp.Body.Close()
					}
				}
			}()
		}

		// Collect results
		successCount := 0
		var totalDuration time.Duration
		var maxDuration time.Duration

		for i := 0; i < numRequests; i++ {
			result := <-results
			totalDuration += result.duration
			if result.duration > maxDuration {
				maxDuration = result.duration
			}
			if result.statusCode == 200 {
				successCount++
			}
		}

		avgDuration := totalDuration / time.Duration(numRequests)
		successRate := float64(successCount) / float64(numRequests)

		assert.GreaterOrEqual(t, successRate, 0.95) // 95% success rate minimum
		assert.Less(t, avgDuration, 2*time.Second)   // Average response under 2 seconds
		assert.Less(t, maxDuration, 5*time.Second)   // Max response under 5 seconds

		t.Logf("Concurrent performance: %d/%d success, avg: %v, max: %v", successCount, numRequests, avgDuration, maxDuration)
	})
}

// testDataIntegrityAndConsistency tests data integrity and consistency
func (suite *IntegrationTestSuite) testDataIntegrityAndConsistency(t *testing.T, hotels []*uuid.UUID, providers []*uuid.UUID) {
	t.Run("Data Consistency Check", func(t *testing.T) {
		// Get all reviews
		resp, err := http.Get(suite.server.URL + "/api/v1/reviews?limit=100")
		require.NoError(t, err)
		defer resp.Body.Close()

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		reviews := response["data"].([]interface{})

		// Verify all reviews have valid hotel and provider IDs
		hotelIDMap := make(map[string]bool)
		for _, hotelID := range hotels {
			hotelIDMap[hotelID.String()] = true
		}
		hotelIDMap[suite.testHotel.ID.String()] = true // Include original test hotel

		providerIDMap := make(map[string]bool)
		for _, providerID := range providers {
			providerIDMap[providerID.String()] = true
		}
		providerIDMap[suite.testProvider.ID.String()] = true // Include original test provider

		for _, item := range reviews {
			review := item.(map[string]interface{})
			
			hotelID := review["hotel_id"].(string)
			providerID := review["provider_id"].(string)
			
			assert.True(t, hotelIDMap[hotelID], "Review references valid hotel ID: %s", hotelID)
			assert.True(t, providerIDMap[providerID], "Review references valid provider ID: %s", providerID)

			// Verify rating is within valid range
			rating := review["rating"].(float64)
			assert.GreaterOrEqual(t, rating, 1.0)
			assert.LessOrEqual(t, rating, 5.0)

			// Verify required fields are present
			assert.NotEmpty(t, review["title"])
			assert.NotEmpty(t, review["comment"])
			assert.NotEmpty(t, review["review_date"])
		}

		t.Logf("Data consistency check passed for %d reviews", len(reviews))
	})

	t.Run("Rating Aggregation Consistency", func(t *testing.T) {
		// Check that hotel ratings are properly aggregated
		for _, hotelID := range hotels[:3] { // Check first 3 hotels
			// Get individual reviews for hotel
			reviewsURL := fmt.Sprintf("%s/api/v1/reviews?hotel_id=%s&limit=20", suite.server.URL, hotelID.String())
			resp, err := http.Get(reviewsURL)
			require.NoError(t, err)
			defer resp.Body.Close()

			var reviewsResponse map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&reviewsResponse)
			require.NoError(t, err)

			reviews := reviewsResponse["data"].([]interface{})
			
			// Calculate average rating
			var totalRating float64
			for _, item := range reviews {
				review := item.(map[string]interface{})
				totalRating += review["rating"].(float64)
			}
			calculatedAvg := totalRating / float64(len(reviews))

			// Get hotel stats
			statsURL := fmt.Sprintf("%s/api/v1/analytics/hotels/%s/stats", suite.server.URL, hotelID.String())
			resp, err = http.Get(statsURL)
			require.NoError(t, err)
			defer resp.Body.Close()

			var statsResponse map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&statsResponse)
			require.NoError(t, err)

			statsData := statsResponse["data"].(map[string]interface{})
			reportedAvg := statsData["average_rating"].(float64)

			// Verify aggregated rating matches calculated rating (within tolerance)
			assert.InDelta(t, calculatedAvg, reportedAvg, 0.01, "Aggregated rating should match calculated average for hotel %s", hotelID.String())
		}
	})
}

// testSecurityAndValidation tests security measures and input validation
func (suite *IntegrationTestSuite) testSecurityAndValidation(t *testing.T) {
	t.Run("SQL Injection Protection", func(t *testing.T) {
		maliciousQueries := []string{
			"'; DROP TABLE reviews; --",
			"' OR '1'='1",
			"'; UPDATE hotels SET name='hacked'; --",
			"' UNION SELECT * FROM providers; --",
		}

		for _, query := range maliciousQueries {
			url := fmt.Sprintf("%s/api/v1/search/reviews?q=%s", suite.server.URL, query)
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should either reject with 400 or return safe results
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusOK}, resp.StatusCode)

			if resp.StatusCode == http.StatusOK {
				var response map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&response)
				require.NoError(t, err)
				// Should not return any suspicious results
				assert.True(t, response["success"].(bool))
			}
		}
	})

	t.Run("Input Validation", func(t *testing.T) {
		// Test invalid rating values
		invalidReview := map[string]interface{}{
			"provider_id":  suite.testProvider.ID.String(),
			"hotel_id":     suite.testHotel.ID.String(),
			"external_id":  "invalid-rating-test",
			"rating":       6.5, // Invalid rating > 5
			"title":        "Invalid rating test",
			"comment":      "This should fail validation",
			"review_date":  time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(invalidReview)
		resp, err := http.Post(suite.server.URL+"/api/v1/reviews", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Test missing required fields
		incompleteReview := map[string]interface{}{
			"rating": 4.0,
			"title":  "Incomplete review",
			// Missing provider_id, hotel_id, comment, review_date
		}

		body, _ = json.Marshal(incompleteReview)
		resp, err = http.Post(suite.server.URL+"/api/v1/reviews", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Rate Limiting", func(t *testing.T) {
		// Make rapid requests to test rate limiting (using a simple endpoint)
		const rapidRequests = 10
		var successCount, rateLimitedCount int

		for i := 0; i < rapidRequests; i++ {
			resp, err := http.Get(suite.server.URL + "/health")
			require.NoError(t, err)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				successCount++
			} else if resp.StatusCode == http.StatusTooManyRequests {
				rateLimitedCount++
			}
		}

		// Most requests should succeed given our test configuration
		assert.GreaterOrEqual(t, successCount, rapidRequests/2)
		t.Logf("Rate limiting test: %d success, %d rate limited out of %d requests", successCount, rateLimitedCount, rapidRequests)
	})

	t.Run("CORS Protection", func(t *testing.T) {
		// Test preflight request
		req, err := http.NewRequest("OPTIONS", suite.server.URL+"/api/v1/hotels", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Methods"))
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Headers"))

		// Test with disallowed origin
		req, err = http.NewRequest("GET", suite.server.URL+"/api/v1/hotels", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://malicious.com")

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Origin should not be reflected for disallowed origins
		assert.NotEqual(t, "http://malicious.com", resp.Header.Get("Access-Control-Allow-Origin"))
	})
}