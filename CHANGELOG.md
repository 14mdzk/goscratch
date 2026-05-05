# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed

- Audit log writing empty `user_id` / `ip_address` / `user_agent` for every row. Reader (`port.ExtractAuditContext`) used bare string keys while writers used typed `logger.ContextKey`, so reads never matched writes. A negative regression test in `internal/port/auditor_test.go` locks the bug from coming back.
- File downloads served via `GET /api/files/download/*` were returning empty or truncated bodies because the handler closed the underlying `io.ReadCloser` before fasthttp's `BodyStreamWriter` finished streaming it. The handler now lets the stream writer own the close, matching fasthttp's contract. Regression-locked by `TestDownloadHandler_StreamingLifetime`.

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
