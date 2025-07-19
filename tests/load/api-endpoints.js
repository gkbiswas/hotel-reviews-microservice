import http from 'k6/http';
import { check, sleep } from 'k6';
import { config, auth, testData, createHeaders, validateSuccessResponse, validateErrorResponse } from './config.js';
import { AuthManager, DataManager, PerformanceMonitor } from './utils.js';

// Global instances
let authManager;
let dataManager;
let performanceMonitor;

// Setup function - runs once per VU
export function setup() {
  // Initialize global instances
  authManager = new AuthManager();
  dataManager = new DataManager(authManager);
  performanceMonitor = new PerformanceMonitor();

  // Pre-create some test data
  const token = authManager.getValidToken(true);
  if (token) {
    // Create test hotels
    const testHotels = [];
    testData.hotels.forEach(hotelData => {
      const hotel = dataManager.createHotel(hotelData);
      if (hotel) {
        testHotels.push(hotel);
      }
    });

    // Create test providers
    const testProviders = [];
    testData.providers.forEach(providerData => {
      const provider = dataManager.createProvider(providerData);
      if (provider) {
        testProviders.push(provider);
      }
    });

    return {
      hotels: testHotels,
      providers: testProviders,
    };
  }

  return {};
}

// Main test function
export default function(data) {
  // Initialize if not already done
  if (!authManager) {
    authManager = new AuthManager();
    dataManager = new DataManager(authManager);
    performanceMonitor = new PerformanceMonitor();
  }

  // Test different API endpoints based on scenario
  const testType = __ENV.TEST_TYPE || 'mixed';
  
  switch (testType) {
    case 'auth':
      testAuthenticationEndpoints();
      break;
    case 'reviews':
      testReviewEndpoints(data);
      break;
    case 'hotels':
      testHotelEndpoints(data);
      break;
    case 'providers':
      testProviderEndpoints(data);
      break;
    case 'search':
      testSearchEndpoints();
      break;
    default:
      testMixedEndpoints(data);
  }

  // Add some think time between requests
  sleep(Math.random() * 2 + 1); // 1-3 seconds
}

// Authentication endpoint tests
function testAuthenticationEndpoints() {
  const group = 'Authentication';
  
  // Test user registration
  const newUser = {
    username: `testuser_${Date.now()}_${Math.random().toString(36).substring(7)}`,
    email: `test_${Date.now()}@example.com`,
    password: 'Test123!@#',
    first_name: 'Test',
    last_name: 'User',
  };

  const registerResponse = http.post(
    `${config.baseUrl}/api/v1/auth/register`,
    JSON.stringify(newUser),
    {
      headers: createHeaders(),
      tags: { endpoint: 'auth', operation: 'register', group }
    }
  );

  check(registerResponse, {
    'registration successful': (r) => r.status === 201,
  });
  performanceMonitor.recordRequest(registerResponse, 'auth/register');

  // Test user login
  const loginResponse = http.post(
    `${config.baseUrl}/api/v1/auth/login`,
    JSON.stringify({
      email: newUser.email,
      password: newUser.password,
    }),
    {
      headers: createHeaders(),
      tags: { endpoint: 'auth', operation: 'login', group }
    }
  );

  let token = null;
  if (loginResponse.status === 200) {
    const loginBody = JSON.parse(loginResponse.body);
    if (loginBody.success && loginBody.data) {
      token = loginBody.data.access_token;
    }
  }

  check(loginResponse, {
    'login successful': (r) => r.status === 200,
    'token received': () => token !== null,
  });
  performanceMonitor.recordRequest(loginResponse, 'auth/login');

  // Test token refresh (if token was received)
  if (token) {
    const refreshResponse = http.post(
      `${config.baseUrl}/api/v1/auth/refresh`,
      '',
      {
        headers: createHeaders(token),
        tags: { endpoint: 'auth', operation: 'refresh', group }
      }
    );

    check(refreshResponse, {
      'token refresh successful': (r) => r.status === 200,
    });
    performanceMonitor.recordRequest(refreshResponse, 'auth/refresh');

    // Test logout
    const logoutResponse = http.post(
      `${config.baseUrl}/api/v1/auth/logout`,
      '',
      {
        headers: createHeaders(token),
        tags: { endpoint: 'auth', operation: 'logout', group }
      }
    );

    check(logoutResponse, {
      'logout successful': (r) => r.status === 200,
    });
    performanceMonitor.recordRequest(logoutResponse, 'auth/logout');
  }
}

// Review endpoint tests
function testReviewEndpoints(data) {
  const group = 'Reviews';
  const token = authManager.getValidToken();
  
  if (!token) {
    console.error('No valid token for review tests');
    return;
  }

  // Test listing reviews
  const listResponse = http.get(
    `${config.baseUrl}/api/v1/reviews?limit=20&offset=0`,
    {
      headers: createHeaders(token),
      tags: { endpoint: 'reviews', operation: 'list', group }
    }
  );

  check(listResponse, {
    'review list successful': (r) => r.status === 200,
  });
  performanceMonitor.recordRequest(listResponse, 'reviews/list');

  // Test creating a review (if we have hotel data)
  if (data.hotels && data.hotels.length > 0) {
    const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
    const reviewData = {
      hotel_id: hotel.id,
      reviewer_info_id: hotel.id, // Using hotel ID as reviewer for simplicity
      rating: Math.floor(Math.random() * 5) + 1,
      title: 'Load Test Review',
      comment: 'This is a review created during load testing.',
      review_date: new Date().toISOString(),
      language: 'en',
    };

    const createResponse = http.post(
      `${config.baseUrl}/api/v1/reviews`,
      JSON.stringify(reviewData),
      {
        headers: createHeaders(token),
        tags: { endpoint: 'reviews', operation: 'create', group }
      }
    );

    let createdReviewId = null;
    if (createResponse.status === 201) {
      const createBody = JSON.parse(createResponse.body);
      if (createBody.success && createBody.data) {
        createdReviewId = createBody.data.id;
      }
    }

    check(createResponse, {
      'review creation successful': (r) => r.status === 201,
      'review ID returned': () => createdReviewId !== null,
    });
    performanceMonitor.recordRequest(createResponse, 'reviews/create');

    // Test getting the created review
    if (createdReviewId) {
      const getResponse = http.get(
        `${config.baseUrl}/api/v1/reviews/${createdReviewId}`,
        {
          headers: createHeaders(token),
          tags: { endpoint: 'reviews', operation: 'get', group }
        }
      );

      check(getResponse, {
        'review get successful': (r) => r.status === 200,
      });
      performanceMonitor.recordRequest(getResponse, 'reviews/get');

      // Test updating the review
      const updateData = {
        rating: 5,
        title: 'Updated Load Test Review',
        comment: 'This review was updated during load testing.',
      };

      const updateResponse = http.put(
        `${config.baseUrl}/api/v1/reviews/${createdReviewId}`,
        JSON.stringify(updateData),
        {
          headers: createHeaders(token),
          tags: { endpoint: 'reviews', operation: 'update', group }
        }
      );

      check(updateResponse, {
        'review update successful': (r) => r.status === 200,
      });
      performanceMonitor.recordRequest(updateResponse, 'reviews/update');

      // Test deleting the review
      const deleteResponse = http.del(
        `${config.baseUrl}/api/v1/reviews/${createdReviewId}`,
        null,
        {
          headers: createHeaders(token),
          tags: { endpoint: 'reviews', operation: 'delete', group }
        }
      );

      check(deleteResponse, {
        'review deletion successful': (r) => r.status === 200,
      });
      performanceMonitor.recordRequest(deleteResponse, 'reviews/delete');
    }
  }

  // Test getting reviews by hotel
  if (data.hotels && data.hotels.length > 0) {
    const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
    const hotelReviewsResponse = http.get(
      `${config.baseUrl}/api/v1/reviews?hotel_id=${hotel.id}&limit=10`,
      {
        headers: createHeaders(token),
        tags: { endpoint: 'reviews', operation: 'list_by_hotel', group }
      }
    );

    check(hotelReviewsResponse, {
      'hotel reviews list successful': (r) => r.status === 200,
    });
    performanceMonitor.recordRequest(hotelReviewsResponse, 'reviews/list_by_hotel');
  }
}

// Hotel endpoint tests
function testHotelEndpoints(data) {
  const group = 'Hotels';
  const token = authManager.getValidToken();
  
  if (!token) {
    console.error('No valid token for hotel tests');
    return;
  }

  // Test listing hotels
  const listResponse = http.get(
    `${config.baseUrl}/api/v1/hotels?limit=20&offset=0`,
    {
      headers: createHeaders(token),
      tags: { endpoint: 'hotels', operation: 'list', group }
    }
  );

  check(listResponse, {
    'hotel list successful': (r) => r.status === 200,
  });
  performanceMonitor.recordRequest(listResponse, 'hotels/list');

  // Test getting a specific hotel
  if (data.hotels && data.hotels.length > 0) {
    const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
    const getResponse = http.get(
      `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
      {
        headers: createHeaders(token),
        tags: { endpoint: 'hotels', operation: 'get', group }
      }
    );

    check(getResponse, {
      'hotel get successful': (r) => r.status === 200,
    });
    performanceMonitor.recordRequest(getResponse, 'hotels/get');
  }

  // Test hotel search with filters
  const searchResponse = http.get(
    `${config.baseUrl}/api/v1/hotels?city=Test&star_rating=4&limit=10`,
    {
      headers: createHeaders(token),
      tags: { endpoint: 'hotels', operation: 'search', group }
    }
  );

  check(searchResponse, {
    'hotel search successful': (r) => r.status === 200,
  });
  performanceMonitor.recordRequest(searchResponse, 'hotels/search');
}

// Provider endpoint tests
function testProviderEndpoints(data) {
  const group = 'Providers';
  const token = authManager.getValidToken();
  
  if (!token) {
    console.error('No valid token for provider tests');
    return;
  }

  // Test listing providers
  const listResponse = http.get(
    `${config.baseUrl}/api/v1/providers?limit=20&offset=0`,
    {
      headers: createHeaders(token),
      tags: { endpoint: 'providers', operation: 'list', group }
    }
  );

  check(listResponse, {
    'provider list successful': (r) => r.status === 200,
  });
  performanceMonitor.recordRequest(listResponse, 'providers/list');

  // Test getting a specific provider
  if (data.providers && data.providers.length > 0) {
    const provider = data.providers[Math.floor(Math.random() * data.providers.length)];
    const getResponse = http.get(
      `${config.baseUrl}/api/v1/providers/${provider.id}`,
      {
        headers: createHeaders(token),
        tags: { endpoint: 'providers', operation: 'get', group }
      }
    );

    check(getResponse, {
      'provider get successful': (r) => r.status === 200,
    });
    performanceMonitor.recordRequest(getResponse, 'providers/get');
  }
}

// Search endpoint tests
function testSearchEndpoints() {
  const group = 'Search';
  const token = authManager.getValidToken();
  
  if (!token) {
    console.error('No valid token for search tests');
    return;
  }

  // Test review search
  const searchQueries = ['great', 'excellent', 'poor', 'average', 'amazing'];
  const query = searchQueries[Math.floor(Math.random() * searchQueries.length)];
  
  const searchResponse = http.get(
    `${config.baseUrl}/api/v1/reviews/search?query=${query}&limit=20`,
    {
      headers: createHeaders(token),
      tags: { endpoint: 'search', operation: 'reviews', group }
    }
  );

  check(searchResponse, {
    'review search successful': (r) => r.status === 200,
  });
  performanceMonitor.recordRequest(searchResponse, 'search/reviews');

  // Test complex search with filters
  const complexSearchResponse = http.get(
    `${config.baseUrl}/api/v1/reviews/search?query=${query}&rating_min=4&rating_max=5&language=en&limit=10`,
    {
      headers: createHeaders(token),
      tags: { endpoint: 'search', operation: 'complex', group }
    }
  );

  check(complexSearchResponse, {
    'complex search successful': (r) => r.status === 200,
  });
  performanceMonitor.recordRequest(complexSearchResponse, 'search/complex');
}

// Mixed endpoint tests (realistic user behavior)
function testMixedEndpoints(data) {
  const endpoints = [
    () => testReviewEndpoints(data),
    () => testHotelEndpoints(data),
    () => testSearchEndpoints(),
  ];

  // Randomly select and execute an endpoint test
  const randomEndpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  randomEndpoint();
}

// Test health and monitoring endpoints
export function testHealthEndpoints() {
  const group = 'Health';
  
  // Test health check
  const healthResponse = http.get(
    `${config.baseUrl}/health`,
    {
      tags: { endpoint: 'health', operation: 'check', group }
    }
  );

  check(healthResponse, {
    'health check successful': (r) => r.status === 200,
    'health response valid': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status && body.timestamp;
      } catch (e) {
        return false;
      }
    },
  });

  // Test metrics endpoint
  const metricsResponse = http.get(
    `${config.baseUrl}/metrics`,
    {
      tags: { endpoint: 'metrics', operation: 'get', group }
    }
  );

  check(metricsResponse, {
    'metrics successful': (r) => r.status === 200,
  });

  // Test circuit breaker health
  const cbHealthResponse = http.get(
    `${config.baseUrl}/health/circuit-breakers`,
    {
      tags: { endpoint: 'health', operation: 'circuit_breakers', group }
    }
  );

  check(cbHealthResponse, {
    'circuit breaker health check successful': (r) => r.status === 200,
  });
}

// Cleanup function - runs once at the end
export function teardown(data) {
  if (dataManager) {
    dataManager.cleanup();
  }

  if (performanceMonitor) {
    const summary = performanceMonitor.getSummary();
    console.log('Performance Summary:', JSON.stringify(summary, null, 2));
  }
}

// Handle different test scenarios
export let options = {
  scenarios: {
    api_load_test: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '2m', target: 20 },
        { duration: '5m', target: 20 },
        { duration: '2m', target: 0 },
      ],
      gracefulRampDown: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'],
    http_req_failed: ['rate<0.05'],
    'http_req_duration{endpoint:reviews}': ['p(95)<500'],
    'http_req_duration{endpoint:hotels}': ['p(95)<400'],
    'http_req_duration{endpoint:auth}': ['p(95)<300'],
  },
};