# PR-09: Rate-Limit Hardening

| Field | Value |
|-------|-------|
| Branch | `feat/rate-limit-hardening` |
| Status | in review (awaiting PR open) |
| Audit source | `docs/audit/2026-05-02-preship-audit.md` |
| Blocked by | PR #3 (auth hardening) ✅ shipped #19 |

---

## Findings Closed

| ID | Location | Description |
|----|----------|-------------|
| should-fix | `internal/platform/http/middleware/rate_limit.go:164` | Redis backend was fixed-window, not sliding. 2× burst possible at boundary. |
| should-fix | `internal/platform/http/middleware/rate_limit.go:73` | `c.IP()` trusted X-Forwarded-For unconditionally, allowing IP spoofing without proxy trust check. |
| should-fix | `internal/platform/http/middleware/rate_limit.go:93` | Memory-backend cleanup goroutine had no exit signal; leaked on shutdown. |

Punch-list row: #9 — "Rate-limit hardening — sliding window Redis, ProxyHeader, memory cleanup stop chan".

---

## Tasks

- [x] Investigate existing rate-limit implementation (fixed-window Redis counter, in-memory sliding window, no stop chan on janitor).
- [x] Implement sliding window for Redis backend — Lua script (`ZREMRANGEBYSCORE` + `ZCARD` + `ZADD`), nanosecond timestamps, atomic, single round trip.
- [x] Add `port.Cache.SlidingWindowAllow` method; implement in `RedisCache` and `NoOpCache`.
- [x] Implement ProxyHeader trust — add `server.trusted_proxies` (CSV, `[]string`) and `server.proxy_header` (string) to `ServerConfig`. Extend `applyEnvOverrides` for `[]string` (CSV split). Wire `fiber.Config.EnableTrustedProxyCheck`, `TrustedProxies`, `ProxyHeader` in `NewServer`. Log warning at boot if `proxy_header` set without `trusted_proxies`.
- [x] Add `stop chan struct{}` + `sync.Once` + `Close()` to `memoryBackend`. Janitor goroutine selects on `ticker.C` and `stop`. Wire `Close()` into `app.Shutdown` via `rateLimitCloser io.Closer`.
- [x] Change `RateLimit()` signature to return `(fiber.Handler, io.Closer)`. Update all call sites (`app.go`, `auth/module.go`).
- [x] Update `testutil/testapp.go` `NewServer` call to pass `isProduction=false` (was missing the arg; integration tests only).
- [x] Fix `mapCache` test fake in `auth/usecase/auth_usecase_test.go` to implement new `SlidingWindowAllow` method.
- [x] Tests:
  - [x] `TestRedisSlidingWindow_AllowsUnderLimit`
  - [x] `TestRedisSlidingWindow_DeniesAtLimit`
  - [x] `TestRedisSlidingWindow_RecoversAfterWindowSlides`
  - [x] `TestNoOpCache_SlidingWindowAllow_AlwaysAllows`
  - [x] `TestProxyHeader_TrustedProxy_UsesXFF`
  - [x] `TestProxyHeader_UntrustedProxy_UsesSocketAddr`
  - [x] `TestMemoryStore_Close_StopsJanitor`
- [x] Update `docs/features/rate-limiting.md`.
- [x] Update `CHANGELOG.md` `[Unreleased]`.
- [x] Write this scope file.

---

## Out of Scope

- New rate-limit policies (per-user tiers, route-specific limits beyond what's already in auth/module.go).
- Distributed lock-based limiter.
- Redis Cluster EVAL → EVALSHA migration.
- Circuit breakers on cache / queue.
- Anything not in the three findings above.
- `make lint` warning about `proxy_header` without `trusted_proxies` in test (test uses `0.0.0.0/0`; that's intentional and documented in the test comment).

### Out-of-Scope Findings (new, not addressed here)

None observed during implementation.

---

## Acceptance

- [x] Tests added for every changed behavior.
- [x] `make lint` clean.
- [x] `make test -race` clean (all packages pass).
- [x] `go vet ./...` clean.
- [x] `docs/features/rate-limiting.md` updated.
- [x] `CHANGELOG.md [Unreleased]` entry added.
- [x] Operator-upgrade notes in PR body (security PR #9 per CLAUDE.md).
