# Opt-In Recipe: Enabling PostGIS and Spatial Primitives

Per [ADR-013](../adr/013-optional-postgis.md), `goscratch` defaults to vanilla `postgres:18-alpine` so non-geospatial projects do not pay the image size or managed-DB extension tax of PostGIS.

If your application needs geospatial queries (`geography(Point, 4326)`, bounding boxes, GeoJSON, Haversine pre-filtering), follow the steps below to restore the archived v1.3 spatial packages and enable PostGIS.

---

## Step 1: Restore the spatial packages from `archive/postgis-v1.x`

All removed spatial code (`internal/shared/domain/geo`, `pkg/geoutil`) and the original extension migration live on the `archive/postgis-v1.x` branch:

```bash
git fetch origin archive/postgis-v1.x

# On a fresh project (before running migrations, or before version 000005):
git checkout origin/archive/postgis-v1.x -- \
  internal/shared/domain/geo \
  pkg/geoutil \
  migrations/000004_postgis.up.sql \
  migrations/000004_postgis.down.sql

cp migrations/000004_postgis.*.sql internal/platform/testutil/migrations/
```

> **Already past migration `000005`?** Do **not** insert `000004_postgis` behind an already-applied `000005`. Restore only the Go packages (`internal/shared/domain/geo` and `pkg/geoutil`), then create a new sequential migration:
>
> ```bash
> make migrate-create NAME=enable_postgis
> ```
>
> Put `CREATE EXTENSION IF NOT EXISTS postgis;` in the new `.up.sql` and `DROP EXTENSION IF EXISTS postgis_topology; DROP EXTENSION IF EXISTS postgis;` in the `.down.sql`, then mirror both files into `internal/platform/testutil/migrations/`.

---

## Step 2: Switch PostgreSQL containers to a PostGIS image

Replace `postgres:18-alpine` with a PostGIS-enabled image (for example `postgis/postgis:18-master`, adding `platform: linux/amd64` only if your host architecture requires emulation for an `amd64`-only image) in:

1. `docker-compose.yml` (`services.postgres.image`)
2. `deploy/docker/docker-compose.prod.yml` (`services.postgres.image` — verify volume mount matches your image's `PGDATA`, `/var/lib/postgresql/data` on older layouts vs `/var/lib/postgresql` on PG 18)
3. `internal/platform/testutil/containers.go` (`StartPostgres`)
4. `migrations/migrate_test.go` (`TestMigrations_UpDownRoundTrip`)

---

## Step 3: Run migrations and verify

```bash
make docker-up
make migrate-up
make lint-migration-fixture
make test
```

If Docker is available, run the integration tests to verify both migrations and `geo.Point` EWKB round-trip behavior against PostgreSQL + PostGIS:

```bash
go test -race -tags=integration ./migrations/... ./internal/shared/domain/geo/...
```

---

## Using the restored types with `pgx` / `sqlc`

- `geo.Point` implements `driver.Valuer` (hex-encoded EWKB string, accepted directly by `pgx` for `geography(Point, 4326)` parameters) and `sql.Scanner` (decodes EWKB text/binary output back into `geo.Point`), plus RFC 7946 GeoJSON marshaling/unmarshaling.
- Map PostgreSQL `geography(Point, 4326)` columns to `github.com/14mdzk/goscratch/internal/shared/domain/geo.Point` in `sqlc.yaml` column/type overrides (`sql_package: "pgx/v5"`).
- `geoutil.Haversine(a, b)` computes great-circle distances in pure Go for pre-filtering before hitting PostgreSQL (`ST_DWithin` / `<->` remains the source of truth for spatial indexes).
