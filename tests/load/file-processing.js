import http from 'k6/http';
import { check, sleep } from 'k6';
import { config, fileProcessing, createHeaders } from './config.js';
import { AuthManager, FileProcessor, PerformanceMonitor, waitForAsyncOperation } from './utils.js';

// Global instances
let authManager;
let fileProcessor;
let performanceMonitor;

// Test configuration for file processing
const FILE_PROCESSING_CONFIG = {
  maxConcurrentUploads: parseInt(__ENV.MAX_CONCURRENT_UPLOADS) || 10,
  maxFileSize: parseInt(__ENV.MAX_FILE_SIZE) || 5 * 1024 * 1024, // 5MB
  processingTimeout: parseInt(__ENV.PROCESSING_TIMEOUT) || 300, // 5 minutes
  fileTypes: ['csv', 'json', 'xml'],
  testFileSizes: [1000, 5000, 10000, 25000, 50000], // Number of records
};

// Setup function
export function setup() {
  authManager = new AuthManager();
  fileProcessor = new FileProcessor(authManager);
  performanceMonitor = new PerformanceMonitor();

  // Ensure we have valid authentication
  const token = authManager.getValidToken();
  if (!token) {
    throw new Error('Failed to authenticate for file processing tests');
  }

  return {
    token: token,
    startTime: Date.now(),
  };
}

// Main test function
export default function(data) {
  if (!authManager) {
    authManager = new AuthManager();
    fileProcessor = new FileProcessor(authManager);
    performanceMonitor = new PerformanceMonitor();
  }

  const testType = __ENV.FILE_TEST_TYPE || 'mixed';
  
  switch (testType) {
    case 'concurrent':
      testConcurrentFileUploads();
      break;
    case 'large_files':
      testLargeFileProcessing();
      break;
    case 'batch':
      testBatchFileProcessing();
      break;
    case 'stress':
      testFileProcessingStress();
      break;
    case 'error_scenarios':
      testFileProcessingErrors();
      break;
    default:
      testMixedFileProcessing();
  }

  // Add think time between operations
  sleep(Math.random() * 3 + 1);
}

// Test concurrent file uploads
function testConcurrentFileUploads() {
  const group = 'Concurrent File Processing';
  
  // Generate multiple CSV files of different sizes
  const files = [];
  for (let i = 0; i < FILE_PROCESSING_CONFIG.maxConcurrentUploads; i++) {
    const recordCount = FILE_PROCESSING_CONFIG.testFileSizes[
      Math.floor(Math.random() * FILE_PROCESSING_CONFIG.testFileSizes.length)
    ];
    const csvContent = fileProcessor.generateLargeCsvContent(recordCount);
    files.push({
      content: csvContent,
      filename: `concurrent_test_${i}_${recordCount}records.csv`,
      recordCount: recordCount,
    });
  }

  // Upload files concurrently
  const uploadPromises = files.map(file => {
    return new Promise((resolve) => {
      const startTime = Date.now();
      const response = fileProcessor.uploadCsvFile(file.content, file.filename);
      const endTime = Date.now();
      
      const success = check(response, {
        'concurrent upload successful': (r) => r.status === 202 || r.status === 200,
        'upload response time acceptable': () => (endTime - startTime) < 30000, // 30 seconds
      });

      performanceMonitor.recordRequest(response, 'file/concurrent_upload');
      
      resolve({
        success: success,
        response: response,
        file: file,
        uploadTime: endTime - startTime,
      });
    });
  });

  // Wait for all uploads to complete
  const results = Promise.all(uploadPromises);
  
  // Analyze concurrent upload results
  let successfulUploads = 0;
  let totalUploadTime = 0;
  
  results.then(uploadResults => {
    uploadResults.forEach(result => {
      if (result.success) {
        successfulUploads++;
      }
      totalUploadTime += result.uploadTime;
    });

    const avgUploadTime = totalUploadTime / uploadResults.length;
    console.log(`Concurrent uploads: ${successfulUploads}/${uploadResults.length} successful, avg time: ${avgUploadTime}ms`);
  });
}

// Test large file processing
function testLargeFileProcessing() {
  const group = 'Large File Processing';
  
  // Test with increasingly large files
  const largeSizes = [10000, 25000, 50000, 100000]; // Number of records
  
  largeSizes.forEach(recordCount => {
    const startTime = Date.now();
    const csvContent = fileProcessor.generateLargeCsvContent(recordCount);
    const filename = `large_file_${recordCount}records.csv`;
    
    // Upload large file
    const uploadResponse = fileProcessor.uploadCsvFile(csvContent, filename);
    const uploadTime = Date.now() - startTime;
    
    const uploadSuccess = check(uploadResponse, {
      [`large file upload successful (${recordCount} records)`]: (r) => r.status === 202 || r.status === 200,
      'large file upload within time limit': () => uploadTime < 60000, // 1 minute
      'large file response has processing ID': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.data && (body.data.processing_id || body.data.id);
        } catch (e) {
          return false;
        }
      },
    });

    performanceMonitor.recordRequest(uploadResponse, `file/large_upload_${recordCount}`);

    if (uploadSuccess && uploadResponse.status === 202) {
      // Monitor processing status for large files
      const body = JSON.parse(uploadResponse.body);
      const processingId = body.data.processing_id || body.data.id;
      
      if (processingId) {
        monitorFileProcessingStatus(processingId, recordCount);
      }
    }

    // Ensure memory usage doesn't grow unbounded
    if (recordCount >= 50000) {
      sleep(5); // Give system time to clean up
    }
  });
}

// Test batch file processing
function testBatchFileProcessing() {
  const group = 'Batch File Processing';
  
  const batchSize = 5;
  const recordsPerFile = 5000;
  
  // Create batch of files
  const batchFiles = [];
  for (let i = 0; i < batchSize; i++) {
    const csvContent = fileProcessor.generateLargeCsvContent(recordsPerFile);
    const filename = `batch_${i}_${recordsPerFile}records.csv`;
    batchFiles.push({ content: csvContent, filename: filename });
  }

  // Upload batch with controlled timing
  const batchStartTime = Date.now();
  const batchResults = [];
  
  batchFiles.forEach((file, index) => {
    const response = fileProcessor.uploadCsvFile(file.content, file.filename);
    
    const success = check(response, {
      [`batch file ${index + 1} upload successful`]: (r) => r.status === 202 || r.status === 200,
    });

    batchResults.push({
      success: success,
      response: response,
      index: index,
    });

    performanceMonitor.recordRequest(response, 'file/batch_upload');
    
    // Small delay between uploads to simulate realistic batch processing
    sleep(0.5);
  });

  const batchEndTime = Date.now();
  const batchDuration = batchEndTime - batchStartTime;
  
  const successfulBatchUploads = batchResults.filter(r => r.success).length;
  
  check(null, {
    'batch processing completed in reasonable time': () => batchDuration < 120000, // 2 minutes
    'majority of batch uploads successful': () => successfulBatchUploads >= Math.ceil(batchSize * 0.8),
  });

  console.log(`Batch processing: ${successfulBatchUploads}/${batchSize} files successful in ${batchDuration}ms`);
}

// Test file processing under stress
function testFileProcessingStress() {
  const group = 'File Processing Stress';
  
  // Rapid fire small file uploads
  const stressTestDuration = 60000; // 1 minute
  const uploadInterval = 1000; // 1 second between uploads
  const recordsPerFile = 1000;
  
  const startTime = Date.now();
  let uploadCount = 0;
  let successCount = 0;
  
  while ((Date.now() - startTime) < stressTestDuration) {
    const csvContent = fileProcessor.generateLargeCsvContent(recordsPerFile);
    const filename = `stress_${uploadCount}_${Date.now()}.csv`;
    
    const response = fileProcessor.uploadCsvFile(csvContent, filename);
    uploadCount++;
    
    const success = check(response, {
      'stress test upload successful': (r) => r.status === 202 || r.status === 200,
      'stress test response time acceptable': (r) => r.timings.duration < 10000, // 10 seconds
    });

    if (success) {
      successCount++;
    }

    performanceMonitor.recordRequest(response, 'file/stress_upload');
    
    sleep(uploadInterval / 1000);
  }

  const successRate = successCount / uploadCount;
  
  check(null, {
    'stress test success rate acceptable': () => successRate >= 0.8, // 80% success rate
    'stress test completed minimum uploads': () => uploadCount >= 30,
  });

  console.log(`Stress test: ${successCount}/${uploadCount} uploads successful (${(successRate * 100).toFixed(2)}%)`);
}

// Test file processing error scenarios
function testFileProcessingErrors() {
  const group = 'File Processing Errors';
  
  // Test invalid file formats
  const invalidCsvContent = 'invalid,csv,content\nwith,malformed\ndata';
  const invalidResponse = fileProcessor.uploadCsvFile(invalidCsvContent, 'invalid.csv');
  
  check(invalidResponse, {
    'invalid CSV handled gracefully': (r) => r.status === 400 || r.status === 422,
  });

  // Test empty file
  const emptyResponse = fileProcessor.uploadCsvFile('', 'empty.csv');
  
  check(emptyResponse, {
    'empty file handled gracefully': (r) => r.status === 400 || r.status === 422,
  });

  // Test oversized file (simulate)
  const oversizedContent = fileProcessor.generateLargeCsvContent(200000); // Very large file
  const oversizedResponse = fileProcessor.uploadCsvFile(oversizedContent, 'oversized.csv');
  
  check(oversizedResponse, {
    'oversized file handled appropriately': (r) => r.status === 413 || r.status === 400 || r.status === 202,
  });

  // Test malformed JSON in CSV
  const malformedCsvContent = `hotel_id,rating,comment,metadata
test-hotel,5,"Great stay","{invalid: json}"
test-hotel-2,4,"Good hotel","{\"valid\": \"json\"}"`;
  
  const malformedResponse = fileProcessor.uploadCsvFile(malformedCsvContent, 'malformed.csv');
  
  check(malformedResponse, {
    'malformed data handled gracefully': (r) => r.status <= 422, // Should not cause server error
  });

  performanceMonitor.recordRequest(invalidResponse, 'file/error_invalid');
  performanceMonitor.recordRequest(emptyResponse, 'file/error_empty');
  performanceMonitor.recordRequest(oversizedResponse, 'file/error_oversized');
  performanceMonitor.recordRequest(malformedResponse, 'file/error_malformed');
}

// Test mixed file processing scenarios
function testMixedFileProcessing() {
  const scenarios = [
    () => testSingleFileUpload(),
    () => testFileWithValidation(),
    () => testFileProcessingStatus(),
    () => testConcurrentSmallFiles(),
  ];

  // Randomly select and execute a scenario
  const randomScenario = scenarios[Math.floor(Math.random() * scenarios.length)];
  randomScenario();
}

// Test single file upload and processing
function testSingleFileUpload() {
  const recordCount = Math.floor(Math.random() * 5000) + 500; // 500-5500 records
  const csvContent = fileProcessor.generateLargeCsvContent(recordCount);
  const filename = `single_${recordCount}_${Date.now()}.csv`;

  const response = fileProcessor.uploadCsvFile(csvContent, filename);
  
  const success = check(response, {
    'single file upload successful': (r) => r.status === 202 || r.status === 200,
    'single file response time reasonable': (r) => r.timings.duration < 15000,
  });

  performanceMonitor.recordRequest(response, 'file/single_upload');

  if (success && response.status === 202) {
    const body = JSON.parse(response.body);
    const processingId = body.data && (body.data.processing_id || body.data.id);
    
    if (processingId) {
      // Quick status check
      sleep(2);
      const statusResponse = fileProcessor.checkProcessingStatus(processingId);
      
      check(statusResponse, {
        'processing status check successful': (r) => r.status === 200,
      });

      performanceMonitor.recordRequest(statusResponse, 'file/status_check');
    }
  }
}

// Test file with validation scenarios
function testFileWithValidation() {
  // Create CSV with mix of valid and invalid data
  const mixedCsvContent = `hotel_id,reviewer_name,rating,title,comment,review_date,language
valid-hotel-1,John Doe,4.5,"Great stay","Excellent service",2024-01-15,en
invalid-hotel,Jane Smith,6.0,"Invalid rating","Rating too high",2024-01-16,en
valid-hotel-2,Bob Johnson,3.5,"Average stay","It was okay",invalid-date,en
valid-hotel-3,Alice Brown,4.0,"Good hotel","Nice experience",2024-01-18,en`;

  const response = fileProcessor.uploadCsvFile(mixedCsvContent, 'validation_test.csv');
  
  check(response, {
    'validation test upload accepted': (r) => r.status === 202 || r.status === 200,
    'validation test has response body': (r) => r.body && r.body.length > 0,
  });

  performanceMonitor.recordRequest(response, 'file/validation_test');
}

// Test concurrent small files
function testConcurrentSmallFiles() {
  const concurrentCount = 3;
  const recordsPerFile = 100;
  
  const requests = [];
  
  for (let i = 0; i < concurrentCount; i++) {
    const csvContent = fileProcessor.generateLargeCsvContent(recordsPerFile);
    const filename = `concurrent_small_${i}_${Date.now()}.csv`;
    
    // Prepare batch request
    requests.push([
      'POST',
      `${config.baseUrl}/api/v1/reviews/upload`,
      {
        file: http.file(csvContent, filename, 'text/csv'),
        provider_id: 'test-provider-id',
      },
      {
        headers: {
          'Authorization': `Bearer ${authManager.getValidToken()}`,
        },
        tags: { endpoint: 'file_processing', operation: 'concurrent_small' }
      }
    ]);
  }

  // Execute batch request
  const responses = http.batch(requests);
  
  let successCount = 0;
  responses.forEach((response, index) => {
    const success = check(response, {
      [`concurrent small file ${index + 1} successful`]: (r) => r.status === 202 || r.status === 200,
    });
    
    if (success) successCount++;
    performanceMonitor.recordRequest(response, 'file/concurrent_small');
  });

  check(null, {
    'all concurrent small files successful': () => successCount === concurrentCount,
  });
}

// Monitor file processing status
function monitorFileProcessingStatus(processingId, recordCount) {
  const maxChecks = 30; // Maximum status checks
  const checkInterval = 5; // Seconds between checks
  let checks = 0;
  let processingComplete = false;

  while (checks < maxChecks && !processingComplete) {
    sleep(checkInterval);
    checks++;

    const statusResponse = fileProcessor.checkProcessingStatus(processingId);
    
    const statusSuccess = check(statusResponse, {
      'processing status check successful': (r) => r.status === 200,
    });

    if (statusSuccess) {
      const statusBody = JSON.parse(statusResponse.body);
      const status = statusBody.data && statusBody.data.status;
      
      console.log(`Processing ${processingId} (${recordCount} records): ${status} (check ${checks})`);
      
      if (status === 'completed' || status === 'failed') {
        processingComplete = true;
        
        check(null, {
          'file processing completed successfully': () => status === 'completed',
          'file processing completed within time limit': () => checks <= 20, // 100 seconds
        });

        if (status === 'completed' && statusBody.data.records_processed) {
          const recordsProcessed = statusBody.data.records_processed;
          check(null, {
            'all records processed correctly': () => recordsProcessed >= recordCount * 0.9, // 90% threshold
          });
          
          console.log(`Processing completed: ${recordsProcessed}/${recordCount} records processed`);
        }
      }
    }

    performanceMonitor.recordRequest(statusResponse, 'file/status_monitor');
  }

  if (!processingComplete) {
    console.warn(`Processing ${processingId} did not complete within time limit (${checks} checks)`);
  }
}

// Teardown function
export function teardown(data) {
  if (performanceMonitor) {
    const summary = performanceMonitor.getSummary();
    console.log('File Processing Performance Summary:', JSON.stringify(summary, null, 2));
  }

  // Log test duration
  const duration = Date.now() - data.startTime;
  console.log(`File processing tests completed in ${duration}ms`);
}

// Export options for k6
export let options = {
  scenarios: {
    file_processing_load: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '1m', target: 5 },   // Ramp up slowly for file processing
        { duration: '5m', target: 5 },   // Maintain load
        { duration: '2m', target: 10 },  // Increase load
        { duration: '5m', target: 10 },  // Maintain higher load
        { duration: '2m', target: 0 },   // Ramp down
      ],
      gracefulRampDown: '1m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<30000', 'p(99)<60000'], // File uploads can be slower
    http_req_failed: ['rate<0.1'], // Higher tolerance for file processing
    'http_req_duration{endpoint:file_processing}': ['p(95)<25000'],
    'http_req_duration{operation:concurrent_upload}': ['p(95)<45000'],
    'http_req_duration{operation:large_upload_50000}': ['p(95)<120000'], // 2 minutes for very large files
  },
};