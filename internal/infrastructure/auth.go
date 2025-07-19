package infrastructure

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/config"
)

const (
	// JWT constants
	AccessTokenExpiry  = 15 * time.Minute
	RefreshTokenExpiry = 7 * 24 * time.Hour
	AccessTokenType    = "access"
	RefreshTokenType   = "refresh"

	// Password constraints
	MinPasswordLength = 8
	MaxPasswordLength = 128

	// Rate limiting constants
	MaxLoginAttempts    = 5
	LoginAttemptWindow  = 15 * time.Minute
	AccountLockDuration = 30 * time.Minute

	// API Key constants
	ApiKeyLength = 32
	ApiKeyPrefix = "hr_"
)

// JWTService implements JWT-based authentication
type JWTService struct {
	secretKey     string
	issuer        string
	logger        *slog.Logger
	circuitBreaker *CircuitBreaker
	retryManager  *RetryManager
}

// NewJWTService creates a new JWT service with circuit breaker and retry support
func NewJWTService(cfg *config.Config, logger *slog.Logger, cb *CircuitBreaker, retry *RetryManager) *JWTService {
	return &JWTService{
		secretKey:     cfg.Auth.JWTSecret,
		issuer:        cfg.Auth.JWTIssuer,
		logger:        logger,
		circuitBreaker: cb,
		retryManager:  retry,
	}
}

// GenerateAccessToken generates a new access token with circuit breaker protection
func (j *JWTService) GenerateAccessToken(ctx context.Context, user *domain.User) (string, error) {
	return j.executeWithResilience(ctx, "GenerateAccessToken", func() (string, error) {
		return j.generateToken(user, AccessTokenType, AccessTokenExpiry)
	})
}

// GenerateRefreshToken generates a new refresh token with circuit breaker protection
func (j *JWTService) GenerateRefreshToken(ctx context.Context, user *domain.User) (string, error) {
	return j.executeWithResilience(ctx, "GenerateRefreshToken", func() (string, error) {
		return j.generateToken(user, RefreshTokenType, RefreshTokenExpiry)
	})
}

// ValidateAccessToken validates an access token with circuit breaker protection
func (j *JWTService) ValidateAccessToken(ctx context.Context, tokenString string) (*domain.JWTClaims, error) {
	return j.executeWithResilienceForClaims(ctx, "ValidateAccessToken", func() (*domain.JWTClaims, error) {
		return j.validateToken(tokenString, AccessTokenType)
	})
}

// ValidateRefreshToken validates a refresh token with circuit breaker protection
func (j *JWTService) ValidateRefreshToken(ctx context.Context, tokenString string) (*domain.JWTClaims, error) {
	return j.executeWithResilienceForClaims(ctx, "ValidateRefreshToken", func() (*domain.JWTClaims, error) {
		return j.validateToken(tokenString, RefreshTokenType)
	})
}

// ExtractUserFromToken extracts user information from token
func (j *JWTService) ExtractUserFromToken(ctx context.Context, tokenString string) (*domain.User, error) {
	claims, err := j.ValidateAccessToken(ctx, tokenString)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:       claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
	}

	return user, nil
}

// generateToken generates a JWT token
func (j *JWTService) generateToken(user *domain.User, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := &domain.JWTClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Roles:     j.extractRoleNames(user.Roles),
		TokenType: tokenType,
		ExpiresAt: now.Add(expiry).Unix(),
		IssuedAt:  now.Unix(),
		Issuer:    j.issuer,
		Subject:   user.ID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    claims.UserID,
		"username":   claims.Username,
		"email":      claims.Email,
		"roles":      claims.Roles,
		"token_type": claims.TokenType,
		"exp":        claims.ExpiresAt,
		"iat":        claims.IssuedAt,
		"iss":        claims.Issuer,
		"sub":        claims.Subject,
	})

	tokenString, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		return "", errors.Wrap(err, "failed to sign token")
	}

	return tokenString, nil
}

// validateToken validates a JWT token
func (j *JWTService) validateToken(tokenString, expectedType string) (*domain.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to parse token")
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims format")
	}

	tokenType, ok := claims["token_type"].(string)
	if !ok || tokenType != expectedType {
		return nil, errors.New("invalid token type")
	}

	userID, err := uuid.Parse(claims["user_id"].(string))
	if err != nil {
		return nil, errors.Wrap(err, "invalid user ID in token")
	}

	jwtClaims := &domain.JWTClaims{
		UserID:    userID,
		Username:  claims["username"].(string),
		Email:     claims["email"].(string),
		Roles:     j.extractRolesFromClaims(claims["roles"]),
		TokenType: tokenType,
		ExpiresAt: int64(claims["exp"].(float64)),
		IssuedAt:  int64(claims["iat"].(float64)),
		Issuer:    claims["iss"].(string),
		Subject:   claims["sub"].(string),
	}

	return jwtClaims, nil
}

// extractRoleNames extracts role names from role entities
func (j *JWTService) extractRoleNames(roles []domain.Role) []string {
	roleNames := make([]string, len(roles))
	for i, role := range roles {
		roleNames[i] = role.Name
	}
	return roleNames
}

// extractRolesFromClaims extracts roles from JWT claims
func (j *JWTService) extractRolesFromClaims(rolesInterface interface{}) []string {
	if rolesInterface == nil {
		return []string{}
	}

	switch roles := rolesInterface.(type) {
	case []interface{}:
		roleNames := make([]string, len(roles))
		for i, role := range roles {
			roleNames[i] = role.(string)
		}
		return roleNames
	case []string:
		return roles
	default:
		return []string{}
	}
}

// executeWithResilience executes a function with circuit breaker and retry support
func (j *JWTService) executeWithResilience(ctx context.Context, operation string, fn func() (string, error)) (string, error) {
	if j.circuitBreaker != nil {
		result, err := j.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			if j.retryManager != nil {
				return j.retryManager.Execute(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
					return fn()
				})
			}
			return fn()
		})
		if err != nil {
			return "", err
		}
		return result.(string), nil
	}

	if j.retryManager != nil {
		result, err := j.retryManager.Execute(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
			return fn()
		})
		if err != nil {
			return "", err
		}
		return result.(string), nil
	}

	return fn()
}

// executeWithResilienceForClaims executes a function returning claims with circuit breaker and retry support
func (j *JWTService) executeWithResilienceForClaims(ctx context.Context, operation string, fn func() (*domain.JWTClaims, error)) (*domain.JWTClaims, error) {
	if j.circuitBreaker != nil {
		result, err := j.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			if j.retryManager != nil {
				return j.retryManager.Execute(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
					return fn()
				})
			}
			return fn()
		})
		if err != nil {
			return nil, err
		}
		return result.(*domain.JWTClaims), nil
	}

	if j.retryManager != nil {
		result, err := j.retryManager.Execute(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
			return fn()
		})
		if err != nil {
			return nil, err
		}
		return result.(*domain.JWTClaims), nil
	}

	return fn()
}

// PasswordService implements password hashing and validation
type PasswordService struct {
	logger *slog.Logger
}

// NewPasswordService creates a new password service
func NewPasswordService(logger *slog.Logger) *PasswordService {
	return &PasswordService{
		logger: logger,
	}
}

// HashPassword hashes a password using bcrypt
func (p *PasswordService) HashPassword(password string) (string, error) {
	if err := p.ValidatePasswordStrength(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, "failed to hash password")
	}

	return string(hash), nil
}

// ComparePassword compares a password with its hash
func (p *PasswordService) ComparePassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return errors.Wrap(err, "password comparison failed")
	}
	return nil
}

// GenerateRandomPassword generates a random password
func (p *PasswordService) GenerateRandomPassword(length int) (string, error) {
	if length < MinPasswordLength {
		length = MinPasswordLength
	}
	if length > MaxPasswordLength {
		length = MaxPasswordLength
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		randomIndex := make([]byte, 1)
		_, err := rand.Read(randomIndex)
		if err != nil {
			return "", errors.Wrap(err, "failed to generate random password")
		}
		b[i] = charset[randomIndex[0]%byte(len(charset))]
	}

	return string(b), nil
}

// ValidatePasswordStrength validates password strength
func (p *PasswordService) ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters long", MinPasswordLength)
	}

	if len(password) > MaxPasswordLength {
		return fmt.Errorf("password must not exceed %d characters", MaxPasswordLength)
	}

	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, char := range password {
		switch {
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

// AuthRepository implements authentication repository with circuit breaker support
type AuthRepository struct {
	db             *gorm.DB
	logger         *slog.Logger
	circuitBreaker *CircuitBreaker
	retryManager    *RetryManager
}

// NewAuthRepository creates a new authentication repository
func NewAuthRepository(db *gorm.DB, logger *slog.Logger, cb *CircuitBreaker, retry *RetryManager) *AuthRepository {
	return &AuthRepository{
		db:             db,
		logger:         logger,
		circuitBreaker: cb,
		retryManager:    retry,
	}
}

// CreateUser creates a new user
func (r *AuthRepository) CreateUser(ctx context.Context, user *domain.User) error {
	return r.executeWithDB(ctx, "CreateUser", func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(user).Error
	})
}

// GetUserByID retrieves a user by ID
func (r *AuthRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.executeWithDB(ctx, "GetUserByID", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("Roles").Preload("Roles.Permissions").First(&user, "id = ?", id).Error
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.executeWithDB(ctx, "GetUserByEmail", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("Roles").Preload("Roles.Permissions").First(&user, "email = ?", email).Error
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (r *AuthRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.executeWithDB(ctx, "GetUserByUsername", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("Roles").Preload("Roles.Permissions").First(&user, "username = ?", username).Error
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates a user
func (r *AuthRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	return r.executeWithDB(ctx, "UpdateUser", func(db *gorm.DB) error {
		return db.WithContext(ctx).Save(user).Error
	})
}

// DeleteUser deletes a user
func (r *AuthRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return r.executeWithDB(ctx, "DeleteUser", func(db *gorm.DB) error {
		return db.WithContext(ctx).Delete(&domain.User{}, "id = ?", id).Error
	})
}

// ListUsers lists users with pagination
func (r *AuthRepository) ListUsers(ctx context.Context, limit, offset int) ([]domain.User, error) {
	var users []domain.User
	err := r.executeWithDB(ctx, "ListUsers", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("Roles").Limit(limit).Offset(offset).Find(&users).Error
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUserLastLogin updates user's last login timestamp
func (r *AuthRepository) UpdateUserLastLogin(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return r.executeWithDB(ctx, "UpdateUserLastLogin", func(db *gorm.DB) error {
		return db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("last_login_at", now).Error
	})
}

// IncrementFailedAttempts increments failed login attempts
func (r *AuthRepository) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	return r.executeWithDB(ctx, "IncrementFailedAttempts", func(db *gorm.DB) error {
		return db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("failed_attempts", gorm.Expr("failed_attempts + 1")).Error
	})
}

// ResetFailedAttempts resets failed login attempts
func (r *AuthRepository) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	return r.executeWithDB(ctx, "ResetFailedAttempts", func(db *gorm.DB) error {
		return db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
			"failed_attempts": 0,
			"locked_until":    nil,
		}).Error
	})
}

// LockUser locks a user account until specified time
func (r *AuthRepository) LockUser(ctx context.Context, userID uuid.UUID, until time.Time) error {
	return r.executeWithDB(ctx, "LockUser", func(db *gorm.DB) error {
		return db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("locked_until", until).Error
	})
}

// CreateSession creates a new session
func (r *AuthRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	return r.executeWithDB(ctx, "CreateSession", func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(session).Error
	})
}

// GetSessionByAccessToken retrieves session by access token
func (r *AuthRepository) GetSessionByAccessToken(ctx context.Context, accessToken string) (*domain.Session, error) {
	var session domain.Session
	err := r.executeWithDB(ctx, "GetSessionByAccessToken", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("User").First(&session, "access_token = ? AND is_active = ?", accessToken, true).Error
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetSessionByRefreshToken retrieves session by refresh token
func (r *AuthRepository) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*domain.Session, error) {
	var session domain.Session
	err := r.executeWithDB(ctx, "GetSessionByRefreshToken", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("User").First(&session, "refresh_token = ? AND is_active = ?", refreshToken, true).Error
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateSession updates a session
func (r *AuthRepository) UpdateSession(ctx context.Context, session *domain.Session) error {
	return r.executeWithDB(ctx, "UpdateSession", func(db *gorm.DB) error {
		return db.WithContext(ctx).Save(session).Error
	})
}

// DeleteSession deletes a session
func (r *AuthRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return r.executeWithDB(ctx, "DeleteSession", func(db *gorm.DB) error {
		return db.WithContext(ctx).Delete(&domain.Session{}, "id = ?", id).Error
	})
}

// DeleteUserSessions deletes all sessions for a user
func (r *AuthRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	return r.executeWithDB(ctx, "DeleteUserSessions", func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.Session{}).Error
	})
}

// CleanupExpiredSessions removes expired sessions
func (r *AuthRepository) CleanupExpiredSessions(ctx context.Context) error {
	return r.executeWithDB(ctx, "CleanupExpiredSessions", func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&domain.Session{}).Error
	})
}

// CreateApiKey creates a new API key
func (r *AuthRepository) CreateApiKey(ctx context.Context, apiKey *domain.ApiKey) error {
	return r.executeWithDB(ctx, "CreateApiKey", func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(apiKey).Error
	})
}

// GetApiKeyByKey retrieves API key by key value
func (r *AuthRepository) GetApiKeyByKey(ctx context.Context, key string) (*domain.ApiKey, error) {
	var apiKey domain.ApiKey
	err := r.executeWithDB(ctx, "GetApiKeyByKey", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("User").First(&apiKey, "key = ? AND is_active = ?", key, true).Error
	})
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

// UpdateApiKeyUsage updates API key usage
func (r *AuthRepository) UpdateApiKeyUsage(ctx context.Context, apiKeyID uuid.UUID) error {
	now := time.Now()
	return r.executeWithDB(ctx, "UpdateApiKeyUsage", func(db *gorm.DB) error {
		return db.WithContext(ctx).Model(&domain.ApiKey{}).Where("id = ?", apiKeyID).Updates(map[string]interface{}{
			"usage_count":  gorm.Expr("usage_count + 1"),
			"last_used_at": now,
		}).Error
	})
}

// CreateLoginAttempt creates a login attempt record
func (r *AuthRepository) CreateLoginAttempt(ctx context.Context, attempt *domain.LoginAttempt) error {
	return r.executeWithDB(ctx, "CreateLoginAttempt", func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(attempt).Error
	})
}

// GetLoginAttempts retrieves login attempts
func (r *AuthRepository) GetLoginAttempts(ctx context.Context, email string, ipAddress string, since time.Time) ([]domain.LoginAttempt, error) {
	var attempts []domain.LoginAttempt
	err := r.executeWithDB(ctx, "GetLoginAttempts", func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("email = ? AND ip_address = ? AND created_at > ?", email, ipAddress, since).Find(&attempts).Error
	})
	if err != nil {
		return nil, err
	}
	return attempts, nil
}

// CreateAuditLog creates an audit log entry
func (r *AuthRepository) CreateAuditLog(ctx context.Context, auditLog *domain.AuditLog) error {
	return r.executeWithDB(ctx, "CreateAuditLog", func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(auditLog).Error
	})
}

// GetUserPermissions retrieves all permissions for a user
func (r *AuthRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Permission, error) {
	var permissions []domain.Permission
	err := r.executeWithDB(ctx, "GetUserPermissions", func(db *gorm.DB) error {
		return db.WithContext(ctx).
			Table("permissions").
			Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
			Joins("JOIN user_roles ON role_permissions.role_id = user_roles.role_id").
			Where("user_roles.user_id = ? AND permissions.is_active = ?", userID, true).
			Find(&permissions).Error
	})
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

// Additional repository methods

// CreateRole creates a new role
func (r *AuthRepository) CreateRole(ctx context.Context, role *domain.Role) error {
	return r.executeWithDB(ctx, "CreateRole", func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(role).Error
	})
}

// GetRoleByID retrieves a role by ID
func (r *AuthRepository) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	var role domain.Role
	err := r.executeWithDB(ctx, "GetRoleByID", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("Permissions").First(&role, "id = ?", id).Error
	})
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetRoleByName retrieves a role by name
func (r *AuthRepository) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	var role domain.Role
	err := r.executeWithDB(ctx, "GetRoleByName", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("Permissions").First(&role, "name = ?", name).Error
	})
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// UpdateRole updates a role
func (r *AuthRepository) UpdateRole(ctx context.Context, role *domain.Role) error {
	return r.executeWithDB(ctx, "UpdateRole", func(db *gorm.DB) error {
		return db.WithContext(ctx).Save(role).Error
	})
}

// DeleteRole deletes a role
func (r *AuthRepository) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return r.executeWithDB(ctx, "DeleteRole", func(db *gorm.DB) error {
		return db.WithContext(ctx).Delete(&domain.Role{}, "id = ?", id).Error
	})
}

// ListRoles lists roles with pagination
func (r *AuthRepository) ListRoles(ctx context.Context, limit, offset int) ([]domain.Role, error) {
	var roles []domain.Role
	err := r.executeWithDB(ctx, "ListRoles", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("Permissions").Limit(limit).Offset(offset).Find(&roles).Error
	})
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// CreatePermission creates a new permission
func (r *AuthRepository) CreatePermission(ctx context.Context, permission *domain.Permission) error {
	return r.executeWithDB(ctx, "CreatePermission", func(db *gorm.DB) error {
		return db.WithContext(ctx).Create(permission).Error
	})
}

// GetPermissionByID retrieves a permission by ID
func (r *AuthRepository) GetPermissionByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	var permission domain.Permission
	err := r.executeWithDB(ctx, "GetPermissionByID", func(db *gorm.DB) error {
		return db.WithContext(ctx).First(&permission, "id = ?", id).Error
	})
	if err != nil {
		return nil, err
	}
	return &permission, nil
}

// GetPermissionByName retrieves a permission by name
func (r *AuthRepository) GetPermissionByName(ctx context.Context, name string) (*domain.Permission, error) {
	var permission domain.Permission
	err := r.executeWithDB(ctx, "GetPermissionByName", func(db *gorm.DB) error {
		return db.WithContext(ctx).First(&permission, "name = ?", name).Error
	})
	if err != nil {
		return nil, err
	}
	return &permission, nil
}

// UpdatePermission updates a permission
func (r *AuthRepository) UpdatePermission(ctx context.Context, permission *domain.Permission) error {
	return r.executeWithDB(ctx, "UpdatePermission", func(db *gorm.DB) error {
		return db.WithContext(ctx).Save(permission).Error
	})
}

// DeletePermission deletes a permission
func (r *AuthRepository) DeletePermission(ctx context.Context, id uuid.UUID) error {
	return r.executeWithDB(ctx, "DeletePermission", func(db *gorm.DB) error {
		return db.WithContext(ctx).Delete(&domain.Permission{}, "id = ?", id).Error
	})
}

// ListPermissions lists permissions with pagination
func (r *AuthRepository) ListPermissions(ctx context.Context, limit, offset int) ([]domain.Permission, error) {
	var permissions []domain.Permission
	err := r.executeWithDB(ctx, "ListPermissions", func(db *gorm.DB) error {
		return db.WithContext(ctx).Limit(limit).Offset(offset).Find(&permissions).Error
	})
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

// AssignRoleToUser assigns a role to a user
func (r *AuthRepository) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return r.executeWithDB(ctx, "AssignRoleToUser", func(db *gorm.DB) error {
		return db.WithContext(ctx).Exec("INSERT INTO user_roles (user_id, role_id) VALUES (?, ?) ON CONFLICT DO NOTHING", userID, roleID).Error
	})
}

// RemoveRoleFromUser removes a role from a user
func (r *AuthRepository) RemoveRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return r.executeWithDB(ctx, "RemoveRoleFromUser", func(db *gorm.DB) error {
		return db.WithContext(ctx).Exec("DELETE FROM user_roles WHERE user_id = ? AND role_id = ?", userID, roleID).Error
	})
}

// GetUserRoles retrieves all roles for a user
func (r *AuthRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	var roles []domain.Role
	err := r.executeWithDB(ctx, "GetUserRoles", func(db *gorm.DB) error {
		return db.WithContext(ctx).
			Table("roles").
			Joins("JOIN user_roles ON roles.id = user_roles.role_id").
			Where("user_roles.user_id = ? AND roles.is_active = ?", userID, true).
			Find(&roles).Error
	})
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// GetRoleUsers retrieves all users for a role
func (r *AuthRepository) GetRoleUsers(ctx context.Context, roleID uuid.UUID) ([]domain.User, error) {
	var users []domain.User
	err := r.executeWithDB(ctx, "GetRoleUsers", func(db *gorm.DB) error {
		return db.WithContext(ctx).
			Table("users").
			Joins("JOIN user_roles ON users.id = user_roles.user_id").
			Where("user_roles.role_id = ? AND users.is_active = ?", roleID, true).
			Find(&users).Error
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

// AssignPermissionToRole assigns a permission to a role
func (r *AuthRepository) AssignPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return r.executeWithDB(ctx, "AssignPermissionToRole", func(db *gorm.DB) error {
		return db.WithContext(ctx).Exec("INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?) ON CONFLICT DO NOTHING", roleID, permissionID).Error
	})
}

// RemovePermissionFromRole removes a permission from a role
func (r *AuthRepository) RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return r.executeWithDB(ctx, "RemovePermissionFromRole", func(db *gorm.DB) error {
		return db.WithContext(ctx).Exec("DELETE FROM role_permissions WHERE role_id = ? AND permission_id = ?", roleID, permissionID).Error
	})
}

// GetRolePermissions retrieves all permissions for a role
func (r *AuthRepository) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	var permissions []domain.Permission
	err := r.executeWithDB(ctx, "GetRolePermissions", func(db *gorm.DB) error {
		return db.WithContext(ctx).
			Table("permissions").
			Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
			Where("role_permissions.role_id = ? AND permissions.is_active = ?", roleID, true).
			Find(&permissions).Error
	})
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

// GetSessionByID retrieves a session by ID
func (r *AuthRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	var session domain.Session
	err := r.executeWithDB(ctx, "GetSessionByID", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("User").First(&session, "id = ?", id).Error
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetApiKeyByID retrieves an API key by ID
func (r *AuthRepository) GetApiKeyByID(ctx context.Context, id uuid.UUID) (*domain.ApiKey, error) {
	var apiKey domain.ApiKey
	err := r.executeWithDB(ctx, "GetApiKeyByID", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("User").First(&apiKey, "id = ?", id).Error
	})
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

// UpdateApiKey updates an API key
func (r *AuthRepository) UpdateApiKey(ctx context.Context, apiKey *domain.ApiKey) error {
	return r.executeWithDB(ctx, "UpdateApiKey", func(db *gorm.DB) error {
		return db.WithContext(ctx).Save(apiKey).Error
	})
}

// DeleteApiKey deletes an API key
func (r *AuthRepository) DeleteApiKey(ctx context.Context, id uuid.UUID) error {
	return r.executeWithDB(ctx, "DeleteApiKey", func(db *gorm.DB) error {
		return db.WithContext(ctx).Delete(&domain.ApiKey{}, "id = ?", id).Error
	})
}

// ListApiKeys lists API keys for a user
func (r *AuthRepository) ListApiKeys(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.ApiKey, error) {
	var apiKeys []domain.ApiKey
	err := r.executeWithDB(ctx, "ListApiKeys", func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("user_id = ?", userID).Limit(limit).Offset(offset).Find(&apiKeys).Error
	})
	if err != nil {
		return nil, err
	}
	return apiKeys, nil
}

// GetAuditLogByID retrieves an audit log by ID
func (r *AuthRepository) GetAuditLogByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	var auditLog domain.AuditLog
	err := r.executeWithDB(ctx, "GetAuditLogByID", func(db *gorm.DB) error {
		return db.WithContext(ctx).Preload("User").First(&auditLog, "id = ?", id).Error
	})
	if err != nil {
		return nil, err
	}
	return &auditLog, nil
}

// ListAuditLogs lists audit logs with optional user filter
func (r *AuthRepository) ListAuditLogs(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	var auditLogs []domain.AuditLog
	err := r.executeWithDB(ctx, "ListAuditLogs", func(db *gorm.DB) error {
		query := db.WithContext(ctx).Preload("User")
		if userID != nil {
			query = query.Where("user_id = ?", *userID)
		}
		return query.Limit(limit).Offset(offset).Find(&auditLogs).Error
	})
	if err != nil {
		return nil, err
	}
	return auditLogs, nil
}

// CleanupOldLoginAttempts removes old login attempts
func (r *AuthRepository) CleanupOldLoginAttempts(ctx context.Context, before time.Time) error {
	return r.executeWithDB(ctx, "CleanupOldLoginAttempts", func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("created_at < ?", before).Delete(&domain.LoginAttempt{}).Error
	})
}

// executeWithDB executes a database operation with circuit breaker and retry support
func (r *AuthRepository) executeWithDB(ctx context.Context, operation string, fn func(*gorm.DB) error) error {
	if r.circuitBreaker != nil {
		_, err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			if r.retryManager != nil {
				_, err := r.retryManager.Execute(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
					return nil, fn(r.db)
				})
				return nil, err
			}
			return nil, fn(r.db)
		})
		return err
	}

	if r.retryManager != nil {
		_, err := r.retryManager.Execute(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
			return nil, fn(r.db)
		})
		return err
	}

	return fn(r.db)
}

// RBACService implements role-based access control
type RBACService struct {
	authRepo       domain.AuthRepository
	logger         *slog.Logger
	circuitBreaker *CircuitBreaker
	retryManager    *RetryManager
}

// NewRBACService creates a new RBAC service
func NewRBACService(authRepo domain.AuthRepository, logger *slog.Logger, cb *CircuitBreaker, retry *RetryManager) *RBACService {
	return &RBACService{
		authRepo:       authRepo,
		logger:         logger,
		circuitBreaker: cb,
		retryManager:    retry,
	}
}

// CheckPermission checks if a user has a specific permission
func (r *RBACService) CheckPermission(ctx context.Context, userID uuid.UUID, resource, action string) (bool, error) {
	permissions, err := r.authRepo.GetUserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, permission := range permissions {
		if permission.Resource == resource && permission.Action == action {
			return true, nil
		}
	}

	return false, nil
}

// ApiKeyService implements API key authentication
type ApiKeyService struct {
	authRepo       domain.AuthRepository
	logger         *slog.Logger
	circuitBreaker *CircuitBreaker
	retryManager    *RetryManager
}

// NewApiKeyService creates a new API key service
func NewApiKeyService(authRepo domain.AuthRepository, logger *slog.Logger, cb *CircuitBreaker, retry *RetryManager) *ApiKeyService {
	return &ApiKeyService{
		authRepo:       authRepo,
		logger:         logger,
		circuitBreaker: cb,
		retryManager:    retry,
	}
}

// GenerateApiKey generates a new API key
func (a *ApiKeyService) GenerateApiKey(ctx context.Context, userID uuid.UUID, name string, scopes []string, expiresAt *time.Time) (*domain.ApiKey, error) {
	// Generate random key
	keyBytes := make([]byte, ApiKeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, errors.Wrap(err, "failed to generate API key")
	}

	key := ApiKeyPrefix + base64.URLEncoding.EncodeToString(keyBytes)

	// Hash the key for storage
	keyHash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hash API key")
	}

	apiKey := &domain.ApiKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		Key:       key, // This will be returned once, then removed
		KeyHash:   string(keyHash),
		IsActive:  true,
		ExpiresAt: expiresAt,
		Scopes:    scopes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := a.authRepo.CreateApiKey(ctx, apiKey); err != nil {
		return nil, err
	}

	return apiKey, nil
}

// ValidateApiKey validates an API key
func (a *ApiKeyService) ValidateApiKey(ctx context.Context, key string) (*domain.ApiKey, error) {
	if !strings.HasPrefix(key, ApiKeyPrefix) {
		return nil, errors.New("invalid API key format")
	}

	// Get all API keys and check against hashes (in production, consider indexing strategies)
	apiKey, err := a.authRepo.GetApiKeyByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	// Check if key is expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("API key has expired")
	}

	// Validate the key hash
	if err := bcrypt.CompareHashAndPassword([]byte(apiKey.KeyHash), []byte(key)); err != nil {
		return nil, errors.New("invalid API key")
	}

	// Update usage
	if err := a.authRepo.UpdateApiKeyUsage(ctx, apiKey.ID); err != nil {
		a.logger.Error("failed to update API key usage", "error", err, "api_key_id", apiKey.ID)
	}

	return apiKey, nil
}

// RateLimitService implements rate limiting for authentication
type RateLimitService struct {
	authRepo       domain.AuthRepository
	logger         *slog.Logger
	circuitBreaker *CircuitBreaker
	retryManager    *RetryManager
}

// NewRateLimitService creates a new rate limiting service
func NewRateLimitService(authRepo domain.AuthRepository, logger *slog.Logger, cb *CircuitBreaker, retry *RetryManager) *RateLimitService {
	return &RateLimitService{
		authRepo:       authRepo,
		logger:         logger,
		circuitBreaker: cb,
		retryManager:    retry,
	}
}

// IsRateLimited checks if a user/IP is rate limited
func (r *RateLimitService) IsRateLimited(ctx context.Context, email, ipAddress string) (bool, error) {
	since := time.Now().Add(-LoginAttemptWindow)
	attempts, err := r.authRepo.GetLoginAttempts(ctx, email, ipAddress, since)
	if err != nil {
		return false, err
	}

	failedAttempts := 0
	for _, attempt := range attempts {
		if !attempt.Success {
			failedAttempts++
		}
	}

	return failedAttempts >= MaxLoginAttempts, nil
}

// RecordLoginAttempt records a login attempt
func (r *RateLimitService) RecordLoginAttempt(ctx context.Context, email, ipAddress, userAgent string, success bool, failureReason string) error {
	attempt := &domain.LoginAttempt{
		ID:            uuid.New(),
		Email:         email,
		IpAddress:     ipAddress,
		UserAgent:     userAgent,
		Success:       success,
		FailureReason: failureReason,
		CreatedAt:     time.Now(),
	}

	return r.authRepo.CreateLoginAttempt(ctx, attempt)
}

// AuditService implements audit logging
type AuditService struct {
	authRepo       domain.AuthRepository
	logger         *slog.Logger
	circuitBreaker *CircuitBreaker
	retryManager    *RetryManager
}

// NewAuditService creates a new audit service
func NewAuditService(authRepo domain.AuthRepository, logger *slog.Logger, cb *CircuitBreaker, retry *RetryManager) *AuditService {
	return &AuditService{
		authRepo:       authRepo,
		logger:         logger,
		circuitBreaker: cb,
		retryManager:    retry,
	}
}

// AuditAction records an audit log entry
func (a *AuditService) AuditAction(ctx context.Context, userID *uuid.UUID, action, resource string, resourceID *uuid.UUID, oldValues, newValues map[string]interface{}, ipAddress, userAgent string) error {
	auditLog := &domain.AuditLog{
		ID:         uuid.New(),
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		OldValues:  oldValues,
		NewValues:  newValues,
		IpAddress:  ipAddress,
		UserAgent:  userAgent,
		Result:     "success",
		CreatedAt:  time.Now(),
	}

	return a.authRepo.CreateAuditLog(ctx, auditLog)
}

// AuthenticationService orchestrates all authentication components
type AuthenticationService struct {
	authRepo         domain.AuthRepository
	jwtService       domain.JWTService
	passwordService  domain.PasswordService
	rbacService      *RBACService
	apiKeyService    *ApiKeyService
	rateLimitService *RateLimitService
	auditService     *AuditService
	logger           *slog.Logger
	circuitBreaker   *CircuitBreaker
	retryManager      *RetryManager
}

// NewAuthenticationService creates a comprehensive authentication service
func NewAuthenticationService(
	authRepo domain.AuthRepository,
	jwtService domain.JWTService,
	passwordService domain.PasswordService,
	rbacService *RBACService,
	apiKeyService *ApiKeyService,
	rateLimitService *RateLimitService,
	auditService *AuditService,
	logger *slog.Logger,
	cb *CircuitBreaker,
	retry *RetryManager,
) *AuthenticationService {
	return &AuthenticationService{
		authRepo:         authRepo,
		jwtService:       jwtService,
		passwordService:  passwordService,
		rbacService:      rbacService,
		apiKeyService:    apiKeyService,
		rateLimitService: rateLimitService,
		auditService:     auditService,
		logger:           logger,
		circuitBreaker:   cb,
		retryManager:      retry,
	}
}

// Login authenticates a user and returns a login response
func (a *AuthenticationService) Login(ctx context.Context, email, password, ipAddress, userAgent string) (*domain.LoginResponse, error) {
	// Check rate limiting
	isRateLimited, err := a.rateLimitService.IsRateLimited(ctx, email, ipAddress)
	if err != nil {
		return nil, err
	}

	if isRateLimited {
		return nil, errors.New("too many failed login attempts, please try again later")
	}

	// Get user by email
	user, err := a.authRepo.GetUserByEmail(ctx, email)
	if err != nil {
		// Record failed attempt
		_ = a.rateLimitService.RecordLoginAttempt(ctx, email, ipAddress, userAgent, false, "user_not_found")
		return nil, errors.New("invalid credentials")
	}

	// Check if user is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		_ = a.rateLimitService.RecordLoginAttempt(ctx, email, ipAddress, userAgent, false, "account_locked")
		return nil, errors.New("account is locked")
	}

	// Verify password
	if err := a.passwordService.ComparePassword(user.PasswordHash, password); err != nil {
		// Increment failed attempts
		_ = a.authRepo.IncrementFailedAttempts(ctx, user.ID)
		
		// Lock account if too many failed attempts
		if user.FailedAttempts+1 >= MaxLoginAttempts {
			lockUntil := time.Now().Add(AccountLockDuration)
			_ = a.authRepo.LockUser(ctx, user.ID, lockUntil)
		}

		_ = a.rateLimitService.RecordLoginAttempt(ctx, email, ipAddress, userAgent, false, "invalid_password")
		return nil, errors.New("invalid credentials")
	}

	// Reset failed attempts on successful login
	_ = a.authRepo.ResetFailedAttempts(ctx, user.ID)

	// Update last login
	_ = a.authRepo.UpdateUserLastLogin(ctx, user.ID)

	// Generate tokens
	accessToken, err := a.jwtService.GenerateAccessToken(ctx, user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := a.jwtService.GenerateRefreshToken(ctx, user)
	if err != nil {
		return nil, err
	}

	// Create session
	session := &domain.Session{
		ID:               uuid.New(),
		UserID:           user.ID,
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresAt:        time.Now().Add(AccessTokenExpiry),
		RefreshExpiresAt: time.Now().Add(RefreshTokenExpiry),
		IsActive:         true,
		UserAgent:        userAgent,
		IpAddress:        ipAddress,
		LastUsedAt:       time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := a.authRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	// Record successful login attempt
	_ = a.rateLimitService.RecordLoginAttempt(ctx, email, ipAddress, userAgent, true, "")

	// Audit successful login
	_ = a.auditService.AuditAction(ctx, &user.ID, "login", "user", &user.ID, nil, nil, ipAddress, userAgent)

	return &domain.LoginResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(AccessTokenExpiry.Seconds()),
	}, nil
}

// RefreshToken refreshes an access token
func (a *AuthenticationService) RefreshToken(ctx context.Context, refreshToken string) (*domain.LoginResponse, error) {
	// Validate refresh token
	claims, err := a.jwtService.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	// Get session
	session, err := a.authRepo.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Check if session is expired
	if session.RefreshExpiresAt.Before(time.Now()) {
		return nil, errors.New("refresh token expired")
	}

	// Get user
	user, err := a.authRepo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Generate new tokens
	newAccessToken, err := a.jwtService.GenerateAccessToken(ctx, user)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := a.jwtService.GenerateRefreshToken(ctx, user)
	if err != nil {
		return nil, err
	}

	// Update session
	session.AccessToken = newAccessToken
	session.RefreshToken = newRefreshToken
	session.ExpiresAt = time.Now().Add(AccessTokenExpiry)
	session.RefreshExpiresAt = time.Now().Add(RefreshTokenExpiry)
	session.LastUsedAt = time.Now()
	session.UpdatedAt = time.Now()

	if err := a.authRepo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		User:         user,
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(AccessTokenExpiry.Seconds()),
	}, nil
}

// Logout logs out a user
func (a *AuthenticationService) Logout(ctx context.Context, accessToken string) error {
	session, err := a.authRepo.GetSessionByAccessToken(ctx, accessToken)
	if err != nil {
		return err
	}

	return a.authRepo.DeleteSession(ctx, session.ID)
}

// ValidateToken validates a JWT token
func (a *AuthenticationService) ValidateToken(ctx context.Context, token string) (*domain.User, error) {
	claims, err := a.jwtService.ValidateAccessToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Get user to ensure it still exists and is active
	user, err := a.authRepo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, errors.New("user account is inactive")
	}

	return user, nil
}

// CheckPermission checks if a user has a specific permission
func (a *AuthenticationService) CheckPermission(ctx context.Context, userID uuid.UUID, resource, action string) (bool, error) {
	return a.rbacService.CheckPermission(ctx, userID, resource, action)
}

// CreateApiKey creates a new API key
func (a *AuthenticationService) CreateApiKey(ctx context.Context, userID uuid.UUID, name string, scopes []string, expiresAt *time.Time) (*domain.ApiKey, error) {
	return a.apiKeyService.GenerateApiKey(ctx, userID, name, scopes, expiresAt)
}

// ValidateApiKey validates an API key
func (a *AuthenticationService) ValidateApiKey(ctx context.Context, key string) (*domain.ApiKey, error) {
	return a.apiKeyService.ValidateApiKey(ctx, key)
}

// IsRateLimited checks if a user/IP is rate limited
func (a *AuthenticationService) IsRateLimited(ctx context.Context, email, ipAddress string) (bool, error) {
	return a.rateLimitService.IsRateLimited(ctx, email, ipAddress)
}

// RecordLoginAttempt records a login attempt
func (a *AuthenticationService) RecordLoginAttempt(ctx context.Context, email, ipAddress, userAgent string, success bool, failureReason string) error {
	return a.rateLimitService.RecordLoginAttempt(ctx, email, ipAddress, userAgent, success, failureReason)
}

// AuditAction records an audit log entry
func (a *AuthenticationService) AuditAction(ctx context.Context, userID *uuid.UUID, action, resource string, resourceID *uuid.UUID, oldValues, newValues map[string]interface{}, ipAddress, userAgent string) error {
	return a.auditService.AuditAction(ctx, userID, action, resource, resourceID, oldValues, newValues, ipAddress, userAgent)
}

// Additional AuthenticationService methods to implement the AuthService interface

// Register registers a new user
func (a *AuthenticationService) Register(ctx context.Context, user *domain.User, password string) error {
	// Hash the password
	hashedPassword, err := a.passwordService.HashPassword(password)
	if err != nil {
		return errors.Wrap(err, "failed to hash password")
	}
	
	user.PasswordHash = hashedPassword
	
	// Create user
	if err := a.authRepo.CreateUser(ctx, user); err != nil {
		return errors.Wrap(err, "failed to create user")
	}
	
	// Assign default role (if exists)
	defaultRole, err := a.authRepo.GetRoleByName(ctx, "user")
	if err == nil {
		_ = a.authRepo.AssignRoleToUser(ctx, user.ID, defaultRole.ID)
	}
	
	return nil
}

// CreateUser creates a new user
func (a *AuthenticationService) CreateUser(ctx context.Context, user *domain.User, password string) error {
	return a.Register(ctx, user, password)
}

// GetUser retrieves a user by ID
func (a *AuthenticationService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return a.authRepo.GetUserByID(ctx, id)
}

// GetUserByEmail retrieves a user by email
func (a *AuthenticationService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return a.authRepo.GetUserByEmail(ctx, email)
}

// UpdateUser updates a user
func (a *AuthenticationService) UpdateUser(ctx context.Context, user *domain.User) error {
	return a.authRepo.UpdateUser(ctx, user)
}

// DeleteUser deletes a user
func (a *AuthenticationService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return a.authRepo.DeleteUser(ctx, id)
}

// ListUsers lists users with pagination
func (a *AuthenticationService) ListUsers(ctx context.Context, limit, offset int) ([]domain.User, error) {
	return a.authRepo.ListUsers(ctx, limit, offset)
}

// ChangePassword changes a user's password
func (a *AuthenticationService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	// Get user
	user, err := a.authRepo.GetUserByID(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "user not found")
	}
	
	// Verify old password
	if err := a.passwordService.ComparePassword(user.PasswordHash, oldPassword); err != nil {
		return errors.New("invalid old password")
	}
	
	// Hash new password
	hashedPassword, err := a.passwordService.HashPassword(newPassword)
	if err != nil {
		return errors.Wrap(err, "failed to hash new password")
	}
	
	// Update password
	user.PasswordHash = hashedPassword
	user.UpdatedAt = time.Now()
	
	return a.authRepo.UpdateUser(ctx, user)
}

// ResetPassword resets a user's password
func (a *AuthenticationService) ResetPassword(ctx context.Context, email string) error {
	// This is a placeholder implementation
	// In a real implementation, you would:
	// 1. Generate a reset token
	// 2. Store it in the database
	// 3. Send an email with the reset link
	
	user, err := a.authRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return errors.Wrap(err, "user not found")
	}
	
	// Generate new password (placeholder)
	newPassword, err := a.passwordService.GenerateRandomPassword(12)
	if err != nil {
		return errors.Wrap(err, "failed to generate password")
	}
	
	// Hash password
	hashedPassword, err := a.passwordService.HashPassword(newPassword)
	if err != nil {
		return errors.Wrap(err, "failed to hash password")
	}
	
	// Update password
	user.PasswordHash = hashedPassword
	user.UpdatedAt = time.Now()
	
	return a.authRepo.UpdateUser(ctx, user)
}

// VerifyEmail verifies a user's email
func (a *AuthenticationService) VerifyEmail(ctx context.Context, token string) error {
	// This is a placeholder implementation
	// In a real implementation, you would:
	// 1. Validate the verification token
	// 2. Mark the user as verified
	return errors.New("not implemented")
}

// Role management methods

// CreateRole creates a new role
func (a *AuthenticationService) CreateRole(ctx context.Context, role *domain.Role) error {
	return a.authRepo.CreateRole(ctx, role)
}

// GetRole retrieves a role by ID
func (a *AuthenticationService) GetRole(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	return a.authRepo.GetRoleByID(ctx, id)
}

// UpdateRole updates a role
func (a *AuthenticationService) UpdateRole(ctx context.Context, role *domain.Role) error {
	return a.authRepo.UpdateRole(ctx, role)
}

// DeleteRole deletes a role
func (a *AuthenticationService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return a.authRepo.DeleteRole(ctx, id)
}

// ListRoles lists roles with pagination
func (a *AuthenticationService) ListRoles(ctx context.Context, limit, offset int) ([]domain.Role, error) {
	return a.authRepo.ListRoles(ctx, limit, offset)
}

// Permission management methods

// CreatePermission creates a new permission
func (a *AuthenticationService) CreatePermission(ctx context.Context, permission *domain.Permission) error {
	return a.authRepo.CreatePermission(ctx, permission)
}

// GetPermission retrieves a permission by ID
func (a *AuthenticationService) GetPermission(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	return a.authRepo.GetPermissionByID(ctx, id)
}

// UpdatePermission updates a permission
func (a *AuthenticationService) UpdatePermission(ctx context.Context, permission *domain.Permission) error {
	return a.authRepo.UpdatePermission(ctx, permission)
}

// DeletePermission deletes a permission
func (a *AuthenticationService) DeletePermission(ctx context.Context, id uuid.UUID) error {
	return a.authRepo.DeletePermission(ctx, id)
}

// ListPermissions lists permissions with pagination
func (a *AuthenticationService) ListPermissions(ctx context.Context, limit, offset int) ([]domain.Permission, error) {
	return a.authRepo.ListPermissions(ctx, limit, offset)
}

// RBAC methods

// AssignRole assigns a role to a user
func (a *AuthenticationService) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return a.authRepo.AssignRoleToUser(ctx, userID, roleID)
}

// RemoveRole removes a role from a user
func (a *AuthenticationService) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return a.authRepo.RemoveRoleFromUser(ctx, userID, roleID)
}

// AssignPermission assigns a permission to a role
func (a *AuthenticationService) AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return a.authRepo.AssignPermissionToRole(ctx, roleID, permissionID)
}

// RemovePermission removes a permission from a role
func (a *AuthenticationService) RemovePermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return a.authRepo.RemovePermissionFromRole(ctx, roleID, permissionID)
}

// GetUserPermissions retrieves all permissions for a user
func (a *AuthenticationService) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Permission, error) {
	return a.authRepo.GetUserPermissions(ctx, userID)
}

// DeleteApiKey deletes an API key
func (a *AuthenticationService) DeleteApiKey(ctx context.Context, id uuid.UUID) error {
	return a.authRepo.DeleteApiKey(ctx, id)
}

// ListApiKeys lists API keys for a user
func (a *AuthenticationService) ListApiKeys(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.ApiKey, error) {
	return a.authRepo.ListApiKeys(ctx, userID, limit, offset)
}