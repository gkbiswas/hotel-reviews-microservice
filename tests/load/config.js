import { check } from 'k6';
import http from 'k6/http';

// Configuration for load testing scenarios
export const config = {
  // Base URL for the API
  baseUrl: __ENV.BASE_URL || 'http://localhost:8080',
  
  // Test scenarios configuration
  scenarios: {
    // Normal load - typical usage patterns
    normal_load: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '2m', target: 10 },   // Ramp up to 10 users over 2 minutes
        { duration: '5m', target: 10 },   // Stay at 10 users for 5 minutes
        { duration: '2m', target: 0 },    // Ramp down to 0 users over 2 minutes
      ],
      gracefulRampDown: '30s',
      tags: { test_type: 'normal_load' },
    },
    
    // Peak load - high traffic simulation
    peak_load: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '3m', target: 50 },   // Ramp up to 50 users over 3 minutes
        { duration: '10m', target: 50 },  // Stay at 50 users for 10 minutes
        { duration: '3m', target: 100 },  // Spike to 100 users
        { duration: '5m', target: 100 },  // Maintain peak for 5 minutes
        { duration: '5m', target: 0 },    // Ramp down to 0 users
      ],
      gracefulRampDown: '1m',
      tags: { test_type: 'peak_load' },
    },
    
    // Stress test - beyond normal capacity
    stress_test: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '5m', target: 100 },  // Ramp up to 100 users
        { duration: '10m', target: 200 }, // Increase to 200 users
        { duration: '5m', target: 300 },  // Stress with 300 users
        { duration: '10m', target: 300 }, // Maintain stress
        { duration: '5m', target: 0 },    // Ramp down
      ],
      gracefulRampDown: '2m',
      tags: { test_type: 'stress_test' },
    },
    
    // Spike test - sudden traffic spikes
    spike_test: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '1m', target: 10 },   // Normal load
        { duration: '1m', target: 200 },  // Sudden spike
        { duration: '3m', target: 200 },  // Maintain spike
        { duration: '1m', target: 10 },   // Return to normal
        { duration: '1m', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'spike_test' },
    },
    
    // Database performance test
    database_load: {
      executor: 'constant-vus',
      vus: 50,
      duration: '10m',
      tags: { test_type: 'database_load' },
    },
    
    // File processing test
    file_processing: {
      executor: 'per-vu-iterations',
      vus: 20,
      iterations: 5,
      maxDuration: '30m',
      tags: { test_type: 'file_processing' },
    }
  },
  
  // Performance thresholds
  thresholds: {
    // HTTP request duration thresholds
    'http_req_duration': ['p(95)<500', 'p(99)<1000'],
    'http_req_duration{test_type:normal_load}': ['p(95)<300'],
    'http_req_duration{test_type:peak_load}': ['p(95)<800'],
    'http_req_duration{test_type:stress_test}': ['p(95)<2000'],
    
    // HTTP request failure rate thresholds
    'http_req_failed': ['rate<0.05'], // Less than 5% failures
    'http_req_failed{test_type:normal_load}': ['rate<0.01'], // Less than 1% for normal load
    'http_req_failed{test_type:peak_load}': ['rate<0.03'], // Less than 3% for peak load
    
    // Request rate thresholds
    'http_reqs': ['rate>100'], // At least 100 requests per second
    
    // Specific endpoint thresholds
    'http_req_duration{endpoint:reviews}': ['p(95)<400'],
    'http_req_duration{endpoint:hotels}': ['p(95)<300'],
    'http_req_duration{endpoint:auth}': ['p(95)<200'],
    
    // Database operation thresholds
    'database_operations': ['p(95)<200'],
    
    // Circuit breaker thresholds
    'circuit_breaker_open': ['rate<0.1'], // Less than 10% circuit breaker activations
  },
  
  // Monitoring and metrics collection
  monitoring: {
    influxdb: {
      enabled: __ENV.INFLUXDB_ENABLED === 'true',
      url: __ENV.INFLUXDB_URL || 'http://localhost:8086',
      database: __ENV.INFLUXDB_DATABASE || 'k6_results',
      tags: {
        environment: __ENV.ENVIRONMENT || 'test',
        version: __ENV.VERSION || 'latest',
      }
    },
    
    prometheus: {
      enabled: __ENV.PROMETHEUS_ENABLED === 'true',
      pushgateway: __ENV.PROMETHEUS_PUSHGATEWAY || 'http://localhost:9091',
    }
  }
};

// Authentication configuration
export const auth = {
  // Test user credentials
  testUser: {
    username: __ENV.TEST_USERNAME || 'testuser',
    password: __ENV.TEST_PASSWORD || 'testpass123',
    email: __ENV.TEST_EMAIL || 'test@example.com',
  },
  
  // Admin user credentials
  adminUser: {
    username: __ENV.ADMIN_USERNAME || 'admin',
    password: __ENV.ADMIN_PASSWORD || 'adminpass123',
    email: __ENV.ADMIN_EMAIL || 'admin@example.com',
  },
  
  // JWT token cache
  tokens: {
    user: null,
    admin: null,
    expiresAt: null,
  }
};

// Test data configuration
export const testData = {
  // Sample hotels for testing
  hotels: [
    {
      name: 'Test Hotel 1',
      address: '123 Test Street',
      city: 'Test City',
      country: 'Test Country',
      star_rating: 4,
    },
    {
      name: 'Test Hotel 2',
      address: '456 Another Street',
      city: 'Another City',
      country: 'Another Country',
      star_rating: 5,
    }
  ],
  
  // Sample reviews for testing
  reviews: [
    {
      rating: 4.5,
      title: 'Great stay!',
      comment: 'Had a wonderful time at this hotel. Staff was friendly and rooms were clean.',
      language: 'en',
    },
    {
      rating: 3.5,
      title: 'Average experience',
      comment: 'Hotel was okay, nothing special but met basic expectations.',
      language: 'en',
    },
    {
      rating: 5.0,
      title: 'Excellent!',
      comment: 'Outstanding service and facilities. Highly recommended!',
      language: 'en',
    }
  ],
  
  // Sample provider data
  providers: [
    {
      name: 'Booking.com',
      base_url: 'https://booking.com',
      is_active: true,
    },
    {
      name: 'Expedia',
      base_url: 'https://expedia.com',
      is_active: true,
    }
  ]
};

// File processing configuration
export const fileProcessing = {
  // Sample CSV content for testing file uploads
  sampleCsvContent: `hotel_id,reviewer_name,rating,title,comment,review_date,language
123e4567-e89b-12d3-a456-426614174000,John Doe,4.5,"Great stay","Had a wonderful time",2024-01-15,en
223e4567-e89b-12d3-a456-426614174001,Jane Smith,3.5,"Average","It was okay",2024-01-16,en
323e4567-e89b-12d3-a456-426614174002,Bob Johnson,5.0,"Excellent","Amazing service",2024-01-17,en`,
  
  // File sizes for testing (in bytes)
  fileSizes: {
    small: 1024 * 10,      // 10KB
    medium: 1024 * 100,    // 100KB
    large: 1024 * 1024,    // 1MB
    xlarge: 1024 * 1024 * 5 // 5MB
  }
};

// Database configuration for direct testing
export const database = {
  // Connection details (if testing direct DB access)
  postgres: {
    host: __ENV.DB_HOST || 'localhost',
    port: __ENV.DB_PORT || '5432',
    database: __ENV.DB_NAME || 'hotel_reviews_test',
    user: __ENV.DB_USER || 'test_user',
    password: __ENV.DB_PASSWORD || 'test_password',
  },
  
  // Redis configuration
  redis: {
    host: __ENV.REDIS_HOST || 'localhost',
    port: __ENV.REDIS_PORT || '6379',
    password: __ENV.REDIS_PASSWORD || '',
  }
};

// Utility functions
export function getRandomElement(array) {
  return array[Math.floor(Math.random() * array.length)];
}

export function generateRandomId() {
  return Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15);
}

export function createHeaders(token = null) {
  const headers = {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  };
  
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  
  return headers;
}

// Response validation helpers
export function validateResponse(response, expectedStatus = 200) {
  return check(response, {
    [`status is ${expectedStatus}`]: (r) => r.status === expectedStatus,
    'response has body': (r) => r.body && r.body.length > 0,
    'response is valid JSON': (r) => {
      try {
        JSON.parse(r.body);
        return true;
      } catch (e) {
        return false;
      }
    },
    'response time < 2s': (r) => r.timings.duration < 2000,
  });
}

export function validateSuccessResponse(response) {
  const result = validateResponse(response);
  
  if (response.status === 200) {
    const body = JSON.parse(response.body);
    return check(body, {
      'response has success field': (b) => 'success' in b,
      'response is successful': (b) => b.success === true,
      'response has data': (b) => 'data' in b,
      'response has timestamp': (b) => 'timestamp' in b,
    }) && result;
  }
  
  return result;
}

export function validateErrorResponse(response, expectedStatus) {
  const result = validateResponse(response, expectedStatus);
  
  const body = JSON.parse(response.body);
  return check(body, {
    'error response has success field': (b) => 'success' in b,
    'error response is not successful': (b) => b.success === false,
    'error response has error message': (b) => 'error' in b && b.error.length > 0,
    'error response has error code': (b) => 'error_code' in b,
  }) && result;
}