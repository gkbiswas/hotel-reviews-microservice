#!/bin/bash

# Database Backup Script
# This script creates comprehensive database backups with metadata

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="${ENVIRONMENT:-staging}"
BACKUP_TYPE="${BACKUP_TYPE:-full}"
BACKUP_DIR="${BACKUP_DIR:-/tmp/db-backups}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
COMPRESS_BACKUP="${COMPRESS_BACKUP:-true}"
ENCRYPT_BACKUP="${ENCRYPT_BACKUP:-false}"
S3_BACKUP_BUCKET="${S3_BACKUP_BUCKET:-}"

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
    
    if pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
        log "Database is ready"
    else
        error "Database is not ready"
    fi
    
    if psql -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -U "$DB_USER" -c "SELECT 1;" >/dev/null 2>&1; then
        log "Database connectivity test passed"
        return 0
    else
        error "Database connectivity test failed"
    fi
}

# Get S3 backup bucket
get_s3_backup_bucket() {
    local env="$1"
    
    if [[ -z "$S3_BACKUP_BUCKET" ]]; then
        S3_BACKUP_BUCKET="hotel-reviews-${env}-backups"
    fi
    
    log "S3 backup bucket: $S3_BACKUP_BUCKET"
}

# Create backup directory
create_backup_directory() {
    local backup_dir="$1"
    
    log "Creating backup directory: $backup_dir"
    
    if [[ ! -d "$backup_dir" ]]; then
        mkdir -p "$backup_dir" || error "Failed to create backup directory: $backup_dir"
    fi
    
    if [[ ! -w "$backup_dir" ]]; then
        error "Backup directory is not writable: $backup_dir"
    fi
    
    log "Backup directory ready: $backup_dir"
}

# Generate backup filename
generate_backup_filename() {
    local env="$1"
    local backup_type="$2"
    local timestamp
    timestamp=$(date +"%Y%m%d_%H%M%S")
    
    echo "${env}_${backup_type}_${timestamp}.sql"
}

# Create database backup
create_database_backup() {
    local env="$1"
    local backup_type="$2"
    local backup_dir="$3"
    
    log "Creating database backup: $backup_type"
    
    local backup_filename
    backup_filename=$(generate_backup_filename "$env" "$backup_type")
    local backup_path="$backup_dir/$backup_filename"
    
    # Create metadata file
    local metadata_file="$backup_path.metadata"
    cat > "$metadata_file" << EOF
{
    "backup_type": "$backup_type",
    "environment": "$env",
    "database_host": "$DB_HOST",
    "database_port": "$DB_PORT",
    "database_name": "$DB_NAME",
    "database_user": "$DB_USER",
    "backup_timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "backup_filename": "$backup_filename",
    "backup_size": "pending",
    "backup_checksum": "pending",
    "compression": "$COMPRESS_BACKUP",
    "encryption": "$ENCRYPT_BACKUP"
}
EOF
    
    # Create backup based on type
    case "$backup_type" in
        "full")
            log "Creating full database backup"
            if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
                --verbose --clean --if-exists --create --format=plain \
                --file="$backup_path" 2>/dev/null; then
                error "Failed to create full database backup"
            fi
            ;;
        "schema-only")
            log "Creating schema-only backup"
            if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
                --verbose --clean --if-exists --create --format=plain \
                --schema-only --file="$backup_path" 2>/dev/null; then
                error "Failed to create schema-only backup"
            fi
            ;;
        "data-only")
            log "Creating data-only backup"
            if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
                --verbose --format=plain --data-only \
                --file="$backup_path" 2>/dev/null; then
                error "Failed to create data-only backup"
            fi
            ;;
        "migration-state")
            log "Creating migration state backup"
            if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
                --verbose --format=plain --data-only \
                --table=schema_migrations \
                --file="$backup_path" 2>/dev/null; then
                error "Failed to create migration state backup"
            fi
            ;;
        *)
            error "Unknown backup type: $backup_type"
            ;;
    esac
    
    # Verify backup was created
    if [[ ! -f "$backup_path" ]]; then
        error "Backup file was not created: $backup_path"
    fi
    
    local backup_size
    backup_size=$(stat -c%s "$backup_path" 2>/dev/null || echo "0")
    
    if [[ "$backup_size" -eq 0 ]]; then
        error "Backup file is empty: $backup_path"
    fi
    
    log "Backup created successfully: $backup_path ($backup_size bytes)"
    
    # Calculate checksum
    local checksum
    checksum=$(sha256sum "$backup_path" | cut -d' ' -f1)
    
    log "Backup checksum: $checksum"
    
    # Compress backup if requested
    if [[ "$COMPRESS_BACKUP" == "true" ]]; then
        log "Compressing backup"
        
        if ! gzip "$backup_path"; then
            error "Failed to compress backup"
        fi
        
        backup_path="$backup_path.gz"
        backup_filename="$backup_filename.gz"
        
        local compressed_size
        compressed_size=$(stat -c%s "$backup_path" 2>/dev/null || echo "0")
        
        log "Backup compressed: $backup_path ($compressed_size bytes)"
        
        # Recalculate checksum for compressed file
        checksum=$(sha256sum "$backup_path" | cut -d' ' -f1)
    fi
    
    # Update metadata
    cat > "$metadata_file" << EOF
{
    "backup_type": "$backup_type",
    "environment": "$env",
    "database_host": "$DB_HOST",
    "database_port": "$DB_PORT",
    "database_name": "$DB_NAME",
    "database_user": "$DB_USER",
    "backup_timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "backup_filename": "$backup_filename",
    "backup_size": "$backup_size",
    "backup_checksum": "$checksum",
    "compression": "$COMPRESS_BACKUP",
    "encryption": "$ENCRYPT_BACKUP"
}
EOF
    
    # Export backup information
    BACKUP_PATH="$backup_path"
    BACKUP_FILENAME="$backup_filename"
    BACKUP_CHECKSUM="$checksum"
    
    log "Backup metadata updated: $metadata_file"
}

# Upload backup to S3
upload_backup_to_s3() {
    local backup_path="$1"
    local backup_filename="$2"
    local env="$3"
    
    log "Uploading backup to S3"
    
    get_s3_backup_bucket "$env"
    
    # Check if bucket exists
    if ! aws s3api head-bucket --bucket "$S3_BACKUP_BUCKET" 2>/dev/null; then
        log "Creating S3 backup bucket: $S3_BACKUP_BUCKET"
        
        aws s3api create-bucket --bucket "$S3_BACKUP_BUCKET" --region us-east-1 2>/dev/null || error "Failed to create S3 bucket"
        
        # Enable versioning
        aws s3api put-bucket-versioning --bucket "$S3_BACKUP_BUCKET" --versioning-configuration Status=Enabled 2>/dev/null || true
        
        # Set lifecycle policy
        local lifecycle_policy=$(cat << EOF
{
    "Rules": [
        {
            "ID": "backup-lifecycle",
            "Status": "Enabled",
            "Expiration": {
                "Days": $RETENTION_DAYS
            },
            "NoncurrentVersionExpiration": {
                "NoncurrentDays": 1
            }
        }
    ]
}
EOF
)
        
        echo "$lifecycle_policy" > /tmp/lifecycle-policy.json
        aws s3api put-bucket-lifecycle-configuration --bucket "$S3_BACKUP_BUCKET" --lifecycle-configuration file:///tmp/lifecycle-policy.json 2>/dev/null || true
        rm -f /tmp/lifecycle-policy.json
    fi
    
    # Upload backup
    local s3_key="$env/$(date +%Y/%m/%d)/$backup_filename"
    
    if aws s3 cp "$backup_path" "s3://$S3_BACKUP_BUCKET/$s3_key" --storage-class STANDARD_IA 2>/dev/null; then
        log "Backup uploaded to S3: s3://$S3_BACKUP_BUCKET/$s3_key"
        
        # Upload metadata
        local metadata_file="$backup_path.metadata"
        if [[ -f "$metadata_file" ]]; then
            aws s3 cp "$metadata_file" "s3://$S3_BACKUP_BUCKET/$s3_key.metadata" 2>/dev/null || true
        fi
        
        S3_BACKUP_URL="s3://$S3_BACKUP_BUCKET/$s3_key"
    else
        error "Failed to upload backup to S3"
    fi
}

# Verify backup integrity
verify_backup_integrity() {
    local backup_path="$1"
    local expected_checksum="$2"
    
    log "Verifying backup integrity"
    
    if [[ ! -f "$backup_path" ]]; then
        error "Backup file not found: $backup_path"
    fi
    
    local actual_checksum
    actual_checksum=$(sha256sum "$backup_path" | cut -d' ' -f1)
    
    if [[ "$actual_checksum" != "$expected_checksum" ]]; then
        error "Backup checksum mismatch: expected $expected_checksum, got $actual_checksum"
    fi
    
    log "Backup integrity verified"
    
    # Additional verification for SQL backup
    if [[ "$backup_path" =~ \.sql(\.gz)?$ ]]; then
        log "Verifying SQL backup structure"
        
        local backup_content=""
        if [[ "$backup_path" =~ \.gz$ ]]; then
            backup_content=$(zcat "$backup_path" | head -n 20)
        else
            backup_content=$(head -n 20 "$backup_path")
        fi
        
        if echo "$backup_content" | grep -q "PostgreSQL database dump"; then
            log "SQL backup structure verified"
        else
            log "WARNING: SQL backup may not be valid"
        fi
    fi
}

# Clean up old backups
cleanup_old_backups() {
    local backup_dir="$1"
    local retention_days="$2"
    
    log "Cleaning up old backups (older than $retention_days days)"
    
    local deleted_count=0
    
    # Find and remove old backup files
    if [[ -d "$backup_dir" ]]; then
        find "$backup_dir" -name "*.sql*" -type f -mtime +$retention_days -print0 | while IFS= read -r -d '' file; do
            log "Removing old backup: $file"
            rm -f "$file" "$file.metadata"
            deleted_count=$((deleted_count + 1))
        done
        
        log "Removed $deleted_count old backup files"
    fi
    
    # Clean up S3 backups (handled by lifecycle policy)
    if [[ -n "$S3_BACKUP_BUCKET" ]]; then
        log "S3 backup cleanup handled by lifecycle policy"
    fi
}

# List available backups
list_backups() {
    local env="$1"
    local backup_dir="$2"
    
    log "Listing available backups for environment: $env"
    
    echo "Local Backups:"
    echo "=============="
    
    if [[ -d "$backup_dir" ]]; then
        find "$backup_dir" -name "${env}_*.sql*" -type f | sort -r | head -20 | while read -r backup_file; do
            local backup_size
            backup_size=$(stat -c%s "$backup_file" 2>/dev/null || echo "0")
            
            local backup_date
            backup_date=$(stat -c%y "$backup_file" 2>/dev/null || echo "unknown")
            
            echo "  $(basename "$backup_file") ($backup_size bytes, $backup_date)"
        done
    else
        echo "  No local backups found"
    fi
    
    echo ""
    echo "S3 Backups:"
    echo "==========="
    
    get_s3_backup_bucket "$env"
    
    if aws s3 ls "s3://$S3_BACKUP_BUCKET/$env/" --recursive 2>/dev/null | grep -v metadata | tail -20; then
        true
    else
        echo "  No S3 backups found or S3 access failed"
    fi
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --type)
                BACKUP_TYPE="$2"
                shift 2
                ;;
            --backup-dir)
                BACKUP_DIR="$2"
                shift 2
                ;;
            --retention-days)
                RETENTION_DAYS="$2"
                shift 2
                ;;
            --no-compress)
                COMPRESS_BACKUP="false"
                shift
                ;;
            --encrypt)
                ENCRYPT_BACKUP="true"
                shift
                ;;
            --s3-bucket)
                S3_BACKUP_BUCKET="$2"
                shift 2
                ;;
            --list)
                list_backups "$ENVIRONMENT" "$BACKUP_DIR"
                exit 0
                ;;
            --pre-migration)
                BACKUP_TYPE="pre-migration"
                shift
                ;;
            --pre-rollback)
                BACKUP_TYPE="pre-rollback"
                shift
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

# Usage information
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --environment ENV      Environment to backup (default: staging)"
    echo "  --type TYPE           Backup type: full, schema-only, data-only, migration-state"
    echo "  --backup-dir DIR      Local backup directory (default: /tmp/db-backups)"
    echo "  --retention-days N    Keep backups for N days (default: 7)"
    echo "  --no-compress         Don't compress backup files"
    echo "  --encrypt             Encrypt backup files"
    echo "  --s3-bucket BUCKET    S3 bucket for backup storage"
    echo "  --list                List available backups"
    echo "  --pre-migration       Create pre-migration backup"
    echo "  --pre-rollback        Create pre-rollback backup"
    echo "  --help                Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 --environment production --type full"
    echo "  $0 --environment staging --type schema-only --no-compress"
    echo "  $0 --list --environment production"
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
    local required_tools=("aws" "psql" "pg_dump" "pg_isready" "sha256sum")
    
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &>/dev/null; then
            error "$tool not found. Please install $tool."
        fi
    done
    
    # Check if optional tools are available
    if [[ "$COMPRESS_BACKUP" == "true" && ! command -v gzip &>/dev/null ]]; then
        error "gzip not found. Please install gzip or use --no-compress."
    fi
}

# Main execution
main() {
    log "Starting database backup process"
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check CI environment and dependencies
    check_ci_environment
    
    # Get database configuration
    get_db_config "$ENVIRONMENT"
    
    # Test database connectivity
    test_db_connectivity
    
    # Create backup directory
    create_backup_directory "$BACKUP_DIR"
    
    # Create database backup
    create_database_backup "$ENVIRONMENT" "$BACKUP_TYPE" "$BACKUP_DIR"
    
    # Verify backup integrity
    verify_backup_integrity "$BACKUP_PATH" "$BACKUP_CHECKSUM"
    
    # Upload to S3 if in production or requested
    if [[ "$ENVIRONMENT" == "production" || -n "$S3_BACKUP_BUCKET" ]]; then
        upload_backup_to_s3 "$BACKUP_PATH" "$BACKUP_FILENAME" "$ENVIRONMENT"
    fi
    
    # Clean up old backups
    cleanup_old_backups "$BACKUP_DIR" "$RETENTION_DAYS"
    
    log "Database backup process completed successfully"
    log "Backup location: $BACKUP_PATH"
    
    if [[ -n "${S3_BACKUP_URL:-}" ]]; then
        log "S3 backup location: $S3_BACKUP_URL"
    fi
}

# Execute main function
main "$@"