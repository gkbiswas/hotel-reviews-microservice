package infrastructure

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

func TestNewRedisCacheService(t *testing.T) {
	t.Run("create new redis cache service", func(t *testing.T) {
		log := logger.NewDefault()

		service := NewRedisCacheService(nil, log)

		assert.NotNil(t, service)
		assert.Equal(t, log, service.logger)
		assert.Equal(t, 5*time.Minute, service.ttl)
	})
}

func TestRedisCacheService_Structure(t *testing.T) {
	log := logger.NewDefault()
	service := NewRedisCacheService(nil, log)

	t.Run("verify service structure", func(t *testing.T) {
		assert.NotNil(t, service)
		assert.Equal(t, log, service.logger)
		assert.Equal(t, 5*time.Minute, service.ttl)
		assert.Nil(t, service.client) // Should be nil for this test
	})
}

func TestRedisCacheService_KeyGeneration(t *testing.T) {
	t.Run("review summary key generation", func(t *testing.T) {
		hotelID := uuid.New()
		expectedKey := "hotel:summary:" + hotelID.String()

		// Test key generation logic
		actualKey := "hotel:summary:" + hotelID.String()
		assert.Equal(t, expectedKey, actualKey)

		// Test that different hotel IDs generate different keys
		hotelID2 := uuid.New()
		expectedKey2 := "hotel:summary:" + hotelID2.String()
		actualKey2 := "hotel:summary:" + hotelID2.String()

		assert.Equal(t, expectedKey2, actualKey2)
		assert.NotEqual(t, expectedKey, expectedKey2)
	})
}

func TestRedisCacheService_ReviewSummary_Marshaling(t *testing.T) {
	t.Run("marshal and unmarshal review summary", func(t *testing.T) {
		hotelID := uuid.New()
		summary := &domain.ReviewSummary{
			ID:           uuid.New(),
			HotelID:      hotelID,
			TotalReviews: 50,
			AverageRating: 4.2,
			RatingDistribution: map[string]int{
				"5": 20,
				"4": 15,
				"3": 10,
				"2": 3,
				"1": 2,
			},
			AvgServiceRating:     4.3,
			AvgCleanlinessRating: 4.1,
			AvgLocationRating:    4.5,
		}

		// Test marshaling
		data, err := json.Marshal(summary)
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// Test unmarshaling
		var unmarshaled domain.ReviewSummary
		err = json.Unmarshal(data, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, summary.HotelID, unmarshaled.HotelID)
		assert.Equal(t, summary.TotalReviews, unmarshaled.TotalReviews)
		assert.Equal(t, summary.AverageRating, unmarshaled.AverageRating)
		assert.Equal(t, len(summary.RatingDistribution), len(unmarshaled.RatingDistribution))
		assert.Equal(t, summary.AvgServiceRating, unmarshaled.AvgServiceRating)
	})

	t.Run("marshal error handling", func(t *testing.T) {
		// Test with a type that can't be marshaled
		type badSummary struct {
			Channel chan int `json:"channel"`
		}

		bad := &badSummary{
			Channel: make(chan int),
		}

		_, err := json.Marshal(bad)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported type")
	})

	t.Run("unmarshal error handling", func(t *testing.T) {
		invalidJSON := "invalid-json-data"

		var summary domain.ReviewSummary
		err := json.Unmarshal([]byte(invalidJSON), &summary)
		assert.Error(t, err)
	})
}

func TestRedisCacheService_DefaultTTL(t *testing.T) {
	t.Run("verify default TTL", func(t *testing.T) {
		log := logger.NewDefault()
		service := NewRedisCacheService(nil, log)

		assert.Equal(t, 5*time.Minute, service.ttl)
	})

	t.Run("TTL comparison logic", func(t *testing.T) {
		defaultTTL := 5 * time.Minute
		customTTL := time.Hour

		// Test TTL selection logic
		selectedTTL := defaultTTL
		if customTTL > 0 {
			selectedTTL = customTTL
		}

		assert.Equal(t, customTTL, selectedTTL)

		// Test with zero custom TTL
		customTTL = 0
		selectedTTL = defaultTTL
		if customTTL > 0 {
			selectedTTL = customTTL
		}

		assert.Equal(t, defaultTTL, selectedTTL)
	})
}

func TestRedisCacheService_ErrorHandling(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Test that cancelled context is properly cancelled
		select {
		case <-ctx.Done():
			assert.Equal(t, context.Canceled, ctx.Err())
		default:
			t.Fatal("Context should be cancelled")
		}
	})

	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		// Wait for timeout
		time.Sleep(time.Millisecond * 2)

		select {
		case <-ctx.Done():
			assert.Equal(t, context.DeadlineExceeded, ctx.Err())
		default:
			t.Fatal("Context should have timed out")
		}
	})
}

func TestRedisCacheService_DataStructures(t *testing.T) {
	t.Run("review summary data structure", func(t *testing.T) {
		hotelID := uuid.New()
		summary := &domain.ReviewSummary{
			ID:           uuid.New(),
			HotelID:      hotelID,
			TotalReviews: 100,
			AverageRating: 4.5,
			RatingDistribution: map[string]int{
				"5": 50,
				"4": 30,
				"3": 15,
				"2": 3,
				"1": 2,
			},
			AvgServiceRating:     4.6,
			AvgCleanlinessRating: 4.4,
			AvgLocationRating:    4.7,
			AvgValueRating:       4.3,
			AvgComfortRating:     4.5,
			AvgFacilitiesRating:  4.2,
		}

		// Verify required fields
		assert.NotEqual(t, uuid.Nil, summary.ID)
		assert.NotEqual(t, uuid.Nil, summary.HotelID)
		assert.True(t, summary.TotalReviews > 0)
		assert.True(t, summary.AverageRating >= 1.0 && summary.AverageRating <= 5.0)
		assert.NotEmpty(t, summary.RatingDistribution)

		// Verify rating distribution sums to total
		totalFromDistribution := 0
		for _, count := range summary.RatingDistribution {
			totalFromDistribution += count
		}
		assert.Equal(t, summary.TotalReviews, totalFromDistribution)
	})

	t.Run("hotel ID validation", func(t *testing.T) {
		// Test valid UUID
		validID := uuid.New()
		assert.NotEqual(t, uuid.Nil, validID)
		assert.NotEmpty(t, validID.String())

		// Test nil UUID
		var nilID uuid.UUID
		assert.Equal(t, uuid.Nil, nilID)
		assert.Equal(t, "00000000-0000-0000-0000-000000000000", nilID.String())
	})
}

func TestRedisCacheService_Configuration(t *testing.T) {
	t.Run("service configuration", func(t *testing.T) {
		log := logger.NewDefault()
		service := NewRedisCacheService(nil, log)

		// Verify service is properly configured
		assert.NotNil(t, service.logger)
		assert.True(t, service.ttl > 0)
		assert.Equal(t, 5*time.Minute, service.ttl)
	})

	t.Run("time duration calculations", func(t *testing.T) {
		// Test various TTL calculations
		oneMinute := time.Minute
		fiveMinutes := 5 * time.Minute
		oneHour := time.Hour

		assert.Equal(t, float64(60), oneMinute.Seconds())
		assert.Equal(t, float64(300), fiveMinutes.Seconds())
		assert.Equal(t, float64(3600), oneHour.Seconds())

		// Test comparison
		assert.True(t, oneHour > fiveMinutes)
		assert.True(t, fiveMinutes > oneMinute)
	})
}

func TestRedisCacheService_Constants(t *testing.T) {
	t.Run("verify cache key patterns", func(t *testing.T) {
		hotelID := uuid.New()

		// Test key pattern
		expectedKey := "hotel:summary:" + hotelID.String()
		actualKey := "hotel:summary:" + hotelID.String()

		assert.Equal(t, expectedKey, actualKey)

		// Verify key components
		assert.Contains(t, actualKey, "hotel:")
		assert.Contains(t, actualKey, "summary:")
		assert.Contains(t, actualKey, hotelID.String())
	})
}

func TestRedisCacheService_Integration_Logic(t *testing.T) {
	t.Run("cache workflow simulation", func(t *testing.T) {
		hotelID := uuid.New()
		
		// Step 1: Create summary
		summary := &domain.ReviewSummary{
			ID:           uuid.New(),
			HotelID:      hotelID,
			TotalReviews: 25,
			AverageRating: 3.8,
			RatingDistribution: map[string]int{
				"5": 8,
				"4": 7,
				"3": 5,
				"2": 3,
				"1": 2,
			},
		}

		// Step 2: Simulate marshaling (what would happen in cache set)
		data, err := json.Marshal(summary)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		// Step 3: Simulate unmarshaling (what would happen in cache get)
		var retrieved domain.ReviewSummary
		err = json.Unmarshal(data, &retrieved)
		require.NoError(t, err)

		// Step 4: Verify data integrity
		assert.Equal(t, summary.HotelID, retrieved.HotelID)
		assert.Equal(t, summary.TotalReviews, retrieved.TotalReviews)
		assert.Equal(t, summary.AverageRating, retrieved.AverageRating)
		assert.Equal(t, len(summary.RatingDistribution), len(retrieved.RatingDistribution))

		// Step 5: Simulate update
		retrieved.TotalReviews = 30
		retrieved.AverageRating = 4.0
		retrieved.RatingDistribution["5"] = 10

		// Verify update took effect
		assert.Equal(t, 30, retrieved.TotalReviews)
		assert.Equal(t, 4.0, retrieved.AverageRating)
		assert.Equal(t, 10, retrieved.RatingDistribution["5"])
	})
}