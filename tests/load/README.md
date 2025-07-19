# Hotel Reviews Load Testing Suite

A comprehensive load testing suite built with k6 to test the Hotel Reviews microservice under various load conditions.

## Overview

This load testing suite provides comprehensive performance testing for:
- API endpoints under load
- Concurrent file processing
- Database performance
- Normal, peak, and stress load scenarios
- Circuit breaker and resilience testing
- Authentication and authorization under load
- Cache performance testing

## Prerequisites

1. **k6 Installation**
   ```bash
   # macOS
   brew install k6
   
   # Ubuntu/Debian
   sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
   echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
   sudo apt-get update
   sudo apt-get install k6
   
   # Windows (chocolatey)
   choco install k6
   ```

2. **Environment Setup**
   - Ensure the Hotel Reviews API is running
   - Configure database connections
   - Set up monitoring tools (optional)

## Test Structure

```
tests/load/
├── config.js              # Configuration and test data
├── utils.js               # Utility classes and functions
├── api-endpoints.js       # API endpoint tests
├── file-processing.js     # File processing tests
├── database-performance.js # Database performance tests
├── scenarios.js           # Load testing scenarios
├── metrics-reporter.js    # Metrics collection and reporting
├── run-*.sh              # Execution scripts
└── README.md             # This file
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BASE_URL` | API base URL | `http://localhost:8080` |
| `TEST_USERNAME` | Test user username | `testuser` |
| `TEST_PASSWORD` | Test user password | `testpass123` |
| `ADMIN_USERNAME` | Admin user username | `admin` |
| `ADMIN_PASSWORD` | Admin user password | `adminpass123` |
| `DB_HOST` | Database host | `localhost` |
| `DB_PORT` | Database port | `5432` |
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |

### Test Configuration

Edit `config.js` to customize:
- Load testing scenarios
- Performance thresholds
- Test data
- Monitoring settings

## Quick Start

### 1. Basic Load Test
```bash
# Run normal load scenario
k6 run tests/load/api-endpoints.js

# Run with custom base URL
BASE_URL=http://localhost:8080 k6 run tests/load/api-endpoints.js
```

### 2. Specific Test Scenarios
```bash
# Stress test
SCENARIO=stress_test k6 run tests/load/scenarios.js

# Peak load test
SCENARIO=peak_load k6 run tests/load/scenarios.js

# Database performance test
k6 run tests/load/database-performance.js

# File processing test
k6 run tests/load/file-processing.js
```

### 3. Using Run Scripts
```bash
# Make scripts executable
chmod +x tests/load/run-*.sh

# Run normal load test
./tests/load/run-normal-load.sh

# Run stress test
./tests/load/run-stress-test.sh

# Run full test suite
./tests/load/run-full-suite.sh
```

## Test Scenarios

### 1. Normal Load (`normal_load`)
- **Purpose**: Baseline performance testing
- **Load**: 10 concurrent users
- **Duration**: 14 minutes
- **Focus**: API endpoints, basic CRUD operations

### 2. Peak Load (`peak_load`)
- **Purpose**: High traffic simulation
- **Load**: 30-80 concurrent users
- **Duration**: 38 minutes
- **Focus**: Sustained high load with traffic spikes

### 3. Stress Test (`stress_test`)
- **Purpose**: Beyond normal capacity testing
- **Load**: 100-200 concurrent users
- **Duration**: 40 minutes
- **Focus**: System breaking points and recovery

### 4. Spike Test (`spike_test`)
- **Purpose**: Sudden traffic increases
- **Load**: 20-100 users (sudden spike)
- **Duration**: 9 minutes
- **Focus**: System response to traffic spikes

### 5. File Processing (`file_processing_load`)
- **Purpose**: Concurrent file upload testing
- **Load**: 15 concurrent uploads
- **Duration**: 45 minutes
- **Focus**: File processing capabilities

### 6. Database Intensive (`database_intensive`)
- **Purpose**: Database performance under load
- **Load**: 25-60 concurrent users
- **Duration**: 36 minutes
- **Focus**: Database queries, transactions, caching

## Test Types

### API Endpoint Testing
```bash
# Test specific endpoint types
TEST_TYPE=auth k6 run tests/load/api-endpoints.js      # Authentication
TEST_TYPE=reviews k6 run tests/load/api-endpoints.js   # Review operations
TEST_TYPE=hotels k6 run tests/load/api-endpoints.js    # Hotel operations
TEST_TYPE=search k6 run tests/load/api-endpoints.js    # Search operations
```

### File Processing Testing
```bash
# Test specific file processing scenarios
FILE_TEST_TYPE=concurrent k6 run tests/load/file-processing.js     # Concurrent uploads
FILE_TEST_TYPE=large_files k6 run tests/load/file-processing.js    # Large file testing
FILE_TEST_TYPE=stress k6 run tests/load/file-processing.js         # File processing stress
```

### Database Testing
```bash
# Test specific database scenarios
DB_TEST_TYPE=concurrent_reads k6 run tests/load/database-performance.js
DB_TEST_TYPE=complex_queries k6 run tests/load/database-performance.js
DB_TEST_TYPE=bulk_operations k6 run tests/load/database-performance.js
DB_TEST_TYPE=cache_performance k6 run tests/load/database-performance.js
```

## Metrics and Monitoring

### Built-in Metrics
- HTTP request duration (p95, p99)
- HTTP request failure rate
- Request rate (RPS)
- Data sent/received

### Custom Metrics
- API response times by endpoint
- Database query performance
- Cache hit/miss rates
- File processing metrics
- Circuit breaker activations
- Authentication success rates

### Exporting Metrics
```bash
# Export to JSON
EXPORT_METRICS=true METRICS_FORMAT=json k6 run tests/load/api-endpoints.js

# Export to Prometheus format
EXPORT_METRICS=true METRICS_FORMAT=prometheus k6 run tests/load/api-endpoints.js

# Export to InfluxDB format
EXPORT_METRICS=true METRICS_FORMAT=influxdb k6 run tests/load/api-endpoints.js
```

### Real-time Monitoring
```bash
# Enable real-time metrics reporting
METRICS_REPORT_INTERVAL=10000 k6 run tests/load/api-endpoints.js

# Send metrics to webhook
METRICS_WEBHOOK_URL=http://localhost:3000/metrics k6 run tests/load/api-endpoints.js
```

## Performance Thresholds

### Default Thresholds
- **p95 Response Time**: < 500ms (normal), < 800ms (peak), < 2000ms (stress)
- **p99 Response Time**: < 1000ms (normal), < 1500ms (peak), < 5000ms (stress)
- **Failure Rate**: < 1% (normal), < 3% (peak), < 10% (stress)
- **Throughput**: > 100 RPS minimum

### Custom Thresholds
```bash
# Set custom thresholds
MIN_SUCCESS_RATE=99.0 MAX_AVG_RESPONSE_TIME=200 k6 run tests/load/api-endpoints.js
```

## Troubleshooting

### Common Issues

1. **Authentication Failures**
   ```bash
   # Check credentials
   TEST_USERNAME=your_username TEST_PASSWORD=your_password k6 run tests/load/api-endpoints.js
   ```

2. **Connection Errors**
   ```bash
   # Verify base URL
   BASE_URL=http://localhost:8080 k6 run tests/load/api-endpoints.js
   ```

3. **Database Connection Issues**
   ```bash
   # Check database configuration
   DB_HOST=localhost DB_PORT=5432 k6 run tests/load/database-performance.js
   ```

4. **High Error Rates**
   - Check server capacity
   - Verify database connections
   - Review circuit breaker settings
   - Monitor system resources

### Debug Mode
```bash
# Enable verbose logging
k6 run --verbose tests/load/api-endpoints.js

# Save detailed results
k6 run --out json=results.json tests/load/api-endpoints.js
```

## Advanced Usage

### Custom Scenarios
Create custom test scenarios by modifying `scenarios.js`:

```javascript
export const myCustomScenario = {
  executor: 'ramping-vus',
  startVUs: 1,
  stages: [
    { duration: '5m', target: 25 },
    { duration: '10m', target: 25 },
    { duration: '5m', target: 0 },
  ],
  tags: { test_type: 'custom' },
};
```

### Integration with CI/CD
```yaml
# GitHub Actions example
- name: Run Load Tests
  run: |
    k6 run --quiet --no-color tests/load/api-endpoints.js
    k6 run --quiet --no-color tests/load/database-performance.js
  env:
    BASE_URL: ${{ secrets.TEST_API_URL }}
    MIN_SUCCESS_RATE: 95.0
```

### Monitoring Integration
```bash
# InfluxDB + Grafana
k6 run --out influxdb=http://localhost:8086/k6 tests/load/api-endpoints.js

# Prometheus
k6 run --out experimental-prometheus-rw tests/load/api-endpoints.js
```

## Test Data Management

### Cleanup
The test suite automatically cleans up created test data. To manually cleanup:

```bash
# Clean up test data
CLEANUP_ONLY=true k6 run tests/load/api-endpoints.js
```

### Test Data Isolation
Use unique test environments or databases for load testing to avoid impacting production data.

## Performance Baselines

### Expected Performance (Normal Load)
- **API Endpoints**: p95 < 300ms
- **Database Queries**: p95 < 200ms
- **File Uploads**: p95 < 15s (depends on file size)
- **Cache Operations**: p95 < 50ms
- **Success Rate**: > 99%

### Scaling Limits
Based on testing, the system should handle:
- **Normal Load**: 50+ concurrent users
- **Peak Load**: 100+ concurrent users
- **File Processing**: 20+ concurrent uploads
- **Database**: 500+ queries per second

## Contributing

When adding new tests:

1. Follow the existing code structure
2. Add appropriate metrics collection
3. Include error handling
4. Document new environment variables
5. Update this README with new test scenarios

## Support

For issues or questions:
1. Check the troubleshooting section
2. Review k6 documentation: https://k6.io/docs/
3. Examine test logs and metrics
4. Verify system resource utilization