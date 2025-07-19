package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
)

// TestAuthenticationService_CheckPermission_NilPointerFix tests the fix for null pointer issues
func TestAuthenticationService_CheckPermission_NilPointerFix(t *testing.T) {
	tests := []struct {
		name          string
		authService   *AuthenticationService
		userID        uuid.UUID
		resource      string
		action        string
		expectError   bool
		expectedError string
	}{
		{
			name: "nil rbacService should return error not panic",
			authService: &AuthenticationService{
				rbacService: nil, // This causes the null pointer
			},
			userID:        uuid.New(),
			resource:      "reviews",
			action:        "read",
			expectError:   true,
			expectedError: "RBAC service not initialized",
		},
		{
			name: "valid rbacService should work",
			authService: &AuthenticationService{
				rbacService: createValidRBACService(), // Valid service
			},
			userID:      uuid.New(),
			resource:    "reviews",
			action:      "read",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// This should not panic
			hasPermission, err := tt.authService.CheckPermission(ctx, tt.userID, tt.resource, tt.action)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.False(t, hasPermission)
			} else {
				require.NoError(t, err)
				// Mock returns true for simplicity
				assert.True(t, hasPermission)
			}
		})
	}
}

// createValidRBACService creates a valid RBAC service for testing
func createValidRBACService() *RBACService {
	// Create a minimal valid RBAC service
	return &RBACService{
		authRepo:       &mockAuthRepo{},
		logger:         nil, // Can be nil for tests
		circuitBreaker: nil, // Can be nil for tests
		retryManager:   nil, // Can be nil for tests
	}
}

// mockAuthRepo is a simple mock for testing
type mockAuthRepo struct{}

func (m *mockAuthRepo) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	return []domain.Role{}, nil
}

func (m *mockAuthRepo) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	return []domain.Permission{}, nil
}

func (m *mockAuthRepo) CreateUser(ctx context.Context, user *domain.User) error {
	return nil
}

func (m *mockAuthRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}

func (m *mockAuthRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateUser(ctx context.Context, user *domain.User) error {
	return nil
}

func (m *mockAuthRepo) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) CreateApiKey(ctx context.Context, apiKey *domain.ApiKey) error {
	return nil
}

func (m *mockAuthRepo) GetApiKeyByKey(ctx context.Context, key string) (*domain.ApiKey, error) {
	return nil, nil
}

func (m *mockAuthRepo) DeleteApiKey(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) CreateRole(ctx context.Context, role *domain.Role) error {
	return nil
}

func (m *mockAuthRepo) AssignRole(ctx context.Context, userID uuid.UUID, role string) error {
	return nil
}

func (m *mockAuthRepo) RevokeRole(ctx context.Context, userID uuid.UUID, role string) error {
	return nil
}

func (m *mockAuthRepo) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	return nil
}

func (m *mockAuthRepo) CreateSession(ctx context.Context, session *domain.Session) error {
	return nil
}

func (m *mockAuthRepo) GetSessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	return nil, nil
}

func (m *mockAuthRepo) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) IncrementLoginAttempts(ctx context.Context, email string) error {
	return nil
}

func (m *mockAuthRepo) ResetLoginAttempts(ctx context.Context, email string) error {
	return nil
}

func (m *mockAuthRepo) LockAccount(ctx context.Context, email string, duration time.Duration) error {
	return nil
}

func (m *mockAuthRepo) IsAccountLocked(ctx context.Context, email string) (bool, error) {
	return false, nil
}

func (m *mockAuthRepo) AssignPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return nil
}

// Missing interface methods from domain.AuthRepository
func (m *mockAuthRepo) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}

func (m *mockAuthRepo) ListUsers(ctx context.Context, limit, offset int) ([]domain.User, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateUserLastLogin(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) LockUser(ctx context.Context, userID uuid.UUID, until time.Time) error {
	return nil
}

func (m *mockAuthRepo) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	return nil, nil
}

func (m *mockAuthRepo) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateRole(ctx context.Context, role *domain.Role) error {
	return nil
}

func (m *mockAuthRepo) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) ListRoles(ctx context.Context, limit, offset int) ([]domain.Role, error) {
	return nil, nil
}

func (m *mockAuthRepo) CreatePermission(ctx context.Context, permission *domain.Permission) error {
	return nil
}

func (m *mockAuthRepo) GetPermissionByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	return nil, nil
}

func (m *mockAuthRepo) GetPermissionByName(ctx context.Context, name string) (*domain.Permission, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdatePermission(ctx context.Context, permission *domain.Permission) error {
	return nil
}

func (m *mockAuthRepo) DeletePermission(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) ListPermissions(ctx context.Context, limit, offset int) ([]domain.Permission, error) {
	return nil, nil
}

func (m *mockAuthRepo) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) RemoveRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) GetRoleUsers(ctx context.Context, roleID uuid.UUID) ([]domain.User, error) {
	return nil, nil
}

func (m *mockAuthRepo) RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Permission, error) {
	// Return some test permissions that include "reviews:read"
	return []domain.Permission{
		{
			ID:       uuid.New(),
			Name:     "reviews:read",
			Resource: "reviews",
			Action:   "read",
		},
		{
			ID:       uuid.New(),
			Name:     "reviews:write",
			Resource: "reviews",
			Action:   "write",
		},
	}, nil
}

func (m *mockAuthRepo) GetSessionByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	return nil, nil
}

func (m *mockAuthRepo) GetSessionByAccessToken(ctx context.Context, accessToken string) (*domain.Session, error) {
	return nil, nil
}

func (m *mockAuthRepo) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*domain.Session, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateSession(ctx context.Context, session *domain.Session) error {
	return nil
}

func (m *mockAuthRepo) DeleteSessionByID(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) CleanupExpiredSessions(ctx context.Context) error {
	return nil
}

func (m *mockAuthRepo) GetApiKeyByID(ctx context.Context, id uuid.UUID) (*domain.ApiKey, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateApiKey(ctx context.Context, apiKey *domain.ApiKey) error {
	return nil
}

func (m *mockAuthRepo) ListApiKeys(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.ApiKey, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateApiKeyUsage(ctx context.Context, apiKeyID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) GetAuditLogByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	return nil, nil
}

func (m *mockAuthRepo) ListAuditLogs(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	return nil, nil
}

func (m *mockAuthRepo) CreateLoginAttempt(ctx context.Context, attempt *domain.LoginAttempt) error {
	return nil
}

func (m *mockAuthRepo) GetLoginAttempts(ctx context.Context, email string, ipAddress string, since time.Time) ([]domain.LoginAttempt, error) {
	return nil, nil
}

func (m *mockAuthRepo) CleanupOldLoginAttempts(ctx context.Context, before time.Time) error {
	return nil
}

// TestAuthenticationService_NilChecks tests all methods for nil pointer safety
func TestAuthenticationService_NilChecks(t *testing.T) {
	t.Run("CheckPermission with nil rbacService", func(t *testing.T) {
		auth := &AuthenticationService{
			rbacService: nil,
		}

		_, err := auth.CheckPermission(context.Background(), uuid.New(), "resource", "action")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RBAC service not initialized")
	})

	t.Run("CreateApiKey with nil apiKeyService", func(t *testing.T) {
		auth := &AuthenticationService{
			apiKeyService: nil,
		}

		_, err := auth.CreateApiKey(context.Background(), uuid.New(), "test", []string{"read"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key service not initialized")
	})

	t.Run("ValidateApiKey with nil apiKeyService", func(t *testing.T) {
		auth := &AuthenticationService{
			apiKeyService: nil,
		}

		_, err := auth.ValidateApiKey(context.Background(), "test-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key service not initialized")
	})
}
