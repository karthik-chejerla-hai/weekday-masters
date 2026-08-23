#!/usr/bin/env bash
# Fails when total Go statement coverage drops below MIN_COVERAGE.
#
# Go has no built-in threshold flag, so the total is read back out of the
# `go tool cover -func` summary, whose last line looks like:
#
#   total:   (statements)   56.1%
set -euo pipefail

summary="${1:?usage: check-go-coverage.sh <coverage-summary.txt>}"
minimum="${MIN_COVERAGE:?MIN_COVERAGE must be set}"

if [[ ! -f "$summary" ]]; then
  echo "coverage summary not found: $summary" >&2
  exit 1
fi

total="$(awk '/^total:/ { gsub(/%/, "", $NF); print $NF }' "$summary")"

if [[ -z "$total" ]]; then
  echo "could not read a total from $summary" >&2
  exit 1
fi

# Bash cannot compare decimals, and awk is already here.
if awk -v got="$total" -v min="$minimum" 'BEGIN { exit !(got < min) }'; then
  echo "::error::Backend coverage ${total}% is below the ${minimum}% floor."
  echo "Add tests, or lower MIN_COVERAGE in .github/workflows/ci.yml with a reason."
  exit 1
fi

echo "Backend coverage ${total}% meets the ${minimum}% floor."
