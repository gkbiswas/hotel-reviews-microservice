package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// HealthStatus represents the health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthMetric represents a health metric
type HealthMetric struct {
	Name        string       `json:"name"`
	Value       float64      `json:"value"`
	Threshold   float64      `json:"threshold"`
	Status      HealthStatus `json:"status"`
	LastUpdated time.Time    `json:"last_updated"`
	Description string       `json:"description"`
}

// HealthCheck represents a health check
type HealthCheck struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Status     HealthStatus           `json:"status"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration"`
	Metrics    []*HealthMetric        `json:"metrics"`
	LastError  string                 `json:"last_error,omitempty"`
	ErrorCount int                    `json:"error_count"`
	Enabled    bool                   `json:"enabled"`
}

// HealthCondition represents a health condition
type HealthCondition struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	MetricName      string        `json:"metric_name"`
	Operator        string        `json:"operator"` // gt, lt, eq, ge, le
	Threshold       float64       `json:"threshold"`
	Window          time.Duration `json:"window"`
	ConsecutiveHits int           `json:"consecutive_hits"`
	Enabled         bool          `json:"enabled"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// HealthEvent represents a health event
type HealthEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // status_change, threshold_breach, recovery
	Status      HealthStatus           `json:"status"`
	PrevStatus  HealthStatus           `json:"prev_status"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details"`
	Timestamp   time.Time              `json:"timestamp"`
	ConditionID string                 `json:"condition_id,omitempty"`
	Severity    string                 `json:"severity"`
}

// HealthChecker monitors system health based on error patterns
type HealthChecker struct {
	config *ErrorHandlerConfig
	logger *logger.Logger
	mu     sync.RWMutex

	// Health tracking
	currentStatus    HealthStatus
	lastStatusChange time.Time
	checks           map[string]*HealthCheck
	conditions       map[string]*HealthCondition
	metrics          map[string]*HealthMetric
	events           []*HealthEvent

	// Error tracking for health
	errorRates  map[string]float64
	errorCounts map[string]int64
	errorTrends map[string][]float64
	lastUpdate  time.Time

	// Thresholds
	healthyThreshold   float64
	degradedThreshold  float64
	unhealthyThreshold float64

	// Background processes
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config *ErrorHandlerConfig, logger *logger.Logger) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())

	hc := &HealthChecker{
		config:             config,
		logger:             logger,
		currentStatus:      HealthStatusHealthy,
		lastStatusChange:   time.Now(),
		checks:             make(map[string]*HealthCheck),
		conditions:         make(map[string]*HealthCondition),
		metrics:            make(map[string]*HealthMetric),
		events:             make([]*HealthEvent, 0),
		errorRates:         make(map[string]float64),
		errorCounts:        make(map[string]int64),
		errorTrends:        make(map[string][]float64),
		lastUpdate:         time.Now(),
		healthyThreshold:   5.0,  // 5 errors per minute
		degradedThreshold:  20.0, // 20 errors per minute
		unhealthyThreshold: 50.0, // 50 errors per minute
		ctx:                ctx,
		cancel:             cancel,
		shutdownCh:         make(chan struct{}),
	}

	// Initialize default checks and conditions
	hc.initializeDefaultChecks()
	hc.initializeDefaultConditions()

	// Start background processes
	hc.startBackgroundProcesses()

	return hc
}

// initializeDefaultChecks initializes default health checks
func (hc *HealthChecker) initializeDefaultChecks() {
	defaultChecks := []*HealthCheck{
		{
			ID:        "error_rate",
			Name:      "Error Rate Check",
			Status:    HealthStatusHealthy,
			Message:   "System error rate is within acceptable limits",
			Details:   make(map[string]interface{}),
			Timestamp: time.Now(),
			Metrics:   make([]*HealthMetric, 0),
			Enabled:   true,
		},
		{
			ID:        "critical_errors",
			Name:      "Critical Errors Check",
			Status:    HealthStatusHealthy,
			Message:   "No critical errors detected",
			Details:   make(map[string]interface{}),
			Timestamp: time.Now(),
			Metrics:   make([]*HealthMetric, 0),
			Enabled:   true,
		},
		{
			ID:        "system_stability",
			Name:      "System Stability Check",
			Status:    HealthStatusHealthy,
			Message:   "System is stable",
			Details:   make(map[string]interface{}),
			Timestamp: time.Now(),
			Metrics:   make([]*HealthMetric, 0),
			Enabled:   true,
		},
		{
			ID:        "error_trends",
			Name:      "Error Trends Check",
			Status:    HealthStatusHealthy,
			Message:   "Error trends are stable",
			Details:   make(map[string]interface{}),
			Timestamp: time.Now(),
			Metrics:   make([]*HealthMetric, 0),
			Enabled:   true,
		},
	}

	for _, check := range defaultChecks {
		hc.checks[check.ID] = check
	}
}

// initializeDefaultConditions initializes default health conditions
func (hc *HealthChecker) initializeDefaultConditions() {
	defaultConditions := []*HealthCondition{
		{
			ID:              "high_error_rate",
			Name:            "High Error Rate",
			Description:     "Triggers when error rate exceeds threshold",
			MetricName:      "error_rate",
			Operator:        "gt",
			Threshold:       20.0,
			Window:          5 * time.Minute,
			ConsecutiveHits: 3,
			Enabled:         true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              "critical_error_spike",
			Name:            "Critical Error Spike",
			Description:     "Triggers when critical errors spike",
			MetricName:      "critical_error_rate",
			Operator:        "gt",
			Threshold:       5.0,
			Window:          2 * time.Minute,
			ConsecutiveHits: 2,
			Enabled:         true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              "error_trend_degradation",
			Name:            "Error Trend Degradation",
			Description:     "Triggers when error trends show degradation",
			MetricName:      "error_trend_slope",
			Operator:        "gt",
			Threshold:       0.5,
			Window:          10 * time.Minute,
			ConsecutiveHits: 5,
			Enabled:         true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              "system_instability",
			Name:            "System Instability",
			Description:     "Triggers when system shows instability",
			MetricName:      "stability_score",
			Operator:        "lt",
			Threshold:       0.7,
			Window:          15 * time.Minute,
			ConsecutiveHits: 3,
			Enabled:         true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	for _, condition := range defaultConditions {
		hc.conditions[condition.ID] = condition
	}
}

// RecordError records an error for health monitoring
func (hc *HealthChecker) RecordError(appErr *AppError) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Update error tracking
	hc.updateErrorTracking(appErr)

	// Update health metrics
	hc.updateHealthMetrics(appErr)

	// Check health conditions
	hc.checkHealthConditions()

	// Update overall health status
	hc.updateOverallHealth()
}

// updateErrorTracking updates error tracking for health monitoring
func (hc *HealthChecker) updateErrorTracking(appErr *AppError) {
	key := string(appErr.Type)
	hc.errorCounts[key]++

	// Update rates
	now := time.Now()
	if now.Sub(hc.lastUpdate) >= time.Minute {
		for errorType, count := range hc.errorCounts {
			rate := float64(count) / now.Sub(hc.lastUpdate).Minutes()
			hc.errorRates[errorType] = rate

			// Update trends
			if trends, exists := hc.errorTrends[errorType]; exists {
				trends = append(trends, rate)
				if len(trends) > 10 {
					trends = trends[1:]
				}
				hc.errorTrends[errorType] = trends
			} else {
				hc.errorTrends[errorType] = []float64{rate}
			}
		}
		hc.lastUpdate = now
	}
}

// updateHealthMetrics updates health metrics
func (hc *HealthChecker) updateHealthMetrics(appErr *AppError) {
	// Update error rate metric
	totalRate := 0.0
	for _, rate := range hc.errorRates {
		totalRate += rate
	}

	hc.updateMetric("error_rate", totalRate, hc.degradedThreshold, "Total error rate per minute")

	// Update critical error rate
	criticalRate := 0.0
	for errorType, rate := range hc.errorRates {
		if errorType == string(ErrorTypeSystem) || errorType == string(ErrorTypeDatabase) {
			criticalRate += rate
		}
	}

	hc.updateMetric("critical_error_rate", criticalRate, 5.0, "Critical error rate per minute")

	// Update stability score
	stabilityScore := hc.calculateStabilityScore()
	hc.updateMetric("stability_score", stabilityScore, 0.7, "System stability score (0-1)")

	// Update error trend slope
	trendSlope := hc.calculateTrendSlope()
	hc.updateMetric("error_trend_slope", trendSlope, 0.5, "Error trend slope (positive = increasing)")

	// Update service availability
	availability := hc.calculateServiceAvailability()
	hc.updateMetric("service_availability", availability, 0.95, "Service availability (0-1)")

	// Update response time impact
	responseTimeImpact := hc.calculateResponseTimeImpact()
	hc.updateMetric("response_time_impact", responseTimeImpact, 0.8, "Response time impact (0-1)")
}

// updateMetric updates a specific health metric
func (hc *HealthChecker) updateMetric(name string, value, threshold float64, description string) {
	status := HealthStatusHealthy
	if value > threshold {
		status = HealthStatusDegraded
		if value > threshold*2 {
			status = HealthStatusUnhealthy
		}
	}

	hc.metrics[name] = &HealthMetric{
		Name:        name,
		Value:       value,
		Threshold:   threshold,
		Status:      status,
		LastUpdated: time.Now(),
		Description: description,
	}
}

// calculateStabilityScore calculates system stability score
func (hc *HealthChecker) calculateStabilityScore() float64 {
	if len(hc.errorRates) == 0 {
		return 1.0
	}

	// Calculate stability based on error rate consistency
	totalRate := 0.0
	maxRate := 0.0

	for _, rate := range hc.errorRates {
		totalRate += rate
		if rate > maxRate {
			maxRate = rate
		}
	}

	if totalRate == 0 {
		return 1.0
	}

	// Stability decreases with higher error rates and rate variance
	avgRate := totalRate / float64(len(hc.errorRates))
	variance := (maxRate - avgRate) / avgRate

	stabilityScore := 1.0 - (avgRate / 100.0) - (variance / 10.0)

	if stabilityScore < 0 {
		stabilityScore = 0
	}
	if stabilityScore > 1 {
		stabilityScore = 1
	}

	return stabilityScore
}

// calculateTrendSlope calculates error trend slope
func (hc *HealthChecker) calculateTrendSlope() float64 {
	if len(hc.errorTrends) == 0 {
		return 0.0
	}

	totalSlope := 0.0
	trendCount := 0

	for _, trends := range hc.errorTrends {
		if len(trends) >= 2 {
			slope := trends[len(trends)-1] - trends[0]
			totalSlope += slope
			trendCount++
		}
	}

	if trendCount == 0 {
		return 0.0
	}

	return totalSlope / float64(trendCount)
}

// calculateServiceAvailability calculates service availability
func (hc *HealthChecker) calculateServiceAvailability() float64 {
	// This is a simplified calculation
	// In practice, this would consider actual service uptime

	totalErrors := 0.0
	for _, rate := range hc.errorRates {
		totalErrors += rate
	}

	// Assume 1000 requests per minute baseline
	baseline := 1000.0
	availability := (baseline - totalErrors) / baseline

	if availability < 0 {
		availability = 0
	}
	if availability > 1 {
		availability = 1
	}

	return availability
}

// calculateResponseTimeImpact calculates response time impact
func (hc *HealthChecker) calculateResponseTimeImpact() float64 {
	// This is a simplified calculation
	// In practice, this would consider actual response time metrics

	totalErrors := 0.0
	for _, rate := range hc.errorRates {
		totalErrors += rate
	}

	// Higher error rates typically correlate with slower response times
	impact := 1.0 - (totalErrors / 100.0)

	if impact < 0 {
		impact = 0
	}
	if impact > 1 {
		impact = 1
	}

	return impact
}

// checkHealthConditions checks all health conditions
func (hc *HealthChecker) checkHealthConditions() {
	for _, condition := range hc.conditions {
		if !condition.Enabled {
			continue
		}

		hc.checkCondition(condition)
	}
}

// checkCondition checks a specific health condition
func (hc *HealthChecker) checkCondition(condition *HealthCondition) {
	metric, exists := hc.metrics[condition.MetricName]
	if !exists {
		return
	}

	triggered := false

	switch condition.Operator {
	case "gt":
		triggered = metric.Value > condition.Threshold
	case "lt":
		triggered = metric.Value < condition.Threshold
	case "eq":
		triggered = metric.Value == condition.Threshold
	case "ge":
		triggered = metric.Value >= condition.Threshold
	case "le":
		triggered = metric.Value <= condition.Threshold
	}

	if triggered {
		hc.handleConditionTriggered(condition, metric)
	}
}

// handleConditionTriggered handles a triggered condition
func (hc *HealthChecker) handleConditionTriggered(condition *HealthCondition, metric *HealthMetric) {
	// Create health event
	event := &HealthEvent{
		ID:         fmt.Sprintf("condition_%s_%d", condition.ID, time.Now().Unix()),
		Type:       "threshold_breach",
		Status:     metric.Status,
		PrevStatus: hc.currentStatus,
		Message:    fmt.Sprintf("Condition '%s' triggered: %s = %.2f (threshold: %.2f)", condition.Name, metric.Name, metric.Value, condition.Threshold),
		Details: map[string]interface{}{
			"condition_id": condition.ID,
			"metric_name":  metric.Name,
			"metric_value": metric.Value,
			"threshold":    condition.Threshold,
			"operator":     condition.Operator,
		},
		Timestamp:   time.Now(),
		ConditionID: condition.ID,
		Severity:    hc.getSeverityFromStatus(metric.Status),
	}

	hc.events = append(hc.events, event)

	// Update related health check
	hc.updateHealthCheck(condition, metric)

	// Log the event
	hc.logger.Warn("Health condition triggered",
		"condition_id", condition.ID,
		"condition_name", condition.Name,
		"metric_name", metric.Name,
		"metric_value", metric.Value,
		"threshold", condition.Threshold,
		"status", metric.Status,
	)
}

// updateHealthCheck updates a health check based on condition
func (hc *HealthChecker) updateHealthCheck(condition *HealthCondition, metric *HealthMetric) {
	// Find related health check
	var checkID string
	switch condition.MetricName {
	case "error_rate":
		checkID = "error_rate"
	case "critical_error_rate":
		checkID = "critical_errors"
	case "stability_score":
		checkID = "system_stability"
	case "error_trend_slope":
		checkID = "error_trends"
	default:
		checkID = "error_rate"
	}

	if check, exists := hc.checks[checkID]; exists {
		check.Status = metric.Status
		check.Message = fmt.Sprintf("Condition '%s' triggered: %s = %.2f", condition.Name, metric.Name, metric.Value)
		check.Timestamp = time.Now()
		check.Details["last_condition"] = condition.ID
		check.Details["last_metric_value"] = metric.Value
		check.Details["last_threshold"] = condition.Threshold

		// Update metrics
		check.Metrics = []*HealthMetric{metric}

		if metric.Status != HealthStatusHealthy {
			check.ErrorCount++
			check.LastError = check.Message
		}
	}
}

// updateOverallHealth updates overall health status
func (hc *HealthChecker) updateOverallHealth() {
	// Determine overall status based on individual checks
	overallStatus := HealthStatusHealthy

	unhealthyCount := 0
	degradedCount := 0

	for _, check := range hc.checks {
		if !check.Enabled {
			continue
		}

		switch check.Status {
		case HealthStatusUnhealthy:
			unhealthyCount++
		case HealthStatusDegraded:
			degradedCount++
		}
	}

	// Determine overall status
	if unhealthyCount > 0 {
		overallStatus = HealthStatusUnhealthy
	} else if degradedCount > 0 {
		overallStatus = HealthStatusDegraded
	}

	// Update status if changed
	if overallStatus != hc.currentStatus {
		prevStatus := hc.currentStatus
		hc.currentStatus = overallStatus
		hc.lastStatusChange = time.Now()

		// Create status change event
		event := &HealthEvent{
			ID:         fmt.Sprintf("status_change_%d", time.Now().Unix()),
			Type:       "status_change",
			Status:     overallStatus,
			PrevStatus: prevStatus,
			Message:    fmt.Sprintf("Overall health status changed from %s to %s", prevStatus, overallStatus),
			Details: map[string]interface{}{
				"unhealthy_checks": unhealthyCount,
				"degraded_checks":  degradedCount,
				"total_checks":     len(hc.checks),
			},
			Timestamp: time.Now(),
			Severity:  hc.getSeverityFromStatus(overallStatus),
		}

		hc.events = append(hc.events, event)

		// Log status change
		hc.logger.Info("Health status changed",
			"prev_status", prevStatus,
			"new_status", overallStatus,
			"unhealthy_checks", unhealthyCount,
			"degraded_checks", degradedCount,
		)
	}
}

// getSeverityFromStatus converts health status to severity
func (hc *HealthChecker) getSeverityFromStatus(status HealthStatus) string {
	switch status {
	case HealthStatusHealthy:
		return "info"
	case HealthStatusDegraded:
		return "warning"
	case HealthStatusUnhealthy:
		return "error"
	default:
		return "unknown"
	}
}

// Public API methods

// IsHealthy returns true if the system is healthy
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.currentStatus == HealthStatusHealthy
}

// GetHealthStatus returns current health status
func (hc *HealthChecker) GetHealthStatus() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return map[string]interface{}{
		"status":             hc.currentStatus,
		"last_status_change": hc.lastStatusChange,
		"checks":             hc.checks,
		"metrics":            hc.metrics,
		"conditions":         hc.conditions,
		"recent_events":      hc.getRecentEvents(10),
		"error_rates":        hc.errorRates,
		"error_trends":       hc.errorTrends,
	}
}

// GetHealthChecks returns all health checks
func (hc *HealthChecker) GetHealthChecks() map[string]*HealthCheck {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	checks := make(map[string]*HealthCheck)
	for k, v := range hc.checks {
		checks[k] = v
	}

	return checks
}

// GetHealthMetrics returns all health metrics
func (hc *HealthChecker) GetHealthMetrics() map[string]*HealthMetric {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	metrics := make(map[string]*HealthMetric)
	for k, v := range hc.metrics {
		metrics[k] = v
	}

	return metrics
}

// GetHealthEvents returns recent health events
func (hc *HealthChecker) GetHealthEvents() []*HealthEvent {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	events := make([]*HealthEvent, len(hc.events))
	copy(events, hc.events)

	return events
}

// getRecentEvents returns recent health events
func (hc *HealthChecker) getRecentEvents(limit int) []*HealthEvent {
	if len(hc.events) <= limit {
		return hc.events
	}

	return hc.events[len(hc.events)-limit:]
}

// AddHealthCheck adds a custom health check
func (hc *HealthChecker) AddHealthCheck(check *HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.checks[check.ID] = check
}

// AddHealthCondition adds a custom health condition
func (hc *HealthChecker) AddHealthCondition(condition *HealthCondition) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.conditions[condition.ID] = condition
}

// RunHealthCheck runs all health checks
func (hc *HealthChecker) RunHealthCheck() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Update all health checks
	for _, check := range hc.checks {
		if !check.Enabled {
			continue
		}

		start := time.Now()

		// Run the check (this is a simplified version)
		// In practice, each check would have its own logic
		check.Timestamp = time.Now()
		check.Duration = time.Since(start)

		// Update check status based on metrics
		hc.updateCheckStatus(check)
	}

	// Update overall health
	hc.updateOverallHealth()
}

// updateCheckStatus updates check status based on current metrics
func (hc *HealthChecker) updateCheckStatus(check *HealthCheck) {
	// This is a simplified status update
	// In practice, each check would have specific logic

	switch check.ID {
	case "error_rate":
		if metric, exists := hc.metrics["error_rate"]; exists {
			check.Status = metric.Status
			check.Message = fmt.Sprintf("Error rate: %.2f/min", metric.Value)
			check.Metrics = []*HealthMetric{metric}
		}
	case "critical_errors":
		if metric, exists := hc.metrics["critical_error_rate"]; exists {
			check.Status = metric.Status
			check.Message = fmt.Sprintf("Critical error rate: %.2f/min", metric.Value)
			check.Metrics = []*HealthMetric{metric}
		}
	case "system_stability":
		if metric, exists := hc.metrics["stability_score"]; exists {
			check.Status = metric.Status
			check.Message = fmt.Sprintf("Stability score: %.2f", metric.Value)
			check.Metrics = []*HealthMetric{metric}
		}
	case "error_trends":
		if metric, exists := hc.metrics["error_trend_slope"]; exists {
			check.Status = metric.Status
			check.Message = fmt.Sprintf("Error trend slope: %.2f", metric.Value)
			check.Metrics = []*HealthMetric{metric}
		}
	}
}

// startBackgroundProcesses starts background health monitoring processes
func (hc *HealthChecker) startBackgroundProcesses() {
	hc.wg.Add(1)
	go hc.healthMonitoringLoop()
}

// healthMonitoringLoop runs health monitoring loop
func (hc *HealthChecker) healthMonitoringLoop() {
	defer hc.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.RunHealthCheck()
		case <-hc.shutdownCh:
			return
		}
	}
}

// Close closes the health checker
func (hc *HealthChecker) Close() {
	hc.cancel()
	close(hc.shutdownCh)
	hc.wg.Wait()
}
