# ADR-013: Optional PostGIS and Archived Spatial Primitives

**Date:** 2026-11-21
**Status:** Accepted
**References:** Issue `#147`, PR `#52`, branch `archive/postgis-v1.x`, [PostGIS Opt-In Recipe](../recipes/postgis.md)

---

## Context

PR `#52` enabled PostGIS across the repository by switching the default PostgreSQL containers to `postgis/postgis:18-master` and adding `000004_postgis.{up,down}.sql` (`CREATE EXTENSION IF NOT EXISTS postgis`). In v1.3, `internal/shared/domain/geo` and `pkg/geoutil` were added so spatial primitives existed in the repository even though no core module imported or used them.

Keeping PostGIS wired unconditionally into the starterkit imposed real cost on non-geospatial projects:

1. **Unpinned, single-arch, heavy container image:** `postgis/postgis:18-master` tracks `-master` (~600 MB vs ~90 MB for `postgres:18-alpine`) and publishes `linux/amd64` only, requiring QEMU/Rosetta emulation or platform overrides on `arm64` hosts while `redis`, `rabbitmq`, and `nginx` run natively.
2. **Migration failures on managed PostgreSQL:** `000004_postgis.up.sql` fails `make migrate-up` on managed PostgreSQL instances where the `postgis` extension is unavailable or `CREATE EXTENSION` requires elevated privileges.
3. **Hidden PostGIS tax in integration tests:** Both `migrations/migrate_test.go`, `internal/platform/testutil/containers.go`, and `internal/shared/domain/geo/point_pgx_integration_test.go` pulled `postgis/postgis:18-master`.
4. **Zero in-repo consumers:** Neither `internal/shared/domain/geo` nor `pkg/geoutil` was imported by `cmd/`, `internal/module/`, `internal/platform/`, `internal/adapter/`, or `internal/port/`.

By contrast, Server-Sent Events (`sse.enabled: false`) remain in core because the in-memory SSE broker requires no external container or database extension and is already runtime-toggleable.

## Decision

1. **Archive PostGIS primitives out of `main`:** Preserve `internal/shared/domain/geo`, `pkg/geoutil`, and `000004_postgis.{up,down}.sql` on branch `archive/postgis-v1.x` (mirroring `archive/audit-v1.x`) and delete them from `main`.
2. **Default to multi-arch `postgres:18-alpine` without pinning `platform:`:** Update `docker-compose.yml`, `deploy/docker/docker-compose.prod.yml`, `internal/platform/testutil/containers.go`, and `migrations/migrate_test.go` to use `postgres:18-alpine`, letting Docker use each host's native Linux architecture (`linux/amd64` on x86_64 Linux/CI, `linux/arm64` on ARM64 Macs/servers). Also align the production Compose data volume mount to PostgreSQL 18's `/var/lib/postgresql` path.
3. **Keep the `000003` → `000005` migration version gap:** Removing `000004_postgis.{up,down}.sql` leaves `000001`–`000003` followed by `000005_audit_read_policy` in `migrations/` and `internal/platform/testutil/migrations/`. `golang-migrate` applies non-contiguous versions cleanly on fresh databases, and avoiding a renumber prevents breaking existing v1.3 deployments already recorded at version `5`.
4. **Provide a copy-paste opt-in recipe:** Document restoration and setup steps in [docs/recipes/postgis.md](../recipes/postgis.md).

## Consequences

- **Pro:** Non-geospatial starterkit clones boot against vanilla PostgreSQL 18 (local Docker, CI testcontainers, and managed cloud Postgres) with smaller native multi-arch images and zero unused spatial packages.
- **Pro:** Teams that need geospatial support can restore tested value types (`Point`, `Polygon`, `BoundingBox`, `Distance`, EWKB `pgx` round-trip, RFC 7946 GeoJSON, `geoutil.Haversine`) from `origin/archive/postgis-v1.x` in two commands.
- **Con / Operator upgrade note:**
  - Fresh clones run migrations `1 → 2 → 3 → 5` without PostGIS.
  - Existing v1.3 databases already at version `5` continue running forward migrations cleanly, but rolling `down` across version `4` requires either restoring `000004_postgis.{up,down}.sql` or managing `schema_migrations` explicitly.
  - Existing Docker volumes initialized with `postgis/postgis:18-master` contain PostGIS extension metadata and `/var/lib/postgresql/data` layout assumptions; upgrading deployments with an existing database volume should keep their PostGIS image or recreate the volume from a clean dump when switching to `postgres:18-alpine`.
