# PR-18: Audit-log Retention Scheduler (variant A — external cron container)

| Field | Value |
|-------|-------|
| Branch | `feat/audit-retention-scheduler` |
| Status | in review |
| Audit source | `internal/worker/handlers/audit_cleanup_handler.go` consumes `audit.cleanup` jobs but nothing dispatches them; operators currently must call `POST /jobs/dispatch` manually |
| Closes | v1.2 punch-list row #18 |

## Decision: Variant A

`docs/audit/v1.2-plan.md` proposed two variants:

- **A** — Document the external-cron pattern in the RUNBOOK and ship a sample container under `deploy/docker/cron-dispatch/`. Zero Go code change.
- **B** — Add an in-process scheduler tick to the worker (`scheduler.Tick(jobType, interval)`).

This PR ships **variant A**, per the plan's stated default. Rationale:

1. **No abstraction without a second consumer.** The only periodic job today is `audit.cleanup`. An in-process scheduler would be a one-consumer abstraction.
2. **Crash isolation.** A misbehaving scheduler cannot wedge the API process.
3. **Independent rollout.** Schedule changes ship without an API restart.
4. **Least privilege.** The dispatch container only needs an admin JWT and outbound HTTPS — no DB or Redis access.

If a second periodic job lands later, variant B can be revisited as a follow-up.

## Tasks

- [x] Add `deploy/docker/cron-dispatch/Dockerfile` — Alpine + busybox-crond + curl + jq + tini.
- [x] Add `deploy/docker/cron-dispatch/dispatch.sh` — POSTs the audit-cleanup job dispatch to the API. Exits non-zero on HTTP error.
- [x] Add `deploy/docker/cron-dispatch/entrypoint.sh` — renders the crontab from `GOSCRATCH_AUDIT_CRON_SCHEDULE` and execs crond.
- [x] Add `deploy/docker/cron-dispatch/crontab.tpl` — template consumed by entrypoint.sh.
- [x] Add `deploy/docker/cron-dispatch/README.md` — operator-facing documentation: build, run, env vars, payload shape, token management, alternatives, manual re-dispatch.
- [x] Add `deploy/docker/cron-dispatch/docker-compose.example.yml` — copy-paste-ready service definition.
- [x] `CHANGELOG.md` `[Unreleased]` entry under "Added" and the operator-upgrade note.
- [x] Update `docs/audit/v1.2-punch-list.md` row #18 status → `in review`.

## Acceptance Criteria

- Container image builds cleanly (`docker build deploy/docker/cron-dispatch`).
- `dispatch.sh` exits 0 on `HTTP 2xx`, non-zero on any other response or missing env.
- No Go code touched. `make lint test` remains green.
- README documents token management, payload shape, alternatives, and manual re-dispatch.

## Out of Scope

- In-process scheduler (variant B). Deferred until a second periodic job exists.
- Helm chart / k8s `CronJob` manifests. Operators on k8s can wire the same pattern using the upstream `curlimages/curl` image and a `CronJob`; documented in README "Alternatives" section.
- Per-tenant `retention_days`. Global today.
- Token rotation tooling — operator's secret manager owns this.
- Re-architecting `audit_cleanup_handler` to take retention from config instead of payload.

## Operator Upgrade Note

Before this PR, operators of any installation that wanted automated audit retention had to wire their own cron. After this PR, operators **still** wire their own cron — but the sample container removes the friction. No behavioural change inside the API binary.

If an operator already has external cron pointed at `/api/jobs/dispatch`, no action is required. The new sample container is opt-in.
