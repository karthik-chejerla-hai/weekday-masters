# Deployment Guide

Deploy Rally to Google Cloud (free tier).

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Firebase       │────▶│  Cloud Run      │────▶│  Neon           │
│  Hosting        │     │  (Go Backend)   │     │  (PostgreSQL)   │
│  (React SPA)    │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

## Preview environments

Every pull request gets its own Neon branch, created by `preview-deploy.yml` and
deleted by `preview-cleanup.yml` when the PR closes. The branch is copy-on-write,
so it is effectively instant and carries production-shaped data, and migrations
run against it before the preview backend is deployed — a PR that adds tables
gets them.

Previews previously pointed straight at the production `DATABASE_URL` with no
migration step, which meant a schema-changing PR deployed a backend querying
columns that did not exist, and every preview wrote to real data.

### What closing a PR tears down

`preview-cleanup.yml` deletes the Neon branch, removes the Cloud Run revision
tag, and deletes the Firebase Hosting preview channel. Its last step then
re-checks the tag and the channel and fails if either survives: every step
before it tolerates "already gone", so without that check the job would report
success whether or not anything was actually removed. That is how the missing
Hosting step went unnoticed — merged PRs kept serving a public preview until the
7-day `expires` on the deploy caught up with them.

Two things outlive it, and are handled by `infra-retention.yml` rather than at
PR-close time:

- **Artifact Registry images.** Every production and preview deploy pushes a
  commit-tagged image. An Artifact Registry cleanup policy, defined in
  `.github/artifact-registry-cleanup-policy.json` and applied by that workflow,
  retains the existing preview behavior: `pr-`-tagged images become eligible
  after 7 days while the policy keeps the 20 most recent versions whatever their
  age. A separate sweep identifies production images by their plain commit-SHA
  tags and keeps only the five newest production builds; preview-only images do
  not count toward that five. Cloud Run imports an image when it creates an
  immutable revision, so deleting the registry copy does not interrupt a deployed
  revision; an older commit can be rebuilt if it ever needs to be redeployed.
- **Cloud Run revisions.** `--remove-tags` takes the `pr-N` URL out of service
  but leaves the revision behind, untagged and serving no traffic. The same
  workflow prunes them, keeping the 20 newest and only ever considering a
  revision that serves no traffic **and** carries no tag — the live revision, the
  rollback target and any open PR's preview each fail at least one of those.

`infra-retention.yml` runs weekly and can be dispatched manually; a manual run
defaults to a dry run, a scheduled one acts. It ends by checking the production
backend still answers on `/health`, which is the thing that would matter if one
of those guards were ever wrong.

Run the workflow manually (`workflow_dispatch`) to sweep up preview channels for
PRs that are already closed. It defaults to a dry run; set `dry_run` to false to
actually delete.

Required repository configuration:

| Name | Kind | Notes |
|------|------|-------|
| `NEON_API_KEY` | secret | Neon account or org API key |
| `NEON_PROJECT_ID` | variable | from the Neon console URL; an identifier, not a credential |
| `NEON_DB_ROLE` | variable | optional; defaults to `neondb_owner` |
| `NEON_DB_NAME` | variable | optional; defaults to `neondb` |

Set the two variables only if your Neon role and database are not the defaults —
check the production `DATABASE_URL`: it reads
`postgres://<role>@<host>/<database>`.


## Prerequisites

1. [Google Cloud account](https://console.cloud.google.com/) with billing enabled
2. [Firebase CLI](https://firebase.google.com/docs/cli): `npm install -g firebase-tools`
3. [gcloud CLI](https://cloud.google.com/sdk/docs/install)
4. [Neon account](https://neon.tech/) (you already have this)

---

## Step 1: Create Google Cloud Project

```bash
# Login to gcloud
gcloud auth login

# Create a new project (or use existing)
gcloud projects create rally --name="Rally"

# Set as default project
gcloud config set project rally

# Enable required APIs
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable artifactregistry.googleapis.com
```

---

## Step 2: Deploy Backend to Cloud Run

### 2.1 Build and Deploy

```bash
cd backend

# Deploy to Cloud Run (builds container automatically)
gcloud run deploy rally-api \
  --source . \
  --region australia-southeast1 \
  --allow-unauthenticated \
  --set-env-vars "GIN_MODE=release" \
  --set-env-vars "TIMEZONE=Australia/Sydney"
```

### 2.2 Set Environment Variables

After initial deployment, set secrets:

```bash
# Set environment variables (replace with your values)
gcloud run services update rally-api \
  --region australia-southeast1 \
  --set-env-vars "DATABASE_URL=postgresql://user:pass@ep-xxx.neon.tech/dbname?sslmode=require" \
  --set-env-vars "AUTH0_DOMAIN=your-tenant.auth0.com" \
  --set-env-vars "AUTH0_AUDIENCE=https://your-api-identifier" \
  --set-env-vars "ADMIN_EMAIL=admin@example.com" \
  --set-env-vars "FRONTEND_URL=https://your-project.web.app"
```

### 2.3 Get Backend URL

```bash
gcloud run services describe rally-api \
  --region australia-southeast1 \
  --format 'value(status.url)'
```

Save this URL – you'll need it for the frontend.

---

## Step 3: Deploy Frontend to Firebase Hosting

### 3.1 Set up Firebase

```bash
# Login to Firebase
firebase login

# Create a new Firebase project (or link to existing GCP project)
firebase projects:create rally --display-name "Rally"

# Or use existing GCP project
firebase projects:addfirebase rally
```

### 3.2 Update Firebase Config

Edit `frontend/.firebaserc`:

```json
{
  "projects": {
    "default": "rally"
  }
}
```

### 3.3 Create Production Environment

```bash
cd frontend

# Create .env.production with your values
cat > .env.production << EOF
VITE_API_URL=https://rally-api-xxxxx-ts.a.run.app/api
VITE_AUTH0_DOMAIN=your-tenant.auth0.com
VITE_AUTH0_CLIENT_ID=your-production-client-id
VITE_AUTH0_AUDIENCE=https://your-api-identifier
EOF
```

### 3.4 Build and Deploy

```bash
# Install dependencies
npm install

# Build for production
npm run build

# Deploy to Firebase
firebase deploy --only hosting
```

Your app will be live at: `https://rally.web.app`

---

## Step 4: Update Auth0 Configuration

Add these URLs to your Auth0 application settings:

**Allowed Callback URLs:**
```
https://rally.web.app/callback
```

**Allowed Logout URLs:**
```
https://rally.web.app
```

**Allowed Web Origins:**
```
https://rally.web.app
```

---

## Step 5: Update Backend CORS

Update the `FRONTEND_URL` environment variable in Cloud Run:

```bash
gcloud run services update rally-api \
  --region australia-southeast1 \
  --update-env-vars "FRONTEND_URL=https://rally.web.app"
```

---

## Quick Deploy Scripts

### Deploy Backend

```bash
#!/bin/bash
cd backend
gcloud run deploy rally-api \
  --source . \
  --region australia-southeast1 \
  --allow-unauthenticated
```

### Deploy Frontend

```bash
#!/bin/bash
cd frontend
npm run build
firebase deploy --only hosting
```

---

## Estimated Free Tier Usage

| Service          | Your App (estimated) | Free Tier Limit   |
|------------------|----------------------|-------------------|
| Cloud Run        | ~10K requests/month  | 2M requests/month |
| Firebase Hosting | ~1GB transfer/month  | 10GB/month        |
| Neon PostgreSQL  | ~100MB storage       | 500MB storage     |

You should stay well within free tier limits for a small club app.

---

## Troubleshooting

### Cold Starts
Cloud Run scales to zero. The first request after idle may take 1–2 seconds.

### CORS Errors
Ensure `FRONTEND_URL` env var in Cloud Run matches your Firebase Hosting URL exactly.

### Database Connection
Ensure your Neon database allows connections from Cloud Run (it should by default).

### Build Failures
```bash
# View Cloud Build logs
gcloud builds list --limit=5
gcloud builds log BUILD_ID
```
## Push notifications (issue #49)

Production sets `FIREBASE_PROJECT_ID`, while the old notification service only
initialized FCM when `FIREBASE_CREDENTIALS` contained a private key. Production
logs confirmed `Firebase credentials not configured, push notifications disabled`
with `NOTIFICATIONS_DISABLED=false` on `rally-club-app/rally-club-api`.
The FCM API was enabled, and the runtime account had the existing Editor role.
The service now uses Application Default
Credentials from the Cloud Run runtime account when a Firebase project is set.
A JSON credentials override remains available for local use.

Before deployment, verify that the Firebase project's `fcm.googleapis.com` API is
enabled and the **runtime** service account has `cloudmessaging.messages.create`
(for example through `roles.firebasecloudmessaging.admin`). The GitHub deployer
identity is not the runtime identity. Keep both notification pause controls in
place; preview deployments must retain `NOTIFICATIONS_DISABLED=true`. Preview
may set `NOTIFICATIONS_DISABLED_ALLOW_EMAIL` to the verified admin email so
one account can test push delivery without contacting other members.

Frontend builds embed only the public Firebase web configuration in
`firebase-messaging-sw.js`. The worker initializes on every start, including when
no page is open. It has a separate scope from the PWA worker, and `getToken` uses
that exact registration. The worker is served with `Cache-Control: no-cache`.
FCM displays notification payloads once; the worker handles clicks and data-only
messages. Signed-in members with existing permission refresh their device token.
The profile page can retry registration when permission exists but registration
failed. Foreground messages appear as a link in the app.

Automatic email alerts and their preference controls are removed. Historical
email delivery fields and old preference columns are retained. Explicit admin
invitation emails use their separate service and existing sending controls.

After deployment, verify startup reports FCM initialization, then use a test
member/device to check foreground, background, and closed-page delivery. Open a
session notification and confirm it navigates to that session. Browser permission
alone does not prove a token is registered or a notification reached the device.
`push_sent` means FCM accepted at least one device delivery; it remains false when
there are no tokens or all devices fail. No live notifications were sent during
local verification.

## Grafana Cloud observability

See [OBSERVABILITY.md](OBSERVABILITY.md) for backend OTLP logs and metrics,
Secret Manager credentials, GitHub deployment variables and Grafana verification.
Export is disabled by default. Enabling it in production also selects
instance-based Cloud Run billing to support background telemetry export.

## Private notification worker

Release 3 adds a private Cloud Run execution mode in the same backend image. Set repository variable `NOTIFICATION_WORKER_ENABLED=true` only after configuring `NOTIFICATION_WORKER_RUNTIME_EMAIL` (existing runtime account with FCM permission) and `NOTIFICATION_SCHEDULER_EMAIL` (dedicated existing scheduler identity). Enable Cloud Scheduler API and give the deploying identity permission to deploy the private service, set that service's invocation IAM and manage scheduler jobs; authorize it to act as the two configured accounts. The scheduler service agent needs the normal OIDC token-minting permission. Do not grant project-wide invocation access.

The deployment script creates `rally-notifications` and its minute drain job plus a separate five-minute health job; PRs use independent `rally-notifications-pr-N` services, jobs, audiences and Neon databases, with delivery disabled and no API diagnostic allowlist. PR close deletes its scheduler before its worker. Worker deployments never mutate the public API service. Secret environment values go through a mode-600 temporary env file which is removed on exit. The worker does not migrate or expose member/admin/money routes.

The worker verifies Google's signed OIDC token, exact audience and verified scheduler email in addition to private Cloud Run IAM. An unauthenticated or member token cannot invoke it. Custom audiences permit the first deployment to start with a known audience before Cloud Run generates its URL: [Google Cloud custom audience documentation](https://docs.cloud.google.com/run/docs/configuring/custom-audiences).

Release 3 keeps the existing API reminder producer and uses the private worker only to drain a bounded durable batch. Production deployment and API startup reject enabled notifications while `NOTIFICATION_WORKER_ENABLED=false`. The supported preparatory deployment has `NOTIFICATIONS_DISABLED=true`; configure the two worker identities and Scheduler API/IAM, then set `NOTIFICATION_WORKER_ENABLED=true`. Its deployment must succeed before the new API revision is deployed. Enable delivery only after verifying the worker and both jobs. Queue delivery has no legacy fallback. Release 4 atomically records the 48-hour RSVP migration and scanner cutover; every Release 3 API revision checks that persisted marker on each legacy tick and then stops producing old reminders. The Release 4 private worker owns the replacement scan. The independent health job returns 503 when durable work is more than five minutes overdue, including when FCM initialization fails. Terminal failed, partial and uncertain drain outcomes emit ERROR aggregate logs even when the HTTP batch succeeds. Both jobs expose execution failures in Cloud Scheduler; configure operator alert routing for their failure logs before production activation. Health performs no delivery. Monitor these job failures; operational delivery stops remain effective. No exactly-once device arrival is promised.
