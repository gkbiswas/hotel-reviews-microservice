import http from 'k6/http';
import { check, sleep } from 'k6';
import { config, database, createHeaders } from './config.js';
import { AuthManager, DatabaseTester, PerformanceMonitor, CacheTester } from './utils.js';

// Global instances
let authManager;
let databaseTester;
let performanceMonitor;
let cacheTester;

// Database performance test configuration
const DB_TEST_CONFIG = {
  maxConcurrentQueries: parseInt(__ENV.MAX_CONCURRENT_QUERIES) || 20,
  queryTimeout: parseInt(__ENV.QUERY_TIMEOUT) || 30000, // 30 seconds
  cacheTestDuration: parseInt(__ENV.CACHE_TEST_DURATION) || 300, // 5 minutes
  bulkInsertBatchSize: parseInt(__ENV.BULK_INSERT_BATCH_SIZE) || 100,
  searchComplexity: __ENV.SEARCH_COMPLEXITY || 'medium', // low, medium, high
};

// Setup function
export function setup() {
  authManager = new AuthManager();
  databaseTester = new DatabaseTester(authManager);
  performanceMonitor = new PerformanceMonitor();
  cacheTester = new CacheTester(authManager);

  // Ensure authentication
  const token = authManager.getValidToken();
  if (!token) {
    throw new Error('Failed to authenticate for database performance tests');
  }

  // Pre-populate test data for performance testing
  const testData = {
    hotels: [],
    reviews: [],
    providers: [],
  };

  // Create test hotels for database testing
  for (let i = 0; i < 10; i++) {
    const hotelData = {
      name: `Performance Test Hotel ${i + 1}`,
      address: `${100 + i} Test Street`,
      city: `Test City ${i + 1}`,
      country: 'Test Country',
      star_rating: Math.floor(Math.random() * 5) + 1,
      description: `This is a test hotel ${i + 1} for database performance testing`,
    };
    
    // Call hotel creation endpoint
    const response = http.post(
      `${config.baseUrl}/api/v1/hotels`,
      JSON.stringify(hotelData),
      {
        headers: createHeaders(authManager.getValidToken(true)), // Admin token
        tags: { endpoint: 'setup', operation: 'create_hotel' }
      }
    );

    if (response.status === 201) {
      const body = JSON.parse(response.body);
      if (body.success && body.data) {
        testData.hotels.push(body.data);
      }
    }
  }

  return testData;
}

// Main database performance test function
export default function(data) {
  if (!authManager) {
    authManager = new AuthManager();
    databaseTester = new DatabaseTester(authManager);
    performanceMonitor = new PerformanceMonitor();
    cacheTester = new CacheTester(authManager);
  }

  const testType = __ENV.DB_TEST_TYPE || 'mixed';
  
  switch (testType) {
    case 'concurrent_reads':
      testConcurrentReads(data);
      break;
    case 'complex_queries':
      testComplexQueries(data);
      break;
    case 'bulk_operations':
      testBulkOperations(data);
      break;
    case 'cache_performance':
      testCachePerformance(data);
      break;
    case 'connection_stress':
      testConnectionPoolStress();
      break;
    case 'indexing_performance':
      testIndexingPerformance(data);
      break;
    case 'transaction_stress':
      testTransactionStress(data);
      break;
    default:
      testMixedDatabaseOperations(data);
  }

  // Add think time between database operations
  sleep(Math.random() * 2 + 0.5);
}

// Test concurrent read operations
function testConcurrentReads(data) {
  const group = 'Concurrent Database Reads';
  
  if (!data.hotels || data.hotels.length === 0) {
    console.warn('No test hotels available for concurrent read tests');
    return;
  }

  // Prepare concurrent read requests
  const readRequests = [];
  const concurrentCount = Math.min(DB_TEST_CONFIG.maxConcurrentQueries, 15);
  
  for (let i = 0; i < concurrentCount; i++) {
    const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
    const endpoints = [
      `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
      `${config.baseUrl}/api/v1/reviews?hotel_id=${hotel.id}&limit=20`,
      `${config.baseUrl}/api/v1/hotels?limit=50&offset=${i * 10}`,
      `${config.baseUrl}/api/v1/reviews?limit=50&offset=${i * 20}`,
    ];
    
    const randomEndpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
    
    readRequests.push([
      'GET',
      randomEndpoint,
      null,
      {
        headers: createHeaders(authManager.getValidToken()),
        tags: { endpoint: 'database', operation: 'concurrent_read', group }
      }
    ]);
  }

  // Execute concurrent reads
  const startTime = Date.now();
  const responses = http.batch(readRequests);
  const endTime = Date.now();
  
  const totalTime = endTime - startTime;
  let successfulReads = 0;
  let totalResponseTime = 0;

  responses.forEach((response, index) => {
    const success = check(response, {
      [`concurrent read ${index + 1} successful`]: (r) => r.status === 200,
      [`concurrent read ${index + 1} within timeout`]: (r) => r.timings.duration < DB_TEST_CONFIG.queryTimeout,
    });

    if (success) {
      successfulReads++;
      totalResponseTime += response.timings.duration;
    }

    performanceMonitor.recordRequest(response, 'database/concurrent_read');
    performanceMonitor.recordDbOperation(response.timings.duration);
  });

  const avgResponseTime = successfulReads > 0 ? totalResponseTime / successfulReads : 0;
  
  check(null, {
    'concurrent reads success rate acceptable': () => successfulReads / concurrentCount >= 0.95,
    'concurrent reads average response time acceptable': () => avgResponseTime < 1000,
    'concurrent reads total time reasonable': () => totalTime < 10000,
  });

  console.log(`Concurrent reads: ${successfulReads}/${concurrentCount} successful, avg: ${avgResponseTime.toFixed(2)}ms, total: ${totalTime}ms`);
}

// Test complex database queries
function testComplexQueries(data) {
  const group = 'Complex Database Queries';
  
  // Test complex search queries
  const searchQueries = [
    'excellent service',
    'great location',
    'poor experience',
    'amazing food',
    'comfortable rooms'
  ];

  searchQueries.forEach((query, index) => {
    const searchResponse = http.get(
      `${config.baseUrl}/api/v1/reviews/search?query=${encodeURIComponent(query)}&limit=50`,
      {
        headers: createHeaders(authManager.getValidToken()),
        tags: { endpoint: 'database', operation: 'complex_search', group }
      }
    );

    check(searchResponse, {
      [`complex search ${index + 1} successful`]: (r) => r.status === 200,
      [`complex search ${index + 1} response time acceptable`]: (r) => r.timings.duration < 5000,
    });

    performanceMonitor.recordRequest(searchResponse, 'database/complex_search');
    performanceMonitor.recordDbOperation(searchResponse.timings.duration);
  });

  // Test aggregation queries
  if (data.hotels && data.hotels.length > 0) {
    data.hotels.slice(0, 3).forEach((hotel, index) => {
      const aggregationResponse = http.get(
        `${config.baseUrl}/api/v1/reviews/statistics?hotel_id=${hotel.id}`,
        {
          headers: createHeaders(authManager.getValidToken()),
          tags: { endpoint: 'database', operation: 'aggregation', group }
        }
      );

      check(aggregationResponse, {
        [`aggregation query ${index + 1} successful`]: (r) => r.status === 200,
        [`aggregation query ${index + 1} response time acceptable`]: (r) => r.timings.duration < 3000,
      });

      performanceMonitor.recordRequest(aggregationResponse, 'database/aggregation');
      performanceMonitor.recordDbOperation(aggregationResponse.timings.duration);
    });
  }

  // Test complex filtering
  const complexFilterResponse = http.get(
    `${config.baseUrl}/api/v1/reviews?rating_min=4&rating_max=5&language=en&limit=100&sort=rating&order=desc`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'database', operation: 'complex_filter', group }
    }
  );

  check(complexFilterResponse, {
    'complex filter query successful': (r) => r.status === 200,
    'complex filter response time acceptable': (r) => r.timings.duration < 2000,
  });

  performanceMonitor.recordRequest(complexFilterResponse, 'database/complex_filter');
  performanceMonitor.recordDbOperation(complexFilterResponse.timings.duration);

  // Test pagination stress
  const paginationSizes = [10, 50, 100, 200];
  paginationSizes.forEach((size, index) => {
    const paginationResponse = http.get(
      `${config.baseUrl}/api/v1/reviews?limit=${size}&offset=${index * size}`,
      {
        headers: createHeaders(authManager.getValidToken()),
        tags: { endpoint: 'database', operation: 'pagination', group }
      }
    );

    check(paginationResponse, {
      [`pagination ${size} records successful`]: (r) => r.status === 200,
      [`pagination ${size} records response time acceptable`]: (r) => r.timings.duration < 3000,
    });

    performanceMonitor.recordRequest(paginationResponse, 'database/pagination');
    performanceMonitor.recordDbOperation(paginationResponse.timings.duration);
  });
}

// Test bulk database operations
function testBulkOperations(data) {
  const group = 'Bulk Database Operations';
  
  if (!data.hotels || data.hotels.length === 0) {
    console.warn('No test hotels available for bulk operations');
    return;
  }

  // Prepare bulk review data
  const bulkReviews = [];
  const batchSize = DB_TEST_CONFIG.bulkInsertBatchSize;
  
  for (let i = 0; i < batchSize; i++) {
    const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
    bulkReviews.push({
      hotel_id: hotel.id,
      reviewer_info_id: hotel.id, // Simplified for testing
      rating: Math.floor(Math.random() * 5) + 1,
      title: `Bulk Test Review ${i + 1}`,
      comment: `This is a bulk test review number ${i + 1} for database performance testing.`,
      review_date: new Date(Date.now() - Math.random() * 365 * 24 * 60 * 60 * 1000).toISOString(),
      language: 'en',
    });
  }

  // Test bulk insert
  const bulkInsertResponse = http.post(
    `${config.baseUrl}/api/v1/reviews/bulk`,
    JSON.stringify(bulkReviews),
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'database', operation: 'bulk_insert', group }
    }
  );

  check(bulkInsertResponse, {
    'bulk insert successful': (r) => r.status === 201 || r.status === 200,
    'bulk insert response time acceptable': (r) => r.timings.duration < 30000, // 30 seconds for bulk
    'bulk insert response has data': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.success && body.data;
      } catch (e) {
        return false;
      }
    },
  });

  performanceMonitor.recordRequest(bulkInsertResponse, 'database/bulk_insert');
  performanceMonitor.recordDbOperation(bulkInsertResponse.timings.duration);

  // Test bulk read after insert
  sleep(1); // Allow time for data to be committed
  
  const bulkReadResponse = http.get(
    `${config.baseUrl}/api/v1/reviews?limit=${batchSize}&sort=created_at&order=desc`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'database', operation: 'bulk_read', group }
    }
  );

  check(bulkReadResponse, {
    'bulk read after insert successful': (r) => r.status === 200,
    'bulk read response time acceptable': (r) => r.timings.duration < 5000,
  });

  performanceMonitor.recordRequest(bulkReadResponse, 'database/bulk_read');
  performanceMonitor.recordDbOperation(bulkReadResponse.timings.duration);
}

// Test cache performance
function testCachePerformance(data) {
  const group = 'Cache Performance';
  
  if (!data.hotels || data.hotels.length === 0) {
    console.warn('No test hotels available for cache performance tests');
    return;
  }

  // Test cache hit/miss patterns
  const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
  
  // First request (cache miss)
  const firstResponse = http.get(
    `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'cache', operation: 'miss', group }
    }
  );

  const firstResponseTime = firstResponse.timings.duration;

  check(firstResponse, {
    'cache miss request successful': (r) => r.status === 200,
  });

  sleep(0.1); // Small delay

  // Second request (should be cache hit)
  const secondResponse = http.get(
    `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'cache', operation: 'hit', group }
    }
  );

  const secondResponseTime = secondResponse.timings.duration;

  check(secondResponse, {
    'cache hit request successful': (r) => r.status === 200,
    'cache hit faster than miss': () => secondResponseTime < firstResponseTime,
    'cache hit response time acceptable': () => secondResponseTime < 100, // Should be very fast
  });

  const cacheEffectiveness = ((firstResponseTime - secondResponseTime) / firstResponseTime) * 100;
  console.log(`Cache effectiveness: ${cacheEffectiveness.toFixed(2)}% improvement (${firstResponseTime}ms -> ${secondResponseTime}ms)`);

  performanceMonitor.recordRequest(firstResponse, 'cache/miss');
  performanceMonitor.recordRequest(secondResponse, 'cache/hit');

  // Test cache invalidation
  const updateData = {
    description: `Updated description for cache test - ${Date.now()}`,
  };

  const updateResponse = http.put(
    `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
    JSON.stringify(updateData),
    {
      headers: createHeaders(authManager.getValidToken(true)), // Admin token
      tags: { endpoint: 'cache', operation: 'invalidate', group }
    }
  );

  check(updateResponse, {
    'cache invalidation update successful': (r) => r.status === 200,
  });

  sleep(0.1);

  // Request after update (should be fresh data, not cached)
  const postUpdateResponse = http.get(
    `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'cache', operation: 'post_invalidation', group }
    }
  );

  check(postUpdateResponse, {
    'post-invalidation request successful': (r) => r.status === 200,
    'cache invalidation working': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.data && body.data.description.includes(Date.now().toString().slice(0, -3)); // Check if updated
      } catch (e) {
        return false;
      }
    },
  });

  performanceMonitor.recordRequest(updateResponse, 'cache/invalidate');
  performanceMonitor.recordRequest(postUpdateResponse, 'cache/post_invalidation');
}

// Test database connection pool stress
function testConnectionPoolStress() {
  const group = 'Connection Pool Stress';
  
  const concurrentConnections = DB_TEST_CONFIG.maxConcurrentQueries;
  const requests = [];
  
  // Create many concurrent requests to stress connection pool
  for (let i = 0; i < concurrentConnections; i++) {
    requests.push([
      'GET',
      `${config.baseUrl}/api/v1/reviews?limit=10&offset=${i * 10}`,
      null,
      {
        headers: createHeaders(authManager.getValidToken()),
        tags: { endpoint: 'database', operation: 'connection_stress', group }
      }
    ]);
  }

  const startTime = Date.now();
  const responses = http.batch(requests);
  const endTime = Date.now();
  
  const totalTime = endTime - startTime;
  let successfulConnections = 0;
  let connectionErrors = 0;

  responses.forEach((response, index) => {
    const success = check(response, {
      [`connection stress ${index + 1} successful`]: (r) => r.status === 200,
      [`connection stress ${index + 1} no timeout`]: (r) => r.timings.duration < DB_TEST_CONFIG.queryTimeout,
    });

    if (success) {
      successfulConnections++;
    } else if (response.status === 503 || response.status === 500) {
      connectionErrors++;
    }

    performanceMonitor.recordRequest(response, 'database/connection_stress');
    performanceMonitor.recordDbOperation(response.timings.duration);
  });

  const successRate = successfulConnections / concurrentConnections;
  
  check(null, {
    'connection pool handles stress well': () => successRate >= 0.9, // 90% success rate
    'connection pool no critical errors': () => connectionErrors < concurrentConnections * 0.1, // Less than 10% errors
    'connection pool stress response time reasonable': () => totalTime < 15000, // 15 seconds total
  });

  console.log(`Connection pool stress: ${successfulConnections}/${concurrentConnections} successful (${(successRate * 100).toFixed(2)}%), ${connectionErrors} errors, total time: ${totalTime}ms`);
}

// Test indexing performance
function testIndexingPerformance(data) {
  const group = 'Indexing Performance';
  
  // Test queries that should benefit from indexing
  const indexedQueries = [
    // Primary key lookups
    () => {
      if (data.hotels && data.hotels.length > 0) {
        const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
        return http.get(
          `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
          {
            headers: createHeaders(authManager.getValidToken()),
            tags: { endpoint: 'database', operation: 'index_pk', group }
          }
        );
      }
      return null;
    },
    
    // Foreign key lookups
    () => {
      if (data.hotels && data.hotels.length > 0) {
        const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
        return http.get(
          `${config.baseUrl}/api/v1/reviews?hotel_id=${hotel.id}`,
          {
            headers: createHeaders(authManager.getValidToken()),
            tags: { endpoint: 'database', operation: 'index_fk', group }
          }
        );
      }
      return null;
    },
    
    // Timestamp-based queries
    () => {
      const fromDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]; // 30 days ago
      return http.get(
        `${config.baseUrl}/api/v1/reviews?from_date=${fromDate}&limit=50`,
        {
          headers: createHeaders(authManager.getValidToken()),
          tags: { endpoint: 'database', operation: 'index_timestamp', group }
        }
      );
    },
    
    // Rating-based queries (likely indexed)
    () => {
      return http.get(
        `${config.baseUrl}/api/v1/reviews?rating_min=4&limit=50`,
        {
          headers: createHeaders(authManager.getValidToken()),
          tags: { endpoint: 'database', operation: 'index_rating', group }
        }
      );
    }
  ];

  indexedQueries.forEach((queryFn, index) => {
    const response = queryFn();
    if (response) {
      check(response, {
        [`indexed query ${index + 1} successful`]: (r) => r.status === 200,
        [`indexed query ${index + 1} fast response`]: (r) => r.timings.duration < 500, // Should be fast due to indexing
      });

      performanceMonitor.recordRequest(response, `database/index_query_${index + 1}`);
      performanceMonitor.recordDbOperation(response.timings.duration);
    }
  });
}

// Test transaction stress
function testTransactionStress(data) {
  const group = 'Transaction Stress';
  
  if (!data.hotels || data.hotels.length === 0) {
    console.warn('No test hotels available for transaction stress tests');
    return;
  }

  // Test rapid create/update/delete operations
  const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
  const transactionOperations = [];

  // Create multiple reviews in quick succession
  for (let i = 0; i < 5; i++) {
    const reviewData = {
      hotel_id: hotel.id,
      reviewer_info_id: hotel.id,
      rating: Math.floor(Math.random() * 5) + 1,
      title: `Transaction Test Review ${i + 1}`,
      comment: `Transaction stress test review ${i + 1}`,
      review_date: new Date().toISOString(),
      language: 'en',
    };

    transactionOperations.push([
      'POST',
      `${config.baseUrl}/api/v1/reviews`,
      JSON.stringify(reviewData),
      {
        headers: createHeaders(authManager.getValidToken()),
        tags: { endpoint: 'database', operation: 'transaction_create', group }
      }
    ]);
  }

  // Execute transaction operations
  const responses = http.batch(transactionOperations);
  
  let successfulTransactions = 0;
  const createdReviewIds = [];

  responses.forEach((response, index) => {
    const success = check(response, {
      [`transaction create ${index + 1} successful`]: (r) => r.status === 201,
      [`transaction create ${index + 1} within time limit`]: (r) => r.timings.duration < 5000,
    });

    if (success) {
      successfulTransactions++;
      try {
        const body = JSON.parse(response.body);
        if (body.success && body.data && body.data.id) {
          createdReviewIds.push(body.data.id);
        }
      } catch (e) {
        // Ignore parsing errors
      }
    }

    performanceMonitor.recordRequest(response, 'database/transaction_create');
    performanceMonitor.recordDbOperation(response.timings.duration);
  });

  check(null, {
    'transaction creates successful': () => successfulTransactions >= 4, // Allow for 1 failure
  });

  // Cleanup created reviews
  createdReviewIds.forEach(reviewId => {
    const deleteResponse = http.del(
      `${config.baseUrl}/api/v1/reviews/${reviewId}`,
      null,
      {
        headers: createHeaders(authManager.getValidToken()),
        tags: { endpoint: 'database', operation: 'transaction_cleanup', group }
      }
    );

    performanceMonitor.recordRequest(deleteResponse, 'database/transaction_cleanup');
  });

  console.log(`Transaction stress: ${successfulTransactions}/5 creates successful, ${createdReviewIds.length} reviews created and cleaned up`);
}

// Test mixed database operations (realistic scenario)
function testMixedDatabaseOperations(data) {
  const operations = [
    () => testSingleRead(data),
    () => testSingleWrite(data),
    () => testSearchOperation(),
    () => testAggregationOperation(data),
  ];

  // Randomly select and execute an operation
  const randomOperation = operations[Math.floor(Math.random() * operations.length)];
  randomOperation();
}

// Helper functions for mixed operations
function testSingleRead(data) {
  if (!data.hotels || data.hotels.length === 0) return;
  
  const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
  const response = http.get(
    `${config.baseUrl}/api/v1/hotels/${hotel.id}`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'database', operation: 'single_read' }
    }
  );

  check(response, {
    'single read successful': (r) => r.status === 200,
    'single read fast': (r) => r.timings.duration < 500,
  });

  performanceMonitor.recordRequest(response, 'database/single_read');
  performanceMonitor.recordDbOperation(response.timings.duration);
}

function testSingleWrite(data) {
  if (!data.hotels || data.hotels.length === 0) return;
  
  const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
  const reviewData = {
    hotel_id: hotel.id,
    reviewer_info_id: hotel.id,
    rating: Math.floor(Math.random() * 5) + 1,
    title: 'Mixed Operation Test Review',
    comment: 'This is a review created during mixed database operations testing.',
    review_date: new Date().toISOString(),
    language: 'en',
  };

  const response = http.post(
    `${config.baseUrl}/api/v1/reviews`,
    JSON.stringify(reviewData),
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'database', operation: 'single_write' }
    }
  );

  check(response, {
    'single write successful': (r) => r.status === 201,
    'single write reasonable time': (r) => r.timings.duration < 2000,
  });

  performanceMonitor.recordRequest(response, 'database/single_write');
  performanceMonitor.recordDbOperation(response.timings.duration);
}

function testSearchOperation() {
  const searchTerms = ['excellent', 'good', 'average', 'poor'];
  const term = searchTerms[Math.floor(Math.random() * searchTerms.length)];
  
  const response = http.get(
    `${config.baseUrl}/api/v1/reviews/search?query=${term}&limit=20`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'database', operation: 'search' }
    }
  );

  check(response, {
    'search operation successful': (r) => r.status === 200,
    'search operation reasonable time': (r) => r.timings.duration < 3000,
  });

  performanceMonitor.recordRequest(response, 'database/search');
  performanceMonitor.recordDbOperation(response.timings.duration);
}

function testAggregationOperation(data) {
  if (!data.hotels || data.hotels.length === 0) return;
  
  const hotel = data.hotels[Math.floor(Math.random() * data.hotels.length)];
  const response = http.get(
    `${config.baseUrl}/api/v1/reviews/statistics?hotel_id=${hotel.id}`,
    {
      headers: createHeaders(authManager.getValidToken()),
      tags: { endpoint: 'database', operation: 'aggregation' }
    }
  );

  check(response, {
    'aggregation operation successful': (r) => r.status === 200,
    'aggregation operation reasonable time': (r) => r.timings.duration < 2000,
  });

  performanceMonitor.recordRequest(response, 'database/aggregation');
  performanceMonitor.recordDbOperation(response.timings.duration);
}

// Teardown function
export function teardown(data) {
  if (performanceMonitor) {
    const summary = performanceMonitor.getSummary();
    console.log('Database Performance Summary:', JSON.stringify(summary, null, 2));
  }

  // Cleanup test data (if needed)
  const adminToken = authManager && authManager.getValidToken(true);
  if (adminToken && data.hotels) {
    data.hotels.forEach(hotel => {
      http.del(`${config.baseUrl}/api/v1/hotels/${hotel.id}`, null, {
        headers: createHeaders(adminToken),
        tags: { endpoint: 'cleanup', operation: 'delete_hotel' }
      });
    });
  }
}

// Export options for k6
export let options = {
  scenarios: {
    database_performance: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '2m', target: 10 },  // Ramp up to 10 users
        { duration: '5m', target: 10 },  // Maintain 10 users
        { duration: '3m', target: 20 },  // Increase to 20 users
        { duration: '5m', target: 20 },  // Maintain 20 users
        { duration: '2m', target: 0 },   // Ramp down
      ],
      gracefulRampDown: '1m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'],
    http_req_failed: ['rate<0.05'],
    'http_req_duration{operation:single_read}': ['p(95)<500'],
    'http_req_duration{operation:single_write}': ['p(95)<2000'],
    'http_req_duration{operation:complex_search}': ['p(95)<5000'],
    'http_req_duration{operation:bulk_insert}': ['p(95)<30000'],
    'http_req_duration{operation:cache_hit}': ['p(95)<100'],
    database_operations: ['p(95)<2000'],
  },
};