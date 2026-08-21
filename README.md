<div align="center">

# 🏸 Rally

### Badminton Club Management, Simplified

[![Live App](https://img.shields.io/badge/🌐_Live_App-rally--club--app.web.app-06b6d4?style=for-the-badge)](https://rally-club-app.web.app)
[![API](https://img.shields.io/badge/🔗_API-Cloud_Run-4285F4?style=for-the-badge)](https://rally-club-api-ef7go5yk7q-ts.a.run.app)

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react&logoColor=black)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?style=flat-square&logo=typescript&logoColor=white)](https://typescriptlang.org)
[![Tailwind](https://img.shields.io/badge/Tailwind_CSS-3.4-06B6D4?style=flat-square&logo=tailwindcss&logoColor=white)](https://tailwindcss.com)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat-square&logo=postgresql&logoColor=white)](https://postgresql.org)
[![License](https://img.shields.io/badge/License-MIT-22c55e?style=flat-square)](LICENSE)

---

**Rally** is a full-stack web app for managing badminton clubs — handle member sign-ups,
schedule game days, track RSVPs, and send notifications, all from one place.

[Getting Started](#-getting-started) · [Features](#-features) · [Architecture](#-architecture) · [Deployment](#-deployment) · [API Reference](#-api-reference)

</div>

---

## ✨ Features

| | Feature | Description |
|---|---------|-------------|
| 🔐 | **Auth & Approval** | Google OAuth via Auth0 with admin approval workflow for new members |
| 📅 | **Session Scheduling** | Create one-off or recurring game days with court-based player limits |
| ✋ | **Smart RSVPs** | 3-day deadline enforcement, late RSVP tracking, admin overrides |
| ⏳ | **Waitlist** | Once a session is full, further RSVPs queue up and are auto-promoted (with a notification) when a spot frees |
| 🏟️ | **Court Calculator** | Auto max players: 1 court → 6, 2 courts → 10, 3 courts → 16 |
| 🔔 | **Push & Email** | FCM push notifications + SendGrid emails for reminders & announcements |
| 📱 | **PWA Ready** | Installable progressive web app with offline support |
| 🛡️ | **Role-Based Access** | Admin and Player roles with layered middleware |
| 🕐 | **Sydney Timezone** | All scheduling uses `Australia/Sydney` — no timezone confusion |

---

## 🏗️ Architecture

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│                  │     │                  │     │                  │
│  Firebase        │────▶│  Cloud Run       │────▶│  Neon            │
│  Hosting         │     │  (Go + Gin)      │     │  (PostgreSQL)    │
│  React SPA       │     │  REST API        │     │                  │
│                  │     │                  │     │                  │
└──────────────────┘     └──────────────────┘     └──────────────────┘
        │                        │
        │                        ├── Auth0 (JWT validation)
        │                        ├── FCM (push notifications)
        │                        └── SendGrid (email)
        │
        └── Auth0 (Google OAuth / PKCE)
```

<details>
<summary><b>Backend Stack</b></summary>

- **Framework:** [Gin](https://gin-gonic.com) — high-performance HTTP router
- **ORM:** [GORM](https://gorm.io) — with PostgreSQL driver
- **Auth:** Auth0 JWT with JWKS (cached 1h)
- **Scheduling:** [robfig/cron](https://github.com/robfig/cron) — session reminders at 24h, 12h, and 6h before deadline
- **Notifications:** Firebase Cloud Messaging + SendGrid (both optional)
- **Patterns:** Handler → Service → DB (no dependency injection, global `database.DB`)

</details>

<details>
<summary><b>Frontend Stack</b></summary>

- **Framework:** [React 18](https://react.dev) + [TypeScript](https://typescriptlang.org)
- **Build:** [Vite](https://vitejs.dev) with PWA plugin
- **Styling:** [Tailwind CSS](https://tailwindcss.com) (cyan primary / amber secondary)
- **Icons:** [Lucide React](https://lucide.dev)
- **Routing:** React Router v6 with protected routes
- **Auth:** `@auth0/auth0-react` (PKCE flow)
- **API:** Axios with Bearer token interceptor

</details>

---

## 🚀 Getting Started

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org)
- [Docker](https://docker.com) (for local database)
- [Auth0 account](https://auth0.com) with Google OAuth configured

### 1. Clone & set up the database

```bash
git clone https://github.com/your-org/weekday-masters.git
cd weekday-masters
docker-compose up -d   # PostgreSQL 16 on localhost:5432
```

### 2. Configure environment

```bash
cp backend/.env.example backend/.env       # edit with your Auth0 + DB settings
cp frontend/.env.example frontend/.env     # edit with your Auth0 + API settings
```

### 3. Start the backend

```bash
cd backend
go mod download
go run ./cmd/migrate        # apply the database schema (run after every schema change)
go run cmd/server/main.go   # → http://localhost:8080
```

> The server no longer migrates on startup — run `./cmd/migrate` explicitly.
> `docker-compose up` does this for you via the one-shot `migrate` service.

### 4. Start the frontend

```bash
cd frontend
npm install
npm run dev                 # → http://localhost:5173
```

> **Tip:** The Vite dev server proxies `/api` requests to `localhost:8080` automatically.

### 5. Run the tests (optional)

```bash
cd backend
go test ./...   # database-backed tests skip unless TEST_DATABASE_URL is set

# Run them for real against a scratch database:
docker run -d --name rally-test-db -p 5433:5432 \
  -e POSTGRES_USER=badminton -e POSTGRES_PASSWORD=badminton123 \
  -e POSTGRES_DB=badminton_club_test postgres:16-alpine
TEST_DATABASE_URL="postgres://badminton:badminton123@localhost:5433/badminton_club_test?sslmode=disable" \
  go test -race ./...
```

> ⚠️ Every test truncates the schema — never point `TEST_DATABASE_URL` at a real database.

---

## ⚙️ Environment Variables

<details>
<summary><b>Backend</b> (<code>backend/.env</code>)</summary>

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://badminton:badminton123@localhost:5432/badminton_club` |
| `AUTH0_DOMAIN` | Auth0 tenant domain | `your-tenant.auth0.com` |
| `AUTH0_AUDIENCE` | Auth0 API identifier | `https://your-api` |
| `ADMIN_EMAIL` | Auto-promoted admin email | `admin@example.com` |
| `FRONTEND_URL` | Frontend URL for CORS | `http://localhost:5173` |
| `SENDGRID_API_KEY` | *(optional)* SendGrid key | `SG.xxx` |
| `FIREBASE_PROJECT_ID` | *(optional)* For push notifications | `rally-club-app` |

</details>

<details>
<summary><b>Frontend</b> (<code>frontend/.env</code>)</summary>

| Variable | Description |
|----------|-------------|
| `VITE_API_URL` | Backend API URL (e.g. `http://localhost:8080/api`) |
| `VITE_AUTH0_DOMAIN` | Auth0 tenant domain |
| `VITE_AUTH0_CLIENT_ID` | Auth0 SPA client ID |
| `VITE_AUTH0_AUDIENCE` | Auth0 API identifier |
| `VITE_FIREBASE_*` | *(optional)* Firebase config for push notifications |

</details>

---

## 📡 API Reference

All endpoints are prefixed with `/api`. Authentication uses Bearer JWT tokens.

<details>
<summary><b>Public</b></summary>

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/club` | Get club info |

</details>

<details>
<summary><b>Authenticated</b> — requires valid JWT</summary>

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/auth/callback` | Sync user on login |
| `GET` | `/api/users/me` | Get current user profile |
| `PUT` | `/api/users/me` | Update profile |

</details>

<details>
<summary><b>Approved Members</b> — requires <code>membership_status = approved</code></summary>

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/users` | List all members |
| `GET` | `/api/sessions` | List sessions |
| `GET` | `/api/sessions/:id` | Get session details |
| `POST` | `/api/sessions/:id/rsvp` | Submit RSVP |
| `PUT` | `/api/sessions/:id/rsvp` | Update RSVP |
| `DELETE` | `/api/sessions/:id/rsvp` | Remove RSVP |

</details>

<details>
<summary><b>Admin</b> — requires <code>role = admin</code></summary>

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/admin/join-requests` | List pending join requests |
| `POST` | `/api/admin/join-requests/:id/approve` | Approve member |
| `POST` | `/api/admin/join-requests/:id/reject` | Reject member |
| `POST` | `/api/admin/sessions` | Create session |
| `PUT` | `/api/admin/sessions/:id` | Update session |
| `DELETE` | `/api/admin/sessions/:id` | Delete session |
| `POST` | `/api/admin/sessions/:id/rsvp/:userId` | Admin add RSVP |

</details>

---

## 🏸 RSVP Rules

```
┌─ Session Created ─────────────────────────────────────┐
│                                                       │
│  Players can freely RSVP IN or OUT                    │
│                                                       │
├─ 3 Days Before Session (23:59 AEST) ── DEADLINE ──────┤
│                                                       │
│  ❌  Players IN cannot switch to OUT                  │
│  ✅  Players OUT can still RSVP IN (marked as late)   │
│  👑  Admins can override any RSVP                     │
│                                                       │
├─ Session Day ─────────────────────────────────────────┤
│                                                       │
│  Capacity exceeded → first-come-first-served          │
│  Admin decides overflow situations manually            │
│                                                       │
└───────────────────────────────────────────────────────┘
```

---

## 🚢 Deployment

CI/CD is fully automated via **GitHub Actions** — push to `main` and everything deploys.

| Component | Platform | Region | URL |
|-----------|----------|--------|-----|
| **Frontend** | Firebase Hosting | Global CDN | [rally-club-app.web.app](https://rally-club-app.web.app) |
| **Backend** | Cloud Run | `australia-southeast1` | [rally-club-api-ef7go5yk7q-ts.a.run.app](https://rally-club-api-ef7go5yk7q-ts.a.run.app) |
| **Database** | Neon PostgreSQL | — | Managed |

```
git push origin main
    │
    ├──▶ Backend: Docker build → Artifact Registry → Cloud Run
    │
    └──▶ Frontend: npm build (with Cloud Run URL) → Firebase Hosting
```

> See **[DEPLOY.md](DEPLOY.md)** for manual deployment and first-time setup instructions.

---

## 📁 Project Structure

```
rally/
├── backend/
│   ├── cmd/server/main.go          # Entry point + graceful shutdown
│   └── internal/
│       ├── config/                  # Env var loading
│       ├── database/                # GORM connection
│       ├── handlers/                # HTTP request handlers
│       ├── middleware/              # Auth, CORS, role checks
│       ├── models/                  # GORM models (UUID PKs)
│       └── services/                # Business logic layer
├── frontend/
│   └── src/
│       ├── components/              # Reusable UI components
│       ├── context/                 # AuthContext (user state)
│       ├── hooks/                   # useApi, useApiMutation
│       ├── pages/                   # Route-level pages
│       ├── services/                # Axios API client
│       └── types/                   # TypeScript interfaces
├── infrastructure/                  # IaC (Terraform)
├── .github/workflows/deploy.yml    # CI/CD pipeline
├── docker-compose.yml              # Local PostgreSQL
└── DEPLOY.md                       # Deployment guide
```

---

## 📄 License

| Service  | Platform         | URL                                                  |
|----------|------------------|------------------------------------------------------|
| Frontend | Firebase Hosting | https://rally-club-app.web.app                       |
| Backend  | Cloud Run        | https://rally-club-api-ef7go5yk7q-ts.a.run.app      |
| Database | Neon PostgreSQL  | -                                                    |

---

<div align="center">

**Built with** ☕ **and** 🏸

[Go](https://go.dev) · [React](https://react.dev) · [Tailwind CSS](https://tailwindcss.com) · [Auth0](https://auth0.com) · [Google Cloud](https://cloud.google.com) · [Firebase](https://firebase.google.com) · [Neon](https://neon.tech)

</div>
