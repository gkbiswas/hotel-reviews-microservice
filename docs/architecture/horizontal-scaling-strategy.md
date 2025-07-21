# Horizontal Scaling Strategy - Hotel Reviews Platform

## Overview

This document outlines the comprehensive horizontal scaling strategy for the Hotel Reviews platform, covering auto-scaling mechanisms, load balancing, distributed caching, and performance optimization patterns.

## Scaling Architecture

### 1. Application Layer Scaling

#### Kubernetes Horizontal Pod Autoscaler (HPA)

```yaml
# Enhanced HPA with multiple metrics
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: hotel-service-hpa
  namespace: hotel-reviews-production
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hotel-service
  minReplicas: 5
  maxReplicas: 50
  metrics:
  # CPU-based scaling
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 60
  # Memory-based scaling
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 70
  # Custom metrics scaling
  - type: Pods
    pods:
      metric:
        name: http_requests_per_second
      target:
        type: AverageValue
        averageValue: "100"
  # External metrics scaling
  - type: External
    external:
      metric:
        name: queue_depth
        selector:
          matchLabels:
            queue: hotel-processing
      target:
        type: AverageValue
        averageValue: "10"
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
      - type: Pods
        value: 4
        periodSeconds: 60
      selectPolicy: Max
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
      - type: Pods
        value: 2
        periodSeconds: 60
      selectPolicy: Min
```

#### Vertical Pod Autoscaler (VPA)

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: hotel-service-vpa
  namespace: hotel-reviews-production
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hotel-service
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: hotel-service
      maxAllowed:
        cpu: "4"
        memory: "8Gi"
      minAllowed:
        cpu: "100m"
        memory: "128Mi"
      controlledResources: ["cpu", "memory"]
      controlledValues: RequestsAndLimits
```

### 2. Database Scaling Patterns

#### Read Replicas Strategy

```go
// Database connection pool with read/write splitting
type ScalableDatabase struct {
    primary   *sql.DB
    replicas  []*sql.DB
    roundRobin int32
    mu        sync.RWMutex
}

func NewScalableDatabase(primaryDSN string, replicaDSNs []string) (*ScalableDatabase, error) {
    primary, err := sql.Open("postgres", primaryDSN)
    if err != nil {
        return nil, err
    }
    
    var replicas []*sql.DB
    for _, dsn := range replicaDSNs {
        replica, err := sql.Open("postgres", dsn)
        if err != nil {
            return nil, err
        }
        replicas = append(replicas, replica)
    }
    
    return &ScalableDatabase{
        primary:  primary,
        replicas: replicas,
    }, nil
}

func (db *ScalableDatabase) WriteDB() *sql.DB {
    return db.primary
}

func (db *ScalableDatabase) ReadDB() *sql.DB {
    if len(db.replicas) == 0 {
        return db.primary
    }
    
    // Round-robin load balancing
    index := atomic.AddInt32(&db.roundRobin, 1) % int32(len(db.replicas))
    return db.replicas[index]
}

// Repository with read/write splitting
type ScalableHotelRepository struct {
    db     *ScalableDatabase
    cache  CacheService
    logger *slog.Logger
}

func (r *ScalableHotelRepository) GetHotel(ctx context.Context, id string) (*Hotel, error) {
    // Try cache first
    if hotel, err := r.cache.Get(ctx, "hotel:"+id); err == nil {
        return hotel.(*Hotel), nil
    }
    
    // Use read replica for queries
    db := r.db.ReadDB()
    
    var hotel Hotel
    err := db.QueryRowContext(ctx, 
        "SELECT id, name, address, rating FROM hotels WHERE id = $1", id).
        Scan(&hotel.ID, &hotel.Name, &hotel.Address, &hotel.Rating)
    
    if err != nil {
        return nil, err
    }
    
    // Cache the result
    r.cache.Set(ctx, "hotel:"+id, &hotel, 15*time.Minute)
    
    return &hotel, nil
}

func (r *ScalableHotelRepository) CreateHotel(ctx context.Context, hotel *Hotel) error {
    // Use primary for writes
    db := r.db.WriteDB()
    
    _, err := db.ExecContext(ctx,
        "INSERT INTO hotels (id, name, address, rating) VALUES ($1, $2, $3, $4)",
        hotel.ID, hotel.Name, hotel.Address, hotel.Rating)
    
    if err != nil {
        return err
    }
    
    // Invalidate cache
    r.cache.Delete(ctx, "hotel:"+hotel.ID)
    
    return nil
}
```

#### Database Sharding Strategy

```go
// Database sharding based on hotel ID
type ShardedDatabase struct {
    shards map[string]*sql.DB
    hasher hash.Hash32
}

func NewShardedDatabase(shardConfigs map[string]string) (*ShardedDatabase, error) {
    shards := make(map[string]*sql.DB)
    
    for shardName, dsn := range shardConfigs {
        db, err := sql.Open("postgres", dsn)
        if err != nil {
            return nil, err
        }
        shards[shardName] = db
    }
    
    return &ShardedDatabase{
        shards: shards,
        hasher: fnv.New32a(),
    }, nil
}

func (db *ShardedDatabase) GetShard(key string) *sql.DB {
    db.hasher.Reset()
    db.hasher.Write([]byte(key))
    hash := db.hasher.Sum32()
    
    shardIndex := hash % uint32(len(db.shards))
    shardName := fmt.Sprintf("shard_%d", shardIndex)
    
    return db.shards[shardName]
}

type ShardedHotelRepository struct {
    db     *ShardedDatabase
    cache  CacheService
    logger *slog.Logger
}

func (r *ShardedHotelRepository) GetHotel(ctx context.Context, id string) (*Hotel, error) {
    shard := r.db.GetShard(id)
    
    var hotel Hotel
    err := shard.QueryRowContext(ctx,
        "SELECT id, name, address, rating FROM hotels WHERE id = $1", id).
        Scan(&hotel.ID, &hotel.Name, &hotel.Address, &hotel.Rating)
    
    return &hotel, err
}
```

### 3. Caching Layer Scaling

#### Distributed Redis Cluster

```go
// Redis cluster client for horizontal scaling
type DistributedCache struct {
    cluster    *redis.ClusterClient
    localCache *ristretto.Cache
    logger     *slog.Logger
}

func NewDistributedCache(addrs []string) (*DistributedCache, error) {
    // Redis cluster configuration
    cluster := redis.NewClusterClient(&redis.ClusterOptions{
        Addrs:              addrs,
        MaxRedirects:       3,
        ReadOnly:          true,
        RouteByLatency:    true,
        RouteRandomly:     true,
        PoolSize:          100,
        MinIdleConns:      20,
        MaxConnAge:        time.Hour,
        PoolTimeout:       30 * time.Second,
        IdleTimeout:       5 * time.Minute,
        IdleCheckFrequency: time.Minute,
    })
    
    // Local L1 cache
    localCache, err := ristretto.NewCache(&ristretto.Config{
        NumCounters: 1e7,     // 10M counters
        MaxCost:     1 << 30, // 1GB
        BufferItems: 64,
    })
    if err != nil {
        return nil, err
    }
    
    return &DistributedCache{
        cluster:    cluster,
        localCache: localCache,
    }, nil
}

func (c *DistributedCache) Get(ctx context.Context, key string) (interface{}, error) {
    // Check L1 cache first
    if value, found := c.localCache.Get(key); found {
        return value, nil
    }
    
    // Check L2 cache (Redis)
    result, err := c.cluster.Get(ctx, key).Result()
    if err != nil {
        if err == redis.Nil {
            return nil, ErrNotFound
        }
        return nil, err
    }
    
    // Store in L1 cache
    c.localCache.Set(key, result, 1)
    
    return result, nil
}

func (c *DistributedCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    // Set in both L1 and L2 caches
    c.localCache.Set(key, value, 1)
    
    return c.cluster.Set(ctx, key, value, ttl).Err()
}

func (c *DistributedCache) Delete(ctx context.Context, key string) error {
    // Delete from both caches
    c.localCache.Del(key)
    
    return c.cluster.Del(ctx, key).Err()
}
```

#### Cache Warming Strategy

```go
// Cache warming service for predictive loading
type CacheWarmingService struct {
    cache      CacheService
    repository HotelRepository
    predictor  *UsagePredictor
    logger     *slog.Logger
}

type UsagePredictor struct {
    analytics AnalyticsService
    ml        MLService
}

func (s *CacheWarmingService) WarmPopularHotels(ctx context.Context) error {
    // Get popular hotels from analytics
    popularHotels, err := s.predictor.analytics.GetPopularHotels(ctx, 100)
    if err != nil {
        return err
    }
    
    // Warm cache concurrently
    semaphore := make(chan struct{}, 10) // Limit concurrency
    var wg sync.WaitGroup
    
    for _, hotelID := range popularHotels {
        wg.Add(1)
        go func(id string) {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            s.warmHotelData(ctx, id)
        }(hotelID)
    }
    
    wg.Wait()
    return nil
}

func (s *CacheWarmingService) warmHotelData(ctx context.Context, hotelID string) {
    // Warm hotel basic info
    hotel, err := s.repository.GetHotel(ctx, hotelID)
    if err != nil {
        s.logger.Error("Failed to warm hotel data", "hotel_id", hotelID, "error", err)
        return
    }
    
    s.cache.Set(ctx, "hotel:"+hotelID, hotel, time.Hour)
    
    // Warm related data
    reviews, _ := s.repository.GetHotelReviews(ctx, hotelID, 10, 0)
    s.cache.Set(ctx, fmt.Sprintf("hotel:%s:reviews", hotelID), reviews, 30*time.Minute)
    
    metrics, _ := s.repository.GetHotelMetrics(ctx, hotelID)
    s.cache.Set(ctx, fmt.Sprintf("hotel:%s:metrics", hotelID), metrics, time.Hour)
}
```

### 4. Load Balancing Strategies

#### Intelligent Load Balancer

```go
// Weighted round-robin load balancer with health checking
type IntelligentLoadBalancer struct {
    backends []Backend
    weights  []int
    current  int32
    mu       sync.RWMutex
}

type Backend struct {
    URL         string
    Weight      int
    HealthScore float64
    LastCheck   time.Time
    Client      *http.Client
}

func (lb *IntelligentLoadBalancer) SelectBackend() *Backend {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    if len(lb.backends) == 0 {
        return nil
    }
    
    // Weighted round-robin with health consideration
    totalWeight := 0
    for i, backend := range lb.backends {
        adjustedWeight := int(float64(backend.Weight) * backend.HealthScore)
        totalWeight += adjustedWeight
        lb.weights[i] = adjustedWeight
    }
    
    if totalWeight == 0 {
        return &lb.backends[0] // Fallback
    }
    
    current := atomic.AddInt32(&lb.current, 1)
    target := int(current) % totalWeight
    
    cumulative := 0
    for i, weight := range lb.weights {
        cumulative += weight
        if target < cumulative {
            return &lb.backends[i]
        }
    }
    
    return &lb.backends[0] // Fallback
}

func (lb *IntelligentLoadBalancer) HealthCheck(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            lb.performHealthChecks(ctx)
        }
    }
}

func (lb *IntelligentLoadBalancer) performHealthChecks(ctx context.Context) {
    var wg sync.WaitGroup
    
    for i := range lb.backends {
        wg.Add(1)
        go func(index int) {
            defer wg.Done()
            lb.checkBackendHealth(ctx, index)
        }(i)
    }
    
    wg.Wait()
}

func (lb *IntelligentLoadBalancer) checkBackendHealth(ctx context.Context, index int) {
    backend := &lb.backends[index]
    
    start := time.Now()
    resp, err := backend.Client.Get(backend.URL + "/health")
    latency := time.Since(start)
    
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    if err != nil || resp.StatusCode != 200 {
        backend.HealthScore = math.Max(0, backend.HealthScore-0.1)
    } else {
        // Health score based on latency
        if latency < 100*time.Millisecond {
            backend.HealthScore = math.Min(1.0, backend.HealthScore+0.1)
        } else if latency < 500*time.Millisecond {
            backend.HealthScore = math.Min(1.0, backend.HealthScore+0.05)
        } else {
            backend.HealthScore = math.Max(0.5, backend.HealthScore-0.05)
        }
    }
    
    backend.LastCheck = time.Now()
    
    if resp != nil {
        resp.Body.Close()
    }
}
```

### 5. Queue-Based Scaling

#### Dynamic Worker Scaling

```go
// Auto-scaling worker pool based on queue depth
type AutoScalingWorkerPool struct {
    queue       chan Job
    workers     []*Worker
    minWorkers  int
    maxWorkers  int
    scaleUp     time.Duration
    scaleDown   time.Duration
    lastScale   time.Time
    mu          sync.RWMutex
    ctx         context.Context
    cancel      context.CancelFunc
}

type Job interface {
    Execute(ctx context.Context) error
}

type Worker struct {
    id     int
    jobs   chan Job
    quit   chan bool
    active bool
}

func NewAutoScalingWorkerPool(minWorkers, maxWorkers int) *AutoScalingWorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    
    pool := &AutoScalingWorkerPool{
        queue:      make(chan Job, 1000),
        minWorkers: minWorkers,
        maxWorkers: maxWorkers,
        scaleUp:    30 * time.Second,
        scaleDown:  2 * time.Minute,
        ctx:        ctx,
        cancel:     cancel,
    }
    
    // Start with minimum workers
    for i := 0; i < minWorkers; i++ {
        pool.addWorker()
    }
    
    // Start scaling monitor
    go pool.monitor()
    
    return pool
}

func (p *AutoScalingWorkerPool) Submit(job Job) {
    select {
    case p.queue <- job:
    default:
        // Queue is full, consider scaling up
        p.considerScaling()
    }
}

func (p *AutoScalingWorkerPool) monitor() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-p.ctx.Done():
            return
        case <-ticker.C:
            p.autoScale()
        }
    }
}

func (p *AutoScalingWorkerPool) autoScale() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    queueLength := len(p.queue)
    activeWorkers := p.countActiveWorkers()
    
    // Scale up conditions
    if queueLength > activeWorkers*2 && 
       len(p.workers) < p.maxWorkers &&
       time.Since(p.lastScale) > p.scaleUp {
        
        p.addWorker()
        p.lastScale = time.Now()
        return
    }
    
    // Scale down conditions
    if queueLength < activeWorkers/2 && 
       len(p.workers) > p.minWorkers &&
       time.Since(p.lastScale) > p.scaleDown {
        
        p.removeWorker()
        p.lastScale = time.Now()
    }
}

func (p *AutoScalingWorkerPool) addWorker() {
    worker := &Worker{
        id:     len(p.workers),
        jobs:   make(chan Job),
        quit:   make(chan bool),
        active: true,
    }
    
    p.workers = append(p.workers, worker)
    
    go func() {
        for {
            select {
            case job := <-p.queue:
                worker.jobs <- job
            case job := <-worker.jobs:
                job.Execute(p.ctx)
            case <-worker.quit:
                worker.active = false
                return
            case <-p.ctx.Done():
                return
            }
        }
    }()
}

func (p *AutoScalingWorkerPool) removeWorker() {
    if len(p.workers) <= p.minWorkers {
        return
    }
    
    // Find least active worker
    workerIndex := len(p.workers) - 1
    worker := p.workers[workerIndex]
    
    worker.quit <- true
    p.workers = p.workers[:workerIndex]
}

func (p *AutoScalingWorkerPool) countActiveWorkers() int {
    count := 0
    for _, worker := range p.workers {
        if worker.active {
            count++
        }
    }
    return count
}
```

### 6. Content Delivery Network (CDN) Scaling

#### CDN Integration Strategy

```go
// CDN-aware file service with automatic cache warming
type CDNFileService struct {
    storage       StorageService
    cdn           CDNService
    cache         CacheService
    logger        *slog.Logger
    warmingQueue  chan string
}

type CDNService interface {
    PurgeCache(ctx context.Context, urls []string) error
    WarmCache(ctx context.Context, urls []string) error
    GetCacheStatus(ctx context.Context, url string) (*CacheStatus, error)
}

type CacheStatus struct {
    URL        string
    CacheHit   bool
    TTL        time.Duration
    EdgeServer string
}

func (s *CDNFileService) UploadFile(ctx context.Context, file *File) (*FileInfo, error) {
    // Upload to primary storage
    info, err := s.storage.Upload(ctx, file)
    if err != nil {
        return nil, err
    }
    
    // Warm CDN cache asynchronously
    select {
    case s.warmingQueue <- info.URL:
    default:
        // Queue is full, skip warming
    }
    
    return info, nil
}

func (s *CDNFileService) GetFile(ctx context.Context, fileID string) (*FileInfo, error) {
    // Check local cache first
    if info, err := s.cache.Get(ctx, "file:"+fileID); err == nil {
        return info.(*FileInfo), nil
    }
    
    // Get from storage
    info, err := s.storage.GetFileInfo(ctx, fileID)
    if err != nil {
        return nil, err
    }
    
    // Cache the file info
    s.cache.Set(ctx, "file:"+fileID, info, time.Hour)
    
    // Check if CDN cache needs warming
    status, err := s.cdn.GetCacheStatus(ctx, info.URL)
    if err == nil && !status.CacheHit {
        select {
        case s.warmingQueue <- info.URL:
        default:
        }
    }
    
    return info, nil
}

func (s *CDNFileService) startCacheWarming(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case url := <-s.warmingQueue:
            if err := s.cdn.WarmCache(ctx, []string{url}); err != nil {
                s.logger.Error("Failed to warm CDN cache", "url", url, "error", err)
            }
        }
    }
}
```

### 7. Database Connection Pool Scaling

#### Dynamic Connection Pool

```go
// Dynamic database connection pool that scales based on load
type DynamicConnectionPool struct {
    config      PoolConfig
    connections chan *sql.DB
    active      int32
    total       int32
    mu          sync.RWMutex
    metrics     *PoolMetrics
}

type PoolConfig struct {
    MinConnections     int
    MaxConnections     int
    ScaleUpThreshold   float64
    ScaleDownThreshold float64
    IdleTimeout        time.Duration
    MaxLifetime        time.Duration
}

type PoolMetrics struct {
    ActiveConnections   int32
    TotalConnections    int32
    WaitingRequests     int32
    AverageWaitTime     time.Duration
    ConnectionErrors    int32
}

func NewDynamicConnectionPool(dsn string, config PoolConfig) (*DynamicConnectionPool, error) {
    pool := &DynamicConnectionPool{
        config:      config,
        connections: make(chan *sql.DB, config.MaxConnections),
        metrics:     &PoolMetrics{},
    }
    
    // Create initial connections
    for i := 0; i < config.MinConnections; i++ {
        if err := pool.addConnection(dsn); err != nil {
            return nil, err
        }
    }
    
    // Start monitoring
    go pool.monitor()
    
    return pool, nil
}

func (p *DynamicConnectionPool) GetConnection(ctx context.Context) (*sql.DB, error) {
    atomic.AddInt32(&p.metrics.WaitingRequests, 1)
    defer atomic.AddInt32(&p.metrics.WaitingRequests, -1)
    
    start := time.Now()
    
    select {
    case conn := <-p.connections:
        atomic.AddInt32(&p.active, 1)
        
        // Update wait time metrics
        waitTime := time.Since(start)
        p.updateAverageWaitTime(waitTime)
        
        return conn, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-time.After(5 * time.Second):
        // Trigger scaling if waiting too long
        p.considerScaling()
        return nil, fmt.Errorf("connection timeout")
    }
}

func (p *DynamicConnectionPool) ReturnConnection(conn *sql.DB) {
    atomic.AddInt32(&p.active, -1)
    
    select {
    case p.connections <- conn:
    default:
        // Pool is full, close connection
        conn.Close()
        atomic.AddInt32(&p.total, -1)
    }
}

func (p *DynamicConnectionPool) monitor() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        p.autoScale()
        p.cleanupIdleConnections()
    }
}

func (p *DynamicConnectionPool) autoScale() {
    active := atomic.LoadInt32(&p.active)
    total := atomic.LoadInt32(&p.total)
    waiting := atomic.LoadInt32(&p.metrics.WaitingRequests)
    
    utilizationRate := float64(active) / float64(total)
    
    // Scale up if utilization is high or requests are waiting
    if (utilizationRate > p.config.ScaleUpThreshold || waiting > 0) && 
       int(total) < p.config.MaxConnections {
        p.scaleUp()
    }
    
    // Scale down if utilization is low
    if utilizationRate < p.config.ScaleDownThreshold && 
       int(total) > p.config.MinConnections {
        p.scaleDown()
    }
}

func (p *DynamicConnectionPool) scaleUp() {
    // Implementation for adding connections
}

func (p *DynamicConnectionPool) scaleDown() {
    // Implementation for removing connections
}
```

### 8. Monitoring and Metrics

#### Scaling Metrics Collection

```go
// Comprehensive scaling metrics collector
type ScalingMetricsCollector struct {
    registry   prometheus.Registerer
    
    // HPA metrics
    currentReplicas  prometheus.Gauge
    desiredReplicas  prometheus.Gauge
    cpuUtilization   prometheus.Gauge
    memoryUtilization prometheus.Gauge
    
    // Queue metrics
    queueDepth       prometheus.Gauge
    processingRate   prometheus.Gauge
    workerCount      prometheus.Gauge
    
    // Cache metrics
    cacheHitRate     prometheus.Gauge
    cacheSize        prometheus.Gauge
    evictionRate     prometheus.Gauge
    
    // Database metrics
    dbConnections    prometheus.Gauge
    dbUtilization    prometheus.Gauge
    queryLatency     prometheus.Histogram
}

func NewScalingMetricsCollector(registry prometheus.Registerer) *ScalingMetricsCollector {
    collector := &ScalingMetricsCollector{
        registry: registry,
        
        currentReplicas: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_current_replicas",
            Help: "Current number of replicas",
        }),
        
        desiredReplicas: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_desired_replicas",
            Help: "Desired number of replicas",
        }),
        
        cpuUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_cpu_utilization_percent",
            Help: "Current CPU utilization percentage",
        }),
        
        memoryUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_memory_utilization_percent",
            Help: "Current memory utilization percentage",
        }),
        
        queueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_queue_depth",
            Help: "Current queue depth",
        }),
        
        processingRate: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_processing_rate",
            Help: "Current processing rate (jobs/second)",
        }),
        
        workerCount: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_worker_count",
            Help: "Current number of workers",
        }),
        
        cacheHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_cache_hit_rate",
            Help: "Cache hit rate percentage",
        }),
        
        cacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_cache_size_bytes",
            Help: "Current cache size in bytes",
        }),
        
        evictionRate: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_cache_eviction_rate",
            Help: "Cache eviction rate (evictions/second)",
        }),
        
        dbConnections: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_db_connections",
            Help: "Current database connections",
        }),
        
        dbUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "scaling_db_utilization_percent",
            Help: "Database utilization percentage",
        }),
        
        queryLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name: "scaling_db_query_duration_seconds",
            Help: "Database query latency",
            Buckets: prometheus.DefBuckets,
        }),
    }
    
    // Register all metrics
    registry.MustRegister(
        collector.currentReplicas,
        collector.desiredReplicas,
        collector.cpuUtilization,
        collector.memoryUtilization,
        collector.queueDepth,
        collector.processingRate,
        collector.workerCount,
        collector.cacheHitRate,
        collector.cacheSize,
        collector.evictionRate,
        collector.dbConnections,
        collector.dbUtilization,
        collector.queryLatency,
    )
    
    return collector
}

func (c *ScalingMetricsCollector) UpdateHPAMetrics(current, desired int, cpuPercent, memoryPercent float64) {
    c.currentReplicas.Set(float64(current))
    c.desiredReplicas.Set(float64(desired))
    c.cpuUtilization.Set(cpuPercent)
    c.memoryUtilization.Set(memoryPercent)
}

func (c *ScalingMetricsCollector) UpdateQueueMetrics(depth int, rate float64, workers int) {
    c.queueDepth.Set(float64(depth))
    c.processingRate.Set(rate)
    c.workerCount.Set(float64(workers))
}

func (c *ScalingMetricsCollector) UpdateCacheMetrics(hitRate, sizeBytes, evictionRate float64) {
    c.cacheHitRate.Set(hitRate)
    c.cacheSize.Set(sizeBytes)
    c.evictionRate.Set(evictionRate)
}

func (c *ScalingMetricsCollector) UpdateDatabaseMetrics(connections int, utilization float64) {
    c.dbConnections.Set(float64(connections))
    c.dbUtilization.Set(utilization)
}

func (c *ScalingMetricsCollector) RecordQueryLatency(duration time.Duration) {
    c.queryLatency.Observe(duration.Seconds())
}
```

## Scaling Best Practices

### 1. Gradual Scaling
- Implement gradual scale-up to avoid overwhelming dependencies
- Use stabilization windows to prevent thrashing
- Set reasonable scaling velocity limits

### 2. Circuit Breakers
- Implement circuit breakers to protect downstream services
- Use bulkhead patterns to isolate critical components
- Implement graceful degradation under load

### 3. Load Testing
- Regular load testing to validate scaling behavior
- Chaos engineering to test failure scenarios
- Performance testing of auto-scaling triggers

### 4. Cost Optimization
- Monitor scaling costs and set budget alerts
- Use spot instances for non-critical workloads
- Implement predictive scaling for known patterns

### 5. Monitoring and Alerting
- Monitor scaling events and their effectiveness
- Alert on scaling failures or unusual patterns
- Track business metrics during scaling events

This horizontal scaling strategy provides comprehensive coverage for scaling the Hotel Reviews platform efficiently and cost-effectively.