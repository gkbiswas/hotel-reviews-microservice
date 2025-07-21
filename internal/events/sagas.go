package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ReviewProcessingSaga handles the complete review processing workflow
type ReviewProcessingSaga struct {
	ID         string
	Status     SagaStatus
	Steps      []SagaStep
	Data       map[string]interface{}
	logger     *slog.Logger
	mu         sync.RWMutex
	
	// Services
	reviewService      ReviewService
	moderationService  ModerationService
	analyticsService   AnalyticsService
	notificationService NotificationService
	searchService      SearchService
}

// ReviewService defines the interface for review operations
type ReviewService interface {
	GetReview(ctx context.Context, reviewID string) (*ReviewData, error)
	UpdateReviewStatus(ctx context.Context, reviewID string, status string) error
	DeleteReview(ctx context.Context, reviewID string) error
}

// ModerationService defines the interface for content moderation
type ModerationService interface {
	ModerateContent(ctx context.Context, content string) (*ModerationResult, error)
	ApproveContent(ctx context.Context, contentID string) error
	RejectContent(ctx context.Context, contentID string, reason string) error
}

// ReviewData represents review information
type ReviewData struct {
	ID      string  `json:"id"`
	HotelID string  `json:"hotel_id"`
	UserID  string  `json:"user_id"`
	Content string  `json:"content"`
	Rating  float64 `json:"rating"`
	Status  string  `json:"status"`
}

// ModerationResult represents the result of content moderation
type ModerationResult struct {
	Approved     bool     `json:"approved"`
	Confidence   float64  `json:"confidence"`
	Reasons      []string `json:"reasons"`
	SentimentScore float64 `json:"sentiment_score"`
}

// NewReviewProcessingSaga creates a new review processing saga
func NewReviewProcessingSaga(
	reviewService ReviewService,
	moderationService ModerationService,
	analyticsService AnalyticsService,
	notificationService NotificationService,
	searchService SearchService,
	logger *slog.Logger,
) *ReviewProcessingSaga {
	sagaID := uuid.New().String()
	
	saga := &ReviewProcessingSaga{
		ID:                  sagaID,
		Status:              SagaStatusPending,
		Data:                make(map[string]interface{}),
		logger:              logger,
		reviewService:       reviewService,
		moderationService:   moderationService,
		analyticsService:    analyticsService,
		notificationService: notificationService,
		searchService:       searchService,
	}
	
	// Define saga steps
	saga.Steps = []SagaStep{
		{
			StepID:      "validate_review",
			Description: "Validate review data",
			Execute:     saga.validateReview,
			Compensate:  saga.compensateValidateReview,
		},
		{
			StepID:      "moderate_content",
			Description: "Moderate review content",
			Execute:     saga.moderateContent,
			Compensate:  saga.compensateModeratecontent,
		},
		{
			StepID:      "update_analytics",
			Description: "Update analytics with review data",
			Execute:     saga.updateAnalytics,
			Compensate:  saga.compensateUpdateAnalytics,
		},
		{
			StepID:      "index_review",
			Description: "Index review for search",
			Execute:     saga.indexReview,
			Compensate:  saga.compensateIndexReview,
		},
		{
			StepID:      "send_notifications",
			Description: "Send relevant notifications",
			Execute:     saga.sendNotifications,
			Compensate:  saga.compensateSendNotifications,
		},
		{
			StepID:      "finalize_review",
			Description: "Finalize review processing",
			Execute:     saga.finalizeReview,
			Compensate:  saga.compensateFinalizeReview,
		},
	}
	
	return saga
}

// Start initiates the saga with a trigger event
func (s *ReviewProcessingSaga) Start(ctx context.Context, triggerEvent *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.Status != SagaStatusPending {
		return fmt.Errorf("saga %s is not in pending status", s.ID)
	}
	
	s.logger.Info("Starting review processing saga", 
		"saga_id", s.ID, 
		"trigger_event", triggerEvent.ID)
	
	// Extract review data from trigger event
	if triggerEvent.Type != ReviewCreatedEvent {
		return fmt.Errorf("invalid trigger event type: %s", triggerEvent.Type)
	}
	
	s.Data["review_id"] = triggerEvent.AggregateID
	s.Data["trigger_event"] = triggerEvent
	s.Status = SagaStatusInProgress
	
	// Execute saga steps
	return s.executeSteps(ctx)
}

// Handle processes events related to this saga
func (s *ReviewProcessingSaga) Handle(ctx context.Context, event *Event) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Only handle events with matching correlation ID
	triggerEvent, ok := s.Data["trigger_event"].(*Event)
	if !ok || event.CorrelationID != triggerEvent.CorrelationID {
		return nil
	}
	
	s.logger.Info("Handling event in saga", 
		"saga_id", s.ID, 
		"event_type", event.Type,
		"event_id", event.ID)
	
	// Handle specific events that might affect saga execution
	switch event.Type {
	case SystemErrorEvent:
		return s.handleSystemError(ctx, event)
	default:
		// Log and ignore unhandled events
		s.logger.Debug("Unhandled event in saga", 
			"saga_id", s.ID, 
			"event_type", event.Type)
	}
	
	return nil
}

// GetSagaID returns the saga identifier
func (s *ReviewProcessingSaga) GetSagaID() string {
	return s.ID
}

// GetStatus returns the current saga status
func (s *ReviewProcessingSaga) GetStatus() SagaStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// executeSteps executes all saga steps in order
func (s *ReviewProcessingSaga) executeSteps(ctx context.Context) error {
	for i, step := range s.Steps {
		s.logger.Info("Executing saga step", 
			"saga_id", s.ID, 
			"step_id", step.StepID,
			"step_index", i)
		
		if err := step.Execute(ctx, s.Data); err != nil {
			s.logger.Error("Saga step failed", 
				"saga_id", s.ID, 
				"step_id", step.StepID,
				"error", err)
			
			// Compensate previous steps
			if err := s.compensateSteps(ctx, i-1); err != nil {
				s.logger.Error("Saga compensation failed", 
					"saga_id", s.ID, 
					"error", err)
			}
			
			s.Status = SagaStatusFailed
			return fmt.Errorf("saga step %s failed: %w", step.StepID, err)
		}
		
		s.logger.Info("Saga step completed", 
			"saga_id", s.ID, 
			"step_id", step.StepID)
	}
	
	s.Status = SagaStatusCompleted
	s.logger.Info("Saga completed successfully", "saga_id", s.ID)
	return nil
}

// compensateSteps executes compensation logic for failed steps
func (s *ReviewProcessingSaga) compensateSteps(ctx context.Context, fromIndex int) error {
	for i := fromIndex; i >= 0; i-- {
		step := s.Steps[i]
		s.logger.Info("Compensating saga step", 
			"saga_id", s.ID, 
			"step_id", step.StepID,
			"step_index", i)
		
		if err := step.Compensate(ctx, s.Data); err != nil {
			s.logger.Error("Saga step compensation failed", 
				"saga_id", s.ID, 
				"step_id", step.StepID,
				"error", err)
			return err
		}
	}
	
	return nil
}

// handleSystemError handles system error events
func (s *ReviewProcessingSaga) handleSystemError(ctx context.Context, event *Event) error {
	s.logger.Error("System error detected in saga", 
		"saga_id", s.ID, 
		"error_event", event.ID)
	
	// Mark saga as failed and initiate compensation
	s.Status = SagaStatusFailed
	return s.compensateSteps(ctx, len(s.Steps)-1)
}

// Saga step implementations

func (s *ReviewProcessingSaga) validateReview(ctx context.Context, data map[string]interface{}) error {
	reviewID, ok := data["review_id"].(string)
	if !ok {
		return fmt.Errorf("missing review_id in saga data")
	}
	
	review, err := s.reviewService.GetReview(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("failed to get review: %w", err)
	}
	
	// Validate review data
	if review.Content == "" {
		return fmt.Errorf("review content is empty")
	}
	
	if review.Rating < 1 || review.Rating > 5 {
		return fmt.Errorf("invalid rating: %f", review.Rating)
	}
	
	// Store review data for subsequent steps
	data["review"] = review
	
	s.logger.Info("Review validation completed", 
		"saga_id", s.ID, 
		"review_id", reviewID)
	
	return nil
}

func (s *ReviewProcessingSaga) compensateValidateReview(ctx context.Context, data map[string]interface{}) error {
	// No compensation needed for validation
	return nil
}

func (s *ReviewProcessingSaga) moderateContent(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return fmt.Errorf("missing review data in saga")
	}
	
	result, err := s.moderationService.ModerateContent(ctx, review.Content)
	if err != nil {
		return fmt.Errorf("content moderation failed: %w", err)
	}
	
	data["moderation_result"] = result
	
	// Update review status based on moderation result
	var status string
	if result.Approved {
		status = "approved"
		if err := s.moderationService.ApproveContent(ctx, review.ID); err != nil {
			return fmt.Errorf("failed to approve content: %w", err)
		}
	} else {
		status = "rejected"
		reason := "Content did not pass moderation"
		if len(result.Reasons) > 0 {
			reason = result.Reasons[0]
		}
		if err := s.moderationService.RejectContent(ctx, review.ID, reason); err != nil {
			return fmt.Errorf("failed to reject content: %w", err)
		}
	}
	
	if err := s.reviewService.UpdateReviewStatus(ctx, review.ID, status); err != nil {
		return fmt.Errorf("failed to update review status: %w", err)
	}
	
	s.logger.Info("Content moderation completed", 
		"saga_id", s.ID, 
		"review_id", review.ID,
		"approved", result.Approved)
	
	return nil
}

func (s *ReviewProcessingSaga) compensateModeratecontent(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return nil // No review to compensate
	}
	
	// Reset review status to pending
	return s.reviewService.UpdateReviewStatus(ctx, review.ID, "pending")
}

func (s *ReviewProcessingSaga) updateAnalytics(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return fmt.Errorf("missing review data in saga")
	}
	
	moderationResult, ok := data["moderation_result"].(*ModerationResult)
	if !ok {
		return fmt.Errorf("missing moderation result in saga")
	}
	
	// Only update analytics for approved reviews
	if !moderationResult.Approved {
		s.logger.Info("Skipping analytics update for rejected review", 
			"saga_id", s.ID, 
			"review_id", review.ID)
		return nil
	}
	
	if err := s.analyticsService.UpdateHotelMetrics(ctx, review.HotelID, review.Rating); err != nil {
		return fmt.Errorf("failed to update hotel metrics: %w", err)
	}
	
	if err := s.analyticsService.TrackUserActivity(ctx, review.UserID, "review_approved"); err != nil {
		return fmt.Errorf("failed to track user activity: %w", err)
	}
	
	s.logger.Info("Analytics update completed", 
		"saga_id", s.ID, 
		"review_id", review.ID)
	
	return nil
}

func (s *ReviewProcessingSaga) compensateUpdateAnalytics(ctx context.Context, data map[string]interface{}) error {
	// In a real implementation, you would reverse the analytics updates
	// This might involve decrementing counters, recalculating averages, etc.
	return nil
}

func (s *ReviewProcessingSaga) indexReview(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return fmt.Errorf("missing review data in saga")
	}
	
	moderationResult, ok := data["moderation_result"].(*ModerationResult)
	if !ok {
		return fmt.Errorf("missing moderation result in saga")
	}
	
	// Only index approved reviews
	if !moderationResult.Approved {
		s.logger.Info("Skipping search indexing for rejected review", 
			"saga_id", s.ID, 
			"review_id", review.ID)
		return nil
	}
	
	// Convert ReviewData to domain.Review for indexing
	domainReview := &domain.Review{
		ID:      review.ID,
		HotelID: review.HotelID,
		UserID:  review.UserID,
		Content: review.Content,
		Rating:  review.Rating,
	}
	
	if err := s.searchService.IndexReview(ctx, domainReview); err != nil {
		return fmt.Errorf("failed to index review: %w", err)
	}
	
	s.logger.Info("Search indexing completed", 
		"saga_id", s.ID, 
		"review_id", review.ID)
	
	return nil
}

func (s *ReviewProcessingSaga) compensateIndexReview(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return nil
	}
	
	// Remove review from search index
	return s.searchService.DeleteReviewIndex(ctx, review.ID)
}

func (s *ReviewProcessingSaga) sendNotifications(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return fmt.Errorf("missing review data in saga")
	}
	
	moderationResult, ok := data["moderation_result"].(*ModerationResult)
	if !ok {
		return fmt.Errorf("missing moderation result in saga")
	}
	
	// Send notification to user about moderation result
	if moderationResult.Approved {
		message := fmt.Sprintf("Your review for hotel %s has been approved and is now live!", review.HotelID)
		if err := s.notificationService.SendPushNotification(ctx, review.UserID, "Review Approved", message); err != nil {
			s.logger.Error("Failed to send approval notification", 
				"saga_id", s.ID, 
				"error", err)
			// Don't fail the saga for notification errors
		}
	} else {
		message := "Your review did not pass moderation. Please review our content guidelines and try again."
		if err := s.notificationService.SendPushNotification(ctx, review.UserID, "Review Rejected", message); err != nil {
			s.logger.Error("Failed to send rejection notification", 
				"saga_id", s.ID, 
				"error", err)
		}
	}
	
	s.logger.Info("Notifications sent", 
		"saga_id", s.ID, 
		"review_id", review.ID)
	
	return nil
}

func (s *ReviewProcessingSaga) compensateSendNotifications(ctx context.Context, data map[string]interface{}) error {
	// No compensation needed for notifications
	return nil
}

func (s *ReviewProcessingSaga) finalizeReview(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return fmt.Errorf("missing review data in saga")
	}
	
	moderationResult, ok := data["moderation_result"].(*ModerationResult)
	if !ok {
		return fmt.Errorf("missing moderation result in saga")
	}
	
	finalStatus := "approved"
	if !moderationResult.Approved {
		finalStatus = "rejected"
	}
	
	if err := s.reviewService.UpdateReviewStatus(ctx, review.ID, finalStatus); err != nil {
		return fmt.Errorf("failed to finalize review status: %w", err)
	}
	
	s.logger.Info("Review processing finalized", 
		"saga_id", s.ID, 
		"review_id", review.ID,
		"final_status", finalStatus)
	
	return nil
}

func (s *ReviewProcessingSaga) compensateFinalizeReview(ctx context.Context, data map[string]interface{}) error {
	review, ok := data["review"].(*ReviewData)
	if !ok {
		return nil
	}
	
	// Reset review to pending status
	return s.reviewService.UpdateReviewStatus(ctx, review.ID, "pending")
}

// HotelOnboardingSaga handles the complete hotel onboarding workflow
type HotelOnboardingSaga struct {
	ID         string
	Status     SagaStatus
	Steps      []SagaStep
	Data       map[string]interface{}
	logger     *slog.Logger
	mu         sync.RWMutex
	
	// Services
	hotelService        HotelService
	validationService   ValidationService
	searchService       SearchService
	notificationService NotificationService
}

// HotelService defines the interface for hotel operations
type HotelService interface {
	GetHotel(ctx context.Context, hotelID string) (*HotelData, error)
	UpdateHotelStatus(ctx context.Context, hotelID string, status string) error
	ValidateHotelData(ctx context.Context, hotelID string) error
}

// ValidationService defines the interface for validation operations
type ValidationService interface {
	ValidateHotelAddress(ctx context.Context, address string) error
	ValidateHotelContact(ctx context.Context, contact string) error
	ValidateHotelLicense(ctx context.Context, license string) error
}

// HotelData represents hotel information
type HotelData struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	ContactInfo string  `json:"contact_info"`
	License     string  `json:"license"`
	Status      string  `json:"status"`
}

// NewHotelOnboardingSaga creates a new hotel onboarding saga
func NewHotelOnboardingSaga(
	hotelService HotelService,
	validationService ValidationService,
	searchService SearchService,
	notificationService NotificationService,
	logger *slog.Logger,
) *HotelOnboardingSaga {
	sagaID := uuid.New().String()
	
	saga := &HotelOnboardingSaga{
		ID:                  sagaID,
		Status:              SagaStatusPending,
		Data:                make(map[string]interface{}),
		logger:              logger,
		hotelService:        hotelService,
		validationService:   validationService,
		searchService:       searchService,
		notificationService: notificationService,
	}
	
	// Define saga steps
	saga.Steps = []SagaStep{
		{
			StepID:      "validate_hotel_data",
			Description: "Validate hotel information",
			Execute:     saga.validateHotelData,
			Compensate:  saga.compensateValidateHotelData,
		},
		{
			StepID:      "validate_address",
			Description: "Validate hotel address",
			Execute:     saga.validateAddress,
			Compensate:  saga.compensateValidateAddress,
		},
		{
			StepID:      "validate_contact",
			Description: "Validate hotel contact information",
			Execute:     saga.validateContact,
			Compensate:  saga.compensateValidateContact,
		},
		{
			StepID:      "validate_license",
			Description: "Validate hotel business license",
			Execute:     saga.validateLicense,
			Compensate:  saga.compensateValidateLicense,
		},
		{
			StepID:      "index_hotel",
			Description: "Index hotel for search",
			Execute:     saga.indexHotel,
			Compensate:  saga.compensateIndexHotel,
		},
		{
			StepID:      "notify_completion",
			Description: "Send onboarding completion notification",
			Execute:     saga.notifyCompletion,
			Compensate:  saga.compensateNotifyCompletion,
		},
	}
	
	return saga
}

// Start initiates the hotel onboarding saga
func (s *HotelOnboardingSaga) Start(ctx context.Context, triggerEvent *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.Status != SagaStatusPending {
		return fmt.Errorf("saga %s is not in pending status", s.ID)
	}
	
	s.logger.Info("Starting hotel onboarding saga", 
		"saga_id", s.ID, 
		"trigger_event", triggerEvent.ID)
	
	if triggerEvent.Type != HotelCreatedEvent {
		return fmt.Errorf("invalid trigger event type: %s", triggerEvent.Type)
	}
	
	s.Data["hotel_id"] = triggerEvent.AggregateID
	s.Data["trigger_event"] = triggerEvent
	s.Status = SagaStatusInProgress
	
	return s.executeSteps(ctx)
}

// Handle processes events related to this saga
func (s *HotelOnboardingSaga) Handle(ctx context.Context, event *Event) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	triggerEvent, ok := s.Data["trigger_event"].(*Event)
	if !ok || event.CorrelationID != triggerEvent.CorrelationID {
		return nil
	}
	
	s.logger.Info("Handling event in hotel onboarding saga", 
		"saga_id", s.ID, 
		"event_type", event.Type)
	
	return nil
}

// GetSagaID returns the saga identifier
func (s *HotelOnboardingSaga) GetSagaID() string {
	return s.ID
}

// GetStatus returns the current saga status
func (s *HotelOnboardingSaga) GetStatus() SagaStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// executeSteps executes all saga steps in order
func (s *HotelOnboardingSaga) executeSteps(ctx context.Context) error {
	for i, step := range s.Steps {
		s.logger.Info("Executing hotel onboarding step", 
			"saga_id", s.ID, 
			"step_id", step.StepID)
		
		if err := step.Execute(ctx, s.Data); err != nil {
			s.logger.Error("Hotel onboarding step failed", 
				"saga_id", s.ID, 
				"step_id", step.StepID,
				"error", err)
			
			if err := s.compensateSteps(ctx, i-1); err != nil {
				s.logger.Error("Hotel onboarding compensation failed", 
					"saga_id", s.ID, 
					"error", err)
			}
			
			s.Status = SagaStatusFailed
			return fmt.Errorf("hotel onboarding step %s failed: %w", step.StepID, err)
		}
	}
	
	s.Status = SagaStatusCompleted
	s.logger.Info("Hotel onboarding saga completed", "saga_id", s.ID)
	return nil
}

// compensateSteps executes compensation logic for failed steps
func (s *HotelOnboardingSaga) compensateSteps(ctx context.Context, fromIndex int) error {
	for i := fromIndex; i >= 0; i-- {
		step := s.Steps[i]
		if err := step.Compensate(ctx, s.Data); err != nil {
			return err
		}
	}
	return nil
}

// Hotel onboarding saga step implementations
func (s *HotelOnboardingSaga) validateHotelData(ctx context.Context, data map[string]interface{}) error {
	hotelID, ok := data["hotel_id"].(string)
	if !ok {
		return fmt.Errorf("missing hotel_id in saga data")
	}
	
	hotel, err := s.hotelService.GetHotel(ctx, hotelID)
	if err != nil {
		return fmt.Errorf("failed to get hotel: %w", err)
	}
	
	if err := s.hotelService.ValidateHotelData(ctx, hotelID); err != nil {
		return fmt.Errorf("hotel data validation failed: %w", err)
	}
	
	data["hotel"] = hotel
	return nil
}

func (s *HotelOnboardingSaga) compensateValidateHotelData(ctx context.Context, data map[string]interface{}) error {
	return nil // No compensation needed
}

func (s *HotelOnboardingSaga) validateAddress(ctx context.Context, data map[string]interface{}) error {
	hotel, ok := data["hotel"].(*HotelData)
	if !ok {
		return fmt.Errorf("missing hotel data")
	}
	
	return s.validationService.ValidateHotelAddress(ctx, hotel.Address)
}

func (s *HotelOnboardingSaga) compensateValidateAddress(ctx context.Context, data map[string]interface{}) error {
	return nil
}

func (s *HotelOnboardingSaga) validateContact(ctx context.Context, data map[string]interface{}) error {
	hotel, ok := data["hotel"].(*HotelData)
	if !ok {
		return fmt.Errorf("missing hotel data")
	}
	
	return s.validationService.ValidateHotelContact(ctx, hotel.ContactInfo)
}

func (s *HotelOnboardingSaga) compensateValidateContact(ctx context.Context, data map[string]interface{}) error {
	return nil
}

func (s *HotelOnboardingSaga) validateLicense(ctx context.Context, data map[string]interface{}) error {
	hotel, ok := data["hotel"].(*HotelData)
	if !ok {
		return fmt.Errorf("missing hotel data")
	}
	
	return s.validationService.ValidateHotelLicense(ctx, hotel.License)
}

func (s *HotelOnboardingSaga) compensateValidateLicense(ctx context.Context, data map[string]interface{}) error {
	return nil
}

func (s *HotelOnboardingSaga) indexHotel(ctx context.Context, data map[string]interface{}) error {
	hotel, ok := data["hotel"].(*HotelData)
	if !ok {
		return fmt.Errorf("missing hotel data")
	}
	
	// Convert to domain model and index
	domainHotel := &domain.Hotel{
		ID:   hotel.ID,
		Name: hotel.Name,
		// ... other fields
	}
	
	return s.searchService.IndexHotel(ctx, domainHotel)
}

func (s *HotelOnboardingSaga) compensateIndexHotel(ctx context.Context, data map[string]interface{}) error {
	hotel, ok := data["hotel"].(*HotelData)
	if !ok {
		return nil
	}
	
	return s.searchService.DeleteHotelIndex(ctx, hotel.ID)
}

func (s *HotelOnboardingSaga) notifyCompletion(ctx context.Context, data map[string]interface{}) error {
	hotel, ok := data["hotel"].(*HotelData)
	if !ok {
		return fmt.Errorf("missing hotel data")
	}
	
	// Update hotel status to active
	if err := s.hotelService.UpdateHotelStatus(ctx, hotel.ID, "active"); err != nil {
		return fmt.Errorf("failed to update hotel status: %w", err)
	}
	
	s.logger.Info("Hotel onboarding completed", 
		"saga_id", s.ID, 
		"hotel_id", hotel.ID)
	
	return nil
}

func (s *HotelOnboardingSaga) compensateNotifyCompletion(ctx context.Context, data map[string]interface{}) error {
	hotel, ok := data["hotel"].(*HotelData)
	if !ok {
		return nil
	}
	
	return s.hotelService.UpdateHotelStatus(ctx, hotel.ID, "pending")
}