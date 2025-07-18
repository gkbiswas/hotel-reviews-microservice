package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// CacheService defines the interface for cache operations
type CacheService interface {
	// Review caching
	GetReview(ctx context.Context, reviewID uuid.UUID) (*domain.Review, error)
	SetReview(ctx context.Context, review *domain.Review) error
	GetReviewsByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]*domain.Review, error)
	SetReviewsByProvider(ctx context.Context, providerID uuid.UUID, reviews []*domain.Review, limit, offset int) error
	GetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int, filters *ReviewFilters) ([]*domain.Review, error)
	SetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, reviews []*domain.Review, limit, offset int, filters *ReviewFilters) error
	InvalidateReview(ctx context.Context, reviewID uuid.UUID) error
	InvalidateReviewsByProvider(ctx context.Context, providerID uuid.UUID) error
	InvalidateReviewsByHotel(ctx context.Context, hotelID uuid.UUID) error

	// Hotel caching
	GetHotel(ctx context.Context, hotelID uuid.UUID) (*domain.Hotel, error)
	SetHotel(ctx context.Context, hotel *domain.Hotel) error
	GetHotelSummary(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error)
	SetHotelSummary(ctx context.Context, summary *domain.ReviewSummary) error
	GetHotelsByCity(ctx context.Context, city string, limit, offset int) ([]*domain.Hotel, error)
	SetHotelsByCity(ctx context.Context, city string, hotels []*domain.Hotel, limit, offset int) error
	InvalidateHotel(ctx context.Context, hotelID uuid.UUID) error
	InvalidateHotelsByCity(ctx context.Context, city string) error

	// Processing status caching
	GetProcessingStatus(ctx context.Context, jobID uuid.UUID) (*domain.ReviewProcessingStatus, error)
	SetProcessingStatus(ctx context.Context, status *domain.ReviewProcessingStatus) error
	UpdateProcessingProgress(ctx context.Context, jobID uuid.UUID, progress ProcessingProgress) error
	GetActiveProcessingJobs(ctx context.Context) ([]*domain.ReviewProcessingStatus, error)
	SetActiveProcessingJobs(ctx context.Context, jobs []*domain.ReviewProcessingStatus) error
	InvalidateProcessingStatus(ctx context.Context, jobID uuid.UUID) error

	// Analytics caching
	GetAnalytics(ctx context.Context, key string) (*AnalyticsData, error)
	SetAnalytics(ctx context.Context, key string, analytics *AnalyticsData) error
	InvalidateAnalytics(ctx context.Context, pattern string) error

	// Cache warming
	WarmupReviews(ctx context.Context, options *WarmupOptions) error
	WarmupHotels(ctx context.Context, options *WarmupOptions) error
	WarmupProcessingStatus(ctx context.Context, options *WarmupOptions) error
	WarmupAnalytics(ctx context.Context, options *WarmupOptions) error

	// Cache management
	GetCacheStats(ctx context.Context) (*CacheStats, error)
	ClearCache(ctx context.Context, pattern string) error
	HealthCheck(ctx context.Context) error
}

// ReviewFilters represents filters for review queries
type ReviewFilters struct {
	MinRating     *float64  `json:"min_rating,omitempty"`
	MaxRating     *float64  `json:"max_rating,omitempty"`
	Language      *string   `json:"language,omitempty"`
	Sentiment     *string   `json:"sentiment,omitempty"`
	DateFrom      *time.Time `json:"date_from,omitempty"`
	DateTo        *time.Time `json:"date_to,omitempty"`
	IsVerified    *bool     `json:"is_verified,omitempty"`
	TripType      *string   `json:"trip_type,omitempty"`
	SortBy        string    `json:"sort_by"`
	SortDirection string    `json:"sort_direction"`
}

// ProcessingProgress represents processing progress information
type ProcessingProgress struct {
	RecordsProcessed int64     `json:"records_processed"`
	RecordsTotal     int64     `json:"records_total"`
	RecordsFailed    int64     `json:"records_failed"`
	ProcessingRate   float64   `json:"processing_rate"`
	EstimatedETA     time.Time `json:"estimated_eta"`
	LastUpdate       time.Time `json:"last_update"`
}

// AnalyticsData represents cached analytics data
type AnalyticsData struct {
	Type        string                 `json:"type"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   time.Time              `json:"timestamp"`
	ExpiresAt   time.Time              `json:"expires_at"`
	Source      string                 `json:"source"`
	RefreshRate time.Duration          `json:"refresh_rate"`
}

// WarmupOptions represents options for cache warming
type WarmupOptions struct {
	ConcurrentWorkers int                    `json:"concurrent_workers"`
	BatchSize         int                    `json:"batch_size"`
	Priority          string                 `json:"priority"`
	Filters           map[string]interface{} `json:"filters"`
	MaxItems          int                    `json:"max_items"`
	ProgressCallback  func(int, int)         `json:"-"`
}

// CacheStats represents cache statistics
type CacheStats struct {
	HitRate              float64            `json:"hit_rate"`
	MissRate             float64            `json:"miss_rate"`
	TotalOperations      int64              `json:"total_operations"`
	KeyspaceStats        map[string]int64   `json:"keyspace_stats"`
	MemoryUsage          int64              `json:"memory_usage"`
	ConnectionPoolStats  map[string]int64   `json:"connection_pool_stats"`
	LastUpdated          time.Time          `json:"last_updated"`
}

// CacheServiceImpl implements the CacheService interface
type CacheServiceImpl struct {
	redisClient     *infrastructure.RedisClient
	reviewService   domain.ReviewService
	logger          *logger.Logger
	config          *CacheServiceConfig
	warmupQueue     chan WarmupTask
	invalidationLog []InvalidationEvent
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// CacheServiceConfig holds configuration for the cache service
type CacheServiceConfig struct {
	// TTL settings
	ReviewTTL          time.Duration `json:"review_ttl"`
	HotelTTL           time.Duration `json:"hotel_ttl"`
	ProcessingTTL      time.Duration `json:"processing_ttl"`
	AnalyticsTTL       time.Duration `json:"analytics_ttl"`
	DefaultTTL         time.Duration `json:"default_ttl"`
	
	// Cache keys
	ReviewKeyPrefix          string `json:"review_key_prefix"`
	HotelKeyPrefix          string `json:"hotel_key_prefix"`
	ProcessingKeyPrefix     string `json:"processing_key_prefix"`
	AnalyticsKeyPrefix      string `json:"analytics_key_prefix"`
	
	// Warming settings
	WarmupConcurrency       int  `json:"warmup_concurrency"`
	WarmupBatchSize         int  `json:"warmup_batch_size"`
	EnableBackgroundWarmup  bool `json:"enable_background_warmup"`
	WarmupInterval          time.Duration `json:"warmup_interval"`
	
	// Invalidation settings
	InvalidationBatchSize   int           `json:"invalidation_batch_size"`
	InvalidationDelay       time.Duration `json:"invalidation_delay"`
	EnableSmartInvalidation bool          `json:"enable_smart_invalidation"`
	
	// Performance settings
	MaxConcurrentOperations int           `json:"max_concurrent_operations"`
	OperationTimeout        time.Duration `json:"operation_timeout"`
	RetryAttempts           int           `json:"retry_attempts"`
	RetryDelay              time.Duration `json:"retry_delay"`
}

// WarmupTask represents a cache warming task
type WarmupTask struct {
	Type     string                 `json:"type"`
	Priority int                    `json:"priority"`
	Data     map[string]interface{} `json:"data"`
	Options  *WarmupOptions         `json:"options"`
}

// InvalidationEvent represents a cache invalidation event
type InvalidationEvent struct {
	Type      string    `json:"type"`
	Key       string    `json:"key"`
	Pattern   string    `json:"pattern"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

// NewCacheService creates a new cache service
func NewCacheService(
	redisClient *infrastructure.RedisClient,
	reviewService domain.ReviewService,
	logger *logger.Logger,
	config *CacheServiceConfig,
) *CacheServiceImpl {
	ctx, cancel := context.WithCancel(context.Background())
	
	service := &CacheServiceImpl{
		redisClient:     redisClient,
		reviewService:   reviewService,
		logger:          logger,
		config:          config,
		warmupQueue:     make(chan WarmupTask, 1000),
		invalidationLog: make([]InvalidationEvent, 0),
		ctx:             ctx,
		cancel:          cancel,
	}
	
	// Start background processes
	if config.EnableBackgroundWarmup {
		service.wg.Add(1)
		go service.warmupWorker()
	}
	
	return service
}

// Close closes the cache service
func (c *CacheServiceImpl) Close() error {
	c.cancel()
	c.wg.Wait()
	return nil
}

// Review Caching Methods

// GetReview retrieves a review from cache
func (c *CacheServiceImpl) GetReview(ctx context.Context, reviewID uuid.UUID) (*domain.Review, error) {
	key := fmt.Sprintf("%s:review:%s", c.config.ReviewKeyPrefix, reviewID.String())
	
	cached, err := c.redisClient.Get(ctx, "reviews", key)
	if err != nil {
		c.logger.Error("Failed to get review from cache", "review_id", reviewID, "error", err)
		return nil, err
	}
	
	if cached == "" {
		return nil, nil // Cache miss
	}
	
	var review domain.Review
	if err := json.Unmarshal([]byte(cached), &review); err != nil {
		c.logger.Error("Failed to unmarshal cached review", "review_id", reviewID, "error", err)
		return nil, err
	}
	
	return &review, nil
}

// SetReview stores a review in cache
func (c *CacheServiceImpl) SetReview(ctx context.Context, review *domain.Review) error {
	key := fmt.Sprintf("%s:review:%s", c.config.ReviewKeyPrefix, review.ID.String())
	
	data, err := json.Marshal(review)
	if err != nil {
		return fmt.Errorf("failed to marshal review: %w", err)
	}
	
	return c.redisClient.Set(ctx, "reviews", key, string(data), c.config.ReviewTTL)
}

// GetReviewsByProvider retrieves reviews by provider from cache
func (c *CacheServiceImpl) GetReviewsByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	key := fmt.Sprintf("%s:provider:%s:reviews:%d:%d", c.config.ReviewKeyPrefix, providerID.String(), limit, offset)
	
	cached, err := c.redisClient.Get(ctx, "provider_reviews", key)
	if err != nil {
		c.logger.Error("Failed to get provider reviews from cache", "provider_id", providerID, "error", err)
		return nil, err
	}
	
	if cached == "" {
		return nil, nil // Cache miss
	}
	
	var reviews []*domain.Review
	if err := json.Unmarshal([]byte(cached), &reviews); err != nil {
		c.logger.Error("Failed to unmarshal cached provider reviews", "provider_id", providerID, "error", err)
		return nil, err
	}
	
	return reviews, nil
}

// SetReviewsByProvider stores reviews by provider in cache
func (c *CacheServiceImpl) SetReviewsByProvider(ctx context.Context, providerID uuid.UUID, reviews []*domain.Review, limit, offset int) error {
	key := fmt.Sprintf("%s:provider:%s:reviews:%d:%d", c.config.ReviewKeyPrefix, providerID.String(), limit, offset)
	
	data, err := json.Marshal(reviews)
	if err != nil {
		return fmt.Errorf("failed to marshal provider reviews: %w", err)
	}
	
	return c.redisClient.Set(ctx, "provider_reviews", key, string(data), c.config.ReviewTTL)
}

// GetReviewsByHotel retrieves reviews by hotel from cache with filters
func (c *CacheServiceImpl) GetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int, filters *ReviewFilters) ([]*domain.Review, error) {
	key := c.buildHotelReviewsKey(hotelID, limit, offset, filters)
	
	cached, err := c.redisClient.Get(ctx, "hotel_reviews", key)
	if err != nil {
		c.logger.Error("Failed to get hotel reviews from cache", "hotel_id", hotelID, "error", err)
		return nil, err
	}
	
	if cached == "" {
		return nil, nil // Cache miss
	}
	
	var reviews []*domain.Review
	if err := json.Unmarshal([]byte(cached), &reviews); err != nil {
		c.logger.Error("Failed to unmarshal cached hotel reviews", "hotel_id", hotelID, "error", err)
		return nil, err
	}
	
	return reviews, nil
}

// SetReviewsByHotel stores reviews by hotel in cache
func (c *CacheServiceImpl) SetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, reviews []*domain.Review, limit, offset int, filters *ReviewFilters) error {
	key := c.buildHotelReviewsKey(hotelID, limit, offset, filters)
	
	data, err := json.Marshal(reviews)
	if err != nil {
		return fmt.Errorf("failed to marshal hotel reviews: %w", err)
	}
	
	return c.redisClient.Set(ctx, "hotel_reviews", key, string(data), c.config.ReviewTTL)
}

// buildHotelReviewsKey builds a cache key for hotel reviews with filters
func (c *CacheServiceImpl) buildHotelReviewsKey(hotelID uuid.UUID, limit, offset int, filters *ReviewFilters) string {
	key := fmt.Sprintf("%s:hotel:%s:reviews:%d:%d", c.config.ReviewKeyPrefix, hotelID.String(), limit, offset)
	
	if filters != nil {
		filterStr := c.serializeFilters(filters)
		if filterStr != "" {
			key += ":" + filterStr
		}
	}
	
	return key
}

// serializeFilters serializes review filters for cache key
func (c *CacheServiceImpl) serializeFilters(filters *ReviewFilters) string {
	if filters == nil {
		return ""
	}
	
	parts := make([]string, 0)
	
	if filters.MinRating != nil {
		parts = append(parts, fmt.Sprintf("min_rating:%.2f", *filters.MinRating))
	}
	if filters.MaxRating != nil {
		parts = append(parts, fmt.Sprintf("max_rating:%.2f", *filters.MaxRating))
	}
	if filters.Language != nil {
		parts = append(parts, fmt.Sprintf("language:%s", *filters.Language))
	}
	if filters.Sentiment != nil {
		parts = append(parts, fmt.Sprintf("sentiment:%s", *filters.Sentiment))
	}
	if filters.IsVerified != nil {
		parts = append(parts, fmt.Sprintf("verified:%t", *filters.IsVerified))
	}
	if filters.TripType != nil {
		parts = append(parts, fmt.Sprintf("trip_type:%s", *filters.TripType))
	}
	if filters.SortBy != "" {
		parts = append(parts, fmt.Sprintf("sort:%s:%s", filters.SortBy, filters.SortDirection))
	}
	
	sort.Strings(parts)
	return strings.Join(parts, "_")
}

// InvalidateReview invalidates a single review
func (c *CacheServiceImpl) InvalidateReview(ctx context.Context, reviewID uuid.UUID) error {
	key := fmt.Sprintf("%s:review:%s", c.config.ReviewKeyPrefix, reviewID.String())
	
	err := c.redisClient.Delete(ctx, "reviews", key)
	if err != nil {
		return err
	}
	
	c.logInvalidation("review", key, "", "single_review_update")
	return nil
}

// InvalidateReviewsByProvider invalidates all reviews for a provider
func (c *CacheServiceImpl) InvalidateReviewsByProvider(ctx context.Context, providerID uuid.UUID) error {
	pattern := fmt.Sprintf("%s:provider:%s:reviews:*", c.config.ReviewKeyPrefix, providerID.String())
	
	err := c.invalidatePattern(ctx, "provider_reviews", pattern)
	if err != nil {
		return err
	}
	
	c.logInvalidation("provider_reviews", "", pattern, "provider_update")
	return nil
}

// InvalidateReviewsByHotel invalidates all reviews for a hotel
func (c *CacheServiceImpl) InvalidateReviewsByHotel(ctx context.Context, hotelID uuid.UUID) error {
	pattern := fmt.Sprintf("%s:hotel:%s:reviews:*", c.config.ReviewKeyPrefix, hotelID.String())
	
	err := c.invalidatePattern(ctx, "hotel_reviews", pattern)
	if err != nil {
		return err
	}
	
	c.logInvalidation("hotel_reviews", "", pattern, "hotel_update")
	return nil
}

// Hotel Caching Methods

// GetHotel retrieves a hotel from cache
func (c *CacheServiceImpl) GetHotel(ctx context.Context, hotelID uuid.UUID) (*domain.Hotel, error) {
	key := hotelID.String()
	
	hotelData, err := c.redisClient.HGetAll(ctx, "hotels", key)
	if err != nil {
		c.logger.Error("Failed to get hotel from cache", "hotel_id", hotelID, "error", err)
		return nil, err
	}
	
	if len(hotelData) == 0 {
		return nil, nil // Cache miss
	}
	
	return c.deserializeHotel(hotelData)
}

// SetHotel stores a hotel in cache
func (c *CacheServiceImpl) SetHotel(ctx context.Context, hotel *domain.Hotel) error {
	key := hotel.ID.String()
	hotelData := c.serializeHotel(hotel)
	
	return c.redisClient.HSetMap(ctx, "hotels", key, hotelData, c.config.HotelTTL)
}

// GetHotelSummary retrieves hotel summary from cache
func (c *CacheServiceImpl) GetHotelSummary(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	key := fmt.Sprintf("%s:summary:%s", c.config.HotelKeyPrefix, hotelID.String())
	
	cached, err := c.redisClient.Get(ctx, "hotel_summaries", key)
	if err != nil {
		c.logger.Error("Failed to get hotel summary from cache", "hotel_id", hotelID, "error", err)
		return nil, err
	}
	
	if cached == "" {
		return nil, nil // Cache miss
	}
	
	var summary domain.ReviewSummary
	if err := json.Unmarshal([]byte(cached), &summary); err != nil {
		c.logger.Error("Failed to unmarshal cached hotel summary", "hotel_id", hotelID, "error", err)
		return nil, err
	}
	
	return &summary, nil
}

// SetHotelSummary stores hotel summary in cache
func (c *CacheServiceImpl) SetHotelSummary(ctx context.Context, summary *domain.ReviewSummary) error {
	key := fmt.Sprintf("%s:summary:%s", c.config.HotelKeyPrefix, summary.HotelID.String())
	
	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal hotel summary: %w", err)
	}
	
	return c.redisClient.Set(ctx, "hotel_summaries", key, string(data), c.config.HotelTTL)
}

// GetHotelsByCity retrieves hotels by city from cache
func (c *CacheServiceImpl) GetHotelsByCity(ctx context.Context, city string, limit, offset int) ([]*domain.Hotel, error) {
	key := fmt.Sprintf("%s:city:%s:hotels:%d:%d", c.config.HotelKeyPrefix, city, limit, offset)
	
	cached, err := c.redisClient.Get(ctx, "city_hotels", key)
	if err != nil {
		c.logger.Error("Failed to get city hotels from cache", "city", city, "error", err)
		return nil, err
	}
	
	if cached == "" {
		return nil, nil // Cache miss
	}
	
	var hotels []*domain.Hotel
	if err := json.Unmarshal([]byte(cached), &hotels); err != nil {
		c.logger.Error("Failed to unmarshal cached city hotels", "city", city, "error", err)
		return nil, err
	}
	
	return hotels, nil
}

// SetHotelsByCity stores hotels by city in cache
func (c *CacheServiceImpl) SetHotelsByCity(ctx context.Context, city string, hotels []*domain.Hotel, limit, offset int) error {
	key := fmt.Sprintf("%s:city:%s:hotels:%d:%d", c.config.HotelKeyPrefix, city, limit, offset)
	
	data, err := json.Marshal(hotels)
	if err != nil {
		return fmt.Errorf("failed to marshal city hotels: %w", err)
	}
	
	return c.redisClient.Set(ctx, "city_hotels", key, string(data), c.config.HotelTTL)
}

// serializeHotel converts hotel to map for hash storage
func (c *CacheServiceImpl) serializeHotel(hotel *domain.Hotel) map[string]string {
	data := make(map[string]string)
	
	data["id"] = hotel.ID.String()
	data["name"] = hotel.Name
	data["address"] = hotel.Address
	data["city"] = hotel.City
	data["country"] = hotel.Country
	data["postal_code"] = hotel.PostalCode
	data["phone"] = hotel.Phone
	data["email"] = hotel.Email
	data["star_rating"] = strconv.Itoa(hotel.StarRating)
	data["description"] = hotel.Description
	data["latitude"] = strconv.FormatFloat(hotel.Latitude, 'f', 8, 64)
	data["longitude"] = strconv.FormatFloat(hotel.Longitude, 'f', 8, 64)
	data["created_at"] = hotel.CreatedAt.Format(time.RFC3339)
	data["updated_at"] = hotel.UpdatedAt.Format(time.RFC3339)
	
	// Serialize amenities
	if amenities, err := json.Marshal(hotel.Amenities); err == nil {
		data["amenities"] = string(amenities)
	}
	
	return data
}

// deserializeHotel converts map to hotel struct
func (c *CacheServiceImpl) deserializeHotel(data map[string]string) (*domain.Hotel, error) {
	hotel := &domain.Hotel{}
	
	if id, err := uuid.Parse(data["id"]); err == nil {
		hotel.ID = id
	}
	
	hotel.Name = data["name"]
	hotel.Address = data["address"]
	hotel.City = data["city"]
	hotel.Country = data["country"]
	hotel.PostalCode = data["postal_code"]
	hotel.Phone = data["phone"]
	hotel.Email = data["email"]
	hotel.Description = data["description"]
	
	if starRating, err := strconv.Atoi(data["star_rating"]); err == nil {
		hotel.StarRating = starRating
	}
	
	if lat, err := strconv.ParseFloat(data["latitude"], 64); err == nil {
		hotel.Latitude = lat
	}
	
	if lng, err := strconv.ParseFloat(data["longitude"], 64); err == nil {
		hotel.Longitude = lng
	}
	
	if createdAt, err := time.Parse(time.RFC3339, data["created_at"]); err == nil {
		hotel.CreatedAt = createdAt
	}
	
	if updatedAt, err := time.Parse(time.RFC3339, data["updated_at"]); err == nil {
		hotel.UpdatedAt = updatedAt
	}
	
	// Deserialize amenities
	if data["amenities"] != "" {
		var amenities []string
		if err := json.Unmarshal([]byte(data["amenities"]), &amenities); err == nil {
			hotel.Amenities = amenities
		}
	}
	
	return hotel, nil
}

// InvalidateHotel invalidates a single hotel
func (c *CacheServiceImpl) InvalidateHotel(ctx context.Context, hotelID uuid.UUID) error {
	key := hotelID.String()
	
	err := c.redisClient.Delete(ctx, "hotels", key)
	if err != nil {
		return err
	}
	
	// Also invalidate hotel summary
	summaryKey := fmt.Sprintf("%s:summary:%s", c.config.HotelKeyPrefix, hotelID.String())
	c.redisClient.Delete(ctx, "hotel_summaries", summaryKey)
	
	c.logInvalidation("hotel", key, "", "hotel_update")
	return nil
}

// InvalidateHotelsByCity invalidates all hotels for a city
func (c *CacheServiceImpl) InvalidateHotelsByCity(ctx context.Context, city string) error {
	pattern := fmt.Sprintf("%s:city:%s:hotels:*", c.config.HotelKeyPrefix, city)
	
	err := c.invalidatePattern(ctx, "city_hotels", pattern)
	if err != nil {
		return err
	}
	
	c.logInvalidation("city_hotels", "", pattern, "city_update")
	return nil
}

// Processing Status Caching Methods

// GetProcessingStatus retrieves processing status from cache
func (c *CacheServiceImpl) GetProcessingStatus(ctx context.Context, jobID uuid.UUID) (*domain.ReviewProcessingStatus, error) {
	key := jobID.String()
	
	statusData, err := c.redisClient.HGetAll(ctx, "processing_status", key)
	if err != nil {
		c.logger.Error("Failed to get processing status from cache", "job_id", jobID, "error", err)
		return nil, err
	}
	
	if len(statusData) == 0 {
		return nil, nil // Cache miss
	}
	
	return c.deserializeProcessingStatus(statusData)
}

// SetProcessingStatus stores processing status in cache
func (c *CacheServiceImpl) SetProcessingStatus(ctx context.Context, status *domain.ReviewProcessingStatus) error {
	key := status.ID.String()
	statusData := c.serializeProcessingStatus(status)
	
	return c.redisClient.HSetMap(ctx, "processing_status", key, statusData, c.config.ProcessingTTL)
}

// UpdateProcessingProgress updates processing progress
func (c *CacheServiceImpl) UpdateProcessingProgress(ctx context.Context, jobID uuid.UUID, progress ProcessingProgress) error {
	key := jobID.String()
	
	updates := map[string]string{
		"records_processed": strconv.FormatInt(progress.RecordsProcessed, 10),
		"records_total":     strconv.FormatInt(progress.RecordsTotal, 10),
		"records_failed":    strconv.FormatInt(progress.RecordsFailed, 10),
		"processing_rate":   strconv.FormatFloat(progress.ProcessingRate, 'f', 2, 64),
		"estimated_eta":     progress.EstimatedETA.Format(time.RFC3339),
		"last_update":       progress.LastUpdate.Format(time.RFC3339),
	}
	
	for field, value := range updates {
		if err := c.redisClient.HSet(ctx, "processing_status", key, field, value, c.config.ProcessingTTL); err != nil {
			return err
		}
	}
	
	return nil
}

// GetActiveProcessingJobs retrieves active processing jobs
func (c *CacheServiceImpl) GetActiveProcessingJobs(ctx context.Context) ([]*domain.ReviewProcessingStatus, error) {
	key := "active_jobs"
	
	cached, err := c.redisClient.Get(ctx, "active_processing", key)
	if err != nil {
		c.logger.Error("Failed to get active processing jobs from cache", "error", err)
		return nil, err
	}
	
	if cached == "" {
		return nil, nil // Cache miss
	}
	
	var jobs []*domain.ReviewProcessingStatus
	if err := json.Unmarshal([]byte(cached), &jobs); err != nil {
		c.logger.Error("Failed to unmarshal cached active jobs", "error", err)
		return nil, err
	}
	
	return jobs, nil
}

// SetActiveProcessingJobs stores active processing jobs
func (c *CacheServiceImpl) SetActiveProcessingJobs(ctx context.Context, jobs []*domain.ReviewProcessingStatus) error {
	key := "active_jobs"
	
	data, err := json.Marshal(jobs)
	if err != nil {
		return fmt.Errorf("failed to marshal active jobs: %w", err)
	}
	
	return c.redisClient.Set(ctx, "active_processing", key, string(data), c.config.ProcessingTTL)
}

// serializeProcessingStatus converts processing status to map
func (c *CacheServiceImpl) serializeProcessingStatus(status *domain.ReviewProcessingStatus) map[string]string {
	data := make(map[string]string)
	
	data["id"] = status.ID.String()
	data["provider_id"] = status.ProviderID.String()
	data["status"] = status.Status
	data["file_url"] = status.FileURL
	data["records_processed"] = strconv.Itoa(status.RecordsProcessed)
	data["records_total"] = strconv.Itoa(status.RecordsTotal)
	data["error_msg"] = status.ErrorMsg
	data["created_at"] = status.CreatedAt.Format(time.RFC3339)
	data["updated_at"] = status.UpdatedAt.Format(time.RFC3339)
	
	if status.StartedAt != nil {
		data["started_at"] = status.StartedAt.Format(time.RFC3339)
	}
	
	if status.CompletedAt != nil {
		data["completed_at"] = status.CompletedAt.Format(time.RFC3339)
	}
	
	return data
}

// deserializeProcessingStatus converts map to processing status struct
func (c *CacheServiceImpl) deserializeProcessingStatus(data map[string]string) (*domain.ReviewProcessingStatus, error) {
	status := &domain.ReviewProcessingStatus{}
	
	if id, err := uuid.Parse(data["id"]); err == nil {
		status.ID = id
	}
	
	if providerID, err := uuid.Parse(data["provider_id"]); err == nil {
		status.ProviderID = providerID
	}
	
	status.Status = data["status"]
	status.FileURL = data["file_url"]
	status.ErrorMsg = data["error_msg"]
	
	if recordsProcessed, err := strconv.Atoi(data["records_processed"]); err == nil {
		status.RecordsProcessed = recordsProcessed
	}
	
	if recordsTotal, err := strconv.Atoi(data["records_total"]); err == nil {
		status.RecordsTotal = recordsTotal
	}
	
	if createdAt, err := time.Parse(time.RFC3339, data["created_at"]); err == nil {
		status.CreatedAt = createdAt
	}
	
	if updatedAt, err := time.Parse(time.RFC3339, data["updated_at"]); err == nil {
		status.UpdatedAt = updatedAt
	}
	
	if data["started_at"] != "" {
		if startedAt, err := time.Parse(time.RFC3339, data["started_at"]); err == nil {
			status.StartedAt = &startedAt
		}
	}
	
	if data["completed_at"] != "" {
		if completedAt, err := time.Parse(time.RFC3339, data["completed_at"]); err == nil {
			status.CompletedAt = &completedAt
		}
	}
	
	return status, nil
}

// InvalidateProcessingStatus invalidates processing status
func (c *CacheServiceImpl) InvalidateProcessingStatus(ctx context.Context, jobID uuid.UUID) error {
	key := jobID.String()
	
	err := c.redisClient.Delete(ctx, "processing_status", key)
	if err != nil {
		return err
	}
	
	// Also invalidate active jobs list
	c.redisClient.Delete(ctx, "active_processing", "active_jobs")
	
	c.logInvalidation("processing_status", key, "", "processing_update")
	return nil
}

// Analytics Caching Methods

// GetAnalytics retrieves analytics data from cache
func (c *CacheServiceImpl) GetAnalytics(ctx context.Context, key string) (*AnalyticsData, error) {
	cacheKey := fmt.Sprintf("%s:analytics:%s", c.config.AnalyticsKeyPrefix, key)
	
	cached, err := c.redisClient.Get(ctx, "analytics", cacheKey)
	if err != nil {
		c.logger.Error("Failed to get analytics from cache", "key", key, "error", err)
		return nil, err
	}
	
	if cached == "" {
		return nil, nil // Cache miss
	}
	
	var analytics AnalyticsData
	if err := json.Unmarshal([]byte(cached), &analytics); err != nil {
		c.logger.Error("Failed to unmarshal cached analytics", "key", key, "error", err)
		return nil, err
	}
	
	return &analytics, nil
}

// SetAnalytics stores analytics data in cache
func (c *CacheServiceImpl) SetAnalytics(ctx context.Context, key string, analytics *AnalyticsData) error {
	cacheKey := fmt.Sprintf("%s:analytics:%s", c.config.AnalyticsKeyPrefix, key)
	
	data, err := json.Marshal(analytics)
	if err != nil {
		return fmt.Errorf("failed to marshal analytics: %w", err)
	}
	
	return c.redisClient.Set(ctx, "analytics", cacheKey, string(data), c.config.AnalyticsTTL)
}

// InvalidateAnalytics invalidates analytics data by pattern
func (c *CacheServiceImpl) InvalidateAnalytics(ctx context.Context, pattern string) error {
	fullPattern := fmt.Sprintf("%s:analytics:%s", c.config.AnalyticsKeyPrefix, pattern)
	
	err := c.invalidatePattern(ctx, "analytics", fullPattern)
	if err != nil {
		return err
	}
	
	c.logInvalidation("analytics", "", fullPattern, "analytics_update")
	return nil
}

// Cache Warming Methods

// WarmupReviews warms up review cache
func (c *CacheServiceImpl) WarmupReviews(ctx context.Context, options *WarmupOptions) error {
	c.logger.Info("Starting review cache warmup", "options", options)
	
	// Implementation would fetch popular reviews and cache them
	// This is a placeholder for the actual implementation
	task := WarmupTask{
		Type:     "reviews",
		Priority: 1,
		Options:  options,
	}
	
	select {
	case c.warmupQueue <- task:
		return nil
	default:
		return fmt.Errorf("warmup queue is full")
	}
}

// WarmupHotels warms up hotel cache
func (c *CacheServiceImpl) WarmupHotels(ctx context.Context, options *WarmupOptions) error {
	c.logger.Info("Starting hotel cache warmup", "options", options)
	
	task := WarmupTask{
		Type:     "hotels",
		Priority: 1,
		Options:  options,
	}
	
	select {
	case c.warmupQueue <- task:
		return nil
	default:
		return fmt.Errorf("warmup queue is full")
	}
}

// WarmupProcessingStatus warms up processing status cache
func (c *CacheServiceImpl) WarmupProcessingStatus(ctx context.Context, options *WarmupOptions) error {
	c.logger.Info("Starting processing status cache warmup", "options", options)
	
	task := WarmupTask{
		Type:     "processing_status",
		Priority: 1,
		Options:  options,
	}
	
	select {
	case c.warmupQueue <- task:
		return nil
	default:
		return fmt.Errorf("warmup queue is full")
	}
}

// WarmupAnalytics warms up analytics cache
func (c *CacheServiceImpl) WarmupAnalytics(ctx context.Context, options *WarmupOptions) error {
	c.logger.Info("Starting analytics cache warmup", "options", options)
	
	task := WarmupTask{
		Type:     "analytics",
		Priority: 1,
		Options:  options,
	}
	
	select {
	case c.warmupQueue <- task:
		return nil
	default:
		return fmt.Errorf("warmup queue is full")
	}
}

// warmupWorker processes warmup tasks
func (c *CacheServiceImpl) warmupWorker() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case task := <-c.warmupQueue:
			c.processWarmupTask(task)
		}
	}
}

// processWarmupTask processes a single warmup task
func (c *CacheServiceImpl) processWarmupTask(task WarmupTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	c.logger.Info("Processing warmup task", "type", task.Type, "priority", task.Priority)
	
	switch task.Type {
	case "reviews":
		c.warmupReviewsData(ctx, task.Options)
	case "hotels":
		c.warmupHotelsData(ctx, task.Options)
	case "processing_status":
		c.warmupProcessingStatusData(ctx, task.Options)
	case "analytics":
		c.warmupAnalyticsData(ctx, task.Options)
	default:
		c.logger.Warn("Unknown warmup task type", "type", task.Type)
	}
}

// warmupReviewsData warms up reviews data
func (c *CacheServiceImpl) warmupReviewsData(ctx context.Context, options *WarmupOptions) {
	// Implementation would fetch and cache popular reviews
	// This is a placeholder for the actual implementation
	c.logger.Info("Warming up reviews data", "options", options)
}

// warmupHotelsData warms up hotels data
func (c *CacheServiceImpl) warmupHotelsData(ctx context.Context, options *WarmupOptions) {
	// Implementation would fetch and cache popular hotels
	// This is a placeholder for the actual implementation
	c.logger.Info("Warming up hotels data", "options", options)
}

// warmupProcessingStatusData warms up processing status data
func (c *CacheServiceImpl) warmupProcessingStatusData(ctx context.Context, options *WarmupOptions) {
	// Implementation would fetch and cache active processing jobs
	// This is a placeholder for the actual implementation
	c.logger.Info("Warming up processing status data", "options", options)
}

// warmupAnalyticsData warms up analytics data
func (c *CacheServiceImpl) warmupAnalyticsData(ctx context.Context, options *WarmupOptions) {
	// Implementation would fetch and cache frequently accessed analytics
	// This is a placeholder for the actual implementation
	c.logger.Info("Warming up analytics data", "options", options)
}

// Cache Management Methods

// GetCacheStats returns cache statistics
func (c *CacheServiceImpl) GetCacheStats(ctx context.Context) (*CacheStats, error) {
	metrics := c.redisClient.GetMetrics()
	
	stats := &CacheStats{
		HitRate:         metrics.HitRate,
		MissRate:        100 - metrics.HitRate,
		TotalOperations: metrics.TotalHits + metrics.TotalMisses + metrics.TotalSets + metrics.TotalDeletes,
		KeyspaceStats:   metrics.KeyspaceCounts,
		LastUpdated:     metrics.LastUpdated,
	}
	
	return stats, nil
}

// ClearCache clears cache by pattern
func (c *CacheServiceImpl) ClearCache(ctx context.Context, pattern string) error {
	return c.invalidatePattern(ctx, "all", pattern)
}

// HealthCheck performs cache health check
func (c *CacheServiceImpl) HealthCheck(ctx context.Context) error {
	return c.redisClient.HealthCheck(ctx)
}

// Helper Methods

// invalidatePattern invalidates keys matching a pattern
func (c *CacheServiceImpl) invalidatePattern(ctx context.Context, keyspace, pattern string) error {
	// This would use Redis SCAN to find matching keys and delete them
	// Implementation would be similar to the one in redis.go
	return nil
}

// logInvalidation logs cache invalidation events
func (c *CacheServiceImpl) logInvalidation(eventType, key, pattern, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	event := InvalidationEvent{
		Type:      eventType,
		Key:       key,
		Pattern:   pattern,
		Timestamp: time.Now(),
		Reason:    reason,
	}
	
	c.invalidationLog = append(c.invalidationLog, event)
	
	// Keep only last 1000 events
	if len(c.invalidationLog) > 1000 {
		c.invalidationLog = c.invalidationLog[1:]
	}
	
	c.logger.Info("Cache invalidation",
		"type", eventType,
		"key", key,
		"pattern", pattern,
		"reason", reason,
	)
}