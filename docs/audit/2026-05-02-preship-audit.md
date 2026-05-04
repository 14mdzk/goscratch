# Pre-Ship Audit — 2026-05-02

Three-pass audit run against `main` after the PR #12 permission-management merge. Goal: decide whether v1.0 is actually ship-ready before reusing this boilerplate on a new project.

**Verdict: do not ship as-is.** 14 block-ship, ~30 should-fix, ~16 nice-to-have.

The boilerplate ships claimed features that are silently broken (audit logging, refresh-token revocation, readiness check), insecure defaults (committed JWT secret, `sslmode=disable`), and a shutdown path that does not cover real work.

---

## Audit Methodology

Three independent passes, read-only:

| Pass | Lens | Subagent |
|------|------|----------|
| 1 | Go architecture / idiomatic correctness, ADR adherence, module-pattern consistency | `everything-claude-code:go-reviewer` |
| 2 | Security posture (OWASP, JWT, file upload, rate limit, CORS, CSP, secrets) | `everything-claude-code:security-reviewer` |
| 3 | Concurrency, goroutine leaks, resource lifecycle, graceful shutdown | general-purpose |

**Overengineer guardrail (applied uniformly):**

- Reject: abstractions without a 2nd consumer, config knobs nobody flips, generic future-proof interfaces, frameworks where stdlib works, defense-in-depth where one layer is already correct, hypothetical nation-state threats.
- Accept: bug / race / leak / panic, security hole, inconsistency with the canonical module pattern (`internal/module/user/`), missing NoOp fallback per ADR-006, spec ↔ code mismatch.

---

## Block-Ship Findings (must fix before any deploy)

### Auth and data integrity

1. `internal/port/auditor.go:70-76` — `ExtractAuditContext` reads `ctx.Value("user_id")` with a plain string key, but `middleware.Auth` writes the value with a typed `logger.ContextKey`. Go context lookups require identical key types. Every audit row therefore stores `user_id=""`, `ip_address=""`, `user_agent=""`. The audit feature listed in the README is non-functional. Fix: read via `logger.UserIDKey`; populate IP and User-Agent in the auth middleware.
2. `config/config.default.json:25` and the committed `config/config.json` — JWT secret `"your-super-secret-key-change-in-production"` ships in the repo. Any deployment that forgets the env override has forgeable tokens. Fix: hard-fail at startup if the secret matches the placeholder or is shorter than 32 bytes.
3. `internal/platform/app/app.go:166` — Casbin init failure silently falls back to `NoOpAdapter`, which permits every `Enforce` call. A transient DB blip at boot opens every authenticated endpoint. Fix: fail-fast.
4. `internal/module/auth/usecase/auth_usecase.go:62` — refresh-token `Set`/`Delete` results discarded with `_ =`. Default config has `RedisEnabled=false`, which selects `NoOpCache`; `Get` always returns `ErrCacheMiss`. Login issues refresh tokens that can never be validated or revoked, and `/auth/logout` is a no-op. Fix: gate refresh-token issuance on cache availability or warn loud at startup.
5. `internal/module/auth/module.go:35` — `/auth/logout` has no auth middleware; any caller can hit it. Fix: require Auth middleware.
6. `internal/platform/http/server.go:35` — `recover.New(EnableStackTrace: true)` unconditional. Production panics leak full Go stack traces to clients. Fix: gate on `IsProduction`.
7. `internal/platform/http/server.go:76` — default error handler returns `err.Error()` verbatim, leaking pgx errors and internal paths. Fix: generic message, log original.
8. `internal/adapter/casbin/casbin.go:210` — `BuildDatabaseURL` hard-codes `sslmode=disable`. Casbin bypasses Postgres TLS even when the main app uses it. Fix: thread `cfg.Database.SSLMode`.
9. `config/config.default.json:19` — default `ssl_mode: "disable"`. Fix: default to `require`; require explicit opt-out for local dev.

### Resource lifecycle

10. `internal/platform/app/app.go:270-318` — `App.Authorizer` field is declared but never assigned in the returned struct literal. The Casbin `*sql.DB` (`casbin.go:21`) is unreachable from `Shutdown`, so it never gets `Close`d. One leaked DB handle per process restart. Also missing `Authorizer.Close()` in the shutdown chain. Fix: assign field, add to shutdown.
11. `internal/adapter/sse/broker.go:42-45` — `Subscribe` keys clients by `userID` and overwrites prior subscriptions without closing the old channel. A user with two SSE tabs leaks the first stream's goroutine forever. Fix: per-connection UUID; close prior channel if collision.
12. `internal/module/sse/handler/sse_handler.go:51` — `clientID == userID` propagates the broker leak. Fix: generate per-request UUID.
13. `internal/module/storage/handler/storage_handler.go:58-71` — `defer reader.Close()` fires when the handler returns, but `c.SendStream(reader)` writes are deferred to fasthttp's `BodyStreamWriter`. The reader is closed before the body is streamed, producing corrupt or empty downloads. Fix: drop the `defer`; fasthttp will `Close` the `io.ReadCloser` after streaming.
14. `internal/worker/worker.go:202-207` and `internal/adapter/queue/rabbitmq.go:88-105` — worker `wg` does not represent real work. `queue.Consume` registers a delivery goroutine and returns immediately, so `wg.Wait()` returns before any handler runs. The retry goroutine (`worker.go:202`) is untracked, ignores `w.ctx`, sleeps for the full backoff delay, and may publish on a closed AMQP channel mid-shutdown. Fix: restructure `Consume` synchronous (or expose a join handle); register retry on `wg`; replace `time.Sleep` with `time.NewTimer` + `select { <-w.ctx.Done() }`.

---

## Should-Fix

### Module-pattern violations (overengineer-safe; aligns to canonical user module)

- `internal/module/role/handler/role_handler.go:13`, `internal/module/storage/handler/storage_handler.go:16`, `internal/module/job/handler/job_handler.go:13` — handlers depend on `*usecase.UseCase` (concrete) instead of a `UseCase` interface. Add `usecase/port.go` per module mirroring user/auth.
- `internal/module/auth/usecase/auth_usecase.go:12` — usecase imports `internal/platform/http/middleware` to use `middleware.Claims`. Layer inversion. Move `Claims` to `internal/module/auth/domain/` (or a token package) and import from there in both usecase and middleware.
- `internal/module/auth/module.go:20` — auth module instantiates a second `userrepo.Repository` for the same pool. Inject the existing user repo from `app.go`.

### Security

- `internal/module/storage/usecase/storage_usecase.go:61` — content-type taken from client multipart header, never verified. Fix: read first 512 bytes, run `http.DetectContentType`, validate against allowlist.
- `internal/platform/http/middleware/rate_limit.go:53` — backend errors fail-open. On Redis failure, `/auth/login` is unlimited. Fix: fail-closed on auth paths; metric on errors.
- `internal/platform/http/middleware/rate_limit.go:164` — Redis backend is fixed-window, not sliding. 2× burst possible at boundary. Fix: Lua sliding-window or sorted-set.
- `internal/platform/http/middleware/rate_limit.go:73` — uses `c.IP()` which trusts X-Forwarded-For by default. Fix: explicit `fiber.Config.ProxyHeader`, only trust behind known proxy.
- `internal/platform/http/middleware/auth.go:129` — empty `iss`/`aud` silently skips validation. Fix: reject token if secret is set and iss/aud are empty.
- `internal/module/auth/usecase/auth_usecase.go:36` — failed logins not audited. Brute-force invisible.
- `internal/platform/app/app.go:228` — `/metrics` exposed unauthenticated on the main port. Fix: bind to localhost or basic auth.
- `internal/module/user/usecase/user_usecase.go:132` — `ChangePassword` does not invalidate refresh tokens. Fix: wildcard delete `refresh:user:<id>:*`.
- `internal/platform/validator/validator.go:102` — `ValidateQuery` skips `SanitizeStruct`. Fix: call sanitizer.
- `internal/adapter/storage/local.go:39` — no `HasPrefix(fullPath, basePath)` guard after `filepath.Join`. Defense-in-depth on path traversal.
- `internal/module/auth/module.go:33-36` — login/refresh routes lack a tighter per-IP rate limit; rely on nginx, but Go-only deployments have only the global 100/60s.
- `deploy/docker/docker-compose.prod.yml:92,176` — migrate container hard-codes `sslmode=disable`; nginx TLS commented out while HSTS is served over HTTP.

### Concurrency / lifecycle

- `internal/adapter/queue/rabbitmq.go:13-16` — single shared `*amqp.Channel` for publish + consume + retries. AMQP channels are not goroutine-safe. Fix: one channel per consumer, one per publisher.
- `internal/adapter/queue/rabbitmq.go:75-83` — no `channel.Qos(prefetchCount, ...)`. Backlog OOMs the worker. Fix: set Qos before Consume.
- `internal/platform/http/middleware/rate_limit.go:93-99` — memory-backend cleanup goroutine has no exit signal. Fix: accept stop chan or context.
- `internal/adapter/email/smtp.go:34-79` — `ctx` ignored; `smtp.SendMail` has no deadline. Blackhole SMTP hangs the worker for OS TCP timeout. Fix: custom dialer with deadline or goroutine + ctx watch.
- `internal/platform/database/postgres.go:84-96` — `tx.Rollback(ctx)` uses the outer (already cancelled on shutdown) ctx. Fix: `context.Background()` with a short timeout.
- `cmd/api/main.go:34,52-58` and `cmd/worker/main.go:55` — `context.Background()` for startup phase, so SIGINT during DB/AMQP dial cannot abort. Single 30s shutdown budget shared across HTTP drain + tracer + 6 adapters; tracer and adapters can receive an already-cancelled ctx. Fix: per-phase sub-budgets; signal-aware startup ctx.
- `internal/platform/app/app.go:301-305` — tracer `Shutdown` runs before adapter `Close` calls; later error logs never get exported. Fix: tracer last.
- `internal/module/sse/handler/sse_handler.go:51-86` — stream goroutine has no `<-c.UserContext().Done()` watcher; relies on `Flush` failing. Fix: select on ctx + heartbeat.
- `internal/adapter/sse/broker.go:60-71` — `Broadcast` silently drops events on slow consumers forever. Fix: counter or evict after N drops.
- `internal/module/health/handler.go:36` — readiness probe is hard-coded `"ok"` with a TODO. K8s routes traffic to dead pods. Fix: real `pgxpool.Ping(ctx)`.

### Idiom

- `internal/module/user/repository/user_repository.go:55,77,213` — `err == pgx.ErrNoRows`. Use `errors.Is`.
- `internal/module/role/usecase/role_usecase.go:33` — `apperr.Internalf("…: %s", err.Error())` severs `errors.Is/As` chains. Use `apperr.ErrInternal.WithError(err)`.

---

## Cross-Cutting Themes

1. **Audit feature is broken end-to-end.** Context keys + missing decorators on storage/job + failed-login gap. Fix as one slice.
2. **NoOp fallbacks are unsafe for security-critical adapters.** ADR-006 says every adapter has a NoOp; the audit shows that for Casbin and refresh-token cache, NoOp silently disables auth controls. Add a carve-out: auth/authz adapters fail-fast or warn loudly.
3. **Shutdown is theater.** `wg` does not cover real work; ctx budget is single-bucketed; ordering is wrong; the Casbin DB is never closed. Whole shutdown path needs one focused PR.
4. **Module-pattern drift.** `role`, `storage`, `job` skip the UseCase-interface step that `user`, `auth` use. Mechanical fix.
5. **Default config ships insecure.** JWT secret, `sslmode=disable`, stack traces, `/metrics` exposure, nginx TLS off. Single secure-defaults PR.

---

## Reference

PR slicing plan: see [`punch-list.md`](./punch-list.md).

ADRs that constrain this audit:

- ADR-001 hexagonal — handler must depend on UseCase interface, not concrete struct.
- ADR-006 NoOp adapters — applies to feature toggles, not to security-critical adapters; this audit recommends a carve-out.
- v0.5 PR #8 (`DBFromContext`) — TX-aware repos.
- v0.5 PR #9 (audit decorator) — audit logic outside usecases.

Reusable lessons captured to wiki at `~/claude-obsidian/wiki/concepts/`:

- Go Context Key Type Safety
- NoOp Adapter Auth Anti-Pattern
- Go Graceful Shutdown Pattern
- fasthttp Stream Body Lifetime
- RabbitMQ Channel Goroutine Safety
- Sliding vs Fixed Window Rate Limit
