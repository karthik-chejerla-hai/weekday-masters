---
name: setup-cloud-run
description: Set up Google Cloud Run resources for backend deployment. Use when rebranding the app or creating fresh GCP infrastructure (Artifact Registry, Cloud Run service), and optionally updating GitHub Actions workflows.
---

# Setup Google Cloud Run for Backend Deployment

This skill creates the necessary Google Cloud resources for deploying the backend to Cloud Run, and optionally updates GitHub Actions workflows to use the new resource names.

## Instructions

You are setting up Google Cloud Run infrastructure for the backend deployment. Follow these steps carefully:

### Step 1: Gather Information

Ask the user for the **new app name** using AskUserQuestion. This name will be used to derive:
- **Artifact Registry repository name**: the app name in lowercase, kebab-case (e.g., `my-app`)
- **Cloud Run service name**: the app name with `-api` suffix (e.g., `my-app-api`)
- **GCP region**: default to `australia-southeast1` but ask if they want a different region

Present the derived names and ask for confirmation before proceeding.

### Step 2: Verify Prerequisites

Run these checks and report results to the user:
```bash
gcloud auth list --filter=status:ACTIVE --format="value(account)"
gcloud config get-value project
```

If no active account or project, tell the user to run `gcloud auth login` and `gcloud config set project PROJECT_ID` first, then re-run this skill.

### Step 3: Enable Required APIs

Run these commands (they are idempotent):
```bash
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable artifactregistry.googleapis.com
```

### Step 4: Create Artifact Registry Repository

Check if the repository already exists first:
```bash
gcloud artifacts repositories describe REPO_NAME --location=REGION 2>/dev/null
```

If it doesn't exist, create it:
```bash
gcloud artifacts repositories create REPO_NAME \
  --repository-format=docker \
  --location=REGION \
  --description="Docker images for APP_NAME backend"
```

If it already exists, inform the user and skip creation.

### Step 5: Deploy Initial Cloud Run Service

Deploy a minimal Cloud Run service to establish it. Use the backend Dockerfile:
```bash
cd backend
gcloud run deploy SERVICE_NAME \
  --source . \
  --region REGION \
  --allow-unauthenticated \
  --set-env-vars "GIN_MODE=release" \
  --set-env-vars "TIMEZONE=Australia/Sydney"
```

After deployment, retrieve and display the service URL:
```bash
gcloud run services describe SERVICE_NAME \
  --region REGION \
  --format 'value(status.url)'
```

### Step 6: Ask About GitHub Actions Update

Ask the user (using AskUserQuestion) whether to update the GitHub Actions workflows to use the new resource names. The options should be:
1. **Yes, update all workflows** - Updates deploy.yml, preview-deploy.yml, and preview-cleanup.yml
2. **No, skip workflow updates** - Only the GCP resources were created

### Step 7: Update GitHub Actions (if confirmed)

If the user confirmed, update the following files:

**`.github/workflows/deploy.yml`:**
- `ARTIFACT_REGISTRY_REPO` env var → new repo name
- `BACKEND_SERVICE_NAME` env var → new service name
- `SENDGRID_FROM_NAME` → new app display name

**`.github/workflows/preview-deploy.yml`:**
- `ARTIFACT_REGISTRY_REPO` env var → new repo name
- `BACKEND_SERVICE_NAME` env var → new service name
- `SENDGRID_FROM_NAME` → new app display name

**`.github/workflows/preview-cleanup.yml`:**
- The hardcoded service name in the `gcloud run services update-traffic` command → new service name

### Step 8: Report GitHub Secrets to Verify

After all changes, inform the user about the GitHub repository secrets they should verify/update. List them in a clear table format:

| Secret | Purpose | Action Needed |
|--------|---------|---------------|
| `GCP_PROJECT_ID` | Google Cloud project ID | Verify it matches the current `gcloud config get-value project` |
| `GCP_SA_KEY` | Service account JSON key | Must have permissions for Artifact Registry and Cloud Run in this project |
| `FIREBASE_PROJECT_ID` | Firebase project | Verify if this needs updating for the rebrand |
| `FRONTEND_URL` | Frontend URL for CORS | Update if the frontend domain changed |
| `DATABASE_URL` | Neon PostgreSQL connection string | Usually unchanged unless using a new DB |
| `AUTH0_DOMAIN` | Auth0 tenant domain | Usually unchanged |
| `AUTH0_AUDIENCE` | Auth0 API identifier | Usually unchanged |
| `AUTH0_CLIENT_ID` | Auth0 client ID | Usually unchanged |
| `ADMIN_EMAIL` | Admin user email | Usually unchanged |
| `SENDGRID_API_KEY` | SendGrid API key | Usually unchanged |
| `SENDGRID_FROM_EMAIL` | Sender email address | May need updating for rebrand |
| `FIREBASE_API_KEY` | Firebase web API key | Verify if using new Firebase project |
| `FIREBASE_MESSAGING_SENDER_ID` | FCM sender ID | Verify if using new Firebase project |
| `FIREBASE_APP_ID` | Firebase app ID | Verify if using new Firebase project |
| `FIREBASE_VAPID_KEY` | Web push VAPID key | Verify if using new Firebase project |

Highlight which secrets are **most likely** to need updating based on whether a rebrand or project migration is happening.

### Important Notes

- Always show the user what commands will be run BEFORE executing them.
- If any GCP command fails, show the error and suggest troubleshooting steps.
- Do NOT modify any secrets or environment variables in Cloud Run beyond the initial deploy — those are managed via GitHub Actions secrets.
- The initial Cloud Run deploy in Step 5 may take a few minutes. Warn the user about this.
