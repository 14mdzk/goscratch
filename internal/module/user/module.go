package user

import (
	"github.com/14mdzk/goscratch/internal/module/user/handler"
	"github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/module/user/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Module represents the user module
type Module struct {
	handler   *handler.Handler
	jwtSecret string
}

// NewModule creates a new user module
func NewModule(pool *pgxpool.Pool, auditor port.Auditor, jwtSecret string) *Module {
	repo := repository.NewRepository(pool)
	uc := usecase.NewUseCase(repo, auditor)
	h := handler.NewHandler(uc)

	return &Module{
		handler:   h,
		jwtSecret: jwtSecret,
	}
}

// RegisterRoutes registers user module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret))

	users := router.Group("/users")

	// Protected routes
	users.Use(authMiddleware)
	users.Get("/", m.handler.List)
	users.Get("/me", m.handler.GetMe)
	users.Get("/:id", m.handler.GetByID)
	users.Post("/", m.handler.Create)
	users.Put("/:id", m.handler.Update)
	users.Delete("/:id", m.handler.Delete)
	users.Post("/me/password", m.handler.ChangePassword)
	users.Post("/:id/activate", m.handler.Activate)
	users.Post("/:id/deactivate", m.handler.Deactivate)
}
