package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ErrorMetricsCollector collects and reports error metrics
type ErrorMetricsCollector struct {
	config *ErrorHandlerConfig
	logger *logger.Logger
	mu     sync.RWMutex

	// Prometheus metrics
	errorCounter        *prometheus.CounterVec
	errorDuration       *prometheus.HistogramVec
	errorSeverityGauge  *prometheus.GaugeVec
	activeErrors        *prometheus.GaugeVec
	errorRateGauge      *prometheus.GaugeVec
	errorPatternCounter *prometheus.CounterVec

	// Internal metrics
	errorCounts       map[string]int64
	errorRates        map[string]float64
	errorSeverities   map[string]map[ErrorSeverity]int64
	errorPatterns     map[string]int64
	lastMetricsUpdate time.Time

	// Background processes
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}
}

// NewErrorMetricsCollector creates a new error metrics collector
func NewErrorMetricsCollector(config *ErrorHandlerConfig, logger *logger.Logger) *ErrorMetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())

	emc := &ErrorMetricsCollector{
		config:            config,
		logger:            logger,
		errorCounts:       make(map[string]int64),
		errorRates:        make(map[string]float64),
		errorSeverities:   make(map[string]map[ErrorSeverity]int64),
		errorPatterns:     make(map[string]int64),
		lastMetricsUpdate: time.Now(),
		ctx:               ctx,
		cancel:            cancel,
		shutdownCh:        make(chan struct{}),
	}

	// Initialize Prometheus metrics
	emc.initializeMetrics()

	// Start background processes
	emc.startBackgroundProcesses()

	return emc
}

// initializeMetrics initializes Prometheus metrics
func (emc *ErrorMetricsCollector) initializeMetrics() {
	emc.errorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "error_handler_errors_total",
			Help: "Total number of errors handled by type and severity",
		},
		[]string{"type", "severity", "category", "retryable"},
	)

	emc.errorDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "error_handler_processing_duration_seconds",
			Help:    "Time spent processing errors",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type", "severity"},
	)

	emc.errorSeverityGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "error_handler_severity_count",
			Help: "Current count of errors by severity level",
		},
		[]string{"severity"},
	)

	emc.activeErrors = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "error_handler_active_errors",
			Help: "Number of active errors by type",
		},
		[]string{"type"},
	)

	emc.errorRateGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "error_handler_error_rate",
			Help: "Error rate by type (errors per minute)",
		},
		[]string{"type"},
	)

	emc.errorPatternCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "error_handler_patterns_total",
			Help: "Total number of error patterns detected",
		},
		[]string{"pattern", "type"},
	)
}

// RecordError records an error occurrence
func (emc *ErrorMetricsCollector) RecordError(appErr *AppError) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		emc.errorDuration.WithLabelValues(
			string(appErr.Type),
			string(appErr.Severity),
		).Observe(duration.Seconds())
	}()

	// Update Prometheus metrics
	emc.errorCounter.WithLabelValues(
		string(appErr.Type),
		string(appErr.Severity),
		string(appErr.Category),
		fmt.Sprintf("%t", appErr.Retryable),
	).Inc()

	// Update internal metrics
	emc.mu.Lock()
	defer emc.mu.Unlock()

	// Update error counts
	typeKey := string(appErr.Type)
	emc.errorCounts[typeKey]++

	// Update severity counts
	if _, exists := emc.errorSeverities[typeKey]; !exists {
		emc.errorSeverities[typeKey] = make(map[ErrorSeverity]int64)
	}
	emc.errorSeverities[typeKey][appErr.Severity]++

	// Update error patterns
	if appErr.Code != "" {
		patternKey := fmt.Sprintf("%s:%s", appErr.Type, appErr.Code)
		emc.errorPatterns[patternKey]++

		emc.errorPatternCounter.WithLabelValues(
			appErr.Code,
			string(appErr.Type),
		).Inc()
	}

	// Update active errors gauge
	emc.activeErrors.WithLabelValues(string(appErr.Type)).Inc()

	// Update severity gauge
	emc.errorSeverityGauge.WithLabelValues(string(appErr.Severity)).Inc()
}

// UpdateErrorStats updates error statistics
func (emc *ErrorMetricsCollector) UpdateErrorStats(errorType, errorCode string, stats *ErrorMetrics) {
	emc.mu.Lock()
	defer emc.mu.Unlock()

	// Calculate error rate (errors per minute)
	duration := time.Since(stats.FirstSeen)
	if duration > 0 {
		rate := float64(stats.Count) / duration.Minutes()
		emc.errorRates[errorType] = rate

		// Update Prometheus gauge
		emc.errorRateGauge.WithLabelValues(errorType).Set(rate)
	}
}

// GetMetrics returns current metrics
func (emc *ErrorMetricsCollector) GetMetrics() map[string]interface{} {
	emc.mu.RLock()
	defer emc.mu.RUnlock()

	return map[string]interface{}{
		"error_counts":     emc.errorCounts,
		"error_rates":      emc.errorRates,
		"error_severities": emc.errorSeverities,
		"error_patterns":   emc.errorPatterns,
		"last_update":      emc.lastMetricsUpdate,
	}
}

// GetErrorRate returns the error rate for a specific type
func (emc *ErrorMetricsCollector) GetErrorRate(errorType string) float64 {
	emc.mu.RLock()
	defer emc.mu.RUnlock()

	if rate, exists := emc.errorRates[errorType]; exists {
		return rate
	}
	return 0
}

// GetErrorCount returns the error count for a specific type
func (emc *ErrorMetricsCollector) GetErrorCount(errorType string) int64 {
	emc.mu.RLock()
	defer emc.mu.RUnlock()

	if count, exists := emc.errorCounts[errorType]; exists {
		return count
	}
	return 0
}

// ResetMetrics resets all metrics
func (emc *ErrorMetricsCollector) ResetMetrics() {
	emc.mu.Lock()
	defer emc.mu.Unlock()

	emc.errorCounts = make(map[string]int64)
	emc.errorRates = make(map[string]float64)
	emc.errorSeverities = make(map[string]map[ErrorSeverity]int64)
	emc.errorPatterns = make(map[string]int64)
	emc.lastMetricsUpdate = time.Now()
}

// startBackgroundProcesses starts background metric collection processes
func (emc *ErrorMetricsCollector) startBackgroundProcesses() {
	emc.wg.Add(1)
	go emc.metricsUpdateLoop()
}

// metricsUpdateLoop runs the metrics update loop
func (emc *ErrorMetricsCollector) metricsUpdateLoop() {
	defer emc.wg.Done()

	ticker := time.NewTicker(emc.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			emc.updateMetrics()
		case <-emc.shutdownCh:
			return
		}
	}
}

// updateMetrics updates the metrics periodically
func (emc *ErrorMetricsCollector) updateMetrics() {
	emc.mu.Lock()
	defer emc.mu.Unlock()

	emc.lastMetricsUpdate = time.Now()

	// Log metrics if enabled
	if emc.config.EnableDetailedLogging {
		emc.logger.Info("Error metrics update",
			"error_counts", emc.errorCounts,
			"error_rates", emc.errorRates,
			"total_patterns", len(emc.errorPatterns),
		)
	}
}

// Close closes the metrics collector
func (emc *ErrorMetricsCollector) Close() {
	emc.cancel()
	close(emc.shutdownCh)
	emc.wg.Wait()
}
