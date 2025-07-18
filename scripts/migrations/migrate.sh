#!/bin/bash

# Database Migration Script with Rollback Capabilities
# This script handles database schema migrations with full rollback support

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIGRATIONS_DIR="$PROJECT_ROOT/migrations"
ENVIRONMENT="${ENVIRONMENT:-staging}"
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"
BACKUP_ENABLED="${BACKUP_ENABLED:-true}"

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

# Create database connection string
get_db_connection_string() {
    echo "postgresql://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME"
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

# Initialize schema migrations table
initialize_migrations_table() {
    log "Initializing schema migrations table"
    
    local sql="
    CREATE TABLE IF NOT EXISTS schema_migrations (
        id SERIAL PRIMARY KEY,
        migration_name VARCHAR(255) NOT NULL UNIQUE,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        rollback_sql TEXT,
        checksum VARCHAR(64)
    );
    
    CREATE INDEX IF NOT EXISTS idx_schema_migrations_name ON schema_migrations(migration_name);
    CREATE INDEX IF NOT EXISTS idx_schema_migrations_applied_at ON schema_migrations(applied_at);
    "
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log "DRY RUN: Would create schema_migrations table"
    else
        psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -c "$sql"
        log "Schema migrations table initialized"
    fi
}

# Get list of applied migrations
get_applied_migrations() {
    psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "SELECT migration_name FROM schema_migrations ORDER BY applied_at;" 2>/dev/null | sed 's/^[ \t]*//' | grep -v '^$' || echo ""
}

# Get migration files
get_migration_files() {
    if [[ ! -d "$MIGRATIONS_DIR" ]]; then
        error "Migrations directory not found: $MIGRATIONS_DIR"
    fi
    
    find "$MIGRATIONS_DIR" -name "*.sql" -not -name "*_rollback.sql" | sort
}

# Calculate file checksum
calculate_checksum() {
    local file="$1"
    sha256sum "$file" | cut -d' ' -f1
}

# Get rollback SQL file
get_rollback_file() {
    local migration_file="$1"
    local rollback_file="${migration_file%.*}_rollback.sql"
    
    if [[ -f "$rollback_file" ]]; then
        echo "$rollback_file"
    else
        echo ""
    fi
}

# Parse migration file for rollback SQL
parse_rollback_sql() {
    local migration_file="$1"
    
    # Look for rollback SQL in comments
    local rollback_sql=""
    local in_rollback_section=false
    
    while IFS= read -r line; do
        if [[ "$line" =~ ^--\ *ROLLBACK\ *START.*$ ]]; then
            in_rollback_section=true
            continue
        fi
        
        if [[ "$line" =~ ^--\ *ROLLBACK\ *END.*$ ]]; then
            in_rollback_section=false
            continue
        fi
        
        if [[ "$in_rollback_section" == true ]]; then
            if [[ "$line" =~ ^--\ *(.*)$ ]]; then
                rollback_sql="$rollback_sql${BASH_REMATCH[1]}"$'\n'
            fi
        fi
    done < "$migration_file"
    
    echo "$rollback_sql"
}

# Apply a single migration
apply_migration() {
    local migration_file="$1"
    local migration_name
    migration_name=$(basename "$migration_file" .sql)
    
    log "Applying migration: $migration_name"
    
    # Calculate checksum
    local checksum
    checksum=$(calculate_checksum "$migration_file")
    
    # Get rollback SQL
    local rollback_sql=""
    local rollback_file
    rollback_file=$(get_rollback_file "$migration_file")
    
    if [[ -f "$rollback_file" ]]; then
        rollback_sql=$(cat "$rollback_file")
        log "Rollback SQL loaded from: $rollback_file"
    else
        # Try to parse rollback SQL from migration file
        rollback_sql=$(parse_rollback_sql "$migration_file")
        if [[ -n "$rollback_sql" ]]; then
            log "Rollback SQL parsed from migration file"
        else
            log "WARNING: No rollback SQL found for migration: $migration_name"
        fi
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log "DRY RUN: Would apply migration $migration_name"
        log "DRY RUN: Migration content preview:"
        head -n 10 "$migration_file" | while read -r line; do
            log "  $line"
        done
        return 0
    fi
    
    # Start transaction
    local transaction_sql="
    BEGIN;
    
    -- Apply migration
    $(cat "$migration_file")
    
    -- Record migration
    INSERT INTO schema_migrations (migration_name, rollback_sql, checksum) 
    VALUES ('$migration_name', \$rollback\$$rollback_sql\$rollback\$, '$checksum');
    
    COMMIT;
    "
    
    # Apply migration with error handling
    if psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -c "$transaction_sql" 2>/dev/null; then
        log "Migration applied successfully: $migration_name"
        
        # Log migration details
        log "Migration details:"
        log "  File: $migration_file"
        log "  Checksum: $checksum"
        log "  Rollback available: $([ -n "$rollback_sql" ] && echo "Yes" || echo "No")"
        
        return 0
    else
        error "Failed to apply migration: $migration_name"
    fi
}

# Check if migration is already applied
is_migration_applied() {
    local migration_name="$1"
    local applied_migrations="$2"
    
    echo "$applied_migrations" | grep -q "^$migration_name$"
}

# Validate migration integrity
validate_migration() {
    local migration_file="$1"
    local migration_name
    migration_name=$(basename "$migration_file" .sql)
    
    log "Validating migration: $migration_name"
    
    # Check if migration file exists
    if [[ ! -f "$migration_file" ]]; then
        error "Migration file not found: $migration_file"
    fi
    
    # Check if migration file is readable
    if [[ ! -r "$migration_file" ]]; then
        error "Migration file not readable: $migration_file"
    fi
    
    # Check for basic SQL syntax
    if ! grep -q ";" "$migration_file"; then
        log "WARNING: Migration file may not contain valid SQL: $migration_file"
    fi
    
    # Check for dangerous operations in production
    if [[ "$ENVIRONMENT" == "production" ]]; then
        local dangerous_operations=("DROP TABLE" "DROP DATABASE" "TRUNCATE" "DELETE FROM")
        
        for operation in "${dangerous_operations[@]}"; do
            if grep -qi "$operation" "$migration_file"; then
                log "WARNING: Dangerous operation detected in production migration: $operation"
                if [[ "$FORCE" != "true" ]]; then
                    error "Dangerous operation detected in production. Use --force to override."
                fi
            fi
        done
    fi
    
    log "Migration validation passed: $migration_name"
}

# Run migrations
run_migrations() {
    local env="$1"
    
    log "Starting database migrations for environment: $env"
    
    # Get database configuration
    get_db_config "$env"
    
    # Test connectivity
    test_db_connectivity
    
    # Initialize migrations table
    initialize_migrations_table
    
    # Get applied migrations
    local applied_migrations
    applied_migrations=$(get_applied_migrations)
    
    log "Applied migrations:"
    if [[ -n "$applied_migrations" ]]; then
        echo "$applied_migrations" | while read -r migration; do
            log "  - $migration"
        done
    else
        log "  No migrations applied yet"
    fi
    
    # Get migration files
    local migration_files
    migration_files=$(get_migration_files)
    
    if [[ -z "$migration_files" ]]; then
        log "No migration files found"
        return 0
    fi
    
    log "Available migration files:"
    echo "$migration_files" | while read -r file; do
        log "  - $(basename "$file")"
    done
    
    # Apply pending migrations
    local pending_migrations=()
    while IFS= read -r migration_file; do
        local migration_name
        migration_name=$(basename "$migration_file" .sql)
        
        if ! is_migration_applied "$migration_name" "$applied_migrations"; then
            pending_migrations+=("$migration_file")
        fi
    done <<< "$migration_files"
    
    if [[ ${#pending_migrations[@]} -eq 0 ]]; then
        log "No pending migrations to apply"
        return 0
    fi
    
    log "Pending migrations to apply: ${#pending_migrations[@]}"
    
    # Apply each pending migration
    for migration_file in "${pending_migrations[@]}"; do
        # Validate migration
        validate_migration "$migration_file"
        
        # Create backup before applying migration
        if [[ "$BACKUP_ENABLED" == "true" && "$DRY_RUN" != "true" ]]; then
            log "Creating backup before applying migration"
            "$SCRIPT_DIR/backup-database.sh" --pre-migration
        fi
        
        # Apply migration
        apply_migration "$migration_file"
        
        # Verify migration was applied
        if [[ "$DRY_RUN" != "true" ]]; then
            local new_applied_migrations
            new_applied_migrations=$(get_applied_migrations)
            local migration_name
            migration_name=$(basename "$migration_file" .sql)
            
            if ! is_migration_applied "$migration_name" "$new_applied_migrations"; then
                error "Migration was not recorded as applied: $migration_name"
            fi
        fi
    done
    
    log "All migrations applied successfully"
    
    # Final validation
    if [[ "$DRY_RUN" != "true" ]]; then
        log "Running post-migration validation"
        validate_schema_integrity
    fi
}

# Validate schema integrity
validate_schema_integrity() {
    log "Validating schema integrity"
    
    # Check for required tables
    local required_tables=("hotel_reviews" "schema_migrations")
    
    for table in "${required_tables[@]}"; do
        if psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "SELECT 1 FROM information_schema.tables WHERE table_name = '$table';" 2>/dev/null | grep -q 1; then
            log "Required table exists: $table"
        else
            error "Required table missing: $table"
        fi
    done
    
    # Check for orphaned data
    local orphaned_count
    orphaned_count=$(psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "SELECT COUNT(*) FROM schema_migrations WHERE migration_name NOT IN (SELECT DISTINCT migration_name FROM schema_migrations);" 2>/dev/null | xargs || echo "0")
    
    if [[ "$orphaned_count" -gt 0 ]]; then
        log "WARNING: Found $orphaned_count orphaned migration records"
    fi
    
    log "Schema integrity validation completed"
}

# Show migration status
show_migration_status() {
    local env="$1"
    
    log "Migration status for environment: $env"
    
    # Get database configuration
    get_db_config "$env"
    
    # Test connectivity
    test_db_connectivity
    
    # Get applied migrations
    local applied_migrations
    applied_migrations=$(get_applied_migrations)
    
    # Get migration files
    local migration_files
    migration_files=$(get_migration_files)
    
    echo "Migration Status Report"
    echo "======================"
    echo "Environment: $env"
    echo "Database: $DB_HOST:$DB_PORT/$DB_NAME"
    echo ""
    
    if [[ -n "$migration_files" ]]; then
        echo "Migration Files:"
        while IFS= read -r migration_file; do
            local migration_name
            migration_name=$(basename "$migration_file" .sql)
            
            local status="PENDING"
            if is_migration_applied "$migration_name" "$applied_migrations"; then
                status="APPLIED"
            fi
            
            echo "  $migration_name: $status"
        done <<< "$migration_files"
    else
        echo "No migration files found"
    fi
    
    echo ""
    
    if [[ -n "$applied_migrations" ]]; then
        echo "Applied Migrations:"
        echo "$applied_migrations" | while read -r migration; do
            local applied_at
            applied_at=$(psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -t -c "SELECT applied_at FROM schema_migrations WHERE migration_name = '$migration';" 2>/dev/null | xargs || echo "Unknown")
            echo "  $migration (applied: $applied_at)"
        done
    else
        echo "No migrations applied yet"
    fi
}

# Usage information
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --environment ENV    Environment to deploy to (default: staging)"
    echo "  --dry-run           Show what would be done without making changes"
    echo "  --force             Force migration even with warnings"
    echo "  --no-backup         Skip backup creation"
    echo "  --status            Show migration status"
    echo "  --help              Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  ENVIRONMENT         Environment name (default: staging)"
    echo "  DRY_RUN            Enable dry run mode (default: false)"
    echo "  FORCE              Force migration (default: false)"
    echo "  BACKUP_ENABLED     Enable backup creation (default: true)"
    echo ""
    echo "Examples:"
    echo "  $0 --environment production"
    echo "  $0 --dry-run --environment staging"
    echo "  $0 --status --environment production"
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --environment)
                ENVIRONMENT="$2"
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
            --status)
                show_migration_status "$ENVIRONMENT"
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
    local required_tools=("aws" "psql" "sha256sum")
    
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &>/dev/null; then
            error "$tool not found. Please install $tool."
        fi
    done
    
    # Check if migrations directory exists
    if [[ ! -d "$MIGRATIONS_DIR" ]]; then
        error "Migrations directory not found: $MIGRATIONS_DIR"
    fi
}

# Main execution
main() {
    log "Starting database migration process"
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Run migrations
    run_migrations "$ENVIRONMENT"
    
    log "Database migration process completed successfully"
}

# Execute main function
main "$@"