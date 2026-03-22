package domain

import "github.com/14mdzk/goscratch/internal/port"

// Role represents a role in the system
type Role struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Permission represents a permission (object + action)
type Permission struct {
	Object string `json:"object"`
	Action string `json:"action"`
}

// RoleAssignment represents a user-to-role assignment
type RoleAssignment struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// PredefinedRoles returns all predefined roles in the system
var PredefinedRoles = []Role{
	{Name: port.RoleSuperAdmin, Description: "Full system access with all permissions"},
	{Name: port.RoleAdmin, Description: "Administrative access with most permissions"},
	{Name: port.RoleEditor, Description: "Can create and edit content"},
	{Name: port.RoleViewer, Description: "Read-only access"},
}

// IsValidRole checks if the given role name is a predefined role
func IsValidRole(name string) bool {
	for _, r := range PredefinedRoles {
		if r.Name == name {
			return true
		}
	}
	return false
}
