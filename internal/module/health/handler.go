package health

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/14mdzk/goscratch/pkg/response"
)

// Handler handles health check requests
type Handler struct{}

// NewHandler creates a new health handler
func NewHandler() *Handler {
	return &Handler{}
}

// HealthCheckResponse represents the health check response
type HealthCheckResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Version   string            `json:"version,omitempty"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// HealthCheck performs a basic health check
func (h *Handler) HealthCheck(c *fiber.Ctx) error {
	return response.Success(c, HealthCheckResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// ReadinessCheck checks if the application is ready to serve traffic
func (h *Handler) ReadinessCheck(c *fiber.Ctx) error {
	// TODO: Add actual readiness checks (db connection, etc.)
	checks := map[string]string{
		"database": "ok",
		"cache":    "ok",
	}

	return response.Success(c, HealthCheckResponse{
		Status:    "ready",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}

// LivenessCheck checks if the application is alive
func (h *Handler) LivenessCheck(c *fiber.Ctx) error {
	return response.Success(c, HealthCheckResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
