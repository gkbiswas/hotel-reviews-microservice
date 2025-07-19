# Hotel Reviews API Documentation

A comprehensive REST API for managing hotel reviews, ratings, and related data with enterprise-grade features including authentication, rate limiting, and monitoring.

## 📋 Table of Contents

- [Quick Start](#quick-start)
- [Authentication](#authentication)
- [API Versioning](#api-versioning)
- [Rate Limiting](#rate-limiting)
- [Error Handling](#error-handling)
- [Pagination](#pagination)
- [Search and Filtering](#search-and-filtering)
- [File Upload](#file-upload)
- [Monitoring and Health](#monitoring-and-health)
- [SDK and Examples](#sdk-and-examples)
- [Testing](#testing)

## 🚀 Quick Start

### Base URLs

| Environment | URL |
|-------------|-----|
| Production | `https://api.hotelreviews.com/api/v1` |
| Staging | `https://staging-api.hotelreviews.com/api/v1` |
| Development | `http://localhost:8080/api/v1` |

### Authentication

1. **Register a new user:**
```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "johndoe",
    "email": "john.doe@example.com",
    "password": "SecurePassword123!",
    "first_name": "John",
    "last_name": "Doe"
  }'
```

2. **Login to get access token:**
```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "password": "SecurePassword123!"
  }'
```

3. **Use the access token in subsequent requests:**
```bash
curl -X GET https://api.hotelreviews.com/api/v1/reviews \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Basic Operations

#### Create a Review
```bash
curl -X POST https://api.hotelreviews.com/api/v1/reviews \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "hotel_id": "123e4567-e89b-12d3-a456-426614174000",
    "reviewer_info_id": "456e7890-e89b-12d3-a456-426614174001",
    "rating": 4.5,
    "title": "Great stay!",
    "comment": "Had a wonderful time at this hotel.",
    "review_date": "2024-01-10T00:00:00Z",
    "language": "en"
  }'
```

#### Get Reviews with Filtering
```bash
curl -X GET "https://api.hotelreviews.com/api/v1/reviews?hotel_id=123e4567-e89b-12d3-a456-426614174000&limit=20" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

#### Search Reviews
```bash
curl -X GET "https://api.hotelreviews.com/api/v1/reviews/search?query=excellent%20service&rating_min=4" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## 🔐 Authentication

The API uses **JWT Bearer Token** authentication with the following flow:

### Token Lifecycle
- **Access Token**: Valid for 1 hour
- **Refresh Token**: Valid for 30 days
- **Token Refresh**: Use `/auth/refresh` endpoint before expiration

### Security Headers
```
Authorization: Bearer <access_token>
Content-Type: application/json
X-API-Version: v1
```

### Token Management
```javascript
// Automatic token refresh example
async function apiRequest(url, options = {}) {
  let token = localStorage.getItem('access_token');
  
  const response = await fetch(url, {
    ...options,
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
      ...options.headers
    }
  });
  
  if (response.status === 401) {
    // Token expired, refresh it
    const refreshResponse = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        refresh_token: localStorage.getItem('refresh_token')
      })
    });
    
    if (refreshResponse.ok) {
      const data = await refreshResponse.json();
      localStorage.setItem('access_token', data.data.access_token);
      // Retry original request
      return apiRequest(url, options);
    }
  }
  
  return response;
}
```

## 📌 API Versioning

### Versioning Strategy
The API uses **URL path versioning** for clear version management:

- **Current Version**: `v1`
- **URL Pattern**: `/api/{version}/{resource}`
- **Example**: `/api/v1/reviews`

### Version Support Policy
- Minimum support period: 12 months after new version release
- Deprecation warnings provided via response headers
- Migration guides available for version transitions

### Version Headers
```
X-API-Version: v1
X-Deprecated-Version: false
X-Sunset-Date: null
```

### Migration Example
```bash
# Current (v1)
GET /api/v1/reviews

# Future (v2) - when available
GET /api/v2/reviews
```

## ⚡ Rate Limiting

### Default Limits
| User Type | Limit |
|-----------|-------|
| Authenticated | 1,000 requests/hour |
| Unauthenticated | 100 requests/hour |

### Endpoint-Specific Limits
| Endpoint | Limit |
|----------|-------|
| `/auth/login` | 10 requests/minute |
| `/reviews/upload` | 5 requests/hour |
| `/reviews/bulk` | 20 requests/hour |

### Rate Limit Headers
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1642249200
Retry-After: 60
```

### Handling Rate Limits
```python
import requests
import time

def make_request_with_retry(url, headers, max_retries=3):
    for attempt in range(max_retries):
        response = requests.get(url, headers=headers)
        
        if response.status_code == 429:
            retry_after = int(response.headers.get('Retry-After', 60))
            print(f"Rate limited. Waiting {retry_after} seconds...")
            time.sleep(retry_after)
            continue
            
        return response
    
    raise Exception("Max retries exceeded")
```

## 🚨 Error Handling

### Standard Error Response Format
```json
{
  "success": false,
  "error": "Human-readable error message",
  "error_code": "MACHINE_READABLE_CODE",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456",
  "details": {
    "additional": "error context"
  }
}
```

### HTTP Status Codes
| Code | Meaning | Description |
|------|---------|-------------|
| 200 | OK | Request successful |
| 201 | Created | Resource created successfully |
| 400 | Bad Request | Invalid request parameters |
| 401 | Unauthorized | Authentication required |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource not found |
| 409 | Conflict | Resource already exists |
| 422 | Validation Error | Request validation failed |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server error |

### Error Codes Reference
| Error Code | Description | Typical HTTP Status |
|------------|-------------|-------------------|
| `UNAUTHORIZED` | Authentication required | 401 |
| `INVALID_TOKEN` | Invalid or expired token | 401 |
| `FORBIDDEN` | Insufficient permissions | 403 |
| `NOT_FOUND` | Resource not found | 404 |
| `VALIDATION_ERROR` | Request validation failed | 422 |
| `RATE_LIMIT_EXCEEDED` | Rate limit exceeded | 429 |
| `CONFLICT` | Resource already exists | 409 |
| `PAYLOAD_TOO_LARGE` | File size exceeds limit | 413 |

### Validation Error Example
```json
{
  "success": false,
  "error": "Request validation failed",
  "error_code": "VALIDATION_ERROR",
  "details": {
    "validation_errors": [
      {
        "field": "email",
        "message": "Invalid email format",
        "code": "INVALID_FORMAT"
      },
      {
        "field": "rating",
        "message": "Rating must be between 1 and 5",
        "code": "OUT_OF_RANGE"
      }
    ]
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

## 📄 Pagination

### Standard Pagination Parameters
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `limit` | integer | Items per page (1-100) | 20 |
| `offset` | integer | Items to skip | 0 |

### Pagination Response Format
```json
{
  "success": true,
  "data": {
    "reviews": [...],
    "pagination": {
      "total": 156,
      "limit": 20,
      "offset": 0,
      "has_next": true,
      "has_previous": false
    }
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Pagination Examples
```bash
# First page (default)
GET /api/v1/reviews?limit=20&offset=0

# Second page
GET /api/v1/reviews?limit=20&offset=20

# Custom page size
GET /api/v1/reviews?limit=50&offset=100
```

### Pagination Best Practices
```javascript
// Client-side pagination helper
class ApiPaginator {
  constructor(baseUrl, headers) {
    this.baseUrl = baseUrl;
    this.headers = headers;
  }
  
  async getPage(page = 1, limit = 20) {
    const offset = (page - 1) * limit;
    const url = `${this.baseUrl}?limit=${limit}&offset=${offset}`;
    
    const response = await fetch(url, { headers: this.headers });
    const data = await response.json();
    
    return {
      items: data.data.reviews,
      pagination: data.data.pagination,
      currentPage: page,
      totalPages: Math.ceil(data.data.pagination.total / limit)
    };
  }
}
```

## 🔍 Search and Filtering

### Full-Text Search
The API provides powerful search capabilities across review content:

```bash
# Basic search
GET /api/v1/reviews/search?query=excellent%20service

# Search with filters
GET /api/v1/reviews/search?query=location&rating_min=4&language=en

# Search with sorting
GET /api/v1/reviews/search?query=breakfast&sort=rating&order=desc
```

### Available Filters

#### Review Filters
| Parameter | Type | Description |
|-----------|------|-------------|
| `hotel_id` | UUID | Filter by hotel |
| `provider_id` | UUID | Filter by provider |
| `rating_min` | number | Minimum rating (1-5) |
| `rating_max` | number | Maximum rating (1-5) |
| `language` | string | Language code (e.g., 'en') |
| `sentiment` | string | positive/negative/neutral |
| `verified_only` | boolean | Only verified reviews |
| `from_date` | date | Reviews from date |
| `to_date` | date | Reviews to date |

#### Hotel Filters
| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Hotel name (partial match) |
| `city` | string | City name |
| `country` | string | Country name |
| `star_rating` | integer | Official star rating (1-5) |
| `min_rating` | number | Minimum review rating |

### Sorting Options
| Field | Description |
|-------|-------------|
| `rating` | Review rating |
| `review_date` | Review date |
| `created_at` | Creation timestamp |
| `helpful_votes` | Helpful votes count |

### Advanced Search Examples
```bash
# Complex review search
GET /api/v1/reviews/search?query=breakfast&hotel_id=123&rating_min=4&verified_only=true&sort=helpful_votes&order=desc

# Hotel search with location
GET /api/v1/hotels?city=London&star_rating=5&min_rating=4.0

# Date range filtering
GET /api/v1/reviews?from_date=2024-01-01&to_date=2024-01-31&sort=rating&order=desc
```

## 📁 File Upload

### CSV Upload for Bulk Review Processing

#### Upload Endpoint
```bash
curl -X POST https://api.hotelreviews.com/api/v1/reviews/upload \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -F "file=@reviews.csv" \
  -F "provider_id=123e4567-e89b-12d3-a456-426614174000" \
  -F "validate_only=false"
```

#### CSV Format Requirements
```csv
hotel_id,reviewer_name,rating,title,comment,review_date,language
123e4567-e89b-12d3-a456-426614174000,John Doe,4.5,"Great stay","Had a wonderful time",2024-01-15,en
223e4567-e89b-12d3-a456-426614174001,Jane Smith,3.5,"Average","It was okay",2024-01-16,en
```

#### Required CSV Columns
| Column | Type | Description |
|--------|------|-------------|
| `hotel_id` | UUID | Hotel identifier |
| `reviewer_name` | string | Reviewer name |
| `rating` | number | Rating (1.0-5.0) |
| `title` | string | Review title |
| `comment` | string | Review content |
| `review_date` | date | Review date (YYYY-MM-DD) |
| `language` | string | Language code |

#### Optional CSV Columns
- `service_rating`, `cleanliness_rating`, `location_rating`
- `value_rating`, `comfort_rating`, `facilities_rating`
- `trip_type`, `room_type`, `external_id`

#### File Constraints
- **Maximum file size**: 10MB
- **Maximum records**: 10,000 per file
- **Supported formats**: CSV only
- **Encoding**: UTF-8

#### Processing Status Tracking
```bash
# Check processing status
GET /api/v1/reviews/processing/{processing_id}

# Response example
{
  "success": true,
  "data": {
    "id": "processing-job-uuid",
    "status": "processing",
    "records_total": 1000,
    "records_processed": 750,
    "records_failed": 5,
    "progress_percentage": 75.0
  }
}
```

### File Upload Best Practices
```python
import requests
import time

def upload_and_monitor(file_path, provider_id, token):
    # Upload file
    with open(file_path, 'rb') as f:
        response = requests.post(
            'https://api.hotelreviews.com/api/v1/reviews/upload',
            headers={'Authorization': f'Bearer {token}'},
            files={'file': f},
            data={'provider_id': provider_id}
        )
    
    if response.status_code != 202:
        raise Exception(f"Upload failed: {response.text}")
    
    processing_id = response.json()['data']['processing_id']
    
    # Monitor processing
    while True:
        status_response = requests.get(
            f'https://api.hotelreviews.com/api/v1/reviews/processing/{processing_id}',
            headers={'Authorization': f'Bearer {token}'}
        )
        
        status_data = status_response.json()['data']
        print(f"Progress: {status_data['progress_percentage']}%")
        
        if status_data['status'] in ['completed', 'failed']:
            return status_data
            
        time.sleep(5)
```

## 🏥 Monitoring and Health

### Health Check Endpoints

#### Basic Health Check
```bash
GET /health

# Response
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "services": {
    "database": "healthy",
    "redis": "healthy",
    "circuit_breakers": "healthy"
  },
  "uptime_seconds": 86400
}
```

#### Circuit Breaker Health
```bash
GET /health/circuit-breakers

# Response
{
  "success": true,
  "data": {
    "circuit_breakers": {
      "database": {
        "state": "closed",
        "failure_count": 0,
        "last_failure_time": null
      },
      "external_service": {
        "state": "half_open",
        "failure_count": 3,
        "last_failure_time": "2024-01-15T10:25:00Z",
        "next_attempt": "2024-01-15T10:35:00Z"
      }
    }
  }
}
```

### Application Metrics
```bash
GET /metrics
Authorization: Bearer YOUR_ACCESS_TOKEN

# Response
{
  "success": true,
  "data": {
    "timestamp": "2024-01-15T10:30:00Z",
    "http_requests": {
      "total": 15420,
      "rate_per_second": 25.7,
      "avg_response_time_ms": 120.5,
      "error_rate": 0.02
    },
    "database": {
      "active_connections": 15,
      "avg_query_time_ms": 45.2,
      "slow_queries": 2
    },
    "cache": {
      "hit_rate": 85.3,
      "memory_usage_mb": 256.7,
      "evictions": 12
    }
  }
}
```

### Monitoring Integration
```yaml
# Prometheus scraping config
- job_name: 'hotel-reviews-api'
  static_configs:
    - targets: ['api.hotelreviews.com:8080']
  metrics_path: '/metrics'
  scrape_interval: 30s
```

## 📚 SDK and Examples

### JavaScript/Node.js SDK Example
```javascript
class HotelReviewsAPI {
  constructor(baseUrl, apiKey) {
    this.baseUrl = baseUrl;
    this.headers = {
      'Authorization': `Bearer ${apiKey}`,
      'Content-Type': 'application/json'
    };
  }
  
  async createReview(reviewData) {
    const response = await fetch(`${this.baseUrl}/reviews`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(reviewData)
    });
    return this.handleResponse(response);
  }
  
  async searchReviews(query, filters = {}) {
    const params = new URLSearchParams({ query, ...filters });
    const response = await fetch(`${this.baseUrl}/reviews/search?${params}`, {
      headers: this.headers
    });
    return this.handleResponse(response);
  }
  
  async handleResponse(response) {
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || 'API request failed');
    }
    return data;
  }
}

// Usage
const api = new HotelReviewsAPI('https://api.hotelreviews.com/api/v1', 'your-token');

const review = await api.createReview({
  hotel_id: '123e4567-e89b-12d3-a456-426614174000',
  reviewer_info_id: '456e7890-e89b-12d3-a456-426614174001',
  rating: 4.5,
  title: 'Great hotel!',
  comment: 'Had an amazing stay.',
  review_date: '2024-01-15T00:00:00Z',
  language: 'en'
});
```

### Python SDK Example
```python
import requests
from typing import Dict, List, Optional

class HotelReviewsAPI:
    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url
        self.headers = {
            'Authorization': f'Bearer {api_key}',
            'Content-Type': 'application/json'
        }
    
    def create_review(self, review_data: Dict) -> Dict:
        response = requests.post(
            f'{self.base_url}/reviews',
            headers=self.headers,
            json=review_data
        )
        return self._handle_response(response)
    
    def search_reviews(self, query: str, **filters) -> Dict:
        params = {'query': query, **filters}
        response = requests.get(
            f'{self.base_url}/reviews/search',
            headers=self.headers,
            params=params
        )
        return self._handle_response(response)
    
    def get_hotels(self, **filters) -> Dict:
        response = requests.get(
            f'{self.base_url}/hotels',
            headers=self.headers,
            params=filters
        )
        return self._handle_response(response)
    
    def _handle_response(self, response: requests.Response) -> Dict:
        data = response.json()
        if not response.ok:
            raise Exception(f"API Error: {data.get('error', 'Unknown error')}")
        return data

# Usage
api = HotelReviewsAPI('https://api.hotelreviews.com/api/v1', 'your-token')

# Create a review
review = api.create_review({
    'hotel_id': '123e4567-e89b-12d3-a456-426614174000',
    'reviewer_info_id': '456e7890-e89b-12d3-a456-426614174001',
    'rating': 4.5,
    'title': 'Great hotel!',
    'comment': 'Had an amazing stay.',
    'review_date': '2024-01-15T00:00:00Z',
    'language': 'en'
})

# Search reviews
results = api.search_reviews(
    'excellent service',
    rating_min=4,
    language='en',
    limit=20
)
```

## 🧪 Testing

### Testing Endpoints
Use the staging environment for testing:
- **Base URL**: `https://staging-api.hotelreviews.com/api/v1`
- **Test credentials available upon request**

### Test Data
The staging environment includes sample data:
- 100+ test hotels
- 1000+ test reviews
- Multiple test users with different permission levels

### Postman Collection
Download our Postman collection for easy API testing:
```bash
curl -o hotel-reviews-api.postman_collection.json \
  https://docs.hotelreviews.com/api/postman/collection.json
```

### Example Test Scripts
```bash
#!/bin/bash
# API smoke test script

API_BASE="https://staging-api.hotelreviews.com/api/v1"
TOKEN="your-test-token"

echo "Testing health endpoint..."
curl -f "$API_BASE/health" || exit 1

echo "Testing authenticated endpoint..."
curl -f -H "Authorization: Bearer $TOKEN" "$API_BASE/reviews?limit=1" || exit 1

echo "Testing search..."
curl -f -H "Authorization: Bearer $TOKEN" "$API_BASE/reviews/search?query=test" || exit 1

echo "All tests passed!"
```

## 📞 Support and Resources

- **API Documentation**: [OpenAPI Specification](./openapi.yaml)
- **SDK Downloads**: Available for JavaScript, Python, Go, Java
- **Support Email**: api-support@hotelreviews.com
- **Status Page**: https://status.hotelreviews.com
- **Developer Portal**: https://developers.hotelreviews.com

## 📋 Changelog

### Version 1.0.0 (Current)
- Initial API release
- Authentication and authorization
- CRUD operations for reviews, hotels, and providers
- Search and filtering capabilities
- File upload and batch processing
- Health monitoring and metrics

### Upcoming Features
- Real-time notifications via WebSockets
- Advanced analytics endpoints
- Machine learning-based review insights
- Enhanced search with semantic matching