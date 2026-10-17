# Background Jobs

## Overview

Background job processing over a queue tier (`goroutine` by default, `rabbitmq` for durable deployments). An HTTP API allows admins to dispatch jobs, and a worker consumes and executes them. Jobs support retry with capped exponential backoff plus jitter.

## API Endpoints

| Method | Path | Auth | Role Required | Description |
|--------|------|------|---------------|-------------|
| POST | `/api/jobs/dispatch` | JWT | admin | Dispatch a new background job |
| GET | `/api/jobs/types` | JWT | admin | List available job types |

## Request/Response Examples

### POST /api/jobs/dispatch

**Request:**
```json
{
  "type": "email.send",
  "payload": {
    "to": "user@example.com",
    "subject": "Welcome",
    "body": "Hello!"
  },
  "max_retry": 5
}
```

`max_retry` is optional (default: 3).

**Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "type": "email.send",
    "status": "queued",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### GET /api/jobs/types

**Response (200):**
```json
{
  "success": true,
  "data": {
    "types": [
      { "type": "email.send", "description": "Send an email to a recipient" },
      { "type": "audit.cleanup", "description": "Clean up old audit log entries" },
      { "type": "notification.send", "description": "Send a notification to a user" }
    ]
  }
}
```

## Available Job Types

| Type | Description |
|------|-------------|
| `email.send` | Send an email to a recipient |
| `audit.cleanup` | Clean up old audit log entries |
| `notification.send` | Send a notification to a user |

## Worker Processing

The worker runs as a separate binary (`cmd/worker/main.go`) that:

1. Declares the queue (durable) on startup
2. Spawns `concurrency` consumer goroutines
3. Each goroutine calls `queue.Consume` with a callback
4. On message receipt, decodes the `Job` JSON, finds the registered handler, and executes it
5. Each job handler gets a 5-minute context timeout

### Retry Logic

Retries hold the delivery: on a handler failure with retries left, the worker
waits out the backoff in the handler's own goroutine, republishes the job
with `attempts+1`, and only then ACKs — the broker owns the job across the
wait, so a restart mid-backoff loses nothing. If shutdown lands mid-wait the
worker does not republish; it returns an error so the broker redelivers
(`attempts` was never republished, so the next run re-increments to the same
number).

- Delay is capped exponential backoff with jitter: `min(1s * 2^(attempts-1), 60s)`
  scaled by a random factor in `[0.5, 1.0)` (attempt 1 → [0.5s, 1s), converging
  to [30s, 60s)). The 60s cap bounds how long one held delivery can occupy a
  consumer channel (consumers dispatch serially; size `worker.concurrency`
  and `rabbitmq.prefetch_count` accordingly).
- Terminal paths never drop silently: exhausted retries, undecodable
  payloads, and unhandled job types are parked verbatim into `<queue>.dlq`
  (e.g. `jobs` → `jobs.dlq`, declared durable at startup) and then ACKed.
  Parked payloads are the original bytes, so replay is a republish away.
  If the park itself fails the delivery is requeued — except undecodable
  payloads, which ACK after a failed park (a requeue loop on an undecodable
  payload has no attempt counter to stop it). No redrive tooling in v1.3:
  inspect and replay via the broker UI (see `docs/RUNBOOK.md` §9).

### Job Struct

```json
{
  "id": "uuid",
  "type": "email.send",
  "payload": { ... },
  "attempts": 0,
  "max_retry": 3,
  "created_at": "2025-01-15T10:30:00Z"
}
```

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `queue.backend` | `QUEUE_BACKEND` | `""` (unset) | Queue tier: `goroutine`, `rabbitmq`, or `redis` (reserved) |
| `worker.in_process` | `WORKER_IN_PROCESS` | `true` | Consume jobs inside the API process (required with `goroutine`, forbidden with `rabbitmq`) |
| `worker.concurrency` | `WORKER_CONCURRENCY` | `2` | Number of consumer goroutines |
| `worker.queue_name` | `WORKER_QUEUE_NAME` | `jobs` | Queue name |
| `worker.exchange` | `WORKER_EXCHANGE` | `""` | Exchange name (RabbitMQ tier) |
| `rabbitmq.enabled` | `RABBITMQ_ENABLED` | `false` | Deprecated alias: when `queue.backend` is unset, `true` selects `rabbitmq` |
| `rabbitmq.url` | `RABBITMQ_URL` | (none) | RabbitMQ connection URL |
| `rabbitmq.prefetch_count` | `RABBITMQ_PREFETCH_COUNT` | `10` | Per-consumer unacknowledged message limit |

## Backend Spectrum

Three tiers behind `port.Queue` (see ADR-010):

- `goroutine` (default): zero infrastructure. Jobs live in process memory
  and die with the process — best-effort durability. Always consumed
  in-process: the API starts its worker before the HTTP listener, and boot
  fails if the worker cannot start. Nothing to operate.
- `rabbitmq`: durable tier. The broker must be reachable or boot fails (fail
  fast — no silent fallback). Consumed by the external `cmd/worker`
  process; the API never joins consumption as a competing consumer.
- `redis`: reserved enum value. Resolves to a boot error until implemented.

When `queue.backend` is unset, the deprecated `rabbitmq.enabled` alias
applies (`true` → `rabbitmq`, `false` → `goroutine`). Unknown values fail
boot. The old `worker.enabled` key is deleted; unknown JSON keys are
otherwise ignored.

### Operator upgrade notes

- At rest, `rabbitmq.enabled=false` now resolves to `goroutine`: jobs are
  consumed in-process instead of silently dropped. Ensure
  `worker.in_process=true` (the shipped default) or boot fails with the
  corrective setting named.
- `rabbitmq.enabled=true` resolves to `rabbitmq` unchanged.
- Recommended: set `queue.backend` explicitly and drop `worker.enabled` /
  `RABBITMQ_ENABLED` from your config.

## Channel & Reconnect Behavior

The RabbitMQ adapter (`internal/adapter/queue/rabbitmq.go`) follows two rules
that matter for operators:

1. **Channel isolation.** AMQP channels are not goroutine-safe, so the adapter
   keeps a cached publisher channel (guarded by a mutex; used by `Publish`,
   `PublishJSON`, `DeclareQueue`, `DeclareExchange`, `BindQueue`) and opens a
   dedicated channel per `Consume` call. A consumer goroutine never shares a
   channel with another consumer or with the publisher.
2. **Prefetch + bounded reconnect.** Before `Consume`, the adapter calls
   `channel.Qos(prefetch_count, 0, false)` so a single consumer never pulls an
   unbounded backlog into memory. If the consumer channel or underlying
   connection is dropped, the adapter receives `NotifyClose`, closes the dead
   channel, and tries to redial / reopen with exponential backoff capped at
   30s. After 5 failed attempts the consumer halts and the failure is logged;
   the parent context cancellation is honored on every wait so shutdown is
   never blocked by a reconnect loop.
3. **Publisher reconnect.** A separate watcher listens for connection-level
   `NotifyClose` and marks the cached publisher channel dead, so an API-only
   process survives a broker bounce: the next publish redials once and
   retries the publish once, and a failed retry surfaces the original error
   unchanged. No new config keys; the reconnect knobs stay code constants.

## Architecture

- `internal/module/job/` - HTTP handler and usecase for dispatching
- `internal/worker/` - Worker, Publisher, Job, and JobHandler interface
- `internal/worker/handlers/` - Concrete job handler implementations
- The API uses `worker.Publisher` to publish jobs
- The worker uses `worker.Worker` to consume and dispatch to registered `JobHandler` implementations

## Audit Logging

Each job dispatched via the API is recorded in `audit_logs` with `action=CREATE`, `resource=job`, `resource_id=<job_id>`, and metadata `{job_type, max_retry}`. Failed dispatch attempts are not audited (no job exists).

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Queue` | RabbitMQ / NoOp | Job publishing and consuming |
| `port.Authorizer` | Casbin / NoOp | Admin role check on API routes |
| `port.Auditor` | PostgreSQL / NoOp | Dispatch audit logging |
