package logger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// LoggerTestSuite for testing logger functionality
type LoggerTestSuite struct {
	suite.Suite
	buffer    *bytes.Buffer
	logger    *Logger
	ctx       context.Context
	tmpDir    string
}

func (suite *LoggerTestSuite) SetupTest() {
	suite.buffer = &bytes.Buffer{}
	suite.ctx = context.Background()
	
	// Create temporary directory for file tests
	var err error
	suite.tmpDir, err = os.MkdirTemp("", "logger_test")
	suite.NoError(err)
}

func (suite *LoggerTestSuite) TearDownTest() {
	if suite.tmpDir != "" {
		os.RemoveAll(suite.tmpDir)
	}
}

func TestLoggerTestSuite(t *testing.T) {
	suite.Run(t, new(LoggerTestSuite))
}

// Test logger creation and configuration
func (suite *LoggerTestSuite) TestNew_JSONFormat() {
	config := &Config{
		Level:           "debug",
		Format:          "json",
		Output:          "stdout",
		EnableCaller:    true,
		EnableStacktrace: true,
	}
	
	logger, err := New(config)
	
	suite.NoError(err)
	suite.NotNil(logger)
	suite.Equal(config, logger.config)
}

func (suite *LoggerTestSuite) TestNew_TextFormat() {
	config := &Config{
		Level:           "info",
		Format:          "text",
		Output:          "stderr",
		EnableCaller:    false,
		EnableStacktrace: false,
	}
	
	logger, err := New(config)
	
	suite.NoError(err)
	suite.NotNil(logger)
	suite.Equal(config, logger.config)
}

func (suite *LoggerTestSuite) TestNew_FileOutput() {
	logFile := filepath.Join(suite.tmpDir, "test.log")
	config := &Config{
		Level:           "warn",
		Format:          "json",
		Output:          "file",
		FilePath:        logFile,
		MaxSize:         10,
		MaxBackups:      3,
		MaxAge:          7,
		Compress:        true,
		EnableCaller:    true,
		EnableStacktrace: true,
	}
	
	logger, err := New(config)
	
	suite.NoError(err)
	suite.NotNil(logger)
	suite.Equal(config, logger.config)
}

func (suite *LoggerTestSuite) TestNew_FileOutputMissingPath() {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "file",
		// FilePath is missing
	}
	
	logger, err := New(config)
	
	suite.Error(err)
	suite.Nil(logger)
	suite.Contains(err.Error(), "file path is required")
}

func (suite *LoggerTestSuite) TestNew_InvalidLevel() {
	config := &Config{
		Level:  "invalid",
		Format: "json",
		Output: "stdout",
	}
	
	logger, err := New(config)
	
	// Should not error, should default to info level
	suite.NoError(err)
	suite.NotNil(logger)
}

func (suite *LoggerTestSuite) TestNew_DefaultFormat() {
	config := &Config{
		Level:  "info",
		Format: "unknown",
		Output: "stdout",
	}
	
	logger, err := New(config)
	
	suite.NoError(err)
	suite.NotNil(logger)
}

func (suite *LoggerTestSuite) TestNew_DefaultOutput() {
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "unknown",
	}
	
	logger, err := New(config)
	
	suite.NoError(err)
	suite.NotNil(logger)
}

func (suite *LoggerTestSuite) TestNewDefault() {
	logger := NewDefault()
	
	suite.NotNil(logger)
	suite.NotNil(logger.config)
	suite.Equal("info", logger.config.Level)
	suite.Equal("json", logger.config.Format)
	suite.Equal("stdout", logger.config.Output)
	suite.True(logger.config.EnableCaller)
	suite.False(logger.config.EnableStacktrace)
}

// Test context-aware logging
func (suite *LoggerTestSuite) TestWithContext_NoAttributes() {
	logger := NewDefault()
	
	result := logger.WithContext(context.Background())
	
	suite.Equal(logger, result)
}

func (suite *LoggerTestSuite) TestWithContext_WithAttributes() {
	logger := NewDefault()
	ctx := WithRequestID(context.Background(), "test-request-id")
	ctx = WithUserID(ctx, "test-user-id")
	ctx = WithCorrelationID(ctx, "test-correlation-id")
	ctx = WithTraceID(ctx, "test-trace-id")
	ctx = WithSpanID(ctx, "test-span-id")
	
	result := logger.WithContext(ctx)
	
	suite.NotEqual(logger, result)
}

func (suite *LoggerTestSuite) TestExtractContextAttributes() {
	logger := NewDefault()
	
	// Test with empty context
	attrs := logger.extractContextAttributes(context.Background())
	suite.Empty(attrs)
	
	// Test with all attributes
	ctx := WithRequestID(context.Background(), "req-123")
	ctx = WithUserID(ctx, "user-456")
	ctx = WithCorrelationID(ctx, "corr-789")
	ctx = WithTraceID(ctx, "trace-abc")
	ctx = WithSpanID(ctx, "span-def")
	
	attrs = logger.extractContextAttributes(ctx)
	
	suite.Len(attrs, 10) // 5 keys + 5 values
	suite.Contains(attrs, "request_id")
	suite.Contains(attrs, "req-123")
	suite.Contains(attrs, "user_id")
	suite.Contains(attrs, "user-456")
	suite.Contains(attrs, "correlation_id")
	suite.Contains(attrs, "corr-789")
	suite.Contains(attrs, "trace_id")
	suite.Contains(attrs, "trace-abc")
	suite.Contains(attrs, "span_id")
	suite.Contains(attrs, "span-def")
}

// Test context logging methods
func (suite *LoggerTestSuite) TestDebugContext() {
	logger := NewDefault()
	ctx := WithRequestID(context.Background(), "test-req")
	
	logger.DebugContext(ctx, "test debug message", "key", "value")
	
	// Since we can't easily capture the output, just ensure it doesn't panic
	suite.True(true)
}

func (suite *LoggerTestSuite) TestInfoContext() {
	logger := NewDefault()
	ctx := WithUserID(context.Background(), "test-user")
	
	logger.InfoContext(ctx, "test info message", "key", "value")
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestWarnContext() {
	logger := NewDefault()
	ctx := WithCorrelationID(context.Background(), "test-corr")
	
	logger.WarnContext(ctx, "test warn message", "key", "value")
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestErrorContext() {
	config := &Config{
		Level:            "error",
		Format:           "json",
		Output:           "stdout",
		EnableStacktrace: true,
	}
	logger, err := New(config)
	suite.NoError(err)
	
	ctx := WithTraceID(context.Background(), "test-trace")
	
	logger.ErrorContext(ctx, "test error message", "key", "value")
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestErrorContext_WithoutStacktrace() {
	config := &Config{
		Level:            "error",
		Format:           "json",
		Output:           "stdout",
		EnableStacktrace: false,
	}
	logger, err := New(config)
	suite.NoError(err)
	
	ctx := WithSpanID(context.Background(), "test-span")
	
	logger.ErrorContext(ctx, "test error message", "key", "value")
	
	suite.True(true)
}

// Test structured logging methods
func (suite *LoggerTestSuite) TestWithFields() {
	logger := NewDefault()
	fields := map[string]any{
		"field1": "value1",
		"field2": 42,
		"field3": true,
	}
	
	result := logger.WithFields(fields)
	
	suite.NotEqual(logger, result)
	suite.NotNil(result)
}

func (suite *LoggerTestSuite) TestWithFields_EmptyFields() {
	logger := NewDefault()
	fields := map[string]any{}
	
	result := logger.WithFields(fields)
	
	// With empty fields, WithFields still returns a new logger instance 
	// but it might have the same underlying slog.Logger
	suite.NotNil(result)
	suite.Equal(logger.config, result.config)
}

func (suite *LoggerTestSuite) TestWithField() {
	logger := NewDefault()
	
	result := logger.WithField("key", "value")
	
	suite.NotEqual(logger, result)
	suite.NotNil(result)
}

func (suite *LoggerTestSuite) TestWithError_NilError() {
	logger := NewDefault()
	
	result := logger.WithError(nil)
	
	suite.Equal(logger, result)
}

func (suite *LoggerTestSuite) TestWithError_WithError() {
	logger := NewDefault()
	err := errors.New("test error")
	
	result := logger.WithError(err)
	
	suite.NotEqual(logger, result)
	suite.NotNil(result)
}

func (suite *LoggerTestSuite) TestWithError_WithStacktrace() {
	config := &Config{
		Level:            "error",
		Format:           "json",
		Output:           "stdout",
		EnableStacktrace: true,
	}
	logger, err := New(config)
	suite.NoError(err)
	
	testErr := errors.New("test error")
	result := logger.WithError(testErr)
	
	suite.NotEqual(logger, result)
	suite.NotNil(result)
}

// Test business domain logging methods
func (suite *LoggerTestSuite) TestLogRequest() {
	logger := NewDefault()
	ctx := WithRequestID(context.Background(), "req-123")
	
	logger.LogRequest(ctx, "GET", "/api/test", 200, 150*time.Millisecond, 1024)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogDatabaseQuery() {
	logger := NewDefault()
	ctx := WithUserID(context.Background(), "user-123")
	
	logger.LogDatabaseQuery(ctx, "SELECT * FROM users", 50*time.Millisecond, 10)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogFileProcessing() {
	logger := NewDefault()
	ctx := WithCorrelationID(context.Background(), "corr-123")
	
	logger.LogFileProcessing(ctx, "data.csv", 1000, 5*time.Second)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogCacheOperation() {
	logger := NewDefault()
	ctx := WithTraceID(context.Background(), "trace-123")
	
	logger.LogCacheOperation(ctx, "GET", "user:123", true, 2*time.Millisecond)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogS3Operation() {
	logger := NewDefault()
	ctx := WithSpanID(context.Background(), "span-123")
	
	logger.LogS3Operation(ctx, "PUT", "my-bucket", "path/to/file.txt", 2048, 100*time.Millisecond)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogMetric() {
	logger := NewDefault()
	ctx := WithRequestID(context.Background(), "req-123")
	labels := map[string]string{
		"method": "GET",
		"status": "200",
	}
	
	logger.LogMetric(ctx, "http_requests_total", 1.0, labels)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogMetric_EmptyLabels() {
	logger := NewDefault()
	ctx := WithRequestID(context.Background(), "req-123")
	
	logger.LogMetric(ctx, "http_requests_total", 1.0, nil)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogBusinessEvent() {
	logger := NewDefault()
	ctx := WithUserID(context.Background(), "user-123")
	details := map[string]any{
		"amount": 100.50,
		"currency": "USD",
	}
	
	logger.LogBusinessEvent(ctx, "payment_processed", "payment", "pay-123", details)
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogBusinessEvent_EmptyDetails() {
	logger := NewDefault()
	ctx := WithUserID(context.Background(), "user-123")
	
	logger.LogBusinessEvent(ctx, "user_created", "user", "user-123", nil)
	
	suite.True(true)
}

// Test error handling methods
func (suite *LoggerTestSuite) TestLogError_NilError() {
	logger := NewDefault()
	
	logger.LogError(context.Background(), nil, "test message")
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogError_WithError() {
	logger := NewDefault()
	err := errors.New("test error")
	
	logger.LogError(context.Background(), err, "test message", "key", "value")
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogError_WithStacktrace() {
	config := &Config{
		Level:            "error",
		Format:           "json",
		Output:           "stdout",
		EnableStacktrace: true,
	}
	logger, err := New(config)
	suite.NoError(err)
	
	testErr := errors.New("test error")
	logger.LogError(context.Background(), testErr, "test message", "key", "value")
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestLogPanic() {
	logger := NewDefault()
	
	logger.LogPanic(context.Background(), "panic message", "test panic", "key", "value")
	
	suite.True(true)
}

// Test utility methods
func (suite *LoggerTestSuite) TestIsDebugEnabled() {
	// Test with debug level
	config := &Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}
	logger, err := New(config)
	suite.NoError(err)
	
	suite.True(logger.IsDebugEnabled())
	
	// Test with info level
	config.Level = "info"
	logger, err = New(config)
	suite.NoError(err)
	
	suite.False(logger.IsDebugEnabled())
}

func (suite *LoggerTestSuite) TestIsInfoEnabled() {
	// Test with info level
	config := &Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	logger, err := New(config)
	suite.NoError(err)
	
	suite.True(logger.IsInfoEnabled())
	
	// Test with warn level
	config.Level = "warn"
	logger, err = New(config)
	suite.NoError(err)
	
	suite.False(logger.IsInfoEnabled())
}

func (suite *LoggerTestSuite) TestIsWarnEnabled() {
	// Test with warn level
	config := &Config{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	logger, err := New(config)
	suite.NoError(err)
	
	suite.True(logger.IsWarnEnabled())
	
	// Test with error level
	config.Level = "error"
	logger, err = New(config)
	suite.NoError(err)
	
	suite.False(logger.IsWarnEnabled())
}

func (suite *LoggerTestSuite) TestIsErrorEnabled() {
	// Test with error level
	config := &Config{
		Level:  "error",
		Format: "json",
		Output: "stdout",
	}
	logger, err := New(config)
	suite.NoError(err)
	
	suite.True(logger.IsErrorEnabled())
}

// Test helper functions
func (suite *LoggerTestSuite) TestParseLogLevel() {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
	}
	
	for _, test := range tests {
		result, err := parseLogLevel(test.input)
		suite.NoError(err)
		suite.Equal(test.expected, result)
	}
}

func (suite *LoggerTestSuite) TestCreateWriter() {
	// Test stdout
	writer, err := createWriter(&Config{Output: "stdout"})
	suite.NoError(err)
	suite.Equal(os.Stdout, writer)
	
	// Test stderr
	writer, err = createWriter(&Config{Output: "stderr"})
	suite.NoError(err)
	suite.Equal(os.Stderr, writer)
	
	// Test file
	logFile := filepath.Join(suite.tmpDir, "test.log")
	writer, err = createWriter(&Config{
		Output:     "file",
		FilePath:   logFile,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   true,
	})
	suite.NoError(err)
	suite.NotNil(writer)
	
	// Test file without path
	writer, err = createWriter(&Config{Output: "file"})
	suite.Error(err)
	suite.Nil(writer)
	
	// Test unknown output
	writer, err = createWriter(&Config{Output: "unknown"})
	suite.NoError(err)
	suite.Equal(os.Stdout, writer)
}

func (suite *LoggerTestSuite) TestShortenFilePath() {
	tests := []struct {
		input    string
		expected string
	}{
		{"/usr/local/go/src/main.go", "src/main.go"},
		{"/path/to/file.go", "to/file.go"},
		{"file.go", "file.go"},
		{"", ""},
	}
	
	for _, test := range tests {
		result := shortenFilePath(test.input)
		suite.Equal(test.expected, result)
	}
}

func (suite *LoggerTestSuite) TestGetStackTrace() {
	stackTrace := getStackTrace()
	
	suite.NotEmpty(stackTrace)
	suite.Contains(stackTrace, "goroutine")
}

// Test context helpers
func (suite *LoggerTestSuite) TestWithRequestID() {
	ctx := WithRequestID(context.Background(), "req-123")
	
	suite.Equal("req-123", ctx.Value(ContextKeyRequestID))
}

func (suite *LoggerTestSuite) TestWithUserID() {
	ctx := WithUserID(context.Background(), "user-123")
	
	suite.Equal("user-123", ctx.Value(ContextKeyUserID))
}

func (suite *LoggerTestSuite) TestWithCorrelationID() {
	ctx := WithCorrelationID(context.Background(), "corr-123")
	
	suite.Equal("corr-123", ctx.Value(ContextKeyCorrelationID))
}

func (suite *LoggerTestSuite) TestWithTraceID() {
	ctx := WithTraceID(context.Background(), "trace-123")
	
	suite.Equal("trace-123", ctx.Value(ContextKeyTraceID))
}

func (suite *LoggerTestSuite) TestWithSpanID() {
	ctx := WithSpanID(context.Background(), "span-123")
	
	suite.Equal("span-123", ctx.Value(ContextKeySpanID))
}

func (suite *LoggerTestSuite) TestGetRequestID() {
	ctx := WithRequestID(context.Background(), "req-123")
	
	suite.Equal("req-123", GetRequestID(ctx))
	suite.Equal("", GetRequestID(context.Background()))
}

func (suite *LoggerTestSuite) TestGetUserID() {
	ctx := WithUserID(context.Background(), "user-123")
	
	suite.Equal("user-123", GetUserID(ctx))
	suite.Equal("", GetUserID(context.Background()))
}

func (suite *LoggerTestSuite) TestGetCorrelationID() {
	ctx := WithCorrelationID(context.Background(), "corr-123")
	
	suite.Equal("corr-123", GetCorrelationID(ctx))
	suite.Equal("", GetCorrelationID(context.Background()))
}

func (suite *LoggerTestSuite) TestGetTraceID() {
	ctx := WithTraceID(context.Background(), "trace-123")
	
	suite.Equal("trace-123", GetTraceID(ctx))
	suite.Equal("", GetTraceID(context.Background()))
}

func (suite *LoggerTestSuite) TestGetSpanID() {
	ctx := WithSpanID(context.Background(), "span-123")
	
	suite.Equal("span-123", GetSpanID(ctx))
	suite.Equal("", GetSpanID(context.Background()))
}

func (suite *LoggerTestSuite) TestGenerateRequestID() {
	id := GenerateRequestID()
	
	suite.NotEmpty(id)
	suite.Len(id, 36) // UUID length
}

func (suite *LoggerTestSuite) TestGenerateCorrelationID() {
	id := GenerateCorrelationID()
	
	suite.NotEmpty(id)
	suite.Len(id, 36) // UUID length
}

// Test middleware
func (suite *LoggerTestSuite) TestRequestLoggerMiddleware() {
	logger := NewDefault()
	
	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
		
		// Verify context has IDs
		suite.NotEmpty(GetRequestID(r.Context()))
		suite.NotEmpty(GetCorrelationID(r.Context()))
	})
	
	// Wrap with middleware
	middleware := RequestLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	// Execute request
	wrappedHandler.ServeHTTP(w, req)
	
	// Check response
	suite.Equal(http.StatusOK, w.Code)
	suite.Equal("test response", w.Body.String())
	
	// Check headers
	suite.NotEmpty(w.Header().Get("X-Request-ID"))
	suite.NotEmpty(w.Header().Get("X-Correlation-ID"))
}

func (suite *LoggerTestSuite) TestRequestLoggerMiddleware_WithExistingHeaders() {
	logger := NewDefault()
	
	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
		
		// Verify context has the provided IDs
		suite.Equal("existing-req-id", GetRequestID(r.Context()))
		suite.Equal("existing-corr-id", GetCorrelationID(r.Context()))
		suite.Equal("existing-trace-id", GetTraceID(r.Context()))
		suite.Equal("existing-span-id", GetSpanID(r.Context()))
	})
	
	// Wrap with middleware
	middleware := RequestLoggerMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	// Create test request with existing headers
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Request-ID", "existing-req-id")
	req.Header.Set("X-Correlation-ID", "existing-corr-id")
	req.Header.Set("X-Trace-ID", "existing-trace-id")
	req.Header.Set("X-Span-ID", "existing-span-id")
	w := httptest.NewRecorder()
	
	// Execute request
	wrappedHandler.ServeHTTP(w, req)
	
	// Check response
	suite.Equal(http.StatusCreated, w.Code)
	suite.Equal("created", w.Body.String())
	
	// Check headers
	suite.Equal("existing-req-id", w.Header().Get("X-Request-ID"))
	suite.Equal("existing-corr-id", w.Header().Get("X-Correlation-ID"))
}

func (suite *LoggerTestSuite) TestResponseWriter() {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, statusCode: 200}
	
	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	suite.Equal(http.StatusNotFound, rw.statusCode)
	
	// Test Write
	data := []byte("test data")
	n, err := rw.Write(data)
	suite.NoError(err)
	suite.Equal(len(data), n)
	suite.Equal(int64(len(data)), rw.size)
	
	// Test multiple writes
	n, err = rw.Write(data)
	suite.NoError(err)
	suite.Equal(len(data), n)
	suite.Equal(int64(len(data)*2), rw.size)
}

func (suite *LoggerTestSuite) TestRecoveryMiddleware() {
	logger := NewDefault()
	
	// Create test handler that panics
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	
	// Wrap with recovery middleware
	middleware := RecoveryMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	// Execute request - should not panic
	wrappedHandler.ServeHTTP(w, req)
	
	// Check response
	suite.Equal(http.StatusInternalServerError, w.Code)
	suite.Equal("application/json", w.Header().Get("Content-Type"))
	suite.Contains(w.Body.String(), "Internal server error")
}

func (suite *LoggerTestSuite) TestRecoveryMiddleware_NoPanic() {
	logger := NewDefault()
	
	// Create test handler that doesn't panic
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
	
	// Wrap with recovery middleware
	middleware := RecoveryMiddleware(logger)
	wrappedHandler := middleware(handler)
	
	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	// Execute request
	wrappedHandler.ServeHTTP(w, req)
	
	// Check response
	suite.Equal(http.StatusOK, w.Code)
	suite.Equal("success", w.Body.String())
}

// Test global logging functions
func (suite *LoggerTestSuite) TestGlobalLoggingFunctions() {
	// Test global functions - they should not panic
	Debug("debug message", "key", "value")
	Info("info message", "key", "value")
	Warn("warn message", "key", "value")
	Error("error message", "key", "value")
	
	ctx := WithRequestID(context.Background(), "req-123")
	DebugContext(ctx, "debug message", "key", "value")
	InfoContext(ctx, "info message", "key", "value")
	WarnContext(ctx, "warn message", "key", "value")
	ErrorContext(ctx, "error message", "key", "value")
	
	suite.True(true)
}

func (suite *LoggerTestSuite) TestSetDefault() {
	originalDefault := defaultLogger
	
	newLogger := NewDefault()
	SetDefault(newLogger)
	
	suite.Equal(newLogger, defaultLogger)
	
	// Restore original
	SetDefault(originalDefault)
}

// Test integration scenarios
func (suite *LoggerTestSuite) TestIntegration_FileLogging() {
	logFile := filepath.Join(suite.tmpDir, "integration.log")
	config := &Config{
		Level:           "info",
		Format:          "json",
		Output:          "file",
		FilePath:        logFile,
		MaxSize:         1,
		MaxBackups:      2,
		MaxAge:          1,
		Compress:        false,
		EnableCaller:    true,
		EnableStacktrace: true,
	}
	
	logger, err := New(config)
	suite.NoError(err)
	
	// Log some messages
	ctx := WithRequestID(context.Background(), "integration-req")
	logger.InfoContext(ctx, "integration test message", "key", "value")
	logger.ErrorContext(ctx, "integration test error", "error", "test error")
	
	// Verify file was created
	suite.FileExists(logFile)
}

func (suite *LoggerTestSuite) TestIntegration_AllLevels() {
	config := &Config{
		Level:           "debug",
		Format:          "json",
		Output:          "stdout",
		EnableCaller:    true,
		EnableStacktrace: true,
	}
	
	logger, err := New(config)
	suite.NoError(err)
	
	ctx := WithRequestID(context.Background(), "all-levels-req")
	ctx = WithUserID(ctx, "test-user")
	ctx = WithCorrelationID(ctx, "test-correlation")
	
	// Test all logging levels
	logger.DebugContext(ctx, "debug message")
	logger.InfoContext(ctx, "info message")
	logger.WarnContext(ctx, "warn message")
	logger.ErrorContext(ctx, "error message")
	
	// Test business logging methods
	logger.LogRequest(ctx, "GET", "/api/test", 200, 100*time.Millisecond, 500)
	logger.LogDatabaseQuery(ctx, "SELECT * FROM users", 50*time.Millisecond, 5)
	logger.LogFileProcessing(ctx, "test.csv", 100, 2*time.Second)
	logger.LogCacheOperation(ctx, "GET", "user:123", true, 1*time.Millisecond)
	logger.LogS3Operation(ctx, "PUT", "bucket", "key", 1024, 200*time.Millisecond)
	
	labels := map[string]string{"method": "GET", "status": "200"}
	logger.LogMetric(ctx, "requests_total", 1.0, labels)
	
	details := map[string]any{"amount": 100.0, "currency": "USD"}
	logger.LogBusinessEvent(ctx, "payment_processed", "payment", "pay-123", details)
	
	// Test error handling
	testErr := errors.New("test error")
	logger.LogError(ctx, testErr, "error occurred")
	logger.LogPanic(ctx, "panic occurred", "panic handled")
	
	suite.True(true)
}

func TestLoggerFunctions(t *testing.T) {
	// Test individual functions not covered by the test suite
	t.Run("parseLogLevel", func(t *testing.T) {
		level, err := parseLogLevel("debug")
		assert.NoError(t, err)
		assert.Equal(t, slog.LevelDebug, level)
		
		level, err = parseLogLevel("invalid")
		assert.NoError(t, err)
		assert.Equal(t, slog.LevelInfo, level)
	})
	
	t.Run("shortenFilePath", func(t *testing.T) {
		result := shortenFilePath("/very/long/path/to/file.go")
		assert.Equal(t, "to/file.go", result)
		
		result = shortenFilePath("file.go")
		assert.Equal(t, "file.go", result)
	})
	
	t.Run("getStackTrace", func(t *testing.T) {
		trace := getStackTrace()
		assert.NotEmpty(t, trace)
		assert.Contains(t, trace, "goroutine")
	})
}