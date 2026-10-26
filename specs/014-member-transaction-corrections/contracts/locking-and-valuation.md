# Transaction boundaries, locks and session valuation

## Global order

All locks live in one database transaction on the caller's `tx`. Acquire only
needed stages, never acquire an earlier stage after a later one:

1. `transactions` table ROW EXCLUSIVE intent lock, before application row locks.
   Import uses SHARE ROW EXCLUSIVE instead, before its own locks.
2. MoneyOperation request reservation (INSERT ON CONFLICT then read/replay).
3. Actor and all old/new member rows FOR SHARE, ordered by UUID. Status-changing
   user paths use FOR NO KEY UPDATE. Recheck actor authorization here.
4. Relevant session rows FOR UPDATE in UUID order, if any. A linked shared expense
   locks its old/new context sessions to prevent deletion/status races. Native
   settlement operations lock the exact session before the settlement row.
5. Existing settlement/version rows where applicable, then exact original
   transaction FOR UPDATE. Immutable routing lookups may precede locks, but
   revalidate associations under locks. Top-ups skip session/settlement stages.
6. **Union** of original and proposed accounts FOR UPDATE in UUID order, including
   original zero legs and all accounts whose before/after is displayed. Acquire
   this set once before reading balances/stock or posting either transaction.
7. Re-read state under locks, compute exact review, check digest, append postings,
   immutable allocations/new settlement and reversal metadata, and completed
   operation result. Commit once. No notifications for these operations.

PostWithin may reacquire already-held account locks; it must not discover extra
accounts partway through a replacement. Prepare account IDs before locking. Use
transaction-aware account lookup/creation; current `PlayerAccountID` may create
through global `database.DB`, which must not be called inside this orchestration.
Approval/invitation should already create accounts; missing legacy accounts are
created in `tx` under user lock and uniqueness protection. Preview cannot create
accounts; missing-account preparation must be a separate explicit migration or
membership repair before review, otherwise return actionable unsupported state.

## Existing paths that must cooperate

Acquire the early table lock at the start of every transaction that may post:
`LedgerService.Post`/generic reversal, `SettlementService.Settle`/reversal,
`CorrectAssetPurchase`, `CorrectAssetBalance`, import asset confirmation and fixture
opening-balance orchestration. `PostWithin` takes it defensively, but this does not
replace early acquisition by callers already holding users/sessions/accounts.
Audit every PostWithin caller at implementation time. Import's strong table lock
already exists; preserve its reviewed-history protections and adopt the common
order before taking user/account locks. No new production import is authorized.

New reviewed paths freeze user status before sessions. Existing ordinary settlement
must resolve its participant/actor set first, lock users, then session and re-read
any default RSVP set. If that set changed, restart the whole transaction with the
new set (bounded retry), never lock newly discovered users after a session. Explicit
correction lines never fall back to RSVP defaults. Do not hold a club settings row
lock: correction uses saved rates. Initial settlement settings handling remains its
existing responsibility, but cannot invert new user/session/account locks.

Removal must use a single tx: member NO KEY UPDATE, future session locks sorted,
player account lock/recheck zero balance, RSVP cancellation/promotion, status/role
update. Acquire the financial table intent unconditionally first, before the advisory
role guard or any user locks, so import exclusion is mechanical. Actor/last-admin checks must be serialized consistently;
use a transaction advisory lock for the last-admin predicate **before user locks**
on all role/membership paths that can remove an approved admin, including
UpdateUserRole, UpdateMemberDetails role changes, and RemoveMember. Finance operations
do not take that advisory lock. Release/promotion messages run after commit.
Don't call public `DeleteRSVP` that commits independently; extract tx-aware helpers.
RSVP creation/update takes target user (and relevant actor) FOR SHARE before
session, rechecking approved status. Otherwise a new RSVP on a session outside
removal's discovered list can slip through. Deletion/promotion helpers must not
introduce late user locks under a held session lock. Test new RSVP vs removal.
Profile/admin/registration writes update only intended fields so a stale Save cannot
restore old approval/role. User-key-share foreign-key access remains compatible
with NO KEY UPDATE; do not upgrade removal to FOR UPDATE.

## Stale review and races

Preview: read-only REPEATABLE READ. Confirm: READ COMMITTED with the lock set above,
then fresh reads; each account writer already cooperates on account locks. Bound
entry counts plus balances/units invalidate a review after an intervening posting.
User share locks prevent membership changes until commit. Session row locks and
version/original locks prevent deletion or competing correction. Pending import
state cannot transition while the early table intent lock excludes import posting.

One same-ID concurrent retry waits on reservation then replays; different IDs on
one original serialize and only one wins. The loser gets a version conflict and
its reservation rolls back. Correction vs legacy inverse uses the same original
lock. Correction vs settlement uses the same session lock; exactly one live
settlement remains. Injected allocation/status/posting failure rolls everything
back. Deadlock/serialization errors never auto-post a different review: roll back
and allow bounded retry of the **same** contents, then an actionable retry error.

## Frozen stock delta algorithm

Let old consumption be `(u,v)`, proposed units `n`, and available current stock
`S=(units,value)` **before any inverse is posted**.

- `n=u`: replacement value `v`, S unchanged.
- `n<u`: replacement value `round_half_up(v*n/u)` (zero at n=0). Add
  `(u-n, v-new_value)` to S. If u=0 then v must be 0; inconsistent data is refused.
- `n>u`: use `S.Consume(n-u)` for extra units only; replacement value is old v
  plus the extra consumed value. Do not add the old consumed stock to S first.

Actual-count: old pair is sum of both recorded bands. Value whole n once; for
three hours extra value is nearest integer third (remainder 2 rounds up), base
gets the rest, preserving the existing 2:1 rule. All physical units stay on base.
Legacy estimated: derive proposed band units from exact decimal hundredths using
the original truncation rule, apply delta algorithm base then extra; returned
base stock can supply extra band. No later settled session is revalued.

New stock must have nonnegative units/value, and zero units implies zero value.
Positive units with zero cents are permitted. Bank/court/player may be negative;
this feature adds no new nonnegative policy there. Validate pending imported asset
state before an asset-moving operation. Use checked arithmetic; `int64(units)*value`
and doubled rounding numerators can overflow even when final cents are valid.

## Allocation and snapshots

Extract allocation from valuation. Supply frozen band court/shuttle totals to a
pure money function, preserving existing largest-remainder and comping rules.
Attendance-only keeps both original asset totals/units; comped-share changes may
change surplus and member totals, and are explicitly reviewed. When everyone stays
in actual mode, split combined total once; otherwise split each band's own players.
Guests have separate head keys but aggregate into host's ledger account.

Copy unchanged original charges for metadata-only corrections. For changed costs
or participation, use stable line keys (never list index) with session seed. Same
canonical inputs on preview/confirm/retry give the same cents. Preserve exact
recorded decimal rates; convert hours/rate to integer hundredths before money
multiplication. New court band snapshots prevent future reconstruction ambiguity.
For old rows reconstruct exact court bands and compare to original court debit;
refuse unsupported data rather than estimate. Unchanged recorded consumption must
also reconcile to original stock debit/units and charge/surplus entries.

After reviewing, append exact original inverse, mark old settlement reversed,
post linked replacement and new settlement/lines, and verify final stock/invariant
in one tx. Do not call ordinary Settle's after-commit notifier. Final net effects
must equal the preview; no intermediate undo notification or UI-visible commit.

Examples: old 6/$24 → 4 means new $16 and return 2/$8. Old 6/$24 → 8 with
current 12/$60 means new $34, consuming 2/$10 from current stock. With attendance
only, court/shuttle deltas are zero even after new purchases/default-rate changes.
