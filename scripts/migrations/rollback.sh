#!/bin/bash

# Database Migration Rollback Script
# This script handles rollback of database migrations with safety checks

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"
BACKUP_ENABLED="${BACKUP_ENABLED:-true}"
ROLLBACK_TO="${ROLLBACK_TO:-}"
ROLLBACK_STEPS="${ROLLBACK_STEPS:-1}"

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
    
    log "Retrieving database configuration for environment: $env"
    
    DB_HOST=$(aws ssm get-parameter --name "/${env}/${service_name}/database/endpoint" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    DB_PORT=$(aws ssm get-parameter --name "/${env}/${service_name}/database/port" --query 'Parameter.Value' --output text 2>/dev/null || echo "5432")
    DB_NAME=$(aws ssm get-parameter --name "/${env}/${service_name}/database/name" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    DB_USER=$(aws ssm get-parameter --name "/${env}/${service_name}/database/username" --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    DB_PASSWORD=$(aws ssm get-parameter --name "/${env}/${service_name}/database/password" --with-decryption --query 'Parameter.Value' --output text 2>/dev/null || echo "")
    
    if [[ -z "$DB_HOST" || -z "$DB_NAME" || -z "$DB_USER" || -z "$DB_PASSWORD" ]]; then
        error "Failed to retrieve database configuration from SSM"
    fi
    
    export PGPASSWORD="$DB_PASSWORD"
    
    log "Database configuration loaded successfully"
}

# Test database connectivity
test_db_connectivity() {
    log "Testing database connectivity"
    
    if psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -c "SELECT 1;" >/dev/null 2>&1; then
        log "Database connectivity test passed"
        return 0
    else
        error "Database connectivity test failed"
    fi
}

# Get applied migrations in reverse order
get_applied_migrations_reverse() {
    psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "
        SELECT migration_name, applied_at, rollback_sql, checksum 
        FROM schema_migrations 
        ORDER BY applied_at DESC, id DESC;
    " 2>/dev/null | sed 's/^[ \t]*//' | grep -v '^$' || echo ""
}

# Get migration details
get_migration_details() {
    local migration_name="$1"
    
    psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "
        SELECT migration_name, applied_at, rollback_sql, checksum 
        FROM schema_migrations 
        WHERE migration_name = '$migration_name';
    " 2>/dev/null | sed 's/^[ \t]*//' | grep -v '^$' || echo ""
}

# Check if migration has rollback SQL
has_rollback_sql() {
    local migration_name="$1"
    
    local rollback_sql
    rollback_sql=$(psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "
        SELECT rollback_sql FROM schema_migrations WHERE migration_name = '$migration_name';
    " 2>/dev/null | sed 's/^[ \t]*//' | grep -v '^$' || echo "")
    
    [[ -n "$rollback_sql" && "$rollback_sql" != "null" ]]
}

# Execute rollback for a single migration
rollback_single_migration() {
    local migration_name="$1"
    
    log "Rolling back migration: $migration_name"
    
    # Get rollback SQL
    local rollback_sql
    rollback_sql=$(psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "
        SELECT rollback_sql FROM schema_migrations WHERE migration_name = '$migration_name';
    " 2>/dev/null | sed 's/^[ \t]*//' | grep -v '^$' || echo "")
    
    if [[ -z "$rollback_sql" || "$rollback_sql" == "null" ]]; then
        error "No rollback SQL found for migration: $migration_name"
    fi
    
    # Validate rollback SQL
    if [[ ${#rollback_sql} -lt 10 ]]; then
        error "Rollback SQL too short, may be invalid: $migration_name"
    fi
    
    log "Rollback SQL preview for $migration_name:"
    echo "$rollback_sql" | head -n 5 | while read -r line; do
        log "  $line"
    done
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log "DRY RUN: Would rollback migration $migration_name"
        return 0
    fi
    
    # Create backup before rollback
    if [[ "$BACKUP_ENABLED" == "true" ]]; then
        log "Creating backup before rollback"
        "$SCRIPT_DIR/backup-database.sh" --pre-rollback
    fi
    
    # Execute rollback in transaction
    local transaction_sql="
    BEGIN;
    
    -- Execute rollback SQL
    $rollback_sql
    
    -- Remove migration record
    DELETE FROM schema_migrations WHERE migration_name = '$migration_name';
    
    COMMIT;
    "
    
    if psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -c "$transaction_sql" 2>/dev/null; then
        log "Migration rolled back successfully: $migration_name"
        return 0
    else
        error "Failed to rollback migration: $migration_name"
    fi
}

# Rollback to a specific migration
rollback_to_migration() {
    local target_migration="$1"
    local env="$2"
    
    log "Rolling back to migration: $target_migration"
    
    # Get database configuration
    get_db_config "$env"
    
    # Test connectivity
    test_db_connectivity
    
    # Get applied migrations
    local applied_migrations
    applied_migrations=$(get_applied_migrations_reverse)
    
    if [[ -z "$applied_migrations" ]]; then
        log "No migrations to rollback"
        return 0
    fi
    
    # Find migrations to rollback
    local migrations_to_rollback=()
    local found_target=false
    
    while IFS='|' read -r migration_name applied_at rollback_sql checksum; do
        # Clean up the fields
        migration_name=$(echo "$migration_name" | xargs)
        applied_at=$(echo "$applied_at" | xargs)
        rollback_sql=$(echo "$rollback_sql" | xargs)
        checksum=$(echo "$checksum" | xargs)
        
        if [[ "$migration_name" == "$target_migration" ]]; then
            found_target=true
            break
        fi
        
        migrations_to_rollback+=("$migration_name")
    done <<< "$applied_migrations"
    
    if [[ "$found_target" != true ]]; then
        error "Target migration not found in applied migrations: $target_migration"
    fi
    
    if [[ ${#migrations_to_rollback[@]} -eq 0 ]]; then
        log "No migrations to rollback (already at target)"
        return 0
    fi
    
    log "Migrations to rollback: ${#migrations_to_rollback[@]}"
    for migration in "${migrations_to_rollback[@]}"; do
        log "  - $migration"
    done
    
    # Validate that all migrations have rollback SQL
    local missing_rollback=()
    for migration in "${migrations_to_rollback[@]}"; do
        if ! has_rollback_sql "$migration"; then
            missing_rollback+=("$migration")
        fi
    done
    
    if [[ ${#missing_rollback[@]} -gt 0 ]]; then
        error "The following migrations don't have rollback SQL: ${missing_rollback[*]}"
    fi
    
    # Execute rollbacks
    for migration in "${migrations_to_rollback[@]}"; do
        rollback_single_migration "$migration"
    done
    
    log "Rollback to $target_migration completed successfully"
}

# Rollback specified number of steps
rollback_steps() {
    local steps="$1"
    local env="$2"
    
    log "Rolling back $steps migration(s)"
    
    # Get database configuration
    get_db_config "$env"
    
    # Test connectivity
    test_db_connectivity
    
    # Get applied migrations
    local applied_migrations
    applied_migrations=$(get_applied_migrations_reverse)
    
    if [[ -z "$applied_migrations" ]]; then
        log "No migrations to rollback"
        return 0
    fi
    
    # Get migrations to rollback
    local migrations_to_rollback=()
    local count=0
    
    while IFS='|' read -r migration_name applied_at rollback_sql checksum && [[ $count -lt $steps ]]; do
        # Clean up the fields
        migration_name=$(echo "$migration_name" | xargs)
        
        migrations_to_rollback+=("$migration_name")
        count=$((count + 1))
    done <<< "$applied_migrations"
    
    if [[ ${#migrations_to_rollback[@]} -eq 0 ]]; then
        log "No migrations to rollback"
        return 0
    fi
    
    log "Migrations to rollback: ${#migrations_to_rollback[@]}"
    for migration in "${migrations_to_rollback[@]}"; do
        log "  - $migration"
    done
    
    # Validate that all migrations have rollback SQL
    local missing_rollback=()
    for migration in "${migrations_to_rollback[@]}"; do
        if ! has_rollback_sql "$migration"; then
            missing_rollback+=("$migration")
        fi
    done
    
    if [[ ${#missing_rollback[@]} -gt 0 ]]; then
        error "The following migrations don't have rollback SQL: ${missing_rollback[*]}"
    fi
    
    # Confirm rollback in production
    if [[ "$ENVIRONMENT" == "production" && "$FORCE" != "true" ]]; then
        log "WARNING: This will rollback $steps migration(s) in PRODUCTION"
        log "Migrations to be rolled back:"
        for migration in "${migrations_to_rollback[@]}"; do
            log "  - $migration"
        done
        log "Use --force to proceed with production rollback"
        error "Production rollback requires --force flag"
    fi
    
    # Execute rollbacks
    for migration in "${migrations_to_rollback[@]}"; do
        rollback_single_migration "$migration"
    done
    
    log "Rollback of $steps migration(s) completed successfully"
}

# Show rollback plan
show_rollback_plan() {
    local env="$1"
    
    log "Rollback plan for environment: $env"
    
    # Get database configuration
    get_db_config "$env"
    
    # Test connectivity
    test_db_connectivity
    
    # Get applied migrations
    local applied_migrations
    applied_migrations=$(get_applied_migrations_reverse)
    
    echo "Rollback Plan"
    echo "============="
    echo "Environment: $env"
    echo "Database: $DB_HOST:$DB_PORT/$DB_NAME"
    echo ""
    
    if [[ -z "$applied_migrations" ]]; then
        echo "No migrations to rollback"
        return 0
    fi
    
    echo "Applied Migrations (newest first):"
    local count=0
    while IFS='|' read -r migration_name applied_at rollback_sql checksum; do
        count=$((count + 1))
        
        # Clean up the fields
        migration_name=$(echo "$migration_name" | xargs)
        applied_at=$(echo "$applied_at" | xargs)
        rollback_sql=$(echo "$rollback_sql" | xargs)
        
        local has_rollback="No"
        if [[ -n "$rollback_sql" && "$rollback_sql" != "null" ]]; then
            has_rollback="Yes"
        fi
        
        echo "  $count. $migration_name (applied: $applied_at, rollback: $has_rollback)"
    done <<< "$applied_migrations"
    
    echo ""
    
    if [[ -n "$ROLLBACK_TO" ]]; then
        echo "Rollback target: $ROLLBACK_TO"
    elif [[ -n "$ROLLBACK_STEPS" ]]; then
        echo "Rollback steps: $ROLLBACK_STEPS"
    fi
}

# Validate rollback safety
validate_rollback_safety() {
    local env="$1"
    
    log "Validating rollback safety"
    
    # Check if we're in production
    if [[ "$env" == "production" ]]; then
        log "WARNING: Rollback in production environment"
        
        if [[ "$FORCE" != "true" ]]; then
            error "Production rollback requires --force flag"
        fi
        
        # Additional production safety checks
        local recent_migrations
        recent_migrations=$(psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "
            SELECT COUNT(*) FROM schema_migrations 
            WHERE applied_at > NOW() - INTERVAL '1 hour';
        " 2>/dev/null | xargs || echo "0")
        
        if [[ "$recent_migrations" -gt 0 ]]; then
            log "WARNING: $recent_migrations migration(s) applied in the last hour"
        fi
        
        # Check for active connections
        local active_connections
        active_connections=$(psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "
            SELECT COUNT(*) FROM pg_stat_activity 
            WHERE state = 'active' AND datname = '$DB_NAME';
        " 2>/dev/null | xargs || echo "0")
        
        if [[ "$active_connections" -gt 5 ]]; then
            log "WARNING: $active_connections active database connections"
        fi
    fi
    
    # Check for data integrity
    local table_count
    table_count=$(psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "
        SELECT COUNT(*) FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_type = 'BASE TABLE';
    " 2>/dev/null | xargs || echo "0")
    
    if [[ "$table_count" -eq 0 ]]; then
        log "WARNING: No tables found in database"
    fi
    
    log "Rollback safety validation completed"
}

# Usage information
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --environment ENV      Environment to rollback (default: staging)"
    echo "  --to MIGRATION         Rollback to specific migration"
    echo "  --steps NUMBER         Number of migrations to rollback (default: 1)"
    echo "  --dry-run             Show what would be done without making changes"
    echo "  --force               Force rollback even with warnings"
    echo "  --no-backup           Skip backup creation"
    echo "  --plan                Show rollback plan"
    echo "  --help                Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  ENVIRONMENT           Environment name (default: staging)"
    echo "  DRY_RUN              Enable dry run mode (default: false)"
    echo "  FORCE                Force rollback (default: false)"
    echo "  BACKUP_ENABLED       Enable backup creation (default: true)"
    echo ""
    echo "Examples:"
    echo "  $0 --steps 1 --environment staging"
    echo "  $0 --to 001_initial_schema --environment production --force"
    echo "  $0 --plan --environment production"
    echo "  $0 --dry-run --steps 2"
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --to)
                ROLLBACK_TO="$2"
                shift 2
                ;;
            --steps)
                ROLLBACK_STEPS="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN="true"
                shift
                ;;
            --force)
                FORCE="true"
                shift
                ;;
            --no-backup)
                BACKUP_ENABLED="false"
                shift
                ;;
            --plan)
                show_rollback_plan "$ENVIRONMENT"
                exit 0
                ;;
            --help)
                usage
                exit 0
                ;;
            *)
                error "Unknown parameter: $1"
                ;;
        esac
    done
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
    local required_tools=("aws" "psql")
    
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &>/dev/null; then
            error "$tool not found. Please install $tool."
        fi
    done
}

# Main execution
main() {
    log "Starting database migration rollback process"
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Validate rollback safety
    validate_rollback_safety "$ENVIRONMENT"
    
    # Execute rollback
    if [[ -n "$ROLLBACK_TO" ]]; then
        rollback_to_migration "$ROLLBACK_TO" "$ENVIRONMENT"
    else
        rollback_steps "$ROLLBACK_STEPS" "$ENVIRONMENT"
    fi
    
    log "Database migration rollback process completed successfully"
}

# Execute main function
main "$@"