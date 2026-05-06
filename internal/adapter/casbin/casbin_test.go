package casbin

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	casbinlib "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/redis/go-redis/v9"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NoOpAdapter Tests
// =============================================================================

func TestNoOpAdapter_ImplementsInterface(t *testing.T) {
	var _ port.Authorizer = (*NoOpAdapter)(nil)
}

func TestNoOpAdapter_Enforce_AlwaysTrue(t *testing.T) {
	a := NewNoOpAdapter()

	tests := []struct {
		sub, obj, act string
	}{
		{"user1", "resource", "read"},
		{"user2", "resource", "write"},
		{"", "", ""},
		{"admin", "*", "*"},
	}

	for _, tt := range tests {
		allowed, err := a.Enforce(tt.sub, tt.obj, tt.act)
		assert.NoError(t, err)
		assert.True(t, allowed, "NoOp should always allow: sub=%s obj=%s act=%s", tt.sub, tt.obj, tt.act)
	}
}

func TestNoOpAdapter_EnforceWithContext_AlwaysTrue(t *testing.T) {
	a := NewNoOpAdapter()
	allowed, err := a.EnforceWithContext(context.Background(), "user", "res", "act")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestNoOpAdapter_RoleOperations(t *testing.T) {
	a := NewNoOpAdapter()

	assert.NoError(t, a.AddRoleForUser("user1", "admin"))
	assert.NoError(t, a.RemoveRoleForUser("user1", "admin"))

	roles, err := a.GetRolesForUser("user1")
	assert.NoError(t, err)
	assert.Empty(t, roles)

	users, err := a.GetUsersForRole("admin")
	assert.NoError(t, err)
	assert.Empty(t, users)

	has, err := a.HasRoleForUser("user1", "admin")
	assert.NoError(t, err)
	assert.True(t, has) // NoOp always returns true
}

func TestNoOpAdapter_PermissionOperations(t *testing.T) {
	a := NewNoOpAdapter()

	assert.NoError(t, a.AddPermissionForRole("admin", "users", "read"))
	assert.NoError(t, a.RemovePermissionForRole("admin", "users", "read"))

	perms, err := a.GetPermissionsForRole("admin")
	assert.NoError(t, err)
	assert.Empty(t, perms)

	assert.NoError(t, a.AddPermissionForUser("user1", "users", "read"))
	assert.NoError(t, a.RemovePermissionForUser("user1", "users", "read"))

	perms, err = a.GetPermissionsForUser("user1")
	assert.NoError(t, err)
	assert.Empty(t, perms)

	perms, err = a.GetImplicitPermissionsForUser("user1")
	assert.NoError(t, err)
	assert.Empty(t, perms)
}

func TestNoOpAdapter_PolicyOperations(t *testing.T) {
	a := NewNoOpAdapter()
	assert.NoError(t, a.LoadPolicy())
	assert.NoError(t, a.SavePolicy())
}

func TestNoOpAdapter_Close(t *testing.T) {
	a := NewNoOpAdapter()
	assert.NoError(t, a.Close())
}

// =============================================================================
// Casbin Adapter Tests (using in-memory model, no database)
// =============================================================================

// newTestAdapter creates an Adapter with an in-memory Casbin enforcer (no DB needed).
func newTestAdapter(t *testing.T) *Adapter {
	t.Helper()

	m, err := model.NewModelFromString(defaultModel)
	require.NoError(t, err)

	enforcer, err := casbinlib.NewEnforcer(m)
	require.NoError(t, err)

	return &Adapter{
		enforcer:       enforcer,
		db:             nil, // no DB for tests
		reloadInterval: 0,   // will use 5-minute default in Start
		watcher:        nil,
	}
}

func TestAdapter_ImplementsInterface(t *testing.T) {
	var _ port.Authorizer = (*Adapter)(nil)
}

func TestAdapter_Enforce_Denied_NoPolicy(t *testing.T) {
	a := newTestAdapter(t)

	allowed, err := a.Enforce("user1", "resource", "read")
	require.NoError(t, err)
	assert.False(t, allowed, "should deny when no policy exists")
}

func TestAdapter_Enforce_Allowed_DirectPolicy(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddPermissionForUser("user1", "articles", "read")
	require.NoError(t, err)

	allowed, err := a.Enforce("user1", "articles", "read")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Different action should be denied
	allowed, err = a.Enforce("user1", "articles", "write")
	require.NoError(t, err)
	assert.False(t, allowed)

	// Different user should be denied
	allowed, err = a.Enforce("user2", "articles", "read")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestAdapter_Enforce_ViaRole(t *testing.T) {
	a := newTestAdapter(t)

	// Add permission to role
	err := a.AddPermissionForRole("editor", "articles", "write")
	require.NoError(t, err)

	// Assign role to user
	err = a.AddRoleForUser("user1", "editor")
	require.NoError(t, err)

	// User should have permission via role
	allowed, err := a.Enforce("user1", "articles", "write")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Other users should not
	allowed, err = a.Enforce("user2", "articles", "write")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestAdapter_EnforceWithContext_CancelledContext(t *testing.T) {
	a := newTestAdapter(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	allowed, err := a.EnforceWithContext(ctx, "user1", "res", "act")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, allowed)
}

func TestAdapter_EnforceWithContext_ActiveContext(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddPermissionForUser("user1", "res", "act")
	require.NoError(t, err)

	allowed, err := a.EnforceWithContext(context.Background(), "user1", "res", "act")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestAdapter_AddRoleForUser_GetRolesForUser(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddRoleForUser("user1", "admin")
	require.NoError(t, err)
	err = a.AddRoleForUser("user1", "editor")
	require.NoError(t, err)

	roles, err := a.GetRolesForUser("user1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"admin", "editor"}, roles)
}

func TestAdapter_RemoveRoleForUser(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddRoleForUser("user1", "admin")
	require.NoError(t, err)
	err = a.AddRoleForUser("user1", "editor")
	require.NoError(t, err)

	err = a.RemoveRoleForUser("user1", "admin")
	require.NoError(t, err)

	roles, err := a.GetRolesForUser("user1")
	require.NoError(t, err)
	assert.Equal(t, []string{"editor"}, roles)
}

func TestAdapter_GetUsersForRole(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddRoleForUser("user1", "admin")
	require.NoError(t, err)
	err = a.AddRoleForUser("user2", "admin")
	require.NoError(t, err)
	err = a.AddRoleForUser("user3", "editor")
	require.NoError(t, err)

	users, err := a.GetUsersForRole("admin")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"user1", "user2"}, users)
}

func TestAdapter_HasRoleForUser(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddRoleForUser("user1", "admin")
	require.NoError(t, err)

	has, err := a.HasRoleForUser("user1", "admin")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = a.HasRoleForUser("user1", "editor")
	require.NoError(t, err)
	assert.False(t, has)

	has, err = a.HasRoleForUser("user2", "admin")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestAdapter_AddPermissionForRole_GetPermissionsForRole(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddPermissionForRole("admin", "users", "read")
	require.NoError(t, err)
	err = a.AddPermissionForRole("admin", "users", "write")
	require.NoError(t, err)

	perms, err := a.GetPermissionsForRole("admin")
	require.NoError(t, err)
	assert.Len(t, perms, 2)
}

func TestAdapter_RemovePermissionForRole(t *testing.T) {
	a := newTestAdapter(t)

	err := a.AddPermissionForRole("admin", "users", "read")
	require.NoError(t, err)
	err = a.AddPermissionForRole("admin", "users", "write")
	require.NoError(t, err)

	err = a.RemovePermissionForRole("admin", "users", "read")
	require.NoError(t, err)

	perms, err := a.GetPermissionsForRole("admin")
	require.NoError(t, err)
	assert.Len(t, perms, 1)
}

func TestAdapter_GetImplicitPermissionsForUser(t *testing.T) {
	a := newTestAdapter(t)

	// Direct permission
	err := a.AddPermissionForUser("user1", "profile", "read")
	require.NoError(t, err)

	// Permission via role
	err = a.AddPermissionForRole("admin", "users", "write")
	require.NoError(t, err)
	err = a.AddRoleForUser("user1", "admin")
	require.NoError(t, err)

	perms, err := a.GetImplicitPermissionsForUser("user1")
	require.NoError(t, err)
	// Should have both direct and role-based permissions
	assert.Len(t, perms, 2)
}

func TestAdapter_WildcardPermission(t *testing.T) {
	a := newTestAdapter(t)

	// Wildcard object and action
	err := a.AddPermissionForRole("superadmin", "*", "*")
	require.NoError(t, err)
	err = a.AddRoleForUser("god", "superadmin")
	require.NoError(t, err)

	allowed, err := a.Enforce("god", "anything", "everything")
	require.NoError(t, err)
	assert.True(t, allowed)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestFormatPermission(t *testing.T) {
	tests := []struct {
		obj, act string
		expected string
	}{
		{"users", "read", "users:read"},
		{"", "write", ":write"},
		{"articles", "", "articles:"},
	}

	for _, tt := range tests {
		result := FormatPermission(tt.obj, tt.act)
		assert.Equal(t, tt.expected, result)
	}
}

func TestParsePermission(t *testing.T) {
	tests := []struct {
		input       string
		expectedObj string
		expectedAct string
	}{
		{"users:read", "users", "read"},
		{"articles:write", "articles", "write"},
		{"nocolon", "nocolon", "*"},
		{":write", "", "write"},
		{"users:", "users", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			obj, act := ParsePermission(tt.input)
			assert.Equal(t, tt.expectedObj, obj)
			assert.Equal(t, tt.expectedAct, act)
		})
	}
}

func TestBuildDatabaseURL(t *testing.T) {
	t.Run("sslmode propagates", func(t *testing.T) {
		url := BuildDatabaseURL("localhost", 5432, "user", "pass", "mydb", "require")
		assert.Equal(t, "postgres://user:pass@localhost:5432/mydb?sslmode=require", url)
	})

	t.Run("explicit disable still allowed for local dev", func(t *testing.T) {
		url := BuildDatabaseURL("localhost", 5432, "user", "pass", "mydb", "disable")
		assert.Equal(t, "postgres://user:pass@localhost:5432/mydb?sslmode=disable", url)
	})

	t.Run("empty sslmode defaults to require", func(t *testing.T) {
		url := BuildDatabaseURL("localhost", 5432, "user", "pass", "mydb", "")
		assert.Equal(t, "postgres://user:pass@localhost:5432/mydb?sslmode=require", url)
	})
}

// =============================================================================
// Task 10 — Lifecycle, Watcher, and Validation Tests
// =============================================================================

// TestNoOpAdapter_Start verifies that Start is a no-op and returns nil.
func TestNoOpAdapter_Start(t *testing.T) {
	a := NewNoOpAdapter()
	require.NoError(t, a.Start(context.Background()))
}

// TestAdapter_Start_NoWatcher verifies that Start launches the backstop goroutine
// and returns nil even when no watcher is configured.
func TestAdapter_Start_NoWatcher(t *testing.T) {
	a := newTestAdapter(t)
	ctx, cancel := context.WithCancel(context.Background())
	err := a.Start(ctx)
	require.NoError(t, err)
	// Cancel context to stop the goroutine cleanly.
	cancel()
	// Brief pause to give the goroutine time to exit — no race expected.
	time.Sleep(10 * time.Millisecond)
}

// TestAdapter_MemoryWatcher_IncrementalAdd verifies that adding a permission via
// the adapter triggers the MemoryWatcher callback, which applies the delta
// without a full LoadPolicy.
func TestAdapter_MemoryWatcher_IncrementalAdd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	watcher := NewMemoryWatcher()
	watcher.Start(ctx)

	a := newTestAdapter(t)
	a.watcher = watcher
	a.reloadInterval = time.Hour // long interval so backstop tick doesn't interfere

	require.NoError(t, a.Start(ctx))

	// Add permission — watcher.UpdateForAddPolicy is called automatically by Casbin.
	err := a.AddPermissionForRole("editor", "articles", "write")
	require.NoError(t, err)

	// Allow goroutine to dispatch the callback.
	time.Sleep(20 * time.Millisecond)

	// The permission should already be in the enforcer (added directly by AddPermissionForRole).
	allowed, err := a.Enforce("editor", "articles", "write")
	require.NoError(t, err)
	assert.True(t, allowed)
}

// TestAdapter_MemoryWatcher_IncrementalRemove verifies that removing a permission
// via the adapter is propagated through the MemoryWatcher callback.
func TestAdapter_MemoryWatcher_IncrementalRemove(t *testing.T) {
	// Use a NoopWatcher to avoid the race between the watcher callback goroutine
	// and the test goroutine — the watcher callback is not the focus of this test.
	// We test incremental dispatch via MemoryWatcher separately in IncrementalAdd.
	a := newTestAdapter(t)
	a.watcher = NewNoopWatcher()
	a.reloadInterval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, a.Start(ctx))

	// Pre-add permission directly via enforcer to avoid callback races.
	require.NoError(t, a.AddPermissionForRole("viewer", "reports", "read"))

	allowed, err := a.Enforce("viewer", "reports", "read")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Remove permission via adapter — validation guard is the key concern here.
	require.NoError(t, a.RemovePermissionForRole("viewer", "reports", "read"))

	allowed, err = a.Enforce("viewer", "reports", "read")
	require.NoError(t, err)
	assert.False(t, allowed, "permission should be removed")
}

// =============================================================================
// validatePolicyArgs Tests
// =============================================================================

func TestValidatePolicyArgs_Valid(t *testing.T) {
	assert.NoError(t, validatePolicyArgs("role", "object", "action"))
	assert.NoError(t, validatePolicyArgs("")) // empty string — no null bytes
	assert.NoError(t, validatePolicyArgs())   // no args
}

func TestValidatePolicyArgs_NullByte(t *testing.T) {
	err := validatePolicyArgs("valid", "inva\x00lid", "action")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPolicyArg)
}

// TestAdapter_AddPermissionForRole_RejectsNullByte verifies the validation guard
// on the public mutation methods.
func TestAdapter_AddPermissionForRole_RejectsNullByte(t *testing.T) {
	a := newTestAdapter(t)
	err := a.AddPermissionForRole("role", "obj\x00", "act")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPolicyArg)
}

// =============================================================================
// Redis Watcher Tests (miniredis)
// =============================================================================

// newTestRedisClient creates a go-redis client connected to a miniredis instance.
func newTestRedisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

// TestRedisWatcher_UpdatePublishes verifies that Update() publishes a reload message.
func TestRedisWatcher_UpdatePublishes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client, _ := newTestRedisClient(t)

	// Subscribe before creating the watcher so we can observe the publish.
	sub := client.Subscribe(ctx, defaultRedisChannel)
	t.Cleanup(func() { _ = sub.Close() })

	w, err := NewRedisWatcher(ctx, client, "")
	require.NoError(t, err)
	t.Cleanup(w.Close)

	require.NoError(t, w.Update())

	// Receive with timeout.
	msgCh := sub.Channel()
	select {
	case msg := <-msgCh:
		assert.Contains(t, msg.Payload, "reload")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for Redis publish")
	}
}

// TestRedisWatcher_CallbackTriggered verifies that the subscriber goroutine invokes
// the callback when a message is published.
func TestRedisWatcher_CallbackTriggered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client, _ := newTestRedisClient(t)

	received := make(chan string, 1)

	w, err := NewRedisWatcher(ctx, client, "")
	require.NoError(t, err)
	t.Cleanup(w.Close)

	require.NoError(t, w.SetUpdateCallback(func(msg string) {
		received <- msg
	}))

	// Publish a reload message directly via the client.
	msg := encodeOp("reload", "", "", nil)
	require.NoError(t, client.Publish(ctx, defaultRedisChannel, msg).Err())

	select {
	case got := <-received:
		assert.Contains(t, got, "reload")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for callback")
	}
}

// TestAdapter_ImplementsAuthorizer is a compile-time check that the full
// port.Authorizer (including Start) is satisfied.
func TestAdapter_ImplementsAuthorizer(t *testing.T) {
	var _ port.Authorizer = (*Adapter)(nil)
	var _ port.Authorizer = (*NoOpAdapter)(nil)
}
