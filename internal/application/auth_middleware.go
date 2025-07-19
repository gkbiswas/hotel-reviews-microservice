package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
)

// AuthMiddleware provides comprehensive authentication and authorization middleware
type AuthMiddleware struct {
	authService      *infrastructure.AuthenticationService
	rateLimiter      *RateLimiter
	circuitBreaker   *infrastructure.CircuitBreaker
	metricsCollector *AuthMetricsCollector
	blacklist        *BlacklistManager
	sessionManager   *SessionManager
	logger           *slog.Logger
	config           *AuthMiddlewareConfig
	mu               sync.RWMutex
}

// AuthMiddlewareConfig holds configuration for the authentication middleware
type AuthMiddlewareConfig struct {
	// JWT Configuration
	JWTSecret        string        `json:"jwt_secret"`
	JWTExpiry        time.Duration `json:"jwt_expiry"`
	JWTRefreshExpiry time.Duration `json:"jwt_refresh_expiry"`
	JWTIssuer        string        `json:"jwt_issuer"`

	// Rate Limiting Configuration
	RateLimitEnabled         bool          `json:"rate_limit_enabled"`
	RateLimitRequests        int           `json:"rate_limit_requests"`
	RateLimitWindow          time.Duration `json:"rate_limit_window"`
	RateLimitBurst           int           `json:"rate_limit_burst"`
	RateLimitCleanupInterval time.Duration `json:"rate_limit_cleanup_interval"`

	// Session Configuration
	SessionTimeout         time.Duration `json:"session_timeout"`
	SessionCleanupInterval time.Duration `json:"session_cleanup_interval"`
	MaxActiveSessions      int           `json:"max_active_sessions"`
	SessionCookieName      string        `json:"session_cookie_name"`
	SessionSecure          bool          `json:"session_secure"`
	SessionHttpOnly        bool          `json:"session_http_only"`
	SessionSameSite        http.SameSite `json:"session_same_site"`

	// Security Configuration
	BlacklistEnabled       bool          `json:"blacklist_enabled"`
	BlacklistCheckInterval time.Duration `json:"blacklist_check_interval"`
	WhitelistEnabled       bool          `json:"whitelist_enabled"`
	WhitelistIPs           []string      `json:"whitelist_ips"`
	TrustedProxies         []string      `json:"trusted_proxies"`

	// API Key Configuration
	APIKeyEnabled       bool     `json:"api_key_enabled"`
	APIKeyHeaders       []string `json:"api_key_headers"`
	APIKeyQueryParam    string   `json:"api_key_query_param"`
	APIKeyHashAlgorithm string   `json:"api_key_hash_algorithm"`

	// Audit Configuration
	AuditEnabled          bool          `json:"audit_enabled"`
	AuditLogSensitiveData bool          `json:"audit_log_sensitive_data"`
	AuditBufferSize       int           `json:"audit_buffer_size"`
	AuditFlushInterval    time.Duration `json:"audit_flush_interval"`

	// Circuit Breaker Configuration
	CircuitBreakerEnabled   bool          `json:"circuit_breaker_enabled"`
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   time.Duration `json:"circuit_breaker_timeout"`
	CircuitBreakerReset     time.Duration `json:"circuit_breaker_reset"`

	// Metrics Configuration
	MetricsEnabled            bool          `json:"metrics_enabled"`
	MetricsCollectionInterval time.Duration `json:"metrics_collection_interval"`
	MetricsRetentionPeriod    time.Duration `json:"metrics_retention_period"`

	// CORS Configuration
	CORSEnabled          bool          `json:"cors_enabled"`
	CORSAllowedOrigins   []string      `json:"cors_allowed_origins"`
	CORSAllowedMethods   []string      `json:"cors_allowed_methods"`
	CORSAllowedHeaders   []string      `json:"cors_allowed_headers"`
	CORSExposedHeaders   []string      `json:"cors_exposed_headers"`
	CORSAllowCredentials bool          `json:"cors_allow_credentials"`
	CORSMaxAge           time.Duration `json:"cors_max_age"`

	// Security Headers
	SecurityHeaders       map[string]string `json:"security_headers"`
	EnableCSP             bool              `json:"enable_csp"`
	CSPDirectives         map[string]string `json:"csp_directives"`
	EnableHSTS            bool              `json:"enable_hsts"`
	HSTSMaxAge            time.Duration     `json:"hsts_max_age"`
	HSTSIncludeSubdomains bool              `json:"hsts_include_subdomains"`
	HSTSPreload           bool              `json:"hsts_preload"`
}

// DefaultAuthMiddlewareConfig returns default configuration
func DefaultAuthMiddlewareConfig() *AuthMiddlewareConfig {
	return &AuthMiddlewareConfig{
		JWTExpiry:                 15 * time.Minute,
		JWTRefreshExpiry:          7 * 24 * time.Hour,
		JWTIssuer:                 "hotel-reviews-service",
		RateLimitEnabled:          true,
		RateLimitRequests:         100,
		RateLimitWindow:           time.Minute,
		RateLimitBurst:            10,
		RateLimitCleanupInterval:  10 * time.Minute,
		SessionTimeout:            30 * time.Minute,
		SessionCleanupInterval:    5 * time.Minute,
		MaxActiveSessions:         10,
		SessionCookieName:         "session_id",
		SessionSecure:             true,
		SessionHttpOnly:           true,
		SessionSameSite:           http.SameSiteStrictMode,
		BlacklistEnabled:          true,
		BlacklistCheckInterval:    5 * time.Minute,
		WhitelistEnabled:          false,
		APIKeyEnabled:             true,
		APIKeyHeaders:             []string{"X-API-Key", "Authorization"},
		APIKeyQueryParam:          "api_key",
		APIKeyHashAlgorithm:       "sha256",
		AuditEnabled:              true,
		AuditLogSensitiveData:     false,
		AuditBufferSize:           1000,
		AuditFlushInterval:        30 * time.Second,
		CircuitBreakerEnabled:     true,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     30 * time.Second,
		CircuitBreakerReset:       60 * time.Second,
		MetricsEnabled:            true,
		MetricsCollectionInterval: 30 * time.Second,
		MetricsRetentionPeriod:    24 * time.Hour,
		CORSEnabled:               true,
		CORSAllowedOrigins:        []string{"*"},
		CORSAllowedMethods:        []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSAllowedHeaders:        []string{"Content-Type", "Authorization", "X-API-Key"},
		CORSExposedHeaders:        []string{"X-Total-Count", "X-Request-ID"},
		CORSAllowCredentials:      true,
		CORSMaxAge:                24 * time.Hour,
		SecurityHeaders: map[string]string{
			"X-Frame-Options":        "DENY",
			"X-Content-Type-Options": "nosniff",
			"X-XSS-Protection":       "1; mode=block",
			"Referrer-Policy":        "strict-origin-when-cross-origin",
			"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
		},
		EnableCSP: true,
		CSPDirectives: map[string]string{
			"default-src": "'self'",
			"script-src":  "'self' 'unsafe-inline'",
			"style-src":   "'self' 'unsafe-inline'",
			"img-src":     "'self' data: https:",
			"font-src":    "'self'",
			"connect-src": "'self'",
			"frame-src":   "'none'",
			"object-src":  "'none'",
			"base-uri":    "'self'",
			"form-action": "'self'",
		},
		EnableHSTS:            true,
		HSTSMaxAge:            365 * 24 * time.Hour,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
	}
}

// RateLimiter implements per-user and per-IP rate limiting
type RateLimiter struct {
	userLimits map[string]*UserRateLimit
	ipLimits   map[string]*IPRateLimit
	mu         sync.RWMutex
	config     *AuthMiddlewareConfig
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// UserRateLimit tracks rate limiting for a specific user
type UserRateLimit struct {
	UserID       uuid.UUID
	RequestCount int
	LastReset    time.Time
	BurstUsed    int
	LastRequest  time.Time
	Blocked      bool
	BlockedUntil time.Time
}

// IPRateLimit tracks rate limiting for a specific IP
type IPRateLimit struct {
	IP           string
	RequestCount int
	LastReset    time.Time
	BurstUsed    int
	LastRequest  time.Time
	Blocked      bool
	BlockedUntil time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *AuthMiddlewareConfig, logger *slog.Logger) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())

	rl := &RateLimiter{
		userLimits: make(map[string]*UserRateLimit),
		ipLimits:   make(map[string]*IPRateLimit),
		config:     config,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start cleanup goroutine
	if config.RateLimitEnabled {
		rl.wg.Add(1)
		go rl.cleanupExpiredLimits()
	}

	return rl
}

// Close shuts down the rate limiter
func (rl *RateLimiter) Close() {
	rl.cancel()
	rl.wg.Wait()
}

// CheckUserRateLimit checks if a user is rate limited
func (rl *RateLimiter) CheckUserRateLimit(userID uuid.UUID) bool {
	if !rl.config.RateLimitEnabled {
		return false
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	key := userID.String()
	now := time.Now()

	limit, exists := rl.userLimits[key]
	if !exists {
		limit = &UserRateLimit{
			UserID:       userID,
			RequestCount: 0,
			LastReset:    now,
			BurstUsed:    0,
			LastRequest:  now,
		}
		rl.userLimits[key] = limit
	}

	// Check if user is currently blocked
	if limit.Blocked && now.Before(limit.BlockedUntil) {
		return true
	}

	// Reset window if needed
	if now.Sub(limit.LastReset) >= rl.config.RateLimitWindow {
		limit.RequestCount = 0
		limit.LastReset = now
		limit.BurstUsed = 0
		limit.Blocked = false
	}

	// Check rate limit
	if limit.RequestCount >= rl.config.RateLimitRequests {
		// Check burst allowance
		if limit.BurstUsed >= rl.config.RateLimitBurst {
			limit.Blocked = true
			limit.BlockedUntil = now.Add(rl.config.RateLimitWindow)
			return true
		}
		limit.BurstUsed++
	}

	limit.RequestCount++
	limit.LastRequest = now

	return false
}

// CheckIPRateLimit checks if an IP is rate limited
func (rl *RateLimiter) CheckIPRateLimit(ip string) bool {
	if !rl.config.RateLimitEnabled {
		return false
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	limit, exists := rl.ipLimits[ip]
	if !exists {
		limit = &IPRateLimit{
			IP:           ip,
			RequestCount: 0,
			LastReset:    now,
			BurstUsed:    0,
			LastRequest:  now,
		}
		rl.ipLimits[ip] = limit
	}

	// Check if IP is currently blocked
	if limit.Blocked && now.Before(limit.BlockedUntil) {
		return true
	}

	// Reset window if needed
	if now.Sub(limit.LastReset) >= rl.config.RateLimitWindow {
		limit.RequestCount = 0
		limit.LastReset = now
		limit.BurstUsed = 0
		limit.Blocked = false
	}

	// Check rate limit
	if limit.RequestCount >= rl.config.RateLimitRequests {
		// Check burst allowance
		if limit.BurstUsed >= rl.config.RateLimitBurst {
			limit.Blocked = true
			limit.BlockedUntil = now.Add(rl.config.RateLimitWindow)
			return true
		}
		limit.BurstUsed++
	}

	limit.RequestCount++
	limit.LastRequest = now

	return false
}

// cleanupExpiredLimits removes expired rate limit entries
func (rl *RateLimiter) cleanupExpiredLimits() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.config.RateLimitCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			rl.performCleanup()
		}
	}
}

// performCleanup removes expired rate limit entries
func (rl *RateLimiter) performCleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-2 * rl.config.RateLimitWindow)

	// Clean up user limits
	for key, limit := range rl.userLimits {
		if limit.LastRequest.Before(cutoff) {
			delete(rl.userLimits, key)
		}
	}

	// Clean up IP limits
	for key, limit := range rl.ipLimits {
		if limit.LastRequest.Before(cutoff) {
			delete(rl.ipLimits, key)
		}
	}
}

// GetRateLimitStats returns current rate limit statistics
func (rl *RateLimiter) GetRateLimitStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	stats := map[string]interface{}{
		"user_limits_count": len(rl.userLimits),
		"ip_limits_count":   len(rl.ipLimits),
		"blocked_users":     0,
		"blocked_ips":       0,
	}

	now := time.Now()
	blockedUsers := 0
	blockedIPs := 0

	for _, limit := range rl.userLimits {
		if limit.Blocked && now.Before(limit.BlockedUntil) {
			blockedUsers++
		}
	}

	for _, limit := range rl.ipLimits {
		if limit.Blocked && now.Before(limit.BlockedUntil) {
			blockedIPs++
		}
	}

	stats["blocked_users"] = blockedUsers
	stats["blocked_ips"] = blockedIPs

	return stats
}

// BlacklistManager manages IP and user blacklists
type BlacklistManager struct {
	blacklistedIPs   map[string]time.Time
	blacklistedUsers map[uuid.UUID]time.Time
	whitelistedIPs   map[string]bool
	mu               sync.RWMutex
	config           *AuthMiddlewareConfig
	logger           *slog.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// NewBlacklistManager creates a new blacklist manager
func NewBlacklistManager(config *AuthMiddlewareConfig, logger *slog.Logger) *BlacklistManager {
	ctx, cancel := context.WithCancel(context.Background())

	bm := &BlacklistManager{
		blacklistedIPs:   make(map[string]time.Time),
		blacklistedUsers: make(map[uuid.UUID]time.Time),
		whitelistedIPs:   make(map[string]bool),
		config:           config,
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
	}

	// Initialize whitelist
	for _, ip := range config.WhitelistIPs {
		bm.whitelistedIPs[ip] = true
	}

	// Start cleanup goroutine
	if config.BlacklistEnabled {
		bm.wg.Add(1)
		go bm.cleanupExpiredBlacklist()
	}

	return bm
}

// Close shuts down the blacklist manager
func (bm *BlacklistManager) Close() {
	bm.cancel()
	bm.wg.Wait()
}

// IsIPBlacklisted checks if an IP is blacklisted
func (bm *BlacklistManager) IsIPBlacklisted(ip string) bool {
	if !bm.config.BlacklistEnabled {
		return false
	}

	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Check whitelist first
	if bm.config.WhitelistEnabled && bm.whitelistedIPs[ip] {
		return false
	}

	expiresAt, exists := bm.blacklistedIPs[ip]
	if !exists {
		return false
	}

	return time.Now().Before(expiresAt)
}

// IsUserBlacklisted checks if a user is blacklisted
func (bm *BlacklistManager) IsUserBlacklisted(userID uuid.UUID) bool {
	if !bm.config.BlacklistEnabled {
		return false
	}

	bm.mu.RLock()
	defer bm.mu.RUnlock()

	expiresAt, exists := bm.blacklistedUsers[userID]
	if !exists {
		return false
	}

	return time.Now().Before(expiresAt)
}

// BlacklistIP adds an IP to the blacklist
func (bm *BlacklistManager) BlacklistIP(ip string, duration time.Duration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.blacklistedIPs[ip] = time.Now().Add(duration)
	bm.logger.Warn("IP blacklisted", "ip", ip, "duration", duration)
}

// BlacklistUser adds a user to the blacklist
func (bm *BlacklistManager) BlacklistUser(userID uuid.UUID, duration time.Duration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.blacklistedUsers[userID] = time.Now().Add(duration)
	bm.logger.Warn("User blacklisted", "user_id", userID, "duration", duration)
}

// RemoveIPFromBlacklist removes an IP from the blacklist
func (bm *BlacklistManager) RemoveIPFromBlacklist(ip string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	delete(bm.blacklistedIPs, ip)
	bm.logger.Info("IP removed from blacklist", "ip", ip)
}

// RemoveUserFromBlacklist removes a user from the blacklist
func (bm *BlacklistManager) RemoveUserFromBlacklist(userID uuid.UUID) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	delete(bm.blacklistedUsers, userID)
	bm.logger.Info("User removed from blacklist", "user_id", userID)
}

// cleanupExpiredBlacklist removes expired blacklist entries
func (bm *BlacklistManager) cleanupExpiredBlacklist() {
	defer bm.wg.Done()

	ticker := time.NewTicker(bm.config.BlacklistCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bm.ctx.Done():
			return
		case <-ticker.C:
			bm.performBlacklistCleanup()
		}
	}
}

// performBlacklistCleanup removes expired blacklist entries
func (bm *BlacklistManager) performBlacklistCleanup() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()

	// Clean up expired IP blacklists
	for ip, expiresAt := range bm.blacklistedIPs {
		if now.After(expiresAt) {
			delete(bm.blacklistedIPs, ip)
			bm.logger.Info("IP blacklist expired", "ip", ip)
		}
	}

	// Clean up expired user blacklists
	for userID, expiresAt := range bm.blacklistedUsers {
		if now.After(expiresAt) {
			delete(bm.blacklistedUsers, userID)
			bm.logger.Info("User blacklist expired", "user_id", userID)
		}
	}
}

// GetBlacklistStats returns current blacklist statistics
func (bm *BlacklistManager) GetBlacklistStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return map[string]interface{}{
		"blacklisted_ips":   len(bm.blacklistedIPs),
		"blacklisted_users": len(bm.blacklistedUsers),
		"whitelisted_ips":   len(bm.whitelistedIPs),
	}
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions map[string]*AuthSession
	mu       sync.RWMutex
	config   *AuthMiddlewareConfig
	logger   *slog.Logger
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// AuthSession represents an authentication session
type AuthSession struct {
	ID           string
	UserID       uuid.UUID
	CreatedAt    time.Time
	LastActivity time.Time
	IPAddress    string
	UserAgent    string
	IsActive     bool
	ExpiresAt    time.Time
	Data         map[string]interface{}
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *AuthMiddlewareConfig, logger *slog.Logger) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())

	sm := &SessionManager{
		sessions: make(map[string]*AuthSession),
		config:   config,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start cleanup goroutine
	sm.wg.Add(1)
	go sm.cleanupExpiredSessions()

	return sm
}

// Close shuts down the session manager
func (sm *SessionManager) Close() {
	sm.cancel()
	sm.wg.Wait()
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(userID uuid.UUID, ipAddress, userAgent string) (*AuthSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if user has exceeded max active sessions
	activeSessions := 0
	for _, session := range sm.sessions {
		if session.UserID == userID && session.IsActive && time.Now().Before(session.ExpiresAt) {
			activeSessions++
		}
	}

	if activeSessions >= sm.config.MaxActiveSessions {
		return nil, fmt.Errorf("maximum active sessions exceeded")
	}

	sessionID := uuid.New().String()
	now := time.Now()

	session := &AuthSession{
		ID:           sessionID,
		UserID:       userID,
		CreatedAt:    now,
		LastActivity: now,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		IsActive:     true,
		ExpiresAt:    now.Add(sm.config.SessionTimeout),
		Data:         make(map[string]interface{}),
	}

	sm.sessions[sessionID] = session

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*AuthSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// UpdateSessionActivity updates the last activity time for a session
func (sm *SessionManager) UpdateSessionActivity(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return
	}

	session.LastActivity = time.Now()
	session.ExpiresAt = time.Now().Add(sm.config.SessionTimeout)
}

// InvalidateSession invalidates a session
func (sm *SessionManager) InvalidateSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return
	}

	session.IsActive = false
	session.ExpiresAt = time.Now()
}

// InvalidateUserSessions invalidates all sessions for a user
func (sm *SessionManager) InvalidateUserSessions(userID uuid.UUID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, session := range sm.sessions {
		if session.UserID == userID {
			session.IsActive = false
			session.ExpiresAt = time.Now()
		}
	}
}

// cleanupExpiredSessions removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.config.SessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.performSessionCleanup()
		}
	}
}

// performSessionCleanup removes expired sessions
func (sm *SessionManager) performSessionCleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()

	for sessionID, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			delete(sm.sessions, sessionID)
		}
	}
}

// GetSessionStats returns current session statistics
func (sm *SessionManager) GetSessionStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	activeSessions := 0
	expiredSessions := 0
	now := time.Now()

	for _, session := range sm.sessions {
		if session.IsActive && now.Before(session.ExpiresAt) {
			activeSessions++
		} else {
			expiredSessions++
		}
	}

	return map[string]interface{}{
		"total_sessions":   len(sm.sessions),
		"active_sessions":  activeSessions,
		"expired_sessions": expiredSessions,
	}
}

// AuthMetricsCollector collects authentication metrics
type AuthMetricsCollector struct {
	metrics map[string]int64
	mu      sync.RWMutex
	config  *AuthMiddlewareConfig
	logger  *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewAuthMetricsCollector creates a new metrics collector
func NewAuthMetricsCollector(config *AuthMiddlewareConfig, logger *slog.Logger) *AuthMetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())

	mc := &AuthMetricsCollector{
		metrics: make(map[string]int64),
		config:  config,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start metrics collection goroutine
	if config.MetricsEnabled {
		mc.wg.Add(1)
		go mc.collectMetrics()
	}

	return mc
}

// Close shuts down the metrics collector
func (mc *AuthMetricsCollector) Close() {
	mc.cancel()
	mc.wg.Wait()
}

// IncrementCounter increments a counter metric
func (mc *AuthMetricsCollector) IncrementCounter(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metrics[name]++
}

// RecordAuth records an authentication event
func (mc *AuthMetricsCollector) RecordAuth(authType, result string) {
	mc.IncrementCounter(fmt.Sprintf("auth_%s_%s", authType, result))
}

// RecordRateLimit records a rate limit event
func (mc *AuthMetricsCollector) RecordRateLimit(limitType string) {
	mc.IncrementCounter(fmt.Sprintf("rate_limit_%s", limitType))
}

// RecordBlacklist records a blacklist event
func (mc *AuthMetricsCollector) RecordBlacklist(listType, action string) {
	mc.IncrementCounter(fmt.Sprintf("blacklist_%s_%s", listType, action))
}

// GetMetrics returns current metrics
func (mc *AuthMetricsCollector) GetMetrics() map[string]int64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := make(map[string]int64)
	for k, v := range mc.metrics {
		metrics[k] = v
	}

	return metrics
}

// collectMetrics collects and logs metrics periodically
func (mc *AuthMetricsCollector) collectMetrics() {
	defer mc.wg.Done()

	ticker := time.NewTicker(mc.config.MetricsCollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.ctx.Done():
			return
		case <-ticker.C:
			mc.logMetrics()
		}
	}
}

// logMetrics logs current metrics
func (mc *AuthMetricsCollector) logMetrics() {
	metrics := mc.GetMetrics()

	mc.logger.Info("Authentication metrics",
		"metrics_count", len(metrics),
		"metrics", metrics,
	)
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(
	authService *infrastructure.AuthenticationService,
	circuitBreaker *infrastructure.CircuitBreaker,
	logger *slog.Logger,
	config *AuthMiddlewareConfig,
) *AuthMiddleware {
	if config == nil {
		config = DefaultAuthMiddlewareConfig()
	}

	return &AuthMiddleware{
		authService:      authService,
		rateLimiter:      NewRateLimiter(config, logger),
		circuitBreaker:   circuitBreaker,
		metricsCollector: NewAuthMetricsCollector(config, logger),
		blacklist:        NewBlacklistManager(config, logger),
		sessionManager:   NewSessionManager(config, logger),
		logger:           logger,
		config:           config,
	}
}

// Close shuts down the authentication middleware
func (am *AuthMiddleware) Close() {
	am.rateLimiter.Close()
	am.blacklist.Close()
	am.sessionManager.Close()
	am.metricsCollector.Close()
}

// AuthenticationMiddleware is the main authentication middleware
func (am *AuthMiddleware) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Add security headers
		am.addSecurityHeaders(w)

		// Handle CORS if enabled
		if am.config.CORSEnabled {
			am.handleCORS(w, r)
			if r.Method == http.MethodOptions {
				return
			}
		}

		// Get client IP
		clientIP := am.getClientIP(r)

		// Check IP blacklist
		if am.blacklist.IsIPBlacklisted(clientIP) {
			am.metricsCollector.RecordBlacklist("ip", "blocked")
			am.writeErrorResponse(w, http.StatusForbidden, "IP address is blacklisted", "")
			return
		}

		// Check IP rate limit
		if am.rateLimiter.CheckIPRateLimit(clientIP) {
			am.metricsCollector.RecordRateLimit("ip")
			am.writeErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded", "")
			return
		}

		// Try to authenticate the request
		user, authType, err := am.authenticateRequest(r)
		if err != nil {
			am.metricsCollector.RecordAuth(authType, "failed")
			am.logger.Warn("Authentication failed", "error", err, "ip", clientIP, "user_agent", r.Header.Get("User-Agent"))
			am.writeErrorResponse(w, http.StatusUnauthorized, "Authentication failed", err.Error())
			return
		}

		// If user is authenticated, check user-specific limits
		if user != nil {
			// Check user blacklist
			if am.blacklist.IsUserBlacklisted(user.ID) {
				am.metricsCollector.RecordBlacklist("user", "blocked")
				am.writeErrorResponse(w, http.StatusForbidden, "User is blacklisted", "")
				return
			}

			// Check user rate limit
			if am.rateLimiter.CheckUserRateLimit(user.ID) {
				am.metricsCollector.RecordRateLimit("user")
				am.writeErrorResponse(w, http.StatusTooManyRequests, "User rate limit exceeded", "")
				return
			}

			// Add user to context
			ctx = context.WithValue(ctx, "user", user)
			ctx = context.WithValue(ctx, "user_id", user.ID)
			ctx = context.WithValue(ctx, "user_email", user.Email)

			am.metricsCollector.RecordAuth(authType, "success")
		}

		// Add authentication metadata to context
		ctx = context.WithValue(ctx, "auth_type", authType)
		ctx = context.WithValue(ctx, "client_ip", clientIP)
		ctx = context.WithValue(ctx, "request_id", uuid.New().String())

		// Continue to next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticateRequest attempts to authenticate the request using various methods
func (am *AuthMiddleware) authenticateRequest(r *http.Request) (*domain.User, string, error) {
	// Try JWT authentication first
	if user, err := am.authenticateJWT(r); err == nil && user != nil {
		return user, "jwt", nil
	}

	// Try API key authentication
	if user, err := am.authenticateAPIKey(r); err == nil && user != nil {
		return user, "apikey", nil
	}

	// Try session authentication
	if user, err := am.authenticateSession(r); err == nil && user != nil {
		return user, "session", nil
	}

	// No authentication found - this might be okay for optional auth endpoints
	return nil, "none", nil
}

// authenticateJWT authenticates using JWT token
func (am *AuthMiddleware) authenticateJWT(r *http.Request) (*domain.User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("no authorization header")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	token := parts[1]
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	// Use circuit breaker for JWT validation
	if am.config.CircuitBreakerEnabled {
		result, err := am.circuitBreaker.Execute(r.Context(), func(ctx context.Context) (interface{}, error) {
			return am.authService.ValidateToken(ctx, token)
		})
		if err != nil {
			return nil, err
		}
		return result.(*domain.User), nil
	}

	return am.authService.ValidateToken(r.Context(), token)
}

// authenticateAPIKey authenticates using API key
func (am *AuthMiddleware) authenticateAPIKey(r *http.Request) (*domain.User, error) {
	if !am.config.APIKeyEnabled {
		return nil, fmt.Errorf("API key authentication disabled")
	}

	var apiKey string

	// Try headers first
	for _, header := range am.config.APIKeyHeaders {
		if key := r.Header.Get(header); key != "" {
			apiKey = key
			break
		}
	}

	// Try query parameter if no header found
	if apiKey == "" && am.config.APIKeyQueryParam != "" {
		apiKey = r.URL.Query().Get(am.config.APIKeyQueryParam)
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key found")
	}

	// Use circuit breaker for API key validation
	if am.config.CircuitBreakerEnabled {
		result, err := am.circuitBreaker.Execute(r.Context(), func(ctx context.Context) (interface{}, error) {
			return am.authService.ValidateApiKey(ctx, apiKey)
		})
		if err != nil {
			return nil, err
		}

		apiKeyData := result.(*domain.ApiKey)
		// Get the user associated with the API key
		return am.authService.GetUser(r.Context(), apiKeyData.UserID)
	}

	apiKeyData, err := am.authService.ValidateApiKey(r.Context(), apiKey)
	if err != nil {
		return nil, err
	}

	return am.authService.GetUser(r.Context(), apiKeyData.UserID)
}

// authenticateSession authenticates using session
func (am *AuthMiddleware) authenticateSession(r *http.Request) (*domain.User, error) {
	// Try session cookie
	cookie, err := r.Cookie(am.config.SessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}

	session, exists := am.sessionManager.GetSession(cookie.Value)
	if !exists {
		return nil, fmt.Errorf("invalid session")
	}

	// Update session activity
	am.sessionManager.UpdateSessionActivity(cookie.Value)

	// Get user
	return am.authService.GetUser(r.Context(), session.UserID)
}

// RequireAuthentication middleware that requires authentication
func (am *AuthMiddleware) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value("user").(*domain.User)
		if !ok || user == nil {
			am.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", "")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequirePermission middleware that requires specific permissions
func (am *AuthMiddleware) RequirePermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value("user").(*domain.User)
			if !ok || user == nil {
				am.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", "")
				return
			}

			hasPermission, err := am.authService.CheckPermission(r.Context(), user.ID, resource, action)
			if err != nil {
				am.logger.Error("Permission check failed", "error", err, "user_id", user.ID, "resource", resource, "action", action)
				am.writeErrorResponse(w, http.StatusInternalServerError, "Permission check failed", "")
				return
			}

			if !hasPermission {
				am.logger.Warn("Permission denied", "user_id", user.ID, "resource", resource, "action", action)
				am.writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions", "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole middleware that requires specific roles
func (am *AuthMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value("user").(*domain.User)
			if !ok || user == nil {
				am.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", "")
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			userRoles := make(map[string]bool)
			for _, role := range user.Roles {
				userRoles[role.Name] = true
			}

			for _, requiredRole := range roles {
				if userRoles[requiredRole] {
					hasRole = true
					break
				}
			}

			if !hasRole {
				am.logger.Warn("Role check failed", "user_id", user.ID, "required_roles", roles, "user_roles", userRoles)
				am.writeErrorResponse(w, http.StatusForbidden, "Insufficient role permissions", "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuthentication middleware that provides optional authentication
func (am *AuthMiddleware) OptionalAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Try to authenticate, but don't fail if it doesn't work
		user, authType, _ := am.authenticateRequest(r)
		if user != nil {
			ctx = context.WithValue(ctx, "user", user)
			ctx = context.WithValue(ctx, "user_id", user.ID)
			ctx = context.WithValue(ctx, "user_email", user.Email)
			ctx = context.WithValue(ctx, "auth_type", authType)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuditMiddleware logs authentication events
func (am *AuthMiddleware) AuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !am.config.AuditEnabled {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Create a response writer that captures the status code
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(wrappedWriter, r)

		// Log audit information asynchronously
		go am.logAuditEvent(r, wrappedWriter.statusCode, time.Since(start))
	})
}

// logAuditEvent logs an audit event
func (am *AuthMiddleware) logAuditEvent(r *http.Request, statusCode int, duration time.Duration) {
	ctx := context.Background()

	var userID *uuid.UUID
	if user, ok := r.Context().Value("user").(*domain.User); ok {
		userID = &user.ID
	}

	authType, _ := r.Context().Value("auth_type").(string)
	clientIP := am.getClientIP(r)
	requestID, _ := r.Context().Value("request_id").(string)

	// Build audit data
	auditData := map[string]interface{}{
		"method":      r.Method,
		"path":        r.URL.Path,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
		"auth_type":   authType,
		"client_ip":   clientIP,
		"user_agent":  r.Header.Get("User-Agent"),
		"request_id":  requestID,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	// Add query parameters if not sensitive
	if !am.config.AuditLogSensitiveData {
		auditData["query_params"] = r.URL.Query()
	}

	// Determine resource and action
	resource, action := am.extractResourceAndAction(r.URL.Path, r.Method)

	// Log audit event
	err := am.authService.AuditAction(
		ctx,
		userID,
		action,
		resource,
		nil,       // resourceID
		nil,       // oldValues
		auditData, // newValues
		clientIP,
		r.Header.Get("User-Agent"),
	)

	if err != nil {
		am.logger.Error("Audit logging failed", "error", err)
	}
}

// Helper methods

// getClientIP extracts the client IP address from the request
func (am *AuthMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])

		// Validate that it's from a trusted proxy
		if am.isTrustedProxy(r.RemoteAddr) {
			return clientIP
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if am.isTrustedProxy(r.RemoteAddr) {
			return xri
		}
	}

	// Check CF-Connecting-IP header (Cloudflare)
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		if am.isTrustedProxy(r.RemoteAddr) {
			return cfIP
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isTrustedProxy checks if the IP is a trusted proxy
func (am *AuthMiddleware) isTrustedProxy(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	for _, trustedProxy := range am.config.TrustedProxies {
		if host == trustedProxy {
			return true
		}
	}

	return false
}

// addSecurityHeaders adds security headers to the response
func (am *AuthMiddleware) addSecurityHeaders(w http.ResponseWriter) {
	// Add configured security headers
	for header, value := range am.config.SecurityHeaders {
		w.Header().Set(header, value)
	}

	// Add Content Security Policy
	if am.config.EnableCSP {
		cspValue := am.buildCSPHeader()
		w.Header().Set("Content-Security-Policy", cspValue)
	}

	// Add HSTS header
	if am.config.EnableHSTS {
		hstsValue := am.buildHSTSHeader()
		w.Header().Set("Strict-Transport-Security", hstsValue)
	}
}

// buildCSPHeader builds the Content Security Policy header value
func (am *AuthMiddleware) buildCSPHeader() string {
	var directives []string

	// Sort directives for consistent output
	var keys []string
	for key := range am.config.CSPDirectives {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		directives = append(directives, fmt.Sprintf("%s %s", key, am.config.CSPDirectives[key]))
	}

	return strings.Join(directives, "; ")
}

// buildHSTSHeader builds the HSTS header value
func (am *AuthMiddleware) buildHSTSHeader() string {
	maxAge := int(am.config.HSTSMaxAge.Seconds())
	hstsValue := fmt.Sprintf("max-age=%d", maxAge)

	if am.config.HSTSIncludeSubdomains {
		hstsValue += "; includeSubDomains"
	}

	if am.config.HSTSPreload {
		hstsValue += "; preload"
	}

	return hstsValue
}

// handleCORS handles CORS preflight and actual requests
func (am *AuthMiddleware) handleCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// Check if origin is allowed
	if !am.isOriginAllowed(origin) {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)

	if am.config.CORSAllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Handle preflight request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(am.config.CORSAllowedMethods, ", "))
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(am.config.CORSAllowedHeaders, ", "))
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(int(am.config.CORSMaxAge.Seconds())))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Handle actual request
	if len(am.config.CORSExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(am.config.CORSExposedHeaders, ", "))
	}
}

// isOriginAllowed checks if the origin is allowed
func (am *AuthMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowedOrigin := range am.config.CORSAllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}

	return false
}

// extractResourceAndAction extracts resource and action from URL path and method
func (am *AuthMiddleware) extractResourceAndAction(path, method string) (string, string) {
	// Remove leading/trailing slashes and split
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 3 {
		return "unknown", strings.ToLower(method)
	}

	// Skip "api" and "v1" parts
	resource := parts[2]
	if len(parts) > 3 && parts[3] != "" {
		resource = parts[3]
	}

	// Map HTTP methods to actions
	actionMap := map[string]string{
		"GET":    "read",
		"POST":   "create",
		"PUT":    "update",
		"PATCH":  "update",
		"DELETE": "delete",
	}

	action := actionMap[strings.ToUpper(method)]
	if action == "" {
		action = strings.ToLower(method)
	}

	return resource, action
}

// writeErrorResponse writes an error response
func (am *AuthMiddleware) writeErrorResponse(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error": message,
		"code":  statusCode,
		"time":  time.Now().UTC().Format(time.RFC3339),
	}

	if details != "" {
		response["details"] = details
	}

	requestID, _ := w.Header()["request_id"]
	if len(requestID) > 0 {
		response["request_id"] = requestID[0]
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log encoding error but don't return since we've already set status
		logger := am.logger
		if logger != nil {
			logger.Error("Failed to encode JSON response", "error", err)
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// AuthMiddlewareChain provides convenient middleware chaining
type AuthMiddlewareChain struct {
	middleware *AuthMiddleware
}

// NewAuthMiddlewareChain creates a new middleware chain
func NewAuthMiddlewareChain(middleware *AuthMiddleware) *AuthMiddlewareChain {
	return &AuthMiddlewareChain{middleware: middleware}
}

// ForPublicEndpoints applies middleware for public endpoints
func (c *AuthMiddlewareChain) ForPublicEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.OptionalAuthentication(
				c.middleware.AuthenticationMiddleware(next),
			),
		)
	}
}

// ForProtectedEndpoints applies middleware for protected endpoints
func (c *AuthMiddlewareChain) ForProtectedEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.RequireAuthentication(
				c.middleware.AuthenticationMiddleware(next),
			),
		)
	}
}

// ForAdminEndpoints applies middleware for admin endpoints
func (c *AuthMiddlewareChain) ForAdminEndpoints() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return c.middleware.AuditMiddleware(
			c.middleware.RequireRole("admin")(
				c.middleware.RequireAuthentication(
					c.middleware.AuthenticationMiddleware(next),
				),
			),
		)
	}
}

// WithPermission adds permission requirement to middleware chain
func (c *AuthMiddlewareChain) WithPermission(resource, action string) func(http.Handler) http.Handler {
	return c.middleware.RequirePermission(resource, action)
}

// WithRole adds role requirement to middleware chain
func (c *AuthMiddlewareChain) WithRole(roles ...string) func(http.Handler) http.Handler {
	return c.middleware.RequireRole(roles...)
}

// SetupRoutes demonstrates how to setup routes with different authentication requirements
func (c *AuthMiddlewareChain) SetupRoutes(router *mux.Router) {
	// Public routes (no authentication required)
	publicRouter := router.PathPrefix("/api/v1/public").Subrouter()
	publicRouter.Use(c.ForPublicEndpoints())

	// Protected routes (requires authentication)
	protectedRouter := router.PathPrefix("/api/v1/protected").Subrouter()
	protectedRouter.Use(c.ForProtectedEndpoints())

	// Admin routes (requires admin role)
	adminRouter := router.PathPrefix("/api/v1/admin").Subrouter()
	adminRouter.Use(c.ForAdminEndpoints())

	// Routes with specific permissions
	reviewRouter := router.PathPrefix("/api/v1/reviews").Subrouter()
	reviewRouter.Use(c.ForProtectedEndpoints())

	// Example: Read reviews - requires read permission
	reviewRouter.Handle("", c.WithPermission("reviews", "read")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handler implementation
		}),
	)).Methods("GET")

	// Example: Create review - requires create permission
	reviewRouter.Handle("", c.WithPermission("reviews", "create")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handler implementation
		}),
	)).Methods("POST")

	// Example: Update review - requires update permission
	reviewRouter.Handle("/{id}", c.WithPermission("reviews", "update")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handler implementation
		}),
	)).Methods("PUT")

	// Example: Delete review - requires delete permission
	reviewRouter.Handle("/{id}", c.WithPermission("reviews", "delete")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handler implementation
		}),
	)).Methods("DELETE")
}

// GetAuthMiddlewareStatus returns status information about the middleware
func (am *AuthMiddleware) GetAuthMiddlewareStatus() map[string]interface{} {
	return map[string]interface{}{
		"rate_limiter":    am.rateLimiter.GetRateLimitStats(),
		"blacklist":       am.blacklist.GetBlacklistStats(),
		"session_manager": am.sessionManager.GetSessionStats(),
		"metrics":         am.metricsCollector.GetMetrics(),
		"circuit_breaker": am.circuitBreaker.GetMetrics(),
		"config":          am.config,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}
}

// CreateSecureHash creates a secure hash for API keys or other sensitive data
func (am *AuthMiddleware) CreateSecureHash(data string) string {
	hasher := sha256.New()
	hasher.Write([]byte(data))
	return hex.EncodeToString(hasher.Sum(nil))
}

// VerifySecureHash verifies a secure hash
func (am *AuthMiddleware) VerifySecureHash(data, hash string) bool {
	computedHash := am.CreateSecureHash(data)
	return computedHash == hash
}

// RefreshUserSession refreshes a user session
func (am *AuthMiddleware) RefreshUserSession(sessionID string) error {
	session, exists := am.sessionManager.GetSession(sessionID)
	if !exists {
		return fmt.Errorf("session not found")
	}

	session.ExpiresAt = time.Now().Add(am.config.SessionTimeout)
	return nil
}

// GetUserFromContext extracts user from request context
func (am *AuthMiddleware) GetUserFromContext(r *http.Request) (*domain.User, bool) {
	user, ok := r.Context().Value("user").(*domain.User)
	return user, ok
}

// GetAuthTypeFromContext extracts auth type from request context
func (am *AuthMiddleware) GetAuthTypeFromContext(r *http.Request) (string, bool) {
	authType, ok := r.Context().Value("auth_type").(string)
	return authType, ok
}

// GetClientIPFromContext extracts client IP from request context
func (am *AuthMiddleware) GetClientIPFromContext(r *http.Request) (string, bool) {
	clientIP, ok := r.Context().Value("client_ip").(string)
	return clientIP, ok
}

// GetRequestIDFromContext extracts request ID from request context
func (am *AuthMiddleware) GetRequestIDFromContext(r *http.Request) (string, bool) {
	requestID, ok := r.Context().Value("request_id").(string)
	return requestID, ok
}
