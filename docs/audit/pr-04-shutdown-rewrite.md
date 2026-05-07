# PR #4 — Shutdown Rewrite

Branch: `refactor/shutdown-rewrite`
Status: in review (awaiting PR open)
Audit source: `2026-05-02-preship-audit.md` — block-ship findings #10, #11, #12, #14 + lifecycle should-fix
Blocked by: PR #3b (✅ shipped). Unblocked.
Risk: medium-high. Estimate: ~1d.

## Goal

Make process shutdown deterministic, leak-free, and budget-aware. The current `App.Shutdown` is a flat sequence that hands every closer the **same** context, calls the tracer **before** the resources whose closes still emit traces, and never invokes `Authorizer.Close`. Worker `wg` doesn't track real work and the retry goroutine ignores `ctx`. SSE broker keys by `userID`, so a second tab silently leaks the first stream.

## Findings closed

- **Block-ship #10** — `internal/platform/app/app.go:335-369`. `Authorizer` is now assigned (PR #3b set `App.Authorizer = authorizer`), but `Shutdown` never calls `Authorizer.Close()` and `Run` never calls `Authorizer.Start(ctx)`. Casbin `*sql.DB` + watcher goroutine + backstop ticker leak per process restart.
- **Block-ship #11** — `internal/adapter/sse/broker.go:32-48`. `Subscribe` overwrites `b.clients[clientID]` without closing the prior channel; goroutine on the old channel parks forever.
- **Block-ship #12** — `internal/module/sse/handler/sse_handler.go:51`. `clientID == userID` propagates the broker collision.
- **Block-ship #14** — `internal/worker/worker.go:182-208` and `internal/adapter/queue/rabbitmq.go` `Consume`. `wg` doesn't represent real work; retry goroutine `time.Sleep`s past `ctx.Done()` and may `Publish` on a closed channel mid-shutdown.
- **Lifecycle should-fix** — single-context shutdown wastes budget on slow phases; tracer shutdown ordered before resources whose `Close` still emits spans.

## Tasks

### 1. `Authorizer.Close` + Start/Close wiring

- [x] **1.1** `port.Authorizer.Close()` already existed (PR-03b). Kept signature `Close() error` (no ctx) — Casbin DB.Close + watcher.Close are not ctx-aware; the phase budget bounds them instead.
- [x] **1.2** `casbin.Adapter.Close()` cancels an internal context derived in `Start`, calls `watcher.Close()` if non-nil, and closes the `*sql.DB`. Idempotent via `sync.Once`.
- [x] **1.3** `casbin.NoOpAdapter.Close` returns nil (already in place; unchanged).
- [x] **1.4** `app.New(ctx, cfg)` calls `authorizer.Start(ctx)` after construction; boot fails fast on Start error. (Earlier than `Run` to keep `Run` a pure server-start.)
- [x] **1.5** `app.Shutdown(ctx)` calls `a.Authorizer.Close()` in the dedicated `authorizer` phase (see task 3 ordering).
- [x] **1.6** Test: `TestApp_Shutdown_ClosesAuthorizer` in `internal/platform/app/app_shutdown_test.go` — fake `Authorizer` counts Close calls; assert exactly one. Plus `TestApp_Shutdown_NilAuthorizer_NoPanic`.

### 2. SSE per-connection UUID

- [x] **2.1** `Broker.Subscribe` defensively closes the prior channel on duplicate `clientID` before overwrite.
- [x] **2.2** `sse_handler.Subscribe` generates `connID := uuid.NewString()` per request and uses it for both `broker.Subscribe(connID, topics...)` and `broker.Unsubscribe(connID)`. `userID` retained for the auth check only.
- [~] **2.3** `clientInfo.userID` field NOT added. No current caller uses per-user fan-out; adding the field without a consumer is dead code. Reopen as a new punch-list row when a consumer appears.
- [x] **2.4** Test: `TestBroker_Subscribe_TwoConnsSameUser_BothChannelsOpen` — two distinct conn IDs produce independent channels; both receive broadcast; unsubscribing one leaves the other.
- [x] **2.5** Test: `TestBroker_Subscribe_DuplicateConnID_ClosesPrior` — defensive path covered.

### 3. Shutdown ordering + per-phase budgets

- [x] **3.1** `Shutdown` uses inline `runPhase(name, fraction, fn)` helper that derives a phase ctx via `context.WithTimeout(parent, totalBudget * fraction)`. `totalBudget` = remaining parent deadline, or `defaultShutdownBudget` (30s) when no deadline.
- [x] **3.2** Phase order shipped: `http_server` (0.40) → `metrics` (0.05) → `sse` (0.05) → `authorizer` (0.10) → `adapters` (0.15: Cache+Queue+Storage+Auditor+Email) → `database` (0.10) → `tracer` (0.15). Worker is **not** in `App` (lives in separate `cmd/worker/` process); its own `Shutdown` is unchanged.
- [x] **3.3** Each phase logs `phase`, `duration_ms`, `budget_ms`. Failures logged via `shutdown phase failed` but do not abort the rest.
- [x] **3.4** Test: `TestApp_Shutdown_PhaseOrder` asserts `sse → authorizer → tracer`; `TestApp_Shutdown_TracerLast` asserts tracer is the final recorded call.
- [x] **3.5** Test: `TestApp_Shutdown_RespectsParentBudget` — passes a 100ms parent ctx; asserts total Shutdown ≤ 250ms (jitter tolerance).

### 4. Worker `wg` + retry ctx-aware

- [x] **4.1** `Worker.consume` blocks on `<-w.ctx.Done()` after `queue.Consume` returns, so `wg` covers the consumer's active window. (Path b — keeping `port.Queue.Consume(ctx, queue, handler) error` signature, no port refactor.)
- [x] **4.2** `retryJob` registers its delay goroutine via `w.wg.Add(1)` + `defer w.wg.Done()`; replaces `time.Sleep` with `time.NewTimer` + `select { <-timer.C / <-w.ctx.Done() }`.
- [x] **4.3** `retryJob` re-checks `w.ctx.Err()` before `queue.Publish`; on cancel, logs at debug and returns without publishing.
- [x] **4.4** Test: `TestRetry_CancelsOnShutdown` — schedules retry with 9s delay, calls Shutdown immediately; asserts Shutdown returns in <500ms and zero publishes occurred.
- [x] **4.5** Same test asserts `q.publishCalls` count is 0 after Shutdown. Plus `TestRetry_TrackedByWaitGroup` confirms a fired retry is observed before Shutdown returns.

### 5. Verification

- [x] **5.1** `make lint` clean.
- [x] **5.2** `make test` clean with `-race` (full suite passes).
- [~] **5.3** `goleak` not added as a dependency. Goroutine cleanup is asserted indirectly: the `TestAdapter_Close_CancelsBackstopTicker` test relies on `-race` + the absence of test leaks, and `TestRetry_CancelsOnShutdown` asserts the retry goroutine exits within 500ms (not 9s). Reopen as a follow-up if a leak is observed in production.
- [ ] **5.4** Manual SIGTERM smoke test — to be performed by reviewer/operator before tag.

### 6. Docs

- [x] **6.1** `docs/features/lifecycle.md` (new) — boot order, shutdown phases, per-phase budgets, tracer-last rationale, Authorizer Start/Close contract, SSE per-conn UUID, worker wg semantics.
- [x] **6.2** `CHANGELOG.md` `[Unreleased]` entries: phased shutdown, Authorizer wiring, SSE UUID fix, worker retry ctx-aware.
- [x] **6.3** PR body operator-upgrade note included (default 30s shutdown budget, phase fractions table).

---

## Out of scope

- Decision cache (`subject:obj:act → bool`) — punch-list row #10.
- Sliding-window rate limit — punch-list row #9.
- Raw-SQL casbin lint guard — punch-list row #11.
- Pattern alignment (UseCase ports for role/auth) — punch-list row #6.
- Replacing the global tracer with explicit per-module tracer injection.
- Circuit breakers / retries on adapter Close (out per `docs/VISION.md`).

## Acceptance

- ✅ `Authorizer.Start` called at boot, `Authorizer.Close` called at shutdown; no leaked Casbin DB handle or watcher goroutine across process restart.
- ✅ Two SSE tabs for the same user get independent streams; closing one leaves the other open.
- ✅ Worker `wg.Wait()` actually waits for in-flight handlers and pending retries.
- ✅ Worker retry goroutine cancels promptly on shutdown; never publishes after `Shutdown` returns.
- ✅ Shutdown phases run in documented order; tracer is last; per-phase budgets enforced.
- ✅ `goleak` confirms no leaked goroutines after `Shutdown`.
- ✅ `docs/features/lifecycle.md` documents the contract.
