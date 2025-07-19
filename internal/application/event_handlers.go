package application

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// EventHandlerConfig represents configuration for event handlers
type EventHandlerConfig struct {
	MaxWorkers          int           `json:"max_workers"`
	MaxRetries          int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`
	RetryBackoffFactor  float64       `json:"retry_backoff_factor"`
	MaxRetryDelay       time.Duration `json:"max_retry_delay"`
	ProcessingTimeout   time.Duration `json:"processing_timeout"`
	BufferSize          int           `json:"buffer_size"`
	EnableMetrics       bool          `json:"enable_metrics"`
	EnableNotifications bool          `json:"enable_notifications"`
	EnableReplay        bool          `json:"enable_replay"`
	DeadLetterThreshold int           `json:"dead_letter_threshold"`
}

// EventHandlerResult represents the result of event handling
type EventHandlerResult struct {
	EventID     uuid.UUID              `json:"event_id"`
	HandlerName string                 `json:"handler_name"`
	ProcessedAt time.Time              `json:"processed_at"`
	Duration    time.Duration          `json:"duration"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	IsReplay    bool                   `json:"is_replay"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// EventHandlerMetrics represents metrics for event handlers
type EventHandlerMetrics struct {
	TotalProcessed   int64                      `json:"total_processed"`
	TotalErrors      int64                      `json:"total_errors"`
	TotalRetries     int64                      `json:"total_retries"`
	AverageLatency   time.Duration              `json:"average_latency"`
	ProcessingRate   float64                    `json:"processing_rate"`
	ErrorRate        float64                    `json:"error_rate"`
	LastProcessedAt  time.Time                  `json:"last_processed_at"`
	EventTypeMetrics map[domain.EventType]int64 `json:"event_type_metrics"`
	HandlerMetrics   map[string]int64           `json:"handler_metrics"`
	ReplayMetrics    map[string]int64           `json:"replay_metrics"`
}

// BaseEventHandler provides common functionality for all event handlers
type BaseEventHandler struct {
	name           string
	config         *EventHandlerConfig
	logger         *logger.Logger
	metrics        *EventHandlerMetrics
	mu             sync.RWMutex
	processingChan chan *EventHandlingTask
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	started        bool
}

// EventHandlingTask represents a task for handling an event
type EventHandlingTask struct {
	ID            uuid.UUID
	Event         domain.DomainEvent
	HandlerName   string
	RetryCount    int
	MaxRetries    int
	CreatedAt     time.Time
	LastAttemptAt time.Time
	NextAttemptAt time.Time
	Context       context.Context
	ResultChan    chan *EventHandlerResult
	IsReplay      bool
	ReplayContext map[string]interface{}
}

// NewBaseEventHandler creates a new base event handler
func NewBaseEventHandler(name string, config *EventHandlerConfig, logger *logger.Logger) *BaseEventHandler {
	ctx, cancel := context.WithCancel(context.Background())

	return &BaseEventHandler{
		name:   name,
		config: config,
		logger: logger,
		metrics: &EventHandlerMetrics{
			EventTypeMetrics: make(map[domain.EventType]int64),
			HandlerMetrics:   make(map[string]int64),
			ReplayMetrics:    make(map[string]int64),
		},
		processingChan: make(chan *EventHandlingTask, config.BufferSize),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the event handler
func (h *BaseEventHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.started {
		return fmt.Errorf("event handler %s already started", h.name)
	}

	h.logger.Info("Starting event handler", "handler", h.name, "workers", h.config.MaxWorkers)

	// Start worker goroutines
	for i := 0; i < h.config.MaxWorkers; i++ {
		h.wg.Add(1)
		go h.workerLoop(i)
	}

	// Start metrics collector
	if h.config.EnableMetrics {
		h.wg.Add(1)
		go h.metricsLoop()
	}

	h.started = true
	return nil
}

// Stop stops the event handler
func (h *BaseEventHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.started {
		return fmt.Errorf("event handler %s not started", h.name)
	}

	h.logger.Info("Stopping event handler", "handler", h.name)

	h.cancel()
	h.wg.Wait()

	h.started = false
	return nil
}

// HandleEventAsync handles an event asynchronously
func (h *BaseEventHandler) HandleEventAsync(ctx context.Context, event domain.DomainEvent) error {
	task := &EventHandlingTask{
		ID:            uuid.New(),
		Event:         event,
		HandlerName:   h.name,
		RetryCount:    0,
		MaxRetries:    h.config.MaxRetries,
		CreatedAt:     time.Now(),
		LastAttemptAt: time.Now(),
		NextAttemptAt: time.Now(),
		Context:       ctx,
		ResultChan:    make(chan *EventHandlerResult, 1),
		IsReplay:      event.IsReplayable(),
	}

	select {
	case h.processingChan <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("event handler %s processing queue is full", h.name)
	}
}

// HandleEventSync handles an event synchronously
func (h *BaseEventHandler) HandleEventSync(ctx context.Context, event domain.DomainEvent) (*EventHandlerResult, error) {
	task := &EventHandlingTask{
		ID:            uuid.New(),
		Event:         event,
		HandlerName:   h.name,
		RetryCount:    0,
		MaxRetries:    h.config.MaxRetries,
		CreatedAt:     time.Now(),
		LastAttemptAt: time.Now(),
		NextAttemptAt: time.Now(),
		Context:       ctx,
		ResultChan:    make(chan *EventHandlerResult, 1),
		IsReplay:      event.IsReplayable(),
	}

	select {
	case h.processingChan <- task:
		// Wait for result
		select {
		case result := <-task.ResultChan:
			return result, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, fmt.Errorf("event handler %s processing queue is full", h.name)
	}
}

// workerLoop is the main loop for worker goroutines
func (h *BaseEventHandler) workerLoop(workerID int) {
	defer h.wg.Done()

	h.logger.Debug("Event handler worker started", "handler", h.name, "worker_id", workerID)

	for {
		select {
		case <-h.ctx.Done():
			h.logger.Debug("Event handler worker stopping", "handler", h.name, "worker_id", workerID)
			return
		case task := <-h.processingChan:
			h.processTask(task)
		}
	}
}

// processTask processes a single event handling task
func (h *BaseEventHandler) processTask(task *EventHandlingTask) {
	startTime := time.Now()

	// Create timeout context
	ctx, cancel := context.WithTimeout(task.Context, h.config.ProcessingTimeout)
	defer cancel()

	// Process the task
	result := &EventHandlerResult{
		EventID:     task.Event.GetID(),
		HandlerName: task.HandlerName,
		ProcessedAt: startTime,
		RetryCount:  task.RetryCount,
		IsReplay:    task.IsReplay,
		Metadata:    make(map[string]interface{}),
	}

	// Handle the event with recovery
	err := h.handleEventWithRecovery(ctx, task.Event, result)

	// Calculate duration
	result.Duration = time.Since(startTime)
	result.Success = err == nil

	if err != nil {
		result.Error = err.Error()
		h.logger.Error("Event handling failed",
			"handler", h.name,
			"event_id", task.Event.GetID(),
			"event_type", task.Event.GetType(),
			"error", err,
			"retry_count", task.RetryCount,
		)

		// Handle retry logic
		if task.RetryCount < task.MaxRetries {
			h.scheduleRetry(task)
			return
		}

		// Send to dead letter queue if max retries exceeded
		if task.RetryCount >= h.config.DeadLetterThreshold {
			h.sendToDeadLetterQueue(task, err)
		}
	} else {
		h.logger.Debug("Event handled successfully",
			"handler", h.name,
			"event_id", task.Event.GetID(),
			"event_type", task.Event.GetType(),
			"duration", result.Duration,
		)
	}

	// Update metrics
	h.updateMetrics(result)

	// Send result if channel is available
	if task.ResultChan != nil {
		select {
		case task.ResultChan <- result:
		default:
			// Channel full or closed, ignore
		}
	}
}

// handleEventWithRecovery handles an event with panic recovery
func (h *BaseEventHandler) handleEventWithRecovery(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in event handler: %v", r)
			result.Metadata["panic"] = r
			result.Metadata["stack_trace"] = string(debug.Stack())
			h.logger.Error("Event handler panic recovered",
				"handler", h.name,
				"event_id", event.GetID(),
				"panic", r,
				"stack_trace", string(debug.Stack()),
			)
		}
	}()

	// This method should be overridden by specific handlers
	return h.handleEvent(ctx, event, result)
}

// handleEvent is the default event handling method (should be overridden)
func (h *BaseEventHandler) handleEvent(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	return fmt.Errorf("handleEvent method not implemented for handler %s", h.name)
}

// scheduleRetry schedules a retry for a failed task
func (h *BaseEventHandler) scheduleRetry(task *EventHandlingTask) {
	task.RetryCount++
	task.LastAttemptAt = time.Now()

	// Calculate next retry delay with exponential backoff
	delay := time.Duration(float64(h.config.RetryDelay) *
		float64(task.RetryCount) * h.config.RetryBackoffFactor)

	if delay > h.config.MaxRetryDelay {
		delay = h.config.MaxRetryDelay
	}

	task.NextAttemptAt = time.Now().Add(delay)

	h.logger.Info("Scheduling event retry",
		"handler", h.name,
		"event_id", task.Event.GetID(),
		"retry_count", task.RetryCount,
		"next_attempt_at", task.NextAttemptAt,
		"delay", delay,
	)

	// Schedule the retry
	go func() {
		time.Sleep(delay)
		select {
		case h.processingChan <- task:
		case <-h.ctx.Done():
			// Handler is stopping, abandon retry
		}
	}()
}

// sendToDeadLetterQueue sends a task to the dead letter queue
func (h *BaseEventHandler) sendToDeadLetterQueue(task *EventHandlingTask, err error) {
	h.logger.Error("Sending event to dead letter queue",
		"handler", h.name,
		"event_id", task.Event.GetID(),
		"event_type", task.Event.GetType(),
		"retry_count", task.RetryCount,
		"error", err,
	)

	// Implementation would depend on your dead letter queue mechanism
	// This could be a database table, message queue, file system, etc.
}

// updateMetrics updates handler metrics
func (h *BaseEventHandler) updateMetrics(result *EventHandlerResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.metrics.TotalProcessed++
	h.metrics.LastProcessedAt = result.ProcessedAt

	if !result.Success {
		h.metrics.TotalErrors++
	}

	if result.RetryCount > 0 {
		h.metrics.TotalRetries++
	}

	// Update latency (simple moving average)
	if h.metrics.AverageLatency == 0 {
		h.metrics.AverageLatency = result.Duration
	} else {
		h.metrics.AverageLatency = (h.metrics.AverageLatency + result.Duration) / 2
	}

	// Update rates
	if h.metrics.TotalProcessed > 0 {
		h.metrics.ErrorRate = float64(h.metrics.TotalErrors) / float64(h.metrics.TotalProcessed)
	}

	// Update handler-specific metrics
	h.metrics.HandlerMetrics[result.HandlerName]++

	// Update replay metrics
	if result.IsReplay {
		h.metrics.ReplayMetrics[result.HandlerName]++
	}
}

// metricsLoop runs the metrics collection loop
func (h *BaseEventHandler) metricsLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.collectMetrics()
		}
	}
}

// collectMetrics collects and logs metrics
func (h *BaseEventHandler) collectMetrics() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Calculate processing rate (events per second)
	if !h.metrics.LastProcessedAt.IsZero() {
		duration := time.Since(h.metrics.LastProcessedAt)
		if duration > 0 {
			h.metrics.ProcessingRate = float64(h.metrics.TotalProcessed) / duration.Seconds()
		}
	}

	h.logger.Info("Event handler metrics",
		"handler", h.name,
		"total_processed", h.metrics.TotalProcessed,
		"total_errors", h.metrics.TotalErrors,
		"total_retries", h.metrics.TotalRetries,
		"average_latency", h.metrics.AverageLatency,
		"processing_rate", h.metrics.ProcessingRate,
		"error_rate", h.metrics.ErrorRate,
	)
}

// GetMetrics returns current metrics
func (h *BaseEventHandler) GetMetrics() *EventHandlerMetrics {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &EventHandlerMetrics{
		TotalProcessed:   h.metrics.TotalProcessed,
		TotalErrors:      h.metrics.TotalErrors,
		TotalRetries:     h.metrics.TotalRetries,
		AverageLatency:   h.metrics.AverageLatency,
		ProcessingRate:   h.metrics.ProcessingRate,
		ErrorRate:        h.metrics.ErrorRate,
		LastProcessedAt:  h.metrics.LastProcessedAt,
		EventTypeMetrics: make(map[domain.EventType]int64),
		HandlerMetrics:   make(map[string]int64),
		ReplayMetrics:    make(map[string]int64),
	}

	// Copy maps
	for k, v := range h.metrics.EventTypeMetrics {
		metrics.EventTypeMetrics[k] = v
	}
	for k, v := range h.metrics.HandlerMetrics {
		metrics.HandlerMetrics[k] = v
	}
	for k, v := range h.metrics.ReplayMetrics {
		metrics.ReplayMetrics[k] = v
	}

	return metrics
}

// NotificationEventHandler handles events that require notifications
type NotificationEventHandler struct {
	*BaseEventHandler
	reviewService       domain.ReviewService
	notificationService NotificationService
}

// NotificationService represents the interface for sending notifications
type NotificationService interface {
	SendEmail(ctx context.Context, to []string, subject, body string) error
	SendSlack(ctx context.Context, channel, message string) error
	SendWebhook(ctx context.Context, url string, payload interface{}) error
	SendSMS(ctx context.Context, to []string, message string) error
}

// NewNotificationEventHandler creates a new notification event handler
func NewNotificationEventHandler(
	config *EventHandlerConfig,
	logger *logger.Logger,
	reviewService domain.ReviewService,
	notificationService NotificationService,
) *NotificationEventHandler {
	base := NewBaseEventHandler("notification", config, logger)

	return &NotificationEventHandler{
		BaseEventHandler:    base,
		reviewService:       reviewService,
		notificationService: notificationService,
	}
}

// handleEvent handles notification events
func (h *NotificationEventHandler) handleEvent(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	switch event.GetType() {
	case domain.FileProcessingStartedEventType:
		return h.handleFileProcessingStarted(ctx, event, result)
	case domain.FileProcessingCompletedEventType:
		return h.handleFileProcessingCompleted(ctx, event, result)
	case domain.FileProcessingFailedEventType:
		return h.handleFileProcessingFailed(ctx, event, result)
	case domain.ReviewBatchProcessedEventType:
		return h.handleReviewBatchProcessed(ctx, event, result)
	case domain.SystemErrorEvent:
		return h.handleSystemError(ctx, event, result)
	default:
		return nil // Ignore unknown events
	}
}

// handleFileProcessingStarted handles file processing started events
func (h *NotificationEventHandler) handleFileProcessingStarted(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	startedEvent, ok := event.(*domain.FileProcessingStartedEvent)
	if !ok {
		return fmt.Errorf("expected FileProcessingStartedEvent, got %T", event)
	}

	// Send notification for large files
	if startedEvent.FileSize > 100*1024*1024 { // 100MB
		message := fmt.Sprintf("Large file processing started: %s (%.2f MB)",
			startedEvent.FileName, float64(startedEvent.FileSize)/(1024*1024))

		if err := h.notificationService.SendSlack(ctx, "processing-alerts", message); err != nil {
			h.logger.Error("Failed to send Slack notification", "error", err)
			return err
		}

		result.Metadata["notification_sent"] = "slack"
	}

	return nil
}

// handleFileProcessingCompleted handles file processing completed events
func (h *NotificationEventHandler) handleFileProcessingCompleted(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	completedEvent, ok := event.(*domain.FileProcessingCompletedEvent)
	if !ok {
		return fmt.Errorf("expected FileProcessingCompletedEvent, got %T", event)
	}

	// Send completion notification
	message := fmt.Sprintf("File processing completed: %d records processed in %s",
		completedEvent.RecordsProcessed, completedEvent.ProcessingDuration)

	if err := h.notificationService.SendSlack(ctx, "processing-status", message); err != nil {
		h.logger.Error("Failed to send completion notification", "error", err)
		return err
	}

	result.Metadata["notification_sent"] = "slack"
	result.Metadata["records_processed"] = completedEvent.RecordsProcessed

	return nil
}

// handleFileProcessingFailed handles file processing failed events
func (h *NotificationEventHandler) handleFileProcessingFailed(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	failedEvent, ok := event.(*domain.FileProcessingFailedEvent)
	if !ok {
		return fmt.Errorf("expected FileProcessingFailedEvent, got %T", event)
	}

	// Send failure alert
	message := fmt.Sprintf("🚨 File processing failed: %s (Retry %d/%d)",
		failedEvent.ErrorMessage, failedEvent.RetryCount, failedEvent.MaxRetries)

	if err := h.notificationService.SendSlack(ctx, "processing-alerts", message); err != nil {
		h.logger.Error("Failed to send failure notification", "error", err)
		return err
	}

	// Send email for critical failures
	if failedEvent.RetryCount >= failedEvent.MaxRetries {
		subject := "Critical: File Processing Failed"
		body := fmt.Sprintf("Job %s has failed after %d retries.\n\nError: %s\n\nJob Details:\n%+v",
			failedEvent.JobID, failedEvent.RetryCount, failedEvent.ErrorMessage, failedEvent)

		if err := h.notificationService.SendEmail(ctx, []string{"ops@company.com"}, subject, body); err != nil {
			h.logger.Error("Failed to send email notification", "error", err)
			return err
		}

		result.Metadata["critical_alert_sent"] = true
	}

	result.Metadata["notification_sent"] = "slack"
	return nil
}

// handleReviewBatchProcessed handles review batch processed events
func (h *NotificationEventHandler) handleReviewBatchProcessed(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	batchEvent, ok := event.(*domain.ReviewBatchProcessedEvent)
	if !ok {
		return fmt.Errorf("expected ReviewBatchProcessedEvent, got %T", event)
	}

	// Send notification for large batches
	if batchEvent.BatchSize > 1000 {
		message := fmt.Sprintf("Large review batch processed: %d reviews in %s",
			batchEvent.ReviewsProcessed, batchEvent.ProcessingTime)

		if err := h.notificationService.SendSlack(ctx, "processing-status", message); err != nil {
			h.logger.Error("Failed to send batch notification", "error", err)
			return err
		}

		result.Metadata["notification_sent"] = "slack"
	}

	return nil
}

// handleSystemError handles system error events
func (h *NotificationEventHandler) handleSystemError(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	// Send critical system error notifications
	message := fmt.Sprintf("🚨 System Error: %s", event.GetMetadata()["error"])

	if err := h.notificationService.SendSlack(ctx, "system-alerts", message); err != nil {
		h.logger.Error("Failed to send system error notification", "error", err)
		return err
	}

	result.Metadata["notification_sent"] = "slack"
	return nil
}

// CanHandle returns true if this handler can handle the event type
func (h *NotificationEventHandler) CanHandle(eventType domain.EventType) bool {
	switch eventType {
	case domain.FileProcessingStartedEventType,
		domain.FileProcessingCompletedEventType,
		domain.FileProcessingFailedEventType,
		domain.ReviewBatchProcessedEventType,
		domain.SystemErrorEvent:
		return true
	default:
		return false
	}
}

// GetHandledEventTypes returns the event types this handler can handle
func (h *NotificationEventHandler) GetHandledEventTypes() []domain.EventType {
	return []domain.EventType{
		domain.FileProcessingStartedEventType,
		domain.FileProcessingCompletedEventType,
		domain.FileProcessingFailedEventType,
		domain.ReviewBatchProcessedEventType,
		domain.SystemErrorEvent,
	}
}

// MetricsEventHandler handles events for metrics collection
type MetricsEventHandler struct {
	*BaseEventHandler
	metricsCollector MetricsCollector
}

// MetricsCollector represents the interface for collecting metrics
type MetricsCollector interface {
	IncrementCounter(name string, tags map[string]string, value float64) error
	RecordGauge(name string, tags map[string]string, value float64) error
	RecordHistogram(name string, tags map[string]string, value float64) error
	RecordTimer(name string, tags map[string]string, duration time.Duration) error
}

// NewMetricsEventHandler creates a new metrics event handler
func NewMetricsEventHandler(
	config *EventHandlerConfig,
	logger *logger.Logger,
	metricsCollector MetricsCollector,
) *MetricsEventHandler {
	base := NewBaseEventHandler("metrics", config, logger)

	return &MetricsEventHandler{
		BaseEventHandler: base,
		metricsCollector: metricsCollector,
	}
}

// handleEvent handles metrics collection events
func (h *MetricsEventHandler) handleEvent(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	switch event.GetType() {
	case domain.FileProcessingStartedEventType:
		return h.handleFileProcessingStartedMetrics(ctx, event, result)
	case domain.FileProcessingCompletedEventType:
		return h.handleFileProcessingCompletedMetrics(ctx, event, result)
	case domain.FileProcessingFailedEventType:
		return h.handleFileProcessingFailedMetrics(ctx, event, result)
	case domain.ReviewProcessedEventType:
		return h.handleReviewProcessedMetrics(ctx, event, result)
	case domain.ReviewBatchProcessedEventType:
		return h.handleReviewBatchProcessedMetrics(ctx, event, result)
	default:
		return nil // Ignore unknown events
	}
}

// handleFileProcessingStartedMetrics handles metrics for file processing started events
func (h *MetricsEventHandler) handleFileProcessingStartedMetrics(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	startedEvent, ok := event.(*domain.FileProcessingStartedEvent)
	if !ok {
		return fmt.Errorf("expected FileProcessingStartedEvent, got %T", event)
	}

	tags := map[string]string{
		"provider": startedEvent.ProviderName,
		"priority": fmt.Sprintf("%d", startedEvent.Priority),
	}

	// Increment processing started counter
	if err := h.metricsCollector.IncrementCounter("file_processing_started", tags, 1); err != nil {
		return err
	}

	// Record file size
	if err := h.metricsCollector.RecordGauge("file_size_bytes", tags, float64(startedEvent.FileSize)); err != nil {
		return err
	}

	// Record estimated records
	if err := h.metricsCollector.RecordGauge("estimated_records", tags, float64(startedEvent.EstimatedRecords)); err != nil {
		return err
	}

	result.Metadata["metrics_recorded"] = []string{"file_processing_started", "file_size_bytes", "estimated_records"}
	return nil
}

// handleFileProcessingCompletedMetrics handles metrics for file processing completed events
func (h *MetricsEventHandler) handleFileProcessingCompletedMetrics(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	completedEvent, ok := event.(*domain.FileProcessingCompletedEvent)
	if !ok {
		return fmt.Errorf("expected FileProcessingCompletedEvent, got %T", event)
	}

	tags := map[string]string{
		"status": "completed",
	}

	// Increment processing completed counter
	if err := h.metricsCollector.IncrementCounter("file_processing_completed", tags, 1); err != nil {
		return err
	}

	// Record processing duration
	if err := h.metricsCollector.RecordTimer("processing_duration", tags, completedEvent.ProcessingDuration); err != nil {
		return err
	}

	// Record records processed
	if err := h.metricsCollector.RecordGauge("records_processed", tags, float64(completedEvent.RecordsProcessed)); err != nil {
		return err
	}

	// Record processing rate
	if err := h.metricsCollector.RecordGauge("processing_rate", tags, completedEvent.ProcessingRate); err != nil {
		return err
	}

	result.Metadata["metrics_recorded"] = []string{"file_processing_completed", "processing_duration", "records_processed", "processing_rate"}
	return nil
}

// handleFileProcessingFailedMetrics handles metrics for file processing failed events
func (h *MetricsEventHandler) handleFileProcessingFailedMetrics(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	failedEvent, ok := event.(*domain.FileProcessingFailedEvent)
	if !ok {
		return fmt.Errorf("expected FileProcessingFailedEvent, got %T", event)
	}

	tags := map[string]string{
		"status":     "failed",
		"error_type": failedEvent.ErrorType,
		"retryable":  fmt.Sprintf("%t", failedEvent.IsRetryable),
	}

	// Increment processing failed counter
	if err := h.metricsCollector.IncrementCounter("file_processing_failed", tags, 1); err != nil {
		return err
	}

	// Record retry count
	if err := h.metricsCollector.RecordHistogram("retry_count", tags, float64(failedEvent.RetryCount)); err != nil {
		return err
	}

	result.Metadata["metrics_recorded"] = []string{"file_processing_failed", "retry_count"}
	return nil
}

// handleReviewProcessedMetrics handles metrics for review processed events
func (h *MetricsEventHandler) handleReviewProcessedMetrics(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	reviewEvent, ok := event.(*domain.ReviewProcessedEvent)
	if !ok {
		return fmt.Errorf("expected ReviewProcessedEvent, got %T", event)
	}

	tags := map[string]string{
		"sentiment": reviewEvent.Sentiment,
		"language":  reviewEvent.Language,
	}

	// Increment review processed counter
	if err := h.metricsCollector.IncrementCounter("review_processed", tags, 1); err != nil {
		return err
	}

	// Record rating
	if err := h.metricsCollector.RecordHistogram("review_rating", tags, reviewEvent.Rating); err != nil {
		return err
	}

	// Record word count
	if err := h.metricsCollector.RecordHistogram("review_word_count", tags, float64(reviewEvent.WordCount)); err != nil {
		return err
	}

	result.Metadata["metrics_recorded"] = []string{"review_processed", "review_rating", "review_word_count"}
	return nil
}

// handleReviewBatchProcessedMetrics handles metrics for review batch processed events
func (h *MetricsEventHandler) handleReviewBatchProcessedMetrics(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	batchEvent, ok := event.(*domain.ReviewBatchProcessedEvent)
	if !ok {
		return fmt.Errorf("expected ReviewBatchProcessedEvent, got %T", event)
	}

	tags := map[string]string{
		"batch_id": batchEvent.BatchID,
	}

	// Increment batch processed counter
	if err := h.metricsCollector.IncrementCounter("review_batch_processed", tags, 1); err != nil {
		return err
	}

	// Record batch size
	if err := h.metricsCollector.RecordHistogram("batch_size", tags, float64(batchEvent.BatchSize)); err != nil {
		return err
	}

	// Record processing time
	if err := h.metricsCollector.RecordTimer("batch_processing_time", tags, batchEvent.ProcessingTime); err != nil {
		return err
	}

	// Record success rate
	successRate := float64(batchEvent.ReviewsProcessed) / float64(batchEvent.BatchSize) * 100
	if err := h.metricsCollector.RecordGauge("batch_success_rate", tags, successRate); err != nil {
		return err
	}

	result.Metadata["metrics_recorded"] = []string{"review_batch_processed", "batch_size", "batch_processing_time", "batch_success_rate"}
	return nil
}

// CanHandle returns true if this handler can handle the event type
func (h *MetricsEventHandler) CanHandle(eventType domain.EventType) bool {
	switch eventType {
	case domain.FileProcessingStartedEventType,
		domain.FileProcessingCompletedEventType,
		domain.FileProcessingFailedEventType,
		domain.ReviewProcessedEventType,
		domain.ReviewBatchProcessedEventType:
		return true
	default:
		return false
	}
}

// GetHandledEventTypes returns the event types this handler can handle
func (h *MetricsEventHandler) GetHandledEventTypes() []domain.EventType {
	return []domain.EventType{
		domain.FileProcessingStartedEventType,
		domain.FileProcessingCompletedEventType,
		domain.FileProcessingFailedEventType,
		domain.ReviewProcessedEventType,
		domain.ReviewBatchProcessedEventType,
	}
}

// StateUpdateEventHandler handles events for state updates and projections
type StateUpdateEventHandler struct {
	*BaseEventHandler
	reviewService   domain.ReviewService
	projectionStore ProjectionStore
}

// ProjectionStore represents the interface for projection storage
type ProjectionStore interface {
	UpdateHotelSummary(ctx context.Context, hotelID uuid.UUID, summary interface{}) error
	UpdateProviderStats(ctx context.Context, providerID uuid.UUID, stats interface{}) error
	UpdateProcessingStats(ctx context.Context, jobID uuid.UUID, stats interface{}) error
	UpdateReviewAnalytics(ctx context.Context, reviewID uuid.UUID, analytics interface{}) error
}

// NewStateUpdateEventHandler creates a new state update event handler
func NewStateUpdateEventHandler(
	config *EventHandlerConfig,
	logger *logger.Logger,
	reviewService domain.ReviewService,
	projectionStore ProjectionStore,
) *StateUpdateEventHandler {
	base := NewBaseEventHandler("state_update", config, logger)

	return &StateUpdateEventHandler{
		BaseEventHandler: base,
		reviewService:    reviewService,
		projectionStore:  projectionStore,
	}
}

// handleEvent handles state update events
func (h *StateUpdateEventHandler) handleEvent(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	switch event.GetType() {
	case domain.ReviewProcessedEventType:
		return h.handleReviewProcessedStateUpdate(ctx, event, result)
	case domain.ReviewBatchProcessedEventType:
		return h.handleReviewBatchProcessedStateUpdate(ctx, event, result)
	case domain.FileProcessingCompletedEventType:
		return h.handleFileProcessingCompletedStateUpdate(ctx, event, result)
	case domain.HotelSummaryUpdatedEventType:
		return h.handleHotelSummaryUpdatedStateUpdate(ctx, event, result)
	default:
		return nil // Ignore unknown events
	}
}

// handleReviewProcessedStateUpdate handles state updates for review processed events
func (h *StateUpdateEventHandler) handleReviewProcessedStateUpdate(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	reviewEvent, ok := event.(*domain.ReviewProcessedEvent)
	if !ok {
		return fmt.Errorf("expected ReviewProcessedEvent, got %T", event)
	}

	// Update review analytics projection
	analytics := map[string]interface{}{
		"review_id":       reviewEvent.ReviewID,
		"sentiment":       reviewEvent.Sentiment,
		"language":        reviewEvent.Language,
		"word_count":      reviewEvent.WordCount,
		"rating":          reviewEvent.Rating,
		"processed_at":    reviewEvent.ProcessedAt,
		"enrichment_data": reviewEvent.EnrichmentData,
	}

	if err := h.projectionStore.UpdateReviewAnalytics(ctx, reviewEvent.ReviewID, analytics); err != nil {
		return fmt.Errorf("failed to update review analytics: %w", err)
	}

	result.Metadata["projection_updated"] = "review_analytics"
	return nil
}

// handleReviewBatchProcessedStateUpdate handles state updates for review batch processed events
func (h *StateUpdateEventHandler) handleReviewBatchProcessedStateUpdate(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	batchEvent, ok := event.(*domain.ReviewBatchProcessedEvent)
	if !ok {
		return fmt.Errorf("expected ReviewBatchProcessedEvent, got %T", event)
	}

	// Update hotel summaries for affected hotels
	for hotelID, reviewCount := range batchEvent.HotelStats {
		summary := map[string]interface{}{
			"hotel_id":         hotelID,
			"new_review_count": reviewCount,
			"batch_id":         batchEvent.BatchID,
			"processing_time":  batchEvent.ProcessingTime,
			"last_updated":     batchEvent.ProcessedAt,
		}

		if err := h.projectionStore.UpdateHotelSummary(ctx, hotelID, summary); err != nil {
			h.logger.Error("Failed to update hotel summary",
				"hotel_id", hotelID,
				"error", err,
			)
			// Continue with other hotels instead of failing completely
		}
	}

	// Update provider stats
	stats := map[string]interface{}{
		"provider_id":       batchEvent.ProviderID,
		"batch_id":          batchEvent.BatchID,
		"reviews_processed": batchEvent.ReviewsProcessed,
		"reviews_failed":    batchEvent.ReviewsFailed,
		"processing_time":   batchEvent.ProcessingTime,
		"validation_stats":  batchEvent.ValidationStats,
		"enrichment_stats":  batchEvent.EnrichmentStats,
		"last_updated":      batchEvent.ProcessedAt,
	}

	if err := h.projectionStore.UpdateProviderStats(ctx, batchEvent.ProviderID, stats); err != nil {
		return fmt.Errorf("failed to update provider stats: %w", err)
	}

	result.Metadata["projections_updated"] = []string{"hotel_summary", "provider_stats"}
	return nil
}

// handleFileProcessingCompletedStateUpdate handles state updates for file processing completed events
func (h *StateUpdateEventHandler) handleFileProcessingCompletedStateUpdate(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	completedEvent, ok := event.(*domain.FileProcessingCompletedEvent)
	if !ok {
		return fmt.Errorf("expected FileProcessingCompletedEvent, got %T", event)
	}

	// Update processing stats
	stats := map[string]interface{}{
		"job_id":              completedEvent.JobID,
		"records_processed":   completedEvent.RecordsProcessed,
		"records_total":       completedEvent.RecordsTotal,
		"records_failed":      completedEvent.RecordsFailed,
		"records_skipped":     completedEvent.RecordsSkipped,
		"processing_duration": completedEvent.ProcessingDuration,
		"processing_rate":     completedEvent.ProcessingRate,
		"completed_at":        completedEvent.CompletedAt,
		"summary":             completedEvent.Summary,
		"metrics":             completedEvent.Metrics,
	}

	if err := h.projectionStore.UpdateProcessingStats(ctx, completedEvent.JobID, stats); err != nil {
		return fmt.Errorf("failed to update processing stats: %w", err)
	}

	result.Metadata["projection_updated"] = "processing_stats"
	return nil
}

// handleHotelSummaryUpdatedStateUpdate handles state updates for hotel summary updated events
func (h *StateUpdateEventHandler) handleHotelSummaryUpdatedStateUpdate(ctx context.Context, event domain.DomainEvent, result *EventHandlerResult) error {
	// This would handle HotelSummaryUpdatedEvent when it's implemented
	// For now, we'll just log it
	h.logger.Info("Hotel summary updated event received", "event_id", event.GetID())
	return nil
}

// CanHandle returns true if this handler can handle the event type
func (h *StateUpdateEventHandler) CanHandle(eventType domain.EventType) bool {
	switch eventType {
	case domain.ReviewProcessedEventType,
		domain.ReviewBatchProcessedEventType,
		domain.FileProcessingCompletedEventType,
		domain.HotelSummaryUpdatedEventType:
		return true
	default:
		return false
	}
}

// GetHandledEventTypes returns the event types this handler can handle
func (h *StateUpdateEventHandler) GetHandledEventTypes() []domain.EventType {
	return []domain.EventType{
		domain.ReviewProcessedEventType,
		domain.ReviewBatchProcessedEventType,
		domain.FileProcessingCompletedEventType,
		domain.HotelSummaryUpdatedEventType,
	}
}

// EventHandlerRegistry manages all event handlers
type EventHandlerRegistry struct {
	handlers map[domain.EventType][]domain.EventHandler
	mu       sync.RWMutex
	logger   *logger.Logger
	started  bool
}

// NewEventHandlerRegistry creates a new event handler registry
func NewEventHandlerRegistry(logger *logger.Logger) *EventHandlerRegistry {
	return &EventHandlerRegistry{
		handlers: make(map[domain.EventType][]domain.EventHandler),
		logger:   logger,
	}
}

// RegisterHandler registers an event handler for specific event types
func (r *EventHandlerRegistry) RegisterHandler(handler domain.EventHandler, eventTypes ...domain.EventType) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, eventType := range eventTypes {
		if r.handlers[eventType] == nil {
			r.handlers[eventType] = make([]domain.EventHandler, 0)
		}
		r.handlers[eventType] = append(r.handlers[eventType], handler)
	}

	r.logger.Info("Event handler registered",
		"handler", fmt.Sprintf("%T", handler),
		"event_types", eventTypes,
	)

	return nil
}

// UnregisterHandler unregisters an event handler
func (r *EventHandlerRegistry) UnregisterHandler(handler domain.EventHandler, eventTypes ...domain.EventType) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, eventType := range eventTypes {
		if handlers, exists := r.handlers[eventType]; exists {
			// Remove the handler from the slice
			for i, h := range handlers {
				if h == handler {
					r.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}

			// Remove the event type if no handlers remain
			if len(r.handlers[eventType]) == 0 {
				delete(r.handlers, eventType)
			}
		}
	}

	r.logger.Info("Event handler unregistered",
		"handler", fmt.Sprintf("%T", handler),
		"event_types", eventTypes,
	)

	return nil
}

// HandleEvent handles an event by dispatching it to all registered handlers
func (r *EventHandlerRegistry) HandleEvent(ctx context.Context, event domain.DomainEvent) error {
	r.mu.RLock()
	handlers, exists := r.handlers[event.GetType()]
	r.mu.RUnlock()

	if !exists || len(handlers) == 0 {
		r.logger.Debug("No handlers registered for event type",
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
		return nil
	}

	// Handle event with all registered handlers
	var errors []error
	for _, handler := range handlers {
		if handler.CanHandle(event.GetType()) {
			if err := handler.HandleEvent(ctx, event); err != nil {
				errors = append(errors, fmt.Errorf("handler %T failed: %w", handler, err))
			}
		}
	}

	// Return combined errors if any
	if len(errors) > 0 {
		return fmt.Errorf("event handling failed: %v", errors)
	}

	return nil
}

// GetHandlers returns all handlers for an event type
func (r *EventHandlerRegistry) GetHandlers(eventType domain.EventType) []domain.EventHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers, exists := r.handlers[eventType]
	if !exists {
		return nil
	}

	// Return a copy to prevent modification
	result := make([]domain.EventHandler, len(handlers))
	copy(result, handlers)
	return result
}

// GetRegisteredEventTypes returns all registered event types
func (r *EventHandlerRegistry) GetRegisteredEventTypes() []domain.EventType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	eventTypes := make([]domain.EventType, 0, len(r.handlers))
	for eventType := range r.handlers {
		eventTypes = append(eventTypes, eventType)
	}

	return eventTypes
}

// Start starts all handlers that implement a Start method
func (r *EventHandlerRegistry) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("event handler registry already started")
	}

	// Start all handlers that have a Start method
	for _, handlerSlice := range r.handlers {
		for _, handler := range handlerSlice {
			if starter, ok := handler.(interface{ Start() error }); ok {
				if err := starter.Start(); err != nil {
					return fmt.Errorf("failed to start handler %T: %w", handler, err)
				}
			}
		}
	}

	r.started = true
	r.logger.Info("Event handler registry started")
	return nil
}

// Stop stops all handlers that implement a Stop method
func (r *EventHandlerRegistry) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return fmt.Errorf("event handler registry not started")
	}

	// Stop all handlers that have a Stop method
	var errors []error
	for _, handlerSlice := range r.handlers {
		for _, handler := range handlerSlice {
			if stopper, ok := handler.(interface{ Stop() error }); ok {
				if err := stopper.Stop(); err != nil {
					errors = append(errors, fmt.Errorf("failed to stop handler %T: %w", handler, err))
				}
			}
		}
	}

	r.started = false
	r.logger.Info("Event handler registry stopped")

	// Return combined errors if any
	if len(errors) > 0 {
		return fmt.Errorf("errors stopping handlers: %v", errors)
	}

	return nil
}

// GetMetrics returns metrics for all handlers that support metrics
func (r *EventHandlerRegistry) GetMetrics() map[string]*EventHandlerMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := make(map[string]*EventHandlerMetrics)

	for _, handlerSlice := range r.handlers {
		for _, handler := range handlerSlice {
			if metricsProvider, ok := handler.(interface{ GetMetrics() *EventHandlerMetrics }); ok {
				handlerName := fmt.Sprintf("%T", handler)
				metrics[handlerName] = metricsProvider.GetMetrics()
			}
		}
	}

	return metrics
}
