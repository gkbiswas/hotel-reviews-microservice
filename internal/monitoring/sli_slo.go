package monitoring

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// SLIType represents the type of SLI
type SLIType string

const (
	SLITypeAvailability SLIType = "availability"
	SLITypeLatency      SLIType = "latency"
	SLITypeErrorRate    SLIType = "error_rate"
	SLITypeThroughput   SLIType = "throughput"
)

// SLOSeverity represents the severity of SLO violations
type SLOSeverity string

const (
	SLOSeverityInfo     SLOSeverity = "info"
	SLOSeverityWarning  SLOSeverity = "warning"
	SLOSeverityCritical SLOSeverity = "critical"
)

// SLI represents a Service Level Indicator
type SLI struct {
	Name        string  `json:"name"`
	Type        SLIType `json:"type"`
	Description string  `json:"description"`
	Query       string  `json:"query"`
	Unit        string  `json:"unit"`
	Service     string  `json:"service"`
	Endpoint    string  `json:"endpoint,omitempty"`
}

// SLO represents a Service Level Objective
type SLO struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Service      string      `json:"service"`
	SLI          SLI         `json:"sli"`
	Target       float64     `json:"target"`
	Window       string      `json:"window"`
	Severity     SLOSeverity `json:"severity"`
	AlertEnabled bool        `json:"alert_enabled"`
}

// SLIResult represents the result of an SLI measurement
type SLIResult struct {
	SLI       SLI       `json:"sli"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// SLOViolation represents an SLO violation
type SLOViolation struct {
	SLO         SLO       `json:"slo"`
	CurrentValue float64   `json:"current_value"`
	Target      float64   `json:"target"`
	Timestamp   time.Time `json:"timestamp"`
	Severity    SLOSeverity `json:"severity"`
}

// SLOManager manages SLIs and SLOs
type SLOManager struct {
	slis    []SLI
	slos    []SLO
	logger  *logrus.Logger
	metrics *MetricsRegistry
}

// NewSLOManager creates a new SLO manager
func NewSLOManager(metrics *MetricsRegistry, logger *logrus.Logger) *SLOManager {
	manager := &SLOManager{
		slis:    []SLI{},
		slos:    []SLO{},
		logger:  logger,
		metrics: metrics,
	}
	
	// Initialize default SLIs and SLOs
	manager.initializeDefaultSLIs()
	manager.initializeDefaultSLOs()
	
	return manager
}

// initializeDefaultSLIs initializes the default SLIs for the hotel reviews service
func (m *SLOManager) initializeDefaultSLIs() {
	m.slis = []SLI{
		// API Availability SLIs
		{
			Name:        "api_availability",
			Type:        SLITypeAvailability,
			Description: "API availability measured by successful requests",
			Query:       "rate(hotel_reviews_http_requests_total{status_code!~\"5..\"}[5m]) / rate(hotel_reviews_http_requests_total[5m])",
			Unit:        "percent",
			Service:     "hotel-reviews-api",
		},
		{
			Name:        "health_check_availability",
			Type:        SLITypeAvailability,
			Description: "Health check endpoint availability",
			Query:       "rate(hotel_reviews_http_requests_total{endpoint=\"/health\",status_code=\"200\"}[5m]) / rate(hotel_reviews_http_requests_total{endpoint=\"/health\"}[5m])",
			Unit:        "percent",
			Service:     "hotel-reviews-api",
			Endpoint:    "/health",
		},
		
		// API Latency SLIs
		{
			Name:        "api_latency_p95",
			Type:        SLITypeLatency,
			Description: "95th percentile API response time",
			Query:       "histogram_quantile(0.95, rate(hotel_reviews_http_request_duration_seconds_bucket[5m]))",
			Unit:        "seconds",
			Service:     "hotel-reviews-api",
		},
		{
			Name:        "api_latency_p99",
			Type:        SLITypeLatency,
			Description: "99th percentile API response time",
			Query:       "histogram_quantile(0.99, rate(hotel_reviews_http_request_duration_seconds_bucket[5m]))",
			Unit:        "seconds",
			Service:     "hotel-reviews-api",
		},
		{
			Name:        "review_processing_latency",
			Type:        SLITypeLatency,
			Description: "Review processing latency",
			Query:       "histogram_quantile(0.95, rate(hotel_reviews_processing_duration_seconds_bucket{operation=\"process\"}[5m]))",
			Unit:        "seconds",
			Service:     "review-processor",
		},
		
		// Error Rate SLIs
		{
			Name:        "api_error_rate",
			Type:        SLITypeErrorRate,
			Description: "API error rate (4xx and 5xx responses)",
			Query:       "rate(hotel_reviews_http_requests_total{status_code=~\"4..|5..\"}[5m]) / rate(hotel_reviews_http_requests_total[5m])",
			Unit:        "percent",
			Service:     "hotel-reviews-api",
		},
		{
			Name:        "processing_error_rate",
			Type:        SLITypeErrorRate,
			Description: "Review processing error rate",
			Query:       "rate(hotel_reviews_processing_errors_total[5m]) / rate(hotel_reviews_processed_total[5m])",
			Unit:        "percent",
			Service:     "review-processor",
		},
		{
			Name:        "database_error_rate",
			Type:        SLITypeErrorRate,
			Description: "Database operation error rate",
			Query:       "rate(hotel_reviews_database_errors_total[5m]) / rate(hotel_reviews_database_queries_total[5m])",
			Unit:        "percent",
			Service:     "database",
		},
		
		// Throughput SLIs
		{
			Name:        "api_throughput",
			Type:        SLITypeThroughput,
			Description: "API requests per second",
			Query:       "rate(hotel_reviews_http_requests_total[5m])",
			Unit:        "requests/second",
			Service:     "hotel-reviews-api",
		},
		{
			Name:        "review_processing_throughput",
			Type:        SLITypeThroughput,
			Description: "Reviews processed per second",
			Query:       "rate(hotel_reviews_processed_total[5m])",
			Unit:        "reviews/second",
			Service:     "review-processor",
		},
		{
			Name:        "file_processing_throughput",
			Type:        SLITypeThroughput,
			Description: "Files processed per hour",
			Query:       "rate(hotel_reviews_files_processed_total{status=\"success\"}[1h]) * 3600",
			Unit:        "files/hour",
			Service:     "file-processor",
		},
	}
}

// initializeDefaultSLOs initializes the default SLOs for the hotel reviews service
func (m *SLOManager) initializeDefaultSLOs() {
	m.slos = []SLO{
		// Availability SLOs
		{
			Name:         "api_availability_slo",
			Description:  "API should be available 99.5% of the time over 30 days",
			Service:      "hotel-reviews-api",
			SLI:          m.getSLIByName("api_availability"),
			Target:       0.995,
			Window:       "30d",
			Severity:     SLOSeverityCritical,
			AlertEnabled: true,
		},
		{
			Name:         "health_check_availability_slo",
			Description:  "Health check should be available 99.9% of the time over 7 days",
			Service:      "hotel-reviews-api",
			SLI:          m.getSLIByName("health_check_availability"),
			Target:       0.999,
			Window:       "7d",
			Severity:     SLOSeverityWarning,
			AlertEnabled: true,
		},
		
		// Latency SLOs
		{
			Name:         "api_latency_p95_slo",
			Description:  "95% of API requests should complete within 500ms",
			Service:      "hotel-reviews-api",
			SLI:          m.getSLIByName("api_latency_p95"),
			Target:       0.5,
			Window:       "1h",
			Severity:     SLOSeverityWarning,
			AlertEnabled: true,
		},
		{
			Name:         "api_latency_p99_slo",
			Description:  "99% of API requests should complete within 2 seconds",
			Service:      "hotel-reviews-api",
			SLI:          m.getSLIByName("api_latency_p99"),
			Target:       2.0,
			Window:       "1h",
			Severity:     SLOSeverityCritical,
			AlertEnabled: true,
		},
		{
			Name:         "review_processing_latency_slo",
			Description:  "95% of reviews should be processed within 10 seconds",
			Service:      "review-processor",
			SLI:          m.getSLIByName("review_processing_latency"),
			Target:       10.0,
			Window:       "1h",
			Severity:     SLOSeverityWarning,
			AlertEnabled: true,
		},
		
		// Error Rate SLOs
		{
			Name:         "api_error_rate_slo",
			Description:  "API error rate should be less than 1% over 1 hour",
			Service:      "hotel-reviews-api",
			SLI:          m.getSLIByName("api_error_rate"),
			Target:       0.01,
			Window:       "1h",
			Severity:     SLOSeverityWarning,
			AlertEnabled: true,
		},
		{
			Name:         "processing_error_rate_slo",
			Description:  "Review processing error rate should be less than 0.5% over 1 hour",
			Service:      "review-processor",
			SLI:          m.getSLIByName("processing_error_rate"),
			Target:       0.005,
			Window:       "1h",
			Severity:     SLOSeverityWarning,
			AlertEnabled: true,
		},
		{
			Name:         "database_error_rate_slo",
			Description:  "Database error rate should be less than 0.1% over 1 hour",
			Service:      "database",
			SLI:          m.getSLIByName("database_error_rate"),
			Target:       0.001,
			Window:       "1h",
			Severity:     SLOSeverityCritical,
			AlertEnabled: true,
		},
		
		// Throughput SLOs
		{
			Name:         "api_throughput_slo",
			Description:  "API should handle at least 100 requests per second",
			Service:      "hotel-reviews-api",
			SLI:          m.getSLIByName("api_throughput"),
			Target:       100.0,
			Window:       "5m",
			Severity:     SLOSeverityInfo,
			AlertEnabled: false,
		},
		{
			Name:         "review_processing_throughput_slo",
			Description:  "System should process at least 1000 reviews per second",
			Service:      "review-processor",
			SLI:          m.getSLIByName("review_processing_throughput"),
			Target:       1000.0,
			Window:       "5m",
			Severity:     SLOSeverityInfo,
			AlertEnabled: false,
		},
		{
			Name:         "file_processing_throughput_slo",
			Description:  "System should process at least 10 files per hour",
			Service:      "file-processor",
			SLI:          m.getSLIByName("file_processing_throughput"),
			Target:       10.0,
			Window:       "1h",
			Severity:     SLOSeverityWarning,
			AlertEnabled: true,
		},
	}
}

// getSLIByName returns an SLI by its name
func (m *SLOManager) getSLIByName(name string) SLI {
	for _, sli := range m.slis {
		if sli.Name == name {
			return sli
		}
	}
	return SLI{}
}

// GetSLIs returns all SLIs
func (m *SLOManager) GetSLIs() []SLI {
	return m.slis
}

// GetSLOs returns all SLOs
func (m *SLOManager) GetSLOs() []SLO {
	return m.slos
}

// GetSLOsByService returns SLOs for a specific service
func (m *SLOManager) GetSLOsByService(service string) []SLO {
	var slos []SLO
	for _, slo := range m.slos {
		if slo.Service == service {
			slos = append(slos, slo)
		}
	}
	return slos
}

// GetSLOsBySeverity returns SLOs with a specific severity
func (m *SLOManager) GetSLOsBySeverity(severity SLOSeverity) []SLO {
	var slos []SLO
	for _, slo := range m.slos {
		if slo.Severity == severity {
			slos = append(slos, slo)
		}
	}
	return slos
}

// AddSLI adds a new SLI
func (m *SLOManager) AddSLI(sli SLI) {
	m.slis = append(m.slis, sli)
}

// AddSLO adds a new SLO
func (m *SLOManager) AddSLO(slo SLO) {
	m.slos = append(m.slos, slo)
}

// EvaluateSLI evaluates an SLI and returns its current value
func (m *SLOManager) EvaluateSLI(ctx context.Context, sli SLI) (SLIResult, error) {
	// This is a placeholder implementation
	// In a real implementation, you would query Prometheus or another metrics system
	
	// For demonstration, return mock values
	var value float64
	switch sli.Type {
	case SLITypeAvailability:
		value = 0.998 // 99.8% availability
	case SLITypeLatency:
		value = 0.3 // 300ms
	case SLITypeErrorRate:
		value = 0.002 // 0.2% error rate
	case SLITypeThroughput:
		value = 150.0 // 150 requests/second
	}
	
	return SLIResult{
		SLI:       sli,
		Value:     value,
		Timestamp: time.Now(),
	}, nil
}

// EvaluateSLO evaluates an SLO and returns whether it's being met
func (m *SLOManager) EvaluateSLO(ctx context.Context, slo SLO) (bool, SLIResult, error) {
	result, err := m.EvaluateSLI(ctx, slo.SLI)
	if err != nil {
		return false, result, err
	}
	
	var met bool
	switch slo.SLI.Type {
	case SLITypeAvailability, SLITypeThroughput:
		met = result.Value >= slo.Target
	case SLITypeLatency, SLITypeErrorRate:
		met = result.Value <= slo.Target
	}
	
	return met, result, nil
}

// CheckSLOViolations checks for SLO violations and returns them
func (m *SLOManager) CheckSLOViolations(ctx context.Context) ([]SLOViolation, error) {
	var violations []SLOViolation
	
	for _, slo := range m.slos {
		if !slo.AlertEnabled {
			continue
		}
		
		met, result, err := m.EvaluateSLO(ctx, slo)
		if err != nil {
			m.logger.WithError(err).WithField("slo", slo.Name).Error("Failed to evaluate SLO")
			continue
		}
		
		if !met {
			violations = append(violations, SLOViolation{
				SLO:          slo,
				CurrentValue: result.Value,
				Target:       slo.Target,
				Timestamp:    time.Now(),
				Severity:     slo.Severity,
			})
		}
	}
	
	return violations, nil
}

// StartSLOMonitoring starts continuous SLO monitoring
func (m *SLOManager) StartSLOMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				violations, err := m.CheckSLOViolations(ctx)
				if err != nil {
					m.logger.WithError(err).Error("Failed to check SLO violations")
					continue
				}
				
				for _, violation := range violations {
					m.logger.WithFields(logrus.Fields{
						"slo":           violation.SLO.Name,
						"service":       violation.SLO.Service,
						"current_value": violation.CurrentValue,
						"target":        violation.Target,
						"severity":      violation.Severity,
					}).Warn("SLO violation detected")
					
					// Update SLO metrics
					m.updateSLOMetrics(violation)
				}
			}
		}
	}()
}

// updateSLOMetrics updates SLO-related metrics
func (m *SLOManager) updateSLOMetrics(violation SLOViolation) {
	slo := violation.SLO
	
	// Update SLI metrics
	switch slo.SLI.Type {
	case SLITypeAvailability:
		m.metrics.SLIAvailability.WithLabelValues(slo.Service, slo.SLI.Endpoint).Set(violation.CurrentValue)
	case SLITypeLatency:
		m.metrics.SLILatency.WithLabelValues(slo.Service, slo.SLI.Endpoint).Observe(violation.CurrentValue)
	case SLITypeErrorRate:
		m.metrics.SLIErrorRate.WithLabelValues(slo.Service, slo.SLI.Endpoint).Set(violation.CurrentValue)
	case SLITypeThroughput:
		m.metrics.SLIThroughput.WithLabelValues(slo.Service, slo.SLI.Endpoint).Set(violation.CurrentValue)
	}
}

// GetSLOReport generates a report of all SLO statuses
func (m *SLOManager) GetSLOReport(ctx context.Context) (map[string]interface{}, error) {
	report := make(map[string]interface{})
	
	var totalSLOs, violatedSLOs int
	var sloResults []map[string]interface{}
	
	for _, slo := range m.slos {
		met, result, err := m.EvaluateSLO(ctx, slo)
		if err != nil {
			continue
		}
		
		totalSLOs++
		if !met {
			violatedSLOs++
		}
		
		sloResult := map[string]interface{}{
			"name":          slo.Name,
			"service":       slo.Service,
			"type":          slo.SLI.Type,
			"target":        slo.Target,
			"current_value": result.Value,
			"met":           met,
			"severity":      slo.Severity,
			"window":        slo.Window,
		}
		
		sloResults = append(sloResults, sloResult)
	}
	
	report["total_slos"] = totalSLOs
	report["violated_slos"] = violatedSLOs
	report["slo_success_rate"] = float64(totalSLOs-violatedSLOs) / float64(totalSLOs)
	report["slos"] = sloResults
	report["timestamp"] = time.Now()
	
	return report, nil
}