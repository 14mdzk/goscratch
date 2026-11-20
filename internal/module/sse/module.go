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
	authorizer     port.Authorizer
	authMiddleware fiber.Handler
}

// NewModule creates a new SSE module
func NewModule(broker port.SSEBroker, authorizer port.Authorizer, authMiddleware fiber.Handler) *Module {
	h := handler.NewHandler(broker)

	return &Module{
		handler:        h,
		authorizer:     authorizer,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers SSE module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	sseGroup := router.Group("/sse")

	// SSE subscribe - requires authentication
	sseGroup.Get("/subscribe", m.authMiddleware, m.handler.Subscribe)

	// Admin-only routes
	sseGroup.Post("/broadcast", m.authMiddleware, middleware.RequirePermission(m.authorizer, "sse", "broadcast"), m.handler.Broadcast)
	sseGroup.Get("/clients", m.authMiddleware, middleware.RequirePermission(m.authorizer, "sse", "read"), m.handler.ClientCount)
}
