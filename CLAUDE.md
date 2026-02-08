# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

RallyUp is a badminton club management app with member registration, session scheduling, RSVP tracking, and notifications. All times use **Australia/Sydney** timezone.

## Development Commands

### Local Database
```bash
docker-compose up -d   # PostgreSQL 16 on localhost:5432 (user: badminton, pass: badminton123, db: badminton_club)
```

### Backend (Go + Gin)
```bash
cd backend
cp .env.example .env        # configure environment
go mod download
go run cmd/server/main.go   # starts on :8080
```

### Frontend (React + Vite)
```bash
cd frontend
cp .env.example .env        # configure environment
npm install
npm run dev                 # starts on :5173, proxies /api → localhost:8080
npm run build               # tsc -b && vite build
npm run lint                # eslint
```

### Docker (backend only)
```bash
cd backend
docker build -t rallyup-api .
# Multi-stage: golang:1.22-alpine → alpine:3.19, binary at ./server, port 8080
```

**No tests exist** in this project currently.

## Architecture

### Backend (`backend/`)

**Module:** `github.com/weekday-masters/backend` (Go 1.22, Gin framework, GORM ORM)

**Entry point:** `cmd/server/main.go` — initializes config, DB, services, router, and graceful shutdown.

**Middleware chain (applied in order):**
1. `middleware.CORS` — global, allows frontend origin
2. `middleware.AuthMiddleware` — validates Auth0 JWT against JWKS (cached 1h)
3. `middleware.RequireApproved()` — checks `membership_status = approved`
4. `middleware.RequireAdmin()` — checks `role = admin`

**Route group nesting:**
```
/api                    → public (auth/callback, club info)
/api [+AuthMiddleware]  → authenticated (user profile, notifications, push tokens)
/api [+RequireApproved] → approved members (sessions, RSVPs, member list)
/api/admin [+RequireAdmin] → admin (join requests, session CRUD, announcements, club settings)
```

**Service layer pattern:** Handlers → Services → GORM/DB. Services contain business logic; handlers do request parsing and response formatting. The global `database.DB` is used directly by services (no DI).

**Key business rules in services:**
- `RSVPService`: enforces 3-day deadline, prevents IN→OUT after deadline (unless admin), tracks `is_late_rsvp` and `added_by_admin` flags
- `SessionService`: calculates `max_players` from courts (1→6, 2→10, 3→16), generates recurring sessions, sets RSVP deadline at sessionDate - 3 days 23:59:59 Sydney time
- `UserService`: auto-promotes user matching `ADMIN_EMAIL` env var on first login
- `SchedulerService`: hourly cron sends 24h/12h session reminders and 6h deadline alerts
- `NotificationService`: FCM push + SendGrid email, both optional and independently initialized

**Models use GORM hooks** (`BeforeCreate`) for UUID generation. All PKs are UUIDs.

### Frontend (`frontend/`)

**Stack:** React 18, TypeScript, Vite, Tailwind CSS (cyan primary / amber secondary palette), PWA-enabled.

**Auth flow:** Auth0 with Google OAuth (PKCE). `AuthContext` wraps the app — on Auth0 authentication, calls `POST /api/auth/callback` to sync user with backend, stores JWT for API calls.

**Routing pattern:** `App.tsx` defines routes wrapped in `ProtectedRoute` which checks `isAuthenticated`, `isApproved`, and `isAdmin` from `AuthContext`. Unapproved users are redirected to `/pending`.

**API client:** `services/api.ts` — singleton Axios instance with Bearer token interceptor. All backend calls go through this.

**State management:** No global store. Uses `AuthContext` for user state and `useApi`/`useApiMutation` hooks for per-component data fetching.

**Vite dev proxy:** `/api` requests forward to `http://localhost:8080` in development.

## Deployment

CI/CD via GitHub Actions (`.github/workflows/deploy.yml`) on push to `main`:
1. Backend: Docker build → GCP Artifact Registry → Cloud Run (australia-southeast1)
2. Frontend: npm build (with Cloud Run URL injected as `VITE_API_URL`) → Firebase Hosting

All secrets are in GitHub repository secrets. See `DEPLOY.md` for manual deployment steps.
