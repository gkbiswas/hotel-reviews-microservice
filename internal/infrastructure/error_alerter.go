package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// AlertLevel represents the alert level
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeErrorRate       AlertType = "error_rate"
	AlertTypeErrorSpike      AlertType = "error_spike"
	AlertTypeNewError        AlertType = "new_error"
	AlertTypeCriticalError   AlertType = "critical_error"
	AlertTypeSystemDown      AlertType = "system_down"
	AlertTypeServiceDegraded AlertType = "service_degraded"
)

// AlertCondition represents an alert condition
type AlertCondition struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Type            AlertType     `json:"type"`
	Level           AlertLevel    `json:"level"`
	Description     string        `json:"description"`
	Threshold       float64       `json:"threshold"`
	Window          time.Duration `json:"window"`
	Cooldown        time.Duration `json:"cooldown"`
	ErrorTypes      []ErrorType   `json:"error_types"`
	Severities      []ErrorSeverity `json:"severities"`
	Enabled         bool          `json:"enabled"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// Alert represents an alert
type Alert struct {
	ID            string                 `json:"id"`
	ConditionID   string                 `json:"condition_id"`
	Type          AlertType              `json:"type"`
	Level         AlertLevel             `json:"level"`
	Title         string                 `json:"title"`
	Message       string                 `json:"message"`
	Details       map[string]interface{} `json:"details"`
	Context       map[string]interface{} `json:"context"`
	Timestamp     time.Time              `json:"timestamp"`
	Status        string                 `json:"status"` // active, resolved, suppressed
	AcknowledgedBy string                `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time            `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time            `json:"resolved_at,omitempty"`
	Count          int                   `json:"count"`
	LastSeen       time.Time             `json:"last_seen"`
	Tags           []string              `json:"tags"`
	Severity       ErrorSeverity         `json:"severity"`
	Source         string                `json:"source"`
	Runbook        string                `json:"runbook,omitempty"`
	Actions        []string              `json:"actions,omitempty"`
}

// AlertChannel represents an alert channel
type AlertChannel interface {
	Send(ctx context.Context, alert *Alert) error
	Name() string
	IsEnabled() bool
}

// ErrorAlerter manages error alerting
type ErrorAlerter struct {
	config        *ErrorHandlerConfig
	logger        *logger.Logger
	mu            sync.RWMutex
	
	// Alert management
	conditions    map[string]*AlertCondition
	activeAlerts  map[string]*Alert
	alertHistory  []*Alert
	channels      []AlertChannel
	
	// Alert suppression
	suppressions  map[string]time.Time
	rateLimits    map[string]*AlertRateLimit
	
	// Metrics tracking
	errorCounts   map[string]int64
	errorRates    map[string]float64
	lastUpdate    time.Time
	
	// Background processes
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	shutdownCh    chan struct{}
}

// AlertRateLimit represents rate limiting for alerts
type AlertRateLimit struct {
	Count      int
	Window     time.Duration
	LastReset  time.Time
	Timestamps []time.Time
}

// NewErrorAlerter creates a new error alerter
func NewErrorAlerter(config *ErrorHandlerConfig, logger *logger.Logger) *ErrorAlerter {
	ctx, cancel := context.WithCancel(context.Background())
	
	ea := &ErrorAlerter{
		config:       config,
		logger:       logger,
		conditions:   make(map[string]*AlertCondition),
		activeAlerts: make(map[string]*Alert),
		alertHistory: make([]*Alert, 0),
		channels:     make([]AlertChannel, 0),
		suppressions: make(map[string]time.Time),
		rateLimits:   make(map[string]*AlertRateLimit),
		errorCounts:  make(map[string]int64),
		errorRates:   make(map[string]float64),
		lastUpdate:   time.Now(),
		ctx:          ctx,
		cancel:       cancel,
		shutdownCh:   make(chan struct{}),
	}
	
	// Initialize default conditions
	ea.initializeDefaultConditions()
	
	// Start background processes
	ea.startBackgroundProcesses()
	
	return ea
}

// initializeDefaultConditions initializes default alert conditions
func (ea *ErrorAlerter) initializeDefaultConditions() {
	defaultConditions := []*AlertCondition{
		{
			ID:          "high_error_rate",
			Name:        "High Error Rate",
			Type:        AlertTypeErrorRate,
			Level:       AlertLevelError,
			Description: "Error rate exceeds threshold",
			Threshold:   10.0, // 10 errors per minute
			Window:      5 * time.Minute,
			Cooldown:    15 * time.Minute,
			ErrorTypes:  []ErrorType{ErrorTypeSystem, ErrorTypeDatabase, ErrorTypeNetwork},
			Severities:  []ErrorSeverity{SeverityHigh, SeverityCritical},
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "critical_error_spike",
			Name:        "Critical Error Spike",
			Type:        AlertTypeErrorSpike,
			Level:       AlertLevelCritical,
			Description: "Spike in critical errors detected",
			Threshold:   5.0, // 5 critical errors in short time
			Window:      2 * time.Minute,
			Cooldown:    10 * time.Minute,
			ErrorTypes:  []ErrorType{},
			Severities:  []ErrorSeverity{SeverityCritical},
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "new_error_pattern",
			Name:        "New Error Pattern",
			Type:        AlertTypeNewError,
			Level:       AlertLevelWarning,
			Description: "New error pattern detected",
			Threshold:   1.0, // Any new error pattern
			Window:      time.Hour,
			Cooldown:    30 * time.Minute,
			ErrorTypes:  []ErrorType{},
			Severities:  []ErrorSeverity{},
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "system_degradation",
			Name:        "System Degradation",
			Type:        AlertTypeServiceDegraded,
			Level:       AlertLevelError,
			Description: "System performance degradation detected",
			Threshold:   20.0, // 20% increase in errors
			Window:      10 * time.Minute,
			Cooldown:    20 * time.Minute,
			ErrorTypes:  []ErrorType{ErrorTypeTimeout, ErrorTypeCircuitBreaker},
			Severities:  []ErrorSeverity{SeverityMedium, SeverityHigh},
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	for _, condition := range defaultConditions {
		ea.conditions[condition.ID] = condition
	}
}

// CheckAlertConditions checks alert conditions for an error
func (ea *ErrorAlerter) CheckAlertConditions(appErr *AppError) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	// Update error tracking
	ea.updateErrorTracking(appErr)
	
	// Check each condition
	for _, condition := range ea.conditions {
		if !condition.Enabled {
			continue
		}
		
		if ea.shouldTriggerAlert(condition, appErr) {
			ea.triggerAlert(condition, appErr)
		}
	}
}

// updateErrorTracking updates error tracking metrics
func (ea *ErrorAlerter) updateErrorTracking(appErr *AppError) {
	key := string(appErr.Type)
	ea.errorCounts[key]++
	
	// Calculate error rate
	now := time.Now()
	if now.Sub(ea.lastUpdate) >= time.Minute {
		for errorType, count := range ea.errorCounts {
			ea.errorRates[errorType] = float64(count) / now.Sub(ea.lastUpdate).Minutes()
		}
		ea.lastUpdate = now
	}
}

// shouldTriggerAlert checks if an alert should be triggered
func (ea *ErrorAlerter) shouldTriggerAlert(condition *AlertCondition, appErr *AppError) bool {
	// Check if error type matches condition
	if len(condition.ErrorTypes) > 0 {
		match := false
		for _, errorType := range condition.ErrorTypes {
			if errorType == appErr.Type {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	
	// Check if severity matches condition
	if len(condition.Severities) > 0 {
		match := false
		for _, severity := range condition.Severities {
			if severity == appErr.Severity {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	
	// Check if in cooldown period
	if ea.isInCooldown(condition.ID) {
		return false
	}
	
	// Check if suppressed
	if ea.isSuppressed(condition.ID) {
		return false
	}
	
	// Check if rate limited
	if ea.isRateLimited(condition.ID) {
		return false
	}
	
	// Check specific condition logic
	switch condition.Type {
	case AlertTypeErrorRate:
		return ea.checkErrorRateCondition(condition, appErr)
	case AlertTypeErrorSpike:
		return ea.checkErrorSpikeCondition(condition, appErr)
	case AlertTypeNewError:
		return ea.checkNewErrorCondition(condition, appErr)
	case AlertTypeCriticalError:
		return appErr.Severity == SeverityCritical
	case AlertTypeServiceDegraded:
		return ea.checkServiceDegradationCondition(condition, appErr)
	default:
		return false
	}
}

// checkErrorRateCondition checks error rate condition
func (ea *ErrorAlerter) checkErrorRateCondition(condition *AlertCondition, appErr *AppError) bool {
	key := string(appErr.Type)
	rate, exists := ea.errorRates[key]
	
	return exists && rate >= condition.Threshold
}

// checkErrorSpikeCondition checks error spike condition
func (ea *ErrorAlerter) checkErrorSpikeCondition(condition *AlertCondition, appErr *AppError) bool {
	key := string(appErr.Type)
	count, exists := ea.errorCounts[key]
	
	// Check if we've seen this many errors in the window
	return exists && float64(count) >= condition.Threshold
}

// checkNewErrorCondition checks new error condition
func (ea *ErrorAlerter) checkNewErrorCondition(condition *AlertCondition, appErr *AppError) bool {
	// This would typically check against a database of known errors
	// For simplicity, we'll assume any error with a new code is new
	return true
}

// checkServiceDegradationCondition checks service degradation condition
func (ea *ErrorAlerter) checkServiceDegradationCondition(condition *AlertCondition, appErr *AppError) bool {
	// This would typically compare current error rates with historical baselines
	// For simplicity, we'll use a basic threshold
	key := string(appErr.Type)
	rate, exists := ea.errorRates[key]
	
	return exists && rate >= condition.Threshold
}

// triggerAlert triggers an alert
func (ea *ErrorAlerter) triggerAlert(condition *AlertCondition, appErr *AppError) {
	alertID := fmt.Sprintf("%s_%s_%d", condition.ID, appErr.Type, time.Now().Unix())
	
	// Check if we already have an active alert for this condition
	if existingAlert, exists := ea.activeAlerts[condition.ID]; exists {
		// Update existing alert
		existingAlert.Count++
		existingAlert.LastSeen = time.Now()
		existingAlert.Details["latest_error"] = appErr
		return
	}
	
	// Create new alert
	alert := &Alert{
		ID:          alertID,
		ConditionID: condition.ID,
		Type:        condition.Type,
		Level:       condition.Level,
		Title:       ea.generateAlertTitle(condition, appErr),
		Message:     ea.generateAlertMessage(condition, appErr),
		Details:     ea.generateAlertDetails(condition, appErr),
		Context:     ea.generateAlertContext(appErr),
		Timestamp:   time.Now(),
		Status:      "active",
		Count:       1,
		LastSeen:    time.Now(),
		Tags:        ea.generateAlertTags(condition, appErr),
		Severity:    appErr.Severity,
		Source:      appErr.Source,
		Runbook:     ea.generateRunbook(condition),
		Actions:     ea.generateActions(condition, appErr),
	}
	
	// Store alert
	ea.activeAlerts[condition.ID] = alert
	ea.alertHistory = append(ea.alertHistory, alert)
	
	// Send alert through channels
	ea.sendAlert(alert)
	
	// Set cooldown
	ea.setCooldown(condition.ID, condition.Cooldown)
	
	// Update rate limit
	ea.updateRateLimit(condition.ID)
	
	// Log alert
	ea.logger.Error("Alert triggered",
		"alert_id", alert.ID,
		"condition_id", condition.ID,
		"type", alert.Type,
		"level", alert.Level,
		"title", alert.Title,
		"error_type", appErr.Type,
		"error_severity", appErr.Severity,
	)
}

// generateAlertTitle generates alert title
func (ea *ErrorAlerter) generateAlertTitle(condition *AlertCondition, appErr *AppError) string {
	switch condition.Type {
	case AlertTypeErrorRate:
		return fmt.Sprintf("High Error Rate: %s", appErr.Type)
	case AlertTypeErrorSpike:
		return fmt.Sprintf("Error Spike Detected: %s", appErr.Type)
	case AlertTypeNewError:
		return fmt.Sprintf("New Error Pattern: %s", appErr.Type)
	case AlertTypeCriticalError:
		return fmt.Sprintf("Critical Error: %s", appErr.Type)
	case AlertTypeServiceDegraded:
		return fmt.Sprintf("Service Degradation: %s", appErr.Type)
	default:
		return fmt.Sprintf("Alert: %s", condition.Name)
	}
}

// generateAlertMessage generates alert message
func (ea *ErrorAlerter) generateAlertMessage(condition *AlertCondition, appErr *AppError) string {
	switch condition.Type {
	case AlertTypeErrorRate:
		rate := ea.errorRates[string(appErr.Type)]
		return fmt.Sprintf("Error rate for %s has exceeded threshold. Current rate: %.2f errors/minute (threshold: %.2f)", 
			appErr.Type, rate, condition.Threshold)
	case AlertTypeErrorSpike:
		count := ea.errorCounts[string(appErr.Type)]
		return fmt.Sprintf("Spike in %s errors detected. Count: %d in the last %v", 
			appErr.Type, count, condition.Window)
	case AlertTypeNewError:
		return fmt.Sprintf("New error pattern detected: %s - %s", appErr.Type, appErr.Message)
	case AlertTypeCriticalError:
		return fmt.Sprintf("Critical error occurred: %s", appErr.Message)
	case AlertTypeServiceDegraded:
		return fmt.Sprintf("Service degradation detected due to %s errors", appErr.Type)
	default:
		return condition.Description
	}
}

// generateAlertDetails generates alert details
func (ea *ErrorAlerter) generateAlertDetails(condition *AlertCondition, appErr *AppError) map[string]interface{} {
	details := map[string]interface{}{
		"condition_name": condition.Name,
		"condition_type": condition.Type,
		"threshold":      condition.Threshold,
		"window":         condition.Window.String(),
		"error_id":       appErr.ID,
		"error_type":     appErr.Type,
		"error_code":     appErr.Code,
		"error_message":  appErr.Message,
		"error_severity": appErr.Severity,
		"error_category": appErr.Category,
		"timestamp":      appErr.Timestamp,
	}
	
	if appErr.Details != nil {
		details["error_details"] = appErr.Details
	}
	
	if appErr.Context != nil {
		details["error_context"] = appErr.Context
	}
	
	// Add current metrics
	if rate, exists := ea.errorRates[string(appErr.Type)]; exists {
		details["current_error_rate"] = rate
	}
	
	if count, exists := ea.errorCounts[string(appErr.Type)]; exists {
		details["current_error_count"] = count
	}
	
	return details
}

// generateAlertContext generates alert context
func (ea *ErrorAlerter) generateAlertContext(appErr *AppError) map[string]interface{} {
	context := map[string]interface{}{
		"service":     "hotel-reviews",
		"environment": "production", // This could be configurable
		"timestamp":   time.Now(),
	}
	
	if appErr.CorrelationID != "" {
		context["correlation_id"] = appErr.CorrelationID
	}
	
	if appErr.RequestID != "" {
		context["request_id"] = appErr.RequestID
	}
	
	if appErr.UserID != "" {
		context["user_id"] = appErr.UserID
	}
	
	if appErr.Source != "" {
		context["source"] = appErr.Source
	}
	
	return context
}

// generateAlertTags generates alert tags
func (ea *ErrorAlerter) generateAlertTags(condition *AlertCondition, appErr *AppError) []string {
	tags := []string{
		fmt.Sprintf("type:%s", appErr.Type),
		fmt.Sprintf("severity:%s", appErr.Severity),
		fmt.Sprintf("category:%s", appErr.Category),
		fmt.Sprintf("alert_type:%s", condition.Type),
		fmt.Sprintf("alert_level:%s", condition.Level),
	}
	
	if appErr.Retryable {
		tags = append(tags, "retryable:true")
	} else {
		tags = append(tags, "retryable:false")
	}
	
	return tags
}

// generateRunbook generates runbook link
func (ea *ErrorAlerter) generateRunbook(condition *AlertCondition) string {
	// This would typically return a link to documentation
	return fmt.Sprintf("https://docs.company.com/runbooks/%s", condition.Type)
}

// generateActions generates suggested actions
func (ea *ErrorAlerter) generateActions(condition *AlertCondition, appErr *AppError) []string {
	actions := []string{
		"Check application logs",
		"Review system metrics",
		"Investigate error context",
	}
	
	switch condition.Type {
	case AlertTypeErrorRate:
		actions = append(actions, "Check system load", "Review recent deployments")
	case AlertTypeErrorSpike:
		actions = append(actions, "Check for recent changes", "Review traffic patterns")
	case AlertTypeNewError:
		actions = append(actions, "Review recent code changes", "Check for new dependencies")
	case AlertTypeCriticalError:
		actions = append(actions, "Escalate to on-call engineer", "Consider service rollback")
	case AlertTypeServiceDegraded:
		actions = append(actions, "Check dependent services", "Review circuit breaker status")
	}
	
	return actions
}

// sendAlert sends alert through all configured channels
func (ea *ErrorAlerter) sendAlert(alert *Alert) {
	for _, channel := range ea.channels {
		if !channel.IsEnabled() {
			continue
		}
		
		go func(ch AlertChannel) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			if err := ch.Send(ctx, alert); err != nil {
				ea.logger.Error("Failed to send alert",
					"alert_id", alert.ID,
					"channel", ch.Name(),
					"error", err,
				)
			}
		}(channel)
	}
}

// Helper methods for cooldown, suppression, and rate limiting

// isInCooldown checks if condition is in cooldown period
func (ea *ErrorAlerter) isInCooldown(conditionID string) bool {
	if cooldownUntil, exists := ea.suppressions[conditionID]; exists {
		return time.Now().Before(cooldownUntil)
	}
	return false
}

// setCooldown sets cooldown period for condition
func (ea *ErrorAlerter) setCooldown(conditionID string, duration time.Duration) {
	ea.suppressions[conditionID] = time.Now().Add(duration)
}

// isSuppressed checks if condition is suppressed
func (ea *ErrorAlerter) isSuppressed(conditionID string) bool {
	// This would typically check a suppression database
	return false
}

// isRateLimited checks if condition is rate limited
func (ea *ErrorAlerter) isRateLimited(conditionID string) bool {
	rateLimit, exists := ea.rateLimits[conditionID]
	if !exists {
		return false
	}
	
	now := time.Now()
	
	// Reset if window has passed
	if now.Sub(rateLimit.LastReset) >= rateLimit.Window {
		rateLimit.Count = 0
		rateLimit.LastReset = now
		rateLimit.Timestamps = []time.Time{}
	}
	
	// Check if we've exceeded the limit
	return rateLimit.Count >= 10 // Max 10 alerts per window
}

// updateRateLimit updates rate limit for condition
func (ea *ErrorAlerter) updateRateLimit(conditionID string) {
	rateLimit, exists := ea.rateLimits[conditionID]
	if !exists {
		rateLimit = &AlertRateLimit{
			Count:      0,
			Window:     time.Hour,
			LastReset:  time.Now(),
			Timestamps: []time.Time{},
		}
		ea.rateLimits[conditionID] = rateLimit
	}
	
	rateLimit.Count++
	rateLimit.Timestamps = append(rateLimit.Timestamps, time.Now())
}

// Public API methods

// AddCondition adds a new alert condition
func (ea *ErrorAlerter) AddCondition(condition *AlertCondition) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	ea.conditions[condition.ID] = condition
}

// RemoveCondition removes an alert condition
func (ea *ErrorAlerter) RemoveCondition(conditionID string) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	delete(ea.conditions, conditionID)
}

// GetConditions returns all alert conditions
func (ea *ErrorAlerter) GetConditions() map[string]*AlertCondition {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	
	conditions := make(map[string]*AlertCondition)
	for k, v := range ea.conditions {
		conditions[k] = v
	}
	
	return conditions
}

// GetActiveAlerts returns all active alerts
func (ea *ErrorAlerter) GetActiveAlerts() map[string]*Alert {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	
	alerts := make(map[string]*Alert)
	for k, v := range ea.activeAlerts {
		alerts[k] = v
	}
	
	return alerts
}

// GetAlertHistory returns alert history
func (ea *ErrorAlerter) GetAlertHistory() []*Alert {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	
	history := make([]*Alert, len(ea.alertHistory))
	copy(history, ea.alertHistory)
	
	return history
}

// AddChannel adds an alert channel
func (ea *ErrorAlerter) AddChannel(channel AlertChannel) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	ea.channels = append(ea.channels, channel)
}

// AcknowledgeAlert acknowledges an alert
func (ea *ErrorAlerter) AcknowledgeAlert(alertID, acknowledgedBy string) error {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	for _, alert := range ea.activeAlerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.AcknowledgedBy = acknowledgedBy
			alert.AcknowledgedAt = &now
			return nil
		}
	}
	
	return fmt.Errorf("alert not found: %s", alertID)
}

// ResolveAlert resolves an alert
func (ea *ErrorAlerter) ResolveAlert(alertID string) error {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	for conditionID, alert := range ea.activeAlerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.Status = "resolved"
			alert.ResolvedAt = &now
			delete(ea.activeAlerts, conditionID)
			return nil
		}
	}
	
	return fmt.Errorf("alert not found: %s", alertID)
}

// SuppressCondition suppresses an alert condition
func (ea *ErrorAlerter) SuppressCondition(conditionID string, duration time.Duration) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	ea.suppressions[conditionID] = time.Now().Add(duration)
}

// startBackgroundProcesses starts background alert processes
func (ea *ErrorAlerter) startBackgroundProcesses() {
	ea.wg.Add(1)
	go ea.alertMaintenanceLoop()
}

// alertMaintenanceLoop runs alert maintenance tasks
func (ea *ErrorAlerter) alertMaintenanceLoop() {
	defer ea.wg.Done()
	
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ea.performMaintenance()
		case <-ea.shutdownCh:
			return
		}
	}
}

// performMaintenance performs alert maintenance tasks
func (ea *ErrorAlerter) performMaintenance() {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	
	now := time.Now()
	
	// Clean up old suppressions
	for conditionID, suppressedUntil := range ea.suppressions {
		if now.After(suppressedUntil) {
			delete(ea.suppressions, conditionID)
		}
	}
	
	// Clean up old rate limits
	for conditionID, rateLimit := range ea.rateLimits {
		if now.Sub(rateLimit.LastReset) >= rateLimit.Window*2 {
			delete(ea.rateLimits, conditionID)
		}
	}
	
	// Clean up old alert history
	if len(ea.alertHistory) > 1000 {
		ea.alertHistory = ea.alertHistory[len(ea.alertHistory)-1000:]
	}
	
	// Auto-resolve old alerts
	for conditionID, alert := range ea.activeAlerts {
		if now.Sub(alert.Timestamp) >= 24*time.Hour {
			alert.Status = "auto-resolved"
			now := time.Now()
			alert.ResolvedAt = &now
			delete(ea.activeAlerts, conditionID)
		}
	}
}

// Close closes the alerter
func (ea *ErrorAlerter) Close() {
	ea.cancel()
	close(ea.shutdownCh)
	ea.wg.Wait()
}