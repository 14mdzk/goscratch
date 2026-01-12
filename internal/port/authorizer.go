package port

import "context"

// Authorizer defines the interface for authorization operations
type Authorizer interface {
	// Enforce checks if subject has permission to perform action on object
	Enforce(sub, obj, act string) (bool, error)

	// EnforceWithContext checks permission with context for cancellation
	EnforceWithContext(ctx context.Context, sub, obj, act string) (bool, error)

	// Role management
	AddRoleForUser(userID, role string) error
	RemoveRoleForUser(userID, role string) error
	GetRolesForUser(userID string) ([]string, error)
	GetUsersForRole(role string) ([]string, error)
	HasRoleForUser(userID, role string) (bool, error)

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

	// Lifecycle
	Close() error
}

// Common roles
const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleEditor     = "editor"
	RoleViewer     = "viewer"
)
