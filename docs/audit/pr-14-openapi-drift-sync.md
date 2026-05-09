# PR-14: OpenAPI Drift Sync (post-v1.1)

| Field | Value |
|-------|-------|
| Branch | `docs/openapi-v1.1-drift-sync` |
| Status | planned |
| Audit source | v1.1 ship cut left `docs/openapi.yaml` behind several behaviour changes (auth hardening, upload validation, rate limiting) |
| Closes | v1.2 punch-list row #14 |

## Goal

`docs/openapi.yaml` does not reflect the runtime behaviour shipped in v1.1. External clients reading the spec receive a wrong contract — missing auth requirements, missing 415/429 responses, incorrect refresh request body. Sync the spec to behaviour without changing any runtime code.

## Tasks

- [ ] Investigate `docs/openapi.yaml` against runtime behaviour established by PR-03 (auth hardening), PR-05 (upload streaming + content-type sniff), PR-09 (rate-limit hardening). Cross-reference against the actual handler code paths in `internal/module/auth/handler.go`, `internal/module/storage/handler.go`, `internal/middleware/ratelimit/`.
- [ ] `/auth/logout`: add `security: [{ bearerAuth: [] }]`; runtime requires `Authorization: Bearer <access_token>` since PR-03.
- [ ] `POST /api/files/upload`: add `415 Unsupported Media Type` response referencing `apperr.UnsupportedMediaType` schema; runtime rejects when `http.DetectContentType` mismatches the upload allowlist.
- [ ] `POST /auth/login` and `POST /auth/refresh`: add `429 TooManyRequests` response + `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` response headers. Define a reusable `responses.TooManyRequests` component and a reusable `RateLimit-*` header set; reference from both endpoints. Apply the same `429` to other globally-rate-limited routes if the global limiter wraps them at the router level (verify against `internal/platform/server/server.go` middleware order before annotating).
- [ ] `POST /auth/refresh` request body description: state explicitly that `user_id` is **not** accepted; the server resolves the userID from the lookup key. Extra fields are ignored, not validated. Update the schema example accordingly.
- [ ] `/metrics`: confirm absent from the spec (correct — bound to `127.0.0.1` since v1.1; not a public surface).
- [ ] Coordinate with PR-13: if PR-13 lands first and adds `/healthz/live` + `/healthz/ready`, this PR reconciles those paths into the spec. If this PR lands first, leave them out.
- [ ] Regenerate Scalar `/docs` rendering and verify in-browser that all four changes render correctly (no schema lint errors, no broken `$ref`).
- [ ] Run any spec linter the repo has (`spectral` or `redocly` if wired); otherwise eyeball-validate via Scalar's built-in lint.
- [ ] `CHANGELOG.md` `[Unreleased]` entry under "Documentation".
- [ ] Update `docs/audit/v1.2-punch-list.md` row #14 status → `in review`.

## Acceptance Criteria

- `make lint test` clean (no Go change; gates the YAML cleanly).
- Scalar `/docs` renders without schema errors.
- `/auth/logout`, `/auth/login`, `/auth/refresh`, `POST /api/files/upload` reflect the v1.1 runtime contract exactly.
- No runtime code changed in this PR (docs-only).
- Reusable `responses.TooManyRequests` component + `RateLimit-*` headers defined once and `$ref`-d, not duplicated per endpoint.

## Out of Scope

- New endpoints or schema additions for unimplemented behaviour.
- Renaming existing schemas to "improve naming" — pure churn, breaks clients.
- Switching spec format (3.0 → 3.1) — separate concern with breaking-tooling risk.
- Generating client SDKs — not asked, and would be churn.
- Adding `/healthz/*` if PR-13 has not landed yet — merge order decides who owns those paths.

## Notes for Implementer

- Do not push or open the PR. Lead reviews diff in worktree before shipping.
- Do not change handler code. Spec must match runtime, not the other way around. If a discrepancy reveals a runtime bug, file a new punch-list row instead.
- Verify each `429` annotation against actual middleware order; do not annotate a route that is not in fact rate-limited.
