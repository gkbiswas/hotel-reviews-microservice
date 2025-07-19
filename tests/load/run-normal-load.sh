#!/bin/bash

# Hotel Reviews Load Testing - Normal Load Scenario
# This script runs the normal load testing scenario

set -e

# Configuration
BASE_URL=${BASE_URL:-"http://localhost:8080"}
SCENARIO=${SCENARIO:-"normal_load"}
RESULTS_DIR="results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Hotel Reviews Load Testing - Normal Load Scenario${NC}"
echo "=================================================="
echo "Base URL: $BASE_URL"
echo "Scenario: $SCENARIO"
echo "Timestamp: $TIMESTAMP"
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

# Function to run test with error handling
run_test() {
    local test_file=$1
    local test_name=$2
    local output_file="$RESULTS_DIR/${test_name}_${TIMESTAMP}.json"
    
    echo -e "${YELLOW}📊 Running $test_name...${NC}"
    
    # Set environment variables for this test
    export BASE_URL
    export SCENARIO
    export TEST_TYPE="mixed"
    export METRICS_REPORT_INTERVAL=30000
    export EXPORT_METRICS=true
    export METRICS_FORMAT=json
    
    # Run the test
    if k6 run \
        --quiet \
        --out json="$output_file" \
        --env BASE_URL="$BASE_URL" \
        --env SCENARIO="$SCENARIO" \
        --env TEST_TYPE="mixed" \
        "$test_file"; then
        echo -e "${GREEN}✅ $test_name completed successfully${NC}"
        echo "Results saved to: $output_file"
        return 0
    else
        echo -e "${RED}❌ $test_name failed${NC}"
        return 1
    fi
}

# Main test execution
echo -e "${BLUE}🎯 Starting Normal Load Test Suite${NC}"
echo ""

# Test results tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 1. API Endpoints Test
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "api-endpoints.js" "api_endpoints_normal_load"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# 2. Database Performance Test (lighter load for normal scenario)
TOTAL_TESTS=$((TOTAL_TESTS + 1))
export DB_TEST_TYPE="mixed"
export MAX_CONCURRENT_QUERIES=10
if run_test "database-performance.js" "database_normal_load"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# 3. File Processing Test (limited for normal load)
TOTAL_TESTS=$((TOTAL_TESTS + 1))
export FILE_TEST_TYPE="mixed"
export MAX_CONCURRENT_UPLOADS=5
if run_test "file-processing.js" "file_processing_normal_load"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Generate summary report
echo -e "${BLUE}📈 Test Summary${NC}"
echo "=============="
echo "Total Tests: $TOTAL_TESTS"
echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"
echo ""

# Calculate success rate
if [ $TOTAL_TESTS -gt 0 ]; then
    SUCCESS_RATE=$(echo "scale=2; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc)
    echo "Success Rate: ${SUCCESS_RATE}%"
else
    SUCCESS_RATE=0
fi

# Create summary report file
SUMMARY_FILE="$RESULTS_DIR/normal_load_summary_${TIMESTAMP}.txt"
cat > "$SUMMARY_FILE" << EOF
Hotel Reviews Load Testing - Normal Load Summary
================================================
Timestamp: $TIMESTAMP
Base URL: $BASE_URL
Scenario: $SCENARIO

Test Results:
- Total Tests: $TOTAL_TESTS
- Passed: $PASSED_TESTS
- Failed: $FAILED_TESTS
- Success Rate: ${SUCCESS_RATE}%

Test Files:
- API Endpoints: api_endpoints_normal_load_${TIMESTAMP}.json
- Database Performance: database_normal_load_${TIMESTAMP}.json
- File Processing: file_processing_normal_load_${TIMESTAMP}.json

Environment Variables:
- BASE_URL=$BASE_URL
- SCENARIO=$SCENARIO
- TEST_TYPE=mixed
- DB_TEST_TYPE=mixed
- FILE_TEST_TYPE=mixed
- MAX_CONCURRENT_QUERIES=10
- MAX_CONCURRENT_UPLOADS=5
EOF

echo "Summary saved to: $SUMMARY_FILE"
echo ""

# Performance analysis
if [ -f "$RESULTS_DIR/api_endpoints_normal_load_${TIMESTAMP}.json" ]; then
    echo -e "${YELLOW}🔍 Quick Performance Analysis${NC}"
    echo "=============================="
    
    # Extract key metrics from the JSON output
    if command -v jq &> /dev/null; then
        echo "API Performance Metrics:"
        jq -r '.metrics | to_entries[] | select(.key | contains("http_req_duration")) | "\(.key): \(.value.avg)ms (avg), \(.value.p95)ms (p95)"' "$RESULTS_DIR/api_endpoints_normal_load_${TIMESTAMP}.json" 2>/dev/null || echo "Could not parse performance metrics"
        
        echo ""
        echo "Error Rate:"
        jq -r '.metrics | to_entries[] | select(.key == "http_req_failed") | "Failed Requests: \(.value.rate * 100)%"' "$RESULTS_DIR/api_endpoints_normal_load_${TIMESTAMP}.json" 2>/dev/null || echo "Could not parse error rate"
    else
        echo "Install 'jq' for detailed performance analysis"
    fi
fi

echo ""

# Final status and recommendations
if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! Normal load performance is acceptable.${NC}"
    exit 0
elif [ $FAILED_TESTS -lt $TOTAL_TESTS ]; then
    echo -e "${YELLOW}⚠️  Some tests failed. Review the results and consider investigating performance issues.${NC}"
    echo ""
    echo "Recommendations:"
    echo "- Check failed test logs for specific errors"
    echo "- Monitor system resources during testing"
    echo "- Review database and cache performance"
    echo "- Consider tuning application configuration"
    exit 1
else
    echo -e "${RED}❌ All tests failed. There may be a serious issue with the system.${NC}"
    echo ""
    echo "Immediate actions:"
    echo "- Verify API is running and accessible"
    echo "- Check system resources (CPU, memory, disk)"
    echo "- Review application logs for errors"
    echo "- Verify database connectivity"
    exit 2
fi