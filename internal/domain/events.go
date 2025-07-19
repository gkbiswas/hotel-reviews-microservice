package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EventVersion represents the version of an event schema
type EventVersion string

const (
	// Event schema versions
	EventVersionV1 EventVersion = "1.0"
	EventVersionV2 EventVersion = "2.0"
	EventVersionV3 EventVersion = "3.0"
)

// EventType represents the type of domain event
type EventType string

const (
	// File Processing Events
	FileProcessingStartedEventType   EventType = "FileProcessingStarted"
	FileProcessingProgressEventType  EventType = "FileProcessingProgress"
	FileProcessingCompletedEventType EventType = "FileProcessingCompleted"
	FileProcessingFailedEventType    EventType = "FileProcessingFailed"
	FileProcessingCancelledEventType EventType = "FileProcessingCancelled"
	FileProcessingRetryEventType     EventType = "FileProcessingRetry"

	// Review Processing Events
	ReviewProcessedEventType         EventType = "ReviewProcessed"
	ReviewValidatedEventType         EventType = "ReviewValidated"
	ReviewEnrichedEventType          EventType = "ReviewEnriched"
	ReviewBatchProcessedEventType    EventType = "ReviewBatchProcessed"
	ReviewProcessingFailedEventType  EventType = "ReviewProcessingFailed"
	ReviewDuplicateDetectedEventType EventType = "ReviewDuplicateDetected"

	// Hotel Events
	HotelCreatedEventType          EventType = "HotelCreated"
	HotelUpdatedEventType          EventType = "HotelUpdated"
	HotelSummaryUpdatedEventType   EventType = "HotelSummaryUpdated"
	HotelAnalyticsUpdatedEventType EventType = "HotelAnalyticsUpdated"

	// Provider Events
	ProviderActivatedEventType   EventType = "ProviderActivated"
	ProviderDeactivatedEventType EventType = "ProviderDeactivated"
	ProviderErrorEvent           EventType = "ProviderError"
	ProviderSyncedEvent          EventType = "ProviderSynced"

	// System Events
	SystemHealthCheckEvent EventType = "SystemHealthCheck"
	SystemErrorEvent       EventType = "SystemError"
	SystemMetricsEvent     EventType = "SystemMetrics"
)

// EventStatus represents the status of an event
type EventStatus string

const (
	EventStatusPublished  EventStatus = "published"
	EventStatusProcessing EventStatus = "processing"
	EventStatusProcessed  EventStatus = "processed"
	EventStatusFailed     EventStatus = "failed"
	EventStatusRetry      EventStatus = "retry"
)

// DomainEvent represents the base interface for all domain events
type DomainEvent interface {
	// GetID returns the unique identifier of the event
	GetID() uuid.UUID

	// GetType returns the type of the event
	GetType() EventType

	// GetVersion returns the schema version of the event
	GetVersion() EventVersion

	// GetAggregateID returns the ID of the aggregate that generated this event
	GetAggregateID() uuid.UUID

	// GetAggregateType returns the type of the aggregate
	GetAggregateType() string

	// GetOccurredAt returns when the event occurred
	GetOccurredAt() time.Time

	// GetSequenceNumber returns the sequence number of this event for the aggregate
	GetSequenceNumber() int64

	// GetCorrelationID returns the correlation ID for tracing
	GetCorrelationID() string

	// GetCausationID returns the ID of the event that caused this event
	GetCausationID() string

	// GetMetadata returns additional metadata
	GetMetadata() map[string]interface{}

	// GetPayload returns the event payload
	GetPayload() interface{}

	// Validate validates the event
	Validate() error

	// Serialize serializes the event to JSON
	Serialize() ([]byte, error)

	// IsReplayable returns true if the event can be replayed
	IsReplayable() bool
}

// BaseEvent provides common functionality for all domain events
type BaseEvent struct {
	ID             uuid.UUID              `json:"id"`
	Type           EventType              `json:"type"`
	Version        EventVersion           `json:"version"`
	AggregateID    uuid.UUID              `json:"aggregate_id"`
	AggregateType  string                 `json:"aggregate_type"`
	OccurredAt     time.Time              `json:"occurred_at"`
	SequenceNumber int64                  `json:"sequence_number"`
	CorrelationID  string                 `json:"correlation_id,omitempty"`
	CausationID    string                 `json:"causation_id,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Payload        interface{}            `json:"payload"`
	IsReplay       bool                   `json:"is_replay"`
}

// GetID returns the unique identifier of the event
func (e *BaseEvent) GetID() uuid.UUID {
	return e.ID
}

// GetType returns the type of the event
func (e *BaseEvent) GetType() EventType {
	return e.Type
}

// GetVersion returns the schema version of the event
func (e *BaseEvent) GetVersion() EventVersion {
	return e.Version
}

// GetAggregateID returns the ID of the aggregate that generated this event
func (e *BaseEvent) GetAggregateID() uuid.UUID {
	return e.AggregateID
}

// GetAggregateType returns the type of the aggregate
func (e *BaseEvent) GetAggregateType() string {
	return e.AggregateType
}

// GetOccurredAt returns when the event occurred
func (e *BaseEvent) GetOccurredAt() time.Time {
	return e.OccurredAt
}

// GetSequenceNumber returns the sequence number of this event for the aggregate
func (e *BaseEvent) GetSequenceNumber() int64 {
	return e.SequenceNumber
}

// GetCorrelationID returns the correlation ID for tracing
func (e *BaseEvent) GetCorrelationID() string {
	return e.CorrelationID
}

// GetCausationID returns the ID of the event that caused this event
func (e *BaseEvent) GetCausationID() string {
	return e.CausationID
}

// GetMetadata returns additional metadata
func (e *BaseEvent) GetMetadata() map[string]interface{} {
	return e.Metadata
}

// GetPayload returns the event payload
func (e *BaseEvent) GetPayload() interface{} {
	return e.Payload
}

// Validate validates the base event
func (e *BaseEvent) Validate() error {
	if e.ID == uuid.Nil {
		return fmt.Errorf("event ID cannot be nil")
	}
	if e.Type == "" {
		return fmt.Errorf("event type cannot be empty")
	}
	if e.Version == "" {
		return fmt.Errorf("event version cannot be empty")
	}
	if e.AggregateID == uuid.Nil {
		return fmt.Errorf("aggregate ID cannot be nil")
	}
	if e.AggregateType == "" {
		return fmt.Errorf("aggregate type cannot be empty")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred at cannot be zero")
	}
	if e.SequenceNumber <= 0 {
		return fmt.Errorf("sequence number must be positive")
	}
	return nil
}

// Serialize serializes the event to JSON
func (e *BaseEvent) Serialize() ([]byte, error) {
	return json.Marshal(e)
}

// IsReplayable returns true if the event can be replayed
func (e *BaseEvent) IsReplayable() bool {
	return e.IsReplay
}

// File Processing Events

// FileProcessingStartedEventPayload represents the payload for file processing started event
type FileProcessingStartedEventPayload struct {
	JobID            uuid.UUID              `json:"job_id"`
	ProviderID       uuid.UUID              `json:"provider_id"`
	ProviderName     string                 `json:"provider_name"`
	FileURL          string                 `json:"file_url"`
	FileName         string                 `json:"file_name"`
	FileSize         int64                  `json:"file_size"`
	EstimatedRecords int64                  `json:"estimated_records"`
	ProcessingConfig map[string]interface{} `json:"processing_config"`
	StartedBy        string                 `json:"started_by"`
	Priority         int                    `json:"priority"`
	TimeoutAt        time.Time              `json:"timeout_at"`
}

// FileProcessingStartedEvent represents a file processing started event
type FileProcessingStartedEvent struct {
	BaseEvent
	JobID            uuid.UUID              `json:"job_id"`
	ProviderID       uuid.UUID              `json:"provider_id"`
	ProviderName     string                 `json:"provider_name"`
	FileURL          string                 `json:"file_url"`
	FileName         string                 `json:"file_name"`
	FileSize         int64                  `json:"file_size"`
	EstimatedRecords int64                  `json:"estimated_records"`
	ProcessingConfig map[string]interface{} `json:"processing_config"`
	StartedBy        string                 `json:"started_by"`
	Priority         int                    `json:"priority"`
	TimeoutAt        time.Time              `json:"timeout_at"`
}

// NewFileProcessingStartedEvent creates a new file processing started event
func NewFileProcessingStartedEvent(
	aggregateID uuid.UUID,
	sequenceNumber int64,
	payload *FileProcessingStartedEventPayload,
	correlationID string,
) *FileProcessingStartedEvent {
	return &FileProcessingStartedEvent{
		BaseEvent: BaseEvent{
			ID:             uuid.New(),
			Type:           FileProcessingStartedEventType,
			Version:        EventVersionV1,
			AggregateID:    aggregateID,
			AggregateType:  "FileProcessingJob",
			OccurredAt:     time.Now(),
			SequenceNumber: sequenceNumber,
			CorrelationID:  correlationID,
			Metadata:       make(map[string]interface{}),
			Payload:        payload,
		},
		JobID:            payload.JobID,
		ProviderID:       payload.ProviderID,
		ProviderName:     payload.ProviderName,
		FileURL:          payload.FileURL,
		FileName:         payload.FileName,
		FileSize:         payload.FileSize,
		EstimatedRecords: payload.EstimatedRecords,
		ProcessingConfig: payload.ProcessingConfig,
		StartedBy:        payload.StartedBy,
		Priority:         payload.Priority,
		TimeoutAt:        payload.TimeoutAt,
	}
}

// Validate validates the file processing started event
func (e *FileProcessingStartedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	if e.JobID == uuid.Nil {
		return fmt.Errorf("job ID cannot be nil")
	}
	if e.ProviderID == uuid.Nil {
		return fmt.Errorf("provider ID cannot be nil")
	}
	if e.FileURL == "" {
		return fmt.Errorf("file URL cannot be empty")
	}
	if e.FileSize < 0 {
		return fmt.Errorf("file size cannot be negative")
	}
	return nil
}

// FileProcessingProgressEventPayload represents the payload for file processing progress event
type FileProcessingProgressEventPayload struct {
	JobID            uuid.UUID `json:"job_id"`
	RecordsProcessed int64     `json:"records_processed"`
	RecordsTotal     int64     `json:"records_total"`
	RecordsFailed    int64     `json:"records_failed"`
	RecordsSkipped   int64     `json:"records_skipped"`
	CurrentBatch     int       `json:"current_batch"`
	TotalBatches     int       `json:"total_batches"`
	ProcessingRate   float64   `json:"processing_rate"`
	EstimatedETA     time.Time `json:"estimated_eta"`
	LastProcessedAt  time.Time `json:"last_processed_at"`
}

// FileProcessingProgressEvent represents a file processing progress event
type FileProcessingProgressEvent struct {
	BaseEvent
	JobID            uuid.UUID `json:"job_id"`
	RecordsProcessed int64     `json:"records_processed"`
	RecordsTotal     int64     `json:"records_total"`
	RecordsFailed    int64     `json:"records_failed"`
	RecordsSkipped   int64     `json:"records_skipped"`
	CurrentBatch     int       `json:"current_batch"`
	TotalBatches     int       `json:"total_batches"`
	ProcessingRate   float64   `json:"processing_rate"`
	EstimatedETA     time.Time `json:"estimated_eta"`
	LastProcessedAt  time.Time `json:"last_processed_at"`
}

// NewFileProcessingProgressEvent creates a new file processing progress event
func NewFileProcessingProgressEvent(
	aggregateID uuid.UUID,
	sequenceNumber int64,
	payload *FileProcessingProgressEventPayload,
	correlationID string,
) *FileProcessingProgressEvent {
	return &FileProcessingProgressEvent{
		BaseEvent: BaseEvent{
			ID:             uuid.New(),
			Type:           FileProcessingProgressEventType,
			Version:        EventVersionV1,
			AggregateID:    aggregateID,
			AggregateType:  "FileProcessingJob",
			OccurredAt:     time.Now(),
			SequenceNumber: sequenceNumber,
			CorrelationID:  correlationID,
			Metadata:       make(map[string]interface{}),
			Payload:        payload,
		},
		JobID:            payload.JobID,
		RecordsProcessed: payload.RecordsProcessed,
		RecordsTotal:     payload.RecordsTotal,
		RecordsFailed:    payload.RecordsFailed,
		RecordsSkipped:   payload.RecordsSkipped,
		CurrentBatch:     payload.CurrentBatch,
		TotalBatches:     payload.TotalBatches,
		ProcessingRate:   payload.ProcessingRate,
		EstimatedETA:     payload.EstimatedETA,
		LastProcessedAt:  payload.LastProcessedAt,
	}
}

// FileProcessingCompletedEventPayload represents the payload for file processing completed event
type FileProcessingCompletedEventPayload struct {
	JobID              uuid.UUID              `json:"job_id"`
	RecordsProcessed   int64                  `json:"records_processed"`
	RecordsTotal       int64                  `json:"records_total"`
	RecordsFailed      int64                  `json:"records_failed"`
	RecordsSkipped     int64                  `json:"records_skipped"`
	ProcessingDuration time.Duration          `json:"processing_duration"`
	ProcessingRate     float64                `json:"processing_rate"`
	CompletedAt        time.Time              `json:"completed_at"`
	Summary            map[string]interface{} `json:"summary"`
	Metrics            map[string]interface{} `json:"metrics"`
}

// FileProcessingCompletedEvent represents a file processing completed event
type FileProcessingCompletedEvent struct {
	BaseEvent
	JobID              uuid.UUID              `json:"job_id"`
	RecordsProcessed   int64                  `json:"records_processed"`
	RecordsTotal       int64                  `json:"records_total"`
	RecordsFailed      int64                  `json:"records_failed"`
	RecordsSkipped     int64                  `json:"records_skipped"`
	ProcessingDuration time.Duration          `json:"processing_duration"`
	ProcessingRate     float64                `json:"processing_rate"`
	CompletedAt        time.Time              `json:"completed_at"`
	Summary            map[string]interface{} `json:"summary"`
	Metrics            map[string]interface{} `json:"metrics"`
}

// NewFileProcessingCompletedEvent creates a new file processing completed event
func NewFileProcessingCompletedEvent(
	aggregateID uuid.UUID,
	sequenceNumber int64,
	payload *FileProcessingCompletedEventPayload,
	correlationID string,
) *FileProcessingCompletedEvent {
	return &FileProcessingCompletedEvent{
		BaseEvent: BaseEvent{
			ID:             uuid.New(),
			Type:           FileProcessingCompletedEventType,
			Version:        EventVersionV1,
			AggregateID:    aggregateID,
			AggregateType:  "FileProcessingJob",
			OccurredAt:     time.Now(),
			SequenceNumber: sequenceNumber,
			CorrelationID:  correlationID,
			Metadata:       make(map[string]interface{}),
			Payload:        payload,
		},
		JobID:              payload.JobID,
		RecordsProcessed:   payload.RecordsProcessed,
		RecordsTotal:       payload.RecordsTotal,
		RecordsFailed:      payload.RecordsFailed,
		RecordsSkipped:     payload.RecordsSkipped,
		ProcessingDuration: payload.ProcessingDuration,
		ProcessingRate:     payload.ProcessingRate,
		CompletedAt:        payload.CompletedAt,
		Summary:            payload.Summary,
		Metrics:            payload.Metrics,
	}
}

// FileProcessingFailedEventPayload represents the payload for file processing failed event
type FileProcessingFailedEventPayload struct {
	JobID              uuid.UUID              `json:"job_id"`
	ErrorMessage       string                 `json:"error_message"`
	ErrorCode          string                 `json:"error_code"`
	ErrorType          string                 `json:"error_type"`
	RecordsProcessed   int64                  `json:"records_processed"`
	RecordsTotal       int64                  `json:"records_total"`
	ProcessingDuration time.Duration          `json:"processing_duration"`
	FailedAt           time.Time              `json:"failed_at"`
	RetryCount         int                    `json:"retry_count"`
	MaxRetries         int                    `json:"max_retries"`
	IsRetryable        bool                   `json:"is_retryable"`
	StackTrace         string                 `json:"stack_trace,omitempty"`
	Context            map[string]interface{} `json:"context,omitempty"`
}

// FileProcessingFailedEvent represents a file processing failed event
type FileProcessingFailedEvent struct {
	BaseEvent
	JobID              uuid.UUID              `json:"job_id"`
	ErrorMessage       string                 `json:"error_message"`
	ErrorCode          string                 `json:"error_code"`
	ErrorType          string                 `json:"error_type"`
	RecordsProcessed   int64                  `json:"records_processed"`
	RecordsTotal       int64                  `json:"records_total"`
	ProcessingDuration time.Duration          `json:"processing_duration"`
	FailedAt           time.Time              `json:"failed_at"`
	RetryCount         int                    `json:"retry_count"`
	MaxRetries         int                    `json:"max_retries"`
	IsRetryable        bool                   `json:"is_retryable"`
	StackTrace         string                 `json:"stack_trace,omitempty"`
	Context            map[string]interface{} `json:"context,omitempty"`
}

// NewFileProcessingFailedEvent creates a new file processing failed event
func NewFileProcessingFailedEvent(
	aggregateID uuid.UUID,
	sequenceNumber int64,
	payload *FileProcessingFailedEventPayload,
	correlationID string,
) *FileProcessingFailedEvent {
	return &FileProcessingFailedEvent{
		BaseEvent: BaseEvent{
			ID:             uuid.New(),
			Type:           FileProcessingFailedEventType,
			Version:        EventVersionV1,
			AggregateID:    aggregateID,
			AggregateType:  "FileProcessingJob",
			OccurredAt:     time.Now(),
			SequenceNumber: sequenceNumber,
			CorrelationID:  correlationID,
			Metadata:       make(map[string]interface{}),
			Payload:        payload,
		},
		JobID:              payload.JobID,
		ErrorMessage:       payload.ErrorMessage,
		ErrorCode:          payload.ErrorCode,
		ErrorType:          payload.ErrorType,
		RecordsProcessed:   payload.RecordsProcessed,
		RecordsTotal:       payload.RecordsTotal,
		ProcessingDuration: payload.ProcessingDuration,
		FailedAt:           payload.FailedAt,
		RetryCount:         payload.RetryCount,
		MaxRetries:         payload.MaxRetries,
		IsRetryable:        payload.IsRetryable,
		StackTrace:         payload.StackTrace,
		Context:            payload.Context,
	}
}

// Review Processing Events

// ReviewProcessedEventPayload represents the payload for review processed event
type ReviewProcessedEventPayload struct {
	ReviewID         uuid.UUID              `json:"review_id"`
	HotelID          uuid.UUID              `json:"hotel_id"`
	ProviderID       uuid.UUID              `json:"provider_id"`
	ExternalID       string                 `json:"external_id"`
	ProcessingJobID  uuid.UUID              `json:"processing_job_id"`
	BatchID          string                 `json:"batch_id"`
	Rating           float64                `json:"rating"`
	Sentiment        string                 `json:"sentiment"`
	Language         string                 `json:"language"`
	WordCount        int                    `json:"word_count"`
	ProcessingStages []string               `json:"processing_stages"`
	ValidationResult map[string]interface{} `json:"validation_result"`
	EnrichmentData   map[string]interface{} `json:"enrichment_data"`
	ProcessedAt      time.Time              `json:"processed_at"`
}

// ReviewProcessedEvent represents a review processed event
type ReviewProcessedEvent struct {
	BaseEvent
	ReviewID         uuid.UUID              `json:"review_id"`
	HotelID          uuid.UUID              `json:"hotel_id"`
	ProviderID       uuid.UUID              `json:"provider_id"`
	ExternalID       string                 `json:"external_id"`
	ProcessingJobID  uuid.UUID              `json:"processing_job_id"`
	BatchID          string                 `json:"batch_id"`
	Rating           float64                `json:"rating"`
	Sentiment        string                 `json:"sentiment"`
	Language         string                 `json:"language"`
	WordCount        int                    `json:"word_count"`
	ProcessingStages []string               `json:"processing_stages"`
	ValidationResult map[string]interface{} `json:"validation_result"`
	EnrichmentData   map[string]interface{} `json:"enrichment_data"`
	ProcessedAt      time.Time              `json:"processed_at"`
}

// NewReviewProcessedEvent creates a new review processed event
func NewReviewProcessedEvent(
	aggregateID uuid.UUID,
	sequenceNumber int64,
	payload *ReviewProcessedEventPayload,
	correlationID string,
) *ReviewProcessedEvent {
	return &ReviewProcessedEvent{
		BaseEvent: BaseEvent{
			ID:             uuid.New(),
			Type:           ReviewProcessedEventType,
			Version:        EventVersionV1,
			AggregateID:    aggregateID,
			AggregateType:  "Review",
			OccurredAt:     time.Now(),
			SequenceNumber: sequenceNumber,
			CorrelationID:  correlationID,
			Metadata:       make(map[string]interface{}),
			Payload:        payload,
		},
		ReviewID:         payload.ReviewID,
		HotelID:          payload.HotelID,
		ProviderID:       payload.ProviderID,
		ExternalID:       payload.ExternalID,
		ProcessingJobID:  payload.ProcessingJobID,
		BatchID:          payload.BatchID,
		Rating:           payload.Rating,
		Sentiment:        payload.Sentiment,
		Language:         payload.Language,
		WordCount:        payload.WordCount,
		ProcessingStages: payload.ProcessingStages,
		ValidationResult: payload.ValidationResult,
		EnrichmentData:   payload.EnrichmentData,
		ProcessedAt:      payload.ProcessedAt,
	}
}

// ReviewBatchProcessedEventPayload represents the payload for review batch processed event
type ReviewBatchProcessedEventPayload struct {
	BatchID          string                 `json:"batch_id"`
	ProcessingJobID  uuid.UUID              `json:"processing_job_id"`
	ProviderID       uuid.UUID              `json:"provider_id"`
	BatchSize        int                    `json:"batch_size"`
	ReviewsProcessed int                    `json:"reviews_processed"`
	ReviewsFailed    int                    `json:"reviews_failed"`
	ProcessingTime   time.Duration          `json:"processing_time"`
	HotelStats       map[uuid.UUID]int      `json:"hotel_stats"`
	ValidationStats  map[string]int         `json:"validation_stats"`
	EnrichmentStats  map[string]int         `json:"enrichment_stats"`
	ProcessedAt      time.Time              `json:"processed_at"`
	Summary          map[string]interface{} `json:"summary"`
}

// ReviewBatchProcessedEvent represents a review batch processed event
type ReviewBatchProcessedEvent struct {
	BaseEvent
	BatchID          string                 `json:"batch_id"`
	ProcessingJobID  uuid.UUID              `json:"processing_job_id"`
	ProviderID       uuid.UUID              `json:"provider_id"`
	BatchSize        int                    `json:"batch_size"`
	ReviewsProcessed int                    `json:"reviews_processed"`
	ReviewsFailed    int                    `json:"reviews_failed"`
	ProcessingTime   time.Duration          `json:"processing_time"`
	HotelStats       map[uuid.UUID]int      `json:"hotel_stats"`
	ValidationStats  map[string]int         `json:"validation_stats"`
	EnrichmentStats  map[string]int         `json:"enrichment_stats"`
	ProcessedAt      time.Time              `json:"processed_at"`
	Summary          map[string]interface{} `json:"summary"`
}

// NewReviewBatchProcessedEvent creates a new review batch processed event
func NewReviewBatchProcessedEvent(
	aggregateID uuid.UUID,
	sequenceNumber int64,
	payload *ReviewBatchProcessedEventPayload,
	correlationID string,
) *ReviewBatchProcessedEvent {
	return &ReviewBatchProcessedEvent{
		BaseEvent: BaseEvent{
			ID:             uuid.New(),
			Type:           ReviewBatchProcessedEventType,
			Version:        EventVersionV1,
			AggregateID:    aggregateID,
			AggregateType:  "ReviewBatch",
			OccurredAt:     time.Now(),
			SequenceNumber: sequenceNumber,
			CorrelationID:  correlationID,
			Metadata:       make(map[string]interface{}),
			Payload:        payload,
		},
		BatchID:          payload.BatchID,
		ProcessingJobID:  payload.ProcessingJobID,
		ProviderID:       payload.ProviderID,
		BatchSize:        payload.BatchSize,
		ReviewsProcessed: payload.ReviewsProcessed,
		ReviewsFailed:    payload.ReviewsFailed,
		ProcessingTime:   payload.ProcessingTime,
		HotelStats:       payload.HotelStats,
		ValidationStats:  payload.ValidationStats,
		EnrichmentStats:  payload.EnrichmentStats,
		ProcessedAt:      payload.ProcessedAt,
		Summary:          payload.Summary,
	}
}

// Event Sourcing Support

// EventStore represents the interface for event storage
type EventStore interface {
	// AppendEvents appends events to the store
	AppendEvents(ctx context.Context, aggregateID uuid.UUID, expectedVersion int64, events []DomainEvent) error

	// GetEvents retrieves events for an aggregate
	GetEvents(ctx context.Context, aggregateID uuid.UUID, fromVersion int64) ([]DomainEvent, error)

	// GetEventsWithSnapshots retrieves events with snapshots
	GetEventsWithSnapshots(ctx context.Context, aggregateID uuid.UUID, fromVersion int64) ([]DomainEvent, *Snapshot, error)

	// GetEventsByType retrieves events by type
	GetEventsByType(ctx context.Context, eventType EventType, fromTime time.Time, limit int) ([]DomainEvent, error)

	// GetEventsByCorrelationID retrieves events by correlation ID
	GetEventsByCorrelationID(ctx context.Context, correlationID string) ([]DomainEvent, error)

	// SaveSnapshot saves a snapshot
	SaveSnapshot(ctx context.Context, snapshot *Snapshot) error

	// GetSnapshot gets the latest snapshot for an aggregate
	GetSnapshot(ctx context.Context, aggregateID uuid.UUID) (*Snapshot, error)

	// GetAllEvents retrieves all events with pagination
	GetAllEvents(ctx context.Context, offset, limit int) ([]DomainEvent, error)

	// GetEventsCount returns the total count of events
	GetEventsCount(ctx context.Context) (int64, error)

	// DeleteEvents deletes events (for testing/cleanup)
	DeleteEvents(ctx context.Context, aggregateID uuid.UUID) error
}

// Snapshot represents a snapshot of an aggregate state
type Snapshot struct {
	AggregateID   uuid.UUID              `json:"aggregate_id"`
	AggregateType string                 `json:"aggregate_type"`
	Version       int64                  `json:"version"`
	Data          map[string]interface{} `json:"data"`
	CreatedAt     time.Time              `json:"created_at"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// EventHandler represents the interface for handling events
type EventHandler interface {
	// HandleEvent handles a single event
	HandleEvent(ctx context.Context, event DomainEvent) error

	// CanHandle returns true if the handler can handle the event type
	CanHandle(eventType EventType) bool

	// GetHandledEventTypes returns the event types this handler can handle
	GetHandledEventTypes() []EventType
}

// EventBus represents the interface for event bus operations
type EventBus interface {
	// Subscribe subscribes a handler to event types
	Subscribe(handler EventHandler, eventTypes ...EventType) error

	// Unsubscribe unsubscribes a handler from event types
	Unsubscribe(handler EventHandler, eventTypes ...EventType) error

	// PublishEvent publishes an event to all subscribers
	PublishEvent(ctx context.Context, event DomainEvent) error

	// PublishEvents publishes multiple events to all subscribers
	PublishEvents(ctx context.Context, events []DomainEvent) error

	// Start starts the event bus
	Start(ctx context.Context) error

	// Stop stops the event bus
	Stop() error
}

// AggregateRoot represents the base interface for aggregate roots
type AggregateRoot interface {
	// GetID returns the aggregate ID
	GetID() uuid.UUID

	// GetVersion returns the current version
	GetVersion() int64

	// GetUncommittedEvents returns uncommitted events
	GetUncommittedEvents() []DomainEvent

	// ClearUncommittedEvents clears uncommitted events
	ClearUncommittedEvents()

	// LoadFromHistory loads the aggregate from historical events
	LoadFromHistory(events []DomainEvent) error

	// ApplyEvent applies an event to the aggregate
	ApplyEvent(event DomainEvent) error

	// GetType returns the aggregate type
	GetType() string
}

// EventMigrator represents the interface for event migrations
type EventMigrator interface {
	// MigrateEvent migrates an event from one version to another
	MigrateEvent(event DomainEvent, targetVersion EventVersion) (DomainEvent, error)

	// CanMigrate returns true if the migrator can migrate the event
	CanMigrate(fromVersion, toVersion EventVersion) bool

	// GetSupportedVersions returns the supported versions
	GetSupportedVersions() []EventVersion
}

// EventSerializer represents the interface for event serialization
type EventSerializer interface {
	// Serialize serializes an event to bytes
	Serialize(event DomainEvent) ([]byte, error)

	// Deserialize deserializes bytes to an event
	Deserialize(data []byte, eventType EventType, version EventVersion) (DomainEvent, error)

	// GetSupportedVersions returns the supported versions
	GetSupportedVersions() []EventVersion
}

// EventRegistry represents the interface for event type registration
type EventRegistry interface {
	// RegisterEventType registers an event type
	RegisterEventType(eventType EventType, factory EventFactory) error

	// GetEventFactory returns the factory for an event type
	GetEventFactory(eventType EventType) (EventFactory, error)

	// GetRegisteredEventTypes returns all registered event types
	GetRegisteredEventTypes() []EventType

	// IsRegistered returns true if the event type is registered
	IsRegistered(eventType EventType) bool
}

// EventFactory represents the interface for creating events
type EventFactory interface {
	// CreateEvent creates a new event instance
	CreateEvent() DomainEvent

	// CreateEventFromPayload creates an event from payload
	CreateEventFromPayload(payload interface{}) (DomainEvent, error)

	// GetEventType returns the event type this factory creates
	GetEventType() EventType

	// GetSupportedVersions returns the supported versions
	GetSupportedVersions() []EventVersion
}

// EventProjector represents the interface for event projections
type EventProjector interface {
	// Project projects an event to update read models
	Project(ctx context.Context, event DomainEvent) error

	// CanProject returns true if the projector can project the event
	CanProject(eventType EventType) bool

	// GetProjectedEventTypes returns the event types this projector handles
	GetProjectedEventTypes() []EventType

	// Reset resets the projection
	Reset(ctx context.Context) error
}

// EventReplayService represents the interface for event replay
type EventReplayService interface {
	// ReplayEvents replays events from a specific point in time
	ReplayEvents(ctx context.Context, fromTime time.Time, toTime time.Time) error

	// ReplayEventsForAggregate replays events for a specific aggregate
	ReplayEventsForAggregate(ctx context.Context, aggregateID uuid.UUID, fromVersion int64) error

	// ReplayEventsByType replays events of a specific type
	ReplayEventsByType(ctx context.Context, eventType EventType, fromTime time.Time) error

	// GetReplayStatus returns the current replay status
	GetReplayStatus(ctx context.Context) (*ReplayStatus, error)
}

// ReplayStatus represents the status of event replay
type ReplayStatus struct {
	IsRunning      bool      `json:"is_running"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	EventsReplayed int64     `json:"events_replayed"`
	TotalEvents    int64     `json:"total_events"`
	LastEventID    uuid.UUID `json:"last_event_id"`
	Progress       float64   `json:"progress"`
	Error          string    `json:"error,omitempty"`
}

// EventMetrics represents event-related metrics
type EventMetrics struct {
	TotalEvents      int64                  `json:"total_events"`
	EventsByType     map[EventType]int64    `json:"events_by_type"`
	EventsByVersion  map[EventVersion]int64 `json:"events_by_version"`
	EventsPerSecond  float64                `json:"events_per_second"`
	LastEventTime    time.Time              `json:"last_event_time"`
	AggregateCount   int64                  `json:"aggregate_count"`
	SnapshotCount    int64                  `json:"snapshot_count"`
	ReplayCount      int64                  `json:"replay_count"`
	PublishFailures  int64                  `json:"publish_failures"`
	HandlingFailures int64                  `json:"handling_failures"`
}

// EventHealthChecker represents the interface for event system health checks
type EventHealthChecker interface {
	// CheckHealth checks the health of the event system
	CheckHealth(ctx context.Context) (*EventHealthStatus, error)

	// CheckEventStore checks the health of the event store
	CheckEventStore(ctx context.Context) error

	// CheckEventBus checks the health of the event bus
	CheckEventBus(ctx context.Context) error

	// CheckEventPublisher checks the health of the event publisher
	CheckEventPublisher(ctx context.Context) error

	// GetMetrics returns event system metrics
	GetMetrics(ctx context.Context) (*EventMetrics, error)
}

// EventHealthStatus represents the health status of the event system
type EventHealthStatus struct {
	IsHealthy       bool                   `json:"is_healthy"`
	EventStore      string                 `json:"event_store"`
	EventBus        string                 `json:"event_bus"`
	EventPublisher  string                 `json:"event_publisher"`
	LastCheck       time.Time              `json:"last_check"`
	Metrics         *EventMetrics          `json:"metrics,omitempty"`
	Issues          []string               `json:"issues,omitempty"`
	ComponentStatus map[string]interface{} `json:"component_status,omitempty"`
}
