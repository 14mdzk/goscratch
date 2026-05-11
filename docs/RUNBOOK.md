# Operator Runbook

Incident-response playbooks for the security-critical operations introduced in v1.1 and v1.2. Each section follows the same shape:

1. **Trigger** — when to run this.
2. **Pre-flight** — sanity checks before mutating state.
3. **Commands** — copy-paste with `$VARS` named for clarity.
4. **Verify** — how to confirm the operation took effect.
5. **Rollback** — undo path, or "not reversible — communicate accordingly".

Conventions:
- `$API` is the base URL of the goscratch API, e.g. `https://api.example.com`.
- `$ADMIN_TOKEN` is an admin-scoped JWT. Mint via the login flow for a service-account user holding the `admin` role.
- `$REDIS` is the Redis URL the API uses, e.g. `redis://redis:6379/0`. `redis-cli -u "$REDIS"` works against the same database the API reads.
- `$DB` is the Postgres URL, e.g. `postgres://goscratch:...@postgres:5432/goscratch`.
- All write operations should be logged in the incident channel before execution. Audit logs land in the `audit_logs` table — see §7.

> **Read this first.** Sections §1, §2, and §5 are destructive at scale. Do a small-blast-radius rehearsal (one user, one IP, one channel) before running the full sweep.

---

## §1. Rotate `JWT_SECRET`

### Trigger

- The current `JWT_SECRET` has been leaked, suspected leaked, or shared off-channel.
- A scheduled rotation per the operator's key-management policy.

### Pre-flight

- Generate a new 32+ byte secret (`openssl rand -base64 48 | head -c 64`).
- Update the secret store (Kubernetes `Secret`, AWS Secrets Manager, etc.). Do **not** commit it.
- Notify users via the established channel: "all sessions will be terminated; please re-login".

### Commands

```bash
# 1. Update the secret in your secret store, then roll the deployment.
#    Kubernetes example:
kubectl set env deployment/goscratch-api JWT_SECRET="$NEW_SECRET"  # via SealedSecret / external-secrets

# 2. Watch the rollout. Pods refuse to start if JWT_SECRET is missing,
#    equals the committed placeholder, or is shorter than 32 bytes
#    (see internal/platform/config/config.go:288-294).
kubectl rollout status deployment/goscratch-api --timeout=5m
```

### Verify

```bash
# Every previously-issued access token is now invalid; an authenticated
# request with an old token returns 401.
curl -s -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer $OLD_TOKEN" \
    "$API/api/users/me"
# Expect: 401

# A fresh login with the new secret works end to end.
curl -s "$API/api/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"alice@example.com","password":"..."}' \
    | jq -r .access_token
```

Old refresh tokens **also** stop working because every refresh re-signs an access token using the current secret. If you also want to invalidate refresh-token storage explicitly, follow §2.

### Rollback

- Re-deploy with the old `JWT_SECRET` value (kept in the secret store's previous version).
- Existing access tokens minted under the old secret will validate again until they expire normally.

### Communication template

> Subject: Security maintenance — please log in again
>
> We rotated our authentication signing key as part of a scheduled security operation. Active sessions have been ended. Please log in again. No action is required for stored data — passwords and account state are unchanged.

---

## §2. Mass refresh-token revoke

### Trigger

- A specific user's account is compromised (single-user revoke).
- A broader incident requires terminating all active refresh tokens (full sweep).
- Forensic policy after a JWT-secret rotation (belt-and-braces — §1 already invalidates them implicitly, but explicit Redis cleanup leaves no residual state).

### Pre-flight

- Identify the scope:
    - Single user: `$USER_ID` (UUID).
    - All users: confirm with incident commander; this terminates every session.
- Check active token count:
    ```bash
    redis-cli -u "$REDIS" --scan --pattern 'refresh:user:*' | wc -l
    ```

### Commands

**Single user — use the public endpoint.** The password-change path already calls `RevokeAllForUser` (see `internal/module/user/usecase/user_usecase.go:194`).

```bash
# Option A: change the user's password — same revoke flow, also locks out
# whoever holds the old credentials. Recommended for compromise response.
curl -X POST "$API/api/users/$USER_ID/password" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"new_password":"<temporary, communicated out-of-band>"}'
```

**Single user — direct Redis sweep (when the API is unreachable).** The key shape is documented in `internal/module/auth/usecase/auth_usecase.go:63-75`.

```bash
# Two key families guard one refresh token (PR-03 dual-key shape):
#   refresh:tok:<sha256-hex(token)>          → userID  (lookup)
#   refresh:user:<userID>:<sha256-hex(token)> → "1"     (revocation index)
# Deleting only the index leaves the lookup orphaned — delete both.

# 1. Find every token-hash for this user.
hashes=$(redis-cli -u "$REDIS" --scan --pattern "refresh:user:$USER_ID:*" \
    | awk -F: '{print $NF}')

# 2. Delete both keys per token.
for h in $hashes; do
    redis-cli -u "$REDIS" DEL "refresh:tok:$h" "refresh:user:$USER_ID:$h"
done
```

**All users — Redis sweep.** Always use `SCAN`, never `KEYS`.

```bash
redis-cli -u "$REDIS" --scan --pattern 'refresh:tok:*'        | xargs -r redis-cli -u "$REDIS" DEL
redis-cli -u "$REDIS" --scan --pattern 'refresh:user:*:*'     | xargs -r redis-cli -u "$REDIS" DEL
```

### Verify

```bash
# Attempting to refresh with a revoked token returns 401.
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$API/api/auth/refresh" \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$REVOKED_REFRESH\"}"
# Expect: 401

# Confirm Redis is clean for that scope.
redis-cli -u "$REDIS" --scan --pattern "refresh:user:$USER_ID:*" | wc -l
# Expect: 0
```

### Rollback

**Not reversible.** Revoked refresh tokens are gone. Affected users must re-login. Communicate accordingly.

---

## §3. Reload Casbin policies on demand

### Trigger

- An out-of-band policy edit was applied directly in the `casbin_rule` table (forbidden by the v1.1 lint guard but possible during incident recovery).
- A pod's in-memory policy state is suspected to be stale.

### Pre-flight

- Confirm whether the watcher is wired (`RedisWatcher` is *not* wired in the current bootstrap as of v1.2 PR-19; the back-stop reload tick is the only propagation mechanism today).
- Check the back-stop interval (`Authorizer.ReloadInterval`, default 5 minutes).

### Commands

**Wait for the back-stop (recommended).** Every API instance runs a periodic `LoadPolicy` from Postgres. If you can wait 5 minutes, do nothing.

**Force a propagation via the watcher channel** (only relevant if `RedisWatcher` is wired in your build):

```bash
# Publish a full-reload signal on the versioned channel.
redis-cli -u "$REDIS" PUBLISH casbin:policy:update:v1 \
    '{"op":"reload","sec":"","ptype":"","params":null}'
```

> **Channel name version.** PR-19 (v1.2) bumped the default to `casbin:policy:update:v1`. If you have any pods still running a pre-v1.2 release that wired the watcher with the old default, also publish to `casbin:policy:update` until those pods are rolled.

**Force a propagation by restarting pods.** Cheapest when watchers are not wired:

```bash
kubectl rollout restart deployment/goscratch-api
```

### Verify

```bash
# Re-check authz for a known-affected user/resource/action.
curl -s "$API/api/some/protected/endpoint" \
    -H "Authorization: Bearer $TEST_TOKEN" \
    -o /dev/null -w "%{http_code}\n"
# Expect the new policy outcome (200 / 403 as applicable).
```

### Rollback

If the reload re-loaded a broken policy, revert by:

1. Restoring the prior `casbin_rule` rows from your DB backup or undo script.
2. Re-publishing the reload signal (or waiting for the back-stop) so all pods pick up the restored state.

---

## §4. Audit-log retention re-run

### Trigger

- The scheduled `cron-dispatch` container (see `deploy/docker/cron-dispatch/README.md`) is down and `audit_logs` is growing past policy.
- An ad-hoc retention pass with a non-default `retention_days` is required (e.g., legal hold release).

### Pre-flight

- Identify the cutoff. Default `retention_days=90` deletes everything older than 90 days.
- Confirm no legal hold prohibits deletion.

### Commands

```bash
curl -X POST "$API/api/jobs/dispatch" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"type":"audit.cleanup","payload":{"retention_days":90},"max_retry":3}'
```

The worker `AuditCleanupHandler` (`internal/worker/handlers/audit_cleanup_handler.go`) deletes in batches of 1000 to bound memory.

### Verify

```bash
# Row count before/after.
psql "$DB" -c 'SELECT count(*) FROM audit_logs;'

# Oldest surviving row is younger than the cutoff.
psql "$DB" -c 'SELECT min(created_at) FROM audit_logs;'
```

The handler also logs `Audit log cleanup completed rows_deleted=...` to stdout.

### Rollback

**Not reversible.** Deleted audit rows are gone. Restore from the DB backup if absolutely required, but coordinate with whoever owns the backup policy — partial restore of one table is non-trivial.

---

## §5. Disable a compromised user fast

### Trigger

- Account takeover suspected or confirmed.
- Insider-threat response.

### Pre-flight

- Capture the userID, current session tokens (for forensic record), and the IP / user-agent from the most recent `audit_logs` rows for the user (see §7).

### Commands

```bash
# 1. Deactivate the user. The handler also calls RevokeAllForUser.
curl -X PATCH "$API/api/users/$USER_ID/deactivate" \
    -H "Authorization: Bearer $ADMIN_TOKEN"

# 2. Force-revoke refresh tokens explicitly (belt-and-braces — the deactivate
#    path already does this; this guards against bugs in the integration).
curl -X POST "$API/api/users/$USER_ID/password" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"new_password":"<random>"}'
```

### Verify

```bash
# Authenticated requests with the user's previous access token return 401.
curl -s -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer $LEAKED_ACCESS" \
    "$API/api/users/me"
# Expect: 401

# Refresh attempts return 401.
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$API/api/auth/refresh" \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$LEAKED_REFRESH\"}"
# Expect: 401

# No active refresh-token keys remain for the user.
redis-cli -u "$REDIS" --scan --pattern "refresh:user:$USER_ID:*" | wc -l
# Expect: 0
```

### Rollback

```bash
curl -X PATCH "$API/api/users/$USER_ID/activate" \
    -H "Authorization: Bearer $ADMIN_TOKEN"
```

The user must re-login (no stored sessions survived).

---

## §6. Cache flush during incident

### Trigger

- Authz decisions look wrong and §3 (policy reload) did not fix it — suspected stale entries in the authz decision cache (`subject:obj:act` keys).
- Rate-limit lockout for legitimate users after a config change.

### Pre-flight

- **Do not** `FLUSHALL`. The Redis instance backs refresh tokens (§2), authz cache, and rate-limit counters. Flushing everything terminates every session.
- Identify the namespace to flush.

### Commands

```bash
# Authz decision cache (PR-10): key shape "authz:cache:<sub>:<obj>:<act>"
redis-cli -u "$REDIS" --scan --pattern 'authz:cache:*' \
    | xargs -r redis-cli -u "$REDIS" DEL

# Rate-limit counters (PR-09): key shape "ratelimit:<bucket>:<id>"
redis-cli -u "$REDIS" --scan --pattern 'ratelimit:*' \
    | xargs -r redis-cli -u "$REDIS" DEL
```

> Both caches refill on demand; flushing causes a transient latency bump while authz lookups re-decide from policy.

### Verify

```bash
redis-cli -u "$REDIS" --scan --pattern 'authz:cache:*' | wc -l   # Expect: 0
```

### Rollback

Not applicable — caches refill themselves. The only "rollback" is to roll back whatever change you were trying to fix in the first place.

---

## §7. Reading audit logs

### Trigger

- Forensic triage during an incident.
- Compliance request ("show me all writes by user X between dates").

### Schema

`audit_logs` columns of interest:

| Column          | Notes                                                                 |
|-----------------|-----------------------------------------------------------------------|
| `id`            | UUID                                                                  |
| `actor_id`      | userID or `null` for unauthenticated paths                            |
| `actor_email`   | denormalised at write time                                            |
| `action`        | e.g. `LOGIN`, `LOGIN_FAILED`, `PASSWORD_CHANGED`, `USER_DEACTIVATED`  |
| `resource`      | e.g. `user:<id>`, `role:<name>`                                       |
| `outcome`       | `success` / `failure`                                                 |
| `ip`            | client IP after trusted-proxy header processing (PR-09)               |
| `user_agent`    | raw                                                                   |
| `created_at`    | `timestamptz`                                                         |

### Common queries

```sql
-- All failed logins for a user in the last 24 hours.
SELECT created_at, ip, user_agent
FROM audit_logs
WHERE action = 'LOGIN_FAILED'
  AND actor_email = 'alice@example.com'
  AND created_at > now() - interval '24 hours'
ORDER BY created_at DESC;

-- All writes by user Y in a window.
SELECT created_at, action, resource, outcome
FROM audit_logs
WHERE actor_id = '<userID>'
  AND action ~ '^(CREATE|UPDATE|DELETE|PASSWORD_CHANGED)'
  AND created_at BETWEEN '2026-05-01' AND '2026-05-10'
ORDER BY created_at;

-- All LOGIN failures from a single IP.
SELECT created_at, actor_email, user_agent
FROM audit_logs
WHERE action = 'LOGIN_FAILED'
  AND ip = '203.0.113.42'
ORDER BY created_at DESC
LIMIT 200;
```

### Verify

- Cross-check against the API access log if you have one (e.g. ingress / load-balancer logs) — IP and timestamp should match within seconds.
- `LOGIN_FAILED` includes the attempted email; `actor_id` is `null` because authentication never succeeded.

### Rollback

N/A — `SELECT` only. Do not `DELETE FROM audit_logs` outside §4.

---

## §8. Reading metrics during incident

### Trigger

- Latency spike, error-rate spike, or capacity question during an incident.

### Endpoints

- `GET /healthz/live` — liveness, no dependencies probed (PR-13).
- `GET /healthz/ready` — readiness, parallel sub-checks for Postgres, Redis, RabbitMQ, Casbin; 503 if any fails.
- `GET /metrics` — Prometheus, **locked down**. Only reachable from the bind addresses configured in `metrics.allowed_cidrs` (PR-02 hardening).

### Common queries (PromQL)

```promql
# Top 10 endpoints by p99 latency over the last 5 minutes.
topk(10, histogram_quantile(0.99,
    sum by (le, route) (
        rate(http_server_request_duration_seconds_bucket[5m])
    )
))

# Rate-limit rejections per minute by endpoint (PR-09 emits 429 with
# `code=RATE_LIMIT_EXCEEDED` and `code=RATE_LIMIT_ERROR`).
sum by (route) (rate(http_server_responses_total{status="429"}[1m]))

# Queue depth (RabbitMQ).
rabbitmq_queue_messages_ready{queue=~"jobs|audit.cleanup"}

# Casbin reload counter (PR-03b). Bumps every back-stop tick and every
# watcher-driven reload. A flat curve means no reloads ran — investigate
# if a policy change isn't taking effect.
sum(rate(casbin_policy_reload_total[15m]))
```

### Health-probe interpretation

`GET /healthz/ready` returns a JSON body whose `checks` array enumerates each sub-check:

```json
{
    "status": "degraded",
    "checks": [
        {"name": "postgres", "status": "ok"},
        {"name": "redis", "status": "ok"},
        {"name": "queue", "status": "fail", "reason": "connection unavailable"},
        {"name": "authz", "status": "ok"}
    ]
}
```

- `queue(noop)` means the operator disabled RabbitMQ; this is expected on minimal deployments.
- `authz(noop)` same.
- All `fail` reasons are sanitised — they do not leak internal addresses or credentials.

### Verify

```bash
curl -s "$API/healthz/ready" | jq
```

If `status` is `degraded` for more than `HEALTH_READINESS_TIMEOUT_SEC` * 2 (default 2 s × 2 = 4 s) across consecutive checks, page the on-call.

### Rollback

N/A — read-only.

---

## Appendix — incident-response checklist

1. **Acknowledge** in the incident channel. Capture timestamp and the on-call paged.
2. **Triage** with §7 (audit logs) and §8 (metrics). Determine blast radius.
3. **Stabilise** with the smallest-blast-radius command first (§5 single user before §2 full sweep; §3 wait-for-backstop before forced reload).
4. **Document** every command run, with exact UTC timestamps, in the incident channel.
5. **Communicate** affected users (template in §1).
6. **Post-incident** — schedule a 24-hour review. Update this RUNBOOK with anything that did not work as documented.
