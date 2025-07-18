#!/bin/bash

# External Dependencies Health Check Script
# This script checks the health of external dependencies (S3, Redis, etc.)

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
TIMEOUT="${TIMEOUT:-30}"
MAX_RETRIES="${MAX_RETRIES:-3}"
RETRY_DELAY="${RETRY_DELAY:-5}"

# Logging
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
}

error() {
    log "ERROR: $*"
    exit 1
}

# Check S3 connectivity and permissions
check_s3_dependency() {
    local env="$1"
    local bucket_name="hotel-reviews-${env}-app-data"
    
    log "Checking S3 dependency for bucket: $bucket_name"
    
    # Check if bucket exists and is accessible
    if aws s3api head-bucket --bucket "$bucket_name" 2>/dev/null; then
        log "S3 bucket '$bucket_name' is accessible"
        
        # Test write permissions
        local test_key="health-check/test-$(date +%s).txt"
        local test_content="Health check test file"
        
        if echo "$test_content" | aws s3 cp - "s3://$bucket_name/$test_key" 2>/dev/null; then
            log "S3 write permissions verified"
            
            # Test read permissions
            if aws s3 cp "s3://$bucket_name/$test_key" - 2>/dev/null | grep -q "$test_content"; then
                log "S3 read permissions verified"
                
                # Cleanup test file
                aws s3 rm "s3://$bucket_name/$test_key" 2>/dev/null || true
                
                return 0
            else
                log "S3 read permissions failed"
                return 1
            fi
        else
            log "S3 write permissions failed"
            return 1
        fi
    else
        log "S3 bucket '$bucket_name' is not accessible"
        return 1
    fi
}

# Check Redis/ElastiCache connectivity (if applicable)
check_redis_dependency() {
    local env="$1"
    local service_name="hotel-reviews-service"
    
    log "Checking Redis dependency"
    
    # Get Redis endpoint from SSM (if configured)
    local redis_endpoint
    redis_endpoint=$(aws ssm get-parameter --name "/${env}/${service_name}/redis/endpoint" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    
    if [[ -z "$redis_endpoint" ]]; then
        log "Redis endpoint not configured, skipping Redis health check"
        return 0
    fi
    
    # Parse host and port
    local redis_host
    local redis_port
    if [[ "$redis_endpoint" =~ ^([^:]+):([0-9]+)$ ]]; then
        redis_host="${BASH_REMATCH[1]}"
        redis_port="${BASH_REMATCH[2]}"
    else
        redis_host="$redis_endpoint"
        redis_port="6379"
    fi
    
    # Test Redis connectivity
    if timeout "$TIMEOUT" bash -c "echo >/dev/tcp/$redis_host/$redis_port" 2>/dev/null; then
        log "Redis connectivity test passed"
        
        # Test Redis ping (if redis-cli is available)
        if command -v redis-cli &>/dev/null; then
            if timeout "$TIMEOUT" redis-cli -h "$redis_host" -p "$redis_port" ping 2>/dev/null | grep -q "PONG"; then
                log "Redis ping test passed"
                return 0
            else
                log "Redis ping test failed"
                return 1
            fi
        else
            log "redis-cli not available, skipping ping test"
            return 0
        fi
    else
        log "Redis connectivity test failed"
        return 1
    fi
}

# Check external API dependencies
check_external_apis() {
    local env="$1"
    
    log "Checking external API dependencies"
    
    # Example external APIs (customize based on your application)
    local apis=(
        "https://api.github.com/status"
        "https://httpbin.org/status/200"
    )
    
    for api in "${apis[@]}"; do
        log "Checking API: $api"
        
        local response_code
        response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time "$TIMEOUT" "$api" || echo "000")
        
        if [[ "$response_code" -ge 200 && "$response_code" -lt 300 ]]; then
            log "API '$api' is healthy (HTTP $response_code)"
        else
            log "WARNING: API '$api' returned HTTP $response_code"
        fi
    done
}

# Check AWS services health
check_aws_services() {
    local env="$1"
    
    log "Checking AWS services health"
    
    # Check ECS service status
    local cluster_name="hotel-reviews-${env}-cluster"
    local service_name="hotel-reviews-service"
    
    if aws ecs describe-services --cluster "$cluster_name" --services "$service_name" --query 'services[0].status' --output text 2>/dev/null | grep -q "ACTIVE"; then
        log "ECS service is active"
    else
        log "WARNING: ECS service is not active"
    fi
    
    # Check RDS status
    local db_identifier="hotel-reviews-${env}-db"
    local db_status
    db_status=$(aws rds describe-db-instances --db-instance-identifier "$db_identifier" --query 'DBInstances[0].DBInstanceStatus' --output text 2>/dev/null || echo "unknown")
    
    if [[ "$db_status" == "available" ]]; then
        log "RDS instance is available"
    else
        log "WARNING: RDS instance status is '$db_status'"
    fi
    
    # Check Application Load Balancer health
    local alb_name="hotel-reviews-${env}-alb"
    local alb_state
    alb_state=$(aws elbv2 describe-load-balancers --names "$alb_name" --query 'LoadBalancers[0].State.Code' --output text 2>/dev/null || echo "unknown")
    
    if [[ "$alb_state" == "active" ]]; then
        log "Application Load Balancer is active"
    else
        log "WARNING: Application Load Balancer state is '$alb_state'"
    fi
}

# Check CloudWatch metrics and alarms
check_cloudwatch_health() {
    local env="$1"
    
    log "Checking CloudWatch alarms"
    
    # Get alarms in ALARM state
    local alarms_in_alarm
    alarms_in_alarm=$(aws cloudwatch describe-alarms --state-value ALARM --query 'MetricAlarms[?contains(AlarmName, `hotel-reviews-'$env'`)].AlarmName' --output text 2>/dev/null || echo "")
    
    if [[ -n "$alarms_in_alarm" ]]; then
        log "WARNING: The following alarms are in ALARM state:"
        echo "$alarms_in_alarm" | tr '\t' '\n' | while read -r alarm; do
            log "  - $alarm"
        done
    else
        log "No alarms in ALARM state"
    fi
    
    # Check if recent metrics are available
    local end_time
    local start_time
    end_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    start_time=$(date -u -d '15 minutes ago' +"%Y-%m-%dT%H:%M:%SZ")
    
    local cluster_name="hotel-reviews-${env}-cluster"
    local service_name="hotel-reviews-service"
    
    # Check ECS CPU metrics
    local cpu_metrics
    cpu_metrics=$(aws cloudwatch get-metric-statistics \
        --namespace AWS/ECS \
        --metric-name CPUUtilization \
        --dimensions Name=ServiceName,Value="$service_name" Name=ClusterName,Value="$cluster_name" \
        --start-time "$start_time" \
        --end-time "$end_time" \
        --period 300 \
        --statistics Average \
        --query 'Datapoints | length(@)' \
        --output text 2>/dev/null || echo "0")
    
    if [[ "$cpu_metrics" -gt 0 ]]; then
        log "ECS CPU metrics are being collected"
    else
        log "WARNING: No recent ECS CPU metrics found"
    fi
}

# Check SSL certificates
check_ssl_certificates() {
    local env="$1"
    
    log "Checking SSL certificates"
    
    # Get load balancer DNS name
    local alb_dns
    alb_dns=$(aws elbv2 describe-load-balancers --names "hotel-reviews-${env}-alb" --query 'LoadBalancers[0].DNSName' --output text 2>/dev/null || echo "")
    
    if [[ -n "$alb_dns" ]]; then
        # Check SSL certificate expiration
        local cert_info
        cert_info=$(timeout "$TIMEOUT" openssl s_client -connect "$alb_dns:443" -servername "$alb_dns" 2>/dev/null | openssl x509 -noout -dates 2>/dev/null || echo "")
        
        if [[ -n "$cert_info" ]]; then
            log "SSL certificate information:"
            echo "$cert_info" | while read -r line; do
                log "  $line"
            done
            
            # Check if certificate is expiring soon (within 30 days)
            local not_after
            not_after=$(echo "$cert_info" | grep "notAfter" | cut -d'=' -f2)
            
            if [[ -n "$not_after" ]]; then
                local expiry_timestamp
                expiry_timestamp=$(date -d "$not_after" +%s 2>/dev/null || echo "0")
                local current_timestamp
                current_timestamp=$(date +%s)
                local days_until_expiry
                days_until_expiry=$(( (expiry_timestamp - current_timestamp) / 86400 ))
                
                if [[ "$days_until_expiry" -lt 30 ]]; then
                    log "WARNING: SSL certificate expires in $days_until_expiry days"
                else
                    log "SSL certificate expires in $days_until_expiry days"
                fi
            fi
        else
            log "WARNING: Could not retrieve SSL certificate information"
        fi
    else
        log "Load balancer DNS name not found, skipping SSL check"
    fi
}

# Run dependency checks with retries
run_dependency_check() {
    local env="$1"
    local retry_count=0
    local failed_checks=()
    
    while [[ $retry_count -lt $MAX_RETRIES ]]; do
        log "Dependency check attempt $((retry_count + 1))/$MAX_RETRIES"
        
        failed_checks=()
        
        # Run all dependency checks
        if ! check_s3_dependency "$env"; then
            failed_checks+=("S3")
        fi
        
        if ! check_redis_dependency "$env"; then
            failed_checks+=("Redis")
        fi
        
        # These checks are informational and don't cause failures
        check_external_apis "$env"
        check_aws_services "$env"
        check_cloudwatch_health "$env"
        check_ssl_certificates "$env"
        
        # If no critical checks failed, we're done
        if [[ ${#failed_checks[@]} -eq 0 ]]; then
            log "All dependency checks passed"
            return 0
        fi
        
        retry_count=$((retry_count + 1))
        if [[ $retry_count -lt $MAX_RETRIES ]]; then
            log "Some dependency checks failed: ${failed_checks[*]}, retrying in ${RETRY_DELAY} seconds..."
            sleep "$RETRY_DELAY"
        fi
    done
    
    error "Dependency checks failed after $MAX_RETRIES attempts: ${failed_checks[*]}"
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
    
    # Check if AWS CLI is available
    if ! command -v aws &>/dev/null; then
        error "AWS CLI not found. Please install AWS CLI."
    fi
    
    # Check if curl is available
    if ! command -v curl &>/dev/null; then
        error "curl not found. Please install curl."
    fi
    
    # Check if openssl is available
    if ! command -v openssl &>/dev/null; then
        log "WARNING: openssl not found. SSL certificate checks will be skipped."
    fi
}

# Main execution
main() {
    log "Starting dependency health check for environment: $ENVIRONMENT"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Run dependency checks
    run_dependency_check "$ENVIRONMENT"
    
    log "Dependency health check completed successfully"
}

# Execute main function
main "$@"