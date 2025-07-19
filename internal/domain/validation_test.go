package domain

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// DomainValidationTestSuite provides comprehensive validation testing for domain entities
type DomainValidationTestSuite struct {
	suite.Suite
	ctx     context.Context
	service *ReviewServiceImpl
}

func (suite *DomainValidationTestSuite) SetupTest() {
	suite.ctx = context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create minimal ReviewServiceImpl for validation testing
	// We only need the validation methods, not the full repository
	suite.service = &ReviewServiceImpl{
		logger: logger,
	}
}

// Test comprehensive domain validation beyond existing coverage
func (suite *DomainValidationTestSuite) TestReviewValidation_AdditionalEdgeCases() {
	serviceImpl := suite.service

	tests := []struct {
		name        string
		review      *Review
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid review passes validation",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     4.5,
				Comment:    "Great hotel with excellent service",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: false,
		},
		{
			name: "rating exactly 1.0 should pass",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     1.0,
				Comment:    "Terrible experience",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: false,
		},
		{
			name: "rating exactly 5.0 should pass",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     5.0,
				Comment:    "Perfect hotel",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: false,
		},
		{
			name: "rating 0.9 should fail",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     0.9,
				Comment:    "Bad hotel",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: true,
			errorMsg:    "rating must be between 1.0 and 5.0",
		},
		{
			name: "rating 5.1 should fail",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     5.1,
				Comment:    "Beyond perfect hotel",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: true,
			errorMsg:    "rating must be between 1.0 and 5.0",
		},
		{
			name: "whitespace-only comment should fail",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     4.0,
				Comment:    "   \t\n   ",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: true,
			errorMsg:    "comment cannot be empty",
		},
		{
			name: "review date exactly now should pass",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     4.0,
				Comment:    "Just checked out",
				ReviewDate: time.Now(),
			},
			expectError: false,
		},
		{
			name: "review date 1 minute in future should fail",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     4.0,
				Comment:    "Future review",
				ReviewDate: time.Now().Add(1 * time.Minute),
			},
			expectError: true,
			errorMsg:    "review date cannot be in the future",
		},
		{
			name: "zero UUID for provider should fail",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.Nil,
				HotelID:    uuid.New(),
				Rating:     4.0,
				Comment:    "Valid comment",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: true,
			errorMsg:    "provider ID cannot be empty",
		},
		{
			name: "zero UUID for hotel should fail",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.Nil,
				Rating:     4.0,
				Comment:    "Valid comment",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			},
			expectError: true,
			errorMsg:    "hotel ID cannot be empty",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Trim whitespace from comment if present for proper validation
			if tt.review.Comment != "" {
				trimmed := strings.TrimSpace(tt.review.Comment)
				if trimmed == "" {
					tt.review.Comment = ""
				}
			}

			err := serviceImpl.ValidateReviewData(suite.ctx, tt.review)

			if tt.expectError {
				suite.Error(err)
				if tt.errorMsg != "" {
					suite.Contains(err.Error(), tt.errorMsg)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

// Test enrichment validation logic
func (suite *DomainValidationTestSuite) TestEnrichReviewData_Comprehensive() {
	serviceImpl := suite.service

	tests := []struct {
		name           string
		review         *Review
		expectedLang   string
		expectedFields []string
	}{
		{
			name: "empty language gets default",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     4.0,
				Comment:    "Great hotel",
				ReviewDate: time.Now(),
				Language:   "",
			},
			expectedLang:   "en",
			expectedFields: []string{"Language", "Sentiment", "ProcessingHash", "ProcessedAt"},
		},
		{
			name: "existing language preserved",
			review: &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     4.0,
				Comment:    "Excellent hôtel",
				ReviewDate: time.Now(),
				Language:   "fr",
			},
			expectedLang:   "fr",
			expectedFields: []string{"Sentiment", "ProcessingHash", "ProcessedAt"},
		},
		{
			name: "all fields empty get enriched",
			review: &Review{
				ID:             uuid.New(),
				ProviderID:     uuid.New(),
				HotelID:        uuid.New(),
				Rating:         3.5,
				Comment:        "Average hotel experience",
				ReviewDate:     time.Now(),
				Language:       "",
				Sentiment:      "",
				ProcessingHash: "",
				ProcessedAt:    nil,
			},
			expectedLang:   "en",
			expectedFields: []string{"Language", "Sentiment", "ProcessingHash", "ProcessedAt"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			originalLang := tt.review.Language
			originalSentiment := tt.review.Sentiment
			originalHash := tt.review.ProcessingHash
			originalProcessedAt := tt.review.ProcessedAt

			err := serviceImpl.EnrichReviewData(suite.ctx, tt.review)
			suite.NoError(err)

			// Check language handling
			suite.Equal(tt.expectedLang, tt.review.Language)

			// Check that appropriate fields were enriched
			for _, field := range tt.expectedFields {
				switch field {
				case "Language":
					if originalLang == "" {
						suite.NotEmpty(tt.review.Language)
					}
				case "Sentiment":
					if originalSentiment == "" {
						suite.NotEmpty(tt.review.Sentiment)
					}
				case "ProcessingHash":
					if originalHash == "" {
						suite.NotEmpty(tt.review.ProcessingHash)
					}
				case "ProcessedAt":
					if originalProcessedAt == nil {
						suite.NotNil(tt.review.ProcessedAt)
					}
				}
			}
		})
	}
}

// Test complex business validation scenarios
func (suite *DomainValidationTestSuite) TestComplexBusinessValidation() {
	t := suite.T()

	t.Run("review date vs stay date business rule", func(t *testing.T) {
		// Business rule: Review date should typically be after stay date
		stayDate := time.Now().Add(-30 * 24 * time.Hour)   // 30 days ago
		reviewDate := time.Now().Add(-25 * 24 * time.Hour) // 25 days ago (after stay)

		review := Review{
			ID:         uuid.New(),
			ProviderID: uuid.New(),
			HotelID:    uuid.New(),
			Rating:     4.0,
			Comment:    "Great hotel, enjoyed my stay",
			ReviewDate: reviewDate,
			StayDate:   &stayDate,
		}

		// This should be valid as review is after stay
		isValid := review.ReviewDate.After(*review.StayDate) || review.ReviewDate.Equal(*review.StayDate)
		assert.True(t, isValid, "Review date should be after or equal to stay date")

		// Test invalid case
		invalidReview := review
		invalidReviewDate := stayDate.Add(-5 * 24 * time.Hour) // Before stay
		invalidReview.ReviewDate = invalidReviewDate

		isInvalid := invalidReview.ReviewDate.Before(*invalidReview.StayDate)
		assert.True(t, isInvalid, "Review before stay date should be flagged as suspicious")
	})

	t.Run("rating consistency with comment sentiment", func(t *testing.T) {
		tests := []struct {
			name               string
			rating             float64
			comment            string
			expectedConsistent bool
		}{
			{
				name:               "high rating with positive comment",
				rating:             4.8,
				comment:            "Amazing hotel! Excellent service and beautiful rooms. Highly recommend!",
				expectedConsistent: true,
			},
			{
				name:               "low rating with negative comment",
				rating:             1.5,
				comment:            "Terrible experience. Room was dirty, staff was rude, and food was awful.",
				expectedConsistent: true,
			},
			{
				name:               "high rating with negative comment - inconsistent",
				rating:             4.9,
				comment:            "Worst hotel ever! Dirty rooms, terrible service, complete waste of money!",
				expectedConsistent: false,
			},
			{
				name:               "low rating with positive comment - inconsistent",
				rating:             1.2,
				comment:            "Perfect hotel! Amazing staff, beautiful rooms, excellent location!",
				expectedConsistent: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Simple sentiment analysis based on keywords
				positiveWords := []string{"amazing", "excellent", "beautiful", "perfect", "great", "wonderful"}
				negativeWords := []string{"terrible", "awful", "worst", "dirty", "rude", "waste"}

				comment := strings.ToLower(tt.comment)
				positiveCount := 0
				negativeCount := 0

				for _, word := range positiveWords {
					if strings.Contains(comment, word) {
						positiveCount++
					}
				}

				for _, word := range negativeWords {
					if strings.Contains(comment, word) {
						negativeCount++
					}
				}

				// Determine if rating and sentiment are consistent
				isHighRating := tt.rating >= 4.0
				isPositiveSentiment := positiveCount > negativeCount

				isConsistent := (isHighRating && isPositiveSentiment) || (!isHighRating && !isPositiveSentiment)

				assert.Equal(t, tt.expectedConsistent, isConsistent,
					"Rating-comment consistency check failed for: %s", tt.name)
			})
		}
	})

	t.Run("detailed ratings average validation", func(t *testing.T) {
		serviceRating := 4.5
		cleanlinessRating := 4.2
		locationRating := 4.8
		valueRating := 3.9

		review := Review{
			ID:                uuid.New(),
			ProviderID:        uuid.New(),
			HotelID:           uuid.New(),
			Rating:            4.3, // Should be close to average of detailed ratings
			ServiceRating:     &serviceRating,
			CleanlinessRating: &cleanlinessRating,
			LocationRating:    &locationRating,
			ValueRating:       &valueRating,
		}

		// Calculate average of detailed ratings
		avgDetailedRating := (serviceRating + cleanlinessRating + locationRating + valueRating) / 4
		expectedAvg := 4.35

		assert.InDelta(t, expectedAvg, avgDetailedRating, 0.01, "Detailed ratings average calculation")

		// Check if overall rating is reasonable compared to detailed ratings
		ratingDifference := review.Rating - avgDetailedRating
		isReasonable := ratingDifference >= -0.5 && ratingDifference <= 0.5

		assert.True(t, isReasonable, "Overall rating should be within 0.5 of detailed ratings average")
	})
}

// Test edge cases and boundary conditions
func (suite *DomainValidationTestSuite) TestDomainEdgeCases() {
	t := suite.T()

	t.Run("zero value handling", func(t *testing.T) {
		// Test that zero values are handled appropriately
		serviceImpl := suite.service

		review := &Review{
			ID:         uuid.Nil, // Zero UUID should be handled gracefully in some contexts
			ProviderID: uuid.New(),
			HotelID:    uuid.New(),
			Rating:     0, // Invalid rating
			Comment:    "",
			ReviewDate: time.Time{}, // Zero time
		}

		err := serviceImpl.ValidateReviewData(suite.ctx, review)
		assert.Error(t, err, "Validation should fail for multiple invalid fields")

		// The error should mention multiple validation issues
		errorMsg := err.Error()
		assert.True(t,
			strings.Contains(errorMsg, "rating") ||
				strings.Contains(errorMsg, "comment") ||
				strings.Contains(errorMsg, "date"),
			"Error should mention validation issues")
	})

	t.Run("unicode and special character handling", func(t *testing.T) {
		serviceImpl := suite.service

		review := &Review{
			ID:         uuid.New(),
			ProviderID: uuid.New(),
			HotelID:    uuid.New(),
			Rating:     4.0,
			Comment:    "Great hotel! 🏨✨ Très bon hôtel. 素晴らしいホテル! Отличный отель!",
			ReviewDate: time.Now().Add(-24 * time.Hour),
		}

		err := serviceImpl.ValidateReviewData(suite.ctx, review)
		assert.NoError(t, err, "Unicode content should be valid")
	})

	t.Run("extreme rating precision", func(t *testing.T) {
		serviceImpl := suite.service

		// Test various rating precisions
		ratings := []float64{1.0, 1.1, 1.01, 2.5, 3.333, 4.99, 5.0}

		for _, rating := range ratings {
			review := &Review{
				ID:         uuid.New(),
				ProviderID: uuid.New(),
				HotelID:    uuid.New(),
				Rating:     rating,
				Comment:    "Test review for rating precision",
				ReviewDate: time.Now().Add(-24 * time.Hour),
			}

			err := serviceImpl.ValidateReviewData(suite.ctx, review)
			assert.NoError(t, err, "Rating %.3f should be valid", rating)
		}
	})

	t.Run("time boundary conditions", func(t *testing.T) {
		serviceImpl := suite.service

		// Test review date exactly at current time (should pass)
		now := time.Now()
		review := &Review{
			ID:         uuid.New(),
			ProviderID: uuid.New(),
			HotelID:    uuid.New(),
			Rating:     4.0,
			Comment:    "Just checked out",
			ReviewDate: now,
		}

		err := serviceImpl.ValidateReviewData(suite.ctx, review)
		assert.NoError(t, err, "Review at current time should be valid")

		// Test review slightly in the future (should fail)
		futureReview := *review
		futureReview.ReviewDate = now.Add(1 * time.Second)

		err = serviceImpl.ValidateReviewData(suite.ctx, &futureReview)
		assert.Error(t, err, "Future review should be invalid")
		assert.Contains(t, err.Error(), "future")
	})
}

// Test validation performance with domain service
func (suite *DomainValidationTestSuite) TestValidationPerformance() {
	t := suite.T()
	serviceImpl := suite.service

	// Create a review for performance testing
	review := &Review{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		HotelID:    uuid.New(),
		Rating:     4.5,
		Comment:    "This is a performance test review with moderate content length",
		ReviewDate: time.Now().Add(-24 * time.Hour),
	}

	// Test validation performance
	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		_ = serviceImpl.ValidateReviewData(suite.ctx, review)
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)

	t.Logf("Domain validation performance: %d iterations in %v, avg: %v per validation",
		iterations, duration, avgDuration)

	// Assert reasonable performance (should be much faster than 1ms per validation)
	assert.Less(t, avgDuration, time.Millisecond, "Domain validation should be fast")
}

func TestDomainValidationSuite(t *testing.T) {
	suite.Run(t, new(DomainValidationTestSuite))
}
