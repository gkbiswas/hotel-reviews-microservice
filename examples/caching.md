# Caching Strategies Guide

## Overview

This guide covers comprehensive caching strategies implemented in the Hotel Reviews Microservice, including multi-layer caching, cache patterns, and performance optimization techniques.

## Cache Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   L1 Cache      │    │   L2 Cache      │    │   Database      │
│   (API Layer)   │    │   (In-Memory)   │    │   (Redis)       │    │   (PostgreSQL)  │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ Request Handler │───▶│ LRU Cache       │───▶│ Distributed     │───▶│ Primary Store   │
│ Business Logic  │    │ 100MB Limit     │    │ TTL: 1-24h      │    │ Persistent Data │
│ Response Format │    │ TTL: 5-15min    │    │ Compression     │    │ ACID Compliance │
└─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │   CDN Cache     │
                       │   (CloudFront)  │
                       │                 │
                       │ Static Assets   │
                       │ API Responses   │
                       │ TTL: 1-7 days   │
                       └─────────────────┘
```

## L1 Cache (In-Memory)

### 1. Implementation

```go
type L1Cache struct {
    cache     map[string]*CacheEntry
    mutex     sync.RWMutex
    maxSize   int64
    currentSize int64
    lru       *list.List
    items     map[string]*list.Element
}

type CacheEntry struct {
    Key        string
    Value      interface{}
    Size       int64
    ExpiresAt  time.Time
    AccessedAt time.Time
    CreatedAt  time.Time
}

func NewL1Cache(maxSizeMB int) *L1Cache {
    return &L1Cache{
        cache:   make(map[string]*CacheEntry),
        maxSize: int64(maxSizeMB) * 1024 * 1024, // Convert MB to bytes
        lru:     list.New(),
        items:   make(map[string]*list.Element),
    }
}

func (c *L1Cache) Get(key string) (interface{}, bool) {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    entry, exists := c.cache[key]
    if !exists {
        return nil, false
    }
    
    // Check expiration
    if entry.ExpiresAt.Before(time.Now()) {
        // Don't remove here to avoid lock upgrade
        go c.removeExpired(key)
        return nil, false
    }
    
    // Update access time and LRU position
    entry.AccessedAt = time.Now()
    c.moveToFront(key)
    
    return entry.Value, true
}

func (c *L1Cache) Set(key string, value interface{}, ttl time.Duration) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    size := estimateSize(value)
    entry := &CacheEntry{
        Key:        key,
        Value:      value,
        Size:       size,
        ExpiresAt:  time.Now().Add(ttl),
        AccessedAt: time.Now(),
        CreatedAt:  time.Now(),
    }
    
    // Remove existing entry if exists
    if existing, exists := c.cache[key]; exists {
        c.currentSize -= existing.Size
        c.removeFromLRU(key)
    }
    
    // Ensure we have space
    for c.currentSize+size > c.maxSize && c.lru.Len() > 0 {
        c.evictLRU()
    }
    
    // Add new entry
    c.cache[key] = entry
    c.currentSize += size
    element := c.lru.PushFront(key)
    c.items[key] = element
}

func (c *L1Cache) evictLRU() {
    back := c.lru.Back()
    if back != nil {
        key := back.Value.(string)
        c.removeFromCache(key)
    }
}

func (c *L1Cache) GetStats() CacheStats {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    return CacheStats{
        Entries:     len(c.cache),
        SizeBytes:   c.currentSize,
        MaxSize:     c.maxSize,
        Utilization: float64(c.currentSize) / float64(c.maxSize) * 100,
    }
}
```

### 2. Hot Data Identification

```go
type HotDataTracker struct {
    accessCount map[string]int64
    lastAccess  map[string]time.Time
    mutex       sync.RWMutex
    threshold   int64
}

func (h *HotDataTracker) RecordAccess(key string) {
    h.mutex.Lock()
    defer h.mutex.Unlock()
    
    h.accessCount[key]++
    h.lastAccess[key] = time.Now()
}

func (h *HotDataTracker) IsHotData(key string) bool {
    h.mutex.RLock()
    defer h.mutex.RUnlock()
    
    count := h.accessCount[key]
    lastAccess := h.lastAccess[key]
    
    // Consider data hot if accessed frequently in recent time
    return count >= h.threshold && 
           time.Since(lastAccess) < 5*time.Minute
}

// Automatic L1 cache population for hot data
func (c *CacheService) AutoPromoteToL1(key string, value interface{}) {
    if c.hotDataTracker.IsHotData(key) {
        c.l1Cache.Set(key, value, 15*time.Minute)
    }
}
```

## L2 Cache (Redis)

### 1. Advanced Redis Operations

```go
type RedisCache struct {
    client   *redis.Client
    pipeline *redis.Pipeline
    mutex    sync.Mutex
}

// Batch operations for better performance
func (r *RedisCache) MGet(ctx context.Context, keys []string) ([]interface{}, error) {
    pipe := r.client.Pipeline()
    
    // Batch get operations
    commands := make([]*redis.StringCmd, len(keys))
    for i, key := range keys {
        commands[i] = pipe.Get(ctx, key)
    }
    
    _, err := pipe.Exec(ctx)
    if err != nil && err != redis.Nil {
        return nil, fmt.Errorf("pipeline exec failed: %w", err)
    }
    
    results := make([]interface{}, len(keys))
    for i, cmd := range commands {
        val, err := cmd.Result()
        if err == redis.Nil {
            results[i] = nil
        } else if err != nil {
            results[i] = nil
        } else {
            var data interface{}
            if err := json.Unmarshal([]byte(val), &data); err == nil {
                results[i] = data
            }
        }
    }
    
    return results, nil
}

// Cache with compression for large objects
func (r *RedisCache) SetCompressed(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return fmt.Errorf("failed to marshal data: %w", err)
    }
    
    // Compress if data is large
    if len(data) > 1024 { // 1KB threshold
        compressed, err := compress(data)
        if err != nil {
            return fmt.Errorf("compression failed: %w", err)
        }
        
        // Use special prefix to indicate compressed data
        return r.client.Set(ctx, "compressed:"+key, compressed, ttl).Err()
    }
    
    return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisCache) GetCompressed(ctx context.Context, key string) (interface{}, error) {
    // Try compressed version first
    compressedKey := "compressed:" + key
    if r.client.Exists(ctx, compressedKey).Val() == 1 {
        compressed, err := r.client.Get(ctx, compressedKey).Bytes()
        if err != nil {
            return nil, err
        }
        
        data, err := decompress(compressed)
        if err != nil {
            return nil, fmt.Errorf("decompression failed: %w", err)
        }
        
        var result interface{}
        if err := json.Unmarshal(data, &result); err != nil {
            return nil, fmt.Errorf("unmarshal failed: %w", err)
        }
        
        return result, nil
    }
    
    // Try regular version
    data, err := r.client.Get(ctx, key).Result()
    if err != nil {
        return nil, err
    }
    
    var result interface{}
    if err := json.Unmarshal([]byte(data), &result); err != nil {
        return nil, fmt.Errorf("unmarshal failed: %w", err)
    }
    
    return result, nil
}
```

### 2. Cache Patterns Implementation

```go
// Write-Through Cache Pattern
func (s *ReviewService) CreateReviewWriteThrough(ctx context.Context, review *Review) error {
    // Write to database first
    if err := s.db.CreateReview(ctx, review); err != nil {
        return fmt.Errorf("database write failed: %w", err)
    }
    
    // Write to cache
    cacheKey := fmt.Sprintf("review:%s", review.ID)
    if err := s.cache.Set(ctx, cacheKey, review, 2*time.Hour); err != nil {
        // Log error but don't fail the operation
        log.Errorf("Cache write failed: %v", err)
    }
    
    return nil
}

// Write-Behind (Write-Back) Cache Pattern
func (s *ReviewService) CreateReviewWriteBehind(ctx context.Context, review *Review) error {
    // Write to cache first
    cacheKey := fmt.Sprintf("review:%s", review.ID)
    if err := s.cache.Set(ctx, cacheKey, review, 2*time.Hour); err != nil {
        return fmt.Errorf("cache write failed: %w", err)
    }
    
    // Mark as dirty for async write to database
    s.writeQueue.Enqueue(&WriteOperation{
        Type:   "create_review",
        Key:    cacheKey,
        Data:   review,
        Queued: time.Now(),
    })
    
    return nil
}

// Cache-Aside Pattern
func (s *ReviewService) GetReviewCacheAside(ctx context.Context, id uuid.UUID) (*Review, error) {
    cacheKey := fmt.Sprintf("review:%s", id)
    
    // Try cache first
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        if review, ok := cached.(*Review); ok {
            return review, nil
        }
    }
    
    // Cache miss - get from database
    review, err := s.db.GetReview(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("database read failed: %w", err)
    }
    
    // Update cache
    go func() {
        if err := s.cache.Set(context.Background(), cacheKey, review, 2*time.Hour); err != nil {
            log.Errorf("Failed to update cache: %v", err)
        }
    }()
    
    return review, nil
}
```

## Cache Strategies by Data Type

### 1. Hotel Data Caching

```go
func (s *HotelService) GetHotelWithMultiLayerCache(ctx context.Context, id uuid.UUID) (*Hotel, error) {
    cacheKey := fmt.Sprintf("hotel:%s", id)
    
    // L1 Cache check
    if hotel, exists := s.l1Cache.Get(cacheKey); exists {
        return hotel.(*Hotel), nil
    }
    
    // L2 Cache check
    if cached, err := s.redisCache.GetCompressed(ctx, cacheKey); err == nil {
        hotel := cached.(*Hotel)
        // Promote to L1 if hot data
        s.autoPromoteToL1(cacheKey, hotel)
        return hotel, nil
    }
    
    // Database fallback
    hotel, err := s.db.GetHotel(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("database read failed: %w", err)
    }
    
    // Cache in both layers
    go s.cacheHotelData(ctx, cacheKey, hotel)
    
    return hotel, nil
}

func (s *HotelService) cacheHotelData(ctx context.Context, key string, hotel *Hotel) {
    // L2 Cache (longer TTL, compressed)
    s.redisCache.SetCompressed(ctx, key, hotel, 4*time.Hour)
    
    // L1 Cache (shorter TTL, uncompressed for speed)
    if s.hotDataTracker.IsHotData(key) {
        s.l1Cache.Set(key, hotel, 10*time.Minute)
    }
}
```

### 2. Search Results Caching

```go
func (s *SearchService) SearchReviewsWithCache(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
    // Generate cache key from query parameters
    cacheKey := s.generateSearchCacheKey(query)
    
    // Check cache
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        result := cached.(*SearchResult)
        result.FromCache = true
        return result, nil
    }
    
    // Perform search
    result, err := s.performSearch(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }
    
    // Cache result with shorter TTL for search queries
    ttl := 30 * time.Minute
    if query.IsPopularQuery() {
        ttl = 2 * time.Hour // Popular queries cached longer
    }
    
    go s.cache.Set(ctx, cacheKey, result, ttl)
    
    return result, nil
}

func (s *SearchService) generateSearchCacheKey(query *SearchQuery) string {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%+v", query)))
    hash := hex.EncodeToString(h.Sum(nil))
    return fmt.Sprintf("search:%s", hash[:16])
}
```

### 3. Analytics Caching

```go
func (s *AnalyticsService) GetHotelStatsWithCache(ctx context.Context, hotelID uuid.UUID, timeframe string) (*HotelStats, error) {
    cacheKey := fmt.Sprintf("analytics:hotel:%s:%s", hotelID, timeframe)
    
    // Check cache
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        return cached.(*HotelStats), nil
    }
    
    // Calculate stats
    stats, err := s.calculateHotelStats(ctx, hotelID, timeframe)
    if err != nil {
        return nil, fmt.Errorf("stats calculation failed: %w", err)
    }
    
    // Cache with TTL based on data freshness requirements
    var ttl time.Duration
    switch timeframe {
    case "realtime":
        ttl = 1 * time.Minute
    case "hourly":
        ttl = 15 * time.Minute
    case "daily":
        ttl = 1 * time.Hour
    case "monthly":
        ttl = 6 * time.Hour
    default:
        ttl = 1 * time.Hour
    }
    
    go s.cache.Set(ctx, cacheKey, stats, ttl)
    
    return stats, nil
}
```

## Cache Invalidation

### 1. Smart Invalidation

```go
type CacheInvalidator struct {
    cache         CacheService
    eventBus      EventBus
    patterns      map[string][]string
    dependencies  map[string][]string
}

func NewCacheInvalidator(cache CacheService, eventBus EventBus) *CacheInvalidator {
    ci := &CacheInvalidator{
        cache:        cache,
        eventBus:     eventBus,
        patterns:     make(map[string][]string),
        dependencies: make(map[string][]string),
    }
    
    // Register invalidation patterns
    ci.registerInvalidationPatterns()
    
    // Subscribe to events
    ci.eventBus.Subscribe("review.*", ci.handleReviewEvents)
    ci.eventBus.Subscribe("hotel.*", ci.handleHotelEvents)
    
    return ci
}

func (ci *CacheInvalidator) registerInvalidationPatterns() {
    // Review events invalidate hotel-related caches
    ci.patterns["review.created"] = []string{
        "hotel:%s:*",
        "analytics:hotel:%s:*",
        "search:hotel:%s:*",
        "top-hotels:*",
    }
    
    ci.patterns["review.updated"] = []string{
        "review:%s",
        "hotel:%s:*",
        "analytics:hotel:%s:*",
    }
    
    ci.patterns["hotel.updated"] = []string{
        "hotel:%s",
        "hotel:%s:*",
        "search:city:%s:*",
    }
    
    // Define cache dependencies
    ci.dependencies["hotel:rating"] = []string{
        "analytics:hotel:*",
        "search:hotel:*",
        "top-hotels:*",
    }
}

func (ci *CacheInvalidator) handleReviewEvents(ctx context.Context, event *Event) error {
    reviewEvent := event.Data.(*ReviewEvent)
    
    patterns, exists := ci.patterns[event.Type]
    if !exists {
        return nil
    }
    
    // Invalidate cache patterns
    for _, pattern := range patterns {
        fullPattern := fmt.Sprintf(pattern, reviewEvent.HotelID)
        if err := ci.cache.InvalidatePattern(ctx, fullPattern); err != nil {
            log.Errorf("Failed to invalidate pattern %s: %v", fullPattern, err)
        }
    }
    
    return nil
}

// Batch invalidation for better performance
func (ci *CacheInvalidator) InvalidateMultiple(ctx context.Context, keys []string) error {
    if len(keys) == 0 {
        return nil
    }
    
    // Group keys by prefix for batch operations
    groups := make(map[string][]string)
    for _, key := range keys {
        prefix := strings.Split(key, ":")[0]
        groups[prefix] = append(groups[prefix], key)
    }
    
    // Process each group
    for _, groupKeys := range groups {
        if err := ci.cache.DeleteMultiple(ctx, groupKeys); err != nil {
            log.Errorf("Failed to delete cache group: %v", err)
        }
    }
    
    return nil
}
```

### 2. Time-Based Invalidation

```go
func (s *CacheService) SetWithAutomaticInvalidation(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    // Set cache entry
    if err := s.Set(ctx, key, value, ttl); err != nil {
        return err
    }
    
    // Schedule invalidation
    s.scheduler.Schedule(time.Now().Add(ttl), func() {
        if err := s.Delete(context.Background(), key); err != nil {
            log.Errorf("Failed to invalidate expired cache key %s: %v", key, err)
        }
    })
    
    return nil
}

// Proactive cache refresh before expiration
func (s *CacheService) RefreshBeforeExpiry(ctx context.Context, key string, refreshFunc func() (interface{}, error)) {
    // Get current TTL
    ttl, err := s.TTL(ctx, key)
    if err != nil {
        log.Errorf("Failed to get TTL for key %s: %v", key, err)
        return
    }
    
    // Schedule refresh at 80% of TTL
    refreshTime := time.Duration(float64(ttl) * 0.8)
    
    s.scheduler.Schedule(time.Now().Add(refreshTime), func() {
        // Refresh data
        newValue, err := refreshFunc()
        if err != nil {
            log.Errorf("Failed to refresh cache key %s: %v", key, err)
            return
        }
        
        // Update cache with new data
        if err := s.Set(context.Background(), key, newValue, ttl); err != nil {
            log.Errorf("Failed to update refreshed cache key %s: %v", key, err)
        }
    })
}
```

## Cache Warming

### 1. Proactive Cache Warming

```go
type CacheWarmer struct {
    cache       CacheService
    db          Database
    analytics   AnalyticsService
    scheduler   Scheduler
}

func (cw *CacheWarmer) WarmPopularContent(ctx context.Context) error {
    // Get popular hotels
    popularHotels, err := cw.analytics.GetPopularHotels(ctx, 100)
    if err != nil {
        return fmt.Errorf("failed to get popular hotels: %w", err)
    }
    
    // Warm hotel data in parallel
    semaphore := make(chan struct{}, 10) // Limit concurrent operations
    var wg sync.WaitGroup
    
    for _, hotel := range popularHotels {
        wg.Add(1)
        go func(h *Hotel) {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            cw.warmHotelData(ctx, h)
        }(hotel)
    }
    
    wg.Wait()
    return nil
}

func (cw *CacheWarmer) warmHotelData(ctx context.Context, hotel *Hotel) {
    hotelKey := fmt.Sprintf("hotel:%s", hotel.ID)
    
    // Cache hotel data
    cw.cache.Set(ctx, hotelKey, hotel, 4*time.Hour)
    
    // Cache recent reviews
    reviews, err := cw.db.GetRecentHotelReviews(ctx, hotel.ID, 20)
    if err == nil {
        reviewsKey := fmt.Sprintf("hotel:%s:reviews:recent", hotel.ID)
        cw.cache.Set(ctx, reviewsKey, reviews, 1*time.Hour)
    }
    
    // Cache analytics
    stats, err := cw.analytics.GetHotelStats(ctx, hotel.ID)
    if err == nil {
        statsKey := fmt.Sprintf("analytics:hotel:%s:stats", hotel.ID)
        cw.cache.Set(ctx, statsKey, stats, 2*time.Hour)
    }
}

// Scheduled cache warming
func (cw *CacheWarmer) ScheduleRegularWarming() {
    // Warm popular content every hour
    cw.scheduler.Every(1*time.Hour, func() {
        if err := cw.WarmPopularContent(context.Background()); err != nil {
            log.Errorf("Failed to warm popular content: %v", err)
        }
    })
    
    // Warm search results every 30 minutes
    cw.scheduler.Every(30*time.Minute, func() {
        if err := cw.WarmPopularSearches(context.Background()); err != nil {
            log.Errorf("Failed to warm popular searches: %v", err)
        }
    })
}
```

## Performance Monitoring

### 1. Cache Metrics

```go
type CacheMetrics struct {
    HitCount    int64   `json:"hit_count"`
    MissCount   int64   `json:"miss_count"`
    HitRatio    float64 `json:"hit_ratio"`
    AvgLatency  float64 `json:"avg_latency_ms"`
    Entries     int64   `json:"entries"`
    MemoryUsage int64   `json:"memory_usage_bytes"`
    Evictions   int64   `json:"evictions"`
}

func (s *CacheService) GetMetrics(ctx context.Context) (*CacheMetrics, error) {
    l1Stats := s.l1Cache.GetStats()
    l2Stats, err := s.redisCache.GetStats(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get Redis stats: %w", err)
    }
    
    totalHits := l1Stats.Hits + l2Stats.Hits
    totalMisses := l1Stats.Misses + l2Stats.Misses
    hitRatio := 0.0
    if totalHits+totalMisses > 0 {
        hitRatio = float64(totalHits) / float64(totalHits+totalMisses) * 100
    }
    
    return &CacheMetrics{
        HitCount:    totalHits,
        MissCount:   totalMisses,
        HitRatio:    hitRatio,
        AvgLatency:  (l1Stats.AvgLatency + l2Stats.AvgLatency) / 2,
        Entries:     l1Stats.Entries + l2Stats.Entries,
        MemoryUsage: l1Stats.MemoryUsage + l2Stats.MemoryUsage,
        Evictions:   l1Stats.Evictions + l2Stats.Evictions,
    }, nil
}

// Real-time cache monitoring
func (s *CacheService) MonitorPerformance(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            metrics, err := s.GetMetrics(ctx)
            if err != nil {
                log.Errorf("Failed to get cache metrics: %v", err)
                continue
            }
            
            // Log metrics
            log.Infof("Cache Performance: Hit Ratio: %.2f%%, Latency: %.2fms, Memory: %dMB", 
                metrics.HitRatio, metrics.AvgLatency, metrics.MemoryUsage/(1024*1024))
            
            // Send to monitoring system
            s.metricsCollector.Record("cache.hit_ratio", metrics.HitRatio)
            s.metricsCollector.Record("cache.latency", metrics.AvgLatency)
            s.metricsCollector.Record("cache.memory_usage", float64(metrics.MemoryUsage))
            
            // Alert on poor performance
            if metrics.HitRatio < 80 {
                s.alertService.Send(&Alert{
                    Type:     "cache_performance",
                    Severity: "warning",
                    Message:  fmt.Sprintf("Cache hit ratio is low: %.2f%%", metrics.HitRatio),
                })
            }
            
        case <-ctx.Done():
            return
        }
    }
}
```

This comprehensive caching guide covers all aspects of the multi-layered caching strategy implemented in the Hotel Reviews Microservice, ensuring optimal performance and efficient resource utilization.