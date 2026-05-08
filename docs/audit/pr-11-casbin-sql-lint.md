# PR-11: Raw-SQL Casbin Lint Guard

| Field | Value |
|-------|-------|
| Branch | `chore/casbin-sql-lint-guard` |
| Status | in review |
| Audit source | Punch-list row #11; split out of PR-03b |
| Closes | punch-list row #11 |

## Goal

Any Go file outside `internal/adapter/casbin/` that executes a raw-SQL `INSERT`, `UPDATE`, or `DELETE` against the `casbin_rules` table bypasses the adapter's watcher and event hooks. The in-memory Casbin enforcer cache will not be notified of the change, silently diverging from the database state until the next backstop-reload tick. As defense-in-depth, a CI lint guard must detect and reject such writes at PR time — before they can ever reach production.

## Tasks

- [x] Investigate Makefile, CI workflows, existing scripts, and `internal/adapter/casbin/` to understand the current lint pipeline and the allowed file set.
- [x] Create `scripts/lint-casbin-sql.sh` — bash script using `git ls-files '*.go'` + `grep -EqHni` pattern `(INSERT|UPDATE|DELETE)[[:space:][:print:]]*casbin_rule`; excludes `internal/adapter/casbin/` prefix; exits 1 with offending file:line on match, exits 0 on clean.
- [x] Wire `lint-casbin-sql` target into `Makefile`; `lint` depends on it.
- [x] Add separate step `Run casbin SQL lint guard` in `.github/workflows/ci.yml` lint job (CI invokes golangci-lint directly, not via `make lint`).
- [x] Add testdata fixture `internal/adapter/casbin/testdata/allowed_sql_fixture.go` (tagged `//go:build ignore`) containing an `INSERT INTO casbin_rules` literal to confirm path exclusion works.
- [x] Transient sanity check: confirmed script exits 1 on a synthetic offender placed outside the allowed path, exits 0 on the current clean tree.
- [x] Update `docs/audit/punch-list.md`: PR #4 status → shipped #24; PR #11 status → in review + link to this file.
- [x] Add `CHANGELOG.md [Unreleased]` entry.

## Allowlist

Paths excluded from the guard (prefix match):

| Path prefix | Rationale |
|-------------|-----------|
| `internal/adapter/casbin/` | The canonical adapter; all writes go through `sqladapter` which notifies the watcher. |
| `scripts/seed/` | One-shot dev bootstrap; runs before the app starts, outside the watcher lifecycle. Functionally equivalent to a migration seeder. |

Any new file that writes to `casbin_rules` outside these prefixes must either (a) use the `port.Authorizer` interface instead, or (b) add an explicit entry to the script's `ALLOWED` array with written justification.

## Out of Scope

- ORM-level writes via gorm/sqlx struct operations — these bypass SQL string scanning entirely. If that coverage is needed, file a new punch-list row.
- Multi-line SQL detection — the guard uses single-line grep, which is sufficient for direct string literals and most hand-written queries. Queries composed across multiple string concatenations are not detected; that is a known limitation, acceptable given the primary threat model (accidental raw-SQL writes, not adversarial code).
- Scanning non-Go files (`.sql`, `.sh`) — migration SQL files legitimately reference `casbin_rules`; excluding them from the scan avoids noise without meaningful risk reduction.
- Changes to the Casbin policy schema or enforcement logic.

## Acceptance Criteria

- `bash scripts/lint-casbin-sql.sh` exits 0 on the current tree with output `casbin SQL guard: clean`.
- Adding a file containing `INSERT INTO casbin_rules` outside `internal/adapter/casbin/` causes the script to exit 1 and print the offending file:line to stderr.
- `make lint` runs `lint-casbin-sql` before golangci-lint; both pass on the current tree.
- GitHub Actions lint job runs the guard as a dedicated step after golangci-lint.
