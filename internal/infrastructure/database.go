package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// Database represents the database connection wrapper
type Database struct {
	DB     *gorm.DB
	config *config.DatabaseConfig
	logger *logger.Logger
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.DatabaseConfig, log *logger.Logger) (*Database, error) {
	db := &Database{
		config: cfg,
		logger: log,
	}

	if err := db.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// connect establishes the database connection with retry logic
func (d *Database) connect() error {
	dsn := d.buildDSN()
	
	var gormDB *gorm.DB
	var err error
	
	maxRetries := 5
	retryDelay := 2 * time.Second
	
	for i := 0; i < maxRetries; i++ {
		gormDB, err = d.attemptConnection(dsn)
		if err == nil {
			break
		}
		
		d.logger.Warn("Database connection failed, retrying...",
			"attempt", i+1,
			"max_retries", maxRetries,
			"error", err,
		)
		
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}
	
	if err != nil {
		return fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
	}
	
	d.DB = gormDB
	
	// Configure connection pool
	if err := d.configureConnectionPool(); err != nil {
		return fmt.Errorf("failed to configure connection pool: %w", err)
	}
	
	d.logger.Info("Database connection established successfully",
		"host", d.config.Host,
		"port", d.config.Port,
		"database", d.config.Name,
	)
	
	return nil
}

// attemptConnection attempts to establish a database connection
func (d *Database) attemptConnection(dsn string) (*gorm.DB, error) {
	// Configure GORM logger
	gormLogLevel := d.getGormLogLevel()
	gormConfig := &gorm.Config{
		Logger: gormLogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormLogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  gormLogLevel,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: false,
	}
	
	return gorm.Open(postgres.Open(dsn), gormConfig)
}

// buildDSN builds the database connection string
func (d *Database) buildDSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		d.config.Host,
		d.config.User,
		d.config.Password,
		d.config.Name,
		d.config.Port,
		d.config.SSLMode,
		d.config.TimeZone,
	)
}

// configureConnectionPool configures the database connection pool
func (d *Database) configureConnectionPool() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	// Set connection pool settings
	sqlDB.SetMaxOpenConns(d.config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(d.config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(d.config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(d.config.ConnMaxIdleTime)
	
	d.logger.Info("Database connection pool configured",
		"max_open_conns", d.config.MaxOpenConns,
		"max_idle_conns", d.config.MaxIdleConns,
		"conn_max_lifetime", d.config.ConnMaxLifetime,
		"conn_max_idle_time", d.config.ConnMaxIdleTime,
	)
	
	return nil
}

// getGormLogLevel converts string log level to GORM log level
func (d *Database) getGormLogLevel() gormLogger.LogLevel {
	switch d.config.LogLevel {
	case "debug":
		return gormLogger.Info
	case "info":
		return gormLogger.Warn
	case "warn":
		return gormLogger.Warn
	case "error":
		return gormLogger.Error
	default:
		return gormLogger.Warn
	}
}

// Migrate runs database migrations
func (d *Database) Migrate() error {
	d.logger.Info("Starting database migration...")
	
	entities := []interface{}{
		&domain.Provider{},
		&domain.Hotel{},
		&domain.ReviewerInfo{},
		&domain.Review{},
		&domain.ReviewSummary{},
		&domain.ReviewProcessingStatus{},
	}
	
	for _, entity := range entities {
		if err := d.DB.AutoMigrate(entity); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", entity, err)
		}
	}
	
	// Create indexes
	if err := d.createIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	
	d.logger.Info("Database migration completed successfully")
	return nil
}

// createIndexes creates additional database indexes for performance
func (d *Database) createIndexes() error {
	indexes := []struct {
		table string
		index string
	}{
		{"reviews", "CREATE INDEX IF NOT EXISTS idx_reviews_hotel_id_rating ON reviews(hotel_id, rating)"},
		{"reviews", "CREATE INDEX IF NOT EXISTS idx_reviews_provider_id_review_date ON reviews(provider_id, review_date)"},
		{"reviews", "CREATE INDEX IF NOT EXISTS idx_reviews_review_date ON reviews(review_date)"},
		{"reviews", "CREATE INDEX IF NOT EXISTS idx_reviews_rating ON reviews(rating)"},
		{"reviews", "CREATE INDEX IF NOT EXISTS idx_reviews_external_id ON reviews(external_id)"},
		{"reviews", "CREATE INDEX IF NOT EXISTS idx_reviews_processing_hash ON reviews(processing_hash)"},
		{"hotels", "CREATE INDEX IF NOT EXISTS idx_hotels_name ON hotels(name)"},
		{"hotels", "CREATE INDEX IF NOT EXISTS idx_hotels_city_country ON hotels(city, country)"},
		{"hotels", "CREATE INDEX IF NOT EXISTS idx_hotels_star_rating ON hotels(star_rating)"},
		{"providers", "CREATE INDEX IF NOT EXISTS idx_providers_name ON providers(name)"},
		{"providers", "CREATE INDEX IF NOT EXISTS idx_providers_is_active ON providers(is_active)"},
		{"reviewer_infos", "CREATE INDEX IF NOT EXISTS idx_reviewer_infos_email ON reviewer_infos(email)"},
		{"reviewer_infos", "CREATE INDEX IF NOT EXISTS idx_reviewer_infos_is_verified ON reviewer_infos(is_verified)"},
		{"review_summaries", "CREATE INDEX IF NOT EXISTS idx_review_summaries_hotel_id ON review_summaries(hotel_id)"},
		{"review_processing_statuses", "CREATE INDEX IF NOT EXISTS idx_review_processing_statuses_provider_id_status ON review_processing_statuses(provider_id, status)"},
		{"review_processing_statuses", "CREATE INDEX IF NOT EXISTS idx_review_processing_statuses_created_at ON review_processing_statuses(created_at)"},
	}
	
	for _, idx := range indexes {
		if err := d.DB.Exec(idx.index).Error; err != nil {
			d.logger.Warn("Failed to create index", "table", idx.table, "error", err)
		}
	}
	
	return nil
}

// HealthCheck performs a database health check
func (d *Database) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	// Test a simple query
	var result int
	if err := d.DB.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error; err != nil {
		return fmt.Errorf("test query failed: %w", err)
	}
	
	return nil
}

// GetStats returns database connection statistics
func (d *Database) GetStats() (*sql.DBStats, error) {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	stats := sqlDB.Stats()
	return &stats, nil
}

// LogStats logs database connection statistics
func (d *Database) LogStats(ctx context.Context) {
	stats, err := d.GetStats()
	if err != nil {
		d.logger.ErrorContext(ctx, "Failed to get database stats", "error", err)
		return
	}
	
	d.logger.InfoContext(ctx, "Database connection statistics",
		"open_connections", stats.OpenConnections,
		"in_use", stats.InUse,
		"idle", stats.Idle,
		"wait_count", stats.WaitCount,
		"wait_duration", stats.WaitDuration,
		"max_idle_closed", stats.MaxIdleClosed,
		"max_idle_time_closed", stats.MaxIdleTimeClosed,
		"max_lifetime_closed", stats.MaxLifetimeClosed,
	)
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.DB == nil {
		return nil
	}
	
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	
	d.logger.Info("Database connection closed successfully")
	return nil
}

// WithContext returns a new GORM DB instance with context
func (d *Database) WithContext(ctx context.Context) *gorm.DB {
	return d.DB.WithContext(ctx)
}

// Transaction executes a function within a database transaction
func (d *Database) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

// BeginTransaction starts a new database transaction
func (d *Database) BeginTransaction(ctx context.Context) *gorm.DB {
	return d.DB.WithContext(ctx).Begin()
}

// Seed seeds the database with initial data
func (d *Database) Seed(ctx context.Context) error {
	d.logger.InfoContext(ctx, "Seeding database with initial data...")
	
	// Create default providers
	providers := []domain.Provider{
		{
			Name:     "Booking.com",
			BaseURL:  "https://www.booking.com",
			IsActive: true,
		},
		{
			Name:     "Expedia",
			BaseURL:  "https://www.expedia.com",
			IsActive: true,
		},
		{
			Name:     "Hotels.com",
			BaseURL:  "https://www.hotels.com",
			IsActive: true,
		},
		{
			Name:     "Agoda",
			BaseURL:  "https://www.agoda.com",
			IsActive: true,
		},
		{
			Name:     "TripAdvisor",
			BaseURL:  "https://www.tripadvisor.com",
			IsActive: true,
		},
	}
	
	for _, provider := range providers {
		var existingProvider domain.Provider
		if err := d.DB.WithContext(ctx).Where("name = ?", provider.Name).First(&existingProvider).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := d.DB.WithContext(ctx).Create(&provider).Error; err != nil {
					return fmt.Errorf("failed to create provider %s: %w", provider.Name, err)
				}
				d.logger.InfoContext(ctx, "Created provider", "name", provider.Name)
			} else {
				return fmt.Errorf("failed to check provider existence: %w", err)
			}
		}
	}
	
	d.logger.InfoContext(ctx, "Database seeding completed successfully")
	return nil
}

// Reset resets the database (drops all tables and recreates them)
func (d *Database) Reset(ctx context.Context) error {
	d.logger.WarnContext(ctx, "Resetting database - all data will be lost!")
	
	entities := []interface{}{
		&domain.ReviewProcessingStatus{},
		&domain.ReviewSummary{},
		&domain.Review{},
		&domain.ReviewerInfo{},
		&domain.Hotel{},
		&domain.Provider{},
	}
	
	// Drop tables in reverse order to handle foreign keys
	for i := len(entities) - 1; i >= 0; i-- {
		if err := d.DB.WithContext(ctx).Migrator().DropTable(entities[i]); err != nil {
			d.logger.WarnContext(ctx, "Failed to drop table", "entity", entities[i], "error", err)
		}
	}
	
	// Run migrations to recreate tables
	if err := d.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations after reset: %w", err)
	}
	
	d.logger.InfoContext(ctx, "Database reset completed successfully")
	return nil
}

// GetConnectionURL returns the database connection URL (without password)
func (d *Database) GetConnectionURL() string {
	return fmt.Sprintf(
		"postgres://%s:***@%s:%d/%s?sslmode=%s&TimeZone=%s",
		d.config.User,
		d.config.Host,
		d.config.Port,
		d.config.Name,
		d.config.SSLMode,
		d.config.TimeZone,
	)
}

// IsHealthy checks if the database is healthy
func (d *Database) IsHealthy(ctx context.Context) bool {
	return d.HealthCheck(ctx) == nil
}

// WaitForConnection waits for database connection to be available
func (d *Database) WaitForConnection(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for database connection: %w", ctx.Err())
		case <-ticker.C:
			if err := d.HealthCheck(ctx); err == nil {
				return nil
			}
		}
	}
}

// StartStatsLogger starts a goroutine that periodically logs database statistics
func (d *Database) StartStatsLogger(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.LogStats(ctx)
			}
		}
	}()
}

// CreateBackup creates a database backup (PostgreSQL specific)
func (d *Database) CreateBackup(ctx context.Context, backupPath string) error {
	// This is a placeholder for backup functionality
	// In production, you would implement pg_dump or similar
	d.logger.InfoContext(ctx, "Database backup requested", "path", backupPath)
	return fmt.Errorf("backup functionality not implemented")
}

// RestoreBackup restores a database backup (PostgreSQL specific)
func (d *Database) RestoreBackup(ctx context.Context, backupPath string) error {
	// This is a placeholder for restore functionality
	// In production, you would implement pg_restore or similar
	d.logger.InfoContext(ctx, "Database restore requested", "path", backupPath)
	return fmt.Errorf("restore functionality not implemented")
}