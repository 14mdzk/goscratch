# Health Probes

## Overview

Three endpoints serve Kubernetes-style probes and back-compat callers. All
endpoints are unauthenticated and intended for load balancers and orchestrators.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz/live` | Liveness — process alive, no dependency check |
| GET | `/healthz/ready` | Readiness — all dependency sub-checks run in parallel |
| GET | `/health` | Deprecated alias for `/healthz/live`; kept for back-compat |

Removed (migrated from v1.0/v1.1): `/health/live`, `/health/ready`.

## Liveness (`/healthz/live`)

Returns `200 {"status":"alive","timestamp":"..."}` unconditionally. Never
probes a dependency. Safe to use as a Kubernetes `livenessProbe`.

## Readiness (`/healthz/ready`)

Runs all registered dependency sub-checks in parallel under a shared deadline
(default 2 s, see [Configuration](#configuration)).

**200 response** — all checks pass:

```json
{
  "status": "ready",
  "timestamp": "2026-05-09T12:00:00Z",
  "checks": {
    "database": "ok",
    "cache": "ok",
    "queue(noop)": "ok",
    "authz(noop)": "ok"
  }
}
```

**503 response** — one or more checks fail:

```json
{
  "status": "degraded",
  "timestamp": "2026-05-09T12:00:00Z",
  "checks": {
    "database": "ping failed",
    "cache": "ok",
    "queue(noop)": "ok",
    "authz(noop)": "ok"
  }
}
```

Raw infrastructure error strings (DSNs, stack traces) are never forwarded to
the response. Each check returns only a short sanitised reason (`"ping failed"`,
`"connection unavailable"`, `"enforce failed"`, `"context deadline exceeded"`).

## Sub-checks

| Name | Adapter | Probe method | NoOp behaviour |
|------|---------|--------------|----------------|
| `database` | `*pgxpool.Pool` | `pool.Ping(ctx)` | n/a — always wired |
| `cache` | `port.Cache` | `Exists("__healthz_probe__")` | `cache(noop)` → always ok |
| `queue` | `port.Queue` | `DeclareQueue("healthz.probe", true)` (idempotent) | `queue(noop)` → always ok |
| `authz` | `port.Authorizer` | `EnforceWithContext(probe)` | `authz(noop)` → always ok |

NoOp adapters are detected via type assertions on the concrete adapter types
(`*cache.NoOpCache`, `*queue.NoOpQueue`, `*casbinadapter.NoOpAdapter`). They
report a `(noop)` suffix in the check name and always return nil so intentionally
disabled dependencies do not cause spurious readiness failures.

### Queue probe note

`port.Queue` does not expose a passive ping primitive and `*queue.RabbitMQ` does
not export its internal connection handle. The queue checker uses `DeclareQueue`
with a permanent sentinel queue (`healthz.probe`, durable=true) as the closest
available live probe; AMQP `queue.declare` is idempotent for an already-existing
queue with identical parameters. A future punch-list item should add `Ping(ctx)`
to `port.Queue` and `*queue.RabbitMQ` to enable a true passive probe.

## Configuration

| Env var | JSON key | Default | Description |
|---------|----------|---------|-------------|
| `HEALTH_READINESS_TIMEOUT_SEC` | `health.readiness_timeout_sec` | `2` | Total deadline (seconds) for all parallel readiness sub-checks |

Zero or negative values fall back to the 2 s default.

## Kubernetes probe configuration example

```yaml
livenessProbe:
  httpGet:
    path: /healthz/live
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 3
```

## Architecture

```
internal/module/health/
  checker.go   — HealthChecker interface + Postgres / cache / queue / authz adapters
  handler.go   — Handler.LivenessCheck, Handler.ReadinessCheck
  module.go    — Route registration (/healthz/live, /healthz/ready, /health alias)
```

Wire-up is in `internal/platform/app/app.go` at the `health.NewModule(...)` call
site, which receives the live `*pgxpool.Pool`, `port.Cache`, `port.Queue`, and
`port.Authorizer` instances from the parent `New(ctx, cfg)` function.
