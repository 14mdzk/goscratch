//go:build integration

package testutil

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// StartPostgres starts a PostgreSQL container, runs migrations, and returns the DSN.
func StartPostgres(ctx context.Context) (connString string, cleanup func(), err error) {
	pgContainer, err := postgres.Run(ctx,
		"postgis/postgis:18-master",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	cleanup = func() {
		if pgContainer != nil {
			_ = pgContainer.Terminate(ctx)
		}
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to get postgres connection string: %w", err)
	}

	// Run migrations
	if err := runMigrations(connStr); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return connStr, cleanup, nil
}

// runMigrations applies database migrations from the embedded migrations directory.
func runMigrations(connStr string) error {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create iofs driver: %w", err)
	}

	// golang-migrate's pgx/v5 driver uses "pgx5://" scheme
	// Convert "postgres://" to "pgx5://"
	pgx5ConnStr := "pgx5" + connStr[len("postgres"):]

	m, err := migrate.NewWithSourceInstance("iofs", d, pgx5ConnStr)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations up: %w", err)
	}

	return nil
}

// StartRedis starts a Redis container and returns the address.
func StartRedis(ctx context.Context) (addr string, cleanup func(), err error) {
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(15*time.Second),
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to start redis container: %w", err)
	}

	cleanup = func() {
		if redisContainer != nil {
			_ = redisContainer.Terminate(ctx)
		}
	}

	host, err := redisContainer.Host(ctx)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to get redis host: %w", err)
	}

	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to get redis port: %w", err)
	}

	return fmt.Sprintf("%s:%s", host, port.Port()), cleanup, nil
}
