package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"time"

	"github.com/casbin/casbin/v3/persist"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/14mdzk/goscratch/internal/adapter/audit"
	"github.com/14mdzk/goscratch/internal/adapter/cache"
	casbinadapter "github.com/14mdzk/goscratch/internal/adapter/casbin"
	emailadapter "github.com/14mdzk/goscratch/internal/adapter/email"
	notificationadapter "github.com/14mdzk/goscratch/internal/adapter/notification"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/adapter/sse"
	"github.com/14mdzk/goscratch/internal/adapter/storage"
	"github.com/14mdzk/goscratch/internal/module/auth"
	"github.com/14mdzk/goscratch/internal/module/docs"
	"github.com/14mdzk/goscratch/internal/module/health"
	"github.com/14mdzk/goscratch/internal/module/job"
	"github.com/14mdzk/goscratch/internal/module/role"
	ssemodule "github.com/14mdzk/goscratch/internal/module/sse"
	storagemodule "github.com/14mdzk/goscratch/internal/module/storage"
	"github.com/14mdzk/goscratch/internal/module/user"
	userrepo "github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/platform/http"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/platform/observability"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/internal/worker/handlers"
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

	// Initialize logger
	logLevel := "info"
	if cfg.IsDevelopment() {
		logLevel = "debug"
	}
	log := logger.New(logger.Config{
		Level:  logLevel,
		Format: "json",
	})

	// Initialize tracing
	var tracerShutdown func(context.Context) error
	if cfg.Observability.Tracing.Enabled {
		log.Info("Initializing OpenTelemetry tracing...", "endpoint", cfg.Observability.Tracing.Endpoint)
		shutdown, err := observability.InitTracer(ctx, observability.TracerConfig{
			ServiceName:    cfg.App.Name,
			ServiceVersion: "1.0.0",
			Environment:    cfg.App.Env,
			Endpoint:       cfg.Observability.Tracing.Endpoint,
			Enabled:        true,
		})
		if err != nil {
			log.Warn("Failed to initialize tracing", "error", err)
		} else {
			tracerShutdown = shutdown
			log.Info("Tracing initialized successfully")
		}
	}

	// Initialize database
	log.Info("Connecting to database...")
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}
	log.Info("Database connected successfully")

	// Initialize cache (Redis or NoOp).
	// WARNING: NoOpCache is a no-op that cannot store or revoke refresh tokens.
	// Running with NoOpCache means:
	//   - Login will be rejected (fail-closed refresh-token gating).
	//   - ChangePassword session revocation will not function.
	// Enable Redis (redis.enabled=true) for any environment that issues JWTs.
	var cacheAdapter port.Cache
	if cfg.Redis.Enabled {
		log.Info("Connecting to Redis...")
		cacheAdapter, err = cache.NewRedisCache(cfg.Redis.Addr(), cfg.Redis.Password, cfg.Redis.DB)
		if err != nil {
			log.Warn("Failed to connect to Redis, using no-op cache", "error", err)
			cacheAdapter = cache.NewNoOpCache()
			log.Warn("SECURITY WARNING: cache is unavailable; login will be rejected and refresh-token revocation will not function")
		} else {
			log.Info("Redis connected successfully")
		}
	} else {
		log.Warn("SECURITY WARNING: Redis is disabled (redis.enabled=false); login will be rejected and refresh-token revocation will not function")
		cacheAdapter = cache.NewNoOpCache()
	}

	// Initialize queue by resolved backend. Durable tiers fail boot when
	// unreachable: silently dropping jobs behind a fallback is worse than
	// not starting (fail fast). The NoOp adapter remains for tests and the
	// health checker's self-skip, but it is never selected here.
	backend, err := cfg.ResolveQueueBackend()
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateQueueWorker(backend); err != nil {
		return nil, err
	}
	var queueAdapter port.Queue
	switch backend {
	case config.QueueBackendRabbitMQ:
		log.Info("Connecting to RabbitMQ...", "backend", backend)
		queueAdapter, err = queue.NewRabbitMQWithOptions(cfg.RabbitMQ.URL, queue.Options{
			PrefetchCount: cfg.RabbitMQ.PrefetchCount,
		})
		if err != nil {
			return nil, fmt.Errorf("rabbitmq backend unreachable: %w", err)
		}
		log.Info("RabbitMQ connected successfully")
	case config.QueueBackendGoroutine:
		log.Info("Using in-memory goroutine queue: zero infrastructure, jobs die with the process", "backend", backend)
		queueAdapter = queue.NewGoroutineQueue()
	default:
		return nil, fmt.Errorf("queue.backend %q is not implemented", backend)
	}

	// Initialize storage
	var storageAdapter port.Storage
	switch cfg.Storage.Mode {
	case "s3":
		log.Info("Initializing S3 storage...")
		storageAdapter, err = storage.NewS3Storage(ctx, storage.S3Config{
			Endpoint:  cfg.Storage.S3.Endpoint,
			Bucket:    cfg.Storage.S3.Bucket,
			Region:    cfg.Storage.S3.Region,
			AccessKey: cfg.Storage.S3.AccessKey,
			SecretKey: cfg.Storage.S3.SecretKey,
		})
		if err != nil {
			log.Warn("Failed to initialize S3 storage, falling back to local", "error", err)
			storageAdapter, _ = storage.NewLocalStorage(cfg.Storage.Local.BasePath, "")
		}
	default:
		log.Info("Initializing local storage...")
		storageAdapter, err = storage.NewLocalStorage(cfg.Storage.Local.BasePath, "")
		if err != nil {
			return nil, err
		}
	}

	// Initialize SSE broker
	var sseBroker port.SSEBroker
	if cfg.SSE.Enabled {
		sseBroker = sse.NewBroker(100)
	} else {
		sseBroker = sse.NewNoOpBroker()
	}

	// Initialize auditor
	var auditor port.Auditor
	if cfg.Audit.Enabled {
		auditor = audit.NewPostgresAuditor(pool)
	} else {
		auditor = audit.NewNoOpAuditor()
	}

	// Initialize authorizer (Casbin).
	// Fail-fast when authorization is explicitly enabled: a transient DB blip at
	// boot must NOT silently open every authenticated endpoint (block-ship #3).
	// The NoOpAdapter is intentionally NOT used as a fallback here.
	var authorizer port.Authorizer
	var watcherRedisClient *redis.Client
	if cfg.Authorization.Enabled {
		log.Info("Initializing Casbin authorization...")
		// Wire the Redis watcher when Redis is available so policy writes
		// propagate to peer pods immediately; otherwise the 5-minute
		// backstop reload remains the convergence path. A watcher failure
		// warns and continues unwatched (mirrors the cache fallback above):
		// losing live propagation must not kill boot.
		var watcher persist.Watcher
		if cfg.Redis.Enabled {
			watcherRedisClient = redis.NewClient(&redis.Options{
				Addr:     cfg.Redis.Addr(),
				Password: cfg.Redis.Password,
				DB:       cfg.Redis.DB,
			})
			rw, werr := casbinadapter.NewRedisWatcher(ctx, watcherRedisClient, "")
			if werr != nil {
				log.Warn("Casbin Redis watcher unavailable; policy propagation falls back to the backstop reload", "error", werr)
				_ = watcherRedisClient.Close()
				watcherRedisClient = nil
			} else {
				log.Info("Casbin Redis watcher subscribed")
				watcher = rw
			}
		}
		authorizer, err = casbinadapter.NewAdapter(casbinadapter.Config{
			DatabaseURL: cfg.Database.DSN(),
			Watcher:     watcher,
		})
		if err != nil {
			return nil, fmt.Errorf("authorization enabled but Casbin init failed: %w", err)
		}
		log.Info("Casbin authorization initialized successfully")
	} else {
		// Authorization is explicitly disabled — use NoOp (e.g. local dev without DB).
		log.Warn("Authorization is disabled (authorization.enabled=false); all permission checks are bypassed")
		authorizer = casbinadapter.NewNoOpAdapter()
	}

	// Initialize email sender
	var emailSender port.EmailSender
	if cfg.Email.Enabled {
		log.Info("Initializing SMTP email sender...")
		emailSender = emailadapter.NewSMTPSender(emailadapter.SMTPConfig{
			Host:     cfg.Email.Host,
			Port:     cfg.Email.Port,
			Username: cfg.Email.Username,
			Password: cfg.Email.Password,
			From:     cfg.Email.From,
		})
	} else {
		emailSender = emailadapter.NewNoOpSender(log)
	}

	// Initialize notification sender
	var notificationSender port.NotificationSender
	if cfg.Notification.Enabled {
		log.Info("Initializing webhook notification sender...")
		notificationSender = notificationadapter.NewWebhookSender(notificationadapter.WebhookConfig{
			URL: cfg.Notification.WebhookURL,
		})
	} else {
		notificationSender = notificationadapter.NewNoOpSender(log)
	}

	// Initialize HTTP server
	server := http.NewServer(cfg.Server, log, cfg.IsProduction())

	// Apply middleware
	app := server.App()
	app.Use(middleware.RequestID())
	app.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
		IsProduction: cfg.IsProduction(),
	}))

	// Config-driven CORS
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
	app.Use(middleware.CORS(corsConfig))

	// Add tracing middleware if enabled
	if cfg.Observability.Tracing.Enabled {
		app.Use(observability.TracingMiddleware(cfg.App.Name))
	}

	// Add metrics middleware if enabled. The middleware records counters on
	// the public listener; the /metrics scrape endpoint is bound to a
	// separate localhost-only listener so the public surface does not leak
	// process internals to unauthenticated callers.
	var metricsServer *nethttp.Server
	if cfg.Observability.Metrics.Enabled {
		app.Use(observability.PrometheusMiddleware())

		mux := nethttp.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsAddr := fmt.Sprintf("127.0.0.1:%d", cfg.Observability.Metrics.Port)
		metricsServer = &nethttp.Server{
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
	}

	app.Use(middleware.Logger(log))

	// Add rate limiting middleware if enabled
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
		}, cacheAdapter)
		rateLimitCloser = rlCloser
		app.Use(rlHandler)
		log.Info("Rate limiting enabled", "max", rlMax, "window_sec", cfg.RateLimit.WindowSec)
	}

	// Initialize worker publisher
	publisher := worker.NewPublisher(queueAdapter, cfg.Worker.QueueName, cfg.Worker.Exchange)

	// Initialize transactor
	transactor := database.NewTransactor(pool)

	// Register modules
	docsModule := docs.NewModule()

	// Wire health checkers. Postgres is always checked. Cache and queue checkers
	// are included unconditionally — their implementations self-skip (return nil)
	// when the underlying adapter is a NoOp. Casbin is included when authorization
	// is enabled (real adapter); the NoOp adapter is self-skipping too.
	healthCheckers := []health.HealthChecker{
		health.NewPostgresChecker(pool),
		health.NewCacheChecker(cacheAdapter),
		health.NewQueueChecker(queueAdapter),
		health.NewAuthzChecker(authorizer),
	}
	healthModule := health.NewModule(cfg.Health.ReadinessTimeout(), healthCheckers...)

	// Single shared user-repo instance wired into both the user module and the
	// auth module so both use the same *pgxpool.Pool connection rather than
	// opening a second one (audit finding: auth/module.go:20 instantiated its
	// own userrepo.Repository).
	sharedUserRepo := userrepo.NewRepository(pool)

	// Auth module is constructed first so its Revoker can be injected into the
	// user module (ChangePassword must revoke auth sessions cross-module).
	authModule := auth.NewModule(sharedUserRepo, cacheAdapter, auditor, cfg.JWT)
	userModule := user.NewModule(pool, transactor, auditor, authorizer, cacheAdapter, cfg.JWT.Secret, authModule.Revoker())
	roleModule := role.NewModule(authorizer, cfg.JWT.Secret)
	storageModule := storagemodule.NewModule(storageAdapter, auditor, cfg.JWT.Secret)
	sseModule := ssemodule.NewModule(sseBroker, authorizer, cfg.JWT.Secret)
	jobModule := job.NewModule(publisher, auditor, authorizer, cfg.JWT.Secret)

	server.RegisterModules(docsModule, healthModule, userModule, authModule, roleModule, storageModule, sseModule, jobModule)

	// Start the in-process worker's dependencies: with the goroutine
	// backend, consumption lives in this process. Handlers mirror
	// cmd/worker so both paths execute the same job types.
	var inProcessWorker *worker.Worker
	if backend == config.QueueBackendGoroutine {
		concurrency := cfg.Worker.Concurrency
		if concurrency <= 0 {
			concurrency = 2
		}
		inProcessWorker = worker.New(queueAdapter, log, worker.Config{
			QueueName:   cfg.Worker.QueueName,
			Exchange:    cfg.Worker.Exchange,
			Concurrency: concurrency,
		})
		inProcessWorker.RegisterHandler(handlers.NewEmailHandler(log, emailSender))
		inProcessWorker.RegisterHandler(handlers.NewNotificationHandler(log, notificationSender))
		inProcessWorker.RegisterHandler(handlers.NewAuditCleanupHandler(pool, log))
	}

	// Start the authorizer lifecycle (backstop reload tick + watcher subscription).
	// Failure here is fatal: a half-initialised authorizer would silently drop
	// policy updates from peer pods.
	if err := authorizer.Start(ctx); err != nil {
		return nil, fmt.Errorf("authorizer start: %w", err)
	}

	return &App{
		Config:             cfg,
		Logger:             log,
		DB:                 pool,
		Server:             server,
		Cache:              cacheAdapter,
		Queue:              queueAdapter,
		Storage:            storageAdapter,
		SSE:                sseBroker,
		Auditor:            auditor,
		Authorizer:         authorizer,
		Email:              emailSender,
		Notification:       notificationSender,
		metricsServer:      metricsServer,
		tracerShutdown:     tracerShutdown,
		rateLimitCloser:    rateLimitCloser,
		inProcessWorker:    inProcessWorker,
		watcherRedisClient: watcherRedisClient,
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
