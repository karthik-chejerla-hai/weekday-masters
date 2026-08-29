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

If port 5433 is already taken by another project, use any free port and match
`TEST_DATABASE_URL` to it.

`internal/services/main_test.go` owns the harness: it migrates the scratch database in
`TestMain`, and `requireDB(t)` truncates every table before each test — then reseeds the
club row and the four club accounts, which are schema-shaped configuration rather than test
data. Add any new table to `truncateAll` in `internal/testsupport/db.go` — so **never point
`TEST_DATABASE_URL` at a database you care about**. Tests skip (not fail) when it is
unset, keeping `go test ./...` green without a database. CI runs them against a
PostgreSQL service container.

Coverage includes the RSVP capacity and waitlist rules
(`internal/services/rsvp_service_test.go`), the ledger and its invariant
(`ledger_service_test.go`), settlement costing (`settlement_service_test.go`), and the
database-free money arithmetic (`internal/services/money`). Two concurrency tests assert
that row locks hold: one that the session lock prevents oversubscription, one that eight
simultaneous settle attempts produce exactly one settlement.

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
- `LedgerService`: **the only writer of ledger entries.** Posts a transaction and its entries inside one DB transaction, locking the accounts it touches `FOR UPDATE` in `id` order, then asserts the club-position identity and rolls back if it does not come to zero. Balances are derived by aggregating entries — there is no cached balance column, so drift is impossible. Corrections are reversing transactions; nothing updates or deletes an entry.
- `SettlementService`: costs a played session into two bands (standard hours, optional extension), splits each band equally among only its own participants, and hands the resulting movements to `LedgerService`. Locks the session row like the RSVP capacity check. Refuses to drive shuttle stock negative, and refuses to settle a session twice

**Models use GORM hooks** (`BeforeCreate`) for UUID generation. All PKs are UUIDs. `Session`
also has a `BeforeSave` hook that derives `starts_at`/`ends_at` from the date plus the
`HH:MM` strings, so create, update and recurring generation cannot diverge.

### Money

Read `.specify/memory/constitution.md` principles V–VII before touching any of this.

- **Integer cents everywhere.** No floats for currency, in models, services, JSON or the
  database. The frontend divides by 100 only in `formatCents`.
- **Sign convention:** amounts are stored the way the account itself reads — a player in
  credit is positive, an asset held is positive. This is deliberately *not* textbook
  double-entry (which would store liabilities negative so a naive `SUM` came to zero). The
  balancing rule is instead the identity `(bank + court credit + shuttle stock) − Σ player
  balances − surplus = 0`, evaluated with a `CASE` over `accounts.kind`.
- **`internal/services/money`** holds the arithmetic and touches no database, so the rules
  most worth testing exhaustively run without infrastructure: largest-remainder splitting
  (charges sum to the total exactly, at every headcount) and shuttle stock carried as a
  (value, units) pair so a $50 tube of twelve stays exact at 416.66… cents each. A per-unit
  price is never stored.
- **Club settings** (`base_hours`, rates, `shuttles_per_hour`, `low_balance_threshold_cents`)
  live on `Club` and are defaults only. Every settlement snapshots what it used, so changing
  a rate never rewrites a settled session.

### Migrating a club onto the ledger

`cmd/seed` loads a Splitwise group export as opening balances, for a club moving
onto Rally with money already owed. It reads the **TOTAL BALANCE** block rather
than replaying the transactions above it, and refuses to run unless the players
sum to the mirror held by the club's own Splitwise member.

Two rules make it safe to point at a real database:

- **The admin must have logged in first.** Opening balances are accepted exactly
  once, so seeding before their row exists would strand their balance with no
  second run to fix it. This is fatal, not a warning.
- **`-emails` decides whether a balance can ever reach its owner.** Rows seeded
  with a real address are claimed by `UserService.RegisterUser` the first time
  that person signs in with a matching *verified* Auth0 email; the claim is a
  compare-and-swap on the `seed|` subject prefix, so it happens once. Without
  `-emails`, rows get unroutable `@seed.invalid` addresses and can never be
  claimed — right for a scratch database, wrong for a real club.

Emails are unique, so a registration that cannot claim a row standing on its
address fails with `ErrEmailAlreadyRegistered` (409) rather than a constraint
violation.

### Frontend (`frontend/`)

**Stack:** React 18, TypeScript, Vite, Tailwind CSS (cyan primary / amber secondary palette), PWA-enabled.

**Auth flow:** Auth0 with Google OAuth (PKCE). `AuthContext` wraps the app — on Auth0 authentication, calls `POST /api/auth/callback` to sync user with backend, stores JWT for API calls. That endpoint requires a valid access token; the backend reads the subject from the token and, for first-time registrations, fetches the authoritative email from Auth0's `/userinfo`. Only display fields (`name`, `profile_picture`) are read from the request body.

**Money screens:** `/money` has Balances, My ledger and (admin) Club assets tabs; `/sessions`
splits Upcoming from History; `/admin/sessions/:id/settle` is the settlement form, which
re-previews on every change so the figures on screen are the figures that will be posted. A
balance chip sits in the header on every screen.

**Routing pattern:** `App.tsx` defines routes wrapped in `ProtectedRoute` which checks `isAuthenticated`, `isApproved`, and `isAdmin` from `AuthContext`. Unapproved users are redirected to `/pending`.

**API client:** `services/api.ts` — singleton Axios instance with Bearer token interceptor. All backend calls go through this.

**State management:** No global store. Uses `AuthContext` for user state and `useApi`/`useApiMutation` hooks for per-component data fetching.

**Vite dev proxy:** `/api` requests forward to `http://localhost:8080` in development.

## Deployment

CI/CD via GitHub Actions (`.github/workflows/deploy.yml`) on push to `main`:
1. Backend: Docker build → GCP Artifact Registry → **run `./cmd/migrate` against `DATABASE_URL`** → Cloud Run (australia-southeast1)
2. Frontend: npm build (with Cloud Run URL injected as `VITE_API_URL`) → Firebase Hosting

All secrets are in GitHub repository secrets. See `DEPLOY.md` for manual deployment steps.
