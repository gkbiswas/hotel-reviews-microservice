#!/bin/bash

# Hotel Reviews Kubernetes Deployment Script
# This script deploys the Hotel Reviews application to Kubernetes using Kustomize

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
ENVIRONMENT="staging"
DRY_RUN=false
WAIT_FOR_ROLLOUT=true
VALIDATE_ONLY=false
VERBOSE=false

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}[HEADER]${NC} $1"
}

# Function to show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Deploy Hotel Reviews application to Kubernetes

OPTIONS:
    -e, --environment   Environment to deploy to (staging|production) [default: staging]
    -d, --dry-run       Perform a dry run without applying changes
    -w, --no-wait       Don't wait for rollout to complete
    -v, --validate      Only validate manifests without deploying
    -V, --verbose       Enable verbose output
    -h, --help          Show this help message

EXAMPLES:
    $0 -e staging                    # Deploy to staging
    $0 -e production                 # Deploy to production
    $0 -e staging -d                 # Dry run for staging
    $0 -e production -v              # Validate production manifests
    $0 -e production -w              # Deploy to production without waiting

EOF
}

# Function to validate environment
validate_environment() {
    if [[ "$ENVIRONMENT" != "staging" && "$ENVIRONMENT" != "production" ]]; then
        print_error "Invalid environment: $ENVIRONMENT. Must be 'staging' or 'production'"
        exit 1
    fi
}

# Function to check prerequisites
check_prerequisites() {
    print_header "Checking prerequisites..."
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check if kustomize is available (or kubectl with kustomize support)
    if ! kubectl kustomize --help &> /dev/null; then
        print_error "kustomize is not available. Please install kustomize or use kubectl with kustomize support"
        exit 1
    fi
    
    # Check if we can connect to the cluster
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster. Please check your kubectl configuration"
        exit 1
    fi
    
    print_status "Prerequisites check passed"
}

# Function to validate manifests
validate_manifests() {
    print_header "Validating Kubernetes manifests..."
    
    local overlay_path="k8s/overlays/$ENVIRONMENT"
    
    if [[ ! -d "$overlay_path" ]]; then
        print_error "Overlay directory not found: $overlay_path"
        exit 1
    fi
    
    # Validate kustomization
    if ! kubectl kustomize "$overlay_path" --dry-run=client > /dev/null; then
        print_error "Kustomization validation failed for $ENVIRONMENT"
        exit 1
    fi
    
    print_status "Manifest validation passed"
}

# Function to check if namespace exists
check_namespace() {
    local namespace="hotel-reviews-$ENVIRONMENT"
    
    if ! kubectl get namespace "$namespace" &> /dev/null; then
        print_warning "Namespace $namespace does not exist. It will be created."
    else
        print_status "Namespace $namespace exists"
    fi
}

# Function to deploy application
deploy_application() {
    print_header "Deploying Hotel Reviews to $ENVIRONMENT environment..."
    
    local overlay_path="k8s/overlays/$ENVIRONMENT"
    local namespace="hotel-reviews-$ENVIRONMENT"
    
    # Apply manifests
    if [[ "$DRY_RUN" == "true" ]]; then
        print_status "Performing dry run..."
        kubectl apply -k "$overlay_path" --dry-run=client
    else
        print_status "Applying manifests..."
        kubectl apply -k "$overlay_path"
    fi
    
    if [[ "$DRY_RUN" == "false" && "$WAIT_FOR_ROLLOUT" == "true" ]]; then
        print_status "Waiting for deployments to complete..."
        
        # Wait for API deployment
        if kubectl get deployment hotel-reviews-api -n "$namespace" &> /dev/null; then
            print_status "Waiting for API deployment..."
            kubectl rollout status deployment/hotel-reviews-api -n "$namespace" --timeout=300s
        fi
        
        # Wait for Worker deployment
        if kubectl get deployment hotel-reviews-worker -n "$namespace" &> /dev/null; then
            print_status "Waiting for Worker deployment..."
            kubectl rollout status deployment/hotel-reviews-worker -n "$namespace" --timeout=300s
        fi
        
        # Wait for StatefulSets
        if kubectl get statefulset hotel-reviews-postgres -n "$namespace" &> /dev/null; then
            print_status "Waiting for PostgreSQL StatefulSet..."
            kubectl rollout status statefulset/hotel-reviews-postgres -n "$namespace" --timeout=300s
        fi
        
        if kubectl get statefulset hotel-reviews-redis -n "$namespace" &> /dev/null; then
            print_status "Waiting for Redis StatefulSet..."
            kubectl rollout status statefulset/hotel-reviews-redis -n "$namespace" --timeout=300s
        fi
        
        if kubectl get statefulset hotel-reviews-kafka -n "$namespace" &> /dev/null; then
            print_status "Waiting for Kafka StatefulSet..."
            kubectl rollout status statefulset/hotel-reviews-kafka -n "$namespace" --timeout=300s
        fi
    fi
    
    print_status "Deployment completed successfully!"
}

# Function to show deployment status
show_deployment_status() {
    print_header "Deployment Status"
    
    local namespace="hotel-reviews-$ENVIRONMENT"
    
    echo
    print_status "Pods:"
    kubectl get pods -n "$namespace" -o wide
    
    echo
    print_status "Services:"
    kubectl get services -n "$namespace"
    
    echo
    print_status "Ingress:"
    kubectl get ingress -n "$namespace"
    
    echo
    print_status "HPA:"
    kubectl get hpa -n "$namespace"
    
    echo
    print_status "PDB:"
    kubectl get pdb -n "$namespace"
    
    if [[ "$VERBOSE" == "true" ]]; then
        echo
        print_status "All resources:"
        kubectl get all -n "$namespace"
    fi
}

# Function to run health checks
run_health_checks() {
    print_header "Running health checks..."
    
    local namespace="hotel-reviews-$ENVIRONMENT"
    
    # Check if all pods are ready
    local not_ready_pods
    not_ready_pods=$(kubectl get pods -n "$namespace" --field-selector=status.phase!=Running -o jsonpath='{.items[*].metadata.name}')
    
    if [[ -n "$not_ready_pods" ]]; then
        print_warning "Some pods are not ready: $not_ready_pods"
    else
        print_status "All pods are ready"
    fi
    
    # Check if services have endpoints
    local services_without_endpoints
    services_without_endpoints=$(kubectl get endpoints -n "$namespace" -o jsonpath='{range .items[?(@.subsets[0].addresses[0].ip=="")]}{.metadata.name}{" "}{end}')
    
    if [[ -n "$services_without_endpoints" ]]; then
        print_warning "Some services have no endpoints: $services_without_endpoints"
    else
        print_status "All services have endpoints"
    fi
}

# Function to show access information
show_access_info() {
    print_header "Access Information"
    
    local namespace="hotel-reviews-$ENVIRONMENT"
    
    # Get ingress information
    local ingress_host
    ingress_host=$(kubectl get ingress hotel-reviews-ingress -n "$namespace" -o jsonpath='{.spec.rules[0].host}' 2>/dev/null || echo "Not configured")
    
    echo
    print_status "Application URLs:"
    if [[ "$ENVIRONMENT" == "staging" ]]; then
        echo "  API: https://staging-api.hotel-reviews.com"
        echo "  Admin: https://staging-admin.hotel-reviews.com"
        echo "  Metrics: https://staging-metrics.hotel-reviews.com"
    else
        echo "  API: https://api.hotel-reviews.com"
        echo "  Admin: https://admin.hotel-reviews.com"
        echo "  Metrics: https://metrics.hotel-reviews.com"
    fi
    
    echo
    print_status "Monitoring URLs:"
    if [[ "$ENVIRONMENT" == "staging" ]]; then
        echo "  Grafana: https://grafana.staging.hotel-reviews.com"
        echo "  Prometheus: https://prometheus.staging.hotel-reviews.com"
    else
        echo "  Grafana: https://grafana.hotel-reviews.com"
        echo "  Prometheus: https://prometheus.hotel-reviews.com"
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -w|--no-wait)
            WAIT_FOR_ROLLOUT=false
            shift
            ;;
        -v|--validate)
            VALIDATE_ONLY=true
            shift
            ;;
        -V|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    print_header "Hotel Reviews Kubernetes Deployment"
    echo "Environment: $ENVIRONMENT"
    echo "Dry Run: $DRY_RUN"
    echo "Validate Only: $VALIDATE_ONLY"
    echo "Wait for Rollout: $WAIT_FOR_ROLLOUT"
    echo "Verbose: $VERBOSE"
    echo
    
    # Validate input
    validate_environment
    
    # Check prerequisites
    check_prerequisites
    
    # Validate manifests
    validate_manifests
    
    # If only validation requested, exit here
    if [[ "$VALIDATE_ONLY" == "true" ]]; then
        print_status "Validation completed successfully!"
        exit 0
    fi
    
    # Check namespace
    check_namespace
    
    # Deploy application
    deploy_application
    
    # Show status if not dry run
    if [[ "$DRY_RUN" == "false" ]]; then
        show_deployment_status
        run_health_checks
        show_access_info
    fi
    
    print_status "Deployment script completed successfully!"
}

# Run main function
main "$@"