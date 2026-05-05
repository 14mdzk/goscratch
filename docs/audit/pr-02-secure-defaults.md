# PR #2 — Secure Defaults

Branch: `feat/secure-defaults`
Closes: block-ship #2, #6, #7, #8, #9 + `/metrics` should-fix from `2026-05-02-preship-audit.md`.
Risk: low. Estimate: ~2h.
Status: pending.

## Goal

The boilerplate ships with a committed JWT placeholder secret, `sslmode=disable`, unconditional Go stack traces on panic, an error handler that echoes the raw error string back to clients, and an unauthenticated `/metrics` endpoint on the public listener. All of those are unsafe defaults for any deployment that forgets the override. This PR makes the defaults safe and forces the operator to opt out explicitly when they want to relax them.

## Findings closed

- **Block-ship #2** — `config/config.default.json:25` and `config/config.json`: JWT secret `"your-super-secret-key-change-in-production"` ships in the repo. Hard-fail at startup if the configured secret matches the placeholder or is shorter than 32 bytes.
- **Block-ship #6** — `internal/platform/http/server.go:35`: `recover.New(EnableStackTrace: true)` is unconditional. Production panics leak the full Go stack to clients. Gate on `cfg.IsProduction()`.
- **Block-ship #7** — `internal/platform/http/server.go:76`: default error handler returns `err.Error()` verbatim. Generic "internal server error" message for 5xx; structured apperr responses preserved; original error logged via `slog`.
- **Block-ship #8** — `internal/adapter/casbin/casbin.go:210`: `BuildDatabaseURL` hard-codes `sslmode=disable`. Thread `cfg.Database.SSLMode` through.
- **Block-ship #9** — `config/config.default.json:19`: default `ssl_mode: "disable"`. Flip to `"require"`; local dev opts out via env override.
- **Should-fix** — `internal/platform/app/app.go:228`: `/metrics` exposed unauthenticated on the main port. Bind to a separate localhost listener on `cfg.Observability.Metrics.Port`.
- **Should-fix** — `deploy/docker/docker-compose.prod.yml:92,176`: migrate container hard-codes `sslmode=disable`; thread the env value the same way the API does.

## Tasks

### 1. JWT secret guard

- [x] **1.1** In `internal/platform/config/config.go`, add a `Validate()` method on `*Config` that returns an error if `cfg.JWT.Secret == "your-super-secret-key-change-in-production"` or `len(cfg.JWT.Secret) < 32`. Error message must mention the env override (`JWT_SECRET`).
- [x] **1.2** Call `cfg.Validate()` from `app.New` before constructing any adapter.
- [x] **1.3** Unit test in `internal/platform/config/config_test.go` covering placeholder rejection, short-secret rejection, and a valid 32+ byte secret pass.

### 2. Default `sslmode=require`

- [x] **2.1** Flip `config/config.default.json:19` `"ssl_mode": "disable"` → `"ssl_mode": "require"`.
- [x] **2.2** Fix `internal/adapter/casbin/casbin.go` `BuildDatabaseURL` to accept and thread an `sslMode` argument. Update existing `BuildDatabaseURL` test to assert the new sslmode value propagates.
- [x] **2.3** Update `deploy/docker/docker-compose.prod.yml:92,176` migrate container so the postgres URL substitutes `${POSTGRES_SSLMODE:-require}` instead of the hard-coded `sslmode=disable`.

### 3. Production stack-trace gate

- [x] **3.1** `internal/platform/http/server.go` `NewServer` now takes `cfg *config.Config` (or at minimum `IsProduction bool`) so `recover.New(EnableStackTrace: !cfg.IsProduction())` becomes possible.
- [x] **3.2** Update the `app.go` call site to pass the new arg.

### 4. Generic error handler

- [x] **4.1** Replace the `defaultErrorHandler` in `internal/platform/http/server.go` with the existing `middleware.ErrorHandler(log)` — it already returns generic messages for unknown errors, preserves apperr responses, and logs via `slog`-compatible logger.
- [x] **4.2** Wire the logger from `app.New` into `http.NewServer` so the middleware can be constructed at server-build time.
- [x] **4.3** Add a regression test in `internal/platform/http/middleware/error_handler_test.go` that asserts a non-apperr error's body does NOT contain the original error string (the existing `TestErrorHandler_UnknownError` is close but does not assert absence).

### 5. `/metrics` lockdown

- [x] **5.1** In `internal/platform/app/app.go`, when `cfg.Observability.Metrics.Enabled`, do NOT register `/metrics` on the public Fiber app. Instead, start a separate `net/http` server bound to `127.0.0.1:<cfg.Observability.Metrics.Port>` serving `promhttp.Handler()`.
- [x] **5.2** Track the metrics server on `App` and shut it down in `App.Shutdown` before the tracer.
- [x] **5.3** Keep `observability.PrometheusMiddleware()` on the public app (it only records metrics; it does not expose them).

### 6. Verification

- [x] **6.1** `make lint` clean.
- [x] **6.2** `make test` clean (unit).

### 7. Docs

- [x] **7.1** Update `docs/features/authentication.md` Configuration section: note that `jwt.secret` is required, must not equal the placeholder, and must be ≥ 32 bytes; startup fails otherwise.
- [x] **7.2** `CHANGELOG.md` `[Unreleased]` entry covering all five items.
- [x] **7.3** PR body must include explicit "operator must change after upgrade" notes for `JWT_SECRET`, `DB_SSL_MODE`, and the metrics port (now bound to localhost; scrape from inside the host / sidecar).

---

## Out of scope (defer to later PRs)

- NoOp-Auditor / Casbin fail-fast carve-out — PR #3 (auth hardening).
- Refresh-token-on-NoOp gate — PR #3.
- Metrics basic-auth alternative — chose localhost listener as simpler; revisit only if a deployment scenario demands remote scrape against the public port.
- nginx TLS turn-on — separate deployment PR.
- Removing the committed `config/config.json` (if present) — handled at repo hygiene level, not here.

## Acceptance

- Booting the API with the committed default JWT secret fails fast with a message naming `JWT_SECRET`.
- `cfg.Database.SSLMode` propagates to the Casbin DB connection and the migrate container.
- A panic in production no longer leaks a Go stack to the client.
- A non-apperr error returns `{"error":{"code":"INTERNAL_ERROR","message":"An internal error occurred"}}` and the original error is in the structured log.
- `curl http://<public>/metrics` returns 404; `curl http://127.0.0.1:9090/metrics` from inside the host returns Prometheus output.
