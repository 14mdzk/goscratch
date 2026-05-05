# PR #7 — RabbitMQ Correctness

Branch: `refactor/rabbitmq-channel-isolation`
Closes: concurrency should-fix items in `2026-05-02-preship-audit.md` (lines 77-78 + reconnect gap).
Risk: medium. Estimate: ~4h.
Status: in-progress (worktree, not yet shipped).

## Goal

The RabbitMQ adapter (`internal/adapter/queue/rabbitmq.go`) currently shares a
single `*amqp.Channel` across publish, consume, and retry paths. AMQP channels
are not goroutine-safe; concurrent use leads to undefined behavior up to and
including connection-wide protocol errors. The same adapter never sets
`channel.Qos`, so a single consumer can pull the entire backlog into memory
and OOM the worker. There is no `NotifyClose` handler, so a dropped
channel/connection silently halts consumption forever.

This PR makes the adapter goroutine-safe by isolating channels per purpose,
adds prefetch (configurable, default 10), and adds a bounded reconnect loop
that respects parent-context cancellation.

## Findings closed

- **Should-fix** — `internal/adapter/queue/rabbitmq.go:13-16`: single shared
  `*amqp.Channel` for publish + consume + retries. Fix: cached publisher
  channel guarded by mutex, one channel per `Consume` call.
- **Should-fix** — `internal/adapter/queue/rabbitmq.go:75-83`: missing
  `channel.Qos(prefetchCount, 0, false)`. Fix: set Qos on the consumer
  channel before `Consume`; add `rabbitmq.prefetch_count` config (default 10).
- **Should-fix (gap noted in audit)** — no `NotifyClose` reconnect handling.
  Fix: `channel.NotifyClose` triggers a bounded reconnect loop (cap 30s,
  max 5 attempts) that exits on `ctx.Done()`.

## Tasks

### 1. Per-purpose channels

- [x] **1.1** Introduce minimal `amqpConnection` and `amqpChannel` interfaces
      in `internal/adapter/queue/rabbitmq.go` covering exactly the methods the
      adapter uses, so tests can inject a fake transport without touching a
      real broker.
- [x] **1.2** Refactor `RabbitMQ` to hold a cached publisher channel (`pubCh`)
      guarded by a `sync.Mutex`. `Publish`, `PublishJSON`, `DeclareQueue`,
      `DeclareExchange`, and `BindQueue` all funnel through `withPubChannel`.
- [x] **1.3** `Consume` opens its own channel via `q.conn.Channel()` and the
      consumer goroutine never shares a channel with the publisher or another
      consumer. Document the chosen shape (cached publisher + per-call
      consumer) in a comment at the top of the file.

### 2. Qos / prefetch

- [x] **2.1** Add `PrefetchCount int` field to `RabbitMQConfig` with json tag
      `prefetch_count` and env tag `RABBITMQ_PREFETCH_COUNT`.
- [x] **2.2** Default to `10` if unset (handled in `Options.withDefaults`).
- [x] **2.3** Update `config/config.default.json` to set
      `rabbitmq.prefetch_count = 10`.
- [x] **2.4** Call `ch.Qos(prefetchCount, 0, false)` on the consumer channel
      before `ch.Consume`; if `Qos` errors, close the channel and surface the
      error to the caller.
- [x] **2.5** Wire `cfg.RabbitMQ.PrefetchCount` through `NewRabbitMQWithOptions`
      from both `internal/platform/app/app.go` and `cmd/worker/main.go`.

### 3. NotifyClose reconnect

- [x] **3.1** Register `ch.NotifyClose(make(chan *amqp.Error, 1))` on the
      consumer channel. On close, attempt to reopen with exponential backoff
      (`base=1s`, doubling, capped at `30s`); cap retries at 5.
- [x] **3.2** If the underlying connection has died, redial first and replace
      both the cached publisher channel and the connection.
- [x] **3.3** Every wait point selects on `ctx.Done()` so shutdown is not
      blocked by the reconnect loop.
- [x] **3.4** When retries exhaust, log and exit the consumer goroutine — the
      caller (worker) sees consumption stop and the metrics + alerting
      surface picks it up.

### 4. Tests

- [x] **4.1** Internal unit tests in
      `internal/adapter/queue/rabbitmq_internal_test.go` using fake
      `amqpConnection` / `amqpChannel`:
      - Publish and Consume open distinct channels.
      - Qos is called on the consumer channel before Consume with the
        configured prefetch.
      - Default prefetch falls back to 10 when `Options{}` passed.
      - `NotifyClose` triggers a reconnect that opens a new consumer channel.
      - Context cancellation halts the reconnect loop without exhausting
        retries.
      - Close is idempotent; Publish-after-Close errors instead of panicking.
- [x] **4.2** Existing constructor-validation test
      (`TestNewRabbitMQ_InvalidURL_ReturnsError`) still passes.

### 5. Docs

- [x] **5.1** Update `docs/features/background-jobs.md` — add the new
      `rabbitmq.prefetch_count` config row and a "Channel & Reconnect Behavior"
      section.
- [x] **5.2** Add a `[Unreleased]` entry in `CHANGELOG.md` covering the fix
      (under `Fixed`) and the new config knob (under `Added`).

### 6. Verification

- [x] **6.1** `make lint` clean.
- [x] **6.2** `make test` clean.
- [ ] **6.3** `make test-integration` clean (no integration tests under
      `-tags=integration` exist for the queue today; this entry stays
      unchecked because there is nothing new to gate).

---

## Out of scope (defer / route to punch list)

- Circuit breaker around publish failures — listed in punch-list "Out of
  Scope".
- Publisher confirms (`channel.Confirm` + delivery confirms) — separate
  durability work.
- Connection pooling / multi-connection sharding — not justified at current
  throughput.
- Generic event-bus abstraction — explicitly deferred per audit cross-cutting
  notes.

## Acceptance

A reviewer running `make test` sees the fake-transport unit tests pass:
distinct channels for publish vs consume, Qos called before Consume, reconnect
on `NotifyClose`, and ctx-cancel halting the reconnect loop. Operators upgrading
get `rabbitmq.prefetch_count = 10` by default and can tune it via config or
`RABBITMQ_PREFETCH_COUNT` without code changes.
