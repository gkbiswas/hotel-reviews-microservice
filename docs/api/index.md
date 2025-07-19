# Hotel Reviews API Documentation

## Overview

The Hotel Reviews API is a comprehensive RESTful service for managing hotel reviews, ratings, and related data. Built with enterprise-grade features including JWT authentication, rate limiting, and real-time monitoring.

### Key Features

- 🔐 **Secure Authentication** - JWT-based authentication with refresh tokens
- ⚡ **High Performance** - Optimized for speed with caching and circuit breakers
- 📊 **Rich Analytics** - Comprehensive review statistics and insights
- 🔍 **Advanced Search** - Full-text search with powerful filtering
- 📁 **Batch Processing** - Bulk operations and CSV file uploads
- 🚀 **Developer Friendly** - Clear documentation, SDKs, and examples

### API Version

**Current Version:** v1  
**Base URL:** `https://api.hotelreviews.com/api/v1`

### Quick Links

- [Authentication Guide](./authentication.md)
- [Rate Limiting](./rate-limiting.md)
- [Error Handling](./errors.md)
- [SDK Generation](./sdk-generation.md)
- [Examples & Postman Collection](../examples/)
- [OpenAPI Specification](../../api/openapi.yaml)

## Getting Started

### 1. Register for an API Account

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "yourname",
    "email": "your.email@example.com",
    "password": "SecurePassword123!",
    "first_name": "Your",
    "last_name": "Name"
  }'
```

### 2. Authenticate to Get Access Token

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "your.email@example.com",
    "password": "SecurePassword123!"
  }'
```

### 3. Make Your First API Call

```bash
curl -X GET https://api.hotelreviews.com/api/v1/reviews \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## API Endpoints

### Authentication
- `POST /auth/register` - Register new user
- `POST /auth/login` - User login
- `POST /auth/refresh` - Refresh access token
- `POST /auth/logout` - User logout

### Reviews
- `GET /reviews` - List reviews
- `POST /reviews` - Create review
- `GET /reviews/{id}` - Get review
- `PUT /reviews/{id}` - Update review
- `DELETE /reviews/{id}` - Delete review
- `GET /reviews/search` - Search reviews
- `GET /reviews/statistics` - Get statistics
- `POST /reviews/bulk` - Bulk create
- `POST /reviews/upload` - Upload CSV file

### Hotels
- `GET /hotels` - List hotels
- `POST /hotels` - Create hotel
- `GET /hotels/{id}` - Get hotel
- `PUT /hotels/{id}` - Update hotel
- `DELETE /hotels/{id}` - Delete hotel

### Providers
- `GET /providers` - List providers
- `POST /providers` - Create provider
- `GET /providers/{id}` - Get provider

### Health & Monitoring
- `GET /health` - Health check
- `GET /health/circuit-breakers` - Circuit breaker status
- `GET /metrics` - Application metrics

## Response Format

All API responses follow a consistent format:

### Success Response
```json
{
  "success": true,
  "data": {
    // Response data here
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

### Error Response
```json
{
  "success": false,
  "error": "Human-readable error message",
  "error_code": "MACHINE_READABLE_CODE",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456",
  "details": {
    // Additional error context
  }
}
```

## Headers

### Request Headers
- `Authorization: Bearer {token}` - Required for authenticated endpoints
- `Content-Type: application/json` - For JSON requests
- `Accept: application/json` - To receive JSON responses

### Response Headers
- `X-RateLimit-Limit` - Request limit per window
- `X-RateLimit-Remaining` - Remaining requests
- `X-RateLimit-Reset` - Reset timestamp
- `X-Request-ID` - Unique request identifier

## Support

- **Documentation**: https://docs.hotelreviews.com
- **API Status**: https://status.hotelreviews.com
- **Support Email**: api-support@hotelreviews.com
- **Developer Forum**: https://forum.hotelreviews.com