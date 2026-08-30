#!/usr/bin/env bash
# Shared helpers for finding and deleting Firebase Hosting preview channels.
#
# Sourced by preview-cleanup.yml from two places — the per-PR teardown and the
# sweep — so that "which channel belongs to which PR" is decided once. Requires
# firebase-tools on PATH and FIREBASE_PROJECT_ID in the environment.
#
# Deliberately sets no shell options: this file is sourced, so anything set here
# would leak into the rest of the step that sourced it.

# list_channels prints one channel id per line. The API returns fully qualified
# names (projects/../sites/../channels/<id>); only the last segment is usable
# as an argument to the CLI.
list_channels() {
  firebase hosting:channel:list --project "$FIREBASE_PROJECT_ID" --json 2>/dev/null |
    jq -r '[.result.channels[]?.name] | .[]' |
    awk -F/ '{print $NF}'
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
  firebase hosting:channel:delete "$channel" --project "$FIREBASE_PROJECT_ID" --force
}
