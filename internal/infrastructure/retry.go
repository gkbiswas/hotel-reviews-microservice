package infrastructure

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// ErrorType represents the type of error for retry classification
// Note: Using ErrorType from error_handler.go for consistency

// Retry error mappings to ErrorType constants from error_handler.go
const (
	// Common retryable error types
	RetryableNetworkError = ErrorTypeNetwork
	RetryableTimeoutError = ErrorTypeTimeout
	RetryableSystemError = ErrorTypeSystem
	RetryableExternalError = ErrorTypeExternal
	
	// Common permanent error types
	PermanentValidationError = ErrorTypeValidation
	PermanentClientError = ErrorTypeClient
	PermanentAuthError = ErrorTypeAuthentication
)

// Note: Using ErrorType.String() method from error_handler.go

// RetryStrategy represents different retry strategies
type RetryStrategy int

const (
	// StrategyFixedDelay uses fixed delay between retries
	StrategyFixedDelay RetryStrategy = iota
	// StrategyExponentialBackoff uses exponential backoff
	StrategyExponentialBackoff
	// StrategyLinearBackoff uses linear backoff
	StrategyLinearBackoff
	// StrategyFibonacciBackoff uses Fibonacci sequence for backoff
	StrategyFibonacciBackoff
	// StrategyCustom allows custom backoff calculation
	StrategyCustom
)

// String returns the string representation of the retry strategy
func (rs RetryStrategy) String() string {
	switch rs {
	case StrategyFixedDelay:
		return "FIXED_DELAY"
	case StrategyExponentialBackoff:
		return "EXPONENTIAL_BACKOFF"
	case StrategyLinearBackoff:
		return "LINEAR_BACKOFF"
	case StrategyFibonacciBackoff:
		return "FIBONACCI_BACKOFF"
	case StrategyCustom:
		return "CUSTOM"
	default:
		return "UNKNOWN"
	}
}

// JitterType represents different jitter types
type JitterType int

const (
	// JitterTypeNone uses no jitter
	JitterTypeNone JitterType = iota
	// JitterTypeFull uses full jitter
	JitterTypeFull
	// JitterTypeEqual uses equal jitter
	JitterTypeEqual
	// JitterTypeDecorrelated uses decorrelated jitter
	JitterTypeDecorrelated
)

// String returns the string representation of the jitter type
func (jt JitterType) String() string {
	switch jt {
	case JitterTypeNone:
		return "NONE"
	case JitterTypeFull:
		return "FULL"
	case JitterTypeEqual:
		return "EQUAL"
	case JitterTypeDecorrelated:
		return "DECORRELATED"
	default:
		return "UNKNOWN"
	}
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func(ctx context.Context, attempt int) (interface{}, error)

// RetryCondition represents a condition for retry
type RetryCondition func(err error, attempt int) bool

// BackoffFunc represents a custom backoff function
type BackoffFunc func(attempt int, baseDelay time.Duration) time.Duration

// DeadLetterHandler handles failed operations
type DeadLetterHandler func(ctx context.Context, operation string, err error, attempts int, metadata map[string]interface{})

// RetryConfig represents retry configuration
type RetryConfig struct {
	// Basic retry configuration
	MaxAttempts         int           `json:"max_attempts" validate:"min=1"`
	BaseDelay           time.Duration `json:"base_delay" validate:"required"`
	MaxDelay            time.Duration `json:"max_delay"`
	Timeout             time.Duration `json:"timeout"`
	
	// Strategy configuration
	Strategy            RetryStrategy `json:"strategy"`
	Multiplier          float64       `json:"multiplier" validate:"min=1"`
	
	// Jitter configuration
	JitterType          JitterType    `json:"jitter_type"`
	JitterMaxDeviation  float64       `json:"jitter_max_deviation" validate:"min=0,max=1"`
	
	// Circuit breaker integration
	EnableCircuitBreaker bool         `json:"enable_circuit_breaker"`
	CircuitBreakerName   string       `json:"circuit_breaker_name"`
	
	// Error handling
	RetryableErrors     []ErrorType   `json:"retryable_errors"`
	PermanentErrors     []ErrorType   `json:"permanent_errors"`
	CustomConditions    []RetryCondition `json:"-"`
	
	// Dead letter queue
	EnableDeadLetter    bool          `json:"enable_dead_letter"`
	DeadLetterHandler   DeadLetterHandler `json:"-"`
	
	// Custom backoff
	CustomBackoff       BackoffFunc   `json:"-"`
	
	// Metrics and logging
	EnableMetrics       bool          `json:"enable_metrics"`
	EnableLogging       bool          `json:"enable_logging"`
	LogLevel            string        `json:"log_level"`
	
	// Operation metadata
	OperationName       string        `json:"operation_name"`
	OperationType       string        `json:"operation_type"`
	Tags                map[string]string `json:"tags"`
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:         3,
		BaseDelay:           100 * time.Millisecond,
		MaxDelay:            30 * time.Second,
		Timeout:             5 * time.Minute,
		Strategy:            StrategyExponentialBackoff,
		Multiplier:          2.0,
		JitterType:          JitterTypeFull,
		JitterMaxDeviation:  0.1,
		EnableCircuitBreaker: true,
		RetryableErrors:     []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeExternal},
		PermanentErrors:     []ErrorType{ErrorTypeValidation, ErrorTypeClient},
		EnableDeadLetter:    true,
		EnableMetrics:       true,
		EnableLogging:       true,
		LogLevel:            "info",
		OperationName:       "default",
		OperationType:       "unknown",
		Tags:                make(map[string]string),
	}
}

// RetryMetrics represents retry metrics
type RetryMetrics struct {
	// Attempt statistics
	TotalAttempts       uint64        `json:"total_attempts"`
	TotalOperations     uint64        `json:"total_operations"`
	SuccessfulOperations uint64       `json:"successful_operations"`
	FailedOperations    uint64        `json:"failed_operations"`
	
	// Retry statistics
	TotalRetries        uint64        `json:"total_retries"`
	RetriesByErrorType  map[ErrorType]uint64 `json:"retries_by_error_type"`
	RetriesByAttempt    map[int]uint64 `json:"retries_by_attempt"`
	
	// Timing statistics
	TotalDuration       time.Duration `json:"total_duration"`
	AverageDuration     time.Duration `json:"average_duration"`
	MaxDuration         time.Duration `json:"max_duration"`
	MinDuration         time.Duration `json:"min_duration"`
	
	// Backoff statistics
	TotalBackoffTime    time.Duration `json:"total_backoff_time"`
	AverageBackoffTime  time.Duration `json:"average_backoff_time"`
	MaxBackoffTime      time.Duration `json:"max_backoff_time"`
	MinBackoffTime      time.Duration `json:"min_backoff_time"`
	
	// Dead letter statistics
	DeadLetterCount     uint64        `json:"dead_letter_count"`
	
	// Circuit breaker statistics
	CircuitBreakerRejects uint64      `json:"circuit_breaker_rejects"`
	
	// Current state
	LastAttemptTime     time.Time     `json:"last_attempt_time"`
	LastSuccessTime     time.Time     `json:"last_success_time"`
	LastFailureTime     time.Time     `json:"last_failure_time"`
	
	// Rates
	SuccessRate         float64       `json:"success_rate"`
	FailureRate         float64       `json:"failure_rate"`
	AverageRetriesPerOperation float64 `json:"average_retries_per_operation"`
	
	// Per-operation metrics
	OperationMetrics    map[string]*OperationMetrics `json:"operation_metrics"`
}

// OperationMetrics represents metrics for a specific operation
type OperationMetrics struct {
	OperationName       string        `json:"operation_name"`
	OperationType       string        `json:"operation_type"`
	TotalAttempts       uint64        `json:"total_attempts"`
	TotalOperations     uint64        `json:"total_operations"`
	SuccessfulOperations uint64       `json:"successful_operations"`
	FailedOperations    uint64        `json:"failed_operations"`
	TotalRetries        uint64        `json:"total_retries"`
	AverageDuration     time.Duration `json:"average_duration"`
	SuccessRate         float64       `json:"success_rate"`
	LastAttemptTime     time.Time     `json:"last_attempt_time"`
	Tags                map[string]string `json:"tags"`
}

// RetryContext represents the context for a retry operation
type RetryContext struct {
	OperationName       string
	OperationType       string
	Attempt             int
	MaxAttempts         int
	StartTime           time.Time
	LastAttemptTime     time.Time
	LastError           error
	LastErrorType       ErrorType
	TotalDuration       time.Duration
	BackoffDuration     time.Duration
	Metadata            map[string]interface{}
	Tags                map[string]string
}

// RetryManager manages retry operations
type RetryManager struct {
	config              *RetryConfig
	metrics             *RetryMetrics
	circuitBreaker      *CircuitBreaker
	logger              *logger.Logger
	
	// Internal state
	mu                  sync.RWMutex
	lastBackoffTime     time.Duration
	fibonacciSequence   []int
	
	// Background processes
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config *RetryConfig, circuitBreaker *CircuitBreaker, logger *logger.Logger) *RetryManager {
	if config == nil {
		config = DefaultRetryConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	rm := &RetryManager{
		config:         config,
		metrics:        &RetryMetrics{
			RetriesByErrorType:  make(map[ErrorType]uint64),
			RetriesByAttempt:    make(map[int]uint64),
			MinDuration:         time.Duration(^uint64(0) >> 1), // Max duration
			MinBackoffTime:      time.Duration(^uint64(0) >> 1), // Max duration
			OperationMetrics:    make(map[string]*OperationMetrics),
		},
		circuitBreaker: circuitBreaker,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		fibonacciSequence: []int{1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377, 610, 987, 1597, 2584, 4181, 6765},
	}
	
	// Start background processes
	if config.EnableMetrics {
		rm.wg.Add(1)
		go rm.metricsLoop()
	}
	
	return rm
}

// Close closes the retry manager
func (rm *RetryManager) Close() {
	rm.cancel()
	rm.wg.Wait()
}

// Execute executes a function with retry logic
func (rm *RetryManager) Execute(ctx context.Context, fn RetryableFunc) (interface{}, error) {
	return rm.ExecuteWithConfig(ctx, fn, rm.config)
}

// ExecuteWithConfig executes a function with custom retry configuration
func (rm *RetryManager) ExecuteWithConfig(ctx context.Context, fn RetryableFunc, config *RetryConfig) (interface{}, error) {
	if config == nil {
		config = rm.config
	}
	
	// Create retry context
	retryCtx := &RetryContext{
		OperationName:   config.OperationName,
		OperationType:   config.OperationType,
		MaxAttempts:     config.MaxAttempts,
		StartTime:       time.Now(),
		Metadata:        make(map[string]interface{}),
		Tags:            config.Tags,
	}
	
	// Create timeout context if specified
	operationCtx := ctx
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		operationCtx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}
	
	// Execute with retry logic
	result, err := rm.executeWithRetry(operationCtx, fn, config, retryCtx)
	
	// Update metrics
	rm.updateMetrics(retryCtx, err)
	
	// Handle dead letter queue
	if err != nil && config.EnableDeadLetter && config.DeadLetterHandler != nil {
		rm.handleDeadLetter(ctx, config, retryCtx, err)
	}
	
	return result, err
}

// executeWithRetry executes a function with retry logic
func (rm *RetryManager) executeWithRetry(ctx context.Context, fn RetryableFunc, config *RetryConfig, retryCtx *RetryContext) (interface{}, error) {
	var lastErr error
	var result interface{}
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		retryCtx.Attempt = attempt
		retryCtx.LastAttemptTime = time.Now()
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// Check circuit breaker
		if config.EnableCircuitBreaker && rm.circuitBreaker != nil {
			if err := rm.checkCircuitBreaker(ctx, config); err != nil {
				rm.recordCircuitBreakerReject()
				rm.circuitBreaker.recordRejection()
				return nil, err
			}
		}
		
		// Execute the function
		executeStart := time.Now()
		result, lastErr = fn(ctx, attempt)
		executeDuration := time.Since(executeStart)
		
		// Update retry context
		retryCtx.LastError = lastErr
		retryCtx.TotalDuration += executeDuration
		
		// If successful, return immediately
		if lastErr == nil {
			// Record successful attempt metrics
			rm.recordAttempt(config, attempt, executeDuration, lastErr)
			
			// Record success with circuit breaker
			if config.EnableCircuitBreaker && rm.circuitBreaker != nil {
				rm.circuitBreaker.recordSuccess()
			}
			
			if config.EnableLogging {
				rm.logSuccess(config, retryCtx)
			}
			return result, nil
		}
		
		// Classify error
		errorType := rm.classifyError(lastErr)
		retryCtx.LastErrorType = errorType
		
		// Record attempt metrics
		rm.recordAttempt(config, attempt, executeDuration, lastErr)
		
		// Record failure with circuit breaker
		if config.EnableCircuitBreaker && rm.circuitBreaker != nil {
			rm.circuitBreaker.recordFailure()
		}
		
		// Check if error is retryable
		if !rm.isRetryable(lastErr, attempt, config) {
			if config.EnableLogging {
				rm.logNonRetryableError(config, retryCtx, lastErr)
			}
			break
		}
		
		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}
		
		// We are going to retry, so count this as a retry
		rm.recordRetry(config, attempt, lastErr)
		
		// Calculate backoff delay
		backoffDelay := rm.calculateBackoff(attempt, config)
		retryCtx.BackoffDuration = backoffDelay
		
		// Log retry attempt
		if config.EnableLogging {
			rm.logRetryAttempt(config, retryCtx, lastErr, backoffDelay)
		}
		
		// Wait before retry
		if backoffDelay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDelay):
			}
		}
	}
	
	if config.EnableLogging {
		rm.logAllAttemptsFailed(config, retryCtx, lastErr)
	}
	
	return result, lastErr
}

// checkCircuitBreaker checks if the circuit breaker allows the operation
func (rm *RetryManager) checkCircuitBreaker(ctx context.Context, config *RetryConfig) error {
	if rm.circuitBreaker == nil {
		return nil
	}
	
	// Check circuit breaker state
	if rm.circuitBreaker.IsOpen() {
		return &CircuitBreakerError{
			State:   StateOpen,
			Message: "circuit breaker is open",
		}
	}
	
	return nil
}

// isRetryable determines if an error is retryable
func (rm *RetryManager) isRetryable(err error, attempt int, config *RetryConfig) bool {
	if err == nil {
		return false
	}
	
	// Check custom conditions first
	for _, condition := range config.CustomConditions {
		if condition != nil && !condition(err, attempt) {
			return false
		}
	}
	
	// Classify error
	errorType := rm.classifyError(err)
	
	// Check if error type is in permanent errors list
	for _, permanentError := range config.PermanentErrors {
		if errorType == permanentError {
			return false
		}
	}
	
	// Check if error type is in retryable errors list
	for _, retryableError := range config.RetryableErrors {
		if errorType == retryableError {
			return true
		}
	}
	
	// Default behavior: retry transient and network errors
	return errorType == ErrorTypeNetwork || errorType == ErrorTypeTimeout
}

// classifyError classifies an error into a specific error type
func (rm *RetryManager) classifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeSystem
	}
	
	errorStr := err.Error()
	errorStrLower := strings.ToLower(errorStr)
	
	// Check for circuit breaker errors
	if IsCircuitBreakerError(err) {
		return ErrorTypeCircuitBreaker
	}
	
	// Check for context errors
	if err == context.Canceled || err == context.DeadlineExceeded {
		return ErrorTypeTimeout
	}
	
	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return ErrorTypeTimeout
		}
		return ErrorTypeNetwork
	}
	
	// Check for common error patterns
	if strings.Contains(errorStrLower, "timeout") || strings.Contains(errorStrLower, "deadline") {
		return ErrorTypeTimeout
	}
	
	if strings.Contains(errorStrLower, "rate limit") || strings.Contains(errorStrLower, "too many requests") {
		return ErrorTypeRateLimit
	}
	
	if strings.Contains(errorStrLower, "connection refused") || 
	   strings.Contains(errorStrLower, "connection reset") ||
	   strings.Contains(errorStrLower, "network") ||
	   strings.Contains(errorStrLower, "dns") {
		return ErrorTypeNetwork
	}
	
	// Check for HTTP status codes (if available in error message)
	if strings.Contains(errorStrLower, "500") || strings.Contains(errorStrLower, "502") ||
	   strings.Contains(errorStrLower, "503") || strings.Contains(errorStrLower, "504") {
		return ErrorTypeExternal
	}
	
	if strings.Contains(errorStrLower, "400") || strings.Contains(errorStrLower, "401") ||
	   strings.Contains(errorStrLower, "403") || strings.Contains(errorStrLower, "404") {
		return ErrorTypeClient
	}
	
	// Check for permanent error patterns
	if strings.Contains(errorStrLower, "invalid") || strings.Contains(errorStrLower, "bad request") ||
	   strings.Contains(errorStrLower, "unauthorized") || strings.Contains(errorStrLower, "forbidden") {
		return ErrorTypeValidation
	}
	
	// Default to system error for unknown errors
	return ErrorTypeSystem
}

// calculateBackoff calculates the backoff delay for a retry attempt
func (rm *RetryManager) calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	var delay time.Duration
	
	switch config.Strategy {
	case StrategyFixedDelay:
		delay = config.BaseDelay
	case StrategyExponentialBackoff:
		delay = rm.calculateExponentialBackoff(attempt, config)
	case StrategyLinearBackoff:
		delay = rm.calculateLinearBackoff(attempt, config)
	case StrategyFibonacciBackoff:
		delay = rm.calculateFibonacciBackoff(attempt, config)
	case StrategyCustom:
		if config.CustomBackoff != nil {
			delay = config.CustomBackoff(attempt, config.BaseDelay)
		} else {
			delay = config.BaseDelay
		}
	default:
		delay = config.BaseDelay
	}
	
	// Apply maximum delay limit
	if config.MaxDelay > 0 && delay > config.MaxDelay {
		delay = config.MaxDelay
	}
	
	// Apply jitter
	delay = rm.applyJitter(delay, config, attempt)
	
	return delay
}

// calculateExponentialBackoff calculates exponential backoff delay
func (rm *RetryManager) calculateExponentialBackoff(attempt int, config *RetryConfig) time.Duration {
	multiplier := config.Multiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}
	
	delay := float64(config.BaseDelay) * math.Pow(multiplier, float64(attempt-1))
	return time.Duration(delay)
}

// calculateLinearBackoff calculates linear backoff delay
func (rm *RetryManager) calculateLinearBackoff(attempt int, config *RetryConfig) time.Duration {
	multiplier := config.Multiplier
	if multiplier <= 0 {
		multiplier = 1.0
	}
	
	delay := float64(config.BaseDelay) * multiplier * float64(attempt)
	return time.Duration(delay)
}

// calculateFibonacciBackoff calculates Fibonacci backoff delay
func (rm *RetryManager) calculateFibonacciBackoff(attempt int, config *RetryConfig) time.Duration {
	fibIndex := attempt - 1
	if fibIndex >= len(rm.fibonacciSequence) {
		fibIndex = len(rm.fibonacciSequence) - 1
	}
	
	delay := float64(config.BaseDelay) * float64(rm.fibonacciSequence[fibIndex])
	return time.Duration(delay)
}

// applyJitter applies jitter to the delay
func (rm *RetryManager) applyJitter(delay time.Duration, config *RetryConfig, attempt int) time.Duration {
	if config.JitterType == JitterTypeNone {
		return delay
	}
	
	jitteredDelay := delay
	maxDeviation := config.JitterMaxDeviation
	if maxDeviation <= 0 {
		maxDeviation = 0.1
	}
	
	switch config.JitterType {
	case JitterTypeFull:
		// Full jitter: random delay between 0 and calculated delay
		jitteredDelay = rm.randomDuration(0, delay)
	case JitterTypeEqual:
		// Equal jitter: delay/2 + random(0, delay/2)
		halfDelay := delay / 2
		jitteredDelay = halfDelay + rm.randomDuration(0, halfDelay)
	case JitterTypeDecorrelated:
		// Decorrelated jitter: random delay between baseDelay and previous delay * 3
		rm.mu.Lock()
		minDelay := config.BaseDelay
		maxDelay := time.Duration(float64(delay) * 3)
		if rm.lastBackoffTime > 0 {
			maxDelay = time.Duration(float64(rm.lastBackoffTime) * 3)
		}
		jitteredDelay = rm.randomDuration(minDelay, maxDelay)
		rm.lastBackoffTime = jitteredDelay
		rm.mu.Unlock()
	}
	
	return jitteredDelay
}

// randomDuration generates a random duration between min and max
func (rm *RetryManager) randomDuration(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	
	diff := max - min
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(diff)))
	if err != nil {
		// Fallback to fixed duration if random generation fails
		return min
	}
	
	return min + time.Duration(nBig.Int64())
}

// recordAttempt records metrics for an attempt
func (rm *RetryManager) recordAttempt(config *RetryConfig, attempt int, duration time.Duration, err error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// Update global metrics
	atomic.AddUint64(&rm.metrics.TotalAttempts, 1)
	
	if attempt == 1 {
		atomic.AddUint64(&rm.metrics.TotalOperations, 1)
	}
	
	if err == nil {
		atomic.AddUint64(&rm.metrics.SuccessfulOperations, 1)
		rm.metrics.LastSuccessTime = time.Now()
	} else {
		// Check if this error will be retried
		if attempt == config.MaxAttempts || !rm.isRetryable(err, attempt, config) {
			atomic.AddUint64(&rm.metrics.FailedOperations, 1)
		}
		rm.metrics.LastFailureTime = time.Now()
	}
	
	// Update per-operation metrics
	rm.updateOperationMetrics(config, attempt, duration, err)
}

// recordRetry records that we're about to retry
func (rm *RetryManager) recordRetry(config *RetryConfig, attempt int, err error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	atomic.AddUint64(&rm.metrics.TotalRetries, 1)
	errorType := rm.classifyError(err)
	rm.metrics.RetriesByErrorType[errorType]++
	rm.metrics.RetriesByAttempt[attempt+1]++  // +1 because next attempt will be attempt+1
}



// updateOperationMetrics updates metrics for a specific operation
func (rm *RetryManager) updateOperationMetrics(config *RetryConfig, attempt int, duration time.Duration, err error) {
	operationKey := fmt.Sprintf("%s:%s", config.OperationName, config.OperationType)
	
	opMetrics, exists := rm.metrics.OperationMetrics[operationKey]
	if !exists {
		opMetrics = &OperationMetrics{
			OperationName: config.OperationName,
			OperationType: config.OperationType,
			Tags:          config.Tags,
		}
		rm.metrics.OperationMetrics[operationKey] = opMetrics
	}
	
	opMetrics.TotalAttempts++
	
	if attempt == 1 {
		opMetrics.TotalOperations++
	}
	
	if err == nil {
		opMetrics.SuccessfulOperations++
	} else if attempt == config.MaxAttempts {
		opMetrics.FailedOperations++
	}
	
	if attempt > 1 {
		opMetrics.TotalRetries++
	}
	
	// Update average duration
	if opMetrics.TotalOperations > 0 {
		opMetrics.AverageDuration = time.Duration(
			(int64(opMetrics.AverageDuration)*int64(opMetrics.TotalOperations-1) + int64(duration)) / int64(opMetrics.TotalOperations),
		)
	}
	
	// Update success rate
	if opMetrics.TotalOperations > 0 {
		opMetrics.SuccessRate = float64(opMetrics.SuccessfulOperations) / float64(opMetrics.TotalOperations) * 100
	}
	
	opMetrics.LastAttemptTime = time.Now()
}

// recordCircuitBreakerReject records a circuit breaker rejection
func (rm *RetryManager) recordCircuitBreakerReject() {
	atomic.AddUint64(&rm.metrics.CircuitBreakerRejects, 1)
}

// updateMetrics updates final metrics after operation completion
func (rm *RetryManager) updateMetrics(retryCtx *RetryContext, finalErr error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// Update average retries per operation
	totalOps := atomic.LoadUint64(&rm.metrics.TotalOperations)
	totalRetries := atomic.LoadUint64(&rm.metrics.TotalRetries)
	if totalOps > 0 {
		rm.metrics.AverageRetriesPerOperation = float64(totalRetries) / float64(totalOps)
	}
	
	// Update backoff statistics
	if retryCtx.BackoffDuration > 0 {
		rm.metrics.TotalBackoffTime += retryCtx.BackoffDuration
		
		if retryCtx.BackoffDuration > rm.metrics.MaxBackoffTime {
			rm.metrics.MaxBackoffTime = retryCtx.BackoffDuration
		}
		if retryCtx.BackoffDuration < rm.metrics.MinBackoffTime {
			rm.metrics.MinBackoffTime = retryCtx.BackoffDuration
		}
		
		// Update average backoff time
		if totalRetries > 0 {
			rm.metrics.AverageBackoffTime = time.Duration(int64(rm.metrics.TotalBackoffTime) / int64(totalRetries))
		}
	}
}

// handleDeadLetter handles dead letter queue operations
func (rm *RetryManager) handleDeadLetter(ctx context.Context, config *RetryConfig, retryCtx *RetryContext, err error) {
	atomic.AddUint64(&rm.metrics.DeadLetterCount, 1)
	
	if config.DeadLetterHandler != nil {
		metadata := map[string]interface{}{
			"operation_name":    retryCtx.OperationName,
			"operation_type":    retryCtx.OperationType,
			"total_attempts":    retryCtx.Attempt,
			"total_duration":    retryCtx.TotalDuration,
			"last_error_type":   string(retryCtx.LastErrorType),
			"backoff_duration":  retryCtx.BackoffDuration,
			"tags":              retryCtx.Tags,
			"start_time":        retryCtx.StartTime,
			"last_attempt_time": retryCtx.LastAttemptTime,
		}
		
		// Copy custom metadata
		for k, v := range retryCtx.Metadata {
			metadata[k] = v
		}
		
		config.DeadLetterHandler(ctx, retryCtx.OperationName, err, retryCtx.Attempt, metadata)
	}
}

// GetMetrics returns current retry metrics
func (rm *RetryManager) GetMetrics() *RetryMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := &RetryMetrics{
		TotalAttempts:         atomic.LoadUint64(&rm.metrics.TotalAttempts),
		TotalOperations:       atomic.LoadUint64(&rm.metrics.TotalOperations),
		SuccessfulOperations:  atomic.LoadUint64(&rm.metrics.SuccessfulOperations),
		FailedOperations:      atomic.LoadUint64(&rm.metrics.FailedOperations),
		TotalRetries:          atomic.LoadUint64(&rm.metrics.TotalRetries),
		RetriesByErrorType:    make(map[ErrorType]uint64),
		RetriesByAttempt:      make(map[int]uint64),
		TotalDuration:         rm.metrics.TotalDuration,
		AverageDuration:       rm.metrics.AverageDuration,
		MaxDuration:           rm.metrics.MaxDuration,
		MinDuration:           rm.metrics.MinDuration,
		TotalBackoffTime:      rm.metrics.TotalBackoffTime,
		AverageBackoffTime:    rm.metrics.AverageBackoffTime,
		MaxBackoffTime:        rm.metrics.MaxBackoffTime,
		MinBackoffTime:        rm.metrics.MinBackoffTime,
		DeadLetterCount:       atomic.LoadUint64(&rm.metrics.DeadLetterCount),
		CircuitBreakerRejects: atomic.LoadUint64(&rm.metrics.CircuitBreakerRejects),
		LastAttemptTime:       rm.metrics.LastAttemptTime,
		LastSuccessTime:       rm.metrics.LastSuccessTime,
		LastFailureTime:       rm.metrics.LastFailureTime,
		SuccessRate:           0,
		FailureRate:           0,
		AverageRetriesPerOperation: rm.metrics.AverageRetriesPerOperation,
		OperationMetrics:      make(map[string]*OperationMetrics),
	}
	
	// Copy maps
	for k, v := range rm.metrics.RetriesByErrorType {
		metrics.RetriesByErrorType[k] = v
	}
	for k, v := range rm.metrics.RetriesByAttempt {
		metrics.RetriesByAttempt[k] = v
	}
	for k, v := range rm.metrics.OperationMetrics {
		metrics.OperationMetrics[k] = &(*v) // Deep copy
	}
	
	// Calculate success and failure rates
	if metrics.TotalOperations > 0 {
		metrics.SuccessRate = float64(metrics.SuccessfulOperations) / float64(metrics.TotalOperations) * 100
		metrics.FailureRate = float64(metrics.FailedOperations) / float64(metrics.TotalOperations) * 100
	}
	
	return metrics
}

// GetOperationMetrics returns metrics for a specific operation
func (rm *RetryManager) GetOperationMetrics(operationName, operationType string) *OperationMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	operationKey := fmt.Sprintf("%s:%s", operationName, operationType)
	opMetrics, exists := rm.metrics.OperationMetrics[operationKey]
	if !exists {
		return nil
	}
	
	// Return a copy
	return &(*opMetrics)
}

// ResetMetrics resets all metrics
func (rm *RetryManager) ResetMetrics() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	rm.metrics = &RetryMetrics{
		RetriesByErrorType: make(map[ErrorType]uint64),
		RetriesByAttempt:   make(map[int]uint64),
		MinDuration:        time.Duration(^uint64(0) >> 1), // Max duration
		MinBackoffTime:     time.Duration(^uint64(0) >> 1), // Max duration
		OperationMetrics:   make(map[string]*OperationMetrics),
	}
}

// Background processes

// metricsLoop runs periodic metrics collection and logging
func (rm *RetryManager) metricsLoop() {
	defer rm.wg.Done()
	
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.logMetrics()
		}
	}
}

// logMetrics logs current metrics
func (rm *RetryManager) logMetrics() {
	if !rm.config.EnableLogging {
		return
	}
	
	metrics := rm.GetMetrics()
	
	rm.logger.Info("Retry manager metrics",
		"total_operations", metrics.TotalOperations,
		"successful_operations", metrics.SuccessfulOperations,
		"failed_operations", metrics.FailedOperations,
		"total_retries", metrics.TotalRetries,
		"success_rate", fmt.Sprintf("%.2f%%", metrics.SuccessRate),
		"failure_rate", fmt.Sprintf("%.2f%%", metrics.FailureRate),
		"average_retries_per_operation", fmt.Sprintf("%.2f", metrics.AverageRetriesPerOperation),
		"average_duration", metrics.AverageDuration,
		"average_backoff_time", metrics.AverageBackoffTime,
		"dead_letter_count", metrics.DeadLetterCount,
		"circuit_breaker_rejects", metrics.CircuitBreakerRejects,
	)
}

// Logging methods

// logSuccess logs a successful operation
func (rm *RetryManager) logSuccess(config *RetryConfig, retryCtx *RetryContext) {
	if config.LogLevel == "debug" || config.LogLevel == "info" {
		rm.logger.Info("Operation completed successfully",
			"operation_name", retryCtx.OperationName,
			"operation_type", retryCtx.OperationType,
			"attempts", retryCtx.Attempt,
			"duration", retryCtx.TotalDuration,
			"tags", retryCtx.Tags,
		)
	}
}

// logRetryAttempt logs a retry attempt
func (rm *RetryManager) logRetryAttempt(config *RetryConfig, retryCtx *RetryContext, err error, backoffDelay time.Duration) {
	if config.LogLevel == "debug" || config.LogLevel == "info" {
		rm.logger.Warn("Retrying operation",
			"operation_name", retryCtx.OperationName,
			"operation_type", retryCtx.OperationType,
			"attempt", retryCtx.Attempt,
			"max_attempts", retryCtx.MaxAttempts,
			"error", err.Error(),
			"error_type", string(retryCtx.LastErrorType),
			"backoff_delay", backoffDelay,
			"strategy", config.Strategy.String(),
			"tags", retryCtx.Tags,
		)
	}
}

// logNonRetryableError logs a non-retryable error
func (rm *RetryManager) logNonRetryableError(config *RetryConfig, retryCtx *RetryContext, err error) {
	rm.logger.Error("Operation failed with non-retryable error",
		"operation_name", retryCtx.OperationName,
		"operation_type", retryCtx.OperationType,
		"attempt", retryCtx.Attempt,
		"error", err.Error(),
		"error_type", string(retryCtx.LastErrorType),
		"duration", retryCtx.TotalDuration,
		"tags", retryCtx.Tags,
	)
}

// logAllAttemptsFailed logs when all retry attempts have failed
func (rm *RetryManager) logAllAttemptsFailed(config *RetryConfig, retryCtx *RetryContext, err error) {
	rm.logger.Error("Operation failed after all retry attempts",
		"operation_name", retryCtx.OperationName,
		"operation_type", retryCtx.OperationType,
		"total_attempts", retryCtx.Attempt,
		"max_attempts", retryCtx.MaxAttempts,
		"final_error", err.Error(),
		"error_type", string(retryCtx.LastErrorType),
		"total_duration", retryCtx.TotalDuration,
		"backoff_duration", retryCtx.BackoffDuration,
		"tags", retryCtx.Tags,
	)
}

// Predefined retry configurations for different operation types

// DatabaseRetryConfig returns retry configuration for database operations
func DatabaseRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:         3,
		BaseDelay:           100 * time.Millisecond,
		MaxDelay:            5 * time.Second,
		Timeout:             30 * time.Second,
		Strategy:            StrategyExponentialBackoff,
		Multiplier:          2.0,
		JitterType:          JitterTypeFull,
		JitterMaxDeviation:  0.1,
		EnableCircuitBreaker: true,
		RetryableErrors:     []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout},
		PermanentErrors:     []ErrorType{ErrorTypeValidation, ErrorTypeClient},
		EnableDeadLetter:    true,
		EnableMetrics:       true,
		EnableLogging:       true,
		LogLevel:            "info",
		OperationName:       "database",
		OperationType:       "query",
		Tags:                map[string]string{"component": "database"},
	}
}

// HTTPRetryConfig returns retry configuration for HTTP operations
func HTTPRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:         5,
		BaseDelay:           200 * time.Millisecond,
		MaxDelay:            10 * time.Second,
		Timeout:             60 * time.Second,
		Strategy:            StrategyExponentialBackoff,
		Multiplier:          2.0,
		JitterType:          JitterTypeEqual,
		JitterMaxDeviation:  0.1,
		EnableCircuitBreaker: true,
		RetryableErrors:     []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeExternal, ErrorTypeRateLimit},
		PermanentErrors:     []ErrorType{ErrorTypeValidation, ErrorTypeClient},
		EnableDeadLetter:    true,
		EnableMetrics:       true,
		EnableLogging:       true,
		LogLevel:            "info",
		OperationName:       "http",
		OperationType:       "request",
		Tags:                map[string]string{"component": "http"},
	}
}

// S3RetryConfig returns retry configuration for S3 operations
func S3RetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:         4,
		BaseDelay:           500 * time.Millisecond,
		MaxDelay:            30 * time.Second,
		Timeout:             5 * time.Minute,
		Strategy:            StrategyExponentialBackoff,
		Multiplier:          2.0,
		JitterType:          JitterTypeFull,
		JitterMaxDeviation:  0.1,
		EnableCircuitBreaker: true,
		RetryableErrors:     []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeExternal, ErrorTypeRateLimit},
		PermanentErrors:     []ErrorType{ErrorTypeValidation, ErrorTypeClient},
		EnableDeadLetter:    true,
		EnableMetrics:       true,
		EnableLogging:       true,
		LogLevel:            "info",
		OperationName:       "s3",
		OperationType:       "upload",
		Tags:                map[string]string{"component": "s3"},
	}
}

// CacheRetryConfig returns retry configuration for cache operations
func CacheRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:         2,
		BaseDelay:           50 * time.Millisecond,
		MaxDelay:            1 * time.Second,
		Timeout:             5 * time.Second,
		Strategy:            StrategyFixedDelay,
		Multiplier:          1.0,
		JitterType:          JitterTypeNone,
		JitterMaxDeviation:  0.0,
		EnableCircuitBreaker: false, // Cache failures are less critical
		RetryableErrors:     []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout},
		PermanentErrors:     []ErrorType{ErrorTypeValidation, ErrorTypeClient},
		EnableDeadLetter:    false,
		EnableMetrics:       true,
		EnableLogging:       true,
		LogLevel:            "warn",
		OperationName:       "cache",
		OperationType:       "get",
		Tags:                map[string]string{"component": "cache"},
	}
}

// KafkaRetryConfig returns retry configuration for Kafka operations
func KafkaRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:         5,
		BaseDelay:           1 * time.Second,
		MaxDelay:            30 * time.Second,
		Timeout:             2 * time.Minute,
		Strategy:            StrategyExponentialBackoff,
		Multiplier:          1.5,
		JitterType:          JitterTypeDecorrelated,
		JitterMaxDeviation:  0.1,
		EnableCircuitBreaker: true,
		RetryableErrors:     []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeExternal},
		PermanentErrors:     []ErrorType{ErrorTypeValidation, ErrorTypeClient},
		EnableDeadLetter:    true,
		EnableMetrics:       true,
		EnableLogging:       true,
		LogLevel:            "info",
		OperationName:       "kafka",
		OperationType:       "produce",
		Tags:                map[string]string{"component": "kafka"},
	}
}

// RetryManagerPool manages multiple retry managers
type RetryManagerPool struct {
	managers map[string]*RetryManager
	mu       sync.RWMutex
	logger   *logger.Logger
}

// NewRetryManagerPool creates a new retry manager pool
func NewRetryManagerPool(logger *logger.Logger) *RetryManagerPool {
	return &RetryManagerPool{
		managers: make(map[string]*RetryManager),
		logger:   logger,
	}
}

// GetManager returns a retry manager for a specific operation type
func (pool *RetryManagerPool) GetManager(operationType string) *RetryManager {
	pool.mu.RLock()
	manager, exists := pool.managers[operationType]
	pool.mu.RUnlock()
	
	if exists {
		return manager
	}
	
	pool.mu.Lock()
	defer pool.mu.Unlock()
	
	// Double-check pattern
	if manager, exists = pool.managers[operationType]; exists {
		return manager
	}
	
	// Create new manager based on operation type
	var config *RetryConfig
	var circuitBreaker *CircuitBreaker
	
	switch operationType {
	case "database":
		config = DatabaseRetryConfig()
		circuitBreaker = NewDatabaseCircuitBreaker(pool.logger)
	case "http":
		config = HTTPRetryConfig()
		circuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig(), pool.logger)
	case "s3":
		config = S3RetryConfig()
		circuitBreaker = NewS3CircuitBreaker(pool.logger)
	case "cache":
		config = CacheRetryConfig()
		circuitBreaker = NewCacheCircuitBreaker(pool.logger)
	case "kafka":
		config = KafkaRetryConfig()
		circuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig(), pool.logger)
	default:
		config = DefaultRetryConfig()
		circuitBreaker = NewCircuitBreaker(DefaultCircuitBreakerConfig(), pool.logger)
	}
	
	manager = NewRetryManager(config, circuitBreaker, pool.logger)
	pool.managers[operationType] = manager
	
	return manager
}

// AddManager adds a custom retry manager to the pool
func (pool *RetryManagerPool) AddManager(operationType string, manager *RetryManager) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	
	pool.managers[operationType] = manager
}

// GetAllManagers returns all retry managers
func (pool *RetryManagerPool) GetAllManagers() map[string]*RetryManager {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	
	result := make(map[string]*RetryManager)
	for k, v := range pool.managers {
		result[k] = v
	}
	
	return result
}

// Close closes all retry managers
func (pool *RetryManagerPool) Close() {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	
	for _, manager := range pool.managers {
		manager.Close()
	}
}

// GetAggregatedMetrics returns aggregated metrics from all managers
func (pool *RetryManagerPool) GetAggregatedMetrics() map[string]*RetryMetrics {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	
	metrics := make(map[string]*RetryMetrics)
	for operationType, manager := range pool.managers {
		metrics[operationType] = manager.GetMetrics()
	}
	
	return metrics
}