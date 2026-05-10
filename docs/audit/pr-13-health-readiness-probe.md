# PR-13: Health Readiness Probe Wired

| Field | Value |
|-------|-------|
| Branch | `feat/health-readiness-probe` |
| Status | in review |
| Audit source | v1.1 punch-list "Follow-up" entry; `internal/module/health/handler.go:36` `TODO: Add actual readiness checks` |
| Closes | v1.2 punch-list row #13 |

## Goal

`/health` and `/health/ready` currently return `200 ok` regardless of dependency state. Production load balancers and Kubernetes probes cannot distinguish "process running" from "process able to serve traffic". A pod with a dead Postgres connection still receives traffic until manual intervention. Wire real dependency checks behind dedicated readiness/liveness endpoints, with parallel sub-checks under a shared deadline, and keep `/health` as a back-compat alias for liveness.

## Tasks

- [x] Investigate `internal/module/health/`, `internal/platform/app/app.go`, `internal/platform/database/postgres.go`, `internal/adapter/cache`, `internal/adapter/queue` to map current handler shape, route registration, and what each adapter exposes for a passive ping.
- [x] Define `port.HealthChecker interface { Name() string; Check(ctx context.Context) error }` in `internal/module/health/checker.go` (or co-locate inside module package; no port file unless a second consumer appears).
- [x] Implement adapters for: Postgres (`pool.Ping(ctx)`), Redis cache (`Ping` if not NoOp), RabbitMQ queue (passive `QueueDeclarePassive` if not NoOp), Casbin enforcer (`Authorizer != nil` + `GetEnforcer() != nil`). NoOp adapters report `name=cache(noop)` and `Check` returns nil so they don't fail readiness on intentional-disabled deployments.
- [x] Refactor `Handler` to accept `[]HealthChecker` via `NewHandler(checkers ...HealthChecker)`.
- [x] Implement `ReadinessCheck`: run all checkers in parallel via `errgroup` under a 2s context timeout (config knob `Health.ReadinessTimeout` default `2s`). Aggregate per-check results into a map `map[string]string` (`"ok"` or error message). Return `503` if any check fails, `200` otherwise.
- [x] Implement `LivenessCheck`: returns `200` always (process up, no dependency probing).
- [x] Update `internal/module/health/module.go` route registration:
  - `GET /healthz/live` → liveness (new canonical).
  - `GET /healthz/ready` → readiness (new canonical).
  - `GET /health` → liveness alias (back-compat for any existing caller; documented as deprecated in handler comment + CHANGELOG).
  - Remove `GET /health/ready` and `GET /health/live` (the old paths are redundant with the new canonical ones; keep only `/health` alias because external callers may already depend on `200`).
- [x] Wire checkers in `internal/platform/app/app.go`: pass `pool`, `cacheAdapter`, `queueAdapter`, `authorizer` to `health.NewModule(...)`. Skip queue checker when `queueAdapter` is `*queue.NoOpQueue`; same for cache.
- [x] Tests in `internal/module/health/handler_test.go`:
  - Liveness returns 200 with no checkers wired.
  - Readiness returns 200 when all checkers pass.
  - Readiness returns 503 with body listing the failing check name when one checker errors.
  - Readiness returns 503 when context deadline exceeds (slow checker beyond timeout).
  - `/health` alias returns 200 with the liveness payload.
- [x] Update `docs/openapi.yaml` to add `/healthz/live` and `/healthz/ready` paths with 200/503 schemas. (Drift entry — coordinate ordering with PR-14 if both touch openapi.yaml; PR-13 lands first, PR-14 reconciles.)
- [x] Update `docs/features/health.md` (create if missing) describing probe semantics, timeout config, behaviour with NoOp adapters.
- [x] `CHANGELOG.md` `[Unreleased]` entry.
- [x] Update `docs/audit/punch-list.md` and `docs/audit/v1.2-punch-list.md`: row #13 status → `in review` with PR link after lead opens it.

## Acceptance Criteria

- `make lint test` clean.
- `curl :PORT/healthz/live` returns `200 {"status":"alive",...}` even with Postgres stopped.
- `curl :PORT/healthz/ready` returns `503` within ~2s when Postgres is stopped, with body naming `database` as the failed check.
- `curl :PORT/healthz/ready` returns `200` with all checks `ok` on a healthy stack.
- `curl :PORT/health` matches the liveness payload (back-compat path).
- No new exported abstractions beyond `health.HealthChecker` (which has ≥2 internal consumers: Postgres + Redis + Queue + Casbin).
- Readiness response leaks no error internals beyond `<check-name>: <short reason>`; never echoes raw error strings from infra (e.g., DSN).

## Out of Scope

- Persistent metrics for readiness flap rate — tracked in `/metrics` already via Prometheus default Go collectors; no new metric needed.
- Dependency-graph visualisation in the response — operators read JSON, not diagrams.
- Per-check independent timeouts — single shared 2s budget is enough; per-check tuning is a config knob nobody flips.
- HTTP 200 with `degraded` status — strict 200/503 binary keeps load-balancer logic trivial.
- Removing `/health` entirely — would break clients silently; keep as alias, deprecate in CHANGELOG.

## Notes for Implementer

- Do not push or open the PR. Lead reviews diff in worktree before shipping.
- Do not touch any auth, casbin, shutdown, or queue adapter internals beyond reading their public surface.
- Do not add a new package. Keep checkers under `internal/module/health/` unless reuse forces otherwise.
- Do not introduce a config knob unless the plan calls for one (only `Health.ReadinessTimeout` is allowed here).

## Implementation Notes

**Queue checker deviation from scope:** `port.Queue` does not expose a passive
ping primitive and `*queue.RabbitMQ` does not export its internal connection
handle, so `QueueDeclarePassive` cannot be called directly (it is not in the
`amqpChannel` internal interface). The queue checker uses `DeclareQueue` with a
permanent sentinel queue (`healthz.probe`, durable=true) as the closest
available live probe; AMQP `queue.declare` is idempotent for an already-existing
queue with identical parameters. A future punch-list item should add `Ping(ctx)`
to `port.Queue` and `*queue.RabbitMQ` to enable a true passive probe.

**Parallel implementation:** Uses `sync.WaitGroup` + goroutines instead of
`golang.org/x/sync/errgroup` because errgroup stops on first error whereas the
spec requires aggregating ALL check results. The shared deadline context is
passed to every checker goroutine and observed via `ctx.Done()` in each
implementation.
