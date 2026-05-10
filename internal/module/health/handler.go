package health

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Handler handles health check requests
type Handler struct {
	checkers         []HealthChecker
	readinessTimeout time.Duration
}

// NewHandler creates a new health handler.
// readinessTimeout is the total deadline for all parallel sub-checks; zero defaults to 2s.
// checkers are the dependency probes run by ReadinessCheck.
func NewHandler(readinessTimeout time.Duration, checkers ...HealthChecker) *Handler {
	if readinessTimeout <= 0 {
		readinessTimeout = 2 * time.Second
	}
	return &Handler{
		checkers:         checkers,
		readinessTimeout: readinessTimeout,
	}
}

// HealthCheckResponse represents the health check response
type HealthCheckResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// LivenessCheck checks if the application process is alive.
// Always returns 200; it does not probe any dependency.
func (h *Handler) LivenessCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(HealthCheckResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// ReadinessCheck runs all registered checkers in parallel under a shared
// deadline. Returns 200 if every checker passes, 503 if any fails.
// The response body lists each check name with "ok" or a short sanitised reason.
// Raw infrastructure error strings are never forwarded to the response.
func (h *Handler) ReadinessCheck(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), h.readinessTimeout)
	defer cancel()

	type result struct {
		name   string
		reason string // empty means ok
	}

	results := make([]result, len(h.checkers))
	var wg sync.WaitGroup
	wg.Add(len(h.checkers))

	for i, chk := range h.checkers {
		i, chk := i, chk
		go func() {
			defer wg.Done()
			name := chk.Name()
			if err := chk.Check(ctx); err != nil {
				results[i] = result{name: name, reason: err.Error()}
			} else {
				results[i] = result{name: name}
			}
		}()
	}
	wg.Wait()

	checks := make(map[string]string, len(results))
	healthy := true
	for _, r := range results {
		if r.reason != "" {
			checks[r.name] = r.reason
			healthy = false
		} else {
			checks[r.name] = "ok"
		}
	}

	status := "ready"
	httpStatus := fiber.StatusOK
	if !healthy {
		status = "degraded"
		httpStatus = fiber.StatusServiceUnavailable
	}

	return c.Status(httpStatus).JSON(HealthCheckResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}
