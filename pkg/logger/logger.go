package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ContextKey represents a key for context values
type ContextKey string

const (
	// ContextKeyRequestID is the key for request ID in context
	ContextKeyRequestID ContextKey = "request_id"
	// ContextKeyUserID is the key for user ID in context
	ContextKeyUserID ContextKey = "user_id"
	// ContextKeyCorrelationID is the key for correlation ID in context
	ContextKeyCorrelationID ContextKey = "correlation_id"
	// ContextKeyTraceID is the key for trace ID in context
	ContextKeyTraceID ContextKey = "trace_id"
	// ContextKeySpanID is the key for span ID in context
	ContextKeySpanID ContextKey = "span_id"
)

// Config represents logger configuration
type Config struct {
	Level           string `json:"level"`
	Format          string `json:"format"`
	Output          string `json:"output"`
	FilePath        string `json:"file_path"`
	MaxSize         int    `json:"max_size"`
	MaxBackups      int    `json:"max_backups"`
	MaxAge          int    `json:"max_age"`
	Compress        bool   `json:"compress"`
	EnableCaller    bool   `json:"enable_caller"`
	EnableStacktrace bool  `json:"enable_stacktrace"`
}

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	config *Config
}

// New creates a new logger instance
func New(config *Config) (*Logger, error) {
	// Parse log level
	level, err := parseLogLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// Create writer based on output configuration
	writer, err := createWriter(config)
	if err != nil {
		return nil, err
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.EnableCaller,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize source attribute
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					// Shorten file path
					src.File = shortenFilePath(src.File)
					return slog.Attr{Key: a.Key, Value: slog.AnyValue(src)}
				}
			}
			
			// Customize time format
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: a.Key, Value: slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))}
			}
			
			return a
		},
	}

	// Create handler based on format
	var handler slog.Handler
	switch config.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		handler = slog.NewJSONHandler(writer, opts)
	}

	// Create logger
	logger := &Logger{
		Logger: slog.New(handler),
		config: config,
	}

	return logger, nil
}

// NewDefault creates a logger with default configuration
func NewDefault() *Logger {
	config := &Config{
		Level:           "info",
		Format:          "json",
		Output:          "stdout",
		EnableCaller:    true,
		EnableStacktrace: false,
	}

	logger, err := New(config)
	if err != nil {
		// Fallback to basic logger
		return &Logger{
			Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level:     slog.LevelInfo,
				AddSource: true,
			})),
			config: config,
		}
	}

	return logger
}

// Context-aware logging methods
func (l *Logger) WithContext(ctx context.Context) *Logger {
	attrs := l.extractContextAttributes(ctx)
	if len(attrs) == 0 {
		return l
	}

	return &Logger{
		Logger: l.Logger.With(attrs...),
		config: l.config,
	}
}

func (l *Logger) extractContextAttributes(ctx context.Context) []any {
	var attrs []any

	if requestID := ctx.Value(ContextKeyRequestID); requestID != nil {
		attrs = append(attrs, "request_id", requestID)
	}

	if userID := ctx.Value(ContextKeyUserID); userID != nil {
		attrs = append(attrs, "user_id", userID)
	}

	if correlationID := ctx.Value(ContextKeyCorrelationID); correlationID != nil {
		attrs = append(attrs, "correlation_id", correlationID)
	}

	if traceID := ctx.Value(ContextKeyTraceID); traceID != nil {
		attrs = append(attrs, "trace_id", traceID)
	}

	if spanID := ctx.Value(ContextKeySpanID); spanID != nil {
		attrs = append(attrs, "span_id", spanID)
	}

	return attrs
}

// Enhanced logging methods with context support
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Debug(msg, args...)
}

func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Info(msg, args...)
}

func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Warn(msg, args...)
}

func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	if l.config.EnableStacktrace {
		args = append(args, "stacktrace", getStackTrace())
	}
	l.WithContext(ctx).Error(msg, args...)
}

// Structured logging methods
func (l *Logger) WithFields(fields map[string]any) *Logger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return &Logger{
		Logger: l.Logger.With(attrs...),
		config: l.config,
	}
}

func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{
		Logger: l.Logger.With(key, value),
		config: l.config,
	}
}

func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	
	attrs := []any{"error", err.Error()}
	if l.config.EnableStacktrace {
		attrs = append(attrs, "stacktrace", getStackTrace())
	}
	
	return &Logger{
		Logger: l.Logger.With(attrs...),
		config: l.config,
	}
}

// Business domain specific logging methods
func (l *Logger) LogRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, size int64) {
	l.WithContext(ctx).Info("HTTP request completed",
		"method", method,
		"path", path,
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
		"response_size", size,
	)
}

func (l *Logger) LogDatabaseQuery(ctx context.Context, query string, duration time.Duration, rowsAffected int64) {
	l.WithContext(ctx).Debug("Database query executed",
		"query", query,
		"duration_ms", duration.Milliseconds(),
		"rows_affected", rowsAffected,
	)
}

func (l *Logger) LogFileProcessing(ctx context.Context, filename string, recordsProcessed int, duration time.Duration) {
	l.WithContext(ctx).Info("File processing completed",
		"filename", filename,
		"records_processed", recordsProcessed,
		"duration_ms", duration.Milliseconds(),
	)
}

func (l *Logger) LogCacheOperation(ctx context.Context, operation, key string, hit bool, duration time.Duration) {
	l.WithContext(ctx).Debug("Cache operation",
		"operation", operation,
		"key", key,
		"hit", hit,
		"duration_ms", duration.Milliseconds(),
	)
}

func (l *Logger) LogS3Operation(ctx context.Context, operation, bucket, key string, size int64, duration time.Duration) {
	l.WithContext(ctx).Debug("S3 operation",
		"operation", operation,
		"bucket", bucket,
		"key", key,
		"size", size,
		"duration_ms", duration.Milliseconds(),
	)
}

func (l *Logger) LogMetric(ctx context.Context, name string, value float64, labels map[string]string) {
	attrs := []any{
		"metric_name", name,
		"metric_value", value,
	}
	
	for k, v := range labels {
		attrs = append(attrs, "label_"+k, v)
	}
	
	l.WithContext(ctx).Debug("Metric recorded", attrs...)
}

func (l *Logger) LogBusinessEvent(ctx context.Context, event string, entityType string, entityID string, details map[string]any) {
	attrs := []any{
		"event", event,
		"entity_type", entityType,
		"entity_id", entityID,
	}
	
	for k, v := range details {
		attrs = append(attrs, k, v)
	}
	
	l.WithContext(ctx).Info("Business event", attrs...)
}

// Error handling methods
func (l *Logger) LogError(ctx context.Context, err error, msg string, args ...any) {
	if err == nil {
		return
	}
	
	attrs := append(args, "error", err.Error())
	if l.config.EnableStacktrace {
		attrs = append(attrs, "stacktrace", getStackTrace())
	}
	
	l.WithContext(ctx).Error(msg, attrs...)
}

func (l *Logger) LogPanic(ctx context.Context, recovered any, msg string, args ...any) {
	attrs := append(args, "panic", recovered, "stacktrace", getStackTrace())
	l.WithContext(ctx).Error(msg, attrs...)
}

// Utility methods
func (l *Logger) IsDebugEnabled() bool {
	return l.Logger.Enabled(context.Background(), slog.LevelDebug)
}

func (l *Logger) IsInfoEnabled() bool {
	return l.Logger.Enabled(context.Background(), slog.LevelInfo)
}

func (l *Logger) IsWarnEnabled() bool {
	return l.Logger.Enabled(context.Background(), slog.LevelWarn)
}

func (l *Logger) IsErrorEnabled() bool {
	return l.Logger.Enabled(context.Background(), slog.LevelError)
}

// Helper functions
func parseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, nil
	}
}

func createWriter(config *Config) (io.Writer, error) {
	switch config.Output {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	case "file":
		if config.FilePath == "" {
			return nil, fmt.Errorf("file path is required for file output")
		}
		
		return &lumberjack.Logger{
			Filename:   config.FilePath,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		}, nil
	default:
		return os.Stdout, nil
	}
}

func shortenFilePath(path string) string {
	// Extract just the filename and parent directory
	parts := strings.Split(path, "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}

func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// Context helpers
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, ContextKeyCorrelationID, correlationID)
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ContextKeyTraceID, traceID)
}

func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, ContextKeySpanID, spanID)
}

func GetRequestID(ctx context.Context) string {
	if id := ctx.Value(ContextKeyRequestID); id != nil {
		return id.(string)
	}
	return ""
}

func GetUserID(ctx context.Context) string {
	if id := ctx.Value(ContextKeyUserID); id != nil {
		return id.(string)
	}
	return ""
}

func GetCorrelationID(ctx context.Context) string {
	if id := ctx.Value(ContextKeyCorrelationID); id != nil {
		return id.(string)
	}
	return ""
}

func GetTraceID(ctx context.Context) string {
	if id := ctx.Value(ContextKeyTraceID); id != nil {
		return id.(string)
	}
	return ""
}

func GetSpanID(ctx context.Context) string {
	if id := ctx.Value(ContextKeySpanID); id != nil {
		return id.(string)
	}
	return ""
}

// GenerateRequestID generates a new request ID
func GenerateRequestID() string {
	return uuid.New().String()
}

// GenerateCorrelationID generates a new correlation ID
func GenerateCorrelationID() string {
	return uuid.New().String()
}

// Middleware for HTTP requests
func RequestLoggerMiddleware(logger *Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Generate request ID if not present
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = GenerateRequestID()
			}
			
			// Generate correlation ID if not present
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = GenerateCorrelationID()
			}
			
			// Add IDs to context
			ctx := WithRequestID(r.Context(), requestID)
			ctx = WithCorrelationID(ctx, correlationID)
			
			// Add trace information if available
			if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
				ctx = WithTraceID(ctx, traceID)
			}
			if spanID := r.Header.Get("X-Span-ID"); spanID != "" {
				ctx = WithSpanID(ctx, spanID)
			}
			
			// Update request with new context
			r = r.WithContext(ctx)
			
			// Set response headers
			w.Header().Set("X-Request-ID", requestID)
			w.Header().Set("X-Correlation-ID", correlationID)
			
			// Wrap response writer to capture status code and size
			ww := &responseWriter{ResponseWriter: w, statusCode: 200}
			
			// Log request start
			logger.InfoContext(ctx, "HTTP request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
			
			// Process request
			next.ServeHTTP(ww, r)
			
			// Log request completion
			duration := time.Since(start)
			logger.LogRequest(ctx, r.Method, r.URL.Path, ww.statusCode, duration, ww.size)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.size += int64(n)
	return n, err
}

// Recovery middleware for panic handling
func RecoveryMiddleware(logger *Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.LogPanic(r.Context(), recovered, "HTTP request panic")
					
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error": "Internal server error"}`))
				}
			}()
			
			next.ServeHTTP(w, r)
		})
	}
}

// Global logger instance
var defaultLogger *Logger

func init() {
	defaultLogger = NewDefault()
}

// Global logging functions
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.DebugContext(ctx, msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.InfoContext(ctx, msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.WarnContext(ctx, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.ErrorContext(ctx, msg, args...)
}

// SetDefault sets the default logger
func SetDefault(logger *Logger) {
	defaultLogger = logger
}