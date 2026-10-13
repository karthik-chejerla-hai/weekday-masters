#!/usr/bin/env bash
# Deploy only the private notification worker; never mutate the public API service.
set -euo pipefail

: "${GCP_PROJECT_ID:?}" "${GCP_REGION:?}" "${NOTIFICATION_WORKER_SERVICE:?}"
worker_job="${NOTIFICATION_WORKER_SERVICE}-minute"
action="${1:-deploy}"
[[ "$NOTIFICATION_WORKER_SERVICE" =~ ^rally-notifications(-pr-[0-9]+)?$ ]] || { echo "Unexpected worker service name" >&2; exit 1; }
if [[ "$action" == "cleanup" && "$NOTIFICATION_WORKER_SERVICE" != *-pr-* ]]; then
  echo "Cleanup requires a preview worker name" >&2; exit 1
fi
if [[ "$action" == "cleanup" ]]; then
  # Closed previews may never have opted into worker creation. Treat only
  # NOT_FOUND as absent; permissions/network failures must not claim success.
  for resource in scheduler run; do
    diagnostic=$(mktemp)
    if [[ "$resource" == "scheduler" ]]; then
      if gcloud scheduler jobs describe "$worker_job" --project="$GCP_PROJECT_ID" --location="$GCP_REGION" > /dev/null 2>"$diagnostic"; then
        gcloud scheduler jobs delete "$worker_job" --project="$GCP_PROJECT_ID" --location="$GCP_REGION" --quiet
      elif ! grep -q 'NOT_FOUND' "$diagnostic"; then cat "$diagnostic" >&2; rm -f "$diagnostic"; exit 1; fi
    else
      if gcloud run services describe "$NOTIFICATION_WORKER_SERVICE" --project="$GCP_PROJECT_ID" --region="$GCP_REGION" > /dev/null 2>"$diagnostic"; then
        gcloud run services delete "$NOTIFICATION_WORKER_SERVICE" --project="$GCP_PROJECT_ID" --region="$GCP_REGION" --quiet
      elif ! grep -q 'NOT_FOUND' "$diagnostic"; then cat "$diagnostic" >&2; rm -f "$diagnostic"; exit 1; fi
    fi
    rm -f "$diagnostic"
  done
  exit 0
fi
[[ "$action" == "deploy" ]] || { echo "Unknown worker action" >&2; exit 1; }
: "${NOTIFICATION_SCHEDULER_EMAIL:?}" "${IMAGE:?}" "${DATABASE_URL:?}" "${NOTIFICATION_WORKER_RUNTIME_EMAIL:?}" "${NOTIFICATION_WORKER_AUDIENCE:?}"
: "${NOTIFICATIONS_DISABLED:?}" "${FRONTEND_URL:?}" "${FIREBASE_PROJECT_ID:?}"
# Separate exact service names, job names and audiences for each PR. Runtime and
# scheduler identities are existing least-privilege accounts configured by owner.
if [[ "$NOTIFICATION_WORKER_SERVICE" == *-pr-* && "$NOTIFICATIONS_DISABLED" != "true" ]]; then
  echo "Preview worker delivery must remain disabled" >&2; exit 1
fi
umask 077
worker_env=$(mktemp)
diagnostic=$(mktemp)
trap 'rm -f "$worker_env" "$diagnostic"' EXIT
export NOTIFICATION_WORKER_AUDIENCE NOTIFICATION_SCHEDULER_EMAIL
python3 - "$worker_env" <<'PY'
import json, os, sys
keys=['DATABASE_URL','NOTIFICATIONS_DISABLED','FRONTEND_URL','FIREBASE_PROJECT_ID','NOTIFICATION_WORKER_AUDIENCE','NOTIFICATION_SCHEDULER_EMAIL']
with open(sys.argv[1],'w') as f:json.dump({**{k:os.environ[k] for k in keys},'GIN_MODE':'release','OTEL_ENABLED':'false','TIMEZONE':'Australia/Sydney'},f)
PY
# A custom audience lets the first revision start with a known exact audience,
# before the generated Cloud Run URL exists. Both IAM and the handler enforce it.
gcloud run deploy "$NOTIFICATION_WORKER_SERVICE" --project="$GCP_PROJECT_ID" --region="$GCP_REGION" \
  --image="$IMAGE" --command=./notification-worker --service-account="$NOTIFICATION_WORKER_RUNTIME_EMAIL" \
  --no-allow-unauthenticated --add-custom-audiences="$NOTIFICATION_WORKER_AUDIENCE" \
  --env-vars-file="$worker_env" --timeout=60 --concurrency=2 --max-instances=2 --min-instances=0 --quiet
# Restrict invocation to the scheduler account, not the public API identity.
gcloud run services add-iam-policy-binding "$NOTIFICATION_WORKER_SERVICE" --project="$GCP_PROJECT_ID" --region="$GCP_REGION" \
  --member="serviceAccount:${NOTIFICATION_SCHEDULER_EMAIL}" --role=roles/run.invoker --quiet
worker_url=$(gcloud run services describe "$NOTIFICATION_WORKER_SERVICE" --project="$GCP_PROJECT_ID" --region="$GCP_REGION" --format='value(status.url)')
[[ "$worker_url" == https://*.run.app ]] || { echo "Invalid private worker URL" >&2; exit 1; }
scheduler_flags=(--project="$GCP_PROJECT_ID" --location="$GCP_REGION" --schedule='* * * * *' --time-zone=Australia/Sydney \
  --uri="${worker_url}/internal/notifications/run" --http-method=POST --oidc-service-account-email="$NOTIFICATION_SCHEDULER_EMAIL" \
  --oidc-token-audience="$NOTIFICATION_WORKER_AUDIENCE" --attempt-deadline=60s --max-retry-attempts=3 --quiet)
if gcloud scheduler jobs describe "$worker_job" --project="$GCP_PROJECT_ID" --location="$GCP_REGION" >/dev/null 2>"$diagnostic"; then
  gcloud scheduler jobs update http "$worker_job" "${scheduler_flags[@]}"
elif grep -q 'NOT_FOUND' "$diagnostic"; then
  gcloud scheduler jobs create http "$worker_job" "${scheduler_flags[@]}"
else
  cat "$diagnostic" >&2; exit 1
fi
