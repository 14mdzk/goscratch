//go:build integration

package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/14mdzk/goscratch/internal/adapter/audit"
	"github.com/14mdzk/goscratch/internal/adapter/cache"
	casbinadapter "github.com/14mdzk/goscratch/internal/adapter/casbin"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/adapter/sse"
	"github.com/14mdzk/goscratch/internal/adapter/storage"
	"github.com/14mdzk/goscratch/internal/module/auth"
	"github.com/14mdzk/goscratch/internal/module/health"
	userrepo "github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/module/job"
	"github.com/14mdzk/goscratch/internal/module/role"
	ssemodule "github.com/14mdzk/goscratch/internal/module/sse"
	storagemodule "github.com/14mdzk/goscratch/internal/module/storage"
	"github.com/14mdzk/goscratch/internal/module/user"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	httpserver "github.com/14mdzk/goscratch/internal/platform/http"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testJWTSecret = "integration-test-secret-key-minimum-length"

// TestJWTSecret returns the JWT secret used for integration tests.
func TestJWTSecret() string {
	return testJWTSecret
}

// TestJWTConfig returns a JWTConfig suitable for integration tests.
func TestJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:          testJWTSecret,
		AccessTokenTTL:  15,
		RefreshTokenTTL: 720,
	}
}

// NewTestApp creates a real Fiber app wired to test container databases,
// with NoOp adapters for RabbitMQ, S3, email, SSE, and tracing.
// Returns the Fiber app (for app.Test()) and a cleanup function.
func NewTestApp(ctx context.Context, pgConnStr, redisAddr string) (*fiber.App, func(), error) {
	log := logger.New(logger.Config{
		Level:  "error",
		Format: "json",
	})

	// Connect to the test PostgreSQL container
	poolCfg, err := pgxpool.ParseConfig(pgConnStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse pg config: %w", err)
	}
	poolCfg.MaxConns = 5
	poolCfg.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pg pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("failed to ping pg: %w", err)
	}

	// Set up Redis cache (real if address provided, otherwise NoOp)
	var cacheAdapter port.Cache
	if redisAddr != "" {
		cacheAdapter, err = cache.NewRedisCache(redisAddr, "", 0)
		if err != nil {
			cacheAdapter = cache.NewNoOpCache()
		}
	} else {
		cacheAdapter = cache.NewNoOpCache()
	}

	// NoOp adapters for services not needed in integration tests
	queueAdapter := queue.NewNoOpQueue()
	sseBroker := sse.NewNoOpBroker()
	auditor := audit.NewNoOpAuditor()
	authorizer := casbinadapter.NewNoOpAdapter()

	storageAdapter, err := storage.NewLocalStorage("/tmp/goscratch-test-storage", "")
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("failed to create local storage: %w", err)
	}

	jwtCfg := TestJWTConfig()

	serverCfg := config.ServerConfig{
		Host:         "127.0.0.1",
		Port:         0,
		ReadTimeout:  30,
		WriteTimeout: 30,
		IdleTimeout:  60,
	}
	server := httpserver.NewServer(serverCfg, log, false)
	app := server.App()

	// Wire up modules exactly like app.go
	publisher := worker.NewPublisher(queueAdapter, "jobs", "")

	transactor := database.NewTransactor(pool)

	healthModule := health.NewModule()
	sharedUserRepo := userrepo.NewRepository(pool)
	authModule := auth.NewModule(sharedUserRepo, cacheAdapter, auditor, jwtCfg)
	userModule := user.NewModule(pool, transactor, auditor, authorizer, cacheAdapter, jwtCfg.Secret, authModule.Revoker())
	roleModule := role.NewModule(authorizer, jwtCfg.Secret)
	storageModule := storagemodule.NewModule(storageAdapter, auditor, jwtCfg.Secret)
	sseModule := ssemodule.NewModule(sseBroker, authorizer, jwtCfg.Secret)
	jobModule := job.NewModule(publisher, auditor, authorizer, jwtCfg.Secret)

	server.RegisterModules(healthModule, userModule, authModule, roleModule, storageModule, sseModule, jobModule)

	cleanup := func() {
		_ = app.Shutdown()
		cacheAdapter.Close()
		queueAdapter.Close()
		storageAdapter.Close()
		sseBroker.Close()
		auditor.Close()
		authorizer.Close()
		pool.Close()
	}

	return app, cleanup, nil
}

// testJWTClaims is a local claims struct for token generation in tests.
// It must stay in sync with the jwtClaims shape in middleware/auth.go and
// auth/usecase/auth_usecase.go.
type testJWTClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// GenerateAccessToken generates a JWT access token for testing purposes.
// The returned token will be parsed correctly by the Auth middleware.
func GenerateAccessToken(userID, email, name string) (string, error) {
	now := time.Now()
	claims := testJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID: userID,
		Email:  email,
		Name:   name,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(testJWTSecret))
}

