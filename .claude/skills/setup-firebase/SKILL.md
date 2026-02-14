---
name: setup-firebase
description: Set up Firebase Hosting for frontend deployment. Use when rebranding the app or creating fresh Firebase infrastructure, and optionally updating GitHub Actions workflows and local config.
---

# Setup Firebase Hosting for Frontend Deployment

This skill creates the necessary Firebase resources for deploying the frontend to Firebase Hosting, updates local configuration, and optionally updates GitHub Actions workflows.

## Instructions

You are setting up Firebase Hosting infrastructure for the frontend deployment. Follow these steps carefully:

### Step 1: Gather Information

Ask the user for the **new app/project name** using AskUserQuestion. This name will be used to derive:
- **Firebase project ID**: the name in lowercase, kebab-case (e.g., `my-app`) — this becomes the project ID
- **Firebase display name**: title-cased version (e.g., `My App`)
- **Hosting URL**: `https://<project-id>.web.app`

Also ask whether this is:
1. **A new Firebase project** — will create a new project and enable Firebase
2. **An existing GCP project** — will add Firebase to an existing GCP project
3. **An existing Firebase project** — will just link to it (no creation needed)

Present the derived names and ask for confirmation before proceeding.

### Step 2: Verify Prerequisites

Run these checks and report results to the user:
```bash
firebase login:list 2>/dev/null || echo "NOT_LOGGED_IN"
```

If not logged in, tell the user to run `firebase login` first, then re-run this skill.

Also check that the Firebase CLI is installed:
```bash
which firebase || echo "NOT_INSTALLED"
```

If not installed, tell the user to run `npm install -g firebase-tools` first.

### Step 3: Create or Link Firebase Project

Based on the user's choice in Step 1:

**Option A — New Firebase project:**
```bash
firebase projects:create PROJECT_ID --display-name "DISPLAY_NAME"
```

**Option B — Add Firebase to existing GCP project:**
```bash
firebase projects:addfirebase PROJECT_ID
```

**Option C — Existing Firebase project:**
Skip creation. Verify it exists:
```bash
firebase projects:list 2>&1 | grep PROJECT_ID
```

If any command fails, show the error. Common issues:
- Project ID already taken → suggest a different ID
- Billing not enabled → direct user to GCP console
- Permissions issue → user needs Firebase Editor or Owner role

### Step 4: Enable Firebase Hosting

Initialize or verify hosting is enabled for the project:
```bash
firebase target:apply hosting PROJECT_ID PROJECT_ID --project PROJECT_ID 2>/dev/null
```

This step may not be strictly necessary if the project was just created, but ensures hosting is properly targeted.

### Step 5: Update Local Firebase Configuration

Update `frontend/.firebaserc` to point to the new project:

The file should contain:
```json
{
  "projects": {
    "default": "PROJECT_ID"
  }
}
```

Use the Edit tool to update the `"default"` value in the existing file.

**Do NOT modify `frontend/firebase.json`** — the hosting config (public dir, rewrites, headers) is project-agnostic.

### Step 6: Do a Test Deploy (Optional)

Ask the user (using AskUserQuestion) whether to do a test deploy now:
1. **Yes, deploy now** — will build and deploy the frontend
2. **No, skip test deploy** — just configure, deploy later via CI

If yes:
```bash
cd frontend
npm ci
npm run build
firebase deploy --only hosting --project PROJECT_ID
```

After deployment, display the live URL: `https://PROJECT_ID.web.app`

### Step 7: Ask About GitHub Actions Update

Ask the user (using AskUserQuestion) whether to update the GitHub Actions workflows to reference the new Firebase project. The options should be:
1. **Yes, update all workflows** — Updates deploy.yml and preview-deploy.yml
2. **No, skip workflow updates** — Only local config was updated

### Step 8: Update GitHub Actions (if confirmed)

If the user confirmed, note that **the workflows use `${{ secrets.FIREBASE_PROJECT_ID }}`** for the project ID — so the workflow files themselves don't need editing for the project ID. The change happens in GitHub secrets.

However, check and update any **hardcoded references** in the workflows. Currently the workflows use secrets consistently, so inform the user:

> The workflow files (`deploy.yml`, `preview-deploy.yml`) already reference `${{ secrets.FIREBASE_PROJECT_ID }}` — no file changes needed. You just need to update the GitHub secret value.

### Step 9: Report GitHub Secrets to Update

After all changes, inform the user about the GitHub repository secrets they need to update. List them clearly:

**Must update for the new Firebase project:**

| Secret | Purpose | Action |
|--------|---------|--------|
| `FIREBASE_PROJECT_ID` | Firebase project ID used in deploy and preview workflows | **Update** to `PROJECT_ID` |
| `GCP_SA_KEY` | Service account JSON key | **Update** — must have Firebase Hosting Admin permissions in the new project. Generate via: `gcloud iam service-accounts keys create` or Firebase console |
| `FIREBASE_API_KEY` | Firebase web API key (used in frontend build) | **Update** — get from Firebase console → Project Settings → General → Web API Key |
| `FIREBASE_MESSAGING_SENDER_ID` | FCM sender ID for push notifications | **Update** — get from Firebase console → Project Settings → Cloud Messaging |
| `FIREBASE_APP_ID` | Firebase app ID | **Update** — get from Firebase console → Project Settings → General → Your apps → App ID |
| `FIREBASE_VAPID_KEY` | VAPID key for web push | **Update** — get from Firebase console → Project Settings → Cloud Messaging → Web Push certificates |

**Likely unchanged (not Firebase-specific):**

| Secret | Purpose | Action |
|--------|---------|--------|
| `AUTH0_DOMAIN` | Auth0 tenant | Usually unchanged |
| `AUTH0_CLIENT_ID` | Auth0 SPA client | Usually unchanged |
| `AUTH0_AUDIENCE` | Auth0 API identifier | Usually unchanged |
| `FRONTEND_URL` | Backend CORS origin | **Update** if domain changed (e.g., `https://PROJECT_ID.web.app`) |

### Step 10: Remind About Auth0 Configuration

Remind the user that if the hosting URL changed, they need to update Auth0 application settings:

- **Allowed Callback URLs**: `https://PROJECT_ID.web.app/callback`
- **Allowed Logout URLs**: `https://PROJECT_ID.web.app`
- **Allowed Web Origins**: `https://PROJECT_ID.web.app`

And update the Cloud Run `FRONTEND_URL` env var (via GitHub secret) so CORS allows the new origin.

### Important Notes

- Always show the user what commands will be run BEFORE executing them.
- If any Firebase command fails, show the error and suggest troubleshooting steps.
- The `firebase.json` hosting configuration (public dir, rewrites, cache headers) is project-agnostic — do NOT modify it.
- Firebase project IDs are globally unique and permanent — they cannot be renamed or deleted easily. Help the user choose carefully.
