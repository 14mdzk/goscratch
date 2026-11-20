package app

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/internal/adapter/cache"
	emailadapter "github.com/14mdzk/goscratch/internal/adapter/email"
	notificationadapter "github.com/14mdzk/goscratch/internal/adapter/notification"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/adapter/sse"
	"github.com/14mdzk/goscratch/internal/adapter/storage"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
)

func newWiringTestLogger() *logger.Logger {
	return logger.New(logger.Config{
		Level:  "error",
		Format: "json",
		Output: &bytes.Buffer{},
	})
}

func TestBuildCORSConfig(t *testing.T) {
	log := newWiringTestLogger()

	t.Run("development_defaults_to_wildcard", func(t *testing.T) {
		cfg := &config.Config{
			App: config.AppConfig{Env: "development"},
		}
		got := buildCORSConfig(cfg, log)
		assert.Equal(t, "*", got.AllowOrigins)
	})

	t.Run("explicit_overrides_preserved", func(t *testing.T) {
		cfg := &config.Config{
			App: config.AppConfig{Env: "production"},
			CORS: config.CORSConfig{
				AllowOrigins:     "https://app.example.com",
				AllowMethods:     "GET,POST",
				AllowHeaders:     "Authorization",
				AllowCredentials: true,
			},
		}
		got := buildCORSConfig(cfg, log)
		assert.Equal(t, "https://app.example.com", got.AllowOrigins)
		assert.Equal(t, "GET,POST", got.AllowMethods)
		assert.Equal(t, "Authorization", got.AllowHeaders)
		assert.True(t, got.AllowCredentials)
	})
}

func TestInitAdapters_DisabledUseNoOp(t *testing.T) {
	log := newWiringTestLogger()
	cfg := &config.Config{
		Storage: config.StorageConfig{
			Mode:  "local",
			Local: config.LocalStorage{BasePath: t.TempDir()},
		},
	}

	cacheAdapter, rateLimiter, err := initCache(cfg, log)
	require.NoError(t, err)
	assert.IsType(t, &cache.NoOpCache{}, cacheAdapter)
	assert.IsType(t, &cache.NoOpCache{}, rateLimiter)
	assert.IsType(t, &sse.NoOpBroker{}, initSSE(cfg))
	assert.IsType(t, &emailadapter.NoOpSender{}, initEmail(cfg, log))
	assert.IsType(t, &notificationadapter.NoOpSender{}, initNotification(cfg, log))

	authz, watcherClient, err := initAuthorizer(context.Background(), cfg, log)
	require.NoError(t, err)
	assert.NotNil(t, authz)
	assert.Nil(t, watcherClient)

	st, err := initStorage(context.Background(), cfg, log)
	require.NoError(t, err)
	assert.NotNil(t, st)
	_ = st.Close()
}

func TestInitCache_FailFastWhenEnabledAndUnreachable(t *testing.T) {
	log := newWiringTestLogger()

	// Bind and immediately close an ephemeral TCP port so dialing fails fast
	// with connection refused rather than waiting for a timeout.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, ln.Close())
	portNum, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	cfg := &config.Config{
		Redis: config.RedisConfig{
			Enabled: true,
			Host:    host,
			Port:    portNum,
		},
	}

	got, limiter, err := initCache(cfg, log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis enabled but unreachable")
	assert.Nil(t, got)
	assert.Nil(t, limiter)
}

func TestInitCache_EnabledConnects(t *testing.T) {
	log := newWiringTestLogger()
	mr := miniredis.RunT(t)

	host, portStr, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)
	portNum, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	cfg := &config.Config{
		Redis: config.RedisConfig{
			Enabled: true,
			Host:    host,
			Port:    portNum,
		},
	}

	got, limiter, err := initCache(cfg, log)
	require.NoError(t, err)
	assert.IsType(t, &cache.RedisCache{}, got)
	assert.IsType(t, &cache.RedisCache{}, limiter)
	require.NoError(t, got.Close())
}

func TestInitStorage_FailFast(t *testing.T) {
	log := newWiringTestLogger()

	t.Run("s3_init_failure_does_not_fallback_to_local", func(t *testing.T) {
		cfg := &config.Config{
			Storage: config.StorageConfig{
				Mode:  "s3",
				Local: config.LocalStorage{BasePath: t.TempDir()},
				S3:    config.S3Storage{Region: "us-east-1"}, // missing bucket
			},
		}

		got, err := initStorage(context.Background(), cfg, log)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "s3 storage enabled but init failed")
		assert.Nil(t, got)
	})

	t.Run("unsupported_mode_fails_fast", func(t *testing.T) {
		cfg := &config.Config{
			Storage: config.StorageConfig{
				Mode:  "gcs",
				Local: config.LocalStorage{BasePath: t.TempDir()},
			},
		}

		got, err := initStorage(context.Background(), cfg, log)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported storage.mode")
		assert.Nil(t, got)
	})

	t.Run("empty_mode_uses_local_storage", func(t *testing.T) {
		cfg := &config.Config{
			Storage: config.StorageConfig{
				Mode:  "",
				Local: config.LocalStorage{BasePath: t.TempDir()},
			},
		}

		got, err := initStorage(context.Background(), cfg, log)
		require.NoError(t, err)
		assert.IsType(t, &storage.LocalStorage{}, got)
		require.NoError(t, got.Close())
	})
}

func TestInitQueue_Goroutine(t *testing.T) {
	log := newWiringTestLogger()
	cfg := &config.Config{
		Queue:  config.QueueConfig{Backend: string(config.QueueBackendGoroutine)},
		Worker: config.WorkerConfig{InProcess: true},
	}

	q, backend, err := initQueue(cfg, log)
	require.NoError(t, err)
	assert.Equal(t, config.QueueBackendGoroutine, backend)
	require.NotNil(t, q)
	_ = q.Close()
}

func TestInitInProcessWorker(t *testing.T) {
	log := newWiringTestLogger()

	t.Run("nil_for_rabbitmq_backend", func(t *testing.T) {
		w := initInProcessWorker(&config.Config{}, log, &infraDeps{
			queueBackend: config.QueueBackendRabbitMQ,
		})
		assert.Nil(t, w)
	})

	t.Run("registers_default_handlers_for_goroutine_backend", func(t *testing.T) {
		q := queue.NewNoOpQueue()
		w := initInProcessWorker(
			&config.Config{Worker: config.WorkerConfig{QueueName: "jobs", Concurrency: 0}},
			log,
			&infraDeps{
				queue:        q,
				queueBackend: config.QueueBackendGoroutine,
				email:        emailadapter.NewNoOpSender(log),
				notification: notificationadapter.NewNoOpSender(log),
			},
		)
		require.NotNil(t, w)
		assert.Equal(t, 2, w.Stats()["concurrency"])
		assert.ElementsMatch(t, []string{
			worker.JobTypeEmailSend,
			worker.JobTypeNotification,
			worker.JobTypeAuditCleanup,
		}, w.Stats()["handlers"])
	})
}

func TestInitHTTP_WithRateLimit(t *testing.T) {
	log := newWiringTestLogger()
	cfg := &config.Config{
		App: config.AppConfig{Env: "development"},
		RateLimit: config.RateLimitConfig{
			Enabled:   true,
			Max:       0, // exercise default
			WindowSec: 0, // exercise default
		},
	}

	h := initHTTP(cfg, log, cache.NewNoOpCache())
	require.NotNil(t, h.server)
	assert.Nil(t, h.metricsServer)
	require.NotNil(t, h.rateLimitCloser)
	require.NoError(t, h.rateLimitCloser.Close())
}
