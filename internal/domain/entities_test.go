package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Test Provider entity
func TestProvider_Creation(t *testing.T) {
	provider := &Provider{
		ID:       uuid.New(),
		Name:     "Booking.com",
		BaseURL:  "https://booking.com",
		IsActive: true,
	}

	assert.NotNil(t, provider)
	assert.NotEqual(t, uuid.Nil, provider.ID)
	assert.Equal(t, "Booking.com", provider.Name)
	assert.Equal(t, "https://booking.com", provider.BaseURL)
	assert.True(t, provider.IsActive)
}

func TestProvider_Validation(t *testing.T) {
	// Test valid provider
	provider := &Provider{
		ID:       uuid.New(),
		Name:     "Test Provider",
		BaseURL:  "https://example.com",
		IsActive: true,
	}

	assert.NotNil(t, provider)
	assert.True(t, len(provider.Name) <= 100)
	assert.Contains(t, provider.BaseURL, "https://")

	// Test provider with empty name
	emptyProvider := &Provider{
		ID:       uuid.New(),
		Name:     "",
		BaseURL:  "https://example.com",
		IsActive: true,
	}

	assert.Equal(t, "", emptyProvider.Name)

	// Test provider with long name
	longName := make([]byte, 150)
	for i := range longName {
		longName[i] = 'a'
	}
	
	longNameProvider := &Provider{
		ID:       uuid.New(),
		Name:     string(longName),
		BaseURL:  "https://example.com",
		IsActive: true,
	}

	assert.True(t, len(longNameProvider.Name) > 100)
}

// Test Hotel entity
func TestHotel_Creation(t *testing.T) {
	hotel := &Hotel{
		ID:          uuid.New(),
		Name:        "Grand Hotel",
		Address:     "123 Main St",
		City:        "New York",
		Country:     "USA",
		PostalCode:  "10001",
		Phone:       "+1-555-0123",
		Email:       "info@grandhotel.com",
		StarRating:  5,
		Description: "Luxury hotel in downtown",
		Website:     "https://grandhotel.com",
		Amenities:   []string{"WiFi", "Pool", "Gym"},
		Images:      []string{"image1.jpg", "image2.jpg"},
		Latitude:    40.7128,
		Longitude:   -74.0060,
	}

	assert.NotNil(t, hotel)
	assert.NotEqual(t, uuid.Nil, hotel.ID)
	assert.Equal(t, "Grand Hotel", hotel.Name)
	assert.Equal(t, "New York", hotel.City)
	assert.Equal(t, "USA", hotel.Country)
	assert.Equal(t, 5, hotel.StarRating)
	assert.Equal(t, 40.7128, hotel.Latitude)
	assert.Equal(t, -74.0060, hotel.Longitude)
	assert.Len(t, hotel.Amenities, 3)
	assert.Len(t, hotel.Images, 2)
}

func TestHotel_Validation(t *testing.T) {
	// Test valid star rating
	hotel := &Hotel{
		ID:         uuid.New(),
		Name:       "Test Hotel",
		StarRating: 3,
		Latitude:   45.0,
		Longitude:  90.0,
	}

	assert.True(t, hotel.StarRating >= 1 && hotel.StarRating <= 5)
	assert.True(t, hotel.Latitude >= -90 && hotel.Latitude <= 90)
	assert.True(t, hotel.Longitude >= -180 && hotel.Longitude <= 180)

	// Test invalid coordinates
	invalidHotel := &Hotel{
		ID:         uuid.New(),
		Name:       "Invalid Hotel",
		StarRating: 3,
		Latitude:   100.0, // Invalid latitude
		Longitude:  200.0, // Invalid longitude
	}

	assert.True(t, invalidHotel.Latitude > 90)
	assert.True(t, invalidHotel.Longitude > 180)
}

// Test ReviewerInfo entity
func TestReviewerInfo_Creation(t *testing.T) {
	memberSince := time.Now().AddDate(-2, 0, 0) // 2 years ago
	
	reviewer := &ReviewerInfo{
		ID:              uuid.New(),
		Name:            "John Doe",
		Email:           "john@example.com",
		Location:        "New York",
		ReviewerLevel:   "Trusted",
		HelpfulVotes:    150,
		IsVerified:      true,
		TotalReviews:    25,
		AverageRating:   4.2,
		MemberSince:     &memberSince,
		ProfileImageURL: "https://example.com/avatar.jpg",
		Bio:             "Travel enthusiast",
	}

	assert.NotNil(t, reviewer)
	assert.NotEqual(t, uuid.Nil, reviewer.ID)
	assert.Equal(t, "John Doe", reviewer.Name)
	assert.Equal(t, "john@example.com", reviewer.Email)
	assert.Equal(t, "Trusted", reviewer.ReviewerLevel)
	assert.Equal(t, 150, reviewer.HelpfulVotes)
	assert.True(t, reviewer.IsVerified)
	assert.Equal(t, 25, reviewer.TotalReviews)
	assert.Equal(t, 4.2, reviewer.AverageRating)
	assert.NotNil(t, reviewer.MemberSince)
}

func TestReviewerInfo_Validation(t *testing.T) {
	reviewer := &ReviewerInfo{
		ID:            uuid.New(),
		Name:          "Test User",
		Email:         "test@example.com",
		AverageRating: 3.75,
	}

	// Test average rating bounds
	assert.True(t, reviewer.AverageRating >= 0.0 && reviewer.AverageRating <= 5.0)

	// Test helpful votes non-negative
	reviewer.HelpfulVotes = 50
	assert.True(t, reviewer.HelpfulVotes >= 0)
}

// Test Review entity
func TestReview_Creation(t *testing.T) {
	reviewDate := time.Now()
	stayDate := time.Now().AddDate(0, -1, 0) // 1 month ago
	
	review := &Review{
		ID:             uuid.New(),
		ProviderID:     uuid.New(),
		HotelID:        uuid.New(),
		ExternalID:     "ext-123",
		Rating:         4.5,
		Title:          "Great stay!",
		Comment:        "Really enjoyed our visit. Clean rooms and friendly staff.",
		ReviewDate:     reviewDate,
		StayDate:       &stayDate,
		TripType:       "Business",
		TravelType:     "Solo",
		RoomType:       "Deluxe",
		IsVerified:     true,
		HelpfulVotes:   5,
		HelpfulCount:   8,
		TotalVotes:     10,
		Pros:           []string{"Clean", "Friendly staff"},
		Cons:           []string{"Noisy"},
		Images:         []string{"room.jpg"},
		Language:       "en",
		Sentiment:      "positive",
		Source:         "mobile",
	}

	assert.NotNil(t, review)
	assert.NotEqual(t, uuid.Nil, review.ID)
	assert.NotEqual(t, uuid.Nil, review.ProviderID)
	assert.NotEqual(t, uuid.Nil, review.HotelID)
	assert.Equal(t, 4.5, review.Rating)
	assert.Equal(t, "Great stay!", review.Title)
	assert.True(t, review.IsVerified)
	assert.Len(t, review.Pros, 2)
	assert.Len(t, review.Cons, 1)
	assert.Len(t, review.Images, 1)
}

func TestReview_RatingValidation(t *testing.T) {
	review := &Review{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		HotelID:    uuid.New(),
		Rating:     4.5,
		ReviewDate: time.Now(),
	}

	// Test valid rating
	assert.True(t, review.Rating >= 1.0 && review.Rating <= 5.0)

	// Test detailed ratings
	serviceRating := 4.0
	cleanlinessRating := 5.0
	locationRating := 3.5
	valueRating := 4.2
	comfortRating := 4.8
	facilitiesRating := 3.9

	review.ServiceRating = &serviceRating
	review.CleanlinessRating = &cleanlinessRating
	review.LocationRating = &locationRating
	review.ValueRating = &valueRating
	review.ComfortRating = &comfortRating
	review.FacilitiesRating = &facilitiesRating

	assert.True(t, *review.ServiceRating >= 1.0 && *review.ServiceRating <= 5.0)
	assert.True(t, *review.CleanlinessRating >= 1.0 && *review.CleanlinessRating <= 5.0)
	assert.True(t, *review.LocationRating >= 1.0 && *review.LocationRating <= 5.0)
	assert.True(t, *review.ValueRating >= 1.0 && *review.ValueRating <= 5.0)
	assert.True(t, *review.ComfortRating >= 1.0 && *review.ComfortRating <= 5.0)
	assert.True(t, *review.FacilitiesRating >= 1.0 && *review.FacilitiesRating <= 5.0)
}

func TestReview_WithMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"source_system": "api_v2",
		"import_batch":  "batch_123",
		"confidence":    0.95,
	}

	review := &Review{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		HotelID:    uuid.New(),
		Rating:     4.0,
		ReviewDate: time.Now(),
		Metadata:   metadata,
	}

	assert.NotNil(t, review.Metadata)
	assert.Equal(t, "api_v2", review.Metadata["source_system"])
	assert.Equal(t, "batch_123", review.Metadata["import_batch"])
	assert.Equal(t, 0.95, review.Metadata["confidence"])
}

// Test ReviewSummary entity
func TestReviewSummary_Creation(t *testing.T) {
	summary := &ReviewSummary{
		ID:                   uuid.New(),
		HotelID:              uuid.New(),
		TotalReviews:         100,
		AverageRating:        4.2,
		RatingDistribution:   map[string]int{"5": 30, "4": 40, "3": 20, "2": 8, "1": 2},
		AvgServiceRating:     4.1,
		AvgCleanlinessRating: 4.5,
		AvgLocationRating:    4.0,
		AvgValueRating:       3.8,
		AvgComfortRating:     4.3,
		AvgFacilitiesRating:  4.0,
		LastReviewDate:       time.Now(),
	}

	assert.NotNil(t, summary)
	assert.NotEqual(t, uuid.Nil, summary.ID)
	assert.NotEqual(t, uuid.Nil, summary.HotelID)
	assert.Equal(t, 100, summary.TotalReviews)
	assert.Equal(t, 4.2, summary.AverageRating)
	assert.Len(t, summary.RatingDistribution, 5)
	assert.Equal(t, 30, summary.RatingDistribution["5"])
	assert.Equal(t, 2, summary.RatingDistribution["1"])
}

func TestReviewSummary_RatingCalculations(t *testing.T) {
	summary := &ReviewSummary{
		ID:                   uuid.New(),
		HotelID:              uuid.New(),
		TotalReviews:         50,
		RatingDistribution:   map[string]int{"5": 10, "4": 20, "3": 15, "2": 3, "1": 2},
		AvgServiceRating:     4.2,
		AvgCleanlinessRating: 4.1,
		AvgLocationRating:    3.9,
		AvgValueRating:       4.0,
		AvgComfortRating:     4.3,
		AvgFacilitiesRating:  4.1,
	}

	// Verify total reviews matches distribution
	totalFromDistribution := 0
	for _, count := range summary.RatingDistribution {
		totalFromDistribution += count
	}
	assert.Equal(t, summary.TotalReviews, totalFromDistribution)

	// Test that all average ratings are within valid range
	assert.True(t, summary.AvgServiceRating >= 0.0 && summary.AvgServiceRating <= 5.0)
	assert.True(t, summary.AvgCleanlinessRating >= 0.0 && summary.AvgCleanlinessRating <= 5.0)
	assert.True(t, summary.AvgLocationRating >= 0.0 && summary.AvgLocationRating <= 5.0)
	assert.True(t, summary.AvgValueRating >= 0.0 && summary.AvgValueRating <= 5.0)
	assert.True(t, summary.AvgComfortRating >= 0.0 && summary.AvgComfortRating <= 5.0)
	assert.True(t, summary.AvgFacilitiesRating >= 0.0 && summary.AvgFacilitiesRating <= 5.0)
}

// Test ReviewProcessingStatus entity
func TestReviewProcessingStatus_Creation(t *testing.T) {
	startedAt := time.Now()
	
	status := &ReviewProcessingStatus{
		ID:               uuid.New(),
		ProviderID:       uuid.New(),
		Status:           "processing",
		StartedAt:        &startedAt,
		RecordsProcessed: 150,
		RecordsTotal:     1000,
		FileURL:          "s3://bucket/file.json",
	}

	assert.NotNil(t, status)
	assert.NotEqual(t, uuid.Nil, status.ID)
	assert.NotEqual(t, uuid.Nil, status.ProviderID)
	assert.Equal(t, "processing", status.Status)
	assert.NotNil(t, status.StartedAt)
	assert.Equal(t, 150, status.RecordsProcessed)
	assert.Equal(t, 1000, status.RecordsTotal)
	assert.Contains(t, status.FileURL, "s3://")
}

func TestReviewProcessingStatus_StatusTransitions(t *testing.T) {
	status := &ReviewProcessingStatus{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		Status:     "pending",
	}

	// Test valid status values
	validStatuses := []string{"pending", "processing", "completed", "failed"}
	for _, validStatus := range validStatuses {
		status.Status = validStatus
		assert.Contains(t, validStatuses, status.Status)
	}

	// Test progress calculation
	status.RecordsProcessed = 250
	status.RecordsTotal = 1000
	progress := float64(status.RecordsProcessed) / float64(status.RecordsTotal) * 100
	assert.Equal(t, 25.0, progress)
}

// Test User entity
func TestUser_Creation(t *testing.T) {
	user := &User{
		ID:               uuid.New(),
		Username:         "testuser",
		Email:            "test@example.com",
		PasswordHash:     "hashed_password_123",
		FirstName:        "John",
		LastName:         "Doe",
		IsActive:         true,
		IsVerified:       false,
		FailedAttempts:   0,
		TwoFactorEnabled: false,
	}

	assert.NotNil(t, user)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "John", user.FirstName)
	assert.Equal(t, "Doe", user.LastName)
	assert.True(t, user.IsActive)
	assert.False(t, user.IsVerified)
	assert.Equal(t, 0, user.FailedAttempts)
	assert.False(t, user.TwoFactorEnabled)
}

func TestUser_Validation(t *testing.T) {
	user := &User{
		ID:       uuid.New(),
		Username: "user123",
		Email:    "user@example.com",
	}

	// Test username length constraints
	assert.True(t, len(user.Username) >= 3 && len(user.Username) <= 50)

	// Test email format (basic check)
	assert.Contains(t, user.Email, "@")
	assert.Contains(t, user.Email, ".")
}

// Test Role entity
func TestRole_Creation(t *testing.T) {
	role := &Role{
		ID:          uuid.New(),
		Name:        "admin",
		Description: "Administrator role with full access",
		IsActive:    true,
	}

	assert.NotNil(t, role)
	assert.NotEqual(t, uuid.Nil, role.ID)
	assert.Equal(t, "admin", role.Name)
	assert.Equal(t, "Administrator role with full access", role.Description)
	assert.True(t, role.IsActive)
}

// Test Permission entity
func TestPermission_Creation(t *testing.T) {
	permission := &Permission{
		ID:          uuid.New(),
		Name:        "reviews.read",
		Resource:    "reviews",
		Action:      "read",
		Description: "Permission to read reviews",
		IsActive:    true,
	}

	assert.NotNil(t, permission)
	assert.NotEqual(t, uuid.Nil, permission.ID)
	assert.Equal(t, "reviews.read", permission.Name)
	assert.Equal(t, "reviews", permission.Resource)
	assert.Equal(t, "read", permission.Action)
	assert.True(t, permission.IsActive)

	// Test valid actions
	validActions := []string{"create", "read", "update", "delete", "execute"}
	assert.Contains(t, validActions, permission.Action)
}

// Test Session entity
func TestSession_Creation(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	refreshExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	
	session := &Session{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		AccessToken:      "access_token_123",
		RefreshToken:     "refresh_token_456",
		ExpiresAt:        expiresAt,
		RefreshExpiresAt: refreshExpiresAt,
		IsActive:         true,
		UserAgent:        "Mozilla/5.0",
		IpAddress:        "192.168.1.1",
		DeviceId:         "device_123",
		LastUsedAt:       time.Now(),
	}

	assert.NotNil(t, session)
	assert.NotEqual(t, uuid.Nil, session.ID)
	assert.NotEqual(t, uuid.Nil, session.UserID)
	assert.Equal(t, "access_token_123", session.AccessToken)
	assert.Equal(t, "refresh_token_456", session.RefreshToken)
	assert.True(t, session.IsActive)
	assert.Equal(t, "Mozilla/5.0", session.UserAgent)
	assert.Equal(t, "192.168.1.1", session.IpAddress)
}

// Test ApiKey entity
func TestApiKey_Creation(t *testing.T) {
	expiresAt := time.Now().Add(365 * 24 * time.Hour) // 1 year
	
	apiKey := &ApiKey{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		Name:       "Test API Key",
		Key:        "api_key_123456",
		KeyHash:    "hashed_key_value",
		IsActive:   true,
		ExpiresAt:  &expiresAt,
		UsageCount: 0,
		RateLimit:  1000,
		Scopes:     []string{"reviews:read", "hotels:read"},
	}

	assert.NotNil(t, apiKey)
	assert.NotEqual(t, uuid.Nil, apiKey.ID)
	assert.NotEqual(t, uuid.Nil, apiKey.UserID)
	assert.Equal(t, "Test API Key", apiKey.Name)
	assert.Equal(t, "api_key_123456", apiKey.Key)
	assert.True(t, apiKey.IsActive)
	assert.NotNil(t, apiKey.ExpiresAt)
	assert.Equal(t, 0, apiKey.UsageCount)
	assert.Equal(t, 1000, apiKey.RateLimit)
	assert.Len(t, apiKey.Scopes, 2)
}

// Test AuditLog entity
func TestAuditLog_Creation(t *testing.T) {
	userID := uuid.New()
	resourceID := uuid.New()
	oldValues := map[string]interface{}{"name": "Old Name"}
	newValues := map[string]interface{}{"name": "New Name"}
	
	auditLog := &AuditLog{
		ID:         uuid.New(),
		UserID:     &userID,
		Action:     "update",
		Resource:   "hotel",
		ResourceID: &resourceID,
		OldValues:  oldValues,
		NewValues:  newValues,
		IpAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		Result:     "success",
		CreatedAt:  time.Now(),
	}

	assert.NotNil(t, auditLog)
	assert.NotEqual(t, uuid.Nil, auditLog.ID)
	assert.NotNil(t, auditLog.UserID)
	assert.Equal(t, "update", auditLog.Action)
	assert.Equal(t, "hotel", auditLog.Resource)
	assert.NotNil(t, auditLog.ResourceID)
	assert.Equal(t, "Old Name", auditLog.OldValues["name"])
	assert.Equal(t, "New Name", auditLog.NewValues["name"])
	assert.Equal(t, "success", auditLog.Result)

	// Test valid results
	validResults := []string{"success", "failure"}
	assert.Contains(t, validResults, auditLog.Result)
}

// Test LoginAttempt entity
func TestLoginAttempt_Creation(t *testing.T) {
	loginAttempt := &LoginAttempt{
		ID:            uuid.New(),
		Email:         "test@example.com",
		IpAddress:     "192.168.1.1",
		UserAgent:     "Mozilla/5.0",
		Success:       false,
		FailureReason: "Invalid password",
		CreatedAt:     time.Now(),
	}

	assert.NotNil(t, loginAttempt)
	assert.NotEqual(t, uuid.Nil, loginAttempt.ID)
	assert.Equal(t, "test@example.com", loginAttempt.Email)
	assert.Equal(t, "192.168.1.1", loginAttempt.IpAddress)
	assert.False(t, loginAttempt.Success)
	assert.Equal(t, "Invalid password", loginAttempt.FailureReason)
}

// Test ReviewStatistics structure
func TestReviewStatistics_Creation(t *testing.T) {
	stats := &ReviewStatistics{
		TotalReviews:       1000,
		AverageRating:      4.2,
		LastUpdated:        time.Now(),
		RatingDistribution: map[int]int{5: 300, 4: 400, 3: 200, 2: 70, 1: 30},
		ReviewsByProvider:  map[string]int{"Booking.com": 600, "Expedia": 400},
	}

	assert.NotNil(t, stats)
	assert.Equal(t, int64(1000), stats.TotalReviews)
	assert.Equal(t, 4.2, stats.AverageRating)
	assert.Len(t, stats.RatingDistribution, 5)
	assert.Len(t, stats.ReviewsByProvider, 2)

	// Verify distribution totals
	totalFromDistribution := 0
	for _, count := range stats.RatingDistribution {
		totalFromDistribution += count
	}
	assert.Equal(t, int(stats.TotalReviews), totalFromDistribution)

	// Verify provider totals
	totalFromProviders := 0
	for _, count := range stats.ReviewsByProvider {
		totalFromProviders += count
	}
	assert.Equal(t, int(stats.TotalReviews), totalFromProviders)
}

// Test entity relationships
func TestEntities_Relationships(t *testing.T) {
	// Test Hotel-Review relationship
	hotel := &Hotel{
		ID:   uuid.New(),
		Name: "Test Hotel",
	}

	review := &Review{
		ID:         uuid.New(),
		HotelID:    hotel.ID,
		ProviderID: uuid.New(),
		Rating:     4.0,
		ReviewDate: time.Now(),
	}

	// Simulate relationship
	hotel.Reviews = []Review{*review}

	assert.Equal(t, hotel.ID, review.HotelID)
	assert.Len(t, hotel.Reviews, 1)
	assert.Equal(t, review.ID, hotel.Reviews[0].ID)

	// Test User-Role relationship (many-to-many)
	user := &User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	adminRole := &Role{
		ID:   uuid.New(),
		Name: "admin",
	}

	userRole := &Role{
		ID:   uuid.New(),
		Name: "user",
	}

	// Simulate many-to-many relationship
	user.Roles = []Role{*adminRole, *userRole}
	adminRole.Users = []User{*user}
	userRole.Users = []User{*user}

	assert.Len(t, user.Roles, 2)
	assert.Len(t, adminRole.Users, 1)
	assert.Len(t, userRole.Users, 1)
}

// Test soft delete functionality
func TestEntities_SoftDelete(t *testing.T) {
	deletedAt := time.Now()
	
	// Test Provider soft delete
	provider := &Provider{
		ID:        uuid.New(),
		Name:      "Deleted Provider",
		DeletedAt: gorm.DeletedAt{Time: deletedAt, Valid: true},
	}

	assert.True(t, provider.DeletedAt.Valid)
	assert.Equal(t, deletedAt, provider.DeletedAt.Time)

	// Test Hotel soft delete
	hotel := &Hotel{
		ID:        uuid.New(),
		Name:      "Deleted Hotel",
		DeletedAt: gorm.DeletedAt{Time: deletedAt, Valid: true},
	}

	assert.True(t, hotel.DeletedAt.Valid)

	// Test Review soft delete
	review := &Review{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		HotelID:    uuid.New(),
		Rating:     4.0,
		ReviewDate: time.Now(),
		DeletedAt:  gorm.DeletedAt{Time: deletedAt, Valid: true},
	}

	assert.True(t, review.DeletedAt.Valid)
}

// Test error constants
func TestEntityErrors(t *testing.T) {
	assert.Equal(t, "review not found", ErrReviewNotFound.Error())
}