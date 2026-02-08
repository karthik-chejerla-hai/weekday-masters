# Weekday Masters - Codebase Summary

A full-stack web application for managing a badminton club — handling member registration, session scheduling, RSVP tracking, and notifications. Designed as a single-club app for a group that plays on weekdays in the **Australia/Sydney** timezone.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Frontend** | React 18 + TypeScript + Vite + Tailwind CSS |
| **Backend** | Go 1.22 + Gin + GORM |
| **Database** | PostgreSQL (Neon cloud) |
| **Auth** | Auth0 with Google OAuth (PKCE flow) |
| **Push Notifications** | Firebase Cloud Messaging (FCM) |
| **Email** | SendGrid |
| **Hosting** | Firebase Hosting (frontend) + Cloud Run (backend) |
| **CI/CD** | GitHub Actions (deploy on push to `main`) |

---

## Core Features

### 1. Authentication & Membership

- Google sign-in via Auth0
- New users land in "pending" status; admins must approve them
- First admin is bootstrapped via `ADMIN_EMAIL` env var (auto-promoted on first login)
- Roles: `pending` → `player` → `admin`

### 2. Session / GameDay Management

- Admins create **one-off** or **recurring weekly** sessions
- Recurring sessions auto-generate up to 4 weeks ahead (cron-based refresh)
- Duration: 1-3 hours; 1-3 courts
- Court-based capacity: 1 court = 6 players, 2 courts = 10, 3 courts = 16
- Sessions can be cancelled with a reason

### 3. RSVP System

- Three-way: **In**, **Maybe**, **Out**
- **3-day deadline** before the session (e.g., Thursday 23:59 for a Sunday session)
- After deadline: players who said "In" **cannot change to Out**
- Admins can override deadlines and add late RSVPs
- First-come-first-served priority tracked via exact RSVP timestamps
- Overflow handling is manual (admin decides)

### 4. Notifications

- Push (Firebase FCM) + Email (SendGrid) — both optional
- 4 notification categories: session reminders, RSVP deadlines, waitlist updates, admin announcements
- Per-user granular preferences (push/email toggle per category)
- Scheduled cron jobs: 24h and 12h session reminders, 6h deadline reminders
- Admin can broadcast announcements to all members

### 5. Admin Panel

- Approve/reject join requests
- Create/update/cancel/delete sessions
- Manage player RSVPs (add players manually)
- Update club settings (name, venue)
- Send announcements

---

## Frontend Structure

### Pages (8 routes)

| Route | Page | Access |
|-------|------|--------|
| `/` | **Home** — landing page with login | Public |
| `/pending` | **PendingApproval** — waiting for admin approval | Authenticated |
| `/dashboard` | **Dashboard** — upcoming sessions, stats | Approved members |
| `/sessions` | **Sessions** — list all upcoming sessions | Approved members |
| `/sessions/:id` | **SessionDetail** — RSVP + player list | Approved members |
| `/profile` | **Profile** — edit phone, notification settings | Authenticated |
| `/admin` | **Admin** — club settings, join requests, announcements | Admin only |
| `/admin/sessions` | **AdminSessions** — create/manage sessions | Admin only |

### Components

| Category | Components | Purpose |
|----------|-----------|---------|
| **Layout** | `Layout.tsx`, `Header.tsx`, `Navigation.tsx` | App shell, top bar, bottom mobile nav |
| **Session** | `SessionCard.tsx` | Session card with expandable player list |
| **RSVP** | `RSVPButton.tsx`, `PlayerList.tsx` | Three-way RSVP control, detailed player listing |
| **UI** | `Badge.tsx`, `Avatar.tsx`, `Loading.tsx` | Status badges, user avatars, loading spinner |
| **Notifications** | `NotificationSettings.tsx` | Push/email preference toggles |

### Key Frontend Architecture

- **Auth**: `AuthContext.tsx` wraps the app, manages Auth0 login/logout, stores user state
- **API Client**: `api.ts` — Axios-based service with Bearer token interceptor
- **Hooks**: `useApi<T>()` for queries, `useApiMutation<T, P>()` for mutations
- **Firebase**: `firebase.ts` + `notifications.ts` — FCM setup, token management, foreground handlers
- **Routing**: `ProtectedRoute` component enforces authentication, approval, and admin checks
- **PWA**: Service worker with Workbox runtime caching for API calls (NetworkFirst strategy)
- **Styling**: Tailwind CSS with custom cyan primary / amber secondary palette; mobile-first responsive design

### TypeScript Types (`src/types/index.ts`)

```typescript
type UserRole = 'pending' | 'player' | 'admin'
type MembershipStatus = 'pending' | 'approved' | 'rejected'
type RSVPStatus = 'in' | 'out' | 'maybe'
type SessionStatus = 'open' | 'closed' | 'cancelled'

interface User { id, auth0_id, email, name, profile_picture, phone_number, role, is_player, membership_status, created_at, updated_at }
interface Club { id, name, venue_name, venue_address, created_at, updated_at }
interface Session { id, title, description, session_date, start_time, end_time, courts, max_players, rsvp_deadline, is_recurring, recurring_day_of_week, recurring_parent_id, status, cancellation_reason, created_by, created_at, updated_at, rsvps?, creator? }
interface RSVP { id, session_id, user_id, status, rsvp_timestamp, is_late_rsvp, added_by_admin, created_at, updated_at, user?, session? }
interface RSVPSummary { total_in, total_out, total_maybe, max_players, spots_left }
```

---

## Backend Structure

### Data Models (8 models)

| Model | Key Fields | Purpose |
|-------|-----------|---------|
| **Club** | name, venue_name, venue_address | Single club configuration |
| **User** | auth0_id, email, name, role, membership_status | Members with roles |
| **Session** | title, session_date, start/end_time, courts, max_players, rsvp_deadline, is_recurring, status | Game sessions |
| **RSVP** | session_id, user_id, status, rsvp_timestamp, is_late_rsvp, added_by_admin | Player attendance |
| **UserNotificationPreferences** | push/email toggles per category | Notification settings |
| **UserPushToken** | token, device_name | FCM device tokens |
| **Notification** | type, title, body, push_sent, email_sent, read_at | Notification history |
| **Announcement** | title, body, created_by, sent_at | Admin broadcasts |

All models use UUID primary keys and GORM hooks for auto-generation.

### Services (5 services)

| Service | Responsibility |
|---------|---------------|
| **UserService** | Create/update users, membership approval/rejection, role management, admin bootstrap |
| **SessionService** | Create/update/cancel sessions, recurring session generation, RSVP deadline calculation |
| **RSVPService** | Create/update/delete RSVPs, deadline enforcement, summary calculation, admin overrides |
| **NotificationService** | Push (FCM) + email (SendGrid) delivery, preferences management, token registration, bulk send |
| **SchedulerService** | Cron jobs — hourly session reminders (24h/12h) and deadline alerts (6h) |

### Handlers (6 handler groups)

| Handler | Endpoints |
|---------|----------|
| **AuthHandler** | `POST /api/auth/callback` |
| **UserHandler** | `GET/PUT /api/users/me`, `GET /api/users` |
| **SessionHandler** | `GET /api/sessions`, `GET /api/sessions/:id`, `GET /api/sessions/cancelled` |
| **RSVPHandler** | `POST/PUT/DELETE /api/sessions/:id/rsvp`, `GET /api/sessions/:id/rsvp/me` |
| **AdminHandler** | Join requests, sessions CRUD, club settings, announcements, admin RSVP |
| **NotificationHandler** | Preferences, push tokens, notification history |

### Middleware

| Middleware | Purpose |
|-----------|---------|
| **AuthMiddleware** | JWT validation against Auth0 JWKS (cached 1h), extracts user from DB |
| **RequireApproved** | Enforces `membership_status = approved` |
| **RequireAdmin** | Enforces `role = admin` |
| **CORS** | Allows frontend origin with credentials |

### API Endpoints (30+)

#### Public
- `GET /health` — Health check
- `GET /api/club` — Get club info

#### Authenticated
- `POST /api/auth/callback` — User registration/login sync
- `GET /api/users/me` — Current user profile
- `PUT /api/users/me` — Update profile (phone number)
- `GET/PUT /api/users/me/notifications` — Notification preferences
- `POST/DELETE /api/users/me/push-tokens` — FCM token management
- `GET /api/users/me/notifications/history` — Notification history
- `POST /api/notifications/:id/read` — Mark notification read

#### Approved Members
- `GET /api/users` — List members
- `GET /api/sessions` — List upcoming sessions
- `GET /api/sessions/cancelled` — List cancelled sessions
- `GET /api/sessions/:id` — Session detail with RSVP summary
- `POST/PUT/DELETE /api/sessions/:id/rsvp` — RSVP management
- `GET /api/sessions/:id/rsvp/me` — User's RSVP for session

#### Admin Only
- `GET /api/admin/join-requests` — Pending membership requests
- `POST /api/admin/join-requests/:id/approve` — Approve member
- `POST /api/admin/join-requests/:id/reject` — Reject member
- `PUT /api/admin/users/:id/role` — Update user role
- `POST /api/admin/sessions` — Create session
- `PUT /api/admin/sessions/:id` — Update session
- `DELETE /api/admin/sessions/:id` — Delete session
- `POST /api/admin/sessions/:id/cancel` — Cancel session with reason
- `POST /api/admin/sessions/:id/rsvp/:userId` — Admin add RSVP
- `PUT /api/admin/club` — Update club settings
- `POST /api/admin/announcements` — Send announcement

### Utilities (`internal/utils/time.go`)

All time operations use **Australia/Sydney** timezone:
- `CalculateRSVPDeadline()` — 3 days before session at 23:59:59
- `NowInSydney()` — Current time in Sydney
- `ParseDateInSydney()` — Parse date string in Sydney TZ
- `FormatDateForDisplay()` / `FormatTimeForDisplay()` — Human-readable formats

---

## Deployment & CI/CD

### Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Firebase       │────>│  Cloud Run      │────>│  Neon           │
│  Hosting        │     │  (Go Backend)   │     │  (PostgreSQL)   │
│  (React SPA)    │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

### GitHub Actions Pipeline (`.github/workflows/deploy.yml`)

Triggers on push to `main` or manual dispatch:

1. **Backend**: Docker multi-stage build → push to GCP Artifact Registry → deploy to Cloud Run (australia-southeast1)
2. **Frontend**: `npm ci && npm run build` with env vars injected → deploy to Firebase Hosting via `action-hosting-deploy`
3. Frontend deployment waits for backend (needs the Cloud Run URL for `VITE_API_URL`)

### Production URLs

| Service | Platform | URL |
|---------|----------|-----|
| Frontend | Firebase Hosting | https://weekday-masters.web.app |
| Backend | Cloud Run | https://weekday-masters-api-1011694988612.australia-southeast1.run.app |
| Database | Neon PostgreSQL | (managed) |

### Environment Variables

#### Backend (Cloud Run)
| Variable | Description |
|----------|-------------|
| `PORT` | Server port (default 8080) |
| `GIN_MODE` | `debug` or `release` |
| `DATABASE_URL` | PostgreSQL connection string |
| `AUTH0_DOMAIN` | Auth0 tenant domain |
| `AUTH0_AUDIENCE` | Auth0 API identifier |
| `ADMIN_EMAIL` | First admin email (auto-promoted) |
| `FRONTEND_URL` | Frontend URL for CORS |
| `TIMEZONE` | Default: `Australia/Sydney` |
| `FIREBASE_PROJECT_ID` | Firebase project (optional) |
| `FIREBASE_CREDENTIALS` | Firebase service account JSON (optional) |
| `SENDGRID_API_KEY` | SendGrid API key (optional) |
| `SENDGRID_FROM_EMAIL` | Sender email for notifications |
| `SENDGRID_FROM_NAME` | Sender display name |
| `SESSION_REMINDER_HOURS_24` | 24h reminder window (default 24) |
| `SESSION_REMINDER_HOURS_12` | 12h reminder window (default 12) |
| `DEADLINE_REMINDER_HOURS` | Deadline reminder window (default 6) |

#### Frontend (Build-time)
| Variable | Description |
|----------|-------------|
| `VITE_API_URL` | Backend API URL |
| `VITE_AUTH0_DOMAIN` | Auth0 tenant domain |
| `VITE_AUTH0_CLIENT_ID` | Auth0 SPA client ID |
| `VITE_AUTH0_AUDIENCE` | Auth0 API identifier |
| `VITE_FIREBASE_*` | Firebase config (7 variables for FCM) |

---

## RSVP Business Rules

1. Players must RSVP **3 days before** the session (deadline: Thursday 23:59:59 AEST/AEDT for Sunday sessions)
2. After deadline:
   - Players who RSVP'd **IN cannot change to OUT**
   - RSVP list is effectively frozen for regular users
   - Admin can still add late RSVPs (flagged as `is_late_rsvp`)
3. When capacity is exceeded: first-come-first-served based on `rsvp_timestamp`; admin decides overflow manually
4. When capacity is under-filled: admin can add players or accept late RSVPs

---

## Project File Structure

```
weekday-masters/
├── .github/workflows/deploy.yml    # CI/CD pipeline
├── backend/
│   ├── cmd/server/main.go          # Entry point, router setup, graceful shutdown
│   ├── Dockerfile                  # Multi-stage Go build
│   ├── go.mod / go.sum             # Go dependencies
│   ├── .env.example                # Environment template
│   └── internal/
│       ├── config/config.go        # Environment variable loading
│       ├── database/db.go          # PostgreSQL connection, migrations, seed
│       ├── handlers/
│       │   ├── admin.go            # Admin endpoints (sessions, members, club, announcements)
│       │   ├── auth.go             # Auth0 callback
│       │   ├── notifications.go    # Notification preferences, tokens, history
│       │   ├── rsvp.go             # RSVP CRUD
│       │   ├── sessions.go         # Session listing and detail
│       │   └── users.go            # User profile and member listing
│       ├── middleware/
│       │   ├── auth.go             # JWT validation, role/membership guards
│       │   └── cors.go             # CORS configuration
│       ├── models/
│       │   ├── club.go             # Club model
│       │   ├── notification.go     # Notification, preferences, push token, announcement models
│       │   ├── rsvp.go             # RSVP model
│       │   ├── session.go          # Session model with capacity calculation
│       │   └── user.go             # User model with roles and membership
│       ├── services/
│       │   ├── notification_service.go  # FCM + SendGrid delivery
│       │   ├── rsvp_service.go          # RSVP logic with deadline enforcement
│       │   ├── scheduler_service.go     # Cron jobs for reminders
│       │   ├── session_service.go       # Session CRUD and recurring generation
│       │   └── user_service.go          # User management and membership
│       └── utils/
│           └── time.go             # Sydney timezone utilities
├── frontend/
│   ├── package.json                # Dependencies and scripts
│   ├── vite.config.ts              # Vite + PWA + proxy config
│   ├── tailwind.config.js          # Custom color palette
│   ├── tsconfig.json               # TypeScript strict mode
│   ├── .env.example                # Dev environment template
│   ├── .env.production.example     # Prod environment template
│   └── src/
│       ├── main.tsx                # Auth0Provider + React mount
│       ├── App.tsx                 # BrowserRouter + ProtectedRoute
│       ├── index.css               # Tailwind + custom component classes
│       ├── context/
│       │   └── AuthContext.tsx      # Auth state, login/logout, user refresh
│       ├── hooks/
│       │   └── useApi.ts           # Generic query/mutation hooks
│       ├── services/
│       │   ├── api.ts              # Axios API client (30+ endpoints)
│       │   ├── firebase.ts         # Firebase app + FCM initialization
│       │   └── notifications.ts    # High-level notification service
│       ├── types/
│       │   └── index.ts            # All TypeScript interfaces
│       ├── components/
│       │   ├── layout/             # Layout, Header, Navigation
│       │   ├── rsvp/               # RSVPButton, PlayerList
│       │   ├── ui/                 # Badge, Avatar, Loading
│       │   └── notifications/      # NotificationSettings
│       └── pages/
│           ├── Home.tsx            # Landing page
│           ├── Dashboard.tsx       # Main dashboard
│           ├── Sessions.tsx        # Session list
│           ├── SessionDetail.tsx   # Single session view
│           ├── Profile.tsx         # User profile + notification settings
│           ├── PendingApproval.tsx  # Membership waiting page
│           ├── Admin.tsx           # Admin dashboard
│           └── AdminSessions.tsx   # Session management
├── docker-compose.yml              # Local PostgreSQL
├── README.md                       # Project overview
├── REQUIREMENTS.md                 # Business requirements
└── DEPLOY.md                       # Deployment guide
```
