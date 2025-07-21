# Security Implementation Guide

## Overview

This guide covers comprehensive security implementation in the Hotel Reviews Microservice, including authentication, authorization, data protection, and security best practices.

## Security Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   WAF/CDN       │    │   API Gateway   │    │   Auth Service  │    │   Application   │
│   (CloudFlare)  │    │   (Kong/Nginx)  │    │   (JWT + OAuth) │    │   Services      │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ DDoS Protection │───▶│ Rate Limiting   │───▶│ Authentication  │───▶│ Authorization   │
│ Bot Detection   │    │ SSL Termination │    │ Token Validation│    │ Input Validation│
│ Geo Blocking    │    │ Request Logging │    │ Session Mgmt    │    │ Data Encryption │
└─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Audit Log     │    │   Secrets Mgmt  │
                       │   (ELK Stack)   │    │   (Vault/KMS)   │
                       │                 │    │                 │
                       │ Security Events │    │ API Keys        │
                       │ Access Logs     │    │ Certificates    │
                       │ Threat Detection│    │ Database Creds  │
                       └─────────────────┘    └─────────────────┘
```

## Authentication and Authorization

### 1. JWT Authentication

```go
type AuthService struct {
    jwtSecret     []byte
    tokenExpiry   time.Duration
    refreshExpiry time.Duration
    userService   UserService
    sessionStore  SessionStore
}

type JWTClaims struct {
    UserID   uuid.UUID `json:"user_id"`
    Email    string    `json:"email"`
    Roles    []string  `json:"roles"`
    IssuedAt int64     `json:"iat"`
    ExpiresAt int64    `json:"exp"`
    jwt.RegisteredClaims
}

func (auth *AuthService) GenerateToken(user *User) (*TokenResponse, error) {
    now := time.Now()
    
    // Access token (short-lived)
    accessClaims := &JWTClaims{
        UserID:   user.ID,
        Email:    user.Email,
        Roles:    user.Roles,
        IssuedAt: now.Unix(),
        ExpiresAt: now.Add(auth.tokenExpiry).Unix(),
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.ID.String(),
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(auth.tokenExpiry)),
            Issuer:    "hotel-reviews-service",
        },
    }
    
    accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    accessTokenString, err := accessToken.SignedString(auth.jwtSecret)
    if err != nil {
        return nil, fmt.Errorf("failed to sign access token: %w", err)
    }
    
    // Refresh token (long-lived)
    refreshClaims := &JWTClaims{
        UserID:   user.ID,
        Email:    user.Email,
        IssuedAt: now.Unix(),
        ExpiresAt: now.Add(auth.refreshExpiry).Unix(),
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.ID.String(),
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(auth.refreshExpiry)),
            Issuer:    "hotel-reviews-service",
        },
    }
    
    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
    refreshTokenString, err := refreshToken.SignedString(auth.jwtSecret)
    if err != nil {
        return nil, fmt.Errorf("failed to sign refresh token: %w", err)
    }
    
    // Store session
    session := &Session{
        ID:           uuid.New(),
        UserID:       user.ID,
        RefreshToken: refreshTokenString,
        IssuedAt:     now,
        ExpiresAt:    now.Add(auth.refreshExpiry),
        IPAddress:    getClientIP(user.Context),
        UserAgent:    getUserAgent(user.Context),
    }
    
    if err := auth.sessionStore.Create(session); err != nil {
        return nil, fmt.Errorf("failed to store session: %w", err)
    }
    
    return &TokenResponse{
        AccessToken:  accessTokenString,
        RefreshToken: refreshTokenString,
        ExpiresIn:    int64(auth.tokenExpiry.Seconds()),
        TokenType:    "Bearer",
    }, nil
}

func (auth *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return auth.jwtSecret, nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("token parsing failed: %w", err)
    }
    
    if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
        // Additional validation
        if time.Now().Unix() > claims.ExpiresAt {
            return nil, fmt.Errorf("token expired")
        }
        
        return claims, nil
    }
    
    return nil, fmt.Errorf("invalid token")
}
```

### 2. Role-Based Access Control (RBAC)

```go
type Permission struct {
    ID          uuid.UUID `json:"id"`
    Name        string    `json:"name"`
    Resource    string    `json:"resource"`
    Action      string    `json:"action"`
    Description string    `json:"description"`
}

type Role struct {
    ID          uuid.UUID    `json:"id"`
    Name        string       `json:"name"`
    Permissions []Permission `json:"permissions"`
    CreatedAt   time.Time    `json:"created_at"`
}

type AuthorizationService struct {
    permissionCache map[string][]Permission
    roleCache       map[string]*Role
    cacheTTL        time.Duration
    mutex           sync.RWMutex
}

func (authz *AuthorizationService) HasPermission(userRoles []string, resource, action string) bool {
    authz.mutex.RLock()
    defer authz.mutex.RUnlock()
    
    for _, roleName := range userRoles {
        role, exists := authz.roleCache[roleName]
        if !exists {
            continue
        }
        
        for _, permission := range role.Permissions {
            if authz.matchesPermission(permission, resource, action) {
                return true
            }
        }
    }
    
    return false
}

func (authz *AuthorizationService) matchesPermission(permission Permission, resource, action string) bool {
    // Exact match
    if permission.Resource == resource && permission.Action == action {
        return true
    }
    
    // Wildcard matching
    if permission.Resource == "*" || permission.Action == "*" {
        return true
    }
    
    // Pattern matching for resources like "hotels:*" or "reviews:own"
    if strings.HasSuffix(permission.Resource, ":*") {
        resourcePrefix := strings.TrimSuffix(permission.Resource, ":*")
        return strings.HasPrefix(resource, resourcePrefix)
    }
    
    return false
}

// Middleware for authorization
func (authz *AuthorizationService) RequirePermission(resource, action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get user claims from context (set by auth middleware)
        claims, exists := c.Get("user_claims")
        if !exists {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        
        userClaims := claims.(*JWTClaims)
        
        // Check permission
        if !authz.HasPermission(userClaims.Roles, resource, action) {
            c.JSON(403, gin.H{
                "error": "forbidden",
                "message": fmt.Sprintf("insufficient permissions for %s:%s", resource, action),
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

### 3. API Key Authentication

```go
type APIKeyService struct {
    db           Database
    keyCache     map[string]*APIKey
    rateLimiter  RateLimiter
    mutex        sync.RWMutex
}

type APIKey struct {
    ID          uuid.UUID `json:"id"`
    UserID      uuid.UUID `json:"user_id"`
    Name        string    `json:"name"`
    KeyHash     string    `json:"key_hash"`
    KeyPrefix   string    `json:"key_prefix"`
    Permissions []string  `json:"permissions"`
    RateLimit   int       `json:"rate_limit"`
    LastUsed    time.Time `json:"last_used"`
    UsageCount  int64     `json:"usage_count"`
    Active      bool      `json:"active"`
    ExpiresAt   time.Time `json:"expires_at"`
    CreatedAt   time.Time `json:"created_at"`
}

func (aks *APIKeyService) CreateAPIKey(userID uuid.UUID, name string, permissions []string) (*APIKey, string, error) {
    // Generate random key
    keyBytes := make([]byte, 32)
    if _, err := rand.Read(keyBytes); err != nil {
        return nil, "", fmt.Errorf("failed to generate random key: %w", err)
    }
    
    plainKey := "ak_" + base64.URLEncoding.EncodeToString(keyBytes)[:40]
    keyHash := hashAPIKey(plainKey)
    
    apiKey := &APIKey{
        ID:          uuid.New(),
        UserID:      userID,
        Name:        name,
        KeyHash:     keyHash,
        KeyPrefix:   plainKey[:8], // Store prefix for identification
        Permissions: permissions,
        RateLimit:   5000, // Default rate limit
        Active:      true,
        ExpiresAt:   time.Now().AddDate(1, 0, 0), // 1 year
        CreatedAt:   time.Now(),
    }
    
    if err := aks.db.CreateAPIKey(apiKey); err != nil {
        return nil, "", fmt.Errorf("failed to store API key: %w", err)
    }
    
    return apiKey, plainKey, nil
}

func (aks *APIKeyService) ValidateAPIKey(key string) (*APIKey, error) {
    if !strings.HasPrefix(key, "ak_") {
        return nil, fmt.Errorf("invalid API key format")
    }
    
    keyHash := hashAPIKey(key)
    prefix := key[:8]
    
    // Check cache first
    aks.mutex.RLock()
    if cached, exists := aks.keyCache[keyHash]; exists {
        aks.mutex.RUnlock()
        return cached, nil
    }
    aks.mutex.RUnlock()
    
    // Get from database
    apiKey, err := aks.db.GetAPIKeyByHash(keyHash)
    if err != nil {
        return nil, fmt.Errorf("API key not found: %w", err)
    }
    
    // Validate key
    if !apiKey.Active {
        return nil, fmt.Errorf("API key is inactive")
    }
    
    if time.Now().After(apiKey.ExpiresAt) {
        return nil, fmt.Errorf("API key expired")
    }
    
    // Check rate limit
    if !aks.rateLimiter.Allow(keyHash, apiKey.RateLimit) {
        return nil, fmt.Errorf("rate limit exceeded")
    }
    
    // Update usage statistics
    go aks.updateUsageStats(apiKey)
    
    // Cache the key
    aks.mutex.Lock()
    aks.keyCache[keyHash] = apiKey
    aks.mutex.Unlock()
    
    return apiKey, nil
}

func hashAPIKey(key string) string {
    hash := sha256.Sum256([]byte(key))
    return hex.EncodeToString(hash[:])
}
```

## Input Validation and Sanitization

### 1. Request Validation

```go
type Validator struct {
    validate *validator.Validate
}

func NewValidator() *Validator {
    validate := validator.New()
    
    // Custom validation rules
    validate.RegisterValidation("hotel_rating", validateHotelRating)
    validate.RegisterValidation("review_content", validateReviewContent)
    validate.RegisterValidation("safe_html", validateSafeHTML)
    
    return &Validator{validate: validate}
}

func validateHotelRating(fl validator.FieldLevel) bool {
    rating := fl.Field().Float()
    return rating >= 0 && rating <= 5
}

func validateReviewContent(fl validator.FieldLevel) bool {
    content := fl.Field().String()
    
    // Check length
    if len(content) < 10 || len(content) > 5000 {
        return false
    }
    
    // Check for spam patterns
    if containsSpam(content) {
        return false
    }
    
    // Check for inappropriate content
    if containsInappropriateContent(content) {
        return false
    }
    
    return true
}

func validateSafeHTML(fl validator.FieldLevel) bool {
    html := fl.Field().String()
    
    // Use bluemonday to sanitize HTML
    policy := bluemonday.UGCPolicy()
    sanitized := policy.Sanitize(html)
    
    // Return true if content wasn't changed (was already safe)
    return sanitized == html
}

// Validation middleware
func ValidationMiddleware(validator *Validator) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get request body
        body, err := ioutil.ReadAll(c.Request.Body)
        if err != nil {
            c.JSON(400, gin.H{"error": "failed to read request body"})
            c.Abort()
            return
        }
        
        // Restore body for next handlers
        c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
        
        // Parse and validate based on content type
        if c.GetHeader("Content-Type") == "application/json" {
            if !json.Valid(body) {
                c.JSON(400, gin.H{"error": "invalid JSON format"})
                c.Abort()
                return
            }
        }
        
        c.Next()
    }
}
```

### 2. SQL Injection Prevention

```go
type SafeQueryBuilder struct {
    db *sql.DB
}

// Parameterized query builder
func (sqb *SafeQueryBuilder) BuildSearchQuery(filters *SearchFilters) (string, []interface{}) {
    var conditions []string
    var args []interface{}
    argIndex := 1
    
    baseQuery := `
        SELECT r.id, r.title, r.content, r.rating, r.review_date, h.name as hotel_name
        FROM reviews r
        JOIN hotels h ON r.hotel_id = h.id
        WHERE r.deleted_at IS NULL
    `
    
    if filters.HotelID != "" {
        conditions = append(conditions, fmt.Sprintf("r.hotel_id = $%d", argIndex))
        args = append(args, filters.HotelID)
        argIndex++
    }
    
    if filters.MinRating > 0 {
        conditions = append(conditions, fmt.Sprintf("r.rating >= $%d", argIndex))
        args = append(args, filters.MinRating)
        argIndex++
    }
    
    if filters.MaxRating > 0 {
        conditions = append(conditions, fmt.Sprintf("r.rating <= $%d", argIndex))
        args = append(args, filters.MaxRating)
        argIndex++
    }
    
    if filters.TextSearch != "" {
        // Use full-text search to prevent injection
        conditions = append(conditions, fmt.Sprintf("to_tsvector('english', r.content) @@ plainto_tsquery('english', $%d)", argIndex))
        args = append(args, filters.TextSearch)
        argIndex++
    }
    
    if len(conditions) > 0 {
        baseQuery += " AND " + strings.Join(conditions, " AND ")
    }
    
    // Safe ordering (whitelist approach)
    allowedSortFields := map[string]string{
        "date":   "r.review_date",
        "rating": "r.rating",
        "hotel":  "h.name",
    }
    
    if sortField, exists := allowedSortFields[filters.SortBy]; exists {
        sortOrder := "ASC"
        if filters.SortOrder == "desc" {
            sortOrder = "DESC"
        }
        baseQuery += fmt.Sprintf(" ORDER BY %s %s", sortField, sortOrder)
    }
    
    // Add limit and offset
    baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
    args = append(args, filters.Limit, filters.Offset)
    
    return baseQuery, args
}
```

## Data Encryption and Protection

### 1. Data Encryption at Rest

```go
type EncryptionService struct {
    key        []byte
    gcm        cipher.AEAD
    keyManager KeyManager
}

func NewEncryptionService(keyManager KeyManager) (*EncryptionService, error) {
    // Get encryption key from key manager
    key, err := keyManager.GetKey("data-encryption")
    if err != nil {
        return nil, fmt.Errorf("failed to get encryption key: %w", err)
    }
    
    // Create AES-GCM cipher
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("failed to create cipher: %w", err)
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCM: %w", err)
    }
    
    return &EncryptionService{
        key:        key,
        gcm:        gcm,
        keyManager: keyManager,
    }, nil
}

func (es *EncryptionService) Encrypt(data []byte) ([]byte, error) {
    // Generate random nonce
    nonce := make([]byte, es.gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return nil, fmt.Errorf("failed to generate nonce: %w", err)
    }
    
    // Encrypt data
    ciphertext := es.gcm.Seal(nonce, nonce, data, nil)
    
    return ciphertext, nil
}

func (es *EncryptionService) Decrypt(ciphertext []byte) ([]byte, error) {
    nonceSize := es.gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, fmt.Errorf("ciphertext too short")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    
    plaintext, err := es.gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, fmt.Errorf("decryption failed: %w", err)
    }
    
    return plaintext, nil
}

// Encrypt sensitive fields in database
func (es *EncryptionService) EncryptSensitiveData(review *Review) error {
    // Encrypt email addresses in author information
    if strings.Contains(review.AuthorName, "@") {
        encrypted, err := es.Encrypt([]byte(review.AuthorName))
        if err != nil {
            return fmt.Errorf("failed to encrypt author name: %w", err)
        }
        review.AuthorName = base64.StdEncoding.EncodeToString(encrypted)
    }
    
    // Encrypt IP addresses if stored
    if review.Metadata != nil {
        if ip, exists := review.Metadata["ip_address"]; exists {
            encrypted, err := es.Encrypt([]byte(ip.(string)))
            if err != nil {
                return fmt.Errorf("failed to encrypt IP address: %w", err)
            }
            review.Metadata["ip_address"] = base64.StdEncoding.EncodeToString(encrypted)
        }
    }
    
    return nil
}
```

### 2. PII Data Masking

```go
type DataMaskingService struct {
    patterns map[string]*regexp.Regexp
}

func NewDataMaskingService() *DataMaskingService {
    patterns := map[string]*regexp.Regexp{
        "email":       regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
        "phone":       regexp.MustCompile(`(\+?1[-.\s]?)?\(?([0-9]{3})\)?[-.\s]?([0-9]{3})[-.\s]?([0-9]{4})`),
        "credit_card": regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
        "ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
    }
    
    return &DataMaskingService{patterns: patterns}
}

func (dms *DataMaskingService) MaskSensitiveData(content string) string {
    masked := content
    
    // Mask email addresses
    masked = dms.patterns["email"].ReplaceAllStringFunc(masked, func(match string) string {
        parts := strings.Split(match, "@")
        if len(parts) != 2 {
            return "***@***.***"
        }
        
        username := parts[0]
        domain := parts[1]
        
        if len(username) <= 2 {
            username = "***"
        } else {
            username = username[:2] + "***"
        }
        
        return username + "@" + domain
    })
    
    // Mask phone numbers
    masked = dms.patterns["phone"].ReplaceAllString(masked, "***-***-****")
    
    // Mask credit card numbers
    masked = dms.patterns["credit_card"].ReplaceAllString(masked, "****-****-****-****")
    
    // Mask SSNs
    masked = dms.patterns["ssn"].ReplaceAllString(masked, "***-**-****")
    
    return masked
}

func (dms *DataMaskingService) MaskReviewContent(review *Review, userRole string) {
    // Only mask for non-admin users
    if userRole == "admin" || userRole == "moderator" {
        return
    }
    
    review.Content = dms.MaskSensitiveData(review.Content)
    review.Title = dms.MaskSensitiveData(review.Title)
    
    // Mask author information
    if review.AuthorName != "" && strings.Contains(review.AuthorName, "@") {
        review.AuthorName = dms.MaskSensitiveData(review.AuthorName)
    }
}
```

## Security Monitoring and Logging

### 1. Security Event Logging

```go
type SecurityLogger struct {
    logger     *logrus.Logger
    alerter    AlertService
    eventStore EventStore
}

type SecurityEvent struct {
    ID          uuid.UUID                `json:"id"`
    Type        string                   `json:"type"`
    Severity    string                   `json:"severity"`
    UserID      *uuid.UUID               `json:"user_id,omitempty"`
    IPAddress   string                   `json:"ip_address"`
    UserAgent   string                   `json:"user_agent"`
    Endpoint    string                   `json:"endpoint"`
    Method      string                   `json:"method"`
    StatusCode  int                      `json:"status_code"`
    Details     map[string]interface{}   `json:"details"`
    Timestamp   time.Time                `json:"timestamp"`
    RequestID   string                   `json:"request_id"`
}

func (sl *SecurityLogger) LogAuthenticationAttempt(ctx context.Context, email string, success bool, reason string) {
    event := &SecurityEvent{
        ID:        uuid.New(),
        Type:      "authentication_attempt",
        Severity:  "info",
        IPAddress: getClientIP(ctx),
        UserAgent: getUserAgent(ctx),
        Details: map[string]interface{}{
            "email":   email,
            "success": success,
            "reason":  reason,
        },
        Timestamp: time.Now(),
        RequestID: getRequestID(ctx),
    }
    
    if !success {
        event.Severity = "warning"
    }
    
    sl.logEvent(event)
    
    // Check for brute force attempts
    go sl.checkBruteForce(ctx, email, getClientIP(ctx))
}

func (sl *SecurityLogger) LogSuspiciousActivity(ctx context.Context, activityType string, details map[string]interface{}) {
    event := &SecurityEvent{
        ID:         uuid.New(),
        Type:       "suspicious_activity",
        Severity:   "high",
        IPAddress:  getClientIP(ctx),
        UserAgent:  getUserAgent(ctx),
        Details:    details,
        Timestamp:  time.Now(),
        RequestID:  getRequestID(ctx),
    }
    
    if userID := getUserIDFromContext(ctx); userID != nil {
        event.UserID = userID
    }
    
    sl.logEvent(event)
    
    // Send immediate alert for high-severity events
    sl.alerter.SendSecurityAlert(event)
}

func (sl *SecurityLogger) checkBruteForce(ctx context.Context, email, ipAddress string) {
    // Count failed attempts in last 15 minutes
    since := time.Now().Add(-15 * time.Minute)
    
    emailAttempts := sl.eventStore.CountFailedLogins(email, since)
    ipAttempts := sl.eventStore.CountFailedLogins(ipAddress, since)
    
    if emailAttempts >= 5 || ipAttempts >= 10 {
        sl.LogSuspiciousActivity(ctx, "brute_force_attempt", map[string]interface{}{
            "email":           email,
            "email_attempts":  emailAttempts,
            "ip_attempts":     ipAttempts,
            "time_window":     "15m",
        })
        
        // Temporarily block the IP
        sl.blockIP(ipAddress, 30*time.Minute)
    }
}
```

### 2. Intrusion Detection

```go
type IntrusionDetectionService struct {
    rules       []DetectionRule
    alerter     AlertService
    blockList   map[string]time.Time
    rateLimiter RateLimiter
    mutex       sync.RWMutex
}

type DetectionRule struct {
    ID          string
    Name        string
    Pattern     *regexp.Regexp
    Threshold   int
    TimeWindow  time.Duration
    Action      string
    Severity    string
}

func (ids *IntrusionDetectionService) LoadDetectionRules() {
    ids.rules = []DetectionRule{
        {
            ID:         "sql_injection",
            Name:       "SQL Injection Attempt",
            Pattern:    regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop|create|alter).*(\s|%20)+(from|where|into)`),
            Threshold:  3,
            TimeWindow: 5 * time.Minute,
            Action:     "block",
            Severity:   "critical",
        },
        {
            ID:         "xss_attempt",
            Name:       "Cross-Site Scripting Attempt",
            Pattern:    regexp.MustCompile(`(?i)<script.*?>.*?</script>|javascript:|on\w+\s*=`),
            Threshold:  2,
            TimeWindow: 5 * time.Minute,
            Action:     "alert",
            Severity:   "high",
        },
        {
            ID:         "directory_traversal",
            Name:       "Directory Traversal Attempt",
            Pattern:    regexp.MustCompile(`(\.\./|\.\.\\|%2e%2e%2f|%2e%2e%5c)`),
            Threshold:  1,
            TimeWindow: 1 * time.Minute,
            Action:     "block",
            Severity:   "high",
        },
        {
            ID:         "excessive_requests",
            Name:       "Excessive Request Rate",
            Threshold:  100,
            TimeWindow: 1 * time.Minute,
            Action:     "rate_limit",
            Severity:   "medium",
        },
    }
}

func (ids *IntrusionDetectionService) AnalyzeRequest(ctx context.Context, req *http.Request) bool {
    clientIP := getClientIP(ctx)
    userAgent := req.UserAgent()
    requestURL := req.URL.String()
    
    // Check if IP is blocked
    ids.mutex.RLock()
    if blockedUntil, exists := ids.blockList[clientIP]; exists && time.Now().Before(blockedUntil) {
        ids.mutex.RUnlock()
        return false // Request blocked
    }
    ids.mutex.RUnlock()
    
    // Analyze against detection rules
    for _, rule := range ids.rules {
        if ids.ruleMatches(rule, req) {
            ids.handleRuleViolation(ctx, rule, clientIP, userAgent, requestURL)
            
            if rule.Action == "block" {
                return false
            }
        }
    }
    
    return true // Request allowed
}

func (ids *IntrusionDetectionService) ruleMatches(rule DetectionRule, req *http.Request) bool {
    // Check pattern-based rules
    if rule.Pattern != nil {
        // Check URL
        if rule.Pattern.MatchString(req.URL.String()) {
            return true
        }
        
        // Check query parameters
        for _, values := range req.URL.Query() {
            for _, value := range values {
                if rule.Pattern.MatchString(value) {
                    return true
                }
            }
        }
        
        // Check headers
        for _, values := range req.Header {
            for _, value := range values {
                if rule.Pattern.MatchString(value) {
                    return true
                }
            }
        }
        
        // Check body for POST requests
        if req.Method == "POST" && req.Body != nil {
            body, err := ioutil.ReadAll(req.Body)
            if err == nil {
                req.Body = ioutil.NopCloser(bytes.NewBuffer(body)) // Restore body
                if rule.Pattern.Match(body) {
                    return true
                }
            }
        }
    }
    
    // Check rate-based rules
    if rule.ID == "excessive_requests" {
        clientIP := getClientIP(req.Context())
        return !ids.rateLimiter.Allow(clientIP, rule.Threshold)
    }
    
    return false
}

func (ids *IntrusionDetectionService) handleRuleViolation(ctx context.Context, rule DetectionRule, clientIP, userAgent, requestURL string) {
    // Log the violation
    violation := &SecurityEvent{
        ID:        uuid.New(),
        Type:      "intrusion_attempt",
        Severity:  rule.Severity,
        IPAddress: clientIP,
        UserAgent: userAgent,
        Details: map[string]interface{}{
            "rule_id":     rule.ID,
            "rule_name":   rule.Name,
            "request_url": requestURL,
            "action":      rule.Action,
        },
        Timestamp: time.Now(),
    }
    
    // Store the event
    ids.alerter.LogSecurityEvent(violation)
    
    // Take action based on rule configuration
    switch rule.Action {
    case "block":
        ids.blockIP(clientIP, 1*time.Hour)
        ids.alerter.SendImmediateAlert(violation)
    case "alert":
        ids.alerter.SendSecurityAlert(violation)
    case "rate_limit":
        ids.rateLimiter.SetLimit(clientIP, rule.Threshold/2) // Reduce rate limit
    }
}

func (ids *IntrusionDetectionService) blockIP(ip string, duration time.Duration) {
    ids.mutex.Lock()
    defer ids.mutex.Unlock()
    
    ids.blockList[ip] = time.Now().Add(duration)
    
    log.Warnf("IP %s blocked for %v due to security violation", ip, duration)
}
```

## Secrets Management

### 1. External Secrets Integration

```go
type SecretsManager struct {
    vaultClient   *vault.Client
    awsKMS       *kms.KMS
    localSecrets map[string]string
    cache        map[string]*CachedSecret
    cacheTTL     time.Duration
    mutex        sync.RWMutex
}

type CachedSecret struct {
    Value     string
    ExpiresAt time.Time
}

func NewSecretsManager(vaultAddr, awsRegion string) (*SecretsManager, error) {
    // Initialize Vault client
    vaultConfig := vault.DefaultConfig()
    vaultConfig.Address = vaultAddr
    vaultClient, err := vault.NewClient(vaultConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create Vault client: %w", err)
    }
    
    // Initialize AWS KMS
    sess := session.Must(session.NewSession(&aws.Config{
        Region: aws.String(awsRegion),
    }))
    awsKMS := kms.New(sess)
    
    return &SecretsManager{
        vaultClient:  vaultClient,
        awsKMS:      awsKMS,
        localSecrets: make(map[string]string),
        cache:       make(map[string]*CachedSecret),
        cacheTTL:    5 * time.Minute,
    }, nil
}

func (sm *SecretsManager) GetSecret(key string) (string, error) {
    // Check cache first
    sm.mutex.RLock()
    if cached, exists := sm.cache[key]; exists && time.Now().Before(cached.ExpiresAt) {
        sm.mutex.RUnlock()
        return cached.Value, nil
    }
    sm.mutex.RUnlock()
    
    var value string
    var err error
    
    // Try Vault first
    if sm.vaultClient != nil {
        value, err = sm.getFromVault(key)
        if err == nil {
            sm.cacheSecret(key, value)
            return value, nil
        }
        log.Warnf("Failed to get secret from Vault: %v", err)
    }
    
    // Try AWS Secrets Manager
    value, err = sm.getFromAWSSecretsManager(key)
    if err == nil {
        sm.cacheSecret(key, value)
        return value, nil
    }
    log.Warnf("Failed to get secret from AWS Secrets Manager: %v", err)
    
    // Fall back to environment variables
    value = os.Getenv(key)
    if value != "" {
        sm.cacheSecret(key, value)
        return value, nil
    }
    
    return "", fmt.Errorf("secret not found: %s", key)
}

func (sm *SecretsManager) getFromVault(key string) (string, error) {
    secret, err := sm.vaultClient.Logical().Read("secret/data/" + key)
    if err != nil {
        return "", fmt.Errorf("vault read error: %w", err)
    }
    
    if secret == nil || secret.Data == nil {
        return "", fmt.Errorf("secret not found in vault: %s", key)
    }
    
    data, ok := secret.Data["data"].(map[string]interface{})
    if !ok {
        return "", fmt.Errorf("invalid secret format in vault")
    }
    
    value, ok := data["value"].(string)
    if !ok {
        return "", fmt.Errorf("secret value not found in vault")
    }
    
    return value, nil
}

func (sm *SecretsManager) cacheSecret(key, value string) {
    sm.mutex.Lock()
    defer sm.mutex.Unlock()
    
    sm.cache[key] = &CachedSecret{
        Value:     value,
        ExpiresAt: time.Now().Add(sm.cacheTTL),
    }
}

// Rotate secrets periodically
func (sm *SecretsManager) StartSecretRotation(ctx context.Context) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            sm.rotateSecrets(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (sm *SecretsManager) rotateSecrets(ctx context.Context) {
    secretsToRotate := []string{
        "jwt_secret",
        "api_encryption_key",
        "database_password",
    }
    
    for _, secretKey := range secretsToRotate {
        if sm.shouldRotateSecret(secretKey) {
            if err := sm.rotateSecret(ctx, secretKey); err != nil {
                log.Errorf("Failed to rotate secret %s: %v", secretKey, err)
            }
        }
    }
}
```

This comprehensive security guide covers authentication, authorization, data protection, monitoring, and best practices for the Hotel Reviews Microservice, ensuring enterprise-grade security implementation.