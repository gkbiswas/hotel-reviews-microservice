# Database Migrations

This directory contains SQL migration files for the Hotel Reviews Microservice database schema.

## Migration Files

### Applied Migrations

1. **001_initial_schema.sql** - Initial database schema
   - Creates all core tables (providers, hotels, reviews, etc.)
   - Sets up indexes, constraints, and foreign keys
   - Includes custom types and functions
   - Creates audit logging infrastructure

2. **002_performance_optimizations.sql** - Performance optimizations
   - Adds performance indexes
   - Creates materialized views for analytics
   - Includes optimization functions
   - Sets up monitoring views

## Schema Overview

### Core Tables

- **providers** - Review providers (Booking.com, Expedia, etc.)
- **hotels** - Hotel information and metadata
- **reviewer_infos** - Reviewer profiles and statistics
- **reviews** - Main reviews table with ratings and comments
- **review_summaries** - Aggregated review statistics per hotel
- **review_processing_statuses** - File processing status tracking
- **hotel_chains** - Hotel chain information
- **audit_logs** - Audit trail for all changes

### Key Features

- **UUID Primary Keys** - All tables use UUID primary keys for scalability
- **Soft Deletes** - All main tables support soft deletes with `deleted_at` column
- **JSONB Support** - Metadata, amenities, and other flexible data stored as JSONB
- **Full-text Search** - GIN indexes for efficient text search
- **Audit Logging** - Complete audit trail for all changes
- **Performance Indexes** - Optimized indexes for common query patterns

## Migration Management

### Using the Migration Script

```bash
# Show current migration status
./migrate.sh status

# Apply all pending migrations
./migrate.sh up

# Rollback to specific version
./migrate.sh down 001_initial_schema

# Create new migration
./migrate.sh create add_new_feature

# Reset database (destructive)
./migrate.sh reset --force

# Validate migration files
./migrate.sh validate
```

### Environment Variables

```bash
# Database connection URL
export DATABASE_URL="postgresql://user:password@localhost:5432/hotel_reviews"

# Or use individual components
export HOTEL_REVIEWS_DATABASE_HOST=localhost
export HOTEL_REVIEWS_DATABASE_PORT=5432
export HOTEL_REVIEWS_DATABASE_USER=postgres
export HOTEL_REVIEWS_DATABASE_PASSWORD=postgres
export HOTEL_REVIEWS_DATABASE_NAME=hotel_reviews
```

### Manual Migration

If you prefer to run migrations manually:

```bash
# Apply initial schema
psql $DATABASE_URL -f 001_initial_schema.sql

# Apply performance optimizations
psql $DATABASE_URL -f 002_performance_optimizations.sql

# Rollback (if needed)
psql $DATABASE_URL -f 001_initial_schema_rollback.sql
```

## Database Schema Design

### Scalability Considerations

1. **Partitioning Ready** - Tables designed for easy partitioning by date
2. **Efficient Indexes** - Covering indexes and partial indexes for performance
3. **Materialized Views** - Pre-computed aggregations for analytics
4. **Connection Pooling** - Optimized for connection pooling

### Security Features

1. **Role-based Access** - Separate roles for app and read-only access
2. **Audit Logging** - Complete audit trail for compliance
3. **Input Validation** - Database-level constraints and checks
4. **SQL Injection Prevention** - Parameterized queries and validation

### Performance Optimizations

1. **Index Strategy** - Comprehensive indexing strategy for all query patterns
2. **Query Optimization** - Efficient functions for common operations
3. **Statistics** - Automated statistics collection and updates
4. **Monitoring** - Built-in performance monitoring views

## Common Operations

### Adding New Migrations

1. Create migration file: `./migrate.sh create feature_name`
2. Edit the generated SQL files
3. Test the migration: `./migrate.sh up --dry-run`
4. Apply the migration: `./migrate.sh up`

### Rollback Strategy

1. Always create rollback files for destructive changes
2. Test rollbacks in staging environment
3. Keep rollback files simple and idempotent
4. Document any data loss implications

### Performance Monitoring

```sql
-- Check slow queries
SELECT * FROM slow_query_candidates;

-- Monitor table performance
SELECT * FROM table_performance_stats;

-- Update review statistics
SELECT optimize_database();

-- Refresh materialized views
SELECT refresh_performance_metrics();
```

## Best Practices

### Migration Development

1. **Incremental Changes** - Keep migrations small and focused
2. **Backwards Compatibility** - Ensure migrations don't break existing code
3. **Testing** - Test migrations in development environment first
4. **Documentation** - Document complex changes and their rationale

### Production Deployment

1. **Backup First** - Always backup before running migrations
2. **Maintenance Window** - Run migrations during low-traffic periods
3. **Monitoring** - Monitor performance after applying migrations
4. **Rollback Plan** - Have a tested rollback plan ready

### Schema Evolution

1. **Additive Changes** - Prefer adding new columns over modifying existing ones
2. **Default Values** - Always provide default values for new columns
3. **Index Creation** - Use `CREATE INDEX CONCURRENTLY` for large tables
4. **Data Migration** - Separate data migration from schema changes

## Troubleshooting

### Common Issues

1. **Lock Conflicts** - Use CONCURRENTLY for index creation
2. **Long Running Migrations** - Break large migrations into smaller chunks
3. **Constraint Violations** - Validate data before adding constraints
4. **Performance Degradation** - Monitor query performance after changes

### Recovery Procedures

1. **Failed Migration** - Use `./migrate.sh repair` to fix migration state
2. **Data Corruption** - Restore from backup and replay migrations
3. **Performance Issues** - Use `ANALYZE` and `VACUUM` commands
4. **Disk Space** - Monitor disk usage during large migrations

## Schema Documentation

For detailed schema documentation, see:
- Database design documents
- API documentation
- Entity relationship diagrams
- Performance benchmarks

## Support

For questions or issues with migrations:
1. Check the troubleshooting section above
2. Review migration logs for error details
3. Consult the database administrator
4. Create an issue in the project repository