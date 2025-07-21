package application

import (
	"encoding/json"
	"fmt"
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

func setupSearchAnalyticsTest() (*gin.Engine, *MockReviewService, *SearchAnalyticsHandlers) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockService := &MockReviewService{}
	testLogger := logger.NewDefault()
	handlers := NewSearchAnalyticsHandlers(mockService, testLogger)

	return router, mockService, handlers
}

func TestSearchAnalyticsHandlers_SearchReviews(t *testing.T) {
	router, mockService, handlers := setupSearchAnalyticsTest()
	router.GET("/search/reviews", handlers.SearchReviews)

	testReviews := []domain.Review{
		{
			ID:       uuid.New(),
			Rating:   4.5,
			Comment:  "Great hotel experience",
			HotelID:  uuid.New(),
		},
		{
			ID:       uuid.New(),
			Rating:   5.0,
			Comment:  "Excellent service",
			HotelID:  uuid.New(),
		},
	}

	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func()
		expectedStatus int
		expectError    bool
	}{
		{
			name:        "successful search with query",
			queryParams: "q=great&limit=10&offset=0",
			setupMocks: func() {
				mockService.On("SearchReviews", mock.Anything, "great", mock.AnythingOfType("map[string]interface {}"), 10, 0).
					Return(testReviews, nil)
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "missing query parameter",
			queryParams:    "limit=10",
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:        "search with filters",
			queryParams: "q=hotel&provider_id=" + uuid.New().String() + "&min_rating=4.0",
			setupMocks: func() {
				mockService.On("SearchReviews", mock.Anything, "hotel", mock.AnythingOfType("map[string]interface {}"), 20, 0).
					Return(testReviews, nil)
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:        "search service error",
			queryParams: "q=error",
			setupMocks: func() {
				mockService.On("SearchReviews", mock.Anything, "error", mock.AnythingOfType("map[string]interface {}"), 20, 0).
					Return(nil, fmt.Errorf("search failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous expectations
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			
			tt.setupMocks()

			req := httptest.NewRequest(http.MethodGet, "/search/reviews?"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "data")
				assert.Contains(t, response, "meta")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestSearchAnalyticsHandlers_GetTopRatedHotels(t *testing.T) {
	router, mockService, handlers := setupSearchAnalyticsTest()
	router.GET("/analytics/hotels/top-rated", handlers.GetTopRatedHotels)

	testHotels := []domain.Hotel{
		{
			ID:   uuid.New(),
			Name: "Luxury Hotel",
		},
		{
			ID:   uuid.New(),
			Name: "Budget Inn",
		},
	}

	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func()
		expectedStatus int
		expectError    bool
	}{
		{
			name:        "successful top hotels retrieval",
			queryParams: "limit=5",
			setupMocks: func() {
				mockService.On("GetTopRatedHotels", mock.Anything, 5).Return(testHotels, nil)
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:        "default limit",
			queryParams: "",
			setupMocks: func() {
				mockService.On("GetTopRatedHotels", mock.Anything, 10).Return(testHotels, nil)
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:        "service error",
			queryParams: "limit=5",
			setupMocks: func() {
				mockService.On("GetTopRatedHotels", mock.Anything, 5).Return(nil, fmt.Errorf("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous expectations
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			
			tt.setupMocks()

			req := httptest.NewRequest(http.MethodGet, "/analytics/hotels/top-rated?"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "data")
				assert.Contains(t, response, "meta")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestSearchAnalyticsHandlers_GetRecentReviews(t *testing.T) {
	router, mockService, handlers := setupSearchAnalyticsTest()
	router.GET("/analytics/reviews/recent", handlers.GetRecentReviews)

	testReviews := []domain.Review{
		{
			ID:         uuid.New(),
			Rating:     4.5,
			Comment:    "Recent review 1",
			ReviewDate: time.Now(),
		},
	}

	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func()
		expectedStatus int
		expectError    bool
	}{
		{
			name:        "successful recent reviews retrieval",
			queryParams: "limit=5",
			setupMocks: func() {
				mockService.On("GetRecentReviews", mock.Anything, 5).Return(testReviews, nil)
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:        "service error",
			queryParams: "limit=5",
			setupMocks: func() {
				mockService.On("GetRecentReviews", mock.Anything, 5).Return(nil, fmt.Errorf("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous expectations
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			
			tt.setupMocks()

			req := httptest.NewRequest(http.MethodGet, "/analytics/reviews/recent?"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			mockService.AssertExpectations(t)
		})
	}
}

func TestSearchAnalyticsHandlers_GetProviderStats(t *testing.T) {
	router, mockService, handlers := setupSearchAnalyticsTest()
	router.GET("/analytics/providers/:id/stats", handlers.GetProviderStats)

	providerID := uuid.New()
	testStats := map[string]interface{}{
		"total_reviews":    100,
		"average_rating":   4.2,
		"rating_breakdown": map[string]int{"5": 40, "4": 30, "3": 20, "2": 10},
	}

	tests := []struct {
		name           string
		providerID     string
		queryParams    string
		setupMocks     func()
		expectedStatus int
		expectError    bool
	}{
		{
			name:        "successful provider stats retrieval",
			providerID:  providerID.String(),
			queryParams: "start_date=2024-01-01&end_date=2024-12-31",
			setupMocks: func() {
				mockService.On("GetReviewStatsByProvider", mock.Anything, providerID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).
					Return(testStats, nil)
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "invalid provider ID",
			providerID:     "invalid-uuid",
			queryParams:    "",
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:        "service error",
			providerID:  providerID.String(),
			queryParams: "",
			setupMocks: func() {
				mockService.On("GetReviewStatsByProvider", mock.Anything, providerID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).
					Return(nil, fmt.Errorf("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous expectations
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			
			tt.setupMocks()

			url := fmt.Sprintf("/analytics/providers/%s/stats?%s", tt.providerID, tt.queryParams)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "data")
				assert.Contains(t, response, "meta")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestSearchAnalyticsHandlers_GetHotelStats(t *testing.T) {
	router, mockService, handlers := setupSearchAnalyticsTest()
	router.GET("/analytics/hotels/:id/stats", handlers.GetHotelStats)

	hotelID := uuid.New()
	testStats := map[string]interface{}{
		"total_reviews":  50,
		"average_rating": 4.5,
	}

	tests := []struct {
		name           string
		hotelID        string
		setupMocks     func()
		expectedStatus int
		expectError    bool
	}{
		{
			name:    "successful hotel stats retrieval",
			hotelID: hotelID.String(),
			setupMocks: func() {
				mockService.On("GetReviewStatsByHotel", mock.Anything, hotelID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).
					Return(testStats, nil)
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "invalid hotel ID",
			hotelID:        "invalid-uuid",
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous expectations
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			
			tt.setupMocks()

			url := fmt.Sprintf("/analytics/hotels/%s/stats", tt.hotelID)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			mockService.AssertExpectations(t)
		})
	}
}

func TestSearchAnalyticsHandlers_GetOverallAnalytics(t *testing.T) {
	router, mockService, handlers := setupSearchAnalyticsTest()
	router.GET("/analytics/overview", handlers.GetOverallAnalytics)

	testReviews := []domain.Review{{ID: uuid.New(), Rating: 5.0}}
	testHotels := []domain.Hotel{{ID: uuid.New(), Name: "Test Hotel"}}

	mockService.On("GetRecentReviews", mock.Anything, 5).Return(testReviews, nil)
	mockService.On("GetTopRatedHotels", mock.Anything, 5).Return(testHotels, nil)

	req := httptest.NewRequest(http.MethodGet, "/analytics/overview", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "data")
	data := response["data"].(map[string]interface{})
	assert.Contains(t, data, "recent_reviews")
	assert.Contains(t, data, "top_hotels")
	assert.Contains(t, data, "timestamp")

	mockService.AssertExpectations(t)
}