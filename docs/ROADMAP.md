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
