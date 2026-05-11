# PR-13b: `port.Queue.Ping` via Passive Declare

| Field | Value |
|-------|-------|
| Branch | `refactor/queue-passive-ping` |
| Status | in review |
| Audit source | v1.2-punch-list.md "Follow-up Items (deferred from Tier A)" row: `port.Queue` has no passive ping primitive; queue health checker uses idempotent `DeclareQueue` which creates a sentinel queue on a fresh broker |
| Closes | v1.2 Tier A follow-up (PR-13 deferral) |

## Goal

Replace the queue health probe's "create-and-call-it-a-ping" workaround with a true passive primitive. Adds `Ping(ctx context.Context) error` to `port.Queue`; implements it in `*queue.RabbitMQ` via `QueueDeclarePassive` on a transient channel, treating `404 NOT_FOUND` as broker-reachable success. `*queue.NoOpQueue` returns nil. The readiness probe in `internal/module/health/checker.go` switches from `DeclareQueue("healthz.probe", true)` to `Ping`, so a fresh broker no longer accumulates a durable sentinel queue.

## Tasks

- [x] Add `Ping(ctx context.Context) error` to `internal/port/queue.go` with a contract comment forbidding broker-side state mutation.
- [x] Implement `Ping` on `*queue.RabbitMQ`: open a transient channel (not the cached pubCh), call `QueueDeclarePassive`, treat `amqp.NotFound` as success, propagate other errors.
- [x] Extend the `amqpChannel` test interface with `QueueDeclarePassive` and update the `fakeChannel` fake + add `nextPassiveDeclareErr` injection to `fakeConn`.
- [x] Implement `Ping` on `*queue.NoOpQueue` returning nil.
- [x] Update mock implementations in `internal/module/job/usecase/job_usecase_test.go` (`MockQueue`) and `internal/worker/publisher_test.go` (`mockQueue`) so they still satisfy `port.Queue`.
- [x] Rewrite `queueChecker.Check` in `internal/module/health/checker.go` to call `Ping`. Remove the stale workaround comment.
- [x] Add unit tests covering: (1) transient-channel isolation from cached publisher, (2) `NotFound` treated as healthy, (3) non-`NotFound` AMQP errors propagated, (4) closed connection errors.
- [x] Add `Ping` row to the `TestNoOpQueue_AllMethodsNoOp` table.
- [x] `CHANGELOG.md` `[Unreleased]` entry under "Changed" describing the new primitive and the sentinel-queue cleanup.
- [x] No `docs/audit/v1.2-punch-list.md` row to flip — this is a Tier A follow-up listed in the footer; remove the entry once shipped.

## Acceptance Criteria

- `make lint test` exit 0.
- Health probe no longer creates `healthz.probe` on the broker.
- `Ping` failure on the pubCh side never poisons subsequent `Publish` calls (separate transient channel).
- Mocks across the repo still satisfy `port.Queue`.

## Out of Scope

- Backporting any sentinel-queue cleanup tooling for operators with existing `healthz.probe` durable queues — operators may delete it manually if desired; harmless if left.
- Changes to consumer reconnect / publisher channel caching strategy.
- Renaming or repurposing `healthz.probe` as a real queue.
- Exposing a wider broker-introspection API (e.g., `ListQueues`).

## Operator Note

Existing brokers that already have a `healthz.probe` durable queue from previous releases will retain it. The queue is no longer touched by the health probe and may be deleted via `rabbitmqctl delete_queue healthz.probe` at any time. No operator action is required.
