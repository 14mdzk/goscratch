package audit

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/audit/handler"
	"github.com/14mdzk/goscratch/internal/module/audit/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the audit module
type Module struct {
	handler           *handler.Handler
	authorizer        port.Authorizer
	jwtSecret         string
	jwtPreviousSecret string
}

// NewModule creates a new audit module
func NewModule(auditor port.Auditor, authorizer port.Authorizer, jwtSecret, jwtPreviousSecret string) *Module {
	uc := usecase.NewUseCase(auditor)
	h := handler.NewHandler(uc)

	return &Module{
		handler:           h,
		authorizer:        authorizer,
		jwtSecret:         jwtSecret,
		jwtPreviousSecret: jwtPreviousSecret,
	}
}

// RegisterRoutes registers audit module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret, m.jwtPreviousSecret))
	audit := router.Group("/audit", authMiddleware)
	audit.Get("/logs", middleware.RequirePermission(m.authorizer, "audit", "read"), m.handler.List)
}
