#!/bin/bash

# Hotel Reviews Load Testing - Full Test Suite
# This script runs the complete load testing suite with all scenarios

set -e

# Configuration
BASE_URL=${BASE_URL:-"http://localhost:8080"}
RESULTS_DIR="results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
FULL_SUITE_LOG="$RESULTS_DIR/full_suite_${TIMESTAMP}.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Create results directory
mkdir -p "$RESULTS_DIR"

# Redirect all output to both console and log file
exec > >(tee -a "$FULL_SUITE_LOG")
exec 2>&1

echo -e "${CYAN}🚀 Hotel Reviews Load Testing - Full Test Suite${NC}"
echo "=================================================="
echo "Base URL: $BASE_URL"
echo "Timestamp: $TIMESTAMP"
echo "Full Suite Log: $FULL_SUITE_LOG"
echo -e "${YELLOW}⏱️  Estimated Total Duration: 3-4 hours${NC}"
echo ""

# Check prerequisites
check_prerequisites() {
    echo -e "${BLUE}🔍 Checking Prerequisites${NC}"
    echo "========================="
    
    # Check k6 installation
    if ! command -v k6 &> /dev/null; then
        echo -e "${RED}❌ k6 is not installed${NC}"
        echo "Please install k6: https://k6.io/docs/getting-started/installation/"
        return 1
    else
        echo -e "${GREEN}✅ k6 is installed${NC}"
    fi
    
    # Check API accessibility
    if curl -s --connect-timeout 10 "$BASE_URL/health" > /dev/null; then
        echo -e "${GREEN}✅ API is accessible at $BASE_URL${NC}"
    else
        echo -e "${RED}❌ API is not accessible at $BASE_URL${NC}"
        echo "Please ensure the Hotel Reviews API is running"
        return 1
    fi
    
    # Check required tools
    local tools=("curl" "bc" "date")
    for tool in "${tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            echo -e "${GREEN}✅ $tool is available${NC}"
        else
            echo -e "${YELLOW}⚠️  $tool is not available (some features may be limited)${NC}"
        fi
    done
    
    # Check optional tools
    local optional_tools=("jq" "top" "iostat")
    echo ""
    echo "Optional tools for enhanced reporting:"
    for tool in "${optional_tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            echo -e "${GREEN}✅ $tool is available${NC}"
        else
            echo -e "${CYAN}ℹ️  $tool is not available (install for enhanced features)${NC}"
        fi
    done
    
    echo ""
    return 0
}

# Function to run individual test scenario
run_scenario() {
    local scenario_name=$1
    local scenario_file=$2
    local estimated_duration=$3
    local description=$4
    
    echo ""
    echo -e "${PURPLE}===================================================${NC}"
    echo -e "${PURPLE}🎯 Running Scenario: $scenario_name${NC}"
    echo -e "${PURPLE}===================================================${NC}"
    echo "Description: $description"
    echo "Estimated Duration: $estimated_duration"
    echo "Start Time: $(date)"
    echo ""
    
    local start_time=$(date +%s)
    local output_file="$RESULTS_DIR/${scenario_name}_${TIMESTAMP}.json"
    local log_file="$RESULTS_DIR/${scenario_name}_${TIMESTAMP}.log"
    
    # Set scenario-specific environment variables
    export BASE_URL
    export SCENARIO="$scenario_name"
    export EXPORT_METRICS=true
    export METRICS_FORMAT=json
    export METRICS_REPORT_INTERVAL=30000
    
    # Run the scenario
    if k6 run \
        --quiet \
        --out json="$output_file" \
        --env BASE_URL="$BASE_URL" \
        --env SCENARIO="$scenario_name" \
        "$scenario_file" 2>&1 | tee "$log_file"; then
        
        local end_time=$(date +%s)
        local actual_duration=$((end_time - start_time))
        local duration_minutes=$((actual_duration / 60))
        
        echo ""
        echo -e "${GREEN}✅ $scenario_name completed successfully${NC}"
        echo "Actual Duration: ${duration_minutes} minutes (${actual_duration} seconds)"
        echo "Results: $output_file"
        echo "Logs: $log_file"
        
        return 0
    else
        local end_time=$(date +%s)
        local actual_duration=$((end_time - start_time))
        local duration_minutes=$((actual_duration / 60))
        
        echo ""
        echo -e "${RED}❌ $scenario_name failed${NC}"
        echo "Duration before failure: ${duration_minutes} minutes (${actual_duration} seconds)"
        echo "Logs: $log_file"
        
        return 1
    fi
}

# Function to analyze scenario results
analyze_scenario_results() {
    local scenario_name=$1
    local result_file="$RESULTS_DIR/${scenario_name}_${TIMESTAMP}.json"
    
    if [ ! -f "$result_file" ]; then
        echo "No results file found for $scenario_name"
        return
    fi
    
    if command -v jq &> /dev/null; then
        echo ""
        echo "📊 $scenario_name Performance Summary:"
        echo "-------------------------------------"
        
        # Extract key metrics
        local avg_duration=$(jq -r '.metrics.http_req_duration.avg // "N/A"' "$result_file" 2>/dev/null)
        local p95_duration=$(jq -r '.metrics.http_req_duration.p95 // "N/A"' "$result_file" 2>/dev/null)
        local failure_rate=$(jq -r '.metrics.http_req_failed.rate // "N/A"' "$result_file" 2>/dev/null)
        local total_requests=$(jq -r '.metrics.http_reqs.count // "N/A"' "$result_file" 2>/dev/null)
        local rps=$(jq -r '.metrics.http_reqs.rate // "N/A"' "$result_file" 2>/dev/null)
        
        echo "Average Response Time: ${avg_duration}ms"
        echo "95th Percentile: ${p95_duration}ms"
        echo "Failure Rate: $(echo "$failure_rate * 100" | bc 2>/dev/null || echo "N/A")%"
        echo "Total Requests: $total_requests"
        echo "Requests/Second: $rps"
    else
        echo "Install jq for detailed performance analysis"
    fi
}

# Check prerequisites first
if ! check_prerequisites; then
    echo -e "${RED}❌ Prerequisites check failed. Please resolve the issues above.${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}🎬 Starting Full Test Suite Execution${NC}"
echo "======================================"

# Test execution tracking
TOTAL_SCENARIOS=0
PASSED_SCENARIOS=0
FAILED_SCENARIOS=0
declare -a SCENARIO_RESULTS

# Scenario definitions with realistic duration estimates
declare -A SCENARIOS=(
    ["normal_load"]="api-endpoints.js|15 minutes|Baseline performance testing with normal user load"
    ["peak_load"]="api-endpoints.js|40 minutes|High traffic simulation with sustained load and spikes"
    ["database_intensive"]="database-performance.js|35 minutes|Database performance testing with concurrent queries"
    ["file_processing_load"]="file-processing.js|45 minutes|Concurrent file upload and processing testing"
    ["stress_test"]="api-endpoints.js|40 minutes|Beyond normal capacity testing to find system limits"
    ["spike_test"]="api-endpoints.js|10 minutes|Sudden traffic increase simulation"
)

# Run each scenario
for scenario in "normal_load" "peak_load" "database_intensive" "file_processing_load" "spike_test" "stress_test"; do
    TOTAL_SCENARIOS=$((TOTAL_SCENARIOS + 1))
    
    IFS='|' read -r scenario_file estimated_duration description <<< "${SCENARIOS[$scenario]}"
    
    if run_scenario "$scenario" "$scenario_file" "$estimated_duration" "$description"; then
        PASSED_SCENARIOS=$((PASSED_SCENARIOS + 1))
        SCENARIO_RESULTS+=("$scenario:PASSED")
        analyze_scenario_results "$scenario"
    else
        FAILED_SCENARIOS=$((FAILED_SCENARIOS + 1))
        SCENARIO_RESULTS+=("$scenario:FAILED")
        
        # Ask if user wants to continue after failure
        if [ "${CI}" != "true" ] && [ "${FORCE_CONTINUE}" != "true" ]; then
            echo ""
            echo -e "${YELLOW}⚠️  Scenario $scenario failed. Continue with remaining tests?${NC}"
            read -p "Continue? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                echo "Test suite execution stopped by user."
                break
            fi
        fi
    fi
    
    # Small break between scenarios
    echo ""
    echo -e "${CYAN}⏸️  Brief pause before next scenario (30 seconds)...${NC}"
    sleep 30
done

echo ""
echo -e "${BLUE}📈 Full Test Suite Summary${NC}"
echo "=========================="
echo "Execution Time: $(date)"
echo "Total Scenarios: $TOTAL_SCENARIOS"
echo -e "Passed: ${GREEN}$PASSED_SCENARIOS${NC}"
echo -e "Failed: ${RED}$FAILED_SCENARIOS${NC}"

# Calculate success rate
if [ $TOTAL_SCENARIOS -gt 0 ]; then
    SUCCESS_RATE=$(echo "scale=2; $PASSED_SCENARIOS * 100 / $TOTAL_SCENARIOS" | bc 2>/dev/null || echo "N/A")
    echo "Success Rate: ${SUCCESS_RATE}%"
fi

echo ""
echo "Scenario Results:"
for result in "${SCENARIO_RESULTS[@]}"; do
    IFS=':' read -r scenario status <<< "$result"
    if [ "$status" = "PASSED" ]; then
        echo -e "  ${GREEN}✅${NC} $scenario"
    else
        echo -e "  ${RED}❌${NC} $scenario"
    fi
done

# Generate comprehensive final report
FINAL_REPORT="$RESULTS_DIR/full_suite_report_${TIMESTAMP}.html"
cat > "$FINAL_REPORT" << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>Hotel Reviews Load Testing - Full Suite Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #f0f8ff; padding: 20px; border-radius: 5px; }
        .summary { background-color: #f9f9f9; padding: 15px; margin: 20px 0; border-radius: 5px; }
        .scenario { margin: 15px 0; padding: 10px; border-left: 4px solid #ddd; }
        .passed { border-left-color: #28a745; }
        .failed { border-left-color: #dc3545; }
        .metrics { background-color: #fff; padding: 10px; margin: 10px 0; border-radius: 3px; }
        table { width: 100%; border-collapse: collapse; margin: 10px 0; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f9fa; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Hotel Reviews Load Testing - Full Suite Report</h1>
        <p><strong>Execution Date:</strong> TIMESTAMP_PLACEHOLDER</p>
        <p><strong>Base URL:</strong> BASE_URL_PLACEHOLDER</p>
        <p><strong>Total Duration:</strong> DURATION_PLACEHOLDER</p>
    </div>
    
    <div class="summary">
        <h2>Executive Summary</h2>
        <table>
            <tr><th>Metric</th><th>Value</th></tr>
            <tr><td>Total Scenarios</td><td>TOTAL_SCENARIOS_PLACEHOLDER</td></tr>
            <tr><td>Passed Scenarios</td><td>PASSED_SCENARIOS_PLACEHOLDER</td></tr>
            <tr><td>Failed Scenarios</td><td>FAILED_SCENARIOS_PLACEHOLDER</td></tr>
            <tr><td>Success Rate</td><td>SUCCESS_RATE_PLACEHOLDER%</td></tr>
        </table>
    </div>
    
    <div class="scenarios">
        <h2>Scenario Results</h2>
        SCENARIOS_PLACEHOLDER
    </div>
    
    <div class="recommendations">
        <h2>Recommendations</h2>
        RECOMMENDATIONS_PLACEHOLDER
    </div>
</body>
</html>
EOF

# Calculate total suite duration
SUITE_END_TIME=$(date +%s)
SUITE_START_TIME=$(stat -c %Y "$FULL_SUITE_LOG" 2>/dev/null || date +%s)
TOTAL_DURATION=$((SUITE_END_TIME - SUITE_START_TIME))
TOTAL_DURATION_HOURS=$((TOTAL_DURATION / 3600))
TOTAL_DURATION_MINUTES=$(((TOTAL_DURATION % 3600) / 60))

# Update HTML report with actual values
sed -i "s/TIMESTAMP_PLACEHOLDER/$(date)/g" "$FINAL_REPORT" 2>/dev/null || true
sed -i "s/BASE_URL_PLACEHOLDER/$BASE_URL/g" "$FINAL_REPORT" 2>/dev/null || true
sed -i "s/DURATION_PLACEHOLDER/${TOTAL_DURATION_HOURS}h ${TOTAL_DURATION_MINUTES}m/g" "$FINAL_REPORT" 2>/dev/null || true
sed -i "s/TOTAL_SCENARIOS_PLACEHOLDER/$TOTAL_SCENARIOS/g" "$FINAL_REPORT" 2>/dev/null || true
sed -i "s/PASSED_SCENARIOS_PLACEHOLDER/$PASSED_SCENARIOS/g" "$FINAL_REPORT" 2>/dev/null || true
sed -i "s/FAILED_SCENARIOS_PLACEHOLDER/$FAILED_SCENARIOS/g" "$FINAL_REPORT" 2>/dev/null || true
sed -i "s/SUCCESS_RATE_PLACEHOLDER/$SUCCESS_RATE/g" "$FINAL_REPORT" 2>/dev/null || true

echo ""
echo -e "${BLUE}📊 Reports Generated${NC}"
echo "==================="
echo "Full Suite Log: $FULL_SUITE_LOG"
echo "HTML Report: $FINAL_REPORT"
echo "Individual Results: $RESULTS_DIR/*_${TIMESTAMP}.json"
echo ""

# Overall assessment and recommendations
echo -e "${BLUE}🎯 Overall Assessment${NC}"
echo "===================="

if [ $FAILED_SCENARIOS -eq 0 ]; then
    echo -e "${GREEN}🎉 Excellent! All scenarios passed successfully.${NC}"
    echo ""
    echo "✅ System Performance Summary:"
    echo "  - All load scenarios handled successfully"
    echo "  - System is ready for production traffic"
    echo "  - Performance meets or exceeds expectations"
    echo "  - No critical issues identified"
    echo ""
    echo "🚀 Recommendations:"
    echo "  - Deploy with confidence"
    echo "  - Set up production monitoring based on test thresholds"
    echo "  - Schedule regular load testing to catch regressions"
    echo "  - Document current performance baselines"
    
elif [ $FAILED_SCENARIOS -eq 1 ]; then
    echo -e "${YELLOW}⚠️  Good performance with one area needing attention.${NC}"
    echo ""
    echo "📊 Assessment:"
    echo "  - Most scenarios performed well"
    echo "  - One component needs optimization"
    echo "  - System is mostly production-ready"
    echo ""
    echo "🔧 Recommendations:"
    echo "  - Investigate and fix the failing scenario"
    echo "  - Consider deploying with monitoring on the weak area"
    echo "  - Plan performance improvements for the failing component"
    
elif [ $FAILED_SCENARIOS -le $((TOTAL_SCENARIOS / 2)) ]; then
    echo -e "${YELLOW}⚠️  Mixed results - significant optimization needed.${NC}"
    echo ""
    echo "📊 Assessment:"
    echo "  - System shows performance issues under load"
    echo "  - Multiple components need attention"
    echo "  - Production deployment should be delayed"
    echo ""
    echo "🔧 Critical Actions:"
    echo "  - Comprehensive performance review required"
    echo "  - Optimize failing components before deployment"
    echo "  - Consider infrastructure scaling"
    echo "  - Implement additional monitoring and alerting"
    
else
    echo -e "${RED}❌ Critical performance issues identified.${NC}"
    echo ""
    echo "🚨 Critical Assessment:"
    echo "  - System fails under most load conditions"
    echo "  - Major architectural review needed"
    echo "  - Not suitable for production deployment"
    echo ""
    echo "🚨 Immediate Actions Required:"
    echo "  - Stop deployment plans"
    echo "  - Conduct thorough performance audit"
    echo "  - Review system architecture and scaling strategy"
    echo "  - Consider infrastructure upgrades"
    echo "  - Implement comprehensive performance monitoring"
fi

echo ""
echo -e "${CYAN}📋 Next Steps${NC}"
echo "============="
echo "1. Review detailed results in: $RESULTS_DIR/"
echo "2. Analyze performance metrics for each scenario"
echo "3. Identify and address performance bottlenecks"
echo "4. Update system configuration based on findings"
echo "5. Re-run specific scenarios after optimizations"
echo "6. Set up production monitoring with test-derived thresholds"
echo ""

echo -e "${BLUE}🏁 Full Test Suite Completed${NC}"
echo "Total Execution Time: ${TOTAL_DURATION_HOURS}h ${TOTAL_DURATION_MINUTES}m"
echo "Thank you for running the comprehensive load testing suite!"

# Final exit status
if [ $FAILED_SCENARIOS -eq 0 ]; then
    exit 0
elif [ $FAILED_SCENARIOS -le $((TOTAL_SCENARIOS / 2)) ]; then
    exit 1
else
    exit 2
fi