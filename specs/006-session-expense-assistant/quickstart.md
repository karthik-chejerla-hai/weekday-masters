# Test issue 40 locally

## Start

Docker must be running. The runner uses Go 1.25.5 and Node 24.21.0 through
`mise` when installed. Otherwise it uses the `go` and `npm` commands on PATH.

From the repository root:

```bash
./scripts/local-issue40.sh
```

The runner starts a dedicated PostgreSQL 16 container, applies migrations, seeds
test data, and starts the API and Vite. Open http://localhost:5173 and sign in
with the Google address configured as `ADMIN_EMAIL` in `backend/.env`.
Auth0 must permit `http://localhost:5173` in its callback, logout, and web-origin
settings. Use the existing project Auth0 configuration in both `.env` files.
The normal verified-email admin registration flow also applies locally.

The local database is `rally_issue40` on `127.0.0.1:5434`, in container
`rally-issue40-db`. The runner overrides `DATABASE_URL`, `FRONTEND_URL`,
`VITE_API_URL`, `NOTIFICATIONS_DISABLED`, and `INVITATION_TEST_EMAILS_ENABLED`.
It cannot inherit the remote database URL in `backend/.env`. Notifications and
invitation test sends stay disabled. It does not change either `.env` file.

Setup logs are in `tmp/issue40/migrate.log` and `tmp/issue40/seed.log`.
The API log is in `tmp/issue40/api.log`. Press Ctrl+C to stop the app.
The database container remains running and retains local changes.

Use `./scripts/local-issue40.sh setup` for migrations and seed only.
Use `./scripts/local-issue40.sh api` and `./scripts/local-issue40.sh ui` in
separate terminals if you want separate server controls.

## Add the Groq key

Add this to the ignored `backend/.env` file:

```dotenv
GROQ_API_KEY=your-key-here
GROQ_MODEL=openai/gpt-oss-120b
GROQ_SPEECH_MODEL=whisper-large-v3-turbo
```

Restart the local runner after adding the key. The frontend never receives it.
Without a key, the expense form works and Ask Rally shows an unavailable message.

## Test the seeded session

Home shows the past `[seed] Thursday night (awaiting settlement)` session above
Next game. Its date is relative to the first seed run, so use the date shown in
the app. The seed output includes a direct link to Ask Rally for that session.

The four confirmed RSVPs are Priya, Marcus, Aiko, and Tom. Dev is an approved
member without an RSVP for this session. Sofia has a pending membership.
The seed also includes an upcoming session and a settled session for comparison.

1. Open the expense action on Home.
2. Type “We played three hours and used eight shuttles.” Or tap Record, speak,
   stop, review the transcript, and press Send message.
3. Review the session date, four players, hours, shuttle count, and shares.
4. Press Confirm expense once.
5. Open See the split. Check Money and Home for the updated balances and assets.

With the untouched fixture, the preview shows $83.00 court cost and $33.33 shuttle
cost, for $116.33 total. Three shares are $29.08 and one is $29.09. The session ID
controls which member receives the extra cent. Shuttle stock falls from 14 to 6.
Court credit falls from $60.00 to -$23.00. The warning requests a top-up before
the next standard two-hour booking, which costs $60.00. Negative court credit is
allowed by the existing ledger; negative shuttle stock is refused.

Use **Use the expense form** to test without Groq. Select 3 hours, enter 8 shuttles,
and press Preview expense. The same review and confirmation apply. Changing the
form clears its preview. Changes to rates, participants, or shuttle stock require
a new preview before saving. Repeated confirmation cannot charge the session twice.

For an early departure, select 3 hours and clear that player's checkbox under
**Who stayed for the extra hour**. Keep their main **Who pays** checkbox selected.
In Ask Rally, include “Jordan left after two hours” with the hours and actual
shuttle count. The review marks that player **2 hours only**. Shuttle cost is
allocated two thirds to the first two hours and one third to the extra hour.
Each time period is shared only by its players, with exact cent rounding.

Ask Rally also supports “How many shuttles do we have?”, “What is Marcus's
balance?”, and “Who has RSVP'd for the next session?”. Approved members can ask
these questions. Only admins can prepare and confirm expenses.

## Repeat a test

The seed is idempotent. It preserves expenses you record and does not reset the
ledger. For another trial, create another past local session and add RSVPs.
Developers can also use the existing admin endpoint
`POST /api/admin/settlements/{id}/reverse`. It restores stock and charges through
an append-only correction, then makes the session available again. Never point
the test harness at this demo database or a remote DB.

## Run the four attendance tests

Use only the dedicated scratch database, never the local demo or a remote target:

```bash
cd backend
TEST_DATABASE_URL='postgres://badminton:badminton123@127.0.0.1:5434/rally_issue40_test?sslmode=disable' \
  mise exec go@1.25.5 -- go test ./internal/services \
  -run '^TestExpenseRequestedScenariosAfterPreviousExpense$' -v -count=1
```

Each case sets up a previous expense and a shuttle purchase before settling the
new session. These tests need no Groq key. See [validation.md](validation.md) for
the four expected asset positions and the full verification results.

## Provider changes

`backend/internal/assistant/provider.go` defines separate `Planner` and
`Transcriber` interfaces. `groq.go` contains all Groq wire formats and HTTP calls.
Add a new adapter and select it in `cmd/server/main.go` to change providers.
The tools, permissions, expense calculator, review UI, and ledger stay the same.
A different Groq tool-capable model can use `GROQ_MODEL`; validate its tool results
before relying on it. GPT-OSS reasoning options are only sent for GPT-OSS models.

## Validation boundary

Automated checks use a fake provider and a separate scratch database. After key
setup, a separate live check passed Groq transcription of synthetic speech and
expense preparation against the local seed. It did not confirm the expense.
Real microphone capture and Google sign-in remain part of the owner's browser trial.
