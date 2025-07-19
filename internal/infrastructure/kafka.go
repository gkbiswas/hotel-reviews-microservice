package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers              []string      `mapstructure:"brokers" json:"brokers"`
	ReviewTopic          string        `mapstructure:"review_topic" json:"review_topic"`
	ProcessingTopic      string        `mapstructure:"processing_topic" json:"processing_topic"`
	DeadLetterTopic      string        `mapstructure:"dead_letter_topic" json:"dead_letter_topic"`
	ConsumerGroup        string        `mapstructure:"consumer_group" json:"consumer_group"`
	BatchSize            int           `mapstructure:"batch_size" json:"batch_size"`
	BatchTimeout         time.Duration `mapstructure:"batch_timeout" json:"batch_timeout"`
	MaxRetries           int           `mapstructure:"max_retries" json:"max_retries"`
	RetryDelay           time.Duration `mapstructure:"retry_delay" json:"retry_delay"`
	EnableSASL           bool          `mapstructure:"enable_sasl" json:"enable_sasl"`
	SASLUsername         string        `mapstructure:"sasl_username" json:"sasl_username"`
	SASLPassword         string        `mapstructure:"sasl_password" json:"sasl_password"`
	EnableTLS            bool          `mapstructure:"enable_tls" json:"enable_tls"`
	MaxMessageSize       int           `mapstructure:"max_message_size" json:"max_message_size"`
	CompressionType      string        `mapstructure:"compression_type" json:"compression_type"`
	ProducerFlushTimeout time.Duration `mapstructure:"producer_flush_timeout" json:"producer_flush_timeout"`
	ConsumerTimeout      time.Duration `mapstructure:"consumer_timeout" json:"consumer_timeout"`
	EnableIdempotence    bool          `mapstructure:"enable_idempotence" json:"enable_idempotence"`
	Partitions           int           `mapstructure:"partitions" json:"partitions"`
	ReplicationFactor    int           `mapstructure:"replication_factor" json:"replication_factor"`
}

// EventType represents different types of events
type EventType string

const (
	// Processing Events
	ProcessingStartedEvent   EventType = "processing.started"
	ProcessingProgressEvent  EventType = "processing.progress"
	ProcessingCompletedEvent EventType = "processing.completed"
	ProcessingFailedEvent    EventType = "processing.failed"
	ProcessingRetryEvent     EventType = "processing.retry"
	ProcessingCancelledEvent EventType = "processing.cancelled"

	// Review Events
	ReviewCreatedEvent    EventType = "review.created"
	ReviewUpdatedEvent    EventType = "review.updated"
	ReviewDeletedEvent    EventType = "review.deleted"
	ReviewValidatedEvent  EventType = "review.validated"
	ReviewEnrichedEvent   EventType = "review.enriched"
	ReviewBatchCreated    EventType = "review.batch.created"
	ReviewBatchFailed     EventType = "review.batch.failed"

	// Hotel Events
	HotelCreatedEvent       EventType = "hotel.created"
	HotelUpdatedEvent       EventType = "hotel.updated"
	HotelSummaryUpdated     EventType = "hotel.summary.updated"
	HotelAnalyticsUpdated   EventType = "hotel.analytics.updated"

	// Provider Events
	ProviderConnectedEvent    EventType = "provider.connected"
	ProviderDisconnectedEvent EventType = "provider.disconnected"
	ProviderErrorEvent        EventType = "provider.error"

	// System Events
	SystemHealthCheckEvent EventType = "system.health.check"
	SystemErrorEvent       EventType = "system.error"
	SystemMetricsEvent     EventType = "system.metrics"
)

// BaseEvent represents the base structure for all events
type BaseEvent struct {
	ID          string                 `json:"id"`
	Type        EventType              `json:"type"`
	Source      string                 `json:"source"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     string                 `json:"version"`
	CorrelationID string               `json:"correlation_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
	Data        map[string]interface{} `json:"data"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProcessingEvent represents file processing events
type ProcessingEvent struct {
	BaseEvent
	JobID            uuid.UUID `json:"job_id"`
	ProviderID       uuid.UUID `json:"provider_id"`
	FileURL          string    `json:"file_url"`
	Status           string    `json:"status"`
	RecordsTotal     int64     `json:"records_total"`
	RecordsProcessed int64     `json:"records_processed"`
	ErrorCount       int64     `json:"error_count"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	ProcessingTime   int64     `json:"processing_time_ms"`
	RetryCount       int       `json:"retry_count"`
}

// ReviewEvent represents review-related events
type ReviewEvent struct {
	BaseEvent
	ReviewID   uuid.UUID `json:"review_id"`
	HotelID    uuid.UUID `json:"hotel_id"`
	ProviderID uuid.UUID `json:"provider_id"`
	Rating     float64   `json:"rating"`
	Sentiment  string    `json:"sentiment,omitempty"`
	Language   string    `json:"language,omitempty"`
	BatchID    string    `json:"batch_id,omitempty"`
	BatchSize  int       `json:"batch_size,omitempty"`
}

// HotelEvent represents hotel-related events
type HotelEvent struct {
	BaseEvent
	HotelID         uuid.UUID `json:"hotel_id"`
	HotelName       string    `json:"hotel_name"`
	City            string    `json:"city,omitempty"`
	Country         string    `json:"country,omitempty"`
	TotalReviews    int       `json:"total_reviews,omitempty"`
	AverageRating   float64   `json:"average_rating,omitempty"`
	PreviousRating  float64   `json:"previous_rating,omitempty"`
	RatingChange    float64   `json:"rating_change,omitempty"`
}

// ProviderEvent represents provider-related events
type ProviderEvent struct {
	BaseEvent
	ProviderID   uuid.UUID `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	LastSync     time.Time `json:"last_sync,omitempty"`
}

// SystemEvent represents system-level events
type SystemEvent struct {
	BaseEvent
	Component     string                 `json:"component"`
	Status        string                 `json:"status"`
	Message       string                 `json:"message,omitempty"`
	Metrics       map[string]interface{} `json:"metrics,omitempty"`
	ErrorDetails  map[string]interface{} `json:"error_details,omitempty"`
}

// KafkaProducer handles publishing events to Kafka
type KafkaProducer struct {
	writer *kafka.Writer
	config *KafkaConfig
	logger *logger.Logger
	mu     sync.RWMutex
}

// NewKafkaProducer creates a new Kafka producer
func NewKafkaProducer(config *KafkaConfig, logger *logger.Logger) (*KafkaProducer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}

	// Note: Compression handling simplified for compatibility

	// Create Kafka writer
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(config.Brokers...),
		BatchSize:              config.BatchSize,
		BatchTimeout:           config.BatchTimeout,
		RequiredAcks:           kafka.RequireAll,
		Async:                  false,
		WriteTimeout:           config.ProducerFlushTimeout,
		ReadTimeout:            config.ConsumerTimeout,
		ErrorLogger:            kafka.LoggerFunc(func(msg string, args ...interface{}) {
			logger.Error("Kafka producer error", "message", fmt.Sprintf(msg, args...))
		}),
	}

	// Configure TLS if enabled
	if config.EnableTLS {
		writer.Transport = &kafka.Transport{
			// TLS: &tls.Config{}, // Configure as needed
		}
	}

	// Configure SASL if enabled
	if config.EnableSASL {
		// Note: SASL configuration simplified for compatibility
		writer.Transport = &kafka.Transport{
			// SASL: mechanism.Plain{
			//     Username: config.SASLUsername,
			//     Password: config.SASLPassword,
			// },
		}
	}

	producer := &KafkaProducer{
		writer: writer,
		config: config,
		logger: logger,
	}

	return producer, nil
}

// PublishEvent publishes an event to the appropriate topic
func (p *KafkaProducer) PublishEvent(ctx context.Context, event interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Serialize event
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Determine topic based on event type
	topic := p.getTopicForEvent(event)
	if topic == "" {
		return fmt.Errorf("unknown event type: %T", event)
	}

	// Create message
	message := kafka.Message{
		Topic: topic,
		Value: eventBytes,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "event-type", Value: []byte(p.getEventType(event))},
			{Key: "source", Value: []byte("hotel-reviews-microservice")},
			{Key: "version", Value: []byte("1.0")},
		},
	}

	// Add partition key if available
	if partitionKey := p.getPartitionKey(event); partitionKey != "" {
		message.Key = []byte(partitionKey)
	}

	// Add correlation ID if available
	if correlationID := p.getCorrelationID(event); correlationID != "" {
		message.Headers = append(message.Headers, kafka.Header{
			Key:   "correlation-id",
			Value: []byte(correlationID),
		})
	}

	// Publish message
	err = p.writer.WriteMessages(ctx, message)
	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to publish event",
			"error", err,
			"topic", topic,
			"event_type", p.getEventType(event),
		)
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.InfoContext(ctx, "Event published successfully",
		"topic", topic,
		"event_type", p.getEventType(event),
		"message_size", len(eventBytes),
	)

	return nil
}

// PublishProcessingEvent publishes a processing event
func (p *KafkaProducer) PublishProcessingEvent(ctx context.Context, event *ProcessingEvent) error {
	return p.PublishEvent(ctx, event)
}

// PublishReviewEvent publishes a review event
func (p *KafkaProducer) PublishReviewEvent(ctx context.Context, event *ReviewEvent) error {
	return p.PublishEvent(ctx, event)
}

// PublishHotelEvent publishes a hotel event
func (p *KafkaProducer) PublishHotelEvent(ctx context.Context, event *HotelEvent) error {
	return p.PublishEvent(ctx, event)
}

// PublishProviderEvent publishes a provider event
func (p *KafkaProducer) PublishProviderEvent(ctx context.Context, event *ProviderEvent) error {
	return p.PublishEvent(ctx, event)
}

// PublishSystemEvent publishes a system event
func (p *KafkaProducer) PublishSystemEvent(ctx context.Context, event *SystemEvent) error {
	return p.PublishEvent(ctx, event)
}

// Close closes the producer
func (p *KafkaProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.writer != nil {
		err := p.writer.Close()
		p.writer = nil
		return err
	}
	return nil
}

// getTopicForEvent determines the appropriate topic for an event
func (p *KafkaProducer) getTopicForEvent(event interface{}) string {
	switch event.(type) {
	case *ProcessingEvent:
		return p.config.ProcessingTopic
	case *ReviewEvent:
		return p.config.ReviewTopic
	case *HotelEvent:
		return p.config.ReviewTopic
	case *ProviderEvent:
		return p.config.ProcessingTopic
	case *SystemEvent:
		return p.config.ProcessingTopic
	default:
		return ""
	}
}

// getEventType extracts the event type from an event
func (p *KafkaProducer) getEventType(event interface{}) string {
	switch e := event.(type) {
	case *ProcessingEvent:
		return string(e.Type)
	case *ReviewEvent:
		return string(e.Type)
	case *HotelEvent:
		return string(e.Type)
	case *ProviderEvent:
		return string(e.Type)
	case *SystemEvent:
		return string(e.Type)
	default:
		return "unknown"
	}
}

// getPartitionKey extracts the partition key from an event
func (p *KafkaProducer) getPartitionKey(event interface{}) string {
	switch e := event.(type) {
	case *ProcessingEvent:
		return e.ProviderID.String()
	case *ReviewEvent:
		return e.HotelID.String()
	case *HotelEvent:
		return e.HotelID.String()
	case *ProviderEvent:
		return e.ProviderID.String()
	case *SystemEvent:
		return e.Component
	default:
		return ""
	}
}

// getCorrelationID extracts the correlation ID from an event
func (p *KafkaProducer) getCorrelationID(event interface{}) string {
	switch e := event.(type) {
	case *ProcessingEvent:
		return e.CorrelationID
	case *ReviewEvent:
		return e.CorrelationID
	case *HotelEvent:
		return e.CorrelationID
	case *ProviderEvent:
		return e.CorrelationID
	case *SystemEvent:
		return e.CorrelationID
	default:
		return ""
	}
}

// EventHandler defines the interface for handling events
type EventHandler interface {
	Handle(ctx context.Context, event interface{}) error
	CanHandle(eventType EventType) bool
}

// KafkaConsumer handles consuming events from Kafka
type KafkaConsumer struct {
	reader   *kafka.Reader
	config   *KafkaConfig
	logger   *logger.Logger
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewKafkaConsumer creates a new Kafka consumer
func NewKafkaConsumer(config *KafkaConfig, logger *logger.Logger) (*KafkaConsumer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}

	if config.ConsumerGroup == "" {
		return nil, fmt.Errorf("consumer group is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.Brokers,
		GroupID:        config.ConsumerGroup,
		Topic:          config.ReviewTopic,
		MaxBytes:       config.MaxMessageSize,
		CommitInterval: 1 * time.Second,
		StartOffset:    kafka.LastOffset,
		ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			logger.Error("Kafka consumer error", "message", fmt.Sprintf(msg, args...))
		}),
	})

	consumer := &KafkaConsumer{
		reader:   reader,
		config:   config,
		logger:   logger,
		handlers: make(map[EventType][]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
	}

	return consumer, nil
}

// RegisterHandler registers an event handler
func (c *KafkaConsumer) RegisterHandler(eventType EventType, handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.handlers[eventType] == nil {
		c.handlers[eventType] = make([]EventHandler, 0)
	}
	c.handlers[eventType] = append(c.handlers[eventType], handler)

	c.logger.Info("Event handler registered",
		"event_type", eventType,
		"handler_count", len(c.handlers[eventType]),
	)
}

// Start starts the consumer
func (c *KafkaConsumer) Start() error {
	c.logger.Info("Starting Kafka consumer",
		"consumer_group", c.config.ConsumerGroup,
		"brokers", c.config.Brokers,
	)

	c.wg.Add(1)
	go c.consumeLoop()

	return nil
}

// Stop stops the consumer
func (c *KafkaConsumer) Stop() error {
	c.logger.Info("Stopping Kafka consumer...")

	c.cancel()
	c.wg.Wait()

	if c.reader != nil {
		err := c.reader.Close()
		c.reader = nil
		return err
	}

	c.logger.Info("Kafka consumer stopped")
	return nil
}

// consumeLoop is the main consumption loop
func (c *KafkaConsumer) consumeLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			message, err := c.reader.ReadMessage(c.ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				c.logger.Error("Failed to read message", "error", err)
				continue
			}

			// Process message
			if err := c.processMessage(c.ctx, message); err != nil {
				c.logger.Error("Failed to process message",
					"error", err,
					"topic", message.Topic,
					"partition", message.Partition,
					"offset", message.Offset,
				)

				// Send to dead letter queue if processing fails
				if err := c.sendToDeadLetterQueue(c.ctx, message, err); err != nil {
					c.logger.Error("Failed to send message to dead letter queue",
						"error", err,
						"original_error", err,
					)
				}
			}
		}
	}
}

// processMessage processes a single message
func (c *KafkaConsumer) processMessage(ctx context.Context, message kafka.Message) error {
	// Extract event type from headers
	eventType := c.getEventTypeFromHeaders(message.Headers)
	if eventType == "" {
		return fmt.Errorf("missing event type in message headers")
	}

	// Deserialize event
	event, err := c.deserializeEvent(message.Value, EventType(eventType))
	if err != nil {
		return fmt.Errorf("failed to deserialize event: %w", err)
	}

	// Get handlers for this event type
	c.mu.RLock()
	handlers, exists := c.handlers[EventType(eventType)]
	c.mu.RUnlock()

	if !exists || len(handlers) == 0 {
		c.logger.Warn("No handlers registered for event type",
			"event_type", eventType,
			"topic", message.Topic,
		)
		return nil
	}

	// Process event with all registered handlers
	for _, handler := range handlers {
		if handler.CanHandle(EventType(eventType)) {
			if err := handler.Handle(ctx, event); err != nil {
				c.logger.Error("Handler failed to process event",
					"error", err,
					"event_type", eventType,
					"handler", fmt.Sprintf("%T", handler),
				)
				return err
			}
		}
	}

	c.logger.Debug("Message processed successfully",
		"event_type", eventType,
		"topic", message.Topic,
		"partition", message.Partition,
		"offset", message.Offset,
		"handlers_count", len(handlers),
	)

	return nil
}

// deserializeEvent deserializes an event based on its type
func (c *KafkaConsumer) deserializeEvent(data []byte, eventType EventType) (interface{}, error) {
	switch {
	case strings.HasPrefix(string(eventType), "processing."):
		var event ProcessingEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		return &event, nil
	case strings.HasPrefix(string(eventType), "review."):
		var event ReviewEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		return &event, nil
	case strings.HasPrefix(string(eventType), "hotel."):
		var event HotelEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		return &event, nil
	case strings.HasPrefix(string(eventType), "provider."):
		var event ProviderEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		return &event, nil
	case strings.HasPrefix(string(eventType), "system."):
		var event SystemEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		return &event, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}

// getEventTypeFromHeaders extracts the event type from message headers
func (c *KafkaConsumer) getEventTypeFromHeaders(headers []kafka.Header) string {
	for _, header := range headers {
		if header.Key == "event-type" {
			return string(header.Value)
		}
	}
	return ""
}

// sendToDeadLetterQueue sends a failed message to the dead letter queue
func (c *KafkaConsumer) sendToDeadLetterQueue(ctx context.Context, message kafka.Message, processingError error) error {
	if c.config.DeadLetterTopic == "" {
		return nil // No dead letter queue configured
	}

	// Create dead letter message
	deadLetterMessage := kafka.Message{
		Topic: c.config.DeadLetterTopic,
		Key:   message.Key,
		Value: message.Value,
		Time:  time.Now(),
		Headers: append(message.Headers, kafka.Header{
			Key:   "processing-error",
			Value: []byte(processingError.Error()),
		}, kafka.Header{
			Key:   "original-topic",
			Value: []byte(message.Topic),
		}, kafka.Header{
			Key:   "original-partition",
			Value: []byte(fmt.Sprintf("%d", message.Partition)),
		}, kafka.Header{
			Key:   "original-offset",
			Value: []byte(fmt.Sprintf("%d", message.Offset)),
		}),
	}

	// Create a temporary writer for dead letter queue
	writer := &kafka.Writer{
		Addr:         kafka.TCP(c.config.Brokers...),
		Topic:        c.config.DeadLetterTopic,
		BatchSize:    1,
		BatchTimeout: 100 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
	defer writer.Close()

	return writer.WriteMessages(ctx, deadLetterMessage)
}

// EventBuilders provide convenient ways to create events

// NewProcessingEvent creates a new processing event
func NewProcessingEvent(eventType EventType, jobID, providerID uuid.UUID, fileURL string) *ProcessingEvent {
	return &ProcessingEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.New().String(),
			Type:      eventType,
			Source:    "hotel-reviews-processing",
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		JobID:      jobID,
		ProviderID: providerID,
		FileURL:    fileURL,
	}
}

// NewReviewEvent creates a new review event
func NewReviewEvent(eventType EventType, reviewID, hotelID, providerID uuid.UUID) *ReviewEvent {
	return &ReviewEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.New().String(),
			Type:      eventType,
			Source:    "hotel-reviews-service",
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		ReviewID:   reviewID,
		HotelID:    hotelID,
		ProviderID: providerID,
	}
}

// NewHotelEvent creates a new hotel event
func NewHotelEvent(eventType EventType, hotelID uuid.UUID, hotelName string) *HotelEvent {
	return &HotelEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.New().String(),
			Type:      eventType,
			Source:    "hotel-reviews-service",
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		HotelID:   hotelID,
		HotelName: hotelName,
	}
}

// NewProviderEvent creates a new provider event
func NewProviderEvent(eventType EventType, providerID uuid.UUID, providerName string) *ProviderEvent {
	return &ProviderEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.New().String(),
			Type:      eventType,
			Source:    "hotel-reviews-service",
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		ProviderID:   providerID,
		ProviderName: providerName,
	}
}

// NewSystemEvent creates a new system event
func NewSystemEvent(eventType EventType, component string) *SystemEvent {
	return &SystemEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.New().String(),
			Type:      eventType,
			Source:    "hotel-reviews-system",
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		Component: component,
	}
}

// KafkaEventPublisher implements the domain event publisher interface
type KafkaEventPublisher struct {
	producer *KafkaProducer
	logger   *logger.Logger
}

// NewKafkaEventPublisher creates a new Kafka event publisher
func NewKafkaEventPublisher(producer *KafkaProducer, logger *logger.Logger) *KafkaEventPublisher {
	return &KafkaEventPublisher{
		producer: producer,
		logger:   logger,
	}
}

// PublishDomainEvent publishes a domain event
func (p *KafkaEventPublisher) PublishDomainEvent(ctx context.Context, event interface{}) error {
	return p.producer.PublishEvent(ctx, event)
}

// ProcessingEventHandler handles processing events
type ProcessingEventHandler struct {
	reviewService domain.ReviewService
	logger        *logger.Logger
}

// NewProcessingEventHandler creates a new processing event handler
func NewProcessingEventHandler(reviewService domain.ReviewService, logger *logger.Logger) *ProcessingEventHandler {
	return &ProcessingEventHandler{
		reviewService: reviewService,
		logger:        logger,
	}
}

// Handle handles a processing event
func (h *ProcessingEventHandler) Handle(ctx context.Context, event interface{}) error {
	processingEvent, ok := event.(*ProcessingEvent)
	if !ok {
		return fmt.Errorf("expected ProcessingEvent, got %T", event)
	}

	h.logger.InfoContext(ctx, "Processing event received",
		"event_type", processingEvent.Type,
		"job_id", processingEvent.JobID,
		"provider_id", processingEvent.ProviderID,
		"status", processingEvent.Status,
	)

	// Update processing status in database
	switch processingEvent.Type {
	case ProcessingStartedEvent:
		return h.handleProcessingStarted(ctx, processingEvent)
	case ProcessingProgressEvent:
		return h.handleProcessingProgress(ctx, processingEvent)
	case ProcessingCompletedEvent:
		return h.handleProcessingCompleted(ctx, processingEvent)
	case ProcessingFailedEvent:
		return h.handleProcessingFailed(ctx, processingEvent)
	default:
		h.logger.WarnContext(ctx, "Unknown processing event type",
			"event_type", processingEvent.Type,
		)
		return nil
	}
}

// CanHandle checks if this handler can handle the given event type
func (h *ProcessingEventHandler) CanHandle(eventType EventType) bool {
	return strings.HasPrefix(string(eventType), "processing.")
}

// handleProcessingStarted handles processing started events
func (h *ProcessingEventHandler) handleProcessingStarted(ctx context.Context, event *ProcessingEvent) error {
	// Update processing status to started
	// This would typically involve updating the database
	h.logger.InfoContext(ctx, "Processing started",
		"job_id", event.JobID,
		"provider_id", event.ProviderID,
		"file_url", event.FileURL,
	)
	return nil
}

// handleProcessingProgress handles processing progress events
func (h *ProcessingEventHandler) handleProcessingProgress(ctx context.Context, event *ProcessingEvent) error {
	// Update processing progress
	h.logger.InfoContext(ctx, "Processing progress",
		"job_id", event.JobID,
		"records_processed", event.RecordsProcessed,
		"records_total", event.RecordsTotal,
		"progress", fmt.Sprintf("%.2f%%", float64(event.RecordsProcessed)/float64(event.RecordsTotal)*100),
	)
	return nil
}

// handleProcessingCompleted handles processing completed events
func (h *ProcessingEventHandler) handleProcessingCompleted(ctx context.Context, event *ProcessingEvent) error {
	// Update processing status to completed
	h.logger.InfoContext(ctx, "Processing completed",
		"job_id", event.JobID,
		"provider_id", event.ProviderID,
		"records_processed", event.RecordsProcessed,
		"processing_time", time.Duration(event.ProcessingTime)*time.Millisecond,
	)
	return nil
}

// handleProcessingFailed handles processing failed events
func (h *ProcessingEventHandler) handleProcessingFailed(ctx context.Context, event *ProcessingEvent) error {
	// Update processing status to failed
	h.logger.ErrorContext(ctx, "Processing failed",
		"job_id", event.JobID,
		"provider_id", event.ProviderID,
		"error_message", event.ErrorMessage,
		"retry_count", event.RetryCount,
	)
	return nil
}

// ReviewEventHandler handles review events
type ReviewEventHandler struct {
	reviewService domain.ReviewService
	logger        *logger.Logger
}

// NewReviewEventHandler creates a new review event handler
func NewReviewEventHandler(reviewService domain.ReviewService, logger *logger.Logger) *ReviewEventHandler {
	return &ReviewEventHandler{
		reviewService: reviewService,
		logger:        logger,
	}
}

// Handle handles a review event
func (h *ReviewEventHandler) Handle(ctx context.Context, event interface{}) error {
	reviewEvent, ok := event.(*ReviewEvent)
	if !ok {
		return fmt.Errorf("expected ReviewEvent, got %T", event)
	}

	h.logger.InfoContext(ctx, "Review event received",
		"event_type", reviewEvent.Type,
		"review_id", reviewEvent.ReviewID,
		"hotel_id", reviewEvent.HotelID,
		"provider_id", reviewEvent.ProviderID,
	)

	// Handle different review events
	switch reviewEvent.Type {
	case ReviewCreatedEvent:
		return h.handleReviewCreated(ctx, reviewEvent)
	case ReviewBatchCreated:
		return h.handleReviewBatchCreated(ctx, reviewEvent)
	case ReviewUpdatedEvent:
		return h.handleReviewUpdated(ctx, reviewEvent)
	case ReviewDeletedEvent:
		return h.handleReviewDeleted(ctx, reviewEvent)
	default:
		h.logger.WarnContext(ctx, "Unknown review event type",
			"event_type", reviewEvent.Type,
		)
		return nil
	}
}

// CanHandle checks if this handler can handle the given event type
func (h *ReviewEventHandler) CanHandle(eventType EventType) bool {
	return strings.HasPrefix(string(eventType), "review.")
}

// handleReviewCreated handles review created events
func (h *ReviewEventHandler) handleReviewCreated(ctx context.Context, event *ReviewEvent) error {
	// Trigger analytics update, cache invalidation, etc.
	h.logger.InfoContext(ctx, "Review created",
		"review_id", event.ReviewID,
		"hotel_id", event.HotelID,
		"rating", event.Rating,
		"sentiment", event.Sentiment,
	)
	return nil
}

// handleReviewBatchCreated handles review batch created events
func (h *ReviewEventHandler) handleReviewBatchCreated(ctx context.Context, event *ReviewEvent) error {
	// Handle batch processing completion
	h.logger.InfoContext(ctx, "Review batch created",
		"batch_id", event.BatchID,
		"batch_size", event.BatchSize,
		"hotel_id", event.HotelID,
		"provider_id", event.ProviderID,
	)
	return nil
}

// handleReviewUpdated handles review updated events
func (h *ReviewEventHandler) handleReviewUpdated(ctx context.Context, event *ReviewEvent) error {
	// Handle review updates
	h.logger.InfoContext(ctx, "Review updated",
		"review_id", event.ReviewID,
		"hotel_id", event.HotelID,
	)
	return nil
}

// handleReviewDeleted handles review deleted events
func (h *ReviewEventHandler) handleReviewDeleted(ctx context.Context, event *ReviewEvent) error {
	// Handle review deletion
	h.logger.InfoContext(ctx, "Review deleted",
		"review_id", event.ReviewID,
		"hotel_id", event.HotelID,
	)
	return nil
}

// RetryableEventHandler wraps an event handler with retry logic
type RetryableEventHandler struct {
	handler    EventHandler
	maxRetries int
	retryDelay time.Duration
	logger     *logger.Logger
}

// NewRetryableEventHandler creates a new retryable event handler
func NewRetryableEventHandler(handler EventHandler, maxRetries int, retryDelay time.Duration, logger *logger.Logger) *RetryableEventHandler {
	return &RetryableEventHandler{
		handler:    handler,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		logger:     logger,
	}
}

// Handle handles an event with retry logic
func (h *RetryableEventHandler) Handle(ctx context.Context, event interface{}) error {
	var lastErr error
	
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		if attempt > 0 {
			h.logger.InfoContext(ctx, "Retrying event handling",
				"attempt", attempt,
				"max_retries", h.maxRetries,
				"event_type", fmt.Sprintf("%T", event),
			)
			time.Sleep(h.retryDelay)
		}
		
		if err := h.handler.Handle(ctx, event); err != nil {
			lastErr = err
			h.logger.WarnContext(ctx, "Event handling failed",
				"error", err,
				"attempt", attempt,
				"max_retries", h.maxRetries,
			)
			continue
		}
		
		return nil
	}
	
	return fmt.Errorf("event handling failed after %d attempts: %w", h.maxRetries, lastErr)
}

// CanHandle checks if this handler can handle the given event type
func (h *RetryableEventHandler) CanHandle(eventType EventType) bool {
	return h.handler.CanHandle(eventType)
}