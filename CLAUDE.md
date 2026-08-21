# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Rally is a badminton club management app with member registration, session scheduling, RSVP tracking, and notifications. All times use **Australia/Sydney** timezone.

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
go run ./cmd/migrate        # apply schema — NOT run by the server
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
docker build -t rally-api .
# Multi-stage: golang:1.22-alpine → alpine:3.19, binaries ./server and ./migrate, port 8080
```

### Tests

```bash
cd backend
go test ./...              # database-backed tests skip without TEST_DATABASE_URL

# To actually run them, point at a scratch database:
docker run -d --name rally-test-db -p 5433:5432 \
  -e POSTGRES_USER=badminton -e POSTGRES_PASSWORD=badminton123 \
  -e POSTGRES_DB=badminton_club_test postgres:16-alpine
TEST_DATABASE_URL="postgres://badminton:badminton123@localhost:5433/badminton_club_test?sslmode=disable" \
  go test -race ./...
```

`internal/services/main_test.go` owns the harness: it migrates the scratch database in
`TestMain`, and `requireDB(t)` truncates every table before each test — so **never point
`TEST_DATABASE_URL` at a database you care about**. Tests skip (not fail) when it is
unset, keeping `go test ./...` green without a database. CI runs them against a
PostgreSQL service container.

Coverage today is the RSVP capacity and waitlist rules (`internal/services/rsvp_service_test.go`),
including a concurrency test that asserts the session row lock prevents oversubscription.
The frontend has no tests.

## Architecture

### Backend (`backend/`)

**Module:** `github.com/weekday-masters/backend` (Go 1.22, Gin framework, GORM ORM)

**Entry points:**
- `cmd/server/main.go` — initializes config, DB, services, router, and graceful shutdown.
- `cmd/migrate/main.go` — applies the schema (`AutoMigrate` + club seed) and exits. Migrations are deliberately **not** run by the server: doing so races across Cloud Run revisions. CI runs this as an explicit step before deploying, and `docker-compose` runs it as a one-shot service.

**Middleware chain (applied in order):**
1. `middleware.CORS` — global, allows frontend origin
2. `middleware.AuthMiddleware` — validates Auth0 JWT against JWKS (cached 1h)
3. `middleware.RequireApproved()` — checks `membership_status = approved`
4. `middleware.RequireAdmin()` — checks `role = admin`

**Route group nesting:**
```
/api                        → public (club info, OpenAPI docs)
/api [+RequireValidToken]   → registration (auth/callback) — valid JWT, user row may not exist yet
/api [+AuthMiddleware]      → authenticated (user profile, notifications, push tokens)
/api [+RequireApproved]     → approved members (sessions, RSVPs, member list)
/api/admin [+RequireAdmin]  → admin (join requests, session CRUD, announcements, club settings)
```

Register approved-only routes on the `approved` group, not `protected` — Gin binds the
handler chain at registration time, so using the wrong group silently skips the middleware.

**Service layer pattern:** Handlers → Services → GORM/DB. Services contain business logic; handlers do request parsing and response formatting. The global `database.DB` is used directly by services (no DI).

**Key business rules in services:**
- `RSVPService`: enforces 3-day deadline, prevents IN→OUT after deadline (unless admin), tracks `is_late_rsvp` and `added_by_admin` flags. Enforces session capacity inside a transaction that locks the session row (`SELECT ... FOR UPDATE`): an "in" request for a full session is stored as `waitlisted` instead, and freeing a confirmed spot auto-promotes the longest-waiting player and notifies them. Admins bypass the cap.
- `SessionService`: calculates `max_players` from courts (1→6, 2→10, 3→16), generates recurring sessions, sets RSVP deadline at sessionDate - 3 days 23:59:59 Sydney time
- `UserService`: auto-promotes user matching `ADMIN_EMAIL` env var on first login — only when the email is **verified and sourced from Auth0**, never from the request body
- `SchedulerService`: hourly cron sends 24h/12h session reminders and 6h deadline alerts
- `NotificationService`: FCM push + SendGrid email, both optional and independently initialized

**Models use GORM hooks** (`BeforeCreate`) for UUID generation. All PKs are UUIDs.

### Frontend (`frontend/`)

**Stack:** React 18, TypeScript, Vite, Tailwind CSS (cyan primary / amber secondary palette), PWA-enabled.

**Auth flow:** Auth0 with Google OAuth (PKCE). `AuthContext` wraps the app — on Auth0 authentication, calls `POST /api/auth/callback` to sync user with backend, stores JWT for API calls. That endpoint requires a valid access token; the backend reads the subject from the token and, for first-time registrations, fetches the authoritative email from Auth0's `/userinfo`. Only display fields (`name`, `profile_picture`) are read from the request body.

**Routing pattern:** `App.tsx` defines routes wrapped in `ProtectedRoute` which checks `isAuthenticated`, `isApproved`, and `isAdmin` from `AuthContext`. Unapproved users are redirected to `/pending`.

**API client:** `services/api.ts` — singleton Axios instance with Bearer token interceptor. All backend calls go through this.

**State management:** No global store. Uses `AuthContext` for user state and `useApi`/`useApiMutation` hooks for per-component data fetching.

**Vite dev proxy:** `/api` requests forward to `http://localhost:8080` in development.

## Deployment

CI/CD via GitHub Actions (`.github/workflows/deploy.yml`) on push to `main`:
1. Backend: Docker build → GCP Artifact Registry → **run `./cmd/migrate` against `DATABASE_URL`** → Cloud Run (australia-southeast1)
2. Frontend: npm build (with Cloud Run URL injected as `VITE_API_URL`) → Firebase Hosting

All secrets are in GitHub repository secrets. See `DEPLOY.md` for manual deployment steps.
