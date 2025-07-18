package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int32

const (
	// StateClosed - Circuit is closed, requests are allowed
	StateClosed CircuitState = iota
	// StateOpen - Circuit is open, requests are rejected
	StateOpen
	// StateHalfOpen - Circuit is half-open, limited requests are allowed
	StateHalfOpen
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerError represents errors from the circuit breaker
type CircuitBreakerError struct {
	State   CircuitState
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return fmt.Sprintf("circuit breaker %s: %s", e.State, e.Message)
}

// IsCircuitBreakerError checks if an error is a circuit breaker error
func IsCircuitBreakerError(err error) bool {
	_, ok := err.(*CircuitBreakerError)
	return ok
}

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	// Name of the circuit breaker
	Name string `json:"name"`
	
	// Failure threshold configuration
	FailureThreshold     uint32        `json:"failure_threshold"`      // Number of failures before opening
	SuccessThreshold     uint32        `json:"success_threshold"`      // Number of successes before closing
	ConsecutiveFailures  uint32        `json:"consecutive_failures"`   // Consecutive failures before opening
	MinimumRequestCount  uint32        `json:"minimum_request_count"`  // Minimum requests before evaluating
	
	// Timeout configuration
	RequestTimeout       time.Duration `json:"request_timeout"`        // Individual request timeout
	OpenTimeout          time.Duration `json:"open_timeout"`           // Time to wait before half-open
	HalfOpenTimeout      time.Duration `json:"half_open_timeout"`      // Time to wait in half-open state
	
	// Window configuration
	WindowSize           time.Duration `json:"window_size"`            // Size of the sliding window
	WindowBuckets        int           `json:"window_buckets"`         // Number of buckets in the window
	
	// Retry configuration
	MaxRetries           int           `json:"max_retries"`            // Maximum retry attempts
	RetryDelay           time.Duration `json:"retry_delay"`            // Delay between retries
	RetryBackoffFactor   float64       `json:"retry_backoff_factor"`   // Exponential backoff factor
	
	// Behavior configuration
	FailFast             bool          `json:"fail_fast"`              // Fail immediately when open
	EnableFallback       bool          `json:"enable_fallback"`        // Enable fallback mechanisms
	EnableMetrics        bool          `json:"enable_metrics"`         // Enable metrics collection
	EnableLogging        bool          `json:"enable_logging"`         // Enable detailed logging
	
	// Health check configuration
	HealthCheckInterval  time.Duration `json:"health_check_interval"`  // Health check interval
	HealthCheckTimeout   time.Duration `json:"health_check_timeout"`   // Health check timeout
	EnableHealthCheck    bool          `json:"enable_health_check"`    // Enable health checks
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:                 "default",
		FailureThreshold:     5,
		SuccessThreshold:     3,
		ConsecutiveFailures:  3,
		MinimumRequestCount:  10,
		RequestTimeout:       30 * time.Second,
		OpenTimeout:          60 * time.Second,
		HalfOpenTimeout:      30 * time.Second,
		WindowSize:           60 * time.Second,
		WindowBuckets:        10,
		MaxRetries:           3,
		RetryDelay:           1 * time.Second,
		RetryBackoffFactor:   2.0,
		FailFast:             true,
		EnableFallback:       true,
		EnableMetrics:        true,
		EnableLogging:        true,
		HealthCheckInterval:  30 * time.Second,
		HealthCheckTimeout:   5 * time.Second,
		EnableHealthCheck:    true,
	}
}

// CircuitBreakerMetrics holds metrics for circuit breaker operations
type CircuitBreakerMetrics struct {
	// Request statistics
	TotalRequests       uint64 `json:"total_requests"`
	TotalSuccesses      uint64 `json:"total_successes"`
	TotalFailures       uint64 `json:"total_failures"`
	TotalTimeouts       uint64 `json:"total_timeouts"`
	TotalRejections     uint64 `json:"total_rejections"`
	
	// State statistics
	StateTransitions    uint64 `json:"state_transitions"`
	TimesOpened         uint64 `json:"times_opened"`
	TimesClosed         uint64 `json:"times_closed"`
	TimesHalfOpened     uint64 `json:"times_half_opened"`
	
	// Timing statistics
	AverageResponseTime time.Duration `json:"average_response_time"`
	MaxResponseTime     time.Duration `json:"max_response_time"`
	MinResponseTime     time.Duration `json:"min_response_time"`
	
	// Current state
	CurrentState        CircuitState  `json:"current_state"`
	LastStateChange     time.Time     `json:"last_state_change"`
	LastFailure         time.Time     `json:"last_failure"`
	LastSuccess         time.Time     `json:"last_success"`
	
	// Rates
	SuccessRate         float64       `json:"success_rate"`
	FailureRate         float64       `json:"failure_rate"`
	
	// Fallback statistics
	FallbackExecutions  uint64        `json:"fallback_executions"`
	FallbackSuccesses   uint64        `json:"fallback_successes"`
	FallbackFailures    uint64        `json:"fallback_failures"`
}

// SlidingWindow represents a sliding window for tracking requests
type SlidingWindow struct {
	buckets     []WindowBucket
	size        time.Duration
	bucketSize  time.Duration
	currentIdx  int
	mu          sync.RWMutex
}

// WindowBucket represents a time bucket in the sliding window
type WindowBucket struct {
	timestamp time.Time
	requests  uint64
	successes uint64
	failures  uint64
	timeouts  uint64
}

// NewSlidingWindow creates a new sliding window
func NewSlidingWindow(size time.Duration, bucketCount int) *SlidingWindow {
	bucketSize := size / time.Duration(bucketCount)
	buckets := make([]WindowBucket, bucketCount)
	
	now := time.Now()
	for i := range buckets {
		buckets[i] = WindowBucket{
			timestamp: now.Add(-size + time.Duration(i)*bucketSize),
		}
	}
	
	return &SlidingWindow{
		buckets:    buckets,
		size:       size,
		bucketSize: bucketSize,
		currentIdx: 0,
	}
}

// Record records a request result in the sliding window
func (w *SlidingWindow) Record(success bool, timeout bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	now := time.Now()
	
	// Update current bucket or create new one if needed
	if now.Sub(w.buckets[w.currentIdx].timestamp) >= w.bucketSize {
		w.currentIdx = (w.currentIdx + 1) % len(w.buckets)
		w.buckets[w.currentIdx] = WindowBucket{
			timestamp: now,
		}
	}
	
	bucket := &w.buckets[w.currentIdx]
	bucket.requests++
	
	if timeout {
		bucket.timeouts++
	} else if success {
		bucket.successes++
	} else {
		bucket.failures++
	}
}

// GetStats returns current statistics from the sliding window
func (w *SlidingWindow) GetStats() (requests, successes, failures, timeouts uint64) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-w.size)
	
	for _, bucket := range w.buckets {
		if bucket.timestamp.After(cutoff) {
			requests += bucket.requests
			successes += bucket.successes
			failures += bucket.failures
			timeouts += bucket.timeouts
		}
	}
	
	return
}

// FallbackFunc represents a fallback function
type FallbackFunc func(ctx context.Context, err error) (interface{}, error)

// HealthCheckFunc represents a health check function
type HealthCheckFunc func(ctx context.Context) error

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config           *CircuitBreakerConfig
	state            int32 // atomic: CircuitState
	window           *SlidingWindow
	metrics          *CircuitBreakerMetrics
	fallback         FallbackFunc
	healthCheck      HealthCheckFunc
	logger           *logger.Logger
	
	// State management
	lastStateChange  time.Time
	consecutiveFailures uint32
	consecutiveSuccesses uint32
	halfOpenRequests uint32
	
	// Synchronization
	mu               sync.RWMutex
	stateMu          sync.Mutex
	
	// Background processes
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger *logger.Logger) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	cb := &CircuitBreaker{
		config:          config,
		state:           int32(StateClosed),
		window:          NewSlidingWindow(config.WindowSize, config.WindowBuckets),
		metrics:         &CircuitBreakerMetrics{
			CurrentState:    StateClosed,
			LastStateChange: time.Now(),
			MinResponseTime: time.Duration(^uint64(0) >> 1), // Max duration
		},
		logger:          logger,
		lastStateChange: time.Now(),
		ctx:             ctx,
		cancel:          cancel,
	}
	
	// Start background processes
	if config.EnableHealthCheck {
		cb.wg.Add(1)
		go cb.healthCheckLoop()
	}
	
	if config.EnableMetrics {
		cb.wg.Add(1)
		go cb.metricsLoop()
	}
	
	return cb
}

// Close closes the circuit breaker and stops background processes
func (cb *CircuitBreaker) Close() {
	cb.cancel()
	cb.wg.Wait()
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	return cb.ExecuteWithFallback(ctx, fn, cb.fallback)
}

// ExecuteWithFallback executes a function with circuit breaker protection and fallback
func (cb *CircuitBreaker) ExecuteWithFallback(ctx context.Context, fn func(ctx context.Context) (interface{}, error), fallback FallbackFunc) (interface{}, error) {
	startTime := time.Now()
	
	// Check if we can execute the request
	if err := cb.canExecute(); err != nil {
		cb.recordRejection()
		
		// Try fallback if available
		if fallback != nil && cb.config.EnableFallback {
			return cb.executeFallback(ctx, fallback, err)
		}
		
		return nil, err
	}
	
	// Create timeout context
	timeoutCtx := ctx
	if cb.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, cb.config.RequestTimeout)
		defer cancel()
	}
	
	// Execute the function
	result, err := cb.executeWithRetry(timeoutCtx, fn)
	
	// Record the result
	duration := time.Since(startTime)
	cb.recordResult(err, duration)
	
	// If execution failed and fallback is available, try fallback
	if err != nil && fallback != nil && cb.config.EnableFallback {
		return cb.executeFallback(ctx, fallback, err)
	}
	
	return result, err
}

// canExecute checks if a request can be executed based on circuit breaker state
func (cb *CircuitBreaker) canExecute() error {
	state := CircuitState(atomic.LoadInt32(&cb.state))
	
	switch state {
	case StateClosed:
		return nil
	case StateOpen:
		// Check if we should transition to half-open
		if cb.shouldTransitionToHalfOpen() {
			cb.transitionToHalfOpen()
			return nil
		}
		return &CircuitBreakerError{
			State:   StateOpen,
			Message: "circuit breaker is open",
		}
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if atomic.LoadUint32(&cb.halfOpenRequests) < cb.config.SuccessThreshold {
			atomic.AddUint32(&cb.halfOpenRequests, 1)
			return nil
		}
		return &CircuitBreakerError{
			State:   StateHalfOpen,
			Message: "circuit breaker is half-open and at capacity",
		}
	default:
		return &CircuitBreakerError{
			State:   state,
			Message: "unknown circuit breaker state",
		}
	}
}

// executeWithRetry executes a function with retry logic
func (cb *CircuitBreaker) executeWithRetry(ctx context.Context, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	var lastErr error
	retryDelay := cb.config.RetryDelay
	
	for attempt := 0; attempt <= cb.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// Execute the function
		result, err := fn(ctx)
		
		// If successful, return immediately
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		
		// Don't retry on the last attempt
		if attempt == cb.config.MaxRetries {
			break
		}
		
		// Check if error is retryable
		if !cb.isRetryableError(err) {
			break
		}
		
		// Wait before retry
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
		}
		
		// Exponential backoff
		retryDelay = time.Duration(float64(retryDelay) * cb.config.RetryBackoffFactor)
	}
	
	return nil, lastErr
}

// executeFallback executes the fallback function
func (cb *CircuitBreaker) executeFallback(ctx context.Context, fallback FallbackFunc, originalErr error) (interface{}, error) {
	atomic.AddUint64(&cb.metrics.FallbackExecutions, 1)
	
	result, err := fallback(ctx, originalErr)
	
	if err != nil {
		atomic.AddUint64(&cb.metrics.FallbackFailures, 1)
	} else {
		atomic.AddUint64(&cb.metrics.FallbackSuccesses, 1)
	}
	
	return result, err
}

// recordResult records the result of an operation
func (cb *CircuitBreaker) recordResult(err error, duration time.Duration) {
	isTimeout := cb.isTimeoutError(err)
	isSuccess := err == nil
	
	// Record in sliding window
	cb.window.Record(isSuccess, isTimeout)
	
	// Update metrics
	cb.updateMetrics(isSuccess, isTimeout, duration)
	
	// Update state based on result
	if isSuccess {
		cb.recordSuccess()
	} else {
		cb.recordFailure()
	}
}

// recordSuccess records a successful operation
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.AddUint64(&cb.metrics.TotalSuccesses, 1)
	cb.metrics.LastSuccess = time.Now()
	
	// Reset consecutive failures
	atomic.StoreUint32(&cb.consecutiveFailures, 0)
	
	// Increment consecutive successes
	successCount := atomic.AddUint32(&cb.consecutiveSuccesses, 1)
	
	// Check if we should close the circuit
	state := CircuitState(atomic.LoadInt32(&cb.state))
	if state == StateHalfOpen && successCount >= cb.config.SuccessThreshold {
		cb.transitionToClosed()
	}
}

// recordFailure records a failed operation
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.AddUint64(&cb.metrics.TotalFailures, 1)
	cb.metrics.LastFailure = time.Now()
	
	// Reset consecutive successes
	atomic.StoreUint32(&cb.consecutiveSuccesses, 0)
	
	// Increment consecutive failures
	failureCount := atomic.AddUint32(&cb.consecutiveFailures, 1)
	
	// Check if we should open the circuit
	state := CircuitState(atomic.LoadInt32(&cb.state))
	if (state == StateClosed || state == StateHalfOpen) && cb.shouldOpen(failureCount) {
		cb.transitionToOpen()
	}
}

// recordRejection records a rejected request
func (cb *CircuitBreaker) recordRejection() {
	atomic.AddUint64(&cb.metrics.TotalRejections, 1)
}

// shouldOpen determines if the circuit should open
func (cb *CircuitBreaker) shouldOpen(consecutiveFailures uint32) bool {
	// Check consecutive failures
	if consecutiveFailures >= cb.config.ConsecutiveFailures {
		return true
	}
	
	// Check failure rate in sliding window
	requests, _, failures, timeouts := cb.window.GetStats()
	
	// Need minimum requests to evaluate
	if requests < uint64(cb.config.MinimumRequestCount) {
		return false
	}
	
	// Calculate failure rate
	totalFailures := failures + timeouts
	failureRate := float64(totalFailures) / float64(requests) * 100
	
	// Check if failure rate exceeds threshold
	return totalFailures >= uint64(cb.config.FailureThreshold) || failureRate > 50.0
}

// shouldTransitionToHalfOpen determines if the circuit should transition to half-open
func (cb *CircuitBreaker) shouldTransitionToHalfOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return time.Since(cb.lastStateChange) >= cb.config.OpenTimeout
}

// State transition methods

// transitionToOpen transitions the circuit to open state
func (cb *CircuitBreaker) transitionToOpen() {
	cb.stateMu.Lock()
	defer cb.stateMu.Unlock()
	
	if CircuitState(atomic.LoadInt32(&cb.state)) == StateOpen {
		return // Already open
	}
	
	atomic.StoreInt32(&cb.state, int32(StateOpen))
	cb.lastStateChange = time.Now()
	
	// Update metrics
	cb.metrics.CurrentState = StateOpen
	cb.metrics.LastStateChange = cb.lastStateChange
	atomic.AddUint64(&cb.metrics.StateTransitions, 1)
	atomic.AddUint64(&cb.metrics.TimesOpened, 1)
	
	if cb.config.EnableLogging {
		cb.logger.Warn("Circuit breaker opened",
			"name", cb.config.Name,
			"consecutive_failures", atomic.LoadUint32(&cb.consecutiveFailures),
		)
	}
}

// transitionToHalfOpen transitions the circuit to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.stateMu.Lock()
	defer cb.stateMu.Unlock()
	
	if CircuitState(atomic.LoadInt32(&cb.state)) == StateHalfOpen {
		return // Already half-open
	}
	
	atomic.StoreInt32(&cb.state, int32(StateHalfOpen))
	atomic.StoreUint32(&cb.halfOpenRequests, 0)
	cb.lastStateChange = time.Now()
	
	// Update metrics
	cb.metrics.CurrentState = StateHalfOpen
	cb.metrics.LastStateChange = cb.lastStateChange
	atomic.AddUint64(&cb.metrics.StateTransitions, 1)
	atomic.AddUint64(&cb.metrics.TimesHalfOpened, 1)
	
	if cb.config.EnableLogging {
		cb.logger.Info("Circuit breaker transitioned to half-open",
			"name", cb.config.Name,
		)
	}
}

// transitionToClosed transitions the circuit to closed state
func (cb *CircuitBreaker) transitionToClosed() {
	cb.stateMu.Lock()
	defer cb.stateMu.Unlock()
	
	if CircuitState(atomic.LoadInt32(&cb.state)) == StateClosed {
		return // Already closed
	}
	
	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreUint32(&cb.consecutiveFailures, 0)
	atomic.StoreUint32(&cb.consecutiveSuccesses, 0)
	atomic.StoreUint32(&cb.halfOpenRequests, 0)
	cb.lastStateChange = time.Now()
	
	// Update metrics
	cb.metrics.CurrentState = StateClosed
	cb.metrics.LastStateChange = cb.lastStateChange
	atomic.AddUint64(&cb.metrics.StateTransitions, 1)
	atomic.AddUint64(&cb.metrics.TimesClosed, 1)
	
	if cb.config.EnableLogging {
		cb.logger.Info("Circuit breaker closed",
			"name", cb.config.Name,
		)
	}
}

// updateMetrics updates circuit breaker metrics
func (cb *CircuitBreaker) updateMetrics(success, timeout bool, duration time.Duration) {
	atomic.AddUint64(&cb.metrics.TotalRequests, 1)
	
	if timeout {
		atomic.AddUint64(&cb.metrics.TotalTimeouts, 1)
	}
	
	// Update timing metrics
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if duration < cb.metrics.MinResponseTime {
		cb.metrics.MinResponseTime = duration
	}
	
	if duration > cb.metrics.MaxResponseTime {
		cb.metrics.MaxResponseTime = duration
	}
	
	// Update average response time (simple moving average)
	if cb.metrics.AverageResponseTime == 0 {
		cb.metrics.AverageResponseTime = duration
	} else {
		cb.metrics.AverageResponseTime = (cb.metrics.AverageResponseTime + duration) / 2
	}
	
	// Update success/failure rates
	totalRequests := atomic.LoadUint64(&cb.metrics.TotalRequests)
	if totalRequests > 0 {
		cb.metrics.SuccessRate = float64(atomic.LoadUint64(&cb.metrics.TotalSuccesses)) / float64(totalRequests) * 100
		cb.metrics.FailureRate = float64(atomic.LoadUint64(&cb.metrics.TotalFailures)) / float64(totalRequests) * 100
	}
}

// isRetryableError determines if an error is retryable
func (cb *CircuitBreaker) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Don't retry circuit breaker errors
	if IsCircuitBreakerError(err) {
		return false
	}
	
	// Don't retry context cancellation errors
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	
	// By default, retry other errors
	return true
}

// isTimeoutError determines if an error is a timeout error
func (cb *CircuitBreaker) isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	
	return err == context.DeadlineExceeded
}

// Background processes

// healthCheckLoop runs periodic health checks
func (cb *CircuitBreaker) healthCheckLoop() {
	defer cb.wg.Done()
	
	if cb.healthCheck == nil {
		return
	}
	
	ticker := time.NewTicker(cb.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-cb.ctx.Done():
			return
		case <-ticker.C:
			cb.performHealthCheck()
		}
	}
}

// performHealthCheck performs a health check
func (cb *CircuitBreaker) performHealthCheck() {
	if cb.healthCheck == nil {
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), cb.config.HealthCheckTimeout)
	defer cancel()
	
	err := cb.healthCheck(ctx)
	state := CircuitState(atomic.LoadInt32(&cb.state))
	
	if err != nil {
		if cb.config.EnableLogging {
			cb.logger.Error("Health check failed",
				"name", cb.config.Name,
				"state", state,
				"error", err,
			)
		}
		
		// If health check fails in closed state, consider opening
		if state == StateClosed {
			cb.recordFailure()
		}
	} else {
		if cb.config.EnableLogging {
			cb.logger.Debug("Health check passed",
				"name", cb.config.Name,
				"state", state,
			)
		}
		
		// If health check passes in open state, consider half-opening
		if state == StateOpen && cb.shouldTransitionToHalfOpen() {
			cb.transitionToHalfOpen()
		}
	}
}

// metricsLoop runs periodic metrics collection
func (cb *CircuitBreaker) metricsLoop() {
	defer cb.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-cb.ctx.Done():
			return
		case <-ticker.C:
			cb.logMetrics()
		}
	}
}

// logMetrics logs current metrics
func (cb *CircuitBreaker) logMetrics() {
	if !cb.config.EnableLogging {
		return
	}
	
	metrics := cb.GetMetrics()
	
	cb.logger.Info("Circuit breaker metrics",
		"name", cb.config.Name,
		"state", metrics.CurrentState,
		"total_requests", metrics.TotalRequests,
		"total_successes", metrics.TotalSuccesses,
		"total_failures", metrics.TotalFailures,
		"total_timeouts", metrics.TotalTimeouts,
		"total_rejections", metrics.TotalRejections,
		"success_rate", fmt.Sprintf("%.2f%%", metrics.SuccessRate),
		"failure_rate", fmt.Sprintf("%.2f%%", metrics.FailureRate),
		"average_response_time", metrics.AverageResponseTime,
		"fallback_executions", metrics.FallbackExecutions,
	)
}

// Public API methods

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	return CircuitState(atomic.LoadInt32(&cb.state))
}

// GetMetrics returns current metrics
func (cb *CircuitBreaker) GetMetrics() *CircuitBreakerMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := &CircuitBreakerMetrics{
		TotalRequests:       atomic.LoadUint64(&cb.metrics.TotalRequests),
		TotalSuccesses:      atomic.LoadUint64(&cb.metrics.TotalSuccesses),
		TotalFailures:       atomic.LoadUint64(&cb.metrics.TotalFailures),
		TotalTimeouts:       atomic.LoadUint64(&cb.metrics.TotalTimeouts),
		TotalRejections:     atomic.LoadUint64(&cb.metrics.TotalRejections),
		StateTransitions:    atomic.LoadUint64(&cb.metrics.StateTransitions),
		TimesOpened:         atomic.LoadUint64(&cb.metrics.TimesOpened),
		TimesClosed:         atomic.LoadUint64(&cb.metrics.TimesClosed),
		TimesHalfOpened:     atomic.LoadUint64(&cb.metrics.TimesHalfOpened),
		AverageResponseTime: cb.metrics.AverageResponseTime,
		MaxResponseTime:     cb.metrics.MaxResponseTime,
		MinResponseTime:     cb.metrics.MinResponseTime,
		CurrentState:        cb.metrics.CurrentState,
		LastStateChange:     cb.metrics.LastStateChange,
		LastFailure:         cb.metrics.LastFailure,
		LastSuccess:         cb.metrics.LastSuccess,
		SuccessRate:         cb.metrics.SuccessRate,
		FailureRate:         cb.metrics.FailureRate,
		FallbackExecutions:  atomic.LoadUint64(&cb.metrics.FallbackExecutions),
		FallbackSuccesses:   atomic.LoadUint64(&cb.metrics.FallbackSuccesses),
		FallbackFailures:    atomic.LoadUint64(&cb.metrics.FallbackFailures),
	}
	
	return metrics
}

// SetFallback sets the fallback function
func (cb *CircuitBreaker) SetFallback(fallback FallbackFunc) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.fallback = fallback
}

// SetHealthCheck sets the health check function
func (cb *CircuitBreaker) SetHealthCheck(healthCheck HealthCheckFunc) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.healthCheck = healthCheck
}

// ForceOpen forces the circuit breaker to open state
func (cb *CircuitBreaker) ForceOpen() {
	cb.transitionToOpen()
}

// ForceClose forces the circuit breaker to closed state
func (cb *CircuitBreaker) ForceClose() {
	cb.transitionToClosed()
}

// ForceHalfOpen forces the circuit breaker to half-open state
func (cb *CircuitBreaker) ForceHalfOpen() {
	cb.transitionToHalfOpen()
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreUint32(&cb.consecutiveFailures, 0)
	atomic.StoreUint32(&cb.consecutiveSuccesses, 0)
	atomic.StoreUint32(&cb.halfOpenRequests, 0)
	cb.lastStateChange = time.Now()
	
	// Reset metrics
	cb.metrics = &CircuitBreakerMetrics{
		CurrentState:    StateClosed,
		LastStateChange: time.Now(),
		MinResponseTime: time.Duration(^uint64(0) >> 1), // Max duration
	}
	
	// Reset sliding window
	cb.window = NewSlidingWindow(cb.config.WindowSize, cb.config.WindowBuckets)
	
	if cb.config.EnableLogging {
		cb.logger.Info("Circuit breaker reset", "name", cb.config.Name)
	}
}

// IsOpen returns true if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.GetState() == StateOpen
}

// IsClosed returns true if the circuit breaker is closed
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.GetState() == StateClosed
}

// IsHalfOpen returns true if the circuit breaker is half-open
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.GetState() == StateHalfOpen
}

// GetName returns the name of the circuit breaker
func (cb *CircuitBreaker) GetName() string {
	return cb.config.Name
}

// GetConfig returns the circuit breaker configuration
func (cb *CircuitBreaker) GetConfig() *CircuitBreakerConfig {
	return cb.config
}

// Service-specific circuit breaker implementations

// S3CircuitBreaker creates a circuit breaker for S3 operations
func NewS3CircuitBreaker(logger *logger.Logger) *CircuitBreaker {
	config := &CircuitBreakerConfig{
		Name:                 "s3",
		FailureThreshold:     5,
		SuccessThreshold:     3,
		ConsecutiveFailures:  3,
		MinimumRequestCount:  10,
		RequestTimeout:       30 * time.Second,
		OpenTimeout:          60 * time.Second,
		HalfOpenTimeout:      30 * time.Second,
		WindowSize:           60 * time.Second,
		WindowBuckets:        10,
		MaxRetries:           3,
		RetryDelay:           1 * time.Second,
		RetryBackoffFactor:   2.0,
		FailFast:             true,
		EnableFallback:       true,
		EnableMetrics:        true,
		EnableLogging:        true,
		HealthCheckInterval:  30 * time.Second,
		HealthCheckTimeout:   5 * time.Second,
		EnableHealthCheck:    false, // S3 doesn't have a simple health check
	}
	
	return NewCircuitBreaker(config, logger)
}

// DatabaseCircuitBreaker creates a circuit breaker for database operations
func NewDatabaseCircuitBreaker(logger *logger.Logger) *CircuitBreaker {
	config := &CircuitBreakerConfig{
		Name:                 "database",
		FailureThreshold:     3,
		SuccessThreshold:     2,
		ConsecutiveFailures:  2,
		MinimumRequestCount:  5,
		RequestTimeout:       10 * time.Second,
		OpenTimeout:          30 * time.Second,
		HalfOpenTimeout:      15 * time.Second,
		WindowSize:           30 * time.Second,
		WindowBuckets:        6,
		MaxRetries:           2,
		RetryDelay:           500 * time.Millisecond,
		RetryBackoffFactor:   2.0,
		FailFast:             true,
		EnableFallback:       true,
		EnableMetrics:        true,
		EnableLogging:        true,
		HealthCheckInterval:  15 * time.Second,
		HealthCheckTimeout:   3 * time.Second,
		EnableHealthCheck:    true,
	}
	
	return NewCircuitBreaker(config, logger)
}

// CacheCircuitBreaker creates a circuit breaker for cache operations
func NewCacheCircuitBreaker(logger *logger.Logger) *CircuitBreaker {
	config := &CircuitBreakerConfig{
		Name:                 "cache",
		FailureThreshold:     10,
		SuccessThreshold:     5,
		ConsecutiveFailures:  5,
		MinimumRequestCount:  20,
		RequestTimeout:       5 * time.Second,
		OpenTimeout:          15 * time.Second,
		HalfOpenTimeout:      10 * time.Second,
		WindowSize:           30 * time.Second,
		WindowBuckets:        6,
		MaxRetries:           1,
		RetryDelay:           100 * time.Millisecond,
		RetryBackoffFactor:   1.5,
		FailFast:             false, // Cache failures are less critical
		EnableFallback:       true,
		EnableMetrics:        true,
		EnableLogging:        true,
		HealthCheckInterval:  10 * time.Second,
		HealthCheckTimeout:   2 * time.Second,
		EnableHealthCheck:    true,
	}
	
	return NewCircuitBreaker(config, logger)
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
	logger   *logger.Logger
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(logger *logger.Logger) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger,
	}
}

// AddCircuitBreaker adds a circuit breaker to the manager
func (m *CircuitBreakerManager) AddCircuitBreaker(name string, breaker *CircuitBreaker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.breakers[name] = breaker
}

// GetCircuitBreaker returns a circuit breaker by name
func (m *CircuitBreakerManager) GetCircuitBreaker(name string) (*CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	breaker, exists := m.breakers[name]
	return breaker, exists
}

// GetAllCircuitBreakers returns all circuit breakers
func (m *CircuitBreakerManager) GetAllCircuitBreakers() map[string]*CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]*CircuitBreaker)
	for name, breaker := range m.breakers {
		result[name] = breaker
	}
	
	return result
}

// Close closes all circuit breakers
func (m *CircuitBreakerManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, breaker := range m.breakers {
		breaker.Close()
	}
}

// GetMetrics returns metrics for all circuit breakers
func (m *CircuitBreakerManager) GetMetrics() map[string]*CircuitBreakerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	metrics := make(map[string]*CircuitBreakerMetrics)
	for name, breaker := range m.breakers {
		metrics[name] = breaker.GetMetrics()
	}
	
	return metrics
}