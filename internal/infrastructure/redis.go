package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host               string        `mapstructure:"host" json:"host"`
	Port               int           `mapstructure:"port" json:"port"`
	Password           string        `mapstructure:"password" json:"password"`
	Database           int           `mapstructure:"database" json:"database"`
	PoolSize           int           `mapstructure:"pool_size" json:"pool_size"`
	MinIdleConns       int           `mapstructure:"min_idle_conns" json:"min_idle_conns"`
	MaxConnAge         time.Duration `mapstructure:"max_conn_age" json:"max_conn_age"`
	PoolTimeout        time.Duration `mapstructure:"pool_timeout" json:"pool_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout" json:"idle_timeout"`
	IdleCheckFrequency time.Duration `mapstructure:"idle_check_frequency" json:"idle_check_frequency"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout" json:"write_timeout"`
	DialTimeout        time.Duration `mapstructure:"dial_timeout" json:"dial_timeout"`
	MaxRetries         int           `mapstructure:"max_retries" json:"max_retries"`
	MinRetryBackoff    time.Duration `mapstructure:"min_retry_backoff" json:"min_retry_backoff"`
	MaxRetryBackoff    time.Duration `mapstructure:"max_retry_backoff" json:"max_retry_backoff"`
	EnableMetrics      bool          `mapstructure:"enable_metrics" json:"enable_metrics"`
	KeyPrefix          string        `mapstructure:"key_prefix" json:"key_prefix"`
}

// CacheConfig holds cache-specific configuration
type CacheConfig struct {
	ReviewTTL          time.Duration `mapstructure:"review_ttl" json:"review_ttl"`
	HotelTTL           time.Duration `mapstructure:"hotel_ttl" json:"hotel_ttl"`
	ProcessingTTL      time.Duration `mapstructure:"processing_ttl" json:"processing_ttl"`
	AnalyticsTTL       time.Duration `mapstructure:"analytics_ttl" json:"analytics_ttl"`
	DefaultTTL         time.Duration `mapstructure:"default_ttl" json:"default_ttl"`
	MaxKeyLength       int           `mapstructure:"max_key_length" json:"max_key_length"`
	EnableCompression  bool          `mapstructure:"enable_compression" json:"enable_compression"`
	CompressionLevel   int           `mapstructure:"compression_level" json:"compression_level"`
	InvalidationBatch  int           `mapstructure:"invalidation_batch" json:"invalidation_batch"`
	WarmupConcurrency  int           `mapstructure:"warmup_concurrency" json:"warmup_concurrency"`
}

// CacheOperation represents different cache operations
type CacheOperation string

const (
	CacheOperationGet    CacheOperation = "get"
	CacheOperationSet    CacheOperation = "set"
	CacheOperationDelete CacheOperation = "delete"
	CacheOperationExists CacheOperation = "exists"
	CacheOperationExpire CacheOperation = "expire"
	CacheOperationHGet   CacheOperation = "hget"
	CacheOperationHSet   CacheOperation = "hset"
	CacheOperationHDel   CacheOperation = "hdel"
	CacheOperationHKeys  CacheOperation = "hkeys"
	CacheOperationHVals  CacheOperation = "hvals"
	CacheOperationHGetAll CacheOperation = "hgetall"
)

// CacheMetrics holds cache operation metrics
type CacheMetrics struct {
	TotalHits           int64                       `json:"total_hits"`
	TotalMisses         int64                       `json:"total_misses"`
	TotalSets           int64                       `json:"total_sets"`
	TotalDeletes        int64                       `json:"total_deletes"`
	TotalErrors         int64                       `json:"total_errors"`
	HitRate             float64                     `json:"hit_rate"`
	AverageLatency      time.Duration               `json:"average_latency"`
	OperationCounts     map[CacheOperation]int64    `json:"operation_counts"`
	KeyspaceCounts      map[string]int64            `json:"keyspace_counts"`
	LastUpdated         time.Time                   `json:"last_updated"`
	ConnectionPoolStats redis.PoolStats             `json:"connection_pool_stats"`
}

// RedisClient wraps Redis client with additional functionality
type RedisClient struct {
	client        redis.UniversalClient
	config        *RedisConfig
	cacheConfig   *CacheConfig
	logger        *logger.Logger
	metrics       *CacheMetrics
	mu            sync.RWMutex
	invalidations chan InvalidationRequest
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// InvalidationRequest represents a cache invalidation request
type InvalidationRequest struct {
	Pattern   string
	Keys      []string
	Immediate bool
	Callback  func(int, error)
}

// NewRedisClient creates a new Redis client with connection pooling
func NewRedisClient(redisConfig *RedisConfig, cacheConfig *CacheConfig, logger *logger.Logger) (*RedisClient, error) {
	// Create Redis client options
	opts := &redis.Options{
		Addr:               fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password:           redisConfig.Password,
		DB:                 redisConfig.Database,
		PoolSize:           redisConfig.PoolSize,
		MinIdleConns:       redisConfig.MinIdleConns,
		MaxConnAge:         redisConfig.MaxConnAge,
		PoolTimeout:        redisConfig.PoolTimeout,
		IdleTimeout:        redisConfig.IdleTimeout,
		IdleCheckFrequency: redisConfig.IdleCheckFrequency,
		ReadTimeout:        redisConfig.ReadTimeout,
		WriteTimeout:       redisConfig.WriteTimeout,
		DialTimeout:        redisConfig.DialTimeout,
		MaxRetries:         redisConfig.MaxRetries,
		MinRetryBackoff:    redisConfig.MinRetryBackoff,
		MaxRetryBackoff:    redisConfig.MaxRetryBackoff,
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create context for background operations
	bgCtx, bgCancel := context.WithCancel(context.Background())

	redisClient := &RedisClient{
		client:        client,
		config:        redisConfig,
		cacheConfig:   cacheConfig,
		logger:        logger,
		metrics:       &CacheMetrics{
			OperationCounts: make(map[CacheOperation]int64),
			KeyspaceCounts:  make(map[string]int64),
		},
		invalidations: make(chan InvalidationRequest, 1000),
		ctx:           bgCtx,
		cancel:        bgCancel,
	}

	// Start background processes
	redisClient.wg.Add(1)
	go redisClient.invalidationWorker()

	if redisConfig.EnableMetrics {
		redisClient.wg.Add(1)
		go redisClient.metricsCollector()
	}

	logger.Info("Redis client initialized successfully",
		"host", redisConfig.Host,
		"port", redisConfig.Port,
		"database", redisConfig.Database,
		"pool_size", redisConfig.PoolSize,
	)

	return redisClient, nil
}

// Close closes the Redis client and background processes
func (r *RedisClient) Close() error {
	r.logger.Info("Closing Redis client...")

	// Cancel background processes
	r.cancel()

	// Wait for background processes to finish
	r.wg.Wait()

	// Close Redis client
	if err := r.client.Close(); err != nil {
		r.logger.Error("Error closing Redis client", "error", err)
		return err
	}

	r.logger.Info("Redis client closed successfully")
	return nil
}

// buildKey builds a cache key with prefix and namespace
func (r *RedisClient) buildKey(keyspace, key string) string {
	if r.config.KeyPrefix != "" {
		return fmt.Sprintf("%s:%s:%s", r.config.KeyPrefix, keyspace, key)
	}
	return fmt.Sprintf("%s:%s", keyspace, key)
}

// hashKey creates a hash of the key if it exceeds max length
func (r *RedisClient) hashKey(key string) string {
	if len(key) <= r.cacheConfig.MaxKeyLength {
		return key
	}

	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// recordMetric records a cache operation metric
func (r *RedisClient) recordMetric(operation CacheOperation, keyspace string, hit bool, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.metrics.OperationCounts[operation]++
	r.metrics.KeyspaceCounts[keyspace]++

	if operation == CacheOperationGet {
		if hit {
			r.metrics.TotalHits++
		} else {
			r.metrics.TotalMisses++
		}
	} else if operation == CacheOperationSet {
		r.metrics.TotalSets++
	} else if operation == CacheOperationDelete {
		r.metrics.TotalDeletes++
	}

	// Update hit rate
	total := r.metrics.TotalHits + r.metrics.TotalMisses
	if total > 0 {
		r.metrics.HitRate = float64(r.metrics.TotalHits) / float64(total) * 100
	}

	// Update average latency (simple moving average)
	if r.metrics.AverageLatency == 0 {
		r.metrics.AverageLatency = latency
	} else {
		r.metrics.AverageLatency = (r.metrics.AverageLatency + latency) / 2
	}

	r.metrics.LastUpdated = time.Now()
}

// recordError records a cache operation error
func (r *RedisClient) recordError(operation CacheOperation, keyspace string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.metrics.TotalErrors++
	r.logger.Error("Cache operation failed",
		"operation", operation,
		"keyspace", keyspace,
		"error", err,
	)
}

// String Cache Operations

// Get retrieves a value from cache
func (r *RedisClient) Get(ctx context.Context, keyspace, key string) (string, error) {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	val, err := r.client.Get(ctx, cacheKey).Result()
	latency := time.Since(startTime)

	if err == redis.Nil {
		r.recordMetric(CacheOperationGet, keyspace, false, latency)
		return "", nil
	} else if err != nil {
		r.recordError(CacheOperationGet, keyspace, err)
		return "", err
	}

	r.recordMetric(CacheOperationGet, keyspace, true, latency)
	return val, nil
}

// Set stores a value in cache with TTL
func (r *RedisClient) Set(ctx context.Context, keyspace, key, value string, ttl time.Duration) error {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	if ttl == 0 {
		ttl = r.cacheConfig.DefaultTTL
	}

	err := r.client.Set(ctx, cacheKey, value, ttl).Err()
	latency := time.Since(startTime)

	if err != nil {
		r.recordError(CacheOperationSet, keyspace, err)
		return err
	}

	r.recordMetric(CacheOperationSet, keyspace, false, latency)
	return nil
}

// Delete removes a key from cache
func (r *RedisClient) Delete(ctx context.Context, keyspace, key string) error {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	err := r.client.Del(ctx, cacheKey).Err()
	latency := time.Since(startTime)

	if err != nil {
		r.recordError(CacheOperationDelete, keyspace, err)
		return err
	}

	r.recordMetric(CacheOperationDelete, keyspace, false, latency)
	return nil
}

// Exists checks if a key exists in cache
func (r *RedisClient) Exists(ctx context.Context, keyspace, key string) (bool, error) {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	count, err := r.client.Exists(ctx, cacheKey).Result()
	latency := time.Since(startTime)

	if err != nil {
		r.recordError(CacheOperationExists, keyspace, err)
		return false, err
	}

	r.recordMetric(CacheOperationExists, keyspace, count > 0, latency)
	return count > 0, nil
}

// Expire sets a TTL on a key
func (r *RedisClient) Expire(ctx context.Context, keyspace, key string, ttl time.Duration) error {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	err := r.client.Expire(ctx, cacheKey, ttl).Err()
	latency := time.Since(startTime)

	if err != nil {
		r.recordError(CacheOperationExpire, keyspace, err)
		return err
	}

	r.recordMetric(CacheOperationExpire, keyspace, false, latency)
	return nil
}

// Hash Cache Operations

// HGet retrieves a field from a hash
func (r *RedisClient) HGet(ctx context.Context, keyspace, key, field string) (string, error) {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	val, err := r.client.HGet(ctx, cacheKey, field).Result()
	latency := time.Since(startTime)

	if err == redis.Nil {
		r.recordMetric(CacheOperationHGet, keyspace, false, latency)
		return "", nil
	} else if err != nil {
		r.recordError(CacheOperationHGet, keyspace, err)
		return "", err
	}

	r.recordMetric(CacheOperationHGet, keyspace, true, latency)
	return val, nil
}

// HSet sets a field in a hash
func (r *RedisClient) HSet(ctx context.Context, keyspace, key, field, value string, ttl time.Duration) error {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	err := r.client.HSet(ctx, cacheKey, field, value).Err()
	if err != nil {
		r.recordError(CacheOperationHSet, keyspace, err)
		return err
	}

	// Set TTL if specified
	if ttl > 0 {
		err = r.client.Expire(ctx, cacheKey, ttl).Err()
		if err != nil {
			r.recordError(CacheOperationHSet, keyspace, err)
			return err
		}
	}

	latency := time.Since(startTime)
	r.recordMetric(CacheOperationHSet, keyspace, false, latency)
	return nil
}

// HGetAll retrieves all fields from a hash
func (r *RedisClient) HGetAll(ctx context.Context, keyspace, key string) (map[string]string, error) {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	vals, err := r.client.HGetAll(ctx, cacheKey).Result()
	latency := time.Since(startTime)

	if err != nil {
		r.recordError(CacheOperationHGetAll, keyspace, err)
		return nil, err
	}

	r.recordMetric(CacheOperationHGetAll, keyspace, len(vals) > 0, latency)
	return vals, nil
}

// HSetMap sets multiple fields in a hash
func (r *RedisClient) HSetMap(ctx context.Context, keyspace, key string, fields map[string]string, ttl time.Duration) error {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	err := r.client.HMSet(ctx, cacheKey, fields).Err()
	if err != nil {
		r.recordError(CacheOperationHSet, keyspace, err)
		return err
	}

	// Set TTL if specified
	if ttl > 0 {
		err = r.client.Expire(ctx, cacheKey, ttl).Err()
		if err != nil {
			r.recordError(CacheOperationHSet, keyspace, err)
			return err
		}
	}

	latency := time.Since(startTime)
	r.recordMetric(CacheOperationHSet, keyspace, false, latency)
	return nil
}

// HDel removes fields from a hash
func (r *RedisClient) HDel(ctx context.Context, keyspace, key string, fields ...string) error {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	err := r.client.HDel(ctx, cacheKey, fields...).Err()
	latency := time.Since(startTime)

	if err != nil {
		r.recordError(CacheOperationHDel, keyspace, err)
		return err
	}

	r.recordMetric(CacheOperationHDel, keyspace, false, latency)
	return nil
}

// HKeys retrieves all field names from a hash
func (r *RedisClient) HKeys(ctx context.Context, keyspace, key string) ([]string, error) {
	startTime := time.Now()
	cacheKey := r.buildKey(keyspace, r.hashKey(key))

	keys, err := r.client.HKeys(ctx, cacheKey).Result()
	latency := time.Since(startTime)

	if err != nil {
		r.recordError(CacheOperationHKeys, keyspace, err)
		return nil, err
	}

	r.recordMetric(CacheOperationHKeys, keyspace, len(keys) > 0, latency)
	return keys, nil
}

// Cache-Aside Pattern Implementations

// ReviewCache implements cache-aside pattern for review data
type ReviewCache struct {
	client        *RedisClient
	reviewService domain.ReviewService
	logger        *logger.Logger
}

// NewReviewCache creates a new review cache
func NewReviewCache(client *RedisClient, reviewService domain.ReviewService, logger *logger.Logger) *ReviewCache {
	return &ReviewCache{
		client:        client,
		reviewService: reviewService,
		logger:        logger,
	}
}

// GetReview retrieves a review using cache-aside pattern
func (c *ReviewCache) GetReview(ctx context.Context, reviewID uuid.UUID) (*domain.Review, error) {
	key := reviewID.String()
	keyspace := "reviews"

	// Try to get from cache first
	cached, err := c.client.Get(ctx, keyspace, key)
	if err != nil {
		c.logger.Error("Failed to get review from cache", "review_id", reviewID, "error", err)
		// Fall through to database
	}

	if cached != "" {
		var review domain.Review
		if err := json.Unmarshal([]byte(cached), &review); err == nil {
			return &review, nil
		}
		c.logger.Error("Failed to unmarshal cached review", "review_id", reviewID, "error", err)
	}

	// Get from database
	review, err := c.reviewService.GetByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if reviewData, err := json.Marshal(review); err == nil {
		if err := c.client.Set(ctx, keyspace, key, string(reviewData), c.client.cacheConfig.ReviewTTL); err != nil {
			c.logger.Error("Failed to cache review", "review_id", reviewID, "error", err)
		}
	}

	return review, nil
}

// GetReviewsByHotel retrieves reviews for a hotel using cache-aside pattern
func (c *ReviewCache) GetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	key := fmt.Sprintf("hotel:%s:reviews:%d:%d", hotelID.String(), limit, offset)
	keyspace := "hotel_reviews"

	// Try to get from cache first
	cached, err := c.client.Get(ctx, keyspace, key)
	if err != nil {
		c.logger.Error("Failed to get hotel reviews from cache", "hotel_id", hotelID, "error", err)
		// Fall through to database
	}

	if cached != "" {
		var reviews []*domain.Review
		if err := json.Unmarshal([]byte(cached), &reviews); err == nil {
			return reviews, nil
		}
		c.logger.Error("Failed to unmarshal cached hotel reviews", "hotel_id", hotelID, "error", err)
	}

	// Get from database
	reviews, err := c.reviewService.GetByHotelID(ctx, hotelID, limit, offset)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if reviewsData, err := json.Marshal(reviews); err == nil {
		if err := c.client.Set(ctx, keyspace, key, string(reviewsData), c.client.cacheConfig.ReviewTTL); err != nil {
			c.logger.Error("Failed to cache hotel reviews", "hotel_id", hotelID, "error", err)
		}
	}

	return reviews, nil
}

// InvalidateReview removes a review from cache
func (c *ReviewCache) InvalidateReview(ctx context.Context, reviewID uuid.UUID) error {
	key := reviewID.String()
	keyspace := "reviews"

	return c.client.Delete(ctx, keyspace, key)
}

// InvalidateHotelReviews removes all cached reviews for a hotel
func (c *ReviewCache) InvalidateHotelReviews(ctx context.Context, hotelID uuid.UUID) error {
	pattern := fmt.Sprintf("hotel:%s:reviews:*", hotelID.String())
	
	req := InvalidationRequest{
		Pattern:   pattern,
		Immediate: false,
		Callback: func(count int, err error) {
			if err != nil {
				c.logger.Error("Failed to invalidate hotel reviews", "hotel_id", hotelID, "error", err)
			} else {
				c.logger.Info("Invalidated hotel reviews", "hotel_id", hotelID, "count", count)
			}
		},
	}

	select {
	case c.client.invalidations <- req:
		return nil
	default:
		return fmt.Errorf("invalidation queue full")
	}
}

// HotelCache implements cache-aside pattern for hotel data
type HotelCache struct {
	client  *RedisClient
	logger  *logger.Logger
}

// NewHotelCache creates a new hotel cache
func NewHotelCache(client *RedisClient, logger *logger.Logger) *HotelCache {
	return &HotelCache{
		client: client,
		logger: logger,
	}
}

// GetHotel retrieves a hotel using cache-aside pattern
func (c *HotelCache) GetHotel(ctx context.Context, hotelID uuid.UUID) (map[string]string, error) {
	key := hotelID.String()
	keyspace := "hotels"

	return c.client.HGetAll(ctx, keyspace, key)
}

// SetHotel stores hotel data in cache
func (c *HotelCache) SetHotel(ctx context.Context, hotelID uuid.UUID, hotel map[string]string) error {
	key := hotelID.String()
	keyspace := "hotels"

	return c.client.HSetMap(ctx, keyspace, key, hotel, c.client.cacheConfig.HotelTTL)
}

// UpdateHotelField updates a specific field in hotel cache
func (c *HotelCache) UpdateHotelField(ctx context.Context, hotelID uuid.UUID, field, value string) error {
	key := hotelID.String()
	keyspace := "hotels"

	return c.client.HSet(ctx, keyspace, key, field, value, c.client.cacheConfig.HotelTTL)
}

// GetHotelField retrieves a specific field from hotel cache
func (c *HotelCache) GetHotelField(ctx context.Context, hotelID uuid.UUID, field string) (string, error) {
	key := hotelID.String()
	keyspace := "hotels"

	return c.client.HGet(ctx, keyspace, key, field)
}

// InvalidateHotel removes a hotel from cache
func (c *HotelCache) InvalidateHotel(ctx context.Context, hotelID uuid.UUID) error {
	key := hotelID.String()
	keyspace := "hotels"

	return c.client.Delete(ctx, keyspace, key)
}

// ProcessingStatusCache implements cache for processing status
type ProcessingStatusCache struct {
	client *RedisClient
	logger *logger.Logger
}

// NewProcessingStatusCache creates a new processing status cache
func NewProcessingStatusCache(client *RedisClient, logger *logger.Logger) *ProcessingStatusCache {
	return &ProcessingStatusCache{
		client: client,
		logger: logger,
	}
}

// GetProcessingStatus retrieves processing status
func (c *ProcessingStatusCache) GetProcessingStatus(ctx context.Context, jobID uuid.UUID) (map[string]string, error) {
	key := jobID.String()
	keyspace := "processing_status"

	return c.client.HGetAll(ctx, keyspace, key)
}

// SetProcessingStatus stores processing status
func (c *ProcessingStatusCache) SetProcessingStatus(ctx context.Context, jobID uuid.UUID, status map[string]string) error {
	key := jobID.String()
	keyspace := "processing_status"

	return c.client.HSetMap(ctx, keyspace, key, status, c.client.cacheConfig.ProcessingTTL)
}

// UpdateProcessingProgress updates processing progress
func (c *ProcessingStatusCache) UpdateProcessingProgress(ctx context.Context, jobID uuid.UUID, progress float64) error {
	key := jobID.String()
	keyspace := "processing_status"

	return c.client.HSet(ctx, keyspace, key, "progress", fmt.Sprintf("%.2f", progress), c.client.cacheConfig.ProcessingTTL)
}

// UpdateProcessingStatus updates processing status
func (c *ProcessingStatusCache) UpdateProcessingStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	key := jobID.String()
	keyspace := "processing_status"

	return c.client.HSet(ctx, keyspace, key, "status", status, c.client.cacheConfig.ProcessingTTL)
}

// InvalidateProcessingStatus removes processing status from cache
func (c *ProcessingStatusCache) InvalidateProcessingStatus(ctx context.Context, jobID uuid.UUID) error {
	key := jobID.String()
	keyspace := "processing_status"

	return c.client.Delete(ctx, keyspace, key)
}

// Cache Invalidation Worker

// invalidationWorker processes cache invalidation requests
func (r *RedisClient) invalidationWorker() {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.invalidations:
			r.processInvalidationRequest(req)
		}
	}
}

// processInvalidationRequest processes a single invalidation request
func (r *RedisClient) processInvalidationRequest(req InvalidationRequest) {
	if req.Pattern != "" {
		// Pattern-based invalidation
		count, err := r.invalidateByPattern(req.Pattern)
		if req.Callback != nil {
			req.Callback(count, err)
		}
	} else if len(req.Keys) > 0 {
		// Key-based invalidation
		count, err := r.invalidateByKeys(req.Keys)
		if req.Callback != nil {
			req.Callback(count, err)
		}
	}
}

// invalidateByPattern invalidates keys matching a pattern
func (r *RedisClient) invalidateByPattern(pattern string) (int, error) {
	ctx := context.Background()
	
	// Get all keys matching the pattern
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, err
	}

	if len(keys) == 0 {
		return 0, nil
	}

	// Delete keys in batches
	batchSize := r.cacheConfig.InvalidationBatch
	if batchSize == 0 {
		batchSize = 100
	}

	totalDeleted := 0
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		deleted, err := r.client.Del(ctx, batch...).Result()
		if err != nil {
			r.logger.Error("Failed to delete batch of keys", "error", err, "batch_size", len(batch))
			continue
		}

		totalDeleted += int(deleted)
	}

	return totalDeleted, nil
}

// invalidateByKeys invalidates specific keys
func (r *RedisClient) invalidateByKeys(keys []string) (int, error) {
	ctx := context.Background()
	
	deleted, err := r.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, err
	}

	return int(deleted), nil
}

// Metrics Collection

// metricsCollector collects cache metrics periodically
func (r *RedisClient) metricsCollector() {
	defer r.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.collectMetrics()
		}
	}
}

// collectMetrics collects and logs cache metrics
func (r *RedisClient) collectMetrics() {
	// Get connection pool stats
	poolStats := r.client.PoolStats()
	
	r.mu.Lock()
	r.metrics.ConnectionPoolStats = *poolStats
	r.mu.Unlock()

	r.logger.Info("Cache metrics",
		"total_hits", r.metrics.TotalHits,
		"total_misses", r.metrics.TotalMisses,
		"hit_rate", fmt.Sprintf("%.2f%%", r.metrics.HitRate),
		"total_sets", r.metrics.TotalSets,
		"total_deletes", r.metrics.TotalDeletes,
		"total_errors", r.metrics.TotalErrors,
		"average_latency", r.metrics.AverageLatency,
		"pool_hits", poolStats.Hits,
		"pool_misses", poolStats.Misses,
		"pool_timeouts", poolStats.Timeouts,
		"pool_total_conns", poolStats.TotalConns,
		"pool_idle_conns", poolStats.IdleConns,
		"pool_stale_conns", poolStats.StaleConns,
	)
}

// GetMetrics returns current cache metrics
func (r *RedisClient) GetMetrics() *CacheMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &CacheMetrics{
		TotalHits:           r.metrics.TotalHits,
		TotalMisses:         r.metrics.TotalMisses,
		TotalSets:           r.metrics.TotalSets,
		TotalDeletes:        r.metrics.TotalDeletes,
		TotalErrors:         r.metrics.TotalErrors,
		HitRate:             r.metrics.HitRate,
		AverageLatency:      r.metrics.AverageLatency,
		OperationCounts:     make(map[CacheOperation]int64),
		KeyspaceCounts:      make(map[string]int64),
		LastUpdated:         r.metrics.LastUpdated,
		ConnectionPoolStats: r.metrics.ConnectionPoolStats,
	}

	// Copy maps
	for k, v := range r.metrics.OperationCounts {
		metrics.OperationCounts[k] = v
	}
	for k, v := range r.metrics.KeyspaceCounts {
		metrics.KeyspaceCounts[k] = v
	}

	return metrics
}

// Cache Warming

// WarmupCache warms up the cache with frequently accessed data
func (r *RedisClient) WarmupCache(ctx context.Context, warmupData map[string]interface{}) error {
	r.logger.Info("Starting cache warmup", "data_sets", len(warmupData))

	// Use semaphore to limit concurrent operations
	sem := make(chan struct{}, r.cacheConfig.WarmupConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for keyspace, data := range warmupData {
		wg.Add(1)
		go func(ks string, d interface{}) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := r.warmupKeyspace(ctx, ks, d); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("warmup failed for keyspace %s: %w", ks, err))
				mu.Unlock()
			}
		}(keyspace, data)
	}

	wg.Wait()

	if len(errors) > 0 {
		r.logger.Error("Cache warmup completed with errors", "errors", errors)
		return fmt.Errorf("cache warmup failed: %v", errors)
	}

	r.logger.Info("Cache warmup completed successfully")
	return nil
}

// warmupKeyspace warms up a specific keyspace
func (r *RedisClient) warmupKeyspace(ctx context.Context, keyspace string, data interface{}) error {
	switch keyspace {
	case "hotels":
		return r.warmupHotels(ctx, data)
	case "reviews":
		return r.warmupReviews(ctx, data)
	case "processing_status":
		return r.warmupProcessingStatus(ctx, data)
	default:
		return fmt.Errorf("unknown keyspace: %s", keyspace)
	}
}

// warmupHotels warms up hotel data
func (r *RedisClient) warmupHotels(ctx context.Context, data interface{}) error {
	hotels, ok := data.(map[string]map[string]string)
	if !ok {
		return fmt.Errorf("invalid hotel data format")
	}

	for hotelID, hotelData := range hotels {
		if err := r.HSetMap(ctx, "hotels", hotelID, hotelData, r.cacheConfig.HotelTTL); err != nil {
			return err
		}
	}

	return nil
}

// warmupReviews warms up review data
func (r *RedisClient) warmupReviews(ctx context.Context, data interface{}) error {
	reviews, ok := data.(map[string]string)
	if !ok {
		return fmt.Errorf("invalid review data format")
	}

	for reviewID, reviewData := range reviews {
		if err := r.Set(ctx, "reviews", reviewID, reviewData, r.cacheConfig.ReviewTTL); err != nil {
			return err
		}
	}

	return nil
}

// warmupProcessingStatus warms up processing status data
func (r *RedisClient) warmupProcessingStatus(ctx context.Context, data interface{}) error {
	statuses, ok := data.(map[string]map[string]string)
	if !ok {
		return fmt.Errorf("invalid processing status data format")
	}

	for jobID, statusData := range statuses {
		if err := r.HSetMap(ctx, "processing_status", jobID, statusData, r.cacheConfig.ProcessingTTL); err != nil {
			return err
		}
	}

	return nil
}

// Health Check

// HealthCheck performs a health check on the Redis connection
func (r *RedisClient) HealthCheck(ctx context.Context) error {
	// Test basic connectivity
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	// Test basic operations
	testKey := "health_check"
	testValue := "ok"
	
	if err := r.Set(ctx, "health", testKey, testValue, time.Minute); err != nil {
		return fmt.Errorf("Redis set operation failed: %w", err)
	}

	if value, err := r.Get(ctx, "health", testKey); err != nil {
		return fmt.Errorf("Redis get operation failed: %w", err)
	} else if value != testValue {
		return fmt.Errorf("Redis value mismatch: expected %s, got %s", testValue, value)
	}

	if err := r.Delete(ctx, "health", testKey); err != nil {
		return fmt.Errorf("Redis delete operation failed: %w", err)
	}

	return nil
}

// GetConnectionStats returns connection pool statistics
func (r *RedisClient) GetConnectionStats() *redis.PoolStats {
	return r.client.PoolStats()
}