package casbin

import (
	"context"

	"github.com/14mdzk/goscratch/internal/port"
)

// NoOpAdapter is a no-operation authorizer that always allows access
// Use this for development or when authorization is disabled
type NoOpAdapter struct{}

// NewNoOpAdapter creates a new NoOp authorizer
func NewNoOpAdapter() *NoOpAdapter {
	return &NoOpAdapter{}
}

// Enforce always returns true
func (a *NoOpAdapter) Enforce(sub, obj, act string) (bool, error) {
	return true, nil
}

// EnforceWithContext always returns true
func (a *NoOpAdapter) EnforceWithContext(ctx context.Context, sub, obj, act string) (bool, error) {
	return true, nil
}

// AddRoleForUser is a no-op
func (a *NoOpAdapter) AddRoleForUser(userID, role string) error {
	return nil
}

// RemoveRoleForUser is a no-op
func (a *NoOpAdapter) RemoveRoleForUser(userID, role string) error {
	return nil
}

// GetRolesForUser returns empty slice
func (a *NoOpAdapter) GetRolesForUser(userID string) ([]string, error) {
	return []string{}, nil
}

// GetUsersForRole returns empty slice
func (a *NoOpAdapter) GetUsersForRole(role string) ([]string, error) {
	return []string{}, nil
}

// HasRoleForUser always returns true
func (a *NoOpAdapter) HasRoleForUser(userID, role string) (bool, error) {
	return true, nil
}

// AddPermissionForRole is a no-op
func (a *NoOpAdapter) AddPermissionForRole(role, obj, act string) error {
	return nil
}

// RemovePermissionForRole is a no-op
func (a *NoOpAdapter) RemovePermissionForRole(role, obj, act string) error {
	return nil
}

// GetPermissionsForRole returns empty slice
func (a *NoOpAdapter) GetPermissionsForRole(role string) ([][]string, error) {
	return [][]string{}, nil
}

// AddPermissionForUser is a no-op
func (a *NoOpAdapter) AddPermissionForUser(userID, obj, act string) error {
	return nil
}

// RemovePermissionForUser is a no-op
func (a *NoOpAdapter) RemovePermissionForUser(userID, obj, act string) error {
	return nil
}

// GetPermissionsForUser returns empty slice
func (a *NoOpAdapter) GetPermissionsForUser(userID string) ([][]string, error) {
	return [][]string{}, nil
}

// GetImplicitPermissionsForUser returns empty slice
func (a *NoOpAdapter) GetImplicitPermissionsForUser(userID string) ([][]string, error) {
	return [][]string{}, nil
}

// LoadPolicy is a no-op
func (a *NoOpAdapter) LoadPolicy() error {
	return nil
}

// SavePolicy is a no-op
func (a *NoOpAdapter) SavePolicy() error {
	return nil
}

// Close is a no-op
func (a *NoOpAdapter) Close() error {
	return nil
}

// Ensure NoOpAdapter implements port.Authorizer
var _ port.Authorizer = (*NoOpAdapter)(nil)
