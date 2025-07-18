#!/bin/bash

# Blue-Green Deployment Script
# This script handles blue-green deployment for ECS services

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
DEPLOYMENT_TYPE="${DEPLOYMENT_TYPE:-blue-green}"
IMAGE_URI="${IMAGE_URI:-}"
TIMEOUT="${TIMEOUT:-600}"
HEALTH_CHECK_RETRIES="${HEALTH_CHECK_RETRIES:-30}"
HEALTH_CHECK_INTERVAL="${HEALTH_CHECK_INTERVAL:-10}"

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
            --image)
                IMAGE_URI="$2"
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
            --timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            *)
                error "Unknown parameter: $1"
                ;;
        esac
    done
    
    if [[ -z "$IMAGE_URI" ]]; then
        error "Image URI is required (--image)"
    fi
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
    LOAD_BALANCER_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/load_balancer_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    LISTENER_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/listener_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    LISTENER_RULE_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/listener_rule_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    TASK_DEFINITION_ARN=$(aws ssm get-parameter --name "/${env}/${service_name}/deployment/task_definition_arn" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    
    if [[ -z "$CLUSTER_NAME" || -z "$BLUE_SERVICE_NAME" || -z "$GREEN_SERVICE_NAME" ]]; then
        error "Failed to retrieve deployment configuration from SSM"
    fi
    
    log "Deployment configuration loaded:"
    log "  Cluster: $CLUSTER_NAME"
    log "  Blue Service: $BLUE_SERVICE_NAME"
    log "  Green Service: $GREEN_SERVICE_NAME"
    log "  Image URI: $IMAGE_URI"
}

# Determine current active color
determine_current_color() {
    log "Determining current active color"
    
    # Check which target group is receiving traffic
    local listener_rules
    listener_rules=$(aws elbv2 describe-rules --listener-arn "$LISTENER_ARN" --query 'Rules[?Priority==`100`].Actions[0].ForwardConfig.TargetGroups' --output json 2>/dev/null || echo "[]")
    
    if [[ "$listener_rules" == "[]" ]]; then
        log "No listener rules found, defaulting to blue as current"
        CURRENT_COLOR="blue"
        TARGET_COLOR="green"
    else
        # Parse the weights to determine active color
        local blue_weight
        local green_weight
        
        blue_weight=$(echo "$listener_rules" | jq -r --arg arn "$BLUE_TARGET_GROUP_ARN" '.[] | .[] | select(.TargetGroupArn == $arn) | .Weight // 0')
        green_weight=$(echo "$listener_rules" | jq -r --arg arn "$GREEN_TARGET_GROUP_ARN" '.[] | .[] | select(.TargetGroupArn == $arn) | .Weight // 0')
        
        if [[ "$blue_weight" -gt "$green_weight" ]]; then
            CURRENT_COLOR="blue"
            TARGET_COLOR="green"
        else
            CURRENT_COLOR="green"
            TARGET_COLOR="blue"
        fi
    fi
    
    log "Current active color: $CURRENT_COLOR"
    log "Target deployment color: $TARGET_COLOR"
    
    # Set service names based on colors
    if [[ "$TARGET_COLOR" == "blue" ]]; then
        TARGET_SERVICE_NAME="$BLUE_SERVICE_NAME"
        TARGET_TARGET_GROUP_ARN="$BLUE_TARGET_GROUP_ARN"
        CURRENT_SERVICE_NAME="$GREEN_SERVICE_NAME"
        CURRENT_TARGET_GROUP_ARN="$GREEN_TARGET_GROUP_ARN"
    else
        TARGET_SERVICE_NAME="$GREEN_SERVICE_NAME"
        TARGET_TARGET_GROUP_ARN="$GREEN_TARGET_GROUP_ARN"
        CURRENT_SERVICE_NAME="$BLUE_SERVICE_NAME"
        CURRENT_TARGET_GROUP_ARN="$BLUE_TARGET_GROUP_ARN"
    fi
}

# Get current task definition
get_current_task_definition() {
    local service_name="$1"
    
    log "Getting current task definition for service: $service_name"
    
    local task_definition_arn
    task_definition_arn=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$service_name" --query 'services[0].taskDefinition' --output text 2>/dev/null || echo "")
    
    if [[ -z "$task_definition_arn" ]]; then
        error "Failed to get task definition for service: $service_name"
    fi
    
    log "Current task definition: $task_definition_arn"
    echo "$task_definition_arn"
}

# Create new task definition with updated image
create_new_task_definition() {
    local current_task_definition_arn="$1"
    local new_image_uri="$2"
    
    log "Creating new task definition with image: $new_image_uri"
    
    # Get current task definition
    local task_definition
    task_definition=$(aws ecs describe-task-definition --task-definition "$current_task_definition_arn" --query 'taskDefinition' --output json 2>/dev/null || echo "{}")
    
    if [[ "$task_definition" == "{}" ]]; then
        error "Failed to retrieve current task definition"
    fi
    
    # Update the image URI in the task definition
    local updated_task_definition
    updated_task_definition=$(echo "$task_definition" | jq --arg image "$new_image_uri" '
        .containerDefinitions[0].image = $image |
        del(.taskDefinitionArn, .revision, .status, .requiresAttributes, .placementConstraints, .compatibilities, .registeredAt, .registeredBy)
    ')
    
    # Add deployment metadata
    local deployment_id
    deployment_id=$(date +%s)
    
    updated_task_definition=$(echo "$updated_task_definition" | jq --arg id "$deployment_id" --arg env "$ENVIRONMENT" --arg color "$TARGET_COLOR" '
        .tags = [
            {
                "key": "DeploymentId",
                "value": $id
            },
            {
                "key": "Environment",
                "value": $env
            },
            {
                "key": "Color",
                "value": $color
            },
            {
                "key": "DeployedAt",
                "value": (now | strftime("%Y-%m-%dT%H:%M:%SZ"))
            }
        ]
    ')
    
    # Register new task definition
    local new_task_definition_arn
    new_task_definition_arn=$(aws ecs register-task-definition --cli-input-json "$updated_task_definition" --query 'taskDefinition.taskDefinitionArn' --output text 2>/dev/null || echo "")
    
    if [[ -z "$new_task_definition_arn" ]]; then
        error "Failed to register new task definition"
    fi
    
    log "New task definition registered: $new_task_definition_arn"
    
    # Export deployment ID for use in other scripts
    DEPLOYMENT_ID="$deployment_id"
    
    echo "$new_task_definition_arn"
}

# Update ECS service with new task definition
update_ecs_service() {
    local service_name="$1"
    local task_definition_arn="$2"
    local desired_count="$3"
    
    log "Updating ECS service: $service_name"
    log "  Task Definition: $task_definition_arn"
    log "  Desired Count: $desired_count"
    
    # Update service
    if aws ecs update-service \
        --cluster "$CLUSTER_NAME" \
        --service "$service_name" \
        --task-definition "$task_definition_arn" \
        --desired-count "$desired_count" \
        --force-new-deployment \
        --deployment-configuration maximumPercent=200,minimumHealthyPercent=100 \
        >/dev/null 2>&1; then
        
        log "Service update initiated successfully"
    else
        error "Failed to update ECS service"
    fi
}

# Wait for service to be stable
wait_for_service_stable() {
    local service_name="$1"
    local timeout="$2"
    
    log "Waiting for service to be stable: $service_name (timeout: ${timeout}s)"
    
    if timeout "$timeout" aws ecs wait services-stable --cluster "$CLUSTER_NAME" --services "$service_name" 2>/dev/null; then
        log "Service is stable: $service_name"
        return 0
    else
        error "Service did not become stable within timeout: $service_name"
    fi
}

# Check target group health
check_target_group_health() {
    local target_group_arn="$1"
    local max_retries="$2"
    local interval="$3"
    
    log "Checking target group health: $target_group_arn"
    
    local retry_count=0
    while [[ $retry_count -lt $max_retries ]]; do
        log "Health check attempt $((retry_count + 1))/$max_retries"
        
        # Get target health
        local target_health
        target_health=$(aws elbv2 describe-target-health --target-group-arn "$target_group_arn" --query 'TargetHealthDescriptions' --output json 2>/dev/null || echo "[]")
        
        if [[ "$target_health" == "[]" ]]; then
            log "No targets found in target group"
            sleep "$interval"
            retry_count=$((retry_count + 1))
            continue
        fi
        
        # Check if all targets are healthy
        local healthy_targets
        local total_targets
        
        healthy_targets=$(echo "$target_health" | jq '[.[] | select(.TargetHealth.State == "healthy")] | length')
        total_targets=$(echo "$target_health" | jq 'length')
        
        log "Healthy targets: $healthy_targets/$total_targets"
        
        if [[ "$healthy_targets" -eq "$total_targets" && "$total_targets" -gt 0 ]]; then
            log "All targets are healthy"
            return 0
        fi
        
        # Log unhealthy targets
        echo "$target_health" | jq -r '.[] | select(.TargetHealth.State != "healthy") | "\(.Target.Id): \(.TargetHealth.State) - \(.TargetHealth.Description)"' | while read -r target_info; do
            log "  Unhealthy target: $target_info"
        done
        
        sleep "$interval"
        retry_count=$((retry_count + 1))
    done
    
    error "Target group health check failed after $max_retries attempts"
}

# Perform application health check
perform_application_health_check() {
    local target_group_arn="$1"
    
    log "Performing application health check"
    
    # Get load balancer DNS name
    local load_balancer_dns
    load_balancer_dns=$(aws elbv2 describe-load-balancers --load-balancer-arns "$LOAD_BALANCER_ARN" --query 'LoadBalancers[0].DNSName' --output text 2>/dev/null || echo "")
    
    if [[ -z "$load_balancer_dns" ]]; then
        error "Failed to get load balancer DNS name"
    fi
    
    # Test health endpoint
    local health_url="http://$load_balancer_dns/api/v1/health"
    local max_retries=10
    local retry_count=0
    
    while [[ $retry_count -lt $max_retries ]]; do
        log "Application health check attempt $((retry_count + 1))/$max_retries"
        
        local response_code
        response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 30 "$health_url" || echo "000")
        
        if [[ "$response_code" -eq 200 ]]; then
            log "Application health check passed"
            return 0
        else
            log "Application health check failed (HTTP $response_code)"
        fi
        
        sleep 10
        retry_count=$((retry_count + 1))
    done
    
    error "Application health check failed after $max_retries attempts"
}

# Scale down current service
scale_down_current_service() {
    local service_name="$1"
    
    log "Scaling down current service: $service_name"
    
    # Get current desired count
    local current_desired_count
    current_desired_count=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$service_name" --query 'services[0].desiredCount' --output text 2>/dev/null || echo "0")
    
    if [[ "$current_desired_count" -eq 0 ]]; then
        log "Service is already scaled down: $service_name"
        return 0
    fi
    
    # Scale down to 0
    if aws ecs update-service \
        --cluster "$CLUSTER_NAME" \
        --service "$service_name" \
        --desired-count 0 \
        >/dev/null 2>&1; then
        
        log "Service scaled down to 0: $service_name"
        
        # Wait for service to be stable
        wait_for_service_stable "$service_name" 300
    else
        error "Failed to scale down service: $service_name"
    fi
}

# Execute blue-green deployment
execute_blue_green_deployment() {
    local env="$1"
    local image_uri="$2"
    
    log "Starting blue-green deployment"
    log "  Environment: $env"
    log "  Image URI: $image_uri"
    
    # Get deployment configuration
    get_deployment_config "$env"
    
    # Determine current and target colors
    determine_current_color
    
    # Get current task definition
    local current_task_definition_arn
    current_task_definition_arn=$(get_current_task_definition "$CURRENT_SERVICE_NAME")
    
    # Create new task definition
    local new_task_definition_arn
    new_task_definition_arn=$(create_new_task_definition "$current_task_definition_arn" "$image_uri")
    
    # Get desired count from current service
    local desired_count
    desired_count=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$CURRENT_SERVICE_NAME" --query 'services[0].desiredCount' --output text 2>/dev/null || echo "2")
    
    log "Desired count: $desired_count"
    
    # Update target service with new task definition
    update_ecs_service "$TARGET_SERVICE_NAME" "$new_task_definition_arn" "$desired_count"
    
    # Wait for deployment to complete
    wait_for_service_stable "$TARGET_SERVICE_NAME" "$TIMEOUT"
    
    # Check target group health
    check_target_group_health "$TARGET_TARGET_GROUP_ARN" "$HEALTH_CHECK_RETRIES" "$HEALTH_CHECK_INTERVAL"
    
    # Perform application health check
    perform_application_health_check "$TARGET_TARGET_GROUP_ARN"
    
    log "Blue-green deployment completed successfully"
    log "  Current color: $CURRENT_COLOR"
    log "  Target color: $TARGET_COLOR"
    log "  Deployment ID: $DEPLOYMENT_ID"
    
    # Output deployment information for other scripts
    echo "::set-output name=deployment-id::$DEPLOYMENT_ID"
    echo "::set-output name=current-color::$CURRENT_COLOR"
    echo "::set-output name=target-color::$TARGET_COLOR"
    
    # Also output to environment for shell scripts
    export DEPLOYMENT_ID
    export CURRENT_COLOR
    export TARGET_COLOR
}

# Execute canary deployment
execute_canary_deployment() {
    local env="$1"
    local image_uri="$2"
    
    log "Starting canary deployment"
    log "  Environment: $env"
    log "  Image URI: $image_uri"
    
    # Get deployment configuration
    get_deployment_config "$env"
    
    # Determine current and target colors
    determine_current_color
    
    # Get current task definition
    local current_task_definition_arn
    current_task_definition_arn=$(get_current_task_definition "$CURRENT_SERVICE_NAME")
    
    # Create new task definition
    local new_task_definition_arn
    new_task_definition_arn=$(create_new_task_definition "$current_task_definition_arn" "$image_uri")
    
    # For canary deployment, start with 1 instance
    local canary_count=1
    
    log "Starting canary deployment with $canary_count instance(s)"
    
    # Update target service with new task definition
    update_ecs_service "$TARGET_SERVICE_NAME" "$new_task_definition_arn" "$canary_count"
    
    # Wait for deployment to complete
    wait_for_service_stable "$TARGET_SERVICE_NAME" "$TIMEOUT"
    
    # Check target group health
    check_target_group_health "$TARGET_TARGET_GROUP_ARN" "$HEALTH_CHECK_RETRIES" "$HEALTH_CHECK_INTERVAL"
    
    log "Canary deployment completed successfully"
    log "  Current color: $CURRENT_COLOR"
    log "  Target color: $TARGET_COLOR"
    log "  Deployment ID: $DEPLOYMENT_ID"
    log "  Canary instances: $canary_count"
    
    # Output deployment information for other scripts
    echo "::set-output name=deployment-id::$DEPLOYMENT_ID"
    echo "::set-output name=current-color::$CURRENT_COLOR"
    echo "::set-output name=target-color::$TARGET_COLOR"
    
    # Also output to environment for shell scripts
    export DEPLOYMENT_ID
    export CURRENT_COLOR
    export TARGET_COLOR
}

# Execute rolling deployment
execute_rolling_deployment() {
    local env="$1"
    local image_uri="$2"
    
    log "Starting rolling deployment"
    log "  Environment: $env"
    log "  Image URI: $image_uri"
    
    # Get deployment configuration
    get_deployment_config "$env"
    
    # For rolling deployment, we update the current active service
    determine_current_color
    
    # Get current task definition
    local current_task_definition_arn
    current_task_definition_arn=$(get_current_task_definition "$CURRENT_SERVICE_NAME")
    
    # Create new task definition
    local new_task_definition_arn
    new_task_definition_arn=$(create_new_task_definition "$current_task_definition_arn" "$image_uri")
    
    # Get current desired count
    local desired_count
    desired_count=$(aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$CURRENT_SERVICE_NAME" --query 'services[0].desiredCount' --output text 2>/dev/null || echo "2")
    
    log "Updating current service with rolling deployment"
    
    # Update current service with new task definition
    update_ecs_service "$CURRENT_SERVICE_NAME" "$new_task_definition_arn" "$desired_count"
    
    # Wait for deployment to complete
    wait_for_service_stable "$CURRENT_SERVICE_NAME" "$TIMEOUT"
    
    # Check target group health
    check_target_group_health "$CURRENT_TARGET_GROUP_ARN" "$HEALTH_CHECK_RETRIES" "$HEALTH_CHECK_INTERVAL"
    
    log "Rolling deployment completed successfully"
    log "  Current color: $CURRENT_COLOR"
    log "  Deployment ID: $DEPLOYMENT_ID"
    
    # Output deployment information for other scripts
    echo "::set-output name=deployment-id::$DEPLOYMENT_ID"
    echo "::set-output name=current-color::$CURRENT_COLOR"
    echo "::set-output name=target-color::$CURRENT_COLOR"
    
    # Also output to environment for shell scripts
    export DEPLOYMENT_ID
    export TARGET_COLOR="$CURRENT_COLOR"
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
    log "Starting deployment process"
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Execute deployment based on type
    case "$DEPLOYMENT_TYPE" in
        "blue-green")
            execute_blue_green_deployment "$ENVIRONMENT" "$IMAGE_URI"
            ;;
        "canary")
            execute_canary_deployment "$ENVIRONMENT" "$IMAGE_URI"
            ;;
        "rolling")
            execute_rolling_deployment "$ENVIRONMENT" "$IMAGE_URI"
            ;;
        *)
            error "Unknown deployment type: $DEPLOYMENT_TYPE"
            ;;
    esac
    
    log "Deployment process completed successfully"
}

# Execute main function
main "$@"