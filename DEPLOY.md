# Deployment Guide

Deploy Weekday Masters to Google Cloud (free tier).

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Firebase       │────▶│  Cloud Run      │────▶│  Neon           │
│  Hosting        │     │  (Go Backend)   │     │  (PostgreSQL)   │
│  (React SPA)    │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

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
gcloud projects create weekday-masters --name="Weekday Masters"

# Set as default project
gcloud config set project weekday-masters

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
gcloud run deploy weekday-masters-api \
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
gcloud run services update weekday-masters-api \
  --region australia-southeast1 \
  --set-env-vars "DATABASE_URL=postgresql://user:pass@ep-xxx.neon.tech/dbname?sslmode=require" \
  --set-env-vars "AUTH0_DOMAIN=your-tenant.auth0.com" \
  --set-env-vars "AUTH0_AUDIENCE=https://your-api-identifier" \
  --set-env-vars "ADMIN_EMAIL=admin@example.com" \
  --set-env-vars "FRONTEND_URL=https://your-project.web.app"
```

### 2.3 Get Backend URL

```bash
gcloud run services describe weekday-masters-api \
  --region australia-southeast1 \
  --format 'value(status.url)'
```

Save this URL - you'll need it for the frontend.

---

## Step 3: Deploy Frontend to Firebase Hosting

### 3.1 Setup Firebase

```bash
# Login to Firebase
firebase login

# Create a new Firebase project (or link to existing GCP project)
firebase projects:create weekday-masters --display-name "Weekday Masters"

# Or use existing GCP project
firebase projects:addfirebase weekday-masters
```

### 3.2 Update Firebase Config

Edit `frontend/.firebaserc`:

```json
{
  "projects": {
    "default": "weekday-masters"
  }
}
```

### 3.3 Create Production Environment

```bash
cd frontend

# Create .env.production with your values
cat > .env.production << EOF
VITE_API_URL=https://weekday-masters-api-xxxxx-ts.a.run.app/api
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

Your app will be live at: `https://weekday-masters.web.app`

---

## Step 4: Update Auth0 Configuration

Add these URLs to your Auth0 application settings:

**Allowed Callback URLs:**
```
https://weekday-masters.web.app/callback
```

**Allowed Logout URLs:**
```
https://weekday-masters.web.app
```

**Allowed Web Origins:**
```
https://weekday-masters.web.app
```

---

## Step 5: Update Backend CORS

Update the `FRONTEND_URL` environment variable in Cloud Run:

```bash
gcloud run services update weekday-masters-api \
  --region australia-southeast1 \
  --update-env-vars "FRONTEND_URL=https://weekday-masters.web.app"
```

---

## Quick Deploy Scripts

### Deploy Backend

```bash
#!/bin/bash
cd backend
gcloud run deploy weekday-masters-api \
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

| Service | Your App (estimated) | Free Tier Limit |
|---------|---------------------|-----------------|
| Cloud Run | ~10K requests/month | 2M requests/month |
| Firebase Hosting | ~1GB transfer/month | 10GB/month |
| Neon PostgreSQL | ~100MB storage | 500MB storage |

You should stay well within free tier limits for a small club app.

---

## Troubleshooting

### Cold Starts
Cloud Run scales to zero. First request after idle may take 1-2 seconds.

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