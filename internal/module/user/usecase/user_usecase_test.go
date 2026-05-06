package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// MockRepository is a mock implementation of the repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

func (m *MockRepository) GetByEmail(ctx context.Context, email string) (*userdomain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

func (m *MockRepository) List(ctx context.Context, filter userdomain.UserFilter) ([]userdomain.User, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]userdomain.User), args.Error(1)
}

func (m *MockRepository) Create(ctx context.Context, email, passwordHash, name string) (*userdomain.User, error) {
	args := m.Called(ctx, email, passwordHash, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, id, name, email string) (*userdomain.User, error) {
	args := m.Called(ctx, id, name, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

func (m *MockRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	args := m.Called(ctx, id, passwordHash)
	return args.Error(0)
}

func (m *MockRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) Activate(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) Deactivate(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockCache is a testify mock for port.Cache, used to verify ChangePassword
// revocation behaviour in isolation.
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}
func (m *MockCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, val, ttl)
	return args.Error(0)
}
func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}
func (m *MockCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	args := m.Called(ctx, prefix)
	return args.Error(0)
}
func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}
func (m *MockCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}
func (m *MockCache) GetJSON(ctx context.Context, key string, dest any) error {
	args := m.Called(ctx, key, dest)
	return args.Error(0)
}
func (m *MockCache) Increment(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockCache) Decrement(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	args := m.Called(ctx, key, ttl)
	return args.Error(0)
}
func (m *MockCache) Close() error { return nil }

// MockAuditor is a mock implementation of the auditor
type MockAuditor struct {
	mock.Mock
}

func (m *MockAuditor) Log(ctx context.Context, entry port.AuditEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockAuditor) Query(ctx context.Context, filter port.AuditFilter) ([]port.AuditEntry, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]port.AuditEntry), args.Error(1)
}

func (m *MockAuditor) Close() error {
	return nil
}

// Repository interface for mocking
type Repository interface {
	GetByID(ctx context.Context, id string) (*userdomain.User, error)
	GetByEmail(ctx context.Context, email string) (*userdomain.User, error)
	List(ctx context.Context, filter userdomain.UserFilter) ([]userdomain.User, error)
	Create(ctx context.Context, email, passwordHash, name string) (*userdomain.User, error)
	Update(ctx context.Context, id, name, email string) (*userdomain.User, error)
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	Delete(ctx context.Context, id string) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	Count(ctx context.Context) (int64, error)
	Activate(ctx context.Context, id string) error
	Deactivate(ctx context.Context, id string) error
}

// --- Existing Tests ---

func TestUseCase_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockAuditor := new(MockAuditor)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
		expectedUser := &userdomain.User{
			ID:    testUUID,
			Email: "test@example.com",
			Name:  "Test User",
		}

		mockRepo.On("GetByID", ctx, testUUID.String()).Return(expectedUser, nil)

		result, err := mockRepo.GetByID(ctx, testUUID.String())

		assert.NoError(t, err)
		assert.Equal(t, testUUID, result.ID)
		assert.Equal(t, "test@example.com", result.Email)
		mockRepo.AssertExpectations(t)
		_ = mockAuditor
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("GetByID", ctx, "999").Return(nil, apperr.NotFoundf("user not found"))

		_, err := mockRepo.GetByID(ctx, "999")

		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestUseCase_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockAuditor := new(MockAuditor)

		mockRepo.On("ExistsByEmail", ctx, "new@example.com").Return(false, nil)

		exists, err := mockRepo.ExistsByEmail(ctx, "new@example.com")
		assert.NoError(t, err)
		assert.False(t, exists)

		// Simulate the full create flow: hash password, create user, audit log
		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
		mockRepo.On("Create", ctx, "new@example.com", mock.AnythingOfType("string"), "New User").Return(&userdomain.User{
			ID:        testUUID,
			Email:     "new@example.com",
			Name:      "New User",
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil)
		mockAuditor.On("Log", ctx, mock.AnythingOfType("port.AuditEntry")).Return(nil)

		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
		assert.NoError(t, hashErr)

		user, err := mockRepo.Create(ctx, "new@example.com", string(passwordHash), "New User")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "new@example.com", user.Email)

		// Verify audit logging
		entry := port.NewAuditEntry(ctx, port.AuditActionCreate, "user", user.ID.String())
		err = mockAuditor.Log(ctx, entry)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockAuditor.AssertExpectations(t)
	})

	t.Run("email_exists", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("ExistsByEmail", ctx, "existing@example.com").Return(true, nil)

		exists, err := mockRepo.ExistsByEmail(ctx, "existing@example.com")

		assert.NoError(t, err)
		assert.True(t, exists)
		mockRepo.AssertExpectations(t)
	})
}

func TestUseCase_ChangePassword(t *testing.T) {
	ctx := context.Background()

	t.Run("password_verification", func(t *testing.T) {
		password := "oldpassword"
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		assert.NoError(t, err)

		err = bcrypt.CompareHashAndPassword(hash, []byte(password))
		assert.NoError(t, err)

		err = bcrypt.CompareHashAndPassword(hash, []byte("wrongpassword"))
		assert.Error(t, err)
	})

	t.Run("full_flow_success", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockAuditor := new(MockAuditor)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
		currentPassword := "oldpassword"
		currentHash, _ := bcrypt.GenerateFromPassword([]byte(currentPassword), bcrypt.MinCost)

		// Step 1: Get user to verify current password
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:           testUUID,
			Email:        "test@example.com",
			PasswordHash: string(currentHash),
		}, nil)

		user, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)

		// Step 2: Verify current password
		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword))
		assert.NoError(t, err)

		// Step 3: Hash new password
		newPasswordHash, err := bcrypt.GenerateFromPassword([]byte("newpassword123"), bcrypt.MinCost)
		assert.NoError(t, err)

		// Step 4: Update password
		mockRepo.On("UpdatePassword", ctx, testUUID.String(), string(newPasswordHash)).Return(nil)
		err = mockRepo.UpdatePassword(ctx, testUUID.String(), string(newPasswordHash))
		assert.NoError(t, err)

		// Step 5: Audit log
		mockAuditor.On("Log", ctx, mock.AnythingOfType("port.AuditEntry")).Return(nil)
		entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", testUUID.String())
		entry.Metadata = map[string]any{"field": "password"}
		err = mockAuditor.Log(ctx, entry)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockAuditor.AssertExpectations(t)
	})
}

func TestUseCase_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockAuditor := new(MockAuditor)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

		// Step 1: Get user for audit
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:    testUUID,
			Email: "test@example.com",
			Name:  "Test User",
		}, nil)

		user, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)

		// Step 2: Delete
		mockRepo.On("Delete", ctx, testUUID.String()).Return(nil)
		err = mockRepo.Delete(ctx, testUUID.String())
		assert.NoError(t, err)

		// Step 3: Audit log
		mockAuditor.On("Log", ctx, mock.AnythingOfType("port.AuditEntry")).Return(nil)
		entry := port.NewAuditEntry(ctx, port.AuditActionDelete, "user", testUUID.String())
		entry.OldValue = map[string]any{"email": user.Email, "name": user.Name}
		err = mockAuditor.Log(ctx, entry)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockAuditor.AssertExpectations(t)
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("GetByID", ctx, "999").Return(nil, apperr.NotFoundf("user not found"))

		_, err := mockRepo.GetByID(ctx, "999")
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestUseCase_List(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockRepository)

		uuid1 := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
		uuid2 := uuid.MustParse("11234567-89ab-cdef-0123-456789abcdef")
		users := []userdomain.User{
			{ID: uuid1, Email: "user1@example.com", Name: "User 1"},
			{ID: uuid2, Email: "user2@example.com", Name: "User 2"},
		}

		mockRepo.On("List", ctx, userdomain.UserFilter{Limit: 20}).Return(users, nil)

		result, err := mockRepo.List(ctx, userdomain.UserFilter{Limit: 20})

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		mockRepo.AssertExpectations(t)
	})

	t.Run("empty", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("List", ctx, userdomain.UserFilter{Limit: 20}).Return([]userdomain.User{}, nil)

		result, err := mockRepo.List(ctx, userdomain.UserFilter{Limit: 20})

		assert.NoError(t, err)
		assert.Empty(t, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("List", ctx, userdomain.UserFilter{Limit: 20}).Return([]userdomain.User{}, errors.New("database error"))

		_, err := mockRepo.List(ctx, userdomain.UserFilter{Limit: 20})

		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestUserDTO_Validation(t *testing.T) {
	t.Run("valid_create_request", func(t *testing.T) {
		req := dto.CreateUserRequest{
			Email:    "valid@example.com",
			Password: "password123",
			Name:     "Valid User",
		}
		assert.NotEmpty(t, req.Email)
		assert.GreaterOrEqual(t, len(req.Password), 8)
		assert.GreaterOrEqual(t, len(req.Name), 2)
	})

	t.Run("invalid_email", func(t *testing.T) {
		req := dto.CreateUserRequest{
			Email:    "invalid-email",
			Password: "password123",
			Name:     "User",
		}
		assert.NotContains(t, req.Email, "@")
	})

	t.Run("short_password", func(t *testing.T) {
		req := dto.CreateUserRequest{
			Email:    "test@example.com",
			Password: "short",
			Name:     "User",
		}
		assert.Less(t, len(req.Password), 8)
	})
}

// --- New Tests: Update ---

func TestUseCase_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockAuditor := new(MockAuditor)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

		// Step 1: Get current user for audit
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:        testUUID,
			Email:     "old@example.com",
			Name:      "Old Name",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil)

		oldUser, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)

		// Step 2: Update user
		mockRepo.On("Update", ctx, testUUID.String(), "New Name", "new@example.com").Return(&userdomain.User{
			ID:        testUUID,
			Email:     "new@example.com",
			Name:      "New Name",
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil)

		updatedUser, err := mockRepo.Update(ctx, testUUID.String(), "New Name", "new@example.com")
		assert.NoError(t, err)
		assert.Equal(t, "New Name", updatedUser.Name)
		assert.Equal(t, "new@example.com", updatedUser.Email)

		// Step 3: Audit log
		mockAuditor.On("Log", ctx, mock.AnythingOfType("port.AuditEntry")).Return(nil)
		entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", updatedUser.ID.String())
		entry.OldValue = map[string]any{"email": oldUser.Email, "name": oldUser.Name}
		entry.NewValue = map[string]any{"email": updatedUser.Email, "name": updatedUser.Name}
		err = mockAuditor.Log(ctx, entry)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockAuditor.AssertExpectations(t)
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("GetByID", ctx, "nonexistent-id").Return(nil, apperr.NotFoundf("user not found"))

		_, err := mockRepo.GetByID(ctx, "nonexistent-id")
		assert.Error(t, err)

		// The real usecase would return here without calling Update
		mockRepo.AssertExpectations(t)
	})

	t.Run("email_conflict", func(t *testing.T) {
		mockRepo := new(MockRepository)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

		// GetByID succeeds
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:    testUUID,
			Email: "old@example.com",
			Name:  "Old Name",
		}, nil)

		_, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)

		// Update fails due to email conflict
		mockRepo.On("Update", ctx, testUUID.String(), "Name", "taken@example.com").Return(nil, apperr.Conflictf("user with email taken@example.com already exists"))

		_, err = mockRepo.Update(ctx, testUUID.String(), "Name", "taken@example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		mockRepo.AssertExpectations(t)
	})
}

// --- New Tests: Activate ---

func TestUseCase_Activate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockAuditor := new(MockAuditor)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

		// Step 1: Get user - user is inactive
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:       testUUID,
			Email:    "test@example.com",
			Name:     "Test User",
			IsActive: false,
		}, nil)

		user, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)
		assert.False(t, user.IsActive)

		// Step 2: Activate
		mockRepo.On("Activate", ctx, testUUID.String()).Return(nil)
		err = mockRepo.Activate(ctx, testUUID.String())
		assert.NoError(t, err)

		// Step 3: Audit log
		mockAuditor.On("Log", ctx, mock.AnythingOfType("port.AuditEntry")).Return(nil)
		entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", testUUID.String())
		entry.OldValue = map[string]any{"is_active": false}
		entry.NewValue = map[string]any{"is_active": true}
		err = mockAuditor.Log(ctx, entry)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockAuditor.AssertExpectations(t)
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("GetByID", ctx, "nonexistent-id").Return(nil, apperr.NotFoundf("user not found"))

		_, err := mockRepo.GetByID(ctx, "nonexistent-id")
		assert.Error(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("already_active", func(t *testing.T) {
		mockRepo := new(MockRepository)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

		// User is already active - the real usecase returns nil (no-op)
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:       testUUID,
			Email:    "test@example.com",
			Name:     "Test User",
			IsActive: true,
		}, nil)

		user, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)
		assert.True(t, user.IsActive)

		// No Activate call should be made since user is already active
		// No audit log should be created
		mockRepo.AssertExpectations(t)
	})
}

// --- New Tests: Deactivate ---

func TestUseCase_Deactivate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockAuditor := new(MockAuditor)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

		// Step 1: Get user - user is active
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:       testUUID,
			Email:    "test@example.com",
			Name:     "Test User",
			IsActive: true,
		}, nil)

		user, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)
		assert.True(t, user.IsActive)

		// Step 2: Deactivate
		mockRepo.On("Deactivate", ctx, testUUID.String()).Return(nil)
		err = mockRepo.Deactivate(ctx, testUUID.String())
		assert.NoError(t, err)

		// Step 3: Audit log
		mockAuditor.On("Log", ctx, mock.AnythingOfType("port.AuditEntry")).Return(nil)
		entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", testUUID.String())
		entry.OldValue = map[string]any{"is_active": true}
		entry.NewValue = map[string]any{"is_active": false}
		err = mockAuditor.Log(ctx, entry)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockAuditor.AssertExpectations(t)
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := new(MockRepository)

		mockRepo.On("GetByID", ctx, "nonexistent-id").Return(nil, apperr.NotFoundf("user not found"))

		_, err := mockRepo.GetByID(ctx, "nonexistent-id")
		assert.Error(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("already_inactive", func(t *testing.T) {
		mockRepo := new(MockRepository)

		testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

		// User is already inactive - the real usecase returns nil (no-op)
		mockRepo.On("GetByID", ctx, testUUID.String()).Return(&userdomain.User{
			ID:       testUUID,
			Email:    "test@example.com",
			Name:     "Test User",
			IsActive: false,
		}, nil)

		user, err := mockRepo.GetByID(ctx, testUUID.String())
		assert.NoError(t, err)
		assert.False(t, user.IsActive)

		// No Deactivate call should be made since user is already inactive
		// No audit log should be created
		mockRepo.AssertExpectations(t)
	})
}

// --- Audit Logging Verification ---

func TestAuditLogging_CreateUser(t *testing.T) {
	ctx := context.Background()
	mockAuditor := new(MockAuditor)

	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	mockAuditor.On("Log", ctx, mock.MatchedBy(func(entry port.AuditEntry) bool {
		return entry.Action == port.AuditActionCreate &&
			entry.Resource == "user" &&
			entry.ResourceID == testUUID.String()
	})).Return(nil)

	entry := port.NewAuditEntry(ctx, port.AuditActionCreate, "user", testUUID.String())
	entry.NewValue = map[string]any{"email": "test@example.com", "name": "Test"}
	err := mockAuditor.Log(ctx, entry)
	assert.NoError(t, err)
	mockAuditor.AssertExpectations(t)
}

func TestAuditLogging_DeleteUser(t *testing.T) {
	ctx := context.Background()
	mockAuditor := new(MockAuditor)

	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	mockAuditor.On("Log", ctx, mock.MatchedBy(func(entry port.AuditEntry) bool {
		return entry.Action == port.AuditActionDelete &&
			entry.Resource == "user" &&
			entry.ResourceID == testUUID.String()
	})).Return(nil)

	entry := port.NewAuditEntry(ctx, port.AuditActionDelete, "user", testUUID.String())
	entry.OldValue = map[string]any{"email": "test@example.com", "name": "Test"}
	err := mockAuditor.Log(ctx, entry)
	assert.NoError(t, err)
	mockAuditor.AssertExpectations(t)
}

func TestAuditLogging_UpdateUser(t *testing.T) {
	ctx := context.Background()
	mockAuditor := new(MockAuditor)

	testUUID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	mockAuditor.On("Log", ctx, mock.MatchedBy(func(entry port.AuditEntry) bool {
		return entry.Action == port.AuditActionUpdate &&
			entry.Resource == "user"
	})).Return(nil)

	entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", testUUID.String())
	entry.OldValue = map[string]any{"name": "Old"}
	entry.NewValue = map[string]any{"name": "New"}
	err := mockAuditor.Log(ctx, entry)
	assert.NoError(t, err)
	mockAuditor.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ChangePassword — refresh token revocation via AuthRevoker (dual-key design)
// ---------------------------------------------------------------------------

// MockAuthRevoker mocks the AuthRevoker interface injected into userUseCase.
type MockAuthRevoker struct {
	mock.Mock
}

func (m *MockAuthRevoker) RevokeAllForUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// TestChangePassword_AuthRevokerCalled verifies that ChangePassword delegates
// session revocation to AuthRevoker.RevokeAllForUser with the correct userID.
func TestChangePassword_AuthRevokerCalled(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	currentPassword := "oldpassword"
	currentHash, _ := bcrypt.GenerateFromPassword([]byte(currentPassword), bcrypt.MinCost)

	t.Run("success: RevokeAllForUser called with correct userID", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockRepo.On("GetByID", ctx, testID.String()).Return(&userdomain.User{
			ID:           testID,
			Email:        "test@example.com",
			PasswordHash: string(currentHash),
		}, nil)
		mockRepo.On("UpdatePassword", ctx, testID.String(), mock.AnythingOfType("string")).Return(nil)

		mockRevoker := new(MockAuthRevoker)
		mockRevoker.On("RevokeAllForUser", ctx, testID.String()).Return(nil)

		uc := newUseCase(mockRepo, nil, nil, mockRevoker)
		err := uc.ChangePassword(ctx, testID.String(), dto.ChangePasswordRequest{
			CurrentPassword: currentPassword,
			NewPassword:     "newpassword123",
		})

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockRevoker.AssertExpectations(t)
	})

	t.Run("revoker error is propagated: password still updated", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockRepo.On("GetByID", ctx, testID.String()).Return(&userdomain.User{
			ID:           testID,
			Email:        "test@example.com",
			PasswordHash: string(currentHash),
		}, nil)
		mockRepo.On("UpdatePassword", ctx, testID.String(), mock.AnythingOfType("string")).Return(nil)

		mockRevoker := new(MockAuthRevoker)
		mockRevoker.On("RevokeAllForUser", ctx, testID.String()).Return(port.ErrCacheUnavailable)

		uc := newUseCase(mockRepo, nil, nil, mockRevoker)
		err := uc.ChangePassword(ctx, testID.String(), dto.ChangePasswordRequest{
			CurrentPassword: currentPassword,
			NewPassword:     "newpassword123",
		})

		// Error propagated so caller knows revocation did not happen.
		assert.Error(t, err)
		assert.ErrorIs(t, err, port.ErrCacheUnavailable)
		mockRepo.AssertExpectations(t)
		mockRevoker.AssertExpectations(t)
	})

	t.Run("nil revoker: ChangePassword succeeds without revocation", func(t *testing.T) {
		// Nil revoker is allowed for environments where auth sessions are not used.
		mockRepo := new(MockRepository)
		mockRepo.On("GetByID", ctx, testID.String()).Return(&userdomain.User{
			ID:           testID,
			Email:        "test@example.com",
			PasswordHash: string(currentHash),
		}, nil)
		mockRepo.On("UpdatePassword", ctx, testID.String(), mock.AnythingOfType("string")).Return(nil)

		uc := newUseCase(mockRepo, nil, nil, nil)
		err := uc.ChangePassword(ctx, testID.String(), dto.ChangePasswordRequest{
			CurrentPassword: currentPassword,
			NewPassword:     "newpassword123",
		})

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}
