# API Reference Documentation

## Overview

The Hotel Reviews Microservice provides a comprehensive REST API for managing hotel reviews, hotels, and analytics. This document provides detailed information about all available endpoints, request/response formats, and usage examples.

## Base URL

```
Production: https://api.hotelreviews.com/api/v1
Staging: https://staging-api.hotelreviews.com/api/v1
Development: http://localhost:8080/api/v1
```

## Authentication

### JWT Authentication

All protected endpoints require a valid JWT token in the Authorization header:

```http
Authorization: Bearer <jwt_token>
```

#### Login Endpoint

```http
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2024-01-16T10:00:00Z",
  "user": {
    "id": "user-123",
    "email": "user@example.com",
    "roles": ["user"]
  }
}
```

### API Key Authentication

For service-to-service communication, use API keys:

```http
X-API-Key: ak_1234567890abcdef
```

## Rate Limiting

- **Default**: 100 requests per minute per IP
- **Authenticated**: 1000 requests per minute per user
- **API Keys**: 5000 requests per minute per key

Rate limit headers are included in responses:

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642262400
```

## Error Handling

### Error Response Format

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "The request is invalid",
    "details": {
      "field": "email",
      "reason": "Invalid email format"
    },
    "request_id": "req-123",
    "timestamp": "2024-01-15T10:00:00Z"
  }
}
```

### HTTP Status Codes

- `200` - Success
- `201` - Created
- `204` - No Content
- `400` - Bad Request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `409` - Conflict
- `422` - Unprocessable Entity
- `429` - Too Many Requests
- `500` - Internal Server Error
- `503` - Service Unavailable

## Reviews API

### Get Review

```http
GET /reviews/{id}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "hotel_id": "hotel-123",
  "provider_id": "provider-789",
  "external_id": "ext-456",
  "title": "Amazing stay!",
  "content": "Great hotel with excellent service...",
  "rating": 4.5,
  "language": "en",
  "author": {
    "name": "John Doe",
    "location": "New York"
  },
  "review_date": "2024-01-15T10:00:00Z",
  "created_at": "2024-01-15T10:05:00Z",
  "updated_at": "2024-01-15T10:05:00Z"
}
```

### Create Review

```http
POST /reviews
Authorization: Bearer <token>
Content-Type: application/json

{
  "hotel_id": "hotel-123",
  "provider_id": "provider-789",
  "external_id": "ext-456",
  "title": "Amazing stay!",
  "content": "Great hotel with excellent service...",
  "rating": 4.5,
  "language": "en",
  "author": {
    "name": "John Doe",
    "location": "New York"
  },
  "review_date": "2024-01-15T10:00:00Z"
}
```

### Update Review

```http
PUT /reviews/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "Updated title",
  "content": "Updated content...",
  "rating": 5.0
}
```

### Delete Review

```http
DELETE /reviews/{id}
Authorization: Bearer <token>
```

### Search Reviews

```http
POST /reviews/search
Content-Type: application/json

{
  "query": "excellent service clean rooms",
  "filters": {
    "rating_min": 4.0,
    "rating_max": 5.0,
    "language": "en",
    "date_from": "2024-01-01",
    "date_to": "2024-12-31",
    "hotel_ids": ["hotel-123", "hotel-456"]
  },
  "sort": "rating_desc",
  "limit": 50,
  "offset": 0
}
```

### Bulk Create Reviews

```http
POST /reviews/bulk
Authorization: Bearer <token>
Content-Type: application/json

{
  "provider_id": "provider-123",
  "file_url": "s3://bucket/reviews.jsonl",
  "callback_url": "https://your-service.com/webhooks/reviews"
}
```

**Response:**
```json
{
  "job_id": "job-123",
  "status": "queued",
  "created_at": "2024-01-15T10:00:00Z",
  "callback_url": "https://your-service.com/webhooks/reviews"
}
```

### Get Bulk Job Status

```http
GET /reviews/bulk/{job_id}/status
Authorization: Bearer <token>
```

**Response:**
```json
{
  "job_id": "job-123",
  "status": "processing",
  "progress": {
    "total": 10000,
    "processed": 7500,
    "failed": 25,
    "percentage": 75.0
  },
  "created_at": "2024-01-15T10:00:00Z",
  "started_at": "2024-01-15T10:01:00Z",
  "estimated_completion": "2024-01-15T10:30:00Z"
}
```

## Hotels API

### Get Hotel

```http
GET /hotels/{id}
```

**Response:**
```json
{
  "id": "hotel-123",
  "name": "Grand Hotel",
  "description": "Luxury hotel in the heart of the city",
  "address": {
    "street": "123 Main St",
    "city": "New York",
    "state": "NY",
    "country": "USA",
    "postal_code": "10001",
    "coordinates": {
      "latitude": 40.7128,
      "longitude": -74.0060
    }
  },
  "contact": {
    "phone": "+1-555-123-4567",
    "email": "info@grandhotel.com",
    "website": "https://grandhotel.com"
  },
  "amenities": ["wifi", "pool", "spa", "gym", "parking"],
  "rating": 4.5,
  "price_range": "$$$$",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:00:00Z"
}
```

### Create Hotel

```http
POST /hotels
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Grand Hotel",
  "description": "Luxury hotel in the heart of the city",
  "address": {
    "street": "123 Main St",
    "city": "New York",
    "state": "NY",
    "country": "USA",
    "postal_code": "10001",
    "coordinates": {
      "latitude": 40.7128,
      "longitude": -74.0060
    }
  },
  "contact": {
    "phone": "+1-555-123-4567",
    "email": "info@grandhotel.com",
    "website": "https://grandhotel.com"
  },
  "amenities": ["wifi", "pool", "spa", "gym", "parking"],
  "price_range": "$$$$"
}
```

### Update Hotel

```http
PUT /hotels/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Grand Luxury Hotel",
  "description": "Updated description...",
  "amenities": ["wifi", "pool", "spa", "gym", "parking", "restaurant"]
}
```

### List Hotels

```http
GET /hotels?city=New York&rating_min=4.0&limit=20&offset=0
```

**Query Parameters:**
- `city` - Filter by city
- `country` - Filter by country
- `rating_min` - Minimum rating
- `rating_max` - Maximum rating
- `amenities` - Filter by amenities (comma-separated)
- `price_range` - Filter by price range
- `limit` - Number of results (default: 20, max: 100)
- `offset` - Pagination offset (default: 0)
- `sort` - Sort order (name_asc, name_desc, rating_asc, rating_desc)

### Get Hotel Reviews

```http
GET /hotels/{id}/reviews?limit=20&offset=0&sort=rating_desc
```

## Analytics API

### Hotel Statistics

```http
GET /analytics/hotels/{id}/stats
```

**Response:**
```json
{
  "hotel_id": "hotel-123",
  "review_count": 1542,
  "average_rating": 4.5,
  "rating_distribution": {
    "1": 12,
    "2": 25,
    "3": 89,
    "4": 456,
    "5": 960
  },
  "monthly_trends": [
    {
      "month": "2024-01",
      "review_count": 128,
      "average_rating": 4.6
    }
  ],
  "sentiment_analysis": {
    "positive": 0.75,
    "neutral": 0.20,
    "negative": 0.05
  },
  "top_keywords": ["clean", "service", "location", "staff", "comfortable"],
  "last_updated": "2024-01-15T10:00:00Z"
}
```

### Top Rated Hotels

```http
GET /analytics/top-hotels?limit=10&city=New York&timeframe=30d
```

### Review Sentiment Analysis

```http
POST /analytics/reviews/sentiment
Content-Type: application/json

{
  "hotel_id": "hotel-123",
  "date_from": "2024-01-01",
  "date_to": "2024-12-31",
  "language": "en"
}
```

### Real-time Metrics

```http
GET /analytics/metrics/realtime
Authorization: Bearer <admin_token>
```

**Response:**
```json
{
  "timestamp": "2024-01-15T10:00:00Z",
  "metrics": {
    "requests_per_second": 150.5,
    "active_connections": 1250,
    "cache_hit_ratio": 0.92,
    "average_response_time": 45.2,
    "error_rate": 0.01
  },
  "system": {
    "cpu_usage": 0.35,
    "memory_usage": 0.68,
    "disk_usage": 0.42
  },
  "database": {
    "active_connections": 25,
    "queries_per_second": 89.3,
    "slow_queries": 2
  }
}
```

## Admin API

### Health Checks

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:00:00Z",
  "version": "1.0.0",
  "uptime": "72h30m15s",
  "checks": {
    "database": "healthy",
    "redis": "healthy",
    "kafka": "healthy"
  }
}
```

### Deep Health Check

```http
GET /health/deep
```

### Component Health

```http
GET /health/database
GET /health/redis  
GET /health/kafka
```

### Cache Management

```http
POST /admin/cache/clear
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "pattern": "reviews:*",
  "confirm": true
}
```

### Cache Warming

```http
POST /admin/cache/warm
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "type": "hotels",
  "criteria": "popular",
  "limit": 100
}
```

### Configuration Reload

```http
POST /admin/config/reload
Authorization: Bearer <admin_token>
```

### Circuit Breaker Status

```http
GET /admin/circuit-breakers
Authorization: Bearer <admin_token>
```

## Webhooks

### Bulk Processing Webhook

When bulk processing jobs complete, the system sends a webhook to the specified callback URL:

```json
{
  "event": "bulk_processing.completed",
  "job_id": "job-123",
  "status": "completed",
  "results": {
    "total": 10000,
    "processed": 9975,
    "failed": 25,
    "success_rate": 0.9975
  },
  "started_at": "2024-01-15T10:00:00Z",
  "completed_at": "2024-01-15T10:25:00Z",
  "duration": "25m0s"
}
```

## SDKs and Client Libraries

### Go SDK

```go
import "github.com/gkbiswas/hotel-reviews-sdk-go"

client := hotelreviews.NewClient("your-api-key")
review, err := client.Reviews.Get(ctx, "review-id")
```

### JavaScript/Node.js SDK

```javascript
const HotelReviews = require('@hotelreviews/sdk');

const client = new HotelReviews({
  apiKey: 'your-api-key',
  baseURL: 'https://api.hotelreviews.com'
});

const review = await client.reviews.get('review-id');
```

### Python SDK

```python
from hotelreviews import Client

client = Client(api_key='your-api-key')
review = client.reviews.get('review-id')
```

## API Limits

### Rate Limits

| Plan | Requests/min | Burst | Notes |
|------|-------------|-------|-------|
| Free | 100 | 200 | Basic usage |
| Pro | 1,000 | 2,000 | Commercial usage |
| Enterprise | 10,000 | 20,000 | High volume |

### Payload Limits

- **Request Body**: 1MB maximum
- **Bulk Upload**: 100MB maximum
- **Query Results**: 1000 records maximum per request

## Versioning

The API uses semantic versioning with the version specified in the URL path:

- **Current Version**: v1
- **Deprecation Notice**: 6 months before removal
- **Breaking Changes**: New major version only

## Support

- **Documentation**: https://docs.hotelreviews.com
- **Support**: gkbiswas@gmail.com
- **Status Page**: https://status.hotelreviews.com