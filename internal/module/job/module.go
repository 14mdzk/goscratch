package job

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/job/handler"
	"github.com/14mdzk/goscratch/internal/module/job/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
)

// Module represents the job module
type Module struct {
	handler        *handler.Handler
	enforcer       port.Enforcer
	authMiddleware fiber.Handler
}

// NewModule creates a new job module
func NewModule(publisher *worker.Publisher, auditor port.Auditor, enforcer port.Enforcer, authMiddleware fiber.Handler) *Module {
	uc := usecase.NewUseCase(publisher)
	var ucIface usecase.UseCase = uc
	if auditor != nil {
		ucIface = usecase.NewAuditedUseCase(ucIface, auditor)
	}
	h := handler.NewHandler(ucIface)

	return &Module{
		handler:        h,
		enforcer:       enforcer,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers job module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	jobs := router.Group("/jobs")

	// All job routes require authentication + admin role
	jobs.Use(m.authMiddleware)
	jobs.Use(middleware.RequireRole(m.enforcer, "admin"))

	jobs.Post("/dispatch", m.handler.Dispatch)
	jobs.Get("/types", m.handler.ListTypes)
}
