# Advanced Retry Mechanism

## Overview

The advanced retry mechanism provides comprehensive retry functionality with exponential backoff, jitter, circuit breaker integration, and configurable maximum attempts. It supports different retry strategies for different error types and proper context cancellation.

## Features

### 1. Exponential Backoff with Configurable Parameters
- **Base Delay**: Starting delay for the first retry
- **Multiplier**: Factor by which delay increases for each retry
- **Max Delay**: Maximum delay cap to prevent extremely long waits
- **Multiple Strategies**: Fixed, exponential, linear, Fibonacci, and custom backoff

### 2. Jitter Implementation
- **Full Jitter**: Random delay between 0 and calculated delay
- **Equal Jitter**: Half of calculated delay + random half
- **Decorrelated Jitter**: Prevents thundering herd effect
- **Configurable Deviation**: Control the amount of randomness

### 3. Circuit Breaker Integration
- **Automatic Circuit Breaking**: Opens circuit on consecutive failures
- **State Management**: Closed, Open, Half-Open states
- **Health Checks**: Periodic health checks to close circuit
- **Metrics Integration**: Failure counting and success rate tracking

### 4. Error Type Classification
- **Transient Errors**: Network timeouts, temporary server errors
- **Permanent Errors**: Client errors, authentication failures
- **Rate Limit Errors**: Too many requests, quota exceeded
- **Network Errors**: Connection refused, DNS failures
- **Server Errors**: 5xx HTTP status codes
- **Client Errors**: 4xx HTTP status codes

### 5. Retry Strategies

#### Fixed Delay
```go
config.Strategy = StrategyFixedDelay
config.BaseDelay = 1 * time.Second
```

#### Exponential Backoff
```go
config.Strategy = StrategyExponentialBackoff
config.BaseDelay = 100 * time.Millisecond
config.Multiplier = 2.0
config.MaxDelay = 30 * time.Second
```

#### Linear Backoff
```go
config.Strategy = StrategyLinearBackoff
config.BaseDelay = 100 * time.Millisecond
config.Multiplier = 1.0
```

#### Fibonacci Backoff
```go
config.Strategy = StrategyFibonacciBackoff
config.BaseDelay = 100 * time.Millisecond
```

#### Custom Backoff
```go
config.Strategy = StrategyCustom
config.CustomBackoff = func(attempt int, baseDelay time.Duration) time.Duration {
    return time.Duration(attempt * attempt) * baseDelay
}
```

### 6. Comprehensive Metrics and Logging

#### Global Metrics
- Total operations and attempts
- Success and failure rates
- Average duration and backoff times
- Retry statistics by error type and attempt
- Circuit breaker rejection counts
- Dead letter queue counts

#### Operation-Specific Metrics
- Per-operation success rates
- Average response times
- Retry patterns by operation type
- Tagged metrics for filtering

#### Detailed Logging
- Retry attempts with context
- Error classification and handling
- Backoff calculations
- Circuit breaker state changes
- Dead letter queue operations

### 7. Custom Retry Conditions and Predicates

#### Error-Based Conditions
```go
config.CustomConditions = []RetryCondition{
    func(err error, attempt int) bool {
        // Only retry network errors
        return isNetworkError(err)
    },
}
```

#### Attempt-Based Conditions
```go
config.CustomConditions = []RetryCondition{
    func(err error, attempt int) bool {
        // Don't retry after 3 attempts for rate limits
        if isRateLimitError(err) {
            return attempt <= 3
        }
        return attempt <= config.MaxAttempts
    },
}
```

### 8. Retry Policies for Different Operation Types

#### Database Operations
```go
config := DatabaseRetryConfig()
// - 3 max attempts
// - 100ms base delay
// - Exponential backoff
// - Retries transient, timeout, network errors
// - Circuit breaker enabled
```

#### HTTP Requests
```go
config := HTTPRetryConfig()
// - 5 max attempts
// - 200ms base delay
// - Exponential backoff with jitter
// - Retries server errors, rate limits
// - Circuit breaker enabled
```

#### S3 Operations
```go
config := S3RetryConfig()
// - 4 max attempts
// - 500ms base delay
// - Exponential backoff
// - Longer timeouts for large files
// - Dead letter queue enabled
```

#### Cache Operations
```go
config := CacheRetryConfig()
// - 2 max attempts
// - 50ms base delay
// - Fixed delay (cache failures are less critical)
// - No circuit breaker
```

### 9. Dead Letter Queue Integration

#### Configuration
```go
config.EnableDeadLetter = true
config.DeadLetterHandler = func(ctx context.Context, operation string, err error, attempts int, metadata map[string]interface{}) {
    // Send to dead letter queue
    // Log for manual review
    // Send alerts
    // Store in database
}
```

#### Use Cases
- Failed operations requiring manual intervention
- Data processing failures
- Critical operations that must not be lost
- Audit trail for failed operations

### 10. Context Cancellation Support

#### Timeout Handling
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := retryManager.Execute(ctx, operation)
```

#### Cancellation Propagation
```go
ctx, cancel := context.WithCancel(context.Background())
// Cancel from another goroutine
go func() {
    time.Sleep(5 * time.Second)
    cancel()
}()

result, err := retryManager.Execute(ctx, operation)
```

## Usage Examples

### Basic Usage

```go
// Create retry manager with default config
config := infrastructure.DefaultRetryConfig()
config.MaxAttempts = 3
config.BaseDelay = 100 * time.Millisecond

rm := infrastructure.NewRetryManager(config, nil, logger)
defer rm.Close()

// Execute operation with retry
operation := func(ctx context.Context, attempt int) (interface{}, error) {
    // Your operation logic here
    return performOperation()
}

result, err := rm.Execute(context.Background(), operation)
```

### With Circuit Breaker

```go
// Create circuit breaker
cbConfig := infrastructure.DefaultCircuitBreakerConfig()
cbConfig.Name = "api_client"
cbConfig.FailureThreshold = 5
circuitBreaker := infrastructure.NewCircuitBreaker(cbConfig, logger)

// Create retry manager with circuit breaker
config := infrastructure.HTTPRetryConfig()
config.EnableCircuitBreaker = true

rm := infrastructure.NewRetryManager(config, circuitBreaker, logger)
defer rm.Close()

result, err := rm.Execute(context.Background(), operation)
```

### Using Retry Manager Pool

```go
// Create pool for managing multiple retry managers
pool := infrastructure.NewRetryManagerPool(logger)
defer pool.Close()

// Get managers for different operation types
dbManager := pool.GetManager("database")
httpManager := pool.GetManager("http")
s3Manager := pool.GetManager("s3")

// Use appropriate manager for each operation
dbResult, err := dbManager.Execute(ctx, databaseOperation)
httpResult, err := httpManager.Execute(ctx, httpOperation)
s3Result, err := s3Manager.Execute(ctx, s3Operation)
```

### Custom Configuration

```go
config := &infrastructure.RetryConfig{
    MaxAttempts:         5,
    BaseDelay:           200 * time.Millisecond,
    MaxDelay:            10 * time.Second,
    Timeout:             2 * time.Minute,
    Strategy:            infrastructure.StrategyExponentialBackoff,
    Multiplier:          1.5,
    JitterType:          infrastructure.JitterTypeEqual,
    JitterMaxDeviation:  0.1,
    EnableCircuitBreaker: true,
    RetryableErrors:     []infrastructure.ErrorType{
        infrastructure.ErrorTypeTransient,
        infrastructure.ErrorTypeTimeout,
        infrastructure.ErrorTypeNetwork,
        infrastructure.ErrorTypeRateLimit,
    },
    PermanentErrors:     []infrastructure.ErrorType{
        infrastructure.ErrorTypePermanent,
        infrastructure.ErrorTypeClient,
    },
    CustomConditions: []infrastructure.RetryCondition{
        func(err error, attempt int) bool {
            // Custom retry logic
            return shouldRetry(err, attempt)
        },
    },
    EnableDeadLetter:    true,
    DeadLetterHandler:   deadLetterHandler,
    EnableMetrics:       true,
    EnableLogging:       true,
    LogLevel:            "info",
    OperationName:       "custom_operation",
    OperationType:       "api_call",
    Tags: map[string]string{
        "service":  "user_service",
        "endpoint": "/api/users",
    },
}
```

## Best Practices

### 1. Choose Appropriate Retry Strategy
- **Fixed Delay**: Simple operations with predictable failures
- **Exponential Backoff**: Most common, good for avoiding thundering herd
- **Linear Backoff**: When you want gradual increase without exponential growth
- **Fibonacci Backoff**: Alternative to exponential with different growth pattern

### 2. Configure Jitter
- Always use jitter in distributed systems
- **Full Jitter**: Most effective for avoiding thundering herd
- **Equal Jitter**: Balance between predictability and randomness
- **Decorrelated Jitter**: Best for highly concurrent systems

### 3. Set Appropriate Timeouts
- **Operation Timeout**: Total time for all retry attempts
- **Request Timeout**: Individual request timeout
- **Circuit Breaker Timeout**: How long to wait before half-open

### 4. Error Classification
- Be specific about which errors to retry
- Don't retry client errors (4xx)
- Retry transient errors (timeouts, temporary failures)
- Consider rate limit errors separately

### 5. Monitor and Alert
- Track retry rates and failure patterns
- Set up alerts for high failure rates
- Monitor circuit breaker state changes
- Review dead letter queue contents

### 6. Use Circuit Breakers
- Enable circuit breakers for external dependencies
- Configure appropriate failure thresholds
- Monitor circuit breaker state
- Implement fallback mechanisms

### 7. Dead Letter Queue
- Use for critical operations that must not be lost
- Process dead letter queue items manually or with separate service
- Set up alerting for dead letter queue growth
- Include sufficient metadata for debugging

## Configuration Reference

### RetryConfig

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `MaxAttempts` | `int` | Maximum number of retry attempts | 3 |
| `BaseDelay` | `time.Duration` | Base delay for first retry | 100ms |
| `MaxDelay` | `time.Duration` | Maximum delay cap | 30s |
| `Timeout` | `time.Duration` | Total operation timeout | 5m |
| `Strategy` | `RetryStrategy` | Retry strategy type | Exponential |
| `Multiplier` | `float64` | Backoff multiplier | 2.0 |
| `JitterType` | `JitterType` | Jitter algorithm | Full |
| `JitterMaxDeviation` | `float64` | Maximum jitter deviation | 0.1 |
| `EnableCircuitBreaker` | `bool` | Enable circuit breaker | true |
| `RetryableErrors` | `[]ErrorType` | Errors to retry | Transient, Timeout, Network |
| `PermanentErrors` | `[]ErrorType` | Errors not to retry | Permanent, Client |
| `CustomConditions` | `[]RetryCondition` | Custom retry conditions | nil |
| `EnableDeadLetter` | `bool` | Enable dead letter queue | true |
| `DeadLetterHandler` | `DeadLetterHandler` | Dead letter handler function | nil |
| `CustomBackoff` | `BackoffFunc` | Custom backoff function | nil |
| `EnableMetrics` | `bool` | Enable metrics collection | true |
| `EnableLogging` | `bool` | Enable detailed logging | true |
| `LogLevel` | `string` | Log level for retry operations | "info" |
| `OperationName` | `string` | Name for the operation | "default" |
| `OperationType` | `string` | Type of operation | "unknown" |
| `Tags` | `map[string]string` | Tags for metrics and logging | {} |

### Error Types

| Type | Description | Retry Recommendation |
|------|-------------|---------------------|
| `ErrorTypeTransient` | Temporary failures | Retry |
| `ErrorTypePermanent` | Permanent failures | Don't retry |
| `ErrorTypeTimeout` | Operation timeouts | Retry |
| `ErrorTypeRateLimit` | Rate limit exceeded | Retry with longer delays |
| `ErrorTypeCircuitBreaker` | Circuit breaker open | Don't retry |
| `ErrorTypeNetwork` | Network failures | Retry |
| `ErrorTypeServer` | Server errors (5xx) | Retry |
| `ErrorTypeClient` | Client errors (4xx) | Don't retry |
| `ErrorTypeUnknown` | Unknown errors | Retry (with caution) |

### Retry Strategies

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `StrategyFixedDelay` | Fixed delay between retries | Simple operations |
| `StrategyExponentialBackoff` | Exponential increase in delay | Most common case |
| `StrategyLinearBackoff` | Linear increase in delay | Gradual backing off |
| `StrategyFibonacciBackoff` | Fibonacci sequence delays | Alternative to exponential |
| `StrategyCustom` | Custom backoff function | Special requirements |

### Jitter Types

| Type | Description | Formula |
|------|-------------|---------|
| `JitterTypeNone` | No jitter | delay |
| `JitterTypeFull` | Full randomization | random(0, delay) |
| `JitterTypeEqual` | Equal jitter | delay/2 + random(0, delay/2) |
| `JitterTypeDecorrelated` | Decorrelated jitter | random(baseDelay, prevDelay * 3) |

## Integration Examples

### With Database Operations

```go
func (r *Repository) GetHotelReviews(ctx context.Context, hotelID string) ([]Review, error) {
    manager := r.retryPool.GetManager("database")
    
    operation := func(ctx context.Context, attempt int) (interface{}, error) {
        return r.db.GetReviewsByHotelID(ctx, hotelID)
    }
    
    result, err := manager.Execute(ctx, operation)
    if err != nil {
        return nil, err
    }
    
    return result.([]Review), nil
}
```

### With HTTP Client

```go
func (c *APIClient) FetchReviews(ctx context.Context, url string) (*http.Response, error) {
    manager := c.retryPool.GetManager("http")
    
    operation := func(ctx context.Context, attempt int) (interface{}, error) {
        req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
        if err != nil {
            return nil, err
        }
        
        return c.httpClient.Do(req)
    }
    
    result, err := manager.Execute(ctx, operation)
    if err != nil {
        return nil, err
    }
    
    return result.(*http.Response), nil
}
```

### With S3 Operations

```go
func (s *S3Service) UploadFile(ctx context.Context, key string, data []byte) error {
    manager := s.retryPool.GetManager("s3")
    
    operation := func(ctx context.Context, attempt int) (interface{}, error) {
        _, err := s.s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
            Bucket: aws.String(s.bucket),
            Key:    aws.String(key),
            Body:   bytes.NewReader(data),
        })
        return nil, err
    }
    
    _, err := manager.Execute(ctx, operation)
    return err
}
```

## Testing

### Unit Tests

```go
func TestRetryManager_ExponentialBackoff(t *testing.T) {
    config := infrastructure.DefaultRetryConfig()
    config.MaxAttempts = 3
    config.BaseDelay = 100 * time.Millisecond
    config.Strategy = infrastructure.StrategyExponentialBackoff
    config.JitterType = infrastructure.JitterTypeNone
    
    rm := infrastructure.NewRetryManager(config, nil, logger)
    defer rm.Close()
    
    start := time.Now()
    fn := func(ctx context.Context, attempt int) (interface{}, error) {
        if attempt < 3 {
            return nil, errors.New("temporary failure")
        }
        return "success", nil
    }
    
    result, err := rm.Execute(context.Background(), fn)
    duration := time.Since(start)
    
    assert.NoError(t, err)
    assert.Equal(t, "success", result)
    assert.True(t, duration >= 300*time.Millisecond) // 100ms + 200ms
}
```

### Integration Tests

```go
func TestRetryManager_WithCircuitBreaker(t *testing.T) {
    cbConfig := infrastructure.DefaultCircuitBreakerConfig()
    cbConfig.FailureThreshold = 2
    cbConfig.ConsecutiveFailures = 2
    circuitBreaker := infrastructure.NewCircuitBreaker(cbConfig, logger)
    
    config := infrastructure.DefaultRetryConfig()
    config.EnableCircuitBreaker = true
    
    rm := infrastructure.NewRetryManager(config, circuitBreaker, logger)
    defer rm.Close()
    
    // Test circuit breaker integration
    failingOperation := func(ctx context.Context, attempt int) (interface{}, error) {
        return nil, errors.New("always fails")
    }
    
    // Execute failing operations to open circuit
    for i := 0; i < 3; i++ {
        _, err := rm.Execute(context.Background(), failingOperation)
        assert.Error(t, err)
    }
    
    // Circuit should be open
    assert.True(t, circuitBreaker.IsOpen())
    
    // Next operation should be rejected
    _, err := rm.Execute(context.Background(), failingOperation)
    assert.Error(t, err)
    assert.True(t, infrastructure.IsCircuitBreakerError(err))
}
```

## Performance Considerations

### Memory Usage
- Retry manager uses minimal memory
- Metrics are stored in memory but bounded
- Circuit breaker uses sliding window with fixed size

### CPU Usage
- Backoff calculations are lightweight
- Jitter uses crypto/rand for true randomness
- Metrics updates use atomic operations

### Concurrency
- All operations are thread-safe
- Uses atomic operations for counters
- Read-write mutexes for configuration access

### Scalability
- Retry manager pool reduces resource usage
- Metrics aggregation across multiple managers
- Efficient error classification with string matching

## Monitoring and Alerting

### Key Metrics to Monitor
- Retry rate by operation type
- Circuit breaker state changes
- Dead letter queue growth
- Average retry duration
- Error type distribution

### Recommended Alerts
- High retry rate (>10% of operations)
- Circuit breaker open for extended period
- Dead letter queue growth
- High failure rate after retries
- Unusual error type patterns

### Dashboards
- Retry rate trends over time
- Success rate by operation type
- Average retry duration
- Circuit breaker state history
- Error type distribution

## Troubleshooting

### Common Issues

1. **Too Many Retries**
   - Check error classification
   - Verify retry conditions
   - Review max attempts configuration

2. **Circuit Breaker Always Open**
   - Check failure thresholds
   - Verify success criteria
   - Review health check implementation

3. **High Retry Latency**
   - Check backoff strategy
   - Verify jitter configuration
   - Review max delay settings

4. **Dead Letter Queue Growth**
   - Review permanent error classification
   - Check dead letter handler
   - Verify retry conditions

### Debug Logging

Enable debug logging to see detailed retry information:

```go
config.EnableLogging = true
config.LogLevel = "debug"
```

This will log:
- Each retry attempt
- Error classification
- Backoff calculations
- Circuit breaker state changes
- Dead letter queue operations

## Migration Guide

### From Simple Retry to Advanced Retry

1. **Replace Simple Retry Logic**
   ```go
   // Old
   for i := 0; i < maxRetries; i++ {
       if err := operation(); err != nil {
           time.Sleep(delay)
           continue
       }
       break
   }
   
   // New
   rm := infrastructure.NewRetryManager(config, nil, logger)
   result, err := rm.Execute(ctx, operation)
   ```

2. **Add Circuit Breaker**
   ```go
   circuitBreaker := infrastructure.NewCircuitBreaker(cbConfig, logger)
   rm := infrastructure.NewRetryManager(config, circuitBreaker, logger)
   ```

3. **Configure Error Handling**
   ```go
   config.RetryableErrors = []infrastructure.ErrorType{
       infrastructure.ErrorTypeTransient,
       infrastructure.ErrorTypeTimeout,
   }
   ```

4. **Add Metrics and Monitoring**
   ```go
   config.EnableMetrics = true
   config.EnableLogging = true
   ```

### Best Practices for Migration

1. Start with default configuration
2. Gradually customize based on specific needs
3. Monitor retry patterns and adjust
4. Add circuit breakers for external dependencies
5. Implement dead letter queue for critical operations
6. Set up proper alerting and monitoring

This comprehensive retry mechanism provides robust error handling, prevents cascade failures, and offers detailed observability for production systems.