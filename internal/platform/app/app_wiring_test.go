package app

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/internal/adapter/cache"
	emailadapter "github.com/14mdzk/goscratch/internal/adapter/email"
	notificationadapter "github.com/14mdzk/goscratch/internal/adapter/notification"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/adapter/sse"
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

	assert.IsType(t, &cache.NoOpCache{}, initCache(cfg, log))
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
