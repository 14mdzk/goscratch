# PR-15: govulncheck CI Job

| Field | Value |
|-------|-------|
| Branch | `ci/govulncheck` |
| Status | planned |
| Audit source | `.github/workflows/ci.yml` does not run `govulncheck`; supply-chain CVE drift is invisible until release time |
| Closes | v1.2 punch-list row #15 |

## Goal

Add CI-time vulnerability scanning of the Go module graph using Go's official `govulncheck`. Block PRs that introduce a known-vulnerable transitive dependency. Provide local parity via `make vuln`.

## Tasks

- [ ] Investigate `.github/workflows/ci.yml`, `Makefile`, and existing lint pipeline so the new step matches conventions (Go install path, caching).
- [ ] Add `make vuln` target to `Makefile`: installs `govulncheck@latest` (or pinned version, see Acceptance), runs `govulncheck ./...` from repo root, exits non-zero on findings.
- [ ] Add a CI job step in `.github/workflows/ci.yml`. Two viable shapes:
  - **A** — new dedicated `vuln` job parallel to `lint`/`test` (cleaner reporting, separate coloured check).
  - **B** — added step inside the existing `lint` job (one fewer runner).
  - **Default lean: A** (separate job lets the operator see vuln status independently of lint flap).
  - Use `actions/setup-go@v5` with `go-version-file: go.mod` for parity with existing jobs.
  - `go install golang.org/x/vuln/cmd/govulncheck@latest` then `govulncheck ./...`.
- [ ] Pin `govulncheck` version explicitly (e.g., `@v1.1.3`) so a CVE DB rev does not silently flap CI; document the upgrade path in the PR body.
- [ ] If the current tree has any pre-existing `govulncheck` finding, fix or document via a `// govulncheck:ignore` annotation with rationale, NOT by lowering the gate. Findings must be addressed in this PR or a follow-up row added before merge.
- [ ] `CHANGELOG.md` `[Unreleased]` entry under "CI / Tooling".
- [ ] Update `docs/audit/v1.2-punch-list.md` row #15 status → `in review`.

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
