# ADR-011: OpenAPI Spec — Hand-Edited 3.0.3 + Schema-Aware Drift Gate

**Date:** 2026-10-06
**Status:** Accepted
**Deciders:** 14mdzk (wayfinder [ticket #92])

---

## Context

The v1.3 plan (Tier B) set out to replace the hand-maintained `openapi: 3.0.3`
spec with a generated artifact: B2 selected a generator, B3 migrated modules to
annotation-driven output, and B4 generated `info.version`/`servers`. B1 had
already shipped the route-vs-spec drift gate (`cmd/openapi-drift`, `make
openapi-drift`, CI lint step) guarding the spec at
`internal/module/docs/openapi.yaml`.

The B2 research ([OpenAPI generation for goscratch][research]; branch
`research/openapi-generation` @ `22ddbd9`; evaluated 2026-10-02, versions
re-verified 2026-10-06) found that **no generator reproduces the committed
contract**:

- `swaggo/swag` stable emits Swagger 2.0 only; its OpenAPI-3 line
  (`v2.0.0-rc6`, still a release candidate as of 2026-10-06) emits OpenAPI 3.1
  only, with package-qualified schema names.
- `danielgtaylor/huma` is a framework replacement: all 38 handlers rewritten,
  the `{success,message,data,error}` envelope traded for RFC 9457
  `application/problem+json`, and Fiber v3 pulled into the module graph.
- `go-swagger` is net/http-first and spec-first; `go-fuego/fuego` requires
  Go 1.26 and replaces Fiber — both conflict with ADR-004.

No candidate reads `go-playground/validator` tags, so constrained DTO fields
would need parallel schema tags in every option. Byte-equivalence against the
committed file is structurally unreachable: no candidate emits `openapi: 3.0.3`.

Two divergent git-tracked specs exist at decision time:
`internal/module/docs/openapi.yaml` (52 KB; embedded via `go:embed`, served at
`/docs/openapi.yaml`, B1-gated) and `docs/openapi.yaml` (58 KB; referenced by no
code, drifted both ways). The docs UI is Scalar, served by the docs module;
`gofiber/swagger` is deprecated and its successor targets Fiber v3.

## Decision

**Stay hand-edited. No generator in v1.3; evolve the B1 gate instead.**

1. **Generator: none adopted.** swaggo/swag is rejected — the v2 line is still
   an RC and 3.1-only, and the project standardised on Scalar (no swagger
   tooling in the stack). huma, go-swagger, and fuego are rejected per the
   research's fit matrix (framework rewrite / net/http-first / Fiber
   replacement respectively).
2. **Single source of truth: `internal/module/docs/openapi.yaml`.** `go:embed`
   pins the spec to the docs package; that file is embedded, served, and gated.
   `docs/openapi.yaml` is retired: its unique content (notably the richer
   health-endpoint prose and readiness 503 documentation) is folded into the
   canonical file, then the file is deleted. Execution: one small PR, alongside
   the health pilot.
3. **B3 recast — one-time reconciliation sweep.** Per-module PRs review every
   operation of the canonical spec against runtime (handler + DTO +
   middleware); mismatches are fixed in the same PR. Health first as pilot:
   fold + reconcile `/health`, `/healthz/live`, and `/healthz/ready` against
   `internal/module/health/handler.go`, and capture the review method as the
   template for the rest. Order after health: auth, user, role, storage, sse,
   job, docs. Acceptance per module: gate green + reviewed conformance. No
   annotation migration; a kin-openapi response-validation contract-test harness
   was considered and deferred as an upgrade path.
4. **B4 recast — static + release-cut sync.** `info.version` joins the release
   checklist (it reads `0.5.0` today); `servers` stays as committed. No
   build-time generation: the committed file remains the source of truth.
5. **Gate: schema-aware via kin-openapi inside `cmd/openapi-drift`.** The gate
   keeps its route diff and adds document validation: the spec parses as valid
   OpenAPI 3.0.x, every `$ref` resolves, operations are complete. Same binary,
   same `make openapi-drift` target, same CI step; kin-openapi (v0.149.0) is
   added as a direct tooling dependency of the module.

**Revisit triggers** (a fired trigger opens a re-evaluation, not an
auto-adoption):

- the spec moves to OpenAPI 3.1 for independent reasons — lifts the format
  objection for every candidate;
- the HTTP layer is standardised — re-opens the huma-class options.

swaggo/swag is excluded by a standing project position (Scalar is the docs
stack; no swagger tooling), so its GA status is not by itself a trigger.
oasdiff (breaking-change detection) and vacuum/spectral (rule linting) remain
post-v1.3 options, not triggers.

## Rationale

B1 already closes the route-drift class at near-zero cost. What remained open
was semantic drift — stale prose, silently diverged files, the repeated
spec-lag PRs (PR-13, PR-14, PR-53) — and the research showed generation's price
(a 3.0.3→3.1 format move, ~38 annotation blocks, parallel schema tags on every
constrained DTO, or a full framework rewrite) far exceeds its benefit against a
contract the project deliberately hand-authors and renders with Scalar. The
reconciliation sweep retires the observed bug class at review cost, and the
kin-openapi extension turns structural rot (dangling refs, invalid documents)
into a CI failure rather than a review hope.

## Supersedes

- Tier B rows B2–B4 as written in `docs/ROADMAP.md` (generator selection,
  annotation migration, generated `info.version`/`servers`). The ROADMAP sync
  lands with execution, not with this ADR.
- The plan's B3 acceptance ("generated spec byte-diffed against the hand-edited
  block") — structurally unmet by design; replaced by reviewed conformance plus
  the strengthened gate.

## Consequences

- v1.3 Tier B execution becomes: (1) fold + delete `docs/openapi.yaml` with the
  health pilot, (2) per-module reconciliation PRs, (3) the gate-extension PR,
  (4) the release-checklist line for `info.version`.
- The server gains no runtime dependency; kin-openapi is a tooling dependency
  used only by `cmd/openapi-drift`.
- "The spec is generated" is explicitly dropped for v1.3 — revisitable only via
  the triggers above.
- Deferred and recorded: swag v2 / huma re-evaluation; contract-test harness;
  oasdiff; a 3.1 migration.

## Sources

- [OpenAPI generation for goscratch — ADR-009 selection (v1.3 ticket B2)][research]
  — research branch `research/openapi-generation`, commit `22ddbd9`.
- `cmd/openapi-drift` (B1), `internal/module/docs/` (embed + Scalar), and the
  PR-14 / PR-53 drift history.

[ticket #92]: https://github.com/14mdzk/goscratch/issues/92
[research]: https://github.com/14mdzk/goscratch/blob/22ddbd91718a44df138f1e45979f370c44cba28e/docs/research/openapi-generation.md
