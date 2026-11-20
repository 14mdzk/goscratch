package app

import (
	"context"
	"fmt"

	"github.com/casbin/casbin/v3/persist"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/14mdzk/goscratch/internal/adapter/audit"
	"github.com/14mdzk/goscratch/internal/adapter/cache"
	casbinadapter "github.com/14mdzk/goscratch/internal/adapter/casbin"
	emailadapter "github.com/14mdzk/goscratch/internal/adapter/email"
	notificationadapter "github.com/14mdzk/goscratch/internal/adapter/notification"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/adapter/sse"
	"github.com/14mdzk/goscratch/internal/adapter/storage"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/platform/observability"
	"github.com/14mdzk/goscratch/internal/platform/version"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// infraDeps bundles the infrastructure adapters constructed during app boot.
type infraDeps struct {
	db                 *pgxpool.Pool
	cache              port.Cache
	queue              port.Queue
	queueBackend       config.QueueBackend
	storage            port.Storage
	sse                port.SSEBroker
	auditor            port.Auditor
	authorizer         port.Authorizer
	email              port.EmailSender
	notification       port.NotificationSender
	tracerShutdown     func(context.Context) error
	watcherRedisClient *redis.Client
}

func initInfra(ctx context.Context, cfg *config.Config, log *logger.Logger) (*infraDeps, error) {
	tracerShutdown := initTracer(ctx, cfg, log)

	log.Info("Connecting to database...")
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}
	log.Info("Database connected successfully")

	cacheAdapter, err := initCache(cfg, log)
	if err != nil {
		pool.Close()
		return nil, err
	}

	queueAdapter, queueBackend, err := initQueue(cfg, log)
	if err != nil {
		_ = cacheAdapter.Close()
		pool.Close()
		return nil, err
	}

	storageAdapter, err := initStorage(ctx, cfg, log)
	if err != nil {
		_ = queueAdapter.Close()
		_ = cacheAdapter.Close()
		pool.Close()
		return nil, err
	}

	sseBroker := initSSE(cfg)
	auditor := initAuditor(cfg, pool, log)

	authorizer, watcherRedisClient, err := initAuthorizer(ctx, cfg, log)
	if err != nil {
		_ = storageAdapter.Close()
		_ = queueAdapter.Close()
		_ = cacheAdapter.Close()
		pool.Close()
		return nil, err
	}

	emailSender := initEmail(cfg, log)
	notificationSender := initNotification(cfg, log)

	return &infraDeps{
		db:                 pool,
		cache:              cacheAdapter,
		queue:              queueAdapter,
		queueBackend:       queueBackend,
		storage:            storageAdapter,
		sse:                sseBroker,
		auditor:            auditor,
		authorizer:         authorizer,
		email:              emailSender,
		notification:       notificationSender,
		tracerShutdown:     tracerShutdown,
		watcherRedisClient: watcherRedisClient,
	}, nil
}

func initTracer(ctx context.Context, cfg *config.Config, log *logger.Logger) func(context.Context) error {
	if !cfg.Observability.Tracing.Enabled {
		return nil
	}
	log.Info("Initializing OpenTelemetry tracing...", "endpoint", cfg.Observability.Tracing.Endpoint)
	shutdown, err := observability.InitTracer(ctx, observability.TracerConfig{
		ServiceName:    cfg.App.Name,
		ServiceVersion: version.Version,
		Environment:    cfg.App.Env,
		Endpoint:       cfg.Observability.Tracing.Endpoint,
		Enabled:        true,
	})
	if err != nil {
		log.Warn("Failed to initialize tracing", "error", err)
		return nil
	}
	log.Info("Tracing initialized successfully")
	return shutdown
}

// initCache initializes Redis when enabled, or returns NoOpCache when explicitly
// disabled. When redis.enabled=true and Redis is unreachable, boot fails fast
// rather than silently starting with broken login and session revocation.
func initCache(cfg *config.Config, log *logger.Logger) (port.Cache, error) {
	if !cfg.Redis.Enabled {
		log.Warn("SECURITY WARNING: Redis is disabled (redis.enabled=false); login will be rejected and refresh-token revocation will not function")
		return cache.NewNoOpCache(), nil
	}

	log.Info("Connecting to Redis...")
	cacheAdapter, err := cache.NewRedisCache(cfg.Redis.Addr(), cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return nil, fmt.Errorf("redis enabled but unreachable: %w", err)
	}
	log.Info("Redis connected successfully")
	return cacheAdapter, nil
}

// initQueue initializes the queue adapter by resolved backend. Durable tiers
// fail boot when unreachable: silently dropping jobs behind a fallback is worse
// than not starting (fail fast). The NoOp adapter remains for tests and the
// health checker's self-skip, but it is never selected here.
func initQueue(cfg *config.Config, log *logger.Logger) (port.Queue, config.QueueBackend, error) {
	backend, err := cfg.ResolveQueueBackend()
	if err != nil {
		return nil, "", err
	}
	if err := cfg.ValidateQueueWorker(backend); err != nil {
		return nil, "", err
	}

	switch backend {
	case config.QueueBackendRabbitMQ:
		log.Info("Connecting to RabbitMQ...", "backend", backend)
		queueAdapter, err := queue.NewRabbitMQWithOptions(cfg.RabbitMQ.URL, queue.Options{
			PrefetchCount: cfg.RabbitMQ.PrefetchCount,
		})
		if err != nil {
			return nil, "", fmt.Errorf("rabbitmq backend unreachable: %w", err)
		}
		log.Info("RabbitMQ connected successfully")
		return queueAdapter, backend, nil
	case config.QueueBackendGoroutine:
		log.Info("Using in-memory goroutine queue: zero infrastructure, jobs die with the process", "backend", backend)
		return queue.NewGoroutineQueue(), backend, nil
	default:
		return nil, "", fmt.Errorf("queue.backend %q is not implemented", backend)
	}
}

// initStorage initializes the configured storage backend. When storage.mode is
// "s3", initialization errors fail boot rather than silently falling back to
// pod-local disk. Unknown modes are rejected.
func initStorage(ctx context.Context, cfg *config.Config, log *logger.Logger) (port.Storage, error) {
	switch cfg.Storage.Mode {
	case "s3":
		log.Info("Initializing S3 storage...")
		storageAdapter, err := storage.NewS3Storage(ctx, storage.S3Config{
			Endpoint:  cfg.Storage.S3.Endpoint,
			Bucket:    cfg.Storage.S3.Bucket,
			Region:    cfg.Storage.S3.Region,
			AccessKey: cfg.Storage.S3.AccessKey,
			SecretKey: cfg.Storage.S3.SecretKey,
		})
		if err != nil {
			return nil, fmt.Errorf("s3 storage enabled but init failed: %w", err)
		}
		return storageAdapter, nil
	case "", "local":
		log.Info("Initializing local storage...")
		return storage.NewLocalStorage(cfg.Storage.Local.BasePath, "")
	default:
		return nil, fmt.Errorf("unsupported storage.mode %q: must be \"local\" or \"s3\"", cfg.Storage.Mode)
	}
}

func initSSE(cfg *config.Config) port.SSEBroker {
	if cfg.SSE.Enabled {
		return sse.NewBroker(100)
	}
	return sse.NewNoOpBroker()
}

// initAuditor initializes audit logging. The Postgres auditor is wrapped so
// failed INSERTs surface in the operator logs instead of vanishing inside the
// decorators' best-effort `_ = auditor.Log(...)` calls (NoOp never fails).
func initAuditor(cfg *config.Config, pool *pgxpool.Pool, log *logger.Logger) port.Auditor {
	if cfg.Audit.Enabled {
		return audit.NewLoggingAuditor(audit.NewPostgresAuditor(pool), log)
	}
	return audit.NewNoOpAuditor()
}

// initAuthorizer initializes Casbin authorization.
// Fail-fast when authorization is explicitly enabled: a transient DB blip at
// boot must NOT silently open every authenticated endpoint (block-ship #3).
// The NoOpAdapter is intentionally NOT used as a fallback here.
func initAuthorizer(ctx context.Context, cfg *config.Config, log *logger.Logger) (port.Authorizer, *redis.Client, error) {
	if !cfg.Authorization.Enabled {
		// Authorization is explicitly disabled — use NoOp (e.g. local dev without DB).
		log.Warn("Authorization is disabled (authorization.enabled=false); all permission checks are bypassed")
		return casbinadapter.NewNoOpAdapter(), nil, nil
	}

	log.Info("Initializing Casbin authorization...")
	// Wire the Redis watcher when Redis is available so policy writes
	// propagate to peer pods immediately; otherwise the 5-minute
	// backstop reload remains the convergence path. A watcher failure
	// warns and continues unwatched (mirrors the cache fallback above):
	// losing live propagation must not kill boot.
	var watcher persist.Watcher
	var watcherRedisClient *redis.Client
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

	authorizer, err := casbinadapter.NewAdapter(casbinadapter.Config{
		DatabaseURL: cfg.Database.DSN(),
		Watcher:     watcher,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("authorization enabled but Casbin init failed: %w", err)
	}
	log.Info("Casbin authorization initialized successfully")
	return authorizer, watcherRedisClient, nil
}

func initEmail(cfg *config.Config, log *logger.Logger) port.EmailSender {
	if !cfg.Email.Enabled {
		return emailadapter.NewNoOpSender(log)
	}
	log.Info("Initializing SMTP email sender...")
	return emailadapter.NewSMTPSender(emailadapter.SMTPConfig{
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		Username: cfg.Email.Username,
		Password: cfg.Email.Password,
		From:     cfg.Email.From,
	})
}

func initNotification(cfg *config.Config, log *logger.Logger) port.NotificationSender {
	if !cfg.Notification.Enabled {
		return notificationadapter.NewNoOpSender(log)
	}
	log.Info("Initializing webhook notification sender...")
	return notificationadapter.NewWebhookSender(notificationadapter.WebhookConfig{
		URL: cfg.Notification.WebhookURL,
	})
}
