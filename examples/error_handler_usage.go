//go:build examples
// +build examples

package main
import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure/middleware"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/gorilla/mux"
)
// This example demonstrates how to use the comprehensive error handling system
func main() {
	// Initialize logger
	loggerInstance := logger.NewDefault()
	// Create error handler configuration
	config := &infrastructure.ErrorHandlerConfig{
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
		DefaultFormat:          infrastructure.FormatJSON,
		IncludeInternalErrors:  false,
		SanitizeUserData:       true,
	}
	// Create circuit breaker (if needed)
	circuitBreaker := infrastructure.NewDatabaseCircuitBreaker(loggerInstance)
	// Create retry config (if needed)
	retryConfig := &infrastructure.RetryConfig{
		MaxRetries:      3,
		InitialDelay:    time.Second,
		MaxDelay:        10 * time.Second,
		BackoffFactor:   2.0,
		EnableJitter:    true,
		RetryableErrors: []infrastructure.ErrorType{infrastructure.ErrorTypeNetwork, infrastructure.ErrorTypeTimeout},
	}
	// Create error handler
	errorHandler := infrastructure.NewErrorHandler(config, loggerInstance, circuitBreaker, retryConfig)
	defer errorHandler.Close()
	// Set up alert channels
	setupAlertChannels(errorHandler, loggerInstance)
	// Create error handler service for use in business logic
	errorService := middleware.NewErrorHandlerService(errorHandler, loggerInstance)
	// Create HTTP router
	router := mux.NewRouter()
	// Add error handler middleware
	errorMiddleware := middleware.NewErrorHandlerMiddleware(errorHandler, loggerInstance)
	router.Use(errorMiddleware.Handle)
	// Add example routes
	addExampleRoutes(router, errorService, loggerInstance)
	// Start server
	fmt.Println("Starting server on :8080...")
	fmt.Println("Try these endpoints:")
	fmt.Println("- GET /api/validation-error")
	fmt.Println("- GET /api/database-error")
	fmt.Println("- GET /api/external-error")
	fmt.Println("- GET /api/business-error")
	fmt.Println("- GET /api/panic-error")
	fmt.Println("- GET /api/success")
	fmt.Println("- GET /api/health")
	fmt.Println("- GET /api/metrics")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
func setupAlertChannels(errorHandler *infrastructure.ErrorHandler, loggerInstance *logger.Logger) {
	// Add logger alert channel
	loggerChannel := infrastructure.NewLoggerAlertChannel(loggerInstance)
	errorHandler.AddChannel(loggerChannel)
	// Add console alert channel
	consoleChannel := infrastructure.NewConsoleAlertChannel(loggerInstance)
	errorHandler.AddChannel(consoleChannel)
	// Add email alert channel (commented out - requires configuration)
	/*
		emailConfig := &infrastructure.EmailConfig{
			Host:     "smtp.gmail.com",
			Port:     587,
			Username: "your-email@gmail.com",
			Password: "your-password",
			From:     "your-email@gmail.com",
			To:       []string{"alerts@company.com"},
		}
		emailChannel := infrastructure.NewEmailAlertChannel(emailConfig, loggerInstance)
		errorHandler.AddChannel(emailChannel)
	*/
	// Add Slack alert channel (commented out - requires webhook URL)
	/*
		slackConfig := &infrastructure.SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
			Channel:    "#alerts",
			Username:   "Error Handler",
			Timeout:    30 * time.Second,
		}
		slackChannel := infrastructure.NewSlackAlertChannel(slackConfig, loggerInstance)
		errorHandler.AddChannel(slackChannel)
	*/
	// Add webhook alert channel (commented out - requires webhook URL)
	/*
		webhookConfig := &infrastructure.WebhookConfig{
			URL:     "https://your-webhook-url.com/alerts",
			Method:  "POST",
			Headers: map[string]string{
				"Authorization": "Bearer your-token",
			},
			Timeout: 30 * time.Second,
		}
		webhookChannel := infrastructure.NewWebhookAlertChannel(webhookConfig, loggerInstance)
		errorHandler.AddChannel(webhookChannel)
	*/
}
func addExampleRoutes(router *mux.Router, errorService *middleware.ErrorHandlerService, loggerInstance *logger.Logger) {
	// Example: Validation error
	router.HandleFunc("/api/validation-error", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Simulate validation error
		validationErr := errors.New("email is required")
		err := errorService.HandleValidationError(ctx, validationErr, "email")
		if err != nil {
			// Error is already handled, just return
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This shouldn't be reached"))
	}).Methods("GET")
	// Example: Database error
	router.HandleFunc("/api/database-error", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Simulate database error
		dbErr := errors.New("connection to database failed")
		err := errorService.HandleDatabaseError(ctx, dbErr, "user_query")
		if err != nil {
			// Error is already handled, just return
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This shouldn't be reached"))
	}).Methods("GET")
	// Example: External service error
	router.HandleFunc("/api/external-error", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Simulate external service error
		extErr := errors.New("payment gateway timeout")
		err := errorService.HandleExternalServiceError(ctx, extErr, "payment_service")
		if err != nil {
			// Error is already handled, just return
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This shouldn't be reached"))
	}).Methods("GET")
	// Example: Business logic error
	router.HandleFunc("/api/business-error", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Simulate business logic error
		err := errorService.CreateBusinessError(
			ctx,
			"INSUFFICIENT_FUNDS",
			"User has insufficient funds for this transaction",
			"You don't have enough money in your account",
			map[string]interface{}{
				"user_id":         "123",
				"required_amount": 100.00,
				"current_balance": 50.00,
			},
		)
		if err != nil {
			// Error is already handled, just return
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This shouldn't be reached"))
	}).Methods("GET")
	// Example: Panic error (will be caught by middleware)
	router.HandleFunc("/api/panic-error", func(w http.ResponseWriter, r *http.Request) {
		// This will trigger panic recovery in middleware
		panic("Something went terribly wrong!")
	}).Methods("GET")
	// Example: Success response
	router.HandleFunc("/api/success", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Success!", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
	}).Methods("GET")
	// Health check endpoint
	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		// In a real application, you would get this from your error handler
		healthStatus := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// This is a simplified JSON marshal - in production use proper JSON handling
		response := fmt.Sprintf(`{
			"status": "%s",
			"timestamp": "%s",
			"version": "%s"
		}`,
			healthStatus["status"],
			healthStatus["timestamp"],
			healthStatus["version"])
		w.Write([]byte(response))
	}).Methods("GET")
	// Metrics endpoint
	router.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		// In a real application, you would get this from your error handler
		metrics := map[string]interface{}{
			"total_errors": 0,
			"error_rate":   0.0,
			"last_error":   nil,
			"timestamp":    time.Now().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// This is a simplified JSON marshal - in production use proper JSON handling
		response := fmt.Sprintf(`{
			"total_errors": %d,
			"error_rate": %.2f,
			"last_error": null,
			"timestamp": "%s"
		}`,
			0,
			0.0,
			metrics["timestamp"])
		w.Write([]byte(response))
	}).Methods("GET")
}
// Example of using error handler in business logic
func exampleBusinessLogic(ctx context.Context, errorService *middleware.ErrorHandlerService) error {
	// Example: Database operation
	if err := performDatabaseOperation(ctx); err != nil {
		return errorService.HandleDatabaseError(ctx, err, "get_user")
	}
	// Example: Validation
	if err := validateUserInput("invalid-email"); err != nil {
		return errorService.HandleValidationError(ctx, err, "email")
	}
	// Example: External service call
	if err := callExternalService(ctx); err != nil {
		return errorService.HandleExternalServiceError(ctx, err, "payment_gateway")
	}
	// Example: Business logic validation
	if !hasPermission(ctx, "read_user") {
		return errorService.CreateBusinessError(
			ctx,
			"INSUFFICIENT_PERMISSIONS",
			"User does not have permission to read user data",
			"You don't have permission to view this information",
			map[string]interface{}{
				"required_permission": "read_user",
				"user_id":             "123",
			},
		)
	}
	return nil
}
// Mock functions for demonstration
func performDatabaseOperation(ctx context.Context) error {
	// Simulate database operation
	return nil
}
func validateUserInput(email string) error {
	if email == "invalid-email" {
		return errors.New("invalid email format")
	}
	return nil
}
func callExternalService(ctx context.Context) error {
	// Simulate external service call
	return nil
}
func hasPermission(ctx context.Context, permission string) bool {
	// Simulate permission check
	return false
}
// Example of using error handler with circuit breaker
func exampleWithCircuitBreaker(ctx context.Context, circuitBreaker *infrastructure.CircuitBreaker) error {
	return circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		// Simulate operation that might fail
		return nil, errors.New("service unavailable")
	})
}
// Example of using error handler with retry logic
func exampleWithRetry(ctx context.Context, errorService *middleware.ErrorHandlerService) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := performNetworkOperation(ctx); err != nil {
			lastErr = err
			// Check if error is retryable
			if errorService.IsRetryableError(err) {
				// Wait before retry
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}
			// Non-retryable error, handle and return
			return errorService.HandleExternalServiceError(ctx, err, "network_service")
		}
		// Success
		return nil
	}
	// All retries failed
	return errorService.HandleExternalServiceError(ctx, lastErr, "network_service")
}
func performNetworkOperation(ctx context.Context) error {
	// Simulate network operation
	return errors.New("network timeout")
}
// Example of custom error types
func exampleCustomErrorTypes(ctx context.Context, errorService *middleware.ErrorHandlerService) error {
	// Create custom error with specific context
	customErr := &infrastructure.AppError{
		Type:        infrastructure.ErrorTypeBusiness,
		Category:    infrastructure.CategoryPermanent,
		Severity:    infrastructure.SeverityMedium,
		Code:        "CUSTOM_BUSINESS_ERROR",
		Message:     "This is a custom business error",
		UserMessage: "Something went wrong with your request",
		Details: map[string]interface{}{
			"custom_field": "custom_value",
			"error_source": "business_logic",
		},
		Context: map[string]interface{}{
			"request_id": "12345",
			"user_id":    "67890",
			"operation":  "custom_operation",
		},
		Timestamp:  time.Now(),
		HTTPStatus: http.StatusBadRequest,
		Retryable:  false,
		RetryAfter: nil,
	}
	return errorService.WrapError(ctx, customErr, "Custom error occurred", map[string]interface{}{
		"additional_context": "wrapped_error",
	})
}
// Example of error pattern matching
func exampleErrorPatternMatching(errorHandler *infrastructure.ErrorHandler) {
	// Add custom error pattern
	customPattern := infrastructure.ErrorPattern{
		Type:           infrastructure.ErrorTypeBusiness,
		MessagePattern: "subscription expired",
		SourcePattern:  "subscription_service",
		HTTPStatus:     http.StatusPaymentRequired,
		Category:       infrastructure.CategoryPermanent,
		Severity:       infrastructure.SeverityMedium,
		Retryable:      false,
	}
	errorHandler.AddErrorPattern(customPattern)
	// Now errors matching this pattern will be classified correctly
	ctx := context.Background()
	subscriptionErr := errors.New("user subscription expired")
	handledErr := errorHandler.Handle(ctx, subscriptionErr)
	fmt.Printf("Error classified as: %s\n", handledErr.Type)
	fmt.Printf("HTTP Status: %d\n", handledErr.HTTPStatus)
	fmt.Printf("Retryable: %t\n", handledErr.Retryable)
}
