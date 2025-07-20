package infrastructure

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// System errors
	ErrorTypeSystem         ErrorType = "system"
	ErrorTypeDatabase       ErrorType = "database"
	ErrorTypeNetwork        ErrorType = "network"
	ErrorTypeTimeout        ErrorType = "timeout"
	ErrorTypeCircuitBreaker ErrorType = "circuit_breaker"
	ErrorTypeRateLimit      ErrorType = "rate_limit"
	ErrorTypeResource       ErrorType = "resource"
	ErrorTypeConfiguration  ErrorType = "configuration"

	// Business errors
	ErrorTypeBusiness       ErrorType = "business"
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeAuthentication ErrorType = "authentication"
	ErrorTypeAuthorization  ErrorType = "authorization"
	ErrorTypeNotFound       ErrorType = "not_found"
	ErrorTypeConflict       ErrorType = "conflict"
	ErrorTypePrecondition   ErrorType = "precondition"

	// External errors
	ErrorTypeExternal   ErrorType = "external"
	ErrorTypeThirdParty ErrorType = "third_party"
	ErrorTypeUpstream   ErrorType = "upstream"
	ErrorTypeDownstream ErrorType = "downstream"

	// Client errors
	ErrorTypeClient           ErrorType = "client"
	ErrorTypeMalformedRequest ErrorType = "malformed_request"
	ErrorTypeUnsupported      ErrorType = "unsupported"
)

// ErrorCategory represents the category of error
type ErrorCategory string

const (
	CategoryTransient    ErrorCategory = "transient"     // Temporary errors that may resolve
	CategoryPermanent    ErrorCategory = "permanent"     // Permanent errors that won't resolve
	CategoryRetryable    ErrorCategory = "retryable"     // Errors that can be retried
	CategoryNonRetryable ErrorCategory = "non_retryable" // Errors that shouldn't be retried
	CategoryCritical     ErrorCategory = "critical"      // Critical errors requiring immediate attention
	CategoryWarning      ErrorCategory = "warning"       // Warning-level errors
	CategoryInfo         ErrorCategory = "info"          // Informational errors
)

// ErrorSeverity represents the severity of an error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ErrorFormat represents the format for error responses
type ErrorFormat string

const (
	FormatJSON ErrorFormat = "json"
	FormatXML  ErrorFormat = "xml"
	FormatText ErrorFormat = "text"
)

// AppError represents a structured application error
type AppError struct {
	ID            string                 `json:"id"`
	Type          ErrorType              `json:"type"`
	Category      ErrorCategory          `json:"category"`
	Severity      ErrorSeverity          `json:"severity"`
	Code          string                 `json:"code"`
	Message       string                 `json:"message"`
	UserMessage   string                 `json:"user_message,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
	Source        string                 `json:"source,omitempty"`
	StackTrace    string                 `json:"stack_trace,omitempty"`
	Cause         error                  `json:"-"`
	RetryAfter    *time.Duration         `json:"retry_after,omitempty"`
	HTTPStatus    int                    `json:"http_status"`
	Internal      bool                   `json:"internal"`
	Retryable     bool                   `json:"retryable"`
	Logged        bool                   `json:"logged"`
	Metrics       *ErrorMetrics          `json:"metrics,omitempty"`
}

// ErrorMetrics represents metrics for an error
type ErrorMetrics struct {
	Count         int64       `json:"count"`
	FirstSeen     time.Time   `json:"first_seen"`
	LastSeen      time.Time   `json:"last_seen"`
	Occurrences   []time.Time `json:"occurrences,omitempty"`
	AffectedUsers []string    `json:"affected_users,omitempty"`
	Sources       []string    `json:"sources,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("error type: %s, code: %s", e.Type, e.Code)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error is retryable
func (e *AppError) IsRetryable() bool {
	// Non-retryable category overrides everything
	if e.Category == CategoryNonRetryable {
		return false
	}
	return e.Retryable || e.Category == CategoryRetryable
}

// IsCritical returns true if the error is critical
func (e *AppError) IsCritical() bool {
	return e.Severity == SeverityCritical || e.Category == CategoryCritical
}

// IsTemporary returns true if the error is temporary
func (e *AppError) IsTemporary() bool {
	return e.Category == CategoryTransient
}

// ToJSON converts the error to JSON format
func (e *AppError) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToXML converts the error to XML format
func (e *AppError) ToXML() ([]byte, error) {
	// Create a simplified version for XML marshaling
	type XMLError struct {
		ID            string        `xml:"id"`
		Type          ErrorType     `xml:"type"`
		Category      ErrorCategory `xml:"category"`
		Severity      ErrorSeverity `xml:"severity"`
		Code          string        `xml:"code"`
		Message       string        `xml:"message"`
		UserMessage   string        `xml:"user_message,omitempty"`
		Timestamp     time.Time     `xml:"timestamp"`
		CorrelationID string        `xml:"correlation_id,omitempty"`
		RequestID     string        `xml:"request_id,omitempty"`
		UserID        string        `xml:"user_id,omitempty"`
		Source        string        `xml:"source,omitempty"`
		StackTrace    string        `xml:"stack_trace,omitempty"`
		HTTPStatus    int           `xml:"http_status"`
		Internal      bool          `xml:"internal"`
		Retryable     bool          `xml:"retryable"`
	}
	
	xmlErr := XMLError{
		ID:            e.ID,
		Type:          e.Type,
		Category:      e.Category,
		Severity:      e.Severity,
		Code:          e.Code,
		Message:       e.Message,
		UserMessage:   e.UserMessage,
		Timestamp:     e.Timestamp,
		CorrelationID: e.CorrelationID,
		RequestID:     e.RequestID,
		UserID:        e.UserID,
		Source:        e.Source,
		StackTrace:    e.StackTrace,
		HTTPStatus:    e.HTTPStatus,
		Internal:      e.Internal,
		Retryable:     e.Retryable,
	}
	
	return xml.Marshal(xmlErr)
}

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	Error     *AppError `json:"error"`
	Success   bool      `json:"success"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path,omitempty"`
	Method    string    `json:"method,omitempty"`
	Version   string    `json:"version,omitempty"`
	TraceID   string    `json:"trace_id,omitempty"`
}

// ErrorPattern represents a pattern for error detection
type ErrorPattern struct {
	Type           ErrorType      `json:"type"`
	MessagePattern string         `json:"message_pattern"`
	SourcePattern  string         `json:"source_pattern"`
	HTTPStatus     int            `json:"http_status"`
	Category       ErrorCategory  `json:"category"`
	Severity       ErrorSeverity  `json:"severity"`
	Retryable      bool           `json:"retryable"`
	RetryAfter     *time.Duration `json:"retry_after,omitempty"`
}

// ErrorHandlerConfig represents the configuration for error handling
type ErrorHandlerConfig struct {
	EnableMetrics          bool          `json:"enable_metrics"`
	EnableAlerting         bool          `json:"enable_alerting"`
	EnableStackTrace       bool          `json:"enable_stack_trace"`
	EnableDetailedLogging  bool          `json:"enable_detailed_logging"`
	EnableErrorAggregation bool          `json:"enable_error_aggregation"`
	EnableRateLimiting     bool          `json:"enable_rate_limiting"`
	MaxStackTraceDepth     int           `json:"max_stack_trace_depth"`
	ErrorRetentionPeriod   time.Duration `json:"error_retention_period"`
	MetricsInterval        time.Duration `json:"metrics_interval"`
	AlertingThreshold      int           `json:"alerting_threshold"`
	AlertingWindow         time.Duration `json:"alerting_window"`
	RateLimitWindow        time.Duration `json:"rate_limit_window"`
	RateLimitThreshold     int           `json:"rate_limit_threshold"`
	DefaultFormat          ErrorFormat   `json:"default_format"`
	IncludeInternalErrors  bool          `json:"include_internal_errors"`
	SanitizeUserData       bool          `json:"sanitize_user_data"`
}

// DefaultErrorHandlerConfig returns the default configuration
func DefaultErrorHandlerConfig() *ErrorHandlerConfig {
	return &ErrorHandlerConfig{
		EnableMetrics:          true,
		EnableAlerting:         true,
		EnableStackTrace:       true,
		EnableDetailedLogging:  true,
		EnableErrorAggregation: true,
		EnableRateLimiting:     true,
		MaxStackTraceDepth:     50,
		ErrorRetentionPeriod:   24 * time.Hour,
		MetricsInterval:        30 * time.Second,
		AlertingThreshold:      10,
		AlertingWindow:         5 * time.Minute,
		RateLimitWindow:        time.Minute,
		RateLimitThreshold:     100,
		DefaultFormat:          FormatJSON,
		IncludeInternalErrors:  false,
		SanitizeUserData:       true,
	}
}

// ErrorHandler handles all application errors
type ErrorHandler struct {
	config         *ErrorHandlerConfig
	logger         *logger.Logger
	patterns       []ErrorPattern
	aggregator     *ErrorAggregator
	metrics        *ErrorMetricsCollector
	alerter        *ErrorAlerter
	rateLimiter    *ErrorRateLimiter
	circuitBreaker *CircuitBreaker
	retryConfig    *RetryConfig
	healthChecker  *HealthChecker
	mu             sync.RWMutex
	errorStats     map[string]*ErrorMetrics
	errorHistory   map[string][]*AppError
	errorPatterns  map[string]int
	correlationMap map[string][]string
	shutdownChan   chan struct{}
	wg             sync.WaitGroup
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(
	config *ErrorHandlerConfig,
	logger *logger.Logger,
	circuitBreaker *CircuitBreaker,
	retryConfig *RetryConfig,
) *ErrorHandler {
	if config == nil {
		config = DefaultErrorHandlerConfig()
	}

	eh := &ErrorHandler{
		config:         config,
		logger:         logger,
		patterns:       defaultErrorPatterns(),
		circuitBreaker: circuitBreaker,
		retryConfig:    retryConfig,
		errorStats:     make(map[string]*ErrorMetrics),
		errorHistory:   make(map[string][]*AppError),
		errorPatterns:  make(map[string]int),
		correlationMap: make(map[string][]string),
		shutdownChan:   make(chan struct{}),
	}

	// Initialize components
	eh.aggregator = NewErrorAggregator(config, logger)
	if config.EnableMetrics {
		eh.metrics = NewErrorMetricsCollector(config, logger)
	}
	eh.alerter = NewErrorAlerter(config, logger)
	eh.rateLimiter = NewErrorRateLimiter(config, logger)
	eh.healthChecker = NewHealthChecker(config, logger)

	// Start background processes
	eh.startBackgroundProcesses()

	return eh
}

// Handle handles an error and returns a structured response
func (eh *ErrorHandler) Handle(ctx context.Context, err error) *AppError {
	if err == nil {
		return nil
	}

	// Check if it's already an AppError
	if appErr, ok := err.(*AppError); ok {
		return eh.enhanceError(ctx, appErr)
	}

	// Create new AppError from regular error
	appErr := eh.createAppError(ctx, err)
	return eh.enhanceError(ctx, appErr)
}

// HandleHTTP handles an error for HTTP responses
func (eh *ErrorHandler) HandleHTTP(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	appErr := eh.Handle(ctx, err)
	eh.writeHTTPResponse(ctx, w, r, appErr)
}

// createAppError creates an AppError from a regular error
func (eh *ErrorHandler) createAppError(ctx context.Context, err error) *AppError {
	// Check if error rate limiting is enabled
	if eh.config.EnableRateLimiting && eh.rateLimiter.ShouldLimit(ctx, err) {
		return eh.createRateLimitError(ctx, err)
	}

	// Classify error based on patterns
	pattern := eh.classifyError(err)

	appErr := &AppError{
		ID:          uuid.New().String(),
		Type:        pattern.Type,
		Category:    pattern.Category,
		Severity:    pattern.Severity,
		Code:        eh.generateErrorCode(pattern.Type, err),
		Message:     err.Error(),
		UserMessage: eh.generateUserMessage(pattern.Type, err),
		Details:     eh.extractErrorDetails(err),
		Context:     eh.extractErrorContext(ctx),
		Timestamp:   time.Now(),
		Source:      eh.getErrorSource(),
		Cause:       err,
		HTTPStatus:  pattern.HTTPStatus,
		Internal:    eh.isInternalError(pattern.Type),
		Retryable:   pattern.Retryable,
		Logged:      false,
		RetryAfter:  pattern.RetryAfter,
	}

	// Add stack trace if enabled
	if eh.config.EnableStackTrace {
		appErr.StackTrace = eh.getStackTrace(eh.config.MaxStackTraceDepth)
	}

	// Add correlation and request IDs
	appErr.CorrelationID = logger.GetCorrelationID(ctx)
	appErr.RequestID = logger.GetRequestID(ctx)
	appErr.UserID = logger.GetUserID(ctx)

	return appErr
}

// enhanceError enhances an existing AppError with additional information
func (eh *ErrorHandler) enhanceError(ctx context.Context, appErr *AppError) *AppError {
	// Initialize metrics if not present
	if appErr.Metrics == nil {
		appErr.Metrics = &ErrorMetrics{
			Count:     1,
			FirstSeen: appErr.Timestamp,
			LastSeen:  appErr.Timestamp,
		}
	}

	// Update error statistics
	eh.updateErrorStats(appErr)

	// Record metrics
	if eh.config.EnableMetrics && eh.metrics != nil {
		eh.metrics.RecordError(appErr)
	}

	// Check for error patterns
	if eh.config.EnableErrorAggregation {
		eh.aggregator.RecordError(appErr)
	}

	// Log error if not already logged
	if !appErr.Logged {
		eh.logError(ctx, appErr)
		appErr.Logged = true
	}

	// Check alerting conditions
	if eh.config.EnableAlerting {
		eh.alerter.CheckAlertConditions(appErr)
	}

	// Update health check status
	eh.healthChecker.RecordError(appErr)

	return appErr
}

// classifyError classifies an error based on patterns
func (eh *ErrorHandler) classifyError(err error) ErrorPattern {
	errMsg := err.Error()
	source := eh.getErrorSource()

	// Check built-in patterns
	for _, pattern := range eh.patterns {
		if eh.matchesPattern(errMsg, pattern.MessagePattern) ||
			eh.matchesPattern(source, pattern.SourcePattern) {
			return pattern
		}
	}

	// Check for specific error types
	switch {
	case eh.isTimeoutError(err):
		return ErrorPattern{
			Type:       ErrorTypeTimeout,
			HTTPStatus: http.StatusRequestTimeout,
			Category:   CategoryTransient,
			Severity:   SeverityMedium,
			Retryable:  true,
			RetryAfter: &[]time.Duration{time.Second * 5}[0],
		}
	case eh.isNetworkError(err):
		return ErrorPattern{
			Type:       ErrorTypeNetwork,
			HTTPStatus: http.StatusBadGateway,
			Category:   CategoryTransient,
			Severity:   SeverityMedium,
			Retryable:  true,
			RetryAfter: &[]time.Duration{time.Second * 3}[0],
		}
	case eh.isDatabaseError(err):
		return ErrorPattern{
			Type:       ErrorTypeDatabase,
			HTTPStatus: http.StatusInternalServerError,
			Category:   CategoryTransient,
			Severity:   SeverityHigh,
			Retryable:  true,
			RetryAfter: &[]time.Duration{time.Second * 2}[0],
		}
	case eh.isValidationError(err):
		return ErrorPattern{
			Type:       ErrorTypeValidation,
			HTTPStatus: http.StatusBadRequest,
			Category:   CategoryPermanent,
			Severity:   SeverityLow,
			Retryable:  false,
		}
	case eh.isAuthenticationError(err):
		return ErrorPattern{
			Type:       ErrorTypeAuthentication,
			HTTPStatus: http.StatusUnauthorized,
			Category:   CategoryPermanent,
			Severity:   SeverityMedium,
			Retryable:  false,
		}
	case eh.isAuthorizationError(err):
		return ErrorPattern{
			Type:       ErrorTypeAuthorization,
			HTTPStatus: http.StatusForbidden,
			Category:   CategoryPermanent,
			Severity:   SeverityMedium,
			Retryable:  false,
		}
	case eh.isNotFoundError(err):
		return ErrorPattern{
			Type:       ErrorTypeNotFound,
			HTTPStatus: http.StatusNotFound,
			Category:   CategoryPermanent,
			Severity:   SeverityLow,
			Retryable:  false,
		}
	case eh.isConflictError(err):
		return ErrorPattern{
			Type:       ErrorTypeConflict,
			HTTPStatus: http.StatusConflict,
			Category:   CategoryPermanent,
			Severity:   SeverityMedium,
			Retryable:  false,
		}
	case eh.isCircuitBreakerError(err):
		return ErrorPattern{
			Type:       ErrorTypeCircuitBreaker,
			HTTPStatus: http.StatusServiceUnavailable,
			Category:   CategoryTransient,
			Severity:   SeverityHigh,
			Retryable:  true,
			RetryAfter: &[]time.Duration{time.Second * 10}[0],
		}
	case eh.isRateLimitError(err):
		return ErrorPattern{
			Type:       ErrorTypeRateLimit,
			HTTPStatus: http.StatusTooManyRequests,
			Category:   CategoryTransient,
			Severity:   SeverityMedium,
			Retryable:  true,
			RetryAfter: &[]time.Duration{time.Second * 60}[0],
		}
	}

	// Default pattern
	return ErrorPattern{
		Type:       ErrorTypeSystem,
		HTTPStatus: http.StatusInternalServerError,
		Category:   CategoryTransient,
		Severity:   SeverityMedium,
		Retryable:  true,
		RetryAfter: &[]time.Duration{time.Second * 5}[0],
	}
}

// writeHTTPResponse writes an HTTP error response
func (eh *ErrorHandler) writeHTTPResponse(ctx context.Context, w http.ResponseWriter, r *http.Request, appErr *AppError) {
	// Determine response format
	format := eh.getResponseFormat(r)

	// Create error response
	response := &ErrorResponse{
		Error:     appErr,
		Success:   false,
		RequestID: appErr.RequestID,
		Timestamp: time.Now(),
		Path:      r.URL.Path,
		Method:    r.Method,
		Version:   "1.0",
		TraceID:   logger.GetTraceID(ctx),
	}

	// Sanitize response if needed
	if eh.config.SanitizeUserData {
		response = eh.sanitizeResponse(response)
	}

	// Set response headers
	w.Header().Set("Content-Type", eh.getContentType(format))
	w.Header().Set("X-Error-ID", appErr.ID)
	w.Header().Set("X-Error-Type", string(appErr.Type))

	if appErr.CorrelationID != "" {
		w.Header().Set("X-Correlation-ID", appErr.CorrelationID)
	}

	if appErr.RetryAfter != nil {
		w.Header().Set("Retry-After", strconv.Itoa(int(appErr.RetryAfter.Seconds())))
	}

	// Write response
	w.WriteHeader(appErr.HTTPStatus)

	var responseData []byte
	var err error

	switch format {
	case FormatJSON:
		responseData, err = json.Marshal(response)
	case FormatXML:
		responseData, err = xml.Marshal(response)
	case FormatText:
		responseData = []byte(appErr.Message)
	default:
		responseData, err = json.Marshal(response)
	}

	if err != nil {
		eh.logger.ErrorContext(ctx, "Failed to marshal error response", "error", err)
		w.Write([]byte(`{"error": "Internal server error"}`))
		return
	}

	w.Write(responseData)
}

// Error type checking methods
func (eh *ErrorHandler) isTimeoutError(err error) bool {
	return strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded") ||
		errors.Is(err, context.DeadlineExceeded)
}

func (eh *ErrorHandler) isNetworkError(err error) bool {
	return strings.Contains(err.Error(), "network") ||
		strings.Contains(err.Error(), "connection") ||
		strings.Contains(err.Error(), "no route to host") ||
		strings.Contains(err.Error(), "connection refused")
}

func (eh *ErrorHandler) isDatabaseError(err error) bool {
	return strings.Contains(err.Error(), "database") ||
		strings.Contains(err.Error(), "sql") ||
		strings.Contains(err.Error(), "postgres") ||
		strings.Contains(err.Error(), "mysql") ||
		strings.Contains(err.Error(), "gorm")
}

func (eh *ErrorHandler) isValidationError(err error) bool {
	return strings.Contains(err.Error(), "validation") ||
		strings.Contains(err.Error(), "invalid") ||
		strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "malformed")
}

func (eh *ErrorHandler) isAuthenticationError(err error) bool {
	return strings.Contains(err.Error(), "authentication") ||
		strings.Contains(err.Error(), "unauthorized") ||
		strings.Contains(err.Error(), "invalid credentials") ||
		strings.Contains(err.Error(), "login failed")
}

func (eh *ErrorHandler) isAuthorizationError(err error) bool {
	return strings.Contains(err.Error(), "authorization") ||
		strings.Contains(err.Error(), "forbidden") ||
		strings.Contains(err.Error(), "access denied") ||
		strings.Contains(err.Error(), "permission denied")
}

func (eh *ErrorHandler) isNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "record not found") ||
		strings.Contains(err.Error(), "does not exist")
}

func (eh *ErrorHandler) isConflictError(err error) bool {
	return strings.Contains(err.Error(), "conflict") ||
		strings.Contains(err.Error(), "already exists") ||
		strings.Contains(err.Error(), "duplicate") ||
		strings.Contains(err.Error(), "unique constraint")
}

func (eh *ErrorHandler) isCircuitBreakerError(err error) bool {
	return IsCircuitBreakerError(err) ||
		strings.Contains(err.Error(), "circuit breaker")
}

func (eh *ErrorHandler) isRateLimitError(err error) bool {
	return strings.Contains(err.Error(), "rate limit") ||
		strings.Contains(err.Error(), "too many requests") ||
		strings.Contains(err.Error(), "quota exceeded")
}

func (eh *ErrorHandler) isInternalError(errorType ErrorType) bool {
	return errorType == ErrorTypeSystem ||
		errorType == ErrorTypeDatabase ||
		errorType == ErrorTypeConfiguration ||
		errorType == ErrorTypeResource
}

// Helper methods
func (eh *ErrorHandler) matchesPattern(text, pattern string) bool {
	if pattern == "" {
		return false
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}

func (eh *ErrorHandler) generateErrorCode(errorType ErrorType, err error) string {
	base := strings.ToUpper(string(errorType))
	hash := fmt.Sprintf("%x", []byte(err.Error()))
	if len(hash) > 8 {
		hash = hash[:8]
	}
	return fmt.Sprintf("%s_%s", base, hash)
}

func (eh *ErrorHandler) generateUserMessage(errorType ErrorType, err error) string {
	switch errorType {
	case ErrorTypeValidation:
		return "Please check your input and try again."
	case ErrorTypeAuthentication:
		return "Please check your credentials and try again."
	case ErrorTypeAuthorization:
		return "You don't have permission to perform this action."
	case ErrorTypeNotFound:
		return "The requested resource was not found."
	case ErrorTypeConflict:
		return "The resource already exists or conflicts with existing data."
	case ErrorTypeTimeout:
		return "The request timed out. Please try again."
	case ErrorTypeNetwork:
		return "Network error occurred. Please check your connection."
	case ErrorTypeRateLimit:
		return "Too many requests. Please wait before trying again."
	case ErrorTypeCircuitBreaker:
		return "Service temporarily unavailable. Please try again later."
	default:
		return "An unexpected error occurred. Please try again."
	}
}

func (eh *ErrorHandler) extractErrorDetails(err error) map[string]interface{} {
	details := make(map[string]interface{})

	// Extract details from wrapped errors
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		details["underlying_error"] = unwrapped.Error()
	}

	// Extract details from error type
	details["error_type"] = fmt.Sprintf("%T", err)

	return details
}

func (eh *ErrorHandler) extractErrorContext(ctx context.Context) map[string]interface{} {
	context := make(map[string]interface{})

	// Add context values
	if requestID := logger.GetRequestID(ctx); requestID != "" {
		context["request_id"] = requestID
	}

	if userID := logger.GetUserID(ctx); userID != "" {
		context["user_id"] = userID
	}

	if traceID := logger.GetTraceID(ctx); traceID != "" {
		context["trace_id"] = traceID
	}

	return context
}

func (eh *ErrorHandler) getErrorSource() string {
	if pc, file, line, ok := runtime.Caller(3); ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			return fmt.Sprintf("%s:%d (%s)", file, line, fn.Name())
		}
		return fmt.Sprintf("%s:%d", file, line)
	}
	return "unknown"
}

func (eh *ErrorHandler) getStackTrace(maxDepth int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Limit stack trace depth
	lines := strings.Split(stack, "\n")
	if len(lines) > maxDepth*2 {
		lines = lines[:maxDepth*2]
	}

	return strings.Join(lines, "\n")
}

func (eh *ErrorHandler) getResponseFormat(r *http.Request) ErrorFormat {
	// Check Accept header
	accept := r.Header.Get("Accept")
	switch {
	case strings.Contains(accept, "application/json"):
		return FormatJSON
	case strings.Contains(accept, "application/xml"):
		return FormatXML
	case strings.Contains(accept, "text/plain"):
		return FormatText
	default:
		return eh.config.DefaultFormat
	}
}

func (eh *ErrorHandler) getContentType(format ErrorFormat) string {
	switch format {
	case FormatJSON:
		return "application/json"
	case FormatXML:
		return "application/xml"
	case FormatText:
		return "text/plain"
	default:
		return "application/json"
	}
}

func (eh *ErrorHandler) sanitizeResponse(response *ErrorResponse) *ErrorResponse {
	// Create a copy
	sanitized := *response
	sanitized.Error = &AppError{}
	*sanitized.Error = *response.Error

	// Remove sensitive information
	if sanitized.Error.Internal {
		sanitized.Error.StackTrace = ""
		sanitized.Error.Details = nil
		sanitized.Error.Context = nil
		sanitized.Error.Source = ""
	}

	return &sanitized
}

func (eh *ErrorHandler) createRateLimitError(ctx context.Context, err error) *AppError {
	return &AppError{
		ID:            uuid.New().String(),
		Type:          ErrorTypeRateLimit,
		Category:      CategoryTransient,
		Severity:      SeverityMedium,
		Code:          "RATE_LIMIT_EXCEEDED",
		Message:       "Rate limit exceeded",
		UserMessage:   "Too many requests. Please wait before trying again.",
		Timestamp:     time.Now(),
		HTTPStatus:    http.StatusTooManyRequests,
		Retryable:     true,
		RetryAfter:    &[]time.Duration{time.Minute}[0],
		CorrelationID: logger.GetCorrelationID(ctx),
		RequestID:     logger.GetRequestID(ctx),
		UserID:        logger.GetUserID(ctx),
	}
}

func (eh *ErrorHandler) updateErrorStats(appErr *AppError) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	key := fmt.Sprintf("%s:%s", appErr.Type, appErr.Code)

	if stats, exists := eh.errorStats[key]; exists {
		atomic.AddInt64(&stats.Count, 1)
		stats.LastSeen = appErr.Timestamp
		stats.Occurrences = append(stats.Occurrences, appErr.Timestamp)

		// Add user ID if present
		if appErr.UserID != "" {
			stats.AffectedUsers = append(stats.AffectedUsers, appErr.UserID)
		}

		// Add source if present
		if appErr.Source != "" {
			stats.Sources = append(stats.Sources, appErr.Source)
		}
	} else {
		eh.errorStats[key] = &ErrorMetrics{
			Count:       1,
			FirstSeen:   appErr.Timestamp,
			LastSeen:    appErr.Timestamp,
			Occurrences: []time.Time{appErr.Timestamp},
			AffectedUsers: func() []string {
				if appErr.UserID != "" {
					return []string{appErr.UserID}
				}
				return nil
			}(),
			Sources: func() []string {
				if appErr.Source != "" {
					return []string{appErr.Source}
				}
				return nil
			}(),
		}
	}
}

func (eh *ErrorHandler) logError(ctx context.Context, appErr *AppError) {
	fields := map[string]interface{}{
		"error_id":       appErr.ID,
		"error_type":     appErr.Type,
		"error_category": appErr.Category,
		"error_severity": appErr.Severity,
		"error_code":     appErr.Code,
		"http_status":    appErr.HTTPStatus,
		"retryable":      appErr.Retryable,
		"internal":       appErr.Internal,
	}

	if appErr.Details != nil {
		fields["details"] = appErr.Details
	}

	if appErr.Context != nil {
		fields["context"] = appErr.Context
	}

	if appErr.Source != "" {
		fields["source"] = appErr.Source
	}

	// Log based on severity
	switch appErr.Severity {
	case SeverityLow:
		eh.logger.WithFields(fields).InfoContext(ctx, appErr.Message)
	case SeverityMedium:
		eh.logger.WithFields(fields).WarnContext(ctx, appErr.Message)
	case SeverityHigh, SeverityCritical:
		// Include stack trace for high severity errors
		if appErr.StackTrace != "" {
			fields["stack_trace"] = appErr.StackTrace
		}
		eh.logger.WithFields(fields).ErrorContext(ctx, appErr.Message)
	}
}

func (eh *ErrorHandler) startBackgroundProcesses() {
	// Start metrics collection
	if eh.config.EnableMetrics {
		eh.wg.Add(1)
		go eh.metricsCollectionLoop()
	}

	// Start error cleanup
	eh.wg.Add(1)
	go eh.cleanupLoop()
}

func (eh *ErrorHandler) metricsCollectionLoop() {
	defer eh.wg.Done()

	ticker := time.NewTicker(eh.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			eh.collectMetrics()
		case <-eh.shutdownChan:
			return
		}
	}
}

func (eh *ErrorHandler) cleanupLoop() {
	defer eh.wg.Done()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			eh.cleanupOldErrors()
		case <-eh.shutdownChan:
			return
		}
	}
}

func (eh *ErrorHandler) collectMetrics() {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	// Collect error statistics
	for key, stats := range eh.errorStats {
		parts := strings.Split(key, ":")
		if len(parts) >= 2 && eh.metrics != nil {
			eh.metrics.UpdateErrorStats(parts[0], parts[1], stats)
		}
	}
}

func (eh *ErrorHandler) cleanupOldErrors() {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	cutoff := time.Now().Add(-eh.config.ErrorRetentionPeriod)

	// Clean up old error statistics
	for key, stats := range eh.errorStats {
		if stats.LastSeen.Before(cutoff) {
			delete(eh.errorStats, key)
		}
	}

	// Clean up old error history
	for key, history := range eh.errorHistory {
		filtered := make([]*AppError, 0)
		for _, err := range history {
			if err.Timestamp.After(cutoff) {
				filtered = append(filtered, err)
			}
		}

		if len(filtered) == 0 {
			delete(eh.errorHistory, key)
		} else {
			eh.errorHistory[key] = filtered
		}
	}
}

// Public API methods

// AddErrorPattern adds a custom error pattern
func (eh *ErrorHandler) AddErrorPattern(pattern ErrorPattern) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	eh.patterns = append(eh.patterns, pattern)
}

// GetErrorStats returns error statistics
func (eh *ErrorHandler) GetErrorStats() map[string]*ErrorMetrics {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	stats := make(map[string]*ErrorMetrics)
	for key, metrics := range eh.errorStats {
		stats[key] = metrics
	}

	return stats
}

// GetErrorHistory returns error history
func (eh *ErrorHandler) GetErrorHistory() map[string][]*AppError {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	history := make(map[string][]*AppError)
	for key, errors := range eh.errorHistory {
		history[key] = errors
	}

	return history
}

// IsHealthy returns true if the error handler is healthy
func (eh *ErrorHandler) IsHealthy() bool {
	return eh.healthChecker.IsHealthy()
}

// GetHealthStatus returns the health status
func (eh *ErrorHandler) GetHealthStatus() map[string]interface{} {
	return eh.healthChecker.GetHealthStatus()
}

// Close closes the error handler
func (eh *ErrorHandler) Close() error {
	close(eh.shutdownChan)
	eh.wg.Wait()

	// Close components
	if eh.metrics != nil {
		eh.metrics.Close()
	}
	if eh.alerter != nil {
		eh.alerter.Close()
	}
	if eh.healthChecker != nil {
		eh.healthChecker.Close()
	}

	return nil
}

// defaultErrorPatterns returns default error patterns
func defaultErrorPatterns() []ErrorPattern {
	return []ErrorPattern{
		{
			Type:           ErrorTypeTimeout,
			MessagePattern: "timeout",
			HTTPStatus:     http.StatusRequestTimeout,
			Category:       CategoryTransient,
			Severity:       SeverityMedium,
			Retryable:      true,
			RetryAfter:     &[]time.Duration{time.Second * 5}[0],
		},
		{
			Type:           ErrorTypeNetwork,
			MessagePattern: "network",
			HTTPStatus:     http.StatusBadGateway,
			Category:       CategoryTransient,
			Severity:       SeverityMedium,
			Retryable:      true,
			RetryAfter:     &[]time.Duration{time.Second * 3}[0],
		},
		{
			Type:           ErrorTypeDatabase,
			MessagePattern: "database",
			HTTPStatus:     http.StatusInternalServerError,
			Category:       CategoryTransient,
			Severity:       SeverityHigh,
			Retryable:      true,
			RetryAfter:     &[]time.Duration{time.Second * 2}[0],
		},
		{
			Type:           ErrorTypeValidation,
			MessagePattern: "validation",
			HTTPStatus:     http.StatusBadRequest,
			Category:       CategoryPermanent,
			Severity:       SeverityLow,
			Retryable:      false,
		},
		{
			Type:           ErrorTypeAuthentication,
			MessagePattern: "authentication",
			HTTPStatus:     http.StatusUnauthorized,
			Category:       CategoryPermanent,
			Severity:       SeverityMedium,
			Retryable:      false,
		},
		{
			Type:           ErrorTypeAuthorization,
			MessagePattern: "authorization",
			HTTPStatus:     http.StatusForbidden,
			Category:       CategoryPermanent,
			Severity:       SeverityMedium,
			Retryable:      false,
		},
		{
			Type:           ErrorTypeNotFound,
			MessagePattern: "not found",
			HTTPStatus:     http.StatusNotFound,
			Category:       CategoryPermanent,
			Severity:       SeverityLow,
			Retryable:      false,
		},
		{
			Type:           ErrorTypeConflict,
			MessagePattern: "conflict",
			HTTPStatus:     http.StatusConflict,
			Category:       CategoryPermanent,
			Severity:       SeverityMedium,
			Retryable:      false,
		},
	}
}
