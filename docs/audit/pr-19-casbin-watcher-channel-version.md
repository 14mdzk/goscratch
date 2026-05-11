# PR-19: Casbin Watcher Channel Versioning

| Field | Value |
|-------|-------|
| Branch | `refactor/casbin-watcher-channel-versioning` |
| Status | in review |
| Audit source | `internal/adapter/casbin/watcher_redis.go:12` hard-codes the channel name `casbin:policy:update`. The pub/sub message JSON shape (`{op,sec,ptype,params}`) is implicit; a future protocol change cross-talks with old instances during a rolling deploy. |
| Closes | v1.2 punch-list row #19 |

## Goal

Bump the default Casbin watcher channel constant from `casbin:policy:update` to `casbin:policy:update:v1` so that any future change to the JSON message shape can be shipped behind a `:v2` bump and old/new instances stop hearing each other on the same channel. Eliminates silent policy-state divergence during rolling deploys.

## Tasks

- [x] Change `defaultRedisChannel` in `internal/adapter/casbin/watcher_redis.go` from `"casbin:policy:update"` to `"casbin:policy:update:v1"`.
- [x] Update the doc comment on `NewRedisWatcher` to reflect the new default.
- [x] Add a unit test asserting `defaultRedisChannel == "casbin:policy:update:v1"` so future protocol changes are forced to bump the suffix consciously.
- [x] `CHANGELOG.md` `[Unreleased]` entry under "Changed" with the rolling-deploy rationale and an operator upgrade note.
- [x] Update `docs/audit/v1.2-punch-list.md` row #19 status → `in review`.

## Acceptance Criteria

- `defaultRedisChannel` is exactly the string `casbin:policy:update:v1`.
- New unit test fails if the constant is ever changed without also bumping the version suffix.
- Existing pub/sub integration tests in `internal/adapter/casbin/casbin_test.go` continue to pass (they read the constant symbol, not the literal).
- `make lint test` green.
- PR body includes the operator upgrade note (acceptance rule 5 — security-affecting).

## Out of Scope

- Embedding a version field inside the JSON envelope. Channel-name versioning is the smaller, less-invasive isolation primitive; envelope schema versioning is deferred until a protocol consumer actually evolves.
- Refactoring `Config.WatcherChannel` resolution path or the empty-string default branch.
- Documenting the message envelope in OpenAPI — pub/sub is not an HTTP surface.
- Coordinated rollout tooling (Helm hooks, deploy gates). The back-stop `ReloadInterval` full reload already covers the cross-version blind window.
- Renaming the existing pub/sub topic in any consumer outside `internal/adapter/casbin/...`.

## Operator Upgrade Note

`RedisWatcher` is not wired in the current bootstrap (`internal/platform/app/app.go` constructs `casbinadapter.Adapter` without setting `Config.Watcher`), so this change has no runtime effect on the shipping binary today. The bump is preventive: it forces any future wiring PR to adopt a versioned channel name from day one rather than retrofitting one later.

If and when `RedisWatcher` is wired in production, deploying *that* version will cross a channel-name boundary from `casbin:policy:update` (any earlier release that wired the watcher with the previous default) to `casbin:policy:update:v1`. During such a rolling-deploy window:

- Live incremental policy updates published by new pods will not reach old pods, and vice versa.
- Both sides still converge to the database state within `Authorizer.ReloadInterval` (back-stop full reload, default 5 min).

No operator action is required for this PR.

## Notes for Implementer

- Do not push or open the PR. Lead reviews diff in worktree before shipping.
- Single-file source change. Test goes in a new `internal/adapter/casbin/watcher_redis_test.go` or extends the existing `casbin_test.go` — implementer's choice; pick whichever keeps the assertion small and isolated.
- If any other constant or call-site outside `internal/adapter/casbin/` references the literal string `casbin:policy:update`, stop and report — that is a wider blast radius than this PR's scope.
