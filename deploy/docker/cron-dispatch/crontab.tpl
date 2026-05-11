# goscratch audit-cleanup cron — rendered at container start by entrypoint.sh.
# Do not edit; the SCHEDULE placeholder is replaced from the
# GOSCRATCH_AUDIT_CRON_SCHEDULE environment variable.
__SCHEDULE__ /usr/local/bin/dispatch.sh >> /proc/1/fd/1 2>> /proc/1/fd/2
