# Feature Specification: Session expense assistant

**Feature Branch**: `feat/issue-40`
**Created**: 2026-10-05
**Status**: Clarified with owner
**Input**: GitHub issues 40, 39 and 31, and the owner's answers in this session.

## User Scenarios & Testing

### User Story 1 - Record a played session (Priority: P1)

An admin records two or three hours and the actual number of shuttles used. The
session's confirmed RSVPs supply the initial participants, including no-shows.
The admin can change this list and select who stayed for the extra hour. When
everyone stays, all selected players pay an equal whole-session share. An early
leaver pays only the first two hours of court hire and their shuttle share.

**Why this priority**: The club needs accurate charges, court credit and stock.
**Independent Test**: Record a seeded past session using the form without an AI key.

**Acceptance Scenarios**:
1. Given a finished session, entering hours and shuttle count produces a preview
   with participants, individual charges, total and remaining assets.
2. Previewing changes no money. Explicit confirmation records the displayed split.
3. Duplicate confirmation records one settlement. Changed rates or stock require
   a new preview. Shares sum exactly to the cost. Stock never becomes negative.
4. All approved members see all outstanding past sessions above Next game, with
   an expense action only for admins. Cancelled and already settled sessions do
   not appear. The list includes correct RSVP counts and older outstanding sessions.
5. The court-credit warning covers the standard two-hour booking.
6. Tests cover two hours with five or six confirmed RSVPs, three hours with five,
   and three hours with six where one leaves after two hours. In the last case,
   allocate shuttle value 2:1 by time, then share each band only among its players.
   Each test starts after a previous expense and checks remaining court credit,
   shuttle units and shuttle value, including a purchase at a different unit cost.

### User Story 2 - Speak or type an expense request (Priority: P1)

The admin taps to record, speaks, reviews the recognised text, and sends it to the
assistant. Text input works too. The assistant asks for missing information and
returns the same expense preview as the form. The admin presses Confirm to save.

**Why this priority**: Voice is the owner's preferred input method.
**Independent Test**: Use a scripted provider in tests, then a real key for a local smoke test.

**Acceptance Scenarios**:
1. A request such as “We played three hours and used eight shuttles” on a selected
   session prepares an equal split. Names and dates refer to real club records.
2. Missing counts, ambiguous sessions or ambiguous names produce a question.
3. Recording failure, missing configuration or provider quota exhaustion leaves
   text/form alternatives available with clear status and retry guidance.
4. The assistant cannot save a settlement from a spoken or typed instruction.
   Only the authenticated admin's separate confirmation can do so.
5. A request such as “Jordan left after two hours” keeps Jordan in the main group
   and excludes Jordan from the extra-hour group. The review shows each player's
   hours, and editing in the form preserves this selection.

### User Story 3 - Ask about club data (Priority: P2)

Approved members ask about upcoming/past sessions, participants, player balances,
court credit or shuttle stock. Answers use current club records.

**Why this priority**: This supplies the common assistant for future features.
**Independent Test**: Ask a balance and stock question as a non-admin.

**Acceptance Scenarios**:
1. Answers use retrieved data and respect the caller's existing access.
2. Unsupported actions are explained without claiming that data was saved.
3. Later score recording can use the same assistant. Any approved member will be
   able to record results; admins will audit and correct them under issue 39.

### Edge Cases

Zero shuttles, no confirmed players, removed players, duplicate names, zero stock,
invalid numbers, no upcoming session, several outstanding sessions, reversals,
concurrent settlements, stale previews, microphone denial, recording cancellation,
provider errors, short audio, quota exhaustion and malformed action requests.

## Requirements

- **FR-001**: Retain the existing settlement history and append-only ledger rules.
- **FR-002**: Use integer cents and actual stock use. Consume the full shuttle count
  once, then allocate its value 2:1 between the two standard hours and one extra
  hour with largest-remainder rounding. Split each band's court and shuttle cost
  among only its participants. If everyone stays, split the total once so shares
  differ by at most one cent. Keep the first two hours selected for early leavers.
- **FR-003**: Offer a complete form independently of voice and the provider.
- **FR-004**: Support voice/text, follow-up questions and explicit review/confirmation.
- **FR-005**: Support approved-member data questions with admin-only expense preparation.
- **FR-006**: Share the assistant across features and allow independent provider replacement.
- **FR-007**: Show all outstanding past sessions above the next session for both roles.
- **FR-008**: Supply local synthetic members, a past session with RSVPs and sufficient assets.
- **FR-009**: Keep credentials on the server. Do not persist recordings or send notifications in local testing.
- **FR-010**: Test money invariants, concurrent writes, route permissions and provider failures.

### Key Entities

Expense preview (session, hours, actual shuttles, participants, charges, remaining
assets and version); settlement (immutable recorded expense with reversal support);
assistant conversation (temporary text turns and selected session); supported action.

## Success Criteria

- **SC-001**: An admin can preview and confirm a seeded session locally using either input mode.
- **SC-002**: Every recorded split reconciles exactly with assets and player balances.
- **SC-003**: Repeated confirmation produces exactly one recorded expense.
- **SC-004**: No outstanding past session disappears because newer sessions were settled.
- **SC-005**: A member can obtain club balances without gaining expense permissions.

## Assumptions

English input initially. Two or three total hours. Actual shuttle use is a whole
number, including zero. Existing advanced band settlements remain compatible.
The first release builds expenses and club questions; scores ship separately.
The owner selected Groq and will supply the key after implementation. We verify
the integration with a test provider until then. Local sign-in uses existing Auth0.
