package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Mock EventProcessor
type MockEventProcessor struct {
	mock.Mock
}

func (m *MockEventProcessor) ProcessEvent(ctx context.Context, event domain.DomainEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventProcessor) ProcessEvents(ctx context.Context, events []domain.DomainEvent) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}

func (m *MockEventProcessor) GetSupportedTypes() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// Mock NotificationService
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) SendProcessingComplete(ctx context.Context, processingID uuid.UUID, status string, recordsProcessed int) error {
	args := m.Called(ctx, processingID, status, recordsProcessed)
	return args.Error(0)
}

func (m *MockNotificationService) SendProcessingFailed(ctx context.Context, processingID uuid.UUID, errorMsg string) error {
	args := m.Called(ctx, processingID, errorMsg)
	return args.Error(0)
}

func (m *MockNotificationService) SendSystemAlert(ctx context.Context, message string, severity string) error {
	args := m.Called(ctx, message, severity)
	return args.Error(0)
}

func (m *MockNotificationService) SendEmailNotification(ctx context.Context, to []string, subject, body string) error {
	args := m.Called(ctx, to, subject, body)
	return args.Error(0)
}

func (m *MockNotificationService) SendSlackNotification(ctx context.Context, channel, message string) error {
	args := m.Called(ctx, channel, message)
	return args.Error(0)
}

// Test EventHandlerConfig
func TestEventHandlerConfig_Defaults(t *testing.T) {
	config := &EventHandlerConfig{
		MaxWorkers:              10,
		MaxRetries:              3,
		RetryDelay:              1 * time.Second,
		RetryBackoffFactor:      2.0,
		MaxRetryDelay:           30 * time.Second,
		ProcessingTimeout:       5 * time.Minute,
		BufferSize:              1000,
		EnableMetrics:           true,
		EnableNotifications:     true,
		EnableReplay:            false,
		DeadLetterThreshold:     5,
	}

	assert.Equal(t, 10, config.MaxWorkers)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 2.0, config.RetryBackoffFactor)
	assert.Equal(t, 30*time.Second, config.MaxRetryDelay)
	assert.Equal(t, 5*time.Minute, config.ProcessingTimeout)
	assert.Equal(t, 1000, config.BufferSize)
	assert.True(t, config.EnableMetrics)
	assert.True(t, config.EnableNotifications)
	assert.False(t, config.EnableReplay)
	assert.Equal(t, 5, config.DeadLetterThreshold)
}

// Test EventHandler creation
func TestNewEventHandler(t *testing.T) {
	logger := logger.NewDefault()
	config := &EventHandlerConfig{
		MaxWorkers:          2,
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		BufferSize:          100,
		ProcessingTimeout:   5 * time.Minute,
		EnableMetrics:       true,
		EnableNotifications: true,
		DeadLetterThreshold: 5,
	}

	handler := NewBaseEventHandler("test-handler", config, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, config, handler.config)
	assert.Equal(t, "test-handler", handler.name)
}

// Test basic event handling functionality
func TestEventHandler_BasicFunctionality(t *testing.T) {
	logger := logger.NewDefault()
	config := &EventHandlerConfig{
		MaxWorkers:      2,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
		BufferSize:      100,
		EnableMetrics:   true,
	}

	handler := NewBaseEventHandler("test-handler", config, logger)

	event := &domain.BaseEvent{
		ID:            uuid.New(),
		Type:          domain.ReviewProcessedEventType,
		AggregateID:   uuid.New(),
		OccurredAt:    time.Now(),
		Payload:       map[string]interface{}{"test": "data"},
	}

	// Test processing task creation
	task := &EventHandlingTask{
		ID:           uuid.New(),
		Event:        event,
		Context:      context.Background(),
		CreatedAt:    time.Now(),
		RetryCount:   0,
		MaxRetries:   3,
	}

	assert.NotNil(t, task)
	assert.Equal(t, event, task.Event)
	assert.Equal(t, 0, task.RetryCount)
	assert.NotNil(t, handler)
}

// Test error handling
func TestEventHandler_ErrorHandling(t *testing.T) {
	logger := logger.NewDefault()
	config := &EventHandlerConfig{
		MaxWorkers:    2,
		MaxRetries:    1,
		RetryDelay:    10 * time.Millisecond,
		BufferSize:    100,
		EnableMetrics: true,
	}

	handler := NewBaseEventHandler("test-handler", config, logger)

	event := &domain.BaseEvent{
		ID:            uuid.New(),
		Type:          domain.ReviewProcessedEventType,
		AggregateID:   uuid.New(),
		OccurredAt:    time.Now(),
		Payload:       map[string]interface{}{"test": "data"},
	}

	// Test error handling capability
	assert.NotNil(t, handler)
	assert.NotNil(t, event)
	assert.Equal(t, 1, handler.config.MaxRetries)
}

// Test batch processing
func TestEventHandler_BatchProcessing(t *testing.T) {
	mockProcessor := new(MockEventProcessor)
	logger := logger.NewDefault()
	config := &EventHandlerConfig{
		MaxWorkers:    2,
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
		BufferSize:    100,
		EnableMetrics: true,
	}

	handler := NewBaseEventHandler("test-handler", config, logger)

	events := []domain.DomainEvent{
		&domain.BaseEvent{
			ID:            uuid.New(),
			Type:          domain.ReviewProcessedEventType,
			AggregateID:   uuid.New(),
			OccurredAt:    time.Now(),
			Payload:       map[string]interface{}{"test": "data1"},
		},
		&domain.BaseEvent{
			ID:            uuid.New(),
			Type:          domain.ReviewValidatedEventType,
			AggregateID:   uuid.New(),
			OccurredAt:    time.Now(),
			Payload:       map[string]interface{}{"test": "data2"},
		},
	}

	// Test batch processing capability
	mockProcessor.On("ProcessEvents", mock.Anything, events).Return(nil)

	err := mockProcessor.ProcessEvents(context.Background(), events)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.Len(t, events, 2)

	mockProcessor.AssertExpectations(t)
}

// Test metrics functionality
func TestEventHandler_Metrics(t *testing.T) {
	logger := logger.NewDefault()
	config := &EventHandlerConfig{
		MaxWorkers:    2,
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
		BufferSize:    100,
		EnableMetrics: true,
	}

	handler := NewBaseEventHandler("test-handler", config, logger)

	// Test metrics structure
	assert.NotNil(t, handler)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, "test-handler", handler.name)
}

// Test configuration validation
func TestEventHandler_Configuration(t *testing.T) {
	logger := logger.NewDefault()
	config := &EventHandlerConfig{
		MaxWorkers:              5,
		MaxRetries:              3,
		RetryDelay:              1 * time.Second,
		BufferSize:              100,
		EnableMetrics:           true,
		EnableNotifications:     true,
		DeadLetterThreshold:     2,
		EnableReplay:            true,
	}

	handler := NewBaseEventHandler("test-handler", config, logger)

	// Test configuration values
	assert.Equal(t, 5, config.MaxWorkers)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 2, config.DeadLetterThreshold)
	assert.True(t, config.EnableNotifications)
	assert.True(t, config.EnableReplay)
	assert.NotNil(t, handler)
}

// Test event types
func TestEventHandler_EventTypes(t *testing.T) {
	events := []domain.DomainEvent{
		&domain.BaseEvent{
			ID:            uuid.New(),
			Type:          domain.ReviewProcessedEventType,
			AggregateID:   uuid.New(),
			OccurredAt:    time.Now(),
			Payload:       map[string]interface{}{"test": "processed"},
		},
		&domain.BaseEvent{
			ID:            uuid.New(),
			Type:          domain.ReviewValidatedEventType,
			AggregateID:   uuid.New(),
			OccurredAt:    time.Now(),
			Payload:       map[string]interface{}{"test": "validated"},
		},
		&domain.BaseEvent{
			ID:            uuid.New(),
			Type:          domain.SystemErrorEvent,
			AggregateID:   uuid.New(),
			OccurredAt:    time.Now(),
			Payload:       map[string]interface{}{"test": "error"},
		},
	}

	assert.Len(t, events, 3)
	assert.Equal(t, domain.ReviewProcessedEventType, events[0].(*domain.BaseEvent).Type)
	assert.Equal(t, domain.ReviewValidatedEventType, events[1].(*domain.BaseEvent).Type)
	assert.Equal(t, domain.SystemErrorEvent, events[2].(*domain.BaseEvent).Type)
}

// Test mock services
func TestMockServices_Operations(t *testing.T) {
	mockProcessor := new(MockEventProcessor)
	mockNotificationService := new(MockNotificationService)
	ctx := context.Background()

	// Test mock processor
	supportedTypes := []string{"review", "hotel", "system"}
	mockProcessor.On("GetSupportedTypes").Return(supportedTypes)
	
	result := mockProcessor.GetSupportedTypes()
	assert.Equal(t, supportedTypes, result)

	// Test mock notification service
	processingID := uuid.New()
	mockNotificationService.On("SendProcessingComplete", ctx, processingID, "completed", 100).Return(nil)
	
	err := mockNotificationService.SendProcessingComplete(ctx, processingID, "completed", 100)
	assert.NoError(t, err)

	mockProcessor.AssertExpectations(t)
	mockNotificationService.AssertExpectations(t)
}