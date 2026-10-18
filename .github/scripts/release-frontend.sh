#!/usr/bin/env bash
set -euo pipefail
: "${VITE_API_URL:?}" "${RELEASE_IMAGE:?}" "${RELEASE_DIGEST:?}"
case "$RELEASE_ENV:$GCP_PROJECT_ID" in
  dev:rally-club-app-dev) frontend=https://rally-club-app-dev.web.app ;;
  prod:rally-club-app) frontend=https://rally-club-app.web.app ;;
  *) echo 'Invalid frontend environment' >&2; exit 1 ;;
esac
export VITE_API_URL="$VITE_API_URL" VITE_AUTH0_DOMAIN="$AUTH0_DOMAIN" VITE_AUTH0_CLIENT_ID="$AUTH0_CLIENT_ID" VITE_AUTH0_AUDIENCE="$AUTH0_AUDIENCE"
export VITE_FIREBASE_API_KEY="$FIREBASE_API_KEY" VITE_FIREBASE_PROJECT_ID="$GCP_PROJECT_ID" VITE_FIREBASE_AUTH_DOMAIN="$GCP_PROJECT_ID.firebaseapp.com"
export VITE_FIREBASE_STORAGE_BUCKET="$GCP_PROJECT_ID.appspot.com" VITE_FIREBASE_APP_ID="$FIREBASE_APP_ID" VITE_FIREBASE_MESSAGING_SENDER_ID="$FIREBASE_MESSAGING_SENDER_ID" VITE_FIREBASE_VAPID_KEY="$FIREBASE_VAPID_KEY"
export VITE_APP_ENVIRONMENT="$RELEASE_ENV" VITE_RELEASE_SHA="$RELEASE_SHA"
(cd frontend && npm ci && npm run build)
python3 scripts/release/identity.py write frontend/dist/release.json
cp frontend/dist/release.json /tmp/release.json
(cd frontend && firebase deploy --only hosting --project="$GCP_PROJECT_ID" --non-interactive)
python3 scripts/release/identity.py live "$frontend/release.json"
{
  echo "### $RELEASE_ENV deployment"
  echo "- Source: $RELEASE_SHA"
  echo "- Site: $frontend"
  echo "- Backend image: $RELEASE_DIGEST"
  echo '- Test sign-in, RSVP, balances, push permission, delivery, and reminder links before approval.'
} >> "$GITHUB_STEP_SUMMARY"
