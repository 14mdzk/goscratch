package user

import (
	"github.com/14mdzk/goscratch/internal/module/user/handler"
	"github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/module/user/usecase"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Module represents the user module
type Module struct {
	handler    *handler.Handler
	authorizer port.Authorizer
	jwtSecret  string
}

// NewModule creates a new user module.
// authRevoker is the auth module's session-revocation interface, injected so
// ChangePassword can terminate all active refresh tokens for the user without
// importing the auth package (avoiding a circular dependency).
func NewModule(pool *pgxpool.Pool, transactor *database.Transactor, auditor port.Auditor, authorizer port.Authorizer, cache port.Cache, jwtSecret string, authRevoker usecase.AuthRevoker) *Module {
	repo := repository.NewRepository(pool)
	uc := usecase.NewUseCase(repo, transactor, cache, authRevoker)
	audited := usecase.NewAuditedUseCase(uc, auditor)
	h := handler.NewHandler(audited)

	return &Module{
		handler:    h,
		authorizer: authorizer,
		jwtSecret:  jwtSecret,
	}
}

// RegisterRoutes registers user module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret))

	users := router.Group("/users")

	// Protected routes - require authentication
	users.Use(authMiddleware)

	// User self-management (no permission required beyond auth)
	users.Get("/me", m.handler.GetMe)
	users.Post("/me/password", m.handler.ChangePassword)

	// User management - require specific permissions
	users.Get("/", middleware.RequirePermission(m.authorizer, "users", "read"), m.handler.List)
	users.Get("/:id", middleware.RequirePermission(m.authorizer, "users", "read"), m.handler.GetByID)
	users.Post("/", middleware.RequirePermission(m.authorizer, "users", "create"), m.handler.Create)
	users.Put("/:id", middleware.RequirePermission(m.authorizer, "users", "update"), m.handler.Update)
	users.Delete("/:id", middleware.RequirePermission(m.authorizer, "users", "delete"), m.handler.Delete)
	users.Post("/:id/activate", middleware.RequirePermission(m.authorizer, "users", "update"), m.handler.Activate)
	users.Post("/:id/deactivate", middleware.RequirePermission(m.authorizer, "users", "update"), m.handler.Deactivate)
}
