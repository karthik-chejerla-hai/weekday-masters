#!/usr/bin/env bash
# Local protocol tests. gcloud is mocked; no cloud resources or real secrets.
set -euo pipefail
script_dir=$(cd "$(dirname "$0")" && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
cat > "$test_dir/gcloud" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_LOG"
if [[ "$*" == *'scheduler jobs describe'* ]]; then
 if [[ "${MOCK_FAILURE:-}" == permission ]]; then echo PERMISSION_DENIED >&2; exit 1; fi
 echo NOT_FOUND >&2; exit 1
fi
if [[ "$*" == *'run services describe'* ]]; then
 if [[ "${MOCK_WORKER:-absent}" == absent ]]; then echo NOT_FOUND >&2; exit 1; fi
 printf 'https://worker.example.run.app\n'
fi
if [[ "$*" == *'run deploy'* ]]; then
 for arg in "$@"; do
  if [[ "$arg" == --env-vars-file=* ]]; then
   file="${arg#*=}"
   python3 - "$file" <<'PY'
import json,sys,os,stat
assert stat.S_IMODE(os.stat(sys.argv[1]).st_mode)==0o600
with open(sys.argv[1]) as f: assert json.load(f)['DATABASE_URL']=='synthetic-test-only'
PY
   printf '%s' "$file" > "$MOCK_ENV_PATH"
  fi
 done
fi
MOCK
chmod +x "$test_dir/gcloud"
export PATH="$test_dir:$PATH" MOCK_LOG="$test_dir/calls" MOCK_ENV_PATH="$test_dir/env-path"
export GCP_PROJECT_ID=synthetic GCP_REGION=australia-southeast1 NOTIFICATION_WORKER_SERVICE=rally-notifications-pr-81
# Never enabled: Scheduler API/IAM is not needed to clean the absent service.
: > "$MOCK_LOG"
bash "$script_dir/notification-worker.sh" cleanup
! grep -q scheduler "$MOCK_LOG"
# Disabling after provisioning still removes jobs before the existing worker.
export MOCK_WORKER=exists NOTIFICATION_WORKER_ENABLED=false
: > "$MOCK_LOG"
bash "$script_dir/notification-worker.sh" cleanup
grep -q 'scheduler jobs describe rally-notifications-pr-81-health' "$MOCK_LOG"
grep -q 'run services delete rally-notifications-pr-81' "$MOCK_LOG"
export MOCK_FAILURE=permission
if bash "$script_dir/notification-worker.sh" cleanup >/dev/null 2>&1; then echo 'Permission failure claimed cleanup success' >&2; exit 1; fi
unset MOCK_FAILURE
export IMAGE=synthetic DATABASE_URL=synthetic-test-only NOTIFICATION_WORKER_RUNTIME_EMAIL=runtime@example.test
export NOTIFICATION_WORKER_AUDIENCE=https://rally.internal/notifications/pr-81 NOTIFICATION_SCHEDULER_EMAIL=scheduler@example.test
export NOTIFICATIONS_DISABLED=true FRONTEND_URL=https://invalid.preview FIREBASE_PROJECT_ID=synthetic
: > "$MOCK_LOG"
bash "$script_dir/notification-worker.sh" deploy
grep -q -- '--no-allow-unauthenticated' "$MOCK_LOG"
grep -q -- '--command=./notification-worker' "$MOCK_LOG"
grep -q 'scheduler jobs create http rally-notifications-pr-81-health' "$MOCK_LOG"
! grep -q synthetic-test-only "$MOCK_LOG"
[[ ! -e "$(cat "$MOCK_ENV_PATH")" ]]
export NOTIFICATIONS_DISABLED=false
if bash "$script_dir/notification-worker.sh" deploy >/dev/null 2>&1; then echo 'Preview delivery allowed' >&2; exit 1; fi
export NOTIFICATION_WORKER_SERVICE=rally-club-api
if bash "$script_dir/notification-worker.sh" cleanup >/dev/null 2>&1; then echo 'Public service cleanup allowed' >&2; exit 1; fi
printf 'Notification worker deployment protocol tests passed.\n'
