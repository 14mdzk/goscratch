package storage

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/storage/handler"
	"github.com/14mdzk/goscratch/internal/module/storage/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
)

// Module represents the storage module
type Module struct {
	handler           *handler.Handler
	jwtSecret         string
	jwtPreviousSecret string
}

// NewModule creates a new storage module
func NewModule(storage port.Storage, auditor port.Auditor, jwtSecret, jwtPreviousSecret string) *Module {
	uc := usecase.NewUseCase(storage, nil)
	var ucIface usecase.UseCase = uc
	if auditor != nil {
		ucIface = usecase.NewAuditedUseCase(ucIface, auditor)
	}
	h := handler.NewHandler(ucIface)

	return &Module{
		handler:           h,
		jwtSecret:         jwtSecret,
		jwtPreviousSecret: jwtPreviousSecret,
	}
}

// RegisterRoutes registers storage module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	authMiddleware := middleware.Auth(middleware.DefaultAuthConfig(m.jwtSecret, m.jwtPreviousSecret))

	files := router.Group("/files")

	// All routes require authentication
	files.Use(authMiddleware)

	files.Post("/upload", m.handler.Upload)
	files.Get("/", m.handler.List)
	files.Get("/url/*", m.handler.GetURL)
	files.Get("/download/*", m.handler.Download)
	files.Delete("/*", m.handler.Delete)
}
