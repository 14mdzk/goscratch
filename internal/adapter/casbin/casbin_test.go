package casbin

import (
	"context"
	"testing"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"

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
		enforcer: enforcer,
		db:       nil, // no DB for tests
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
	url := BuildDatabaseURL("localhost", 5432, "user", "pass", "mydb")
	assert.Equal(t, "postgres://user:pass@localhost:5432/mydb?sslmode=disable", url)
}
