package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Mock ReviewService for testing
type MockReviewService struct {
	mock.Mock
}

func (m *MockReviewService) CreateReview(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewService) GetReviewByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockReviewService) GetReviewsByHotel(ctx context.Context, hotelID uuid.UUID, limit, offset int) ([]domain.Review, error) {
	args := m.Called(ctx, hotelID, limit, offset)
	return args.Get(0).([]domain.Review), args.Error(1)
}

func (m *MockReviewService) GetReviewsByProvider(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]domain.Review, error) {
	args := m.Called(ctx, providerID, limit, offset)
	return args.Get(0).([]domain.Review), args.Error(1)
}

func (m *MockReviewService) SearchReviews(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]domain.Review, error) {
	args := m.Called(ctx, query, filters, limit, offset)
	return args.Get(0).([]domain.Review), args.Error(1)
}

func (m *MockReviewService) CreateHotel(ctx context.Context, hotel *domain.Hotel) error {
	args := m.Called(ctx, hotel)
	return args.Error(0)
}

func (m *MockReviewService) GetHotelByID(ctx context.Context, id uuid.UUID) (*domain.Hotel, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.Hotel), args.Error(1)
}

func (m *MockReviewService) ListHotels(ctx context.Context, limit, offset int) ([]domain.Hotel, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]domain.Hotel), args.Error(1)
}

func (m *MockReviewService) CreateProvider(ctx context.Context, provider *domain.Provider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockReviewService) GetProviderByID(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockReviewService) ListProviders(ctx context.Context, limit, offset int) ([]domain.Provider, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]domain.Provider), args.Error(1)
}

func (m *MockReviewService) GetReviewSummary(ctx context.Context, hotelID uuid.UUID) (*domain.ReviewSummary, error) {
	args := m.Called(ctx, hotelID)
	return args.Get(0).(*domain.ReviewSummary), args.Error(1)
}

func (m *MockReviewService) GetTopRatedHotels(ctx context.Context, limit int) ([]domain.Hotel, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]domain.Hotel), args.Error(1)
}

func (m *MockReviewService) GetRecentReviews(ctx context.Context, limit int) ([]domain.Review, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]domain.Review), args.Error(1)
}

// Additional methods to match domain.ReviewService interface
func (m *MockReviewService) UpdateReview(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewService) DeleteReview(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewService) UpdateHotel(ctx context.Context, hotel *domain.Hotel) error {
	args := m.Called(ctx, hotel)
	return args.Error(0)
}

func (m *MockReviewService) DeleteHotel(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewService) GetProviderByName(ctx context.Context, name string) (*domain.Provider, error) {
	args := m.Called(ctx, name)
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

func (m *MockReviewService) ProcessReviewFile(ctx context.Context, fileURL string, providerID uuid.UUID) (*domain.ReviewProcessingStatus, error) {
	args := m.Called(ctx, fileURL, providerID)
	return args.Get(0).(*domain.ReviewProcessingStatus), args.Error(1)
}

func (m *MockReviewService) GetProcessingStatus(ctx context.Context, id uuid.UUID) (*domain.ReviewProcessingStatus, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.ReviewProcessingStatus), args.Error(1)
}

func (m *MockReviewService) GetProcessingHistory(ctx context.Context, providerID uuid.UUID, limit, offset int) ([]domain.ReviewProcessingStatus, error) {
	args := m.Called(ctx, providerID, limit, offset)
	return args.Get(0).([]domain.ReviewProcessingStatus), args.Error(1)
}

func (m *MockReviewService) CancelProcessing(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewService) GetReviewStatsByProvider(ctx context.Context, providerID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error) {
	args := m.Called(ctx, providerID, startDate, endDate)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockReviewService) GetReviewStatsByHotel(ctx context.Context, hotelID uuid.UUID, startDate, endDate time.Time) (map[string]interface{}, error) {
	args := m.Called(ctx, hotelID, startDate, endDate)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

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
	return args.Get(0).([]domain.Review), args.Error(1)
}

func (m *MockReviewService) ProcessReviewBatch(ctx context.Context, reviews []domain.Review) error {
	args := m.Called(ctx, reviews)
	return args.Error(0)
}

func (m *MockReviewService) ImportReviewsFromFile(ctx context.Context, fileURL string, providerID uuid.UUID) error {
	args := m.Called(ctx, fileURL, providerID)
	return args.Error(0)
}

func (m *MockReviewService) ExportReviewsToFile(ctx context.Context, filters map[string]interface{}, format string) (string, error) {
	args := m.Called(ctx, filters, format)
	return args.String(0), args.Error(1)
}

// Test Suite
type HandlersTestSuite struct {
	suite.Suite
	handlers      *Handlers
	mockService   *MockReviewService
	router        *mux.Router
	testLogger    *logger.Logger
}

func (suite *HandlersTestSuite) SetupTest() {
	suite.mockService = new(MockReviewService)
	suite.testLogger = logger.NewDefault()
	suite.handlers = NewHandlers(suite.mockService, suite.testLogger)
	suite.router = mux.NewRouter()
	suite.handlers.SetupRoutes(suite.router)
}

func (suite *HandlersTestSuite) TearDownTest() {
	suite.mockService.AssertExpectations(suite.T())
}

func TestHandlersTestSuite(t *testing.T) {
	suite.Run(t, new(HandlersTestSuite))
}

// Test HealthCheck handler
func (suite *HandlersTestSuite) TestHealthCheck() {
	// Arrange
	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.HealthCheck(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	
	// Extract HealthResponse from Data
	dataBytes, _ := json.Marshal(response.Data)
	var healthResponse HealthResponse
	err = json.Unmarshal(dataBytes, &healthResponse)
	suite.NoError(err)
	suite.Equal("healthy", healthResponse.Status)
	suite.Equal("1.0.0", healthResponse.Version)
}

// Test CreateReview handler - Success
func (suite *HandlersTestSuite) TestCreateReview_Success() {
	// Arrange
	reviewRequest := CreateReviewRequest{
		ProviderID:     uuid.New().String(),
		HotelID:        uuid.New().String(),
		ReviewerInfoID: uuid.New().String(),
		Rating:         4.5,
		Title:          "Great hotel",
		Comment:        "Excellent service",
		ReviewDate:     time.Now(),
	}
	
	jsonBody, _ := json.Marshal(reviewRequest)
	req, _ := http.NewRequest("POST", "/api/v1/reviews", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.mockService.On("CreateReview", mock.Anything, mock.AnythingOfType("*domain.Review")).Return(nil)

	// Act
	suite.handlers.CreateReview(rr, req)

	// Assert
	suite.Equal(http.StatusCreated, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
}

// Test CreateReview handler - Invalid JSON
func (suite *HandlersTestSuite) TestCreateReview_InvalidJSON() {
	// Arrange
	req, _ := http.NewRequest("POST", "/api/v1/reviews", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.CreateReview(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid request format")
}

// Test CreateReview handler - Service Error
func (suite *HandlersTestSuite) TestCreateReview_ServiceError() {
	// Arrange
	reviewRequest := CreateReviewRequest{
		ProviderID:     uuid.New().String(),
		HotelID:        uuid.New().String(),
		ReviewerInfoID: uuid.New().String(),
		Rating:         4.5,
		Title:          "Great hotel",
		Comment:        "Excellent service",
		ReviewDate:     time.Now(),
	}
	
	jsonBody, _ := json.Marshal(reviewRequest)
	req, _ := http.NewRequest("POST", "/api/v1/reviews", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.mockService.On("CreateReview", mock.Anything, mock.AnythingOfType("*domain.Review")).Return(errors.New("service error"))

	// Act
	suite.handlers.CreateReview(rr, req)

	// Assert
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to create review")
}

// Test GetReview handler - Success
func (suite *HandlersTestSuite) TestGetReview_Success() {
	// Arrange
	reviewID := uuid.New()
	expectedReview := &domain.Review{
		ID:      reviewID,
		Rating:  4.5,
		Comment: "Great hotel",
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/reviews/"+reviewID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": reviewID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetReviewByID", mock.Anything, reviewID).Return(expectedReview, nil)

	// Act
	suite.handlers.GetReview(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetReview handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetReview_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/reviews/invalid-uuid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetReview(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid review ID")
}

// Test GetReview handler - Not Found
func (suite *HandlersTestSuite) TestGetReview_NotFound() {
	// Arrange
	reviewID := uuid.New()
	
	req, _ := http.NewRequest("GET", "/api/v1/reviews/"+reviewID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": reviewID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetReviewByID", mock.Anything, reviewID).Return((*domain.Review)(nil), errors.New("not found"))

	// Act
	suite.handlers.GetReview(rr, req)

	// Assert
	suite.Equal(http.StatusNotFound, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Review not found")
}

// Test GetReviewsByHotel handler - Success
func (suite *HandlersTestSuite) TestGetReviewsByHotel_Success() {
	// Arrange
	hotelID := uuid.New()
	expectedReviews := []domain.Review{
		{ID: uuid.New(), Rating: 4.5, Comment: "Great hotel"},
		{ID: uuid.New(), Rating: 4.0, Comment: "Good service"},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/hotels/"+hotelID.String()+"/reviews?limit=10&offset=0", nil)
	req = mux.SetURLVars(req, map[string]string{"hotel_id": hotelID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetReviewsByHotel", mock.Anything, hotelID, 10, 0).Return(expectedReviews, nil)

	// Act
	suite.handlers.GetReviewsByHotel(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetReviewsByHotel handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetReviewsByHotel_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/hotels/invalid-uuid/reviews", nil)
	req = mux.SetURLVars(req, map[string]string{"hotel_id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetReviewsByHotel(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid hotel ID")
}

// Test GetProcessingHistory handler - Success
func (suite *HandlersTestSuite) TestGetProcessingHistory_Success() {
	// Arrange
	providerID := uuid.New()
	expectedHistory := []domain.ReviewProcessingStatus{
		{ID: uuid.New(), ProviderID: providerID, Status: "completed"},
		{ID: uuid.New(), ProviderID: providerID, Status: "failed"},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/processing/history/"+providerID.String()+"?limit=10&offset=0", nil)
	req = mux.SetURLVars(req, map[string]string{"provider_id": providerID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetProcessingHistory", mock.Anything, providerID, 10, 0).Return(expectedHistory, nil)

	// Act
	suite.handlers.GetProcessingHistory(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetProcessingHistory handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetProcessingHistory_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/processing/history/invalid-uuid", nil)
	req = mux.SetURLVars(req, map[string]string{"provider_id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetProcessingHistory(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid provider ID")
}

// Test CreateProvider handler - Service Error
func (suite *HandlersTestSuite) TestCreateProvider_ServiceError() {
	// Arrange
	providerRequest := CreateProviderRequest{
		Name:     "Test Provider",
		BaseURL:  "https://example.com",
		IsActive: true,
	}
	
	jsonBody, _ := json.Marshal(providerRequest)
	req, _ := http.NewRequest("POST", "/api/v1/providers", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.mockService.On("CreateProvider", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(errors.New("service error"))

	// Act
	suite.handlers.CreateProvider(rr, req)

	// Assert
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to create provider")
}

// Test CreateHotel handler - Success
func (suite *HandlersTestSuite) TestCreateHotel_Success() {
	// Arrange
	hotelRequest := CreateHotelRequest{
		Name:        "Grand Hotel",
		Address:     "123 Main St",
		City:        "New York",
		Country:     "USA",
		Description: "A grand hotel in the heart of the city",
		Amenities:   []string{"WiFi", "Pool", "Gym"},
	}
	
	jsonBody, _ := json.Marshal(hotelRequest)
	req, _ := http.NewRequest("POST", "/api/v1/hotels", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.mockService.On("CreateHotel", mock.Anything, mock.AnythingOfType("*domain.Hotel")).Return(nil)

	// Act
	suite.handlers.CreateHotel(rr, req)

	// Assert
	suite.Equal(http.StatusCreated, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
}

// Test CreateHotel handler - Invalid JSON
func (suite *HandlersTestSuite) TestCreateHotel_InvalidJSON() {
	// Arrange
	req, _ := http.NewRequest("POST", "/api/v1/hotels", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.CreateHotel(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid request format")
}

// Test GetHotel handler - Success
func (suite *HandlersTestSuite) TestGetHotel_Success() {
	// Arrange
	hotelID := uuid.New()
	expectedHotel := &domain.Hotel{
		ID:      hotelID,
		Name:    "Grand Hotel",
		Address: "123 Main St",
		City:    "New York",
		Country: "USA",
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/hotels/"+hotelID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": hotelID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetHotelByID", mock.Anything, hotelID).Return(expectedHotel, nil)

	// Act
	suite.handlers.GetHotel(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetHotel handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetHotel_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/hotels/invalid-uuid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetHotel(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid hotel ID")
}

// Test SearchReviews handler - Success
func (suite *HandlersTestSuite) TestSearchReviews_Success() {
	// Arrange
	expectedReviews := []domain.Review{
		{ID: uuid.New(), Rating: 4.5, Comment: "Great hotel"},
		{ID: uuid.New(), Rating: 4.0, Comment: "Good service"},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/search/reviews?q=great&limit=10&offset=0", nil)
	rr := httptest.NewRecorder()

	suite.mockService.On("SearchReviews", mock.Anything, "great", mock.Anything, 10, 0).Return(expectedReviews, nil)

	// Act
	suite.handlers.SearchReviews(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetTopRatedHotels handler - Success
func (suite *HandlersTestSuite) TestGetTopRatedHotels_Success() {
	// Arrange
	expectedHotels := []domain.Hotel{
		{ID: uuid.New(), Name: "Grand Hotel", StarRating: 5},
		{ID: uuid.New(), Name: "Luxury Inn", StarRating: 5},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/hotels/top-rated?limit=10", nil)
	rr := httptest.NewRecorder()

	suite.mockService.On("GetTopRatedHotels", mock.Anything, 10).Return(expectedHotels, nil)

	// Act
	suite.handlers.GetTopRatedHotels(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetRecentReviews handler - Success
func (suite *HandlersTestSuite) TestGetRecentReviews_Success() {
	// Arrange
	expectedReviews := []domain.Review{
		{ID: uuid.New(), Rating: 4.5, Comment: "Recent review"},
		{ID: uuid.New(), Rating: 4.0, Comment: "Another recent review"},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/reviews/recent?limit=10", nil)
	rr := httptest.NewRecorder()

	suite.mockService.On("GetRecentReviews", mock.Anything, 10).Return(expectedReviews, nil)

	// Act
	suite.handlers.GetRecentReviews(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test ProcessFile handler - Success
func (suite *HandlersTestSuite) TestProcessFile_Success() {
	// Arrange
	providerID := uuid.New()
	processRequest := ProcessFileRequest{
		FileURL:    "https://my-bucket.s3.amazonaws.com/test-file.jsonl",
		ProviderID: providerID.String(),
	}
	
	jsonBody, _ := json.Marshal(processRequest)
	req, _ := http.NewRequest("POST", "/api/v1/process", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	expectedStatus := &domain.ReviewProcessingStatus{
		ID:         uuid.New(),
		ProviderID: providerID,
		FileURL:    processRequest.FileURL,
		Status:     "pending",
	}

	suite.mockService.On("ProcessReviewFile", mock.Anything, processRequest.FileURL, providerID).Return(expectedStatus, nil)

	// Act
	suite.handlers.ProcessFile(rr, req)

	// Assert
	suite.Equal(http.StatusAccepted, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test ProcessFile handler - Invalid JSON
func (suite *HandlersTestSuite) TestProcessFile_InvalidJSON() {
	// Arrange
	req, _ := http.NewRequest("POST", "/api/v1/process", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.ProcessFile(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid request format")
}

func (suite *HandlersTestSuite) TestProcessFile_ValidationFailed() {
	// Arrange - empty FileURL should be rejected by service
	processRequest := ProcessFileRequest{
		FileURL:    "", // Invalid empty URL
		ProviderID: uuid.New().String(),
	}
	
	// Mock service to return validation error for empty URL
	providerID, _ := uuid.Parse(processRequest.ProviderID)
	suite.mockService.On("ProcessReviewFile", mock.Anything, "", providerID).Return((*domain.ReviewProcessingStatus)(nil), errors.New("validation failed: empty file URL"))
	
	jsonBody, _ := json.Marshal(processRequest)
	req, _ := http.NewRequest("POST", "/api/v1/process", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	// Act
	suite.handlers.ProcessFile(rr, req)
	
	// Assert
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to start file processing")
}

func (suite *HandlersTestSuite) TestProcessFile_InvalidProviderID() {
	// Arrange
	processRequest := ProcessFileRequest{
		FileURL:    "https://my-bucket.s3.amazonaws.com/test-file.jsonl",
		ProviderID: "invalid-uuid",
	}
	
	jsonBody, _ := json.Marshal(processRequest)
	req, _ := http.NewRequest("POST", "/api/v1/process", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	// Act
	suite.handlers.ProcessFile(rr, req)
	
	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid provider ID")
}

func (suite *HandlersTestSuite) TestProcessFile_ServiceError() {
	// Arrange
	providerID := uuid.New()
	processRequest := ProcessFileRequest{
		FileURL:    "https://my-bucket.s3.amazonaws.com/test-file.jsonl",
		ProviderID: providerID.String(),
	}
	
	jsonBody, _ := json.Marshal(processRequest)
	req, _ := http.NewRequest("POST", "/api/v1/process", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	// Mock service to return error
	suite.mockService.On("ProcessReviewFile", mock.Anything, processRequest.FileURL, providerID).Return((*domain.ReviewProcessingStatus)(nil), errors.New("service error"))
	
	// Act
	suite.handlers.ProcessFile(rr, req)
	
	// Assert
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to start file processing")
}

// Test GetProcessingStatus handler - Success
func (suite *HandlersTestSuite) TestGetProcessingStatus_Success() {
	// Arrange
	processingID := uuid.New()
	expectedStatus := &domain.ReviewProcessingStatus{
		ID:              processingID,
		ProviderID:      uuid.New(),
		FileURL:         "https://my-bucket.s3.amazonaws.com/test-file.jsonl",
		Status:          "completed",
		RecordsTotal:    100,
		RecordsProcessed: 100,
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/processing/"+processingID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": processingID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetProcessingStatus", mock.Anything, processingID).Return(expectedStatus, nil)

	// Act
	suite.handlers.GetProcessingStatus(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetProcessingStatus handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetProcessingStatus_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/processing/invalid-uuid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetProcessingStatus(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid processing ID")
}

// Test GetReviewsByProvider handler - Success
func (suite *HandlersTestSuite) TestGetReviewsByProvider_Success() {
	// Arrange
	providerID := uuid.New()
	expectedReviews := []domain.Review{
		{ID: uuid.New(), Rating: 4.5, Comment: "Great review"},
		{ID: uuid.New(), Rating: 4.0, Comment: "Good review"},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/providers/"+providerID.String()+"/reviews?limit=10&offset=0", nil)
	req = mux.SetURLVars(req, map[string]string{"provider_id": providerID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetReviewsByProvider", mock.Anything, providerID, 10, 0).Return(expectedReviews, nil)

	// Act
	suite.handlers.GetReviewsByProvider(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetReviewsByProvider handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetReviewsByProvider_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/providers/invalid-uuid/reviews", nil)
	req = mux.SetURLVars(req, map[string]string{"provider_id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetReviewsByProvider(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid provider ID")
}

// Test ListHotels handler - Success
func (suite *HandlersTestSuite) TestListHotels_Success() {
	// Arrange
	expectedHotels := []domain.Hotel{
		{ID: uuid.New(), Name: "Grand Hotel", City: "New York"},
		{ID: uuid.New(), Name: "Luxury Inn", City: "San Francisco"},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/hotels?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()

	suite.mockService.On("ListHotels", mock.Anything, 10, 0).Return(expectedHotels, nil)

	// Act
	suite.handlers.ListHotels(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetProvider handler - Success
func (suite *HandlersTestSuite) TestGetProvider_Success() {
	// Arrange
	providerID := uuid.New()
	expectedProvider := &domain.Provider{
		ID:       providerID,
		Name:     "Test Provider",
		BaseURL:  "https://example.com",
		IsActive: true,
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/providers/"+providerID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": providerID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetProviderByID", mock.Anything, providerID).Return(expectedProvider, nil)

	// Act
	suite.handlers.GetProvider(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetProvider handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetProvider_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/providers/invalid-uuid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetProvider(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid provider ID")
}

// Test ListProviders handler - Success
func (suite *HandlersTestSuite) TestListProviders_Success() {
	// Arrange
	expectedProviders := []domain.Provider{
		{ID: uuid.New(), Name: "Test Provider 1", IsActive: true},
		{ID: uuid.New(), Name: "Test Provider 2", IsActive: false},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/providers?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()

	suite.mockService.On("ListProviders", mock.Anything, 10, 0).Return(expectedProviders, nil)

	// Act
	suite.handlers.ListProviders(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetReviewSummary handler - Success
func (suite *HandlersTestSuite) TestGetReviewSummary_Success() {
	// Arrange
	hotelID := uuid.New()
	expectedSummary := &domain.ReviewSummary{
		HotelID:           hotelID,
		TotalReviews:      100,
		AverageRating:     4.5,
		RatingDistribution: map[string]int{"1": 5, "2": 10, "3": 15, "4": 30, "5": 40},
	}
	
	req, _ := http.NewRequest("GET", "/api/v1/hotels/"+hotelID.String()+"/summary", nil)
	req = mux.SetURLVars(req, map[string]string{"hotel_id": hotelID.String()})
	rr := httptest.NewRecorder()

	suite.mockService.On("GetReviewSummary", mock.Anything, hotelID).Return(expectedSummary, nil)

	// Act
	suite.handlers.GetReviewSummary(rr, req)

	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

// Test GetReviewSummary handler - Invalid UUID
func (suite *HandlersTestSuite) TestGetReviewSummary_InvalidUUID() {
	// Arrange
	req, _ := http.NewRequest("GET", "/api/v1/hotels/invalid-uuid/summary", nil)
	req = mux.SetURLVars(req, map[string]string{"hotel_id": "invalid-uuid"})
	rr := httptest.NewRecorder()

	// Act
	suite.handlers.GetReviewSummary(rr, req)

	// Assert
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid hotel ID")
}

// Test Middleware Functions
func (suite *HandlersTestSuite) TestLoggingMiddleware() {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	
	req, _ := http.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	// Act
	loggingHandler := suite.handlers.LoggingMiddleware(testHandler)
	loggingHandler.ServeHTTP(rr, req)
	
	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("test response", rr.Body.String())
}

func (suite *HandlersTestSuite) TestRecoveryMiddleware() {
	// Arrange
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	
	req, _ := http.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	// Act
	recoveryHandler := suite.handlers.RecoveryMiddleware(panicHandler)
	recoveryHandler.ServeHTTP(rr, req)
	
	// Assert
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Internal server error")
}

func (suite *HandlersTestSuite) TestCORSMiddleware() {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	
	// Act
	corsHandler := suite.handlers.CORSMiddleware(testHandler)
	corsHandler.ServeHTTP(rr, req)
	
	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("*", rr.Header().Get("Access-Control-Allow-Origin"))
	suite.Equal("GET, POST, PUT, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
	suite.Equal("Content-Type, Authorization, X-Request-ID, X-Correlation-ID", rr.Header().Get("Access-Control-Allow-Headers"))
	suite.Equal("test response", rr.Body.String())
}

func (suite *HandlersTestSuite) TestCORSMiddleware_OPTIONS() {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("should not reach here"))
	})
	
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	
	// Act
	corsHandler := suite.handlers.CORSMiddleware(testHandler)
	corsHandler.ServeHTTP(rr, req)
	
	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("*", rr.Header().Get("Access-Control-Allow-Origin"))
	suite.Equal("GET, POST, PUT, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
	suite.Equal("Content-Type, Authorization, X-Request-ID, X-Correlation-ID", rr.Header().Get("Access-Control-Allow-Headers"))
	suite.Empty(rr.Body.String())
}

func (suite *HandlersTestSuite) TestContentTypeMiddleware_GET() {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	
	req, _ := http.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	// Act
	contentTypeHandler := suite.handlers.ContentTypeMiddleware(testHandler)
	contentTypeHandler.ServeHTTP(rr, req)
	
	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("test response", rr.Body.String())
}

func (suite *HandlersTestSuite) TestContentTypeMiddleware_POST_ValidContentType() {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})
	
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	// Act
	contentTypeHandler := suite.handlers.ContentTypeMiddleware(testHandler)
	contentTypeHandler.ServeHTTP(rr, req)
	
	// Assert
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("test response", rr.Body.String())
}

func (suite *HandlersTestSuite) TestContentTypeMiddleware_POST_InvalidContentType() {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("should not reach here"))
	})
	
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	
	// Act
	contentTypeHandler := suite.handlers.ContentTypeMiddleware(testHandler)
	contentTypeHandler.ServeHTTP(rr, req)
	
	// Assert
	suite.Equal(http.StatusUnsupportedMediaType, rr.Code)
	
	var response APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	suite.NoError(err)
	suite.False(response.Success)
	suite.Contains(response.Error, "Content-Type must be application/json")
}

func (suite *HandlersTestSuite) TestBuildFilters_EmptyRequest() {
	// Arrange
	req, _ := http.NewRequest("GET", "/test", nil)
	
	// Act
	filters := suite.handlers.buildFilters(req)
	
	// Assert
	suite.Empty(filters)
}

func (suite *HandlersTestSuite) TestBuildFilters_AllValidFilters() {
	// Arrange
	hotelID := uuid.New()
	providerID := uuid.New()
	url := "/test?rating=4.5&min_rating=2.0&max_rating=5.0&provider_id=" + providerID.String() + 
		"&hotel_id=" + hotelID.String() + "&language=en&sentiment=positive&is_verified=true" +
		"&start_date=2023-01-01&end_date=2023-12-31"
	req, _ := http.NewRequest("GET", url, nil)
	
	// Act
	filters := suite.handlers.buildFilters(req)
	
	// Assert
	suite.Equal(4.5, filters["rating"])
	suite.Equal(2.0, filters["min_rating"])
	suite.Equal(5.0, filters["max_rating"])
	suite.Equal(providerID, filters["provider_id"])
	suite.Equal(hotelID, filters["hotel_id"])
	suite.Equal("en", filters["language"])
	suite.Equal("positive", filters["sentiment"])
	suite.Equal(true, filters["is_verified"])
	
	startDate, ok := filters["start_date"].(time.Time)
	suite.True(ok)
	suite.Equal(2023, startDate.Year())
	suite.Equal(time.January, startDate.Month())
	suite.Equal(1, startDate.Day())
	
	endDate, ok := filters["end_date"].(time.Time)
	suite.True(ok)
	suite.Equal(2023, endDate.Year())
	suite.Equal(time.December, endDate.Month())
	suite.Equal(31, endDate.Day())
}

func (suite *HandlersTestSuite) TestBuildFilters_InvalidValues() {
	// Arrange
	url := "/test?rating=invalid&min_rating=abc&provider_id=invalid-uuid&hotel_id=bad-uuid" +
		"&is_verified=not-bool&start_date=invalid-date&end_date=2023-13-50"
	req, _ := http.NewRequest("GET", url, nil)
	
	// Act
	filters := suite.handlers.buildFilters(req)
	
	// Assert - invalid values should be ignored
	suite.Empty(filters)
}

func (suite *HandlersTestSuite) TestBuildFilters_PartialValidValues() {
	// Arrange
	hotelID := uuid.New()
	url := "/test?rating=4.5&min_rating=invalid&provider_id=invalid-uuid&hotel_id=" + hotelID.String() + 
		"&language=en&is_verified=not-bool&start_date=2023-01-01&end_date=invalid-date"
	req, _ := http.NewRequest("GET", url, nil)
	
	// Act
	filters := suite.handlers.buildFilters(req)
	
	// Assert - only valid values should be included
	suite.Equal(4.5, filters["rating"])
	suite.Equal(hotelID, filters["hotel_id"])
	suite.Equal("en", filters["language"])
	
	startDate, ok := filters["start_date"].(time.Time)
	suite.True(ok)
	suite.Equal(2023, startDate.Year())
	
	// Invalid values should not be present
	_, hasMinRating := filters["min_rating"]
	suite.False(hasMinRating)
	_, hasProviderID := filters["provider_id"]
	suite.False(hasProviderID)
	_, hasIsVerified := filters["is_verified"]
	suite.False(hasIsVerified)
	_, hasEndDate := filters["end_date"]
	suite.False(hasEndDate)
}

// Additional error path tests for Application layer
func (suite *HandlersTestSuite) TestCreateReview_InvalidProviderID() {
	reqBody := CreateReviewRequest{
		ProviderID:        "invalid-uuid",
		HotelID:           uuid.New().String(),
		ReviewerInfoID:    uuid.New().String(),
		Rating:            4.5,
		Comment:           "Great hotel!",
		ReviewDate:        time.Now(),
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	suite.handlers.CreateReview(rr, req)
	
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid request data")
}

func (suite *HandlersTestSuite) TestCreateReview_InvalidHotelID() {
	reqBody := CreateReviewRequest{
		ProviderID:        uuid.New().String(),
		HotelID:           "invalid-uuid",
		ReviewerInfoID:    uuid.New().String(),
		Rating:            4.5,
		Comment:           "Great hotel!",
		ReviewDate:        time.Now(),
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	suite.handlers.CreateReview(rr, req)
	
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid request data")
}

func (suite *HandlersTestSuite) TestCreateReview_InvalidReviewerInfoID() {
	reqBody := CreateReviewRequest{
		ProviderID:        uuid.New().String(),
		HotelID:           uuid.New().String(),
		ReviewerInfoID:    "invalid-uuid",
		Rating:            4.5,
		Comment:           "Great hotel!",
		ReviewDate:        time.Now(),
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	suite.handlers.CreateReview(rr, req)
	
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid request data")
}

func (suite *HandlersTestSuite) TestCreateHotel_ServiceError() {
	reqBody := CreateHotelRequest{
		Name:        "Test Hotel",
		Address:     "123 Test St",
		City:        "Test City",
		Country:     "Test Country",
		StarRating:  4,
		Latitude:    40.7128,
		Longitude:   -74.0060,
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/hotels", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	suite.mockService.On("CreateHotel", mock.Anything, mock.AnythingOfType("*domain.Hotel")).Return(errors.New("service error"))
	
	suite.handlers.CreateHotel(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to create hotel")
}

func (suite *HandlersTestSuite) TestCreateProvider_Success() {
	reqBody := CreateProviderRequest{
		Name:     "Test Provider",
		BaseURL:  "https://example.com",
		IsActive: true,
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	suite.mockService.On("CreateProvider", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)
	
	suite.handlers.CreateProvider(rr, req)
	
	suite.Equal(http.StatusCreated, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.True(response.Success)
	suite.NotNil(response.Data)
}

func (suite *HandlersTestSuite) TestCreateProvider_InvalidJSON() {
	req := httptest.NewRequest("POST", "/providers", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	suite.handlers.CreateProvider(rr, req)
	
	suite.Equal(http.StatusBadRequest, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Invalid request format")
}

func (suite *HandlersTestSuite) TestGetHotel_NotFound() {
	hotelID := uuid.New()
	
	req := httptest.NewRequest("GET", "/hotels/"+hotelID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": hotelID.String()})
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetHotelByID", mock.Anything, hotelID).Return((*domain.Hotel)(nil), errors.New("not found"))
	
	suite.handlers.GetHotel(rr, req)
	
	suite.Equal(http.StatusNotFound, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Hotel not found")
}

func (suite *HandlersTestSuite) TestGetProvider_NotFound() {
	providerID := uuid.New()
	
	req := httptest.NewRequest("GET", "/providers/"+providerID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": providerID.String()})
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetProviderByID", mock.Anything, providerID).Return((*domain.Provider)(nil), errors.New("not found"))
	
	suite.handlers.GetProvider(rr, req)
	
	suite.Equal(http.StatusNotFound, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Provider not found")
}

func (suite *HandlersTestSuite) TestListHotels_ServiceError() {
	req := httptest.NewRequest("GET", "/hotels", nil)
	rr := httptest.NewRecorder()
	
	suite.mockService.On("ListHotels", mock.Anything, 20, 0).Return([]domain.Hotel{}, errors.New("service error"))
	
	suite.handlers.ListHotels(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to list hotels")
}

func (suite *HandlersTestSuite) TestListProviders_ServiceError() {
	req := httptest.NewRequest("GET", "/providers", nil)
	rr := httptest.NewRecorder()
	
	suite.mockService.On("ListProviders", mock.Anything, 20, 0).Return([]domain.Provider{}, errors.New("service error"))
	
	suite.handlers.ListProviders(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to list providers")
}

func (suite *HandlersTestSuite) TestGetReviewSummary_NotFound() {
	hotelID := uuid.New()
	
	req := httptest.NewRequest("GET", "/hotels/"+hotelID.String()+"/summary", nil)
	req = mux.SetURLVars(req, map[string]string{"hotel_id": hotelID.String()})
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetReviewSummary", mock.Anything, hotelID).Return((*domain.ReviewSummary)(nil), errors.New("not found"))
	
	suite.handlers.GetReviewSummary(rr, req)
	
	suite.Equal(http.StatusNotFound, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Review summary not found")
}

func (suite *HandlersTestSuite) TestGetTopRatedHotels_ServiceError() {
	req := httptest.NewRequest("GET", "/analytics/top-hotels", nil)
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetTopRatedHotels", mock.Anything, 10).Return([]domain.Hotel{}, errors.New("service error"))
	
	suite.handlers.GetTopRatedHotels(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to get top rated hotels")
}

func (suite *HandlersTestSuite) TestGetRecentReviews_ServiceError() {
	req := httptest.NewRequest("GET", "/analytics/recent-reviews", nil)
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetRecentReviews", mock.Anything, 20).Return([]domain.Review{}, errors.New("service error"))
	
	suite.handlers.GetRecentReviews(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to get recent reviews")
}

func (suite *HandlersTestSuite) TestGetReviewsByHotel_ServiceError() {
	hotelID := uuid.New()
	
	req := httptest.NewRequest("GET", "/hotels/"+hotelID.String()+"/reviews", nil)
	req = mux.SetURLVars(req, map[string]string{"hotel_id": hotelID.String()})
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetReviewsByHotel", mock.Anything, hotelID, 20, 0).Return([]domain.Review{}, errors.New("service error"))
	
	suite.handlers.GetReviewsByHotel(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to get reviews")
}

func (suite *HandlersTestSuite) TestGetReviewsByProvider_ServiceError() {
	providerID := uuid.New()
	
	req := httptest.NewRequest("GET", "/providers/"+providerID.String()+"/reviews", nil)
	req = mux.SetURLVars(req, map[string]string{"provider_id": providerID.String()})
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetReviewsByProvider", mock.Anything, providerID, 20, 0).Return([]domain.Review{}, errors.New("service error"))
	
	suite.handlers.GetReviewsByProvider(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to get reviews")
}

func (suite *HandlersTestSuite) TestSearchReviews_ServiceError() {
	req := httptest.NewRequest("GET", "/reviews?q=test", nil)
	rr := httptest.NewRecorder()
	
	suite.mockService.On("SearchReviews", mock.Anything, "test", mock.Anything, 20, 0).Return([]domain.Review{}, errors.New("service error"))
	
	suite.handlers.SearchReviews(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to search reviews")
}

func (suite *HandlersTestSuite) TestGetProcessingHistory_ServiceError() {
	providerID := uuid.New()
	
	req := httptest.NewRequest("GET", "/processing/history/"+providerID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"provider_id": providerID.String()})
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetProcessingHistory", mock.Anything, providerID, 20, 0).Return([]domain.ReviewProcessingStatus{}, errors.New("service error"))
	
	suite.handlers.GetProcessingHistory(rr, req)
	
	suite.Equal(http.StatusInternalServerError, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Failed to get processing history")
}

func (suite *HandlersTestSuite) TestGetProcessingStatus_NotFound() {
	processingID := uuid.New()
	
	req := httptest.NewRequest("GET", "/processing/"+processingID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": processingID.String()})
	rr := httptest.NewRecorder()
	
	suite.mockService.On("GetProcessingStatus", mock.Anything, processingID).Return((*domain.ReviewProcessingStatus)(nil), errors.New("not found"))
	
	suite.handlers.GetProcessingStatus(rr, req)
	
	suite.Equal(http.StatusNotFound, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.False(response.Success)
	suite.Contains(response.Error, "Processing status not found")
}

func (suite *HandlersTestSuite) TestGetTopRatedHotels_WithLimit() {
	req := httptest.NewRequest("GET", "/analytics/top-hotels?limit=5", nil)
	rr := httptest.NewRecorder()
	
	expectedHotels := []domain.Hotel{
		{
			ID:   uuid.New(),
			Name: "Top Hotel",
		},
	}
	
	suite.mockService.On("GetTopRatedHotels", mock.Anything, 5).Return(expectedHotels, nil)
	
	suite.handlers.GetTopRatedHotels(rr, req)
	
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.True(response.Success)
	suite.NotNil(response.Data)
	suite.Equal(5, response.Meta.Limit)
}

func (suite *HandlersTestSuite) TestGetRecentReviews_WithLimit() {
	req := httptest.NewRequest("GET", "/analytics/recent-reviews?limit=5", nil)
	rr := httptest.NewRecorder()
	
	expectedReviews := []domain.Review{
		{
			ID:      uuid.New(),
			Comment: "Recent review",
		},
	}
	
	suite.mockService.On("GetRecentReviews", mock.Anything, 5).Return(expectedReviews, nil)
	
	suite.handlers.GetRecentReviews(rr, req)
	
	suite.Equal(http.StatusOK, rr.Code)
	
	var response APIResponse
	json.Unmarshal(rr.Body.Bytes(), &response)
	suite.True(response.Success)
	suite.NotNil(response.Data)
	suite.Equal(5, response.Meta.Limit)
}