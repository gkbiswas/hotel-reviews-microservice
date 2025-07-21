package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gkbiswas/hotel-reviews/internal/domain"
)

// ReviewAnalyticsHandler handles review events for analytics
type ReviewAnalyticsHandler struct {
	analyticsService AnalyticsService
	logger          *slog.Logger
}

// AnalyticsService defines the interface for analytics operations
type AnalyticsService interface {
	UpdateHotelMetrics(ctx context.Context, hotelID string, rating float64) error
	TrackUserActivity(ctx context.Context, userID string, activity string) error
	RecordSearchEvent(ctx context.Context, query string, results int) error
}

// NewReviewAnalyticsHandler creates a new review analytics handler
func NewReviewAnalyticsHandler(analyticsService AnalyticsService, logger *slog.Logger) *ReviewAnalyticsHandler {
	return &ReviewAnalyticsHandler{
		analyticsService: analyticsService,
		logger:          logger,
	}
}

// Handle processes review events for analytics
func (h *ReviewAnalyticsHandler) Handle(ctx context.Context, event *Event) error {
	switch event.Type {
	case ReviewCreatedEvent:
		return h.handleReviewCreated(ctx, event)
	case ReviewUpdatedEvent:
		return h.handleReviewUpdated(ctx, event)
	case ReviewDeletedEvent:
		return h.handleReviewDeleted(ctx, event)
	default:
		return nil // Ignore unhandled events
	}
}

// GetEventTypes returns the event types this handler is interested in
func (h *ReviewAnalyticsHandler) GetEventTypes() []EventType {
	return []EventType{
		ReviewCreatedEvent,
		ReviewUpdatedEvent,
		ReviewDeletedEvent,
	}
}

func (h *ReviewAnalyticsHandler) handleReviewCreated(ctx context.Context, event *Event) error {
	h.logger.Info("Processing review created event for analytics", 
		"event_id", event.ID, 
		"correlation_id", event.CorrelationID)

	// Extract review data from event
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	hotelID, ok := data["hotel_id"].(string)
	if !ok {
		return fmt.Errorf("missing hotel_id in event data")
	}

	rating, ok := data["rating"].(float64)
	if !ok {
		return fmt.Errorf("missing rating in event data")
	}

	userID, ok := data["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id in event data")
	}

	// Update analytics
	if err := h.analyticsService.UpdateHotelMetrics(ctx, hotelID, rating); err != nil {
		h.logger.Error("Failed to update hotel metrics", 
			"error", err, 
			"hotel_id", hotelID)
		return err
	}

	if err := h.analyticsService.TrackUserActivity(ctx, userID, "review_created"); err != nil {
		h.logger.Error("Failed to track user activity", 
			"error", err, 
			"user_id", userID)
		return err
	}

	return nil
}

func (h *ReviewAnalyticsHandler) handleReviewUpdated(ctx context.Context, event *Event) error {
	// Similar implementation for review updates
	h.logger.Info("Processing review updated event for analytics", 
		"event_id", event.ID)
	return nil
}

func (h *ReviewAnalyticsHandler) handleReviewDeleted(ctx context.Context, event *Event) error {
	// Similar implementation for review deletions
	h.logger.Info("Processing review deleted event for analytics", 
		"event_id", event.ID)
	return nil
}

// NotificationHandler handles events that require notifications
type NotificationHandler struct {
	notificationService NotificationService
	logger             *slog.Logger
}

// NotificationService defines the interface for notification operations
type NotificationService interface {
	SendEmail(ctx context.Context, to, subject, body string) error
	SendSMS(ctx context.Context, to, message string) error
	SendPushNotification(ctx context.Context, userID, title, message string) error
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(notificationService NotificationService, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		logger:             logger,
	}
}

// Handle processes events that require notifications
func (h *NotificationHandler) Handle(ctx context.Context, event *Event) error {
	switch event.Type {
	case UserRegisteredEvent:
		return h.handleUserRegistered(ctx, event)
	case ReviewCreatedEvent:
		return h.handleReviewCreated(ctx, event)
	case ReviewModeratedEvent:
		return h.handleReviewModerated(ctx, event)
	default:
		return nil
	}
}

// GetEventTypes returns the event types this handler is interested in
func (h *NotificationHandler) GetEventTypes() []EventType {
	return []EventType{
		UserRegisteredEvent,
		ReviewCreatedEvent,
		ReviewModeratedEvent,
	}
}

func (h *NotificationHandler) handleUserRegistered(ctx context.Context, event *Event) error {
	h.logger.Info("Sending welcome notification for new user", 
		"event_id", event.ID)

	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	email, ok := data["email"].(string)
	if !ok {
		return fmt.Errorf("missing email in event data")
	}

	name, ok := data["name"].(string)
	if !ok {
		name = "User" // Default fallback
	}

	subject := "Welcome to Hotel Reviews!"
	body := fmt.Sprintf("Hi %s,\n\nWelcome to Hotel Reviews! We're excited to have you on board.\n\nBest regards,\nThe Hotel Reviews Team", name)

	return h.notificationService.SendEmail(ctx, email, subject, body)
}

func (h *NotificationHandler) handleReviewCreated(ctx context.Context, event *Event) error {
	// Notify hotel owners about new reviews
	h.logger.Info("Processing review created notification", 
		"event_id", event.ID)
	return nil
}

func (h *NotificationHandler) handleReviewModerated(ctx context.Context, event *Event) error {
	// Notify users about review moderation status
	h.logger.Info("Processing review moderation notification", 
		"event_id", event.ID)
	return nil
}

// SearchIndexHandler handles events that require search index updates
type SearchIndexHandler struct {
	searchService SearchService
	logger       *slog.Logger
}

// SearchService defines the interface for search operations
type SearchService interface {
	IndexHotel(ctx context.Context, hotel *domain.Hotel) error
	UpdateHotelIndex(ctx context.Context, hotelID string, updates map[string]interface{}) error
	DeleteHotelIndex(ctx context.Context, hotelID string) error
	IndexReview(ctx context.Context, review *domain.Review) error
	UpdateReviewIndex(ctx context.Context, reviewID string, updates map[string]interface{}) error
	DeleteReviewIndex(ctx context.Context, reviewID string) error
}

// NewSearchIndexHandler creates a new search index handler
func NewSearchIndexHandler(searchService SearchService, logger *slog.Logger) *SearchIndexHandler {
	return &SearchIndexHandler{
		searchService: searchService,
		logger:       logger,
	}
}

// Handle processes events that require search index updates
func (h *SearchIndexHandler) Handle(ctx context.Context, event *Event) error {
	switch event.Type {
	case HotelCreatedEvent:
		return h.handleHotelCreated(ctx, event)
	case HotelUpdatedEvent:
		return h.handleHotelUpdated(ctx, event)
	case HotelDeletedEvent:
		return h.handleHotelDeleted(ctx, event)
	case ReviewCreatedEvent:
		return h.handleReviewCreated(ctx, event)
	case ReviewUpdatedEvent:
		return h.handleReviewUpdated(ctx, event)
	case ReviewDeletedEvent:
		return h.handleReviewDeleted(ctx, event)
	default:
		return nil
	}
}

// GetEventTypes returns the event types this handler is interested in
func (h *SearchIndexHandler) GetEventTypes() []EventType {
	return []EventType{
		HotelCreatedEvent,
		HotelUpdatedEvent,
		HotelDeletedEvent,
		ReviewCreatedEvent,
		ReviewUpdatedEvent,
		ReviewDeletedEvent,
	}
}

func (h *SearchIndexHandler) handleHotelCreated(ctx context.Context, event *Event) error {
	h.logger.Info("Indexing new hotel for search", 
		"event_id", event.ID)

	// In a real implementation, you would deserialize the hotel data
	// and index it in your search engine (e.g., Elasticsearch)
	return nil
}

func (h *SearchIndexHandler) handleHotelUpdated(ctx context.Context, event *Event) error {
	h.logger.Info("Updating hotel search index", 
		"event_id", event.ID)

	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	hotelID := event.AggregateID
	return h.searchService.UpdateHotelIndex(ctx, hotelID, data)
}

func (h *SearchIndexHandler) handleHotelDeleted(ctx context.Context, event *Event) error {
	h.logger.Info("Removing hotel from search index", 
		"event_id", event.ID)

	hotelID := event.AggregateID
	return h.searchService.DeleteHotelIndex(ctx, hotelID)
}

func (h *SearchIndexHandler) handleReviewCreated(ctx context.Context, event *Event) error {
	h.logger.Info("Indexing new review for search", 
		"event_id", event.ID)
	return nil
}

func (h *SearchIndexHandler) handleReviewUpdated(ctx context.Context, event *Event) error {
	h.logger.Info("Updating review search index", 
		"event_id", event.ID)

	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	reviewID := event.AggregateID
	return h.searchService.UpdateReviewIndex(ctx, reviewID, data)
}

func (h *SearchIndexHandler) handleReviewDeleted(ctx context.Context, event *Event) error {
	h.logger.Info("Removing review from search index", 
		"event_id", event.ID)

	reviewID := event.AggregateID
	return h.searchService.DeleteReviewIndex(ctx, reviewID)
}

// CacheInvalidationHandler handles events that require cache invalidation
type CacheInvalidationHandler struct {
	cacheService CacheService
	logger      *slog.Logger
}

// CacheService defines the interface for cache operations
type CacheService interface {
	Delete(ctx context.Context, key string) error
	DeletePattern(ctx context.Context, pattern string) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// NewCacheInvalidationHandler creates a new cache invalidation handler
func NewCacheInvalidationHandler(cacheService CacheService, logger *slog.Logger) *CacheInvalidationHandler {
	return &CacheInvalidationHandler{
		cacheService: cacheService,
		logger:      logger,
	}
}

// Handle processes events that require cache invalidation
func (h *CacheInvalidationHandler) Handle(ctx context.Context, event *Event) error {
	switch event.Type {
	case HotelUpdatedEvent:
		return h.handleHotelUpdated(ctx, event)
	case ReviewCreatedEvent, ReviewUpdatedEvent, ReviewDeletedEvent:
		return h.handleReviewChanged(ctx, event)
	default:
		return nil
	}
}

// GetEventTypes returns the event types this handler is interested in
func (h *CacheInvalidationHandler) GetEventTypes() []EventType {
	return []EventType{
		HotelUpdatedEvent,
		ReviewCreatedEvent,
		ReviewUpdatedEvent,
		ReviewDeletedEvent,
	}
}

func (h *CacheInvalidationHandler) handleHotelUpdated(ctx context.Context, event *Event) error {
	h.logger.Info("Invalidating hotel cache", 
		"event_id", event.ID, 
		"hotel_id", event.AggregateID)

	hotelID := event.AggregateID
	
	// Invalidate specific hotel cache
	if err := h.cacheService.Delete(ctx, fmt.Sprintf("hotel:%s", hotelID)); err != nil {
		h.logger.Error("Failed to invalidate hotel cache", "error", err)
		return err
	}

	// Invalidate hotel list caches
	if err := h.cacheService.DeletePattern(ctx, "hotels:*"); err != nil {
		h.logger.Error("Failed to invalidate hotel list cache", "error", err)
		return err
	}

	return nil
}

func (h *CacheInvalidationHandler) handleReviewChanged(ctx context.Context, event *Event) error {
	h.logger.Info("Invalidating review cache", 
		"event_id", event.ID, 
		"review_id", event.AggregateID)

	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	hotelID, ok := data["hotel_id"].(string)
	if !ok {
		return fmt.Errorf("missing hotel_id in event data")
	}

	reviewID := event.AggregateID

	// Invalidate specific review cache
	if err := h.cacheService.Delete(ctx, fmt.Sprintf("review:%s", reviewID)); err != nil {
		h.logger.Error("Failed to invalidate review cache", "error", err)
		return err
	}

	// Invalidate hotel reviews cache
	if err := h.cacheService.DeletePattern(ctx, fmt.Sprintf("hotel:%s:reviews:*", hotelID)); err != nil {
		h.logger.Error("Failed to invalidate hotel reviews cache", "error", err)
		return err
	}

	// Invalidate hotel metrics cache (since reviews affect ratings)
	if err := h.cacheService.Delete(ctx, fmt.Sprintf("hotel:%s:metrics", hotelID)); err != nil {
		h.logger.Error("Failed to invalidate hotel metrics cache", "error", err)
		return err
	}

	return nil
}

// AuditLogHandler handles events for audit logging
type AuditLogHandler struct {
	auditService AuditService
	logger      *slog.Logger
}

// AuditService defines the interface for audit operations
type AuditService interface {
	LogEvent(ctx context.Context, event *AuditEvent) error
}

// AuditEvent represents an audit log entry
type AuditEvent struct {
	ID            string                 `json:"id"`
	EventType     string                 `json:"event_type"`
	UserID        string                 `json:"user_id"`
	ResourceType  string                 `json:"resource_type"`
	ResourceID    string                 `json:"resource_id"`
	Action        string                 `json:"action"`
	Details       map[string]interface{} `json:"details"`
	IPAddress     string                 `json:"ip_address"`
	UserAgent     string                 `json:"user_agent"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlation_id"`
}

// NewAuditLogHandler creates a new audit log handler
func NewAuditLogHandler(auditService AuditService, logger *slog.Logger) *AuditLogHandler {
	return &AuditLogHandler{
		auditService: auditService,
		logger:      logger,
	}
}

// Handle processes events for audit logging
func (h *AuditLogHandler) Handle(ctx context.Context, event *Event) error {
	// Convert domain event to audit event
	auditEvent := &AuditEvent{
		ID:            event.ID,
		EventType:     string(event.Type),
		ResourceType:  event.AggregateType,
		ResourceID:    event.AggregateID,
		Details:       event.Metadata,
		Timestamp:     event.Timestamp,
		CorrelationID: event.CorrelationID,
	}

	// Extract user information from metadata if available
	if userID, ok := event.Metadata["user_id"].(string); ok {
		auditEvent.UserID = userID
	}

	if ipAddress, ok := event.Metadata["ip_address"].(string); ok {
		auditEvent.IPAddress = ipAddress
	}

	if userAgent, ok := event.Metadata["user_agent"].(string); ok {
		auditEvent.UserAgent = userAgent
	}

	// Determine action based on event type
	auditEvent.Action = h.getActionFromEventType(event.Type)

	h.logger.Info("Logging audit event", 
		"event_id", event.ID, 
		"event_type", event.Type)

	return h.auditService.LogEvent(ctx, auditEvent)
}

// GetEventTypes returns all event types for comprehensive audit logging
func (h *AuditLogHandler) GetEventTypes() []EventType {
	return []EventType{
		UserRegisteredEvent,
		UserUpdatedEvent,
		UserDeletedEvent,
		HotelCreatedEvent,
		HotelUpdatedEvent,
		HotelDeletedEvent,
		ReviewCreatedEvent,
		ReviewUpdatedEvent,
		ReviewDeletedEvent,
		ReviewModeratedEvent,
		FileUploadedEvent,
		FileProcessedEvent,
		FileDeletedEvent,
	}
}

func (h *AuditLogHandler) getActionFromEventType(eventType EventType) string {
	switch eventType {
	case UserRegisteredEvent, HotelCreatedEvent, ReviewCreatedEvent, FileUploadedEvent:
		return "CREATE"
	case UserUpdatedEvent, HotelUpdatedEvent, ReviewUpdatedEvent, ReviewModeratedEvent:
		return "UPDATE"
	case UserDeletedEvent, HotelDeletedEvent, ReviewDeletedEvent, FileDeletedEvent:
		return "DELETE"
	case FileProcessedEvent:
		return "PROCESS"
	default:
		return "UNKNOWN"
	}
}

// ErrorHandler handles system errors and creates error events
type ErrorHandler struct {
	logger *slog.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// Handle processes system error events
func (h *ErrorHandler) Handle(ctx context.Context, event *Event) error {
	if event.Type != SystemErrorEvent {
		return nil
	}

	h.logger.Error("System error event received", 
		"event_id", event.ID,
		"correlation_id", event.CorrelationID,
		"data", event.Data)

	// In a real implementation, you might:
	// - Send alerts to monitoring systems
	// - Create incident tickets
	// - Trigger automated recovery procedures
	// - Notify on-call engineers

	return nil
}

// GetEventTypes returns the event types this handler is interested in
func (h *ErrorHandler) GetEventTypes() []EventType {
	return []EventType{SystemErrorEvent}
}