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
	handler        *handler.Handler
	authorizer     port.Authorizer
	authMiddleware fiber.Handler
}

// NewModule creates a new audit module
func NewModule(auditor port.Auditor, authorizer port.Authorizer, authMiddleware fiber.Handler) *Module {
	uc := usecase.NewUseCase(auditor)
	h := handler.NewHandler(uc)

	return &Module{
		handler:        h,
		authorizer:     authorizer,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers audit module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	audit := router.Group("/audit", m.authMiddleware)
	audit.Get("/logs", middleware.RequirePermission(m.authorizer, "audit", "read"), m.handler.List)
}
