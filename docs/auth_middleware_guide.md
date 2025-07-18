# Authentication Middleware Guide

## Overview

The Authentication Middleware provides comprehensive authentication and authorization capabilities for HTTP handlers in the hotel reviews microservice. It supports multiple authentication methods, rate limiting, security features, and audit logging.

## Features

### 1. JWT Token Validation
- Validates JWT tokens from `Authorization: Bearer <token>` header
- Supports configurable token expiry and refresh tokens
- Integrates with circuit breaker for resilience
- Extracts user information from token claims

### 2. API Key Authentication
- Supports API key authentication for service-to-service calls
- Configurable API key headers (`X-API-Key`, `Authorization`)
- Query parameter support for API keys
- Scope-based access control for API keys

### 3. Role-Based Access Control (RBAC)
- Hierarchical role system with permissions
- Resource and action-based permission checking
- Role inheritance and permission aggregation
- Dynamic role assignment and removal

### 4. Rate Limiting
- Per-user rate limiting with configurable limits
- Per-IP rate limiting for DDoS protection
- Burst allowance for legitimate traffic spikes
- Automatic cleanup of expired rate limit entries

### 5. Security Features
- IP blacklisting and whitelisting
- Session management with timeout
- Circuit breaker integration for auth services
- Security headers (CSP, HSTS, etc.)
- CORS handling with configurable policies

### 6. Audit Logging
- Comprehensive audit trail for all authentication events
- Configurable sensitive data logging
- Request correlation with unique request IDs
- Failed authentication attempt tracking

### 7. Metrics Collection
- Real-time authentication metrics
- Rate limiting statistics
- Session management metrics
- Circuit breaker metrics

## Configuration

### Basic Configuration

```go
config := &application.AuthMiddlewareConfig{
    // JWT Configuration
    JWTSecret:        "your-super-secret-key",
    JWTExpiry:        15 * time.Minute,
    JWTRefreshExpiry: 7 * 24 * time.Hour,
    JWTIssuer:        "hotel-reviews-service",
    
    // Rate Limiting
    RateLimitEnabled:  true,
    RateLimitRequests: 100,
    RateLimitWindow:   time.Minute,
    RateLimitBurst:    10,
    
    // Security
    BlacklistEnabled: true,
    WhitelistEnabled: false,
    TrustedProxies:   []string{"127.0.0.1"},
    
    // API Keys
    APIKeyEnabled:    true,
    APIKeyHeaders:    []string{"X-API-Key"},
    APIKeyQueryParam: "api_key",
    
    // Audit & Metrics
    AuditEnabled:   true,
    MetricsEnabled: true,
}
```

### Advanced Configuration

```go
config := application.DefaultAuthMiddlewareConfig()

// Session Configuration
config.SessionTimeout = 30 * time.Minute
config.MaxActiveSessions = 5
config.SessionCookieName = "app_session"
config.SessionSecure = true
config.SessionHttpOnly = true

// CORS Configuration
config.CORSEnabled = true
config.CORSAllowedOrigins = []string{"https://yourapp.com"}
config.CORSAllowedMethods = []string{"GET", "POST", "PUT", "DELETE"}
config.CORSAllowCredentials = true

// Security Headers
config.SecurityHeaders = map[string]string{
    "X-Frame-Options":        "DENY",
    "X-Content-Type-Options": "nosniff",
    "X-XSS-Protection":       "1; mode=block",
}

// Content Security Policy
config.EnableCSP = true
config.CSPDirectives = map[string]string{
    "default-src": "'self'",
    "script-src":  "'self' 'unsafe-inline'",
    "style-src":   "'self' 'unsafe-inline'",
}
```

## Usage Examples

### Basic Setup

```go
// Create middleware
authMiddleware := application.NewAuthMiddleware(
    authService,
    circuitBreaker,
    logger,
    config,
)
defer authMiddleware.Close()

// Create middleware chain
chain := application.NewAuthMiddlewareChain(authMiddleware)

// Setup router
router := mux.NewRouter()
```

### Public Endpoints (Optional Authentication)

```go
// Public API - authentication is optional
publicAPI := router.PathPrefix("/api/v1/public").Subrouter()
publicAPI.Use(chain.ForPublicEndpoints())

publicAPI.HandleFunc("/reviews", getPublicReviewsHandler).Methods("GET")
publicAPI.HandleFunc("/hotels", getPublicHotelsHandler).Methods("GET")
```

### Protected Endpoints (Authentication Required)

```go
// Protected API - authentication required
protectedAPI := router.PathPrefix("/api/v1/protected").Subrouter()
protectedAPI.Use(chain.ForProtectedEndpoints())

protectedAPI.HandleFunc("/profile", getUserProfileHandler).Methods("GET")
protectedAPI.HandleFunc("/api-keys", createAPIKeyHandler).Methods("POST")
```

### Permission-Based Access Control

```go
// Reviews API with specific permissions
reviewsAPI := router.PathPrefix("/api/v1/reviews").Subrouter()
reviewsAPI.Use(chain.ForProtectedEndpoints())

// Read reviews - requires 'reviews:read' permission
reviewsAPI.Handle("", chain.WithPermission("reviews", "read")(
    http.HandlerFunc(getReviewsHandler),
)).Methods("GET")

// Create review - requires 'reviews:create' permission
reviewsAPI.Handle("", chain.WithPermission("reviews", "create")(
    http.HandlerFunc(createReviewHandler),
)).Methods("POST")

// Update review - requires 'reviews:update' permission
reviewsAPI.Handle("/{id}", chain.WithPermission("reviews", "update")(
    http.HandlerFunc(updateReviewHandler),
)).Methods("PUT")

// Delete review - requires 'reviews:delete' permission
reviewsAPI.Handle("/{id}", chain.WithPermission("reviews", "delete")(
    http.HandlerFunc(deleteReviewHandler),
)).Methods("DELETE")
```

### Role-Based Access Control

```go
// Admin endpoints - requires admin role
adminAPI := router.PathPrefix("/api/v1/admin").Subrouter()
adminAPI.Use(chain.ForAdminEndpoints())

adminAPI.HandleFunc("/users", listUsersHandler).Methods("GET")
adminAPI.HandleFunc("/metrics", getMetricsHandler).Methods("GET")

// Moderator endpoints - requires moderator or admin role
moderatorAPI := router.PathPrefix("/api/v1/moderator").Subrouter()
moderatorAPI.Use(chain.ForProtectedEndpoints())
moderatorAPI.Use(chain.WithRole("moderator", "admin"))

moderatorAPI.HandleFunc("/reviews/pending", getPendingReviewsHandler).Methods("GET")
```

### Custom Middleware Combinations

```go
// Custom middleware for specific requirements
customHandler := chain.ForProtectedEndpoints()(
    chain.WithPermission("reviews", "create")(
        chain.WithRole("verified_user")(
            http.HandlerFunc(createVerifiedReviewHandler),
        ),
    ),
)

router.Handle("/api/v1/reviews/verified", customHandler).Methods("POST")
```

## Authentication Methods

### 1. JWT Token Authentication

```bash
# Using Bearer token
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
     https://api.example.com/api/v1/protected/profile
```

### 2. API Key Authentication

```bash
# Using header
curl -H "X-API-Key: your-api-key-here" \
     https://api.example.com/api/v1/service/data

# Using query parameter
curl "https://api.example.com/api/v1/service/data?api_key=your-api-key-here"
```

### 3. Session Authentication

```bash
# Using session cookie
curl -b "session_id=abc123def456" \
     https://api.example.com/api/v1/protected/profile
```

## Handler Implementation

### Accessing User Information

```go
func getUserProfileHandler(w http.ResponseWriter, r *http.Request) {
    // Get authenticated user from context
    user, ok := r.Context().Value("user").(*domain.User)
    if !ok {
        http.Error(w, "User not found", http.StatusUnauthorized)
        return
    }
    
    // Get additional authentication info
    authType, _ := r.Context().Value("auth_type").(string)
    clientIP, _ := r.Context().Value("client_ip").(string)
    requestID, _ := r.Context().Value("request_id").(string)
    
    // Use user information
    response := map[string]interface{}{
        "user_id":    user.ID,
        "username":   user.Username,
        "email":      user.Email,
        "auth_type":  authType,
        "client_ip":  clientIP,
        "request_id": requestID,
    }
    
    json.NewEncoder(w).Encode(response)
}
```

### Permission Checking

```go
func updateReviewHandler(w http.ResponseWriter, r *http.Request) {
    user, ok := r.Context().Value("user").(*domain.User)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Additional permission check if needed
    hasPermission, err := authService.CheckPermission(
        r.Context(),
        user.ID,
        "reviews",
        "update",
    )
    if err != nil || !hasPermission {
        http.Error(w, "Insufficient permissions", http.StatusForbidden)
        return
    }
    
    // Handle review update
    // ...
}
```

## Rate Limiting

### Configuration

```go
config := &application.AuthMiddlewareConfig{
    RateLimitEnabled:  true,
    RateLimitRequests: 100,        // 100 requests
    RateLimitWindow:   time.Minute, // per minute
    RateLimitBurst:    10,          // with 10 request burst
}
```

### Monitoring Rate Limits

```go
// Get rate limit statistics
rateLimitStats := authMiddleware.GetRateLimitStats()
fmt.Printf("Blocked users: %d\n", rateLimitStats["blocked_users"])
fmt.Printf("Blocked IPs: %d\n", rateLimitStats["blocked_ips"])
```

## Security Features

### IP Blacklisting

```go
// Blacklist an IP address
authMiddleware.BlacklistIP("192.168.1.100", 24*time.Hour)

// Blacklist a user
authMiddleware.BlacklistUser(userID, 1*time.Hour)

// Check if IP is blacklisted
if authMiddleware.IsIPBlacklisted("192.168.1.100") {
    // Handle blacklisted IP
}
```

### Security Headers

The middleware automatically adds security headers:

```
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'
```

### CORS Configuration

```go
config.CORSEnabled = true
config.CORSAllowedOrigins = []string{"https://yourapp.com"}
config.CORSAllowedMethods = []string{"GET", "POST", "PUT", "DELETE"}
config.CORSAllowedHeaders = []string{"Content-Type", "Authorization"}
config.CORSAllowCredentials = true
```

## Session Management

### Session Creation

```go
session, err := authMiddleware.CreateSession(
    userID,
    clientIP,
    userAgent,
)
if err != nil {
    // Handle error
}
```

### Session Validation

```go
session, exists := authMiddleware.GetSession(sessionID)
if !exists {
    // Session not found or expired
}
```

### Session Cleanup

```go
// Invalidate specific session
authMiddleware.InvalidateSession(sessionID)

// Invalidate all user sessions
authMiddleware.InvalidateUserSessions(userID)
```

## Audit Logging

### Audit Events

The middleware automatically logs:
- Authentication attempts (success/failure)
- Permission checks
- Rate limit violations
- Security events (blacklisting, etc.)

### Audit Data Structure

```json
{
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "action": "login",
  "resource": "user",
  "timestamp": "2023-10-01T10:00:00Z",
  "client_ip": "192.168.1.1",
  "user_agent": "Mozilla/5.0...",
  "request_id": "req-123456",
  "result": "success",
  "method": "POST",
  "path": "/api/v1/auth/login",
  "status_code": 200,
  "duration_ms": 150
}
```

## Metrics Collection

### Available Metrics

```go
metrics := authMiddleware.GetMetrics()

// Authentication metrics
fmt.Printf("JWT success: %d\n", metrics["auth_jwt_success"])
fmt.Printf("JWT failed: %d\n", metrics["auth_jwt_failed"])
fmt.Printf("API key success: %d\n", metrics["auth_apikey_success"])

// Rate limiting metrics
fmt.Printf("User rate limits: %d\n", metrics["rate_limit_user"])
fmt.Printf("IP rate limits: %d\n", metrics["rate_limit_ip"])

// Blacklist metrics
fmt.Printf("IP blacklisted: %d\n", metrics["blacklist_ip_blocked"])
fmt.Printf("User blacklisted: %d\n", metrics["blacklist_user_blocked"])
```

### Metrics Monitoring

```go
// Get comprehensive status
status := authMiddleware.GetAuthMiddlewareStatus()
fmt.Printf("Status: %+v\n", status)
```

## Circuit Breaker Integration

### Configuration

```go
config.CircuitBreakerEnabled = true
config.CircuitBreakerThreshold = 5
config.CircuitBreakerTimeout = 30 * time.Second
config.CircuitBreakerReset = 60 * time.Second
```

### Monitoring

```go
cbMetrics := authMiddleware.GetCircuitBreakerMetrics()
fmt.Printf("Circuit breaker state: %s\n", cbMetrics.CurrentState)
fmt.Printf("Total requests: %d\n", cbMetrics.TotalRequests)
fmt.Printf("Success rate: %.2f%%\n", cbMetrics.SuccessRate)
```

## Error Handling

### Error Response Format

```json
{
  "error": "Authentication failed",
  "code": 401,
  "time": "2023-10-01T10:00:00Z",
  "details": "Invalid token",
  "request_id": "req-123456"
}
```

### Common Error Codes

- `401 Unauthorized`: Authentication required or failed
- `403 Forbidden`: Insufficient permissions or blacklisted
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: System error

## Testing

### Unit Tests

```go
func TestAuthMiddleware(t *testing.T) {
    // Create test middleware
    middleware := createTestAuthMiddleware()
    defer middleware.Close()
    
    // Test JWT authentication
    user := createTestUser()
    mockAuthService.On("ValidateToken", mock.Anything, "valid-token").Return(user, nil)
    
    // Test request
    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer valid-token")
    
    rr := httptest.NewRecorder()
    handler := middleware.AuthenticationMiddleware(testHandler)
    handler.ServeHTTP(rr, req)
    
    assert.Equal(t, http.StatusOK, rr.Code)
}
```

### Integration Tests

```go
func TestAuthMiddlewareIntegration(t *testing.T) {
    // Setup test server
    server := httptest.NewServer(setupTestRoutes())
    defer server.Close()
    
    // Test authentication flow
    token := loginAndGetToken(server.URL)
    
    // Test protected endpoint
    req, _ := http.NewRequest("GET", server.URL+"/api/v1/protected/profile", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    
    resp, err := http.DefaultClient.Do(req)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Performance Considerations

### Caching

- JWT tokens are validated on each request
- Consider caching valid tokens for short periods
- Rate limit data is stored in memory with periodic cleanup

### Database Queries

- User and permission lookups are made for each authenticated request
- Consider implementing caching for user permissions
- Use database connection pooling

### Memory Usage

- Rate limit data is stored in memory
- Session data is stored in memory
- Implement cleanup routines for expired data

## Best Practices

### Security

1. **Use HTTPS Only**: Always use HTTPS in production
2. **Secure JWT Secrets**: Use strong, randomly generated JWT secrets
3. **Token Expiry**: Use short-lived access tokens with refresh tokens
4. **Rate Limiting**: Implement appropriate rate limits for your use case
5. **Input Validation**: Always validate input data
6. **Audit Logging**: Enable comprehensive audit logging

### Performance

1. **Connection Pooling**: Use connection pooling for database operations
2. **Caching**: Implement caching for frequently accessed data
3. **Async Operations**: Use async operations for audit logging
4. **Cleanup**: Implement regular cleanup of expired data

### Monitoring

1. **Metrics**: Monitor authentication metrics
2. **Alerting**: Set up alerts for security events
3. **Logging**: Use structured logging for better analysis
4. **Health Checks**: Implement health checks for auth services

## Troubleshooting

### Common Issues

1. **Invalid Token**: Check JWT secret and token format
2. **Permission Denied**: Verify user roles and permissions
3. **Rate Limited**: Check rate limit configuration
4. **Session Expired**: Verify session timeout configuration

### Debug Logging

```go
// Enable debug logging
config.AuditEnabled = true
config.AuditLogSensitiveData = true // Only in development
config.MetricsEnabled = true
```

### Health Checks

```go
// Check middleware health
status := authMiddleware.GetAuthMiddlewareStatus()
if status["circuit_breaker"].(map[string]interface{})["state"] == "open" {
    // Circuit breaker is open
}
```

## Migration Guide

### From Basic Auth

1. Replace basic auth with JWT tokens
2. Implement user registration and login endpoints
3. Add role and permission management
4. Configure rate limiting and security features

### From Session-Only Auth

1. Add JWT token support alongside sessions
2. Implement API key authentication for services
3. Add permission-based access control
4. Configure audit logging and metrics

## Contributing

When contributing to the authentication middleware:

1. Follow the existing code style
2. Add comprehensive tests
3. Update documentation
4. Consider security implications
5. Add metrics for new features

## License

This authentication middleware is part of the hotel reviews microservice and follows the same license terms.