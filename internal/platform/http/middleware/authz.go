package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/response"
)

// AuthzConfig holds configuration for authorization middleware
type AuthzConfig struct {
	Enforcer port.Enforcer
	Object   string // e.g., "users", "orders"
	Action   string // e.g., "read", "create", "update", "delete"
}

// RequirePermission creates middleware that checks if user has the required permission
func RequirePermission(enforcer port.Enforcer, obj, act string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return response.Unauthorized(c, "authentication required")
		}

		allowed, err := enforcer.EnforceWithContext(c.UserContext(), userID, obj, act)
		if err != nil {
			return response.Fail(c, apperr.Internalf("authorization check failed"))
		}

		if !allowed {
			return response.Forbidden(c, "insufficient permissions")
		}

		return c.Next()
	}
}

// RequireRole creates middleware that checks if user has the required role
func RequireRole(enforcer port.Enforcer, role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return response.Unauthorized(c, "authentication required")
		}

		hasRole, err := enforcer.HasRoleForUser(userID, role)
		if err != nil {
			return response.Fail(c, apperr.Internalf("authorization check failed"))
		}

		if !hasRole {
			return response.Forbidden(c, "insufficient role")
		}

		return c.Next()
	}
}

// RequireAnyPermission creates middleware that checks if user has any of the required permissions
func RequireAnyPermission(enforcer port.Enforcer, permissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return response.Unauthorized(c, "authentication required")
		}

		for _, perm := range permissions {
			obj, act := parsePermission(perm)
			allowed, err := enforcer.EnforceWithContext(c.UserContext(), userID, obj, act)
			if err != nil {
				continue
			}
			if allowed {
				return c.Next()
			}
		}

		return response.Forbidden(c, "insufficient permissions")
	}
}

// RequireAllPermissions creates middleware that checks if user has all required permissions
func RequireAllPermissions(enforcer port.Enforcer, permissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return response.Unauthorized(c, "authentication required")
		}

		for _, perm := range permissions {
			obj, act := parsePermission(perm)
			allowed, err := enforcer.EnforceWithContext(c.UserContext(), userID, obj, act)
			if err != nil || !allowed {
				return response.Forbidden(c, "insufficient permissions")
			}
		}

		return c.Next()
	}
}

// RequireAnyRole creates middleware that checks if user has any of the specified roles
func RequireAnyRole(enforcer port.Enforcer, roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return response.Unauthorized(c, "authentication required")
		}

		for _, role := range roles {
			hasRole, err := enforcer.HasRoleForUser(userID, role)
			if err != nil {
				continue
			}
			if hasRole {
				return c.Next()
			}
		}

		return response.Forbidden(c, "insufficient role")
	}
}

// parsePermission splits "object:action" into obj and act
func parsePermission(perm string) (obj, act string) {
	for i := 0; i < len(perm); i++ {
		if perm[i] == ':' {
			return perm[:i], perm[i+1:]
		}
	}
	return perm, "*"
}
