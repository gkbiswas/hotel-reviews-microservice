package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
)

// Define test error variables for handler testing
var (
	ErrPasswordTooWeak     = errors.New("password is too weak")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// MockPasswordServiceForHandlers for testing (to avoid conflicts)
type MockPasswordServiceForHandlers struct {
	mock.Mock
}

func (m *MockPasswordServiceForHandlers) HashPassword(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *MockPasswordServiceForHandlers) ComparePassword(hashedPassword, password string) error {
	args := m.Called(hashedPassword, password)
	return args.Error(0)
}

func (m *MockPasswordServiceForHandlers) GenerateRandomPassword(length int) (string, error) {
	args := m.Called(length)
	return args.String(0), args.Error(1)
}

func (m *MockPasswordServiceForHandlers) ValidatePasswordStrength(password string) error {
	args := m.Called(password)
	return args.Error(0)
}

// Test helper functions for comprehensive handler testing
func createTestAuthHandlersComprehensive() (*AuthHandlers, *MockPasswordServiceForHandlers) {
	mockPasswordService := &MockPasswordServiceForHandlers{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a real infrastructure service with nil dependencies for testing
	authService := &infrastructure.AuthenticationService{}

	handlers := &AuthHandlers{
		authService:     authService,
		passwordService: mockPasswordService,
		logger:          logger,
	}

	return handlers, mockPasswordService
}

func createTestUserForHandlers() *domain.User {
	return &domain.User{
		ID:        uuid.New(),
		Username:  "testuser",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestSessionForHandlers() *domain.Session {
	return &domain.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
		CreatedAt:    time.Now(),
	}
}

// Test HealthCheck handler - this should work without mocks
func TestAuthHandlers_HealthCheck_Comprehensive(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handlers.HealthCheck(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "service is healthy", response["message"])
}

// Test utility functions that don't require service dependencies
func TestAuthHandlers_GetClientIP_Comprehensive(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()

	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.100",
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 192.168.1.100",
			},
			expectedIP: "203.0.113.1",
		},
		{
			name:       "RemoteAddr fallback",
			remoteAddr: "127.0.0.1:54321",
			expectedIP: "127.0.0.1:54321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}

			ip := handlers.getClientIP(req)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

// Test pagination parsing utility
func TestAuthHandlers_ParsePaginationParams_Comprehensive(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()

	tests := []struct {
		name           string
		queryParams    map[string]string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name: "valid parameters",
			queryParams: map[string]string{
				"limit":  "25",
				"offset": "50",
			},
			expectedLimit:  25,
			expectedOffset: 50,
		},
		{
			name:           "no parameters - defaults",
			queryParams:    map[string]string{},
			expectedLimit:  20,
			expectedOffset: 0,
		},
		{
			name: "invalid parameters - use defaults",
			queryParams: map[string]string{
				"limit":  "invalid",
				"offset": "also_invalid",
			},
			expectedLimit:  20,
			expectedOffset: 0,
		},
		{
			name: "limit too high - capped to max",
			queryParams: map[string]string{
				"limit": "1000",
			},
			expectedLimit:  100,
			expectedOffset: 0,
		},
		{
			name: "negative values - use defaults",
			queryParams: map[string]string{
				"limit":  "-10",
				"offset": "-5",
			},
			expectedLimit:  20,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()

			limit, offset := handlers.parsePaginationParams(req)
			assert.Equal(t, tt.expectedLimit, limit)
			assert.Equal(t, tt.expectedOffset, offset)
		})
	}
}

// Test register handler with password hashing error
func TestAuthHandlers_Register_PasswordValidation(t *testing.T) {
	handlers, mockPasswordService := createTestAuthHandlersComprehensive()

	// Test password hashing failure
	t.Run("password hashing fails", func(t *testing.T) {
		// Use a valid password that passes validation but fails hashing
		validPassword := "validpassword123"
		mockPasswordService.On("HashPassword", validPassword).Return("", errors.New("hashing failed"))

		reqBody := RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: validPassword,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handlers.Register(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "error")

		mockPasswordService.AssertExpectations(t)
	})
}

// Test request validation
func TestAuthHandlers_ValidateRequest_Comprehensive(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()

	tests := []struct {
		name        string
		request     interface{}
		expectError bool
	}{
		{
			name: "valid register request",
			request: RegisterRequest{
				Username:  "validuser",
				Email:     "valid@example.com",
				Password:  "validpassword123",
				FirstName: "Valid",
				LastName:  "User",
			},
			expectError: false,
		},
		{
			name: "invalid email",
			request: RegisterRequest{
				Username: "validuser",
				Email:    "invalid-email",
				Password: "validpassword123",
			},
			expectError: true,
		},
		{
			name: "short password",
			request: RegisterRequest{
				Username: "validuser",
				Email:    "valid@example.com",
				Password: "short",
			},
			expectError: true,
		},
		{
			name: "missing required fields",
			request: RegisterRequest{
				FirstName: "Test",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handlers.validateRequest(tt.request)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test error response helper
func TestAuthHandlers_WriteErrorResponse_Comprehensive(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()

	tests := []struct {
		name           string
		statusCode     int
		message        string
		details        string
		expectedStatus int
	}{
		{
			name:           "bad request error",
			statusCode:     http.StatusBadRequest,
			message:        "Invalid request",
			details:        "Missing required field",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized error",
			statusCode:     http.StatusUnauthorized,
			message:        "Unauthorized",
			details:        "Invalid credentials",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "internal server error",
			statusCode:     http.StatusInternalServerError,
			message:        "Internal error",
			details:        "Database connection failed",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			handlers.writeErrorResponse(rec, tt.statusCode, tt.message, tt.details)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.message, response["error"])
			assert.Equal(t, tt.details, response["details"])
			assert.Contains(t, response, "time")
			assert.Equal(t, float64(tt.statusCode), response["code"])
		})
	}
}

// Test success response helper
func TestAuthHandlers_WriteSuccessResponse_Comprehensive(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()

	testData := map[string]interface{}{
		"id":   "123",
		"name": "test",
	}

	tests := []struct {
		name           string
		statusCode     int
		message        string
		data           interface{}
		expectedStatus int
	}{
		{
			name:           "successful creation",
			statusCode:     http.StatusCreated,
			message:        "User created successfully",
			data:           testData,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "successful retrieval",
			statusCode:     http.StatusOK,
			message:        "Users retrieved successfully",
			data:           []interface{}{testData},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "no data response",
			statusCode:     http.StatusOK,
			message:        "Operation completed",
			data:           nil,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			handlers.writeSuccessResponse(rec, tt.statusCode, tt.message, tt.data)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.message, response["message"])
			assert.Contains(t, response, "time")

			if tt.data != nil {
				assert.Contains(t, response, "data")
			}
		})
	}
}

// Test request body parsing errors
func TestAuthHandlers_Register_RequestParsingErrors(t *testing.T) {
	handlers, mockPasswordService := createTestAuthHandlersComprehensive()

	tests := []struct {
		name           string
		requestBody    string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "invalid JSON",
			requestBody:    `{"invalid": json}`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty request body",
			requestBody:    "",
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-JSON content type",
			requestBody:    "some data",
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", tt.contentType)
			rec := httptest.NewRecorder()

			handlers.Register(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "error")

			mockPasswordService.AssertExpectations(t)
		})
	}
}

// Test Login request parsing errors
func TestAuthHandlers_Login_RequestParsingErrors(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "malformed JSON",
			requestBody:    `{"email": "test@example.com", "password":}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing required fields",
			requestBody:    `{"email": "test@example.com"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid email format",
			requestBody:    `{"email": "invalid-email", "password": "password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handlers.Login(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "error")
		})
	}
}

// Test getUserFromContext utility
func TestAuthHandlers_GetUserFromContext_Comprehensive(t *testing.T) {
	handlers, _ := createTestAuthHandlersComprehensive()
	testUser := createTestUserForHandlers()

	tests := []struct {
		name         string
		contextUser  *domain.User
		expectedUser *domain.User
	}{
		{
			name:         "user in context",
			contextUser:  testUser,
			expectedUser: testUser,
		},
		{
			name:         "no user in context",
			contextUser:  nil,
			expectedUser: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			if tt.contextUser != nil {
				ctx := context.WithValue(req.Context(), "user", tt.contextUser)
				req = req.WithContext(ctx)
			}

			user := handlers.getUserFromContext(req)
			assert.Equal(t, tt.expectedUser, user)
		})
	}
}

// Test that handlers properly handle missing authentication service methods
// This tests the error handling when the underlying service methods return errors
func TestAuthHandlers_ServiceErrorHandling(t *testing.T) {
	handlers, mockPasswordService := createTestAuthHandlersComprehensive()

	// Test that when the service has nil dependencies, it handles errors gracefully
	t.Run("Register with service error", func(t *testing.T) {
		// Mock password hashing to succeed so we can test service error
		mockPasswordService.On("HashPassword", "password123").Return("hashedpassword", nil)
		
		reqBody := RegisterRequest{
			Username:  "testuser",
			Email:     "test@example.com",
			Password:  "password123",
			FirstName: "Test",
			LastName:  "User",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handlers.Register(rec, req)

		// Should return an error status since the underlying service will fail
		assert.True(t, rec.Code >= 400, "Expected error status code, got %d", rec.Code)
	})

	t.Run("Login with service error", func(t *testing.T) {
		reqBody := LoginRequest{
			Email:    "test@example.com",
			Password: "password123",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Real-IP", "192.168.1.1")
		req.Header.Set("User-Agent", "test-agent")
		rec := httptest.NewRecorder()

		handlers.Login(rec, req)

		// Should return an error status since the underlying service will fail
		assert.True(t, rec.Code >= 400, "Expected error status code, got %d", rec.Code)
	})
}
