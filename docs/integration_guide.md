# Integration Guide: Full-Featured Hotel Reviews Microservice

## Overview

This guide explains how to integrate all features of the hotel reviews microservice into your main application. The integrated system includes:

- **Core Review Management**: Hotel reviews CRUD operations
- **Circuit Breaker Protection**: Fault tolerance for external services
- **Retry Mechanisms**: Automatic retry with exponential backoff
- **JWT Authentication**: Secure user authentication
- **RBAC Authorization**: Role-based access control
- **API Key Authentication**: Service-to-service authentication
- **Comprehensive Monitoring**: Health checks and metrics

## Integration Steps

### 1. Replace Main Application

**Current**: `cmd/api/main.go` (basic functionality)
**New**: `cmd/api/main_integrated.go` (full-featured)

```bash
# Backup original main.go
mv cmd/api/main.go cmd/api/main_original.go

# Use the integrated version
mv cmd/api/main_integrated.go cmd/api/main.go
```

### 2. Update Configuration

Add authentication configuration to your `config.yaml`:

```yaml
auth:
  enabled: true
  jwt:
    secret: "your-jwt-secret-key"
    access_token_duration: 15m
    refresh_token_duration: 168h
    issuer: "hotel-reviews-service"
    audience: "hotel-reviews-users"
  password:
    min_length: 8
    require_uppercase: true
    require_lowercase: true
    require_numbers: true
    require_symbols: true
  rate_limit:
    login_attempts: 5
    window_minutes: 15
    lockout_duration: 30m
  session:
    cleanup_interval: 1h
    max_concurrent_sessions: 5
  api_key:
    enabled: true
    default_expiry: 8760h # 1 year
    
redis:
  address: "localhost:6379"
  password: ""
  database: 0
  pool_size: 10
  
circuit_breaker:
  enabled: true
  database:
    failure_threshold: 5
    timeout: 30s
  cache:
    failure_threshold: 10
    timeout: 15s
  s3:
    failure_threshold: 3
    timeout: 60s
    
retry:
  enabled: true
  max_attempts: 3
  base_delay: 1s
  max_delay: 30s
```

### 3. Database Migration

Run the authentication database migration:

```bash
# Apply authentication schema
./hotel-reviews -mode migrate -migrate-up

# Create default admin user
./hotel-reviews -mode create-admin -admin-email admin@yourcompany.com

# Generate API key for service-to-service calls
./hotel-reviews -mode generate-api-key -api-key-name production-service -api-key-scopes read,write,admin
```

### 4. Environment Variables

Set required environment variables:

```bash
export HOTEL_REVIEWS_JWT_SECRET="your-super-secure-jwt-secret-key"
export HOTEL_REVIEWS_DATABASE_URL="postgresql://user:pass@localhost:5432/hotel_reviews"
export HOTEL_REVIEWS_REDIS_ADDRESS="localhost:6379"
export HOTEL_REVIEWS_S3_BUCKET="your-s3-bucket"
export HOTEL_REVIEWS_S3_REGION="us-west-2"
```

### 5. Update Dependencies

Ensure all required dependencies are installed:

```bash
go mod tidy
```

## Feature Overview

### 🔐 Authentication Endpoints

```
POST /api/v1/auth/register     - User registration
POST /api/v1/auth/login        - User login
POST /api/v1/auth/refresh      - Token refresh
POST /api/v1/auth/logout       - User logout
GET  /api/v1/auth/profile      - User profile
PUT  /api/v1/auth/profile      - Update profile
POST /api/v1/auth/change-password - Change password
```

### 🛡️ Protected API Endpoints

**Reviews (JWT Required)**:
```
GET  /api/v1/reviews          - List reviews
POST /api/v1/reviews          - Create review
GET  /api/v1/reviews/:id      - Get review
PUT  /api/v1/reviews/:id      - Update review
DELETE /api/v1/reviews/:id    - Delete review
```

**Hotels (JWT Required)**:
```
GET  /api/v1/hotels           - List hotels
POST /api/v1/hotels           - Create hotel
GET  /api/v1/hotels/:id       - Get hotel
PUT  /api/v1/hotels/:id       - Update hotel
DELETE /api/v1/hotels/:id     - Delete hotel
```

### 👑 Admin Endpoints (Admin Role Required)

```
GET  /api/v1/admin/users              - List users
POST /api/v1/admin/users              - Create user
GET  /api/v1/admin/users/:id          - Get user
PUT  /api/v1/admin/users/:id          - Update user
DELETE /api/v1/admin/users/:id        - Delete user
POST /api/v1/admin/users/:id/activate - Activate user
GET  /api/v1/admin/roles              - List roles
POST /api/v1/admin/roles              - Create role
GET  /api/v1/admin/api-keys           - List API keys
POST /api/v1/admin/api-keys           - Create API key
POST /api/v1/admin/system/reset-circuit-breakers - Reset circuit breakers
```

### 🔑 Service Endpoints (API Key Required)

```
POST /api/v1/service/reviews/bulk     - Bulk create reviews
GET  /api/v1/service/reviews/export   - Export reviews
POST /api/v1/service/reviews/import   - Import reviews
GET  /api/v1/service/stats/reviews    - Review statistics
GET  /api/v1/service/health          - Service health
```

### 📊 Monitoring Endpoints

```
GET  /health                         - Overall health
GET  /health/circuit-breakers        - Circuit breaker health
GET  /metrics/circuit-breakers       - Circuit breaker metrics
```

## Usage Examples

### 1. Starting the Integrated Application

```bash
# Start with all features enabled
./hotel-reviews -mode server

# Start without authentication (development only)
./hotel-reviews -mode server -disable-auth

# Start without circuit breakers
./hotel-reviews -mode server -disable-circuit-breaker

# Start without retry mechanisms
./hotel-reviews -mode server -disable-retry
```

### 2. User Authentication Flow

```bash
# Register a new user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!",
    "first_name": "John",
    "last_name": "Doe"
  }'

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!"
  }'

# Use JWT token for protected endpoints
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/reviews
```

### 3. Admin Operations

```bash
# List all users (admin only)
curl -H "Authorization: Bearer ADMIN_JWT_TOKEN" \
  http://localhost:8080/api/v1/admin/users

# Create API key (admin only)
curl -X POST http://localhost:8080/api/v1/admin/api-keys \
  -H "Authorization: Bearer ADMIN_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mobile-app",
    "scopes": ["read", "write"]
  }'

# Reset circuit breakers (admin only)
curl -X POST http://localhost:8080/api/v1/admin/system/reset-circuit-breakers \
  -H "Authorization: Bearer ADMIN_JWT_TOKEN"
```

### 4. Service-to-Service Authentication

```bash
# Use API key for service endpoints
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/service/reviews/export

# Bulk create reviews
curl -X POST http://localhost:8080/api/v1/service/reviews/bulk \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '[
    {
      "hotel_id": "hotel-uuid",
      "rating": 4.5,
      "comment": "Great stay!"
    }
  ]'
```

## Configuration Options

### Authentication Configuration

```yaml
auth:
  enabled: true                    # Enable/disable authentication
  jwt:
    secret: "secret-key"           # JWT signing secret
    access_token_duration: 15m     # Access token lifetime
    refresh_token_duration: 168h   # Refresh token lifetime
  password:
    min_length: 8                  # Minimum password length
    require_uppercase: true        # Require uppercase letters
    require_lowercase: true        # Require lowercase letters
    require_numbers: true          # Require numbers
    require_symbols: true          # Require symbols
  rate_limit:
    login_attempts: 5              # Max login attempts
    window_minutes: 15             # Rate limit window
    lockout_duration: 30m          # Account lockout duration
```

### Circuit Breaker Configuration

```yaml
circuit_breaker:
  enabled: true                    # Enable/disable circuit breakers
  database:
    failure_threshold: 5           # Failures before opening
    success_threshold: 3           # Successes before closing
    timeout: 30s                   # Open timeout
  cache:
    failure_threshold: 10          # Cache is less critical
    success_threshold: 5
    timeout: 15s
  s3:
    failure_threshold: 3           # S3 is critical
    success_threshold: 2
    timeout: 60s
```

### Retry Configuration

```yaml
retry:
  enabled: true                    # Enable/disable retry mechanisms
  database:
    max_attempts: 3                # Maximum retry attempts
    base_delay: 500ms              # Base delay between retries
    max_delay: 5s                  # Maximum delay
    multiplier: 2.0                # Exponential backoff multiplier
  cache:
    max_attempts: 2                # Fewer retries for cache
    base_delay: 100ms
    max_delay: 1s
  s3:
    max_attempts: 5                # More retries for S3
    base_delay: 1s
    max_delay: 30s
```

## Security Considerations

### 1. JWT Security

- **Secret Key**: Use a strong, random secret key (minimum 32 characters)
- **Token Expiry**: Short-lived access tokens (15 minutes recommended)
- **Refresh Tokens**: Longer-lived but revocable (7 days recommended)
- **Token Storage**: Store refresh tokens securely (httpOnly cookies recommended)

### 2. API Key Security

- **Key Generation**: Use cryptographically secure random generation
- **Key Storage**: Hash API keys in database
- **Key Rotation**: Implement regular key rotation
- **Scope Limitations**: Limit API key scopes to minimum required permissions

### 3. Rate Limiting

- **Login Protection**: Limit login attempts per IP/user
- **API Protection**: Rate limit API endpoints
- **Account Lockout**: Temporary lockout after failed attempts
- **Distributed Rate Limiting**: Use Redis for distributed rate limiting

### 4. Password Security

- **Hashing**: Use bcrypt with appropriate cost factor
- **Strength Requirements**: Enforce strong password policies
- **History**: Prevent password reuse
- **Expiry**: Optional password expiry policies

## Monitoring and Observability

### 1. Health Checks

```bash
# Overall health
curl http://localhost:8080/health

# Circuit breaker health
curl http://localhost:8080/health/circuit-breakers

# Authentication status
./hotel-reviews -mode auth-status
```

### 2. Metrics Collection

- **Authentication Metrics**: Login attempts, success rates, active sessions
- **Circuit Breaker Metrics**: State transitions, failure rates, response times
- **Retry Metrics**: Retry attempts, success rates, backoff timing
- **API Metrics**: Request counts, response times, error rates

### 3. Audit Logging

- **Authentication Events**: Login, logout, password changes
- **Authorization Events**: Permission checks, role changes
- **Administrative Events**: User management, system configuration
- **Security Events**: Failed authentication, suspicious activities

## Troubleshooting

### Common Issues

1. **Authentication Failures**
   - Check JWT secret configuration
   - Verify token expiry settings
   - Check database connectivity

2. **Circuit Breaker Issues**
   - Monitor failure thresholds
   - Check service health
   - Review timeout settings

3. **Performance Issues**
   - Monitor retry backoff timing
   - Check circuit breaker state
   - Review rate limiting configuration

### Debug Commands

```bash
# Check authentication status
./hotel-reviews -mode auth-status

# Check circuit breaker status
./hotel-reviews -mode circuit-breaker-status

# Check overall health
./hotel-reviews -mode health-check

# Reset circuit breakers
./hotel-reviews -reset-circuit-breakers
```

## Migration from Original Application

### Step-by-Step Migration

1. **Backup Current Application**
   ```bash
   cp cmd/api/main.go cmd/api/main_backup.go
   ```

2. **Deploy Integrated Application**
   ```bash
   cp cmd/api/main_integrated.go cmd/api/main.go
   ```

3. **Run Database Migrations**
   ```bash
   ./hotel-reviews -mode migrate -migrate-up
   ```

4. **Create Initial Admin User**
   ```bash
   ./hotel-reviews -mode create-admin -admin-email admin@yourcompany.com
   ```

5. **Update Configuration**
   - Add authentication settings
   - Configure circuit breakers
   - Set up retry policies

6. **Test Integration**
   - Verify authentication works
   - Test circuit breaker behavior
   - Validate retry mechanisms

7. **Update Client Applications**
   - Add JWT token handling
   - Update API endpoints
   - Handle new error responses

## Rollback Strategy

If issues arise during integration:

1. **Immediate Rollback**
   ```bash
   cp cmd/api/main_backup.go cmd/api/main.go
   go build -o hotel-reviews cmd/api/main.go
   ```

2. **Database Rollback**
   ```bash
   ./hotel-reviews -mode migrate -migrate-down
   ```

3. **Configuration Rollback**
   - Remove authentication settings
   - Disable circuit breakers
   - Revert to original configuration

## Support

For issues or questions regarding the integration:

1. **Check Documentation**: Review all documentation files
2. **Check Examples**: Review example usage files
3. **Check Logs**: Enable debug logging for troubleshooting
4. **Check Health Endpoints**: Monitor system health
5. **Check Metrics**: Review circuit breaker and retry metrics

The integrated application provides a robust, production-ready microservice with comprehensive authentication, authorization, and resilience features.