//go:build integration

package migrations_test

import (
	"context"
	"embed"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed *.sql
var migrationsFS embed.FS

// TestMigrations_UpDownRoundTrip applies every up migration in this directory
// against a fresh PostGIS container, then rolls all the way down and asserts
// the public schema is empty apart from golang-migrate's own version table.
// It runs against the real migrations/ directory (not the testutil fixture
// copy) so a broken down migration shows here, not in prod.
//
// Requires Docker. Run with: go test -tags=integration ./migrations/
func TestMigrations_UpDownRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgis/postgis:18-master",
		// 18-master publishes amd64 only; emulate there (no-op on amd64 CI).
		testcontainers.WithImagePlatform("linux/amd64"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(120*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	d, err := iofs.New(migrationsFS, ".")
	require.NoError(t, err)

	// golang-migrate's pgx/v5 driver uses the "pgx5://" scheme.
	m, err := migrate.NewWithSourceInstance("iofs", d, "pgx5"+connStr[len("postgres"):])
	require.NoError(t, err)
	defer m.Close()

	// All the way up.
	require.NoError(t, m.Up())
	version, dirty, err := m.Version()
	require.NoError(t, err)
	assert.False(t, dirty, "migration left a dirty version")
	assert.Equal(t, uint(4), version, "expected all 4 migrations applied")

	// Sanity: the migrated objects exist.
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	var usersExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'users')`,
	).Scan(&usersExists))
	assert.True(t, usersExists, "users table must exist after Up")

	// All the way down.
	require.NoError(t, m.Down())
	_, _, err = m.Version()
	assert.ErrorIs(t, err, migrate.ErrNilVersion, "version table must be empty after full Down")

	// Public schema holds nothing but the migrate version table.
	rows, err := pool.Query(ctx, `
		SELECT tablename FROM pg_tables WHERE schemaname = 'public'
		UNION ALL
		SELECT viewname FROM pg_views WHERE schemaname = 'public'
	`)
	require.NoError(t, err)
	defer rows.Close()

	var leftovers []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		if name != "schema_migrations" {
			leftovers = append(leftovers, name)
		}
	}
	require.NoError(t, rows.Err())
	assert.Empty(t, leftovers, "down migrations left objects behind")
}
