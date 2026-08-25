---

description: "Task list for Club Ledger and Session Settlement"
---

# Tasks: Club Ledger and Session Settlement

**Input**: Design documents from `/specs/001-club-ledger-settlement/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test tasks are included and are **not optional here**. Constitution Principle VII
requires the accounting invariants to be enforced in code *and* covered by tests, and
Principle VIII requires CI coverage floors to be met.

**Organization**: Grouped by user story so each is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on incomplete work)
- **[Story]**: US1–US4, mapping to the prioritised stories in spec.md

## Path Conventions

Web app: `backend/internal/...`, `frontend/src/...` per plan.md.

---

## Phase 1: Setup

**Purpose**: Make the work verifiable before any of it is written.

- [X] T001 Start the scratch test database and confirm `TEST_DATABASE_URL` runs the existing suite green, per the commands in specs/001-club-ledger-settlement/quickstart.md
- [X] T002 [P] Create the pure-arithmetic package with its doc comment explaining why it holds no database access, in backend/internal/services/money/doc.go
- [X] T003 [P] Add `Account`, `Transaction`, `LedgerEntry`, `Settlement`, `ChargeLine` TypeScript types in frontend/src/types/index.ts

**Checkpoint**: Tests runnable, empty scaffolding in place.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The ledger core. Every user story depends on this.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Models and schema

- [X] T004 [P] Create `Account` model with `kind`, nullable `user_id`, and `BeforeCreate` UUID hook in backend/internal/models/account.go
- [X] T005 [P] Create `Transaction` and `LedgerEntry` models, with `reverses_transaction_id` and the nullable `units` column, in backend/internal/models/ledger.go
- [X] T006 Register the three new models in `database.Migrate()` in backend/internal/database/db.go
- [X] T007 Add the raw-SQL constraints AutoMigrate cannot express — unique index on `transactions.reverses_transaction_id`, and the partial unique index on live settlements — guarded by `IF NOT EXISTS`, in backend/internal/database/db.go
- [X] T008 Seed the four club accounts (bank, court credit, shuttle stock, surplus) alongside the existing default-club seed in backend/internal/database/db.go

### Money arithmetic (no database)

- [X] T009 [P] Implement `SplitLargestRemainder(total int64, participants []uuid.UUID, seed uuid.UUID)` returning exact per-participant cents, in backend/internal/services/money/split.go
- [X] T010 [P] Implement `ConsumeStock(stockCents int64, stockUnits int, wantUnits int)` returning consumption value with round-half-up integer arithmetic, in backend/internal/services/money/stock.go
- [X] T011 [P] Unit-test `SplitLargestRemainder`: sums exactly for n=1..20 across awkward totals, is deterministic for a given seed, and varies the odd-cent recipient across seeds, in backend/internal/services/money/split_test.go
- [X] T012 [P] Unit-test `ConsumeStock` including the $50/12 tube case, blended stock, and the zero-units guard, in backend/internal/services/money/stock_test.go

### Ledger service

- [X] T013 Define typed errors and their wire codes (`shuttle_stock_short`, `session_already_settled`, `transaction_already_reversed`, `invariant_violated`, `not_settleable`) per contracts/README.md, in backend/internal/services/ledger_errors.go
- [X] T014 Implement `LedgerService.post()` — opens a transaction, locks the touched accounts `FOR UPDATE` in `id` order, writes entries, asserts the club-position identity, and rolls back on violation — in backend/internal/services/ledger_service.go
- [X] T015 Implement balance derivation (`BalanceOf`, `AllPlayerBalances`, `StockPosition`) aggregating over `ledger_entries` with the sign convention from research.md R1, in backend/internal/services/ledger_service.go
- [X] T016 Implement account provisioning — create a player account when a user reaches `approved` — in backend/internal/services/user_service.go
- [X] T017 [P] Test that `post()` rolls back and writes nothing when handed unbalanced movements, in backend/internal/services/ledger_service_test.go
- [X] T018 [P] Test that `LedgerService` exposes no method that updates or deletes a `LedgerEntry` (invariant I4), in backend/internal/services/ledger_service_test.go

**Checkpoint**: Money can be moved safely and the invariant cannot be violated. User stories can begin.

---

## Phase 3: User Story 1 — Keep the books without Splitwise (Priority: P1) 🎯 MVP

**Goal**: Every player has a balance, the admin records top-ups, everyone can see balances and their own history.

**Independent Test**: Seed opening balances, record top-ups, confirm all members see the same balances and each player's history reconciles to their balance.

### Backend

- [ ] T019 [US1] Implement `RecordTopup` and `RecordWithdrawal` on backend/internal/services/ledger_service.go
- [ ] T020 [US1] Implement `RecordOpeningBalances`, computing surplus as the balancing figure and rejecting a second call, in backend/internal/services/ledger_service.go
- [ ] T021 [US1] Implement `ReverseTransaction`, posting the exact negation including `units`, in backend/internal/services/ledger_service.go
- [ ] T022 [US1] Implement `MyEntries` with running `balance_after_cents` over the ordered entries, in backend/internal/services/ledger_service.go
- [ ] T023 [US1] Create handlers for the read endpoints in contracts/ledger.md (`GET /api/accounts`, `/api/accounts/me`, `/api/accounts/me/entries`) in backend/internal/handlers/ledger.go
- [ ] T024 [US1] Create handlers for the write endpoints (topup, withdrawal, opening-balances, reverse) in backend/internal/handlers/ledger.go
- [ ] T025 [US1] Register reads on the `approved` group and every write on the `admin` group in backend/cmd/server/main.go — Principle III, verify the group before moving on

### Backend tests

- [ ] T026 [P] [US1] Test top-up moves bank and player together and leaves the identity balanced, in backend/internal/services/ledger_service_test.go
- [ ] T027 [P] [US1] Test reversal restores balances, leaves both transactions visible, and refuses a second reversal, in backend/internal/services/ledger_service_test.go
- [ ] T028 [P] [US1] Test opening balances seed players, assets and surplus so the identity holds, and reject a second call, in backend/internal/services/ledger_service_test.go
- [ ] T029 [P] [US1] Test `MyEntries` running balance equals the derived balance at every row, in backend/internal/services/ledger_service_test.go

### Frontend

- [ ] T030 [P] [US1] Add ledger calls (`getAccounts`, `getMyAccount`, `getMyEntries`, `recordTopup`, `recordWithdrawal`, `recordOpeningBalances`, `reverseTransaction`) to frontend/src/services/api.ts
- [ ] T031 [P] [US1] Create `formatCents` and `BalanceChip` (colour-coded ok/low/negative) in frontend/src/components/money/BalanceChip.tsx
- [ ] T032 [US1] Add the balance chip to the header, right of the avatar, in frontend/src/components/layout/Header.tsx
- [ ] T033 [US1] Add the Money item to the bottom nav in frontend/src/components/layout/Navigation.tsx
- [ ] T034 [US1] Create the Money page shell with Balances / My ledger tabs (Club assets tab added in US3) in frontend/src/pages/Money.tsx
- [ ] T035 [P] [US1] Create the balances list in frontend/src/components/money/BalancesList.tsx
- [ ] T036 [P] [US1] Create the itemised ledger list with running balance in frontend/src/components/money/LedgerList.tsx
- [ ] T037 [US1] Create the admin top-up form in frontend/src/components/money/TopupForm.tsx
- [ ] T038 [US1] Register the `/money` route in frontend/src/App.tsx

### Frontend tests

- [ ] T039 [P] [US1] Test `formatCents` never uses floating point and renders negatives correctly, in frontend/src/components/money/BalanceChip.test.tsx
- [ ] T040 [P] [US1] Test the Money page renders balances and the caller's history, in frontend/src/pages/Money.test.tsx

**Checkpoint**: Splitwise can be switched off. This is the MVP.

---

## Phase 4: Session Timestamps (Independent)

**Purpose**: Give sessions a dependable past/upcoming boundary and remove the read-time `HH:MM` parse. Per research.md R10 this blocks US2's history listing but **not** US1 — it can be done in parallel with Phase 3.

- [ ] T041 Add nullable `starts_at` / `ends_at` `timestamptz` fields to the `Session` model in backend/internal/models/session.go
- [ ] T042 [P] Implement `ResolveSessionTimes(date time.Time, start, end string)` returning Sydney-resolved instants, DST-safe, in backend/internal/utils/time.go
- [ ] T043 Populate the timestamps on session create and update in backend/internal/services/session_service.go
- [ ] T044 Add the idempotent SQL backfill (`WHERE starts_at IS NULL`, using `AT TIME ZONE 'Australia/Sydney'`) to `database.Migrate()` in backend/internal/database/db.go
- [ ] T045 Replace `parseSessionDateTime` with the stored `starts_at` in backend/internal/services/scheduler_service.go
- [ ] T046 Switch upcoming/cancelled session listing from `session_date >= today` to `ends_at >= now()` in backend/internal/services/session_service.go
- [ ] T047 [P] Test resolution across the October and April DST boundaries, in backend/internal/utils/time_test.go
- [ ] T048 [P] Test that a session which ended earlier today is no longer listed as upcoming, in backend/internal/services/session_service_test.go

**Checkpoint**: Session times are trustworthy; Principle IV satisfied.

---

## Phase 5: User Story 2 — Settle a night's play (Priority: P2)

**Goal**: Turn a played session into per-player charges, drawn from club assets.

**Independent Test**: Settle a session with full-night players, an early leaver, a guest and a no-show; confirm each charge is right, charges sum exactly to cost, and assets fall by the same total.

**Depends on**: Phase 2, Phase 4.

### Backend

- [ ] T049 [P] [US2] Create `Settlement` model with snapshotted rates and `reversed_at` in backend/internal/models/settlement.go
- [ ] T050 [P] [US2] Create `ChargeLine` model with `in_base`, `in_extra`, `comped`, `guest_name` in backend/internal/models/settlement.go
- [ ] T051 [US2] Add the club rate and threshold settings fields, with defaults, to backend/internal/models/club.go
- [ ] T052 [US2] Register the new models and seed club setting defaults in backend/internal/database/db.go
- [ ] T053 [US2] Implement band costing — court cost per band, shuttle units per band, per-band split — in backend/internal/services/settlement_service.go
- [ ] T054 [US2] Implement `Preview`, returning bands, lines and `stock_after` without writing, in backend/internal/services/settlement_service.go
- [ ] T055 [US2] Implement `Settle` — session row lock, live-settlement check, stock sufficiency check, then a single `LedgerService.post()` covering court credit, shuttle stock, player charges and comped surplus — in backend/internal/services/settlement_service.go
- [ ] T056 [US2] Make settlement reversal stamp `settlements.reversed_at` so the session can be settled again, in backend/internal/services/settlement_service.go
- [ ] T057 [US2] Implement session history listing and the public settlement breakdown in backend/internal/services/settlement_service.go
- [ ] T058 [US2] Create settlement handlers per contracts/settlement.md in backend/internal/handlers/settlement.go
- [ ] T059 [US2] Register preview and settle on `admin`, history and breakdown on `approved`, in backend/cmd/server/main.go — Principle III

### Backend tests

- [ ] T060 [P] [US2] Test the worked example from data-model.md end to end: band totals, the 1695/1694 and 1096/1095 splits, and stock at 9 units / $37.50 afterwards, in backend/internal/services/settlement_service_test.go
- [ ] T061 [P] [US2] Test an early leaver pays nothing toward the extra hour and stayers cover it, in backend/internal/services/settlement_service_test.go
- [ ] T062 [P] [US2] Test a comped line charges zero, leaves every other charge unchanged, and moves surplus by exactly that share, in backend/internal/services/settlement_service_test.go
- [ ] T063 [P] [US2] Test a guest counts as a head and charges the host, in backend/internal/services/settlement_service_test.go
- [ ] T064 [P] [US2] Test short stock returns `shuttle_stock_short` with the shortfall and writes nothing, in backend/internal/services/settlement_service_test.go
- [ ] T065 [P] [US2] Test a second settle returns `session_already_settled`, and that reversing then re-settling works, in backend/internal/services/settlement_service_test.go
- [ ] T066 [US2] Concurrency test: two simultaneous settle calls produce exactly one settlement, mirroring the existing RSVP capacity test, in backend/internal/services/settlement_service_test.go
- [ ] T067 [P] [US2] Test that changing club rates does not alter an already-settled session, in backend/internal/services/settlement_service_test.go

### Frontend

- [ ] T068 [P] [US2] Add settlement calls (`previewSettlement`, `settleSession`, `listSessionHistory`, `getSessionSettlement`) to frontend/src/services/api.ts
- [ ] T069 [US2] Add Upcoming / History tabs to frontend/src/pages/Sessions.tsx
- [ ] T070 [P] [US2] Create the past-session card showing total and player count in frontend/src/components/sessions/PastSessionCard.tsx
- [ ] T071 [P] [US2] Create the settlement breakdown view, readable by any member, in frontend/src/components/settlement/SettlementBreakdown.tsx
- [ ] T072 [US2] Create the admin settlement form — participant rows pre-ticked, extra-hour toggle, comp toggle, rate overrides — in frontend/src/pages/AdminSettlement.tsx
- [ ] T073 [P] [US2] Create the participant row and guest row controls in frontend/src/components/settlement/ParticipantRow.tsx
- [ ] T074 [US2] Re-preview on every form change so displayed numbers always match what will be posted, in frontend/src/pages/AdminSettlement.tsx
- [ ] T075 [US2] Handle `shuttle_stock_short` by offering to record the purchase inline, then retrying, in frontend/src/pages/AdminSettlement.tsx
- [ ] T076 [US2] Register the settlement route in frontend/src/App.tsx

### Frontend tests

- [ ] T077 [P] [US2] Test the settlement form previews on change and posts what it displayed, in frontend/src/pages/AdminSettlement.test.tsx
- [ ] T078 [P] [US2] Test the short-stock inline purchase prompt appears and retries, in frontend/src/pages/AdminSettlement.test.tsx
- [ ] T079 [P] [US2] Test the History tab renders past sessions and their breakdowns, in frontend/src/pages/Sessions.test.tsx

**Checkpoint**: The full Splitwise replacement is live.

---

## Phase 6: User Story 3 — Know the club's true position (Priority: P3)

**Goal**: One view answering whether the club is square with its players.

**Independent Test**: Record a court-credit top-up and a shuttle purchase, settle a session, confirm the position reconciles and the low-credit warning fires.

**Depends on**: Phase 2. Reads better after Phase 5 but does not require it.

- [ ] T080 [US3] Implement `RecordCourtCreditPurchase` and `RecordShuttlePurchase` in backend/internal/services/ledger_service.go
- [ ] T081 [US3] Implement `Position`, returning assets, liabilities, surplus and `balanced`, in backend/internal/services/ledger_service.go
- [ ] T082 [US3] Implement the `court_credit_short` warning by comparing remaining credit against the next scheduled session's cost, in backend/internal/services/ledger_service.go
- [ ] T083 [US3] Implement the integrity check that recomputes balances from entries and reports the residual, in backend/internal/services/ledger_service.go
- [ ] T084 [US3] Create position and asset-purchase handlers in backend/internal/handlers/ledger.go
- [ ] T085 [US3] Register all of these on the `admin` group in backend/cmd/server/main.go — Principle III
- [ ] T086 [P] [US3] Test a court-credit purchase moves no player balance and keeps the identity balanced, in backend/internal/services/ledger_service_test.go
- [ ] T087 [P] [US3] Test buying a tube at a new price blends the average without altering existing stock value, in backend/internal/services/ledger_service_test.go
- [ ] T088 [P] [US3] Test the integrity check reports balanced after a full sequence of top-ups, purchases and settlements, in backend/internal/services/ledger_service_test.go
- [ ] T089 [P] [US3] Add position and purchase calls to frontend/src/services/api.ts
- [ ] T090 [US3] Add the admin-only Club assets tab to frontend/src/pages/Money.tsx
- [ ] T091 [P] [US3] Create the position panel with the assets-vs-liabilities reconciliation and warnings in frontend/src/components/money/PositionPanel.tsx
- [ ] T092 [P] [US3] Create the court-credit and shuttle purchase forms in frontend/src/components/money/AssetPurchaseForms.tsx
- [ ] T093 [P] [US3] Test the Club assets tab is hidden from non-admins, in frontend/src/pages/Money.test.tsx

**Checkpoint**: The club's true position is legible.

---

## Phase 7: User Story 4 — Nudge players who are running low (Priority: P4)

**Goal**: Players below threshold are told, on the back of the settlement that put them there.

**Independent Test**: Settle a session crossing each threshold; confirm the right people get the right notification once, and that opted-out members get nothing.

**Depends on**: Phase 5.

- [ ] T094 [P] [US4] Add `balance_low` and `balance_negative` notification types in backend/internal/models/notification.go
- [ ] T095 [US4] Add `push_balance_alerts` / `email_balance_alerts` preference fields and the two cases in `IsPushEnabledForType` / `IsEmailEnabledForType`, in backend/internal/models/notification.go
- [ ] T096 [US4] Dispatch balance reminders after a successful settlement, to that session's participants only, at most once each, in backend/internal/services/settlement_service.go
- [ ] T097 [US4] Compose reminder copy carrying what the session cost and the resulting balance, in backend/internal/services/notification_service.go
- [ ] T098 [P] [US4] Test low and negative thresholds notify the right participants and nobody else, in backend/internal/services/settlement_service_test.go
- [ ] T099 [P] [US4] Test a low-balance member who did not play is not notified, in backend/internal/services/settlement_service_test.go
- [ ] T100 [P] [US4] Test opted-out members are not notified, in backend/internal/services/settlement_service_test.go
- [ ] T101 [P] [US4] Add the balance alert toggles to frontend/src/components/notifications/NotificationSettings.tsx

**Checkpoint**: All four stories delivered.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [ ] T102 [P] Document the new endpoints in the OpenAPI spec served by backend/internal/handlers/openapi.go
- [ ] T103 [P] Update CLAUDE.md with the ledger service pattern, the sign convention, and the money package
- [ ] T104 Verify coverage floors are met and ratchet `MIN_COVERAGE` up if the new code lifts the baseline, in .github/workflows/ci.yml and frontend/vite.config.ts — Principle VIII
- [ ] T105 [P] Add a Playwright end-to-end run covering top-up → settle → history → balance, in frontend/e2e/
- [ ] T106 Run every validation scenario in specs/001-club-ledger-settlement/quickstart.md against a fresh database and confirm each expected outcome

---

## Dependencies

```
Phase 1 Setup
   └─> Phase 2 Foundational  ◄── blocks everything
          ├─> Phase 3 US1 (P1) ─────────────┐   MVP, shippable alone
          ├─> Phase 4 Session timestamps ───┤   independent, parallel with US1
          │                                  ▼
          │                          Phase 5 US2 (P2)  needs Phase 2 + Phase 4
          │                                  ├─> Phase 7 US4 (P4)
          └─> Phase 6 US3 (P3)  needs Phase 2 only
                                     └─> Phase 8 Polish
```

**Story independence**

- **US1** depends only on Foundational. Ship it alone and Splitwise is redundant.
- **US3** depends only on Foundational — its accounts already exist from Phase 2. It reads better after US2 but does not require it.
- **US2** needs Phase 4 for the past/upcoming boundary.
- **US4** needs US2, because reminders fire on settlement.

## Parallel Opportunities

- **Phase 2**: T004/T005 together; T009–T012 (the whole money package) fully parallel and database-free.
- **Phase 3**: T026–T029 backend tests parallel; T030/T031/T035/T036 frontend parallel.
- **Phase 3 + Phase 4** run concurrently — different files, no shared dependency.
- **Phase 5**: T060–T065 and T067 parallel; T066 must run alone (concurrency test).
- **Phase 6**: T086–T088 and T089/T091/T092/T093 parallel.

## Implementation Strategy

**MVP** = Phase 1 + Phase 2 + Phase 3. At that checkpoint the club has accounts, balances,
top-ups, history and reversal — the whole reason for leaving Splitwise. Everything after is
additive.

**Then** Phase 4 → Phase 5 to get settlement, which is the point at which the app is doing
arithmetic nobody has to check by hand.

**Then** Phase 6 and Phase 7 in either order.

**Order within a phase**: models → pure arithmetic → service → handlers → routes → frontend
→ tests, except in the money package where tests come with the functions because they are
the specification.

**Go-live** is a separate act, not a task here: seed opening balances from Splitwise
(T020's endpoint) once every member is onboarded, along with opening shuttle stock and court
credit.

## Task Summary

| Phase | Tasks | Count |
|---|---|---|
| 1 Setup | T001–T003 | 3 |
| 2 Foundational | T004–T018 | 15 |
| 3 US1 (P1) MVP | T019–T040 | 22 |
| 4 Session timestamps | T041–T048 | 8 |
| 5 US2 (P2) | T049–T079 | 31 |
| 6 US3 (P3) | T080–T093 | 14 |
| 7 US4 (P4) | T094–T101 | 8 |
| 8 Polish | T102–T106 | 5 |
| **Total** | | **106** |
