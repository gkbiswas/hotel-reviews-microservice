# Rate Limiting Guide

## Overview

The Hotel Reviews API implements rate limiting to ensure fair usage and protect against abuse. This guide explains the rate limiting policies, headers, and best practices for handling rate limits.

## Rate Limit Tiers

### Default Limits

| User Type | Limit | Window | Description |
|-----------|-------|--------|-------------|
| **Unauthenticated** | 100 requests | 1 hour | Public endpoints only |
| **Authenticated** | 1,000 requests | 1 hour | Standard user limit |
| **Premium** | 5,000 requests | 1 hour | Premium account limit |
| **Enterprise** | Custom | Custom | Contact sales |

### Endpoint-Specific Limits

Some endpoints have stricter limits due to their resource-intensive nature:

| Endpoint | Limit | Window | Reason |
|----------|-------|--------|--------|
| `/auth/login` | 10 requests | 1 minute | Prevent brute force |
| `/auth/register` | 5 requests | 1 hour | Prevent spam accounts |
| `/reviews/upload` | 5 requests | 1 hour | File processing overhead |
| `/reviews/bulk` | 20 requests | 1 hour | Database intensive |
| `/reviews/search` | 100 requests | 1 minute | Search engine load |

## Rate Limit Headers

Every API response includes rate limit information in the headers:

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1642252800
X-RateLimit-Reset-After: 3245
Retry-After: 60
```

### Header Definitions

| Header | Type | Description |
|--------|------|-------------|
| `X-RateLimit-Limit` | integer | Maximum requests allowed in the window |
| `X-RateLimit-Remaining` | integer | Requests remaining in current window |
| `X-RateLimit-Reset` | integer | Unix timestamp when the window resets |
| `X-RateLimit-Reset-After` | integer | Seconds until the window resets |
| `Retry-After` | integer | Seconds to wait before retrying (only on 429) |

## Rate Limit Response

When rate limit is exceeded, the API returns:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1642252800
Retry-After: 3245

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

## Handling Rate Limits

### JavaScript Example

```javascript
class RateLimitedApiClient {
  constructor(baseUrl, apiKey) {
    this.baseUrl = baseUrl;
    this.apiKey = apiKey;
    this.rateLimitInfo = {
      limit: null,
      remaining: null,
      resetAt: null
    };
  }

  async makeRequest(endpoint, options = {}) {
    // Check if we should wait before making request
    if (this.shouldWaitForRateLimit()) {
      const waitTime = this.getWaitTime();
      console.log(`Rate limit approaching, waiting ${waitTime}ms`);
      await this.sleep(waitTime);
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json',
        ...options.headers
      }
    });

    // Update rate limit info
    this.updateRateLimitInfo(response);

    // Handle rate limit exceeded
    if (response.status === 429) {
      const retryAfter = response.headers.get('Retry-After');
      const waitTime = (parseInt(retryAfter) || 60) * 1000;
      
      console.log(`Rate limit exceeded, waiting ${waitTime}ms`);
      await this.sleep(waitTime);
      
      // Retry the request
      return this.makeRequest(endpoint, options);
    }

    return response;
  }

  updateRateLimitInfo(response) {
    this.rateLimitInfo = {
      limit: parseInt(response.headers.get('X-RateLimit-Limit')),
      remaining: parseInt(response.headers.get('X-RateLimit-Remaining')),
      resetAt: parseInt(response.headers.get('X-RateLimit-Reset')) * 1000
    };
  }

  shouldWaitForRateLimit() {
    if (!this.rateLimitInfo.remaining) return false;
    
    // If less than 10% of rate limit remaining, consider waiting
    const threshold = this.rateLimitInfo.limit * 0.1;
    return this.rateLimitInfo.remaining < threshold;
  }

  getWaitTime() {
    const now = Date.now();
    const resetTime = this.rateLimitInfo.resetAt;
    const timeUntilReset = resetTime - now;
    
    // Calculate optimal wait time based on remaining requests
    const requestsPerMs = this.rateLimitInfo.remaining / timeUntilReset;
    return Math.min(1000 / requestsPerMs, 5000); // Max 5 second wait
  }

  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}

// Usage
const client = new RateLimitedApiClient('https://api.hotelreviews.com/api/v1', 'your-token');

// Make requests with automatic rate limit handling
const response = await client.makeRequest('/reviews');
```

### Python Example

```python
import time
import requests
from datetime import datetime

class RateLimitedApiClient:
    def __init__(self, base_url, api_key):
        self.base_url = base_url
        self.api_key = api_key
        self.session = requests.Session()
        self.session.headers.update({
            'Authorization': f'Bearer {api_key}',
            'Content-Type': 'application/json'
        })
        self.rate_limit_info = {
            'limit': None,
            'remaining': None,
            'reset_at': None
        }
    
    def make_request(self, method, endpoint, **kwargs):
        # Check if we should throttle
        if self.should_throttle():
            wait_time = self.get_wait_time()
            print(f"Throttling request, waiting {wait_time:.2f}s")
            time.sleep(wait_time)
        
        url = f"{self.base_url}{endpoint}"
        response = self.session.request(method, url, **kwargs)
        
        # Update rate limit info
        self.update_rate_limit_info(response)
        
        # Handle rate limit exceeded
        if response.status_code == 429:
            retry_after = int(response.headers.get('Retry-After', 60))
            print(f"Rate limit exceeded, waiting {retry_after}s")
            time.sleep(retry_after)
            return self.make_request(method, endpoint, **kwargs)
        
        return response
    
    def update_rate_limit_info(self, response):
        headers = response.headers
        self.rate_limit_info = {
            'limit': int(headers.get('X-RateLimit-Limit', 0)),
            'remaining': int(headers.get('X-RateLimit-Remaining', 0)),
            'reset_at': int(headers.get('X-RateLimit-Reset', 0))
        }
    
    def should_throttle(self):
        if not self.rate_limit_info['remaining']:
            return False
        
        # Throttle if less than 20% remaining
        threshold = self.rate_limit_info['limit'] * 0.2
        return self.rate_limit_info['remaining'] < threshold
    
    def get_wait_time(self):
        now = time.time()
        reset_time = self.rate_limit_info['reset_at']
        time_until_reset = max(reset_time - now, 1)
        
        # Calculate delay to spread remaining requests
        remaining = self.rate_limit_info['remaining']
        if remaining > 0:
            return min(time_until_reset / remaining, 5.0)
        return 5.0

# Usage with automatic retry and throttling
client = RateLimitedApiClient('https://api.hotelreviews.com/api/v1', 'your-token')

# Make requests
response = client.make_request('GET', '/reviews', params={'limit': 20})
```

### Go Example

```go
package main

import (
    "fmt"
    "net/http"
    "strconv"
    "sync"
    "time"
)

type RateLimitInfo struct {
    Limit     int
    Remaining int
    ResetAt   time.Time
    mu        sync.Mutex
}

type RateLimitedClient struct {
    BaseURL       string
    APIKey        string
    Client        *http.Client
    RateLimitInfo *RateLimitInfo
}

func NewRateLimitedClient(baseURL, apiKey string) *RateLimitedClient {
    return &RateLimitedClient{
        BaseURL: baseURL,
        APIKey:  apiKey,
        Client:  &http.Client{Timeout: 30 * time.Second},
        RateLimitInfo: &RateLimitInfo{},
    }
}

func (c *RateLimitedClient) DoRequest(req *http.Request) (*http.Response, error) {
    // Add authentication
    req.Header.Set("Authorization", "Bearer "+c.APIKey)
    
    // Check if we should throttle
    if c.shouldThrottle() {
        waitTime := c.getWaitTime()
        fmt.Printf("Throttling request, waiting %v\n", waitTime)
        time.Sleep(waitTime)
    }
    
    // Make request
    resp, err := c.Client.Do(req)
    if err != nil {
        return nil, err
    }
    
    // Update rate limit info
    c.updateRateLimitInfo(resp)
    
    // Handle rate limit exceeded
    if resp.StatusCode == http.StatusTooManyRequests {
        retryAfter := resp.Header.Get("Retry-After")
        waitSeconds, _ := strconv.Atoi(retryAfter)
        if waitSeconds == 0 {
            waitSeconds = 60
        }
        
        fmt.Printf("Rate limit exceeded, waiting %d seconds\n", waitSeconds)
        time.Sleep(time.Duration(waitSeconds) * time.Second)
        
        // Retry request
        return c.DoRequest(req)
    }
    
    return resp, nil
}

func (c *RateLimitedClient) updateRateLimitInfo(resp *http.Response) {
    c.RateLimitInfo.mu.Lock()
    defer c.RateLimitInfo.mu.Unlock()
    
    if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
        c.RateLimitInfo.Limit, _ = strconv.Atoi(limit)
    }
    
    if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
        c.RateLimitInfo.Remaining, _ = strconv.Atoi(remaining)
    }
    
    if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
        resetUnix, _ := strconv.ParseInt(reset, 10, 64)
        c.RateLimitInfo.ResetAt = time.Unix(resetUnix, 0)
    }
}

func (c *RateLimitedClient) shouldThrottle() bool {
    c.RateLimitInfo.mu.Lock()
    defer c.RateLimitInfo.mu.Unlock()
    
    if c.RateLimitInfo.Limit == 0 {
        return false
    }
    
    // Throttle if less than 20% remaining
    threshold := int(float64(c.RateLimitInfo.Limit) * 0.2)
    return c.RateLimitInfo.Remaining < threshold
}

func (c *RateLimitedClient) getWaitTime() time.Duration {
    c.RateLimitInfo.mu.Lock()
    defer c.RateLimitInfo.mu.Unlock()
    
    timeUntilReset := time.Until(c.RateLimitInfo.ResetAt)
    if timeUntilReset <= 0 {
        return 0
    }
    
    // Spread remaining requests evenly
    if c.RateLimitInfo.Remaining > 0 {
        waitTime := timeUntilReset / time.Duration(c.RateLimitInfo.Remaining)
        // Cap at 5 seconds
        if waitTime > 5*time.Second {
            return 5 * time.Second
        }
        return waitTime
    }
    
    return 5 * time.Second
}
```

## Best Practices

### 1. Monitor Rate Limit Headers

Always check rate limit headers in responses:

```javascript
function checkRateLimit(response) {
  const remaining = response.headers.get('X-RateLimit-Remaining');
  const limit = response.headers.get('X-RateLimit-Limit');
  
  const percentageUsed = ((limit - remaining) / limit) * 100;
  
  if (percentageUsed > 80) {
    console.warn(`Rate limit warning: ${percentageUsed.toFixed(1)}% used`);
  }
  
  return {
    limit: parseInt(limit),
    remaining: parseInt(remaining),
    percentageUsed
  };
}
```

### 2. Implement Exponential Backoff

For repeated rate limit errors:

```javascript
async function requestWithBackoff(url, options, maxRetries = 3) {
  let lastError;
  
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch(url, options);
      
      if (response.status !== 429) {
        return response;
      }
      
      const retryAfter = parseInt(response.headers.get('Retry-After') || '60');
      const backoffTime = Math.min(retryAfter * 1000, 60000 * Math.pow(2, i));
      
      console.log(`Rate limited, retry ${i + 1}/${maxRetries} after ${backoffTime}ms`);
      await new Promise(resolve => setTimeout(resolve, backoffTime));
      
    } catch (error) {
      lastError = error;
      const backoffTime = 1000 * Math.pow(2, i);
      await new Promise(resolve => setTimeout(resolve, backoffTime));
    }
  }
  
  throw lastError || new Error('Max retries exceeded');
}
```

### 3. Implement Request Queuing

Queue requests to avoid hitting limits:

```javascript
class RequestQueue {
  constructor(maxConcurrent = 5, minDelay = 100) {
    this.queue = [];
    this.active = 0;
    this.maxConcurrent = maxConcurrent;
    this.minDelay = minDelay;
    this.lastRequestTime = 0;
  }

  async add(requestFn) {
    return new Promise((resolve, reject) => {
      this.queue.push({ requestFn, resolve, reject });
      this.processQueue();
    });
  }

  async processQueue() {
    if (this.active >= this.maxConcurrent || this.queue.length === 0) {
      return;
    }

    // Ensure minimum delay between requests
    const now = Date.now();
    const timeSinceLastRequest = now - this.lastRequestTime;
    if (timeSinceLastRequest < this.minDelay) {
      setTimeout(() => this.processQueue(), this.minDelay - timeSinceLastRequest);
      return;
    }

    const { requestFn, resolve, reject } = this.queue.shift();
    this.active++;
    this.lastRequestTime = Date.now();

    try {
      const result = await requestFn();
      resolve(result);
    } catch (error) {
      reject(error);
    } finally {
      this.active--;
      this.processQueue();
    }
  }
}

// Usage
const queue = new RequestQueue(5, 200); // Max 5 concurrent, 200ms between requests

async function getReview(id) {
  return queue.add(() => 
    fetch(`/api/v1/reviews/${id}`, {
      headers: { 'Authorization': 'Bearer token' }
    })
  );
}
```

### 4. Cache Responses

Reduce API calls by caching:

```javascript
class CachedApiClient {
  constructor(apiClient, cacheTime = 60000) {
    this.apiClient = apiClient;
    this.cache = new Map();
    this.cacheTime = cacheTime;
  }

  async get(endpoint, options = {}) {
    const cacheKey = `${endpoint}:${JSON.stringify(options)}`;
    const cached = this.cache.get(cacheKey);
    
    if (cached && Date.now() - cached.timestamp < this.cacheTime) {
      console.log(`Cache hit for ${endpoint}`);
      return cached.data;
    }
    
    const response = await this.apiClient.get(endpoint, options);
    const data = await response.json();
    
    this.cache.set(cacheKey, {
      data,
      timestamp: Date.now()
    });
    
    // Clean old cache entries
    this.cleanCache();
    
    return data;
  }

  cleanCache() {
    const now = Date.now();
    for (const [key, value] of this.cache.entries()) {
      if (now - value.timestamp > this.cacheTime) {
        this.cache.delete(key);
      }
    }
  }
}
```

## Rate Limit Strategies

### 1. Batch Operations

Instead of individual requests, use bulk endpoints:

```javascript
// Bad: Multiple individual requests
for (const review of reviews) {
  await createReview(review); // 100 requests for 100 reviews
}

// Good: Single bulk request
await createReviewsBulk(reviews); // 1 request for 100 reviews
```

### 2. Request Prioritization

Prioritize critical requests when approaching limits:

```javascript
class PriorityRequestQueue {
  constructor(rateLimitInfo) {
    this.queues = {
      high: [],
      medium: [],
      low: []
    };
    this.rateLimitInfo = rateLimitInfo;
  }

  async add(requestFn, priority = 'medium') {
    // If plenty of rate limit remaining, execute immediately
    if (this.rateLimitInfo.remaining > this.rateLimitInfo.limit * 0.5) {
      return requestFn();
    }
    
    // Otherwise, queue based on priority
    return new Promise((resolve, reject) => {
      this.queues[priority].push({ requestFn, resolve, reject });
      this.processQueues();
    });
  }

  async processQueues() {
    // Process high priority first
    for (const priority of ['high', 'medium', 'low']) {
      const queue = this.queues[priority];
      if (queue.length > 0 && this.rateLimitInfo.remaining > 0) {
        const { requestFn, resolve, reject } = queue.shift();
        try {
          const result = await requestFn();
          resolve(result);
        } catch (error) {
          reject(error);
        }
      }
    }
  }
}
```

### 3. Webhook Integration

For heavy operations, use webhooks instead of polling:

```javascript
// Instead of polling for status
async function pollForCompletion(jobId) {
  while (true) {
    const status = await checkStatus(jobId); // Uses rate limit
    if (status.complete) break;
    await sleep(5000);
  }
}

// Use webhooks
async function processWithWebhook(data, webhookUrl) {
  const response = await startProcessing(data, {
    webhook_url: webhookUrl,
    webhook_events: ['completed', 'failed']
  });
  // No polling needed - webhook will notify when complete
}
```

## Monitoring and Alerts

### Set Up Rate Limit Monitoring

```javascript
class RateLimitMonitor {
  constructor(alertThreshold = 0.8) {
    this.alertThreshold = alertThreshold;
    this.alerts = [];
  }

  checkResponse(response) {
    const limit = parseInt(response.headers.get('X-RateLimit-Limit'));
    const remaining = parseInt(response.headers.get('X-RateLimit-Remaining'));
    const usage = (limit - remaining) / limit;

    if (usage > this.alertThreshold) {
      this.sendAlert({
        level: usage > 0.95 ? 'critical' : 'warning',
        message: `Rate limit usage at ${(usage * 100).toFixed(1)}%`,
        limit,
        remaining,
        timestamp: new Date().toISOString()
      });
    }

    return { limit, remaining, usage };
  }

  sendAlert(alert) {
    this.alerts.push(alert);
    
    // Send to monitoring service
    if (alert.level === 'critical') {
      console.error(`CRITICAL: ${alert.message}`);
      // Send to PagerDuty, Slack, etc.
    } else {
      console.warn(`WARNING: ${alert.message}`);
    }
  }

  getMetrics() {
    return {
      alerts: this.alerts,
      criticalAlerts: this.alerts.filter(a => a.level === 'critical').length,
      warningAlerts: this.alerts.filter(a => a.level === 'warning').length
    };
  }
}
```

## Rate Limit Increase

If you need higher rate limits:

1. **Premium Plans**: Upgrade to a premium account for higher limits
2. **Enterprise**: Contact sales for custom limits
3. **Optimization**: Review your integration for optimization opportunities

Contact gkbiswas@gmail.com with:
- Current usage patterns
- Expected growth
- Use case description
- Technical contact information