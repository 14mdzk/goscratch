# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### CI / Tooling

- Added `make vuln` target: installs `govulncheck@v1.3.0` (pinned) and runs `govulncheck ./...` against the module graph. Exits non-zero on any finding.
- Added `vuln` job to `.github/workflows/ci.yml` running in parallel with `lint`, `test`, and `build`. Uses `actions/setup-go@v5` with `go-version-file: go.mod`. Pins `govulncheck@v1.3.0`. Closes v1.2 punch-list row #15.

### Security

- Bumped Go toolchain directive `go 1.25.1` → `go 1.25.10`. Clears 27 stdlib CVEs reachable from application call paths including two `html/template` XSS bypasses (GO-2026-4982, GO-2026-4980), a TLS 1.3 KeyUpdate DoS (GO-2026-4870), and incorrect TLS encryption-level handling (GO-2026-4340). See https://go.dev/doc/devel/release for full release notes.
- Bumped `golang.org/x/net` `v0.50.0` → `v0.53.0`. Fixes HTTP/2 server panic on crafted frames (GO-2026-4559, fixed in v0.51.0) and infinite loop on bad SETTINGS_MAX_FRAME_SIZE (GO-2026-4918, fixed in v0.53.0).
- Bumped `github.com/gofiber/fiber/v2` `v2.52.10` → `v2.52.12`. Patches route-parameter overflow leading to DoS (GO-2026-4543). See https://github.com/gofiber/fiber/releases for release notes. No API or middleware behaviour changes observed.

## [1.1.0] - 2026-05-09 — Hardening

Pre-ship hardening release. Closes the 2026-05-02 audit punch-list (`docs/audit/punch-list.md`): 14 block-ship findings + ~30 should-fix findings shipped across 11 PRs (#13, #15, #16, #17, #18, #19, #22, #24, #25, #26, #27, #28). Several entries below are **breaking** — operators upgrading from v1.0 must read the [Secure-defaults checklist](docs/QUICKSTART.md#secure-defaults-checklist) before booting.

### Changed

- Role module now exposes a `UseCase` interface (`internal/module/role/usecase/port.go`); handler depends on the interface, not the concrete struct.
- Auth module's `NewModule` accepts `usecase.UserRepo` (interface) instead of `*pgxpool.Pool`; `app.go` injects the shared `*userrepo.Repository` created for the user module, eliminating the duplicate repo.
- JWT `Claims` mapped to a domain type (`internal/module/auth/domain/claims.go`); middleware stores `*authdomain.Claims` in context; handlers and usecases no longer depend on `jwt.RegisteredClaims`.
- Sentinel error comparisons (`err == pgx.ErrNoRows`, `err == redis.Nil`) replaced with `errors.Is`; role-usecase internal errors use `apperr.ErrInternal.WithError(err)` to preserve `errors.Is/As` chains.
- Redis rate limiter now uses a **sliding window** algorithm (atomic Lua script: `ZREMRANGEBYSCORE` + `ZCARD` + `ZADD` in one round trip) instead of the previous fixed-window counter. This eliminates the 2× burst that was possible at window boundaries. Existing fixed-window counter keys in Redis are orphaned and will TTL out naturally; rate-limit decisions during the rollout window may differ slightly near window boundaries. Closes punch-list #9 (audit: `rate_limit.go:164`).
- `RateLimit(cfg, cache)` now returns `(fiber.Handler, io.Closer)` instead of `fiber.Handler`. The closer is wired into `app.Shutdown`.

### Added

- `App.Shutdown` is now phased with per-phase deadline budgets (40% HTTP server, 5% metrics, 5% SSE, 10% authorizer, 15% bulk adapters, 10% DB, 15% tracer). Each phase logs its received budget and observed duration. Tracer is the **last** phase so spans emitted by every prior phase still flush. Closes PR-04 task 3.
- `App.Authorizer` field is now actually populated in `app.New` (previously declared but never assigned, leaking the Casbin `*sql.DB` per process restart) and `app.Authorizer.Start(ctx)` is invoked at boot. Closes block-ship #10.
- CI lint that rejects raw-SQL writes to `casbin_rule(s)` outside the casbin adapter (`internal/adapter/casbin/`). Guard is wired into `make lint` and the GitHub Actions lint job. Closes punch-list row #11.
- `server.trusted_proxies` config (`SERVER_TRUSTED_PROXIES`, CSV of CIDR strings) and `server.proxy_header` config (`SERVER_PROXY_HEADER`, default `X-Forwarded-For` when trusted proxies are set). Fiber's `EnableTrustedProxyCheck` is now wired from config so `c.IP()` only reads the proxy header from trusted upstream addresses. Closes punch-list #9 (audit: `rate_limit.go:73`).
- `port.Cache.SlidingWindowAllow` method. `RedisCache` implements it via Lua; `NoOpCache` always allows (in-memory backend is used for single-instance deployments without Redis).
- `memoryBackend.Close()` — stops the janitor goroutine via a stop channel. Idempotent (`sync.Once`). `app.Shutdown` now calls it. Closes punch-list #9 (audit: `rate_limit.go:93`).
- Casbin decision cache (`subject:obj:act → bool`) with LRU eviction and explicit invalidation on every policy mutation. Default capacity is 10 000 entries. Disabled by setting `Config.DecisionCacheSize` to a negative value. Closes punch-list row #10.
- `port.Authorizer` interface gains `Start(ctx context.Context) error` for lifecycle management. All implementors (`Adapter`, `NoOpAdapter`, and test mocks) updated. Closes PR-03b task 1.
- `casbin.Config` extended with `ReloadInterval time.Duration` (0 → 5-minute default) and `Watcher persist.Watcher` (nil = backstop-tick only). Closes PR-03b task 3.
- `casbin.Adapter.Start(ctx)` wires the watcher callback and launches the backstop-reload tick goroutine that calls `LoadPolicy()` every `ReloadInterval`, cancelling on `ctx.Done()`. Closes PR-03b task 4.
- `casbin.NoopWatcher` — `persist.WatcherEx` stub; all methods are no-ops. Useful for single-instance deployments that rely solely on the backstop tick. Closes PR-03b task 5.
- `casbin.MemoryWatcher` — `persist.WatcherEx` backed by a buffered in-process channel (size 64). `UpdateForAddPolicy` / `UpdateForRemovePolicy` send delta messages; `Update` / `UpdateForSavePolicy` / `UpdateForRemoveFilteredPolicy` send full-reload signals. Background goroutine dispatches messages to the registered callback. Closes PR-03b task 6.
- `casbin.RedisWatcher` — `persist.WatcherEx` backed by Redis Pub/Sub on channel `casbin:policy:update` (configurable). Same delta/reload message protocol as `MemoryWatcher`. `Close()` releases the subscription. Closes PR-03b task 7.
- Incremental policy apply in `Adapter.makeUpdateCallback`: decodes the watcher message and calls `enforcer.AddPolicy` / `enforcer.RemovePolicy` etc. instead of always doing `LoadPolicy`. Falls back to full reload for unknown op codes. Closes PR-03b task 8.
- `casbin.validatePolicyArgs` — package-private guard that rejects any policy argument containing null bytes (`\x00`), returning an error wrapping `casbin.ErrInvalidPolicyArg`. Applied in `AddRoleForUser`, `RemoveRoleForUser`, `AddPermissionForRole`, `RemovePermissionForRole`, `AddPermissionForUser`, `RemovePermissionForUser`. Closes PR-03b task 9.
- `docs/features/authorization.md` — documents `Start`, watcher options, backstop tick, incremental load, and input validation. Closes PR-03b docs task.

### Security

- **Breaking.** `/auth/logout` now requires a valid JWT (`Authorization: Bearer <access_token>`). Previously any caller could hit the endpoint. Closes block-ship #5.
- **Breaking.** Login is now **fail-closed** on the cache: if Redis is unavailable or disabled, `/auth/login` and `/auth/refresh` return a 500 error rather than issuing an unrevocable token. Operators must enable Redis (`redis.enabled=true`) for auth to work. A `SECURITY WARNING` is logged at startup when Redis is disabled. Closes block-ship #4.
- **Breaking.** Refresh-token cache now uses a **dual-key design**: a lookup key `refresh:tok:<sha256-hex(token)>` (value: userID) and a per-user index key `refresh:user:<userID>:<sha256-hex(token)>` (value: `1`). Both keys share the same TTL. This replaces the previous single `refresh:user:<userID>:<sha256(token)[:16]>` key shape. Existing refresh tokens issued before this upgrade are invalid and users must re-login.
- **Breaking.** `user_id` is **removed** from the `POST /api/auth/refresh` request body. The server resolves the user ID from the lookup key; no client-supplied hint is needed or accepted. Clients that send `user_id` must remove it (extra JSON fields are ignored, but the field is no longer documented or validated).
- **Breaking.** `POST /api/users/me/password` (ChangePassword) now revokes all active refresh tokens for the user via the auth module's `Revoker.RevokeAllForUser`. On NoOpCache the operation returns an error indicating revocation did not occur. Closes should-fix (user_usecase:ChangePassword).
- **Breaking.** `Config.Validate` now also rejects empty `jwt.issuer` and `jwt.audience`. Operators must set `JWT_ISSUER` and `JWT_AUDIENCE` (or accept the non-empty defaults in `config.default.json`). Closes should-fix (middleware/auth:129).
- `parseToken` now unconditionally validates `iss` and `aud` claims. Previously an empty `JWTIssuer`/`JWTAudience` in the server config silently skipped validation; now it rejects the token. Closes should-fix (middleware/auth:129).
- `/auth/login` and `/auth/refresh` are now protected by a tight per-IP rate limit (20 req / 5 min), fail-closed: on Redis failure the request is rejected rather than allowed through. Closes should-fix (rate_limit:53).
- Casbin init failure when `authorization.enabled=true` is now a hard startup error. The `NoOpAdapter` is no longer used as a fallback; a transient DB blip at boot will not silently open every authenticated endpoint. Closes block-ship #3.
- `NoOpAdapter` (Casbin) is documented as test-only via `internal/adapter/casbin/doc.go`. Do not wire it into production code paths where authorization is expected. Closes block-ship #3 (documentation).
- `port.Cache` interface gains `DeleteByPrefix(ctx, prefix) error`. `NoOpCache.DeleteByPrefix` returns `port.ErrCacheUnavailable` (new sentinel) so callers on auth paths know revocation did not happen. `RedisCache.DeleteByPrefix` uses SCAN + DEL.
- **Breaking.** `app.New` now hard-fails at startup if `jwt.secret` is empty, equals the committed placeholder `your-super-secret-key-change-in-production`, or is shorter than 32 bytes. Operators upgrading must set a real `JWT_SECRET` (≥ 32 bytes) before the API will boot. Closes block-ship #2.
- **Breaking.** `config/config.default.json` default `database.ssl_mode` flipped from `"disable"` to `"require"`. Local dev must set `DB_SSL_MODE=disable` explicitly (Docker Compose dev stack already does). Closes block-ship #9.
- `internal/adapter/casbin.BuildDatabaseURL` now threads the configured `sslMode` instead of hard-coding `disable`; empty value defaults to `require`. Closes block-ship #8.
- `deploy/docker/docker-compose.prod.yml` migrate container reads `${POSTGRES_SSLMODE:-require}` instead of hard-coded `sslmode=disable`.
- Production no longer leaks Go stack traces on panic. `recover` middleware's `EnableStackTrace` is now gated on `!cfg.IsProduction()`. Closes block-ship #6.
- HTTP error handler returns a generic `INTERNAL_ERROR` body for non-`apperr` errors and logs the original via the structured logger. `apperr`-typed responses (developer-chosen messages) are unchanged. Closes block-ship #7.
- `/metrics` no longer registers on the public Fiber listener. A separate `127.0.0.1:<observability.metrics.port>` `net/http` server now serves Prometheus scrapes. Operators must scrape from inside the host (or via a sidecar) instead of the public address. Closes the `/metrics` should-fix.

### Fixed

- SSE broker keyed subscriptions by `userID`; a second tab from the same user silently overwrote the first subscription, leaking the first stream's goroutine forever. The handler now generates a per-connection UUID; the broker keys by this UUID and defensively closes any prior channel on collision so the reader exits its `range` loop. Closes block-ship #11/#12.
- `casbin.Adapter.Close` now stops the backstop reload ticker (via an internal cancel derived from the parent ctx in `Start`), closes the configured watcher, and closes the database handle — idempotent via `sync.Once`. Previously `Close` only closed the DB, leaking the ticker goroutine and watcher subscription. Closes block-ship #10 (lifecycle).
- Worker `wg` now covers the consumer's active window (`consume` blocks on `w.ctx.Done()` after registering with the queue) and the retry goroutine (`wg.Add(1)` before spawn). Retry replaces `time.Sleep` with `time.NewTimer` + `select { <-timer.C / <-w.ctx.Done() }` and re-checks `w.ctx.Err()` before publishing — Shutdown no longer waits for the full backoff and the retry never publishes after `w.cancel()`. Closes block-ship #14.
- Audit log writing empty `user_id` / `ip_address` / `user_agent` for every row. Reader (`port.ExtractAuditContext`) used bare string keys while writers used typed `logger.ContextKey`, so reads never matched writes. A negative regression test in `internal/port/auditor_test.go` locks the bug from coming back.
- File downloads served via `GET /api/files/download/*` were returning empty or truncated bodies because the handler closed the underlying `io.ReadCloser` before fasthttp's `BodyStreamWriter` finished streaming it. The handler now lets the stream writer own the close, matching fasthttp's contract. Regression-locked by `TestDownloadHandler_StreamingLifetime`.
- SMTP `Send` now honours the caller's `ctx` deadline. Previously `internal/adapter/email/smtp.go` called `net/smtp.SendMail`, which has no timeout — a blackhole SMTP server (TCP accepts, never replies) would wedge the worker for the OS TCP timeout (often >2 minutes). The adapter now dials with `net.Dialer.DialContext`, applies the ctx deadline to the conn, and walks the SMTP exchange manually with a cancel-watcher goroutine. A 30s default deadline is applied when the caller did not set one.
- Postgres `Transactor.WithTx` now rolls back with a fresh `context.Background()`-derived ctx (5s timeout) instead of the outer ctx. On shutdown the outer ctx is cancelled, so the previous code returned `context.Canceled` from `tx.Rollback` and the rollback never reached the server, leaving transactions `idle in transaction` until the server-side timeout fired.
- RabbitMQ adapter (`internal/adapter/queue/rabbitmq.go`) shared a single `*amqp.Channel` for publish + consume + retries. AMQP channels are not goroutine-safe and the adapter is now restructured: a cached publisher channel guarded by a mutex, and a dedicated channel per `Consume` call. `channel.Qos(prefetch, 0, false)` is now invoked before `Consume`. A `NotifyClose` reconnect loop with exponential backoff (capped at 30s, 5 attempts) re-establishes the consumer channel on broker drop and exits cleanly on parent context cancellation.
- Leaked janitor goroutine on application shutdown: the in-memory rate-limit store's cleanup goroutine had no exit signal, causing a goroutine leak whenever the application shut down. Closes punch-list #9 (audit: `rate_limit.go:93`).

### Security

- Local storage adapter now enforces a path-prefix guard: any key whose cleaned absolute path would resolve outside the configured base directory is rejected with `ErrPathEscapesBase` before any filesystem call. Defense-in-depth on top of the usecase-layer `..` sanitizer.
- Upload endpoint no longer trusts the client-supplied multipart `Content-Type` header. The first 512 bytes are sniffed via `http.DetectContentType` and validated against the allowlist; mismatches are rejected with HTTP 415 (`apperr.ErrUnsupportedMediaType`). The peeked prefix is re-streamed to the storage adapter so no bytes are lost.

### Added

- `apperr.ErrUnsupportedMediaType` and `apperr.UnsupportedMediaTypef` for HTTP 415 responses.
- Typed context keys `logger.IPAddressKey` and `logger.UserAgentKey`; auth middleware now writes IP and User-Agent into the request context for downstream auditor and logger consumers.
- Audit decorator for the storage module — `Upload` records `CREATE file`, `Delete` records `DELETE file`. Read paths (`Download`, `GetURL`, `List`) are not audited.
- Audit decorator for the job module — `Dispatch` records `CREATE job` with `{job_type, max_retry}` metadata.
- Failed-login audit entries — every `LOGIN` failure records `resource_id = attempted_email`, `metadata.outcome = failed`, and a sanitized `metadata.reason` (`invalid_credentials` / `user_inactive` / `unknown`). Brute-force activity against a single email is now visible.
- Successful-login audit entries now populate `resource_id` with the authenticated user ID (was previously empty).
- `UseCase` interface ports for `internal/module/storage/usecase` and `internal/module/job/usecase` so the decorator pattern can wrap the concrete implementations.
- `rabbitmq.prefetch_count` config (default `10`, env `RABBITMQ_PREFETCH_COUNT`) caps the per-consumer unacknowledged message backlog.

## [0.5.0] - 2026-03-27

### Added

- CI/CD pipeline with GitHub Actions: lint, test, build jobs (#3)
- Integration tests using testcontainers-go for PostgreSQL and Redis (#6)
- Security headers middleware (X-Content-Type-Options, X-Frame-Options, CSP, HSTS) (#4)
- Input sanitization using bluemonday for XSS protection (#4)
- Config-driven CORS with production wildcard warning (#4)
- JWT issuer/audience claims for token hardening (#4)
- OpenAPI 3.0 specification covering all 34 endpoints (#5)
- Scalar API reference endpoint at `/docs` (#5)
- Production Docker Compose with all services, healthchecks, resource limits (#3)
- Systemd unit files for API and Worker (#3)
- Nginx reverse proxy config with TLS placeholder and rate limiting (#3)
- Database transaction patterns with `DBFromContext` and TX-aware repositories (#8)
- Audit decorator pattern for user and auth modules (#9)
- UseCase port interfaces for user and auth modules (#9)
- `make test-ci` and `make test-integration` Makefile targets
- Database package unit tests (Transactor, GetTx, DBFromContext)

### Changed

- Audit logging extracted from usecases into decorator wrappers (#9)
- User repository is now transaction-aware (queries auto-participate in active TX) (#8)
- User `Create` operation wrapped in atomic transaction (email check + insert) (#8)
- Handlers depend on UseCase interfaces instead of concrete types (#9)
- Fixed pre-existing lint errors: errcheck, gocritic, gofmt, staticcheck (#3)
- Removed deprecated linters from `.golangci.yml` (#3)
- User repository tests tagged with `//go:build integration` (#3)

## [0.4.0] - 2026-03-26

### Added

- Air hot-reload configs for API (`.air.api.toml`) and Worker (`.air.worker.toml`)
- `.env` file support via godotenv (3-layer config: JSON defaults -> .env -> env vars)
- `.env.example` with commonly-changed values
- `.golangci.yml` with sensible linting defaults
- 10 feature specification documents (`docs/features/`)
- 7 architecture decision records (`docs/adr/`)
- Default roles (superadmin, admin, viewer) and 17 permissions in seed data
- Default role assignments for seed users
- UNIQUE INDEX on `casbin_rules` for idempotent inserts

### Changed

- Renamed `config/config.example.json` to `config/config.default.json`
- Replaced `sleep 3` in Makefile with `pg_isready` healthcheck wait
- Removed Wire references from Makefile and install-tools
- `make dev` now uses Air for hot-reload

## [0.3.0] - 2026-03-25

### Added

- Role & permission management module (9 endpoints: assign, revoke, list, CRUD permissions)
- File storage module (5 endpoints: upload, download, delete, URL, list)
- SSE HTTP endpoints (3 endpoints: subscribe, broadcast, client count)
- Email service with SMTP adapter and NoOp fallback
- Job publishing API (2 endpoints: dispatch, list types)
- Rate limiting middleware (sliding window, in-memory + Redis backends)
- Email port interface (`port.EmailSender`)
- Email config and rate limit config sections

## [0.2.0] - 2026-03-25

Merged into v0.1.0. All test backfill was done as part of v0.1 completion.

## [0.1.0] - 2026-03-24

### Added

- JWT authentication (login, refresh token, logout) with bcrypt password hashing
- User CRUD with activate/deactivate, password change, soft delete
- Bidirectional cursor-based pagination
- Casbin v3 RBAC with PostgreSQL-backed policy storage
- PostgreSQL audit logging with NoOp fallback
- Redis cache adapter with NoOp fallback
- RabbitMQ queue adapter with NoOp fallback
- File storage adapters (S3 + local filesystem) with NoOp fallback
- In-memory SSE broker with NoOp fallback
- Background job worker with exponential backoff retry
- JWT auth middleware, permission/role middleware
- Request logging, error handler, request ID, CORS middleware
- Prometheus metrics and OpenTelemetry tracing
- Structured logging (slog)
- Application error types, HTTP response helpers, PostgreSQL utilities
- Optional types (Opt, NOpt) for nullable fields
- Input validation with go-playground/validator
- Configuration loading from JSON with environment variable overrides
- Health check endpoint
- Multi-stage Dockerfile for API and Worker
- Docker Compose development environment
- PostgreSQL migrations (users, audit_logs, casbin_rules)
- Database seed script with test users
- Makefile with development commands
- Comprehensive test coverage (23 test suites, 90%+)
