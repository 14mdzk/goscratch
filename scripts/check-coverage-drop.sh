#!/usr/bin/env bash
# check-coverage-drop.sh — Fail when combined Go coverage drops more than the
# allowed threshold (percentage points) below the main-branch baseline.
#
# The CI coverage-gate job feeds this two `go test -coverprofile` files: the
# current run's profile and the baseline profile downloaded from the latest
# successful main run. Totals are computed straight from the profile lines
# (covered statements / total statements, same weighting `go tool cover`
# reports) WITHOUT resolving the source files — deliberately so, because the
# baseline profile may reference files that do not exist in the PR checkout
# (added on main after the PR branched) and vice versa; `go tool cover`
# hard-fails on those (exit 2, seen live on PR #132). Tier F4: coverage must
# not drift down as features outpace tests.
#
# Usage: bash scripts/check-coverage-drop.sh <current.prof> <baseline.prof> [max_drop_points]
#   exit 0 — drop within budget (or baseline above current: never a failure)
#   exit 1 — drop exceeds budget, or inputs unusable
set -euo pipefail

CURRENT="${1:?usage: check-coverage-drop.sh <current.prof> <baseline.prof> [max_drop_points]}"
BASELINE="${2:?usage: check-coverage-drop.sh <current.prof> <baseline.prof> [max_drop_points]}"
MAX_DROP="${3:-1.0}"

for f in "$CURRENT" "$BASELINE"; do
	if [[ ! -s "$f" ]]; then
		echo "check-coverage-drop: profile missing or empty: $f" >&2
		exit 1
	fi
done

# total_of prints the profile total as a percentage with one decimal (same
# shape as `go tool cover -func`'s total line). Skips the `mode:` header;
# every other line is `<path>:<span> <numStmt> <count>` in all three modes
# (set/count/atomic), and a statement counts as covered when count > 0.
# Prints nothing and exits 1 when the profile holds no statements.
total_of() {
	awk '$1 != "mode:" {stmts += $2; if ($3 > 0) {covered += $2}} END {if (stmts == 0) {exit 1}; printf "%.1f", 100 * covered / stmts}' "$1"
}

CUR_TOTAL="$(total_of "$CURRENT" || true)"
BASE_TOTAL="$(total_of "$BASELINE" || true)"

if [[ -z "$CUR_TOTAL" || -z "$BASE_TOTAL" ]]; then
	echo "check-coverage-drop: could not parse totals (current='$CUR_TOTAL', baseline='$BASE_TOTAL')" >&2
	exit 1
fi

echo "check-coverage-drop: baseline=${BASE_TOTAL}% current=${CUR_TOTAL}% budget=${MAX_DROP}pt"

awk -v base="$BASE_TOTAL" -v cur="$CUR_TOTAL" -v max="$MAX_DROP" 'BEGIN {
	drop = base - cur
	if (drop > max) {
		printf "check-coverage-drop: FAIL — coverage dropped %.2fpt (%.2f%% -> %.2f%%), budget %.2fpt\n", drop, base, cur, max > "/dev/stderr"
		exit 1
	}
	printf "check-coverage-drop: OK — drop %.2fpt within %.2fpt budget\n", (drop < 0 ? 0 : drop), max
}'
