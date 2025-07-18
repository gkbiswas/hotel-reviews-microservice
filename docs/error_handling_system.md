# Comprehensive Error Handling System

This document describes the comprehensive error handling system implemented for the hotel reviews microservice. The system provides centralized error handling, classification, monitoring, alerting, and recovery capabilities.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Features](#features)
4. [Components](#components)
5. [Usage Guide](#usage-guide)
6. [Configuration](#configuration)
7. [Error Types and Classification](#error-types-and-classification)
8. [Monitoring and Alerting](#monitoring-and-alerting)
9. [Integration](#integration)
10. [Best Practices](#best-practices)
11. [Examples](#examples)
12. [API Reference](#api-reference)

## Overview

The error handling system provides a comprehensive solution for managing errors in distributed systems. It includes:

- **Centralized Error Handling**: All errors are processed through a single handler
- **Error Classification**: Automatic categorization of errors by type, severity, and category
- **Correlation IDs**: Request tracing across services
- **Structured Responses**: Consistent error response formatting
- **Metrics Collection**: Real-time error monitoring and analysis
- **Alerting**: Configurable alerts for critical errors
- **Rate Limiting**: Error-based rate limiting to prevent abuse
- **Health Monitoring**: System health based on error patterns
- **Recovery Mechanisms**: Integration with circuit breakers and retry logic

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │    │   Application   │    │   Database      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Error Handler Middleware                     │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Error Handler Core                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Classifier    │  │   Correlator    │  │   Formatter     │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                 │
                ┌────────────────┼────────────────┐
                │                │                │
                ▼                ▼                ▼
    ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
    │     Metrics     │  │    Alerting     │  │  Health Check   │
    │   Collection    │  │   Integration   │  │   Integration   │
    └─────────────────┘  └─────────────────┘  └─────────────────┘
                │                │                │
                ▼                ▼                ▼
    ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
    │   Prometheus    │  │ Alert Channels  │  │  Health Status  │
    │    Metrics      │  │ (Email, Slack)  │  │   Dashboard     │
    └─────────────────┘  └─────────────────┘  └─────────────────┘
```

## Features

### Core Features

- **Error Classification**: Automatic categorization of errors
- **Request Tracing**: Correlation IDs for tracking requests across services
- **Structured Responses**: Consistent error response formatting
- **Stack Trace Management**: Configurable stack trace inclusion
- **Context Preservation**: Error context and metadata preservation

### Monitoring and Observability

- **Metrics Collection**: Real-time error metrics with Prometheus integration
- **Error Aggregation**: Pattern detection and error clustering
- **Health Monitoring**: System health assessment based on error patterns
- **Trend Analysis**: Error trend detection and forecasting

### Alerting and Notification

- **Multi-Channel Alerting**: Email, Slack, webhook, and console notifications
- **Alert Conditions**: Configurable alert rules and thresholds
- **Alert Suppression**: Cooldown and rate limiting for alerts
- **Alert Correlation**: Related alert grouping and deduplication

### Recovery and Resilience

- **Circuit Breaker Integration**: Automatic service protection
- **Retry Logic**: Configurable retry policies
- **Rate Limiting**: Error-based rate limiting
- **Fallback Mechanisms**: Graceful degradation options

## Components

### 1. Error Handler Core

The main error handling component that processes all errors.

**File**: `internal/infrastructure/error_handler.go`

**Key Classes**:
- `ErrorHandler`: Main error handling orchestrator
- `AppError`: Structured error representation
- `ErrorPattern`: Error classification patterns

### 2. Error Metrics Collector

Collects and reports error metrics to monitoring systems.

**File**: `internal/infrastructure/error_metrics.go`

**Features**:
- Prometheus metrics integration
- Real-time error counting
- Error rate calculation
- Performance metrics

### 3. Error Aggregator

Aggregates errors for pattern detection and analysis.

**File**: `internal/infrastructure/error_aggregator.go`

**Features**:
- Error signature generation
- Pattern clustering
- Trend analysis
- Historical data aggregation

### 4. Error Alerter

Manages alert conditions and notifications.

**File**: `internal/infrastructure/error_alerter.go`

**Features**:
- Alert condition management
- Multi-channel notifications
- Alert suppression
- Alert correlation

### 5. Error Rate Limiter

Implements error-based rate limiting.

**File**: `internal/infrastructure/error_rate_limiter.go`

**Features**:
- Multiple rate limiting strategies
- User-based and IP-based limiting
- Configurable thresholds
- Burst protection

### 6. Health Checker

Monitors system health based on error patterns.

**File**: `internal/infrastructure/error_health_checker.go`

**Features**:
- Health status assessment
- Condition monitoring
- Health metrics
- Status history

### 7. Alert Channels

Various notification channels for alerts.

**File**: `internal/infrastructure/alert_channels.go`

**Channels**:
- Email notifications
- Slack integration
- Webhook notifications
- Console output
- Logger integration

### 8. Middleware

HTTP middleware for error handling integration.

**File**: `internal/infrastructure/middleware/error_handler_middleware.go`

**Features**:
- HTTP error interception
- Panic recovery
- Response formatting
- Context preservation

## Usage Guide

### Basic Setup

```go
package main

import (
    "github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
    "github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

func main() {
    // Initialize logger
    logger := logger.NewDefault()
    
    // Create error handler configuration
    config := infrastructure.DefaultErrorHandlerConfig()
    
    // Create error handler
    errorHandler := infrastructure.NewErrorHandler(config, logger, nil, nil)
    defer errorHandler.Close()
    
    // Use error handler
    ctx := context.Background()
    err := errors.New("something went wrong")
    appError := errorHandler.Handle(ctx, err)
    
    fmt.Printf("Error ID: %s\n", appError.ID)
    fmt.Printf("Error Type: %s\n", appError.Type)
}
```

### HTTP Integration

```go
package main

import (
    "net/http"
    "github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
    "github.com/gorilla/mux"
)

func main() {
    // Create error handler
    errorHandler := infrastructure.NewErrorHandler(config, logger, nil, nil)
    
    // Create middleware
    errorMiddleware := middleware.NewErrorHandlerMiddleware(errorHandler, logger)
    
    // Create router
    router := mux.NewRouter()
    router.Use(errorMiddleware.Handle)
    
    // Add routes
    router.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
        // Any error here will be handled by middleware
        return errors.New("test error")
    })
    
    http.ListenAndServe(":8080", router)
}
```

### Business Logic Integration

```go
package service

import (
    "context"
    "github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
)

type UserService struct {
    errorService *middleware.ErrorHandlerService
}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    // Database operation
    user, err := s.db.GetUser(userID)
    if err != nil {
        return nil, s.errorService.HandleDatabaseError(ctx, err, "get_user")
    }
    
    // Validation
    if err := s.validateUser(user); err != nil {
        return nil, s.errorService.HandleValidationError(ctx, err, "user_validation")
    }
    
    // External service call
    profile, err := s.externalService.GetProfile(userID)
    if err != nil {
        return nil, s.errorService.HandleExternalServiceError(ctx, err, "profile_service")
    }
    
    return user, nil
}
```

## Configuration

### Error Handler Configuration

```go
type ErrorHandlerConfig struct {
    EnableMetrics         bool          `json:"enable_metrics"`
    EnableAlerting        bool          `json:"enable_alerting"`
    EnableStackTrace      bool          `json:"enable_stack_trace"`
    EnableDetailedLogging bool          `json:"enable_detailed_logging"`
    EnableErrorAggregation bool         `json:"enable_error_aggregation"`
    EnableRateLimiting    bool          `json:"enable_rate_limiting"`
    MaxStackTraceDepth    int           `json:"max_stack_trace_depth"`
    ErrorRetentionPeriod  time.Duration `json:"error_retention_period"`
    MetricsInterval       time.Duration `json:"metrics_interval"`
    AlertingThreshold     int           `json:"alerting_threshold"`
    AlertingWindow        time.Duration `json:"alerting_window"`
    RateLimitWindow       time.Duration `json:"rate_limit_window"`
    RateLimitThreshold    int           `json:"rate_limit_threshold"`
    DefaultFormat         ErrorFormat   `json:"default_format"`
    IncludeInternalErrors bool          `json:"include_internal_errors"`
    SanitizeUserData      bool          `json:"sanitize_user_data"`
}
```

### Alert Channel Configuration

```go
// Email configuration
emailConfig := &infrastructure.EmailConfig{
    Host:     "smtp.gmail.com",
    Port:     587,
    Username: "your-email@gmail.com",
    Password: "your-password",
    From:     "your-email@gmail.com",
    To:       []string{"alerts@company.com"},
}

// Slack configuration
slackConfig := &infrastructure.SlackConfig{
    WebhookURL: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
    Channel:    "#alerts",
    Username:   "Error Handler",
    Timeout:    30 * time.Second,
}

// Webhook configuration
webhookConfig := &infrastructure.WebhookConfig{
    URL:     "https://your-webhook-url.com/alerts",
    Method:  "POST",
    Headers: map[string]string{
        "Authorization": "Bearer your-token",
    },
    Timeout: 30 * time.Second,
}
```

## Error Types and Classification

### Error Types

```go
const (
    // System errors
    ErrorTypeSystem           ErrorType = "system"
    ErrorTypeDatabase         ErrorType = "database"
    ErrorTypeNetwork          ErrorType = "network"
    ErrorTypeTimeout          ErrorType = "timeout"
    ErrorTypeCircuitBreaker   ErrorType = "circuit_breaker"
    ErrorTypeRateLimit        ErrorType = "rate_limit"
    ErrorTypeResource         ErrorType = "resource"
    ErrorTypeConfiguration    ErrorType = "configuration"

    // Business errors
    ErrorTypeBusiness         ErrorType = "business"
    ErrorTypeValidation       ErrorType = "validation"
    ErrorTypeAuthentication   ErrorType = "authentication"
    ErrorTypeAuthorization    ErrorType = "authorization"
    ErrorTypeNotFound         ErrorType = "not_found"
    ErrorTypeConflict         ErrorType = "conflict"
    ErrorTypePrecondition     ErrorType = "precondition"

    // External errors
    ErrorTypeExternal         ErrorType = "external"
    ErrorTypeThirdParty       ErrorType = "third_party"
    ErrorTypeUpstream         ErrorType = "upstream"
    ErrorTypeDownstream       ErrorType = "downstream"

    // Client errors
    ErrorTypeClient           ErrorType = "client"
    ErrorTypeMalformedRequest ErrorType = "malformed_request"
    ErrorTypeUnsupported      ErrorType = "unsupported"
)
```

### Error Categories

```go
const (
    CategoryTransient    ErrorCategory = "transient"    // Temporary errors
    CategoryPermanent    ErrorCategory = "permanent"    // Permanent errors
    CategoryRetryable    ErrorCategory = "retryable"    // Retryable errors
    CategoryNonRetryable ErrorCategory = "non_retryable" // Non-retryable errors
    CategoryCritical     ErrorCategory = "critical"     // Critical errors
    CategoryWarning      ErrorCategory = "warning"      // Warning errors
    CategoryInfo         ErrorCategory = "info"         // Info errors
)
```

### Error Severity

```go
const (
    SeverityLow      ErrorSeverity = "low"
    SeverityMedium   ErrorSeverity = "medium"
    SeverityHigh     ErrorSeverity = "high"
    SeverityCritical ErrorSeverity = "critical"
)
```

### HTTP Status Mapping

| Error Type | HTTP Status | Description |
|------------|-------------|-------------|
| ErrorTypeValidation | 400 | Bad Request |
| ErrorTypeAuthentication | 401 | Unauthorized |
| ErrorTypeAuthorization | 403 | Forbidden |
| ErrorTypeNotFound | 404 | Not Found |
| ErrorTypeConflict | 409 | Conflict |
| ErrorTypeRateLimit | 429 | Too Many Requests |
| ErrorTypeDatabase | 500 | Internal Server Error |
| ErrorTypeSystem | 500 | Internal Server Error |
| ErrorTypeNetwork | 502 | Bad Gateway |
| ErrorTypeCircuitBreaker | 503 | Service Unavailable |
| ErrorTypeTimeout | 504 | Gateway Timeout |

## Monitoring and Alerting

### Metrics

The system exports the following Prometheus metrics:

```
# Total errors by type and severity
error_handler_errors_total{type="database",severity="high",category="transient",retryable="true"}

# Error processing duration
error_handler_processing_duration_seconds{type="database",severity="high"}

# Current error count by severity
error_handler_severity_count{severity="high"}

# Active errors by type
error_handler_active_errors{type="database"}

# Error rate (errors per minute)
error_handler_error_rate{type="database"}

# Error patterns detected
error_handler_patterns_total{pattern="CONNECTION_FAILED",type="database"}
```

### Alert Conditions

Default alert conditions include:

1. **High Error Rate**: Triggers when error rate exceeds 10 errors/minute
2. **Critical Error Spike**: Triggers when 5+ critical errors occur within 2 minutes
3. **New Error Pattern**: Triggers when a new error pattern is detected
4. **System Degradation**: Triggers when error trends show degradation

### Health Checks

The system provides health checks based on error patterns:

```go
// Health check endpoint
func healthCheck(errorHandler *infrastructure.ErrorHandler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if errorHandler.IsHealthy() {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("OK"))
        } else {
            w.WriteHeader(http.StatusServiceUnavailable)
            w.Write([]byte("DEGRADED"))
        }
    }
}
```

## Integration

### Circuit Breaker Integration

```go
// Create circuit breaker
circuitBreaker := infrastructure.NewDatabaseCircuitBreaker(logger)

// Create error handler with circuit breaker
errorHandler := infrastructure.NewErrorHandler(config, logger, circuitBreaker, nil)

// Use with circuit breaker
result, err := circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
    return performDatabaseOperation(ctx)
})

if err != nil {
    appError := errorHandler.Handle(ctx, err)
    // Handle circuit breaker errors
}
```

### Retry Integration

```go
// Create retry configuration
retryConfig := &infrastructure.RetryConfig{
    MaxRetries:       3,
    InitialDelay:     time.Second,
    MaxDelay:         10 * time.Second,
    BackoffFactor:    2.0,
    EnableJitter:     true,
    RetryableErrors:  []infrastructure.ErrorType{
        infrastructure.ErrorTypeNetwork,
        infrastructure.ErrorTypeTimeout,
    },
}

// Create error handler with retry config
errorHandler := infrastructure.NewErrorHandler(config, logger, nil, retryConfig)
```

### Distributed Tracing

```go
// Add trace context to errors
ctx = logger.WithTraceID(ctx, "trace-123")
ctx = logger.WithSpanID(ctx, "span-456")

// Error will include trace information
err := errors.New("database error")
appError := errorHandler.Handle(ctx, err)

// Trace ID will be included in error response
fmt.Printf("Trace ID: %s\n", appError.Context["trace_id"])
```

## Best Practices

### 1. Error Handling

- Always handle errors at service boundaries
- Use appropriate error types for different scenarios
- Include relevant context in error details
- Don't expose internal errors to external users

### 2. Logging

- Log errors at appropriate levels
- Include correlation IDs for request tracing
- Use structured logging for better analysis
- Avoid logging sensitive information

### 3. Monitoring

- Monitor error rates and patterns
- Set up appropriate alerts for critical errors
- Review error trends regularly
- Use health checks for service monitoring

### 4. Recovery

- Implement circuit breakers for external dependencies
- Use retry logic for transient errors
- Provide fallback mechanisms where possible
- Consider graceful degradation strategies

### 5. Testing

- Test error handling paths
- Verify error responses and status codes
- Test alert conditions and notifications
- Validate metrics collection

## Examples

### Complete Example

See `examples/error_handler_usage.go` for a complete example demonstrating:

- Error handler setup
- HTTP middleware integration
- Business logic error handling
- Alert channel configuration
- Health checks and metrics

### Testing Example

See `internal/infrastructure/error_handler_test.go` for comprehensive tests covering:

- Error classification
- HTTP response formatting
- Error statistics
- Concurrency handling
- Performance benchmarks

## API Reference

### ErrorHandler

```go
type ErrorHandler struct {
    // Methods
    Handle(ctx context.Context, err error) *AppError
    HandleHTTP(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
    AddErrorPattern(pattern ErrorPattern)
    GetErrorStats() map[string]*ErrorMetrics
    IsHealthy() bool
    Close() error
}
```

### AppError

```go
type AppError struct {
    ID             string
    Type           ErrorType
    Category       ErrorCategory
    Severity       ErrorSeverity
    Code           string
    Message        string
    UserMessage    string
    Details        map[string]interface{}
    Context        map[string]interface{}
    Timestamp      time.Time
    CorrelationID  string
    RequestID      string
    HTTPStatus     int
    Retryable      bool
    
    // Methods
    Error() string
    Unwrap() error
    IsRetryable() bool
    IsCritical() bool
    IsTemporary() bool
    ToJSON() ([]byte, error)
    ToXML() ([]byte, error)
}
```

### ErrorHandlerService

```go
type ErrorHandlerService struct {
    // Methods
    WrapError(ctx context.Context, err error, message string, details map[string]interface{}) error
    HandleServiceError(ctx context.Context, err error, service string) error
    HandleDatabaseError(ctx context.Context, err error, operation string) error
    HandleValidationError(ctx context.Context, err error, field string) error
    HandleExternalServiceError(ctx context.Context, err error, service string) error
    CreateBusinessError(ctx context.Context, code, message, userMessage string, details map[string]interface{}) error
    IsRetryableError(err error) bool
    IsCriticalError(err error) bool
    IsTemporaryError(err error) bool
}
```

## Conclusion

This comprehensive error handling system provides a robust foundation for managing errors in distributed systems. It includes all the essential features needed for production systems:

- Centralized error handling and classification
- Request correlation and tracing
- Structured error responses
- Real-time monitoring and alerting
- Health checks and metrics
- Integration with resilience patterns

The system is designed to be extensible and configurable, allowing teams to adapt it to their specific needs while maintaining consistency and reliability across services.

For more examples and detailed usage instructions, refer to the examples directory and test files.