# Pre-Ship Punch List

Source: [`2026-05-02-preship-audit.md`](./2026-05-02-preship-audit.md). Sliced for smallest blast radius first.

Each PR closes a coherent set of findings. Order chosen so each PR is independently shippable and reviewable in <1 hour where possible.

| PR | Title | Closes | Risk | Est | Status |
|----|-------|--------|------|-----|--------|
| 1 | [Audit fix](./pr-01-audit-fix.md) — context keys, decorators on storage/job, failed-login | Block-ship #1; should-fix audit gaps | low | 2h | ✅ shipped [#13](https://github.com/14mdzk/goscratch/pull/13) |
| 2 | Secure defaults — JWT secret guard, `sslmode=require`, prod stack-trace gate, generic error handler, `/metrics` lockdown | Block-ship #2, #6, #7, #8, #9 + `/metrics` should-fix | low | 2h | pending |
| 3 | Auth hardening — logout authn, Casbin fail-fast, refresh-on-NoOp gate, rate-limit fail-closed, iss/aud strict | Block-ship #3, #4, #5 + 4 should-fix | medium | 4h | blocked by #1 |
| 4 | Shutdown rewrite — `Authorizer` wired + closed, sub-budgets, tracer last, SSE per-conn UUID, worker `wg` covers real work, retry select on ctx | Block-ship #10, #11, #12, #14 + lifecycle should-fix | medium-high | 1d | blocked by #3 |
| 5 | [Storage download streaming + path-prefix guard + content-type sniff](./pr-05-storage-download-streaming.md) | Block-ship #13 + 2 should-fix | low | 3h | implemented in `fix/storage-download-streaming` (worktree, awaiting review) |
| 6 | Pattern alignment — UseCase interfaces for role/storage/job, auth user-repo reuse, Claims to domain, `errors.Is` | Idiom should-fix batch | low | 3h | partial (storage+job ports landed in #1) |
| 7 | RabbitMQ correctness — per-goroutine channels, `Qos`, NotifyClose reconnect | Concurrency should-fix | medium | 4h | pending |
| 8 | SMTP + Postgres rollback context discipline | 2 should-fix | low | 1h | pending |
| 9 | Rate-limit hardening — sliding window Redis, ProxyHeader, memory cleanup stop chan | 3 should-fix | low | 3h | blocked by #3 |

---

## PR Branch Names

```
feat/audit-context-keys
feat/secure-defaults
feat/auth-hardening
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

- Tag `v1.1.0`. Update `CHANGELOG.md` with a "Hardening" section.
- Add `docs/audit/` to README's docs section.
- Add a "secure-defaults checklist" note to `docs/QUICKSTART.md` so anyone cloning the repo for a new project does not accidentally ship the test secret.
