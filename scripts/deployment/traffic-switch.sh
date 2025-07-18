#!/bin/bash

# Traffic Switching Script for Blue-Green Deployments
# This script handles traffic switching between blue and green environments

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
DEPLOYMENT_TYPE="${DEPLOYMENT_TYPE:-blue-green}"
TARGET_COLOR="${TARGET_COLOR:-}"
CURRENT_COLOR="${CURRENT_COLOR:-}"
SWITCH_STRATEGY="${SWITCH_STRATEGY:-immediate}"
CANARY_PERCENTAGE="${CANARY_PERCENTAGE:-10}"
GRADUAL_STEPS="${GRADUAL_STEPS:-5}"
GRADUAL_INTERVAL="${GRADUAL_INTERVAL:-300}"

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
            --current-color)
                CURRENT_COLOR="$2"
                shift 2
                ;;
            --environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --deployment-type)
                DEPLOYMENT_TYPE="$2"
                shift 2
                ;;
            --strategy)
                SWITCH_STRATEGY="$2"
                shift 2
                ;;
            --canary-percentage)
                CANARY_PERCENTAGE="$2"
                shift 2
                ;;
            --gradual-steps)
                GRADUAL_STEPS="$2"
                shift 2
                ;;
            --gradual-interval)
                GRADUAL_INTERVAL="$2"
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
    
    if [[ -z "$CURRENT_COLOR" ]]; then
        error "Current color is required (--current-color)"
    fi
}

# Get deployment configuration
get_deployment_config() {
    local env="$1"
    local service_name="hotel-reviews-service"
    
    log "Retrieving deployment configuration for environment: $env"
    
    BLUE_TARGET_GROUP_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/blue_target_group_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    GREEN_TARGET_GROUP_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/green_target_group_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    LISTENER_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/listener_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    LISTENER_RULE_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/listener_rule_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    
    if [[ -z "$BLUE_TARGET_GROUP_ARN" || -z "$GREEN_TARGET_GROUP_ARN" || -z "$LISTENER_ARN" || -z "$LISTENER_RULE_ARN" ]]; then
        error "Failed to retrieve deployment configuration from SSM"
    fi
    
    # Set target group ARNs based on colors
    if [[ "$TARGET_COLOR" == "blue" ]]; then
        TARGET_TARGET_GROUP_ARN="$BLUE_TARGET_GROUP_ARN"
        CURRENT_TARGET_GROUP_ARN="$GREEN_TARGET_GROUP_ARN"
    else
        TARGET_TARGET_GROUP_ARN="$GREEN_TARGET_GROUP_ARN"
        CURRENT_TARGET_GROUP_ARN="$BLUE_TARGET_GROUP_ARN"
    fi
    
    log "Deployment configuration loaded:"
    log "  Blue Target Group: $BLUE_TARGET_GROUP_ARN"
    log "  Green Target Group: $GREEN_TARGET_GROUP_ARN"
    log "  Listener ARN: $LISTENER_ARN"
    log "  Listener Rule ARN: $LISTENER_RULE_ARN"
    log "  Current Color: $CURRENT_COLOR"
    log "  Target Color: $TARGET_COLOR"
}

# Get current traffic weights
get_current_traffic_weights() {
    log "Getting current traffic weights"
    
    local listener_rule_info
    listener_rule_info=$(aws elbv2 describe-rules --rule-arns "$LISTENER_RULE_ARN" --query 'Rules[0].Actions[0].ForwardConfig.TargetGroups' --output json 2>/dev/null || echo "[]")
    
    if [[ "$listener_rule_info" == "[]" ]]; then
        error "Failed to retrieve listener rule information"
    fi
    
    BLUE_WEIGHT=$(echo "$listener_rule_info" | jq -r --arg arn "$BLUE_TARGET_GROUP_ARN" '.[] | select(.TargetGroupArn == $arn) | .Weight // 0')
    GREEN_WEIGHT=$(echo "$listener_rule_info" | jq -r --arg arn "$GREEN_TARGET_GROUP_ARN" '.[] | select(.TargetGroupArn == $arn) | .Weight // 0')
    
    log "Current traffic weights:"
    log "  Blue: $BLUE_WEIGHT%"
    log "  Green: $GREEN_WEIGHT%"
}

# Update traffic weights
update_traffic_weights() {
    local blue_weight="$1"
    local green_weight="$2"
    
    log "Updating traffic weights: Blue=$blue_weight%, Green=$green_weight%"
    
    # Create the forward action configuration
    local forward_config=$(cat << EOF
{
    "TargetGroups": [
        {
            "TargetGroupArn": "$BLUE_TARGET_GROUP_ARN",
            "Weight": $blue_weight
        },
        {
            "TargetGroupArn": "$GREEN_TARGET_GROUP_ARN",
            "Weight": $green_weight
        }
    ]
}
EOF
)
    
    # Update the listener rule
    if aws elbv2 modify-rule \
        --rule-arn "$LISTENER_RULE_ARN" \
        --actions Type=forward,ForwardConfig="$forward_config" \
        >/dev/null 2>&1; then
        
        log "Traffic weights updated successfully"
    else
        error "Failed to update traffic weights"
    fi
    
    # Verify the update
    sleep 5
    get_current_traffic_weights
    
    if [[ "$BLUE_WEIGHT" -eq "$blue_weight" && "$GREEN_WEIGHT" -eq "$green_weight" ]]; then
        log "Traffic weight update verified"
    else
        error "Traffic weight update verification failed"
    fi
}

# Monitor traffic switch
monitor_traffic_switch() {
    local target_blue_weight="$1"
    local target_green_weight="$2"
    local monitor_duration="$3"
    
    log "Monitoring traffic switch for ${monitor_duration} seconds"
    
    local start_time
    start_time=$(date +%s)
    local end_time
    end_time=$((start_time + monitor_duration))
    
    while [[ $(date +%s) -lt $end_time ]]; do
        # Check target group health
        check_target_group_health "$TARGET_TARGET_GROUP_ARN" 3 10
        
        # Check application metrics
        check_application_metrics "$target_blue_weight" "$target_green_weight"
        
        # Check for errors
        check_error_rates
        
        sleep 30
    done
    
    log "Traffic switch monitoring completed"
}

# Check target group health
check_target_group_health() {
    local target_group_arn="$1"
    local max_retries="$2"
    local interval="$3"
    
    local retry_count=0
    while [[ $retry_count -lt $max_retries ]]; do
        # Get target health
        local target_health
        target_health=$(aws elbv2 describe-target-health --target-group-arn "$target_group_arn" --query 'TargetHealthDescriptions' --output json 2>/dev/null || echo "[]")
        
        if [[ "$target_health" == "[]" ]]; then
            log "WARNING: No targets found in target group"
            return 1
        fi
        
        # Check if all targets are healthy
        local healthy_targets
        local total_targets
        
        healthy_targets=$(echo "$target_health" | jq '[.[] | select(.TargetHealth.State == "healthy")] | length')
        total_targets=$(echo "$target_health" | jq 'length')
        
        if [[ "$healthy_targets" -eq "$total_targets" && "$total_targets" -gt 0 ]]; then
            log "All targets are healthy: $healthy_targets/$total_targets"
            return 0
        fi
        
        log "Some targets are unhealthy: $healthy_targets/$total_targets"
        
        sleep "$interval"
        retry_count=$((retry_count + 1))
    done
    
    log "WARNING: Target group health check failed"
    return 1
}

# Check application metrics
check_application_metrics() {
    local blue_weight="$1"
    local green_weight="$2"
    
    log "Checking application metrics"
    
    # Get ALB metrics
    local end_time
    local start_time
    end_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    start_time=$(date -u -d '5 minutes ago' +"%Y-%m-%dT%H:%M:%SZ")
    
    # Check request count
    local request_count
    request_count=$(aws cloudwatch get-metric-statistics \
        --namespace AWS/ApplicationELB \
        --metric-name RequestCount \
        --dimensions Name=LoadBalancer,Value="$(basename "$LOAD_BALANCER_ARN")" \
        --start-time "$start_time" \
        --end-time "$end_time" \
        --period 300 \
        --statistics Sum \
        --query 'Datapoints[0].Sum' \
        --output text 2>/dev/null || echo "0")
    
    # Check target response time
    local response_time
    response_time=$(aws cloudwatch get-metric-statistics \
        --namespace AWS/ApplicationELB \
        --metric-name TargetResponseTime \
        --dimensions Name=LoadBalancer,Value="$(basename "$LOAD_BALANCER_ARN")" \
        --start-time "$start_time" \
        --end-time "$end_time" \
        --period 300 \
        --statistics Average \
        --query 'Datapoints[0].Average' \
        --output text 2>/dev/null || echo "0")
    
    # Check HTTP 5xx errors
    local error_count
    error_count=$(aws cloudwatch get-metric-statistics \
        --namespace AWS/ApplicationELB \
        --metric-name HTTPCode_Target_5XX_Count \
        --dimensions Name=LoadBalancer,Value="$(basename "$LOAD_BALANCER_ARN")" \
        --start-time "$start_time" \
        --end-time "$end_time" \
        --period 300 \
        --statistics Sum \
        --query 'Datapoints[0].Sum' \
        --output text 2>/dev/null || echo "0")
    
    log "Application metrics:"
    log "  Request Count: $request_count"
    log "  Response Time: ${response_time}s"
    log "  Error Count: $error_count"
    
    # Check if metrics are within acceptable ranges
    if [[ "$request_count" != "None" && "$request_count" != "0" ]]; then
        local error_rate
        error_rate=$(echo "scale=2; $error_count / $request_count * 100" | bc -l 2>/dev/null || echo "0")
        
        log "  Error Rate: ${error_rate}%"
        
        if (( $(echo "$error_rate > 5" | bc -l) )); then
            log "WARNING: High error rate detected: ${error_rate}%"
            return 1
        fi
    fi
    
    if [[ "$response_time" != "None" && "$response_time" != "0" ]]; then
        if (( $(echo "$response_time > 5" | bc -l) )); then
            log "WARNING: High response time detected: ${response_time}s"
            return 1
        fi
    fi
    
    log "Application metrics are within acceptable ranges"
    return 0
}

# Check error rates
check_error_rates() {
    log "Checking error rates"
    
    # Get error metrics for the last 5 minutes
    local end_time
    local start_time
    end_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    start_time=$(date -u -d '5 minutes ago' +"%Y-%m-%dT%H:%M:%SZ")
    
    # Check HTTP 4xx errors
    local error_4xx
    error_4xx=$(aws cloudwatch get-metric-statistics \
        --namespace AWS/ApplicationELB \
        --metric-name HTTPCode_Target_4XX_Count \
        --dimensions Name=LoadBalancer,Value="$(basename "$LOAD_BALANCER_ARN")" \
        --start-time "$start_time" \
        --end-time "$end_time" \
        --period 300 \
        --statistics Sum \
        --query 'Datapoints[0].Sum' \
        --output text 2>/dev/null || echo "0")
    
    # Check HTTP 5xx errors
    local error_5xx
    error_5xx=$(aws cloudwatch get-metric-statistics \
        --namespace AWS/ApplicationELB \
        --metric-name HTTPCode_Target_5XX_Count \
        --dimensions Name=LoadBalancer,Value="$(basename "$LOAD_BALANCER_ARN")" \
        --start-time "$start_time" \
        --end-time "$end_time" \
        --period 300 \
        --statistics Sum \
        --query 'Datapoints[0].Sum' \
        --output text 2>/dev/null || echo "0")
    
    log "Error rates:"
    log "  4xx errors: $error_4xx"
    log "  5xx errors: $error_5xx"
    
    # Check if error rates are acceptable
    if [[ "$error_5xx" != "None" && "$error_5xx" != "0" ]]; then
        if [[ "$error_5xx" -gt 10 ]]; then
            log "WARNING: High 5xx error rate detected: $error_5xx"
            return 1
        fi
    fi
    
    return 0
}

# Execute immediate traffic switch
execute_immediate_switch() {
    log "Executing immediate traffic switch"
    
    if [[ "$TARGET_COLOR" == "blue" ]]; then
        update_traffic_weights 100 0
    else
        update_traffic_weights 0 100
    fi
    
    # Monitor for 2 minutes
    monitor_traffic_switch 100 0 120
    
    log "Immediate traffic switch completed"
}

# Execute canary traffic switch
execute_canary_switch() {
    local canary_percentage="$1"
    
    log "Executing canary traffic switch with $canary_percentage% traffic"
    
    local current_percentage
    current_percentage=$((100 - canary_percentage))
    
    if [[ "$TARGET_COLOR" == "blue" ]]; then
        update_traffic_weights "$canary_percentage" "$current_percentage"
    else
        update_traffic_weights "$current_percentage" "$canary_percentage"
    fi
    
    # Monitor canary deployment
    monitor_traffic_switch "$canary_percentage" "$current_percentage" 300
    
    log "Canary traffic switch completed"
}

# Execute gradual traffic switch
execute_gradual_switch() {
    local steps="$1"
    local interval="$2"
    
    log "Executing gradual traffic switch with $steps steps, ${interval}s interval"
    
    local step_percentage
    step_percentage=$((100 / steps))
    
    for ((i=1; i<=steps; i++)); do
        local target_percentage
        target_percentage=$((i * step_percentage))
        
        if [[ "$target_percentage" -gt 100 ]]; then
            target_percentage=100
        fi
        
        local current_percentage
        current_percentage=$((100 - target_percentage))
        
        log "Step $i/$steps: Switching to $target_percentage% traffic"
        
        if [[ "$TARGET_COLOR" == "blue" ]]; then
            update_traffic_weights "$target_percentage" "$current_percentage"
        else
            update_traffic_weights "$current_percentage" "$target_percentage"
        fi
        
        # Monitor this step
        monitor_traffic_switch "$target_percentage" "$current_percentage" "$interval"
        
        # If not the last step, wait before next step
        if [[ $i -lt $steps ]]; then
            log "Waiting ${interval}s before next step"
            sleep "$interval"
        fi
    done
    
    log "Gradual traffic switch completed"
}

# Execute blue-green traffic switch
execute_blue_green_switch() {
    local strategy="$1"
    
    log "Executing blue-green traffic switch with strategy: $strategy"
    
    case "$strategy" in
        "immediate")
            execute_immediate_switch
            ;;
        "canary")
            execute_canary_switch "$CANARY_PERCENTAGE"
            ;;
        "gradual")
            execute_gradual_switch "$GRADUAL_STEPS" "$GRADUAL_INTERVAL"
            ;;
        *)
            error "Unknown switch strategy: $strategy"
            ;;
    esac
}

# Execute canary traffic increase
execute_canary_increase() {
    log "Executing canary traffic increase"
    
    # Get current weights
    get_current_traffic_weights
    
    # Increase canary traffic gradually
    local target_percentage=50
    
    if [[ "$TARGET_COLOR" == "blue" ]]; then
        if [[ "$BLUE_WEIGHT" -lt "$target_percentage" ]]; then
            update_traffic_weights "$target_percentage" $((100 - target_percentage))
        fi
    else
        if [[ "$GREEN_WEIGHT" -lt "$target_percentage" ]]; then
            update_traffic_weights $((100 - target_percentage)) "$target_percentage"
        fi
    fi
    
    # Monitor increased traffic
    monitor_traffic_switch "$target_percentage" $((100 - target_percentage)) 300
    
    log "Canary traffic increase completed"
}

# Rollback traffic switch
rollback_traffic_switch() {
    log "Rolling back traffic switch"
    
    if [[ "$CURRENT_COLOR" == "blue" ]]; then
        update_traffic_weights 100 0
    else
        update_traffic_weights 0 100
    fi
    
    log "Traffic switch rollback completed"
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
    local required_tools=("aws" "jq" "bc")
    
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &>/dev/null; then
            error "$tool not found. Please install $tool."
        fi
    done
}

# Main execution
main() {
    log "Starting traffic switching process"
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Get deployment configuration
    get_deployment_config "$ENVIRONMENT"
    
    # Get current traffic weights
    get_current_traffic_weights
    
    # Execute traffic switch based on deployment type
    case "$DEPLOYMENT_TYPE" in
        "blue-green")
            execute_blue_green_switch "$SWITCH_STRATEGY"
            ;;
        "canary")
            execute_canary_increase
            ;;
        "rolling")
            log "Rolling deployment does not require traffic switching"
            ;;
        *)
            error "Unknown deployment type: $DEPLOYMENT_TYPE"
            ;;
    esac
    
    log "Traffic switching process completed successfully"
}

# Execute main function
main "$@"