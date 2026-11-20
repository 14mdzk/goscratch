package role

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/role/handler"
	"github.com/14mdzk/goscratch/internal/module/role/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the role management module
type Module struct {
	handler        *handler.Handler
	authorizer     port.Authorizer
	authMiddleware fiber.Handler
}

// NewModule creates a new role module
func NewModule(authorizer port.Authorizer, authMiddleware fiber.Handler) *Module {
	uc := usecase.NewUseCase(authorizer)
	h := handler.NewHandler(uc)

	return &Module{
		handler:        h,
		authorizer:     authorizer,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers role module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	requireRead := middleware.RequirePermission(m.authorizer, "roles", "read")
	requireManage := middleware.RequirePermission(m.authorizer, "roles", "manage")

	// Role management routes
	roles := router.Group("/roles")
	roles.Use(m.authMiddleware)

	roles.Get("/", requireRead, m.handler.ListRoles)
	// Register /permissions before /:role/permissions to avoid route conflicts
	roles.Get("/permissions", requireRead, m.handler.ListAllPermissions)
	roles.Post("/assign", requireManage, m.handler.AssignRole)
	roles.Post("/revoke", requireManage, m.handler.RevokeRole)
	roles.Get("/:role/users", requireRead, m.handler.GetRoleUsers)
	roles.Get("/:role/permissions", requireRead, m.handler.GetRolePermissions)
	roles.Post("/:role/permissions", requireManage, m.handler.AddRolePermission)
	roles.Delete("/:role/permissions", requireManage, m.handler.RemoveRolePermission)

	// User role/permission lookup routes (under /users/:id)
	users := router.Group("/users")
	users.Use(m.authMiddleware)

	users.Get("/:id/roles", requireRead, m.handler.GetUserRoles)
	users.Get("/:id/permissions", requireRead, m.handler.GetUserPermissions)
	users.Post("/:id/permissions", requireManage, m.handler.AddUserPermission)
	users.Delete("/:id/permissions", requireManage, m.handler.RemoveUserPermission)
	users.Get("/:id/permissions/check", requireRead, m.handler.CheckUserPermission)
}
