#!/usr/bin/env bash
# Runs a gcloud command, retrying when Cloud Run rejects it because something
# else updated the service first.
#
# Cloud Run guards a service with optimistic concurrency, so two overlapping
# writes to the same service make the slower one fail. This first bit us when
# every PR preview was a tagged revision of the production service. Each PR now
# has a service of its own, but overlap is still possible: two releases on
# main, a release and a legacy tag cleanup, or a cancelled preview run that is
# still mid-write when its replacement starts.
#
#   ERROR: (gcloud.run.deploy) ABORTED: Conflict for resource 'rally-club-api':
#   version '1787464187027721' was specified but current version is '1787464187827232'
#
# That is a lost race, not a bad deploy — the same command succeeds once the
# other write lands. Retrying with backoff is the fix.
#
# A GitHub `concurrency` group would be the obvious alternative, but it is the
# wrong tool: only one run may sit pending per group, and a third arrival
# cancels the one already waiting. Merging a PR that re-syncs six others would
# leave four previews cancelled instead of queued.
#
# Usage:
#   .github/scripts/gcloud-retry-on-conflict.sh gcloud run deploy ... [flags]
set -uo pipefail

max_attempts="${MAX_ATTEMPTS:-6}"
attempt=1

while true; do
  if output="$("$@" 2>&1)"; then
    printf '%s\n' "$output"
    exit 0
  fi
  printf '%s\n' "$output"

  # Only a lost race is worth retrying. A bad image or a missing permission
  # fails identically six times over and buries the real error.
  if ! grep -qiE 'ABORTED|Conflict for resource' <<<"$output"; then
    echo "::error::Deploy failed for a reason other than a concurrent update; not retrying."
    exit 1
  fi

  if (( attempt >= max_attempts )); then
    echo "::error::Service was still being updated concurrently after ${max_attempts} attempts."
    exit 1
  fi

  # Backoff grows, with jitter so simultaneous losers do not retry in lockstep.
  delay=$(( attempt * 15 + RANDOM % 15 ))
  echo "Concurrent update detected; retrying in ${delay}s (attempt ${attempt}/${max_attempts})."
  sleep "${delay}"
  attempt=$(( attempt + 1 ))
done
