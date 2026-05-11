# Pre-Ship Punch List

Source: [`2026-05-02-preship-audit.md`](./2026-05-02-preship-audit.md). Sliced for smallest blast radius first.

Each PR closes a coherent set of findings. Order chosen so each PR is independently shippable and reviewable in <1 hour where possible.

| PR | Title | Closes | Risk | Est | Status |
|----|-------|--------|------|-----|--------|
| 1 | [Audit fix](./pr-01-audit-fix.md) — context keys, decorators on storage/job, failed-login | Block-ship #1; should-fix audit gaps | low | 2h | ✅ shipped [#13](https://github.com/14mdzk/goscratch/pull/13) |
| 2 | [Secure defaults](./pr-02-secure-defaults.md) — JWT secret guard, `sslmode=require`, prod stack-trace gate, generic error handler, `/metrics` lockdown | Block-ship #2, #6, #7, #8, #9 + `/metrics` should-fix | low | 2h | ✅ shipped [#15](https://github.com/14mdzk/goscratch/pull/15) |
| 3 | [Auth hardening](./pr-03-auth-hardening.md) — logout authn, Casbin fail-fast, refresh-on-NoOp gate, rate-limit fail-closed, iss/aud strict, dual-key revoke | Block-ship #3, #4, #5 + 4 should-fix | medium | 4h | ✅ shipped [#19](https://github.com/14mdzk/goscratch/pull/19) |
| 3b | [Authz cache infra](./pr-03b-authz-cache-infra.md) — `Authorizer.Start` lifecycle, pluggable `persist.WatcherEx` (noop/memory/redis), backstop reload tick, incremental policy load, `validatePolicyArgs` guard | Cross-cutting (auth lifecycle + perf) | medium | 6h | ✅ shipped [#22](https://github.com/14mdzk/goscratch/pull/22) |
| 4 | [Shutdown rewrite](./pr-04-shutdown-rewrite.md) — `Authorizer` wired + closed, sub-budgets, tracer last, SSE per-conn UUID, worker `wg` covers real work, retry select on ctx | Block-ship #10, #11, #12, #14 + lifecycle should-fix | medium-high | 1d | ✅ shipped [#24](https://github.com/14mdzk/goscratch/pull/24) |
| 5 | [Storage download streaming + path-prefix guard + content-type sniff](./pr-05-storage-download-streaming.md) | Block-ship #13 + 2 should-fix | low | 3h | ✅ shipped [#16](https://github.com/14mdzk/goscratch/pull/16) |
| 6 | [Pattern alignment](./pr-06-pattern-alignment.md) — UseCase interfaces for role/storage/job, auth user-repo reuse, Claims to domain, `errors.Is` | Idiom should-fix batch | low | 3h | ✅ shipped [#27](https://github.com/14mdzk/goscratch/pull/27) |
| 7 | [RabbitMQ correctness](./pr-07-rabbitmq-correctness.md) — per-goroutine channels, `Qos`, NotifyClose reconnect | Concurrency should-fix | medium | 4h | ✅ shipped [#17](https://github.com/14mdzk/goscratch/pull/17) |
| 8 | [SMTP + Postgres rollback context discipline](./pr-08-smtp-and-tx-context.md) | 2 should-fix | low | 1h | ✅ shipped [#18](https://github.com/14mdzk/goscratch/pull/18) |
| 9 | [Rate-limit hardening](./pr-09-rate-limit-hardening.md) — sliding window Redis, trusted-proxy header, memory cleanup stop chan | 3 should-fix | low | 3h | ✅ shipped [#26](https://github.com/14mdzk/goscratch/pull/26) |
| 10 | [Authz decision cache](./pr-10-authz-decision-cache.md) — `subject:obj:act → bool` cache with explicit invalidation matrix + bench evidence | Perf follow-up | medium | 4h | ✅ shipped [#28](https://github.com/14mdzk/goscratch/pull/28) |
| 11 | [Raw-SQL lint guard](./pr-11-casbin-sql-lint.md) — CI script rejects `(INSERT\|UPDATE\|DELETE).*casbin_rule` outside `internal/adapter/casbin/...`; wire into `make lint` + CI | Defense-in-depth (split out of #3b) | low | 1h | ✅ shipped [#25](https://github.com/14mdzk/goscratch/pull/25) |
| 12 | [Release cut](./pr-12-release-cut.md) — v1.1.0 CHANGELOG slice + README/QUICKSTART/ROADMAP doc sync | "After PRs Land" doc gaps | low | 1h | ✅ shipped [#30](https://github.com/14mdzk/goscratch/pull/30) |

---

## PR Branch Names

```
feat/audit-context-keys
feat/secure-defaults
feat/auth-hardening
feat/authz-cache-infra
refactor/shutdown-rewrite
fix/storage-download-streaming
refactor/usecase-port-alignment
refactor/rabbitmq-channel-isolation
fix/smtp-and-tx-context
feat/rate-limit-hardening
```

---

## Acceptance Per PR

Every PR must:

1. Add or update tests covering the changed behavior (no test = no merge).
2. Update `docs/features/<feature>.md` if behavior changes.
3. Add a `CHANGELOG.md` entry under an `[Unreleased]` section.
4. Run `make lint test` clean.
5. For security PRs (#2, #3, #5, #9): include an explicit "what an operator must change after upgrade" note in the PR body.

---

## Out of Scope (deferred / overengineer guard)

- Circuit breakers on cache / queue.
- Generic event bus to replace direct queue calls.
- Plugin system for adapters.
- gRPC alongside REST (explicitly out of scope per `docs/VISION.md`).
- Multi-tenancy.

---

## After PRs Land

- [x] Tag `v1.1.0`. Update `CHANGELOG.md` with a "Hardening" section. — Closed by PR #12 (CHANGELOG cut). Tag pushed by lead post-merge.
- [x] Add `docs/audit/` to README's docs section. — Closed by PR #12.
- [x] Add a "secure-defaults checklist" note to `docs/QUICKSTART.md` so anyone cloning the repo for a new project does not accidentally ship the test secret. — Closed by PR #12.

## Follow-up (out of v1.1 scope)

- Health readiness probe — `internal/module/health/handler.go:36` returns `ok` without pinging dependencies. Track as new punch-list row when v1.2 milestone is opened.
