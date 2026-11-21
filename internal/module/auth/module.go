package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/auth/handler"
	"github.com/14mdzk/goscratch/internal/module/auth/usecase"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the auth module
type Module struct {
	handler        *handler.Handler
	rateLimiter    port.RateLimiter
	authMiddleware fiber.Handler
	revoker        usecase.Revoker
}

// NewModule creates a new auth module.
// userRepo is the narrow user-lookup interface satisfied by *userrepo.Repository,
// shared with the user module from the composition root.
func NewModule(userRepo usecase.UserRepo, cache port.Cache, rateLimiter port.RateLimiter, auditor port.Auditor, jwtCfg config.JWTConfig, authMiddleware fiber.Handler) *Module {
	uc := usecase.NewUseCase(userRepo, cache, jwtCfg)
	audited := usecase.NewAuditedUseCase(uc, auditor)
	h := handler.NewHandler(audited)

	// uc satisfies usecase.Service (UseCase + Revoker) at compile time, so we
	// expose Revoker directly without a runtime type assertion or audit wrapper.
	return &Module{
		handler:        h,
		rateLimiter:    rateLimiter,
		authMiddleware: authMiddleware,
		revoker:        uc,
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
	// FailClosed: if the rate limiter backend is unavailable the request is
	// rejected rather than allowed through unchecked (block-ship #4 should-fix).
	// The closer is intentionally discarded when rateLimiter is non-nil because
	// the Redis backend's Close is a no-op (the underlying connection is owned
	// and closed by App.Shutdown).
	authRateLimit, _ := middleware.RateLimit(middleware.RateLimitConfig{
		Max:        20,
		Window:     5 * time.Minute,
		UseRedis:   true,
		FailClosed: true,
	}, m.rateLimiter)

	authGroup.Post("/login", authRateLimit, m.handler.Login)
	authGroup.Post("/refresh", authRateLimit, m.handler.Refresh)

	// Logout is authenticated — Auth middleware validates the JWT before the
	// handler runs. The callerID is read from the JWT claims by the handler.
	authGroup.Post("/logout", m.authMiddleware, m.handler.Logout)
}
