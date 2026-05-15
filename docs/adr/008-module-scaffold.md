# ADR-008: Module Scaffold Generator

**Date:** 2026-05-15
**Status:** Accepted
**Deciders:** Lead, implementer-scaffold

---

## Context

Modules added after the initial `user` and `auth` baseline (role, storage, job) drifted
from the canonical layout — private repo interfaces, port files, and constructor shapes
diverged in small but compounding ways (v1.1 PR-06 "Pattern alignment"). The only
reliable fix is to eliminate manual copy-paste at write-time via a generator.

## Decision

Ship a code-generation CLI (`cmd/scaffold`) as part of the repository. The `module`
subcommand creates `internal/module/<name>/{domain,usecase,handler}/` from embedded
templates that mirror the `user` and `auth` modules exactly. Key design choices:

1. **Single binary, subcommand extensible.** Invoked as `go run ./cmd/scaffold module <name>`.
   The `migration` subcommand (wave 2, row A2) slots into the same binary with no refactor.
2. **`embed.FS` + `text/template`.** Templates live in `cmd/scaffold/templates/module/`
   and are embedded at compile time. The binary is fully self-contained; no file-system
   access to templates is needed at runtime.
3. **`go.mod`-derived module path.** `{{ .ModulePath }}` is read by walking up from `cwd`
   to the nearest `go.mod`. Import paths stay correct on forks or renames without manual
   template edits.
4. **Fail-fast validation.** Names must match `^[a-z][a-z0-9_]*$`, must not be a Go
   reserved word, and must not collide with an existing directory. Idempotency is strict:
   no `--force` flag; deletion is the operator's call.
5. **`TODO(scaffold)` comments, not auto-wiring.** Generated files include explicit
   `TODO(scaffold)` instructions describing what must be wired in
   `internal/platform/app/app.go`. Auto-editing `app.go` is deferred to a future wave.

## Canonical layout enforced by the generator

```
internal/module/<name>/
  domain/<name>_domain.go      Entity struct
  usecase/port.go              UseCase interface (handler depends on this)
  usecase/<name>_usecase.go    Concrete implementation
  usecase/<name>_usecase_test.go  Table-driven smoke tests
  handler/<name>_handler.go    Fiber handler (depends on usecase.UseCase)
  handler/<name>_handler_test.go  Table-driven smoke tests
```

Source of truth: `internal/module/user/` and `internal/module/auth/`.

## Consequences

- New modules start from a known-good shape; pattern drift is a generator bug, not a
  per-developer decision.
- Operators must still wire the module into `app.go` by hand (intentional; auto-editing
  lifecycle code is out of scope for A1 and a footgun risk).
- The `migration` subcommand (A2) reuses the same binary and embed pattern.
