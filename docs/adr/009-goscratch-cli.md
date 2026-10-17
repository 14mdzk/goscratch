# ADR-009: goscratch CLI — Template-First Surface

**Date:** 2026-10-05
**Status:** Accepted
**Deciders:** 14mdzk (wayfinder ticket #86)

---

## Context

A1 shipped `cmd/scaffold module <name>` (ADR-008): a single-command generator
invoked as `go run ./cmd/scaffold module <name>`. The v1.3 plan (Phase 5)
graduates it into a `goscratch` CLI that encodes the canonical module pattern
across all 8 reference modules (auth, docs, health, job, role, sse, storage,
user). The plan sketched four `make:*` commands; [ticket #86] prototyped three
candidate surfaces — colon commands (plan-literal), nested subcommands
(framework-style), and template-first — and selected the template-first surface.

[ticket #86]: https://github.com/14mdzk/goscratch/issues/86

Prototype (primary source, throwaway branch):
`prototype/goscratch-cli-surface` @ `73fbc1d` — a mock terminal with all three
surfaces, guided scenarios, and the would-be file changes.

## Decision

**Template-first surface.**

```
goscratch new <template> <name> [flags]   instantiate a template
goscratch templates                       list available templates
goscratch version                         print version
goscratch help [template|command]         show help
```

**Templates (v1.3):** `module`, `migration`, `domain`, `job` — the four
generators the plan specifies.

**Flags:**

| Flag | Applies to | Meaning |
|------|------------|---------|
| `--dry-run` | all | print the files that would be created; write nothing |
| `--type=entity\|value-object` | domain | kind of domain type |
| `--module <name>` | domain | owning module; may be omitted when run from inside `internal/module/<module>/` (inferred from cwd) |

**Behavior locked with this decision:**

- Terse errors: one `error:` line plus `exit status 1`; no usage dump.
- Validation follows ADR-008: names match `^[a-z][a-z0-9_]*$`, Go reserved words
  rejected, collisions refused, no `--force` — delete first to regenerate.
- Migrations are generated directly (no dependency on the `migrate` binary):
  `<seq>_<name>.up.sql` and `.down.sql`, 6-digit zero-padded sequence continuing
  from the highest existing migration.
- Templates live in `cmd/goscratch/templates/<template>/`, embedded with
  `embed.FS`; they mirror the reference modules and co-evolve in the same PR
  when the canonical pattern changes. Import paths come from the nearest
  `go.mod` (walk up from cwd).
- `make new-module` delegates: `@go run ./cmd/goscratch new module $(name)`.
  `cmd/scaffold` is removed in the same change — no deprecation shim.
- The CLI lives at `cmd/goscratch` and installs via `go install ./cmd/goscratch`.
  `make build` keeps building only the API and worker into `bin/` (avoids
  clobbering the existing `bin/goscratch` server binary).

**Deferred (considered; deliberately not v1.3):**

- `resource` preset (module + entity + migration in one step): the entity-name
  derivation rule is fragile, and the three templates compose without it. It
  adds cleanly later as just another template.
- Interactive picker when `<template>` is omitted: bare `goscratch new` prints
  the template list and usage instead.
- `--dir` and shell completion: revisit if the surface grows.

## Rationale

Same-repo, same Go module (unchanged from the plan):

1. The generator reads `go.mod` for the module path — it must live in the repo
   it generates for.
2. Templates encode the canonical pattern and must co-evolve with the modules.
3. Single import path for runtime and tooling; users install alongside the
   project.

Template-first specifically: the surface stays flat as generators grow — a new
generator is a new template, not a new verb tree — and `goscratch templates`
makes the model discoverable without reading the CLI source. The colon and
nested variants were rejected for framing the surface around fixed verbs,
duplicating parse/validation machinery per command, and (for the framework
option) adding a dependency and a usage-dump error tone the codebase does not
otherwise use.

## Relationship to ADR-008

ADR-008's validation rules, `embed.FS` template strategy, `go.mod`-derived
module path, and `TODO(scaffold)` wiring convention carry forward unchanged.
Its CLI shape (`cmd/scaffold module <name>`, with `make:module` and friends as
the sketched extension direction) is superseded by this ADR.

## Consequences

- `goscratch new module <name>` replaces `go run ./cmd/scaffold module <name>`;
  `make new-module` keeps working via delegation.
- The CLI stays zero-dependency (stdlib dispatcher; the colon and nested
  framework options were rejected).
- ADR numbering is settled: this ADR holds 009; the queue backend decision holds
  010; the OpenAPI generation ADR is renumbered to 011.
