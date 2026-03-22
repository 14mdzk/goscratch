package role

import (
	"github.com/14mdzk/goscratch/internal/module/role/handler"
	"github.com/14mdzk/goscratch/internal/module/role/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/gofiber/fiber/v2"
)

// Module represents the role management module
type Module struct {
	handler    *handler.Handler
	authorizer port.Authorizer
	jwtSecret  string
}

// NewModule creates a new role module
func NewModule(authorizer port.Authorizer, jwtSecret string) *Module {
	uc := usecase.NewUseCase(authorizer)
	h := handler.NewHandler(uc)

	return &Module{
		handler:    h,
		authorizer: authorizer,
		jwtSecret:  jwtSecret,
	}
}

// RegisterRoutes registers role module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret))
	adminOnly := middleware.RequireAnyRole(m.authorizer, port.RoleSuperAdmin, port.RoleAdmin)

	// Role management routes
	roles := router.Group("/roles")
	roles.Use(authMiddleware)
	roles.Use(adminOnly)

	roles.Get("/", m.handler.ListRoles)
	roles.Post("/assign", m.handler.AssignRole)
	roles.Post("/revoke", m.handler.RevokeRole)
	roles.Get("/:role/users", m.handler.GetRoleUsers)
	roles.Get("/:role/permissions", m.handler.GetRolePermissions)
	roles.Post("/:role/permissions", m.handler.AddRolePermission)
	roles.Delete("/:role/permissions", m.handler.RemoveRolePermission)

	// User role/permission lookup routes (under /users/:id)
	users := router.Group("/users")
	users.Use(authMiddleware)
	users.Use(adminOnly)

	users.Get("/:id/roles", m.handler.GetUserRoles)
	users.Get("/:id/permissions", m.handler.GetUserPermissions)
}
