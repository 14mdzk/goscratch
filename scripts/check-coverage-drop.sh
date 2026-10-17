#!/usr/bin/env bash
# check-coverage-drop.sh — Fail when combined Go coverage drops more than the
# allowed threshold (percentage points) below the main-branch baseline.
#
# The CI coverage-gate job feeds this two `go test -coverprofile` files: the
# current run's profile and the baseline profile downloaded from the latest
# successful main run. Totals come from `go tool cover -func`, so weighting
# matches the number CI reports. Tier F4: coverage must not drift down as
# features outpace tests.
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

total_of() {
	go tool cover -func="$1" | awk '/^total:/ {gsub(/%/, "", $3); print $3}'
}

CUR_TOTAL="$(total_of "$CURRENT")"
BASE_TOTAL="$(total_of "$BASELINE")"

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
