# Phase 1 Data Model: Club Ledger and Session Settlement

All monetary fields are `int64` cents. All timestamps are `timestamptz`, written and read in
Australia/Sydney. All primary keys are UUIDs generated in `BeforeCreate`, per Principle I.

---

## New entities

### Account

A place a balance accumulates. Rows exist for identity, naming, and row-level locking; the
balance itself is derived from entries (research.md R4).

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK |
| `kind` | text | `player` \| `bank` \| `court_credit` \| `shuttle_stock` \| `surplus` |
| `user_id` | uuid, null | Set only when `kind = player`. Unique when present. |
| `name` | text | Display label, e.g. "Court credit (venue account)" |
| `created_at` / `updated_at` | timestamptz | |

**Rules**

- Exactly one account each of kind `bank`, `court_credit`, `shuttle_stock`, `surplus`.
- Exactly one `player` account per user, created when a user is first approved.
- The admin's `player` account is an ordinary player account, distinct from the club
  accounts (FR-002).
- Accounts are never deleted. A departing member's account is retained with its history.

### Transaction

One recorded financial event.

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK |
| `kind` | text | see transaction templates below |
| `session_id` | uuid, null | Set on `session_settlement` |
| `reverses_transaction_id` | uuid, null | Set on `reversal`. Unique — a transaction may be reversed at most once. |
| `description` | text | Free text shown in ledger listings |
| `occurred_at` | timestamptz | When the money moved in the real world; may predate `created_at` |
| `created_by` | uuid | Admin who recorded it |
| `created_at` | timestamptz | |

### LedgerEntry

One account's part of a transaction. **Append-only**: no update or delete path exists
(Principle VI).

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK |
| `transaction_id` | uuid | FK, indexed |
| `account_id` | uuid | FK, indexed |
| `amount_cents` | bigint | Signed, in the account's own reading (research.md R1) |
| `units` | integer, null | Shuttle count. Non-null only on `shuttle_stock` entries. |
| `created_at` | timestamptz | |

### Settlement

The record of a session being costed. Rates are snapshotted so later settings changes never
rewrite history (FR-017).

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK |
| `session_id` | uuid | FK. At most one live settlement per session — partial unique index where `reversed_at IS NULL`. |
| `transaction_id` | uuid | The `session_settlement` transaction it posted |
| `base_hours` | numeric(4,2) | Snapshot |
| `base_rate_cents` | bigint | Snapshot, per hour |
| `extra_hours` | numeric(4,2) | Snapshot. `0` when play did not run long. |
| `extra_rate_cents` | bigint | Snapshot, per hour |
| `shuttles_per_hour` | numeric(4,2) | Snapshot |
| `base_shuttle_cents` | bigint | Value of shuttles consumed by the base band |
| `extra_shuttle_cents` | bigint | Value of shuttles consumed by the extra band |
| `settled_at` / `settled_by` | timestamptz / uuid | |
| `reversed_at` / `reversed_by` | timestamptz, null / uuid, null | Set when the settlement is reversed, freeing the session to be settled again |

### ChargeLine

One participant's part of a settlement. Collectively these are the record of who played
(FR-020), including lines charged at zero.

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK |
| `settlement_id` | uuid | FK, indexed |
| `user_id` | uuid | Whose account bears the charge. For a guest line, the host. |
| `guest_name` | text, null | Non-null makes this a guest line |
| `in_base` | bool | Participated in the base band |
| `in_extra` | bool | Participated in the extra band |
| `comped` | bool | Counted as a head, charged nothing; the share goes to surplus (FR-021) |
| `amount_cents` | bigint | What this line was actually charged. `0` when comped. |

**Rules**

- `in_base` or `in_extra` must be true — a line that participated in nothing is not a line.
- `in_extra` may only be true when `settlement.extra_hours > 0`.
- A user may appear on multiple lines: their own, plus one per guest they hosted.

---

## Extended entities

### Club — new settings fields

| Field | Type | Default |
|---|---|---|
| `base_hours` | numeric(4,2) | `2.00` |
| `base_rate_cents` | bigint | `3000` ($30/hour) |
| `extra_rate_cents` | bigint | `2300` ($23/hour) |
| `shuttles_per_hour` | numeric(4,2) | `5.00` |
| `low_balance_threshold_cents` | bigint | `2000` ($20) |

Defaults are seeded alongside the existing default-club seed in `database.Migrate()`.

### Session — resolved timestamps

| Field | Type | Notes |
|---|---|---|
| `starts_at` | timestamptz, null | Resolved from `session_date` + `start_time` in Sydney |
| `ends_at` | timestamptz, null | Resolved from `session_date` + `end_time` in Sydney |

Populated on create and update, backfilled once by SQL (research.md R10). The existing
`start_time` / `end_time` strings remain the editing interface. Nullable during migration;
non-null in practice once backfilled.

---

## Transaction templates

Each kind moves a fixed set of accounts. `LedgerService` owns these templates; nothing else
constructs entries.

| Kind | Bank | Court credit | Shuttle stock | Player | Surplus |
|---|---|---|---|---|---|
| `player_topup` | `+X` | | | `+X` | |
| `withdrawal` | `−X` | | | `−X` | |
| `court_credit_purchase` | `−X` | `+X` | | | |
| `shuttle_purchase` | `−X` | | `+X`, `+n units` | | |
| `session_settlement` | | `−court` | `−shuttles`, `−n units` | `−share` each | `+/− residual` |
| `opening_balance` | `+X` | `+X` | `+X`, `+n` | `+X` each | balancing figure |
| `reversal` | negation of the referenced transaction | | | | |

For `session_settlement` the surplus movement is exactly the total of comped shares —
negative, since the club absorbs them. When nobody is comped, surplus does not move: charges
sum precisely to consumption, because largest-remainder allocation loses nothing (R5).

---

## Invariants

Asserted inside every posting transaction, before commit (research.md R2):

**I1 — Club position.**

```
(bank + court_credit + shuttle_stock) − Σ(player balances) − surplus = 0
```

**I2 — Non-negative shuttle stock.** `shuttle_stock.units ≥ 0` after any posting.
Settlement checks this before posting and aborts with a typed shortfall error (R7).

**I3 — Exact split.** For each band, `Σ charge_line.amount_cents + comped_total = band_total`.

**I4 — Append-only.** No code path updates or deletes a `LedgerEntry`. Enforced by review
and by the absence of any such method on `LedgerService`; a test asserts the service exposes
no mutating entry method.

**I5 — One live settlement per session.** Enforced under the session row lock and backstopped
by a partial unique index (R8).

---

## Worked example

Club settings at defaults. Shuttle stock holds 24 units valued at 10000c (two $50 tubes).
Six heads play the base two hours — five members plus one guest hosted by the admin — and
four stay for the extra hour.

**Base band**

| | |
|---|---|
| Court | 2h × 3000 = **6000c** |
| Shuttles needed | 2h × 5 = 10 units |
| Shuttle value | round(10 × 10000 ÷ 24) = **4167c** |
| Band total | **10167c** |
| Split across 6 | 10167 ÷ 6 = 1694 base, remainder 3 → three lines at **1695c**, three at **1694c** |

**Extra band** — stock is now 14 units at 5833c.

| | |
|---|---|
| Court | 1h × 2300 = **2300c** |
| Shuttles needed | 1h × 5 = 5 units |
| Shuttle value | round(5 × 5833 ÷ 14) = **2083c** |
| Band total | **4383c** |
| Split across 4 | 4383 ÷ 4 = 1095 base, remainder 3 → three lines at **1096c**, one at **1095c** |

**Postings**

| Account | Movement |
|---|---|
| Court credit | −8300c |
| Shuttle stock | −6250c, −15 units |
| Player accounts | −14550c in total across ten charge lines |
| Surplus | 0 (nobody comped) |

Charges total 14550c; consumption totals 8300 + 6250 = 14550c. I1 holds, I3 holds per band.
Stock afterwards is 9 units at 3750c — a blended 416.67c per shuttle, unchanged, as it must
be when nothing was bought or sold in between.
