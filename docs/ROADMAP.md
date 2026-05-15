# Roadmap

## v0.1 - Foundation (DONE)

Core infrastructure and patterns established. All features implemented and tested.

### Commits

- `c30d347` init
- `b7dc133` feat(authz): implement Casbin-based permission authorization
- `6124f61` feat(worker): add RabbitMQ background job processing
- `9eb21dc` feat: bidirectional cursor-based pagination for user listing
- `c00998b` feat: robust bidirectional cursor pagination with explicit direction handling
- `2364d8c` test: comprehensive test coverage for v0.1 foundation (18% -> 90%+)

### Features

| Feature | Module | Code | Tests |
|---------|--------|------|-------|
| JWT Authentication (login/refresh/logout) | `auth` | Done | Done - handler + usecase full flow |
| User CRUD (create, read, update, soft delete) | `user` | Done | Done - handler + usecase + repository |
| User activate/deactivate | `user` | Done | Done |
| Change password | `user` | Done | Done |
| Bidirectional cursor pagination | `shared/domain` | Done | Done - including bidirectional tests |
| RBAC with Casbin | `adapter/casbin` | Done | Done |
| Audit logging (PostgreSQL) | `adapter/audit` | Done | Done |
| Redis cache | `adapter/cache` | Done | Done - with miniredis |
| RabbitMQ queue | `adapter/queue` | Done | Done |
| File storage (S3 + Local) | `adapter/storage` | Done | Done |
| SSE broker (in-memory) | `adapter/sse` | Done | Done |
| Background job worker | `worker` | Done | Done - lifecycle, job, publisher |
| Audit cleanup handler | `worker/handlers` | Done | Done |
| JWT auth middleware | `middleware` | Done | Done |
| Permission/role middleware | `middleware` | Done | Done |
| Request logging middleware | `middleware` | Done | Done |
| Error handler middleware | `middleware` | Done | Done |
| Request ID + CORS middleware | `middleware` | Done | Done |
| Prometheus metrics | `observability` | Done | N/A (infrastructure) |
| OpenTelemetry tracing | `observability` | Done | N/A (infrastructure) |
| Structured logging | `pkg/logger` | Done | Done |
| App error types | `pkg/apperr` | Done | Done |
| HTTP response helpers | `pkg/response` | Done | Done |
| PostgreSQL utilities | `pkg/pgutil` | Done | Done |
| Optional types (Opt/NOpt) | `pkg/types` | Done | Done |
| Input validation | `platform/validator` | Done | Done - including ValidateAndBind, ValidateQuery |
| Config (JSON + env) | `platform/config` | Done | Done - load, overrides, helpers |
| Health checks | `health` | Done | Done |
| Dockerfile (multi-stage) | `Dockerfile` | Done | N/A - Go 1.25 + worker target |
| Docker Compose | `docker-compose.yml` | Done | N/A |
| Migrations | `migrations/` | Done | N/A |
| Seed data | `scripts/seed` | Done | N/A |
| Makefile | `Makefile` | Done | N/A |

### Test Coverage (v0.1 final)

| Category | Test Suites |
|----------|-------------|
| Modules (auth/user/health handler + usecase + repo) | 6 |
| Adapters (cache, queue, audit, casbin, SSE, storage) | 6 |
| Middleware (auth, authz, error, request ID, logger) | 1 (combined) |
| Platform (config, validator) | 2 |
| Worker (worker, job, publisher, handlers) | 2 |
| Pkg (apperr, response, pgutil, types, logger) | 5 |
| Shared domain (cursor) | 1 |
| **Total** | **23 suites** |

---

## v0.2 - Complete v0.1 Tests + Fixes (DONE)

Merged into v0.1 completion. All test backfill was done as part of commit `2364d8c`.

- [x] Auth handler tests (Login, Refresh, Logout)
- [x] Auth usecase full flow tests (9 scenarios)
- [x] User handler tests (all 9 methods)
- [x] User usecase missing tests (Update, Activate, Deactivate + audit)
- [x] User repository missing tests (UpdatePassword, Activate, Deactivate, Count)
- [x] All 6 adapter test suites (cache, queue, audit, casbin, SSE, storage)
- [x] All middleware tests (auth, authz, error handler, request ID, logger)
- [x] Worker tests (lifecycle, job encode/decode, publisher, handlers)
- [x] Utility tests (response, pgutil, types, logger)
- [x] Platform tests (config loading + env overrides, validator expansion)
- [x] Shared domain tests (NewBidirectionalCursorPage)
- [x] Fix Dockerfile Go version (1.22 -> 1.25) + worker build target
- [x] Health handler tests

---

## v0.3 - Complete the Starterkit (DONE)

All missing features implemented with handler + usecase + tests.

### Commit

- `f86f46c` feat: complete starterkit with role management, file storage, SSE, email, job API, and rate limiting

### Features Delivered

| Feature | Status | Endpoints | Tests |
|---------|--------|-----------|-------|
| **Role & Permission Management** | Done | 9 endpoints (assign, revoke, list roles, role users, role permissions, add/remove permission, user roles, user permissions) | Handler + Usecase (30 tests) |
| **File Upload/Download API** | Done | 5 endpoints (upload, download, delete, get URL, list) with path traversal protection, content type validation, size limits | Handler + Usecase |
| **SSE HTTP Endpoints** | Done | 3 endpoints (subscribe, broadcast, client count) wired to existing broker | Handler |
| **Email Service Integration** | Done | SMTP adapter + NoOp fallback, email port interface, replaced stub handler | Adapter tests |
| **Job Publishing API** | Done | 2 endpoints (dispatch job, list job types) admin-only | Handler + Usecase |
| **Rate Limiting Middleware** | Done | Sliding window with in-memory + Redis backends, X-RateLimit headers, 429 response | Middleware tests |

### New Modules Added

```
internal/module/role/       - Role & permission management
internal/module/storage/    - File upload/download
internal/module/sse/        - Server-Sent Events
internal/module/job/        - Job dispatching
internal/adapter/email/     - SMTP + NoOp email adapters
internal/port/email.go      - Email sender interface
internal/platform/http/middleware/rate_limit.go
```

### Config Additions

- `email` config (enabled, host, port, username, password, from)
- `rate_limit` config (enabled, max, window_sec)

### Remaining Gaps (completed in v0.4)

- [x] Feature specs (`docs/features/`) for each feature
- [x] Seed data update for default roles + permissions

### Test Coverage (v0.3 final)

| Category | Test Suites |
|----------|-------------|
| All previous (v0.1) | 23 |
| Role module (handler + usecase) | 2 |
| Storage module (handler + usecase) | 2 |
| SSE module (handler) | 1 |
| Email adapter | 1 |
| Job module (handler + usecase) | 2 |
| **Total** | **31 suites, 0 failures** |

---

## v0.4 - Developer Experience (DONE)

Improve the daily development workflow.

### Commit

- `8396ff0` chore: v0.4 developer experience improvements and documentation

### Features Delivered

- [x] Add `air` for hot-reload (API + Worker configs: `.air.api.toml`, `.air.worker.toml`)
- [x] Add `.env` file support (godotenv, 3-layer config: JSON defaults → .env → env vars)
- [x] Add `.env.example` with commonly-changed values
- [x] Rename `config.example.json` → `config.default.json`
- [x] Replace `sleep 3` in Makefile with `pg_isready` healthcheck wait
- [x] Add `.golangci.yml` with sensible defaults
- [x] Remove Wire references from Makefile and install-tools
- [x] Write 10 feature specs (`docs/features/`)
- [x] Write 7 ADRs for key decisions (`docs/adr/`)
- [x] Update seed data with default roles, permissions, and role assignments
- [x] Add UNIQUE INDEX to `casbin_rules` migration for idempotent inserts

---

## v0.5 - Production Hardening (DONE)

### Wave 1

| Feature | PR | Status |
|---------|-----|--------|
| CI/CD pipeline (GitHub Actions: lint, test, build) | #3 | Done |
| Pre-existing lint fixes (errcheck, gocritic, gofmt, staticcheck) | #3 | Done |
| Integration tests (testcontainers-go: Postgres, Redis) | #6 | Done |
| Security headers middleware | #4 | Done |
| Config-driven CORS (production wildcard warning) | #4 | Done |
| JWT hardening (issuer/audience claims) | #4 | Done |
| Input sanitization (bluemonday XSS stripping) | #4 | Done |
| OpenAPI 3.0 spec (34 endpoints documented) | #5 | Done |
| Scalar API reference endpoint (`/docs`) | #5 | Done |
| Production Docker Compose (all services) | #3 | Done |
| Systemd unit files (API + Worker) | #3 | Done |
| Nginx reverse proxy config | #3 | Done |
| Docs CSP fix for Scalar CDN | #7 | Done |

### Wave 2

| Feature | PR | Status |
|---------|-----|--------|
| Database transaction patterns (`DBFromContext`, TX-aware repos) | #8 | Done |
| Audit refactoring with Decorator Pattern (user + auth) | #9 | Done |

---

## v1.0 - Release (DONE)

- [x] README overhaul (quick start, feature list, architecture overview)
- [x] Template-friendly setup (`scripts/setup.sh` for clone-and-rename)
- [x] Changelog (`CHANGELOG.md` covering v0.1.0 through v0.5.0)
- [x] All features documented with specs (10 feature specs in `docs/features/`)
- [x] All features tested (90%+ unit coverage, integration tests for core flows)
- [x] 5-minute onboarding verified (`docs/QUICKSTART.md`)

---

## v1.1 - Pre-Ship Hardening (DONE)

Triggered by the 2026-05-02 pre-ship audit (see `docs/audit/2026-05-02-preship-audit.md`). 14 block-ship + ~30 should-fix findings closed across 11 hardening PRs plus one release-cut PR. Tagged `v1.1.0` on 2026-05-09.

PR slicing plan + status ledger: see `docs/audit/punch-list.md`. Per-PR scope files: `docs/audit/pr-NN-*.md`.

| PR | Title | Status |
|----|-------|--------|
| 1 | Audit fix — context keys + decorators on storage/job + failed-login | Shipped [#13](https://github.com/14mdzk/goscratch/pull/13) |
| 2 | Secure defaults — JWT secret guard, `sslmode=require`, prod stack-trace gate, generic error handler, `/metrics` lockdown | Shipped [#15](https://github.com/14mdzk/goscratch/pull/15) |
| 3 | Auth hardening — logout authn, Casbin fail-fast, refresh-on-NoOp gate, rate-limit fail-closed, iss/aud strict, dual-key revoke | Shipped [#19](https://github.com/14mdzk/goscratch/pull/19) |
| 3b | Authz cache infra — `Authorizer.Start` lifecycle, pluggable watchers (noop/memory/redis), backstop reload tick, incremental load, policy-arg validation | Shipped [#22](https://github.com/14mdzk/goscratch/pull/22) |
| 4 | Shutdown rewrite — `Authorizer` wired + closed, sub-budgets, tracer last, SSE per-conn UUID, worker `wg` covers real work, retry select on ctx | Shipped [#24](https://github.com/14mdzk/goscratch/pull/24) |
| 5 | Storage download streaming + path-prefix guard + content-type sniff | Shipped [#16](https://github.com/14mdzk/goscratch/pull/16) |
| 6 | Pattern alignment — UseCase interfaces for role/storage/job, auth user-repo reuse, Claims to domain, `errors.Is` | Shipped [#27](https://github.com/14mdzk/goscratch/pull/27) |
| 7 | RabbitMQ correctness — per-goroutine channels, `Qos`, NotifyClose reconnect | Shipped [#17](https://github.com/14mdzk/goscratch/pull/17) |
| 8 | SMTP + Postgres rollback context discipline | Shipped [#18](https://github.com/14mdzk/goscratch/pull/18) |
| 9 | Rate-limit hardening — sliding window Redis, trusted-proxy header, memory cleanup stop chan | Shipped [#26](https://github.com/14mdzk/goscratch/pull/26) |
| 10 | Authz decision cache — `subject:obj:act → bool` LRU with explicit invalidation matrix + bench evidence | Shipped [#28](https://github.com/14mdzk/goscratch/pull/28) |
| 11 | Raw-SQL casbin lint guard — CI script rejects writes to `casbin_rule(s)` outside the adapter | Shipped [#25](https://github.com/14mdzk/goscratch/pull/25) |
| 12 | Release cut — v1.1.0 CHANGELOG slice, README/QUICKSTART/ROADMAP doc sync | This PR |

### Cross-cutting themes the audit surfaced

1. Audit feature is broken end-to-end (context-key type mismatch silently empties every audit row).
2. NoOp fallbacks unsafe for security-critical adapters; ADR-006 needs a carve-out for authz / refresh / audit.
3. Shutdown is theater (`wg` does not cover real work, single ctx budget, wrong ordering, Casbin `*sql.DB` never closed).
4. Module-pattern drift (role / storage / job skip the UseCase-interface step that user / auth follow).
5. Default config ships insecure (committed JWT secret, `sslmode=disable`, unconditional stack traces, `/metrics` unauthenticated, nginx TLS commented out).

### Reusable lessons captured to wiki

`~/claude-obsidian/wiki/concepts/`:

- Go Context Key Type Safety
- NoOp Adapter Auth Anti-Pattern
- Go Graceful Shutdown Pattern
- fasthttp Stream Body Lifetime
- RabbitMQ Channel Goroutine Safety
- Sliding vs Fixed Window Rate Limit

---

## v1.2 - Production-Readiness Follow-ups (RELEASE PENDING)

Plan source: [`docs/audit/v1.2-plan.md`](./audit/v1.2-plan.md). Slicing ledger: [`docs/audit/v1.2-punch-list.md`](./audit/v1.2-punch-list.md).

v1.1 closed correctness and security defects in the code. v1.2 closes the gaps surrounding the code: operator runbooks, supply-chain CVE scanning, dependency-drift control, and integration-test coverage left after the v1.1 hardening rush. Three tiers, ~5 working days total. All 10 planned PRs plus the PR-13 follow-up have merged; tag + `[Unreleased]` → `[1.2.0]` cut pending.

### Tier A — Security / CI Hardening

| PR | Title | Status |
|----|-------|--------|
| 13 | Health readiness probe wired (`/healthz/live`, `/healthz/ready`, `/health` alias) | Shipped [#36](https://github.com/14mdzk/goscratch/pull/36) |
| 13b | `port.Queue.Ping` via `QueueDeclarePassive` (no sentinel queue on fresh broker) | Shipped [#45](https://github.com/14mdzk/goscratch/pull/45) |
| 14 | OpenAPI drift sync — `/auth/logout` bearer, upload 415, rate-limited 429 + `RateLimit-*` headers | Shipped [#34](https://github.com/14mdzk/goscratch/pull/34) |
| 15 | `govulncheck` CI job + `make vuln` target | Shipped [#35](https://github.com/14mdzk/goscratch/pull/35) |
| 16 | Dependabot config — gomod + actions + docker, weekly | Shipped [#33](https://github.com/14mdzk/goscratch/pull/33) |

### Tier B — Operator Surface

| PR | Title | Status |
|----|-------|--------|
| 17 | `docs/RUNBOOK.md` — incident playbooks for v1.1 security ops | Shipped [#48](https://github.com/14mdzk/goscratch/pull/48) |
| 18 | Audit-log retention scheduler — external cron pattern + sample container | Shipped [#47](https://github.com/14mdzk/goscratch/pull/47) |
| 19 | Casbin watcher channel versioning — `casbin:policy:update:v1` | Shipped [#44](https://github.com/14mdzk/goscratch/pull/44) |

### Tier C — Test Coverage

| PR | Title | Status |
|----|-------|--------|
| 20 | Auth dual-key revoke integration test (Postgres + Redis testcontainer) | Shipped [#49](https://github.com/14mdzk/goscratch/pull/49) |
| 21 | Casbin watcher e2e test (memory + redis, two enforcers) | Shipped [#51](https://github.com/14mdzk/goscratch/pull/51) |
| 22 | Worker shutdown wg race test (slow-handler + mid-backoff retry) | Shipped [#46](https://github.com/14mdzk/goscratch/pull/46) |

### Tier D — Stack drift (added post-plan)

Surfaced after the v1.2 punch-list was sliced; landed alongside the original tiers.

| PR | Title | Status |
|----|-------|--------|
| 52 | PostGIS-enabled Postgres image + `000004_postgis` extension migration | Shipped [#52](https://github.com/14mdzk/goscratch/pull/52) |
| 53 | Compose platform tag, Redis 7→8.6, `/health/*` → `/healthz/*` OpenAPI sync, dev-default `ssl_mode=disable` + `redis.enabled=true` | Shipped [#53](https://github.com/14mdzk/goscratch/pull/53) |

### Out of scope for v1.2

Inherits v1.1 out-of-scope (circuit breakers, event bus, plugin adapters, gRPC, multi-tenancy) and adds: distributed tracing across services, OAuth provider integration, webhook delivery system. Same overengineer guard from the 2026-05-02 audit.

### After v1.2 PRs land

- Cut `CHANGELOG.md` `[Unreleased]` → `[1.2.0] - <date>`.
- Tag `v1.2.0`.
- Re-link `docs/RUNBOOK.md` from README "Documentation" section (done in PR-17, verify).
- Drain v1.2 follow-up rows F1–F3 (Compose tag drift outside Dependabot; `postgres:17-alpine` → 18 in `internal/platform/testutil/containers.go`; `TestJWTConfig` issuer/audience defaults).

---

## v1.3 - Developer Tooling + Spec Generation + Spatial Primitives (PLANNED)

Theme: take the drift-prone surfaces v1.1/v1.2 audits kept catching (hand-edited OpenAPI spec, module-pattern divergence) and turn them into generated artifacts. Ship spatial **types** that any future module can consume so PR-52's PostGIS dependency is justified without committing this repo to a domain (`location`/`points_of_interest`) it has no first consumer for. Slice picked for smallest blast radius first; each PR ≤ 1 day where possible.

### Tier A — Developer tooling (codegen + scaffold)

The repo already has a canonical module shape (user, auth: handler ↔ usecase interface ↔ domain ↔ port ↔ tests). v1.1 PR-06 ("Pattern alignment") existed only because role/storage/job drifted from it. Solve the drift at write-time, not at audit-time.

| PR | Title | Why |
|----|-------|-----|
| A1 | `cmd/scaffold` + `make new-module name=foo` — generates `internal/module/foo/{handler,usecase,domain,port}.go` plus `_test.go` stubs from a `templates/module/` directory following the user-module pattern (UseCase interface, domain types, port interface, handler bound to interface, table-driven test skeleton) | Canonical pattern is documented in `docs/features/` but enforced only by review. Generator removes the drift class PR-06 cleaned up |
| A2 | `make new-migration name=foo` — emits paired `NNNNNN_foo.up.sql` / `.down.sql` from a template with the next zero-padded sequence number computed from `migrations/` | Today the sequence number is hand-picked; a parallel branch can collide. Generator is one Bash function but worth committing |
| A3 | Scaffold ADR — `docs/adr/008-module-scaffold.md` capturing the canonical layout the templates enforce, so reviewers can point at one source of truth | Template + ADR co-evolve. Without the ADR, future drift just moves into the template |

### Tier B — OpenAPI spec generation (phased)

Spec is hand-maintained in `internal/module/docs/openapi.yaml` (~1.5k lines). PR-14 was a drift-sync PR. PR-53 caught a missed health-route rename. The class of bug repeats every release. Two-phase: cheap drift gate first, real generator second.

| PR | Title | Why |
|----|-------|-----|
| B1 | Route-vs-spec drift CI check — script walks Fiber's registered route table (via `app.Stack()`), diffs against `openapi.yaml` paths + methods, fails CI on missing/extra/renamed entries. Wire into `make lint` and `.github/workflows/ci.yml` | Cheap; catches the PR-14 + PR-53 bug class without committing to an annotation framework yet |
| B2 | Pick generator + prove on one module — evaluate `swaggo/swag`, `go-swagger`, and `danielgtaylor/huma` against this codebase's `fiber` + `apperr.ErrorResponse` shape, write ADR `009-openapi-generation.md` selecting one, migrate the `health` module (smallest surface) as a proof | Don't migrate 34 endpoints blindly; prove the pattern on the smallest module first, capture the rough edges in the ADR |
| B3 | Migrate remaining modules to annotation-driven spec — `auth`, `user`, `role`, `storage`, `sse`, `job`, `docs` — one PR per module, each PR diffs generated spec against hand-edited spec to assert byte-equivalence before deleting the hand-edited block | Reversible per-module; if the generator falls down on one endpoint, that module stays hand-edited and the rest still get the win |
| B4 | OpenAPI `info.version` from `git describe` at build time, `servers:` from env | Final hand-edited fields. After B3 + B4 the spec is fully generated |

### Tier C — Spatial primitives (types only, no domain module)

Goal is to justify the PostGIS dependency PR-52 introduced without committing the repo to a `location` / `points_of_interest` domain it has no consumer for. Ship the **types and adapter glue**; let the first real consumer (in this repo or a downstream fork) define its own table.

| PR | Title | Why |
|----|-------|-----|
| C1 | `internal/shared/domain/geo` — `Point{Lon, Lat float64}`, `BoundingBox`, `Polygon`, `Distance` value types with constructors that validate `lat ∈ [-90,90]`, `lon ∈ [-180,180]`, well-formed ring closure for polygons | Type-level guarantees beat runtime checks scattered across consumers |
| C2 | pgx `Scan` / `Value` round-trip on `geography(Point,4326)` for `geo.Point` — WKB encode/decode helper + table-driven test against `postgis/postgis:18-master` testcontainer | Without this, every consumer reinvents WKB plumbing |
| C3 | GeoJSON `MarshalJSON` / `UnmarshalJSON` on `geo.Point`, `geo.Polygon` — RFC 7946 conformant | Same reason as C2; GeoJSON is the wire format consumers will reach for first |
| C4 | `pkg/geoutil` — pure-Go helpers that do not require Postgres: `Haversine(a, b Point) Distance`, `(BoundingBox).Contains(Point) bool` | Lets callers compute without a round-trip. Postgres remains source of truth; the helpers are for filters before the query |

### Tier D — Surface cleanup deferred from v1.1/v1.2

| PR | Title | Why |
|----|-------|-----|
| D1 | Remove deprecated `GET /health` alias (announced in v1.2 CHANGELOG as "to be removed in a future major") | Carry-cost: handler + route + OpenAPI noise. v1.3 is one release of overlap |
| D2 | Wire `RedisWatcher` in `internal/platform/app/app.go` — currently constructed-but-not-used; channel was pre-versioned in PR-19 anticipating this | Multi-instance deploys silently fall back to backstop-reload tick today |
| D3 | Casbin decision-cache invalidation on `RedisWatcher` update event | PR-10 invalidation matrix assumes local writes; cluster deploys can serve stale `Enforce` results until the backstop reload tick |
| D4 | Drain v1.2 follow-up rows F1–F3 — Compose tag drift not covered by Dependabot (track manually); `postgres:17-alpine` → `postgis/postgis:18-master` in `internal/platform/testutil/containers.go` (closes F2); `TestJWTConfig` issuer/audience defaults (closes F3) | F2/F3 block integration tests on any consumer of C1–C3 |

### Tier E — Observability + auth follow-ons

| PR | Title | Why |
|----|-------|-----|
| E1 | OpenTelemetry trace-context propagation across worker job dispatch — inject W3C `traceparent` on enqueue, extract on dequeue, link spans | Worker-side spans are orphaned today |
| E2 | Refresh-token rotation (re-issue on refresh + revoke previous) | Industry baseline; PR-03 landed dual-key revoke but not rotation |
| E3 | `JWT_SECRET` rotation support — accept both previous + current during a `JWT_SECRET_PREVIOUS` overlap window | RUNBOOK §1 documents the no-overlap rotation as a known sharp edge |
| E4 | Admin paginated audit-log query endpoint (filter by action/user/date) | Audit table is write-only from the API today; RUNBOOK §7 documents direct-SQL as the workaround |

### Tier F — Test + tooling debt

| PR | Title | Why |
|----|-------|-----|
| F1 | Migration up/down round-trip test — apply all up, then all down, assert empty schema | No coverage today; `000004_postgis` regression would only show in prod |
| F2 | k6 load harness — script targeting `/auth/login`, `/files/upload`, `/files/{id}` to baseline req/s + p99 under sliding-window rate-limit | No numbers behind "production-ready"; first benchmark sets the floor |
| F3 | `make lint` includes `goimports -local github.com/14mdzk/goscratch` | Import-grouping drift on every PR |
| F4 | CI coverage drop gate — fail PR if combined coverage drops > 1% from `main` | Coverage drifts down as features outpace tests |

### Out of scope for v1.3

Inherits v1.1/v1.2 out-of-scope (circuit breakers, event bus, plugin adapters, gRPC, multi-tenancy, distributed tracing across services). Adds:

- `location` / `points_of_interest` / any concrete geospatial domain module — Tier C ships **types**, not products. The first consumer defines the table.
- Geo-fencing / geo-routing engine — same reason; primitives, not products.
- Map-tile / vector-tile serving — no frontend in this repo.
- Real-time location streaming over SSE — speculative, design once a consumer exists.
- Multi-region replication / read-replicas — single-region remains the documented target.
- Full code-first replacement of `internal/module/docs/openapi.yaml` in one PR — Tier B is deliberately phased; never delete the hand-edited spec until the generated spec passes byte-diff per module.

---

## v1.4+ - Future Prospects (uncommitted, needs discussion)

Tracked so they are not re-debated each cycle. None are sliced or scheduled. Each needs a design discussion + an ADR before any slicing PR. Promotion criteria: a real first consumer asks, or a v1.x audit surfaces it as a blocker.

| Prospect | Status | Promote when |
|---|---|---|
| **OAuth / OIDC provider integration** | Discussion needed | Real downstream consumer asks; today VISION says JWT is the auth model |
| **Webhook delivery subsystem** | Discussion needed | Event taxonomy exists; today the queue is the de-facto event bus and there is no consumer outside the worker |
| **Multi-tenancy** (schema-per-tenant vs row-level) | Discussion needed | First multi-tenant ask lands with a concrete isolation requirement |
| **gRPC alongside REST** | Out per VISION | A service-to-service consumer materializes; revisit VISION first |
| **Plugin / adapter registry** | Discussion needed | Adapter count crosses a threshold where `app.go` wiring becomes painful (today: ~8 adapters, not painful) |
| **Frontend reference app** | Out of repo scope | If shipped, lives in a sibling repo, not here |
| **Distributed tracing across services** | Discussion needed | Second service exists |
| **Real-time location streaming over SSE** | Discussion needed | Geospatial consumer of v1.3 Tier C primitives exists and asks for it |
