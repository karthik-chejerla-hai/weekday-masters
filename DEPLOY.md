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

Two things are deliberately **not** cleaned up, and both accumulate:

- **Cloud Run revisions.** `--remove-tags` takes the `pr-N` URL out of service
  but leaves the revision behind, untagged and serving no traffic. They are free
  but count against the per-service revision quota.
- **Artifact Registry images.** Every push to a PR builds and pushes a
  `pr-N-<sha>` image and nothing deletes it. This one costs storage. A registry
  cleanup policy is the fix, not a workflow step.

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