package app

import (
	auditmodule "github.com/14mdzk/goscratch/internal/module/audit"
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
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/internal/worker/handlers"
	"github.com/14mdzk/goscratch/pkg/logger"
)

func initModules(cfg *config.Config, log *logger.Logger, infra *infraDeps, server *http.Server) *worker.Worker {
	publisher := worker.NewPublisher(infra.queue, cfg.Worker.QueueName, cfg.Worker.Exchange)
	transactor := database.NewTransactor(infra.db)

	docsModule := docs.NewModule()

	// Wire health checkers. Postgres is always checked. Cache and queue checkers
	// are included unconditionally — their implementations self-skip (return nil)
	// when the underlying adapter is a NoOp. Casbin is included when authorization
	// is enabled (real adapter); the NoOp adapter is self-skipping too.
	healthCheckers := []health.HealthChecker{
		health.NewPostgresChecker(infra.db),
		health.NewCacheChecker(infra.cache),
		health.NewQueueChecker(infra.queue),
		health.NewAuthzChecker(infra.authorizer),
	}
	healthModule := health.NewModule(cfg.Health.ReadinessTimeout(), healthCheckers...)

	// Single shared user-repo instance wired into both the user module and the
	// auth module so both use the same *pgxpool.Pool connection rather than
	// opening a second one (audit finding: auth/module.go:20 instantiated its
	// own userrepo.Repository).
	sharedUserRepo := userrepo.NewRepository(infra.db)

	// Build the JWT auth middleware once from cfg.JWT (including configured
	// issuer, audience, and overlap-rotation secret) and inject it into every
	// module that mounts protected routes.
	authMiddleware := middleware.Auth(middleware.AuthConfigFromJWT(cfg.JWT))

	// Auth module is constructed first so its Revoker can be injected into the
	// user module (ChangePassword must revoke auth sessions cross-module).
	authModule := auth.NewModule(sharedUserRepo, infra.cache, infra.auditor, cfg.JWT, authMiddleware)
	userModule := user.NewModule(infra.db, transactor, infra.auditor, infra.authorizer, infra.cache, authMiddleware, authModule.Revoker())
	roleModule := role.NewModule(infra.authorizer, authMiddleware)
	storageModule := storagemodule.NewModule(infra.storage, infra.auditor, authMiddleware)
	sseModule := ssemodule.NewModule(infra.sse, infra.authorizer, authMiddleware)
	jobModule := job.NewModule(publisher, infra.auditor, infra.authorizer, authMiddleware)
	auditModule := auditmodule.NewModule(infra.auditor, infra.authorizer, authMiddleware)

	server.RegisterModules(
		docsModule,
		healthModule,
		userModule,
		authModule,
		roleModule,
		storageModule,
		sseModule,
		jobModule,
		auditModule,
	)

	return initInProcessWorker(cfg, log, infra)
}

// initInProcessWorker builds the in-process worker when queue.backend is
// goroutine. Handlers match cmd/worker via handlers.RegisterDefault so both
// paths execute the same job types.
func initInProcessWorker(cfg *config.Config, log *logger.Logger, infra *infraDeps) *worker.Worker {
	if infra.queueBackend != config.QueueBackendGoroutine {
		return nil
	}

	concurrency := cfg.Worker.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}

	inProcessWorker := worker.New(infra.queue, log, worker.Config{
		QueueName:   cfg.Worker.QueueName,
		Exchange:    cfg.Worker.Exchange,
		Concurrency: concurrency,
	})
	handlers.RegisterDefault(inProcessWorker, handlers.Dependencies{
		Logger:       log,
		DB:           infra.db,
		Email:        infra.email,
		Notification: infra.notification,
	})
	return inProcessWorker
}
