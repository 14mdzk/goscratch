# PR-17: `docs/RUNBOOK.md`

| Field | Value |
|-------|-------|
| Branch | `docs/operator-runbook` |
| Status | in review |
| Audit source | No incident playbook for the security-critical operations introduced in v1.1 / v1.2 |
| Closes | v1.2 punch-list row #17 |

## Goal

Ship `docs/RUNBOOK.md` covering the eight operational scenarios enumerated in `docs/audit/v1.2-plan.md` §PR-17, each with trigger / pre-flight / commands / verify / rollback structure.

## Tasks

- [x] Add `docs/RUNBOOK.md` with sections §1–§8 covering: rotate `JWT_SECRET`, mass refresh-token revoke (single user + full sweep, API + direct Redis), Casbin policy reload, audit-log retention re-run, fast user disable, cache flush, reading audit logs (SQL recipes), reading metrics during incident (PromQL recipes + health-probe interpretation).
- [x] Cross-reference v1.1 + v1.2 PRs (PR-03 dual-key shape, PR-09 rate-limit headers, PR-10 authz cache, PR-13 readiness, PR-18 cron-dispatch, PR-19 versioned channel).
- [x] Link `docs/RUNBOOK.md` from `README.md` "Documentation" section.
- [x] `CHANGELOG.md` `[Unreleased]` entry under "Documentation".
- [x] Update `docs/audit/v1.2-punch-list.md` row #17 status → `in review` (no longer "blocked by 18, 19" since both are shipped).
- [x] Add an "incident-response checklist" appendix.

## Acceptance Criteria

- Every section: trigger condition, pre-flight, commands, verification, rollback.
- Commands are copy-paste with named placeholders (`$API`, `$REDIS`, `$DB`, `$ADMIN_TOKEN`).
- Redis sweeps use `SCAN`, never `KEYS`.
- Section ordering matches v1.2-plan.md §PR-17.
- No code change. `make lint test` remains green.

## Out of Scope

- Disaster-recovery playbooks (DB restore from backup, region failover) — separate doc, separate PR.
- Specific secret-manager wiring (AWS / GCP / Vault) — the RUNBOOK is implementation-agnostic.
- SRE alerting rules / Grafana dashboards — referenced indirectly via PromQL examples; full dashboard JSON is a separate concern.
- Operator-facing CLI tooling.

## Notes

`docs/RUNBOOK.md` references PR-19's versioned channel and PR-18's cron-dispatch container, both of which are in-review in the same v1.2 batch. If the operator deploys this RUNBOOK against a binary that does not yet wire `RedisWatcher`, the §3 "Force a propagation via the watcher channel" subsection is harmless — they will simply rely on the back-stop reload tick documented in the same section.
