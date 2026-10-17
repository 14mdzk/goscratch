# Lifecycle: boot, shutdown, and per-phase budgets

This document describes how `App.New` brings dependencies online and how `App.Shutdown` brings them back down deterministically. The contract here is load-bearing for two reasons:

1. Casbin holds its own `*sql.DB` and a backstop-reload ticker; both must be closed on shutdown or every process restart leaks them.
2. Multiple goroutines (HTTP server, SSE streams, worker consumers holding retry backoffs, the tracer's batch span exporter) emit work that depends on resources owned by other phases. Shutdown order matters: close downstream emitters first, sinks last.

## Boot order (`app.New`)

`app.New(ctx, cfg)` constructs adapters in the following order. Failures abort boot — this is intentional. Silent fallback to a no-op adapter for a security-critical path (authz, cache for refresh-token revocation) was a block-ship finding in the pre-ship audit and has been removed.

1. `cfg.Validate()` — secure-defaults invariants (JWT secret length, `iss`/`aud` non-empty, etc.).
2. Logger.
3. Tracer (`observability.InitTracer`) — only if `observability.tracing.enabled`.
4. Database pool (`pgxpool`).
5. Cache (Redis or `NoOpCache` warning).
6. Queue (RabbitMQ or `NoOpQueue`).
7. Storage (S3 or local).
8. SSE broker.
9. Auditor.
10. **Authorizer** — Casbin `Adapter` if `authorization.enabled=true`, otherwise `NoOpAdapter`. The `Authorizer` is then `Start`ed (`Adapter.Start` wires the `persist.Watcher` callback and launches the backstop reload ticker).
11. Email sender.
12. HTTP server + middleware + module registration.

`Authorizer.Start(ctx)` is the lifecycle hook introduced in PR-03b. The backstop ticker derives an internal cancel context from the parent so `Authorizer.Close` can stop the goroutine even when the parent ctx is still alive.

## Shutdown phases

`App.Shutdown(parent)` runs each phase under its own ctx derived from the parent deadline. Phase budgets are fractions of the total budget; if the parent has no deadline, a default of 30s is used.

| # | Phase        | Fraction | What runs                                                  | Why this position                                                                                  |
|---|--------------|---------:|------------------------------------------------------------|----------------------------------------------------------------------------------------------------|
| 1 | `http_server`|     0.30 | `Server.Shutdown(ctx)`                                     | Drain in-flight HTTP requests first; clients may be mid-stream.                                    |
| 2 | `worker`     |     0.15 | `inProcessWorker.Shutdown(ctx)` (nil unless in-process)    | Drain queued jobs right after the listener stops: held retry waits abort on ctx-cancel and redeliver. |
| 3 | `metrics`    |     0.05 | `metricsServer.Shutdown(ctx)`                              | Internal listener; fast.                                                                           |
| 4 | `sse`        |     0.05 | `SSE.Close()`                                              | Close subscriber channels so `range` loops exit before downstream adapters yank.                  |
| 5 | `authorizer` |     0.10 | `Authorizer.Close()` — cancels ticker, closes watcher + DB | Before the main DB pool so an in-flight policy reload cannot race a shutting-down pool.            |
| 6 | `adapters`   |     0.10 | `Cache.Close`, `Queue.Close`, `Storage.Close`, `Auditor.Close`, `Email.Close` (+ watcher Redis client) | Bulk batch; none of these accept a ctx, so the phase budget bounds them. |
| 7 | `database`   |     0.10 | `DB.Close()`                                               | Last database-using thing closed.                                                                  |
| 8 | `tracer`     |     0.15 | `tracerShutdown(ctx)`                                      | **Last.** Spans emitted by every prior phase still flush through it.                               |

Each phase logs `phase`, `duration_ms`, and `budget_ms`. Failure in one phase is logged but does not abort subsequent phases — best-effort cleanup is the goal.

### Why tracer last

The OTel batch span processor flushes on `Shutdown`. If we close the tracer before the DB / cache / queue / authorizer phases, any spans those phases produce (timeouts, errors during their own `Close`) are dropped. The previous shutdown closed the tracer in the middle of the sequence; PR-04 reordered it to the tail.

### Why authorizer before DB

`Authorizer.Close` closes the Casbin `*sql.DB` it owns, but the enforcer can also synchronously reload from the main `pgxpool` if a watcher event arrives mid-shutdown. Closing the authorizer first stops the watcher subscription and ticker so no `LoadPolicy` is in flight when the application pool closes.

## SSE per-connection UUID

`Broker.Subscribe(clientID, topics...)` keys subscriptions by `clientID`. The handler now generates `connID := uuid.NewString()` per request. Pre-fix, the handler passed `userID`, so a second tab from the same user silently overwrote the first subscription, leaking the first stream's goroutine forever. The broker also defensively closes the prior channel on collision (UUIDv4 should never collide; this is defense-in-depth).

`Unsubscribe(connID)` is what the handler calls when the SSE stream ends — never `Unsubscribe(userID)`.

## Worker shutdown

`worker.Shutdown(ctx)` cancels the worker's internal context (`w.cancel()`) and waits on `w.wg`. Two things were broken pre-PR-04:

1. `consume(workerID)` called `queue.Consume(ctx, ...)` — RabbitMQ's `Consume` registers a delivery goroutine and returns immediately, so `wg.Done` fired before any handler ran. `consume` now blocks on `<-w.ctx.Done()` after `Consume` registers, so `wg` actually represents an active consumer.
2. Retries used to spawn an untracked `retryJob` goroutine that `time.Sleep`ed the backoff and then `Publish`ed — on shutdown it kept sleeping and could publish on a closed channel after the queue adapter's `Close`. Retries now hold the delivery inside the handler goroutine (`holdForRetry` with `time.NewTimer` + `select { <-timer.C / <-w.ctx.Done() }`, re-checking `w.ctx.Err()` before `Publish`): no retry goroutines exist, so shutdown never waits out a backoff. A shutdown-cancel mid-wait returns an error without republishing, so the adapter Nacks and the broker redelivers with the same attempt number.

`wg.Wait()` therefore waits only for the consumer windows; every held retry inside them aborts on ctx-cancel.

## Operator guidance

- Set `server.shutdown_timeout` (or pass a deadline to your `Shutdown(ctx)` call) high enough for the longest expected drain. Default budget is 30s if no deadline is supplied.
- Each phase's budget is a fraction; the HTTP server gets the largest slice (30%), followed by the in-process worker drain (15%). If your traffic patterns include long-poll endpoints or large in-flight uploads, consider raising the total deadline rather than reshuffling fractions.
- The phased Shutdown is best-effort: a failed phase logs the error but does not abort. Watch the structured `shutdown phase failed` and `shutdown phase complete` log lines to spot regressions.
