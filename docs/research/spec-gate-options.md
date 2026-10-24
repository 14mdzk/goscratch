# Research: spec-gate hardening options (oasdiff, spectral, contract tests)

Ticket: #143 (child of v1.4 Route Map #141). Question: which spec-gate
hardening options fit this codebase's hand-edited `openapi: 3.0.3` contract
(ADR-011: stay hand-edited; `cmd/openapi-drift` + kin-openapi is the baseline gate).

Baseline (local, read 2026-10-24): `cmd/openapi-drift/main.go` diffs Fiber
routes vs `internal/module/docs/openapi.yaml` paths (structural `{*}` param
normalisation) and validates the document with kin-openapi v0.149.0
(`go.mod`); CI runs it as `go run ./cmd/openapi-drift` in the `lint` job
(`.github/workflows/ci.yml`). The spec is 1961 lines / 31 paths / 38
operations, `info.version: "1.3.0"`, single `servers` entry
`http://localhost:8080`. Every claim below is cited to the primary source
that owns it; local findings were checked by static inspection of the
committed spec, not by running the candidate tools.

## 1. oasdiff as a breaking-change CI gate

oasdiff compares a base spec against a revision spec and classifies each
detected change; its catalogue counts 773 change kinds across 10 OpenAPI
areas: 350 breaking, 19 warnings, 404 informational [S1][S2]. The CLI
surface is `oasdiff breaking base.yaml revision.yaml` (exit 1 when breaking
changes are found), plus `changelog`, `diff`, and `validate` subcommands,
with output formats text/json/yaml/html/markdown/githubactions [S3]. The
GitHub Action `oasdiff/oasdiff-action/breaking` diffs the PR spec against
the base branch and emits inline `::error::` annotations, driven by
`base:`/`revision:` inputs [S4].

### What it flags (and why that matches this repo's risk)

oasdiff judges a change against the declared contract, not server leniency:
narrowing a request (e.g. adding a required request property, even one with
a default) is breaking; widening a response is breaking; withdrawing a
status code or media type a client may rely on is breaking; a change the
definition cannot prove safe is a warning [S2]. That derivation is enforced
by tests in the oasdiff codebase so severity cannot drift from the rule
[S2]. For this repo the drift class that matters is exactly this one: the
B1 gate only sees method+path shape, so a handler that starts requiring a
new field, drops a status code, or renames a schema property passes the
current gate silently — oasdiff flags all three.

### False-positive rate on this spec: low but structurally biased to over-report

Two properties bound the noise. First, the stated policy is "when a change
cannot be proven safe, oasdiff reports it as breaking: reporting a safe
change as breaking costs a reviewer one look, while reporting a breaking
change as safe ships it to production" [S2]. Expect intentional
over-reporting on the compatible-evolution side — e.g. a new *optional*
response property widens the response set and is therefore breaking under
the derivation even though most JSON clients ignore unknown fields. That is
a deliberate policy choice, not a bug, and the correct response is triage,
not distrust. Second, this spec's idioms interact with two matching
options: endpoint matching ignores path-parameter names by default
(`include-path-params: false`) [S4], which agrees with the drift gate's
`{*}` normalisation (`main.go` `normalizePath`), and nearly every response
uses the `allOf` envelope pattern (`SuccessResponse` + `data` override), for
which the action offers `flatten-allof` to merge `allOf` before diffing
[S4] — that flag should be on from day one.

### Config surface (enough for a gate, small enough to own)

Fail threshold `fail-on: ERR|WARN` (empty = report only) [S4]; kind-level
exclusion via `exclude-elements` (endpoints, request, response, description,
title, summary, examples…) [S4][S5]; regex suppression files
`err-ignore`/`warn-ignore` for accepted breaks [S4][S6]; custom severity
overrides via `severity-levels` [S6]; `filter-extension` to skip paths
carrying a marker extension [S4]; `composed` mode for multi-file specs (not
needed: single file here); everything committable as `.oasdiff.yaml` at the
repo root (CLI flags override file values; `with:` inputs override file
values in the action) [S4][S6]; `review: false` keeps the comparison inside
CI instead of uploading an encrypted side-by-side review to oasdiff.com
[S4]. Install in CI needs no new toolchain decision beyond what the repo
already does: prebuilt binaries, `brew install oasdiff`, the
`curl … install.sh` installer, a Docker image, or the action itself [S3][S4];
`go install` of the module is a further option since oasdiff is itself a Go
program built on kin-openapi [S7][S8].

### Alternatives considered

`oasdiff validate spec.yaml` [S3] adds nothing here — kin-openapi already
validates the document inside `cmd/openapi-drift`. Optic-style
(opt-in review/approve flows) belong to oasdiff Pro [S4] and exceed the
ask; the free `breaking` action plus `.oasdiff.yaml` covers the gate.

## 2. Spectral as a spec-lint gate

Spectral is a generic YAML/JSON linter driven by a ruleset file (convention
`.spectral.yaml`); the documented starting point is
`extends: ["spectral:oas", …]` and `spectral lint myapifile.yaml` [S9], with
a `stoplightio/spectral-action` that lints a `file_glob` and honours the
repo-root `.spectral.yaml` (or a `spectral_ruleset` input) [S10]. Install is
npm (`@stoplight/spectral-cli`), yarn, or Docker [S9] — the one real cost
for this repo, which is otherwise Go-only in CI.

### Which ruleset fits: `spectral:oas` with the `recommended` subset

The `oas` ruleset is defined in source for all of OAS 2/3.0/3.1/3.2, each
rule carrying `recommended: true|false` and an optional severity override;
rules without an override default to warn, `severity: 0`/`'error'` means
error [S11]. The `recommended` subset for a 3.0.x document is the right
scope: it covers structural validity (`oas3-schema`, error — redundant with
the kin-openapi gate but harmless), route/parameter integrity
(`path-params`, `operation-operationId-unique`, `operation-parameters`,
`operation-tag-defined`, all recommended; the first two error-severity),
response completeness (`operation-success-response`), example/schema
consistency (`oas3-valid-media-example`, `oas3-valid-schema-example`,
`typed-enum`, `array-items` — several error-severity), and hygiene
(`operation-description`, `operation-tags`, `info-contact`,
`info-description`, `oas3-api-servers`, `oas3-server-trailing-slash`,
`oas3-server-variables`, `oas3-unused-component`, `no-$ref-siblings`)
[S11][S12]. Deliberately out of scope: the `false`-recommended style rules
(`info-license`, `license-url`, `openapi-tags-alphabetical`,
`operation-singular-tag`, `tag-description`, `oas3-parameter-description`,
`oas2-*`) [S11] — adopting them would be inventing a style guide, not
closing drift. A security-ruleset add-on (`spectral:owasp`) exists in the
same ecosystem [S13] and is a later, separate decision.

### Signal vs noise on the current spec: expect near-clean

Checked statically against the committed spec (38 operations, 31 paths):

- Clean: all 38 operations carry `operationId`, `tags`, and `description`;
  all `operationId`s unique; every used tag is in the global `tags` list
  and vice versa; both path parameters (`id`, `role`) are declared
  `in: path, required: true` in `components/parameters` and referenced via
  `$ref` (the `path-params` rule runs on the resolved document) [S11][S14];
  every `type: array` (11) has a sibling `items` (11); all schemas,
  responses, parameters, and headers in `components` are referenced
  (`oas3-unused-component` only scans those component types, so the
  `bearerAuth` scheme used via `security:` requirements is not flagged)
  [S15]; `$ref`s stand alone in their mappings (`no-$ref-siblings`,
  error-severity, fires zero times); `servers` is present with no trailing
  slash; no `externalValue` (the examples xor rule is satisfied).
- Must confirm with a trial run before enforcing: the error-severity example
  rules (`oas3-valid-media-example`, `oas3-valid-schema-example`) validate
  every inline `example:`/`default:` against its schema [S12] — the spec has
  dozens (e.g. `example: 5`, `example: "secretpass"`, `example: admin` on a
  `RoleName` schema) and a single mismatch fails the gate. That trial run
  is the whole adoption risk; everything else above says the recommended set
  will be quiet.

### Alternatives considered

vacuum (pb33f) is the Go-native alternative: single binary, no Node
toolchain, and it consumes Spectral rulesets / emits Spectral-compatible
reports [S16] — strictly better CI fit for a Go-only repo *if* a trial run
confirms its `oas` coverage matches the rules above; evaluate it head to
head with Spectral in the pilot rather than committing now. Redocly CLI
lint is a third option but repeats Spectral's Node-toolchain cost with no
fit advantage identified.

## 3. Contract-test harness (spec-vs-handler response validation)

ADR-011 already names the shape: "a kin-openapi response-validation
contract-test harness was considered and deferred as an upgrade path" [S17].
The cheapest harness that closes the remaining drift class (handler returns
something the spec does not describe, or vice versa) without a full e2e
suite is an in-process Go test that the repo can already build:

- kin-openapi is a *direct* dependency (v0.149.0) with documented
  request/response validation entry points: `openapi3filter.ValidateRequest`
  taking `RequestValidationInput{Request, PathParams, Route}` and
  `openapi3filter.ValidateResponse` taking
  `ResponseValidationInput{RequestValidationInput, Status, Header}` plus
  `SetBodyBytes` [S18]. Route lookup comes from the `routers` package,
  which ships both a `gorillamux` and a dependency-free `legacy` router
  [S19] — the legacy router plus `http.Request` values synthesised from
  Fiber's `app.Test` responses avoids every new dependency: build the same
  stub-dependency Fiber app `cmd/openapi-drift` already builds, exercise
  each operation with `app.Test`, feed status/headers/body into
  `ValidateResponse`. Handlers needing live stores stay out of scope exactly
  as they are in the drift gate (nil-safe/stub registration); contract tests
  cover the deterministic routes first and skip the rest with a named
  reason, mirroring the gate's exclusion-list precedent.
- Thinner wrappers exist and add no capability worth adopting yet:
  `httptest-openapi` ("Go OpenAPI Contract Verification for use with
  `net/http`") and `apisprout` ("lightweight … OpenAPI 3 mock server with
  validation") are both listed as kin-openapi dependents [S7]; either can
  replace hand-rolled glue later. Heavier harnesses (property-based servers
  such as schemathesis, full e2e suites) were not evaluated: they violate
  the "without a full e2e suite" constraint in the question.

What it catches that neither gate above can: status codes the handler emits
but the spec omits (and vice versa), envelope-shape violations
(`success/message/data/error`), and schema violations in real bodies. What
it does not do: replace the drift gate (route coverage stays the gate's
job) or the breaking-change gate (cross-revision policy stays oasdiff's).

## 4. `info.servers` env templating that stays valid and drift-gated

The OpenAPI 3.0.3 Server Object already provides the mechanism, so no
templating layer is needed: `url` is REQUIRED, supports `{variable}`
substitution, and MAY be relative; each variable takes a `variables` entry
whose `default` is REQUIRED (and SHOULD match `enum` when present) [S20].
Consequences for the scope decision:

- The committed file stays exactly as committed — a single
  `http://localhost:8080` entry, an explicit multi-entry list
  (development/staging/production, the spec's own example pattern) [S20],
  or a variableised URL such as
  `https://{environment}.example.com:{port}/{basePath}` with defaults —
  are *all* valid 3.0.3 documents. kin-openapi validation keeps passing
  unchanged, and the drift gate is structurally blind to `servers` (it only
  reads `paths` for method+path keys [S21]), so servers edits can never trip
  it. Scalar renders the `servers` array as an environment picker natively,
  and the docs module serves the embedded file untouched (`go:embed` +
  `/docs/openapi.yaml` handler) [S22].
- Therefore: do not introduce build-time env substitution, `sed`-style
  rewriting, or a second served copy — any of those forks the served
  contract from the gated file and re-creates the two-spec drift ADR-011
  just retired [S17]. If per-env defaults are wanted later, use Server
  Variables with committed defaults; the `oas3-server-variables` spectral
  rule (error severity, `requireDefault: true`) then enforces the defaults
  stay present [S12]. Note the drift gate's `normalizePath` strips trailing
  slashes for *paths*; keep `servers` URLs slash-free independently
  (`oas3-server-trailing-slash` covers it if Spectral is adopted) [S12].

## 5. Recommendation (for the scope decision)

Adopt in this order; each step is independently shippable and none reopens
ADR-011's hand-edited-3.0.3 position [S17]:

1. **oasdiff breaking-change gate (do first).** Highest value per line of
   config: it is the only option that sees cross-revision semantic breaks
   (required-field additions, dropped statuses, schema renames) that the
   route-shape gate cannot. Concrete scope: `oasdiff/oasdiff-action/breaking`
   (or the CLI equivalent if the team prefers one mechanism in CI) with
   `fail-on: ERR`, `flatten-allof: true`, committed `.oasdiff.yaml`, and
   `review: false` until the team wants shared review links; WARN stays
   advisory while `warn-ignore` accumulates. Risk is bounded: the
   over-reporting policy [S2] is handled by triage + ignore files, and the
   repo's `allOf` envelopes + `{*}`-style params already have matching
   flags [S4].
2. **Spectral lint pilot (do second, time-boxed).** Scope to
   `extends: ["spectral:oas"]` with the `recommended` subset only; the
   static audit above predicts near-zero findings, but the pilot's exit
   criterion is one trial run proving the error-severity example rules pass
   on the current spec. Run vacuum in the same pilot (Go binary, Spectral
   ruleset compatible [S16]) and pick whichever is quieter to operate; the
   ruleset file is portable between them. Cost to acknowledge: a Node
   toolchain in CI if Spectral wins.
3. **kin-openapi response-validation contract tests (do third, thinnest
   slice).** A Go test beside `cmd/openapi-drift` reusing its stub-app
   construction, driving deterministic routes through `app.Test` into
   `ValidateResponse` [S18][S19]; no new dependencies (kin-openapi direct,
   legacy router stdlib-only). Start with health/auth happy paths; expand
   per module as in the B3 sweep precedent [S17].
4. **`servers`: change nothing about the mechanism.** Keep the single
   committed entry (or a variableised URL with defaults when multi-env is
   genuinely needed) [S20]; explicitly rule out build-time substitution or a
   second served copy. No gate changes required — neither the drift gate
   [S21] nor oasdiff's breaking derivation (server edits touch neither the
   valid-request nor valid-response set [S2]) treats this as breakage.

Explicit non-goals for v1.4: OpenAPI 3.1 migration (a listed revisit
trigger, not a gate) [S17]; Pro/approve-flow tooling [S4]; the OWASP
ruleset or non-`recommended` style rules [S11][S13]; any e2e or
property-based harness.

## Sources

- [S1] oasdiff breaking-change rule catalogue (773 kinds; 350 breaking / 19
  warnings / 404 informational) — <https://www.oasdiff.com/docs/breaking-changes>
- [S2] How oasdiff decides what is breaking (contract-based derivation,
  warn-on-unprovable, test-enforced severity) — same page as [S1].
- [S3] oasdiff CLI quick start (install: brew / install.sh / Docker /
  binaries; `breaking|changelog|diff|validate`; exit codes; formats) —
  <https://www.oasdiff.com/docs/getting-started>
- [S4] oasdiff GitHub Action (`breaking` action, `fail-on`, `exclude-elements`,
  `flatten-allof`, `err-ignore`/`warn-ignore`, `filter-extension`,
  `composed`, `.oasdiff.yaml`, `review`) —
  <https://www.oasdiff.com/docs/github-action>
- [S5] oasdiff action `fail-on` levels ERR/WARN/INFO (corroborated) —
  pkg.go.dev `oasdiff` command docs,
  <https://pkg.go.dev/github.com/tufin/oasdiff>
- [S6] oasdiff config-file reference (lookup order, precedence,
  `severity-levels`, relative-path resolution) —
  <https://github.com/oasdiff/oasdiff/blob/main/docs/CONFIG-FILES.md>
- [S7] kin-openapi README (dependents incl. oasdiff, apisprout,
  httptest-openapi; validation feature list) —
  <https://github.com/getkin/kin-openapi> (README, "Introduction" and
  alternatives/dependents sections)
- [S8] oasdiff built on kin-openapi (dependency claim via [S7]; consistent
  with `go.mod` carrying `github.com/oasdiff/yaml{,3}` indirect entries).
- [S9] Spectral installation and usage (npm/yarn/Docker; `.spectral.yaml`;
  `extends: ["spectral:oas", …]`; `spectral lint`) —
  <https://github.com/stoplightio/spectral> (README, develop branch).
- [S10] Spectral GitHub Action (`file_glob`, `spectral_ruleset`,
  root `.spectral.yaml`) —
  <https://github.com/stoplightio/spectral-action> (README, master branch).
- [S11] Spectral `oas` ruleset source (rule list, `recommended` flags,
  severity defaults/overrides, formats) —
  <https://github.com/stoplightio/spectral/blob/develop/packages/rulesets/src/oas/index.ts>
- [S12] Spectral oas3-specific rules (`oas3-valid-*-example`,
  `oas3-schema`, `oas3-server-variables`, `oas3-server-trailing-slash`,
  `oas3-unused-component`, `no-$ref-siblings`) — same file as [S11].
- [S13] Spectral OWASP ruleset existence (ecosystem add-on, out of scope) —
  Spectral docs "Spectral Rulesets",
  <https://meta.stoplight.io/docs/spectral/01baf06bdd05a-rulesets>
  (referenced from [S9] README "Custom Rulesets" page).
- [S14] Spectral `oasPathParam` function (resolved-document parameter check;
  equivalent-path detection) —
  <https://github.com/stoplightio/spectral/blob/develop/packages/rulesets/src/oas/functions/oasPathParam.ts>
- [S15] Spectral `oasUnusedComponent` function (scans schemas, responses,
  parameters, examples, requestBodies, headers, links, callbacks — not
  securitySchemes) —
  <https://github.com/stoplightio/spectral/blob/develop/packages/rulesets/src/oas/functions/oasUnusedComponent.ts>
- [S16] vacuum (Go-native OpenAPI linter, Spectral-ruleset compatible) —
  project README as mirrored on
  <https://socket.dev/npm/package/@quobix/vacuum/overview/0.17.13>
  (verbatim README excerpt; official docs site unreachable from here, so
  this stays a pilot candidate, not a committed claim).
- [S17] ADR-011 (hand-edited 3.0.3, drift gate, deferred options: oasdiff,
  vacuum/spectral, contract harness, 3.1 migration) —
  `docs/adr/011-openapi-generation.md`.
- [S18] kin-openapi request/response validation API
  (`RequestValidationInput`, `ValidateRequest`, `ResponseValidationInput`,
  `SetBodyBytes`, `ValidateResponse`, body-decoder registration) —
  kin-openapi README "Validating HTTP requests/responses",
  <https://github.com/getkin/kin-openapi>.
- [S19] kin-openapi `routers` package (`gorillamux` + stdlib-only `legacy`) —
  <https://pkg.go.dev/github.com/getkin/kin-openapi/routers>.
- [S20] OpenAPI 3.0.3 Server Object / Server Variable Object (`url`
  REQUIRED, `{variable}` substitution, MAY be relative; variable `default`
  REQUIRED) — <https://spec.openapis.org/oas/v3.0.3#server-object>.
- [S21] Drift gate reads only `paths` for method+path keys —
  `cmd/openapi-drift/main.go` (`collectSpecRoutes`, `normalizePath`).
- [S22] Docs module serves the embedded spec untouched —
  `internal/module/docs/module.go` (`go:embed openapi.yaml`,
  `/docs/openapi.yaml` handler).
