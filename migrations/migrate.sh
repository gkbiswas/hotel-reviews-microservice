#!/bin/bash

# Hotel Reviews Database Migration Script
# Usage: ./migrate.sh [COMMAND] [OPTIONS]

set -e

# Configuration
DEFAULT_DATABASE_URL="postgresql://postgres:postgres@localhost:5432/hotel_reviews"
MIGRATIONS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCK_FILE="/tmp/hotel_reviews_migration.lock"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DATABASE_URL="${DATABASE_URL:-$DEFAULT_DATABASE_URL}"
DRY_RUN=false
VERBOSE=false
FORCE=false

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

# Print usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  up                    Apply all pending migrations"
    echo "  down [VERSION]        Rollback to specific version (or previous)"
    echo "  status               Show migration status"
    echo "  create [NAME]        Create new migration file"
    echo "  reset                Drop all tables and rerun migrations"
    echo "  validate             Validate migration files"
    echo "  repair               Mark failed migrations as resolved"
    echo ""
    echo "Options:"
    echo "  -d, --database-url URL    Database connection URL"
    echo "  -n, --dry-run             Show what would be done without executing"
    echo "  -v, --verbose             Verbose output"
    echo "  -f, --force               Force operation (skip confirmations)"
    echo "  -h, --help                Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  DATABASE_URL              Database connection URL"
    echo ""
    echo "Examples:"
    echo "  $0 up                     # Apply all pending migrations"
    echo "  $0 down 001               # Rollback to version 001"
    echo "  $0 status                 # Show current migration status"
    echo "  $0 create add_indexes     # Create new migration file"
    echo "  $0 reset --force          # Reset database (destructive)"
}

# Parse command line arguments
COMMAND=""
while [[ $# -gt 0 ]]; do
    case $1 in
        up|down|status|create|reset|validate|repair)
            COMMAND="$1"
            shift
            ;;
        -d|--database-url)
            DATABASE_URL="$2"
            shift 2
            ;;
        -n|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            if [[ -z "$COMMAND" ]]; then
                log_error "Unknown command: $1"
                usage
                exit 1
            else
                # Store additional arguments for commands that need them
                ARGS+=("$1")
            fi
            shift
            ;;
    esac
done

# Check if command is provided
if [[ -z "$COMMAND" ]]; then
    log_error "No command specified"
    usage
    exit 1
fi

# Check if psql is available
if ! command -v psql &> /dev/null; then
    log_error "psql is not installed or not in PATH"
    exit 1
fi

# Create lock file to prevent concurrent migrations
create_lock() {
    if [[ -f "$LOCK_FILE" ]]; then
        log_error "Another migration is already running (lock file exists: $LOCK_FILE)"
        exit 1
    fi
    
    echo "$$" > "$LOCK_FILE"
    trap 'rm -f "$LOCK_FILE"' EXIT
}

# Execute SQL command
execute_sql() {
    local sql="$1"
    local description="$2"
    
    if [[ "$VERBOSE" == true ]]; then
        log_info "Executing: $description"
        if [[ "$DRY_RUN" != true ]]; then
            echo "$sql"
        fi
    fi
    
    if [[ "$DRY_RUN" != true ]]; then
        psql "$DATABASE_URL" -c "$sql" || {
            log_error "Failed to execute: $description"
            exit 1
        }
    else
        log_info "DRY RUN: Would execute: $description"
    fi
}

# Execute SQL file
execute_sql_file() {
    local file="$1"
    local description="$2"
    
    if [[ ! -f "$file" ]]; then
        log_error "Migration file not found: $file"
        exit 1
    fi
    
    if [[ "$VERBOSE" == true ]]; then
        log_info "Executing file: $file"
        log_info "Description: $description"
    fi
    
    if [[ "$DRY_RUN" != true ]]; then
        psql "$DATABASE_URL" -f "$file" || {
            log_error "Failed to execute migration file: $file"
            exit 1
        }
    else
        log_info "DRY RUN: Would execute file: $file"
    fi
}

# Initialize migration tracking table
init_migration_table() {
    local sql="
    CREATE TABLE IF NOT EXISTS schema_migrations (
        version VARCHAR(255) PRIMARY KEY,
        applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
        execution_time_ms INTEGER,
        checksum VARCHAR(64)
    );
    
    CREATE INDEX IF NOT EXISTS idx_schema_migrations_applied_at 
    ON schema_migrations(applied_at);
    "
    
    execute_sql "$sql" "Initialize migration tracking table"
}

# Get applied migrations
get_applied_migrations() {
    psql "$DATABASE_URL" -t -c "SELECT version FROM schema_migrations ORDER BY version;" 2>/dev/null | grep -v '^$' | sed 's/^ *//' || true
}

# Get available migrations
get_available_migrations() {
    find "$MIGRATIONS_DIR" -name "*.sql" -not -name "*_rollback.sql" | sort | while read -r file; do
        basename "$file" .sql
    done
}

# Get pending migrations
get_pending_migrations() {
    local applied_migrations
    applied_migrations=$(get_applied_migrations)
    
    get_available_migrations | while read -r migration; do
        if ! echo "$applied_migrations" | grep -q "^$migration$"; then
            echo "$migration"
        fi
    done
}

# Calculate file checksum
calculate_checksum() {
    local file="$1"
    if [[ -f "$file" ]]; then
        sha256sum "$file" | cut -d' ' -f1
    else
        echo ""
    fi
}

# Command implementations
cmd_status() {
    log_info "Migration Status"
    echo "=================="
    
    # Check if database is accessible
    if ! psql "$DATABASE_URL" -c "SELECT 1;" >/dev/null 2>&1; then
        log_error "Cannot connect to database: $DATABASE_URL"
        exit 1
    fi
    
    # Initialize migration table if needed
    init_migration_table
    
    local applied_migrations
    applied_migrations=$(get_applied_migrations)
    
    local pending_migrations
    pending_migrations=$(get_pending_migrations)
    
    echo ""
    echo "Applied migrations:"
    if [[ -n "$applied_migrations" ]]; then
        echo "$applied_migrations" | while read -r migration; do
            echo "  ✓ $migration"
        done
    else
        echo "  (none)"
    fi
    
    echo ""
    echo "Pending migrations:"
    if [[ -n "$pending_migrations" ]]; then
        echo "$pending_migrations" | while read -r migration; do
            echo "  ○ $migration"
        done
    else
        echo "  (none)"
    fi
    
    echo ""
    local total_applied
    total_applied=$(echo "$applied_migrations" | wc -l)
    if [[ -z "$applied_migrations" ]]; then
        total_applied=0
    fi
    
    local total_pending
    total_pending=$(echo "$pending_migrations" | wc -l)
    if [[ -z "$pending_migrations" ]]; then
        total_pending=0
    fi
    
    echo "Summary: $total_applied applied, $total_pending pending"
}

cmd_up() {
    log_info "Applying pending migrations..."
    
    create_lock
    init_migration_table
    
    local pending_migrations
    pending_migrations=$(get_pending_migrations)
    
    if [[ -z "$pending_migrations" ]]; then
        log_success "No pending migrations to apply"
        return 0
    fi
    
    echo "$pending_migrations" | while read -r migration; do
        local migration_file="$MIGRATIONS_DIR/${migration}.sql"
        local start_time=$(date +%s%3N)
        
        log_info "Applying migration: $migration"
        
        # Calculate checksum
        local checksum
        checksum=$(calculate_checksum "$migration_file")
        
        # Execute migration
        execute_sql_file "$migration_file" "Migration: $migration"
        
        # Record migration
        local end_time=$(date +%s%3N)
        local execution_time=$((end_time - start_time))
        
        local record_sql="
        INSERT INTO schema_migrations (version, execution_time_ms, checksum) 
        VALUES ('$migration', $execution_time, '$checksum')
        ON CONFLICT (version) DO UPDATE SET 
            applied_at = CURRENT_TIMESTAMP,
            execution_time_ms = $execution_time,
            checksum = '$checksum';
        "
        
        execute_sql "$record_sql" "Record migration: $migration"
        
        log_success "Applied migration: $migration (${execution_time}ms)"
    done
    
    log_success "All migrations applied successfully"
}

cmd_down() {
    local target_version="${ARGS[0]}"
    
    if [[ -z "$target_version" ]]; then
        log_error "Target version is required for rollback"
        exit 1
    fi
    
    log_warning "Rolling back to version: $target_version"
    
    if [[ "$FORCE" != true ]]; then
        echo -n "Are you sure you want to rollback? This may result in data loss. [y/N]: "
        read -r confirm
        if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
            log_info "Rollback cancelled"
            exit 0
        fi
    fi
    
    create_lock
    init_migration_table
    
    local applied_migrations
    applied_migrations=$(get_applied_migrations)
    
    # Find migrations to rollback (in reverse order)
    local migrations_to_rollback
    migrations_to_rollback=$(echo "$applied_migrations" | grep -A 999 "^$target_version$" | tail -n +2 | tac)
    
    if [[ -z "$migrations_to_rollback" ]]; then
        log_info "No migrations to rollback"
        return 0
    fi
    
    echo "$migrations_to_rollback" | while read -r migration; do
        local rollback_file="$MIGRATIONS_DIR/${migration}_rollback.sql"
        
        if [[ ! -f "$rollback_file" ]]; then
            log_error "Rollback file not found: $rollback_file"
            exit 1
        fi
        
        log_info "Rolling back migration: $migration"
        
        # Execute rollback
        execute_sql_file "$rollback_file" "Rollback: $migration"
        
        # Remove migration record
        local remove_sql="DELETE FROM schema_migrations WHERE version = '$migration';"
        execute_sql "$remove_sql" "Remove migration record: $migration"
        
        log_success "Rolled back migration: $migration"
    done
    
    log_success "Rollback completed successfully"
}

cmd_create() {
    local migration_name="${ARGS[0]}"
    
    if [[ -z "$migration_name" ]]; then
        log_error "Migration name is required"
        exit 1
    fi
    
    # Generate migration number
    local timestamp=$(date +%Y%m%d%H%M%S)
    local migration_number
    migration_number=$(find "$MIGRATIONS_DIR" -name "*.sql" | wc -l | xargs printf "%03d")
    
    local migration_filename="${migration_number}_${migration_name}.sql"
    local rollback_filename="${migration_number}_${migration_name}_rollback.sql"
    
    local migration_file="$MIGRATIONS_DIR/$migration_filename"
    local rollback_file="$MIGRATIONS_DIR/$rollback_filename"
    
    # Create migration file
    cat > "$migration_file" << EOF
-- ============================================================================
-- Migration: $migration_filename
-- Description: TODO: Add description
-- Version: 1.0.0
-- Created: $(date +%Y-%m-%d)
-- ============================================================================

-- TODO: Add your migration SQL here

-- ============================================================================
-- MIGRATION COMPLETION
-- ============================================================================

-- Insert migration record
INSERT INTO schema_migrations (version) VALUES ('${migration_number}_${migration_name}')
ON CONFLICT (version) DO NOTHING;

-- Final message
DO \$\$
BEGIN
    RAISE NOTICE 'Migration ${migration_number}_${migration_name} completed successfully';
END \$\$;
EOF

    # Create rollback file
    cat > "$rollback_file" << EOF
-- ============================================================================
-- Migration Rollback: $rollback_filename
-- Description: Rollback for $migration_name
-- Version: 1.0.0
-- Created: $(date +%Y-%m-%d)
-- ============================================================================

-- TODO: Add your rollback SQL here

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '${migration_number}_${migration_name}';

-- Final message
DO \$\$
BEGIN
    RAISE NOTICE 'Migration ${migration_number}_${migration_name} rollback completed successfully';
END \$\$;
EOF

    log_success "Created migration files:"
    log_info "  Up:   $migration_file"
    log_info "  Down: $rollback_file"
}

cmd_reset() {
    log_warning "This will DROP ALL TABLES and rerun all migrations"
    log_warning "ALL DATA WILL BE LOST!"
    
    if [[ "$FORCE" != true ]]; then
        echo -n "Are you sure you want to reset the database? [y/N]: "
        read -r confirm
        if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
            log_info "Reset cancelled"
            exit 0
        fi
    fi
    
    create_lock
    
    # Drop all tables
    local drop_sql="
    DROP SCHEMA public CASCADE;
    CREATE SCHEMA public;
    GRANT ALL ON SCHEMA public TO postgres;
    GRANT ALL ON SCHEMA public TO public;
    "
    
    execute_sql "$drop_sql" "Drop all tables"
    
    # Rerun all migrations
    cmd_up
    
    log_success "Database reset completed"
}

cmd_validate() {
    log_info "Validating migration files..."
    
    local errors=0
    
    # Check for missing rollback files
    get_available_migrations | while read -r migration; do
        local migration_file="$MIGRATIONS_DIR/${migration}.sql"
        local rollback_file="$MIGRATIONS_DIR/${migration}_rollback.sql"
        
        if [[ ! -f "$rollback_file" ]]; then
            log_warning "Missing rollback file: $rollback_file"
            ((errors++))
        fi
        
        # Check SQL syntax (basic check)
        if ! psql "$DATABASE_URL" --set ON_ERROR_STOP=1 -f "$migration_file" --dry-run >/dev/null 2>&1; then
            log_error "SQL syntax error in: $migration_file"
            ((errors++))
        fi
    done
    
    if [[ $errors -eq 0 ]]; then
        log_success "All migration files are valid"
    else
        log_error "Found $errors validation errors"
        exit 1
    fi
}

cmd_repair() {
    log_info "Repairing migration state..."
    
    # TODO: Implement repair logic
    # This could include:
    # - Marking failed migrations as resolved
    # - Fixing checksum mismatches
    # - Synchronizing migration state
    
    log_info "Repair functionality not implemented yet"
}

# Main execution
main() {
    if [[ "$VERBOSE" == true ]]; then
        log_info "Database URL: $DATABASE_URL"
        log_info "Migrations directory: $MIGRATIONS_DIR"
        log_info "Dry run: $DRY_RUN"
        log_info "Force: $FORCE"
        echo
    fi
    
    case "$COMMAND" in
        up)
            cmd_up
            ;;
        down)
            cmd_down
            ;;
        status)
            cmd_status
            ;;
        create)
            cmd_create
            ;;
        reset)
            cmd_reset
            ;;
        validate)
            cmd_validate
            ;;
        repair)
            cmd_repair
            ;;
        *)
            log_error "Unknown command: $COMMAND"
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"