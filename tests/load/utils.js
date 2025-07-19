import http from 'k6/http';
import { check, sleep } from 'k6';
import { config, auth, createHeaders, validateSuccessResponse } from './config.js';

// Authentication utilities
export class AuthManager {
  constructor() {
    this.tokens = {
      user: null,
      admin: null,
      expiresAt: null,
    };
  }

  // Login and get JWT token
  login(credentials, isAdmin = false) {
    const loginUrl = `${config.baseUrl}/api/v1/auth/login`;
    const payload = JSON.stringify({
      email: credentials.email,
      password: credentials.password,
    });

    const response = http.post(loginUrl, payload, {
      headers: createHeaders(),
      tags: { endpoint: 'auth', operation: 'login' }
    });

    if (response.status === 200) {
      const body = JSON.parse(response.body);
      if (body.success && body.data && body.data.access_token) {
        const token = body.data.access_token;
        const expiresAt = new Date(Date.now() + (body.data.expires_in || 3600) * 1000);
        
        if (isAdmin) {
          this.tokens.admin = token;
        } else {
          this.tokens.user = token;
        }
        this.tokens.expiresAt = expiresAt;
        
        return token;
      }
    }
    
    console.error(`Login failed: ${response.status} - ${response.body}`);
    return null;
  }

  // Get valid token (refresh if needed)
  getValidToken(isAdmin = false) {
    const tokenKey = isAdmin ? 'admin' : 'user';
    const token = this.tokens[tokenKey];
    
    // Check if token exists and is not expired
    if (token && this.tokens.expiresAt && new Date() < this.tokens.expiresAt) {
      return token;
    }
    
    // Login to get new token
    const credentials = isAdmin ? auth.adminUser : auth.testUser;
    return this.login(credentials, isAdmin);
  }

  // Register a new user (for testing)
  register(userData) {
    const registerUrl = `${config.baseUrl}/api/v1/auth/register`;
    const payload = JSON.stringify(userData);

    const response = http.post(registerUrl, payload, {
      headers: createHeaders(),
      tags: { endpoint: 'auth', operation: 'register' }
    });

    return validateSuccessResponse(response);
  }

  // Logout
  logout(token) {
    const logoutUrl = `${config.baseUrl}/api/v1/auth/logout`;
    
    const response = http.post(logoutUrl, '', {
      headers: createHeaders(token),
      tags: { endpoint: 'auth', operation: 'logout' }
    });

    return response.status === 200;
  }
}

// Data management utilities
export class DataManager {
  constructor(authManager) {
    this.authManager = authManager;
    this.createdResources = {
      hotels: [],
      reviews: [],
      providers: [],
    };
  }

  // Create a test hotel
  createHotel(hotelData) {
    const token = this.authManager.getValidToken(true); // Admin token needed
    if (!token) return null;

    const createUrl = `${config.baseUrl}/api/v1/hotels`;
    const payload = JSON.stringify(hotelData);

    const response = http.post(createUrl, payload, {
      headers: createHeaders(token),
      tags: { endpoint: 'hotels', operation: 'create' }
    });

    if (response.status === 201) {
      const body = JSON.parse(response.body);
      if (body.success && body.data) {
        this.createdResources.hotels.push(body.data.id);
        return body.data;
      }
    }

    return null;
  }

  // Create a test review
  createReview(reviewData) {
    const token = this.authManager.getValidToken();
    if (!token) return null;

    const createUrl = `${config.baseUrl}/api/v1/reviews`;
    const payload = JSON.stringify(reviewData);

    const response = http.post(createUrl, payload, {
      headers: createHeaders(token),
      tags: { endpoint: 'reviews', operation: 'create' }
    });

    if (response.status === 201) {
      const body = JSON.parse(response.body);
      if (body.success && body.data) {
        this.createdResources.reviews.push(body.data.id);
        return body.data;
      }
    }

    return null;
  }

  // Create a test provider
  createProvider(providerData) {
    const token = this.authManager.getValidToken(true); // Admin token needed
    if (!token) return null;

    const createUrl = `${config.baseUrl}/api/v1/providers`;
    const payload = JSON.stringify(providerData);

    const response = http.post(createUrl, payload, {
      headers: createHeaders(token),
      tags: { endpoint: 'providers', operation: 'create' }
    });

    if (response.status === 201) {
      const body = JSON.parse(response.body);
      if (body.success && body.data) {
        this.createdResources.providers.push(body.data.id);
        return body.data;
      }
    }

    return null;
  }

  // Get random existing hotel ID
  getRandomHotelId() {
    const token = this.authManager.getValidToken();
    if (!token) return null;

    const listUrl = `${config.baseUrl}/api/v1/hotels?limit=10`;
    const response = http.get(listUrl, {
      headers: createHeaders(token),
      tags: { endpoint: 'hotels', operation: 'list' }
    });

    if (response.status === 200) {
      const body = JSON.parse(response.body);
      if (body.success && body.data && body.data.length > 0) {
        const randomIndex = Math.floor(Math.random() * body.data.length);
        return body.data[randomIndex].id;
      }
    }

    return null;
  }

  // Cleanup created resources
  cleanup() {
    const token = this.authManager.getValidToken(true);
    if (!token) return;

    // Cleanup reviews
    this.createdResources.reviews.forEach(id => {
      http.del(`${config.baseUrl}/api/v1/reviews/${id}`, null, {
        headers: createHeaders(token),
        tags: { endpoint: 'reviews', operation: 'delete' }
      });
    });

    // Cleanup hotels
    this.createdResources.hotels.forEach(id => {
      http.del(`${config.baseUrl}/api/v1/hotels/${id}`, null, {
        headers: createHeaders(token),
        tags: { endpoint: 'hotels', operation: 'delete' }
      });
    });

    // Cleanup providers
    this.createdResources.providers.forEach(id => {
      http.del(`${config.baseUrl}/api/v1/providers/${id}`, null, {
        headers: createHeaders(token),
        tags: { endpoint: 'providers', operation: 'delete' }
      });
    });
  }
}

// Performance monitoring utilities
export class PerformanceMonitor {
  constructor() {
    this.metrics = {
      requests: 0,
      errors: 0,
      responseTimes: [],
      circuitBreakerActivations: 0,
      dbOperations: 0,
    };
  }

  // Record request metrics
  recordRequest(response, operation = 'unknown') {
    this.metrics.requests++;
    this.metrics.responseTimes.push(response.timings.duration);

    if (response.status >= 400) {
      this.metrics.errors++;
    }

    // Check for circuit breaker headers
    if (response.headers['X-Circuit-Breaker-State'] === 'open') {
      this.metrics.circuitBreakerActivations++;
    }

    // Custom metrics for monitoring
    if (response.timings.duration > 1000) {
      console.warn(`Slow response detected: ${operation} took ${response.timings.duration}ms`);
    }
  }

  // Record database operation
  recordDbOperation(duration) {
    this.metrics.dbOperations++;
    // Add custom metric for database operations
    // This would be integrated with the actual monitoring system
  }

  // Get performance summary
  getSummary() {
    const avgResponseTime = this.metrics.responseTimes.length > 0 
      ? this.metrics.responseTimes.reduce((a, b) => a + b, 0) / this.metrics.responseTimes.length 
      : 0;

    return {
      totalRequests: this.metrics.requests,
      totalErrors: this.metrics.errors,
      errorRate: this.metrics.requests > 0 ? this.metrics.errors / this.metrics.requests : 0,
      avgResponseTime: avgResponseTime,
      circuitBreakerActivations: this.metrics.circuitBreakerActivations,
      dbOperations: this.metrics.dbOperations,
    };
  }
}

// File processing utilities
export class FileProcessor {
  constructor(authManager) {
    this.authManager = authManager;
  }

  // Upload a CSV file for processing
  uploadCsvFile(csvContent, filename = 'test-reviews.csv') {
    const token = this.authManager.getValidToken();
    if (!token) return null;

    const uploadUrl = `${config.baseUrl}/api/v1/reviews/upload`;
    
    // Create form data
    const formData = {
      file: http.file(csvContent, filename, 'text/csv'),
      provider_id: 'test-provider-id',
    };

    const response = http.post(uploadUrl, formData, {
      headers: {
        'Authorization': `Bearer ${token}`,
        // Don't set Content-Type for multipart/form-data, k6 will set it automatically
      },
      tags: { endpoint: 'file_processing', operation: 'upload' }
    });

    return response;
  }

  // Check processing status
  checkProcessingStatus(processId) {
    const token = this.authManager.getValidToken();
    if (!token) return null;

    const statusUrl = `${config.baseUrl}/api/v1/reviews/processing/${processId}`;
    
    const response = http.get(statusUrl, {
      headers: createHeaders(token),
      tags: { endpoint: 'file_processing', operation: 'status' }
    });

    return response;
  }

  // Generate large CSV content for testing
  generateLargeCsvContent(numRows = 1000) {
    let csvContent = 'hotel_id,reviewer_name,rating,title,comment,review_date,language\n';
    
    for (let i = 0; i < numRows; i++) {
      const hotelId = `hotel-${Math.floor(Math.random() * 100) + 1}`;
      const reviewerName = `Reviewer ${i + 1}`;
      const rating = (Math.random() * 4 + 1).toFixed(1); // 1.0 to 5.0
      const title = `Review Title ${i + 1}`;
      const comment = `This is a test comment for review ${i + 1}. Lorem ipsum dolor sit amet.`;
      const reviewDate = new Date(Date.now() - Math.random() * 365 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
      const language = 'en';
      
      csvContent += `${hotelId},"${reviewerName}",${rating},"${title}","${comment}",${reviewDate},${language}\n`;
    }
    
    return csvContent;
  }
}

// Load balancing and routing utilities
export class LoadBalancer {
  constructor(endpoints) {
    this.endpoints = endpoints || [config.baseUrl];
    this.currentIndex = 0;
  }

  // Get next endpoint in round-robin fashion
  getNextEndpoint() {
    const endpoint = this.endpoints[this.currentIndex];
    this.currentIndex = (this.currentIndex + 1) % this.endpoints.length;
    return endpoint;
  }

  // Health check all endpoints
  healthCheck() {
    const results = [];
    
    this.endpoints.forEach((endpoint, index) => {
      const healthUrl = `${endpoint}/health`;
      const response = http.get(healthUrl, {
        timeout: '5s',
        tags: { endpoint: 'health', instance: index }
      });
      
      results.push({
        endpoint: endpoint,
        healthy: response.status === 200,
        responseTime: response.timings.duration,
      });
    });
    
    return results;
  }
}

// Database testing utilities
export class DatabaseTester {
  constructor(authManager) {
    this.authManager = authManager;
  }

  // Test database query performance
  testDatabaseQuery(queryType, parameters = {}) {
    const token = this.authManager.getValidToken();
    if (!token) return null;

    let url;
    let method = 'GET';
    let payload = null;

    switch (queryType) {
      case 'complex_search':
        url = `${config.baseUrl}/api/v1/reviews/search?query=${parameters.query || 'test'}&limit=${parameters.limit || 50}`;
        break;
      case 'aggregation':
        url = `${config.baseUrl}/api/v1/reviews/statistics?hotel_id=${parameters.hotel_id || ''}`;
        break;
      case 'bulk_insert':
        url = `${config.baseUrl}/api/v1/reviews/bulk`;
        method = 'POST';
        payload = JSON.stringify(parameters.data || []);
        break;
      case 'concurrent_reads':
        url = `${config.baseUrl}/api/v1/reviews?limit=${parameters.limit || 100}&offset=${parameters.offset || 0}`;
        break;
      default:
        return null;
    }

    const options = {
      headers: createHeaders(token),
      tags: { endpoint: 'database', operation: queryType }
    };

    const response = method === 'POST' 
      ? http.post(url, payload, options)
      : http.get(url, options);

    return response;
  }

  // Simulate database connection pool exhaustion
  simulateConnectionPoolStress(concurrentRequests = 50) {
    const requests = [];
    const token = this.authManager.getValidToken();
    
    if (!token) return [];

    for (let i = 0; i < concurrentRequests; i++) {
      const url = `${config.baseUrl}/api/v1/reviews?limit=100&offset=${i * 100}`;
      requests.push([
        'GET',
        url,
        null,
        {
          headers: createHeaders(token),
          tags: { endpoint: 'database', operation: 'connection_stress' }
        }
      ]);
    }

    // Execute all requests concurrently
    return http.batch(requests);
  }
}

// Cache testing utilities
export class CacheTester {
  constructor(authManager) {
    this.authManager = authManager;
  }

  // Test cache hit/miss scenarios
  testCachePerformance(resourceType, resourceId) {
    const token = this.authManager.getValidToken();
    if (!token) return null;

    const url = `${config.baseUrl}/api/v1/${resourceType}/${resourceId}`;
    
    // First request (likely cache miss)
    const firstResponse = http.get(url, {
      headers: createHeaders(token),
      tags: { endpoint: resourceType, operation: 'cache_miss' }
    });

    // Small delay to ensure first request completes
    sleep(0.1);

    // Second request (should be cache hit)
    const secondResponse = http.get(url, {
      headers: createHeaders(token),
      tags: { endpoint: resourceType, operation: 'cache_hit' }
    });

    return {
      firstResponse,
      secondResponse,
      cacheEffective: secondResponse.timings.duration < firstResponse.timings.duration,
    };
  }

  // Test cache invalidation
  testCacheInvalidation(resourceType, resourceId, updateData) {
    const token = this.authManager.getValidToken();
    if (!token) return null;

    const getUrl = `${config.baseUrl}/api/v1/${resourceType}/${resourceId}`;
    const updateUrl = `${config.baseUrl}/api/v1/${resourceType}/${resourceId}`;

    // Get initial cached data
    const initialResponse = http.get(getUrl, {
      headers: createHeaders(token),
      tags: { endpoint: resourceType, operation: 'cache_before_update' }
    });

    // Update the resource (should invalidate cache)
    const updateResponse = http.put(updateUrl, JSON.stringify(updateData), {
      headers: createHeaders(token),
      tags: { endpoint: resourceType, operation: 'update' }
    });

    sleep(0.1);

    // Get updated data (should be fresh, not cached)
    const updatedResponse = http.get(getUrl, {
      headers: createHeaders(token),
      tags: { endpoint: resourceType, operation: 'cache_after_update' }
    });

    return {
      initialResponse,
      updateResponse,
      updatedResponse,
    };
  }
}

// Utility function to wait for async operations
export function waitForAsyncOperation(checkFunction, maxAttempts = 30, intervalMs = 1000) {
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    if (checkFunction()) {
      return true;
    }
    sleep(intervalMs / 1000); // k6 sleep expects seconds
  }
  return false;
}

// Utility function to generate random test data
export function generateRandomReviewData(hotelId) {
  const titles = [
    'Great stay!',
    'Excellent service',
    'Average experience',
    'Could be better',
    'Outstanding hotel',
    'Disappointing visit',
    'Perfect location',
    'Good value for money'
  ];

  const comments = [
    'Had a wonderful time at this hotel. Staff was friendly and rooms were clean.',
    'The service was exceptional and the facilities were top-notch.',
    'Hotel was okay, nothing special but met basic expectations.',
    'There were some issues with the room but staff resolved them quickly.',
    'Cannot recommend this place enough. Everything was perfect!',
    'Expected more for the price. Room was dated and service was slow.',
    'Location is perfect for exploring the city. Hotel itself is decent.',
    'Great amenities and fair pricing. Would definitely stay again.'
  ];

  return {
    hotel_id: hotelId,
    rating: Math.floor(Math.random() * 5) + 1,
    title: titles[Math.floor(Math.random() * titles.length)],
    comment: comments[Math.floor(Math.random() * comments.length)],
    review_date: new Date(Date.now() - Math.random() * 365 * 24 * 60 * 60 * 1000).toISOString(),
    language: 'en',
  };
}