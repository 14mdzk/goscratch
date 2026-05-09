# PR-16: Dependabot Config

| Field | Value |
|-------|-------|
| Branch | `ci/dependabot` |
| Status | in review |
| Audit source | No `.github/dependabot.yml` exists; go.mod, GitHub Actions, and Docker base images are never auto-bumped |
| Closes | v1.2 punch-list row #16 |

## Goal

Enable weekly automated dependency-update PRs across the three ecosystems used by this repo: Go modules, GitHub Actions, and Docker base images. Group minor + patch updates per ecosystem into a single weekly PR to avoid PR storm; major updates open individually so a human can review breaking changes.

## Tasks

- [x] Investigate `go.mod`, `.github/workflows/*.yml`, and all `Dockerfile`/`Dockerfile.*` paths in the repo to enumerate the ecosystems and Dockerfile directories Dependabot must monitor.
- [x] Create `.github/dependabot.yml`:
  - `version: 2`
  - `updates:` block with three ecosystems:
    - `package-ecosystem: gomod`, `directory: "/"`, `schedule.interval: weekly`, `groups: minor-and-patch: { update-types: [minor, patch] }`, label `dependencies`, `open-pull-requests-limit: 5`.
    - `package-ecosystem: github-actions`, `directory: "/"`, weekly, grouped same way, label `ci`.
    - `package-ecosystem: docker` for each Dockerfile directory found (likely `/` or `/deploy/docker/api`, `/deploy/docker/worker` — confirm during investigation), weekly, label `docker`.
  - Major updates remain ungrouped (Dependabot default).
- [x] If a `dependabot.yml` already exists with partial config, extend it; do not overwrite without confirming with lead.
- [x] Verify YAML lint via `yamllint` if installed, otherwise via the GitHub Dependabot config validator (`.github/dependabot.yml` is parsed by GitHub on push).
- [x] `CHANGELOG.md` `[Unreleased]` entry under "CI / Tooling".
- [x] Update `docs/audit/v1.2-punch-list.md` row #16 status → `in review`.

## Acceptance Criteria

- `.github/dependabot.yml` exists, valid YAML, schema v2.
- All three ecosystems present (gomod, github-actions, docker) with weekly schedule.
- Minor + patch grouped per ecosystem; majors ungrouped.
- `open-pull-requests-limit: 5` per ecosystem to cap noise.
- Each Dockerfile in the repo is monitored (no missed directories).
- After merge: within one week, expect Dependabot to open at least one initial PR (verify via `gh pr list -A app/dependabot` once GitHub picks it up; not blocking on merge).

## Out of Scope

- Auto-merging Dependabot PRs — separate concern, would need a workflow + risk policy.
- Renovate config — same job, different tool, no reason to switch.
- Custom commit-message conventions for Dependabot — defaults are fine.
- Allowlist/denylist of specific packages — premature; address per-PR if a problematic dep appears.

## Notes for Implementer

- Do not push or open the PR. Lead reviews diff in worktree before shipping.
- Do not modify any other workflow file. This PR is config-only.
- Confirm Dockerfile paths by listing them; do not invent directories.
