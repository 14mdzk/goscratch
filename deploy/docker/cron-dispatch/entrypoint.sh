#!/bin/sh
# entrypoint.sh — renders the crontab from a template using the env-supplied
# schedule, then exec's crond in the foreground. Logs are written to stdout so
# the container is observable with the usual `docker logs` / `kubectl logs`.
set -eu

schedule="${GOSCRATCH_AUDIT_CRON_SCHEDULE:-0 3 * * *}"

# Render the crontab. Use a temp file so a malformed template never wipes the
# real crontab mid-rotation.
tmp=$(mktemp)
sed "s|__SCHEDULE__|$schedule|" /etc/crontabs/root.tpl > "$tmp"
mv "$tmp" /etc/crontabs/root
chmod 0600 /etc/crontabs/root

echo "entrypoint.sh: cron schedule installed as: $schedule"

# crond -f: foreground; -d 8: log to stdout at debug level 8 (busybox-specific).
exec crond -f -d 8
