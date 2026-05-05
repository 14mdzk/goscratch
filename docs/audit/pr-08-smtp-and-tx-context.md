# PR #8 — SMTP + Postgres Rollback Context Discipline

Branch: `fix/smtp-and-tx-context`
Closes: 2 should-fix from `2026-05-02-preship-audit.md` (lines 80-81).
Risk: low. Estimate: ~1h.
Status: ready for review.

## Goal

Two unrelated but mechanically similar context-discipline bugs. Both let the
process appear healthy while a downstream resource (SMTP worker, Postgres
transaction) is stuck or leaking. Fix them together because the test surface
and reviewer load is small.

## Findings closed

- **Should-fix** — `internal/adapter/email/smtp.go:34-79`: `Send(ctx, msg)` ignores `ctx` and calls `smtp.SendMail`, which has no deadline. A blackhole SMTP server (TCP accepts, never replies) wedges the calling goroutine for the OS TCP timeout (often >2 minutes), defeating the queue worker's ctx-driven shutdown and per-job deadlines.
- **Should-fix** — `internal/platform/database/postgres.go:84-96`: `WithTx` forwards the outer `ctx` to `tx.Rollback`. On shutdown the outer ctx is already cancelled, so `tx.Rollback(ctx)` returns `context.Canceled` immediately and the rollback never reaches the server. The transaction stays open until the connection is recycled or the server's idle-in-transaction timeout fires.

## Tasks

### 1. SMTP deadline

- [x] **1.1** Replace `smtp.SendMail` with a context-aware exchange in `internal/adapter/email/smtp.go`. Implementation shape (a) from the PR brief:
      - `net.Dialer{}.DialContext(ctx, "tcp", addr)` for the TCP dial.
      - `conn.SetDeadline(deadline)` so every read/write inside the SMTP exchange honours the ctx deadline.
      - A watcher goroutine: `select { <-ctx.Done(): conn.Close() | <-exchangeDone }` so cancellation mid-exchange unblocks any in-flight read/write.
      - Manual walk: `smtp.NewClient` → `Hello` → opportunistic `STARTTLS` (when advertised) → `Auth` (when configured) → `Mail` / `Rcpt` / `Data` / `Quit`.
- [x] **1.2** Default-deadline guard: if the caller's ctx has no deadline, wrap with `context.WithTimeout(ctx, defaultSMTPTimeout)` (30s). Prevents accidental unbounded waits when callers forget to set one. Documented in the source.
- [x] **1.3** Preserve the existing public surface: `NewSMTPSender`, `SMTPConfig`, `Send(ctx, msg)`, `Close()`. No call-site changes required.

### 2. Tx rollback context

- [x] **2.1** In `internal/platform/database/postgres.go`, both rollback paths (panic-recover and fn-error) now build a fresh ctx via `context.WithTimeout(context.Background(), rollbackTimeout)` and pass that to `tx.Rollback`. Comment in source explains why ("outer ctx may already be cancelled on shutdown; rollback must still run").
- [x] **2.2** Introduce package-private `txBeginner` interface (just `Begin(ctx) (pgx.Tx, error)`) so unit tests can inject a fake. `*pgxpool.Pool` satisfies it; the public `NewTransactor(pool *pgxpool.Pool)` signature is unchanged.

### 3. Tests

- [x] **3.1** `internal/adapter/email/email_test.go` — `TestSMTPSender_Send_ContextDeadline`. Spins up a `net.Listen` fake that accepts TCP but never writes the SMTP greeting, then asserts `Send` returns an error inside ~3s when ctx deadline is 500ms. Without the fix the test would hang on the OS TCP timeout. A `time.AfterFunc(5s, listener.Close)` is the hard upper bound so a regression cannot wedge the test binary.
- [x] **3.2** `internal/platform/database/postgres_test.go` — `TestWithTx_RollbackUsesFreshContext`. Uses a `fakeTx` (embeds `pgx.Tx` so the interface is satisfied without re-declaring every method) and a `fakeBeginner`. Calls `WithTx` with a pre-cancelled outer ctx and asserts:
      - `Rollback` was invoked.
      - The ctx received by `Rollback` had `Err() == nil` at call time.
      - The ctx carried a deadline within `rollbackTimeout` of "now".
      - Plus a happy path: fn returns nil → `Commit` invoked, `Rollback` not invoked.

### 4. Docs

- [x] **4.1** `CHANGELOG.md` `[Unreleased]` — entry under "Fixed" covering both bugs.
- [x] **4.2** `docs/features/email.md` — note that `Send` honours ctx deadlines and applies a 30s default when none is set.
- [x] **4.3** No `docs/features/database.md` exists; database transaction notes already live near the audit feature docs and don't need an update for this fix.

### 5. Verification

- [x] **5.1** `make lint` clean.
- [x] **5.2** `make test` clean (unit, with `-race`).

---

## Out of scope (defer to later PRs / punch-list)

- Refactoring the SMTP exchange to support implicit TLS (port 465). The current adapter only opportunistically upgrades via STARTTLS, matching upstream `net/smtp.SendMail`. A separate finding if needed.
- Connection pooling for SMTP. Single-shot dial per send is fine for the current send rate; revisit if email volume grows.
- Retry / backoff on SMTP transient errors. Worker layer concern; out of scope here.
- Rollback-on-Commit-failure: when `tx.Commit` returns an error pgx auto-rolls back, but a future audit may still want to log this case. Not in this PR.
- Refactoring `Transactor` to a fully mockable interface for repository-layer tests. The narrow `txBeginner` interface added here is the minimum required for this PR's tests; a broader refactor goes in PR #6 (pattern alignment).

## Acceptance

A reviewer running `make lint test` sees both new tests pass. Killing the SMTP
adapter mid-send (by closing the conn) returns within the ctx deadline rather
than blocking on the OS TCP timeout. Forcing a transactional usecase to error
during shutdown produces a real `ROLLBACK` on the server (verifiable in the
postgres `pg_stat_activity` view) instead of leaving the transaction
`idle in transaction` until the server-side timeout.
