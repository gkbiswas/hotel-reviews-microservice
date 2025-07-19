package infrastructure

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// ErrorAggregator aggregates and analyzes error patterns
type ErrorAggregator struct {
	config *ErrorHandlerConfig
	logger *logger.Logger
	mu     sync.RWMutex

	// Error pattern tracking
	errorPatterns   map[string]*ErrorPattern
	errorSignatures map[string]*ErrorSignature
	errorClusters   map[string]*ErrorCluster
	errorTrends     map[string]*ErrorTrend

	// Time-based aggregation
	hourlyStats map[string]*HourlyErrorStats
	dailyStats  map[string]*DailyErrorStats

	// Background processes
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}
}

// ErrorSignature represents a unique error signature
type ErrorSignature struct {
	ID             string                 `json:"id"`
	Type           ErrorType              `json:"type"`
	MessagePattern string                 `json:"message_pattern"`
	StackPattern   string                 `json:"stack_pattern"`
	SourcePattern  string                 `json:"source_pattern"`
	Count          int64                  `json:"count"`
	FirstSeen      time.Time              `json:"first_seen"`
	LastSeen       time.Time              `json:"last_seen"`
	AffectedUsers  []string               `json:"affected_users"`
	Sources        []string               `json:"sources"`
	Severity       ErrorSeverity          `json:"severity"`
	Category       ErrorCategory          `json:"category"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// ErrorCluster represents a cluster of related errors
type ErrorCluster struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Signatures      []*ErrorSignature `json:"signatures"`
	Count           int64             `json:"count"`
	FirstSeen       time.Time         `json:"first_seen"`
	LastSeen        time.Time         `json:"last_seen"`
	Severity        ErrorSeverity     `json:"severity"`
	Category        ErrorCategory     `json:"category"`
	AffectedSystems []string          `json:"affected_systems"`
	RootCause       string            `json:"root_cause,omitempty"`
	Resolution      string            `json:"resolution,omitempty"`
	Status          string            `json:"status"`
}

// ErrorTrend represents error trend analysis
type ErrorTrend struct {
	Type           ErrorType     `json:"type"`
	Period         time.Duration `json:"period"`
	Count          int64         `json:"count"`
	Rate           float64       `json:"rate"`
	Trend          string        `json:"trend"` // increasing, decreasing, stable
	PercentChange  float64       `json:"percent_change"`
	Predictions    []float64     `json:"predictions"`
	Confidence     float64       `json:"confidence"`
	LastCalculated time.Time     `json:"last_calculated"`
}

// HourlyErrorStats represents hourly error statistics
type HourlyErrorStats struct {
	Hour      time.Time               `json:"hour"`
	Counts    map[ErrorType]int64     `json:"counts"`
	Total     int64                   `json:"total"`
	Severity  map[ErrorSeverity]int64 `json:"severity"`
	TopErrors []string                `json:"top_errors"`
}

// DailyErrorStats represents daily error statistics
type DailyErrorStats struct {
	Date      time.Time               `json:"date"`
	Counts    map[ErrorType]int64     `json:"counts"`
	Total     int64                   `json:"total"`
	Severity  map[ErrorSeverity]int64 `json:"severity"`
	TopErrors []string                `json:"top_errors"`
	Trends    map[string]float64      `json:"trends"`
}

// NewErrorAggregator creates a new error aggregator
func NewErrorAggregator(config *ErrorHandlerConfig, logger *logger.Logger) *ErrorAggregator {
	ctx, cancel := context.WithCancel(context.Background())

	ea := &ErrorAggregator{
		config:          config,
		logger:          logger,
		errorPatterns:   make(map[string]*ErrorPattern),
		errorSignatures: make(map[string]*ErrorSignature),
		errorClusters:   make(map[string]*ErrorCluster),
		errorTrends:     make(map[string]*ErrorTrend),
		hourlyStats:     make(map[string]*HourlyErrorStats),
		dailyStats:      make(map[string]*DailyErrorStats),
		ctx:             ctx,
		cancel:          cancel,
		shutdownCh:      make(chan struct{}),
	}

	// Start background processes
	ea.startBackgroundProcesses()

	return ea
}

// RecordError records an error for aggregation
func (ea *ErrorAggregator) RecordError(appErr *AppError) {
	// Generate error signature
	signature := ea.generateErrorSignature(appErr)

	// Update signature statistics
	ea.updateSignatureStats(signature, appErr)

	// Update time-based statistics
	ea.updateTimeBasedStats(appErr)

	// Update clusters
	ea.updateClusters(signature, appErr)

	// Update trends
	ea.updateTrends(appErr)
}

// generateErrorSignature generates a unique signature for an error
func (ea *ErrorAggregator) generateErrorSignature(appErr *AppError) *ErrorSignature {
	// Create signature ID based on error characteristics
	messagePattern := ea.normalizeMessage(appErr.Message)
	stackPattern := ea.normalizeStackTrace(appErr.StackTrace)
	sourcePattern := ea.normalizeSource(appErr.Source)

	signatureID := fmt.Sprintf("%s:%s:%s:%s",
		appErr.Type,
		ea.hashString(messagePattern),
		ea.hashString(stackPattern),
		ea.hashString(sourcePattern))

	ea.mu.Lock()
	defer ea.mu.Unlock()

	if signature, exists := ea.errorSignatures[signatureID]; exists {
		return signature
	}

	// Create new signature
	signature := &ErrorSignature{
		ID:             signatureID,
		Type:           appErr.Type,
		MessagePattern: messagePattern,
		StackPattern:   stackPattern,
		SourcePattern:  sourcePattern,
		Count:          0,
		FirstSeen:      appErr.Timestamp,
		LastSeen:       appErr.Timestamp,
		AffectedUsers:  []string{},
		Sources:        []string{},
		Severity:       appErr.Severity,
		Category:       appErr.Category,
		Metadata:       make(map[string]interface{}),
	}

	ea.errorSignatures[signatureID] = signature
	return signature
}

// updateSignatureStats updates signature statistics
func (ea *ErrorAggregator) updateSignatureStats(signature *ErrorSignature, appErr *AppError) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	signature.Count++
	signature.LastSeen = appErr.Timestamp

	// Update affected users
	if appErr.UserID != "" {
		if !ea.containsString(signature.AffectedUsers, appErr.UserID) {
			signature.AffectedUsers = append(signature.AffectedUsers, appErr.UserID)
		}
	}

	// Update sources
	if appErr.Source != "" {
		if !ea.containsString(signature.Sources, appErr.Source) {
			signature.Sources = append(signature.Sources, appErr.Source)
		}
	}

	// Update metadata
	if appErr.Details != nil {
		for k, v := range appErr.Details {
			signature.Metadata[k] = v
		}
	}
}

// updateTimeBasedStats updates time-based error statistics
func (ea *ErrorAggregator) updateTimeBasedStats(appErr *AppError) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Update hourly stats
	hour := appErr.Timestamp.Truncate(time.Hour)
	hourKey := hour.Format("2006-01-02T15")

	if stats, exists := ea.hourlyStats[hourKey]; exists {
		stats.Counts[appErr.Type]++
		stats.Total++
		stats.Severity[appErr.Severity]++
	} else {
		ea.hourlyStats[hourKey] = &HourlyErrorStats{
			Hour:      hour,
			Counts:    map[ErrorType]int64{appErr.Type: 1},
			Total:     1,
			Severity:  map[ErrorSeverity]int64{appErr.Severity: 1},
			TopErrors: []string{},
		}
	}

	// Update daily stats
	day := appErr.Timestamp.Truncate(24 * time.Hour)
	dayKey := day.Format("2006-01-02")

	if stats, exists := ea.dailyStats[dayKey]; exists {
		stats.Counts[appErr.Type]++
		stats.Total++
		stats.Severity[appErr.Severity]++
	} else {
		ea.dailyStats[dayKey] = &DailyErrorStats{
			Date:      day,
			Counts:    map[ErrorType]int64{appErr.Type: 1},
			Total:     1,
			Severity:  map[ErrorSeverity]int64{appErr.Severity: 1},
			TopErrors: []string{},
			Trends:    make(map[string]float64),
		}
	}
}

// updateClusters updates error clusters
func (ea *ErrorAggregator) updateClusters(signature *ErrorSignature, appErr *AppError) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Find existing cluster or create new one
	clusterID := ea.findClusterForSignature(signature)

	if cluster, exists := ea.errorClusters[clusterID]; exists {
		cluster.Count++
		cluster.LastSeen = appErr.Timestamp

		// Add signature if not already present
		if !ea.containsSignature(cluster.Signatures, signature) {
			cluster.Signatures = append(cluster.Signatures, signature)
		}
	} else {
		// Create new cluster
		cluster := &ErrorCluster{
			ID:              clusterID,
			Name:            ea.generateClusterName(signature),
			Description:     ea.generateClusterDescription(signature),
			Signatures:      []*ErrorSignature{signature},
			Count:           1,
			FirstSeen:       appErr.Timestamp,
			LastSeen:        appErr.Timestamp,
			Severity:        signature.Severity,
			Category:        signature.Category,
			AffectedSystems: []string{},
			Status:          "active",
		}

		ea.errorClusters[clusterID] = cluster
	}
}

// updateTrends updates error trends
func (ea *ErrorAggregator) updateTrends(appErr *AppError) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	trendKey := string(appErr.Type)

	if trend, exists := ea.errorTrends[trendKey]; exists {
		trend.Count++
		trend.LastCalculated = time.Now()

		// Recalculate trend
		ea.calculateTrend(trend)
	} else {
		// Create new trend
		trend := &ErrorTrend{
			Type:           appErr.Type,
			Period:         time.Hour,
			Count:          1,
			Rate:           0,
			Trend:          "stable",
			PercentChange:  0,
			Predictions:    []float64{},
			Confidence:     0.5,
			LastCalculated: time.Now(),
		}

		ea.errorTrends[trendKey] = trend
	}
}

// Helper methods

// normalizeMessage normalizes error message for pattern matching
func (ea *ErrorAggregator) normalizeMessage(message string) string {
	// Remove specific IDs, numbers, and timestamps
	normalized := strings.ToLower(message)

	// Replace UUIDs with placeholder
	normalized = ea.replacePattern(normalized, `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`, "UUID")

	// Replace numbers with placeholder
	normalized = ea.replacePattern(normalized, `\d+`, "NUMBER")

	// Replace timestamps with placeholder
	normalized = ea.replacePattern(normalized, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, "TIMESTAMP")

	return normalized
}

// normalizeStackTrace normalizes stack trace for pattern matching
func (ea *ErrorAggregator) normalizeStackTrace(stackTrace string) string {
	if stackTrace == "" {
		return ""
	}

	// Extract function names and remove line numbers
	lines := strings.Split(stackTrace, "\n")
	var functions []string

	for _, line := range lines {
		if strings.Contains(line, "(") {
			parts := strings.Split(line, "(")
			if len(parts) > 0 {
				functions = append(functions, strings.TrimSpace(parts[0]))
			}
		}
	}

	// Return first few functions as pattern
	if len(functions) > 5 {
		functions = functions[:5]
	}

	return strings.Join(functions, "->")
}

// normalizeSource normalizes source location for pattern matching
func (ea *ErrorAggregator) normalizeSource(source string) string {
	if source == "" {
		return ""
	}

	// Extract package and function name, remove line numbers
	parts := strings.Split(source, ":")
	if len(parts) > 0 {
		return parts[0]
	}

	return source
}

// hashString creates a hash of a string
func (ea *ErrorAggregator) hashString(s string) string {
	if s == "" {
		return "empty"
	}

	// Simple hash function for demonstration
	hash := 0
	for _, char := range s {
		hash = hash*31 + int(char)
	}

	return fmt.Sprintf("%x", hash)
}

// replacePattern replaces pattern in text (simplified regex replacement)
func (ea *ErrorAggregator) replacePattern(text, pattern, replacement string) string {
	// This is a simplified version - in production, use proper regex
	return text
}

// containsString checks if slice contains string
func (ea *ErrorAggregator) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// containsSignature checks if slice contains signature
func (ea *ErrorAggregator) containsSignature(slice []*ErrorSignature, signature *ErrorSignature) bool {
	for _, s := range slice {
		if s.ID == signature.ID {
			return true
		}
	}
	return false
}

// findClusterForSignature finds appropriate cluster for signature
func (ea *ErrorAggregator) findClusterForSignature(signature *ErrorSignature) string {
	// Simple clustering based on error type and severity
	return fmt.Sprintf("%s_%s", signature.Type, signature.Severity)
}

// generateClusterName generates a name for the cluster
func (ea *ErrorAggregator) generateClusterName(signature *ErrorSignature) string {
	return fmt.Sprintf("%s Errors (%s)", signature.Type, signature.Severity)
}

// generateClusterDescription generates a description for the cluster
func (ea *ErrorAggregator) generateClusterDescription(signature *ErrorSignature) string {
	return fmt.Sprintf("Cluster of %s errors with %s severity", signature.Type, signature.Severity)
}

// calculateTrend calculates trend for error type
func (ea *ErrorAggregator) calculateTrend(trend *ErrorTrend) {
	// Simple trend calculation - in production, use proper time series analysis
	currentRate := float64(trend.Count)

	if trend.Rate > 0 {
		trend.PercentChange = ((currentRate - trend.Rate) / trend.Rate) * 100

		if trend.PercentChange > 10 {
			trend.Trend = "increasing"
		} else if trend.PercentChange < -10 {
			trend.Trend = "decreasing"
		} else {
			trend.Trend = "stable"
		}
	}

	trend.Rate = currentRate
}

// Public API methods

// GetErrorSignatures returns all error signatures
func (ea *ErrorAggregator) GetErrorSignatures() map[string]*ErrorSignature {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	signatures := make(map[string]*ErrorSignature)
	for k, v := range ea.errorSignatures {
		signatures[k] = v
	}

	return signatures
}

// GetErrorClusters returns all error clusters
func (ea *ErrorAggregator) GetErrorClusters() map[string]*ErrorCluster {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	clusters := make(map[string]*ErrorCluster)
	for k, v := range ea.errorClusters {
		clusters[k] = v
	}

	return clusters
}

// GetErrorTrends returns all error trends
func (ea *ErrorAggregator) GetErrorTrends() map[string]*ErrorTrend {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	trends := make(map[string]*ErrorTrend)
	for k, v := range ea.errorTrends {
		trends[k] = v
	}

	return trends
}

// GetHourlyStats returns hourly error statistics
func (ea *ErrorAggregator) GetHourlyStats() map[string]*HourlyErrorStats {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	stats := make(map[string]*HourlyErrorStats)
	for k, v := range ea.hourlyStats {
		stats[k] = v
	}

	return stats
}

// GetDailyStats returns daily error statistics
func (ea *ErrorAggregator) GetDailyStats() map[string]*DailyErrorStats {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	stats := make(map[string]*DailyErrorStats)
	for k, v := range ea.dailyStats {
		stats[k] = v
	}

	return stats
}

// GetTopErrorSignatures returns top error signatures by count
func (ea *ErrorAggregator) GetTopErrorSignatures(limit int) []*ErrorSignature {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	var signatures []*ErrorSignature
	for _, sig := range ea.errorSignatures {
		signatures = append(signatures, sig)
	}

	// Sort by count (descending)
	sort.Slice(signatures, func(i, j int) bool {
		return signatures[i].Count > signatures[j].Count
	})

	if limit > 0 && len(signatures) > limit {
		signatures = signatures[:limit]
	}

	return signatures
}

// GetErrorAnalysis returns comprehensive error analysis
func (ea *ErrorAggregator) GetErrorAnalysis() map[string]interface{} {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	return map[string]interface{}{
		"total_signatures": len(ea.errorSignatures),
		"total_clusters":   len(ea.errorClusters),
		"total_trends":     len(ea.errorTrends),
		"hourly_stats":     len(ea.hourlyStats),
		"daily_stats":      len(ea.dailyStats),
		"top_signatures":   ea.GetTopErrorSignatures(10),
		"active_clusters":  ea.getActiveClusters(),
		"trending_errors":  ea.getTrendingErrors(),
	}
}

// getActiveClusters returns active error clusters
func (ea *ErrorAggregator) getActiveClusters() []*ErrorCluster {
	var active []*ErrorCluster

	for _, cluster := range ea.errorClusters {
		if cluster.Status == "active" {
			active = append(active, cluster)
		}
	}

	return active
}

// getTrendingErrors returns trending errors
func (ea *ErrorAggregator) getTrendingErrors() []*ErrorTrend {
	var trending []*ErrorTrend

	for _, trend := range ea.errorTrends {
		if trend.Trend == "increasing" {
			trending = append(trending, trend)
		}
	}

	return trending
}

// startBackgroundProcesses starts background aggregation processes
func (ea *ErrorAggregator) startBackgroundProcesses() {
	ea.wg.Add(1)
	go ea.aggregationLoop()
}

// aggregationLoop runs the aggregation loop
func (ea *ErrorAggregator) aggregationLoop() {
	defer ea.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ea.performAggregation()
		case <-ea.shutdownCh:
			return
		}
	}
}

// performAggregation performs periodic aggregation
func (ea *ErrorAggregator) performAggregation() {
	// Update top errors for hourly stats
	ea.updateTopErrors()

	// Calculate trends
	ea.calculateAllTrends()

	// Clean up old data
	ea.cleanupOldData()
}

// updateTopErrors updates top errors for time-based stats
func (ea *ErrorAggregator) updateTopErrors() {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Update hourly stats
	for _, stats := range ea.hourlyStats {
		var topErrors []string

		// Sort error types by count
		type errorCount struct {
			errorType ErrorType
			count     int64
		}

		var counts []errorCount
		for errorType, count := range stats.Counts {
			counts = append(counts, errorCount{errorType, count})
		}

		sort.Slice(counts, func(i, j int) bool {
			return counts[i].count > counts[j].count
		})

		// Get top 5
		limit := 5
		if len(counts) < limit {
			limit = len(counts)
		}

		for i := 0; i < limit; i++ {
			topErrors = append(topErrors, string(counts[i].errorType))
		}

		stats.TopErrors = topErrors
	}

	// Update daily stats similarly
	for _, stats := range ea.dailyStats {
		var topErrors []string

		type errorCount struct {
			errorType ErrorType
			count     int64
		}

		var counts []errorCount
		for errorType, count := range stats.Counts {
			counts = append(counts, errorCount{errorType, count})
		}

		sort.Slice(counts, func(i, j int) bool {
			return counts[i].count > counts[j].count
		})

		limit := 5
		if len(counts) < limit {
			limit = len(counts)
		}

		for i := 0; i < limit; i++ {
			topErrors = append(topErrors, string(counts[i].errorType))
		}

		stats.TopErrors = topErrors
	}
}

// calculateAllTrends calculates trends for all error types
func (ea *ErrorAggregator) calculateAllTrends() {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	for _, trend := range ea.errorTrends {
		ea.calculateTrend(trend)
	}
}

// cleanupOldData cleans up old aggregation data
func (ea *ErrorAggregator) cleanupOldData() {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	cutoff := time.Now().Add(-ea.config.ErrorRetentionPeriod)

	// Clean up hourly stats
	for key, stats := range ea.hourlyStats {
		if stats.Hour.Before(cutoff) {
			delete(ea.hourlyStats, key)
		}
	}

	// Clean up daily stats
	for key, stats := range ea.dailyStats {
		if stats.Date.Before(cutoff) {
			delete(ea.dailyStats, key)
		}
	}

	// Clean up old signatures
	for key, signature := range ea.errorSignatures {
		if signature.LastSeen.Before(cutoff) {
			delete(ea.errorSignatures, key)
		}
	}

	// Clean up old clusters
	for key, cluster := range ea.errorClusters {
		if cluster.LastSeen.Before(cutoff) {
			delete(ea.errorClusters, key)
		}
	}
}

// Close closes the aggregator
func (ea *ErrorAggregator) Close() {
	ea.cancel()
	close(ea.shutdownCh)
	ea.wg.Wait()
}
