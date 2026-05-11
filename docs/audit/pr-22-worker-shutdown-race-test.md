# PR-22: Worker Shutdown WG Race Regression Tests

| Field | Value |
|-------|-------|
| Branch | `test/worker-shutdown-race` |
| Status | in review |
| Audit source | PR-04 fixed `wg` coverage + retry timer (#14 block-ship); no regression tests were added at the time; a future refactor could silently re-introduce the race |
| Closes | v1.2 punch-list row #22 |

## Goal

Add two regression tests in `internal/worker/worker_test.go` that lock the
block-ship #14 fix from PR-04 and will fail if the fix is ever reverted:

1. **Slow-handler WG test** — asserts `Shutdown` blocks until an in-flight
   handler finishes; catches a regression where `wg.Done()` fires before the
   handler returns.
2. **Mid-backoff cancellation test** — exercises the full
   `handleMessage → retryJob` path and asserts the retry timer exits on
   `ctx.Done()` instead of sleeping the full backoff; catches a regression where
   `retryJob` uses `time.Sleep` instead of a timer + select.

## Tasks

- [x] Read `internal/worker/worker.go`, `worker_test.go`, `publisher_test.go`, and `port/queue.go` to understand the shutdown/wg/retry path.
- [x] Add `dispatchingQueue` helper type (test-only, same file) that calls the handler synchronously inside `Consume` then blocks on `ctx.Done()`, enabling full-path slow-handler testing without modifying production code.
- [x] Add `TestShutdown_WaitsForSlowHandler` — 500ms slow handler, shutdown triggered after handler starts, asserts elapsed ≥ remaining handler sleep and Shutdown does not time out.
- [x] Add `TestRetry_MidBackoff_CancelsOnCtxDone` — always-failing handler at Attempts=2 (→ 9s backoff), shutdown immediately after `handleMessage` returns, asserts Shutdown completes in <500ms and no Publish occurs.
- [x] `make lint test` green (exit 0).
- [x] `CHANGELOG.md` `[Unreleased]` entry under "Testing".
- [x] `docs/audit/v1.2-punch-list.md` row #22 status → `in review`.

## Acceptance Criteria

- `TestShutdown_WaitsForSlowHandler` fails if `wg.Done()` is moved before `handler.Handle()` returns in `consume`.
- `TestRetry_MidBackoff_CancelsOnCtxDone` fails if `retryJob` replaces the `timer + select` with `time.Sleep`.
- `make lint test` exits 0.
- No production (non-test) Go files edited.

## Out of Scope

- Any change to `internal/worker/` non-test `.go` files.
- Coverage of queue channel size or other concurrency primitives.
- testcontainers — these are unit tests with no real broker.
- Any test outside `internal/worker/`.
