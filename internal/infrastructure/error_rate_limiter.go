package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// RateLimitStrategy represents the rate limiting strategy
type RateLimitStrategy string

const (
	RateLimitStrategyFixed   RateLimitStrategy = "fixed"   // Fixed window
	RateLimitStrategySliding RateLimitStrategy = "sliding" // Sliding window
	RateLimitStrategyToken   RateLimitStrategy = "token"   // Token bucket
	RateLimitStrategyLeaky   RateLimitStrategy = "leaky"   // Leaky bucket
)

// RateLimitRule represents a rate limiting rule
type RateLimitRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Strategy    RateLimitStrategy `json:"strategy"`
	Limit       int               `json:"limit"`
	Window      time.Duration     `json:"window"`
	ErrorTypes  []ErrorType       `json:"error_types"`
	Severities  []ErrorSeverity   `json:"severities"`
	UserBased   bool              `json:"user_based"`
	IPBased     bool              `json:"ip_based"`
	GlobalLimit bool              `json:"global_limit"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RateLimitBucket represents a rate limiting bucket
type RateLimitBucket struct {
	Key        string            `json:"key"`
	Count      int               `json:"count"`
	LastReset  time.Time         `json:"last_reset"`
	LastAccess time.Time         `json:"last_access"`
	Timestamps []time.Time       `json:"timestamps"`
	Tokens     int               `json:"tokens"`
	MaxTokens  int               `json:"max_tokens"`
	RefillRate time.Duration     `json:"refill_rate"`
	Strategy   RateLimitStrategy `json:"strategy"`
}

// RateLimitResult represents the result of rate limiting
type RateLimitResult struct {
	Allowed      bool          `json:"allowed"`
	Remaining    int           `json:"remaining"`
	ResetTime    time.Time     `json:"reset_time"`
	RetryAfter   time.Duration `json:"retry_after"`
	RuleID       string        `json:"rule_id"`
	Key          string        `json:"key"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// ErrorRateLimiter manages rate limiting for errors
type ErrorRateLimiter struct {
	config *ErrorHandlerConfig
	logger *logger.Logger
	mu     sync.RWMutex

	// Rate limiting rules
	rules   map[string]*RateLimitRule
	buckets map[string]*RateLimitBucket

	// Statistics
	stats map[string]*RateLimitStats

	// Background processes
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}
}

// RateLimitStats represents rate limiting statistics
type RateLimitStats struct {
	RuleID          string    `json:"rule_id"`
	TotalRequests   int64     `json:"total_requests"`
	AllowedRequests int64     `json:"allowed_requests"`
	BlockedRequests int64     `json:"blocked_requests"`
	LastRequest     time.Time `json:"last_request"`
	FirstRequest    time.Time `json:"first_request"`
}

// NewErrorRateLimiter creates a new error rate limiter
func NewErrorRateLimiter(config *ErrorHandlerConfig, logger *logger.Logger) *ErrorRateLimiter {
	ctx, cancel := context.WithCancel(context.Background())

	erl := &ErrorRateLimiter{
		config:     config,
		logger:     logger,
		rules:      make(map[string]*RateLimitRule),
		buckets:    make(map[string]*RateLimitBucket),
		stats:      make(map[string]*RateLimitStats),
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
	}

	// Initialize default rules
	erl.initializeDefaultRules()

	// Start background processes
	erl.startBackgroundProcesses()

	return erl
}

// initializeDefaultRules initializes default rate limiting rules
func (erl *ErrorRateLimiter) initializeDefaultRules() {
	defaultRules := []*RateLimitRule{
		{
			ID:          "global_error_limit",
			Name:        "Global Error Limit",
			Strategy:    RateLimitStrategySliding,
			Limit:       100,
			Window:      time.Minute,
			ErrorTypes:  []ErrorType{},
			Severities:  []ErrorSeverity{},
			UserBased:   false,
			IPBased:     false,
			GlobalLimit: true,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "user_error_limit",
			Name:        "Per-User Error Limit",
			Strategy:    RateLimitStrategyFixed,
			Limit:       50,
			Window:      time.Minute,
			ErrorTypes:  []ErrorType{},
			Severities:  []ErrorSeverity{},
			UserBased:   true,
			IPBased:     false,
			GlobalLimit: false,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "critical_error_limit",
			Name:        "Critical Error Limit",
			Strategy:    RateLimitStrategyToken,
			Limit:       10,
			Window:      time.Minute,
			ErrorTypes:  []ErrorType{},
			Severities:  []ErrorSeverity{SeverityCritical},
			UserBased:   false,
			IPBased:     false,
			GlobalLimit: true,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "validation_error_limit",
			Name:        "Validation Error Limit",
			Strategy:    RateLimitStrategyLeaky,
			Limit:       200,
			Window:      time.Minute,
			ErrorTypes:  []ErrorType{ErrorTypeValidation},
			Severities:  []ErrorSeverity{},
			UserBased:   true,
			IPBased:     true,
			GlobalLimit: false,
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, rule := range defaultRules {
		erl.rules[rule.ID] = rule
	}
}

// ShouldLimit checks if an error should be rate limited
func (erl *ErrorRateLimiter) ShouldLimit(ctx context.Context, err error) bool {
	// Convert to AppError if needed
	var appErr *AppError
	if ae, ok := err.(*AppError); ok {
		appErr = ae
	} else {
		// Create minimal AppError for rate limiting
		appErr = &AppError{
			Type:      ErrorTypeSystem,
			Severity:  SeverityMedium,
			Timestamp: time.Now(),
			UserID:    logger.GetUserID(ctx),
		}
	}

	// Check against all applicable rules
	for _, rule := range erl.rules {
		if !rule.Enabled {
			continue
		}

		if erl.ruleMatches(rule, appErr) {
			result := erl.checkRateLimit(rule, appErr, ctx)
			if !result.Allowed {
				erl.logger.WarnContext(ctx, "Error rate limited",
					"rule_id", rule.ID,
					"rule_name", rule.Name,
					"error_type", appErr.Type,
					"user_id", appErr.UserID,
					"remaining", result.Remaining,
					"retry_after", result.RetryAfter,
				)
				return true
			}
		}
	}

	return false
}

// ruleMatches checks if a rule matches the error
func (erl *ErrorRateLimiter) ruleMatches(rule *RateLimitRule, appErr *AppError) bool {
	// Check error types
	if len(rule.ErrorTypes) > 0 {
		match := false
		for _, errorType := range rule.ErrorTypes {
			if errorType == appErr.Type {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Check severities
	if len(rule.Severities) > 0 {
		match := false
		for _, severity := range rule.Severities {
			if severity == appErr.Severity {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	return true
}

// checkRateLimit checks rate limit for a rule
func (erl *ErrorRateLimiter) checkRateLimit(rule *RateLimitRule, appErr *AppError, ctx context.Context) *RateLimitResult {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	// Generate bucket key
	bucketKey := erl.generateBucketKey(rule, appErr, ctx)

	// Get or create bucket
	bucket, exists := erl.buckets[bucketKey]
	if !exists {
		bucket = erl.createBucket(bucketKey, rule)
		erl.buckets[bucketKey] = bucket
	}

	// Update statistics
	erl.updateStats(rule.ID, appErr)

	// Check rate limit based on strategy
	switch rule.Strategy {
	case RateLimitStrategyFixed:
		return erl.checkFixedWindow(bucket, rule)
	case RateLimitStrategySliding:
		return erl.checkSlidingWindow(bucket, rule)
	case RateLimitStrategyToken:
		return erl.checkTokenBucket(bucket, rule)
	case RateLimitStrategyLeaky:
		return erl.checkLeakyBucket(bucket, rule)
	default:
		return &RateLimitResult{
			Allowed: true,
			RuleID:  rule.ID,
			Key:     bucketKey,
		}
	}
}

// generateBucketKey generates a unique key for the rate limiting bucket
func (erl *ErrorRateLimiter) generateBucketKey(rule *RateLimitRule, appErr *AppError, ctx context.Context) string {
	var keyParts []string

	keyParts = append(keyParts, rule.ID)

	if rule.GlobalLimit {
		keyParts = append(keyParts, "global")
	}

	if rule.UserBased && appErr.UserID != "" {
		keyParts = append(keyParts, "user", appErr.UserID)
	}

	if rule.IPBased {
		// This would typically extract IP from context
		// For simplicity, we'll use a placeholder
		keyParts = append(keyParts, "ip", "127.0.0.1")
	}

	if len(keyParts) == 1 {
		keyParts = append(keyParts, "default")
	}

	return fmt.Sprintf("%s", keyParts)
}

// createBucket creates a new rate limiting bucket
func (erl *ErrorRateLimiter) createBucket(key string, rule *RateLimitRule) *RateLimitBucket {
	now := time.Now()

	bucket := &RateLimitBucket{
		Key:        key,
		Count:      0,
		LastReset:  now,
		LastAccess: now,
		Timestamps: []time.Time{},
		Tokens:     rule.Limit,
		MaxTokens:  rule.Limit,
		RefillRate: rule.Window / time.Duration(rule.Limit),
		Strategy:   rule.Strategy,
	}

	return bucket
}

// checkFixedWindow checks fixed window rate limiting
func (erl *ErrorRateLimiter) checkFixedWindow(bucket *RateLimitBucket, rule *RateLimitRule) *RateLimitResult {
	now := time.Now()

	// Reset if window has passed
	if now.Sub(bucket.LastReset) >= rule.Window {
		bucket.Count = 0
		bucket.LastReset = now
	}

	// Check if limit exceeded
	if bucket.Count >= rule.Limit {
		return &RateLimitResult{
			Allowed:      false,
			Remaining:    0,
			ResetTime:    bucket.LastReset.Add(rule.Window),
			RetryAfter:   bucket.LastReset.Add(rule.Window).Sub(now),
			RuleID:       rule.ID,
			Key:          bucket.Key,
			ErrorMessage: fmt.Sprintf("Rate limit exceeded for rule %s", rule.Name),
		}
	}

	// Increment counter
	bucket.Count++
	bucket.LastAccess = now

	return &RateLimitResult{
		Allowed:   true,
		Remaining: rule.Limit - bucket.Count,
		ResetTime: bucket.LastReset.Add(rule.Window),
		RuleID:    rule.ID,
		Key:       bucket.Key,
	}
}

// checkSlidingWindow checks sliding window rate limiting
func (erl *ErrorRateLimiter) checkSlidingWindow(bucket *RateLimitBucket, rule *RateLimitRule) *RateLimitResult {
	now := time.Now()
	cutoff := now.Add(-rule.Window)

	// Remove old timestamps
	var validTimestamps []time.Time
	for _, ts := range bucket.Timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	bucket.Timestamps = validTimestamps

	// Check if limit exceeded
	if len(bucket.Timestamps) >= rule.Limit {
		oldestTimestamp := bucket.Timestamps[0]
		return &RateLimitResult{
			Allowed:      false,
			Remaining:    0,
			ResetTime:    oldestTimestamp.Add(rule.Window),
			RetryAfter:   oldestTimestamp.Add(rule.Window).Sub(now),
			RuleID:       rule.ID,
			Key:          bucket.Key,
			ErrorMessage: fmt.Sprintf("Rate limit exceeded for rule %s", rule.Name),
		}
	}

	// Add current timestamp
	bucket.Timestamps = append(bucket.Timestamps, now)
	bucket.LastAccess = now

	return &RateLimitResult{
		Allowed:   true,
		Remaining: rule.Limit - len(bucket.Timestamps),
		ResetTime: now.Add(rule.Window),
		RuleID:    rule.ID,
		Key:       bucket.Key,
	}
}

// checkTokenBucket checks token bucket rate limiting
func (erl *ErrorRateLimiter) checkTokenBucket(bucket *RateLimitBucket, rule *RateLimitRule) *RateLimitResult {
	now := time.Now()

	// Refill tokens based on time elapsed
	elapsed := now.Sub(bucket.LastAccess)
	tokensToAdd := int(elapsed / bucket.RefillRate)

	if tokensToAdd > 0 {
		bucket.Tokens = min(bucket.MaxTokens, bucket.Tokens+tokensToAdd)
		bucket.LastAccess = now
	}

	// Check if tokens available
	if bucket.Tokens <= 0 {
		return &RateLimitResult{
			Allowed:      false,
			Remaining:    0,
			ResetTime:    now.Add(bucket.RefillRate),
			RetryAfter:   bucket.RefillRate,
			RuleID:       rule.ID,
			Key:          bucket.Key,
			ErrorMessage: fmt.Sprintf("Rate limit exceeded for rule %s", rule.Name),
		}
	}

	// Consume token
	bucket.Tokens--
	bucket.LastAccess = now

	return &RateLimitResult{
		Allowed:   true,
		Remaining: bucket.Tokens,
		ResetTime: now.Add(bucket.RefillRate),
		RuleID:    rule.ID,
		Key:       bucket.Key,
	}
}

// checkLeakyBucket checks leaky bucket rate limiting
func (erl *ErrorRateLimiter) checkLeakyBucket(bucket *RateLimitBucket, rule *RateLimitRule) *RateLimitResult {
	now := time.Now()

	// Leak tokens based on time elapsed
	elapsed := now.Sub(bucket.LastAccess)
	tokensToLeak := int(elapsed / bucket.RefillRate)

	if tokensToLeak > 0 {
		bucket.Count = max(0, bucket.Count-tokensToLeak)
		bucket.LastAccess = now
	}

	// Check if bucket is full
	if bucket.Count >= rule.Limit {
		return &RateLimitResult{
			Allowed:      false,
			Remaining:    0,
			ResetTime:    now.Add(bucket.RefillRate),
			RetryAfter:   bucket.RefillRate,
			RuleID:       rule.ID,
			Key:          bucket.Key,
			ErrorMessage: fmt.Sprintf("Rate limit exceeded for rule %s", rule.Name),
		}
	}

	// Add to bucket
	bucket.Count++
	bucket.LastAccess = now

	return &RateLimitResult{
		Allowed:   true,
		Remaining: rule.Limit - bucket.Count,
		ResetTime: now.Add(bucket.RefillRate),
		RuleID:    rule.ID,
		Key:       bucket.Key,
	}
}

// updateStats updates rate limiting statistics
func (erl *ErrorRateLimiter) updateStats(ruleID string, appErr *AppError) {
	stats, exists := erl.stats[ruleID]
	if !exists {
		stats = &RateLimitStats{
			RuleID:       ruleID,
			FirstRequest: appErr.Timestamp,
		}
		erl.stats[ruleID] = stats
	}

	stats.TotalRequests++
	stats.LastRequest = appErr.Timestamp
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Public API methods

// AddRule adds a new rate limiting rule
func (erl *ErrorRateLimiter) AddRule(rule *RateLimitRule) {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	erl.rules[rule.ID] = rule
}

// RemoveRule removes a rate limiting rule
func (erl *ErrorRateLimiter) RemoveRule(ruleID string) {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	delete(erl.rules, ruleID)

	// Clean up associated buckets
	for key, bucket := range erl.buckets {
		if bucket.Key == ruleID {
			delete(erl.buckets, key)
		}
	}
}

// GetRules returns all rate limiting rules
func (erl *ErrorRateLimiter) GetRules() map[string]*RateLimitRule {
	erl.mu.RLock()
	defer erl.mu.RUnlock()

	rules := make(map[string]*RateLimitRule)
	for k, v := range erl.rules {
		rules[k] = v
	}

	return rules
}

// GetBuckets returns all rate limiting buckets
func (erl *ErrorRateLimiter) GetBuckets() map[string]*RateLimitBucket {
	erl.mu.RLock()
	defer erl.mu.RUnlock()

	buckets := make(map[string]*RateLimitBucket)
	for k, v := range erl.buckets {
		buckets[k] = v
	}

	return buckets
}

// GetStats returns rate limiting statistics
func (erl *ErrorRateLimiter) GetStats() map[string]*RateLimitStats {
	erl.mu.RLock()
	defer erl.mu.RUnlock()

	stats := make(map[string]*RateLimitStats)
	for k, v := range erl.stats {
		stats[k] = v
	}

	return stats
}

// ResetBucket resets a specific bucket
func (erl *ErrorRateLimiter) ResetBucket(bucketKey string) {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	if bucket, exists := erl.buckets[bucketKey]; exists {
		bucket.Count = 0
		bucket.LastReset = time.Now()
		bucket.Timestamps = []time.Time{}
		bucket.Tokens = bucket.MaxTokens
	}
}

// ResetAllBuckets resets all buckets
func (erl *ErrorRateLimiter) ResetAllBuckets() {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	now := time.Now()
	for _, bucket := range erl.buckets {
		bucket.Count = 0
		bucket.LastReset = now
		bucket.Timestamps = []time.Time{}
		bucket.Tokens = bucket.MaxTokens
	}
}

// GetRateLimitStatus returns current rate limit status
func (erl *ErrorRateLimiter) GetRateLimitStatus() map[string]interface{} {
	erl.mu.RLock()
	defer erl.mu.RUnlock()

	status := map[string]interface{}{
		"total_rules":    len(erl.rules),
		"active_buckets": len(erl.buckets),
		"total_stats":    len(erl.stats),
	}

	// Add rule status
	ruleStatus := make(map[string]interface{})
	for ruleID, rule := range erl.rules {
		ruleStatus[ruleID] = map[string]interface{}{
			"enabled":  rule.Enabled,
			"strategy": rule.Strategy,
			"limit":    rule.Limit,
			"window":   rule.Window.String(),
		}
	}
	status["rules"] = ruleStatus

	// Add bucket status
	bucketStatus := make(map[string]interface{})
	for key, bucket := range erl.buckets {
		bucketStatus[key] = map[string]interface{}{
			"count":       bucket.Count,
			"tokens":      bucket.Tokens,
			"last_access": bucket.LastAccess,
			"strategy":    bucket.Strategy,
		}
	}
	status["buckets"] = bucketStatus

	return status
}

// startBackgroundProcesses starts background rate limiting processes
func (erl *ErrorRateLimiter) startBackgroundProcesses() {
	erl.wg.Add(1)
	go erl.cleanupLoop()
}

// cleanupLoop runs cleanup tasks
func (erl *ErrorRateLimiter) cleanupLoop() {
	defer erl.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			erl.performCleanup()
		case <-erl.shutdownCh:
			return
		}
	}
}

// performCleanup performs cleanup of old buckets and stats
func (erl *ErrorRateLimiter) performCleanup() {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	now := time.Now()

	// Clean up old buckets
	for key, bucket := range erl.buckets {
		if now.Sub(bucket.LastAccess) >= time.Hour {
			delete(erl.buckets, key)
		}
	}

	// Clean up old stats
	for ruleID, stats := range erl.stats {
		if now.Sub(stats.LastRequest) >= 24*time.Hour {
			delete(erl.stats, ruleID)
		}
	}
}

// Close closes the rate limiter
func (erl *ErrorRateLimiter) Close() {
	erl.cancel()
	close(erl.shutdownCh)
	erl.wg.Wait()
}
