#!/usr/bin/env bash
# lint-migration-fixture.sh — Reject drift between the canonical migrations
# and the testutil fixture copy.
#
# internal/platform/testutil/migrations/ must stay byte-identical (*.sql) to
# migrations/ because testutil.StartPostgres embeds the copy, not the
# originals (go:embed cannot follow a symlink out of the package dir, so a
# single-sourced layout is not possible). A silent drift means integration
# tests exercise a schema that differs from prod (issue #119: the copy once
# lacked 000004_postgis entirely plus the 000003 seed row).
#
# Usage: bash scripts/lint-migration-fixture.sh
# Exit 0 = in sync. Exit 1 = drift found (diff printed to stderr).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

drift=0
for src in migrations/*.up.sql migrations/*.down.sql; do
	base="$(basename "$src")"
	dst="internal/platform/testutil/migrations/$base"
	if [[ ! -f "$dst" ]]; then
		echo "migration fixture drift: missing $dst (copy of $src)" >&2
		drift=1
	elif ! cmp -s "$src" "$dst"; then
		echo "migration fixture drift: $dst differs from $src" >&2
		diff -u "$src" "$dst" >&2 || true
		drift=1
	fi
done

# Stray files in the fixture (anything but the mirrored *.sql) are drift too.
for dst in internal/platform/testutil/migrations/*; do
	base="$(basename "$dst")"
	if [[ ! -f "migrations/$base" ]]; then
		echo "migration fixture drift: stray $dst (no counterpart in migrations/)" >&2
		drift=1
	fi
done

if [[ "$drift" -ne 0 ]]; then
	echo "migration-fixture: IN SYNC CHECK FAILED — copy migrations/*.sql over internal/platform/testutil/migrations/" >&2
	exit 1
fi
echo "migration-fixture: in sync"
