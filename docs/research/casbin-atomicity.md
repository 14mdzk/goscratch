# Casbin v3 policy-write atomicity — findings (role-module decision)

| Field | Value |
|---|---|
| Ticket | Research: Casbin v3 policy-write atomicity options (#88) |
| Feeds | Decide role-module write atomicity (#91) |
| Date | 2026-10-02 |
| Method | Read-only source review: Blank-Xu/sql-adapter v1.2.1 and casbin/casbin v3.10.0 from the Go module cache; repo `internal/adapter/casbin` + `internal/module/role` (branch `research/casbin-atomicity`, on `main` @ `90ed69d`). No runtime/concurrency experiments. |

## TL;DR

- The plan's premise — *"role assignment touches multiple Casbin policy rows; needs atomicity"* — does **not** match the current code. Every role-module write path issues **exactly one** Casbin mutation; sql-adapter executes it as **one SQL statement**.
- Casbin v3.10.0 **has** a real buffered transaction API (`TransactionalEnforcer` / `Transaction`, two-phase commit) — but it hard-requires the adapter to implement `persist.TransactionalAdapter`. **Blank-Xu/sql-adapter v1.2.1 does not implement it**, so `BeginTransaction` fails with `adapter does not support transactions`.
- sql-adapter already wraps *multi-row* calls (`AddPolicies`, `RemovePolicies`, `SavePolicy`) in a **per-call** DB transaction; single-row `AddPolicy` / `RemovePolicy` are single `ExecContext` statements with no explicit tx.
- Cross-call transactions therefore require a **custom adapter**. Given today's write shapes, per-call atomicity + the `casbin_rules` UNIQUE index already covers what the code does; the decision may legitimately be *"no transaction infrastructure now — document, and keep the one-mutation-per-operation discipline"*.

## 1. How policy writes happen today (this repo)

- **Connection**: `internal/adapter/casbin/casbin.go:93` — `sql.Open("pgx", cfg.DatabaseURL)` — a dedicated `database/sql` pool, separate from the app's `pgxpool`; `:104` — `sqladapter.NewAdapter(db, "postgres", "casbin_rules")`; `:121` — `casbin.NewEnforcer(m, adapter)`. AutoSave defaults to **true** (`casbin/enforcer.go:192`), so every enforcer mutation writes through the adapter synchronously.
- **Mutation surface** (`internal/adapter/casbin/casbin.go`):
  - `AddRoleForUser` `:278-287` → `enforcer.AddGroupingPolicy` (one row)
  - `RemoveRoleForUser` `:291-300` → `enforcer.RemoveGroupingPolicy`
  - `AddPermissionForRole` `:323-332` → `enforcer.AddPolicy`; `RemovePermissionForRole` `:337-346`
  - `AddPermissionForUser` `:355-364` → `enforcer.AddPolicy`; `RemovePermissionForUser` `:368-377`
  - After success: `cache.invalidateSub(user)` or `cache.flush()` (`:284, :296, :329, :343, :361, :374`).
- **Watcher**: `Start` `:166-200` attaches a `persist.Watcher` to the enforcer, so mutations notify the watcher (auto-notify default on); the callback `:205-231` applies incremental ops on peers and flushes the cache. Backstop full reload every `ReloadInterval` (default 5 min).
- **Role usecase write paths** (`internal/module/role/usecase/role_usecase.go`) — one mutation call each:

| Operation | Read-check | Mutation |
|---|---|---|
| AssignRole | `:36` HasRoleForUser | `:44` AddRoleForUser |
| RevokeRole | `:58` HasRoleForUser | `:66` RemoveRoleForUser |
| AddPermissionForRole | — | `:121` |
| RemovePermissionForRole | — | `:134` |
| AddPermissionForUser | — | `:207` |
| RemovePermissionForUser | — | `:215` |

- **SQL shape** (sql-adapter): single-row `AddPolicy` → `dao.InsertRow` → `execSQL` → `db.ExecContext` (`dao.go:314-316`, `:147-158`); single-row `RemovePolicy` → `dao.DeleteByArgs` → `execSQL` (`dao.go:357-379`). One statement per call.
- **Guard**: `migrations/000003_casbin_rules.up.sql:16` — `CREATE UNIQUE INDEX idx_casbin_rules_unique ON casbin_rules(p_type, v0..v5)`; concurrent duplicate assignment fails loudly instead of corrupting (seed inserts are `ON CONFLICT DO NOTHING`).

## 2. What sql-adapter v1.2.1 offers

- **Interfaces implemented** (`adapter.go:23-33`): `Adapter`, `ContextAdapter`, `FilteredAdapter`, `ContextFilteredAdapter`, `BatchAdapter`, `ContextBatchAdapter`, `UpdatableAdapter`, `ContextUpdatableAdapter`. **No `persist.TransactionalAdapter`.**
- Constructor takes a **caller-owned `*sql.DB`** (`adapter.go:47-58`, comment: *"db should connected to database and controlled by user"*).
- **Execution paths** (`dao.go`):
  - single-row → `execSQL` → `db.ExecContext` (`:147-158`) — no explicit transaction;
  - multi-row/bulk → `execTxSQL` (`:160-218`) — `db.BeginTx` + exec + commit/rollback. Used by `InsertRows` (`:318-321`), `UpdateRows`, `UpdateFilteredRows`, `DeleteRows`, `DeleteAllAndInsertRows`. So `AddPolicies` / `RemovePolicies` / `SavePolicy` (batch surfaces) are **already atomic per call**.
- `persist.TransactionalAdapter` is not required for these batch paths — Casbin's own API-level batching is enough for multi-row atomicity within one call.

## 3. What Casbin v3.10.0 offers

- `persist.TransactionalAdapter` (`persist/transaction.go:21-25`): `BeginTransaction(ctx) (TransactionContext, error)`.
- `persist.TransactionContext` (`:30-36`): `Commit()`, `Rollback()`, `GetAdapter() Adapter` (an adapter bound to the transaction).
- `TransactionalEnforcer` (`enforcer_transactional.go:30-36`), `NewTransactionalEnforcer` (`:39`), `BeginTransaction` (`:52-81`), `WithTransaction` (`:101+`).
- `Transaction` (`transaction.go:35`) buffers ops — `AddPolicy`/`AddPolicies`/`RemovePolicy`/`UpdatePolicy` + grouping variants (`:50-380`) — into a `TransactionBuffer` (`transaction_buffer.go`), and exposes `HasOperations`/`OperationCount`.
- **Commit is two-phase** (`transaction_commit.go:27-98`): (1) conflict detection against `modelVersion`; (2) apply buffered ops to the DB through `txContext.GetAdapter()` (`:135-158`, with `BatchAdapter`/`UpdatableAdapter` fast paths at `:160-208`); (3) commit the DB transaction; (4) apply ops to the in-memory model — a post-DB-commit model update failure is reported as a *"critical error"*. A commit lock with timeout serializes commits.
- `Rollback` (`:105-133`) rolls back the DB transaction and clears state.
- **The gate**: `BeginTransaction` returns `errors.New("adapter does not support transactions")` unless the enforcer's adapter implements `TransactionalAdapter` (`enforcer_transactional.go:53-56`). **sql-adapter v1.2.1 does not**, so this API is unavailable as-is.
- Even when used, **watcher notification and decision-cache invalidation are not part of the transaction** — they are post-commit side effects you must wire after `Commit()`.

## 4. Options — feasibility, not a choice

| # | Option | What it takes | Cost | Notes |
|---|---|---|---|---|
| a | **Accept per-call atomicity + document** | Docs + the rule that role mutations stay single-call; rely on the unique index for races | Low | Matches today's actual behavior; no new failure modes |
| b | **Custom `TransactionalAdapter` + `enforcer.WithTransaction`** | New adapter implementing `TransactionalAdapter` (`BeginTransaction` → `GetAdapter` returning a tx-bound adapter). sql-adapter's `dao` is unexported, so the tx-bound adapter must execute insert/delete/update/load against a `*sql.Tx` itself (or use a different library) | Medium-high | Only real mechanism for cross-call transactions; must wire watcher notify + cache invalidation after commit; decide whether `LoadPolicy` still uses sql-adapter |
| c | **Route through the app `pgxpool`** | `stdlib.OpenDBFromPool` yields a `*sql.DB` over the pool — but tx-binding needs (b) anyway; consolidates connection ownership across the Casbin SQL-lint boundary | approx. (b) + plumbing | Does not, by itself, add atomicity; benefits are pool consolidation only |
| d | **Use batch adapter calls for multi-row ops** | Nothing new — sql-adapter already runs batches in a per-call tx (`execTxSQL`) | Low | The cheapest path *if* a future operation batches mutations |
| e | **Compensating cleanup** | Application-level undo on partial failure | Medium | Only relevant once a logical operation spans multiple calls — none does today |

## 5. What this means for the decision (#91)

- **Check the premise first**: the role module currently performs one policy mutation per user action (`role_usecase.go:44, :66, :121, :134, :207, :215`), and each single-row write is one SQL statement. "Multiple policy rows per operation" is not true of today's code; the plan line may be stale or anticipating a future bulk operation.
- If the decision is to **provide cross-call transactions anyway**, option (b) is the only mechanism, gated on a custom `TransactionalAdapter`; budget for adapter code + tests + post-commit watcher/cache side effects.
- If the decision is to **not add infrastructure**, document the per-call atomicity, keep mutations single-call by convention, keep the unique index, and choose (d) when a batch operation first appears.
- Residual risks to name in either case: check-then-act on `HasRoleForUser` → `AddRoleForUser` (bounded by the unique index; duplicates surface as an error), best-effort watcher propagation (converges via the 5-minute backstop reload), and audit rows written by the decorator outside the policy write (intended).

## 6. Caveats

- Read-only source review; no execution, property, or concurrent-writer experiments.
- Versions pinned as of 2026-10-02: `github.com/Blank-Xu/sql-adapter v1.2.1`, `github.com/casbin/casbin/v3 v3.10.0` (per `go.mod`). The Casbin transaction files carry 2025 copyright headers — the API is relatively new; maturity beyond source inspection was not assessed.
- `RemovePolicyCtx` / `DeleteByArgs` SQL path verified by source only (`dao.go:357-379`), not executed.

## 7. Sources

Upstream (module cache, v-pinned):
- `github.com/Blank-Xu/sql-adapter@v1.2.1/adapter.go:23-33,47-58,204-275`; `dao.go:119-158,160-218,314-379`
- `github.com/casbin/casbin/v3@v3.10.0/enforcer_transactional.go:39-115`; `transaction.go:35-410`; `transaction_commit.go:27-260`; `transaction_buffer.go:27-127`; `persist/transaction.go:21-46`; `enforcer.go:192,625-627`

Repo:
- `internal/adapter/casbin/casbin.go:93-160,166-231,278-402`
- `internal/module/role/usecase/role_usecase.go:36-215`
- `migrations/000003_casbin_rules.up.sql:1-16`
