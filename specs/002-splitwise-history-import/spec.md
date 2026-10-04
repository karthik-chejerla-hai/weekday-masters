# Feature Specification: Splitwise history import

**Feature Branch**: `feat/set-up-prod`
**Created**: 2026-10-04
**Status**: Implemented and imported in production. Notifications remain paused.

## User Scenarios & Testing

### User Story 1 - Import exact history without sending messages (Priority: P1)

The administrator imports the full export into an isolated local database before production. Only the eight explicitly mapped people receive user records. Every original balance movement remains attributable to its source name.

**Independent Test**: Import a reconciled export, compare every posted movement and all closing balances, then import it again. The second run must create nothing.

**Acceptance Scenarios**:
1. Given a complete name-to-email mapping, importing creates claimable member records without invitations, reminders, or push messages.
2. Given an unmapped former participant, their history shows their original name and an inactive label, with no user record or sign-in identity.
3. Given a malformed or unbalanced export, the import refuses all writes.
4. Given any already-posted history, a different or overlapping import refuses to modify it.

### User Story 2 - Read historical session charges (Priority: P2)

Members can view the original game descriptions and exact allocated costs. Imported history does not need RSVPs or fictional court rates, times, or shuttle consumption.

**Independent Test**: Open an imported session with an inactive participant and compare its displayed amounts to the export.

**Acceptance Scenarios**:
1. Game and extra-hour rows appear as historical sessions with the source description and title play date.
2. An explicitly reviewed payer allocation records both their payment and their own expense share.
3. Other expenses, payments and adjustments remain in the personal ledger but do not become badminton sessions.
4. Imported sessions cannot be processed by the live settlement calculator.

### User Story 3 - Review assets before invitations (Priority: P3)

The administrator supplies the bank balance, court credit, shuttle count and stock value at the export cutoff. Missing asset values remain explicitly unverified.

**Independent Test**: Record the asset snapshot and prove player balances are unchanged and the club identity is still zero.

**Acceptance Scenarios**:
1. The test import succeeds before asset figures are available, while the review reports that asset verification is pending.
2. Recording assets never replays historical charges or replaces ledger entries.
3. All notification delivery stays paused until setup review is complete.

### Edge Cases

- CSV net balances are not always gross expense shares; a member may also have paid the bill.
- Original descriptions and recorded dates can disagree. Preserve both; do not guess missing play times.
- Former participants can have zero closing balances but non-zero historical activity.
- Repeated imports, email collisions, changed source data, mixed currency, and non-zero inactive balances fail visibly.

## Requirements

- **FR-001**: Preserve every original transaction and all non-zero participant movements, in integer cents.
- **FR-002**: Create users only for supplied, uniquely mapped email addresses. Match existing members by verified identity conventions.
- **FR-003**: Retain unmapped people as inactive historical participants without user records.
- **FR-004**: Check every transaction, every participant total, every session allocation, and the final club identity automatically.
- **FR-005**: Import atomically and prevent duplicate or overlapping application.
- **FR-006**: Pause all delivery before newly imported members become visible to notification jobs.
- **FR-007**: Keep raw exports, mappings and reconciliation reports out of version control.
- **FR-008**: Preserve past session charges independently of the current costing rules, with no RSVPs.
- **FR-009**: Distinguish unverified assets from confirmed balances. Record asset confirmation as a new ledger transaction.
- **FR-010**: Provide a review report before any production import or invitation sending.

### Key Entities

- Import batch: source identity, mapping identity, verification state and asset confirmation.
- Historical participant: source name, optional current member identity, inactive state and balance.
- Source record: original date, title, category, cost, exact changes and optional session allocation.

## Success Criteria

- Every imported member balance equals the source closing balance to the cent.
- Every source movement has exactly one matching posted ledger movement.
- Every session's allocated charges total its original cost.
- Only the eight supplied current members have user records in the isolated rehearsal.
- Importing twice changes no counts or balances.
- No notification or invitation is sent during setup.

## Assumptions

- Local rehearsal passed review. Production preparation and replication are authorized. Sending invitations requires a separate explicit confirmation.
- The source cutoff is the export's total-balance date.
- Asset figures are a later input, not guessed from the club mirror column.
- Pagination beyond the current 50 records is a separate follow-up, as requested.
- The user selected play dates from titles. Date-less titles use recorded dates and are flagged. The first member-funded session payer was explicitly confirmed.
