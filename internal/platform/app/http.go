package app

import (
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/http"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/platform/observability"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// httpDeps bundles the HTTP server and its supporting listeners/closers.
type httpDeps struct {
	server          *http.Server
	metricsServer   *nethttp.Server
	rateLimitCloser io.Closer
}

func initHTTP(cfg *config.Config, log *logger.Logger, rateLimiter port.RateLimiter) *httpDeps {
	server := http.NewServer(cfg.Server, log, cfg.IsProduction())
	app := server.App()

	app.Use(middleware.RequestID())
	app.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
		IsProduction: cfg.IsProduction(),
	}))
	app.Use(middleware.CORS(buildCORSConfig(cfg, log)))

	if cfg.Observability.Tracing.Enabled {
		app.Use(observability.TracingMiddleware(cfg.App.Name))
	}

	var metricsServer *nethttp.Server
	if cfg.Observability.Metrics.Enabled {
		app.Use(observability.PrometheusMiddleware())
		metricsServer = startMetricsServer(cfg, log)
	}

	app.Use(middleware.Logger(log))

	var rateLimitCloser io.Closer
	if cfg.RateLimit.Enabled {
		rlWindow := time.Duration(cfg.RateLimit.WindowSec) * time.Second
		if rlWindow <= 0 {
			rlWindow = 1 * time.Minute
		}
		rlMax := cfg.RateLimit.Max
		if rlMax <= 0 {
			rlMax = 100
		}
		rlHandler, rlCloser := middleware.RateLimit(middleware.RateLimitConfig{
			Max:      rlMax,
			Window:   rlWindow,
			UseRedis: cfg.Redis.Enabled,
		}, rateLimiter)
		rateLimitCloser = rlCloser
		app.Use(rlHandler)
		log.Info("Rate limiting enabled", "max", rlMax, "window_sec", cfg.RateLimit.WindowSec)
	}

	return &httpDeps{
		server:          server,
		metricsServer:   metricsServer,
		rateLimitCloser: rateLimitCloser,
	}
}

func buildCORSConfig(cfg *config.Config, log *logger.Logger) middleware.CORSConfig {
	corsConfig := middleware.DefaultCORSConfig()
	if cfg.CORS.AllowOrigins != "" {
		corsConfig.AllowOrigins = cfg.CORS.AllowOrigins
	} else if cfg.IsDevelopment() {
		corsConfig.AllowOrigins = "*"
	}
	if cfg.CORS.AllowMethods != "" {
		corsConfig.AllowMethods = cfg.CORS.AllowMethods
	}
	if cfg.CORS.AllowHeaders != "" {
		corsConfig.AllowHeaders = cfg.CORS.AllowHeaders
	}
	corsConfig.AllowCredentials = cfg.CORS.AllowCredentials
	if cfg.IsProduction() && corsConfig.AllowOrigins == "*" {
		log.Warn("CORS wildcard origin '*' is used in production - this is insecure, set explicit origins via CORS_ALLOW_ORIGINS")
	}
	return corsConfig
}

// startMetricsServer binds the /metrics Prometheus scrape endpoint to a
// separate localhost-only listener so the public surface does not leak
// process internals to unauthenticated callers.
func startMetricsServer(cfg *config.Config, log *logger.Logger) *nethttp.Server {
	mux := nethttp.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	metricsAddr := fmt.Sprintf("127.0.0.1:%d", cfg.Observability.Metrics.Port)
	metricsServer := &nethttp.Server{
		Addr:              metricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("Metrics endpoint enabled", "addr", metricsAddr, "path", "/metrics")
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			log.Error("Metrics server failed", "error", err)
		}
	}()
	return metricsServer
}
