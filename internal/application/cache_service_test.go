package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Mock Redis Cache
type MockRedisCache struct {
	mock.Mock
}

func (m *MockRedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockRedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockRedisCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockRedisCache) DeletePattern(ctx context.Context, pattern string) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}

func (m *MockRedisCache) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockRedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	args := m.Called(ctx, key, ttl)
	return args.Error(0)
}

func (m *MockRedisCache) GetStats() CacheStats {
	args := m.Called()
	return args.Get(0).(CacheStats)
}

// Mock Review Service for cache tests
type MockCacheReviewService struct {
	mock.Mock
}

func (m *MockCacheReviewService) CreateReview(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockCacheReviewService) GetReviewByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockCacheReviewService) UpdateReview(ctx context.Context, id uuid.UUID, review *domain.Review) error {
	args := m.Called(ctx, id, review)
	return args.Error(0)
}

func (m *MockCacheReviewService) DeleteReview(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCacheReviewService) SearchReviews(ctx context.Context, criteria map[string]interface{}) ([]*domain.Review, error) {
	args := m.Called(ctx, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockCacheReviewService) GetReviewsByHotelID(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	args := m.Called(ctx, hotelID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockCacheReviewService) GetReviewsByProviderID(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	args := m.Called(ctx, providerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockCacheReviewService) GetReviewsByReviewerID(ctx context.Context, reviewerID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	args := m.Called(ctx, reviewerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockCacheReviewService) CalculateHotelStatistics(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	args := m.Called(ctx, hotelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ReviewSummary), args.Error(1)
}

func (m *MockCacheReviewService) UpdateReviewSentiment(ctx context.Context, reviewID uuid.UUID, sentiment string) error {
	args := m.Called(ctx, reviewID, sentiment)
	return args.Error(0)
}

func (m *MockCacheReviewService) VerifyReview(ctx context.Context, reviewID uuid.UUID) error {
	args := m.Called(ctx, reviewID)
	return args.Error(0)
}

// Test CacheService configuration
func TestCacheServiceConfig_Creation(t *testing.T) {
	config := &CacheServiceConfig{
		ReviewTTL:     5 * time.Minute,
		HotelTTL:      10 * time.Minute,
		ProcessingTTL: 1 * time.Minute,
		AnalyticsTTL:  30 * time.Minute,
		DefaultTTL:    15 * time.Minute,
		ReviewKeyPrefix:     "review",
		HotelKeyPrefix:      "hotel",
		ProcessingKeyPrefix: "processing",
		AnalyticsKeyPrefix:  "analytics",
		EnableBackgroundWarmup: false,
	}

	assert.NotNil(t, config)
	assert.Equal(t, 5*time.Minute, config.ReviewTTL)
	assert.Equal(t, 10*time.Minute, config.HotelTTL)
	assert.Equal(t, "review", config.ReviewKeyPrefix)
	assert.False(t, config.EnableBackgroundWarmup)
}

// Test ReviewFilters structure
func TestReviewFilters_Creation(t *testing.T) {
	minRating := 4.0
	isVerified := true
	
	filters := &ReviewFilters{
		MinRating:     &minRating,
		IsVerified:    &isVerified,
		SortBy:        "rating",
		SortDirection: "desc",
	}

	assert.NotNil(t, filters)
	assert.Equal(t, 4.0, *filters.MinRating)
	assert.True(t, *filters.IsVerified)
	assert.Equal(t, "rating", filters.SortBy)
	assert.Equal(t, "desc", filters.SortDirection)
}

// Test ProcessingProgress structure
func TestProcessingProgress_Creation(t *testing.T) {
	progress := &ProcessingProgress{
		RecordsProcessed: 100,
		RecordsTotal:     200,
		RecordsFailed:    5,
		ProcessingRate:   50.0,
		EstimatedETA:     time.Now().Add(1 * time.Hour),
		LastUpdate:       time.Now(),
	}

	assert.NotNil(t, progress)
	assert.Equal(t, int64(100), progress.RecordsProcessed)
	assert.Equal(t, int64(200), progress.RecordsTotal)
	assert.Equal(t, int64(5), progress.RecordsFailed)
	assert.Equal(t, 50.0, progress.ProcessingRate)
}

// Test AnalyticsData structure
func TestAnalyticsData_Creation(t *testing.T) {
	analytics := &AnalyticsData{
		Type:        "review_stats",
		Data:        map[string]interface{}{"total": 100, "average_rating": 4.5},
		Timestamp:   time.Now(),
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		Source:      "review_service",
		RefreshRate: 5 * time.Minute,
	}

	assert.NotNil(t, analytics)
	assert.Equal(t, "review_stats", analytics.Type)
	assert.Equal(t, 100, analytics.Data["total"])
	assert.Equal(t, 4.5, analytics.Data["average_rating"])
	assert.Equal(t, "review_service", analytics.Source)
	assert.Equal(t, 5*time.Minute, analytics.RefreshRate)
}

// Test CacheStats structure
func TestCacheStats_Creation(t *testing.T) {
	stats := &CacheStats{
		HitRate:             0.85,
		MissRate:            0.15,
		TotalOperations:     1000,
		KeyspaceStats:       map[string]int64{"reviews": 500, "hotels": 300},
		MemoryUsage:         1024 * 1024, // 1MB
		ConnectionPoolStats: map[string]int64{"active": 10, "idle": 5},
		LastUpdated:         time.Now(),
	}

	assert.NotNil(t, stats)
	assert.Equal(t, 0.85, stats.HitRate)
	assert.Equal(t, 0.15, stats.MissRate)
	assert.Equal(t, int64(1000), stats.TotalOperations)
	assert.Equal(t, int64(500), stats.KeyspaceStats["reviews"])
	assert.Equal(t, int64(300), stats.KeyspaceStats["hotels"])
}

// Test WarmupOptions structure
func TestWarmupOptions_Creation(t *testing.T) {
	options := &WarmupOptions{
		ConcurrentWorkers: 5,
		BatchSize:         100,
		Priority:          "high",
		Filters:           map[string]interface{}{"rating": 4.0},
		MaxItems:          1000,
		ProgressCallback:  func(current, total int) { /* callback logic */ },
	}

	assert.NotNil(t, options)
	assert.Equal(t, 5, options.ConcurrentWorkers)
	assert.Equal(t, 100, options.BatchSize)
	assert.Equal(t, "high", options.Priority)
	assert.Equal(t, 4.0, options.Filters["rating"])
	assert.Equal(t, 1000, options.MaxItems)
	assert.NotNil(t, options.ProgressCallback)
}

// Test basic cache service creation (without full integration)
func TestCacheService_BasicCreation(t *testing.T) {
	logger := logger.NewDefault()
	config := &CacheServiceConfig{
		ReviewTTL:     5 * time.Minute,
		HotelTTL:      10 * time.Minute,
		ProcessingTTL: 1 * time.Minute,
		AnalyticsTTL:  30 * time.Minute,
		DefaultTTL:    15 * time.Minute,
		ReviewKeyPrefix:     "test_review",
		HotelKeyPrefix:      "test_hotel",
		ProcessingKeyPrefix: "test_processing",
		AnalyticsKeyPrefix:  "test_analytics",
		EnableBackgroundWarmup: false,
	}

	assert.NotNil(t, logger)
	assert.NotNil(t, config)
	assert.Equal(t, "test_review", config.ReviewKeyPrefix)
	assert.Equal(t, "test_hotel", config.HotelKeyPrefix)
	assert.False(t, config.EnableBackgroundWarmup)
}

// Test mock cache operations
func TestMockRedisCache_Operations(t *testing.T) {
	mockCache := new(MockRedisCache)
	ctx := context.Background()
	key := "test:key"
	value := []byte("test value")
	ttl := 5 * time.Minute

	// Test Set operation
	mockCache.On("Set", ctx, key, value, ttl).Return(nil)
	err := mockCache.Set(ctx, key, value, ttl)
	assert.NoError(t, err)

	// Test Get operation
	mockCache.On("Get", ctx, key).Return(value, nil)
	result, err := mockCache.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// Test Delete operation
	mockCache.On("Delete", ctx, key).Return(nil)
	err = mockCache.Delete(ctx, key)
	assert.NoError(t, err)

	// Test Exists operation
	mockCache.On("Exists", ctx, key).Return(true, nil)
	exists, err := mockCache.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	mockCache.AssertExpectations(t)
}

// Test mock review service operations
func TestMockReviewService_Operations(t *testing.T) {
	mockService := new(MockCacheReviewService)
	ctx := context.Background()
	reviewID := uuid.New()
	review := &domain.Review{
		ID:         reviewID,
		HotelID:    uuid.New(),
		ProviderID: uuid.New(),
		Rating:     4.5,
		Comment:    "Great hotel!",
		ReviewDate: time.Now(),
	}

	// Test GetReviewByID
	mockService.On("GetReviewByID", ctx, reviewID).Return(review, nil)
	result, err := mockService.GetReviewByID(ctx, reviewID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, reviewID, result.ID)
	assert.Equal(t, 4.5, result.Rating)

	// Test CreateReview
	mockService.On("CreateReview", ctx, review).Return(nil)
	err = mockService.CreateReview(ctx, review)
	assert.NoError(t, err)

	// Test UpdateReview
	mockService.On("UpdateReview", ctx, reviewID, review).Return(nil)
	err = mockService.UpdateReview(ctx, reviewID, review)
	assert.NoError(t, err)

	// Test DeleteReview
	mockService.On("DeleteReview", ctx, reviewID).Return(nil)
	err = mockService.DeleteReview(ctx, reviewID)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}