package app

import (
	"context"

	"github.com/14mdzk/goscratch/internal/adapter/audit"
	"github.com/14mdzk/goscratch/internal/adapter/cache"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/adapter/sse"
	"github.com/14mdzk/goscratch/internal/adapter/storage"
	"github.com/14mdzk/goscratch/internal/module/auth"
	"github.com/14mdzk/goscratch/internal/module/health"
	"github.com/14mdzk/goscratch/internal/module/user"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/platform/http"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/platform/observability"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// App holds all application dependencies
type App struct {
	Config         *config.Config
	Logger         *logger.Logger
	DB             *pgxpool.Pool
	Server         *http.Server
	Cache          port.Cache
	Queue          port.Queue
	Storage        port.Storage
	SSE            port.SSEBroker
	Auditor        port.Auditor
	tracerShutdown func(context.Context) error
}

// New creates a new App instance with all dependencies
func New(ctx context.Context, cfg *config.Config) (*App, error) {
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

	// Initialize cache (Redis or NoOp)
	var cacheAdapter port.Cache
	if cfg.Redis.Enabled {
		log.Info("Connecting to Redis...")
		cacheAdapter, err = cache.NewRedisCache(cfg.Redis.Addr(), cfg.Redis.Password, cfg.Redis.DB)
		if err != nil {
			log.Warn("Failed to connect to Redis, using no-op cache", "error", err)
			cacheAdapter = cache.NewNoOpCache()
		} else {
			log.Info("Redis connected successfully")
		}
	} else {
		cacheAdapter = cache.NewNoOpCache()
	}

	// Initialize queue (RabbitMQ or NoOp)
	var queueAdapter port.Queue
	if cfg.RabbitMQ.Enabled {
		log.Info("Connecting to RabbitMQ...")
		queueAdapter, err = queue.NewRabbitMQ(cfg.RabbitMQ.URL)
		if err != nil {
			log.Warn("Failed to connect to RabbitMQ, using no-op queue", "error", err)
			queueAdapter = queue.NewNoOpQueue()
		} else {
			log.Info("RabbitMQ connected successfully")
		}
	} else {
		queueAdapter = queue.NewNoOpQueue()
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

	// Initialize HTTP server
	server := http.NewServer(cfg.Server, log)

	// Apply middleware
	app := server.App()
	app.Use(middleware.RequestID())
	app.Use(middleware.CORS(middleware.DefaultCORSConfig()))

	// Add tracing middleware if enabled
	if cfg.Observability.Tracing.Enabled {
		app.Use(observability.TracingMiddleware(cfg.App.Name))
	}

	// Add metrics middleware if enabled
	if cfg.Observability.Metrics.Enabled {
		app.Use(observability.PrometheusMiddleware())
		// Register metrics endpoint
		app.Get("/metrics", observability.MetricsHandler())
		log.Info("Metrics endpoint enabled", "path", "/metrics")
	}

	app.Use(middleware.Logger(log))

	// Register modules
	healthModule := health.NewModule()
	userModule := user.NewModule(pool, auditor, cfg.JWT.Secret)
	authModule := auth.NewModule(pool, cacheAdapter, auditor, cfg.JWT)

	server.RegisterModules(healthModule, userModule, authModule)

	return &App{
		Config:         cfg,
		Logger:         log,
		DB:             pool,
		Server:         server,
		Cache:          cacheAdapter,
		Queue:          queueAdapter,
		Storage:        storageAdapter,
		SSE:            sseBroker,
		Auditor:        auditor,
		tracerShutdown: tracerShutdown,
	}, nil
}

// Start starts the application
func (a *App) Start() error {
	a.Logger.Info("Starting application", "port", a.Config.Server.Port)
	return a.Server.Start()
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown(ctx context.Context) error {
	a.Logger.Info("Shutting down application...")

	// Close server
	if err := a.Server.Shutdown(ctx); err != nil {
		a.Logger.Error("Failed to shutdown server", "error", err)
	}

	// Shutdown tracer
	if a.tracerShutdown != nil {
		if err := a.tracerShutdown(ctx); err != nil {
			a.Logger.Error("Failed to shutdown tracer", "error", err)
		}
	}

	// Close resources
	a.Cache.Close()
	a.Queue.Close()
	a.Storage.Close()
	a.SSE.Close()
	a.Auditor.Close()
	a.DB.Close()

	a.Logger.Info("Application shutdown complete")
	return nil
}
