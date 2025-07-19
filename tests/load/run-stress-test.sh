#!/bin/bash

# Hotel Reviews Load Testing - Stress Test Scenario
# This script runs the stress testing scenario to test system limits

set -e

# Configuration
BASE_URL=${BASE_URL:-"http://localhost:8080"}
SCENARIO=${SCENARIO:-"stress_test"}
RESULTS_DIR="results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

echo -e "${RED}🔥 Hotel Reviews Load Testing - Stress Test Scenario${NC}"
echo "====================================================="
echo "Base URL: $BASE_URL"
echo "Scenario: $SCENARIO"
echo "Timestamp: $TIMESTAMP"
echo -e "${YELLOW}⚠️  WARNING: This is a stress test that will push the system to its limits${NC}"
echo ""

# Create results directory
mkdir -p "$RESULTS_DIR"

# Check if k6 is installed
if ! command -v k6 &> /dev/null; then
    echo -e "${RED}❌ k6 is not installed. Please install k6 first.${NC}"
    echo "Visit: https://k6.io/docs/getting-started/installation/"
    exit 1
fi

# Check if API is accessible
echo -e "${YELLOW}🔍 Checking API accessibility...${NC}"
if curl -s --connect-timeout 5 "$BASE_URL/health" > /dev/null; then
    echo -e "${GREEN}✅ API is accessible${NC}"
else
    echo -e "${RED}❌ API is not accessible at $BASE_URL${NC}"
    echo "Please ensure the Hotel Reviews API is running"
    exit 1
fi

# Pre-stress test warnings and confirmations
echo -e "${PURPLE}⚡ Stress Test Configuration${NC}"
echo "- Maximum Virtual Users: 200"
echo "- Test Duration: ~40 minutes"
echo "- Database Stress: High concurrent queries"
echo "- File Processing: Large file uploads"
echo "- Expected Error Rate: Up to 10%"
echo ""

# Ask for confirmation unless in CI mode
if [ "${CI}" != "true" ] && [ "${FORCE}" != "true" ]; then
    read -p "Are you sure you want to proceed with stress testing? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Stress test cancelled."
        exit 0
    fi
fi

# Function to run test with error handling and extended monitoring
run_stress_test() {
    local test_file=$1
    local test_name=$2
    local output_file="$RESULTS_DIR/${test_name}_${TIMESTAMP}.json"
    local log_file="$RESULTS_DIR/${test_name}_${TIMESTAMP}.log"
    
    echo -e "${YELLOW}🔥 Running stress test: $test_name...${NC}"
    echo "Expected duration: 40+ minutes"
    echo "Output: $output_file"
    echo "Logs: $log_file"
    echo ""
    
    # Set stress test environment variables
    export BASE_URL
    export SCENARIO
    export TEST_TYPE="mixed"
    export ERROR_TOLERANCE="high"
    export METRICS_REPORT_INTERVAL=15000  # More frequent reporting during stress
    export EXPORT_METRICS=true
    export METRICS_FORMAT=json
    export MIN_SUCCESS_RATE=90.0  # Lower threshold for stress test
    export MAX_AVG_RESPONSE_TIME=5000  # Higher threshold for stress test
    
    # Start monitoring in background if available
    if command -v top &> /dev/null; then
        echo "Starting system monitoring..."
        top -b -n1 | head -20 > "$RESULTS_DIR/system_before_${test_name}_${TIMESTAMP}.txt" 2>/dev/null || true
    fi
    
    # Record start time
    local start_time=$(date +%s)
    
    # Run the stress test
    if k6 run \
        --quiet \
        --out json="$output_file" \
        --env BASE_URL="$BASE_URL" \
        --env SCENARIO="$SCENARIO" \
        --env TEST_TYPE="mixed" \
        --env ERROR_TOLERANCE="high" \
        "$test_file" 2>&1 | tee "$log_file"; then
        
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        echo -e "${GREEN}✅ $test_name completed successfully${NC}"
        echo "Duration: ${duration} seconds"
        echo "Results saved to: $output_file"
        
        # Post-test system monitoring
        if command -v top &> /dev/null; then
            top -b -n1 | head -20 > "$RESULTS_DIR/system_after_${test_name}_${TIMESTAMP}.txt" 2>/dev/null || true
        fi
        
        return 0
    else
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        echo -e "${RED}❌ $test_name failed or was terminated${NC}"
        echo "Duration before failure: ${duration} seconds"
        echo "Check logs: $log_file"
        return 1
    fi
}

# Function to monitor system resources during stress test
monitor_system() {
    local test_name=$1
    local monitor_file="$RESULTS_DIR/system_monitor_${test_name}_${TIMESTAMP}.log"
    
    echo "Starting system monitoring for $test_name..."
    
    # Monitor system resources every 30 seconds
    while true; do
        {
            echo "=== $(date) ==="
            if command -v free &> /dev/null; then
                echo "Memory Usage:"
                free -h
            fi
            if command -v iostat &> /dev/null; then
                echo "I/O Stats:"
                iostat -x 1 1 | tail -n +4
            fi
            if command -v ps &> /dev/null; then
                echo "Top Processes:"
                ps aux | head -10
            fi
            echo ""
        } >> "$monitor_file" 2>/dev/null
        
        sleep 30
    done &
    
    echo $! > "$RESULTS_DIR/monitor_pid_${test_name}.tmp"
}

# Function to stop system monitoring
stop_monitoring() {
    local test_name=$1
    local pid_file="$RESULTS_DIR/monitor_pid_${test_name}.tmp"
    
    if [ -f "$pid_file" ]; then
        local monitor_pid=$(cat "$pid_file")
        if ps -p "$monitor_pid" > /dev/null 2>&1; then
            kill "$monitor_pid" 2>/dev/null || true
        fi
        rm -f "$pid_file"
    fi
}

# Main stress test execution
echo -e "${BLUE}🎯 Starting Stress Test Suite${NC}"
echo "This will test the system beyond normal capacity"
echo ""

# Test results tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 1. API Endpoints Stress Test
TOTAL_TESTS=$((TOTAL_TESTS + 1))
echo -e "${PURPLE}=== API Endpoints Stress Test ===${NC}"
monitor_system "api_stress"
if run_stress_test "api-endpoints.js" "api_endpoints_stress"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi
stop_monitoring "api_stress"
echo ""

# 2. Database Stress Test (high concurrent load)
TOTAL_TESTS=$((TOTAL_TESTS + 1))
echo -e "${PURPLE}=== Database Stress Test ===${NC}"
export DB_TEST_TYPE="stress"
export MAX_CONCURRENT_QUERIES=50
monitor_system "db_stress"
if run_stress_test "database-performance.js" "database_stress"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi
stop_monitoring "db_stress"
echo ""

# 3. File Processing Stress Test (maximum concurrent uploads)
TOTAL_TESTS=$((TOTAL_TESTS + 1))
echo -e "${PURPLE}=== File Processing Stress Test ===${NC}"
export FILE_TEST_TYPE="stress"
export MAX_CONCURRENT_UPLOADS=20
export PROCESSING_TIMEOUT=1800  # 30 minutes
monitor_system "file_stress"
if run_stress_test "file-processing.js" "file_processing_stress"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi
stop_monitoring "file_stress"
echo ""

# Generate comprehensive stress test report
echo -e "${BLUE}📈 Stress Test Summary${NC}"
echo "====================="
echo "Total Tests: $TOTAL_TESTS"
echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"
echo ""

# Calculate success rate
if [ $TOTAL_TESTS -gt 0 ]; then
    SUCCESS_RATE=$(echo "scale=2; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc 2>/dev/null || echo "N/A")
    echo "Success Rate: ${SUCCESS_RATE}%"
else
    SUCCESS_RATE=0
fi

# Create comprehensive summary report
SUMMARY_FILE="$RESULTS_DIR/stress_test_summary_${TIMESTAMP}.txt"
cat > "$SUMMARY_FILE" << EOF
Hotel Reviews Load Testing - Stress Test Summary
================================================
Timestamp: $TIMESTAMP
Base URL: $BASE_URL
Scenario: $SCENARIO

Stress Test Configuration:
- Maximum Virtual Users: 200
- Database Concurrent Queries: 50
- File Concurrent Uploads: 20
- Error Tolerance: High (up to 10% failures expected)
- Test Duration: ~40 minutes per test

Test Results:
- Total Tests: $TOTAL_TESTS
- Passed: $PASSED_TESTS
- Failed: $FAILED_TESTS
- Success Rate: ${SUCCESS_RATE}%

Test Files:
- API Endpoints Stress: api_endpoints_stress_${TIMESTAMP}.json
- Database Stress: database_stress_${TIMESTAMP}.json
- File Processing Stress: file_processing_stress_${TIMESTAMP}.json

System Monitoring:
- System monitoring logs available in results directory
- Check *_monitor_*.log files for resource utilization

Environment Variables:
- BASE_URL=$BASE_URL
- SCENARIO=$SCENARIO
- ERROR_TOLERANCE=high
- MAX_CONCURRENT_QUERIES=50
- MAX_CONCURRENT_UPLOADS=20
- MIN_SUCCESS_RATE=90.0
- MAX_AVG_RESPONSE_TIME=5000ms
EOF

echo "Comprehensive summary saved to: $SUMMARY_FILE"
echo ""

# Advanced performance analysis for stress test
if command -v jq &> /dev/null; then
    echo -e "${YELLOW}🔍 Stress Test Performance Analysis${NC}"
    echo "===================================="
    
    for test_type in "api_endpoints_stress" "database_stress" "file_processing_stress"; do
        local result_file="$RESULTS_DIR/${test_type}_${TIMESTAMP}.json"
        if [ -f "$result_file" ]; then
            echo ""
            echo "=== $test_type ==="
            
            # Extract key stress test metrics
            echo "Response Time Analysis:"
            jq -r '.metrics | to_entries[] | select(.key | contains("http_req_duration")) | "\(.key): avg=\(.value.avg)ms, p95=\(.value.p95)ms, p99=\(.value.p99)ms, max=\(.value.max)ms"' "$result_file" 2>/dev/null | head -5
            
            echo ""
            echo "Error Analysis:"
            jq -r '.metrics | to_entries[] | select(.key == "http_req_failed") | "Failed Requests: \(.value.rate * 100)% (\(.value.fails) failures out of \(.value.passes + .value.fails) total)"' "$result_file" 2>/dev/null
            
            echo ""
            echo "Throughput:"
            jq -r '.metrics | to_entries[] | select(.key == "http_reqs") | "Total Requests: \(.value.count), Rate: \(.value.rate) req/s"' "$result_file" 2>/dev/null
        fi
    done
else
    echo "Install 'jq' for detailed stress test analysis"
fi

echo ""

# Stress test specific recommendations
echo -e "${BLUE}🎯 Stress Test Analysis & Recommendations${NC}"
echo "========================================="

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}🎉 Excellent! System passed all stress tests.${NC}"
    echo ""
    echo "Key findings:"
    echo "- System can handle extreme load conditions"
    echo "- All components (API, database, file processing) are resilient"
    echo "- Error rates remained within acceptable limits"
    echo ""
    echo "Recommendations:"
    echo "- Current system capacity is sufficient for expected peak loads"
    echo "- Consider implementing auto-scaling for even higher loads"
    echo "- Monitor these metrics in production for early warning"
elif [ $FAILED_TESTS -eq 1 ]; then
    echo -e "${YELLOW}⚠️  Good performance with one area needing attention.${NC}"
    echo ""
    echo "Recommendations:"
    echo "- Investigate the failed test component"
    echo "- Consider optimizing the specific failing area"
    echo "- System is mostly ready for high load scenarios"
    echo "- Implement monitoring for the weak component"
elif [ $FAILED_TESTS -eq 2 ]; then
    echo -e "${YELLOW}⚠️  System shows stress under extreme load.${NC}"
    echo ""
    echo "Recommendations:"
    echo "- Significant performance tuning needed"
    echo "- Review database connection pooling and queries"
    echo "- Consider horizontal scaling"
    echo "- Implement circuit breakers and rate limiting"
    echo "- Review system resource allocation"
else
    echo -e "${RED}❌ System failed under stress conditions.${NC}"
    echo ""
    echo "Critical actions needed:"
    echo "- Immediate performance investigation required"
    echo "- Review system architecture for bottlenecks"
    echo "- Consider significant infrastructure upgrades"
    echo "- Implement comprehensive monitoring and alerting"
    echo "- Review database and application optimization"
fi

echo ""
echo -e "${BLUE}📊 Next Steps${NC}"
echo "============"
echo "1. Review detailed test results in the results directory"
echo "2. Analyze system monitoring logs for resource bottlenecks"
echo "3. Compare with baseline performance tests"
echo "4. Plan performance improvements based on findings"
echo "5. Re-run stress tests after optimizations"

# Final exit status
if [ $FAILED_TESTS -eq 0 ]; then
    exit 0
elif [ $FAILED_TESTS -lt $TOTAL_TESTS ]; then
    exit 1
else
    exit 2
fi