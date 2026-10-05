# Feature Specification: Doubles game scores

**Feature Branch**: `feat/issue-39-game-scores`
**Created**: 2026-10-05
**Status**: Ready for planning
**Input**: Issue #39, record spoken game results and settle head-to-head claims.

## User Scenarios & Testing

### User Story 1 - Record a doubles result (Priority: P1)

An approved member opens a session, speaks four names and a score, reviews the teams and score, and saves one completed game. A form works when voice is unavailable.

**Why this priority**: Results are the source of every comparison.
**Independent Test**: Record a 21–17 game with four distinct members and find it after reloading.

**Acceptance Scenarios**:
1. Given a session that has started, when a member enters four different approved members and two unequal whole-number scores from 0 to 99, then Save records one doubles game.
2. Given a clear spoken result, when the assistant resolves the names, then it offers a review form without writing a result.
3. Given missing or ambiguous names or scores, then the assistant requests clarification and does not invent a player or score.
4. Given a retry of the same save, then the system retains one game. A separate game with the same teams and score is allowed.
5. Given unavailable voice, then manual entry remains usable.

### User Story 2 - Correct the record (Priority: P2)

The recorder or an admin can correct or void a result. All members can see who recorded and changed it.

**Why this priority**: Entry errors must not become disputed statistics.
**Independent Test**: Correct a score, inspect its history, void it, and confirm it no longer contributes to comparisons.

**Acceptance Scenarios**:
1. The recorder and admins can change players or scores and void mistaken results; other members cannot.
2. Concurrent corrections to the same version cannot silently replace each other.
3. Every saved version retains teams, scores, actor and time. Voided results remain in session history with a clear label.

### User Story 3 - Compare opponents (Priority: P2)

Members compare one player against another or one exact doubles team against another. They see wins, losses, points and paged game history across sessions.

**Why this priority**: This provides the evidence for club banter.
**Independent Test**: Seed games with changing partners; verify player comparisons count opposing sides only and exact-team comparisons ignore other partners.

**Acceptance Scenarios**:
1. Player comparisons include a game once only when the players are on opposing sides.
2. Exact-team comparisons ignore the order of partners within each team.
3. Wins, losses and points match all active games, regardless of pagination. Corrections update the totals; voided games do not count.
4. Empty comparisons show no games, not an error. Historical results remain readable after member removal.

### Edge Cases

Reject repeated players, missing scores, ties, negative or fractional scores, future or cancelled sessions, and stale edits. Preserve results when a session has results and an admin attempts deletion. Repeated speech with the same names is resolved by explicit selection or follow-up, never first-match selection. Network errors keep the reviewed entry available for retry.

## Requirements

### Functional Requirements

- **FR-001**: Only approved members can read game records or comparisons and create results.
- **FR-002**: Each game belongs to one scheduled session and contains exactly two teams of two distinct club members and one unequal score pair.
- **FR-003**: Voice is prominent on the session score page. Typed messages and a manual form are available. Speech and assistant conversation are not retained by Rally.
- **FR-004**: A spoken result requires explicit review and Save. The assistant cannot write results.
- **FR-005**: Save retries are idempotent. Independent identical games remain possible.
- **FR-006**: Only the recorder and approved admins can correct or void results, with version checks and visible history.
- **FR-007**: Player and exact-team comparisons show wins, losses, points and paged history across all recorded sessions.
- **FR-008**: Displays use member display names and Sydney session dates. Historical records survive removal of members and prevent session deletion.
- **FR-009**: Game recording does not change RSVPs, expenses or money.
- **FR-010**: Tests must cover access, score validation, concurrency, voice preview, corrections and head-to-head calculations.

### Key Entities

- **Game**: Session, two pairs of members, scores, recorder, current version and void status.
- **Game revision**: Saved version, actor and time, retained for audit.
- **Comparison**: Two opponents or two exact teams, totals and matching game history.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A clear spoken result requires no manual player or score entry and only one explicit Save after review.
- **SC-002**: Every accepted retry of a save produces exactly one game.
- **SC-003**: Comparison totals agree exactly with recorded, non-voided games in test fixtures, including changing partners and reversed team order.
- **SC-004**: Every correction identifies its actor and preserves the prior result.

## Assumptions

- First release supports approved club members, including invited members. Guest identities, singles, tournaments and multi-game matches are outside scope.
- Scores use club rules: unequal integers from 0 to 99. Tournament target and deuce rules are not enforced.
- A session must have started. Past sessions can receive results. RSVP status does not limit score entry.
- Network access is required. Speech uses the existing configured provider. Manual entry needs no provider key.
- Voiding is final; a replacement game can be recorded if needed.

## Clarifications

### Session 2026-10-05

- Q: Supported game format? A: Doubles only, one score per game.
- Q: Save and correction rights? A: Any approved member can save; recorder and admins can correct; no opponent approval.
- Q: Comparison scope? A: Player against player and exact doubles teams, with wins, losses, scores and game history.
