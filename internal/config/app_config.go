package config

import (
	"fmt"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/application"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/monitoring"
)

// AppConfig represents the complete application configuration
type AppConfig struct {
	// Server configuration
	Server ServerConfig `json:"server" yaml:"server" validate:"required"`
	
	// Database configuration
	Database infrastructure.DatabaseConfig `json:"database" yaml:"database" validate:"required"`
	
	// Redis configuration
	Redis infrastructure.RedisConfig `json:"redis" yaml:"redis" validate:"required"`
	
	// Cache configuration
	Cache infrastructure.CacheConfig `json:"cache" yaml:"cache" validate:"required"`
	
	// Authentication middleware configuration
	Auth application.AuthMiddlewareConfig `json:"auth" yaml:"auth" validate:"required"`
	
	// Event handler configuration
	EventHandler application.EventHandlerConfig `json:"event_handler" yaml:"event_handler" validate:"required"`
	
	// Cache service configuration
	CacheService application.CacheServiceConfig `json:"cache_service" yaml:"cache_service" validate:"required"`
	
	// Processing configuration
	Processing application.ProcessingConfig `json:"processing" yaml:"processing" validate:"required"`
	
	// Error handler configuration
	ErrorHandler infrastructure.ErrorHandlerConfig `json:"error_handler" yaml:"error_handler" validate:"required"`
	
	// Monitoring configuration
	Monitoring monitoring.Config `json:"monitoring" yaml:"monitoring" validate:"required"`
	
	// Circuit breaker configuration
	CircuitBreaker infrastructure.CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker" validate:"required"`
	
	// Application-specific settings
	App AppSettings `json:"app" yaml:"app" validate:"required"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host         string        `json:"host" yaml:"host" validate:"required"`
	Port         int           `json:"port" yaml:"port" validate:"min=1,max=65535"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout" validate:"min=1s"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" validate:"min=1s"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout" validate:"min=1s"`
	EnableTLS    bool          `json:"enable_tls" yaml:"enable_tls"`
	TLSCertFile  string        `json:"tls_cert_file" yaml:"tls_cert_file"`
	TLSKeyFile   string        `json:"tls_key_file" yaml:"tls_key_file"`
	EnablePprof  bool          `json:"enable_pprof" yaml:"enable_pprof"`
	PprofPort    int           `json:"pprof_port" yaml:"pprof_port" validate:"min=1,max=65535"`
}

// AppSettings represents application-specific settings
type AppSettings struct {
	Name                string `json:"name" yaml:"name" validate:"required"`
	Version             string `json:"version" yaml:"version" validate:"required"`
	Environment         string `json:"environment" yaml:"environment" validate:"required,oneof=development staging production"`
	LogLevel            string `json:"log_level" yaml:"log_level" validate:"required,oneof=debug info warn error"`
	EnableMetrics       bool   `json:"enable_metrics" yaml:"enable_metrics"`
	EnableTracing       bool   `json:"enable_tracing" yaml:"enable_tracing"`
	EnableHealthChecks  bool   `json:"enable_health_checks" yaml:"enable_health_checks"`
	ShutdownTimeout     time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout" validate:"min=1s"`
	MaxRequestSize      int64  `json:"max_request_size" yaml:"max_request_size" validate:"min=1"`
	EnableRateLimit     bool   `json:"enable_rate_limit" yaml:"enable_rate_limit"`
	EnableAuthentication bool   `json:"enable_authentication" yaml:"enable_authentication"`
}

// GetDefaultConfig returns the default application configuration
func GetDefaultConfig() *AppConfig {
	return &AppConfig{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			EnableTLS:    false,
			TLSCertFile:  "",
			TLSKeyFile:   "",
			EnablePprof:  false,
			PprofPort:    6060,
		},
		Database: infrastructure.DatabaseConfig{
			Host:         "localhost",
			Port:         5432,
			Database:     "hotel_reviews",
			Username:     "postgres",
			Password:     "",
			SSLMode:      "disable",
			MaxConns:     25,
			MinConns:     5,
			ConnTTL:      time.Hour,
			QueryTimeout: 30 * time.Second,
		},
		Redis: infrastructure.RedisConfig{
			Host:               "localhost",
			Port:               6379,
			Password:           "",
			Database:           0,
			MaxRetries:         3,
			MinRetryBackoff:    8 * time.Millisecond,
			MaxRetryBackoff:    512 * time.Millisecond,
			DialTimeout:        5 * time.Second,
			ReadTimeout:        3 * time.Second,
			WriteTimeout:       3 * time.Second,
			PoolSize:           10,
			MinIdleConns:       5,
			MaxConnAge:         time.Hour,
			PoolTimeout:        4 * time.Second,
			IdleTimeout:        5 * time.Minute,
			IdleCheckFrequency: time.Minute,
		},
		Cache: infrastructure.CacheConfig{
			ReviewTTL:          time.Hour,
			HotelTTL:           2 * time.Hour,
			ProviderTTL:        4 * time.Hour,
			StatisticsTTL:      30 * time.Minute,
			SearchTTL:          15 * time.Minute,
			DefaultTTL:         time.Hour,
			MaxKeyLength:       250,
			EnableCompression:  true,
			CompressionLevel:   6,
			PrefixSeparator:    ":",
		},
		Auth: *application.DefaultAuthMiddlewareConfig(),
		EventHandler: application.EventHandlerConfig{
			MaxWorkers:          10,
			MaxRetries:          3,
			RetryDelay:          time.Second,
			RetryBackoffFactor:  2.0,
			MaxRetryDelay:       30 * time.Second,
			ProcessingTimeout:   30 * time.Second,
			BufferSize:          100,
			EnableMetrics:       true,
			EnableNotifications: true,
			EnableReplay:        false,
			DeadLetterThreshold: 5,
		},
		CacheService: application.CacheServiceConfig{
			ReviewTTL:                  time.Hour,
			HotelTTL:                   2 * time.Hour,
			ProcessingTTL:              30 * time.Minute,
			AnalyticsTTL:               time.Hour,
			DefaultTTL:                 time.Hour,
			ReviewKeyPrefix:            "review",
			HotelKeyPrefix:             "hotel",
			ProcessingKeyPrefix:        "processing",
			AnalyticsKeyPrefix:         "analytics",
			WarmupConcurrency:          5,
			WarmupBatchSize:            100,
			EnableBackgroundWarmup:     true,
			WarmupInterval:             time.Hour,
			InvalidationBatchSize:      50,
			InvalidationDelay:          time.Second,
			EnableSmartInvalidation:    true,
			MaxConcurrentOperations:    10,
			OperationTimeout:           30 * time.Second,
			RetryAttempts:              3,
			RetryDelay:                 time.Second,
		},
		Processing: application.ProcessingConfig{
			MaxWorkers:         5,
			MaxConcurrentFiles: 3,
			MaxRetries:         3,
			RetryDelay:         5 * time.Second,
			ProcessingTimeout:  5 * time.Minute,
			WorkerIdleTimeout:  30 * time.Second,
			MetricsInterval:    30 * time.Second,
		},
		ErrorHandler: infrastructure.ErrorHandlerConfig{
			EnableMetrics:          true,
			EnableAlerting:         true,
			EnableStackTrace:       true,
			EnableDetailedLogging:  true,
			EnableErrorAggregation: true,
			EnableRateLimiting:     true,
			MaxStackTraceDepth:     50,
			ErrorRetentionPeriod:   24 * time.Hour,
			MetricsInterval:        time.Minute,
			AlertingThreshold:      100,
			AlertingWindow:         5 * time.Minute,
			RateLimitWindow:        time.Minute,
			RateLimitThreshold:     10,
			DefaultFormat:          infrastructure.FormatJSON,
			IncludeInternalErrors:  false,
		},
		Monitoring: monitoring.Config{
			MetricsEnabled:       true,
			MetricsPath:          "/metrics",
			TracingEnabled:       false,
			TracingServiceName:   "hotel-reviews",
			TracingVersion:       "1.0.0",
			TracingEnvironment:   "development",
			JaegerEndpoint:       "http://localhost:14268/api/traces",
			TracingSamplingRate:  0.1,
			HealthEnabled:        true,
			HealthPath:           "/health",
		},
		CircuitBreaker: *infrastructure.DefaultCircuitBreakerConfig(),
		App: AppSettings{
			Name:                "hotel-reviews-service",
			Version:             "1.0.0",
			Environment:         "development",
			LogLevel:            "info",
			EnableMetrics:       true,
			EnableTracing:       false,
			EnableHealthChecks:  true,
			ShutdownTimeout:     30 * time.Second,
			MaxRequestSize:      10 * 1024 * 1024, // 10MB
			EnableRateLimit:     true,
			EnableAuthentication: true,
		},
	}
}

// Validate validates the entire application configuration
func (c *AppConfig) Validate() error {
	// Custom validation logic can be added here
	// For example, check that TLS settings are consistent
	if c.Server.EnableTLS {
		if c.Server.TLSCertFile == "" || c.Server.TLSKeyFile == "" {
			return fmt.Errorf("TLS enabled but cert or key file not specified")
		}
	}
	
	// Validate that database and Redis configurations are compatible
	if c.Database.MaxConns < c.Database.MinConns {
		return fmt.Errorf("database max_conns (%d) cannot be less than min_conns (%d)", 
			c.Database.MaxConns, c.Database.MinConns)
	}
	
	// Validate processing configuration
	if c.Processing.MaxConcurrentFiles > c.Processing.MaxWorkers {
		return fmt.Errorf("max_concurrent_files (%d) cannot exceed max_workers (%d)",
			c.Processing.MaxConcurrentFiles, c.Processing.MaxWorkers)
	}
	
	// Validate environment-specific settings
	if c.App.Environment == "production" {
		if c.App.LogLevel == "debug" {
			return fmt.Errorf("debug log level not allowed in production")
		}
		if !c.Server.EnableTLS {
			return fmt.Errorf("TLS must be enabled in production")
		}
	}
	
	return nil
}

// GetServerAddr returns the full server address
func (c *AppConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetDatabaseDSN returns the database connection string
func (c *AppConfig) GetDatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.Username, 
		c.Database.Password, c.Database.Database, c.Database.SSLMode)
}

// GetRedisAddr returns the Redis connection address
func (c *AppConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// IsProduction returns true if running in production environment
func (c *AppConfig) IsProduction() bool {
	return c.App.Environment == "production"
}

// IsDevelopment returns true if running in development environment
func (c *AppConfig) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// GetConfigSummary returns a summary of key configuration values
func (c *AppConfig) GetConfigSummary() map[string]interface{} {
	return map[string]interface{}{
		"app_name":        c.App.Name,
		"app_version":     c.App.Version,
		"environment":     c.App.Environment,
		"server_addr":     c.GetServerAddr(),
		"database_host":   fmt.Sprintf("%s:%d", c.Database.Host, c.Database.Port),
		"redis_addr":      c.GetRedisAddr(),
		"log_level":       c.App.LogLevel,
		"metrics_enabled": c.App.EnableMetrics,
		"tracing_enabled": c.App.EnableTracing,
		"auth_enabled":    c.App.EnableAuthentication,
		"tls_enabled":     c.Server.EnableTLS,
	}
}