# ADR-006: API Design Principles

* Status: Accepted
* Date: 2024-01-15
* Authors: Development Team
* Deciders: Technical Leadership

## Context

The hotel reviews microservice provides a RESTful HTTP API that needs to:
- Follow consistent design patterns for developer experience
- Support various client types (web apps, mobile apps, third-party integrations)
- Handle different authentication and authorization scenarios
- Provide clear error responses and status codes
- Support pagination, filtering, and sorting for large datasets
- Maintain backward compatibility as the API evolves
- Follow industry standards and best practices

We need to establish clear API design principles to ensure:
- Consistency across all endpoints
- Intuitive developer experience
- Proper HTTP semantics
- Scalable request/response patterns
- Security best practices

## Decision

We will follow **RESTful API design principles** with these specific guidelines:

### 1. Resource-Based URLs
```
✅ Good: GET /api/v1/reviews/123
❌ Bad:  GET /api/v1/getReview?id=123

✅ Good: POST /api/v1/hotels/456/reviews
❌ Bad:  POST /api/v1/createReviewForHotel
```

### 2. HTTP Methods and Status Codes
```
GET    /reviews       → 200 OK (list)
GET    /reviews/123   → 200 OK (item) | 404 Not Found
POST   /reviews       → 201 Created | 400 Bad Request | 422 Unprocessable Entity
PUT    /reviews/123   → 200 OK | 404 Not Found | 400 Bad Request
PATCH  /reviews/123   → 200 OK | 404 Not Found | 400 Bad Request  
DELETE /reviews/123   → 204 No Content | 404 Not Found
```

### 3. Consistent Response Format
```json
{
  "data": { ... },           // Single resource
  "meta": {                  // Metadata
    "pagination": { ... },
    "total_count": 150
  },
  "links": {                 // Navigation links
    "self": "/api/v1/reviews?page=2",
    "next": "/api/v1/reviews?page=3",
    "prev": "/api/v1/reviews?page=1"
  }
}
```

### 4. Error Response Format
```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Request validation failed",
    "details": [
      {
        "field": "rating",
        "code": "INVALID_RANGE", 
        "message": "Rating must be between 1.0 and 5.0"
      }
    ]
  },
  "meta": {
    "request_id": "req-abc123",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

## Rationale

**Benefits of RESTful Design:**
- **Predictable**: Developers can predict endpoint behavior
- **Cacheable**: GET requests can be cached by HTTP infrastructure
- **Stateless**: Each request contains all necessary information
- **Scalable**: Horizontal scaling is straightforward
- **Tool-Friendly**: Standard HTTP tools work out of the box

**Consistency Benefits:**
- **Developer Experience**: Reduced learning curve for API consumers
- **Maintenance**: Easier to maintain and extend consistent patterns
- **Testing**: Standard patterns enable comprehensive test coverage
- **Documentation**: Auto-generated docs work better with consistent patterns

**Industry Standards:**
- Follows JSON:API and Google API Design Guide principles
- Compatible with OpenAPI/Swagger specifications
- Supports standard HTTP caching and security headers

## Consequences

### Positive
- Intuitive API for developers familiar with REST principles
- Excellent tooling support (Postman, curl, HTTP libraries)
- Built-in HTTP caching and security capabilities
- Easy to document and test
- Compatible with API gateways and proxy tools
- Supports various client types and platforms

### Negative
- Some operations don't map naturally to CRUD operations
- Multiple round trips may be needed for complex operations
- Over-fetching or under-fetching data in some scenarios
- REST constraints may limit some optimization opportunities

### Neutral
- Team needs training on REST best practices
- API versioning strategy required for evolution
- Documentation standards must be maintained

## Implementation

### 1. URL Structure and Versioning
```
Base URL: https://api.hotel-reviews.com
Version:  /api/v1/

Resources:
/api/v1/reviews                    # Review collection
/api/v1/reviews/{id}               # Individual review
/api/v1/hotels                     # Hotel collection  
/api/v1/hotels/{id}                # Individual hotel
/api/v1/hotels/{id}/reviews        # Hotel's reviews
/api/v1/providers                  # Provider collection
/api/v1/providers/{id}/reviews     # Provider's reviews
```

### 2. Request/Response Headers
```http
# Standard headers for all requests
Content-Type: application/json
Accept: application/json
Authorization: Bearer {token}
X-Request-ID: {uuid}

# CORS headers
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE
Access-Control-Allow-Headers: Content-Type, Authorization

# Cache headers for GET requests
Cache-Control: public, max-age=300
ETag: "abc123"
Last-Modified: Wed, 15 Jan 2024 10:30:00 GMT
```

### 3. Pagination Pattern
```http
GET /api/v1/reviews?page=2&limit=20&sort=created_at:desc

Response:
{
  "data": [...],
  "meta": {
    "pagination": {
      "current_page": 2,
      "per_page": 20,
      "total_pages": 15,
      "total_count": 290,
      "has_next_page": true,
      "has_prev_page": true
    }
  },
  "links": {
    "first": "/api/v1/reviews?page=1&limit=20",
    "prev": "/api/v1/reviews?page=1&limit=20", 
    "self": "/api/v1/reviews?page=2&limit=20",
    "next": "/api/v1/reviews?page=3&limit=20",
    "last": "/api/v1/reviews?page=15&limit=20"
  }
}
```

### 4. Filtering and Sorting
```http
# Filtering
GET /api/v1/reviews?hotel_id=123&rating_min=4.0&created_after=2024-01-01

# Sorting  
GET /api/v1/reviews?sort=rating:desc,created_at:asc

# Field selection
GET /api/v1/reviews?fields=id,rating,comment,created_at

# Search
GET /api/v1/reviews?q=excellent+service
```

### 5. HTTP Status Code Usage
```go
// Success responses
200 OK          - Successful GET, PUT, PATCH
201 Created     - Successful POST with resource creation
204 No Content  - Successful DELETE

// Client error responses  
400 Bad Request      - Invalid request format
401 Unauthorized     - Authentication required
403 Forbidden        - Authorization failed
404 Not Found        - Resource doesn't exist
409 Conflict         - Resource conflict (duplicate)
422 Unprocessable    - Validation failed
429 Too Many Requests - Rate limit exceeded

// Server error responses
500 Internal Server Error - Unexpected server error
503 Service Unavailable   - Temporary service issues
```

### 6. Request Validation
```go
type CreateReviewRequest struct {
    HotelID    uuid.UUID `json:"hotel_id" validate:"required"`
    ProviderID uuid.UUID `json:"provider_id" validate:"required"`
    Rating     float64   `json:"rating" validate:"required,min=1.0,max=5.0"`
    Comment    string    `json:"comment" validate:"required,min=10,max=1000"`
    ReviewDate time.Time `json:"review_date" validate:"required"`
}

func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
    var req CreateReviewRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Request body must be valid JSON")
        return
    }
    
    if err := h.validator.Struct(req); err != nil {
        h.writeValidationError(w, err)
        return
    }
    
    // Process valid request...
}
```

### 7. Content Negotiation
```go
func (h *Handler) GetReview(w http.ResponseWriter, r *http.Request) {
    // Support multiple response formats
    accept := r.Header.Get("Accept")
    
    review, err := h.service.GetReview(r.Context(), reviewID)
    if err != nil {
        h.writeError(w, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found")
        return
    }
    
    switch accept {
    case "application/json":
        h.writeJSON(w, http.StatusOK, review)
    case "application/xml":
        h.writeXML(w, http.StatusOK, review)
    default:
        h.writeJSON(w, http.StatusOK, review) // Default to JSON
    }
}
```

### 8. API Documentation
```go
// OpenAPI/Swagger annotations
// @Summary Create a new review
// @Description Create a new hotel review with rating and comments
// @Tags reviews
// @Accept json
// @Produce json
// @Param request body CreateReviewRequest true "Review data"
// @Success 201 {object} ReviewResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Router /reviews [post]
func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
    // Implementation...
}
```

### 9. Rate Limiting
```http
# Rate limit headers
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1642261800

# When rate limit exceeded (429 Too Many Requests)
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "API rate limit exceeded",
    "details": {
      "limit": 1000,
      "window": "1h",
      "reset_at": "2024-01-15T11:30:00Z"
    }
  }
}
```

### 10. API Versioning Strategy
```
# URL versioning (current approach)
GET /api/v1/reviews
GET /api/v2/reviews

# Header versioning (future consideration)
GET /api/reviews
Accept: application/vnd.hotel-reviews.v1+json

# Deprecation headers
Sunset: Wed, 15 Jan 2025 10:30:00 GMT
Deprecation: true
Link: </api/v2/reviews>; rel="successor-version"
```

## Related Decisions

- [ADR-003](003-error-handling-strategy.md): Error response format
- [ADR-007](007-authentication-authorization.md): API authentication
- [ADR-010](010-monitoring-logging.md): API request logging and metrics

## Notes

This API design follows these key principles:

1. **REST Maturity Level 2**: Proper use of HTTP verbs and status codes
2. **Richardson Maturity Model**: Resource-based URLs with HTTP verbs
3. **Google API Design Guide**: Consistent patterns and naming
4. **JSON:API**: Response structure and error handling patterns

**Implementation Guidelines:**
- Use middleware for cross-cutting concerns (auth, logging, CORS)
- Implement request/response validation at API boundary
- Provide comprehensive error messages with actionable details
- Support both development and production-friendly error formats
- Include proper HTTP headers for caching and security

**Evolution Strategy:**
- Maintain backward compatibility within major versions
- Use feature flags for gradual rollouts of new functionality
- Provide clear migration guides when deprecating endpoints
- Monitor API usage to inform versioning decisions

The API design prioritizes developer experience while maintaining performance and security requirements for production usage.