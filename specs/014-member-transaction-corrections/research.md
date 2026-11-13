# Design decisions: member corrections and shared expenses

Date: 2026-10-26. Baseline: merged PR #103, `9b83151`.
This is repository research, not a claim that the proposed flows are implemented.
No additional framework, provider, database or money engine is needed.

## 1. Keep orchestration above the one ledger writer

**Decision:** Add `MemberMoneyService` for reviewed operations and
`SharedExpenseService` for bill validation/allocation. Extract transaction-scoped
settlement helpers; `LedgerService.PostWithin` remains the only entry writer.
Use existing reversal/correction links and the partial unique live-settlement index.

**Rationale:** Public `Settle` commits and notifies; public reversal opens its own
transaction. Neither can be called midway through an atomic replacement.

**Alternatives considered:** Mutating original rows loses auditability. Calling
undo and settle sequentially exposes an unsettled interval and partial failure.
A new generic accounting framework duplicates existing tested rules.

Evidence: `backend/internal/services/{ledger_service,settlement_service,asset_correction}.go`.

## 2. Stateless review, durable successful submission

**Decision:** Preview is a read-only repeatable-read transaction returning
canonical inputs, exact effects and SHA-256 fingerprint. Confirmation reconstructs
that representation under locks. A new `MoneyOperation` row durably binds a
client-generated request UUID to actor, action, target and canonical contents.
An identical successful retry returns its stored response before freshness checks.

**Rationale:** Existing expense fingerprints bind costs but not all displayed
balances or current memberships. Existing asset retries are specific to a
corrected original, not a reusable submission protocol. A plain digest is enough:
the server recomputes effects and authorizes the action; the digest grants no
permission and the client never supplies authoritative movements.

**Alternatives considered:** Persisting previews would make opening/cancelling
review write data. Process-memory idempotency fails across Cloud Run revisions.
A new signing secret adds deployment work without replacing server validation.

Evidence: `expense_service.go`, `asset_correction.go`, `balance_correction.go`.

## 3. Freeze membership before financial locks

**Decision:** Lock actor and the union of original/proposed users `FOR SHARE`,
in UUID order, before session/original/account locks. Membership writes use
`FOR NO KEY UPDATE`; removal becomes one transaction including RSVP release and
final balance recheck. Replace stale whole-row profile saves with targeted updates.

**Rationale:** Middleware alone cannot prevent approval/role changes during a
confirmation. Current removal checks balances and commits RSVP changes separately.
User `FOR UPDATE` can conflict with foreign-key key-share locks while a session
is held; `NO KEY UPDATE` avoids that inversion. All helpers must use the caller's
`tx`, including RSVP promotion. Notifications run only after successful commit.

**Alternatives considered:** User locks after session invert removal's order.
Wrapping the current public removal helpers in a transaction does not enlist their
independently committed writes. A club-wide membership mutex is unnecessarily broad.

Evidence: `user_service.go:RemoveMember`, `rsvp_service.go`, `settlement_service.go:Settle`.

## 4. Acquire the transactions table intent lock early

**Decision:** Every financial orchestration first takes
`LOCK TABLE transactions IN ROW EXCLUSIVE MODE`. The importer retains its
`SHARE ROW EXCLUSIVE` lock, acquired before any other financial locks. Ordinary
writers remain mutually compatible; import is excluded before writers hold users,
sessions or accounts. See the precise adoption list in the locking contract.

**Rationale:** The importer currently takes the table lock before accounts, while
ordinary posting locks accounts before inserting into that table. Introducing
new account-holding paths without a protocol retains a table/account deadlock.

**Alternatives considered:** An intent lock only inside `PostWithin` is too late
for callers that already lock accounts. A new advisory-lock namespace is unnecessary
when PostgreSQL's existing table lock expresses the exclusion directly.

Evidence: `splitwise_import_service.go:ImportSplitwiseWithOptions`,
`settlement_service.go:Settle`, `asset_correction.go`, `balance_correction.go`.

## 5. Retain zero-net allocations and zero-valued member legs

**Decision:** Every shared-expense allocation has one player ledger leg, even
when paid minus owed is zero. Keep immutable paid/owed allocations independently.
Personal funding has no asset legs; bank funding adds exactly one bank leg.

**Rationale:** `PostWithin` rejects an empty list but already accepts zero-valued
legs. This retains all-zero-net bills and exact inverse support without inventing
cash movements or bypassing the invariant. Allocations remain the source of
personal inclusion and gross amounts; they must not depend on a nonzero leg.

**Alternatives considered:** Dropping zero-net participants hides their expenses.
A separate empty-posting writer unnecessarily broadens the ledger interface.

Evidence: `ledger_service.go:PostWithin`, `ledger_activity.go`.

## 6. Order correction effects explicitly

**Decision:** Add operation identity and step to newly posted transactions.
History uses one shared ordering definition: effective timestamp, recording group,
then inverse step 0 before replacement step 1. Operation sequences resolve equal
recording timestamps. Legacy rows retain their existing entry timestamp/UUID order.
Both raw running balances and grouped activity use this definition.

**Rationale:** Reversal/replacement can share PostgreSQL timestamps; random entry
UUID order does not preserve posting order. Transaction ID and recording time are
currently hidden in `LedgerEntryView` JSON and must become additive read fields.

**Alternatives considered:** Reordering only grouped rows leaves raw balances
inconsistent. Updating old entry timestamps would violate immutable history.

Evidence: `ledger_history.go:ledgerHistorySQL`, `ledger_activity.go:ledgerOrder`.

## 7. Separate stock valuation from charge allocation

**Decision:** Extract charge allocation over explicit court/shuttle band totals.
Keep unchanged stock pairs, return proportional original value for reductions,
and consume only additional units from stock BEFORE undo. Use saved modes/rates.
Use checked integer/rational arithmetic, never a rounded unit price.

**Rationale:** Existing `cost` always consumes from current stock. Running it on
stock after reversal reprices original consumption. Legacy court arithmetic uses
float-backed hours; reconstruct exact decimal hundredths and verify against the
original debit, refusing unsupported data. New replacements snapshot exact court
band cents. Actual-count expenses retain their existing member-only restrictions;
legacy guests and comped lines retain their meaning.

**Alternatives considered:** Repricing the whole session changes historical cost.
Treating legacy expenses as actual-count drops guest/comped semantics.

Evidence: `settlement_service.go:cost`, `models/settlement.go:CourtCents`,
`services/money/{split,stock}.go`.

## 8. Stabilize allocation identity and numeric limits

**Decision:** Shared equal splits use stable expense-root UUID as seed and member
UUIDs, independent of selection order and replacement version. Session correction
lines have stable allocation keys (existing line ID for retained legacy lines;
client UUID for additions). Pure metadata corrections copy original charge amounts.
Use integer cents bounded to 4,503,599,627,370,495, checked totals/deltas and checked
multiplication or `math/big` intermediates. Never assume the current stock helper's
int64 multiplication is safe at that bound.

**Rationale:** Existing `splitBand` includes list index in remainder identity.
Reordering an editor must not redistribute cents. Existing amount bounds do not
by themselves prevent overflow in proportional multiplication or outcome balances.

**Alternatives considered:** Random seeds per preview make confirmation unstable.
Floating-point currency violates the constitution. Arbitrary tighter product limits
would change which otherwise valid existing balances can be corrected.

Evidence: `settlement_service.go:splitBand`, `money/split.go`, `money/stock.go`,
`balance_correction.go`.

All design questions identified during this research have a decision. Verification
of their implementation belongs to the uncompleted tasks, not this document.
