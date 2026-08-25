# Contract: Accounts, Balances, and Transactions

Conventions in [README.md](./README.md).

---

## Reads

### `GET /api/accounts` — group `approved`

Every player's balance. Visible to all approved members (FR-024).

```json
{
  "items": [
    { "user_id": "…", "name": "Karthik", "profile_picture": "…", "balance_cents": 4250 },
    { "user_id": "…", "name": "Priya",   "profile_picture": "…", "balance_cents": 3100 },
    { "user_id": "…", "name": "Jono",    "profile_picture": "…", "balance_cents": -825 }
  ]
}
```

Club accounts are **not** included here; they are admin-only via `/api/admin/position`.

### `GET /api/accounts/me` — group `approved`

The caller's own balance. Deliberately minimal — this backs the header chip (FR-030) and is
requested on most screens.

```json
{ "balance_cents": 4250, "state": "ok" }
```

`state` is `ok` | `low` | `negative`, derived from the club's threshold so the frontend does
not duplicate the rule.

### `GET /api/accounts/me/entries?limit=&offset=` — group `approved`

The caller's own itemised history, newest first, reconciling to their balance (FR-028).

```json
{
  "items": [
    {
      "id": "…",
      "occurred_at": "2026-08-25T21:15:00+10:00",
      "kind": "session_settlement",
      "description": "Tuesday session — 2h + extra hour",
      "amount_cents": -2790,
      "balance_after_cents": 4250,
      "session_id": "…",
      "reversed": false
    },
    {
      "id": "…",
      "occurred_at": "2026-08-19T09:02:00+10:00",
      "kind": "player_topup",
      "description": "Bank transfer",
      "amount_cents": 5000,
      "balance_after_cents": 7040,
      "reversed": false
    }
  ],
  "total": 34
}
```

`balance_after_cents` is a running balance computed over the ordered entries, so a member
can follow how their balance was arrived at rather than having to add it up themselves.

### `GET /api/admin/position` — group `admin`

The club's true position (FR-029, User Story 3).

```json
{
  "assets": {
    "bank_cents": 18500,
    "court_credit_cents": 1700,
    "shuttle_stock_cents": 3750,
    "shuttle_stock_units": 9,
    "total_cents": 23950
  },
  "liabilities": { "player_balances_cents": 23950 },
  "surplus_cents": 0,
  "balanced": true,
  "warnings": [
    {
      "code": "court_credit_short",
      "message": "Court credit covers $17.00; the next session needs $60.00.",
      "next_session_id": "…"
    }
  ]
}
```

`balanced` restates invariant I1 for the reader. It is always `true` in normal operation —
a posting that would make it false is rolled back — so a `false` here means something
bypassed `LedgerService`.

### `GET /api/admin/position/integrity` — group `admin`

Recomputes every balance from `ledger_entries` and reports the identity. Exists so the
invariant can be checked independently of the code that maintains it.

```json
{ "balanced": true, "residual_cents": 0, "entries_checked": 1284, "accounts": 19 }
```

---

## Writes

All writes are `admin` (FR-007). All return the created transaction with its entries.

### `POST /api/admin/transactions/topup`

```json
{ "user_id": "…", "amount_cents": 5000, "occurred_at": "2026-08-19T09:02:00+10:00", "description": "Bank transfer" }
```

Moves: bank `+X`, that player `+X`.

### `POST /api/admin/transactions/withdrawal`

Same shape. Moves: bank `−X`, that player `−X`. For settling up with a departing member.

### `POST /api/admin/transactions/court-credit`

```json
{ "amount_cents": 10000, "occurred_at": "…", "description": "Venue account top-up" }
```

Moves: bank `−X`, court credit `+X`. No player balance changes.

### `POST /api/admin/transactions/shuttle-purchase`

```json
{ "units": 12, "amount_cents": 5000, "occurred_at": "…", "description": "1 tube" }
```

Moves: bank `−X`, shuttle stock `+X` and `+units`. Blends into weighted-average cost
automatically; existing stock value is untouched (research.md R6).

### `POST /api/admin/transactions/opening-balances`

Go-live seeding (FR-008). Accepted once; a second call returns `409`.

```json
{
  "occurred_at": "2026-09-01T00:00:00+10:00",
  "players": [ { "user_id": "…", "balance_cents": 4250 } ],
  "bank_cents": 18500,
  "court_credit_cents": 1700,
  "shuttle_stock": { "units": 9, "amount_cents": 3750 }
}
```

Surplus is the balancing figure, computed so that I1 holds — it is not supplied by the
caller.

### `POST /api/admin/transactions/:id/reverse`

```json
{ "description": "Recorded against the wrong player" }
```

Posts the exact negation of the referenced transaction (research.md R9). Returns `409`
`transaction_already_reversed` if it has been reversed before. Reversing a
`session_settlement` also stamps `settlements.reversed_at`, freeing the session to be
settled again.

There is no endpoint to edit or delete a transaction or entry, by design (Principle VI).
