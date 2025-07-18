#!/bin/bash

# Post-deployment Health Check Script
# This script performs comprehensive health checks after deployment

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
TARGET_COLOR="${TARGET_COLOR:-}"
TIMEOUT="${TIMEOUT:-30}"
MAX_RETRIES="${MAX_RETRIES:-10}"
RETRY_DELAY="${RETRY_DELAY:-30}"

# Logging
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
}

error() {
    log "ERROR: $*"
    exit 1
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --target-color)
                TARGET_COLOR="$2"
                shift 2
                ;;
            --environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            --max-retries)
                MAX_RETRIES="$2"
                shift 2
                ;;
            --retry-delay)
                RETRY_DELAY="$2"
                shift 2
                ;;
            *)
                error "Unknown parameter: $1"
                ;;
        esac
    done
    
    if [[ -z "$TARGET_COLOR" ]]; then
        error "Target color is required (--target-color)"
    fi
}

# Get deployment configuration
get_deployment_config() {
    local env="$1"
    local service_name="hotel-reviews-service"
    
    # Get target group ARN based on color
    if [[ "$TARGET_COLOR" == "blue" ]]; then
        TARGET_GROUP_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/blue_target_group_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    elif [[ "$TARGET_COLOR" == "green" ]]; then
        TARGET_GROUP_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/green_target_group_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    else
        error "Invalid target color: $TARGET_COLOR (must be 'blue' or 'green')"
    fi
    
    LOAD_BALANCER_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/load_balancer_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    CLUSTER_NAME=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/cluster_name" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    SERVICE_NAME="${service_name}-${TARGET_COLOR}"
    
    if [[ -z "$TARGET_GROUP_ARN" || -z "$LOAD_BALANCER_ARN" || -z "$CLUSTER_NAME" ]]; then
        error "Failed to retrieve deployment configuration from SSM"
    fi
    
    log "Deployment configuration retrieved:"
    log "  Target Color: $TARGET_COLOR"
    log "  Target Group ARN: $TARGET_GROUP_ARN"
    log "  Load Balancer ARN: $LOAD_BALANCER_ARN"
    log "  Cluster Name: $CLUSTER_NAME"
    log "  Service Name: $SERVICE_NAME"
}

# Get load balancer DNS name
get_load_balancer_dns() {
    local lb_arn="$1"
    
    LOAD_BALANCER_DNS=$(aws elbv2 describe-load-balancers --load-balancer-arns "$lb_arn" --query 'LoadBalancers[0].DNSName' --output text 2>/dev/null || echo "")
    
    if [[ -z "$LOAD_BALANCER_DNS" ]]; then
        error "Failed to retrieve load balancer DNS name"
    fi
    
    log "Load balancer DNS: $LOAD_BALANCER_DNS"
}

# Check ECS service health
check_ecs_service_health() {
    local cluster_name="$1"
    local service_name="$2"
    
    log "Checking ECS service health: $service_name"
    
    # Get service details
    local service_info
    service_info=$(aws ecs describe-services --cluster "$cluster_name" --services "$service_name" --query 'services[0]' --output json 2>/dev/null || echo "{}")
    
    if [[ "$service_info" == "{}" ]]; then
        error "Service $service_name not found in cluster $cluster_name"
    fi
    
    # Check service status
    local service_status
    service_status=$(echo "$service_info" | jq -r '.status // "UNKNOWN"')
    
    if [[ "$service_status" != "ACTIVE" ]]; then
        error "Service status is not ACTIVE: $service_status"
    fi
    
    log "Service status: $service_status"
    
    # Check running vs desired count
    local running_count
    local desired_count
    running_count=$(echo "$service_info" | jq -r '.runningCount // 0')
    desired_count=$(echo "$service_info" | jq -r '.desiredCount // 0')
    
    log "Running tasks: $running_count, Desired tasks: $desired_count"
    
    if [[ "$running_count" -ne "$desired_count" ]]; then
        error "Running task count ($running_count) does not match desired count ($desired_count)"
    fi
    
    # Check task health
    local task_arns
    task_arns=$(aws ecs list-tasks --cluster "$cluster_name" --service-name "$service_name" --query 'taskArns' --output text 2>/dev/null || echo "")
    
    if [[ -z "$task_arns" ]]; then
        error "No tasks found for service $service_name"
    fi
    
    local healthy_tasks=0
    local total_tasks=0
    
    for task_arn in $task_arns; do
        total_tasks=$((total_tasks + 1))
        
        local task_info
        task_info=$(aws ecs describe-tasks --cluster "$cluster_name" --tasks "$task_arn" --query 'tasks[0]' --output json 2>/dev/null || echo "{}")
        
        local task_status
        task_status=$(echo "$task_info" | jq -r '.lastStatus // "UNKNOWN"')
        
        if [[ "$task_status" == "RUNNING" ]]; then
            healthy_tasks=$((healthy_tasks + 1))
            log "Task $task_arn is running"
        else
            log "WARNING: Task $task_arn status is $task_status"
        fi
    done
    
    log "Healthy tasks: $healthy_tasks/$total_tasks"
    
    if [[ "$healthy_tasks" -ne "$total_tasks" ]]; then
        error "Not all tasks are healthy: $healthy_tasks/$total_tasks"
    fi
}

# Check target group health
check_target_group_health() {
    local target_group_arn="$1"
    
    log "Checking target group health"
    
    # Get target health
    local target_health
    target_health=$(aws elbv2 describe-target-health --target-group-arn "$target_group_arn" --query 'TargetHealthDescriptions' --output json 2>/dev/null || echo "[]")
    
    if [[ "$target_health" == "[]" ]]; then
        error "No targets found in target group"
    fi
    
    local healthy_targets=0
    local total_targets=0
    
    echo "$target_health" | jq -r '.[] | "\(.Target.Id) \(.TargetHealth.State)"' | while read -r target_id target_state; do
        total_targets=$((total_targets + 1))
        
        if [[ "$target_state" == "healthy" ]]; then
            healthy_targets=$((healthy_targets + 1))
            log "Target $target_id is healthy"
        else
            log "WARNING: Target $target_id is $target_state"
        fi
    done
    
    # Get the counts from the subshell
    healthy_targets=$(echo "$target_health" | jq '[.[] | select(.TargetHealth.State == "healthy")] | length')
    total_targets=$(echo "$target_health" | jq 'length')
    
    log "Healthy targets: $healthy_targets/$total_targets"
    
    if [[ "$healthy_targets" -ne "$total_targets" ]]; then
        error "Not all targets are healthy: $healthy_targets/$total_targets"
    fi
}

# Check application health endpoint
check_application_health() {
    local dns_name="$1"
    
    log "Checking application health endpoint"
    
    local health_url="http://$dns_name/api/v1/health"
    
    # Test health endpoint
    local response_code
    local response_body
    response_code=$(curl -s -o /tmp/health_response.json -w "%{http_code}" --max-time "$TIMEOUT" "$health_url" || echo "000")
    response_body=$(cat /tmp/health_response.json 2>/dev/null || echo "")
    
    log "Health endpoint response code: $response_code"
    
    if [[ "$response_code" -ne 200 ]]; then
        error "Health endpoint returned HTTP $response_code"
    fi
    
    # Parse health response
    if [[ -n "$response_body" ]]; then
        local health_status
        health_status=$(echo "$response_body" | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")
        
        if [[ "$health_status" == "healthy" || "$health_status" == "ok" ]]; then
            log "Application health status: $health_status"
        else
            error "Application health status is not healthy: $health_status"
        fi
        
        # Log additional health information
        local checks
        checks=$(echo "$response_body" | jq -r '.checks // {}' 2>/dev/null || echo "{}")
        
        if [[ "$checks" != "{}" ]]; then
            log "Health check details:"
            echo "$checks" | jq -r 'to_entries[] | "  \(.key): \(.value.status // .value)"' | while read -r line; do
                log "$line"
            done
        fi
    else
        log "WARNING: Empty response body from health endpoint"
    fi
    
    # Cleanup
    rm -f /tmp/health_response.json
}

# Check application metrics endpoint
check_application_metrics() {
    local dns_name="$1"
    
    log "Checking application metrics endpoint"
    
    local metrics_url="http://$dns_name/metrics"
    
    # Test metrics endpoint
    local response_code
    response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time "$TIMEOUT" "$metrics_url" || echo "000")
    
    log "Metrics endpoint response code: $response_code"
    
    if [[ "$response_code" -eq 200 ]]; then
        log "Metrics endpoint is accessible"
    else
        log "WARNING: Metrics endpoint returned HTTP $response_code"
    fi
}

# Check application API endpoints
check_application_endpoints() {
    local dns_name="$1"
    
    log "Checking application API endpoints"
    
    local endpoints=(
        "/api/v1/health"
        "/api/v1/reviews"
        "/api/v1/hotels"
    )
    
    for endpoint in "${endpoints[@]}"; do
        local url="http://$dns_name$endpoint"
        
        log "Testing endpoint: $endpoint"
        
        local response_code
        response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time "$TIMEOUT" "$url" || echo "000")
        
        # Different expected response codes for different endpoints
        case $endpoint in
            "/api/v1/health")
                if [[ "$response_code" -eq 200 ]]; then
                    log "  ✓ Health endpoint is accessible"
                else
                    error "Health endpoint returned HTTP $response_code"
                fi
                ;;
            "/api/v1/reviews"|"/api/v1/hotels")
                if [[ "$response_code" -eq 200 || "$response_code" -eq 404 ]]; then
                    log "  ✓ API endpoint is accessible (HTTP $response_code)"
                else
                    log "  ⚠ API endpoint returned HTTP $response_code"
                fi
                ;;
            *)
                if [[ "$response_code" -ge 200 && "$response_code" -lt 500 ]]; then
                    log "  ✓ Endpoint is accessible (HTTP $response_code)"
                else
                    log "  ⚠ Endpoint returned HTTP $response_code"
                fi
                ;;
        esac
    done
}

# Check application performance
check_application_performance() {
    local dns_name="$1"
    
    log "Checking application performance"
    
    local health_url="http://$dns_name/api/v1/health"
    
    # Measure response time
    local response_time
    response_time=$(curl -s -o /dev/null -w "%{time_total}" --max-time "$TIMEOUT" "$health_url" || echo "0")
    
    log "Health endpoint response time: ${response_time}s"
    
    # Check if response time is acceptable (< 2 seconds)
    if (( $(echo "$response_time < 2.0" | bc -l) )); then
        log "Response time is acceptable"
    else
        log "WARNING: Response time is high: ${response_time}s"
    fi
    
    # Test concurrent requests
    log "Testing concurrent requests"
    
    local concurrent_requests=5
    local temp_dir="/tmp/perf_test_$$"
    mkdir -p "$temp_dir"
    
    for i in $(seq 1 $concurrent_requests); do
        (
            curl -s -o "$temp_dir/response_$i.txt" -w "%{http_code}" --max-time "$TIMEOUT" "$health_url" > "$temp_dir/code_$i.txt"
        ) &
    done
    
    wait
    
    # Check results
    local successful_requests=0
    for i in $(seq 1 $concurrent_requests); do
        local code
        code=$(cat "$temp_dir/code_$i.txt" 2>/dev/null || echo "000")
        
        if [[ "$code" -eq 200 ]]; then
            successful_requests=$((successful_requests + 1))
        fi
    done
    
    log "Concurrent requests: $successful_requests/$concurrent_requests successful"
    
    if [[ "$successful_requests" -eq "$concurrent_requests" ]]; then
        log "All concurrent requests successful"
    else
        log "WARNING: Some concurrent requests failed"
    fi
    
    # Cleanup
    rm -rf "$temp_dir"
}

# Run post-deployment health checks
run_post_deployment_checks() {
    local env="$1"
    local retry_count=0
    
    while [[ $retry_count -lt $MAX_RETRIES ]]; do
        log "Post-deployment health check attempt $((retry_count + 1))/$MAX_RETRIES"
        
        # Get deployment configuration
        get_deployment_config "$env"
        
        # Get load balancer DNS
        get_load_balancer_dns "$LOAD_BALANCER_ARN"
        
        # Wait for deployment to stabilize
        if [[ $retry_count -eq 0 ]]; then
            log "Waiting for deployment to stabilize..."
            sleep 60
        fi
        
        # Run health checks
        if check_ecs_service_health "$CLUSTER_NAME" "$SERVICE_NAME" && \
           check_target_group_health "$TARGET_GROUP_ARN" && \
           check_application_health "$LOAD_BALANCER_DNS"; then
            
            # Additional checks (non-blocking)
            check_application_metrics "$LOAD_BALANCER_DNS"
            check_application_endpoints "$LOAD_BALANCER_DNS"
            check_application_performance "$LOAD_BALANCER_DNS"
            
            log "All post-deployment health checks passed"
            return 0
        fi
        
        retry_count=$((retry_count + 1))
        if [[ $retry_count -lt $MAX_RETRIES ]]; then
            log "Health checks failed, retrying in ${RETRY_DELAY} seconds..."
            sleep "$RETRY_DELAY"
        fi
    done
    
    error "Post-deployment health checks failed after $MAX_RETRIES attempts"
}

# Check if running in CI/CD environment
check_ci_environment() {
    if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
        log "Running in GitHub Actions environment"
        # Set up AWS credentials if not already configured
        if [[ -z "${AWS_ACCESS_KEY_ID:-}" ]]; then
            log "AWS credentials not found in environment"
            return 1
        fi
    fi
    
    # Check if required tools are available
    local required_tools=("aws" "curl" "jq" "bc")
    
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &>/dev/null; then
            error "$tool not found. Please install $tool."
        fi
    done
}

# Main execution
main() {
    log "Starting post-deployment health check for environment: $ENVIRONMENT"
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Run post-deployment checks
    run_post_deployment_checks "$ENVIRONMENT"
    
    log "Post-deployment health check completed successfully"
}

# Execute main function
main "$@"