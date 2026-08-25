# Quickstart: Validating the Club Ledger

How to run this feature locally and prove it works. Entity details are in
[data-model.md](./data-model.md); endpoint shapes are in [contracts/](./contracts/).

---

## Prerequisites

```bash
docker-compose up -d          # PostgreSQL 16 on :5432
cd backend && cp .env.example .env
go mod download
go run ./cmd/migrate          # applies schema, seeds club settings, backfills session times
go run cmd/server/main.go     # :8080

cd ../frontend && cp .env.example .env
npm install && npm run dev    # :5173
```

A scratch database for the test suite, per CLAUDE.md:

```bash
docker run -d --name rally-test-db -p 5433:5432 \
  -e POSTGRES_USER=badminton -e POSTGRES_PASSWORD=badminton123 \
  -e POSTGRES_DB=badminton_club_test postgres:16-alpine
```

> `requireDB(t)` truncates every table before each test. Never point `TEST_DATABASE_URL`
> at a database whose contents matter.

---

## Running the tests

```bash
cd backend
go test ./...                                   # DB-backed tests skip; must stay green

TEST_DATABASE_URL="postgres://badminton:badminton123@localhost:5433/badminton_club_test?sslmode=disable" \
  go test -race ./...                           # full suite, including concurrency

go test ./internal/services/money/...           # pure arithmetic, no database needed

cd ../frontend && npm test && npm run lint && npm run build
```

The `money` package is deliberately database-free so the split and stock-costing rules can
be exhaustively tested without infrastructure (Principle VIII).

---

## Validation scenarios

Each maps to acceptance scenarios in [spec.md](./spec.md). Run against a fresh database.

### 1 — Ledger foundation (User Story 1)

1. Approve two members. Confirm each has a player account at zero.
2. Record a $50 top-up for one. Their balance reads $50.00; the bank reads $50.00.
3. Open the money section as the *other* member. Both balances are visible (FR-024).
4. Check `GET /api/admin/position/integrity` → `balanced: true`.
5. Reverse the top-up. Both entries remain visible; the balance returns to zero.

**Proves**: accounts exist, entries are additive, corrections are reversals not edits, and
the invariant holds across all of it.

### 2 — Settlement arithmetic (User Story 2)

Set up the worked example from [data-model.md](./data-model.md): stock of 24 units at
$100.00, six heads on the base band (one of them a guest), four staying for the extra hour.

1. Preview the settlement. Base band totals $101.67, extra band $43.83.
2. Confirm three base lines are $16.95 and three are $16.94 — the odd cents distributed,
   not absorbed.
3. Settle. Court credit falls $83.00; stock falls $62.50 and 15 units; charges total
   $145.50, matching consumption exactly.
4. Stock afterwards reads 9 units at $37.50 — still $4.1667 a shuttle.
5. Attempt to settle again → `409 session_already_settled`.
6. Reverse the settlement, then settle again with a different set of lines. Balances unwind
   and re-apply correctly.

**Proves**: band splitting, largest-remainder allocation, weighted-average stock costing,
one-way-door settlement, and reversal.

### 3 — The awkward cases

| Scenario | Expected |
|---|---|
| Comp a player | Everyone else's charge is unchanged; the comped line is $0.00; surplus goes negative by exactly that share |
| Settle needing more shuttles than stock holds | `422 shuttle_stock_short` with `required_units` / `available_units`; **nothing written**; recording the purchase then settling succeeds |
| Settle with every line unticked | Cost drawn from club assets, absorbed by surplus, no player charged |
| Two settle requests at once | Exactly one settlement; the other gets `409`. This is the concurrency test, mirroring the existing RSVP capacity test |
| Change club rates, then view an old session | Old session still shows its snapshotted rates |

### 4 — Club position (User Story 3)

1. Record a $100 court-credit purchase → bank down $100, court credit up $100, no player
   balance moves.
2. Buy a tube at a different price → stock blends; existing stock value unchanged.
3. Settle a session until court credit is below the next session's cost → the position view
   raises `court_credit_short`.
4. `assets.total_cents` equals `player_balances_cents + surplus_cents`.

### 5 — Reminders (User Story 4)

1. Settle a session that leaves one participant at $8.00 and another at −$4.00.
2. First receives `balance_low`, second receives `balance_negative`, each once.
3. A member with a low balance who did **not** play receives nothing (the accepted gap
   recorded in the spec's assumptions).
4. A member who has disabled balance alerts receives nothing.

---

## What "done" looks like

- `go test ./...` green with no database; full suite green with one.
- Coverage floors met on both backend and frontend (CI enforces).
- `GET /api/admin/position/integrity` returns `balanced: true` after every scenario above.
- No code path anywhere updates or deletes a `LedgerEntry`.
