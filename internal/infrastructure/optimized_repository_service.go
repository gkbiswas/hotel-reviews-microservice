package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// OptimizedRepositoryService wraps the repository with advanced caching and optimization
type OptimizedRepositoryService struct {
	repository    domain.ReviewRepository
	advancedCache *AdvancedCacheService
	logger        *logger.Logger
}

// NewOptimizedRepositoryService creates a new optimized repository service
func NewOptimizedRepositoryService(
	repository domain.ReviewRepository,
	advancedCache *AdvancedCacheService,
	logger *logger.Logger,
) *OptimizedRepositoryService {
	return &OptimizedRepositoryService{
		repository:    repository,
		advancedCache: advancedCache,
		logger:        logger,
	}
}

// GetReviewSummaryOptimized gets review summary with intelligent caching
func (o *OptimizedRepositoryService) GetReviewSummaryOptimized(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	start := time.Now()

	// Try advanced cache first
	summary, err := o.advancedCache.GetReviewSummaryWithCache(ctx, hotelID)
	if err != nil {
		o.logger.ErrorContext(ctx, "Cache error for review summary", "error", err, "hotel_id", hotelID)
		// Fall through to database
	} else if summary != nil {
		o.logger.DebugContext(ctx, "Review summary cache hit", 
			"hotel_id", hotelID, 
			"duration_ms", time.Since(start).Milliseconds())
		return summary, nil
	}

	// Cache miss - get from database
	summary, err = o.repository.GetReviewSummaryByHotelID(ctx, hotelID)
	if err != nil {
		return nil, err
	}

	if summary != nil {
		// Store in cache asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := o.advancedCache.SetReviewSummaryWithCache(ctx, hotelID, summary); err != nil {
				o.logger.ErrorContext(ctx, "Failed to cache review summary", "error", err, "hotel_id", hotelID)
			}
		}()
	}

	o.logger.DebugContext(ctx, "Review summary database hit", 
		"hotel_id", hotelID, 
		"duration_ms", time.Since(start).Milliseconds())

	return summary, nil
}

// SearchReviewsOptimized performs optimized search with result caching
func (o *OptimizedRepositoryService) SearchReviewsOptimized(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]domain.Review, error) {
	start := time.Now()

	// Generate cache key based on search parameters
	searchHash := o.generateSearchHash(query, filters, limit, offset)

	// Try cache first (but only for reasonable result sets)
	if limit <= 100 { // Only cache smaller result sets
		reviews, err := o.advancedCache.GetSearchResultsWithCache(ctx, searchHash)
		if err != nil {
			o.logger.ErrorContext(ctx, "Cache error for search results", "error", err, "search_hash", searchHash)
		} else if reviews != nil {
			o.logger.DebugContext(ctx, "Search results cache hit", 
				"search_hash", searchHash, 
				"result_count", len(reviews),
				"duration_ms", time.Since(start).Milliseconds())
			return reviews, nil
		}
	}

	// Cache miss - search database
	reviews, err := o.repository.Search(ctx, query, filters, limit, offset)
	if err != nil {
		return nil, err
	}

	// Cache results asynchronously for smaller result sets
	if len(reviews) <= 100 {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := o.advancedCache.SetSearchResultsWithCache(ctx, searchHash, reviews); err != nil {
				o.logger.ErrorContext(ctx, "Failed to cache search results", "error", err, "search_hash", searchHash)
			}
		}()
	}

	o.logger.DebugContext(ctx, "Search results database hit", 
		"search_hash", searchHash,
		"result_count", len(reviews),
		"duration_ms", time.Since(start).Milliseconds())

	return reviews, nil
}

// GetReviewsByHotelOptimized gets hotel reviews with intelligent caching and pagination
func (o *OptimizedRepositoryService) GetReviewsByHotelOptimized(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]domain.Review, error) {
	start := time.Now()

	// For first page with reasonable limits, try caching
	if offset == 0 && limit <= 50 {
		cacheKey := fmt.Sprintf("hotel:reviews:first_page:%s:%d", hotelID.String(), limit)
		
		strategy := &CacheStrategy{
			TTL:           5 * time.Minute,
			UseLocalCache: true,
			WarmOnMiss:    false,
		}

		data, err := o.advancedCache.GetWithStrategy(ctx, cacheKey, strategy)
		if err != nil {
			o.logger.ErrorContext(ctx, "Cache error for hotel reviews", "error", err, "hotel_id", hotelID)
		} else if data != nil {
			var reviews []domain.Review
			if err := json.Unmarshal(data, &reviews); err == nil {
				o.logger.DebugContext(ctx, "Hotel reviews cache hit", 
					"hotel_id", hotelID,
					"result_count", len(reviews),
					"duration_ms", time.Since(start).Milliseconds())
				return reviews, nil
			}
		}
	}

	// Get from database
	reviews, err := o.repository.GetByHotel(ctx, hotelID, limit, offset)
	if err != nil {
		return nil, err
	}

	// Cache first page results asynchronously
	if offset == 0 && limit <= 50 && len(reviews) > 0 {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			cacheKey := fmt.Sprintf("hotel:reviews:first_page:%s:%d", hotelID.String(), limit)
			data, err := json.Marshal(reviews)
			if err != nil {
				return
			}

			strategy := &CacheStrategy{
				TTL:           5 * time.Minute,
				UseLocalCache: true,
			}

			if err := o.advancedCache.SetWithStrategy(ctx, cacheKey, data, strategy); err != nil {
				o.logger.ErrorContext(ctx, "Failed to cache hotel reviews", "error", err, "hotel_id", hotelID)
			}
		}()
	}

	o.logger.DebugContext(ctx, "Hotel reviews database hit", 
		"hotel_id", hotelID,
		"result_count", len(reviews),
		"duration_ms", time.Since(start).Milliseconds())

	return reviews, nil
}

// UpdateReviewSummaryOptimized updates review summary and invalidates cache
func (o *OptimizedRepositoryService) UpdateReviewSummaryOptimized(ctx context.Context, hotelID uuid.UUID) error {
	start := time.Now()

	// Update in database
	err := o.repository.UpdateReviewSummary(ctx, hotelID)
	if err != nil {
		return err
	}

	// Invalidate related cache entries asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := o.advancedCache.InvalidateHotelData(ctx, hotelID); err != nil {
			o.logger.ErrorContext(ctx, "Failed to invalidate hotel cache after summary update", 
				"error", err, "hotel_id", hotelID)
		}
	}()

	o.logger.InfoContext(ctx, "Review summary updated and cache invalidated", 
		"hotel_id", hotelID,
		"duration_ms", time.Since(start).Milliseconds())

	return nil
}

// BatchUpdateReviewSummariesOptimized updates multiple review summaries efficiently
func (o *OptimizedRepositoryService) BatchUpdateReviewSummariesOptimized(ctx context.Context, hotelIDs []uuid.UUID) error {
	start := time.Now()

	// Process in smaller batches to avoid overwhelming the database
	batchSize := 10
	for i := 0; i < len(hotelIDs); i += batchSize {
		end := i + batchSize
		if end > len(hotelIDs) {
			end = len(hotelIDs)
		}

		batch := hotelIDs[i:end]
		
		// Update each hotel's summary
		for _, hotelID := range batch {
			if err := o.repository.UpdateReviewSummary(ctx, hotelID); err != nil {
				o.logger.ErrorContext(ctx, "Failed to update review summary in batch", 
					"error", err, "hotel_id", hotelID)
				continue
			}
		}

		// Invalidate cache for the batch
		go func(batchHotelIDs []uuid.UUID) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			for _, hotelID := range batchHotelIDs {
				if err := o.advancedCache.InvalidateHotelData(ctx, hotelID); err != nil {
					o.logger.ErrorContext(ctx, "Failed to invalidate hotel cache in batch", 
						"error", err, "hotel_id", hotelID)
				}
			}
		}(batch)

		// Small delay between batches to avoid overwhelming the system
		if end < len(hotelIDs) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	o.logger.InfoContext(ctx, "Batch review summary update completed", 
		"hotel_count", len(hotelIDs),
		"duration_ms", time.Since(start).Milliseconds())

	return nil
}

// WarmFrequentlyAccessedData preloads cache for popular hotels
func (o *OptimizedRepositoryService) WarmFrequentlyAccessedData(ctx context.Context, limit int) error {
	start := time.Now()

	// Get top rated hotels (these are likely to be frequently accessed)
	hotels, err := o.repository.GetTopRatedHotels(ctx, limit)
	if err != nil {
		return fmt.Errorf("failed to get top rated hotels for cache warming: %w", err)
	}

	hotelIDs := make([]uuid.UUID, len(hotels))
	for i, hotel := range hotels {
		hotelIDs[i] = hotel.ID
	}

	// Warm the cache
	if err := o.advancedCache.WarmCache(ctx, hotelIDs); err != nil {
		return fmt.Errorf("failed to warm cache: %w", err)
	}

	// Preload review summaries
	for _, hotelID := range hotelIDs {
		go func(id uuid.UUID) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			_, _ = o.GetReviewSummaryOptimized(ctx, id)
		}(hotelID)
	}

	o.logger.InfoContext(ctx, "Cache warming completed", 
		"hotel_count", len(hotelIDs),
		"duration_ms", time.Since(start).Milliseconds())

	return nil
}

// GetOptimizationMetrics returns performance metrics
func (o *OptimizedRepositoryService) GetOptimizationMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	// Get cache statistics
	cacheStats := o.advancedCache.GetCacheStats()
	metrics["cache"] = map[string]interface{}{
		"redis_hit_rate":  float64(cacheStats.RedisHits) / float64(cacheStats.RedisHits+cacheStats.RedisMisses) * 100,
		"local_hit_rate":  float64(cacheStats.LocalHits) / float64(cacheStats.LocalHits+cacheStats.LocalMisses) * 100,
		"redis_hits":      cacheStats.RedisHits,
		"redis_misses":    cacheStats.RedisMisses,
		"local_hits":      cacheStats.LocalHits,
		"local_misses":    cacheStats.LocalMisses,
		"warming_requests": cacheStats.WarmingRequests,
		"evictions":       cacheStats.Evictions,
	}

	// Get query performance metrics if available
	if repo, ok := o.repository.(*ReviewRepository); ok {
		metrics["database"] = repo.GetQueryPerformanceSummary()
		metrics["slow_queries"] = repo.GetSlowQueries(5)
		metrics["optimization_suggestions"] = repo.GetOptimizationSuggestions()
	}

	return metrics
}

// generateSearchHash creates a hash for search parameters
func (o *OptimizedRepositoryService) generateSearchHash(query string, filters map[string]interface{}, limit, offset int) string {
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("q:%s|l:%d|o:%d", query, limit, offset)))
	
	// Add filters in a consistent order
	for key, value := range filters {
		hash.Write([]byte(fmt.Sprintf("|%s:%v", key, value)))
	}
	
	return fmt.Sprintf("%x", hash.Sum(nil))[:16] // Use first 16 chars
}

// Cleanup performs maintenance tasks
func (o *OptimizedRepositoryService) Cleanup(ctx context.Context) {
	o.advancedCache.CleanupLocalCache()
	o.logger.InfoContext(ctx, "Optimization service cleanup completed")
}

// StartBackgroundTasks starts background optimization tasks
func (o *OptimizedRepositoryService) StartBackgroundTasks(ctx context.Context) {
	// Start cache cleanup
	o.advancedCache.StartBackgroundCleanup(ctx)

	// Start periodic cache warming
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := o.WarmFrequentlyAccessedData(ctx, 50); err != nil {
					o.logger.ErrorContext(ctx, "Failed to warm cache in background", "error", err)
				}
			}
		}
	}()

	o.logger.InfoContext(ctx, "Background optimization tasks started")
}