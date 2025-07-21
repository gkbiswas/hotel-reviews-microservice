#!/bin/bash
set -euo pipefail

# Hotel Reviews Microservice Deployment Script
# This script handles deployments to different environments

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DEPLOYMENT_TIMEOUT=600  # 10 minutes

# Default values
ENVIRONMENT=""
IMAGE_TAG=""
DRY_RUN=false
SKIP_TESTS=false
SKIP_MIGRATIONS=false
DEPLOYMENT_TYPE="rolling"  # rolling, blue-green, canary
VERBOSE=false

# Function to print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Hotel Reviews Microservice Deployment Script

OPTIONS:
    -e, --environment ENV    Target environment (local, staging, production)
    -t, --tag TAG           Docker image tag to deploy
    -d, --dry-run          Perform a dry run without actual deployment
    -s, --skip-tests       Skip running tests before deployment
    -m, --skip-migrations  Skip database migrations
    -T, --type TYPE        Deployment type (rolling, blue-green, canary)
    -v, --verbose          Enable verbose output
    -h, --help             Show this help message

EXAMPLES:
    $0 -e staging -t v1.2.3
    $0 -e production -t v1.2.3 -T blue-green
    $0 -e staging -t latest --dry-run

ENVIRONMENTS:
    local      - Local development environment
    staging    - Staging environment for testing
    production - Production environment

DEPLOYMENT TYPES:
    rolling    - Rolling update deployment (default)
    blue-green - Blue-green deployment with traffic switch
    canary     - Canary deployment with gradual traffic shift
EOF
}

# Function to log messages
log() {
    local level=$1
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        INFO)
            echo -e "${BLUE}[INFO]${NC} ${timestamp} - $message"
            ;;
        WARN)
            echo -e "${YELLOW}[WARN]${NC} ${timestamp} - $message"
            ;;
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${timestamp} - $message"
            ;;
        SUCCESS)
            echo -e "${GREEN}[SUCCESS]${NC} ${timestamp} - $message"
            ;;
        DEBUG)
            if [[ "$VERBOSE" == "true" ]]; then
                echo -e "[DEBUG] ${timestamp} - $message"
            fi
            ;;
    esac
}

# Function to check prerequisites
check_prerequisites() {
    log INFO "Checking prerequisites..."
    
    # Check required tools
    local required_tools=("docker" "kubectl" "aws" "jq")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            log ERROR "Required tool '$tool' is not installed"
            exit 1
        fi
    done
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        log ERROR "AWS credentials not configured or invalid"
        exit 1
    fi
    
    # Check kubectl context
    if [[ "$ENVIRONMENT" != "local" ]]; then
        local current_context=$(kubectl config current-context)
        log DEBUG "Current kubectl context: $current_context"
        
        if ! kubectl cluster-info &> /dev/null; then
            log ERROR "Unable to connect to Kubernetes cluster"
            exit 1
        fi
    fi
    
    log SUCCESS "Prerequisites check passed"
}

# Function to validate environment configuration
validate_environment() {
    log INFO "Validating environment configuration..."
    
    case $ENVIRONMENT in
        local|staging|production)
            ;;
        *)
            log ERROR "Invalid environment: $ENVIRONMENT"
            log ERROR "Valid environments: local, staging, production"
            exit 1
            ;;
    esac
    
    # Check if configuration file exists
    local config_file="${PROJECT_ROOT}/configs/${ENVIRONMENT}.yml"
    if [[ ! -f "$config_file" ]]; then
        log ERROR "Configuration file not found: $config_file"
        exit 1
    fi
    
    log SUCCESS "Environment configuration validated"
}

# Function to run tests
run_tests() {
    if [[ "$SKIP_TESTS" == "true" ]]; then
        log WARN "Skipping tests as requested"
        return 0
    fi
    
    log INFO "Running tests..."
    
    cd "$PROJECT_ROOT"
    
    # Run unit tests
    if ! go test ./... -v; then
        log ERROR "Unit tests failed"
        return 1
    fi
    
    # Run integration tests for staging and production
    if [[ "$ENVIRONMENT" != "local" ]]; then
        if ! go test ./tests/integration/... -v; then
            log ERROR "Integration tests failed"
            return 1
        fi
    fi
    
    log SUCCESS "All tests passed"
}

# Function to build and push Docker image
build_and_push_image() {
    log INFO "Building and pushing Docker image..."
    
    local registry="ghcr.io/gkbiswas"
    local image_name="hotel-reviews"
    local full_image_tag="${registry}/${image_name}:${IMAGE_TAG}"
    
    cd "$PROJECT_ROOT"
    
    # Build Docker image using optimized Dockerfile
    if ! docker build -f docker/Dockerfile.optimized --target runtime -t "$full_image_tag" .; then
        log ERROR "Docker build failed"
        return 1
    fi
    
    # Push image if not dry run
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! docker push "$full_image_tag"; then
            log ERROR "Docker push failed"
            return 1
        fi
    else
        log INFO "DRY RUN: Would push image $full_image_tag"
    fi
    
    log SUCCESS "Docker image built and pushed: $full_image_tag"
}

# Function to run database migrations
run_migrations() {
    if [[ "$SKIP_MIGRATIONS" == "true" ]]; then
        log WARN "Skipping database migrations as requested"
        return 0
    fi
    
    log INFO "Running database migrations..."
    
    # Create backup before migrations (production only)
    if [[ "$ENVIRONMENT" == "production" ]]; then
        log INFO "Creating database backup before migrations..."
        if [[ "$DRY_RUN" == "false" ]]; then
            "${SCRIPT_DIR}/backup-database.sh" --environment "$ENVIRONMENT" --pre-migration
        else
            log INFO "DRY RUN: Would create database backup"
        fi
    fi
    
    # Run migrations
    local migration_script="${SCRIPT_DIR}/migrate.sh"
    if [[ -f "$migration_script" ]]; then
        if [[ "$DRY_RUN" == "false" ]]; then
            if ! "$migration_script" --environment "$ENVIRONMENT"; then
                log ERROR "Database migrations failed"
                return 1
            fi
        else
            log INFO "DRY RUN: Would run database migrations"
        fi
    else
        log WARN "Migration script not found, skipping migrations"
    fi
    
    log SUCCESS "Database migrations completed"
}

# Function to deploy to local environment
deploy_local() {
    log INFO "Deploying to local environment..."
    
    cd "$PROJECT_ROOT"
    
    # Start local services with Docker Compose
    if [[ "$DRY_RUN" == "false" ]]; then
        docker-compose -f docker-compose.local.yml up -d
        
        # Wait for services to be ready
        log INFO "Waiting for services to be ready..."
        sleep 10
        
        # Build and run the application
        go build -o bin/hotel-reviews ./cmd/api
        ./bin/hotel-reviews -config configs/local.yml &
        
        local app_pid=$!
        echo $app_pid > /tmp/hotel-reviews.pid
        
        # Wait for application to be ready
        local attempts=0
        while [[ $attempts -lt 30 ]]; do
            if curl -f http://localhost:8080/api/v1/health &> /dev/null; then
                break
            fi
            sleep 2
            ((attempts++))
        done
        
        if [[ $attempts -eq 30 ]]; then
            log ERROR "Application failed to start within timeout"
            return 1
        fi
    else
        log INFO "DRY RUN: Would start local environment with Docker Compose"
    fi
    
    log SUCCESS "Local deployment completed"
}

# Function to deploy to Kubernetes (staging/production)
deploy_kubernetes() {
    log INFO "Deploying to Kubernetes environment: $ENVIRONMENT"
    
    local registry="ghcr.io/gkbiswas"
    local image_name="hotel-reviews"
    local full_image_tag="${registry}/${image_name}:${IMAGE_TAG}"
    
    # Apply Kubernetes manifests
    local k8s_dir="${PROJECT_ROOT}/k8s/${ENVIRONMENT}"
    if [[ ! -d "$k8s_dir" ]]; then
        log ERROR "Kubernetes manifests directory not found: $k8s_dir"
        return 1
    fi
    
    case $DEPLOYMENT_TYPE in
        rolling)
            deploy_rolling_update "$full_image_tag"
            ;;
        blue-green)
            deploy_blue_green "$full_image_tag"
            ;;
        canary)
            deploy_canary "$full_image_tag"
            ;;
        *)
            log ERROR "Unknown deployment type: $DEPLOYMENT_TYPE"
            return 1
            ;;
    esac
}

# Function to perform rolling update deployment
deploy_rolling_update() {
    local image_tag=$1
    log INFO "Performing rolling update deployment..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Update deployment image
        kubectl set image deployment/hotel-reviews-service \
            hotel-reviews="$image_tag" \
            --namespace="hotel-reviews-${ENVIRONMENT}"
        
        # Wait for rollout to complete
        kubectl rollout status deployment/hotel-reviews-service \
            --namespace="hotel-reviews-${ENVIRONMENT}" \
            --timeout="${DEPLOYMENT_TIMEOUT}s"
    else
        log INFO "DRY RUN: Would update deployment with image $image_tag"
    fi
    
    log SUCCESS "Rolling update deployment completed"
}

# Function to perform blue-green deployment
deploy_blue_green() {
    local image_tag=$1
    log INFO "Performing blue-green deployment..."
    
    # Determine current and target colors
    local current_color=$(kubectl get service hotel-reviews-service \
        --namespace="hotel-reviews-${ENVIRONMENT}" \
        -o jsonpath='{.spec.selector.color}' 2>/dev/null || echo "blue")
    
    local target_color="green"
    if [[ "$current_color" == "green" ]]; then
        target_color="blue"
    fi
    
    log INFO "Current color: $current_color, Target color: $target_color"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Deploy to target environment
        kubectl set image deployment/hotel-reviews-service-"$target_color" \
            hotel-reviews="$image_tag" \
            --namespace="hotel-reviews-${ENVIRONMENT}"
        
        # Wait for deployment to be ready
        kubectl rollout status deployment/hotel-reviews-service-"$target_color" \
            --namespace="hotel-reviews-${ENVIRONMENT}" \
            --timeout="${DEPLOYMENT_TIMEOUT}s"
        
        # Run health checks on target environment
        health_check_blue_green "$target_color"
        
        # Switch traffic to target environment
        kubectl patch service hotel-reviews-service \
            --namespace="hotel-reviews-${ENVIRONMENT}" \
            -p '{"spec":{"selector":{"color":"'$target_color'"}}}'
        
        log INFO "Traffic switched to $target_color environment"
    else
        log INFO "DRY RUN: Would deploy to $target_color environment and switch traffic"
    fi
    
    log SUCCESS "Blue-green deployment completed"
}

# Function to perform canary deployment
deploy_canary() {
    local image_tag=$1
    log INFO "Performing canary deployment..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Deploy canary version
        kubectl set image deployment/hotel-reviews-service-canary \
            hotel-reviews="$image_tag" \
            --namespace="hotel-reviews-${ENVIRONMENT}"
        
        # Wait for canary deployment
        kubectl rollout status deployment/hotel-reviews-service-canary \
            --namespace="hotel-reviews-${ENVIRONMENT}" \
            --timeout="${DEPLOYMENT_TIMEOUT}s"
        
        # Gradually increase canary traffic
        local traffic_percentages=(10 25 50 75 100)
        for percentage in "${traffic_percentages[@]}"; do
            log INFO "Switching $percentage% traffic to canary"
            
            # Update traffic split (implementation depends on service mesh/ingress)
            # This is a placeholder - actual implementation would depend on Istio/Nginx/etc.
            
            # Monitor metrics for 2 minutes
            sleep 120
            
            # Check if canary is healthy
            if ! health_check_canary; then
                log ERROR "Canary health check failed, rolling back"
                # Rollback canary
                kubectl rollout undo deployment/hotel-reviews-service-canary \
                    --namespace="hotel-reviews-${ENVIRONMENT}"
                return 1
            fi
        done
        
        # Promote canary to main deployment
        kubectl set image deployment/hotel-reviews-service \
            hotel-reviews="$image_tag" \
            --namespace="hotel-reviews-${ENVIRONMENT}"
        
        kubectl rollout status deployment/hotel-reviews-service \
            --namespace="hotel-reviews-${ENVIRONMENT}" \
            --timeout="${DEPLOYMENT_TIMEOUT}s"
    else
        log INFO "DRY RUN: Would perform canary deployment with gradual traffic shift"
    fi
    
    log SUCCESS "Canary deployment completed"
}

# Function to run health checks for blue-green deployment
health_check_blue_green() {
    local target_color=$1
    log INFO "Running health checks for $target_color environment..."
    
    local service_url="http://hotel-reviews-service-${target_color}.hotel-reviews-${ENVIRONMENT}.svc.cluster.local:8080"
    
    # Wait for service to be ready
    local attempts=0
    while [[ $attempts -lt 30 ]]; do
        if kubectl exec -n "hotel-reviews-${ENVIRONMENT}" \
            deployment/hotel-reviews-service-"$target_color" -- \
            curl -f "${service_url}/api/v1/health" &> /dev/null; then
            break
        fi
        sleep 10
        ((attempts++))
    done
    
    if [[ $attempts -eq 30 ]]; then
        log ERROR "Health check failed for $target_color environment"
        return 1
    fi
    
    log SUCCESS "Health check passed for $target_color environment"
}

# Function to run health checks for canary deployment
health_check_canary() {
    log INFO "Running health checks for canary deployment..."
    
    # Check error rate and response time metrics
    # This would typically query Prometheus or other monitoring system
    local error_rate=$(kubectl exec -n monitoring deployment/prometheus -- \
        promtool query instant 'rate(http_requests_total{job="hotel-reviews-canary",code=~"5.."}[5m]) / rate(http_requests_total{job="hotel-reviews-canary"}[5m])' \
        2>/dev/null | grep -o '[0-9]*\.[0-9]*' | head -1 || echo "0")
    
    if (( $(echo "$error_rate > 0.05" | bc -l) )); then
        log ERROR "Canary error rate too high: $error_rate"
        return 1
    fi
    
    log SUCCESS "Canary health checks passed"
    return 0
}

# Function to run post-deployment verification
post_deployment_verification() {
    log INFO "Running post-deployment verification..."
    
    local base_url
    case $ENVIRONMENT in
        local)
            base_url="http://localhost:8080"
            ;;
        staging)
            base_url="https://staging-api.hotel-reviews.com"
            ;;
        production)
            base_url="https://api.hotel-reviews.com"
            ;;
    esac
    
    # Run basic health checks
    local endpoints=("/api/v1/health" "/api/v1/health/ready" "/api/v1/metrics")
    for endpoint in "${endpoints[@]}"; do
        log INFO "Testing endpoint: $endpoint"
        if ! curl -f "${base_url}${endpoint}" &> /dev/null; then
            log ERROR "Endpoint check failed: $endpoint"
            return 1
        fi
    done
    
    # Run smoke tests
    if [[ -f "${SCRIPT_DIR}/../testing/smoke-tests.sh" ]]; then
        if ! "${SCRIPT_DIR}/../testing/smoke-tests.sh" --environment "$ENVIRONMENT"; then
            log ERROR "Smoke tests failed"
            return 1
        fi
    fi
    
    log SUCCESS "Post-deployment verification completed"
}

# Function to send deployment notification
send_notification() {
    local status=$1
    log INFO "Sending deployment notification..."
    
    local message
    case $status in
        success)
            message="✅ Deployment successful: hotel-reviews:${IMAGE_TAG} to ${ENVIRONMENT}"
            ;;
        failure)
            message="❌ Deployment failed: hotel-reviews:${IMAGE_TAG} to ${ENVIRONMENT}"
            ;;
    esac
    
    # Send to Slack if webhook is configured
    if [[ -n "${SLACK_WEBHOOK:-}" ]]; then
        curl -X POST "$SLACK_WEBHOOK" \
            -H 'Content-type: application/json' \
            --data "{\"text\":\"$message\"}" &> /dev/null || true
    fi
    
    log DEBUG "Notification sent: $message"
}

# Function to cleanup
cleanup() {
    log INFO "Cleaning up..."
    
    # Remove temporary files
    rm -f /tmp/hotel-reviews-deployment-*
    
    # Kill local application if running
    if [[ -f /tmp/hotel-reviews.pid ]]; then
        local pid=$(cat /tmp/hotel-reviews.pid)
        if kill "$pid" 2>/dev/null; then
            log DEBUG "Stopped local application (PID: $pid)"
        fi
        rm -f /tmp/hotel-reviews.pid
    fi
}

# Main deployment function
main() {
    log INFO "Starting deployment process..."
    log INFO "Environment: $ENVIRONMENT"
    log INFO "Image Tag: $IMAGE_TAG"
    log INFO "Deployment Type: $DEPLOYMENT_TYPE"
    log INFO "Dry Run: $DRY_RUN"
    
    # Set trap for cleanup
    trap cleanup EXIT
    
    # Run deployment steps
    check_prerequisites
    validate_environment
    run_tests
    
    if [[ "$ENVIRONMENT" != "local" ]]; then
        build_and_push_image
    fi
    
    run_migrations
    
    case $ENVIRONMENT in
        local)
            deploy_local
            ;;
        staging|production)
            deploy_kubernetes
            ;;
    esac
    
    post_deployment_verification
    
    log SUCCESS "Deployment completed successfully!"
    send_notification success
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -t|--tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -s|--skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        -m|--skip-migrations)
            SKIP_MIGRATIONS=true
            shift
            ;;
        -T|--type)
            DEPLOYMENT_TYPE="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log ERROR "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [[ -z "$ENVIRONMENT" ]]; then
    log ERROR "Environment is required"
    usage
    exit 1
fi

if [[ -z "$IMAGE_TAG" ]]; then
    if [[ "$ENVIRONMENT" == "local" ]]; then
        IMAGE_TAG="local"
    else
        log ERROR "Image tag is required for non-local environments"
        usage
        exit 1
    fi
fi

# Run main function with error handling
if ! main; then
    log ERROR "Deployment failed!"
    send_notification failure
    exit 1
fi