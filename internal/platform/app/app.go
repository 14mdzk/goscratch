package app

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/http"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// App holds all application dependencies
type App struct {
	Config          *config.Config
	Logger          *logger.Logger
	DB              *pgxpool.Pool
	Server          *http.Server
	Cache           port.Cache
	Queue           port.Queue
	Storage         port.Storage
	SSE             port.SSEBroker
	Auditor         port.Auditor
	Authorizer      port.Authorizer
	Email           port.EmailSender
	Notification    port.NotificationSender
	metricsServer   *nethttp.Server
	tracerShutdown  func(context.Context) error
	rateLimitCloser io.Closer
	// inProcessWorker runs job consumption inside the API process. Non-nil
	// only when queue.backend=goroutine with worker.in_process=true.
	inProcessWorker *worker.Worker
	// watcherRedisClient is the dedicated Redis connection of the Casbin
	// watcher. Non-nil only when the watcher is wired (Redis + authorization
	// both enabled and subscribe succeeded); closed at shutdown.
	watcherRedisClient *redis.Client
}

// New creates a new App instance with all dependencies
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	// Validate secure-defaults invariants before constructing any adapter.
	// A bad JWT secret (placeholder or too short) means every issued token
	// is forgeable, so we hard-fail rather than silently boot.
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	logLevel := "info"
	if cfg.IsDevelopment() {
		logLevel = "debug"
	}
	log := logger.New(logger.Config{
		Level:  logLevel,
		Format: "json",
	})

	infra, err := initInfra(ctx, cfg, log)
	if err != nil {
		return nil, err
	}

	httpLayer := initHTTP(cfg, log, infra.cache)
	inProcessWorker := initModules(cfg, log, infra, httpLayer.server)

	// Start the authorizer lifecycle (backstop reload tick + watcher subscription).
	// Failure here is fatal: a half-initialised authorizer would silently drop
	// policy updates from peer pods.
	if err := infra.authorizer.Start(ctx); err != nil {
		return nil, fmt.Errorf("authorizer start: %w", err)
	}

	return &App{
		Config:             cfg,
		Logger:             log,
		DB:                 infra.db,
		Server:             httpLayer.server,
		Cache:              infra.cache,
		Queue:              infra.queue,
		Storage:            infra.storage,
		SSE:                infra.sse,
		Auditor:            infra.auditor,
		Authorizer:         infra.authorizer,
		Email:              infra.email,
		Notification:       infra.notification,
		metricsServer:      httpLayer.metricsServer,
		tracerShutdown:     infra.tracerShutdown,
		rateLimitCloser:    httpLayer.rateLimitCloser,
		inProcessWorker:    inProcessWorker,
		watcherRedisClient: infra.watcherRedisClient,
	}, nil
}

// Start starts the application. With the goroutine backend the in-process
// worker starts before the HTTP listener so no request can publish a job
// nothing consumes; a worker start failure aborts boot.
func (a *App) Start() error {
	if a.inProcessWorker != nil {
		if err := a.inProcessWorker.Start(); err != nil {
			return fmt.Errorf("in-process worker start: %w", err)
		}
	}
	a.Logger.Info("Starting application", "port", a.Config.Server.Port)
	return a.Server.Start()
}

// defaultShutdownBudget is the fallback total budget when the parent context
// has no deadline. The runPhase fractions are calibrated against this.
const defaultShutdownBudget = 30 * time.Second

// Shutdown gracefully shuts down the application in deterministic phases.
//
// Phase order is chosen so that downstream emitters drain before their sinks
// close: HTTP requests finish (server), policy bus quiets (authorizer), SSE
// streams disconnect cleanly, then DB closes, then the tracer is stopped LAST
// so spans emitted by prior phases still flush. Each phase gets a fraction of
// the total deadline budget so a slow first phase cannot starve later ones.
func (a *App) Shutdown(ctx context.Context) error {
	a.Logger.Info("Shutting down application...")

	totalBudget := defaultShutdownBudget
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 {
			totalBudget = remaining
		}
	}

	runPhase := func(name string, fraction float64, fn func(context.Context) error) {
		budget := time.Duration(float64(totalBudget) * fraction)
		if budget <= 0 {
			budget = 100 * time.Millisecond
		}
		phaseCtx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		start := time.Now()
		if err := fn(phaseCtx); err != nil {
			a.Logger.Error("shutdown phase failed", "phase", name, "error", err)
		}
		a.Logger.Info("shutdown phase complete",
			"phase", name,
			"duration_ms", time.Since(start).Milliseconds(),
			"budget_ms", budget.Milliseconds(),
		)
	}

	// 1. HTTP server — drain in-flight requests. Gets the largest slice
	//    because clients may be mid-stream.
	runPhase("http_server", 0.30, func(ctx context.Context) error {
		if a.Server == nil {
			return nil
		}
		return a.Server.Shutdown(ctx)
	})

	// 2. In-process worker — drain queued jobs after the listener stops
	//    taking new requests (no-op unless the goroutine backend runs
	//    in-process).
	runPhase("worker", 0.15, func(ctx context.Context) error {
		if a.inProcessWorker == nil {
			return nil
		}
		return a.inProcessWorker.Shutdown(ctx)
	})

	// 3. Metrics listener — internal, fast.
	runPhase("metrics", 0.05, func(ctx context.Context) error {
		if a.metricsServer == nil {
			return nil
		}
		return a.metricsServer.Shutdown(ctx)
	})

	// 3. SSE broker — close subscriber channels so range loops exit before
	//    we yank their downstream adapters.
	runPhase("sse", 0.05, func(_ context.Context) error {
		if a.SSE == nil {
			return nil
		}
		return a.SSE.Close()
	})

	// 4. Authorizer — Casbin DB handle + watcher goroutine + backstop ticker.
	//    Closed before the main DB pool so no in-flight policy load races
	//    against a shutting-down pool.
	runPhase("authorizer", 0.10, func(_ context.Context) error {
		if a.Authorizer == nil {
			return nil
		}
		return a.Authorizer.Close()
	})

	// 5. Bulk adapters. None of these accept a ctx; we run them under the
	//    phase budget so a hung Close cannot stall the rest.
	runPhase("adapters", 0.10, func(_ context.Context) error {
		if a.rateLimitCloser != nil {
			if err := a.rateLimitCloser.Close(); err != nil {
				a.Logger.Error("rate limit backend close", "error", err)
			}
		}
		if a.Cache != nil {
			_ = a.Cache.Close()
		}
		if a.watcherRedisClient != nil {
			_ = a.watcherRedisClient.Close()
		}
		if a.Queue != nil {
			_ = a.Queue.Close()
		}
		if a.Storage != nil {
			_ = a.Storage.Close()
		}
		if a.Auditor != nil {
			_ = a.Auditor.Close()
		}
		if a.Email != nil {
			_ = a.Email.Close()
		}
		if a.Notification != nil {
			_ = a.Notification.Close()
		}
		return nil
	})

	// 6. DB — last database-using thing closed.
	runPhase("database", 0.10, func(_ context.Context) error {
		if a.DB != nil {
			a.DB.Close()
		}
		return nil
	})

	// 7. Tracer — LAST so spans from every prior phase flush through it.
	runPhase("tracer", 0.15, func(ctx context.Context) error {
		if a.tracerShutdown == nil {
			return nil
		}
		return a.tracerShutdown(ctx)
	})

	a.Logger.Info("Application shutdown complete")
	return nil
}
