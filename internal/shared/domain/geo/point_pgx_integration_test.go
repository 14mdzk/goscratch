//go:build integration

package geo_test

import (
	"context"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPostGIS spins up a postgis/postgis:18-master container with PostGIS
// extension enabled and returns a connected pool plus a cleanup function.
func startPostGIS(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgis/postgis:18-master",
		postgres.WithDatabase("geotestdb"),
		postgres.WithUsername("geouser"),
		postgres.WithPassword("geopass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("startPostGIS: failed to start container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("startPostGIS: failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("startPostGIS: failed to create pool: %v", err)
	}

	// Enable PostGIS extension.
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS postgis")
	if err != nil {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("startPostGIS: failed to create postgis extension: %v", err)
	}

	cleanup := func() {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
	}
	return pool, cleanup
}

// TestPointRoundTripPostGIS verifies the production query shape:
//
//	INSERT INTO t (loc) VALUES ($1)   -- geo.Point.Value() sends hex EWKB string
//	SELECT loc FROM t WHERE id = $1   -- Scan receives hex EWKB from text protocol
//
// No PostGIS helper functions wrap the parameter or the result. This is the
// exact path application code takes when storing and loading a
// geography(Point,4326) column.
func TestPointRoundTripPostGIS(t *testing.T) {
	pool, cleanup := startPostGIS(t)
	defer cleanup()

	ctx := context.Background()

	// Create a minimal table with a geography(Point,4326) column.
	_, err := pool.Exec(ctx, `
		CREATE TEMP TABLE geo_test (
			id   SERIAL PRIMARY KEY,
			loc  geography(Point,4326) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("CREATE TEMP TABLE: %v", err)
	}

	cases := []struct {
		name string
		p    geo.Point
	}{
		{"origin", geo.MustPoint(0, 0)},
		{"positive", geo.MustPoint(139.6917, 35.6895)},  // Tokyo
		{"negative", geo.MustPoint(-43.1729, -22.9068)}, // Rio
		{"antipode", geo.MustPoint(-179.9999, -89.9999)},
		{"max", geo.MustPoint(180, 90)},
		{"min", geo.MustPoint(-180, -90)},
		{"prime meridian", geo.MustPoint(0, 51.4779)},  // Greenwich
		{"new york", geo.MustPoint(-74.006, 40.7128)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// INSERT — raw bind: Value() returns a hex EWKB string; pgx sends
			// it with OID 25 (text) and PostGIS accepts it for geography columns.
			var id int
			err := pool.QueryRow(ctx,
				"INSERT INTO geo_test (loc) VALUES ($1) RETURNING id",
				tc.p,
			).Scan(&id)
			if err != nil {
				t.Fatalf("INSERT %v: %v", tc.p, err)
			}

			// SELECT — raw column: pgx text protocol sends hex-encoded EWKB;
			// Scan decodes it via DecodeEWKB. No ST_* wrapper, no ::geometry cast.
			var got geo.Point
			err = pool.QueryRow(ctx,
				"SELECT loc FROM geo_test WHERE id = $1",
				id,
			).Scan(&got)
			if err != nil {
				t.Fatalf("SELECT %v: %v", tc.p, err)
			}

			// geography(Point,4326) stores double-precision coordinates; no
			// quantization occurs on standard PostGIS builds so exact equality holds.
			if got.Lon != tc.p.Lon || got.Lat != tc.p.Lat {
				t.Errorf("round-trip mismatch: inserted {%v, %v}, got {%v, %v}",
					tc.p.Lon, tc.p.Lat, got.Lon, got.Lat)
			}
		})
	}
}
