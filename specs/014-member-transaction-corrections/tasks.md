# Tasks: transaction corrections and shared expenses

Audited 2026-10-25 against the current checkout and GitHub's merged PR list.
Checked items are complete at the stated scope. An existing primitive does not
mean its new shared-expense or member-correction integration is complete.
Evidence and requirement-by-requirement gaps: [implementation-status.md](implementation-status.md).

Design completed 2026-10-26 after merging PR #103 (`9b83151`).
[implementation-design.md](implementation-design.md), [data-model.md](data-model.md),
[research.md](research.md), contracts/ and quickstart.md are the implementation inputs.
The ignored local plan.md is the Spec Kit entry point; implementation-design.md is
the tracked summary. D/S/R identifiers below retain the historical scope audit.
Sequential T identifiers below are the executable work list; unchecked means unbuilt.

## Specification — done

- [x] S001 Formalize the specification and review its requirements.
- [x] S002 Specify session stock valuation, inactive-member and history-date rules.
- [x] S003 Record the quality checklist and resolved review findings.
- [x] S004 Reconcile the proposal with shipped PRs and separate existing work from gaps.

## Already implemented — reuse without rebuilding

- [x] D001 Integer-cent ledger posting, balanced movements and club-position checks
  with account locks in stable ID order — PR #24; FR-016 foundation.
- [x] D002 Immutable ledger entries and original/reversal transaction links —
  PR #24; FR-011/012/019 foundation.
- [x] D003 Exact transaction reversal, including top-up bank/member effects and
  shuttle units, with a second reversal rejected — PR #24; FR-012 foundation.
- [x] D004 Settlement reversal restores original charges/assets and permits
  resettlement on the successful path — PR #24; atomicity remains R001.
- [x] D005 Deterministic largest-remainder splitting and exact stock unit/value
  arithmetic — PR #24; reuse for FR-004/014/015, not a new split engine.
- [x] D006 Settlement rate snapshots, base/extra-hour participant splits,
  hosted guests and comped-share surplus costing — PR #24; FR-009/014/015 foundation.
- [x] D007 Personal/all-member ledger views, top-up filtering, inactive labels
  and full-history running balances before filtering/paging — PR #36;
  existing portions of FR-001/019/021/022.
- [x] D008 Grouped native/imported session rows before pagination, expandable
  member shares, gross paid/owed information and balances — PR #36;
  existing session portions of FR-006/020. New shared-expense rows remain R009.
- [x] D009 Actual-count expense preview/confirm, attendance-bound cost fingerprint,
  stale-cost rejection and serialized stock consumption — PR #42;
  FR-018 foundation, not new correction/balance-bound previews.
- [x] D010 Sydney play-date personal spend, gross imported session charges,
  hosted guest charges and exclusion of reversed charges/non-session spending —
  PR #42; existing portion of FR-023.
- [x] D011 Audited bank/court asset-balance correction with review and historical
  audit restrictions — PR #67; existing workflow preserved by FR-025.
- [x] D012 Member top-up disclosure and header balance refresh after recording —
  PR #73; existing entry point and refresh portion of FR-002/024.
- [x] D013 Club asset disclosure/purchase forms and position refresh — PR #74;
  existing asset UI preserved, not a new form to implement here.
- [x] D014 Asset-purchase correction: mandatory reason, atomic exact reversal and
  linked replacement, original date retention, same-payload retry, competing-write
  protection, stock guards, admin access and review UI — PR #102;
  existing asset scope of FR-010/011/017/019/025.
- [x] D015 Import/audit reversal exclusions and imported-session settlement guard —
  PRs #36/#43/#67; existing restrictions to preserve under FR-025.

## Remaining implementation

- [x] R001 Make session reversal atomic with settlement state; close generic
  session reversal and reversal-of-reversal bypasses (FR-012/013/017/025).
- [x] R002 Complete detailed schema/contracts, preview binding, lock-order design
  and dependency-ordered execution tasks; reuse D001–D015.
- [ ] R003 Add durable submission identity/payload replay for new member operations;
  retain successful retries before stale-review checks (FR-017/018).
- [ ] R004 Add top-up correction preview/confirm for member, amount, date and note;
  existing RecordTopup and generic reversal do not provide this (FR-009–FR-012).
- [ ] R005 Add shared-expense/version/allocation models, explicit migration and
  test-harness truncation; retain zero-net allocations (FR-003–FR-008/020).
- [ ] R006 Add shared-expense exact preview/confirm using D005, with personal,
  multiple-payer, club funding and custom shares (FR-003–FR-008/016/018).
- [ ] R007 Add shared-expense correction/reversal and atomic rollback/concurrency
  coverage, reusing D002/D014 patterns (FR-010–FR-012/017).
- [ ] R008 Add mobile shared-expense entry and member correction/reversal review
  controls, required reasons and exact before/after outcomes (FR-002/006/010/012/024).
- [ ] R009 Extend D007/D008 with shared-expense grouping, zero-net personal history,
  correction links and Sessions/Shared expenses filters including their inverses
  (FR-019–FR-021). Existing grouped game history is already done.
- [ ] R010 Implement member-operation effective dates and recording-time display,
  preserving full-history running balances from D007 (FR-019/022).
- [ ] R011 Add native-session correction from saved inputs, preserving original
  attendance-only costs and implementing shuttle delta valuation (FR-009/013–FR-015).
- [ ] R012 Apply approved-admin mutation access, inactive-original-member rules,
  exact reviewed-balance binding and new-type restrictions (FR-001/007/018/025).
- [ ] R013 Integrate new operations with existing balance/position/spend refresh;
  verify new shared expenses stay excluded from D010 and send no new notifications
  (FR-023/024/026). Existing spend queries/header event need reuse, not replacement.
- [ ] R014 Run new scratch-database race/rollback/access tests, frontend tests,
  lint, build and mobile browser QA against all acceptance scenarios.

## PR 1 — merged as #103

[pr1-reversal-safety.md](pr1-reversal-safety.md) records the implementation,
contracts, lock order and regression coverage. R001 is complete: both reversal
endpoints share an atomic session path; reversal-of-reversal is rejected. Approved
admin access is enforced for both routes. R012 remains open for the remaining
new mutation types and inactive-participant/preview rules.

Full backend `go test -race ./...` passed against the dedicated disposable
PostgreSQL database, including all database-backed packages. No frontend/schema
changes are needed for PR 1. Later PR work remains unchecked above.

## Verification during the earlier baseline audit

- Passed `go test ./internal/services/money` (cached).
- Passed the existing LedgerBrowser, LedgerGameRow, AssetPurchaseHistory and
  TopupForm test files: 4 files, 25 tests.
- Inspected the existing database-backed tests for ledger, stock restoration,
  history, expense previews, spend and asset corrections. They were not rerun
  against a scratch database during this documentation audit.
- Full feature validation remains R014; passing existing tests does not establish
  support for new shared expenses or member/session correction flows.


## Phase 1 — setup/design (completed; PR 2 input)

- [x] T001 Record merged baseline, constitution checks, complete design/contracts and executable task sequence in specs/014-member-transaction-corrections/implementation-design.md and related artifacts.

No new project/toolchain setup is needed. Reuse D001–D015 and R001. Tests below are
required by the financial/concurrency acceptance criteria and constitution.

## Phase 2 — foundation (PR 2; blocks new user operations)

Goal: one reviewed, durable, transaction-safe operation protocol, including
reviewed reversal for top-ups/native sessions. Independent check: retry, rollback,
competing requests and membership/import contention produce the exact contract.

- [ ] T002 Add MoneyOperation UUID/sequence/payload/result model and nullable transaction operation links in backend/internal/models/money_operation.go and backend/internal/models/ledger.go per data-model.md.
- [ ] T003 Add additive idempotent migration, deferred target FK, operation-step uniqueness/checks and upgrade coverage in backend/internal/database/db.go and backend/internal/database/member_money_migration_test.go; register money_operations in backend/internal/testsupport/db.go.
- [ ] T004 Write replay/hash/rollback/dual-reservation-original-lock contention tests in backend/internal/services/member_money_service_test.go, including identical retry after subsequent correction and request-ID reuse by another actor.
- [ ] T005 Implement strict canonical input/digest/numeric helpers, duplicate JSON-key rejection and common DTO/errors in backend/internal/services/member_money_service.go and backend/internal/handlers/member_money.go; use contracts/operations.md bounds and exact same request replay.
- [ ] T006 Implement caller-tx reservation/replay/completion and table-intent helper in backend/internal/services/member_money_service.go and backend/internal/services/ledger_service.go; no pending reservation may commit and successful replay precedes stale-state checks.
- [ ] T007 Apply early table intent before prelocks in backend/internal/services/ledger_service.go, settlement_service.go, asset_correction.go and balance_correction.go; audit splitwise_import_service.go and backend/cmd/seed callers without weakening import locks or audit guards.
- [ ] T008 Write membership/removal/new-RSVP/profile/last-admin concurrency and rollback tests in backend/internal/services/user_service_test.go and backend/internal/services/rsvp_service_test.go.
- [ ] T009 Add sorted user SHARE locking and transaction-aware membership/removal with NO KEY UPDATE, last-admin predicate guard, targeted profile/role writes and after-commit promotion delivery in backend/internal/services/user_service.go; pass caller tx through cancellation helpers.
- [ ] T010 Add tx-aware RSVP removal/promotion helpers and target-user approval locking before session in backend/internal/services/rsvp_service.go; adapt ordinary settlement participant discovery/revalidation before session in backend/internal/services/settlement_service.go.
- [ ] T011 Extract transaction-scoped inverse/settlement undo helpers and implement reviewed top-up/native-session reversal previews/confirms in backend/internal/services/member_money_service.go, ledger_service.go and settlement_service.go; prelock complete account union, backdate only new reviewed inverses, suppress new notifications.
- [ ] T012 Register approved-member transaction detail and approved-admin preview/confirm routes in backend/internal/handlers/member_money.go and backend/cmd/server/main.go; coordinate supported legacy reversal envelope dispatch and backend/internal/handlers/openapi.yaml in the same change.
- [ ] T013 Add HTTP access/strict-input/unsupported-kind/legacy-bypass tests in backend/internal/handlers/member_money_test.go; extend backend/internal/services/reversal_safety_test.go with reviewed path and importer/account-lock race coverage.
- [ ] T014 Add common reviewed DTO and API methods plus exact retry-payload state helpers in frontend/src/types/index.ts, frontend/src/services/api.ts and frontend/src/components/money/useMoneyOperation.ts; preserve current HTTP client and existing unrelated asset workflows.

Gate: T002–T014 complete with scratch-DB race suite; existing asset/import tests
pass. Shared kind is not enabled until its type dispatch and read path exist.

## Phase 3 — US1 correct/reverse a top-up (P1; PR 3, first UI increment)

Goal: edit saved member/amount/date/note or undo, review exact effects and retain
history. Independent test: quickstart scenario 1 plus debt, conflict and cancel.

- [ ] T015 [US1] Write top-up amount/recipient/date/note correction, inactive-original, metadata-only, no-change, inverse immutability and bank/member invariant cases in backend/internal/services/topup_correction_test.go.
- [ ] T016 [US1] Implement top-up saved-input validation, exact inverse/linked replacement and reviewed before/after union in backend/internal/services/topup_correction.go using the foundation caller transaction and original-kind dispatch.
- [ ] T017 [US1] Extend backend/internal/handlers/member_money.go and backend/internal/handlers/openapi.yaml for top-up correction transport and transaction detail/allowed-action links; test failures/access in backend/internal/handlers/member_money_test.go.
- [ ] T018 [US1] Implement common effective/recording-group operation order and expose transaction IDs/times/audit links in backend/internal/services/ledger_history.go and backend/internal/services/ledger_activity.go, preserving all legacy relative order per contracts/history.md.
- [ ] T019 [US1] Add tied timestamp, same-date exact-time preservation, changed-date/DST and full-history-balance tests in backend/internal/services/ledger_history_test.go and ledger_activity_test.go; top-up filter includes inverse rows.
- [ ] T020 [US1] Implement saved top-up editor and reusable old/new effects review with mandatory reason, exact retry and stale-review behavior in frontend/src/components/money/TopupCorrection.tsx and MoneyOperationReview.tsx.
- [ ] T021 [US1] Add transaction detail/correction-chain navigation and supported Correct/Reverse controls to frontend/src/components/money/LedgerList.tsx, LedgerGameRow.tsx and frontend/src/pages/SessionSettlement.tsx; inactive/reversal records link to current version without mutation controls.
- [ ] T022 [US1] Integrate accepted/replayed operation refresh with existing balance-change event and history/position/spend reload in frontend/src/pages/Money.tsx and frontend/src/components/money/LedgerBrowser.tsx.
- [ ] T023 [US1] Test review/cancel/reason, conflict retaining edits, unknown-response exact retry, member access and live refetch in frontend/src/components/money/TopupCorrection.test.tsx and MoneyOperationReview.test.tsx.
- [ ] T024 [US1] Add desktop/mobile top-up correction and reviewed top-up/session reversal journeys in frontend/e2e/member-money-correction.spec.ts using existing synthetic fixtures.

US1 gate: fully usable editor/review/history/refresh and HTTP/backend checks pass;
no shared expense or session correction is required for this increment.

## Phase 4 — US2 record a shared expense (P1; PR 4)

Goal: personal single/multiple payer or bank-funded equal/custom bill with full
zero-net detail. Independent test: $120/4, multiple payer, bank and 1-cent/3 cases.

- [ ] T025 [US2] Add immutable version/allocation models including participates, root seed and session restriction in backend/internal/models/shared_expense.go; migrate/check/index them in backend/internal/database/db.go and reset both tables in backend/internal/testsupport/db.go.
- [ ] T026 [P] [US2] Add deterministic equal/custom paid/owed allocation and checked cent-total tests in backend/internal/services/money/shared_expense_test.go, including reorder, one participant, payer-only and one cent across three participants.
- [ ] T027 [US2] Implement pure shared allocation validation using existing largest remainder in backend/internal/services/money/shared_expense.go; keep paid/owed/participation distinct and use stable expense root UUID.
- [ ] T028 [US2] Write personal/bank-funded invariant, all-zero-net leg retention, stale review, retry and allocation-write rollback tests in backend/internal/services/shared_expense_service_test.go.
- [ ] T029 [US2] Implement shared create preview/confirm, memberships, original session context locking and zero member legs through PostWithin in backend/internal/services/shared_expense_service.go; add native kind dispatch guards before any shared write is enabled.
- [ ] T030 [US2] Add approved admin shared entry/suggestion routes, strict contracts/OpenAPI and access tests in backend/internal/handlers/shared_expense.go, shared_expense_test.go, openapi.yaml and backend/cmd/server/main.go.
- [ ] T031 [US2] Extend full grouped activity with shared shape, participates/zero-net personal discovery and basic shared type filter in backend/internal/services/ledger_activity.go and ledger_history.go; protect referenced session deletion in backend/internal/services/session_service.go.
- [ ] T032 [US2] Add zero-net/payer-only/group-before-page/shared-linked-session-spend-exclusion and deletion tests in backend/internal/services/ledger_activity_test.go, ledger_spend_test.go and session_service_test.go.
- [ ] T033 [US2] Add shared types/API methods and entry disclosure beside top-ups in frontend/src/types/index.ts, frontend/src/services/api.ts and frontend/src/pages/Money.tsx.
- [ ] T034 [US2] Build mobile description/date/category/funding/payer/participant/equal-custom form and explicit session suggestions in frontend/src/components/money/SharedExpenseForm.tsx using common review/refresh behavior.
- [ ] T035 [US2] Build full Paid/Share/Change/Balance-after grouped row and integrate personal/all-member history in frontend/src/components/money/LedgerSharedExpenseRow.tsx and LedgerBrowser.tsx.
- [ ] T036 [US2] Test all entry funding/split modes, zero-net detail, invalid totals, retry/staleness and mobile journey in frontend/src/components/money/SharedExpenseForm.test.tsx, LedgerSharedExpenseRow.test.tsx and frontend/e2e/shared-expenses.spec.ts.

US2 gate: readable retained allocations accompany every new record; shared never
counts as badminton spend. Do not expose entry with an old backend revision.

## Phase 5 — US3 correct native session (P1; default PR 6)

Goal: correct the entire exact live settlement using saved costs/mode. Independent
test: attendance-only four→three, stock decrease/increase and rollback/contention.
May be scheduled after US1 independently of US2/US4; default PR order is below.

- [ ] T037 [US3] Add nullable exact court band snapshots and stable line allocation keys in backend/internal/models/settlement.go, explicit migration/upgrade handling in backend/internal/database/db.go and saved snapshots for new ordinary settlement in backend/internal/services/settlement_service.go.
- [ ] T038 [US3] Write frozen-band/stock-delta/overflow/zero/full-use/legacy-base-then-extra tests in backend/internal/services/money/settlement_correction_test.go using the six/$24 examples and unchanged later-session inputs.
- [ ] T039 [US3] Extract pure allocation from stock valuation and implement checked whole-count/legacy-band delta rules in backend/internal/services/money/settlement_correction.go and backend/internal/services/settlement_service.go without changing ordinary settlement results.
- [ ] T040 [US3] Write attendance-only, comped/guest, everyone-stays, mode retention, metadata/key-only/no-change, legacy reconciliation and inactive-original tests in backend/internal/services/settlement_correction_test.go.
- [ ] T041 [US3] Implement saved editor normalization, exact legacy verification, fingerprint and atomic old-state/inverse/replacement/lines in backend/internal/services/settlement_correction.go; never call independent public Settle/Reverse or their notifiers.
- [ ] T042 [US3] Add failure injection, correction-vs-settle/reverse and union-account concurrency tests in backend/internal/services/settlement_correction_test.go; assert one live settlement, invariant, exact review and original immutability.
- [ ] T043 [US3] Register exact-settlement editor/preview/confirm routes and errors in backend/internal/handlers/member_money.go, backend/cmd/server/main.go and backend/internal/handlers/openapi.yaml; test route approval and imported/unsupported cases in member_money_test.go.
- [ ] T044 [US3] Build saved actual/estimated session editor with stable line keys and original cost review in frontend/src/components/money/SessionCorrection.tsx; integrate from frontend/src/pages/SessionSettlement.tsx with history/spend refresh.
- [ ] T045 [US3] Test original defaults despite later rate/stock changes, guest/comped legacy controls, mode restrictions, stock shortage and explicit renewed review in frontend/src/components/money/SessionCorrection.test.tsx and frontend/e2e/member-money-correction.spec.ts.

## Phase 6 — US4 correct/reverse shared expense (P2; PR 5)

Goal: change any bill field or undo a current version, preserving all allocations
and funding effects. Independent test: personal→bank funding, then undo replacement.

- [ ] T046 [US4] Add shared correction/reversal invariants, funding switch, metadata-only, no-change and inactive union cases in backend/internal/services/shared_expense_service_test.go.
- [ ] T047 [US4] Implement current-version-only exact inverse/linked immutable replacement and same-root seeded preview in backend/internal/services/shared_expense_service.go and member_money_service.go.
- [ ] T048 [US4] Add shared dual-confirmation, correction-vs-reversal, allocation/result failure rollback and repeated correction-chain tests in backend/internal/services/shared_expense_service_test.go; assert original allocations unchanged.
- [ ] T049 [US4] Extend detail/dispatch/OpenAPI and legacy-reviewed reverse refusal tests in backend/internal/handlers/member_money.go, member_money_test.go and openapi.yaml for shared kinds.
- [ ] T050 [US4] Add saved shared editor/correction/reversal actions and reason/audit links in frontend/src/components/money/SharedExpenseForm.tsx and LedgerSharedExpenseRow.tsx, preserving exact unknown-response retry payloads.
- [ ] T051 [US4] Test funding/category/date/description/allocation correction and reversal audit navigation in frontend/src/components/money/SharedExpenseForm.test.tsx and frontend/e2e/shared-expenses.spec.ts.

## Phase 7 — US5 complete history and clarity (P2; PR 7)

Goal: complete filters, audit/time explanation and zero-net history across pages.
Independent test: >50 mixed original/inverse/replacement/imported activities in
every scope/type with raw/full-detail balance reconciliation. US1/US2 already
ship their essential history; this phase closes the full cross-type matrix.

- [ ] T052 [US5] Complete All/Sessions/Top-ups/Shared expenses filters with legacy type=topup alias, original-family inverses and consistent ordering across raw/grouped readers in backend/internal/services/ledger_history.go, ledger_activity.go and backend/internal/handlers/ledger.go.
- [ ] T053 [US5] Add mixed-source >50-row scope/type/group/paging, zero-net inverse, tied operation sequence and backdated running-balance tests in backend/internal/services/ledger_history_test.go and ledger_activity_test.go.
- [ ] T054 [US5] Add HTTP contract coverage for additive identity/time/allowed-action fields, complete filtered groups and aliases in backend/internal/handlers/ledger_test.go and member_money_test.go; finalize backend/internal/handlers/openapi.yaml.
- [ ] T055 [US5] Complete type selector, effective/recorded dates, original/replacement navigation and recalculated-balance explanation in frontend/src/components/money/LedgerBrowser.tsx, LedgerList.tsx, LedgerGameRow.tsx and LedgerSharedExpenseRow.tsx.
- [ ] T056 [US5] Validate selected-member/full-group visibility, inactive labels, ordinary-member read access and all type filters in frontend/src/components/money/LedgerBrowser.test.tsx and frontend/e2e/money.spec.ts.

## Phase 8 — final verification and documentation (PR 7 gate)

- [ ] T057 Re-run full scratch-database race/rollback/access/import/asset compatibility matrix for backend/internal/services and backend/internal/handlers; check new operation notifications remain silent and ordinary settlement notification coverage still passes.
- [ ] T058 Run frontend tests/lint/build and desktop/mobile Playwright acceptance in frontend/e2e/member-money-correction.spec.ts, shared-expenses.spec.ts and money.spec.ts; resolve focus/labels/360px overflow and form-retention failures.
- [ ] T059 Verify additive migration on a populated scratch pre-feature schema, repeat migrate and backend rollout/new-kind rollback constraints in backend/internal/database/member_money_migration_test.go; record actual results in specs/014-member-transaction-corrections/quickstart.md.
- [ ] T060 Update actual completion/evidence and operational behavior in specs/014-member-transaction-corrections/tasks.md, implementation-status.md and quickstart.md; only mark scope checks complete after corresponding tests pass.

## Dependencies, PR boundaries and independent increments

Execution graph: T001 → foundation T002–T014 → US1 T015–T024 → US2 T025–T036
→ US4 T046–T051. US3 T037–T045 depends on foundation and US1 review/history but
not on shared expenses; default scheduling is after US4. US5 completion T052–T056
integrates all three types; final T057–T060 requires every story.

Default PR sequence: #103 merged → PR 2 foundation → PR 3 US1 → PR 4 US2 →
PR 5 US4 → PR 6 US3 → PR 7 US5/full verification. The first usable new correction
increment is PR 3; it must not start before the design/foundation gates. Each PR
runs its own relevant checks and includes its necessary history/refresh, not just
the final test PR. All implementation tasks remain unchecked in this design PR.

Parallel opportunities: after relevant model/protocol decisions land, tests and UI
for US1 may develop separately from backend implementation, but gate on agreed
DTOs. US2 pure arithmetic tests (T026 [P]) can run alongside schema work T025.
US3 pure arithmetic tests can be prepared while saved-editor components develop;
shared files such as api.ts, types and ledger readers must be integrated serially.
US4 UI tests may be prepared independently of service tests once US2 contracts
exist. US5 frontend filter tests and backend paging fixtures can be prepared
independently; common order changes must land together. These describe scheduling
options, not an instruction to spawn agents during implementation.

## Requirement traceability and R-scope closure

FR-001: T009–T013/T017/T030/T043/T049/T054. FR-002: T033–T036.
FR-003–FR-005: T025–T029/T034/T036. FR-006: T005/T020/T034/T044.
FR-007: T008–T010/T015/T028/T040/T046. FR-008: T030–T032/T034.
FR-009: T015–T017/T037–T045. FR-010–FR-012: T011–T013/T015–T024/T046–T051.
FR-013–FR-015: merged R001 plus T037–T045. FR-016: T006–T007/T016/T028/T039/T042.
FR-017–FR-018: T002–T014 and each story's contention/stale-preview tests.
FR-019: T018–T024/T035/T050/T053–T056. FR-020: T031–T032/T035/T053.
FR-021: T019/T031/T052–T056. FR-022: T018–T019/T053/T055.
FR-023: T022/T032/T044/T057. FR-024: T020–T024/T033–T036/T044–T045/T050.
FR-025: T007/T011–T013/T030–T032/T043/T049/T057.
FR-026: T011/T029/T041/T057.

R002 completed by design/T001. R003 maps T002–T014; R004 T015–T024; R005
T025; R006 T026–T036; R007 T046–T051; R008 UI tasks across US1/2/3/4;
R009 T018–T019/T031–T032/T052–T056; R010 T018–T019/T053/T055;
R011 T037–T045; R012 foundation and per-type access/membership tasks;
R013 each story's refresh/spend/silence tasks; R014 T057–T060 plus per-story tests.
