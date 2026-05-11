# PR-21: Casbin watcher e2e test (memory + redis, two enforcers)

| Field | Value |
|-------|-------|
| Branch | `test/casbin-watcher-e2e` |
| Status | in review |
| Audit source | No test exercises the watcher → enforcer notification loop end-to-end (PR-03b shipped the watcher infrastructure without coverage) |
| Closes | v1.2 punch-list row #21 |

## Goal

Add end-to-end tests for the Casbin watcher notification loop shipped in PR-03b.
Two logical `Adapter` instances share one watcher. Mutate policy on instance A;
assert instance B sees the change via the incremental-apply path — not the
full-reload backstop. Cover both `MemoryWatcher` (single-instance, in-proc
broadcast) and `RedisWatcher` (multi-instance, pub/sub via miniredis).

## Tasks

- [x] Read `watcher_memory.go`, `watcher_redis.go`, `casbin.go`, `casbin_test.go`
  to understand the existing Adapter API, Watcher port, and `encodeOp` envelope.
- [x] Write `watcher_e2e_test.go` with five tests:
  - `TestWatcherE2E_Memory` — add-policy propagated via `MemoryWatcher`.
  - `TestWatcherE2E_Memory_Remove` — remove-policy propagated via `MemoryWatcher`.
  - `TestWatcherE2E_Redis` — add-policy propagated via `RedisWatcher` (miniredis).
  - `TestWatcherE2E_Redis_Remove` — remove-policy propagated via `RedisWatcher`.
  - `TestWatcherE2E_Redis_IsolatedChannels` — pair2 is unaffected by pair1 messages.
- [x] Backstop tick set to 24 h (`backstopSafe`) so it cannot fire during the
  test — ensures only the watcher path drives propagation.
- [x] Verified `make lint test` exits 0 with full race detector (`go test -race`).
- [x] Updated `CHANGELOG.md` `[Unreleased]` under "Testing".
- [x] Updated `docs/audit/v1.2-punch-list.md` row #21 status → `in review`.

## Design Notes

### Thread-safety
Casbin's plain `Enforcer` is not internally thread-safe. A naive "two full
`Adapter` instances sharing one `RedisWatcher`" design causes a data race: both
the watcher's subscriber goroutine (writes the enforcer via callback) and the
test goroutine (reads via `Enforce()`) access the same enforcer concurrently.

The fix uses a "publisher/subscriber split":
- **A (publisher)**: a full `Adapter` with a real watcher (for MemoryWatcher) or
  a `NoopWatcher` + explicit `publishOp` call (for RedisWatcher). Only A's
  enforcer is written by the test goroutine.
- **B (subscriber)**: a bare `casbinlib.Enforcer` with no `SetWatcher` call. B's
  callback is registered directly on the `notifyWatcher` shim via
  `SetUpdateCallback`. Because B's enforcer has no watcher, `Enforce.AddPolicy`
  inside the callback does NOT re-publish — this eliminates the cascade.

### `notifyWatcher` shim
Wraps a real `WatcherEx` and signals a `chan struct{}` after each callback
invocation. Tests call `waitForUpdate(t)` to block until the callback write
completes before reading B's enforcer — eliminating the concurrent-access window.

`notifyWatcher` implements `persist.WatcherEx` (not just `persist.Watcher`) so
that `enforcer.SetWatcher` detects it as WatcherEx and skips the generic
`SetUpdateCallback(func(string){LoadPolicy()})` call.

### `applyOp` helper
An in-package (same package as `casbin`) helper that mirrors `makeUpdateCallback`
but operates on a bare `*casbinlib.Enforcer` without setting a watcher. Used as
B's callback in all e2e tests.

### RedisWatcher channel
Tests use `casbin:policy:update:v1` — the channel name from PR-19 (versioned).
The isolated-channel test uses `pair1` / `pair2` sub-channels.

## Acceptance Criteria

- [x] Test fails if B never sees the policy change within the deadline.
- [x] Test fails if the change can only be seen via full-reload (backstop disabled).
- [x] Both `MemoryWatcher` and `RedisWatcher` covered (add + remove).
- [x] `make lint test` exits 0 (including `-race`).

## Out of Scope

- Wiring `RedisWatcher` into `internal/platform/app/app.go`.
- Adding a new watcher implementation.
- Casbin upstream test fixtures.
- Any non-test `.go` file under `internal/adapter/casbin/`.
