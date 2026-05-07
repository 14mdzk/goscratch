# PR #4 — Shutdown Rewrite

Branch: `refactor/shutdown-rewrite`
Status: pending
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

- [ ] **1.1** Extend `port.Authorizer` with `Close(ctx context.Context) error`. Update every implementor (`casbin.Adapter`, `casbin.NoOpAdapter`, any test fakes).
- [ ] **1.2** `casbin.Adapter.Close(ctx)` cancels the backstop-ticker context, calls `watcher.Close()` if non-nil, and closes the underlying `*sql.DB` if owned. Idempotent (sentinel-guarded).
- [ ] **1.3** `casbin.NoOpAdapter.Close` returns nil.
- [ ] **1.4** `app.Run(ctx)` calls `a.Authorizer.Start(ctx)` once, before `a.Server.Start()`. Boot fails fast on Start error.
- [ ] **1.5** `app.Shutdown(ctx)` calls `a.Authorizer.Close(authzCtx)` in the resource-close phase (see task 3 ordering).
- [ ] **1.6** Test: `TestApp_Shutdown_ClosesAuthorizer` — fake `Authorizer` records Close-calls; assert exactly one.

### 2. SSE per-connection UUID

- [ ] **2.1** `internal/adapter/sse/broker.Subscribe` accepts a per-connection ID (UUID) the caller generates; if collision, close the prior channel before overwrite (defensive — should never happen with UUID).
- [ ] **2.2** `internal/module/sse/handler/sse_handler.go` generates `connID := uuid.NewString()` per request; uses that for `Subscribe` / `Unsubscribe`. `userID` retained on the `clientInfo` for fan-out filtering.
- [ ] **2.3** `Broker.Broadcast` continues to filter by topic; per-user fan-out (if any) iterates clients and matches `clientInfo.userID`.
- [ ] **2.4** Test: `TestBroker_Subscribe_TwoTabsForSameUser` — two `Subscribe` calls with same `userID` produce two distinct channels; both receive broadcast; `Unsubscribe` of one leaves the other open.
- [ ] **2.5** Test: `TestBroker_Subscribe_DuplicateConnID_ClosesPrior` — defensive path.

### 3. Shutdown ordering + per-phase budgets

- [ ] **3.1** Replace flat `Shutdown(ctx)` with phased budgets. New helper `withBudget(parent, fraction) (context.Context, context.CancelFunc)` derives a child ctx with a fraction of the remaining deadline (e.g., 0.4 for HTTP server, 0.3 for resources, 0.2 for tracer, 0.1 buffer).
- [ ] **3.2** Phase order:
  1. HTTP server (`a.Server.Shutdown`) — drain in-flight requests.
  2. Metrics listener (`a.metricsServer.Shutdown`).
  3. Worker (`a.Worker.Shutdown` if present) — stops consumers + retry goroutines.
  4. SSE broker (`a.SSE.Close()`) — disconnects subscribers cleanly so trace exports don't race.
  5. Authorizer (`a.Authorizer.Close`) — Casbin DB + watcher goroutine.
  6. Other adapters (`Cache`, `Queue`, `Storage`, `Auditor`, `Email`).
  7. DB (`a.DB.Close()`) — last DB-using thing closed.
  8. Tracer (`a.tracerShutdown`) — **last**, so spans from prior phases flush.
- [ ] **3.3** Each phase logs duration on completion. Phase failure logs but does not abort subsequent phases (best-effort cleanup).
- [ ] **3.4** Test: `TestApp_Shutdown_PhaseOrder` — instrument fakes record call order; assert tracer is last and Authorizer is before DB.
- [ ] **3.5** Test: `TestApp_Shutdown_BudgetSplit` — pass a 100ms parent deadline; assert no single phase consumes > its allotted fraction (use a fake that records its received deadline).

### 4. Worker `wg` + retry ctx-aware

- [ ] **4.1** Refactor `internal/worker/worker.go` so `wg.Add(1)` covers the actual handler goroutine path, not just `Consume` registration. Either: (a) `Consume` returns a `<-chan Delivery` and the worker spawns its own goroutines under `wg`, or (b) `Consume` accepts a callback and spawns under a `wg` injected by the worker.
- [ ] **4.2** `retryJob` registers its delay-goroutine on `w.wg`; replaces `time.Sleep(delay)` with `select { case <-time.After(delay): case <-w.ctx.Done(): return }`.
- [ ] **4.3** `retryJob` checks `w.ctx.Err()` before `Publish`; on cancel, log + return without publishing.
- [ ] **4.4** Test: `TestWorker_Shutdown_WaitsForRetry` — schedule a retry with long delay, call Shutdown, assert Wait returns when ctx cancels (not after the full delay).
- [ ] **4.5** Test: `TestWorker_Retry_NoPublishAfterShutdown` — fake queue records publishes; assert zero publishes after Shutdown returns.

### 5. Verification

- [ ] **5.1** `make lint` clean.
- [ ] **5.2** `make test` clean (race detector on).
- [ ] **5.3** Goroutine leak check on `TestApp_Shutdown_*` (uber-go/goleak or counted-goroutine assertion).
- [ ] **5.4** Manual: run app, send `SIGTERM`; verify logs show phase order + per-phase durations + zero leaked goroutines (`/debug/pprof/goroutine?debug=1` if exposed).

### 6. Docs

- [ ] **6.1** `docs/features/lifecycle.md` (new) — boot order, shutdown phases, per-phase budgets, tracer-last rationale, Authorizer Start/Close contract.
- [ ] **6.2** `CHANGELOG.md` `[Unreleased]` entries for each finding closed.
- [ ] **6.3** PR body operator-upgrade note: shutdown timeout configuration (`server.shutdown_timeout`, default 30s); per-phase fractions documented.

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
