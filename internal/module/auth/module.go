package auth

import (
	"time"

	"github.com/14mdzk/goscratch/internal/module/auth/handler"
	"github.com/14mdzk/goscratch/internal/module/auth/usecase"
	userrepo "github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Module represents the auth module
type Module struct {
	handler   *handler.Handler
	jwtSecret string
	cache     port.Cache
	revoker   usecase.Revoker
}

// NewModule creates a new auth module
func NewModule(pool *pgxpool.Pool, cache port.Cache, auditor port.Auditor, jwtCfg config.JWTConfig) *Module {
	userRepo := userrepo.NewRepository(pool)
	uc := usecase.NewUseCase(userRepo, cache, jwtCfg)
	audited := usecase.NewAuditedUseCase(uc, auditor)
	h := handler.NewHandler(audited)

	// Expose the concrete usecase as a Revoker so other modules (user) can call
	// RevokeAllForUser without going through the audit decorator.
	return &Module{
		handler:   h,
		jwtSecret: jwtCfg.Secret,
		cache:     cache,
		revoker:   uc.(usecase.Revoker),
	}
}

// Revoker returns the auth module's session-revocation interface.
// The user module uses this to revoke all refresh tokens on ChangePassword.
func (m *Module) Revoker() usecase.Revoker {
	return m.revoker
}

// RegisterRoutes registers auth module routes.
//
//   - /login and /refresh are public but protected by a tight per-IP rate limit
//     (20 req / 5 min, fail-closed) to throttle credential-stuffing attempts.
//   - /logout requires a valid JWT (Auth middleware) so an unauthenticated caller
//     cannot hit the endpoint at all (block-ship #5).
func (m *Module) RegisterRoutes(router fiber.Router) {
	authGroup := router.Group("/auth")

	// Tight rate limit applied only to the login and refresh endpoints.
	// FailClosed: if the cache backend is unavailable the request is rejected
	// rather than allowed through unchecked (block-ship #4 should-fix).
	authRateLimit := middleware.RateLimit(middleware.RateLimitConfig{
		Max:        20,
		Window:     5 * time.Minute,
		UseRedis:   true,
		FailClosed: true,
	}, m.cache)

	authGroup.Post("/login", authRateLimit, m.handler.Login)
	authGroup.Post("/refresh", authRateLimit, m.handler.Refresh)

	// Logout is authenticated — Auth middleware validates the JWT before the
	// handler runs. The callerID is read from the JWT claims by the handler.
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret))
	authGroup.Post("/logout", authMiddleware, m.handler.Logout)
}
