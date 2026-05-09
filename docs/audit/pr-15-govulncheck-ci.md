# PR-15: govulncheck CI Job

| Field | Value |
|-------|-------|
| Branch | `ci/govulncheck` |
| Status | in review |
| Audit source | `.github/workflows/ci.yml` does not run `govulncheck`; supply-chain CVE drift is invisible until release time |
| Closes | v1.2 punch-list row #15 |

## Goal

Add CI-time vulnerability scanning of the Go module graph using Go's official `govulncheck`. Block PRs that introduce a known-vulnerable transitive dependency. Provide local parity via `make vuln`.

## Tasks

- [x] Investigate `.github/workflows/ci.yml`, `Makefile`, and existing lint pipeline so the new step matches conventions (Go install path, caching).
- [x] Add `make vuln` target to `Makefile`: installs `govulncheck@v1.3.0` (pinned), runs `govulncheck ./...` from repo root, exits non-zero on findings.
- [x] Add a CI job step in `.github/workflows/ci.yml`. Shape A chosen: new dedicated `vuln` job parallel to `lint`/`test`/`build`. Uses `actions/setup-go@v5` with `go-version-file: go.mod`. Pins `govulncheck@v1.3.0`.
- [x] Pin `govulncheck` version explicitly (`@v1.3.0` — current stable as of 2026-05-09).
- [x] Pre-existing govulncheck findings resolved via lead-authorized dep bumps: `go 1.25.1` → `go 1.25.10`, `golang.org/x/net v0.50.0` → `v0.53.0`, `gofiber/fiber/v2 v2.52.10` → `v2.52.12`. `make vuln` now exits 0 with zero findings. `make lint test` remain green.
- [x] `CHANGELOG.md` `[Unreleased]` entry under "CI / Tooling" and "Security".
- [x] Update `docs/audit/v1.2-punch-list.md` row #15 status → `in review`.

## Acceptance Criteria

- `make vuln` runs cleanly on the current tree.
- A new commit introducing a known-vulnerable dep version (test by temporarily pinning a flagged module locally, do not commit) makes `make vuln` exit non-zero.
- CI shows a `vuln` check (or step) on every PR; failure blocks merge.
- `govulncheck` version pinned (no `@latest` in CI).
- No code change beyond `Makefile` + `.github/workflows/ci.yml`.

## Out of Scope

- Scanning container images for CVEs (Trivy/Grype) — separate concern, separate PR if needed.
- SBOM generation — not requested, no current consumer.
- License scanning — out of scope for security PR.
- Rewriting any other CI job.
- Auto-bumping vulnerable deps — that is Dependabot's job (PR-16).

## Notes for Implementer

- Do not push or open the PR. Lead reviews diff in worktree before shipping.
- If a real vuln surfaces in the current tree, stop and report it to the lead before patching — may need a separate punch-list row.
- Do not skip findings via env vars or `-mode=binary` workarounds. The gate is binary: green or fail.
