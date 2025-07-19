package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
)

// TestReviewData represents test review data structure
type TestReviewData struct {
	ID            string  `json:"id"`
	HotelID       string  `json:"hotel_id"`
	HotelName     string  `json:"hotel_name"`
	Rating        float64 `json:"rating"`
	Title         string  `json:"title"`
	Comment       string  `json:"comment"`
	ReviewDate    string  `json:"review_date"`
	Language      string  `json:"language"`
	ReviewerName  string  `json:"reviewer_name"`
	ReviewerEmail string  `json:"reviewer_email"`
}

// TestDataGenerator provides utilities for generating test data
type TestDataGenerator struct {
	hotelID    uuid.UUID
	providerID uuid.UUID
	reviewerID uuid.UUID
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator(hotelID, providerID, reviewerID uuid.UUID) *TestDataGenerator {
	return &TestDataGenerator{
		hotelID:    hotelID,
		providerID: providerID,
		reviewerID: reviewerID,
	}
}

// GenerateReviewData generates test review data
func (g *TestDataGenerator) GenerateReviewData(count int) []TestReviewData {
	reviews := make([]TestReviewData, count)

	comments := []string{
		"Amazing hotel with excellent service and beautiful rooms!",
		"Great location, friendly staff, and clean facilities.",
		"Good value for money, would recommend to others.",
		"Average experience, nothing special but adequate.",
		"Poor service and outdated facilities, disappointed.",
		"Fantastic stay! The staff went above and beyond.",
		"Beautiful hotel with stunning views and great amenities.",
		"Comfortable rooms and convenient location near attractions.",
		"Excellent breakfast and helpful concierge service.",
		"Clean and modern hotel with professional staff.",
	}

	titles := []string{
		"Excellent stay!",
		"Great hotel",
		"Good value",
		"Average experience",
		"Disappointing",
		"Fantastic!",
		"Beautiful property",
		"Convenient location",
		"Great service",
		"Clean and modern",
	}

	for i := 0; i < count; i++ {
		rating := float64(1 + (i % 5)) // Cycle through ratings 1-5
		commentIndex := i % len(comments)
		titleIndex := i % len(titles)

		reviews[i] = TestReviewData{
			ID:            fmt.Sprintf("test-review-%d", i),
			HotelID:       g.hotelID.String(),
			HotelName:     "Test Hotel",
			Rating:        rating,
			Title:         titles[titleIndex],
			Comment:       comments[commentIndex],
			ReviewDate:    time.Now().AddDate(0, 0, -i).Format(time.RFC3339),
			Language:      "en",
			ReviewerName:  "Test Reviewer",
			ReviewerEmail: "test@example.com",
		}
	}

	return reviews
}

// GenerateInvalidReviewData generates invalid test review data for error testing
func (g *TestDataGenerator) GenerateInvalidReviewData() []TestReviewData {
	return []TestReviewData{
		{
			ID:            "invalid-rating",
			HotelID:       g.hotelID.String(),
			HotelName:     "Test Hotel",
			Rating:        6.0, // Invalid rating > 5
			Title:         "Invalid rating test",
			Comment:       "This review has an invalid rating",
			ReviewDate:    time.Now().Format(time.RFC3339),
			Language:      "en",
			ReviewerName:  "Test Reviewer",
			ReviewerEmail: "test@example.com",
		},
		{
			ID:            "empty-comment",
			HotelID:       g.hotelID.String(),
			HotelName:     "Test Hotel",
			Rating:        4.0,
			Title:         "Empty comment test",
			Comment:       "", // Empty comment
			ReviewDate:    time.Now().Format(time.RFC3339),
			Language:      "en",
			ReviewerName:  "Test Reviewer",
			ReviewerEmail: "test@example.com",
		},
		{
			ID:            "future-date",
			HotelID:       g.hotelID.String(),
			HotelName:     "Test Hotel",
			Rating:        4.0,
			Title:         "Future date test",
			Comment:       "This review has a future date",
			ReviewDate:    time.Now().AddDate(0, 0, 1).Format(time.RFC3339), // Future date
			Language:      "en",
			ReviewerName:  "Test Reviewer",
			ReviewerEmail: "test@example.com",
		},
		{
			ID:            "missing-hotel-id",
			HotelID:       "", // Missing hotel ID
			HotelName:     "Test Hotel",
			Rating:        4.0,
			Title:         "Missing hotel ID test",
			Comment:       "This review has missing hotel ID",
			ReviewDate:    time.Now().Format(time.RFC3339),
			Language:      "en",
			ReviewerName:  "Test Reviewer",
			ReviewerEmail: "test@example.com",
		},
	}
}

// GenerateMultiLanguageReviewData generates reviews in multiple languages
func (g *TestDataGenerator) GenerateMultiLanguageReviewData() []TestReviewData {
	return []TestReviewData{
		{
			ID:            "en-review",
			HotelID:       g.hotelID.String(),
			HotelName:     "Test Hotel",
			Rating:        4.5,
			Title:         "Great hotel",
			Comment:       "This hotel was amazing with excellent service!",
			ReviewDate:    time.Now().Format(time.RFC3339),
			Language:      "en",
			ReviewerName:  "John Smith",
			ReviewerEmail: "john@example.com",
		},
		{
			ID:            "es-review",
			HotelID:       g.hotelID.String(),
			HotelName:     "Test Hotel",
			Rating:        4.0,
			Title:         "Buen hotel",
			Comment:       "El hotel fue muy bueno con excelente servicio!",
			ReviewDate:    time.Now().Format(time.RFC3339),
			Language:      "es",
			ReviewerName:  "Juan García",
			ReviewerEmail: "juan@example.com",
		},
		{
			ID:            "fr-review",
			HotelID:       g.hotelID.String(),
			HotelName:     "Test Hotel",
			Rating:        3.5,
			Title:         "Bon hôtel",
			Comment:       "L'hôtel était très bien avec un excellent service!",
			ReviewDate:    time.Now().Format(time.RFC3339),
			Language:      "fr",
			ReviewerName:  "Pierre Dubois",
			ReviewerEmail: "pierre@example.com",
		},
	}
}

// FileTestUtils provides utilities for file operations in tests
type FileTestUtils struct {
	tempDir string
}

// NewFileTestUtils creates a new file test utils instance
func NewFileTestUtils() *FileTestUtils {
	return &FileTestUtils{
		tempDir: os.TempDir(),
	}
}

// CreateJSONLinesFile creates a JSON Lines file with the given data
func (f *FileTestUtils) CreateJSONLinesFile(t *testing.T, filename string, data []TestReviewData) string {
	filePath := filepath.Join(f.tempDir, filename)

	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer file.Close()

	for _, item := range data {
		jsonData, err := json.Marshal(item)
		require.NoError(t, err)

		_, err = file.Write(jsonData)
		require.NoError(t, err)

		_, err = file.Write([]byte("\n"))
		require.NoError(t, err)
	}

	return filePath
}

// CreateLargeJSONLinesFile creates a large JSON Lines file for performance testing
func (f *FileTestUtils) CreateLargeJSONLinesFile(t *testing.T, filename string, recordCount int, generator *TestDataGenerator) string {
	filePath := filepath.Join(f.tempDir, filename)

	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer file.Close()

	// Generate data in batches to avoid memory issues
	batchSize := 1000
	for i := 0; i < recordCount; i += batchSize {
		end := i + batchSize
		if end > recordCount {
			end = recordCount
		}

		batchData := generator.GenerateReviewData(end - i)

		for j, item := range batchData {
			// Update ID to be unique across batches
			item.ID = fmt.Sprintf("large-review-%d", i+j)

			jsonData, err := json.Marshal(item)
			require.NoError(t, err)

			_, err = file.Write(jsonData)
			require.NoError(t, err)

			_, err = file.Write([]byte("\n"))
			require.NoError(t, err)
		}
	}

	return filePath
}

// CreateCorruptedJSONLinesFile creates a file with corrupted JSON data
func (f *FileTestUtils) CreateCorruptedJSONLinesFile(t *testing.T, filename string) string {
	filePath := filepath.Join(f.tempDir, filename)

	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer file.Close()

	corruptedData := []string{
		`{"id": "valid-1", "rating": 4.0, "comment": "Valid review"}`,
		`{"id": "invalid-json", "rating": 4.0, "comment": "Missing closing brace"`,
		`invalid json line without proper format`,
		`{"id": "valid-2", "rating": 3.5, "comment": "Another valid review"}`,
		`{"id": "incomplete",`,
	}

	for _, line := range corruptedData {
		_, err = file.Write([]byte(line + "\n"))
		require.NoError(t, err)
	}

	return filePath
}

// GetFileContent reads and returns the content of a file
func (f *FileTestUtils) GetFileContent(t *testing.T, filePath string) string {
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	return string(content)
}

// CleanupFile removes a test file
func (f *FileTestUtils) CleanupFile(filePath string) {
	os.Remove(filePath)
}

// DatabaseTestUtils provides utilities for database testing
type DatabaseTestUtils struct {
	repo domain.ReviewRepository
}

// NewDatabaseTestUtils creates a new database test utils instance
func NewDatabaseTestUtils(repo domain.ReviewRepository) *DatabaseTestUtils {
	return &DatabaseTestUtils{
		repo: repo,
	}
}

// CreateTestProvider creates a test provider in the database
func (d *DatabaseTestUtils) CreateTestProvider(ctx context.Context, t *testing.T, name string) *domain.Provider {
	provider := &domain.Provider{
		ID:       uuid.New(),
		Name:     name,
		BaseURL:  fmt.Sprintf("https://%s.example.com", strings.ToLower(name)),
		IsActive: true,
	}

	err := d.repo.CreateProvider(ctx, provider)
	require.NoError(t, err)

	return provider
}

// CreateTestHotel creates a test hotel in the database
func (d *DatabaseTestUtils) CreateTestHotel(ctx context.Context, t *testing.T, name, city, country string) *domain.Hotel {
	hotel := &domain.Hotel{
		ID:          uuid.New(),
		Name:        name,
		Address:     "123 Test Street",
		City:        city,
		Country:     country,
		StarRating:  4,
		Description: fmt.Sprintf("Test hotel %s in %s, %s", name, city, country),
		Latitude:    40.7128 + float64(len(name))*0.001, // Slightly different coordinates
		Longitude:   -74.0060 + float64(len(name))*0.001,
	}

	err := d.repo.CreateHotel(ctx, hotel)
	require.NoError(t, err)

	return hotel
}

// CreateTestReviewerInfo creates a test reviewer in the database
func (d *DatabaseTestUtils) CreateTestReviewerInfo(ctx context.Context, t *testing.T, name, email, country string) *domain.ReviewerInfo {
	reviewer := &domain.ReviewerInfo{
		ID:            uuid.New(),
		Name:          name,
		Email:         email,
		Country:       country,
		IsVerified:    true,
		TotalReviews:  0,
		AverageRating: 0.0,
	}

	err := d.repo.CreateReviewerInfo(ctx, reviewer)
	require.NoError(t, err)

	return reviewer
}

// CountReviewsByProvider counts reviews for a specific provider
func (d *DatabaseTestUtils) CountReviewsByProvider(ctx context.Context, t *testing.T, providerID uuid.UUID) int {
	reviews, err := d.repo.GetByProvider(ctx, providerID, 10000, 0)
	require.NoError(t, err)
	return len(reviews)
}

// CountReviewsByHotel counts reviews for a specific hotel
func (d *DatabaseTestUtils) CountReviewsByHotel(ctx context.Context, t *testing.T, hotelID uuid.UUID) int {
	reviews, err := d.repo.GetByHotel(ctx, hotelID, 10000, 0)
	require.NoError(t, err)
	return len(reviews)
}

// GetReviewByExternalID finds a review by external ID
func (d *DatabaseTestUtils) GetReviewByExternalID(ctx context.Context, t *testing.T, externalID string) *domain.Review {
	// Search for review with specific external ID
	filters := map[string]interface{}{
		"external_id": externalID,
	}

	reviews, err := d.repo.Search(ctx, "", filters, 1, 0)
	require.NoError(t, err)

	if len(reviews) == 0 {
		return nil
	}

	return &reviews[0]
}

// CleanupTestData removes test data from the database
func (d *DatabaseTestUtils) CleanupTestData(ctx context.Context, t *testing.T, providerID uuid.UUID) {
	// Get all reviews for the provider
	reviews, err := d.repo.GetByProvider(ctx, providerID, 10000, 0)
	require.NoError(t, err)

	// Delete all reviews
	for _, review := range reviews {
		err = d.repo.DeleteByID(ctx, review.ID)
		require.NoError(t, err)
	}

	// Delete the provider
	err = d.repo.DeleteProvider(ctx, providerID)
	require.NoError(t, err)
}

// AssertionUtils provides custom assertion utilities
type AssertionUtils struct{}

// NewAssertionUtils creates a new assertion utils instance
func NewAssertionUtils() *AssertionUtils {
	return &AssertionUtils{}
}

// AssertReviewDataMatches asserts that review data matches expected values
func (a *AssertionUtils) AssertReviewDataMatches(t *testing.T, expected TestReviewData, actual domain.Review) {
	require.Equal(t, expected.Rating, actual.Rating, "Rating should match")
	require.Equal(t, expected.Title, actual.Title, "Title should match")
	require.Equal(t, expected.Comment, actual.Comment, "Comment should match")
	require.Equal(t, expected.Language, actual.Language, "Language should match")
	require.Equal(t, expected.ID, actual.ExternalID, "External ID should match")

	// Parse expected date and compare
	expectedDate, err := time.Parse(time.RFC3339, expected.ReviewDate)
	require.NoError(t, err)
	require.True(t, expectedDate.Equal(actual.ReviewDate), "Review date should match")
}

// AssertProcessingCompleted asserts that processing completed successfully
func (a *AssertionUtils) AssertProcessingCompleted(t *testing.T, status *domain.ReviewProcessingStatus, expectedRecords int64) {
	require.Equal(t, "completed", status.Status, "Processing should be completed")
	require.Equal(t, expectedRecords, status.RecordsProcessed, "Records processed should match")
	require.Empty(t, status.ErrorMsg, "Should have no error message")
	require.NotNil(t, status.CompletedAt, "Should have completion timestamp")
}

// AssertProcessingFailed asserts that processing failed with expected error
func (a *AssertionUtils) AssertProcessingFailed(t *testing.T, status *domain.ReviewProcessingStatus, expectedErrorSubstring string) {
	require.Equal(t, "failed", status.Status, "Processing should be failed")
	require.NotEmpty(t, status.ErrorMsg, "Should have error message")
	require.Contains(t, status.ErrorMsg, expectedErrorSubstring, "Error message should contain expected substring")
	require.NotNil(t, status.CompletedAt, "Should have completion timestamp")
}

// PerformanceTestUtils provides utilities for performance testing
type PerformanceTestUtils struct{}

// NewPerformanceTestUtils creates a new performance test utils instance
func NewPerformanceTestUtils() *PerformanceTestUtils {
	return &PerformanceTestUtils{}
}

// MeasureProcessingTime measures the time taken for processing
func (p *PerformanceTestUtils) MeasureProcessingTime(operation func()) time.Duration {
	start := time.Now()
	operation()
	return time.Since(start)
}

// AssertProcessingTimeWithinRange asserts that processing time is within expected range
func (p *PerformanceTestUtils) AssertProcessingTimeWithinRange(t *testing.T, duration time.Duration, min, max time.Duration) {
	require.True(t, duration >= min, "Processing time should be at least %v, got %v", min, duration)
	require.True(t, duration <= max, "Processing time should be at most %v, got %v", max, duration)
}

// CalculateProcessingRate calculates the processing rate (records per second)
func (p *PerformanceTestUtils) CalculateProcessingRate(recordCount int, duration time.Duration) float64 {
	if duration.Seconds() == 0 {
		return 0
	}
	return float64(recordCount) / duration.Seconds()
}

// AssertProcessingRateAboveThreshold asserts that processing rate meets minimum threshold
func (p *PerformanceTestUtils) AssertProcessingRateAboveThreshold(t *testing.T, rate float64, threshold float64) {
	require.True(t, rate >= threshold, "Processing rate should be at least %.2f records/sec, got %.2f", threshold, rate)
}

// S3TestUtils provides utilities for S3 testing
type S3TestUtils struct {
	s3Client domain.S3Client
	bucket   string
}

// NewS3TestUtils creates a new S3 test utils instance
func NewS3TestUtils(s3Client domain.S3Client, bucket string) *S3TestUtils {
	return &S3TestUtils{
		s3Client: s3Client,
		bucket:   bucket,
	}
}

// UploadTestFile uploads a test file to S3
func (s *S3TestUtils) UploadTestFile(ctx context.Context, t *testing.T, filename, content string) {
	err := s.s3Client.UploadFile(ctx, s.bucket, filename, strings.NewReader(content), "text/plain")
	require.NoError(t, err)
}

// UploadFileFromPath uploads a file from local path to S3
func (s *S3TestUtils) UploadFileFromPath(ctx context.Context, t *testing.T, localPath, s3Key string) {
	file, err := os.Open(localPath)
	require.NoError(t, err)
	defer file.Close()

	err = s.s3Client.UploadFile(ctx, s.bucket, s3Key, file, "application/json")
	require.NoError(t, err)
}

// AssertFileExists asserts that a file exists in S3
func (s *S3TestUtils) AssertFileExists(ctx context.Context, t *testing.T, key string) {
	exists, err := s.s3Client.FileExists(ctx, s.bucket, key)
	require.NoError(t, err)
	require.True(t, exists, "File %s should exist in S3", key)
}

// AssertFileNotExists asserts that a file does not exist in S3
func (s *S3TestUtils) AssertFileNotExists(ctx context.Context, t *testing.T, key string) {
	exists, err := s.s3Client.FileExists(ctx, s.bucket, key)
	require.NoError(t, err)
	require.False(t, exists, "File %s should not exist in S3", key)
}

// GetFileContent downloads and returns the content of a file from S3
func (s *S3TestUtils) GetFileContent(ctx context.Context, t *testing.T, key string) string {
	reader, err := s.s3Client.DownloadFile(ctx, s.bucket, key)
	require.NoError(t, err)
	defer reader.Close()

	content, err := io.ReadAll(reader)
	require.NoError(t, err)

	return string(content)
}

// CleanupTestFiles removes test files from S3
func (s *S3TestUtils) CleanupTestFiles(ctx context.Context, t *testing.T, keys []string) {
	for _, key := range keys {
		err := s.s3Client.DeleteFile(ctx, s.bucket, key)
		if err != nil {
			t.Logf("Warning: failed to cleanup S3 file %s: %v", key, err)
		}
	}
}

// GetS3URL returns the S3 URL for a key
func (s *S3TestUtils) GetS3URL(key string) string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, key)
}
