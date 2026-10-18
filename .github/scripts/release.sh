#!/usr/bin/env bash
# The same source and backend image move from dev to prod. Never seed prod.
set -euo pipefail
for name in RELEASE_ENV RELEASE_SHA GCP_PROJECT_ID RUNTIME_SA AUTH0_DOMAIN AUTH0_CLIENT_ID AUTH0_AUDIENCE ADMIN_EMAIL FIREBASE_API_KEY FIREBASE_APP_ID FIREBASE_MESSAGING_SENDER_ID FIREBASE_VAPID_KEY; do
  [[ -n "${!name:-}" ]] || { echo "Missing release setting: $name" >&2; exit 1; }
done
[[ "$RELEASE_SHA" =~ ^[a-f0-9]{40}$ ]] || { echo 'Invalid source commit' >&2; exit 1; }
case "$RELEASE_ENV:$GCP_PROJECT_ID" in
  dev:rally-club-app-dev) secret=dev-database-url; repository=rally; frontend=https://rally-club-app-dev.web.app ;;
  prod:rally-club-app) secret=''; repository=rally-club-api-registry; frontend=https://rally-club-app.web.app ;;
  *) echo 'Environment and cloud project do not match' >&2; exit 1 ;;
esac
[[ "$(git rev-parse HEAD)" == "$RELEASE_SHA" ]] || { echo 'Checkout does not match release' >&2; exit 1; }
region=australia-southeast1
export NOTIFICATIONS_DISABLED="${NOTIFICATIONS_DISABLED:-false}"
worker=false
if [[ -f backend/cmd/notification-worker/main.go ]]; then
  worker=true
  [[ -f .github/scripts/notification-worker.sh ]] || { echo 'Worker deploy helper is missing' >&2; exit 1; }
  : "${NOTIFICATION_SCHEDULER_EMAIL:?Private worker scheduler identity is required}"
  : "${WORKER_RUNTIME_SA:?Private worker runtime identity is required}"
fi
image="$region-docker.pkg.dev/$GCP_PROJECT_ID/$repository/rally-club-api:$RELEASE_SHA"
# Migration and deployment need different access. Credentials never enter release artifacts.
if [[ "$RELEASE_ENV" == dev ]]; then
  DATABASE_URL=$(gcloud secrets versions access latest --secret="$secret" --project="$GCP_PROJECT_ID")
  echo "::add-mask::$DATABASE_URL"
  export DATABASE_URL
  docker build -t "$image" backend
  docker save "$image" | gzip > /tmp/rally-image.tar.gz
else
  : "${DATABASE_URL:?Missing production database}"
  # Verify dev still runs the exact image that the reviewer tested.
  python3 scripts/release/identity.py verify /tmp/dev-release/release.json
  docker load -i /tmp/dev-release/rally-image.tar.gz
  source_image=$(python3 -c 'import json; print(json.load(open("/tmp/dev-release/release.json"))["image"])')
  docker tag "$source_image" "$image"
fi
docker push "$image"
digest=$(gcloud artifacts docker images describe "$image" --project="$GCP_PROJECT_ID" --format='value(image_summary.digest)')
[[ "$digest" =~ ^sha256:[a-f0-9]{64}$ ]] || { echo 'Image digest is missing' >&2; exit 1; }
if [[ "$RELEASE_ENV" == prod ]]; then
  expected=$(python3 -c 'import json; print(json.load(open("/tmp/dev-release/release.json"))["digest"])')
  [[ "$digest" == "$expected" ]] || { echo 'Production image differs from tested image' >&2; exit 1; }
fi
(cd backend && go run ./cmd/migrate)
flags=(--project="$GCP_PROJECT_ID" --region="$region" --image="${image%:*}@$digest"
  --service-account="$RUNTIME_SA" --allow-unauthenticated --min-instances=1 --max-instances=1 --no-cpu-throttling
  --update-env-vars="GIN_MODE=release,TIMEZONE=Australia/Sydney,FRONTEND_URL=$frontend,FIREBASE_PROJECT_ID=$GCP_PROJECT_ID,AUTH0_DOMAIN=$AUTH0_DOMAIN,AUTH0_AUDIENCE=$AUTH0_AUDIENCE,ADMIN_EMAIL=$ADMIN_EMAIL,NOTIFICATIONS_DISABLED=$NOTIFICATIONS_DISABLED,NOTIFICATION_WORKER_ENABLED=$worker,INVITATION_TEST_EMAILS_ENABLED=false,SESSION_REMINDER_HOURS_24=24,SESSION_REMINDER_HOURS_12=12,DEADLINE_REMINDER_HOURS=6")
if [[ "$RELEASE_ENV" == dev ]]; then
  flags+=(--update-secrets="DATABASE_URL=$secret:latest" --update-env-vars='OTEL_ENABLED=false,SENDGRID_API_KEY=,GROQ_API_KEY=')
  # Synthetic rows only. This command cannot read production data.
  (cd backend && SEED_ALLOW_PREVIEW=true go run ./cmd/seed -env=preview)
else
  # Preserve production's existing telemetry configuration and secrets.
  # update-env-vars avoids removing settings added by later notification PRs.
  flags+=(--update-env-vars="DATABASE_URL=$DATABASE_URL,GROQ_API_KEY=${GROQ_API_KEY:-},SENDGRID_API_KEY=${SENDGRID_API_KEY:-},SENDGRID_FROM_EMAIL=${SENDGRID_FROM_EMAIL:-},SENDGRID_FROM_NAME=Rally")
fi
if [[ "$worker" == true ]]; then
  export GCP_REGION="$region" IMAGE="${image%:*}@$digest" FRONTEND_URL="$frontend" FIREBASE_PROJECT_ID="$GCP_PROJECT_ID"
  export NOTIFICATION_WORKER_SERVICE=rally-notifications NOTIFICATION_WORKER_RUNTIME_EMAIL="$WORKER_RUNTIME_SA"
  export NOTIFICATION_WORKER_AUDIENCE="https://rally.internal/notifications/$RELEASE_ENV"
  bash .github/scripts/notification-worker.sh deploy
fi
# Repeated update flags preserve settings not owned by this release script.
gcloud run deploy rally-club-api "${flags[@]}" --quiet
url=$(gcloud run services describe rally-club-api --project="$GCP_PROJECT_ID" --region="$region" --format='value(status.url)')
curl --fail --retry 8 --retry-all-errors --retry-delay 5 "$url/health"
{
  echo "VITE_API_URL=$url/api"
  echo "RELEASE_IMAGE=$image"
  echo "RELEASE_DIGEST=$digest"
} >> "$GITHUB_ENV"
