package port

import "context"

// Enforcer evaluates access-control checks. HTTP middleware and modules that
// only guard routes depend on this narrow interface rather than the full
// Authorizer lifecycle and policy-management surface.
type Enforcer interface {
	// Enforce checks if subject has permission to perform action on object
	Enforce(sub, obj, act string) (bool, error)

	// EnforceWithContext checks permission with context for cancellation
	EnforceWithContext(ctx context.Context, sub, obj, act string) (bool, error)

	// HasRoleForUser checks whether userID holds the given role
	HasRoleForUser(userID, role string) (bool, error)
}

// PolicyManager manages RBAC role assignments, permissions, and policy persistence.
type PolicyManager interface {
	// Role management
	AddRoleForUser(userID, role string) error
	RemoveRoleForUser(userID, role string) error
	GetRolesForUser(userID string) ([]string, error)
	GetUsersForRole(role string) ([]string, error)

	// Permission management for roles
	AddPermissionForRole(role, obj, act string) error
	RemovePermissionForRole(role, obj, act string) error
	GetPermissionsForRole(role string) ([][]string, error)

	// Direct user permissions (bypass roles)
	AddPermissionForUser(userID, obj, act string) error
	RemovePermissionForUser(userID, obj, act string) error
	GetPermissionsForUser(userID string) ([][]string, error)

	// Get all implicit permissions for a user (including via roles)
	GetImplicitPermissionsForUser(userID string) ([][]string, error)

	// Policy management
	LoadPolicy() error
	SavePolicy() error
}

// Authorizer is the full lifecycle-managed authorization port wired at the
// composition root. Consumers that only check permissions should depend on
// Enforcer instead.
type Authorizer interface {
	Enforcer
	PolicyManager

	// Lifecycle
	Start(ctx context.Context) error
	Close() error
}

// Common roles
const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleEditor     = "editor"
	RoleViewer     = "viewer"
)
