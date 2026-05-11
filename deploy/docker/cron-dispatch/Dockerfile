# syntax=docker/dockerfile:1.7
#
# cron-dispatch — minimal scheduled-job dispatcher for the goscratch API.
#
# This container is the **production pattern** for ticking the audit-log
# retention cleanup (and any future periodic job). It runs busybox crond,
# loads JWT credentials at start-up, and POSTs to /jobs/dispatch on the
# configured API host on a cron schedule.
#
# Operator pulls this image (or builds it from this Dockerfile) and sets:
#
#   GOSCRATCH_API_BASE_URL          e.g. https://api.example.com
#   GOSCRATCH_API_TOKEN             admin-scoped JWT
#   GOSCRATCH_AUDIT_RETENTION_DAYS  integer, default 90
#   GOSCRATCH_AUDIT_CRON_SCHEDULE   cron expression, default "0 3 * * *" (daily 03:00 UTC)
#
# No code is built — this is pure tooling. The image is intentionally
# tiny so that operators can audit it end-to-end in seconds.
FROM alpine:3.23

RUN apk add --no-cache curl tini jq ca-certificates \
    && update-ca-certificates

# dispatch.sh is the actual cron payload — it composes the JSON body and
# POSTs to /jobs/dispatch. crontab.tpl is rendered at start-up by entrypoint.sh
# so the schedule can come from an environment variable instead of a baked-in
# crontab file.
COPY dispatch.sh   /usr/local/bin/dispatch.sh
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY crontab.tpl   /etc/crontabs/root.tpl

RUN chmod +x /usr/local/bin/dispatch.sh /usr/local/bin/entrypoint.sh

# Default cron expression. Override with GOSCRATCH_AUDIT_CRON_SCHEDULE at runtime.
ENV GOSCRATCH_AUDIT_CRON_SCHEDULE="0 3 * * *"
ENV GOSCRATCH_AUDIT_RETENTION_DAYS="90"

# tini reaps zombies left by crond's short-lived dispatch.sh invocations.
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/entrypoint.sh"]
