package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/14mdzk/goscratch/internal/module/auth/handler"
	"github.com/14mdzk/goscratch/internal/module/auth/usecase"
	userrepo "github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the auth module
type Module struct {
	handler *handler.Handler
}

// NewModule creates a new auth module
func NewModule(pool *pgxpool.Pool, cache port.Cache, auditor port.Auditor, jwtCfg config.JWTConfig) *Module {
	userRepo := userrepo.NewRepository(pool)
	uc := usecase.NewUseCase(userRepo, cache, auditor, jwtCfg)
	h := handler.NewHandler(uc)

	return &Module{
		handler: h,
	}
}

// RegisterRoutes registers auth module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	auth := router.Group("/auth")

	auth.Post("/login", m.handler.Login)
	auth.Post("/refresh", m.handler.Refresh)
	auth.Post("/logout", m.handler.Logout)
}
