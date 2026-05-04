# PR #1 — Audit Fix

Branch: `feat/audit-context-keys`
Closes: block-ship #1 + audit should-fix gaps from `2026-05-02-preship-audit.md`.
Risk: low. Estimate: ~2h.

## Goal

The audit feature is shipped in the README but produces empty `user_id`, `ip_address`, `user_agent` for every row, and skips two high-value module surfaces (storage, job) and the failed-login event entirely. This PR makes the audit feature actually do what the README claims.

## Findings closed

- **Block-ship #1** — `internal/port/auditor.go:70-76`: `ExtractAuditContext` reads with string-literal keys; writer uses typed `logger.ContextKey`. Result: every audit row has empty `UserID`, `IPAddress`, `UserAgent`.
- **Should-fix** — `internal/platform/http/middleware/auth.go`: `UserIDKey` is written, but no IP / User-Agent values populate the context.
- **Should-fix** — `internal/module/storage/usecase/`: no `audit_decorator.go`; file uploads / downloads / deletes are not audited despite being security-relevant.
- **Should-fix** — `internal/module/job/usecase/`: no `audit_decorator.go`; job dispatch is not audited.
- **Should-fix** — `internal/module/auth/usecase/auth_usecase.go:36`: failed login attempts are not audited (current decorator only logs success). Brute-force activity is invisible.

## Tasks

### 1. Typed context keys

- [ ] **1.1** In `pkg/logger/logger.go`, add `IPAddressKey ContextKey = "ip_address"` and `UserAgentKey ContextKey = "user_agent"` next to existing `UserIDKey` and `RequestIDKey` constants.
- [ ] **1.2** In `pkg/logger/logger_test.go`, mirror the existing `UserIDKey` test pattern: assert `WithContext` propagates the new keys to the log record. (Tests for these two keys do not need to assert audit behavior; that is covered in step 4.)

### 2. Auth middleware writes IP + User-Agent

- [ ] **2.1** In `internal/platform/http/middleware/auth.go`, after the existing `setContextValue(ctx, logger.UserIDKey, claims.UserID)` call (lines 67 and 91), add:
      - `setContextValue(ctx, logger.IPAddressKey, c.IP())`
      - `setContextValue(ctx, logger.UserAgentKey, c.Get("User-Agent"))`
- [ ] **2.2** Verify in tests (`auth_test.go`) that the request context after middleware contains all three values. Use the typed keys, not string literals.

### 3. Fix the auditor reader

- [ ] **3.1** In `internal/port/auditor.go`:
      - Add `import "github.com/14mdzk/goscratch/pkg/logger"` (verify no import cycle with `pkg/logger`; if cycle exists, move keys to a new `pkg/ctxkeys` package and update step 1 + 2.1 accordingly — this is the more robust shape and worth it if blocked).
      - Replace the three `ctx.Value("user_id")` / `"ip_address"` / `"user_agent"` reads with typed-key reads using `logger.UserIDKey`, `logger.IPAddressKey`, `logger.UserAgentKey`.
- [ ] **3.2** Add a regression test in `internal/port/auditor_test.go` (create the file if absent) that:
      - Builds a context using `context.WithValue(ctx, logger.UserIDKey, "u-1")` (typed key).
      - Calls `ExtractAuditContext(ctx)`.
      - Asserts `UserID == "u-1"`. Same for IP / User-Agent.
      - **Also** add a negative test: `context.WithValue(ctx, "user_id", "u-1")` (string literal) must NOT populate the field — this locks the bug from regressing.

### 4. Storage audit decorator

- [ ] **4.1** Create `internal/module/storage/usecase/audit_decorator.go` mirroring the shape of `internal/module/user/usecase/audit_decorator.go`.
- [ ] **4.2** Decorate the operations that mutate state: `Upload`, `Delete`. (Skip `Download`, `GetURL`, `List` — read-only, noisy.) Audit action mapping: `Upload → CREATE`, `Delete → DELETE`. Resource = `"file"`, ResourceID = file path / object key.
- [ ] **4.3** Wire the decorator in `internal/module/storage/module.go` exactly like `user/module.go` does — wrap the concrete usecase with the audited variant when an `Auditor` port is available.
- [ ] **4.4** Add `internal/module/storage/usecase/audit_decorator_test.go` mirroring `user/usecase/audit_decorator_test.go`.

### 5. Job audit decorator

- [ ] **5.1** Create `internal/module/job/usecase/audit_decorator.go` following the same pattern.
- [ ] **5.2** Decorate `Dispatch`. Resource = `"job"`, ResourceID = job ID, Action = `CREATE`. Capture the job type in `Metadata`.
- [ ] **5.3** Wire in `internal/module/job/module.go`.
- [ ] **5.4** Tests mirror the user decorator tests.

### 6. Failed-login audit

- [ ] **6.1** In `internal/module/auth/usecase/audit_decorator.go`, extend the `Login` wrapper so that on the inner usecase returning a non-nil error, an audit entry is written with `Action = LOGIN`, `ResourceID = email-from-request` (so brute-force on a single email is detectable), and `Metadata = {"outcome":"failed", "reason":<sanitized error category>}`.
      - Sanitize `reason` to `"invalid_credentials"` or `"user_inactive"` — never echo the raw error string back into the audit row.
- [ ] **6.2** On success, the existing success-path entry already writes `UserID` from the inner result; verify `ResourceID` is also populated with the user ID (the audit currently records LOGIN with empty ResourceID per security audit findings).
- [ ] **6.3** Add a test in `audit_decorator_test.go` covering both branches: success → 1 row with action LOGIN + populated ResourceID; failure → 1 row with action LOGIN + outcome=failed + ResourceID = email.

### 7. Verification

- [ ] **7.1** `make lint` clean.
- [ ] **7.2** `make test` clean (unit).
- [ ] **7.3** `make test-integration` clean (touches the postgres-backed audit adapter).
- [ ] **7.4** Run `make dev` locally; hit `/auth/login` (success and failure) + `/api/v1/users/me` + a storage upload; query `audit_logs` directly:
      ```sql
      SELECT user_id, action, resource, resource_id, ip_address, user_agent, created_at
      FROM audit_logs ORDER BY created_at DESC LIMIT 10;
      ```
      Confirm none of `user_id`, `ip_address`, `user_agent` are empty for authenticated requests.

### 8. Docs

- [ ] **8.1** Update `docs/features/authentication.md` — add a paragraph: "Failed login attempts are recorded in `audit_logs` with `action=LOGIN`, `resource_id=<attempted_email>`, `metadata.outcome=failed`."
- [ ] **8.2** Update `docs/features/file-storage.md` and `docs/features/background-jobs.md` — add a sentence noting upload/delete/dispatch are audited.
- [ ] **8.3** Add `[Unreleased]` section to `CHANGELOG.md` if absent; entry: `Fixed audit log producing empty user_id/ip_address/user_agent (typed context-key bug). Added audit coverage for storage upload/delete, job dispatch, and failed login attempts.`

### 9. PR description checklist

- [ ] Quote the audit row shape before/after (one example each).
- [ ] Note: no schema migration; only column population changed.
- [ ] Note: ADR-006 NoOp Auditor still works; if `Auditor = NoOp`, decorators no-op.
- [ ] Link `docs/audit/2026-05-02-preship-audit.md` block-ship #1.

---

## Out of scope (defer to later PRs)

- NoOp-Auditor carve-out / fail-fast behavior — handled by PR #3 (auth hardening).
- Wider context-key cleanup beyond the three identifiers — handled module-by-module if more bugs surface.
- Audit log retention / cleanup policy — already handled by `worker/handlers/audit_cleanup_handler.go`.

## Acceptance

A reviewer running `make test-integration` then `psql` against the test DB sees populated `user_id`, `ip_address`, `user_agent` for every authenticated request, plus rows for storage uploads, job dispatches, and failed logins. The negative test in step 3.2 fails if anyone reintroduces a string-literal `ctx.Value("user_id")`.
