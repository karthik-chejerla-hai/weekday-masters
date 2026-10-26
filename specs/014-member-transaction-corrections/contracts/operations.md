# Reviewed money operation API

Planned contract, not current endpoints. Prefix every path with `/api`.
Use `services/api.ts` in the browser; Gin handlers parse, services decide rules.
All preview/confirm writes require authenticated **approved admins**, including
service-level actor revalidation. Approved members may read transaction details.
Use strict JSON: reject unknown fields, fractional cents, invalid UUIDs, duplicate
members/keys and overflowing totals. Limit JSON bodies to 128 KiB and member/line
arrays to 100; reject oversized bodies before parsing. Reject duplicate JSON
object keys using a strict decoding helper; DisallowUnknownFields alone does not
reject them. Cents, sum totals, signed deltas and current/new displayed balances
must all fit ±4,503,599,627,370,495; nonnegative bill/paid/owed/stock fields must
fit 0 through that maximum. Check intermediate int64 arithmetic too; decline
unsupported out-of-range historical outcomes with an actionable error.

## Endpoints

- `GET /transactions/:id`: approved-member detail including original, immediate
  inverse/replacement, current actionable descendant, reason, recorder/times,
  allocations or settlement snapshot and allowed actions. Include the exact
  transaction ID, not merely a session ID. Never expose private notification data.
- `POST /admin/transactions/:id/correction/preview` then
  `POST /admin/transactions/:id/correction/confirm`: top-up or shared correction;
  route by persisted kind, never client-asserted type.
- `POST /admin/transactions/:id/reversal/preview` then
  `POST /admin/transactions/:id/reversal/confirm`: reviewed exact inverse for a
  current supported top-up, shared expense or native settlement transaction.
- `POST /admin/shared-expenses/preview` then
  `POST /admin/shared-expenses/confirm`: create a bill/version.
- `GET /admin/settlements/:id/correction`: editor defaults from that exact saved
  settlement, mode and lines; imported sessions are refused.
- `POST /admin/settlements/:id/correction/preview` then
  `POST /admin/settlements/:id/correction/confirm`: whole-settlement replacement.
- `GET /admin/shared-expenses/session-suggestions?session_id=UUID`: optional,
  read-only, approved native-session confirmed RSVP suggestions. Called only on
  explicit user action, never silently replaces entered participants. Link validity
  is checked again during confirmation. Shared context may be an existing native
  session; it need not be finished. Settlement correction requires finished,
  non-cancelled, non-imported session with this exact live settlement.

## Common request envelope

Preview: `{request_id, reason?, proposed}`. Correction/reversal requires trimmed
reason of 1–1,000 Unicode characters; creation needs no correction reason.
Confirmation sends the **same** envelope plus `expected_preview` (64 hex chars).
For reversal `proposed` is `{}`; clients never specify inverse movements.
`request_id` is a nonzero client UUID retained for retries; changed accepted
contents require a new ID after success. Before a failed/uncommitted confirmation,
a refreshed preview may use the same ID. Proposed entity IDs stay stable during
editing so allocation order cannot change on each review.

### Top-up proposal

`{user_id, amount_cents, occurred_date, description}`. Amount positive, within
safe-cent bounds; description trimmed, at most 1,000 characters (empty allowed).
Date is `YYYY-MM-DD`, no future Sydney date. The editor starts with exact existing
member/amount/date/note. Same date preserves the exact original effective instant;
changed date resolves Sydney midnight. No withdrawal, opening/import, asset audit,
asset purchase or reversal kind can enter this flow. Verify the original has exactly
its expected member/bank movements before reconstructing it; refuse malformed data.

### Shared proposal

`{version_id, expense_id, description, occurred_date, category, funding,
session_id, split_mode, allocations}`. Optional session uses null to clear.
Description required 1–1,000 chars; category `food|other`, funding
`personal|club_bank`, split mode `equal|custom`, plus positive `bill_cents`.
Allocations are `{user_id, paid_cents, participates, owed_cents?}`.
One row per union of payers/participants. In equal mode omit owed amounts;
server allocates only participating rows with root UUID seed. In custom mode
participating rows supply exact owed amounts (zero allowed); nonparticipants
owe zero. A nonparticipating row must have positive paid amount. New expenses
use approved existing members; corrections may retain/change inactive users
already on that exact version. Newly added users must be approved. Bank funding
requires every member paid amount to be zero and at least one participant.
Personal funding requires sum paid = bill; all modes sum owed = bill.

Correction inherits target root ID; a different root is rejected. Proposed
version ID must be new. Selected order is normalized by user UUID. No-change
comparison excludes reason, request ID, proposed new version ID and array order;
metadata-only changes remain real changes. The optional session never changes
RSVPs, scores, badminton charges or spend classification.

### Session proposal

`{mode, description, rates, lines, actual_shuttles?}`. Mode is returned by the
editor as `actual|estimated` and must match the saved version. No session/date
switch. Rates include `base_hours_hundredths`, `extra_hours_hundredths`,
`base_rate_cents`, `extra_rate_cents`, `shuttles_per_hour_hundredths` from the
saved snapshot; integer hundredths adapt to existing decimal hour columns.
Actual mode requires base 200, extra 0/100, explicit count 0–200 and retains
existing no-guest/no-comping/member-in-base restrictions. Estimated mode keeps
saved hour/rate meaning, with nonnegative hundredths and bounded derived units.
Hours must fit existing numeric(4,2) columns (base 1–9999 hundredths, extra
0–9999); shuttle rate 0–9999 hundredths. Derived total shuttle use is at most
2,147,483,647 units and every multiplication is checked.

Lines: `{allocation_key,user_id,guest_name,in_base,in_extra,comped}`. Preserve
keys for retained lines, new UUID keys for additions. A retained key cannot be
reassigned to a different member/guest; remove/add instead. No duplicate member
line; guests remain charged to their host, distinct guests have distinct keys.
For retained logical member/guest lines, saved keys are mandatory; removing and
readding identical attendance cannot change keys to reshuffle cents. No-change
comparison for all three correction types ignores request/reason/new IDs, key-only
churn and array order. Session metadata-only changes copy exact original amounts.
Description max 1,000; guest names trimmed 1–255 when present. Require at least
one relevant head for every nonzero band. No current defaults or RSVP fallback
may enter correction. Reordering lines alone is a no-op.

## Preview response (200)

`{request_id, action, target_transaction_id, target_settlement_id?, input,
expected_preview, original?, replacement?, effects, explanation}`.
`input` is the canonical request (with derived allocations). `effects.members`
contains the **union** of old/new users, including zero-net/removed users:
`{user_id, display_name, inactive, original_paid_cents, original_owed_cents,
replacement_paid_cents, replacement_owed_cents, delta_cents,
current_balance_cents, new_balance_cents}`. Top-ups are labeled deposits rather
than pretending paid/owed is a bill; their bill-only fields are null.
`effects.assets` contains each account touched by original or proposal, even if
its net delta is zero: kind, before, delta, after and stock units where relevant.
Show surplus effects for comped sessions. Do not include unrelated balances.
`original/replacement` include cost-band totals, consumed stock pairs and lines
where applicable. Explanation states that saving/undoing records transfers no money.

Fingerprint is SHA-256 of versioned canonical UTF-8 JSON with a documented field
order; sort user/account lists by UUID and normalize strings, dates, nulls and
explicit defaults. Domain-tag with `member-money-review:v1`. Include actor ID/
role/status, action/target, request identity, proposed IDs/fields, reason, exact
original link/live-state/snapshot, affected member statuses/display names, optional
session existence/native/status facts, original/replacement effects, and each
reviewed account's current value/units plus entry count. Include assets-pending
state for asset-moving operations. Count detects relevant postings even when
net balance returns to the same value. Purely personal expenses don't bind bank,
stock, unrelated users or club defaults. A database migration changing digest
semantics bumps the tag, producing refresh-required for old previews.

Preview reads one repeatable-read snapshot and does not create reservation rows,
ledger entries, allocations or notifications. There is no arbitrary TTL; changed
bound state invalidates the review. Future-date checks are re-evaluated at confirm.

## Confirmation response and retry semantics

Return 201 on first success with `{operation_id, original_transaction_id?,
reversal_transaction_id?, replacement_transaction_id?, settlement_id?,
shared_expense_version_id?, accepted_review, links}`. A byte-equivalent successful
retry returns the same HTTP status and stored DTO, even when the current version
or balances changed later. Current authorization still applies before returning it.

Within one DB transaction: early table lock; reserve the request UUID with
`INSERT ... ON CONFLICT DO NOTHING` (deferred target FK, see data model); read the existing row after any conflicting
insert completes. If owned by another actor or different payload return 409
`request_conflict` without disclosing its stored response. If same and complete,
return replay before live-version/stale-review checks. If newly inserted, lock
and validate all relevant state, rebuild preview and compare; then post exact
inverse/replacement, allocations/settlement metadata and completed response before
commit. A failed confirmation rolls back reservation too. Never catch a PostgreSQL
unique violation and continue in an aborted transaction; use ON CONFLICT.

The request payload hash includes the accepted fingerprint. Canonical hash
comparison ignores input array order and whitespace normalization only, not changed
business contents. A network timeout is an uncertain response: retry exactly the
same confirm payload/ID before generating a new review. Store client payload until
success or a definitive rejection; don't silently substitute a newly fetched digest.

## Errors and supported legacy paths

Use existing `{code,message,details?}` errors. 400 `invalid_input`; 401 unauthenticated;
403 `forbidden`; 404 `not_found`; 409 `preview_required`, `preview_changed`,
`request_conflict`, `transaction_already_reversed`, `transaction_changed`,
`assets_pending`, `invalid_stock`; 422 `no_change`, `unsupported_transaction`,
`legacy_correction_unsupported`, `reversal_not_allowed`. Existing shuttle shortfall
code/details stay compatible. Relevant error details may provide fresh record links,
not silently refreshed confirmation. No rejected operation writes financial state.

When each supported type ships, legacy `/admin/transactions/:id/reverse` and
`/admin/settlements/:id/reverse` accept the reviewed request envelope and delegate
to this orchestration for that type. Without fingerprint/request/reason they return
`409 preview_required` and no write; update client wrappers, OpenAPI and tests in
the same release. Session reviewed-reversal support lands with the foundation so
there is no unreviewed session bypass while correction is still pending. Types
outside this feature retain their previous policy (including existing protected
import/audit restrictions and asset-purchase correction endpoints). Do not blanket
route every kind through the new member flow. Shared kind must be recognized by
legacy dispatch before its first record can be created.
