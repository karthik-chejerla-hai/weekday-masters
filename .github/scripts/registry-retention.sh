#!/usr/bin/env bash
# Helpers for pruning preview build artefacts.
#
# Sourced by infra-retention.yml. The filtering lives here rather than inline in
# the workflow so it can be exercised against captured `gcloud ... --format=json`
# output without deploying anything — the last cleanup bug in this repo was a
# shell mistake that only CI could see, and once was enough.
#
# Deliberately sets no shell options: this file is sourced, so anything set here
# would leak into the rest of the step that sourced it.

# cutoff_rfc3339 echoes the timestamp N days ago, the form gcloud reports times
# in, so comparisons are plain string comparisons on a fixed-width format.
cutoff_rfc3339() {
  date -u -d "${1} days ago" +%Y-%m-%dT%H:%M:%S 2>/dev/null ||
    date -u -v-"${1}"d +%Y-%m-%dT%H:%M:%S
}

# stale_pr_digests reads `gcloud artifacts docker images list --include-tags
# --format=json` on stdin and prints the full digest reference of every preview
# image older than the cutoff.
#
# Only images whose tags are ALL preview tags are eligible. Production images
# are tagged with the full commit sha and previews with "pr-<number>-<sha>", so
# a version carrying both — possible if a PR is merged at the same sha — is a
# production image wearing a preview tag, and deleting it would pull the image
# out from under a revision that may still need to scale up.
stale_pr_digests() {
  local cutoff="$1"
  jq -r --arg cutoff "$cutoff" '
    def tag_list:
      if (.tags | type) == "array" then .tags
      elif (.tags | type) == "string" then (.tags | split(","))
      else [] end
      | map(select(length > 0));

    .[]
    | . as $image
    | tag_list as $tags
    | select($tags | length > 0)
    | select($tags | all(startswith("pr-")))
    | select($image.createTime < $cutoff)
    | "\($image.package)@\($image.version)"
  '
}

# prunable_revisions reads `gcloud run revisions list --format=json` on stdin and
# prints the revisions safe to delete, newest first, skipping the newest $keep.
#
# A revision is only ever a candidate when it serves no traffic and carries no
# tag. Those two conditions are what make this safe: the serving revision, the
# rollback target and any live preview all fail at least one of them.
prunable_revisions() {
  local keep="$1"
  jq -r --argjson keep "$keep" '
    [ .[]
      | select(((.status.conditions // []) | length) >= 0)
      | { name: .metadata.name,
          created: .metadata.creationTimestamp,
          traffic: ((.status.traffic // []) | map(.percent // 0) | add // 0),
          tags: ((.status.traffic // []) | map(.tag // empty) | length) }
    ]
    | sort_by(.created) | reverse
    | .[$keep:]
    | map(select(.traffic == 0 and .tags == 0))
    | .[].name
  '
}
