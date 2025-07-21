package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type MockReviewService struct {
	mock.Mock
}

func (m *MockReviewService) CreateReview(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}


func (m *MockReviewService) GetReviews(ctx context.Context, filter domain.ReviewFilter) ([]*domain.Review, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockReviewService) UpdateReview(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewService) DeleteReview(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewService) GetReviewStats(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	args := m.Called(ctx, hotelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ReviewSummary), args.Error(1)
}

func (m *MockReviewService) GetReviewByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockReviewService) GetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]domain.Review, error) {
	args := m.Called(ctx, hotelID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Review), args.Error(1)
}

func (m *MockReviewService) GetReviewsByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]domain.Review, error) {
	args := m.Called(ctx, providerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Review), args.Error(1)
}

func (m *MockReviewService) SearchReviews(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]domain.Review, error) {
	args := m.Called(ctx, query, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Review), args.Error(1)
}

// Hotel operations
func (m *MockReviewService) CreateHotel(ctx context.Context, hotel *domain.Hotel) error {
	args := m.Called(ctx, hotel)
	return args.Error(0)
}

func (m *MockReviewService) GetHotelByID(ctx context.Context, id uuid.UUID) (*domain.Hotel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Hotel), args.Error(1)
}

func (m *MockReviewService) UpdateHotel(ctx context.Context, hotel *domain.Hotel) error {
	args := m.Called(ctx, hotel)
	return args.Error(0)
}

func (m *MockReviewService) DeleteHotel(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewService) ListHotels(ctx context.Context, limit, offset int) ([]domain.Hotel, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Hotel), args.Error(1)
}

// Provider operations
func (m *MockReviewService) CreateProvider(ctx context.Context, provider *domain.Provider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockReviewService) GetProviderByID(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockReviewService) GetProviderByName(ctx context.Context, name string) (*domain.Provider, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockReviewService) UpdateProvider(ctx context.Context, provider *domain.Provider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockReviewService) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewService) ListProviders(ctx context.Context, limit, offset int) ([]domain.Provider, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Provider), args.Error(1)
}

// File processing operations
func (m *MockReviewService) ProcessReviewFile(ctx context.Context, fileURL string, providerID uuid.UUID) (*domain.ReviewProcessingStatus, error) {
	args := m.Called(ctx, fileURL, providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ReviewProcessingStatus), args.Error(1)
}

func (m *MockReviewService) GetProcessingStatus(ctx context.Context, id uuid.UUID) (*domain.ReviewProcessingStatus, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ReviewProcessingStatus), args.Error(1)
}

func (m *MockReviewService) GetProcessingHistory(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]domain.ReviewProcessingStatus, error) {
	args := m.Called(ctx, providerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.ReviewProcessingStatus), args.Error(1)
}

func (m *MockReviewService) CancelProcessing(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Analytics operations
func (m *MockReviewService) GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	args := m.Called(ctx, hotelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ReviewSummary), args.Error(1)
}

func (m *MockReviewService) GetReviewStatsByProvider(ctx context.Context, providerID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error) {
	args := m.Called(ctx, providerID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockReviewService) GetReviewStatsByHotel(ctx context.Context, hotelID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error) {
	args := m.Called(ctx, hotelID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockReviewService) GetTopRatedHotels(ctx context.Context, limit int) ([]domain.Hotel, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Hotel), args.Error(1)
}

func (m *MockReviewService) GetRecentReviews(ctx context.Context, limit int) ([]domain.Review, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Review), args.Error(1)
}

// Review validation and enrichment methods
func (m *MockReviewService) ValidateReviewData(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewService) EnrichReviewData(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewService) DetectDuplicateReviews(ctx context.Context, review *domain.Review) ([]domain.Review, error) {
	args := m.Called(ctx, review)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Review), args.Error(1)
}

type MockHotelService struct {
	mock.Mock
}

func (m *MockHotelService) CreateHotel(ctx context.Context, hotel *domain.Hotel) error {
	args := m.Called(ctx, hotel)
	return args.Error(0)
}

func (m *MockHotelService) GetHotel(ctx context.Context, id uuid.UUID) (*domain.Hotel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Hotel), args.Error(1)
}

func (m *MockHotelService) GetHotels(ctx context.Context, filter domain.HotelFilter) ([]*domain.Hotel, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Hotel), args.Error(1)
}

func (m *MockHotelService) UpdateHotel(ctx context.Context, hotel *domain.Hotel) error {
	args := m.Called(ctx, hotel)
	return args.Error(0)
}

func (m *MockHotelService) DeleteHotel(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockHotelService) GetHotelsByCity(ctx context.Context, city string, limit, offset int) ([]*domain.Hotel, error) {
	args := m.Called(ctx, city, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Hotel), args.Error(1)
}

func (m *MockHotelService) GetHotelsByRating(ctx context.Context, minRating int, limit, offset int) ([]*domain.Hotel, error) {
	args := m.Called(ctx, minRating, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Hotel), args.Error(1)
}

type MockProviderService struct {
	mock.Mock
}

func (m *MockProviderService) CreateProvider(ctx context.Context, provider *domain.Provider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockProviderService) GetProvider(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockProviderService) GetProviders(ctx context.Context, filter domain.ProviderFilter) ([]*domain.Provider, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Provider), args.Error(1)
}

func (m *MockProviderService) UpdateProvider(ctx context.Context, provider *domain.Provider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockProviderService) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProviderService) GetActiveProviders(ctx context.Context) ([]*domain.Provider, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Provider), args.Error(1)
}

func (m *MockProviderService) ToggleProviderStatus(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockProviderService) ProcessReviewBatch(ctx context.Context, reviews []domain.Review) error {
	args := m.Called(ctx, reviews)
	return args.Error(0)
}

func (m *MockProviderService) ImportReviewsFromFile(ctx context.Context, fileURL string, providerID uuid.UUID) error {
	args := m.Called(ctx, fileURL, providerID)
	return args.Error(0)
}

func (m *MockProviderService) ExportReviewsToFile(ctx context.Context, filters map[string]interface{}, format string) (string, error) {
	args := m.Called(ctx, filters, format)
	return args.String(0), args.Error(1)
}

// Test setup helper
func setupTestRouter() (*gin.Engine, *MockReviewService, *MockHotelService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	mockReviewService := &MockReviewService{}
	mockHotelService := &MockHotelService{}
	
	// Create logger for handlers
	testLogger := &logger.Logger{Logger: slog.Default()}
	
	// Create real handlers with mock services
	reviewHandlers := NewReviewHandlers(mockReviewService, testLogger)
	hotelHandlers := NewHotelHandlers(mockHotelService, testLogger)
	
	// Add handlers using real implementations
	api := router.Group("/api/v1")
	{
		reviews := api.Group("/reviews")
		{
			reviews.GET("", reviewHandlers.GetReviews)
			reviews.POST("", reviewHandlers.CreateReview)
			reviews.GET("/:id", reviewHandlers.GetReview)
			reviews.PUT("/:id", reviewHandlers.UpdateReview)
			reviews.DELETE("/:id", reviewHandlers.DeleteReview)
		}
		
		hotels := api.Group("/hotels")
		{
			hotels.GET("", hotelHandlers.GetHotels)
			hotels.POST("", hotelHandlers.CreateHotel)
			hotels.GET("/:id", hotelHandlers.GetHotel)
			hotels.PUT("/:id", hotelHandlers.UpdateHotel)
			hotels.DELETE("/:id", hotelHandlers.DeleteHotel)
		}
	}
	
	return router, mockReviewService, mockHotelService
}

func TestReviewHandlers_GetReviews(t *testing.T) {
	router, mockReviewService, _ := setupTestRouter()
	
	tests := []struct {
		name           string
		setupMocks     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful get reviews",
			setupMocks: func() {
				// For GetReviews without filters, handler returns empty list
				// No mock setup needed since handler doesn't call service for general queries
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"reviews":[],"meta":{"limit":20,"offset":0,"count":0}}`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			
			req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews", nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.JSONEq(t, tt.expectedBody, w.Body.String())
			
			mockReviewService.AssertExpectations(t)
		})
	}
}

func TestReviewHandlers_CreateReview(t *testing.T) {
	
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMocks     func(*MockReviewService)
		expectedStatus int
		expectError    bool
	}{
		{
			name: "successful review creation",
			requestBody: map[string]interface{}{
				"hotel_id":    uuid.New().String(),
				"provider_id": uuid.New().String(),
				"rating":      4.5,
				"title":       "Great hotel stay",
				"comment":     "Had a wonderful stay at this hotel. The service was excellent and the room was very comfortable.",
				"language":    "en",
			},
			setupMocks: func(mockReviewService *MockReviewService) {
				mockReviewService.On("CreateReview", mock.Anything, mock.AnythingOfType("*domain.Review")).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "invalid request body",
			requestBody: map[string]interface{}{
				"invalid_field": "invalid_value",
			},
			setupMocks: func(mockReviewService *MockReviewService) {
				// No mocks needed for validation errors - handler should return 400 before calling service
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "malformed JSON",
			requestBody:    `{"invalid": json}`,
			setupMocks:     func(mockReviewService *MockReviewService) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh router and mocks for each test case
			router, mockReviewService, _ := setupTestRouter()
			tt.setupMocks(mockReviewService)
			
			var body *bytes.Buffer
			if str, ok := tt.requestBody.(string); ok {
				body = bytes.NewBufferString(str)
			} else {
				jsonBody, err := json.Marshal(tt.requestBody)
				require.NoError(t, err)
				body = bytes.NewBuffer(jsonBody)
			}
			
			req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectError {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			}
			
			mockReviewService.AssertExpectations(t)
		})
	}
}

func TestReviewHandlers_GetReviewByID(t *testing.T) {
	router, mockReviewService, _ := setupTestRouter()
	
	validID := uuid.New()
	invalidID := "invalid-uuid"
	
	tests := []struct {
		name           string
		reviewID       string
		setupMocks     func()
		expectedStatus int
	}{
		{
			name:     "successful get review by ID",
			reviewID: validID.String(),
			setupMocks: func() {
				mockReview := &domain.Review{
					ID:         validID,
					HotelID:    uuid.New(),
					ProviderID: uuid.New(),
					Rating:     4.5,
					Title:      "Great hotel",
					Comment:    "Had a wonderful stay",
				}
				mockReviewService.On("GetReviewByID", mock.Anything, validID).Return(mockReview, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid UUID format",
			reviewID:       invalidID,
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/reviews/%s", tt.reviewID), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			mockReviewService.AssertExpectations(t)
		})
	}
}

func TestReviewHandlers_UpdateReview(t *testing.T) {
	validID := uuid.New()
	
	tests := []struct {
		name           string
		reviewID       string
		requestBody    interface{}
		setupMocks     func(*MockReviewService)
		expectedStatus int
	}{
		{
			name:     "successful review update",
			reviewID: validID.String(),
			requestBody: map[string]interface{}{
				"hotel_id":    uuid.New().String(),
				"provider_id": uuid.New().String(),
				"rating":      5.0,
				"title":       "Updated title",
				"comment":     "Updated comment with much more content to meet the minimum length requirement",
				"language":    "en",
			},
			setupMocks: func(mockReviewService *MockReviewService) {
				mockReviewService.On("UpdateReview", mock.Anything, mock.AnythingOfType("*domain.Review")).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh router and mocks for each test case
			router, mockReviewService, _ := setupTestRouter()
			tt.setupMocks(mockReviewService)
			
			jsonBody, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)
			
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/reviews/%s", tt.reviewID), bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			mockReviewService.AssertExpectations(t)
		})
	}
}

func TestReviewHandlers_DeleteReview(t *testing.T) {
	validID := uuid.New()
	
	tests := []struct {
		name           string
		reviewID       string
		setupMocks     func(*MockReviewService)
		expectedStatus int
	}{
		{
			name:     "successful review deletion",
			reviewID: validID.String(),
			setupMocks: func(mockReviewService *MockReviewService) {
				mockReviewService.On("DeleteReview", mock.Anything, validID).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh router and mocks for each test case
			router, mockReviewService, _ := setupTestRouter()
			tt.setupMocks(mockReviewService)
			
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/reviews/%s", tt.reviewID), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			mockReviewService.AssertExpectations(t)
		})
	}
}

func TestHotelHandlers_GetHotels(t *testing.T) {
	router, _, mockHotelService := setupTestRouter()
	
	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful get hotels",
			setupMocks: func() {
				mockHotelService.On("GetHotels", mock.Anything, mock.AnythingOfType("domain.HotelFilter")).Return([]*domain.Hotel{}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"hotels":[],"pagination":{"limit":20,"offset":0,"total":0}}`,
		},
		{
			name:        "get hotels with pagination",
			queryParams: "?limit=10&offset=0",
			setupMocks: func() {
				mockHotelService.On("GetHotels", mock.Anything, mock.AnythingOfType("domain.HotelFilter")).Return([]*domain.Hotel{}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"hotels":[],"pagination":{"limit":10,"offset":0,"total":0}}`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			
			url := "/api/v1/hotels" + tt.queryParams
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.JSONEq(t, tt.expectedBody, w.Body.String())
			
			mockHotelService.AssertExpectations(t)
		})
	}
}

func TestHotelHandlers_CreateHotel(t *testing.T) {
	router, _, mockHotelService := setupTestRouter()
	
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMocks     func()
		expectedStatus int
		expectError    bool
	}{
		{
			name: "successful hotel creation",
			requestBody: map[string]interface{}{
				"name":        "Test Hotel",
				"address":     "123 Test Street",
				"city":        "Test City",
				"country":     "Test Country",
				"star_rating": 5,
				"description": "A test hotel",
			},
			setupMocks: func() {
				mockHotelService.On("CreateHotel", mock.Anything, mock.AnythingOfType("*domain.Hotel")).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "invalid request body",
			requestBody: map[string]interface{}{
				"invalid_field": "invalid_value",
			},
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			
			jsonBody, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)
			
			req := httptest.NewRequest(http.MethodPost, "/api/v1/hotels", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectError {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			}
			
			mockHotelService.AssertExpectations(t)
		})
	}
}

func TestMiddleware_RequestLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Add request logging middleware
	router.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		
		// Log the request (in a real implementation, this would use a proper logger)
		t.Logf("Request: %s %s - Status: %d - Duration: %v", 
			c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	})
	
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Add error handling middleware
	router.Use(func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal server error",
					"message": fmt.Sprintf("%v", err),
				})
				c.Abort()
			}
		}()
		c.Next()
	})
	
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})
	
	router.GET("/normal", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "panic recovery",
			path:           "/panic",
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "normal operation",
			path:           "/normal",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectError {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expectValid bool
		fieldErrors []string
	}{
		{
			name: "valid review input",
			input: map[string]interface{}{
				"hotel_id":       uuid.New().String(),
				"provider_id":    uuid.New().String(),
				"rating":         4.5,
				"title":          "Great hotel",
				"comment":        "Had a wonderful stay",
				"reviewer_name":  "John Doe",
				"reviewer_email": "john@example.com",
			},
			expectValid: true,
			fieldErrors: nil,
		},
		{
			name: "invalid rating",
			input: map[string]interface{}{
				"hotel_id":       uuid.New().String(),
				"provider_id":    uuid.New().String(),
				"rating":         6.0, // Invalid: should be 1-5
				"title":          "Great hotel",
				"comment":        "Had a wonderful stay",
				"reviewer_name":  "John Doe",
				"reviewer_email": "john@example.com",
			},
			expectValid: false,
			fieldErrors: []string{"rating"},
		},
		{
			name: "missing required fields",
			input: map[string]interface{}{
				"title":   "Great hotel",
				"comment": "Had a wonderful stay",
			},
			expectValid: false,
			fieldErrors: []string{"hotel_id", "provider_id", "rating"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In a real implementation, this would validate using the actual validation logic
			// For now, we just test the structure
			jsonData, err := json.Marshal(tt.input)
			require.NoError(t, err)
			
			var result map[string]interface{}
			err = json.Unmarshal(jsonData, &result)
			require.NoError(t, err)
			
			// Basic validation logic simulation
			requiredFields := []string{"hotel_id", "provider_id", "rating"}
			var missingFields []string
			
			for _, field := range requiredFields {
				if _, exists := result[field]; !exists {
					missingFields = append(missingFields, field)
				}
			}
			
			if rating, exists := result["rating"]; exists {
				if r, ok := rating.(float64); ok && (r < 1.0 || r > 5.0) {
					missingFields = append(missingFields, "rating")
				}
			}
			
			isValid := len(missingFields) == 0
			assert.Equal(t, tt.expectValid, isValid)
			
			if !tt.expectValid && tt.fieldErrors != nil {
				for _, expectedError := range tt.fieldErrors {
					assert.Contains(t, missingFields, expectedError)
				}
			}
		})
	}
}

// Tests for actual implemented hotel handlers
func TestNewHotelHandlers_CRUD_Operations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testLogger := &logger.Logger{Logger: slog.Default()} // Simple test logger
	
	t.Run("CreateHotel_Success", func(t *testing.T) {
		mockHotelService := &MockHotelService{}
		handlers := NewHotelHandlers(mockHotelService, testLogger)
		
		hotel := domain.Hotel{
			Name:        "Test Hotel",
			Address:     "123 Test St",
			City:        "Test City",
			Country:     "Test Country",
			StarRating:  4,
			Description: "A test hotel",
		}
		
		mockHotelService.On("CreateHotel", mock.Anything, mock.AnythingOfType("*domain.Hotel")).Return(nil)
		
		body, _ := json.Marshal(hotel)
		req := httptest.NewRequest("POST", "/hotels", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/hotels", handlers.CreateHotel)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Hotel created successfully", response["message"])
		
		mockHotelService.AssertExpectations(t)
	})
	
	t.Run("CreateHotel_ValidationError", func(t *testing.T) {
		mockHotelService := &MockHotelService{}
		handlers := NewHotelHandlers(mockHotelService, testLogger)
		
		hotel := domain.Hotel{
			Address: "123 Test St", // Missing required name
		}
		
		// This test should fail validation in the handler before reaching the service
		// So we don't need to mock the service call
		
		body, _ := json.Marshal(hotel)
		req := httptest.NewRequest("POST", "/hotels", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/hotels", handlers.CreateHotel)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "VALIDATION_FAILED", response["error"])
		
		mockHotelService.AssertExpectations(t)
	})
	
	t.Run("CreateHotel_AlreadyExists", func(t *testing.T) {
		mockHotelService := &MockHotelService{}
		handlers := NewHotelHandlers(mockHotelService, testLogger)
		
		hotel := domain.Hotel{
			Name:       "Existing Hotel",
			Address:    "123 Main St",
			City:       "Test City",
			Country:    "Test Country",
			StarRating: 4,
		}
		
		mockHotelService.On("CreateHotel", mock.Anything, mock.AnythingOfType("*domain.Hotel")).
			Return(domain.ErrHotelAlreadyExists).Once()
		
		body, _ := json.Marshal(hotel)
		req := httptest.NewRequest("POST", "/hotels", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/hotels", handlers.CreateHotel)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusConflict, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "HOTEL_ALREADY_EXISTS", response["error"])
		
		mockHotelService.AssertExpectations(t)
	})
	
	t.Run("GetHotel_Success", func(t *testing.T) {
		mockHotelService := &MockHotelService{}
		handlers := NewHotelHandlers(mockHotelService, testLogger)
		
		hotelID := uuid.New()
		hotel := &domain.Hotel{
			ID:          hotelID,
			Name:        "Test Hotel",
			City:        "Test City",
			StarRating:  4,
		}
		
		mockHotelService.On("GetHotel", mock.Anything, hotelID).Return(hotel, nil)
		
		req := httptest.NewRequest("GET", "/hotels/"+hotelID.String(), nil)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.GET("/hotels/:id", handlers.GetHotel)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response domain.Hotel
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, hotel.Name, response.Name)
		assert.Equal(t, hotel.City, response.City)
		
		mockHotelService.AssertExpectations(t)
	})
	
	t.Run("GetHotel_NotFound", func(t *testing.T) {
		mockHotelService := &MockHotelService{}
		handlers := NewHotelHandlers(mockHotelService, testLogger)
		
		hotelID := uuid.New()
		
		mockHotelService.On("GetHotel", mock.Anything, hotelID).Return(nil, domain.ErrHotelNotFound)
		
		req := httptest.NewRequest("GET", "/hotels/"+hotelID.String(), nil)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.GET("/hotels/:id", handlers.GetHotel)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "HOTEL_NOT_FOUND", response["error"])
		
		mockHotelService.AssertExpectations(t)
	})
	
	t.Run("GetHotel_InvalidID", func(t *testing.T) {
		mockHotelService := &MockHotelService{}
		handlers := NewHotelHandlers(mockHotelService, testLogger)
		
		req := httptest.NewRequest("GET", "/hotels/invalid-uuid", nil)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.GET("/hotels/:id", handlers.GetHotel)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "INVALID_ID", response["error"])
		
		mockHotelService.AssertExpectations(t)
	})
}

// Tests for actual implemented provider handlers  
func TestNewProviderHandlers_CRUD_Operations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("CreateProvider_Success", func(t *testing.T) {
		mockProviderService := &MockProviderService{}
		testLogger := &logger.Logger{Logger: slog.Default()}
		handlers := NewProviderHandlers(mockProviderService, testLogger)
		
		provider := domain.Provider{
			Name:     "Test Provider",
			BaseURL:  "https://test-provider.com",
			IsActive: true,
		}
		
		mockProviderService.On("CreateProvider", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)
		
		body, _ := json.Marshal(provider)
		req := httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/providers", handlers.CreateProvider)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Provider created successfully", response["message"])
		
		mockProviderService.AssertExpectations(t)
	})
	
	t.Run("CreateProvider_ValidationError", func(t *testing.T) {
		mockProviderService := &MockProviderService{}
		testLogger := &logger.Logger{Logger: slog.Default()}
		handlers := NewProviderHandlers(mockProviderService, testLogger)
		
		provider := domain.Provider{
			BaseURL:  "https://test-provider.com", // Missing required name
			IsActive: true,
		}
		
		// This test should fail validation in the handler before reaching the service
		// So we don't need to mock the service call
		
		body, _ := json.Marshal(provider)
		req := httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/providers", handlers.CreateProvider)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "VALIDATION_FAILED", response["error"])
		
		mockProviderService.AssertExpectations(t)
	})
	
	t.Run("CreateProvider_AlreadyExists", func(t *testing.T) {
		mockProviderService := &MockProviderService{}
		testLogger := &logger.Logger{Logger: slog.Default()}
		handlers := NewProviderHandlers(mockProviderService, testLogger)
		
		provider := domain.Provider{
			Name:     "Existing Provider",
			IsActive: true,
		}
		
		mockProviderService.On("CreateProvider", mock.Anything, mock.AnythingOfType("*domain.Provider")).
			Return(domain.ErrProviderAlreadyExists)
		
		body, _ := json.Marshal(provider)
		req := httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/providers", handlers.CreateProvider)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusConflict, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "PROVIDER_ALREADY_EXISTS", response["error"])
		
		mockProviderService.AssertExpectations(t)
	})
	
	t.Run("GetProvider_Success", func(t *testing.T) {
		mockProviderService := &MockProviderService{}
		testLogger := &logger.Logger{Logger: slog.Default()}
		handlers := NewProviderHandlers(mockProviderService, testLogger)
		
		providerID := uuid.New()
		provider := &domain.Provider{
			ID:       providerID,
			Name:     "Test Provider",
			BaseURL:  "https://test-provider.com",
			IsActive: true,
		}
		
		mockProviderService.On("GetProvider", mock.Anything, providerID).Return(provider, nil)
		
		req := httptest.NewRequest("GET", "/providers/"+providerID.String(), nil)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.GET("/providers/:id", handlers.GetProvider)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response domain.Provider
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, provider.Name, response.Name)
		assert.Equal(t, provider.BaseURL, response.BaseURL)
		
		mockProviderService.AssertExpectations(t)
	})
	
	t.Run("GetProvider_NotFound", func(t *testing.T) {
		mockProviderService := &MockProviderService{}
		testLogger := &logger.Logger{Logger: slog.Default()}
		handlers := NewProviderHandlers(mockProviderService, testLogger)
		
		providerID := uuid.New()
		
		mockProviderService.On("GetProvider", mock.Anything, providerID).Return(nil, domain.ErrProviderNotFound)
		
		req := httptest.NewRequest("GET", "/providers/"+providerID.String(), nil)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.GET("/providers/:id", handlers.GetProvider)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "PROVIDER_NOT_FOUND", response["error"])
		
		mockProviderService.AssertExpectations(t)
	})
	
	t.Run("ToggleProviderStatus_Success", func(t *testing.T) {
		mockProviderService := &MockProviderService{}
		testLogger := &logger.Logger{Logger: slog.Default()}
		handlers := NewProviderHandlers(mockProviderService, testLogger)
		
		providerID := uuid.New()
		originalProvider := &domain.Provider{
			ID:       providerID,
			Name:     "Test Provider",
			BaseURL:  "https://test-provider.com",
			IsActive: true, // original status
		}
		
		// The handler first gets the provider, then updates it
		mockProviderService.On("GetProvider", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(originalProvider, nil)
		mockProviderService.On("UpdateProvider", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)
		
		req := httptest.NewRequest("PATCH", "/providers/"+providerID.String()+"/toggle-status", nil)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.PATCH("/providers/:id/toggle-status", handlers.ToggleProviderStatus)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response domain.Provider
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, false, response.IsActive) // Should be toggled to false
		
		mockProviderService.AssertExpectations(t)
	})
	
	t.Run("GetProviders_WithFilters", func(t *testing.T) {
		mockProviderService := &MockProviderService{}
		testLogger := &logger.Logger{Logger: slog.Default()}
		handlers := NewProviderHandlers(mockProviderService, testLogger)
		
		providers := []*domain.Provider{
			{
				ID:       uuid.New(),
				Name:     "Active Provider",
				IsActive: true,
			},
		}
		
		mockProviderService.On("GetProviders", mock.Anything, mock.AnythingOfType("domain.ProviderFilter")).
			Return(providers, nil)
		
		req := httptest.NewRequest("GET", "/providers?is_active=true&limit=10", nil)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.GET("/providers", handlers.GetProviders)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "providers")
		
		providersData := response["providers"].([]interface{})
		assert.Len(t, providersData, 1)
		
		mockProviderService.AssertExpectations(t)
	})
}