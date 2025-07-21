package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// SecurityMiddleware provides comprehensive security middleware
type SecurityMiddleware struct {
	rateLimiter *infrastructure.RateLimiter
	config      *config.SecurityConfig
	logger      *logger.Logger
	redisClient *redis.Client
}

// NewSecurityMiddleware creates a new security middleware instance
func NewSecurityMiddleware(
	config *config.SecurityConfig,
	logger *logger.Logger,
	redisClient *redis.Client,
) *SecurityMiddleware {
	rateLimiter := infrastructure.NewRateLimiter(
		config.RateLimit,
		config.RateLimitWindow,
		redisClient,
	)

	return &SecurityMiddleware{
		rateLimiter: rateLimiter,
		config:      config,
		logger:      logger,
		redisClient: redisClient,
	}
}

// RateLimitMiddleware implements rate limiting
func (s *SecurityMiddleware) RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client identifier (IP address or user ID if authenticated)
		clientID := s.getClientIdentifier(c)
		
		// Check rate limit
		allowed, err := s.rateLimiter.Allow(c.Request.Context(), clientID)
		if err != nil {
			s.logger.ErrorContext(c.Request.Context(), "Rate limiter error",
				"client_id", clientID,
				"error", err,
			)
			// On error, allow the request but log it
			c.Next()
			return
		}

		if !allowed {
			s.logger.WarnContext(c.Request.Context(), "Rate limit exceeded",
				"client_id", clientID,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
			)
			
			// Add rate limit headers
			s.addRateLimitHeaders(c, clientID)
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "RATE_LIMIT_EXCEEDED",
				"message": "Too many requests. Please try again later.",
				"retry_after": s.config.RateLimitWindow.Seconds(),
			})
			c.Abort()
			return
		}

		// Add rate limit headers for successful requests
		s.addRateLimitHeaders(c, clientID)
		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func (s *SecurityMiddleware) SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")
		
		// XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")
		
		// Prevent information disclosure
		c.Header("X-Powered-By", "")
		c.Header("Server", "")
		
		// HSTS for HTTPS
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		
		// Content Security Policy
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		
		// Referrer Policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing
func (s *SecurityMiddleware) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Define allowed origins (in production, this should be configurable)
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"https://yourdomain.com",
		}
		
		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}
		
		if allowed || origin == "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// InputValidationMiddleware validates and sanitizes input
func (s *SecurityMiddleware) InputValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check request size
		if c.Request.ContentLength > 10*1024*1024 { // 10MB limit
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "REQUEST_TOO_LARGE",
				"message": "Request body too large",
			})
			c.Abort()
			return
		}
		
		// Validate Content-Type for POST/PUT/PATCH requests
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			if contentType != "" {
				// Allow specific content types
				allowedTypes := []string{
					"application/json",
					"application/x-www-form-urlencoded",
					"multipart/form-data",
				}
				
				valid := false
				for _, allowedType := range allowedTypes {
					if strings.Contains(contentType, allowedType) {
						valid = true
						break
					}
				}
				
				if !valid {
					c.JSON(http.StatusUnsupportedMediaType, gin.H{
						"error":   "UNSUPPORTED_MEDIA_TYPE",
						"message": "Unsupported content type",
					})
					c.Abort()
					return
				}
			}
		}
		
		// Validate query parameters
		if err := s.validateQueryParams(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "INVALID_QUERY_PARAMS",
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// IPWhitelistMiddleware restricts access based on IP addresses
func (s *SecurityMiddleware) IPWhitelistMiddleware(allowedIPs []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(allowedIPs) == 0 {
			c.Next()
			return
		}
		
		clientIP := c.ClientIP()
		
		allowed := false
		for _, ip := range allowedIPs {
			if clientIP == ip {
				allowed = true
				break
			}
		}
		
		if !allowed {
			s.logger.WarnContext(c.Request.Context(), "IP not whitelisted",
				"client_ip", clientIP,
				"path", c.Request.URL.Path,
			)
			
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "IP_NOT_ALLOWED",
				"message": "Access denied from this IP address",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestSizeLimitMiddleware limits request body size for specific routes
func (s *SecurityMiddleware) RequestSizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "REQUEST_TOO_LARGE",
				"message": fmt.Sprintf("Request body exceeds %d bytes", maxSize),
			})
			c.Abort()
			return
		}
		
		// Set a limit on the request body reader
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		
		c.Next()
	}
}

// DDoSProtectionMiddleware provides basic DDoS protection
func (s *SecurityMiddleware) DDoSProtectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		
		// Check for suspicious patterns
		if s.isSuspiciousRequest(c) {
			s.logger.WarnContext(c.Request.Context(), "Suspicious request detected",
				"client_ip", clientIP,
				"user_agent", c.GetHeader("User-Agent"),
				"path", c.Request.URL.Path,
			)
			
			// Add delay for suspicious requests
			time.Sleep(time.Second * 2)
		}
		
		c.Next()
	}
}

// Helper methods

func (s *SecurityMiddleware) getClientIdentifier(c *gin.Context) string {
	// Try to get user ID from context (if authenticated)
	if userID, exists := c.Get("user_id"); exists {
		return fmt.Sprintf("user:%v", userID)
	}
	
	// Fall back to IP address
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

func (s *SecurityMiddleware) addRateLimitHeaders(c *gin.Context, clientID string) {
	// Add standard rate limit headers
	c.Header("X-RateLimit-Limit", strconv.Itoa(s.config.RateLimit))
	c.Header("X-RateLimit-Window", s.config.RateLimitWindow.String())
	
	// Try to get remaining requests (this would require extending the rate limiter)
	// For now, just add the basic headers
	if s.redisClient != nil {
		key := fmt.Sprintf("rate_limit:%s", clientID)
		count, err := s.redisClient.Get(c.Request.Context(), key).Int()
		if err == nil {
			remaining := s.config.RateLimit - count
			if remaining < 0 {
				remaining = 0
			}
			c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		}
	}
}

func (s *SecurityMiddleware) validateQueryParams(c *gin.Context) error {
	// Validate common query parameters
	if limit := c.Query("limit"); limit != "" {
		if val, err := strconv.Atoi(limit); err != nil {
			return fmt.Errorf("invalid limit parameter")
		} else if val < 0 || val > 1000 {
			return fmt.Errorf("limit must be between 0 and 1000")
		}
	}
	
	if offset := c.Query("offset"); offset != "" {
		if val, err := strconv.Atoi(offset); err != nil {
			return fmt.Errorf("invalid offset parameter")
		} else if val < 0 {
			return fmt.Errorf("offset must be non-negative")
		}
	}
	
	// Check for potential SQL injection patterns in query parameters
	for key, values := range c.Request.URL.Query() {
		for _, value := range values {
			if s.containsSQLInjectionPattern(value) {
				s.logger.WarnContext(c.Request.Context(), "Potential SQL injection attempt",
					"param", key,
					"value", value,
					"client_ip", c.ClientIP(),
				)
				return fmt.Errorf("invalid characters in parameter %s", key)
			}
		}
	}
	
	return nil
}

func (s *SecurityMiddleware) containsSQLInjectionPattern(value string) bool {
	// Basic SQL injection pattern detection
	patterns := []string{
		"'", "--", "/*", "*/", "xp_", "sp_", "union", "select", "insert", "delete", "update", "drop", "create", "alter",
	}
	
	lowerValue := strings.ToLower(value)
	for _, pattern := range patterns {
		if strings.Contains(lowerValue, pattern) {
			return true
		}
	}
	
	return false
}

func (s *SecurityMiddleware) isSuspiciousRequest(c *gin.Context) bool {
	userAgent := c.GetHeader("User-Agent")
	
	// Check for common attack patterns
	suspiciousPatterns := []string{
		"sqlmap", "nikto", "nmap", "masscan", "zap", "burp", "scanner",
		"bot", "crawler", "wget", "curl", "python-requests",
	}
	
	lowerUA := strings.ToLower(userAgent)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerUA, pattern) {
			return true
		}
	}
	
	// Check for missing User-Agent (potential bot)
	if userAgent == "" {
		return true
	}
	
	// Check for rapid requests from same IP (basic check)
	// This could be enhanced with Redis-based tracking
	
	return false
}