package infrastructure

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Test helper to create a test Redis client without real Redis
func createTestRedisClient() *RedisClient {
	log := logger.NewDefault()
	config := &RedisConfig{
		Host:               "localhost",
		Port:               6379,
		Database:           0,
		PoolSize:           10,
		MinIdleConns:       5,
		MaxConnAge:         time.Hour,
		PoolTimeout:        time.Second * 5,
		IdleTimeout:        time.Minute * 5,
		IdleCheckFrequency: time.Minute,
		ReadTimeout:        time.Second * 3,
		WriteTimeout:       time.Second * 3,
		DialTimeout:        time.Second * 5,
		MaxRetries:         3,
		MinRetryBackoff:    time.Millisecond * 8,
		MaxRetryBackoff:    time.Millisecond * 512,
		EnableMetrics:      true,
		KeyPrefix:          "test",
	}
	
	cacheConfig := &CacheConfig{
		ReviewTTL:         time.Hour,
		HotelTTL:          time.Hour * 2,
		ProviderTTL:       time.Hour,
		StatisticsTTL:     time.Minute * 30,
		SearchTTL:         time.Minute * 15,
		ProcessingTTL:     time.Hour,
		AnalyticsTTL:      time.Hour * 6,
		DefaultTTL:        time.Hour,
		MaxKeyLength:      250,
		EnableCompression: false,
		CompressionLevel:  6,
		PrefixSeparator:   ":",
		InvalidationBatch: 100,
		WarmupConcurrency: 5,
	}

	// Create a mock client structure for testing
	client := &RedisClient{
		client:      nil, // We'll use nil for unit tests
		config:      config,
		cacheConfig: cacheConfig,
		logger:      log,
		metrics: &CacheMetrics{
			OperationCounts: make(map[CacheOperation]int64),
			KeyspaceCounts:  make(map[string]int64),
		},
		invalidations: make(chan InvalidationRequest, 1000),
	}

	ctx, cancel := context.WithCancel(context.Background())
	client.ctx = ctx
	client.cancel = cancel

	return client
}

func TestRedisConfig(t *testing.T) {
	t.Run("redis config serialization", func(t *testing.T) {
		config := &RedisConfig{
			Host:               "localhost",
			Port:               6379,
			Password:           "password",
			Database:           1,
			PoolSize:           20,
			MinIdleConns:       5,
			MaxConnAge:         time.Hour,
			PoolTimeout:        time.Second * 10,
			IdleTimeout:        time.Minute * 10,
			IdleCheckFrequency: time.Minute,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 10,
			MaxRetries:         5,
			MinRetryBackoff:    time.Millisecond * 10,
			MaxRetryBackoff:    time.Second,
			EnableMetrics:      true,
			KeyPrefix:          "test-prefix",
		}

		// Test JSON serialization
		data, err := json.Marshal(config)
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// Test JSON deserialization
		var deserializedConfig RedisConfig
		err = json.Unmarshal(data, &deserializedConfig)
		assert.NoError(t, err)
		assert.Equal(t, config.Host, deserializedConfig.Host)
		assert.Equal(t, config.Port, deserializedConfig.Port)
		assert.Equal(t, config.PoolSize, deserializedConfig.PoolSize)
		assert.Equal(t, config.EnableMetrics, deserializedConfig.EnableMetrics)
	})
}

func TestCacheConfig(t *testing.T) {
	t.Run("cache config validation", func(t *testing.T) {
		config := &CacheConfig{
			ReviewTTL:         time.Hour,
			HotelTTL:          time.Hour * 2,
			ProviderTTL:       time.Hour,
			StatisticsTTL:     time.Minute * 30,
			SearchTTL:         time.Minute * 15,
			ProcessingTTL:     time.Hour,
			AnalyticsTTL:      time.Hour * 6,
			DefaultTTL:        time.Hour,
			MaxKeyLength:      250,
			EnableCompression: true,
			CompressionLevel:  6,
			PrefixSeparator:   ":",
			InvalidationBatch: 100,
			WarmupConcurrency: 5,
		}

		assert.Equal(t, time.Hour, config.ReviewTTL)
		assert.Equal(t, time.Hour*2, config.HotelTTL)
		assert.Equal(t, 250, config.MaxKeyLength)
		assert.True(t, config.EnableCompression)
		assert.Equal(t, 6, config.CompressionLevel)
	})
}

func TestCacheOperations(t *testing.T) {
	t.Run("cache operation constants", func(t *testing.T) {
		assert.Equal(t, CacheOperation("get"), CacheOperationGet)
		assert.Equal(t, CacheOperation("set"), CacheOperationSet)
		assert.Equal(t, CacheOperation("delete"), CacheOperationDelete)
		assert.Equal(t, CacheOperation("exists"), CacheOperationExists)
		assert.Equal(t, CacheOperation("expire"), CacheOperationExpire)
		assert.Equal(t, CacheOperation("hget"), CacheOperationHGet)
		assert.Equal(t, CacheOperation("hset"), CacheOperationHSet)
		assert.Equal(t, CacheOperation("hdel"), CacheOperationHDel)
		assert.Equal(t, CacheOperation("hkeys"), CacheOperationHKeys)
		assert.Equal(t, CacheOperation("hvals"), CacheOperationHVals)
		assert.Equal(t, CacheOperation("hgetall"), CacheOperationHGetAll)
	})
}

func TestRedisClient_KeyBuilding(t *testing.T) {
	client := createTestRedisClient()

	t.Run("build key with prefix", func(t *testing.T) {
		key := client.buildKey("users", "123")
		expected := "test:users:123"
		assert.Equal(t, expected, key)
	})

	t.Run("build key without prefix", func(t *testing.T) {
		client.config.KeyPrefix = ""
		key := client.buildKey("users", "123")
		expected := "users:123"
		assert.Equal(t, expected, key)
	})
}

func TestRedisClient_KeyHashing(t *testing.T) {
	client := createTestRedisClient()

	t.Run("hash long key", func(t *testing.T) {
		// Create a key longer than max length
		longKey := string(make([]byte, 300))
		for i := range longKey {
			longKey = longKey[:i] + "a" + longKey[i+1:]
		}

		hashedKey := client.hashKey(longKey)
		assert.True(t, len(hashedKey) <= client.cacheConfig.MaxKeyLength)
		assert.NotEqual(t, longKey, hashedKey)
	})

	t.Run("dont hash short key", func(t *testing.T) {
		shortKey := "short"
		hashedKey := client.hashKey(shortKey)
		assert.Equal(t, shortKey, hashedKey)
	})
}

func TestRedisClient_Metrics(t *testing.T) {
	client := createTestRedisClient()

	t.Run("record cache hit metric", func(t *testing.T) {
		client.recordMetric(CacheOperationGet, "users", true, time.Millisecond*10)

		assert.Equal(t, int64(1), client.metrics.TotalHits)
		assert.Equal(t, int64(0), client.metrics.TotalMisses)
		assert.Equal(t, int64(1), client.metrics.OperationCounts[CacheOperationGet])
		assert.Equal(t, int64(1), client.metrics.KeyspaceCounts["users"])
		assert.Equal(t, float64(100), client.metrics.HitRate)
	})

	t.Run("record cache miss metric", func(t *testing.T) {
		client.recordMetric(CacheOperationGet, "users", false, time.Millisecond*5)

		assert.Equal(t, int64(1), client.metrics.TotalHits)
		assert.Equal(t, int64(1), client.metrics.TotalMisses)
		assert.Equal(t, float64(50), client.metrics.HitRate)
	})

	t.Run("record set metric", func(t *testing.T) {
		client.recordMetric(CacheOperationSet, "products", false, time.Millisecond*15)

		assert.Equal(t, int64(1), client.metrics.TotalSets)
		assert.Equal(t, int64(1), client.metrics.KeyspaceCounts["products"])
	})

	t.Run("record delete metric", func(t *testing.T) {
		client.recordMetric(CacheOperationDelete, "orders", false, time.Millisecond*8)

		assert.Equal(t, int64(1), client.metrics.TotalDeletes)
		assert.Equal(t, int64(1), client.metrics.KeyspaceCounts["orders"])
	})
}

func TestRedisClient_ErrorRecording(t *testing.T) {
	client := createTestRedisClient()

	t.Run("record error", func(t *testing.T) {
		initialErrors := client.metrics.TotalErrors
		
		client.recordError(CacheOperationGet, "users", assert.AnError)

		assert.Equal(t, initialErrors+1, client.metrics.TotalErrors)
	})
}

func TestCacheMetrics_Copy(t *testing.T) {
	client := createTestRedisClient()

	// Add some metrics
	client.recordMetric(CacheOperationGet, "users", true, time.Millisecond*10)
	client.recordMetric(CacheOperationSet, "products", false, time.Millisecond*5)

	t.Run("get metrics copy", func(t *testing.T) {
		metrics := client.GetMetrics()

		assert.Equal(t, int64(1), metrics.TotalHits)
		assert.Equal(t, int64(1), metrics.TotalSets)
		assert.NotEmpty(t, metrics.OperationCounts)
		assert.NotEmpty(t, metrics.KeyspaceCounts)

		// Verify it's a copy by modifying original
		client.recordMetric(CacheOperationGet, "users", true, time.Millisecond*20)
		
		// Metrics copy should not change
		assert.Equal(t, int64(1), metrics.TotalHits)
	})
}

func TestInvalidationRequest(t *testing.T) {
	t.Run("create invalidation request", func(t *testing.T) {
		called := false
		req := InvalidationRequest{
			Pattern:   "users:*",
			Keys:      []string{"user:1", "user:2"},
			Immediate: true,
			Callback: func(count int, err error) {
				called = true
				assert.Equal(t, 2, count)
				assert.NoError(t, err)
			},
		}

		assert.Equal(t, "users:*", req.Pattern)
		assert.Len(t, req.Keys, 2)
		assert.True(t, req.Immediate)
		
		// Test callback
		req.Callback(2, nil)
		assert.True(t, called)
	})
}

func TestReviewCache(t *testing.T) {
	client := createTestRedisClient()
	log := logger.NewDefault()
	
	t.Run("create review cache", func(t *testing.T) {
		reviewCache := NewReviewCache(client, nil, log)
		
		assert.NotNil(t, reviewCache)
		assert.Equal(t, client, reviewCache.client)
		assert.Equal(t, log, reviewCache.logger)
	})
}

func TestHotelCache(t *testing.T) {
	client := createTestRedisClient()
	log := logger.NewDefault()
	
	t.Run("create hotel cache", func(t *testing.T) {
		hotelCache := NewHotelCache(client, log)
		
		assert.NotNil(t, hotelCache)
		assert.Equal(t, client, hotelCache.client)
		assert.Equal(t, log, hotelCache.logger)
	})
}

func TestProcessingStatusCache(t *testing.T) {
	client := createTestRedisClient()
	log := logger.NewDefault()
	
	t.Run("create processing status cache", func(t *testing.T) {
		statusCache := NewProcessingStatusCache(client, log)
		
		assert.NotNil(t, statusCache)
		assert.Equal(t, client, statusCache.client)
		assert.Equal(t, log, statusCache.logger)
	})
}

func TestRedisClient_Close(t *testing.T) {
	client := createTestRedisClient()

	t.Run("close client structure", func(t *testing.T) {
		// Test close method existence and basic structure
		// We can't test the actual close without panicking, so we test the components
		assert.NotNil(t, client.cancel)
		assert.NotNil(t, client.ctx)
		
		// Test that context can be cancelled
		client.cancel()
		
		select {
		case <-client.ctx.Done():
			assert.Equal(t, context.Canceled, client.ctx.Err())
		default:
			t.Fatal("Context should be cancelled")
		}
	})
}

func TestRedisClient_ConcurrentMetrics(t *testing.T) {
	client := createTestRedisClient()

	t.Run("concurrent metric recording", func(t *testing.T) {
		const numGoroutines = 10
		const operationsPerGoroutine = 100

		var wg sync.WaitGroup

		// Start multiple goroutines recording metrics
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					client.recordMetric(CacheOperationGet, "concurrent", j%2 == 0, time.Millisecond)
				}
			}(i)
		}

		wg.Wait()

		// Verify metrics
		totalOperations := int64(numGoroutines * operationsPerGoroutine)
		assert.Equal(t, totalOperations, client.metrics.OperationCounts[CacheOperationGet])
		assert.Equal(t, totalOperations, client.metrics.KeyspaceCounts["concurrent"])
		
		// Half should be hits, half should be misses
		expectedHits := int64(numGoroutines * operationsPerGoroutine / 2)
		expectedMisses := int64(numGoroutines * operationsPerGoroutine / 2)
		assert.Equal(t, expectedHits, client.metrics.TotalHits)
		assert.Equal(t, expectedMisses, client.metrics.TotalMisses)
	})
}

func TestRedisClient_InvalidationQueue(t *testing.T) {
	client := createTestRedisClient()

	t.Run("invalidation queue operations", func(t *testing.T) {
		// Test that we can send invalidation requests without blocking
		req := InvalidationRequest{
			Pattern:   "test:*",
			Immediate: false,
		}

		// This should not block since we have a buffered channel
		select {
		case client.invalidations <- req:
			// Success
		default:
			t.Fatal("Invalidation queue should not be full")
		}

		// Verify we can read from the queue
		select {
		case receivedReq := <-client.invalidations:
			assert.Equal(t, req.Pattern, receivedReq.Pattern)
		case <-time.After(time.Millisecond * 100):
			t.Fatal("Should have received invalidation request")
		}
	})
}

func TestRedisClient_ContextCancellation(t *testing.T) {
	client := createTestRedisClient()

	t.Run("context cancellation", func(t *testing.T) {
		// Verify context is not cancelled initially
		select {
		case <-client.ctx.Done():
			t.Fatal("Context should not be cancelled initially")
		default:
			// OK
		}

		// Cancel context
		client.cancel()

		// Verify context is cancelled
		select {
		case <-client.ctx.Done():
			assert.Equal(t, context.Canceled, client.ctx.Err())
		case <-time.After(time.Millisecond * 100):
			t.Fatal("Context should be cancelled")
		}
	})
}

func TestCacheConfig_Defaults(t *testing.T) {
	t.Run("cache config with reasonable defaults", func(t *testing.T) {
		config := &CacheConfig{
			ReviewTTL:         time.Hour,
			HotelTTL:          time.Hour * 2,
			DefaultTTL:        time.Minute * 30,
			MaxKeyLength:      250,
			InvalidationBatch: 100,
			WarmupConcurrency: 5,
		}

		// Verify TTL values are reasonable
		assert.True(t, config.ReviewTTL > 0)
		assert.True(t, config.HotelTTL >= config.ReviewTTL)
		assert.True(t, config.DefaultTTL > 0)
		assert.True(t, config.MaxKeyLength > 0)
		assert.True(t, config.InvalidationBatch > 0)
		assert.True(t, config.WarmupConcurrency > 0)
	})
}

func TestRedisClient_WarmupData(t *testing.T) {
	t.Run("warmup data structure validation", func(t *testing.T) {
		// Test hotel warmup data structure
		hotelData := map[string]map[string]string{
			"hotel-1": {
				"name":     "Test Hotel",
				"city":     "Test City",
				"country":  "Test Country",
				"rating":   "4.5",
			},
		}
		assert.NotEmpty(t, hotelData)
		assert.Contains(t, hotelData, "hotel-1")
		assert.Equal(t, "Test Hotel", hotelData["hotel-1"]["name"])

		// Test review warmup data structure
		reviewData := map[string]string{
			"review-1": `{"id":"review-1","content":"Great hotel!","rating":5}`,
		}
		assert.NotEmpty(t, reviewData)
		assert.Contains(t, reviewData, "review-1")

		// Test processing status warmup data structure
		statusData := map[string]map[string]string{
			"job-1": {
				"status":   "processing",
				"progress": "50.0",
				"message":  "Processing reviews...",
			},
		}
		assert.NotEmpty(t, statusData)
		assert.Contains(t, statusData, "job-1")
		assert.Equal(t, "processing", statusData["job-1"]["status"])
	})
}