package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ConfigTestSuite for testing config functionality
type ConfigTestSuite struct {
	suite.Suite
	tmpDir string
}

func (suite *ConfigTestSuite) SetupTest() {
	// Create temporary directory for test files
	var err error
	suite.tmpDir, err = os.MkdirTemp("", "config_test")
	suite.NoError(err)
	
	// Clean up only our environment variables before each test
	suite.cleanupTestEnvVars()
}

func (suite *ConfigTestSuite) TearDownTest() {
	if suite.tmpDir != "" {
		os.RemoveAll(suite.tmpDir)
	}
	suite.cleanupTestEnvVars()
}

func (suite *ConfigTestSuite) cleanupTestEnvVars() {
	// Clean up only our test environment variables
	testEnvVars := []string{
		"HOTEL_REVIEWS_DATABASE_HOST",
		"HOTEL_REVIEWS_DATABASE_PORT", 
		"HOTEL_REVIEWS_DATABASE_USER",
		"HOTEL_REVIEWS_DATABASE_PASSWORD",
		"HOTEL_REVIEWS_DATABASE_NAME",
		"HOTEL_REVIEWS_DATABASE_SSL_MODE",
		"HOTEL_REVIEWS_DATABASE_MAX_OPEN_CONNS",
		"HOTEL_REVIEWS_DATABASE_MAX_IDLE_CONNS",
		"HOTEL_REVIEWS_DATABASE_CONN_MAX_LIFETIME",
		"HOTEL_REVIEWS_DATABASE_CONN_MAX_IDLE_TIME",
		"HOTEL_REVIEWS_DATABASE_TIMEZONE",
		"HOTEL_REVIEWS_DATABASE_LOG_LEVEL",
		"HOTEL_REVIEWS_S3_REGION",
		"HOTEL_REVIEWS_S3_ACCESS_KEY_ID",
		"HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY",
		"HOTEL_REVIEWS_S3_BUCKET",
		"HOTEL_REVIEWS_S3_ENDPOINT",
		"HOTEL_REVIEWS_S3_USE_SSL",
		"HOTEL_REVIEWS_S3_FORCE_PATH_STYLE",
		"HOTEL_REVIEWS_S3_TIMEOUT",
		"HOTEL_REVIEWS_S3_RETRY_COUNT",
		"HOTEL_REVIEWS_S3_RETRY_DELAY",
		"HOTEL_REVIEWS_S3_UPLOAD_PART_SIZE",
		"HOTEL_REVIEWS_S3_DOWNLOAD_PART_SIZE",
		"HOTEL_REVIEWS_SERVER_HOST",
		"HOTEL_REVIEWS_SERVER_PORT",
		"HOTEL_REVIEWS_SERVER_READ_TIMEOUT",
		"HOTEL_REVIEWS_SERVER_WRITE_TIMEOUT",
		"HOTEL_REVIEWS_SERVER_IDLE_TIMEOUT",
		"HOTEL_REVIEWS_SERVER_SHUTDOWN_TIMEOUT",
		"HOTEL_REVIEWS_SERVER_MAX_HEADER_BYTES",
		"HOTEL_REVIEWS_SERVER_ENABLE_CORS",
		"HOTEL_REVIEWS_SERVER_ENABLE_GZIP",
		"HOTEL_REVIEWS_SERVER_ENABLE_METRICS",
		"HOTEL_REVIEWS_SERVER_ENABLE_PPROF",
		"HOTEL_REVIEWS_SERVER_TLS_CERT_FILE",
		"HOTEL_REVIEWS_SERVER_TLS_KEY_FILE",
		"HOTEL_REVIEWS_SERVER_TRUSTED_PROXIES",
		"HOTEL_REVIEWS_LOG_LEVEL",
		"HOTEL_REVIEWS_LOG_FORMAT",
		"HOTEL_REVIEWS_LOG_OUTPUT",
		"HOTEL_REVIEWS_LOG_FILE_PATH",
		"HOTEL_REVIEWS_LOG_MAX_SIZE",
		"HOTEL_REVIEWS_LOG_MAX_BACKUPS",
		"HOTEL_REVIEWS_LOG_MAX_AGE",
		"HOTEL_REVIEWS_LOG_COMPRESS",
		"HOTEL_REVIEWS_LOG_ENABLE_CALLER",
		"HOTEL_REVIEWS_LOG_ENABLE_STACKTRACE",
		"HOTEL_REVIEWS_CACHE_TYPE",
		"HOTEL_REVIEWS_CACHE_HOST",
		"HOTEL_REVIEWS_CACHE_PORT",
		"HOTEL_REVIEWS_CACHE_PASSWORD",
		"HOTEL_REVIEWS_CACHE_DATABASE",
		"HOTEL_REVIEWS_CACHE_POOL_SIZE",
		"HOTEL_REVIEWS_CACHE_MIN_IDLE_CONNS",
		"HOTEL_REVIEWS_CACHE_DIAL_TIMEOUT",
		"HOTEL_REVIEWS_CACHE_READ_TIMEOUT",
		"HOTEL_REVIEWS_CACHE_WRITE_TIMEOUT",
		"HOTEL_REVIEWS_CACHE_TTL",
		"HOTEL_REVIEWS_CACHE_MAX_MEMORY",
		"HOTEL_REVIEWS_METRICS_ENABLED",
		"HOTEL_REVIEWS_METRICS_TYPE",
		"HOTEL_REVIEWS_METRICS_HOST",
		"HOTEL_REVIEWS_METRICS_PORT",
		"HOTEL_REVIEWS_METRICS_PATH",
		"HOTEL_REVIEWS_METRICS_NAMESPACE",
		"HOTEL_REVIEWS_METRICS_SERVICE_NAME",
		"HOTEL_REVIEWS_METRICS_ENVIRONMENT",
		"HOTEL_REVIEWS_METRICS_VERSION",
		"HOTEL_REVIEWS_SECURITY_JWT_SECRET",
		"HOTEL_REVIEWS_SECURITY_JWT_EXPIRATION",
		"HOTEL_REVIEWS_SECURITY_RATE_LIMIT",
		"HOTEL_REVIEWS_SECURITY_RATE_LIMIT_WINDOW",
		"HOTEL_REVIEWS_SECURITY_ENABLE_API_KEY",
		"HOTEL_REVIEWS_SECURITY_API_KEY_HEADER",
		"HOTEL_REVIEWS_SECURITY_ENABLE_ENCRYPTION",
		"HOTEL_REVIEWS_SECURITY_ENCRYPTION_KEY",
	}
	
	for _, envVar := range testEnvVars {
		os.Unsetenv(envVar)
	}
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

// Test basic config loading with defaults
func (suite *ConfigTestSuite) TestLoad_WithDefaults() {
	// Set all required environment variables
	os.Setenv("HOTEL_REVIEWS_DATABASE_HOST", "localhost")
	os.Setenv("HOTEL_REVIEWS_DATABASE_USER", "postgres")
	os.Setenv("HOTEL_REVIEWS_DATABASE_PASSWORD", "postgres")
	os.Setenv("HOTEL_REVIEWS_DATABASE_NAME", "hotel_reviews")
	os.Setenv("HOTEL_REVIEWS_S3_REGION", "us-east-1")
	os.Setenv("HOTEL_REVIEWS_S3_ACCESS_KEY_ID", "test-access-key")
	os.Setenv("HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY", "test-secret-key")
	os.Setenv("HOTEL_REVIEWS_S3_BUCKET", "test-bucket")
	os.Setenv("HOTEL_REVIEWS_SECURITY_JWT_SECRET", "test-jwt-secret")
	os.Setenv("HOTEL_REVIEWS_AUTH_JWT_SECRET", "test-auth-jwt-secret")
	
	config, err := Load()
	
	suite.NoError(err)
	suite.NotNil(config)
	
	// Verify defaults are set
	suite.Equal("localhost", config.Database.Host)
	suite.Equal(5432, config.Database.Port)
	suite.Equal("postgres", config.Database.User)
	suite.Equal("postgres", config.Database.Password)
	suite.Equal("hotel_reviews", config.Database.Name)
	suite.Equal("disable", config.Database.SSLMode)
	suite.Equal(25, config.Database.MaxOpenConns)
	suite.Equal(25, config.Database.MaxIdleConns)
	suite.Equal(5*time.Minute, config.Database.ConnMaxLifetime)
	suite.Equal(5*time.Minute, config.Database.ConnMaxIdleTime)
	suite.Equal("UTC", config.Database.TimeZone)
	suite.Equal("warn", config.Database.LogLevel)
	
	// Verify S3 defaults
	suite.Equal("us-east-1", config.S3.Region)
	suite.Equal("test-access-key", config.S3.AccessKeyID)
	suite.Equal("test-secret-key", config.S3.SecretAccessKey)
	suite.Equal("test-bucket", config.S3.Bucket)
	suite.True(config.S3.UseSSL)
	suite.False(config.S3.ForcePathStyle)
	suite.Equal(30*time.Second, config.S3.Timeout)
	suite.Equal(3, config.S3.RetryCount)
	suite.Equal(1*time.Second, config.S3.RetryDelay)
	suite.Equal(int64(5*1024*1024), config.S3.UploadPartSize)
	suite.Equal(int64(5*1024*1024), config.S3.DownloadPartSize)
	
	// Verify server defaults
	suite.Equal("0.0.0.0", config.Server.Host)
	suite.Equal(8080, config.Server.Port)
	suite.Equal(10*time.Second, config.Server.ReadTimeout)
	suite.Equal(10*time.Second, config.Server.WriteTimeout)
	suite.Equal(60*time.Second, config.Server.IdleTimeout)
	suite.Equal(30*time.Second, config.Server.ShutdownTimeout)
	suite.Equal(1<<20, config.Server.MaxHeaderBytes)
	suite.True(config.Server.EnableCORS)
	suite.True(config.Server.EnableGzip)
	suite.True(config.Server.EnableMetrics)
	suite.False(config.Server.EnablePprof)
	suite.Empty(config.Server.TrustedProxies)
	
	// Verify log defaults
	suite.Equal("info", config.Log.Level)
	suite.Equal("json", config.Log.Format)
	suite.Equal("stdout", config.Log.Output)
	suite.Equal(100, config.Log.MaxSize)
	suite.Equal(3, config.Log.MaxBackups)
	suite.Equal(28, config.Log.MaxAge)
	suite.True(config.Log.Compress)
	suite.True(config.Log.EnableCaller)
	suite.False(config.Log.EnableStacktrace)
	
	// Verify cache defaults
	suite.Equal("redis", config.Cache.Type)
	suite.Equal("localhost", config.Cache.Host)
	suite.Equal(6379, config.Cache.Port)
	suite.Equal(0, config.Cache.Database)
	suite.Equal(10, config.Cache.PoolSize)
	suite.Equal(5, config.Cache.MinIdleConns)
	suite.Equal(5*time.Second, config.Cache.DialTimeout)
	suite.Equal(3*time.Second, config.Cache.ReadTimeout)
	suite.Equal(3*time.Second, config.Cache.WriteTimeout)
	suite.Equal(1*time.Hour, config.Cache.TTL)
	suite.Equal(int64(100*1024*1024), config.Cache.MaxMemory)
	
	// Verify metrics defaults
	suite.True(config.Metrics.Enabled)
	suite.Equal("prometheus", config.Metrics.Type)
	suite.Equal("localhost", config.Metrics.Host)
	suite.Equal(9090, config.Metrics.Port)
	suite.Equal("/metrics", config.Metrics.Path)
	suite.Equal("hotel_reviews", config.Metrics.Namespace)
	suite.Equal("hotel-reviews-api", config.Metrics.ServiceName)
	suite.Equal("development", config.Metrics.Environment)
	suite.Equal("1.0.0", config.Metrics.Version)
	
	// Verify processing defaults
	suite.Equal(1000, config.Processing.BatchSize)
	suite.Equal(4, config.Processing.WorkerCount)
	suite.Equal(int64(100*1024*1024), config.Processing.MaxFileSize)
	suite.Equal(30*time.Minute, config.Processing.ProcessingTimeout)
	suite.Equal(3, config.Processing.MaxRetries)
	suite.Equal(5*time.Second, config.Processing.RetryDelay)
	suite.True(config.Processing.EnableValidation)
	suite.True(config.Processing.EnableDuplicateCheck)
	suite.Equal("/tmp/hotel-reviews", config.Processing.TempDirectory)
	
	// Verify security defaults
	suite.Equal("test-jwt-secret", config.Security.JWTSecret)
	suite.Equal(24*time.Hour, config.Security.JWTExpiration)
	suite.Equal(1000, config.Security.RateLimit)
	suite.Equal(1*time.Hour, config.Security.RateLimitWindow)
	suite.False(config.Security.EnableAPIKey)
	suite.Equal("X-API-Key", config.Security.APIKeyHeader)
	suite.False(config.Security.EnableEncryption)
}

func (suite *ConfigTestSuite) TestLoad_WithEnvironmentVariables() {
	// Set custom environment variables (all required fields)
	os.Setenv("HOTEL_REVIEWS_DATABASE_HOST", "custom-db-host")
	os.Setenv("HOTEL_REVIEWS_DATABASE_PORT", "5433")
	os.Setenv("HOTEL_REVIEWS_DATABASE_USER", "custom-user")
	os.Setenv("HOTEL_REVIEWS_DATABASE_PASSWORD", "custom-password")
	os.Setenv("HOTEL_REVIEWS_DATABASE_NAME", "custom-db")
	os.Setenv("HOTEL_REVIEWS_S3_REGION", "us-west-2")
	os.Setenv("HOTEL_REVIEWS_S3_ACCESS_KEY_ID", "custom-access-key")
	os.Setenv("HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY", "custom-secret-key")
	os.Setenv("HOTEL_REVIEWS_S3_BUCKET", "custom-bucket")
	os.Setenv("HOTEL_REVIEWS_SERVER_PORT", "9090")
	os.Setenv("HOTEL_REVIEWS_LOG_LEVEL", "debug")
	os.Setenv("HOTEL_REVIEWS_SECURITY_JWT_SECRET", "custom-jwt-secret")
	
	config, err := Load()
	
	suite.NoError(err)
	suite.NotNil(config)
	
	// Verify custom values from environment
	suite.Equal("custom-db-host", config.Database.Host)
	suite.Equal(5433, config.Database.Port)
	suite.Equal("custom-user", config.Database.User)
	suite.Equal("custom-password", config.Database.Password)
	suite.Equal("custom-db", config.Database.Name)
	suite.Equal("us-west-2", config.S3.Region)
	suite.Equal("custom-access-key", config.S3.AccessKeyID)
	suite.Equal("custom-secret-key", config.S3.SecretAccessKey)
	suite.Equal("custom-bucket", config.S3.Bucket)
	suite.Equal(9090, config.Server.Port)
	suite.Equal("debug", config.Log.Level)
	suite.Equal("custom-jwt-secret", config.Security.JWTSecret)
}

func (suite *ConfigTestSuite) TestLoad_WithConfigFile() {
	// Create a test config file
	configFile := filepath.Join(suite.tmpDir, "config.yaml")
	configContent := `
database:
  host: file-db-host
  port: 5434
  user: file-user
  password: file-password
  name: file-db
  ssl_mode: require
  max_open_conns: 50
  max_idle_conns: 25
  conn_max_lifetime: 10m
  conn_max_idle_time: 5m
  timezone: America/New_York
  log_level: info

s3:
  region: eu-west-1
  access_key_id: file-access-key
  secret_access_key: file-secret-key
  bucket: file-bucket
  use_ssl: false
  force_path_style: true
  timeout: 45s
  retry_count: 5
  retry_delay: 2s
  upload_part_size: 10485760
  download_part_size: 10485760

server:
  host: 127.0.0.1
  port: 8090
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 120s
  shutdown_timeout: 45s
  max_header_bytes: 2097152
  enable_cors: false
  enable_gzip: false
  enable_metrics: false
  enable_pprof: true
  trusted_proxies:
    - 192.168.1.0/24
    - 10.0.0.0/8

log:
  level: warn
  format: text
  output: stderr
  max_size: 200
  max_backups: 5
  max_age: 14
  compress: false
  enable_caller: false
  enable_stacktrace: true

cache:
  type: memory
  host: cache-host
  port: 6380
  password: cache-password
  database: 1
  pool_size: 20
  min_idle_conns: 10
  dial_timeout: 10s
  read_timeout: 5s
  write_timeout: 5s
  ttl: 2h
  max_memory: 209715200

metrics:
  enabled: false
  type: jaeger
  host: metrics-host
  port: 14268
  path: /api/traces
  namespace: custom_namespace
  service_name: custom-service
  environment: staging
  version: 2.0.0

notification:
  email:
    enabled: true
    host: smtp.example.com
    port: 465
    username: test@example.com
    password: email-password
    from: noreply@example.com
    use_tls: false
  slack:
    enabled: true
    webhook_url: https://hooks.slack.com/services/test
    channel: "#alerts"
    username: Custom Bot
    icon_emoji: ":warning:"
    icon_url: https://example.com/icon.png

processing:
  batch_size: 2000
  worker_count: 8
  max_file_size: 209715200
  processing_timeout: 60m
  max_retries: 5
  retry_delay: 10s
  enable_validation: false
  enable_duplicate_check: false
  temp_directory: /custom/temp

security:
  jwt_secret: file-jwt-secret
  jwt_expiration: 48h
  rate_limit: 2000
  rate_limit_window: 2h
  enable_api_key: true
  api_key_header: X-Custom-API-Key
  enable_encryption: true
  encryption_key: file-encryption-key
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	suite.NoError(err)
	
	// Change to the directory containing the config file
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(suite.tmpDir)
	
	config, err := Load()
	
	suite.NoError(err)
	suite.NotNil(config)
	
	// Verify values from config file
	suite.Equal("file-db-host", config.Database.Host)
	suite.Equal(5434, config.Database.Port)
	suite.Equal("file-user", config.Database.User)
	suite.Equal("file-password", config.Database.Password)
	suite.Equal("file-db", config.Database.Name)
	suite.Equal("require", config.Database.SSLMode)
	suite.Equal(50, config.Database.MaxOpenConns)
	suite.Equal(25, config.Database.MaxIdleConns)
	suite.Equal(10*time.Minute, config.Database.ConnMaxLifetime)
	suite.Equal(5*time.Minute, config.Database.ConnMaxIdleTime)
	suite.Equal("America/New_York", config.Database.TimeZone)
	suite.Equal("info", config.Database.LogLevel)
	
	suite.Equal("eu-west-1", config.S3.Region)
	suite.Equal("file-access-key", config.S3.AccessKeyID)
	suite.Equal("file-secret-key", config.S3.SecretAccessKey)
	suite.Equal("file-bucket", config.S3.Bucket)
	suite.False(config.S3.UseSSL)
	suite.True(config.S3.ForcePathStyle)
	suite.Equal(45*time.Second, config.S3.Timeout)
	suite.Equal(5, config.S3.RetryCount)
	suite.Equal(2*time.Second, config.S3.RetryDelay)
	suite.Equal(int64(10485760), config.S3.UploadPartSize)
	suite.Equal(int64(10485760), config.S3.DownloadPartSize)
	
	suite.Equal("127.0.0.1", config.Server.Host)
	suite.Equal(8090, config.Server.Port)
	suite.Equal(15*time.Second, config.Server.ReadTimeout)
	suite.Equal(15*time.Second, config.Server.WriteTimeout)
	suite.Equal(120*time.Second, config.Server.IdleTimeout)
	suite.Equal(45*time.Second, config.Server.ShutdownTimeout)
	suite.Equal(2097152, config.Server.MaxHeaderBytes)
	suite.False(config.Server.EnableCORS)
	suite.False(config.Server.EnableGzip)
	suite.False(config.Server.EnableMetrics)
	suite.True(config.Server.EnablePprof)
	suite.Len(config.Server.TrustedProxies, 2)
	suite.Contains(config.Server.TrustedProxies, "192.168.1.0/24")
	suite.Contains(config.Server.TrustedProxies, "10.0.0.0/8")
	
	suite.Equal("warn", config.Log.Level)
	suite.Equal("text", config.Log.Format)
	suite.Equal("stderr", config.Log.Output)
	suite.Equal(200, config.Log.MaxSize)
	suite.Equal(5, config.Log.MaxBackups)
	suite.Equal(14, config.Log.MaxAge)
	suite.False(config.Log.Compress)
	suite.False(config.Log.EnableCaller)
	suite.True(config.Log.EnableStacktrace)
	
	suite.Equal("memory", config.Cache.Type)
	suite.Equal("cache-host", config.Cache.Host)
	suite.Equal(6380, config.Cache.Port)
	suite.Equal("cache-password", config.Cache.Password)
	suite.Equal(1, config.Cache.Database)
	suite.Equal(20, config.Cache.PoolSize)
	suite.Equal(10, config.Cache.MinIdleConns)
	suite.Equal(10*time.Second, config.Cache.DialTimeout)
	suite.Equal(5*time.Second, config.Cache.ReadTimeout)
	suite.Equal(5*time.Second, config.Cache.WriteTimeout)
	suite.Equal(2*time.Hour, config.Cache.TTL)
	suite.Equal(int64(209715200), config.Cache.MaxMemory)
	
	suite.False(config.Metrics.Enabled)
	suite.Equal("jaeger", config.Metrics.Type)
	suite.Equal("metrics-host", config.Metrics.Host)
	suite.Equal(14268, config.Metrics.Port)
	suite.Equal("/api/traces", config.Metrics.Path)
	suite.Equal("custom_namespace", config.Metrics.Namespace)
	suite.Equal("custom-service", config.Metrics.ServiceName)
	suite.Equal("staging", config.Metrics.Environment)
	suite.Equal("2.0.0", config.Metrics.Version)
	
	suite.True(config.Notification.Email.Enabled)
	suite.Equal("smtp.example.com", config.Notification.Email.Host)
	suite.Equal(465, config.Notification.Email.Port)
	suite.Equal("test@example.com", config.Notification.Email.Username)
	suite.Equal("email-password", config.Notification.Email.Password)
	suite.Equal("noreply@example.com", config.Notification.Email.From)
	suite.False(config.Notification.Email.UseTLS)
	
	suite.True(config.Notification.Slack.Enabled)
	suite.Equal("https://hooks.slack.com/services/test", config.Notification.Slack.WebhookURL)
	suite.Equal("#alerts", config.Notification.Slack.Channel)
	suite.Equal("Custom Bot", config.Notification.Slack.Username)
	suite.Equal(":warning:", config.Notification.Slack.IconEmoji)
	suite.Equal("https://example.com/icon.png", config.Notification.Slack.IconURL)
	
	suite.Equal(2000, config.Processing.BatchSize)
	suite.Equal(8, config.Processing.WorkerCount)
	suite.Equal(int64(209715200), config.Processing.MaxFileSize)
	suite.Equal(60*time.Minute, config.Processing.ProcessingTimeout)
	suite.Equal(5, config.Processing.MaxRetries)
	suite.Equal(10*time.Second, config.Processing.RetryDelay)
	suite.False(config.Processing.EnableValidation)
	suite.False(config.Processing.EnableDuplicateCheck)
	suite.Equal("/custom/temp", config.Processing.TempDirectory)
	
	suite.Equal("file-jwt-secret", config.Security.JWTSecret)
	suite.Equal(48*time.Hour, config.Security.JWTExpiration)
	suite.Equal(2000, config.Security.RateLimit)
	suite.Equal(2*time.Hour, config.Security.RateLimitWindow)
	suite.True(config.Security.EnableAPIKey)
	suite.Equal("X-Custom-API-Key", config.Security.APIKeyHeader)
	suite.True(config.Security.EnableEncryption)
	suite.Equal("file-encryption-key", config.Security.EncryptionKey)
}

// Test validation failures
func (suite *ConfigTestSuite) TestLoad_ValidationFailures() {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "Missing database host",
			envVars:  map[string]string{"HOTEL_REVIEWS_DATABASE_HOST": ""},
			expected: "database host is required",
		},
		{
			name:     "Invalid database port",
			envVars:  map[string]string{"HOTEL_REVIEWS_DATABASE_PORT": "0"},
			expected: "database port must be between 1 and 65535",
		},
		{
			name:     "Invalid database port high",
			envVars:  map[string]string{"HOTEL_REVIEWS_DATABASE_PORT": "65536"},
			expected: "database port must be between 1 and 65535",
		},
		{
			name:     "Missing database user",
			envVars:  map[string]string{"HOTEL_REVIEWS_DATABASE_USER": ""},
			expected: "database user is required",
		},
		{
			name:     "Missing database password",
			envVars:  map[string]string{"HOTEL_REVIEWS_DATABASE_PASSWORD": ""},
			expected: "database password is required",
		},
		{
			name:     "Missing database name",
			envVars:  map[string]string{"HOTEL_REVIEWS_DATABASE_NAME": ""},
			expected: "database name is required",
		},
		{
			name:     "Missing S3 region",
			envVars:  map[string]string{"HOTEL_REVIEWS_S3_REGION": ""},
			expected: "S3 region is required",
		},
		{
			name:     "Missing S3 access key ID",
			envVars:  map[string]string{"HOTEL_REVIEWS_S3_ACCESS_KEY_ID": ""},
			expected: "S3 access key ID is required",
		},
		{
			name:     "Missing S3 secret access key",
			envVars:  map[string]string{"HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY": ""},
			expected: "S3 secret access key is required",
		},
		{
			name:     "Missing S3 bucket",
			envVars:  map[string]string{"HOTEL_REVIEWS_S3_BUCKET": ""},
			expected: "S3 bucket is required",
		},
		{
			name:     "Invalid server port",
			envVars:  map[string]string{"HOTEL_REVIEWS_SERVER_PORT": "0"},
			expected: "server port must be between 1 and 65535",
		},
		{
			name:     "Invalid server port high",
			envVars:  map[string]string{"HOTEL_REVIEWS_SERVER_PORT": "65536"},
			expected: "server port must be between 1 and 65535",
		},
		{
			name:     "Invalid log level",
			envVars:  map[string]string{"HOTEL_REVIEWS_LOG_LEVEL": "invalid"},
			expected: "log level must be one of: debug, info, warn, error",
		},
		{
			name:     "Invalid log format",
			envVars:  map[string]string{"HOTEL_REVIEWS_LOG_FORMAT": "invalid"},
			expected: "log format must be one of: json, text",
		},
		{
			name:     "Invalid log output",
			envVars:  map[string]string{"HOTEL_REVIEWS_LOG_OUTPUT": "invalid"},
			expected: "log output must be one of: stdout, stderr, file",
		},
		{
			name: "Missing log file path",
			envVars: map[string]string{
				"HOTEL_REVIEWS_LOG_OUTPUT":    "file",
				"HOTEL_REVIEWS_LOG_FILE_PATH": "",
			},
			expected: "log file path is required when output is file",
		},
		{
			name:     "Invalid cache type",
			envVars:  map[string]string{"HOTEL_REVIEWS_CACHE_TYPE": "invalid"},
			expected: "cache type must be one of: redis, memory",
		},
		{
			name:     "Missing JWT secret",
			envVars:  map[string]string{"HOTEL_REVIEWS_SECURITY_JWT_SECRET": ""},
			expected: "JWT secret is required",
		},
	}
	
	for _, test := range tests {
		suite.Run(test.name, func() {
			// Clean environment
			suite.cleanupTestEnvVars()
			
			// Set ALL required values to valid defaults
			os.Setenv("HOTEL_REVIEWS_DATABASE_HOST", "localhost")
			os.Setenv("HOTEL_REVIEWS_DATABASE_USER", "testuser")
			os.Setenv("HOTEL_REVIEWS_DATABASE_PASSWORD", "testpass")
			os.Setenv("HOTEL_REVIEWS_DATABASE_NAME", "testdb")
			os.Setenv("HOTEL_REVIEWS_S3_REGION", "us-east-1")
			os.Setenv("HOTEL_REVIEWS_S3_ACCESS_KEY_ID", "test-access-key")
			os.Setenv("HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY", "test-secret-key")
			os.Setenv("HOTEL_REVIEWS_S3_BUCKET", "test-bucket")
			os.Setenv("HOTEL_REVIEWS_SECURITY_JWT_SECRET", "test-jwt-secret")
	os.Setenv("HOTEL_REVIEWS_AUTH_JWT_SECRET", "test-auth-jwt-secret")
			
			// Set test-specific environment variables
			for key, value := range test.envVars {
				os.Setenv(key, value)
			}
			
			_, err := Load()
			suite.Error(err)
			suite.Contains(err.Error(), test.expected)
		})
	}
}

func (suite *ConfigTestSuite) TestLoad_TLSValidation() {
	// Set all required environment variables
	os.Setenv("HOTEL_REVIEWS_DATABASE_HOST", "localhost")
	os.Setenv("HOTEL_REVIEWS_DATABASE_USER", "postgres")
	os.Setenv("HOTEL_REVIEWS_DATABASE_PASSWORD", "postgres")
	os.Setenv("HOTEL_REVIEWS_DATABASE_NAME", "hotel_reviews")
	os.Setenv("HOTEL_REVIEWS_S3_REGION", "us-east-1")
	os.Setenv("HOTEL_REVIEWS_S3_ACCESS_KEY_ID", "test-access-key")
	os.Setenv("HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY", "test-secret-key")
	os.Setenv("HOTEL_REVIEWS_S3_BUCKET", "test-bucket")
	os.Setenv("HOTEL_REVIEWS_SECURITY_JWT_SECRET", "test-jwt-secret")
	os.Setenv("HOTEL_REVIEWS_AUTH_JWT_SECRET", "test-auth-jwt-secret")
	
	// Test missing TLS key file
	os.Setenv("HOTEL_REVIEWS_SERVER_TLS_CERT_FILE", "cert.pem")
	
	_, err := Load()
	suite.Error(err)
	suite.Contains(err.Error(), "TLS key file is required when TLS cert file is provided")
	
	// Test missing TLS cert file
	os.Unsetenv("HOTEL_REVIEWS_SERVER_TLS_CERT_FILE")
	os.Setenv("HOTEL_REVIEWS_SERVER_TLS_KEY_FILE", "key.pem")
	
	_, err = Load()
	suite.Error(err)
	suite.Contains(err.Error(), "TLS cert file is required when TLS key file is provided")
	
	// Test non-existent TLS cert file
	os.Setenv("HOTEL_REVIEWS_SERVER_TLS_CERT_FILE", "/non/existent/cert.pem")
	os.Setenv("HOTEL_REVIEWS_SERVER_TLS_KEY_FILE", "key.pem")
	
	_, err = Load()
	suite.Error(err)
	suite.Contains(err.Error(), "TLS cert file does not exist")
	
	// Test non-existent TLS key file
	certFile := filepath.Join(suite.tmpDir, "cert.pem")
	keyFile := filepath.Join(suite.tmpDir, "key.pem")
	
	// Create cert file but not key file
	err = os.WriteFile(certFile, []byte("cert content"), 0644)
	suite.NoError(err)
	
	os.Setenv("HOTEL_REVIEWS_SERVER_TLS_CERT_FILE", certFile)
	os.Setenv("HOTEL_REVIEWS_SERVER_TLS_KEY_FILE", "/non/existent/key.pem")
	
	_, err = Load()
	suite.Error(err)
	suite.Contains(err.Error(), "TLS key file does not exist")
	
	// Test valid TLS files
	err = os.WriteFile(keyFile, []byte("key content"), 0644)
	suite.NoError(err)
	
	os.Setenv("HOTEL_REVIEWS_SERVER_TLS_KEY_FILE", keyFile)
	
	config, err := Load()
	suite.NoError(err)
	suite.NotNil(config)
	suite.Equal(certFile, config.Server.TLSCertFile)
	suite.Equal(keyFile, config.Server.TLSKeyFile)
}

// Test helper methods
func (suite *ConfigTestSuite) TestGetDatabaseURL() {
	config := &Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "testuser",
			Password: "testpass",
			Name:     "testdb",
			SSLMode:  "require",
			TimeZone: "UTC",
		},
	}
	
	expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=require&timezone=UTC"
	actual := config.GetDatabaseURL()
	
	suite.Equal(expected, actual)
}

func (suite *ConfigTestSuite) TestGetServerAddress() {
	config := &Config{
		Server: ServerConfig{
			Host: "192.168.1.1",
			Port: 8080,
		},
	}
	
	expected := "192.168.1.1:8080"
	actual := config.GetServerAddress()
	
	suite.Equal(expected, actual)
}

func (suite *ConfigTestSuite) TestGetCacheAddress() {
	config := &Config{
		Cache: CacheConfig{
			Host: "cache.example.com",
			Port: 6379,
		},
	}
	
	expected := "cache.example.com:6379"
	actual := config.GetCacheAddress()
	
	suite.Equal(expected, actual)
}

func (suite *ConfigTestSuite) TestEnvironmentChecks() {
	// Test production environment
	config := &Config{
		Metrics: MetricsConfig{
			Environment: "production",
		},
	}
	
	suite.True(config.IsProduction())
	suite.False(config.IsDevelopment())
	suite.False(config.IsTestEnvironment())
	
	// Test development environment
	config.Metrics.Environment = "development"
	
	suite.False(config.IsProduction())
	suite.True(config.IsDevelopment())
	suite.False(config.IsTestEnvironment())
	
	// Test test environment
	config.Metrics.Environment = "test"
	
	suite.False(config.IsProduction())
	suite.False(config.IsDevelopment())
	suite.True(config.IsTestEnvironment())
	
	// Test other environment
	config.Metrics.Environment = "staging"
	
	suite.False(config.IsProduction())
	suite.False(config.IsDevelopment())
	suite.False(config.IsTestEnvironment())
}

// Test contains helper function
func (suite *ConfigTestSuite) TestContains() {
	slice := []string{"a", "b", "c"}
	
	suite.True(contains(slice, "a"))
	suite.True(contains(slice, "b"))
	suite.True(contains(slice, "c"))
	suite.False(contains(slice, "d"))
	suite.False(contains(slice, ""))
	
	// Test empty slice
	emptySlice := []string{}
	suite.False(contains(emptySlice, "a"))
}

// Test individual functions
func TestConfigFunctions(t *testing.T) {
	t.Run("contains", func(t *testing.T) {
		slice := []string{"debug", "info", "warn", "error"}
		
		assert.True(t, contains(slice, "debug"))
		assert.True(t, contains(slice, "info"))
		assert.True(t, contains(slice, "warn"))
		assert.True(t, contains(slice, "error"))
		assert.False(t, contains(slice, "trace"))
		assert.False(t, contains(slice, ""))
		
		// Test empty slice
		emptySlice := []string{}
		assert.False(t, contains(emptySlice, "debug"))
	})
}

// Test environment variable override precedence
func (suite *ConfigTestSuite) TestEnvironmentOverridesPrecedence() {
	// Create a config file
	configFile := filepath.Join(suite.tmpDir, "config.yaml")
	configContent := `
database:
  host: file-host
  port: 5432
  user: file-user
  password: file-password
  name: file-db
server:
  port: 8080
log:
  level: info
s3:
  region: us-east-1
  access_key_id: file-access-key
  secret_access_key: file-secret-key
  bucket: file-bucket
security:
  jwt_secret: file-jwt-secret
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	suite.NoError(err)
	
	// Set environment variables that should override config file
	os.Setenv("HOTEL_REVIEWS_DATABASE_HOST", "env-host")
	os.Setenv("HOTEL_REVIEWS_DATABASE_USER", "env-user")
	os.Setenv("HOTEL_REVIEWS_DATABASE_PASSWORD", "env-password")
	os.Setenv("HOTEL_REVIEWS_DATABASE_NAME", "env-db")
	os.Setenv("HOTEL_REVIEWS_S3_REGION", "us-west-2")
	os.Setenv("HOTEL_REVIEWS_SERVER_PORT", "9090")
	os.Setenv("HOTEL_REVIEWS_LOG_LEVEL", "debug")
	os.Setenv("HOTEL_REVIEWS_S3_ACCESS_KEY_ID", "env-access-key")
	os.Setenv("HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY", "env-secret-key")
	os.Setenv("HOTEL_REVIEWS_S3_BUCKET", "env-bucket")
	os.Setenv("HOTEL_REVIEWS_SECURITY_JWT_SECRET", "env-jwt-secret")
	
	// Change to the directory containing the config file
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(suite.tmpDir)
	
	config, err := Load()
	
	suite.NoError(err)
	suite.NotNil(config)
	
	// Verify environment variables override config file
	suite.Equal("env-host", config.Database.Host)
	suite.Equal("env-user", config.Database.User)
	suite.Equal("env-password", config.Database.Password)
	suite.Equal("env-db", config.Database.Name)
	suite.Equal("us-west-2", config.S3.Region)
	suite.Equal(9090, config.Server.Port)
	suite.Equal("debug", config.Log.Level)
	suite.Equal("env-access-key", config.S3.AccessKeyID)
	suite.Equal("env-secret-key", config.S3.SecretAccessKey)
	suite.Equal("env-bucket", config.S3.Bucket)
	suite.Equal("env-jwt-secret", config.Security.JWTSecret)
	
	// Verify config file values are used when no environment variable is set
	suite.Equal(5432, config.Database.Port) // From config file
}

// Test invalid config file
func (suite *ConfigTestSuite) TestLoad_InvalidConfigFile() {
	// Create an invalid config file
	configFile := filepath.Join(suite.tmpDir, "config.yaml")
	configContent := `
invalid yaml content:
  - missing closing bracket
  - invalid: [unclosed
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	suite.NoError(err)
	
	// Change to the directory containing the config file
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(suite.tmpDir)
	
	_, err = Load()
	suite.Error(err)
	suite.Contains(err.Error(), "error reading config file")
}

// Test complete configuration structure
func (suite *ConfigTestSuite) TestCompleteConfigStructure() {
	// Set all required environment variables
	os.Setenv("HOTEL_REVIEWS_DATABASE_HOST", "localhost")
	os.Setenv("HOTEL_REVIEWS_DATABASE_USER", "postgres")
	os.Setenv("HOTEL_REVIEWS_DATABASE_PASSWORD", "postgres")
	os.Setenv("HOTEL_REVIEWS_DATABASE_NAME", "hotel_reviews")
	os.Setenv("HOTEL_REVIEWS_S3_REGION", "us-east-1")
	os.Setenv("HOTEL_REVIEWS_S3_ACCESS_KEY_ID", "test-access-key")
	os.Setenv("HOTEL_REVIEWS_S3_SECRET_ACCESS_KEY", "test-secret-key")
	os.Setenv("HOTEL_REVIEWS_S3_BUCKET", "test-bucket")
	os.Setenv("HOTEL_REVIEWS_SECURITY_JWT_SECRET", "test-jwt-secret")
	os.Setenv("HOTEL_REVIEWS_AUTH_JWT_SECRET", "test-auth-jwt-secret")
	
	config, err := Load()
	suite.NoError(err)
	suite.NotNil(config)
	
	// Verify all config sections are present
	suite.NotNil(config.Database)
	suite.NotNil(config.S3)
	suite.NotNil(config.Server)
	suite.NotNil(config.Log)
	suite.NotNil(config.Cache)
	suite.NotNil(config.Metrics)
	suite.NotNil(config.Notification)
	suite.NotNil(config.Notification.Email)
	suite.NotNil(config.Notification.Slack)
	suite.NotNil(config.Processing)
	suite.NotNil(config.Security)
	
	// Verify notification defaults
	suite.False(config.Notification.Email.Enabled)
	suite.Equal(587, config.Notification.Email.Port)
	suite.True(config.Notification.Email.UseTLS)
	
	suite.False(config.Notification.Slack.Enabled)
	suite.Equal("Hotel Reviews Bot", config.Notification.Slack.Username)
	suite.Equal(":hotel:", config.Notification.Slack.IconEmoji)
}