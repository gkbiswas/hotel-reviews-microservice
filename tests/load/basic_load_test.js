import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics
export const errorRate = new Rate('errors');
export const requestDuration = new Trend('request_duration');
export const requestCount = new Counter('requests');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up
    { duration: '60s', target: 10 },  // Stay at 10 users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'],
    http_req_failed: ['rate<0.05'],
    errors: ['rate<0.1'],
    requests: ['count>100'],
  },
  ext: {
    loadimpact: {
      distribution: {
        'amazon:us:ashburn': { loadZone: 'amazon:us:ashburn', percent: 100 },
      },
    },
  },
};

// Base URL for the API
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Test data
const testData = {
  hotels: [
    { id: 1, name: 'Grand Hotel', rating: 4.5 },
    { id: 2, name: 'City Lodge', rating: 3.8 },
    { id: 3, name: 'Mountain Resort', rating: 4.2 },
  ],
  reviews: [
    { hotel_id: 1, rating: 5, comment: 'Excellent service and amenities' },
    { hotel_id: 2, rating: 4, comment: 'Good value for money' },
    { hotel_id: 3, rating: 3, comment: 'Average experience' },
  ],
};

// Helper functions
function makeRequest(url, params = {}) {
  const response = http.get(url, params);
  requestCount.add(1);
  requestDuration.add(response.timings.duration);
  return response;
}

function checkResponse(response, expectedStatus = 200, description = 'request') {
  const success = check(response, {
    [`${description} status is ${expectedStatus}`]: (r) => r.status === expectedStatus,
    [`${description} response time < 1000ms`]: (r) => r.timings.duration < 1000,
  });
  
  if (!success) {
    errorRate.add(1);
    console.log(`Failed ${description}: ${response.status} - ${response.body}`);
  }
  
  return success;
}

// Test scenarios
export default function() {
  // Health check
  healthCheck();
  
  // API endpoints
  testAPIEndpoints();
  
  // Load testing scenarios
  testHotelOperations();
  
  // Error handling
  testErrorHandling();
  
  // Performance testing
  testPerformance();
  
  sleep(1);
}

function healthCheck() {
  const response = makeRequest(`${BASE_URL}/api/v1/health`);
  checkResponse(response, 200, 'health check');
}

function testAPIEndpoints() {
  // Test metrics endpoint
  const metricsResponse = makeRequest(`${BASE_URL}/api/v1/metrics`);
  checkResponse(metricsResponse, 200, 'metrics endpoint');
  
  // Test readiness endpoint
  const readinessResponse = makeRequest(`${BASE_URL}/api/v1/ready`);
  checkResponse(readinessResponse, 200, 'readiness endpoint');
}

function testHotelOperations() {
  // Test hotel listing
  const hotelsResponse = makeRequest(`${BASE_URL}/api/v1/hotels`);
  checkResponse(hotelsResponse, 200, 'hotels listing');
  
  // Test individual hotel retrieval
  const hotelId = testData.hotels[Math.floor(Math.random() * testData.hotels.length)].id;
  const hotelResponse = makeRequest(`${BASE_URL}/api/v1/hotels/${hotelId}`);
  checkResponse(hotelResponse, 200, 'hotel retrieval');
  
  // Test hotel reviews
  const reviewsResponse = makeRequest(`${BASE_URL}/api/v1/hotels/${hotelId}/reviews`);
  checkResponse(reviewsResponse, 200, 'hotel reviews');
  
  // Test search functionality
  const searchResponse = makeRequest(`${BASE_URL}/api/v1/hotels/search?q=hotel`);
  checkResponse(searchResponse, 200, 'hotel search');
}

function testErrorHandling() {
  // Test 404 handling
  const notFoundResponse = makeRequest(`${BASE_URL}/api/v1/hotels/999999`);
  checkResponse(notFoundResponse, 404, 'not found handling');
  
  // Test invalid endpoints
  const invalidResponse = makeRequest(`${BASE_URL}/api/v1/invalid-endpoint`);
  checkResponse(invalidResponse, 404, 'invalid endpoint handling');
}

function testPerformance() {
  // Concurrent requests simulation
  const responses = [];
  for (let i = 0; i < 5; i++) {
    responses.push(makeRequest(`${BASE_URL}/api/v1/health`));
  }
  
  responses.forEach((response, index) => {
    checkResponse(response, 200, `concurrent request ${index + 1}`);
  });
}

// Setup function (runs once at the beginning)
export function setup() {
  console.log('Setting up load test...');
  
  // Verify that the service is up
  const response = http.get(`${BASE_URL}/api/v1/health`);
  if (response.status !== 200) {
    throw new Error(`Service is not healthy: ${response.status}`);
  }
  
  console.log('Load test setup complete');
  return { baseUrl: BASE_URL };
}

// Teardown function (runs once at the end)
export function teardown(data) {
  console.log('Tearing down load test...');
  
  // Final health check
  const response = http.get(`${data.baseUrl}/api/v1/health`);
  console.log(`Final health check status: ${response.status}`);
  
  console.log('Load test teardown complete');
}

// Custom check functions
export function checkPerformance() {
  return {
    'p95 response time < 500ms': (summary) => summary.metrics.http_req_duration.values.p95 < 500,
    'p99 response time < 1000ms': (summary) => summary.metrics.http_req_duration.values.p99 < 1000,
    'error rate < 1%': (summary) => summary.metrics.http_req_failed.values.rate < 0.01,
  };
}

// Export test results
export function handleSummary(data) {
  return {
    'load-test-summary.json': JSON.stringify(data, null, 2),
    'load-test-summary.html': generateHTMLReport(data),
  };
}

function generateHTMLReport(data) {
  const metrics = data.metrics;
  
  return `
    <!DOCTYPE html>
    <html>
    <head>
        <title>Load Test Report</title>
        <style>
            body { font-family: Arial, sans-serif; margin: 20px; }
            .metric { margin: 10px 0; }
            .pass { color: green; }
            .fail { color: red; }
            table { border-collapse: collapse; width: 100%; }
            th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
            th { background-color: #f2f2f2; }
        </style>
    </head>
    <body>
        <h1>Load Test Report</h1>
        <h2>Test Summary</h2>
        <div class="metric">Duration: ${data.state.testRunDurationMs}ms</div>
        <div class="metric">VUs: ${data.state.vusMax}</div>
        <div class="metric">Iterations: ${data.state.iterations}</div>
        
        <h2>Performance Metrics</h2>
        <table>
            <tr><th>Metric</th><th>Value</th><th>Status</th></tr>
            <tr>
                <td>HTTP Request Duration (p95)</td>
                <td>${metrics.http_req_duration.values.p95.toFixed(2)}ms</td>
                <td class="${metrics.http_req_duration.values.p95 < 1000 ? 'pass' : 'fail'}">
                    ${metrics.http_req_duration.values.p95 < 1000 ? 'PASS' : 'FAIL'}
                </td>
            </tr>
            <tr>
                <td>HTTP Request Duration (p99)</td>
                <td>${metrics.http_req_duration.values.p99.toFixed(2)}ms</td>
                <td class="${metrics.http_req_duration.values.p99 < 2000 ? 'pass' : 'fail'}">
                    ${metrics.http_req_duration.values.p99 < 2000 ? 'PASS' : 'FAIL'}
                </td>
            </tr>
            <tr>
                <td>HTTP Request Failed Rate</td>
                <td>${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%</td>
                <td class="${metrics.http_req_failed.values.rate < 0.05 ? 'pass' : 'fail'}">
                    ${metrics.http_req_failed.values.rate < 0.05 ? 'PASS' : 'FAIL'}
                </td>
            </tr>
            <tr>
                <td>Total Requests</td>
                <td>${metrics.http_reqs.values.count}</td>
                <td class="${metrics.http_reqs.values.count > 100 ? 'pass' : 'fail'}">
                    ${metrics.http_reqs.values.count > 100 ? 'PASS' : 'FAIL'}
                </td>
            </tr>
        </table>
        
        <h2>Detailed Metrics</h2>
        <pre>${JSON.stringify(metrics, null, 2)}</pre>
    </body>
    </html>
  `;
}