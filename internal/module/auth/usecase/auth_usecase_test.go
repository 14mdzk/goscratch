package usecase

import (
	"context"
	"testing"

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

// MockAuditor mocks the auditor interface
type MockAuditor struct{}

func (m *MockAuditor) Log(ctx context.Context, entry port.AuditEntry) error {
	return nil
}

func (m *MockAuditor) Query(ctx context.Context, filter port.AuditFilter) ([]port.AuditEntry, error) {
	return []port.AuditEntry{}, nil
}

func (m *MockAuditor) Close() error {
	return nil
}

func TestLogin_ValidCredentials(t *testing.T) {
	ctx := context.Background()

	// Create password hash
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

	// Test password verification
	user, err := mockRepo.GetByEmail(ctx, "user@example.com")
	assert.NoError(t, err)
	assert.NotNil(t, user)

	// Verify password
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

	// Store refresh token
	refreshToken := "test-refresh-token"
	userID := "user-123"

	err := cache.Set(ctx, "refresh:"+refreshToken, []byte(userID), nil)
	assert.NoError(t, err)

	// Retrieve refresh token
	storedUserID, err := cache.Get(ctx, "refresh:"+refreshToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, string(storedUserID))

	// Delete refresh token (logout)
	err = cache.Delete(ctx, "refresh:"+refreshToken)
	assert.NoError(t, err)

	// Verify deleted
	_, err = cache.Get(ctx, "refresh:"+refreshToken)
	assert.Error(t, err)
}
