#!/bin/bash

# Database Health Check Script
# This script checks database connectivity and basic health metrics

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
TIMEOUT="${TIMEOUT:-30}"
MAX_RETRIES="${MAX_RETRIES:-5}"
RETRY_DELAY="${RETRY_DELAY:-10}"

# Logging
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
}

error() {
    log "ERROR: $*"
    exit 1
}

# Get database configuration from SSM
get_db_config() {
    local env="$1"
    local service_name="hotel-reviews-service"
    
    DB_HOST=$(aws ssm get-parameter --name "/${env}/${service_name}/database/endpoint" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    DB_PORT=$(aws ssm get-parameter --name "/${env}/${service_name}/database/port" --query 'Parameter.Value' --output text 2>/dev/null || echo "5432")
    DB_NAME=$(aws ssm get-parameter --name "/${env}/${service_name}/database/name" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    DB_USER=$(aws ssm get-parameter --name "/${env}/${service_name}/database/username" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    DB_PASSWORD=$(aws ssm get-parameter --name "/${env}/${service_name}/database/password" --with-decryption --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    
    if [[ -z "$DB_HOST" || -z "$DB_NAME" || -z "$DB_USER" || -z "$DB_PASSWORD" ]]; then
        error "Failed to retrieve database configuration from SSM"
    fi
}

# Test database connectivity
test_connectivity() {
    local host="$1"
    local port="$2"
    local timeout="$3"
    
    log "Testing database connectivity to $host:$port"
    
    if timeout "$timeout" bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null; then
        log "Database connectivity test passed"
        return 0
    else
        log "Database connectivity test failed"
        return 1
    fi
}

# Test database authentication
test_authentication() {
    local host="$1"
    local port="$2"
    local dbname="$3"
    local user="$4"
    local password="$5"
    local timeout="$6"
    
    log "Testing database authentication"
    
    export PGPASSWORD="$password"
    
    if timeout "$timeout" psql -h "$host" -p "$port" -d "$dbname" -U "$user" -c "SELECT 1;" >/dev/null 2>&1; then
        log "Database authentication test passed"
        return 0
    else
        log "Database authentication test failed"
        return 1
    fi
}

# Check database health metrics
check_health_metrics() {
    local host="$1"
    local port="$2"
    local dbname="$3"
    local user="$4"
    local password="$5"
    
    log "Checking database health metrics"
    
    export PGPASSWORD="$password"
    
    # Check database size
    local db_size
    db_size=$(psql -h "$host" -p "$port" -d "$dbname" -U "$user" -t -c "SELECT pg_size_pretty(pg_database_size('$dbname'));" 2>/dev/null | xargs || echo "unknown")
    log "Database size: $db_size"
    
    # Check active connections
    local active_connections
    active_connections=$(psql -h "$host" -p "$port" -d "$dbname" -U "$user" -t -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'active';" 2>/dev/null | xargs || echo "unknown")
    log "Active connections: $active_connections"
    
    # Check for long-running queries
    local long_queries
    long_queries=$(psql -h "$host" -p "$port" -d "$dbname" -U "$user" -t -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'active' AND now() - query_start > interval '5 minutes';" 2>/dev/null | xargs || echo "unknown")
    log "Long-running queries (>5min): $long_queries"
    
    # Check for blocked queries
    local blocked_queries
    blocked_queries=$(psql -h "$host" -p "$port" -d "$dbname" -U "$user" -t -c "SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock';" 2>/dev/null | xargs || echo "unknown")
    log "Blocked queries: $blocked_queries"
    
    # Check replication lag (if applicable)
    local replication_lag
    replication_lag=$(psql -h "$host" -p "$port" -d "$dbname" -U "$user" -t -c "SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) AS replication_lag;" 2>/dev/null | xargs || echo "N/A")
    log "Replication lag: $replication_lag seconds"
    
    # Validate critical metrics
    if [[ "$active_connections" != "unknown" && "$active_connections" -gt 80 ]]; then
        log "WARNING: High number of active connections: $active_connections"
    fi
    
    if [[ "$long_queries" != "unknown" && "$long_queries" -gt 0 ]]; then
        log "WARNING: Long-running queries detected: $long_queries"
    fi
    
    if [[ "$blocked_queries" != "unknown" && "$blocked_queries" -gt 0 ]]; then
        log "WARNING: Blocked queries detected: $blocked_queries"
    fi
    
    if [[ "$replication_lag" != "N/A" && "$replication_lag" != "unknown" ]]; then
        if (( $(echo "$replication_lag > 60" | bc -l) )); then
            log "WARNING: High replication lag: $replication_lag seconds"
        fi
    fi
}

# Check required tables exist
check_required_tables() {
    local host="$1"
    local port="$2"
    local dbname="$3"
    local user="$4"
    local password="$5"
    
    log "Checking required tables exist"
    
    export PGPASSWORD="$password"
    
    local required_tables=("hotel_reviews" "schema_migrations" "users" "hotels")
    
    for table in "${required_tables[@]}"; do
        if psql -h "$host" -p "$port" -d "$dbname" -U "$user" -t -c "SELECT 1 FROM information_schema.tables WHERE table_name = '$table';" 2>/dev/null | grep -q 1; then
            log "Table '$table' exists"
        else
            log "WARNING: Table '$table' does not exist"
        fi
    done
}

# Run database health check with retries
run_health_check() {
    local env="$1"
    local retry_count=0
    
    while [[ $retry_count -lt $MAX_RETRIES ]]; do
        log "Database health check attempt $((retry_count + 1))/$MAX_RETRIES"
        
        # Get database configuration
        get_db_config "$env"
        
        # Test connectivity
        if test_connectivity "$DB_HOST" "$DB_PORT" "$TIMEOUT"; then
            # Test authentication
            if test_authentication "$DB_HOST" "$DB_PORT" "$DB_NAME" "$DB_USER" "$DB_PASSWORD" "$TIMEOUT"; then
                # Check health metrics
                check_health_metrics "$DB_HOST" "$DB_PORT" "$DB_NAME" "$DB_USER" "$DB_PASSWORD"
                
                # Check required tables
                check_required_tables "$DB_HOST" "$DB_PORT" "$DB_NAME" "$DB_USER" "$DB_PASSWORD"
                
                log "Database health check completed successfully"
                return 0
            fi
        fi
        
        retry_count=$((retry_count + 1))
        if [[ $retry_count -lt $MAX_RETRIES ]]; then
            log "Health check failed, retrying in ${RETRY_DELAY} seconds..."
            sleep "$RETRY_DELAY"
        fi
    done
    
    error "Database health check failed after $MAX_RETRIES attempts"
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
    
    # Check if psql is available
    if ! command -v psql &>/dev/null; then
        error "PostgreSQL client (psql) not found. Please install PostgreSQL client."
    fi
    
    # Check if bc is available (for numeric comparisons)
    if ! command -v bc &>/dev/null; then
        error "bc (basic calculator) not found. Please install bc."
    fi
}

# Main execution
main() {
    log "Starting database health check for environment: $ENVIRONMENT"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Run health check
    run_health_check "$ENVIRONMENT"
    
    log "Database health check completed successfully"
}

# Execute main function
main "$@"