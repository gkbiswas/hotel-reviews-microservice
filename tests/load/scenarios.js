import { config } from './config.js';

// Load Testing Scenarios Configuration
// This file defines various load testing scenarios for different testing objectives

export const scenarios = {
  // Normal Load Testing - Simulates typical usage patterns
  normal_load: {
    executor: 'ramping-vus',
    exec: 'normalLoadTest',
    startVUs: 1,
    stages: [
      { duration: '2m', target: 10 },   // Gradual ramp-up to normal load
      { duration: '10m', target: 10 },  // Sustained normal load
      { duration: '2m', target: 0 },    // Graceful ramp-down
    ],
    gracefulRampDown: '30s',
    tags: { test_type: 'normal_load' },
    env: {
      TEST_TYPE: 'mixed',
      API_CALL_DISTRIBUTION: 'normal', // 70% reads, 20% writes, 10% complex operations
    }
  },

  // Peak Load Testing - Simulates high traffic periods
  peak_load: {
    executor: 'ramping-vus',
    exec: 'peakLoadTest',
    startVUs: 5,
    stages: [
      { duration: '3m', target: 30 },   // Quick ramp-up to peak load
      { duration: '15m', target: 50 },  // Sustained peak load
      { duration: '5m', target: 80 },   // Traffic spike simulation
      { duration: '10m', target: 50 },  // Return to sustained peak
      { duration: '5m', target: 0 },    // Ramp-down
    ],
    gracefulRampDown: '1m',
    tags: { test_type: 'peak_load' },
    env: {
      TEST_TYPE: 'mixed',
      API_CALL_DISTRIBUTION: 'peak', // 60% reads, 30% writes, 10% complex operations
      CACHE_TEST_ENABLED: 'true',
    }
  },

  // Stress Testing - Tests system beyond normal capacity
  stress_test: {
    executor: 'ramping-vus',
    exec: 'stressTest',
    startVUs: 10,
    stages: [
      { duration: '5m', target: 100 },  // Aggressive ramp-up
      { duration: '10m', target: 150 }, // High stress level
      { duration: '5m', target: 200 },  // Maximum stress
      { duration: '10m', target: 200 }, // Sustained maximum stress
      { duration: '5m', target: 100 },  // Gradual recovery
      { duration: '5m', target: 0 },    // Complete ramp-down
    ],
    gracefulRampDown: '2m',
    tags: { test_type: 'stress_test' },
    env: {
      TEST_TYPE: 'mixed',
      API_CALL_DISTRIBUTION: 'stress', // 50% reads, 40% writes, 10% complex operations
      ERROR_TOLERANCE: 'high',
      MAX_CONCURRENT_QUERIES: '50',
    }
  },

  // Spike Testing - Sudden traffic increases
  spike_test: {
    executor: 'ramping-vus',
    exec: 'spikeTest',
    startVUs: 5,
    stages: [
      { duration: '2m', target: 20 },   // Normal baseline
      { duration: '30s', target: 100 }, // Sudden spike
      { duration: '3m', target: 100 },  // Sustained spike
      { duration: '30s', target: 20 },  // Return to baseline
      { duration: '2m', target: 20 },   // Sustained baseline
      { duration: '1m', target: 0 },    // Ramp-down
    ],
    tags: { test_type: 'spike_test' },
    env: {
      TEST_TYPE: 'reviews', // Focus on most critical endpoints
      SPIKE_INTENSITY: 'high',
    }
  },

  // Volume Testing - Large amounts of data processing
  volume_test: {
    executor: 'constant-vus',
    exec: 'volumeTest',
    vus: 20,
    duration: '30m',
    tags: { test_type: 'volume_test' },
    env: {
      TEST_TYPE: 'mixed',
      VOLUME_TEST_MODE: 'true',
      BULK_INSERT_BATCH_SIZE: '500',
      FILE_TEST_TYPE: 'large_files',
    }
  },

  // Endurance Testing - Long-running stability test
  endurance_test: {
    executor: 'constant-vus',
    exec: 'enduranceTest',
    vus: 15,
    duration: '2h',
    tags: { test_type: 'endurance_test' },
    env: {
      TEST_TYPE: 'mixed',
      MEMORY_LEAK_DETECTION: 'true',
      CONNECTION_POOL_MONITORING: 'true',
    }
  },

  // Concurrent File Processing Test
  file_processing_load: {
    executor: 'per-vu-iterations',
    exec: 'fileProcessingTest',
    vus: 15,
    iterations: 10,
    maxDuration: '45m',
    tags: { test_type: 'file_processing' },
    env: {
      FILE_TEST_TYPE: 'concurrent',
      MAX_CONCURRENT_UPLOADS: '15',
      PROCESSING_TIMEOUT: '1800', // 30 minutes
    }
  },

  // Database Performance Test
  database_intensive: {
    executor: 'ramping-vus',
    exec: 'databasePerformanceTest',
    startVUs: 5,
    stages: [
      { duration: '3m', target: 25 },   // Ramp-up for database load
      { duration: '15m', target: 40 },  // Sustained database load
      { duration: '5m', target: 60 },   // Peak database load
      { duration: '10m', target: 40 },  // Return to sustained load
      { duration: '3m', target: 0 },    // Ramp-down
    ],
    gracefulRampDown: '2m',
    tags: { test_type: 'database_intensive' },
    env: {
      DB_TEST_TYPE: 'mixed',
      MAX_CONCURRENT_QUERIES: '30',
      CACHE_TEST_DURATION: '900', // 15 minutes
    }
  },

  // API-focused Load Test
  api_focused: {
    executor: 'ramping-arrival-rate',
    exec: 'apiEndpointsTest',
    startRate: 10,
    timeUnit: '1s',
    stages: [
      { duration: '2m', target: 20 },   // 20 req/s
      { duration: '10m', target: 50 },  // 50 req/s
      { duration: '5m', target: 100 },  // 100 req/s peak
      { duration: '10m', target: 50 },  // Return to 50 req/s
      { duration: '3m', target: 0 },    // Ramp-down
    ],
    preAllocatedVUs: 20,
    maxVUs: 100,
    tags: { test_type: 'api_focused' },
    env: {
      TEST_TYPE: 'mixed',
      API_RATE_LIMIT_TEST: 'true',
    }
  },

  // Circuit Breaker Test
  circuit_breaker_test: {
    executor: 'constant-vus',
    exec: 'circuitBreakerTest',
    vus: 30,
    duration: '15m',
    tags: { test_type: 'circuit_breaker' },
    env: {
      TEST_TYPE: 'error_scenarios',
      CIRCUIT_BREAKER_TEST: 'true',
      ERROR_INJECTION_RATE: '0.3', // 30% error rate to trigger circuit breakers
    }
  },

  // Authentication Load Test
  auth_load_test: {
    executor: 'ramping-vus',
    exec: 'authLoadTest',
    startVUs: 1,
    stages: [
      { duration: '2m', target: 20 },   // Authentication load
      { duration: '10m', target: 35 },  // Sustained auth load
      { duration: '3m', target: 50 },   // Peak auth load
      { duration: '5m', target: 35 },   // Return to sustained
      { duration: '2m', target: 0 },    // Ramp-down
    ],
    tags: { test_type: 'auth_load' },
    env: {
      TEST_TYPE: 'auth',
      AUTH_TOKEN_REFRESH_TEST: 'true',
      CONCURRENT_LOGIN_TEST: 'true',
    }
  },

  // Search Performance Test
  search_performance: {
    executor: 'constant-arrival-rate',
    exec: 'searchPerformanceTest',
    rate: 30,
    timeUnit: '1s',
    duration: '20m',
    preAllocatedVUs: 10,
    maxVUs: 50,
    tags: { test_type: 'search_performance' },
    env: {
      TEST_TYPE: 'search',
      SEARCH_COMPLEXITY: 'high',
      SEARCH_CACHE_TEST: 'true',
    }
  },

  // Chaos Testing - Simulates various failure scenarios
  chaos_test: {
    executor: 'constant-vus',
    exec: 'chaosTest',
    vus: 20,
    duration: '25m',
    tags: { test_type: 'chaos_test' },
    env: {
      TEST_TYPE: 'mixed',
      CHAOS_MODE: 'true',
      RANDOM_DELAYS: 'true',
      NETWORK_FAILURES: 'true',
      TIMEOUT_VARIATIONS: 'true',
    }
  },

  // Mobile/API Client Simulation
  mobile_client_simulation: {
    executor: 'ramping-vus',
    exec: 'mobileClientTest',
    startVUs: 5,
    stages: [
      { duration: '3m', target: 25 },   // Mobile users coming online
      { duration: '15m', target: 40 },  // Active mobile usage
      { duration: '5m', target: 60 },   // Peak mobile usage
      { duration: '10m', target: 40 },  // Return to normal
      { duration: '2m', target: 0 },    // Users going offline
    ],
    tags: { test_type: 'mobile_client' },
    env: {
      TEST_TYPE: 'mixed',
      MOBILE_API_PATTERNS: 'true',
      OFFLINE_SYNC_SIMULATION: 'true',
      BACKGROUND_REFRESH: 'true',
    }
  },

  // Microservices Integration Test
  microservices_integration: {
    executor: 'constant-vus',
    exec: 'microservicesTest',
    vus: 25,
    duration: '30m',
    tags: { test_type: 'microservices_integration' },
    env: {
      TEST_TYPE: 'mixed',
      SERVICE_MESH_TEST: 'true',
      INTER_SERVICE_CALLS: 'true',
      DISTRIBUTED_TRACING: 'true',
    }
  }
};

// Scenario-specific thresholds
export const scenarioThresholds = {
  normal_load: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'], // Very low failure rate for normal load
  },
  
  peak_load: {
    http_req_duration: ['p(95)<800', 'p(99)<1500'],
    http_req_failed: ['rate<0.03'],
  },
  
  stress_test: {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'],
    http_req_failed: ['rate<0.10'], // Higher tolerance for stress testing
  },
  
  spike_test: {
    http_req_duration: ['p(95)<1500', 'p(99)<3000'],
    http_req_failed: ['rate<0.05'],
  },
  
  volume_test: {
    http_req_duration: ['p(95)<3000', 'p(99)<8000'],
    http_req_failed: ['rate<0.05'],
    'http_req_duration{operation:bulk_insert}': ['p(95)<60000'], // 1 minute for bulk operations
  },
  
  endurance_test: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'],
    http_req_failed: ['rate<0.02'],
    // Memory and connection pool metrics would be monitored externally
  },
  
  file_processing: {
    http_req_duration: ['p(95)<30000', 'p(99)<60000'], // File processing is slower
    http_req_failed: ['rate<0.08'],
  },
  
  database_intensive: {
    http_req_duration: ['p(95)<1500', 'p(99)<3000'],
    http_req_failed: ['rate<0.05'],
    database_operations: ['p(95)<1000'],
  },
  
  api_focused: {
    http_req_duration: ['p(95)<600', 'p(99)<1200'],
    http_req_failed: ['rate<0.02'],
    http_reqs: ['rate>40'], // Minimum request rate
  },
  
  circuit_breaker: {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'],
    http_req_failed: ['rate<0.50'], // High failure rate expected to test circuit breakers
    circuit_breaker_open: ['rate>0.1'], // Expect circuit breaker activations
  },
  
  auth_load: {
    http_req_duration: ['p(95)<400', 'p(99)<800'],
    http_req_failed: ['rate<0.02'],
    'http_req_duration{endpoint:auth}': ['p(95)<300'],
  },
  
  search_performance: {
    http_req_duration: ['p(95)<800', 'p(99)<1500'],
    http_req_failed: ['rate<0.03'],
    'http_req_duration{operation:complex_search}': ['p(95)<1200'],
  }
};

// Test execution functions for different scenarios
export function normalLoadTest() {
  // Import and execute normal load patterns
  import('./api-endpoints.js').then(module => {
    module.default();
  });
}

export function peakLoadTest() {
  // Import and execute peak load patterns with cache testing
  import('./api-endpoints.js').then(module => {
    module.default();
  });
  
  // Additional cache performance testing
  if (__ENV.CACHE_TEST_ENABLED === 'true') {
    import('./database-performance.js').then(module => {
      module.testCachePerformance();
    });
  }
}

export function stressTest() {
  // Execute stress testing with higher error tolerance
  import('./api-endpoints.js').then(module => {
    module.default();
  });
  
  // Additional database stress testing
  import('./database-performance.js').then(module => {
    module.default();
  });
}

export function spikeTest() {
  // Focus on critical review endpoints during spike
  import('./api-endpoints.js').then(module => {
    module.default();
  });
}

export function volumeTest() {
  // Execute volume testing with large data sets
  import('./file-processing.js').then(module => {
    module.default();
  });
  
  import('./database-performance.js').then(module => {
    module.default();
  });
}

export function enduranceTest() {
  // Long-running stability test
  import('./api-endpoints.js').then(module => {
    module.default();
  });
}

export function fileProcessingTest() {
  // Dedicated file processing test
  import('./file-processing.js').then(module => {
    module.default();
  });
}

export function databasePerformanceTest() {
  // Dedicated database performance test
  import('./database-performance.js').then(module => {
    module.default();
  });
}

export function apiEndpointsTest() {
  // API-focused testing
  import('./api-endpoints.js').then(module => {
    module.default();
  });
}

export function circuitBreakerTest() {
  // Circuit breaker and resilience testing
  import('./api-endpoints.js').then(module => {
    module.default();
  });
  
  // Additional error injection and circuit breaker testing would go here
}

export function authLoadTest() {
  // Authentication-focused load testing
  import('./api-endpoints.js').then(module => {
    module.testAuthenticationEndpoints();
  });
}

export function searchPerformanceTest() {
  // Search performance testing
  import('./api-endpoints.js').then(module => {
    module.testSearchEndpoints();
  });
}

export function chaosTest() {
  // Chaos engineering testing with random failures
  import('./api-endpoints.js').then(module => {
    module.default();
  });
  
  // Chaos testing would include random delays, timeouts, and failures
}

export function mobileClientTest() {
  // Mobile client usage pattern simulation
  import('./api-endpoints.js').then(module => {
    module.default();
  });
}

export function microservicesTest() {
  // Microservices integration testing
  import('./api-endpoints.js').then(module => {
    module.default();
  });
  
  import('./database-performance.js').then(module => {
    module.default();
  });
}

// Scenario selection helper
export function getScenarioConfig(scenarioName) {
  const scenario = scenarios[scenarioName];
  const thresholds = scenarioThresholds[scenarioName] || {};
  
  if (!scenario) {
    throw new Error(`Unknown scenario: ${scenarioName}`);
  }
  
  return {
    scenarios: { [scenarioName]: scenario },
    thresholds: {
      ...config.thresholds, // Base thresholds
      ...thresholds, // Scenario-specific thresholds
    }
  };
}

// Multi-scenario configuration
export function getMultiScenarioConfig(scenarioNames) {
  const selectedScenarios = {};
  const combinedThresholds = { ...config.thresholds };
  
  scenarioNames.forEach(name => {
    if (scenarios[name]) {
      selectedScenarios[name] = scenarios[name];
      
      // Merge scenario-specific thresholds
      const scenarioThreshold = scenarioThresholds[name];
      if (scenarioThreshold) {
        Object.assign(combinedThresholds, scenarioThreshold);
      }
    }
  });
  
  return {
    scenarios: selectedScenarios,
    thresholds: combinedThresholds,
  };
}

// Default export for k6 options
export const options = getScenarioConfig(__ENV.SCENARIO || 'normal_load');