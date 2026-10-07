#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
source .github/scripts/registry-retention.sh

images='[
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:newest","createTime":"2026-10-07T05:00:00","tags":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:preview","createTime":"2026-10-07T04:30:00","tags":["pr-55-bbbbbbb"]},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:mixed","createTime":"2026-10-07T04:00:00","tags":["cccccccccccccccccccccccccccccccccccccccc","pr-54-ccccccc"]},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/other-api","version":"sha256:other","createTime":"2026-10-07T03:30:00","tags":["dddddddddddddddddddddddddddddddddddddddd"]},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:third","createTime":"2026-10-07T03:00:00","tags":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:fourth","createTime":"2026-10-07T02:00:00","tags":["ffffffffffffffffffffffffffffffffffffffff"]},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:oldest","createTime":"2026-10-07T01:00:00","tags":["1111111111111111111111111111111111111111"]},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:manual","createTime":"2026-10-06T23:00:00","tags":["latest"]},
  {"package":"australia-southeast1-docker.pkg.dev/project/repo/rally-club-api","version":"sha256:untagged","createTime":"2026-10-07T00:00:00","tags":[]}
]'

actual=$(printf '%s' "$images" | stale_main_digests 2 rally-club-api)
expected='australia-southeast1-docker.pkg.dev/project/repo/rally-club-api@sha256:third
australia-southeast1-docker.pkg.dev/project/repo/rally-club-api@sha256:fourth
australia-southeast1-docker.pkg.dev/project/repo/rally-club-api@sha256:oldest'

if [ "$actual" != "$expected" ]; then
  echo "unexpected production deletion candidates" >&2
  diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2 || true
  exit 1
fi

if [ -n "$(printf '%s' "$images" | stale_main_digests 5 rally-club-api)" ]; then
  echo "five production images should fit within a keep count of five" >&2
  exit 1
fi

echo "registry retention tests passed"
