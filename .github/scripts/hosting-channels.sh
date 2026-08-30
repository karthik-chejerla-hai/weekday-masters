#!/usr/bin/env bash
# Shared helpers for finding and deleting Firebase Hosting preview channels.
#
# Sourced by preview-cleanup.yml from two places — the per-PR teardown and the
# sweep — so that "which channel belongs to which PR" is decided once. Requires
# firebase-tools on PATH and FIREBASE_PROJECT_ID in the environment.
#
# Every command passes --config: the Hosting commands refuse to run outside a
# directory holding firebase.json ("Not in a Firebase app directory"), and this
# repo keeps it under frontend/ while the workflow runs from the repo root.
#
# Deliberately sets no shell options: this file is sourced, so anything set here
# would leak into the rest of the step that sourced it.

# The Hosting config this repo deploys from. Overridable, but there is only one.
FIREBASE_CONFIG="${FIREBASE_CONFIG:-frontend/firebase.json}"

# list_channels prints one channel id per line. The API returns fully qualified
# names (projects/../sites/../channels/<id>); only the last segment is usable
# as an argument to the CLI.
#
# Two failure modes are handled explicitly, because the first version of this
# script hit both and reported a successful cleanup while deleting nothing:
#
#   1. The CLI's exit status has to be checked on its own. Reading it through a
#      pipeline gives the status of the last stage instead — jq and awk both
#      succeed on empty input — and GitHub's default shell is `bash -e`, not
#      `-o pipefail`, so the failure passes straight through.
#   2. stderr must not be discarded, or the reason is gone.
list_channels() {
  local raw ids
  if ! raw=$(firebase hosting:channel:list \
    --project "$FIREBASE_PROJECT_ID" \
    --config "$FIREBASE_CONFIG" --json 2>&1); then
    echo "firebase hosting:channel:list failed:" >&2
    echo "$raw" >&2
    return 1
  fi

  ids=$(printf '%s' "$raw" | jq -r '[.result.channels[]?.name] | .[]' 2>/dev/null |
    awk -F/ '{print $NF}')

  # Every site has a "live" channel. An empty list therefore means the query did
  # not work, not that there is nothing to clean — treating those two as the
  # same thing is exactly what produced a green run that deleted nothing.
  if [ -z "$ids" ]; then
    echo "::error::Hosting channel list came back empty, which cannot be right - refusing to report a clean teardown" >&2
    echo "$raw" >&2
    return 1
  fi

  printf '%s\n' "$ids"
}

# pr_number_for_channel echoes the PR number a channel belongs to, or nothing
# when the channel is not a PR preview. The deploy action names channels
# "pr<number>-<branch>", truncated to fit Firebase's limit, so the number is
# read from the front rather than the (possibly cut) branch name.
pr_number_for_channel() {
  local channel="$1"
  [[ "$channel" =~ ^pr([0-9]+)(-.*)?$ ]] && echo "${BASH_REMATCH[1]}"
}

# channel_belongs_to_pr is the yes/no form, for the per-PR teardown.
channel_belongs_to_pr() {
  local channel="$1" pr="$2"
  [ "$(pr_number_for_channel "$channel")" = "$pr" ]
}

# delete_channel removes one channel. "live" is the production site and is
# never a preview, so it is refused outright rather than trusted not to appear.
delete_channel() {
  local channel="$1"
  if [ "$channel" = "live" ]; then
    echo "::error::refusing to delete the live channel"
    return 1
  fi
  firebase hosting:channel:delete "$channel" \
    --project "$FIREBASE_PROJECT_ID" \
    --config "$FIREBASE_CONFIG" --force
}
