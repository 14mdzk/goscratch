package sse

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/sse/handler"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the SSE module
type Module struct {
	handler        *handler.Handler
	enforcer       port.Enforcer
	authMiddleware fiber.Handler
}

// NewModule creates a new SSE module
func NewModule(broker port.SSEBroker, enforcer port.Enforcer, authMiddleware fiber.Handler) *Module {
	h := handler.NewHandler(broker)

	return &Module{
		handler:        h,
		enforcer:       enforcer,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers SSE module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	sseGroup := router.Group("/sse")

	// SSE subscribe - requires authentication
	sseGroup.Get("/subscribe", m.authMiddleware, m.handler.Subscribe)

	// Admin-only routes
	sseGroup.Post("/broadcast", m.authMiddleware, middleware.RequirePermission(m.enforcer, "sse", "broadcast"), m.handler.Broadcast)
	sseGroup.Get("/clients", m.authMiddleware, middleware.RequirePermission(m.enforcer, "sse", "read"), m.handler.ClientCount)
}
