package sse

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/sse/handler"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the SSE module
type Module struct {
	handler           *handler.Handler
	authorizer        port.Authorizer
	jwtSecret         string
	jwtPreviousSecret string
}

// NewModule creates a new SSE module
func NewModule(broker port.SSEBroker, authorizer port.Authorizer, jwtSecret, jwtPreviousSecret string) *Module {
	h := handler.NewHandler(broker)

	return &Module{
		handler:           h,
		authorizer:        authorizer,
		jwtSecret:         jwtSecret,
		jwtPreviousSecret: jwtPreviousSecret,
	}
}

// RegisterRoutes registers SSE module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret, m.jwtPreviousSecret))

	sseGroup := router.Group("/sse")

	// SSE subscribe - requires authentication
	sseGroup.Get("/subscribe", authMiddleware, m.handler.Subscribe)

	// Admin-only routes
	sseGroup.Post("/broadcast", authMiddleware, middleware.RequirePermission(m.authorizer, "sse", "broadcast"), m.handler.Broadcast)
	sseGroup.Get("/clients", authMiddleware, middleware.RequirePermission(m.authorizer, "sse", "read"), m.handler.ClientCount)
}
