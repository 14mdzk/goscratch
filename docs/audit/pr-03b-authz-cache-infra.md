# PR #3b — Authz Cache Infra

Branch: `feat/authz-cache-infra`
Closes: cross-cutting authz lifecycle + perf foundation. Unblocks PR #4 shutdown rewrite (Authorizer must be wired+closeable) and punch-list row #10 (decision cache).
Risk: medium. Estimate: ~6h.
Status: blocked by PR #3.

## Goal

PR #3 makes the authz path correct. This PR makes it sustainable:

1. Hide Casbin behind an `Authorizer` interface with an explicit lifecycle (`Close`) so PR #4 can wire shutdown cleanly.
2. Introduce a pluggable `persist.Watcher` so policy mutations on one pod fan out to every pod's enforcer cache (observer pattern). Drivers: `noop`, `memory`, `redis`. Operators or downstream forks can register custom drivers.
3. Add a periodic backstop reload tick to mitigate dropped pubsub events.
4. Use Casbin's incremental policy load (`UpdateForAddPolicy`, `UpdateForRemovePolicy`, etc) so reloads are O(diff) not O(all-policies) once the watcher contract is in place.
5. Add a CI lint guard: any raw `casbin_rule` SQL outside the adapter package fails the build, so the enforcer-API-as-single-write-path invariant holds.

This PR does **not** add a decision cache (subject:obj:act → bool); that needs benchmark evidence and an explicit invalidation matrix and is tracked as punch-list row #10.

## Findings closed

- **Block-ship #10 (foundation only)** — `internal/platform/app/app.go:270-318`: `App.Authorizer` field declared but never assigned. PR #4 fixes wiring; this PR provides the interface + lifecycle for that wiring to land cleanly.
- **Cross-cutting theme #2** — NoOp fallbacks unsafe for security adapters. Locked in here via the lint guard + watcher contract.

## Tasks

### 1. `Authorizer` interface + concrete

- [ ] **1.1** Define `internal/port/authorizer.go`:
  ```go
  type Authorizer interface {
      Enforce(ctx context.Context, sub, obj, act string) (bool, error)
      AddPolicy(ctx context.Context, params ...string) (bool, error)
      RemovePolicy(ctx context.Context, params ...string) (bool, error)
      AddRoleForUser(ctx context.Context, user, role string) (bool, error)
      DeleteRoleForUser(ctx context.Context, user, role string) (bool, error)
      LoadPolicy(ctx context.Context) error
      Close(ctx context.Context) error
  }
  ```
- [ ] **1.2** Move concrete Casbin enforcer construction into `internal/adapter/casbin/authorizer.go`, returning `port.Authorizer`. Internally holds `*casbin.SyncedEnforcer` + the watcher.
- [ ] **1.3** Update every call site that imports Casbin directly (handlers, middleware) to depend on `port.Authorizer`.
- [ ] **1.4** Test: `internal/adapter/casbin/authorizer_test.go` — basic enforce, add/remove policy, role grant/revoke, idempotent `Close`.

### 2. `persist.Watcher` factory + drivers

- [ ] **2.1** Add `internal/adapter/casbin/watcher/registry.go`:
  ```go
  type Factory func(cfg Config) (persist.Watcher, error)
  func Register(name string, f Factory) { ... }
  func Build(cfg Config) (persist.Watcher, error) { ... }
  ```
  `Config` carries `Driver string`, `Redis RedisConfig`, plus opaque map for custom drivers.
- [ ] **2.2** Driver `noop`: `Update` no-op, `SetUpdateCallback` stores callback but never fires. For single-pod dev with no fan-out need.
- [ ] **2.3** Driver `memory`: in-process `chan struct{}` bus. `Update` publishes, callback fires asynchronously. For tests + single-binary deploys (so callback path is exercised).
- [ ] **2.4** Driver `redis`: thin wrapper over the existing Redis client publishing/subscribing on a configurable channel (`authz.casbin.policy` default). On `Close`, unsubscribe and close subscriber goroutine.
- [ ] **2.5** Wire driver selection via `cfg.Authz.Watcher.Driver` (new section under `internal/platform/config/config.go`).
- [ ] **2.6** Test per driver: `Update` triggers `SetUpdateCallback`'s callback; `Close` is idempotent and stops goroutines (verified via `goleak` or counted-goroutine assertion).
- [ ] **2.7** Test: factory rejects unknown driver name with explicit error.

### 3. Lifecycle wiring

- [ ] **3.1** Watcher constructed in `app.New` before authorizer; passed into authorizer constructor; authorizer calls `enforcer.SetWatcher(w)` and `w.SetUpdateCallback(func(string){ _ = enforcer.LoadPolicy() })`.
- [ ] **3.2** `Authorizer.Close` calls `watcher.Close` then enforcer's underlying `*sql.DB.Close` if owned. Idempotent.
- [ ] **3.3** Mutation write path: every mutation goes through `Authorizer.AddPolicy/RemovePolicy/...` only. After a successful DB commit Casbin's enforcer auto-publishes via the watcher.
- [ ] **3.4** Test: end-to-end memory-driver — pod A's `AddPolicy` triggers pod B's `LoadPolicy` callback within N ms (use shared bus in test).

### 4. Backstop reload tick

- [ ] **4.1** Authorizer spawns one ticker goroutine on construction: `time.NewTicker(cfg.Authz.ReloadInterval)`, default 5min. On each tick: `LoadPolicy`. Errors logged, never panicked.
- [ ] **4.2** Ticker stopped in `Close`. Verified with `goleak`.
- [ ] **4.3** Config: `cfg.Authz.ReloadInterval` (duration). Zero or negative disables the backstop (documented).
- [ ] **4.4** Test: ticker fires `LoadPolicy` at least twice over a short interval; `Close` stops it.

### 5. Incremental policy load

- [ ] **5.1** Switch to `casbin.SyncedCachedEnforcer` or implement watcher event dispatch using the `WatcherEx` interface so `UpdateForAddPolicy` / `UpdateForRemovePolicy` etc trigger targeted incremental updates instead of full `LoadPolicy`.
- [ ] **5.2** The backstop tick still uses full `LoadPolicy` as the safety net.
- [ ] **5.3** Test: add a single policy on pod A → pod B sees the new rule without a full reload (assert via instrumentation counter on `LoadPolicy`).

### 6. Raw-SQL lint guard

- [ ] **6.1** Add `scripts/lint-casbin-write-path.sh`: `git grep -nE "(INSERT|UPDATE|DELETE)\s+.*casbin_rule"` returns non-empty only inside `internal/adapter/casbin/...`. Exit 1 otherwise.
- [ ] **6.2** Wire the script into `Makefile`'s `lint` target and CI workflow.
- [ ] **6.3** Test: a fixture commit adding `DELETE FROM casbin_rule` outside the adapter trips the script (exercised manually + asserted via a unit test that runs the script against a temp tree).

### 7. Verification

- [ ] **7.1** `make lint` clean (including new guard).
- [ ] **7.2** `make test` clean. Goroutine leak check passes.
- [ ] **7.3** Manual: spin up two API pods sharing Postgres + Redis; `AddPolicy` on pod A reflected on pod B within tick.

### 8. Docs

- [ ] **8.1** Create `docs/features/authorization.md`:
  - Architecture diagram (handler → Authorizer port → Casbin adapter → watcher driver).
  - Write-path invariant: mutate via `Authorizer` only; raw SQL guard.
  - Driver matrix: noop / memory / redis / custom.
  - Reload semantics: watcher event → incremental update; backstop tick → full reload.
  - Lifecycle: boot subscribe, runtime fan-out, shutdown cascade.
- [ ] **8.2** `CHANGELOG.md` `[Unreleased]` entry.
- [ ] **8.3** PR body operator-upgrade notes: new config keys (`authz.watcher.driver`, `authz.reload_interval`), default `noop` for back-compat, recommend `redis` for multi-pod.

---

## Out of scope

- Decision cache `subject:obj:act → bool` — punch-list row #10.
- Casbin role hierarchy migration — not part of this batch.
- Replacing Casbin entirely — not on the roadmap.
- gRPC mgmt API for policy admin — out per VISION.

## Acceptance

- `port.Authorizer` is the only authz type imported outside `internal/adapter/casbin/`.
- `Authorizer.Close` stops watcher + ticker; `goleak` confirms no leaked goroutines.
- Watcher driver pluggable via config; unknown driver fails fast at boot.
- Mutation on pod A propagates to pod B's enforcer via the configured watcher driver.
- Raw `casbin_rule` SQL outside the adapter trips CI.
- `docs/features/authorization.md` describes the write-path invariant and reload semantics.
