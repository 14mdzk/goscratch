# PR-06 — Pattern Alignment

| Field | Value |
|-------|-------|
| Branch | `refactor/usecase-port-alignment` |
| Status | in review (awaiting PR open) |
| Audit source | `docs/audit/2026-05-02-preship-audit.md` — Should-Fix: Module-pattern violations; Idiom |
| Blocked-by | PR #3b ✅ |

---

## Findings Closed

| ID | Finding |
|----|---------|
| audit:idiom-1 | `err == pgx.ErrNoRows` in user repository (lines 55, 77, 213) |
| audit:idiom-2 | `err == redis.Nil` in redis adapter |
| audit:idiom-3 | `apperr.Internalf("…: %s", err.Error())` in role usecase severs errors.Is chain |
| audit:pattern-1 | role handler depends on `*usecase.UseCase` concrete type |
| audit:pattern-2 | auth module creates its own `userrepo.Repository` (duplicate pool use) |
| audit:pattern-3 | `middleware.Claims` embeds `jwt.RegisteredClaims` — layer inversion into auth usecase |

---

## Tasks

- [x] Investigate existing role, auth, claims, errors patterns
- [x] errors.Is sweep — `user_repository.go` (3x), `adapter/cache/redis.go` (1x)
- [x] Role UseCase port — `internal/module/role/usecase/port.go`, handler updated to use interface, compile-time assertion added, concrete struct renamed to `roleUseCase`
- [x] Role usecase error chains — `apperr.Internalf(…, err.Error())` → `apperr.ErrInternal.WithError(err)` throughout role usecase
- [x] Claims domain — `internal/module/auth/domain/claims.go`; `middleware.Claims` kept for JWT signing/parsing, `toDomainClaims` maps to domain type; `GetClaims` returns `*authdomain.Claims`; auth usecase no longer imports middleware
- [x] Auth user-repo reuse — `usecase.UserRepo` interface exported; `auth.NewModule` accepts interface; `app.go` creates single `sharedUserRepo` and injects into both user and auth modules; testapp.go updated
- [x] Tests — `TestRoleHandler_UsesPort` (hand-rolled fake proving interface dep); `TestToDomainClaims_RoundTrip` (JWT→domain mapping); `TestRoleUseCase_ErrorsIsWrappedInternal` (errors.Is chain)
- [x] Docs — `docs/architecture/patterns.md` created; `CHANGELOG.md` `[Unreleased]` `### Changed` entry added
- [x] Scope file (this file)
- [x] Verification — `make lint` and `make test -race` clean

---

## Out of Scope

- Any port idiom not in the four listed (storage/job ports already landed in PR #1 / #13)
- Broader DDD migration
- Renaming modules or packages
- Fixing other `apperr.Internalf` calls in modules other than role (auth, storage, user — not flagged in audit)
- Pre-existing testapp.go signature mismatch (`user.NewModule` missing `cache` + `authRevoker` args) — fixed as a side-effect since the file was already being edited for the auth user-repo change
- Integration test (`//go:build integration`) coverage — no new integration tests added for this PR

---

## Out-of-Scope Findings (new, for punch-list)

- `internal/platform/testutil/testapp.go` had a pre-existing compile bug (under `integration` build tag): `user.NewModule` call was missing `cache port.Cache` and `authRevoker usecase.AuthRevoker` arguments after PR #19 added them. Fixed in this PR as a side-effect.

---

## Acceptance

1. [x] Tests added/updated for changed behavior — yes, three new tests
2. [x] `make lint test` clean
3. [x] `docs/architecture/patterns.md` created
4. [x] `CHANGELOG.md` `[Unreleased]` entry present
5. Not a security PR — no "operator must change after upgrade" note required
