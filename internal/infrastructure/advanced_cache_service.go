package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
)

// AdvancedCacheService provides sophisticated caching strategies
type AdvancedCacheService struct {
	client         *redis.Client
	logger         *slog.Logger
	defaultTTL     time.Duration
	
	// Cache warming
	warmingEnabled bool
	warmingMutex   sync.Mutex
	
	// Multi-level caching
	localCache     map[string]*cacheEntry
	localCacheMux  sync.RWMutex
	localCacheTTL  time.Duration
	
	// Cache statistics
	stats         *CacheStats
	statsMutex    sync.RWMutex
}

// cacheEntry represents a local cache entry
type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	RedisHits       int64
	RedisMisses     int64
	LocalHits       int64
	LocalMisses     int64
	WarmingRequests int64
	Evictions       int64
}

// CacheStrategy defines caching behavior for different data types
type CacheStrategy struct {
	TTL              time.Duration
	UseLocalCache    bool
	WarmOnMiss       bool
	PartitionKey     string
	CompressionLevel int
}

// NewAdvancedCacheService creates a new advanced cache service
func NewAdvancedCacheService(client *redis.Client, logger *slog.Logger) *AdvancedCacheService {
	return &AdvancedCacheService{
		client:         client,
		logger:         logger,
		defaultTTL:     5 * time.Minute,
		warmingEnabled: true,
		localCache:     make(map[string]*cacheEntry),
		localCacheTTL:  30 * time.Second,
		stats:          &CacheStats{},
	}
}

// GetWithStrategy retrieves data using the specified caching strategy
func (a *AdvancedCacheService) GetWithStrategy(ctx context.Context, key string, strategy *CacheStrategy) ([]byte, error) {
	// Try local cache first if enabled
	if strategy.UseLocalCache {
		if data := a.getFromLocalCache(key); data != nil {
			a.incrementLocalHits()
			return data, nil
		}
		a.incrementLocalMisses()
	}

	// Try Redis cache
	data, err := a.getFromRedis(ctx, key)
	if err != nil {
		return nil, err
	}

	if data != nil {
		a.incrementRedisHits()
		
		// Store in local cache if enabled
		if strategy.UseLocalCache {
			a.setLocalCache(key, data, strategy.TTL)
		}
		
		return data, nil
	}

	a.incrementRedisMisses()

	// Trigger cache warming if enabled
	if strategy.WarmOnMiss && a.warmingEnabled {
		go a.warmCache(ctx, key, strategy)
	}

	return nil, nil
}

// SetWithStrategy stores data using the specified caching strategy
func (a *AdvancedCacheService) SetWithStrategy(ctx context.Context, key string, data []byte, strategy *CacheStrategy) error {
	// Set in Redis
	err := a.setRedis(ctx, key, data, strategy.TTL)
	if err != nil {
		return err
	}

	// Set in local cache if enabled
	if strategy.UseLocalCache {
		a.setLocalCache(key, data, strategy.TTL)
	}

	return nil
}

// GetReviewSummaryWithCache retrieves review summary with advanced caching
func (a *AdvancedCacheService) GetReviewSummaryWithCache(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	key := fmt.Sprintf("hotel:summary:v2:%s", hotelID.String())
	
	strategy := &CacheStrategy{
		TTL:           10 * time.Minute,
		UseLocalCache: true,
		WarmOnMiss:    true,
	}

	data, err := a.GetWithStrategy(ctx, key, strategy)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil // Cache miss
	}

	var summary domain.ReviewSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		a.logger.Error("Failed to unmarshal review summary", "error", err, "key", key)
		// Invalidate corrupted cache entry
		_ = a.Delete(ctx, key)
		return nil, nil
	}

	return &summary, nil
}

// SetReviewSummaryWithCache stores review summary with advanced caching
func (a *AdvancedCacheService) SetReviewSummaryWithCache(ctx context.Context, hotelID uuid.UUID, summary *domain.ReviewSummary) error {
	key := fmt.Sprintf("hotel:summary:v2:%s", hotelID.String())
	
	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal review summary: %w", err)
	}

	strategy := &CacheStrategy{
		TTL:           10 * time.Minute,
		UseLocalCache: true,
	}

	return a.SetWithStrategy(ctx, key, data, strategy)
}

// GetSearchResultsWithCache retrieves search results with caching
func (a *AdvancedCacheService) GetSearchResultsWithCache(ctx context.Context, searchHash string) ([]domain.Review, error) {
	key := fmt.Sprintf("search:results:%s", searchHash)
	
	strategy := &CacheStrategy{
		TTL:           2 * time.Minute, // Shorter TTL for search results
		UseLocalCache: false,          // Don't use local cache for large datasets
		WarmOnMiss:    false,
	}

	data, err := a.GetWithStrategy(ctx, key, strategy)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil // Cache miss
	}

	var reviews []domain.Review
	if err := json.Unmarshal(data, &reviews); err != nil {
		a.logger.Error("Failed to unmarshal search results", "error", err, "key", key)
		_ = a.Delete(ctx, key)
		return nil, nil
	}

	return reviews, nil
}

// SetSearchResultsWithCache stores search results with caching
func (a *AdvancedCacheService) SetSearchResultsWithCache(ctx context.Context, searchHash string, reviews []domain.Review) error {
	key := fmt.Sprintf("search:results:%s", searchHash)
	
	data, err := json.Marshal(reviews)
	if err != nil {
		return fmt.Errorf("failed to marshal search results: %w", err)
	}

	strategy := &CacheStrategy{
		TTL:           2 * time.Minute,
		UseLocalCache: false,
	}

	return a.SetWithStrategy(ctx, key, data, strategy)
}

// WarmCache preloads cache for frequently accessed data
func (a *AdvancedCacheService) WarmCache(ctx context.Context, hotelIDs []uuid.UUID) error {
	if !a.warmingEnabled {
		return nil
	}

	a.warmingMutex.Lock()
	defer a.warmingMutex.Unlock()

	a.logger.Info("Starting cache warming", "hotel_count", len(hotelIDs))

	for _, hotelID := range hotelIDs {
		key := fmt.Sprintf("hotel:summary:v2:%s", hotelID.String())
		
		// Check if already cached
		exists, err := a.client.Exists(ctx, key).Result()
		if err != nil {
			a.logger.Error("Failed to check cache existence during warming", "error", err, "key", key)
			continue
		}

		if exists > 0 {
			continue // Already cached
		}

		// This would need to be connected to the actual data service
		// For now, just mark as warming requested
		a.incrementWarmingRequests()
	}

	a.logger.Info("Cache warming completed", "hotel_count", len(hotelIDs))
	return nil
}

// InvalidatePattern removes all cache entries matching a pattern
func (a *AdvancedCacheService) InvalidatePattern(ctx context.Context, pattern string) error {
	keys, err := a.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for pattern %s: %w", pattern, err)
	}

	if len(keys) == 0 {
		return nil
	}

	if err := a.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}

	// Also remove from local cache
	a.localCacheMux.Lock()
	for _, key := range keys {
		delete(a.localCache, key)
	}
	a.localCacheMux.Unlock()

	a.logger.Info("Invalidated cache pattern", "pattern", pattern, "keys_removed", len(keys))
	return nil
}

// InvalidateHotelData removes all cache entries for a specific hotel
func (a *AdvancedCacheService) InvalidateHotelData(ctx context.Context, hotelID uuid.UUID) error {
	patterns := []string{
		fmt.Sprintf("hotel:summary:*:%s", hotelID.String()),
		fmt.Sprintf("hotel:reviews:*:%s", hotelID.String()),
		fmt.Sprintf("hotel:stats:*:%s", hotelID.String()),
	}

	for _, pattern := range patterns {
		if err := a.InvalidatePattern(ctx, pattern); err != nil {
			a.logger.Error("Failed to invalidate pattern", "pattern", pattern, "error", err)
		}
	}

	return nil
}

// GetCacheStats returns current cache statistics
func (a *AdvancedCacheService) GetCacheStats() *CacheStats {
	a.statsMutex.RLock()
	defer a.statsMutex.RUnlock()

	// Return a copy
	return &CacheStats{
		RedisHits:       a.stats.RedisHits,
		RedisMisses:     a.stats.RedisMisses,
		LocalHits:       a.stats.LocalHits,
		LocalMisses:     a.stats.LocalMisses,
		WarmingRequests: a.stats.WarmingRequests,
		Evictions:       a.stats.Evictions,
	}
}

// CleanupLocalCache removes expired entries from local cache
func (a *AdvancedCacheService) CleanupLocalCache() {
	a.localCacheMux.Lock()
	defer a.localCacheMux.Unlock()

	now := time.Now()
	evicted := 0

	for key, entry := range a.localCache {
		if now.After(entry.expiresAt) {
			delete(a.localCache, key)
			evicted++
		}
	}

	if evicted > 0 {
		a.incrementEvictions(int64(evicted))
		a.logger.Debug("Local cache cleanup completed", "evicted", evicted)
	}
}

// StartBackgroundCleanup starts a goroutine for periodic local cache cleanup
func (a *AdvancedCacheService) StartBackgroundCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.CleanupLocalCache()
			}
		}
	}()
}

// Helper methods for local cache
func (a *AdvancedCacheService) getFromLocalCache(key string) []byte {
	a.localCacheMux.RLock()
	defer a.localCacheMux.RUnlock()

	entry, exists := a.localCache[key]
	if !exists {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		// Entry expired, clean it up
		go func() {
			a.localCacheMux.Lock()
			delete(a.localCache, key)
			a.localCacheMux.Unlock()
		}()
		return nil
	}

	return entry.data
}

func (a *AdvancedCacheService) setLocalCache(key string, data []byte, ttl time.Duration) {
	if ttl == 0 {
		ttl = a.localCacheTTL
	}

	a.localCacheMux.Lock()
	defer a.localCacheMux.Unlock()

	a.localCache[key] = &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

// Helper methods for Redis operations
func (a *AdvancedCacheService) getFromRedis(ctx context.Context, key string) ([]byte, error) {
	val, err := a.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from Redis: %w", err)
	}
	return []byte(val), nil
}

func (a *AdvancedCacheService) setRedis(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = a.defaultTTL
	}

	err := a.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set Redis cache: %w", err)
	}
	return nil
}

// Basic cache operations for compatibility
func (a *AdvancedCacheService) Get(ctx context.Context, key string) ([]byte, error) {
	strategy := &CacheStrategy{
		TTL:           a.defaultTTL,
		UseLocalCache: true,
		WarmOnMiss:    false,
	}
	return a.GetWithStrategy(ctx, key, strategy)
}

func (a *AdvancedCacheService) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	strategy := &CacheStrategy{
		TTL:           expiration,
		UseLocalCache: true,
	}
	return a.SetWithStrategy(ctx, key, value, strategy)
}

func (a *AdvancedCacheService) Delete(ctx context.Context, key string) error {
	// Delete from Redis
	err := a.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}

	// Delete from local cache
	a.localCacheMux.Lock()
	delete(a.localCache, key)
	a.localCacheMux.Unlock()

	return nil
}

func (a *AdvancedCacheService) Exists(ctx context.Context, key string) (bool, error) {
	// Check local cache first
	if a.getFromLocalCache(key) != nil {
		return true, nil
	}

	// Check Redis
	n, err := a.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check Redis existence: %w", err)
	}
	return n > 0, nil
}

func (a *AdvancedCacheService) FlushAll(ctx context.Context) error {
	// Clear Redis
	err := a.client.FlushAll(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to flush Redis: %w", err)
	}

	// Clear local cache
	a.localCacheMux.Lock()
	a.localCache = make(map[string]*cacheEntry)
	a.localCacheMux.Unlock()

	return nil
}

// Statistics helper methods
func (a *AdvancedCacheService) incrementRedisHits() {
	a.statsMutex.Lock()
	a.stats.RedisHits++
	a.statsMutex.Unlock()
}

func (a *AdvancedCacheService) incrementRedisMisses() {
	a.statsMutex.Lock()
	a.stats.RedisMisses++
	a.statsMutex.Unlock()
}

func (a *AdvancedCacheService) incrementLocalHits() {
	a.statsMutex.Lock()
	a.stats.LocalHits++
	a.statsMutex.Unlock()
}

func (a *AdvancedCacheService) incrementLocalMisses() {
	a.statsMutex.Lock()
	a.stats.LocalMisses++
	a.statsMutex.Unlock()
}

func (a *AdvancedCacheService) incrementWarmingRequests() {
	a.statsMutex.Lock()
	a.stats.WarmingRequests++
	a.statsMutex.Unlock()
}

func (a *AdvancedCacheService) incrementEvictions(count int64) {
	a.statsMutex.Lock()
	a.stats.Evictions += count
	a.statsMutex.Unlock()
}

func (a *AdvancedCacheService) warmCache(ctx context.Context, key string, strategy *CacheStrategy) {
	a.incrementWarmingRequests()
	// Implementation would depend on the specific data source
	// This is a placeholder for the warming logic
}