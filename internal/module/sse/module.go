package sse

import (
	"github.com/14mdzk/goscratch/internal/module/sse/handler"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/gofiber/fiber/v2"
)

// Module represents the SSE module
type Module struct {
	handler    *handler.Handler
	authorizer port.Authorizer
	jwtSecret  string
}

// NewModule creates a new SSE module
func NewModule(broker port.SSEBroker, authorizer port.Authorizer, jwtSecret string) *Module {
	h := handler.NewHandler(broker)

	return &Module{
		handler:    h,
		authorizer: authorizer,
		jwtSecret:  jwtSecret,
	}
}

// RegisterRoutes registers SSE module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret))

	sseGroup := router.Group("/sse")

	// SSE subscribe - requires authentication
	sseGroup.Get("/subscribe", authMiddleware, m.handler.Subscribe)

	// Admin-only routes
	sseGroup.Post("/broadcast", authMiddleware, middleware.RequirePermission(m.authorizer, "sse", "broadcast"), m.handler.Broadcast)
	sseGroup.Get("/clients", authMiddleware, middleware.RequirePermission(m.authorizer, "sse", "read"), m.handler.ClientCount)
}
