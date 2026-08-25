# Phase 0 Research: Club Ledger and Session Settlement

Ten decisions that had to be made before a data model could be drawn. Each records what was
chosen, why, and what was rejected.

---

## R1. Sign convention for ledger entries

**Decision**: Store every amount with the sign that account's own reading uses. A player in
credit has a positive balance. An asset account holds a positive value. The balancing rule
is not "entries sum to zero" but the typed identity:

```
(bank + court_credit + shuttle_stock) - (Σ player balances) - surplus = 0
```

**Rationale**: Textbook double-entry stores liabilities negative so that a naive
`SUM(amount) = 0` holds. That would mean a player in credit is stored as a negative number
and flipped at the API boundary — a sign flip that has to be remembered in every new query,
export, and admin script, and that reads wrong to anyone inspecting the database directly.
This is a club app maintained by one person; stored data matching what humans see is worth
more than the simpler aggregate. The identity is still a single SQL expression and is just
as testable:

```sql
SUM(CASE WHEN a.kind IN ('bank','court_credit','shuttle_stock')
         THEN e.amount_cents ELSE -e.amount_cents END) = 0
```

**Alternatives considered**: Classic credit-normal storage with a display flip — rejected
for the footgun above. Storing an explicit debit/credit pair per entry — rejected as
ceremony that buys nothing at this scale.

---

## R2. Where the invariant is enforced

**Decision**: In `LedgerService.post()`, inside the same database transaction that writes
the entries, immediately before commit. The check recomputes the identity across all
accounts and returns an error — rolling the transaction back — if it is non-zero.

**Rationale**: A database constraint cannot express a cross-row aggregate, and a background
checker finds corruption after it has been shown to users. Enforcing at the single write
choke-point means no caller can bypass it, and the rollback makes a violated invariant
impossible to persist rather than merely detectable. Recomputation costs one aggregate
query over a table with a few thousand rows.

**Alternatives considered**: A `CHECK` constraint — not expressible. A nightly
reconciliation job — detects rather than prevents; retained anyway as an admin-visible
integrity endpoint (contracts/ledger.md), which is cheap once the query exists.

---

## R3. Making `LedgerService` the sole writer

**Decision**: `LedgerEntry` and `Transaction` rows are created only by `LedgerService`.
`SettlementService` composes a settlement by handing `LedgerService` a set of intended
movements; it never writes entries itself. No handler, and no other service, touches those
tables.

**Rationale**: Principle VII's invariant is only as good as the narrowest gate it can be
enforced at. One writer means one place to audit.

---

## R4. Derived balances rather than a cached column

**Decision**: `Account` carries no `balance_cents` column. Balances are computed by
aggregating `ledger_entries`. Account rows exist for identity, naming, and row-level
locking.

**Rationale**: A cached balance can drift from its entries, and reconciling the two becomes
a permanent maintenance tax. At this scale the aggregate is free — a few thousand rows, a
handful of accounts. Deriving makes drift structurally impossible rather than merely
unlikely. If the ledger ever grows enough to matter, a cached column can be added behind
the same service method without changing any caller.

**Alternatives considered**: Cached balance updated in the posting transaction, with a
periodic recompute-and-compare — rejected as premature, and it reintroduces exactly the
class of bug the append-only design exists to eliminate.

---

## R5. Largest-remainder allocation, and who pays the odd cent

**Decision**: For a band total `T` split among `n` participants: `base = T / n` (integer
division), `r = T - base*n`. Every participant is charged `base`; the first `r`
participants in a per-settlement deterministic order are charged one cent more. The order
is a sort on `sha256(settlement_id || participant_id)`.

**Rationale**: Largest remainder guarantees the charges sum to exactly `T` — Principle V's
requirement — without absorbing or inventing value. Seeding the order with the settlement
id means the same person is not always the one paying the extra cent, while remaining fully
deterministic and reproducible in tests.

**Alternatives considered**: Sorting by participant id — deterministic but assigns the odd
cent to the same person every session. Pushing the remainder to club surplus — makes
surplus drift on rounding rather than only on comping, muddying its meaning.

---

## R6. Weighted-average shuttle cost

**Decision**: `shuttle_stock` carries both a value (`amount_cents`) and a quantity
(`units`), both derived from entries. At consumption:

```
consumption_cents = round_half_up(count * stock_value / stock_units)
```

computed in integer arithmetic, then stock is reduced by `consumption_cents` and `count`.
No per-unit price is ever stored.

**Rationale**: A $50 tube of twelve is 416.66… cents a shuttle. Storing that rounded and
multiplying it back leaks value on every session. Deriving from the authoritative pair
(value, units) means the stock value is always exactly what was paid minus what was
consumed, and buying at a new price blends automatically without touching existing stock.

**Alternatives considered**: A configured per-shuttle price — cannot represent a real
purchase and drifts from what was actually paid. FIFO lot tracking — correct but far more
machinery than a club bag of shuttles justifies.

---

## R7. What happens when stock is short

**Decision**: Settlement computes required shuttles up front. If `stock_units` is less than
required, it aborts with a typed error carrying the shortfall, and posts nothing. The API
returns a structured error the settlement form uses to offer recording the missing purchase
inline; the admin records it and re-submits.

**Rationale**: Follows the spec's resolved clarification. It also removes a concept
entirely: because stock can never be zero or negative at consumption, there is never a
division by zero and no fallback price rule is needed anywhere in the design.

**Alternatives considered**: Allowing negative stock with a fallback price — needs a
fallback price concept, and a negative bag of shuttles is not a thing.

---

## R8. Settlement idempotency and locking

**Decision**: Settling acquires, in one transaction and in this order:

1. the `sessions` row, `FOR UPDATE` — mirroring the existing RSVP capacity pattern;
2. the `accounts` rows being touched, `FOR UPDATE`, ordered by `id`.

Under that lock the service verifies no live settlement exists for the session. A partial
unique index is added as a backstop:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_settlements_one_live_per_session
  ON settlements (session_id) WHERE reversed_at IS NULL;
```

**Rationale**: The session lock makes the check-then-act sequence safe, exactly as it
already does for RSVP capacity, so the pattern is familiar in this codebase. Locking
accounts in `id` order prevents deadlock between two settlements touching overlapping
accounts. The partial index is a database-level guarantee that survives a future caller who
forgets the lock; `AutoMigrate` cannot express a partial index, so it is issued as raw SQL
in `database.Migrate()`, guarded by `IF NOT EXISTS`.

**Alternatives considered**: Relying on the index alone — turns a race into an opaque
constraint violation rather than a clean domain error. Relying on the lock alone — no
backstop.

---

## R9. Reversal

**Decision**: `Transaction.reverses_transaction_id` points at the reversed transaction. A
reversal writes entries that are the exact negation of the original, including shuttle
units. A unique index on `reverses_transaction_id` makes double-reversal impossible.
Reversing a settlement also stamps `settlements.reversed_at`, which frees the session to be
settled again.

**Rationale**: Negation is correct by construction — if the original satisfied the identity,
so does its negation, so the invariant check cannot fail on a reversal. It also means
reversal needs no knowledge of what the original meant, only what it moved. Balances unwind
correctly even if the players involved have transacted since, because entries are additive
rather than snapshot-based.

---

## R10. Session timestamps, and the scope of that migration

**Decision**: Add nullable `starts_at` and `ends_at` (`timestamptz`) to `Session`, populated
on create and update from the existing date and `HH:MM` inputs, and backfilled once in
`database.Migrate()`:

```sql
UPDATE sessions
   SET starts_at = (session_date + start_time::time) AT TIME ZONE 'Australia/Sydney',
       ends_at   = (session_date + end_time::time)   AT TIME ZONE 'Australia/Sydney'
 WHERE starts_at IS NULL;
```

The existing `start_time` / `end_time` strings stay as the editing interface and the display
value; the timestamps become the thing business rules compare against.

**Rationale**: PostgreSQL's `AT TIME ZONE` resolves Sydney local time to an instant with DST
handled correctly, so the backfill is right without application code looping over rows. The
change is additive and idempotent, so it is safe to re-run and needs no coordination with a
deploy.

**Scope correction**: The spec assumed billing depends on this. It does not — billed hours
are an explicit input at settlement (FR-016). The migration is needed because the History
tab needs a dependable past/upcoming boundary (today: `session_date >= today`, which treats
a session that finished three hours ago as upcoming) and because
`SchedulerService.parseSessionDateTime` reconstructs an instant from a date plus an `HH:MM`
string on every read, which Principle IV forbids. It is therefore independent work that can
land before, alongside, or after P1, and must not be treated as blocking the ledger.

**Alternatives considered**: Replacing the string columns outright — larger blast radius
across the frontend and admin forms for no benefit while the strings remain the natural
editing interface. Computing the boundary in Go on read — the read-time parse Principle IV
exists to remove.
