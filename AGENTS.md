# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

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
go run ./cmd/seed -env=local # optional: test members and sessions
go run cmd/server/main.go   # starts on :8080
```

### Seeding test data

`cmd/seed` fills a **non-production** database with six members covering the states that are
tedious to reach by hand — in credit, under the low-balance threshold, in debt, a pending join
request, an unclaimed invite — plus a settled session, a session still awaiting settlement, and
an upcoming one with RSVPs. It assumes the schema exists, so run `./cmd/migrate` first.

`-env` is required and has no production option. `-env=local` refuses any database that is not
on loopback; `-env=preview` additionally requires `SEED_ALLOW_PREVIEW=true`, which only
`preview-deploy.yml` sets. Preview environments are Neon branches of production, so they already
contain every real member — the guard that matters is not "is this database empty" but "is this
production", and the seed only ever writes rows it owns (`@seed.invalid` emails, `[seed]` titles)
and never modifies one it does not.

It is idempotent, so the preview workflow re-runs it on every push. There is deliberately no
reset flag: undoing the money would mean deleting ledger entries, and the ledger is append-only.
To start over, recreate the database or delete the PR's Neon branch.

**Seeding the club's real roll.** `-from <export.csv>` reads a Splitwise export and seeds the
actual members and their closing balances instead of the invented six:

```bash
go run ./cmd/seed -env=local -from ~/Downloads/export.csv \
  -bank 40000 -court-credit 15000 -shuttle-value 8740 -shuttle-units 21
```

The export is never committed — it carries real names and financial positions — so this is a
local-only mode and previews keep the synthetic roster. `LoadRoster` refuses any file that does
not reconcile: every transaction must sum to zero across the account columns, and the computed
column sums must equal the totals the export states. A fixture that silently disagrees with the
spreadsheet would be worse than none.

Members keep their real names but get `@seed.invalid` addresses, so a fixture row can never be
mailed or confused for the real person's account. Balances are posted as one opening-balance
transaction, and no session is settled — settling would charge the players and move them off the
exported figures. Without the asset flags the whole balance is recorded as bank, which balances
but claims the club holds no court credit and no shuttles.

### Importing real Splitwise history

Use `backend/cmd/import-splitwise`, not the fixture seed command. It imports reviewed
identity mappings, original transactions, historical charges and current assets
without sending invitations. Private inputs and reports stay in the ignored
`tmp/splitwise-import/` directory. Production requires an explicit target confirmation
and disabled notifications. See `specs/002-splitwise-history-import/quickstart.md`
for backup, rehearsal, import and verification steps. Invitations require separate
owner confirmation.

### Local expense assistant test

Run `./scripts/local-issue40.sh` from the repository root. It uses a dedicated
loopback PostgreSQL container on port 5434, applies migrations, seeds synthetic
members and a past session with four confirmed RSVPs, and starts both servers.
It overrides any remote `DATABASE_URL` from `.env` and disables notifications.
Add `GROQ_API_KEY` to `backend/.env` and restart for voice/text support. The form
does not need a key. See `specs/006-session-expense-assistant/quickstart.md`.

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
/api/admin [+RequireAdmin]  → admin (join requests, member CRUD, session CRUD, announcements,
                              club settings)
```

Register approved-only routes on the `approved` group, not `protected` — Gin binds the
handler chain at registration time, so using the wrong group silently skips the middleware.

**Service layer pattern:** Handlers → Services → GORM/DB. Services contain business logic; handlers do request parsing and response formatting. The global `database.DB` is used directly by services (no DI).

**Key business rules in services:**
- `RSVPService`: enforces 3-day deadline, prevents IN→OUT after deadline (unless admin), tracks `is_late_rsvp` and `added_by_admin` flags. Enforces session capacity inside a transaction that locks the session row (`SELECT ... FOR UPDATE`): an "in" request for a full session is stored as `waitlisted` instead, and freeing a confirmed spot auto-promotes the longest-waiting player and notifies them. Admins bypass the cap.
- `SessionService`: calculates `max_players` from courts (1→6, 2→10, 3→16), generates recurring sessions, sets RSVP deadline at sessionDate - 3 days 23:59:59 Sydney time.
  Cancellation locks the session and atomically stores its status, optional reason, an
  announcement and notification history for every approved member. Only the first request
  sends email/push through existing preferences and notification stops. Finished or settled
  sessions cannot be cancelled. Cancellation retains RSVPs and blocks edits, deletion and
  waitlist promotion. Other recurring dates stay intact. Announcements include Sydney dates
  and the next non-cancelled session, or an explicit no-next-session message.
- `UserService`: auto-promotes user matching `ADMIN_EMAIL` env var on first login — only when
  the email is **verified and sourced from Auth0**, never from the request body. Also owns admin
  member management: an invited member is a real, chargeable row created before their first
  sign-in, carrying `auth0_id = "invite:<uuid>"` in place of a subject; `RegisterUser` adopts that
  row when a verified email matches, so the invite is claimed rather than duplicated. Removal is a
  status change to `removed`, never a delete — the ledger is append-only and RSVPs and settlements
  reference the row — and is refused while the member has a non-zero balance, is the only admin, or
  is the caller. It cancels their upcoming RSVPs first, promoting from the waitlist
- `SchedulerService`: hourly cron sends 24h/12h session reminders and 6h deadline alerts
- `NotificationService`: FCM push only. Uses the configured Firebase project and Cloud Run Application Default Credentials, or explicit local credentials. Automatic email alerts are removed; SendGrid remains only for explicit invitations. The browser uses a dedicated push-worker scope and embeds public Firebase config during the build.
- `InvitationService`: admin-only preview and explicit send/resend for approved members who have not signed in. Preview and send share an escaped HTML template. `InvitationDelivery` records provider acceptance, failure, or uncertainty, never claims inbox delivery, and separates test attempts. Request IDs and a one-minute per-address cooldown prevent duplicate submissions. Member sends obey both notification stops. `INVITATION_TEST_EMAILS_ENABLED=true` permits only explicit test copies to the signed-in admin while those stops remain active. It defaults to false. No send occurs when adding a member or opening a preview.
- `LedgerService`: **the only writer of ledger entries.** Posts a transaction and its entries inside one DB transaction, locking the accounts it touches `FOR UPDATE` in `id` order, then asserts the club-position identity and rolls back if it does not come to zero. Balances are derived by aggregating entries — there is no cached balance column, so drift is impossible. Corrections are reversing transactions; nothing updates or deletes an entry.
- `SettlementService`: costs a played session into two bands (standard hours, optional extension), splits each band equally among only its own participants, and hands the resulting movements to `LedgerService`. Locks the session row like the RSVP capacity check. Refuses to drive shuttle stock negative, and refuses to settle a session twice
- `ExpenseService`: prepares and confirms actual-count expenses for 2 or 3 hours.
  It defaults to confirmed RSVPs and accepts a separate extra-hour subset. Actual
  shuttle stock is consumed once; its value splits 2:1 by time for three hours.
  Each band is shared only by its players. When everyone stays, the whole cost
  splits once to keep shares within one cent. It requires a reviewed fingerprint
  that includes attendance. Settlement locks affected accounts in ledger order before
  reading stock. Changed costs/stock reject confirmation; duplicates cannot post.
  `Settlement.ActualShuttles` is nullable so old estimated settlements retain their
  original meaning. Court top-up warnings use a standard two-hour booking.
- `GameService`: doubles-only results for started native sessions. Any approved member can
  save a reviewed result. Four distinct approved members and unequal integer scores
  from 0 to 99 are required. Create request IDs prevent duplicate retries. The
  recorder and approved admins can correct or void with a version check. Each
  write appends an immutable `GameRevision`; voided games stay visible but do not
  count in head-to-head records. Player comparisons count opposite sides only;
  exact-team comparisons ignore partner order. Statistics are derived in SQL.
  Referenced sessions cannot be deleted. Game data never changes money or RSVPs.
- `AssistantService`: a bounded tool loop for approved members. Read tools resolve
  sessions, members, balances and stock from services. Only admins receive
  `prepare_expense`; no assistant tool writes money. Explicit HTTP confirmation
  uses `ExpenseService`. Provider interfaces and Groq HTTP code live under
  `internal/assistant`. Audio and conversation history are not persisted by Rally.
  Approved members also receive `prepare_game`, which resolves exact names and
  rejects ambiguous names. It only creates a review form; Save uses `GameService`.

**Display names**: `User.DisplayName()` — the nickname a member chose, else their **first name**
— is what should reach a screen; the full `Name` is for identifying them. Members set their own
nickname on `/profile` and admins can correct it. Three places implement the same rule and must
stay in step: `models.User.DisplayName`, the SQL
`COALESCE(NULLIF(nickname, ''), split_part(name, ' ', 1))` used for balances and ordering, and the
frontend's `utils/members.displayName`. The ledger names player accounts by it (kept in sync by
`syncAccountName` on every edit), so one person cannot appear under two names at once.

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

### Frontend (`frontend/`)

**Stack:** React 18, TypeScript, Vite, Tailwind CSS (cyan primary / amber secondary palette), PWA-enabled.

**Auth flow:** Auth0 with Google OAuth (PKCE). `AuthContext` wraps the app — on Auth0 authentication, calls `POST /api/auth/callback` to sync user with backend, stores JWT for API calls. That endpoint requires a valid access token; the backend reads the subject from the token and, for first-time registrations, fetches the authoritative email from Auth0's `/userinfo`. Only display fields (`name`, `profile_picture`) are read from the request body.

**Admin screens:** `/admin` is the dashboard (join requests, club settings, announcements) and
`/admin/members` manages the roll — add, edit, remove and reinstate. The two are deliberately
separate: the dashboard handles people asking to join, the members page handles people who are (or
were) in the club. Each member has an expandable push settings panel. Its admin-only read endpoint
returns saved preferences and device registration metadata, never FCM tokens. A missing preference
record stays null. Registration dates do not establish current browser permission or delivery.

The **Not signed in** tab has invitation review controls: exact desktop/mobile email previews,
test copy to the current admin, and paused or available member sending. `/welcome` is the public
invitation landing page. It uses Google account selection and returns to a sign-in result screen.
`handleAuthRedirect` updates both browser history and React Router after OAuth. The invitation
URL contains no credentials and never grants membership; verified Auth0 email still claims the
existing member row, retaining its ledger and RSVPs.

**Money screens:** `/money` has Balances, Ledger, Club assets and Analytics tabs for all approved members. The three asset cards highlight dollar values, with shuttle count below its value. Asset purchase forms remain admin-only. `LedgerHandler.RegisterRoutes` binds the same access checks in the server and handler tests. The ledger defaults to the caller, with all-member and top-up-only filters and paged history. `/accounts/activity` groups each game into one expandable row before filtering and pagination; expansion retains all member shares and balances. Imported regular and extra-hour charges combine by import and play date. Native settlements remain distinct. Raw entry endpoints remain available. Running balances use full account history before filtering. Imported source titles and category/source metadata come from retained Splitwise records. Asset movement dates use Sydney time; shuttle audit dates come only from confirmed opening/import snapshots. `/sessions`
splits Upcoming from History; `/admin/sessions/:id/settle` is the settlement form, which
re-previews on every change so the figures on screen are the figures that will be posted. A
balance chip sits in the header on every screen. A personal spend chip beside it opens
`/money?tab=analytics`. `GET /accounts/me/spend` returns only the approved caller's
YTD and all-time session charges plus monthly YTD amounts, using Sydney play dates.
Native charge lines include hosted guests; imported sessions use reviewed gross
charges, not net credits. Deposits, opening balances and reversed charges do not
count. Totals cover recorded history through today and refresh on the existing
balance-change event.

**Game screens:** `/sessions/:id/games` provides voice review, a manual form, paged
results, correction controls and revision history. `/games` compares players or
exact doubles teams across sessions. Both require approved membership. Native
session history cards link to score entry; imported Splitwise cards do not.

**Routing pattern:** `App.tsx` defines routes wrapped in `ProtectedRoute` which checks `isAuthenticated`, `isApproved`, and `isAdmin` from `AuthContext`. Unapproved users are redirected to `/pending`.

**API client:** `services/api.ts` — singleton Axios instance with Bearer token interceptor. All backend calls go through this.

**State management:** No global store. Uses `AuthContext` for user state and `useApi`/`useApiMutation` hooks for per-component data fetching.

**Vite dev proxy:** `/api` requests forward to `http://localhost:8080` in development.

## Deployment

CI/CD via GitHub Actions (`.github/workflows/deploy.yml`) on push to `main`:
1. Backend: Docker build → GCP Artifact Registry → **run `./cmd/migrate` against `DATABASE_URL`** → Cloud Run (australia-southeast1)
2. Frontend: npm build (with Cloud Run URL injected as `VITE_API_URL`) → Firebase Hosting

All secrets are in GitHub repository secrets. See `DEPLOY.md` for manual deployment steps.
