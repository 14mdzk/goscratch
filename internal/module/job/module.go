package job

import (
	"github.com/14mdzk/goscratch/internal/module/job/handler"
	"github.com/14mdzk/goscratch/internal/module/job/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/gofiber/fiber/v2"
)

// Module represents the job module
type Module struct {
	handler    *handler.Handler
	authorizer port.Authorizer
	jwtSecret  string
}

// NewModule creates a new job module
func NewModule(publisher *worker.Publisher, authorizer port.Authorizer, jwtSecret string) *Module {
	uc := usecase.NewUseCase(publisher)
	h := handler.NewHandler(uc)

	return &Module{
		handler:    h,
		authorizer: authorizer,
		jwtSecret:  jwtSecret,
	}
}

// RegisterRoutes registers job module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret))

	jobs := router.Group("/jobs")

	// All job routes require authentication + admin role
	jobs.Use(authMiddleware)
	jobs.Use(middleware.RequireRole(m.authorizer, "admin"))

	jobs.Post("/dispatch", m.handler.Dispatch)
	jobs.Get("/types", m.handler.ListTypes)
}
