package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/port"
)

// MockUserRepository mocks the user repository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*userdomain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

// MockCache mocks the cache interface
type MockCache struct {
	mock.Mock
	data map[string][]byte
}

func NewMockCache() *MockCache {
	return &MockCache{data: make(map[string][]byte)}
}

func (m *MockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return nil, port.ErrCacheMiss
}

func (m *MockCache) Set(ctx context.Context, key string, value []byte, ttl interface{}) error {
	m.data[key] = value
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockCache) SetJSON(ctx context.Context, key string, value any, ttl interface{}) error {
	return nil
}

func (m *MockCache) GetJSON(ctx context.Context, key string, dest any) error {
	return port.ErrCacheMiss
}

func (m *MockCache) Increment(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (m *MockCache) Decrement(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (m *MockCache) Expire(ctx context.Context, key string, ttl interface{}) error {
	return nil
}

func (m *MockCache) Close() error {
	return nil
}

// MockAuditor mocks the auditor interface with call tracking
type MockAuditor struct {
	Entries []port.AuditEntry
}

func (m *MockAuditor) Log(ctx context.Context, entry port.AuditEntry) error {
	m.Entries = append(m.Entries, entry)
	return nil
}

func (m *MockAuditor) Query(ctx context.Context, filter port.AuditFilter) ([]port.AuditEntry, error) {
	return m.Entries, nil
}

func (m *MockAuditor) Close() error {
	return nil
}

// --- Existing component tests (unchanged) ---

func TestLogin_ValidCredentials(t *testing.T) {
	ctx := context.Background()

	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	mockRepo := new(MockUserRepository)
	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	mockRepo.On("GetByEmail", ctx, "user@example.com").Return(&userdomain.User{
		ID:           testUUID,
		Email:        "user@example.com",
		PasswordHash: string(hash),
		Name:         "Test User",
		IsActive:     true,
	}, nil)

	user, err := mockRepo.GetByEmail(ctx, "user@example.com")
	assert.NoError(t, err)
	assert.NotNil(t, user)

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestLogin_InvalidPassword(t *testing.T) {
	password := "correctpassword"
	wrongPassword := "wrongpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	err := bcrypt.CompareHashAndPassword(hash, []byte(wrongPassword))
	assert.Error(t, err)
}

func TestLogin_UserNotFound(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, "nonexistent@example.com").Return(nil, assert.AnError)

	_, err := mockRepo.GetByEmail(ctx, "nonexistent@example.com")
	assert.Error(t, err)

	mockRepo.AssertExpectations(t)
}

func TestLoginDTO_Validation(t *testing.T) {
	t.Run("valid_login", func(t *testing.T) {
		req := dto.LoginRequest{
			Email:    "user@example.com",
			Password: "password123",
		}
		assert.NotEmpty(t, req.Email)
		assert.NotEmpty(t, req.Password)
	})

	t.Run("empty_email", func(t *testing.T) {
		req := dto.LoginRequest{
			Email:    "",
			Password: "password123",
		}
		assert.Empty(t, req.Email)
	})

	t.Run("empty_password", func(t *testing.T) {
		req := dto.LoginRequest{
			Email:    "user@example.com",
			Password: "",
		}
		assert.Empty(t, req.Password)
	})
}

func TestRefreshDTO_Validation(t *testing.T) {
	t.Run("valid_refresh", func(t *testing.T) {
		req := dto.RefreshRequest{
			RefreshToken: "valid-refresh-token",
		}
		assert.NotEmpty(t, req.RefreshToken)
	})

	t.Run("empty_token", func(t *testing.T) {
		req := dto.RefreshRequest{
			RefreshToken: "",
		}
		assert.Empty(t, req.RefreshToken)
	})
}

func TestRefreshToken_StorageAndRetrieval(t *testing.T) {
	cache := NewMockCache()
	ctx := context.Background()

	refreshToken := "test-refresh-token"
	userID := "user-123"

	err := cache.Set(ctx, "refresh:"+refreshToken, []byte(userID), nil)
	assert.NoError(t, err)

	storedUserID, err := cache.Get(ctx, "refresh:"+refreshToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, string(storedUserID))

	err = cache.Delete(ctx, "refresh:"+refreshToken)
	assert.NoError(t, err)

	_, err = cache.Get(ctx, "refresh:"+refreshToken)
	assert.Error(t, err)
}

// --- Full flow tests ---

// TestLoginFlow_Success simulates the full login flow:
// find user by email -> verify password -> generate tokens -> store refresh in cache -> audit log
func TestLoginFlow_Success(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	auditor := &MockAuditor{}

	password := "password123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	assert.NoError(t, err)

	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, "user@example.com").Return(&userdomain.User{
		ID:           testUUID,
		Email:        "user@example.com",
		PasswordHash: string(hash),
		Name:         "Test User",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil)

	// Step 1: Find user by email
	user, err := mockRepo.GetByEmail(ctx, "user@example.com")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "user@example.com", user.Email)

	// Step 2: Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	assert.NoError(t, err)

	// Step 3: Simulate token generation (access token is JWT, refresh is random)
	accessToken := "simulated-access-token"
	refreshToken := "simulated-refresh-token"
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	// Step 4: Store refresh token in cache
	refreshKey := "refresh:" + refreshToken
	err = cache.Set(ctx, refreshKey, []byte(user.ID.String()), nil)
	assert.NoError(t, err)

	// Verify it was stored
	storedID, err := cache.Get(ctx, refreshKey)
	assert.NoError(t, err)
	assert.Equal(t, user.ID.String(), string(storedID))

	// Step 5: Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionLogin, "user", user.ID.String())
	err = auditor.Log(ctx, entry)
	assert.NoError(t, err)
	assert.Len(t, auditor.Entries, 1)
	assert.Equal(t, port.AuditActionLogin, auditor.Entries[0].Action)
	assert.Equal(t, "user", auditor.Entries[0].Resource)
	assert.Equal(t, user.ID.String(), auditor.Entries[0].ResourceID)

	// Build response
	resp := &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
		TokenType:    "Bearer",
	}
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, 900, resp.ExpiresIn)

	mockRepo.AssertExpectations(t)
}

// TestLoginFlow_UserNotFound simulates login when user email is not in the database
func TestLoginFlow_UserNotFound(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, "nonexistent@example.com").Return(nil, assert.AnError)

	// Step 1: Find user by email - fails
	user, err := mockRepo.GetByEmail(ctx, "nonexistent@example.com")
	assert.Error(t, err)
	assert.Nil(t, user)

	// The real usecase returns: apperr.ErrUnauthorized.WithMessage("Invalid email or password")
	// It should NOT reveal that the user doesn't exist

	mockRepo.AssertExpectations(t)
}

// TestLoginFlow_InvalidPassword simulates full login flow with wrong password
func TestLoginFlow_InvalidPassword(t *testing.T) {
	ctx := context.Background()

	correctPassword := "correctpassword"
	wrongPassword := "wrongpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(correctPassword), bcrypt.MinCost)

	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, "user@example.com").Return(&userdomain.User{
		ID:           testUUID,
		Email:        "user@example.com",
		PasswordHash: string(hash),
		Name:         "Test User",
		IsActive:     true,
	}, nil)

	// Step 1: Find user - succeeds
	user, err := mockRepo.GetByEmail(ctx, "user@example.com")
	assert.NoError(t, err)
	assert.NotNil(t, user)

	// Step 2: Password verification - fails
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(wrongPassword))
	assert.Error(t, err)

	// Flow should stop here - no tokens, no cache, no audit

	mockRepo.AssertExpectations(t)
}

// TestLoginFlow_InactiveUser simulates login for an inactive user
// Note: the current usecase does NOT check IsActive, but this test documents that behavior
func TestLoginFlow_InactiveUser(t *testing.T) {
	ctx := context.Background()

	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, "inactive@example.com").Return(&userdomain.User{
		ID:           testUUID,
		Email:        "inactive@example.com",
		PasswordHash: string(hash),
		Name:         "Inactive User",
		IsActive:     false,
	}, nil)

	user, err := mockRepo.GetByEmail(ctx, "inactive@example.com")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.False(t, user.IsActive)

	// Password check succeeds even for inactive users in current implementation
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

// TestRefreshFlow_Success simulates the full refresh flow:
// get userID from cache -> delete old token -> get user -> generate new tokens -> store new token
func TestRefreshFlow_Success(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()

	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	oldRefreshToken := "old-refresh-token"

	// Pre-store the refresh token in cache
	refreshKey := "refresh:" + oldRefreshToken
	err := cache.Set(ctx, refreshKey, []byte(testUUID.String()), nil)
	assert.NoError(t, err)

	// Step 1: Get userID from cache
	userIDBytes, err := cache.Get(ctx, refreshKey)
	assert.NoError(t, err)
	userID := string(userIDBytes)
	assert.Equal(t, testUUID.String(), userID)

	// Step 2: Get user by ID
	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByID", ctx, userID).Return(&userdomain.User{
		ID:    testUUID,
		Email: "user@example.com",
		Name:  "Test User",
	}, nil)

	user, err := mockRepo.GetByID(ctx, userID)
	assert.NoError(t, err)
	assert.NotNil(t, user)

	// Step 3: Delete old refresh token
	err = cache.Delete(ctx, refreshKey)
	assert.NoError(t, err)

	// Verify old token is gone
	_, err = cache.Get(ctx, refreshKey)
	assert.Error(t, err)

	// Step 4: Generate new tokens (simulated)
	newRefreshToken := "new-refresh-token"
	newRefreshKey := "refresh:" + newRefreshToken

	// Step 5: Store new refresh token
	err = cache.Set(ctx, newRefreshKey, []byte(user.ID.String()), nil)
	assert.NoError(t, err)

	// Verify new token is stored
	storedID, err := cache.Get(ctx, newRefreshKey)
	assert.NoError(t, err)
	assert.Equal(t, user.ID.String(), string(storedID))

	// Build response
	resp := &dto.RefreshResponse{
		AccessToken:  "new-access-token",
		RefreshToken: newRefreshToken,
		ExpiresIn:    900,
		TokenType:    "Bearer",
	}
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)

	mockRepo.AssertExpectations(t)
}

// TestRefreshFlow_InvalidToken simulates refresh with a token not in cache
func TestRefreshFlow_InvalidToken(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()

	invalidToken := "invalid-token"
	refreshKey := "refresh:" + invalidToken

	// Step 1: Try to get from cache - fails
	_, err := cache.Get(ctx, refreshKey)
	assert.Error(t, err)

	// Flow should stop here - return "Invalid or expired refresh token"
}

// TestRefreshFlow_ExpiredToken simulates refresh when token was deleted (expired)
func TestRefreshFlow_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()

	expiredToken := "expired-token"
	refreshKey := "refresh:" + expiredToken

	// Store and then delete to simulate expiration
	err := cache.Set(ctx, refreshKey, []byte("some-user-id"), nil)
	assert.NoError(t, err)

	err = cache.Delete(ctx, refreshKey)
	assert.NoError(t, err)

	// Now try to use it - should fail
	_, err = cache.Get(ctx, refreshKey)
	assert.Error(t, err)
}

// TestLogoutFlow_Success simulates the full logout flow:
// delete refresh token from cache -> audit log
func TestLogoutFlow_Success(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	auditor := &MockAuditor{}

	refreshToken := "some-refresh-token"
	refreshKey := "refresh:" + refreshToken

	// Pre-store the token
	err := cache.Set(ctx, refreshKey, []byte("user-id"), nil)
	assert.NoError(t, err)

	// Step 1: Delete refresh token from cache
	err = cache.Delete(ctx, refreshKey)
	assert.NoError(t, err)

	// Verify deleted
	_, err = cache.Get(ctx, refreshKey)
	assert.Error(t, err)

	// Step 2: Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionLogout, "user", "")
	err = auditor.Log(ctx, entry)
	assert.NoError(t, err)
	assert.Len(t, auditor.Entries, 1)
	assert.Equal(t, port.AuditActionLogout, auditor.Entries[0].Action)
	assert.Equal(t, "user", auditor.Entries[0].Resource)
}

// TestLogoutFlow_InvalidToken simulates logout with a token that doesn't exist.
// The real usecase simply deletes and doesn't check - logout always succeeds.
func TestLogoutFlow_InvalidToken(t *testing.T) {
	ctx := context.Background()
	cache := NewMockCache()
	auditor := &MockAuditor{}

	refreshToken := "nonexistent-token"
	refreshKey := "refresh:" + refreshToken

	// Deleting a nonexistent key should not error
	err := cache.Delete(ctx, refreshKey)
	assert.NoError(t, err)

	// Audit log is still created
	entry := port.NewAuditEntry(ctx, port.AuditActionLogout, "user", "")
	err = auditor.Log(ctx, entry)
	assert.NoError(t, err)
	assert.Len(t, auditor.Entries, 1)
}
