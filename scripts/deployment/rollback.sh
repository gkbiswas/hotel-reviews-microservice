#!/bin/bash

# Deployment Rollback Script
# This script handles automatic rollback of deployments

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
DEPLOYMENT_ID="${DEPLOYMENT_ID:-}"
CURRENT_COLOR="${CURRENT_COLOR:-}"
TARGET_COLOR="${TARGET_COLOR:-}"
ROLLBACK_REASON="${ROLLBACK_REASON:-deployment_failure}"
TIMEOUT="${TIMEOUT:-600}"

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
            --environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --deployment-id)
                DEPLOYMENT_ID="$2"
                shift 2
                ;;
            --current-color)
                CURRENT_COLOR="$2"
                shift 2
                ;;
            --target-color)
                TARGET_COLOR="$2"
                shift 2
                ;;
            --reason)
                ROLLBACK_REASON="$2"
                shift 2
                ;;
            --timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            *)
                error "Unknown parameter: $1"
                ;;
        esac
    done
}

# Get deployment configuration
get_deployment_config() {
    local env="$1"
    local service_name="hotel-reviews-service"
    
    log "Retrieving deployment configuration for environment: $env"
    
    CLUSTER_NAME=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/cluster_name" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    BLUE_SERVICE_NAME=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/blue_service_name" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    GREEN_SERVICE_NAME=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/green_service_name" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    BLUE_TARGET_GROUP_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/blue_target_group_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    GREEN_TARGET_GROUP_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/green_target_group_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    LISTENER_RULE_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/listener_rule_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    
    if [[ -z "$CLUSTER_NAME" || -z "$BLUE_SERVICE_NAME" || -z "$GREEN_SERVICE_NAME" ]]; then
        error "Failed to retrieve deployment configuration from SSM"
    fi
    
    log "Deployment configuration loaded:"
    log "  Cluster: $CLUSTER_NAME"
    log "  Blue Service: $BLUE_SERVICE_NAME"
    log "  Green Service: $GREEN_SERVICE_NAME"
}

# Switch traffic back to previous version
switch_traffic_back() {
    local current_color="$1"
    
    log "Switching traffic back to previous version: $current_color"
    
    # Create the forward action configuration
    local forward_config
    if [[ "$current_color" == "blue" ]]; then
        forward_config=$(cat << EOF
{
    "TargetGroups": [
        {
            "TargetGroupArn": "$BLUE_TARGET_GROUP_ARN",
            "Weight": 100
        },
        {
            "TargetGroupArn": "$GREEN_TARGET_GROUP_ARN",
            "Weight": 0
        }
    ]
}
EOF
)
    else
        forward_config=$(cat << EOF
{
    "TargetGroups": [
        {
            "TargetGroupArn": "$BLUE_TARGET_GROUP_ARN",
            "Weight": 0
        },
        {
            "TargetGroupArn": "$GREEN_TARGET_GROUP_ARN",
            "Weight": 100
        }
    ]
}
EOF
)
    fi
    
    # Update the listener rule
    if aws elbv2 modify-rule \
        --rule-arn "$LISTENER_RULE_ARN" \
        --actions Type=forward,ForwardConfig="$forward_config" \
        >/dev/null 2>&1; then
        
        log "Traffic switched back to $current_color successfully"
    else
        error "Failed to switch traffic back to $current_color"
    fi
}

# Scale down failed deployment
scale_down_failed_deployment() {
    local target_color="$1"
    
    log "Scaling down failed deployment: $target_color"
    
    local service_name
    if [[ "$target_color" == "blue" ]]; then
        service_name="$BLUE_SERVICE_NAME"
    else
        service_name="$GREEN_SERVICE_NAME"
    fi
    
    # Scale down to 0
    if aws ecs update-service \
        --cluster "$CLUSTER_NAME" \
        --service "$service_name" \
        --desired-count 0 \
        >/dev/null 2>&1; then
        
        log "Failed deployment scaled down: $service_name"
        
        # Wait for service to be stable
        if timeout "$TIMEOUT" aws ecs wait services-stable --cluster "$CLUSTER_NAME" --services "$service_name" 2>/dev/null; then
            log "Service is stable after scaling down: $service_name"
        else
            log "WARNING: Service did not become stable after scaling down: $service_name"
        fi
    else
        error "Failed to scale down failed deployment: $service_name"
    fi
}

# Verify rollback success
verify_rollback_success() {
    local current_color="$1"
    
    log "Verifying rollback success"
    
    # Check service health
    local service_name
    local target_group_arn
    
    if [[ "$current_color" == "blue" ]]; then
        service_name="$BLUE_SERVICE_NAME"
        target_group_arn="$BLUE_TARGET_GROUP_ARN"
    else
        service_name="$GREEN_SERVICE_NAME"
        target_group_arn="$GREEN_TARGET_GROUP_ARN"
    fi
    
    # Check ECS service health
    local service_status
    service_status=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$service_name" --query 'services[0].status' --output text 2>/dev/null || echo "UNKNOWN")
    
    if [[ "$service_status" != "ACTIVE" ]]; then
        error "Service status is not ACTIVE after rollback: $service_status"
    fi
    
    # Check running vs desired count
    local running_count
    local desired_count
    running_count=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$service_name" --query 'services[0].runningCount' --output text 2>/dev/null || echo "0")
    desired_count=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$service_name" --query 'services[0].desiredCount' --output text 2>/dev/null || echo "0")
    
    if [[ "$running_count" -ne "$desired_count" ]]; then
        error "Running task count ($running_count) does not match desired count ($desired_count) after rollback"
    fi
    
    log "ECS service health verified: $service_name"
    
    # Check target group health
    local target_health
    target_health=$(aws elbv2 describe-target-health --target-group-arn "$target_group_arn" --query 'TargetHealthDescriptions' --output json 2>/dev/null || echo "[]")
    
    if [[ "$target_health" == "[]" ]]; then
        error "No targets found in target group after rollback"
    fi
    
    local healthy_targets
    local total_targets
    
    healthy_targets=$(echo "$target_health" | jq '[.[] | select(.TargetHealth.State == "healthy")] | length')
    total_targets=$(echo "$target_health" | jq 'length')
    
    if [[ "$healthy_targets" -ne "$total_targets" ]]; then
        error "Not all targets are healthy after rollback: $healthy_targets/$total_targets"
    fi
    
    log "Target group health verified: $healthy_targets/$total_targets healthy"
    
    # Check application health
    local load_balancer_arn
    load_balancer_arn=$(aws ssm get-parameter --name "/${ENVIRONMENT}/hotel-reviews-service/deployment/load_balancer_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    
    if [[ -n "$load_balancer_arn" ]]; then
        local load_balancer_dns
        load_balancer_dns=$(aws elbv2 describe-load-balancers --load-balancer-arns "$load_balancer_arn" --query 'LoadBalancers[0].DNSName' --output text 2>/dev/null || echo "")
        
        if [[ -n "$load_balancer_dns" ]]; then
            local health_url="http://$load_balancer_dns/api/v1/health"
            local response_code
            response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 30 "$health_url" || echo "000")
            
            if [[ "$response_code" -eq 200 ]]; then
                log "Application health verified: HTTP $response_code"
            else
                error "Application health check failed after rollback: HTTP $response_code"
            fi
        fi
    fi
    
    log "Rollback verification completed successfully"
}

# Record rollback event
record_rollback_event() {
    local env="$1"
    local deployment_id="$2"
    local reason="$3"
    
    log "Recording rollback event"
    
    # Create rollback record
    local rollback_record=$(cat << EOF
{
    "rollback_timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "deployment_id": "$deployment_id",
    "environment": "$env",
    "reason": "$reason",
    "current_color": "$CURRENT_COLOR",
    "target_color": "$TARGET_COLOR",
    "rollback_success": true
}
EOF
)
    
    # Store rollback record in SSM
    local rollback_key="/${env}/hotel-reviews-service/rollbacks/${deployment_id}"
    
    if aws ssm put-parameter \
        --name "$rollback_key" \
        --value "$rollback_record" \
        --type "String" \
        --overwrite \
        >/dev/null 2>&1; then
        
        log "Rollback event recorded: $rollback_key"
    else
        log "WARNING: Failed to record rollback event"
    fi
}

# Send rollback notification
send_rollback_notification() {
    local env="$1"
    local deployment_id="$2"
    local reason="$3"
    
    log "Sending rollback notification"
    
    # Get SNS topic ARN
    local sns_topic_arn
    sns_topic_arn=$(aws sns list-topics --query "Topics[?contains(TopicArn, '${env}-alerts')].TopicArn" --output text 2>/dev/null || echo "")
    
    if [[ -n "$sns_topic_arn" ]]; then
        local message="Deployment Rollback Alert

Environment: $env
Deployment ID: $deployment_id
Reason: $reason
Current Color: $CURRENT_COLOR
Target Color: $TARGET_COLOR
Rollback Time: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

The deployment has been automatically rolled back due to: $reason

Please investigate the failure and ensure the application is functioning correctly."
        
        if aws sns publish \
            --topic-arn "$sns_topic_arn" \
            --subject "Deployment Rollback - $env" \
            --message "$message" \
            >/dev/null 2>&1; then
            
            log "Rollback notification sent successfully"
        else
            log "WARNING: Failed to send rollback notification"
        fi
    else
        log "WARNING: SNS topic not found for rollback notification"
    fi
}

# Execute rollback
execute_rollback() {
    local env="$1"
    local deployment_id="$2"
    local current_color="$3"
    local target_color="$4"
    local reason="$5"
    
    log "Starting deployment rollback"
    log "  Environment: $env"
    log "  Deployment ID: $deployment_id"
    log "  Current Color: $current_color"
    log "  Target Color: $target_color"
    log "  Reason: $reason"
    
    # Get deployment configuration
    get_deployment_config "$env"
    
    # Switch traffic back to previous version
    switch_traffic_back "$current_color"
    
    # Scale down failed deployment
    scale_down_failed_deployment "$target_color"
    
    # Verify rollback success
    verify_rollback_success "$current_color"
    
    # Record rollback event
    record_rollback_event "$env" "$deployment_id" "$reason"
    
    # Send rollback notification
    send_rollback_notification "$env" "$deployment_id" "$reason"
    
    log "Deployment rollback completed successfully"
}

# Check if rollback is needed
check_rollback_needed() {
    local target_color="$1"
    
    log "Checking if rollback is needed for: $target_color"
    
    # Check if target service is healthy
    local service_name
    local target_group_arn
    
    if [[ "$target_color" == "blue" ]]; then
        service_name="$BLUE_SERVICE_NAME"
        target_group_arn="$BLUE_TARGET_GROUP_ARN"
    else
        service_name="$GREEN_SERVICE_NAME"
        target_group_arn="$GREEN_TARGET_GROUP_ARN"
    fi
    
    # Check ECS service status
    local service_status
    service_status=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$service_name" --query 'services[0].status' --output text 2>/dev/null || echo "UNKNOWN")
    
    if [[ "$service_status" != "ACTIVE" ]]; then
        log "Service status is not ACTIVE: $service_status"
        return 0  # Rollback needed
    fi
    
    # Check if deployment is stable
    local deployment_status
    deployment_status=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$service_name" --query 'services[0].deployments[0].status' --output text 2>/dev/null || echo "UNKNOWN")
    
    if [[ "$deployment_status" == "FAILED" ]]; then
        log "Deployment status is FAILED"
        return 0  # Rollback needed
    fi
    
    # Check target group health
    local target_health
    target_health=$(aws elbv2 describe-target-health --target-group-arn "$target_group_arn" --query 'TargetHealthDescriptions' --output json 2>/dev/null || echo "[]")
    
    if [[ "$target_health" == "[]" ]]; then
        log "No targets found in target group"
        return 0  # Rollback needed
    fi
    
    local healthy_targets
    local total_targets
    
    healthy_targets=$(echo "$target_health" | jq '[.[] | select(.TargetHealth.State == "healthy")] | length')
    total_targets=$(echo "$target_health" | jq 'length')
    
    if [[ "$healthy_targets" -eq 0 ]]; then
        log "No healthy targets in target group"
        return 0  # Rollback needed
    fi
    
    log "No rollback needed - deployment appears healthy"
    return 1  # No rollback needed
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
    local required_tools=("aws" "jq" "curl")
    
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &>/dev/null; then
            error "$tool not found. Please install $tool."
        fi
    done
}

# Main execution
main() {
    log "Starting deployment rollback process"
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Set default values if not provided
    if [[ -z "$DEPLOYMENT_ID" ]]; then
        DEPLOYMENT_ID=$(date +%s)
    fi
    
    if [[ -z "$CURRENT_COLOR" || -z "$TARGET_COLOR" ]]; then
        log "Colors not specified, attempting to determine from current state"
        
        # Get deployment configuration to determine colors
        get_deployment_config "$ENVIRONMENT"
        
        # Check which service is currently active
        local blue_desired
        local green_desired
        
        blue_desired=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$BLUE_SERVICE_NAME" --query 'services[0].desiredCount' --output text 2>/dev/null || echo "0")
        green_desired=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$GREEN_SERVICE_NAME" --query 'services[0].desiredCount' --output text 2>/dev/null || echo "0")
        
        if [[ "$blue_desired" -gt "$green_desired" ]]; then
            CURRENT_COLOR="blue"
            TARGET_COLOR="green"
        else
            CURRENT_COLOR="green"
            TARGET_COLOR="blue"
        fi
        
        log "Determined colors: Current=$CURRENT_COLOR, Target=$TARGET_COLOR"
    fi
    
    # Check if rollback is actually needed
    if [[ "$ROLLBACK_REASON" == "health_check" ]]; then
        get_deployment_config "$ENVIRONMENT"
        
        if check_rollback_needed "$TARGET_COLOR"; then
            log "Rollback is needed based on health checks"
        else
            log "Rollback is not needed - deployment is healthy"
            exit 0
        fi
    fi
    
    # Execute rollback
    execute_rollback "$ENVIRONMENT" "$DEPLOYMENT_ID" "$CURRENT_COLOR" "$TARGET_COLOR" "$ROLLBACK_REASON"
    
    log "Deployment rollback process completed successfully"
}

# Execute main function
main "$@"