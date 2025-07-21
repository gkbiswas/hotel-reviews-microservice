package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType represents different types of events in the system
type EventType string

const (
	// User events
	UserRegisteredEvent EventType = "user.registered"
	UserUpdatedEvent    EventType = "user.updated"
	UserDeletedEvent    EventType = "user.deleted"

	// Hotel events
	HotelCreatedEvent EventType = "hotel.created"
	HotelUpdatedEvent EventType = "hotel.updated"
	HotelDeletedEvent EventType = "hotel.deleted"

	// Review events
	ReviewCreatedEvent   EventType = "review.created"
	ReviewUpdatedEvent   EventType = "review.updated"
	ReviewDeletedEvent   EventType = "review.deleted"
	ReviewModeratedEvent EventType = "review.moderated"

	// File events
	FileUploadedEvent  EventType = "file.uploaded"
	FileProcessedEvent EventType = "file.processed"
	FileDeletedEvent   EventType = "file.deleted"

	// Analytics events
	PageViewEvent       EventType = "analytics.page_view"
	SearchPerformedEvent EventType = "analytics.search_performed"
	BookingAttemptEvent EventType = "analytics.booking_attempt"

	// System events
	SystemHealthEvent   EventType = "system.health"
	SystemErrorEvent    EventType = "system.error"
	SystemMetricEvent   EventType = "system.metric"
)

// Event represents a domain event in the system
type Event struct {
	ID            string                 `json:"id"`
	Type          EventType              `json:"type"`
	AggregateID   string                 `json:"aggregate_id"`
	AggregateType string                 `json:"aggregate_type"`
	Version       int64                  `json:"version"`
	Data          interface{}            `json:"data"`
	Metadata      map[string]interface{} `json:"metadata"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlation_id"`
	CausationID   string                 `json:"causation_id"`
}

// NewEvent creates a new event with required fields
func NewEvent(eventType EventType, aggregateID, aggregateType string, data interface{}) *Event {
	return &Event{
		ID:            uuid.New().String(),
		Type:          eventType,
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Version:       1,
		Data:          data,
		Metadata:      make(map[string]interface{}),
		Timestamp:     time.Now().UTC(),
		CorrelationID: uuid.New().String(),
	}
}

// WithMetadata adds metadata to the event
func (e *Event) WithMetadata(key string, value interface{}) *Event {
	e.Metadata[key] = value
	return e
}

// WithCorrelationID sets the correlation ID for event tracing
func (e *Event) WithCorrelationID(correlationID string) *Event {
	e.CorrelationID = correlationID
	return e
}

// WithCausationID sets the causation ID for event causality
func (e *Event) WithCausationID(causationID string) *Event {
	e.CausationID = causationID
	return e
}

// ToJSON serializes the event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes an event from JSON
func FromJSON(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return &event, nil
}

// EventHandler defines the interface for handling events
type EventHandler interface {
	Handle(ctx context.Context, event *Event) error
	GetEventTypes() []EventType
}

// EventBus defines the interface for publishing and subscribing to events
type EventBus interface {
	Publish(ctx context.Context, event *Event) error
	Subscribe(handler EventHandler) error
	Unsubscribe(handler EventHandler) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// MemoryEventBus is an in-memory implementation of EventBus for testing
type MemoryEventBus struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
}

// NewMemoryEventBus creates a new in-memory event bus
func NewMemoryEventBus() *MemoryEventBus {
	return &MemoryEventBus{
		handlers: make(map[EventType][]EventHandler),
		stopCh:   make(chan struct{}),
	}
}

// Publish publishes an event to all registered handlers
func (bus *MemoryEventBus) Publish(ctx context.Context, event *Event) error {
	bus.mu.RLock()
	handlers, exists := bus.handlers[event.Type]
	bus.mu.RUnlock()

	if !exists {
		return nil // No handlers registered for this event type
	}

	// Handle events asynchronously
	for _, handler := range handlers {
		go func(h EventHandler) {
			if err := h.Handle(ctx, event); err != nil {
				// Log error - in production, you might want to implement retry logic
				fmt.Printf("Error handling event %s: %v\n", event.Type, err)
			}
		}(handler)
	}

	return nil
}

// Subscribe registers an event handler for specific event types
func (bus *MemoryEventBus) Subscribe(handler EventHandler) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	eventTypes := handler.GetEventTypes()
	for _, eventType := range eventTypes {
		bus.handlers[eventType] = append(bus.handlers[eventType], handler)
	}

	return nil
}

// Unsubscribe removes an event handler
func (bus *MemoryEventBus) Unsubscribe(handler EventHandler) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	eventTypes := handler.GetEventTypes()
	for _, eventType := range eventTypes {
		handlers := bus.handlers[eventType]
		for i, h := range handlers {
			if h == handler {
				bus.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}

	return nil
}

// Start starts the event bus
func (bus *MemoryEventBus) Start(ctx context.Context) error {
	bus.mu.Lock()
	bus.running = true
	bus.mu.Unlock()
	return nil
}

// Stop stops the event bus
func (bus *MemoryEventBus) Stop(ctx context.Context) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	if bus.running {
		close(bus.stopCh)
		bus.running = false
	}

	return nil
}

// EventStore defines the interface for storing and retrieving events
type EventStore interface {
	Append(ctx context.Context, streamID string, events []*Event) error
	Read(ctx context.Context, streamID string, fromVersion int64) ([]*Event, error)
	ReadAll(ctx context.Context, fromPosition int64) ([]*Event, error)
	GetLatestVersion(ctx context.Context, streamID string) (int64, error)
}

// EventStream represents a stream of events for a specific aggregate
type EventStream struct {
	StreamID string   `json:"stream_id"`
	Version  int64    `json:"version"`
	Events   []*Event `json:"events"`
}

// MemoryEventStore is an in-memory implementation of EventStore for testing
type MemoryEventStore struct {
	streams map[string][]*Event
	mu      sync.RWMutex
}

// NewMemoryEventStore creates a new in-memory event store
func NewMemoryEventStore() *MemoryEventStore {
	return &MemoryEventStore{
		streams: make(map[string][]*Event),
	}
}

// Append appends events to a stream
func (store *MemoryEventStore) Append(ctx context.Context, streamID string, events []*Event) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	currentVersion := int64(0)
	if existingEvents, exists := store.streams[streamID]; exists {
		currentVersion = int64(len(existingEvents))
	}

	// Set version numbers for new events
	for i, event := range events {
		event.Version = currentVersion + int64(i) + 1
	}

	store.streams[streamID] = append(store.streams[streamID], events...)
	return nil
}

// Read reads events from a stream starting from a specific version
func (store *MemoryEventStore) Read(ctx context.Context, streamID string, fromVersion int64) ([]*Event, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	events, exists := store.streams[streamID]
	if !exists {
		return nil, fmt.Errorf("stream %s not found", streamID)
	}

	if fromVersion <= 0 {
		fromVersion = 1
	}

	var result []*Event
	for _, event := range events {
		if event.Version >= fromVersion {
			result = append(result, event)
		}
	}

	return result, nil
}

// ReadAll reads all events from all streams starting from a specific position
func (store *MemoryEventStore) ReadAll(ctx context.Context, fromPosition int64) ([]*Event, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	var allEvents []*Event
	for _, events := range store.streams {
		allEvents = append(allEvents, events...)
	}

	// Sort by timestamp
	// In a real implementation, you'd want a more sophisticated ordering mechanism
	var result []*Event
	position := int64(0)
	for _, event := range allEvents {
		position++
		if position >= fromPosition {
			result = append(result, event)
		}
	}

	return result, nil
}

// GetLatestVersion returns the latest version of a stream
func (store *MemoryEventStore) GetLatestVersion(ctx context.Context, streamID string) (int64, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	events, exists := store.streams[streamID]
	if !exists {
		return 0, nil
	}

	return int64(len(events)), nil
}

// Saga represents a distributed transaction coordinator
type Saga interface {
	Start(ctx context.Context, triggerEvent *Event) error
	Handle(ctx context.Context, event *Event) error
	GetSagaID() string
	GetStatus() SagaStatus
}

// SagaStatus represents the status of a saga
type SagaStatus string

const (
	SagaStatusPending    SagaStatus = "pending"
	SagaStatusInProgress SagaStatus = "in_progress"
	SagaStatusCompleted  SagaStatus = "completed"
	SagaStatusFailed     SagaStatus = "failed"
	SagaStatusCancelled  SagaStatus = "cancelled"
)

// SagaStep represents a step in a saga
type SagaStep struct {
	StepID      string
	Execute     func(ctx context.Context, data interface{}) error
	Compensate  func(ctx context.Context, data interface{}) error
	Description string
}

// SagaManager manages running sagas
type SagaManager struct {
	sagas map[string]Saga
	mu    sync.RWMutex
}

// NewSagaManager creates a new saga manager
func NewSagaManager() *SagaManager {
	return &SagaManager{
		sagas: make(map[string]Saga),
	}
}

// StartSaga starts a new saga
func (sm *SagaManager) StartSaga(ctx context.Context, saga Saga, triggerEvent *Event) error {
	sm.mu.Lock()
	sm.sagas[saga.GetSagaID()] = saga
	sm.mu.Unlock()

	return saga.Start(ctx, triggerEvent)
}

// HandleEvent routes events to appropriate sagas
func (sm *SagaManager) HandleEvent(ctx context.Context, event *Event) error {
	sm.mu.RLock()
	sagas := make([]Saga, 0, len(sm.sagas))
	for _, saga := range sm.sagas {
		sagas = append(sagas, saga)
	}
	sm.mu.RUnlock()

	for _, saga := range sagas {
		if err := saga.Handle(ctx, event); err != nil {
			return fmt.Errorf("saga %s failed to handle event: %w", saga.GetSagaID(), err)
		}
	}

	return nil
}

// RemoveCompletedSagas removes completed or failed sagas
func (sm *SagaManager) RemoveCompletedSagas() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, saga := range sm.sagas {
		status := saga.GetStatus()
		if status == SagaStatusCompleted || status == SagaStatusFailed || status == SagaStatusCancelled {
			delete(sm.sagas, id)
		}
	}
}

// Projection defines the interface for event projections
type Projection interface {
	Handle(ctx context.Context, event *Event) error
	GetProjectionName() string
	GetLastProcessedEvent() int64
	SetLastProcessedEvent(position int64)
}

// ProjectionManager manages event projections
type ProjectionManager struct {
	projections []Projection
	eventStore  EventStore
	mu          sync.RWMutex
	running     bool
	stopCh      chan struct{}
}

// NewProjectionManager creates a new projection manager
func NewProjectionManager(eventStore EventStore) *ProjectionManager {
	return &ProjectionManager{
		eventStore: eventStore,
		stopCh:     make(chan struct{}),
	}
}

// AddProjection adds a projection to be managed
func (pm *ProjectionManager) AddProjection(projection Projection) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.projections = append(pm.projections, projection)
}

// Start starts processing events for all projections
func (pm *ProjectionManager) Start(ctx context.Context) error {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return fmt.Errorf("projection manager is already running")
	}
	pm.running = true
	pm.mu.Unlock()

	go pm.processEvents(ctx)
	return nil
}

// Stop stops the projection manager
func (pm *ProjectionManager) Stop(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		close(pm.stopCh)
		pm.running = false
	}

	return nil
}

// processEvents continuously processes events for all projections
func (pm *ProjectionManager) processEvents(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second) // Poll every second
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.processProjections(ctx)
		}
	}
}

// processProjections processes events for all projections
func (pm *ProjectionManager) processProjections(ctx context.Context) {
	pm.mu.RLock()
	projections := make([]Projection, len(pm.projections))
	copy(projections, pm.projections)
	pm.mu.RUnlock()

	for _, projection := range projections {
		if err := pm.processProjection(ctx, projection); err != nil {
			fmt.Printf("Error processing projection %s: %v\n", projection.GetProjectionName(), err)
		}
	}
}

// processProjection processes events for a single projection
func (pm *ProjectionManager) processProjection(ctx context.Context, projection Projection) error {
	lastProcessed := projection.GetLastProcessedEvent()
	events, err := pm.eventStore.ReadAll(ctx, lastProcessed+1)
	if err != nil {
		return fmt.Errorf("failed to read events: %w", err)
	}

	for i, event := range events {
		if err := projection.Handle(ctx, event); err != nil {
			return fmt.Errorf("projection failed to handle event: %w", err)
		}
		projection.SetLastProcessedEvent(lastProcessed + int64(i) + 1)
	}

	return nil
}

// OutboxPattern implements the transactional outbox pattern
type OutboxEvent struct {
	ID        string    `json:"id"`
	EventData []byte    `json:"event_data"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
}

// OutboxRepository defines the interface for outbox persistence
type OutboxRepository interface {
	Save(ctx context.Context, event *OutboxEvent) error
	GetUnpublished(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkAsPublished(ctx context.Context, eventID string) error
	Delete(ctx context.Context, eventID string) error
}

// OutboxPublisher publishes events from the outbox
type OutboxPublisher struct {
	repository OutboxRepository
	eventBus   EventBus
	running    bool
	stopCh     chan struct{}
	mu         sync.RWMutex
}

// NewOutboxPublisher creates a new outbox publisher
func NewOutboxPublisher(repository OutboxRepository, eventBus EventBus) *OutboxPublisher {
	return &OutboxPublisher{
		repository: repository,
		eventBus:   eventBus,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the outbox publisher
func (op *OutboxPublisher) Start(ctx context.Context) error {
	op.mu.Lock()
	if op.running {
		op.mu.Unlock()
		return fmt.Errorf("outbox publisher is already running")
	}
	op.running = true
	op.mu.Unlock()

	go op.publishLoop(ctx)
	return nil
}

// Stop stops the outbox publisher
func (op *OutboxPublisher) Stop(ctx context.Context) error {
	op.mu.Lock()
	defer op.mu.Unlock()

	if op.running {
		close(op.stopCh)
		op.running = false
	}

	return nil
}

// publishLoop continuously publishes unpublished events
func (op *OutboxPublisher) publishLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-op.stopCh:
			return
		case <-ticker.C:
			if err := op.publishPendingEvents(ctx); err != nil {
				fmt.Printf("Error publishing outbox events: %v\n", err)
			}
		}
	}
}

// publishPendingEvents publishes all pending events from the outbox
func (op *OutboxPublisher) publishPendingEvents(ctx context.Context) error {
	events, err := op.repository.GetUnpublished(ctx, 100) // Process in batches of 100
	if err != nil {
		return fmt.Errorf("failed to get unpublished events: %w", err)
	}

	for _, outboxEvent := range events {
		event, err := FromJSON(outboxEvent.EventData)
		if err != nil {
			fmt.Printf("Failed to deserialize event %s: %v\n", outboxEvent.ID, err)
			continue
		}

		if err := op.eventBus.Publish(ctx, event); err != nil {
			fmt.Printf("Failed to publish event %s: %v\n", outboxEvent.ID, err)
			continue
		}

		if err := op.repository.MarkAsPublished(ctx, outboxEvent.ID); err != nil {
			fmt.Printf("Failed to mark event %s as published: %v\n", outboxEvent.ID, err)
			continue
		}
	}

	return nil
}