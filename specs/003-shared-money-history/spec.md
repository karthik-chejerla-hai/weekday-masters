# Feature Specification: Shared assets and member history

**Feature Branch**: `feat/set-up-prod` (existing work preserved)
**Created**: 2026-10-04
**Status**: Implemented and verified locally

## User Scenarios & Testing

### User Story 1 - See club assets (P1)

An approved member sees court credit, shuttle count and value, and bank funds in three cards. Desktop shows one row. Mobile shows three compact rows without horizontal scrolling. Each card gives the date of its data. Only an admin sees or can use purchase forms.

Acceptance: Compare both roles against the same confirmed asset snapshot. Both see identical figures and dates. A member cannot record a purchase. A pending or removed member cannot read club finances.

### User Story 2 - Read and filter the ledger (P1)

The ledger starts with the current member's entries. One switch selects all member entries. A second switch shows only top-ups. Each row shows the actual title, its type icon, and a Splitwise source badge only for imports. The all-member view identifies the member and marks inactive participants.

Acceptance: A game keeps its original title. A food expense has a food icon. A payment for a game does not become a top-up merely because it credits its payer. Filtered rows keep their full-history running balance.

### User Story 3 - Reach older transactions (P1)

A member can keep loading older records beyond the first 50 without losing the current filters.

Acceptance: Load a history with more than 100 entries to its end. All entries remain reachable, in order, without duplicates. Failed page requests can be retried. Changing filters resets the page.

### Edge Cases

- No date or physical stock audit exists: show that it is not recorded.
- A purchase or consumption changes stock after its last audit: show both audit and movement dates.
- Empty results, failed reads, fast filter changes, reversed entries, inactive participants without user accounts, and equal transaction times.

## Requirements

- FR-001: Share read-only asset information with every approved member.
- FR-002: Keep all money writes restricted to admins.
- FR-003: Show court credit, shuttle value/count, and bank funds in that order. Desktop shows one row; mobile shows three compact rows without horizontal scrolling. All three cards highlight the dollar value. The shuttle count appears in the secondary line.
- FR-004: Show bank and court dates from their last recorded movement. Show the shuttle audit date from the confirmed opening/cutover snapshot, and its last movement date separately.
- FR-005: Default to the current member's ledger. Allow all member entries, including inactive participants, and a top-up-only filter.
- FR-006: Preserve complete running balances before filters and pagination.
- FR-007: Use source titles and type icons, with a distinct Splitwise badge for imported history.
- FR-008: Load records beyond 50 with an accessible control, progress, retry, and end states.
- FR-009: Do not alter money, user identities, imported data, or notification settings.

### Key Entities

- Asset position: current funds and stock, last movement dates, last confirmed stock audit.
- Member ledger entry: owner, title, source, transaction type, amount, and historical balance.

## Success Criteria

- SC-001: Both roles see the same three asset values and dates.
- SC-002: All existing history is reachable with either member scope and either top-up filter state.
- SC-003: Every filtered running balance matches the same entry in the unfiltered ledger.
- SC-004: Automated checks reject unauthorized financial reads and writes.

## Assumptions and Clarification

- “All” means all member-account entries, including former participants. It does not duplicate bank and surplus accounting legs.
- Explicit top-up descriptions and incoming Splitwise payments to the club identify imported top-ups. Credits for supplied goods and payments between members are distinct.
- A confirmed import or opening stock snapshot is an audit. A later game or purchase is not a physical audit.
- No new audit entry form is requested. Unknown audit dates remain unknown.
- Deployment and invitations are outside this change. Notifications remain paused.

## Revision: One expandable entry per game

Requested after reviewing the all-member ledger. Replace separate account rows for a game with one collapsed game row. Expansion shows each charged account's share, any amount paid on behalf of the group, and its balance after the last posting for that game. Label these figures explicitly. The game header shows total charged, not a selected person's balance.

Imported regular and extra-hour records on the same play date in the same import form one game, consistent with session history. Native settlement transactions stay distinct, including reversed settlements and later corrections. Native sessions on the same date are not merged. Non-game entries retain their existing behavior.

Name the tab "Ledger". Mine filters the list to games the caller took part in, but expansion retains the complete split. All shows all games. Top-up-only excludes game rows. Group before applying pagination so page boundaries cannot split or repeat a game. No ledger entries or money values are modified.

Acceptance: One game row expands/collapses by mouse and keyboard. Its shares reconcile with source charges, including a member-funded game and inactive participants. Extra-hour rows combine once. A game with a comped member or a guest preserves those charges correctly. Existing filters and paging still work.

## Correction: Imported session history and mobile assets

An old scheduled session without any native settlement is already accounted for when Splitwise has a game on its stored play date. History shows the imported game once, labelled "Settled in Splitwise". Suppress the redundant schedule row before counting and paging. Keep the original schedule and all financial records intact. A native settlement, including a reversed one, remains distinct.

Links to the old scheduled ID resolve to the imported breakdown. Preview and settlement requests for that old ID must refuse a second charge. Combine regular and extra-hour source rows as before. Match the play date, not the date when Splitwise recorded the expense.

Acceptance: Test duplicate schedule rows, a cancelled schedule, source/play-date differences, pagination, native active/reversed settlements, and unchanged ledger balances after a rejected duplicate charge. Inspect all three asset cards at 320px and 390px widths and desktop. Values, counts and dates must remain visible without horizontal scrolling.
