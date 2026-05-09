# PR-12: v1.1.0 Release Cut + Doc Sync

| Field | Value |
|-------|-------|
| Branch | `chore/release-v1.1.0` |
| Status | in review |
| Audit source | Punch-list "After PRs Land" section + ROADMAP/feature drift |
| Closes | Punch-list "After PRs Land" items 1–3; row #12 |

## Goal

All 11 hardening PRs from `docs/audit/punch-list.md` are shipped. The repository state is `v1.1.0`-ready, but the surrounding documentation has not been cut for release:

- `CHANGELOG.md` still has every hardening entry under `[Unreleased]`.
- `README.md` "Documentation" section does not link `docs/audit/` (the audit + punch-list trail is invisible to a fresh cloner).
- `docs/QUICKSTART.md` does not list the secure-defaults an operator must set after upgrading from v1.0 (JWT_SECRET length/non-placeholder, sslmode, JWT_ISSUER/AUDIENCE, redis.enabled fail-closed, trusted proxies, /metrics binding).
- `docs/ROADMAP.md` v1.1 section is still labelled `(PLANNED)` with every row in `Planned` status, and is missing rows for the three PRs added mid-flight (#3b, #10, #11).

This PR closes the documentation gap so `git tag v1.1.0` is honest.

## Tasks

- [x] Cut `CHANGELOG.md` `[Unreleased]` → `## [1.1.0] - 2026-05-09 — Hardening`. Re-open an empty `[Unreleased]` block above it. No content changes inside the existing entries.
- [x] Update `README.md` "Documentation" section to add `docs/audit/` (audit + per-PR scope files + punch-list).
- [x] Append "Secure-defaults checklist" section to `docs/QUICKSTART.md`. Operators upgrading from v1.0 must verify each item before booting.
- [x] Update `docs/ROADMAP.md`:
  - Flip `v1.1 - Pre-Ship Hardening (PLANNED)` → `(DONE)`.
  - Replace the "Planned" status column with shipped PR links for rows 1–9.
  - Add three rows for PRs #3b (authz cache infra), #10 (decision cache), #11 (raw-SQL lint guard).
- [x] Update `docs/audit/punch-list.md`: add row #12 (this PR), append entry to "After PRs Land" with status.
- [x] `make lint` clean.
- [x] Tick all task boxes in this file in the same commit (or a follow-up commit on the same branch) before the PR is opened.

## Out of Scope

- `git tag v1.1.0` push — performed by the lead **after** this PR merges, not as part of the PR diff.
- README "After PRs Land" `Hardening` summary block — the CHANGELOG release entry already serves this.
- Code changes of any kind. Docs only.
- Health-check readiness probe (`internal/module/health/handler.go:36` TODO) — tracked as separate punch-list row #13 / future PR.
- v1.2 milestone planning — needs its own brainstorm session.

## Acceptance

1. `[Unreleased]` in `CHANGELOG.md` exists and is empty (or only contains entries added after this PR).
2. `## [1.1.0] - 2026-05-09 — Hardening` section contains every entry that was previously under `[Unreleased]`, byte-for-byte.
3. `README.md` "Documentation" section lists `docs/audit/` with a one-line description.
4. `docs/QUICKSTART.md` has a "Secure-defaults checklist" section covering JWT_SECRET, sslmode, JWT_ISSUER/AUDIENCE, redis.enabled, trusted proxies, /metrics.
5. `docs/ROADMAP.md` v1.1 table shows every row as shipped with PR links; rows for #3b, #10, #11 present.
6. `docs/audit/punch-list.md` contains row #12 with status `in review` and the link to this file.
7. `make lint` exits 0.
