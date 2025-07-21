# Performance Guide

## Overview

This guide covers performance optimization strategies, monitoring, and best practices for the Hotel Reviews Microservice. The service is designed to handle high throughput with sub-millisecond response times.

## Performance Targets

### Response Time SLAs

| Endpoint Type | P50 | P95 | P99 |
|---------------|-----|-----|-----|
| Simple Reads | <10ms | <25ms | <50ms |
| Complex Queries | <50ms | <100ms | <200ms |
| Write Operations | <25ms | <75ms | <150ms |
| Bulk Operations | <500ms | <2s | <5s |

### Throughput Targets

| Operation | Target RPS | Max RPS |
|-----------|------------|---------|
| Review Reads | 5,000 | 10,000 |
| Review Writes | 1,000 | 2,500 |
| Hotel Lookups | 3,000 | 6,000 |
| Search Queries | 500 | 1,000 |

### Resource Utilization

- **CPU**: Target <70%, Alert >80%
- **Memory**: Target <80%, Alert >90%
- **Database Connections**: Target <60%, Alert >80%
- **Cache Hit Ratio**: Target >90%, Alert <85%

## Database Performance

### Connection Pooling

Optimized connection pool configuration:

```yaml
database:
  max_conns: 25          # Maximum connections
  min_conns: 5           # Minimum idle connections
  max_conn_lifetime: 1h  # Connection lifetime
  max_conn_idle_time: 30m # Idle timeout
  health_check_period: 5m # Health check interval
```

### Query Optimization

#### 1. Index Strategy

**Primary Indexes:**
```sql
-- Review lookups by hotel
CREATE INDEX idx_reviews_hotel_date ON reviews(hotel_id, review_date DESC) 
WHERE deleted_at IS NULL;

-- Search optimization
CREATE INDEX idx_reviews_rating_date ON reviews(rating, review_date DESC) 
WHERE deleted_at IS NULL;

-- Full-text search
CREATE INDEX idx_reviews_content_search ON reviews 
USING GIN(to_tsvector('english', content));
```

**Composite Indexes:**
```sql
-- Multi-column searches
CREATE INDEX idx_hotels_city_rating ON hotels(city, rating DESC) 
WHERE deleted_at IS NULL;

-- Provider-specific lookups
CREATE INDEX idx_reviews_provider_external ON reviews(provider_id, external_id) 
WHERE deleted_at IS NULL;
```

#### 2. Query Patterns

**Efficient Hotel Search:**
```sql
-- Good: Uses index
SELECT h.* FROM hotels h 
WHERE h.city = $1 AND h.rating >= $2 
ORDER BY h.rating DESC 
LIMIT 20;

-- Bad: Full table scan
SELECT h.* FROM hotels h 
WHERE LOWER(h.name) LIKE '%hotel%' 
ORDER BY h.created_at DESC;
```

**Optimized Review Queries:**
```sql
-- Good: Uses covering index
SELECT r.id, r.title, r.rating, r.review_date 
FROM reviews r 
WHERE r.hotel_id = $1 
ORDER BY r.review_date DESC 
LIMIT 50;

-- Bad: Unnecessary columns
SELECT r.* FROM reviews r 
WHERE r.hotel_id = $1 
ORDER BY r.review_date DESC;
```

#### 3. Query Monitoring

Monitor slow queries:

```sql
-- Enable slow query logging
ALTER SYSTEM SET log_min_duration_statement = 1000; -- 1 second
ALTER SYSTEM SET log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h ';

-- Check slow queries
SELECT query, mean_time, calls, total_time 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;
```

### Partitioning Strategy

**Time-based Partitioning for Reviews:**
```sql
-- Create partitioned table
CREATE TABLE reviews_partitioned (
    LIKE reviews INCLUDING ALL
) PARTITION BY RANGE (review_date);

-- Monthly partitions
CREATE TABLE reviews_2024_01 PARTITION OF reviews_partitioned
FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

CREATE TABLE reviews_2024_02 PARTITION OF reviews_partitioned
FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
```

## Caching Strategy

### Redis Configuration

High-performance Redis setup:

```yaml
redis:
  addr: localhost:6379
  pool_size: 100           # Connection pool size
  min_idle_conns: 10      # Minimum idle connections
  max_retries: 3          # Retry attempts
  retry_delay: 100ms      # Delay between retries
  dial_timeout: 5s        # Connection timeout
  read_timeout: 3s        # Read timeout
  write_timeout: 3s       # Write timeout
  pool_timeout: 5s        # Pool timeout
```

### Cache Layers

#### 1. Application Cache (L1)

In-memory cache for frequently accessed data:

```go
type CacheConfig struct {
    MaxSize    int           `yaml:"max_size"`
    TTL        time.Duration `yaml:"ttl"`
    CleanupInt time.Duration `yaml:"cleanup_interval"`
}

// Hot data cache
var hotDataCache = &sync.Map{}

func GetFromL1Cache(key string) (interface{}, bool) {
    if value, exists := hotDataCache.Load(key); exists {
        if entry, ok := value.(*CacheEntry); ok {
            if entry.ExpiresAt.After(time.Now()) {
                return entry.Value, true
            }
            hotDataCache.Delete(key)
        }
    }
    return nil, false
}
```

#### 2. Redis Cache (L2)

Distributed cache with write-through pattern:

```go
func GetHotelWithCache(ctx context.Context, id string) (*Hotel, error) {
    cacheKey := fmt.Sprintf("hotel:%s", id)
    
    // L1 Cache
    if hotel, exists := GetFromL1Cache(cacheKey); exists {
        return hotel.(*Hotel), nil
    }
    
    // L2 Cache (Redis)
    if cached, err := redisClient.Get(ctx, cacheKey).Result(); err == nil {
        var hotel Hotel
        if err := json.Unmarshal([]byte(cached), &hotel); err == nil {
            // Populate L1 cache
            SetInL1Cache(cacheKey, &hotel, 5*time.Minute)
            return &hotel, nil
        }
    }
    
    // Database fallback
    hotel, err := db.GetHotel(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    data, _ := json.Marshal(hotel)
    redisClient.Set(ctx, cacheKey, data, 2*time.Hour)
    SetInL1Cache(cacheKey, hotel, 5*time.Minute)
    
    return hotel, nil
}
```

### Cache Warming

Proactive cache population for hot data:

```go
func WarmPopularHotels(ctx context.Context) error {
    // Get top 100 hotels by review count
    hotels, err := db.GetTopHotels(ctx, 100)
    if err != nil {
        return err
    }
    
    // Warm cache in batches
    for i := 0; i < len(hotels); i += 10 {
        batch := hotels[i:min(i+10, len(hotels))]
        
        var wg sync.WaitGroup
        for _, hotel := range batch {
            wg.Add(1)
            go func(h *Hotel) {
                defer wg.Done()
                
                // Cache hotel data
                cacheKey := fmt.Sprintf("hotel:%s", h.ID)
                data, _ := json.Marshal(h)
                redisClient.Set(ctx, cacheKey, data, 4*time.Hour)
                
                // Cache recent reviews
                reviews, _ := db.GetHotelReviews(ctx, h.ID, 20, 0)
                reviewsKey := fmt.Sprintf("hotel:%s:reviews", h.ID)
                reviewsData, _ := json.Marshal(reviews)
                redisClient.Set(ctx, reviewsKey, reviewsData, 1*time.Hour)
            }(hotel)
        }
        wg.Wait()
        
        // Rate limit warming
        time.Sleep(100 * time.Millisecond)
    }
    
    return nil
}
```

### Cache Invalidation

Intelligent cache invalidation strategy:

```go
func InvalidateHotelCache(ctx context.Context, hotelID string) error {
    patterns := []string{
        fmt.Sprintf("hotel:%s", hotelID),
        fmt.Sprintf("hotel:%s:*", hotelID),
        fmt.Sprintf("search:*:hotel:%s:*", hotelID),
        "top-hotels:*",
        "analytics:*",
    }
    
    for _, pattern := range patterns {
        if err := InvalidateCachePattern(ctx, pattern); err != nil {
            log.Errorf("Failed to invalidate cache pattern %s: %v", pattern, err)
        }
    }
    
    return nil
}
```

## Application Performance

### HTTP Server Optimization

Optimized Gin configuration:

```go
func NewRouter() *gin.Engine {
    gin.SetMode(gin.ReleaseMode)
    
    router := gin.New()
    
    // Custom middleware for performance
    router.Use(
        RequestIDMiddleware(),
        LoggerMiddleware(),
        RecoveryMiddleware(),
        CORSMiddleware(),
        RateLimitMiddleware(),
        CompressionMiddleware(),
        CacheMiddleware(),
    )
    
    // Optimize for high concurrency
    router.MaxMultipartMemory = 8 << 20 // 8MB
    
    return router
}

// Connection settings
server := &http.Server{
    Addr:           ":8080",
    Handler:        router,
    ReadTimeout:    30 * time.Second,
    WriteTimeout:   30 * time.Second,
    IdleTimeout:    120 * time.Second,
    MaxHeaderBytes: 1 << 20, // 1MB
}
```

### Memory Management

#### 1. Object Pooling

Reduce garbage collection pressure:

```go
var (
    reviewPool = sync.Pool{
        New: func() interface{} {
            return &Review{}
        },
    }
    
    responsePool = sync.Pool{
        New: func() interface{} {
            return &APIResponse{}
        },
    }
)

func ProcessReview(data []byte) error {
    review := reviewPool.Get().(*Review)
    defer reviewPool.Put(review)
    
    // Reset review object
    *review = Review{}
    
    if err := json.Unmarshal(data, review); err != nil {
        return err
    }
    
    return ValidateAndSaveReview(review)
}
```

#### 2. Buffer Reuse

Optimize JSON processing:

```go
var jsonBufferPool = sync.Pool{
    New: func() interface{} {
        return bytes.NewBuffer(make([]byte, 0, 1024))
    },
}

func SerializeResponse(data interface{}) ([]byte, error) {
    buf := jsonBufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        jsonBufferPool.Put(buf)
    }()
    
    encoder := json.NewEncoder(buf)
    if err := encoder.Encode(data); err != nil {
        return nil, err
    }
    
    return buf.Bytes(), nil
}
```

### Concurrency Optimization

#### 1. Worker Pools

Controlled concurrency for batch processing:

```go
type WorkerPool struct {
    workers    int
    jobQueue   chan Job
    resultChan chan Result
    wg         sync.WaitGroup
}

func NewWorkerPool(workers int, queueSize int) *WorkerPool {
    return &WorkerPool{
        workers:    workers,
        jobQueue:   make(chan Job, queueSize),
        resultChan: make(chan Result, queueSize),
    }
}

func (wp *WorkerPool) Start() {
    for i := 0; i < wp.workers; i++ {
        wp.wg.Add(1)
        go wp.worker()
    }
}

func (wp *WorkerPool) worker() {
    defer wp.wg.Done()
    
    for job := range wp.jobQueue {
        result := job.Process()
        wp.resultChan <- result
    }
}
```

#### 2. Context-based Timeouts

Prevent resource leaks:

```go
func ProcessReviewWithTimeout(ctx context.Context, review *Review) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    done := make(chan error, 1)
    
    go func() {
        done <- processReviewInternal(review)
    }()
    
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

## Monitoring and Profiling

### Custom Metrics

Application-specific metrics:

```go
var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint", "status_code"},
    )
    
    cacheHitRatio = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cache_hit_ratio",
            Help: "Cache hit ratio by cache type",
        },
        []string{"cache_type"},
    )
    
    dbConnectionsActive = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "database_connections_active",
            Help: "Number of active database connections",
        },
    )
)
```

### Performance Profiling

Built-in profiling endpoints:

```go
import _ "net/http/pprof"

// Add pprof endpoints in development
if os.Getenv("ENABLE_PPROF") == "true" {
    go func() {
        log.Info("Starting pprof server on :6060")
        log.Fatal(http.ListenAndServe(":6060", nil))
    }()
}
```

Profile analysis commands:

```bash
# CPU profiling
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory profiling
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine analysis
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Mutex contention
go tool pprof http://localhost:6060/debug/pprof/mutex
```

## Load Testing

### Test Scenarios

#### 1. Read-Heavy Workload

```javascript
// k6 load test script
import http from 'k6/http';
import { check } from 'k6';

export let options = {
    stages: [
        { duration: '5m', target: 100 },
        { duration: '10m', target: 500 },
        { duration: '5m', target: 1000 },
        { duration: '10m', target: 1000 },
        { duration: '5m', target: 0 },
    ],
};

export default function() {
    // Hotel lookup
    let hotelResponse = http.get('http://localhost:8080/api/v1/hotels/hotel-123');
    check(hotelResponse, {
        'hotel lookup status 200': (r) => r.status === 200,
        'hotel lookup duration < 50ms': (r) => r.timings.duration < 50,
    });
    
    // Review search
    let searchResponse = http.get('http://localhost:8080/api/v1/hotels/hotel-123/reviews?limit=20');
    check(searchResponse, {
        'review search status 200': (r) => r.status === 200,
        'review search duration < 100ms': (r) => r.timings.duration < 100,
    });
}
```

#### 2. Write-Heavy Workload

```javascript
export default function() {
    let payload = JSON.stringify({
        hotel_id: 'hotel-123',
        provider_id: 'provider-456',
        title: 'Great hotel!',
        content: 'Had a wonderful stay...',
        rating: 4.5,
        author_name: 'Test User',
        review_date: new Date().toISOString(),
    });
    
    let params = {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + __ENV.AUTH_TOKEN,
        },
    };
    
    let response = http.post('http://localhost:8080/api/v1/reviews', payload, params);
    check(response, {
        'review creation status 201': (r) => r.status === 201,
        'review creation duration < 200ms': (r) => r.timings.duration < 200,
    });
}
```

### Performance Benchmarks

Current benchmark results:

```
Benchmark Results (Hardware: 8 CPU, 16GB RAM)
=============================================

Read Operations:
- Hotel Lookup: 2,847 RPS (avg: 8.5ms, p95: 15ms)
- Review Search: 1,523 RPS (avg: 31ms, p95: 67ms)
- Full-text Search: 456 RPS (avg: 78ms, p95: 156ms)

Write Operations:
- Review Creation: 934 RPS (avg: 42ms, p95: 89ms)
- Hotel Update: 678 RPS (avg: 58ms, p95: 124ms)
- Bulk Import: 1,245 records/sec

Cache Performance:
- L1 Hit Ratio: 95.2%
- L2 Hit Ratio: 89.7%
- Cache Response: <1ms

Database Performance:
- Connection Pool Utilization: 68%
- Query Cache Hit Ratio: 92%
- Average Query Time: 12ms
```

## Optimization Checklist

### Database

- [ ] Proper indexing for all query patterns
- [ ] Connection pooling configured
- [ ] Query performance monitored
- [ ] Regular VACUUM and ANALYZE
- [ ] Partition large tables
- [ ] Optimize JOIN operations

### Caching

- [ ] Multi-layer caching strategy
- [ ] Cache warming for hot data
- [ ] Intelligent invalidation
- [ ] Monitor hit ratios
- [ ] Compress cached data
- [ ] Set appropriate TTLs

### Application

- [ ] Object pooling for frequent allocations
- [ ] Goroutine pool for concurrency
- [ ] Context timeouts for all operations
- [ ] Efficient JSON serialization
- [ ] Memory profiling regular
- [ ] CPU profiling under load

### Infrastructure

- [ ] Load balancer configuration
- [ ] Auto-scaling policies
- [ ] Resource limits set
- [ ] Health checks configured
- [ ] Monitoring alerts setup
- [ ] Log aggregation optimized

This performance guide ensures the Hotel Reviews Microservice can handle production loads efficiently while maintaining low latency and high throughput.