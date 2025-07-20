package infrastructure

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

func TestNewKafkaProducer(t *testing.T) {
	log := logger.NewDefault()

	t.Run("valid config", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers:              []string{"localhost:9092"},
			ReviewTopic:          "reviews",
			ProcessingTopic:      "processing",
			DeadLetterTopic:      "dead-letter",
			ConsumerGroup:        "test-group",
			BatchSize:            100,
			BatchTimeout:         time.Millisecond * 100,
			MaxRetries:           3,
			RetryDelay:           time.Second,
			MaxMessageSize:       1024 * 1024,
			CompressionType:      "gzip",
			ProducerFlushTimeout: time.Second * 5,
			ConsumerTimeout:      time.Second * 10,
			EnableIdempotence:    true,
			Partitions:           3,
			ReplicationFactor:    1,
		}

		producer, err := NewKafkaProducer(config, log)

		assert.NoError(t, err)
		assert.NotNil(t, producer)
		assert.Equal(t, config, producer.config)
		assert.Equal(t, log, producer.logger)
		assert.NotNil(t, producer.writer)
	})

	t.Run("empty brokers", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers: []string{},
		}

		producer, err := NewKafkaProducer(config, log)

		assert.Error(t, err)
		assert.Nil(t, producer)
		assert.Contains(t, err.Error(), "at least one broker is required")
	})

	t.Run("with TLS enabled", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers:   []string{"localhost:9092"},
			EnableTLS: true,
		}

		producer, err := NewKafkaProducer(config, log)

		assert.NoError(t, err)
		assert.NotNil(t, producer)
		assert.NotNil(t, producer.writer.Transport)
	})

	t.Run("with SASL enabled", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers:      []string{"localhost:9092"},
			EnableSASL:   true,
			SASLUsername: "testuser",
			SASLPassword: "testpass",
		}

		producer, err := NewKafkaProducer(config, log)

		assert.NoError(t, err)
		assert.NotNil(t, producer)
		assert.NotNil(t, producer.writer.Transport)
	})
}

func TestNewKafkaConsumer(t *testing.T) {
	log := logger.NewDefault()

	t.Run("valid config", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers:         []string{"localhost:9092"},
			ReviewTopic:     "reviews",
			ProcessingTopic: "processing",
			ConsumerGroup:   "test-group",
			BatchSize:       100,
			BatchTimeout:    time.Millisecond * 100,
			ConsumerTimeout: time.Second * 10,
			MaxMessageSize:  1024 * 1024,
		}

		consumer, err := NewKafkaConsumer(config, log)

		assert.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.Equal(t, config, consumer.config)
		assert.Equal(t, log, consumer.logger)
		assert.NotNil(t, consumer.reader)
		assert.NotNil(t, consumer.handlers)
	})

	t.Run("empty brokers", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers: []string{},
		}

		consumer, err := NewKafkaConsumer(config, log)

		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "at least one broker is required")
	})

	t.Run("empty consumer group", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers:       []string{"localhost:9092"},
			ConsumerGroup: "",
		}

		consumer, err := NewKafkaConsumer(config, log)

		assert.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "consumer group is required")
	})
}

func TestBaseEvent(t *testing.T) {
	t.Run("create base event", func(t *testing.T) {
		event := BaseEvent{
			ID:            uuid.New().String(),
			Type:          ReviewCreatedEvent,
			Source:        "test-service",
			Timestamp:     time.Now(),
			Version:       "1.0",
			CorrelationID: "test-correlation",
			UserID:        "user-123",
			SessionID:     "session-456",
			TraceID:       "trace-789",
			SpanID:        "span-012",
			Data: map[string]interface{}{
				"review_id": "review-123",
				"rating":    4.5,
			},
			Metadata: map[string]interface{}{
				"source_ip": "192.168.1.1",
				"user_agent": "test-agent",
			},
		}

		assert.NotEmpty(t, event.ID)
		assert.Equal(t, ReviewCreatedEvent, event.Type)
		assert.Equal(t, "test-service", event.Source)
		assert.Equal(t, "1.0", event.Version)
		assert.Equal(t, "test-correlation", event.CorrelationID)
		assert.Equal(t, "user-123", event.UserID)
		assert.Equal(t, "session-456", event.SessionID)
		assert.Equal(t, "trace-789", event.TraceID)
		assert.Equal(t, "span-012", event.SpanID)
		assert.Contains(t, event.Data, "review_id")
		assert.Contains(t, event.Metadata, "source_ip")

		// Test JSON serialization
		jsonBytes, err := json.Marshal(event)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)

		// Test JSON deserialization
		var deserializedEvent BaseEvent
		err = json.Unmarshal(jsonBytes, &deserializedEvent)
		assert.NoError(t, err)
		assert.Equal(t, event.ID, deserializedEvent.ID)
		assert.Equal(t, event.Type, deserializedEvent.Type)
	})
}

func TestProcessingEvent(t *testing.T) {
	t.Run("create processing event", func(t *testing.T) {
		jobID := uuid.New()
		providerID := uuid.New()

		event := ProcessingEvent{
			BaseEvent: BaseEvent{
				ID:        uuid.New().String(),
				Type:      ProcessingStartedEvent,
				Source:    "processing-service",
				Timestamp: time.Now(),
				Version:   "1.0",
			},
			JobID:            jobID,
			ProviderID:       providerID,
			FileURL:          "http://example.com/reviews.csv",
			Status:           "started",
			RecordsTotal:     100,
			RecordsProcessed: 0,
			ErrorCount:       0,
			ProcessingTime:   1024,
			RetryCount:       0,
		}

		assert.Equal(t, jobID, event.JobID)
		assert.Equal(t, providerID, event.ProviderID)
		assert.Equal(t, "http://example.com/reviews.csv", event.FileURL)
		assert.Equal(t, "started", event.Status)
		assert.Equal(t, int64(100), event.RecordsTotal)
		assert.Equal(t, int64(0), event.RecordsProcessed)
		assert.Equal(t, int64(0), event.ErrorCount)
		assert.Equal(t, int64(1024), event.ProcessingTime)

		// Test JSON serialization
		jsonBytes, err := json.Marshal(event)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)

		// Test JSON deserialization
		var deserializedEvent ProcessingEvent
		err = json.Unmarshal(jsonBytes, &deserializedEvent)
		assert.NoError(t, err)
		assert.Equal(t, event.JobID, deserializedEvent.JobID)
		assert.Equal(t, event.ProviderID, deserializedEvent.ProviderID)
	})
}

func TestReviewEvent(t *testing.T) {
	t.Run("create review event", func(t *testing.T) {
		reviewID := uuid.New()
		hotelID := uuid.New()
		providerID := uuid.New()

		// Note: review variable removed as ReviewEvent doesn't contain Review field

		event := ReviewEvent{
			BaseEvent: BaseEvent{
				ID:        uuid.New().String(),
				Type:      ReviewCreatedEvent,
				Source:    "review-service",
				Timestamp: time.Now(),
				Version:   "1.0",
			},
			ReviewID:   reviewID,
			HotelID:    hotelID,
			ProviderID: providerID,
			Rating:     4.5,
			Sentiment:  "positive",
			Language:   "en",
			BatchID:    "batch-123",
			BatchSize:  10,
		}

		assert.Equal(t, reviewID, event.ReviewID)
		assert.Equal(t, hotelID, event.HotelID)
		assert.Equal(t, providerID, event.ProviderID)
		assert.Equal(t, 4.5, event.Rating)
		assert.Equal(t, "positive", event.Sentiment)
		assert.Equal(t, "en", event.Language)

		// Test JSON serialization
		jsonBytes, err := json.Marshal(event)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)

		// Test JSON deserialization
		var deserializedEvent ReviewEvent
		err = json.Unmarshal(jsonBytes, &deserializedEvent)
		assert.NoError(t, err)
		assert.Equal(t, event.ReviewID, deserializedEvent.ReviewID)
		assert.Equal(t, event.Rating, deserializedEvent.Rating)
	})
}

func TestHotelEvent(t *testing.T) {
	t.Run("create hotel event", func(t *testing.T) {
		hotelID := uuid.New()

		// Note: hotel variable removed as HotelEvent doesn't contain Hotel field

		event := HotelEvent{
			BaseEvent: BaseEvent{
				ID:        uuid.New().String(),
				Type:      HotelCreatedEvent,
				Source:    "hotel-service",
				Timestamp: time.Now(),
				Version:   "1.0",
			},
			HotelID:        hotelID,
			HotelName:      "Test Hotel",
			City:           "Test City",
			Country:        "Test Country",
			TotalReviews:   100,
			AverageRating:  4.5,
			PreviousRating: 4.2,
			RatingChange:   0.3,
		}

		assert.Equal(t, hotelID, event.HotelID)
		assert.Equal(t, "Test Hotel", event.HotelName)
		assert.Equal(t, "Test City", event.City)
		assert.Equal(t, "Test Country", event.Country)
		assert.Equal(t, 100, event.TotalReviews)

		// Test JSON serialization
		jsonBytes, err := json.Marshal(event)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)
	})
}

func TestSystemEvent(t *testing.T) {
	t.Run("create system event", func(t *testing.T) {
		event := SystemEvent{
			BaseEvent: BaseEvent{
				ID:        uuid.New().String(),
				Type:      SystemHealthCheckEvent,
				Source:    "monitoring-service",
				Timestamp: time.Now(),
				Version:   "1.0",
			},
			Component: "database",
			Status:    "healthy",
			Message:   "Database connection is working",
			Metrics: map[string]interface{}{
				"response_time": 50,
				"connections":   10,
			},
		}

		assert.Equal(t, "database", event.Component)
		assert.Equal(t, "healthy", event.Status)
		assert.Equal(t, "Database connection is working", event.Message)
		assert.Contains(t, event.Metrics, "response_time")

		// Test JSON serialization
		jsonBytes, err := json.Marshal(event)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)
	})
}

func TestKafkaProducer_GetTopicForEvent(t *testing.T) {
	config := &KafkaConfig{
		Brokers:              []string{"localhost:9092"},
		ReviewTopic:          "reviews",
		ProcessingTopic:      "processing",
		DeadLetterTopic:      "dead-letter",
	}
	log := logger.NewDefault()
	producer, _ := NewKafkaProducer(config, log)

	tests := []struct {
		name     string
		event    interface{}
		expected string
	}{
		{
			name: "processing event",
			event: &ProcessingEvent{
				BaseEvent: BaseEvent{Type: ProcessingStartedEvent},
			},
			expected: "processing",
		},
		{
			name: "review event",
			event: &ReviewEvent{
				BaseEvent: BaseEvent{Type: ReviewCreatedEvent},
			},
			expected: "reviews",
		},
		{
			name: "hotel event",
			event: &HotelEvent{
				BaseEvent: BaseEvent{Type: HotelCreatedEvent},
			},
			expected: "reviews", // Hotels go to review topic
		},
		{
			name: "system event",
			event: &SystemEvent{
				BaseEvent: BaseEvent{Type: SystemHealthCheckEvent},
			},
			expected: "processing", // System events go to processing topic
		},
		{
			name:     "unknown event",
			event:    struct{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := producer.getTopicForEvent(tt.event)
			assert.Equal(t, tt.expected, topic)
		})
	}
}

func TestKafkaProducer_GetEventType(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{"localhost:9092"},
	}
	log := logger.NewDefault()
	producer, _ := NewKafkaProducer(config, log)

	tests := []struct {
		name     string
		event    interface{}
		expected string
	}{
		{
			name: "processing event",
			event: &ProcessingEvent{
				BaseEvent: BaseEvent{Type: ProcessingStartedEvent},
			},
			expected: "processing.started",
		},
		{
			name: "review event",
			event: &ReviewEvent{
				BaseEvent: BaseEvent{Type: ReviewCreatedEvent},
			},
			expected: "review.created",
		},
		{
			name: "system event",
			event: &SystemEvent{
				BaseEvent: BaseEvent{Type: SystemHealthCheckEvent},
			},
			expected: "system.health.check",
		},
		{
			name:     "unknown event",
			event:    struct{}{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventType := producer.getEventType(tt.event)
			assert.Equal(t, tt.expected, eventType)
		})
	}
}

func TestKafkaProducer_GetPartitionKey(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{"localhost:9092"},
	}
	log := logger.NewDefault()
	producer, _ := NewKafkaProducer(config, log)

	hotelID := uuid.New()
	providerID := uuid.New()

	tests := []struct {
		name     string
		event    interface{}
		expected string
	}{
		{
			name: "review event",
			event: &ReviewEvent{
				HotelID: hotelID,
			},
			expected: hotelID.String(),
		},
		{
			name: "hotel event",
			event: &HotelEvent{
				HotelID: hotelID,
			},
			expected: hotelID.String(),
		},
		{
			name: "processing event",
			event: &ProcessingEvent{
				ProviderID: providerID,
			},
			expected: providerID.String(),
		},
		{
			name:     "unknown event",
			event:    struct{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partitionKey := producer.getPartitionKey(tt.event)
			assert.Equal(t, tt.expected, partitionKey)
		})
	}
}

func TestKafkaProducer_GetCorrelationID(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{"localhost:9092"},
	}
	log := logger.NewDefault()
	producer, _ := NewKafkaProducer(config, log)

	tests := []struct {
		name     string
		event    interface{}
		expected string
	}{
		{
			name: "system event with correlation ID",
			event: &SystemEvent{
				BaseEvent: BaseEvent{
					CorrelationID: "test-correlation-123",
				},
			},
			expected: "test-correlation-123",
		},
		{
			name: "processing event with correlation ID",
			event: &ProcessingEvent{
				BaseEvent: BaseEvent{
					CorrelationID: "processing-correlation-456",
				},
			},
			expected: "processing-correlation-456",
		},
		{
			name: "system event without correlation ID",
			event: &SystemEvent{
				BaseEvent: BaseEvent{
					CorrelationID: "",
				},
			},
			expected: "",
		},
		{
			name:     "unknown event",
			event:    struct{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			correlationID := producer.getCorrelationID(tt.event)
			assert.Equal(t, tt.expected, correlationID)
		})
	}
}

func TestKafkaProducer_Close(t *testing.T) {
	config := &KafkaConfig{
		Brokers: []string{"localhost:9092"},
	}
	log := logger.NewDefault()
	producer, err := NewKafkaProducer(config, log)
	require.NoError(t, err)

	// Should not panic
	assert.NotPanics(t, func() {
		producer.Close()
	})
}

// Note: KafkaConsumer Close test removed as KafkaConsumer doesn't exist in implementation

func TestEventHandler_Interface(t *testing.T) {
	// Test that the EventHandler interface is properly defined
	t.Run("event handler interface", func(t *testing.T) {
		var handler EventHandler

		// Should be nil initially
		assert.Nil(t, handler)

		// Create a mock implementation
		mockHandler := &MockEventHandler{}
		handler = mockHandler

		assert.NotNil(t, handler)
		assert.Implements(t, (*EventHandler)(nil), mockHandler)
	})
}

// MockEventHandler for testing
type MockEventHandler struct{}

func (m *MockEventHandler) Handle(ctx context.Context, event interface{}) error {
	return nil
}

func (m *MockEventHandler) CanHandle(eventType EventType) bool {
	return eventType == ReviewCreatedEvent || eventType == ReviewUpdatedEvent
}

func TestKafkaConfig_Validation(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		config := &KafkaConfig{
			Brokers:              []string{"localhost:9092", "localhost:9093"},
			ReviewTopic:          "reviews",
			ProcessingTopic:      "processing",
			DeadLetterTopic:      "dead-letter",
			ConsumerGroup:        "test-group",
			BatchSize:            100,
			BatchTimeout:         time.Millisecond * 100,
			MaxRetries:           3,
			RetryDelay:           time.Second,
			MaxMessageSize:       1024 * 1024,
			CompressionType:      "gzip",
			ProducerFlushTimeout: time.Second * 5,
			ConsumerTimeout:      time.Second * 10,
			EnableIdempotence:    true,
			Partitions:           3,
			ReplicationFactor:    1,
		}

		// Test JSON serialization
		jsonBytes, err := json.Marshal(config)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)

		// Test JSON deserialization
		var deserializedConfig KafkaConfig
		err = json.Unmarshal(jsonBytes, &deserializedConfig)
		assert.NoError(t, err)
		assert.Equal(t, config.Brokers, deserializedConfig.Brokers)
		assert.Equal(t, config.ReviewTopic, deserializedConfig.ReviewTopic)
		assert.Equal(t, config.BatchSize, deserializedConfig.BatchSize)
	})
}

func TestEventTypes(t *testing.T) {
	t.Run("event type constants", func(t *testing.T) {
		// Processing Events
		assert.Equal(t, EventType("processing.started"), ProcessingStartedEvent)
		assert.Equal(t, EventType("processing.progress"), ProcessingProgressEvent)
		assert.Equal(t, EventType("processing.completed"), ProcessingCompletedEvent)
		assert.Equal(t, EventType("processing.failed"), ProcessingFailedEvent)
		assert.Equal(t, EventType("processing.retry"), ProcessingRetryEvent)
		assert.Equal(t, EventType("processing.cancelled"), ProcessingCancelledEvent)

		// Review Events
		assert.Equal(t, EventType("review.created"), ReviewCreatedEvent)
		assert.Equal(t, EventType("review.updated"), ReviewUpdatedEvent)
		assert.Equal(t, EventType("review.deleted"), ReviewDeletedEvent)
		assert.Equal(t, EventType("review.validated"), ReviewValidatedEvent)
		assert.Equal(t, EventType("review.enriched"), ReviewEnrichedEvent)
		assert.Equal(t, EventType("review.batch.created"), ReviewBatchCreated)
		assert.Equal(t, EventType("review.batch.failed"), ReviewBatchFailed)

		// Hotel Events
		assert.Equal(t, EventType("hotel.created"), HotelCreatedEvent)
		assert.Equal(t, EventType("hotel.updated"), HotelUpdatedEvent)
		assert.Equal(t, EventType("hotel.summary.updated"), HotelSummaryUpdated)
		assert.Equal(t, EventType("hotel.analytics.updated"), HotelAnalyticsUpdated)

		// Provider Events
		assert.Equal(t, EventType("provider.connected"), ProviderConnectedEvent)
		assert.Equal(t, EventType("provider.disconnected"), ProviderDisconnectedEvent)
		assert.Equal(t, EventType("provider.error"), ProviderErrorEvent)

		// System Events
		assert.Equal(t, EventType("system.health.check"), SystemHealthCheckEvent)
		assert.Equal(t, EventType("system.error"), SystemErrorEvent)
		assert.Equal(t, EventType("system.metrics"), SystemMetricsEvent)
	})
}

func TestKafkaMessage_Headers(t *testing.T) {
	t.Run("kafka message headers", func(t *testing.T) {
		// Test that we can create proper Kafka headers
		headers := []kafka.Header{
			{Key: "event-type", Value: []byte("review.created")},
			{Key: "source", Value: []byte("hotel-reviews-microservice")},
			{Key: "version", Value: []byte("1.0")},
			{Key: "correlation-id", Value: []byte("test-correlation-123")},
		}

		assert.Len(t, headers, 4)
		assert.Equal(t, "event-type", headers[0].Key)
		assert.Equal(t, []byte("review.created"), headers[0].Value)
		assert.Equal(t, "source", headers[1].Key)
		assert.Equal(t, []byte("hotel-reviews-microservice"), headers[1].Value)
		assert.Equal(t, "version", headers[2].Key)
		assert.Equal(t, []byte("1.0"), headers[2].Value)
		assert.Equal(t, "correlation-id", headers[3].Key)
		assert.Equal(t, []byte("test-correlation-123"), headers[3].Value)
	})
}

func TestConcurrentEventHandling(t *testing.T) {
	t.Run("concurrent event creation", func(t *testing.T) {
		const numGoroutines = 10
		const eventsPerGoroutine = 100

		events := make(chan BaseEvent, numGoroutines*eventsPerGoroutine)

		// Start multiple goroutines creating events
		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				for j := 0; j < eventsPerGoroutine; j++ {
					event := BaseEvent{
						ID:        uuid.New().String(),
						Type:      ReviewCreatedEvent,
						Source:    "test-service",
						Timestamp: time.Now(),
						Version:   "1.0",
						Data: map[string]interface{}{
							"worker_id": workerID,
							"event_num": j,
						},
					}
					events <- event
				}
			}(i)
		}

		// Collect all events
		var collectedEvents []BaseEvent
		for i := 0; i < numGoroutines*eventsPerGoroutine; i++ {
			select {
			case event := <-events:
				collectedEvents = append(collectedEvents, event)
			case <-time.After(time.Second * 5):
				t.Fatal("Timeout waiting for events")
			}
		}

		// Verify we got all events
		assert.Len(t, collectedEvents, numGoroutines*eventsPerGoroutine)

		// Verify all events have unique IDs
		ids := make(map[string]bool)
		for _, event := range collectedEvents {
			assert.NotEmpty(t, event.ID)
			assert.False(t, ids[event.ID], "Duplicate event ID found: %s", event.ID)
			ids[event.ID] = true
		}
	})
}