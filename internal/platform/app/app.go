package app

import (
	"context"
	"time"

	"github.com/14mdzk/goscratch/internal/adapter/audit"
	"github.com/14mdzk/goscratch/internal/adapter/cache"
	casbinadapter "github.com/14mdzk/goscratch/internal/adapter/casbin"
	emailadapter "github.com/14mdzk/goscratch/internal/adapter/email"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/adapter/sse"
	"github.com/14mdzk/goscratch/internal/adapter/storage"
	"github.com/14mdzk/goscratch/internal/module/auth"
	"github.com/14mdzk/goscratch/internal/module/health"
	"github.com/14mdzk/goscratch/internal/module/job"
	"github.com/14mdzk/goscratch/internal/module/role"
	ssemodule "github.com/14mdzk/goscratch/internal/module/sse"
	storagemodule "github.com/14mdzk/goscratch/internal/module/storage"
	"github.com/14mdzk/goscratch/internal/module/user"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/platform/http"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/platform/observability"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
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
	Authorizer     port.Authorizer
	Email          port.EmailSender
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

	// Initialize authorizer (Casbin)
	var authorizer port.Authorizer
	if cfg.Authorization.Enabled {
		log.Info("Initializing Casbin authorization...")
		authorizer, err = casbinadapter.NewAdapter(casbinadapter.Config{
			DatabaseURL: cfg.Database.DSN(),
		})
		if err != nil {
			log.Warn("Failed to initialize Casbin, using no-op authorizer", "error", err)
			authorizer = casbinadapter.NewNoOpAdapter()
		} else {
			log.Info("Casbin authorization initialized successfully")
		}
	} else {
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

	// Initialize HTTP server
	server := http.NewServer(cfg.Server, log)

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

	// Add metrics middleware if enabled
	if cfg.Observability.Metrics.Enabled {
		app.Use(observability.PrometheusMiddleware())
		// Register metrics endpoint
		app.Get("/metrics", observability.MetricsHandler())
		log.Info("Metrics endpoint enabled", "path", "/metrics")
	}

	app.Use(middleware.Logger(log))

	// Add rate limiting middleware if enabled
	if cfg.RateLimit.Enabled {
		rlWindow := time.Duration(cfg.RateLimit.WindowSec) * time.Second
		if rlWindow <= 0 {
			rlWindow = 1 * time.Minute
		}
		rlMax := cfg.RateLimit.Max
		if rlMax <= 0 {
			rlMax = 100
		}
		app.Use(middleware.RateLimit(middleware.RateLimitConfig{
			Max:      rlMax,
			Window:   rlWindow,
			UseRedis: cfg.Redis.Enabled,
		}, cacheAdapter))
		log.Info("Rate limiting enabled", "max", rlMax, "window_sec", cfg.RateLimit.WindowSec)
	}

	// Initialize worker publisher
	publisher := worker.NewPublisher(queueAdapter, cfg.Worker.QueueName, cfg.Worker.Exchange)

	// Register modules
	healthModule := health.NewModule()
	userModule := user.NewModule(pool, auditor, authorizer, cfg.JWT.Secret)
	authModule := auth.NewModule(pool, cacheAdapter, auditor, cfg.JWT)
	roleModule := role.NewModule(authorizer, cfg.JWT.Secret)
	storageModule := storagemodule.NewModule(storageAdapter, cfg.JWT.Secret)
	sseModule := ssemodule.NewModule(sseBroker, authorizer, cfg.JWT.Secret)
	jobModule := job.NewModule(publisher, authorizer, cfg.JWT.Secret)

	server.RegisterModules(healthModule, userModule, authModule, roleModule, storageModule, sseModule, jobModule)

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
		Email:          emailSender,
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
	a.Email.Close()
	a.DB.Close()

	a.Logger.Info("Application shutdown complete")
	return nil
}
