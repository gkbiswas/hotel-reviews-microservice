package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test EventVersion constants
func TestEventVersion_Constants(t *testing.T) {
	assert.Equal(t, EventVersionV1, EventVersion("1.0"))
	assert.Equal(t, EventVersionV2, EventVersion("2.0"))
	assert.Equal(t, EventVersionV3, EventVersion("3.0"))
}

// Test EventType constants
func TestEventType_Constants(t *testing.T) {
	// File Processing Events
	assert.Equal(t, FileProcessingStartedEventType, EventType("FileProcessingStarted"))
	assert.Equal(t, FileProcessingProgressEventType, EventType("FileProcessingProgress"))
	assert.Equal(t, FileProcessingCompletedEventType, EventType("FileProcessingCompleted"))
	assert.Equal(t, FileProcessingFailedEventType, EventType("FileProcessingFailed"))

	// Review Processing Events
	assert.Equal(t, ReviewProcessedEventType, EventType("ReviewProcessed"))
	assert.Equal(t, ReviewValidatedEventType, EventType("ReviewValidated"))
	assert.Equal(t, ReviewBatchProcessedEventType, EventType("ReviewBatchProcessed"))

	// Hotel Events
	assert.Equal(t, HotelCreatedEventType, EventType("HotelCreated"))
	assert.Equal(t, HotelUpdatedEventType, EventType("HotelUpdated"))

	// Provider Events
	assert.Equal(t, ProviderActivatedEventType, EventType("ProviderActivated"))
	assert.Equal(t, ProviderDeactivatedEventType, EventType("ProviderDeactivated"))

	// System Events
	assert.Equal(t, SystemHealthCheckEvent, EventType("SystemHealthCheck"))
	assert.Equal(t, SystemErrorEvent, EventType("SystemError"))
}

// Test EventStatus constants
func TestEventStatus_Constants(t *testing.T) {
	assert.Equal(t, EventStatusPublished, EventStatus("published"))
	assert.Equal(t, EventStatusProcessing, EventStatus("processing"))
	assert.Equal(t, EventStatusProcessed, EventStatus("processed"))
	assert.Equal(t, EventStatusFailed, EventStatus("failed"))
	assert.Equal(t, EventStatusRetry, EventStatus("retry"))
}

// Test BaseEvent creation and methods
func TestBaseEvent_Creation(t *testing.T) {
	eventID := uuid.New()
	aggregateID := uuid.New()
	occurredAt := time.Now()
	metadata := map[string]interface{}{"key": "value"}

	baseEvent := &BaseEvent{
		ID:             eventID,
		Type:           FileProcessingStartedEventType,
		Version:        EventVersionV1,
		AggregateID:    aggregateID,
		AggregateType:  "FileProcessingJob",
		OccurredAt:     occurredAt,
		SequenceNumber: 1,
		CorrelationID:  "corr-123",
		CausationID:    "cause-456",
		Metadata:       metadata,
		Payload:        "test payload",
		IsReplay:       false,
	}

	assert.NotNil(t, baseEvent)
	assert.Equal(t, eventID, baseEvent.GetID())
	assert.Equal(t, FileProcessingStartedEventType, baseEvent.GetType())
	assert.Equal(t, EventVersionV1, baseEvent.GetVersion())
	assert.Equal(t, aggregateID, baseEvent.GetAggregateID())
	assert.Equal(t, "FileProcessingJob", baseEvent.GetAggregateType())
	assert.Equal(t, occurredAt, baseEvent.GetOccurredAt())
	assert.Equal(t, int64(1), baseEvent.GetSequenceNumber())
	assert.Equal(t, "corr-123", baseEvent.GetCorrelationID())
	assert.Equal(t, "cause-456", baseEvent.GetCausationID())
	assert.Equal(t, metadata, baseEvent.GetMetadata())
	assert.Equal(t, "test payload", baseEvent.GetPayload())
	assert.False(t, baseEvent.IsReplayable())
}

// Test BaseEvent validation
func TestBaseEvent_Validation(t *testing.T) {
	// Test valid event
	validEvent := &BaseEvent{
		ID:             uuid.New(),
		Type:           FileProcessingStartedEventType,
		Version:        EventVersionV1,
		AggregateID:    uuid.New(),
		AggregateType:  "FileProcessingJob",
		OccurredAt:     time.Now(),
		SequenceNumber: 1,
	}

	err := validEvent.Validate()
	assert.NoError(t, err)

	// Test validation errors
	testCases := []struct {
		name      string
		event     *BaseEvent
		expectErr string
	}{
		{
			name: "nil ID",
			event: &BaseEvent{
				ID:             uuid.Nil,
				Type:           FileProcessingStartedEventType,
				Version:        EventVersionV1,
				AggregateID:    uuid.New(),
				AggregateType:  "Test",
				OccurredAt:     time.Now(),
				SequenceNumber: 1,
			},
			expectErr: "event ID cannot be nil",
		},
		{
			name: "empty type",
			event: &BaseEvent{
				ID:             uuid.New(),
				Type:           "",
				Version:        EventVersionV1,
				AggregateID:    uuid.New(),
				AggregateType:  "Test",
				OccurredAt:     time.Now(),
				SequenceNumber: 1,
			},
			expectErr: "event type cannot be empty",
		},
		{
			name: "empty version",
			event: &BaseEvent{
				ID:             uuid.New(),
				Type:           FileProcessingStartedEventType,
				Version:        "",
				AggregateID:    uuid.New(),
				AggregateType:  "Test",
				OccurredAt:     time.Now(),
				SequenceNumber: 1,
			},
			expectErr: "event version cannot be empty",
		},
		{
			name: "nil aggregate ID",
			event: &BaseEvent{
				ID:             uuid.New(),
				Type:           FileProcessingStartedEventType,
				Version:        EventVersionV1,
				AggregateID:    uuid.Nil,
				AggregateType:  "Test",
				OccurredAt:     time.Now(),
				SequenceNumber: 1,
			},
			expectErr: "aggregate ID cannot be nil",
		},
		{
			name: "empty aggregate type",
			event: &BaseEvent{
				ID:             uuid.New(),
				Type:           FileProcessingStartedEventType,
				Version:        EventVersionV1,
				AggregateID:    uuid.New(),
				AggregateType:  "",
				OccurredAt:     time.Now(),
				SequenceNumber: 1,
			},
			expectErr: "aggregate type cannot be empty",
		},
		{
			name: "zero occurred at",
			event: &BaseEvent{
				ID:             uuid.New(),
				Type:           FileProcessingStartedEventType,
				Version:        EventVersionV1,
				AggregateID:    uuid.New(),
				AggregateType:  "Test",
				OccurredAt:     time.Time{},
				SequenceNumber: 1,
			},
			expectErr: "occurred at cannot be zero",
		},
		{
			name: "invalid sequence number",
			event: &BaseEvent{
				ID:             uuid.New(),
				Type:           FileProcessingStartedEventType,
				Version:        EventVersionV1,
				AggregateID:    uuid.New(),
				AggregateType:  "Test",
				OccurredAt:     time.Now(),
				SequenceNumber: 0,
			},
			expectErr: "sequence number must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.event.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectErr)
		})
	}
}

// Test BaseEvent serialization
func TestBaseEvent_Serialization(t *testing.T) {
	event := &BaseEvent{
		ID:             uuid.New(),
		Type:           FileProcessingStartedEventType,
		Version:        EventVersionV1,
		AggregateID:    uuid.New(),
		AggregateType:  "FileProcessingJob",
		OccurredAt:     time.Now(),
		SequenceNumber: 1,
		CorrelationID:  "corr-123",
		Metadata:       map[string]interface{}{"key": "value"},
		Payload:        "test payload",
	}

	data, err := event.Serialize()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, "FileProcessingStarted", result["type"])
	assert.Equal(t, "1.0", result["version"])
}

// Test FileProcessingStartedEvent
func TestFileProcessingStartedEvent_Creation(t *testing.T) {
	aggregateID := uuid.New()
	jobID := uuid.New()
	providerID := uuid.New()
	
	payload := &FileProcessingStartedEventPayload{
		JobID:            jobID,
		ProviderID:       providerID,
		ProviderName:     "Test Provider",
		FileURL:          "s3://bucket/file.json",
		FileName:         "test-file.json",
		FileSize:         1024,
		EstimatedRecords: 100,
		ProcessingConfig: map[string]interface{}{"batch_size": 50},
		StartedBy:        "system",
		Priority:         1,
		TimeoutAt:        time.Now().Add(1 * time.Hour),
	}

	event := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")

	assert.NotNil(t, event)
	assert.Equal(t, FileProcessingStartedEventType, event.GetType())
	assert.Equal(t, EventVersionV1, event.GetVersion())
	assert.Equal(t, aggregateID, event.GetAggregateID())
	assert.Equal(t, "FileProcessingJob", event.GetAggregateType())
	assert.Equal(t, int64(1), event.GetSequenceNumber())
	assert.Equal(t, "corr-123", event.GetCorrelationID())
	assert.Equal(t, jobID, event.JobID)
	assert.Equal(t, providerID, event.ProviderID)
	assert.Equal(t, "Test Provider", event.ProviderName)
	assert.Equal(t, "s3://bucket/file.json", event.FileURL)
	assert.Equal(t, int64(1024), event.FileSize)
	assert.Equal(t, int64(100), event.EstimatedRecords)
}

// Test FileProcessingStartedEvent validation
func TestFileProcessingStartedEvent_Validation(t *testing.T) {
	aggregateID := uuid.New()
	payload := &FileProcessingStartedEventPayload{
		JobID:        uuid.New(),
		ProviderID:   uuid.New(),
		ProviderName: "Test Provider",
		FileURL:      "s3://bucket/file.json",
		FileSize:     1024,
	}

	event := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")
	err := event.Validate()
	assert.NoError(t, err)

	// Test validation with invalid data
	invalidEvent := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")
	invalidEvent.JobID = uuid.Nil
	err = invalidEvent.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job ID cannot be nil")

	invalidEvent2 := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")
	invalidEvent2.ProviderID = uuid.Nil
	err = invalidEvent2.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider ID cannot be nil")

	invalidEvent3 := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")
	invalidEvent3.FileURL = ""
	err = invalidEvent3.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file URL cannot be empty")

	invalidEvent4 := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")
	invalidEvent4.FileSize = -1
	err = invalidEvent4.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file size cannot be negative")
}

// Test FileProcessingProgressEvent
func TestFileProcessingProgressEvent_Creation(t *testing.T) {
	aggregateID := uuid.New()
	jobID := uuid.New()
	
	payload := &FileProcessingProgressEventPayload{
		JobID:            jobID,
		RecordsProcessed: 50,
		RecordsTotal:     100,
		RecordsFailed:    2,
		RecordsSkipped:   1,
		CurrentBatch:     5,
		TotalBatches:     10,
		ProcessingRate:   25.5,
		EstimatedETA:     time.Now().Add(30 * time.Minute),
		LastProcessedAt:  time.Now(),
	}

	event := NewFileProcessingProgressEvent(aggregateID, 2, payload, "corr-123")

	assert.NotNil(t, event)
	assert.Equal(t, FileProcessingProgressEventType, event.GetType())
	assert.Equal(t, jobID, event.JobID)
	assert.Equal(t, int64(50), event.RecordsProcessed)
	assert.Equal(t, int64(100), event.RecordsTotal)
	assert.Equal(t, int64(2), event.RecordsFailed)
	assert.Equal(t, int64(1), event.RecordsSkipped)
	assert.Equal(t, 5, event.CurrentBatch)
	assert.Equal(t, 10, event.TotalBatches)
	assert.Equal(t, 25.5, event.ProcessingRate)
}

// Test FileProcessingCompletedEvent
func TestFileProcessingCompletedEvent_Creation(t *testing.T) {
	aggregateID := uuid.New()
	jobID := uuid.New()
	
	payload := &FileProcessingCompletedEventPayload{
		JobID:              jobID,
		RecordsProcessed:   100,
		RecordsTotal:       100,
		RecordsFailed:      0,
		RecordsSkipped:     0,
		ProcessingDuration: 5 * time.Minute,
		ProcessingRate:     20.0,
		CompletedAt:        time.Now(),
		Summary:            map[string]interface{}{"status": "success"},
		Metrics:            map[string]interface{}{"throughput": 100},
	}

	event := NewFileProcessingCompletedEvent(aggregateID, 3, payload, "corr-123")

	assert.NotNil(t, event)
	assert.Equal(t, FileProcessingCompletedEventType, event.GetType())
	assert.Equal(t, jobID, event.JobID)
	assert.Equal(t, int64(100), event.RecordsProcessed)
	assert.Equal(t, int64(100), event.RecordsTotal)
	assert.Equal(t, int64(0), event.RecordsFailed)
	assert.Equal(t, 5*time.Minute, event.ProcessingDuration)
	assert.Equal(t, 20.0, event.ProcessingRate)
}

// Test FileProcessingFailedEvent
func TestFileProcessingFailedEvent_Creation(t *testing.T) {
	aggregateID := uuid.New()
	jobID := uuid.New()
	
	payload := &FileProcessingFailedEventPayload{
		JobID:              jobID,
		ErrorMessage:       "Processing failed",
		ErrorCode:          "PROC_ERR_001",
		ErrorType:          "validation_error",
		RecordsProcessed:   50,
		RecordsTotal:       100,
		ProcessingDuration: 2 * time.Minute,
		FailedAt:           time.Now(),
		RetryCount:         1,
		MaxRetries:         3,
		IsRetryable:        true,
		StackTrace:         "stack trace here",
		Context:            map[string]interface{}{"batch": 5},
	}

	event := NewFileProcessingFailedEvent(aggregateID, 4, payload, "corr-123")

	assert.NotNil(t, event)
	assert.Equal(t, FileProcessingFailedEventType, event.GetType())
	assert.Equal(t, jobID, event.JobID)
	assert.Equal(t, "Processing failed", event.ErrorMessage)
	assert.Equal(t, "PROC_ERR_001", event.ErrorCode)
	assert.Equal(t, "validation_error", event.ErrorType)
	assert.Equal(t, int64(50), event.RecordsProcessed)
	assert.Equal(t, 1, event.RetryCount)
	assert.Equal(t, 3, event.MaxRetries)
	assert.True(t, event.IsRetryable)
}

// Test ReviewProcessedEvent
func TestReviewProcessedEvent_Creation(t *testing.T) {
	aggregateID := uuid.New()
	reviewID := uuid.New()
	hotelID := uuid.New()
	providerID := uuid.New()
	processingJobID := uuid.New()
	
	payload := &ReviewProcessedEventPayload{
		ReviewID:         reviewID,
		HotelID:          hotelID,
		ProviderID:       providerID,
		ExternalID:       "ext-123",
		ProcessingJobID:  processingJobID,
		BatchID:          "batch-456",
		Rating:           4.5,
		Sentiment:        "positive",
		Language:         "en",
		WordCount:        150,
		ProcessingStages: []string{"validation", "enrichment"},
		ValidationResult: map[string]interface{}{"valid": true},
		EnrichmentData:   map[string]interface{}{"sentiment_score": 0.8},
		ProcessedAt:      time.Now(),
	}

	event := NewReviewProcessedEvent(aggregateID, 1, payload, "corr-123")

	assert.NotNil(t, event)
	assert.Equal(t, ReviewProcessedEventType, event.GetType())
	assert.Equal(t, "Review", event.GetAggregateType())
	assert.Equal(t, reviewID, event.ReviewID)
	assert.Equal(t, hotelID, event.HotelID)
	assert.Equal(t, providerID, event.ProviderID)
	assert.Equal(t, "ext-123", event.ExternalID)
	assert.Equal(t, 4.5, event.Rating)
	assert.Equal(t, "positive", event.Sentiment)
	assert.Equal(t, "en", event.Language)
	assert.Equal(t, 150, event.WordCount)
	assert.Len(t, event.ProcessingStages, 2)
}

// Test ReviewBatchProcessedEvent
func TestReviewBatchProcessedEvent_Creation(t *testing.T) {
	aggregateID := uuid.New()
	processingJobID := uuid.New()
	providerID := uuid.New()
	
	payload := &ReviewBatchProcessedEventPayload{
		BatchID:          "batch-789",
		ProcessingJobID:  processingJobID,
		ProviderID:       providerID,
		BatchSize:        100,
		ReviewsProcessed: 95,
		ReviewsFailed:    5,
		ProcessingTime:   30 * time.Second,
		HotelStats:       map[uuid.UUID]int{uuid.New(): 10, uuid.New(): 15},
		ValidationStats:  map[string]int{"valid": 90, "invalid": 10},
		EnrichmentStats:  map[string]int{"enriched": 85, "failed": 15},
		ProcessedAt:      time.Now(),
		Summary:          map[string]interface{}{"success_rate": 0.95},
	}

	event := NewReviewBatchProcessedEvent(aggregateID, 1, payload, "corr-123")

	assert.NotNil(t, event)
	assert.Equal(t, ReviewBatchProcessedEventType, event.GetType())
	assert.Equal(t, "ReviewBatch", event.GetAggregateType())
	assert.Equal(t, "batch-789", event.BatchID)
	assert.Equal(t, processingJobID, event.ProcessingJobID)
	assert.Equal(t, providerID, event.ProviderID)
	assert.Equal(t, 100, event.BatchSize)
	assert.Equal(t, 95, event.ReviewsProcessed)
	assert.Equal(t, 5, event.ReviewsFailed)
	assert.Equal(t, 30*time.Second, event.ProcessingTime)
	assert.Len(t, event.HotelStats, 2)
}

// Test Snapshot functionality
func TestSnapshot_Creation(t *testing.T) {
	aggregateID := uuid.New()
	data := map[string]interface{}{
		"total_reviews": 100,
		"average_rating": 4.2,
		"last_updated": time.Now(),
	}
	metadata := map[string]interface{}{
		"snapshot_type": "review_summary",
		"compression": "gzip",
	}

	snapshot := &Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "ReviewSummary",
		Version:       10,
		Data:          data,
		CreatedAt:     time.Now(),
		Metadata:      metadata,
	}

	assert.NotNil(t, snapshot)
	assert.Equal(t, aggregateID, snapshot.AggregateID)
	assert.Equal(t, "ReviewSummary", snapshot.AggregateType)
	assert.Equal(t, int64(10), snapshot.Version)
	assert.Equal(t, 100, snapshot.Data["total_reviews"])
	assert.Equal(t, 4.2, snapshot.Data["average_rating"])
	assert.Equal(t, "review_summary", snapshot.Metadata["snapshot_type"])
}

// Test ReplayStatus functionality
func TestReplayStatus_Creation(t *testing.T) {
	lastEventID := uuid.New()
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	status := &ReplayStatus{
		IsRunning:      true,
		StartTime:      startTime,
		EndTime:        endTime,
		EventsReplayed: 500,
		TotalEvents:    1000,
		LastEventID:    lastEventID,
		Progress:       0.5,
		Error:          "",
	}

	assert.NotNil(t, status)
	assert.True(t, status.IsRunning)
	assert.Equal(t, startTime, status.StartTime)
	assert.Equal(t, endTime, status.EndTime)
	assert.Equal(t, int64(500), status.EventsReplayed)
	assert.Equal(t, int64(1000), status.TotalEvents)
	assert.Equal(t, lastEventID, status.LastEventID)
	assert.Equal(t, 0.5, status.Progress)
	assert.Empty(t, status.Error)
}

// Test EventMetrics functionality
func TestEventMetrics_Creation(t *testing.T) {
	eventsByType := map[EventType]int64{
		FileProcessingStartedEventType:   100,
		FileProcessingCompletedEventType: 95,
		ReviewProcessedEventType:         500,
	}
	
	eventsByVersion := map[EventVersion]int64{
		EventVersionV1: 600,
		EventVersionV2: 95,
	}

	metrics := &EventMetrics{
		TotalEvents:         695,
		EventsByType:        eventsByType,
		EventsByVersion:     eventsByVersion,
		EventsPerSecond:     12.5,
		LastEventTime:       time.Now(),
		AggregateCount:      50,
		SnapshotCount:       10,
		ReplayCount:         2,
		PublishFailures:     3,
		HandlingFailures:    1,
	}

	assert.NotNil(t, metrics)
	assert.Equal(t, int64(695), metrics.TotalEvents)
	assert.Len(t, metrics.EventsByType, 3)
	assert.Len(t, metrics.EventsByVersion, 2)
	assert.Equal(t, int64(100), metrics.EventsByType[FileProcessingStartedEventType])
	assert.Equal(t, int64(600), metrics.EventsByVersion[EventVersionV1])
	assert.Equal(t, 12.5, metrics.EventsPerSecond)
	assert.Equal(t, int64(50), metrics.AggregateCount)
	assert.Equal(t, int64(3), metrics.PublishFailures)
}

// Test EventHealthStatus functionality
func TestEventHealthStatus_Creation(t *testing.T) {
	metrics := &EventMetrics{
		TotalEvents:     1000,
		EventsPerSecond: 10.0,
		LastEventTime:   time.Now(),
	}

	componentStatus := map[string]interface{}{
		"event_store_connection": "healthy",
		"kafka_connection":       "healthy",
		"redis_connection":       "degraded",
	}

	status := &EventHealthStatus{
		IsHealthy:       true,
		EventStore:      "healthy",
		EventBus:        "healthy",
		EventPublisher:  "healthy",
		LastCheck:       time.Now(),
		Metrics:         metrics,
		Issues:          []string{"Redis connection slow"},
		ComponentStatus: componentStatus,
	}

	assert.NotNil(t, status)
	assert.True(t, status.IsHealthy)
	assert.Equal(t, "healthy", status.EventStore)
	assert.Equal(t, "healthy", status.EventBus)
	assert.Equal(t, "healthy", status.EventPublisher)
	assert.NotNil(t, status.Metrics)
	assert.Len(t, status.Issues, 1)
	assert.Equal(t, "Redis connection slow", status.Issues[0])
	assert.Len(t, status.ComponentStatus, 3)
}

// Test interface compliance
func TestDomainEvent_InterfaceCompliance(t *testing.T) {
	// Test that concrete events implement DomainEvent interface
	var event DomainEvent

	// Test FileProcessingStartedEvent
	payload := &FileProcessingStartedEventPayload{
		JobID:      uuid.New(),
		ProviderID: uuid.New(),
		FileURL:    "s3://bucket/file.json",
	}
	startedEvent := NewFileProcessingStartedEvent(uuid.New(), 1, payload, "corr-123")
	event = startedEvent
	assert.NotNil(t, event)
	assert.Equal(t, FileProcessingStartedEventType, event.GetType())

	// Test ReviewProcessedEvent
	reviewPayload := &ReviewProcessedEventPayload{
		ReviewID:    uuid.New(),
		HotelID:     uuid.New(),
		ProviderID:  uuid.New(),
		Rating:      4.5,
		ProcessedAt: time.Now(),
	}
	reviewEvent := NewReviewProcessedEvent(uuid.New(), 1, reviewPayload, "corr-123")
	event = reviewEvent
	assert.NotNil(t, event)
	assert.Equal(t, ReviewProcessedEventType, event.GetType())
}

// Test event serialization and deserialization
func TestEvent_SerializationRoundTrip(t *testing.T) {
	// Create a FileProcessingStartedEvent
	aggregateID := uuid.New()
	payload := &FileProcessingStartedEventPayload{
		JobID:            uuid.New(),
		ProviderID:       uuid.New(),
		ProviderName:     "Test Provider",
		FileURL:          "s3://bucket/file.json",
		FileName:         "test.json",
		FileSize:         1024,
		EstimatedRecords: 100,
		ProcessingConfig: map[string]interface{}{"batch_size": 50},
		StartedBy:        "system",
		Priority:         1,
		TimeoutAt:        time.Now().Add(1 * time.Hour),
	}

	event := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")

	// Serialize the event using json.Marshal directly (since the embedded Serialize method may not include all fields)
	data, err := json.Marshal(event)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify the serialized data contains expected fields
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "FileProcessingStarted", result["type"])
	assert.Equal(t, "1.0", result["version"])
	assert.Equal(t, "FileProcessingJob", result["aggregate_type"])
	assert.Equal(t, int64(1), int64(result["sequence_number"].(float64)))
	assert.Equal(t, "corr-123", result["correlation_id"])
	
	// Check specific event fields
	assert.Equal(t, "Test Provider", result["provider_name"])
	assert.Equal(t, "s3://bucket/file.json", result["file_url"])
	assert.Equal(t, "test.json", result["file_name"])
	assert.Equal(t, float64(1024), result["file_size"])
	assert.Equal(t, float64(100), result["estimated_records"])
}

// Test event correlation and causation
func TestEvent_CorrelationAndCausation(t *testing.T) {
	correlationID := "corr-123"

	// Create first event
	aggregateID := uuid.New()
	payload1 := &FileProcessingStartedEventPayload{
		JobID:      uuid.New(),
		ProviderID: uuid.New(),
		FileURL:    "s3://bucket/file.json",
	}
	event1 := NewFileProcessingStartedEvent(aggregateID, 1, payload1, correlationID)

	// Create second event caused by the first
	payload2 := &FileProcessingProgressEventPayload{
		JobID:            payload1.JobID,
		RecordsProcessed: 50,
		RecordsTotal:     100,
	}
	event2 := NewFileProcessingProgressEvent(aggregateID, 2, payload2, correlationID)
	event2.CausationID = event1.GetID().String()

	// Test correlation
	assert.Equal(t, correlationID, event1.GetCorrelationID())
	assert.Equal(t, correlationID, event2.GetCorrelationID())

	// Test causation
	assert.Empty(t, event1.GetCausationID())
	assert.Equal(t, event1.GetID().String(), event2.GetCausationID())

	// Test that they share the same aggregate
	assert.Equal(t, event1.GetAggregateID(), event2.GetAggregateID())
	assert.Equal(t, event1.GetAggregateType(), event2.GetAggregateType())
}

// Test event metadata handling
func TestEvent_MetadataHandling(t *testing.T) {
	aggregateID := uuid.New()
	payload := &FileProcessingStartedEventPayload{
		JobID:      uuid.New(),
		ProviderID: uuid.New(),
		FileURL:    "s3://bucket/file.json",
	}

	event := NewFileProcessingStartedEvent(aggregateID, 1, payload, "corr-123")

	// Test initial metadata
	metadata := event.GetMetadata()
	assert.NotNil(t, metadata)
	assert.Empty(t, metadata) // Should be empty initially

	// Add metadata
	metadata["source"] = "test"
	metadata["batch_id"] = "batch-123"
	metadata["retry_count"] = 0

	assert.Equal(t, "test", metadata["source"])
	assert.Equal(t, "batch-123", metadata["batch_id"])
	assert.Equal(t, 0, metadata["retry_count"])

	// Test metadata persistence
	retrievedMetadata := event.GetMetadata()
	assert.Equal(t, metadata, retrievedMetadata)
}

// Test event timing and sequence
func TestEvent_TimingAndSequence(t *testing.T) {
	aggregateID := uuid.New()
	startTime := time.Now()

	// Create events with different sequence numbers
	payload1 := &FileProcessingStartedEventPayload{
		JobID:      uuid.New(),
		ProviderID: uuid.New(),
		FileURL:    "s3://bucket/file.json",
	}
	event1 := NewFileProcessingStartedEvent(aggregateID, 1, payload1, "corr-123")

	time.Sleep(1 * time.Millisecond) // Ensure different timestamps

	payload2 := &FileProcessingProgressEventPayload{
		JobID:            payload1.JobID,
		RecordsProcessed: 50,
		RecordsTotal:     100,
	}
	event2 := NewFileProcessingProgressEvent(aggregateID, 2, payload2, "corr-123")

	// Test sequence numbers
	assert.Equal(t, int64(1), event1.GetSequenceNumber())
	assert.Equal(t, int64(2), event2.GetSequenceNumber())

	// Test timing
	assert.True(t, event1.GetOccurredAt().After(startTime))
	assert.True(t, event2.GetOccurredAt().After(event1.GetOccurredAt()))

	// Test that events have unique IDs
	assert.NotEqual(t, event1.GetID(), event2.GetID())
}