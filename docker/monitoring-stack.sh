#!/bin/bash

# Hotel Reviews Monitoring Stack Management Script
# This script helps manage the complete monitoring stack including Prometheus, Grafana, Jaeger, and AlertManager

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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
    echo -e "${BLUE}[MONITORING]${NC} $1"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
    print_status "Docker is running"
}

# Function to check if Docker Compose is available
check_docker_compose() {
    if ! command -v docker-compose > /dev/null 2>&1; then
        print_error "Docker Compose is not installed. Please install Docker Compose and try again."
        exit 1
    fi
    print_status "Docker Compose is available"
}

# Function to start the monitoring stack
start_monitoring() {
    print_header "Starting monitoring stack..."
    
    # Start the core services first
    print_status "Starting core services (PostgreSQL, Redis, MinIO)..."
    docker-compose up -d postgres redis minio minio-client
    
    # Wait a bit for core services to be ready
    print_status "Waiting for core services to be ready..."
    sleep 10
    
    # Start the application
    print_status "Starting application service..."
    docker-compose up -d hotel-reviews-api
    
    # Wait for application to be ready
    print_status "Waiting for application to be ready..."
    sleep 15
    
    # Start monitoring services
    print_status "Starting monitoring services..."
    docker-compose --profile monitoring up -d
    
    print_status "Monitoring stack started successfully!"
    print_services_info
}

# Function to stop the monitoring stack
stop_monitoring() {
    print_header "Stopping monitoring stack..."
    docker-compose --profile monitoring down
    docker-compose down
    print_status "Monitoring stack stopped"
}

# Function to restart the monitoring stack
restart_monitoring() {
    print_header "Restarting monitoring stack..."
    stop_monitoring
    sleep 5
    start_monitoring
}

# Function to show service information
print_services_info() {
    print_header "Monitoring Services Information:"
    echo ""
    echo -e "${GREEN}Application Services:${NC}"
    echo "  • Hotel Reviews API: http://localhost:8080"
    echo "  • API Health Check: http://localhost:8080/api/v1/health"
    echo "  • API Metrics: http://localhost:8080/api/v1/metrics"
    echo ""
    echo -e "${GREEN}Monitoring Services:${NC}"
    echo "  • Prometheus: http://localhost:9090"
    echo "  • Grafana: http://localhost:3000 (admin/admin)"
    echo "  • AlertManager: http://localhost:9093"
    echo "  • Jaeger: http://localhost:16686"
    echo ""
    echo -e "${GREEN}Infrastructure Services:${NC}"
    echo "  • PostgreSQL: localhost:5432"
    echo "  • Redis: localhost:6379"
    echo "  • MinIO: http://localhost:9001 (minioadmin/minioadmin)"
    echo ""
    echo -e "${GREEN}Exporters:${NC}"
    echo "  • Node Exporter: http://localhost:9100"
    echo "  • PostgreSQL Exporter: http://localhost:9187"
    echo "  • Redis Exporter: http://localhost:9121"
    echo "  • Nginx Exporter: http://localhost:9113"
    echo ""
}

# Function to check service health
check_health() {
    print_header "Checking service health..."
    
    services=(
        "hotel-reviews-api:8080/api/v1/health:Application"
        "prometheus:9090/-/healthy:Prometheus"
        "grafana:3000/api/health:Grafana"
        "alertmanager:9093/-/healthy:AlertManager"
        "jaeger:16686:Jaeger"
    )
    
    for service in "${services[@]}"; do
        IFS=':' read -r container port path name <<< "$service"
        if [ -z "$path" ]; then
            path=""
        fi
        
        if docker exec -it "hotel-reviews-$container" 2>/dev/null curl -f "http://localhost:$port$path" > /dev/null 2>&1; then
            print_status "$name is healthy"
        else
            print_warning "$name health check failed"
        fi
    done
}

# Function to show logs
show_logs() {
    local service="$1"
    if [ -z "$service" ]; then
        print_header "Available services for logs:"
        docker-compose ps --services
        echo ""
        print_warning "Usage: $0 logs <service-name>"
        return 1
    fi
    
    print_header "Showing logs for $service..."
    docker-compose logs -f "$service"
}

# Function to show resource usage
show_stats() {
    print_header "Container resource usage:"
    docker stats --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"
}

# Function to backup monitoring data
backup_data() {
    print_header "Creating backup of monitoring data..."
    
    backup_dir="./backups/$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$backup_dir"
    
    # Backup Prometheus data
    print_status "Backing up Prometheus data..."
    docker run --rm -v "$(pwd)/backups:/backup" -v "hotel-reviews_prometheus_data:/data" alpine tar czf "/backup/prometheus_$(date +%Y%m%d_%H%M%S).tar.gz" -C /data .
    
    # Backup Grafana data
    print_status "Backing up Grafana data..."
    docker run --rm -v "$(pwd)/backups:/backup" -v "hotel-reviews_grafana_data:/data" alpine tar czf "/backup/grafana_$(date +%Y%m%d_%H%M%S).tar.gz" -C /data .
    
    print_status "Backup completed in $backup_dir"
}

# Function to update configurations
update_configs() {
    print_header "Updating monitoring configurations..."
    
    # Reload Prometheus configuration
    print_status "Reloading Prometheus configuration..."
    curl -X POST http://localhost:9090/-/reload || print_warning "Failed to reload Prometheus config"
    
    print_status "Configuration update completed"
}

# Function to generate load for testing
generate_load() {
    print_header "Generating test load..."
    print_status "Sending test requests to the API..."
    
    for i in {1..100}; do
        curl -s http://localhost:8080/api/v1/health > /dev/null || true
        curl -s http://localhost:8080/api/v1/metrics > /dev/null || true
        sleep 0.1
    done
    
    print_status "Load generation completed"
}

# Function to show help
show_help() {
    echo "Hotel Reviews Monitoring Stack Management"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  start       Start the monitoring stack"
    echo "  stop        Stop the monitoring stack"
    echo "  restart     Restart the monitoring stack"
    echo "  status      Show service information and URLs"
    echo "  health      Check health of all services"
    echo "  logs        Show logs for a specific service"
    echo "  stats       Show container resource usage"
    echo "  backup      Backup monitoring data"
    echo "  update      Update and reload configurations"
    echo "  load        Generate test load for monitoring"
    echo "  help        Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  $0 logs prometheus"
    echo "  $0 health"
    echo ""
}

# Main script logic
main() {
    check_docker
    check_docker_compose
    
    case "${1:-}" in
        start)
            start_monitoring
            ;;
        stop)
            stop_monitoring
            ;;
        restart)
            restart_monitoring
            ;;
        status)
            print_services_info
            ;;
        health)
            check_health
            ;;
        logs)
            show_logs "$2"
            ;;
        stats)
            show_stats
            ;;
        backup)
            backup_data
            ;;
        update)
            update_configs
            ;;
        load)
            generate_load
            ;;
        help|--help|-h)
            show_help
            ;;
        "")
            print_error "No command specified"
            show_help
            exit 1
            ;;
        *)
            print_error "Unknown command: $1"
            show_help
            exit 1
            ;;
    esac
}

# Run the main function with all arguments
main "$@"