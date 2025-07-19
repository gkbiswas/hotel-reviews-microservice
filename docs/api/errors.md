# Error Handling Guide

## Overview

The Hotel Reviews API uses consistent error responses across all endpoints. This guide covers error formats, common error codes, and best practices for error handling.

## Error Response Format

All errors follow a standard JSON format:

```json
{
  "success": false,
  "error": "Human-readable error message",
  "error_code": "MACHINE_READABLE_CODE",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456",
  "details": {
    // Additional context (optional)
  }
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `success` | boolean | Always `false` for errors |
| `error` | string | Human-readable error message |
| `error_code` | string | Machine-readable error code |
| `timestamp` | string | ISO 8601 timestamp |
| `trace_id` | string | Unique identifier for request tracing |
| `details` | object | Additional error context (optional) |

## HTTP Status Codes

### Success Codes (2xx)

| Code | Name | Description |
|------|------|-------------|
| 200 | OK | Request successful |
| 201 | Created | Resource created successfully |
| 202 | Accepted | Request accepted for processing |
| 204 | No Content | Request successful, no content to return |

### Client Error Codes (4xx)

| Code | Name | Description |
|------|------|-------------|
| 400 | Bad Request | Invalid request parameters |
| 401 | Unauthorized | Authentication required |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource not found |
| 409 | Conflict | Resource already exists |
| 413 | Payload Too Large | Request body too large |
| 422 | Unprocessable Entity | Validation error |
| 429 | Too Many Requests | Rate limit exceeded |

### Server Error Codes (5xx)

| Code | Name | Description |
|------|------|-------------|
| 500 | Internal Server Error | Unexpected server error |
| 502 | Bad Gateway | Upstream service error |
| 503 | Service Unavailable | Service temporarily unavailable |
| 504 | Gateway Timeout | Upstream service timeout |

## Error Codes Reference

### Authentication Errors

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `UNAUTHORIZED` | 401 | No authentication provided |
| `INVALID_TOKEN` | 401 | Invalid or expired token |
| `TOKEN_EXPIRED` | 401 | Access token has expired |
| `INVALID_CREDENTIALS` | 401 | Wrong username/password |
| `ACCOUNT_LOCKED` | 401 | Account temporarily locked |
| `UNVERIFIED_EMAIL` | 401 | Email verification required |

Example:
```json
{
  "success": false,
  "error": "Invalid or expired token",
  "error_code": "INVALID_TOKEN",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

### Authorization Errors

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `FORBIDDEN` | 403 | Insufficient permissions |
| `ROLE_REQUIRED` | 403 | Specific role required |
| `SUBSCRIPTION_REQUIRED` | 403 | Premium subscription needed |
| `RESOURCE_LIMIT_EXCEEDED` | 403 | Account resource limit reached |

Example:
```json
{
  "success": false,
  "error": "Insufficient permissions for this operation",
  "error_code": "FORBIDDEN",
  "details": {
    "required_role": "admin",
    "current_roles": ["user"]
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

### Validation Errors

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `VALIDATION_ERROR` | 422 | Request validation failed |
| `INVALID_FORMAT` | 422 | Invalid data format |
| `MISSING_REQUIRED_FIELD` | 422 | Required field missing |
| `OUT_OF_RANGE` | 422 | Value outside allowed range |
| `DUPLICATE_VALUE` | 422 | Unique constraint violation |

Example:
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
        "code": "INVALID_FORMAT",
        "value": "not-an-email"
      },
      {
        "field": "rating",
        "message": "Rating must be between 1 and 5",
        "code": "OUT_OF_RANGE",
        "value": 6
      }
    ]
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

### Resource Errors

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource already exists |
| `RESOURCE_LOCKED` | 423 | Resource is locked |
| `GONE` | 410 | Resource permanently deleted |

Example:
```json
{
  "success": false,
  "error": "The requested resource was not found",
  "error_code": "NOT_FOUND",
  "details": {
    "resource_type": "review",
    "resource_id": "550e8400-e29b-41d4-a716-446655440000"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

### Rate Limiting Errors

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `DAILY_LIMIT_EXCEEDED` | 429 | Daily quota exceeded |
| `CONCURRENT_LIMIT_EXCEEDED` | 429 | Too many concurrent requests |

Example:
```json
{
  "success": false,
  "error": "Rate limit exceeded",
  "error_code": "RATE_LIMIT_EXCEEDED",
  "details": {
    "limit": 1000,
    "window": "1 hour",
    "retry_after": 3245
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

### Business Logic Errors

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `INVALID_OPERATION` | 400 | Operation not allowed |
| `PREREQUISITE_FAILED` | 412 | Required condition not met |
| `BUSINESS_RULE_VIOLATION` | 422 | Business rule violated |
| `INSUFFICIENT_FUNDS` | 402 | Payment required |

Example:
```json
{
  "success": false,
  "error": "Cannot delete hotel with existing reviews",
  "error_code": "INVALID_OPERATION",
  "details": {
    "hotel_id": "123e4567-e89b-12d3-a456-426614174000",
    "review_count": 42
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

### Server Errors

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `INTERNAL_ERROR` | 500 | Internal server error |
| `DATABASE_ERROR` | 500 | Database operation failed |
| `EXTERNAL_SERVICE_ERROR` | 502 | External service failed |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily down |
| `TIMEOUT` | 504 | Request timeout |

Example:
```json
{
  "success": false,
  "error": "An internal server error occurred",
  "error_code": "INTERNAL_ERROR",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
```

## Error Handling Best Practices

### 1. Check Status Code First

```javascript
async function handleApiResponse(response) {
  // Success responses (2xx)
  if (response.ok) {
    return response.json();
  }
  
  // Error responses
  const errorData = await response.json();
  
  switch (response.status) {
    case 400:
      throw new BadRequestError(errorData);
    case 401:
      throw new UnauthorizedError(errorData);
    case 403:
      throw new ForbiddenError(errorData);
    case 404:
      throw new NotFoundError(errorData);
    case 422:
      throw new ValidationError(errorData);
    case 429:
      throw new RateLimitError(errorData);
    default:
      throw new ApiError(errorData);
  }
}
```

### 2. Create Custom Error Classes

```javascript
class ApiError extends Error {
  constructor(errorData) {
    super(errorData.error);
    this.name = 'ApiError';
    this.errorCode = errorData.error_code;
    this.details = errorData.details;
    this.traceId = errorData.trace_id;
    this.timestamp = errorData.timestamp;
  }
}

class ValidationError extends ApiError {
  constructor(errorData) {
    super(errorData);
    this.name = 'ValidationError';
    this.validationErrors = errorData.details?.validation_errors || [];
  }
  
  getFieldErrors() {
    return this.validationErrors.reduce((acc, error) => {
      acc[error.field] = error.message;
      return acc;
    }, {});
  }
}

class RateLimitError extends ApiError {
  constructor(errorData) {
    super(errorData);
    this.name = 'RateLimitError';
    this.retryAfter = errorData.details?.retry_after;
  }
  
  getRetryAfterMs() {
    return this.retryAfter ? this.retryAfter * 1000 : 60000;
  }
}
```

### 3. Implement Comprehensive Error Handling

```javascript
class ApiClient {
  async makeRequest(url, options = {}) {
    try {
      const response = await fetch(url, options);
      return await this.handleResponse(response);
    } catch (error) {
      return this.handleError(error);
    }
  }
  
  async handleResponse(response) {
    const data = await response.json();
    
    if (!response.ok) {
      throw this.createError(response.status, data);
    }
    
    return data;
  }
  
  createError(status, errorData) {
    switch (errorData.error_code) {
      case 'VALIDATION_ERROR':
        return new ValidationError(errorData);
      case 'RATE_LIMIT_EXCEEDED':
        return new RateLimitError(errorData);
      case 'UNAUTHORIZED':
      case 'INVALID_TOKEN':
        return new UnauthorizedError(errorData);
      case 'FORBIDDEN':
        return new ForbiddenError(errorData);
      case 'NOT_FOUND':
        return new NotFoundError(errorData);
      default:
        return new ApiError(errorData);
    }
  }
  
  handleError(error) {
    if (error instanceof ApiError) {
      // Log API errors with trace ID for debugging
      console.error(`API Error [${error.traceId}]:`, error.message);
      
      // Handle specific error types
      if (error instanceof UnauthorizedError) {
        // Redirect to login
        window.location.href = '/login';
      } else if (error instanceof RateLimitError) {
        // Show rate limit message
        this.showErrorMessage(`Please wait ${error.retryAfter} seconds before trying again`);
      } else if (error instanceof ValidationError) {
        // Show field-specific errors
        this.showValidationErrors(error.getFieldErrors());
      }
    } else {
      // Network or other errors
      console.error('Request failed:', error);
      this.showErrorMessage('Network error. Please check your connection.');
    }
    
    throw error;
  }
}
```

### 4. User-Friendly Error Messages

```javascript
function getErrorMessage(error) {
  // Map technical errors to user-friendly messages
  const errorMessages = {
    'INVALID_TOKEN': 'Your session has expired. Please log in again.',
    'FORBIDDEN': 'You don\'t have permission to perform this action.',
    'NOT_FOUND': 'The requested item could not be found.',
    'RATE_LIMIT_EXCEEDED': 'Too many requests. Please slow down.',
    'VALIDATION_ERROR': 'Please check your input and try again.',
    'INTERNAL_ERROR': 'Something went wrong. Please try again later.',
    'NETWORK_ERROR': 'Connection error. Please check your internet.',
  };
  
  if (error instanceof ApiError) {
    return errorMessages[error.errorCode] || error.message;
  }
  
  return errorMessages.NETWORK_ERROR;
}

// Display errors in UI
function displayError(error) {
  const message = getErrorMessage(error);
  
  // Show toast notification
  showToast({
    type: 'error',
    message: message,
    duration: 5000,
    action: error instanceof UnauthorizedError ? {
      label: 'Login',
      onClick: () => window.location.href = '/login'
    } : null
  });
  
  // Log for debugging
  if (error.traceId) {
    console.error(`Error trace ID: ${error.traceId}`);
  }
}
```

### 5. Form Validation Error Display

```javascript
class FormValidator {
  constructor(formElement) {
    this.form = formElement;
    this.errors = {};
  }
  
  displayApiErrors(validationError) {
    // Clear previous errors
    this.clearErrors();
    
    // Display field-specific errors
    validationError.validationErrors.forEach(error => {
      this.setFieldError(error.field, error.message);
    });
  }
  
  setFieldError(fieldName, message) {
    const field = this.form.querySelector(`[name="${fieldName}"]`);
    if (!field) return;
    
    // Add error class
    field.classList.add('error');
    
    // Show error message
    const errorElement = document.createElement('div');
    errorElement.className = 'field-error';
    errorElement.textContent = message;
    field.parentElement.appendChild(errorElement);
    
    this.errors[fieldName] = message;
  }
  
  clearErrors() {
    // Remove all error classes and messages
    this.form.querySelectorAll('.error').forEach(el => {
      el.classList.remove('error');
    });
    
    this.form.querySelectorAll('.field-error').forEach(el => {
      el.remove();
    });
    
    this.errors = {};
  }
}

// Usage
async function submitForm(formData) {
  const validator = new FormValidator(document.getElementById('review-form'));
  
  try {
    const response = await apiClient.createReview(formData);
    // Success - redirect or show success message
  } catch (error) {
    if (error instanceof ValidationError) {
      validator.displayApiErrors(error);
    } else {
      displayError(error);
    }
  }
}
```

### 6. Retry Logic for Transient Errors

```javascript
class RetryableApiClient extends ApiClient {
  constructor(maxRetries = 3) {
    super();
    this.maxRetries = maxRetries;
    this.retryableErrors = [
      'INTERNAL_ERROR',
      'SERVICE_UNAVAILABLE',
      'TIMEOUT',
      'EXTERNAL_SERVICE_ERROR'
    ];
  }
  
  async makeRequestWithRetry(url, options = {}, retryCount = 0) {
    try {
      return await this.makeRequest(url, options);
    } catch (error) {
      if (this.shouldRetry(error, retryCount)) {
        const delay = this.getRetryDelay(error, retryCount);
        console.log(`Retrying after ${delay}ms (attempt ${retryCount + 1}/${this.maxRetries})`);
        
        await this.sleep(delay);
        return this.makeRequestWithRetry(url, options, retryCount + 1);
      }
      
      throw error;
    }
  }
  
  shouldRetry(error, retryCount) {
    if (retryCount >= this.maxRetries) return false;
    
    if (error instanceof RateLimitError) return true;
    
    if (error instanceof ApiError) {
      return this.retryableErrors.includes(error.errorCode);
    }
    
    // Retry on network errors
    return error.name === 'NetworkError' || error.name === 'TimeoutError';
  }
  
  getRetryDelay(error, retryCount) {
    if (error instanceof RateLimitError) {
      return error.getRetryAfterMs();
    }
    
    // Exponential backoff: 1s, 2s, 4s
    return Math.min(1000 * Math.pow(2, retryCount), 10000);
  }
  
  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}
```

## Error Monitoring and Logging

### Client-Side Error Tracking

```javascript
class ErrorTracker {
  constructor(apiKey) {
    this.apiKey = apiKey;
    this.errors = [];
  }
  
  trackError(error, context = {}) {
    const errorInfo = {
      timestamp: new Date().toISOString(),
      error: {
        message: error.message,
        code: error.errorCode || 'UNKNOWN',
        trace_id: error.traceId,
        stack: error.stack
      },
      context: {
        url: window.location.href,
        user_agent: navigator.userAgent,
        ...context
      }
    };
    
    this.errors.push(errorInfo);
    
    // Send to monitoring service
    if (this.shouldReport(error)) {
      this.sendToMonitoring(errorInfo);
    }
  }
  
  shouldReport(error) {
    // Don't report client errors like validation
    const clientErrors = ['VALIDATION_ERROR', 'UNAUTHORIZED', 'FORBIDDEN'];
    return !clientErrors.includes(error.errorCode);
  }
  
  async sendToMonitoring(errorInfo) {
    try {
      await fetch('https://monitoring.example.com/errors', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-Key': this.apiKey
        },
        body: JSON.stringify(errorInfo)
      });
    } catch (e) {
      console.error('Failed to send error to monitoring:', e);
    }
  }
}

// Global error handler
const errorTracker = new ErrorTracker('monitoring-api-key');

window.addEventListener('unhandledrejection', event => {
  if (event.reason instanceof ApiError) {
    errorTracker.trackError(event.reason, {
      type: 'unhandled_promise_rejection'
    });
  }
});
```

## Common Error Scenarios

### Expired Token

```javascript
// Automatically refresh token on 401
apiClient.interceptors.response.use(
  response => response,
  async error => {
    if (error.errorCode === 'TOKEN_EXPIRED' && !error.config._retry) {
      error.config._retry = true;
      
      try {
        await authService.refreshToken();
        return apiClient.request(error.config);
      } catch (refreshError) {
        authService.logout();
        window.location.href = '/login';
      }
    }
    
    return Promise.reject(error);
  }
);
```

### Network Errors

```javascript
// Detect and handle network errors
async function fetchWithNetworkError(url, options) {
  try {
    const response = await fetch(url, options);
    return response;
  } catch (error) {
    if (!navigator.onLine) {
      throw new NetworkError('No internet connection');
    }
    
    if (error.name === 'AbortError') {
      throw new TimeoutError('Request timeout');
    }
    
    throw new NetworkError('Network request failed');
  }
}
```

### Circuit Breaker Pattern

```javascript
class CircuitBreaker {
  constructor(threshold = 5, timeout = 60000) {
    this.failures = 0;
    this.threshold = threshold;
    this.timeout = timeout;
    this.state = 'closed'; // closed, open, half-open
    this.nextAttempt = Date.now();
  }
  
  async call(fn) {
    if (this.state === 'open') {
      if (Date.now() < this.nextAttempt) {
        throw new Error('Circuit breaker is open');
      }
      this.state = 'half-open';
    }
    
    try {
      const result = await fn();
      this.onSuccess();
      return result;
    } catch (error) {
      this.onFailure();
      throw error;
    }
  }
  
  onSuccess() {
    this.failures = 0;
    this.state = 'closed';
  }
  
  onFailure() {
    this.failures++;
    if (this.failures >= this.threshold) {
      this.state = 'open';
      this.nextAttempt = Date.now() + this.timeout;
    }
  }
}

// Usage
const breaker = new CircuitBreaker();

async function makeResilientRequest(url, options) {
  return breaker.call(() => fetch(url, options));
}
```

## Troubleshooting Guide

### Common Integration Issues

#### 1. CORS Errors

**Problem:** Browser blocks requests due to CORS policy

**Solution:**
```javascript
// Ensure proper CORS headers in requests
const response = await fetch('https://api.hotelreviews.com/api/v1/reviews', {
  method: 'GET',
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'Content-Type': 'application/json',
    'Origin': 'https://yourapp.com'
  },
  mode: 'cors',
  credentials: 'include'
});
```

#### 2. Token Parsing Errors

**Problem:** "Invalid token format" error

**Solution:**
```javascript
// Ensure token is properly formatted
function validateTokenFormat(token) {
  const parts = token.split('.');
  if (parts.length !== 3) {
    throw new Error('Invalid JWT format');
  }
  
  try {
    // Validate each part is base64
    parts.forEach(part => {
      atob(part.replace(/-/g, '+').replace(/_/g, '/'));
    });
    return true;
  } catch (e) {
    throw new Error('Invalid JWT encoding');
  }
}
```

#### 3. Rate Limit Loops

**Problem:** Application gets stuck in rate limit retry loops

**Solution:**
```javascript
// Implement maximum retry limits
class RateLimitHandler {
  constructor(maxRetries = 3, maxWaitTime = 300000) { // 5 minutes max
    this.maxRetries = maxRetries;
    this.maxWaitTime = maxWaitTime;
  }
  
  async handleRateLimit(response, retryCount = 0) {
    if (retryCount >= this.maxRetries) {
      throw new Error('Maximum rate limit retries exceeded');
    }
    
    const retryAfter = parseInt(response.headers.get('Retry-After') || '60');
    const waitTime = Math.min(retryAfter * 1000, this.maxWaitTime);
    
    if (waitTime > this.maxWaitTime) {
      throw new Error('Rate limit wait time exceeds maximum');
    }
    
    await this.sleep(waitTime);
    return retryCount + 1;
  }
  
  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}
```

#### 4. Concurrent Request Errors

**Problem:** "Too many concurrent connections" errors

**Solution:**
```javascript
// Implement connection pooling
class ConnectionPool {
  constructor(maxConcurrent = 10) {
    this.maxConcurrent = maxConcurrent;
    this.active = 0;
    this.queue = [];
  }
  
  async execute(fn) {
    while (this.active >= this.maxConcurrent) {
      await new Promise(resolve => this.queue.push(resolve));
    }
    
    this.active++;
    try {
      return await fn();
    } finally {
      this.active--;
      const next = this.queue.shift();
      if (next) next();
    }
  }
}

const pool = new ConnectionPool(10);

// Use the pool for all requests
function makePooledRequest(url, options) {
  return pool.execute(() => fetch(url, options));
}
```

### Error Recovery Strategies

#### 1. Graceful Degradation

```javascript
class ApiClientWithFallback {
  async getReviews(hotelId) {
    try {
      // Try primary API
      return await this.fetchFromApi(`/reviews?hotel_id=${hotelId}`);
    } catch (error) {
      if (error.errorCode === 'SERVICE_UNAVAILABLE') {
        // Fall back to cached data
        return this.getCachedReviews(hotelId);
      }
      throw error;
    }
  }
  
  getCachedReviews(hotelId) {
    const cached = localStorage.getItem(`reviews_${hotelId}`);
    if (cached) {
      const data = JSON.parse(cached);
      console.warn('Using cached data due to API unavailability');
      return data;
    }
    throw new Error('No cached data available');
  }
}
```

#### 2. Error Aggregation

```javascript
class ErrorAggregator {
  constructor() {
    this.errors = [];
    this.errorCounts = {};
  }
  
  addError(error) {
    this.errors.push({
      error,
      timestamp: new Date(),
      context: this.getCurrentContext()
    });
    
    // Count by error code
    const code = error.errorCode || 'UNKNOWN';
    this.errorCounts[code] = (this.errorCounts[code] || 0) + 1;
    
    // Check for patterns
    this.checkErrorPatterns();
  }
  
  checkErrorPatterns() {
    // Alert if too many errors of same type
    Object.entries(this.errorCounts).forEach(([code, count]) => {
      if (count > 10 && count % 10 === 0) {
        console.error(`High frequency of ${code} errors: ${count} occurrences`);
        this.sendAlert(code, count);
      }
    });
  }
  
  getCurrentContext() {
    return {
      url: window.location.href,
      userAgent: navigator.userAgent,
      timestamp: new Date().toISOString()
    };
  }
  
  sendAlert(errorCode, count) {
    // Send to monitoring service
    fetch('/api/monitoring/alerts', {
      method: 'POST',
      body: JSON.stringify({
        type: 'high_error_frequency',
        error_code: errorCode,
        count: count,
        context: this.getCurrentContext()
      })
    }).catch(console.error);
  }
}
```

### Debugging Tools

#### API Error Inspector

```javascript
class ApiErrorInspector {
  static inspect(error) {
    console.group(`API Error: ${error.errorCode || 'UNKNOWN'}`);
    console.error('Message:', error.message);
    console.error('Code:', error.errorCode);
    console.error('Trace ID:', error.traceId);
    console.error('Timestamp:', error.timestamp);
    
    if (error.details) {
      console.group('Details:');
      console.table(error.details);
      console.groupEnd();
    }
    
    if (error.stack) {
      console.error('Stack trace:', error.stack);
    }
    
    console.groupEnd();
    
    // Return formatted error info
    return {
      summary: `${error.errorCode}: ${error.message}`,
      traceUrl: `https://api.hotelreviews.com/debug/trace/${error.traceId}`,
      timestamp: error.timestamp,
      details: error.details
    };
  }
}

// Usage
try {
  await apiClient.createReview(data);
} catch (error) {
  const errorInfo = ApiErrorInspector.inspect(error);
  // Use errorInfo for debugging or user display
}
```

#### Request/Response Logger

```javascript
class ApiLogger {
  constructor(enabled = true) {
    this.enabled = enabled;
    this.logs = [];
  }
  
  logRequest(url, options) {
    if (!this.enabled) return;
    
    const log = {
      id: Date.now(),
      type: 'request',
      url,
      method: options.method || 'GET',
      headers: options.headers,
      body: options.body,
      timestamp: new Date().toISOString()
    };
    
    this.logs.push(log);
    console.log('API Request:', log);
    return log.id;
  }
  
  logResponse(requestId, response, data) {
    if (!this.enabled) return;
    
    const log = {
      id: requestId,
      type: 'response',
      status: response.status,
      statusText: response.statusText,
      headers: Object.fromEntries(response.headers.entries()),
      data: data,
      timestamp: new Date().toISOString()
    };
    
    this.logs.push(log);
    console.log('API Response:', log);
  }
  
  logError(requestId, error) {
    if (!this.enabled) return;
    
    const log = {
      id: requestId,
      type: 'error',
      error: {
        message: error.message,
        code: error.errorCode,
        details: error.details
      },
      timestamp: new Date().toISOString()
    };
    
    this.logs.push(log);
    console.error('API Error:', log);
  }
  
  exportLogs() {
    return this.logs;
  }
  
  clearLogs() {
    this.logs = [];
  }
}

// Integrate with API client
const logger = new ApiLogger();

async function makeLoggedRequest(url, options) {
  const requestId = logger.logRequest(url, options);
  
  try {
    const response = await fetch(url, options);
    const data = await response.json();
    
    logger.logResponse(requestId, response, data);
    
    if (!response.ok) {
      throw new ApiError(data);
    }
    
    return data;
  } catch (error) {
    logger.logError(requestId, error);
    throw error;
  }
}
```

## Support Resources

### Error Reporting

If you encounter persistent errors:

1. **Collect Error Information**
   - Error code and message
   - Trace ID from response
   - Request details (endpoint, method, headers)
   - Timestamp of occurrence

2. **Report to Support**
   - Email: api-support@hotelreviews.com
   - Include trace ID in subject line
   - Attach request/response logs

3. **Status Page**
   - Check https://status.hotelreviews.com
   - Subscribe to incident notifications
   - View historical uptime

### Developer Resources

- **API Playground**: https://playground.hotelreviews.com
- **Error Code Reference**: https://docs.hotelreviews.com/errors
- **Community Forum**: https://forum.hotelreviews.com
- **Stack Overflow Tag**: `hotel-reviews-api`