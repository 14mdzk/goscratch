package http

import (
	"context"
	"fmt"
	"time"

	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Server wraps the Fiber app with configuration
type Server struct {
	app    *fiber.App
	cfg    config.ServerConfig
	logger *logger.Logger
}

// NewServer creates a new HTTP server. The error handler is wired to the
// generic middleware variant so 5xx responses do not echo the original
// error.Error() to the client; production also disables stack traces from
// the recover middleware.
func NewServer(cfg config.ServerConfig, log *logger.Logger, isProduction bool) *Server {
	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
		// Centralized error handler returns a generic message for unknown
		// errors and preserves apperr-typed structured responses.
		ErrorHandler: middleware.ErrorHandler(log),
		// Disable startup message
		DisableStartupMessage: true,
	})

	// Add recovery middleware. Stack traces are gated on non-production to
	// avoid leaking the Go stack to clients via the panic body.
	app.Use(recover.New(recover.Config{
		EnableStackTrace: !isProduction,
	}))

	return &Server{
		app:    app,
		cfg:    cfg,
		logger: log,
	}
}

// App returns the underlying Fiber app for route registration
func (s *Server) App() *fiber.App {
	return s.app
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.logger.Info("Starting HTTP server", "addr", addr)
	return s.app.Listen(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	return s.app.ShutdownWithContext(ctx)
}

// RouteRegistrar is an interface for modules to register their routes
type RouteRegistrar interface {
	RegisterRoutes(router fiber.Router)
}

// RegisterModules registers multiple modules with the server
func (s *Server) RegisterModules(modules ...RouteRegistrar) {
	for _, m := range modules {
		m.RegisterRoutes(s.app)
	}
}

// Group creates a new route group
func (s *Server) Group(prefix string, handlers ...fiber.Handler) fiber.Router {
	return s.app.Group(prefix, handlers...)
}

// Static serves static files
func (s *Server) Static(prefix, root string) {
	s.app.Static(prefix, root)
}
