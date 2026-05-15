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

// TestPointRoundTripPostGIS verifies that geo.Point encodes to EWKB, is
// accepted by a real PostGIS geography(Point,4326) column, and decodes back
// to the exact same coordinate pair via Scan.
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
		{"positive", geo.MustPoint(139.6917, 35.6895)},   // Tokyo
		{"negative", geo.MustPoint(-43.1729, -22.9068)},  // Rio
		{"antipode", geo.MustPoint(-179.9999, -89.9999)},
		{"max", geo.MustPoint(180, 90)},
		{"min", geo.MustPoint(-180, -90)},
		{"prime meridian", geo.MustPoint(0, 51.4779)},     // Greenwich
		{"new york", geo.MustPoint(-74.006, 40.7128)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Insert using driver.Valuer (Value() returns EWKB bytes).
			var id int
			err := pool.QueryRow(ctx,
				"INSERT INTO geo_test (loc) VALUES (ST_GeomFromWKB($1, 4326)) RETURNING id",
				tc.p,
			).Scan(&id)
			if err != nil {
				t.Fatalf("INSERT %v: %v", tc.p, err)
			}

			// Select back and decode via Scan.
			// Cast geography to geometry before calling ST_AsEWKB so that
			// the SRID tag is preserved and our decoder receives a full EWKB.
			var got geo.Point
			err = pool.QueryRow(ctx,
				"SELECT ST_AsEWKB(loc::geometry) FROM geo_test WHERE id = $1",
				id,
			).Scan(&got)
			if err != nil {
				t.Fatalf("SELECT %v: %v", tc.p, err)
			}

			const tolerance = 1e-9
			lonDiff := tc.p.Lon - got.Lon
			latDiff := tc.p.Lat - got.Lat
			if lonDiff > tolerance || lonDiff < -tolerance ||
				latDiff > tolerance || latDiff < -tolerance {
				t.Errorf("round-trip mismatch: inserted {%v, %v}, got {%v, %v}",
					tc.p.Lon, tc.p.Lat, got.Lon, got.Lat)
			}
		})
	}
}
