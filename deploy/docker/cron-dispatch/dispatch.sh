#!/bin/sh
# dispatch.sh — POSTs an audit-cleanup job dispatch to the goscratch API.
#
# Required environment:
#   GOSCRATCH_API_BASE_URL          e.g. https://api.example.com (no trailing slash)
#   GOSCRATCH_API_TOKEN             admin-scoped JWT
#   GOSCRATCH_AUDIT_RETENTION_DAYS  integer; defaults to 90 if unset or non-positive
#
# Exit codes:
#   0  dispatched successfully (HTTP 201)
#   1  configuration error (missing env)
#   2  API rejected the request (non-2xx)
set -eu

if [ -z "${GOSCRATCH_API_BASE_URL:-}" ]; then
    echo "dispatch.sh: GOSCRATCH_API_BASE_URL is required" >&2
    exit 1
fi
if [ -z "${GOSCRATCH_API_TOKEN:-}" ]; then
    echo "dispatch.sh: GOSCRATCH_API_TOKEN is required" >&2
    exit 1
fi

retention="${GOSCRATCH_AUDIT_RETENTION_DAYS:-90}"
case "$retention" in
    ''|*[!0-9]*) retention=90 ;;
esac
if [ "$retention" -le 0 ]; then
    retention=90
fi

body=$(jq -n --argjson days "$retention" '{
    type: "audit.cleanup",
    payload: { retention_days: $days },
    max_retry: 3
}')

# /jobs/dispatch is admin-only. The token is sent as a bearer header.
# --fail-with-body causes curl to exit non-zero on HTTP errors while still
# echoing the response body so operators can debug.
http_code=$(curl -sS \
    -o /tmp/dispatch-response.json \
    -w "%{http_code}" \
    -X POST "${GOSCRATCH_API_BASE_URL%/}/api/jobs/dispatch" \
    -H "Authorization: Bearer ${GOSCRATCH_API_TOKEN}" \
    -H "Content-Type: application/json" \
    --data "$body" \
    --max-time 30)

if [ "$http_code" -lt 200 ] || [ "$http_code" -ge 300 ]; then
    echo "dispatch.sh: HTTP $http_code from /jobs/dispatch" >&2
    cat /tmp/dispatch-response.json >&2 || true
    echo >&2
    exit 2
fi

job_id=$(jq -r '.data.id // .id // "<unknown>"' /tmp/dispatch-response.json 2>/dev/null || echo "<unknown>")
ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "$ts dispatch.sh: audit.cleanup dispatched retention_days=$retention job_id=$job_id"
