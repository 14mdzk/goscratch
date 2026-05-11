# cron-dispatch

A minimal Alpine + busybox-crond container that periodically POSTs to
`/api/jobs/dispatch` on the goscratch API so that periodic background jobs
(currently only `audit.cleanup`) actually run.

This is the **production pattern** for v1.2. The API does **not** ship an
in-process scheduler — operators are expected to schedule periodic dispatches
externally. This image is one such option; any cron-like dispatcher (k8s
`CronJob`, GitHub Actions schedule, Airflow, Temporal, etc.) is equally valid.
See the *Alternatives* section below.

## Why a separate container?

- **Crash isolation**: a misbehaving scheduler cannot wedge the API process.
- **Independent rollout**: the schedule changes without restarting the API.
- **Least privilege**: the dispatch container only needs an admin JWT and
  outbound HTTPS — no DB or Redis access.
- **No code surface in the API**: no new config knob, no new lifecycle hook,
  no new package to maintain. Matches the "no abstraction without a 2nd
  consumer" guard documented in `docs/audit/v1.2-plan.md`.

## Build

```bash
docker build -t goscratch-cron-dispatch:0.1.0 deploy/docker/cron-dispatch
```

## Run (local smoke test)

```bash
docker run --rm \
    -e GOSCRATCH_API_BASE_URL=https://api.example.com \
    -e GOSCRATCH_API_TOKEN="$ADMIN_JWT" \
    -e GOSCRATCH_AUDIT_RETENTION_DAYS=90 \
    -e GOSCRATCH_AUDIT_CRON_SCHEDULE="*/5 * * * *" \
    goscratch-cron-dispatch:0.1.0
```

Set the cron expression to `*/5 * * * *` while validating, then revert to the
production default `0 3 * * *` (daily 03:00 UTC).

## Required Environment

| Variable                          | Required | Default       | Notes                                                                                                  |
|-----------------------------------|----------|---------------|--------------------------------------------------------------------------------------------------------|
| `GOSCRATCH_API_BASE_URL`          | yes      | —             | No trailing slash. Path `/api/jobs/dispatch` is appended.                                              |
| `GOSCRATCH_API_TOKEN`             | yes      | —             | Long-lived admin-scoped JWT. See *Token Management* below.                                             |
| `GOSCRATCH_AUDIT_RETENTION_DAYS`  | no       | `90`          | Forwarded into the job payload as `retention_days`. Non-positive values fall back to `90`.             |
| `GOSCRATCH_AUDIT_CRON_SCHEDULE`   | no       | `0 3 * * *`   | Standard 5-field busybox cron expression. Rendered into `/etc/crontabs/root` at container start.       |

## Payload

The container POSTs the following body to `/api/jobs/dispatch`:

```json
{
    "type": "audit.cleanup",
    "payload": {
        "retention_days": 90
    },
    "max_retry": 3
}
```

The endpoint requires:

- A valid JWT in the `Authorization: Bearer` header.
- The token's subject must hold the `admin` role.

A successful dispatch returns `HTTP 201` with the created `Job` envelope.

## Token Management

`GOSCRATCH_API_TOKEN` is sensitive. Do not commit it. Recommended pattern:

1. Mint a long-lived JWT for a dedicated service-account user with only the
   `admin` role. Keep the TTL short enough that compromise has a bounded
   blast radius (e.g., 24 h) and refresh from a secret manager.
2. Inject via Docker secret, Kubernetes secret, or `--env-file`. Never via
   a `-e GOSCRATCH_API_TOKEN=...` in a versioned compose file.
3. Rotate on the same cadence as other admin credentials. The image takes
   the token from the environment on every cron tick, so a restart picks up
   a rotated value with no rebuild.

## Operational Runbook

| Symptom                                                  | Likely cause                                                | Fix                                                                                          |
|----------------------------------------------------------|-------------------------------------------------------------|----------------------------------------------------------------------------------------------|
| Container logs `HTTP 401`                                | Token expired or stripped of `admin` role                   | Rotate `GOSCRATCH_API_TOKEN`; restart container.                                             |
| Container logs `HTTP 403`                                | Token user lost admin role                                  | Re-grant the role, then rotate the token.                                                    |
| Container logs `HTTP 429`                                | Global API rate limit hit (rare for a single daily POST)    | Stagger the cron schedule away from peak hours.                                              |
| Container logs `HTTP 503`                                | API health-gated cache backend                              | Investigate the API's `/healthz/ready` first.                                                |
| `audit_logs` row count grows unbounded                   | Dispatch container not running or token broken              | `docker logs` / `kubectl logs` the dispatch pod. Verify cron fired.                          |
| Operator wants to run cleanup *now*                      | —                                                           | `docker exec <container> /usr/local/bin/dispatch.sh` (does not wait for the cron tick).      |

## Manual Re-Dispatch

To run the cleanup outside the cron schedule (incident response, backfill,
testing):

```bash
docker exec <container-id> /usr/local/bin/dispatch.sh
```

Or from outside the container, using `curl` directly:

```bash
curl -X POST "$GOSCRATCH_API_BASE_URL/api/jobs/dispatch" \
    -H "Authorization: Bearer $GOSCRATCH_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"type":"audit.cleanup","payload":{"retention_days":90},"max_retry":3}'
```

## Alternatives

Any of the following replace this image cleanly — the contract is "something
POSTs an `audit.cleanup` dispatch on a schedule":

- **Kubernetes `CronJob`** — preferred when already running on k8s; no
  container image needed beyond a stock `curlimages/curl` plus an inline
  command. The token comes from a `Secret` mounted as env.
- **GitHub Actions schedule** — viable for low-stakes environments; not
  recommended for production because the schedule runner is outside the
  operator's blast radius.
- **systemd timers** — viable on bare metal / VM deployments.
- **Temporal / Airflow** — overkill unless other periodic workflows exist.

The "no abstraction without a 2nd consumer" guard in the audit plan rejects
adding an in-process scheduler today. If a second periodic job lands, revisit
whether to ship `scheduler.Tick(jobType, interval)` inside the API and
deprecate this container.

## Out of Scope

- Backfill of historical audit logs that pre-date the retention boundary —
  delete those manually if needed.
- Per-tenant retention policies — `retention_days` is global today.
- Authentication via mTLS — JWT is the auth model per `docs/VISION.md`.
