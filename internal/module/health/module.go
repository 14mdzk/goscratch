package health

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// Module represents the health module
type Module struct {
	handler *Handler
}

// NewModule creates a new health module.
// readinessTimeout is passed to the handler as the shared deadline for all
// parallel sub-checks; zero defaults to 2s.
// checkers are the dependency probes run on GET /healthz/ready.
func NewModule(readinessTimeout time.Duration, checkers ...HealthChecker) *Module {
	handler := NewHandler(readinessTimeout, checkers...)
	return &Module{
		handler: handler,
	}
}

// RegisterRoutes registers health module routes.
//
// Canonical paths:
//   - GET /healthz/live   — liveness (process alive, no dependency check)
//   - GET /healthz/ready  — readiness (all dependency sub-checks)
//
// Removed paths (previously /health, /health/ready, /health/live):
// those old paths are no longer registered; callers must migrate to
// /healthz/*. The /health alias was deprecated in v1.2 and removed in v1.3.
func (m *Module) RegisterRoutes(router fiber.Router) {
	router.Get("/healthz/live", m.handler.LivenessCheck)
	router.Get("/healthz/ready", m.handler.ReadinessCheck)
}
