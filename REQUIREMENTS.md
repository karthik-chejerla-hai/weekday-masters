# Badminton Club Management App - Requirements

## Overview
A full-stack application to manage a badminton club, including member management, session scheduling, and RSVP tracking.

---

## Features

### 1. User Management & Membership
- Users can request to join the club
- Join requests must be approved by an admin before the user becomes a club member
- **Roles:**
  - **Admin**: Can manage sessions, approve join requests, manage players
  - **Standard User**: Regular club member
  - **Player**: A role for participation tracking (all admins and standard users are typically players, but this may change)
- First admin is configured via environment variable (email), auto-promoted on first login

### 2. Sessions / GameDays
- A club can have multiple sessions/gamedays
- Session duration: minimum 1 hour, maximum 3 hours
- Only admin users can create sessions
- Session types:
  - **Recurring weekly sessions** (auto-generated 2 weeks in advance)
  - **One-off sessions**
- Venue information stored at club level (not per-session)

### 3. Court & Player Capacity
Player limits based on available courts:
| Courts | Max Players |
|--------|-------------|
| 1      | 6           |
| 2      | 10          |
| 3      | 16          |

### 4. RSVP System
- Players must RSVP their availability **up to 3 days in advance**
- Example: For a Sunday session, RSVP deadline is Thursday end of day (23:59:59 AEST/AEDT)
- **After deadline:**
  - Players who RSVP'd IN cannot opt out
  - RSVP list is frozen
- **Capacity handling:**
  - If fewer players than required: Admin can add players or accept late RSVPs
  - If more players than max capacity: Admin decides manually who participates
- **Important:** Track exact timestamp of each RSVP for priority ordering (first-come, first-served)

---

## Technical Stack

### Frontend
- **Framework:** React with TypeScript
- **Design:** Mobile-first, responsive, modern UI
- **Styling:** Beautiful colors and icons (Tailwind CSS + Lucide icons)

### Backend
- **Language:** Go (Golang)
- **Framework:** Gin
- **Architecture:** Backend for Frontend (BFF) pattern
- **Database:** PostgreSQL

### Authentication
- **Provider:** Auth0 (already configured)
- **Flow:** OAuth 2.0 Authorization Code with PKCE
- **Identity Provider:** Google (users sign in with personal Google accounts)
- **User Data Retrieved:**
  - Name
  - Email
  - Profile Picture
  - Phone Number (optional, collected in profile settings)

---

## Configuration

### Environment Variables
| Variable | Description |
|----------|-------------|
| `ADMIN_EMAIL` | Email of the first admin user (auto-promoted on first login) |
| `DATABASE_URL` | PostgreSQL connection string |
| `AUTH0_DOMAIN` | Auth0 tenant domain |
| `AUTH0_CLIENT_ID` | Auth0 application client ID |
| `AUTH0_AUDIENCE` | Auth0 API audience |
| `TIMEZONE` | Default: `Australia/Sydney` |

### Time Zone
- All session times and RSVP deadlines use **Australia/Sydney (AEST/AEDT)**
- Frontend displays times in this timezone

---

## Decisions Made

| Question | Decision |
|----------|----------|
| Database | PostgreSQL |
| Phone Number Collection | Optional field in profile settings |
| Admin Bootstrap | First admin email from env var, auto-promoted on first login |
| Club Scope | Single club only |
| Notifications | Deferred to later phase (MVP without email notifications) |
| Overflow Handling | Admin decides manually (no automatic waitlist) |
| Recurring Session Generation | 2 weeks in advance |
| Auth0 Setup | Already configured by user |
| Venue Information | Club-level only |
| Time Zone | Australia/Sydney (AEST/AEDT) |

---

## Out of Scope (Future Phases)
- Email/push notifications
- Multi-club support
- Automatic waitlist management
- Payment integration
- Player statistics/leaderboards

---

## Version History
- **v1.0** - Initial requirements (2025-12-26)
- **v1.1** - Added clarified decisions and configuration details (2025-12-26)
