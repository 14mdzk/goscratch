# Roadmap

## v0.1 - Foundation (PARTIALLY DONE)

Core infrastructure and patterns established. Implementation is largely complete but test coverage is severely lacking.

### Features Implemented

| Feature | Module | Code | Tests | Test Gaps |
|---------|--------|------|-------|-----------|
| JWT Authentication | `auth` | Done | Partial | Handler untested. Usecase only tests components, not full Login/Refresh/Logout flows. No JWT generation/validation tests. |
| User CRUD | `user` | Done | Partial | **Handler 0% tested** (all 9 methods). Usecase missing Update/Activate/Deactivate. Repository missing UpdatePassword/Activate/Deactivate. |
| Bidirectional cursor pagination | `shared/domain` | Done | Good | Missing `NewBidirectionalCursorPage()` dedicated tests |
| RBAC with Casbin | `adapter/casbin` | Done | None | 0% - all methods untested |
| Audit logging | `adapter/audit` | Done | None | 0% - both Postgres and NoOp untested |
| Redis cache | `adapter/cache` | Done | None | 0% - all methods untested |
| RabbitMQ queue | `adapter/queue` | Done | None | 0% - all methods untested |
| File storage (S3 + Local) | `adapter/storage` | Done | None | 0% - all methods untested |
| SSE broker | `adapter/sse` | Done | None | 0% - all methods untested |
| Background job worker | `worker` | Done | None | 0% - 19+ functions untested |
| Email job handler | `worker/handlers` | Stub | None | Stub implementation, no tests |
| Audit cleanup handler | `worker/handlers` | Done | None | 0% - untested |
| JWT auth middleware | `middleware` | Done | None | 0% - untested |
| Permission/role middleware | `middleware` | Done | None | 0% - untested |
| Request logging middleware | `middleware` | Done | None | 0% - untested |
| Error handler middleware | `middleware` | Done | None | 0% - untested |
| Request ID + CORS middleware | `middleware` | Done | None | 0% - untested |
| Prometheus metrics | `observability` | Done | None | 0% - untested |
| OpenTelemetry tracing | `observability` | Done | None | 0% - untested |
| Structured logging | `pkg/logger` | Done | None | 0% - 7 functions untested |
| App error types | `pkg/apperr` | Done | Complete | Well tested |
| HTTP response helpers | `pkg/response` | Done | None | 0% - 11 functions untested |
| PostgreSQL utilities | `pkg/pgutil` | Done | None | 0% - 8 functions untested |
| Optional types (Opt/NOpt) | `pkg/types` | Done | None | 0% - 15+ functions untested |
| Input validation | `platform/validator` | Done | Partial | Only basic Validate() tested. Missing ValidateAndBind, ValidateQuery, HandleValidationError. |
| Config (JSON + env) | `platform/config` | Done | None | 0% - Load, applyEnvOverrides untested |
| Database connection | `platform/database` | Done | None | 0% - untested |
| App bootstrap / DI | `platform/app` | Done | None | 0% - untested |
| Health checks | `health` | Done | Minimal | Only basic health check. Readiness/Liveness untested. Handler barely exists. |
| Docker Compose | `docker-compose.yml` | Done | N/A | - |
| Migrations | `migrations/` | Done | N/A | - |
| Seed data | `scripts/seed` | Done | N/A | - |
| Makefile | `Makefile` | Done | N/A | - |
| Dockerfile | `Dockerfile` | Outdated | N/A | Go version wrong (1.22 vs 1.25). No worker target. |

### Test Coverage Summary (v0.1)

| Category | Total Functions | Tested | Coverage |
|----------|----------------|--------|----------|
| Modules (handler/usecase/repo) | ~42 | ~12 (partial) | ~19% |
| Adapters | ~50 | 0 | 0% |
| Middleware | ~15 | 0 | 0% |
| Worker | ~22 | 0 | 0% |
| Platform | ~15 | ~5 (partial) | ~20% |
| Pkg (utilities) | ~45 | ~12 | ~27% |
| Shared domain | ~8 | ~6 | ~75% |
| **Total** | **~197** | **~35** | **~18%** |

---

## v0.2 - Complete v0.1 (Tests + Fixes)

Before adding any new features, ensure every existing feature is properly tested. No new code until the foundation is solid.

> Rule: Every function with business logic must have tests. Every handler must have HTTP tests.

### 2.1 Auth Module - Complete Tests

- [ ] `auth/handler` - Tests for Login, Refresh, Logout endpoints (request parsing, validation, response format, error cases)
- [ ] `auth/usecase` - Full flow tests: Login (success, bad password, user not found, inactive user), Refresh (success, invalid token, expired), Logout (success, invalid token)
- [ ] JWT generation + validation tests (token expiry, invalid secret, malformed tokens)
- [ ] Audit logging integration in auth flows

**Agent:** `test-automator` + `golang-pro`

### 2.2 User Module - Complete Tests

- [ ] `user/handler` - Tests for ALL 9 handler methods (GetByID, List, Create, Update, Delete, GetMe, ChangePassword, Activate, Deactivate)
- [ ] `user/usecase` - Add missing: Update, Activate, Deactivate tests + audit logging verification
- [ ] `user/repository` - Add missing: UpdatePassword, Activate, Deactivate, Count tests
- [ ] `user/dto` - Request validation tests for all DTOs

**Agent:** `test-automator` + `golang-pro`

### 2.3 Adapter Tests

- [ ] `adapter/cache` - NoOp + Redis tests (use miniredis or interface mock)
- [ ] `adapter/queue` - NoOp + RabbitMQ tests (mock AMQP channel)
- [ ] `adapter/audit` - NoOp + Postgres auditor tests (Log, Query)
- [ ] `adapter/casbin` - NoOp + Casbin adapter tests (Enforce, roles, permissions)
- [ ] `adapter/sse` - Broker tests (Subscribe, Unsubscribe, Broadcast, BroadcastToTopic, SendTo, concurrency)
- [ ] `adapter/storage` - Local tests (with temp dirs) + S3 tests (with mock)

**Agent:** `test-automator` + `golang-pro`

### 2.4 Middleware Tests

- [ ] Auth middleware (valid JWT, expired JWT, missing token, malformed header, cookie extraction)
- [ ] Authz middleware (RequirePermission, RequireRole, RequireAnyPermission, RequireAllPermissions)
- [ ] Error handler (AppError mapping, Fiber error handling, 500 logging)
- [ ] Request ID (generation, propagation)
- [ ] Logger middleware (request/response logging)

**Agent:** `test-automator` + `golang-pro`

### 2.5 Worker Tests

- [ ] `worker` - Worker lifecycle (Start, Shutdown, Stats), job processing, retry with exponential backoff, error handling
- [ ] `worker/job.go` - NewJob, Encode/DecodeJob, UnmarshalPayload, CanRetry, IncrementAttempts
- [ ] `worker/publisher.go` - Publish, PublishWithRetry, PublishRaw
- [ ] `worker/handlers` - AuditCleanupHandler (success, DB failure)

**Agent:** `test-automator` + `golang-pro`

### 2.6 Utility & Platform Tests

- [ ] `pkg/response` - All 11 response helpers (Success, Paginated, Created, Fail, etc.)
- [ ] `pkg/pgutil` - UUID conversion, error detection (duplicate key, FK violation, etc.)
- [ ] `pkg/types` - Opt/NOpt marshal/unmarshal, Get/GetOr, Fiber decoders
- [ ] `pkg/logger` - New, WithContext, WithField, WithError
- [ ] `platform/validator` - ValidateAndBind, ValidateQuery, HandleValidationError, custom validation
- [ ] `platform/config` - Load from JSON, env override precedence, missing file handling
- [ ] `shared/domain` - NewBidirectionalCursorPage dedicated tests

**Agent:** `test-automator` + `golang-pro`

### 2.7 Infrastructure Fixes

- [ ] Fix Dockerfile Go version (1.22 -> 1.25)
- [ ] Add worker build target to Dockerfile
- [ ] Fix health handler (real readiness checks: DB, cache, queue)

**Agent:** `docker-expert` + `golang-pro`

### Target: 90%+ function coverage across all packages

---

## v0.3 - Complete the Starterkit

All missing features. Every feature ships with handler + usecase + tests + feature spec.

> Rule: No feature is "done" without tests and documentation.

### 3.1 Role & Permission Management (Priority: Critical)

Casbin RBAC exists but has no API. Without this, authorization is unusable in production.

- [ ] Feature spec (`docs/features/role-management.md`)
- [ ] `role` module: domain, DTOs, handler, usecase
- [ ] Endpoints: assign/remove role, list roles, manage permissions
- [ ] Unit tests (handler + usecase)
- [ ] Seed data update (default roles + permissions)
- [ ] Migration if needed

**Agent:** `api-designer` + `golang-pro` + `test-automator`

### 3.2 File Upload/Download API (Priority: High)

Storage adapters are ready but have no HTTP layer.

- [ ] Feature spec (`docs/features/file-storage.md`)
- [ ] `storage` module: handler, usecase
- [ ] Endpoints: upload, download, delete, list files
- [ ] File size limits, content type validation
- [ ] Unit tests (handler + usecase)

**Agent:** `api-designer` + `golang-pro` + `test-automator`

### 3.3 SSE HTTP Endpoints (Priority: Medium)

Broker is ready but not wired to routes.

- [ ] Feature spec (`docs/features/sse.md`)
- [ ] SSE handler: subscribe endpoint, event streaming
- [ ] Wire into app bootstrap and routes
- [ ] Unit tests

**Agent:** `golang-pro` + `test-automator`

### 3.4 Email Service Integration (Priority: Medium)

Email handler is a stub.

- [ ] Feature spec (`docs/features/email.md`)
- [ ] Email adapter with port interface (SMTP default)
- [ ] Replace stub with real implementation + NoOp adapter
- [ ] Unit tests

**Agent:** `golang-pro` + `test-automator`

### 3.5 Job Publishing API (Priority: Medium)

Jobs can only be triggered internally.

- [ ] Feature spec (`docs/features/background-jobs.md`)
- [ ] HTTP endpoints to dispatch jobs (admin-only)
- [ ] Unit tests

**Agent:** `api-designer` + `golang-pro`

### 3.6 Rate Limiting Middleware (Priority: Medium)

No request flood protection.

- [ ] Feature spec (`docs/features/rate-limiting.md`)
- [ ] Implementation (Redis-backed + in-memory fallback)
- [ ] Configurable per-route or global
- [ ] Unit tests

**Agent:** `golang-pro` + `test-automator`

---

## v0.4 - Developer Experience

Improve the daily development workflow.

- [ ] Add `air` for hot-reload (API + Worker configs)
- [ ] Add `.env` file support (godotenv, 3-layer config)
- [ ] Add `.env.example` with commonly-changed values
- [ ] Rename `config.example.json` -> `config.default.json`
- [ ] Replace `sleep 3` in Makefile with proper healthcheck wait
- [ ] Add `.golangci.yml` with sensible defaults
- [ ] Remove Wire references from Makefile and install-tools
- [ ] Write feature specs for all features (`docs/features/`)
- [ ] Write ADRs for key decisions (`docs/adr/`)

**Agent:** `docker-expert` + `documentation-engineer`

---

## v0.5 - Production Hardening

- [ ] CI/CD pipeline (GitHub Actions: lint, test, build)
- [ ] Integration tests (Docker-based, database + cache + queue)
- [ ] Security review (input sanitization, CORS hardening, JWT best practices)
- [ ] API documentation (OpenAPI/Swagger spec)
- [ ] Example deployment configs (Docker Compose production, systemd)
- [ ] Database transaction patterns where needed

**Agent:** `code-reviewer` + `docker-expert`

---

## v1.0 - Release

- [ ] README overhaul (quick start, feature list, architecture overview)
- [ ] Template-friendly setup (easy clone-and-rename workflow)
- [ ] Changelog
- [ ] All features documented with specs
- [ ] All features tested (90%+ coverage)
- [ ] 5-minute onboarding verified (clone -> setup -> running API)

**Agent:** `documentation-engineer`
