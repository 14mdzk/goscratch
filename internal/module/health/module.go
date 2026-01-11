package health

import (
	"github.com/gofiber/fiber/v2"
)

// Module represents the health module
type Module struct {
	handler *Handler
}

// NewModule creates a new health module
func NewModule() *Module {
	handler := NewHandler()
	return &Module{
		handler: handler,
	}
}

// RegisterRoutes registers health module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	router.Get("/health", m.handler.HealthCheck)
	router.Get("/health/ready", m.handler.ReadinessCheck)
	router.Get("/health/live", m.handler.LivenessCheck)
}
