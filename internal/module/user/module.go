package user

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/user/handler"
	"github.com/14mdzk/goscratch/internal/module/user/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the user module
type Module struct {
	handler        *handler.Handler
	enforcer       port.Enforcer
	authMiddleware fiber.Handler
}

// NewModule creates a new user module.
// repo is injected (shared with auth in the composition root) and transactor is
// passed via port.Transactor so neither the module nor its usecase imports
// concrete database or repository packages.
// authRevoker is the auth module's session-revocation interface, injected so
// ChangePassword can terminate all active refresh tokens for the user without
// importing the auth package (avoiding a circular dependency).
func NewModule(repo usecase.UserRepo, transactor port.Transactor, auditor port.Auditor, enforcer port.Enforcer, cache port.Cache, authMiddleware fiber.Handler, authRevoker usecase.AuthRevoker) *Module {
	uc := usecase.NewUseCase(repo, transactor, cache, authRevoker)
	audited := usecase.NewAuditedUseCase(uc, auditor)
	h := handler.NewHandler(audited)

	return &Module{
		handler:        h,
		enforcer:       enforcer,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers user module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	users := router.Group("/users")

	// Protected routes - require authentication
	users.Use(m.authMiddleware)

	// User self-management (no permission required beyond auth)
	users.Get("/me", m.handler.GetMe)
	users.Post("/me/password", m.handler.ChangePassword)

	// User management - require specific permissions
	users.Get("/", middleware.RequirePermission(m.enforcer, "users", "read"), m.handler.List)
	users.Get("/:id", middleware.RequirePermission(m.enforcer, "users", "read"), m.handler.GetByID)
	users.Post("/", middleware.RequirePermission(m.enforcer, "users", "create"), m.handler.Create)
	users.Put("/:id", middleware.RequirePermission(m.enforcer, "users", "update"), m.handler.Update)
	users.Delete("/:id", middleware.RequirePermission(m.enforcer, "users", "delete"), m.handler.Delete)
	users.Post("/:id/activate", middleware.RequirePermission(m.enforcer, "users", "update"), m.handler.Activate)
	users.Post("/:id/deactivate", middleware.RequirePermission(m.enforcer, "users", "update"), m.handler.Deactivate)
}
