# PR-03 Auth Hardening

**Branch:** `feat/auth-hardening`
**Status:** ready for review
**Audit source:** `2026-05-02-preship-audit.md` — block-ship #3, #4, #5 + four should-fix items

---

## Scope

Close the remaining auth-security block-ship findings and their closely related should-fix items that belong in the same blast radius.

### In scope (tasks 1–7)

- [x] 1. Logout authn middleware — `/auth/logout` protected by `Auth` middleware; handler verifies callerID from JWT claims
- [x] 2. Refresh-token cache gating — `port.ErrCacheUnavailable` sentinel added; fail-closed login (error if cache unavailable); NoOp logs warning at boot; **dual-key design**: lookup key `refresh:tok:<sha256-hex(token)>` + per-user index key `refresh:user:<userID>:<sha256-hex(token)>` (full 64-char hash); `user_id` removed from `/auth/refresh` request body; `Logout` validates token owner before delete (oracle-avoidance); `ChangePassword` delegates to `auth.Revoker.RevokeAllForUser` via injected port
- [x] 3. Casbin fail-fast — removed NoOp fallback in `app.go` when `authorization.enabled=true`; `NoOpAdapter` marked test-only via `doc.go`
- [x] 4. Rate-limit `FailClosed` field — added to `RateLimitConfig`; applied to `/auth/login` + `/auth/refresh` (20/5min); error logged via `slog.Error` on backend failure
- [x] 5. JWT iss/aud strict — `Config.Validate` requires non-empty `jwt.issuer` / `jwt.audience`; `config.default.json` already carries non-empty values; `parseToken` rejects tokens with empty/mismatched iss or aud
- [x] 6. Failed-login audit row — already implemented in PR-01 via `AuditedUseCase.Login`; verified: `auth.login.failed` covered by `AuditActionLogin` with `metadata.outcome=failed`
- [x] 7. `ChangePassword` invalidates all refresh keys — `Cache.DeleteByPrefix` added to port + both adapters; user usecase calls it after password update; NoOp returns `ErrCacheUnavailable`
- [x] 7a. **Bug fix — index-key as revocation gate** — `Refresh` now validates BOTH the lookup key AND the per-user index key; absence of the index key (e.g., after `RevokeAllForUser`) rejects the token with 401 even if the lookup key is still cached (orphan). `mapCache.DeleteByPrefix` in tests now performs a real prefix scan. Regression test `TestRefresh_RevokedByPasswordChange` added.

### Out of scope (defer)

- Authorizer interface changes (PR-03b)
- Watcher abstraction for Casbin (PR-03b)
- Sliding-window rate limiter (PR-09)
- SSE per-conn UUID (PR-04)
- Worker wg covers real work (PR-04)
- Shutdown rewrite (PR-04)

---

## Docs to update

- [x] `docs/features/authentication.md` — fail-closed login, dual-key shape, `user_id` removed from refresh body, `Logout` oracle-avoidance, `Revoker` port for ChangePassword, iss/aud required
- [x] `CHANGELOG.md` `[Unreleased]` — entries covering all items including dual-key API change and `user_id` removal

---

## Verification (task 8)

- [x] 8.1 `make lint` passes
- [x] 8.2 `make test` passes (race detector on) — includes `TestRefresh_RevokedByPasswordChange`
- [x] 8.3 All task checkboxes above are ticked
