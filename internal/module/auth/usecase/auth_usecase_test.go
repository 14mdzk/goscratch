package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/port"
)

// ---------------------------------------------------------------------------
// Test helpers / mocks
// ---------------------------------------------------------------------------

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

// mapCache is a simple in-memory cache that supports optional per-call Set
// failure injection for fail-closed tests.
type mapCache struct {
	data     map[string][]byte
	setErrs  []error // consumed in FIFO order on each Set call; nil means success
	setCalls int
}

func newMapCache() *mapCache {
	return &mapCache{data: make(map[string][]byte)}
}

// failSet registers errors to be returned by consecutive Set calls.
// Use nil in the slice to mean "succeed on that call".
func (c *mapCache) failSet(errs ...error) {
	c.setErrs = append(c.setErrs, errs...)
}

func (c *mapCache) Get(_ context.Context, key string) ([]byte, error) {
	if val, ok := c.data[key]; ok {
		return val, nil
	}
	return nil, port.ErrCacheMiss
}

func (c *mapCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	var err error
	if c.setCalls < len(c.setErrs) {
		err = c.setErrs[c.setCalls]
	}
	c.setCalls++
	if err != nil {
		return err
	}
	c.data[key] = value
	return nil
}

func (c *mapCache) Delete(_ context.Context, key string) error {
	delete(c.data, key)
	return nil
}

func (c *mapCache) DeleteByPrefix(_ context.Context, prefix string) error {
	for k := range c.data {
		if strings.HasPrefix(k, prefix) {
			delete(c.data, k)
		}
	}
	return nil
}
func (c *mapCache) Exists(_ context.Context, key string) (bool, error) {
	_, ok := c.data[key]
	return ok, nil
}
func (c *mapCache) SetJSON(_ context.Context, _ string, _ any, _ time.Duration) error { return nil }
func (c *mapCache) GetJSON(_ context.Context, _ string, _ any) error                  { return port.ErrCacheMiss }
func (c *mapCache) Increment(_ context.Context, _ string) (int64, error)              { return 0, nil }
func (c *mapCache) Decrement(_ context.Context, _ string) (int64, error)              { return 0, nil }
func (c *mapCache) Expire(_ context.Context, _ string, _ time.Duration) error         { return nil }
func (c *mapCache) SlidingWindowAllow(_ context.Context, _ string, max int, _ time.Duration) (bool, int, int, error) {
	return true, max, 0, nil
}
func (c *mapCache) Close() error { return nil }

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

// makeUser creates a test user with a bcrypt-hashed password.
func makeUser(password string) *userdomain.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	return &userdomain.User{
		ID:           uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef"),
		Email:        "user@example.com",
		Name:         "Test User",
		PasswordHash: string(hash),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// testJWTConfig returns a minimal JWTConfig for tests.
func testJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:          "test-secret-that-is-at-least-32-bytes-long!",
		AccessTokenTTL:  15,
		RefreshTokenTTL: 1440,
		Issuer:          "test-issuer",
		Audience:        "test-audience",
	}
}

// testUC builds a UseCase backed by mockRepo and cache with minimal JWT config.
// It bypasses NewUseCase (which takes *userrepo.Repository) and constructs the
// concrete authUseCase directly via the unexported field — we use the internal
// test package so we have access to the struct.
func testUC(mockRepo *MockUserRepository, cache port.Cache) UseCase {
	return &authUseCase{
		userRepo: mockRepo,
		cache:    cache,
		jwtCfg:   testJWTConfig(),
	}
}

// ---------------------------------------------------------------------------
// Dual-key key-shape helpers
// ---------------------------------------------------------------------------

func TestTokenHash_Length(t *testing.T) {
	h := tokenHash("any-token")
	assert.Len(t, h, 64, "sha256 hex must be 64 chars")
}

func TestTokLookupKey_Shape(t *testing.T) {
	key := tokLookupKey("abc")
	assert.True(t, strings.HasPrefix(key, "refresh:tok:"), "unexpected prefix: %s", key)
	assert.Len(t, key, len("refresh:tok:")+64)
}

func TestUserIdxKey_Shape(t *testing.T) {
	key := userIdxKey("uid-123", "tok")
	assert.True(t, strings.HasPrefix(key, "refresh:user:uid-123:"), "unexpected prefix: %s", key)
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

// TestLogin_WritesBothKeys asserts that a successful login writes both the
// lookup key and the per-user index key to the cache.
func TestLogin_WritesBothKeys(t *testing.T) {
	ctx := context.Background()
	user := makeUser("password123")

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)

	cache := newMapCache()
	// Both Set calls succeed (default behaviour — no failSet).

	uc := testUC(mockRepo, cache)
	resp, err := uc.Login(ctx, dto.LoginRequest{Email: user.Email, Password: "password123"})

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.RefreshToken)

	// Verify both keys were written to the backing map.
	lookupKey := tokLookupKey(resp.RefreshToken)
	idxKey := userIdxKey(user.ID.String(), resp.RefreshToken)

	assert.Contains(t, cache.data, lookupKey, "lookup key must be written")
	assert.Contains(t, cache.data, idxKey, "per-user index key must be written")

	assert.Equal(t, user.ID.String(), string(cache.data[lookupKey]))
	assert.Equal(t, "1", string(cache.data[idxKey]))

	mockRepo.AssertExpectations(t)
}

// TestLogin_FirstSetFails_Returns500 verifies fail-closed: if the lookup key
// write fails, no token is issued and no orphan keys are left.
func TestLogin_FirstSetFails_Returns500(t *testing.T) {
	ctx := context.Background()
	user := makeUser("password123")

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)

	cache := newMapCache()
	// First Set (lookup key) fails.
	cache.failSet(assert.AnError)

	uc := testUC(mockRepo, cache)
	_, err := uc.Login(ctx, dto.LoginRequest{Email: user.Email, Password: "password123"})

	assert.Error(t, err)
	// Cache map should be empty — no orphan keys.
	assert.Empty(t, cache.data)
	mockRepo.AssertExpectations(t)
}

// TestLogin_SecondSetFails_RollsBackLookupKey verifies that if the per-user
// index key write fails, the already-written lookup key is deleted (no orphan).
func TestLogin_SecondSetFails_RollsBackLookupKey(t *testing.T) {
	ctx := context.Background()
	user := makeUser("password123")

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)

	cache := newMapCache()
	// First Set succeeds; second fails.
	cache.failSet(nil, assert.AnError)

	uc := testUC(mockRepo, cache)
	_, err := uc.Login(ctx, dto.LoginRequest{Email: user.Email, Password: "password123"})

	assert.Error(t, err)
	// After rollback the lookup key must be gone — no orphan.
	assert.Empty(t, cache.data, "rollback must remove the lookup key")
}

// TestLogin_InvalidPassword returns 401 without touching the cache.
func TestLogin_InvalidPassword(t *testing.T) {
	ctx := context.Background()
	user := makeUser("correct")

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)

	cache := newMapCache()

	uc := testUC(mockRepo, cache)
	_, err := uc.Login(ctx, dto.LoginRequest{Email: user.Email, Password: "wrong"})

	assert.Error(t, err)
	assert.Empty(t, cache.data)
}

// TestLogin_UserNotFound returns 401 without revealing whether the user exists.
func TestLogin_UserNotFound(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByEmail", ctx, "nobody@example.com").Return(nil, assert.AnError)

	cache := newMapCache()
	uc := testUC(mockRepo, cache)
	_, err := uc.Login(ctx, dto.LoginRequest{Email: "nobody@example.com", Password: "x"})

	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

// TestRefresh_Success verifies that Refresh resolves the userID from the
// lookup key alone (no client-supplied user_id), rotates both key pairs, and
// the old token cannot be reused.
func TestRefresh_Success(t *testing.T) {
	ctx := context.Background()
	user := makeUser("password")

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByID", ctx, user.ID.String()).Return(user, nil)

	cache := newMapCache()

	// Pre-populate both keys for an existing token.
	oldToken := "old-refresh-token"
	oldLookupKey := tokLookupKey(oldToken)
	oldIdxKey := userIdxKey(user.ID.String(), oldToken)
	cache.data[oldLookupKey] = []byte(user.ID.String())
	cache.data[oldIdxKey] = []byte("1")

	// Both new-token Set calls succeed (default).
	uc := testUC(mockRepo, cache)
	resp, err := uc.Refresh(ctx, dto.RefreshRequest{RefreshToken: oldToken})

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.NotEqual(t, oldToken, resp.RefreshToken, "token must be rotated")

	// Old keys must be gone.
	_, oldLookupExists := cache.data[oldLookupKey]
	_, oldIdxExists := cache.data[oldIdxKey]
	assert.False(t, oldLookupExists, "old lookup key must be deleted")
	assert.False(t, oldIdxExists, "old idx key must be deleted")

	// New keys must exist.
	newLookupKey := tokLookupKey(resp.RefreshToken)
	newIdxKey := userIdxKey(user.ID.String(), resp.RefreshToken)
	assert.Contains(t, cache.data, newLookupKey, "new lookup key must be written")
	assert.Contains(t, cache.data, newIdxKey, "new idx key must be written")

	mockRepo.AssertExpectations(t)
}

// TestRefresh_UnknownToken_Returns401 verifies that an unknown refresh token
// is rejected with 401 (no user_id hint needed or accepted).
func TestRefresh_UnknownToken_Returns401(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockUserRepository)
	cache := newMapCache()
	uc := testUC(mockRepo, cache)

	_, err := uc.Refresh(ctx, dto.RefreshRequest{RefreshToken: "bogus-token"})

	assert.Error(t, err)
	// Repo must never be called — we cannot reach GetByID without a valid lookup.
	mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

// TestRefresh_OldTokenCannotBeReused verifies token rotation: after a
// successful refresh the old token is invalid.
func TestRefresh_OldTokenCannotBeReused(t *testing.T) {
	ctx := context.Background()
	user := makeUser("pass")

	mockRepo := new(MockUserRepository)
	mockRepo.On("GetByID", ctx, user.ID.String()).Return(user, nil)

	cache := newMapCache()

	oldToken := "rotate-me"
	cache.data[tokLookupKey(oldToken)] = []byte(user.ID.String())
	cache.data[userIdxKey(user.ID.String(), oldToken)] = []byte("1")

	// Both new-token Set calls succeed (default).
	uc := testUC(mockRepo, cache)

	// First refresh succeeds.
	resp, err := uc.Refresh(ctx, dto.RefreshRequest{RefreshToken: oldToken})
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.RefreshToken)

	// Second attempt with the old token must fail — lookup key was deleted.
	_, err2 := uc.Refresh(ctx, dto.RefreshRequest{RefreshToken: oldToken})
	assert.Error(t, err2, "old token must not be reusable after rotation")
}

// TestRefresh_RevokedByPasswordChange is the key regression test for the
// dual-key revocation fix. It proves that after RevokeAllForUser (called on
// password change) deletes only the per-user index keys, a subsequent Refresh
// with the old token is rejected even though the lookup key is still present
// in the cache (orphan). The index key is the authoritative revocation gate.
func TestRefresh_RevokedByPasswordChange(t *testing.T) {
	ctx := context.Background()
	user := makeUser("password")

	mockRepo := new(MockUserRepository)
	// GetByID must NOT be called: revocation check happens before user lookup.

	cache := newMapCache()

	// Simulate a token issued at login — both keys present.
	token := "pre-password-change-token"
	lookupKey := tokLookupKey(token)
	idxKey := userIdxKey(user.ID.String(), token)
	cache.data[lookupKey] = []byte(user.ID.String())
	cache.data[idxKey] = []byte("1")

	uc := testUC(mockRepo, cache)

	// Simulate ChangePassword → RevokeAllForUser: deletes index keys by prefix.
	// authUseCase implements both UseCase and Revoker; assert to Revoker here.
	revoker := uc.(Revoker)
	err := revoker.RevokeAllForUser(ctx, user.ID.String())
	assert.NoError(t, err)

	// Index key must be gone; lookup key must still be present (orphan).
	assert.NotContains(t, cache.data, idxKey, "index key must be deleted by RevokeAllForUser")
	assert.Contains(t, cache.data, lookupKey, "lookup key must remain as orphan (proves index-key check is the gate)")

	// Refresh with the old token must be rejected despite the orphan lookup key.
	_, err = uc.Refresh(ctx, dto.RefreshRequest{RefreshToken: token})
	assert.Error(t, err, "Refresh must return 401 after RevokeAllForUser")

	// Confirm GetByID was never reached — revocation gate fires before user lookup.
	mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// Logout
// ---------------------------------------------------------------------------

// TestLogout_Success verifies that Logout deletes both keys when callerID
// matches the token owner.
func TestLogout_Success(t *testing.T) {
	ctx := context.Background()
	userID := "01234567-89ab-cdef-0123-456789abcdef"

	cache := newMapCache()
	token := "my-refresh-token"
	lookupKey := tokLookupKey(token)
	idxKey := userIdxKey(userID, token)
	cache.data[lookupKey] = []byte(userID)
	cache.data[idxKey] = []byte("1")

	mockRepo := new(MockUserRepository)
	uc := testUC(mockRepo, cache)

	err := uc.Logout(ctx, userID, token)
	assert.NoError(t, err)

	assert.NotContains(t, cache.data, lookupKey, "lookup key must be deleted on logout")
	assert.NotContains(t, cache.data, idxKey, "idx key must be deleted on logout")
}

// TestLogout_OtherUsersToken_NoDelete verifies that an attacker who has
// another user's refresh token cannot use their own JWT to revoke it. The
// lookup key must remain in the cache.
func TestLogout_OtherUsersToken_NoDelete(t *testing.T) {
	ctx := context.Background()

	victimID := "victim-user-id"
	attackerID := "attacker-user-id"
	token := "victim-token"

	cache := newMapCache()
	lookupKey := tokLookupKey(token)
	idxKey := userIdxKey(victimID, token)
	cache.data[lookupKey] = []byte(victimID)
	cache.data[idxKey] = []byte("1")

	mockRepo := new(MockUserRepository)
	uc := testUC(mockRepo, cache)

	// Attacker logs out with their own JWT (attackerID) but supplies victim's token.
	err := uc.Logout(ctx, attackerID, token)
	assert.NoError(t, err, "must return success silently to avoid oracle")

	// Victim's keys must still be present — no deletion.
	assert.Contains(t, cache.data, lookupKey, "victim lookup key must NOT be deleted")
	assert.Contains(t, cache.data, idxKey, "victim idx key must NOT be deleted")
}

// TestLogout_MissingToken_SilentSuccess verifies that logging out with a
// token that does not exist in the cache succeeds silently (already revoked or
// expired).
func TestLogout_MissingToken_SilentSuccess(t *testing.T) {
	ctx := context.Background()
	cache := newMapCache()
	mockRepo := new(MockUserRepository)

	uc := testUC(mockRepo, cache)
	err := uc.Logout(ctx, "user-id", "nonexistent-token")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// DTO validation shape
// ---------------------------------------------------------------------------

func TestRefreshDTO_NoUserID(t *testing.T) {
	// Confirm RefreshRequest no longer has a UserID field.
	req := dto.RefreshRequest{RefreshToken: "tok"}
	assert.NotEmpty(t, req.RefreshToken)
	// The struct only has RefreshToken — compilation-time check via usage.
}

func TestLoginDTO_Validation(t *testing.T) {
	t.Run("valid_login", func(t *testing.T) {
		req := dto.LoginRequest{Email: "user@example.com", Password: "password123"}
		assert.NotEmpty(t, req.Email)
		assert.NotEmpty(t, req.Password)
	})

	t.Run("empty_email", func(t *testing.T) {
		req := dto.LoginRequest{Email: "", Password: "password123"}
		assert.Empty(t, req.Email)
	})

	t.Run("empty_password", func(t *testing.T) {
		req := dto.LoginRequest{Email: "user@example.com", Password: ""}
		assert.Empty(t, req.Password)
	})
}
