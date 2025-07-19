import http from 'k6/http';
import { check } from 'k6';
import { Trend, Counter, Rate, Gauge } from 'k6/metrics';

// Custom metrics for detailed performance monitoring
export const customMetrics = {
  // API-specific metrics
  apiResponseTime: new Trend('api_response_time', true),
  apiSuccessRate: new Rate('api_success_rate'),
  apiThroughput: new Counter('api_throughput'),
  
  // Database-specific metrics
  databaseQueryTime: new Trend('database_query_time', true),
  databaseConnectionTime: new Trend('database_connection_time', true),
  databaseOperationsCount: new Counter('database_operations_count'),
  databaseErrorRate: new Rate('database_error_rate'),
  
  // Cache-specific metrics
  cacheHitRate: new Rate('cache_hit_rate'),
  cacheMissRate: new Rate('cache_miss_rate'),
  cacheResponseTime: new Trend('cache_response_time', true),
  
  // File processing metrics
  fileUploadSize: new Trend('file_upload_size', true),
  fileProcessingTime: new Trend('file_processing_time', true),
  fileProcessingSuccessRate: new Rate('file_processing_success_rate'),
  fileConcurrentUploads: new Gauge('file_concurrent_uploads'),
  
  // Circuit breaker metrics
  circuitBreakerOpenRate: new Rate('circuit_breaker_open_rate'),
  circuitBreakerRequestsBlocked: new Counter('circuit_breaker_requests_blocked'),
  
  // Authentication metrics
  authTokenGenerationTime: new Trend('auth_token_generation_time', true),
  authSuccessRate: new Rate('auth_success_rate'),
  authTokenRefreshCount: new Counter('auth_token_refresh_count'),
  
  // Business metrics
  reviewsCreatedCount: new Counter('reviews_created_count'),
  reviewsSearchedCount: new Counter('reviews_searched_count'),
  hotelsQueriedCount: new Counter('hotels_queried_count'),
  
  // Error tracking metrics
  httpErrorsByCode: new Counter('http_errors_by_code'),
  applicationErrorsCount: new Counter('application_errors_count'),
  timeoutErrorsCount: new Counter('timeout_errors_count'),
  
  // Resource utilization metrics (simulated)
  memoryUsageEstimate: new Gauge('memory_usage_estimate'),
  cpuUsageEstimate: new Gauge('cpu_usage_estimate'),
  connectionPoolUsage: new Gauge('connection_pool_usage'),
};

// Metrics collector class
export class MetricsCollector {
  constructor() {
    this.startTime = Date.now();
    this.requestCount = 0;
    this.errorCount = 0;
    this.successCount = 0;
    this.totalResponseTime = 0;
    
    // Operation counters
    this.operationCounts = {
      create: 0,
      read: 0,
      update: 0,
      delete: 0,
      search: 0,
      file_upload: 0,
      auth: 0,
    };
    
    // Response time buckets
    this.responseTimeBuckets = {
      fast: 0,      // < 200ms
      normal: 0,    // 200ms - 1s
      slow: 0,      // 1s - 5s
      very_slow: 0, // > 5s
    };
    
    // Error type tracking
    this.errorTypes = {
      network: 0,
      timeout: 0,
      server_error: 0,
      client_error: 0,
      authentication: 0,
      authorization: 0,
      validation: 0,
    };
  }

  // Record API request metrics
  recordApiRequest(response, operation = 'unknown', endpoint = 'unknown') {
    this.requestCount++;
    
    const responseTime = response.timings.duration;
    this.totalResponseTime += responseTime;
    
    // Record in custom metrics
    customMetrics.apiResponseTime.add(responseTime, {
      endpoint: endpoint,
      operation: operation,
    });
    
    customMetrics.apiThroughput.add(1, {
      endpoint: endpoint,
      operation: operation,
    });
    
    // Categorize response time
    if (responseTime < 200) {
      this.responseTimeBuckets.fast++;
    } else if (responseTime < 1000) {
      this.responseTimeBuckets.normal++;
    } else if (responseTime < 5000) {
      this.responseTimeBuckets.slow++;
    } else {
      this.responseTimeBuckets.very_slow++;
    }
    
    // Track operation types
    if (this.operationCounts.hasOwnProperty(operation)) {
      this.operationCounts[operation]++;
    }
    
    // Success/error tracking
    if (response.status >= 200 && response.status < 400) {
      this.successCount++;
      customMetrics.apiSuccessRate.add(true, {
        endpoint: endpoint,
        operation: operation,
      });
    } else {
      this.errorCount++;
      customMetrics.apiSuccessRate.add(false, {
        endpoint: endpoint,
        operation: operation,
      });
      
      // Record error by status code
      customMetrics.httpErrorsByCode.add(1, {
        status_code: response.status.toString(),
        endpoint: endpoint,
      });
      
      // Categorize error type
      this.categorizeError(response.status);
    }
    
    // Check for circuit breaker headers
    if (response.headers['X-Circuit-Breaker-State']) {
      const state = response.headers['X-Circuit-Breaker-State'];
      if (state === 'open') {
        customMetrics.circuitBreakerOpenRate.add(true);
        customMetrics.circuitBreakerRequestsBlocked.add(1);
      } else {
        customMetrics.circuitBreakerOpenRate.add(false);
      }
    }
    
    // Estimate resource usage based on response times and patterns
    this.updateResourceEstimates(responseTime, response.status);
  }

  // Record database operation metrics
  recordDatabaseOperation(duration, operation = 'query', success = true) {
    customMetrics.databaseQueryTime.add(duration, {
      operation: operation,
    });
    
    customMetrics.databaseOperationsCount.add(1, {
      operation: operation,
    });
    
    customMetrics.databaseErrorRate.add(!success, {
      operation: operation,
    });
    
    // Estimate connection pool usage
    const poolUsage = Math.min(100, (duration / 1000) * 10); // Rough estimate
    customMetrics.connectionPoolUsage.set(poolUsage);
  }

  // Record cache operation metrics
  recordCacheOperation(hit = true, responseTime = 0, operation = 'get') {
    if (hit) {
      customMetrics.cacheHitRate.add(true, { operation: operation });
      customMetrics.cacheMissRate.add(false, { operation: operation });
    } else {
      customMetrics.cacheHitRate.add(false, { operation: operation });
      customMetrics.cacheMissRate.add(true, { operation: operation });
    }
    
    if (responseTime > 0) {
      customMetrics.cacheResponseTime.add(responseTime, {
        hit: hit.toString(),
        operation: operation,
      });
    }
  }

  // Record file processing metrics
  recordFileProcessing(fileSize, processingTime, success = true, concurrentUploads = 1) {
    customMetrics.fileUploadSize.add(fileSize);
    customMetrics.fileProcessingTime.add(processingTime);
    customMetrics.fileProcessingSuccessRate.add(success);
    customMetrics.fileConcurrentUploads.set(concurrentUploads);
  }

  // Record authentication metrics
  recordAuthentication(operation, responseTime, success = true) {
    if (operation === 'login' || operation === 'register') {
      customMetrics.authTokenGenerationTime.add(responseTime, {
        operation: operation,
      });
    }
    
    customMetrics.authSuccessRate.add(success, {
      operation: operation,
    });
    
    if (operation === 'refresh') {
      customMetrics.authTokenRefreshCount.add(1);
    }
  }

  // Record business operation metrics
  recordBusinessOperation(operation, data = {}) {
    switch (operation) {
      case 'review_created':
        customMetrics.reviewsCreatedCount.add(1, data);
        break;
      case 'review_searched':
        customMetrics.reviewsSearchedCount.add(1, data);
        break;
      case 'hotel_queried':
        customMetrics.hotelsQueriedCount.add(1, data);
        break;
    }
  }

  // Categorize error types
  categorizeError(statusCode) {
    if (statusCode === 401) {
      this.errorTypes.authentication++;
    } else if (statusCode === 403) {
      this.errorTypes.authorization++;
    } else if (statusCode >= 400 && statusCode < 500) {
      this.errorTypes.client_error++;
      if (statusCode === 422) {
        this.errorTypes.validation++;
      }
    } else if (statusCode >= 500) {
      this.errorTypes.server_error++;
    } else if (statusCode === 0) {
      this.errorTypes.network++;
    }
  }

  // Update resource usage estimates
  updateResourceEstimates(responseTime, statusCode) {
    // Simple heuristic-based resource usage estimation
    const baseMemory = 50; // Base memory usage percentage
    const baseCpu = 30;    // Base CPU usage percentage
    
    // Increase estimates based on response time
    const memoryIncrease = Math.min(40, responseTime / 100);
    const cpuIncrease = Math.min(50, responseTime / 50);
    
    // Adjust for error conditions
    const errorMultiplier = (statusCode >= 500) ? 1.5 : 1.0;
    
    const estimatedMemory = Math.min(100, (baseMemory + memoryIncrease) * errorMultiplier);
    const estimatedCpu = Math.min(100, (baseCpu + cpuIncrease) * errorMultiplier);
    
    customMetrics.memoryUsageEstimate.set(estimatedMemory);
    customMetrics.cpuUsageEstimate.set(estimatedCpu);
  }

  // Generate summary report
  generateSummary() {
    const duration = (Date.now() - this.startTime) / 1000; // in seconds
    const avgResponseTime = this.requestCount > 0 ? this.totalResponseTime / this.requestCount : 0;
    const successRate = this.requestCount > 0 ? (this.successCount / this.requestCount) * 100 : 0;
    const throughput = this.requestCount / duration; // requests per second
    
    return {
      test_duration_seconds: duration,
      total_requests: this.requestCount,
      successful_requests: this.successCount,
      failed_requests: this.errorCount,
      success_rate_percent: successRate.toFixed(2),
      average_response_time_ms: avgResponseTime.toFixed(2),
      throughput_rps: throughput.toFixed(2),
      
      operation_breakdown: this.operationCounts,
      response_time_distribution: this.responseTimeBuckets,
      error_type_breakdown: this.errorTypes,
      
      performance_classification: this.classifyPerformance(avgResponseTime, successRate),
    };
  }

  // Classify overall performance
  classifyPerformance(avgResponseTime, successRate) {
    if (successRate >= 99 && avgResponseTime < 200) {
      return 'excellent';
    } else if (successRate >= 95 && avgResponseTime < 500) {
      return 'good';
    } else if (successRate >= 90 && avgResponseTime < 1000) {
      return 'acceptable';
    } else if (successRate >= 80 && avgResponseTime < 2000) {
      return 'poor';
    } else {
      return 'unacceptable';
    }
  }

  // Export metrics to external systems
  exportMetrics(format = 'json') {
    const summary = this.generateSummary();
    
    switch (format) {
      case 'prometheus':
        return this.formatPrometheusMetrics(summary);
      case 'influxdb':
        return this.formatInfluxDBMetrics(summary);
      case 'json':
      default:
        return JSON.stringify(summary, null, 2);
    }
  }

  // Format metrics for Prometheus
  formatPrometheusMetrics(summary) {
    let prometheus = '';
    
    prometheus += `# HELP load_test_requests_total Total number of requests\n`;
    prometheus += `# TYPE load_test_requests_total counter\n`;
    prometheus += `load_test_requests_total ${summary.total_requests}\n\n`;
    
    prometheus += `# HELP load_test_success_rate Success rate percentage\n`;
    prometheus += `# TYPE load_test_success_rate gauge\n`;
    prometheus += `load_test_success_rate ${summary.success_rate_percent}\n\n`;
    
    prometheus += `# HELP load_test_avg_response_time Average response time in milliseconds\n`;
    prometheus += `# TYPE load_test_avg_response_time gauge\n`;
    prometheus += `load_test_avg_response_time ${summary.average_response_time_ms}\n\n`;
    
    prometheus += `# HELP load_test_throughput Throughput in requests per second\n`;
    prometheus += `# TYPE load_test_throughput gauge\n`;
    prometheus += `load_test_throughput ${summary.throughput_rps}\n\n`;
    
    // Operation breakdown
    Object.entries(summary.operation_breakdown).forEach(([operation, count]) => {
      prometheus += `load_test_operations_total{operation="${operation}"} ${count}\n`;
    });
    
    return prometheus;
  }

  // Format metrics for InfluxDB
  formatInfluxDBMetrics(summary) {
    const timestamp = Date.now() * 1000000; // InfluxDB expects nanoseconds
    let influx = '';
    
    influx += `load_test_summary,test_type=${__ENV.TEST_TYPE || 'unknown'} `;
    influx += `total_requests=${summary.total_requests}i,`;
    influx += `success_rate=${summary.success_rate_percent},`;
    influx += `avg_response_time=${summary.average_response_time_ms},`;
    influx += `throughput=${summary.throughput_rps} ${timestamp}\n`;
    
    // Operation breakdown
    Object.entries(summary.operation_breakdown).forEach(([operation, count]) => {
      influx += `load_test_operations,operation=${operation},test_type=${__ENV.TEST_TYPE || 'unknown'} `;
      influx += `count=${count}i ${timestamp}\n`;
    });
    
    // Response time distribution
    Object.entries(summary.response_time_distribution).forEach(([bucket, count]) => {
      influx += `load_test_response_times,bucket=${bucket},test_type=${__ENV.TEST_TYPE || 'unknown'} `;
      influx += `count=${count}i ${timestamp}\n`;
    });
    
    return influx;
  }
}

// Real-time metrics reporting function
export function reportRealTimeMetrics(metricsCollector) {
  const interval = parseInt(__ENV.METRICS_REPORT_INTERVAL) || 30000; // 30 seconds default
  
  setInterval(() => {
    const summary = metricsCollector.generateSummary();
    
    // Send to monitoring endpoints if configured
    if (__ENV.METRICS_WEBHOOK_URL) {
      http.post(__ENV.METRICS_WEBHOOK_URL, JSON.stringify(summary), {
        headers: { 'Content-Type': 'application/json' },
        tags: { endpoint: 'metrics', operation: 'webhook' }
      });
    }
    
    // Log summary to console
    console.log(`[METRICS] ${new Date().toISOString()} - Performance: ${summary.performance_classification}, RPS: ${summary.throughput_rps}, Success: ${summary.success_rate_percent}%`);
    
  }, interval);
}

// Metrics validation functions
export function validateMetricsThresholds(metricsCollector) {
  const summary = metricsCollector.generateSummary();
  const thresholds = {
    min_success_rate: parseFloat(__ENV.MIN_SUCCESS_RATE) || 95.0,
    max_avg_response_time: parseFloat(__ENV.MAX_AVG_RESPONSE_TIME) || 1000.0,
    min_throughput: parseFloat(__ENV.MIN_THROUGHPUT) || 10.0,
  };
  
  const validations = {
    success_rate_ok: parseFloat(summary.success_rate_percent) >= thresholds.min_success_rate,
    response_time_ok: parseFloat(summary.average_response_time_ms) <= thresholds.max_avg_response_time,
    throughput_ok: parseFloat(summary.throughput_rps) >= thresholds.min_throughput,
  };
  
  const allValid = Object.values(validations).every(v => v);
  
  return {
    passed: allValid,
    validations: validations,
    thresholds: thresholds,
    actual: {
      success_rate: summary.success_rate_percent,
      avg_response_time: summary.average_response_time_ms,
      throughput: summary.throughput_rps,
    }
  };
}

// Global metrics collector instance
export const globalMetricsCollector = new MetricsCollector();

// Helper function to record common request patterns
export function recordRequest(response, endpoint, operation, additionalData = {}) {
  globalMetricsCollector.recordApiRequest(response, operation, endpoint);
  
  // Record specific business operations
  if (operation === 'create' && endpoint === 'reviews') {
    globalMetricsCollector.recordBusinessOperation('review_created', additionalData);
  } else if (operation === 'search' && endpoint === 'reviews') {
    globalMetricsCollector.recordBusinessOperation('review_searched', additionalData);
  } else if (operation === 'get' && endpoint === 'hotels') {
    globalMetricsCollector.recordBusinessOperation('hotel_queried', additionalData);
  }
  
  // Auto-detect cache operations from headers
  if (response.headers['X-Cache-Status']) {
    const cacheHit = response.headers['X-Cache-Status'] === 'hit';
    globalMetricsCollector.recordCacheOperation(cacheHit, response.timings.duration, operation);
  }
  
  // Auto-detect database operations from timing headers
  if (response.headers['X-Database-Query-Time']) {
    const dbTime = parseFloat(response.headers['X-Database-Query-Time']);
    globalMetricsCollector.recordDatabaseOperation(dbTime, operation, response.status < 400);
  }
}

// Export function for test teardown
export function exportFinalMetrics() {
  const summary = globalMetricsCollector.generateSummary();
  
  console.log('\n=== FINAL LOAD TEST METRICS SUMMARY ===');
  console.log(JSON.stringify(summary, null, 2));
  
  // Validate against thresholds
  const validation = validateMetricsThresholds(globalMetricsCollector);
  
  console.log('\n=== THRESHOLD VALIDATION ===');
  console.log(`Overall Status: ${validation.passed ? 'PASSED' : 'FAILED'}`);
  console.log('Validations:');
  Object.entries(validation.validations).forEach(([key, passed]) => {
    console.log(`  ${key}: ${passed ? 'PASS' : 'FAIL'}`);
  });
  
  // Export to external systems if configured
  if (__ENV.EXPORT_METRICS === 'true') {
    const format = __ENV.METRICS_FORMAT || 'json';
    const exportedMetrics = globalMetricsCollector.exportMetrics(format);
    
    if (__ENV.METRICS_OUTPUT_FILE) {
      // In a real scenario, you'd write to a file or send to an external system
      console.log(`\n=== EXPORTED METRICS (${format.toUpperCase()}) ===`);
      console.log(exportedMetrics);
    }
  }
  
  return summary;
}