package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the main configuration structure
type Config struct {
	Database     DatabaseConfig     `mapstructure:"database" json:"database"`
	S3           S3Config           `mapstructure:"s3" json:"s3"`
	Server       ServerConfig       `mapstructure:"server" json:"server"`
	Log          LogConfig          `mapstructure:"log" json:"log"`
	Cache        CacheConfig        `mapstructure:"cache" json:"cache"`
	Metrics      MetricsConfig      `mapstructure:"metrics" json:"metrics"`
	Notification NotificationConfig `mapstructure:"notification" json:"notification"`
	Processing   ProcessingConfig   `mapstructure:"processing" json:"processing"`
	Security     SecurityConfig     `mapstructure:"security" json:"security"`
	Auth         AuthConfig         `mapstructure:"auth" json:"auth"`
	Kafka        KafkaConfig        `mapstructure:"kafka" json:"kafka"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host" json:"host" validate:"required"`
	Port            int           `mapstructure:"port" json:"port" validate:"required,min=1,max=65535"`
	User            string        `mapstructure:"user" json:"user" validate:"required"`
	Password        string        `mapstructure:"password" json:"password" validate:"required"`
	Name            string        `mapstructure:"name" json:"name" validate:"required"`
	SSLMode         string        `mapstructure:"ssl_mode" json:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time" json:"conn_max_idle_time"`
	TimeZone        string        `mapstructure:"timezone" json:"timezone"`
	LogLevel        string        `mapstructure:"log_level" json:"log_level"`
}

// S3Config represents AWS S3 configuration
type S3Config struct {
	Region           string        `mapstructure:"region" json:"region" validate:"required"`
	AccessKeyID      string        `mapstructure:"access_key_id" json:"access_key_id" validate:"required"`
	SecretAccessKey  string        `mapstructure:"secret_access_key" json:"secret_access_key" validate:"required"`
	SessionToken     string        `mapstructure:"session_token" json:"session_token"`
	Bucket           string        `mapstructure:"bucket" json:"bucket" validate:"required"`
	Endpoint         string        `mapstructure:"endpoint" json:"endpoint"`
	UseSSL           bool          `mapstructure:"use_ssl" json:"use_ssl"`
	ForcePathStyle   bool          `mapstructure:"force_path_style" json:"force_path_style"`
	Timeout          time.Duration `mapstructure:"timeout" json:"timeout"`
	RetryCount       int           `mapstructure:"retry_count" json:"retry_count"`
	RetryDelay       time.Duration `mapstructure:"retry_delay" json:"retry_delay"`
	UploadPartSize   int64         `mapstructure:"upload_part_size" json:"upload_part_size"`
	DownloadPartSize int64         `mapstructure:"download_part_size" json:"download_part_size"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host            string        `mapstructure:"host" json:"host"`
	Port            int           `mapstructure:"port" json:"port" validate:"required,min=1,max=65535"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout" json:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout" json:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" json:"shutdown_timeout"`
	MaxHeaderBytes  int           `mapstructure:"max_header_bytes" json:"max_header_bytes"`
	EnableCORS      bool          `mapstructure:"enable_cors" json:"enable_cors"`
	EnableGzip      bool          `mapstructure:"enable_gzip" json:"enable_gzip"`
	EnableMetrics   bool          `mapstructure:"enable_metrics" json:"enable_metrics"`
	EnablePprof     bool          `mapstructure:"enable_pprof" json:"enable_pprof"`
	TLSCertFile     string        `mapstructure:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile      string        `mapstructure:"tls_key_file" json:"tls_key_file"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies" json:"trusted_proxies"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level            string `mapstructure:"level" json:"level" validate:"required,oneof=debug info warn error"`
	Format           string `mapstructure:"format" json:"format" validate:"required,oneof=json text"`
	Output           string `mapstructure:"output" json:"output" validate:"required,oneof=stdout stderr file"`
	FilePath         string `mapstructure:"file_path" json:"file_path"`
	MaxSize          int    `mapstructure:"max_size" json:"max_size"`
	MaxBackups       int    `mapstructure:"max_backups" json:"max_backups"`
	MaxAge           int    `mapstructure:"max_age" json:"max_age"`
	Compress         bool   `mapstructure:"compress" json:"compress"`
	EnableCaller     bool   `mapstructure:"enable_caller" json:"enable_caller"`
	EnableStacktrace bool   `mapstructure:"enable_stacktrace" json:"enable_stacktrace"`
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	Type         string        `mapstructure:"type" json:"type" validate:"required,oneof=redis memory"`
	Host         string        `mapstructure:"host" json:"host"`
	Port         int           `mapstructure:"port" json:"port"`
	Password     string        `mapstructure:"password" json:"password"`
	Database     int           `mapstructure:"database" json:"database"`
	PoolSize     int           `mapstructure:"pool_size" json:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" json:"write_timeout"`
	TTL          time.Duration `mapstructure:"ttl" json:"ttl"`
	MaxMemory    int64         `mapstructure:"max_memory" json:"max_memory"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled     bool   `mapstructure:"enabled" json:"enabled"`
	Type        string `mapstructure:"type" json:"type" validate:"oneof=prometheus jaeger"`
	Host        string `mapstructure:"host" json:"host"`
	Port        int    `mapstructure:"port" json:"port"`
	Path        string `mapstructure:"path" json:"path"`
	Namespace   string `mapstructure:"namespace" json:"namespace"`
	ServiceName string `mapstructure:"service_name" json:"service_name"`
	Environment string `mapstructure:"environment" json:"environment"`
	Version     string `mapstructure:"version" json:"version"`
}

// NotificationConfig represents notification configuration
type NotificationConfig struct {
	Email EmailConfig `mapstructure:"email" json:"email"`
	Slack SlackConfig `mapstructure:"slack" json:"slack"`
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled" json:"enabled"`
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	Username string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	From     string `mapstructure:"from" json:"from"`
	UseTLS   bool   `mapstructure:"use_tls" json:"use_tls"`
}

// SlackConfig represents Slack notification configuration
type SlackConfig struct {
	Enabled    bool   `mapstructure:"enabled" json:"enabled"`
	WebhookURL string `mapstructure:"webhook_url" json:"webhook_url"`
	Channel    string `mapstructure:"channel" json:"channel"`
	Username   string `mapstructure:"username" json:"username"`
	IconEmoji  string `mapstructure:"icon_emoji" json:"icon_emoji"`
	IconURL    string `mapstructure:"icon_url" json:"icon_url"`
}

// ProcessingConfig represents file processing configuration
type ProcessingConfig struct {
	BatchSize            int           `mapstructure:"batch_size" json:"batch_size"`
	WorkerCount          int           `mapstructure:"worker_count" json:"worker_count"`
	MaxFileSize          int64         `mapstructure:"max_file_size" json:"max_file_size"`
	ProcessingTimeout    time.Duration `mapstructure:"processing_timeout" json:"processing_timeout"`
	MaxRetries           int           `mapstructure:"max_retries" json:"max_retries"`
	RetryDelay           time.Duration `mapstructure:"retry_delay" json:"retry_delay"`
	EnableValidation     bool          `mapstructure:"enable_validation" json:"enable_validation"`
	EnableDuplicateCheck bool          `mapstructure:"enable_duplicate_check" json:"enable_duplicate_check"`
	TempDirectory        string        `mapstructure:"temp_directory" json:"temp_directory"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	JWTSecret        string        `mapstructure:"jwt_secret" json:"jwt_secret" validate:"required"`
	JWTExpiration    time.Duration `mapstructure:"jwt_expiration" json:"jwt_expiration"`
	RateLimit        int           `mapstructure:"rate_limit" json:"rate_limit"`
	RateLimitWindow  time.Duration `mapstructure:"rate_limit_window" json:"rate_limit_window"`
	EnableAPIKey     bool          `mapstructure:"enable_api_key" json:"enable_api_key"`
	APIKeyHeader     string        `mapstructure:"api_key_header" json:"api_key_header"`
	EnableEncryption bool          `mapstructure:"enable_encryption" json:"enable_encryption"`
	EncryptionKey    string        `mapstructure:"encryption_key" json:"encryption_key"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	JWTSecret               string        `mapstructure:"jwt_secret" json:"jwt_secret" validate:"required"`
	JWTIssuer               string        `mapstructure:"jwt_issuer" json:"jwt_issuer"`
	AccessTokenExpiry       time.Duration `mapstructure:"access_token_expiry" json:"access_token_expiry"`
	RefreshTokenExpiry      time.Duration `mapstructure:"refresh_token_expiry" json:"refresh_token_expiry"`
	MaxLoginAttempts        int           `mapstructure:"max_login_attempts" json:"max_login_attempts"`
	LoginAttemptWindow      time.Duration `mapstructure:"login_attempt_window" json:"login_attempt_window"`
	AccountLockDuration     time.Duration `mapstructure:"account_lock_duration" json:"account_lock_duration"`
	PasswordMinLength       int           `mapstructure:"password_min_length" json:"password_min_length"`
	PasswordMaxLength       int           `mapstructure:"password_max_length" json:"password_max_length"`
	RequireStrongPassword   bool          `mapstructure:"require_strong_password" json:"require_strong_password"`
	EnableTwoFactor         bool          `mapstructure:"enable_two_factor" json:"enable_two_factor"`
	EnableEmailVerification bool          `mapstructure:"enable_email_verification" json:"enable_email_verification"`
	EnablePasswordReset     bool          `mapstructure:"enable_password_reset" json:"enable_password_reset"`
	EnableSessionCleanup    bool          `mapstructure:"enable_session_cleanup" json:"enable_session_cleanup"`
	SessionCleanupInterval  time.Duration `mapstructure:"session_cleanup_interval" json:"session_cleanup_interval"`
	EnableAuditLogging      bool          `mapstructure:"enable_audit_logging" json:"enable_audit_logging"`
	EnableRateLimiting      bool          `mapstructure:"enable_rate_limiting" json:"enable_rate_limiting"`
	BCryptCost              int           `mapstructure:"bcrypt_cost" json:"bcrypt_cost"`
	ApiKeyLength            int           `mapstructure:"api_key_length" json:"api_key_length"`
	ApiKeyPrefix            string        `mapstructure:"api_key_prefix" json:"api_key_prefix"`
	DefaultRole             string        `mapstructure:"default_role" json:"default_role"`
}

// KafkaConfig represents Kafka configuration
type KafkaConfig struct {
	Brokers              []string      `mapstructure:"brokers" json:"brokers" validate:"required"`
	ReviewTopic          string        `mapstructure:"review_topic" json:"review_topic" validate:"required"`
	ProcessingTopic      string        `mapstructure:"processing_topic" json:"processing_topic" validate:"required"`
	DeadLetterTopic      string        `mapstructure:"dead_letter_topic" json:"dead_letter_topic"`
	ConsumerGroup        string        `mapstructure:"consumer_group" json:"consumer_group" validate:"required"`
	BatchSize            int           `mapstructure:"batch_size" json:"batch_size"`
	BatchTimeout         time.Duration `mapstructure:"batch_timeout" json:"batch_timeout"`
	MaxRetries           int           `mapstructure:"max_retries" json:"max_retries"`
	RetryDelay           time.Duration `mapstructure:"retry_delay" json:"retry_delay"`
	EnableSASL           bool          `mapstructure:"enable_sasl" json:"enable_sasl"`
	SASLUsername         string        `mapstructure:"sasl_username" json:"sasl_username"`
	SASLPassword         string        `mapstructure:"sasl_password" json:"sasl_password"`
	EnableTLS            bool          `mapstructure:"enable_tls" json:"enable_tls"`
	MaxMessageSize       int           `mapstructure:"max_message_size" json:"max_message_size"`
	CompressionType      string        `mapstructure:"compression_type" json:"compression_type"`
	ProducerFlushTimeout time.Duration `mapstructure:"producer_flush_timeout" json:"producer_flush_timeout"`
	ConsumerTimeout      time.Duration `mapstructure:"consumer_timeout" json:"consumer_timeout"`
	EnableIdempotence    bool          `mapstructure:"enable_idempotence" json:"enable_idempotence"`
	Partitions           int           `mapstructure:"partitions" json:"partitions"`
	ReplicationFactor    int           `mapstructure:"replication_factor" json:"replication_factor"`
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Configure viper
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/hotel-reviews")
	v.AddConfigPath("$HOME/.hotel-reviews")

	// Enable environment variable support
	v.SetEnvPrefix("HOTEL_REVIEWS")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicitly bind environment variables for required fields
	bindEnvironmentVariables(v)

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, continue with environment variables and defaults
	}

	// Unmarshal configuration
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// bindEnvironmentVariables explicitly binds environment variables to viper keys
func bindEnvironmentVariables(v *viper.Viper) {
	// Database configuration
	v.BindEnv("database.host", "HOTEL_REVIEWS_DATABASE_HOST")
	v.BindEnv("database.port", "HOTEL_REVIEWS_DATABASE_PORT")
	v.BindEnv("database.user", "HOTEL_REVIEWS_DATABASE_USER")
	v.BindEnv("database.password", "HOTEL_REVIEWS_DATABASE_PASSWORD")
	v.BindEnv("database.name", "HOTEL_REVIEWS_DATABASE_NAME")
	v.BindEnv("database.ssl_mode", "HOTEL_REVIEWS_DATABASE_SSL_MODE")
	v.BindEnv("database.max_open_conns", "HOTEL_REVIEWS_DATABASE_MAX_OPEN_CONNS")
	v.BindEnv("database.max_idle_conns", "HOTEL_REVIEWS_DATABASE_MAX_IDLE_CONNS")
	v.BindEnv("database.conn_max_lifetime", "HOTEL_REVIEWS_DATABASE_CONN_MAX_LIFETIME")
	v.BindEnv("database.conn_max_idle_time", "HOTEL_REVIEWS_DATABASE_CONN_MAX_IDLE_TIME")
	v.BindEnv("database.timezone", "HOTEL_REVIEWS_DATABASE_TIMEZONE")
	v.BindEnv("database.log_level", "HOTEL_REVIEWS_DATABASE_LOG_LEVEL")

	// S3 configuration
	v.BindEnv("s3.region", "HOTEL_REVIEWS_S3_REGION")
	v.BindEnv("s3.access_key_id", "HOTEL_REVIEWS_S3_ACCESS_KEY_ID")
	v.BindEnv("s3.secret_access_key", "HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY")
	v.BindEnv("s3.session_token", "HOTEL_REVIEWS_S3_SESSION_TOKEN")
	v.BindEnv("s3.bucket", "HOTEL_REVIEWS_S3_BUCKET")
	v.BindEnv("s3.endpoint", "HOTEL_REVIEWS_S3_ENDPOINT")
	v.BindEnv("s3.use_ssl", "HOTEL_REVIEWS_S3_USE_SSL")
	v.BindEnv("s3.force_path_style", "HOTEL_REVIEWS_S3_FORCE_PATH_STYLE")
	v.BindEnv("s3.timeout", "HOTEL_REVIEWS_S3_TIMEOUT")
	v.BindEnv("s3.retry_count", "HOTEL_REVIEWS_S3_RETRY_COUNT")
	v.BindEnv("s3.retry_delay", "HOTEL_REVIEWS_S3_RETRY_DELAY")
	v.BindEnv("s3.upload_part_size", "HOTEL_REVIEWS_S3_UPLOAD_PART_SIZE")
	v.BindEnv("s3.download_part_size", "HOTEL_REVIEWS_S3_DOWNLOAD_PART_SIZE")

	// Server configuration
	v.BindEnv("server.host", "HOTEL_REVIEWS_SERVER_HOST")
	v.BindEnv("server.port", "HOTEL_REVIEWS_SERVER_PORT")
	v.BindEnv("server.read_timeout", "HOTEL_REVIEWS_SERVER_READ_TIMEOUT")
	v.BindEnv("server.write_timeout", "HOTEL_REVIEWS_SERVER_WRITE_TIMEOUT")
	v.BindEnv("server.idle_timeout", "HOTEL_REVIEWS_SERVER_IDLE_TIMEOUT")
	v.BindEnv("server.shutdown_timeout", "HOTEL_REVIEWS_SERVER_SHUTDOWN_TIMEOUT")
	v.BindEnv("server.max_header_bytes", "HOTEL_REVIEWS_SERVER_MAX_HEADER_BYTES")
	v.BindEnv("server.enable_cors", "HOTEL_REVIEWS_SERVER_ENABLE_CORS")
	v.BindEnv("server.enable_gzip", "HOTEL_REVIEWS_SERVER_ENABLE_GZIP")
	v.BindEnv("server.enable_metrics", "HOTEL_REVIEWS_SERVER_ENABLE_METRICS")
	v.BindEnv("server.enable_pprof", "HOTEL_REVIEWS_SERVER_ENABLE_PPROF")
	v.BindEnv("server.tls_cert_file", "HOTEL_REVIEWS_SERVER_TLS_CERT_FILE")
	v.BindEnv("server.tls_key_file", "HOTEL_REVIEWS_SERVER_TLS_KEY_FILE")
	v.BindEnv("server.trusted_proxies", "HOTEL_REVIEWS_SERVER_TRUSTED_PROXIES")

	// Log configuration
	v.BindEnv("log.level", "HOTEL_REVIEWS_LOG_LEVEL")
	v.BindEnv("log.format", "HOTEL_REVIEWS_LOG_FORMAT")
	v.BindEnv("log.output", "HOTEL_REVIEWS_LOG_OUTPUT")
	v.BindEnv("log.file_path", "HOTEL_REVIEWS_LOG_FILE_PATH")
	v.BindEnv("log.max_size", "HOTEL_REVIEWS_LOG_MAX_SIZE")
	v.BindEnv("log.max_backups", "HOTEL_REVIEWS_LOG_MAX_BACKUPS")
	v.BindEnv("log.max_age", "HOTEL_REVIEWS_LOG_MAX_AGE")
	v.BindEnv("log.compress", "HOTEL_REVIEWS_LOG_COMPRESS")
	v.BindEnv("log.enable_caller", "HOTEL_REVIEWS_LOG_ENABLE_CALLER")
	v.BindEnv("log.enable_stacktrace", "HOTEL_REVIEWS_LOG_ENABLE_STACKTRACE")

	// Cache configuration
	v.BindEnv("cache.type", "HOTEL_REVIEWS_CACHE_TYPE")
	v.BindEnv("cache.host", "HOTEL_REVIEWS_CACHE_HOST")
	v.BindEnv("cache.port", "HOTEL_REVIEWS_CACHE_PORT")
	v.BindEnv("cache.password", "HOTEL_REVIEWS_CACHE_PASSWORD")
	v.BindEnv("cache.database", "HOTEL_REVIEWS_CACHE_DATABASE")
	v.BindEnv("cache.pool_size", "HOTEL_REVIEWS_CACHE_POOL_SIZE")
	v.BindEnv("cache.min_idle_conns", "HOTEL_REVIEWS_CACHE_MIN_IDLE_CONNS")
	v.BindEnv("cache.dial_timeout", "HOTEL_REVIEWS_CACHE_DIAL_TIMEOUT")
	v.BindEnv("cache.read_timeout", "HOTEL_REVIEWS_CACHE_READ_TIMEOUT")
	v.BindEnv("cache.write_timeout", "HOTEL_REVIEWS_CACHE_WRITE_TIMEOUT")
	v.BindEnv("cache.ttl", "HOTEL_REVIEWS_CACHE_TTL")
	v.BindEnv("cache.max_memory", "HOTEL_REVIEWS_CACHE_MAX_MEMORY")

	// Metrics configuration
	v.BindEnv("metrics.enabled", "HOTEL_REVIEWS_METRICS_ENABLED")
	v.BindEnv("metrics.type", "HOTEL_REVIEWS_METRICS_TYPE")
	v.BindEnv("metrics.host", "HOTEL_REVIEWS_METRICS_HOST")
	v.BindEnv("metrics.port", "HOTEL_REVIEWS_METRICS_PORT")
	v.BindEnv("metrics.path", "HOTEL_REVIEWS_METRICS_PATH")
	v.BindEnv("metrics.namespace", "HOTEL_REVIEWS_METRICS_NAMESPACE")
	v.BindEnv("metrics.service_name", "HOTEL_REVIEWS_METRICS_SERVICE_NAME")
	v.BindEnv("metrics.environment", "HOTEL_REVIEWS_METRICS_ENVIRONMENT")
	v.BindEnv("metrics.version", "HOTEL_REVIEWS_METRICS_VERSION")

	// Notification configuration
	v.BindEnv("notification.email.enabled", "HOTEL_REVIEWS_NOTIFICATION_EMAIL_ENABLED")
	v.BindEnv("notification.email.host", "HOTEL_REVIEWS_NOTIFICATION_EMAIL_HOST")
	v.BindEnv("notification.email.port", "HOTEL_REVIEWS_NOTIFICATION_EMAIL_PORT")
	v.BindEnv("notification.email.username", "HOTEL_REVIEWS_NOTIFICATION_EMAIL_USERNAME")
	v.BindEnv("notification.email.password", "HOTEL_REVIEWS_NOTIFICATION_EMAIL_PASSWORD")
	v.BindEnv("notification.email.from", "HOTEL_REVIEWS_NOTIFICATION_EMAIL_FROM")
	v.BindEnv("notification.email.use_tls", "HOTEL_REVIEWS_NOTIFICATION_EMAIL_USE_TLS")
	v.BindEnv("notification.slack.enabled", "HOTEL_REVIEWS_NOTIFICATION_SLACK_ENABLED")
	v.BindEnv("notification.slack.webhook_url", "HOTEL_REVIEWS_NOTIFICATION_SLACK_WEBHOOK_URL")
	v.BindEnv("notification.slack.channel", "HOTEL_REVIEWS_NOTIFICATION_SLACK_CHANNEL")
	v.BindEnv("notification.slack.username", "HOTEL_REVIEWS_NOTIFICATION_SLACK_USERNAME")
	v.BindEnv("notification.slack.icon_emoji", "HOTEL_REVIEWS_NOTIFICATION_SLACK_ICON_EMOJI")
	v.BindEnv("notification.slack.icon_url", "HOTEL_REVIEWS_NOTIFICATION_SLACK_ICON_URL")

	// Processing configuration
	v.BindEnv("processing.batch_size", "HOTEL_REVIEWS_PROCESSING_BATCH_SIZE")
	v.BindEnv("processing.worker_count", "HOTEL_REVIEWS_PROCESSING_WORKER_COUNT")
	v.BindEnv("processing.max_file_size", "HOTEL_REVIEWS_PROCESSING_MAX_FILE_SIZE")
	v.BindEnv("processing.processing_timeout", "HOTEL_REVIEWS_PROCESSING_PROCESSING_TIMEOUT")
	v.BindEnv("processing.max_retries", "HOTEL_REVIEWS_PROCESSING_MAX_RETRIES")
	v.BindEnv("processing.retry_delay", "HOTEL_REVIEWS_PROCESSING_RETRY_DELAY")
	v.BindEnv("processing.enable_validation", "HOTEL_REVIEWS_PROCESSING_ENABLE_VALIDATION")
	v.BindEnv("processing.enable_duplicate_check", "HOTEL_REVIEWS_PROCESSING_ENABLE_DUPLICATE_CHECK")
	v.BindEnv("processing.temp_directory", "HOTEL_REVIEWS_PROCESSING_TEMP_DIRECTORY")

	// Security configuration
	v.BindEnv("security.jwt_secret", "HOTEL_REVIEWS_SECURITY_JWT_SECRET")
	v.BindEnv("security.jwt_expiration", "HOTEL_REVIEWS_SECURITY_JWT_EXPIRATION")
	v.BindEnv("security.rate_limit", "HOTEL_REVIEWS_SECURITY_RATE_LIMIT")
	v.BindEnv("security.rate_limit_window", "HOTEL_REVIEWS_SECURITY_RATE_LIMIT_WINDOW")
	v.BindEnv("security.enable_api_key", "HOTEL_REVIEWS_SECURITY_ENABLE_API_KEY")
	v.BindEnv("security.api_key_header", "HOTEL_REVIEWS_SECURITY_API_KEY_HEADER")
	v.BindEnv("security.enable_encryption", "HOTEL_REVIEWS_SECURITY_ENABLE_ENCRYPTION")
	v.BindEnv("security.encryption_key", "HOTEL_REVIEWS_SECURITY_ENCRYPTION_KEY")

	// Authentication configuration
	v.BindEnv("auth.jwt_secret", "HOTEL_REVIEWS_AUTH_JWT_SECRET")
	v.BindEnv("auth.jwt_issuer", "HOTEL_REVIEWS_AUTH_JWT_ISSUER")
	v.BindEnv("auth.access_token_expiry", "HOTEL_REVIEWS_AUTH_ACCESS_TOKEN_EXPIRY")
	v.BindEnv("auth.refresh_token_expiry", "HOTEL_REVIEWS_AUTH_REFRESH_TOKEN_EXPIRY")
	v.BindEnv("auth.max_login_attempts", "HOTEL_REVIEWS_AUTH_MAX_LOGIN_ATTEMPTS")
	v.BindEnv("auth.login_attempt_window", "HOTEL_REVIEWS_AUTH_LOGIN_ATTEMPT_WINDOW")
	v.BindEnv("auth.account_lock_duration", "HOTEL_REVIEWS_AUTH_ACCOUNT_LOCK_DURATION")
	v.BindEnv("auth.password_min_length", "HOTEL_REVIEWS_AUTH_PASSWORD_MIN_LENGTH")
	v.BindEnv("auth.password_max_length", "HOTEL_REVIEWS_AUTH_PASSWORD_MAX_LENGTH")
	v.BindEnv("auth.require_strong_password", "HOTEL_REVIEWS_AUTH_REQUIRE_STRONG_PASSWORD")
	v.BindEnv("auth.enable_two_factor", "HOTEL_REVIEWS_AUTH_ENABLE_TWO_FACTOR")
	v.BindEnv("auth.enable_email_verification", "HOTEL_REVIEWS_AUTH_ENABLE_EMAIL_VERIFICATION")
	v.BindEnv("auth.enable_password_reset", "HOTEL_REVIEWS_AUTH_ENABLE_PASSWORD_RESET")
	v.BindEnv("auth.enable_session_cleanup", "HOTEL_REVIEWS_AUTH_ENABLE_SESSION_CLEANUP")
	v.BindEnv("auth.session_cleanup_interval", "HOTEL_REVIEWS_AUTH_SESSION_CLEANUP_INTERVAL")
	v.BindEnv("auth.enable_audit_logging", "HOTEL_REVIEWS_AUTH_ENABLE_AUDIT_LOGGING")
	v.BindEnv("auth.enable_rate_limiting", "HOTEL_REVIEWS_AUTH_ENABLE_RATE_LIMITING")
	v.BindEnv("auth.bcrypt_cost", "HOTEL_REVIEWS_AUTH_BCRYPT_COST")
	v.BindEnv("auth.api_key_length", "HOTEL_REVIEWS_AUTH_API_KEY_LENGTH")
	v.BindEnv("auth.api_key_prefix", "HOTEL_REVIEWS_AUTH_API_KEY_PREFIX")
	v.BindEnv("auth.default_role", "HOTEL_REVIEWS_AUTH_DEFAULT_ROLE")

	// Kafka configuration
	v.BindEnv("kafka.brokers", "HOTEL_REVIEWS_KAFKA_BROKERS")
	v.BindEnv("kafka.review_topic", "HOTEL_REVIEWS_KAFKA_REVIEW_TOPIC")
	v.BindEnv("kafka.processing_topic", "HOTEL_REVIEWS_KAFKA_PROCESSING_TOPIC")
	v.BindEnv("kafka.dead_letter_topic", "HOTEL_REVIEWS_KAFKA_DEAD_LETTER_TOPIC")
	v.BindEnv("kafka.consumer_group", "HOTEL_REVIEWS_KAFKA_CONSUMER_GROUP")
	v.BindEnv("kafka.batch_size", "HOTEL_REVIEWS_KAFKA_BATCH_SIZE")
	v.BindEnv("kafka.batch_timeout", "HOTEL_REVIEWS_KAFKA_BATCH_TIMEOUT")
	v.BindEnv("kafka.max_retries", "HOTEL_REVIEWS_KAFKA_MAX_RETRIES")
	v.BindEnv("kafka.retry_delay", "HOTEL_REVIEWS_KAFKA_RETRY_DELAY")
	v.BindEnv("kafka.enable_sasl", "HOTEL_REVIEWS_KAFKA_ENABLE_SASL")
	v.BindEnv("kafka.sasl_username", "HOTEL_REVIEWS_KAFKA_SASL_USERNAME")
	v.BindEnv("kafka.sasl_password", "HOTEL_REVIEWS_KAFKA_SASL_PASSWORD")
	v.BindEnv("kafka.enable_tls", "HOTEL_REVIEWS_KAFKA_ENABLE_TLS")
	v.BindEnv("kafka.max_message_size", "HOTEL_REVIEWS_KAFKA_MAX_MESSAGE_SIZE")
	v.BindEnv("kafka.compression_type", "HOTEL_REVIEWS_KAFKA_COMPRESSION_TYPE")
	v.BindEnv("kafka.producer_flush_timeout", "HOTEL_REVIEWS_KAFKA_PRODUCER_FLUSH_TIMEOUT")
	v.BindEnv("kafka.consumer_timeout", "HOTEL_REVIEWS_KAFKA_CONSUMER_TIMEOUT")
	v.BindEnv("kafka.enable_idempotence", "HOTEL_REVIEWS_KAFKA_ENABLE_IDEMPOTENCE")
	v.BindEnv("kafka.partitions", "HOTEL_REVIEWS_KAFKA_PARTITIONS")
	v.BindEnv("kafka.replication_factor", "HOTEL_REVIEWS_KAFKA_REPLICATION_FACTOR")
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Database defaults (required fields have no defaults)
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 25)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.conn_max_idle_time", "5m")
	v.SetDefault("database.timezone", "UTC")
	v.SetDefault("database.log_level", "warn")

	// S3 defaults (required fields have no defaults)
	v.SetDefault("s3.use_ssl", true)
	v.SetDefault("s3.force_path_style", false)
	v.SetDefault("s3.timeout", "30s")
	v.SetDefault("s3.retry_count", 3)
	v.SetDefault("s3.retry_delay", "1s")
	v.SetDefault("s3.upload_part_size", 5*1024*1024)   // 5MB
	v.SetDefault("s3.download_part_size", 5*1024*1024) // 5MB

	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("server.max_header_bytes", 1<<20) // 1MB
	v.SetDefault("server.enable_cors", true)
	v.SetDefault("server.enable_gzip", true)
	v.SetDefault("server.enable_metrics", true)
	v.SetDefault("server.enable_pprof", false)
	v.SetDefault("server.trusted_proxies", []string{})

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 3)
	v.SetDefault("log.max_age", 28)
	v.SetDefault("log.compress", true)
	v.SetDefault("log.enable_caller", true)
	v.SetDefault("log.enable_stacktrace", false)

	// Cache defaults
	v.SetDefault("cache.type", "redis")
	v.SetDefault("cache.host", "localhost")
	v.SetDefault("cache.port", 6379)
	v.SetDefault("cache.database", 0)
	v.SetDefault("cache.pool_size", 10)
	v.SetDefault("cache.min_idle_conns", 5)
	v.SetDefault("cache.dial_timeout", "5s")
	v.SetDefault("cache.read_timeout", "3s")
	v.SetDefault("cache.write_timeout", "3s")
	v.SetDefault("cache.ttl", "1h")
	v.SetDefault("cache.max_memory", 100*1024*1024) // 100MB

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.type", "prometheus")
	v.SetDefault("metrics.host", "localhost")
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.path", "/metrics")
	v.SetDefault("metrics.namespace", "hotel_reviews")
	v.SetDefault("metrics.service_name", "hotel-reviews-api")
	v.SetDefault("metrics.environment", "development")
	v.SetDefault("metrics.version", "1.0.0")

	// Notification defaults
	v.SetDefault("notification.email.enabled", false)
	v.SetDefault("notification.email.port", 587)
	v.SetDefault("notification.email.use_tls", true)
	v.SetDefault("notification.slack.enabled", false)
	v.SetDefault("notification.slack.username", "Hotel Reviews Bot")
	v.SetDefault("notification.slack.icon_emoji", ":hotel:")

	// Processing defaults
	v.SetDefault("processing.batch_size", 1000)
	v.SetDefault("processing.worker_count", 4)
	v.SetDefault("processing.max_file_size", 100*1024*1024) // 100MB
	v.SetDefault("processing.processing_timeout", "30m")
	v.SetDefault("processing.max_retries", 3)
	v.SetDefault("processing.retry_delay", "5s")
	v.SetDefault("processing.enable_validation", true)
	v.SetDefault("processing.enable_duplicate_check", true)
	v.SetDefault("processing.temp_directory", "/tmp/hotel-reviews")

	// Security defaults
	v.SetDefault("security.jwt_expiration", "24h")
	v.SetDefault("security.rate_limit", 1000)
	v.SetDefault("security.rate_limit_window", "1h")
	v.SetDefault("security.enable_api_key", false)
	v.SetDefault("security.api_key_header", "X-API-Key")
	v.SetDefault("security.enable_encryption", false)

	// Authentication defaults
	v.SetDefault("auth.jwt_issuer", "hotel-reviews-api")
	v.SetDefault("auth.access_token_expiry", "15m")
	v.SetDefault("auth.refresh_token_expiry", "168h") // 7 days = 7 * 24 hours
	v.SetDefault("auth.max_login_attempts", 5)
	v.SetDefault("auth.login_attempt_window", "15m")
	v.SetDefault("auth.account_lock_duration", "30m")
	v.SetDefault("auth.password_min_length", 8)
	v.SetDefault("auth.password_max_length", 128)
	v.SetDefault("auth.require_strong_password", true)
	v.SetDefault("auth.enable_two_factor", false)
	v.SetDefault("auth.enable_email_verification", false)
	v.SetDefault("auth.enable_password_reset", true)
	v.SetDefault("auth.enable_session_cleanup", true)
	v.SetDefault("auth.session_cleanup_interval", "1h")
	v.SetDefault("auth.enable_audit_logging", true)
	v.SetDefault("auth.enable_rate_limiting", true)
	v.SetDefault("auth.bcrypt_cost", 12)
	v.SetDefault("auth.api_key_length", 32)
	v.SetDefault("auth.api_key_prefix", "hr_")
	v.SetDefault("auth.default_role", "user")

	// Kafka defaults
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.review_topic", "hotel-reviews")
	v.SetDefault("kafka.processing_topic", "hotel-reviews-processing")
	v.SetDefault("kafka.dead_letter_topic", "hotel-reviews-dlq")
	v.SetDefault("kafka.consumer_group", "hotel-reviews-consumer")
	v.SetDefault("kafka.batch_size", 100)
	v.SetDefault("kafka.batch_timeout", "1s")
	v.SetDefault("kafka.max_retries", 3)
	v.SetDefault("kafka.retry_delay", "1s")
	v.SetDefault("kafka.enable_sasl", false)
	v.SetDefault("kafka.enable_tls", false)
	v.SetDefault("kafka.max_message_size", 1*1024*1024) // 1MB
	v.SetDefault("kafka.compression_type", "snappy")
	v.SetDefault("kafka.producer_flush_timeout", "10s")
	v.SetDefault("kafka.consumer_timeout", "10s")
	v.SetDefault("kafka.enable_idempotence", true)
	v.SetDefault("kafka.partitions", 3)
	v.SetDefault("kafka.replication_factor", 1)
}

// validate validates the configuration
func validate(config *Config) error {
	// Validate database configuration
	if config.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if config.Database.Port <= 0 || config.Database.Port > 65535 {
		return fmt.Errorf("database port must be between 1 and 65535")
	}
	if config.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if config.Database.Password == "" {
		return fmt.Errorf("database password is required")
	}
	if config.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}

	// Validate S3 configuration
	if config.S3.Region == "" {
		return fmt.Errorf("S3 region is required")
	}
	if config.S3.AccessKeyID == "" {
		return fmt.Errorf("S3 access key ID is required")
	}
	if config.S3.SecretAccessKey == "" {
		return fmt.Errorf("S3 secret access key is required")
	}
	if config.S3.Bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}

	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}

	// Validate log configuration
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, config.Log.Level) {
		return fmt.Errorf("log level must be one of: %s", strings.Join(validLogLevels, ", "))
	}

	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, config.Log.Format) {
		return fmt.Errorf("log format must be one of: %s", strings.Join(validLogFormats, ", "))
	}

	validLogOutputs := []string{"stdout", "stderr", "file"}
	if !contains(validLogOutputs, config.Log.Output) {
		return fmt.Errorf("log output must be one of: %s", strings.Join(validLogOutputs, ", "))
	}

	if config.Log.Output == "file" && config.Log.FilePath == "" {
		return fmt.Errorf("log file path is required when output is file")
	}

	// Validate cache configuration
	validCacheTypes := []string{"redis", "memory"}
	if !contains(validCacheTypes, config.Cache.Type) {
		return fmt.Errorf("cache type must be one of: %s", strings.Join(validCacheTypes, ", "))
	}

	// Validate security configuration
	if config.Security.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required")
	}

	// Validate authentication configuration
	if config.Auth.JWTSecret == "" {
		return fmt.Errorf("authentication JWT secret is required")
	}
	if config.Auth.JWTIssuer == "" {
		return fmt.Errorf("authentication JWT issuer is required")
	}
	if config.Auth.AccessTokenExpiry <= 0 {
		return fmt.Errorf("access token expiry must be positive")
	}
	if config.Auth.RefreshTokenExpiry <= 0 {
		return fmt.Errorf("refresh token expiry must be positive")
	}
	if config.Auth.MaxLoginAttempts <= 0 {
		return fmt.Errorf("max login attempts must be positive")
	}
	if config.Auth.LoginAttemptWindow <= 0 {
		return fmt.Errorf("login attempt window must be positive")
	}
	if config.Auth.AccountLockDuration <= 0 {
		return fmt.Errorf("account lock duration must be positive")
	}
	if config.Auth.PasswordMinLength < 4 {
		return fmt.Errorf("password min length must be at least 4")
	}
	if config.Auth.PasswordMaxLength < config.Auth.PasswordMinLength {
		return fmt.Errorf("password max length must be greater than or equal to min length")
	}
	if config.Auth.BCryptCost < 4 || config.Auth.BCryptCost > 31 {
		return fmt.Errorf("bcrypt cost must be between 4 and 31")
	}
	if config.Auth.ApiKeyLength < 16 {
		return fmt.Errorf("API key length must be at least 16")
	}
	if config.Auth.ApiKeyPrefix == "" {
		return fmt.Errorf("API key prefix is required")
	}
	if config.Auth.DefaultRole == "" {
		return fmt.Errorf("default role is required")
	}

	// Validate TLS configuration
	if config.Server.TLSCertFile != "" && config.Server.TLSKeyFile == "" {
		return fmt.Errorf("TLS key file is required when TLS cert file is provided")
	}
	if config.Server.TLSKeyFile != "" && config.Server.TLSCertFile == "" {
		return fmt.Errorf("TLS cert file is required when TLS key file is provided")
	}

	// Validate file paths exist
	if config.Server.TLSCertFile != "" {
		if _, err := os.Stat(config.Server.TLSCertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file does not exist: %s", config.Server.TLSCertFile)
		}
	}
	if config.Server.TLSKeyFile != "" {
		if _, err := os.Stat(config.Server.TLSKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file does not exist: %s", config.Server.TLSKeyFile)
		}
	}

	// Validate Kafka configuration
	if len(config.Kafka.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	if config.Kafka.ReviewTopic == "" {
		return fmt.Errorf("Kafka review topic is required")
	}
	if config.Kafka.ProcessingTopic == "" {
		return fmt.Errorf("Kafka processing topic is required")
	}
	if config.Kafka.ConsumerGroup == "" {
		return fmt.Errorf("Kafka consumer group is required")
	}
	if config.Kafka.BatchSize <= 0 {
		return fmt.Errorf("Kafka batch size must be positive")
	}
	if config.Kafka.MaxRetries < 0 {
		return fmt.Errorf("Kafka max retries must be non-negative")
	}
	if config.Kafka.MaxMessageSize <= 0 {
		return fmt.Errorf("Kafka max message size must be positive")
	}
	if config.Kafka.Partitions <= 0 {
		return fmt.Errorf("Kafka partitions must be positive")
	}
	if config.Kafka.ReplicationFactor <= 0 {
		return fmt.Errorf("Kafka replication factor must be positive")
	}

	// Validate Kafka compression type
	validCompressionTypes := []string{"gzip", "snappy", "lz4", "zstd", "none"}
	if !contains(validCompressionTypes, config.Kafka.CompressionType) {
		return fmt.Errorf("Kafka compression type must be one of: %s", strings.Join(validCompressionTypes, ", "))
	}

	// Validate SASL configuration
	if config.Kafka.EnableSASL {
		if config.Kafka.SASLUsername == "" {
			return fmt.Errorf("SASL username is required when SASL is enabled")
		}
		if config.Kafka.SASLPassword == "" {
			return fmt.Errorf("SASL password is required when SASL is enabled")
		}
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetDatabaseURL returns the database connection URL
func (c *Config) GetDatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&timezone=%s",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
		c.Database.SSLMode,
		c.Database.TimeZone,
	)
}

// GetServerAddress returns the server address
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetCacheAddress returns the cache address
func (c *Config) GetCacheAddress() string {
	return fmt.Sprintf("%s:%d", c.Cache.Host, c.Cache.Port)
}

// IsProduction returns true if the environment is production
func (c *Config) IsProduction() bool {
	return c.Metrics.Environment == "production"
}

// IsDevelopment returns true if the environment is development
func (c *Config) IsDevelopment() bool {
	return c.Metrics.Environment == "development"
}

// IsTestEnvironment returns true if the environment is test
func (c *Config) IsTestEnvironment() bool {
	return c.Metrics.Environment == "test"
}
