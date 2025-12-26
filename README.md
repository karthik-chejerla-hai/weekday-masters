# Weekday Masters - Badminton Club Management App

A full-stack application to manage a badminton club, including member management, session scheduling, and RSVP tracking.

## Tech Stack

- **Frontend**: React + TypeScript + Vite + Tailwind CSS
- **Backend**: Go + Gin + GORM
- **Database**: PostgreSQL
- **Authentication**: Auth0 with Google OAuth

## Features

- User registration with admin approval workflow
- Role-based access (Admin, Player)
- Session/GameDay management (one-off and recurring)
- RSVP system with 3-day deadline enforcement
- Court-based player limits (1 court = 6 players, 2 courts = 10, 3 courts = 16)
- Mobile-first responsive design

## Prerequisites

- Go 1.22+
- Node.js 18+
- Docker & Docker Compose
- Auth0 account with Google OAuth configured

## Quick Start

### 1. Start the Database

```bash
docker-compose up -d
```

### 2. Configure Environment Variables

**Backend** (`backend/.env`):
```bash
cp backend/.env.example backend/.env
# Edit backend/.env with your settings
```

**Frontend** (`frontend/.env`):
```bash
cp frontend/.env.example frontend/.env
# Edit frontend/.env with your Auth0 settings
```

### 3. Auth0 Configuration

1. Create a Single Page Application in Auth0
2. Add `http://localhost:5173` to:
   - Allowed Callback URLs
   - Allowed Logout URLs
   - Allowed Web Origins
3. Enable Google social connection
4. Create an API in Auth0 with your audience identifier
5. Copy the Domain, Client ID, and Audience to your `.env` files

### 4. Run the Backend

```bash
cd backend
go mod download
go run cmd/server/main.go
```

The API will be available at `http://localhost:8080`

### 5. Run the Frontend

```bash
cd frontend
npm install
npm run dev
```

The app will be available at `http://localhost:5173`

## Environment Variables

### Backend

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@localhost:5432/db` |
| `AUTH0_DOMAIN` | Auth0 tenant domain | `your-tenant.auth0.com` |
| `AUTH0_AUDIENCE` | Auth0 API identifier | `https://your-api` |
| `ADMIN_EMAIL` | Email of first admin (auto-promoted) | `admin@example.com` |
| `FRONTEND_URL` | Frontend URL for CORS | `http://localhost:5173` |

### Frontend

| Variable | Description |
|----------|-------------|
| `VITE_API_URL` | Backend API URL |
| `VITE_AUTH0_DOMAIN` | Auth0 tenant domain |
| `VITE_AUTH0_CLIENT_ID` | Auth0 SPA client ID |
| `VITE_AUTH0_AUDIENCE` | Auth0 API identifier |

## API Endpoints

### Public
- `GET /api/club` - Get club info

### Authenticated
- `POST /api/auth/callback` - User registration/login
- `GET /api/users/me` - Get current user
- `PUT /api/users/me` - Update profile
- `GET /api/users` - List members
- `GET /api/sessions` - List sessions
- `GET /api/sessions/:id` - Get session details
- `POST /api/sessions/:id/rsvp` - Submit RSVP
- `PUT /api/sessions/:id/rsvp` - Update RSVP
- `DELETE /api/sessions/:id/rsvp` - Remove RSVP

### Admin Only
- `GET /api/admin/join-requests` - List pending requests
- `POST /api/admin/join-requests/:id/approve` - Approve request
- `POST /api/admin/join-requests/:id/reject` - Reject request
- `POST /api/admin/sessions` - Create session
- `PUT /api/admin/sessions/:id` - Update session
- `DELETE /api/admin/sessions/:id` - Delete session
- `POST /api/admin/sessions/:id/rsvp/:userId` - Admin add RSVP

## Project Structure

```
weekday-masters/
├── backend/
│   ├── cmd/server/main.go       # Entry point
│   ├── internal/
│   │   ├── config/              # Configuration
│   │   ├── database/            # DB connection
│   │   ├── handlers/            # HTTP handlers
│   │   ├── middleware/          # Auth middleware
│   │   ├── models/              # Data models
│   │   ├── services/            # Business logic
│   │   └── utils/               # Utilities
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/          # React components
│   │   ├── context/             # Auth context
│   │   ├── hooks/               # Custom hooks
│   │   ├── pages/               # Page components
│   │   ├── services/            # API client
│   │   └── types/               # TypeScript types
│   └── package.json
├── docker-compose.yml
└── README.md
```

## RSVP Rules

1. Players must RSVP 3 days before the session (deadline: Thursday 23:59 for Sunday sessions)
2. After deadline:
   - Players who RSVP'd IN cannot change to OUT
   - Admin can still add late RSVPs
3. When capacity is exceeded, first-come-first-served based on RSVP timestamp
4. Admin decides overflow situations manually

## License

MIT
