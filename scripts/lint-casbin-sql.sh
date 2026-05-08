#!/usr/bin/env bash
# lint-casbin-sql.sh — Reject raw-SQL writes to casbin_rule(s) outside
# the allowed packages. Any INSERT/UPDATE/DELETE touching the casbin_rule
# table in a .go file outside the allowlist is a bypass of the watcher/event
# hooks and will silently desync the in-memory enforcer cache.
#
# Usage: bash scripts/lint-casbin-sql.sh
# Exit 0 = clean. Exit 1 = offending files found (paths printed to stderr).
#
# Allowlist (prefix match against git-relative path):
#   internal/adapter/casbin/ — the canonical adapter; all writes go through
#                              sqladapter which notifies the watcher.
#   scripts/seed/            — one-shot dev bootstrap; runs before the app
#                              starts, outside the watcher lifecycle.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

ALLOWED=(
    "internal/adapter/casbin/"
    "scripts/seed/"
)

PATTERN='(INSERT|UPDATE|DELETE)[[:space:][:print:]]*casbin_rule'

found=0

is_allowed() {
    local file="$1"
    for prefix in "${ALLOWED[@]}"; do
        case "$file" in
            "${prefix}"*) return 0 ;;
        esac
    done
    return 1
}

while IFS= read -r file; do
    is_allowed "$file" && continue

    matches=$(grep -EHni "$PATTERN" "$file" 2>/dev/null || true)
    if [ -n "$matches" ]; then
        printf '%s\n' "$matches" >&2
        found=1
    fi
done < <(git ls-files '*.go')

if [ "$found" -ne 0 ]; then
    echo "casbin SQL guard: FAIL — raw SQL writes to casbin_rule(s) detected outside allowed paths" >&2
    exit 1
fi

echo "casbin SQL guard: clean"
