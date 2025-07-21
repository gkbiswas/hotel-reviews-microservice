# Load Testing Guide

## Overview

This guide provides comprehensive load testing strategies and scripts for the Hotel Reviews Microservice, including performance benchmarking, stress testing, and scalability validation.

## Load Testing Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Load Tester   │    │   API Gateway   │    │   Application   │    │   Backend       │
│   (k6/JMeter)   │    │   (Rate Limit)  │    │   Instances     │    │   Services      │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ Virtual Users   │───▶│ Load Balancer   │───▶│ Hotel Reviews   │───▶│ PostgreSQL      │
│ Test Scenarios  │    │ Circuit Breaker │    │ Microservice    │    │ Redis Cache     │
│ Metrics Collection    │ Health Checks   │    │ Auto Scaling    │    │ Kafka Queue     │
└─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Monitoring    │    │   Metrics       │
                       │   Dashboard     │    │   Collection    │
                       │                 │    │                 │
                       │ Grafana         │    │ Prometheus      │
                       │ Real-time       │    │ Custom Metrics  │
                       │ Alerting        │    │ SLA Tracking    │
                       └─────────────────┘    └─────────────────┘
```

## k6 Load Testing Scripts

### 1. Basic Performance Test

```javascript
// scripts/performance-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
export let errorRate = new Rate('errors');
export let responseTime = new Trend('response_time');
export let requestCount = new Counter('request_count');

export let options = {
  stages: [
    { duration: '2m', target: 100 },  // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 200 },  // Ramp up to 200 users
    { duration: '5m', target: 200 },  // Stay at 200 users
    { duration: '2m', target: 0 },    // Ramp down to 0 users
  ],
  thresholds: {
    'http_req_duration': ['p(95)<500'],  // 95% of requests must complete below 500ms
    'http_req_failed': ['rate<0.01'],    // Error rate must be below 1%
    'checks': ['rate>0.99'],             // 99% of checks must pass
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function() {
  let response;
  
  // Test hotel listing endpoint
  response = http.get(`${BASE_URL}/api/v1/hotels`, {
    headers: {
      'Content-Type': 'application/json',
    },
  });
  
  check(response, {
    'hotels list status is 200': (r) => r.status === 200,
    'hotels list response time < 100ms': (r) => r.timings.duration < 100,
    'hotels list has data': (r) => JSON.parse(r.body).length > 0,
  });
  
  errorRate.add(response.status !== 200);
  responseTime.add(response.timings.duration);
  requestCount.add(1);
  
  // Test specific hotel endpoint
  const hotelId = 'hotel-001';
  response = http.get(`${BASE_URL}/api/v1/hotels/${hotelId}`, {
    headers: {
      'Content-Type': 'application/json',
    },
  });
  
  check(response, {
    'hotel detail status is 200': (r) => r.status === 200,
    'hotel detail response time < 50ms': (r) => r.timings.duration < 50,
    'hotel detail has correct id': (r) => JSON.parse(r.body).id === hotelId,
  });
  
  // Test reviews endpoint
  response = http.get(`${BASE_URL}/api/v1/hotels/${hotelId}/reviews?limit=20`, {
    headers: {
      'Content-Type': 'application/json',
    },
  });
  
  check(response, {
    'reviews status is 200': (r) => r.status === 200,
    'reviews response time < 200ms': (r) => r.timings.duration < 200,
  });
  
  sleep(1); // Wait 1 second between iterations
}
```

### 2. Write-Heavy Load Test

```javascript
// scripts/write-load-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';

// Load test data
const testData = new SharedArray('review data', function() {
  return [
    {
      title: 'Great stay!',
      content: 'Had an amazing experience at this hotel. Staff was friendly and rooms were clean.',
      rating: 5.0,
      author_name: 'John Smith',
      author_location: 'New York, NY'
    },
    {
      title: 'Good location',
      content: 'Hotel is well located but room service could be improved.',
      rating: 4.0,
      author_name: 'Jane Doe',
      author_location: 'Los Angeles, CA'
    },
    {
      title: 'Average experience',
      content: 'Nothing special, but clean and decent for the price.',
      rating: 3.5,
      author_name: 'Bob Johnson',
      author_location: 'Chicago, IL'
    }
  ];
});

export let options = {
  stages: [
    { duration: '1m', target: 10 },   // Warm up
    { duration: '3m', target: 50 },   // Ramp up to moderate load
    { duration: '5m', target: 50 },   // Maintain load
    { duration: '1m', target: 0 },    // Cool down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<1000'], // 95% of write requests < 1s
    'http_req_failed': ['rate<0.05'],     // Error rate < 5% for writes
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || 'your-test-token';

export default function() {
  const reviewData = testData[Math.floor(Math.random() * testData.length)];
  
  const payload = JSON.stringify({
    hotel_id: 'hotel-001',
    provider_id: '550e8400-e29b-41d4-a716-446655440001',
    title: reviewData.title,
    content: reviewData.content,
    rating: reviewData.rating,
    author_name: reviewData.author_name,
    author_location: reviewData.author_location,
    review_date: new Date().toISOString(),
  });
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${AUTH_TOKEN}`,
    },
  };
  
  const response = http.post(`${BASE_URL}/api/v1/reviews`, payload, params);
  
  check(response, {
    'review creation status is 201': (r) => r.status === 201,
    'review creation response time < 500ms': (r) => r.timings.duration < 500,
    'review has id': (r) => JSON.parse(r.body).id !== undefined,
  });
  
  if (response.status !== 201) {
    console.log(`Failed to create review: ${response.status} - ${response.body}`);
  }
  
  sleep(Math.random() * 2 + 1); // Random sleep 1-3 seconds
}
```

### 3. Stress Test

```javascript
// scripts/stress-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '1m', target: 100 },   // Normal load
    { duration: '2m', target: 200 },   // Above normal
    { duration: '5m', target: 300 },   // High load
    { duration: '5m', target: 400 },   // Very high load
    { duration: '10m', target: 500 },  // Extreme load
    { duration: '3m', target: 0 },     // Recovery
  ],
  thresholds: {
    'http_req_duration': ['p(99)<2000'], // 99% under 2s even under stress
    'http_req_failed': ['rate<0.1'],      // Less than 10% errors
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function() {
  // Mixed workload to stress different parts of the system
  const endpoints = [
    '/api/v1/hotels',
    '/api/v1/hotels/hotel-001',
    '/api/v1/hotels/hotel-001/reviews',
    '/api/v1/analytics/top-hotels',
    '/health',
  ];
  
  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const response = http.get(`${BASE_URL}${endpoint}`);
  
  check(response, {
    'status is not 5xx': (r) => r.status < 500,
    'response time < 5s': (r) => r.timings.duration < 5000,
  });
  
  // No sleep to maximize stress
}
```

### 4. Bulk Processing Load Test

```javascript
// scripts/bulk-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '30s', target: 5 },    // Start with few concurrent bulk operations
    { duration: '2m', target: 10 },    // Increase bulk load
    { duration: '5m', target: 10 },    // Maintain bulk load
    { duration: '30s', target: 0 },    // Cool down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<5000'], // Bulk operations can take longer
    'http_req_failed': ['rate<0.02'],     // Very low error tolerance for bulk
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || 'your-test-token';

export default function() {
  // Simulate bulk review processing
  const bulkPayload = JSON.stringify({
    provider_id: '550e8400-e29b-41d4-a716-446655440001',
    file_url: 'https://example.com/sample-reviews.jsonl',
    callback_url: 'https://webhook.example.com/bulk-complete',
  });
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${AUTH_TOKEN}`,
    },
  };
  
  const response = http.post(`${BASE_URL}/api/v1/reviews/bulk`, bulkPayload, params);
  
  check(response, {
    'bulk job submitted': (r) => r.status === 202,
    'bulk response has job_id': (r) => JSON.parse(r.body).job_id !== undefined,
  });
  
  if (response.status === 202) {
    const jobId = JSON.parse(response.body).job_id;
    
    // Check job status after submission
    sleep(2);
    const statusResponse = http.get(`${BASE_URL}/api/v1/reviews/bulk/${jobId}/status`, {
      headers: { 'Authorization': `Bearer ${AUTH_TOKEN}` }
    });
    
    check(statusResponse, {
      'job status check successful': (r) => r.status === 200,
      'job has status field': (r) => JSON.parse(r.body).status !== undefined,
    });
  }
  
  sleep(10); // Longer sleep for bulk operations
}
```

### 5. Spike Test

```javascript
// scripts/spike-test.js
import http from 'k6/http';
import { check } from 'k6';

export let options = {
  stages: [
    { duration: '1m', target: 10 },    // Normal load
    { duration: '10s', target: 1000 }, // Sudden spike
    { duration: '1m', target: 1000 },  // Maintain spike
    { duration: '10s', target: 10 },   // Drop back to normal
    { duration: '2m', target: 10 },    // Recovery period
  ],
  thresholds: {
    'http_req_failed': ['rate<0.2'],    // Allow higher error rate during spike
    'http_req_duration': ['p(90)<3000'], // 90% under 3s
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function() {
  // Focus on cached endpoints during spike test
  const response = http.get(`${BASE_URL}/api/v1/hotels/hotel-001`);
  
  check(response, {
    'spike test status ok': (r) => r.status === 200 || r.status === 429, // 429 = rate limited
    'spike test has response': (r) => r.body.length > 0,
  });
}
```

## JMeter Test Plans

### 1. Basic HTTP Test Plan

```xml
<?xml version="1.0" encoding="UTF-8"?>
<jmeterTestPlan version="1.2" properties="5.0" jmeter="5.4.1">
  <hashTree>
    <TestPlan guiclass="TestPlanGui" testclass="TestPlan" testname="Hotel Reviews Load Test" enabled="true">
      <stringProp name="TestPlan.comments">Hotel Reviews Microservice Load Test</stringProp>
      <boolProp name="TestPlan.functional_mode">false</boolProp>
      <boolProp name="TestPlan.tearDown_on_shutdown">true</boolProp>
      <boolProp name="TestPlan.serialize_threadgroups">false</boolProp>
      <elementProp name="TestPlan.arguments" elementType="Arguments" guiclass="ArgumentsPanel" testclass="Arguments" testname="User Defined Variables" enabled="true">
        <collectionProp name="Arguments.arguments">
          <elementProp name="BASE_URL" elementType="Argument">
            <stringProp name="Argument.name">BASE_URL</stringProp>
            <stringProp name="Argument.value">http://localhost:8080</stringProp>
            <stringProp name="Argument.metadata">=</stringProp>
          </elementProp>
        </collectionProp>
      </elementProp>
      <stringProp name="TestPlan.user_define_classpath"></stringProp>
    </TestPlan>
    <hashTree>
      <ThreadGroup guiclass="ThreadGroupGui" testclass="ThreadGroup" testname="Hotel API Users" enabled="true">
        <stringProp name="ThreadGroup.on_sample_error">continue</stringProp>
        <elementProp name="ThreadGroup.main_controller" elementType="LoopController" guiclass="LoopControllerGui" testclass="LoopController" testname="Loop Controller" enabled="true">
          <boolProp name="LoopController.continue_forever">false</boolProp>
          <intProp name="LoopController.loops">10</intProp>
        </elementProp>
        <stringProp name="ThreadGroup.num_threads">50</stringProp>
        <stringProp name="ThreadGroup.ramp_time">60</stringProp>
        <boolProp name="ThreadGroup.scheduler">true</boolProp>
        <stringProp name="ThreadGroup.duration">300</stringProp>
        <stringProp name="ThreadGroup.delay">0</stringProp>
      </ThreadGroup>
      <hashTree>
        <HTTPSamplerProxy guiclass="HttpTestSampleGui" testclass="HTTPSamplerProxy" testname="Get Hotels List" enabled="true">
          <elementProp name="HTTPsampler.Arguments" elementType="Arguments" guiclass="HTTPArgumentsPanel" testclass="Arguments" testname="User Defined Variables" enabled="true">
            <collectionProp name="Arguments.arguments"/>
          </elementProp>
          <stringProp name="HTTPSampler.domain">localhost</stringProp>
          <stringProp name="HTTPSampler.port">8080</stringProp>
          <stringProp name="HTTPSampler.protocol">http</stringProp>
          <stringProp name="HTTPSampler.contentEncoding"></stringProp>
          <stringProp name="HTTPSampler.path">/api/v1/hotels</stringProp>
          <stringProp name="HTTPSampler.method">GET</stringProp>
          <boolProp name="HTTPSampler.follow_redirects">true</boolProp>
          <boolProp name="HTTPSampler.auto_redirects">false</boolProp>
          <boolProp name="HTTPSampler.use_keepalive">true</boolProp>
          <boolProp name="HTTPSampler.DO_MULTIPART_POST">false</boolProp>
          <stringProp name="HTTPSampler.embedded_url_re"></stringProp>
          <stringProp name="HTTPSampler.connect_timeout"></stringProp>
          <stringProp name="HTTPSampler.response_timeout"></stringProp>
        </HTTPSamplerProxy>
        <hashTree>
          <ResponseAssertion guiclass="AssertionGui" testclass="ResponseAssertion" testname="Response Assertion" enabled="true">
            <collectionProp name="Asserion.test_strings">
              <stringProp name="49586">200</stringProp>
            </collectionProp>
            <stringProp name="Assertion.custom_message"></stringProp>
            <stringProp name="Assertion.test_field">Assertion.response_code</stringProp>
            <boolProp name="Assertion.assume_success">false</boolProp>
            <intProp name="Assertion.test_type">1</intProp>
          </ResponseAssertion>
        </hashTree>
      </hashTree>
    </hashTree>
  </hashTree>
</jmeterTestPlan>
```

## Performance Monitoring During Tests

### 1. Real-time Monitoring Script

```bash
#!/bin/bash
# scripts/monitor-performance.sh

# Function to collect system metrics
collect_metrics() {
    echo "=== System Metrics at $(date) ==="
    
    # CPU Usage
    echo "CPU Usage:"
    top -bn1 | grep "Cpu(s)" | awk '{print $2 + $4"%"}'
    
    # Memory Usage
    echo "Memory Usage:"
    free -h | grep '^Mem' | awk '{print $3 "/" $2 " (" $3/$2*100.0 "%)"}'
    
    # Disk I/O
    echo "Disk I/O:"
    iostat -x 1 1 | grep -E "(Device|sda|nvme)"
    
    # Network
    echo "Network:"
    cat /proc/net/dev | grep eth0
    
    # Application metrics
    echo "Application Metrics:"
    curl -s http://localhost:8080/metrics | grep -E "(http_requests_total|http_request_duration|cache_hit_ratio)"
    
    echo "================================="
    echo ""
}

# Function to monitor application logs
monitor_logs() {
    echo "=== Recent Application Logs ==="
    tail -n 10 /var/log/hotel-reviews/app.log | grep -E "(ERROR|WARN|performance)"
    echo "================================="
    echo ""
}

# Main monitoring loop
echo "Starting performance monitoring..."
echo "Press Ctrl+C to stop"

while true; do
    collect_metrics
    monitor_logs
    sleep 30
done
```

### 2. Database Performance Monitoring

```sql
-- scripts/db-performance.sql
-- Monitor database performance during load tests

-- Long running queries
SELECT 
    pid,
    now() - pg_stat_activity.query_start AS duration,
    query 
FROM pg_stat_activity 
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes'
AND state = 'active'
ORDER BY duration DESC;

-- Database connections
SELECT 
    state,
    COUNT(*) as connection_count
FROM pg_stat_activity 
WHERE datname = 'hotel_reviews'
GROUP BY state;

-- Table statistics
SELECT 
    schemaname,
    tablename,
    seq_scan,
    seq_tup_read,
    idx_scan,
    idx_tup_fetch,
    n_tup_ins,
    n_tup_upd,
    n_tup_del
FROM pg_stat_user_tables
WHERE schemaname = 'public'
ORDER BY seq_tup_read DESC;

-- Cache hit ratio
SELECT 
    datname,
    numbackends,
    xact_commit,
    xact_rollback,
    blks_read,
    blks_hit,
    ROUND(blks_hit * 100.0 / (blks_hit + blks_read), 2) AS cache_hit_ratio
FROM pg_stat_database
WHERE datname = 'hotel_reviews';

-- Index usage
SELECT 
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;
```

## Load Test Automation

### 1. CI/CD Load Test Pipeline

```yaml
# .github/workflows/load-test.yml
name: Load Testing

on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
  workflow_dispatch:
    inputs:
      test_type:
        description: 'Type of load test'
        required: true
        default: 'performance'
        type: choice
        options:
          - performance
          - stress
          - spike
          - bulk

jobs:
  load-test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:14
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: hotel_reviews
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      redis:
        image: redis:7
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      
      - name: Build application
        run: |
          go build -o app cmd/api/main.go
      
      - name: Start application
        run: |
          ./app server --config config/test.yaml &
          sleep 10
        env:
          DATABASE_HOST: localhost
          DATABASE_PASSWORD: postgres
          REDIS_ADDR: localhost:6379
      
      - name: Install k6
        run: |
          wget -q -O - https://github.com/grafana/k6/releases/download/v0.46.0/k6-v0.46.0-linux-amd64.tar.gz | tar -xz
          sudo cp k6-v0.46.0-linux-amd64/k6 /usr/local/bin/
      
      - name: Run load test
        run: |
          TEST_TYPE=${{ github.event.inputs.test_type || 'performance' }}
          k6 run --out json=results.json scripts/${TEST_TYPE}-test.js
        env:
          BASE_URL: http://localhost:8080
      
      - name: Parse results
        run: |
          node scripts/parse-results.js results.json
      
      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: load-test-results
          path: |
            results.json
            summary.html
```

### 2. Results Analysis Script

```javascript
// scripts/parse-results.js
const fs = require('fs');

function parseK6Results(filePath) {
    const results = fs.readFileSync(filePath, 'utf8')
        .trim()
        .split('\n')
        .map(line => JSON.parse(line));
    
    const summary = {
        total_requests: 0,
        failed_requests: 0,
        avg_response_time: 0,
        p95_response_time: 0,
        throughput: 0,
        error_rate: 0,
    };
    
    const responseTimes = [];
    let totalDuration = 0;
    
    results.forEach(result => {
        if (result.type === 'Point') {
            const metric = result.metric;
            const value = result.data.value;
            
            switch (metric) {
                case 'http_reqs':
                    summary.total_requests += value;
                    break;
                case 'http_req_failed':
                    if (value === 1) summary.failed_requests++;
                    break;
                case 'http_req_duration':
                    responseTimes.push(value);
                    break;
            }
        }
    });
    
    // Calculate statistics
    if (responseTimes.length > 0) {
        responseTimes.sort((a, b) => a - b);
        summary.avg_response_time = responseTimes.reduce((a, b) => a + b) / responseTimes.length;
        summary.p95_response_time = responseTimes[Math.floor(responseTimes.length * 0.95)];
    }
    
    summary.error_rate = (summary.failed_requests / summary.total_requests) * 100;
    
    // Generate HTML report
    const htmlReport = `
<!DOCTYPE html>
<html>
<head>
    <title>Load Test Results</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .metric { margin: 10px 0; }
        .pass { color: green; }
        .fail { color: red; }
    </style>
</head>
<body>
    <h1>Load Test Results</h1>
    <div class="metric">Total Requests: ${summary.total_requests}</div>
    <div class="metric">Failed Requests: ${summary.failed_requests}</div>
    <div class="metric ${summary.error_rate < 1 ? 'pass' : 'fail'}">Error Rate: ${summary.error_rate.toFixed(2)}%</div>
    <div class="metric">Average Response Time: ${summary.avg_response_time.toFixed(2)}ms</div>
    <div class="metric ${summary.p95_response_time < 500 ? 'pass' : 'fail'}">95th Percentile: ${summary.p95_response_time.toFixed(2)}ms</div>
    <div class="metric">Throughput: ${(summary.total_requests / 300).toFixed(2)} RPS</div>
</body>
</html>
    `;
    
    fs.writeFileSync('summary.html', htmlReport);
    
    console.log('Load Test Summary:');
    console.log('=================');
    console.log(`Total Requests: ${summary.total_requests}`);
    console.log(`Failed Requests: ${summary.failed_requests}`);
    console.log(`Error Rate: ${summary.error_rate.toFixed(2)}%`);
    console.log(`Average Response Time: ${summary.avg_response_time.toFixed(2)}ms`);
    console.log(`95th Percentile: ${summary.p95_response_time.toFixed(2)}ms`);
    console.log(`Throughput: ${(summary.total_requests / 300).toFixed(2)} RPS`);
    
    // Exit with error code if thresholds are not met
    if (summary.error_rate > 1 || summary.p95_response_time > 500) {
        console.log('\n❌ Load test thresholds not met!');
        process.exit(1);
    } else {
        console.log('\n✅ All load test thresholds passed!');
        process.exit(0);
    }
}

const resultsFile = process.argv[2];
if (!resultsFile) {
    console.error('Usage: node parse-results.js <results.json>');
    process.exit(1);
}

parseK6Results(resultsFile);
```

## Best Practices

### 1. Load Testing Checklist

- **Environment Preparation**
  - [ ] Test environment mirrors production
  - [ ] Database populated with realistic data
  - [ ] All external dependencies mocked or available
  - [ ] Monitoring systems active

- **Test Design**
  - [ ] Realistic user behavior patterns
  - [ ] Appropriate think times between requests
  - [ ] Mixed read/write workloads
  - [ ] Edge case scenarios included

- **Execution**
  - [ ] Gradual ramp-up to avoid overwhelming
  - [ ] Monitor system resources during tests
  - [ ] Record application logs and metrics
  - [ ] Document any failures or anomalies

- **Analysis**
  - [ ] Compare results against SLA requirements
  - [ ] Identify performance bottlenecks
  - [ ] Analyze error patterns and causes
  - [ ] Plan optimization improvements

This comprehensive load testing guide ensures thorough performance validation of the Hotel Reviews Microservice under various load conditions.