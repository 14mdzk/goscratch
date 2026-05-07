# PR #3b — Authz Cache Infra

Branch: `feat/authz-cache-infra`
Status: ✅ shipped [#22](https://github.com/14mdzk/goscratch/pull/22) (merged 2026-05-06)
Closes: cross-cutting authz lifecycle + perf foundation. Unblocks PR #4 shutdown rewrite.
Risk: medium. Estimate (actual): ~6h.

## Goal

PR #3 makes the authz path correct. PR #3b makes it sustainable:

1. Hide Casbin lifecycle behind `Authorizer.Start(ctx)` so PR #4 can wire boot/shutdown cleanly.
2. Pluggable `persist.WatcherEx` so policy mutations on one pod fan out to every pod's enforcer cache. Drivers shipped: `noop`, `memory`, `redis`.
3. Periodic backstop reload tick (default 5min) to mitigate dropped pubsub events.
4. Casbin `WatcherEx` incremental ops (add/remove single policy) so reloads are O(diff) not O(all-policies). Unknown ops fall back to `LoadPolicy()`.
5. `validatePolicyArgs` rejects null-byte arguments on every mutation entry point.

This PR does **not** add a decision cache (`subject:obj:act → bool`); tracked as punch-list row #10.

## Findings closed

- **Block-ship #10 (foundation only)** — `internal/platform/app/app.go:270-318`: `App.Authorizer` field declared but never assigned. PR #3b provides interface + lifecycle hook (`Start`); PR #4 wires it.
- **Cross-cutting theme #2** — NoOp fallbacks unsafe for security adapters. Locked in via watcher contract.

## Tasks

### 1. `Authorizer.Start` lifecycle

- [x] **1.1** `port.Authorizer` adds `Start(ctx context.Context) error`. All implementors updated.
- [x] **1.2** `casbin.NoOpAdapter.Start` is no-op (`internal/adapter/casbin/noop.go`).
- [x] **1.3** Test: `TestNoOpAdapter_Start` — no panic, returns nil.

### 2. `persist.WatcherEx` drivers

- [x] **2.1** `casbin.Config` extends with `Watcher persist.Watcher` (nil = backstop only) + `ReloadInterval time.Duration` (0 → default 5m).
- [x] **2.2** `watcher_noop.go` — `NoopWatcher` implements `persist.WatcherEx`; all methods return nil; `NewNoopWatcher()` constructor.
- [x] **2.3** `watcher_memory.go` — `MemoryWatcher` channel-bus; `Update` / `UpdateForAddPolicy` / `UpdateForRemovePolicy` / `UpdateForAddPolicies` / `UpdateForRemovePolicies` / `UpdateForSavePolicy` send op messages; background goroutine fires registered callback. `NewMemoryWatcher()` constructor.
- [x] **2.4** `watcher_redis.go` — `RedisWatcher` over `go-redis/v9` Pub/Sub on configurable channel (default `casbin:policy:update`). JSON envelope `{"op","sec","ptype","params"}`. Subscriber goroutine decodes + dispatches. `NewRedisWatcher(ctx, client, channel)` constructor; `Close` calls `pubsub.Close()`.

### 3. Adapter wiring

- [x] **3.1** `casbin.Adapter.Start(ctx)` wires `enforcer.SetUpdateCallback` + `enforcer.SetWatcher`; spawns backstop ticker goroutine (`time.NewTicker(ReloadInterval)`); cancels on `ctx.Done()`.
- [x] **3.2** `makeUpdateCallback` decodes JSON op messages: `add_policy`, `remove_policy`, `add_grouping`, `remove_grouping` apply directly via `enforcer.AddPolicy / RemovePolicy / AddGroupingPolicy / RemoveGroupingPolicy`. Unknown ops → `LoadPolicy()` fallback.

### 4. Incremental policy load

- [x] **4.1** Casbin `WatcherEx` interface satisfied; per-op delta path exercised in tests.
- [x] **4.2** Backstop tick uses full `LoadPolicy` as safety net.

### 5. Input validation guard

- [x] **5.1** `validatePolicyArgs` rejects null bytes (`\x00`) on `AddPermissionForRole`, `RemovePermissionForRole`, `AddPermissionForUser`, `RemovePermissionForUser`, `AddRoleForUser`, `RemoveRoleForUser`. Returns `fmt.Errorf("invalid policy arg %q: %w", arg, ErrInvalidPolicyArg)`.
- [x] **5.2** `ErrInvalidPolicyArg` package-level sentinel.
- [x] **5.3** Test: `TestValidatePolicyArgs`.

### 6. Tests

- [x] **6.1** `TestAdapter_Start_BackstopTick` — verify tick fires `LoadPolicy`.
- [x] **6.2** `TestAdapter_MemoryWatcher_IncrementalAdd` — add policy via watcher, enforcer reflects without full reload.
- [x] **6.3** `TestAdapter_MemoryWatcher_IncrementalRemove`.
- [x] **6.4** `TestRedisWatcher_*` using `miniredis`.
- [x] **6.5** `TestNoOpAdapter_Start`.
- [x] **6.6** `TestValidatePolicyArgs`.

### 7. Docs

- [x] **7.1** `docs/features/authorization.md` — architecture, lifecycle, watcher options, reload semantics, write-path invariant.
- [x] **7.2** `CHANGELOG.md` `[Unreleased]` entry.

### 8. Verification

- [x] **8.1** `make lint` clean.
- [x] **8.2** `make test` clean (race detector on); 11 new tests pass.
- [x] **8.3** All shipped task checkboxes ticked.

---

## Deferred from original scope

These were in the pre-merge plan but moved out:

- **App-level `Authorizer.Start()` + `Close()` wiring** in `app.go` → PR #4 (shutdown rewrite). PR body explicitly notes this.
- **`Authorizer.Close(ctx)` interface method** → PR #4 (when shutdown wiring lands; current shipped surface is `Start` only).
- **Watcher factory registry** (`watcher/registry.go` with `Register(name, factory)`) → simplified out. Drivers constructed directly per config; pluggability achieved via `persist.Watcher` interface.
- **Raw-SQL lint guard** (`scripts/lint-casbin-write-path.sh` enforcing no raw `casbin_rule` SQL outside adapter) → new punch-list row #11.

## Out of scope (still deferred)

- Decision cache `subject:obj:act → bool` — punch-list row #10.
- Casbin role hierarchy migration.
- Replacing Casbin entirely.
- gRPC mgmt API for policy admin.

## Acceptance (post-merge)

- ✅ `port.Authorizer.Start(ctx)` is the lifecycle hook for PR #4 to call.
- ✅ Watcher driver pluggable via injected `persist.Watcher`; nil = backstop tick only.
- ✅ Mutation on pod A propagates to pod B's enforcer via configured watcher driver (memory + redis covered by tests).
- ✅ `validatePolicyArgs` blocks null-byte injection on all six mutation methods.
- ✅ `docs/features/authorization.md` documents lifecycle, drivers, reload semantics.
- ⏭ App-level wiring + `Close` → PR #4.
- ⏭ Raw-SQL lint guard → PR #11 (new row).
