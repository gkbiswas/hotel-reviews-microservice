#!/bin/bash

# Hotel Reviews Microservice Docker Build Script
# Usage: ./build.sh [OPTIONS]

set -e

# Default values
IMAGE_NAME="hotel-reviews-microservice"
IMAGE_TAG="latest"
DOCKERFILE_PATH="docker/Dockerfile"
BUILD_CONTEXT="."
PUSH_IMAGE=false
REGISTRY=""
PLATFORM="linux/amd64"
BUILD_ARGS=""
CACHE_FROM=""
NO_CACHE=false
VERBOSE=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -n, --name NAME           Image name (default: $IMAGE_NAME)"
    echo "  -t, --tag TAG             Image tag (default: $IMAGE_TAG)"
    echo "  -f, --file FILE           Dockerfile path (default: $DOCKERFILE_PATH)"
    echo "  -c, --context DIR         Build context (default: $BUILD_CONTEXT)"
    echo "  -p, --push                Push image to registry after build"
    echo "  -r, --registry REGISTRY   Registry URL for pushing"
    echo "  --platform PLATFORM       Target platform (default: $PLATFORM)"
    echo "  --build-arg ARG=VALUE     Build argument"
    echo "  --cache-from IMAGE        Cache from image"
    echo "  --no-cache                Don't use cache when building"
    echo "  -v, --verbose             Verbose output"
    echo "  -h, --help                Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                                          # Build with defaults"
    echo "  $0 -t v1.0.0 -p                           # Build and push version 1.0.0"
    echo "  $0 --build-arg VERSION=1.0.0 --no-cache   # Build with version arg, no cache"
    echo "  $0 -r registry.example.com -p             # Push to custom registry"
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -t|--tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        -f|--file)
            DOCKERFILE_PATH="$2"
            shift 2
            ;;
        -c|--context)
            BUILD_CONTEXT="$2"
            shift 2
            ;;
        -p|--push)
            PUSH_IMAGE=true
            shift
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        --platform)
            PLATFORM="$2"
            shift 2
            ;;
        --build-arg)
            BUILD_ARGS="$BUILD_ARGS --build-arg $2"
            shift 2
            ;;
        --cache-from)
            CACHE_FROM="--cache-from $2"
            shift 2
            ;;
        --no-cache)
            NO_CACHE=true
            shift
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
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Construct full image name
if [[ -n "$REGISTRY" ]]; then
    FULL_IMAGE_NAME="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
else
    FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"
fi

# Verbose output
if [[ "$VERBOSE" == true ]]; then
    log_info "Build configuration:"
    log_info "  Image Name: $FULL_IMAGE_NAME"
    log_info "  Dockerfile: $DOCKERFILE_PATH"
    log_info "  Context: $BUILD_CONTEXT"
    log_info "  Platform: $PLATFORM"
    log_info "  Push: $PUSH_IMAGE"
    log_info "  No Cache: $NO_CACHE"
    log_info "  Build Args: $BUILD_ARGS"
    log_info "  Cache From: $CACHE_FROM"
    echo
fi

# Check if Docker is installed and running
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running"
    exit 1
fi

# Check if Dockerfile exists
if [[ ! -f "$DOCKERFILE_PATH" ]]; then
    log_error "Dockerfile not found: $DOCKERFILE_PATH"
    exit 1
fi

# Check if build context exists
if [[ ! -d "$BUILD_CONTEXT" ]]; then
    log_error "Build context directory not found: $BUILD_CONTEXT"
    exit 1
fi

# Build Docker command
DOCKER_BUILD_CMD="docker build"
DOCKER_BUILD_CMD="$DOCKER_BUILD_CMD --platform $PLATFORM"
DOCKER_BUILD_CMD="$DOCKER_BUILD_CMD -f $DOCKERFILE_PATH"
DOCKER_BUILD_CMD="$DOCKER_BUILD_CMD -t $FULL_IMAGE_NAME"

if [[ "$NO_CACHE" == true ]]; then
    DOCKER_BUILD_CMD="$DOCKER_BUILD_CMD --no-cache"
fi

if [[ -n "$BUILD_ARGS" ]]; then
    DOCKER_BUILD_CMD="$DOCKER_BUILD_CMD $BUILD_ARGS"
fi

if [[ -n "$CACHE_FROM" ]]; then
    DOCKER_BUILD_CMD="$DOCKER_BUILD_CMD $CACHE_FROM"
fi

DOCKER_BUILD_CMD="$DOCKER_BUILD_CMD $BUILD_CONTEXT"

# Print build command if verbose
if [[ "$VERBOSE" == true ]]; then
    log_info "Build command: $DOCKER_BUILD_CMD"
    echo
fi

# Start build
log_info "Starting Docker build..."
log_info "Building image: $FULL_IMAGE_NAME"

# Execute build command
if [[ "$VERBOSE" == true ]]; then
    eval $DOCKER_BUILD_CMD
else
    eval $DOCKER_BUILD_CMD > /dev/null 2>&1
fi

# Check build result
if [[ $? -eq 0 ]]; then
    log_success "Docker build completed successfully!"
else
    log_error "Docker build failed!"
    exit 1
fi

# Get image info
IMAGE_SIZE=$(docker images --format "table {{.Size}}" $FULL_IMAGE_NAME | tail -n +2)
IMAGE_ID=$(docker images --format "table {{.ID}}" $FULL_IMAGE_NAME | tail -n +2)

log_info "Image ID: $IMAGE_ID"
log_info "Image Size: $IMAGE_SIZE"

# Push image if requested
if [[ "$PUSH_IMAGE" == true ]]; then
    log_info "Pushing image to registry..."
    
    if docker push $FULL_IMAGE_NAME; then
        log_success "Image pushed successfully!"
    else
        log_error "Failed to push image!"
        exit 1
    fi
fi

# Security scan (if tools are available)
if command -v trivy &> /dev/null; then
    log_info "Running security scan with Trivy..."
    trivy image --exit-code 0 --severity HIGH,CRITICAL $FULL_IMAGE_NAME
fi

# Final message
log_success "Build process completed!"
log_info "Image: $FULL_IMAGE_NAME"
log_info "Size: $IMAGE_SIZE"

if [[ "$PUSH_IMAGE" == true ]]; then
    log_info "Status: Built and pushed to registry"
else
    log_info "Status: Built locally"
fi

# Show next steps
echo
log_info "Next steps:"
log_info "  Run locally: docker run -p 8080:8080 $FULL_IMAGE_NAME"
log_info "  Run with compose: docker-compose up -d"
log_info "  Test health check: curl http://localhost:8080/api/v1/health"