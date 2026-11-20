# ADR-012: Fail-Fast Boot and Explicit NoOp Selection

**Date:** 2026-11-21
**Status:** Accepted
**Amends:** [ADR-006: Adapter Pattern with NoOp Fallbacks](006-adapter-pattern-with-noop.md)

---

## Context

[ADR-006](006-adapter-pattern-with-noop.md) established the NoOp adapter pattern so a fresh clone can run with minimal external dependencies and unit tests need no live services. As originally written, ADR-006 also allowed *silent automatic fallback* to a NoOp (or local) adapter when an enabled service failed to initialize — citing Redis falling back to `NoOpCache` and S3 falling back to `LocalStorage` as graceful degradation.

In production, silent fallback on an explicitly enabled dependency causes severe failure modes:

1. **Silent security bypass or broken auth:**
   - Falling back to `casbin.NoOpAdapter` when `authorization.enabled=true` and Casbin fails to initialize opens every authenticated endpoint to every user (resolved in v1.2 PR-03).
   - Falling back to `cache.NoOpCache` when `redis.enabled=true` and Redis is unreachable leaves the process running with `/auth/login` returning 500s and `ChangePassword` session revocation broken (resolved in PR #157).
2. **Silent data loss or split-brain storage:**
   - Falling back to `queue.NoOpQueue` when `queue.backend=rabbitmq` is unreachable silently drops dispatched background jobs (resolved in v1.3 [ADR-010](010-queue-backend-spectrum.md)).
   - Falling back to `storage.LocalStorage` (and discarding its constructor error) when `storage.mode="s3"` fails to initialize writes uploads onto pod-local ephemeral disk, breaking multi-replica deployments and losing files on restart (resolved in PR #157).
3. **Silent config typos:**
   - Defaulting any unknown `storage.mode` value to local disk masks configuration mistakes (e.g., `STORAGE_MODE=S3` or `gcs`).

## Decision

**Separate intentional opt-out from startup failure.** NoOp and local adapters remain first-class for explicit configuration and unit tests, but an explicitly enabled external dependency must **fail fast at boot** rather than silently downgrading.

### Boot contract by adapter

| Adapter / Service | Explicit Opt-Out (NoOp / Local) | Enabled / Selected & Init Fails |
|---|---|---|
| **PostgreSQL** (`database.*`) | None (always required) | **Fail boot** |
| **Cache / Redis** (`redis.enabled`) | `false` → `NoOpCache` + `SECURITY WARNING` | `true` & unreachable → **Fail boot** |
| **Queue** (`queue.backend`) | `goroutine` → in-process `GoroutineQueue` | `rabbitmq` & unreachable, or invalid backend → **Fail boot** |
| **Storage** (`storage.mode`) | `""` or `"local"` → `LocalStorage` | `"s3"` & init fails (incl. empty bucket), or unknown mode → **Fail boot** |
| **Authorization** (`authorization.enabled`) | `false` → `NoOpAdapter` + warning | `true` & Casbin init or `Start` fails → **Fail boot** |
| **SSE** (`sse.enabled`) | `false` → `NoOpBroker` | In-memory (`NewBroker`); no external dial |
| **Audit** (`audit.enabled`) | `false` → `NoOpAuditor` | Uses main `pgxpool.Pool` (already verified) |
| **Email** (`email.enabled`) | `false` → `NoOpSender` | `true` → `SMTPSender` (dials per send) |
| **Notification** (`notification.enabled`) | `false` → `NoOpSender` | `true` & empty `webhook_url` → **Fail boot** (`cfg.Validate()`) |

### Scoped exceptions (non-fatal warn & continue)

Two auxiliary observability/acceleration hooks are explicitly permitted to warn and continue on startup failure because losing them does not corrupt data or bypass security:

1. **OpenTelemetry tracing (`observability.tracing.enabled`):** if OTLP exporter initialization fails, log a warning and continue with the global no-op tracer provider.
2. **Casbin Redis watcher (`redis.enabled` + `authorization.enabled`):** if subscribing to the policy-invalidation pub/sub channel fails, close the dedicated watcher Redis client, log a warning, and continue relying on Casbin's 5-minute backstop reload.

### Partial-boot resource cleanup

When `initInfra` fails partway through startup (for example, PostgreSQL connects but Redis or S3 fails), every adapter already opened in earlier steps is closed before returning the error so failed boots do not leak connections or file descriptors.

## Consequences

- **Pro:** Misconfigurations and unreachable dependencies surface immediately during deployment rollouts (Kubernetes readiness/crashloop) instead of serving degraded traffic with broken login or pod-local uploads.
- **Pro:** Local development with minimal infrastructure still works via explicit opt-out flags (`REDIS_ENABLED=false`, `AUTHORIZATION_ENABLED=false`, `STORAGE_MODE=local`, `QUEUE_BACKEND=goroutine`).
- **Pro:** Every adapter's NoOp variant remains available for unit testing and explicit disablement.
- **Con / Operator note:** Because `config/config.default.json` enables Redis (`redis.enabled: true`) by default, local runs require either a reachable Redis instance (`make docker-up`) or explicit `REDIS_ENABLED=false`. Deployments with `STORAGE_MODE=s3` must supply a non-empty `S3_BUCKET` and valid AWS/S3 configuration.
