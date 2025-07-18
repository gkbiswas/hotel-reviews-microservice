package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// ErrorHandlerMiddleware provides centralized error handling for HTTP requests
type ErrorHandlerMiddleware struct {
	errorHandler *infrastructure.ErrorHandler
	logger       *logger.Logger
}

// NewErrorHandlerMiddleware creates a new error handler middleware
func NewErrorHandlerMiddleware(errorHandler *infrastructure.ErrorHandler, logger *logger.Logger) *ErrorHandlerMiddleware {
	return &ErrorHandlerMiddleware{
		errorHandler: errorHandler,
		logger:       logger,
	}
}

// Handle returns the middleware handler function
func (ehm *ErrorHandlerMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create error-aware response writer
		eww := &errorAwareResponseWriter{
			ResponseWriter: w,
			request:        r,
			errorHandler:   ehm.errorHandler,
			logger:         ehm.logger,
		}
		
		// Set up panic recovery
		defer func() {
			if recovered := recover(); recovered != nil {
				ehm.handlePanic(eww, r, recovered)
			}
		}()
		
		// Continue with the next handler
		next.ServeHTTP(eww, r)
	})
}

// errorAwareResponseWriter wraps http.ResponseWriter to provide error handling
type errorAwareResponseWriter struct {
	http.ResponseWriter
	request        *http.Request
	errorHandler   *infrastructure.ErrorHandler
	logger         *logger.Logger
	statusCode     int
	bytesWritten   int64
	headerWritten  bool
}

// WriteHeader captures the status code and handles errors
func (w *errorAwareResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	
	// Check if this is an error status code
	if statusCode >= 400 && !w.headerWritten {
		// Create error from status code
		err := w.createErrorFromStatusCode(statusCode)
		if err != nil {
			// Handle the error through the error handler
			appErr := w.errorHandler.Handle(w.request.Context(), err)
			if appErr != nil {
				// Update response based on error handling
				w.updateResponseForError(appErr)
			}
		}
	}
	
	w.headerWritten = true
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response data
func (w *errorAwareResponseWriter) Write(data []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	
	n, err := w.ResponseWriter.Write(data)
	w.bytesWritten += int64(n)
	return n, err
}

// createErrorFromStatusCode creates an error from HTTP status code
func (w *errorAwareResponseWriter) createErrorFromStatusCode(statusCode int) error {
	switch statusCode {
	case http.StatusBadRequest:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeValidation,
			Code:        "BAD_REQUEST",
			Message:     "Bad request",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusUnauthorized:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeAuthentication,
			Code:        "UNAUTHORIZED",
			Message:     "Unauthorized",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusForbidden:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeAuthorization,
			Code:        "FORBIDDEN",
			Message:     "Forbidden",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusNotFound:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeNotFound,
			Code:        "NOT_FOUND",
			Message:     "Resource not found",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusConflict:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeConflict,
			Code:        "CONFLICT",
			Message:     "Resource conflict",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusTooManyRequests:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeRateLimit,
			Code:        "TOO_MANY_REQUESTS",
			Message:     "Too many requests",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusInternalServerError:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeSystem,
			Code:        "INTERNAL_SERVER_ERROR",
			Message:     "Internal server error",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusBadGateway:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeNetwork,
			Code:        "BAD_GATEWAY",
			Message:     "Bad gateway",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusServiceUnavailable:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeCircuitBreaker,
			Code:        "SERVICE_UNAVAILABLE",
			Message:     "Service unavailable",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	case http.StatusGatewayTimeout:
		return &infrastructure.AppError{
			Type:        infrastructure.ErrorTypeTimeout,
			Code:        "GATEWAY_TIMEOUT",
			Message:     "Gateway timeout",
			HTTPStatus:  statusCode,
			Timestamp:   time.Now(),
		}
	default:
		if statusCode >= 500 {
			return &infrastructure.AppError{
				Type:        infrastructure.ErrorTypeSystem,
				Code:        "SERVER_ERROR",
				Message:     "Server error",
				HTTPStatus:  statusCode,
				Timestamp:   time.Now(),
			}
		} else if statusCode >= 400 {
			return &infrastructure.AppError{
				Type:        infrastructure.ErrorTypeClient,
				Code:        "CLIENT_ERROR",
				Message:     "Client error",
				HTTPStatus:  statusCode,
				Timestamp:   time.Now(),
			}
		}
		return nil
	}
}

// updateResponseForError updates the response based on error handling
func (w *errorAwareResponseWriter) updateResponseForError(appErr *infrastructure.AppError) {
	// Add error-specific headers
	w.Header().Set("X-Error-ID", appErr.ID)
	w.Header().Set("X-Error-Type", string(appErr.Type))
	w.Header().Set("X-Error-Code", appErr.Code)
	
	if appErr.CorrelationID != "" {
		w.Header().Set("X-Correlation-ID", appErr.CorrelationID)
	}
	
	if appErr.RetryAfter != nil {
		w.Header().Set("Retry-After", appErr.RetryAfter.String())
	}
	
	// Update status code if needed
	if appErr.HTTPStatus != 0 && appErr.HTTPStatus != w.statusCode {
		w.statusCode = appErr.HTTPStatus
	}
}

// handlePanic handles panics and converts them to errors
func (ehm *ErrorHandlerMiddleware) handlePanic(w *errorAwareResponseWriter, r *http.Request, recovered interface{}) {
	// Create error from panic
	panicErr := &infrastructure.AppError{
		Type:        infrastructure.ErrorTypeSystem,
		Category:    infrastructure.CategoryCritical,
		Severity:    infrastructure.SeverityCritical,
		Code:        "PANIC",
		Message:     "Application panic occurred",
		Details: map[string]interface{}{
			"panic_value": recovered,
			"request_uri": r.RequestURI,
			"method":      r.Method,
		},
		Timestamp:  time.Now(),
		HTTPStatus: http.StatusInternalServerError,
	}
	
	// Handle the panic error
	appErr := ehm.errorHandler.Handle(r.Context(), panicErr)
	
	// Write error response
	if !w.headerWritten {
		ehm.errorHandler.HandleHTTP(r.Context(), w, r, appErr)
	}
	
	// Log the panic
	ehm.logger.ErrorContext(r.Context(), "Application panic recovered",
		"panic_value", recovered,
		"error_id", appErr.ID,
		"request_uri", r.RequestURI,
		"method", r.Method,
	)
}

// ErrorHandlerFunc provides a function-based error handler for use in handlers
type ErrorHandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)

// NewErrorHandlerFunc creates a new error handler function
func NewErrorHandlerFunc(errorHandler *infrastructure.ErrorHandler) ErrorHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		if err == nil {
			return
		}
		
		errorHandler.HandleHTTP(ctx, w, r, err)
	}
}

// ErrorHandlerService provides error handling utilities for services
type ErrorHandlerService struct {
	errorHandler *infrastructure.ErrorHandler
	logger       *logger.Logger
}

// NewErrorHandlerService creates a new error handler service
func NewErrorHandlerService(errorHandler *infrastructure.ErrorHandler, logger *logger.Logger) *ErrorHandlerService {
	return &ErrorHandlerService{
		errorHandler: errorHandler,
		logger:       logger,
	}
}

// WrapError wraps an error with additional context
func (ehs *ErrorHandlerService) WrapError(ctx context.Context, err error, message string, details map[string]interface{}) error {
	if err == nil {
		return nil
	}
	
	// Create enhanced error
	wrappedErr := &infrastructure.AppError{
		Type:          infrastructure.ErrorTypeSystem,
		Category:      infrastructure.CategoryTransient,
		Severity:      infrastructure.SeverityMedium,
		Code:          "WRAPPED_ERROR",
		Message:       message,
		Details:       details,
		Timestamp:     time.Now(),
		Cause:         err,
		CorrelationID: logger.GetCorrelationID(ctx),
		RequestID:     logger.GetRequestID(ctx),
		UserID:        logger.GetUserID(ctx),
	}
	
	return ehs.errorHandler.Handle(ctx, wrappedErr)
}

// HandleServiceError handles service-level errors
func (ehs *ErrorHandlerService) HandleServiceError(ctx context.Context, err error, service string) error {
	if err == nil {
		return nil
	}
	
	// Create service error
	serviceErr := &infrastructure.AppError{
		Type:      infrastructure.ErrorTypeSystem,
		Category:  infrastructure.CategoryTransient,
		Severity:  infrastructure.SeverityMedium,
		Code:      "SERVICE_ERROR",
		Message:   err.Error(),
		Details: map[string]interface{}{
			"service": service,
		},
		Timestamp:     time.Now(),
		Source:        service,
		Cause:         err,
		CorrelationID: logger.GetCorrelationID(ctx),
		RequestID:     logger.GetRequestID(ctx),
		UserID:        logger.GetUserID(ctx),
	}
	
	return ehs.errorHandler.Handle(ctx, serviceErr)
}

// HandleDatabaseError handles database-specific errors
func (ehs *ErrorHandlerService) HandleDatabaseError(ctx context.Context, err error, operation string) error {
	if err == nil {
		return nil
	}
	
	// Create database error
	dbErr := &infrastructure.AppError{
		Type:      infrastructure.ErrorTypeDatabase,
		Category:  infrastructure.CategoryTransient,
		Severity:  infrastructure.SeverityHigh,
		Code:      "DATABASE_ERROR",
		Message:   err.Error(),
		Details: map[string]interface{}{
			"operation": operation,
		},
		Timestamp:     time.Now(),
		Source:        "database",
		Cause:         err,
		Retryable:     true,
		CorrelationID: logger.GetCorrelationID(ctx),
		RequestID:     logger.GetRequestID(ctx),
		UserID:        logger.GetUserID(ctx),
	}
	
	return ehs.errorHandler.Handle(ctx, dbErr)
}

// HandleValidationError handles validation errors
func (ehs *ErrorHandlerService) HandleValidationError(ctx context.Context, err error, field string) error {
	if err == nil {
		return nil
	}
	
	// Create validation error
	validationErr := &infrastructure.AppError{
		Type:      infrastructure.ErrorTypeValidation,
		Category:  infrastructure.CategoryPermanent,
		Severity:  infrastructure.SeverityLow,
		Code:      "VALIDATION_ERROR",
		Message:   err.Error(),
		UserMessage: "Please check your input and try again.",
		Details: map[string]interface{}{
			"field": field,
		},
		Timestamp:     time.Now(),
		HTTPStatus:    http.StatusBadRequest,
		Retryable:     false,
		CorrelationID: logger.GetCorrelationID(ctx),
		RequestID:     logger.GetRequestID(ctx),
		UserID:        logger.GetUserID(ctx),
	}
	
	return ehs.errorHandler.Handle(ctx, validationErr)
}

// HandleExternalServiceError handles external service errors
func (ehs *ErrorHandlerService) HandleExternalServiceError(ctx context.Context, err error, service string) error {
	if err == nil {
		return nil
	}
	
	// Create external service error
	extErr := &infrastructure.AppError{
		Type:      infrastructure.ErrorTypeExternal,
		Category:  infrastructure.CategoryTransient,
		Severity:  infrastructure.SeverityMedium,
		Code:      "EXTERNAL_SERVICE_ERROR",
		Message:   err.Error(),
		Details: map[string]interface{}{
			"external_service": service,
		},
		Timestamp:     time.Now(),
		Source:        service,
		Cause:         err,
		Retryable:     true,
		CorrelationID: logger.GetCorrelationID(ctx),
		RequestID:     logger.GetRequestID(ctx),
		UserID:        logger.GetUserID(ctx),
	}
	
	return ehs.errorHandler.Handle(ctx, extErr)
}

// CreateBusinessError creates a business logic error
func (ehs *ErrorHandlerService) CreateBusinessError(ctx context.Context, code, message, userMessage string, details map[string]interface{}) error {
	businessErr := &infrastructure.AppError{
		Type:        infrastructure.ErrorTypeBusiness,
		Category:    infrastructure.CategoryPermanent,
		Severity:    infrastructure.SeverityMedium,
		Code:        code,
		Message:     message,
		UserMessage: userMessage,
		Details:     details,
		Timestamp:   time.Now(),
		HTTPStatus:  http.StatusBadRequest,
		Retryable:   false,
		CorrelationID: logger.GetCorrelationID(ctx),
		RequestID:     logger.GetRequestID(ctx),
		UserID:        logger.GetUserID(ctx),
	}
	
	return ehs.errorHandler.Handle(ctx, businessErr)
}

// IsRetryableError checks if an error is retryable
func (ehs *ErrorHandlerService) IsRetryableError(err error) bool {
	if appErr, ok := err.(*infrastructure.AppError); ok {
		return appErr.IsRetryable()
	}
	return false
}

// IsCriticalError checks if an error is critical
func (ehs *ErrorHandlerService) IsCriticalError(err error) bool {
	if appErr, ok := err.(*infrastructure.AppError); ok {
		return appErr.IsCritical()
	}
	return false
}

// IsTemporaryError checks if an error is temporary
func (ehs *ErrorHandlerService) IsTemporaryError(err error) bool {
	if appErr, ok := err.(*infrastructure.AppError); ok {
		return appErr.IsTemporary()
	}
	return false
}

// GetErrorCode gets the error code from an error
func (ehs *ErrorHandlerService) GetErrorCode(err error) string {
	if appErr, ok := err.(*infrastructure.AppError); ok {
		return appErr.Code
	}
	return "UNKNOWN_ERROR"
}

// GetErrorType gets the error type from an error
func (ehs *ErrorHandlerService) GetErrorType(err error) infrastructure.ErrorType {
	if appErr, ok := err.(*infrastructure.AppError); ok {
		return appErr.Type
	}
	return infrastructure.ErrorTypeSystem
}

// GetRetryAfter gets the retry after duration from an error
func (ehs *ErrorHandlerService) GetRetryAfter(err error) *time.Duration {
	if appErr, ok := err.(*infrastructure.AppError); ok {
		return appErr.RetryAfter
	}
	return nil
}