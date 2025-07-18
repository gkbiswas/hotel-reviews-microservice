# Authentication and Authorization System

This document describes the comprehensive JWT-based authentication and RBAC authorization system implemented for the hotel reviews microservice.

## Table of Contents

1. [Overview](#overview)
2. [Features](#features)
3. [Architecture](#architecture)
4. [Components](#components)
5. [Database Schema](#database-schema)
6. [API Endpoints](#api-endpoints)
7. [Configuration](#configuration)
8. [Usage Examples](#usage-examples)
9. [Security Considerations](#security-considerations)
10. [Monitoring and Logging](#monitoring-and-logging)

## Overview

The authentication system provides a complete security framework for the hotel reviews microservice, including:

- **JWT-based authentication** with access and refresh tokens
- **Role-Based Access Control (RBAC)** system
- **API key authentication** for service-to-service calls
- **Rate limiting** for authentication attempts
- **Audit logging** for security events
- **Session management** with token refresh
- **Password security** with bcrypt hashing
- **Circuit breaker and retry** integration

## Features

### Authentication Features

- ✅ User registration and login
- ✅ JWT token generation and validation
- ✅ Access token (15 minutes) and refresh token (7 days) support
- ✅ Password strength validation
- ✅ Account lockout after failed attempts
- ✅ Session management
- ✅ Password reset functionality
- ✅ Email verification (configurable)
- ✅ Two-factor authentication (configurable)

### Authorization Features

- ✅ Role-based access control (RBAC)
- ✅ Permission-based resource access
- ✅ API key authentication for services
- ✅ Scope-based API key permissions
- ✅ Middleware for endpoint protection
- ✅ Optional authentication endpoints

### Security Features

- ✅ Rate limiting for login attempts
- ✅ Account lockout protection
- ✅ Audit logging for all authentication events
- ✅ Secure password hashing with bcrypt
- ✅ IP-based rate limiting
- ✅ Circuit breaker integration
- ✅ Retry mechanism support

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │    │   API Gateway   │    │   Auth Service  │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                        │                        │
         │ 1. Login Request       │                        │
         ├────────────────────────┼────────────────────────┤
         │                        │ 2. Validate Credentials│
         │                        ├────────────────────────┤
         │                        │ 3. Generate JWT        │
         │                        ├────────────────────────┤
         │ 4. Return JWT          │                        │
         ├────────────────────────┼────────────────────────┤
         │                        │                        │
         │ 5. Protected Request   │                        │
         │    (Bearer Token)      │                        │
         ├────────────────────────┼────────────────────────┤
         │                        │ 6. Validate JWT        │
         │                        ├────────────────────────┤
         │                        │ 7. Check Permissions   │
         │                        ├────────────────────────┤
         │ 8. Response            │                        │
         ├────────────────────────┼────────────────────────┤
```

## Components

### 1. Authentication Service

Main orchestrator that coordinates all authentication operations:

- **Location**: `internal/infrastructure/auth.go`
- **Responsibilities**:
  - User registration and login
  - Token generation and validation
  - Session management
  - Password management
  - Integration with other services

### 2. JWT Service

Handles JWT token operations:

- **Features**:
  - Access and refresh token generation
  - Token validation and parsing
  - Claims extraction
  - Circuit breaker integration

### 3. RBAC Service

Manages role-based access control:

- **Features**:
  - Permission checking
  - Role validation
  - Resource-based access control

### 4. API Key Service

Handles service-to-service authentication:

- **Features**:
  - API key generation
  - Key validation
  - Scope-based permissions
  - Usage tracking

### 5. Rate Limiting Service

Prevents brute force attacks:

- **Features**:
  - Login attempt tracking
  - IP-based rate limiting
  - Configurable limits and windows

### 6. Audit Service

Logs security events:

- **Features**:
  - Authentication event logging
  - User action tracking
  - Security incident recording

### 7. Middleware

HTTP middleware for endpoint protection:

- **Location**: `internal/infrastructure/middleware/auth_middleware.go`
- **Types**:
  - JWT authentication middleware
  - API key authentication middleware
  - Permission-based middleware
  - Role-based middleware
  - Rate limiting middleware
  - Audit middleware

## Database Schema

### Core Tables

#### Users Table
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    is_verified BOOLEAN DEFAULT FALSE,
    last_login_at TIMESTAMP,
    failed_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### Roles Table
```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### Permissions Table
```sql
CREATE TABLE permissions (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(20) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### Sessions Table
```sql
CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    user_agent TEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Junction Tables

#### User-Role Assignment
```sql
CREATE TABLE user_roles (
    user_id UUID REFERENCES users(id),
    role_id UUID REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id)
);
```

#### Role-Permission Assignment
```sql
CREATE TABLE role_permissions (
    role_id UUID REFERENCES roles(id),
    permission_id UUID REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);
```

### Audit Tables

#### API Keys Table
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    expires_at TIMESTAMP,
    scopes JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### Audit Logs Table
```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    ip_address VARCHAR(45),
    result VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## API Endpoints

### Authentication Endpoints

#### Register User
```http
POST /api/v1/auth/register
Content-Type: application/json

{
    "username": "john_doe",
    "email": "john@example.com",
    "password": "SecurePassword123!",
    "first_name": "John",
    "last_name": "Doe"
}
```

#### Login
```http
POST /api/v1/auth/login
Content-Type: application/json

{
    "email": "john@example.com",
    "password": "SecurePassword123!"
}
```

Response:
```json
{
    "message": "login successful",
    "data": {
        "user": {
            "id": "123e4567-e89b-12d3-a456-426614174000",
            "username": "john_doe",
            "email": "john@example.com"
        },
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "expires_in": 900
    }
}
```

#### Refresh Token
```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### Logout
```http
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
```

### Protected Endpoints

#### Get User Profile
```http
GET /api/v1/user/profile
Authorization: Bearer <access_token>
```

#### Update Profile
```http
PUT /api/v1/user/profile
Authorization: Bearer <access_token>
Content-Type: application/json

{
    "first_name": "John",
    "last_name": "Smith"
}
```

#### Change Password
```http
POST /api/v1/user/change-password
Authorization: Bearer <access_token>
Content-Type: application/json

{
    "old_password": "OldPassword123!",
    "new_password": "NewPassword123!"
}
```

### API Key Management

#### Create API Key
```http
POST /api/v1/api-keys
Authorization: Bearer <access_token>
Content-Type: application/json

{
    "name": "My API Key",
    "scopes": ["reviews.read", "hotels.read"],
    "expires_at": "2024-12-31T23:59:59Z"
}
```

#### List API Keys
```http
GET /api/v1/api-keys
Authorization: Bearer <access_token>
```

#### Delete API Key
```http
DELETE /api/v1/api-keys/{id}
Authorization: Bearer <access_token>
```

### Admin Endpoints

#### List Users
```http
GET /api/v1/admin/users
Authorization: Bearer <admin_access_token>
```

#### Get User
```http
GET /api/v1/admin/users/{id}
Authorization: Bearer <admin_access_token>
```

#### Assign Role
```http
POST /api/v1/admin/users/{id}/roles
Authorization: Bearer <admin_access_token>
Content-Type: application/json

{
    "role_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

### Service Endpoints

#### Service Status
```http
GET /api/v1/service/status
X-API-Key: hr_abc123def456...
```

## Configuration

### Environment Variables

```bash
# Authentication Configuration
HOTEL_REVIEWS_AUTH_JWT_SECRET=your_super_secret_jwt_key_here
HOTEL_REVIEWS_AUTH_JWT_ISSUER=hotel-reviews-api
HOTEL_REVIEWS_AUTH_ACCESS_TOKEN_EXPIRY=15m
HOTEL_REVIEWS_AUTH_REFRESH_TOKEN_EXPIRY=7d

# Security Settings
HOTEL_REVIEWS_AUTH_MAX_LOGIN_ATTEMPTS=5
HOTEL_REVIEWS_AUTH_LOGIN_ATTEMPT_WINDOW=15m
HOTEL_REVIEWS_AUTH_ACCOUNT_LOCK_DURATION=30m

# Password Policy
HOTEL_REVIEWS_AUTH_PASSWORD_MIN_LENGTH=8
HOTEL_REVIEWS_AUTH_PASSWORD_MAX_LENGTH=128
HOTEL_REVIEWS_AUTH_REQUIRE_STRONG_PASSWORD=true
HOTEL_REVIEWS_AUTH_BCRYPT_COST=12

# Features
HOTEL_REVIEWS_AUTH_ENABLE_RATE_LIMITING=true
HOTEL_REVIEWS_AUTH_ENABLE_AUDIT_LOGGING=true
HOTEL_REVIEWS_AUTH_ENABLE_SESSION_CLEANUP=true
HOTEL_REVIEWS_AUTH_SESSION_CLEANUP_INTERVAL=1h

# API Keys
HOTEL_REVIEWS_AUTH_API_KEY_LENGTH=32
HOTEL_REVIEWS_AUTH_API_KEY_PREFIX=hr_
HOTEL_REVIEWS_AUTH_DEFAULT_ROLE=user
```

### YAML Configuration

```yaml
auth:
  jwt_secret: your_super_secret_jwt_key_here
  jwt_issuer: hotel-reviews-api
  access_token_expiry: 15m
  refresh_token_expiry: 7d
  max_login_attempts: 5
  login_attempt_window: 15m
  account_lock_duration: 30m
  password_min_length: 8
  password_max_length: 128
  require_strong_password: true
  enable_rate_limiting: true
  enable_audit_logging: true
  bcrypt_cost: 12
  api_key_length: 32
  api_key_prefix: hr_
  default_role: user
```

## Usage Examples

### Basic Authentication Flow

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
)

func main() {
    // Initialize authentication service
    authService := infrastructure.NewAuthenticationService(/* parameters */)
    
    ctx := context.Background()
    
    // Register user
    user := &domain.User{
        Username: "john_doe",
        Email: "john@example.com",
        FirstName: "John",
        LastName: "Doe",
    }
    
    err := authService.Register(ctx, user, "SecurePassword123!")
    if err != nil {
        log.Fatalf("Registration failed: %v", err)
    }
    
    // Login
    loginResponse, err := authService.Login(ctx, user.Email, "SecurePassword123!", "127.0.0.1", "Client/1.0")
    if err != nil {
        log.Fatalf("Login failed: %v", err)
    }
    
    fmt.Printf("Access Token: %s\n", loginResponse.AccessToken)
    
    // Validate token
    validatedUser, err := authService.ValidateToken(ctx, loginResponse.AccessToken)
    if err != nil {
        log.Fatalf("Token validation failed: %v", err)
    }
    
    fmt.Printf("Validated User: %s\n", validatedUser.Email)
}
```

### Middleware Usage

```go
package main

import (
    "net/http"
    "github.com/gorilla/mux"
    
    "github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
)

func main() {
    authMiddleware := middleware.NewAuthMiddleware(authService, logger)
    authChain := middleware.NewAuthMiddlewareChain(authMiddleware)
    
    router := mux.NewRouter()
    
    // Public endpoints
    publicRouter := router.PathPrefix("/api/v1/public").Subrouter()
    publicRouter.Use(authChain.ForPublicEndpoints())
    
    // Protected endpoints
    protectedRouter := router.PathPrefix("/api/v1/protected").Subrouter()
    protectedRouter.Use(authChain.ForProtectedEndpoints())
    
    // Admin endpoints
    adminRouter := router.PathPrefix("/api/v1/admin").Subrouter()
    adminRouter.Use(authChain.ForAdminEndpoints())
    
    // Service endpoints
    serviceRouter := router.PathPrefix("/api/v1/service").Subrouter()
    serviceRouter.Use(authChain.ForServiceEndpoints())
    
    // Start server
    http.ListenAndServe(":8080", router)
}
```

### Permission Check Example

```go
func checkUserPermission(authService *infrastructure.AuthenticationService, userID uuid.UUID) {
    ctx := context.Background()
    
    // Check if user can read reviews
    canRead, err := authService.CheckPermission(ctx, userID, "reviews", "read")
    if err != nil {
        log.Printf("Permission check failed: %v", err)
        return
    }
    
    if canRead {
        fmt.Println("User can read reviews")
    } else {
        fmt.Println("User cannot read reviews")
    }
}
```

## Security Considerations

### Password Security

- **Bcrypt hashing** with configurable cost (default: 12)
- **Password strength validation** with configurable requirements
- **Password history** (optional feature)
- **Secure password generation** for resets

### Token Security

- **Short-lived access tokens** (15 minutes default)
- **Secure refresh tokens** (7 days default)
- **Token blacklisting** on logout
- **Automatic token cleanup**

### Rate Limiting

- **Login attempt limiting** (5 attempts per 15 minutes)
- **IP-based rate limiting**
- **Account lockout** after failed attempts
- **Configurable limits and windows**

### API Key Security

- **Secure key generation** with cryptographic randomness
- **Key hashing** for storage
- **Scope-based permissions**
- **Usage tracking and rate limiting**

### Network Security

- **HTTPS enforcement** (recommended)
- **CORS configuration**
- **Trusted proxy support**
- **IP whitelisting** (optional)

## Monitoring and Logging

### Audit Logging

All authentication events are logged with:

- **User information** (ID, email, IP address)
- **Action details** (login, logout, permission check)
- **Result status** (success/failure)
- **Timestamp and metadata**

### Metrics

The system provides metrics for:

- **Authentication attempts** (success/failure rates)
- **Token generation and validation** counts
- **API key usage** statistics
- **Permission check** performance
- **Rate limiting** triggers

### Health Checks

- **Service health** endpoint
- **Database connectivity** check
- **JWT service** status
- **Rate limiter** health

### Example Monitoring Setup

```go
// Prometheus metrics
var (
    loginAttempts = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "auth_login_attempts_total",
            Help: "Total number of login attempts",
        },
        []string{"status"},
    )
    
    tokenValidations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "auth_token_validations_total",
            Help: "Total number of token validations",
        },
        []string{"status"},
    )
)

// Register metrics
prometheus.MustRegister(loginAttempts, tokenValidations)
```

## Troubleshooting

### Common Issues

1. **JWT Secret Configuration**
   - Ensure JWT secret is at least 32 characters
   - Use environment variables in production
   - Rotate secrets regularly

2. **Database Migrations**
   - Run migrations before starting the service
   - Check database connectivity
   - Verify schema matches expected structure

3. **Rate Limiting**
   - Adjust limits based on usage patterns
   - Monitor rate limit triggers
   - Implement proper error handling

4. **Permission Errors**
   - Verify role and permission assignments
   - Check RBAC configuration
   - Validate permission names match resources

### Debug Mode

Enable debug logging for troubleshooting:

```yaml
log:
  level: debug
  enable_caller: true
  enable_stacktrace: true
```

### Performance Tuning

1. **Token Expiry**
   - Adjust token expiry based on security requirements
   - Balance security vs. user experience

2. **Database Optimization**
   - Add indexes for frequent queries
   - Optimize query patterns
   - Consider caching for permission checks

3. **Rate Limiting**
   - Tune limits based on traffic patterns
   - Use distributed rate limiting for scaling
   - Monitor and adjust thresholds

This comprehensive authentication system provides enterprise-grade security with extensive configurability and monitoring capabilities, making it suitable for production use in the hotel reviews microservice.