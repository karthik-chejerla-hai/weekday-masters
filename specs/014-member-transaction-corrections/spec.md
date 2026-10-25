# Feature Specification: Transaction corrections and shared expenses

**Feature directory**: `specs/014-member-transaction-corrections`

**Created**: 2026-10-25

**Status**: Reviewed draft with implementation baseline audited 2026-10-25.
Existing completed work is marked in [tasks.md](tasks.md). PR 1 reversal safety
is implemented and validated; later feature work has not started. Proposed
defaults below remain subject to product review.

**Input**: “Add an ability to correct or reverse a txn - session charge, or a
 topup” and record non-badminton expenses, adjusting each player's balance
according to who paid and who owed.

## Existing implementation baseline

The ledger, reversal primitive, session costing, shared member history and
personal spend already exist. Asset audits and asset-purchase corrections have
also shipped. Reuse these capabilities; do not rebuild them for this feature.

Completed baseline items D001–D015 and feature work R001–R014 are tracked in
[tasks.md](tasks.md). [implementation-status.md](implementation-status.md) maps
each requirement to shipped PRs, source/tests and the exact remaining gap.
“Partial” means the existing behavior covers only part of a requirement; it is
not an instruction to recreate that behavior. In particular, existing backend
reversal support is complete as a primitive, while its new reviewed member UI,
mandatory reasons and retry protocol remain. Atomic session-state coordination
and reversal-of-reversal rejection are now complete in PR 1.

## User Scenarios & Testing

### User Story 1 — Correct or reverse a top-up (Priority: P1)

An approved admin finds a top-up in Money, corrects a mistake or reverses the
record, and reviews the effect before confirming. Members can understand the
reason without losing the original record.

**Why this priority**: A wrong amount or wrong recipient directly affects both
member credit and recorded bank funds.

**Independent Test**: Record a top-up, correct its amount and recipient, then
reverse the replacement and reconcile both member and bank balances.

**Acceptance Scenarios**:

1. **Given** Alex received a recorded $50 top-up, **when** an admin corrects it
   to $40 with a reason, **then** review shows Alex -$10 and bank -$10; saving
   applies exactly those net changes and retains the original and its reversal.
2. **Given** a $50 top-up was assigned to Alex instead of Bea, **when** corrected
   to Bea, **then** Alex decreases $50, Bea increases $50, and bank has no net
   change. Both members' before/after balances are shown.
3. **Given** Alex has since spent the credit, **when** the top-up is reversed,
   **then** the exact original member and bank movements are undone, even if
   Alex becomes in debt. No bank transfer or refund is initiated.
4. **Given** the current record is already reversed or corrected, **when** an
   admin tries to change that same version, **then** it cannot be changed again;
   the interface links to its current replacement or reversal.

### User Story 2 — Record a shared meal or other expense (Priority: P1)

An admin records a group bill against the existing Rally member balances,
identifying who paid and who benefited. They can check an equal or custom split
before saving.

**Why this priority**: This replaces manual balance adjustments with one
explainable record of an expense.

**Independent Test**: Record the $120 dinner example and reconcile each member's
balance and club assets; repeat with multiple payers and club funding.

**Acceptance Scenarios**:

1. **Given** Alex personally paid $120 for Alex, Bea, Chris and Dev, **when**
   split equally, **then** each owes $30, Alex gains $90 credit, the others each
   lose $30 credit, and bank, court credit, shuttles and surplus stay unchanged.
2. **Given** Alex paid $80 and Bea paid $40 for the same four-person dinner,
   **when** saved, **then** Alex gains $50, Bea gains $10 and the others each
   lose $30. Paid amounts and shares remain separately visible.
3. **Given** the club bank paid that $120 bill, **when** split among four people,
   **then** bank decreases $120 and each member decreases $30. Review identifies
   club funding explicitly; no member is credited as the payer.
4. **Given** a $10 bill split equally three ways, **when** reviewed and saved,
   **then** shares are $3.34, $3.33 and $3.33 in a stable order, total $10 exactly,
   and retries or selection order cannot change who receives the extra cent.
5. **Given** a payer did not attend, or a member paid exactly their share,
   **when** recorded, **then** payer-only and zero-net participants remain in
   the breakdown and in their personal history.
6. **Given** incorrect paid/share totals or incomplete details, **when** an
   admin attempts review, **then** the form identifies the discrepancy and no
   balance changes are recorded.

### User Story 3 — Correct or reverse a native session charge (Priority: P1)

An admin corrects attendance, extra-hour participation or recorded costs on a
settled native session. They review all affected members and assets together.

**Why this priority**: One participant's correction can change everyone else's
share and cannot safely be treated as a standalone balance edit.

**Independent Test**: Correct a settled session after later sessions and asset
purchases exist; verify exact net changes and unchanged later settlements.

**Acceptance Scenarios**:

1. **Given** a $120 session split among four members, **when** an incorrectly
   included fourth member is removed, **then** the remaining three owe $40 each,
   the removed member regains $30, and court/shuttle totals remain unchanged.
2. **Given** only attendance changes after shuttle prices or club defaults
   changed, **when** corrected, **then** original cost totals, units and rates
   are retained; current defaults do not reprice the session.
3. **Given** a session used six shuttles valued at $24, **when** corrected to
   four, **then** its new shuttle cost is $16 and stock regains two units and $8.
   In a separate increase case, if current stock is twelve units valued at $60,
   correcting six to eight costs $34: original $24 plus $10 for two extra units.
4. **Given** an active settlement, **when** reversed, **then** all original
   member charges, asset usage and any comped-share surplus movement are undone
   together and the session becomes eligible for settlement again.
5. **Given** any failure, duplicate submission or competing correction, **when**
   confirming, **then** the result is one complete operation or no change; a
   session cannot be left with reversed money and an active old settlement.
6. **Given** a legacy estimated settlement or hosted guest/comped line, **when**
   opened for correction, **then** its recorded meaning and line details survive;
   it is not silently converted to an actual-count expense.

### User Story 4 — Correct a shared expense (Priority: P2)

An admin corrects the bill, payers, participants, allocation, funding source,
category, date or description using the same review flow.

**Why this priority**: Shared bills need the same traceable correction experience
as top-ups and session charges.

**Independent Test**: Change a personally funded bill to club-funded, then
reverse the replacement; reconcile every balance and retained allocation.

**Acceptance Scenarios**:

1. **Given** a $120 four-person meal paid by Alex, **when** changed to club-paid,
   **then** review shows Alex -$120, bank -$120 and the other members no net
   change; saving retains all original paid amounts and shares in history.
2. **Given** a recorded meal, **when** reversed, **then** its exact member and
   bank effects are undone; no money is sent and its reason stays visible.
3. **Given** a correction or reversal changes no balances (for example category
   only), **when** confirmed, **then** the revision remains traceable without
   inventing a cash movement or dropping zero-net allocations.

### User Story 5 — Understand money history (Priority: P2)

Approved members can find their expenses and understand gross shares, payments,
net changes and corrections without inspecting accounting entries.

**Why this priority**: The payer's share cannot be inferred from their net credit.

**Independent Test**: Browse personal/all-member views and each type filter,
including corrected, reversed and zero-net expenses across several pages.

**Acceptance Scenarios**:

1. **Given** a dinner, **when** viewing history, **then** one expandable row per
   recorded expense version shows title, bill, date, payer summary, participant
   count, and the caller's share/net change. Expansion shows Paid, Share, Change
   and Balance after for each affected member.
2. **Given** a correction, **when** reading history, **then** the original is
   marked Reversed/Corrected, its reversal and replacement are linked, and only
   the replacement counts as an active expense. Actor, reason and recording
   times are accessible from either record.
3. **Given** a meal optionally linked to a session, **when** viewing personal
   badminton spending, **then** the meal is excluded. Session corrections and
   reversals update badminton spending using the original Sydney play date.
4. **Given** an unapproved member or non-admin, **when** attempting a money
   mutation by any available route, **then** access is refused without changes.

### Edge Cases

- A lost response followed by a retry returns the original successful result;
  reusing its submission identity with different contents is rejected.
- A balance, stock position, membership status or original transaction changes
  after review: require refreshed review before confirming affected outcomes.
- A member becomes inactive: exact reversal remains possible. A correction may
  retain or change an original member's allocation, including an inactive
  original member, but newly introduced members must be approved. Show inactive
  status explicitly; do not substitute a different person with the same name.
- Everyone paid exactly their own share: retain the shared bill and allocations
  even though all balance changes are zero.
- An amount/share is negative, not exact to a cent, too large, or totals overflow:
  reject the input before any change. Zero paid/share amounts are permitted for
  relevant members; the bill and total owed must be positive.
- Empty selection, duplicate members, no payer for personal funding, missing
  description, future Sydney expense/top-up dates or an invalid session link:
  reject with an actionable message. Sessions must already be settleable.
- A decrease in shuttle usage refunds its original recorded value; an increase
  fails when current stock cannot supply the additional units. A correction or
  reversal must not leave negative units/value or value with zero stock.
- A full reversal can occur after the member was removed or other money moved.
  It must use the original amounts, not a reconstruction from current rates.
- Imported transactions, opening balances, withdrawals, asset snapshots and
  reversal transactions are not editable through this feature.

## Requirements

### Functional Requirements

- **FR-001**: Only approved admins may create shared expenses or correct/reverse
  supported transactions. All approved members may read shared breakdowns and
  their correction history, consistent with existing money visibility.
- **FR-002**: Money → Balances MUST offer Record shared expense alongside Record
  top-up. The form MUST ask description, Sydney date, Food/Other category, bill,
  funding, payers and participants, then show a separate confirmation review.
- **FR-003**: Personal funding MUST support one or multiple member payers. Club
  bank funding MUST be an explicit alternative. Mixing bank and personal funding
  in one expense is out of scope for this version.
- **FR-004**: The form MUST default to equal shares and allow exact individual
  amounts. Paid amounts and owed amounts MUST each sum to the bill, with club
  payment represented separately for bank-funded bills. Splits MUST be exact to
  a cent with deterministic remainder allocation independent of selection order.
- **FR-005**: For personal funding, member change MUST equal paid minus owed and
  total member change MUST be zero; club assets and surplus MUST be unchanged.
  For bank funding, each member change MUST be minus owed and bank MUST decrease
  by the bill; other assets and surplus MUST be unchanged.
- **FR-006**: The review MUST show every affected member's Paid, Share, net Change
  and Current → New balance, including both old/new members in corrections,
  plus each affected club asset and any surplus movement. It MUST explain that
  recording or undoing an entry does not transfer money.
- **FR-007**: New expenses MUST use approved existing members. Historical
  corrections MAY retain inactive members already on that exact original record;
  no newly introduced inactive member may receive a payment/share. Reversals
  MUST remain possible regardless of original members' current status.
- **FR-008**: An optional native-session link MAY suggest participants only when
  explicitly requested. It MUST NOT change RSVPs, scores or badminton costs.
  A guest's cost may be included in a selected member's explicit share; separate
  guest accounts, non-member debt and guest invoicing are out of scope.
- **FR-009**: Top-up correction MUST allow member, amount, date and note changes.
  Session correction MUST allow the original editor's attendance, hosted guests,
  comped lines, base/extra-hour participation and cost inputs, retaining the
  distinction between legacy estimated and actual-count expenses.
- **FR-010**: Shared-expense correction MUST allow every recorded expense field
  to be corrected, including funding and allocation. All corrections MUST open
  from the recorded current version and MUST require a reason of 1–1,000
  characters after trimming surrounding whitespace. No-change corrections MUST
  be rejected; metadata-only changes are legitimate revisions.
- **FR-011**: Correction MUST atomically undo the exact original effects and
  record a linked replacement. Original entries and allocations MUST remain
  immutable. The replacement MAY later be corrected using the same process.
- **FR-012**: Reversal MUST require a reviewed exact inverse and mandatory reason
  with the same limits as correction. An original may be reversed once; reversal
  of a reversal is not supported. A previously corrected original cannot be
  corrected/reversed again; its current replacement is the actionable record.
- **FR-013**: Native-session operations MUST act on the entire settlement. Money,
  old/new settlement state and correction links MUST change together. Reversal
  makes a session unsettled; correction leaves exactly one live settlement.
- **FR-014**: Attendance-only session corrections MUST retain original asset
  consumption and per-band cost totals. Reallocation MUST preserve the existing
  extra-hour, largest-remainder, hosted-guest and comped-share rules; any change
  to surplus due to comping MUST be included in review.
- **FR-015**: Changed session costs MUST default to recorded rates and inputs,
  never current club defaults. Correcting one session MUST NOT reprice later
  sessions. Shuttle valuation MUST follow the policy below; arbitrary typed
  shuttle unit prices or an unrelated player's balance adjustment are forbidden.
- **FR-016**: Supported new writes MUST enforce balanced movements and the club
  identity: bank + court credit + shuttle value = member balances + surplus.
  Debt/negative member balances are permitted and MUST be shown clearly. This
  feature introduces no new nonnegative-bank/court-credit policy.
- **FR-017**: Every mutation MUST be safe to retry, with one durable result per
  submission identity and contents. Competing operations on a current version
  MUST yield at most one success. Failed operations MUST leave no partial money,
  expense allocations, stock changes or settlement state.
- **FR-018**: Confirmation MUST bind the reviewed inputs, current version and
  affected balance/asset outcomes. Changed relevant state MUST cause a refresh
  and explicit renewed review, rather than posting different values. Resolving
  an already successful identical retry MUST precede stale-review rejection.
- **FR-019**: History MUST retain original, reversal and replacement records with
  links, readable status, actors, reasons, occurrence dates and recording times.
  Reversal and replacement MUST be distinguishable from active original spending.
- **FR-020**: Expense versions MUST group all their member allocations before
  personal/type filters and pagination. Payers and participants, including
  zero-net members, MUST find the record in personal history. Full detail MUST
  survive filtering; an expense MUST NOT split across pages by member.
- **FR-021**: History MUST offer All, Sessions, Top-ups and Shared expenses type
  filters plus the existing personal/all-member selection. All MUST retain
  existing imported and miscellaneous history. Reversals MUST follow their
  original type filter and never be inferred as top-ups from positive amounts.
- **FR-022**: Running balances MUST use full effective account history before
  filters and pagination. For these new member correction/reversal operations,
  the inverse MUST use the original movement's effective occurrence timestamp;
  the replacement MUST use its corrected occurrence timestamp (original by
  default). The actual correction recording time MUST always be shown separately.
  Backdated changes therefore recompute effective historical balances; explain
  that these are not snapshots of what the app displayed before correction.
- **FR-023**: Session play date MUST remain its session's Sydney date, independent
  of when a correction is recorded. Shared expenses MUST be excluded from the
  badminton spend chip and analytics, even when linked to a session. Only live
  replacement session charges MUST count toward that spending.
- **FR-024**: Successful mutations MUST refresh displayed member balances, club
  position, money history and affected spend summaries without a manual reload.
  Cancelling review MUST make no persistent change. Errors MUST retain entered
  details and provide a safe retry or refresh path.
- **FR-025**: Imported history, opening balances, withdrawals and audited asset
  snapshots MUST retain existing restrictions. Existing asset-purchase correction
  workflows MUST continue to work. No generic undo route may bypass type-specific
  settlement state or historical restrictions.
- **FR-026**: This version MUST NOT send new email/push alerts for shared expenses
  or correction/reversal operations. Ordinary existing settlement notifications
  are unaffected. No intermediate undo state may generate a notification.

### Shuttle valuation policy

The policy preserves already recorded cost for unchanged usage. It values only
extra usage from stock held at correction time; it does not recost the complete
session at today's average price.

- For an actual-count settlement, use the recorded whole-session consumed units
  and value as one pair. Unchanged count retains the exact original total value.
- A reduced count receives the proportional original value, rounded to the
  nearest cent, halves upward; zero count has zero value. The difference in
  units and cents returns to stock. Never store a rounded per-shuttle price.
- An increased count retains the complete original value and adds the value of
  only the extra units, consumed proportionally from current stock with the
  existing nearest-cent rule. Using all remaining stock consumes all its value.
- After valuing the whole actual count, distribute its value across played
  hours using the original actual-count rules (2:1 for three hours), then split
  each band among its own participants. With everyone staying, retain the
  existing single combined split rule.
- For legacy estimated settlements, apply the unchanged/reduced/increased rule
  separately to each recorded band's unit/value pair, processing base then
  extra against the remaining correction-time stock. Returned units/value are
  available to later bands. Preserve legacy guest and comped-share behavior.
- A later correction uses its immediate current replacement's recorded pairs.
  Exact reversal always restores that version's exact posted units/value.
- If retained legacy data cannot support the required calculation, refuse the
  correction with an explanation; never guess a price or mutate earlier history.

### Key Entities

- **Financial transaction**: A recorded event with type, effective occurrence,
  recording time and recorder; original/reversal/replacement relationships and
  mandatory correction reason preserve its audit trail.
- **Shared expense**: Description, category, date, positive bill, funding choice,
  optional session context and immutable allocations for one recorded version.
- **Member allocation**: The member's actual paid amount, gross share and derived
  net movement; retained even when net movement is zero.
- **Settlement**: The native session's recorded inputs, participant lines, asset
  consumption and live/reversed state; corrected as a whole.
- **Reviewed change**: The current version and proposed inputs plus the exact
  member/asset effects an admin confirms; invalidated by relevant state changes.
- **Submission identity**: Distinguishes a retry from a new operation and binds
  one successful outcome to one set of confirmed contents.

## Success Criteria

### Measurable Outcomes

- **SC-001**: An admin can record a four-person, single-payer equal dinner split
  in at most two minutes on a mobile screen, including review, without editing
  any person's balance individually.
- **SC-002**: Every accepted expense's paid and owed totals equal its bill to
  the cent; every accepted correction/reversal reconciles all member and club
  changes exactly. All acceptance fixtures satisfy the club-position identity.
- **SC-003**: Every corrected/reversed record remains traceable to its original,
  actor, reason and recording time from its details, with no overwritten history.
- **SC-004**: Repeating one confirmed submission ten times creates exactly one
  operation; two competing changes to the same original yield at most one
  success and never a partial financial or settlement result.
- **SC-005**: All personal/type/paged-history acceptance cases show the complete
  correct breakdown and effective running balances, including zero-net members;
  meals change badminton spending by exactly zero.
- **SC-006**: All unauthorized and unsupported-type acceptance cases produce
  zero persistent changes; cancelling any review also produces zero changes.
- **SC-007**: Attendance-only corrections change asset totals by exactly zero;
  changed-count fixtures match the valuation examples and leave every later
  settled session's recorded charges unchanged.

## Assumptions

- Admin-only money entry follows current Rally permissions; members can read
  all existing shared money history. Member self-service proposals/approvals
  are outside this version.
- All amounts are Australian dollars, represented exactly in cents. Dates and
  future-date checks use Australia/Sydney. Reason text is trimmed before the
  length limit is applied. Names follow existing display-name conventions;
  duplicate display names need a secondary identity cue for selection.
- Equal splits reuse the existing deterministic expense-specific member ordering
  to award remainder cents; the same expense retains its allocation across
  review and retries, independently of selection order.
  New expenses allow one member or more; even a one-member or all-zero-net
  allocation remains a valid recorded expense.
- Food and Other categories, multiple personal payers, club bank funding,
  unequal shares and optional native-session context are proposed defaults.
  Mixed funding, percentage/weighted splits, receipts, recurring expenses,
  non-member accounts, balance transfers unrelated to an expense, automated
  refunds, exports and new notification flows are out of scope.
- Correction effective-date behavior in FR-022 applies to this feature's member
  transactions. It does not retrospectively rewrite existing corrections or
  change the existing asset-purchase correction workflow.
- Existing imported-history restrictions and audit workflows remain dependencies;
  this feature does not create a new imported-history correction mechanism.
- The user subsequently authorized PR 1 reversal safety. Its scope and completed
  implementation are recorded in pr1-reversal-safety.md. Later feature flows and
  production financial operations are not part of that implementation.
