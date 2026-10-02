# OpenAPI generation for goscratch — ADR-009 selection (v1.3 ticket B2)

| Field | Value |
|---|---|
| Ticket | v1.3 Phase 6, B2 — "evaluate OpenAPI generation options for this codebase, recommend one" |
| Feeds | ADR-009 (selection); B3 (byte-diff gate); B4 (per-module generation) |
| Repo | `github.com/14mdzk/goscratch` @ `research/openapi-generation` worktree |
| Evaluated as-of | **2026-10-02** |
| Author | wayfinder research agent |

> Scope note: every version, release date and behaviour below was taken from the upstream
> project's own repo/releases/docs, or from a **local compile+run probe** in
> `$SCRATCHPAD/swagtest` and `$SCRATCHPAD/humatest` (throwaway modules, not the repo). No
> secondary blog write-ups were used. Caveats are listed at the end.

---

## 1. Recommendation (decision-ready)

**Primary: do not adopt a runtime/generation framework in v1.3. Keep the hand-edited `openapi: 3.0.3`
spec and evolve the existing B1 drift gate into a schema-aware gate. Re-baseline ADR-009 to "hand-edited
spec + drift gate" and revisit generation when a concrete trigger fires (§6).**

Rationale in one paragraph: none of the three generators reproduce this codebase's committed
`openapi: 3.0.3` contract. `swaggo/swag`'s stable line emits **Swagger 2.0 only**, and its
OpenAPI-3 line (`v2.0.0-rc*`) emits **OpenAPI 3.1.0 only** and is still a **release candidate**;
`go-swagger` is net/http-first and spec-first, a poor fit for Fiber v2; `huma` is the only option with
a first-party Fiber v2 adapter but it mandates a **handler-signature + error-model rewrite of every
module**, trades the `{success,message,data,error}` envelope for RFC 9457 `application/problem+json`,
and drags Fiber **v3** into the module graph. The cost of any migration exceeds the benefit of the
route-level guarantee B1 already gives.

**Runner-up (adopt only if a trigger fires): `swaggo/swag` v2 in `--v3.1` mode, annotation-driven.**
It is the only generator that adds **zero runtime coupling** to the Fiber app — it is a CLI that parses
Go comments, needs no adapter, no middleware and no handler changes — and its output is **byte-stable**
across runs (verified). Pick it if/when the spec is migrated to OpenAPI 3.1 for other reasons, or swag
v2 reaches GA. It is *not* free: it forces a 3.0.3 → 3.1.0 spec-format change, package-qualified schema
names, and a rewrite of the hand-written descriptions into annotations.

**Reject now:** `go-swagger` (net/http, spec-first, Swagger 2.0), `danielgtaylor/huma` (rewrite-scale
friction; see §3), `go-fuego/fuego` (no Fiber adapter, replaces Fiber, Go 1.26).

---

## 2. What constrains the answer in *this* codebase

| Lever | Where | Why it matters |
|---|---|---|
| Fiber **v2**, not v3 | `go.mod` → `github.com/gofiber/fiber/v2 v2.52.13` | Rules out Fiber-v3-only middleware; not all generators have a v2 adapter. |
| Frozen **`openapi: 3.0.3`** | `internal/module/docs/openapi.yaml:1` | Swag cannot emit 3.0.3 (stable = 2.0; v2 = 3.1). A 3.0→3.1 switch was explicitly deferred as "breaking-tooling risk" in `docs/audit/pr-14-openapi-drift-sync.md`. |
| Hand-written prose + `allOf` envelopes + examples | `internal/module/docs/openapi.yaml` (1770 lines, 31 paths, 38 operations, 34 schemas) | Regeneration as the source of truth discards or forces re-annotation of this value. |
| Existing route-vs-spec drift gate | `cmd/openapi-drift/main.go` | Already guarantees every Fiber route has a spec entry. Generation raises the ceiling, not the floor. |
| Response envelope `{success,message,data,error}` | `pkg/response/response.go` | Not the shape any generator emits by default. |
| Error codes enum (9) | `pkg/apperr/error.go:88-99`; spec `openapi.yaml:1341-1350` | Must be expressed as a `enum`. |
| JWT **bearer** security | `internal/platform/http/middleware/auth.go`; spec `openapi.yaml:1183-1188` | `type: http, scheme: bearer, bearerFormat: JWT`. |
| `go-playground/validator` tags | `internal/module/user/dto/user_dto.go:7-8` (`validate:"required,email,min=8"`) | **No generator in this set reads these natively** (§4). |
| Cursor pagination + `types.Opt[T]` query filters | `dto/user_dto.go:37-43` (`query:"cursor"`, `Opt[string]`) | Generic `Opt[T]` is not reflectable into a scalar schema by any generator. |
| Manual DI, per-module `RegisterRoutes` | `internal/platform/http/server.go:90-100`, `internal/module/user/module.go:39-58` | A code-first framework (Huma/Fuego) inverts this whole structure. |

---

## 3. Options — fit matrix

| Option | Fiber v2 fit | Glue needed | Runtime coupling | Generates | Blast radius |
|---|---|---|---|---|---|
| **swaggo/swag** (annotations) | ✅ none — CLI only, framework-agnostic | annotations on handlers + DTO tags; **no** middleware (Scalar already serves the UI) | none (build-time only) | Swagger 2.0 (stable) / OpenAPI **3.1** (`--v3.1`) | low — additive comments |
| **go-swagger** | ❌ net/http-centric | spec-first: write spec → generate server/types; or `swagger generate spec` from annotations | none (CLI) | Swagger 2.0 | high — inverts workflow |
| **danielgtaylor/huma** | ✅ official adapter `humafiber.NewV2` (added v2.38.0) | rewrite every handler to `(ctx,input)(output,error)`; register operations; override error model; register bearer scheme; redo auth/permission mw as Huma middleware | high — Huma owns routing | OpenAPI **3.1** | very high — all 8 modules + `server.go` + drift gate |
| **go-fuego/fuego** | ❌ no Fiber adapter (net/http; Gin/Echo adaptors only) | replace Fiber with Fuego's router | replaces Fiber | OpenAPI 3.1 | total rewrite, violates ADR-004 |
| **Alternative** (spec lint/validate only) | ✅ framework-agnostic | `kin-openapi` / Spectral / vacuum on the YAML | none | *validates*, not generates | low — complements B1 |

### 3a. swaggo/swag

- **Fiber fit:** generation is framework-agnostic (`swag init` parses comments), so Fiber v2 is a
  non-issue. The only Fiber-specific artifact is the Swagger-UI middleware, which this repo does **not**
  need: `internal/module/docs/module.go` already serves the YAML and renders it via Scalar.
- **Maintenance caution:** `github.com/gofiber/swagger` (the Fiber v2 UI middleware, last release
  **v1.1.1, 2025-01-10**) is **explicitly deprecated** — its README says *"no longer maintained, use
  `github.com/gofiber/contrib/v3/swaggo`"*, and its `main` `go.mod` now targets **Fiber v3**
  (`fiber/v3 v3.0.0-rc.2`). Irrelevant to generation, but confirms swaggo's Fiber story is drifting to v3.
- **Output version (verified by probe):** `swag v2.0.0-rc6` default → `swagger: "2.0"`
  (`#/definitions/...`); with `--v3.1` → `openapi: 3.1.0` (`#/components/schemas/...`). **There is no
  3.0.x mode.**
- **Determinism (verified):** two consecutive runs produced **byte-identical** `swagger.yaml` in both
  modes (`--generatedTime` defaults to `false`). Byte-diff is therefore feasible — against a
  *generated* spec, not the current hand spec.
- **Shapes:** envelope via `@Success 200 {object} PaginatedUserResponse`; error enum via a Go struct
  `enums:"BAD_REQUEST,...,VALIDATION_ERROR"`; bearer via `@securityDefinitions.bearerauth BearerAuth`.
  Schema names come out **package-qualified** (`main.UserResponse`), unlike the hand spec's bare names.
- **Migration cost:** ~one annotation block per operation (38 operations) + dual tags on DTO fields.
  No handler-logic change, no routing change, no new runtime dep.
- **Pin:** `github.com/swaggo/swag/v2 v2.0.0-rc6` (RC, 2026-09-13). Stable `v1.16.6` (2025-07-29) is
  Swagger 2.0 — unusable against a 3.0.3 spec.

### 3b. go-swagger

- Spec-first ecosystem (generate server/client/types from a spec), `swagger generate spec` is its
  annotation path; net/http-first. Emits **Swagger 2.0**. Upstream is alive — **v0.36.6 (2026-09-10)** —
  but it is architecturally the opposite of a Fiber/`fiber.Ctx` codebase and would invert the
  spec-matches-runtime direction the repo has committed to (`pr-14`: "spec must match runtime, not the
  other way around"). **Reject.**

### 3c. danielgtaylor/huma (net/http + Chi heritage — quantified honestly)

- **Fiber fit is real and official:** adapter `github.com/danielgtaylor/huma/v2/adapters/humafiber`
  exposes `NewV2(r *fiberV2.App, ...)` / `NewV2WithGroup(...)` for **Fiber v2**, added in **v2.38.0**
  (`New` targets Fiber v3, `NewV2` targets Fiber v2). I verified this by compiling and running a
  minimal Huma+Fiber v2.52.15 program: it built and emitted OpenAPI.
- **But it is a framework replacement, not an add-on.** Huma owns operation registration and the
  request/response lifecycle. Adopting it means rewriting all 38 handlers to Huma signatures,
  re-implementing `middleware.Auth`/`RequirePermission` as Huma middleware, and reconciling with
  `cmd/openapi-drift` (which walks `app.Stack()`). The pkg.go.dev docs and Huma's own README describe it
  as "a micro framework … bring your own router", i.e. it expects to be the HTTP layer.
- **Error model conflict (verified):** default error responses are `application/problem+json` with
  Huma's injected `ErrorModel`/`ErrorDetail` (RFC 9457) — the opposite of this repo's
  `{success,message,data,error}` envelope (`pkg/response/response.go`). The envelope must be re-modeled
  as custom output schemas, and `apperr` codes re-mapped.
- **Validator tags:** Huma uses its **own** tag set (`doc`,`format`,`minLength`,`maxLength`,`enum`,
  `default`,`pattern`,…) — **not** `go-playground/validator` (`validate:"required,email,min=8"`). So
  `dto` fields need parallel Huma tags too, or custom schema resolvers.
- **Security:** bearer auth is **not** inferred; the `Authorization` header surfaced (in my probe) as a
  plain header parameter, not `security:[{bearerAuth:[]}]`. A `huma.SecurityScheme` must be registered
  and attached per operation.
- **Determinism (verified):** byte-identical across runs.
- **Dependency weight (verified):** adding Huma v2.39.1 to a Fiber v2 module pulls **both**
  `gofiber/fiber/v2 v2.52.15` **and `gofiber/fiber/v3 v3.4.0`** (indirect) plus `gofiber/schema`,
  `gofiber/utils/v2`, `tinylib/msgp`, etc. Huma also requires `fiber/v2 >= v2.52.14` (repo pins
  v2.52.13). Net: a heavier module graph and a Fiber patch bump, to gain 3.1 output the repo doesn't
  currently target.
- **Pin:** `github.com/danielgtaylor/huma/v2 v2.39.1` (2026-07-29). Actively developed.
- **Verdict:** technically capable and the strongest *generation runtime*, but the per-endpoint and
  cross-cutting cost is the largest of the set and it conflicts with existing patterns. **Reject for
  v1.3; note as the best candidate if the HTTP layer is ever standardised.**

### 3d. go-fuego/fuego (the "credible Fiber-friendly alternative")

- **Not** Fiber-friendly despite the name-drop: it is 100 % `net/http`; its only framework adaptors are
  **Gin and Echo**. There is no Fiber adaptor. Its `go.mod` requires **`go 1.26.6`** (repo is on
  `go 1.25.11`). Adopting it means replacing Fiber — directly at odds with ADR-004 (Fiber over Echo).
- **Maintenance:** latest release **v0.18.0 (2025-02-05)** — pre-1.0, ~20 months stale as of 2026-10.
- **Verdict:** reject.

### 3e. Non-generator Fiber-agnostic alternative worth pairing with the baseline

- `getkin/kin-openapi` (validate/serve), Spectral/vacuum (lint), `oasdiff`/`libopenapi` (spec **diff**).
  These **validate** the hand spec and can power a structural CI gate without touching Go handlers, and
  they are what a "keep hand-edited" decision should invest in.

---

## 4. How this codebase's shapes surface in generated specs

| Shape | swaggo/swag | huma | go-swagger |
|---|---|---|---|
| Response envelope `{success,message,data,error}` | declare an envelope struct and `{object} Envelope` in `@Success`; or annotate `allOf`-equivalent per op | model as Huma output `Body`; must **override** the default RFC 9457 `ErrorModel` | declare in spec (spec-first) |
| `apperr` code enum (9 values) | `enums:"..."` tag on the `code` field (verified output) | `enum:"a,b,c"` tag on the field | spec enum (spec-first) |
| JWT bearer | `@securityDefinitions.bearerauth BearerAuth` + `@Security BearerAuth` (emits `type:http, scheme:bearer`; **no `bearerFormat` unless added**) | register `huma.SecurityScheme`, attach `op.Security`; not inferred (probe showed a plain header param) | spec `securityDefinitions` (spec-first) |
| `go-playground/validator` tags | **not read** — swag reads `validate` only for `required`/`optional`; `email`/`min`/`max` come from separate `format`/`minLength`/`maxLength` tags → **dual-tag DTOs** | **not read** — Huma uses its own `format`/`minLength`/`pattern`… tags → **dual-tag DTOs** | not read |
| Cursor pagination query params (`cursor`,`limit`) | `@Param cursor query string`, `@Param limit query int … default(20)` (verified) | `query:"cursor"`, `query:"limit" default:"20" minimum:"1" maximum:"100"` (verified) | spec params (spec-first) |
| `types.Opt[T]` filters (`search`,`email`,`is_active`) | reflected as a nested object → needs `swaggertype:"string"` override / override file | reflected as `{Set,Val}` object → needs a custom schema/resolver | n/a |

**The validator-tag gap is the single biggest per-field friction for swag:** every constrained DTO field
needs a second, parallel schema tag (`format:"email"`, `minLength:"8"`, …) or a `.swaggo` override,
because `validate:"required,email,min=8"` is opaque to both generators.

---

## 5. Migration cost & determinism / byte-diff (does it satisfy B3?)

### 5a. Per-endpoint migration cost

| Option | Per-operation work | Cross-cutting work | New runtime dep |
|---|---|---|---|
| swaggo (annotations) | +1 annotation block (~6–10 lines) per op × 38 ops; DTO dual-tags | CI: `swag init` + `git diff --exit-code`; decide spec as artifact | none |
| huma | rewrite handler signature + input/output structs per op × 38; re-model errors | replace authz mw, rewire `server.go`, revisit drift gate; per-op `Security` | huma + **fiber/v3** + schema/utils |
| go-swagger | spec-first: relocate contract into spec, regenerate scaffolding | invert workflow | CLI only |
| Fuego | full router replacement | rewrite bootstrap + all middleware | fuego + Go 1.26 |

Roughly: swag ≈ *hours* of comment authoring + a CI job; huma ≈ *days–weeks* of code surgery.

### 5b. Determinism and the B3 byte-diff premise — **verified empirically**

- **swag v2.0.0-rc6:** two runs → **byte-identical** `swagger.yaml` (both 2.0 and 3.1 modes); no
  timestamps by default.
- **huma v2.39.1:** two runs → **byte-identical** output.

So generators *can* be byte-diffed and driven by CI. **But B3 as literally worded cannot be met by any
option:** the committed spec is `openapi: 3.0.3` with `#/components/schemas/`, hand-written `allOf`
envelopes, prose and examples. swag emits **2.0 or 3.1** and package-qualified names
(`main.UserResponse`); huma emits **3.1** with injected `ErrorModel`. Neither reproduces the current
file byte-for-byte.

**Consequence for B3:** B3 must be re-read as *"the committed `openapi.yaml` becomes a generated
artifact; CI regenerates and fails on any diff"* (the `swag init && git diff --exit-code` pattern).
Under that reading:
- swag v2 satisfies it **only after** a one-time spec-format migration to 3.1 and re-authoring of the
  prose as annotations;
- the hand-edited baseline satisfies it trivially today (the gate compares committed vs committed).

If B3 is meant to *preserve* the current 3.0.3 file as the source of truth, then **no generator passes**
and the baseline is the only correct choice.

---

## 6. Maintenance signals & exact pins (as-of 2026-10-02)

| Project | Latest release | Date | Notes |
|---|---|---|---|
| `swaggo/swag` (v1, stable) | **v1.16.6** | 2025-07-29 | Swagger 2.0 only. Active. |
| `swaggo/swag` (v2, OpenAPI 3) | **v2.0.0-rc6** | 2026-09-13 | **Still an RC** (rc1→rc6 over ~2 yrs). `-v3.1` opt-in; default still 2.0. |
| `gofiber/swagger` (UI mw) | **v1.1.1** | 2025-01-10 | **Deprecated**; successor `gofiber/contrib/v3/swaggo` targets Fiber **v3**. |
| `go-swagger/go-swagger` | **v0.36.6** | 2026-09-10 | Active. |
| `danielgtaylor/huma` | **v2.39.1** | 2026-07-29 | Active; Fiber v2 adapter since v2.38.0. |
| `go-fuego/fuego` | **v0.18.0** | 2025-02-05 | Pre-1.0; no Fiber adaptor; `go.mod` needs Go 1.26.6. |

**Trigger conditions to revisit generation:**
1. `swaggo/swag` v2 reaches **GA** (non-RC), **or**
2. the project migrates the spec to **OpenAPI 3.1** for reasons independent of B2, **or**
3. the HTTP layer is standardised (at which point re-evaluate **huma** with `humafiber.NewV2`).

Until a trigger fires, the honest ADR-009 outcome is: **hand-edited 3.0.3 + a stronger drift gate.**

---

## 7. Incidental finding (worth a punch-list row)

There are **two** git-tracked, hand-maintained specs: `internal/module/docs/openapi.yaml` (52 KB,
**embedded** by `internal/module/docs/module.go` and the one B1 guards) and `docs/openapi.yaml`
(58 KB). Both declare `openapi: 3.0.3`, both list the **same 31 paths**, but the files **differ** byte-for-byte.
The drift gate only reads the embedded one. Any generation decision must first collapse these to a
single source of truth (otherwise the "generated vs committed" diff is meaningless). Not fixed here —
out of scope, read-only ticket.

---

## 8. Sources (primary, with versions)

Upstream repos/releases/docs:
- `swaggo/swag` releases & tags — `v1.16.6` (2025-07-29), `v2.0.0-rc6` (2026-09-13); README (`master`,
  `v2` branch) documenting Swagger 2.0 output, the `--v3.1` flag, and the `validate` field
  `required`/`optional`-only semantics.
- `swaggo/swag` probe run locally: `swag v2.0.0-rc6` default → `swagger:"2.0"`; `--v3.1` →
  `openapi: 3.1.0`; byte-identical across runs.
- `gofiber/swagger` README (deprecation notice) + `go.mod` (`fiber/v3 v3.0.0-rc.2`).
- `gofiber/contrib` README (successor `v3/swaggo`).
- `go-swagger/go-swagger` release `v0.36.6` (2026-09-10).
- `danielgtaylor/huma` release `v2.39.1` (2026-07-29); `pkg.go.dev/.../adapters/humafiber`
  (`NewV2` for Fiber v2, added v2.38.0); `huma.rocks/features/request-validation/` (its own tag set);
  README (RFC 9457 default errors).
- `danielgtaylor/huma` probe run locally: compiles against Fiber v2.52.15; `openapi: 3.1.0`;
  `application/problem+json` + `ErrorModel`; byte-identical across runs; pulls `fiber/v3 v3.4.0` indirect.
- `go-fuego/fuego` release `v0.18.0` (2025-02-05); README; `go.mod`.

Repo (read-only):
- `internal/module/docs/openapi.yaml` (1770 lines, 31 paths, 38 ops, 34 schemas) — envelope/security/enum shapes.
- `pkg/response/response.go`, `pkg/apperr/error.go` — envelope + 9 error codes.
- `internal/platform/http/{server.go,middleware/auth.go}`, `internal/module/{user/module.go,user/dto,user/handler}`, `internal/platform/validator/validator.go`.
- `cmd/openapi-drift/{main.go,drift_test.go}`.
- `docs/audit/{v1.3-plan-revised.md,v1.3-pr-B1-openapi-drift-ci.md,pr-14-openapi-drift-sync.md}`; `docs/adr/` (no ADR-009 yet).

## 9. Confidence caveats — what I could NOT verify

- **No end-to-end probe against the repo's own spec/DTOs.** The swag and huma probes are *minimal
  throwaway modules in the scratchpad* that mirror the codebase's shapes; they do not exercise the real
  38 handlers, the real `internal/module/docs/openapi.yaml`, or the drift gate.
- **Byte-diff between a *generated* spec and the *current* hand spec was not executed** (the generators
  can't target 3.0.3, so such a diff is structurally impossible today). Determinism was verified
  run-to-run, not generated-vs-committed.
- **swag v2 is an RC.** Behaviour observed on `v2.0.0-rc6` may change before GA; the `--v3.1`-only
  output and package-qualified names are pinned to that RC.
- **`swaggo/swag` stable vs v2 semantics:** the `v2` module name denotes the *tool* major version —
  its default output is still Swagger 2.0. I verified the emitted versions directly rather than trusting
  the module name.
- **`gofiber/contrib` v2 (Fiber v2) swaggo middleware:** I could not retrieve a `v2/` subtree from the
  contrib repo (404), so I state only that the *named successor* is `contrib/v3` (Fiber v3). This is
  moot for generation (Scalar already serves the UI) but relevant if the repo ever wants swaggo's UI mw
  on Fiber v2.
- **Huma's exact OpenAPI-3.0 downgrade options** were not exercised; I observed 3.1.0 output only. Huma
  documents OpenAPI **3.1** as its target.
- **`types.Opt[T]` behaviour** under each generator is inferred from how each reflects structs, not
  executed against the actual `ListUsersRequest`.
- Release dates/versions reflect the upstream state on **2026-10-02**; pins should be re-checked at
  ADR-009 authoring time.
