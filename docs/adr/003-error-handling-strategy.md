# ADR-003: Error Handling Strategy

* Status: Accepted
* Date: 2024-01-15
* Authors: Development Team
* Deciders: Technical Leadership

## Context

The hotel reviews microservice needs a consistent error handling strategy that:
- Provides meaningful error messages to clients
- Enables proper logging and monitoring
- Maintains security by not exposing internal details
- Supports different error types (validation, business logic, infrastructure)
- Integrates well with HTTP responses and structured logging

We need to handle errors consistently across:
- HTTP API endpoints
- Business logic validation
- Database operations
- External service integrations
- Authentication and authorization

## Decision

We will adopt a **structured error handling approach** with:
1. **Custom error types** for different categories
2. **Error wrapping** to maintain context
3. **Centralized error handling** in HTTP middleware
4. **Structured logging** with error details
5. **Client-safe error responses** that don't expose internals

### Error Type Hierarchy:
```go
// Base error interface
type AppError interface {
    error
    Code() string
    HTTPStatus() int
    Details() map[string]interface{}
    LogLevel() string
}

// Specific error types
type ValidationError struct { ... }
type BusinessLogicError struct { ... }
type InfrastructureError struct { ... }
type AuthenticationError struct { ... }
```

### Error Response Format:
```json
{
  "error": "VALIDATION_FAILED",
  "message": "Invalid request data",
  "details": {
    "field": "email",
    "reason": "invalid format"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req-123"
}
```

## Rationale

**Benefits of Structured Errors:**
- **Consistent API**: All endpoints return errors in the same format
- **Client-Friendly**: Error codes allow programmatic error handling
- **Security**: Internal details not exposed to clients
- **Debugging**: Full context preserved in logs
- **Monitoring**: Error codes enable alerting and metrics

**Alternatives Considered:**
- **Simple string errors**: Rejected due to lack of structure and context
- **HTTP status codes only**: Rejected due to limited expressiveness
- **Exception-based**: Go doesn't have exceptions, errors are values

**Trade-offs:**
- More complex error handling code
- Need to define error types for different scenarios
- Requires discipline to use consistently

## Consequences

### Positive
- Consistent error handling across the application
- Client applications can handle errors programmatically
- Better debugging with structured error context
- Security through controlled error exposure
- Improved monitoring and alerting capabilities
- Clear separation between user and system errors

### Negative
- More verbose error handling code
- Need to maintain error type definitions
- Developers must understand error hierarchy
- Additional testing required for error scenarios

### Neutral
- Error handling patterns must be documented
- Team training required on error handling standards
- Code reviews must enforce error handling consistency

## Implementation

### Error Type Definitions:
```go
// Validation errors (400 Bad Request)
type ValidationError struct {
    Field   string
    Value   interface{}
    Message string
}

// Business logic errors (422 Unprocessable Entity)
type BusinessLogicError struct {
    Operation string
    Reason    string
    Context   map[string]interface{}
}

// Infrastructure errors (500 Internal Server Error)
type InfrastructureError struct {
    Component string
    Operation string
    Cause     error
}

// Authentication errors (401 Unauthorized)
type AuthenticationError struct {
    Reason string
}
```

### Error Handling Middleware:
```go
func ErrorHandlingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // Handle panics
                handlePanic(w, r, err)
            }
        }()
        
        // Use response writer wrapper to capture errors
        wrapper := &responseWriter{ResponseWriter: w}
        next.ServeHTTP(wrapper, r)
        
        if wrapper.error != nil {
            handleError(w, r, wrapper.error)
        }
    })
}
```

### Error Wrapping Pattern:
```go
// In infrastructure layer
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Review, error) {
    row := r.db.QueryRowContext(ctx, query, id)
    if err := row.Scan(&review); err != nil {
        if err == sql.ErrNoRows {
            return nil, NewNotFoundError("review", id.String())
        }
        return nil, fmt.Errorf("failed to query review: %w", err)
    }
    return review, nil
}

// In application layer
func (s *Service) GetReview(ctx context.Context, id uuid.UUID) (*Review, error) {
    review, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get review operation failed: %w", err)
    }
    return review, nil
}
```

### Logging Integration:
```go
func handleError(w http.ResponseWriter, r *http.Request, err error) {
    var appErr AppError
    if errors.As(err, &appErr) {
        logError(r.Context(), appErr)
        writeErrorResponse(w, appErr)
    } else {
        // Unknown error - log full details, return generic message
        logger.Error("Unexpected error", "error", err, "path", r.URL.Path)
        writeGenericErrorResponse(w)
    }
}
```

### Error Testing Strategy:
```go
func TestService_GetReview_NotFound(t *testing.T) {
    mockRepo := &MockRepository{}
    mockRepo.On("GetByID", mock.Anything, testID).
        Return(nil, NewNotFoundError("review", testID.String()))
    
    service := NewService(mockRepo)
    _, err := service.GetReview(context.Background(), testID)
    
    assert.Error(t, err)
    var notFoundErr *NotFoundError
    assert.True(t, errors.As(err, &notFoundErr))
}
```

## Related Decisions

- [ADR-001](001-clean-architecture-adoption.md): Architecture affects error flow
- [ADR-005](005-logging-and-monitoring.md): Error logging integration
- [ADR-006](006-api-design-principles.md): HTTP error response format

## Notes

This error handling strategy is influenced by:
- Go's idiomatic error handling patterns
- "Effective Go" error handling guidelines
- REST API best practices for error responses
- Security considerations for error information disclosure

The implementation prioritizes:
1. **Developer Experience**: Clear error messages and context
2. **Client Experience**: Consistent, actionable error responses  
3. **Operations**: Structured logging for monitoring and debugging
4. **Security**: Controlled information disclosure

Error codes follow a hierarchical naming convention:
- `VALIDATION_*`: Input validation errors
- `BUSINESS_*`: Business rule violations
- `AUTH_*`: Authentication/authorization errors
- `SYSTEM_*`: Infrastructure/system errors